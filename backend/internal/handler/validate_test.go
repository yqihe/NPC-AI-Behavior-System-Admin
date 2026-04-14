package handler

import (
	"strings"
	"testing"

	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
)

// ============================================================
// CheckName
// ============================================================

func TestCheckName(t *testing.T) {
	const (
		maxLen  = 32
		errCode = errcode.ErrFieldNameInvalid
		subject = "字段标识"
	)

	tests := []struct {
		name          string
		input         string
		wantNil       bool
		wantCode      int
		wantMsgSubstr string
	}{
		{"合法_全小写", "field_name", true, 0, ""},
		{"合法_含数字", "field_1", true, 0, ""},
		{"空串", "", false, errCode, "字段标识不能为空"},
		{"非法_大写", "FieldName", false, errCode, ""},            // 走 errcode 默认消息
		{"非法_数字开头", "1field", false, errCode, ""},
		{"非法_短横线", "field-name", false, errCode, ""},
		{"非法_空格", "field name", false, errCode, ""},
		{"非法_中文", "字段名", false, errCode, ""},
		{"非法_下划线开头", "_field", false, errCode, ""},
		{"超长_33_ASCII", strings.Repeat("a", 33), false, errCode, "长度不能超过 32 个字符"},
		{"临界_32_ASCII", strings.Repeat("a", 32), true, 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckName(tt.input, maxLen, errCode, subject)
			if tt.wantNil {
				if err != nil {
					t.Fatalf("want nil, got err: %v (code=%d)", err.Message, err.Code)
				}
				return
			}
			if err == nil {
				t.Fatalf("want error, got nil")
			}
			if err.Code != tt.wantCode {
				t.Errorf("code: want %d, got %d", tt.wantCode, err.Code)
			}
			if tt.wantMsgSubstr != "" && !strings.Contains(err.Message, tt.wantMsgSubstr) {
				t.Errorf("message: want substr %q, got %q", tt.wantMsgSubstr, err.Message)
			}
		})
	}
}

// TestCheckName_ErrCodeTransparency 验证 errCode 参数不同模块透传正确
func TestCheckName_ErrCodeTransparency(t *testing.T) {
	cases := []struct {
		name    string
		errCode int
	}{
		{"field", errcode.ErrFieldNameInvalid},
		{"template", errcode.ErrTemplateNameInvalid},
		{"event_type", errcode.ErrEventTypeNameInvalid},
		{"fsm_config", errcode.ErrFsmConfigNameInvalid},
		{"ext_schema", errcode.ErrExtSchemaNameInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := CheckName("", 32, c.errCode, "某标识")
			if err == nil || err.Code != c.errCode {
				t.Fatalf("want code %d, got %v", c.errCode, err)
			}
		})
	}
}

// ============================================================
// CheckLabel
// ============================================================

func TestCheckLabel(t *testing.T) {
	const (
		maxLen  = 10
		subject = "中文标签"
	)

	tests := []struct {
		name          string
		input         string
		wantNil       bool
		wantMsgSubstr string
	}{
		{"合法_ASCII", "Label", true, ""},
		{"合法_中文", "字段标签", true, ""},
		{"合法_混合", "Label标签", true, ""},
		{"空串", "", false, "中文标签不能为空"},
		// 关键用例：10 个中文在 UTF-8 下是 30 字节，必须用 RuneCountInString 而非 len 才能判断是否 <= 10
		{"临界_10_中文", "一二三四五六七八九十", true, ""},
		// 11 个中文必须被拒绝
		{"超长_11_中文", "一二三四五六七八九十一", false, "长度不能超过 10 个字符"},
		{"超长_11_ASCII", "abcdefghijk", false, "长度不能超过 10 个字符"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckLabel(tt.input, maxLen, subject)
			if tt.wantNil {
				if err != nil {
					t.Fatalf("want nil, got err: %v (code=%d)", err.Message, err.Code)
				}
				return
			}
			if err == nil {
				t.Fatalf("want error, got nil")
			}
			if err.Code != errcode.ErrBadRequest {
				t.Errorf("code: want %d (ErrBadRequest), got %d", errcode.ErrBadRequest, err.Code)
			}
			if tt.wantMsgSubstr != "" && !strings.Contains(err.Message, tt.wantMsgSubstr) {
				t.Errorf("message: want substr %q, got %q", tt.wantMsgSubstr, err.Message)
			}
		})
	}
}
