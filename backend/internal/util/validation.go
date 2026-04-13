package util

import "github.com/yqihe/npc-ai-admin/backend/internal/errcode"

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

// SuccessMsg 构造 *string 成功消息（Update/ToggleEnabled 返回值用）
func SuccessMsg(msg string) *string {
	return &msg
}
