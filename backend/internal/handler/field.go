package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
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
		fields.GET("/:name", h.Get)
		fields.PUT("/:name", h.Update)
		fields.DELETE("/:name", h.Delete)
		fields.GET("/:name/references", h.GetReferences)
		fields.POST("/check-name", h.CheckName)
		fields.POST("/batch-delete", h.BatchDelete)
		fields.PUT("/batch-category", h.BatchUpdateCategory)
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
			Code:    errcode.ErrInternal,
			Message: errcode.Msg(errcode.ErrInternal),
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    errcode.Success,
		Data:    data,
		Message: errcode.Msg(errcode.Success),
	})
}

// Create 创建字段
// POST /api/v1/fields
func (h *FieldHandler) Create(c *gin.Context) {
	var req model.CreateFieldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Debug("handler.创建字段-参数解析失败", "error", err)
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    errcode.ErrBadRequest,
			Message: "请求参数格式错误",
		})
		return
	}

	slog.Debug("handler.创建字段", "name", req.Name, "type", req.Type, "category", req.Category)

	id, err := h.fieldService.Create(c.Request.Context(), &req)
	if err != nil {
		var ecErr *errcode.Error
		if errors.As(err, &ecErr) {
			c.JSON(http.StatusBadRequest, model.Response{
				Code:    ecErr.Code,
				Message: ecErr.Message,
			})
			return
		}
		slog.Error("handler.创建字段失败", "error", err)
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    errcode.ErrInternal,
			Message: errcode.Msg(errcode.ErrInternal),
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    errcode.Success,
		Data:    gin.H{"id": id, "name": req.Name},
		Message: "创建成功",
	})
}

// Get 字段详情
// GET /api/v1/fields/:name
func (h *FieldHandler) Get(c *gin.Context) {
	name := c.Param("name")

	slog.Debug("handler.字段详情", "name", name)

	field, err := h.fieldService.GetByName(c.Request.Context(), name)
	if err != nil {
		var ecErr *errcode.Error
		if errors.As(err, &ecErr) {
			c.JSON(http.StatusNotFound, model.Response{
				Code:    ecErr.Code,
				Message: ecErr.Message,
			})
			return
		}
		slog.Error("handler.字段详情失败", "error", err)
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    errcode.ErrInternal,
			Message: errcode.Msg(errcode.ErrInternal),
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    errcode.Success,
		Data:    field,
		Message: errcode.Msg(errcode.Success),
	})
}

// Update 编辑字段
// PUT /api/v1/fields/:name
func (h *FieldHandler) Update(c *gin.Context) {
	name := c.Param("name")

	var req model.UpdateFieldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Debug("handler.编辑字段-参数解析失败", "error", err)
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    errcode.ErrBadRequest,
			Message: "请求参数格式错误",
		})
		return
	}

	slog.Debug("handler.编辑字段", "name", name, "type", req.Type, "version", req.Version)

	err := h.fieldService.Update(c.Request.Context(), name, &req)
	if err != nil {
		var ecErr *errcode.Error
		if errors.As(err, &ecErr) {
			c.JSON(http.StatusBadRequest, model.Response{
				Code:    ecErr.Code,
				Message: ecErr.Message,
			})
			return
		}
		slog.Error("handler.编辑字段失败", "error", err)
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    errcode.ErrInternal,
			Message: errcode.Msg(errcode.ErrInternal),
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    errcode.Success,
		Message: "保存成功",
	})
}

// Delete 删除字段
// DELETE /api/v1/fields/:name
func (h *FieldHandler) Delete(c *gin.Context) {
	name := c.Param("name")

	slog.Debug("handler.删除字段", "name", name)

	result, err := h.fieldService.Delete(c.Request.Context(), name)
	if err != nil {
		var ecErr *errcode.Error
		if errors.As(err, &ecErr) {
			// 被引用禁止删除 → 返回引用列表
			c.JSON(http.StatusBadRequest, model.Response{
				Code:    ecErr.Code,
				Message: ecErr.Message,
				Data:    result,
			})
			return
		}
		slog.Error("handler.删除字段失败", "error", err)
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    errcode.ErrInternal,
			Message: errcode.Msg(errcode.ErrInternal),
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    errcode.Success,
		Message: "删除成功",
	})
}

// CheckName 字段标识唯一性校验
// POST /api/v1/fields/check-name
func (h *FieldHandler) CheckName(c *gin.Context) {
	var req model.CheckNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    errcode.ErrBadRequest,
			Message: "请求参数格式错误",
		})
		return
	}

	slog.Debug("handler.校验字段名", "name", req.Name)

	result, err := h.fieldService.CheckName(c.Request.Context(), req.Name)
	if err != nil {
		slog.Error("handler.校验字段名失败", "error", err)
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    errcode.ErrInternal,
			Message: errcode.Msg(errcode.ErrInternal),
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    errcode.Success,
		Data:    result,
		Message: errcode.Msg(errcode.Success),
	})
}

// GetReferences 字段引用详情
// GET /api/v1/fields/:name/references
func (h *FieldHandler) GetReferences(c *gin.Context) {
	name := c.Param("name")

	slog.Debug("handler.引用详情", "name", name)

	detail, err := h.fieldService.GetReferences(c.Request.Context(), name)
	if err != nil {
		var ecErr *errcode.Error
		if errors.As(err, &ecErr) {
			c.JSON(http.StatusNotFound, model.Response{
				Code:    ecErr.Code,
				Message: ecErr.Message,
			})
			return
		}
		slog.Error("handler.引用详情失败", "error", err)
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    errcode.ErrInternal,
			Message: errcode.Msg(errcode.ErrInternal),
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    errcode.Success,
		Data:    detail,
		Message: errcode.Msg(errcode.Success),
	})
}

// BatchDelete 批量删除字段
// POST /api/v1/fields/batch-delete
func (h *FieldHandler) BatchDelete(c *gin.Context) {
	var req model.BatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    errcode.ErrBadRequest,
			Message: "请求参数格式错误",
		})
		return
	}

	slog.Debug("handler.批量删除", "names", req.Names, "count", len(req.Names))

	result, err := h.fieldService.BatchDelete(c.Request.Context(), req.Names)
	if err != nil {
		slog.Error("handler.批量删除失败", "error", err)
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    errcode.ErrInternal,
			Message: errcode.Msg(errcode.ErrInternal),
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    errcode.Success,
		Data:    result,
		Message: result.Message,
	})
}

// BatchUpdateCategory 批量修改分类
// PUT /api/v1/fields/batch-category
func (h *FieldHandler) BatchUpdateCategory(c *gin.Context) {
	var req model.BatchCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{
			Code:    errcode.ErrBadRequest,
			Message: "请求参数格式错误",
		})
		return
	}

	slog.Debug("handler.批量修改分类", "names", req.Names, "category", req.Category)

	affected, err := h.fieldService.BatchUpdateCategory(c.Request.Context(), &req)
	if err != nil {
		var ecErr *errcode.Error
		if errors.As(err, &ecErr) {
			c.JSON(http.StatusBadRequest, model.Response{
				Code:    ecErr.Code,
				Message: ecErr.Message,
			})
			return
		}
		slog.Error("handler.批量修改分类失败", "error", err)
		c.JSON(http.StatusInternalServerError, model.Response{
			Code:    errcode.ErrInternal,
			Message: errcode.Msg(errcode.ErrInternal),
		})
		return
	}

	c.JSON(http.StatusOK, model.Response{
		Code:    errcode.Success,
		Data:    gin.H{"affected": affected},
		Message: fmt.Sprintf("%d 项已更新", affected),
	})
}
