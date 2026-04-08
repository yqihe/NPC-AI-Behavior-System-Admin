package mysql

import "errors"

var (
	// ErrNotFound 记录不存在
	ErrNotFound = errors.New("record not found")

	// ErrVersionConflict 乐观锁版本冲突
	ErrVersionConflict = errors.New("version conflict")

	// ErrDuplicate 唯一键冲突
	ErrDuplicate = errors.New("duplicate entry")
)
