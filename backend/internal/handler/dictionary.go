package handler

import (
	"context"
	"log/slog"

	"github.com/yqihe/npc-ai-admin/backend/internal/cache"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
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
func (h *DictionaryHandler) List(_ context.Context, req *model.DictListRequest) (*model.DictListResponse, error) {
	if req.Group == "" {
		return nil, errcode.Newf(errcode.ErrBadRequest, "参数 group 不能为空")
	}

	slog.Debug("handler.字典列表", "group", req.Group)

	items := h.dictCache.ListByGroup(req.Group)
	return &model.DictListResponse{Items: items}, nil
}
