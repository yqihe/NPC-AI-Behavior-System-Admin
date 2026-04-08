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

// Add 添加引用关系（事务内）
func (s *FieldRefStore) Add(ctx context.Context, tx *sqlx.Tx, ref *model.FieldRef) error {
	_, err := tx.ExecContext(ctx,
		`INSERT IGNORE INTO field_refs (field_name, ref_type, ref_name) VALUES (?, ?, ?)`,
		ref.FieldName, ref.RefType, ref.RefName,
	)
	if err != nil {
		return fmt.Errorf("add field ref: %w", err)
	}
	return nil
}

// Remove 移除引用关系（事务内）
func (s *FieldRefStore) Remove(ctx context.Context, tx *sqlx.Tx, ref *model.FieldRef) error {
	_, err := tx.ExecContext(ctx,
		`DELETE FROM field_refs WHERE field_name = ? AND ref_type = ? AND ref_name = ?`,
		ref.FieldName, ref.RefType, ref.RefName,
	)
	if err != nil {
		return fmt.Errorf("remove field ref: %w", err)
	}
	return nil
}

// RemoveByRef 移除某个引用方的所有引用（如删除模板时，清理该模板的所有字段引用）
func (s *FieldRefStore) RemoveByRef(ctx context.Context, tx *sqlx.Tx, refType, refName string) ([]string, error) {
	// 先查出被引用的字段列表（用于后续 ref_count 维护）
	refs := make([]model.FieldRef, 0)
	err := s.db.SelectContext(ctx, &refs,
		`SELECT field_name, ref_type, ref_name FROM field_refs WHERE ref_type = ? AND ref_name = ?`,
		refType, refName,
	)
	if err != nil {
		return nil, fmt.Errorf("query refs by ref: %w", err)
	}

	fieldNames := make([]string, 0, len(refs))
	for _, r := range refs {
		fieldNames = append(fieldNames, r.FieldName)
	}

	// 删除
	_, err = tx.ExecContext(ctx,
		`DELETE FROM field_refs WHERE ref_type = ? AND ref_name = ?`,
		refType, refName,
	)
	if err != nil {
		return nil, fmt.Errorf("remove refs by ref: %w", err)
	}

	return fieldNames, nil
}

// GetByFieldName 查询某个字段的所有引用方（主键索引前缀）
func (s *FieldRefStore) GetByFieldName(ctx context.Context, fieldName string) ([]model.FieldRef, error) {
	refs := make([]model.FieldRef, 0)
	err := s.db.SelectContext(ctx, &refs,
		`SELECT field_name, ref_type, ref_name FROM field_refs WHERE field_name = ?`,
		fieldName,
	)
	if err != nil {
		return nil, fmt.Errorf("get refs by field name: %w", err)
	}
	return refs, nil
}

// GetByRefName 查询某个引用方引用了哪些字段（走 idx_ref）
func (s *FieldRefStore) GetByRefName(ctx context.Context, refType, refName string) ([]model.FieldRef, error) {
	refs := make([]model.FieldRef, 0)
	err := s.db.SelectContext(ctx, &refs,
		`SELECT field_name, ref_type, ref_name FROM field_refs WHERE ref_type = ? AND ref_name = ?`,
		refType, refName,
	)
	if err != nil {
		return nil, fmt.Errorf("get refs by ref name: %w", err)
	}
	return refs, nil
}

// HasRefs 检查字段是否有引用（删除前检查）
func (s *FieldRefStore) HasRefs(ctx context.Context, fieldName string) (bool, error) {
	var count int
	err := s.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM field_refs WHERE field_name = ?`, fieldName,
	)
	if err != nil {
		return false, fmt.Errorf("check has refs: %w", err)
	}
	return count > 0, nil
}

// HasRefsTx 事务内检查引用（原子删除用，防 TOCTOU）
func (s *FieldRefStore) HasRefsTx(ctx context.Context, tx *sqlx.Tx, fieldName string) (bool, error) {
	var count int
	err := tx.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM field_refs WHERE field_name = ?`, fieldName,
	)
	if err != nil {
		return false, fmt.Errorf("check has refs tx: %w", err)
	}
	return count > 0, nil
}
