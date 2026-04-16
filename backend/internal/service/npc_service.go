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
		return nil, errcode.New(errcode.ErrNPCNotFound)
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
			return nil, errcode.New(errcode.ErrNPCNotFound)
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
				return nil, errcode.New(errcode.ErrNPCNotFound)
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
		return nil, errcode.New(errcode.ErrNPCNotFound)
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
			return nil, errcode.New(errcode.ErrNPCNotFound)
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

// ExportAll 导出所有已启用且未删除的 NPC，组装 NPCExportItem
//
// 直查 MySQL，不走缓存（导出场景需要最新数据）。
func (s *NpcService) ExportAll(ctx context.Context) ([]model.NPCExportItem, error) {
	rows, err := s.store.ExportAll(ctx)
	if err != nil {
		slog.Error("service.导出NPC失败", "error", err)
		return nil, fmt.Errorf("export npcs: %w", err)
	}

	items := make([]model.NPCExportItem, 0, len(rows))
	for _, n := range rows {
		item, err := assembleExportItem(n)
		if err != nil {
			slog.Error("service.导出NPC-组装失败", "error", err, "name", n.Name)
			return nil, fmt.Errorf("assemble export item for npc %q: %w", n.Name, err)
		}
		items = append(items, item)
	}
	return items, nil
}

// assembleExportItem 将 NPC 裸行组装为导出结构
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
