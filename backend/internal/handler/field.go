package handler

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	"github.com/yqihe/npc-ai-admin/backend/internal/service"
)

// FieldHandler 字段管理 HTTP handler
type FieldHandler struct {
	fieldService *service.FieldService
}

// NewFieldHandler 创建 FieldHandler
func NewFieldHandler(fieldService *service.FieldService) *FieldHandler {
	return &FieldHandler{fieldService: fieldService}
}

// RegisterRoutes 注册路由
func (h *FieldHandler) RegisterRoutes(r *gin.RouterGroup) {
	fields := r.Group("/fields")
	{
		fields.GET("", h.List)
	}
}

// List 字段列表
// GET /api/v1/fields?label=&type=&category=&page=1&page_size=20
func (h *FieldHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	q := &model.FieldListQuery{
		Label:    c.Query("label"),
		Type:     c.Query("type"),
		Category: c.Query("category"),
		Page:     page,
		PageSize: pageSize,
	}

	slog.Debug("handler.字段列表", "label", q.Label, "type", q.Type, "category", q.Category, "page", q.Page)

	data, err := h.fieldService.List(c.Request.Context(), q)
	if err != nil {
		slog.Error("handler.字段列表失败", "error", err)
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    50000,
			Message: "查询字段列表失败，请稍后重试",
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    0,
		Data:    data,
		Message: "success",
	})
}
