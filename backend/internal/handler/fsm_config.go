package handler

import (
	"log/slog"
	"net/http"

	"github.com/npc-admin/backend/internal/model"
	"github.com/npc-admin/backend/internal/service"
)

type FsmConfigHandler struct {
	svc *service.FsmConfigService
}

func NewFsmConfigHandler(svc *service.FsmConfigService) *FsmConfigHandler {
	return &FsmConfigHandler{svc: svc}
}

func (h *FsmConfigHandler) List(w http.ResponseWriter, r *http.Request) {
	slog.Debug("handler.fsm_config.list")
	docs, err := h.svc.List(r.Context())
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, model.NewListResponse(docs))
}

func (h *FsmConfigHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := pathName(r)
	slog.Debug("handler.fsm_config.get", "name", name)
	doc, err := h.svc.Get(r.Context(), name)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (h *FsmConfigHandler) Create(w http.ResponseWriter, r *http.Request) {
	doc, ok := decodeBody(w, r)
	if !ok {
		return
	}
	slog.Debug("handler.fsm_config.create", "name", doc.Name)
	if err := h.svc.Create(r.Context(), doc); err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, doc)
}

func (h *FsmConfigHandler) Update(w http.ResponseWriter, r *http.Request) {
	name := pathName(r)
	doc, ok := decodeBody(w, r)
	if !ok {
		return
	}
	slog.Debug("handler.fsm_config.update", "name", name)
	if err := h.svc.Update(r.Context(), name, doc); err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (h *FsmConfigHandler) Delete(w http.ResponseWriter, r *http.Request) {
	name := pathName(r)
	slog.Debug("handler.fsm_config.delete", "name", name)
	if err := h.svc.Delete(r.Context(), name); err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}
