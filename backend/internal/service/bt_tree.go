package service

import (
	shared "github.com/yqihe/npc-ai-admin/backend/internal/service/shared"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/jmoiron/sqlx"
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
		return nil, errcode.Newf(errcode.ErrBtTreeNotFound, "行为树 ID=%d 不存在", id)
	}
	return d, nil
}

// paramSpec 单个参数规格，对应 bt_node_types.param_schema.params[i]。
// 仅 service 层消费，保持 unexported。
type paramSpec struct {
	Name     string   `json:"name"`
	Label    string   `json:"label"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Options  []string `json:"options,omitempty"`
}

// nodeParamSchema 节点类型的完整 params 规格
type nodeParamSchema struct {
	Params []paramSpec `json:"params"`
}

func (s *nodeParamSchema) hasParams() bool { return len(s.Params) > 0 }

// btNodeTypeLookup 给 validateConfigImpl 做的最小只读抽象。
// *storemysql.BtNodeTypeStore 天然满足；单测可注入 map-backed fake 避免 DB。
type btNodeTypeLookup interface {
	ListEnabledTypes(ctx context.Context) (map[string]string, error)
	ListParamSchemas(ctx context.Context) (map[string]json.RawMessage, error)
}

// validateConfig 解析 JSON 并递归校验节点树结构 —— 方法壳，委托到 validateConfigImpl。
func (s *BtTreeService) validateConfig(ctx context.Context, config json.RawMessage) error {
	return validateConfigImpl(ctx, s.nodeTypeStore, config)
}

// validateConfigImpl 行为树结构校验本体。抽成包级函数 + btNodeTypeLookup 接口，
// 预加载失败 / param_schema 解析失败 fail-fast（audit-T4 Q2 决策：不静默降级）。
func validateConfigImpl(ctx context.Context, lookup btNodeTypeLookup, config json.RawMessage) error {
	if len(config) == 0 {
		return errcode.Newf(errcode.ErrBtTreeConfigInvalid, "行为树 config 不能为空")
	}

	// 预加载节点类型（type_name → category）
	nodeTypes, err := lookup.ListEnabledTypes(ctx)
	if err != nil {
		return fmt.Errorf("load enabled node types: %w", err)
	}

	// 预加载 param_schema（原始 JSON）并逐条解析为 nodeParamSchema
	rawSchemas, err := lookup.ListParamSchemas(ctx)
	if err != nil {
		return fmt.Errorf("load param schemas: %w", err)
	}
	paramSchemas := make(map[string]*nodeParamSchema, len(rawSchemas))
	for typeName, raw := range rawSchemas {
		var ps nodeParamSchema
		if err := json.Unmarshal(raw, &ps); err != nil {
			slog.Error("service.行为树校验-解析节点类型参数规格失败",
				"type_name", typeName, "error", err, "raw", string(raw))
			return fmt.Errorf("unmarshal param_schema %q: %w", typeName, err)
		}
		paramSchemas[typeName] = &ps
	}

	var root map[string]any
	if err := json.Unmarshal(config, &root); err != nil {
		return errcode.Newf(errcode.ErrBtTreeConfigInvalid, "config 必须是合法 JSON 对象")
	}

	return validateBtNode(root, nodeTypes, paramSchemas, 0)
}

// validateBtNode 递归校验节点结构合法性
//
// nodeTypes:    type_name → category（enabled 且 not deleted）
// paramSchemas: type_name → 参数规格（预先 unmarshal 好，null 表示该类型无 params）
// depth:        当前递归深度，超过 20 返回 44006
func validateBtNode(node map[string]any, nodeTypes map[string]string, paramSchemas map[string]*nodeParamSchema, depth int) error {
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

	// 顶层字段白名单：仅允许 type / params / children / child
	// 拦截旧格式裸字段如 {type:"stub_action", action:"wait_idle"}
	for k := range node {
		if k != "type" && k != "params" && k != "children" && k != "child" {
			return errcode.Newf(errcode.ErrBtNodeBareFields,
				"节点 %q 含未知字段 %q（仅允许 type/params/children/child）", typeName, k)
		}
	}

	if err := validateBtCategory(category, typeName, node, nodeTypes, paramSchemas, depth); err != nil {
		return err
	}

	// 消费 param_schema 校验 params（仅当该节点类型声明了参数）
	if schema, ok := paramSchemas[typeName]; ok && schema.hasParams() {
		if err := validateNodeParams(typeName, node, schema); err != nil {
			return err
		}
	}

	return nil
}

// validateBtCategory 按 category 分派到具体分支校验器。
func validateBtCategory(category, typeName string, node map[string]any, nodeTypes map[string]string, paramSchemas map[string]*nodeParamSchema, depth int) error {
	switch category {
	case "composite":
		return validateBtComposite(typeName, node, nodeTypes, paramSchemas, depth)
	case "decorator":
		return validateBtDecorator(typeName, node, nodeTypes, paramSchemas, depth)
	case "leaf":
		return validateBtLeaf(node)
	}
	return nil
}

// validateBtComposite 校验 composite 节点结构并递归子节点。
func validateBtComposite(typeName string, node map[string]any, nodeTypes map[string]string, paramSchemas map[string]*nodeParamSchema, depth int) error {
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
		if err := validateBtNode(child, nodeTypes, paramSchemas, depth+1); err != nil {
			return err
		}
	}
	return nil
}

// validateBtDecorator 校验 decorator 节点结构并递归 child。
func validateBtDecorator(typeName string, node map[string]any, nodeTypes map[string]string, paramSchemas map[string]*nodeParamSchema, depth int) error {
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
	return validateBtNode(child, nodeTypes, paramSchemas, depth+1)
}

// validateBtLeaf 校验 leaf 节点不应有 children / child 字段。
func validateBtLeaf(node map[string]any) error {
	if _, hasChildren := node["children"]; hasChildren {
		return errcode.Newf(errcode.ErrBtTreeConfigInvalid, "leaf 节点不能有 children 字段")
	}
	if _, hasChild := node["child"]; hasChild {
		return errcode.Newf(errcode.ErrBtTreeConfigInvalid, "leaf 节点不能有 child 字段")
	}
	return nil
}

// validateNodeParams 校验节点 params 对象整体合法性
// 前提：schema 非 nil 且 schema.hasParams()
func validateNodeParams(typeName string, node map[string]any, schema *nodeParamSchema) error {
	paramsRaw, hasParams := node["params"]
	if !hasParams {
		return errcode.Newf(errcode.ErrBtNodeBareFields,
			"节点 %q 缺少 params 字段", typeName)
	}
	params, ok := paramsRaw.(map[string]any)
	if !ok {
		return errcode.Newf(errcode.ErrBtNodeBareFields,
			"节点 %q 的 params 必须是对象", typeName)
	}
	for _, p := range schema.Params {
		val, exists := params[p.Name]
		if p.Required && !exists {
			return errcode.Newf(errcode.ErrBtNodeParamMissing,
				"节点 %q 缺少必填参数 %q", typeName, p.Name)
		}
		if exists {
			if err := validateParamValue(typeName, p, val); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateParamValue 校验单个 param 的值类型
//
// 已支持的 schema 类型：bb_key / string / float / select。
// 未知 param.Type 显式 fail（audit-T4 Q2 决策：不静默通过，防止未来新增
// 类型时漏校验）。TODO(future): 如需 int/bool/array 参数，在此 switch 加 case。
func validateParamValue(typeName string, p paramSpec, val any) error {
	switch p.Type {
	case "bb_key", "string":
		s, ok := val.(string)
		if !ok || s == "" {
			return errcode.Newf(errcode.ErrBtNodeParamType,
				"节点 %q 参数 %q 必须是非空字符串", typeName, p.Name)
		}
	case "float":
		// json.Unmarshal 到 any 后所有数字都是 float64（dev-rules/go.md）
		if _, ok := val.(float64); !ok {
			return errcode.Newf(errcode.ErrBtNodeParamType,
				"节点 %q 参数 %q 必须是数字", typeName, p.Name)
		}
	case "select":
		s, ok := val.(string)
		if !ok {
			return errcode.Newf(errcode.ErrBtNodeParamType,
				"节点 %q 参数 %q 必须是字符串枚举", typeName, p.Name)
		}
		if len(p.Options) > 0 && !slices.Contains(p.Options, s) {
			return errcode.Newf(errcode.ErrBtNodeParamEnum,
				"节点 %q 参数 %q 取值 %q 不在允许集合 %v", typeName, p.Name, s, p.Options)
		}
	default:
		return errcode.Newf(errcode.ErrBtNodeParamType,
			"节点 %q 参数 %q 的 schema 类型 %q 未知（validator 不支持）", typeName, p.Name, p.Type)
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
			return nil, errcode.Newf(errcode.ErrBtTreeNotFound, "行为树 ID=%d 不存在", id)
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
				return nil, errcode.Newf(errcode.ErrBtTreeNotFound, "行为树 ID=%d 不存在", id)
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
		return nil, errcode.Newf(errcode.ErrBtTreeNotFound, "行为树 ID=%d 不存在", id)
	}
	return d, nil
}

// ---- 事务版方法（handler 跨模块编排用）----

// CreateInTx 事务内创建行为树（校验 + store 写入 + 节点类型引用同步，不清缓存）
func (s *BtTreeService) CreateInTx(ctx context.Context, tx *sqlx.Tx, req *model.CreateBtTreeRequest) (int64, error) {
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

	slog.Info("service.创建行为树成功(tx)", "id", id, "name", req.Name)
	return id, nil
}

// UpdateInTx 事务内编辑行为树（校验 + store 写入 + 节点类型引用同步，不清缓存）
//
// 返回旧 config（handler 用于提取旧 BB Keys diff）。
func (s *BtTreeService) UpdateInTx(ctx context.Context, tx *sqlx.Tx, req *model.UpdateBtTreeRequest) (*model.BtTree, error) {
	slog.Debug("service.编辑行为树", "id", req.ID)

	d, err := s.getOrNotFound(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	// 启用中禁止编辑
	if d.Enabled {
		return nil, errcode.New(errcode.ErrBtTreeEditNotDisabled)
	}

	// 校验节点树结构
	if err := s.validateConfig(ctx, req.Config); err != nil {
		return nil, err
	}

	if err := s.store.UpdateInTx(ctx, tx, req); err != nil {
		if errors.Is(err, errcode.ErrVersionConflict) {
			return nil, errcode.New(errcode.ErrBtTreeVersionConflict)
		}
		slog.Error("service.编辑行为树失败", "error", err, "id", req.ID)
		return nil, fmt.Errorf("update bt_tree: %w", err)
	}

	if err := s.store.SyncNodeTypeRefsTx(ctx, tx, req.ID, req.Config); err != nil {
		slog.Error("service.编辑行为树-同步节点类型引用失败", "error", err, "id", req.ID)
		return nil, fmt.Errorf("sync node type refs: %w", err)
	}

	slog.Info("service.编辑行为树成功(tx)", "id", req.ID)
	return d, nil // 返回旧数据，handler 用于 BB Key diff
}

// SoftDeleteInTx 事务内软删除行为树（前置校验 + store 写入 + 节点类型引用清理，不清缓存）
func (s *BtTreeService) SoftDeleteInTx(ctx context.Context, tx *sqlx.Tx, id int64) (*model.BtTree, error) {
	slog.Debug("service.删除行为树", "id", id)

	d, err := s.getOrNotFound(ctx, id)
	if err != nil {
		return nil, err
	}

	// 启用中禁止删除
	if d.Enabled {
		return nil, errcode.New(errcode.ErrBtTreeDeleteNotDisabled)
	}

	if err := s.store.SoftDeleteInTx(ctx, tx, id); err != nil {
		if errors.Is(err, errcode.ErrNotFound) {
			return nil, errcode.Newf(errcode.ErrBtTreeNotFound, "行为树 ID=%d 不存在", id)
		}
		slog.Error("service.删除行为树失败", "error", err, "id", id)
		return nil, fmt.Errorf("soft delete bt_tree: %w", err)
	}

	if err := s.store.DeleteNodeTypeRefsTx(ctx, tx, id); err != nil {
		slog.Error("service.删除行为树-清理节点类型引用失败", "error", err, "id", id)
		return nil, fmt.Errorf("delete node type refs: %w", err)
	}

	slog.Info("service.删除行为树成功(tx)", "id", id, "name", d.Name)
	return d, nil
}

// InvalidateDetail 清单条缓存（handler commit 前调用）
func (s *BtTreeService) InvalidateDetail(ctx context.Context, id int64) {
	s.cache.DelDetail(ctx, id)
}

// InvalidateList 清列表缓存（handler commit 前调用）
func (s *BtTreeService) InvalidateList(ctx context.Context) {
	s.cache.InvalidateList(ctx)
}

// ExtractBBKeys 从行为树 config JSON 中提取所有 BB Key 名（去重），供 handler 层用于引用同步。
//
// 内部预加载节点类型 bb_key 参数名，再解析 config 提取值。
// 返回 map[keyName → true]。
func (s *BtTreeService) ExtractBBKeys(ctx context.Context, config json.RawMessage) (map[string]bool, error) {
	nodeParamTypes, err := s.nodeTypeStore.ListBBKeyParamNames(ctx)
	if err != nil {
		return nil, fmt.Errorf("list bb key param names: %w", err)
	}
	return storemysql.ExtractBBKeysFromConfig(config, nodeParamTypes)
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
