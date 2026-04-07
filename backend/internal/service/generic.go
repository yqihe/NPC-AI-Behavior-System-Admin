package service

import (
	"context"
	"log/slog"

	"github.com/npc-admin/backend/internal/cache"
	"github.com/npc-admin/backend/internal/model"
	"github.com/npc-admin/backend/internal/store"
	"github.com/npc-admin/backend/internal/validator"
)

// GenericService 是通用的 CRUD 服务，适用于任意实体集合。
// 通过 collection 和可选的 SchemaValidator 参数化行为。
type GenericService struct {
	store      store.Store
	cache      cache.Cache
	collection string
	validator  *validator.SchemaValidator // 可选，nil 表示不校验
}

// NewGenericService 创建通用 CRUD 服务。
// validator 可为 nil，表示该集合不做 schema 校验。
func NewGenericService(s store.Store, c cache.Cache, collection string, v *validator.SchemaValidator) *GenericService {
	return &GenericService{
		store:      s,
		cache:      c,
		collection: collection,
		validator:  v,
	}
}

// List 返回集合中所有文档，优先从缓存获取。
func (s *GenericService) List(ctx context.Context) ([]model.Document, error) {
	// 尝试缓存
	docs, err := s.cache.GetList(ctx, s.collection)
	if err == nil {
		slog.Debug("service.list_cache_hit", "collection", s.collection)
		return docs, nil
	}

	// 缓存未命中，查数据库
	slog.Debug("service.list_cache_miss", "collection", s.collection)
	docs, err = s.store.List(ctx, s.collection)
	if err != nil {
		return nil, err
	}

	// 写缓存（失败不阻塞）
	if cacheErr := s.cache.SetList(ctx, s.collection, docs); cacheErr != nil {
		slog.Warn("service.list_cache_set_error", "collection", s.collection, "err", cacheErr)
	}

	return docs, nil
}

// Get 按名称获取单个文档。
func (s *GenericService) Get(ctx context.Context, name string) (model.Document, error) {
	return s.store.Get(ctx, s.collection, name)
}

// Create 创建文档，写入前执行 schema 校验（如果配置了 validator）。
func (s *GenericService) Create(ctx context.Context, doc model.Document) error {
	if err := s.validate(ctx, doc); err != nil {
		return err
	}

	if err := s.store.Create(ctx, s.collection, doc); err != nil {
		return err
	}

	s.invalidateCache(ctx)
	return nil
}

// Update 更新文档，写入前执行 schema 校验（如果配置了 validator）。
func (s *GenericService) Update(ctx context.Context, name string, doc model.Document) error {
	if err := s.validate(ctx, doc); err != nil {
		return err
	}

	if err := s.store.Update(ctx, s.collection, name, doc); err != nil {
		return err
	}

	s.invalidateCache(ctx)
	return nil
}

// Delete 删除文档。
func (s *GenericService) Delete(ctx context.Context, name string) error {
	if err := s.store.Delete(ctx, s.collection, name); err != nil {
		return err
	}

	s.invalidateCache(ctx)
	return nil
}

// validate 执行 schema 校验（如果 validator 不为 nil）。
func (s *GenericService) validate(ctx context.Context, doc model.Document) error {
	if s.validator == nil {
		return nil
	}
	return s.validator.ValidateAll(ctx, doc.Config)
}

// invalidateCache 清除该集合的列表缓存。
func (s *GenericService) invalidateCache(ctx context.Context) {
	if err := s.cache.Invalidate(ctx, s.collection); err != nil {
		slog.Warn("service.cache_invalidate_error", "collection", s.collection, "err", err)
	}
}
