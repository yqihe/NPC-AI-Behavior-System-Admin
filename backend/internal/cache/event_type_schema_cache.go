package cache

import (
	"context"
	"log/slog"
	"sync"

	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	"github.com/yqihe/npc-ai-admin/backend/internal/store/mysql"
)

// EventTypeSchemaCache 事件类型扩展字段 Schema 内存缓存
//
// 和 DictCache 同构：启动时全量加载，写后 Reload，运行时只读内存。
// 数据量极小（< 100 条），不走 Redis。
type EventTypeSchemaCache struct {
	mu      sync.RWMutex
	store   *mysql.EventTypeSchemaStore
	schemas []model.EventTypeSchemaLite // 启用的，按 sort_order 排好序
	byName  map[string]*model.EventTypeSchemaLite
}

// NewEventTypeSchemaCache 创建 EventTypeSchemaCache
func NewEventTypeSchemaCache(store *mysql.EventTypeSchemaStore) *EventTypeSchemaCache {
	return &EventTypeSchemaCache{
		store:   store,
		schemas: make([]model.EventTypeSchemaLite, 0),
		byName:  make(map[string]*model.EventTypeSchemaLite),
	}
}

// Load 从 MySQL 全量加载启用的扩展字段定义（启动时调用）
func (c *EventTypeSchemaCache) Load(ctx context.Context) error {
	items, err := c.store.ListEnabled(ctx)
	if err != nil {
		return err
	}

	byName := make(map[string]*model.EventTypeSchemaLite, len(items))
	for i := range items {
		byName[items[i].FieldName] = &items[i]
	}

	c.mu.Lock()
	c.schemas = items
	c.byName = byName
	c.mu.Unlock()

	slog.Info("cache.事件类型扩展字段Schema加载完成", "count", len(items))
	return nil
}

// Reload 写操作后同步调用，重新全量加载
func (c *EventTypeSchemaCache) Reload(ctx context.Context) error {
	return c.Load(ctx)
}

// ListEnabled 返回所有启用的扩展字段定义（已按 sort_order 排序）
//
// 返回副本防外部修改。
func (c *EventTypeSchemaCache) ListEnabled() []model.EventTypeSchemaLite {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]model.EventTypeSchemaLite, len(c.schemas))
	copy(result, c.schemas)
	return result
}

// SetSchemasForTest 测试用：直接注入 schemas，绕过 Load 的 DB 依赖。
// 生产路径永远不应调用此方法。
func (c *EventTypeSchemaCache) SetSchemasForTest(items []model.EventTypeSchemaLite) {
	byName := make(map[string]*model.EventTypeSchemaLite, len(items))
	for i := range items {
		byName[items[i].FieldName] = &items[i]
	}
	c.mu.Lock()
	c.schemas = items
	c.byName = byName
	c.mu.Unlock()
}

// GetByFieldName 按 field_name 查找扩展字段定义
//
// 找不到返回 nil, false。
func (c *EventTypeSchemaCache) GetByFieldName(fieldName string) (*model.EventTypeSchemaLite, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	s, ok := c.byName[fieldName]
	if !ok {
		return nil, false
	}
	// 返回副本
	cp := *s
	return &cp, true
}
