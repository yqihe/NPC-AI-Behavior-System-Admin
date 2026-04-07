package handler

import (
	"log/slog"
	"net/http"

	"github.com/npc-admin/backend/internal/model"
	"github.com/npc-admin/backend/internal/service"
)

// GenericHandler 是通用的 CRUD HTTP handler，适用于任意实体集合。
type GenericHandler struct {
	service    *service.GenericService
	apiPrefix  string
	allowSlash bool // name 是否允许包含 "/"（如行为树 "civilian/idle"）
}

// NewGenericHandler 创建通用 CRUD handler。
func NewGenericHandler(svc *service.GenericService, apiPrefix string, allowSlash bool) *GenericHandler {
	return &GenericHandler{
		service:    svc,
		apiPrefix:  apiPrefix,
		allowSlash: allowSlash,
	}
}

// List 返回集合中所有文档。
func (h *GenericHandler) List(w http.ResponseWriter, r *http.Request) {
	slog.Debug("handler.list", "prefix", h.apiPrefix)

	docs, err := h.service.List(r.Context())
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, model.NewListResponse(docs))
}

// Get 按名称获取单个文档。
func (h *GenericHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := h.extractName(r)
	if name == "" {
		writeError(w, http.StatusBadRequest, "缺少资源名称")
		return
	}
	slog.Debug("handler.get", "prefix", h.apiPrefix, "name", name)

	doc, err := h.service.Get(r.Context(), name)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, doc)
}

// Create 创建新文档。
func (h *GenericHandler) Create(w http.ResponseWriter, r *http.Request) {
	doc, ok := decodeBody(w, r)
	if !ok {
		return
	}
	slog.Debug("handler.create", "prefix", h.apiPrefix, "name", doc.Name)

	if err := h.service.Create(r.Context(), doc); err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, doc)
}

// Update 更新已有文档。
func (h *GenericHandler) Update(w http.ResponseWriter, r *http.Request) {
	name := h.extractName(r)
	if name == "" {
		writeError(w, http.StatusBadRequest, "缺少资源名称")
		return
	}

	doc, ok := decodeBody(w, r)
	if !ok {
		return
	}
	slog.Debug("handler.update", "prefix", h.apiPrefix, "name", name)

	if err := h.service.Update(r.Context(), name, doc); err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, doc)
}

// Delete 删除文档。
func (h *GenericHandler) Delete(w http.ResponseWriter, r *http.Request) {
	name := h.extractName(r)
	if name == "" {
		writeError(w, http.StatusBadRequest, "缺少资源名称")
		return
	}
	slog.Debug("handler.delete", "prefix", h.apiPrefix, "name", name)

	if err := h.service.Delete(r.Context(), name); err != nil {
		handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// extractName 从 URL 中提取资源名称。
func (h *GenericHandler) extractName(r *http.Request) string {
	if h.allowSlash {
		return pathNameAfterPrefix(r, h.apiPrefix+"/")
	}
	return pathName(r)
}
