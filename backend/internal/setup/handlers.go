package setup

import (
	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/handler"
)

// Handlers 聚合所有 handler
type Handlers struct {
	Field           *handler.FieldHandler
	Dict            *handler.DictionaryHandler
	Template        *handler.TemplateHandler
	EventType       *handler.EventTypeHandler
	EventTypeSchema *handler.EventTypeSchemaHandler
	FsmConfig       *handler.FsmConfigHandler
	Export          *handler.ExportHandler
}

// NewHandlers 一次性初始化所有 handler
func NewHandlers(st *Stores, svc *Services, mc *MemCaches, cfg *config.Config) *Handlers {
	return &Handlers{
		Field:           handler.NewFieldHandler(svc.Field, svc.Template, svc.FsmConfig, &cfg.Validation),
		Dict:            handler.NewDictionaryHandler(mc.Dict),
		Template:        handler.NewTemplateHandler(st.DB, svc.Template, svc.Field, &cfg.Validation),
		EventType:       handler.NewEventTypeHandler(svc.EventType, svc.EventTypeSchema, &cfg.EventType),
		EventTypeSchema: handler.NewEventTypeSchemaHandler(svc.EventTypeSchema, svc.EventType, &cfg.EventTypeSchema),
		FsmConfig:       handler.NewFsmConfigHandler(st.DB, svc.FsmConfig, svc.Field, &cfg.FsmConfig),
		Export:          handler.NewExportHandler(svc.EventType, svc.FsmConfig),
	}
}
