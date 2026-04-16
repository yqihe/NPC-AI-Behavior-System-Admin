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

// NPCCache Redis NPC 缓存
//
// 与其他模块缓存完全同构：
// 详情 Cache-Aside + 分布式锁 + 空标记 + 列表版本号。
type NPCCache struct {
	rdb *redis.Client
}

// NewNPCCache 创建 NPCCache
func NewNPCCache(rdb *redis.Client) *NPCCache {
	return &NPCCache{rdb: rdb}
}

// ---- 单条缓存 ----

// GetDetail 查单条 NPC 缓存
func (c *NPCCache) GetDetail(ctx context.Context, id int64) (*model.NPC, bool, error) {
	key := rcfg.NPCDetailKey(id)
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		slog.Debug("cache.NPC详情未命中", "id", id)
		return nil, false, nil
	}
	if err != nil {
		slog.Error("cache.NPC详情读取失败", "error", err, "id", id)
		return nil, false, err
	}

	// 空值标记
	if string(data) == rcfg.NullMarker {
		slog.Debug("cache.NPC详情命中空值", "id", id)
		return nil, true, nil
	}

	var n model.NPC
	if err := json.Unmarshal(data, &n); err != nil {
		slog.Error("cache.NPC详情反序列化失败", "error", err, "id", id)
		return nil, false, err
	}

	slog.Debug("cache.NPC详情命中", "id", id)
	return &n, true, nil
}

// SetDetail 写单条 NPC 缓存
//
// n 为 nil 时写入空值标记防穿透。
func (c *NPCCache) SetDetail(ctx context.Context, id int64, n *model.NPC) {
	key := rcfg.NPCDetailKey(id)
	var data []byte
	if n == nil {
		data = []byte(rcfg.NullMarker)
	} else {
		var err error
		data, err = json.Marshal(n)
		if err != nil {
			slog.Error("cache.NPC详情序列化失败", "error", err, "id", id)
			return
		}
	}

	if err := c.rdb.Set(ctx, key, data, rcfg.TTL(rcfg.DetailTTLBase, rcfg.DetailTTLJitter)).Err(); err != nil {
		slog.Error("cache.NPC详情写入失败", "error", err, "id", id)
	}
}

// DelDetail 删单条 NPC 缓存
func (c *NPCCache) DelDetail(ctx context.Context, id int64) {
	key := rcfg.NPCDetailKey(id)
	if err := c.rdb.Del(ctx, key).Err(); err != nil {
		slog.Error("cache.NPC详情删除失败", "error", err, "id", id)
	}
}

// ---- 列表缓存 ----

// getListVersion 获取当前 NPC 列表缓存版本号
func (c *NPCCache) getListVersion(ctx context.Context) int64 {
	v, err := c.rdb.Get(ctx, rcfg.NPCListVersionKey).Int64()
	if err != nil {
		return 0
	}
	return v
}

// GetList 查 NPC 列表缓存（带版本号，类型安全）
func (c *NPCCache) GetList(ctx context.Context, q *model.NPCListQuery) (*model.NPCListData, bool, error) {
	version := c.getListVersion(ctx)
	key := rcfg.NPCListKey(version, q.Label, q.Name, q.TemplateName, q.Enabled, q.Page, q.PageSize)
	data, err := c.rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		slog.Debug("cache.NPC列表未命中", "key", key)
		return nil, false, nil
	}
	if err != nil {
		slog.Error("cache.NPC列表读取失败", "error", err, "key", key)
		return nil, false, err
	}

	var list model.NPCListData
	if err := json.Unmarshal(data, &list); err != nil {
		slog.Error("cache.NPC列表反序列化失败", "error", err, "key", key)
		return nil, false, err
	}

	slog.Debug("cache.NPC列表命中", "key", key)
	return &list, true, nil
}

// SetList 写 NPC 列表缓存（带当前版本号）
func (c *NPCCache) SetList(ctx context.Context, q *model.NPCListQuery, list *model.NPCListData) {
	version := c.getListVersion(ctx)
	key := rcfg.NPCListKey(version, q.Label, q.Name, q.TemplateName, q.Enabled, q.Page, q.PageSize)
	data, err := json.Marshal(list)
	if err != nil {
		slog.Error("cache.NPC列表序列化失败", "error", err)
		return
	}

	if err := c.rdb.Set(ctx, key, data, rcfg.TTL(rcfg.ListTTLBase, rcfg.ListTTLJitter)).Err(); err != nil {
		slog.Error("cache.NPC列表写入失败", "error", err, "key", key)
	}
}

// InvalidateList 使所有 NPC 列表缓存失效
//
// 只需 INCR 版本号，旧版本 key 自然过期（redis-red-lines: 禁止 SCAN+DEL）。
func (c *NPCCache) InvalidateList(ctx context.Context) {
	if err := c.rdb.Incr(ctx, rcfg.NPCListVersionKey).Err(); err != nil {
		slog.Error("cache.NPC列表版本号递增失败", "error", err)
	}
}

// ---- 分布式锁 ----

// TryLock 尝试获取分布式锁（防缓存击穿）。
//
// 返回非空 lockID 表示获锁成功，空串表示未获锁（SetNX 失败）。
// lockID 须原样传给 Unlock，确保只删自己的锁。
func (c *NPCCache) TryLock(ctx context.Context, id int64, expire time.Duration) (string, error) {
	key := rcfg.NPCLockKey(id)
	lockID := fmt.Sprintf("%d-%d", id, time.Now().UnixNano())
	ok, err := c.rdb.SetNX(ctx, key, lockID, expire).Result()
	if err != nil {
		return "", fmt.Errorf("npc try lock: %w", err)
	}
	if !ok {
		return "", nil
	}
	return lockID, nil
}

// Unlock 释放分布式锁（Lua 原子解锁，只删 lockID 匹配的 key）
func (c *NPCCache) Unlock(ctx context.Context, id int64, lockID string) {
	key := rcfg.NPCLockKey(id)
	if err := c.rdb.Eval(ctx, rcfg.LuaUnlock, []string{key}, lockID).Err(); err != nil {
		slog.Error("cache.NPC释放锁失败", "error", err, "key", key)
	}
}
