package main

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// runtime_bb_key_seed.go — 31 条内置运行时 BB Key 种子
//
// 每条逐字对齐 Server internal/core/blackboard/keys.go：
//   - Name：NewKey[T]("...") 第一参数
//   - Type：Go 泛型参数映射（float64→float / int64→integer / string→string / bool→bool）
//   - GroupName：keys.go `// --- xxx ---` 分节注释对齐
//   - Label / Description：从 keys.go 尾部行注释提炼为中文
//
// enabled=1 立即可用（走 RuntimeBbKeyStore.CreateEnabled 对应的 SQL 语义）。
// INSERT IGNORE 幂等，同 name 重跑自然跳过。

type runtimeBbKeySeed struct {
	Name        string
	Type        string
	GroupName   string
	Label       string
	Description string
}

// runtimeBbKeyFixtures 31 条内置 key（分布：13 float / 4 integer / 12 string / 2 bool）
var runtimeBbKeyFixtures = []runtimeBbKeySeed{
	// --- 威胁相关（3）---
	{Name: "threat_level", Type: "float", GroupName: "threat", Label: "威胁等级", Description: "当前威胁等级 0~100，决策中心写入，FSM 读取"},
	{Name: "threat_source", Type: "string", GroupName: "threat", Label: "威胁来源", Description: "威胁来源 ID，决策中心写入"},
	{Name: "threat_expire_at", Type: "integer", GroupName: "threat", Label: "威胁过期时间", Description: "威胁过期时间戳（毫秒），决策中心写入，FSM 读取"},

	// --- 事件相关（2）---
	{Name: "last_event_type", Type: "string", GroupName: "event", Label: "最近事件类型", Description: "最近一次感知到的事件类型，决策中心写入，FSM 读取"},
	{Name: "current_time", Type: "integer", GroupName: "event", Label: "当前时间戳", Description: "当前时间戳（毫秒），Runtime 每 Tick 更新"},

	// --- FSM 状态（1）---
	{Name: "fsm_state", Type: "string", GroupName: "fsm", Label: "当前 FSM 状态", Description: "当前 FSM 状态名，FSM 引擎写入"},

	// --- NPC 实例（3）---
	{Name: "npc_type", Type: "string", GroupName: "npc", Label: "NPC 类型", Description: "NPC 类型名，创建时写入"},
	{Name: "npc_pos_x", Type: "float", GroupName: "npc", Label: "NPC 位置 X", Description: "NPC 位置 X 坐标，Runtime 每 Tick 更新"},
	{Name: "npc_pos_z", Type: "float", GroupName: "npc", Label: "NPC 位置 Z", Description: "NPC 位置 Z 坐标，Runtime 每 Tick 更新"},

	// --- 行为追踪（3）---
	{Name: "current_action", Type: "string", GroupName: "action", Label: "当前子行为", Description: "BT 当前执行的子行为名"},
	{Name: "alert_start_tick", Type: "integer", GroupName: "action", Label: "警戒起始时刻", Description: "进入 Alarmed 状态的时间戳"},
	{Name: "exit_cleanup_done", Type: "string", GroupName: "action", Label: "退出清理标记", Description: "FSM OnExit 清理完成标记"},

	// --- 需求系统（2）---
	{Name: "need_lowest", Type: "string", GroupName: "need", Label: "最低需求名", Description: "当前最低需求名"},
	{Name: "need_lowest_val", Type: "float", GroupName: "need", Label: "最低需求值", Description: "当前最低需求值"},

	// --- 情绪系统（2）---
	{Name: "emotion_dominant", Type: "string", GroupName: "emotion", Label: "主导情绪名", Description: "主导情绪名"},
	{Name: "emotion_dominant_val", Type: "float", GroupName: "emotion", Label: "主导情绪值", Description: "主导情绪值"},

	// --- 记忆系统（2）---
	{Name: "memory_count", Type: "integer", GroupName: "memory", Label: "记忆条目数", Description: "当前记忆条目数"},
	{Name: "memory_threat_value", Type: "float", GroupName: "memory", Label: "最高威胁记忆值", Description: "最高威胁记忆的 value"},

	// --- 社交系统（6）---
	{Name: "group_id", Type: "string", GroupName: "social", Label: "群组 ID", Description: "群组 ID"},
	{Name: "social_role", Type: "string", GroupName: "social", Label: "社交角色", Description: "社交角色（leader/follower 触发队形逻辑，其他自由值无队形行为）"},
	{Name: "leader_lost", Type: "bool", GroupName: "social", Label: "leader 失踪", Description: "leader 被移除"},
	{Name: "group_alert", Type: "bool", GroupName: "social", Label: "群组告警", Description: "同组有成员 Flee"},
	{Name: "follow_target_x", Type: "float", GroupName: "social", Label: "跟随目标 X", Description: "leader X 坐标（follower 用）"},
	{Name: "follow_target_z", Type: "float", GroupName: "social", Label: "跟随目标 Z", Description: "leader Z 坐标（follower 用）"},

	// --- 决策系统（4）---
	{Name: "decision_winner", Type: "string", GroupName: "decision", Label: "仲裁胜出维度", Description: "仲裁胜出维度（threat/needs/emotion）"},
	{Name: "threat_score", Type: "float", GroupName: "decision", Label: "威胁原始分", Description: "威胁原始分"},
	{Name: "need_score", Type: "float", GroupName: "decision", Label: "需求原始分", Description: "需求原始分"},
	{Name: "emotion_score", Type: "float", GroupName: "decision", Label: "情绪原始分", Description: "情绪原始分"},

	// --- 移动系统（3）---
	{Name: "move_state", Type: "string", GroupName: "move", Label: "移动状态", Description: "移动状态（idle/moving/arrived）"},
	{Name: "move_target_x", Type: "float", GroupName: "move", Label: "移动目标 X", Description: "当前移动目标 X"},
	{Name: "move_target_z", Type: "float", GroupName: "move", Label: "移动目标 Z", Description: "当前移动目标 Z"},
}

// seedRuntimeBbKeys 批量写入 31 条内置 runtime BB key（INSERT IGNORE 幂等）
func seedRuntimeBbKeys(ctx context.Context, db *sqlx.DB) error {
	const insertSQL = `
INSERT IGNORE INTO runtime_bb_keys (name, type, label, description, group_name, enabled, version, deleted, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, 1, 1, 0, NOW(), NOW())`

	inserted := 0
	skipped := 0
	for _, s := range runtimeBbKeyFixtures {
		result, err := db.ExecContext(ctx, insertSQL, s.Name, s.Type, s.Label, s.Description, s.GroupName)
		if err != nil {
			return fmt.Errorf("insert runtime_bb_key %q: %w", s.Name, err)
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			skipped++
			fmt.Printf("  [跳过] runtime_bb_key %s（已存在）\n", s.Name)
		} else {
			inserted++
		}
	}

	fmt.Printf("运行时 BB Key 写入完成：新增 %d 条，跳过 %d 条（已存在）\n", inserted, skipped)
	return nil
}
