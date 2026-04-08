package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yqihe/npc-ai-admin/backend/internal/handler"
)

// Setup 注册所有路由
func Setup(r *gin.Engine, fh *handler.FieldHandler, dh *handler.DictionaryHandler) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := r.Group("/api/v1")

	// 字段管理
	fields := v1.Group("/fields")
	{
		fields.GET("/list", handler.WrapGet(fh.List))
		fields.POST("/create", handler.WrapPost(fh.Create))
		fields.POST("/detail", handler.WrapCtx(fh.Get))
		fields.POST("/update", handler.WrapCtx(fh.Update))
		fields.POST("/delete", handler.WrapCtx(fh.Delete))
		fields.POST("/references", handler.WrapCtx(fh.GetReferences))
		fields.POST("/check-name", handler.WrapCtx(fh.CheckName))
		fields.POST("/batch-delete", handler.WrapCtx(fh.BatchDelete))
		fields.POST("/batch-category", handler.WrapCtx(fh.BatchUpdateCategory))
	}

	// 字典选项
	v1.GET("/dictionaries", handler.WrapGet(dh.List))
}
