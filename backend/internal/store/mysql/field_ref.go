package mysql

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
)

// FieldRefStore field_refs 表操作
type FieldRefStore struct {
	db *sqlx.DB
}

// NewFieldRefStore 创建 FieldRefStore
func NewFieldRefStore(db *sqlx.DB) *FieldRefStore {
	return &FieldRefStore{db: db}
}

// Add 添加引用关系（事务内，按 ID）
func (s *FieldRefStore) Add(ctx context.Context, tx *sqlx.Tx, fieldID int64, refType string, refID int64) error {
	_, err := tx.ExecContext(ctx,
		`INSERT IGNORE INTO field_refs (field_id, ref_type, ref_id) VALUES (?, ?, ?)`,
		fieldID, refType, refID,
	)
	if err != nil {
		return fmt.Errorf("add field ref: %w", err)
	}
	return nil
}

// Remove 移除单条引用关系（事务内，按 ID）
func (s *FieldRefStore) Remove(ctx context.Context, tx *sqlx.Tx, fieldID int64, refType string, refID int64) error {
	_, err := tx.ExecContext(ctx,
		`DELETE FROM field_refs WHERE field_id = ? AND ref_type = ? AND ref_id = ?`,
		fieldID, refType, refID,
	)
	if err != nil {
		return fmt.Errorf("remove field ref: %w", err)
	}
	return nil
}

// RemoveBySource 移除某个引用方的所有引用，返回被引用的字段 ID 列表（用于缓存失效）
// 例：删除 reference 类型字段时，清理它对其他字段的引用
func (s *FieldRefStore) RemoveBySource(ctx context.Context, tx *sqlx.Tx, refType string, refID int64) ([]int64, error) {
	// 先查出被引用的字段 ID 列表（必须在同一事务内）
	refs := make([]model.FieldRef, 0)
	err := tx.SelectContext(ctx, &refs,
		`SELECT field_id, ref_type, ref_id FROM field_refs WHERE ref_type = ? AND ref_id = ?`,
		refType, refID,
	)
	if err != nil {
		return nil, fmt.Errorf("query refs by source: %w", err)
	}

	fieldIDs := make([]int64, 0, len(refs))
	for _, r := range refs {
		fieldIDs = append(fieldIDs, r.FieldID)
	}

	// 删除
	_, err = tx.ExecContext(ctx,
		`DELETE FROM field_refs WHERE ref_type = ? AND ref_id = ?`,
		refType, refID,
	)
	if err != nil {
		return nil, fmt.Errorf("remove refs by source: %w", err)
	}

	return fieldIDs, nil
}

// GetByFieldID 查询某个字段的所有引用方（主键索引前缀）
func (s *FieldRefStore) GetByFieldID(ctx context.Context, fieldID int64) ([]model.FieldRef, error) {
	refs := make([]model.FieldRef, 0)
	err := s.db.SelectContext(ctx, &refs,
		`SELECT field_id, ref_type, ref_id FROM field_refs WHERE field_id = ?`,
		fieldID,
	)
	if err != nil {
		return nil, fmt.Errorf("get refs by field id: %w", err)
	}
	return refs, nil
}

// HasRefs 非事务检查引用（编辑前检查，判断是否需要约束保护）
func (s *FieldRefStore) HasRefs(ctx context.Context, fieldID int64) (bool, error) {
	var count int
	err := s.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM field_refs WHERE field_id = ?`, fieldID,
	)
	if err != nil {
		return false, fmt.Errorf("check has refs: %w", err)
	}
	return count > 0, nil
}

// HasRefsTx 事务内检查引用（删除前检查，防 TOCTOU）
// FOR SHARE 保证当前读 + 阻止并发 INSERT/DELETE
func (s *FieldRefStore) HasRefsTx(ctx context.Context, tx *sqlx.Tx, fieldID int64) (bool, error) {
	var count int
	err := tx.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM field_refs WHERE field_id = ? FOR SHARE`, fieldID,
	)
	if err != nil {
		return false, fmt.Errorf("check has refs tx: %w", err)
	}
	return count > 0, nil
}
