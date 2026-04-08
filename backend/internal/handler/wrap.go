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

// WrapPost 包装 POST handler（需要 gin.Context + 请求体）
// handler 签名：func(*gin.Context, *Req) (*Resp, error)
func WrapPost[Req any, Resp any](fn func(*gin.Context, *Req) (*Resp, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req Req
		if err := c.ShouldBindJSON(&req); err != nil {
			slog.Debug("handler.参数解析失败", "error", err)
			writeJSON(c, errcode.ErrBadRequest, nil, "请求参数格式错误")
			return
		}

		resp, err := fn(c, &req)
		if err != nil {
			writeError(c, err, resp)
			return
		}

		writeJSON(c, errcode.Success, resp, errcode.Msg(errcode.Success))
	}
}

// WrapCtx 包装 POST handler（纯 context + 请求体，不需要 gin.Context）
// handler 签名：func(context.Context, *Req) (*Resp, error)
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

// WrapGet 包装 GET handler（从 query params 取参，不解析 body）
// handler 签名：func(*gin.Context) (any, error)
func WrapGet(fn func(*gin.Context) (any, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		resp, err := fn(c)
		if err != nil {
			writeError(c, err, nil)
			return
		}

		writeJSON(c, errcode.Success, resp, errcode.Msg(errcode.Success))
	}
}

// writeError 统一错误响应
func writeError(c *gin.Context, err error, data any) {
	var ecErr *errcode.Error
	if errors.As(err, &ecErr) {
		// 业务错误：可能带 data（如删除时返回引用列表）
		writeJSON(c, ecErr.Code, data, ecErr.Message)
		return
	}
	// 系统错误：不暴露 Go error
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
