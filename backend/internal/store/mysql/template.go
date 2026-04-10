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

// TemplateStore templates 表操作
//
// 严格遵守"分层职责"硬规则：只对 templates 表 CRUD，
// 不读写 fields / field_refs 等其它模块的表。
type TemplateStore struct {
	db *sqlx.DB
}

// NewTemplateStore 创建 TemplateStore
func NewTemplateStore(db *sqlx.DB) *TemplateStore {
	return &TemplateStore{db: db}
}

// DB 暴露数据库连接（handler 层开跨模块事务用）
func (s *TemplateStore) DB() *sqlx.DB {
	return s.db
}

// CreateTx 事务内创建模板
//
// 模板创建永远是跨模块事务的一部分（同时要写 field_refs + bump fields.ref_count），
// 所以只提供 Tx 版本，由 handler 层开启事务。
func (s *TemplateStore) CreateTx(ctx context.Context, tx *sqlx.Tx, req *model.CreateTemplateRequest, fieldsJSON []byte) (int64, error) {
	now := time.Now()
	result, err := tx.ExecContext(ctx,
		`INSERT INTO templates (name, label, description, fields, ref_count, enabled, version, deleted, created_at, updated_at)
		 VALUES (?, ?, ?, ?, 0, 0, 1, 0, ?, ?)`,
		req.Name, req.Label, req.Description, fieldsJSON, now, now,
	)
	if err != nil {
		return 0, fmt.Errorf("insert template: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	return id, nil
}

// GetByID 按主键查询模板
func (s *TemplateStore) GetByID(ctx context.Context, id int64) (*model.Template, error) {
	var t model.Template
	err := s.db.GetContext(ctx, &t,
		`SELECT id, name, label, description, fields, ref_count, enabled, version, deleted, created_at, updated_at
		 FROM templates WHERE id = ? AND deleted = 0`, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get template by id: %w", err)
	}
	return &t, nil
}

// ExistsByName 检查 name 是否已存在（含软删除）
//
// 已删除的 name 永久不可复用，防止历史 NPC 引用混乱。
func (s *TemplateStore) ExistsByName(ctx context.Context, name string) (bool, error) {
	var count int
	err := s.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM templates WHERE name = ?`, name)
	if err != nil {
		return false, fmt.Errorf("check template name exists: %w", err)
	}
	return count > 0, nil
}

// List 分页列表查询
//
// 走覆盖索引 idx_list (deleted, id, name, label, ref_count, enabled, created_at)，
// 返回 TemplateListItem（不含 fields/description，减小网络传输）。
func (s *TemplateStore) List(ctx context.Context, q *model.TemplateListQuery) ([]model.TemplateListItem, int64, error) {
	where := []string{"deleted = 0"}
	args := make([]any, 0, 2)

	if q.Label != "" {
		where = append(where, "label LIKE ?")
		args = append(args, "%"+escapeLike(q.Label)+"%")
	}
	if q.Enabled != nil {
		where = append(where, "enabled = ?")
		args = append(args, *q.Enabled)
	}

	whereClause := strings.Join(where, " AND ")

	// 计数
	var total int64
	countSQL := "SELECT COUNT(*) FROM templates WHERE " + whereClause
	if err := s.db.GetContext(ctx, &total, countSQL, args...); err != nil {
		return nil, 0, fmt.Errorf("count templates: %w", err)
	}

	if total == 0 {
		return make([]model.TemplateListItem, 0), 0, nil
	}

	// 分页查询（覆盖索引，按 id DESC）
	offset := (q.Page - 1) * q.PageSize
	listSQL := fmt.Sprintf(
		`SELECT id, name, label, ref_count, enabled, created_at
		 FROM templates WHERE %s ORDER BY id DESC LIMIT ? OFFSET ?`,
		whereClause,
	)
	listArgs := make([]any, len(args), len(args)+2)
	copy(listArgs, args)
	listArgs = append(listArgs, q.PageSize, offset)

	items := make([]model.TemplateListItem, 0)
	if err := s.db.SelectContext(ctx, &items, listSQL, listArgs...); err != nil {
		return nil, 0, fmt.Errorf("list templates: %w", err)
	}

	return items, total, nil
}

// UpdateTx 事务内编辑模板（乐观锁，按 ID）
//
// rows=0 → ErrVersionConflict（version 不匹配 或 记录已删除）。
// service 层先 GetByID 预检查，rows=0 即视为版本冲突。
func (s *TemplateStore) UpdateTx(ctx context.Context, tx *sqlx.Tx, req *model.UpdateTemplateRequest, fieldsJSON []byte) error {
	result, err := tx.ExecContext(ctx,
		`UPDATE templates SET label = ?, description = ?, fields = ?, version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ? AND deleted = 0`,
		req.Label, req.Description, fieldsJSON, time.Now(), req.ID, req.Version,
	)
	if err != nil {
		return fmt.Errorf("update template: %w", err)
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

// SoftDeleteTx 事务内软删除模板
func (s *TemplateStore) SoftDeleteTx(ctx context.Context, tx *sqlx.Tx, id int64) error {
	result, err := tx.ExecContext(ctx,
		`UPDATE templates SET deleted = 1, updated_at = ? WHERE id = ? AND deleted = 0`,
		time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("soft delete template: %w", err)
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
//
// 单模块写，不需要事务（不联动其它表）。
func (s *TemplateStore) ToggleEnabled(ctx context.Context, id int64, enabled bool, version int) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE templates SET enabled = ?, version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ? AND deleted = 0`,
		enabled, time.Now(), id, version,
	)
	if err != nil {
		return fmt.Errorf("toggle template enabled: %w", err)
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

// IncrRefCountTx 事务内 ref_count + 1（NPC 模块创建时调用）
func (s *TemplateStore) IncrRefCountTx(ctx context.Context, tx *sqlx.Tx, id int64) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE templates SET ref_count = ref_count + 1 WHERE id = ? AND deleted = 0`, id)
	if err != nil {
		return fmt.Errorf("incr template ref count: %w", err)
	}
	return nil
}

// DecrRefCountTx 事务内 ref_count - 1（NPC 模块删除时调用）
func (s *TemplateStore) DecrRefCountTx(ctx context.Context, tx *sqlx.Tx, id int64) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE templates SET ref_count = ref_count - 1 WHERE id = ? AND deleted = 0 AND ref_count > 0`, id)
	if err != nil {
		return fmt.Errorf("decr template ref count: %w", err)
	}
	return nil
}

// GetRefCountTx 事务内获取引用计数（FOR SHARE 防 TOCTOU）
//
// 删除前必须用此方法在事务内重新读取 ref_count，
// 防止"读时无引用 → NPC 模块刚好新建引用 → 仍然删除"的竞态。
func (s *TemplateStore) GetRefCountTx(ctx context.Context, tx *sqlx.Tx, id int64) (int, error) {
	var count int
	err := tx.GetContext(ctx, &count,
		`SELECT ref_count FROM templates WHERE id = ? AND deleted = 0 FOR SHARE`, id)
	if err == sql.ErrNoRows {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("get template ref count tx: %w", err)
	}
	return count, nil
}

// GetByIDs 批量查询模板精简信息（IN 查询，走主键）
//
// 给字段管理 GetReferences 跨模块编排时补 template label 用。
func (s *TemplateStore) GetByIDs(ctx context.Context, ids []int64) ([]model.TemplateLite, error) {
	if len(ids) == 0 {
		return make([]model.TemplateLite, 0), nil
	}
	query, args, err := sqlx.In(
		`SELECT id, name, label FROM templates WHERE id IN (?) AND deleted = 0`, ids)
	if err != nil {
		return nil, fmt.Errorf("build in query: %w", err)
	}
	query = s.db.Rebind(query)

	templates := make([]model.TemplateLite, 0)
	if err := s.db.SelectContext(ctx, &templates, query, args...); err != nil {
		return nil, fmt.Errorf("get templates by ids: %w", err)
	}
	return templates, nil
}
