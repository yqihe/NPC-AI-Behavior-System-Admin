# 整体架构规划 — 任务拆解

## 任务总览

```
T1  Go 模块初始化 + 数据模型
T2  MongoDB Store 层
T3  Redis Cache 层
T4  事件类型 Validator
T5  NPC 类型 / FSM / BT Validator
T6  事件类型 Service + Handler
T7  NPC 类型 Service + Handler
T8  FSM 配置 Service + Handler
T9  BT 配置 Service + Handler
T10 路由注册 + main.go + 启动完整后端
T11 前端项目初始化 + 布局 + 路由
T12 前端 API 层 + Axios 封装
T13 事件类型列表页 + 表单页
T14 NPC 类型列表页 + 表单页
T15 FSM 配置列表页 + 表单页（含条件编辑器）
T16 BT 配置列表页 + 表单页（含节点编辑器）
T17 Docker Compose + Dockerfile + Nginx
T18 后端集成测试
```

---

## T1: Go 模块初始化 + 数据模型 (R2, R4, R10)

**涉及文件**：
- `backend/go.mod`
- `backend/internal/model/document.go`

**做完的样子**：
- `go.mod` 已初始化，声明 module path，依赖 mongo-driver、go-redis
- `Document` struct 定义完成，带 `json` + `bson` tag
- `json.RawMessage` ↔ `bson.Raw` 转换辅助函数可用
- nil slice/map 初始化辅助函数（`EnsureItems`）可用
- `go build ./...` 通过

[x] 已完成

---

## T2: MongoDB Store 层 (R2, R4, R9, R10)

**涉及文件**：
- `backend/internal/store/mongo.go`
- `backend/internal/store/store.go`

**做完的样子**：
- `store.go` 定义 `Store` 接口：`List(ctx, collection) → []Document`、`Get(ctx, collection, name)`、`Create(ctx, collection, doc)`、`Update(ctx, collection, name, doc)`、`Delete(ctx, collection, name)`
- `mongo.go` 实现 `MongoStore`：连接初始化、确保 4 个 collection 的 name unique index
- 所有操作带 `context.WithTimeout(ctx, 5*time.Second)`
- `FindOne` 无结果返回自定义 `ErrNotFound`，`InsertOne` 重复返回自定义 `ErrDuplicate`
- `_id` / ObjectId 不出现在返回结果中
- List 返回空 slice（不是 nil）
- `json.RawMessage` ↔ `bson.Raw` 转换在此层处理
- `go build ./...` 通过

[x] 已完成

---

## T3: Redis Cache 层 (R5)

**涉及文件**：
- `backend/internal/cache/redis.go`
- `backend/internal/cache/cache.go`

**做完的样子**：
- `cache.go` 定义 `Cache` 接口：`GetList(ctx, collection) → ([]Document, error)`、`SetList(ctx, collection, docs)`、`Invalidate(ctx, collection)`
- `redis.go` 实现 `RedisCache`：连接初始化，key 前缀 `admin:`，TTL 5 分钟
- `GetList` 未命中返回自定义 `ErrCacheMiss`（封装 redis.Nil）
- Redis 连接失败时所有方法返回 error（service 层降级处理）
- `go build ./...` 通过

[x] 已完成

---

## T4: 事件类型 Validator (R8)

**涉及文件**：
- `backend/internal/validator/event_type.go`
- `backend/internal/validator/common.go`

**做完的样子**：
- `common.go` 定义 `ValidationError` 类型（包含 `[]string` 错误列表，中文描述）和通用校验辅助函数
- `event_type.go` 实现 `ValidateEventType(config json.RawMessage) error`：
  - 反序列化为临时结构体
  - 校验 name 非空、severity ∈ [0,100]、ttl > 0、perception_mode ∈ {visual, auditory, global}、range ≥ 0
- 校验失败返回 `ValidationError`，含所有违反项的中文描述
- `go build ./...` 通过

[x] 已完成

---

## T5: NPC 类型 / FSM / BT Validator (R8)

**涉及文件**：
- `backend/internal/validator/npc_type.go`
- `backend/internal/validator/fsm_config.go`
- `backend/internal/validator/bt_tree.go`

**做完的样子**：
- `npc_type.go`：校验 type_name 非空、perception 范围 ≥ 0、fsm_ref 存在（调 store 检查）、bt_refs 每个值存在（调 store 检查）
- `fsm_config.go`：校验 initial_state 在 states 中、transition from/to 在 states 中、priority > 0、condition 结构合法（递归校验 and/or/叶子节点）
- `bt_tree.go`：校验节点 type 合法、复合节点有 children、叶子节点有 params、递归校验子节点
- 所有错误描述为中文
- `go build ./...` 通过

[x] 已完成

---

## T6: 事件类型 Service + Handler (R2, R3, R5, R9, R10)

**涉及文件**：
- `backend/internal/service/event_type.go`
- `backend/internal/handler/event_type.go`

**做完的样子**：
- `service/event_type.go`：EventTypeService 编排 store + cache + validator
  - List：cache hit → 返回；miss → store 查询 → set cache → 返回
  - Get：直接 store 查询
  - Create：validate → store.Create → cache.Invalidate
  - Update：validate → store.Update → cache.Invalidate
  - Delete：store.Delete → cache.Invalidate（事件类型无引用检查）
  - Redis 失败降级，记 slog.Warn
- `handler/event_type.go`：5 个 HTTP handler 函数
  - 请求解析（`json.NewDecoder` + `http.MaxBytesReader`）
  - 调 service
  - 错误映射：ErrNotFound→404、ErrDuplicate→409、ValidationError→422、其他→500（不暴露原始 error）
  - 响应格式：`{"items":[...]}` / `{"name":"...","config":{...}}` / `{"error":"中文"}`
- `go build ./...` 通过

[x] 已完成

---

## T7: NPC 类型 Service + Handler (R2, R3, R5, R8, R9)

**涉及文件**：
- `backend/internal/service/npc_type.go`
- `backend/internal/handler/npc_type.go`

**做完的样子**：
- 与 T6 结构一致
- Create/Update 时 validator 做 fsm_ref / bt_refs 引用检查
- Delete 前检查无其他配置引用此 NPC 类型（当前无引用关系，直接删）
- `go build ./...` 通过

[x] 已完成

---

## T8: FSM 配置 Service + Handler (R2, R3, R5, R8, R9)

**涉及文件**：
- `backend/internal/service/fsm_config.go`
- `backend/internal/handler/fsm_config.go`

**做完的样子**：
- 与 T6 结构一致
- Delete 前检查 npc_types 中是否有 fsm_ref 指向此 name，有则拒绝（422 + 中文提示"该状态机正在被 NPC 类型 xxx 引用，无法删除"）
- `go build ./...` 通过

[x] 已完成

---

## T9: BT 配置 Service + Handler (R2, R3, R5, R8, R9)

**涉及文件**：
- `backend/internal/service/bt_tree.go`
- `backend/internal/handler/bt_tree.go`

**做完的样子**：
- 与 T6 结构一致
- Delete 前检查 npc_types 中是否有 bt_refs 值指向此 name，有则拒绝
- `go build ./...` 通过

[x] 已完成

---

## T10: 路由注册 + main.go + 启动完整后端 (R1, R2)

**涉及文件**：
- `backend/internal/handler/router.go`
- `backend/cmd/admin/main.go`

**做完的样子**：
- `router.go`：注册所有路由 `GET/POST/PUT/DELETE /api/v1/{collection}/{name?}`，CORS 中间件允许前端跨域
- `main.go`：读取环境变量（MONGO_URI、REDIS_ADDR、LISTEN_ADDR）、初始化 MongoDB 连接 + Redis 连接、创建 store/cache/validator/service/handler、启动 HTTP server :9821、graceful shutdown
- `go build ./cmd/admin/` 产出可执行文件
- 本地 `go run ./cmd/admin/` 配合 docker 中的 mongo + redis 可正常启动，curl 测试 20 个端点均可达
- `go test ./...` 通过

[x] 已完成

---

## T11: 前端项目初始化 + 布局 + 路由 (R6)

**涉及文件**：
- `frontend/package.json`
- `frontend/vite.config.js`
- `frontend/src/main.js`
- `frontend/src/App.vue`
- `frontend/src/router/index.js`
- `frontend/src/components/AppLayout.vue`
- `frontend/index.html`

**做完的样子**：
- `npm create vue` 初始化项目（或手动搭建）
- 安装 `element-plus`、`vue-router`、`axios`、`unplugin-vue-components`、`unplugin-auto-import`
- `vite.config.js` 配置 Element Plus 按需引入 + dev proxy（`/api` → `localhost:9821`）
- `AppLayout.vue`：el-container + el-aside（侧边导航菜单，4 个模块） + el-main
- `router/index.js`：8 个路由（4 个列表页 + 4 个表单页），懒加载
- `npm run dev` 可启动，浏览器看到布局框架 + 侧边导航

[x] 已完成

---

## T12: 前端 API 层 + Axios 封装 (R6, R7)

**涉及文件**：
- `frontend/src/api/index.js`
- `frontend/src/api/eventType.js`
- `frontend/src/api/npcType.js`
- `frontend/src/api/fsmConfig.js`
- `frontend/src/api/btTree.js`

**做完的样子**：
- `index.js`：Axios 实例，baseURL 从 `import.meta.env.VITE_API_BASE` 读取，响应拦截器（错误弹 ElMessage + reject）
- 4 个 API 文件各导出 `list()`、`get(name)`、`create(data)`、`update(name, data)`、`remove(name)` 五个函数
- 无硬编码 URL

[x] 已完成

---

## T13: 事件类型列表页 + 表单页 (R6, R7)

**涉及文件**：
- `frontend/src/views/EventTypeList.vue`
- `frontend/src/views/EventTypeForm.vue`

**做完的样子**：
- **列表页**：el-table 展示 name、severity、ttl、perception_mode、range；顶部"新建"按钮跳转表单页；每行"编辑"/"删除"操作；删除用 el-popconfirm 确认
- **表单页**：name（el-input）、severity（el-slider 0-100）、ttl（el-input-number）、perception_mode（el-radio-group）、range（el-slider 0-1000）；el-form 校验；提交按钮 loading 防重复
- 新建/编辑复用同一页面，通过路由参数区分
- 浏览器可完成事件类型的完整 CRUD 流程

[x] 已完成

---

## T14: NPC 类型列表页 + 表单页 (R6, R7)

**涉及文件**：
- `frontend/src/views/NpcTypeList.vue`
- `frontend/src/views/NpcTypeForm.vue`

**做完的样子**：
- **列表页**：el-table 展示 type_name、fsm_ref、bt_refs 数量、视觉/听觉范围
- **表单页**：type_name（el-input）、fsm_ref（el-select 下拉已有 FSM 列表）、bt_refs（动态表单：状态名→BT 名，状态名从 FSM 配置的 states 自动获取，BT 名用 el-select 下拉已有 BT 列表）、visual_range/auditory_range（el-slider）
- fsm_ref 选择后自动拉取 states 刷新 bt_refs 映射

[x] 已完成

---

## T15: FSM 配置列表页 + 表单页 (R6, R7)

**涉及文件**：
- `frontend/src/views/FsmConfigList.vue`
- `frontend/src/views/FsmConfigForm.vue`
- `frontend/src/components/ConditionEditor.vue`

**做完的样子**：
- **列表页**：el-table 展示 name、状态数、转换数、初始状态
- **表单页**：
  - 状态管理：动态增删状态名（el-tag + el-input）
  - initial_state：el-select 从已添加的状态中选
  - 转换列表：动态增删，每条转换含 from（el-select）、to（el-select）、priority（el-input-number）、condition（ConditionEditor 组件）
- **ConditionEditor**：递归组件
  - 叶子条件：key（el-select，中文标签）+ op（el-select）+ value/ref_key（el-input 或 el-select）
  - 组合条件：and/or 切换 + 子条件列表（动态增删，递归嵌套 ConditionEditor）

[x] 已完成

---

## T16: BT 配置列表页 + 表单页 (R6, R7)

**涉及文件**：
- `frontend/src/views/BtTreeList.vue`
- `frontend/src/views/BtTreeForm.vue`
- `frontend/src/components/BtNodeEditor.vue`

**做完的样子**：
- **列表页**：el-table 展示 name、根节点类型、子节点数（第一层）
- **表单页**：BtNodeEditor 组件编辑根节点
- **BtNodeEditor**：递归组件
  - type（el-select：sequence/selector/parallel/inverter/check_bb_float/check_bb_string/set_bb_value/stub_action）
  - 复合节点：children 列表，每个子节点递归渲染 BtNodeEditor，支持增删子节点
  - 叶子节点：params 表单（根据 type 动态渲染不同字段）
  - 树形缩进展示父子关系

[x] 已完成

---

## T17: Docker Compose + Dockerfile + Nginx (R1)

**涉及文件**：
- `docker-compose.yml`
- `Dockerfile.backend`
- `Dockerfile.frontend`
- `frontend/nginx.conf`

**做完的样子**：
- `Dockerfile.backend`：多阶段构建（golang:1.23-alpine build → alpine run），暴露 9821
- `Dockerfile.frontend`：多阶段构建（node:20-alpine build → nginx:alpine），复制 nginx.conf
- `nginx.conf`：80 端口，`/api/` 反代到 `admin-backend:9821`，其余走前端 SPA（try_files）
- `docker-compose.yml`：4 个服务（admin-backend、admin-frontend、mongo、redis）+ volumes
- `docker compose up --build` 一键启动，无报错，浏览器访问 `localhost:3000` 可正常使用

[x] 已完成

---

## T18: 后端集成测试 (R2, R3, R4, R5, R8, R9, R10)

**涉及文件**：
- `backend/internal/store/mongo_test.go`
- `backend/internal/cache/redis_test.go`
- `backend/internal/validator/validator_test.go`

**做完的样子**：
- `mongo_test.go`：连真实 MongoDB（docker），测试 CRUD 全流程、duplicate key、not found、nil slice 返回 `[]`、写入后 mongosh 验证 `{name, config}` 格式
- `redis_test.go`：连真实 Redis（docker），测试 Get/Set/Del、TTL、cache miss
- `validator_test.go`：4 种 config 的合法/非法用例、引用检查、中文错误描述
- `go test ./...` 全部通过
- `go test -race ./...` 全部通过

[x] 已完成（单元测试 41 个全部通过，集成测试需 docker 环境）

---

## 依赖顺序

```
T1 (model)
├─→ T2 (store)
│   ├─→ T4 (event validator)
│   ├─→ T5 (npc/fsm/bt validator)
│   │   ├─→ T6 (event service+handler)
│   │   ├─→ T7 (npc service+handler)
│   │   ├─→ T8 (fsm service+handler)
│   │   └─→ T9 (bt service+handler)
│   │       └─→ T10 (router + main.go)
│   └─→ T18 (集成测试，可在 T10 后开始)
├─→ T3 (cache)
│   └─→ T6~T9 也依赖 cache
│
T11 (前端初始化，可与 T1 并行)
├─→ T12 (API 层)
│   ├─→ T13 (事件类型页面)
│   ├─→ T14 (NPC 类型页面)
│   ├─→ T15 (FSM 页面)
│   └─→ T16 (BT 页面)
│
T17 (Docker，在 T10+T16 完成后)
```

**建议执行顺序**：T1 → T2 + T3 并行 → T4 → T5 → T6 → T7 → T8 → T9 → T10 → T11 → T12 → T13 → T14 → T15 → T16 → T17 → T18

前端 T11-T12 可与后端 T4-T5 并行启动。
