package service

import (
	"errors"
	"testing"

	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
)

// ============================================================
// validateBtNode 硬化后的单元测试（T5）
//
// 测试 validator 数据驱动分支：顶层字段白名单 + params 必填 / 类型 / 枚举校验。
// validateBtNode 是 package-private 函数，本测试直接调用，
// 手工构造 nodeTypes / paramSchemas map 模拟 store 预加载结果。
// ============================================================

// 共享 fixture：模拟 seed 的 10 个节点类型里本测试会用到的那部分
var testNodeTypes = map[string]string{
	"sequence":       "composite",
	"selector":       "composite",
	"inverter":       "decorator",
	"stub_action":    "leaf",
	"check_bb_float": "leaf",
	"move_to":        "leaf",
}

var testParamSchemas = map[string]*nodeParamSchema{
	"stub_action": {Params: []paramSpec{
		{Name: "name", Type: "string", Required: true},
		{Name: "result", Type: "select", Options: []string{"success", "failure", "running"}, Required: true},
	}},
	"check_bb_float": {Params: []paramSpec{
		{Name: "key", Type: "bb_key", Required: true},
		{Name: "op", Type: "select", Options: []string{"<", "<=", ">", ">=", "==", "!="}, Required: true},
		{Name: "value", Type: "float", Required: true},
	}},
	"move_to": {Params: []paramSpec{
		{Name: "target_key_x", Type: "bb_key", Required: true},
		{Name: "target_key_z", Type: "bb_key", Required: true},
		{Name: "speed", Type: "float", Required: false},
	}},
}

func TestValidateBtNode_ParamsHardening(t *testing.T) {
	cases := []struct {
		name     string
		node     map[string]any
		wantCode int // 0 = 期望 nil error
	}{
		{
			name:     "顶层裸字段 action",
			node:     map[string]any{"type": "stub_action", "action": "wait_idle"},
			wantCode: errcode.ErrBtNodeBareFields,
		},
		{
			name:     "叶子缺 params",
			node:     map[string]any{"type": "check_bb_float"},
			wantCode: errcode.ErrBtNodeBareFields,
		},
		{
			name:     "params 是数组非对象",
			node:     map[string]any{"type": "stub_action", "params": []any{}},
			wantCode: errcode.ErrBtNodeBareFields,
		},
		{
			name: "params 缺必填 result",
			node: map[string]any{
				"type":   "stub_action",
				"params": map[string]any{"name": "X"},
			},
			wantCode: errcode.ErrBtNodeParamMissing,
		},
		{
			name: "params.name 类型错（number 而非 string）",
			node: map[string]any{
				"type":   "stub_action",
				"params": map[string]any{"name": float64(123), "result": "success"},
			},
			wantCode: errcode.ErrBtNodeParamType,
		},
		{
			name: "params.result 枚举非法",
			node: map[string]any{
				"type":   "stub_action",
				"params": map[string]any{"name": "X", "result": "maybe"},
			},
			wantCode: errcode.ErrBtNodeParamEnum,
		},
		{
			name: "bb_key 空串",
			node: map[string]any{
				"type":   "check_bb_float",
				"params": map[string]any{"key": "", "op": ">", "value": float64(1)},
			},
			wantCode: errcode.ErrBtNodeParamType,
		},
		{
			name: "最简合法 sequence + stub_action",
			node: map[string]any{
				"type": "sequence",
				"children": []any{
					map[string]any{
						"type":   "stub_action",
						"params": map[string]any{"name": "X", "result": "success"},
					},
				},
			},
			wantCode: 0,
		},
		{
			name: "move_to 合法",
			node: map[string]any{
				"type":   "move_to",
				"params": map[string]any{"target_key_x": "a", "target_key_z": "b"},
			},
			wantCode: 0,
		},
		{
			name: "move_to 缺 target_key_z",
			node: map[string]any{
				"type":   "move_to",
				"params": map[string]any{"target_key_x": "a"},
			},
			wantCode: errcode.ErrBtNodeParamMissing,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateBtNode(c.node, testNodeTypes, testParamSchemas, 0)
			if c.wantCode == 0 {
				if err != nil {
					t.Errorf("want nil error, got %v", err)
				}
				return
			}
			var codeErr *errcode.Error
			if !errors.As(err, &codeErr) {
				t.Fatalf("want *errcode.Error, got %T: %v", err, err)
			}
			if codeErr.Code != c.wantCode {
				t.Errorf("want code=%d, got code=%d (msg=%q)", c.wantCode, codeErr.Code, codeErr.Message)
			}
		})
	}
}
