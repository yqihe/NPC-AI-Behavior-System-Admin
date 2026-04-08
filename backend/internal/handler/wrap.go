package handler

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yqihe/npc-ai-admin/backend/internal/errcode"
	"github.com/yqihe/npc-ai-admin/backend/internal/model"
)

// WrapCtx 将 handler 包装为 gin.HandlerFunc
// handler 签名统一为：func(context.Context, *Req) (*Resp, error)
// 自动处理：JSON 解析 → 调用 → 统一响应
func WrapCtx[Req any, Resp any](fn func(context.Context, *Req) (*Resp, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req Req
		if err := c.ShouldBindJSON(&req); err != nil {
			slog.Debug("handler.参数解析失败", "error", err)
			writeJSON(c, errcode.ErrBadRequest, nil, "请求参数格式错误")
			return
		}

		resp, err := fn(c.Request.Context(), &req)
		if err != nil {
			writeError(c, err, resp)
			return
		}

		writeJSON(c, errcode.Success, resp, errcode.Msg(errcode.Success))
	}
}

// writeError 统一错误响应
func writeError(c *gin.Context, err error, data any) {
	var ecErr *errcode.Error
	if errors.As(err, &ecErr) {
		writeJSON(c, ecErr.Code, data, ecErr.Message)
		return
	}
	slog.Error("handler.内部错误", "error", err)
	writeJSON(c, errcode.ErrInternal, nil, errcode.Msg(errcode.ErrInternal))
}

// writeJSON 写 JSON 响应
func writeJSON(c *gin.Context, code int, data any, message string) {
	c.JSON(http.StatusOK, model.Response{
		Code:    code,
		Data:    data,
		Message: message,
	})
}
