# 任务拆解：配置导出 API（config-export）

## [x] T1: 新增 ConfigExportHandler (R1, R2, R3, R4, R5, R6)

**涉及文件**：
- `backend/internal/handler/config_export.go`（新增）

**做什么**：
- 创建 `ConfigExportHandler` struct，依赖 `store.Store`
- 实现 `ExportCollection(collection string) http.HandlerFunc` 闭包方法
- 内部逻辑：5s 超时 context → Store.List → model.NewListResponse → writeJSON

**做完的样子**：文件存在，编译通过（`go build ./...`）

---

## [x] T2: 注册路由 + 注入依赖 (R7, R8)

**涉及文件**：
- `backend/internal/handler/router.go`（修改）
- `backend/cmd/admin/main.go`（修改）

**做什么**：
- router.go：NewRouter 新增 `configExport *ConfigExportHandler` 参数，注册 4 条 `/api/configs/` 路由，套 corsMiddleware
- main.go：创建 `ConfigExportHandler`（传入 mongoStore），传给 NewRouter

**做完的样子**：`docker compose up --build` 成功启动，`curl http://localhost:9821/api/configs/event_types` 返回 `{"items": [...]}`

---

## T3: 端到端验证 + 文档同步 (R1-R8)

**涉及文件**：
- `docs/specs/config-export/tasks.md`（标记完成）
- `CLAUDE.md`（更新架构描述，新增导出 API 说明）

**做什么**：
- 用 curl 逐个验证 4 个接口：有数据返回正确、空 collection 返回 `{"items": []}`
- 验证 CORS header 存在
- 同步 CLAUDE.md 中的架构说明和接口文档
- 通知游戏服务端接口已就绪

**做完的样子**：4 个接口全部返回正确数据，CLAUDE.md 已更新，游戏服务端可以开始对接
