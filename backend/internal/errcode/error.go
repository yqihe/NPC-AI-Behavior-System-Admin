package errcode

import "fmt"

// Error 业务错误（携带错误码）
type Error struct {
	Code    int
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

// New 创建业务错误，使用默认消息
func New(code int) *Error {
	return &Error{Code: code, Message: Msg(code)}
}

// Newf 创建业务错误，自定义消息
func Newf(code int, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}
