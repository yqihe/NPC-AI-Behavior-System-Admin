package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	shared "github.com/yqihe/npc-ai-admin/backend/internal/service/shared"
	storemysql "github.com/yqihe/npc-ai-admin/backend/internal/store/mysql"
	storeredis "github.com/yqihe/npc-ai-admin/backend/internal/store/redis"
	rcfg "github.com/yqihe/npc-ai-admin/backend/internal/store/redis/shared"
	"github.com/yqihe/npc-ai-admin/backend/internal/util"
)

// RegionService 区域业务逻辑
//
// 持有 TemplateService 依赖用于 T7 validateSpawnTable（本期 T6 注入不消费，
// T7 接入 CheckEnabledByNames/GetByNames 做 spawn_entry.template_ref 引用完整性校验）。
type RegionService struct {
	store           *storemysql.RegionStore
	cache           *storeredis.RegionCache
	templateService *TemplateService
	pagCfg          *config.PaginationConfig
}

// NewRegionService 创建 RegionService
func NewRegionService(
	store *storemysql.RegionStore,
	cache *storeredis.RegionCache,
	templateService *TemplateService,
	pagCfg *config.PaginationConfig,
) *RegionService {
	return &RegionService{
		store:           store,
		cache:           cache,
		templateService: templateService,
		pagCfg:          pagCfg,
	}
}

// ---- 内部辅助 ----

func (s *RegionService) getOrNotFound(ctx context.Context, id int64) (*model.Region, error) {
	r, err := s.store.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get region %d: %w", id, err)
	}
	if r == nil {
		return nil, errcode.Newf(errcode.ErrRegionNotFound, "区域 ID=%d 不存在", id)
	}
	return r, nil
}

// validateBasicShape 基础结构校验（T6 骨架级，T7 扩展 spawn_table 语义与 template_ref 引用）
//
// 本期只校验：
//  - region_type 在 ValidRegionTypes 白名单内
//  - spawn_table 是合法 JSON 数组（json.Valid 级别）
//
// T7 接入后追加：SpawnEntry 自洽性 + template_ref 引用完整性（走 templateService）。
func (s *RegionService) validateBasicShape(regionType string, spawnTable json.RawMessage) error {
	if !util.ValidRegionTypes[regionType] {
		return errcode.Newf(errcode.ErrRegionTypeInvalid, "区域类型 %q 不在允许枚举内", regionType)
	}
	if len(spawnTable) == 0 {
		return errcode.Newf(errcode.ErrRegionSpawnEntryInvalid, "spawn_table 不能为空（空数组请传 '[]'）")
	}
	if !json.Valid(spawnTable) {
		return errcode.Newf(errcode.ErrRegionSpawnEntryInvalid, "spawn_table 必须是合法 JSON")
	}
	// TODO(T7): validateSpawnTable — SpawnEntry 自洽性 + template_ref 引用完整性
	return nil
}

// ---- CRUD ----

// List 分页列表
func (s *RegionService) List(ctx context.Context, q *model.RegionListQuery) (*model.ListData, error) {
	shared.NormalizePagination(&q.Page, &q.PageSize, s.pagCfg.DefaultPage, s.pagCfg.DefaultPageSize, s.pagCfg.MaxPageSize)

	// 查缓存
	if cached, hit, err := s.cache.GetList(ctx, q); err == nil && hit {
		slog.Debug("service.区域列表.缓存命中")
		return cached.ToListData(), nil
	}

	// 查 MySQL
	items, total, err := s.store.List(ctx, q)
	if err != nil {
		return nil, err
	}

	// 写缓存
	listData := &model.RegionListData{
		Items:    items,
		Total:    total,
		Page:     q.Page,
		PageSize: q.PageSize,
	}
	s.cache.SetList(ctx, q, listData)

	return listData.ToListData(), nil
}

// GetByID 查详情（Cache-Aside + 分布式锁 + 空标记）
func (s *RegionService) GetByID(ctx context.Context, id int64) (*model.Region, error) {
	// 1. 查缓存
	if cached, hit, err := s.cache.GetDetail(ctx, id); err == nil && hit {
		if cached == nil {
			return nil, errcode.Newf(errcode.ErrRegionNotFound, "区域 ID=%d 不存在", id)
		}
		return cached, nil
	}

	// 2. 分布式锁防击穿
	lockID, lockErr := s.cache.TryLock(ctx, id, rcfg.LockExpire)
	if lockErr != nil {
		slog.Warn("service.获取区域锁失败，降级直查MySQL", "error", lockErr, "id", id)
	}
	if lockID != "" {
		defer s.cache.Unlock(ctx, id, lockID)
		// double-check
		if cached, hit, err := s.cache.GetDetail(ctx, id); err == nil && hit {
			if cached == nil {
				return nil, errcode.Newf(errcode.ErrRegionNotFound, "区域 ID=%d 不存在", id)
			}
			return cached, nil
		}
	}

	// 3. 查 MySQL
	r, err := s.store.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get region: %w", err)
	}

	// 4. 写缓存（含空标记）
	s.cache.SetDetail(ctx, id, r)

	if r == nil {
		return nil, errcode.Newf(errcode.ErrRegionNotFound, "区域 ID=%d 不存在", id)
	}
	return r, nil
}

// Create 创建区域
func (s *RegionService) Create(ctx context.Context, req *model.CreateRegionRequest) (*model.CreateRegionResponse, error) {
	slog.Debug("service.创建区域", "region_id", req.RegionID)

	if err := s.validateBasicShape(req.RegionType, req.SpawnTable); err != nil {
		return nil, err
	}

	// region_id 唯一性（含软删除）
	exists, err := s.store.ExistsByRegionID(ctx, req.RegionID)
	if err != nil {
		slog.Error("service.创建区域-检查唯一性失败", "error", err, "region_id", req.RegionID)
		return nil, fmt.Errorf("check region_id exists: %w", err)
	}
	if exists {
		return nil, errcode.Newf(errcode.ErrRegionIDExists, "区域标识 '%s' 已存在", req.RegionID)
	}

	id, err := s.store.Create(ctx, req)
	if err != nil {
		if errors.Is(err, errcode.ErrDuplicate) {
			return nil, errcode.Newf(errcode.ErrRegionIDExists, "区域标识 '%s' 已存在", req.RegionID)
		}
		slog.Error("service.创建区域失败", "error", err, "region_id", req.RegionID)
		return nil, fmt.Errorf("create region: %w", err)
	}

	s.cache.InvalidateList(ctx)
	slog.Info("service.创建区域成功", "id", id, "region_id", req.RegionID)
	return &model.CreateRegionResponse{ID: id, RegionID: req.RegionID}, nil
}

// Update 编辑区域（启用中禁止）
func (s *RegionService) Update(ctx context.Context, req *model.UpdateRegionRequest) error {
	slog.Debug("service.编辑区域", "id", req.ID)

	r, err := s.getOrNotFound(ctx, req.ID)
	if err != nil {
		return err
	}

	if r.Enabled {
		return errcode.New(errcode.ErrRegionEditNotDisabled)
	}

	if err := s.validateBasicShape(req.RegionType, req.SpawnTable); err != nil {
		return err
	}

	if err := s.store.Update(ctx, req); err != nil {
		if errors.Is(err, errcode.ErrVersionConflict) {
			return errcode.New(errcode.ErrRegionVersionConflict)
		}
		slog.Error("service.编辑区域失败", "error", err, "id", req.ID)
		return fmt.Errorf("update region: %w", err)
	}

	s.cache.DelDetail(ctx, req.ID)
	s.cache.InvalidateList(ctx)
	slog.Info("service.编辑区域成功", "id", req.ID)
	return nil
}

// SoftDelete 软删除区域（启用中禁止）
func (s *RegionService) SoftDelete(ctx context.Context, id int64) error {
	slog.Debug("service.删除区域", "id", id)

	r, err := s.getOrNotFound(ctx, id)
	if err != nil {
		return err
	}

	if r.Enabled {
		return errcode.New(errcode.ErrRegionDeleteNotDisabled)
	}

	if err := s.store.SoftDelete(ctx, id); err != nil {
		if errors.Is(err, errcode.ErrNotFound) {
			return errcode.Newf(errcode.ErrRegionNotFound, "区域 ID=%d 不存在", id)
		}
		slog.Error("service.删除区域失败", "error", err, "id", id)
		return fmt.Errorf("soft delete region: %w", err)
	}

	s.cache.DelDetail(ctx, id)
	s.cache.InvalidateList(ctx)
	slog.Info("service.删除区域成功", "id", id, "region_id", r.RegionID)
	return nil
}

// ToggleEnabled 切换启用/停用
func (s *RegionService) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) error {
	slog.Debug("service.切换区域启用", "id", req.ID)

	if _, err := s.getOrNotFound(ctx, req.ID); err != nil {
		return err
	}

	if err := s.store.ToggleEnabled(ctx, req); err != nil {
		if errors.Is(err, errcode.ErrVersionConflict) {
			return errcode.New(errcode.ErrRegionVersionConflict)
		}
		slog.Error("service.切换区域启用失败", "error", err, "id", req.ID)
		return fmt.Errorf("toggle region enabled: %w", err)
	}

	s.cache.DelDetail(ctx, req.ID)
	s.cache.InvalidateList(ctx)
	slog.Info("service.切换区域启用成功", "id", req.ID, "enabled", req.Enabled)
	return nil
}
