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

// RegionCache Redis 区域缓存
//
// 与其他模块缓存完全同构：
// 详情 Cache-Aside + 分布式锁 + 空标记 + 列表版本号。
type RegionCache struct {
	rdb *redis.Client
}

// NewRegionCache 创建 RegionCache
func NewRegionCache(rdb *redis.Client) *RegionCache {
	return &RegionCache{rdb: rdb}
}

// ---- 单条缓存 ----

// GetDetail 查单条区域缓存
func (c *RegionCache) GetDetail(ctx context.Context, id int64) (*model.Region, bool, error) {
	key := rcfg.RegionDetailKey(id)
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		slog.Debug("cache.区域详情未命中", "id", id)
		return nil, false, nil
	}
	if err != nil {
		slog.Error("cache.区域详情读取失败", "error", err, "id", id)
		return nil, false, err
	}

	// 空值标记
	if string(data) == rcfg.NullMarker {
		slog.Debug("cache.区域详情命中空值", "id", id)
		return nil, true, nil
	}

	var d model.Region
	if err := json.Unmarshal(data, &d); err != nil {
		slog.Error("cache.区域详情反序列化失败", "error", err, "id", id)
		return nil, false, err
	}

	slog.Debug("cache.区域详情命中", "id", id)
	return &d, true, nil
}

// SetDetail 写单条区域缓存
//
// d 为 nil 时写入空值标记防穿透。
func (c *RegionCache) SetDetail(ctx context.Context, id int64, d *model.Region) {
	key := rcfg.RegionDetailKey(id)
	var data []byte
	if d == nil {
		data = []byte(rcfg.NullMarker)
	} else {
		var err error
		data, err = json.Marshal(d)
		if err != nil {
			slog.Error("cache.区域详情序列化失败", "error", err, "id", id)
			return
		}
	}

	if err := c.rdb.Set(ctx, key, data, rcfg.TTL(rcfg.DetailTTLBase, rcfg.DetailTTLJitter)).Err(); err != nil {
		slog.Error("cache.区域详情写入失败", "error", err, "id", id)
	}
}

// DelDetail 删单条区域缓存
func (c *RegionCache) DelDetail(ctx context.Context, id int64) {
	key := rcfg.RegionDetailKey(id)
	if err := c.rdb.Del(ctx, key).Err(); err != nil {
		slog.Error("cache.区域详情删除失败", "error", err, "id", id)
	}
}

// ---- 列表缓存 ----

// getListVersion 获取当前区域列表缓存版本号
func (c *RegionCache) getListVersion(ctx context.Context) int64 {
	v, err := c.rdb.Get(ctx, rcfg.RegionListVersionKey).Int64()
	if err != nil {
		return 0
	}
	return v
}

// GetList 查区域列表缓存（带版本号，类型安全）
func (c *RegionCache) GetList(ctx context.Context, q *model.RegionListQuery) (*model.RegionListData, bool, error) {
	version := c.getListVersion(ctx)
	key := rcfg.RegionListKey(version, q.RegionID, q.DisplayName, q.RegionType, q.Enabled, q.Page, q.PageSize)
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		slog.Debug("cache.区域列表未命中", "key", key)
		return nil, false, nil
	}
	if err != nil {
		slog.Error("cache.区域列表读取失败", "error", err, "key", key)
		return nil, false, err
	}

	var list model.RegionListData
	if err := json.Unmarshal(data, &list); err != nil {
		slog.Error("cache.区域列表反序列化失败", "error", err, "key", key)
		return nil, false, err
	}

	slog.Debug("cache.区域列表命中", "key", key)
	return &list, true, nil
}

// SetList 写区域列表缓存（带当前版本号）
func (c *RegionCache) SetList(ctx context.Context, q *model.RegionListQuery, list *model.RegionListData) {
	version := c.getListVersion(ctx)
	key := rcfg.RegionListKey(version, q.RegionID, q.DisplayName, q.RegionType, q.Enabled, q.Page, q.PageSize)
	data, err := json.Marshal(list)
	if err != nil {
		slog.Error("cache.区域列表序列化失败", "error", err)
		return
	}

	if err := c.rdb.Set(ctx, key, data, rcfg.TTL(rcfg.ListTTLBase, rcfg.ListTTLJitter)).Err(); err != nil {
		slog.Error("cache.区域列表写入失败", "error", err, "key", key)
	}
}

// InvalidateList 使所有区域列表缓存失效
//
// 只需 INCR 版本号，旧版本 key 自然过期（redis-red-lines: 禁止 SCAN+DEL）。
func (c *RegionCache) InvalidateList(ctx context.Context) {
	if err := c.rdb.Incr(ctx, rcfg.RegionListVersionKey).Err(); err != nil {
		slog.Error("cache.区域列表版本号递增失败", "error", err)
	}
}

// ---- 分布式锁 ----

// TryLock 尝试获取分布式锁（防缓存击穿）。
//
// 返回非空 lockID 表示获锁成功，空串表示未获锁（SetNX 失败）。
// lockID 须原样传给 Unlock，确保只删自己的锁。
func (c *RegionCache) TryLock(ctx context.Context, id int64, expire time.Duration) (string, error) {
	key := rcfg.RegionLockKey(id)
	lockID := fmt.Sprintf("%d-%d", id, time.Now().UnixNano())
	ok, err := c.rdb.SetNX(ctx, key, lockID, expire).Result()
	if err != nil {
		return "", fmt.Errorf("region try lock: %w", err)
	}
	if !ok {
		return "", nil
	}
	return lockID, nil
}

// Unlock 释放分布式锁（Lua 原子解锁，只删 lockID 匹配的 key）
func (c *RegionCache) Unlock(ctx context.Context, id int64, lockID string) {
	key := rcfg.RegionLockKey(id)
	if err := c.rdb.Eval(ctx, rcfg.LuaUnlock, []string{key}, lockID).Err(); err != nil {
		slog.Error("cache.区域释放锁失败", "error", err, "key", key)
	}
}
