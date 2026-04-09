package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
)

// FieldStore fields 表操作
type FieldStore struct {
	db *sqlx.DB
}

// NewFieldStore 创建 FieldStore
func NewFieldStore(db *sqlx.DB) *FieldStore {
	return &FieldStore{db: db}
}

// DB 暴露数据库连接（service 层开事务用）
func (s *FieldStore) DB() *sqlx.DB {
	return s.db
}

// Create 创建字段，返回自增 ID
func (s *FieldStore) Create(ctx context.Context, req *model.CreateFieldRequest) (int64, error) {
	now := time.Now()
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO fields (name, label, type, category, properties, ref_count, enabled, version, deleted, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, 0, 0, 1, 0, ?, ?)`,
		req.Name, req.Label, req.Type, req.Category, string(req.Properties), now, now,
	)
	if err != nil {
		return 0, fmt.Errorf("insert field: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	return id, nil
}

// GetByID 按主键查询单条详情
func (s *FieldStore) GetByID(ctx context.Context, id int64) (*model.Field, error) {
	var f model.Field
	err := s.db.GetContext(ctx, &f,
		`SELECT id, name, label, type, category, properties, ref_count, enabled, version, deleted, created_at, updated_at
		 FROM fields WHERE id = ? AND deleted = 0`, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get field by id: %w", err)
	}
	return &f, nil
}

// GetByName 按 name 查询单条详情（check-name 和内部用，走 uk_name）
func (s *FieldStore) GetByName(ctx context.Context, name string) (*model.Field, error) {
	var f model.Field
	err := s.db.GetContext(ctx, &f,
		`SELECT id, name, label, type, category, properties, ref_count, enabled, version, deleted, created_at, updated_at
		 FROM fields WHERE name = ? AND deleted = 0`, name)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get field by name: %w", err)
	}
	return &f, nil
}

// ExistsByName 检查 name 是否已存在（含软删除）
func (s *FieldStore) ExistsByName(ctx context.Context, name string) (bool, error) {
	var count int
	err := s.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM fields WHERE name = ?`, name)
	if err != nil {
		return false, fmt.Errorf("check name exists: %w", err)
	}
	return count > 0, nil
}

// List 分页列表查询（覆盖索引，不回表）
func (s *FieldStore) List(ctx context.Context, q *model.FieldListQuery) ([]model.FieldListItem, int64, error) {
	where := []string{"deleted = 0"}
	args := make([]any, 0, 4)

	if q.Label != "" {
		where = append(where, "label LIKE ?")
		args = append(args, "%"+escapeLike(q.Label)+"%")
	}
	if q.Type != "" {
		where = append(where, "type = ?")
		args = append(args, q.Type)
	}
	if q.Category != "" {
		where = append(where, "category = ?")
		args = append(args, q.Category)
	}
	if q.Enabled != nil {
		where = append(where, "enabled = ?")
		args = append(args, *q.Enabled)
	}

	whereClause := strings.Join(where, " AND ")

	// 计数
	var total int64
	countSQL := "SELECT COUNT(*) FROM fields WHERE " + whereClause
	if err := s.db.GetContext(ctx, &total, countSQL, args...); err != nil {
		return nil, 0, fmt.Errorf("count fields: %w", err)
	}

	if total == 0 {
		return make([]model.FieldListItem, 0), 0, nil
	}

	// 分页查询
	offset := (q.Page - 1) * q.PageSize
	listSQL := fmt.Sprintf(
		`SELECT id, name, label, type, category, ref_count, enabled, created_at
		 FROM fields WHERE %s ORDER BY id DESC LIMIT ? OFFSET ?`,
		whereClause,
	)
	listArgs := make([]any, len(args), len(args)+2)
	copy(listArgs, args)
	listArgs = append(listArgs, q.PageSize, offset)

	items := make([]model.FieldListItem, 0)
	if err := s.db.SelectContext(ctx, &items, listSQL, listArgs...); err != nil {
		return nil, 0, fmt.Errorf("list fields: %w", err)
	}

	return items, total, nil
}

// Update 编辑字段（乐观锁，按 ID）
func (s *FieldStore) Update(ctx context.Context, req *model.UpdateFieldRequest) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE fields SET label = ?, type = ?, category = ?, properties = ?, version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ? AND deleted = 0`,
		req.Label, req.Type, req.Category, string(req.Properties), time.Now(), req.ID, req.Version,
	)
	if err != nil {
		return fmt.Errorf("update field: %w", err)
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

// SoftDeleteTx 事务内软删除字段（按 ID）
func (s *FieldStore) SoftDeleteTx(ctx context.Context, tx *sqlx.Tx, id int64) error {
	result, err := tx.ExecContext(ctx,
		`UPDATE fields SET deleted = 1, updated_at = ? WHERE id = ? AND deleted = 0`,
		time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("soft delete: %w", err)
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

// ToggleEnabled 切换启用/停用（乐观锁，按 ID）
func (s *FieldStore) ToggleEnabled(ctx context.Context, id int64, enabled bool, version int) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE fields SET enabled = ?, version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ? AND deleted = 0`,
		enabled, time.Now(), id, version,
	)
	if err != nil {
		return fmt.Errorf("toggle enabled: %w", err)
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

// IncrRefCountTx 事务内 ref_count + 1（按 ID）
func (s *FieldStore) IncrRefCountTx(ctx context.Context, tx *sqlx.Tx, id int64) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE fields SET ref_count = ref_count + 1 WHERE id = ? AND deleted = 0`, id)
	if err != nil {
		return fmt.Errorf("incr ref count: %w", err)
	}
	return nil
}

// DecrRefCountTx 事务内 ref_count - 1（按 ID）
func (s *FieldStore) DecrRefCountTx(ctx context.Context, tx *sqlx.Tx, id int64) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE fields SET ref_count = ref_count - 1 WHERE id = ? AND deleted = 0 AND ref_count > 0`, id)
	if err != nil {
		return fmt.Errorf("decr ref count: %w", err)
	}
	return nil
}

// GetByIDs 批量查询字段（IN 查询，走主键）
func (s *FieldStore) GetByIDs(ctx context.Context, ids []int64) ([]model.Field, error) {
	if len(ids) == 0 {
		return make([]model.Field, 0), nil
	}
	query, args, err := sqlx.In(
		`SELECT id, name, label, type, category, properties, ref_count, enabled, version, deleted, created_at, updated_at
		 FROM fields WHERE id IN (?) AND deleted = 0`, ids)
	if err != nil {
		return nil, fmt.Errorf("build in query: %w", err)
	}
	query = s.db.Rebind(query)

	fields := make([]model.Field, 0)
	if err := s.db.SelectContext(ctx, &fields, query, args...); err != nil {
		return nil, fmt.Errorf("get fields by ids: %w", err)
	}
	return fields, nil
}

// GetRefCountTx 事务内获取引用计数（FOR SHARE 防 TOCTOU）
func (s *FieldStore) GetRefCountTx(ctx context.Context, tx *sqlx.Tx, id int64) (int, error) {
	var count int
	err := tx.GetContext(ctx, &count,
		`SELECT ref_count FROM fields WHERE id = ? AND deleted = 0 FOR SHARE`, id)
	if err == sql.ErrNoRows {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("get ref count tx: %w", err)
	}
	return count, nil
}

// escapeLike 转义 LIKE 通配符，防止用户输入 % 或 _ 匹配所有记录
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}
