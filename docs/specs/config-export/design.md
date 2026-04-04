# 设计方案：配置导出 API（config-export）

## 方案描述

在现有后端中新增一组只读 GET 接口，路径前缀 `/api/configs/`，直接调用 Store.List 查询 MongoDB 并返回全量配置。

### 接口定义

```
GET /api/configs/event_types  → {"items": [{name, config}, ...]}
GET /api/configs/npc_types    → {"items": [{name, config}, ...]}
GET /api/configs/fsm_configs  → {"items": [{name, config}, ...]}
GET /api/configs/bt_trees     → {"items": [{name, config}, ...]}
```

- HTTP 200，Content-Type: application/json
- 空 collection → `{"items": []}`
- 错误 → `{"error": "..."}` + 500

### 代码结构

新增 `backend/internal/handler/config_export.go`：

```go
type ConfigExportHandler struct {
    store store.Store
}

func NewConfigExportHandler(s store.Store) *ConfigExportHandler {
    return &ConfigExportHandler{store: s}
}

func (h *ConfigExportHandler) ExportCollection(collection string) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
        defer cancel()
        docs, err := h.store.List(ctx, collection)
        if err != nil {
            slog.Error("config_export.list", "collection", collection, "err", err)
            writeError(w, http.StatusInternalServerError, "服务器内部错误，请联系开发人员")
            return
        }
        writeJSON(w, http.StatusOK, model.NewListResponse(docs))
    }
}
```

要点：
- **直接依赖 Store 接口**，不经过 Service 层（不需要校验、缓存失效等写操作逻辑）
- **ExportCollection 用闭包返回 HandlerFunc**，4 个接口复用同一方法，只传不同 collection 名
- **自带 5s 超时 context**，遵守 red-line（禁止无超时的外部调用）

### 路由注册

`router.go` 中 NewRouter 新增参数 `configExport *ConfigExportHandler`：

```go
mux.HandleFunc("/api/configs/event_types", corsMiddleware(configExport.ExportCollection("event_types")))
mux.HandleFunc("/api/configs/npc_types", corsMiddleware(configExport.ExportCollection("npc_types")))
mux.HandleFunc("/api/configs/fsm_configs", corsMiddleware(configExport.ExportCollection("fsm_configs")))
mux.HandleFunc("/api/configs/bt_trees", corsMiddleware(configExport.ExportCollection("bt_trees")))
```

### main.go 改动

```go
configExportH := handler.NewConfigExportHandler(mongoStore)
router := handler.NewRouter(eventTypeH, npcTypeH, fsmConfigH, btTreeH, configExportH)
```

## 方案对比

| 维度 | 方案 A：handler 直接调 Store（选定） | 方案 B：复用现有 Service.List |
|------|--------------------------------------|-------------------------------|
| 依赖链 | handler → store | handler → service → cache → store |
| 缓存 | 不走 Redis 缓存 | 走 Redis 缓存 |
| 适用场景 | 低频调用（服务端启动时一次） | 高频调用 |
| 复杂度 | 新增 1 个 handler 文件 | 需要 4 个 service 引用 |

**选 A 的理由**：导出接口只在游戏服务端启动时调一次，不需要缓存。走 Service 层会引入 4 个 service 依赖和不必要的 cache 交互。直接调 Store.List 最简单，且 Store 接口已有——不新增任何抽象。

## 红线检查

逐条对照 `docs/architecture/red-lines.md`：

| 红线 | 是否涉及 | 说明 |
|------|---------|------|
| 禁止暴露技术细节给策划 | 不涉及 | 接口面向服务端，非面向 UI |
| 禁止破坏游戏服务端数据格式 | 不涉及 | 只读，不写入 |
| 禁止安全隐患 | OK | 不接受用户输入，无注入风险 |
| 禁止缓存与数据库不一致 | 不涉及 | 不使用缓存 |
| 禁止引用完整性破坏 | 不涉及 | 只读 |
| 禁止 MongoDB 操作符注入 | OK | 查询为 `find({})`，无用户输入 |
| 禁止无超时的外部调用 | OK | handler 内 `context.WithTimeout(5s)` |
| 禁止与游戏服务端 Schema 不一致 | OK | 透传 `{name, config}` 原始结构，不做任何转换 |
| 禁止绕过后端 API 直接操作 MongoDB | 不涉及 | 只读查询 |
| 禁止收到协作方请求后不回复就动手 | OK | 已先回复再动手 |

**无红线冲突。**

## 扩展性影响

- **新增配置类型**：在 router.go 加一行 `mux.HandleFunc("/api/configs/<新collection>", ...)` 即可，无需改 handler 代码——ExportCollection 是通用的
- **新增表单字段**：完全透明，config 是 `json.RawMessage` 透传

**正面影响**：导出接口的存在使得 ADMIN 平台成为配置的唯一出口，所有消费方通过 HTTP API 获取，架构更清晰。

## 依赖方向

```
main.go
  ↓ 创建
ConfigExportHandler → Store (接口)
                        ↓ 实现
                      MongoStore → MongoDB
```

单向向下，无循环依赖。ConfigExportHandler 不依赖 service/cache/validator。

## Go 陷阱检查

| 陷阱 | 是否涉及 | 说明 |
|------|---------|------|
| nil slice → JSON null | OK | 已由 `model.NewListResponse` 保证 `[]` |
| 共享状态 | 无 | handler 无状态，Store 已线程安全 |
| error 处理 | OK | 500 不暴露原始 error，写 slog |
| context 超时 | OK | 5s WithTimeout |

## 前端陷阱检查

不涉及前端改动。

## 配置变更

无新增配置文件。无新增环境变量（ADMIN 侧不需要，游戏服务端侧的 `NPC_ADMIN_API` 是他们的事）。

## 测试策略

- **单元测试**：不需要——handler 逻辑极简（调 Store.List + writeJSON），集成测试覆盖更有价值
- **集成测试**：通过 `curl` 或游戏服务端实际调用验证：
  1. 启动 ADMIN 服务（docker compose up）
  2. 通过管理接口写入测试数据
  3. 调用 `GET /api/configs/event_types` 等接口，验证返回格式和数据
  4. 游戏服务端用 HTTPSource 拉取，验证能正常加载
