package handler

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	"github.com/yqihe/npc-ai-admin/backend/internal/service"
)

// FieldHandler 字段管理业务处理
type FieldHandler struct {
	fieldService *service.FieldService
}

// NewFieldHandler 创建 FieldHandler
func NewFieldHandler(fieldService *service.FieldService) *FieldHandler {
	return &FieldHandler{fieldService: fieldService}
}

// List 字段列表
func (h *FieldHandler) List(c *gin.Context) (any, error) {
	q := &model.FieldListQuery{
		Label:    c.Query("label"),
		Type:     c.Query("type"),
		Category: c.Query("category"),
	}
	fmt.Sscanf(c.DefaultQuery("page", "1"), "%d", &q.Page)
	fmt.Sscanf(c.DefaultQuery("page_size", "20"), "%d", &q.PageSize)

	slog.Debug("handler.字段列表", "label", q.Label, "type", q.Type, "category", q.Category, "page", q.Page)

	return h.fieldService.List(c.Request.Context(), q)
}

// Create 创建字段
func (h *FieldHandler) Create(c *gin.Context, req *model.CreateFieldRequest) (*model.CreateFieldResponse, error) {
	slog.Debug("handler.创建字段", "name", req.Name, "type", req.Type, "category", req.Category)

	id, err := h.fieldService.Create(c.Request.Context(), req)
	if err != nil {
		return nil, err
	}

	return &model.CreateFieldResponse{ID: id, Name: req.Name}, nil
}

// Get 字段详情
func (h *FieldHandler) Get(ctx context.Context, req *model.NameRequest) (*model.Field, error) {
	slog.Debug("handler.字段详情", "name", req.Name)

	return h.fieldService.GetByName(ctx, req.Name)
}

// Update 编辑字段
func (h *FieldHandler) Update(ctx context.Context, req *model.UpdateFieldRequest) (*string, error) {
	slog.Debug("handler.编辑字段", "name", req.Name, "type", req.Type, "version", req.Version)

	err := h.fieldService.Update(ctx, req.Name, req)
	if err != nil {
		return nil, err
	}

	msg := "保存成功"
	return &msg, nil
}

// Delete 删除字段
func (h *FieldHandler) Delete(ctx context.Context, req *model.NameRequest) (*service.DeleteResult, error) {
	slog.Debug("handler.删除字段", "name", req.Name)

	return h.fieldService.Delete(ctx, req.Name)
}

// CheckName 字段标识唯一性校验
func (h *FieldHandler) CheckName(ctx context.Context, req *model.CheckNameRequest) (*model.CheckNameResult, error) {
	slog.Debug("handler.校验字段名", "name", req.Name)

	return h.fieldService.CheckName(ctx, req.Name)
}

// GetReferences 字段引用详情
func (h *FieldHandler) GetReferences(ctx context.Context, req *model.NameRequest) (*model.ReferenceDetail, error) {
	slog.Debug("handler.引用详情", "name", req.Name)

	return h.fieldService.GetReferences(ctx, req.Name)
}

// BatchDelete 批量删除字段
func (h *FieldHandler) BatchDelete(ctx context.Context, req *model.BatchDeleteRequest) (*model.BatchDeleteResult, error) {
	slog.Debug("handler.批量删除", "names", req.Names, "count", len(req.Names))

	return h.fieldService.BatchDelete(ctx, req.Names)
}

// BatchUpdateCategory 批量修改分类
func (h *FieldHandler) BatchUpdateCategory(ctx context.Context, req *model.BatchCategoryRequest) (*model.BatchCategoryResponse, error) {
	slog.Debug("handler.批量修改分类", "names", req.Names, "category", req.Category)

	affected, err := h.fieldService.BatchUpdateCategory(ctx, req)
	if err != nil {
		return nil, err
	}

	return &model.BatchCategoryResponse{Affected: affected}, nil
}
