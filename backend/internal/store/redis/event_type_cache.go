package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
)

// EventTypeCache Redis 事件类型缓存
//
// 和 FieldCache / TemplateCache 完全同构：
// 详情 Cache-Aside + 分布式锁 + 空标记 + 列表版本号。
type EventTypeCache struct {
	rdb *redis.Client
}

// NewEventTypeCache 创建 EventTypeCache
func NewEventTypeCache(rdb *redis.Client) *EventTypeCache {
	return &EventTypeCache{rdb: rdb}
}

// ---- 单条缓存 ----

// GetDetail 查单条事件类型缓存
func (c *EventTypeCache) GetDetail(ctx context.Context, id int64) (*model.EventType, bool, error) {
	key := EventTypeDetailKey(id)
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		slog.Debug("cache.事件类型详情未命中", "id", id)
		return nil, false, nil
	}
	if err != nil {
		slog.Error("cache.事件类型详情读取失败", "error", err, "id", id)
		return nil, false, err
	}

	// 空值标记
	if string(data) == nullMarker {
		slog.Debug("cache.事件类型详情命中空值", "id", id)
		return nil, true, nil
	}

	var et model.EventType
	if err := json.Unmarshal(data, &et); err != nil {
		slog.Error("cache.事件类型详情反序列化失败", "error", err, "id", id)
		return nil, false, err
	}

	slog.Debug("cache.事件类型详情命中", "id", id)
	return &et, true, nil
}

// SetDetail 写单条事件类型缓存
//
// et 为 nil 时写入空值标记防穿透。
func (c *EventTypeCache) SetDetail(ctx context.Context, id int64, et *model.EventType) {
	key := EventTypeDetailKey(id)
	var data []byte
	if et == nil {
		data = []byte(nullMarker)
	} else {
		var err error
		data, err = json.Marshal(et)
		if err != nil {
			slog.Error("cache.事件类型详情序列化失败", "error", err, "id", id)
			return
		}
	}

	if err := c.rdb.Set(ctx, key, data, ttl(detailTTLBase, detailTTLJitter)).Err(); err != nil {
		slog.Error("cache.事件类型详情写入失败", "error", err, "id", id)
	}
}

// DelDetail 删单条事件类型缓存
func (c *EventTypeCache) DelDetail(ctx context.Context, id int64) {
	key := EventTypeDetailKey(id)
	if err := c.rdb.Del(ctx, key).Err(); err != nil {
		slog.Error("cache.事件类型详情删除失败", "error", err, "id", id)
	}
}

// ---- 列表缓存 ----

// getListVersion 获取当前事件类型列表缓存版本号
func (c *EventTypeCache) getListVersion(ctx context.Context) int64 {
	v, err := c.rdb.Get(ctx, eventTypeListVersionKey).Int64()
	if err != nil {
		return 0
	}
	return v
}

// GetList 查事件类型列表缓存（带版本号，类型安全）
func (c *EventTypeCache) GetList(ctx context.Context, q *model.EventTypeListQuery) (*model.EventTypeListData, bool, error) {
	version := c.getListVersion(ctx)
	key := EventTypeListKey(version, q.Label, q.PerceptionMode, q.Enabled, q.Page, q.PageSize)
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		slog.Debug("cache.事件类型列表未命中", "key", key)
		return nil, false, nil
	}
	if err != nil {
		slog.Error("cache.事件类型列表读取失败", "error", err, "key", key)
		return nil, false, err
	}

	var list model.EventTypeListData
	if err := json.Unmarshal(data, &list); err != nil {
		slog.Error("cache.事件类型列表反序列化失败", "error", err, "key", key)
		return nil, false, err
	}

	slog.Debug("cache.事件类型列表命中", "key", key)
	return &list, true, nil
}

// SetList 写事件类型列表缓存（带当前版本号）
func (c *EventTypeCache) SetList(ctx context.Context, q *model.EventTypeListQuery, list *model.EventTypeListData) {
	version := c.getListVersion(ctx)
	key := EventTypeListKey(version, q.Label, q.PerceptionMode, q.Enabled, q.Page, q.PageSize)
	data, err := json.Marshal(list)
	if err != nil {
		slog.Error("cache.事件类型列表序列化失败", "error", err)
		return
	}

	if err := c.rdb.Set(ctx, key, data, ttl(listTTLBase, listTTLJitter)).Err(); err != nil {
		slog.Error("cache.事件类型列表写入失败", "error", err, "key", key)
	}
}

// InvalidateList 使所有事件类型列表缓存失效
//
// 只需 INCR 版本号，旧版本 key 自然过期（redis-red-lines: 禁止 SCAN+DEL）。
func (c *EventTypeCache) InvalidateList(ctx context.Context) {
	if err := c.rdb.Incr(ctx, eventTypeListVersionKey).Err(); err != nil {
		slog.Error("cache.事件类型列表版本号递增失败", "error", err)
	}
}

// ---- 分布式锁 ----

// TryLock 尝试获取分布式锁（防缓存击穿）
func (c *EventTypeCache) TryLock(ctx context.Context, id int64, expire time.Duration) (bool, error) {
	key := EventTypeLockKey(id)
	ok, err := c.rdb.SetNX(ctx, key, "1", expire).Result()
	if err != nil {
		return false, fmt.Errorf("event_type try lock: %w", err)
	}
	return ok, nil
}

// Unlock 释放分布式锁
func (c *EventTypeCache) Unlock(ctx context.Context, id int64) {
	key := EventTypeLockKey(id)
	if err := c.rdb.Del(ctx, key).Err(); err != nil {
		slog.Error("cache.事件类型释放锁失败", "error", err, "key", key)
	}
}
