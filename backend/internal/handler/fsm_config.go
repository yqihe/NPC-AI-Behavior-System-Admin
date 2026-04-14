package handler

import (
	shared "github.com/yqihe/npc-ai-admin/backend/internal/handler/shared"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/jmoiron/sqlx"
	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	"github.com/yqihe/npc-ai-admin/backend/internal/service"
)

// FsmConfigHandler 状态机管理 HTTP handler
type FsmConfigHandler struct {
	db               *sqlx.DB
	fsmConfigService *service.FsmConfigService
	fieldService     *service.FieldService
	fsmCfg           *config.FsmConfigConfig
}

// NewFsmConfigHandler 创建 FsmConfigHandler
func NewFsmConfigHandler(
	db *sqlx.DB,
	fsmConfigService *service.FsmConfigService,
	fieldService *service.FieldService,
	fsmCfg *config.FsmConfigConfig,
) *FsmConfigHandler {
	return &FsmConfigHandler{
		db:               db,
		fsmConfigService: fsmConfigService,
		fieldService:     fieldService,
		fsmCfg:           fsmCfg,
	}
}

// ---- 接口实现 ----

// List 状态机列表
func (h *FsmConfigHandler) List(ctx context.Context, req *model.FsmConfigListQuery) (*model.ListData, error) {
	slog.Debug("handler.状态机列表", "label", req.Label)
	return h.fsmConfigService.List(ctx, req)
}

// Create 创建状态机
//
// 跨模块事务：写 fsm_configs + 维护 field_refs(ref_type='fsm') BB Key 引用。
func (h *FsmConfigHandler) Create(ctx context.Context, req *model.CreateFsmConfigRequest) (*model.CreateFsmConfigResponse, error) {
	if e := shared.CheckName(req.Name, h.fsmCfg.NameMaxLength, errcode.ErrFsmConfigNameInvalid, "状态机标识"); e != nil {
		return nil, e
	}
	if e := shared.CheckLabel(req.DisplayName, h.fsmCfg.DisplayNameMaxLength, "中文名称"); e != nil {
		return nil, e
	}

	slog.Debug("handler.创建状态机", "name", req.Name)

	tx, err := h.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			slog.Warn("handler.状态机创建事务回滚失败", "error", rbErr)
		}
	}()

	id, _, err := h.fsmConfigService.CreateInTx(ctx, tx, req)
	if err != nil {
		return nil, err
	}

	// BB Key 引用追踪
	newKeys := service.ExtractBBKeys(req.Transitions)
	emptyKeys := make(map[string]bool)
	affected, err := h.fieldService.SyncFsmBBKeyRefs(ctx, tx, id, emptyKeys, newKeys)
	if err != nil {
		return nil, fmt.Errorf("sync bb key refs: %w", err)
	}

	// 先清缓存再 Commit（消除 Commit 后清缓存窗口期的脏读风险）
	h.fsmConfigService.InvalidateList(ctx)
	h.fieldService.InvalidateDetails(ctx, affected)

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &model.CreateFsmConfigResponse{ID: id, Name: req.Name}, nil
}

// Get 状态机详情
func (h *FsmConfigHandler) Get(ctx context.Context, req *model.IDRequest) (*model.FsmConfigDetail, error) {
	if err := shared.CheckID(req.ID); err != nil {
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
//
// 跨模块事务：更新 fsm_configs + diff BB Key 引用。
func (h *FsmConfigHandler) Update(ctx context.Context, req *model.UpdateFsmConfigRequest) (*string, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	if err := shared.CheckVersion(req.Version); err != nil {
		return nil, err
	}
	if e := shared.CheckLabel(req.DisplayName, h.fsmCfg.DisplayNameMaxLength, "中文名称"); e != nil {
		return nil, e
	}

	slog.Debug("handler.编辑状态机", "id", req.ID, "version", req.Version)

	tx, err := h.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			slog.Warn("handler.状态机编辑事务回滚失败", "error", rbErr)
		}
	}()

	oldFc, err := h.fsmConfigService.UpdateInTx(ctx, tx, req)
	if err != nil {
		return nil, err
	}

	// BB Key diff
	oldKeys := service.ExtractBBKeysFromConfigJSON(oldFc.ConfigJSON)
	newKeys := service.ExtractBBKeys(req.Transitions)
	affected, err := h.fieldService.SyncFsmBBKeyRefs(ctx, tx, req.ID, oldKeys, newKeys)
	if err != nil {
		return nil, fmt.Errorf("sync bb key refs: %w", err)
	}

	// 先清缓存再 Commit（消除 Commit 后清缓存窗口期的脏读风险）
	h.fsmConfigService.InvalidateDetail(ctx, req.ID)
	h.fsmConfigService.InvalidateList(ctx)
	h.fieldService.InvalidateDetails(ctx, affected)

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return shared.SuccessMsg("保存成功"), nil
}

// Delete 删除状态机
//
// 跨模块事务：软删 fsm_configs + 清理 BB Key 引用。
func (h *FsmConfigHandler) Delete(ctx context.Context, req *model.IDRequest) (*model.DeleteResult, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}

	slog.Debug("handler.删除状态机", "id", req.ID)

	tx, err := h.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			slog.Warn("handler.状态机删除事务回滚失败", "error", rbErr)
		}
	}()

	fc, err := h.fsmConfigService.SoftDeleteInTx(ctx, tx, req.ID)
	if err != nil {
		return nil, err
	}

	// 清理 BB Key 引用
	affected, err := h.fieldService.CleanFsmBBKeyRefs(ctx, tx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("clean bb key refs: %w", err)
	}

	// 先清缓存再 Commit（消除 Commit 后清缓存窗口期的脏读风险）
	h.fsmConfigService.InvalidateDetail(ctx, req.ID)
	h.fsmConfigService.InvalidateList(ctx)
	h.fieldService.InvalidateDetails(ctx, affected)

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	slog.Info("handler.删除状态机成功", "id", req.ID, "name", fc.Name)
	return &model.DeleteResult{ID: fc.ID, Name: fc.Name, Label: fc.DisplayName}, nil
}

// CheckName 状态机标识唯一性校验
func (h *FsmConfigHandler) CheckName(ctx context.Context, req *model.CheckNameRequest) (*model.CheckNameResult, error) {
	if err := shared.CheckName(req.Name, h.fsmCfg.NameMaxLength, errcode.ErrFsmConfigNameInvalid, "状态机标识"); err != nil {
		return nil, err
	}

	slog.Debug("handler.校验状态机标识", "name", req.Name)

	return h.fsmConfigService.CheckName(ctx, req.Name)
}

// ToggleEnabled 启用/停用切换
func (h *FsmConfigHandler) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) (*string, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	if err := shared.CheckVersion(req.Version); err != nil {
		return nil, err
	}

	slog.Debug("handler.切换启用", "id", req.ID, "enabled", req.Enabled)

	if err := h.fsmConfigService.ToggleEnabled(ctx, req); err != nil {
		return nil, err
	}
	return shared.SuccessMsg("操作成功"), nil
}
