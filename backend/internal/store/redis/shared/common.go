package shared

import (
	"context"
	"log/slog"
	"math/rand"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// 缓存共享常量
const (
	NullMarker      = `{"_null":true}`
	DetailTTLBase   = 5 * time.Minute
	DetailTTLJitter = 30 * time.Second
	ListTTLBase     = 1 * time.Minute
	ListTTLJitter   = 10 * time.Second
	LockExpire      = 3 * time.Second
	PingTimeout     = 500 * time.Millisecond

	// LuaUnlock 原子解锁脚本：只删除 value 与 lockID 匹配的 key，防止误删他人的锁
	LuaUnlock = `if redis.call('get',KEYS[1])==ARGV[1] then return redis.call('del',KEYS[1]) else return 0 end`
)

// TTL 带随机抖动的过期时间，防缓存雪崩
func TTL(base time.Duration, jitter time.Duration) time.Duration {
	return base + time.Duration(rand.Int63n(int64(jitter)))
}

// Ping 检查 Redis 连接
func Ping(ctx context.Context, rdb *goredis.Client) error {
	return rdb.Ping(ctx).Err()
}

// Available 检查 Redis 是否可用（降级判断）
func Available(ctx context.Context, rdb *goredis.Client) bool {
	ctx2, cancel := context.WithTimeout(ctx, PingTimeout)
	defer cancel()
	err := rdb.Ping(ctx2).Err()
	if err != nil {
		slog.Warn("cache.Redis不可用，降级直查MySQL", "error", err)
	}
	return err == nil
}
