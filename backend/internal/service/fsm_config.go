package service

import (
	shared "github.com/yqihe/npc-ai-admin/backend/internal/service/shared"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	storemysql "github.com/yqihe/npc-ai-admin/backend/internal/store/mysql"
	storeredis "github.com/yqihe/npc-ai-admin/backend/internal/store/redis"
)

// 条件操作符白名单（对齐游戏服务端 rule.validOps）
var validConditionOps = map[string]bool{
	"==": true, "!=": true,
	">": true, ">=": true,
	"<": true, "<=": true,
	"in": true,
}

// FsmConfigService 状态机管理业务逻辑
//
// 只持有自身的 store/cache，不持有其他模块的 store/service。
type FsmConfigService struct {
	store     *storemysql.FsmConfigStore
	dictStore *storemysql.FsmStateDictStore
	cache     *storeredis.FsmConfigCache
	pagCfg    *config.PaginationConfig
	fsmCfg    *config.FsmConfigConfig
}

// NewFsmConfigService 创建 FsmConfigService
func NewFsmConfigService(
	store *storemysql.FsmConfigStore,
	dictStore *storemysql.FsmStateDictStore,
	cache *storeredis.FsmConfigCache,
	pagCfg *config.PaginationConfig,
	fsmCfg *config.FsmConfigConfig,
) *FsmConfigService {
	return &FsmConfigService{
		store:     store,
		dictStore: dictStore,
		cache:     cache,
		pagCfg:    pagCfg,
		fsmCfg:    fsmCfg,
	}
}

// ---- 辅助方法 ----

func (s *FsmConfigService) getOrNotFound(ctx context.Context, id int64) (*model.FsmConfig, error) {
	fc, err := s.store.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get fsm_config %d: %w", id, err)
	}
	if fc == nil {
		return nil, errcode.Newf(errcode.ErrFsmConfigNotFound, "状态机 ID=%d 不存在", id)
	}
	return fc, nil
}

// buildConfigJSON 组装 config_json
func (s *FsmConfigService) buildConfigJSON(initialState string, states []model.FsmState, transitions []model.FsmTransition) (json.RawMessage, error) {
	configMap := map[string]interface{}{
		"initial_state": initialState,
		"states":        states,
		"transitions":   transitions,
	}
	data, err := json.Marshal(configMap)
	if err != nil {
		return nil, fmt.Errorf("marshal config_json: %w", err)
	}
	return data, nil
}

// ---- 配置完整性校验 ----

// validateConfig 校验 FSM 配置的完整性
func (s *FsmConfigService) validateConfig(initialState string, states []model.FsmState, transitions []model.FsmTransition) *errcode.Error {
	// R11: states 不能为空
	if len(states) == 0 {
		return errcode.New(errcode.ErrFsmConfigStatesEmpty)
	}

	// 状态数上限
	if s.fsmCfg.MaxStates > 0 && len(states) > s.fsmCfg.MaxStates {
		return errcode.Newf(errcode.ErrFsmConfigStatesEmpty, "状态数量不能超过 %d", s.fsmCfg.MaxStates)
	}

	// R12: 状态名非空且不重复
	stateSet := make(map[string]bool, len(states))
	for _, st := range states {
		if st.Name == "" {
			return errcode.Newf(errcode.ErrFsmConfigStateNameInvalid, "状态名不能为空")
		}
		if stateSet[st.Name] {
			return errcode.Newf(errcode.ErrFsmConfigStateNameInvalid, "状态名 '%s' 重复", st.Name)
		}
		stateSet[st.Name] = true
	}

	// R13: initial_state 必须是 states 中的某个
	if !stateSet[initialState] {
		return errcode.Newf(errcode.ErrFsmConfigInitialInvalid, "初始状态 '%s' 不在状态列表中", initialState)
	}

	// 转换数上限
	if s.fsmCfg.MaxTransitions > 0 && len(transitions) > s.fsmCfg.MaxTransitions {
		return errcode.Newf(errcode.ErrFsmConfigTransitionInvalid, "转换规则数量不能超过 %d", s.fsmCfg.MaxTransitions)
	}

	// R14: from/to 必须在 states 中 + R15: priority >= 0
	for i, tr := range transitions {
		if !stateSet[tr.From] {
			return errcode.Newf(errcode.ErrFsmConfigTransitionInvalid, "转换规则 #%d: from 状态 '%s' 不存在", i+1, tr.From)
		}
		if !stateSet[tr.To] {
			return errcode.Newf(errcode.ErrFsmConfigTransitionInvalid, "转换规则 #%d: to 状态 '%s' 不存在", i+1, tr.To)
		}
		if tr.Priority < 0 {
			return errcode.Newf(errcode.ErrFsmConfigTransitionInvalid, "转换规则 #%d: priority 不能为负数", i+1)
		}

		// R16: 条件树校验
		maxDepth := 10
		if s.fsmCfg.ConditionMaxDepth > 0 {
			maxDepth = s.fsmCfg.ConditionMaxDepth
		}
		if e := s.validateCondition(&tr.Condition, 0, maxDepth); e != nil {
			return errcode.Newf(errcode.ErrFsmConfigConditionInvalid, "转换规则 #%d: %s", i+1, e.Error())
		}
	}

	return nil
}

// validateCondition 递归校验条件树
func (s *FsmConfigService) validateCondition(cond *model.FsmCondition, depth, maxDepth int) *errcode.Error {
	// 空条件 = 无条件转换，始终 true
	if cond.IsEmpty() {
		return nil
	}

	// 深度限制
	if depth > maxDepth {
		return errcode.Newf(errcode.ErrFsmConfigConditionInvalid, "条件嵌套深度超过 %d 层", maxDepth)
	}

	isLeaf := cond.Key != ""
	hasAnd := len(cond.And) > 0
	hasOr := len(cond.Or) > 0

	// 叶/组合互斥
	if isLeaf && (hasAnd || hasOr) {
		return errcode.Newf(errcode.ErrFsmConfigConditionInvalid, "条件节点不能同时有 key 和 and/or")
	}
	if hasAnd && hasOr {
		return errcode.Newf(errcode.ErrFsmConfigConditionInvalid, "条件节点不能同时有 and 和 or")
	}

	// 叶节点校验
	if isLeaf {
		if !validConditionOps[cond.Op] {
			return errcode.Newf(errcode.ErrFsmConfigConditionInvalid, "不支持的操作符 '%s'", cond.Op)
		}
		// value 和 ref_key 不能同时非空
		hasValue := len(cond.Value) > 0 && string(cond.Value) != "null"
		hasRefKey := cond.RefKey != ""
		if hasValue && hasRefKey {
			return errcode.Newf(errcode.ErrFsmConfigConditionInvalid, "value 和 ref_key 不能同时设置")
		}
		// value 和 ref_key 不能同时为空（除非空条件，已在上面处理）
		if !hasValue && !hasRefKey {
			return errcode.Newf(errcode.ErrFsmConfigConditionInvalid, "value 和 ref_key 不能同时为空")
		}
		return nil
	}

	// 组合节点校验
	if hasAnd {
		for i := range cond.And {
			if e := s.validateCondition(&cond.And[i], depth+1, maxDepth); e != nil {
				return e
			}
		}
	}
	if hasOr {
		for i := range cond.Or {
			if e := s.validateCondition(&cond.Or[i], depth+1, maxDepth); e != nil {
				return e
			}
		}
	}

	return nil
}

// ---- CRUD ----

// List 分页列表
func (s *FsmConfigService) List(ctx context.Context, q *model.FsmConfigListQuery) (*model.ListData, error) {
	// 分页校正
	shared.NormalizePagination(&q.Page, &q.PageSize, s.pagCfg.DefaultPage, s.pagCfg.DefaultPageSize, s.pagCfg.MaxPageSize)

	// 查缓存（Redis 挂了跳过，降级直查 MySQL）
	if cached, hit, err := s.cache.GetList(ctx, q); err == nil && hit {
		slog.Debug("service.状态机列表.缓存命中")
		return cached.ToListData(), nil
	}

	// 查 MySQL
	items, total, err := s.store.List(ctx, q)
	if err != nil {
		return nil, err
	}

	// 从 config_json 抽展示字段
	listItems := make([]model.FsmConfigListItem, 0, len(items))
	initialNames := make([]string, 0, len(items))
	for _, fc := range items {
		item := model.FsmConfigListItem{
			ID:          fc.ID,
			Name:        fc.Name,
			DisplayName: fc.DisplayName,
			Enabled:     fc.Enabled,
			CreatedAt:   fc.CreatedAt,
		}
		// unmarshal config_json 抽展示值
		var cfg struct {
			InitialState string           `json:"initial_state"`
			States       []model.FsmState `json:"states"`
		}
		if err := json.Unmarshal(fc.ConfigJSON, &cfg); err == nil {
			item.InitialState = cfg.InitialState
			item.StateCount = len(cfg.States)
			if cfg.InitialState != "" {
				initialNames = append(initialNames, cfg.InitialState)
			}
		}
		listItems = append(listItems, item)
	}

	// 批量解析 initial_state 中文名
	labelMap, _ := s.dictStore.GetDisplayNamesByNames(ctx, initialNames)
	for i := range listItems {
		if label, ok := labelMap[listItems[i].InitialState]; ok {
			listItems[i].InitialStateLabel = label
		}
	}

	// 写缓存
	listData := &model.FsmConfigListData{
		Items:    listItems,
		Total:    total,
		Page:     q.Page,
		PageSize: q.PageSize,
	}
	s.cache.SetList(ctx, q, listData)

	return listData.ToListData(), nil
}

// Create 创建状态机配置
func (s *FsmConfigService) Create(ctx context.Context, req *model.CreateFsmConfigRequest) (int64, error) {
	slog.Debug("service.创建状态机", "name", req.Name)

	// name 唯一性（含软删除）
	exists, err := s.store.ExistsByName(ctx, req.Name)
	if err != nil {
		slog.Error("service.创建状态机-检查唯一性失败", "error", err, "name", req.Name)
		return 0, fmt.Errorf("check name exists: %w", err)
	}
	if exists {
		return 0, errcode.Newf(errcode.ErrFsmConfigNameExists, "状态机标识 '%s' 已存在", req.Name)
	}

	// 配置完整性校验
	if e := s.validateConfig(req.InitialState, req.States, req.Transitions); e != nil {
		return 0, e
	}

	// 拼 config_json
	configJSON, err := s.buildConfigJSON(req.InitialState, req.States, req.Transitions)
	if err != nil {
		return 0, err
	}

	// 写 MySQL
	id, err := s.store.Create(ctx, req, configJSON)
	if err != nil {
		if errors.Is(err, errcode.ErrDuplicate) {
			return 0, errcode.Newf(errcode.ErrFsmConfigNameExists, "状态机标识 '%s' 已存在", req.Name)
		}
		slog.Error("service.创建状态机失败", "error", err, "name", req.Name)
		return 0, fmt.Errorf("create fsm_config: %w", err)
	}

	// 清列表缓存
	s.cache.InvalidateList(ctx)

	slog.Info("service.创建状态机成功", "id", id, "name", req.Name)
	return id, nil
}

// GetByID 查详情（Cache-Aside + 分布式锁 + 空标记）
func (s *FsmConfigService) GetByID(ctx context.Context, id int64) (*model.FsmConfig, error) {
	// 1. 查缓存（Redis 挂了跳过，降级直查 MySQL）
	if cached, hit, err := s.cache.GetDetail(ctx, id); err == nil && hit {
		if cached == nil {
			return nil, errcode.New(errcode.ErrFsmConfigNotFound)
		}
		return cached, nil
	}

	// 2. 分布式锁防击穿
	lockTTL := 3 * time.Second
	if s.fsmCfg.CacheLockTTL > 0 {
		lockTTL = s.fsmCfg.CacheLockTTL
	}
	lockID, lockErr := s.cache.TryLock(ctx, id, lockTTL)
	if lockErr != nil {
		slog.Warn("service.获取锁失败，降级直查MySQL", "error", lockErr, "id", id)
	}
	if lockID != "" {
		defer s.cache.Unlock(ctx, id, lockID)
		// double-check
		if cached, hit, err := s.cache.GetDetail(ctx, id); err == nil && hit {
			if cached == nil {
				return nil, errcode.New(errcode.ErrFsmConfigNotFound)
			}
			return cached, nil
		}
	}

	// 3. 查 MySQL
	fc, err := s.store.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 4. 写缓存（含空标记）
	s.cache.SetDetail(ctx, id, fc)

	if fc == nil {
		return nil, errcode.New(errcode.ErrFsmConfigNotFound)
	}
	return fc, nil
}

// Update 编辑状态机配置
func (s *FsmConfigService) Update(ctx context.Context, req *model.UpdateFsmConfigRequest) error {
	slog.Debug("service.编辑状态机", "id", req.ID)

	fc, err := s.getOrNotFound(ctx, req.ID)
	if err != nil {
		return err
	}

	// 启用中禁止编辑
	if fc.Enabled {
		return errcode.New(errcode.ErrFsmConfigEditNotDisabled)
	}

	// 配置完整性校验
	if e := s.validateConfig(req.InitialState, req.States, req.Transitions); e != nil {
		return e
	}

	// 拼 config_json
	configJSON, err := s.buildConfigJSON(req.InitialState, req.States, req.Transitions)
	if err != nil {
		return err
	}

	// 乐观锁更新
	if err := s.store.Update(ctx, req, configJSON); err != nil {
		if errors.Is(err, errcode.ErrVersionConflict) {
			return errcode.New(errcode.ErrFsmConfigVersionConflict)
		}
		slog.Error("service.编辑状态机失败", "error", err, "id", req.ID)
		return fmt.Errorf("update fsm_config: %w", err)
	}

	// 清缓存
	s.cache.DelDetail(ctx, req.ID)
	s.cache.InvalidateList(ctx)

	slog.Info("service.编辑状态机成功", "id", req.ID)
	return nil
}

// Delete 软删除状态机配置
func (s *FsmConfigService) Delete(ctx context.Context, id int64) (*model.DeleteResult, error) {
	fc, err := s.getOrNotFound(ctx, id)
	if err != nil {
		return nil, err
	}

	// 启用中禁止删除
	if fc.Enabled {
		return nil, errcode.New(errcode.ErrFsmConfigDeleteNotDisabled)
	}

	// 本期 ref_count 不接入，直接删
	// TODO: NPC 管理上线后加 ref_count 检查 + FOR SHARE 防 TOCTOU

	if err := s.store.SoftDelete(ctx, id); err != nil {
		if errors.Is(err, errcode.ErrNotFound) {
			return nil, errcode.New(errcode.ErrFsmConfigNotFound)
		}
		slog.Error("service.删除状态机失败", "error", err, "id", id)
		return nil, fmt.Errorf("soft delete fsm_config: %w", err)
	}

	// 清缓存
	s.cache.DelDetail(ctx, id)
	s.cache.InvalidateList(ctx)

	slog.Info("service.删除状态机成功", "id", id, "name", fc.Name)
	return &model.DeleteResult{ID: id, Name: fc.Name, Label: fc.DisplayName}, nil
}

// ---- 事务版方法（handler 跨模块编排用）----

// CreateInTx 事务内创建状态机（校验 + store 写入，不清缓存）
func (s *FsmConfigService) CreateInTx(ctx context.Context, tx *sqlx.Tx, req *model.CreateFsmConfigRequest) (int64, json.RawMessage, error) {
	// name 唯一性
	exists, err := s.store.ExistsByName(ctx, req.Name)
	if err != nil {
		return 0, nil, fmt.Errorf("check name exists: %w", err)
	}
	if exists {
		return 0, nil, errcode.Newf(errcode.ErrFsmConfigNameExists, "状态机标识 '%s' 已存在", req.Name)
	}

	// 配置完整性校验
	if e := s.validateConfig(req.InitialState, req.States, req.Transitions); e != nil {
		return 0, nil, e
	}

	configJSON, err := s.buildConfigJSON(req.InitialState, req.States, req.Transitions)
	if err != nil {
		return 0, nil, err
	}

	id, err := s.store.CreateTx(ctx, tx, req, configJSON)
	if err != nil {
		if errors.Is(err, errcode.ErrDuplicate) {
			return 0, nil, errcode.Newf(errcode.ErrFsmConfigNameExists, "状态机标识 '%s' 已存在", req.Name)
		}
		slog.Error("service.创建状态机失败", "error", err, "name", req.Name)
		return 0, nil, fmt.Errorf("create fsm_config: %w", err)
	}

	slog.Info("service.创建状态机成功(tx)", "id", id, "name", req.Name)
	return id, configJSON, nil
}

// UpdateInTx 事务内编辑状态机（校验 + store 写入，不清缓存）
//
// 返回旧 config_json（handler 用于提取旧 BB Keys diff）。
func (s *FsmConfigService) UpdateInTx(ctx context.Context, tx *sqlx.Tx, req *model.UpdateFsmConfigRequest) (*model.FsmConfig, error) {
	fc, err := s.getOrNotFound(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	if fc.Enabled {
		return nil, errcode.New(errcode.ErrFsmConfigEditNotDisabled)
	}

	if e := s.validateConfig(req.InitialState, req.States, req.Transitions); e != nil {
		return nil, e
	}

	configJSON, err := s.buildConfigJSON(req.InitialState, req.States, req.Transitions)
	if err != nil {
		return nil, err
	}

	if err := s.store.UpdateTx(ctx, tx, req, configJSON); err != nil {
		if errors.Is(err, errcode.ErrVersionConflict) {
			return nil, errcode.New(errcode.ErrFsmConfigVersionConflict)
		}
		slog.Error("service.编辑状态机失败", "error", err, "id", req.ID)
		return nil, fmt.Errorf("update fsm_config: %w", err)
	}

	slog.Info("service.编辑状态机成功(tx)", "id", req.ID)
	return fc, nil // 返回旧数据，handler 用于 BB Key diff
}

// SoftDeleteInTx 事务内软删除状态机（前置校验 + store 写入，不清缓存）
func (s *FsmConfigService) SoftDeleteInTx(ctx context.Context, tx *sqlx.Tx, id int64) (*model.FsmConfig, error) {
	fc, err := s.getOrNotFound(ctx, id)
	if err != nil {
		return nil, err
	}

	if fc.Enabled {
		return nil, errcode.New(errcode.ErrFsmConfigDeleteNotDisabled)
	}

	// TODO: NPC 管理上线后加引用检查

	if err := s.store.SoftDeleteTx(ctx, tx, id); err != nil {
		if errors.Is(err, errcode.ErrNotFound) {
			return nil, errcode.New(errcode.ErrFsmConfigNotFound)
		}
		slog.Error("service.删除状态机失败", "error", err, "id", id)
		return nil, fmt.Errorf("soft delete fsm_config: %w", err)
	}

	slog.Info("service.删除状态机成功(tx)", "id", id)
	return fc, nil
}

// InvalidateDetail 清单条缓存（handler commit 后调用）
func (s *FsmConfigService) InvalidateDetail(ctx context.Context, id int64) {
	s.cache.DelDetail(ctx, id)
}

// InvalidateList 清列表缓存（handler commit 后调用）
func (s *FsmConfigService) InvalidateList(ctx context.Context) {
	s.cache.InvalidateList(ctx)
}

// ExtractBBKeys 从 transitions 中提取 BB Key name 集合
func ExtractBBKeys(transitions []model.FsmTransition) map[string]bool {
	keys := make(map[string]bool)
	for _, tr := range transitions {
		collectConditionKeys(&tr.Condition, keys)
	}
	return keys
}

// ExtractBBKeysFromConfigJSON 从 config_json 中提取 BB Key name 集合
func ExtractBBKeysFromConfigJSON(configJSON json.RawMessage) map[string]bool {
	var cfg struct {
		Transitions []model.FsmTransition `json:"transitions"`
	}
	if err := json.Unmarshal(configJSON, &cfg); err != nil {
		return make(map[string]bool)
	}
	return ExtractBBKeys(cfg.Transitions)
}

func collectConditionKeys(cond *model.FsmCondition, keys map[string]bool) {
	if cond.IsEmpty() {
		return
	}
	if cond.Key != "" {
		keys[cond.Key] = true
	}
	if cond.RefKey != "" {
		keys[cond.RefKey] = true
	}
	for i := range cond.And {
		collectConditionKeys(&cond.And[i], keys)
	}
	for i := range cond.Or {
		collectConditionKeys(&cond.Or[i], keys)
	}
}

// CheckName 唯一性校验
func (s *FsmConfigService) CheckName(ctx context.Context, name string) (*model.CheckNameResult, error) {
	exists, err := s.store.ExistsByName(ctx, name)
	if err != nil {
		slog.Error("service.校验状态机标识失败", "error", err, "name", name)
		return nil, fmt.Errorf("check name: %w", err)
	}
	if exists {
		return &model.CheckNameResult{Available: false, Message: "该状态机标识已存在"}, nil
	}
	return &model.CheckNameResult{Available: true, Message: "该标识可用"}, nil
}

// ToggleEnabled 切换启用/停用（由调用方指定目标状态，幂等安全）
func (s *FsmConfigService) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) error {
	if _, err := s.getOrNotFound(ctx, req.ID); err != nil {
		return err
	}

	if err := s.store.ToggleEnabled(ctx, req); err != nil {
		if errors.Is(err, errcode.ErrVersionConflict) {
			return errcode.New(errcode.ErrFsmConfigVersionConflict)
		}
		slog.Error("service.切换启用失败", "error", err, "id", req.ID)
		return fmt.Errorf("toggle enabled: %w", err)
	}

	// 清缓存
	s.cache.DelDetail(ctx, req.ID)
	s.cache.InvalidateList(ctx)

	slog.Info("service.切换启用成功", "id", req.ID, "enabled", req.Enabled)
	return nil
}

// ExportAll 导出所有已启用的状态机配置
func (s *FsmConfigService) ExportAll(ctx context.Context) ([]model.FsmConfigExportItem, error) {
	return s.store.ExportAll(ctx)
}
