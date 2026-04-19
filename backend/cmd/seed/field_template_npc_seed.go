package main

// 外部契约数据 seed（对齐 docs/specs/external-contract-admin-shape-alignment/）
//
// T2 阶段：仅 seedFields（9 个字段，含 hp 孤儿 enabled=0）
// T3 阶段：将在本文件追加 seedTemplates / seedNPCs 并由 seedFieldsTemplatesNPCs 聚合调用

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jmoiron/sqlx"
	"github.com/yqihe/npc-ai-admin/backend/internal/util"
)

// 字段 name 常量（T2 seed + T3 模板 field_id 查找共用，避免裸字符串散落）
const (
	fieldNameMaxHp           = "max_hp"
	fieldNameMoveSpeed       = "move_speed"
	fieldNamePerceptionRange = "perception_range"
	fieldNameAttackPower     = "attack_power"
	fieldNameDefense         = "defense"
	fieldNameAggression      = "aggression"
	fieldNameIsBoss          = "is_boss"
	fieldNameLootTable       = "loot_table"
	fieldNameHp              = "hp"
)

// seedFieldsTemplatesNPCs 外部契约数据 seed 聚合入口。
// T2 阶段只执行 seedFields；T3 追加 seedTemplates / seedNPCs。
func seedFieldsTemplatesNPCs(ctx context.Context, db *sqlx.DB) error {
	if err := seedFields(ctx, db); err != nil {
		return fmt.Errorf("seed fields: %w", err)
	}
	return nil
}

// fieldSeed 描述一个字段的 seed 数据。
// Enabled 默认 true；孤儿字段（hp）显式为 false。
// DefaultValue / Constraints 为原始 JSON，嵌入 properties 时保留类型（不转字符串）。
type fieldSeed struct {
	Name         string
	Label        string
	Type         string
	Category     string
	ExposeBB     bool
	Enabled      bool
	Description  string
	DefaultValue json.RawMessage
	Constraints  json.RawMessage
}

// seedFields 幂等写入 9 个字段（8 正常 + hp 孤儿）。
// 冲突策略 INSERT IGNORE：name 已存在则跳过，不覆盖运营手改的 constraints/label。
// enabled 列单独控制（8 正常=1，hp=0），不走 FieldStore.Create 的硬编码 enabled=0 路径。
func seedFields(ctx context.Context, db *sqlx.DB) error {
	fields := []fieldSeed{
		{
			Name: fieldNameMaxHp, Label: "最大生命值", Type: util.FieldTypeFloat,
			Category: util.FieldCategoryBasic, ExposeBB: true, Enabled: true,
			Description:  "最大生命值（战斗系统的核心状态）",
			DefaultValue: json.RawMessage(`100`),
			Constraints:  json.RawMessage(`{"min":1,"max":10000}`),
		},
		{
			Name: fieldNameMoveSpeed, Label: "移动速度", Type: util.FieldTypeFloat,
			Category: util.FieldCategoryMovement, ExposeBB: true, Enabled: true,
			Description:  "每秒移动距离（单位/秒）",
			DefaultValue: json.RawMessage(`3.0`),
			Constraints:  json.RawMessage(`{"min":0,"max":20}`),
		},
		{
			Name: fieldNamePerceptionRange, Label: "感知范围", Type: util.FieldTypeFloat,
			Category: util.FieldCategoryPerception, ExposeBB: true, Enabled: true,
			Description:  "NPC 发现目标的最大距离",
			DefaultValue: json.RawMessage(`20.0`),
			Constraints:  json.RawMessage(`{"min":0,"max":200}`),
		},
		{
			Name: fieldNameAttackPower, Label: "攻击力", Type: util.FieldTypeFloat,
			Category: util.FieldCategoryCombat, ExposeBB: true, Enabled: true,
			Description:  "攻击造成的基础伤害",
			DefaultValue: json.RawMessage(`15.0`),
			Constraints:  json.RawMessage(`{"min":0,"max":9999}`),
		},
		{
			Name: fieldNameDefense, Label: "防御力", Type: util.FieldTypeFloat,
			Category: util.FieldCategoryCombat, ExposeBB: true, Enabled: true,
			Description:  "受到攻击的基础减伤",
			DefaultValue: json.RawMessage(`5.0`),
			Constraints:  json.RawMessage(`{"min":0,"max":9999}`),
		},
		{
			Name: fieldNameAggression, Label: "攻击性", Type: util.FieldTypeSelect,
			Category: util.FieldCategoryPersonality, ExposeBB: true, Enabled: true,
			Description:  "NPC 的攻击倾向（决定遭遇时的行为选择）",
			DefaultValue: json.RawMessage(`"neutral"`),
			Constraints: json.RawMessage(
				`{"options":[{"value":"aggressive","label":"主动攻击"},{"value":"neutral","label":"中立"},{"value":"passive","label":"被动"}],"minSelect":1,"maxSelect":1}`,
			),
		},
		{
			Name: fieldNameIsBoss, Label: "是否 Boss", Type: util.FieldTypeBoolean,
			Category: util.FieldCategoryCombat, ExposeBB: true, Enabled: true,
			Description:  "是否为 Boss 级 NPC（影响战斗规则与掉落）",
			DefaultValue: json.RawMessage(`false`),
			Constraints:  json.RawMessage(`{}`),
		},
		{
			Name: fieldNameLootTable, Label: "掉落表", Type: util.FieldTypeString,
			Category: util.FieldCategoryInteraction, ExposeBB: false, Enabled: true,
			Description:  "死亡后掉落表 ref，由服务端 loot 系统查询解析",
			DefaultValue: json.RawMessage(`""`),
			Constraints:  json.RawMessage(`{}`),
		},
		// 孤儿字段：仅为 guard_basic 兼容 snapshot §4 的 {hp: 100}
		// enabled=0 确保 UI 字段选择器默认隐藏；41008 解封后一次性清除
		// （memory project_guard_basic_hp_deferred.md）
		{
			Name: fieldNameHp, Label: "旧血量（历史遗留，请用 max_hp）", Type: util.FieldTypeFloat,
			Category: util.FieldCategoryBasic, ExposeBB: false, Enabled: false,
			Description:  "仅用于 guard_basic 兼容 snapshot，不建议引用；请用 max_hp",
			DefaultValue: json.RawMessage(`100`),
			Constraints:  json.RawMessage(`{}`),
		},
	}

	const insertSQL = `
INSERT IGNORE INTO fields (name, label, type, category, properties, expose_bb, enabled, version, deleted, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, 1, 0, NOW(), NOW())`

	inserted := 0
	skipped := 0
	for _, s := range fields {
		propsBytes, err := json.Marshal(map[string]any{
			"description":   s.Description,
			"expose_bb":     s.ExposeBB,
			"default_value": s.DefaultValue,
			"constraints":   s.Constraints,
		})
		if err != nil {
			return fmt.Errorf("marshal properties for field %q: %w", s.Name, err)
		}
		result, err := db.ExecContext(ctx, insertSQL,
			s.Name, s.Label, s.Type, s.Category, string(propsBytes), s.ExposeBB, s.Enabled,
		)
		if err != nil {
			return fmt.Errorf("insert field %q: %w", s.Name, err)
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			skipped++
			slog.Info("seed.字段.跳过", "name", s.Name, "reason", "已存在")
			fmt.Printf("  [跳过] 字段 %s（已存在）\n", s.Name)
		} else {
			inserted++
		}
	}

	fmt.Printf("字段写入完成：新增 %d 条，跳过 %d 条（已存在）\n", inserted, skipped)
	return nil
}
