package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

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

// respondError 统一错误响应
func respondError(c *gin.Context, err error) {
	var ecErr *errcode.Error
	if errors.As(err, &ecErr) {
		c.JSON(http.StatusOK, model.Response{
			Code:    ecErr.Code,
			Message: ecErr.Message,
		})
		return
	}
	slog.Error("handler.内部错误", "error", err)
	c.JSON(http.StatusOK, model.Response{
		Code:    errcode.ErrInternal,
		Message: errcode.Msg(errcode.ErrInternal),
	})
}

// respondOK 统一成功响应
func respondOK(c *gin.Context, data any, message string) {
	c.JSON(http.StatusOK, model.Response{
		Code:    errcode.Success,
		Data:    data,
		Message: message,
	})
}

// respondBadRequest 参数解析失败
func respondBadRequest(c *gin.Context, err error) {
	slog.Debug("handler.参数解析失败", "error", err)
	c.JSON(http.StatusOK, model.Response{
		Code:    errcode.ErrBadRequest,
		Message: "请求参数格式错误",
	})
}

// List 字段列表
// GET /api/v1/fields/list?label=&type=&category=&page=1&page_size=20
func (h *FieldHandler) List(c *gin.Context) {
	q := &model.FieldListQuery{
		Label:    c.Query("label"),
		Type:     c.Query("type"),
		Category: c.Query("category"),
	}
	// GET 请求的分页参数从 query string 取
	fmt.Sscanf(c.DefaultQuery("page", "1"), "%d", &q.Page)
	fmt.Sscanf(c.DefaultQuery("page_size", "20"), "%d", &q.PageSize)

	slog.Debug("handler.字段列表", "label", q.Label, "type", q.Type, "category", q.Category, "page", q.Page)

	data, err := h.fieldService.List(c.Request.Context(), q)
	if err != nil {
		respondError(c, err)
		return
	}

	respondOK(c, data, errcode.Msg(errcode.Success))
}

// Create 创建字段
// POST /api/v1/fields/create
func (h *FieldHandler) Create(c *gin.Context) {
	var req model.CreateFieldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}

	slog.Debug("handler.创建字段", "name", req.Name, "type", req.Type, "category", req.Category)

	id, err := h.fieldService.Create(c.Request.Context(), &req)
	if err != nil {
		respondError(c, err)
		return
	}

	respondOK(c, gin.H{"id": id, "name": req.Name}, "创建成功")
}

// Get 字段详情
// POST /api/v1/fields/detail
func (h *FieldHandler) Get(c *gin.Context) {
	var req model.NameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}

	slog.Debug("handler.字段详情", "name", req.Name)

	field, err := h.fieldService.GetByName(c.Request.Context(), req.Name)
	if err != nil {
		respondError(c, err)
		return
	}

	respondOK(c, field, errcode.Msg(errcode.Success))
}

// Update 编辑字段
// POST /api/v1/fields/update
func (h *FieldHandler) Update(c *gin.Context) {
	var req model.UpdateFieldRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}

	slog.Debug("handler.编辑字段", "name", req.Name, "type", req.Type, "version", req.Version)

	err := h.fieldService.Update(c.Request.Context(), req.Name, &req)
	if err != nil {
		respondError(c, err)
		return
	}

	respondOK(c, nil, "保存成功")
}

// Delete 删除字段
// POST /api/v1/fields/delete
func (h *FieldHandler) Delete(c *gin.Context) {
	var req model.NameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}

	slog.Debug("handler.删除字段", "name", req.Name)

	result, err := h.fieldService.Delete(c.Request.Context(), req.Name)
	if err != nil {
		// 被引用时 err 是 errcode.Error，但 result 也有值（引用列表）
		var ecErr *errcode.Error
		if errors.As(err, &ecErr) {
			c.JSON(http.StatusOK, model.Response{
				Code:    ecErr.Code,
				Message: ecErr.Message,
				Data:    result,
			})
			return
		}
		respondError(c, err)
		return
	}

	respondOK(c, result, "删除成功")
}

// CheckName 字段标识唯一性校验
// POST /api/v1/fields/check-name
func (h *FieldHandler) CheckName(c *gin.Context) {
	var req model.CheckNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}

	slog.Debug("handler.校验字段名", "name", req.Name)

	result, err := h.fieldService.CheckName(c.Request.Context(), req.Name)
	if err != nil {
		respondError(c, err)
		return
	}

	respondOK(c, result, errcode.Msg(errcode.Success))
}

// GetReferences 字段引用详情
// POST /api/v1/fields/references
func (h *FieldHandler) GetReferences(c *gin.Context) {
	var req model.NameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}

	slog.Debug("handler.引用详情", "name", req.Name)

	detail, err := h.fieldService.GetReferences(c.Request.Context(), req.Name)
	if err != nil {
		respondError(c, err)
		return
	}

	respondOK(c, detail, errcode.Msg(errcode.Success))
}

// BatchDelete 批量删除字段
// POST /api/v1/fields/batch-delete
func (h *FieldHandler) BatchDelete(c *gin.Context) {
	var req model.BatchDeleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}

	slog.Debug("handler.批量删除", "names", req.Names, "count", len(req.Names))

	result, err := h.fieldService.BatchDelete(c.Request.Context(), req.Names)
	if err != nil {
		respondError(c, err)
		return
	}

	respondOK(c, result, result.Message)
}

// BatchUpdateCategory 批量修改分类
// POST /api/v1/fields/batch-category
func (h *FieldHandler) BatchUpdateCategory(c *gin.Context) {
	var req model.BatchCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}

	slog.Debug("handler.批量修改分类", "names", req.Names, "category", req.Category)

	affected, err := h.fieldService.BatchUpdateCategory(c.Request.Context(), &req)
	if err != nil {
		respondError(c, err)
		return
	}

	respondOK(c, gin.H{"affected": affected}, fmt.Sprintf("%d 项已更新", affected))
}
