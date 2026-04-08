package service

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/yqihe/npc-ai-admin/backend/internal/cache"
	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	"github.com/yqihe/npc-ai-admin/backend/internal/store/mysql"
	"github.com/yqihe/npc-ai-admin/backend/internal/validator"
)

// FieldService 字段管理业务逻辑
type FieldService struct {
	fieldStore *mysql.FieldStore
	dictCache  *cache.DictCache
	validator  *validator.FieldValidator
	pagCfg     *config.PaginationConfig
}

// NewFieldService 创建 FieldService
func NewFieldService(fieldStore *mysql.FieldStore, dictCache *cache.DictCache, v *validator.FieldValidator, pagCfg *config.PaginationConfig) *FieldService {
	return &FieldService{
		fieldStore: fieldStore,
		dictCache:  dictCache,
		validator:  v,
		pagCfg:     pagCfg,
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
