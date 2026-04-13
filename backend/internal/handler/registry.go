package handler

import (
	"github.com/jmoiron/sqlx"
	"github.com/yqihe/npc-ai-admin/backend/internal/cache"
	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/service"
)

// Handlers 聚合所有 handler，新增模块加一行
type Handlers struct {
	Field           *FieldHandler
	Dict            *DictionaryHandler
	Template        *TemplateHandler
	EventType       *EventTypeHandler
	EventTypeSchema *EventTypeSchemaHandler
	FsmConfig       *FsmConfigHandler
	Export          *ExportHandler
}

// NewHandlers 一次性初始化所有 handler
func NewHandlers(db *sqlx.DB, svc *service.Services, mc *cache.MemCaches, cfg *config.Config) *Handlers {
	return &Handlers{
		Field:           NewFieldHandler(svc.Field, svc.Template, &cfg.Validation),
		Dict:            NewDictionaryHandler(mc.Dict),
		Template:        NewTemplateHandler(db, svc.Template, svc.Field, &cfg.Validation),
		EventType:       NewEventTypeHandler(svc.EventType, svc.EventTypeSchema, &cfg.EventType),
		EventTypeSchema: NewEventTypeSchemaHandler(svc.EventTypeSchema, &cfg.EventTypeSchema),
		FsmConfig:       NewFsmConfigHandler(svc.FsmConfig, &cfg.FsmConfig),
		Export:          NewExportHandler(svc.EventType, svc.FsmConfig),
	}
}
