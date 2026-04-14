package handler

import (
	"bytes"
	"context"
	"log/slog"

	"github.com/yqihe/npc-ai-admin/backend/internal/config"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
	"github.com/yqihe/npc-ai-admin/backend/internal/service"
	"github.com/yqihe/npc-ai-admin/backend/internal/util"
)

// EventTypeSchemaHandler 扩展字段 Schema 管理 HTTP handler
type EventTypeSchemaHandler struct {
	schemaService    *service.EventTypeSchemaService
	eventTypeService *service.EventTypeService
	etsCfg           *config.EventTypeSchemaConfig
}

// NewEventTypeSchemaHandler 创建 EventTypeSchemaHandler
func NewEventTypeSchemaHandler(
	schemaService *service.EventTypeSchemaService,
	eventTypeService *service.EventTypeService,
	etsCfg *config.EventTypeSchemaConfig,
) *EventTypeSchemaHandler {
	return &EventTypeSchemaHandler{
		schemaService:    schemaService,
		eventTypeService: eventTypeService,
		etsCfg:           etsCfg,
	}
}

// ---- 前置校验 ----

func checkFieldType(fieldType string) *errcode.Error {
	if !util.ValidExtFieldTypes[fieldType] {
		return errcode.New(errcode.ErrExtSchemaTypeInvalid)
	}
	return nil
}

func checkJSONObjectShape(data []byte, fieldDesc string) *errcode.Error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return errcode.Newf(errcode.ErrBadRequest, "%s 必须是 JSON 对象", fieldDesc)
	}
	return nil
}

// ---- 接口实现 ----

// EventTypeSchemaListResponse 列表响应包装
type EventTypeSchemaListResponse struct {
	Items []model.EventTypeSchema `json:"items"`
}

// List 扩展字段 Schema 列表
func (h *EventTypeSchemaHandler) List(ctx context.Context, req *model.EventTypeSchemaListQuery) (*EventTypeSchemaListResponse, error) {
	slog.Debug("handler.event_type_schema.list")
	items, err := h.schemaService.List(ctx, req)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = make([]model.EventTypeSchema, 0)
	}
	// 填充 has_refs
	h.schemaService.FillHasRefs(ctx, items)
	return &EventTypeSchemaListResponse{Items: items}, nil
}

// Create 创建扩展字段定义
func (h *EventTypeSchemaHandler) Create(ctx context.Context, req *model.CreateEventTypeSchemaRequest) (*model.CreateEventTypeSchemaResponse, error) {
	slog.Debug("handler.event_type_schema.create", "field_name", req.FieldName)

	if e := CheckName(req.FieldName, h.etsCfg.FieldNameMaxLength, errcode.ErrExtSchemaNameInvalid, "扩展字段标识"); e != nil {
		return nil, e
	}
	if e := CheckLabel(req.FieldLabel, h.etsCfg.FieldLabelMaxLength, "扩展字段中文名"); e != nil {
		return nil, e
	}
	if e := checkFieldType(req.FieldType); e != nil {
		return nil, e
	}
	if e := checkJSONObjectShape(req.Constraints, "约束"); e != nil {
		return nil, e
	}
	if len(req.DefaultValue) == 0 {
		return nil, errcode.Newf(errcode.ErrBadRequest, "默认值不能为空")
	}

	id, err := h.schemaService.Create(ctx, req)
	if err != nil {
		return nil, err
	}

	return &model.CreateEventTypeSchemaResponse{ID: id, FieldName: req.FieldName}, nil
}

// Update 编辑扩展字段定义
func (h *EventTypeSchemaHandler) Update(ctx context.Context, req *model.UpdateEventTypeSchemaRequest) (*model.Empty, error) {
	slog.Debug("handler.event_type_schema.update", "id", req.ID)

	if req.ID <= 0 {
		return nil, errcode.Newf(errcode.ErrBadRequest, "ID 必须 > 0")
	}
	if req.Version <= 0 {
		return nil, errcode.Newf(errcode.ErrBadRequest, "version 必须 > 0")
	}
	if e := CheckLabel(req.FieldLabel, h.etsCfg.FieldLabelMaxLength, "扩展字段中文名"); e != nil {
		return nil, e
	}
	if e := checkJSONObjectShape(req.Constraints, "约束"); e != nil {
		return nil, e
	}
	if len(req.DefaultValue) == 0 {
		return nil, errcode.Newf(errcode.ErrBadRequest, "默认值不能为空")
	}

	if err := h.schemaService.Update(ctx, req); err != nil {
		return nil, err
	}
	return &model.Empty{}, nil
}

// Delete 删除扩展字段定义
func (h *EventTypeSchemaHandler) Delete(ctx context.Context, req *model.IDRequest) (*model.Empty, error) {
	slog.Debug("handler.event_type_schema.delete", "id", req.ID)

	if req.ID <= 0 {
		return nil, errcode.Newf(errcode.ErrBadRequest, "ID 必须 > 0")
	}

	if err := h.schemaService.Delete(ctx, req.ID); err != nil {
		return nil, err
	}
	return &model.Empty{}, nil
}

// ToggleEnabled 启用/停用切换
func (h *EventTypeSchemaHandler) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) (*model.Empty, error) {
	slog.Debug("handler.event_type_schema.toggle_enabled", "id", req.ID)

	if req.ID <= 0 {
		return nil, errcode.Newf(errcode.ErrBadRequest, "ID 必须 > 0")
	}
	if req.Version <= 0 {
		return nil, errcode.Newf(errcode.ErrBadRequest, "version 必须 > 0")
	}

	if err := h.schemaService.ToggleEnabled(ctx, req.ID, req.Version); err != nil {
		return nil, err
	}
	return &model.Empty{}, nil
}

// GetReferences 扩展字段引用详情
//
// 跨模块编排：SchemaService 返回 event_type IDs，handler 调 EventTypeService 补 display_name。
func (h *EventTypeSchemaHandler) GetReferences(ctx context.Context, req *model.IDRequest) (*model.SchemaReferenceDetail, error) {
	if req.ID <= 0 {
		return nil, errcode.Newf(errcode.ErrBadRequest, "ID 必须 > 0")
	}

	slog.Debug("handler.event_type_schema.references", "id", req.ID)

	detail, err := h.schemaService.GetReferences(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	// 跨模块补齐事件类型 display_name
	if len(detail.EventTypes) > 0 {
		for i := range detail.EventTypes {
			et, err := h.eventTypeService.GetByID(ctx, detail.EventTypes[i].RefID)
			if err != nil {
				slog.Warn("handler.补事件类型label失败", "error", err, "id", detail.EventTypes[i].RefID)
				continue
			}
			if et != nil {
				detail.EventTypes[i].Label = et.DisplayName
			}
		}
	}

	return detail, nil
}
