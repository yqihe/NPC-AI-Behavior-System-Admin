package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/npc-admin/backend/internal/cache"
	"github.com/npc-admin/backend/internal/model"
	"github.com/npc-admin/backend/internal/store"
	"github.com/npc-admin/backend/internal/validator"
)

const btTreesCollection = "bt_trees"

type BtTreeService struct {
	store store.Store
	cache cache.Cache
}

func NewBtTreeService(s store.Store, c cache.Cache) *BtTreeService {
	return &BtTreeService{store: s, cache: c}
}

func (s *BtTreeService) List(ctx context.Context) ([]model.Document, error) {
	docs, err := s.cache.GetList(ctx, btTreesCollection)
	if err == nil {
		return docs, nil
	}
	if !errors.Is(err, cache.ErrCacheMiss) {
		slog.Warn("service.bt_tree.cache_get_error", "err", err)
	}
	docs, err = s.store.List(ctx, btTreesCollection)
	if err != nil {
		return nil, err
	}
	if err := s.cache.SetList(ctx, btTreesCollection, docs); err != nil {
		slog.Warn("service.bt_tree.cache_set_error", "err", err)
	}
	return docs, nil
}

func (s *BtTreeService) Get(ctx context.Context, name string) (model.Document, error) {
	return s.store.Get(ctx, btTreesCollection, name)
}

func (s *BtTreeService) Create(ctx context.Context, doc model.Document) error {
	if err := validator.ValidateBtTree(json.RawMessage(doc.Config)); err != nil {
		return err
	}
	if err := s.store.Create(ctx, btTreesCollection, doc); err != nil {
		return err
	}
	s.invalidateCache(ctx)
	return nil
}

func (s *BtTreeService) Update(ctx context.Context, name string, doc model.Document) error {
	if err := validator.ValidateBtTree(json.RawMessage(doc.Config)); err != nil {
		return err
	}
	if err := s.store.Update(ctx, btTreesCollection, name, doc); err != nil {
		return err
	}
	s.invalidateCache(ctx)
	return nil
}

func (s *BtTreeService) Delete(ctx context.Context, name string) error {
	// 删除前检查是否有 NPC 类型的 bt_refs 引用此 BT
	if err := s.checkBtRef(ctx, name); err != nil {
		return err
	}
	if err := s.store.Delete(ctx, btTreesCollection, name); err != nil {
		return err
	}
	s.invalidateCache(ctx)
	return nil
}

// checkBtRef 检查是否有 NPC 类型的 bt_refs 值指向此 BT 名称。
func (s *BtTreeService) checkBtRef(ctx context.Context, btName string) error {
	npcDocs, err := s.store.List(ctx, "npc_types")
	if err != nil {
		return fmt.Errorf("检查 BT 引用时出错: %w", err)
	}
	for _, doc := range npcDocs {
		var cfg struct {
			BtRefs map[string]string `json:"bt_refs"`
		}
		if err := json.Unmarshal(doc.Config, &cfg); err != nil {
			slog.Error("service.bt_tree.check_ref_unmarshal", "npc", doc.Name, "err", err)
			return fmt.Errorf("解析 NPC 类型 \"%s\" 配置时出错: %w", doc.Name, err)
		}
		for _, ref := range cfg.BtRefs {
			if ref == btName {
				return &validator.ValidationError{
					Errors: []string{fmt.Sprintf("该行为树正在被 NPC 类型 \"%s\" 引用，无法删除", doc.Name)},
				}
			}
		}
	}
	return nil
}

func (s *BtTreeService) invalidateCache(ctx context.Context) {
	if err := s.cache.Invalidate(ctx, btTreesCollection); err != nil {
		slog.Warn("service.bt_tree.cache_invalidate_error", "err", err)
	}
}
