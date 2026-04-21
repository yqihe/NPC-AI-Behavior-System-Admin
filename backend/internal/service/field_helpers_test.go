package service

import (
	"encoding/json"
	"testing"

	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
)

// ============================================================
// field.go 解析辅助函数测试：parseProperties / parseRefFieldIDs /
// validatePropertiesConstraints。
//
// 前两个纯函数无 receiver；validatePropertiesConstraints 挂 *FieldService
// 但实际只调 shared.ValidateConstraintsSelf（纯），可 zero-value 测。
//
// detectCyclicRef 需要 FieldStore mock，未抽接口前先跳过。
// ============================================================

// --- parseProperties ---

func TestParseProperties(t *testing.T) {
	t.Run("空 raw 返回零值非 nil", func(t *testing.T) {
		got, err := parseProperties(nil)
		if err != nil {
			t.Fatalf("want nil err, got %v", err)
		}
		if got == nil {
			t.Fatal("want non-nil empty FieldProperties")
		}
		if got.Description != "" || got.ExposeBB {
			t.Errorf("expected zero values, got %+v", got)
		}
	})

	t.Run("合法 JSON", func(t *testing.T) {
		raw := json.RawMessage(`{
			"description":"HP 值",
			"expose_bb":true,
			"default_value":100,
			"constraints":{"min":0,"max":200}
		}`)
		got, err := parseProperties(raw)
		if err != nil {
			t.Fatalf("want nil err, got %v", err)
		}
		if got.Description != "HP 值" || !got.ExposeBB {
			t.Errorf("字段丢失: %+v", got)
		}
		if string(got.DefaultValue) != "100" {
			t.Errorf("default_value 错: %s", got.DefaultValue)
		}
	})

	t.Run("非法 JSON 返错", func(t *testing.T) {
		_, err := parseProperties(json.RawMessage(`{not-json}`))
		if err == nil {
			t.Fatal("want err, got nil")
		}
	})
}

// --- parseRefFieldIDs ---

func TestParseRefFieldIDs(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want []int64
	}{
		{"空 raw 返 nil", ``, nil},
		{"非法 JSON 返 nil（容错降级）", `{not-json}`, nil},
		{"缺 refs 字段返空", `{"other":1}`, nil},
		{"合法 refs", `{"refs":[1,2,3]}`, []int64{1, 2, 3}},
		{"空 refs 数组", `{"refs":[]}`, []int64{}},
		{"refs 带其他约束", `{"refs":[10],"min":0}`, []int64{10}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseRefFieldIDs(json.RawMessage(c.raw))
			if len(got) != len(c.want) {
				t.Fatalf("want %v, got %v", c.want, got)
			}
			for i := range c.want {
				if got[i] != c.want[i] {
					t.Errorf("[%d]: want %d, got %d", i, c.want[i], got[i])
				}
			}
		})
	}
}

// --- validatePropertiesConstraints ---

func TestValidatePropertiesConstraints(t *testing.T) {
	s := &FieldService{}

	t.Run("reference 类型直接放行（不校验）", func(t *testing.T) {
		// reference 的 refs 由 validateReferenceRefs 单独处理
		raw := json.RawMessage(`{"constraints":{"min":9999,"max":0}}`) // 故意 min>max
		if err := s.validatePropertiesConstraints("reference", raw); err != nil {
			t.Errorf("reference 应放行，got %v", err)
		}
	})

	t.Run("空 properties 放行", func(t *testing.T) {
		if err := s.validatePropertiesConstraints("integer", nil); err != nil {
			t.Errorf("want nil, got %v", err)
		}
	})

	t.Run("非法 JSON 容错放行", func(t *testing.T) {
		// parseProperties 失败 → 返 nil（非静默错误，约定为放行，由上层字段校验兜底）
		if err := s.validatePropertiesConstraints("integer", json.RawMessage(`{not-json}`)); err != nil {
			t.Errorf("want nil, got %v", err)
		}
	})

	t.Run("空 constraints 放行", func(t *testing.T) {
		raw := json.RawMessage(`{"description":"x"}`)
		if err := s.validatePropertiesConstraints("integer", raw); err != nil {
			t.Errorf("want nil, got %v", err)
		}
	})

	t.Run("integer 合法 min<=max 通过", func(t *testing.T) {
		raw := json.RawMessage(`{"constraints":{"min":0,"max":100}}`)
		if err := s.validatePropertiesConstraints("integer", raw); err != nil {
			t.Errorf("want nil, got %v", err)
		}
	})

	t.Run("integer min>max 返错", func(t *testing.T) {
		raw := json.RawMessage(`{"constraints":{"min":100,"max":0}}`)
		err := s.validatePropertiesConstraints("integer", raw)
		if err == nil {
			t.Fatal("want err, got nil")
		}
		if err.Code != errcode.ErrBadRequest {
			t.Errorf("want code=%d, got code=%d", errcode.ErrBadRequest, err.Code)
		}
	})

	t.Run("float precision<=0 返错", func(t *testing.T) {
		raw := json.RawMessage(`{"constraints":{"precision":0}}`)
		err := s.validatePropertiesConstraints("float", raw)
		if err == nil {
			t.Fatal("want err, got nil")
		}
	})

	t.Run("string maxLength<minLength 返错", func(t *testing.T) {
		raw := json.RawMessage(`{"constraints":{"minLength":10,"maxLength":5}}`)
		err := s.validatePropertiesConstraints("string", raw)
		if err == nil {
			t.Fatal("want err, got nil")
		}
	})

	t.Run("bool 任何 constraints 放行", func(t *testing.T) {
		raw := json.RawMessage(`{"constraints":{"anything":"goes"}}`)
		if err := s.validatePropertiesConstraints("bool", raw); err != nil {
			t.Errorf("want nil, got %v", err)
		}
	})
}
