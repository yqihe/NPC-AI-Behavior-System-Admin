package handler

import (
	"errors"
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
		fields.POST("", h.Create)
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

// Create 创建字段
// POST /api/v1/fields
func (h *FieldHandler) Create(c *gin.Context) {
	var req model.CreateFieldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Debug("handler.创建字段-参数解析失败", "error", err)
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    40002,
			Message: "请求参数格式错误",
		})
		return
	}

	slog.Debug("handler.创建字段", "name", req.Name, "type", req.Type, "category", req.Category)

	id, err := h.fieldService.Create(c.Request.Context(), &req)
	if err != nil {
		var svcErr *service.ServiceError
		if errors.As(err, &svcErr) {
			c.JSON(http.StatusBadRequest, model.Response{
				Code:    svcErr.Code,
				Message: svcErr.Message,
			})
			return
		}
		slog.Error("handler.创建字段失败", "error", err)
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    50000,
			Message: "创建字段失败，请稍后重试",
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    0,
		Data:    gin.H{"id": id, "name": req.Name},
		Message: "创建成功",
	})
}
