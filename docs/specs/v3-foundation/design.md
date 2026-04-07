# 需求 0：设计方案

## 方案描述

### 总体思路

保留现有架构分层（handler → service → store → model），保留 Store/Cache 接口和 MongoStore/RedisCache 实现（它们已经是通用的），**清除实体特定的 handler/service/validator，替换为注册制的通用模块**。

### 后端架构（改造后）

```
cmd/admin/main.go
  │  注册实体集合 + 对应 schema 集合名
  │
  ├─ handler/
  │   ├─ router.go          保留框架，改为注册制路由
  │   ├─ common.go          保留，无改动
  │   ├─ generic.go         新增：通用 CRUD handler（替代 4 个实体 handler）
  │   ├─ readonly.go        新增：只读 handler（component-schemas / npc-presets）
  │   └─ config_export.go   保留，适配新集合名
  │   ✗ event_type.go       删除
  │   ✗ npc_type.go         删除
  │   ✗ fsm_config.go       删除
  │   ✗ bt_tree.go          删除
  │
  ├─ service/
  │   ├─ generic.go         新增：通用 CRUD service（替代 4 个实体 service）
  │   └─ readonly.go        新增：只读 service（读取 schema/preset 集合）
  │   ✗ event_type.go       删除
  │   ✗ npc_type.go         删除
  │   ✗ fsm_config.go       删除
  │   ✗ bt_tree.go          删除
  │
  ├─ validator/
  │   ├─ common.go          保留 ValidationError 类型
  │   └─ schema.go          新增：JSON Schema 校验器
  │   ✗ event_type.go       删除
  │   ✗ npc_type.go         删除
  │   ✗ fsm_config.go       删除
  │   ✗ bt_tree.go          删除
  │   ✗ bt_tree_test.go     删除
  │
  ├─ store/
  │   ├─ store.go           保留，不改
  │   ├─ mongo.go           改：Collections 列表更新为 V3 集合
  │   └─ store_test.go      保留，适配新集合名
  │
  ├─ cache/                  完全不改
  │
  └─ model/
      └─ document.go        保留，不改（{name, config} 格式不变）
```

### 核心设计：注册制通用 CRUD

V2 的问题是每个实体类型各有一套 handler + service + validator，导致加实体就要加代码。V3 改为：

**1. 实体注册表（main.go）**

```go
// EntityConfig 定义一个可管理的实体类型
type EntityConfig struct {
    APIPrefix      string // "/api/v1/npc-templates"
    Collection     string // "npc_templates"
    SchemaCollection string // "component_schemas"（可选，空=不校验）
    AllowSlash     bool   // name 是否允许 "/"（行为树需要）
}

entities := []EntityConfig{
    {"/api/v1/npc-templates", "npc_templates", "", false},
    {"/api/v1/event-types", "event_types", "", false},
    {"/api/v1/fsm-configs", "fsm_configs", "", false},
    {"/api/v1/bt-trees", "bt_trees", "", true},
    {"/api/v1/regions", "regions", "", false},
}
```

每个实体用同一个 GenericHandler 实例化，通过 EntityConfig 区分行为。新增实体只需在此处加一行。

**2. 通用 Handler（generic.go）**

```go
type GenericHandler struct {
    service    *service.GenericService
    apiPrefix  string
    allowSlash bool
}
```

提供 List / Get / Create / Update / Delete 五个方法，逻辑与旧 handler 完全一致，只是不绑定具体实体。`allowSlash` 控制 name 提取方式（pathName vs pathNameAfterPrefix）。

**3. 通用 Service（generic.go）**

```go
type GenericService struct {
    store      store.Store
    cache      cache.Cache
    collection string
    validator  *validator.SchemaValidator // 可选
}
```

与旧 service 逻辑一致：List 带缓存 → Create/Update 校验+写入+清缓存 → Delete 写入+清缓存。校验通过 SchemaValidator 执行（如果配置了的话）。

**V3 阶段暂不实现引用完整性检查**（旧的 checkFsmRef/checkBtRef），原因：组件化后引用关系由 schema 定义，引用检查逻辑需要在需求 2 中根据组件 schema 重新设计。

**4. Schema 校验器（schema.go）**

```go
type SchemaValidator struct {
    store store.Store
    schemaCollection string
    compiler *jsonschema.Compiler
}

// Validate 根据 component_schemas 集合中的 schema 校验 config
func (v *SchemaValidator) Validate(ctx context.Context, config json.RawMessage) error
```

使用 `github.com/santhosh-tekuri/jsonschema/v6` 库。需求 0 阶段只实现基础框架：能加载 schema、能校验、能返回 ValidationError。复杂的组件组合校验逻辑留给需求 2。

**如果 component_schemas 集合为空（尚未导入 schema），则跳过校验。** 这允许在没有 schema 的情况下先存数据，不阻塞开发流程。

**5. 只读 Handler/Service（readonly.go）**

为 `component-schemas` 和 `npc-presets` 提供只读 API（仅 List + Get），不走校验、不走缓存。这些数据量极小且几乎不变，不需要缓存层。

**6. 配置导出（config_export.go）**

保留现有逻辑，更新集合名映射：

```go
// V3 导出路径
/api/configs/npc_templates    // 替代 npc_types
/api/configs/event_types      // 不变
/api/configs/fsm_configs      // 不变
/api/configs/bt_trees         // 不变
/api/configs/regions          // 新增
```

### 前端架构（改造后）

```
src/
├─ main.js              不改
├─ App.vue              不改
├─ router/index.js      重写：新路由结构
├─ api/
│   ├─ index.js         保留 Axios 实例
│   ├─ generic.js       新增：通用 CRUD 工厂函数
│   ├─ schema.js        新增：component-schemas / npc-presets 只读 API
│   ✗ eventType.js      删除
│   ✗ npcType.js        删除
│   ✗ fsmConfig.js      删除
│   ✗ btTree.js         删除
│
├─ components/
│   ├─ AppLayout.vue    重写：新侧边栏分组
│   ✗ BtNodeEditor.vue  删除（需求 6 重建）
│   ✗ ConditionEditor.vue 删除（需求 6 重建）
│
├─ views/
│   ├─ Dashboard.vue    重写：适配新数据
│   ├─ PlaceholderList.vue  新增：通用占位列表页
│   ├─ PlaceholderForm.vue  新增：通用占位表单页
│   ✗ EventTypeList.vue     删除
│   ✗ EventTypeForm.vue     删除
│   ✗ NpcTypeList.vue       删除
│   ✗ NpcTypeForm.vue       删除
│   ✗ FsmConfigList.vue     删除
│   ✗ FsmConfigForm.vue     删除
│   ✗ BtTreeList.vue        删除
│   ✗ BtTreeForm.vue        删除
│
└─ utils/
    └─ nameRules.js     保留，改为引用通用 API
```

**通用 API 工厂（generic.js）**

```js
// 工厂函数：给定资源路径，返回 CRUD 方法集
export function createApi(resource) {
  return {
    list: () => request.get(`/${resource}`),
    get: (name) => request.get(`/${resource}/${encodeURIComponent(name)}`),
    create: (data) => request.post(`/${resource}`, data),
    update: (name, data) => request.put(`/${resource}/${encodeURIComponent(name)}`, data),
    remove: (name) => request.delete(`/${resource}/${encodeURIComponent(name)}`),
  }
}

// 预定义
export const npcTemplateApi = createApi('npc-templates')
export const eventTypeApi = createApi('event-types')
export const fsmConfigApi = createApi('fsm-configs')
export const btTreeApi = createApi('bt-trees')
export const regionApi = createApi('regions')
```

**侧边栏分组（AppLayout.vue）**

```
配置管理
  ├─ NPC 模板     → /npc-templates
  ├─ 事件类型     → /event-types
  ├─ 状态机       → /fsm-configs
  └─ 行为树       → /bt-trees

世界管理
  └─ 区域管理     → /regions

系统设置
  ├─ Schema 管理  → /schemas
  └─ 导出管理     → /exports
```

**占位页面（PlaceholderList.vue / PlaceholderForm.vue）**

所有菜单暂时指向同一个占位列表页（通过路由 meta 传递标题），显示 `el-empty` + "待接入动态表单"。后续需求逐个替换为真实页面。

### 新增依赖

**后端**：
- `github.com/santhosh-tekuri/jsonschema/v6` — Go JSON Schema Draft 7 校验库，star 1.8k+，支持 if/then/else，无外部依赖

**前端**：
- `@lljj/vue3-form-element` — Vue 3 + Element Plus JSON Schema 表单渲染库（需求 1 实际使用，需求 0 只引入验证可用）

### MongoDB 集合变更

| 集合 | 状态 | 用途 |
|------|------|------|
| `npc_templates` | 新增（替代 npc_types） | NPC 模板配置 |
| `event_types` | 保留 | 事件类型配置 |
| `fsm_configs` | 保留 | 状态机配置 |
| `bt_trees` | 保留 | 行为树配置 |
| `regions` | 新增 | 区域配置 |
| `component_schemas` | 新增 | 组件字段 Schema（ADMIN 元数据） |
| `npc_presets` | 新增 | NPC 预设模板定义（ADMIN 元数据） |

所有集合都建 `name` unique index（通过 MongoStore.ensureIndexes）。

`component_schemas` 和 `npc_presets` 的文档格式不强制 `{name, config}`——它们是 ADMIN 私有元数据，游戏服务端不直接读取 MongoDB，而是通过 ADMIN API 获取。

---

## 方案对比

### 方案 A（选定）：注册制通用 CRUD

如上所述。一个 GenericHandler + 一个 GenericService，通过 EntityConfig 注册不同实体。

**优点**：
- 新增实体类型零代码（加一行注册）
- 代码量大幅减少（4 套 handler/service → 1 套）
- 与 schema 驱动天然契合

**缺点**：
- 引用完整性检查（如"删 FSM 前检查 NPC 是否引用"）需要额外机制，不能直接写在通用 service 里
- 需要设计扩展点，让后续需求能插入实体特定逻辑

### 方案 B（不选）：保留实体特定 handler/service，只替换 validator

保留 4 个 handler + 4 个 service，只把 validator 换成 schema 驱动。

**优点**：
- 改动最小，风险低
- 实体特定逻辑（引用检查）保留原位

**缺点**：
- 新增实体仍需写新 handler + service，不满足"新增配置类型只需加一行"的扩展性要求
- 代码重复度高（4 个 handler 逻辑几乎一样）
- 不符合需求 0 的核心目标

**不选原因**：无法满足扩展轴"新增配置类型只需加一组 handler/service/store/validator"的要求。方案 A 更进一步——连 handler/service 都不用加。

---

## 红线检查

### `docs/standards/red-lines.md`

| 红线 | 合规 | 说明 |
|------|------|------|
| 禁止静默降级 | ✅ | 校验错误返回 422，不 fallback |
| 禁止无效配置 silent return | ✅ | schema 集合为空时跳过校验但记 Debug 日志 |
| 禁止安全隐患 | ✅ | name 参数 URL 解码，查询用精确匹配 |
| 禁止无超时 IO | ✅ | 保留 5s timeout |
| 禁止过度设计 | ✅ | 只读 handler 不加缓存（数据量极小） |
| 禁止协作失序 | ✅ | 已与服务端 CC 对齐 |

### `docs/standards/go-red-lines.md`

| 红线 | 合规 | 说明 |
|------|------|------|
| 禁止资源泄漏 | ✅ | Close 逻辑不变 |
| nil slice → null | ✅ | 保留 NewListResponse 的 make 初始化 |
| 禁止 writeError 后不 return | ✅ | 通用 handler 沿用现有模式 |
| 禁止 500 暴露 Go error | ✅ | 沿用 handleServiceError |

### `docs/standards/frontend-red-lines.md`

| 红线 | 合规 | 说明 |
|------|------|------|
| 禁止数据源污染 | ✅ | 占位页无数据操作 |
| 禁止放行无效输入 | ✅ | 需求 0 阶段无表单输入 |
| 禁止 URL 编码遗漏 | ✅ | 通用 API 工厂内置 encodeURIComponent |

### `docs/architecture/red-lines.md`

| 红线 | 合规 | 说明 |
|------|------|------|
| 禁止暴露技术细节 | ✅ | 占位页不暴露任何技术信息 |
| 禁止修改 {name, config} 格式 | ✅ | 游戏数据集合保持 {name, config}；ADMIN 元数据集合（schemas/presets）不受此限 |
| 禁止缓存不一致 | ✅ | Cache-Aside 模式不变 |
| 禁止引用完整性破坏 | ⚠️ | V3 暂时移除旧的引用检查，需求 2 重新设计。**风险可控**：需求 0 完成后所有集合为空，无数据可破坏 |
| 禁止绕过 REST API | ✅ | schema 导入走种子脚本但通过 store 层（等同 API 写入） |
| 禁止 ADMIN 过度设计 | ✅ | 不加权限/版本/审批 |

### 需要更新的红线

| 红线文档 | 更新内容 |
|----------|----------|
| `architecture/red-lines.md` | 新增："ADMIN 元数据集合（component_schemas, npc_presets）不受 {name, config} 格式限制" |
| `architecture/red-lines.md` | 更新 BB Key 条目：从"禁止写入未注册的 BB Key"改为"BB Key 白名单由 component schema 的 blackboard_keys 字段定义，BT 编辑器只允许选择当前 NPC 模板拥有的 keys" |

---

## 扩展性影响

- **新增配置类型**：✅ 正面。只需在 main.go 加一行 EntityConfig 注册
- **新增表单字段**：✅ 正面。字段由 schema 定义，ADMIN 代码无需改动

---

## 依赖方向

```
cmd/admin/main.go
    ↓ imports
handler/generic.go, handler/readonly.go, handler/config_export.go
    ↓ imports
service/generic.go, service/readonly.go
    ↓ imports
validator/schema.go    cache/     store/
    ↓ imports           (不变)     (不变)
model/document.go
```

**单向向下，无循环依赖。** 与 V2 方向一致。

---

## Go 陷阱检查

| 陷阱 | 是否涉及 | 处理 |
|------|---------|------|
| nil slice → null | 是 | 保留 make 初始化 |
| json/bson tag | 是 | 新 struct 同时写两个 tag |
| cursor.Close context | 是 | 保留 context.Background() |
| 500 暴露 error | 是 | 保留 handleServiceError |
| 共享状态 | 否 | 无新 goroutine |

---

## 前端陷阱检查

| 陷阱 | 是否涉及 | 处理 |
|------|---------|------|
| 响应式解构 | 否 | 占位页无复杂状态 |
| el-form prop 匹配 | 否 | 需求 0 无表单 |
| Axios 拦截器吞错 | 是 | 保留现有 interceptor |
| 动态导入路径 | 是 | 路由懒加载用显式路径 |

---

## 配置变更

### 新增环境变量

无。MongoDB / Redis / 端口配置不变。

### Docker Compose

无变更。

### MongoDB 集合

新增 3 个集合（component_schemas, npc_presets, regions），替换 1 个（npc_types → npc_templates）。通过 MongoStore.ensureIndexes 自动创建。

---

## 测试策略

### 需求 0 的测试范围

| 测试类型 | 覆盖内容 |
|----------|----------|
| 后端单元测试 | SchemaValidator 基础校验（有 schema / 无 schema / 校验失败） |
| 后端集成测试 | GenericService CRUD 全流程（依赖 Docker MongoDB） |
| 前端手动验证 | 侧边栏分组、占位页渲染、API 空列表返回 |
| Docker 启动测试 | `docker compose up --build` 无报错 |

### 不测什么

- 不测动态表单渲染（需求 1）
- 不测组件组合校验（需求 2）
- 不测关键字搜索（需求 4）
