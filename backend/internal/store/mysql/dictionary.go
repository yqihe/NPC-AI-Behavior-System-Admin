package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
)

// DictionaryStore dictionaries 表操作
type DictionaryStore struct {
	db *sqlx.DB
}

// NewDictionaryStore 创建 DictionaryStore
func NewDictionaryStore(db *sqlx.DB) *DictionaryStore {
	return &DictionaryStore{db: db}
}

// ListByGroup 按 group 查询所有启用的选项（覆盖索引）
func (s *DictionaryStore) ListByGroup(ctx context.Context, groupName string) ([]model.Dictionary, error) {
	items := make([]model.Dictionary, 0)
	err := s.db.SelectContext(ctx, &items,
		`SELECT id, group_name, name, label, sort_order, extra, enabled, created_at, updated_at
		 FROM dictionaries
		 WHERE group_name = ? AND enabled = 1
		 ORDER BY sort_order ASC`,
		groupName,
	)
	if err != nil {
		return nil, fmt.Errorf("list dictionaries by group: %w", err)
	}
	return items, nil
}

// ListAll 查询所有启用的字典（启动时全量加载）
func (s *DictionaryStore) ListAll(ctx context.Context) ([]model.Dictionary, error) {
	items := make([]model.Dictionary, 0)
	err := s.db.SelectContext(ctx, &items,
		`SELECT id, group_name, name, label, sort_order, extra, enabled, created_at, updated_at
		 FROM dictionaries
		 WHERE enabled = 1
		 ORDER BY group_name ASC, sort_order ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list all dictionaries: %w", err)
	}
	return items, nil
}

// Create 创建字典条目
func (s *DictionaryStore) Create(ctx context.Context, d *model.Dictionary) (int64, error) {
	now := time.Now()
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO dictionaries (group_name, name, label, sort_order, extra, enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		d.GroupName, d.Name, d.Label, d.SortOrder, d.Extra, d.Enabled, now, now,
	)
	if err != nil {
		return 0, fmt.Errorf("insert dictionary: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	return id, nil
}

// BatchCreate 批量创建（种子脚本用）
func (s *DictionaryStore) BatchCreate(ctx context.Context, items []model.Dictionary) error {
	if len(items) == 0 {
		return nil
	}
	now := time.Now()
	for i := range items {
		items[i].CreatedAt = now
		items[i].UpdatedAt = now
		items[i].Enabled = true
	}

	_, err := s.db.NamedExecContext(ctx,
		`INSERT IGNORE INTO dictionaries (group_name, name, label, sort_order, extra, enabled, created_at, updated_at)
		 VALUES (:group_name, :name, :label, :sort_order, :extra, :enabled, :created_at, :updated_at)`,
		items,
	)
	if err != nil {
		return fmt.Errorf("batch insert dictionaries: %w", err)
	}
	return nil
}

// GetByGroupAndName 按 group + name 查询单条
func (s *DictionaryStore) GetByGroupAndName(ctx context.Context, groupName, name string) (*model.Dictionary, error) {
	var d model.Dictionary
	err := s.db.GetContext(ctx, &d,
		`SELECT id, group_name, name, label, sort_order, extra, enabled, created_at, updated_at
		 FROM dictionaries WHERE group_name = ? AND name = ?`,
		groupName, name,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get dictionary: %w", err)
	}
	return &d, nil
}
