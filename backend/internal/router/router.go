package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yqihe/npc-ai-admin/backend/internal/handler"
)

// Setup 注册所有路由
func Setup(r *gin.Engine, fieldHandler *handler.FieldHandler, dictHandler *handler.DictionaryHandler) {
	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := r.Group("/api/v1")

	// 字段管理 — 全部 GET/POST
	fields := v1.Group("/fields")
	{
		fields.GET("/list", fieldHandler.List)               // 列表
		fields.POST("/create", fieldHandler.Create)          // 新建
		fields.POST("/detail", fieldHandler.Get)             // 详情
		fields.POST("/update", fieldHandler.Update)          // 编辑
		fields.POST("/delete", fieldHandler.Delete)          // 删除
		fields.POST("/references", fieldHandler.GetReferences) // 引用详情
		fields.POST("/check-name", fieldHandler.CheckName)   // 唯一性校验
		fields.POST("/batch-delete", fieldHandler.BatchDelete) // 批量删除
		fields.POST("/batch-category", fieldHandler.BatchUpdateCategory) // 批量修改分类
	}

	// 字典选项
	v1.GET("/dictionaries", dictHandler.List)
}
