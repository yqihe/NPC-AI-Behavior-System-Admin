package service

import (
	shared "github.com/yqihe/npc-ai-admin/backend/internal/service/shared"
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/yqihe/npc-ai-admin/backend/internal/cache"
	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	storemysql "github.com/yqihe/npc-ai-admin/backend/internal/store/mysql"
	storeredis "github.com/yqihe/npc-ai-admin/backend/internal/store/redis"
	rcfg "github.com/yqihe/npc-ai-admin/backend/internal/store/redis/shared"
	"github.com/yqihe/npc-ai-admin/backend/internal/util"
)

// FsmStateDictService 状态字典业务逻辑
type FsmStateDictService struct {
	store          *storemysql.FsmStateDictStore
	fsmConfigStore *storemysql.FsmConfigStore
	cache          *storeredis.FsmStateDictCache
	dictCache      *cache.DictCache
	pagCfg         *config.PaginationConfig
	dictCfg        *config.FsmStateDictConfig
}

// NewFsmStateDictService 创建 FsmStateDictService
func NewFsmStateDictService(
	store *storemysql.FsmStateDictStore,
	fsmConfigStore *storemysql.FsmConfigStore,
	redisCache *storeredis.FsmStateDictCache,
	dictCache *cache.DictCache,
	pagCfg *config.PaginationConfig,
	dictCfg *config.FsmStateDictConfig,
) *FsmStateDictService {
	return &FsmStateDictService{
		store:          store,
		fsmConfigStore: fsmConfigStore,
		cache:          redisCache,
		dictCache:      dictCache,
		pagCfg:         pagCfg,
		dictCfg:        dictCfg,
	}
}

// ---- 辅助方法 ----

func (s *FsmStateDictService) getOrNotFound(ctx context.Context, id int64) (*model.FsmStateDict, error) {
	d, err := s.store.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get fsm_state_dict %d: %w", id, err)
	}
	if d == nil {
		return nil, errcode.Newf(errcode.ErrFsmStateDictNotFound, "状态字典 ID=%d 不存在", id)
	}
	return d, nil
}

// ---- CRUD ----

// List 分页列表
func (s *FsmStateDictService) List(ctx context.Context, q *model.FsmStateDictListQuery) (*model.ListData, error) {
	shared.NormalizePagination(&q.Page, &q.PageSize, s.pagCfg.DefaultPage, s.pagCfg.DefaultPageSize, s.pagCfg.MaxPageSize)

	// 查缓存（Redis 挂了跳过，降级直查 MySQL）
	if cached, hit, err := s.cache.GetList(ctx, q); err == nil && hit {
		slog.Debug("service.状态字典列表.缓存命中")
		return cached.ToListData(), nil
	}

	// 查 MySQL
	items, total, err := s.store.List(ctx, q)
	if err != nil {
		return nil, err
	}

	// 翻译分类标签
	for i := range items {
		items[i].CategoryLabel = s.dictCache.GetLabel(util.DictGroupFsmStateCategory, items[i].Category)
	}

	// 写缓存
	listData := &model.FsmStateDictListData{
		Items:    items,
		Total:    total,
		Page:     q.Page,
		PageSize: q.PageSize,
	}
	s.cache.SetList(ctx, q, listData)

	return listData.ToListData(), nil
}

// GetByID 查详情（Cache-Aside + 分布式锁 + 空标记）
func (s *FsmStateDictService) GetByID(ctx context.Context, id int64) (*model.FsmStateDict, error) {
	// 1. 查缓存
	if cached, hit, err := s.cache.GetDetail(ctx, id); err == nil && hit {
		if cached == nil {
			return nil, errcode.New(errcode.ErrFsmStateDictNotFound)
		}
		return cached, nil
	}

	// 2. 分布式锁防击穿
	lockID, lockErr := s.cache.TryLock(ctx, id, rcfg.LockExpire)
	if lockErr != nil {
		slog.Warn("service.获取锁失败，降级直查MySQL", "error", lockErr, "id", id)
	}
	if lockID != "" {
		defer s.cache.Unlock(ctx, id, lockID)
		// double-check
		if cached, hit, err := s.cache.GetDetail(ctx, id); err == nil && hit {
			if cached == nil {
				return nil, errcode.New(errcode.ErrFsmStateDictNotFound)
			}
			return cached, nil
		}
	}

	// 3. 查 MySQL
	d, err := s.store.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 4. 写缓存（含空标记）
	s.cache.SetDetail(ctx, id, d)

	if d == nil {
		return nil, errcode.New(errcode.ErrFsmStateDictNotFound)
	}
	return d, nil
}

// Create 创建状态字典条目
func (s *FsmStateDictService) Create(ctx context.Context, req *model.CreateFsmStateDictRequest) (int64, error) {
	slog.Debug("service.创建状态字典", "name", req.Name)

	// name 唯一性（含软删除）
	exists, err := s.store.ExistsByName(ctx, req.Name)
	if err != nil {
		slog.Error("service.创建状态字典-检查唯一性失败", "error", err, "name", req.Name)
		return 0, fmt.Errorf("check name exists: %w", err)
	}
	if exists {
		return 0, errcode.Newf(errcode.ErrFsmStateDictNameExists, "状态标识 '%s' 已存在", req.Name)
	}

	// 写 MySQL
	id, err := s.store.Create(ctx, req)
	if err != nil {
		if errors.Is(err, errcode.ErrDuplicate) {
			return 0, errcode.Newf(errcode.ErrFsmStateDictNameExists, "状态标识 '%s' 已存在", req.Name)
		}
		slog.Error("service.创建状态字典失败", "error", err, "name", req.Name)
		return 0, fmt.Errorf("create fsm_state_dict: %w", err)
	}

	// 清列表缓存
	s.cache.InvalidateList(ctx)

	slog.Info("service.创建状态字典成功", "id", id, "name", req.Name)
	return id, nil
}

// Update 编辑状态字典条目
func (s *FsmStateDictService) Update(ctx context.Context, req *model.UpdateFsmStateDictRequest) error {
	slog.Debug("service.编辑状态字典", "id", req.ID)

	if _, err := s.getOrNotFound(ctx, req.ID); err != nil {
		return err
	}

	// 乐观锁更新
	if err := s.store.Update(ctx, req); err != nil {
		if errors.Is(err, errcode.ErrVersionConflict) {
			return errcode.New(errcode.ErrFsmStateDictVersionConflict)
		}
		slog.Error("service.编辑状态字典失败", "error", err, "id", req.ID)
		return fmt.Errorf("update fsm_state_dict: %w", err)
	}

	// 清缓存
	s.cache.DelDetail(ctx, req.ID)
	s.cache.InvalidateList(ctx)

	slog.Info("service.编辑状态字典成功", "id", req.ID)
	return nil
}

// Delete 软删除状态字典条目
//
// 被 FSM 引用时返回 (*FsmStateDictDeleteResult{ReferencedBy: [...]}, ErrFsmStateDictInUse)，
// WrapCtx 会将 resp 作为 data 携带在错误响应中。
func (s *FsmStateDictService) Delete(ctx context.Context, id int64) (*model.FsmStateDictDeleteResult, error) {
	slog.Debug("service.删除状态字典", "id", id)

	d, err := s.getOrNotFound(ctx, id)
	if err != nil {
		return nil, err
	}

	// 启用中禁止删除
	if d.Enabled {
		return nil, errcode.New(errcode.ErrFsmStateDictDeleteNotDisabled)
	}

	// 引用检查：扫描 fsm_configs.config_json
	refs, err := s.fsmConfigStore.ListFsmConfigsReferencingState(ctx, d.Name, 20)
	if err != nil {
		slog.Error("service.删除状态字典-引用扫描失败", "error", err, "name", d.Name)
		return nil, fmt.Errorf("scan fsm refs: %w", err)
	}
	if len(refs) > 0 {
		slog.Info("service.删除状态字典-被引用拒绝", "name", d.Name, "ref_count", len(refs))
		return &model.FsmStateDictDeleteResult{ReferencedBy: refs}, errcode.New(errcode.ErrFsmStateDictInUse)
	}

	// 软删除
	if err := s.store.SoftDelete(ctx, id); err != nil {
		if errors.Is(err, errcode.ErrNotFound) {
			return nil, errcode.New(errcode.ErrFsmStateDictNotFound)
		}
		slog.Error("service.删除状态字典失败", "error", err, "id", id)
		return nil, fmt.Errorf("soft delete fsm_state_dict: %w", err)
	}

	// 清缓存
	s.cache.DelDetail(ctx, id)
	s.cache.InvalidateList(ctx)

	slog.Info("service.删除状态字典成功", "id", id, "name", d.Name)
	return &model.FsmStateDictDeleteResult{ID: id, Name: d.Name, DisplayName: d.DisplayName}, nil
}

// CheckName 唯一性校验
func (s *FsmStateDictService) CheckName(ctx context.Context, name string) (*model.CheckNameResult, error) {
	exists, err := s.store.ExistsByName(ctx, name)
	if err != nil {
		slog.Error("service.校验状态标识失败", "error", err, "name", name)
		return nil, fmt.Errorf("check name: %w", err)
	}
	if exists {
		return &model.CheckNameResult{Available: false, Message: "该状态标识已存在"}, nil
	}
	return &model.CheckNameResult{Available: true, Message: "该标识可用"}, nil
}

// ToggleEnabled 切换启用/停用
func (s *FsmStateDictService) ToggleEnabled(ctx context.Context, id int64, version int) error {
	slog.Debug("service.切换状态字典启用", "id", id)

	d, err := s.getOrNotFound(ctx, id)
	if err != nil {
		return err
	}

	newEnabled := !d.Enabled
	if err := s.store.ToggleEnabled(ctx, id, newEnabled, version); err != nil {
		if errors.Is(err, errcode.ErrVersionConflict) {
			return errcode.New(errcode.ErrFsmStateDictVersionConflict)
		}
		slog.Error("service.切换状态字典启用失败", "error", err, "id", id)
		return fmt.Errorf("toggle enabled: %w", err)
	}

	// 清缓存
	s.cache.DelDetail(ctx, id)
	s.cache.InvalidateList(ctx)

	slog.Info("service.切换状态字典启用成功", "id", id, "enabled", newEnabled)
	return nil
}


