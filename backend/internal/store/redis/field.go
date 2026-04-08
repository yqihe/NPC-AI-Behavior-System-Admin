package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/yqihe/npc-ai-admin/backend/internal/cache"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
)

// FieldCache Redis 字段缓存
type FieldCache struct {
	rdb *redis.Client
}

// NewFieldCache 创建 FieldCache
func NewFieldCache(rdb *redis.Client) *FieldCache {
	return &FieldCache{rdb: rdb}
}

// nullMarker 空值标记，防缓存穿透
const nullMarker = `{"_null":true}`

// ttl 带随机抖动的过期时间，防缓存雪崩
func ttl(base time.Duration, jitter time.Duration) time.Duration {
	return base + time.Duration(rand.Int63n(int64(jitter)))
}

// ---- 单条缓存 ----

// GetDetail 查单条缓存
func (c *FieldCache) GetDetail(ctx context.Context, name string) (*model.Field, bool, error) {
	key := cache.FieldDetailKey(name)
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		slog.Debug("cache.字段详情未命中", "name", name)
		return nil, false, nil
	}
	if err != nil {
		slog.Error("cache.字段详情读取失败", "error", err, "name", name)
		return nil, false, err
	}

	// 空值标记
	if string(data) == nullMarker {
		slog.Debug("cache.字段详情命中空值", "name", name)
		return nil, true, nil
	}

	var field model.Field
	if err := json.Unmarshal(data, &field); err != nil {
		slog.Error("cache.字段详情反序列化失败", "error", err, "name", name)
		return nil, false, err
	}

	slog.Debug("cache.字段详情命中", "name", name)
	return &field, true, nil
}

// SetDetail 写单条缓存
func (c *FieldCache) SetDetail(ctx context.Context, name string, field *model.Field) {
	key := cache.FieldDetailKey(name)
	var data []byte
	if field == nil {
		data = []byte(nullMarker)
	} else {
		var err error
		data, err = json.Marshal(field)
		if err != nil {
			slog.Error("cache.字段详情序列化失败", "error", err, "name", name)
			return
		}
	}

	if err := c.rdb.Set(ctx, key, data, ttl(5*time.Minute, 30*time.Second)).Err(); err != nil {
		slog.Error("cache.字段详情写入失败", "error", err, "name", name)
	}
}

// DelDetail 删单条缓存
func (c *FieldCache) DelDetail(ctx context.Context, name string) {
	key := cache.FieldDetailKey(name)
	if err := c.rdb.Del(ctx, key).Err(); err != nil {
		slog.Error("cache.字段详情删除失败", "error", err, "name", name)
	}
}

// ---- 列表缓存 ----

// GetList 查列表缓存
func (c *FieldCache) GetList(ctx context.Context, q *model.FieldListQuery) (*model.ListData, bool, error) {
	key := cache.FieldListKey(0, q.Type, q.Category, q.Label, q.Page, q.PageSize)
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		slog.Debug("cache.字段列表未命中", "key", key)
		return nil, false, nil
	}
	if err != nil {
		slog.Error("cache.字段列表读取失败", "error", err, "key", key)
		return nil, false, err
	}

	var list model.ListData
	if err := json.Unmarshal(data, &list); err != nil {
		slog.Error("cache.字段列表反序列化失败", "error", err, "key", key)
		return nil, false, err
	}

	slog.Debug("cache.字段列表命中", "key", key)
	return &list, true, nil
}

// SetList 写列表缓存
func (c *FieldCache) SetList(ctx context.Context, q *model.FieldListQuery, list *model.ListData) {
	key := cache.FieldListKey(0, q.Type, q.Category, q.Label, q.Page, q.PageSize)
	data, err := json.Marshal(list)
	if err != nil {
		slog.Error("cache.字段列表序列化失败", "error", err)
		return
	}

	if err := c.rdb.Set(ctx, key, data, ttl(1*time.Minute, 10*time.Second)).Err(); err != nil {
		slog.Error("cache.字段列表写入失败", "error", err, "key", key)
	}
}

// InvalidateList 清除所有列表缓存（字段 CRUD 后调用）
func (c *FieldCache) InvalidateList(ctx context.Context) {
	pattern := cache.FieldListPattern()
	iter := c.rdb.Scan(ctx, 0, pattern, 100).Iterator()
	keys := make([]string, 0)
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		slog.Error("cache.字段列表扫描失败", "error", err)
		return
	}
	if len(keys) > 0 {
		if err := c.rdb.Del(ctx, keys...).Err(); err != nil {
			slog.Error("cache.字段列表批量删除失败", "error", err, "count", len(keys))
		} else {
			slog.Debug("cache.字段列表缓存已清除", "count", len(keys))
		}
	}
}

// ---- 健康检查 ----

// Ping 检查 Redis 连接
func (c *FieldCache) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// Available 检查 Redis 是否可用（降级判断）
func (c *FieldCache) Available(ctx context.Context) bool {
	ctx2, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	err := c.rdb.Ping(ctx2).Err()
	if err != nil {
		slog.Warn("cache.Redis不可用，降级本地", "error", err)
	}
	return err == nil
}

// ---- 分布式锁 ----

// TryLock 尝试获取分布式锁（防缓存击穿）
func (c *FieldCache) TryLock(ctx context.Context, name string, expire time.Duration) (bool, error) {
	key := cache.FieldLockKey(name)
	ok, err := c.rdb.SetNX(ctx, key, "1", expire).Result()
	if err != nil {
		return false, fmt.Errorf("try lock: %w", err)
	}
	return ok, nil
}

// Unlock 释放分布式锁
func (c *FieldCache) Unlock(ctx context.Context, name string) {
	key := cache.FieldLockKey(name)
	c.rdb.Del(ctx, key)
}
