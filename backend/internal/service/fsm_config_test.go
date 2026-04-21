package service

import (
	"encoding/json"
	"testing"

	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
)

// ============================================================
// FSM config 纯辅助方法测试：buildConfigJSON / validateConfig /
// collectConditionKeys / ExtractBBKeys / ExtractBBKeysFromConfigJSON。
//
// 均无 DB/cache 依赖；buildConfigJSON 不访问 struct 字段可用 zero-value，
// validateConfig 只读 s.fsmCfg 上限字段，本地构造即可。
// ============================================================

// 复用 fsm_condition_test.go 里的 mkLeafCond

func mkStates(names ...string) []model.FsmState {
	out := make([]model.FsmState, len(names))
	for i, n := range names {
		out[i] = model.FsmState{Name: n}
	}
	return out
}

// --- buildConfigJSON ---

func TestFsmBuildConfigJSON(t *testing.T) {
	s := &FsmConfigService{}

	t.Run("完整 states + transitions 正常", func(t *testing.T) {
		cfg := []model.FsmTransition{
			{From: "Idle", To: "Patrol", Priority: 1,
				Condition: mkLeafCond("hp", ">", "50", "")},
		}
		data, err := s.buildConfigJSON("Idle", mkStates("Idle", "Patrol"), cfg)
		if err != nil {
			t.Fatalf("want nil err, got %v", err)
		}
		var m map[string]interface{}
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if m["initial_state"] != "Idle" {
			t.Errorf("initial_state 错: %v", m["initial_state"])
		}
		states, ok := m["states"].([]interface{})
		if !ok || len(states) != 2 {
			t.Fatalf("states 序列化错: %v", m["states"])
		}
		trs, ok := m["transitions"].([]interface{})
		if !ok || len(trs) != 1 {
			t.Fatalf("transitions 序列化错: %v", m["transitions"])
		}
	})

	t.Run("空状态空转换也可序列化", func(t *testing.T) {
		data, err := s.buildConfigJSON("", nil, nil)
		if err != nil {
			t.Fatalf("want nil err, got %v", err)
		}
		// 至少能解析回来
		var m map[string]interface{}
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if m["initial_state"] != "" {
			t.Errorf("initial_state 应为空串: %v", m["initial_state"])
		}
	})
}

// --- validateConfig ---

func TestFsmValidateConfig(t *testing.T) {
	// 宽松上限：单测不测上限逻辑（除 1 个专门用例），避免被无关字段影响
	s := &FsmConfigService{
		fsmCfg: &config.FsmConfigConfig{
			MaxStates:         50,
			MaxTransitions:    100,
			ConditionMaxDepth: 10,
		},
	}
	validCond := mkLeafCond("hp", ">", "50", "")

	cases := []struct {
		name        string
		initial     string
		states      []model.FsmState
		transitions []model.FsmTransition
		wantCode    int
	}{
		// ---- states 校验 ----
		{
			name:     "states 为空",
			initial:  "Idle",
			states:   nil,
			wantCode: errcode.ErrFsmConfigStatesEmpty,
		},
		{
			name:     "状态名空串",
			initial:  "Idle",
			states:   []model.FsmState{{Name: ""}},
			wantCode: errcode.ErrFsmConfigStateNameInvalid,
		},
		{
			name:     "状态名重复",
			initial:  "Idle",
			states:   mkStates("Idle", "Idle"),
			wantCode: errcode.ErrFsmConfigStateNameInvalid,
		},

		// ---- initial_state 校验 ----
		{
			name:     "initial_state 不在 states",
			initial:  "Unknown",
			states:   mkStates("Idle", "Patrol"),
			wantCode: errcode.ErrFsmConfigInitialInvalid,
		},

		// ---- transitions 校验 ----
		{
			name:    "transition.from 不存在",
			initial: "Idle",
			states:  mkStates("Idle", "Patrol"),
			transitions: []model.FsmTransition{
				{From: "Ghost", To: "Patrol", Priority: 0, Condition: validCond},
			},
			wantCode: errcode.ErrFsmConfigTransitionInvalid,
		},
		{
			name:    "transition.to 不存在",
			initial: "Idle",
			states:  mkStates("Idle", "Patrol"),
			transitions: []model.FsmTransition{
				{From: "Idle", To: "Ghost", Priority: 0, Condition: validCond},
			},
			wantCode: errcode.ErrFsmConfigTransitionInvalid,
		},
		{
			name:    "transition.priority < 0",
			initial: "Idle",
			states:  mkStates("Idle", "Patrol"),
			transitions: []model.FsmTransition{
				{From: "Idle", To: "Patrol", Priority: -1, Condition: validCond},
			},
			wantCode: errcode.ErrFsmConfigTransitionInvalid,
		},
		{
			name:    "condition 非法传播",
			initial: "Idle",
			states:  mkStates("Idle", "Patrol"),
			transitions: []model.FsmTransition{
				{From: "Idle", To: "Patrol", Priority: 0,
					Condition: mkLeafCond("hp", "unsupported_op", "50", "")},
			},
			wantCode: errcode.ErrFsmConfigConditionInvalid,
		},

		// ---- 合法 ----
		{
			name:    "最简合法（单状态+无转换）",
			initial: "Idle",
			states:  mkStates("Idle"),
			wantCode: 0,
		},
		{
			name:    "完整合法（多状态+多转换）",
			initial: "Idle",
			states:  mkStates("Idle", "Patrol", "Combat"),
			transitions: []model.FsmTransition{
				{From: "Idle", To: "Patrol", Priority: 1, Condition: validCond},
				{From: "Patrol", To: "Combat", Priority: 2,
					Condition: mkLeafCond("enemy_seen", "==", "true", "")},
			},
			wantCode: 0,
		},
		{
			name:    "空条件合法（transition 无条件即自动触发）",
			initial: "Idle",
			states:  mkStates("Idle", "Patrol"),
			transitions: []model.FsmTransition{
				{From: "Idle", To: "Patrol", Priority: 0, Condition: model.FsmCondition{}},
			},
			wantCode: 0,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := s.validateConfig(c.initial, c.states, c.transitions)
			if c.wantCode == 0 {
				if err != nil {
					t.Fatalf("want nil, got %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("want err code=%d, got nil", c.wantCode)
			}
			if err.Code != c.wantCode {
				t.Errorf("want code=%d, got code=%d (msg=%q)", c.wantCode, err.Code, err.Message)
			}
		})
	}
}

func TestFsmValidateConfig_StatesCountLimit(t *testing.T) {
	// MaxStates=2，给 3 个 → 超限
	s := &FsmConfigService{
		fsmCfg: &config.FsmConfigConfig{
			MaxStates:         2,
			MaxTransitions:    100,
			ConditionMaxDepth: 10,
		},
	}
	err := s.validateConfig("A", mkStates("A", "B", "C"), nil)
	if err == nil || err.Code != errcode.ErrFsmConfigStatesEmpty {
		t.Fatalf("want ErrFsmConfigStatesEmpty, got %v", err)
	}
}

func TestFsmValidateConfig_TransitionsCountLimit(t *testing.T) {
	s := &FsmConfigService{
		fsmCfg: &config.FsmConfigConfig{
			MaxStates:         50,
			MaxTransitions:    1, // 允许 1 条
			ConditionMaxDepth: 10,
		},
	}
	cond := mkLeafCond("hp", ">", "50", "")
	transitions := []model.FsmTransition{
		{From: "A", To: "B", Priority: 0, Condition: cond},
		{From: "B", To: "A", Priority: 0, Condition: cond}, // 超限
	}
	err := s.validateConfig("A", mkStates("A", "B"), transitions)
	if err == nil || err.Code != errcode.ErrFsmConfigTransitionInvalid {
		t.Fatalf("want ErrFsmConfigTransitionInvalid, got %v", err)
	}
}

// --- collectConditionKeys + ExtractBBKeys + ExtractBBKeysFromConfigJSON ---

func TestCollectConditionKeys(t *testing.T) {
	t.Run("空条件无 key", func(t *testing.T) {
		keys := make(map[string]bool)
		collectConditionKeys(&model.FsmCondition{}, keys)
		if len(keys) != 0 {
			t.Fatalf("expected empty, got %v", keys)
		}
	})

	t.Run("叶节点只有 key", func(t *testing.T) {
		keys := make(map[string]bool)
		c := mkLeafCond("hp", ">", "50", "")
		collectConditionKeys(&c, keys)
		if !keys["hp"] || len(keys) != 1 {
			t.Fatalf("expected {hp}, got %v", keys)
		}
	})

	t.Run("叶节点 key+ref_key 两个都采集", func(t *testing.T) {
		keys := make(map[string]bool)
		c := mkLeafCond("hp", ">", "", "threshold")
		collectConditionKeys(&c, keys)
		if !keys["hp"] || !keys["threshold"] || len(keys) != 2 {
			t.Fatalf("expected {hp,threshold}, got %v", keys)
		}
	})

	t.Run("组合节点 and+or 递归 + 跨层去重", func(t *testing.T) {
		// hp 在 and 第 1 项，又在 or 第 1 项 → 只记一次
		cond := model.FsmCondition{
			And: []model.FsmCondition{
				mkLeafCond("hp", ">", "50", ""),
				mkLeafCond("mp", "<", "30", ""),
			},
			Or: []model.FsmCondition{
				mkLeafCond("hp", "<", "10", ""), // 去重
				mkLeafCond("enemy_dist", "<", "5", "ref_range"),
			},
		}
		keys := make(map[string]bool)
		collectConditionKeys(&cond, keys)
		want := map[string]bool{"hp": true, "mp": true, "enemy_dist": true, "ref_range": true}
		if len(keys) != len(want) {
			t.Fatalf("want=%v got=%v", want, keys)
		}
		for k := range want {
			if !keys[k] {
				t.Errorf("missing %q", k)
			}
		}
	})
}

func TestExtractBBKeys(t *testing.T) {
	transitions := []model.FsmTransition{
		{From: "A", To: "B", Priority: 0,
			Condition: mkLeafCond("hp", ">", "50", "")},
		{From: "B", To: "A", Priority: 0,
			Condition: mkLeafCond("hp", "<", "10", "")}, // hp 去重
		{From: "A", To: "C", Priority: 0,
			Condition: mkLeafCond("mp", ">", "20", "ref_mp_min")},
	}
	keys := ExtractBBKeys(transitions)
	want := map[string]bool{"hp": true, "mp": true, "ref_mp_min": true}
	if len(keys) != len(want) {
		t.Fatalf("want=%v got=%v", want, keys)
	}
	for k := range want {
		if !keys[k] {
			t.Errorf("missing %q", k)
		}
	}
}

func TestExtractBBKeysFromConfigJSON(t *testing.T) {
	t.Run("合法 config 提取去重", func(t *testing.T) {
		cfg := json.RawMessage(`{
			"initial_state":"A",
			"states":[{"name":"A"},{"name":"B"}],
			"transitions":[
				{"from":"A","to":"B","priority":0,"condition":{"key":"hp","op":">","value":50}},
				{"from":"B","to":"A","priority":0,"condition":{"key":"hp","op":"<","value":10}}
			]
		}`)
		keys := ExtractBBKeysFromConfigJSON(cfg)
		if len(keys) != 1 || !keys["hp"] {
			t.Fatalf("want {hp}, got %v", keys)
		}
	})

	t.Run("非法 JSON 返空 map", func(t *testing.T) {
		keys := ExtractBBKeysFromConfigJSON(json.RawMessage(`not-json`))
		if keys == nil {
			t.Fatal("want non-nil empty map")
		}
		if len(keys) != 0 {
			t.Errorf("want empty, got %v", keys)
		}
	})
}
