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
		{GroupName: model.DictGroupFieldType, Name: "integer", Label: "整数", SortOrder: 1, Extra: mustRawJSON(map[string]any{
			"constraint_schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"min":  map[string]any{"type": "number", "title": "最小值"},
					"max":  map[string]any{"type": "number", "title": "最大值"},
					"step": map[string]any{"type": "number", "title": "步长", "default": 1},
				},
			},
		})},
		{GroupName: model.DictGroupFieldType, Name: "float", Label: "浮点数", SortOrder: 2, Extra: mustRawJSON(map[string]any{
			"constraint_schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"min":       map[string]any{"type": "number", "title": "最小值"},
					"max":       map[string]any{"type": "number", "title": "最大值"},
					"precision": map[string]any{"type": "number", "title": "小数位数"},
				},
			},
		})},
		{GroupName: model.DictGroupFieldType, Name: "string", Label: "文本", SortOrder: 3, Extra: mustRawJSON(map[string]any{
			"constraint_schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"minLength": map[string]any{"type": "number", "title": "最小长度"},
					"maxLength": map[string]any{"type": "number", "title": "最大长度"},
					"pattern":   map[string]any{"type": "string", "title": "正则校验"},
				},
			},
		})},
		{GroupName: model.DictGroupFieldType, Name: "boolean", Label: "布尔", SortOrder: 4, Extra: mustRawJSON(map[string]any{
			"constraint_schema": map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		})},
		{GroupName: model.DictGroupFieldType, Name: "select", Label: "选择", SortOrder: 5, Extra: mustRawJSON(map[string]any{
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
		{GroupName: model.DictGroupFieldType, Name: "reference", Label: "引用", SortOrder: 6, Extra: mustRawJSON(map[string]any{
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
		{GroupName: model.DictGroupFieldCategory, Name: "basic", Label: "基础属性", SortOrder: 1},
		{GroupName: model.DictGroupFieldCategory, Name: "combat", Label: "战斗属性", SortOrder: 2},
		{GroupName: model.DictGroupFieldCategory, Name: "perception", Label: "感知属性", SortOrder: 3},
		{GroupName: model.DictGroupFieldCategory, Name: "movement", Label: "移动属性", SortOrder: 4},
		{GroupName: model.DictGroupFieldCategory, Name: "interaction", Label: "交互属性", SortOrder: 5},
		{GroupName: model.DictGroupFieldCategory, Name: "personality", Label: "个性属性", SortOrder: 6},
	}

	// field_properties: 4 种动态表单属性
	fieldProperties := []model.Dictionary{
		{GroupName: model.DictGroupFieldProperties, Name: "description", Label: "描述说明", SortOrder: 1, Extra: mustRawJSON(map[string]any{
			"input_type": "textarea", "required": false,
		})},
		{GroupName: model.DictGroupFieldProperties, Name: "expose_bb", Label: "暴露 BB Key", SortOrder: 2, Extra: mustRawJSON(map[string]any{
			"input_type": "radio_bool", "required": true, "default": false,
		})},
		{GroupName: model.DictGroupFieldProperties, Name: "default_value", Label: "默认值", SortOrder: 3, Extra: mustRawJSON(map[string]any{
			"input_type": "dynamic", "required": false,
		})},
		{GroupName: model.DictGroupFieldProperties, Name: "constraints", Label: "约束配置", SortOrder: 4, Extra: mustRawJSON(map[string]any{
			"input_type": "constraints", "required": false,
		})},
	}

	all := make([]model.Dictionary, 0, len(fieldTypes)+len(fieldCategories)+len(fieldProperties))
	all = append(all, fieldTypes...)
	all = append(all, fieldCategories...)
	all = append(all, fieldProperties...)

	if err := store.BatchCreate(ctx, all); err != nil {
		slog.Error("seed.写入种子数据失败", "error", err)
		os.Exit(1)
	}

	fmt.Printf("种子数据写入完成：%d 条\n", len(all))
}

func mustRawJSON(v any) *json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("marshal json: %v", err))
	}
	raw := json.RawMessage(b)
	return &raw
}
