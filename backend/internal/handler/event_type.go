package handler

import (
	shared "github.com/yqihe/npc-ai-admin/backend/internal/handler/shared"
	"context"
	"encoding/json"
	"log/slog"

	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	"github.com/yqihe/npc-ai-admin/backend/internal/service"
	"github.com/yqihe/npc-ai-admin/backend/internal/util"
)

// EventTypeHandler 事件类型管理 HTTP handler
type EventTypeHandler struct {
	eventTypeService       *service.EventTypeService
	eventTypeSchemaService *service.EventTypeSchemaService
	etCfg                  *config.EventTypeConfig
}

// NewEventTypeHandler 创建 EventTypeHandler
func NewEventTypeHandler(
	eventTypeService *service.EventTypeService,
	eventTypeSchemaService *service.EventTypeSchemaService,
	etCfg *config.EventTypeConfig,
) *EventTypeHandler {
	return &EventTypeHandler{
		eventTypeService:       eventTypeService,
		eventTypeSchemaService: eventTypeSchemaService,
		etCfg:                  etCfg,
	}
}

// ---- 前置校验 ----

func checkPerceptionMode(mode string) *errcode.Error {
	if !util.ValidPerceptionModes[mode] {
		return errcode.New(errcode.ErrEventTypeModeInvalid)
	}
	return nil
}

func checkSeverity(severity float64) *errcode.Error {
	if severity < 0 || severity > 100 {
		return errcode.New(errcode.ErrEventTypeSeverityInvalid)
	}
	return nil
}

func checkTTL(ttl float64) *errcode.Error {
	if ttl <= 0 {
		return errcode.New(errcode.ErrEventTypeTTLInvalid)
	}
	return nil
}

func checkRange(rangeVal float64) *errcode.Error {
	if rangeVal < 0 {
		return errcode.New(errcode.ErrEventTypeRangeInvalid)
	}
	return nil
}

func checkExtensionsShape(extensions map[string]interface{}) *errcode.Error {
	// extensions 是 map[string]interface{}，JSON 绑定时如果传 null/数组/标量会失败
	// 这里只做额外的空 key 检查
	for k := range extensions {
		if k == "" {
			return errcode.Newf(errcode.ErrBadRequest, "扩展字段 key 不能为空")
		}
	}
	return nil
}

// ---- 接口实现 ----

// List 事件类型列表
func (h *EventTypeHandler) List(ctx context.Context, req *model.EventTypeListQuery) (*model.ListData, error) {
	slog.Debug("handler.事件类型列表", "label", req.Label, "mode", req.PerceptionMode)
	return h.eventTypeService.List(ctx, req)
}

// Create 创建事件类型
func (h *EventTypeHandler) Create(ctx context.Context, req *model.CreateEventTypeRequest) (*model.CreateEventTypeResponse, error) {
	// Handler 格式校验
	if e := shared.CheckName(req.Name, h.etCfg.NameMaxLength, errcode.ErrEventTypeNameInvalid, "事件标识"); e != nil {
		return nil, e
	}
	if e := shared.CheckLabel(req.DisplayName, h.etCfg.DisplayNameMaxLength, "中文名称"); e != nil {
		return nil, e
	}
	if e := checkPerceptionMode(req.PerceptionMode); e != nil {
		return nil, e
	}
	if e := checkSeverity(req.DefaultSeverity); e != nil {
		return nil, e
	}
	if e := checkTTL(req.DefaultTTL); e != nil {
		return nil, e
	}
	if e := checkRange(req.Range); e != nil {
		return nil, e
	}
	// global 模式后端兜底
	if req.PerceptionMode == util.PerceptionModeGlobal {
		req.Range = 0
	}
	if e := checkExtensionsShape(req.Extensions); e != nil {
		return nil, e
	}

	slog.Debug("handler.创建事件类型", "name", req.Name, "mode", req.PerceptionMode)

	id, err := h.eventTypeService.Create(ctx, req)
	if err != nil {
		return nil, err
	}

	return &model.CreateEventTypeResponse{ID: id, Name: req.Name}, nil
}

// Get 事件类型详情（跨模块拼装：事件类型 + 扩展字段 schema）
func (h *EventTypeHandler) Get(ctx context.Context, req *model.IDRequest) (*model.EventTypeDetail, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}

	slog.Debug("handler.事件类型详情", "id", req.ID)

	et, err := h.eventTypeService.GetByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	// unmarshal config_json
	var config map[string]interface{}
	if et.ConfigJSON != nil {
		if err := json.Unmarshal(et.ConfigJSON, &config); err != nil {
			slog.Error("handler.event_type.get.unmarshal_config", "error", err, "id", req.ID)
			config = make(map[string]interface{})
		}
	}
	if config == nil {
		config = make(map[string]interface{})
	}

	// 拿扩展字段 schema：启用的 + 虽然禁用但 config 里有值的
	schemas := h.eventTypeSchemaService.ListEnabled()
	enabledNames := make(map[string]bool, len(schemas))
	for _, s := range schemas {
		enabledNames[s.FieldName] = true
	}

	// 系统字段集合（不是扩展字段）
	systemKeys := map[string]bool{
		"display_name": true, "default_severity": true,
		"default_ttl": true, "perception_mode": true, "range": true,
	}

	// 检查 config 中是否有禁用 schema 的值
	var missingNames []string
	for k := range config {
		if !systemKeys[k] && !enabledNames[k] {
			missingNames = append(missingNames, k)
		}
	}
	if len(missingNames) > 0 {
		// 拉全量 schema（含禁用），补上缺失的
		allSchemas, err := h.eventTypeSchemaService.ListAllLite(ctx)
		if err != nil {
			slog.Error("handler.event_type.get.list_all_schemas", "error", err)
		} else {
			missingSet := make(map[string]bool, len(missingNames))
			for _, n := range missingNames {
				missingSet[n] = true
			}
			for _, s := range allSchemas {
				if missingSet[s.FieldName] {
					schemas = append(schemas, s)
				}
			}
		}
	}

	detail := &model.EventTypeDetail{
		ID:              et.ID,
		Name:            et.Name,
		DisplayName:     et.DisplayName,
		PerceptionMode:  et.PerceptionMode,
		Enabled:         et.Enabled,
		Version:         et.Version,
		CreatedAt:       et.CreatedAt,
		UpdatedAt:       et.UpdatedAt,
		Config:          config,
		ExtensionSchema: schemas,
	}

	return detail, nil
}

// Update 编辑事件类型
func (h *EventTypeHandler) Update(ctx context.Context, req *model.UpdateEventTypeRequest) (*string, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	if err := shared.CheckVersion(req.Version); err != nil {
		return nil, err
	}
	if e := shared.CheckLabel(req.DisplayName, h.etCfg.DisplayNameMaxLength, "中文名称"); e != nil {
		return nil, e
	}
	if e := checkPerceptionMode(req.PerceptionMode); e != nil {
		return nil, e
	}
	if e := checkSeverity(req.DefaultSeverity); e != nil {
		return nil, e
	}
	if e := checkTTL(req.DefaultTTL); e != nil {
		return nil, e
	}
	if e := checkRange(req.Range); e != nil {
		return nil, e
	}
	if req.PerceptionMode == util.PerceptionModeGlobal {
		req.Range = 0
	}
	if e := checkExtensionsShape(req.Extensions); e != nil {
		return nil, e
	}

	slog.Debug("handler.编辑事件类型", "id", req.ID, "version", req.Version)

	if err := h.eventTypeService.Update(ctx, req); err != nil {
		return nil, err
	}
	return shared.SuccessMsg("保存成功"), nil
}

// Delete 删除事件类型
func (h *EventTypeHandler) Delete(ctx context.Context, req *model.IDRequest) (*model.DeleteResult, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}

	slog.Debug("handler.删除事件类型", "id", req.ID)

	return h.eventTypeService.Delete(ctx, req.ID)
}

// CheckName 事件标识唯一性校验
func (h *EventTypeHandler) CheckName(ctx context.Context, req *model.CheckNameRequest) (*model.CheckNameResult, error) {
	if err := shared.CheckName(req.Name, h.etCfg.NameMaxLength, errcode.ErrEventTypeNameInvalid, "事件标识"); err != nil {
		return nil, err
	}

	slog.Debug("handler.校验事件标识", "name", req.Name)

	return h.eventTypeService.CheckName(ctx, req.Name)
}

// ToggleEnabled 启用/停用切换
func (h *EventTypeHandler) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) (*string, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	if err := shared.CheckVersion(req.Version); err != nil {
		return nil, err
	}

	slog.Debug("handler.切换启用", "id", req.ID, "enabled", req.Enabled)

	if err := h.eventTypeService.ToggleEnabled(ctx, req); err != nil {
		return nil, err
	}
	return shared.SuccessMsg("操作成功"), nil
}

