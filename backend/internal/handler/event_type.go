package handler

import (
	"log/slog"
	"net/http"

	"github.com/npc-admin/backend/internal/model"
	"github.com/npc-admin/backend/internal/service"
)

// EventTypeHandler 处理事件类型的 HTTP 请求。
type EventTypeHandler struct {
	svc *service.EventTypeService
}

// NewEventTypeHandler 创建事件类型 handler。
func NewEventTypeHandler(svc *service.EventTypeService) *EventTypeHandler {
	return &EventTypeHandler{svc: svc}
}

// List GET /api/v1/event-types
func (h *EventTypeHandler) List(w http.ResponseWriter, r *http.Request) {
	slog.Debug("handler.event_type.list")

	docs, err := h.svc.List(r.Context())
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, model.NewListResponse(docs))
}

// Get GET /api/v1/event-types/{name}
func (h *EventTypeHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := pathName(r)
	slog.Debug("handler.event_type.get", "name", name)

	doc, err := h.svc.Get(r.Context(), name)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

// Create POST /api/v1/event-types
func (h *EventTypeHandler) Create(w http.ResponseWriter, r *http.Request) {
	doc, ok := decodeBody(w, r)
	if !ok {
		return
	}
	slog.Debug("handler.event_type.create", "name", doc.Name)

	if err := h.svc.Create(r.Context(), doc); err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, doc)
}

// Update PUT /api/v1/event-types/{name}
func (h *EventTypeHandler) Update(w http.ResponseWriter, r *http.Request) {
	name := pathName(r)
	doc, ok := decodeBody(w, r)
	if !ok {
		return
	}
	slog.Debug("handler.event_type.update", "name", name)

	if err := h.svc.Update(r.Context(), name, doc); err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

// Delete DELETE /api/v1/event-types/{name}
func (h *EventTypeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	name := pathName(r)
	slog.Debug("handler.event_type.delete", "name", name)

	if err := h.svc.Delete(r.Context(), name); err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}
