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
	"github.com/yqihe/npc-ai-admin/backend/internal/validator"
)

// FieldService 字段管理业务逻辑
type FieldService struct {
	fieldStore    *storemysql.FieldStore
	fieldRefStore *storemysql.FieldRefStore
	dictCache     *cache.DictCache
	validator     *validator.FieldValidator
	pagCfg        *config.PaginationConfig
}

// NewFieldService 创建 FieldService
func NewFieldService(fieldStore *storemysql.FieldStore, fieldRefStore *storemysql.FieldRefStore, dictCache *cache.DictCache, v *validator.FieldValidator, pagCfg *config.PaginationConfig) *FieldService {
	return &FieldService{
		fieldStore:    fieldStore,
		fieldRefStore: fieldRefStore,
		dictCache:     dictCache,
		validator:     v,
		pagCfg:        pagCfg,
	}
}

// List 字段列表（分页 + 筛选 + label 翻译）
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

	items, total, err := s.fieldStore.List(ctx, q)
	if err != nil {
		slog.Error("service.字段列表查询失败", "error", err, "query", q)
		return nil, err
	}

	for i := range items {
		items[i].TypeLabel = s.dictCache.GetLabel("field_type", items[i].Type)
		items[i].CategoryLabel = s.dictCache.GetLabel("field_category", items[i].Category)
	}

	return &model.ListData{
		Items:    items,
		Total:    total,
		Page:     q.Page,
		PageSize: q.PageSize,
	}, nil
}

// Create 创建字段
func (s *FieldService) Create(ctx context.Context, req *model.CreateFieldRequest) (int64, error) {
	if err := s.validator.ValidateCreate(req); err != nil {
		return 0, err
	}

	exists, err := s.fieldStore.ExistsByName(ctx, req.Name)
	if err != nil {
		slog.Error("service.创建字段-检查唯一性失败", "error", err, "name", req.Name)
		return 0, fmt.Errorf("check name exists: %w", err)
	}
	if exists {
		return 0, errcode.Newf(errcode.ErrFieldNameExists, "字段标识 '%s' 已存在", req.Name)
	}

	id, err := s.fieldStore.Create(ctx, req)
	if err != nil {
		slog.Error("service.创建字段失败", "error", err, "name", req.Name)
		return 0, fmt.Errorf("create field: %w", err)
	}

	slog.Info("service.创建字段成功", "name", req.Name, "id", id)
	return id, nil
}

// GetByName 查询字段详情
func (s *FieldService) GetByName(ctx context.Context, name string) (*model.Field, error) {
	field, err := s.fieldStore.GetByName(ctx, name)
	if err != nil {
		slog.Error("service.查询字段详情失败", "error", err, "name", name)
		return nil, fmt.Errorf("get field: %w", err)
	}
	if field == nil {
		return nil, errcode.Newf(errcode.ErrFieldRefNotFound, "字段 '%s' 不存在", name)
	}
	return field, nil
}

// Update 编辑字段（含硬约束检查）
func (s *FieldService) Update(ctx context.Context, name string, req *model.UpdateFieldRequest) error {
	// 1. 参数校验
	if err := s.validator.ValidateUpdate(req); err != nil {
		return err
	}

	// 2. 查旧数据
	old, err := s.fieldStore.GetByName(ctx, name)
	if err != nil {
		slog.Error("service.编辑字段-查旧数据失败", "error", err, "name", name)
		return fmt.Errorf("get old field: %w", err)
	}
	if old == nil {
		return errcode.Newf(errcode.ErrFieldRefNotFound, "字段 '%s' 不存在", name)
	}

	// 3. 硬约束检查
	if old.Type != req.Type && old.RefCount > 0 {
		return errcode.Newf(errcode.ErrFieldRefChangeType, "该字段已被 %d 个模板/字段引用，无法修改类型", old.RefCount)
	}

	// 4. 乐观锁写入
	err = s.fieldStore.Update(ctx, name, req)
	if err != nil {
		if errors.Is(err, storemysql.ErrVersionConflict) {
			return errcode.New(errcode.ErrFieldVersionConflict)
		}
		slog.Error("service.编辑字段失败", "error", err, "name", name)
		return fmt.Errorf("update field: %w", err)
	}

	slog.Info("service.编辑字段成功", "name", name)
	return nil
}

// DeleteResult 删除结果
type DeleteResult struct {
	Deleted    bool            `json:"deleted"`
	References []model.FieldRef `json:"references,omitempty"`
}

// Delete 删除字段（硬约束：被引用时禁止删除）
func (s *FieldService) Delete(ctx context.Context, name string) (*DeleteResult, error) {
	// 1. 检查字段是否存在
	field, err := s.fieldStore.GetByName(ctx, name)
	if err != nil {
		slog.Error("service.删除字段-查询失败", "error", err, "name", name)
		return nil, fmt.Errorf("get field: %w", err)
	}
	if field == nil {
		return nil, errcode.Newf(errcode.ErrFieldRefNotFound, "字段 '%s' 不存在", name)
	}

	// 2. 查引用关系
	refs, err := s.fieldRefStore.GetByFieldName(ctx, name)
	if err != nil {
		slog.Error("service.删除字段-查引用失败", "error", err, "name", name)
		return nil, fmt.Errorf("get refs: %w", err)
	}

	// 3. 有引用 → 禁止删除，返回引用列表
	if len(refs) > 0 {
		return &DeleteResult{Deleted: false, References: refs}, errcode.New(errcode.ErrFieldRefDelete)
	}

	// 4. 无引用 → 软删除
	if err := s.fieldStore.SoftDelete(ctx, name); err != nil {
		if errors.Is(err, storemysql.ErrNotFound) {
			return nil, errcode.Newf(errcode.ErrFieldRefNotFound, "字段 '%s' 不存在", name)
		}
		slog.Error("service.删除字段失败", "error", err, "name", name)
		return nil, fmt.Errorf("soft delete: %w", err)
	}

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

// GetReferences 查询字段引用详情（不 JOIN，两次独立查询）
func (s *FieldService) GetReferences(ctx context.Context, name string) (*model.ReferenceDetail, error) {
	// 1. 检查字段存在
	field, err := s.fieldStore.GetByName(ctx, name)
	if err != nil {
		slog.Error("service.引用详情-查字段失败", "error", err, "name", name)
		return nil, fmt.Errorf("get field: %w", err)
	}
	if field == nil {
		return nil, errcode.Newf(errcode.ErrFieldRefNotFound, "字段 '%s' 不存在", name)
	}

	// 2. 查引用关系（主键索引前缀）
	refs, err := s.fieldRefStore.GetByFieldName(ctx, name)
	if err != nil {
		slog.Error("service.引用详情-查引用失败", "error", err, "name", name)
		return nil, fmt.Errorf("get refs: %w", err)
	}

	// 3. 按 ref_type 分组，收集 ref_name
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

	// 4. IN 查 fields 拿 label（走 uk_name）
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

	// 5. 模板引用 — 模板表未建，暂时只返回 ref_name
	for _, n := range templateNames {
		result.Templates = append(result.Templates, model.ReferenceItem{
			RefType: "template",
			RefName: n,
			Label:   n, // TODO: 模板管理完成后 IN 查 templates 拿 label
		})
	}

	return result, nil
}
