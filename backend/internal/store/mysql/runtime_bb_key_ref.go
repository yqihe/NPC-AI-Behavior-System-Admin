package mysql

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
)

// RuntimeBbKeyRefStore runtime_bb_key_refs 表操作
//
// 与 FieldRefStore 平行：结构对称（三元组 + 反向索引），语义独立（指向 runtime_bb_keys 表）。
type RuntimeBbKeyRefStore struct {
	db *sqlx.DB
}

// NewRuntimeBbKeyRefStore 创建 RuntimeBbKeyRefStore
func NewRuntimeBbKeyRefStore(db *sqlx.DB) *RuntimeBbKeyRefStore {
	return &RuntimeBbKeyRefStore{db: db}
}

// AddBatch 批量添加引用（事务内，用于 FSM/BT Create/Update 时同步引用）
//
// 插入 N 行 (runtime_key_id ∈ keyIDs, ref_type, ref_id)；INSERT IGNORE 屏蔽主键重复。
// keyIDs 为空时直接返回 nil，不发起 SQL。
func (s *RuntimeBbKeyRefStore) AddBatch(ctx context.Context, tx *sqlx.Tx, refType string, refID int64, keyIDs []int64) error {
	if len(keyIDs) == 0 {
		return nil
	}
	placeholders := make([]string, 0, len(keyIDs))
	args := make([]any, 0, len(keyIDs)*4)
	now := time.Now()
	for _, keyID := range keyIDs {
		placeholders = append(placeholders, "(?, ?, ?, ?)")
		args = append(args, keyID, refType, refID, now)
	}
	sqlStr := `INSERT IGNORE INTO runtime_bb_key_refs (runtime_key_id, ref_type, ref_id, created_at) VALUES ` +
		strings.Join(placeholders, ", ")
	if _, err := tx.ExecContext(ctx, sqlStr, args...); err != nil {
		return fmt.Errorf("add runtime_bb_key refs batch: %w", err)
	}
	return nil
}

// DeleteByRefAndKeyIDs 移除指定 (refType, refID) 下某批 keyID 的引用（sync diff 用）
//
// 用途：FSM/BT Update 时 diff oldKeys vs newKeys，对 removedKeys 调用本函数。
// keyIDs 为空时直接返回 nil，不发起 SQL。
func (s *RuntimeBbKeyRefStore) DeleteByRefAndKeyIDs(ctx context.Context, tx *sqlx.Tx, refType string, refID int64, keyIDs []int64) error {
	if len(keyIDs) == 0 {
		return nil
	}
	query, args, err := sqlx.In(
		`DELETE FROM runtime_bb_key_refs WHERE ref_type = ? AND ref_id = ? AND runtime_key_id IN (?)`,
		refType, refID, keyIDs,
	)
	if err != nil {
		return fmt.Errorf("build in delete: %w", err)
	}
	query = tx.Rebind(query)
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("delete runtime_bb_key refs by ref and keys: %w", err)
	}
	return nil
}

// DeleteByRef 移除某个引用方的所有引用（FSM/BT 删除级联用），返回被影响的 runtime_key_id 列表（缓存失效用）
func (s *RuntimeBbKeyRefStore) DeleteByRef(ctx context.Context, tx *sqlx.Tx, refType string, refID int64) ([]int64, error) {
	refs := make([]model.RuntimeBbKeyRef, 0)
	if err := tx.SelectContext(ctx, &refs,
		`SELECT runtime_key_id, ref_type, ref_id, created_at FROM runtime_bb_key_refs WHERE ref_type = ? AND ref_id = ?`,
		refType, refID,
	); err != nil {
		return nil, fmt.Errorf("query runtime_bb_key refs by ref: %w", err)
	}

	keyIDs := make([]int64, 0, len(refs))
	for _, r := range refs {
		keyIDs = append(keyIDs, r.RuntimeKeyID)
	}

	if _, err := tx.ExecContext(ctx,
		`DELETE FROM runtime_bb_key_refs WHERE ref_type = ? AND ref_id = ?`,
		refType, refID,
	); err != nil {
		return nil, fmt.Errorf("delete runtime_bb_key refs by ref: %w", err)
	}
	return keyIDs, nil
}

// ListByRef 查询某个引用方当前引用的所有 runtime_key_id（走反向索引 idx_reverse）
//
// 用途：FSM/BT Update 前取 oldKeys 集合，与 newKeys diff。
func (s *RuntimeBbKeyRefStore) ListByRef(ctx context.Context, refType string, refID int64) ([]int64, error) {
	ids := make([]int64, 0)
	if err := s.db.SelectContext(ctx, &ids,
		`SELECT runtime_key_id FROM runtime_bb_key_refs WHERE ref_type = ? AND ref_id = ?`,
		refType, refID,
	); err != nil {
		return nil, fmt.Errorf("list runtime_bb_key refs by ref: %w", err)
	}
	return ids, nil
}

// ListByKeyID 查询某个 key 的所有引用方（/:id/references 端点用，走主键前缀）
func (s *RuntimeBbKeyRefStore) ListByKeyID(ctx context.Context, keyID int64) ([]model.RuntimeBbKeyRef, error) {
	refs := make([]model.RuntimeBbKeyRef, 0)
	if err := s.db.SelectContext(ctx, &refs,
		`SELECT runtime_key_id, ref_type, ref_id, created_at FROM runtime_bb_key_refs WHERE runtime_key_id = ?`,
		keyID,
	); err != nil {
		return nil, fmt.Errorf("list runtime_bb_key refs by key id: %w", err)
	}
	return refs, nil
}

// HasRefs 非事务检查引用（列表页填充 has_refs / 编辑前 pre-check）
func (s *RuntimeBbKeyRefStore) HasRefs(ctx context.Context, keyID int64) (bool, error) {
	var count int
	if err := s.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM runtime_bb_key_refs WHERE runtime_key_id = ?`, keyID,
	); err != nil {
		return false, fmt.Errorf("check runtime_bb_key has refs: %w", err)
	}
	return count > 0, nil
}

// HasRefsTx 事务内检查引用（Delete 前 TOCTOU 防护，FOR SHARE 阻塞并发 INSERT/DELETE）
func (s *RuntimeBbKeyRefStore) HasRefsTx(ctx context.Context, tx *sqlx.Tx, keyID int64) (bool, error) {
	var count int
	if err := tx.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM runtime_bb_key_refs WHERE runtime_key_id = ? FOR SHARE`, keyID,
	); err != nil {
		return false, fmt.Errorf("check runtime_bb_key has refs tx: %w", err)
	}
	return count > 0, nil
}

// CountByKeyIDs 批量统计每个 keyID 的引用数（列表页填充 has_refs / ref_count 用）
//
// 返回 map[keyID → count]，不在 map 中的 keyID 表示 0 引用。
// keyIDs 为空时直接返回空 map，不发起 SQL。
func (s *RuntimeBbKeyRefStore) CountByKeyIDs(ctx context.Context, keyIDs []int64) (map[int64]int, error) {
	result := make(map[int64]int)
	if len(keyIDs) == 0 {
		return result, nil
	}

	query, args, err := sqlx.In(
		`SELECT runtime_key_id, COUNT(*) AS cnt FROM runtime_bb_key_refs WHERE runtime_key_id IN (?) GROUP BY runtime_key_id`,
		keyIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("build in count: %w", err)
	}
	query = s.db.Rebind(query)

	rows, err := s.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("count runtime_bb_key refs by keys: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var cnt int
		if err := rows.Scan(&id, &cnt); err != nil {
			return nil, fmt.Errorf("scan count row: %w", err)
		}
		result[id] = cnt
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return result, nil
}
