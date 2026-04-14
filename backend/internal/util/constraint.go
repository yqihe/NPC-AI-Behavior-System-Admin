package util

// 字段/扩展字段约束的公共校验工具。
//
// 被字段管理（CheckConstraintTightened）和事件类型扩展字段（ValidateValue/ValidateConstraintsSelf）共用。
// 此文件无状态，不持有任何 store/cache。

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
)

// ──────────────────────────────────────────────
// JSON 解析辅助
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
	case "int", "integer":
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
		s = ""
	}
	runeLen := utf8.RuneCountInString(s)
	if minLen, ok := GetFloat(cm["minLength"]); ok && float64(runeLen) < minLen {
		return errcode.Newf(errcode.ErrBadRequest, "字符串长度 %d 小于最小长度 %v", runeLen, minLen)
	}
	if maxLen, ok := GetFloat(cm["maxLength"]); ok && float64(runeLen) > maxLen {
		return errcode.Newf(errcode.ErrBadRequest, "字符串长度 %d 大于最大长度 %v", runeLen, maxLen)
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
	options := ParseSelectOptions(cm["options"])
	if len(options) == 0 {
		return nil
	}
	optSet := make(map[string]bool, len(options))
	for _, o := range options {
		optSet[o] = true
	}

	var arr []string
	if err := json.Unmarshal(value, &arr); err == nil {
		for _, v := range arr {
			if !optSet[v] {
				return errcode.Newf(errcode.ErrBadRequest, "选项 '%s' 不在允许范围内", v)
			}
		}
		return nil
	}

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
// errCode 由调用方传入（字段模块用 40000，扩展字段模块用 42025）。
func ValidateConstraintsSelf(fieldType string, constraints json.RawMessage, errCode int) *errcode.Error {
	cm, err := ParseConstraintsMap(constraints)
	if err != nil {
		return errcode.Newf(errCode, "约束 JSON 解析失败")
	}

	switch fieldType {
	case "int", "integer":
		return selfCheckMinMax(cm, errCode)
	case "float":
		if e := selfCheckMinMax(cm, errCode); e != nil {
			return e
		}
		if prec, ok := GetFloat(cm["precision"]); ok && prec <= 0 {
			return errcode.Newf(errCode, "precision 必须大于 0")
		}
		return nil
	case "string":
		return selfCheckLengthRange(cm, errCode)
	case "bool", "boolean":
		return nil
	case "select":
		return selfCheckSelect(cm, errCode)
	case "reference":
		// reference 字段的约束由专用逻辑校验（refs 非空、目标存在、非嵌套）
		return nil
	default:
		return errcode.Newf(errCode, "不支持的字段类型: %s", fieldType)
	}
}

func selfCheckMinMax(cm map[string]json.RawMessage, errCode int) *errcode.Error {
	min, hasMin := GetFloat(cm["min"])
	max, hasMax := GetFloat(cm["max"])
	if hasMin && hasMax && min > max {
		return errcode.Newf(errCode, "min (%v) 不能大于 max (%v)", min, max)
	}
	return nil
}

func selfCheckLengthRange(cm map[string]json.RawMessage, errCode int) *errcode.Error {
	minLen, hasMin := GetFloat(cm["minLength"])
	maxLen, hasMax := GetFloat(cm["maxLength"])
	if hasMin && hasMax && minLen > maxLen {
		return errcode.Newf(errCode, "minLength (%v) 不能大于 maxLength (%v)", minLen, maxLen)
	}
	if hasMin && minLen < 0 {
		return errcode.Newf(errCode, "minLength 不能为负数")
	}
	if hasMax && maxLen < 0 {
		return errcode.Newf(errCode, "maxLength 不能为负数")
	}
	return nil
}

func selfCheckSelect(cm map[string]json.RawMessage, errCode int) *errcode.Error {
	// 校验 options：必须存在且非空，且 value 不重复
	if rawOpts, ok := cm["options"]; ok && len(rawOpts) > 0 {
		var options []struct {
			Value json.RawMessage `json:"value"`
		}
		if err := json.Unmarshal(rawOpts, &options); err != nil {
			return errcode.Newf(errCode, "options 解析失败")
		}
		if len(options) == 0 {
			return errcode.Newf(errCode, "select 字段 options 不能为空")
		}
		seen := make(map[string]bool, len(options))
		for _, o := range options {
			key := string(o.Value)
			if key == "" {
				return errcode.Newf(errCode, "select option.value 不能为空")
			}
			if seen[key] {
				return errcode.Newf(errCode, "select options 存在重复 value: %s", key)
			}
			seen[key] = true
		}
	}

	minSel, hasMin := GetFloat(cm["minSelect"])
	maxSel, hasMax := GetFloat(cm["maxSelect"])
	if hasMin && hasMax && minSel > maxSel {
		return errcode.Newf(errCode, "minSelect (%v) 不能大于 maxSelect (%v)", minSel, maxSel)
	}
	if hasMin && minSel < 0 {
		return errcode.Newf(errCode, "minSelect 不能为负数")
	}
	return nil
}

// ──────────────────────────────────────────────
// CheckConstraintTightened 约束收紧检查
// ──────────────────────────────────────────────

// CheckConstraintTightened 检查约束是否被收紧
//
// 被引用的字段/扩展字段编辑时调用：约束只能放宽不能收紧。
// errCode 由调用方传入（字段模块用 40007，扩展字段模块用自己的错误码）。
// 返回 nil 表示未收紧（允许保存），返回 *errcode.Error 表示收紧（拒绝保存）。
func CheckConstraintTightened(fieldType string, oldConstraints, newConstraints json.RawMessage, errCode int) *errcode.Error {
	oldMap, err := ParseConstraintsMap(oldConstraints)
	if err != nil {
		return nil
	}
	newMap, err := ParseConstraintsMap(newConstraints)
	if err != nil {
		return nil
	}

	switch fieldType {
	case "integer", "int", "float":
		if oldMin, ok := GetFloat(oldMap["min"]); ok {
			if newMin, ok2 := GetFloat(newMap["min"]); ok2 && newMin > oldMin {
				return errcode.Newf(errCode, "最小值从 %v 收紧为 %v，请先移除引用", oldMin, newMin)
			}
		}
		if oldMax, ok := GetFloat(oldMap["max"]); ok {
			if newMax, ok2 := GetFloat(newMap["max"]); ok2 && newMax < oldMax {
				return errcode.Newf(errCode, "最大值从 %v 收紧为 %v，请先移除引用", oldMax, newMax)
			}
		}
		if fieldType == "float" {
			if oldPrec, ok := GetFloat(oldMap["precision"]); ok {
				if newPrec, ok2 := GetFloat(newMap["precision"]); ok2 && newPrec < oldPrec {
					return errcode.Newf(errCode, "precision 从 %v 降低为 %v，请先移除引用", oldPrec, newPrec)
				}
			}
		}

	case "string":
		if oldMinLen, ok := GetFloat(oldMap["minLength"]); ok {
			if newMinLen, ok2 := GetFloat(newMap["minLength"]); ok2 && newMinLen > oldMinLen {
				return errcode.Newf(errCode, "最小长度从 %v 收紧为 %v，请先移除引用", oldMinLen, newMinLen)
			}
		}
		if oldMaxLen, ok := GetFloat(oldMap["maxLength"]); ok {
			if newMaxLen, ok2 := GetFloat(newMap["maxLength"]); ok2 && newMaxLen < oldMaxLen {
				return errcode.Newf(errCode, "最大长度从 %v 收紧为 %v，请先移除引用", oldMaxLen, newMaxLen)
			}
		}
		oldPat := GetString(oldMap["pattern"])
		newPat := GetString(newMap["pattern"])
		if newPat != "" && newPat != oldPat {
			return errcode.Newf(errCode, "pattern 从 %q 变更为 %q，可能使已有数据失效，请先移除引用", oldPat, newPat)
		}

	case "select":
		oldOptions := ParseSelectOptions(oldMap["options"])
		newOptions := ParseSelectOptions(newMap["options"])
		if len(oldOptions) > 0 {
			newSet := make(map[string]bool, len(newOptions))
			for _, o := range newOptions {
				newSet[o] = true
			}
			for _, o := range oldOptions {
				if !newSet[o] {
					return errcode.Newf(errCode, "选项 '%s' 被删除，请先移除引用", o)
				}
			}
		}
		if oldMinSel, ok := GetFloat(oldMap["minSelect"]); ok {
			if newMinSel, ok2 := GetFloat(newMap["minSelect"]); ok2 && newMinSel > oldMinSel {
				return errcode.Newf(errCode, "minSelect 从 %v 收紧为 %v，请先移除引用", oldMinSel, newMinSel)
			}
		}
		if oldMaxSel, ok := GetFloat(oldMap["maxSelect"]); ok {
			if newMaxSel, ok2 := GetFloat(newMap["maxSelect"]); ok2 && newMaxSel < oldMaxSel {
				return errcode.Newf(errCode, "maxSelect 从 %v 收紧为 %v，请先移除引用", oldMaxSel, newMaxSel)
			}
		}
	}

	return nil
}
