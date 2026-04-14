package shared

// validate.go — handler 层请求校验辅助
//
// 只放 handler 层通用的校验和响应函数。
// 业务校验（如"启用中禁止编辑"）留在各 handler 方法中，不放这里。

import (
	"regexp"
	"unicode/utf8"

	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
)

// ============================================================
// ID / Version / 必填校验
// ============================================================

// CheckID 校验 ID 合法性
func CheckID(id int64) *errcode.Error {
	if id <= 0 {
		return errcode.Newf(errcode.ErrBadRequest, "ID 不合法")
	}
	return nil
}

// CheckVersion 校验乐观锁版本号
func CheckVersion(version int) *errcode.Error {
	if version <= 0 {
		return errcode.Newf(errcode.ErrBadRequest, "版本号不合法")
	}
	return nil
}

// CheckRequired 校验必填字段非空
func CheckRequired(value, fieldName string) *errcode.Error {
	if value == "" {
		return errcode.Newf(errcode.ErrBadRequest, "%s 不能为空", fieldName)
	}
	return nil
}

// ============================================================
// 标识符正则
// ============================================================

// IdentPattern 通用标识符正则：a-z 开头，仅 a-z0-9_
//
// 所有配置类型（字段/模板/事件类型/扩展字段 Schema/状态机）的 name/field_name 共用。
var IdentPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// ============================================================
// 名称格式校验（标识符类）
// ============================================================

// CheckName 校验标识符名称（小写+数字+下划线，a-z 开头，有长度上限）
//
// 所有配置类型的 name/field_name 共用。subject 用于错误消息（"字段标识"/"模板标识"/...）。
// errCode 由调用方传入（各模块独立：ErrFieldNameInvalid / ErrTemplateNameInvalid 等）。
func CheckName(name string, maxLen int, errCode int, subject string) *errcode.Error {
	if name == "" {
		return errcode.Newf(errCode, "%s不能为空", subject)
	}
	if !IdentPattern.MatchString(name) {
		return errcode.New(errCode)
	}
	if len(name) > maxLen {
		return errcode.Newf(errCode, "%s长度不能超过 %d 个字符", subject, maxLen)
	}
	return nil
}

// ============================================================
// 标签格式校验（中文展示名/Label）
// ============================================================

// CheckLabel 校验中文标签/展示名（非空 + UTF-8 字符数上限）
//
// 所有配置类型的 label / display_name 共用。subject 是字段描述（"中文标签"/"中文名称"/"扩展字段中文名"）。
// 统一返回 ErrBadRequest（所有模块当前都用此码）。
func CheckLabel(label string, maxLen int, subject string) *errcode.Error {
	if label == "" {
		return errcode.Newf(errcode.ErrBadRequest, "%s不能为空", subject)
	}
	if utf8.RuneCountInString(label) > maxLen {
		return errcode.Newf(errcode.ErrBadRequest, "%s长度不能超过 %d 个字符", subject, maxLen)
	}
	return nil
}

// ============================================================
// 响应辅助
// ============================================================

// SuccessMsg 构造 *string 成功消息（Update/ToggleEnabled 返回值用）
func SuccessMsg(msg string) *string {
	return &msg
}
