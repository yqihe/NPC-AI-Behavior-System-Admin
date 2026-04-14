package mysql

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
)

// SchemaRefStore schema_refs 表操作
type SchemaRefStore struct {
	db *sqlx.DB
}

// NewSchemaRefStore 创建 SchemaRefStore
func NewSchemaRefStore(db *sqlx.DB) *SchemaRefStore {
	return &SchemaRefStore{db: db}
}

// Add 添加引用关系（事务内，事件类型创建/编辑时）
func (s *SchemaRefStore) Add(ctx context.Context, tx *sqlx.Tx, schemaID int64, refType string, refID int64) error {
	_, err := tx.ExecContext(ctx,
		`INSERT IGNORE INTO schema_refs (schema_id, ref_type, ref_id) VALUES (?, ?, ?)`,
		schemaID, refType, refID,
	)
	if err != nil {
		return fmt.Errorf("add schema ref: %w", err)
	}
	return nil
}

// Remove 移除单条引用关系（事务内，事件类型编辑时）
func (s *SchemaRefStore) Remove(ctx context.Context, tx *sqlx.Tx, schemaID int64, refType string, refID int64) error {
	_, err := tx.ExecContext(ctx,
		`DELETE FROM schema_refs WHERE schema_id = ? AND ref_type = ? AND ref_id = ?`,
		schemaID, refType, refID,
	)
	if err != nil {
		return fmt.Errorf("remove schema ref: %w", err)
	}
	return nil
}

// RemoveByRef 移除某个引用方的所有引用（事务内，事件类型删除时）
// 返回被引用的 schema ID 列表（用于缓存失效）
func (s *SchemaRefStore) RemoveByRef(ctx context.Context, tx *sqlx.Tx, refType string, refID int64) ([]int64, error) {
	// 先查出被引用的 schema ID 列表
	refs := make([]model.SchemaRef, 0)
	err := tx.SelectContext(ctx, &refs,
		`SELECT schema_id, ref_type, ref_id FROM schema_refs WHERE ref_type = ? AND ref_id = ?`,
		refType, refID,
	)
	if err != nil {
		return nil, fmt.Errorf("query schema refs by ref: %w", err)
	}

	schemaIDs := make([]int64, 0, len(refs))
	for _, r := range refs {
		schemaIDs = append(schemaIDs, r.SchemaID)
	}

	// 删除
	_, err = tx.ExecContext(ctx,
		`DELETE FROM schema_refs WHERE ref_type = ? AND ref_id = ?`,
		refType, refID,
	)
	if err != nil {
		return nil, fmt.Errorf("remove schema refs by ref: %w", err)
	}

	return schemaIDs, nil
}

// HasRefs 非事务检查引用（编辑保护：约束收紧检查前判断）
func (s *SchemaRefStore) HasRefs(ctx context.Context, schemaID int64) (bool, error) {
	var count int
	err := s.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM schema_refs WHERE schema_id = ?`, schemaID,
	)
	if err != nil {
		return false, fmt.Errorf("check schema has refs: %w", err)
	}
	return count > 0, nil
}

// HasRefsTx 事务内检查引用（删除保护，FOR SHARE 防 TOCTOU）
func (s *SchemaRefStore) HasRefsTx(ctx context.Context, tx *sqlx.Tx, schemaID int64) (bool, error) {
	var count int
	err := tx.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM schema_refs WHERE schema_id = ? FOR SHARE`, schemaID,
	)
	if err != nil {
		return false, fmt.Errorf("check schema has refs tx: %w", err)
	}
	return count > 0, nil
}

// GetBySchemaID 查询某个扩展字段的所有引用方（references API）
func (s *SchemaRefStore) GetBySchemaID(ctx context.Context, schemaID int64) ([]model.SchemaRef, error) {
	refs := make([]model.SchemaRef, 0)
	err := s.db.SelectContext(ctx, &refs,
		`SELECT schema_id, ref_type, ref_id FROM schema_refs WHERE schema_id = ?`,
		schemaID,
	)
	if err != nil {
		return nil, fmt.Errorf("get schema refs by schema id: %w", err)
	}
	return refs, nil
}
