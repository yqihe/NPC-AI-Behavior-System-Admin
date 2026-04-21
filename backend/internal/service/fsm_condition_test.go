package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
)

// ============================================================
// validateCondition — 纯递归校验方法，不访问 s.store/s.cache
//
// 测试用 zero-value *FsmConfigService（对齐 region_test.go 先例）。
// gocyclo=21 baseline debt：本测覆盖 5 类失败模式 + 叶合法 + 递归传播，
// 为后续 refactor 做 safety net（docs/specs/verify-pipeline 记载）。
// ============================================================

// mkLeafCond 构造叶节点（value 传 JSON 字符串，传空串=不设置）
func mkLeafCond(key, op, valueJSON, refKey string) model.FsmCondition {
	c := model.FsmCondition{Key: key, Op: op, RefKey: refKey}
	if valueJSON != "" {
		c.Value = json.RawMessage(valueJSON)
	}
	return c
}

func TestValidateCondition(t *testing.T) {
	s := &FsmConfigService{}
	const maxDepth = 5

	type tc struct {
		name      string
		cond      model.FsmCondition
		wantErr   bool
		wantSub   string // 错误消息子串（空则只断言 errCode）
	}
	tests := []tc{
		// ---- 空条件 ----
		{
			"空条件_全零值",
			model.FsmCondition{},
			false, "",
		},
		{
			"空条件_value=null_等同未设置",
			// Key 为空 → IsEmpty 看不到 Key；但 IsEmpty 只检查 Key/And/Or，
			// 所以 {Value: "null"} 走空分支
			model.FsmCondition{Value: json.RawMessage("null")},
			false, "",
		},

		// ---- 叶节点合法 ----
		{
			"叶_value合法",
			mkLeafCond("hp", ">", "50", ""),
			false, "",
		},
		{
			"叶_ref_key合法",
			mkLeafCond("hp", ">", "", "max_hp"),
			false, "",
		},
		{
			"叶_op=in",
			mkLeafCond("state", "in", `["Idle","Patrol"]`, ""),
			false, "",
		},

		// ---- 叶节点非法 ----
		{
			"叶_op非法",
			mkLeafCond("hp", "~=", "50", ""),
			true, "操作符",
		},
		{
			"叶_value和ref_key同时设置",
			mkLeafCond("hp", ">", "50", "max_hp"),
			true, "同时设置",
		},
		{
			"叶_op合法但value和ref_key都为空",
			model.FsmCondition{Key: "hp", Op: ">"},
			true, "同时为空",
		},
		{
			"叶_value=null_且ref_key为空_视为都空",
			// hasValue=false(null被排除)+hasRefKey=false → "同时为空"
			model.FsmCondition{Key: "hp", Op: ">", Value: json.RawMessage("null")},
			true, "同时为空",
		},

		// ---- 结构互斥 ----
		{
			"叶_和and同时出现",
			model.FsmCondition{
				Key: "hp", Op: ">", Value: json.RawMessage("50"),
				And: []model.FsmCondition{mkLeafCond("mp", ">", "10", "")},
			},
			true, "不能同时有 key 和 and/or",
		},
		{
			"叶_和or同时出现",
			model.FsmCondition{
				Key: "hp", Op: ">", Value: json.RawMessage("50"),
				Or: []model.FsmCondition{mkLeafCond("mp", ">", "10", "")},
			},
			true, "不能同时有 key 和 and/or",
		},
		{
			"and和or同时出现",
			model.FsmCondition{
				And: []model.FsmCondition{mkLeafCond("hp", ">", "50", "")},
				Or:  []model.FsmCondition{mkLeafCond("mp", ">", "10", "")},
			},
			true, "不能同时有 and 和 or",
		},

		// ---- 递归 ----
		{
			"and_所有子节点合法",
			model.FsmCondition{
				And: []model.FsmCondition{
					mkLeafCond("hp", ">", "50", ""),
					mkLeafCond("mp", ">", "10", ""),
				},
			},
			false, "",
		},
		{
			"or_所有子节点合法",
			model.FsmCondition{
				Or: []model.FsmCondition{
					mkLeafCond("hp", ">", "50", ""),
					mkLeafCond("mp", ">", "10", ""),
				},
			},
			false, "",
		},
		{
			"and_子节点非法向上传播",
			model.FsmCondition{
				And: []model.FsmCondition{
					mkLeafCond("hp", ">", "50", ""),
					mkLeafCond("mp", "~=", "10", ""), // 非法 op
				},
			},
			true, "操作符",
		},
		{
			"or_子节点非法向上传播",
			model.FsmCondition{
				Or: []model.FsmCondition{
					mkLeafCond("hp", "~=", "50", ""), // 非法 op
				},
			},
			true, "操作符",
		},
		{
			"嵌套and_or合法",
			model.FsmCondition{
				And: []model.FsmCondition{
					mkLeafCond("hp", ">", "50", ""),
					{Or: []model.FsmCondition{
						mkLeafCond("mp", ">", "10", ""),
						mkLeafCond("stamina", ">", "0", ""),
					}},
				},
			},
			false, "",
		},

		// ---- 深度限制（maxDepth=5；depth 自 0 起递增，>5 触发）----
		// buildNestedAnd(n) 构造 n 层 and 链末端叶；初次调用 depth=0 → 叶 depth=n-1。
		{
			"深度恰好到maxDepth",
			buildNestedAnd(6), // 叶 depth=5，5>5 为 false → PASS
			false, "",
		},
		{
			"深度超限",
			buildNestedAnd(7), // 叶 depth=6，6>5 → 触发
			true, "嵌套深度",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := s.validateCondition(&tt.cond, 0, maxDepth)
			if tt.wantErr {
				if got == nil {
					t.Fatalf("期望出错，实际返回 nil")
				}
				if got.Code != errcode.ErrFsmConfigConditionInvalid {
					t.Fatalf("期望 errcode=%d，实际 %d", errcode.ErrFsmConfigConditionInvalid, got.Code)
				}
				if tt.wantSub != "" && !strings.Contains(got.Message, tt.wantSub) {
					t.Fatalf("期望消息含 %q，实际 %q", tt.wantSub, got.Message)
				}
			} else {
				if got != nil {
					t.Fatalf("期望 nil，实际 %+v", got)
				}
			}
		})
	}
}

// buildNestedAnd 构造深度为 n 的 and 链末端带合法叶
// depth=1 → leaf; depth=2 → and[leaf]; depth=3 → and[and[leaf]]; ...
func buildNestedAnd(depth int) model.FsmCondition {
	if depth <= 1 {
		return mkLeafCond("hp", ">", "50", "")
	}
	return model.FsmCondition{
		And: []model.FsmCondition{buildNestedAnd(depth - 1)},
	}
}
