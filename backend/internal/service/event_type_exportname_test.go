package service

import (
	"encoding/json"
	"testing"
)

// TestInjectNameIntoConfig 回归锁死：导出时必须把外层 name 注入 config JSON。
// 游戏服务端 cmd/server/main.go:249 用 cfg.Name（从 config 内部读）作索引键，
// 缺失则所有事件落到空字符串 key，事件系统失效。
func TestInjectNameIntoConfig(t *testing.T) {
	t.Run("普通对象注入 name", func(t *testing.T) {
		in := json.RawMessage(`{"display_name":"爆炸","default_severity":80}`)
		out, err := injectNameIntoConfig("explosion", in)
		if err != nil {
			t.Fatalf("want nil err, got %v", err)
		}
		var m map[string]interface{}
		if err := json.Unmarshal(out, &m); err != nil {
			t.Fatalf("unmarshal result: %v", err)
		}
		if m["name"] != "explosion" {
			t.Errorf("name want=explosion got=%v", m["name"])
		}
		if m["display_name"] != "爆炸" {
			t.Errorf("display_name 字段被破坏: %v", m["display_name"])
		}
	})

	t.Run("外层 name 覆盖 config 内已有 name", func(t *testing.T) {
		in := json.RawMessage(`{"name":"stale","display_name":"爆炸"}`)
		out, err := injectNameIntoConfig("explosion", in)
		if err != nil {
			t.Fatalf("want nil err, got %v", err)
		}
		var m map[string]interface{}
		_ = json.Unmarshal(out, &m)
		if m["name"] != "explosion" {
			t.Errorf("外层 name 应覆盖 config 内 name, got=%v", m["name"])
		}
	})

	t.Run("空对象也能注入", func(t *testing.T) {
		in := json.RawMessage(`{}`)
		out, err := injectNameIntoConfig("shout", in)
		if err != nil {
			t.Fatalf("want nil err, got %v", err)
		}
		var m map[string]interface{}
		_ = json.Unmarshal(out, &m)
		if m["name"] != "shout" {
			t.Errorf("name want=shout got=%v", m["name"])
		}
	})

	t.Run("非法 JSON 返错", func(t *testing.T) {
		in := json.RawMessage(`not-json`)
		if _, err := injectNameIntoConfig("x", in); err == nil {
			t.Fatal("want err, got nil")
		}
	})

	// null config 触发 m == nil → 重建空 map 分支
	t.Run("null config 仍能注入 name", func(t *testing.T) {
		out, err := injectNameIntoConfig("only_name", json.RawMessage(`null`))
		if err != nil {
			t.Fatalf("want nil err, got %v", err)
		}
		var m map[string]interface{}
		if err := json.Unmarshal(out, &m); err != nil {
			t.Fatalf("unmarshal result: %v", err)
		}
		if m["name"] != "only_name" {
			t.Errorf("name want=only_name got=%v", m["name"])
		}
		if len(m) != 1 {
			t.Errorf("null config 重建 map 后只应有 name, got %v", m)
		}
	})
}
