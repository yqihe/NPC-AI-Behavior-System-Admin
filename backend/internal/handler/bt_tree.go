package handler

import (
	"log/slog"
	"net/http"

	"github.com/npc-admin/backend/internal/model"
	"github.com/npc-admin/backend/internal/service"
)

// btTreePrefix 是行为树单条资源的 URL 前缀，用于从路径中提取含 "/" 的 name。
const btTreePrefix = "/api/v1/bt-trees/"

type BtTreeHandler struct {
	svc *service.BtTreeService
}

func NewBtTreeHandler(svc *service.BtTreeService) *BtTreeHandler {
	return &BtTreeHandler{svc: svc}
}

func (h *BtTreeHandler) List(w http.ResponseWriter, r *http.Request) {
	slog.Debug("handler.bt_tree.list")
	docs, err := h.svc.List(r.Context())
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, model.NewListResponse(docs))
}

func (h *BtTreeHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := pathNameAfterPrefix(r, btTreePrefix)
	slog.Debug("handler.bt_tree.get", "name", name)
	doc, err := h.svc.Get(r.Context(), name)
	if err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (h *BtTreeHandler) Create(w http.ResponseWriter, r *http.Request) {
	doc, ok := decodeBody(w, r)
	if !ok {
		return
	}
	slog.Debug("handler.bt_tree.create", "name", doc.Name)
	if err := h.svc.Create(r.Context(), doc); err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, doc)
}

func (h *BtTreeHandler) Update(w http.ResponseWriter, r *http.Request) {
	name := pathNameAfterPrefix(r, btTreePrefix)
	doc, ok := decodeBody(w, r)
	if !ok {
		return
	}
	slog.Debug("handler.bt_tree.update", "name", name)
	if err := h.svc.Update(r.Context(), name, doc); err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, doc)
}

func (h *BtTreeHandler) Delete(w http.ResponseWriter, r *http.Request) {
	name := pathNameAfterPrefix(r, btTreePrefix)
	slog.Debug("handler.bt_tree.delete", "name", name)
	if err := h.svc.Delete(r.Context(), name); err != nil {
		handleServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}
