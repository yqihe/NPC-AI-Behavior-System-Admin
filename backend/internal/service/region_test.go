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
// RegionService 纯方法测试（对齐 npc_service_test.go T8 先例）
//
// 全部纯方法：validateSpawnTable 结构错分支、CollectExportRefs、
// BuildExportDanglingError、AssembleExportItems。
// 零外部依赖，所以全用 zero-value *RegionService，不引入 mock。
// 引用校验段（npcService.LookupByNames）需要 store/cache 的用例属
// integration 范畴，按 design §8.1 延后。
// ============================================================

// mkRegion 构造测试 region 行（SpawnTable 传 JSON 字符串）
func mkRegion(regionID, displayName, regionType, spawnTableJSON string) model.Region {
	if spawnTableJSON == "" {
		spawnTableJSON = "[]"
	}
	return model.Region{
		RegionID:    regionID,
		DisplayName: displayName,
		RegionType:  regionType,
		SpawnTable:  json.RawMessage(spawnTableJSON),
		Enabled:     true,
		Version:     1,
	}
}

// assertErrCode 断言返回的 *errcode.Error 匹配期望 code
func assertErrCode(t *testing.T, err error, want int, hint string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s: expected error code %d, got nil", hint, want)
	}
	var ecErr *errcode.Error
	if !errors.As(err, &ecErr) {
		t.Fatalf("%s: expected *errcode.Error, got %T: %v", hint, err, err)
	}
	if ecErr.Code != want {
		t.Fatalf("%s: code want %d got %d (msg=%q)", hint, want, ecErr.Code, ecErr.Message)
	}
}

// ============================================================
// validateSpawnTable（结构校验分支）
// ============================================================

// 空数组合法：Unmarshal 成功 + 零条目 → nil（无引用校验）
func TestRegion_ValidateSpawnTable_Empty(t *testing.T) {
	s := &RegionService{}
	if err := s.validateSpawnTable(context.Background(), json.RawMessage(`[]`)); err != nil {
		t.Fatalf("empty spawn_table 应合法: %v", err)
	}
}

// count=0 非法（count 必须 >=1）
func TestRegion_ValidateSpawnTable_NegativeCount(t *testing.T) {
	s := &RegionService{}
	raw := json.RawMessage(`[{"template_ref":"a","count":0,"spawn_points":[{"x":1,"z":2}]}]`)
	err := s.validateSpawnTable(context.Background(), raw)
	assertErrCode(t, err, errcode.ErrRegionSpawnEntryInvalid, "count=0")
}

// spawn_points 数 (1) < count (2)
func TestRegion_ValidateSpawnTable_PointsLessThanCount(t *testing.T) {
	s := &RegionService{}
	raw := json.RawMessage(`[{"template_ref":"a","count":2,"spawn_points":[{"x":1,"z":2}]}]`)
	err := s.validateSpawnTable(context.Background(), raw)
	assertErrCode(t, err, errcode.ErrRegionSpawnEntryInvalid, "points<count")
}

// 非数组 JSON（对象）→ unmarshal 失败
func TestRegion_ValidateSpawnTable_BadJSON(t *testing.T) {
	s := &RegionService{}
	raw := json.RawMessage(`{"not":"array"}`)
	err := s.validateSpawnTable(context.Background(), raw)
	assertErrCode(t, err, errcode.ErrRegionSpawnEntryInvalid, "bad json")
	if !strings.Contains(err.Error(), "spawn_table") {
		t.Fatalf("错误消息应含 'spawn_table'，实际: %q", err.Error())
	}
}

// ============================================================
// CollectExportRefs
// ============================================================

func TestRegion_CollectExportRefs_Empty(t *testing.T) {
	s := &RegionService{}
	refs, err := s.CollectExportRefs(nil)
	if err != nil {
		t.Fatalf("nil rows: %v", err)
	}
	if refs == nil || refs.TemplateIndex == nil {
		t.Fatalf("expected non-nil map, got %#v", refs)
	}
	if len(refs.TemplateIndex) != 0 {
		t.Fatalf("expected empty index, got %v", refs.TemplateIndex)
	}
}

// 2 region 各引 1 template → 反查索引聚合正确；空 spawn_table 不入索引
func TestRegion_CollectExportRefs_Multi(t *testing.T) {
	s := &RegionService{}
	rows := []model.Region{
		mkRegion("village_outskirts", "村外", "wilderness",
			`[{"template_ref":"villager_guard","count":1,"spawn_points":[{"x":1,"z":2}]}]`),
		mkRegion("town_square", "镇广场", "town",
			`[{"template_ref":"villager_guard","count":1,"spawn_points":[{"x":3,"z":4}]}]`),
		mkRegion("empty_spot", "空地", "wilderness", `[]`), // 空 spawn_table 不入索引
	}
	refs, err := s.CollectExportRefs(rows)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	got := refs.TemplateIndex["villager_guard"]
	if len(got) != 2 {
		t.Fatalf("TemplateIndex[villager_guard]: want 2 regions, got %v", got)
	}
	// 顺序对应 rows 遍历顺序
	if got[0] != "village_outskirts" || got[1] != "town_square" {
		t.Fatalf("TemplateIndex 顺序不对: %v", got)
	}
	if _, ok := refs.TemplateIndex[""]; ok {
		t.Fatalf("空 template_ref 不应入索引")
	}
}

// ============================================================
// BuildExportDanglingError
// ============================================================

func TestRegion_BuildExportDanglingError_AllValid(t *testing.T) {
	s := &RegionService{}
	refs := &RegionExportRefs{
		TemplateIndex: map[string][]string{"villager_guard": {"village_outskirts"}},
	}
	got := s.BuildExportDanglingError(refs, nil)
	if got != nil {
		t.Fatalf("notOK 空时应返 nil，实际 %#v", got)
	}
}

func TestRegion_BuildExportDanglingError_SomeMissing(t *testing.T) {
	s := &RegionService{}
	refs := &RegionExportRefs{
		TemplateIndex: map[string][]string{
			"villager_guard": {"village_outskirts", "town_square"},
			"merchant":       {"town_square"},
		},
	}
	got := s.BuildExportDanglingError(refs, []string{"villager_guard"})
	if got == nil || len(got.Details) != 2 {
		t.Fatalf("want 2 details, got %#v", got)
	}
	for i, d := range got.Details {
		if d.RefType != model.ExportRefTypeNpcTemplate {
			t.Fatalf("Details[%d].RefType: want npc_template_ref, got %q", i, d.RefType)
		}
		if d.RefValue != "villager_guard" {
			t.Fatalf("Details[%d].RefValue: want villager_guard, got %q", i, d.RefValue)
		}
		if d.Reason != model.ExportRefReasonMissingOrDisabled {
			t.Fatalf("Details[%d].Reason: want %q, got %q",
				i, model.ExportRefReasonMissingOrDisabled, d.Reason)
		}
		// NPCName 字段此处承载 region_id（T8 约定）
		if d.NPCName != "village_outskirts" && d.NPCName != "town_square" {
			t.Fatalf("Details[%d].NPCName（此处承载 region_id）意外值: %q", i, d.NPCName)
		}
	}
}

// ============================================================
// AssembleExportItems
// ============================================================

func TestRegion_AssembleExportItems_Empty(t *testing.T) {
	s := &RegionService{}
	got := s.AssembleExportItems(nil)
	if got == nil {
		t.Fatalf("expected non-nil empty slice (避免 JSON null)")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %d items", len(got))
	}
}

func TestRegion_AssembleExportItems_OneRow(t *testing.T) {
	s := &RegionService{}
	rows := []model.Region{
		mkRegion("village_outskirts", "村外", "wilderness",
			`[{"template_ref":"villager_guard","count":1,"spawn_points":[{"x":1,"z":2}]}]`),
	}
	items := s.AssembleExportItems(rows)
	if len(items) != 1 {
		t.Fatalf("want 1 item, got %d", len(items))
	}
	// envelope.Name 应承载 region_id（HTTPSource 路由键）
	if items[0].Name != "village_outskirts" {
		t.Fatalf("envelope.Name: want village_outskirts, got %q", items[0].Name)
	}
	// config.Name 应承载 display_name（Server Zone.Name 显示名）
	if items[0].Config.Name != "村外" {
		t.Fatalf("config.Name: want 村外, got %q", items[0].Config.Name)
	}
	if items[0].Config.RegionID != "village_outskirts" {
		t.Fatalf("config.RegionID: want village_outskirts, got %q", items[0].Config.RegionID)
	}
	if items[0].Config.RegionType != "wilderness" {
		t.Fatalf("config.RegionType: want wilderness, got %q", items[0].Config.RegionType)
	}
	// spawn_table 原样透传（json.RawMessage 不经 Go struct 中转）
	if !strings.Contains(string(items[0].Config.SpawnTable), "villager_guard") {
		t.Fatalf("spawn_table 应原样透传，实际 %s", items[0].Config.SpawnTable)
	}
}

// ============================================================
// validateRegionType 白名单枚举校验
// ============================================================

func TestRegion_ValidateRegionType(t *testing.T) {
	s := &RegionService{}

	cases := []struct {
		name     string
		input    string
		wantCode int // 0 = 期望 nil
	}{
		{"合法 wilderness", "wilderness", 0},
		{"合法 town", "town", 0},
		{"空串非法", "", errcode.ErrRegionTypeInvalid},
		{"大小写敏感（wilderness != Wilderness）", "Wilderness", errcode.ErrRegionTypeInvalid},
		{"未知枚举", "dungeon", errcode.ErrRegionTypeInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := s.validateRegionType(c.input)
			if c.wantCode == 0 {
				if err != nil {
					t.Fatalf("want nil, got %v", err)
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
