package service

import (
	"context"
	"errors"
	"log/slog"

	"github.com/npc-admin/backend/internal/cache"
	"github.com/npc-admin/backend/internal/model"
	"github.com/npc-admin/backend/internal/store"
	"github.com/npc-admin/backend/internal/validator"
)

const npcTypesCollection = "npc_types"

type NpcTypeService struct {
	store store.Store
	cache cache.Cache
}

func NewNpcTypeService(s store.Store, c cache.Cache) *NpcTypeService {
	return &NpcTypeService{store: s, cache: c}
}

func (s *NpcTypeService) List(ctx context.Context) ([]model.Document, error) {
	docs, err := s.cache.GetList(ctx, npcTypesCollection)
	if err == nil {
		return docs, nil
	}
	if !errors.Is(err, cache.ErrCacheMiss) {
		slog.Warn("service.npc_type.cache_get_error", "err", err)
	}
	docs, err = s.store.List(ctx, npcTypesCollection)
	if err != nil {
		return nil, err
	}
	if err := s.cache.SetList(ctx, npcTypesCollection, docs); err != nil {
		slog.Warn("service.npc_type.cache_set_error", "err", err)
	}
	return docs, nil
}

func (s *NpcTypeService) Get(ctx context.Context, name string) (model.Document, error) {
	return s.store.Get(ctx, npcTypesCollection, name)
}

func (s *NpcTypeService) Create(ctx context.Context, doc model.Document) error {
	if err := validator.ValidateNpcType(doc.Config, s.store, ctx); err != nil {
		return err
	}
	if err := s.store.Create(ctx, npcTypesCollection, doc); err != nil {
		return err
	}
	s.invalidateCache(ctx)
	return nil
}

func (s *NpcTypeService) Update(ctx context.Context, name string, doc model.Document) error {
	if err := validator.ValidateNpcType(doc.Config, s.store, ctx); err != nil {
		return err
	}
	if err := s.store.Update(ctx, npcTypesCollection, name, doc); err != nil {
		return err
	}
	s.invalidateCache(ctx)
	return nil
}

func (s *NpcTypeService) Delete(ctx context.Context, name string) error {
	if err := s.store.Delete(ctx, npcTypesCollection, name); err != nil {
		return err
	}
	s.invalidateCache(ctx)
	return nil
}

func (s *NpcTypeService) invalidateCache(ctx context.Context) {
	if err := s.cache.Invalidate(ctx, npcTypesCollection); err != nil {
		slog.Warn("service.npc_type.cache_invalidate_error", "err", err)
	}
}
