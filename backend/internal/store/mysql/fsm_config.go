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

// FsmConfigStore fsm_configs 表操作
//
// 严格遵守"分层职责"硬规则：只对 fsm_configs 表 CRUD。
type FsmConfigStore struct {
	db *sqlx.DB
}

// NewFsmConfigStore 创建 FsmConfigStore
func NewFsmConfigStore(db *sqlx.DB) *FsmConfigStore {
	return &FsmConfigStore{db: db}
}

// DB 暴露数据库连接（handler 层开跨模块事务用）
func (s *FsmConfigStore) DB() *sqlx.DB {
	return s.db
}

// Create 创建状态机配置，返回自增 ID
func (s *FsmConfigStore) Create(ctx context.Context, req *model.CreateFsmConfigRequest, configJSON json.RawMessage) (int64, error) {
	now := time.Now()
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO fsm_configs (name, display_name, config_json, enabled, version, created_at, updated_at, deleted)
		 VALUES (?, ?, ?, 0, 1, ?, ?, 0)`,
		req.Name, req.DisplayName, configJSON, now, now,
	)
	if err != nil {
		if shared.Is1062(err) {
			return 0, errcode.ErrDuplicate
		}
		return 0, fmt.Errorf("insert fsm_config: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	return id, nil
}

// GetByID 按主键查询状态机配置（含 config_json）
func (s *FsmConfigStore) GetByID(ctx context.Context, id int64) (*model.FsmConfig, error) {
	var fc model.FsmConfig
	err := s.db.GetContext(ctx, &fc,
		`SELECT id, name, display_name, config_json, enabled, version, created_at, updated_at, deleted
		 FROM fsm_configs WHERE id = ? AND deleted = 0`, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get fsm_config by id: %w", err)
	}
	return &fc, nil
}

// ExistsByName 检查 name 是否已存在（含软删除）
//
// 不过滤 deleted：已删除的 name 永久不可复用。
func (s *FsmConfigStore) ExistsByName(ctx context.Context, name string) (bool, error) {
	var count int
	err := s.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM fsm_configs WHERE name = ?`, name)
	if err != nil {
		return false, fmt.Errorf("check fsm_config name exists: %w", err)
	}
	return count > 0, nil
}

// List 分页列表查询
//
// 走 idx_list (deleted, enabled, id DESC)。
// 列表返回核心列 + config_json（service 层会 unmarshal 抽取展示字段）。
func (s *FsmConfigStore) List(ctx context.Context, q *model.FsmConfigListQuery) ([]model.FsmConfig, int64, error) {
	where := []string{"deleted = 0"}
	args := make([]any, 0, 4)

	if q.Name != "" {
		where = append(where, "name LIKE ?")
		args = append(args, "%"+shared.EscapeLike(q.Name)+"%")
	}
	if q.Label != "" {
		where = append(where, "display_name LIKE ?")
		args = append(args, "%"+shared.EscapeLike(q.Label)+"%")
	}
	if q.Enabled != nil {
		where = append(where, "enabled = ?")
		args = append(args, *q.Enabled)
	}

	whereClause := strings.Join(where, " AND ")

	// 计数
	var total int64
	countSQL := "SELECT COUNT(*) FROM fsm_configs WHERE " + whereClause
	if err := s.db.GetContext(ctx, &total, countSQL, args...); err != nil {
		return nil, 0, fmt.Errorf("count fsm_configs: %w", err)
	}

	if total == 0 {
		return make([]model.FsmConfig, 0), 0, nil
	}

	// 分页查询（按 id DESC）
	offset := (q.Page - 1) * q.PageSize
	listSQL := fmt.Sprintf(
		`SELECT id, name, display_name, config_json, enabled, version, created_at, updated_at
		 FROM fsm_configs WHERE %s ORDER BY id DESC LIMIT ? OFFSET ?`,
		whereClause,
	)
	listArgs := make([]any, len(args), len(args)+2)
	copy(listArgs, args)
	listArgs = append(listArgs, q.PageSize, offset)

	items := make([]model.FsmConfig, 0)
	if err := s.db.SelectContext(ctx, &items, listSQL, listArgs...); err != nil {
		return nil, 0, fmt.Errorf("list fsm_configs: %w", err)
	}

	return items, total, nil
}

// Update 编辑状态机配置（乐观锁，按 ID）
//
// rows=0 → errcode.ErrVersionConflict（version 不匹配 或 记录已删除）。
func (s *FsmConfigStore) Update(ctx context.Context, req *model.UpdateFsmConfigRequest, configJSON json.RawMessage) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE fsm_configs SET display_name = ?, config_json = ?, version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ? AND deleted = 0`,
		req.DisplayName, configJSON, time.Now(), req.ID, req.Version,
	)
	if err != nil {
		return fmt.Errorf("update fsm_config: %w", err)
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

// SoftDelete 软删除状态机配置
func (s *FsmConfigStore) SoftDelete(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE fsm_configs SET deleted = 1, updated_at = ? WHERE id = ? AND deleted = 0`,
		time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("soft delete fsm_config: %w", err)
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
func (s *FsmConfigStore) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE fsm_configs SET enabled = ?, version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ? AND deleted = 0`,
		req.Enabled, time.Now(), req.ID, req.Version,
	)
	if err != nil {
		return fmt.Errorf("toggle fsm_config enabled: %w", err)
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

// CreateTx 事务内创建状态机配置，返回自增 ID
func (s *FsmConfigStore) CreateTx(ctx context.Context, tx *sqlx.Tx, req *model.CreateFsmConfigRequest, configJSON json.RawMessage) (int64, error) {
	now := time.Now()
	result, err := tx.ExecContext(ctx,
		`INSERT INTO fsm_configs (name, display_name, config_json, enabled, version, created_at, updated_at, deleted)
		 VALUES (?, ?, ?, 0, 1, ?, ?, 0)`,
		req.Name, req.DisplayName, configJSON, now, now,
	)
	if err != nil {
		if shared.Is1062(err) {
			return 0, errcode.ErrDuplicate
		}
		return 0, fmt.Errorf("insert fsm_config tx: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	return id, nil
}

// UpdateTx 事务内编辑状态机配置（乐观锁）
func (s *FsmConfigStore) UpdateTx(ctx context.Context, tx *sqlx.Tx, req *model.UpdateFsmConfigRequest, configJSON json.RawMessage) error {
	result, err := tx.ExecContext(ctx,
		`UPDATE fsm_configs SET display_name = ?, config_json = ?, version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ? AND deleted = 0`,
		req.DisplayName, configJSON, time.Now(), req.ID, req.Version,
	)
	if err != nil {
		return fmt.Errorf("update fsm_config tx: %w", err)
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

// SoftDeleteTx 事务内软删除状态机配置
func (s *FsmConfigStore) SoftDeleteTx(ctx context.Context, tx *sqlx.Tx, id int64) error {
	result, err := tx.ExecContext(ctx,
		`UPDATE fsm_configs SET deleted = 1, updated_at = ? WHERE id = ? AND deleted = 0`,
		time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("soft delete fsm_config tx: %w", err)
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

// ListFsmConfigsReferencingState 查询所有在 config_json.states[].name 中引用了指定状态名的 FSM 配置
//
// 用于状态字典删除前的引用检查（T4 R17）。
// LIMIT 避免响应过大（最多 20 条）。
func (s *FsmConfigStore) ListFsmConfigsReferencingState(ctx context.Context, stateName string, limit int) ([]model.FsmConfigRef, error) {
	refs := make([]model.FsmConfigRef, 0)
	err := s.db.SelectContext(ctx, &refs,
		`SELECT id, name, display_name, enabled
		 FROM fsm_configs
		 WHERE deleted = 0
		   AND JSON_SEARCH(config_json, 'one', ?, NULL, '$.states[*].name') IS NOT NULL
		 LIMIT ?`,
		stateName, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list fsm_configs referencing state %q: %w", stateName, err)
	}
	return refs, nil
}

// GetByName 按 name 查询状态机配置（WHERE name=? AND deleted=0），未找到返回 nil, nil
func (s *FsmConfigStore) GetByName(ctx context.Context, name string) (*model.FsmConfig, error) {
	var fc model.FsmConfig
	err := s.db.GetContext(ctx, &fc,
		`SELECT id, name, display_name, config_json, enabled, version, created_at, updated_at, deleted
		 FROM fsm_configs WHERE name = ? AND deleted = 0`, name)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get fsm_config by name: %w", err)
	}
	return &fc, nil
}

// ExportAll 导出所有已启用且未删除的状态机配置
//
// 返回 (name, config_json) 二元组，handler 层原样输出到 HTTP 响应。
func (s *FsmConfigStore) ExportAll(ctx context.Context) ([]model.FsmConfigExportItem, error) {
	items := make([]model.FsmConfigExportItem, 0)
	err := s.db.SelectContext(ctx, &items,
		`SELECT name, config_json AS config FROM fsm_configs WHERE deleted = 0 AND enabled = 1 ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("export fsm_configs: %w", err)
	}
	return items, nil
}
