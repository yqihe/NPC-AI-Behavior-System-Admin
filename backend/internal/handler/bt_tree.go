package handler

import (
	shared "github.com/yqihe/npc-ai-admin/backend/internal/handler/shared"
	"context"
	"log/slog"
	"regexp"

	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	"github.com/yqihe/npc-ai-admin/backend/internal/service"
)

// btTreeNameRe 行为树 name 合法格式：小写字母开头，仅含小写字母/数字/下划线/斜杠
// 与字段标识不同，斜杠用于前端分组展示
var btTreeNameRe = regexp.MustCompile(`^[a-z][a-z0-9_/]*$`)

// BtTreeHandler 行为树管理 HTTP handler
type BtTreeHandler struct {
	svc   *service.BtTreeService
	btCfg *config.BtTreeConfig
}

// NewBtTreeHandler 创建 BtTreeHandler
func NewBtTreeHandler(
	svc *service.BtTreeService,
	btCfg *config.BtTreeConfig,
) *BtTreeHandler {
	return &BtTreeHandler{svc: svc, btCfg: btCfg}
}

// checkBtTreeName 校验 bt_tree name 格式（允许斜杠）
func checkBtTreeName(name string, maxLen int) *errcode.Error {
	if name == "" {
		return errcode.Newf(errcode.ErrBtTreeNameInvalid, "行为树标识不能为空")
	}
	if !btTreeNameRe.MatchString(name) {
		return errcode.New(errcode.ErrBtTreeNameInvalid)
	}
	if len(name) > maxLen {
		return errcode.Newf(errcode.ErrBtTreeNameInvalid, "行为树标识长度不能超过 %d 个字符", maxLen)
	}
	return nil
}

// ---- 接口实现 ----

// List 行为树列表
func (h *BtTreeHandler) List(ctx context.Context, req *model.BtTreeListQuery) (*model.ListData, error) {
	slog.Debug("handler.行为树列表", "display_name", req.DisplayName)
	return h.svc.List(ctx, req)
}

// Create 创建行为树
func (h *BtTreeHandler) Create(ctx context.Context, req *model.CreateBtTreeRequest) (*model.CreateBtTreeResponse, error) {
	if err := checkBtTreeName(req.Name, h.btCfg.NameMaxLength); err != nil {
		return nil, err
	}
	if err := shared.CheckLabel(req.DisplayName, h.btCfg.DisplayNameMaxLength, "行为树中文名"); err != nil {
		return nil, err
	}
	slog.Debug("handler.创建行为树", "name", req.Name)

	id, err := h.svc.Create(ctx, req)
	if err != nil {
		return nil, err
	}
	return &model.CreateBtTreeResponse{ID: id, Name: req.Name}, nil
}

// Get 行为树详情
func (h *BtTreeHandler) Get(ctx context.Context, req *model.IDRequest) (*model.BtTree, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	slog.Debug("handler.行为树详情", "id", req.ID)
	return h.svc.GetByID(ctx, req.ID)
}

// Update 编辑行为树
func (h *BtTreeHandler) Update(ctx context.Context, req *model.UpdateBtTreeRequest) (*string, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	if err := shared.CheckVersion(req.Version); err != nil {
		return nil, err
	}
	if err := shared.CheckLabel(req.DisplayName, h.btCfg.DisplayNameMaxLength, "行为树中文名"); err != nil {
		return nil, err
	}
	slog.Debug("handler.编辑行为树", "id", req.ID)

	if err := h.svc.Update(ctx, req); err != nil {
		return nil, err
	}
	return shared.SuccessMsg("保存成功"), nil
}

// Delete 删除行为树
func (h *BtTreeHandler) Delete(ctx context.Context, req *model.IDRequest) (*model.DeleteResult, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	slog.Debug("handler.删除行为树", "id", req.ID)

	if err := h.svc.Delete(ctx, req.ID); err != nil {
		return nil, err
	}
	return &model.DeleteResult{ID: req.ID}, nil
}

// CheckName name 唯一性校验
func (h *BtTreeHandler) CheckName(ctx context.Context, req *model.CheckNameRequest) (*model.CheckNameResult, error) {
	if err := checkBtTreeName(req.Name, h.btCfg.NameMaxLength); err != nil {
		return nil, err
	}
	slog.Debug("handler.校验行为树标识", "name", req.Name)
	return h.svc.CheckName(ctx, req.Name)
}

// ToggleEnabled 切换启用/停用
func (h *BtTreeHandler) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) (*string, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	if err := shared.CheckVersion(req.Version); err != nil {
		return nil, err
	}
	slog.Debug("handler.切换行为树启用", "id", req.ID)

	if err := h.svc.ToggleEnabled(ctx, req); err != nil {
		return nil, err
	}
	return shared.SuccessMsg("操作成功"), nil
}
