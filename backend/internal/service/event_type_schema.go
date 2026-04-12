package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/yqihe/npc-ai-admin/backend/internal/cache"
	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	"github.com/yqihe/npc-ai-admin/backend/internal/service/constraint"
	storemysql "github.com/yqihe/npc-ai-admin/backend/internal/store/mysql"
	"github.com/yqihe/npc-ai-admin/backend/internal/util"
)

// EventTypeSchemaService 事件类型扩展字段 Schema 业务逻辑
type EventTypeSchemaService struct {
	store       *storemysql.EventTypeSchemaStore
	schemaCache *cache.EventTypeSchemaCache
	etsCfg      *config.EventTypeSchemaConfig
}

// NewEventTypeSchemaService 创建 EventTypeSchemaService
func NewEventTypeSchemaService(
	store *storemysql.EventTypeSchemaStore,
	schemaCache *cache.EventTypeSchemaCache,
	etsCfg *config.EventTypeSchemaConfig,
) *EventTypeSchemaService {
	return &EventTypeSchemaService{
		store:       store,
		schemaCache: schemaCache,
		etsCfg:      etsCfg,
	}
}

// ---- 辅助 ----

func (s *EventTypeSchemaService) getOrNotFound(ctx context.Context, id int64) (*model.EventTypeSchema, error) {
	ets, err := s.store.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get event_type_schema %d: %w", id, err)
	}
	if ets == nil {
		return nil, errcode.Newf(errcode.ErrExtSchemaNotFound, "扩展字段定义 ID=%d 不存在", id)
	}
	return ets, nil
}

// ---- CRUD ----

// List 列表查询（量小直查 MySQL，不走 Redis）
func (s *EventTypeSchemaService) List(ctx context.Context, q *model.EventTypeSchemaListQuery) ([]model.EventTypeSchema, error) {
	return s.store.List(ctx, q)
}

// ListEnabled 返回所有启用的扩展字段定义（内存缓存）
func (s *EventTypeSchemaService) ListEnabled() []model.EventTypeSchemaLite {
	return s.schemaCache.ListEnabled()
}

// Create 创建扩展字段定义
func (s *EventTypeSchemaService) Create(ctx context.Context, req *model.CreateEventTypeSchemaRequest) (int64, error) {
	slog.Debug("service.event_type_schema.create", "field_name", req.FieldName)

	// field_name 唯一性（含软删除）
	exists, err := s.store.ExistsByFieldName(ctx, req.FieldName)
	if err != nil {
		return 0, err
	}
	if exists {
		return 0, errcode.Newf(errcode.ErrExtSchemaNameExists, "扩展字段标识 '%s' 已存在", req.FieldName)
	}

	// field_type 枚举校验
	if !util.ValidExtFieldTypes[req.FieldType] {
		return 0, errcode.Newf(errcode.ErrExtSchemaTypeInvalid, "扩展字段类型 '%s' 不合法", req.FieldType)
	}

	// constraints 自洽校验
	if e := constraint.ValidateConstraintsSelf(req.FieldType, req.Constraints); e != nil {
		return 0, e
	}

	// default_value 必须符合 constraints
	if e := constraint.ValidateValue(req.FieldType, req.Constraints, req.DefaultValue); e != nil {
		return 0, errcode.Newf(errcode.ErrExtSchemaDefaultInvalid, "默认值不符合约束: %s", e.Error())
	}

	// 数量上限检查
	if s.etsCfg.MaxSchemas > 0 {
		all, err := s.store.List(ctx, nil)
		if err != nil {
			return 0, err
		}
		if len(all) >= s.etsCfg.MaxSchemas {
			return 0, errcode.Newf(errcode.ErrBadRequest, "扩展字段数量已达上限 %d", s.etsCfg.MaxSchemas)
		}
	}

	// 写 MySQL
	id, err := s.store.Create(ctx, req)
	if err != nil {
		return 0, err
	}

	// 重新加载内存缓存
	if err := s.schemaCache.Reload(ctx); err != nil {
		slog.Error("service.event_type_schema.reload_after_create", "error", err)
	}

	slog.Info("service.event_type_schema.created", "id", id, "field_name", req.FieldName)
	return id, nil
}

// Update 编辑扩展字段定义
func (s *EventTypeSchemaService) Update(ctx context.Context, req *model.UpdateEventTypeSchemaRequest) error {
	slog.Debug("service.event_type_schema.update", "id", req.ID)

	ets, err := s.getOrNotFound(ctx, req.ID)
	if err != nil {
		return err
	}

	if e := constraint.ValidateConstraintsSelf(ets.FieldType, req.Constraints); e != nil {
		return e
	}

	// default_value 符合新 constraints
	if e := constraint.ValidateValue(ets.FieldType, req.Constraints, req.DefaultValue); e != nil {
		return errcode.Newf(errcode.ErrExtSchemaDefaultInvalid, "默认值不符合约束: %s", e.Error())
	}

	// 乐观锁更新
	if err := s.store.Update(ctx, req); err != nil {
		if errors.Is(err, storemysql.ErrVersionConflict) {
			return errcode.New(errcode.ErrExtSchemaVersionConflict)
		}
		return err
	}

	// 重新加载内存缓存
	if err := s.schemaCache.Reload(ctx); err != nil {
		slog.Error("service.event_type_schema.reload_after_update", "error", err)
	}

	slog.Info("service.event_type_schema.updated", "id", req.ID)
	return nil
}

// Delete 软删除扩展字段定义
func (s *EventTypeSchemaService) Delete(ctx context.Context, id int64) error {
	slog.Debug("service.event_type_schema.delete", "id", id)

	ets, err := s.getOrNotFound(ctx, id)
	if err != nil {
		return err
	}

	// 必须先停用
	if ets.Enabled {
		return errcode.New(errcode.ErrExtSchemaDeleteNotDisabled)
	}

	if err := s.store.SoftDelete(ctx, id); err != nil {
		if errors.Is(err, storemysql.ErrNotFound) {
			return errcode.New(errcode.ErrExtSchemaNotFound)
		}
		return err
	}

	// 重新加载内存缓存
	if err := s.schemaCache.Reload(ctx); err != nil {
		slog.Error("service.event_type_schema.reload_after_delete", "error", err)
	}

	slog.Info("service.event_type_schema.deleted", "id", id)
	return nil
}

// ToggleEnabled 切换启用/停用
func (s *EventTypeSchemaService) ToggleEnabled(ctx context.Context, id int64, version int) error {
	slog.Debug("service.event_type_schema.toggle_enabled", "id", id)

	ets, err := s.getOrNotFound(ctx, id)
	if err != nil {
		return err
	}

	newEnabled := !ets.Enabled
	if err := s.store.ToggleEnabled(ctx, id, newEnabled, version); err != nil {
		if errors.Is(err, storemysql.ErrVersionConflict) {
			return errcode.New(errcode.ErrExtSchemaVersionConflict)
		}
		return err
	}

	// 重新加载内存缓存
	if err := s.schemaCache.Reload(ctx); err != nil {
		slog.Error("service.event_type_schema.reload_after_toggle", "error", err)
	}

	slog.Info("service.event_type_schema.toggled", "id", id, "enabled", newEnabled)
	return nil
}
