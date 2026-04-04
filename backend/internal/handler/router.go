package handler

import (
	"net/http"
	"strings"
)

// NewRouter 注册所有 API 路由并返回 http.Handler。
// Go 1.21 的 http.ServeMux 不支持方法匹配，手动分发。
func NewRouter(
	eventType *EventTypeHandler,
	npcType *NpcTypeHandler,
	fsmConfig *FsmConfigHandler,
	btTree *BtTreeHandler,
	configExport *ConfigExportHandler,
) http.Handler {
	mux := http.NewServeMux()

	// 管理接口（前端 CRUD）
	mux.HandleFunc("/api/v1/event-types", corsMiddleware(resourceHandler(eventType.List, eventType.Create)))
	mux.HandleFunc("/api/v1/event-types/", corsMiddleware(resourceItemHandler(eventType.Get, eventType.Update, eventType.Delete)))

	mux.HandleFunc("/api/v1/npc-types", corsMiddleware(resourceHandler(npcType.List, npcType.Create)))
	mux.HandleFunc("/api/v1/npc-types/", corsMiddleware(resourceItemHandler(npcType.Get, npcType.Update, npcType.Delete)))

	mux.HandleFunc("/api/v1/fsm-configs", corsMiddleware(resourceHandler(fsmConfig.List, fsmConfig.Create)))
	mux.HandleFunc("/api/v1/fsm-configs/", corsMiddleware(resourceItemHandler(fsmConfig.Get, fsmConfig.Update, fsmConfig.Delete)))

	mux.HandleFunc("/api/v1/bt-trees", corsMiddleware(resourceHandler(btTree.List, btTree.Create)))
	mux.HandleFunc("/api/v1/bt-trees/", corsMiddleware(resourceItemHandler(btTree.Get, btTree.Update, btTree.Delete)))

	// 配置导出接口（供游戏服务端拉取全量配置）
	mux.HandleFunc("/api/configs/event_types", corsMiddleware(configExport.ExportCollection("event_types")))
	mux.HandleFunc("/api/configs/npc_types", corsMiddleware(configExport.ExportCollection("npc_types")))
	mux.HandleFunc("/api/configs/fsm_configs", corsMiddleware(configExport.ExportCollection("fsm_configs")))
	mux.HandleFunc("/api/configs/bt_trees", corsMiddleware(configExport.ExportCollection("bt_trees")))

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
