package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
)

// ============================================================
// validateSpawnTableImpl + 两个拆出的纯辅助单测：
// 注入 fakeNpcNameLookup 避免 NpcService/DB。
// ============================================================

type fakeNpcNameLookup struct {
	statusMap map[string]bool
	err       error
	gotNames  []string
}

func (f *fakeNpcNameLookup) LookupByNames(_ context.Context, names []string) (map[string]bool, error) {
	f.gotNames = append([]string(nil), names...)
	if f.err != nil {
		return nil, f.err
	}
	return f.statusMap, nil
}

// ---------- validateSpawnTableImpl 主路径 ----------

func TestValidateSpawnTableImpl_EmptyRawRejected(t *testing.T) {
	lookup := &fakeNpcNameLookup{}
	err := validateSpawnTableImpl(context.Background(), lookup, nil)
	if err == nil {
		t.Fatal("want err, got nil")
	}
	var codeErr *errcode.Error
	if !errors.As(err, &codeErr) || codeErr.Code != errcode.ErrRegionSpawnEntryInvalid {
		t.Errorf("want ErrRegionSpawnEntryInvalid, got %v", err)
	}
	if len(lookup.gotNames) != 0 {
		t.Errorf("空 raw 不应调用 lookup, got %v", lookup.gotNames)
	}
}

func TestValidateSpawnTableImpl_InvalidJSONRejected(t *testing.T) {
	lookup := &fakeNpcNameLookup{}
	err := validateSpawnTableImpl(context.Background(), lookup, json.RawMessage(`{not-json}`))
	if err == nil {
		t.Fatal("want err, got nil")
	}
	if !strings.Contains(err.Error(), "必须是合法 JSON 数组") {
		t.Errorf("err 应含 '必须是合法 JSON 数组', got %q", err.Error())
	}
	if len(lookup.gotNames) != 0 {
		t.Errorf("非法 JSON 不应调用 lookup, got %v", lookup.gotNames)
	}
}

func TestValidateSpawnTableImpl_EmptyArrayAllowed(t *testing.T) {
	lookup := &fakeNpcNameLookup{}
	err := validateSpawnTableImpl(context.Background(), lookup, json.RawMessage(`[]`))
	if err != nil {
		t.Errorf("want nil, got %v", err)
	}
	if len(lookup.gotNames) != 0 {
		t.Errorf("空数组不应调用 lookup, got %v", lookup.gotNames)
	}
}

func TestValidateSpawnTableImpl_LookupErrorWrapped(t *testing.T) {
	lookup := &fakeNpcNameLookup{err: errors.New("db down")}
	raw := json.RawMessage(`[{"template_ref":"a","count":1,"spawn_points":[{"x":0,"z":0}]}]`)
	err := validateSpawnTableImpl(context.Background(), lookup, raw)
	if err == nil {
		t.Fatal("want err, got nil")
	}
	// 非 *errcode.Error
	var codeErr *errcode.Error
	if errors.As(err, &codeErr) {
		t.Errorf("不应是 *errcode.Error, got %v", codeErr)
	}
	if !strings.Contains(err.Error(), "lookup npcs by names") {
		t.Errorf("err 应含 'lookup npcs by names', got %q", err.Error())
	}
}

func TestValidateSpawnTableImpl_HappyOneEntry(t *testing.T) {
	lookup := &fakeNpcNameLookup{statusMap: map[string]bool{"villager_merchant": true}}
	raw := json.RawMessage(`[
		{"template_ref":"villager_merchant","count":2,
		 "spawn_points":[{"x":0,"z":0},{"x":1,"z":1}],
		 "wander_radius":5.0,"respawn_seconds":30}
	]`)
	err := validateSpawnTableImpl(context.Background(), lookup, raw)
	if err != nil {
		t.Errorf("want nil, got %v", err)
	}
	if len(lookup.gotNames) != 1 || lookup.gotNames[0] != "villager_merchant" {
		t.Errorf("want names=[villager_merchant], got %v", lookup.gotNames)
	}
}

func TestValidateSpawnTableImpl_HappyMultipleEntriesDedup(t *testing.T) {
	// 3 条 entry, 2 个不同 template_ref，预期去重后发 2 个
	lookup := &fakeNpcNameLookup{
		statusMap: map[string]bool{"a": true, "b": true},
	}
	raw := json.RawMessage(`[
		{"template_ref":"a","count":1,"spawn_points":[{"x":0,"z":0}]},
		{"template_ref":"b","count":1,"spawn_points":[{"x":1,"z":1}]},
		{"template_ref":"a","count":2,"spawn_points":[{"x":2,"z":2},{"x":3,"z":3}]}
	]`)
	err := validateSpawnTableImpl(context.Background(), lookup, raw)
	if err != nil {
		t.Errorf("want nil, got %v", err)
	}
	if len(lookup.gotNames) != 2 {
		t.Errorf("去重后应为 2 个 names, got %v", lookup.gotNames)
	}
	// 保序：首次出现顺序
	if lookup.gotNames[0] != "a" || lookup.gotNames[1] != "b" {
		t.Errorf("want names=[a,b] 保序, got %v", lookup.gotNames)
	}
}

func TestValidateSpawnTableImpl_MissingTemplateError(t *testing.T) {
	// a 存在启用，ghost 不存在 → 先报"不存在"
	lookup := &fakeNpcNameLookup{statusMap: map[string]bool{"a": true}}
	raw := json.RawMessage(`[
		{"template_ref":"a","count":1,"spawn_points":[{"x":0,"z":0}]},
		{"template_ref":"ghost","count":1,"spawn_points":[{"x":1,"z":1}]}
	]`)
	err := validateSpawnTableImpl(context.Background(), lookup, raw)
	if err == nil {
		t.Fatal("want err, got nil")
	}
	var codeErr *errcode.Error
	if !errors.As(err, &codeErr) || codeErr.Code != errcode.ErrRegionTemplateRefNotFound {
		t.Errorf("want ErrRegionTemplateRefNotFound, got %v", err)
	}
	if !strings.Contains(err.Error(), "ghost") {
		t.Errorf("err 应提及 ghost, got %q", err.Error())
	}
}

func TestValidateSpawnTableImpl_DisabledTemplateError(t *testing.T) {
	// 两个都存在但其中一个未启用 → 报"未启用"
	lookup := &fakeNpcNameLookup{statusMap: map[string]bool{"a": true, "b": false}}
	raw := json.RawMessage(`[
		{"template_ref":"a","count":1,"spawn_points":[{"x":0,"z":0}]},
		{"template_ref":"b","count":1,"spawn_points":[{"x":1,"z":1}]}
	]`)
	err := validateSpawnTableImpl(context.Background(), lookup, raw)
	if err == nil {
		t.Fatal("want err, got nil")
	}
	var codeErr *errcode.Error
	if !errors.As(err, &codeErr) || codeErr.Code != errcode.ErrRegionTemplateRefDisabled {
		t.Errorf("want ErrRegionTemplateRefDisabled, got %v", err)
	}
	if !strings.Contains(err.Error(), "b") {
		t.Errorf("err 应提及 b, got %q", err.Error())
	}
}

func TestValidateSpawnTableImpl_MissingTakesPrecedenceOverDisabled(t *testing.T) {
	// 同时有 missing 和 disabled → 先报 missing（按"结构错 → 不存在 → 未启用"优先级）
	lookup := &fakeNpcNameLookup{statusMap: map[string]bool{"disabled_npc": false}}
	raw := json.RawMessage(`[
		{"template_ref":"disabled_npc","count":1,"spawn_points":[{"x":0,"z":0}]},
		{"template_ref":"missing_npc","count":1,"spawn_points":[{"x":1,"z":1}]}
	]`)
	err := validateSpawnTableImpl(context.Background(), lookup, raw)
	if err == nil {
		t.Fatal("want err, got nil")
	}
	var codeErr *errcode.Error
	if !errors.As(err, &codeErr) || codeErr.Code != errcode.ErrRegionTemplateRefNotFound {
		t.Errorf("want ErrRegionTemplateRefNotFound 优先, got %v", err)
	}
}

// ---------- validateSpawnEntriesAndCollectNames 结构校验 6 分支 ----------

func TestValidateSpawnEntriesAndCollectNames_EmptyTemplateRef(t *testing.T) {
	entries := []model.SpawnEntry{{TemplateRef: "", Count: 1, SpawnPoints: []model.SpawnPoint{{}}}}
	_, err := validateSpawnEntriesAndCollectNames(entries)
	if err == nil {
		t.Fatal("want err, got nil")
	}
	if !strings.Contains(err.Error(), "template_ref 不能为空") {
		t.Errorf("err 应含 'template_ref 不能为空', got %q", err.Error())
	}
}

func TestValidateSpawnEntriesAndCollectNames_CountLessThan1(t *testing.T) {
	entries := []model.SpawnEntry{{TemplateRef: "a", Count: 0, SpawnPoints: []model.SpawnPoint{{}}}}
	_, err := validateSpawnEntriesAndCollectNames(entries)
	if err == nil {
		t.Fatal("want err, got nil")
	}
	if !strings.Contains(err.Error(), "count 必须 >= 1") {
		t.Errorf("err 应含 'count 必须 >= 1', got %q", err.Error())
	}

	// 负数 count 同样
	entries = []model.SpawnEntry{{TemplateRef: "a", Count: -1, SpawnPoints: []model.SpawnPoint{{}}}}
	_, err = validateSpawnEntriesAndCollectNames(entries)
	if err == nil {
		t.Fatal("负数 count: want err, got nil")
	}
}

func TestValidateSpawnEntriesAndCollectNames_SpawnPointsFewerThanCount(t *testing.T) {
	entries := []model.SpawnEntry{{
		TemplateRef: "a", Count: 3,
		SpawnPoints: []model.SpawnPoint{{X: 0, Z: 0}, {X: 1, Z: 1}},
	}}
	_, err := validateSpawnEntriesAndCollectNames(entries)
	if err == nil {
		t.Fatal("want err, got nil")
	}
	if !strings.Contains(err.Error(), "刷怪点数") {
		t.Errorf("err 应含 '刷怪点数', got %q", err.Error())
	}
}

func TestValidateSpawnEntriesAndCollectNames_NegativeWanderRadius(t *testing.T) {
	entries := []model.SpawnEntry{{
		TemplateRef: "a", Count: 1,
		SpawnPoints:  []model.SpawnPoint{{}},
		WanderRadius: -0.1,
	}}
	_, err := validateSpawnEntriesAndCollectNames(entries)
	if err == nil {
		t.Fatal("want err, got nil")
	}
	if !strings.Contains(err.Error(), "wander_radius 不能为负") {
		t.Errorf("err 应含 'wander_radius 不能为负', got %q", err.Error())
	}
}

func TestValidateSpawnEntriesAndCollectNames_NegativeRespawnSeconds(t *testing.T) {
	entries := []model.SpawnEntry{{
		TemplateRef: "a", Count: 1,
		SpawnPoints:    []model.SpawnPoint{{}},
		RespawnSeconds: -1,
	}}
	_, err := validateSpawnEntriesAndCollectNames(entries)
	if err == nil {
		t.Fatal("want err, got nil")
	}
	if !strings.Contains(err.Error(), "respawn_seconds 不能为负") {
		t.Errorf("err 应含 'respawn_seconds 不能为负', got %q", err.Error())
	}
}

func TestValidateSpawnEntriesAndCollectNames_HappyDedupeAndOrder(t *testing.T) {
	entries := []model.SpawnEntry{
		{TemplateRef: "b", Count: 1, SpawnPoints: []model.SpawnPoint{{}}},
		{TemplateRef: "a", Count: 1, SpawnPoints: []model.SpawnPoint{{}}},
		{TemplateRef: "b", Count: 1, SpawnPoints: []model.SpawnPoint{{}}},
		{TemplateRef: "c", Count: 1, SpawnPoints: []model.SpawnPoint{{}}},
	}
	names, err := validateSpawnEntriesAndCollectNames(entries)
	if err != nil {
		t.Fatalf("want nil, got %v", err)
	}
	want := []string{"b", "a", "c"}
	if len(names) != len(want) {
		t.Fatalf("len: want %d, got %d (%v)", len(want), len(names), names)
	}
	for i, v := range want {
		if names[i] != v {
			t.Errorf("[%d]: want %q, got %q", i, v, names[i])
		}
	}
}

func TestValidateSpawnEntriesAndCollectNames_ErrorIncludesIndex(t *testing.T) {
	// 第 2 条才出错 → err 应含 [1]
	entries := []model.SpawnEntry{
		{TemplateRef: "a", Count: 1, SpawnPoints: []model.SpawnPoint{{}}},
		{TemplateRef: "", Count: 1, SpawnPoints: []model.SpawnPoint{{}}},
	}
	_, err := validateSpawnEntriesAndCollectNames(entries)
	if err == nil {
		t.Fatal("want err, got nil")
	}
	if !strings.Contains(err.Error(), "[1]") {
		t.Errorf("err 应含 '[1]' 索引, got %q", err.Error())
	}
}

// ---------- classifySpawnTableRefs 3 分支 ----------

func TestClassifySpawnTableRefs_AllPresent(t *testing.T) {
	names := []string{"a", "b"}
	status := map[string]bool{"a": true, "b": true}
	if err := classifySpawnTableRefs(names, status); err != nil {
		t.Errorf("want nil, got %v", err)
	}
}

func TestClassifySpawnTableRefs_CollectsAllMissing(t *testing.T) {
	// 两个都缺失 → err 应含两个名字
	names := []string{"x", "y"}
	status := map[string]bool{}
	err := classifySpawnTableRefs(names, status)
	if err == nil {
		t.Fatal("want err, got nil")
	}
	if !strings.Contains(err.Error(), "x") || !strings.Contains(err.Error(), "y") {
		t.Errorf("err 应含 x 和 y, got %q", err.Error())
	}
}

func TestClassifySpawnTableRefs_CollectsAllDisabled(t *testing.T) {
	names := []string{"a", "b"}
	status := map[string]bool{"a": false, "b": false}
	err := classifySpawnTableRefs(names, status)
	if err == nil {
		t.Fatal("want err, got nil")
	}
	var codeErr *errcode.Error
	if !errors.As(err, &codeErr) || codeErr.Code != errcode.ErrRegionTemplateRefDisabled {
		t.Errorf("want ErrRegionTemplateRefDisabled, got %v", err)
	}
}

func TestClassifySpawnTableRefs_EmptyNamesPass(t *testing.T) {
	// 上游保证空数组直接返回；防御性检查：空 names 应放行
	if err := classifySpawnTableRefs(nil, nil); err != nil {
		t.Errorf("nil names: want nil, got %v", err)
	}
	if err := classifySpawnTableRefs([]string{}, map[string]bool{}); err != nil {
		t.Errorf("empty names: want nil, got %v", err)
	}
}
