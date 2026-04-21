package shared

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
)

// ============================================================
// validate.go 全量单测：NormalizePagination + ValidateValue + ValidateConstraintsSelf。
// 所有分支纯逻辑，无 DB。
// ============================================================

// --------- NormalizePagination ---------

func TestNormalizePagination(t *testing.T) {
	cases := []struct {
		name                      string
		page, pageSize            int
		defPage, defPageSize, max int
		wantPage, wantPageSize    int
	}{
		{"page<1", 0, 20, 1, 20, 100, 1, 20},
		{"page<1 负数", -5, 20, 1, 20, 100, 1, 20},
		{"pageSize<1", 2, 0, 1, 20, 100, 2, 20},
		{"pageSize 超限", 2, 500, 1, 20, 100, 2, 100},
		{"正常不变", 3, 50, 1, 20, 100, 3, 50},
		{"边界 pageSize==max", 1, 100, 1, 20, 100, 1, 100},
		{"page=1 pageSize=1 合法", 1, 1, 1, 20, 100, 1, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p, ps := c.page, c.pageSize
			NormalizePagination(&p, &ps, c.defPage, c.defPageSize, c.max)
			if p != c.wantPage || ps != c.wantPageSize {
				t.Errorf("want (%d,%d), got (%d,%d)", c.wantPage, c.wantPageSize, p, ps)
			}
		})
	}
}

// --------- ValidateValue: int ---------

func TestValidateValue_Int(t *testing.T) {
	cases := []struct {
		name        string
		constraints string
		value       string
		wantErr     bool
		wantMsg     string
	}{
		{"合法 int", `{"min":0,"max":100}`, `50`, false, ""},
		{"等于 min", `{"min":0}`, `0`, false, ""},
		{"等于 max", `{"max":100}`, `100`, false, ""},
		{"小数被拒", `{}`, `3.14`, true, "整数字段不能传入小数"},
		{"非数字被拒", `{}`, `"abc"`, true, "值必须是数字"},
		{"低于 min", `{"min":10}`, `5`, true, "小于最小值"},
		{"高于 max", `{"max":10}`, `100`, true, "大于最大值"},
		{"无约束任意整数", `{}`, `99999`, false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateValue("int", json.RawMessage(c.constraints), json.RawMessage(c.value))
			checkErr(t, err, c.wantErr, c.wantMsg)
		})
	}
	// integer 别名等价
	if err := ValidateValue("integer", json.RawMessage(`{}`), json.RawMessage(`42`)); err != nil {
		t.Errorf("integer 别名: want nil, got %v", err)
	}
}

// --------- ValidateValue: float ---------

func TestValidateValue_Float(t *testing.T) {
	cases := []struct {
		name        string
		constraints string
		value       string
		wantErr     bool
		wantMsg     string
	}{
		{"合法 float", `{"min":0.0,"max":1.0}`, `0.5`, false, ""},
		{"整数被接受", `{}`, `7`, false, ""},
		{"非数字被拒", `{}`, `"nan"`, true, "值必须是数字"},
		{"低于 min", `{"min":1.5}`, `1.0`, true, "小于最小值"},
		{"高于 max", `{"max":1.5}`, `2.0`, true, "大于最大值"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateValue("float", json.RawMessage(c.constraints), json.RawMessage(c.value))
			checkErr(t, err, c.wantErr, c.wantMsg)
		})
	}
}

// --------- ValidateValue: string ---------

func TestValidateValue_String(t *testing.T) {
	cases := []struct {
		name        string
		constraints string
		value       string
		wantErr     bool
		wantMsg     string
	}{
		{"合法 string", `{"minLength":1,"maxLength":10}`, `"hello"`, false, ""},
		{"中文按 rune 计长", `{"maxLength":3}`, `"你好吗"`, false, ""},
		{"太短", `{"minLength":5}`, `"ab"`, true, "小于最小长度"},
		{"太长", `{"maxLength":3}`, `"hello"`, true, "大于最大长度"},
		{"空串被当作长度 0", `{"minLength":1}`, `""`, true, "小于最小长度"},
		{"null 被当作空串", `{"minLength":1}`, `null`, true, "小于最小长度"},
		{"空 raw 被当作空串", `{"minLength":1}`, ``, true, "小于最小长度"},
		{"无约束任意字符串", `{}`, `"whatever"`, false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateValue("string", json.RawMessage(c.constraints), json.RawMessage(c.value))
			checkErr(t, err, c.wantErr, c.wantMsg)
		})
	}
}

// --------- ValidateValue: bool ---------

func TestValidateValue_Bool(t *testing.T) {
	if err := ValidateValue("bool", json.RawMessage(`{}`), json.RawMessage(`true`)); err != nil {
		t.Errorf("true: want nil, got %v", err)
	}
	if err := ValidateValue("boolean", json.RawMessage(`{}`), json.RawMessage(`false`)); err != nil {
		t.Errorf("boolean 别名: want nil, got %v", err)
	}
	err := ValidateValue("bool", json.RawMessage(`{}`), json.RawMessage(`"true"`))
	if err == nil {
		t.Fatal("字符串 'true' 应被拒")
	}
	if !strings.Contains(err.Error(), "值必须是布尔类型") {
		t.Errorf("err 应含 '值必须是布尔类型', got %q", err.Error())
	}
}

// --------- ValidateValue: select ---------

func TestValidateValue_Select(t *testing.T) {
	const opts = `{"options":[{"value":"a","label":"A"},{"value":"b","label":"B"}]}`

	// 标量形式
	if err := ValidateValue("select", json.RawMessage(opts), json.RawMessage(`"a"`)); err != nil {
		t.Errorf("合法标量: want nil, got %v", err)
	}
	err := ValidateValue("select", json.RawMessage(opts), json.RawMessage(`"z"`))
	if err == nil {
		t.Fatal("'z' 应被拒")
	}
	if !strings.Contains(err.Error(), "不在允许范围内") {
		t.Errorf("err 应含 '不在允许范围内', got %q", err.Error())
	}

	// 数组形式（多选）
	if err := ValidateValue("select", json.RawMessage(opts), json.RawMessage(`["a","b"]`)); err != nil {
		t.Errorf("合法数组: want nil, got %v", err)
	}
	err = ValidateValue("select", json.RawMessage(opts), json.RawMessage(`["a","z"]`))
	if err == nil {
		t.Fatal("数组含非法项 'z' 应被拒")
	}

	// options 缺失 → 放行任意值
	if err := ValidateValue("select", json.RawMessage(`{}`), json.RawMessage(`"whatever"`)); err != nil {
		t.Errorf("空 options: want nil, got %v", err)
	}

	// 标量但空串 → 不触发 optSet 校验（s==""）
	if err := ValidateValue("select", json.RawMessage(opts), json.RawMessage(``)); err != nil {
		t.Errorf("空 value: want nil, got %v", err)
	}
}

// --------- ValidateValue: default + invalid constraints ---------

func TestValidateValue_UnknownType(t *testing.T) {
	err := ValidateValue("unknown-type", json.RawMessage(`{}`), json.RawMessage(`0`))
	if err == nil {
		t.Fatal("unknown 类型应被拒")
	}
	if !strings.Contains(err.Error(), "不支持的字段类型") {
		t.Errorf("err 应含 '不支持的字段类型', got %q", err.Error())
	}
}

func TestValidateValue_InvalidConstraints(t *testing.T) {
	// constraints 非法 JSON → ErrBadRequest
	err := ValidateValue("int", json.RawMessage(`{not-json}`), json.RawMessage(`0`))
	if err == nil {
		t.Fatal("want err, got nil")
	}
	if err.Code != errcode.ErrBadRequest {
		t.Errorf("want code=%d, got code=%d", errcode.ErrBadRequest, err.Code)
	}
}

// --------- ValidateConstraintsSelf ---------

func TestValidateConstraintsSelf_IntMinGtMax(t *testing.T) {
	err := ValidateConstraintsSelf("int", json.RawMessage(`{"min":10,"max":5}`), errcode.ErrBadRequest)
	if err == nil {
		t.Fatal("want err, got nil")
	}
	if !strings.Contains(err.Error(), "min") || !strings.Contains(err.Error(), "max") {
		t.Errorf("err 应提及 min/max, got %q", err.Error())
	}
}

func TestValidateConstraintsSelf_IntValid(t *testing.T) {
	if err := ValidateConstraintsSelf("int", json.RawMessage(`{"min":0,"max":100}`), errcode.ErrBadRequest); err != nil {
		t.Errorf("want nil, got %v", err)
	}
	if err := ValidateConstraintsSelf("integer", json.RawMessage(`{}`), errcode.ErrBadRequest); err != nil {
		t.Errorf("无约束: want nil, got %v", err)
	}
}

func TestValidateConstraintsSelf_FloatPrecisionInvalid(t *testing.T) {
	err := ValidateConstraintsSelf("float", json.RawMessage(`{"precision":0}`), errcode.ErrBadRequest)
	if err == nil {
		t.Fatal("precision=0 应被拒")
	}
	if !strings.Contains(err.Error(), "precision 必须大于 0") {
		t.Errorf("err 应含 'precision 必须大于 0', got %q", err.Error())
	}

	// 负数同样
	err = ValidateConstraintsSelf("float", json.RawMessage(`{"precision":-1}`), errcode.ErrBadRequest)
	if err == nil {
		t.Fatal("precision=-1 应被拒")
	}
}

func TestValidateConstraintsSelf_FloatMinGtMaxTakesPrecedence(t *testing.T) {
	// min>max 优先于 precision 检查
	err := ValidateConstraintsSelf("float", json.RawMessage(`{"min":10,"max":1,"precision":0}`), errcode.ErrBadRequest)
	if err == nil {
		t.Fatal("want err, got nil")
	}
	if !strings.Contains(err.Error(), "min") {
		t.Errorf("先报 min>max, got %q", err.Error())
	}
}

func TestValidateConstraintsSelf_FloatValid(t *testing.T) {
	if err := ValidateConstraintsSelf("float", json.RawMessage(`{"min":0,"max":1,"precision":2}`), errcode.ErrBadRequest); err != nil {
		t.Errorf("want nil, got %v", err)
	}
}

func TestValidateConstraintsSelf_StringLengthRange(t *testing.T) {
	cases := []struct {
		name        string
		constraints string
		wantErr     bool
		wantMsg     string
	}{
		{"min>max", `{"minLength":10,"maxLength":5}`, true, "minLength"},
		{"minLength 负", `{"minLength":-1}`, true, "不能为负数"},
		{"maxLength 负", `{"maxLength":-1}`, true, "不能为负数"},
		{"合法", `{"minLength":1,"maxLength":100}`, false, ""},
		{"无约束", `{}`, false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateConstraintsSelf("string", json.RawMessage(c.constraints), errcode.ErrBadRequest)
			checkErr(t, err, c.wantErr, c.wantMsg)
		})
	}
}

func TestValidateConstraintsSelf_BoolAndReferenceAlwaysPass(t *testing.T) {
	if err := ValidateConstraintsSelf("bool", json.RawMessage(`{}`), errcode.ErrBadRequest); err != nil {
		t.Errorf("bool: want nil, got %v", err)
	}
	if err := ValidateConstraintsSelf("boolean", json.RawMessage(`{"anything":"goes"}`), errcode.ErrBadRequest); err != nil {
		t.Errorf("boolean 别名: want nil, got %v", err)
	}
	if err := ValidateConstraintsSelf("reference", json.RawMessage(`{"refs":[1,2]}`), errcode.ErrBadRequest); err != nil {
		t.Errorf("reference: want nil, got %v", err)
	}
}

func TestValidateConstraintsSelf_UnknownType(t *testing.T) {
	err := ValidateConstraintsSelf("weird-type", json.RawMessage(`{}`), errcode.ErrBadRequest)
	if err == nil {
		t.Fatal("want err, got nil")
	}
	if !strings.Contains(err.Error(), "不支持的字段类型") {
		t.Errorf("err 应含 '不支持的字段类型', got %q", err.Error())
	}
}

func TestValidateConstraintsSelf_InvalidConstraintsJSON(t *testing.T) {
	err := ValidateConstraintsSelf("int", json.RawMessage(`{not-json}`), 42025)
	if err == nil {
		t.Fatal("want err, got nil")
	}
	if err.Code != 42025 {
		t.Errorf("want code=42025, got code=%d", err.Code)
	}
	if !strings.Contains(err.Error(), "约束 JSON 解析失败") {
		t.Errorf("err 应含 '约束 JSON 解析失败', got %q", err.Error())
	}
}

func TestValidateConstraintsSelf_SelectOptions(t *testing.T) {
	cases := []struct {
		name        string
		constraints string
		wantErr     bool
		wantMsg     string
	}{
		{"合法 options", `{"options":[{"value":"a"},{"value":"b"}]}`, false, ""},
		// 外层合法 JSON、内层 options 不是数组（是字符串）→ 内部 Unmarshal 失败
		{"options 解析失败", `{"options":"not-array"}`, true, "options 解析失败"},
		// 空数组被视为非法（代码实际会进入 if 块后 len==0 判空）
		{"options 空数组", `{"options":[]}`, true, "select 字段 options 不能为空"},
		// 空 value 通过"缺 value 字段"触发（元素没有 value key，Unmarshal 后 RawMessage 为零值）
		{"缺 value 字段", `{"options":[{"label":"A"},{"value":"a"}]}`, true, "option.value 不能为空"},
		{"重复 value", `{"options":[{"value":"a"},{"value":"a"}]}`, true, "存在重复 value"},
		{"minSelect>maxSelect", `{"options":[{"value":"a"}],"minSelect":5,"maxSelect":1}`, true, "minSelect"},
		{"minSelect 负", `{"options":[{"value":"a"}],"minSelect":-1}`, true, "不能为负数"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateConstraintsSelf("select", json.RawMessage(c.constraints), errcode.ErrBadRequest)
			checkErr(t, err, c.wantErr, c.wantMsg)
		})
	}
}

func TestValidateConstraintsSelf_SelectNoOptionsKeyAllowed(t *testing.T) {
	// options key 缺失时（cm 中不存在 "options" key），跳过 options 校验
	if err := ValidateConstraintsSelf("select", json.RawMessage(`{}`), errcode.ErrBadRequest); err != nil {
		t.Errorf("无 options key: want nil, got %v", err)
	}
}

// --------- helpers ---------

func checkErr(t *testing.T, err *errcode.Error, wantErr bool, wantMsg string) {
	t.Helper()
	if wantErr {
		if err == nil {
			t.Fatal("want err, got nil")
		}
		if wantMsg != "" && !strings.Contains(err.Error(), wantMsg) {
			t.Errorf("err 应含 %q, got %q", wantMsg, err.Error())
		}
		return
	}
	if err != nil {
		t.Errorf("want nil, got %v", err)
	}
}
