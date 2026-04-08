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

// Create 创建字段
func (s *FieldStore) Create(ctx context.Context, req *model.CreateFieldRequest) (int64, error) {
	now := time.Now()
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO fields (name, label, type, category, properties, ref_count, version, deleted, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, 0, 1, 0, ?, ?)`,
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

// GetByName 按 name 查询单条详情（走 uk_name）
func (s *FieldStore) GetByName(ctx context.Context, name string) (*model.Field, error) {
	var f model.Field
	err := s.db.GetContext(ctx, &f,
		`SELECT id, name, label, type, category, properties, ref_count, version, deleted, created_at, updated_at
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
		`SELECT id, name, label, type, category, ref_count, created_at
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

// Update 编辑字段（乐观锁）
func (s *FieldStore) Update(ctx context.Context, name string, req *model.UpdateFieldRequest) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE fields SET label = ?, type = ?, category = ?, properties = ?, version = version + 1, updated_at = ?
		 WHERE name = ? AND version = ? AND deleted = 0`,
		req.Label, req.Type, req.Category, string(req.Properties), time.Now(), name, req.Version,
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

// SoftDelete 软删除字段
func (s *FieldStore) SoftDelete(ctx context.Context, name string) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE fields SET deleted = 1, updated_at = ? WHERE name = ? AND deleted = 0`,
		time.Now(), name,
	)
	if err != nil {
		return fmt.Errorf("soft delete field: %w", err)
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

// BatchUpdateCategory 批量修改分类
func (s *FieldStore) BatchUpdateCategory(ctx context.Context, names []string, category string) (int64, error) {
	if len(names) == 0 {
		return 0, nil
	}
	query, args, err := sqlx.In(
		`UPDATE fields SET category = ?, updated_at = ? WHERE name IN (?) AND deleted = 0`,
		category, time.Now(), names,
	)
	if err != nil {
		return 0, fmt.Errorf("build in query: %w", err)
	}
	query = s.db.Rebind(query)
	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("batch update category: %w", err)
	}
	return result.RowsAffected()
}

// GetRefCount 获取引用计数
func (s *FieldStore) GetRefCount(ctx context.Context, name string) (int, error) {
	var count int
	err := s.db.GetContext(ctx, &count,
		`SELECT ref_count FROM fields WHERE name = ? AND deleted = 0`, name)
	if err == sql.ErrNoRows {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("get ref count: %w", err)
	}
	return count, nil
}

// IncrRefCount ref_count + 1
func (s *FieldStore) IncrRefCount(ctx context.Context, tx *sqlx.Tx, name string) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE fields SET ref_count = ref_count + 1 WHERE name = ? AND deleted = 0`, name)
	if err != nil {
		return fmt.Errorf("incr ref count: %w", err)
	}
	return nil
}

// DecrRefCount ref_count - 1
func (s *FieldStore) DecrRefCount(ctx context.Context, tx *sqlx.Tx, name string) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE fields SET ref_count = ref_count - 1 WHERE name = ? AND deleted = 0 AND ref_count > 0`, name)
	if err != nil {
		return fmt.Errorf("decr ref count: %w", err)
	}
	return nil
}

// GetByNames 批量查询字段（IN 查询，走 uk_name）
func (s *FieldStore) GetByNames(ctx context.Context, names []string) ([]model.Field, error) {
	if len(names) == 0 {
		return make([]model.Field, 0), nil
	}
	query, args, err := sqlx.In(
		`SELECT id, name, label, type, category, properties, ref_count, version, deleted, created_at, updated_at
		 FROM fields WHERE name IN (?) AND deleted = 0`, names)
	if err != nil {
		return nil, fmt.Errorf("build in query: %w", err)
	}
	query = s.db.Rebind(query)

	fields := make([]model.Field, 0)
	if err := s.db.SelectContext(ctx, &fields, query, args...); err != nil {
		return nil, fmt.Errorf("get fields by names: %w", err)
	}
	return fields, nil
}

// escapeLike 转义 LIKE 通配符，防止用户输入 % 或 _ 匹配所有记录
func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}
