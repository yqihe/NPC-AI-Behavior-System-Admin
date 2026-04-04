package handler

import (
	"log/slog"
	"net/http"

	"github.com/npc-admin/backend/internal/model"
	"github.com/npc-admin/backend/internal/service"
)

type NpcTypeHandler struct {
	svc *service.NpcTypeService
}

func NewNpcTypeHandler(svc *service.NpcTypeService) *NpcTypeHandler {
	return &NpcTypeHandler{svc: svc}
}

func (h *NpcTypeHandler) List(w http.ResponseWriter, r *http.Request) {
	slog.Debug("handler.npc_type.list")
	docs, err := h.svc.List(r.Context())
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, model.NewListResponse(docs))
}

func (h *NpcTypeHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := pathName(r)
	slog.Debug("handler.npc_type.get", "name", name)
	doc, err := h.svc.Get(r.Context(), name)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (h *NpcTypeHandler) Create(w http.ResponseWriter, r *http.Request) {
	doc, ok := decodeBody(w, r)
	if !ok {
		return
	}
	slog.Debug("handler.npc_type.create", "name", doc.Name)
	if err := h.svc.Create(r.Context(), doc); err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, doc)
}

func (h *NpcTypeHandler) Update(w http.ResponseWriter, r *http.Request) {
	name := pathName(r)
	doc, ok := decodeBody(w, r)
	if !ok {
		return
	}
	slog.Debug("handler.npc_type.update", "name", name)
	if err := h.svc.Update(r.Context(), name, doc); err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (h *NpcTypeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	name := pathName(r)
	slog.Debug("handler.npc_type.delete", "name", name)
	if err := h.svc.Delete(r.Context(), name); err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}
