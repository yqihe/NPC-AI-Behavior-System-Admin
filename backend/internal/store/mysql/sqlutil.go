package mysql

// sqlutil.go — store/mysql 层 SQL 辅助
//
// 只放 MySQL 层通用的工具函数，无业务语义。

import (
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
)

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

// ============================================================
// MySQL 错误识别
// ============================================================

// Is1062 判断 err 是否为 MySQL duplicate entry (1062)。
//
// 穿透 fmt.Errorf wrap，store 层 CREATE 出现唯一键冲突时使用。
func Is1062(err error) bool {
	var me *mysql.MySQLError
	return errors.As(err, &me) && me.Number == 1062
}
