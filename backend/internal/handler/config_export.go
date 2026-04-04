package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/npc-admin/backend/internal/model"
	"github.com/npc-admin/backend/internal/store"
)

// ConfigExportHandler 处理配置导出的 HTTP 请求。
// 供游戏服务端启动时拉取全量配置，直接调用 Store 层（不经过 Service/Cache）。
type ConfigExportHandler struct {
	store store.Store
}

// NewConfigExportHandler 创建配置导出 handler。
func NewConfigExportHandler(s store.Store) *ConfigExportHandler {
	return &ConfigExportHandler{store: s}
}

// ExportCollection 返回指定 collection 的全量配置列表。
// 用法：mux.HandleFunc("/api/configs/event_types", h.ExportCollection("event_types"))
func (h *ConfigExportHandler) ExportCollection(collection string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.Debug("config_export.list", "collection", collection)

		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()

		docs, err := h.store.List(ctx, collection)
		if err != nil {
			slog.Error("config_export.list_error", "collection", collection, "err", err)
			writeError(w, http.StatusInternalServerError, "服务器内部错误，请联系开发人员")
			return
		}

		writeJSON(w, http.StatusOK, model.NewListResponse(docs))
	}
}
