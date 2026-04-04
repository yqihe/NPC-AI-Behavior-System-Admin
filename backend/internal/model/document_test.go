package model

import (
	"encoding/json"
	"testing"
)

func TestToBsonDocumentAndBack(t *testing.T) {
	original := Document{
		Name:   "explosion",
		Config: json.RawMessage(`{"default_severity":80,"default_ttl":15.0,"perception_mode":"auditory","range":500.0}`),
	}

	bdoc, err := ToBsonDocument(original)
	if err != nil {
		t.Fatalf("ToBsonDocument failed: %v", err)
	}
	if bdoc.Name != "explosion" {
		t.Errorf("Name mismatch: got %q, want %q", bdoc.Name, "explosion")
	}
	if len(bdoc.Config) == 0 {
		t.Fatal("Config is empty after ToBsonDocument")
	}

	roundtrip, err := FromBsonDocument(bdoc)
	if err != nil {
		t.Fatalf("FromBsonDocument failed: %v", err)
	}
	if roundtrip.Name != original.Name {
		t.Errorf("Name mismatch after roundtrip: got %q, want %q", roundtrip.Name, original.Name)
	}

	// 验证 config 内容一致（解析为 map 比较，避免 key 顺序问题）
	var origMap, rtMap map[string]any
	if err := json.Unmarshal(original.Config, &origMap); err != nil {
		t.Fatalf("Unmarshal original config: %v", err)
	}
	if err := json.Unmarshal(roundtrip.Config, &rtMap); err != nil {
		t.Fatalf("Unmarshal roundtrip config: %v", err)
	}
	for k, v := range origMap {
		if rtMap[k] != v {
			t.Errorf("Config field %q: got %v, want %v", k, rtMap[k], v)
		}
	}
}

func TestToBsonDocument_InvalidJSON(t *testing.T) {
	doc := Document{
		Name:   "bad",
		Config: json.RawMessage(`{invalid json}`),
	}
	_, err := ToBsonDocument(doc)
	if err == nil {
		t.Fatal("Expected error for invalid JSON, got nil")
	}
}

func TestNewListResponse_NilSlice(t *testing.T) {
	resp := NewListResponse(nil)
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	got := string(data)
	want := `{"items":[]}`
	if got != want {
		t.Errorf("NewListResponse(nil) serialized to %s, want %s", got, want)
	}
}

func TestNewListResponse_EmptySlice(t *testing.T) {
	resp := NewListResponse(make([]Document, 0))
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	got := string(data)
	want := `{"items":[]}`
	if got != want {
		t.Errorf("NewListResponse(empty) serialized to %s, want %s", got, want)
	}
}

func TestNewListResponse_WithItems(t *testing.T) {
	docs := []Document{
		{Name: "a", Config: json.RawMessage(`{"x":1}`)},
	}
	resp := NewListResponse(docs)
	if len(resp.Items) != 1 {
		t.Errorf("Items count: got %d, want 1", len(resp.Items))
	}
}

func TestErrorResponse_Format(t *testing.T) {
	resp := ErrorResponse{Error: "名称已存在"}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	got := string(data)
	want := `{"error":"名称已存在"}`
	if got != want {
		t.Errorf("ErrorResponse serialized to %s, want %s", got, want)
	}
}

// 攻击性测试：空 config
func TestToBsonDocument_EmptyConfig(t *testing.T) {
	doc := Document{
		Name:   "empty",
		Config: json.RawMessage(`{}`),
	}
	bdoc, err := ToBsonDocument(doc)
	if err != nil {
		t.Fatalf("ToBsonDocument failed for empty config: %v", err)
	}
	rt, err := FromBsonDocument(bdoc)
	if err != nil {
		t.Fatalf("FromBsonDocument failed for empty config: %v", err)
	}
	if string(rt.Config) != `{}` {
		t.Errorf("Empty config roundtrip: got %s, want {}", string(rt.Config))
	}
}

// 攻击性测试：嵌套复杂 config（模拟 FSM 条件）
func TestToBsonDocument_NestedConfig(t *testing.T) {
	configJSON := `{"initial_state":"Idle","states":[{"name":"Idle"}],"transitions":[{"from":"Idle","to":"Alarmed","priority":10,"condition":{"and":[{"key":"threat_level","op":">=","value":50}]}}]}`
	doc := Document{
		Name:   "civilian",
		Config: json.RawMessage(configJSON),
	}
	bdoc, err := ToBsonDocument(doc)
	if err != nil {
		t.Fatalf("ToBsonDocument failed for nested config: %v", err)
	}
	rt, err := FromBsonDocument(bdoc)
	if err != nil {
		t.Fatalf("FromBsonDocument failed for nested config: %v", err)
	}

	// 确认嵌套结构完整
	var m map[string]any
	if err := json.Unmarshal(rt.Config, &m); err != nil {
		t.Fatalf("Unmarshal roundtrip config: %v", err)
	}
	if m["initial_state"] != "Idle" {
		t.Errorf("initial_state: got %v, want Idle", m["initial_state"])
	}
}

// 攻击性测试：nil RawMessage
func TestToBsonDocument_NilConfig(t *testing.T) {
	doc := Document{
		Name:   "nil-config",
		Config: nil,
	}
	_, err := ToBsonDocument(doc)
	if err == nil {
		t.Fatal("Expected error for nil config, got nil")
	}
}
