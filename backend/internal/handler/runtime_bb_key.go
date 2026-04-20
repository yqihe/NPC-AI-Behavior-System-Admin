package handler

import (
	"context"
	"log/slog"

	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	"github.com/yqihe/npc-ai-admin/backend/internal/service"
	shared "github.com/yqihe/npc-ai-admin/backend/internal/handler/shared"
)

// RuntimeBbKeyHandler 运行时 BB Key 业务处理
type RuntimeBbKeyHandler struct {
	runtimeBbKeyService *service.RuntimeBbKeyService
	fsmConfigService    *service.FsmConfigService // 跨模块编排：GetReferences 补 FSM display_name
	btTreeService       *service.BtTreeService    // 跨模块编排：GetReferences 补 BT display_name
	valCfg              *config.ValidationConfig
}

// NewRuntimeBbKeyHandler 创建 RuntimeBbKeyHandler
//
// name/label 长度约束复用 FieldNameMaxLength / FieldLabelMaxLength（同为 VARCHAR(64) 限制，
// 见 migration 014；不单独加配置项避免配置膨胀，对齐 red-lines/general §禁止过度设计）。
func NewRuntimeBbKeyHandler(
	runtimeBbKeyService *service.RuntimeBbKeyService,
	fsmConfigService *service.FsmConfigService,
	btTreeService *service.BtTreeService,
	valCfg *config.ValidationConfig,
) *RuntimeBbKeyHandler {
	return &RuntimeBbKeyHandler{
		runtimeBbKeyService: runtimeBbKeyService,
		fsmConfigService:    fsmConfigService,
		btTreeService:       btTreeService,
		valCfg:              valCfg,
	}
}

// List 运行时 BB Key 列表
func (h *RuntimeBbKeyHandler) List(ctx context.Context, req *model.RuntimeBbKeyListQuery) (*model.ListData, error) {
	slog.Debug("handler.运行时BBKey列表", "name", req.Name, "label", req.Label, "type", req.Type, "group", req.GroupName, "page", req.Page)
	return h.runtimeBbKeyService.List(ctx, req)
}

// Create 创建运行时 BB Key
func (h *RuntimeBbKeyHandler) Create(ctx context.Context, req *model.CreateRuntimeBbKeyRequest) (*model.CreateFieldResponse, error) {
	if err := shared.CheckName(req.Name, h.valCfg.FieldNameMaxLength, errcode.ErrRuntimeBBKeyNameInvalid, "运行时 BB Key 标识"); err != nil {
		return nil, err
	}
	if err := shared.CheckLabel(req.Label, h.valCfg.FieldLabelMaxLength, "中文标签"); err != nil {
		return nil, err
	}
	if err := shared.CheckRequired(req.Type, "类型"); err != nil {
		return nil, err
	}
	if err := shared.CheckRequired(req.GroupName, "分组"); err != nil {
		return nil, err
	}

	slog.Debug("handler.创建运行时BBKey", "name", req.Name, "type", req.Type, "group", req.GroupName)

	id, err := h.runtimeBbKeyService.Create(ctx, req)
	if err != nil {
		return nil, err
	}
	return &model.CreateFieldResponse{ID: id, Name: req.Name}, nil
}

// Get 运行时 BB Key 详情（按 ID）
func (h *RuntimeBbKeyHandler) Get(ctx context.Context, req *model.IDRequest) (*model.RuntimeBbKey, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	slog.Debug("handler.运行时BBKey详情", "id", req.ID)
	return h.runtimeBbKeyService.GetByID(ctx, req.ID)
}

// Update 编辑运行时 BB Key（按 ID，name 不可变）
func (h *RuntimeBbKeyHandler) Update(ctx context.Context, req *model.UpdateRuntimeBbKeyRequest) (*string, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	if err := shared.CheckVersion(req.Version); err != nil {
		return nil, err
	}
	if err := shared.CheckLabel(req.Label, h.valCfg.FieldLabelMaxLength, "中文标签"); err != nil {
		return nil, err
	}
	if err := shared.CheckRequired(req.Type, "类型"); err != nil {
		return nil, err
	}
	if err := shared.CheckRequired(req.GroupName, "分组"); err != nil {
		return nil, err
	}

	slog.Debug("handler.编辑运行时BBKey", "id", req.ID, "type", req.Type, "version", req.Version)

	if err := h.runtimeBbKeyService.Update(ctx, req); err != nil {
		return nil, err
	}
	return shared.SuccessMsg("保存成功"), nil
}

// Delete 软删除运行时 BB Key（按 ID）
func (h *RuntimeBbKeyHandler) Delete(ctx context.Context, req *model.IDRequest) (*model.DeleteResult, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	slog.Debug("handler.删除运行时BBKey", "id", req.ID)
	return h.runtimeBbKeyService.Delete(ctx, req.ID)
}

// CheckName 运行时 BB Key 标识唯一性校验（跨表：fields + runtime_bb_keys 双向）
func (h *RuntimeBbKeyHandler) CheckName(ctx context.Context, req *model.CheckNameRequest) (*model.CheckNameResult, error) {
	if err := shared.CheckName(req.Name, h.valCfg.FieldNameMaxLength, errcode.ErrRuntimeBBKeyNameInvalid, "运行时 BB Key 标识"); err != nil {
		return nil, err
	}

	slog.Debug("handler.校验运行时BBKey名", "name", req.Name)

	conflict, source, err := h.runtimeBbKeyService.CheckName(ctx, req.Name)
	if err != nil {
		return nil, err
	}
	if !conflict {
		return &model.CheckNameResult{Available: true, Message: "该标识可用"}, nil
	}
	msg := "该标识已存在"
	switch source {
	case "field":
		msg = "该标识与字段标识冲突"
	case "runtime_bb_key":
		msg = "该运行时 BB Key 标识已存在"
	}
	return &model.CheckNameResult{Available: false, Message: msg}, nil
}

// GetReferences 运行时 BB Key 引用详情（按 ID）
//
// 跨模块编排：runtimeBbKeyService 只返回 ID 列表，
// handler 调 fsmConfigService/btTreeService 补齐 FSM/BT 的 display_name。
func (h *RuntimeBbKeyHandler) GetReferences(ctx context.Context, req *model.IDRequest) (*model.RuntimeBbKeyReferenceDetail, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	slog.Debug("handler.运行时BBKey引用详情", "id", req.ID)

	detail, err := h.runtimeBbKeyService.GetReferences(ctx, req.ID)
	if err != nil {
		return nil, err
	}

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

	for i := range detail.Bts {
		bt, err := h.btTreeService.GetByID(ctx, detail.Bts[i].RefID)
		if err != nil {
			slog.Warn("handler.补BT_label失败", "error", err, "bt_id", detail.Bts[i].RefID)
			continue
		}
		if bt != nil {
			detail.Bts[i].Label = bt.DisplayName
		}
	}

	return detail, nil
}

// ToggleEnabled 切换启用/停用（按 ID）
func (h *RuntimeBbKeyHandler) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) (*string, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	if err := shared.CheckVersion(req.Version); err != nil {
		return nil, err
	}
	slog.Debug("handler.切换运行时BBKey启用", "id", req.ID, "enabled", req.Enabled)

	if err := h.runtimeBbKeyService.ToggleEnabled(ctx, req); err != nil {
		return nil, err
	}
	return shared.SuccessMsg("操作成功"), nil
}
