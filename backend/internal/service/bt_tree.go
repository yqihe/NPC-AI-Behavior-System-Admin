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

// BtTreeService 行为树业务逻辑
type BtTreeService struct {
	store         *storemysql.BtTreeStore
	nodeTypeStore *storemysql.BtNodeTypeStore
	cache         *storeredis.BtTreeCache
	pagCfg        *config.PaginationConfig
	btCfg         *config.BtTreeConfig
}

// NewBtTreeService 创建 BtTreeService
func NewBtTreeService(
	store *storemysql.BtTreeStore,
	nodeTypeStore *storemysql.BtNodeTypeStore,
	redisCache *storeredis.BtTreeCache,
	pagCfg *config.PaginationConfig,
	btCfg *config.BtTreeConfig,
) *BtTreeService {
	return &BtTreeService{
		store:         store,
		nodeTypeStore: nodeTypeStore,
		cache:         redisCache,
		pagCfg:        pagCfg,
		btCfg:         btCfg,
	}
}

// ---- 内部辅助 ----

func (s *BtTreeService) getOrNotFound(ctx context.Context, id int64) (*model.BtTree, error) {
	d, err := s.store.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get bt_tree %d: %w", id, err)
	}
	if d == nil {
		return nil, errcode.New(errcode.ErrBtTreeNotFound)
	}
	return d, nil
}

// validateConfig 解析 JSON 并递归校验节点树结构
func (s *BtTreeService) validateConfig(ctx context.Context, config json.RawMessage) error {
	if len(config) == 0 {
		return errcode.Newf(errcode.ErrBtTreeConfigInvalid, "行为树 config 不能为空")
	}

	// 预加载节点类型（type_name → category）
	nodeTypes, err := s.nodeTypeStore.ListEnabledTypes(ctx)
	if err != nil {
		return fmt.Errorf("load enabled node types: %w", err)
	}

	var root map[string]any
	if err := json.Unmarshal(config, &root); err != nil {
		return errcode.Newf(errcode.ErrBtTreeConfigInvalid, "config 必须是合法 JSON 对象")
	}

	return validateBtNode(root, nodeTypes, 0)
}

// validateBtNode 递归校验节点结构合法性
//
// nodeTypes: type_name → category（从 BtNodeTypeStore 预加载 enabled 且 not deleted 的类型）
// depth: 当前递归深度，超过 20 返回 44006
func validateBtNode(node map[string]any, nodeTypes map[string]string, depth int) error {
	if depth > 20 {
		return errcode.New(errcode.ErrBtTreeNodeDepthExceeded)
	}

	typeName, ok := node["type"].(string)
	if !ok || typeName == "" {
		return errcode.New(errcode.ErrBtTreeConfigInvalid)
	}

	category, exists := nodeTypes[typeName]
	if !exists {
		return errcode.Newf(errcode.ErrBtTreeNodeTypeNotFound, "节点类型 %q 不存在或已禁用", typeName)
	}

	switch category {
	case "composite":
		children, ok := node["children"].([]any)
		if !ok || len(children) == 0 {
			return errcode.Newf(errcode.ErrBtTreeConfigInvalid, "composite 节点 %q 必须有非空 children", typeName)
		}
		if _, hasChild := node["child"]; hasChild {
			return errcode.Newf(errcode.ErrBtTreeConfigInvalid, "composite 节点不应有 child 字段")
		}
		for _, c := range children {
			child, ok := c.(map[string]any)
			if !ok {
				return errcode.New(errcode.ErrBtTreeConfigInvalid)
			}
			if err := validateBtNode(child, nodeTypes, depth+1); err != nil {
				return err
			}
		}

	case "decorator":
		childRaw, ok := node["child"]
		if !ok || childRaw == nil {
			return errcode.Newf(errcode.ErrBtTreeConfigInvalid, "decorator 节点 %q 必须有 child", typeName)
		}
		child, ok := childRaw.(map[string]any)
		if !ok {
			return errcode.New(errcode.ErrBtTreeConfigInvalid)
		}
		if _, hasChildren := node["children"]; hasChildren {
			return errcode.Newf(errcode.ErrBtTreeConfigInvalid, "decorator 节点不应有 children 字段")
		}
		if err := validateBtNode(child, nodeTypes, depth+1); err != nil {
			return err
		}

	case "leaf":
		if _, hasChildren := node["children"]; hasChildren {
			return errcode.Newf(errcode.ErrBtTreeConfigInvalid, "leaf 节点不能有 children 字段")
		}
		if _, hasChild := node["child"]; hasChild {
			return errcode.Newf(errcode.ErrBtTreeConfigInvalid, "leaf 节点不能有 child 字段")
		}
	}

	return nil
}

// ---- CRUD ----

// List 分页列表
func (s *BtTreeService) List(ctx context.Context, q *model.BtTreeListQuery) (*model.ListData, error) {
	shared.NormalizePagination(&q.Page, &q.PageSize, s.pagCfg.DefaultPage, s.pagCfg.DefaultPageSize, s.pagCfg.MaxPageSize)

	// 查缓存
	if cached, hit, err := s.cache.GetList(ctx, q); err == nil && hit {
		slog.Debug("service.行为树列表.缓存命中")
		return cached.ToListData(), nil
	}

	// 查 MySQL
	items, total, err := s.store.List(ctx, q)
	if err != nil {
		return nil, err
	}

	// 写缓存
	listData := &model.BtTreeListData{
		Items:    items,
		Total:    total,
		Page:     q.Page,
		PageSize: q.PageSize,
	}
	s.cache.SetList(ctx, q, listData)

	return listData.ToListData(), nil
}

// GetByID 查详情（Cache-Aside + 分布式锁 + 空标记）
func (s *BtTreeService) GetByID(ctx context.Context, id int64) (*model.BtTree, error) {
	// 1. 查缓存
	if cached, hit, err := s.cache.GetDetail(ctx, id); err == nil && hit {
		if cached == nil {
			return nil, errcode.New(errcode.ErrBtTreeNotFound)
		}
		return cached, nil
	}

	// 2. 分布式锁防击穿
	lockID, lockErr := s.cache.TryLock(ctx, id, rcfg.LockExpire)
	if lockErr != nil {
		slog.Warn("service.获取行为树锁失败，降级直查MySQL", "error", lockErr, "id", id)
	}
	if lockID != "" {
		defer s.cache.Unlock(ctx, id, lockID)
		// double-check
		if cached, hit, err := s.cache.GetDetail(ctx, id); err == nil && hit {
			if cached == nil {
				return nil, errcode.New(errcode.ErrBtTreeNotFound)
			}
			return cached, nil
		}
	}

	// 3. 查 MySQL
	d, err := s.store.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get bt_tree: %w", err)
	}

	// 4. 写缓存（含空标记）
	s.cache.SetDetail(ctx, id, d)

	if d == nil {
		return nil, errcode.New(errcode.ErrBtTreeNotFound)
	}
	return d, nil
}

// Create 创建行为树
//
// 事务内同时写 bt_trees + bt_node_type_refs，保证节点类型引用表与 config 一致。
func (s *BtTreeService) Create(ctx context.Context, req *model.CreateBtTreeRequest) (int64, error) {
	slog.Debug("service.创建行为树", "name", req.Name)

	// 校验节点树结构
	if err := s.validateConfig(ctx, req.Config); err != nil {
		return 0, err
	}

	// name 唯一性（含软删除）
	exists, err := s.store.ExistsByName(ctx, req.Name)
	if err != nil {
		slog.Error("service.创建行为树-检查唯一性失败", "error", err, "name", req.Name)
		return 0, fmt.Errorf("check name exists: %w", err)
	}
	if exists {
		return 0, errcode.Newf(errcode.ErrBtTreeNameExists, "行为树标识 '%s' 已存在", req.Name)
	}

	// 事务：写 bt_trees + bt_node_type_refs
	tx, err := s.store.DB().BeginTxx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			slog.Warn("service.创建行为树事务回滚失败", "error", rbErr)
		}
	}()

	id, err := s.store.CreateInTx(ctx, tx, req)
	if err != nil {
		if errors.Is(err, errcode.ErrDuplicate) {
			return 0, errcode.Newf(errcode.ErrBtTreeNameExists, "行为树标识 '%s' 已存在", req.Name)
		}
		slog.Error("service.创建行为树失败", "error", err, "name", req.Name)
		return 0, fmt.Errorf("create bt_tree: %w", err)
	}

	if err := s.store.SyncNodeTypeRefsTx(ctx, tx, id, req.Config); err != nil {
		slog.Error("service.创建行为树-同步节点类型引用失败", "error", err, "id", id)
		return 0, fmt.Errorf("sync node type refs: %w", err)
	}

	// 先清缓存再 Commit
	s.cache.InvalidateList(ctx)

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}

	slog.Info("service.创建行为树成功", "id", id, "name", req.Name)
	return id, nil
}

// Update 编辑行为树（启用中禁止编辑）
//
// 事务内同时更新 bt_trees + bt_node_type_refs。
func (s *BtTreeService) Update(ctx context.Context, req *model.UpdateBtTreeRequest) error {
	slog.Debug("service.编辑行为树", "id", req.ID)

	d, err := s.getOrNotFound(ctx, req.ID)
	if err != nil {
		return err
	}

	// 启用中禁止编辑
	if d.Enabled {
		return errcode.New(errcode.ErrBtTreeEditNotDisabled)
	}

	// 校验节点树结构
	if err := s.validateConfig(ctx, req.Config); err != nil {
		return err
	}

	// 事务：更新 bt_trees + bt_node_type_refs
	tx, err := s.store.DB().BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			slog.Warn("service.编辑行为树事务回滚失败", "error", rbErr)
		}
	}()

	if err := s.store.UpdateInTx(ctx, tx, req); err != nil {
		if errors.Is(err, errcode.ErrVersionConflict) {
			return errcode.New(errcode.ErrBtTreeVersionConflict)
		}
		slog.Error("service.编辑行为树失败", "error", err, "id", req.ID)
		return fmt.Errorf("update bt_tree: %w", err)
	}

	if err := s.store.SyncNodeTypeRefsTx(ctx, tx, req.ID, req.Config); err != nil {
		slog.Error("service.编辑行为树-同步节点类型引用失败", "error", err, "id", req.ID)
		return fmt.Errorf("sync node type refs: %w", err)
	}

	// 先清缓存再 Commit
	s.cache.DelDetail(ctx, req.ID)
	s.cache.InvalidateList(ctx)

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	slog.Info("service.编辑行为树成功", "id", req.ID)
	return nil
}

// Delete 软删除行为树（启用中禁止删除）
//
// 事务内同时软删 bt_trees + 清理 bt_node_type_refs。
func (s *BtTreeService) Delete(ctx context.Context, id int64) error {
	slog.Debug("service.删除行为树", "id", id)

	d, err := s.getOrNotFound(ctx, id)
	if err != nil {
		return err
	}

	// 启用中禁止删除
	if d.Enabled {
		return errcode.New(errcode.ErrBtTreeDeleteNotDisabled)
	}

	// 事务：软删 bt_trees + 清理 bt_node_type_refs
	tx, err := s.store.DB().BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			slog.Warn("service.删除行为树事务回滚失败", "error", rbErr)
		}
	}()

	if err := s.store.SoftDeleteInTx(ctx, tx, id); err != nil {
		if errors.Is(err, errcode.ErrNotFound) {
			return errcode.New(errcode.ErrBtTreeNotFound)
		}
		slog.Error("service.删除行为树失败", "error", err, "id", id)
		return fmt.Errorf("soft delete bt_tree: %w", err)
	}

	if err := s.store.DeleteNodeTypeRefsTx(ctx, tx, id); err != nil {
		slog.Error("service.删除行为树-清理节点类型引用失败", "error", err, "id", id)
		return fmt.Errorf("delete node type refs: %w", err)
	}

	// 先清缓存再 Commit
	s.cache.DelDetail(ctx, id)
	s.cache.InvalidateList(ctx)

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	slog.Info("service.删除行为树成功", "id", id, "name", d.Name)
	return nil
}

// ToggleEnabled 切换启用/停用
func (s *BtTreeService) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) error {
	slog.Debug("service.切换行为树启用", "id", req.ID)

	if _, err := s.getOrNotFound(ctx, req.ID); err != nil {
		return err
	}

	if err := s.store.ToggleEnabled(ctx, req); err != nil {
		if errors.Is(err, errcode.ErrVersionConflict) {
			return errcode.New(errcode.ErrBtTreeVersionConflict)
		}
		slog.Error("service.切换行为树启用失败", "error", err, "id", req.ID)
		return fmt.Errorf("toggle bt_tree enabled: %w", err)
	}

	// 清缓存
	s.cache.DelDetail(ctx, req.ID)
	s.cache.InvalidateList(ctx)

	slog.Info("service.切换行为树启用成功", "id", req.ID, "enabled", req.Enabled)
	return nil
}

// CheckName name 唯一性校验
func (s *BtTreeService) CheckName(ctx context.Context, name string) (*model.CheckNameResult, error) {
	exists, err := s.store.ExistsByName(ctx, name)
	if err != nil {
		slog.Error("service.校验行为树标识失败", "error", err, "name", name)
		return nil, fmt.Errorf("check name: %w", err)
	}
	if exists {
		return &model.CheckNameResult{Available: false, Message: "该行为树标识已存在"}, nil
	}
	return &model.CheckNameResult{Available: true, Message: "该标识可用"}, nil
}

// CheckEnabledByNames 批量校验行为树是否存在且已启用（供 NPC handler 调用）
//
// 返回不存在或已停用的 name 列表（notOK）。
// names 为空时直接返回 nil, nil，不发起查询。
func (s *BtTreeService) CheckEnabledByNames(ctx context.Context, names []string) (notOK []string, err error) {
	if len(names) == 0 {
		return nil, nil
	}
	enabledSet, err := s.store.GetEnabledByNames(ctx, names)
	if err != nil {
		return nil, fmt.Errorf("get enabled bt_trees by names: %w", err)
	}
	for _, name := range names {
		if !enabledSet[name] {
			notOK = append(notOK, name)
		}
	}
	return notOK, nil
}

// ExportAll 导出所有已启用且未删除的行为树（直查 MySQL，不走缓存）
func (s *BtTreeService) ExportAll(ctx context.Context) ([]model.BtTreeExportItem, error) {
	items, err := s.store.ExportAll(ctx)
	if err != nil {
		slog.Error("service.导出行为树失败", "error", err)
		return nil, fmt.Errorf("export bt_trees: %w", err)
	}
	return items, nil
}
