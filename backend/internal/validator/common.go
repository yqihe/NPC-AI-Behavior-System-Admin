package validator

import (
	"fmt"
	"strings"
)

// ValidationError 包含一组校验失败的中文描述。
// 实现 error 接口，service 层可用 errors.As 提取。
type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("校验失败: %s", strings.Join(e.Errors, "; "))
}

// validationBuilder 用于收集校验错误。
type validationBuilder struct {
	errors []string
}

func (b *validationBuilder) add(msg string) {
	b.errors = append(b.errors, msg)
}

func (b *validationBuilder) addf(format string, args ...any) {
	b.errors = append(b.errors, fmt.Sprintf(format, args...))
}

// result 返回 nil（无错误）或 *ValidationError。
func (b *validationBuilder) result() error {
	if len(b.errors) == 0 {
		return nil
	}
	return &ValidationError{Errors: b.errors}
}
