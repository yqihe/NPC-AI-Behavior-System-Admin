package setup

import (
	"context"
	"log/slog"

	goredis "github.com/redis/go-redis/v9"
	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	storeredis "github.com/yqihe/npc-ai-admin/backend/internal/store/redis"
)

// Caches 聚合 Redis 连接 + 所有 cache
type Caches struct {
	RDB          *goredis.Client
	Field        *storeredis.FieldCache
	Template     *storeredis.TemplateCache
	EventType    *storeredis.EventTypeCache
	FsmConfig    *storeredis.FsmConfigCache
	FsmStateDict *storeredis.FsmStateDictCache
	BtTree       *storeredis.BtTreeCache
	BtNodeType   *storeredis.BtNodeTypeCache
	Npc          *storeredis.NPCCache
	RuntimeBbKey *storeredis.RuntimeBbKeyCache
	Region       *storeredis.RegionCache
}

// NewCaches 连接 Redis + 一次性初始化所有 cache
//
// Redis 不可用时仅打 Warn 日志，不终止启动（缓存降级）。
func NewCaches(ctx context.Context, cfg *config.RedisConfig) *Caches {
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		slog.Warn("启动.Redis连接失败，缓存将降级", "error", err)
	} else {
		slog.Info("启动.Redis连接成功", "addr", cfg.Addr)
	}

	return &Caches{
		RDB:          rdb,
		Field:        storeredis.NewFieldCache(rdb),
		Template:     storeredis.NewTemplateCache(rdb),
		EventType:    storeredis.NewEventTypeCache(rdb),
		FsmConfig:    storeredis.NewFsmConfigCache(rdb),
		FsmStateDict: storeredis.NewFsmStateDictCache(rdb),
		BtTree:       storeredis.NewBtTreeCache(rdb),
		BtNodeType:   storeredis.NewBtNodeTypeCache(rdb),
		Npc:          storeredis.NewNPCCache(rdb),
		RuntimeBbKey: storeredis.NewRuntimeBbKeyCache(rdb),
		Region:       storeredis.NewRegionCache(rdb),
	}
}

// Close 关闭 Redis 连接
func (c *Caches) Close() error { return c.RDB.Close() }
