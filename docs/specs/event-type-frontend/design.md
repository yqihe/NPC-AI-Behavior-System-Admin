# 事件类型管理 — 前端页面开发 · 设计方案

---

## 方案描述

### 整体策略

与字段管理 / 模板管理同构：一组 API 模块 + List 页 + Form 页 + 路由注册 + 侧栏菜单项。不引入新架构概念，不新建 store。

### 1. API 模块 `frontend/src/api/eventTypes.ts`

#### 类型定义

```ts
/** 列表查询参数 */
export interface EventTypeListQuery {
  label?: string           // display_name 模糊搜索
  perception_mode?: string // "visual" | "auditory" | "global" 精确筛选
  enabled?: boolean | null // null=不筛选
  page: number
  page_size: number
}

/** 列表项（后端 EventTypeListItem 对应） */
export interface EventTypeListItem {
  id: number
  name: string
  display_name: string
  perception_mode: string
  enabled: boolean
  created_at: string
  default_severity: number
  default_ttl: number
  range: number
}

/** 扩展字段 Schema（detail 接口返回） */
export interface ExtensionSchemaItem {
  field_name: string
  field_label: string
  field_type: string          // "int" | "float" | "string" | "bool" | "select"
  constraints: Record<string, unknown>
  default_value: unknown
  sort_order: number
}

/** 详情响应 */
export interface EventTypeDetail {
  id: number
  name: string
  display_name: string
  enabled: boolean
  version: number
  created_at: string
  updated_at: string
  config: Record<string, unknown>   // 系统字段 + 扩展字段合并后的 JSON
  extension_schema: ExtensionSchemaItem[]
}

/** 创建请求 */
export interface CreateEventTypeRequest {
  name: string
  display_name: string
  perception_mode: string
  default_severity: number
  default_ttl: number
  range: number
  extensions?: Record<string, unknown>
}

/** 编辑请求 */
export interface UpdateEventTypeRequest {
  id: number
  display_name: string
  perception_mode: string
  default_severity: number
  default_ttl: number
  range: number
  extensions: Record<string, unknown>
  version: number
}

export interface CheckNameResult {
  available: boolean
  message: string
}
```

#### 错误码

```ts
export const EVENT_TYPE_ERR = {
  NAME_EXISTS:          42001,
  NAME_INVALID:         42002,
  MODE_INVALID:         42003,
  SEVERITY_INVALID:     42004,
  TTL_INVALID:          42005,
  RANGE_INVALID:        42006,
  EXT_VALUE_INVALID:    42007,
  REF_DELETE:           42008,
  VERSION_CONFLICT:     42010,
  NOT_FOUND:            42011,
  DELETE_NOT_DISABLED:  42012,
  EDIT_NOT_DISABLED:    42015,
} as const
```

#### API 函数

```ts
export const eventTypeApi = {
  list:          (params) => request.post('/event-types/list', params),
  create:        (data)   => request.post('/event-types/create', data),
  detail:        (id)     => request.post('/event-types/detail', { id }),
  update:        (data)   => request.post('/event-types/update', data),
  delete:        (id)     => request.post('/event-types/delete', { id }),
  checkName:     (name)   => request.post('/event-types/check-name', { name }),
  toggleEnabled: (id, enabled, version) =>
    request.post('/event-types/toggle-enabled', { id, enabled, version }),
}
```

### 2. 列表页 `EventTypeList.vue`

#### 结构（与 FieldList.vue 同构）

```
.event-type-list (flex column, height 100%)
├── .page-header：标题「事件类型管理」+ 「新建事件类型」按钮
├── .filter-bar：
│   ├── el-input（label 搜索，flex: 1.5）
│   ├── el-select（感知模式，flex: 1）── 硬编码选项 Visual/Auditory/Global
│   ├── el-select（启用状态，flex: 1）── 已启用/已禁用
│   ├── 搜索按钮 + 重置按钮
├── .table-wrap：
│   ├── el-table（v-loading）
│   │   ├── ID (70px)
│   │   ├── 事件标识 (min-width: 140)
│   │   ├── 中文名称 (min-width: 120)
│   │   ├── 感知模式 (100px) ── el-tag 彩色区分
│   │   ├── 严重度 (80px)
│   │   ├── TTL (80px)
│   │   ├── 范围 (80px)
│   │   ├── 启用 (80px) ── el-switch
│   │   ├── 创建时间 (170px)
│   │   └── 操作 (120px, fixed right) ── 编辑/删除
│   └── .pagination-wrap
└── EnabledGuardDialog ref
```

#### 感知模式 Tag 映射

| perception_mode | Tag 文案 | Tag 颜色 |
|---|---|---|
| `visual` | Visual | `type="success"` (绿) |
| `auditory` | Auditory | `type=""` (蓝/默认) |
| `global` | Global | `type="info"` (灰) |

#### 关键交互

- **感知模式筛选选项**：硬编码三个 `el-option`（Visual/Auditory/Global），不走字典 API。理由：后端枚举固定三个值，不存在运行时扩展需求。
- **Toggle**：与 FieldList 相同——弹 confirm → detail 拿 version → toggleEnabled → 刷新。
- **编辑/删除守卫**：`row.enabled` → `guardRef.open({ action, entityType: 'event-type', entity })`。
- **禁用行样式**：`row-disabled` CSS class + `:deep(.row-disabled td:not(:nth-last-child(-n+1))) { opacity: 0.5 }`（操作列不灰）。
- **空数据**：`el-empty` + 「新建事件类型」引导按钮。
- **删除确认**：已停用 → `ElMessageBox.confirm` 显示事件名和标识，确认后调 `delete(id)`。

### 3. 表单页 `EventTypeForm.vue`

#### 结构

```
.event-type-form (flex column, height 100%)
├── .form-header：返回 | 新建/编辑事件类型
├── .form-scroll（flex: 1, overflow-y: auto, padding: 24px 32px, gap: 16px）
│   ├── 基本信息卡片（白底圆角边框）
│   │   ├── 卡片标题「基本信息」（蓝色竖条）
│   │   ├── el-form（label-width: 120px）
│   │   │   ├── 事件标识（name）
│   │   │   │   ├── 新建：el-input + blur 校验 + status hint
│   │   │   │   └── 编辑：el-input disabled + Lock 图标
│   │   │   ├── 中文名称（display_name）
│   │   │   ├── 感知模式（perception_mode）── el-select
│   │   │   ├── 默认严重度（default_severity）── el-input-number, 0-100
│   │   │   ├── 默认 TTL（default_ttl）── el-input-number, >0
│   │   │   └── 感知范围（range）── el-input-number, >=0 + Global 提示
│   ├── 扩展字段卡片（条件渲染：extension_schema 非空时显示）
│   │   ├── 卡片标题「扩展字段」（橙色竖条 + "可选" Tag）
│   │   ├── 提示框：说明扩展字段来源和作用
│   │   └── 扩展字段表单行（按 schema 动态渲染，见下文）
│   └── FormFooter（取消 + 保存）
```

#### 扩展字段动态渲染

不引入独立 SchemaForm 组件（避免过度抽象），直接在 EventTypeForm 中通过 `v-for` + `v-if` 渲染：

```vue
<el-form-item
  v-for="ext in extensionSchema"
  :key="ext.field_name"
  :label="ext.field_label"
>
  <!-- int / float -->
  <el-input-number v-if="ext.field_type === 'int' || ext.field_type === 'float'"
    v-model="extensionValues[ext.field_name]"
    :controls="false"
    :step="ext.field_type === 'float' ? 0.1 : 1"
    :placeholder="`默认: ${ext.default_value}`"
    @change="markDirty(ext.field_name)"
  />
  <!-- string -->
  <el-input v-else-if="ext.field_type === 'string'"
    v-model="extensionValues[ext.field_name]"
    :placeholder="`默认: ${ext.default_value}`"
    @input="markDirty(ext.field_name)"
  />
  <!-- bool -->
  <el-switch v-else-if="ext.field_type === 'bool'"
    v-model="extensionValues[ext.field_name]"
    @change="markDirty(ext.field_name)"
  />
  <!-- select -->
  <el-select v-else-if="ext.field_type === 'select'"
    v-model="extensionValues[ext.field_name]"
    @change="markDirty(ext.field_name)"
  >
    <el-option v-for="opt in getSelectOptions(ext)" ... />
  </el-select>
  <!-- 类型 + 默认值提示 -->
  <div class="ext-hint">类型: {{ ext.field_type }} · 默认值: {{ ext.default_value }}</div>
</el-form-item>
```

#### dirty 跟踪

```ts
const extensionValues = reactive<Record<string, unknown>>({})
const dirtyExtensions = reactive<Set<string>>(new Set())

function markDirty(fieldName: string) {
  dirtyExtensions.add(fieldName)
}
```

提交时只收集 dirty 的扩展字段：

```ts
function buildExtensions(): Record<string, unknown> {
  const result: Record<string, unknown> = {}
  for (const key of dirtyExtensions) {
    result[key] = extensionValues[key]
  }
  return result
}
```

编辑模式加载 detail 时，从 `config` 中提取扩展字段值并标记为 dirty：

```ts
function loadExtensionsFromConfig(
  config: Record<string, unknown>,
  schema: ExtensionSchemaItem[],
) {
  const systemKeys = new Set([
    'display_name', 'default_severity', 'default_ttl',
    'perception_mode', 'range',
  ])
  for (const ext of schema) {
    if (ext.field_name in config && !systemKeys.has(ext.field_name)) {
      extensionValues[ext.field_name] = config[ext.field_name]
      dirtyExtensions.add(ext.field_name)
    } else {
      extensionValues[ext.field_name] = ext.default_value
    }
  }
}
```

#### 感知模式 → 范围联动

```ts
function handleModeChange(mode: string) {
  if (mode === 'global') {
    form.range = 0
  }
}
```
模板中 range 输入框在 `form.perception_mode === 'global'` 时 disabled + 提示「Global 模式范围自动置 0」。

#### 表单校验规则

```ts
const rules = {
  name: [
    { required: true, message: '请输入事件标识', trigger: 'blur' },
    { pattern: /^[a-z][a-z0-9_]*$/, message: '小写字母开头，仅含小写字母、数字、下划线', trigger: 'blur' },
  ],
  display_name: [
    { required: true, message: '请输入中文名称', trigger: 'blur' },
  ],
  perception_mode: [
    { required: true, message: '请选择感知模式', trigger: 'change' },
  ],
  default_severity: [
    { required: true, message: '请输入默认严重度', trigger: 'blur' },
  ],
  default_ttl: [
    { required: true, message: '请输入默认 TTL', trigger: 'blur' },
  ],
  range: [
    { required: true, message: '请输入感知范围', trigger: 'blur' },
  ],
}
```

#### 错误处理

| 错误码 | 处理 |
|--------|------|
| 42001 NAME_EXISTS | `nameStatus = 'taken'`，内联红字 |
| 42002 NAME_INVALID | `nameStatus = 'taken'`，内联红字 |
| 42010 VERSION_CONFLICT | `ElMessageBox.alert` 提示刷新 |
| 42011 NOT_FOUND | `ElMessage.error` + `router.push('/event-types')` |
| 42015 EDIT_NOT_DISABLED | `ElMessage.error`（兜底，正常流程前端已拦截） |
| 其他 | 拦截器已 toast |

### 4. EnabledGuardDialog 扩展

当前 `EntityType = 'field' | 'template'`，扩展为 `'field' | 'template' | 'event-type'`。

需要改动的点：

| 改动项 | 现状 | 新增 |
|--------|------|------|
| `EntityType` 联合类型 | `'field' \| 'template'` | `\| 'event-type'` |
| `entityTypeLabel` | field→字段, template→模板 | event-type→事件类型 |
| `refTargetLabel` | field→模板或字段, template→NPC | event-type→FSM 或 BT |
| `reasonText`（edit） | 字段/模板各一段 | 事件类型一段：「已启用的事件类型对 FSM/BT 可见...」 |
| `onActOnce` toggle 逻辑 | if field / else template | + else if event-type |
| import | fieldApi, templateApi | + eventTypeApi |
| 路由跳转 | `/fields/:id/edit`, `/templates/:id/edit` | `/event-types/:id/edit` |
| 版本冲突码 | FIELD_ERR / TEMPLATE_ERR | EVENT_TYPE_ERR |

`GuardEntity` 接口无需改动——事件类型列表项传 `{ id, name, label: display_name, ref_count: 0 }`，ref_count 本期恒 0。

### 5. 路由注册

```ts
// router/index.ts 新增
{
  path: '/event-types',
  name: 'event-type-list',
  component: () => import('@/views/EventTypeList.vue'),
  meta: { title: '事件类型管理' },
},
{
  path: '/event-types/create',
  name: 'event-type-create',
  component: () => import('@/views/EventTypeForm.vue'),
  meta: { title: '新建事件类型', isCreate: true },
},
{
  path: '/event-types/:id/edit',
  name: 'event-type-edit',
  component: () => import('@/views/EventTypeForm.vue'),
  meta: { title: '编辑事件类型', isCreate: false },
},
```

### 6. 侧栏菜单

`AppLayout.vue` 在 `el-sub-menu index="group-config"` 内新增：

```vue
<el-menu-item index="/event-types">
  <el-icon><Lightning /></el-icon>
  <span>事件类型</span>
</el-menu-item>
```

图标使用 `@element-plus/icons-vue` 中的合适图标（如 `Lightning` 或 `Bell`，取决于可用性）。菜单项排在"模板管理"和"字段管理"之后。

---

## 方案对比

### 方案 A（采用）：内联扩展字段渲染

在 `EventTypeForm.vue` 内用 `v-for` + `v-if` 直接渲染 5 种扩展字段类型。

**优点**：
- 无新组件，代码集中在一个文件
- 扩展字段只有事件类型使用，不存在跨模块复用场景
- 渲染逻辑简单（5 个 v-if 分支，每个几行）

**缺点**：
- 如果未来 FSM/BT 也需要扩展字段，需要提取公共组件

### 方案 B（不采用）：独立 SchemaForm 组件

抽取 `SchemaForm.vue` 组件，接收 `schema[]` + `values` + `dirty` props。

**不选理由**：
- 当前只有事件类型使用扩展字段，FSM/BT 暂无此需求
- 过早抽象违反「禁止为只有一个调用点的逻辑创建抽象层」红线
- 未来真有复用需求时，从 EventTypeForm 提取不比从零开始更难

### 方案 C（不采用）：Pinia Store 管理状态

引入 `stores/eventType.ts` 管理列表/表单状态。

**不选理由**：
- 字段管理和模板管理均未使用 Pinia Store，直接在组件内管理 reactive 状态
- 事件类型不存在跨组件共享状态的需求
- 保持与现有模块一致的模式

---

## 红线检查

### 通用红线 `general.md`

| 红线 | 合规 | 说明 |
|------|------|------|
| 禁止静默降级 | ✅ | API 错误由拦截器 toast，不 fallback |
| 禁止信任前端校验 | ✅ | 前端校验 + 后端校验双重保障 |
| 禁止为单调用点建抽象 | ✅ | 扩展字段内联渲染，不抽 SchemaForm |
| 禁止过度设计 | ✅ | 不引入 Pinia Store |

### 前端红线 `frontend.md`

| 红线 | 合规 | 说明 |
|------|------|------|
| 禁止 reactive 不带显式泛型 | ✅ | `reactive<FormState>(...)` |
| 禁止 @change 回调省略参数类型 | ✅ | `(val: string \| number \| boolean) => ...` |
| 禁止只跑 vite build | ✅ | 验收 R31 要求 vue-tsc 通过 |
| 禁止枚举值用自由文本输入 | ✅ | perception_mode 用 el-select |
| 禁止 URL 编码遗漏 | ✅ | 不拼接 URL 参数（POST body） |

### ADMIN 红线 `admin/red-lines.md`

| 红线 | 合规 | 说明 |
|------|------|------|
| 禁止暴露技术细节给策划 | ✅ | 错误提示中文，Tag 用可读标签 |
| 禁止表单对非技术用户不友好 | ✅ | 空数据有引导、blur 校验名称、确认弹窗显示名称 |
| 禁止 toggle 直接用列表 version | ✅ | 先 detail 拿 version 再 toggle |
| 禁止绕过 EnabledGuardDialog | ✅ | 走统一的 EnabledGuardDialog 组件 |
| 禁止 guard 弹窗私有副本 | ✅ | 扩展现有组件，新增 entityType case |
| 禁止删除后自动触发删除 | ✅ | delete 场景只停用+刷新列表 |
| 禁止侧栏用 el-menu-item-group | ✅ | 继续用 el-sub-menu |
| 禁止 handler 错误码混用 | ✅ | name → NAME_INVALID，其他 → 对应错误码 |

### Go 红线 `go.md` / MySQL `mysql.md` / Redis `redis.md` / Cache `cache.md`

不涉及（纯前端改动）。

---

## 扩展性影响

**正面**。

- **新增配置类型扩展轴**：事件类型前端与字段/模板完全同构，验证了"加一组 api + views + 修改 router/sidebar"即可接入新配置类型的模式。后续 FSM/BT/区域可复制相同模式。
- **新增表单字段扩展轴**：扩展字段通过 `extension_schema` 动态渲染，Schema 管理页（另起 spec）新增/修改 schema 后，事件类型表单**自动**展示新字段，无需修改前端代码。

---

## 依赖方向

```
router/index.ts  ──→  views/EventTypeList.vue  ──→  api/eventTypes.ts
                  ──→  views/EventTypeForm.vue  ──→  api/eventTypes.ts
                                                 ──→  api/request.ts

views/EventTypeList.vue ──→ components/EnabledGuardDialog.vue ──→ api/eventTypes.ts
                                                               ──→ api/fields.ts (已有)
                                                               ──→ api/templates.ts (已有)

components/AppLayout.vue (侧栏新增菜单项，无代码依赖)
```

单向向下，无循环依赖。

---

## 陷阱检查（dev-rules/frontend.md）

| 陷阱 | 应对 |
|------|------|
| reactive 解构丢响应性 | 不解构 form/query，直接 `form.name` 访问 |
| el-form prop 与 :model 不匹配 | prop 全部与 form 字段名一致 |
| el-select v-model 类型 | enabled 用 boolean，perception_mode 用 string |
| 并发竞态（快速双击保存） | submitting ref 禁用按钮 |
| el-input-number 精度 | severity/TTL/range 用 float64，step 不设或 0.1，不会遇到精度问题 |
| 同组件多路由不刷新 | AppLayout 已有 `:key="route.fullPath"`，无需额外处理 |
| 列表接口可能缺 version | toggle 前先调 detail 拿 version |
| @change 隐式 any | 所有回调显式声明参数类型 |

---

## 配置变更

无。不需要新增/修改 JSON 配置文件或环境变量。

---

## 测试策略

### 类型检查
- `npx vue-tsc --noEmit` 验证所有新增文件无类型错误（R31）

### 手动验证（对照验收标准 R1-R31）
- 启动后端 + 前端 dev server
- 列表页：增删改查 + 筛选 + 分页 + toggle + 空数据引导
- 表单页：新建完整流程 + 编辑回填 + 标识符校验 + Global 范围联动 + 扩展字段 dirty 跟踪
- Guard 弹窗：编辑/删除启用中事件类型 → 停用 → 跳转/刷新
- 错误场景：版本冲突、名称重复、不存在的 ID
