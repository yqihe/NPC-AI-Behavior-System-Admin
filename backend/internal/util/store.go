package util

// store 层通用工具：SQL LIKE 转义等。

import "strings"

// ============================================================
// SQL LIKE 转义
// ============================================================

// EscapeLike 转义 LIKE 通配符，防止用户输入 % 或 _ 匹配所有记录
//
// 所有 store 的模糊搜索共用（mysql-red-lines: 禁止 LIKE 不转义）。
func EscapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}
