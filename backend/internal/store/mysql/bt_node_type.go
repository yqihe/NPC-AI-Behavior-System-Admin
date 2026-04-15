package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	shared "github.com/yqihe/npc-ai-admin/backend/internal/store/mysql/shared"
)

// BtNodeTypeStore bt_node_types 表操作
type BtNodeTypeStore struct {
	db *sqlx.DB
}

// NewBtNodeTypeStore 创建 BtNodeTypeStore
func NewBtNodeTypeStore(db *sqlx.DB) *BtNodeTypeStore {
	return &BtNodeTypeStore{db: db}
}

// Create 插入新节点类型，is_builtin 固定为 0；唯一冲突返回 errcode.ErrDuplicate
func (s *BtNodeTypeStore) Create(ctx context.Context, req *model.CreateBtNodeTypeRequest) (int64, error) {
	now := time.Now()
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO bt_node_types (type_name, category, label, description, param_schema, is_builtin, enabled, version, created_at, updated_at, deleted)
		 VALUES (?, ?, ?, ?, ?, 0, 1, 1, ?, ?, 0)`,
		req.TypeName, req.Category, req.Label, req.Description, req.ParamSchema, now, now,
	)
	if err != nil {
		if shared.Is1062(err) {
			return 0, errcode.ErrDuplicate
		}
		return 0, fmt.Errorf("insert bt_node_type: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	return id, nil
}

// GetByID 按主键查询节点类型（deleted=0），未找到返回 errcode.ErrNotFound
func (s *BtNodeTypeStore) GetByID(ctx context.Context, id int64) (*model.BtNodeType, error) {
	var t model.BtNodeType
	err := s.db.GetContext(ctx, &t,
		`SELECT id, type_name, category, label, description, param_schema, is_builtin, enabled, version, created_at, updated_at, deleted
		 FROM bt_node_types WHERE id = ? AND deleted = 0`, id)
	if err == sql.ErrNoRows {
		return nil, errcode.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get bt_node_type by id: %w", err)
	}
	return &t, nil
}

// ExistsByTypeName 检查 type_name 是否已存在（含软删除行）
func (s *BtNodeTypeStore) ExistsByTypeName(ctx context.Context, typeName string) (bool, error) {
	var count int
	err := s.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM bt_node_types WHERE type_name = ?`, typeName)
	if err != nil {
		return false, fmt.Errorf("check bt_node_type type_name exists: %w", err)
	}
	return count > 0, nil
}

// List 分页列表，支持 type_name 前缀匹配、category 精确匹配、enabled 筛选
func (s *BtNodeTypeStore) List(ctx context.Context, q *model.BtNodeTypeListQuery) ([]model.BtNodeTypeListItem, int64, error) {
	where := []string{"deleted = 0"}
	args := make([]any, 0, 5)

	if q.TypeName != "" {
		escaped := shared.EscapeLike(q.TypeName)
		where = append(where, "type_name LIKE ?")
		args = append(args, escaped+"%")
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
	countSQL := "SELECT COUNT(*) FROM bt_node_types WHERE " + whereClause
	if err := s.db.GetContext(ctx, &total, countSQL, args...); err != nil {
		return nil, 0, fmt.Errorf("count bt_node_types: %w", err)
	}

	if total == 0 {
		return make([]model.BtNodeTypeListItem, 0), 0, nil
	}

	// 分页查询（按 id DESC）
	offset := (q.Page - 1) * q.PageSize
	listSQL := fmt.Sprintf(
		`SELECT id, type_name, category, label, is_builtin, enabled
		 FROM bt_node_types WHERE %s ORDER BY id DESC LIMIT ? OFFSET ?`,
		whereClause,
	)
	listArgs := make([]any, len(args), len(args)+2)
	copy(listArgs, args)
	listArgs = append(listArgs, q.PageSize, offset)

	items := make([]model.BtNodeTypeListItem, 0)
	if err := s.db.SelectContext(ctx, &items, listSQL, listArgs...); err != nil {
		return nil, 0, fmt.Errorf("list bt_node_types: %w", err)
	}

	return items, total, nil
}

// Update 乐观锁更新（仅 label/description/param_schema），0 rows → errcode.ErrVersionConflict
func (s *BtNodeTypeStore) Update(ctx context.Context, req *model.UpdateBtNodeTypeRequest) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE bt_node_types SET label = ?, description = ?, param_schema = ?, version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ? AND deleted = 0`,
		req.Label, req.Description, req.ParamSchema, time.Now(), req.ID, req.Version,
	)
	if err != nil {
		return fmt.Errorf("update bt_node_type: %w", err)
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

// Delete 软删除（deleted=1）+ 乐观锁，0 rows → errcode.ErrVersionConflict
func (s *BtNodeTypeStore) Delete(ctx context.Context, id int64, version int) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE bt_node_types SET deleted = 1, updated_at = ?
		 WHERE id = ? AND version = ? AND deleted = 0`,
		time.Now(), id, version,
	)
	if err != nil {
		return fmt.Errorf("soft delete bt_node_type: %w", err)
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

// ToggleEnabled 乐观锁切换 enabled，0 rows → errcode.ErrVersionConflict
func (s *BtNodeTypeStore) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE bt_node_types SET enabled = ?, version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ? AND deleted = 0`,
		req.Enabled, time.Now(), req.ID, req.Version,
	)
	if err != nil {
		return fmt.Errorf("toggle bt_node_type enabled: %w", err)
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

// ListEnabledTypes 返回所有 enabled=1 AND deleted=0 的 type_name → category map（节点树校验用）
func (s *BtNodeTypeStore) ListEnabledTypes(ctx context.Context) (map[string]string, error) {
	type row struct {
		TypeName string `db:"type_name"`
		Category string `db:"category"`
	}
	var rows []row
	if err := s.db.SelectContext(ctx, &rows,
		`SELECT type_name, category FROM bt_node_types WHERE enabled = 1 AND deleted = 0`); err != nil {
		return nil, fmt.Errorf("list enabled bt_node_types: %w", err)
	}
	m := make(map[string]string, len(rows))
	for _, r := range rows {
		m[r.TypeName] = r.Category
	}
	return m, nil
}
