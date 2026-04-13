package cache

import (
	"context"
	"fmt"

	storemysql "github.com/yqihe/npc-ai-admin/backend/internal/store/mysql"
)

// MemCaches 聚合所有内存缓存，新增模块加一行
type MemCaches struct {
	Dict            *DictCache
	EventTypeSchema *EventTypeSchemaCache
}

// NewMemCaches 一次性初始化所有内存缓存并从 MySQL 加载
func NewMemCaches(ctx context.Context, stores *storemysql.Stores) (*MemCaches, error) {
	dictCache := NewDictCache(stores.Dict)
	if err := dictCache.Load(ctx); err != nil {
		return nil, fmt.Errorf("加载字典缓存: %w", err)
	}

	etSchemaCache := NewEventTypeSchemaCache(stores.EventTypeSchema)
	if err := etSchemaCache.Load(ctx); err != nil {
		return nil, fmt.Errorf("加载事件类型Schema缓存: %w", err)
	}

	return &MemCaches{
		Dict:            dictCache,
		EventTypeSchema: etSchemaCache,
	}, nil
}
