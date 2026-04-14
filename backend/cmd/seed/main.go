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

	// fsm_state_category: 5 种状态分类
	// name 使用中文，与 fsm_state_dicts.category 字段值保持一致（软约束）
	fsmStateCategories := []model.Dictionary{
		{GroupName: util.DictGroupFsmStateCategory, Name: "通用", Label: "通用", SortOrder: 1},
		{GroupName: util.DictGroupFsmStateCategory, Name: "战斗", Label: "战斗", SortOrder: 2},
		{GroupName: util.DictGroupFsmStateCategory, Name: "移动", Label: "移动", SortOrder: 3},
		{GroupName: util.DictGroupFsmStateCategory, Name: "社交", Label: "社交", SortOrder: 4},
		{GroupName: util.DictGroupFsmStateCategory, Name: "活动", Label: "活动", SortOrder: 5},
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
		// 通用（4 条）
		{Name: "idle", DisplayName: "空闲", Category: "通用", Description: "NPC 无事可做时的默认待机状态"},
		{Name: "moving", DisplayName: "移动中", Category: "通用", Description: "NPC 正在向目标位置移动"},
		{Name: "interacting", DisplayName: "交互中", Category: "通用", Description: "NPC 正在与对象或玩家交互"},
		{Name: "busy", DisplayName: "忙碌", Category: "通用", Description: "NPC 正在执行占用行动槽的任务"},

		// 战斗（11 条）
		{Name: "alert", DisplayName: "警戒", Category: "战斗", Description: "NPC 发现威胁，进入警觉状态"},
		{Name: "engage", DisplayName: "接战", Category: "战斗", Description: "NPC 选定目标并准备发起攻击"},
		{Name: "attack_melee", DisplayName: "近战攻击", Category: "战斗", Description: "NPC 执行近战攻击动作"},
		{Name: "attack_ranged", DisplayName: "远程攻击", Category: "战斗", Description: "NPC 执行远程攻击动作"},
		{Name: "cast_spell", DisplayName: "施法", Category: "战斗", Description: "NPC 正在释放技能或法术"},
		{Name: "dodge", DisplayName: "闪避", Category: "战斗", Description: "NPC 执行闪避/回避动作"},
		{Name: "stagger", DisplayName: "硬直", Category: "战斗", Description: "NPC 受击后进入短暂硬直状态"},
		{Name: "dying", DisplayName: "濒死", Category: "战斗", Description: "NPC 生命值极低，进入濒死状态"},
		{Name: "dead", DisplayName: "死亡", Category: "战斗", Description: "NPC 已死亡"},
		{Name: "flee", DisplayName: "逃跑", Category: "战斗", Description: "NPC 判定无法胜出，选择逃离"},
		{Name: "revive", DisplayName: "复活", Category: "战斗", Description: "NPC 从死亡或濒死状态恢复"},

		// 移动（6 条）
		{Name: "patrol", DisplayName: "巡逻", Category: "移动", Description: "NPC 沿预设路径或区域巡逻"},
		{Name: "wander", DisplayName: "游荡", Category: "移动", Description: "NPC 在一定范围内随机漫步"},
		{Name: "chase", DisplayName: "追击", Category: "移动", Description: "NPC 追踪并靠近目标"},
		{Name: "return_home", DisplayName: "返回原点", Category: "移动", Description: "NPC 返回出生点或指定锚点"},
		{Name: "follow", DisplayName: "跟随", Category: "移动", Description: "NPC 跟随指定目标移动"},
		{Name: "escort", DisplayName: "护送", Category: "移动", Description: "NPC 护送目标前往指定位置"},

		// 社交（5 条）
		{Name: "greet", DisplayName: "打招呼", Category: "社交", Description: "NPC 主动向玩家或其他 NPC 打招呼"},
		{Name: "talk", DisplayName: "对话中", Category: "社交", Description: "NPC 正在进行对话交流"},
		{Name: "trade", DisplayName: "交易中", Category: "社交", Description: "NPC 正在与玩家进行商品交易"},
		{Name: "quest_offer", DisplayName: "发布任务", Category: "社交", Description: "NPC 向玩家提供或说明任务"},
		{Name: "farewell", DisplayName: "告别", Category: "社交", Description: "NPC 结束交互并告别"},

		// 活动（5 条）
		{Name: "sleep", DisplayName: "睡眠", Category: "活动", Description: "NPC 正在休息或睡眠"},
		{Name: "eat", DisplayName: "进食", Category: "活动", Description: "NPC 正在进食"},
		{Name: "sit", DisplayName: "坐下", Category: "活动", Description: "NPC 处于坐姿休息状态"},
		{Name: "craft", DisplayName: "制作", Category: "活动", Description: "NPC 正在制作道具或物品"},
		{Name: "gather", DisplayName: "采集", Category: "活动", Description: "NPC 正在采集资源"},
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
