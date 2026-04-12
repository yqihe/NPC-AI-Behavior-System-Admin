package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/yqihe/npc-ai-admin/backend/internal/cache"
	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	"github.com/yqihe/npc-ai-admin/backend/internal/service/constraint"
	storemysql "github.com/yqihe/npc-ai-admin/backend/internal/store/mysql"
	storeredis "github.com/yqihe/npc-ai-admin/backend/internal/store/redis"
)

// EventTypeService 事件类型管理业务逻辑
//
// 只持有自身的 store/cache，不持有其他模块的 store/service。
// EventTypeSchemaCache 是内存缓存，语义上是"基础设施"（类似 DictCache），允许直接调用。
type EventTypeService struct {
	store       *storemysql.EventTypeStore
	cache       *storeredis.EventTypeCache
	schemaCache *cache.EventTypeSchemaCache
	pagCfg      *config.PaginationConfig
	etCfg       *config.EventTypeConfig
}

// NewEventTypeService 创建 EventTypeService
func NewEventTypeService(
	store *storemysql.EventTypeStore,
	cache *storeredis.EventTypeCache,
	schemaCache *cache.EventTypeSchemaCache,
	pagCfg *config.PaginationConfig,
	etCfg *config.EventTypeConfig,
) *EventTypeService {
	return &EventTypeService{
		store:       store,
		cache:       cache,
		schemaCache: schemaCache,
		pagCfg:      pagCfg,
		etCfg:       etCfg,
	}
}

// ---- 辅助方法 ----

func (s *EventTypeService) getOrNotFound(ctx context.Context, id int64) (*model.EventType, error) {
	et, err := s.store.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get event_type %d: %w", id, err)
	}
	if et == nil {
		return nil, errcode.Newf(errcode.ErrEventTypeNotFound, "事件类型 ID=%d 不存在", id)
	}
	return et, nil
}

// buildConfigJSON 合并系统字段 + 校验后的扩展字段为 config_json
func (s *EventTypeService) buildConfigJSON(
	displayName, perceptionMode string,
	defaultSeverity, defaultTTL, rangeMeters float64,
	extensions map[string]interface{},
) (json.RawMessage, error) {
	configMap := map[string]interface{}{
		"display_name":     displayName,
		"default_severity": defaultSeverity,
		"default_ttl":      defaultTTL,
		"perception_mode":  perceptionMode,
		"range":            rangeMeters,
	}
	for k, v := range extensions {
		configMap[k] = v
	}
	data, err := json.Marshal(configMap)
	if err != nil {
		return nil, fmt.Errorf("marshal config_json: %w", err)
	}
	return data, nil
}

// validateExtensions 校验扩展字段值是否符合 schema 约束
//
// 这是 ADMIN 侧契约承诺的核心：保存扩展字段值前必须校验。
func (s *EventTypeService) validateExtensions(extensions map[string]interface{}) *errcode.Error {
	if len(extensions) == 0 {
		return nil
	}
	for key, val := range extensions {
		schema, ok := s.schemaCache.GetByFieldName(key)
		if !ok {
			return errcode.Newf(errcode.ErrExtSchemaNotFound, "扩展字段 '%s' 定义不存在", key)
		}
		// 注意：GetByFieldName 只返回 enabled=1 的缓存条目，
		// 所以如果能查到就一定是启用的，不需要额外检查 enabled

		// 将 val 序列化为 json.RawMessage 做校验
		valJSON, err := json.Marshal(val)
		if err != nil {
			return errcode.Newf(errcode.ErrEventTypeExtValueInvalid, "扩展字段 '%s' 值序列化失败", key)
		}
		if e := constraint.ValidateValue(schema.FieldType, schema.Constraints, valJSON); e != nil {
			return errcode.Newf(errcode.ErrEventTypeExtValueInvalid, "扩展字段 '%s': %s", key, e.Error())
		}
	}
	return nil
}

// ---- CRUD ----

// List 分页列表
func (s *EventTypeService) List(ctx context.Context, q *model.EventTypeListQuery) (*model.ListData, error) {
	// 分页校正
	if q.Page < 1 {
		q.Page = s.pagCfg.DefaultPage
	}
	if q.PageSize < 1 {
		q.PageSize = s.pagCfg.DefaultPageSize
	}
	if q.PageSize > s.pagCfg.MaxPageSize {
		q.PageSize = s.pagCfg.MaxPageSize
	}

	// 查缓存
	cached, hit, _ := s.cache.GetList(ctx, q)
	if hit && cached != nil {
		slog.Debug("service.event_type.list.cache_hit")
		return cached.ToListData(), nil
	}

	// 查 MySQL
	items, total, err := s.store.List(ctx, q)
	if err != nil {
		return nil, err
	}

	// 从 config_json 抽展示字段
	listItems := make([]model.EventTypeListItem, 0, len(items))
	for _, et := range items {
		item := model.EventTypeListItem{
			ID:             et.ID,
			Name:           et.Name,
			DisplayName:    et.DisplayName,
			PerceptionMode: et.PerceptionMode,
			Enabled:        et.Enabled,
			CreatedAt:      et.CreatedAt,
		}
		// unmarshal config_json 抽展示值
		var cfg map[string]interface{}
		if err := json.Unmarshal(et.ConfigJSON, &cfg); err == nil {
			if v, ok := cfg["default_severity"].(float64); ok {
				item.DefaultSeverity = v
			}
			if v, ok := cfg["default_ttl"].(float64); ok {
				item.DefaultTTL = v
			}
			if v, ok := cfg["range"].(float64); ok {
				item.Range = v
			}
		}
		listItems = append(listItems, item)
	}

	// 写缓存
	listData := &model.EventTypeListData{
		Items:    listItems,
		Total:    total,
		Page:     q.Page,
		PageSize: q.PageSize,
	}
	s.cache.SetList(ctx, q, listData)

	return listData.ToListData(), nil
}

// Create 创建事件类型
func (s *EventTypeService) Create(ctx context.Context, req *model.CreateEventTypeRequest) (int64, error) {
	slog.Debug("service.event_type.create", "name", req.Name)

	// name 唯一性（含软删除）
	exists, err := s.store.ExistsByName(ctx, req.Name)
	if err != nil {
		return 0, err
	}
	if exists {
		return 0, errcode.Newf(errcode.ErrEventTypeNameExists, "事件标识 '%s' 已存在", req.Name)
	}

	// 扩展字段约束校验
	if e := s.validateExtensions(req.Extensions); e != nil {
		return 0, e
	}

	// global 模式强制 range=0
	rangeVal := req.Range
	if req.PerceptionMode == model.PerceptionModeGlobal {
		rangeVal = 0
	}

	// 拼 config_json
	configJSON, err := s.buildConfigJSON(req.DisplayName, req.PerceptionMode, req.DefaultSeverity, req.DefaultTTL, rangeVal, req.Extensions)
	if err != nil {
		return 0, err
	}

	// 写 MySQL
	id, err := s.store.Create(ctx, req.Name, req.DisplayName, req.PerceptionMode, configJSON)
	if err != nil {
		return 0, err
	}

	// 清列表缓存
	s.cache.InvalidateList(ctx)

	slog.Info("service.event_type.created", "id", id, "name", req.Name)
	return id, nil
}

// GetByID 查详情（Cache-Aside + 分布式锁 + 空标记）
func (s *EventTypeService) GetByID(ctx context.Context, id int64) (*model.EventType, error) {
	// 1. 查缓存
	et, hit, _ := s.cache.GetDetail(ctx, id)
	if hit {
		if et == nil {
			return nil, errcode.New(errcode.ErrEventTypeNotFound)
		}
		return et, nil
	}

	// 2. 分布式锁防击穿
	lockTTL := 3 * time.Second
	if s.etCfg.CacheLockTTL > 0 {
		lockTTL = s.etCfg.CacheLockTTL
	}
	locked, _ := s.cache.TryLock(ctx, id, lockTTL)
	if locked {
		defer s.cache.Unlock(ctx, id)
		// double-check
		et, hit, _ = s.cache.GetDetail(ctx, id)
		if hit {
			if et == nil {
				return nil, errcode.New(errcode.ErrEventTypeNotFound)
			}
			return et, nil
		}
	}

	// 3. 查 MySQL
	et, err := s.store.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// 4. 写缓存（含空标记）
	s.cache.SetDetail(ctx, id, et)

	if et == nil {
		return nil, errcode.New(errcode.ErrEventTypeNotFound)
	}
	return et, nil
}

// Update 编辑事件类型
func (s *EventTypeService) Update(ctx context.Context, req *model.UpdateEventTypeRequest) error {
	slog.Debug("service.event_type.update", "id", req.ID)

	et, err := s.getOrNotFound(ctx, req.ID)
	if err != nil {
		return err
	}

	// 启用中禁止编辑
	if et.Enabled {
		return errcode.New(errcode.ErrEventTypeEditNotDisabled)
	}

	// 扩展字段约束校验
	if e := s.validateExtensions(req.Extensions); e != nil {
		return e
	}

	// global 模式强制 range=0
	rangeVal := req.Range
	if req.PerceptionMode == model.PerceptionModeGlobal {
		rangeVal = 0
	}

	// 拼 config_json
	configJSON, err := s.buildConfigJSON(req.DisplayName, req.PerceptionMode, req.DefaultSeverity, req.DefaultTTL, rangeVal, req.Extensions)
	if err != nil {
		return err
	}

	// 乐观锁更新
	if err := s.store.Update(ctx, req.ID, req.DisplayName, req.PerceptionMode, configJSON, req.Version); err != nil {
		if errors.Is(err, storemysql.ErrVersionConflict) {
			return errcode.New(errcode.ErrEventTypeVersionConflict)
		}
		return err
	}

	// 清缓存
	s.cache.DelDetail(ctx, req.ID)
	s.cache.InvalidateList(ctx)

	slog.Info("service.event_type.updated", "id", req.ID)
	return nil
}

// Delete 软删除事件类型
func (s *EventTypeService) Delete(ctx context.Context, id int64) error {
	slog.Debug("service.event_type.delete", "id", id)

	et, err := s.getOrNotFound(ctx, id)
	if err != nil {
		return err
	}

	// 启用中禁止删除
	if et.Enabled {
		return errcode.New(errcode.ErrEventTypeDeleteNotDisabled)
	}

	// 本期 ref_count 不接入，直接删
	// TODO: FSM/BT 上线后加 ref_count 检查 + FOR SHARE 防 TOCTOU

	if err := s.store.SoftDelete(ctx, id); err != nil {
		if errors.Is(err, storemysql.ErrNotFound) {
			return errcode.New(errcode.ErrEventTypeNotFound)
		}
		return err
	}

	// 清缓存
	s.cache.DelDetail(ctx, id)
	s.cache.InvalidateList(ctx)

	slog.Info("service.event_type.deleted", "id", id)
	return nil
}

// CheckName 唯一性校验
func (s *EventTypeService) CheckName(ctx context.Context, name string) (*model.CheckNameResult, error) {
	exists, err := s.store.ExistsByName(ctx, name)
	if err != nil {
		return nil, err
	}
	result := &model.CheckNameResult{Available: !exists}
	if exists {
		result.Message = "该事件标识已存在"
	}
	return result, nil
}

// ToggleEnabled 切换启用/停用
func (s *EventTypeService) ToggleEnabled(ctx context.Context, id int64, version int) error {
	slog.Debug("service.event_type.toggle_enabled", "id", id)

	et, err := s.getOrNotFound(ctx, id)
	if err != nil {
		return err
	}

	newEnabled := !et.Enabled
	if err := s.store.ToggleEnabled(ctx, id, newEnabled, version); err != nil {
		if errors.Is(err, storemysql.ErrVersionConflict) {
			return errcode.New(errcode.ErrEventTypeVersionConflict)
		}
		return err
	}

	// 清缓存
	s.cache.DelDetail(ctx, id)
	s.cache.InvalidateList(ctx)

	slog.Info("service.event_type.toggled", "id", id, "enabled", newEnabled)
	return nil
}

// ExportAll 导出所有已启用的事件类型
func (s *EventTypeService) ExportAll(ctx context.Context) ([]model.EventTypeExportItem, error) {
	return s.store.ExportAll(ctx)
}
