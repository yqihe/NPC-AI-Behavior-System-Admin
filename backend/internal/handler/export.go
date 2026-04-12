package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yqihe/npc-ai-admin/backend/internal/service"
)

// ExportHandler 配置导出 API
//
// 统一放所有 /api/configs/* 导出接口。
// 不走 WrapCtx（导出 API 格式与 CRUD 不同）。
type ExportHandler struct {
	eventTypeService *service.EventTypeService
}

// NewExportHandler 创建 ExportHandler
func NewExportHandler(eventTypeService *service.EventTypeService) *ExportHandler {
	return &ExportHandler{
		eventTypeService: eventTypeService,
	}
}

// exportResponse 导出 API 统一响应格式
type exportResponse struct {
	Items interface{} `json:"items"`
}

// EventTypes GET /api/configs/event_types
//
// 返回所有已启用且未删除的事件类型。
// config 字段直接从 config_json 列原样展开，不经过 Go struct 中转。
func (h *ExportHandler) EventTypes(c *gin.Context) {
	slog.Debug("handler.export.event_types")

	items, err := h.eventTypeService.ExportAll(c.Request.Context())
	if err != nil {
		slog.Error("handler.export.event_types.error", "error", err)
		c.JSON(http.StatusInternalServerError, exportResponse{Items: make([]interface{}, 0)})
		return
	}

	// 空数据返回 {"items": []}
	if len(items) == 0 {
		c.JSON(http.StatusOK, exportResponse{Items: make([]interface{}, 0)})
		return
	}

	c.JSON(http.StatusOK, exportResponse{Items: items})
}
