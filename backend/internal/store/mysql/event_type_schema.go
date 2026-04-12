package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/yqihe/npc-ai-admin/backend/internal/model"
)

// EventTypeSchemaStore event_type_schema 表操作
type EventTypeSchemaStore struct {
	db interface {
		GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
		SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
		ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	}
}

// NewEventTypeSchemaStore 创建 EventTypeSchemaStore
func NewEventTypeSchemaStore(db interface {
	GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}) *EventTypeSchemaStore {
	return &EventTypeSchemaStore{db: db}
}

// Create 创建扩展字段定义，返回自增 ID
func (s *EventTypeSchemaStore) Create(ctx context.Context, req *model.CreateEventTypeSchemaRequest) (int64, error) {
	now := time.Now()
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO event_type_schema (field_name, field_label, field_type, constraints, default_value, sort_order, enabled, version, created_at, updated_at, deleted)
		 VALUES (?, ?, ?, ?, ?, ?, 1, 1, ?, ?, 0)`,
		req.FieldName, req.FieldLabel, req.FieldType, req.Constraints, req.DefaultValue, req.SortOrder, now, now,
	)
	if err != nil {
		return 0, fmt.Errorf("insert event_type_schema: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	return id, nil
}

// GetByID 按主键查询
func (s *EventTypeSchemaStore) GetByID(ctx context.Context, id int64) (*model.EventTypeSchema, error) {
	var ets model.EventTypeSchema
	err := s.db.GetContext(ctx, &ets,
		`SELECT id, field_name, field_label, field_type, constraints, default_value, sort_order, enabled, version, created_at, updated_at, deleted
		 FROM event_type_schema WHERE id = ? AND deleted = 0`, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get event_type_schema by id: %w", err)
	}
	return &ets, nil
}

// ExistsByFieldName 检查 field_name 是否已存在（含软删除）
func (s *EventTypeSchemaStore) ExistsByFieldName(ctx context.Context, fieldName string) (bool, error) {
	var count int
	err := s.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM event_type_schema WHERE field_name = ?`, fieldName)
	if err != nil {
		return false, fmt.Errorf("check event_type_schema field_name exists: %w", err)
	}
	return count > 0, nil
}

// List 列表查询（可按 enabled 筛选，按 sort_order ASC, id ASC 排序）
func (s *EventTypeSchemaStore) List(ctx context.Context, q *model.EventTypeSchemaListQuery) ([]model.EventTypeSchema, error) {
	query := `SELECT id, field_name, field_label, field_type, constraints, default_value, sort_order, enabled, version, created_at, updated_at
		 FROM event_type_schema WHERE deleted = 0`
	args := make([]any, 0, 1)

	if q != nil && q.Enabled != nil {
		query += " AND enabled = ?"
		args = append(args, *q.Enabled)
	}

	query += " ORDER BY sort_order ASC, id ASC"

	items := make([]model.EventTypeSchema, 0)
	if err := s.db.SelectContext(ctx, &items, query, args...); err != nil {
		return nil, fmt.Errorf("list event_type_schema: %w", err)
	}
	return items, nil
}

// ListEnabled 全量拉启用的（给内存缓存 Load 用）
func (s *EventTypeSchemaStore) ListEnabled(ctx context.Context) ([]model.EventTypeSchemaLite, error) {
	items := make([]model.EventTypeSchemaLite, 0)
	err := s.db.SelectContext(ctx, &items,
		`SELECT field_name, field_label, field_type, constraints, default_value, sort_order
		 FROM event_type_schema WHERE deleted = 0 AND enabled = 1
		 ORDER BY sort_order ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list enabled event_type_schema: %w", err)
	}
	return items, nil
}

// Update 编辑扩展字段定义（乐观锁）
func (s *EventTypeSchemaStore) Update(ctx context.Context, req *model.UpdateEventTypeSchemaRequest) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE event_type_schema SET field_label = ?, constraints = ?, default_value = ?, sort_order = ?, version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ? AND deleted = 0`,
		req.FieldLabel, req.Constraints, req.DefaultValue, req.SortOrder, time.Now(), req.ID, req.Version,
	)
	if err != nil {
		return fmt.Errorf("update event_type_schema: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrVersionConflict
	}
	return nil
}

// SoftDelete 软删除
func (s *EventTypeSchemaStore) SoftDelete(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE event_type_schema SET deleted = 1, updated_at = ? WHERE id = ? AND deleted = 0`,
		time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("soft delete event_type_schema: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ToggleEnabled 切换启用/停用（乐观锁）
func (s *EventTypeSchemaStore) ToggleEnabled(ctx context.Context, id int64, enabled bool, version int) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE event_type_schema SET enabled = ?, version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ? AND deleted = 0`,
		enabled, time.Now(), id, version,
	)
	if err != nil {
		return fmt.Errorf("toggle event_type_schema enabled: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return ErrVersionConflict
	}
	return nil
}
