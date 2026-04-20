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
	FsmStateDict    *handler.FsmStateDictHandler
	BtTree          *handler.BtTreeHandler
	BtNodeType      *handler.BtNodeTypeHandler
	Export          *handler.ExportHandler
	Npc             *handler.NpcHandler
	RuntimeBbKey    *handler.RuntimeBbKeyHandler
}

// NewHandlers 一次性初始化所有 handler
func NewHandlers(st *Stores, svc *Services, mc *MemCaches, cfg *config.Config) *Handlers {
	return &Handlers{
		Field:           handler.NewFieldHandler(svc.Field, svc.Template, svc.FsmConfig, &cfg.Validation),
		Dict:            handler.NewDictionaryHandler(mc.Dict),
		Template:        handler.NewTemplateHandler(st.DB, svc.Template, svc.Field, svc.Npc, &cfg.Validation),
		EventType:       handler.NewEventTypeHandler(svc.EventType, &cfg.EventType),
		EventTypeSchema: handler.NewEventTypeSchemaHandler(svc.EventTypeSchema, svc.EventType, &cfg.EventTypeSchema),
		FsmConfig:       handler.NewFsmConfigHandler(st.DB, svc.FsmConfig, svc.Field, svc.EventTypeSchema, svc.Npc, svc.RuntimeBbKey, &cfg.FsmConfig),
		FsmStateDict:    handler.NewFsmStateDictHandler(svc.FsmStateDict, &cfg.FsmStateDict),
		BtTree:          handler.NewBtTreeHandler(st.DB, svc.BtTree, svc.Field, svc.EventTypeSchema, svc.Npc, svc.RuntimeBbKey, &cfg.BtTree),
		BtNodeType:      handler.NewBtNodeTypeHandler(svc.BtNodeType, &cfg.BtNodeType),
		Export:          handler.NewExportHandler(svc.EventType, svc.FsmConfig, svc.BtTree, svc.Npc),
		Npc:             handler.NewNpcHandler(svc.Npc, svc.Template, svc.Field, svc.FsmConfig, svc.BtTree, &cfg.Validation),
		RuntimeBbKey:    handler.NewRuntimeBbKeyHandler(svc.RuntimeBbKey, svc.FsmConfig, svc.BtTree, &cfg.Validation),
	}
}
