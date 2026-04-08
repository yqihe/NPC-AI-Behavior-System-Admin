package handler

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/yqihe/npc-ai-admin/backend/internal/cache"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
)

// DictionaryHandler 字典业务处理
type DictionaryHandler struct {
	dictCache *cache.DictCache
}

// NewDictionaryHandler 创建 DictionaryHandler
func NewDictionaryHandler(dictCache *cache.DictCache) *DictionaryHandler {
	return &DictionaryHandler{dictCache: dictCache}
}

// List 查询指定 group 的字典选项
func (h *DictionaryHandler) List(c *gin.Context) (any, error) {
	group := c.Query("group")
	if group == "" {
		return nil, errcode.Newf(errcode.ErrBadRequest, "参数 group 不能为空")
	}

	slog.Debug("handler.字典列表", "group", group)

	return h.dictCache.ListByGroup(group), nil
}
