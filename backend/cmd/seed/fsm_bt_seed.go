package main

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// fsm_bt_seed.go 覆盖 external-contract-admin-shape-alignment 冷启动遗留缺口。
//
// 背景：NPC seed 硬引用 3 个 FSM + 6 棵 BT，但 seed 从不创建它们；富 DB 卷掩盖了这个
// 假阳性直到 2026-04-20 R15 起步时暴露。fixture 由 Server CC 提供（2026-04-20）。
//
// Phase 2（2026-04-20 post-freeze）：Server CC 把 police FSM/BT 翻译为 combat 4 态
// (Idle/Patrol/Chase/Attack)，去掉历史 6 态中的 Flee/Dead（无血量系统，留 state 会引发
// 策划层歧义）；guard/patrol 重命名为 bt/guard/patrol 以对齐 bt/domain/leaf 命名约定。
// 详见 docs/specs/seed-fsm-bt-coverage/。

type fsmConfigSeed struct {
	Name        string
	DisplayName string
	ConfigJSON  string
}

type btTreeSeed struct {
	Name        string
	DisplayName string
	Description string
	Config      string
}

// ── FSM 配置（3 条）──

var fsmConfigFixtures = []fsmConfigSeed{
	{
		Name:        "fsm_combat_basic",
		DisplayName: "通用战斗状态机",
		ConfigJSON:  `{"initial_state":"Idle","states":[{"name":"Idle"},{"name":"Patrol"},{"name":"Chase"},{"name":"Attack"}],"transitions":[{"from":"Idle","to":"Patrol","priority":1,"condition":{"key":"last_event_type","op":"==","value":""}},{"from":"Patrol","to":"Chase","priority":10,"condition":{"and":[{"key":"threat_level","op":">=","value":30},{"key":"threat_expire_at","op":">","value":"","ref_key":"current_time"}]}},{"from":"Chase","to":"Attack","priority":10,"condition":{"and":[{"key":"threat_level","op":">=","value":60},{"key":"threat_expire_at","op":">","value":"","ref_key":"current_time"}]}},{"from":"Attack","to":"Chase","priority":5,"condition":{"key":"threat_level","op":"<","value":40}},{"from":"Chase","to":"Patrol","priority":5,"condition":{"or":[{"key":"threat_level","op":"<","value":10},{"key":"threat_expire_at","op":"<=","value":"","ref_key":"current_time"}]}},{"from":"Attack","to":"Patrol","priority":3,"condition":{"or":[{"key":"threat_level","op":"<","value":10},{"key":"threat_expire_at","op":"<=","value":"","ref_key":"current_time"}]}}]}`,
	},
	{
		Name:        "fsm_passive",
		DisplayName: "被动 NPC 状态机",
		ConfigJSON:  `{"initial_state":"wander","states":[{"name":"wander"}]}`,
	},
	{
		Name:        "guard",
		DisplayName: "守卫状态机",
		ConfigJSON:  `{"initial_state":"Patrol","states":[{"name":"Patrol"},{"name":"Alert"},{"name":"Defend"}],"transitions":[{"from":"Patrol","to":"Alert","priority":10,"condition":{"key":"last_event_type","op":"!=","value":""}},{"from":"Alert","to":"Defend","priority":10,"condition":{"and":[{"key":"threat_level","op":">=","value":60},{"key":"threat_expire_at","op":">","value":"","ref_key":"current_time"}]}},{"from":"Alert","to":"Patrol","priority":5,"condition":{"key":"last_event_type","op":"==","value":""}},{"from":"Defend","to":"Patrol","priority":5,"condition":{"or":[{"key":"threat_level","op":"<","value":20},{"key":"threat_expire_at","op":"<=","value":"","ref_key":"current_time"}]}}]}`,
	},
}

// ── BT 树（6 棵）──

var btTreeFixtures = []btTreeSeed{
	{
		Name:        "bt/combat/idle",
		DisplayName: "战斗-待机",
		Description: "战斗状态机 Idle 状态 BT（idle_stand 占位）",
		Config:      `{"type":"stub_action","params":{"name":"idle_stand","result":"success"}}`,
	},
	{
		Name:        "bt/combat/patrol",
		DisplayName: "战斗-巡逻",
		Description: "战斗状态机 Patrol 状态 BT（写 current_action + patrol_waypoints）",
		Config:      `{"type":"sequence","children":[{"type":"set_bb_value","params":{"key":"current_action","value":"patrol_move"}},{"type":"stub_action","params":{"name":"patrol_waypoints","result":"success"}}]}`,
	},
	{
		Name:        "bt/combat/chase",
		DisplayName: "战斗-追击",
		Description: "战斗状态机 Chase 状态 BT（写 current_action + approach_target）",
		Config:      `{"type":"sequence","children":[{"type":"set_bb_value","params":{"key":"current_action","value":"detect_target"}},{"type":"stub_action","params":{"name":"approach_target","result":"success"}}]}`,
	},
	{
		Name:        "bt/combat/attack",
		DisplayName: "战斗-攻击",
		Description: "战斗状态机 Attack 状态 BT（draw_weapon → equip_weapon → engage_target → engage）",
		Config:      `{"type":"sequence","children":[{"type":"set_bb_value","params":{"key":"current_action","value":"draw_weapon"}},{"type":"stub_action","params":{"name":"equip_weapon","result":"success"}},{"type":"set_bb_value","params":{"key":"current_action","value":"engage_target"}},{"type":"stub_action","params":{"name":"engage","result":"success"}}]}`,
	},
	{
		Name:        "bt/passive/wander",
		DisplayName: "被动-游荡",
		Description: "被动 NPC 游荡占位 BT",
		Config:      `{"type":"stub_action","params":{"name":"wander","result":"success"}}`,
	},
	{
		Name:        "bt/guard/patrol",
		DisplayName: "守卫-巡逻",
		Description: "守卫 Patrol 状态 BT（static post 岗位巡逻语义）",
		Config:      `{"type":"sequence","children":[{"type":"set_bb_value","params":{"key":"current_action","value":"guard_patrol"}},{"type":"stub_action","params":{"name":"patrol_post","result":"success"}}]}`,
	},
}

// seedFsmConfigs 幂等写入 3 条 FSM 配置，enabled=1（导出可用）。
// 冲突策略：INSERT IGNORE — name 已存在则跳过，不覆盖运营手改的 config/display_name。
func seedFsmConfigs(ctx context.Context, db *sqlx.DB) error {
	const insertSQL = `
INSERT IGNORE INTO fsm_configs (name, display_name, config_json, enabled, version, deleted, created_at, updated_at)
VALUES (?, ?, ?, 1, 1, 0, NOW(), NOW())`

	inserted := 0
	skipped := 0
	for _, s := range fsmConfigFixtures {
		result, err := db.ExecContext(ctx, insertSQL, s.Name, s.DisplayName, s.ConfigJSON)
		if err != nil {
			return fmt.Errorf("insert fsm_config %q: %w", s.Name, err)
		}
		if rows, _ := result.RowsAffected(); rows > 0 {
			inserted++
		} else {
			skipped++
			fmt.Printf("  [跳过] FSM %s（已存在）\n", s.Name)
		}
	}

	fmt.Printf("FSM 配置写入完成：新增 %d 条，跳过 %d 条（已存在）\n", inserted, skipped)
	return nil
}

// seedBtTrees 幂等写入 6 棵 BT，enabled=1（导出可用）。
// 冲突策略：INSERT IGNORE — name 已存在则跳过。
func seedBtTrees(ctx context.Context, db *sqlx.DB) error {
	const insertSQL = `
INSERT IGNORE INTO bt_trees (name, display_name, description, config, enabled, version, deleted, created_at, updated_at)
VALUES (?, ?, ?, ?, 1, 1, 0, NOW(), NOW())`

	inserted := 0
	skipped := 0
	for _, s := range btTreeFixtures {
		result, err := db.ExecContext(ctx, insertSQL, s.Name, s.DisplayName, s.Description, s.Config)
		if err != nil {
			return fmt.Errorf("insert bt_tree %q: %w", s.Name, err)
		}
		if rows, _ := result.RowsAffected(); rows > 0 {
			inserted++
		} else {
			skipped++
			fmt.Printf("  [跳过] BT %s（已存在）\n", s.Name)
		}
	}

	fmt.Printf("行为树写入完成：新增 %d 条，跳过 %d 条（已存在）\n", inserted, skipped)
	return nil
}
