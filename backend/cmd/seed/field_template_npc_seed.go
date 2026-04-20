package main

// 外部契约数据 seed（对齐 docs/specs/external-contract-admin-shape-alignment/）
//
// 幂等写入 9 字段 + 4 模板 + 6 NPC，全部走 INSERT IGNORE 语义。
// 由 main.go 在 dictionary / fsm_state / bt_node_type seed 完成后调用。

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/jmoiron/sqlx"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	"github.com/yqihe/npc-ai-admin/backend/internal/util"
)

// 字段 name 常量（seed + 模板 field_id 查找 + NPC 字段快照共用，避免裸字符串散落）
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
	// 5 个组件 opt-in bool 字段（api-contract.md v1.1 §组件 opt-in 依赖矩阵）
	fieldNameEnableMemory      = "enable_memory"
	fieldNameEnableEmotion     = "enable_emotion"
	fieldNameEnableNeeds       = "enable_needs"
	fieldNameEnablePersonality = "enable_personality"
	fieldNameEnableSocial      = "enable_social"
	// Social 组件字段值来源（Server admin_template.go:296-300 从 fields.group_id / fields.social_role 读）
	fieldNameGroupID    = "group_id"
	fieldNameSocialRole = "social_role"
)

// 模板 name 常量（NPC seed 通过 TemplateName 引用）
const (
	templateNameWarriorBase = "warrior_base"
	templateNameRangerBase  = "ranger_base"
	templateNamePassiveNPC  = "passive_npc"
	templateNameTplGuard    = "tpl_guard"
)

// NPC name 常量（R12 要求 npc name 走 const，避免裸字符串散落）
const (
	npcNameWolfCommon       = "wolf_common"
	npcNameWolfAlpha        = "wolf_alpha"
	npcNameGoblinArcher     = "goblin_archer"
	npcNameVillagerMerchant = "villager_merchant"
	npcNameVillagerGuard    = "villager_guard"
	npcNameGuardBasic       = "guard_basic"
)

// FSM ref 常量（snapshot §4 实际使用值）
const (
	fsmRefCombatBasic = "fsm_combat_basic"
	fsmRefPassive     = "fsm_passive"
	fsmRefGuard       = "guard"
)

// 行为树 tree name 常量（snapshot §4 bt_refs 值）
const (
	btTreeCombatAttack   = "bt/combat/attack"
	btTreeCombatChase    = "bt/combat/chase"
	btTreeCombatIdle     = "bt/combat/idle"
	btTreeCombatPatrol   = "bt/combat/patrol"
	btTreePassiveWander  = "bt/passive/wander"
	btTreeGuardPatrol    = "bt/guard/patrol"
)

// seedFieldsTemplatesNPCs 外部契约数据 seed 聚合入口。
// 依次：字段 → 模板（含 field_refs）→ NPC（含 npc_bt_refs）。
// 每步幂等，冲突跳过不报错。
func seedFieldsTemplatesNPCs(ctx context.Context, db *sqlx.DB) error {
	if err := seedFields(ctx, db); err != nil {
		return fmt.Errorf("seed fields: %w", err)
	}
	if err := seedTemplates(ctx, db); err != nil {
		return fmt.Errorf("seed templates: %w", err)
	}
	if err := seedNPCs(ctx, db); err != nil {
		return fmt.Errorf("seed npcs: %w", err)
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
		// 5 个组件 opt-in bool 字段（api-contract.md v1.1 §组件 opt-in 依赖矩阵）
		// expose_bb=false：服务端启动期读取，不进 BB；default_value=false 锁定 absent≡false 语义
		{
			Name: fieldNameEnableMemory, Label: "启用记忆", Type: util.FieldTypeBoolean,
			Category: util.FieldCategoryComponent, ExposeBB: false, Enabled: true,
			Description:  "是否启用记忆能力（写入威胁记忆，是 emotion 的前置）",
			DefaultValue: json.RawMessage(`false`),
			Constraints:  json.RawMessage(`{}`),
		},
		{
			Name: fieldNameEnableEmotion, Label: "启用情绪", Type: util.FieldTypeBoolean,
			Category: util.FieldCategoryComponent, ExposeBB: false, Enabled: true,
			Description:  "是否启用情绪能力（读记忆累积 fear，需同时启用 memory）",
			DefaultValue: json.RawMessage(`false`),
			Constraints:  json.RawMessage(`{}`),
		},
		{
			Name: fieldNameEnableNeeds, Label: "启用需求", Type: util.FieldTypeBoolean,
			Category: util.FieldCategoryComponent, ExposeBB: false, Enabled: true,
			Description:  "是否启用需求能力（计算最低需求，驱动决策）",
			DefaultValue: json.RawMessage(`false`),
			Constraints:  json.RawMessage(`{}`),
		},
		{
			Name: fieldNameEnablePersonality, Label: "启用性格", Type: util.FieldTypeBoolean,
			Category: util.FieldCategoryComponent, ExposeBB: false, Enabled: true,
			Description:  "是否启用性格能力（覆盖默认决策权重）",
			DefaultValue: json.RawMessage(`false`),
			Constraints:  json.RawMessage(`{}`),
		},
		{
			Name: fieldNameEnableSocial, Label: "启用社交", Type: util.FieldTypeBoolean,
			Category: util.FieldCategoryComponent, ExposeBB: false, Enabled: true,
			Description:  "是否启用社交能力（group/follower/leader 机制）",
			DefaultValue: json.RawMessage(`false`),
			Constraints:  json.RawMessage(`{}`),
		},
		// Social 组件字段值（启用 social 时配合使用；白名单由 Server 侧 SocialFactory 校验）
		{
			Name: fieldNameGroupID, Label: "社交组 ID", Type: util.FieldTypeString,
			Category: util.FieldCategoryComponent, ExposeBB: false, Enabled: true,
			Description:  "社交分组标识（如派系名）；仅在 enable_social=true 时生效",
			DefaultValue: json.RawMessage(`""`),
			Constraints:  json.RawMessage(`{}`),
		},
		{
			Name: fieldNameSocialRole, Label: "社交角色", Type: util.FieldTypeString,
			Category: util.FieldCategoryComponent, ExposeBB: false, Enabled: true,
			Description:  "社交角色标识（如 leader/follower/trader）；仅在 enable_social=true 时生效",
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

// ──────────────────────────────────────────────
// 模板 seed
// ──────────────────────────────────────────────

// templateSeed 描述一个模板的 seed 数据。
// FieldNames 按 NPC 表单展示顺序排列；tpl_guard 占位模板为空切片。
type templateSeed struct {
	Name        string
	Label       string
	Description string
	FieldNames  []string
}

// seedTemplates 幂等写入 4 个模板（warrior_base / ranger_base / passive_npc / tpl_guard）
// 并补建 field_refs（ref_type='template'）。
//
// 冲突策略：templates 表 INSERT IGNORE，冲突跳过不覆盖运营手改的 label/description/fields。
// 无论 templates 是否新插入，都对现有 template_id 尝试 INSERT IGNORE field_refs，
// 确保引用关系总能补齐（field_refs PK 天然幂等）。
func seedTemplates(ctx context.Context, db *sqlx.DB) error {
	templates := []templateSeed{
		{
			Name: templateNameWarriorBase, Label: "战士基础模板",
			Description: "战士类 NPC 的字段集合（含 3 个 opt-in 组件开关，默认 false）",
			FieldNames: []string{
				fieldNameAggression, fieldNameAttackPower, fieldNameDefense,
				fieldNameIsBoss, fieldNameLootTable, fieldNameMaxHp,
				fieldNameMoveSpeed, fieldNamePerceptionRange,
				fieldNameEnableMemory, fieldNameEnableEmotion, fieldNameEnablePersonality,
			},
		},
		{
			Name: templateNameRangerBase, Label: "游侠基础模板",
			Description: "游侠类 NPC 字段集合（无 is_boss）",
			FieldNames: []string{
				fieldNameAggression, fieldNameAttackPower, fieldNameDefense,
				fieldNameLootTable, fieldNameMaxHp, fieldNameMoveSpeed,
				fieldNamePerceptionRange,
			},
		},
		{
			Name: templateNamePassiveNPC, Label: "被动 NPC 模板",
			Description: "非战斗 NPC 最小字段集（含 social opt-in + group_id/social_role 配合字段）",
			FieldNames: []string{
				fieldNameAggression, fieldNameMaxHp,
				fieldNameMoveSpeed, fieldNamePerceptionRange,
				fieldNameEnableSocial, fieldNameGroupID, fieldNameSocialRole,
			},
		},
		{
			Name: templateNameTplGuard, Label: "守卫历史模板",
			Description: "历史遗留占位模板，仅为兼容 guard_basic；41008 解封后一次性清除",
			FieldNames: []string{}, // tpl_guard.fields = []（make([]TemplateFieldEntry, 0) 避免 nil→null）
		},
	}

	// 一次性加载 9 字段 name→id 映射，避免逐模板反复查询
	fieldIDs, err := loadFieldIDMap(ctx, db)
	if err != nil {
		return fmt.Errorf("load field ids for templates: %w", err)
	}

	const insertSQL = `
INSERT IGNORE INTO templates (name, label, description, fields, enabled, version, deleted, created_at, updated_at)
VALUES (?, ?, ?, ?, 1, 1, 0, NOW(), NOW())`

	const selectIDSQL = `SELECT id FROM templates WHERE name = ? AND deleted = 0`

	const insertRefSQL = `
INSERT IGNORE INTO field_refs (field_id, ref_type, ref_id) VALUES (?, ?, ?)`

	inserted := 0
	skipped := 0
	refInserted := 0
	for _, t := range templates {
		// 构造 template.fields JSON：保持顺序 + 非 nil（空数组也用 []TemplateFieldEntry{}）
		entries := make([]model.TemplateFieldEntry, 0, len(t.FieldNames))
		for _, fname := range t.FieldNames {
			id, ok := fieldIDs[fname]
			if !ok {
				return fmt.Errorf("template %q references unknown field %q (seedFields must run first)", t.Name, fname)
			}
			entries = append(entries, model.TemplateFieldEntry{FieldID: id, Required: false})
		}
		fieldsJSON, err := json.Marshal(entries)
		if err != nil {
			return fmt.Errorf("marshal template fields for %q: %w", t.Name, err)
		}

		result, err := db.ExecContext(ctx, insertSQL, t.Name, t.Label, t.Description, string(fieldsJSON))
		if err != nil {
			return fmt.Errorf("insert template %q: %w", t.Name, err)
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			skipped++
			slog.Info("seed.模板.跳过", "name", t.Name, "reason", "已存在")
			fmt.Printf("  [跳过] 模板 %s（已存在）\n", t.Name)
		} else {
			inserted++
		}

		// 取 template_id（新插入或已存在都要拿到）用于 field_refs 补建
		var templateID int64
		if err := db.GetContext(ctx, &templateID, selectIDSQL, t.Name); err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("template %q not found after insert (concurrent delete?)", t.Name)
			}
			return fmt.Errorf("lookup template id for %q: %w", t.Name, err)
		}

		// 对每个 field 补 field_refs（ref_type='template'），INSERT IGNORE 幂等
		for _, entry := range entries {
			refResult, err := db.ExecContext(ctx, insertRefSQL, entry.FieldID, util.RefTypeTemplate, templateID)
			if err != nil {
				return fmt.Errorf("insert field_ref (field=%d, template=%d): %w", entry.FieldID, templateID, err)
			}
			if refRows, _ := refResult.RowsAffected(); refRows > 0 {
				refInserted++
			}
		}
	}

	fmt.Printf("模板写入完成：新增 %d 条，跳过 %d 条（已存在），field_refs 新增 %d 条\n",
		inserted, skipped, refInserted)
	return nil
}

// ──────────────────────────────────────────────
// NPC seed
// ──────────────────────────────────────────────

// npcFieldValue 描述 NPC fields 快照中的单个字段取值。
type npcFieldValue struct {
	FieldName string
	Value     json.RawMessage
}

// npcSeed 描述一个 NPC 实例的 seed 数据。
// FieldValues 顺序即 fields JSON 数组顺序（与模板 FieldNames 对齐；guard_basic 例外：
// 引用的是孤儿 hp 字段，不在 tpl_guard.fields 内）。
type npcSeed struct {
	Name         string
	Label        string
	TemplateName string
	FieldValues  []npcFieldValue
	FsmRef       string
	BtRefs       map[string]string
}

// seedNPCs 幂等写入 6 个 NPC（snapshot §4 冻结数据）并展开 npc_bt_refs。
//
// guard_basic 的 fields 仅含 hp（OQ3 方案 A：hp 孤儿字段 enabled=0 但仍可被 NPC 引用）。
// 其余 NPC 按模板 FieldNames 顺序组装快照（snapshot §4 数值）。
func seedNPCs(ctx context.Context, db *sqlx.DB) error {
	// 预加载 field/template name→id
	fieldIDs, err := loadFieldIDMap(ctx, db)
	if err != nil {
		return fmt.Errorf("load field ids for npcs: %w", err)
	}
	templateIDs, err := loadTemplateIDMap(ctx, db)
	if err != nil {
		return fmt.Errorf("load template ids for npcs: %w", err)
	}

	// 4 个战斗 NPC 共用的 bt_refs（warrior_base / ranger_base）
	combatBtRefs := map[string]string{
		"attack": btTreeCombatAttack,
		"chase":  btTreeCombatChase,
		"idle":   btTreeCombatIdle,
		"patrol": btTreeCombatPatrol,
	}

	npcs := []npcSeed{
		{
			Name: npcNameWolfCommon, Label: "普通狼",
			TemplateName: templateNameWarriorBase,
			FieldValues: []npcFieldValue{
				{FieldName: fieldNameAggression, Value: json.RawMessage(`"aggressive"`)},
				{FieldName: fieldNameAttackPower, Value: json.RawMessage(`18.5`)},
				{FieldName: fieldNameDefense, Value: json.RawMessage(`8.0`)},
				{FieldName: fieldNameIsBoss, Value: json.RawMessage(`false`)},
				{FieldName: fieldNameLootTable, Value: json.RawMessage(`"loot_wolf_common"`)},
				{FieldName: fieldNameMaxHp, Value: json.RawMessage(`120`)},
				{FieldName: fieldNameMoveSpeed, Value: json.RawMessage(`5.5`)},
				{FieldName: fieldNamePerceptionRange, Value: json.RawMessage(`20.0`)},
				{FieldName: fieldNameEnableMemory, Value: json.RawMessage(`false`)},
				{FieldName: fieldNameEnableEmotion, Value: json.RawMessage(`false`)},
				{FieldName: fieldNameEnablePersonality, Value: json.RawMessage(`false`)},
			},
			FsmRef: fsmRefCombatBasic, BtRefs: combatBtRefs,
		},
		{
			Name: npcNameWolfAlpha, Label: "头狼",
			TemplateName: templateNameWarriorBase,
			FieldValues: []npcFieldValue{
				{FieldName: fieldNameAggression, Value: json.RawMessage(`"aggressive"`)},
				{FieldName: fieldNameAttackPower, Value: json.RawMessage(`45.0`)},
				{FieldName: fieldNameDefense, Value: json.RawMessage(`25.0`)},
				{FieldName: fieldNameIsBoss, Value: json.RawMessage(`true`)},
				{FieldName: fieldNameLootTable, Value: json.RawMessage(`"loot_wolf_alpha"`)},
				{FieldName: fieldNameMaxHp, Value: json.RawMessage(`800`)},
				{FieldName: fieldNameMoveSpeed, Value: json.RawMessage(`6.0`)},
				{FieldName: fieldNamePerceptionRange, Value: json.RawMessage(`30.0`)},
				// Phase 2 demo：boss 开记忆 + 情绪（记仇 + 愤怒累积）
				{FieldName: fieldNameEnableMemory, Value: json.RawMessage(`true`)},
				{FieldName: fieldNameEnableEmotion, Value: json.RawMessage(`true`)},
				{FieldName: fieldNameEnablePersonality, Value: json.RawMessage(`false`)},
			},
			FsmRef: fsmRefCombatBasic, BtRefs: combatBtRefs,
		},
		{
			Name: npcNameGoblinArcher, Label: "哥布林弓手",
			TemplateName: templateNameRangerBase,
			FieldValues: []npcFieldValue{
				{FieldName: fieldNameAggression, Value: json.RawMessage(`"aggressive"`)},
				{FieldName: fieldNameAttackPower, Value: json.RawMessage(`22.0`)},
				{FieldName: fieldNameDefense, Value: json.RawMessage(`3.0`)},
				{FieldName: fieldNameLootTable, Value: json.RawMessage(`"loot_goblin"`)},
				{FieldName: fieldNameMaxHp, Value: json.RawMessage(`60`)},
				{FieldName: fieldNameMoveSpeed, Value: json.RawMessage(`4.0`)},
				{FieldName: fieldNamePerceptionRange, Value: json.RawMessage(`35.0`)},
			},
			FsmRef: fsmRefCombatBasic, BtRefs: combatBtRefs,
		},
		{
			Name: npcNameVillagerMerchant, Label: "村庄商人",
			TemplateName: templateNamePassiveNPC,
			FieldValues: []npcFieldValue{
				{FieldName: fieldNameAggression, Value: json.RawMessage(`"passive"`)},
				{FieldName: fieldNameMaxHp, Value: json.RawMessage(`100`)},
				{FieldName: fieldNameMoveSpeed, Value: json.RawMessage(`2.0`)},
				{FieldName: fieldNamePerceptionRange, Value: json.RawMessage(`10.0`)},
				// Phase 2 demo：村民派系商人，social 路径覆盖
				{FieldName: fieldNameEnableSocial, Value: json.RawMessage(`true`)},
				{FieldName: fieldNameGroupID, Value: json.RawMessage(`"merchant_guild"`)},
				{FieldName: fieldNameSocialRole, Value: json.RawMessage(`"trader"`)},
			},
			FsmRef: fsmRefPassive,
			BtRefs: map[string]string{
				"idle":   btTreeCombatIdle,
				"wander": btTreePassiveWander,
			},
		},
		{
			Name: npcNameVillagerGuard, Label: "村卫兵",
			TemplateName: templateNameWarriorBase,
			FieldValues: []npcFieldValue{
				{FieldName: fieldNameAggression, Value: json.RawMessage(`"neutral"`)},
				{FieldName: fieldNameAttackPower, Value: json.RawMessage(`15.0`)},
				{FieldName: fieldNameDefense, Value: json.RawMessage(`20.0`)},
				{FieldName: fieldNameIsBoss, Value: json.RawMessage(`false`)},
				{FieldName: fieldNameLootTable, Value: json.RawMessage(`""`)},
				{FieldName: fieldNameMaxHp, Value: json.RawMessage(`200`)},
				{FieldName: fieldNameMoveSpeed, Value: json.RawMessage(`3.0`)},
				{FieldName: fieldNamePerceptionRange, Value: json.RawMessage(`25.0`)},
				// Phase 2 demo：中立守卫开性格（复用 aggression=neutral 驱动 decision_weights）
				{FieldName: fieldNameEnableMemory, Value: json.RawMessage(`false`)},
				{FieldName: fieldNameEnableEmotion, Value: json.RawMessage(`false`)},
				{FieldName: fieldNameEnablePersonality, Value: json.RawMessage(`true`)},
			},
			FsmRef: fsmRefCombatBasic, BtRefs: combatBtRefs,
		},
		// guard_basic：唯一引用孤儿 hp 字段的 NPC（tpl_guard.fields=[] 但本 NPC 快照带 hp）
		{
			Name: npcNameGuardBasic, Label: "基础守卫",
			TemplateName: templateNameTplGuard,
			FieldValues: []npcFieldValue{
				{FieldName: fieldNameHp, Value: json.RawMessage(`100`)},
			},
			FsmRef: fsmRefGuard,
			BtRefs: map[string]string{"patrol": btTreeGuardPatrol},
		},
	}

	const insertNPCSQL = `
INSERT IGNORE INTO npcs (name, label, description, template_id, template_name, fields, fsm_ref, bt_refs,
                         enabled, version, created_at, updated_at, deleted)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, 1, NOW(), NOW(), 0)`

	const selectNPCIDSQL = `SELECT id FROM npcs WHERE name = ? AND deleted = 0`

	const insertBtRefSQL = `
INSERT IGNORE INTO npc_bt_refs (npc_id, bt_tree_name) VALUES (?, ?)`

	inserted := 0
	skipped := 0
	btRefInserted := 0
	for _, n := range npcs {
		templateID, ok := templateIDs[n.TemplateName]
		if !ok {
			return fmt.Errorf("npc %q references unknown template %q (seedTemplates must run first)", n.Name, n.TemplateName)
		}

		// 组装 fields JSON: []{field_id, name, required, value}
		entries := make([]model.NPCFieldEntry, 0, len(n.FieldValues))
		for _, v := range n.FieldValues {
			fid, ok := fieldIDs[v.FieldName]
			if !ok {
				return fmt.Errorf("npc %q references unknown field %q", n.Name, v.FieldName)
			}
			entries = append(entries, model.NPCFieldEntry{
				FieldID:  fid,
				Name:     v.FieldName,
				Required: false,
				Value:    v.Value,
			})
		}
		fieldsJSON, err := json.Marshal(entries)
		if err != nil {
			return fmt.Errorf("marshal npc fields for %q: %w", n.Name, err)
		}

		// bt_refs JSON（非 nil：空 map 也 marshal 为 {}，但本 seed 数据中 BtRefs 永不为空）
		btRefsJSON, err := json.Marshal(n.BtRefs)
		if err != nil {
			return fmt.Errorf("marshal npc bt_refs for %q: %w", n.Name, err)
		}

		result, err := db.ExecContext(ctx, insertNPCSQL,
			n.Name, n.Label, "", templateID, n.TemplateName,
			string(fieldsJSON), n.FsmRef, string(btRefsJSON),
		)
		if err != nil {
			return fmt.Errorf("insert npc %q: %w", n.Name, err)
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			skipped++
			slog.Info("seed.NPC.跳过", "name", n.Name, "reason", "已存在")
			fmt.Printf("  [跳过] NPC %s（已存在）\n", n.Name)
		} else {
			inserted++
		}

		// 取 npc_id（新插入或已存在都要拿到）用于 npc_bt_refs 补建
		var npcID int64
		if err := db.GetContext(ctx, &npcID, selectNPCIDSQL, n.Name); err != nil {
			if err == sql.ErrNoRows {
				return fmt.Errorf("npc %q not found after insert (concurrent delete?)", n.Name)
			}
			return fmt.Errorf("lookup npc id for %q: %w", n.Name, err)
		}

		// 展开 bt_refs 到 npc_bt_refs（去重：多个 state 可引用同一棵树）
		seen := make(map[string]struct{}, len(n.BtRefs))
		for _, tree := range n.BtRefs {
			if tree == "" {
				continue
			}
			if _, dup := seen[tree]; dup {
				continue
			}
			seen[tree] = struct{}{}
			btResult, err := db.ExecContext(ctx, insertBtRefSQL, npcID, tree)
			if err != nil {
				return fmt.Errorf("insert npc_bt_ref (npc=%d, tree=%q): %w", npcID, tree, err)
			}
			if btRows, _ := btResult.RowsAffected(); btRows > 0 {
				btRefInserted++
			}
		}
	}

	fmt.Printf("NPC 写入完成：新增 %d 条，跳过 %d 条（已存在），npc_bt_refs 新增 %d 条\n",
		inserted, skipped, btRefInserted)
	return nil
}

// ──────────────────────────────────────────────
// 辅助：name→id 映射加载
// ──────────────────────────────────────────────

// loadFieldIDMap 查询本 seed 涉及的 16 个字段 name→id 映射。
// 包括 hp 孤儿字段（enabled=0 但 deleted=0 仍可查到）以及 Phase 2 新增的 5 opt-in
// bool + 2 social string 字段。
func loadFieldIDMap(ctx context.Context, db *sqlx.DB) (map[string]int64, error) {
	names := []string{
		fieldNameMaxHp, fieldNameMoveSpeed, fieldNamePerceptionRange,
		fieldNameAttackPower, fieldNameDefense, fieldNameAggression,
		fieldNameIsBoss, fieldNameLootTable, fieldNameHp,
		fieldNameEnableMemory, fieldNameEnableEmotion, fieldNameEnableNeeds,
		fieldNameEnablePersonality, fieldNameEnableSocial,
		fieldNameGroupID, fieldNameSocialRole,
	}
	query, args, err := sqlx.In(`SELECT id, name FROM fields WHERE name IN (?) AND deleted=0`, names)
	if err != nil {
		return nil, fmt.Errorf("build field IN query: %w", err)
	}
	query = db.Rebind(query)

	type row struct {
		ID   int64  `db:"id"`
		Name string `db:"name"`
	}
	var rows []row
	if err := db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("query field ids: %w", err)
	}
	result := make(map[string]int64, len(rows))
	for _, r := range rows {
		result[r.Name] = r.ID
	}
	return result, nil
}

// loadTemplateIDMap 查询本 seed 涉及的 4 个模板 name→id 映射。
func loadTemplateIDMap(ctx context.Context, db *sqlx.DB) (map[string]int64, error) {
	names := []string{
		templateNameWarriorBase, templateNameRangerBase,
		templateNamePassiveNPC, templateNameTplGuard,
	}
	query, args, err := sqlx.In(`SELECT id, name FROM templates WHERE name IN (?) AND deleted=0`, names)
	if err != nil {
		return nil, fmt.Errorf("build template IN query: %w", err)
	}
	query = db.Rebind(query)

	type row struct {
		ID   int64  `db:"id"`
		Name string `db:"name"`
	}
	var rows []row
	if err := db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("query template ids: %w", err)
	}
	result := make(map[string]int64, len(rows))
	for _, r := range rows {
		result[r.Name] = r.ID
	}
	return result, nil
}
