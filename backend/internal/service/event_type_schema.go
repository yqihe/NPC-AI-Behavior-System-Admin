package service

import (
	shared "github.com/yqihe/npc-ai-admin/backend/internal/service/shared"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jmoiron/sqlx"
	"github.com/yqihe/npc-ai-admin/backend/internal/cache"
	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	storemysql "github.com/yqihe/npc-ai-admin/backend/internal/store/mysql"
	"github.com/yqihe/npc-ai-admin/backend/internal/util"
)

// EventTypeSchemaService 事件类型扩展字段 Schema 业务逻辑
type EventTypeSchemaService struct {
	store          *storemysql.EventTypeSchemaStore
	schemaRefStore *storemysql.SchemaRefStore
	schemaCache    *cache.EventTypeSchemaCache
	etsCfg         *config.EventTypeSchemaConfig
	pagCfg         *config.PaginationConfig
}

// NewEventTypeSchemaService 创建 EventTypeSchemaService
func NewEventTypeSchemaService(
	store *storemysql.EventTypeSchemaStore,
	schemaRefStore *storemysql.SchemaRefStore,
	schemaCache *cache.EventTypeSchemaCache,
	etsCfg *config.EventTypeSchemaConfig,
	pagCfg *config.PaginationConfig,
) *EventTypeSchemaService {
	return &EventTypeSchemaService{
		store:          store,
		schemaRefStore: schemaRefStore,
		schemaCache:    schemaCache,
		etsCfg:         etsCfg,
		pagCfg:         pagCfg,
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

// GetByID 按主键查询（供 handler 层获取删除结果用）
func (s *EventTypeSchemaService) GetByID(ctx context.Context, id int64) (*model.EventTypeSchema, error) {
	return s.store.GetByID(ctx, id)
}

// List 分页列表查询（直查 MySQL，不走 Redis）
func (s *EventTypeSchemaService) List(ctx context.Context, q *model.EventTypeSchemaListQuery) (*model.ListData, error) {
	shared.NormalizePagination(&q.Page, &q.PageSize, s.pagCfg.DefaultPage, s.pagCfg.DefaultPageSize, s.pagCfg.MaxPageSize)

	items, total, err := s.store.List(ctx, q)
	if err != nil {
		return nil, err
	}
	return &model.ListData{
		Items:    items,
		Total:    total,
		Page:     q.Page,
		PageSize: q.PageSize,
	}, nil
}

// ListEnabled 返回所有启用的扩展字段定义（内存缓存）
func (s *EventTypeSchemaService) ListEnabled() []model.EventTypeSchemaLite {
	return s.schemaCache.ListEnabled()
}

// ListAllLite 返回所有未删除的扩展字段定义（含禁用的，给详情页合并用）
func (s *EventTypeSchemaService) ListAllLite(ctx context.Context) ([]model.EventTypeSchemaLite, error) {
	return s.store.ListAllLite(ctx)
}

// Create 创建扩展字段定义
func (s *EventTypeSchemaService) Create(ctx context.Context, req *model.CreateEventTypeSchemaRequest) (int64, error) {
	slog.Debug("service.创建扩展字段", "field_name", req.FieldName)

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
	if e := shared.ValidateConstraintsSelf(req.FieldType, req.Constraints, errcode.ErrExtSchemaConstraintsInvalid); e != nil {
		return 0, e
	}

	// default_value 必须符合 constraints
	if e := shared.ValidateValue(req.FieldType, req.Constraints, req.DefaultValue); e != nil {
		return 0, errcode.Newf(errcode.ErrExtSchemaDefaultInvalid, "默认值不符合约束: %s", e.Error())
	}

	// 数量上限检查
	if s.etsCfg.MaxSchemas > 0 {
		_, total, err := s.store.List(ctx, &model.EventTypeSchemaListQuery{Page: 1, PageSize: 1})
		if err != nil {
			return 0, err
		}
		if int(total) >= s.etsCfg.MaxSchemas {
			return 0, errcode.Newf(errcode.ErrBadRequest, "扩展字段数量已达上限 %d", s.etsCfg.MaxSchemas)
		}
	}

	// 写 MySQL
	id, err := s.store.Create(ctx, req)
	if err != nil {
		if errors.Is(err, errcode.ErrDuplicate) {
			return 0, errcode.Newf(errcode.ErrExtSchemaNameExists, "扩展字段标识 '%s' 已存在", req.FieldName)
		}
		return 0, err
	}

	// 重新加载内存缓存
	if err := s.schemaCache.Reload(ctx); err != nil {
		slog.Error("service.创建扩展字段-重载缓存失败", "error", err)
	}

	slog.Info("service.创建扩展字段成功", "id", id, "field_name", req.FieldName)
	return id, nil
}

// Update 编辑扩展字段定义
func (s *EventTypeSchemaService) Update(ctx context.Context, req *model.UpdateEventTypeSchemaRequest) error {
	slog.Debug("service.编辑扩展字段", "id", req.ID)

	ets, err := s.getOrNotFound(ctx, req.ID)
	if err != nil {
		return err
	}

	tx, err := s.store.DB().BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			slog.Warn("service.编辑扩展字段事务回滚失败", "error", rbErr)
		}
	}()

	// FOR SHARE：锁住 schema_refs 行，阻塞并发写入
	// 被引用时禁止收紧约束（类型不可变已天然满足：UpdateRequest 不含 FieldType）
	hasRefs, err := s.schemaRefStore.HasRefsTx(ctx, tx, req.ID)
	if err != nil {
		slog.Error("service.查询扩展字段引用失败", "error", err, "id", req.ID)
		return fmt.Errorf("check schema refs: %w", err)
	}
	if hasRefs {
		if e := CheckConstraintTightened(ets.FieldType, ets.Constraints, req.Constraints, errcode.ErrExtSchemaRefTighten); e != nil {
			return e
		}
	}

	// 纯计算校验，无副作用，在事务内调用无问题
	if e := shared.ValidateConstraintsSelf(ets.FieldType, req.Constraints, errcode.ErrExtSchemaConstraintsInvalid); e != nil {
		return e
	}

	// default_value 符合新 constraints
	if e := shared.ValidateValue(ets.FieldType, req.Constraints, req.DefaultValue); e != nil {
		return errcode.Newf(errcode.ErrExtSchemaDefaultInvalid, "默认值不符合约束: %s", e.Error())
	}

	// 乐观锁更新（事务内）
	if err := s.store.UpdateTx(ctx, tx, req); err != nil {
		if errors.Is(err, errcode.ErrVersionConflict) {
			return errcode.New(errcode.ErrExtSchemaVersionConflict)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}

	// 内存缓存必须在 Commit 成功后 Reload（全量重查 DB，Commit 前读到旧数据）
	if err := s.schemaCache.Reload(ctx); err != nil {
		slog.Error("service.编辑扩展字段-重载缓存失败", "error", err)
	}

	slog.Info("service.编辑扩展字段成功", "id", req.ID)
	return nil
}

// Delete 软删除扩展字段定义，返回被删实体（供 handler 构造响应用）
func (s *EventTypeSchemaService) Delete(ctx context.Context, id int64) (*model.EventTypeSchema, error) {
	slog.Debug("service.删除扩展字段", "id", id)

	ets, err := s.getOrNotFound(ctx, id)
	if err != nil {
		return nil, err
	}

	// 必须先停用
	if ets.Enabled {
		return nil, errcode.New(errcode.ErrExtSchemaDeleteNotDisabled)
	}

	tx, err := s.store.DB().BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			slog.Warn("service.删除扩展字段事务回滚失败", "error", rbErr)
		}
	}()

	// FOR SHARE：锁住 schema_refs 行，阻塞并发写入
	hasRefs, err := s.schemaRefStore.HasRefsTx(ctx, tx, id)
	if err != nil {
		slog.Error("service.查询扩展字段引用失败", "error", err, "id", id)
		return nil, fmt.Errorf("check schema refs: %w", err)
	}
	if hasRefs {
		return nil, errcode.New(errcode.ErrExtSchemaRefDelete)
	}

	if err := s.store.SoftDeleteTx(ctx, tx, id); err != nil {
		if errors.Is(err, errcode.ErrNotFound) {
			return nil, errcode.New(errcode.ErrExtSchemaNotFound)
		}
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	// 内存缓存必须在 Commit 成功后 Reload（全量重查 DB，Commit 前读到旧数据）
	if err := s.schemaCache.Reload(ctx); err != nil {
		slog.Error("service.删除扩展字段-重载缓存失败", "error", err)
	}

	slog.Info("service.删除扩展字段成功", "id", id, "field_name", ets.FieldName)
	return ets, nil
}

// ToggleEnabled 切换启用/停用
func (s *EventTypeSchemaService) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) error {
	slog.Debug("service.切换扩展字段启用", "id", req.ID)

	if _, err := s.getOrNotFound(ctx, req.ID); err != nil {
		return err
	}

	if err := s.store.ToggleEnabled(ctx, req); err != nil {
		if errors.Is(err, errcode.ErrVersionConflict) {
			return errcode.New(errcode.ErrExtSchemaVersionConflict)
		}
		return err
	}

	// 重新加载内存缓存
	if err := s.schemaCache.Reload(ctx); err != nil {
		slog.Error("service.切换扩展字段启用-重载缓存失败", "error", err)
	}

	slog.Info("service.切换扩展字段启用成功", "id", req.ID, "enabled", req.Enabled)
	return nil
}

// ---- 引用查询 ----

// GetReferences 查询扩展字段引用详情
//
// 返回 SchemaReferenceDetail，其中 EventTypes 的 Label 为空，由 handler 跨模块补齐。
func (s *EventTypeSchemaService) GetReferences(ctx context.Context, id int64) (*model.SchemaReferenceDetail, error) {
	ets, err := s.getOrNotFound(ctx, id)
	if err != nil {
		return nil, err
	}

	refs, err := s.schemaRefStore.GetBySchemaID(ctx, id)
	if err != nil {
		slog.Error("service.查询扩展字段引用失败", "error", err, "id", id)
		return nil, fmt.Errorf("get schema refs: %w", err)
	}

	eventTypes := make([]model.SchemaReferenceItem, 0)
	fsmConfigs := make([]model.SchemaReferenceItem, 0)
	btTrees := make([]model.SchemaReferenceItem, 0)
	for _, r := range refs {
		item := model.SchemaReferenceItem{RefType: r.RefType, RefID: r.RefID}
		switch r.RefType {
		case util.RefTypeEventType:
			eventTypes = append(eventTypes, item)
		case util.RefTypeFsm:
			fsmConfigs = append(fsmConfigs, item)
		case util.RefTypeBt:
			btTrees = append(btTrees, item)
		}
	}

	return &model.SchemaReferenceDetail{
		SchemaID:   id,
		FieldLabel: ets.FieldLabel,
		EventTypes: eventTypes,
		FsmConfigs: fsmConfigs,
		BtTrees:    btTrees,
	}, nil
}

// FillHasRefs 为扩展字段列表填充 has_refs
func (s *EventTypeSchemaService) FillHasRefs(ctx context.Context, items []model.EventTypeSchema) {
	for i := range items {
		hasRefs, err := s.schemaRefStore.HasRefs(ctx, items[i].ID)
		if err != nil {
			slog.Warn("service.填充扩展字段has_refs失败", "error", err, "id", items[i].ID)
			continue
		}
		items[i].HasRefs = hasRefs
	}
}

// ---- FSM/BT BB Key → schema_refs 维护 ----

// SyncFsmSchemaRefs 同步 FSM 条件中 BB Key 对事件扩展字段的引用关系（事务内）
//
// 对称 FieldService.SyncFsmBBKeyRefs：同一组 BB Key name，
// 来自 fields 表的写 field_refs，来自 event_type_schema 表的写 schema_refs。
// 返回 affected schema IDs（用于清缓存）。
func (s *EventTypeSchemaService) SyncFsmSchemaRefs(ctx context.Context, tx *sqlx.Tx, fsmID int64, oldKeys, newKeys map[string]bool) ([]int64, error) {
	return s.syncSchemaRefs(ctx, tx, util.RefTypeFsm, fsmID, oldKeys, newKeys)
}

// CleanFsmSchemaRefs 清理 FSM 删除时的所有扩展字段引用（事务内）
func (s *EventTypeSchemaService) CleanFsmSchemaRefs(ctx context.Context, tx *sqlx.Tx, fsmID int64) ([]int64, error) {
	return s.schemaRefStore.RemoveByRef(ctx, tx, util.RefTypeFsm, fsmID)
}

// SyncBtSchemaRefs 同步行为树节点中 BB Key 对事件扩展字段的引用关系（事务内）
func (s *EventTypeSchemaService) SyncBtSchemaRefs(ctx context.Context, tx *sqlx.Tx, btTreeID int64, oldKeys, newKeys map[string]bool) ([]int64, error) {
	return s.syncSchemaRefs(ctx, tx, util.RefTypeBt, btTreeID, oldKeys, newKeys)
}

// CleanBtSchemaRefs 清理行为树删除时的所有扩展字段引用（事务内）
func (s *EventTypeSchemaService) CleanBtSchemaRefs(ctx context.Context, tx *sqlx.Tx, btTreeID int64) ([]int64, error) {
	return s.schemaRefStore.RemoveByRef(ctx, tx, util.RefTypeBt, btTreeID)
}

// syncSchemaRefs 通用 diff 同步，对齐 EventTypeService.syncSchemaRefs 模式
func (s *EventTypeSchemaService) syncSchemaRefs(ctx context.Context, tx *sqlx.Tx, refType string, refID int64, oldKeys, newKeys map[string]bool) ([]int64, error) {
	affected := make([]int64, 0)

	// toAdd: newKeys 中有但 oldKeys 没有
	for key := range newKeys {
		if !oldKeys[key] {
			id, err := s.store.GetIDByFieldNameTx(ctx, tx, key)
			if err != nil {
				return nil, fmt.Errorf("lookup schema id for %q: %w", key, err)
			}
			if id == 0 {
				continue // 不是扩展字段（可能是 NPC 字段或运行时 Key），跳过
			}
			if err := s.schemaRefStore.Add(ctx, tx, id, refType, refID); err != nil {
				return nil, fmt.Errorf("add schema ref %s → %s %d: %w", key, refType, refID, err)
			}
			affected = append(affected, id)
		}
	}

	// toRemove: oldKeys 中有但 newKeys 没有
	for key := range oldKeys {
		if !newKeys[key] {
			id, err := s.store.GetIDByFieldNameTx(ctx, tx, key)
			if err != nil {
				slog.Warn("service.查schema_id失败", "key", key, "error", err)
				continue
			}
			if id > 0 {
				if err := s.schemaRefStore.Remove(ctx, tx, id, refType, refID); err != nil {
					return nil, fmt.Errorf("remove schema ref %s: %w", key, err)
				}
				affected = append(affected, id)
			}
		}
	}

	return affected, nil
}
