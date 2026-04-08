package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/yqihe/npc-ai-admin/backend/internal/cache"
	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	storemysql "github.com/yqihe/npc-ai-admin/backend/internal/store/mysql"
	storeredis "github.com/yqihe/npc-ai-admin/backend/internal/store/redis"
)

// FieldService 字段管理业务逻辑
type FieldService struct {
	fieldStore    *storemysql.FieldStore
	fieldRefStore *storemysql.FieldRefStore
	fieldCache    *storeredis.FieldCache
	dictCache     *cache.DictCache
	pagCfg        *config.PaginationConfig
}

// NewFieldService 创建 FieldService
func NewFieldService(fieldStore *storemysql.FieldStore, fieldRefStore *storemysql.FieldRefStore, fieldCache *storeredis.FieldCache, dictCache *cache.DictCache, pagCfg *config.PaginationConfig) *FieldService {
	return &FieldService{
		fieldStore:    fieldStore,
		fieldRefStore: fieldRefStore,
		fieldCache:    fieldCache,
		dictCache:     dictCache,
		pagCfg:        pagCfg,
	}
}

// ---- 业务校验（需查缓存/DB） ----

func (s *FieldService) checkTypeExists(typ string) *errcode.Error {
	if !s.dictCache.Exists("field_type", typ) {
		return errcode.Newf(errcode.ErrFieldTypeNotFound, "字段类型 '%s' 不存在", typ)
	}
	return nil
}

func (s *FieldService) checkCategoryExists(category string) *errcode.Error {
	if !s.dictCache.Exists("field_category", category) {
		return errcode.Newf(errcode.ErrFieldCategoryNotFound, "标签分类 '%s' 不存在", category)
	}
	return nil
}

// ---- 业务方法 ----

// List 字段列表（Cache-Aside：Redis → MySQL → 写 Redis）
func (s *FieldService) List(ctx context.Context, q *model.FieldListQuery) (*model.ListData, error) {
	if q.Page <= 0 {
		q.Page = s.pagCfg.DefaultPage
	}
	if q.PageSize <= 0 {
		q.PageSize = s.pagCfg.DefaultPageSize
	}
	if q.PageSize > s.pagCfg.MaxPageSize {
		q.PageSize = s.pagCfg.MaxPageSize
	}

	// 1. 查 Redis 缓存（Redis 挂了跳过，降级直查 MySQL）
	if cached, hit, err := s.fieldCache.GetList(ctx, q); err == nil && hit {
		return cached, nil
	}

	// 2. 查 MySQL
	items, total, err := s.fieldStore.List(ctx, q)
	if err != nil {
		slog.Error("service.字段列表查询失败", "error", err, "query", q)
		return nil, err
	}

	for i := range items {
		items[i].TypeLabel = s.dictCache.GetLabel("field_type", items[i].Type)
		items[i].CategoryLabel = s.dictCache.GetLabel("field_category", items[i].Category)
	}

	result := &model.ListData{
		Items:    items,
		Total:    total,
		Page:     q.Page,
		PageSize: q.PageSize,
	}

	// 3. 写 Redis 缓存（失败只记日志，不影响响应）
	s.fieldCache.SetList(ctx, q, result)

	return result, nil
}

// Create 创建字段
func (s *FieldService) Create(ctx context.Context, req *model.CreateFieldRequest) (int64, error) {
	// 业务校验：type/category 存在性
	if err := s.checkTypeExists(req.Type); err != nil {
		return 0, err
	}
	if err := s.checkCategoryExists(req.Category); err != nil {
		return 0, err
	}

	// 业务校验：name 唯一性（含软删除）
	exists, err := s.fieldStore.ExistsByName(ctx, req.Name)
	if err != nil {
		slog.Error("service.创建字段-检查唯一性失败", "error", err, "name", req.Name)
		return 0, fmt.Errorf("check name exists: %w", err)
	}
	if exists {
		return 0, errcode.Newf(errcode.ErrFieldNameExists, "字段标识 '%s' 已存在", req.Name)
	}

	// 写入
	id, err := s.fieldStore.Create(ctx, req)
	if err != nil {
		slog.Error("service.创建字段失败", "error", err, "name", req.Name)
		return 0, fmt.Errorf("create field: %w", err)
	}

	// 清缓存
	s.fieldCache.InvalidateList(ctx)

	slog.Info("service.创建字段成功", "name", req.Name, "id", id)
	return id, nil
}

// GetByName 查询字段详情（Cache-Aside：Redis → MySQL → 写 Redis）
func (s *FieldService) GetByName(ctx context.Context, name string) (*model.Field, error) {
	// 1. 查 Redis 缓存
	if cached, hit, err := s.fieldCache.GetDetail(ctx, name); err == nil && hit {
		if cached == nil {
			return nil, errcode.Newf(errcode.ErrFieldRefNotFound, "字段 '%s' 不存在", name)
		}
		return cached, nil
	}

	// 2. 查 MySQL
	field, err := s.fieldStore.GetByName(ctx, name)
	if err != nil {
		slog.Error("service.查询字段详情失败", "error", err, "name", name)
		return nil, fmt.Errorf("get field: %w", err)
	}

	// 3. 写 Redis（field 为 nil 时也缓存，防穿透）
	s.fieldCache.SetDetail(ctx, name, field)

	if field == nil {
		return nil, errcode.Newf(errcode.ErrFieldRefNotFound, "字段 '%s' 不存在", name)
	}
	return field, nil
}

// Update 编辑字段
func (s *FieldService) Update(ctx context.Context, name string, req *model.UpdateFieldRequest) error {
	// 业务校验：type/category 存在性
	if err := s.checkTypeExists(req.Type); err != nil {
		return err
	}
	if err := s.checkCategoryExists(req.Category); err != nil {
		return err
	}

	// 查旧数据
	old, err := s.fieldStore.GetByName(ctx, name)
	if err != nil {
		slog.Error("service.编辑字段-查旧数据失败", "error", err, "name", name)
		return fmt.Errorf("get old field: %w", err)
	}
	if old == nil {
		return errcode.Newf(errcode.ErrFieldRefNotFound, "字段 '%s' 不存在", name)
	}

	// 硬约束：被引用时禁止改 type
	if old.Type != req.Type && old.RefCount > 0 {
		return errcode.Newf(errcode.ErrFieldRefChangeType, "该字段已被 %d 个模板/字段引用，无法修改类型", old.RefCount)
	}

	// 乐观锁写入
	err = s.fieldStore.Update(ctx, name, req)
	if err != nil {
		if errors.Is(err, storemysql.ErrVersionConflict) {
			return errcode.New(errcode.ErrFieldVersionConflict)
		}
		slog.Error("service.编辑字段失败", "error", err, "name", name)
		return fmt.Errorf("update field: %w", err)
	}

	// 清缓存
	s.fieldCache.DelDetail(ctx, name)
	s.fieldCache.InvalidateList(ctx)

	slog.Info("service.编辑字段成功", "name", name)
	return nil
}

// DeleteResult 删除结果
type DeleteResult struct {
	Deleted    bool             `json:"deleted"`
	References []model.FieldRef `json:"references,omitempty"`
}

// Delete 删除字段（硬约束：被引用时禁止删除）
func (s *FieldService) Delete(ctx context.Context, name string) (*DeleteResult, error) {
	field, err := s.fieldStore.GetByName(ctx, name)
	if err != nil {
		slog.Error("service.删除字段-查询失败", "error", err, "name", name)
		return nil, fmt.Errorf("get field: %w", err)
	}
	if field == nil {
		return nil, errcode.Newf(errcode.ErrFieldRefNotFound, "字段 '%s' 不存在", name)
	}

	refs, err := s.fieldRefStore.GetByFieldName(ctx, name)
	if err != nil {
		slog.Error("service.删除字段-查引用失败", "error", err, "name", name)
		return nil, fmt.Errorf("get refs: %w", err)
	}

	if len(refs) > 0 {
		return &DeleteResult{Deleted: false, References: refs}, errcode.New(errcode.ErrFieldRefDelete)
	}

	if err := s.fieldStore.SoftDelete(ctx, name); err != nil {
		if errors.Is(err, storemysql.ErrNotFound) {
			return nil, errcode.Newf(errcode.ErrFieldRefNotFound, "字段 '%s' 不存在", name)
		}
		slog.Error("service.删除字段失败", "error", err, "name", name)
		return nil, fmt.Errorf("soft delete: %w", err)
	}

	// 清缓存
	s.fieldCache.DelDetail(ctx, name)
	s.fieldCache.InvalidateList(ctx)

	slog.Info("service.删除字段成功", "name", name)
	return &DeleteResult{Deleted: true}, nil
}

// CheckName 校验字段标识是否可用
func (s *FieldService) CheckName(ctx context.Context, name string) (*model.CheckNameResult, error) {
	exists, err := s.fieldStore.ExistsByName(ctx, name)
	if err != nil {
		slog.Error("service.校验字段名失败", "error", err, "name", name)
		return nil, fmt.Errorf("check name: %w", err)
	}
	if exists {
		return &model.CheckNameResult{Available: false, Message: "该字段标识已存在"}, nil
	}
	return &model.CheckNameResult{Available: true, Message: "该标识可用"}, nil
}

// GetReferences 查询字段引用详情
func (s *FieldService) GetReferences(ctx context.Context, name string) (*model.ReferenceDetail, error) {
	field, err := s.fieldStore.GetByName(ctx, name)
	if err != nil {
		slog.Error("service.引用详情-查字段失败", "error", err, "name", name)
		return nil, fmt.Errorf("get field: %w", err)
	}
	if field == nil {
		return nil, errcode.Newf(errcode.ErrFieldRefNotFound, "字段 '%s' 不存在", name)
	}

	refs, err := s.fieldRefStore.GetByFieldName(ctx, name)
	if err != nil {
		slog.Error("service.引用详情-查引用失败", "error", err, "name", name)
		return nil, fmt.Errorf("get refs: %w", err)
	}

	templateNames := make([]string, 0)
	fieldNames := make([]string, 0)
	for _, r := range refs {
		switch r.RefType {
		case "template":
			templateNames = append(templateNames, r.RefName)
		case "field":
			fieldNames = append(fieldNames, r.RefName)
		}
	}

	result := &model.ReferenceDetail{
		FieldName:  name,
		FieldLabel: field.Label,
		Templates:  make([]model.ReferenceItem, 0, len(templateNames)),
		Fields:     make([]model.ReferenceItem, 0, len(fieldNames)),
	}

	if len(fieldNames) > 0 {
		fieldList, err := s.fieldStore.GetByNames(ctx, fieldNames)
		if err != nil {
			slog.Error("service.引用详情-查字段label失败", "error", err)
			return nil, fmt.Errorf("get field labels: %w", err)
		}
		labelMap := make(map[string]string, len(fieldList))
		for _, f := range fieldList {
			labelMap[f.Name] = f.Label
		}
		for _, n := range fieldNames {
			result.Fields = append(result.Fields, model.ReferenceItem{
				RefType: "field",
				RefName: n,
				Label:   labelMap[n],
			})
		}
	}

	for _, n := range templateNames {
		result.Templates = append(result.Templates, model.ReferenceItem{
			RefType: "template",
			RefName: n,
			Label:   n, // TODO: 模板管理完成后 IN 查 templates 拿 label
		})
	}

	return result, nil
}

// BatchDelete 批量删除
func (s *FieldService) BatchDelete(ctx context.Context, names []string) (*model.BatchDeleteResult, error) {
	fields, err := s.fieldStore.GetByNames(ctx, names)
	if err != nil {
		slog.Error("service.批量删除-查字段失败", "error", err)
		return nil, fmt.Errorf("get fields: %w", err)
	}
	labelMap := make(map[string]string, len(fields))
	for _, f := range fields {
		labelMap[f.Name] = f.Label
	}

	deleted := make([]string, 0)
	skipped := make([]model.BatchDeleteSkipped, 0)

	for _, name := range names {
		hasRefs, err := s.fieldRefStore.HasRefs(ctx, name)
		if err != nil {
			slog.Error("service.批量删除-查引用失败", "error", err, "name", name)
			skipped = append(skipped, model.BatchDeleteSkipped{Name: name, Label: labelMap[name], Reason: "查询引用失败"})
			continue
		}
		if hasRefs {
			skipped = append(skipped, model.BatchDeleteSkipped{Name: name, Label: labelMap[name], Reason: "被引用无法删除"})
			continue
		}
		if err := s.fieldStore.SoftDelete(ctx, name); err != nil {
			slog.Error("service.批量删除-删除失败", "error", err, "name", name)
			skipped = append(skipped, model.BatchDeleteSkipped{Name: name, Label: labelMap[name], Reason: "删除失败"})
			continue
		}
		deleted = append(deleted, name)
	}

	msg := fmt.Sprintf("%d 项已删除", len(deleted))
	if len(skipped) > 0 {
		msg += fmt.Sprintf("，%d 项因被引用无法删除", len(skipped))
	}

	// 清缓存（有删除才清）
	if len(deleted) > 0 {
		for _, name := range deleted {
			s.fieldCache.DelDetail(ctx, name)
		}
		s.fieldCache.InvalidateList(ctx)
	}

	slog.Info("service.批量删除完成", "deleted", len(deleted), "skipped", len(skipped))
	return &model.BatchDeleteResult{Deleted: deleted, Skipped: skipped, Message: msg}, nil
}

// BatchUpdateCategory 批量修改分类
func (s *FieldService) BatchUpdateCategory(ctx context.Context, req *model.BatchCategoryRequest) (int64, error) {
	// 业务校验：分类存在性
	if err := s.checkCategoryExists(req.Category); err != nil {
		return 0, err
	}

	affected, err := s.fieldStore.BatchUpdateCategory(ctx, req.Names, req.Category)
	if err != nil {
		slog.Error("service.批量修改分类失败", "error", err)
		return 0, fmt.Errorf("batch update category: %w", err)
	}

	// 清缓存
	s.fieldCache.InvalidateList(ctx)

	slog.Info("service.批量修改分类成功", "affected", affected, "category", req.Category)
	return affected, nil
}
