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

// NpcStore npcs 及 npc_bt_refs 表操作
//
// 严格遵守"分层职责"硬规则：只对 npcs / npc_bt_refs 表 CRUD，
// 不读写 templates / fsm_configs / bt_trees 等其它模块的表。
type NpcStore struct {
	db *sqlx.DB
}

// NewNpcStore 创建 NpcStore
func NewNpcStore(db *sqlx.DB) *NpcStore {
	return &NpcStore{db: db}
}

// DB 暴露数据库连接（service 层开事务用）
func (s *NpcStore) DB() *sqlx.DB {
	return s.db
}

// ──────────────────────────────────────────────
// 读操作
// ──────────────────────────────────────────────

// GetByID 按主键查询 NPC，未找到返回 nil, nil
func (s *NpcStore) GetByID(ctx context.Context, id int64) (*model.NPC, error) {
	var n model.NPC
	err := s.db.GetContext(ctx, &n,
		`SELECT id, name, label, description, template_id, template_name, fields, fsm_ref, bt_refs,
		        enabled, version, created_at, updated_at, deleted
		 FROM npcs WHERE id = ? AND deleted = 0`, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get npc by id: %w", err)
	}
	return &n, nil
}

// ExistsByName 检查 name 是否已存在（含软删除行）
//
// 不过滤 deleted：已删除的 name 永久不可复用。
func (s *NpcStore) ExistsByName(ctx context.Context, name string) (bool, error) {
	var count int
	err := s.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM npcs WHERE name = ?`, name)
	if err != nil {
		return false, fmt.Errorf("check npc name exists: %w", err)
	}
	return count > 0, nil
}

// List 分页列表查询
//
// label/name 两端模糊匹配，template_name 精确匹配，enabled 三态筛选。
// 走覆盖索引 idx_list (deleted, id, name, label, template_name, enabled, created_at)。
// 列表只取核心列，不返回 fields/bt_refs/description（减少传输量）。
func (s *NpcStore) List(ctx context.Context, q *model.NPCListQuery) ([]model.NPCListItem, int64, error) {
	where := []string{"deleted = 0"}
	args := make([]any, 0, 5)

	if q.Label != "" {
		where = append(where, "label LIKE ?")
		args = append(args, "%"+shared.EscapeLike(q.Label)+"%")
	}
	if q.Name != "" {
		where = append(where, "name LIKE ?")
		args = append(args, "%"+shared.EscapeLike(q.Name)+"%")
	}
	if q.TemplateName != "" {
		where = append(where, "template_name = ?")
		args = append(args, q.TemplateName)
	}
	if q.Enabled != nil {
		where = append(where, "enabled = ?")
		args = append(args, *q.Enabled)
	}

	whereClause := strings.Join(where, " AND ")

	// 计数
	var total int64
	countSQL := "SELECT COUNT(*) FROM npcs WHERE " + whereClause
	if err := s.db.GetContext(ctx, &total, countSQL, args...); err != nil {
		return nil, 0, fmt.Errorf("count npcs: %w", err)
	}

	if total == 0 {
		return make([]model.NPCListItem, 0), 0, nil
	}

	// 分页查询（按 id DESC）
	// fsm_ref 不在覆盖索引内，但 WHERE 子句仍命中索引，额外回表可接受
	offset := (q.Page - 1) * q.PageSize
	listSQL := fmt.Sprintf(
		`SELECT id, name, label, template_id, template_name, fsm_ref, enabled, created_at
		 FROM npcs WHERE %s ORDER BY id DESC LIMIT ? OFFSET ?`,
		whereClause,
	)
	listArgs := make([]any, len(args), len(args)+2)
	copy(listArgs, args)
	listArgs = append(listArgs, q.PageSize, offset)

	items := make([]model.NPCListItem, 0)
	if err := s.db.SelectContext(ctx, &items, listSQL, listArgs...); err != nil {
		return nil, 0, fmt.Errorf("list npcs: %w", err)
	}

	return items, total, nil
}

// CountByTemplateID 按 template_id 统计引用数（供 TemplateHandler Delete/EditFields 使用）
func (s *NpcStore) CountByTemplateID(ctx context.Context, templateID int64) (int64, error) {
	var count int64
	err := s.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM npcs WHERE template_id = ? AND deleted = 0`, templateID)
	if err != nil {
		return 0, fmt.Errorf("count npcs by template_id: %w", err)
	}
	return count, nil
}

// CountByBtTreeName 按行为树名统计引用数（供 BtTreeHandler Delete 使用）
//
// 走 npc_bt_refs.idx_bt_name 索引，替代 JSON_SEARCH 全表扫。
func (s *NpcStore) CountByBtTreeName(ctx context.Context, btName string) (int64, error) {
	var count int64
	err := s.db.GetContext(ctx, &count,
		`SELECT COUNT(*)
		 FROM npc_bt_refs r
		 INNER JOIN npcs n ON r.npc_id = n.id
		 WHERE r.bt_tree_name = ? AND n.deleted = 0`,
		btName,
	)
	if err != nil {
		return 0, fmt.Errorf("count npcs by bt_tree_name: %w", err)
	}
	return count, nil
}

// CountByFsmRef 按 fsm_ref 统计引用数（供 FsmConfigHandler Delete 使用）
//
// 走 idx_fsm (fsm_ref, deleted) 索引。
func (s *NpcStore) CountByFsmRef(ctx context.Context, fsmName string) (int64, error) {
	var count int64
	err := s.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM npcs WHERE fsm_ref = ? AND deleted = 0`, fsmName)
	if err != nil {
		return 0, fmt.Errorf("count npcs by fsm_ref: %w", err)
	}
	return count, nil
}

// ListByTemplateID 分页查询引用了指定模板的 NPC 精简列表（供 TemplateHandler GetReferences 使用）
func (s *NpcStore) ListByTemplateID(ctx context.Context, templateID int64, page, pageSize int) ([]model.NPCLite, int64, error) {
	var total int64
	if err := s.db.GetContext(ctx, &total,
		`SELECT COUNT(*) FROM npcs WHERE template_id = ? AND deleted = 0`, templateID); err != nil {
		return nil, 0, fmt.Errorf("count npcs by template_id: %w", err)
	}

	if total == 0 {
		return make([]model.NPCLite, 0), 0, nil
	}

	offset := (page - 1) * pageSize
	items := make([]model.NPCLite, 0)
	if err := s.db.SelectContext(ctx, &items,
		`SELECT id, name, label FROM npcs WHERE template_id = ? AND deleted = 0 ORDER BY id DESC LIMIT ? OFFSET ?`,
		templateID, pageSize, offset,
	); err != nil {
		return nil, 0, fmt.Errorf("list npcs by template_id: %w", err)
	}
	return items, total, nil
}

// ExportAll 导出所有已启用且未删除的 NPC 裸行
//
// 返回 []model.NPC，service 层负责组装 NPCExportItem（template_ref / fields map / behavior）。
func (s *NpcStore) ExportAll(ctx context.Context) ([]model.NPC, error) {
	items := make([]model.NPC, 0)
	err := s.db.SelectContext(ctx, &items,
		`SELECT id, name, label, description, template_id, template_name, fields, fsm_ref, bt_refs,
		        enabled, version, created_at, updated_at, deleted
		 FROM npcs WHERE deleted = 0 AND enabled = 1 ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("export npcs: %w", err)
	}
	return items, nil
}

// ──────────────────────────────────────────────
// 写操作（事务变体，service 层自行开事务）
// ──────────────────────────────────────────────

// CreateInTx 事务内创建 NPC，返回自增 ID
//
// enabled 默认 1（NPC 是成品，创建即启用，区别于模板/行为树的 0）。
// fieldsJSON：handler 层组装的字段快照 JSON；btRefsJSON：BtRefs map 的 JSON 序列化。
func (s *NpcStore) CreateInTx(ctx context.Context, tx *sqlx.Tx, req *model.CreateNPCRequest, fieldsJSON, btRefsJSON []byte) (int64, error) {
	now := time.Now()
	result, err := tx.ExecContext(ctx,
		`INSERT INTO npcs (name, label, description, template_id, template_name, fields, fsm_ref, bt_refs,
		                   enabled, version, created_at, updated_at, deleted)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, 1, ?, ?, 0)`,
		req.Name, req.Label, req.Description, req.TemplateID, req.TemplateName,
		fieldsJSON, req.FsmRef, btRefsJSON, now, now,
	)
	if err != nil {
		if shared.Is1062(err) {
			return 0, errcode.ErrDuplicate
		}
		return 0, fmt.Errorf("insert npc: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	return id, nil
}

// InsertBtRefsInTx 事务内批量插入 npc_bt_refs（去重 bt_tree_name）
//
// BtRefs map 中同一棵行为树可被多个状态引用，插入前去重。
// 空 map 或所有 value 为空串时直接返回 nil。
func (s *NpcStore) InsertBtRefsInTx(ctx context.Context, tx *sqlx.Tx, npcID int64, btRefs map[string]string) error {
	// 去重 bt_tree_name（多个 state 可引用同一棵树）
	seen := make(map[string]struct{})
	for _, btName := range btRefs {
		if btName != "" {
			seen[btName] = struct{}{}
		}
	}
	if len(seen) == 0 {
		return nil
	}

	placeholders := strings.Repeat("(?,?),", len(seen))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, 0, len(seen)*2)
	for btName := range seen {
		args = append(args, npcID, btName)
	}

	if _, err := tx.ExecContext(ctx,
		"INSERT INTO npc_bt_refs (npc_id, bt_tree_name) VALUES "+placeholders,
		args...,
	); err != nil {
		return fmt.Errorf("insert npc_bt_refs: %w", err)
	}
	return nil
}

// UpdateInTx 事务内编辑 NPC（乐观锁，按 ID）
//
// rows=0 → errcode.ErrVersionConflict（version 不匹配 或 记录已删除）。
// fieldsJSON：重新组装的字段快照 JSON；btRefsJSON：BtRefs map 的 JSON 序列化。
func (s *NpcStore) UpdateInTx(ctx context.Context, tx *sqlx.Tx, req *model.UpdateNPCRequest, fieldsJSON, btRefsJSON []byte) error {
	result, err := tx.ExecContext(ctx,
		`UPDATE npcs SET label = ?, description = ?, fields = ?, fsm_ref = ?, bt_refs = ?,
		                 version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ? AND deleted = 0`,
		req.Label, req.Description, fieldsJSON, req.FsmRef, btRefsJSON,
		time.Now(), req.ID, req.Version,
	)
	if err != nil {
		return fmt.Errorf("update npc: %w", err)
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

// DeleteBtRefsInTx 事务内删除指定 NPC 的所有 npc_bt_refs 引用记录
func (s *NpcStore) DeleteBtRefsInTx(ctx context.Context, tx *sqlx.Tx, npcID int64) error {
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM npc_bt_refs WHERE npc_id = ?`, npcID); err != nil {
		return fmt.Errorf("delete npc_bt_refs: %w", err)
	}
	return nil
}

// SoftDeleteInTx 事务内软删除 NPC，0 rows → errcode.ErrNotFound
func (s *NpcStore) SoftDeleteInTx(ctx context.Context, tx *sqlx.Tx, id int64) error {
	result, err := tx.ExecContext(ctx,
		`UPDATE npcs SET deleted = 1, updated_at = ? WHERE id = ? AND deleted = 0`,
		time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("soft delete npc: %w", err)
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
//
// 单模块写，不需要事务（不联动其它表）。
func (s *NpcStore) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE npcs SET enabled = ?, version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ? AND deleted = 0`,
		req.Enabled, time.Now(), req.ID, req.Version,
	)
	if err != nil {
		return fmt.Errorf("toggle npc enabled: %w", err)
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
