package mysql

import (
	"encoding/json"
	"sort"
	"testing"
)

// ============================================================
// BT config 纯提取器测试：walkNodes / extractBBKeys /
// ExtractBBKeysFromConfig / extractTypeNamesFromConfig。
//
// 价值：这些函数纯（无 DB/tx 依赖）、递归、驱动 BB Key 运行时注册表和
// bt_node_type_refs 同步。bb-key-runtime-registry T18 smoke 期曾在
// "params 子对象嵌套读取" 处返工，补测作回归网。
// ============================================================

// --- walkNodes ---

func TestWalkNodes(t *testing.T) {
	root := map[string]any{
		"type": "sequence",
		"children": []any{
			map[string]any{
				"type":  "inverter",
				"child": map[string]any{"type": "leaf_a"},
			},
			map[string]any{"type": "leaf_b"},
		},
	}

	var visited []string
	walkNodes(root, func(n map[string]any) {
		if t, ok := n["type"].(string); ok {
			visited = append(visited, t)
		}
	})

	sort.Strings(visited)
	want := []string{"inverter", "leaf_a", "leaf_b", "sequence"}
	if len(visited) != len(want) {
		t.Fatalf("len(visited)=%d want=%d: %v", len(visited), len(want), visited)
	}
	for i := range want {
		if visited[i] != want[i] {
			t.Fatalf("visited[%d]=%q want=%q", i, visited[i], want[i])
		}
	}
}

func TestWalkNodes_LeafOnly(t *testing.T) {
	root := map[string]any{"type": "leaf_x"}
	count := 0
	walkNodes(root, func(map[string]any) { count++ })
	if count != 1 {
		t.Fatalf("leaf-only root should visit 1 node, got %d", count)
	}
}

func TestWalkNodes_MalformedChildrenIgnored(t *testing.T) {
	// children 非数组 / 元素非对象 / child 非对象 → 不 panic，不遍历非法项
	root := map[string]any{
		"type":     "sequence",
		"children": "not-array",
	}
	count := 0
	walkNodes(root, func(map[string]any) { count++ })
	if count != 1 {
		t.Fatalf("malformed children should not recurse, got %d", count)
	}

	root2 := map[string]any{
		"type":     "sequence",
		"children": []any{"not-a-map", float64(1), nil},
	}
	count = 0
	walkNodes(root2, func(map[string]any) { count++ })
	if count != 1 {
		t.Fatalf("non-map children elements should be skipped, got %d", count)
	}

	root3 := map[string]any{"type": "inverter", "child": "not-a-map"}
	count = 0
	walkNodes(root3, func(map[string]any) { count++ })
	if count != 1 {
		t.Fatalf("non-map child should be skipped, got %d", count)
	}
}

// --- extractBBKeys ---

func TestExtractBBKeys(t *testing.T) {
	// 模拟 bt_node_types.param_schema 里 type=bb_key 的参数名预加载结果
	nodeParamTypes := map[string][]string{
		"check_bb_float": {"key"},
		"move_to":        {"target_key_x", "target_key_z"},
		"stub_action":    nil, // 无 bb_key 参数
	}

	root := map[string]any{
		"type": "sequence",
		"children": []any{
			map[string]any{
				"type":   "check_bb_float",
				"params": map[string]any{"key": "hp", "op": ">", "value": float64(50)},
			},
			map[string]any{
				"type":   "move_to",
				"params": map[string]any{"target_key_x": "x", "target_key_z": "z"},
			},
			map[string]any{
				"type":   "stub_action",
				"params": map[string]any{"name": "idle"},
			},
		},
	}

	got := extractBBKeys(root, nodeParamTypes)
	sort.Strings(got)
	want := []string{"hp", "x", "z"}
	if len(got) != len(want) {
		t.Fatalf("len(got)=%d want=%d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d]=%q want=%q", i, got[i], want[i])
		}
	}
}

func TestExtractBBKeys_ParamsMustBeNested(t *testing.T) {
	// 回归：T18 smoke 修过一次 — 参数必须嵌在 params 子对象里；
	// 顶层裸字段 key 不应被采集（旧格式已被 validateBtNode 拦截）。
	nodeParamTypes := map[string][]string{"check_bb_float": {"key"}}

	root := map[string]any{
		"type": "check_bb_float",
		"key":  "bare_key_should_not_collect", // 顶层裸字段
		"params": map[string]any{
			"key": "nested_key",
		},
	}
	got := extractBBKeys(root, nodeParamTypes)
	if len(got) != 1 || got[0] != "nested_key" {
		t.Fatalf("expected [nested_key], got %v", got)
	}
}

func TestExtractBBKeys_SkipUnknownTypeAndEmpty(t *testing.T) {
	nodeParamTypes := map[string][]string{"check_bb_float": {"key"}}

	root := map[string]any{
		"type": "sequence",
		"children": []any{
			// 类型不在 nodeParamTypes → 整个节点跳过
			map[string]any{"type": "unknown_type", "params": map[string]any{"key": "x"}},
			// type 非字符串 → 跳过
			map[string]any{"type": float64(123)},
			// 空 key 值 → 丢弃
			map[string]any{"type": "check_bb_float", "params": map[string]any{"key": ""}},
			// key 非 string → 丢弃
			map[string]any{"type": "check_bb_float", "params": map[string]any{"key": float64(1)}},
			// 合法
			map[string]any{"type": "check_bb_float", "params": map[string]any{"key": "ok"}},
		},
	}
	got := extractBBKeys(root, nodeParamTypes)
	if len(got) != 1 || got[0] != "ok" {
		t.Fatalf("expected [ok], got %v", got)
	}
}

func TestExtractBBKeys_ParamsMissingOrWrongType(t *testing.T) {
	nodeParamTypes := map[string][]string{"check_bb_float": {"key"}}

	cases := []map[string]any{
		{"type": "check_bb_float"}, // 无 params
		{"type": "check_bb_float", "params": "not-a-map"},
		{"type": "check_bb_float", "params": nil},
		{"type": "check_bb_float", "params": []any{}},
	}
	for i, n := range cases {
		got := extractBBKeys(n, nodeParamTypes)
		if len(got) != 0 {
			t.Fatalf("case %d: expected empty, got %v", i, got)
		}
	}
}

// --- ExtractBBKeysFromConfig ---

func TestExtractBBKeysFromConfig_DedupAndJSON(t *testing.T) {
	nodeParamTypes := map[string][]string{"check_bb_float": {"key"}}

	// 同一 key 在两处出现应去重；返回 map 结构
	cfg := json.RawMessage(`{
		"type": "sequence",
		"children": [
			{"type": "check_bb_float", "params": {"key": "hp"}},
			{"type": "check_bb_float", "params": {"key": "hp"}},
			{"type": "check_bb_float", "params": {"key": "mp"}}
		]
	}`)

	m, err := ExtractBBKeysFromConfig(cfg, nodeParamTypes)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(m) != 2 || !m["hp"] || !m["mp"] {
		t.Fatalf("expected {hp:true, mp:true}, got %v", m)
	}
}

func TestExtractBBKeysFromConfig_InvalidJSON(t *testing.T) {
	_, err := ExtractBBKeysFromConfig(json.RawMessage(`{not-valid}`), nil)
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

// --- extractTypeNamesFromConfig ---

func TestExtractTypeNamesFromConfig(t *testing.T) {
	cfg := json.RawMessage(`{
		"type": "sequence",
		"children": [
			{"type": "inverter", "child": {"type": "stub_action"}},
			{"type": "stub_action"}
		]
	}`)
	got, err := extractTypeNamesFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	sort.Strings(got)
	want := []string{"inverter", "sequence", "stub_action"} // stub_action 去重
	if len(got) != len(want) {
		t.Fatalf("len(got)=%d want=%d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d]=%q want=%q", i, got[i], want[i])
		}
	}
}

func TestExtractTypeNamesFromConfig_InvalidJSON(t *testing.T) {
	_, err := extractTypeNamesFromConfig(json.RawMessage(`[not,valid`))
	if err == nil {
		t.Fatal("expected unmarshal error")
	}
}

func TestExtractTypeNamesFromConfig_EmptyOrMissingType(t *testing.T) {
	// type 空串或非字符串应被忽略（seen[""] 不加入）
	cfg := json.RawMessage(`{"type":"","children":[{"type":123},{"type":"real"}]}`)
	got, err := extractTypeNamesFromConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(got) != 1 || got[0] != "real" {
		t.Fatalf("expected [real], got %v", got)
	}
}
