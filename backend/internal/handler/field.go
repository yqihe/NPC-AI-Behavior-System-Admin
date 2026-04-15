package handler

import (
	shared "github.com/yqihe/npc-ai-admin/backend/internal/handler/shared"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	"github.com/yqihe/npc-ai-admin/backend/internal/service"
)

// FieldHandler 字段管理业务处理
type FieldHandler struct {
	fieldService     *service.FieldService
	templateService  *service.TemplateService  // 跨模块编排：GetReferences 补 template label
	fsmConfigService *service.FsmConfigService // 跨模块编排：GetReferences 补 FSM display_name
	valCfg           *config.ValidationConfig
}

// NewFieldHandler 创建 FieldHandler
func NewFieldHandler(
	fieldService *service.FieldService,
	templateService *service.TemplateService,
	fsmConfigService *service.FsmConfigService,
	valCfg *config.ValidationConfig,
) *FieldHandler {
	return &FieldHandler{
		fieldService:     fieldService,
		templateService:  templateService,
		fsmConfigService: fsmConfigService,
		valCfg:           valCfg,
	}
}

// ---- 前置校验（必填/格式/长度，不查 DB） ----

// checkPropertiesShape 校验 properties 原始字节必须是 JSON 对象（首字符 '{'）。
// 防御客户端传 null / [] / "foo" / 123 / true 这类非对象形状；
// json.RawMessage 对 `null` 不会变 nil，是 []byte("null")，所以需要单独拦。
func checkPropertiesShape(raw json.RawMessage) *errcode.Error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return errcode.Newf(errcode.ErrBadRequest, "properties 不能为空")
	}
	if trimmed[0] != '{' {
		return errcode.Newf(errcode.ErrBadRequest, "properties 必须是 JSON 对象")
	}
	return nil
}

// ---- 业务处理 ----

// List 字段列表
func (h *FieldHandler) List(ctx context.Context, req *model.FieldListQuery) (*model.ListData, error) {
	slog.Debug("handler.字段列表", "label", req.Label, "type", req.Type, "category", req.Category, "page", req.Page)

	return h.fieldService.List(ctx, req)
}

// Create 创建字段
func (h *FieldHandler) Create(ctx context.Context, req *model.CreateFieldRequest) (*model.CreateFieldResponse, error) {
	if err := shared.CheckName(req.Name, h.valCfg.FieldNameMaxLength, errcode.ErrFieldNameInvalid, "字段标识"); err != nil {
		return nil, err
	}
	if err := shared.CheckLabel(req.Label, h.valCfg.FieldLabelMaxLength, "中文标签"); err != nil {
		return nil, err
	}
	if err := shared.CheckRequired(req.Type, "字段类型"); err != nil {
		return nil, err
	}
	if err := shared.CheckRequired(req.Category, "标签分类"); err != nil {
		return nil, err
	}
	if err := checkPropertiesShape(req.Properties); err != nil {
		return nil, err
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
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}

	slog.Debug("handler.字段详情", "id", req.ID)

	return h.fieldService.GetByID(ctx, req.ID)
}

// Update 编辑字段（按 ID）
func (h *FieldHandler) Update(ctx context.Context, req *model.UpdateFieldRequest) (*string, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	if err := shared.CheckVersion(req.Version); err != nil {
		return nil, err
	}
	if err := shared.CheckLabel(req.Label, h.valCfg.FieldLabelMaxLength, "中文标签"); err != nil {
		return nil, err
	}
	if err := shared.CheckRequired(req.Type, "字段类型"); err != nil {
		return nil, err
	}
	if err := shared.CheckRequired(req.Category, "标签分类"); err != nil {
		return nil, err
	}
	if err := checkPropertiesShape(req.Properties); err != nil {
		return nil, err
	}

	slog.Debug("handler.编辑字段", "id", req.ID, "type", req.Type, "version", req.Version)

	err := h.fieldService.Update(ctx, req)
	if err != nil {
		return nil, err
	}

	return shared.SuccessMsg("保存成功"), nil
}

// Delete 软删除字段（按 ID）
func (h *FieldHandler) Delete(ctx context.Context, req *model.IDRequest) (*model.DeleteResult, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}

	slog.Debug("handler.删除字段", "id", req.ID)

	return h.fieldService.Delete(ctx, req.ID)
}

// CheckName 字段标识唯一性校验（先校验格式/长度，再查 DB）
func (h *FieldHandler) CheckName(ctx context.Context, req *model.CheckNameRequest) (*model.CheckNameResult, error) {
	if err := shared.CheckName(req.Name, h.valCfg.FieldNameMaxLength, errcode.ErrFieldNameInvalid, "字段标识"); err != nil {
		return nil, err
	}

	slog.Debug("handler.校验字段名", "name", req.Name)

	return h.fieldService.CheckName(ctx, req.Name)
}

// GetReferences 字段引用详情（按 ID）
//
// 跨模块编排：FieldService 只返回字段模块内的数据（templates 数组只有 RefID 不带 Label），
// handler 调 templateService.GetByIDsLite 跨模块补齐 template label。
func (h *FieldHandler) GetReferences(ctx context.Context, req *model.IDRequest) (*model.ReferenceDetail, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}

	slog.Debug("handler.引用详情", "id", req.ID)

	detail, err := h.fieldService.GetReferences(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	// 跨模块补齐 template label
	if len(detail.Templates) > 0 {
		templateIDs := make([]int64, 0, len(detail.Templates))
		for _, t := range detail.Templates {
			templateIDs = append(templateIDs, t.RefID)
		}
		tplLites, err := h.templateService.GetByIDsLite(ctx, templateIDs)
		if err != nil {
			slog.Error("handler.补模板label失败", "error", err, "ids", templateIDs)
			return nil, fmt.Errorf("get template lites: %w", err)
		}
		labelMap := make(map[int64]string, len(tplLites))
		for _, t := range tplLites {
			labelMap[t.ID] = t.Label
		}
		for i := range detail.Templates {
			refID := detail.Templates[i].RefID
			if label, ok := labelMap[refID]; ok {
				detail.Templates[i].Label = label
			} else {
				// 引用的模板已被删除（理论上不应发生，因为字段被引用时模板不能删）
				slog.Warn("handler.引用详情模板缺失", "field_id", req.ID, "template_id", refID)
			}
		}
	}

	// 跨模块补齐 FSM display_name
	if len(detail.Fsms) > 0 {
		for i := range detail.Fsms {
			fc, err := h.fsmConfigService.GetByID(ctx, detail.Fsms[i].RefID)
			if err != nil {
				slog.Warn("handler.补FSM_label失败", "error", err, "fsm_id", detail.Fsms[i].RefID)
				continue
			}
			if fc != nil {
				detail.Fsms[i].Label = fc.DisplayName
			}
		}
	}

	return detail, nil
}

// ToggleEnabled 切换启用/停用（按 ID）
func (h *FieldHandler) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) (*string, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	if err := shared.CheckVersion(req.Version); err != nil {
		return nil, err
	}

	slog.Debug("handler.切换启用", "id", req.ID, "enabled", req.Enabled)

	err := h.fieldService.ToggleEnabled(ctx, req)
	if err != nil {
		return nil, err
	}

	return shared.SuccessMsg("操作成功"), nil
}
