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

// RuntimeBbKeyCache Redis 运行时 BB Key 缓存
type RuntimeBbKeyCache struct {
	rdb *redis.Client
}

// NewRuntimeBbKeyCache 创建 RuntimeBbKeyCache
func NewRuntimeBbKeyCache(rdb *redis.Client) *RuntimeBbKeyCache {
	return &RuntimeBbKeyCache{rdb: rdb}
}

// ---- 单条缓存 ----

// GetDetail 查单条缓存
func (c *RuntimeBbKeyCache) GetDetail(ctx context.Context, id int64) (*model.RuntimeBbKey, bool, error) {
	key := rcfg.RuntimeBbKeyDetailKey(id)
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		slog.Debug("cache.运行时BBKey详情未命中", "id", id)
		return nil, false, nil
	}
	if err != nil {
		slog.Error("cache.运行时BBKey详情读取失败", "error", err, "id", id)
		return nil, false, err
	}

	if string(data) == rcfg.NullMarker {
		slog.Debug("cache.运行时BBKey详情命中空值", "id", id)
		return nil, true, nil
	}

	var rbk model.RuntimeBbKey
	if err := json.Unmarshal(data, &rbk); err != nil {
		slog.Error("cache.运行时BBKey详情反序列化失败", "error", err, "id", id)
		return nil, false, err
	}

	slog.Debug("cache.运行时BBKey详情命中", "id", id)
	return &rbk, true, nil
}

// SetDetail 写单条缓存
func (c *RuntimeBbKeyCache) SetDetail(ctx context.Context, id int64, rbk *model.RuntimeBbKey) {
	key := rcfg.RuntimeBbKeyDetailKey(id)
	var data []byte
	if rbk == nil {
		data = []byte(rcfg.NullMarker)
	} else {
		var err error
		data, err = json.Marshal(rbk)
		if err != nil {
			slog.Error("cache.运行时BBKey详情序列化失败", "error", err, "id", id)
			return
		}
	}

	if err := c.rdb.Set(ctx, key, data, rcfg.TTL(rcfg.DetailTTLBase, rcfg.DetailTTLJitter)).Err(); err != nil {
		slog.Error("cache.运行时BBKey详情写入失败", "error", err, "id", id)
	}
}

// DelDetail 删单条缓存
func (c *RuntimeBbKeyCache) DelDetail(ctx context.Context, id int64) {
	key := rcfg.RuntimeBbKeyDetailKey(id)
	if err := c.rdb.Del(ctx, key).Err(); err != nil {
		slog.Error("cache.运行时BBKey详情删除失败", "error", err, "id", id)
	}
}

// ---- 列表缓存 ----

// getListVersion 获取当前列表缓存版本号
func (c *RuntimeBbKeyCache) getListVersion(ctx context.Context) int64 {
	v, err := c.rdb.Get(ctx, rcfg.RuntimeBbKeyListVersionKey).Int64()
	if err != nil {
		return 0
	}
	return v
}

// GetList 查列表缓存（带版本号）
func (c *RuntimeBbKeyCache) GetList(ctx context.Context, q *model.RuntimeBbKeyListQuery) (*model.RuntimeBbKeyListData, bool, error) {
	version := c.getListVersion(ctx)
	key := rcfg.RuntimeBbKeyListKey(version, q.Name, q.Label, q.Type, q.GroupName, q.Enabled, q.Page, q.PageSize)
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		slog.Debug("cache.运行时BBKey列表未命中", "key", key)
		return nil, false, nil
	}
	if err != nil {
		slog.Error("cache.运行时BBKey列表读取失败", "error", err, "key", key)
		return nil, false, err
	}

	var list model.RuntimeBbKeyListData
	if err := json.Unmarshal(data, &list); err != nil {
		slog.Error("cache.运行时BBKey列表反序列化失败", "error", err, "key", key)
		return nil, false, err
	}

	slog.Debug("cache.运行时BBKey列表命中", "key", key)
	return &list, true, nil
}

// SetList 写列表缓存（带当前版本号）
func (c *RuntimeBbKeyCache) SetList(ctx context.Context, q *model.RuntimeBbKeyListQuery, list *model.RuntimeBbKeyListData) {
	version := c.getListVersion(ctx)
	key := rcfg.RuntimeBbKeyListKey(version, q.Name, q.Label, q.Type, q.GroupName, q.Enabled, q.Page, q.PageSize)
	data, err := json.Marshal(list)
	if err != nil {
		slog.Error("cache.运行时BBKey列表序列化失败", "error", err)
		return
	}

	if err := c.rdb.Set(ctx, key, data, rcfg.TTL(rcfg.ListTTLBase, rcfg.ListTTLJitter)).Err(); err != nil {
		slog.Error("cache.运行时BBKey列表写入失败", "error", err, "key", key)
	}
}

// InvalidateList 使所有列表缓存失效
// 只需 INCR 版本号，旧版本的 key 自然过期，无需 SCAN
func (c *RuntimeBbKeyCache) InvalidateList(ctx context.Context) {
	if err := c.rdb.Incr(ctx, rcfg.RuntimeBbKeyListVersionKey).Err(); err != nil {
		slog.Error("cache.运行时BBKey列表版本号递增失败", "error", err)
	}
}

// ---- 分布式锁 ----

// TryLock 尝试获取分布式锁（防缓存击穿）。
//
// 返回非空 lockID 表示获锁成功，空串表示未获锁（SetNX 失败）。
// lockID 须原样传给 Unlock，确保只删自己的锁。
func (c *RuntimeBbKeyCache) TryLock(ctx context.Context, id int64, expire time.Duration) (string, error) {
	key := rcfg.RuntimeBbKeyLockKey(id)
	lockID := fmt.Sprintf("%d-%d", id, time.Now().UnixNano())
	ok, err := c.rdb.SetNX(ctx, key, lockID, expire).Result()
	if err != nil {
		return "", fmt.Errorf("runtime_bb_key try lock: %w", err)
	}
	if !ok {
		return "", nil
	}
	return lockID, nil
}

// Unlock 释放分布式锁（Lua 原子解锁，只删 lockID 匹配的 key）
func (c *RuntimeBbKeyCache) Unlock(ctx context.Context, id int64, lockID string) {
	key := rcfg.RuntimeBbKeyLockKey(id)
	if err := c.rdb.Eval(ctx, rcfg.LuaUnlock, []string{key}, lockID).Err(); err != nil {
		slog.Error("cache.运行时BBKey释放锁失败", "error", err, "key", key)
	}
}
