package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yqihe/npc-ai-admin/backend/internal/handler"
)

// Setup 注册所有路由
func Setup(r *gin.Engine, h *handler.Handlers) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := r.Group("/api/v1")

	// 字段管理（8 个接口）
	fields := v1.Group("/fields")
	{
		fields.POST("/list", handler.WrapCtx(h.Field.List))
		fields.POST("/create", handler.WrapCtx(h.Field.Create))
		fields.POST("/detail", handler.WrapCtx(h.Field.Get))
		fields.POST("/update", handler.WrapCtx(h.Field.Update))
		fields.POST("/delete", handler.WrapCtx(h.Field.Delete))
		fields.POST("/references", handler.WrapCtx(h.Field.GetReferences))
		fields.POST("/check-name", handler.WrapCtx(h.Field.CheckName))
		fields.POST("/toggle-enabled", handler.WrapCtx(h.Field.ToggleEnabled))
	}

	// 模板管理（8 个接口）
	templates := v1.Group("/templates")
	{
		templates.POST("/list", handler.WrapCtx(h.Template.List))
		templates.POST("/create", handler.WrapCtx(h.Template.Create))
		templates.POST("/detail", handler.WrapCtx(h.Template.Get))
		templates.POST("/update", handler.WrapCtx(h.Template.Update))
		templates.POST("/delete", handler.WrapCtx(h.Template.Delete))
		templates.POST("/check-name", handler.WrapCtx(h.Template.CheckName))
		templates.POST("/references", handler.WrapCtx(h.Template.GetReferences))
		templates.POST("/toggle-enabled", handler.WrapCtx(h.Template.ToggleEnabled))
	}

	// 字典选项
	v1.POST("/dictionaries", handler.WrapCtx(h.Dict.List))

	// 事件类型管理（7 个接口）
	eventTypes := v1.Group("/event-types")
	{
		eventTypes.POST("/list", handler.WrapCtx(h.EventType.List))
		eventTypes.POST("/create", handler.WrapCtx(h.EventType.Create))
		eventTypes.POST("/detail", handler.WrapCtx(h.EventType.Get))
		eventTypes.POST("/update", handler.WrapCtx(h.EventType.Update))
		eventTypes.POST("/delete", handler.WrapCtx(h.EventType.Delete))
		eventTypes.POST("/check-name", handler.WrapCtx(h.EventType.CheckName))
		eventTypes.POST("/toggle-enabled", handler.WrapCtx(h.EventType.ToggleEnabled))
	}

	// 扩展字段 Schema 管理（5 个接口）
	eventTypeSchema := v1.Group("/event-type-schema")
	{
		eventTypeSchema.POST("/list", handler.WrapCtx(h.EventTypeSchema.List))
		eventTypeSchema.POST("/create", handler.WrapCtx(h.EventTypeSchema.Create))
		eventTypeSchema.POST("/update", handler.WrapCtx(h.EventTypeSchema.Update))
		eventTypeSchema.POST("/delete", handler.WrapCtx(h.EventTypeSchema.Delete))
		eventTypeSchema.POST("/toggle-enabled", handler.WrapCtx(h.EventTypeSchema.ToggleEnabled))
	}

	// 状态机管理（7 个接口）
	fsmConfigs := v1.Group("/fsm-configs")
	{
		fsmConfigs.POST("/list", handler.WrapCtx(h.FsmConfig.List))
		fsmConfigs.POST("/create", handler.WrapCtx(h.FsmConfig.Create))
		fsmConfigs.POST("/detail", handler.WrapCtx(h.FsmConfig.Get))
		fsmConfigs.POST("/update", handler.WrapCtx(h.FsmConfig.Update))
		fsmConfigs.POST("/delete", handler.WrapCtx(h.FsmConfig.Delete))
		fsmConfigs.POST("/check-name", handler.WrapCtx(h.FsmConfig.CheckName))
		fsmConfigs.POST("/toggle-enabled", handler.WrapCtx(h.FsmConfig.ToggleEnabled))
	}

	// 配置导出 API
	configs := r.Group("/api/configs")
	{
		configs.GET("/event_types", h.Export.EventTypes)
		configs.GET("/fsm_configs", h.Export.FsmConfigs)
	}
}
