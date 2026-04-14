package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
)

// ============================================================
// CheckConstraintTightened
// ============================================================

func TestCheckConstraintTightened(t *testing.T) {
	const errCode = errcode.ErrFieldRefTighten

	type tc struct {
		name          string
		fieldType     string
		oldC          string
		newC          string
		wantErr       bool
		wantMsgSubstr string
	}
	tests := []tc{
		// ---- integer / int 数字范围 ----
		{"int_min_收紧", "integer", `{"min":10}`, `{"min":20}`, true, "最小值"},
		{"int_min_放宽", "integer", `{"min":20}`, `{"min":10}`, false, ""},
		{"int_max_收紧", "integer", `{"max":100}`, `{"max":50}`, true, "最大值"},
		{"int_max_放宽", "integer", `{"max":50}`, `{"max":100}`, false, ""},
		{"int_新增min_不算收紧", "integer", `{}`, `{"min":10}`, false, ""}, // old 无 min → 不校验
		{"int_删除max_放宽", "integer", `{"max":100}`, `{}`, false, ""},    // new 无 max → 放宽
		{"int_别名int行为一致", "int", `{"min":10}`, `{"min":20}`, true, "最小值"},

		// ---- float 含 precision ----
		{"float_precision_降低", "float", `{"precision":2}`, `{"precision":1}`, true, "precision"},
		{"float_precision_提升_放宽", "float", `{"precision":1}`, `{"precision":2}`, false, ""},
		{"float_min_收紧", "float", `{"min":0.1}`, `{"min":0.5}`, true, "最小值"},

		// ---- string 长度 / pattern ----
		{"string_minLength_收紧", "string", `{"minLength":3}`, `{"minLength":5}`, true, "最小长度"},
		{"string_maxLength_收紧", "string", `{"maxLength":20}`, `{"maxLength":10}`, true, "最大长度"},
		{"string_pattern_变更", "string", `{"pattern":"^[a-z]+$"}`, `{"pattern":"^[A-Z]+$"}`, true, "pattern"},
		{"string_pattern_移除", "string", `{"pattern":"^[a-z]+$"}`, `{}`, false, ""}, // newPat 为空 → 放宽
		{"string_pattern_保持", "string", `{"pattern":"^[a-z]+$"}`, `{"pattern":"^[a-z]+$"}`, false, ""},

		// ---- select 选项 / minSelect / maxSelect ----
		{
			"select_删除选项",
			"select",
			`{"options":[{"value":"a"},{"value":"b"},{"value":"c"}]}`,
			`{"options":[{"value":"a"},{"value":"b"}]}`,
			true, "选项",
		},
		{
			"select_新增选项_放宽",
			"select",
			`{"options":[{"value":"a"},{"value":"b"}]}`,
			`{"options":[{"value":"a"},{"value":"b"},{"value":"c"}]}`,
			false, "",
		},
		{"select_minSelect_收紧", "select", `{"minSelect":1}`, `{"minSelect":2}`, true, "minSelect"},
		{"select_maxSelect_收紧", "select", `{"maxSelect":3}`, `{"maxSelect":2}`, true, "maxSelect"},

		// ---- bool / reference / unknown：switch 未命中分支 → 不报错 ----
		{"bool_不校验", "bool", `{}`, `{}`, false, ""},
		{"unknown_type_不校验", "unknown", `{"min":10}`, `{"min":20}`, false, ""},

		// ---- 空 constraints ----
		{"空约束_到_空约束", "integer", `{}`, `{}`, false, ""},
		{"nil_oldConstraints", "integer", ``, `{"min":10}`, false, ""}, // old 空 map → 取不到 min，不校验
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckConstraintTightened(tt.fieldType, json.RawMessage(tt.oldC), json.RawMessage(tt.newC), errCode)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("want error, got nil")
				}
				if err.Code != errCode {
					t.Errorf("code: want %d, got %d", errCode, err.Code)
				}
				if tt.wantMsgSubstr != "" && !strings.Contains(err.Message, tt.wantMsgSubstr) {
					t.Errorf("message: want substr %q, got %q", tt.wantMsgSubstr, err.Message)
				}
				return
			}
			if err != nil {
				t.Fatalf("want nil, got err: %v (code=%d)", err.Message, err.Code)
			}
		})
	}
}

// TestCheckConstraintTightened_ErrCodeTransparency 验证 errCode 透传
// field 模块用 ErrFieldRefTighten，扩展字段模块用 ErrExtSchemaRefTighten。
func TestCheckConstraintTightened_ErrCodeTransparency(t *testing.T) {
	cases := []struct {
		name    string
		errCode int
	}{
		{"field", errcode.ErrFieldRefTighten},
		{"ext_schema", errcode.ErrExtSchemaRefTighten},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := CheckConstraintTightened("integer",
				json.RawMessage(`{"min":10}`),
				json.RawMessage(`{"min":20}`),
				c.errCode)
			if err == nil || err.Code != c.errCode {
				t.Fatalf("want code %d, got %v", c.errCode, err)
			}
		})
	}
}
