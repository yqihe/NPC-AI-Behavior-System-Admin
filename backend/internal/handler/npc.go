package handler

import (
	hshared "github.com/yqihe/npc-ai-admin/backend/internal/handler/shared"
	svcshared "github.com/yqihe/npc-ai-admin/backend/internal/service/shared"
	"context"
	"encoding/json"
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
//   - Create/Update：handler 负责模板/字段/FSM/BT 校验 + 快照组装，service 只做写 DB + 缓存
//   - Get（详情）：handler 负责跨模块补全 FieldLite + TemplateLabel
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
// 私有校验辅助
// ──────────────────────────────────────────────

// checkDescription 描述长度校验
func (h *NpcHandler) checkDescription(description string) *errcode.Error {
	if utf8.RuneCountInString(description) > h.valCfg.DescriptionMaxLength {
		return errcode.Newf(errcode.ErrBadRequest, "描述长度不能超过 %d 个字符", h.valCfg.DescriptionMaxLength)
	}
	return nil
}

// isJSONNull 判断 json.RawMessage 是否为 JSON null（未提供时视为 null）
func isJSONNull(v json.RawMessage) bool {
	return len(v) == 0 || string(v) == "null"
}

// validateFieldValues 按模板字段顺序校验字段值，返回组装后的快照
//
// templateEntries: 模板字段顺序（field_id, required）
// fieldMap:       字段元数据 map（field_id → FieldLite，含 Type + Properties）
// valueMap:       前端传入值 map（field_id → value）
func validateFieldValues(
	templateEntries []model.TemplateFieldEntry,
	fieldMap map[int64]model.FieldLite,
	valueMap map[int64]json.RawMessage,
) ([]model.NPCFieldEntry, *errcode.Error) {
	snapshot := make([]model.NPCFieldEntry, 0, len(templateEntries))
	for _, entry := range templateEntries {
		field, ok := fieldMap[entry.FieldID]
		if !ok {
			// 字段已删除或不可见（罕见，skip + slog，保持快照 field_id 但 value=null）
			slog.Warn("handler.NPC字段校验-字段元数据缺失", "field_id", entry.FieldID)
			snapshot = append(snapshot, model.NPCFieldEntry{
				FieldID:  entry.FieldID,
				Required: entry.Required,
				Value:    json.RawMessage("null"),
			})
			continue
		}

		value := valueMap[entry.FieldID]

		// null 检查
		if isJSONNull(value) {
			if entry.Required {
				return nil, errcode.Newf(errcode.ErrNPCFieldRequired,
					"字段 '%s' 为必填项，值不能为空", field.Name)
			}
			value = json.RawMessage("null")
		} else {
			// 类型 + 约束校验
			var props model.FieldProperties
			if len(field.Properties) > 0 {
				_ = json.Unmarshal(field.Properties, &props) // 解析失败时 props 为零值
			}
			if verr := svcshared.ValidateValue(field.Type, props.Constraints, value); verr != nil {
				return nil, errcode.Newf(errcode.ErrNPCFieldValueInvalid,
					"字段 '%s' 值不符合约束: %s", field.Name, verr.Message)
			}
		}

		snapshot = append(snapshot, model.NPCFieldEntry{
			FieldID:  entry.FieldID,
			Name:     field.Name,
			Required: entry.Required,
			Value:    value,
		})
	}
	return snapshot, nil
}

// validateBehaviorConfig 校验 FSM + BT 配置合法性
//
// 返回 FSM states 集合（bt_refs 状态键校验用）；fsmRef 为空时返回空 map。
func (h *NpcHandler) validateBehaviorConfig(
	ctx context.Context,
	fsmRef string,
	btRefs map[string]string,
) (map[string]bool, error) {
	fsmStates := make(map[string]bool)

	if len(btRefs) > 0 && fsmRef == "" {
		return nil, errcode.New(errcode.ErrNPCBtWithoutFsm)
	}

	if fsmRef != "" {
		fsm, err := h.fsmService.GetEnabledByName(ctx, fsmRef)
		if err != nil {
			return nil, err
		}

		// 解析 FSM states 列表（供 bt_refs 键校验）
		var fsmCfg struct {
			States []struct {
				Name string `json:"name"`
			} `json:"states"`
		}
		if err := json.Unmarshal(fsm.ConfigJSON, &fsmCfg); err != nil {
			return nil, fmt.Errorf("parse fsm states: %w", err)
		}
		for _, s := range fsmCfg.States {
			fsmStates[s.Name] = true
		}
	}

	if len(btRefs) > 0 {
		// 收集非空 bt_tree_name（值为空串表示该状态不绑 BT，跳过校验）
		btNames := make([]string, 0, len(btRefs))
		seen := make(map[string]bool)
		for stateName, btName := range btRefs {
			if btName == "" {
				continue
			}
			// 校验 state_name 在 FSM states 中
			if !fsmStates[stateName] {
				return nil, errcode.Newf(errcode.ErrNPCBtStateInvalid,
					"行为树绑定的状态名 '%s' 不在状态机 '%s' 的状态列表中", stateName, fsmRef)
			}
			if !seen[btName] {
				btNames = append(btNames, btName)
				seen[btName] = true
			}
		}

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
	}

	return fsmStates, nil
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

	// 2. 校验模板
	tpl, err := h.templateService.GetByID(ctx, req.TemplateID)
	if err != nil {
		return nil, err
	}
	if tpl == nil {
		return nil, errcode.New(errcode.ErrNPCTemplateNotFound)
	}
	if !tpl.Enabled {
		return nil, errcode.New(errcode.ErrNPCTemplateDisabled)
	}

	// 3. 解析模板字段列表
	templateEntries, err := h.templateService.ParseFieldEntries(tpl.Fields)
	if err != nil {
		slog.Error("handler.创建NPC-解析模板字段失败", "error", err, "template_id", req.TemplateID)
		return nil, fmt.Errorf("parse template fields: %w", err)
	}

	// 4. 批量拿字段元数据
	fieldIDs := make([]int64, len(templateEntries))
	for i, e := range templateEntries {
		fieldIDs[i] = e.FieldID
	}
	fieldLites, err := h.fieldService.GetByIDsLite(ctx, fieldIDs)
	if err != nil {
		return nil, err
	}
	fieldMap := make(map[int64]model.FieldLite, len(fieldLites))
	for _, fl := range fieldLites {
		if fl.ID > 0 {
			fieldMap[fl.ID] = fl
		}
	}

	// 5. 构造 valueMap（field_id → raw value）
	valueMap := make(map[int64]json.RawMessage, len(req.FieldValues))
	for _, fv := range req.FieldValues {
		valueMap[fv.FieldID] = fv.Value
	}

	// 6. 字段值校验 + 快照组装
	snapshot, verr := validateFieldValues(templateEntries, fieldMap, valueMap)
	if verr != nil {
		return nil, verr
	}

	// 7. 行为配置校验
	if _, err := h.validateBehaviorConfig(ctx, req.FsmRef, req.BtRefs); err != nil {
		return nil, err
	}

	// 8. 填入 server-side 字段
	req.TemplateName = tpl.Name
	req.FieldsSnapshot = snapshot
	if req.BtRefs == nil {
		req.BtRefs = make(map[string]string)
	}

	// 9. 创建
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

	// 解析字段快照
	var fieldEntries []model.NPCFieldEntry
	if err := json.Unmarshal(npc.Fields, &fieldEntries); err != nil {
		slog.Error("handler.NPC详情-解析字段快照失败", "error", err, "id", req.ID)
		return nil, fmt.Errorf("parse npc fields: %w", err)
	}

	// 批量拿字段元数据
	fieldIDs := make([]int64, len(fieldEntries))
	for i, e := range fieldEntries {
		fieldIDs[i] = e.FieldID
	}
	fieldLites, err := h.fieldService.GetByIDsLite(ctx, fieldIDs)
	if err != nil {
		return nil, err
	}
	fieldLiteMap := make(map[int64]model.FieldLite, len(fieldLites))
	for _, fl := range fieldLites {
		if fl.ID > 0 {
			fieldLiteMap[fl.ID] = fl
		}
	}

	// 拼装 []NPCDetailField（按快照顺序）
	detailFields := make([]model.NPCDetailField, 0, len(fieldEntries))
	for _, entry := range fieldEntries {
		fl, ok := fieldLiteMap[entry.FieldID]
		if !ok {
			slog.Warn("handler.NPC详情-字段元数据缺失", "field_id", entry.FieldID)
			continue
		}
		detailFields = append(detailFields, model.NPCDetailField{
			FieldID:       entry.FieldID,
			Name:          entry.Name,
			Label:         fl.Label,
			Type:          fl.Type,
			Category:      fl.Category,
			CategoryLabel: fl.CategoryLabel,
			Enabled:       fl.Enabled,
			Required:      entry.Required,
			Value:         entry.Value,
		})
	}

	// 补全 TemplateLabel
	templateLabel := ""
	lites, err := h.templateService.GetByIDsLite(ctx, []int64{npc.TemplateID})
	if err == nil && len(lites) > 0 {
		templateLabel = lites[0].Label
	}

	// 解析 bt_refs
	var btRefs map[string]string
	if len(npc.BtRefs) > 0 {
		_ = json.Unmarshal(npc.BtRefs, &btRefs)
	}
	if btRefs == nil {
		btRefs = make(map[string]string)
	}

	return &model.NPCDetail{
		ID:            npc.ID,
		Name:          npc.Name,
		Label:         npc.Label,
		Description:   npc.Description,
		TemplateID:    npc.TemplateID,
		TemplateName:  npc.TemplateName,
		TemplateLabel: templateLabel,
		Enabled:       npc.Enabled,
		Version:       npc.Version,
		Fields:        detailFields,
		FsmRef:        npc.FsmRef,
		BtRefs:        btRefs,
	}, nil
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

	// 取当前 NPC（用于字段快照回填）
	npc, err := h.npcService.GetByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	// 解析字段快照（保留 field_id/name/required 元数据，仅更新 value）
	var oldEntries []model.NPCFieldEntry
	if err := json.Unmarshal(npc.Fields, &oldEntries); err != nil {
		slog.Error("handler.编辑NPC-解析字段快照失败", "error", err, "id", req.ID)
		return nil, fmt.Errorf("parse npc fields: %w", err)
	}

	// 重建 templateEntries（从快照中恢复，不重查模板）
	templateEntries := make([]model.TemplateFieldEntry, len(oldEntries))
	for i, e := range oldEntries {
		templateEntries[i] = model.TemplateFieldEntry{
			FieldID:  e.FieldID,
			Required: e.Required,
		}
	}

	// 批量拿字段元数据（供类型/约束校验）
	fieldIDs := make([]int64, len(oldEntries))
	for i, e := range oldEntries {
		fieldIDs[i] = e.FieldID
	}
	fieldLites, err := h.fieldService.GetByIDsLite(ctx, fieldIDs)
	if err != nil {
		return nil, err
	}
	fieldMap := make(map[int64]model.FieldLite, len(fieldLites))
	for _, fl := range fieldLites {
		if fl.ID > 0 {
			fieldMap[fl.ID] = fl
		}
	}

	// 构造 valueMap
	valueMap := make(map[int64]json.RawMessage, len(req.FieldValues))
	for _, fv := range req.FieldValues {
		valueMap[fv.FieldID] = fv.Value
	}

	// 字段值校验 + 快照重组装
	snapshot, verr := validateFieldValues(templateEntries, fieldMap, valueMap)
	if verr != nil {
		return nil, verr
	}

	// 行为配置校验
	if _, err := h.validateBehaviorConfig(ctx, req.FsmRef, req.BtRefs); err != nil {
		return nil, err
	}

	// 填入快照 name（从旧快照获取，保持不变）
	for i := range snapshot {
		if old := findEntry(oldEntries, snapshot[i].FieldID); old != nil {
			snapshot[i].Name = old.Name
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

// findEntry 在 NPCFieldEntry 切片中按 field_id 查找
func findEntry(entries []model.NPCFieldEntry, fieldID int64) *model.NPCFieldEntry {
	for i := range entries {
		if entries[i].FieldID == fieldID {
			return &entries[i]
		}
	}
	return nil
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
