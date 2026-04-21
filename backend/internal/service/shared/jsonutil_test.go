package shared

import (
	"encoding/json"
	"strings"
	"testing"
)

// ============================================================
// jsonutil.go 全量单测：6 个纯函数均无 DB/业务依赖。
// ============================================================

func TestParseConstraintsMap_EmptyReturnsEmptyMap(t *testing.T) {
	m, err := ParseConstraintsMap(nil)
	if err != nil {
		t.Fatalf("nil: want nil err, got %v", err)
	}
	if m == nil || len(m) != 0 {
		t.Errorf("nil: want empty non-nil map, got %v", m)
	}

	m, err = ParseConstraintsMap(json.RawMessage{})
	if err != nil {
		t.Fatalf("empty slice: want nil err, got %v", err)
	}
	if m == nil || len(m) != 0 {
		t.Errorf("empty slice: want empty non-nil map, got %v", m)
	}
}

func TestParseConstraintsMap_ValidJSON(t *testing.T) {
	raw := json.RawMessage(`{"min":0,"max":100,"label":"ok"}`)
	m, err := ParseConstraintsMap(raw)
	if err != nil {
		t.Fatalf("want nil err, got %v", err)
	}
	if len(m) != 3 {
		t.Errorf("want 3 keys, got %d", len(m))
	}
	if string(m["min"]) != "0" {
		t.Errorf("min: want '0', got %q", string(m["min"]))
	}
	if string(m["max"]) != "100" {
		t.Errorf("max: want '100', got %q", string(m["max"]))
	}
	if string(m["label"]) != `"ok"` {
		t.Errorf("label: want '\"ok\"', got %q", string(m["label"]))
	}
}

func TestParseConstraintsMap_InvalidJSONErrors(t *testing.T) {
	raw := json.RawMessage(`{not-json}`)
	_, err := ParseConstraintsMap(raw)
	if err == nil {
		t.Fatal("want err, got nil")
	}
	if !strings.Contains(err.Error(), "unmarshal constraints") {
		t.Errorf("want err 含 'unmarshal constraints', got %q", err.Error())
	}
}

func TestGetFloat(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantVal float64
		wantOK  bool
	}{
		{"正整数", `42`, 42, true},
		{"负整数", `-7`, -7, true},
		{"零", `0`, 0, true},
		{"小数", `3.14`, 3.14, true},
		{"字符串非法", `"abc"`, 0, false},
		{"空 raw 非法", ``, 0, false},
		// Go json quirk: Unmarshal("null", &float64) 是 no-op 不报错 → ok=true，值保持零
		{"null 被当作 0", `null`, 0, true},
		{"bool 非法", `true`, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v, ok := GetFloat(json.RawMessage(c.raw))
			if ok != c.wantOK {
				t.Errorf("ok: want %v, got %v", c.wantOK, ok)
			}
			if v != c.wantVal {
				t.Errorf("val: want %v, got %v", c.wantVal, v)
			}
		})
	}
}

func TestGetString(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"空 raw", ``, ""},
		{"普通字符串", `"hello"`, "hello"},
		{"中文", `"你好"`, "你好"},
		{"空串", `""`, ""},
		{"非字符串 number", `123`, ""},
		{"非字符串 null", `null`, ""},
		{"非字符串 bool", `true`, ""},
		{"非字符串 array", `["x"]`, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := GetString(json.RawMessage(c.raw))
			if got != c.want {
				t.Errorf("want %q, got %q", c.want, got)
			}
		})
	}
}

func TestGetBool(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantVal bool
		wantOK  bool
	}{
		{"true", `true`, true, true},
		{"false", `false`, false, true},
		{"数字非法", `1`, false, false},
		{"字符串非法", `"true"`, false, false},
		{"空 raw 非法", ``, false, false},
		// Go json quirk: Unmarshal("null", &bool) 是 no-op 不报错 → ok=true，值保持零
		{"null 被当作 false", `null`, false, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v, ok := GetBool(json.RawMessage(c.raw))
			if ok != c.wantOK {
				t.Errorf("ok: want %v, got %v", c.wantOK, ok)
			}
			if v != c.wantVal {
				t.Errorf("val: want %v, got %v", c.wantVal, v)
			}
		})
	}
}

func TestIsJSONNull(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{"空 raw → true", ``, true},
		{"字面 null → true", `null`, true},
		{"数字 → false", `0`, false},
		{"空串 → false", `""`, false},
		{"bool → false", `false`, false},
		{"空对象 → false", `{}`, false},
		{"空数组 → false", `[]`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := IsJSONNull(json.RawMessage(c.raw))
			if got != c.want {
				t.Errorf("want %v, got %v", c.want, got)
			}
		})
	}
}

func TestParseSelectOptions_Empty(t *testing.T) {
	if got := ParseSelectOptions(nil); got != nil {
		t.Errorf("nil: want nil, got %v", got)
	}
	if got := ParseSelectOptions(json.RawMessage{}); got != nil {
		t.Errorf("empty: want nil, got %v", got)
	}
}

func TestParseSelectOptions_Valid(t *testing.T) {
	raw := json.RawMessage(`[
		{"value":"a","label":"A"},
		{"value":"b","label":"B"},
		{"value":"c","label":"C"}
	]`)
	got := ParseSelectOptions(raw)
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("len: want %d, got %d (%v)", len(want), len(got), got)
	}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("[%d]: want %q, got %q", i, v, got[i])
		}
	}
}

func TestParseSelectOptions_InvalidJSONReturnsNil(t *testing.T) {
	if got := ParseSelectOptions(json.RawMessage(`{not-array}`)); got != nil {
		t.Errorf("非法 JSON: want nil, got %v", got)
	}
	// 非数组的合法 JSON 也返回 nil
	if got := ParseSelectOptions(json.RawMessage(`{"value":"x"}`)); got != nil {
		t.Errorf("对象而非数组: want nil, got %v", got)
	}
}

func TestParseSelectOptions_MissingValueField(t *testing.T) {
	// 元素缺 value 字段：Unmarshal 不报错，value 零值为空串
	raw := json.RawMessage(`[{"label":"A"},{"value":"b"}]`)
	got := ParseSelectOptions(raw)
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
	if got[0] != "" || got[1] != "b" {
		t.Errorf("want ['','b'], got %v", got)
	}
}
