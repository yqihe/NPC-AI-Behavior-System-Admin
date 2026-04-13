package errcode

import "errors"

// Store 层哨兵错误
//
// store 层返回，service 层用 errors.Is() 捕获后翻译为业务错误码。
var (
	ErrNotFound        = errors.New("record not found")
	ErrVersionConflict = errors.New("version conflict")
	ErrDuplicate       = errors.New("duplicate entry")
)
