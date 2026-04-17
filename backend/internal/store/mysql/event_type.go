package mysql

import (
	shared "github.com/yqihe/npc-ai-admin/backend/internal/store/mysql/shared"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
)

// EventTypeStore event_types 表操作
//
// 严格遵守"分层职责"硬规则：只对 event_types 表 CRUD，
// 不读写 event_type_schema 等其它模块的表。
type EventTypeStore struct {
	db *sqlx.DB
}

// NewEventTypeStore 创建 EventTypeStore
func NewEventTypeStore(db *sqlx.DB) *EventTypeStore {
	return &EventTypeStore{db: db}
}

// DB 暴露数据库连接（handler 层开跨模块事务用）
func (s *EventTypeStore) DB() *sqlx.DB {
	return s.db
}

// Create 创建事件类型，返回自增 ID
func (s *EventTypeStore) Create(ctx context.Context, req *model.CreateEventTypeRequest, configJSON json.RawMessage) (int64, error) {
	now := time.Now()
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO event_types (name, display_name, perception_mode, config_json, enabled, version, created_at, updated_at, deleted)
		 VALUES (?, ?, ?, ?, 0, 1, ?, ?, 0)`,
		req.Name, req.DisplayName, req.PerceptionMode, configJSON, now, now,
	)
	if err != nil {
		if shared.Is1062(err) {
			return 0, errcode.ErrDuplicate
		}
		return 0, fmt.Errorf("insert event_type: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	return id, nil
}

// GetByID 按主键查询事件类型（含 config_json）
func (s *EventTypeStore) GetByID(ctx context.Context, id int64) (*model.EventType, error) {
	var et model.EventType
	err := s.db.GetContext(ctx, &et,
		`SELECT id, name, display_name, perception_mode, config_json, enabled, version, created_at, updated_at, deleted
		 FROM event_types WHERE id = ? AND deleted = 0`, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get event_type by id: %w", err)
	}
	return &et, nil
}

// ExistsByName 检查 name 是否已存在（含软删除）
//
// 不过滤 deleted：已删除的 name 永久不可复用。
func (s *EventTypeStore) ExistsByName(ctx context.Context, name string) (bool, error) {
	var count int
	err := s.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM event_types WHERE name = ?`, name)
	if err != nil {
		return false, fmt.Errorf("check event_type name exists: %w", err)
	}
	return count > 0, nil
}

// List 分页列表查询
//
// 走 idx_list (deleted, enabled, id DESC)。
// 列表返回核心列 + config_json（service 层会 unmarshal 抽取展示字段）。
func (s *EventTypeStore) List(ctx context.Context, q *model.EventTypeListQuery) ([]model.EventType, int64, error) {
	where := []string{"deleted = 0"}
	args := make([]any, 0, 5)

	if q.Name != "" {
		where = append(where, "name LIKE ?")
		args = append(args, "%"+shared.EscapeLike(q.Name)+"%")
	}
	if q.Label != "" {
		where = append(where, "display_name LIKE ?")
		args = append(args, "%"+shared.EscapeLike(q.Label)+"%")
	}
	if q.PerceptionMode != "" {
		where = append(where, "perception_mode = ?")
		args = append(args, q.PerceptionMode)
	}
	if q.Enabled != nil {
		where = append(where, "enabled = ?")
		args = append(args, *q.Enabled)
	}

	whereClause := strings.Join(where, " AND ")

	// 计数
	var total int64
	countSQL := "SELECT COUNT(*) FROM event_types WHERE " + whereClause
	if err := s.db.GetContext(ctx, &total, countSQL, args...); err != nil {
		return nil, 0, fmt.Errorf("count event_types: %w", err)
	}

	if total == 0 {
		return make([]model.EventType, 0), 0, nil
	}

	// 分页查询（按 id DESC）
	offset := (q.Page - 1) * q.PageSize
	listSQL := fmt.Sprintf(
		`SELECT id, name, display_name, perception_mode, config_json, enabled, version, created_at, updated_at
		 FROM event_types WHERE %s ORDER BY id DESC LIMIT ? OFFSET ?`,
		whereClause,
	)
	listArgs := make([]any, len(args), len(args)+2)
	copy(listArgs, args)
	listArgs = append(listArgs, q.PageSize, offset)

	items := make([]model.EventType, 0)
	if err := s.db.SelectContext(ctx, &items, listSQL, listArgs...); err != nil {
		return nil, 0, fmt.Errorf("list event_types: %w", err)
	}

	return items, total, nil
}

// Update 编辑事件类型（乐观锁，按 ID）
//
// rows=0 → errcode.ErrVersionConflict（version 不匹配 或 记录已删除）。
func (s *EventTypeStore) Update(ctx context.Context, req *model.UpdateEventTypeRequest, configJSON json.RawMessage) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE event_types SET display_name = ?, perception_mode = ?, config_json = ?, version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ? AND deleted = 0`,
		req.DisplayName, req.PerceptionMode, configJSON, time.Now(), req.ID, req.Version,
	)
	if err != nil {
		return fmt.Errorf("update event_type: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return errcode.ErrVersionConflict
	}
	return nil
}

// SoftDelete 软删除事件类型
func (s *EventTypeStore) SoftDelete(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE event_types SET deleted = 1, updated_at = ? WHERE id = ? AND deleted = 0`,
		time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("soft delete event_type: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return errcode.ErrNotFound
	}
	return nil
}

// ToggleEnabled 切换启用/停用（乐观锁，按 ID）
func (s *EventTypeStore) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE event_types SET enabled = ?, version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ? AND deleted = 0`,
		req.Enabled, time.Now(), req.ID, req.Version,
	)
	if err != nil {
		return fmt.Errorf("toggle event_type enabled: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return errcode.ErrVersionConflict
	}
	return nil
}

// ---- 事务版方法（handler 跨模块编排用）----

// CreateTx 事务内创建事件类型，返回自增 ID
func (s *EventTypeStore) CreateTx(ctx context.Context, tx *sqlx.Tx, req *model.CreateEventTypeRequest, configJSON json.RawMessage) (int64, error) {
	now := time.Now()
	result, err := tx.ExecContext(ctx,
		`INSERT INTO event_types (name, display_name, perception_mode, config_json, enabled, version, created_at, updated_at, deleted)
		 VALUES (?, ?, ?, ?, 0, 1, ?, ?, 0)`,
		req.Name, req.DisplayName, req.PerceptionMode, configJSON, now, now,
	)
	if err != nil {
		if shared.Is1062(err) {
			return 0, errcode.ErrDuplicate
		}
		return 0, fmt.Errorf("insert event_type tx: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	return id, nil
}

// UpdateTx 事务内编辑事件类型（乐观锁）
func (s *EventTypeStore) UpdateTx(ctx context.Context, tx *sqlx.Tx, req *model.UpdateEventTypeRequest, configJSON json.RawMessage) error {
	result, err := tx.ExecContext(ctx,
		`UPDATE event_types SET display_name = ?, perception_mode = ?, config_json = ?, version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ? AND deleted = 0`,
		req.DisplayName, req.PerceptionMode, configJSON, time.Now(), req.ID, req.Version,
	)
	if err != nil {
		return fmt.Errorf("update event_type tx: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return errcode.ErrVersionConflict
	}
	return nil
}

// SoftDeleteTx 事务内软删除事件类型
func (s *EventTypeStore) SoftDeleteTx(ctx context.Context, tx *sqlx.Tx, id int64) error {
	result, err := tx.ExecContext(ctx,
		`UPDATE event_types SET deleted = 1, updated_at = ? WHERE id = ? AND deleted = 0`,
		time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("soft delete event_type tx: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if rows == 0 {
		return errcode.ErrNotFound
	}
	return nil
}

// ExportAll 导出所有已启用且未删除的事件类型
//
// 返回 (name, config_json) 二元组，handler 层原样输出到 HTTP 响应。
func (s *EventTypeStore) ExportAll(ctx context.Context) ([]model.EventTypeExportItem, error) {
	items := make([]model.EventTypeExportItem, 0)
	err := s.db.SelectContext(ctx, &items,
		`SELECT name, config_json AS config FROM event_types WHERE deleted = 0 AND enabled = 1 ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("export event_types: %w", err)
	}
	return items, nil
}
