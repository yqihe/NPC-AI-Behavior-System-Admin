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

// RegionStore regions 表操作
type RegionStore struct {
	db *sqlx.DB
}

// NewRegionStore 创建 RegionStore
func NewRegionStore(db *sqlx.DB) *RegionStore {
	return &RegionStore{db: db}
}

// DB 暴露数据库连接
func (s *RegionStore) DB() *sqlx.DB {
	return s.db
}

// Create 插入新区域，region_id 唯一冲突返回 errcode.ErrDuplicate
func (s *RegionStore) Create(ctx context.Context, req *model.CreateRegionRequest) (int64, error) {
	now := time.Now()
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO regions (region_id, display_name, region_type, spawn_table, enabled, version, created_at, updated_at, deleted)
		 VALUES (?, ?, ?, ?, 0, 1, ?, ?, 0)`,
		req.RegionID, req.DisplayName, req.RegionType, req.SpawnTable, now, now,
	)
	if err != nil {
		if shared.Is1062(err) {
			return 0, errcode.ErrDuplicate
		}
		return 0, fmt.Errorf("insert region: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	return id, nil
}

// GetByID 按主键查询区域（含 spawn_table），未找到返回 nil, nil
func (s *RegionStore) GetByID(ctx context.Context, id int64) (*model.Region, error) {
	var r model.Region
	err := s.db.GetContext(ctx, &r,
		`SELECT id, region_id, display_name, region_type, spawn_table, enabled, version, created_at, updated_at, deleted
		 FROM regions WHERE id = ? AND deleted = 0`, id)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get region by id: %w", err)
	}
	return &r, nil
}

// GetByRegionID 按业务键查询区域（含 spawn_table），未找到返回 nil, nil
func (s *RegionStore) GetByRegionID(ctx context.Context, regionID string) (*model.Region, error) {
	var r model.Region
	err := s.db.GetContext(ctx, &r,
		`SELECT id, region_id, display_name, region_type, spawn_table, enabled, version, created_at, updated_at, deleted
		 FROM regions WHERE region_id = ? AND deleted = 0`, regionID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get region by region_id: %w", err)
	}
	return &r, nil
}

// ExistsByRegionID 检查 region_id 是否已存在（含软删除行）
//
// 不过滤 deleted：已删除的 region_id 永久不可复用（对齐 bt_trees 先例）。
func (s *RegionStore) ExistsByRegionID(ctx context.Context, regionID string) (bool, error) {
	var count int
	err := s.db.GetContext(ctx, &count,
		`SELECT COUNT(*) FROM regions WHERE region_id = ?`, regionID)
	if err != nil {
		return false, fmt.Errorf("check region_id exists: %w", err)
	}
	return count > 0, nil
}

// List 分页列表查询
//
// region_id/display_name 两端模糊匹配，region_type 精确筛选，enabled 精确筛选。
// 列表只取核心列，不返回 spawn_table（减少传输量）。
func (s *RegionStore) List(ctx context.Context, q *model.RegionListQuery) ([]model.RegionListItem, int64, error) {
	where := []string{"deleted = 0"}
	args := make([]any, 0, 4)

	if q.RegionID != "" {
		where = append(where, "region_id LIKE ?")
		args = append(args, "%"+shared.EscapeLike(q.RegionID)+"%")
	}
	if q.DisplayName != "" {
		where = append(where, "display_name LIKE ?")
		args = append(args, "%"+shared.EscapeLike(q.DisplayName)+"%")
	}
	if q.RegionType != "" {
		where = append(where, "region_type = ?")
		args = append(args, q.RegionType)
	}
	if q.Enabled != nil {
		where = append(where, "enabled = ?")
		args = append(args, *q.Enabled)
	}

	whereClause := strings.Join(where, " AND ")

	// 计数
	var total int64
	countSQL := "SELECT COUNT(*) FROM regions WHERE " + whereClause
	if err := s.db.GetContext(ctx, &total, countSQL, args...); err != nil {
		return nil, 0, fmt.Errorf("count regions: %w", err)
	}

	if total == 0 {
		return make([]model.RegionListItem, 0), 0, nil
	}

	// 分页查询（按 id DESC）
	offset := (q.Page - 1) * q.PageSize
	listSQL := fmt.Sprintf(
		`SELECT id, region_id, display_name, region_type, enabled, created_at
		 FROM regions WHERE %s ORDER BY id DESC LIMIT ? OFFSET ?`,
		whereClause,
	)
	listArgs := make([]any, len(args), len(args)+2)
	copy(listArgs, args)
	listArgs = append(listArgs, q.PageSize, offset)

	items := make([]model.RegionListItem, 0)
	if err := s.db.SelectContext(ctx, &items, listSQL, listArgs...); err != nil {
		return nil, 0, fmt.Errorf("list regions: %w", err)
	}

	return items, total, nil
}

// Update 编辑区域（乐观锁，按 ID；region_id 不可变，不在 SET 里）
//
// rows=0 → errcode.ErrVersionConflict（version 不匹配 或 记录已删除）。
func (s *RegionStore) Update(ctx context.Context, req *model.UpdateRegionRequest) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE regions SET display_name = ?, region_type = ?, spawn_table = ?, version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ? AND deleted = 0`,
		req.DisplayName, req.RegionType, req.SpawnTable, time.Now(), req.ID, req.Version,
	)
	if err != nil {
		return fmt.Errorf("update region: %w", err)
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

// SoftDelete 软删除区域，0 rows → errcode.ErrNotFound
func (s *RegionStore) SoftDelete(ctx context.Context, id int64) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE regions SET deleted = 1, updated_at = ? WHERE id = ? AND deleted = 0`,
		time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("soft delete region: %w", err)
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
func (s *RegionStore) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE regions SET enabled = ?, version = version + 1, updated_at = ?
		 WHERE id = ? AND version = ? AND deleted = 0`,
		req.Enabled, time.Now(), req.ID, req.Version,
	)
	if err != nil {
		return fmt.Errorf("toggle region enabled: %w", err)
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

// ExportAll 导出所有已启用且未删除的区域（含完整行，供 service 层装配成 RegionExportItem）
//
// 返回 []Region 而非 []RegionExportItem：envelope name 与 config.name 分层语义由 service 层负责，
// store 层只负责取行（对齐 export-ref-validation 的 NpcStore.ExportAll 模式）。
func (s *RegionStore) ExportAll(ctx context.Context) ([]model.Region, error) {
	items := make([]model.Region, 0)
	err := s.db.SelectContext(ctx, &items,
		`SELECT id, region_id, display_name, region_type, spawn_table, enabled, version, created_at, updated_at, deleted
		 FROM regions WHERE deleted = 0 AND enabled = 1 ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("export regions: %w", err)
	}
	return items, nil
}
