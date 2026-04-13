package constraint

import (
	"encoding/json"

	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
)

// CheckConstraintTightened 检查约束是否被收紧
//
// 被引用的字段/扩展字段编辑时调用：约束只能放宽不能收紧。
// errCode 由调用方传入（字段模块用 40007，扩展字段模块用自己的错误码）。
// 返回 nil 表示未收紧（允许保存），返回 *errcode.Error 表示收紧（拒绝保存）。
//
// 覆盖规则：
//   - integer/float: min 只能调小，max 只能调大，float precision 只能调大
//   - string: minLength 只能调小，maxLength 只能调大，pattern 不能新增/变更（可删除=放宽）
//   - select: options 只能新增不能删除，minSelect 只能调小，maxSelect 只能调大
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
		// float 专属：precision 只能变大，变小会截断已存数据
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
		// pattern 变化判定：
		//   old=""   new=""    → 允许（未变）
		//   old=""   new="^x$" → 拒绝（新增 pattern，旧数据可能不匹配）
		//   old=P    new=P     → 允许（未变）
		//   old=P    new=Q     → 拒绝（pattern 变化）
		//   old=P    new=""    → 允许（移除 pattern = 放宽）
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
		// minSelect 只能变小，maxSelect 只能变大
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
