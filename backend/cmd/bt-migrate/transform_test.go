package main

import (
	"reflect"
	"testing"
)

// ============================================================
// transformNode 纯函数测试（T6）
//
// 覆盖 design §1.4 规则表 6 条规则 + 1 个 BT #4 完整树用例 + 幂等性。
// ============================================================

func TestTransformNode(t *testing.T) {
	cases := []struct {
		name     string
		input    map[string]any
		treeName string
		want     map[string]any
	}{
		{
			name:     "stub_action 裸 action → params.name + 补 default result",
			input:    map[string]any{"type": "stub_action", "action": "wait_idle"},
			treeName: "bt/combat/idle",
			want: map[string]any{
				"type":   "stub_action",
				"params": map[string]any{"name": "wait_idle", "result": "success"},
			},
		},
		{
			name:     "check_bb_float 裸字段（key 已规范）→ 收进 params",
			input:    map[string]any{"type": "check_bb_float", "op": ">", "value": float64(0), "key": "hp"},
			treeName: "bt/combat/attack",
			want: map[string]any{
				"type":   "check_bb_float",
				"params": map[string]any{"key": "hp", "op": ">", "value": float64(0)},
			},
		},
		{
			name:     "check_bb_float 裸字段 target_key → 迁移为 key",
			input:    map[string]any{"type": "check_bb_float", "op": ">", "value": float64(0), "target_key": "perception_range"},
			treeName: "bt/combat/chase",
			want: map[string]any{
				"type":   "check_bb_float",
				"params": map[string]any{"key": "perception_range", "op": ">", "value": float64(0)},
			},
		},
		{
			name: "已规范 stub_action + 剔除 category 游离字段",
			input: map[string]any{
				"type":     "stub_action",
				"params":   map[string]any{"name": "patrol_action", "result": "success"},
				"category": "leaf",
			},
			treeName: "guard/patrol",
			want: map[string]any{
				"type":   "stub_action",
				"params": map[string]any{"name": "patrol_action", "result": "success"},
			},
		},
		{
			name: "sequence 递归 children（裸 → 规范）",
			input: map[string]any{
				"type": "sequence",
				"children": []any{
					map[string]any{"type": "stub_action", "action": "wait_idle"},
					map[string]any{"type": "stub_action", "action": "look_around"},
				},
			},
			treeName: "bt/combat/idle",
			want: map[string]any{
				"type": "sequence",
				"children": []any{
					map[string]any{"type": "stub_action", "params": map[string]any{"name": "wait_idle", "result": "success"}},
					map[string]any{"type": "stub_action", "params": map[string]any{"name": "look_around", "result": "success"}},
				},
			},
		},
		{
			name: "BT #4 完整树 — 两个空 stub_action 按位置填占位",
			input: map[string]any{
				"type": "sequence",
				"children": []any{
					map[string]any{"op": ">", "key": "perception_range", "type": "check_bb_float", "value": float64(0)},
					map[string]any{"type": "stub_action"},
					map[string]any{"type": "stub_action"},
				},
			},
			treeName: "bt/combat/attack",
			want: map[string]any{
				"type": "sequence",
				"children": []any{
					map[string]any{"type": "check_bb_float", "params": map[string]any{"key": "perception_range", "op": ">", "value": float64(0)}},
					map[string]any{"type": "stub_action", "params": map[string]any{"name": "attack_prepare", "result": "success"}},
					map[string]any{"type": "stub_action", "params": map[string]any{"name": "attack_strike", "result": "success"}},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, _, err := transformNode(c.input, c.treeName, "$")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("transform mismatch\nwant: %#v\n got: %#v", c.want, got)
			}
		})
	}
}

// TestTransformNode_Idempotent 幂等性：已规范节点再次 transform 应得到相同结果
func TestTransformNode_Idempotent(t *testing.T) {
	already := map[string]any{
		"type": "sequence",
		"children": []any{
			map[string]any{
				"type":   "stub_action",
				"params": map[string]any{"name": "attack_prepare", "result": "success"},
			},
			map[string]any{
				"type":   "check_bb_float",
				"params": map[string]any{"key": "hp", "op": "<", "value": float64(30)},
			},
			map[string]any{
				"type":  "inverter",
				"child": map[string]any{"type": "stub_action", "params": map[string]any{"name": "X", "result": "success"}},
			},
		},
	}
	got, warnings, err := transformNode(already, "bt/combat/attack", "$")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, already) {
		t.Errorf("transform not idempotent\nbefore: %#v\nafter:  %#v", already, got)
	}
	if len(warnings) != 0 {
		t.Errorf("idempotent run should emit zero warnings, got %v", warnings)
	}
}

// TestTransformNode_ErrorCases 结构性致命错误
func TestTransformNode_ErrorCases(t *testing.T) {
	cases := []struct {
		name  string
		input map[string]any
	}{
		{"缺 type 字段", map[string]any{"children": []any{}}},
		{"type 非字符串", map[string]any{"type": float64(123)}},
		{"children 非数组", map[string]any{"type": "sequence", "children": "not-array"}},
		{"children 含非对象元素", map[string]any{"type": "sequence", "children": []any{"scalar"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := transformNode(c.input, "test", "$")
			if err == nil {
				t.Errorf("want error, got nil")
			}
		})
	}
}
