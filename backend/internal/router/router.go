package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yqihe/npc-ai-admin/backend/internal/handler"
)

// Setup 注册所有路由
func Setup(r *gin.Engine, fh *handler.FieldHandler, dh *handler.DictionaryHandler, th *handler.TemplateHandler) {
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
}
