package cache

import (
	"context"
	"errors"

	"github.com/npc-admin/backend/internal/model"
)

// ErrCacheMiss 表示缓存未命中。
var ErrCacheMiss = errors.New("缓存未命中")

// Cache 定义配置列表的缓存接口。
// service 层在 GetList 返回 ErrCacheMiss 时回源 MongoDB，其余 error 降级跳过缓存。
type Cache interface {
	GetList(ctx context.Context, collection string) ([]model.Document, error)
	SetList(ctx context.Context, collection string, docs []model.Document) error
	Invalidate(ctx context.Context, collection string) error
}
