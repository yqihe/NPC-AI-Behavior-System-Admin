package service

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/yqihe/npc-ai-admin/backend/internal/cache"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
)

// ============================================================
// event_type.go 纯辅助方法测试：buildConfigJSON / extractExtensionKeys /
// validateExtensions。
//
// 这三者不依赖 store/db，只依赖 schemaCache（内存）或纯 json 操作。
// 用零值 *EventTypeService + SetSchemasForTest 注入 fixture。
// ============================================================

// --- buildConfigJSON ---

func TestBuildConfigJSON(t *testing.T) {
	s := &EventTypeService{}

	t.Run("系统字段 + 扩展字段合并", func(t *testing.T) {
		data, err := s.buildConfigJSON("爆炸", "global", 80, 3600, 0, map[string]interface{}{
			"damage": 100.0,
			"tag":    "weapon",
		})
		if err != nil {
			t.Fatalf("want nil err, got %v", err)
		}
		var m map[string]interface{}
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		// 系统字段
		if m["display_name"] != "爆炸" || m["perception_mode"] != "global" {
			t.Errorf("系统字段被破坏: %v", m)
		}
		if m["default_severity"].(float64) != 80 || m["default_ttl"].(float64) != 3600 {
			t.Errorf("severity/ttl 错: %v", m)
		}
		if m["range"].(float64) != 0 {
			t.Errorf("range 错: %v", m["range"])
		}
		// 扩展字段
		if m["damage"].(float64) != 100 || m["tag"] != "weapon" {
			t.Errorf("扩展字段丢失: %v", m)
		}
	})

	t.Run("扩展字段 key 与系统字段冲突 — 扩展覆盖", func(t *testing.T) {
		// 当前实现用 range-for 覆盖，扩展字段值会压过系统字段。
		// 这是已有行为，用测试锁死。
		data, _ := s.buildConfigJSON("X", "area", 0, 0, 50, map[string]interface{}{
			"range": 999.0,
		})
		var m map[string]interface{}
		_ = json.Unmarshal(data, &m)
		if m["range"].(float64) != 999 {
			t.Errorf("扩展字段未覆盖系统字段: %v", m["range"])
		}
	})

	t.Run("空扩展字段合法", func(t *testing.T) {
		data, err := s.buildConfigJSON("X", "area", 10, 60, 5, nil)
		if err != nil {
			t.Fatalf("want nil err, got %v", err)
		}
		var m map[string]interface{}
		_ = json.Unmarshal(data, &m)
		if m["display_name"] != "X" {
			t.Errorf("display_name 错: %v", m)
		}
	})

	// 触发 json.Marshal 失败分支：chan 无法被 json 序列化
	t.Run("不可序列化扩展字段返 marshal err", func(t *testing.T) {
		_, err := s.buildConfigJSON("X", "area", 0, 0, 0, map[string]interface{}{
			"bad": make(chan int),
		})
		if err == nil {
			t.Fatal("want err, got nil")
		}
		if !strings.Contains(err.Error(), "marshal config_json") {
			t.Errorf("err 应含 'marshal config_json', got %q", err.Error())
		}
	})
}

// --- extractExtensionKeys ---

func TestExtractExtensionKeys(t *testing.T) {
	s := &EventTypeService{}

	t.Run("系统字段被排除，只留扩展字段", func(t *testing.T) {
		cfg := json.RawMessage(`{
			"display_name":"爆炸","default_severity":80,"default_ttl":3600,
			"perception_mode":"global","range":0,
			"damage":100,"tag":"weapon"
		}`)
		keys := s.extractExtensionKeys(cfg)
		if len(keys) != 2 || !keys["damage"] || !keys["tag"] {
			t.Fatalf("expected {damage,tag}, got %v", keys)
		}
	})

	t.Run("无扩展字段返回空 map", func(t *testing.T) {
		cfg := json.RawMessage(`{"display_name":"X","default_severity":0,"default_ttl":0,"perception_mode":"area","range":5}`)
		keys := s.extractExtensionKeys(cfg)
		if len(keys) != 0 {
			t.Errorf("expected empty, got %v", keys)
		}
	})

	t.Run("非法 JSON 容错返回空 map", func(t *testing.T) {
		keys := s.extractExtensionKeys(json.RawMessage(`not-json`))
		if keys == nil {
			t.Fatal("want non-nil empty map, got nil")
		}
		if len(keys) != 0 {
			t.Errorf("want empty, got %v", keys)
		}
	})
}

// --- validateExtensions ---

func TestValidateExtensions(t *testing.T) {
	// schemaCache 用 nil store 构造（SetSchemasForTest 不走 DB）
	sc := cache.NewEventTypeSchemaCache(nil)
	sc.SetSchemasForTest([]model.EventTypeSchemaLite{
		{
			ID:          1,
			FieldName:   "damage",
			FieldLabel:  "伤害",
			FieldType:   "float",
			Constraints: json.RawMessage(`{"min":0,"max":1000}`),
			Enabled:     true,
		},
		{
			ID:          2,
			FieldName:   "tag",
			FieldLabel:  "标签",
			FieldType:   "string",
			Constraints: json.RawMessage(`{"maxLength":16}`),
			Enabled:     true,
		},
	})
	s := &EventTypeService{schemaCache: sc}

	t.Run("空扩展字段直接通过", func(t *testing.T) {
		if err := s.validateExtensions(nil); err != nil {
			t.Errorf("want nil, got %v", err)
		}
		if err := s.validateExtensions(map[string]interface{}{}); err != nil {
			t.Errorf("want nil, got %v", err)
		}
	})

	t.Run("所有值合法通过", func(t *testing.T) {
		err := s.validateExtensions(map[string]interface{}{
			"damage": 100.0,
			"tag":    "fire",
		})
		if err != nil {
			t.Errorf("want nil, got %v", err)
		}
	})

	t.Run("未知 schema 返回 ErrExtSchemaNotFound", func(t *testing.T) {
		err := s.validateExtensions(map[string]interface{}{
			"unknown_field": 1,
		})
		var codeErr *errcode.Error
		if !errors.As(err, &codeErr) {
			t.Fatalf("want *errcode.Error, got %T: %v", err, err)
		}
		if codeErr.Code != errcode.ErrExtSchemaNotFound {
			t.Errorf("want code=%d, got code=%d", errcode.ErrExtSchemaNotFound, codeErr.Code)
		}
	})

	t.Run("值违反约束返回 ErrEventTypeExtValueInvalid", func(t *testing.T) {
		// damage max=1000，给 2000 超限
		err := s.validateExtensions(map[string]interface{}{
			"damage": 2000.0,
		})
		var codeErr *errcode.Error
		if !errors.As(err, &codeErr) {
			t.Fatalf("want *errcode.Error, got %T: %v", err, err)
		}
		if codeErr.Code != errcode.ErrEventTypeExtValueInvalid {
			t.Errorf("want code=%d, got code=%d (msg=%q)",
				errcode.ErrEventTypeExtValueInvalid, codeErr.Code, codeErr.Message)
		}
	})

	// 触发 json.Marshal(val) 失败分支：damage schema 存在，但 value 是 chan 无法序列化
	t.Run("扩展字段值不可序列化返 ErrEventTypeExtValueInvalid", func(t *testing.T) {
		err := s.validateExtensions(map[string]interface{}{
			"damage": make(chan int),
		})
		var codeErr *errcode.Error
		if !errors.As(err, &codeErr) {
			t.Fatalf("want *errcode.Error, got %T: %v", err, err)
		}
		if codeErr.Code != errcode.ErrEventTypeExtValueInvalid {
			t.Errorf("want code=%d, got code=%d", errcode.ErrEventTypeExtValueInvalid, codeErr.Code)
		}
		if !strings.Contains(codeErr.Message, "序列化失败") {
			t.Errorf("err 应含 '序列化失败', got %q", codeErr.Message)
		}
	})
}
