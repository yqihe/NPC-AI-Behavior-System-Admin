package handler

import (
	"context"
	"encoding/json"
	"log/slog"
	"unicode/utf8"

	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	"github.com/yqihe/npc-ai-admin/backend/internal/service"
)

// FsmConfigHandler 状态机管理 HTTP handler
type FsmConfigHandler struct {
	fsmConfigService *service.FsmConfigService
	fsmCfg           *config.FsmConfigConfig
}

// NewFsmConfigHandler 创建 FsmConfigHandler
func NewFsmConfigHandler(
	fsmConfigService *service.FsmConfigService,
	fsmCfg *config.FsmConfigConfig,
) *FsmConfigHandler {
	return &FsmConfigHandler{
		fsmConfigService: fsmConfigService,
		fsmCfg:           fsmCfg,
	}
}

// ---- 前置校验 ----

func (h *FsmConfigHandler) checkName(name string) *errcode.Error {
	if name == "" {
		return errcode.Newf(errcode.ErrFsmConfigNameInvalid, "状态机标识不能为空")
	}
	if !identPattern.MatchString(name) {
		return errcode.New(errcode.ErrFsmConfigNameInvalid)
	}
	if len(name) > h.fsmCfg.NameMaxLength {
		return errcode.Newf(errcode.ErrFsmConfigNameInvalid, "状态机标识长度不能超过 %d 个字符", h.fsmCfg.NameMaxLength)
	}
	return nil
}

func (h *FsmConfigHandler) checkDisplayName(displayName string) *errcode.Error {
	if displayName == "" {
		return errcode.Newf(errcode.ErrBadRequest, "中文名称不能为空")
	}
	if utf8.RuneCountInString(displayName) > h.fsmCfg.DisplayNameMaxLength {
		return errcode.Newf(errcode.ErrBadRequest, "中文名称长度不能超过 %d 个字符", h.fsmCfg.DisplayNameMaxLength)
	}
	return nil
}

// ---- 接口实现 ----

// List 状态机列表
func (h *FsmConfigHandler) List(ctx context.Context, req *model.FsmConfigListQuery) (*model.ListData, error) {
	slog.Debug("handler.状态机列表", "label", req.Label)
	return h.fsmConfigService.List(ctx, req)
}

// Create 创建状态机
func (h *FsmConfigHandler) Create(ctx context.Context, req *model.CreateFsmConfigRequest) (*model.CreateFsmConfigResponse, error) {
	// Handler 格式校验
	if e := h.checkName(req.Name); e != nil {
		return nil, e
	}
	if e := h.checkDisplayName(req.DisplayName); e != nil {
		return nil, e
	}

	slog.Debug("handler.创建状态机", "name", req.Name)

	id, err := h.fsmConfigService.Create(ctx, req)
	if err != nil {
		return nil, err
	}

	return &model.CreateFsmConfigResponse{ID: id, Name: req.Name}, nil
}

// Get 状态机详情
func (h *FsmConfigHandler) Get(ctx context.Context, req *model.IDRequest) (*model.FsmConfigDetail, error) {
	if err := checkID(req.ID); err != nil {
		return nil, err
	}

	slog.Debug("handler.状态机详情", "id", req.ID)

	fc, err := h.fsmConfigService.GetByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	// unmarshal config_json
	var cfg map[string]interface{}
	if fc.ConfigJSON != nil {
		if err := json.Unmarshal(fc.ConfigJSON, &cfg); err != nil {
			slog.Error("handler.fsm_config.get.unmarshal_config", "error", err, "id", req.ID)
			cfg = make(map[string]interface{})
		}
	}
	if cfg == nil {
		cfg = make(map[string]interface{})
	}

	detail := &model.FsmConfigDetail{
		ID:          fc.ID,
		Name:        fc.Name,
		DisplayName: fc.DisplayName,
		Enabled:     fc.Enabled,
		Version:     fc.Version,
		CreatedAt:   fc.CreatedAt,
		UpdatedAt:   fc.UpdatedAt,
		Config:      cfg,
	}

	return detail, nil
}

// Update 编辑状态机
func (h *FsmConfigHandler) Update(ctx context.Context, req *model.UpdateFsmConfigRequest) (*string, error) {
	if err := checkID(req.ID); err != nil {
		return nil, err
	}
	if err := checkVersion(req.Version); err != nil {
		return nil, err
	}
	if e := h.checkDisplayName(req.DisplayName); e != nil {
		return nil, e
	}

	slog.Debug("handler.编辑状态机", "id", req.ID, "version", req.Version)

	if err := h.fsmConfigService.Update(ctx, req); err != nil {
		return nil, err
	}
	return successMsg("保存成功"), nil
}

// Delete 删除状态机
func (h *FsmConfigHandler) Delete(ctx context.Context, req *model.IDRequest) (*model.DeleteResult, error) {
	if err := checkID(req.ID); err != nil {
		return nil, err
	}

	slog.Debug("handler.删除状态机", "id", req.ID)

	return h.fsmConfigService.Delete(ctx, req.ID)
}

// CheckName 状态机标识唯一性校验
func (h *FsmConfigHandler) CheckName(ctx context.Context, req *model.CheckNameRequest) (*model.CheckNameResult, error) {
	if err := h.checkName(req.Name); err != nil {
		return nil, err
	}

	slog.Debug("handler.校验状态机标识", "name", req.Name)

	return h.fsmConfigService.CheckName(ctx, req.Name)
}

// ToggleEnabled 启用/停用切换
func (h *FsmConfigHandler) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) (*string, error) {
	if err := checkID(req.ID); err != nil {
		return nil, err
	}
	if err := checkVersion(req.Version); err != nil {
		return nil, err
	}

	slog.Debug("handler.切换启用", "id", req.ID, "enabled", req.Enabled)

	if err := h.fsmConfigService.ToggleEnabled(ctx, req); err != nil {
		return nil, err
	}
	return successMsg("操作成功"), nil
}
