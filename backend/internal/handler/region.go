package handler

import (
	"context"
	"log/slog"

	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	shared "github.com/yqihe/npc-ai-admin/backend/internal/handler/shared"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	"github.com/yqihe/npc-ai-admin/backend/internal/service"
)

// RegionHandler 区域管理 HTTP handler
//
// region_id 复用 shared.IdentPattern（^[a-z][a-z0-9_]*$），错误码独占 ErrRegionIDInvalid。
// display_name 复用 CheckLabel（UTF-8 字符数限）。
// 所有 7 端点走 WrapCtx 泛型包装，请求/响应约定见 router.go。
type RegionHandler struct {
	svc       *service.RegionService
	regionCfg *config.RegionConfig
}

// NewRegionHandler 创建 RegionHandler
func NewRegionHandler(svc *service.RegionService, regionCfg *config.RegionConfig) *RegionHandler {
	return &RegionHandler{svc: svc, regionCfg: regionCfg}
}

// checkRegionID 复用 shared.CheckName 做 region_id 校验，错误码映射到 47002
func checkRegionID(regionID string, maxLen int) *errcode.Error {
	return shared.CheckName(regionID, maxLen, errcode.ErrRegionIDInvalid, "区域标识")
}

// ---- 接口实现 ----

// List 区域列表
func (h *RegionHandler) List(ctx context.Context, req *model.RegionListQuery) (*model.ListData, error) {
	slog.Debug("handler.区域列表", "region_id", req.RegionID, "region_type", req.RegionType)
	return h.svc.List(ctx, req)
}

// Create 创建区域
func (h *RegionHandler) Create(ctx context.Context, req *model.CreateRegionRequest) (*model.CreateRegionResponse, error) {
	if err := checkRegionID(req.RegionID, h.regionCfg.NameMaxLength); err != nil {
		return nil, err
	}
	if err := shared.CheckLabel(req.DisplayName, h.regionCfg.DisplayNameMaxLength, "区域中文名"); err != nil {
		return nil, err
	}
	slog.Debug("handler.创建区域", "region_id", req.RegionID)
	return h.svc.Create(ctx, req)
}

// Get 区域详情
func (h *RegionHandler) Get(ctx context.Context, req *model.IDRequest) (*model.Region, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	slog.Debug("handler.区域详情", "id", req.ID)
	return h.svc.GetByID(ctx, req.ID)
}

// Update 编辑区域（启用中禁止 — 43xxx 族错误由 service 层返）
func (h *RegionHandler) Update(ctx context.Context, req *model.UpdateRegionRequest) (*string, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	if err := shared.CheckVersion(req.Version); err != nil {
		return nil, err
	}
	if err := shared.CheckLabel(req.DisplayName, h.regionCfg.DisplayNameMaxLength, "区域中文名"); err != nil {
		return nil, err
	}
	slog.Debug("handler.编辑区域", "id", req.ID)
	if err := h.svc.Update(ctx, req); err != nil {
		return nil, err
	}
	return shared.SuccessMsg("保存成功"), nil
}

// Delete 软删除区域（启用中禁止 — 47008 由 service 层返）
func (h *RegionHandler) Delete(ctx context.Context, req *model.IDRequest) (*model.DeleteResult, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	slog.Debug("handler.删除区域", "id", req.ID)

	r, err := h.svc.GetByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	if err := h.svc.SoftDelete(ctx, req.ID); err != nil {
		return nil, err
	}

	slog.Info("handler.删除区域成功", "id", r.ID, "region_id", r.RegionID)
	return &model.DeleteResult{ID: r.ID, Name: r.RegionID, Label: r.DisplayName}, nil
}

// ToggleEnabled 切换启用/停用（目标值语义：req.Enabled 是目标值，非翻转）
func (h *RegionHandler) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) (*string, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	if err := shared.CheckVersion(req.Version); err != nil {
		return nil, err
	}
	slog.Debug("handler.切换区域启用", "id", req.ID, "enabled", req.Enabled)
	if err := h.svc.ToggleEnabled(ctx, req); err != nil {
		return nil, err
	}
	return shared.SuccessMsg("操作成功"), nil
}

// CheckName 校验 region_id 唯一性（创建前置）
func (h *RegionHandler) CheckName(ctx context.Context, req *model.CheckNameRequest) (*model.CheckNameResult, error) {
	if err := checkRegionID(req.Name, h.regionCfg.NameMaxLength); err != nil {
		return nil, err
	}
	slog.Debug("handler.校验区域标识", "name", req.Name)
	return h.svc.CheckName(ctx, req.Name)
}
