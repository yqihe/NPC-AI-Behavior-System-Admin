# 需求 1：任务拆解

## T1: 后端新增 node_type_schemas / condition_type_schemas 集合 + 只读 API (R3, R4, R7)

**修改文件：**
- `backend/internal/store/mongo.go` — Collections 新增 `node_type_schemas`、`condition_type_schemas`
- `backend/cmd/admin/main.go` — 注册 2 个新的 ReadOnlyHandler

**做完了是什么样：** `GET /api/v1/node-type-schemas` 返回 `{"items": []}`，`GET /api/v1/condition-type-schemas` 返回 `{"items": []}`。`go build ./...` 和 `go test ./...` 通过。

---

## T2: 种子脚本 — 框架 + 组件 schema 导入 (R1, R5)

**新增文件：**
- `backend/cmd/seed/main.go` — 种子脚本入口

**职责：**
- 读取 `--schemas-dir` 参数指定的目录
- 扫描 `components/*.json` → 逐个读取 → `ReplaceOne` upsert 到 `component_schemas` 集合
- 文件名去 `.json` 作为 `name`，整个 JSON 内容作为 `config`
- 包含 `region.json` → name 为 `_region` 存入 `component_schemas`
- 幂等：多次运行结果一致

**做完了是什么样：** 运行 `go run ./cmd/seed/ --schemas-dir=<path>` 后，`component_schemas` 集合包含 11 个文档（10 组件 + 1 区域）。再次运行不产生重复。

---

## T3: 种子脚本 — 预设 + 节点类型 + 条件类型导入 (R2, R3, R4, R5)

**修改文件：**
- `backend/cmd/seed/main.go` — 新增 presets / node_types / condition_types 目录扫描

**导入映射：**
- `presets/*.json` → `npc_presets`（4 个）
- `node_types/*.json` → `node_type_schemas`（8 个）
- `condition_types/*.json` → `condition_type_schemas`（2 个）

**做完了是什么样：** 运行种子脚本后，4 个集合分别包含正确数量的文档。`GET /api/v1/node-type-schemas` 返回 8 个，`GET /api/v1/condition-type-schemas` 返回 2 个。

---

## T4: 前端 SchemaForm 组件 — 基础渲染 (R13, R18)

**新增文件：**
- `frontend/src/components/SchemaForm.vue` — JSON Schema 表单包装组件

**职责：**
- 接收 `schema`（JSON Schema 对象）和 `v-model`（表单数据）
- 传递给 VueForm 渲染
- 无 schema 时降级显示 JSON 文本编辑器（el-input textarea）
- 展示中文 title / description（来自 schema）

**做完了是什么样：** `<SchemaForm v-model="data" :schema="schema" />` 能渲染 string/number/enum/array/object 字段，中文标题和描述可见。

---

## T5: 前端 SchemaForm 组件 — 条件字段处理 (R14)

**修改文件：**
- `frontend/src/components/SchemaForm.vue` — 新增条件字段解析和动态显隐逻辑

**逻辑：**
- 从 `schema.allOf` 提取 `if/then` 条件规则
- 生成 flatSchema（移除 allOf，所有字段保留为可选）
- 监听触发字段变化 → 动态计算 uiSchema（`ui:hidden`）
- 条件必填的验证交给后端

**做完了是什么样：** 传入 movement schema → 选 `wander` → `wander_radius` 出现；选 `patrol` → `patrol_waypoints` 出现，`wander_radius` 消失。

---

## T6: 前端通用列表页 GenericList.vue (R8-R12, R17)

**新增文件：**
- `frontend/src/views/GenericList.vue` — 通用 CRUD 列表页

**职责：**
- 通过路由 meta 获取：`title`、`api`（CRUD 方法集）、`entityPath`（路由前缀）
- 表格列：name + config 摘要（前 3 个字段值）
- 操作列：编辑（跳转表单页）、删除（popconfirm）
- 顶部：新建按钮
- 空状态：el-empty
- 删除后刷新列表 + 清缓存

**做完了是什么样：** 访问 `/event-types` → 看到列表表格（可能为空）→ 有"新建"按钮 → 删除有确认弹窗。

---

## T7: 前端通用表单页 GenericForm.vue (R8-R12, R19)

**新增文件：**
- `frontend/src/views/GenericForm.vue` — 通用 CRUD 表单页

**职责：**
- 路由参数：有 `:name` → 编辑模式（加载数据），路径含 `/new` → 新建模式
- name 输入框（新建可编辑 + nameRules 校验，编辑锁定）
- SchemaForm 渲染 config（如果路由 meta 提供了 schema）
- 无 schema 时 SchemaForm 降级为 JSON 编辑器
- 保存按钮（loading 防重复）→ 调用 create/update API → 成功返回列表
- 错误提示用中文（R19）

**做完了是什么样：** 点击"新建" → 进入表单页 → 填写 name + config → 保存 → 返回列表看到新数据。编辑模式下 name 锁定，config 回显。

---

## T8: 路由重构 — 所有实体指向真实页面 (R8-R12, R16)

**修改文件：**
- `frontend/src/router/index.js` — 每个实体注册 list + new + edit 三条路由
- `frontend/src/api/schema.js` — 新增 nodeTypeSchemaApi / conditionTypeSchemaApi

**路由结构（每个实体）：**
```
/event-types          → GenericList
/event-types/new      → GenericForm（新建）
/event-types/:name(.*)→ GenericForm（编辑）
```

**路由 meta 包含：** `title`、`api`（从 generic.js 导入）、`entityPath`、`schemaName`（可选）。

**做完了是什么样：** `npm run build` 通过。5 个实体各 3 条路由（共 15 条），PlaceholderList.vue 可删除。

---

## T9: Schema 管理页 SchemaManager.vue (R15)

**新增文件：**
- `frontend/src/views/SchemaManager.vue` — 只读展示所有 schema

**职责：**
- 4 个 Tab：组件 Schema / NPC 预设 / BT 节点类型 / FSM 条件类型
- 每个 Tab 调用对应只读 API 获取数据
- 每项展示：display_name + 关键信息摘要
- 可展开查看完整 JSON（el-collapse）

**修改文件：**
- `frontend/src/router/index.js` — `/schemas` 指向 SchemaManager

**做完了是什么样：** 点击侧边栏"Schema 管理" → 看到 4 个 Tab → 组件 Tab 显示 10 个组件的 display_name + blackboard_keys。

---

## T10: 导出管理页 ExportManager.vue + 清理 (R16)

**新增文件：**
- `frontend/src/views/ExportManager.vue` — 导出接口一览

**职责：**
- 列出所有导出接口 URL（`/api/configs/{collection}`）
- 每行显示：集合名 + URL + 数据条数
- "复制 URL" 按钮

**修改/删除文件：**
- `frontend/src/router/index.js` — `/exports` 指向 ExportManager
- 删除 `frontend/src/views/PlaceholderList.vue`（不再需要）

**做完了是什么样：** 点击"导出管理" → 看到 5 个导出接口。PlaceholderList.vue 已删除。`npm run build` 通过。

---

## T11: 更新文档 + Roadmap (R6, R7)

**修改文件：**
- `docs/specs/v3-roadmap.md` — 需求 1 状态更新为"已完成"
- `docs/specs/schema-driven-form/tasks.md` — 标记所有任务完成

**验证：**
- `go test ./...` 全部通过
- `docker compose up --build` 启动成功
- 种子脚本导入 + 前端 CRUD 全流程端到端通过

**做完了是什么样：** 文档反映当前状态，端到端验证通过。

---

## 任务依赖顺序

```
T1（后端新集合 + API）
  → T2（种子脚本 — 组件 schema）
    → T3（种子脚本 — 预设 + 节点 + 条件）

T4（SchemaForm 基础渲染）
  → T5（SchemaForm 条件字段）
    → T6（GenericList）
      → T7（GenericForm）
        → T8（路由重构）
          → T9（Schema 管理页）
            → T10（导出管理页 + 清理）

T3 + T10 → T11（文档 + 端到端验证）
```

后端（T1-T3）和前端（T4-T5）可并行。T6 开始需要后端 API 可用。

---

## 任务 × 验收标准映射

| 验收标准 | 覆盖任务 |
|----------|----------|
| R1: component_schemas 10 个文档 | T2 |
| R2: npc_presets 4 个文档 | T3 |
| R3: node-type-schemas 返回 8 个 | T1, T3 |
| R4: condition-type-schemas 返回 2 个 | T1, T3 |
| R5: 种子脚本幂等 | T2, T3 |
| R6: go test 通过 | T11 |
| R7: docker compose up 成功 | T11 |
| R8-R12: 5 个实体 CRUD | T6, T7, T8 |
| R13: 动态渲染字段 | T4 |
| R14: 条件字段 | T5 |
| R15: Schema 管理页 | T9 |
| R16: npm run build 通过 | T8, T10 |
| R17: 列表展示关键字段 | T6 |
| R18: 中文标题和描述 | T4 |
| R19: 校验错误中文提示 | T7 |
