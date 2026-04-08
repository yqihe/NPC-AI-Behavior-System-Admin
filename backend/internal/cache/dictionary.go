package cache

import (
	"context"
	"log/slog"
	"sync"

	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	"github.com/yqihe/npc-ai-admin/backend/internal/store/mysql"
)

// DictCache dictionaries 内存缓存
// 启动时从 MySQL 全量加载，运行时用 map 翻译 label，不查表不 JOIN
type DictCache struct {
	mu    sync.RWMutex
	store *mysql.DictionaryStore

	// group → name → Dictionary
	data map[string]map[string]model.Dictionary
}

// NewDictCache 创建 DictCache
func NewDictCache(store *mysql.DictionaryStore) *DictCache {
	return &DictCache{
		store: store,
		data:  make(map[string]map[string]model.Dictionary),
	}
}

// Load 从 MySQL 全量加载到内存
func (c *DictCache) Load(ctx context.Context) error {
	items, err := c.store.ListAll(ctx)
	if err != nil {
		return err
	}

	data := make(map[string]map[string]model.Dictionary)
	for _, item := range items {
		if _, ok := data[item.GroupName]; !ok {
			data[item.GroupName] = make(map[string]model.Dictionary)
		}
		data[item.GroupName][item.Name] = item
	}

	c.mu.Lock()
	c.data = data
	c.mu.Unlock()

	slog.Info("cache.字典加载完成", "groups", len(data), "total", len(items))
	return nil
}

// GetLabel 翻译 name → 中文 label
func (c *DictCache) GetLabel(group, name string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if g, ok := c.data[group]; ok {
		if d, ok := g[name]; ok {
			return d.Label
		}
	}
	return name
}

// ListByGroup 获取某个 group 下所有选项（前端下拉用）
func (c *DictCache) ListByGroup(group string) []model.DictionaryItem {
	c.mu.RLock()
	defer c.mu.RUnlock()

	g, ok := c.data[group]
	if !ok {
		return make([]model.DictionaryItem, 0)
	}

	items := make([]model.DictionaryItem, 0, len(g))
	for _, d := range g {
		items = append(items, model.DictionaryItem{
			Name:  d.Name,
			Label: d.Label,
			Extra: d.Extra,
		})
	}
	return items
}

// Exists 检查某个 group + name 是否存在
func (c *DictCache) Exists(group, name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if g, ok := c.data[group]; ok {
		_, ok := g[name]
		return ok
	}
	return false
}
