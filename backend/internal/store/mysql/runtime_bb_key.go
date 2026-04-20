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

// RuntimeBbKeyStore runtime_bb_keys 表操作
type RuntimeBbKeyStore struct {
	db *sqlx.DB
}

// NewRuntimeBbKeyStore 创建 RuntimeBbKeyStore
func NewRuntimeBbKeyStore(db *sqlx.DB) *RuntimeBbKeyStore {
	return &RuntimeBbKeyStore{db: db}
}

// DB 暴露数据库连接（service 层开事务用）
func (s *RuntimeBbKeyStore) DB() *sqlx.DB {
	return s.db
}

// Create 创建运行时 BB Key，返回自增 ID
//
// 对齐 FieldStore 约定：新建 enabled=0（强制 admin 明确 toggle-on 后才可被 FSM/BT 引用）。
// seed 批量写入时可用 CreateEnabled 快捷版本（cmd/seed 内置）。
func (s *RuntimeBbKeyStore) Create(ctx context.Context, req *model.CreateRuntimeBbKeyRequest) (int64, error) {
	now := time.Now()
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO runtime_bb_keys (name, type, label, description, group_name, enabled, version, deleted, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, 0, 1, 0, ?, ?)`,
		req.Name, req.Type, req.Label, req.Description, req.GroupName, now, now,
	)
	if err != nil {
		if shared.Is1062(err) {
			return 0, errcode.ErrDuplicate
		}
		return 0, fmt.Errorf("insert runtime_bb_key: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	return id, nil
}

// CreateEnabled seed 专用：批量写入的内置 key 直接 enabled=1（对齐 Server keys.go 语义）。
func (s *RuntimeBbKeyStore) CreateEnabled(ctx context.Context, req *model.CreateRuntimeBbKeyRequest) (int64, error) {
	now := time.Now()
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO runtime_bb_keys (name, type, label, description, group_name, enabled, version, deleted, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, 1, 1, 0, ?, ?)`,
		req.Name, req.Type, req.Label, req.Description, req.GroupName, now, now,
	)
	if err != nil {
		if shared.Is1062(err) {
			return 0, errcode.ErrDuplicate
		}
		return 0, fmt.Errorf("insert runtime_bb_key (enabled): %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	return id, nil
}

// GetByID 按主键查询单条详情
func (s *RuntimeBbKeyStore) GetByID(ctx context.Context, id int64) (*model.RuntimeBbKey, error) {
	var k model.RuntimeBbKey
	err := s.db.GetContext(ctx, &k,
		`SELECT id, name, type, label, description, group_name, enabled, version, deleted, created_at, updated_at
		 FROM runtime_bb_keys WHERE id = ? AND deleted = 0`, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get runtime_bb_key by id: %w", err)
	}
	return &k, nil
}

// GetByName 按 name 查询单条详情（走 uk_name）
func (s *RuntimeBbKeyStore) GetByName(ctx context.Context, name string) (*model.RuntimeBbKey, error) {
	var k model.RuntimeBbKey
	err := s.db.GetContext(ctx, &k,
		`SELECT id, name, type, label, description, group_name, enabled, version, deleted, created_at, updated_at
		 FROM runtime_bb_keys WHERE name = ? AND deleted = 0`, name)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get runtime_bb_key by name: %w", err)
	}
	return &k, nil
}

// ExistsByName 检查 name 是否已存在（含软删除行，对齐 uk_name 唯一性约束含义）
func (s *RuntimeBbKeyStore) ExistsByName(ctx context.Context, name string) (bool, error) {
	var count int
	err := s.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM runtime_bb_keys WHERE name = ?`, name)
	if err != nil {
		return false, fmt.Errorf("check runtime_bb_key name exists: %w", err)
	}
	return count > 0, nil
}

// List 分页列表查询（覆盖索引，不回表）
func (s *RuntimeBbKeyStore) List(ctx context.Context, q *model.RuntimeBbKeyListQuery) ([]model.RuntimeBbKeyListItem, int64, error) {
	where := []string{"deleted = 0"}
	args := make([]any, 0, 5)

	if q.Name != "" {
		where = append(where, "name LIKE ?")
		args = append(args, "%"+shared.EscapeLike(q.Name)+"%")
	}
	if q.Label != "" {
		where = append(where, "label LIKE ?")
		args = append(args, "%"+shared.EscapeLike(q.Label)+"%")
	}
	if q.Type != "" {
		where = append(where, "type = ?")
		args = append(args, q.Type)
	}
	if q.GroupName != "" {
		where = append(where, "group_name = ?")
		args = append(args, q.GroupName)
	}
	if q.Enabled != nil {
		where = append(where, "enabled = ?")
		args = append(args, *q.Enabled)
	}

	whereClause := strings.Join(where, " AND ")

	var total int64
	countSQL := "SELECT COUNT(*) FROM runtime_bb_keys WHERE " + whereClause
	if err := s.db.GetContext(ctx, &total, countSQL, args...); err != nil {
		return nil, 0, fmt.Errorf("count runtime_bb_keys: %w", err)
	}

	if total == 0 {
		return make([]model.RuntimeBbKeyListItem, 0), 0, nil
	}

	offset := (q.Page - 1) * q.PageSize
	listSQL := fmt.Sprintf(
		`SELECT id, name, type, label, group_name, enabled, created_at
		 FROM runtime_bb_keys WHERE %s ORDER BY id DESC LIMIT ? OFFSET ?`,
		whereClause,
	)
	listArgs := make([]any, len(args), len(args)+2)
	copy(listArgs, args)
	listArgs = append(listArgs, q.PageSize, offset)

	items := make([]model.RuntimeBbKeyListItem, 0)
	if err := s.db.SelectContext(ctx, &items, listSQL, listArgs...); err != nil {
		return nil, 0, fmt.Errorf("list runtime_bb_keys: %w", err)
	}

	return items, total, nil
}

// Update 编辑（乐观锁，按 ID）—— name 不可变，对齐 bt_trees/fsm_configs
func (s *RuntimeBbKeyStore) Update(ctx context.Context, req *model.UpdateRuntimeBbKeyRequest) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE runtime_bb_keys SET type = ?, label = ?, description = ?, group_name = ?, version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ? AND deleted = 0`,
		req.Type, req.Label, req.Description, req.GroupName, time.Now(), req.ID, req.Version,
	)
	if err != nil {
		return fmt.Errorf("update runtime_bb_key: %w", err)
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

// UpdateTx 事务内编辑（乐观锁）
func (s *RuntimeBbKeyStore) UpdateTx(ctx context.Context, tx *sqlx.Tx, req *model.UpdateRuntimeBbKeyRequest) error {
	result, err := tx.ExecContext(ctx,
		`UPDATE runtime_bb_keys SET type = ?, label = ?, description = ?, group_name = ?, version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ? AND deleted = 0`,
		req.Type, req.Label, req.Description, req.GroupName, time.Now(), req.ID, req.Version,
	)
	if err != nil {
		return fmt.Errorf("update runtime_bb_key tx: %w", err)
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

// SoftDeleteTx 事务内软删除（按 ID）
func (s *RuntimeBbKeyStore) SoftDeleteTx(ctx context.Context, tx *sqlx.Tx, id int64) error {
	result, err := tx.ExecContext(ctx,
		`UPDATE runtime_bb_keys SET deleted = 1, updated_at = ? WHERE id = ? AND deleted = 0`,
		time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("soft delete runtime_bb_key: %w", err)
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
func (s *RuntimeBbKeyStore) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE runtime_bb_keys SET enabled = ?, version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ? AND deleted = 0`,
		req.Enabled, time.Now(), req.ID, req.Version,
	)
	if err != nil {
		return fmt.Errorf("toggle runtime_bb_key enabled: %w", err)
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

// GetByIDs 批量查询（IN 查询，走主键）
func (s *RuntimeBbKeyStore) GetByIDs(ctx context.Context, ids []int64) ([]model.RuntimeBbKey, error) {
	if len(ids) == 0 {
		return make([]model.RuntimeBbKey, 0), nil
	}
	query, args, err := sqlx.In(
		`SELECT id, name, type, label, description, group_name, enabled, version, deleted, created_at, updated_at
		 FROM runtime_bb_keys WHERE id IN (?) AND deleted = 0`, ids)
	if err != nil {
		return nil, fmt.Errorf("build in query: %w", err)
	}
	query = s.db.Rebind(query)

	keys := make([]model.RuntimeBbKey, 0)
	if err := s.db.SelectContext(ctx, &keys, query, args...); err != nil {
		return nil, fmt.Errorf("get runtime_bb_keys by ids: %w", err)
	}
	return keys, nil
}

// GetByNames 批量按 name 查询（IN 查询，走 uk_name）
//
// 用途：FSM/BT 条件树 BB Key 引用追踪 —— 把 name 解析为 runtime_bb_key ID。
func (s *RuntimeBbKeyStore) GetByNames(ctx context.Context, names []string) ([]model.RuntimeBbKey, error) {
	if len(names) == 0 {
		return make([]model.RuntimeBbKey, 0), nil
	}
	query, args, err := sqlx.In(
		`SELECT id, name, type, label, description, group_name, enabled, version, deleted, created_at, updated_at
		 FROM runtime_bb_keys WHERE name IN (?) AND deleted = 0`, names)
	if err != nil {
		return nil, fmt.Errorf("build in query: %w", err)
	}
	query = s.db.Rebind(query)

	keys := make([]model.RuntimeBbKey, 0)
	if err := s.db.SelectContext(ctx, &keys, query, args...); err != nil {
		return nil, fmt.Errorf("get runtime_bb_keys by names: %w", err)
	}
	return keys, nil
}

// GetEnabledByNames 批量查询指定 name 中已启用且未删除的运行时 key 名集合
//
// 返回 map[name → true]，不在 map 中的 name 表示不存在或未启用（停用 / 软删）。
// names 为空时直接返回空 map，不发起数据库查询。
//
// 用途：FSM/BT Create/Update 时 service 层 CheckByNames 走此路径。
func (s *RuntimeBbKeyStore) GetEnabledByNames(ctx context.Context, names []string) (map[string]bool, error) {
	result := make(map[string]bool)
	if len(names) == 0 {
		return result, nil
	}

	query, args, err := sqlx.In(
		`SELECT name FROM runtime_bb_keys WHERE name IN (?) AND enabled = 1 AND deleted = 0`, names)
	if err != nil {
		return nil, fmt.Errorf("build in query: %w", err)
	}
	query = s.db.Rebind(query)

	rows := make([]string, 0)
	if err := s.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("get enabled runtime_bb_keys by names: %w", err)
	}
	for _, name := range rows {
		result[name] = true
	}
	return result, nil
}
