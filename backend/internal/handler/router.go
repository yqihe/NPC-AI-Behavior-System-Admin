package handler

import (
	"net/http"
	"strings"
)

// EntityConfig 定义一个可管理的实体类型的路由配置。
type EntityConfig struct {
	APIPrefix  string // 如 "/api/v1/npc-templates"
	Collection string // 如 "npc_templates"
	AllowSlash bool   // name 是否允许 "/"（行为树需要）
}

// NewRouter 注册所有 API 路由并返回 http.Handler。
// handlers 是通过 EntityConfig 实例化的通用 CRUD handler 列表。
func NewRouter(
	handlers []*GenericHandler,
	readonlyHandlers []*ReadOnlyHandler,
	configExport *ConfigExportHandler,
	exportCollections []EntityConfig,
) http.Handler {
	mux := http.NewServeMux()

	// 注册管理接口（前端 CRUD）
	for _, h := range handlers {
		prefix := h.apiPrefix
		mux.HandleFunc(prefix, corsMiddleware(resourceHandler(h.List, h.Create)))
		mux.HandleFunc(prefix+"/", corsMiddleware(resourceItemHandler(h.Get, h.Update, h.Delete)))
	}

	// 注册只读接口（ADMIN 元数据）
	for _, h := range readonlyHandlers {
		prefix := h.apiPrefix
		mux.HandleFunc(prefix, corsMiddleware(readOnlyResourceHandler(h.List)))
		mux.HandleFunc(prefix+"/", corsMiddleware(readOnlyResourceItemHandler(h.Get)))
	}

	// 注册配置导出接口（供游戏服务端拉取全量配置）
	for _, ec := range exportCollections {
		mux.HandleFunc("/api/configs/"+ec.Collection, corsMiddleware(configExport.ExportCollection(ec.Collection)))
	}

	return mux
}

// resourceHandler 处理集合级别的请求（列表 / 创建）。
func resourceHandler(list, create http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			list(w, r)
		case http.MethodPost:
			create(w, r)
		case http.MethodOptions:
			w.WriteHeader(http.StatusNoContent)
		default:
			writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		}
	}
}

// readOnlyResourceHandler 处理只读集合级别的请求（仅列表）。
func readOnlyResourceHandler(list http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			list(w, r)
		case http.MethodOptions:
			w.WriteHeader(http.StatusNoContent)
		default:
			writeError(w, http.StatusMethodNotAllowed, "该接口为只读，不支持写操作")
		}
	}
}

// readOnlyResourceItemHandler 处理只读单条资源的请求（仅详情）。
func readOnlyResourceItemHandler(get http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			get(w, r)
		case http.MethodOptions:
			w.WriteHeader(http.StatusNoContent)
		default:
			writeError(w, http.StatusMethodNotAllowed, "该接口为只读，不支持写操作")
		}
	}
}

// resourceItemHandler 处理单条资源的请求（详情 / 更新 / 删除）。
func resourceItemHandler(get, update, del http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			get(w, r)
		case http.MethodPut:
			update(w, r)
		case http.MethodDelete:
			del(w, r)
		case http.MethodOptions:
			w.WriteHeader(http.StatusNoContent)
		default:
			writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		}
	}
}

// corsMiddleware 允许前端跨域访问。
func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Methods", strings.Join([]string{"GET", "POST", "PUT", "DELETE", "OPTIONS"}, ", "))
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next(w, r)
	}
}
