package service

import (
	"context"
	"log/slog"

	"github.com/yqihe/npc-ai-admin/backend/internal/cache"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	"github.com/yqihe/npc-ai-admin/backend/internal/store/mysql"
)

// FieldService 字段管理业务逻辑
type FieldService struct {
	fieldStore *mysql.FieldStore
	dictCache  *cache.DictCache
}

// NewFieldService 创建 FieldService
func NewFieldService(fieldStore *mysql.FieldStore, dictCache *cache.DictCache) *FieldService {
	return &FieldService{
		fieldStore: fieldStore,
		dictCache:  dictCache,
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
