package handler

import (
	hshared "github.com/yqihe/npc-ai-admin/backend/internal/handler/shared"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"unicode/utf8"

	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	"github.com/yqihe/npc-ai-admin/backend/internal/service"
)

// NpcHandler NPC 管理 HTTP handler
//
// 跨模块编排者：
//   - Create/Update：handler 负责跨模块数据获取，service 负责校验+快照组装+写 DB
//   - Get（详情）：handler 负责跨模块获取 FieldLite + TemplateLabel，service 负责拼装
//   - List：handler 负责批量补全 TemplateLabel
type NpcHandler struct {
	npcService      *service.NpcService
	templateService *service.TemplateService
	fieldService    *service.FieldService
	fsmService      *service.FsmConfigService
	btService       *service.BtTreeService
	valCfg          *config.ValidationConfig
}

// NewNpcHandler 创建 NpcHandler
func NewNpcHandler(
	npcService *service.NpcService,
	templateService *service.TemplateService,
	fieldService *service.FieldService,
	fsmService *service.FsmConfigService,
	btService *service.BtTreeService,
	valCfg *config.ValidationConfig,
) *NpcHandler {
	return &NpcHandler{
		npcService:      npcService,
		templateService: templateService,
		fieldService:    fieldService,
		fsmService:      fsmService,
		btService:       btService,
		valCfg:          valCfg,
	}
}

// ──────────────────────────────────────────────
// 格式校验辅助
// ──────────────────────────────────────────────

// checkDescription 描述长度校验
func (h *NpcHandler) checkDescription(description string) *errcode.Error {
	if utf8.RuneCountInString(description) > h.valCfg.DescriptionMaxLength {
		return errcode.Newf(errcode.ErrBadRequest, "描述长度不能超过 %d 个字符", h.valCfg.DescriptionMaxLength)
	}
	return nil
}

// collectBtNames 从 btRefs 中收集去重后的非空行为树名列表
func collectBtNames(btRefs map[string]string) []string {
	seen := make(map[string]bool)
	names := make([]string, 0, len(btRefs))
	for _, btName := range btRefs {
		if btName != "" && !seen[btName] {
			seen[btName] = true
			names = append(names, btName)
		}
	}
	return names
}

// ──────────────────────────────────────────────
// 接口实现
// ──────────────────────────────────────────────

// List NPC 列表（含跨模块补全 TemplateLabel）
func (h *NpcHandler) List(ctx context.Context, q *model.NPCListQuery) (*model.ListData, error) {
	slog.Debug("handler.NPC列表", "label", q.Label, "name", q.Name, "page", q.Page)

	listData, err := h.npcService.List(ctx, q)
	if err != nil {
		return nil, err
	}

	// 批量补全 TemplateLabel
	if len(listData.Items) > 0 {
		seen := make(map[int64]bool)
		ids := make([]int64, 0)
		for _, item := range listData.Items {
			if item.TemplateID > 0 && !seen[item.TemplateID] {
				seen[item.TemplateID] = true
				ids = append(ids, item.TemplateID)
			}
		}
		if len(ids) > 0 {
			lites, err := h.templateService.GetByIDsLite(ctx, ids)
			if err == nil {
				labelMap := make(map[int64]string, len(lites))
				for _, tpl := range lites {
					labelMap[tpl.ID] = tpl.Label
				}
				for i := range listData.Items {
					listData.Items[i].TemplateLabel = labelMap[listData.Items[i].TemplateID]
				}
			}
		}
	}

	return listData.ToListData(), nil
}

// Create 创建 NPC（跨模块校验 + 快照组装）
func (h *NpcHandler) Create(ctx context.Context, req *model.CreateNPCRequest) (*model.CreateNPCResponse, error) {
	// 1. 格式校验
	if err := hshared.CheckName(req.Name, h.valCfg.NPCNameMaxLength, errcode.ErrNPCNameInvalid, "NPC 标识"); err != nil {
		return nil, err
	}
	if err := hshared.CheckLabel(req.Label, h.valCfg.FieldLabelMaxLength, "中文标签"); err != nil {
		return nil, err
	}
	if err := h.checkDescription(req.Description); err != nil {
		return nil, err
	}
	if req.TemplateID <= 0 {
		return nil, errcode.Newf(errcode.ErrBadRequest, "template_id 不合法")
	}

	slog.Debug("handler.创建NPC", "name", req.Name)

	// 2. 跨模块获取：模板 + 模板字段列表
	tpl, err := h.templateService.GetByID(ctx, req.TemplateID)
	if err != nil {
		var ecErr *errcode.Error
		if errors.As(err, &ecErr) && ecErr.Code == errcode.ErrTemplateNotFound {
			return nil, errcode.New(errcode.ErrNPCTemplateNotFound)
		}
		return nil, err
	}
	if !tpl.Enabled {
		return nil, errcode.New(errcode.ErrNPCTemplateDisabled)
	}

	templateEntries, err := h.templateService.ParseFieldEntries(tpl.Fields)
	if err != nil {
		slog.Error("handler.创建NPC-解析模板字段失败", "error", err, "template_id", req.TemplateID)
		return nil, fmt.Errorf("parse template fields: %w", err)
	}

	// 3. 跨模块获取：字段元数据
	fieldIDs := make([]int64, len(templateEntries))
	for i, e := range templateEntries {
		fieldIDs[i] = e.FieldID
	}
	fieldLites, err := h.fieldService.GetByIDsLite(ctx, fieldIDs)
	if err != nil {
		return nil, err
	}

	// 4. service 层校验 + 快照组装
	snapshot, verr := h.npcService.BuildFieldSnapshot(templateEntries, fieldLites, req.FieldValues)
	if verr != nil {
		return nil, verr
	}

	// 5. 跨模块校验：行为配置
	fsmStates, err := h.fsmService.GetStateNames(ctx, req.FsmRef)
	if err != nil {
		return nil, err
	}
	if verr := h.npcService.ValidateBehaviorRefs(req.FsmRef, req.BtRefs, fsmStates); verr != nil {
		return nil, verr
	}
	// 校验 BT 存在且启用
	btNames := collectBtNames(req.BtRefs)
	if len(btNames) > 0 {
		notOK, err := h.btService.CheckEnabledByNames(ctx, btNames)
		if err != nil {
			return nil, fmt.Errorf("check bt trees: %w", err)
		}
		if len(notOK) > 0 {
			return nil, errcode.Newf(errcode.ErrNPCBtNotFound,
				"行为树 '%s' 不存在或未启用", notOK[0])
		}
	}

	// 6. 填入 server-side 字段
	req.TemplateName = tpl.Name
	req.FieldsSnapshot = snapshot
	if req.BtRefs == nil {
		req.BtRefs = make(map[string]string)
	}

	// 7. 创建
	id, err := h.npcService.Create(ctx, req)
	if err != nil {
		return nil, err
	}

	slog.Info("handler.创建NPC成功", "id", id, "name", req.Name)
	return &model.CreateNPCResponse{ID: id, Name: req.Name}, nil
}

// Get NPC 详情（跨模块拼装 NPCDetail）
func (h *NpcHandler) Get(ctx context.Context, req *model.IDRequest) (*model.NPCDetail, error) {
	if err := hshared.CheckID(req.ID); err != nil {
		return nil, err
	}
	slog.Debug("handler.NPC详情", "id", req.ID)

	npc, err := h.npcService.GetByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	// 跨模块获取：字段元数据
	fieldIDs, err := h.npcService.ExtractFieldIDsFromNPC(npc.Fields)
	if err != nil {
		slog.Error("handler.NPC详情-解析字段快照失败", "error", err, "id", req.ID)
		return nil, err
	}
	fieldLites, err := h.fieldService.GetByIDsLite(ctx, fieldIDs)
	if err != nil {
		return nil, err
	}

	// 跨模块获取：模板标签
	templateLabel := ""
	lites, err := h.templateService.GetByIDsLite(ctx, []int64{npc.TemplateID})
	if err == nil && len(lites) > 0 {
		templateLabel = lites[0].Label
	}

	// service 层拼装详情
	return h.npcService.BuildDetail(npc, fieldLites, templateLabel)
}

// Update 编辑 NPC（跨模块字段校验 + 快照重组装）
func (h *NpcHandler) Update(ctx context.Context, req *model.UpdateNPCRequest) (*string, error) {
	if err := hshared.CheckID(req.ID); err != nil {
		return nil, err
	}
	if err := hshared.CheckVersion(req.Version); err != nil {
		return nil, err
	}
	if err := hshared.CheckLabel(req.Label, h.valCfg.FieldLabelMaxLength, "中文标签"); err != nil {
		return nil, err
	}
	if err := h.checkDescription(req.Description); err != nil {
		return nil, err
	}

	slog.Debug("handler.编辑NPC", "id", req.ID)

	// 取当前 NPC
	npc, err := h.npcService.GetByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	// service 层解析旧快照
	oldEntries, templateEntries, err := h.npcService.ParseNPCFieldEntries(npc.Fields)
	if err != nil {
		slog.Error("handler.编辑NPC-解析字段快照失败", "error", err, "id", req.ID)
		return nil, err
	}

	// 跨模块获取：字段元数据
	fieldIDs := make([]int64, len(templateEntries))
	for i, e := range templateEntries {
		fieldIDs[i] = e.FieldID
	}
	fieldLites, err := h.fieldService.GetByIDsLite(ctx, fieldIDs)
	if err != nil {
		return nil, err
	}

	// service 层校验 + 快照重组装
	snapshot, verr := h.npcService.BuildFieldSnapshot(templateEntries, fieldLites, req.FieldValues)
	if verr != nil {
		return nil, verr
	}
	h.npcService.FillSnapshotNames(snapshot, oldEntries)

	// 跨模块校验：行为配置
	fsmStates, err := h.fsmService.GetStateNames(ctx, req.FsmRef)
	if err != nil {
		return nil, err
	}
	if verr := h.npcService.ValidateBehaviorRefs(req.FsmRef, req.BtRefs, fsmStates); verr != nil {
		return nil, verr
	}
	btNames := collectBtNames(req.BtRefs)
	if len(btNames) > 0 {
		notOK, err := h.btService.CheckEnabledByNames(ctx, btNames)
		if err != nil {
			return nil, fmt.Errorf("check bt trees: %w", err)
		}
		if len(notOK) > 0 {
			return nil, errcode.Newf(errcode.ErrNPCBtNotFound,
				"行为树 '%s' 不存在或未启用", notOK[0])
		}
	}

	req.FieldsSnapshot = snapshot
	if req.BtRefs == nil {
		req.BtRefs = make(map[string]string)
	}

	if err := h.npcService.Update(ctx, req); err != nil {
		return nil, err
	}

	slog.Info("handler.编辑NPC成功", "id", req.ID)
	return hshared.SuccessMsg("保存成功"), nil
}

// Delete 删除 NPC（启用中 → 45013 守卫）
func (h *NpcHandler) Delete(ctx context.Context, req *model.IDRequest) (*model.DeleteResult, error) {
	if err := hshared.CheckID(req.ID); err != nil {
		return nil, err
	}
	slog.Debug("handler.删除NPC", "id", req.ID)

	// 守卫：启用中禁止删除
	npc, err := h.npcService.GetByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	if npc.Enabled {
		return nil, errcode.New(errcode.ErrNPCDeleteNotDisabled)
	}

	return h.npcService.SoftDelete(ctx, req.ID)
}

// CheckName NPC 标识唯一性校验
func (h *NpcHandler) CheckName(ctx context.Context, req *model.CheckNameRequest) (*model.CheckNameResult, error) {
	if err := hshared.CheckName(req.Name, h.valCfg.NPCNameMaxLength, errcode.ErrNPCNameInvalid, "NPC 标识"); err != nil {
		return nil, err
	}
	slog.Debug("handler.校验NPC标识", "name", req.Name)
	return h.npcService.CheckName(ctx, req.Name)
}

// ToggleEnabled 切换启用/停用（乐观锁）
func (h *NpcHandler) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) (*string, error) {
	if err := hshared.CheckID(req.ID); err != nil {
		return nil, err
	}
	if err := hshared.CheckVersion(req.Version); err != nil {
		return nil, err
	}
	slog.Debug("handler.切换NPC启用", "id", req.ID)

	if err := h.npcService.ToggleEnabled(ctx, req); err != nil {
		return nil, err
	}
	return hshared.SuccessMsg("操作成功"), nil
}
