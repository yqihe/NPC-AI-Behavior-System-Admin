package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	rcfg "github.com/yqihe/npc-ai-admin/backend/internal/store/redis/shared"
)

// TemplateCache Redis 模板缓存
//
// 严格遵守"分层职责"硬规则：只缓存 templates 裸行（*model.Template），
// 不缓存任何 fields 补全后的复合数据。字段补全是 handler 层的拼装动作。
type TemplateCache struct {
	rdb *redis.Client
}

// NewTemplateCache 创建 TemplateCache
func NewTemplateCache(rdb *redis.Client) *TemplateCache {
	return &TemplateCache{rdb: rdb}
}

// ---- 单条缓存 ----

// GetDetail 查单条模板缓存（裸行）
func (c *TemplateCache) GetDetail(ctx context.Context, id int64) (*model.Template, bool, error) {
	key := rcfg.TemplateDetailKey(id)
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		slog.Debug("cache.模板详情未命中", "id", id)
		return nil, false, nil
	}
	if err != nil {
		slog.Error("cache.模板详情读取失败", "error", err, "id", id)
		return nil, false, err
	}

	// 空值标记
	if string(data) == rcfg.NullMarker {
		slog.Debug("cache.模板详情命中空值", "id", id)
		return nil, true, nil
	}

	var tpl model.Template
	if err := json.Unmarshal(data, &tpl); err != nil {
		slog.Error("cache.模板详情反序列化失败", "error", err, "id", id)
		return nil, false, err
	}

	slog.Debug("cache.模板详情命中", "id", id)
	return &tpl, true, nil
}

// SetDetail 写单条模板缓存
//
// tpl 为 nil 时写入空值标记防穿透。
func (c *TemplateCache) SetDetail(ctx context.Context, id int64, tpl *model.Template) {
	key := rcfg.TemplateDetailKey(id)
	var data []byte
	if tpl == nil {
		data = []byte(rcfg.NullMarker)
	} else {
		var err error
		data, err = json.Marshal(tpl)
		if err != nil {
			slog.Error("cache.模板详情序列化失败", "error", err, "id", id)
			return
		}
	}

	if err := c.rdb.Set(ctx, key, data, rcfg.TTL(rcfg.DetailTTLBase, rcfg.DetailTTLJitter)).Err(); err != nil {
		slog.Error("cache.模板详情写入失败", "error", err, "id", id)
	}
}

// DelDetail 删单条模板缓存
func (c *TemplateCache) DelDetail(ctx context.Context, id int64) {
	key := rcfg.TemplateDetailKey(id)
	if err := c.rdb.Del(ctx, key).Err(); err != nil {
		slog.Error("cache.模板详情删除失败", "error", err, "id", id)
	}
}

// ---- 列表缓存 ----

// getListVersion 获取当前模板列表缓存版本号
func (c *TemplateCache) getListVersion(ctx context.Context) int64 {
	v, err := c.rdb.Get(ctx, rcfg.TemplateListVersionKey).Int64()
	if err != nil {
		return 0
	}
	return v
}

// GetList 查模板列表缓存（带版本号）
func (c *TemplateCache) GetList(ctx context.Context, q *model.TemplateListQuery) (*model.TemplateListData, bool, error) {
	version := c.getListVersion(ctx)
	key := rcfg.TemplateListKey(version, q.Name, q.Label, q.Enabled, q.Page, q.PageSize)
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		slog.Debug("cache.模板列表未命中", "key", key)
		return nil, false, nil
	}
	if err != nil {
		slog.Error("cache.模板列表读取失败", "error", err, "key", key)
		return nil, false, err
	}

	var list model.TemplateListData
	if err := json.Unmarshal(data, &list); err != nil {
		slog.Error("cache.模板列表反序列化失败", "error", err, "key", key)
		return nil, false, err
	}

	slog.Debug("cache.模板列表命中", "key", key)
	return &list, true, nil
}

// SetList 写模板列表缓存（带当前版本号）
func (c *TemplateCache) SetList(ctx context.Context, q *model.TemplateListQuery, list *model.TemplateListData) {
	version := c.getListVersion(ctx)
	key := rcfg.TemplateListKey(version, q.Name, q.Label, q.Enabled, q.Page, q.PageSize)
	data, err := json.Marshal(list)
	if err != nil {
		slog.Error("cache.模板列表序列化失败", "error", err)
		return
	}

	if err := c.rdb.Set(ctx, key, data, rcfg.TTL(rcfg.ListTTLBase, rcfg.ListTTLJitter)).Err(); err != nil {
		slog.Error("cache.模板列表写入失败", "error", err, "key", key)
	}
}

// InvalidateList 使所有模板列表缓存失效
//
// 只需 INCR 版本号，旧版本的 key 自然过期，无需 SCAN（redis-red-lines）。
func (c *TemplateCache) InvalidateList(ctx context.Context) {
	if err := c.rdb.Incr(ctx, rcfg.TemplateListVersionKey).Err(); err != nil {
		slog.Error("cache.模板列表版本号递增失败", "error", err)
	}
}

// ---- 分布式锁 ----

// TryLock 尝试获取分布式锁（防缓存击穿）。
//
// 返回非空 lockID 表示获锁成功，空串表示未获锁（SetNX 失败）。
// lockID 须原样传给 Unlock，确保只删自己的锁。
func (c *TemplateCache) TryLock(ctx context.Context, id int64, expire time.Duration) (string, error) {
	key := rcfg.TemplateLockKey(id)
	lockID := fmt.Sprintf("%d-%d", id, time.Now().UnixNano())
	ok, err := c.rdb.SetNX(ctx, key, lockID, expire).Result()
	if err != nil {
		return "", fmt.Errorf("template try lock: %w", err)
	}
	if !ok {
		return "", nil
	}
	return lockID, nil
}

// Unlock 释放分布式锁（Lua 原子解锁，只删 lockID 匹配的 key）
func (c *TemplateCache) Unlock(ctx context.Context, id int64, lockID string) {
	key := rcfg.TemplateLockKey(id)
	if err := c.rdb.Eval(ctx, rcfg.LuaUnlock, []string{key}, lockID).Err(); err != nil {
		slog.Error("cache.模板释放锁失败", "error", err, "key", key)
	}
}
