package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jmoiron/sqlx"
)

type btNodeTypeSeed struct {
	TypeName    string
	Category    string
	Label       string
	Description string
	ParamSchema json.RawMessage
}

var builtinNodeTypes = []btNodeTypeSeed{
	{
		TypeName:    "sequence",
		Category:    "composite",
		Label:       "序列",
		Description: "顺序执行子节点，全部成功才成功，任一失败立即停止",
		ParamSchema: json.RawMessage(`{"params":[]}`),
	},
	{
		TypeName:    "selector",
		Category:    "composite",
		Label:       "选择器",
		Description: "顺序执行子节点，第一个成功即返回成功，全部失败才失败",
		ParamSchema: json.RawMessage(`{"params":[]}`),
	},
	{
		TypeName:    "parallel",
		Category:    "composite",
		Label:       "并行",
		Description: "同时执行全部子节点",
		ParamSchema: json.RawMessage(`{"params":[]}`),
	},
	{
		TypeName:    "inverter",
		Category:    "decorator",
		Label:       "取反",
		Description: "翻转子节点的执行结果（成功↔失败）",
		ParamSchema: json.RawMessage(`{"params":[]}`),
	},
	{
		TypeName:    "check_bb_float",
		Category:    "leaf",
		Label:       "检查浮点 BB",
		Description: "读取 Blackboard 浮点值并与阈值比较",
		ParamSchema: json.RawMessage(`{"params":[{"name":"key","label":"BB Key","type":"bb_key","required":true},{"name":"op","label":"操作符","type":"select","options":["<","<=",">",">=","==","!="],"required":true},{"name":"value","label":"比较值","type":"float","required":true}]}`),
	},
	{
		TypeName:    "check_bb_string",
		Category:    "leaf",
		Label:       "检查字符串 BB",
		Description: "读取 Blackboard 字符串值并与目标值比较",
		ParamSchema: json.RawMessage(`{"params":[{"name":"key","label":"BB Key","type":"bb_key","required":true},{"name":"op","label":"操作符","type":"select","options":["==","!="],"required":true},{"name":"value","label":"比较值","type":"string","required":true}]}`),
	},
	{
		TypeName:    "set_bb_value",
		Category:    "leaf",
		Label:       "设置 BB 值",
		Description: "向 Blackboard 写入指定 Key 的值",
		ParamSchema: json.RawMessage(`{"params":[{"name":"key","label":"BB Key","type":"bb_key","required":true},{"name":"value","label":"设定值","type":"string","required":true}]}`),
	},
	{
		TypeName:    "stub_action",
		Category:    "leaf",
		Label:       "存根动作",
		Description: "占位动作节点，返回固定结果（调试/占位用）",
		ParamSchema: json.RawMessage(`{"params":[{"name":"name","label":"动作名","type":"string","required":true},{"name":"result","label":"返回结果","type":"select","options":["success","failure","running"],"required":true}]}`),
	},
}

// seedBtNodeTypes 幂等写入内置节点类型。
// 按 type_name 检查存在性，已存在则跳过，不存在则 INSERT（is_builtin=1）。
func seedBtNodeTypes(ctx context.Context, db *sqlx.DB) error {
	const checkSQL = `SELECT COUNT(*) FROM bt_node_types WHERE type_name=? AND deleted=0`
	const insertSQL = `
INSERT INTO bt_node_types (type_name, category, label, description, param_schema, is_builtin, enabled, version, deleted, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, 1, 1, 1, 0, NOW(), NOW())`

	inserted := 0
	skipped := 0
	for _, s := range builtinNodeTypes {
		var count int
		if err := db.QueryRowContext(ctx, checkSQL, s.TypeName).Scan(&count); err != nil {
			return fmt.Errorf("check bt_node_type %q: %w", s.TypeName, err)
		}
		if count > 0 {
			skipped++
			fmt.Printf("  [跳过] %s（已存在）\n", s.TypeName)
			continue
		}
		if _, err := db.ExecContext(ctx, insertSQL,
			s.TypeName, s.Category, s.Label, s.Description, string(s.ParamSchema),
		); err != nil {
			return fmt.Errorf("insert bt_node_type %q: %w", s.TypeName, err)
		}
		inserted++
	}

	fmt.Printf("内置节点类型写入完成：新增 %d 条，跳过 %d 条（已存在）\n", inserted, skipped)
	return nil
}
