package service

import (
	"context"

	"github.com/npc-admin/backend/internal/model"
	"github.com/npc-admin/backend/internal/store"
)

// ReadOnlyService 提供只读数据访问，不走缓存。
// 用于 component_schemas、npc_presets 等 ADMIN 元数据集合。
type ReadOnlyService struct {
	store      store.Store
	collection string
}

// NewReadOnlyService 创建只读服务。
func NewReadOnlyService(s store.Store, collection string) *ReadOnlyService {
	return &ReadOnlyService{store: s, collection: collection}
}

// List 返回集合中所有文档。
func (s *ReadOnlyService) List(ctx context.Context) ([]model.Document, error) {
	return s.store.List(ctx, s.collection)
}

// Get 按名称获取单个文档。
func (s *ReadOnlyService) Get(ctx context.Context, name string) (model.Document, error) {
	return s.store.Get(ctx, s.collection, name)
}
