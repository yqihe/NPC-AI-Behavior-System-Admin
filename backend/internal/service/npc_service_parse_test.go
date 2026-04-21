package service

import (
	"encoding/json"
	"testing"

	"github.com/yqihe/npc-ai-admin/backend/internal/model"
)

// ============================================================
// NpcService 纯解析辅助方法测试：
// ExtractFieldIDsFromNPC / ParseNPCFieldEntries / FillSnapshotNames。
//
// 均无 store/cache 依赖，zero-value *NpcService 即可测。
// ============================================================

// --- ExtractFieldIDsFromNPC ---

func TestExtractFieldIDsFromNPC(t *testing.T) {
	s := &NpcService{}

	t.Run("合法数组 → ids", func(t *testing.T) {
		raw := json.RawMessage(`[
			{"field_id":1,"name":"hp","required":true,"value":100},
			{"field_id":2,"name":"mp","required":false,"value":30}
		]`)
		ids, err := s.ExtractFieldIDsFromNPC(raw)
		if err != nil {
			t.Fatalf("want nil err, got %v", err)
		}
		if len(ids) != 2 || ids[0] != 1 || ids[1] != 2 {
			t.Errorf("want [1,2], got %v", ids)
		}
	})

	t.Run("空数组 → 空切片", func(t *testing.T) {
		ids, err := s.ExtractFieldIDsFromNPC(json.RawMessage(`[]`))
		if err != nil {
			t.Fatalf("want nil err, got %v", err)
		}
		if ids == nil {
			t.Fatal("want non-nil empty slice")
		}
		if len(ids) != 0 {
			t.Errorf("want len=0, got %d", len(ids))
		}
	})

	t.Run("非法 JSON 返错", func(t *testing.T) {
		_, err := s.ExtractFieldIDsFromNPC(json.RawMessage(`{not-array}`))
		if err == nil {
			t.Fatal("want err, got nil")
		}
	})
}

// --- ParseNPCFieldEntries ---

func TestParseNPCFieldEntries(t *testing.T) {
	s := &NpcService{}

	t.Run("合法数组 → 两侧切片", func(t *testing.T) {
		raw := json.RawMessage(`[
			{"field_id":1,"name":"hp","required":true,"value":100},
			{"field_id":2,"name":"mp","required":false,"value":30}
		]`)
		entries, tplEntries, err := s.ParseNPCFieldEntries(raw)
		if err != nil {
			t.Fatalf("want nil err, got %v", err)
		}
		if len(entries) != 2 || entries[0].FieldID != 1 || entries[0].Name != "hp" || !entries[0].Required {
			t.Errorf("entries 解析错: %+v", entries)
		}
		if len(tplEntries) != 2 || tplEntries[0].FieldID != 1 || !tplEntries[0].Required ||
			tplEntries[1].FieldID != 2 || tplEntries[1].Required {
			t.Errorf("tplEntries 解析错: %+v", tplEntries)
		}
	})

	t.Run("空数组 → 两侧空切片", func(t *testing.T) {
		entries, tplEntries, err := s.ParseNPCFieldEntries(json.RawMessage(`[]`))
		if err != nil {
			t.Fatalf("want nil err, got %v", err)
		}
		if len(entries) != 0 || len(tplEntries) != 0 {
			t.Errorf("want empty, got entries=%v tpl=%v", entries, tplEntries)
		}
	})

	t.Run("非法 JSON 返错", func(t *testing.T) {
		_, _, err := s.ParseNPCFieldEntries(json.RawMessage(`{not-array}`))
		if err == nil {
			t.Fatal("want err, got nil")
		}
	})
}

// --- FillSnapshotNames ---

func TestFillSnapshotNames(t *testing.T) {
	s := &NpcService{}

	t.Run("老快照有 name → 回填", func(t *testing.T) {
		snapshot := []model.NPCFieldEntry{
			{FieldID: 1, Name: ""},
			{FieldID: 2, Name: ""},
		}
		old := []model.NPCFieldEntry{
			{FieldID: 1, Name: "hp"},
			{FieldID: 2, Name: "mp"},
		}
		s.FillSnapshotNames(snapshot, old)
		if snapshot[0].Name != "hp" || snapshot[1].Name != "mp" {
			t.Errorf("回填失败: %+v", snapshot)
		}
	})

	t.Run("老快照缺 id → 保留新 name 不覆盖", func(t *testing.T) {
		snapshot := []model.NPCFieldEntry{
			{FieldID: 1, Name: "orig1"},
			{FieldID: 99, Name: "orig99"}, // 老快照没有 99
		}
		old := []model.NPCFieldEntry{
			{FieldID: 1, Name: "hp"},
		}
		s.FillSnapshotNames(snapshot, old)
		if snapshot[0].Name != "hp" {
			t.Errorf("id=1 应被回填为 hp, got %q", snapshot[0].Name)
		}
		if snapshot[1].Name != "orig99" {
			t.Errorf("id=99 老无对应, 应保留原值 orig99, got %q", snapshot[1].Name)
		}
	})

	t.Run("老快照为空 → snapshot 原样不变", func(t *testing.T) {
		snapshot := []model.NPCFieldEntry{
			{FieldID: 1, Name: "orig1"},
		}
		s.FillSnapshotNames(snapshot, nil)
		if snapshot[0].Name != "orig1" {
			t.Errorf("应保留原值, got %q", snapshot[0].Name)
		}
	})

	t.Run("snapshot 为空 → 不 panic", func(t *testing.T) {
		old := []model.NPCFieldEntry{{FieldID: 1, Name: "hp"}}
		s.FillSnapshotNames(nil, old)
		s.FillSnapshotNames([]model.NPCFieldEntry{}, old)
		// no panic = pass
	})

	t.Run("老快照 name 为空串 → 也会覆盖 snapshot", func(t *testing.T) {
		// 当前实现语义：只要 oldMap 里有 id 就覆盖，即便 name 是空串
		// （业务约束由上层保证 old 里的 name 总是非空）
		snapshot := []model.NPCFieldEntry{{FieldID: 1, Name: "new_name"}}
		old := []model.NPCFieldEntry{{FieldID: 1, Name: ""}}
		s.FillSnapshotNames(snapshot, old)
		if snapshot[0].Name != "" {
			t.Errorf("当前行为：有对应 id 就覆盖；got %q", snapshot[0].Name)
		}
	})
}
