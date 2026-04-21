package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yqihe/npc-ai-admin/backend/internal/model"
)

// ============================================================
// NpcService 导出辅助方法测试（覆盖 T6 的 4 个纯方法）
//
// service.NpcService 不持有 fsm/bt service（项目硬约束），所以本测试无需 mock；
// CollectExportRefs / BuildExportDanglingError / AssembleExportItems 都是
// 纯输入输出，零外部依赖。ExportRows 是 store passthrough，由 store 层覆盖。
// ============================================================

// mkNPC 测试 NPC 行构造器
func mkNPC(name, fsm, btJSON, fieldsJSON, tplName string) model.NPC {
	if btJSON == "" {
		btJSON = "{}"
	}
	if fieldsJSON == "" {
		fieldsJSON = "[]"
	}
	return model.NPC{
		Name:         name,
		Label:        name,
		TemplateName: tplName,
		Fields:       json.RawMessage(fieldsJSON),
		FsmRef:       fsm,
		BtRefs:       json.RawMessage(btJSON),
	}
}

// ============================================================
// CollectExportRefs
// ============================================================

func TestCollectExportRefs_Empty(t *testing.T) {
	s := &NpcService{}
	refs, err := s.CollectExportRefs(nil)
	if err != nil {
		t.Fatalf("nil rows: %v", err)
	}
	if refs == nil || refs.FsmIndex == nil || refs.BtIndex == nil {
		t.Fatalf("expected non-nil maps, got %#v", refs)
	}
	if len(refs.FsmIndex) != 0 || len(refs.BtIndex) != 0 {
		t.Fatalf("expected empty indices, got FsmIndex=%v BtIndex=%v",
			refs.FsmIndex, refs.BtIndex)
	}
}

func TestCollectExportRefs_AllRefs(t *testing.T) {
	s := &NpcService{}
	rows := []model.NPC{
		mkNPC("A", "guard", `{"patrol":"p1"}`, "", "tpl"),
		mkNPC("B", "guard", `{"patrol":"p1","alert":"a1"}`, "", "tpl"),
		mkNPC("C", "", `{"idle":"i1"}`, "", "tpl"),    // 空 fsm_ref 不入 FsmIndex
		mkNPC("D", "trader", `{}`, "", "tpl"),         // 空 bt_refs 不入 BtIndex
	}
	refs, err := s.CollectExportRefs(rows)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}

	// FsmIndex: guard → [A,B], trader → [D]
	if got := refs.FsmIndex["guard"]; len(got) != 2 || got[0] != "A" || got[1] != "B" {
		t.Fatalf("FsmIndex[guard]: want [A B], got %v", got)
	}
	if got := refs.FsmIndex["trader"]; len(got) != 1 || got[0] != "D" {
		t.Fatalf("FsmIndex[trader]: want [D], got %v", got)
	}
	if _, ok := refs.FsmIndex[""]; ok {
		t.Fatalf("empty fsm_ref must not be a key")
	}

	// BtIndex: p1 → [(A,patrol),(B,patrol)], a1 → [(B,alert)], i1 → [(C,idle)]
	if got := refs.BtIndex["p1"]; len(got) != 2 {
		t.Fatalf("BtIndex[p1]: want 2 usages, got %d", len(got))
	}
	a1 := refs.BtIndex["a1"]
	if len(a1) != 1 || a1[0].NPCName != "B" || a1[0].State != "alert" {
		t.Fatalf("BtIndex[a1]: want [(B,alert)], got %v", a1)
	}
	i1 := refs.BtIndex["i1"]
	if len(i1) != 1 || i1[0].NPCName != "C" || i1[0].State != "idle" {
		t.Fatalf("BtIndex[i1]: want [(C,idle)], got %v", i1)
	}
}

func TestCollectExportRefs_BadJSON(t *testing.T) {
	s := &NpcService{}
	rows := []model.NPC{mkNPC("X", "g", `not json`, "", "tpl")}
	_, err := s.CollectExportRefs(rows)
	if err == nil {
		t.Fatalf("expected unmarshal error, got nil")
	}
	if !strings.Contains(err.Error(), "unmarshal bt_refs") {
		t.Fatalf("expected 'unmarshal bt_refs' in error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "npc=X") {
		t.Fatalf("expected NPC name in error, got %q", err.Error())
	}
}

// ============================================================
// BuildExportDanglingError
// ============================================================

func TestBuildExportDanglingError_AllValid(t *testing.T) {
	s := &NpcService{}
	refs := &NPCExportRefs{
		FsmIndex: map[string][]string{"guard": {"A"}},
		BtIndex:  map[string][]NPCExportBtUsage{"p1": {{NPCName: "A", State: "patrol"}}},
	}
	got := s.BuildExportDanglingError(refs, nil, nil)
	if got != nil {
		t.Fatalf("expected nil for all-valid, got %#v", got)
	}
}

func TestBuildExportDanglingError_FsmMissing(t *testing.T) {
	s := &NpcService{}
	refs := &NPCExportRefs{
		FsmIndex: map[string][]string{"guard": {"A", "B"}},
		BtIndex:  map[string][]NPCExportBtUsage{},
	}
	got := s.BuildExportDanglingError(refs, []string{"guard"}, nil)
	if got == nil || len(got.Details) != 2 {
		t.Fatalf("want 2 details, got %#v", got)
	}
	for i, d := range got.Details {
		if d.RefType != model.ExportRefTypeFsm {
			t.Fatalf("Details[%d].RefType: want fsm_ref, got %q", i, d.RefType)
		}
		if d.RefValue != "guard" {
			t.Fatalf("Details[%d].RefValue: want guard, got %q", i, d.RefValue)
		}
		if d.Reason != model.ExportRefReasonMissingOrDisabled {
			t.Fatalf("Details[%d].Reason: want %q, got %q",
				i, model.ExportRefReasonMissingOrDisabled, d.Reason)
		}
		if d.State != "" {
			t.Fatalf("Details[%d].State: FSM detail must not carry State, got %q", i, d.State)
		}
	}
}

func TestBuildExportDanglingError_BtMissing(t *testing.T) {
	s := &NpcService{}
	refs := &NPCExportRefs{
		FsmIndex: map[string][]string{},
		BtIndex: map[string][]NPCExportBtUsage{
			"p1": {{NPCName: "A", State: "patrol"}, {NPCName: "B", State: "patrol"}},
		},
	}
	got := s.BuildExportDanglingError(refs, nil, []string{"p1"})
	if got == nil || len(got.Details) != 2 {
		t.Fatalf("want 2 details, got %#v", got)
	}
	for i, d := range got.Details {
		if d.RefType != model.ExportRefTypeBt {
			t.Fatalf("Details[%d].RefType: want bt_ref, got %q", i, d.RefType)
		}
		if d.RefValue != "p1" {
			t.Fatalf("Details[%d].RefValue: want p1, got %q", i, d.RefValue)
		}
		if d.State != "patrol" {
			t.Fatalf("Details[%d].State: want patrol, got %q", i, d.State)
		}
	}
}

func TestBuildExportDanglingError_FsmAndBt(t *testing.T) {
	s := &NpcService{}
	refs := &NPCExportRefs{
		FsmIndex: map[string][]string{"guard": {"A"}},
		BtIndex:  map[string][]NPCExportBtUsage{"p1": {{NPCName: "A", State: "patrol"}}},
	}
	got := s.BuildExportDanglingError(refs, []string{"guard"}, []string{"p1"})
	if got == nil || len(got.Details) != 2 {
		t.Fatalf("want 2 details, got %#v", got)
	}
	// 顺序契约：FSM 在前，BT 在后（design §1.4）
	if got.Details[0].RefType != model.ExportRefTypeFsm {
		t.Fatalf("Details[0]: want FSM first, got %q", got.Details[0].RefType)
	}
	if got.Details[1].RefType != model.ExportRefTypeBt {
		t.Fatalf("Details[1]: want BT second, got %q", got.Details[1].RefType)
	}
}

// ============================================================
// AssembleExportItems
// ============================================================

func TestAssembleExportItems_Empty(t *testing.T) {
	s := &NpcService{}
	got, err := s.AssembleExportItems(nil)
	if err != nil {
		t.Fatalf("nil rows: %v", err)
	}
	if got == nil {
		t.Fatalf("expected non-nil empty slice (避免 JSON null)")
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %d items", len(got))
	}
}

// TestAssembleExportItems_BadFieldsPropagates 触发 assembleExportItem fields unmarshal err
// + AssembleExportItems 外层 return nil, fmt.Errorf 包装路径
func TestAssembleExportItems_BadFieldsPropagates(t *testing.T) {
	s := &NpcService{}
	rows := []model.NPC{
		{
			Name:   "bad_fields",
			Fields: json.RawMessage(`{not-json}`),
			BtRefs: json.RawMessage(`{}`),
		},
	}
	_, err := s.AssembleExportItems(rows)
	if err == nil {
		t.Fatal("want err, got nil")
	}
	if !strings.Contains(err.Error(), "assemble export item for npc") {
		t.Errorf("want 外层包装前缀, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "unmarshal fields") {
		t.Errorf("want 'unmarshal fields' 内层前缀, got %q", err.Error())
	}
}

// TestAssembleExportItems_BadBtRefsPropagates 触发 assembleExportItem bt_refs unmarshal err
func TestAssembleExportItems_BadBtRefsPropagates(t *testing.T) {
	s := &NpcService{}
	rows := []model.NPC{
		{
			Name:   "bad_bt",
			Fields: json.RawMessage(`[]`),
			BtRefs: json.RawMessage(`{not-json}`),
		},
	}
	_, err := s.AssembleExportItems(rows)
	if err == nil {
		t.Fatal("want err, got nil")
	}
	if !strings.Contains(err.Error(), "unmarshal bt_refs") {
		t.Errorf("want 'unmarshal bt_refs', got %q", err.Error())
	}
}

// TestBuildExportDanglingError_DefensiveEmptyDetails 触发 notOK 非空但反查索引无匹配的防御分支
// （理论上 handler 不会这么调用，但代码有这个 guard）
func TestBuildExportDanglingError_DefensiveEmptyDetails(t *testing.T) {
	s := &NpcService{}
	refs := &NPCExportRefs{
		FsmIndex: map[string][]string{}, // 没有任何 NPC 引用 "ghost_fsm"
		BtIndex:  map[string][]NPCExportBtUsage{},
	}
	got := s.BuildExportDanglingError(refs, []string{"ghost_fsm"}, []string{"ghost_bt"})
	if got != nil {
		t.Errorf("防御分支：反查索引空 → 应返回 nil，避免空 Details 错误，got %+v", got)
	}
}

func TestAssembleExportItems_OneRow(t *testing.T) {
	s := &NpcService{}
	rows := []model.NPC{
		mkNPC("guard_basic", "guard", `{"patrol":"p1"}`,
			`[{"name":"hp","value":100}]`, "tpl_guard"),
	}
	items, err := s.AssembleExportItems(rows)
	if err != nil {
		t.Fatalf("assemble: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 item, got %d", len(items))
	}
	it := items[0]
	if it.Name != "guard_basic" {
		t.Fatalf("Name: want guard_basic, got %q", it.Name)
	}
	if it.Config.TemplateRef != "tpl_guard" {
		t.Fatalf("Config.TemplateRef: want tpl_guard, got %q", it.Config.TemplateRef)
	}
	if it.Config.Behavior.FsmRef != "guard" {
		t.Fatalf("Config.Behavior.FsmRef: want guard, got %q", it.Config.Behavior.FsmRef)
	}
	if it.Config.Behavior.BtRefs["patrol"] != "p1" {
		t.Fatalf("Config.Behavior.BtRefs: want {patrol:p1}, got %#v", it.Config.Behavior.BtRefs)
	}
}
