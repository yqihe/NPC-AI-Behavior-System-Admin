package handler

import (
	shared "github.com/yqihe/npc-ai-admin/backend/internal/handler/shared"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"unicode/utf8"

	"github.com/jmoiron/sqlx"
	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	"github.com/yqihe/npc-ai-admin/backend/internal/service"
)

// TemplateHandler 模板管理 HTTP handler
//
// 严格遵守"分层职责"硬规则：handler 是"用例编排者"。
// 跨模块路径（Create/Update/Delete/Get）由 handler 开启事务、
// 调用 templateService + fieldService 协同完成、commit 后清缓存。
type TemplateHandler struct {
	db              *sqlx.DB
	templateService *service.TemplateService
	fieldService    *service.FieldService
	npcService      *service.NpcService
	valCfg          *config.ValidationConfig
}

// NewTemplateHandler 创建 TemplateHandler
func NewTemplateHandler(
	db *sqlx.DB,
	templateService *service.TemplateService,
	fieldService *service.FieldService,
	npcService *service.NpcService,
	valCfg *config.ValidationConfig,
) *TemplateHandler {
	return &TemplateHandler{
		db:              db,
		templateService: templateService,
		fieldService:    fieldService,
		npcService:      npcService,
		valCfg:          valCfg,
	}
}

// ---- 前置校验（必填/格式/长度，不查 DB） ----

// checkDescription 描述长度校验
func (h *TemplateHandler) checkDescription(description string) *errcode.Error {
	if utf8.RuneCountInString(description) > h.valCfg.DescriptionMaxLength {
		return errcode.Newf(errcode.ErrBadRequest, "描述长度不能超过 %d 个字符", h.valCfg.DescriptionMaxLength)
	}
	return nil
}

// checkTemplateFields 字段数组前置校验（非空 + 元素结构合法 + 不重复）
func checkTemplateFields(fields []model.TemplateFieldEntry) *errcode.Error {
	if len(fields) == 0 {
		return errcode.New(errcode.ErrTemplateNoFields)
	}
	seen := make(map[int64]bool, len(fields))
	for i, f := range fields {
		if f.FieldID <= 0 {
			return errcode.Newf(errcode.ErrBadRequest, "fields[%d].field_id 必须 > 0", i)
		}
		if seen[f.FieldID] {
			return errcode.Newf(errcode.ErrBadRequest, "fields 中 field_id %d 重复", f.FieldID)
		}
		seen[f.FieldID] = true
	}
	return nil
}

// extractFieldIDs 从 fields 数组提取 field_id 列表（保持顺序）
func extractFieldIDs(fields []model.TemplateFieldEntry) []int64 {
	ids := make([]int64, len(fields))
	for i, f := range fields {
		ids[i] = f.FieldID
	}
	return ids
}

// ---- 单模块路径 ----

// List 模板列表
func (h *TemplateHandler) List(ctx context.Context, q *model.TemplateListQuery) (*model.ListData, error) {
	slog.Debug("handler.模板列表", "label", q.Label, "enabled", q.Enabled, "page", q.Page)
	return h.templateService.List(ctx, q)
}

// CheckName 模板标识唯一性校验
func (h *TemplateHandler) CheckName(ctx context.Context, req *model.CheckNameRequest) (*model.CheckNameResult, error) {
	if err := shared.CheckName(req.Name, h.valCfg.TemplateNameMaxLength, errcode.ErrTemplateNameInvalid, "模板标识"); err != nil {
		return nil, err
	}
	slog.Debug("handler.校验模板名", "name", req.Name)
	return h.templateService.CheckName(ctx, req.Name)
}

// ToggleEnabled 切换启用/停用
func (h *TemplateHandler) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) (*string, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	if err := shared.CheckVersion(req.Version); err != nil {
		return nil, err
	}
	slog.Debug("handler.切换模板启用", "id", req.ID, "enabled", req.Enabled)
	if err := h.templateService.ToggleEnabled(ctx, req); err != nil {
		return nil, err
	}
	return shared.SuccessMsg("操作成功"), nil
}

// GetReferences 引用详情（列出引用该模板的 NPC，最多 50 条）
func (h *TemplateHandler) GetReferences(ctx context.Context, req *model.IDRequest) (*model.TemplateReferenceDetail, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	slog.Debug("handler.模板引用详情", "id", req.ID)

	tpl, err := h.templateService.GetByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	npcs, _, _ := h.npcService.ListByTemplateID(ctx, tpl.ID, 1, 50)
	npcItems := make([]model.TemplateReferenceItem, 0, len(npcs))
	for _, n := range npcs {
		npcItems = append(npcItems, model.TemplateReferenceItem{
			NPCID:   n.ID,
			NPCName: n.Name,
		})
	}
	return &model.TemplateReferenceDetail{
		TemplateID:    tpl.ID,
		TemplateLabel: tpl.Label,
		NPCs:          npcItems,
	}, nil
}

// ---- 跨模块路径（含事务编排） ----

// Create 创建模板
//
// 跨模块事务流程：
//  1. 格式校验
//  2. 模板自身业务校验（service.ExistsByName）
//  3. 跨模块校验（fieldService.ValidateFieldsForTemplate）
//  4. 开 tx → templateService.CreateTx → fieldService.AttachToTemplateTx → Commit
//  5. 清两个模块的缓存
func (h *TemplateHandler) Create(ctx context.Context, req *model.CreateTemplateRequest) (*model.CreateTemplateResponse, error) {
	// 1. 格式校验
	if err := shared.CheckName(req.Name, h.valCfg.TemplateNameMaxLength, errcode.ErrTemplateNameInvalid, "模板标识"); err != nil {
		return nil, err
	}
	if err := shared.CheckLabel(req.Label, h.valCfg.FieldLabelMaxLength, "中文标签"); err != nil {
		return nil, err
	}
	if err := h.checkDescription(req.Description); err != nil {
		return nil, err
	}
	if err := checkTemplateFields(req.Fields); err != nil {
		return nil, err
	}

	slog.Debug("handler.创建模板", "name", req.Name, "fields_count", len(req.Fields))

	fieldIDs := extractFieldIDs(req.Fields)

	// 2. 模板自身业务校验：name 唯一性（service 内 CreateTx 也会做，
	//    但前置查可以在事务前给前端友好提示）
	exists, err := h.templateService.ExistsByName(ctx, req.Name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errcode.Newf(errcode.ErrTemplateNameExists, "模板标识 '%s' 已存在", req.Name)
	}

	// 3. 跨模块校验：字段必须存在 + 启用
	if err := h.fieldService.ValidateFieldsForTemplate(ctx, fieldIDs); err != nil {
		return nil, err
	}

	// 4. 跨模块事务
	tx, err := h.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			slog.Warn("handler.模板创建事务回滚失败", "error", rbErr)
		}
	}()

	templateID, err := h.templateService.CreateTx(ctx, tx, req)
	if err != nil {
		return nil, err
	}

	affected, err := h.fieldService.AttachToTemplateTx(ctx, tx, templateID, fieldIDs)
	if err != nil {
		return nil, err
	}

	// 5. 先清缓存再 Commit（消除 Commit 后清缓存窗口期的脏读风险）
	h.templateService.InvalidateList(ctx)
	h.fieldService.InvalidateDetails(ctx, affected)

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	slog.Info("handler.创建模板成功", "id", templateID, "name", req.Name)
	return &model.CreateTemplateResponse{ID: templateID, Name: req.Name}, nil
}

// Get 模板详情
//
// 跨模块拼装流程：
//  1. service.GetByID 拿 *Template 裸行（走自己的 cache）
//  2. ParseFieldEntries 解 fields JSON
//  3. fieldService.GetByIDsLite 跨模块拿字段精简列表（走字段方 cache）
//  4. handler 拼装 TemplateDetail（按 entries 顺序对齐 + Required + Enabled）
func (h *TemplateHandler) Get(ctx context.Context, req *model.IDRequest) (*model.TemplateDetail, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	slog.Debug("handler.模板详情", "id", req.ID)

	tpl, err := h.templateService.GetByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	entries, err := h.templateService.ParseFieldEntries(tpl.Fields)
	if err != nil {
		slog.Error("handler.解析模板 fields JSON 失败", "error", err, "id", req.ID)
		return nil, fmt.Errorf("parse template fields: %w", err)
	}

	fieldIDs := make([]int64, len(entries))
	for i, e := range entries {
		fieldIDs[i] = e.FieldID
	}

	// 跨模块取字段精简（按 fieldIDs 顺序对齐）
	fieldLites, err := h.fieldService.GetByIDsLite(ctx, fieldIDs)
	if err != nil {
		return nil, err
	}

	// 拼装 TemplateFieldItem，保持原顺序
	items := make([]model.TemplateFieldItem, 0, len(entries))
	for i, e := range entries {
		fl := fieldLites[i]
		if fl.ID == 0 {
			// 缺失字段：理论上不会发生（删除前必须先解除引用），warn 跳过
			slog.Warn("handler.模板详情字段缺失", "template_id", req.ID, "field_id", e.FieldID)
			continue
		}
		items = append(items, model.TemplateFieldItem{
			FieldID:       fl.ID,
			Name:          fl.Name,
			Label:         fl.Label,
			Type:          fl.Type,
			Category:      fl.Category,
			CategoryLabel: fl.CategoryLabel,
			Enabled:       fl.Enabled,
			Required:      e.Required,
		})
	}

	return &model.TemplateDetail{
		ID:          tpl.ID,
		Name:        tpl.Name,
		Label:       tpl.Label,
		Description: tpl.Description,
		Enabled:   tpl.Enabled,
		Version:   tpl.Version,
		CreatedAt: tpl.CreatedAt,
		UpdatedAt:   tpl.UpdatedAt,
		Fields:      items,
	}, nil
}

// Update 编辑模板
//
// 跨模块事务流程：
//  1. 格式校验
//  2. service.GetByID 拿旧 tpl + ParseFieldEntries 拿 oldEntries
//  3. service.UpdateTx 在 tx 内做 enabled/diff/写 templates
//  4. fields 集合变了 → 跨模块 Detach + Attach
//  5. Commit → 清两个模块缓存
func (h *TemplateHandler) Update(ctx context.Context, req *model.UpdateTemplateRequest) (*string, error) {
	// 1. 格式校验
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	if err := shared.CheckVersion(req.Version); err != nil {
		return nil, err
	}
	if err := shared.CheckLabel(req.Label, h.valCfg.FieldLabelMaxLength, "中文标签"); err != nil {
		return nil, err
	}
	if err := h.checkDescription(req.Description); err != nil {
		return nil, err
	}
	if err := checkTemplateFields(req.Fields); err != nil {
		return nil, err
	}

	slog.Debug("handler.编辑模板", "id", req.ID, "version", req.Version)

	// 2. 拿旧 tpl 与 oldEntries
	old, err := h.templateService.GetByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	oldEntries, err := h.templateService.ParseFieldEntries(old.Fields)
	if err != nil {
		slog.Error("handler.解析模板 fields JSON 失败", "error", err, "id", req.ID)
		return nil, fmt.Errorf("parse old fields: %w", err)
	}

	// 3. 跨模块校验（仅校验新增字段）
	//    在事务外预校验：先计算 toAdd（service 同样会算一次，但耗时极小）
	toAddPre, _ := diffNewFieldIDs(oldEntries, req.Fields)
	if len(toAddPre) > 0 {
		if err := h.fieldService.ValidateFieldsForTemplate(ctx, toAddPre); err != nil {
			return nil, err
		}
	}

	// 字段有变更时：存在 NPC 引用则拒绝修改字段配置
	if fieldsWillChange(oldEntries, req.Fields) {
		if count, err := h.npcService.CountByTemplateID(ctx, req.ID); err != nil {
			return nil, err
		} else if count > 0 {
			return nil, errcode.New(errcode.ErrTemplateRefEditFields)
		}
	}

	// 4. 跨模块事务
	tx, err := h.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			slog.Warn("handler.模板编辑事务回滚失败", "error", rbErr)
		}
	}()

	fieldsChanged, toAdd, toRemove, err := h.templateService.UpdateTx(ctx, tx, req, old, oldEntries)
	if err != nil {
		return nil, err
	}

	var attachAffected, detachAffected []int64
	// 集合变更才需要操作 field_refs（required-only 变化没有 add/remove）
	if fieldsChanged && (len(toAdd) > 0 || len(toRemove) > 0) {
		// 先 detach 再 attach（顺序无关，但保持一致便于排查死锁）
		if len(toRemove) > 0 {
			detachAffected, err = h.fieldService.DetachFromTemplateTx(ctx, tx, req.ID, toRemove)
			if err != nil {
				return nil, err
			}
		}
		if len(toAdd) > 0 {
			attachAffected, err = h.fieldService.AttachToTemplateTx(ctx, tx, req.ID, toAdd)
			if err != nil {
				return nil, err
			}
		}
	}

	// 5. 先清缓存再 Commit（消除 Commit 后清缓存窗口期的脏读风险）
	h.templateService.InvalidateDetail(ctx, req.ID)
	h.templateService.InvalidateList(ctx)
	if len(detachAffected) > 0 {
		h.fieldService.InvalidateDetails(ctx, detachAffected)
	}
	if len(attachAffected) > 0 {
		h.fieldService.InvalidateDetails(ctx, attachAffected)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	slog.Info("handler.编辑模板成功", "id", req.ID, "fields_changed", fieldsChanged)
	return shared.SuccessMsg("保存成功"), nil
}

// Delete 删除模板
//
// 跨模块事务流程：
//  1. 格式校验
//  2. service.GetByID + enabled 校验 (41009)
//  3. ParseFieldEntries 拿要解除引用的 fieldIDs
//  4. tx → SoftDeleteTx + DetachFromTemplateTx → Commit
//  5. 清两个模块缓存
func (h *TemplateHandler) Delete(ctx context.Context, req *model.IDRequest) (*model.DeleteResult, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	slog.Debug("handler.删除模板", "id", req.ID)

	tpl, err := h.templateService.GetByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	// 必须先停用
	if tpl.Enabled {
		return nil, errcode.New(errcode.ErrTemplateDeleteNotDisabled)
	}

	// 跨模块引用检查：存在 NPC 引用则拒绝删除
	if count, err := h.npcService.CountByTemplateID(ctx, tpl.ID); err != nil {
		return nil, err
	} else if count > 0 {
		return nil, errcode.New(errcode.ErrTemplateRefDelete)
	}

	// 解 fields 拿要 detach 的 fieldIDs
	entries, err := h.templateService.ParseFieldEntries(tpl.Fields)
	if err != nil {
		slog.Error("handler.解析模板 fields 失败", "error", err, "id", req.ID)
		return nil, fmt.Errorf("parse template fields: %w", err)
	}
	fieldIDs := make([]int64, len(entries))
	for i, e := range entries {
		fieldIDs[i] = e.FieldID
	}

	// 跨模块事务
	tx, err := h.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
			slog.Warn("handler.模板删除事务回滚失败", "error", rbErr)
		}
	}()

	if err := h.templateService.SoftDeleteTx(ctx, tx, req.ID); err != nil {
		return nil, err
	}

	affected, err := h.fieldService.DetachFromTemplateTx(ctx, tx, req.ID, fieldIDs)
	if err != nil {
		return nil, err
	}

	// 先清缓存再 Commit（消除 Commit 后清缓存窗口期的脏读风险）
	h.templateService.InvalidateDetail(ctx, req.ID)
	h.templateService.InvalidateList(ctx)
	h.fieldService.InvalidateDetails(ctx, affected)

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	slog.Info("handler.删除模板成功", "id", req.ID, "name", tpl.Name)
	return &model.DeleteResult{ID: tpl.ID, Name: tpl.Name, Label: tpl.Label}, nil
}

// ---- 内部辅助 ----

// fieldsWillChange 判断字段配置是否有变更（集合 + 顺序 + required 任一不同均视为变更）
//
// 用于事务外预判：若有变更则校验 NPC 引用，避免破坏 NPC 快照一致性。
func fieldsWillChange(old, new []model.TemplateFieldEntry) bool {
	if len(old) != len(new) {
		return true
	}
	for i := range old {
		if old[i].FieldID != new[i].FieldID || old[i].Required != new[i].Required {
			return true
		}
	}
	return false
}

// diffNewFieldIDs 计算新增的 fieldIDs（用于事务前校验启用状态）
//
// handler 在事务外预校验只关心 toAdd（toRemove 不需要校验启用）。
// service 内部会再做一次完整 diff，handler 这里的预校验是为了提前给前端友好错误。
func diffNewFieldIDs(old, new []model.TemplateFieldEntry) (toAdd []int64, toRemove []int64) {
	oldSet := make(map[int64]bool, len(old))
	for _, e := range old {
		oldSet[e.FieldID] = true
	}
	newSet := make(map[int64]bool, len(new))
	for _, e := range new {
		newSet[e.FieldID] = true
	}
	toAdd = make([]int64, 0)
	toRemove = make([]int64, 0)
	for _, e := range new {
		if !oldSet[e.FieldID] {
			toAdd = append(toAdd, e.FieldID)
		}
	}
	for _, e := range old {
		if !newSet[e.FieldID] {
			toRemove = append(toRemove, e.FieldID)
		}
	}
	return toAdd, toRemove
}
