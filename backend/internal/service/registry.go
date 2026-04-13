package service

import (
	"github.com/yqihe/npc-ai-admin/backend/internal/cache"
	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	storemysql "github.com/yqihe/npc-ai-admin/backend/internal/store/mysql"
	storeredis "github.com/yqihe/npc-ai-admin/backend/internal/store/redis"
)

// Services 聚合所有 service，新增模块加一行
type Services struct {
	Field           *FieldService
	Template        *TemplateService
	EventType       *EventTypeService
	EventTypeSchema *EventTypeSchemaService
	FsmConfig       *FsmConfigService
}

// NewServices 一次性初始化所有 service
func NewServices(
	st *storemysql.Stores,
	rc *storeredis.Caches,
	mc *cache.MemCaches,
	cfg *config.Config,
) *Services {
	return &Services{
		Field:           NewFieldService(st.Field, st.FieldRef, rc.Field, mc.Dict, &cfg.Pagination),
		Template:        NewTemplateService(st.Template, rc.Template, &cfg.Pagination),
		EventType:       NewEventTypeService(st.EventType, rc.EventType, mc.EventTypeSchema, &cfg.Pagination, &cfg.EventType),
		EventTypeSchema: NewEventTypeSchemaService(st.EventTypeSchema, mc.EventTypeSchema, &cfg.EventTypeSchema),
		FsmConfig:       NewFsmConfigService(st.FsmConfig, rc.FsmConfig, &cfg.Pagination, &cfg.FsmConfig),
	}
}
