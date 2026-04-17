package handler

import (
	shared "github.com/yqihe/npc-ai-admin/backend/internal/handler/shared"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"regexp"

	"github.com/jmoiron/sqlx"
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
	db            *sqlx.DB
	svc           *service.BtTreeService
	fieldService  *service.FieldService
	schemaService *service.EventTypeSchemaService
	npcService    *service.NpcService
	btCfg         *config.BtTreeConfig
}

// NewBtTreeHandler 创建 BtTreeHandler
func NewBtTreeHandler(
	db *sqlx.DB,
	svc *service.BtTreeService,
	fieldService *service.FieldService,
	schemaService *service.EventTypeSchemaService,
	npcService *service.NpcService,
	btCfg *config.BtTreeConfig,
) *BtTreeHandler {
	return &BtTreeHandler{db: db, svc: svc, fieldService: fieldService, schemaService: schemaService, npcService: npcService, btCfg: btCfg}
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
	slog.Debug("handler.行为树列表", "name", req.Name, "display_name", req.DisplayName)
	return h.svc.List(ctx, req)
}

// Create 创建行为树
//
// 跨模块事务：写 bt_trees + 维护 field_refs(ref_type='bt') BB Key 引用。
func (h *BtTreeHandler) Create(ctx context.Context, req *model.CreateBtTreeRequest) (*model.CreateBtTreeResponse, error) {
	if err := checkBtTreeName(req.Name, h.btCfg.NameMaxLength); err != nil {
		return nil, err
	}
	if err := shared.CheckLabel(req.DisplayName, h.btCfg.DisplayNameMaxLength, "行为树中文名"); err != nil {
		return nil, err
	}
	slog.Debug("handler.创建行为树", "name", req.Name)

	tx, err := h.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			slog.Warn("handler.行为树创建事务回滚失败", "error", rbErr)
		}
	}()

	id, err := h.svc.CreateInTx(ctx, tx, req)
	if err != nil {
		return nil, err
	}

	// BB Key 引用追踪
	newKeys, err := h.svc.ExtractBBKeys(ctx, req.Config)
	if err != nil {
		return nil, fmt.Errorf("extract bb keys: %w", err)
	}
	emptyKeys := make(map[string]bool)
	affected, err := h.fieldService.SyncBtBBKeyRefs(ctx, tx, id, emptyKeys, newKeys)
	if err != nil {
		return nil, fmt.Errorf("sync bb key refs: %w", err)
	}
	if _, err := h.schemaService.SyncBtSchemaRefs(ctx, tx, id, emptyKeys, newKeys); err != nil {
		return nil, fmt.Errorf("sync bt schema refs: %w", err)
	}

	// 先清缓存再 Commit
	h.svc.InvalidateList(ctx)
	h.fieldService.InvalidateDetails(ctx, affected)

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
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
//
// 跨模块事务：更新 bt_trees + diff BB Key 引用。
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

	tx, err := h.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			slog.Warn("handler.行为树编辑事务回滚失败", "error", rbErr)
		}
	}()

	oldBt, err := h.svc.UpdateInTx(ctx, tx, req)
	if err != nil {
		return nil, err
	}

	// BB Key diff
	oldKeys, err := h.svc.ExtractBBKeys(ctx, oldBt.Config)
	if err != nil {
		return nil, fmt.Errorf("extract old bb keys: %w", err)
	}
	newKeys, err := h.svc.ExtractBBKeys(ctx, req.Config)
	if err != nil {
		return nil, fmt.Errorf("extract new bb keys: %w", err)
	}
	affected, err := h.fieldService.SyncBtBBKeyRefs(ctx, tx, req.ID, oldKeys, newKeys)
	if err != nil {
		return nil, fmt.Errorf("sync bb key refs: %w", err)
	}
	if _, err := h.schemaService.SyncBtSchemaRefs(ctx, tx, req.ID, oldKeys, newKeys); err != nil {
		return nil, fmt.Errorf("sync bt schema refs: %w", err)
	}

	// 先清缓存再 Commit
	h.svc.InvalidateDetail(ctx, req.ID)
	h.svc.InvalidateList(ctx)
	h.fieldService.InvalidateDetails(ctx, affected)

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return shared.SuccessMsg("保存成功"), nil
}

// Delete 删除行为树
//
// 跨模块事务：软删 bt_trees + 清理 BB Key 引用。
func (h *BtTreeHandler) Delete(ctx context.Context, req *model.IDRequest) (*model.DeleteResult, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	slog.Debug("handler.删除行为树", "id", req.ID)

	// 获取行为树（含 name，用于 NPC 引用检查）
	btree, err := h.svc.GetByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	// 必须先停用
	if btree.Enabled {
		return nil, errcode.New(errcode.ErrBtTreeDeleteNotDisabled)
	}

	// 跨模块引用检查：存在 NPC 引用则拒绝删除
	if count, _ := h.npcService.CountByBtTreeName(ctx, btree.Name); count > 0 {
		return nil, errcode.New(errcode.ErrBtTreeRefDelete)
	}

	tx, err := h.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			slog.Warn("handler.行为树删除事务回滚失败", "error", rbErr)
		}
	}()

	bt, err := h.svc.SoftDeleteInTx(ctx, tx, req.ID)
	if err != nil {
		return nil, err
	}

	// 清理 BB Key 引用（field_refs + schema_refs）
	affected, err := h.fieldService.CleanBtBBKeyRefs(ctx, tx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("clean bb key refs: %w", err)
	}
	if _, err := h.schemaService.CleanBtSchemaRefs(ctx, tx, req.ID); err != nil {
		return nil, fmt.Errorf("clean bt schema refs: %w", err)
	}

	// 先清缓存再 Commit
	h.svc.InvalidateDetail(ctx, req.ID)
	h.svc.InvalidateList(ctx)
	h.fieldService.InvalidateDetails(ctx, affected)

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	slog.Info("handler.删除行为树成功", "id", req.ID, "name", bt.Name)
	return &model.DeleteResult{ID: bt.ID, Name: bt.Name, Label: bt.DisplayName}, nil
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

// GetReferences 行为树引用详情（列出引用该行为树的 NPC，最多 50 条）
func (h *BtTreeHandler) GetReferences(ctx context.Context, req *model.IDRequest) (*model.BtTreeReferenceDetail, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	slog.Debug("handler.行为树引用详情", "id", req.ID)

	bt, err := h.svc.GetByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	npcs, _, _ := h.npcService.ListByBtTreeName(ctx, bt.Name, 1, 50)
	return &model.BtTreeReferenceDetail{
		BtTreeID:    bt.ID,
		BtTreeLabel: bt.DisplayName,
		NPCs:        npcs,
	}, nil
}
