package redis

import goredis "github.com/redis/go-redis/v9"

// Caches 聚合所有 Redis cache，新增模块加一行
type Caches struct {
	Field     *FieldCache
	Template  *TemplateCache
	EventType *EventTypeCache
	FsmConfig *FsmConfigCache
}

// NewCaches 一次性初始化所有 Redis cache
func NewCaches(rdb *goredis.Client) *Caches {
	return &Caches{
		Field:     NewFieldCache(rdb),
		Template:  NewTemplateCache(rdb),
		EventType: NewEventTypeCache(rdb),
		FsmConfig: NewFsmConfigCache(rdb),
	}
}
