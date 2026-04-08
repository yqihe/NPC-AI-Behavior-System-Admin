package handler

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yqihe/npc-ai-admin/backend/internal/cache"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
)

// DictionaryHandler dictionaries HTTP handler
type DictionaryHandler struct {
	dictCache *cache.DictCache
}

// NewDictionaryHandler 创建 DictionaryHandler
func NewDictionaryHandler(dictCache *cache.DictCache) *DictionaryHandler {
	return &DictionaryHandler{dictCache: dictCache}
}

// List 查询指定 group 的字典选项
// GET /api/v1/dictionaries?group=field_type
func (h *DictionaryHandler) List(c *gin.Context) {
	group := c.Query("group")
	if group == "" {
		c.JSON(http.StatusOK, model.Response{
			Code:    errcode.ErrBadRequest,
			Message: "参数 group 不能为空",
		})
		return
	}

	slog.Debug("handler.字典列表", "group", group)

	items := h.dictCache.ListByGroup(group)

	respondOK(c, items, errcode.Msg(errcode.Success))
}
