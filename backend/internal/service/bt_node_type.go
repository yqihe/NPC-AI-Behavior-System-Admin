package service

import (
	shared "github.com/yqihe/npc-ai-admin/backend/internal/service/shared"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"unicode/utf8"

	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	storemysql "github.com/yqihe/npc-ai-admin/backend/internal/store/mysql"
	storeredis "github.com/yqihe/npc-ai-admin/backend/internal/store/redis"
	rcfg "github.com/yqihe/npc-ai-admin/backend/internal/store/redis/shared"
)

// validParamTypes 合法的 param_schema 参数类型枚举
var validParamTypes = map[string]bool{
	"bb_key":  true,
	"string":  true,
	"float":   true,
	"integer": true,
	"bool":    true,
	"select":  true,
}

// validCategories 合法的 category 枚举
var validCategories = map[string]bool{
	"composite": true,
	"decorator": true,
	"leaf":      true,
}

// btNodeTypeNameRe type_name 合法格式：小写字母开头，仅含小写字母/数字/下划线
var btNodeTypeNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// BtNodeTypeService 节点类型业务逻辑
type BtNodeTypeService struct {
	store    *storemysql.BtNodeTypeStore
	btStore  *storemysql.BtTreeStore
	cache    *storeredis.BtNodeTypeCache
	pagCfg   *config.PaginationConfig
	nodeCfg  *config.BtNodeTypeConfig
}

// NewBtNodeTypeService 创建 BtNodeTypeService
func NewBtNodeTypeService(
	store *storemysql.BtNodeTypeStore,
	btStore *storemysql.BtTreeStore,
	redisCache *storeredis.BtNodeTypeCache,
	pagCfg *config.PaginationConfig,
	nodeCfg *config.BtNodeTypeConfig,
) *BtNodeTypeService {
	return &BtNodeTypeService{
		store:   store,
		btStore: btStore,
		cache:   redisCache,
		pagCfg:  pagCfg,
		nodeCfg: nodeCfg,
	}
}

// ---- 内部辅助 ----

func (s *BtNodeTypeService) getOrNotFound(ctx context.Context, id int64) (*model.BtNodeType, error) {
	d, err := s.store.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, errcode.ErrNotFound) {
			return nil, errcode.New(errcode.ErrBtNodeTypeNotFound)
		}
		return nil, fmt.Errorf("get bt_node_type %d: %w", id, err)
	}
	return d, nil
}

// validateTypeName 校验 type_name 格式和长度
func (s *BtNodeTypeService) validateTypeName(name string) error {
	if name == "" {
		return errcode.Newf(errcode.ErrBtNodeTypeNameInvalid, "type_name 不能为空")
	}
	if len(name) > s.nodeCfg.NameMaxLength {
		return errcode.Newf(errcode.ErrBtNodeTypeNameInvalid, "type_name 长度不能超过 %d", s.nodeCfg.NameMaxLength)
	}
	if !btNodeTypeNameRe.MatchString(name) {
		return errcode.Newf(errcode.ErrBtNodeTypeNameInvalid, "type_name 只能包含小写字母、数字、下划线，且以小写字母开头")
	}
	return nil
}

// validateParamSchema 校验 param_schema 结构合法性
//
// 格式：{"params": [{"name":"...", "label":"...", "type":"...", "required":bool}, ...]}
// select 类型必须有非空 options 数组。
func validateParamSchema(schema json.RawMessage) error {
	if len(schema) == 0 {
		return errcode.Newf(errcode.ErrBtNodeTypeParamSchemaInvalid, "param_schema 不能为空")
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(schema, &top); err != nil {
		return errcode.Newf(errcode.ErrBtNodeTypeParamSchemaInvalid, "param_schema 必须是 JSON 对象")
	}
	rawParams, ok := top["params"]
	if !ok {
		return errcode.Newf(errcode.ErrBtNodeTypeParamSchemaInvalid, "param_schema 缺少 params 字段")
	}

	var params []map[string]json.RawMessage
	if err := json.Unmarshal(rawParams, &params); err != nil {
		return errcode.Newf(errcode.ErrBtNodeTypeParamSchemaInvalid, "param_schema.params 必须是数组")
	}

	for i, p := range params {
		nameRaw, ok := p["name"]
		if !ok {
			return errcode.Newf(errcode.ErrBtNodeTypeParamSchemaInvalid, "params[%d] 缺少 name 字段", i)
		}
		var name string
		if err := json.Unmarshal(nameRaw, &name); err != nil || name == "" {
			return errcode.Newf(errcode.ErrBtNodeTypeParamSchemaInvalid, "params[%d].name 必须是非空字符串", i)
		}

		labelRaw, ok := p["label"]
		if !ok {
			return errcode.Newf(errcode.ErrBtNodeTypeParamSchemaInvalid, "params[%d] 缺少 label 字段", i)
		}
		var label string
		if err := json.Unmarshal(labelRaw, &label); err != nil || label == "" {
			return errcode.Newf(errcode.ErrBtNodeTypeParamSchemaInvalid, "params[%d].label 必须是非空字符串", i)
		}

		typeRaw, ok := p["type"]
		if !ok {
			return errcode.Newf(errcode.ErrBtNodeTypeParamSchemaInvalid, "params[%d] 缺少 type 字段", i)
		}
		var paramType string
		if err := json.Unmarshal(typeRaw, &paramType); err != nil || !validParamTypes[paramType] {
			return errcode.Newf(errcode.ErrBtNodeTypeParamSchemaInvalid,
				"params[%d].type 非法，合法值: bb_key/string/float/integer/bool/select", i)
		}

		// select 类型必须有非空 options 数组
		if paramType == "select" {
			optRaw, ok := p["options"]
			if !ok {
				return errcode.Newf(errcode.ErrBtNodeTypeParamSchemaInvalid,
					"params[%d]: select 类型必须有 options 字段", i)
			}
			var opts []any
			if err := json.Unmarshal(optRaw, &opts); err != nil || len(opts) == 0 {
				return errcode.Newf(errcode.ErrBtNodeTypeParamSchemaInvalid,
					"params[%d]: select 类型的 options 不能为空", i)
			}
		}
	}
	return nil
}

// ---- CRUD ----

// List 分页列表
func (s *BtNodeTypeService) List(ctx context.Context, q *model.BtNodeTypeListQuery) (*model.ListData, error) {
	shared.NormalizePagination(&q.Page, &q.PageSize, s.pagCfg.DefaultPage, s.pagCfg.DefaultPageSize, s.pagCfg.MaxPageSize)

	// 查缓存
	if cached, hit, err := s.cache.GetList(ctx, q); err == nil && hit {
		slog.Debug("service.节点类型列表.缓存命中")
		return cached.ToListData(), nil
	}

	// 查 MySQL
	items, total, err := s.store.List(ctx, q)
	if err != nil {
		return nil, err
	}

	// 写缓存
	listData := &model.BtNodeTypeListData{
		Items:    items,
		Total:    total,
		Page:     q.Page,
		PageSize: q.PageSize,
	}
	s.cache.SetList(ctx, q, listData)

	return listData.ToListData(), nil
}

// GetByID 查详情（Cache-Aside + 分布式锁 + 空标记）
func (s *BtNodeTypeService) GetByID(ctx context.Context, id int64) (*model.BtNodeType, error) {
	// 1. 查缓存
	if cached, hit, err := s.cache.GetDetail(ctx, id); err == nil && hit {
		if cached == nil {
			return nil, errcode.New(errcode.ErrBtNodeTypeNotFound)
		}
		return cached, nil
	}

	// 2. 分布式锁防击穿
	lockID, lockErr := s.cache.TryLock(ctx, id, rcfg.LockExpire)
	if lockErr != nil {
		slog.Warn("service.获取节点类型锁失败，降级直查MySQL", "error", lockErr, "id", id)
	}
	if lockID != "" {
		defer s.cache.Unlock(ctx, id, lockID)
		// double-check
		if cached, hit, err := s.cache.GetDetail(ctx, id); err == nil && hit {
			if cached == nil {
				return nil, errcode.New(errcode.ErrBtNodeTypeNotFound)
			}
			return cached, nil
		}
	}

	// 3. 查 MySQL
	d, err := s.store.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, errcode.ErrNotFound) {
			s.cache.SetDetail(ctx, id, nil)
			return nil, errcode.New(errcode.ErrBtNodeTypeNotFound)
		}
		return nil, fmt.Errorf("get bt_node_type: %w", err)
	}

	// 4. 写缓存
	s.cache.SetDetail(ctx, id, d)
	return d, nil
}

// Create 创建节点类型
func (s *BtNodeTypeService) Create(ctx context.Context, req *model.CreateBtNodeTypeRequest) (int64, error) {
	slog.Debug("service.创建节点类型", "type_name", req.TypeName)

	// type_name 格式校验
	if err := s.validateTypeName(req.TypeName); err != nil {
		return 0, err
	}

	// label 长度校验（中文用 RuneCountInString）
	if req.Label == "" || utf8.RuneCountInString(req.Label) > s.nodeCfg.LabelMaxLength {
		return 0, errcode.Newf(errcode.ErrBadRequest, "label 不能为空且长度不能超过 %d", s.nodeCfg.LabelMaxLength)
	}

	// category 校验
	if !validCategories[req.Category] {
		return 0, errcode.New(errcode.ErrBtNodeTypeCategoryInvalid)
	}

	// param_schema 校验
	if err := validateParamSchema(req.ParamSchema); err != nil {
		return 0, err
	}

	// type_name 唯一性（含软删除）
	exists, err := s.store.ExistsByTypeName(ctx, req.TypeName)
	if err != nil {
		slog.Error("service.创建节点类型-检查唯一性失败", "error", err, "type_name", req.TypeName)
		return 0, fmt.Errorf("check type_name exists: %w", err)
	}
	if exists {
		return 0, errcode.Newf(errcode.ErrBtNodeTypeNameExists, "节点类型标识 '%s' 已存在", req.TypeName)
	}

	// 写 MySQL
	id, err := s.store.Create(ctx, req)
	if err != nil {
		if errors.Is(err, errcode.ErrDuplicate) {
			return 0, errcode.Newf(errcode.ErrBtNodeTypeNameExists, "节点类型标识 '%s' 已存在", req.TypeName)
		}
		slog.Error("service.创建节点类型失败", "error", err, "type_name", req.TypeName)
		return 0, fmt.Errorf("create bt_node_type: %w", err)
	}

	// 清列表缓存
	s.cache.InvalidateList(ctx)

	slog.Info("service.创建节点类型成功", "id", id, "type_name", req.TypeName)
	return id, nil
}

// Update 编辑节点类型（内置类型不可编辑）
func (s *BtNodeTypeService) Update(ctx context.Context, req *model.UpdateBtNodeTypeRequest) error {
	slog.Debug("service.编辑节点类型", "id", req.ID)

	d, err := s.getOrNotFound(ctx, req.ID)
	if err != nil {
		return err
	}

	// 内置类型不可编辑
	if d.IsBuiltin {
		return errcode.New(errcode.ErrBtNodeTypeBuiltinEdit)
	}

	// 启用中禁止编辑
	if d.Enabled {
		return errcode.New(errcode.ErrBtNodeTypeEditNotDisabled)
	}

	// label 长度校验
	if req.Label == "" || utf8.RuneCountInString(req.Label) > s.nodeCfg.LabelMaxLength {
		return errcode.Newf(errcode.ErrBadRequest, "label 不能为空且长度不能超过 %d", s.nodeCfg.LabelMaxLength)
	}

	// param_schema 校验
	if err := validateParamSchema(req.ParamSchema); err != nil {
		return err
	}

	// 乐观锁更新
	if err := s.store.Update(ctx, req); err != nil {
		if errors.Is(err, errcode.ErrVersionConflict) {
			return errcode.New(errcode.ErrBtNodeTypeVersionConflict)
		}
		slog.Error("service.编辑节点类型失败", "error", err, "id", req.ID)
		return fmt.Errorf("update bt_node_type: %w", err)
	}

	// 清缓存
	s.cache.DelDetail(ctx, req.ID)
	s.cache.InvalidateList(ctx)

	slog.Info("service.编辑节点类型成功", "id", req.ID)
	return nil
}

// Delete 软删除节点类型
//
// 内置类型返回 44023；启用中返回 44020；
// 被行为树引用返回 (*BtNodeTypeDeleteResult{ReferencedBy:[...]}, 44022)。
func (s *BtNodeTypeService) Delete(ctx context.Context, id int64, version int) (*model.BtNodeTypeDeleteResult, error) {
	slog.Debug("service.删除节点类型", "id", id)

	d, err := s.getOrNotFound(ctx, id)
	if err != nil {
		return nil, err
	}

	// 内置类型不可删除
	if d.IsBuiltin {
		return nil, errcode.New(errcode.ErrBtNodeTypeBuiltinDelete)
	}

	// 启用中禁止删除
	if d.Enabled {
		return nil, errcode.New(errcode.ErrBtNodeTypeDeleteNotDisabled)
	}

	// 引用检查：扫描 bt_trees.config 中是否有节点使用该 type_name
	used, err := s.btStore.IsNodeTypeUsed(ctx, d.TypeName)
	if err != nil {
		slog.Error("service.删除节点类型-引用扫描失败", "error", err, "type_name", d.TypeName)
		return nil, fmt.Errorf("scan bt_tree refs: %w", err)
	}
	if used {
		refs, err := s.btStore.GetNodeTypeUsages(ctx, d.TypeName)
		if err != nil {
			slog.Error("service.删除节点类型-获取引用列表失败", "error", err, "type_name", d.TypeName)
			return nil, fmt.Errorf("get bt_tree usages: %w", err)
		}
		slog.Info("service.删除节点类型-被引用拒绝", "type_name", d.TypeName, "ref_count", len(refs))
		return &model.BtNodeTypeDeleteResult{ReferencedBy: refs}, errcode.New(errcode.ErrBtNodeTypeRefDelete)
	}

	// 软删除
	if err := s.store.Delete(ctx, id, version); err != nil {
		if errors.Is(err, errcode.ErrVersionConflict) {
			return nil, errcode.New(errcode.ErrBtNodeTypeVersionConflict)
		}
		slog.Error("service.删除节点类型失败", "error", err, "id", id)
		return nil, fmt.Errorf("soft delete bt_node_type: %w", err)
	}

	// 清缓存
	s.cache.DelDetail(ctx, id)
	s.cache.InvalidateList(ctx)

	slog.Info("service.删除节点类型成功", "id", id, "type_name", d.TypeName)
	return &model.BtNodeTypeDeleteResult{ID: id, TypeName: d.TypeName, Label: d.Label}, nil
}

// ToggleEnabled 切换启用/停用
func (s *BtNodeTypeService) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) error {
	slog.Debug("service.切换节点类型启用", "id", req.ID)

	if _, err := s.getOrNotFound(ctx, req.ID); err != nil {
		return err
	}

	if err := s.store.ToggleEnabled(ctx, req); err != nil {
		if errors.Is(err, errcode.ErrVersionConflict) {
			return errcode.New(errcode.ErrBtNodeTypeVersionConflict)
		}
		slog.Error("service.切换节点类型启用失败", "error", err, "id", req.ID)
		return fmt.Errorf("toggle bt_node_type enabled: %w", err)
	}

	// 清缓存
	s.cache.DelDetail(ctx, req.ID)
	s.cache.InvalidateList(ctx)

	slog.Info("service.切换节点类型启用成功", "id", req.ID, "enabled", req.Enabled)
	return nil
}

// CheckName type_name 唯一性校验
func (s *BtNodeTypeService) CheckName(ctx context.Context, typeName string) (*model.CheckNameResult, error) {
	exists, err := s.store.ExistsByTypeName(ctx, typeName)
	if err != nil {
		slog.Error("service.校验节点类型标识失败", "error", err, "type_name", typeName)
		return nil, fmt.Errorf("check type_name: %w", err)
	}
	if exists {
		return &model.CheckNameResult{Available: false, Message: "该节点类型标识已存在"}, nil
	}
	return &model.CheckNameResult{Available: true, Message: "该标识可用"}, nil
}
