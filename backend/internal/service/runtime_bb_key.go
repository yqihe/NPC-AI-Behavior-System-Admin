package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"regexp"

	"github.com/jmoiron/sqlx"
	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	shared "github.com/yqihe/npc-ai-admin/backend/internal/service/shared"
	storemysql "github.com/yqihe/npc-ai-admin/backend/internal/store/mysql"
	storeredis "github.com/yqihe/npc-ai-admin/backend/internal/store/redis"
	rcfg "github.com/yqihe/npc-ai-admin/backend/internal/store/redis/shared"
	"github.com/yqihe/npc-ai-admin/backend/internal/util"
)

// ──────────────────────────────────────────────────────
// 枚举白名单（对齐 Server internal/core/blackboard/keys.go）
// ──────────────────────────────────────────────────────

// validRuntimeBbKeyTypes 运行时 BB Key 合法类型 4 枚举。
// 锁定 design §0（§R11 type 规范化枚举）。DB 不走 CHECK 约束，靠本 map 拦截。
var validRuntimeBbKeyTypes = map[string]bool{
	"integer": true,
	"float":   true,
	"string":  true,
	"bool":    true,
}

// validRuntimeBbKeyGroups 运行时 BB Key 合法分组 11 枚举。
// 锁定 design §0（§R3 grouping 机制）。与 Server keys.go 分节注释逐字对齐。
var validRuntimeBbKeyGroups = map[string]bool{
	"threat":   true,
	"event":    true,
	"fsm":      true,
	"npc":      true,
	"action":   true,
	"need":     true,
	"emotion":  true,
	"memory":   true,
	"social":   true,
	"decision": true,
	"move":     true,
}

// runtimeBbKeyNameRE name 合法格式：小写字母开头，仅允许 [a-z0-9_]，长度 2~64。
// 对齐 Server keys.go NewKey 第一参数约定 + migration VARCHAR(64)。
var runtimeBbKeyNameRE = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)

// ──────────────────────────────────────────────────────
// RuntimeBbKeyService
// ──────────────────────────────────────────────────────

// RuntimeBbKeyService 运行时 BB Key 业务逻辑
//
// 分层约定（对齐 red-lines/go.md §禁止分层倒置）：
//   - 持 store / refStore / cache / fieldStore（仅读，跨模块 name 冲突检测用）
//   - 不持 FsmConfigService / BtTreeService（跨模块调用归 handler）
type RuntimeBbKeyService struct {
	store      *storemysql.RuntimeBbKeyStore
	refStore   *storemysql.RuntimeBbKeyRefStore
	cache      *storeredis.RuntimeBbKeyCache
	fieldStore *storemysql.FieldStore // 仅读：CheckName 时查字段表冲突
	pagCfg     *config.PaginationConfig
}

// NewRuntimeBbKeyService 创建 RuntimeBbKeyService
func NewRuntimeBbKeyService(
	store *storemysql.RuntimeBbKeyStore,
	refStore *storemysql.RuntimeBbKeyRefStore,
	cache *storeredis.RuntimeBbKeyCache,
	fieldStore *storemysql.FieldStore,
	pagCfg *config.PaginationConfig,
) *RuntimeBbKeyService {
	return &RuntimeBbKeyService{
		store:      store,
		refStore:   refStore,
		cache:      cache,
		fieldStore: fieldStore,
		pagCfg:     pagCfg,
	}
}

// ---- 业务校验辅助 ----

func (s *RuntimeBbKeyService) validateName(name string) *errcode.Error {
	if !runtimeBbKeyNameRE.MatchString(name) {
		return errcode.Newf(errcode.ErrRuntimeBBKeyNameInvalid, "运行时 BB Key 标识 '%s' 格式非法（仅允许小写字母开头 + 字母/数字/下划线，长度 2~64）", name)
	}
	return nil
}

func (s *RuntimeBbKeyService) validateType(typ string) *errcode.Error {
	if !validRuntimeBbKeyTypes[typ] {
		return errcode.Newf(errcode.ErrRuntimeBBKeyTypeInvalid, "运行时 BB Key 类型 '%s' 非法（仅允许 integer/float/string/bool）", typ)
	}
	return nil
}

func (s *RuntimeBbKeyService) validateGroupName(group string) *errcode.Error {
	if !validRuntimeBbKeyGroups[group] {
		return errcode.Newf(errcode.ErrRuntimeBBKeyGroupNameInvalid, "运行时 BB Key 分组 '%s' 非法", group)
	}
	return nil
}

// getOrNotFound 按 ID 查 + 判空
func (s *RuntimeBbKeyService) getOrNotFound(ctx context.Context, id int64) (*model.RuntimeBbKey, error) {
	k, err := s.store.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get runtime_bb_key %d: %w", id, err)
	}
	if k == nil {
		return nil, errcode.Newf(errcode.ErrRuntimeBBKeyNotFound, "运行时 BB Key ID=%d 不存在", id)
	}
	return k, nil
}

// ---- 业务方法 ----

// List 列表（Cache-Aside）
func (s *RuntimeBbKeyService) List(ctx context.Context, q *model.RuntimeBbKeyListQuery) (*model.ListData, error) {
	shared.NormalizePagination(&q.Page, &q.PageSize, s.pagCfg.DefaultPage, s.pagCfg.DefaultPageSize, s.pagCfg.MaxPageSize)

	if cached, hit, err := s.cache.GetList(ctx, q); err == nil && hit {
		return cached.ToListData(), nil
	}

	items, total, err := s.store.List(ctx, q)
	if err != nil {
		slog.Error("service.运行时BBKey列表查询失败", "error", err, "query", q)
		return nil, err
	}

	result := &model.RuntimeBbKeyListData{
		Items:    items,
		Total:    total,
		Page:     q.Page,
		PageSize: q.PageSize,
	}

	s.cache.SetList(ctx, q, result)
	return result.ToListData(), nil
}

// GetByID 详情（Cache-Aside + 分布式锁防击穿 + 实时填 has_refs/ref_count）
func (s *RuntimeBbKeyService) GetByID(ctx context.Context, id int64) (*model.RuntimeBbKey, error) {
	if cached, hit, err := s.cache.GetDetail(ctx, id); err == nil && hit {
		if cached == nil {
			return nil, errcode.Newf(errcode.ErrRuntimeBBKeyNotFound, "运行时 BB Key ID=%d 不存在", id)
		}
		s.fillRefStats(ctx, cached)
		return cached, nil
	}

	lockID, lockErr := s.cache.TryLock(ctx, id, rcfg.LockExpire)
	if lockErr != nil {
		slog.Warn("service.运行时BBKey锁获取失败，降级直查MySQL", "error", lockErr, "id", id)
	}
	if lockID != "" {
		defer s.cache.Unlock(ctx, id, lockID)
	}

	if lockID != "" {
		if cached, hit, err := s.cache.GetDetail(ctx, id); err == nil && hit {
			if cached == nil {
				return nil, errcode.Newf(errcode.ErrRuntimeBBKeyNotFound, "运行时 BB Key ID=%d 不存在", id)
			}
			s.fillRefStats(ctx, cached)
			return cached, nil
		}
	}

	k, err := s.store.GetByID(ctx, id)
	if err != nil {
		slog.Error("service.运行时BBKey详情查询失败", "error", err, "id", id)
		return nil, fmt.Errorf("get runtime_bb_key: %w", err)
	}

	s.cache.SetDetail(ctx, id, k)

	if k == nil {
		return nil, errcode.Newf(errcode.ErrRuntimeBBKeyNotFound, "运行时 BB Key ID=%d 不存在", id)
	}

	s.fillRefStats(ctx, k)
	return k, nil
}

// fillRefStats 实时填充 HasRefs / RefCount（不进缓存，引用随 FSM/BT 写操作变化）
func (s *RuntimeBbKeyService) fillRefStats(ctx context.Context, k *model.RuntimeBbKey) {
	refs, err := s.refStore.ListByKeyID(ctx, k.ID)
	if err != nil {
		slog.Warn("service.运行时BBKey引用查询失败，降级为无引用", "error", err, "id", k.ID)
		return
	}
	k.RefCount = len(refs)
	k.HasRefs = len(refs) > 0
}

// CheckName 校验 name 是否可用（跨表：fields + runtime_bb_keys 双向冲突检测）
//
// 返回：(conflict, source, err)
//   - conflict=false: name 可用
//   - conflict=true, source="field": 与某个 field.name 冲突
//   - conflict=true, source="runtime_bb_key": 与已有 runtime_bb_key.name 冲突（含软删）
//
// 在 service 层做单查询对冲突的 TOCTOU 容忍：运行时 key 创建频率低（运营行为），
// 并发两个相同 name 创建的窗口极窄，真冲突时 MySQL 唯一键兜底。详见 design §2.2。
func (s *RuntimeBbKeyService) CheckName(ctx context.Context, name string) (bool, string, error) {
	if err := s.validateName(name); err != nil {
		return false, "", err
	}

	field, err := s.fieldStore.GetByName(ctx, name)
	if err != nil {
		slog.Error("service.运行时BBKey-CheckName-字段冲突查询失败", "error", err, "name", name)
		return false, "", fmt.Errorf("check field conflict: %w", err)
	}
	if field != nil {
		return true, "field", nil
	}

	exists, err := s.store.ExistsByName(ctx, name)
	if err != nil {
		slog.Error("service.运行时BBKey-CheckName-自冲突查询失败", "error", err, "name", name)
		return false, "", fmt.Errorf("check self conflict: %w", err)
	}
	if exists {
		return true, "runtime_bb_key", nil
	}
	return false, "", nil
}

// CheckByNames 批量校验一组 name 是否全部是"已启用的运行时 key"
//
// 返回 notOK：不存在或已停用的 name 列表。空 names → nil, nil。
// 用途：FSM/BT Create/Update 时 handler 调用，与 fieldService 的类似接口并行运行。
// 字段 name 与运行时 key name 不会同时匹配（name 全局唯一），调用方按"两路都 notOK"判非法。
func (s *RuntimeBbKeyService) CheckByNames(ctx context.Context, names []string) ([]string, error) {
	if len(names) == 0 {
		return nil, nil
	}
	enabledSet, err := s.store.GetEnabledByNames(ctx, names)
	if err != nil {
		return nil, fmt.Errorf("get enabled runtime_bb_keys by names: %w", err)
	}
	notOK := make([]string, 0)
	for _, name := range names {
		if !enabledSet[name] {
			notOK = append(notOK, name)
		}
	}
	return notOK, nil
}

// Create 创建运行时 BB Key
func (s *RuntimeBbKeyService) Create(ctx context.Context, req *model.CreateRuntimeBbKeyRequest) (int64, error) {
	if err := s.validateName(req.Name); err != nil {
		return 0, err
	}
	if err := s.validateType(req.Type); err != nil {
		return 0, err
	}
	if err := s.validateGroupName(req.GroupName); err != nil {
		return 0, err
	}

	// 跨表 name 冲突检测（field + 自身）
	field, err := s.fieldStore.GetByName(ctx, req.Name)
	if err != nil {
		return 0, fmt.Errorf("check field conflict: %w", err)
	}
	if field != nil {
		return 0, errcode.Newf(errcode.ErrRuntimeBBKeyNameConflictWithField, "运行时 BB Key 标识 '%s' 与字段标识冲突", req.Name)
	}

	exists, err := s.store.ExistsByName(ctx, req.Name)
	if err != nil {
		return 0, fmt.Errorf("check name exists: %w", err)
	}
	if exists {
		return 0, errcode.Newf(errcode.ErrRuntimeBBKeyNameExists, "运行时 BB Key 标识 '%s' 已存在", req.Name)
	}

	id, err := s.store.Create(ctx, req)
	if err != nil {
		if errors.Is(err, errcode.ErrDuplicate) {
			return 0, errcode.Newf(errcode.ErrRuntimeBBKeyNameExists, "运行时 BB Key 标识 '%s' 已存在", req.Name)
		}
		slog.Error("service.创建运行时BBKey失败", "error", err, "name", req.Name)
		return 0, fmt.Errorf("create runtime_bb_key: %w", err)
	}

	s.cache.InvalidateList(ctx)
	slog.Info("service.创建运行时BBKey成功", "name", req.Name, "id", id)
	return id, nil
}

// Update 编辑（仅未启用时可编辑；name 不可变）
func (s *RuntimeBbKeyService) Update(ctx context.Context, req *model.UpdateRuntimeBbKeyRequest) error {
	if err := s.validateType(req.Type); err != nil {
		return err
	}
	if err := s.validateGroupName(req.GroupName); err != nil {
		return err
	}

	old, err := s.getOrNotFound(ctx, req.ID)
	if err != nil {
		return err
	}

	// 硬约束：必须未启用才能编辑（对齐 field 模块）
	if old.Enabled {
		return errcode.New(errcode.ErrRuntimeBBKeyEditNotDisabled)
	}

	if err := s.store.Update(ctx, req); err != nil {
		if errors.Is(err, errcode.ErrVersionConflict) {
			return errcode.New(errcode.ErrRuntimeBBKeyVersionConflict)
		}
		slog.Error("service.编辑运行时BBKey失败", "error", err, "id", req.ID)
		return fmt.Errorf("update runtime_bb_key: %w", err)
	}

	s.cache.DelDetail(ctx, req.ID)
	s.cache.InvalidateList(ctx)

	slog.Info("service.编辑运行时BBKey成功", "id", req.ID)
	return nil
}

// Delete 软删除（仅未启用 + 无引用时可删）
func (s *RuntimeBbKeyService) Delete(ctx context.Context, id int64) (*model.DeleteResult, error) {
	k, err := s.getOrNotFound(ctx, id)
	if err != nil {
		return nil, err
	}

	if k.Enabled {
		return nil, errcode.New(errcode.ErrRuntimeBBKeyDeleteNotDisabled)
	}

	// 事务内：FOR SHARE 检查引用 + 软删（TOCTOU 防护）
	tx, err := s.store.DB().BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			slog.Warn("service.删除运行时BBKey事务回滚失败", "error", rbErr)
		}
	}()

	hasRefs, err := s.refStore.HasRefsTx(ctx, tx, id)
	if err != nil {
		return nil, fmt.Errorf("check refs in tx: %w", err)
	}
	if hasRefs {
		return nil, errcode.New(errcode.ErrRuntimeBBKeyHasRefs)
	}

	if err := s.store.SoftDeleteTx(ctx, tx, id); err != nil {
		if errors.Is(err, errcode.ErrNotFound) {
			return nil, errcode.Newf(errcode.ErrRuntimeBBKeyNotFound, "运行时 BB Key ID=%d 不存在", id)
		}
		return nil, fmt.Errorf("soft delete: %w", err)
	}

	// 先清缓存再 Commit（对齐 field 模块 cache red-lines §写后清缓存顺序）
	s.cache.DelDetail(ctx, id)
	s.cache.InvalidateList(ctx)

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	slog.Info("service.删除运行时BBKey成功", "id", id, "name", k.Name)
	return &model.DeleteResult{ID: id, Name: k.Name, Label: k.Label}, nil
}

// ToggleEnabled 切换启用/停用
func (s *RuntimeBbKeyService) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) error {
	if _, err := s.getOrNotFound(ctx, req.ID); err != nil {
		return err
	}

	if err := s.store.ToggleEnabled(ctx, req); err != nil {
		if errors.Is(err, errcode.ErrVersionConflict) {
			return errcode.New(errcode.ErrRuntimeBBKeyVersionConflict)
		}
		slog.Error("service.切换运行时BBKey启用失败", "error", err, "id", req.ID)
		return fmt.Errorf("toggle enabled: %w", err)
	}

	s.cache.DelDetail(ctx, req.ID)
	s.cache.InvalidateList(ctx)

	slog.Info("service.切换运行时BBKey启用成功", "id", req.ID, "enabled", req.Enabled)
	return nil
}

// GetReferences 引用详情（/:id/references 端点）
//
// 返回 FSM 引用 ID 列表 + BT 引用 ID 列表；handler 跨模块补齐 label。
func (s *RuntimeBbKeyService) GetReferences(ctx context.Context, id int64) (*model.RuntimeBbKeyReferenceDetail, error) {
	k, err := s.getOrNotFound(ctx, id)
	if err != nil {
		return nil, err
	}

	refs, err := s.refStore.ListByKeyID(ctx, id)
	if err != nil {
		slog.Error("service.运行时BBKey引用详情-查引用失败", "error", err, "id", id)
		return nil, fmt.Errorf("list refs: %w", err)
	}

	result := &model.RuntimeBbKeyReferenceDetail{
		KeyID:    id,
		KeyName:  k.Name,
		KeyLabel: k.Label,
		Fsms:     make([]model.ReferenceItem, 0),
		Bts:      make([]model.ReferenceItem, 0),
	}
	for _, r := range refs {
		item := model.ReferenceItem{RefType: r.RefType, RefID: r.RefID}
		switch r.RefType {
		case util.RefTypeFsm:
			result.Fsms = append(result.Fsms, item)
		case util.RefTypeBt:
			result.Bts = append(result.Bts, item)
		}
	}
	return result, nil
}

// ──────────────────────────────────────────────────────
// 引用同步（FSM/BT Create/Update 时 handler 在事务内调用）
// ──────────────────────────────────────────────────────
//
// 算法与 FieldService.SyncFsmBBKeyRefs 对称并行运行：
//   - field key name → field_refs 表（field 服务负责）
//   - runtime key name → runtime_bb_key_refs 表（本服务负责）
//   - 同一份 newKeys 集合 送给两个 service，各筛各的（name 不在自己管辖表时 skip）
//   - 未匹配任何一方的 name → 由 FSM/BT validator 前置 400 拦截

// SyncFsmRefs 同步 FSM 条件树中对运行时 key 的引用（事务内）
//
// oldKeys/newKeys: 条件树提取的 BB Key name 集合。
// 内部解析 name → runtime_key_id，只追踪 runtime_bb_keys 表的 Key（field 表 Key 跳过）。
// 返回 affected runtime_key IDs（handler 用于清缓存）。
func (s *RuntimeBbKeyService) SyncFsmRefs(
	ctx context.Context, tx *sqlx.Tx, fsmID int64,
	oldKeys, newKeys map[string]bool,
) ([]int64, error) {
	return s.syncRefs(ctx, tx, util.RefTypeFsm, fsmID, oldKeys, newKeys)
}

// SyncBtRefs 同步行为树节点中对运行时 key 的引用（事务内）
func (s *RuntimeBbKeyService) SyncBtRefs(
	ctx context.Context, tx *sqlx.Tx, btTreeID int64,
	oldKeys, newKeys map[string]bool,
) ([]int64, error) {
	return s.syncRefs(ctx, tx, util.RefTypeBt, btTreeID, oldKeys, newKeys)
}

// syncRefs FSM/BT 引用同步的共用算法（对齐 FieldService 两路对称但提取共同实现）
func (s *RuntimeBbKeyService) syncRefs(
	ctx context.Context, tx *sqlx.Tx, refType string, refID int64,
	oldKeys, newKeys map[string]bool,
) ([]int64, error) {
	toAdd := make([]string, 0)
	toRemove := make([]string, 0)
	for k := range newKeys {
		if !oldKeys[k] {
			toAdd = append(toAdd, k)
		}
	}
	for k := range oldKeys {
		if !newKeys[k] {
			toRemove = append(toRemove, k)
		}
	}

	if len(toAdd) == 0 && len(toRemove) == 0 {
		return nil, nil
	}

	// 批量解析 name → runtime_key_id（合并 toAdd+toRemove 查一次）
	allNames := make([]string, 0, len(toAdd)+len(toRemove))
	allNames = append(allNames, toAdd...)
	allNames = append(allNames, toRemove...)
	keys, err := s.store.GetByNames(ctx, allNames)
	if err != nil {
		return nil, fmt.Errorf("get runtime_bb_keys by names: %w", err)
	}
	nameToID := make(map[string]int64, len(keys))
	for _, k := range keys {
		nameToID[k.Name] = k.ID
	}

	// 收集 add/remove 的 runtime_key_id（跳过字段 key，它们不在本表）
	addIDs := make([]int64, 0, len(toAdd))
	for _, name := range toAdd {
		if id, ok := nameToID[name]; ok {
			addIDs = append(addIDs, id)
		}
	}
	removeIDs := make([]int64, 0, len(toRemove))
	for _, name := range toRemove {
		if id, ok := nameToID[name]; ok {
			removeIDs = append(removeIDs, id)
		}
	}

	if err := s.refStore.AddBatch(ctx, tx, refType, refID, addIDs); err != nil {
		return nil, fmt.Errorf("add runtime_bb_key refs (%s %d): %w", refType, refID, err)
	}
	if err := s.refStore.DeleteByRefAndKeyIDs(ctx, tx, refType, refID, removeIDs); err != nil {
		return nil, fmt.Errorf("delete runtime_bb_key refs (%s %d): %w", refType, refID, err)
	}

	affected := make([]int64, 0, len(addIDs)+len(removeIDs))
	affected = append(affected, addIDs...)
	affected = append(affected, removeIDs...)
	return affected, nil
}

// DeleteRefsByFsmID FSM 删除时清理该 FSM 所有 runtime_bb_key 引用（事务内）
//
// 返回被影响的 runtime_key IDs（handler 用于清缓存）。
func (s *RuntimeBbKeyService) DeleteRefsByFsmID(ctx context.Context, tx *sqlx.Tx, fsmID int64) ([]int64, error) {
	return s.refStore.DeleteByRef(ctx, tx, util.RefTypeFsm, fsmID)
}

// DeleteRefsByBtID BT 删除时清理该行为树所有 runtime_bb_key 引用（事务内）
func (s *RuntimeBbKeyService) DeleteRefsByBtID(ctx context.Context, tx *sqlx.Tx, btTreeID int64) ([]int64, error) {
	return s.refStore.DeleteByRef(ctx, tx, util.RefTypeBt, btTreeID)
}
