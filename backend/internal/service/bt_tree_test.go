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

// ============================================================
// validateBtNode 结构分支 safety net（补 params 硬化以外的分支）
//
// 目标：深度/type 字段/category 互斥/递归传播 = 完整覆盖结构校验路径，
// 为后续按 category 分派拆分提供零行为变更保证。
// ============================================================

func TestValidateBtNode_Structural(t *testing.T) {
	// stub_action 的合法 leaf（复用为 children/child 填充物）
	validLeaf := map[string]any{
		"type":   "stub_action",
		"params": map[string]any{"name": "X", "result": "success"},
	}

	cases := []struct {
		name     string
		node     map[string]any
		depth    int
		wantCode int // 0 = 期望 nil error
	}{
		// ---- 深度边界 ----
		{
			name:     "深度恰好=20_合法",
			node:     validLeaf,
			depth:    20,
			wantCode: 0,
		},
		{
			name:     "深度=21_超限",
			node:     validLeaf,
			depth:    21,
			wantCode: errcode.ErrBtTreeNodeDepthExceeded,
		},

		// ---- type 字段 ----
		{
			name:     "缺 type 字段",
			node:     map[string]any{"params": map[string]any{}},
			wantCode: errcode.ErrBtTreeConfigInvalid,
		},
		{
			name:     "type 非字符串",
			node:     map[string]any{"type": float64(123)},
			wantCode: errcode.ErrBtTreeConfigInvalid,
		},
		{
			name:     "type 空字符串",
			node:     map[string]any{"type": ""},
			wantCode: errcode.ErrBtTreeConfigInvalid,
		},
		{
			name:     "type 不在 nodeTypes",
			node:     map[string]any{"type": "unknown_type"},
			wantCode: errcode.ErrBtTreeNodeTypeNotFound,
		},

		// ---- composite 分支 ----
		{
			name:     "composite 缺 children",
			node:     map[string]any{"type": "sequence"},
			wantCode: errcode.ErrBtTreeConfigInvalid,
		},
		{
			name:     "composite children 空数组",
			node:     map[string]any{"type": "sequence", "children": []any{}},
			wantCode: errcode.ErrBtTreeConfigInvalid,
		},
		{
			name:     "composite children 非数组类型",
			node:     map[string]any{"type": "sequence", "children": "not-array"},
			wantCode: errcode.ErrBtTreeConfigInvalid,
		},
		{
			name: "composite 同时有 child",
			node: map[string]any{
				"type":     "sequence",
				"children": []any{validLeaf},
				"child":    validLeaf,
			},
			wantCode: errcode.ErrBtTreeConfigInvalid,
		},
		{
			name: "composite children 元素非对象",
			node: map[string]any{
				"type":     "sequence",
				"children": []any{"not-a-map"},
			},
			wantCode: errcode.ErrBtTreeConfigInvalid,
		},
		{
			name: "composite 子节点非法向上传播",
			node: map[string]any{
				"type": "sequence",
				"children": []any{
					map[string]any{"type": "unknown_type"},
				},
			},
			wantCode: errcode.ErrBtTreeNodeTypeNotFound,
		},

		// ---- decorator 分支 ----
		{
			name:     "decorator 缺 child",
			node:     map[string]any{"type": "inverter"},
			wantCode: errcode.ErrBtTreeConfigInvalid,
		},
		{
			name:     "decorator child=nil",
			node:     map[string]any{"type": "inverter", "child": nil},
			wantCode: errcode.ErrBtTreeConfigInvalid,
		},
		{
			name:     "decorator child 非对象",
			node:     map[string]any{"type": "inverter", "child": "not-a-map"},
			wantCode: errcode.ErrBtTreeConfigInvalid,
		},
		{
			name: "decorator 同时有 children",
			node: map[string]any{
				"type":     "inverter",
				"child":    validLeaf,
				"children": []any{validLeaf},
			},
			wantCode: errcode.ErrBtTreeConfigInvalid,
		},
		{
			name: "decorator 子节点非法向上传播",
			node: map[string]any{
				"type":  "inverter",
				"child": map[string]any{"type": "unknown_type"},
			},
			wantCode: errcode.ErrBtTreeNodeTypeNotFound,
		},
		{
			name: "decorator 合法（包裹 leaf）",
			node: map[string]any{
				"type":  "inverter",
				"child": validLeaf,
			},
			wantCode: 0,
		},

		// ---- leaf 分支 ----
		{
			name: "leaf 带 children 字段",
			node: map[string]any{
				"type":     "stub_action",
				"params":   map[string]any{"name": "X", "result": "success"},
				"children": []any{validLeaf},
			},
			// 顶层白名单在 children/child 拦截前放行，这里走 leaf 分支 → ErrBtTreeConfigInvalid
			wantCode: errcode.ErrBtTreeConfigInvalid,
		},
		{
			name: "leaf 带 child 字段",
			node: map[string]any{
				"type":   "stub_action",
				"params": map[string]any{"name": "X", "result": "success"},
				"child":  validLeaf,
			},
			wantCode: errcode.ErrBtTreeConfigInvalid,
		},

		// ---- 嵌套合法 ----
		{
			name: "合法 sequence + inverter + leaf",
			node: map[string]any{
				"type": "sequence",
				"children": []any{
					map[string]any{
						"type":  "inverter",
						"child": validLeaf,
					},
					validLeaf,
				},
			},
			wantCode: 0,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateBtNode(c.node, testNodeTypes, testParamSchemas, c.depth)
			if c.wantCode == 0 {
				if err != nil {
					t.Fatalf("want nil error, got %v", err)
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
