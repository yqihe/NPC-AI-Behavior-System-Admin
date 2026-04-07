package handler

import (
	"log/slog"
	"net/http"

	"github.com/npc-admin/backend/internal/model"
	"github.com/npc-admin/backend/internal/service"
)

// ReadOnlyHandler 提供只读 HTTP API（List + Get），不支持写操作。
// 用于 component-schemas、npc-presets 等 ADMIN 元数据。
type ReadOnlyHandler struct {
	service   *service.ReadOnlyService
	apiPrefix string
}

// NewReadOnlyHandler 创建只读 handler。
func NewReadOnlyHandler(svc *service.ReadOnlyService, apiPrefix string) *ReadOnlyHandler {
	return &ReadOnlyHandler{service: svc, apiPrefix: apiPrefix}
}

// List 返回集合中所有文档。
func (h *ReadOnlyHandler) List(w http.ResponseWriter, r *http.Request) {
	slog.Debug("handler.readonly_list", "prefix", h.apiPrefix)

	docs, err := h.service.List(r.Context())
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, model.NewListResponse(docs))
}

// Get 按名称获取单个文档。
func (h *ReadOnlyHandler) Get(w http.ResponseWriter, r *http.Request) {
	name := pathName(r)
	if name == "" {
		writeError(w, http.StatusBadRequest, "缺少资源名称")
		return
	}
	slog.Debug("handler.readonly_get", "prefix", h.apiPrefix, "name", name)

	doc, err := h.service.Get(r.Context(), name)
	if err != nil {
		handleServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, doc)
}
