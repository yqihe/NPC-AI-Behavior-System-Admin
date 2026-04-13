package setup

import (
	"context"
	"fmt"

	"github.com/yqihe/npc-ai-admin/backend/internal/cache"
)

// MemCaches 聚合所有内存缓存
type MemCaches struct {
	Dict            *cache.DictCache
	EventTypeSchema *cache.EventTypeSchemaCache
}

// NewMemCaches 一次性初始化所有内存缓存并从 MySQL 加载
func NewMemCaches(ctx context.Context, st *Stores) (*MemCaches, error) {
	dictCache := cache.NewDictCache(st.Dict)
	if err := dictCache.Load(ctx); err != nil {
		return nil, fmt.Errorf("加载字典缓存: %w", err)
	}

	etSchemaCache := cache.NewEventTypeSchemaCache(st.EventTypeSchema)
	if err := etSchemaCache.Load(ctx); err != nil {
		return nil, fmt.Errorf("加载事件类型Schema缓存: %w", err)
	}

	return &MemCaches{
		Dict:            dictCache,
		EventTypeSchema: etSchemaCache,
	}, nil
}
