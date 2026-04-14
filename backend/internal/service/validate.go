package service

// validate.go — service 层校验辅助
//
// 分页规范化、字段值校验、约束自洽校验。
// 业务规则（如"约束只能放宽"）不放这里，见 constraint_check.go。

import (
	"encoding/json"
	"unicode/utf8"

	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
)

// ============================================================
// 分页规范化
// ============================================================

// NormalizePagination 分页参数校正（所有 List 方法共享）
func NormalizePagination(page, pageSize *int, defaultPage, defaultPageSize, maxPageSize int) {
	if *page < 1 {
		*page = defaultPage
	}
	if *pageSize < 1 {
		*pageSize = defaultPageSize
	}
	if *pageSize > maxPageSize {
		*pageSize = maxPageSize
	}
}

// ============================================================
// ValidateValue 单值校验
// ============================================================

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

// ============================================================
// ValidateConstraintsSelf 约束自洽校验
// ============================================================

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
