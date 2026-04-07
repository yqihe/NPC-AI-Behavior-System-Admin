# 需求 0：任务拆解

> **状态：全部完成** — 2026-04-07

## [x] T1: 清除后端旧 validator (R16)

**删除文件：**
- `backend/internal/validator/event_type.go`
- `backend/internal/validator/npc_type.go`
- `backend/internal/validator/fsm_config.go`
- `backend/internal/validator/bt_tree.go`
- `backend/internal/validator/bt_tree_test.go`

**保留文件：**
- `backend/internal/validator/common.go`（ValidationError 类型，后续复用）

**做完了是什么样：** `validator/` 目录下只剩 `common.go`，`go build ./...` 会报编译错误（service 层引用了已删的 validator），这是预期的，T2 修复。

---

## [x] T2: 清除后端旧 handler 和 service (R5, R16)

**删除文件：**
- `backend/internal/handler/event_type.go`
- `backend/internal/handler/npc_type.go`
- `backend/internal/handler/fsm_config.go`
- `backend/internal/handler/bt_tree.go`
- `backend/internal/service/event_type.go`
- `backend/internal/service/npc_type.go`
- `backend/internal/service/fsm_config.go`
- `backend/internal/service/bt_tree.go`

**保留文件：**
- `backend/internal/handler/router.go`（T5 重写）
- `backend/internal/handler/common.go`（不改）
- `backend/internal/handler/config_export.go`（T7 适配）

**做完了是什么样：** handler/ 剩 `router.go`、`common.go`、`config_export.go`；service/ 为空目录。`go build ./...` 仍报错（main.go 引用已删模块），T5 修复。

---

## [x] T3: 更新 MongoStore 集合列表 (R1)

**修改文件：**
- `backend/internal/store/mongo.go` — 更新 `Collections` 变量为 V3 集合列表

**变更内容：**
```go
// V2
var Collections = []string{"event_types", "npc_types", "fsm_configs", "bt_trees"}

// V3
var Collections = []string{
    "npc_templates", "event_types", "fsm_configs", "bt_trees",
    "regions",
    "component_schemas", "npc_presets",
}
```

**做完了是什么样：** 启动时为 7 个集合建 unique index。`store_test.go` 如有硬编码旧集合名则同步更新。

---

## [x] T4: 引入 JSON Schema 校验库 + 实现 SchemaValidator (R6, R8)

**修改文件：**
- `backend/go.mod` — 新增 `github.com/santhosh-tekuri/jsonschema/v6`

**新增文件：**
- `backend/internal/validator/schema.go` — SchemaValidator 实现

**SchemaValidator 职责：**
- 接收 `store.Store` 和 schema 集合名
- `Validate(ctx, config json.RawMessage) error`：从 schema 集合加载 schema → 编译 → 校验 config
- schema 集合为空时跳过校验（记 Debug 日志）
- 校验失败返回 `*ValidationError`

**做完了是什么样：** `go build ./...` 编译通过（但 main.go 还不引用 validator，所以不影响启动）。SchemaValidator 可独立测试。

---

## [x] T5: 实现通用 GenericHandler + GenericService + 重写路由注册 (R1, R7)

**新增文件：**
- `backend/internal/handler/generic.go` — GenericHandler（List/Get/Create/Update/Delete）
- `backend/internal/service/generic.go` — GenericService（CRUD + 缓存 + 可选校验）

**修改文件：**
- `backend/internal/handler/router.go` — 改为注册制，接收 EntityConfig 列表循环注册路由
- `backend/cmd/admin/main.go` — 定义 EntityConfig 列表，实例化 GenericHandler/Service，调用 NewRouter

**EntityConfig 注册表（main.go）：**
```go
entities := []handler.EntityConfig{
    {APIPrefix: "/api/v1/npc-templates", Collection: "npc_templates"},
    {APIPrefix: "/api/v1/event-types", Collection: "event_types"},
    {APIPrefix: "/api/v1/fsm-configs", Collection: "fsm_configs"},
    {APIPrefix: "/api/v1/bt-trees", Collection: "bt_trees", AllowSlash: true},
    {APIPrefix: "/api/v1/regions", Collection: "regions"},
}
```

**做完了是什么样：** `docker compose up --build` 启动成功。`GET /api/v1/npc-templates` 返回 `{"items": []}`。旧路径 `/api/v1/npc-types` 返回 404。

---

## [x] T6: 实现只读 Handler + Service（component-schemas / npc-presets）(R3, R4)

**新增文件：**
- `backend/internal/handler/readonly.go` — ReadOnlyHandler（List + Get）
- `backend/internal/service/readonly.go` — ReadOnlyService（直接读 store，不走缓存）

**修改文件：**
- `backend/internal/handler/router.go` — 注册只读路由
- `backend/cmd/admin/main.go` — 实例化只读 handler

**注册路由：**
```
GET /api/v1/component-schemas      → ReadOnlyHandler.List
GET /api/v1/component-schemas/{name} → ReadOnlyHandler.Get
GET /api/v1/npc-presets             → ReadOnlyHandler.List
GET /api/v1/npc-presets/{name}      → ReadOnlyHandler.Get
```

**做完了是什么样：** `GET /api/v1/component-schemas` 返回 `{"items": []}`。POST/PUT/DELETE 返回 405。

---

## [x] T7: 适配配置导出接口 (R9)

**修改文件：**
- `backend/internal/handler/config_export.go` — 更新导出路径映射

**变更内容：**
```go
// V2
/api/configs/npc_types → collection "npc_types"

// V3
/api/configs/npc_templates → collection "npc_templates"
/api/configs/regions       → collection "regions"（新增）
// event_types, fsm_configs, bt_trees 不变
```

**修改文件：**
- `backend/internal/handler/router.go` — 注册新导出路由

**做完了是什么样：** `GET /api/configs/npc_templates` 返回 `{"items": []}`。旧路径 `/api/configs/npc_types` 返回 404。

---

## [x] T8: 后端测试 (R2)

**修改文件：**
- `backend/internal/store/store_test.go` — 适配新集合名（如有引用旧名）

**新增文件：**
- `backend/internal/validator/schema_test.go` — SchemaValidator 单元测试（有 schema 校验 / 无 schema 跳过 / 校验失败）

**做完了是什么样：** `go test ./...` 全部通过。

---

## [x] T9: 清除前端旧页面和组件 (R13)

**删除文件：**
- `frontend/src/views/EventTypeList.vue`
- `frontend/src/views/EventTypeForm.vue`
- `frontend/src/views/NpcTypeList.vue`
- `frontend/src/views/NpcTypeForm.vue`
- `frontend/src/views/FsmConfigList.vue`
- `frontend/src/views/FsmConfigForm.vue`
- `frontend/src/views/BtTreeList.vue`
- `frontend/src/views/BtTreeForm.vue`
- `frontend/src/components/BtNodeEditor.vue`
- `frontend/src/components/ConditionEditor.vue`

**删除文件：**
- `frontend/src/api/eventType.js`
- `frontend/src/api/npcType.js`
- `frontend/src/api/fsmConfig.js`
- `frontend/src/api/btTree.js`

**保留文件：**
- `frontend/src/api/index.js`（Axios 实例）
- `frontend/src/utils/nameRules.js`
- `frontend/src/views/Dashboard.vue`（T12 重写）
- `frontend/src/components/AppLayout.vue`（T11 重写）

**做完了是什么样：** `src/views/` 只剩 `Dashboard.vue`，`src/components/` 只剩 `AppLayout.vue`，`src/api/` 只剩 `index.js`。`npm run dev` 报错（路由引用已删组件），T11 修复。

---

## [x] T10: 新增前端通用 API 层 + Schema API (R15)

**新增文件：**
- `frontend/src/api/generic.js` — 通用 CRUD 工厂函数 + 预定义实体 API
- `frontend/src/api/schema.js` — component-schemas / npc-presets 只读 API

**generic.js 核心：**
```js
export function createApi(resource) {
  return {
    list: () => request.get(`/${resource}`),
    get: (name) => request.get(`/${resource}/${encodeURIComponent(name)}`),
    create: (data) => request.post(`/${resource}`, data),
    update: (name, data) => request.put(`/${resource}/${encodeURIComponent(name)}`, data),
    remove: (name) => request.delete(`/${resource}/${encodeURIComponent(name)}`),
  }
}
export const npcTemplateApi = createApi('npc-templates')
export const eventTypeApi = createApi('event-types')
export const fsmConfigApi = createApi('fsm-configs')
export const btTreeApi = createApi('bt-trees')
export const regionApi = createApi('regions')
```

**做完了是什么样：** API 层就绪，可在任何组件中 `import { npcTemplateApi } from '@/api/generic'` 使用。

---

## [x] T11: 重写侧边栏 + 路由 + 占位页 (R10, R11, R12)

**修改文件：**
- `frontend/src/components/AppLayout.vue` — 新侧边栏分组
- `frontend/src/router/index.js` — 新路由结构

**新增文件：**
- `frontend/src/views/PlaceholderList.vue` — 通用占位列表页（el-empty + "待接入动态表单"）

**侧边栏菜单结构：**
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

**路由全部指向 PlaceholderList.vue**，通过路由 meta 传递页面标题。

**做完了是什么样：** `npm run dev` 正常启动。侧边栏显示三组菜单。点击任何菜单进入占位页，显示对应标题 + "暂无数据"。

---

## [x] T12: 重写 Dashboard + 引入前端 Schema 渲染库 (R10, R14)

**修改文件：**
- `frontend/src/views/Dashboard.vue` — 适配新实体列表（NPC 模板、事件、FSM、BT、区域），移除旧的创建引导
- `frontend/package.json` — 引入 `@lljj/vue3-form-element`

**做完了是什么样：** Dashboard 显示新的实体统计卡片（全部为 0）。`package.json` 中可见 schema 渲染库依赖。`npm run dev` 正常。

---

## [x] T13: 更新红线文档 + Roadmap 状态 (R17)

**修改文件：**
- `docs/architecture/red-lines.md` — 新增 ADMIN 元数据集合格式说明；更新 BB Key 条目
- `docs/specs/v3-roadmap.md` — 更新需求 0 状态为"已完成"
- `docs/specs/v3-foundation/tasks.md` — 标记所有任务完成

**做完了是什么样：** 红线文档与 V3 架构一致。Roadmap 反映当前进度。

---

## 任务依赖顺序

```
T1（清 validator）
  → T2（清 handler/service）
    → T3（更新集合列表）
      → T4（Schema 校验器）
        → T5（通用 CRUD + 路由 + main.go）
          → T6（只读 API）
            → T7（导出接口适配）
              → T8（后端测试）

T9（清前端旧页面）
  → T10（通用 API 层）
    → T11（侧边栏 + 路由 + 占位页）
      → T12（Dashboard + Schema 库引入）

T8 + T12 → T13（文档更新）
```

**后端（T1-T8）和前端（T9-T12）可并行。** T13 等两边都完成后执行。

---

## 任务 × 验收标准 映射

| 验收标准 | 覆盖任务 |
|----------|----------|
| R1: docker compose up --build 成功 | T5 |
| R2: go test 全部通过 | T8 |
| R3: GET component-schemas 返回 200 | T6 |
| R4: GET npc-presets 返回 200 | T6 |
| R5: 旧 CRUD API 返回 404 | T2, T5 |
| R6: JSON Schema 库在 go.mod 中 | T4 |
| R7: 通用 CRUD 支持任意集合 | T5 |
| R8: Schema 校验器可校验 config | T4 |
| R9: 导出接口适配新结构 | T7 |
| R10: npm run dev 正常 | T11, T12 |
| R11: 侧边栏新分组 | T11 |
| R12: 占位页显示"暂无数据" | T11 |
| R13: 旧页面已删除 | T9 |
| R14: Schema 渲染库在 package.json 中 | T12 |
| R15: 通用 API 层 | T10 |
| R16: 无硬编码字段残留 | T1, T2 |
| R17: 无 TODO 占位 | T13 |
