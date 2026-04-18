package service

import (
	shared "github.com/yqihe/npc-ai-admin/backend/internal/service/shared"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	storemysql "github.com/yqihe/npc-ai-admin/backend/internal/store/mysql"
	storeredis "github.com/yqihe/npc-ai-admin/backend/internal/store/redis"
	rcfg "github.com/yqihe/npc-ai-admin/backend/internal/store/redis/shared"
)

// NpcService NPC 管理业务逻辑
//
// 严格遵守"分层职责"硬规则：只持有自身的 store/cache，
// 不持有 templateService / fieldService / fsmService / btService。
// 跨模块校验（模板存在性/字段校验/FSM&BT 可用性）由 handler 层负责。
type NpcService struct {
	store  *storemysql.NpcStore
	cache  *storeredis.NPCCache
	pagCfg *config.PaginationConfig
}

// NewNpcService 创建 NpcService
func NewNpcService(
	store *storemysql.NpcStore,
	cache *storeredis.NPCCache,
	pagCfg *config.PaginationConfig,
) *NpcService {
	return &NpcService{
		store:  store,
		cache:  cache,
		pagCfg: pagCfg,
	}
}

// ──────────────────────────────────────────────
// 内部辅助
// ──────────────────────────────────────────────

// getOrNotFound 按 ID 查 NPC，nil → ErrNPCNotFound(45003)
func (s *NpcService) getOrNotFound(ctx context.Context, id int64) (*model.NPC, error) {
	n, err := s.store.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get npc %d: %w", id, err)
	}
	if n == nil {
		return nil, errcode.Newf(errcode.ErrNPCNotFound, "NPC ID=%d 不存在", id)
	}
	return n, nil
}

// ──────────────────────────────────────────────
// CRUD
// ──────────────────────────────────────────────

// List 分页列表（Cache-Aside），返回类型安全的 *NPCListData
//
// 返回 *NPCListData 而非通用 *ListData，使 handler 层可对 Items 切片逐条补全 TemplateLabel。
func (s *NpcService) List(ctx context.Context, q *model.NPCListQuery) (*model.NPCListData, error) {
	shared.NormalizePagination(&q.Page, &q.PageSize, s.pagCfg.DefaultPage, s.pagCfg.DefaultPageSize, s.pagCfg.MaxPageSize)

	// 查缓存
	if cached, hit, err := s.cache.GetList(ctx, q); err == nil && hit {
		slog.Debug("service.NPC列表.缓存命中")
		return cached, nil
	}

	// 查 MySQL
	items, total, err := s.store.List(ctx, q)
	if err != nil {
		return nil, err
	}

	// 写缓存
	listData := &model.NPCListData{
		Items:    items,
		Total:    total,
		Page:     q.Page,
		PageSize: q.PageSize,
	}
	s.cache.SetList(ctx, q, listData)

	return listData, nil
}

// GetByID 查详情（Cache-Aside + 分布式锁 + 空标记）
func (s *NpcService) GetByID(ctx context.Context, id int64) (*model.NPC, error) {
	// 1. 查缓存
	if cached, hit, err := s.cache.GetDetail(ctx, id); err == nil && hit {
		if cached == nil {
			return nil, errcode.Newf(errcode.ErrNPCNotFound, "NPC ID=%d 不存在", id)
		}
		return cached, nil
	}

	// 2. 分布式锁防击穿
	lockID, lockErr := s.cache.TryLock(ctx, id, rcfg.LockExpire)
	if lockErr != nil {
		slog.Warn("service.获取NPC锁失败，降级直查MySQL", "error", lockErr, "id", id)
	}
	if lockID != "" {
		defer s.cache.Unlock(ctx, id, lockID)
		// double-check
		if cached, hit, err := s.cache.GetDetail(ctx, id); err == nil && hit {
			if cached == nil {
				return nil, errcode.Newf(errcode.ErrNPCNotFound, "NPC ID=%d 不存在", id)
			}
			return cached, nil
		}
	}

	// 3. 查 MySQL
	n, err := s.store.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get npc: %w", err)
	}

	// 4. 写缓存（含空标记）
	s.cache.SetDetail(ctx, id, n)

	if n == nil {
		return nil, errcode.Newf(errcode.ErrNPCNotFound, "NPC ID=%d 不存在", id)
	}
	return n, nil
}

// Create 创建 NPC
//
// handler 层在调用前需填入 req.TemplateName 和 req.FieldsSnapshot。
// 事务内同时写 npcs + npc_bt_refs，保证引用关系表与 bt_refs 列一致。
func (s *NpcService) Create(ctx context.Context, req *model.CreateNPCRequest) (int64, error) {
	slog.Debug("service.创建NPC", "name", req.Name)

	// name 唯一性（含软删除）
	exists, err := s.store.ExistsByName(ctx, req.Name)
	if err != nil {
		slog.Error("service.创建NPC-检查唯一性失败", "error", err, "name", req.Name)
		return 0, fmt.Errorf("check name exists: %w", err)
	}
	if exists {
		return 0, errcode.Newf(errcode.ErrNPCNameExists, "NPC 标识 '%s' 已存在", req.Name)
	}

	// 序列化字段快照
	fieldsJSON, err := json.Marshal(req.FieldsSnapshot)
	if err != nil {
		return 0, fmt.Errorf("marshal fields snapshot: %w", err)
	}

	// 序列化 bt_refs（nil map → "{}"）
	btRefsJSON, err := json.Marshal(req.BtRefs)
	if err != nil {
		return 0, fmt.Errorf("marshal bt_refs: %w", err)
	}

	// 事务：写 npcs + npc_bt_refs
	tx, err := s.store.DB().BeginTxx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			slog.Warn("service.创建NPC事务回滚失败", "error", rbErr)
		}
	}()

	id, err := s.store.CreateInTx(ctx, tx, req, fieldsJSON, btRefsJSON)
	if err != nil {
		if errors.Is(err, errcode.ErrDuplicate) {
			return 0, errcode.Newf(errcode.ErrNPCNameExists, "NPC 标识 '%s' 已存在", req.Name)
		}
		slog.Error("service.创建NPC失败", "error", err, "name", req.Name)
		return 0, fmt.Errorf("create npc: %w", err)
	}

	if err := s.store.InsertBtRefsInTx(ctx, tx, id, req.BtRefs); err != nil {
		slog.Error("service.创建NPC-写入bt_refs引用失败", "error", err, "id", id)
		return 0, fmt.Errorf("insert bt_refs: %w", err)
	}

	// 先清缓存再 Commit
	s.cache.InvalidateList(ctx)

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}

	slog.Info("service.创建NPC成功", "id", id, "name", req.Name)
	return id, nil
}

// Update 编辑 NPC（乐观锁）
//
// handler 层在调用前需填入 req.FieldsSnapshot（重新组装的快照）。
// 事务内同时更新 npcs + 替换 npc_bt_refs。
func (s *NpcService) Update(ctx context.Context, req *model.UpdateNPCRequest) error {
	slog.Debug("service.编辑NPC", "id", req.ID)

	if _, err := s.getOrNotFound(ctx, req.ID); err != nil {
		return err
	}

	// 序列化字段快照
	fieldsJSON, err := json.Marshal(req.FieldsSnapshot)
	if err != nil {
		return fmt.Errorf("marshal fields snapshot: %w", err)
	}

	// 序列化 bt_refs
	btRefsJSON, err := json.Marshal(req.BtRefs)
	if err != nil {
		return fmt.Errorf("marshal bt_refs: %w", err)
	}

	// 事务：更新 npcs + 替换 npc_bt_refs
	tx, err := s.store.DB().BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			slog.Warn("service.编辑NPC事务回滚失败", "error", rbErr)
		}
	}()

	if err := s.store.UpdateInTx(ctx, tx, req, fieldsJSON, btRefsJSON); err != nil {
		if errors.Is(err, errcode.ErrVersionConflict) {
			return errcode.New(errcode.ErrNPCVersionConflict)
		}
		slog.Error("service.编辑NPC失败", "error", err, "id", req.ID)
		return fmt.Errorf("update npc: %w", err)
	}

	if err := s.store.DeleteBtRefsInTx(ctx, tx, req.ID); err != nil {
		slog.Error("service.编辑NPC-清理bt_refs引用失败", "error", err, "id", req.ID)
		return fmt.Errorf("delete bt_refs: %w", err)
	}

	if err := s.store.InsertBtRefsInTx(ctx, tx, req.ID, req.BtRefs); err != nil {
		slog.Error("service.编辑NPC-写入bt_refs引用失败", "error", err, "id", req.ID)
		return fmt.Errorf("insert bt_refs: %w", err)
	}

	// 先清缓存再 Commit
	s.cache.DelDetail(ctx, req.ID)
	s.cache.InvalidateList(ctx)

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	slog.Info("service.编辑NPC成功", "id", req.ID)
	return nil
}

// SoftDelete 软删除 NPC（启用中禁止删除）
//
// 事务内同时软删 npcs + 清理 npc_bt_refs。
func (s *NpcService) SoftDelete(ctx context.Context, id int64) (*model.DeleteResult, error) {
	slog.Debug("service.删除NPC", "id", id)

	n, err := s.getOrNotFound(ctx, id)
	if err != nil {
		return nil, err
	}

	// 启用中禁止删除（handler 层已先行拦截，此处为防御性校验）
	if n.Enabled {
		return nil, errcode.New(errcode.ErrNPCDeleteNotDisabled)
	}

	// 事务：软删 npcs + 清理 npc_bt_refs
	tx, err := s.store.DB().BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			slog.Warn("service.删除NPC事务回滚失败", "error", rbErr)
		}
	}()

	if err := s.store.SoftDeleteInTx(ctx, tx, id); err != nil {
		if errors.Is(err, errcode.ErrNotFound) {
			return nil, errcode.Newf(errcode.ErrNPCNotFound, "NPC ID=%d 不存在", id)
		}
		slog.Error("service.删除NPC失败", "error", err, "id", id)
		return nil, fmt.Errorf("soft delete npc: %w", err)
	}

	if err := s.store.DeleteBtRefsInTx(ctx, tx, id); err != nil {
		slog.Error("service.删除NPC-清理bt_refs引用失败", "error", err, "id", id)
		return nil, fmt.Errorf("delete bt_refs: %w", err)
	}

	// 先清缓存再 Commit
	s.cache.DelDetail(ctx, id)
	s.cache.InvalidateList(ctx)

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	slog.Info("service.删除NPC成功", "id", id, "name", n.Name)
	return &model.DeleteResult{ID: n.ID, Name: n.Name, Label: n.Label}, nil
}

// ToggleEnabled 切换启用/停用（乐观锁）
func (s *NpcService) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) error {
	slog.Debug("service.切换NPC启用", "id", req.ID)

	if _, err := s.getOrNotFound(ctx, req.ID); err != nil {
		return err
	}

	if err := s.store.ToggleEnabled(ctx, req); err != nil {
		if errors.Is(err, errcode.ErrVersionConflict) {
			return errcode.New(errcode.ErrNPCVersionConflict)
		}
		slog.Error("service.切换NPC启用失败", "error", err, "id", req.ID)
		return fmt.Errorf("toggle npc enabled: %w", err)
	}

	// 清缓存
	s.cache.DelDetail(ctx, req.ID)
	s.cache.InvalidateList(ctx)

	slog.Info("service.切换NPC启用成功", "id", req.ID, "enabled", req.Enabled)
	return nil
}

// CheckName name 唯一性校验（含软删除记录）
func (s *NpcService) CheckName(ctx context.Context, name string) (*model.CheckNameResult, error) {
	exists, err := s.store.ExistsByName(ctx, name)
	if err != nil {
		slog.Error("service.校验NPC标识失败", "error", err, "name", name)
		return nil, fmt.Errorf("check name: %w", err)
	}
	if exists {
		return &model.CheckNameResult{Available: false, Message: "该 NPC 标识已存在"}, nil
	}
	return &model.CheckNameResult{Available: true, Message: "该标识可用"}, nil
}

// ──────────────────────────────────────────────
// 业务校验 + 数据组装（handler 拿到跨模块数据后调用，纯业务逻辑不查其他模块）
// ──────────────────────────────────────────────

// BuildFieldSnapshot 按模板字段顺序校验字段值并组装快照
//
// templateEntries: 模板字段顺序（field_id, required）
// fieldLites:      字段元数据（handler 从 fieldService 批量拿到）
// fieldValues:     前端传入的字段值列表
//
// 纯业务逻辑：构建 map → 校验必填/类型/约束 → 返回快照。不访问任何外部服务。
func (s *NpcService) BuildFieldSnapshot(
	templateEntries []model.TemplateFieldEntry,
	fieldLites []model.FieldLite,
	fieldValues []model.NPCFieldValue,
) ([]model.NPCFieldEntry, *errcode.Error) {
	// 构建 fieldMap
	fieldMap := make(map[int64]model.FieldLite, len(fieldLites))
	for _, fl := range fieldLites {
		if fl.ID > 0 {
			fieldMap[fl.ID] = fl
		}
	}

	// 构建 valueMap
	valueMap := make(map[int64]json.RawMessage, len(fieldValues))
	for _, fv := range fieldValues {
		valueMap[fv.FieldID] = fv.Value
	}

	snapshot := make([]model.NPCFieldEntry, 0, len(templateEntries))
	for _, entry := range templateEntries {
		field, ok := fieldMap[entry.FieldID]
		if !ok {
			slog.Warn("service.NPC字段校验-字段元数据缺失", "field_id", entry.FieldID)
			snapshot = append(snapshot, model.NPCFieldEntry{
				FieldID:  entry.FieldID,
				Required: entry.Required,
				Value:    json.RawMessage("null"),
			})
			continue
		}

		value := valueMap[entry.FieldID]

		if shared.IsJSONNull(value) {
			if entry.Required {
				return nil, errcode.Newf(errcode.ErrNPCFieldRequired,
					"字段 '%s' 为必填项，值不能为空", field.Name)
			}
			value = json.RawMessage("null")
		} else {
			var props model.FieldProperties
			if len(field.Properties) > 0 {
				_ = json.Unmarshal(field.Properties, &props)
			}
			if verr := shared.ValidateValue(field.Type, props.Constraints, value); verr != nil {
				return nil, errcode.Newf(errcode.ErrNPCFieldValueInvalid,
					"字段 '%s' 值不符合约束: %s", field.Name, verr.Message)
			}
		}

		snapshot = append(snapshot, model.NPCFieldEntry{
			FieldID:  entry.FieldID,
			Name:     field.Name,
			Required: entry.Required,
			Value:    value,
		})
	}
	return snapshot, nil
}

// ValidateBehaviorRefs 校验 bt_refs 的状态键在 FSM states 中存在
//
// 纯业务规则校验，不访问外部服务。
// fsmStates: handler 从 FsmConfigService 获取的状态名集合。
// btRefs:    前端传入的 {state_name → bt_tree_name} 映射。
// fsmRef:    用于错误提示。
func (s *NpcService) ValidateBehaviorRefs(fsmRef string, btRefs map[string]string, fsmStates map[string]bool) *errcode.Error {
	if len(btRefs) > 0 && fsmRef == "" {
		return errcode.New(errcode.ErrNPCBtWithoutFsm)
	}
	for stateName, btName := range btRefs {
		if btName == "" {
			continue
		}
		if !fsmStates[stateName] {
			return errcode.Newf(errcode.ErrNPCBtStateInvalid,
				"行为树绑定的状态名 '%s' 不在状态机 '%s' 的状态列表中", stateName, fsmRef)
		}
	}
	return nil
}

// BuildDetail 组装 NPC 详情响应
//
// 解析 JSON 快照 + 拼装字段元数据 + bt_refs 反序列化。
// fieldLites/templateLabel 由 handler 从对应 service 获取后传入。
func (s *NpcService) BuildDetail(npc *model.NPC, fieldLites []model.FieldLite, templateLabel string) (*model.NPCDetail, error) {
	// 解析字段快照
	var fieldEntries []model.NPCFieldEntry
	if err := json.Unmarshal(npc.Fields, &fieldEntries); err != nil {
		return nil, fmt.Errorf("parse npc fields: %w", err)
	}

	// 构建 fieldLiteMap
	fieldLiteMap := make(map[int64]model.FieldLite, len(fieldLites))
	for _, fl := range fieldLites {
		if fl.ID > 0 {
			fieldLiteMap[fl.ID] = fl
		}
	}

	// 拼装 []NPCDetailField
	detailFields := make([]model.NPCDetailField, 0, len(fieldEntries))
	for _, entry := range fieldEntries {
		fl, ok := fieldLiteMap[entry.FieldID]
		if !ok {
			slog.Warn("service.NPC详情-字段元数据缺失", "field_id", entry.FieldID)
			continue
		}
		detailFields = append(detailFields, model.NPCDetailField{
			FieldID:       entry.FieldID,
			Name:          entry.Name,
			Label:         fl.Label,
			Type:          fl.Type,
			Category:      fl.Category,
			CategoryLabel: fl.CategoryLabel,
			Enabled:       fl.Enabled,
			Required:      entry.Required,
			Value:         entry.Value,
		})
	}

	// 解析 bt_refs
	var btRefs map[string]string
	if len(npc.BtRefs) > 0 {
		_ = json.Unmarshal(npc.BtRefs, &btRefs)
	}
	if btRefs == nil {
		btRefs = make(map[string]string)
	}

	return &model.NPCDetail{
		ID:            npc.ID,
		Name:          npc.Name,
		Label:         npc.Label,
		Description:   npc.Description,
		TemplateID:    npc.TemplateID,
		TemplateName:  npc.TemplateName,
		TemplateLabel: templateLabel,
		Enabled:       npc.Enabled,
		Version:       npc.Version,
		Fields:        detailFields,
		FsmRef:        npc.FsmRef,
		BtRefs:        btRefs,
	}, nil
}

// ExtractFieldIDsFromNPC 从 NPC fields JSON 中提取 field_id 列表（handler 跨模块拿 FieldLite 用）
func (s *NpcService) ExtractFieldIDsFromNPC(fieldsJSON json.RawMessage) ([]int64, error) {
	var entries []model.NPCFieldEntry
	if err := json.Unmarshal(fieldsJSON, &entries); err != nil {
		return nil, fmt.Errorf("parse npc fields: %w", err)
	}
	ids := make([]int64, len(entries))
	for i, e := range entries {
		ids[i] = e.FieldID
	}
	return ids, nil
}

// ParseNPCFieldEntries 解析 NPC fields JSON 为快照条目 + 模板字段条目（Update 重建快照用）
func (s *NpcService) ParseNPCFieldEntries(fieldsJSON json.RawMessage) ([]model.NPCFieldEntry, []model.TemplateFieldEntry, error) {
	var entries []model.NPCFieldEntry
	if err := json.Unmarshal(fieldsJSON, &entries); err != nil {
		return nil, nil, fmt.Errorf("parse npc fields: %w", err)
	}
	templateEntries := make([]model.TemplateFieldEntry, len(entries))
	for i, e := range entries {
		templateEntries[i] = model.TemplateFieldEntry{
			FieldID:  e.FieldID,
			Required: e.Required,
		}
	}
	return entries, templateEntries, nil
}

// FillSnapshotNames 用旧快照的 Name 回填新快照（Update 时字段 name 从旧快照保留）
func (s *NpcService) FillSnapshotNames(snapshot []model.NPCFieldEntry, oldEntries []model.NPCFieldEntry) {
	oldMap := make(map[int64]string, len(oldEntries))
	for _, e := range oldEntries {
		oldMap[e.FieldID] = e.Name
	}
	for i := range snapshot {
		if name, ok := oldMap[snapshot[i].FieldID]; ok {
			snapshot[i].Name = name
		}
	}
}

// ──────────────────────────────────────────────
// 跨模块对外接口（供其他 handler 调用，不暴露 store 细节）
// ──────────────────────────────────────────────

// CountByTemplateID 统计引用了指定模板的 NPC 数（供 TemplateHandler 引用检查）
func (s *NpcService) CountByTemplateID(ctx context.Context, templateID int64) (int64, error) {
	return s.store.CountByTemplateID(ctx, templateID)
}

// CountByBtTreeName 统计引用了指定行为树的 NPC 数（供 BtTreeHandler 引用检查）
func (s *NpcService) CountByBtTreeName(ctx context.Context, btName string) (int64, error) {
	return s.store.CountByBtTreeName(ctx, btName)
}

// CountByFsmRef 统计引用了指定 FSM 的 NPC 数（供 FsmConfigHandler 引用检查）
func (s *NpcService) CountByFsmRef(ctx context.Context, fsmName string) (int64, error) {
	return s.store.CountByFsmRef(ctx, fsmName)
}

// ListByTemplateID 分页查询引用了指定模板的 NPC 精简列表（供 TemplateHandler GetReferences）
func (s *NpcService) ListByTemplateID(ctx context.Context, templateID int64, page, pageSize int) ([]model.NPCLite, int64, error) {
	return s.store.ListByTemplateID(ctx, templateID, page, pageSize)
}

// ListByFsmRef 分页查询引用了指定状态机的 NPC 精简列表（供 FsmConfigHandler GetReferences）
func (s *NpcService) ListByFsmRef(ctx context.Context, fsmName string, page, pageSize int) ([]model.NPCLite, int64, error) {
	return s.store.ListByFsmRef(ctx, fsmName, page, pageSize)
}

// ListByBtTreeName 分页查询引用了指定行为树的 NPC 精简列表（供 BtTreeHandler GetReferences）
func (s *NpcService) ListByBtTreeName(ctx context.Context, btName string, page, pageSize int) ([]model.NPCLite, int64, error) {
	return s.store.ListByBtTreeName(ctx, btName, page, pageSize)
}

// ──────────────────────────────────────────────
// 导出（handler 编排，service 4 个纯方法 + ExportRows 直查）
//
// NpcService 严格遵守"分层职责"：不持有 fsmConfigService / btTreeService。
// 跨模块校验（FSM/BT 启用状态）由 handler 编排（见 ExportHandler.NPCTemplates）。
// ──────────────────────────────────────────────

// NPCExportRefs 导出引用反查索引
//
// CollectExportRefs 产物，BuildExportDanglingError 输入。
// FsmIndex: fsmName → 引用它的 NPC 名列表
// BtIndex:  btName  → 引用它的 (npcName, state) 列表
type NPCExportRefs struct {
	FsmIndex map[string][]string
	BtIndex  map[string][]NPCExportBtUsage
}

// NPCExportBtUsage 一条 BT 引用的来源（dangling details 反查用）
type NPCExportBtUsage struct {
	NPCName string
	State   string
}

// ExportRows 直查 MySQL 取所有已启用未删除 NPC 原始行
//
// 不走缓存（导出场景需要最新数据）。返回原始 model.NPC，
// 由调用方（handler）编排引用校验和最终装配。
func (s *NpcService) ExportRows(ctx context.Context) ([]model.NPC, error) {
	rows, err := s.store.ExportAll(ctx)
	if err != nil {
		slog.Error("service.导出NPC.取行失败", "error", err)
		return nil, fmt.Errorf("export rows: %w", err)
	}
	return rows, nil
}

// CollectExportRefs 纯函数：扫 rows 构建 FSM/BT 反查索引
//
// 解析每行 BtRefs JSON 失败立即返 error（数据损坏不放行）。
// 空 fsm_ref / 空 bt_refs map 不进入 index（视为合法的"无行为配置"）。
func (s *NpcService) CollectExportRefs(rows []model.NPC) (*NPCExportRefs, error) {
	refs := &NPCExportRefs{
		FsmIndex: make(map[string][]string),
		BtIndex:  make(map[string][]NPCExportBtUsage),
	}
	for _, n := range rows {
		if n.FsmRef != "" {
			refs.FsmIndex[n.FsmRef] = append(refs.FsmIndex[n.FsmRef], n.Name)
		}
		var btRefs map[string]string
		if err := json.Unmarshal(n.BtRefs, &btRefs); err != nil {
			return nil, fmt.Errorf("collect refs: unmarshal bt_refs (npc=%s): %w", n.Name, err)
		}
		for state, btName := range btRefs {
			refs.BtIndex[btName] = append(refs.BtIndex[btName], NPCExportBtUsage{
				NPCName: n.Name,
				State:   state,
			})
		}
	}
	return refs, nil
}

// BuildExportDanglingError 纯函数：notOK 名列表 + 反查索引 → 结构化错误
//
// 全部正常返回 nil。任一 notOK 非空时返回 *ExportDanglingRefError，
// Details 按 (FSM 全部) → (BT 全部) 顺序展开。
//
// 一个 NPC 多次引用同一悬空 BT 不会发生（bt_refs 是 map[state]btName，
// 同 NPC 内 state 唯一），所以 Details 不需要去重。
func (s *NpcService) BuildExportDanglingError(
	refs *NPCExportRefs,
	fsmNotOK []string,
	btNotOK []string,
) *errcode.ExportDanglingRefError {
	if len(fsmNotOK) == 0 && len(btNotOK) == 0 {
		return nil
	}
	details := make([]model.NPCExportDanglingRef, 0, len(fsmNotOK)+len(btNotOK))
	for _, fsmName := range fsmNotOK {
		for _, npcName := range refs.FsmIndex[fsmName] {
			details = append(details, model.NPCExportDanglingRef{
				NPCName:  npcName,
				RefType:  model.ExportRefTypeFsm,
				RefValue: fsmName,
				Reason:   model.ExportRefReasonMissingOrDisabled,
			})
		}
	}
	for _, btName := range btNotOK {
		for _, u := range refs.BtIndex[btName] {
			details = append(details, model.NPCExportDanglingRef{
				NPCName:  u.NPCName,
				RefType:  model.ExportRefTypeBt,
				RefValue: btName,
				Reason:   model.ExportRefReasonMissingOrDisabled,
				State:    u.State,
			})
		}
	}
	if len(details) == 0 {
		// 防御：notOK 非空但反查索引找不到对应 NPC（理论不可能，因为 handler
		// 把 keysOf(refs.*Index) 传进 CheckEnabledByNames 再回来），返 nil 避免空错误。
		return nil
	}
	return &errcode.ExportDanglingRefError{Details: details}
}

// AssembleExportItems 纯函数：rows → []NPCExportItem
//
// 抽自既有 ExportAll 的装配段。任一行解析失败立即返 error，不部分装配。
func (s *NpcService) AssembleExportItems(rows []model.NPC) ([]model.NPCExportItem, error) {
	items := make([]model.NPCExportItem, 0, len(rows))
	for _, n := range rows {
		item, err := assembleExportItem(n)
		if err != nil {
			slog.Error("service.导出NPC.装配失败", "error", err, "name", n.Name)
			return nil, fmt.Errorf("assemble export item for npc %q: %w", n.Name, err)
		}
		items = append(items, item)
	}
	return items, nil
}


// assembleExportItem 将 NPC 裸行组装为导出结构（包内 helper，AssembleExportItems 调用）
func assembleExportItem(n model.NPC) (model.NPCExportItem, error) {
	// 解析字段快照 → map[name]value
	var fieldEntries []model.NPCFieldEntry
	if err := json.Unmarshal(n.Fields, &fieldEntries); err != nil {
		return model.NPCExportItem{}, fmt.Errorf("unmarshal fields: %w", err)
	}
	fieldsMap := make(map[string]json.RawMessage, len(fieldEntries))
	for _, f := range fieldEntries {
		fieldsMap[f.Name] = f.Value
	}

	// 解析 bt_refs
	var btRefs map[string]string
	if err := json.Unmarshal(n.BtRefs, &btRefs); err != nil {
		return model.NPCExportItem{}, fmt.Errorf("unmarshal bt_refs: %w", err)
	}

	return model.NPCExportItem{
		Name: n.Name,
		Config: model.NPCExportConfig{
			TemplateRef: n.TemplateName,
			Fields:      fieldsMap,
			Behavior: model.NPCExportBehavior{
				FsmRef: n.FsmRef,
				BtRefs: btRefs,
			},
		},
	}, nil
}
