package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/npc-admin/backend/internal/model"
)

const (
	keyPrefix = "admin:"
	cacheTTL  = 5 * time.Minute
)

// RedisCache 实现 Cache 接口，使用 Redis 缓存配置列表。
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache 创建 Redis 缓存实例并验证连接。
func NewRedisCache(ctx context.Context, addr string) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		return nil, fmt.Errorf("cache.redis_ping: %w", err)
	}

	slog.Info("cache.redis_connected", "addr", addr)
	return &RedisCache{client: client}, nil
}

// Close 关闭 Redis 连接，用于优雅关闭。
func (c *RedisCache) Close() error {
	return c.client.Close()
}

func (c *RedisCache) cacheKey(collection string) string {
	return keyPrefix + collection + ":list"
}

func (c *RedisCache) GetList(ctx context.Context, collection string) ([]model.Document, error) {
	key := c.cacheKey(collection)
	data, err := c.client.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			slog.Debug("cache.miss", "collection", collection)
			return nil, ErrCacheMiss
		}
		slog.Warn("cache.get_error", "collection", collection, "err", err)
		return nil, fmt.Errorf("cache.get: %w", err)
	}

	var docs []model.Document
	if err := json.Unmarshal(data, &docs); err != nil {
		slog.Warn("cache.unmarshal_error", "collection", collection, "err", err)
		// 反序列化失败视为 miss，让 service 层回源
		return nil, ErrCacheMiss
	}

	slog.Debug("cache.hit", "collection", collection, "count", len(docs))
	return docs, nil
}

func (c *RedisCache) SetList(ctx context.Context, collection string, docs []model.Document) error {
	key := c.cacheKey(collection)

	data, err := json.Marshal(docs)
	if err != nil {
		slog.Warn("cache.marshal_error", "collection", collection, "err", err)
		return fmt.Errorf("cache.set_marshal: %w", err)
	}

	if err := c.client.Set(ctx, key, data, cacheTTL).Err(); err != nil {
		slog.Warn("cache.set_error", "collection", collection, "err", err)
		return fmt.Errorf("cache.set: %w", err)
	}

	slog.Debug("cache.set", "collection", collection, "count", len(docs))
	return nil
}

func (c *RedisCache) Invalidate(ctx context.Context, collection string) error {
	key := c.cacheKey(collection)
	if err := c.client.Del(ctx, key).Err(); err != nil {
		slog.Warn("cache.invalidate_error", "collection", collection, "err", err)
		return fmt.Errorf("cache.invalidate: %w", err)
	}

	slog.Debug("cache.invalidated", "collection", collection)
	return nil
}
