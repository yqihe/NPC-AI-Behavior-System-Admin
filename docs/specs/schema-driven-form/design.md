# 需求 1：设计方案

## 方案描述

### 总体思路

三个层面的工作：
1. **后端**：种子脚本导入 schema + 新增 2 个只读 API
2. **前端核心**：通用 SchemaForm 组件（JSON Schema → Element Plus 表单）
3. **前端页面**：通用列表页 + 表单页，替换所有占位页

### 后端改动

#### 1. 种子脚本 `cmd/seed/main.go`

独立 CLI 工具，从服务端 repo 的 `configs/schemas/` 读取 JSON 文件，写入 MongoDB。

```bash
# 用法
cd backend && go run ./cmd/seed/ --schemas-dir=../../NPC-AI-Behavior-System-Server/configs/schemas
```

导入映射：
| 源目录 | MongoDB 集合 | name 取值 |
|--------|-------------|-----------|
| `components/*.json` | `component_schemas` | 文件名去 .json（如 `movement`） |
| `presets/*.json` | `npc_presets` | 文件名去 .json（如 `simple`） |
| `node_types/*.json` | `node_type_schemas` | 文件名去 .json（如 `check_bb_float`） |
| `condition_types/*.json` | `condition_type_schemas` | 文件名去 .json（如 `leaf`） |

**幂等策略**：对每个文件执行 `ReplaceOne` with `upsert=true`，name 相同则覆盖。

文档格式：`{name: "movement", config: {整个 JSON 文件内容}}`，保持 `{name, config}` 统一格式。

#### 2. 新增只读 API

在 `main.go` 中注册两个新的 ReadOnlyHandler：

```
GET /api/v1/node-type-schemas        → 8 个 BT 节点类型
GET /api/v1/condition-type-schemas    → 2 个 FSM 条件类型
```

MongoStore.Collections 新增 `node_type_schemas`、`condition_type_schemas`。

#### 3. 区域 schema 处理

`region.json` 不属于"组件"，但也需要存储。存入 `component_schemas` 集合，name 为 `_region`（下划线前缀区分）。前端 Schema 管理页面展示时归类为"区域"。

### 前端改动

#### 1. 核心：SchemaForm 组件

**问题**：`@lljj/vue3-form-element`（VueForm）不支持 `allOf + if/then` 条件字段渲染。AJV 校验能处理，但字段不会根据条件自动显示/隐藏。

**解决方案**：包装 VueForm，在外层处理条件逻辑。

```
SchemaForm.vue
├─ 接收原始 schema（含 allOf/if/then）
├─ 预处理：提取条件规则，生成 flatSchema（去掉 allOf，所有字段都保留）
├─ 监听 formData 变化 → 计算哪些字段应该隐藏
├─ 动态生成 uiSchema（用 ui:hidden 控制显隐）
├─ 渲染 VueForm（传入 flatSchema + uiSchema + formData）
└─ 提交时用后端做完整校验（后端 SchemaValidator 支持完整 Draft 7）
```

**条件字段处理流程**：

```js
// 从 schema.allOf 提取条件规则
const conditionalRules = [
  { triggerField: "move_type", triggerValue: "wander", requiredFields: ["wander_radius"] },
  { triggerField: "move_type", triggerValue: "patrol", requiredFields: ["patrol_waypoints"] },
]

// 监听 formData.move_type 变化 → 动态设置 uiSchema
watch(() => formData.value.move_type, (val) => {
  uiSchema.value = {
    wander_radius: { 'ui:hidden': val !== 'wander' },
    patrol_waypoints: { 'ui:hidden': val !== 'patrol' },
  }
})
```

**SchemaForm.vue 对外接口**：

```vue
<SchemaForm
  v-model="configData"
  :schema="jsonSchema"
  @submit="handleSubmit"
/>
```

Props:
- `modelValue` — 表单数据（v-model）
- `schema` — JSON Schema 对象
- `readonly` — 只读模式（Schema 管理页用）

Events:
- `submit` — 表单提交（数据已通过前端基础校验）

#### 2. 通用列表页 GenericList.vue

替换 PlaceholderList.vue，适用于所有实体。

```
GenericList.vue
├─ 表格列：name + config 摘要（前 3 个字段值）
├─ 操作列：编辑 / 删除
├─ 顶部：新建按钮
├─ 空状态：el-empty + 引导新建
├─ 删除确认：明确对象名
└─ 通过路由 meta 获取：页面标题、API 实例、实体路径
```

config 摘要列：从 config 对象中取前 3 个非嵌套字段，格式如 `fsm_ref: civilian | visual_range: 200`。

#### 3. 通用表单页 GenericForm.vue

```
GenericForm.vue
├─ 路由参数判断：有 name → 编辑模式（加载已有数据），无 name → 新建模式
├─ name 输入框（新建时可编辑，编辑时锁定）
├─ SchemaForm 渲染 config 字段
├─ 保存按钮（loading 防重复点击）
├─ 保存逻辑：组装 {name, config} → 调用 API create/update → 成功后返回列表
└─ 通过路由 meta 获取：页面标题、API 实例、schema 名称
```

**schema 来源**：
- 区域页面 → 使用 `_region` schema 的 `schema` 字段
- 其他实体 → 暂不关联 schema，config 用自由 JSON 编辑（SchemaForm 无 schema 时降级为 JSON 编辑器）
- NPC 模板 → 需求 2 做组件化表单，本需求先做基础 CRUD

#### 4. Schema 管理页 SchemaManager.vue

只读展示所有 schema，按类型分 Tab：

```
SchemaManager.vue
├─ Tab 1: 组件 Schema（10 个）— 展示 display_name + blackboard_keys + schema 预览
├─ Tab 2: NPC 预设（4 个）— 展示 display_name + required/default/optional 组件列表
├─ Tab 3: BT 节点类型（8 个）— 展示 display_name + category + params_schema 预览
├─ Tab 4: FSM 条件类型（2 个）— 展示 display_name + params_schema 预览
└─ 每项可展开查看完整 JSON（el-collapse + 代码高亮）
```

#### 5. 路由更新

```js
// 列表 + 表单路由模式
{ path: '/event-types', component: GenericList, meta: { ... } },
{ path: '/event-types/new', component: GenericForm, meta: { ... } },
{ path: '/event-types/:name(.*)', component: GenericForm, meta: { ... } },
```

每个实体的 meta 包含：`title`、`apiInstance`（从 generic.js 导入）、`schemaName`（可选）。

#### 6. 导出管理页 ExportManager.vue

展示各集合的导出 URL 和数据预览：

```
ExportManager.vue
├─ 每个导出集合一行：集合名 + 导出 URL + 数据条数 + "预览"按钮
└─ 预览弹窗：JSON 格式展示导出数据
```

### MongoDB 集合变更

新增 2 个集合（需求 0 遗漏）：

| 集合 | 用途 |
|------|------|
| `node_type_schemas` | BT 节点类型 schema |
| `condition_type_schemas` | FSM 条件类型 schema |

---

## 方案对比

### 方案 A（选定）：VueForm 包装 + 条件字段外部处理

如上所述。用 `@lljj/vue3-form-element` 渲染基础字段，SchemaForm 包装层处理 `allOf/if/then` 条件逻辑。

**优点**：
- 利用已安装的库，不引入新依赖
- 基础字段（string/number/enum/array/object）开箱即用
- 条件逻辑集中在一个解析函数，不散落到各处

**缺点**：
- 需要写条件字段解析逻辑（约 50 行）
- VueForm 的 AJV 版本是 6.x，只支持 Draft 4/6，if/then 是 Draft 7 特性，前端校验不覆盖条件必填（后端兜底）

### 方案 B（不选）：完全自定义表单渲染器

不用 VueForm，自己写 JSON Schema → Element Plus 的渲染器。

**优点**：
- 完全控制渲染逻辑，原生支持 if/then
- 不依赖第三方库

**缺点**：
- 工作量大（string/number/array/object 各需要处理，约 300-500 行）
- 需要自己处理校验、错误展示、嵌套对象
- 毕设时间不允许

**不选原因**：工作量远超方案 A，且基础字段渲染 VueForm 已经做得很好。

---

## 红线检查

### `docs/standards/red-lines.md`

| 红线 | 合规 | 说明 |
|------|------|------|
| 禁止静默降级 | ✅ | schema 缺失时明确提示"无 schema，自由编辑模式" |
| 禁止安全隐患 | ✅ | 种子脚本读本地文件不涉及外部输入 |
| 禁止过度设计 | ✅ | 条件字段解析只处理 allOf+if/then 一种模式，不做通用解析器 |

### `docs/standards/go-red-lines.md`

| 红线 | 合规 | 说明 |
|------|------|------|
| nil slice → null | ✅ | 种子脚本不涉及 API 响应 |
| 禁止无超时 IO | ✅ | 种子脚本 MongoDB 操作带 timeout |

### `docs/standards/frontend-red-lines.md`

| 红线 | 合规 | 说明 |
|------|------|------|
| 禁止数据源污染 | ✅ | 列表数据用 ref，过滤用 computed |
| 禁止放行无效输入 | ✅ | enum 字段用 el-select（VueForm 自动处理） |
| 禁止 URL 编码遗漏 | ✅ | 沿用 generic.js 的 encodeURIComponent |

### `docs/architecture/red-lines.md`

| 红线 | 合规 | 说明 |
|------|------|------|
| 禁止暴露技术细节 | ✅ | 表单展示 schema 中的中文 title/description |
| 禁止让策划手写 JSON | ⚠️ | 无 schema 的实体暂时降级为 JSON 编辑器——仅在 schema 未导入时出现，正常流程中运营不会遇到 |
| 禁止 {name,config} 格式破坏 | ✅ | schema 文档也用 {name, config} 格式存储 |
| 禁止表单不友好 | ✅ | 空列表用 el-empty，删除确认明确对象名 |

---

## 扩展性影响

- **新增配置类型**：✅ 正面。种子脚本加一个目录映射 + 路由加一组 meta 即可
- **新增表单字段**：✅ 正面。修改 schema JSON 后重新导入，前端自动适配

---

## 依赖方向

```
cmd/seed/main.go → store/ → model/
                  （独立 CLI，不依赖 handler/service）

前端:
GenericList.vue / GenericForm.vue
  → SchemaForm.vue → @lljj/vue3-form-element
  → api/generic.js
  → api/schema.js
SchemaManager.vue → api/schema.js
ExportManager.vue → api/generic.js
```

单向向下，无循环。

---

## Go 陷阱检查

| 陷阱 | 是否涉及 | 处理 |
|------|---------|------|
| json/bson 序列化 | 是（种子脚本） | 读 JSON 文件 → bson.UnmarshalExtJSON 转 BSON → 存 MongoDB |
| context 超时 | 是 | 种子脚本每个 upsert 操作 5s timeout |
| nil slice | 否 | 种子脚本不返回 API 响应 |

---

## 前端陷阱检查

| 陷阱 | 是否涉及 | 处理 |
|------|---------|------|
| 响应式解构 | 是 | SchemaForm 中 formData 用 ref，不解构 |
| el-form prop 匹配 | 是 | VueForm 内部处理，我们不直接用 el-form |
| Axios 竞态 | 是 | 保存按钮加 loading 防重复 |
| 动态导入路径 | 是 | 路由懒加载用显式路径 |
| v-for key | 是 | 列表用 item.name 作为 key |

---

## 配置变更

### Docker Compose
无变更。

### 新增 CLI 入口
`cmd/seed/main.go` — 独立于 `cmd/admin/main.go`，不影响主服务。

### 种子脚本运行方式
```bash
# 本地开发
cd backend && go run ./cmd/seed/ --schemas-dir=../../NPC-AI-Behavior-System-Server/configs/schemas

# Docker 环境（可选：Dockerfile 中 COPY 脚本 + 启动时执行）
```

---

## 测试策略

| 测试类型 | 覆盖内容 |
|----------|----------|
| 种子脚本集成测试 | 导入 → 查询 → 验证文档数量和格式 |
| 后端 API 测试 | curl 验证 node-type-schemas / condition-type-schemas 返回正确 |
| 前端构建 | `npm run build` 通过 |
| 前端手动测试 | 各实体页面 CRUD 全流程 |
| 条件字段测试 | movement 选 wander → wander_radius 出现；选 patrol → patrol_waypoints 出现 |
| Docker 集成 | `docker compose up --build` 启动 + 种子脚本导入 + 前端访问 |
