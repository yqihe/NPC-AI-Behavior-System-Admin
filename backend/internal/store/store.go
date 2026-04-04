package store

import (
	"context"
	"errors"

	"github.com/npc-admin/backend/internal/model"
)

// 自定义错误，service 层根据这些错误映射 HTTP 状态码。
var (
	ErrNotFound  = errors.New("记录不存在")
	ErrDuplicate = errors.New("名称已存在")
)

// Store 定义配置数据的存储接口。
// collection 参数对应 MongoDB collection 名（event_types / npc_types / fsm_configs / bt_trees）。
type Store interface {
	List(ctx context.Context, collection string) ([]model.Document, error)
	Get(ctx context.Context, collection string, name string) (model.Document, error)
	Create(ctx context.Context, collection string, doc model.Document) error
	Update(ctx context.Context, collection string, name string, doc model.Document) error
	Delete(ctx context.Context, collection string, name string) error
}
