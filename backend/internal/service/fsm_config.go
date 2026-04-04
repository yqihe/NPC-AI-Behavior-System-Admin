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

const fsmConfigsCollection = "fsm_configs"

type FsmConfigService struct {
	store store.Store
	cache cache.Cache
}

func NewFsmConfigService(s store.Store, c cache.Cache) *FsmConfigService {
	return &FsmConfigService{store: s, cache: c}
}

func (s *FsmConfigService) List(ctx context.Context) ([]model.Document, error) {
	docs, err := s.cache.GetList(ctx, fsmConfigsCollection)
	if err == nil {
		return docs, nil
	}
	if !errors.Is(err, cache.ErrCacheMiss) {
		slog.Warn("service.fsm_config.cache_get_error", "err", err)
	}
	docs, err = s.store.List(ctx, fsmConfigsCollection)
	if err != nil {
		return nil, err
	}
	if err := s.cache.SetList(ctx, fsmConfigsCollection, docs); err != nil {
		slog.Warn("service.fsm_config.cache_set_error", "err", err)
	}
	return docs, nil
}

func (s *FsmConfigService) Get(ctx context.Context, name string) (model.Document, error) {
	return s.store.Get(ctx, fsmConfigsCollection, name)
}

func (s *FsmConfigService) Create(ctx context.Context, doc model.Document) error {
	if err := validator.ValidateFsmConfig(json.RawMessage(doc.Config)); err != nil {
		return err
	}
	if err := s.store.Create(ctx, fsmConfigsCollection, doc); err != nil {
		return err
	}
	s.invalidateCache(ctx)
	return nil
}

func (s *FsmConfigService) Update(ctx context.Context, name string, doc model.Document) error {
	if err := validator.ValidateFsmConfig(json.RawMessage(doc.Config)); err != nil {
		return err
	}
	if err := s.store.Update(ctx, fsmConfigsCollection, name, doc); err != nil {
		return err
	}
	s.invalidateCache(ctx)
	return nil
}

func (s *FsmConfigService) Delete(ctx context.Context, name string) error {
	// 删除前检查是否有 NPC 类型引用此 FSM
	if err := s.checkFsmRef(ctx, name); err != nil {
		return err
	}
	if err := s.store.Delete(ctx, fsmConfigsCollection, name); err != nil {
		return err
	}
	s.invalidateCache(ctx)
	return nil
}

// checkFsmRef 检查是否有 NPC 类型的 fsm_ref 指向此 FSM 名称。
func (s *FsmConfigService) checkFsmRef(ctx context.Context, fsmName string) error {
	npcDocs, err := s.store.List(ctx, "npc_types")
	if err != nil {
		return fmt.Errorf("检查 FSM 引用时出错: %w", err)
	}
	for _, doc := range npcDocs {
		var cfg struct {
			FsmRef string `json:"fsm_ref"`
		}
		if err := json.Unmarshal(doc.Config, &cfg); err != nil {
			slog.Error("service.fsm_config.check_ref_unmarshal", "npc", doc.Name, "err", err)
			return fmt.Errorf("解析 NPC 类型 \"%s\" 配置时出错: %w", doc.Name, err)
		}
		if cfg.FsmRef == fsmName {
			return &validator.ValidationError{
				Errors: []string{fmt.Sprintf("该状态机正在被 NPC 类型 \"%s\" 引用，无法删除", doc.Name)},
			}
		}
	}
	return nil
}

func (s *FsmConfigService) invalidateCache(ctx context.Context) {
	if err := s.cache.Invalidate(ctx, fsmConfigsCollection); err != nil {
		slog.Warn("service.fsm_config.cache_invalidate_error", "err", err)
	}
}
