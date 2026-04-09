package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"time"

	"github.com/redis/go-redis/v9"
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

const (
	// nullMarker 空值标记，防缓存穿透
	nullMarker = `{"_null":true}`

	// 缓存 TTL 配置
	detailTTLBase   = 5 * time.Minute  // 单条缓存基础过期时间
	detailTTLJitter = 30 * time.Second  // 单条缓存随机抖动
	listTTLBase     = 1 * time.Minute   // 列表缓存基础过期时间
	listTTLJitter   = 10 * time.Second  // 列表缓存随机抖动
	lockExpire      = 3 * time.Second   // 分布式锁默认过期时间
	pingTimeout     = 500 * time.Millisecond // 健康检查超时
)

// ttl 带随机抖动的过期时间，防缓存雪崩
func ttl(base time.Duration, jitter time.Duration) time.Duration {
	return base + time.Duration(rand.Int63n(int64(jitter)))
}

// ---- 单条缓存 ----

// GetDetail 查单条缓存
func (c *FieldCache) GetDetail(ctx context.Context, id int64) (*model.Field, bool, error) {
	key := FieldDetailKey(id)
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		slog.Debug("cache.字段详情未命中", "id", id)
		return nil, false, nil
	}
	if err != nil {
		slog.Error("cache.字段详情读取失败", "error", err, "id", id)
		return nil, false, err
	}

	// 空值标记
	if string(data) == nullMarker {
		slog.Debug("cache.字段详情命中空值", "id", id)
		return nil, true, nil
	}

	var field model.Field
	if err := json.Unmarshal(data, &field); err != nil {
		slog.Error("cache.字段详情反序列化失败", "error", err, "id", id)
		return nil, false, err
	}

	slog.Debug("cache.字段详情命中", "id", id)
	return &field, true, nil
}

// SetDetail 写单条缓存
func (c *FieldCache) SetDetail(ctx context.Context, id int64, field *model.Field) {
	key := FieldDetailKey(id)
	var data []byte
	if field == nil {
		data = []byte(nullMarker)
	} else {
		var err error
		data, err = json.Marshal(field)
		if err != nil {
			slog.Error("cache.字段详情序列化失败", "error", err, "id", id)
			return
		}
	}

	if err := c.rdb.Set(ctx, key, data, ttl(detailTTLBase, detailTTLJitter)).Err(); err != nil {
		slog.Error("cache.字段详情写入失败", "error", err, "id", id)
	}
}

// DelDetail 删单条缓存
func (c *FieldCache) DelDetail(ctx context.Context, id int64) {
	key := FieldDetailKey(id)
	if err := c.rdb.Del(ctx, key).Err(); err != nil {
		slog.Error("cache.字段详情删除失败", "error", err, "id", id)
	}
}

// ---- 列表缓存 ----

// getListVersion 获取当前列表缓存版本号
func (c *FieldCache) getListVersion(ctx context.Context) int64 {
	v, err := c.rdb.Get(ctx, fieldListVersionKey).Int64()
	if err != nil {
		return 0
	}
	return v
}

// GetList 查列表缓存（带版本号）
func (c *FieldCache) GetList(ctx context.Context, q *model.FieldListQuery) (*model.FieldListData, bool, error) {
	version := c.getListVersion(ctx)
	key := FieldListKey(version, q.Type, q.Category, q.Label, q.Enabled, q.Page, q.PageSize)
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		slog.Debug("cache.字段列表未命中", "key", key)
		return nil, false, nil
	}
	if err != nil {
		slog.Error("cache.字段列表读取失败", "error", err, "key", key)
		return nil, false, err
	}

	var list model.FieldListData
	if err := json.Unmarshal(data, &list); err != nil {
		slog.Error("cache.字段列表反序列化失败", "error", err, "key", key)
		return nil, false, err
	}

	slog.Debug("cache.字段列表命中", "key", key)
	return &list, true, nil
}

// SetList 写列表缓存（带当前版本号）
func (c *FieldCache) SetList(ctx context.Context, q *model.FieldListQuery, list *model.FieldListData) {
	version := c.getListVersion(ctx)
	key := FieldListKey(version, q.Type, q.Category, q.Label, q.Enabled, q.Page, q.PageSize)
	data, err := json.Marshal(list)
	if err != nil {
		slog.Error("cache.字段列表序列化失败", "error", err)
		return
	}

	if err := c.rdb.Set(ctx, key, data, ttl(listTTLBase, listTTLJitter)).Err(); err != nil {
		slog.Error("cache.字段列表写入失败", "error", err, "key", key)
	}
}

// InvalidateList 使所有列表缓存失效
// 只需 INCR 版本号，旧版本的 key 自然过期，无需 SCAN
func (c *FieldCache) InvalidateList(ctx context.Context) {
	if err := c.rdb.Incr(ctx, fieldListVersionKey).Err(); err != nil {
		slog.Error("cache.列表版本号递增失败", "error", err)
	}
}

// ---- 健康检查 ----

// Ping 检查 Redis 连接
func (c *FieldCache) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// Available 检查 Redis 是否可用（降级判断）
func (c *FieldCache) Available(ctx context.Context) bool {
	ctx2, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()
	err := c.rdb.Ping(ctx2).Err()
	if err != nil {
		slog.Warn("cache.Redis不可用，降级本地", "error", err)
	}
	return err == nil
}

// ---- 分布式锁 ----

// TryLock 尝试获取分布式锁（防缓存击穿）
func (c *FieldCache) TryLock(ctx context.Context, id int64, expire time.Duration) (bool, error) {
	key := FieldLockKey(id)
	ok, err := c.rdb.SetNX(ctx, key, "1", expire).Result()
	if err != nil {
		return false, fmt.Errorf("try lock: %w", err)
	}
	return ok, nil
}

// Unlock 释放分布式锁
func (c *FieldCache) Unlock(ctx context.Context, id int64) {
	key := FieldLockKey(id)
	if err := c.rdb.Del(ctx, key).Err(); err != nil {
		slog.Error("cache.释放锁失败", "error", err, "key", key)
	}
}
