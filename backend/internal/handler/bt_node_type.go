package handler

import (
	shared "github.com/yqihe/npc-ai-admin/backend/internal/handler/shared"
	"context"
	"log/slog"

	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	"github.com/yqihe/npc-ai-admin/backend/internal/service"
)


// BtNodeTypeHandler 节点类型管理 HTTP handler
type BtNodeTypeHandler struct {
	svc     *service.BtNodeTypeService
	nodeCfg *config.BtNodeTypeConfig
}

// NewBtNodeTypeHandler 创建 BtNodeTypeHandler
func NewBtNodeTypeHandler(
	svc *service.BtNodeTypeService,
	nodeCfg *config.BtNodeTypeConfig,
) *BtNodeTypeHandler {
	return &BtNodeTypeHandler{svc: svc, nodeCfg: nodeCfg}
}

// ---- 接口实现 ----

// List 节点类型列表
func (h *BtNodeTypeHandler) List(ctx context.Context, req *model.BtNodeTypeListQuery) (*model.ListData, error) {
	slog.Debug("handler.节点类型列表", "type_name", req.TypeName, "label", req.Label, "category", req.Category)
	return h.svc.List(ctx, req)
}

// Create 创建节点类型
func (h *BtNodeTypeHandler) Create(ctx context.Context, req *model.CreateBtNodeTypeRequest) (*model.CreateBtNodeTypeResponse, error) {
	if err := shared.CheckName(req.TypeName, h.nodeCfg.NameMaxLength, errcode.ErrBtNodeTypeNameInvalid, "节点类型标识"); err != nil {
		return nil, err
	}
	if err := shared.CheckLabel(req.Label, h.nodeCfg.LabelMaxLength, "节点类型标签"); err != nil {
		return nil, err
	}
	slog.Debug("handler.创建节点类型", "type_name", req.TypeName)

	id, err := h.svc.Create(ctx, req)
	if err != nil {
		return nil, err
	}
	return &model.CreateBtNodeTypeResponse{ID: id, TypeName: req.TypeName}, nil
}

// Get 节点类型详情
func (h *BtNodeTypeHandler) Get(ctx context.Context, req *model.IDRequest) (*model.BtNodeType, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	slog.Debug("handler.节点类型详情", "id", req.ID)
	return h.svc.GetByID(ctx, req.ID)
}

// Update 编辑节点类型
func (h *BtNodeTypeHandler) Update(ctx context.Context, req *model.UpdateBtNodeTypeRequest) (*string, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	if err := shared.CheckVersion(req.Version); err != nil {
		return nil, err
	}
	if err := shared.CheckLabel(req.Label, h.nodeCfg.LabelMaxLength, "节点类型标签"); err != nil {
		return nil, err
	}
	slog.Debug("handler.编辑节点类型", "id", req.ID)

	if err := h.svc.Update(ctx, req); err != nil {
		return nil, err
	}
	return shared.SuccessMsg("保存成功"), nil
}

// Delete 删除节点类型
//
// 被引用时响应 44022，data 携带 BtNodeTypeDeleteResult（含 ReferencedBy 列表）。
// WrapCtx 在 err 非 nil 时自动将 resp 作为 data 输出（见 wrap.go writeError）。
func (h *BtNodeTypeHandler) Delete(ctx context.Context, req *model.IDRequest) (*model.BtNodeTypeDeleteResult, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	slog.Debug("handler.删除节点类型", "id", req.ID)

	return h.svc.Delete(ctx, req.ID)
}

// CheckName type_name 唯一性校验
func (h *BtNodeTypeHandler) CheckName(ctx context.Context, req *model.CheckNameRequest) (*model.CheckNameResult, error) {
	if err := shared.CheckName(req.Name, h.nodeCfg.NameMaxLength, errcode.ErrBtNodeTypeNameInvalid, "节点类型标识"); err != nil {
		return nil, err
	}
	slog.Debug("handler.校验节点类型标识", "type_name", req.Name)
	return h.svc.CheckName(ctx, req.Name)
}

// ToggleEnabled 切换启用/停用
func (h *BtNodeTypeHandler) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) (*string, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	if err := shared.CheckVersion(req.Version); err != nil {
		return nil, err
	}
	slog.Debug("handler.切换节点类型启用", "id", req.ID)

	if err := h.svc.ToggleEnabled(ctx, req); err != nil {
		return nil, err
	}
	return shared.SuccessMsg("操作成功"), nil
}
