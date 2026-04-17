package mysql

import (
	shared "github.com/yqihe/npc-ai-admin/backend/internal/store/mysql/shared"
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
)

// FsmStateDictStore fsm_state_dicts 表操作
type FsmStateDictStore struct {
	db *sqlx.DB
}

// NewFsmStateDictStore 创建 FsmStateDictStore
func NewFsmStateDictStore(db *sqlx.DB) *FsmStateDictStore {
	return &FsmStateDictStore{db: db}
}

// DB 暴露数据库连接（service 层开事务用）
func (s *FsmStateDictStore) DB() *sqlx.DB {
	return s.db
}

// Create 创建状态字典条目，返回自增 ID
func (s *FsmStateDictStore) Create(ctx context.Context, req *model.CreateFsmStateDictRequest) (int64, error) {
	now := time.Now()
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO fsm_state_dicts (name, display_name, category, description, enabled, version, created_at, updated_at, deleted)
		 VALUES (?, ?, ?, ?, 1, 1, ?, ?, 0)`,
		req.Name, req.DisplayName, req.Category, req.Description, now, now,
	)
	if err != nil {
		if shared.Is1062(err) {
			return 0, errcode.ErrDuplicate
		}
		return 0, fmt.Errorf("insert fsm_state_dict: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	return id, nil
}

// GetByID 按主键查询状态字典条目
func (s *FsmStateDictStore) GetByID(ctx context.Context, id int64) (*model.FsmStateDict, error) {
	var d model.FsmStateDict
	err := s.db.GetContext(ctx, &d,
		`SELECT id, name, display_name, category, description, enabled, version, created_at, updated_at, deleted
		 FROM fsm_state_dicts WHERE id = ? AND deleted = 0`, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get fsm_state_dict by id: %w", err)
	}
	return &d, nil
}

// ExistsByName 检查 name 是否已存在（含软删除）
func (s *FsmStateDictStore) ExistsByName(ctx context.Context, name string) (bool, error) {
	var count int
	err := s.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM fsm_state_dicts WHERE name = ?`, name)
	if err != nil {
		return false, fmt.Errorf("check fsm_state_dict name exists: %w", err)
	}
	return count > 0, nil
}

// List 分页列表查询
//
// 支持 name/display_name 模糊（OR）+ category 精确 + enabled 筛选；走 idx_list。
func (s *FsmStateDictStore) List(ctx context.Context, q *model.FsmStateDictListQuery) ([]model.FsmStateDictListItem, int64, error) {
	where := []string{"deleted = 0"}
	args := make([]any, 0, 5)

	if q.Name != "" {
		where = append(where, "name LIKE ?")
		args = append(args, "%"+shared.EscapeLike(q.Name)+"%")
	}
	if q.DisplayName != "" {
		where = append(where, "display_name LIKE ?")
		args = append(args, "%"+shared.EscapeLike(q.DisplayName)+"%")
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
	countSQL := "SELECT COUNT(*) FROM fsm_state_dicts WHERE " + whereClause
	if err := s.db.GetContext(ctx, &total, countSQL, args...); err != nil {
		return nil, 0, fmt.Errorf("count fsm_state_dicts: %w", err)
	}

	if total == 0 {
		return make([]model.FsmStateDictListItem, 0), 0, nil
	}

	// 分页查询（按 id DESC）
	offset := (q.Page - 1) * q.PageSize
	listSQL := fmt.Sprintf(
		`SELECT id, name, display_name, category, enabled, created_at
		 FROM fsm_state_dicts WHERE %s ORDER BY id DESC LIMIT ? OFFSET ?`,
		whereClause,
	)
	listArgs := make([]any, len(args), len(args)+2)
	copy(listArgs, args)
	listArgs = append(listArgs, q.PageSize, offset)

	items := make([]model.FsmStateDictListItem, 0)
	if err := s.db.SelectContext(ctx, &items, listSQL, listArgs...); err != nil {
		return nil, 0, fmt.Errorf("list fsm_state_dicts: %w", err)
	}

	return items, total, nil
}

// Update 编辑状态字典条目（乐观锁）
//
// rows=0 → errcode.ErrVersionConflict。
func (s *FsmStateDictStore) Update(ctx context.Context, req *model.UpdateFsmStateDictRequest) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE fsm_state_dicts SET display_name = ?, category = ?, description = ?, version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ? AND deleted = 0`,
		req.DisplayName, req.Category, req.Description, time.Now(), req.ID, req.Version,
	)
	if err != nil {
		return fmt.Errorf("update fsm_state_dict: %w", err)
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

// SoftDelete 软删除状态字典条目
func (s *FsmStateDictStore) SoftDelete(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE fsm_state_dicts SET deleted = 1, updated_at = ? WHERE id = ? AND deleted = 0`,
		time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("soft delete fsm_state_dict: %w", err)
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

// ToggleEnabled 切换启用/停用（乐观锁）
func (s *FsmStateDictStore) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE fsm_state_dicts SET enabled = ?, version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ? AND deleted = 0`,
		req.Enabled, time.Now(), req.ID, req.Version,
	)
	if err != nil {
		return fmt.Errorf("toggle fsm_state_dict enabled: %w", err)
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

// GetDisplayNamesByNames 批量返回 name -> display_name 映射，用于列表页展示中文名
func (s *FsmStateDictStore) GetDisplayNamesByNames(ctx context.Context, names []string) (map[string]string, error) {
	if len(names) == 0 {
		return map[string]string{}, nil
	}
	type row struct {
		Name        string `db:"name"`
		DisplayName string `db:"display_name"`
	}
	query, args, err := sqlx.In(
		`SELECT name, display_name FROM fsm_state_dicts WHERE deleted = 0 AND name IN (?)`,
		names,
	)
	if err != nil {
		return nil, fmt.Errorf("build GetDisplayNamesByNames query: %w", err)
	}
	query = s.db.Rebind(query)
	var rows []row
	if err := s.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("GetDisplayNamesByNames: %w", err)
	}
	m := make(map[string]string, len(rows))
	for _, r := range rows {
		m[r.Name] = r.DisplayName
	}
	return m, nil
}

// ListCategories 返回所有未删除条目的分类（DISTINCT）
func (s *FsmStateDictStore) ListCategories(ctx context.Context) ([]string, error) {
	categories := make([]string, 0)
	err := s.db.SelectContext(ctx, &categories,
		`SELECT DISTINCT category FROM fsm_state_dicts WHERE deleted = 0 ORDER BY category`)
	if err != nil {
		return nil, fmt.Errorf("list fsm_state_dict categories: %w", err)
	}
	return categories, nil
}
