package service

import (
	shared "github.com/yqihe/npc-ai-admin/backend/internal/service/shared"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jmoiron/sqlx"
	"github.com/yqihe/npc-ai-admin/backend/internal/cache"
	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	storemysql "github.com/yqihe/npc-ai-admin/backend/internal/store/mysql"
	storeredis "github.com/yqihe/npc-ai-admin/backend/internal/store/redis"
	rcfg "github.com/yqihe/npc-ai-admin/backend/internal/store/redis/shared"
	"github.com/yqihe/npc-ai-admin/backend/internal/util"
)

// EventTypeService 事件类型管理业务逻辑
//
// SchemaRefStore 用于维护扩展字段引用关系（事件类型 CRUD 时写 schema_refs）。
// schemaStore 用于在写 schema_refs 时按 field_name 查 schema ID——必须走 DB
// 而非只含启用条目的内存缓存，否则 schema 被禁用后 ref 无法正确记录。
type EventTypeService struct {
	store          *storemysql.EventTypeStore
	schemaStore    *storemysql.EventTypeSchemaStore
	schemaRefStore *storemysql.SchemaRefStore
	cache          *storeredis.EventTypeCache
	schemaCache    *cache.EventTypeSchemaCache
	pagCfg         *config.PaginationConfig
	etCfg          *config.EventTypeConfig
}

// NewEventTypeService 创建 EventTypeService
func NewEventTypeService(
	store *storemysql.EventTypeStore,
	schemaStore *storemysql.EventTypeSchemaStore,
	schemaRefStore *storemysql.SchemaRefStore,
	cache *storeredis.EventTypeCache,
	schemaCache *cache.EventTypeSchemaCache,
	pagCfg *config.PaginationConfig,
	etCfg *config.EventTypeConfig,
) *EventTypeService {
	return &EventTypeService{
		store:          store,
		schemaStore:    schemaStore,
		schemaRefStore: schemaRefStore,
		cache:          cache,
		schemaCache:    schemaCache,
		pagCfg:         pagCfg,
		etCfg:          etCfg,
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
		if e := shared.ValidateValue(schema.FieldType, schema.Constraints, valJSON); e != nil {
			return errcode.Newf(errcode.ErrEventTypeExtValueInvalid, "扩展字段 '%s': %s", key, e.Error())
		}
	}
	return nil
}

// ---- CRUD ----

// List 分页列表
func (s *EventTypeService) List(ctx context.Context, q *model.EventTypeListQuery) (*model.ListData, error) {
	// 分页校正
	shared.NormalizePagination(&q.Page, &q.PageSize, s.pagCfg.DefaultPage, s.pagCfg.DefaultPageSize, s.pagCfg.MaxPageSize)

	// 查缓存（Redis 挂了跳过，降级直查 MySQL）
	if cached, hit, err := s.cache.GetList(ctx, q); err == nil && hit {
		slog.Debug("service.事件类型列表.缓存命中")
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
//
// 事务内同时写 event_types + schema_refs。
func (s *EventTypeService) Create(ctx context.Context, req *model.CreateEventTypeRequest) (int64, error) {
	slog.Debug("service.创建事件类型", "name", req.Name)

	// name 唯一性（含软删除）
	exists, err := s.store.ExistsByName(ctx, req.Name)
	if err != nil {
		slog.Error("service.创建事件类型-检查唯一性失败", "error", err, "name", req.Name)
		return 0, fmt.Errorf("check name exists: %w", err)
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
	if req.PerceptionMode == util.PerceptionModeGlobal {
		rangeVal = 0
	}

	// 拼 config_json
	configJSON, err := s.buildConfigJSON(req.DisplayName, req.PerceptionMode, req.DefaultSeverity, req.DefaultTTL, rangeVal, req.Extensions)
	if err != nil {
		return 0, err
	}

	// 事务：写 event_types + schema_refs
	tx, err := s.store.DB().BeginTxx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			slog.Warn("service.事件类型创建事务回滚失败", "error", rbErr)
		}
	}()

	id, err := s.store.CreateTx(ctx, tx, req, configJSON)
	if err != nil {
		if errors.Is(err, errcode.ErrDuplicate) {
			return 0, errcode.Newf(errcode.ErrEventTypeNameExists, "事件标识 '%s' 已存在", req.Name)
		}
		slog.Error("service.创建事件类型失败", "error", err, "name", req.Name)
		return 0, fmt.Errorf("create event_type: %w", err)
	}

	// 写 schema_refs
	if err := s.attachSchemaRefs(ctx, tx, id, req.Extensions); err != nil {
		return 0, err
	}

	// 先清缓存再 Commit（消除 Commit 后清缓存窗口期的脏读风险）
	s.cache.InvalidateList(ctx)

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit: %w", err)
	}

	slog.Info("service.创建事件类型成功", "id", id, "name", req.Name)
	return id, nil
}

// GetByID 查详情（Cache-Aside + 分布式锁 + 空标记）
func (s *EventTypeService) GetByID(ctx context.Context, id int64) (*model.EventType, error) {
	// 1. 查缓存（Redis 挂了跳过，降级直查 MySQL）
	if cached, hit, err := s.cache.GetDetail(ctx, id); err == nil && hit {
		if cached == nil {
			return nil, errcode.New(errcode.ErrEventTypeNotFound)
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
				return nil, errcode.New(errcode.ErrEventTypeNotFound)
			}
			return cached, nil
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
//
// 事务内同时更新 event_types + diff schema_refs。
func (s *EventTypeService) Update(ctx context.Context, req *model.UpdateEventTypeRequest) error {
	slog.Debug("service.编辑事件类型", "id", req.ID)

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
	if req.PerceptionMode == util.PerceptionModeGlobal {
		rangeVal = 0
	}

	// 拼 config_json
	configJSON, err := s.buildConfigJSON(req.DisplayName, req.PerceptionMode, req.DefaultSeverity, req.DefaultTTL, rangeVal, req.Extensions)
	if err != nil {
		return err
	}

	// 解析旧 extension keys
	oldExtKeys := s.extractExtensionKeys(et.ConfigJSON)
	newExtKeys := make(map[string]bool, len(req.Extensions))
	for k := range req.Extensions {
		newExtKeys[k] = true
	}

	// 事务：更新 event_types + diff schema_refs
	tx, err := s.store.DB().BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			slog.Warn("service.事件类型编辑事务回滚失败", "error", rbErr)
		}
	}()

	if err := s.store.UpdateTx(ctx, tx, req, configJSON); err != nil {
		if errors.Is(err, errcode.ErrVersionConflict) {
			return errcode.New(errcode.ErrEventTypeVersionConflict)
		}
		slog.Error("service.编辑事件类型失败", "error", err, "id", req.ID)
		return fmt.Errorf("update event_type: %w", err)
	}

	// diff schema_refs
	if err := s.syncSchemaRefs(ctx, tx, req.ID, oldExtKeys, newExtKeys); err != nil {
		return err
	}

	// 先清缓存再 Commit（消除 Commit 后清缓存窗口期的脏读风险）
	s.cache.DelDetail(ctx, req.ID)
	s.cache.InvalidateList(ctx)

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	slog.Info("service.编辑事件类型成功", "id", req.ID)
	return nil
}

// Delete 软删除事件类型
//
// 事务内同时软删 event_types + 清理 schema_refs。
func (s *EventTypeService) Delete(ctx context.Context, id int64) (*model.DeleteResult, error) {
	et, err := s.getOrNotFound(ctx, id)
	if err != nil {
		return nil, err
	}

	// 启用中禁止删除
	if et.Enabled {
		return nil, errcode.New(errcode.ErrEventTypeDeleteNotDisabled)
	}

	// 事务：软删 event_types + 清理 schema_refs
	tx, err := s.store.DB().BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			slog.Warn("service.事件类型删除事务回滚失败", "error", rbErr)
		}
	}()

	if err := s.store.SoftDeleteTx(ctx, tx, id); err != nil {
		if errors.Is(err, errcode.ErrNotFound) {
			return nil, errcode.New(errcode.ErrEventTypeNotFound)
		}
		slog.Error("service.删除事件类型失败", "error", err, "id", id)
		return nil, fmt.Errorf("soft delete event_type: %w", err)
	}

	// 清理该事件类型的所有 schema_refs
	if _, err := s.schemaRefStore.RemoveByRef(ctx, tx, util.RefTypeEventType, id); err != nil {
		return nil, fmt.Errorf("remove schema refs: %w", err)
	}

	// 先清缓存再 Commit（消除 Commit 后清缓存窗口期的脏读风险）
	s.cache.DelDetail(ctx, id)
	s.cache.InvalidateList(ctx)

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	slog.Info("service.删除事件类型成功", "id", id, "name", et.Name)
	return &model.DeleteResult{ID: id, Name: et.Name, Label: et.DisplayName}, nil
}

// CheckName 唯一性校验
func (s *EventTypeService) CheckName(ctx context.Context, name string) (*model.CheckNameResult, error) {
	exists, err := s.store.ExistsByName(ctx, name)
	if err != nil {
		slog.Error("service.校验事件标识失败", "error", err, "name", name)
		return nil, fmt.Errorf("check name: %w", err)
	}
	if exists {
		return &model.CheckNameResult{Available: false, Message: "该事件标识已存在"}, nil
	}
	return &model.CheckNameResult{Available: true, Message: "该标识可用"}, nil
}

// ToggleEnabled 切换启用/停用（由调用方指定目标状态，幂等安全）
func (s *EventTypeService) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) error {
	if _, err := s.getOrNotFound(ctx, req.ID); err != nil {
		return err
	}

	if err := s.store.ToggleEnabled(ctx, req); err != nil {
		if errors.Is(err, errcode.ErrVersionConflict) {
			return errcode.New(errcode.ErrEventTypeVersionConflict)
		}
		slog.Error("service.切换启用失败", "error", err, "id", req.ID)
		return fmt.Errorf("toggle enabled: %w", err)
	}

	// 清缓存
	s.cache.DelDetail(ctx, req.ID)
	s.cache.InvalidateList(ctx)

	slog.Info("service.切换启用成功", "id", req.ID, "enabled", req.Enabled)
	return nil
}

// ExportAll 导出所有已启用的事件类型
//
// 将外层 name 注入 config JSON 内部。游戏服务端 cmd/server/main.go:249
// 从 config 反序列化出 EventTypeConfig 后用 cfg.Name 作索引键——
// 若不注入，所有事件都落到空字符串 key，事件系统失效。
// 其他配置（FSM/BT/NPCTemplate）用外层 name 寻址，不受此影响。
func (s *EventTypeService) ExportAll(ctx context.Context) ([]model.EventTypeExportItem, error) {
	items, err := s.store.ExportAll(ctx)
	if err != nil {
		return nil, err
	}
	for i := range items {
		merged, mergeErr := injectNameIntoConfig(items[i].Name, items[i].Config)
		if mergeErr != nil {
			slog.Error("service.事件类型导出.注入name失败", "name", items[i].Name, "error", mergeErr)
			return nil, fmt.Errorf("inject name into config for %q: %w", items[i].Name, mergeErr)
		}
		items[i].Config = merged
	}
	return items, nil
}

// injectNameIntoConfig 把 name 合并到 config JSON 中（已有同名 key 以外层 name 为准）。
func injectNameIntoConfig(name string, config json.RawMessage) (json.RawMessage, error) {
	var m map[string]interface{}
	if err := json.Unmarshal(config, &m); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	if m == nil {
		m = make(map[string]interface{})
	}
	m["name"] = name
	return json.Marshal(m)
}

// GetDetail 查详情并拼装 EventTypeDetail（含 config 展开 + 扩展字段 schema 合并）
//
// 业务逻辑：
//  1. 从 cache/DB 拿 EventType 裸行
//  2. unmarshal config_json
//  3. 合并扩展字段 schema（启用的 + 虽然禁用但 config 里有值的）
func (s *EventTypeService) GetDetail(ctx context.Context, id int64) (*model.EventTypeDetail, error) {
	et, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// unmarshal config_json
	var config map[string]interface{}
	if et.ConfigJSON != nil {
		if err := json.Unmarshal(et.ConfigJSON, &config); err != nil {
			slog.Error("service.事件类型详情.unmarshal_config", "error", err, "id", id)
			config = make(map[string]interface{})
		}
	}
	if config == nil {
		config = make(map[string]interface{})
	}

	// 拿扩展字段 schema：启用的 + 虽然禁用但 config 里有值的
	schemas := s.schemaCache.ListEnabled()
	enabledNames := make(map[string]bool, len(schemas))
	for _, sc := range schemas {
		enabledNames[sc.FieldName] = true
	}

	// 系统字段集合
	systemKeys := map[string]bool{
		"display_name": true, "default_severity": true,
		"default_ttl": true, "perception_mode": true, "range": true,
	}

	// 检查 config 中是否有禁用 schema 的值
	var missingNames []string
	for k := range config {
		if !systemKeys[k] && !enabledNames[k] {
			missingNames = append(missingNames, k)
		}
	}
	if len(missingNames) > 0 {
		allSchemas, err := s.schemaStore.ListAllLite(ctx)
		if err != nil {
			slog.Error("service.事件类型详情.list_all_schemas", "error", err)
		} else {
			missingSet := make(map[string]bool, len(missingNames))
			for _, n := range missingNames {
				missingSet[n] = true
			}
			for _, sc := range allSchemas {
				if missingSet[sc.FieldName] {
					schemas = append(schemas, sc)
				}
			}
		}
	}

	return &model.EventTypeDetail{
		ID:              et.ID,
		Name:            et.Name,
		DisplayName:     et.DisplayName,
		PerceptionMode:  et.PerceptionMode,
		Enabled:         et.Enabled,
		Version:         et.Version,
		CreatedAt:       et.CreatedAt,
		UpdatedAt:       et.UpdatedAt,
		Config:          config,
		ExtensionSchema: schemas,
	}, nil
}

// ---- schema_refs 维护辅助 ----

// extractExtensionKeys 从 config_json 中提取扩展字段 key 集合
//
// 系统字段（display_name 等）排除，剩余 key 即扩展字段。
func (s *EventTypeService) extractExtensionKeys(configJSON json.RawMessage) map[string]bool {
	systemKeys := map[string]bool{
		"display_name":     true,
		"default_severity": true,
		"default_ttl":      true,
		"perception_mode":  true,
		"range":            true,
	}
	var config map[string]interface{}
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return make(map[string]bool)
	}
	keys := make(map[string]bool)
	for k := range config {
		if !systemKeys[k] {
			keys[k] = true
		}
	}
	return keys
}

// attachSchemaRefs 为事件类型的扩展字段写入 schema_refs（事务内）
//
// 必须走 DB 查 schema ID，不能依赖只含启用条目的内存缓存——
// 若 schema 已禁用，缓存中查不到，ref 会被漏写，导致删除检查失效。
func (s *EventTypeService) attachSchemaRefs(ctx context.Context, tx *sqlx.Tx, eventTypeID int64, extensions map[string]interface{}) error {
	for key := range extensions {
		id, err := s.schemaStore.GetIDByFieldNameTx(ctx, tx, key)
		if err != nil {
			return fmt.Errorf("lookup schema id for %q: %w", key, err)
		}
		if id == 0 {
			continue // schema 不存在（已被硬/软删除），跳过
		}
		if err := s.schemaRefStore.Add(ctx, tx, id, util.RefTypeEventType, eventTypeID); err != nil {
			return fmt.Errorf("add schema ref %s → event_type %d: %w", key, eventTypeID, err)
		}
	}
	return nil
}

// syncSchemaRefs diff 旧/新扩展字段 key，增删 schema_refs（事务内）
//
// 新增和删除都走 DB 查 schema ID，不依赖内存缓存，确保 disabled schema 的 ref 也能正确维护。
func (s *EventTypeService) syncSchemaRefs(ctx context.Context, tx *sqlx.Tx, eventTypeID int64, oldKeys, newKeys map[string]bool) error {
	// toAdd: newKeys 中有但 oldKeys 没有
	for key := range newKeys {
		if !oldKeys[key] {
			id, err := s.schemaStore.GetIDByFieldNameTx(ctx, tx, key)
			if err != nil {
				return fmt.Errorf("lookup schema id for %q: %w", key, err)
			}
			if id == 0 {
				continue // schema 不存在，跳过
			}
			if err := s.schemaRefStore.Add(ctx, tx, id, util.RefTypeEventType, eventTypeID); err != nil {
				return fmt.Errorf("add schema ref %s: %w", key, err)
			}
		}
	}
	// toRemove: oldKeys 中有但 newKeys 没有
	for key := range oldKeys {
		if !newKeys[key] {
			id, err := s.schemaStore.GetIDByFieldNameTx(ctx, tx, key)
			if err != nil {
				slog.Warn("service.查schema_id失败", "key", key, "error", err)
				continue
			}
			if id > 0 {
				if err := s.schemaRefStore.Remove(ctx, tx, id, util.RefTypeEventType, eventTypeID); err != nil {
					return fmt.Errorf("remove schema ref %s: %w", key, err)
				}
			}
		}
	}
	return nil
}
