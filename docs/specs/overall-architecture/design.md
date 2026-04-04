# 整体架构规划 — 设计方案

## 方案描述

### 整体架构

```
┌────────────────────────────────────┐
│  Vue 3 + Element Plus 前端          │
│  Vite dev :3000 / Nginx prod :3000 │
│                                     │
│  views/    → 4 个模块页面            │
│  components/ → 通用组件              │
│  api/      → Axios 封装             │
│  router/   → Vue Router            │
└──────────────┬─────────────────────┘
               │ REST API (JSON)
               │ baseURL: /api/v1
┌──────────────▼─────────────────────┐
│  Go HTTP 后端  :9821                │
│                                     │
│  handler/  → 路由 + 请求解析/响应     │
│  service/  → 业务逻辑 + 校验调用      │
│  store/    → MongoDB CRUD           │
│  cache/    → Redis 缓存              │
│  validator/ → 配置校验器             │
│  model/    → 数据模型定义            │
└──────┬────────────┬────────────────┘
       │            │
┌──────▼──────┐ ┌──▼────────┐
│  MongoDB    │ │  Redis     │
│  :27017     │ │  :6379     │
│  db: npc_ai │ │  prefix:   │
│             │ │  admin:    │
└─────────────┘ └────────────┘
```

### 后端分层

```
handler → service → store
                  → cache
           ↓
         validator
           ↓
         model
```

- **handler**：HTTP 路由注册、请求解析（JSON→model）、调用 service、响应格式化。不含业务逻辑
- **service**：编排 store + cache + validator。写操作流程：校验 → store 写入 → cache 失效。读操作流程：cache 命中 → 返回；未命中 → store 查询 → 写 cache → 返回
- **store**：纯 MongoDB 操作。每个 collection 一组方法（List/Get/Create/Update/Delete）。所有操作带 `context.WithTimeout`。提供 `Close(ctx)` 用于优雅关闭时断开 MongoDB 连接
- **cache**：Redis 封装。只缓存 List 结果（配置数量少，单条查询不缓存）。提供 Get/Set/Del 方法。提供 `Close()` 用于优雅关闭时断开 Redis 连接
- **validator**：纯函数校验器。接收 config 数据，返回 `[]string` 错误列表或 nil。可调用 store 做引用检查
- **model**：Go struct 定义，同时带 `json` 和 `bson` tag

### REST API 设计

所有接口路径前缀 `/api/v1`：

| 方法 | 路径 | 说明 | 请求体 | 响应 |
|------|------|------|--------|------|
| GET | `/event-types` | 列表 | - | `{"items": [...]}` |
| GET | `/event-types/:name` | 详情 | - | `{"name": "...", "config": {...}}` |
| POST | `/event-types` | 创建 | `{"name": "...", "config": {...}}` | `{"name": "...", "config": {...}}` |
| PUT | `/event-types/:name` | 更新 | `{"name": "...", "config": {...}}` | `{"name": "...", "config": {...}}` |
| DELETE | `/event-types/:name` | 删除 | - | `{}` |

同样模式适用于 `/npc-types`、`/fsm-configs`、`/bt-trees`（共 4 × 5 = 20 个端点）。

错误响应统一：`{"error": "中文描述"}`。

### MongoDB 数据模型

与游戏服务端完全一致，4 个 collection，统一 `{name, config}` 格式：

```go
// model/document.go — 通用文档结构
type Document struct {
    Name   string          `json:"name" bson:"name"`
    Config json.RawMessage `json:"config" bson:"config"`
}
```

`config` 使用 `json.RawMessage`（对应 bson 序列化时用 `bson.Raw`），原因：
- 4 种配置的 config 内部结构完全不同
- 运营平台只做「透传 + 校验」，不需要把 config 反序列化为具体 Go struct
- 校验时按需解析为临时结构体，校验完丢弃

**但是**，为校验和前端表单服务，需要为每种 config 定义校验用的临时结构体（不存储，只在 validator 中使用）：

```go
// validator/event_type.go 中的校验用结构体
type eventConfig struct {
    Name            string   `json:"name"`
    DefaultSeverity *float64 `json:"default_severity"` // float64，与游戏服务端 EventTypeConfig.DefaultSeverity 一致
    DefaultTTL      *float64 `json:"default_ttl"`
    PerceptionMode  string   `json:"perception_mode"`
    Range           *float64 `json:"range"`
}
```

### MongoDB 写入细节

关键问题：`json.RawMessage` 存入 MongoDB 时，如果用标准 `bson.Marshal`，会把 JSON 字符串当作 bytes 存储，而不是 BSON 文档。游戏服务端取出时就无法正确解析。

**解决方案**：写入前用 `bson.UnmarshalExtJSON` 将 `json.RawMessage` 转换为 `bson.Raw`，保证存储为 BSON 文档结构。读取时 MongoDB driver 返回 `bson.Raw`，再用 `bson.MarshalExtJSON` 转回 JSON bytes。

```go
// store 写入时
type bsonDocument struct {
    Name   string   `bson:"name"`
    Config bson.Raw `bson:"config"`
}

func toBsonDoc(doc model.Document) (bsonDocument, error) {
    var raw bson.Raw
    err := bson.UnmarshalExtJSON(doc.Config, true, &raw)
    return bsonDocument{Name: doc.Name, Config: raw}, err
}
```

### Redis 缓存策略

| 维度 | 决策 |
|------|------|
| 缓存范围 | 仅缓存 List 结果（4 个 key） |
| key 格式 | `admin:event_types:list`、`admin:npc_types:list`、`admin:fsm_configs:list`、`admin:bt_trees:list` |
| TTL | 5 分钟（兜底过期） |
| 失效策略 | Cache-Aside：写操作成功后 Del 对应 key |
| 序列化 | `json.Marshal` / `json.Unmarshal` |
| 未命中 | `redis.Nil` → 查 MongoDB → Set 缓存 → 返回 |
| Redis 宕机 | 降级为直接查 MongoDB，记 slog.Warn，不影响功能 |

### 配置校验规则

#### 事件类型（event_types）
- `name` 非空
- `default_severity` 范围 [0, 100]（float64 类型，与游戏服务端一致）
- `default_ttl` > 0
- `perception_mode` 必须是 `visual` / `auditory` / `global` 之一
- `range` ≥ 0

#### NPC 类型（npc_types）
- `type_name` 非空
- `fsm_ref` 非空 **且 fsm_configs 中存在该 name**（引用检查）
- `bt_refs` 每个值 **在 bt_trees 中存在**（引用检查）
- `perception.visual_range` ≥ 0
- `perception.auditory_range` ≥ 0

#### FSM 配置（fsm_configs）
- `initial_state` 非空且必须在 states 列表中
- `states` 至少 1 个，且状态名不能重复（同名覆盖不报错，服务端加载时报 duplicate state error）
- 每个 transition 的 `from` 和 `to` 必须在 states 中存在
- `priority` > 0
- condition 结构合法：叶子节点需要 `key` + `op` + (`value` 或 `ref_key`)；`and`/`or` 数组非空
- condition 节点不能同时有 `key` 和 `and`/`or`（叶子条件与组合条件互斥）
- `op` 必须是游戏服务端支持的操作符之一：`==`、`!=`、`>`、`>=`、`<`、`<=`、`in`（无效操作符在游戏服务端运行时静默返回 false，难以排查）

#### BT 行为树（bt_trees）
- `type` 必须是合法节点类型：
  - 复合节点：`sequence` / `selector` / `parallel`
  - 装饰节点：`inverter`
  - 叶子节点：`check_bb_float` / `check_bb_string` / `set_bb_value` / `stub_action`
- 复合节点（`sequence` / `selector` / `parallel`）必须有 `children` 数组且非空
- 装饰节点（`inverter`）必须有 `child` 字段（单个子节点对象，不是数组）——与游戏服务端 `TreeConfig.Child *TreeConfig` 对齐
- `parallel` 节点的 `params.policy` 如存在，必须是 `require_all` 或 `require_one`（无效值游戏服务端静默降级为 require_all，难以排查）
- 叶子节点必须有 `params` 对象
- `set_bb_value` / `check_bb_float` / `check_bb_string` 的 `params.key` 必须在 Blackboard Key 白名单内（来源：游戏服务端 `blackboard/keys.go`），未注册的 key 会导致服务端 panic
- `stub_action` 的 `params.result` 必须是 `success` / `failure` / `running`（无效值服务端默认 success，行为不符预期）
- 递归校验所有子节点

### 前端架构

```
frontend/src/
├── api/                    # Axios 实例 + 4 组 API 调用
│   ├── index.js            # Axios 实例配置（baseURL、拦截器）
│   ├── eventType.js        # event-types CRUD
│   ├── npcType.js          # npc-types CRUD
│   ├── fsmConfig.js        # fsm-configs CRUD
│   └── btTree.js           # bt-trees CRUD
├── router/
│   └── index.js            # Vue Router 路由表
├── views/
│   ├── EventTypeList.vue   # 事件类型列表页
│   ├── EventTypeForm.vue   # 事件类型编辑表单
│   ├── NpcTypeList.vue     # NPC 类型列表页
│   ├── NpcTypeForm.vue     # NPC 类型编辑表单
│   ├── FsmConfigList.vue   # FSM 列表页
│   ├── FsmConfigForm.vue   # FSM 编辑表单
│   ├── BtTreeList.vue      # BT 列表页
│   └── BtTreeForm.vue      # BT 编辑表单
├── components/
│   ├── AppLayout.vue       # 整体布局（侧边栏 + 主区域）
│   ├── ConditionEditor.vue # FSM 条件构造器
│   └── BtNodeEditor.vue    # BT 节点递归编辑器
├── App.vue
└── main.js
```

#### 页面设计原则

- **列表页**：el-table 展示 name + 关键字段摘要，顶部"新建"按钮，每行"编辑"/"删除"操作
- **表单页**：用独立路由页面（非 dialog），el-form 表单。简单字段用 el-input / el-slider / el-select / el-radio-group，复杂字段用专用组件（ConditionEditor / BtNodeEditor）
- **友好化**：BB Key 名称（如 `threat_level`）在 UI 中显示为中文标签（如"威胁等级"），通过前端 mapping 实现
- **防重复提交**：提交按钮在请求期间 loading + disabled
- **删除确认**：el-popconfirm 二次确认

#### Axios 拦截器

```js
// 响应拦截器
service.interceptors.response.use(
  response => response.data,
  error => {
    const msg = error.response?.data?.error || '网络异常，请稍后重试'
    ElMessage.error(msg)
    return Promise.reject(error)  // 必须 reject，不能吞掉
  }
)
```

### Docker Compose 编排

```yaml
services:
  admin-backend:
    build:
      context: .
      dockerfile: Dockerfile.backend
    ports: ["9821:9821"]
    environment:
      - MONGO_URI=mongodb://mongo:27017
      - REDIS_ADDR=redis:6379
    depends_on: [mongo, redis]

  admin-frontend:
    build:
      context: .
      dockerfile: Dockerfile.frontend
    ports: ["3000:80"]
    depends_on: [admin-backend]

  mongo:
    image: mongo:7
    ports: ["27017:27017"]
    volumes: ["mongo_data:/data/db"]

  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]

volumes:
  mongo_data:
```

- **Dockerfile.backend**：多阶段构建（`golang:1.23-alpine` build → `alpine` run）
- **Dockerfile.frontend**：多阶段构建（`node:20-alpine` build → `nginx:alpine` serve），nginx.conf 配置 `/api/` 反代到 `admin-backend:9821`

---

## 方案对比

### 方案 A（选定）：标准分层 REST + MongoDB RawMessage 透传

如上所述。后端用 `json.RawMessage` / `bson.Raw` 透传 config，validator 按需解析校验。

**优点**：
- config 字段不绑定具体 Go struct，游戏服务端新增字段时运营平台无需改 model
- 四种 config 复用同一个 store 层（Document 结构统一）
- 校验逻辑与存储解耦

**缺点**：
- 需要处理 `json.RawMessage` ↔ `bson.Raw` 转换
- 校验时需要额外的反序列化步骤

### 方案 B（备选）：每种 config 定义完整 Go struct

为每种 config 定义完整的 Go struct（EventTypeConfig、NpcTypeConfig、FsmConfig、BtTreeConfig），存储时完整序列化/反序列化。

**不选原因**：
- 游戏服务端改 config 字段时，运营平台必须同步改 Go struct，耦合度高
- 4 种 config 结构差异大（BT 是递归树、FSM 有条件嵌套），统一处理复杂
- 运营平台本质是「配置编辑器」，不需要在 Go 层面理解全部字段

### 方案 C（备选）：前端直连 MongoDB

去掉 Go 后端，前端通过 MongoDB REST API 或 Realm 直接操作。

**不选原因**：
- 红线：前端不直接操作 MongoDB
- 无法做服务端校验（红线：不信任前端校验）
- 无法做引用完整性检查
- 安全风险极高

---

## 红线检查

| 红线 | 状态 | 说明 |
|------|------|------|
| 禁止暴露 BB Key 原始名称 | ✅ | 前端用 mapping 显示中文标签 |
| 禁止策划手写 JSON | ✅ | 全部通过表单输入 |
| 禁止暴露错误堆栈 | ✅ | handler 统一返回中文 error，原始 err 写 slog |
| 禁止修改 collection 结构 | ✅ | 严格遵循 `{name, config}` 格式 |
| 禁止 config 中加额外字段 | ✅ | RawMessage 透传，不注入字段 |
| 禁止删除被引用的条目 | ✅ | validator 做引用检查后再删 |
| 禁止拼接用户输入为查询 | ✅ | name 精确匹配 `bson.M{"name": name}` |
| 禁止信任前端校验 | ✅ | 后端 validator 全部重做 |
| 禁止暴露 ObjectId | ✅ | API 响应中不含 `_id`，用 name 作为标识 |
| 禁止缓存无 TTL | ✅ | 所有 key 5 分钟 TTL |
| 禁止写操作后不清缓存 | ✅ | Cache-Aside，写成功立即 Del |
| 禁止 MongoDB `$` 操作符注入 | ✅ | name 精确匹配，拒绝 `$` 开头 key |
| 禁止无超时外部调用 | ✅ | 所有 MongoDB/Redis 操作带 context.WithTimeout |
| 禁止实现认证/权限 | ✅ | 不做 |
| 禁止实现版本控制/回滚 | ✅ | 不做 |
| 禁止实现实时协作 | ✅ | 不做 |
| 禁止实现审批工作流 | ✅ | 保存即生效 |

---

## 扩展性影响

| 扩展方向 | 影响 | 说明 |
|---------|------|------|
| 新增配置类型 | **正面** | store 层 Document 结构通用，新增 collection 只需加一组 handler/service/validator 文件 + 前端页面 |
| 新增表单字段 | **正面** | config 透传不绑定 struct，新增字段只需改前端表单 + validator 校验 |

---

## 依赖方向

```
cmd/admin/main.go
    ├─→ handler/     → service/  → store/
    │                            → cache/
    │                → validator/ → store/ (引用检查)
    │                            → model/
    └─→ model/

外部依赖（单向）：
  store/     → go.mongodb.org/mongo-driver
  cache/     → github.com/redis/go-redis
  handler/   → net/http (标准库)
```

handler → service → store/cache/validator → model，严格单向向下，无循环依赖。

---

## Go 陷阱检查

| 陷阱 | 本方案应对 |
|------|-----------|
| nil slice → JSON null | model.Document 的 List 结果用 `make([]Document, 0)` 初始化 |
| json.RawMessage ↔ bson.Raw | 写入用 `bson.UnmarshalExtJSON`，读取用 `bson.MarshalExtJSON` |
| omitempty 吞零值 | config 字段不加 omitempty |
| bson tag 漏写 | Document struct 同时写 json + bson tag |
| handler 写错误后忘 return | writeError 辅助函数写完后调用方必须 return |
| MongoDB context 超时 | store 层统一 5s timeout |
| ErrNoDocuments | store.Get 单独处理为 404 |
| duplicate key 11000 | store.Create 检查 WriteException code，service 转为 409 |

## 服务端对接陷阱检查（联调反馈）

| 陷阱 | 本方案应对 |
|------|-----------|
| inverter 用 child 不是 children | 装饰节点单独分类，校验 child 单对象，前端产出 `{"type":"inverter","child":{...}}` |
| default_severity 服务端是 float64 | 校验结构体用 `*float64`，不用 `*int` |
| FSM op 无效值静默失效 | 校验层白名单拦截：==、!=、>、>=、<、<=、in |
| parallel policy 无效值静默降级 | 校验 policy 仅允许 require_all / require_one |
| Store/Cache 缺 Close() | MongoStore 保存 client 引用暴露 Close(ctx)；RedisCache 暴露 Close() |
| BB key 未注册导致服务端 panic | 后端 validator 白名单校验，前端 BB key 用 el-select 下拉（不允许手动输入） |
| stub_action result 无效值静默降级 | 后端校验 success/failure/running 枚举，前端下拉限定三个选项 |
| FSM 状态名重复 | 校验层检测 stateSet 重复名并报错 |
| condition 叶子+组合混用 | 校验层拒绝同时包含 key 和 and/or 的 condition 节点 |
| itoa 不处理负数 | 替换为 strconv.Itoa |

## 前端陷阱检查

| 陷阱 | 本方案应对 |
|------|-----------|
| ref 忘 .value | script setup 中统一用 ref，模板自动解包 |
| el-form prop 不匹配 | prop 严格与 model 字段名一致 |
| dialog 表单残留 | 用独立路由页面而非 dialog，每次进入自动初始化 |
| 请求竞态 | 提交按钮 loading + disabled |
| 后端返回 null 数组 | 后端保证 `[]`；前端做防御 `(data.items \|\| [])` |
| baseURL 硬编码 | 用 `VITE_API_BASE` 环境变量 |
| scoped 样式穿透 | 需要时用 `:deep()` |

---

## 配置变更

### 新增文件

| 文件 | 说明 |
|------|------|
| `docker-compose.yml` | 4 个服务编排 |
| `Dockerfile.backend` | Go 多阶段构建 |
| `Dockerfile.frontend` | Node 构建 + Nginx 部署 |
| `frontend/.env.development` | `VITE_API_BASE=http://localhost:9821` |
| `frontend/.env.production` | `VITE_API_BASE=` （空，走 nginx 反代） |

### 不改动

- `configs/` 目录 — 只用作参考，运营平台不读取本地 JSON 文件
- 游戏服务端任何文件

---

## 测试策略

### 后端单元测试

| 包 | 测试覆盖 |
|---|---------|
| `store/` | 连真实 MongoDB（docker）；List/Get/Create/Update/Delete 各 collection；duplicate key → error；not found → ErrNoDocuments |
| `cache/` | 连真实 Redis（docker）；Get/Set/Del；TTL 过期；redis.Nil 处理 |
| `validator/` | 每种 config 的合法/非法用例；引用检查（mock store 接口） |
| `service/` | 编排逻辑：创建成功→缓存失效、校验失败→不写 DB、删除被引用→拒绝 |
| `handler/` | HTTP 测试：httptest.NewServer，验证状态码和响应格式 |

### 后端集成测试

- `docker compose up -d mongo redis` → `go test ./...` → 验证完整链路
- `go test -race ./...` — 必须通过

### 前端验证

- `npm run build` — 构建无报错
- 浏览器手动测试 — 四个模块的列表/新建/编辑/删除流程
- 浏览器控制台 — 无 JS 错误、无 Vue warning

### 端到端验证

- `docker compose up --build` → 浏览器操作 → mongosh 验证数据格式
