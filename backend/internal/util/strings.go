package util

import (
	"regexp"
	"strings"
)

// IdentPattern 通用标识符正则：a-z 开头，仅 a-z0-9_
//
// 所有配置类型（字段/模板/事件类型/扩展字段 Schema）的 name/field_name 共用。
var IdentPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// EscapeLike 转义 LIKE 通配符，防止用户输入 % 或 _ 匹配所有记录
//
// 所有 store 的模糊搜索共用（mysql-red-lines: 禁止 LIKE 不转义）。
func EscapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}
