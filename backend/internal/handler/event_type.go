package handler

import (
	shared "github.com/yqihe/npc-ai-admin/backend/internal/handler/shared"
	"context"
	"log/slog"

	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	"github.com/yqihe/npc-ai-admin/backend/internal/service"
	"github.com/yqihe/npc-ai-admin/backend/internal/util"
)

// EventTypeHandler 事件类型管理 HTTP handler
type EventTypeHandler struct {
	eventTypeService *service.EventTypeService
	etCfg            *config.EventTypeConfig
}

// NewEventTypeHandler 创建 EventTypeHandler
func NewEventTypeHandler(
	eventTypeService *service.EventTypeService,
	etCfg *config.EventTypeConfig,
) *EventTypeHandler {
	return &EventTypeHandler{
		eventTypeService: eventTypeService,
		etCfg:            etCfg,
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
	slog.Debug("handler.事件类型列表", "name", req.Name, "label", req.Label, "mode", req.PerceptionMode)
	return h.eventTypeService.List(ctx, req)
}

// Create 创建事件类型
func (h *EventTypeHandler) Create(ctx context.Context, req *model.CreateEventTypeRequest) (*model.CreateEventTypeResponse, error) {
	// Handler 格式校验
	if err := shared.CheckName(req.Name, h.etCfg.NameMaxLength, errcode.ErrEventTypeNameInvalid, "事件标识"); err != nil {
		return nil, err
	}
	if err := shared.CheckLabel(req.DisplayName, h.etCfg.DisplayNameMaxLength, "中文名称"); err != nil {
		return nil, err
	}
	if err := checkPerceptionMode(req.PerceptionMode); err != nil {
		return nil, err
	}
	if err := checkSeverity(req.DefaultSeverity); err != nil {
		return nil, err
	}
	if err := checkTTL(req.DefaultTTL); err != nil {
		return nil, err
	}
	if err := checkRange(req.Range); err != nil {
		return nil, err
	}
	// global 模式后端兜底
	if req.PerceptionMode == util.PerceptionModeGlobal {
		req.Range = 0
	}
	if err := checkExtensionsShape(req.Extensions); err != nil {
		return nil, err
	}

	slog.Debug("handler.创建事件类型", "name", req.Name, "mode", req.PerceptionMode)

	id, err := h.eventTypeService.Create(ctx, req)
	if err != nil {
		return nil, err
	}

	return &model.CreateEventTypeResponse{ID: id, Name: req.Name}, nil
}

// Get 事件类型详情
func (h *EventTypeHandler) Get(ctx context.Context, req *model.IDRequest) (*model.EventTypeDetail, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	slog.Debug("handler.事件类型详情", "id", req.ID)
	return h.eventTypeService.GetDetail(ctx, req.ID)
}

// Update 编辑事件类型
func (h *EventTypeHandler) Update(ctx context.Context, req *model.UpdateEventTypeRequest) (*string, error) {
	if err := shared.CheckID(req.ID); err != nil {
		return nil, err
	}
	if err := shared.CheckVersion(req.Version); err != nil {
		return nil, err
	}
	if err := shared.CheckLabel(req.DisplayName, h.etCfg.DisplayNameMaxLength, "中文名称"); err != nil {
		return nil, err
	}
	if err := checkPerceptionMode(req.PerceptionMode); err != nil {
		return nil, err
	}
	if err := checkSeverity(req.DefaultSeverity); err != nil {
		return nil, err
	}
	if err := checkTTL(req.DefaultTTL); err != nil {
		return nil, err
	}
	if err := checkRange(req.Range); err != nil {
		return nil, err
	}
	if req.PerceptionMode == util.PerceptionModeGlobal {
		req.Range = 0
	}
	if err := checkExtensionsShape(req.Extensions); err != nil {
		return nil, err
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

