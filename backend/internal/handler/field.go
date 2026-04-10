package handler

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"unicode/utf8"

	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	"github.com/yqihe/npc-ai-admin/backend/internal/service"
)

var namePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// FieldHandler 字段管理业务处理
type FieldHandler struct {
	fieldService *service.FieldService
	valCfg       *config.ValidationConfig
}

// NewFieldHandler 创建 FieldHandler
func NewFieldHandler(fieldService *service.FieldService, valCfg *config.ValidationConfig) *FieldHandler {
	return &FieldHandler{fieldService: fieldService, valCfg: valCfg}
}

// ---- 前置校验（必填/格式/长度，不查 DB） ----

func (h *FieldHandler) checkName(name string) *errcode.Error {
	if name == "" {
		return errcode.Newf(errcode.ErrFieldNameInvalid, "字段标识不能为空")
	}
	if !namePattern.MatchString(name) {
		return errcode.New(errcode.ErrFieldNameInvalid)
	}
	if len(name) > h.valCfg.FieldNameMaxLength {
		return errcode.Newf(errcode.ErrFieldNameInvalid, "字段标识长度不能超过 %d 个字符", h.valCfg.FieldNameMaxLength)
	}
	return nil
}

func (h *FieldHandler) checkLabel(label string) *errcode.Error {
	if label == "" {
		return errcode.Newf(errcode.ErrBadRequest, "中文标签不能为空")
	}
	if utf8.RuneCountInString(label) > h.valCfg.FieldLabelMaxLength {
		return errcode.Newf(errcode.ErrBadRequest, "中文标签长度不能超过 %d 个字符", h.valCfg.FieldLabelMaxLength)
	}
	return nil
}

func checkRequired(value, fieldName string) *errcode.Error {
	if value == "" {
		return errcode.Newf(errcode.ErrBadRequest, "%s 不能为空", fieldName)
	}
	return nil
}

func checkID(id int64) *errcode.Error {
	if id <= 0 {
		return errcode.Newf(errcode.ErrBadRequest, "ID 不合法")
	}
	return nil
}

func checkVersion(version int) *errcode.Error {
	if version <= 0 {
		return errcode.Newf(errcode.ErrBadRequest, "版本号不合法")
	}
	return nil
}

func successMsg(msg string) *string {
	return &msg
}

// ---- 业务处理 ----

// List 字段列表
func (h *FieldHandler) List(ctx context.Context, req *model.FieldListQuery) (*model.ListData, error) {
	slog.Debug("handler.字段列表", "label", req.Label, "type", req.Type, "category", req.Category, "page", req.Page)

	return h.fieldService.List(ctx, req)
}

// Create 创建字段
func (h *FieldHandler) Create(ctx context.Context, req *model.CreateFieldRequest) (*model.CreateFieldResponse, error) {
	if err := h.checkName(req.Name); err != nil {
		return nil, err
	}
	if err := h.checkLabel(req.Label); err != nil {
		return nil, err
	}
	if err := checkRequired(req.Type, "字段类型"); err != nil {
		return nil, err
	}
	if err := checkRequired(req.Category, "标签分类"); err != nil {
		return nil, err
	}
	if req.Properties == nil {
		return nil, errcode.Newf(errcode.ErrBadRequest, "properties 不能为空")
	}

	slog.Debug("handler.创建字段", "name", req.Name, "type", req.Type, "category", req.Category)

	id, err := h.fieldService.Create(ctx, req)
	if err != nil {
		return nil, err
	}

	return &model.CreateFieldResponse{ID: id, Name: req.Name}, nil
}

// Get 字段详情（按 ID）
func (h *FieldHandler) Get(ctx context.Context, req *model.IDRequest) (*model.Field, error) {
	if err := checkID(req.ID); err != nil {
		return nil, err
	}

	slog.Debug("handler.字段详情", "id", req.ID)

	return h.fieldService.GetByID(ctx, req.ID)
}

// Update 编辑字段（按 ID）
func (h *FieldHandler) Update(ctx context.Context, req *model.UpdateFieldRequest) (*string, error) {
	if err := checkID(req.ID); err != nil {
		return nil, err
	}
	if err := h.checkLabel(req.Label); err != nil {
		return nil, err
	}
	if err := checkRequired(req.Type, "字段类型"); err != nil {
		return nil, err
	}
	if err := checkRequired(req.Category, "标签分类"); err != nil {
		return nil, err
	}
	if req.Properties == nil {
		return nil, errcode.Newf(errcode.ErrBadRequest, "properties 不能为空")
	}
	if err := checkVersion(req.Version); err != nil {
		return nil, err
	}

	slog.Debug("handler.编辑字段", "id", req.ID, "type", req.Type, "version", req.Version)

	err := h.fieldService.Update(ctx, req)
	if err != nil {
		return nil, err
	}

	return successMsg("保存成功"), nil
}

// Delete 软删除字段（按 ID）
func (h *FieldHandler) Delete(ctx context.Context, req *model.IDRequest) (*model.DeleteResult, error) {
	if err := checkID(req.ID); err != nil {
		return nil, err
	}

	slog.Debug("handler.删除字段", "id", req.ID)

	return h.fieldService.Delete(ctx, req.ID)
}

// CheckName 字段标识唯一性校验
func (h *FieldHandler) CheckName(ctx context.Context, req *model.CheckNameRequest) (*model.CheckNameResult, error) {
	if err := checkRequired(req.Name, "字段标识"); err != nil {
		return nil, err
	}

	slog.Debug("handler.校验字段名", "name", req.Name)

	return h.fieldService.CheckName(ctx, req.Name)
}

// GetReferences 字段引用详情（按 ID）
//
// 跨模块编排：FieldService 只返回字段模块内的数据（templates 数组只有 ID），
// handler 负责调用模板模块补齐 template label。
// TODO: 模板管理模块上线后，改为调用 templateService.GetByIDsLite(templateIDs)
//       批量取 label，替换下方的占位 fallback。
func (h *FieldHandler) GetReferences(ctx context.Context, req *model.IDRequest) (*model.ReferenceDetail, error) {
	if err := checkID(req.ID); err != nil {
		return nil, err
	}

	slog.Debug("handler.引用详情", "id", req.ID)

	detail, err := h.fieldService.GetReferences(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	// 跨模块补齐 template label（当前 templateService 未上线，使用占位）
	for i := range detail.Templates {
		if detail.Templates[i].Label == "" {
			detail.Templates[i].Label = fmt.Sprintf("模板#%d", detail.Templates[i].RefID)
		}
	}

	return detail, nil
}

// ToggleEnabled 切换启用/停用（按 ID）
func (h *FieldHandler) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) (*string, error) {
	if err := checkID(req.ID); err != nil {
		return nil, err
	}
	if err := checkVersion(req.Version); err != nil {
		return nil, err
	}

	slog.Debug("handler.切换启用", "id", req.ID, "enabled", req.Enabled)

	err := h.fieldService.ToggleEnabled(ctx, req)
	if err != nil {
		return nil, err
	}

	return successMsg("操作成功"), nil
}
