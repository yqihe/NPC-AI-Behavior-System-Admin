package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/yqihe/npc-ai-admin/backend/internal/cache"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	"github.com/yqihe/npc-ai-admin/backend/internal/store/mysql"
	"github.com/yqihe/npc-ai-admin/backend/internal/validator"
)

// FieldService 字段管理业务逻辑
type FieldService struct {
	fieldStore *mysql.FieldStore
	dictCache  *cache.DictCache
	validator  *validator.FieldValidator
}

// NewFieldService 创建 FieldService
func NewFieldService(fieldStore *mysql.FieldStore, dictCache *cache.DictCache, v *validator.FieldValidator) *FieldService {
	return &FieldService{
		fieldStore: fieldStore,
		dictCache:  dictCache,
		validator:  v,
	}
}

// List 字段列表（分页 + 筛选 + label 翻译）
func (s *FieldService) List(ctx context.Context, q *model.FieldListQuery) (*model.ListData, error) {
	// 参数默认值
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 20
	}
	if q.PageSize > 100 {
		q.PageSize = 100
	}

	items, total, err := s.fieldStore.List(ctx, q)
	if err != nil {
		slog.Error("service.字段列表查询失败", "error", err, "query", q)
		return nil, err
	}

	// 内存 map 翻译 type/category label
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

// ServiceError 业务错误（携带错误码）
type ServiceError struct {
	Code    int
	Message string
}

func (e *ServiceError) Error() string {
	return e.Message
}

// Create 创建字段
func (s *FieldService) Create(ctx context.Context, req *model.CreateFieldRequest) (int64, error) {
	// 1. 参数校验
	if code, msg := s.validator.ValidateCreate(req); code != 0 {
		return 0, &ServiceError{Code: code, Message: msg}
	}

	// 2. 唯一性检查（含软删除）
	exists, err := s.fieldStore.ExistsByName(ctx, req.Name)
	if err != nil {
		slog.Error("service.创建字段-检查唯一性失败", "error", err, "name", req.Name)
		return 0, fmt.Errorf("check name exists: %w", err)
	}
	if exists {
		return 0, &ServiceError{Code: 40001, Message: fmt.Sprintf("字段标识 '%s' 已存在", req.Name)}
	}

	// 3. 写入
	id, err := s.fieldStore.Create(ctx, req)
	if err != nil {
		slog.Error("service.创建字段失败", "error", err, "name", req.Name)
		return 0, fmt.Errorf("create field: %w", err)
	}

	slog.Info("service.创建字段成功", "name", req.Name, "id", id)
	return id, nil
}
