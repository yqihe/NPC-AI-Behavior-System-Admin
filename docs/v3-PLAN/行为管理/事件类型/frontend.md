# 事件类型管理 — 前端设计

> **实现状态**：已完成。事件类型 CRUD + 扩展字段 Schema 管理均已落地。扩展字段删除流程已接入引用追踪（`schemaReferences`）。
> 通用前端规范见 `docs/development/standards/red-lines/frontend.md` 和 `dev-rules/frontend.md`。
> 扩展字段（Schema）管理的前端实现详见姐妹文档：`docs/v3-PLAN/系统设置/Schema管理/frontend.md`。本文聚焦 **事件类型本体** 的 List + Form 两个主页面。

---

## 1. 目录结构

```
frontend/src/
├── api/
│   └── eventTypes.ts                    # 类型定义 + EVENT_TYPE_ERR + EXT_SCHEMA_ERR + API 函数
│                                         # 含 SchemaReferenceItem / SchemaReferenceDetail
├── views/
│   ├── EventTypeList.vue                # 列表页：筛选(display_name / perception_mode / enabled) + 分页 + toggle + 编辑删除守卫
│   ├── EventTypeForm.vue                # 新建/编辑/查看共用：基本信息 + 扩展字段(按 sort_order) + dirty 追踪
│   ├── EventTypeSchemaList.vue          # 扩展字段 Schema 列表页：筛选 + toggle + 守卫 + 引用详情弹窗
│   └── EventTypeSchemaForm.vue          # 扩展字段 Schema 新建/编辑/查看表单
├── components/
│   ├── EnabledGuardDialog.vue           # 启用守卫（复用，entityType 含 'event-type' | 'event-type-schema'）
│   ├── FieldConstraintInteger.vue       # 约束组件（复用，typeName 支持 'int'/'float'/'integer'）
│   ├── FieldConstraintString.vue        # 约束组件（复用）
│   └── FieldConstraintSelect.vue        # 约束组件（复用）
└── router/index.ts                      # 8 条路由（事件类型 4 + 扩展字段 4）
```

不使用 Pinia / Vuex，所有状态在组件内 `ref` / `reactive` 管理。`fields.ts` 是跨模块共享类型（`ListData<T>`、`CheckNameResult`）的单一权威来源，`eventTypes.ts` 从中 re-export 以保持契约一致。

---

## 2. 页面路由

| 路径 | 组件 | route meta | 说明 |
|---|---|---|---|
| `/event-types` | EventTypeList | — | 列表页 |
| `/event-types/create` | EventTypeForm | `isCreate: true` | 新建页 |
| `/event-types/:id/view` | EventTypeForm | `isCreate: false, isView: true` | 查看页（只读） |
| `/event-types/:id/edit` | EventTypeForm | `isCreate: false` | 编辑页 |
| `/event-type-schemas` | EventTypeSchemaList | — | 扩展字段列表页 |
| `/event-type-schemas/create` | EventTypeSchemaForm | `isCreate: true` | 扩展字段新建页 |
| `/event-type-schemas/:id/view` | EventTypeSchemaForm | `isCreate: false, isView: true` | 扩展字段查看页 |
| `/event-type-schemas/:id/edit` | EventTypeSchemaForm | `isCreate: false` | 扩展字段编辑页 |

侧边栏菜单：「配置管理」分组下「事件类型」+「事件扩展字段」。

---

## 3. 组件树

```
EventTypeList.vue
  └─ EnabledGuardDialog (entityType: 'event-type')
       传入 entity: { id, name, label }   ← 无 ref_count，Guard 只判断「已禁用」条件

EventTypeForm.vue (三模式：create / edit / view)
  ├─ 基本信息卡片（蓝色 title-bar）
  │   name / display_name / perception_mode / range / default_severity / default_ttl
  │   - perception_mode === 'global' 时 range 自动置 0 并 disabled
  │   - name 创建态 blur 校验重名 + 格式；编辑/查看态锁定
  └─ 扩展字段卡片（橙色 title-bar，按 sort_order ASC 排序）
      ├─ 启用字段：按 field_type 动态渲染（int/float/string/bool/select）+ 约束占位
      └─ 禁用但 config 中有值的字段：灰显 + 「已禁用」el-tag + 所有控件 disabled
         （"对旧保留"模式 — 后端 detail 会一并返回，前端只读展示，提交时原值透传不进 dirty）

EventTypeSchemaList.vue
  ├─ EnabledGuardDialog (entityType: 'event-type-schema')
  │   传入 entity: { id, name: field_name, label: field_label }   ← 无 ref_count
  └─ 引用详情 el-dialog (refDialog.visible)
      └─ 一个 section：事件类型引用（SchemaReferenceItem[]）

EventTypeSchemaForm.vue (三模式：create / edit / view)
  ├─ field_name (创建可编辑 + blur 校验 / 编辑查看锁定 + Lock 图标)
  ├─ field_label
  ├─ field_type (int/float/string/bool/select；编辑态锁定)
  ├─ 约束配置 (FieldConstraintInteger / String / Select，支持 disabled prop)
  ├─ default_value (按 field_type 动态渲染，提交前本地校验约束范围)
  └─ sort_order (el-input-number)
```

---

## 4. 类型契约

### 事件类型本体

```ts
// --- api/eventTypes.ts ---
import type { ListData, CheckNameResult } from './fields'

/** 列表查询参数 */
export interface EventTypeListQuery {
  label?: string
  perception_mode?: string
  enabled?: boolean | null
  page: number
  page_size: number
}

/** 列表项 */
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

/** 扩展字段 Schema（详情接口嵌入） */
export interface ExtensionSchemaItem {
  field_name: string
  field_label: string
  field_type: string
  constraints: Record<string, unknown>
  default_value: unknown
  sort_order: number
  enabled: boolean   // false 时前端灰显"已禁用"
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
  config: Record<string, unknown>          // 系统字段 + 扩展字段值（extensions 扁平化）
  extension_schema: ExtensionSchemaItem[]  // 启用 ∪（禁用但 config 有值），按 sort_order ASC
}

/** 创建请求 */
export interface CreateEventTypeRequest {
  name: string
  display_name: string
  perception_mode: string
  default_severity: number
  default_ttl: number
  range: number
  extensions?: Record<string, unknown>  // 仅提交 dirty=true 的字段
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
```

### 扩展字段 Schema + 引用追踪

```ts
/** 扩展字段 Schema 完整信息（schema list 接口返回） */
export interface EventTypeSchemaFull {
  id: number
  field_name: string
  field_label: string
  field_type: string
  constraints: Record<string, unknown>
  default_value: unknown
  sort_order: number
  enabled: boolean
  has_refs: boolean     // ← 后端聚合标志：是否被任何 event_type 引用
                        //   列表页用作展示辅助（未来可在表头加徽标）
                        //   删除决策仍以实时 schemaReferences 结果为准，不依赖此快照
  version: number
  created_at: string
  updated_at: string
}

/** 引用详情（单条引用方） */
export interface SchemaReferenceItem {
  ref_type: string   // 当前固定为 'event_type'
  ref_id: number
  label: string      // 事件类型的 display_name
}

/** 引用详情聚合 */
export interface SchemaReferenceDetail {
  schema_id: number
  field_label: string
  event_types: SchemaReferenceItem[]
}

/** 列表/创建/编辑请求与以往一致（无变化） */
export interface ExtSchemaListQuery { enabled?: boolean }
export interface CreateExtSchemaRequest { field_name; field_label; field_type; constraints; default_value; sort_order }
export interface UpdateExtSchemaRequest { id; field_label; constraints; default_value; sort_order; version }
```

### 错误码

```ts
// 事件类型错误码（42001-42015）
export const EVENT_TYPE_ERR = {
  NAME_EXISTS: 42001, NAME_INVALID: 42002, MODE_INVALID: 42003,
  SEVERITY_INVALID: 42004, TTL_INVALID: 42005, RANGE_INVALID: 42006,
  EXT_VALUE_INVALID: 42007, REF_DELETE: 42008,
  VERSION_CONFLICT: 42010, NOT_FOUND: 42011,
  DELETE_NOT_DISABLED: 42012, EDIT_NOT_DISABLED: 42015,
} as const

// 扩展字段 Schema 错误码（42020-42031）
export const EXT_SCHEMA_ERR = {
  NAME_EXISTS: 42020, NAME_INVALID: 42021, NOT_FOUND: 42022,
  DISABLED: 42023, TYPE_INVALID: 42024, CONSTRAINTS_INVALID: 42025,
  DEFAULT_INVALID: 42026, DELETE_NOT_DISABLED: 42027,
  REF_TIGHTEN: 42028,           // ← 新增：有引用时不允许收紧约束
  REF_DELETE: 42029,            // ← 新增：有引用时不允许删除（后端兜底）
  VERSION_CONFLICT: 42030, EDIT_NOT_DISABLED: 42031,
} as const
```

**扩展字段 dirty 追踪**：

- `dirty=false`：运营未主动修改，使用 schema 默认值，提交时不进 `extensions` payload
- `dirty=true`：运营主动设置过值，提交时写入 `extensions` 对象
- 编辑模式加载时，config 中已有值的字段自动标记 `dirty=true`
- **禁用但有值的字段** 不参与 dirty 追踪，前端以只读形式展示，提交时由后端保留原值

---

## 5. API 调用映射

### 事件类型

| UI 操作 | API 函数 | 后端端点 |
|---|---|---|
| 列表 / 筛选 / 翻页 | `eventTypeApi.list(params)` | `POST /api/v1/event-types/list` |
| 新建 | `eventTypeApi.create(data)` | `POST /api/v1/event-types/create` |
| 详情 | `eventTypeApi.detail(id)` | `POST /api/v1/event-types/detail` |
| 编辑 | `eventTypeApi.update(data)` | `POST /api/v1/event-types/update` |
| 删除 | `eventTypeApi.delete(id)` | `POST /api/v1/event-types/delete` |
| 标识符校验 | `eventTypeApi.checkName(name)` | `POST /api/v1/event-types/check-name` |
| 启用/禁用 | `eventTypeApi.toggleEnabled(id, enabled, version)` | `POST /api/v1/event-types/toggle-enabled` |

`delete` 返回 `ApiResponse<{ id: number; name: string; label: string }>`（与 Field/Template 对齐）。

### 扩展字段 Schema（在本模块 API 中聚合）

| UI 操作 | API 函数 | 后端端点 |
|---|---|---|
| Schema 列表 | `eventTypeApi.schemaList(params?)` | `POST /api/v1/event-type-schema/list` |
| Schema 列表（仅启用） | `eventTypeApi.schemaListEnabled()` | `POST /api/v1/event-type-schema/list` (enabled=true) |
| Schema 新建 | `eventTypeApi.schemaCreate(data)` | `POST /api/v1/event-type-schema/create` |
| Schema 编辑 | `eventTypeApi.schemaUpdate(data)` | `POST /api/v1/event-type-schema/update` |
| Schema 删除 | `eventTypeApi.schemaDelete(id)` | `POST /api/v1/event-type-schema/delete` |
| Schema 启用/禁用 | `eventTypeApi.schemaToggleEnabled(id, enabled, version)` | `POST /api/v1/event-type-schema/toggle-enabled` |
| Schema 引用详情 | `eventTypeApi.schemaReferences(id)` | `POST /api/v1/event-type-schema/references` |

**注意**：Schema 无 detail 接口。编辑/查看页、EnabledGuardDialog 的禁用调用，均通过 `schemaList()` 全量获取后按 ID 查找以获取最新 `version`。

---

## 6. 错误码处理

### 事件类型

| 错误码 | 常量名 | UI 反馈 |
|---|---|---|
| 42001 | NAME_EXISTS | `nameStatus='taken'` + form 内联红字 |
| 42002 | NAME_INVALID | `nameStatus='taken'` + form 内联红字 |
| 42003 | MODE_INVALID | `ElMessage.error` 兜底（前端 select 已限制） |
| 42004 | SEVERITY_INVALID | `ElMessage.error` 兜底 |
| 42005 | TTL_INVALID | `ElMessage.error` 兜底 |
| 42006 | RANGE_INVALID | `ElMessage.error` 兜底 |
| 42007 | EXT_VALUE_INVALID | `ElMessage.error` 提示扩展字段值不合法 |
| 42008 | REF_DELETE | `ElMessage.error` 提示事件类型被引用无法删除（后端兜底，列表页当前未主动查引用） |
| 42010 | VERSION_CONFLICT | `ElMessageBox.alert` 提示刷新，随后 `fetchList()` |
| 42011 | NOT_FOUND | `ElMessage.error` + 跳转列表 |
| 42012 | DELETE_NOT_DISABLED | `EnabledGuardDialog` 前端拦截 |
| 42015 | EDIT_NOT_DISABLED | `EnabledGuardDialog` 前端拦截；提交兜底 `ElMessage.warning` |

### 扩展字段 Schema 相关错误码

见 `docs/v3-PLAN/系统设置/Schema管理/frontend.md` 第 6 节。要点：
- 42028（REF_TIGHTEN）：编辑提交时若后端判定收紧了约束且存在引用，前端 `ElMessage.error`
- 42029（REF_DELETE）：删除时后端兜底（前端已先查 `schemaReferences`，正常情况下不应触发）；若触发，重新拉引用详情展示

---

## 7. 关键实现细节

### 7.1 扩展字段排序与"对旧保留"展示

- 扩展字段按 `sort_order ASC` 排序显示（通过 `sortedExtensionSchema` computed 派生）
- 后端 detail 接口返回的 `extension_schema` 包含 **启用的 schema ∪ 禁用但 config 中留有旧值的 schema**
- **禁用字段对旧保留**：
  - 行容器 `opacity: 0.55`
  - 标签右侧附加 `<el-tag type="info">已禁用</el-tag>`
  - 所有控件 `disabled`
  - 值来自 `config[field_name]`，原样展示，不可修改
  - **不进入 dirty 追踪**，提交时不写入 `extensions` payload，后端保留 MongoDB 中的旧值
- 这是对"字段下线不等于已有数据作废"的妥协 — 策划先禁用 schema 停止新事件类型使用，历史数据在 NPC/事件侧继续可读

### 7.2 感知模式联动（global 模式 range 锁定）

- `perception_mode === 'global'` 时，`range` 自动置为 0 且 disabled
- 切换回 `visual` / `auditory` 时 `range` 控件解除 disabled
- 表单 disabled 条件显式写成 `isView || form.perception_mode === 'global'`（不能省略 `isView`，否则 View 模式下 input-number 仍可交互）
- 与游戏服务端 CalcThreat 对齐：global 事件忽略距离衰减，`range=0` 是语义占位

### 7.3 EnabledGuardDialog 使用约定

- 传入 `entity` 仅包含 `{ id, name, label }`，**不传 ref_count**（Guard 组件本身也已移除 `ref_count` 字段）
- 删除场景 Guard 展示单一条件：「已禁用」
- 编辑场景 Guard 展示两步操作步骤（禁用 → 编辑 → 再启用）
- Guard 内的"立即禁用"按钮：对 event-type 调 `detail()` 预取 version；对 event-type-schema 调 `schemaList()` 查找 version

### 7.4 约束组件复用

`FieldConstraintInteger` 的 `typeName` prop 同时支持 `'integer'`（字段管理）和 `'int'`（Schema），标题显示均为「整数类型 — 约束配置」。`FieldConstraintSelect` 接受 `disabled` prop，在查看模式下传入 `true`。

### 7.5 View 模式显式 disabled 条件

EventTypeForm 中各控件的 disabled 条件显式包含 `isView`：

- **range**：`isView || form.perception_mode === 'global'`
- **扩展字段**：`isView || !ext.enabled`

省略 `isView` 前缀会导致 Element Plus 控件在查看模式下仍可交互（edge case）。

### 7.6 CSS / 行为一致性对齐

- 表单底部按钮区域类名统一为 `.form-actions`
- 列表页禁用行统一样式：`:deep(.row-disabled td:not(:nth-last-child(-n+3))) { opacity: 0.5 }`（最后三列为操作列，保持可读）
- `delete` 返回类型 `{ id, name, label }` 与 Field/Template 对齐

### 7.7 版本冲突后刷新

`handleToggle` 的 catch 中检测 `VERSION_CONFLICT` 后调 `fetchList()` 刷新列表数据，避免用户看到陈旧的 version。

### 7.8 错误码常量使用

所有 catch 分支必须用 `EVENT_TYPE_ERR.xxx` / `EXT_SCHEMA_ERR.xxx` 常量，禁止魔法数字。`EventTypeForm` 的 version 使用独立 `ref(0)` 存储（与 FieldForm 一致），不用 `detail.value!.version` 非空断言。

### 7.9 事件类型表单透明接入引用追踪

EventTypeForm **本身不感知** 后端新增的 `schema_refs` 跟踪机制 — 这是 T13-T17 工作在 service/store 层完成的反向索引。前端表单提交时依旧只提交 `extensions` payload，`schema_refs` 由后端自动维护，前端无需任何改动。
