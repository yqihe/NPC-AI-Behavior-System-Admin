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

// FieldCache Redis 字段缓存
type FieldCache struct {
	rdb *redis.Client
}

// NewFieldCache 创建 FieldCache
func NewFieldCache(rdb *redis.Client) *FieldCache {
	return &FieldCache{rdb: rdb}
}

// ---- 单条缓存 ----

// GetDetail 查单条缓存
func (c *FieldCache) GetDetail(ctx context.Context, id int64) (*model.Field, bool, error) {
	key := rcfg.FieldDetailKey(id)
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
	if string(data) == rcfg.NullMarker {
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
	key := rcfg.FieldDetailKey(id)
	var data []byte
	if field == nil {
		data = []byte(rcfg.NullMarker)
	} else {
		var err error
		data, err = json.Marshal(field)
		if err != nil {
			slog.Error("cache.字段详情序列化失败", "error", err, "id", id)
			return
		}
	}

	if err := c.rdb.Set(ctx, key, data, rcfg.TTL(rcfg.DetailTTLBase, rcfg.DetailTTLJitter)).Err(); err != nil {
		slog.Error("cache.字段详情写入失败", "error", err, "id", id)
	}
}

// DelDetail 删单条缓存
func (c *FieldCache) DelDetail(ctx context.Context, id int64) {
	key := rcfg.FieldDetailKey(id)
	if err := c.rdb.Del(ctx, key).Err(); err != nil {
		slog.Error("cache.字段详情删除失败", "error", err, "id", id)
	}
}

// ---- 列表缓存 ----

// getListVersion 获取当前列表缓存版本号
func (c *FieldCache) getListVersion(ctx context.Context) int64 {
	v, err := c.rdb.Get(ctx, rcfg.FieldListVersionKey).Int64()
	if err != nil {
		return 0
	}
	return v
}

// GetList 查列表缓存（带版本号）
func (c *FieldCache) GetList(ctx context.Context, q *model.FieldListQuery) (*model.FieldListData, bool, error) {
	version := c.getListVersion(ctx)
	key := rcfg.FieldListKey(version, q.Name, q.Type, q.Category, q.Label, q.Enabled, q.ExposesBB, q.Page, q.PageSize)
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
	key := rcfg.FieldListKey(version, q.Name, q.Type, q.Category, q.Label, q.Enabled, q.ExposesBB, q.Page, q.PageSize)
	data, err := json.Marshal(list)
	if err != nil {
		slog.Error("cache.字段列表序列化失败", "error", err)
		return
	}

	if err := c.rdb.Set(ctx, key, data, rcfg.TTL(rcfg.ListTTLBase, rcfg.ListTTLJitter)).Err(); err != nil {
		slog.Error("cache.字段列表写入失败", "error", err, "key", key)
	}
}

// InvalidateList 使所有列表缓存失效
// 只需 INCR 版本号，旧版本的 key 自然过期，无需 SCAN
func (c *FieldCache) InvalidateList(ctx context.Context) {
	if err := c.rdb.Incr(ctx, rcfg.FieldListVersionKey).Err(); err != nil {
		slog.Error("cache.字段列表版本号递增失败", "error", err)
	}
}

// ---- 分布式锁 ----

// TryLock 尝试获取分布式锁（防缓存击穿）。
//
// 返回非空 lockID 表示获锁成功，空串表示未获锁（SetNX 失败）。
// lockID 须原样传给 Unlock，确保只删自己的锁。
func (c *FieldCache) TryLock(ctx context.Context, id int64, expire time.Duration) (string, error) {
	key := rcfg.FieldLockKey(id)
	lockID := fmt.Sprintf("%d-%d", id, time.Now().UnixNano())
	ok, err := c.rdb.SetNX(ctx, key, lockID, expire).Result()
	if err != nil {
		return "", fmt.Errorf("field try lock: %w", err)
	}
	if !ok {
		return "", nil
	}
	return lockID, nil
}

// Unlock 释放分布式锁（Lua 原子解锁，只删 lockID 匹配的 key）
func (c *FieldCache) Unlock(ctx context.Context, id int64, lockID string) {
	key := rcfg.FieldLockKey(id)
	if err := c.rdb.Eval(ctx, rcfg.LuaUnlock, []string{key}, lockID).Err(); err != nil {
		slog.Error("cache.字段释放锁失败", "error", err, "key", key)
	}
}
