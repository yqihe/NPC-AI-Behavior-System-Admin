// Package constraint 提供字段约束的公共校验工具。
//
// 被字段管理（checkConstraintTightened）和事件类型扩展字段（validateExtensions）共用。
// 此包无状态，不持有任何 store/cache。
package constraint

import (
	"encoding/json"
	"fmt"

	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
)

// ──────────────────────────────────────────────
// JSON 解析辅助（从 field.go 抽出）
// ──────────────────────────────────────────────

// ParseConstraintsMap 解析 constraints JSON 为 key→RawMessage map
func ParseConstraintsMap(raw json.RawMessage) (map[string]json.RawMessage, error) {
	if len(raw) == 0 {
		return make(map[string]json.RawMessage), nil
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("unmarshal constraints: %w", err)
	}
	return m, nil
}

// GetFloat 从 json.RawMessage 提取 float64
func GetFloat(raw json.RawMessage) (float64, bool) {
	var v float64
	if err := json.Unmarshal(raw, &v); err != nil {
		return 0, false
	}
	return v, true
}

// GetString 从 json.RawMessage 提取字符串，失败返回空串
func GetString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

// GetBool 从 json.RawMessage 提取 bool
func GetBool(raw json.RawMessage) (bool, bool) {
	var v bool
	if err := json.Unmarshal(raw, &v); err != nil {
		return false, false
	}
	return v, true
}

// ParseSelectOptions 解析 select 类型的 options 数组，返回 value 列表
func ParseSelectOptions(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var options []struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(raw, &options); err != nil {
		return nil
	}
	values := make([]string, 0, len(options))
	for _, o := range options {
		values = append(values, o.Value)
	}
	return values
}

// ──────────────────────────────────────────────
// ValidateValue 校验单个值是否符合 (fieldType, constraints)
// ──────────────────────────────────────────────

// ValidateValue 校验 value 是否符合 fieldType + constraints 约束
//
// 返回 nil 表示通过，返回 *errcode.Error 表示违反约束。
// 使用方：事件类型扩展字段值校验、Schema default_value 校验。
func ValidateValue(fieldType string, constraints json.RawMessage, value json.RawMessage) *errcode.Error {
	cm, err := ParseConstraintsMap(constraints)
	if err != nil {
		return errcode.New(errcode.ErrBadRequest)
	}

	switch fieldType {
	case "int":
		return validateInt(cm, value)
	case "integer": // 字段管理用 "integer"，事件类型扩展用 "int"
		return validateInt(cm, value)
	case "float":
		return validateFloat(cm, value)
	case "string":
		return validateString(cm, value)
	case "bool":
		return validateBool(value)
	case "select":
		return validateSelect(cm, value)
	default:
		return errcode.Newf(errcode.ErrBadRequest, "不支持的字段类型: %s", fieldType)
	}
}

func validateInt(cm map[string]json.RawMessage, value json.RawMessage) *errcode.Error {
	v, ok := GetFloat(value)
	if !ok {
		return errcode.Newf(errcode.ErrBadRequest, "值必须是数字")
	}
	if min, ok := GetFloat(cm["min"]); ok && v < min {
		return errcode.Newf(errcode.ErrBadRequest, "值 %v 小于最小值 %v", v, min)
	}
	if max, ok := GetFloat(cm["max"]); ok && v > max {
		return errcode.Newf(errcode.ErrBadRequest, "值 %v 大于最大值 %v", v, max)
	}
	return nil
}

func validateFloat(cm map[string]json.RawMessage, value json.RawMessage) *errcode.Error {
	v, ok := GetFloat(value)
	if !ok {
		return errcode.Newf(errcode.ErrBadRequest, "值必须是数字")
	}
	if min, ok := GetFloat(cm["min"]); ok && v < min {
		return errcode.Newf(errcode.ErrBadRequest, "值 %v 小于最小值 %v", v, min)
	}
	if max, ok := GetFloat(cm["max"]); ok && v > max {
		return errcode.Newf(errcode.ErrBadRequest, "值 %v 大于最大值 %v", v, max)
	}
	return nil
}

func validateString(cm map[string]json.RawMessage, value json.RawMessage) *errcode.Error {
	s := GetString(value)
	if len(value) == 0 || (len(value) == 4 && string(value) == "null") {
		// null / 缺失视为空串
		s = ""
	}
	if minLen, ok := GetFloat(cm["minLength"]); ok && float64(len(s)) < minLen {
		return errcode.Newf(errcode.ErrBadRequest, "字符串长度 %d 小于最小长度 %v", len(s), minLen)
	}
	if maxLen, ok := GetFloat(cm["maxLength"]); ok && float64(len(s)) > maxLen {
		return errcode.Newf(errcode.ErrBadRequest, "字符串长度 %d 大于最大长度 %v", len(s), maxLen)
	}
	return nil
}

func validateBool(value json.RawMessage) *errcode.Error {
	_, ok := GetBool(value)
	if !ok {
		return errcode.Newf(errcode.ErrBadRequest, "值必须是布尔类型")
	}
	return nil
}

func validateSelect(cm map[string]json.RawMessage, value json.RawMessage) *errcode.Error {
	// select 值可以是单个字符串或字符串数组
	options := ParseSelectOptions(cm["options"])
	if len(options) == 0 {
		return nil // 无选项约束
	}
	optSet := make(map[string]bool, len(options))
	for _, o := range options {
		optSet[o] = true
	}

	// 尝试解析为字符串数组
	var arr []string
	if err := json.Unmarshal(value, &arr); err == nil {
		for _, v := range arr {
			if !optSet[v] {
				return errcode.Newf(errcode.ErrBadRequest, "选项 '%s' 不在允许范围内", v)
			}
		}
		return nil
	}

	// 尝试解析为单个字符串
	s := GetString(value)
	if s != "" && !optSet[s] {
		return errcode.Newf(errcode.ErrBadRequest, "选项 '%s' 不在允许范围内", s)
	}

	return nil
}

// ──────────────────────────────────────────────
// ValidateConstraintsSelf 校验约束自身是否自洽
// ──────────────────────────────────────────────

// ValidateConstraintsSelf 校验 constraints 内部是否自洽
//
// 例如 int 的 min <= max，select 的 minSelect <= maxSelect。
// 使用方：Schema 管理页创建/编辑扩展字段定义时调用。
func ValidateConstraintsSelf(fieldType string, constraints json.RawMessage) *errcode.Error {
	cm, err := ParseConstraintsMap(constraints)
	if err != nil {
		return errcode.Newf(errcode.ErrExtSchemaConstraintsInvalid, "约束 JSON 解析失败")
	}

	switch fieldType {
	case "int", "integer":
		return selfCheckMinMax(cm)
	case "float":
		if e := selfCheckMinMax(cm); e != nil {
			return e
		}
		if prec, ok := GetFloat(cm["precision"]); ok && prec < 0 {
			return errcode.Newf(errcode.ErrExtSchemaConstraintsInvalid, "precision 不能为负数")
		}
		return nil
	case "string":
		return selfCheckLengthRange(cm)
	case "bool":
		return nil
	case "select":
		return selfCheckSelect(cm)
	default:
		return errcode.Newf(errcode.ErrExtSchemaConstraintsInvalid, "不支持的字段类型: %s", fieldType)
	}
}

func selfCheckMinMax(cm map[string]json.RawMessage) *errcode.Error {
	min, hasMin := GetFloat(cm["min"])
	max, hasMax := GetFloat(cm["max"])
	if hasMin && hasMax && min > max {
		return errcode.Newf(errcode.ErrExtSchemaConstraintsInvalid, "min (%v) 不能大于 max (%v)", min, max)
	}
	return nil
}

func selfCheckLengthRange(cm map[string]json.RawMessage) *errcode.Error {
	minLen, hasMin := GetFloat(cm["minLength"])
	maxLen, hasMax := GetFloat(cm["maxLength"])
	if hasMin && hasMax && minLen > maxLen {
		return errcode.Newf(errcode.ErrExtSchemaConstraintsInvalid, "minLength (%v) 不能大于 maxLength (%v)", minLen, maxLen)
	}
	if hasMin && minLen < 0 {
		return errcode.Newf(errcode.ErrExtSchemaConstraintsInvalid, "minLength 不能为负数")
	}
	return nil
}

func selfCheckSelect(cm map[string]json.RawMessage) *errcode.Error {
	minSel, hasMin := GetFloat(cm["minSelect"])
	maxSel, hasMax := GetFloat(cm["maxSelect"])
	if hasMin && hasMax && minSel > maxSel {
		return errcode.Newf(errcode.ErrExtSchemaConstraintsInvalid, "minSelect (%v) 不能大于 maxSelect (%v)", minSel, maxSel)
	}
	if hasMin && minSel < 0 {
		return errcode.Newf(errcode.ErrExtSchemaConstraintsInvalid, "minSelect 不能为负数")
	}
	return nil
}
