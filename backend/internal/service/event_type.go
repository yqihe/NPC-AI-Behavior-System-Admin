package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/npc-admin/backend/internal/cache"
	"github.com/npc-admin/backend/internal/model"
	"github.com/npc-admin/backend/internal/store"
	"github.com/npc-admin/backend/internal/validator"
)

const eventTypesCollection = "event_types"

// EventTypeService 编排 store + cache + validator。
type EventTypeService struct {
	store store.Store
	cache cache.Cache
}

// NewEventTypeService 创建事件类型服务。
func NewEventTypeService(s store.Store, c cache.Cache) *EventTypeService {
	return &EventTypeService{store: s, cache: c}
}

func (s *EventTypeService) List(ctx context.Context) ([]model.Document, error) {
	// 尝试从缓存读取
	docs, err := s.cache.GetList(ctx, eventTypesCollection)
	if err == nil {
		return docs, nil
	}
	if !errors.Is(err, cache.ErrCacheMiss) {
		slog.Warn("service.event_type.cache_get_error", "err", err)
	}

	// 回源 MongoDB
	docs, err = s.store.List(ctx, eventTypesCollection)
	if err != nil {
		return nil, err
	}

	// 写入缓存（失败不影响返回）
	if err := s.cache.SetList(ctx, eventTypesCollection, docs); err != nil {
		slog.Warn("service.event_type.cache_set_error", "err", err)
	}

	return docs, nil
}

func (s *EventTypeService) Get(ctx context.Context, name string) (model.Document, error) {
	return s.store.Get(ctx, eventTypesCollection, name)
}

func (s *EventTypeService) Create(ctx context.Context, doc model.Document) error {
	if err := validator.ValidateEventType(json.RawMessage(doc.Config)); err != nil {
		return err
	}
	if err := s.store.Create(ctx, eventTypesCollection, doc); err != nil {
		return err
	}
	s.invalidateCache(ctx)
	return nil
}

func (s *EventTypeService) Update(ctx context.Context, name string, doc model.Document) error {
	if err := validator.ValidateEventType(json.RawMessage(doc.Config)); err != nil {
		return err
	}
	if err := s.store.Update(ctx, eventTypesCollection, name, doc); err != nil {
		return err
	}
	s.invalidateCache(ctx)
	return nil
}

func (s *EventTypeService) Delete(ctx context.Context, name string) error {
	if err := s.store.Delete(ctx, eventTypesCollection, name); err != nil {
		return err
	}
	s.invalidateCache(ctx)
	return nil
}

func (s *EventTypeService) invalidateCache(ctx context.Context) {
	if err := s.cache.Invalidate(ctx, eventTypesCollection); err != nil {
		slog.Warn("service.event_type.cache_invalidate_error", "err", err)
	}
}
