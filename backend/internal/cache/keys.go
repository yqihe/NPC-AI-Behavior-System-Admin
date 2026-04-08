package cache

import "fmt"

// Redis key 统一管理
// 所有 key 都通过函数生成，不在业务代码中拼字符串

const (
	prefixDict        = "dict:"          // 字典缓存
	prefixFieldList   = "fields:list:"   // 字段列表分页缓存
	prefixFieldDetail = "fields:detail:" // 字段单条缓存
	prefixFieldLock   = "fields:lock:"   // 字段分布式锁

	// FieldListVersionKey 列表缓存版本号
	// 写操作 INCR 此 key，列表缓存 key 带版本号，旧缓存自然过期，无需 SCAN
	FieldListVersionKey = "fields:list:version"
)

// DictKey 字典缓存 key: dict:{group}
func DictKey(group string) string {
	return prefixDict + group
}

// FieldListKey 字段列表缓存 key（带版本号，版本变更后旧 key 自然过期）
func FieldListKey(version int64, typ, category, label string, page, pageSize int) string {
	return fmt.Sprintf("%sv%d:%s:%s:%s:%d:%d", prefixFieldList, version, typ, category, label, page, pageSize)
}

// FieldDetailKey 字段详情缓存 key: fields:detail:{name}
func FieldDetailKey(name string) string {
	return prefixFieldDetail + name
}

// FieldLockKey 字段分布式锁 key: fields:lock:{name}
func FieldLockKey(name string) string {
	return prefixFieldLock + name
}

