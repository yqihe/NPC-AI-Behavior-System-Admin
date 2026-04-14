package setup

import (
	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/service"
)

// Services 聚合所有 service
type Services struct {
	Field           *service.FieldService
	Template        *service.TemplateService
	EventType       *service.EventTypeService
	EventTypeSchema *service.EventTypeSchemaService
	FsmConfig       *service.FsmConfigService
}

// NewServices 一次性初始化所有 service
func NewServices(st *Stores, rc *Caches, mc *MemCaches, cfg *config.Config) *Services {
	return &Services{
		Field:           service.NewFieldService(st.Field, st.FieldRef, rc.Field, mc.Dict, &cfg.Pagination),
		Template:        service.NewTemplateService(st.Template, rc.Template, &cfg.Pagination),
		EventType:       service.NewEventTypeService(st.EventType, st.SchemaRef, rc.EventType, mc.EventTypeSchema, &cfg.Pagination, &cfg.EventType),
		EventTypeSchema: service.NewEventTypeSchemaService(st.EventTypeSchema, st.SchemaRef, mc.EventTypeSchema, &cfg.EventTypeSchema),
		FsmConfig:       service.NewFsmConfigService(st.FsmConfig, rc.FsmConfig, &cfg.Pagination, &cfg.FsmConfig),
	}
}
