package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/npc-admin/backend/internal/model"
	"github.com/npc-admin/backend/internal/store"
	"github.com/npc-admin/backend/internal/validator"
)

const maxBodySize = 1 << 20 // 1MB

// writeJSON 写入 JSON 响应。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError 写入错误响应。
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, model.ErrorResponse{Error: msg})
}

// handleServiceError 将 service 层错误映射为 HTTP 响应。
func handleServiceError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "记录不存在")
		return
	}
	if errors.Is(err, store.ErrDuplicate) {
		writeError(w, http.StatusConflict, "名称已存在")
		return
	}
	var ve *validator.ValidationError
	if errors.As(err, &ve) {
		writeError(w, http.StatusUnprocessableEntity, ve.Error())
		return
	}
	// 500: 不暴露原始 error
	slog.Error("handler.internal_error", "err", err)
	writeError(w, http.StatusInternalServerError, "服务器内部错误，请联系开发人员")
}

// pathName 从 URL path 末尾提取 name 参数并 URL 解码。
// 例如 /api/v1/event-types/explosion → "explosion"
func pathName(r *http.Request) string {
	parts := strings.Split(strings.TrimRight(r.URL.Path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// pathNameAfterPrefix 从 URL 中截取指定前缀之后的部分作为 name。
// 用于 name 可能包含 "/" 的资源（如行为树 "civilian/idle"）。
// 优先使用 RawPath（保留 %2F 编码），再 URL 解码得到原始 name。
// 例如 /api/v1/bt-trees/civilian%2Fidle（前缀 /api/v1/bt-trees/）→ "civilian/idle"
func pathNameAfterPrefix(r *http.Request, prefix string) string {
	// 优先用 RawPath（保留 %2F），如果为空则回退到 Path
	path := r.URL.RawPath
	if path == "" {
		path = r.URL.Path
	}
	raw := strings.TrimPrefix(path, prefix)
	raw = strings.TrimRight(raw, "/")
	if raw == "" {
		return ""
	}
	decoded, err := url.PathUnescape(raw)
	if err != nil {
		return raw
	}
	return decoded
}

// decodeBody 读取并解析请求体为 Document。
func decodeBody(w http.ResponseWriter, r *http.Request) (model.Document, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)
	var doc model.Document
	if err := json.NewDecoder(r.Body).Decode(&doc); err != nil {
		writeError(w, http.StatusBadRequest, "请求体格式错误")
		return model.Document{}, false
	}
	return doc, true
}
