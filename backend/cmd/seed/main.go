package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	"github.com/yqihe/npc-ai-admin/backend/internal/util"
	"github.com/yqihe/npc-ai-admin/backend/internal/store/mysql"
)

func main() {
	configPath := flag.String("config", "config.yaml", "配置文件路径")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		slog.Error("seed.加载配置失败", "error", err)
		os.Exit(1)
	}

	db, err := sqlx.Connect("mysql", cfg.MySQL.DSN)
	if err != nil {
		slog.Error("seed.连接MySQL失败", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store := mysql.NewDictionaryStore(db)

	// field_type: 6 种字段类型
	fieldTypes := []model.Dictionary{
		{GroupName: util.DictGroupFieldType, Name: "integer", Label: "整数", SortOrder: 1, Extra: mustRawJSON(map[string]any{
			"constraint_schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"min":  map[string]any{"type": "number", "title": "最小值"},
					"max":  map[string]any{"type": "number", "title": "最大值"},
					"step": map[string]any{"type": "number", "title": "步长", "default": 1},
				},
			},
		})},
		{GroupName: util.DictGroupFieldType, Name: "float", Label: "浮点数", SortOrder: 2, Extra: mustRawJSON(map[string]any{
			"constraint_schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"min":       map[string]any{"type": "number", "title": "最小值"},
					"max":       map[string]any{"type": "number", "title": "最大值"},
					"precision": map[string]any{"type": "number", "title": "小数位数"},
				},
			},
		})},
		{GroupName: util.DictGroupFieldType, Name: "string", Label: "文本", SortOrder: 3, Extra: mustRawJSON(map[string]any{
			"constraint_schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"minLength": map[string]any{"type": "number", "title": "最小长度"},
					"maxLength": map[string]any{"type": "number", "title": "最大长度"},
					"pattern":   map[string]any{"type": "string", "title": "正则校验"},
				},
			},
		})},
		{GroupName: util.DictGroupFieldType, Name: "boolean", Label: "布尔", SortOrder: 4, Extra: mustRawJSON(map[string]any{
			"constraint_schema": map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		})},
		{GroupName: util.DictGroupFieldType, Name: "select", Label: "选择", SortOrder: 5, Extra: mustRawJSON(map[string]any{
			"constraint_schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"options": map[string]any{
						"type":  "array",
						"title": "选项列表",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"value": map[string]any{"type": "string", "title": "值"},
								"label": map[string]any{"type": "string", "title": "标签"},
							},
						},
					},
					"minSelect": map[string]any{"type": "number", "title": "最少选择数", "default": 1},
					"maxSelect": map[string]any{"type": "number", "title": "最多选择数", "default": 1},
				},
			},
		})},
		{GroupName: util.DictGroupFieldType, Name: "reference", Label: "引用", SortOrder: 6, Extra: mustRawJSON(map[string]any{
			"constraint_schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"refs": map[string]any{
						"type":  "array",
						"title": "引用字段列表",
						"items": map[string]any{"type": "integer"},
					},
				},
			},
		})},
	}

	// field_category: 6 种标签分类
	fieldCategories := []model.Dictionary{
		{GroupName: util.DictGroupFieldCategory, Name: "basic", Label: "基础属性", SortOrder: 1},
		{GroupName: util.DictGroupFieldCategory, Name: "combat", Label: "战斗属性", SortOrder: 2},
		{GroupName: util.DictGroupFieldCategory, Name: "perception", Label: "感知属性", SortOrder: 3},
		{GroupName: util.DictGroupFieldCategory, Name: "movement", Label: "移动属性", SortOrder: 4},
		{GroupName: util.DictGroupFieldCategory, Name: "interaction", Label: "交互属性", SortOrder: 5},
		{GroupName: util.DictGroupFieldCategory, Name: "personality", Label: "个性属性", SortOrder: 6},
		{GroupName: util.DictGroupFieldCategory, Name: "component", Label: "能力开关", SortOrder: 7},
	}

	// field_properties: 4 种动态表单属性
	fieldProperties := []model.Dictionary{
		{GroupName: util.DictGroupFieldProperties, Name: "description", Label: "描述说明", SortOrder: 1, Extra: mustRawJSON(map[string]any{
			"input_type": "textarea", "required": false,
		})},
		{GroupName: util.DictGroupFieldProperties, Name: "expose_bb", Label: "暴露 BB Key", SortOrder: 2, Extra: mustRawJSON(map[string]any{
			"input_type": "radio_bool", "required": true, "default": false,
		})},
		{GroupName: util.DictGroupFieldProperties, Name: "default_value", Label: "默认值", SortOrder: 3, Extra: mustRawJSON(map[string]any{
			"input_type": "dynamic", "required": false,
		})},
		{GroupName: util.DictGroupFieldProperties, Name: "constraints", Label: "约束配置", SortOrder: 4, Extra: mustRawJSON(map[string]any{
			"input_type": "constraints", "required": false,
		})},
	}

	// fsm_state_category: 5 种状态分类（name 为英文标识符，label 为中文显示名）
	fsmStateCategories := []model.Dictionary{
		{GroupName: util.DictGroupFsmStateCategory, Name: "general",  Label: "通用", SortOrder: 1},
		{GroupName: util.DictGroupFsmStateCategory, Name: "combat",   Label: "战斗", SortOrder: 2},
		{GroupName: util.DictGroupFsmStateCategory, Name: "movement", Label: "移动", SortOrder: 3},
		{GroupName: util.DictGroupFsmStateCategory, Name: "social",   Label: "社交", SortOrder: 4},
		{GroupName: util.DictGroupFsmStateCategory, Name: "activity", Label: "活动", SortOrder: 5},
	}

	all := make([]model.Dictionary, 0, len(fieldTypes)+len(fieldCategories)+len(fieldProperties)+len(fsmStateCategories))
	all = append(all, fieldTypes...)
	all = append(all, fieldCategories...)
	all = append(all, fieldProperties...)
	all = append(all, fsmStateCategories...)

	if err := store.BatchCreate(ctx, all); err != nil {
		slog.Error("seed.写入种子数据失败", "error", err)
		os.Exit(1)
	}

	fmt.Printf("种子数据写入完成：%d 条\n", len(all))

	// FSM 状态字典种子
	if err := seedFsmStateDicts(ctx, db); err != nil {
		slog.Error("seed.状态字典写入失败", "error", err)
		os.Exit(1)
	}

	// 修复存量数据：将 dictionaries 和 fsm_state_dicts 中的中文 category 改为英文标识符
	if err := migrateFsmStateCategoryToEnglish(ctx, db); err != nil {
		slog.Error("seed.分类迁移失败", "error", err)
		os.Exit(1)
	}

	// 内置行为树节点类型种子
	if err := seedBtNodeTypes(ctx, db); err != nil {
		slog.Error("seed.内置节点类型写入失败", "error", err)
		os.Exit(1)
	}

	// FSM 配置 + 行为树种子（冷启动覆盖，填平 NPC 硬引用的 3 FSM + 6 BT）
	// 见 docs/specs/seed-fsm-bt-coverage/
	if err := seedFsmConfigs(ctx, db); err != nil {
		slog.Error("seed.FSM 配置写入失败", "error", err)
		os.Exit(1)
	}
	if err := seedBtTrees(ctx, db); err != nil {
		slog.Error("seed.行为树写入失败", "error", err)
		os.Exit(1)
	}

	// 事件类型种子（服务端 HTTPSource 对空 items 硬失败，冷启动必须非空）
	// 见 docs/specs/seed-fsm-bt-coverage/（第二批）
	if err := seedEventTypes(ctx, db); err != nil {
		slog.Error("seed.事件类型写入失败", "error", err)
		os.Exit(1)
	}

	// 外部契约数据种子（字段 + 模板 + NPC），对齐联调 snapshot §4
	// 见 docs/specs/external-contract-admin-shape-alignment/
	if err := seedFieldsTemplatesNPCs(ctx, db); err != nil {
		slog.Error("seed.外部契约数据写入失败", "error", err)
		os.Exit(1)
	}

	// 运行时 BB Key 种子（31 条内置 key，对齐 Server keys.go）
	// 见 docs/specs/bb-key-runtime-registry/
	if err := seedRuntimeBbKeys(ctx, db); err != nil {
		slog.Error("seed.运行时 BB Key 写入失败", "error", err)
		os.Exit(1)
	}

	// 区域类型字典 + village_outskirts fixture（必须在 seedFieldsTemplatesNPCs 之后）
	// 见 docs/specs/regions-module/
	if err := seedRegionTypeDict(ctx, db); err != nil {
		slog.Error("seed.区域类型字典写入失败", "error", err)
		os.Exit(1)
	}
	if err := seedRegions(ctx, db); err != nil {
		slog.Error("seed.区域种子写入失败", "error", err)
		os.Exit(1)
	}

	printPostSeedWarning()
}

// printPostSeedWarning 提示运行者 seed 完成后需同步缓存状态，
// 避免已启动的 admin-backend 读到旧数据。
func printPostSeedWarning() {
	fmt.Println()
	fmt.Println("⚠️  若 admin-backend 已启动，请重启或清 Redis 缓存以同步新数据")
}

func mustRawJSON(v any) *json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("marshal json: %v", err))
	}
	raw := json.RawMessage(b)
	return &raw
}

type fsmStateDictSeed struct {
	Name        string
	DisplayName string
	Category    string
	Description string
}

func seedFsmStateDicts(ctx context.Context, db *sqlx.DB) error {
	seeds := []fsmStateDictSeed{
		// general（4 条）
		{Name: "idle", DisplayName: "空闲", Category: "general", Description: "NPC 无事可做时的默认待机状态"},
		{Name: "moving", DisplayName: "移动中", Category: "general", Description: "NPC 正在向目标位置移动"},
		{Name: "interacting", DisplayName: "交互中", Category: "general", Description: "NPC 正在与对象或玩家交互"},
		{Name: "busy", DisplayName: "忙碌", Category: "general", Description: "NPC 正在执行占用行动槽的任务"},

		// combat（11 条）
		{Name: "alert", DisplayName: "警戒", Category: "combat", Description: "NPC 发现威胁，进入警觉状态"},
		{Name: "engage", DisplayName: "接战", Category: "combat", Description: "NPC 选定目标并准备发起攻击"},
		{Name: "attack_melee", DisplayName: "近战攻击", Category: "combat", Description: "NPC 执行近战攻击动作"},
		{Name: "attack_ranged", DisplayName: "远程攻击", Category: "combat", Description: "NPC 执行远程攻击动作"},
		{Name: "cast_spell", DisplayName: "施法", Category: "combat", Description: "NPC 正在释放技能或法术"},
		{Name: "dodge", DisplayName: "闪避", Category: "combat", Description: "NPC 执行闪避/回避动作"},
		{Name: "stagger", DisplayName: "硬直", Category: "combat", Description: "NPC 受击后进入短暂硬直状态"},
		{Name: "dying", DisplayName: "濒死", Category: "combat", Description: "NPC 生命值极低，进入濒死状态"},
		{Name: "dead", DisplayName: "死亡", Category: "combat", Description: "NPC 已死亡"},
		{Name: "flee", DisplayName: "逃跑", Category: "combat", Description: "NPC 判定无法胜出，选择逃离"},
		{Name: "revive", DisplayName: "复活", Category: "combat", Description: "NPC 从死亡或濒死状态恢复"},

		// movement（6 条）
		{Name: "patrol", DisplayName: "巡逻", Category: "movement", Description: "NPC 沿预设路径或区域巡逻"},
		{Name: "wander", DisplayName: "游荡", Category: "movement", Description: "NPC 在一定范围内随机漫步"},
		{Name: "chase", DisplayName: "追击", Category: "movement", Description: "NPC 追踪并靠近目标"},
		{Name: "return_home", DisplayName: "返回原点", Category: "movement", Description: "NPC 返回出生点或指定锚点"},
		{Name: "follow", DisplayName: "跟随", Category: "movement", Description: "NPC 跟随指定目标移动"},
		{Name: "escort", DisplayName: "护送", Category: "movement", Description: "NPC 护送目标前往指定位置"},

		// social（5 条）
		{Name: "greet", DisplayName: "打招呼", Category: "social", Description: "NPC 主动向玩家或其他 NPC 打招呼"},
		{Name: "talk", DisplayName: "对话中", Category: "social", Description: "NPC 正在进行对话交流"},
		{Name: "trade", DisplayName: "交易中", Category: "social", Description: "NPC 正在与玩家进行商品交易"},
		{Name: "quest_offer", DisplayName: "发布任务", Category: "social", Description: "NPC 向玩家提供或说明任务"},
		{Name: "farewell", DisplayName: "告别", Category: "social", Description: "NPC 结束交互并告别"},

		// activity（5 条）
		{Name: "sleep", DisplayName: "睡眠", Category: "activity", Description: "NPC 正在休息或睡眠"},
		{Name: "eat", DisplayName: "进食", Category: "activity", Description: "NPC 正在进食"},
		{Name: "sit", DisplayName: "坐下", Category: "activity", Description: "NPC 处于坐姿休息状态"},
		{Name: "craft", DisplayName: "制作", Category: "activity", Description: "NPC 正在制作道具或物品"},
		{Name: "gather", DisplayName: "采集", Category: "activity", Description: "NPC 正在采集资源"},
	}

	const insertSQL = `
INSERT IGNORE INTO fsm_state_dicts (name, display_name, category, description, enabled, version, deleted, created_at, updated_at)
VALUES (?, ?, ?, ?, 1, 1, 0, NOW(), NOW())`

	skipped := 0
	inserted := 0
	for _, s := range seeds {
		result, err := db.ExecContext(ctx, insertSQL, s.Name, s.DisplayName, s.Category, s.Description)
		if err != nil {
			return fmt.Errorf("insert fsm_state_dict %q: %w", s.Name, err)
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			skipped++
			fmt.Printf("  [跳过] %s（已存在）\n", s.Name)
		} else {
			inserted++
		}
	}

	fmt.Printf("FSM 状态字典写入完成：新增 %d 条，跳过 %d 条（已存在）\n", inserted, skipped)
	return nil
}

// migrateFsmStateCategoryToEnglish 清理 dictionaries 中残留的中文 name 条目，
// 并将 fsm_state_dicts.category 从中文值迁移为英文标识符（幂等，可重复执行）。
//
// 迁移策略：
//   - dictionaries：English name 条目由 BatchCreate（INSERT IGNORE）确保存在；
//     旧中文 name 条目直接 DELETE（UPDATE 因 unique 约束冲突无法改名）。
//   - fsm_state_dicts：直接 UPDATE category 中文值 → 英文值。
func migrateFsmStateCategoryToEnglish(ctx context.Context, db *sqlx.DB) error {
	zhNames := []string{"通用", "战斗", "移动", "社交", "活动"}
	zhToEn := map[string]string{
		"通用": "general",
		"战斗": "combat",
		"移动": "movement",
		"社交": "social",
		"活动": "activity",
	}

	// 删除旧中文 name 字典条目（English 条目已由 INSERT IGNORE 插入）
	delDict := int64(0)
	for _, zh := range zhNames {
		res, err := db.ExecContext(ctx,
			`DELETE FROM dictionaries WHERE group_name=? AND name=?`,
			util.DictGroupFsmStateCategory, zh)
		if err != nil {
			return fmt.Errorf("delete dict entry %q: %w", zh, err)
		}
		n, _ := res.RowsAffected()
		delDict += n
	}

	// 将 fsm_state_dicts.category 中文值更新为英文
	totalState := int64(0)
	for zh, en := range zhToEn {
		res, err := db.ExecContext(ctx,
			`UPDATE fsm_state_dicts SET category=?, updated_at=NOW() WHERE category=?`,
			en, zh)
		if err != nil {
			return fmt.Errorf("migrate fsm_state_dicts category %q→%q: %w", zh, en, err)
		}
		n, _ := res.RowsAffected()
		totalState += n
	}

	fmt.Printf("分类迁移完成：删除旧字典条目 %d 行，状态字典更新 %d 行\n", delDict, totalState)
	return nil
}
