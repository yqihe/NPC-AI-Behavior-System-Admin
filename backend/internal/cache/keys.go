package cache

import "fmt"

// Redis key 统一管理
// 所有 key 都通过函数生成，不在业务代码中拼字符串

const (
	prefixDict        = "dict:"         // 字典缓存
	prefixFieldList   = "fields:list:"  // 字段列表分页缓存
	prefixFieldDetail = "fields:detail:" // 字段单条缓存
	prefixFieldLock   = "fields:lock:"  // 字段分布式锁
)

// DictKey 字典缓存 key: dict:{group}
func DictKey(group string) string {
	return prefixDict + group
}

// FieldListKey 字段列表缓存 key
func FieldListKey(deleted int, typ, category, label string, page, pageSize int) string {
	return fmt.Sprintf("%s%d:%s:%s:%s:%d:%d", prefixFieldList, deleted, typ, category, label, page, pageSize)
}

// FieldDetailKey 字段详情缓存 key: fields:detail:{name}
func FieldDetailKey(name string) string {
	return prefixFieldDetail + name
}

// FieldLockKey 字段分布式锁 key: fields:lock:{name}
func FieldLockKey(name string) string {
	return prefixFieldLock + name
}

// FieldListPattern 字段列表缓存模糊匹配（清除用）
func FieldListPattern() string {
	return prefixFieldList + "*"
}
