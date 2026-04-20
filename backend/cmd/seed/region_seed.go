package main

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/yqihe/npc-ai-admin/backend/internal/util"
)

// region_seed.go — regions-module T12 种子
//
// 两部分：
//  1. region_type 字典组（2 枚举：wilderness / town）— 前端 RegionList/RegionForm 下拉选项数据源
//  2. regions 表 1 条 fixture（village_outskirts）— 联调 villager_guard × 2 spawn，Server HTTPSource 接入即跑
//
// 均 INSERT IGNORE 幂等，同 name 重跑自然跳过。
//
// 依赖顺序（见 main.go 编排）：
//   seedFieldsTemplatesNPCs → seedRegionTypeDict → seedRegions
// 因 regions.spawn_table.template_ref 必须指向已存在的 NPC 记录（T7 validateSpawnTable 引用校验）。

// seedRegionTypeDict 写入 2 枚举字典（wilderness / town）到 dictionaries 表
func seedRegionTypeDict(ctx context.Context, db *sqlx.DB) error {
	entries := []struct {
		Name      string
		Label     string
		SortOrder int
	}{
		{Name: "wilderness", Label: "野外", SortOrder: 1},
		{Name: "town", Label: "城镇", SortOrder: 2},
	}

	const insertSQL = `
INSERT IGNORE INTO dictionaries (group_name, name, label, sort_order, created_at, updated_at)
VALUES (?, ?, ?, ?, NOW(), NOW())`

	inserted := 0
	skipped := 0
	for _, e := range entries {
		result, err := db.ExecContext(ctx, insertSQL, util.DictGroupRegionType, e.Name, e.Label, e.SortOrder)
		if err != nil {
			return fmt.Errorf("insert region_type dict %q: %w", e.Name, err)
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			skipped++
			fmt.Printf("  [跳过] region_type 字典 %s（已存在）\n", e.Name)
		} else {
			inserted++
		}
	}

	fmt.Printf("区域类型字典写入完成：新增 %d 条，跳过 %d 条（已存在）\n", inserted, skipped)
	return nil
}

// seedRegions 写入 village_outskirts fixture（启用状态，1 条 spawn_entry 引 villager_guard × 2）
//
// Server HTTPSource 接入 ADMIN API 后，该 region 立即被 LoadAllRegionConfigs 拉取，
// Server 启动时在该 zone 内 spawn 出 2 个 villager_guard（引 T12 NPC 种子）。
func seedRegions(ctx context.Context, db *sqlx.DB) error {
	const regionID = "village_outskirts"
	const displayName = "村庄外围"
	const regionType = "wilderness"
	const spawnTable = `[{"template_ref":"villager_guard","count":2,"spawn_points":[{"x":10,"z":20},{"x":15,"z":20}],"wander_radius":5,"respawn_seconds":60}]`

	const insertSQL = `
INSERT IGNORE INTO regions (region_id, display_name, region_type, spawn_table, enabled, version, deleted, created_at, updated_at)
VALUES (?, ?, ?, ?, 1, 1, 0, NOW(), NOW())`

	result, err := db.ExecContext(ctx, insertSQL, regionID, displayName, regionType, spawnTable)
	if err != nil {
		return fmt.Errorf("insert region %q: %w", regionID, err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		fmt.Printf("  [跳过] region %s（已存在）\n", regionID)
		fmt.Printf("区域种子写入完成：新增 0 条，跳过 1 条（已存在）\n")
	} else {
		fmt.Printf("区域种子写入完成：新增 1 条（%s）\n", regionID)
	}
	return nil
}
