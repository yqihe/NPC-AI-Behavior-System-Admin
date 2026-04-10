package redis

import "fmt"

// Redis key 统一管理
// 所有 key 都通过函数生成，不在业务代码中拼字符串

const (
	prefixDict        = "dict:"          // 字典缓存
	prefixFieldList   = "fields:list:"   // 字段列表分页缓存
	prefixFieldDetail = "fields:detail:" // 字段单条缓存
	prefixFieldLock   = "fields:lock:"   // 字段分布式锁

	prefixTemplateList   = "templates:list:"   // 模板列表分页缓存
	prefixTemplateDetail = "templates:detail:" // 模板单条缓存（仅 templates 裸行，不含字段补全）
	prefixTemplateLock   = "templates:lock:"   // 模板分布式锁

	// fieldListVersionKey 字段列表缓存版本号
	// 写操作 INCR 此 key，列表缓存 key 带版本号，旧缓存自然过期，无需 SCAN
	fieldListVersionKey = "fields:list:version"

	// templateListVersionKey 模板列表缓存版本号（同字段方案）
	templateListVersionKey = "templates:list:version"
)

// DictKey 字典缓存 key: dict:{group}
func DictKey(group string) string {
	return prefixDict + group
}

// FieldListKey 字段列表缓存 key（带版本号，版本变更后旧 key 自然过期）
// enabled: nil=不筛选("*"), true="1", false="0"
func FieldListKey(version int64, typ, category, label string, enabled *bool, page, pageSize int) string {
	e := "*"
	if enabled != nil {
		if *enabled {
			e = "1"
		} else {
			e = "0"
		}
	}
	return fmt.Sprintf("%sv%d:%s:%s:%s:%s:%d:%d", prefixFieldList, version, typ, category, label, e, page, pageSize)
}

// FieldDetailKey 字段详情缓存 key: fields:detail:{id}
func FieldDetailKey(id int64) string {
	return fmt.Sprintf("%s%d", prefixFieldDetail, id)
}

// FieldLockKey 字段分布式锁 key: fields:lock:{id}
func FieldLockKey(id int64) string {
	return fmt.Sprintf("%s%d", prefixFieldLock, id)
}

// TemplateListKey 模板列表缓存 key（带版本号，版本变更后旧 key 自然过期）
// enabled: nil=不筛选("*"), true="1", false="0"
func TemplateListKey(version int64, label string, enabled *bool, page, pageSize int) string {
	e := "*"
	if enabled != nil {
		if *enabled {
			e = "1"
		} else {
			e = "0"
		}
	}
	return fmt.Sprintf("%sv%d:%s:%s:%d:%d", prefixTemplateList, version, label, e, page, pageSize)
}

// TemplateDetailKey 模板详情缓存 key: templates:detail:{id}
func TemplateDetailKey(id int64) string {
	return fmt.Sprintf("%s%d", prefixTemplateDetail, id)
}

// TemplateLockKey 模板分布式锁 key: templates:lock:{id}
func TemplateLockKey(id int64) string {
	return fmt.Sprintf("%s%d", prefixTemplateLock, id)
}
