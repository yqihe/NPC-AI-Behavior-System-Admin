package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yqihe/npc-ai-admin/backend/internal/handler"
)

// Setup 注册所有路由
func Setup(r *gin.Engine, fh *handler.FieldHandler, dh *handler.DictionaryHandler, th *handler.TemplateHandler, eth *handler.EventTypeHandler, etsh *handler.EventTypeSchemaHandler, fch *handler.FsmConfigHandler, exh *handler.ExportHandler) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := r.Group("/api/v1")

	// 字段管理（8 个接口）
	fields := v1.Group("/fields")
	{
		fields.POST("/list", handler.WrapCtx(fh.List))
		fields.POST("/create", handler.WrapCtx(fh.Create))
		fields.POST("/detail", handler.WrapCtx(fh.Get))
		fields.POST("/update", handler.WrapCtx(fh.Update))
		fields.POST("/delete", handler.WrapCtx(fh.Delete))
		fields.POST("/references", handler.WrapCtx(fh.GetReferences))
		fields.POST("/check-name", handler.WrapCtx(fh.CheckName))
		fields.POST("/toggle-enabled", handler.WrapCtx(fh.ToggleEnabled))
	}

	// 模板管理（8 个接口）
	templates := v1.Group("/templates")
	{
		templates.POST("/list", handler.WrapCtx(th.List))
		templates.POST("/create", handler.WrapCtx(th.Create))
		templates.POST("/detail", handler.WrapCtx(th.Get))
		templates.POST("/update", handler.WrapCtx(th.Update))
		templates.POST("/delete", handler.WrapCtx(th.Delete))
		templates.POST("/check-name", handler.WrapCtx(th.CheckName))
		templates.POST("/references", handler.WrapCtx(th.GetReferences))
		templates.POST("/toggle-enabled", handler.WrapCtx(th.ToggleEnabled))
	}

	// 字典选项
	v1.POST("/dictionaries", handler.WrapCtx(dh.List))

	// 事件类型管理（7 个接口）
	eventTypes := v1.Group("/event-types")
	{
		eventTypes.POST("/list", handler.WrapCtx(eth.List))
		eventTypes.POST("/create", handler.WrapCtx(eth.Create))
		eventTypes.POST("/detail", handler.WrapCtx(eth.Get))
		eventTypes.POST("/update", handler.WrapCtx(eth.Update))
		eventTypes.POST("/delete", handler.WrapCtx(eth.Delete))
		eventTypes.POST("/check-name", handler.WrapCtx(eth.CheckName))
		eventTypes.POST("/toggle-enabled", handler.WrapCtx(eth.ToggleEnabled))
	}

	// 扩展字段 Schema 管理（5 个接口）
	eventTypeSchema := v1.Group("/event-type-schema")
	{
		eventTypeSchema.POST("/list", handler.WrapCtx(etsh.List))
		eventTypeSchema.POST("/create", handler.WrapCtx(etsh.Create))
		eventTypeSchema.POST("/update", handler.WrapCtx(etsh.Update))
		eventTypeSchema.POST("/delete", handler.WrapCtx(etsh.Delete))
		eventTypeSchema.POST("/toggle-enabled", handler.WrapCtx(etsh.ToggleEnabled))
	}

	// 状态机管理（7 个接口）
	fsmConfigs := v1.Group("/fsm-configs")
	{
		fsmConfigs.POST("/list", handler.WrapCtx(fch.List))
		fsmConfigs.POST("/create", handler.WrapCtx(fch.Create))
		fsmConfigs.POST("/detail", handler.WrapCtx(fch.Get))
		fsmConfigs.POST("/update", handler.WrapCtx(fch.Update))
		fsmConfigs.POST("/delete", handler.WrapCtx(fch.Delete))
		fsmConfigs.POST("/check-name", handler.WrapCtx(fch.CheckName))
		fsmConfigs.POST("/toggle-enabled", handler.WrapCtx(fch.ToggleEnabled))
	}

	// 配置导出 API
	configs := r.Group("/api/configs")
	{
		configs.GET("/event_types", exh.EventTypes)
		configs.GET("/fsm_configs", exh.FsmConfigs)
	}
}
