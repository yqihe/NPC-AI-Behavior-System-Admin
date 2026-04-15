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

// BtTreeStore bt_trees 表操作
//
// 严格遵守"分层职责"硬规则：只对 bt_trees 表 CRUD。
type BtTreeStore struct {
	db *sqlx.DB
}

// NewBtTreeStore 创建 BtTreeStore
func NewBtTreeStore(db *sqlx.DB) *BtTreeStore {
	return &BtTreeStore{db: db}
}

// Create 插入新行为树，唯一冲突返回 errcode.ErrDuplicate
func (s *BtTreeStore) Create(ctx context.Context, req *model.CreateBtTreeRequest) (int64, error) {
	now := time.Now()
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO bt_trees (name, display_name, description, config, enabled, version, created_at, updated_at, deleted)
		 VALUES (?, ?, ?, ?, 0, 1, ?, ?, 0)`,
		req.Name, req.DisplayName, req.Description, req.Config, now, now,
	)
	if err != nil {
		if shared.Is1062(err) {
			return 0, errcode.ErrDuplicate
		}
		return 0, fmt.Errorf("insert bt_tree: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	return id, nil
}

// GetByID 按主键查询行为树（含 config），未找到返回 errcode.ErrNotFound
func (s *BtTreeStore) GetByID(ctx context.Context, id int64) (*model.BtTree, error) {
	var bt model.BtTree
	err := s.db.GetContext(ctx, &bt,
		`SELECT id, name, display_name, description, config, enabled, version, created_at, updated_at, deleted
		 FROM bt_trees WHERE id = ? AND deleted = 0`, id)
	if err == sql.ErrNoRows {
		return nil, errcode.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get bt_tree by id: %w", err)
	}
	return &bt, nil
}

// ExistsByName 检查 name 是否已存在（含软删除行）
//
// 不过滤 deleted：已删除的 name 永久不可复用。
func (s *BtTreeStore) ExistsByName(ctx context.Context, name string) (bool, error) {
	var count int
	err := s.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM bt_trees WHERE name = ?`, name)
	if err != nil {
		return false, fmt.Errorf("check bt_tree name exists: %w", err)
	}
	return count > 0, nil
}

// List 分页列表查询
//
// name 前缀匹配，display_name 两端模糊匹配，enabled 精确筛选。
// 列表只取核心列，不返回 config（减少传输量）。
func (s *BtTreeStore) List(ctx context.Context, q *model.BtTreeListQuery) ([]model.BtTreeListItem, int64, error) {
	where := []string{"deleted = 0"}
	args := make([]any, 0, 4)

	if q.Name != "" {
		where = append(where, "name LIKE ?")
		args = append(args, shared.EscapeLike(q.Name)+"%")
	}
	if q.DisplayName != "" {
		where = append(where, "display_name LIKE ?")
		args = append(args, "%"+shared.EscapeLike(q.DisplayName)+"%")
	}
	if q.Enabled != nil {
		where = append(where, "enabled = ?")
		args = append(args, *q.Enabled)
	}

	whereClause := strings.Join(where, " AND ")

	// 计数
	var total int64
	countSQL := "SELECT COUNT(*) FROM bt_trees WHERE " + whereClause
	if err := s.db.GetContext(ctx, &total, countSQL, args...); err != nil {
		return nil, 0, fmt.Errorf("count bt_trees: %w", err)
	}

	if total == 0 {
		return make([]model.BtTreeListItem, 0), 0, nil
	}

	// 分页查询（按 id DESC）
	offset := (q.Page - 1) * q.PageSize
	listSQL := fmt.Sprintf(
		`SELECT id, name, display_name, enabled, created_at
		 FROM bt_trees WHERE %s ORDER BY id DESC LIMIT ? OFFSET ?`,
		whereClause,
	)
	listArgs := make([]any, len(args), len(args)+2)
	copy(listArgs, args)
	listArgs = append(listArgs, q.PageSize, offset)

	items := make([]model.BtTreeListItem, 0)
	if err := s.db.SelectContext(ctx, &items, listSQL, listArgs...); err != nil {
		return nil, 0, fmt.Errorf("list bt_trees: %w", err)
	}

	return items, total, nil
}

// Update 编辑行为树（乐观锁，按 ID）
//
// rows=0 → errcode.ErrVersionConflict（version 不匹配 或 记录已删除）。
func (s *BtTreeStore) Update(ctx context.Context, req *model.UpdateBtTreeRequest) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE bt_trees SET display_name = ?, description = ?, config = ?, version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ? AND deleted = 0`,
		req.DisplayName, req.Description, req.Config, time.Now(), req.ID, req.Version,
	)
	if err != nil {
		return fmt.Errorf("update bt_tree: %w", err)
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

// Delete 软删除行为树（乐观锁，按 ID）
//
// rows=0 → errcode.ErrVersionConflict（version 不匹配 或 记录已删除）。
func (s *BtTreeStore) Delete(ctx context.Context, id int64, version int) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE bt_trees SET deleted = 1, updated_at = ? WHERE id = ? AND version = ? AND deleted = 0`,
		time.Now(), id, version,
	)
	if err != nil {
		return fmt.Errorf("soft delete bt_tree: %w", err)
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

// ToggleEnabled 切换启用/停用（乐观锁，按 ID）
func (s *BtTreeStore) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE bt_trees SET enabled = ?, version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ? AND deleted = 0`,
		req.Enabled, time.Now(), req.ID, req.Version,
	)
	if err != nil {
		return fmt.Errorf("toggle bt_tree enabled: %w", err)
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

// ExportAll 导出所有已启用且未删除的行为树
//
// 返回 (name, config) 二元组，handler 层原样输出到 HTTP 响应。
func (s *BtTreeStore) ExportAll(ctx context.Context) ([]model.BtTreeExportItem, error) {
	items := make([]model.BtTreeExportItem, 0)
	err := s.db.SelectContext(ctx, &items,
		`SELECT name, config FROM bt_trees WHERE deleted = 0 AND enabled = 1 ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("export bt_trees: %w", err)
	}
	return items, nil
}
