package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/yqihe/npc-ai-admin/backend/internal/util"
)

// event_type_seed.go 覆盖 R15 smoke 第二批 blocker：
// 服务端 HTTPSource 对 /api/configs/event_types 空 items 硬失败
// （internal/config/http_source.go optional:false），ADMIN 冷启动后该表为空
// → Server 容器 crashloop。
//
// 5 条最小集由 Server CC 提供（2026-04-20），对齐服务端仓 configs/events/*.json。
// perception_mode 枚举：global / auditory / visual（util.ValidPerceptionModes）。
// config_json 对齐 EventTypeService.buildConfigJSON：display_name +
// default_severity + default_ttl + perception_mode + range。

type eventTypeSeed struct {
	Name            string
	DisplayName     string
	PerceptionMode  string
	DefaultSeverity float64
	DefaultTTL      float64
	Range           float64
}

var eventTypeFixtures = []eventTypeSeed{
	{
		Name: "earthquake", DisplayName: "地震",
		PerceptionMode: util.PerceptionModeGlobal,
		DefaultSeverity: 95, DefaultTTL: 30, Range: 0,
	},
	{
		Name: "explosion", DisplayName: "爆炸",
		PerceptionMode: util.PerceptionModeAuditory,
		DefaultSeverity: 80, DefaultTTL: 15, Range: 500,
	},
	{
		Name: "fire", DisplayName: "起火",
		PerceptionMode: util.PerceptionModeVisual,
		DefaultSeverity: 60, DefaultTTL: 20, Range: 150,
	},
	{
		Name: "gunshot", DisplayName: "枪声",
		PerceptionMode: util.PerceptionModeAuditory,
		DefaultSeverity: 90, DefaultTTL: 10, Range: 300,
	},
	{
		Name: "shout", DisplayName: "呼喊",
		PerceptionMode: util.PerceptionModeAuditory,
		DefaultSeverity: 30, DefaultTTL: 8, Range: 200,
	},
}

// seedEventTypes 幂等写入 5 条事件类型，enabled=1。
// 冲突策略：INSERT IGNORE — name 已存在则跳过。
func seedEventTypes(ctx context.Context, db *sqlx.DB) error {
	const insertSQL = `
INSERT IGNORE INTO event_types (name, display_name, perception_mode, config_json, enabled, version, deleted, created_at, updated_at)
VALUES (?, ?, ?, ?, 1, 1, 0, NOW(), NOW())`

	inserted := 0
	skipped := 0
	for _, s := range eventTypeFixtures {
		configMap := map[string]interface{}{
			"display_name":     s.DisplayName,
			"default_severity": s.DefaultSeverity,
			"default_ttl":      s.DefaultTTL,
			"perception_mode":  s.PerceptionMode,
			"range":            s.Range,
		}
		configJSON, err := json.Marshal(configMap)
		if err != nil {
			return fmt.Errorf("marshal event_type %q config: %w", s.Name, err)
		}

		result, err := db.ExecContext(ctx, insertSQL,
			s.Name, s.DisplayName, s.PerceptionMode, string(configJSON))
		if err != nil {
			return fmt.Errorf("insert event_type %q: %w", s.Name, err)
		}
		if rows, _ := result.RowsAffected(); rows > 0 {
			inserted++
		} else {
			skipped++
			fmt.Printf("  [跳过] 事件类型 %s（已存在）\n", s.Name)
		}
	}

	fmt.Printf("事件类型写入完成：新增 %d 条，跳过 %d 条（已存在）\n", inserted, skipped)
	return nil
}
