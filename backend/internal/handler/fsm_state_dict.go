package handler

import (
	shared "github.com/yqihe/npc-ai-admin/backend/internal/handler/shared"
	"context"
	"log/slog"
	"unicode/utf8"

	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	"github.com/yqihe/npc-ai-admin/backend/internal/service"
)

// FsmStateDictHandler 状态字典管理 HTTP handler
type FsmStateDictHandler struct {
	dictService *service.FsmStateDictService
	dictCfg     *config.FsmStateDictConfig
}

// NewFsmStateDictHandler 创建 FsmStateDictHandler
func NewFsmStateDictHandler(
	dictService *service.FsmStateDictService,
	dictCfg *config.FsmStateDictConfig,
) *FsmStateDictHandler {
	return &FsmStateDictHandler{
		dictService: dictService,
		dictCfg:     dictCfg,
	}
}

// ---- 接口实现 ----

// List 状态字典列表
func (h *FsmStateDictHandler) List(ctx context.Context, req *model.FsmStateDictListQuery) (*model.ListData, error) {
	slog.Debug("handler.状态字典列表", "name", req.Name, "category", req.Category)
	return h.dictService.List(ctx, req)
}

// Create 创建状态字典条目
func (h *FsmStateDictHandler) Create(ctx context.Context, req *model.CreateFsmStateDictRequest) (*model.CreateFsmStateDictResponse, error) {
	if e := shared.CheckName(req.Name, h.dictCfg.NameMaxLength, errcode.ErrFsmStateDictNameInvalid, "状态标识"); e != nil {
		return nil, e
	}
	if e := shared.CheckLabel(req.DisplayName, h.dictCfg.DisplayNameMaxLength, "状态中文名"); e != nil {
		return nil, e
	}
	if e := checkCategory(req.Category, h.dictCfg.CategoryMaxLength); e != nil {
		return nil, e
	}
	if e := checkDescription(req.Description, h.dictCfg.DescriptionMaxLength); e != nil {
		return nil, e
	}

	slog.Debug("handler.创建状态字典", "name", req.Name)

	id, err := h.dictService.Create(ctx, req)
	if err != nil {
		return nil, err
	}
	return &model.CreateFsmStateDictResponse{ID: id, Name: req.Name}, nil
}

// Get 状态字典详情
func (h *FsmStateDictHandler) Get(ctx context.Context, req *model.IDRequest) (*model.FsmStateDict, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	slog.Debug("handler.状态字典详情", "id", req.ID)
	return h.dictService.GetByID(ctx, req.ID)
}

// Update 编辑状态字典条目
func (h *FsmStateDictHandler) Update(ctx context.Context, req *model.UpdateFsmStateDictRequest) (*string, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	if err := shared.CheckVersion(req.Version); err != nil {
		return nil, err
	}
	if e := shared.CheckLabel(req.DisplayName, h.dictCfg.DisplayNameMaxLength, "状态中文名"); e != nil {
		return nil, e
	}
	if e := checkCategory(req.Category, h.dictCfg.CategoryMaxLength); e != nil {
		return nil, e
	}
	if e := checkDescription(req.Description, h.dictCfg.DescriptionMaxLength); e != nil {
		return nil, e
	}

	slog.Debug("handler.编辑状态字典", "id", req.ID)

	if err := h.dictService.Update(ctx, req); err != nil {
		return nil, err
	}
	return shared.SuccessMsg("保存成功"), nil
}

// Delete 删除状态字典条目
func (h *FsmStateDictHandler) Delete(ctx context.Context, req *model.IDRequest) (*model.FsmStateDictDeleteResult, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	slog.Debug("handler.删除状态字典", "id", req.ID)
	return h.dictService.Delete(ctx, req.ID)
}

// CheckName 标识唯一性校验
func (h *FsmStateDictHandler) CheckName(ctx context.Context, req *model.CheckNameRequest) (*model.CheckNameResult, error) {
	if e := shared.CheckName(req.Name, h.dictCfg.NameMaxLength, errcode.ErrFsmStateDictNameInvalid, "状态标识"); e != nil {
		return nil, e
	}
	slog.Debug("handler.校验状态标识", "name", req.Name)
	return h.dictService.CheckName(ctx, req.Name)
}

// ToggleEnabled 启用/停用切换
func (h *FsmStateDictHandler) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) (*string, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	if err := shared.CheckVersion(req.Version); err != nil {
		return nil, err
	}
	slog.Debug("handler.切换状态字典启用", "id", req.ID)
	if err := h.dictService.ToggleEnabled(ctx, req.ID, req.Version); err != nil {
		return nil, err
	}
	return shared.SuccessMsg("操作成功"), nil
}

// ---- 前置校验辅助 ----

// checkCategory 校验分类：非空 + UTF-8 字符数上限
func checkCategory(category string, maxLen int) *errcode.Error {
	if category == "" {
		return errcode.Newf(errcode.ErrBadRequest, "分类不能为空")
	}
	if utf8.RuneCountInString(category) > maxLen {
		return errcode.Newf(errcode.ErrBadRequest, "分类长度不能超过 %d 个字符", maxLen)
	}
	return nil
}

// checkDescription 校验说明：可空，UTF-8 字符数上限
func checkDescription(description string, maxLen int) *errcode.Error {
	if maxLen > 0 && utf8.RuneCountInString(description) > maxLen {
		return errcode.Newf(errcode.ErrBadRequest, "说明长度不能超过 %d 个字符", maxLen)
	}
	return nil
}
