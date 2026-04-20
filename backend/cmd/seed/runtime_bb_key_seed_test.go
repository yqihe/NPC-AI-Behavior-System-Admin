package main

import (
	"regexp"
	"testing"
)

// ============================================================
// runtimeBbKeyFixtures 契约测试（T17）
//
// 锁住 design §0 的 "31 条 / 4 type / 11 group" 分布契约与 Server keys.go 对齐。
// 任何未来改动 fixture 导致分布偏移，这里会立即 fire。
//
// 不走 DB —— 纯数据结构断言，对齐项目既有 pure function 测试风格。
// ============================================================

// 与 service/runtime_bb_key.go runtimeBbKeyNameRE 保持一致（跨包不 import private，复制声明）
var seedNameRE = regexp.MustCompile(`^[a-z][a-z0-9_]{1,63}$`)

func TestRuntimeBbKeyFixtures_Count(t *testing.T) {
	if got := len(runtimeBbKeyFixtures); got != 31 {
		t.Fatalf("fixture 总数应为 31（与 Server keys.go 对齐），实际 %d", got)
	}
}

func TestRuntimeBbKeyFixtures_NameUniqueAndValid(t *testing.T) {
	seen := make(map[string]bool, len(runtimeBbKeyFixtures))
	for _, s := range runtimeBbKeyFixtures {
		if !seedNameRE.MatchString(s.Name) {
			t.Errorf("name %q 不符合 regex ^[a-z][a-z0-9_]{1,63}$", s.Name)
		}
		if seen[s.Name] {
			t.Errorf("name %q 重复", s.Name)
		}
		seen[s.Name] = true
	}
}

func TestRuntimeBbKeyFixtures_TypeDistribution(t *testing.T) {
	// design §0 锁定：13 float + 4 integer + 12 string + 2 bool
	wantDist := map[string]int{
		"float":   13,
		"integer": 4,
		"string":  12,
		"bool":    2,
	}

	got := make(map[string]int, 4)
	for _, s := range runtimeBbKeyFixtures {
		got[s.Type]++
	}

	for typ, want := range wantDist {
		if got[typ] != want {
			t.Errorf("type %q 分布应为 %d，实际 %d", typ, want, got[typ])
		}
	}
	// 不应出现 4 枚举外的 type
	for typ := range got {
		if _, ok := wantDist[typ]; !ok {
			t.Errorf("意外 type %q（应仅限 integer/float/string/bool）", typ)
		}
	}
}

func TestRuntimeBbKeyFixtures_GroupDistribution(t *testing.T) {
	// design §0 锁定：11 组 + 每组条目数（与 Server keys.go 分节注释对齐）
	wantDist := map[string]int{
		"threat":   3,
		"event":    2,
		"fsm":      1,
		"npc":      3,
		"action":   3,
		"need":     2,
		"emotion":  2,
		"memory":   2,
		"social":   6,
		"decision": 4,
		"move":     3,
	}

	got := make(map[string]int, 11)
	for _, s := range runtimeBbKeyFixtures {
		got[s.GroupName]++
	}

	if len(got) != 11 {
		t.Errorf("分组数量应为 11，实际 %d（组：%v）", len(got), got)
	}
	for g, want := range wantDist {
		if got[g] != want {
			t.Errorf("group %q 条目数应为 %d，实际 %d", g, want, got[g])
		}
	}
	for g := range got {
		if _, ok := wantDist[g]; !ok {
			t.Errorf("意外 group %q（应仅限 design §0 的 11 组）", g)
		}
	}
}

func TestRuntimeBbKeyFixtures_LabelAndDescriptionNonEmpty(t *testing.T) {
	for _, s := range runtimeBbKeyFixtures {
		if s.Label == "" {
			t.Errorf("name %q 的 Label 不能为空", s.Name)
		}
		if s.Description == "" {
			t.Errorf("name %q 的 Description 不能为空", s.Name)
		}
	}
}
