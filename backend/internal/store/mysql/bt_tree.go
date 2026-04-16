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

// BtTreeStore bt_trees 及 bt_node_type_refs 表操作
type BtTreeStore struct {
	db *sqlx.DB
}

// NewBtTreeStore 创建 BtTreeStore
func NewBtTreeStore(db *sqlx.DB) *BtTreeStore {
	return &BtTreeStore{db: db}
}

// DB 暴露数据库连接（service 层开事务用）
func (s *BtTreeStore) DB() *sqlx.DB {
	return s.db
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

// GetByID 按主键查询行为树（含 config），未找到返回 nil, nil
func (s *BtTreeStore) GetByID(ctx context.Context, id int64) (*model.BtTree, error) {
	var bt model.BtTree
	err := s.db.GetContext(ctx, &bt,
		`SELECT id, name, display_name, description, config, enabled, version, created_at, updated_at, deleted
		 FROM bt_trees WHERE id = ? AND deleted = 0`, id)
	if err == sql.ErrNoRows {
		return nil, nil
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

// SoftDelete 软删除行为树，0 rows → errcode.ErrNotFound
func (s *BtTreeStore) SoftDelete(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE bt_trees SET deleted = 1, updated_at = ? WHERE id = ? AND deleted = 0`,
		time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("soft delete bt_tree: %w", err)
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

// walkNodes 递归遍历节点树，对每个节点调用 visit。
//
// 节点结构：{"type": "...", "children": [...], "child": {...}, ...}
// composite 节点用 children（数组），decorator 节点用 child（单节点），leaf 节点两者均无。
func walkNodes(node map[string]any, visit func(map[string]any)) {
	visit(node)

	// composite: children 数组
	if raw, ok := node["children"]; ok {
		if arr, ok := raw.([]any); ok {
			for _, item := range arr {
				if child, ok := item.(map[string]any); ok {
					walkNodes(child, visit)
				}
			}
		}
	}

	// decorator: child 单节点
	if raw, ok := node["child"]; ok {
		if child, ok := raw.(map[string]any); ok {
			walkNodes(child, visit)
		}
	}
}

// GetNodeTypeUsages 返回使用指定节点类型的行为树 name 列表。
//
// 走 bt_node_type_refs.idx_type_name 索引，O(引用数)，替代原 JSON_SEARCH 全表扫。
func (s *BtTreeStore) GetNodeTypeUsages(ctx context.Context, typeName string) ([]string, error) {
	names := make([]string, 0)
	err := s.db.SelectContext(ctx, &names,
		`SELECT bt.name
		 FROM bt_trees bt
		 INNER JOIN bt_node_type_refs r ON r.bt_tree_id = bt.id
		 WHERE r.type_name = ? AND bt.deleted = 0`,
		typeName,
	)
	if err != nil {
		return nil, fmt.Errorf("get bt_tree usages for node type %q: %w", typeName, err)
	}
	return names, nil
}

// extractBBKeys 递归提取节点树中所有 bb_key 类型参数的值。
//
// nodeParamTypes: type_name → 该类型下 param_schema 中 type=bb_key 的参数名列表。
// 由调用方（service 层）从 bt_node_types 表预加载，避免 store 间循环依赖。
func extractBBKeys(node map[string]any, nodeParamTypes map[string][]string) []string {
	keys := make([]string, 0)
	walkNodes(node, func(n map[string]any) {
		typeName, ok := n["type"].(string)
		if !ok {
			return
		}
		bbParamNames, ok := nodeParamTypes[typeName]
		if !ok {
			return
		}
		for _, paramName := range bbParamNames {
			if val, ok := n[paramName].(string); ok && val != "" {
				keys = append(keys, val)
			}
		}
	})
	return keys
}

// IsBBKeyUsed 检查指定 BB Key 是否被任意行为树的节点引用。
//
// nodeParamTypes: type_name → bb_key 参数名列表，由调用方预加载。
// 全量扫描 deleted=0 的 bt_trees.config；json.Unmarshal 失败时返回 error，不跳过。
func (s *BtTreeStore) IsBBKeyUsed(ctx context.Context, bbKey string, nodeParamTypes map[string][]string) (bool, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT config FROM bt_trees WHERE deleted = 0`)
	if err != nil {
		return false, fmt.Errorf("query bt_trees for bb key check: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var configStr string
		if err := rows.Scan(&configStr); err != nil {
			return false, fmt.Errorf("scan bt_tree config: %w", err)
		}
		var root map[string]any
		if err := json.Unmarshal([]byte(configStr), &root); err != nil {
			return false, fmt.Errorf("unmarshal bt_tree config: %w", err)
		}
		for _, k := range extractBBKeys(root, nodeParamTypes) {
			if k == bbKey {
				return true, nil
			}
		}
	}
	if err := rows.Err(); err != nil {
		return false, fmt.Errorf("iterate bt_trees: %w", err)
	}
	return false, nil
}

// GetBBKeyUsages 返回引用指定 BB Key 的行为树 name 列表。
//
// nodeParamTypes: type_name → bb_key 参数名列表，由调用方预加载。
// 全量扫描 deleted=0 的 bt_trees；json.Unmarshal 失败时返回 error，不跳过。
func (s *BtTreeStore) GetBBKeyUsages(ctx context.Context, bbKey string, nodeParamTypes map[string][]string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT name, config FROM bt_trees WHERE deleted = 0`)
	if err != nil {
		return nil, fmt.Errorf("query bt_trees for bb key usages: %w", err)
	}
	defer rows.Close()

	names := make([]string, 0)
	for rows.Next() {
		var name, configStr string
		if err := rows.Scan(&name, &configStr); err != nil {
			return nil, fmt.Errorf("scan bt_tree row: %w", err)
		}
		var root map[string]any
		if err := json.Unmarshal([]byte(configStr), &root); err != nil {
			return nil, fmt.Errorf("unmarshal bt_tree config (name=%s): %w", name, err)
		}
		for _, k := range extractBBKeys(root, nodeParamTypes) {
			if k == bbKey {
				names = append(names, name)
				break
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bt_trees: %w", err)
	}
	return names, nil
}

// ---- bt_node_type_refs 维护 ----

// extractTypeNamesFromConfig 从 config JSON 中提取所有节点类型名（去重）。
func extractTypeNamesFromConfig(config json.RawMessage) ([]string, error) {
	var root map[string]any
	if err := json.Unmarshal(config, &root); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	seen := make(map[string]bool)
	walkNodes(root, func(node map[string]any) {
		if t, ok := node["type"].(string); ok && t != "" {
			seen[t] = true
		}
	})
	names := make([]string, 0, len(seen))
	for t := range seen {
		names = append(names, t)
	}
	return names, nil
}

// SyncNodeTypeRefsTx 替换行为树的节点类型引用（事务内：先全删再批量插入）。
//
// 在 Create/Update 时调用，保证 bt_node_type_refs 与 bt_trees.config 一致。
func (s *BtTreeStore) SyncNodeTypeRefsTx(ctx context.Context, tx *sqlx.Tx, btTreeID int64, config json.RawMessage) error {
	typeNames, err := extractTypeNamesFromConfig(config)
	if err != nil {
		return fmt.Errorf("extract type names: %w", err)
	}

	// 先删旧引用
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM bt_node_type_refs WHERE bt_tree_id = ?`, btTreeID); err != nil {
		return fmt.Errorf("delete bt_node_type_refs: %w", err)
	}

	// 批量插入新引用
	if len(typeNames) == 0 {
		return nil
	}
	placeholders := strings.Repeat("(?,?),", len(typeNames))
	placeholders = placeholders[:len(placeholders)-1] // 去掉末尾逗号
	args := make([]any, 0, len(typeNames)*2)
	for _, t := range typeNames {
		args = append(args, btTreeID, t)
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO bt_node_type_refs (bt_tree_id, type_name) VALUES "+placeholders,
		args...,
	); err != nil {
		return fmt.Errorf("insert bt_node_type_refs: %w", err)
	}
	return nil
}

// DeleteNodeTypeRefsTx 删除行为树的所有节点类型引用（软删除时调用）。
func (s *BtTreeStore) DeleteNodeTypeRefsTx(ctx context.Context, tx *sqlx.Tx, btTreeID int64) error {
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM bt_node_type_refs WHERE bt_tree_id = ?`, btTreeID); err != nil {
		return fmt.Errorf("delete bt_node_type_refs: %w", err)
	}
	return nil
}

// ---- 事务版方法（service 层单模块事务用）----

// CreateInTx 事务内插入行为树，返回自增 ID
func (s *BtTreeStore) CreateInTx(ctx context.Context, tx *sqlx.Tx, req *model.CreateBtTreeRequest) (int64, error) {
	now := time.Now()
	result, err := tx.ExecContext(ctx,
		`INSERT INTO bt_trees (name, display_name, description, config, enabled, version, created_at, updated_at, deleted)
		 VALUES (?, ?, ?, ?, 0, 1, ?, ?, 0)`,
		req.Name, req.DisplayName, req.Description, req.Config, now, now,
	)
	if err != nil {
		if shared.Is1062(err) {
			return 0, errcode.ErrDuplicate
		}
		return 0, fmt.Errorf("insert bt_tree tx: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	return id, nil
}

// UpdateInTx 事务内编辑行为树（乐观锁）
func (s *BtTreeStore) UpdateInTx(ctx context.Context, tx *sqlx.Tx, req *model.UpdateBtTreeRequest) error {
	result, err := tx.ExecContext(ctx,
		`UPDATE bt_trees SET display_name = ?, description = ?, config = ?, version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ? AND deleted = 0`,
		req.DisplayName, req.Description, req.Config, time.Now(), req.ID, req.Version,
	)
	if err != nil {
		return fmt.Errorf("update bt_tree tx: %w", err)
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

// SoftDeleteInTx 事务内软删除行为树
func (s *BtTreeStore) SoftDeleteInTx(ctx context.Context, tx *sqlx.Tx, id int64) error {
	result, err := tx.ExecContext(ctx,
		`UPDATE bt_trees SET deleted = 1, updated_at = ? WHERE id = ? AND deleted = 0`,
		time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("soft delete bt_tree tx: %w", err)
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
