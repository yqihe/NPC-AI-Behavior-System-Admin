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
	FsmStateDict    *service.FsmStateDictService
	BtTree          *service.BtTreeService
	BtNodeType      *service.BtNodeTypeService
	Npc             *service.NpcService
	RuntimeBbKey    *service.RuntimeBbKeyService
	Region          *service.RegionService
}

// NewServices 一次性初始化所有 service
//
// RegionService 依赖 NpcService（见 validateSpawnTable 的 template_ref 校验），
// 所以必须在 NpcService 构造之后再造，用两段式赋值。
func NewServices(st *Stores, rc *Caches, mc *MemCaches, cfg *config.Config) *Services {
	npcSvc := service.NewNpcService(st.Npc, rc.Npc, &cfg.Pagination)
	return &Services{
		Field:           service.NewFieldService(st.Field, st.FieldRef, rc.Field, mc.Dict, st.RuntimeBbKey, &cfg.Pagination),
		Template:        service.NewTemplateService(st.Template, rc.Template, &cfg.Pagination),
		EventType:       service.NewEventTypeService(st.EventType, st.EventTypeSchema, st.SchemaRef, rc.EventType, mc.EventTypeSchema, &cfg.Pagination, &cfg.EventType),
		EventTypeSchema: service.NewEventTypeSchemaService(st.EventTypeSchema, st.SchemaRef, mc.EventTypeSchema, &cfg.EventTypeSchema, &cfg.Pagination),
		FsmConfig:       service.NewFsmConfigService(st.FsmConfig, st.FsmStateDict, rc.FsmConfig, &cfg.Pagination, &cfg.FsmConfig),
		FsmStateDict:    service.NewFsmStateDictService(st.FsmStateDict, st.FsmConfig, rc.FsmStateDict, mc.Dict, &cfg.Pagination, &cfg.FsmStateDict),
		BtTree:          service.NewBtTreeService(st.BtTree, st.BtNodeType, rc.BtTree, &cfg.Pagination, &cfg.BtTree),
		BtNodeType:      service.NewBtNodeTypeService(st.BtNodeType, st.BtTree, rc.BtNodeType, &cfg.Pagination, &cfg.BtNodeType),
		Npc:             npcSvc,
		RuntimeBbKey:    service.NewRuntimeBbKeyService(st.RuntimeBbKey, st.RuntimeBbKeyRef, rc.RuntimeBbKey, st.Field, &cfg.Pagination),
		Region:          service.NewRegionService(st.Region, rc.Region, npcSvc, &cfg.Pagination),
	}
}
