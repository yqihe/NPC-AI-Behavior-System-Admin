# 事件类型管理 — 前端设计

> **实现状态**：已完成。事件类型 CRUD + 扩展字段 Schema 管理均已落地。
> 通用前端规范见 `docs/development/standards/red-lines/frontend.md` 和 `dev-rules/frontend.md`。

---

## 1. 目录结构

```
frontend/src/
├── api/
│   └── eventTypes.ts                    # 类型定义 + EVENT_TYPE_ERR + EXT_SCHEMA_ERR + API 函数
├── views/
│   ├── EventTypeList.vue                # 列表页：筛选(display_name / perception_mode / enabled) + 分页 + toggle + 编辑删除守卫
│   ├── EventTypeForm.vue                # 新建/编辑/查看共用：系统字段 + 扩展字段(按 sort_order 排序) + dirty 追踪
│   ├── EventTypeSchemaList.vue          # 扩展字段 Schema 列表页：筛选(enabled) + toggle + 编辑删除守卫
│   └── EventTypeSchemaForm.vue          # 扩展字段 Schema 新建/编辑/查看表单页
├── components/
│   ├── EnabledGuardDialog.vue           # 启用守卫（复用，entityType 含 'event-type' | 'event-type-schema'）
│   ├── FieldConstraintInteger.vue       # 约束组件（复用，typeName 支持 'int'/'float'/'integer'）
│   ├── FieldConstraintString.vue        # 约束组件（复用）
│   └── FieldConstraintSelect.vue        # 约束组件（复用）
└── router/index.ts                      # 8 条路由（事件类型 4 + 扩展字段 4）
```

不使用 Pinia/Vuex 状态管理，所有状态在组件内 `ref` / `reactive` 管理。

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

EventTypeForm.vue (三模式：create / edit / view)
  ├─ 基本信息卡片（蓝色 title-bar）
  │   name / display_name / perception_mode / range / default_severity / default_ttl
  └─ 扩展字段卡片（橙色 title-bar，按 sort_order 排序）
      ├─ 启用字段：可编辑
      └─ 禁用但有值的字段：灰显 + 「已禁用」tag + disabled

EventTypeSchemaList.vue
  └─ EnabledGuardDialog (entityType: 'event-type-schema')

EventTypeSchemaForm.vue (三模式：create / edit / view)
  ├─ field_name (创建可编辑 / 编辑查看锁定)
  ├─ field_label / field_type (编辑时 field_type 锁定)
  ├─ 约束配置 (FieldConstraintInteger / String / Select)
  ├─ default_value (按 field_type 动态渲染)
  └─ sort_order
```

---

## 4. 类型契约

```ts
// --- api/eventTypes.ts ---

// 共享类型从 fields.ts 导入（单一权威定义，不重复声明）
import type { ListData, CheckNameResult } from './fields'

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
  VERSION_CONFLICT: 42030, EDIT_NOT_DISABLED: 42031,
} as const

// 扩展字段 Schema（detail 接口返回）
interface ExtensionSchemaItem {
  field_name: string; field_label: string; field_type: string
  constraints: Record<string, unknown>; default_value: unknown
  sort_order: number; enabled: boolean
}
```

**扩展字段 dirty 追踪**：

- `dirty=false`：字段未被运营主动修改，使用 schema 默认值，提交时不进 payload
- `dirty=true`：运营主动设置过值，提交时写入 `extensions` 对象
- 编辑模式加载时，config 中已有值的字段自动标记 `dirty=true`

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

### 扩展字段 Schema

| UI 操作 | API 函数 | 后端端点 |
|---|---|---|
| Schema 列表 | `eventTypeApi.schemaList(params?)` | `POST /api/v1/event-type-schema/list` |
| Schema 列表（仅启用） | `eventTypeApi.schemaListEnabled()` | `POST /api/v1/event-type-schema/list` (enabled=true) |
| Schema 新建 | `eventTypeApi.schemaCreate(data)` | `POST /api/v1/event-type-schema/create` |
| Schema 编辑 | `eventTypeApi.schemaUpdate(data)` | `POST /api/v1/event-type-schema/update` |
| Schema 删除 | `eventTypeApi.schemaDelete(id)` | `POST /api/v1/event-type-schema/delete` |
| Schema 启用/禁用 | `eventTypeApi.schemaToggleEnabled(id, enabled, version)` | `POST /api/v1/event-type-schema/toggle-enabled` |

**注意**：Schema 无 detail 接口，编辑/查看页通过 `schemaList()` 全量获取后按 ID 查找。无 checkName 接口，标识符重复在提交时通过 42020 错误码处理。

---

## 6. 错误码处理

### 事件类型

| 错误码 | UI 反馈 |
|---|---|
| 42001 标识已存在 | nameStatus='taken' + form 内联红字 |
| 42002 标识格式非法 | nameStatus='taken' + form 内联红字 |
| 42003-42006 参数校验 | `ElMessage.error` toast（前端 select/slider 已限制，兜底） |
| 42007 扩展字段违反约束 | `ElMessage.error` toast |
| 42010 版本冲突 | `ElMessageBox.alert` 提示刷新 |
| 42011 不存在 | `ElMessage.error` + 跳转列表 |
| 42012/42015 须先禁用 | `EnabledGuardDialog` 前端拦截 |

### 扩展字段 Schema

| 错误码 | UI 反馈 |
|---|---|
| 42020 标识已存在 | nameStatus='taken' + form 内联红字 |
| 42021 标识格式非法 | nameStatus='taken' + form 内联红字 |
| 42022 不存在 | `ElMessage.error` + 跳转列表 |
| 42024-42026 参数校验 | 拦截器 toast |
| 42027 删除须先禁用 | `ElMessage.warning` |
| 42030 版本冲突 | `ElMessageBox.alert` 提示刷新 |
| 42031 编辑须先禁用 | `ElMessage.warning` |

---

## 7. 关键实现细节

### 扩展字段排序与禁用展示

- 扩展字段按 `sort_order ASC` 排序显示（通过 `sortedExtensionSchema` computed）
- 后端 detail 接口返回 `extension_schema` 包含启用 + 有值的禁用 schema
- 禁用字段：整行 `opacity: 0.55` + 「已禁用」`el-tag` + 所有控件 `disabled`
- 禁用字段的值只读展示，不可修改，不进 dirty 追踪

### 感知模式联动

- `perception_mode = 'global'` 时，`range` 自动置为 0 且 disabled
- 切换回 `visual` / `auditory` 时恢复可编辑

### 约束组件复用

`FieldConstraintInteger` 的 `typeName` prop 同时支持 `'integer'`（字段管理）和 `'int'`（Schema），标题显示均为「整数类型 — 约束配置」。

### 与 Field/Template 的一致性对齐

以下模式已统一对齐，新模块开发时必须遵循：

- **共享类型**：`ListData<T>` 和 `CheckNameResult` 统一从 `fields.ts` 导入，`eventTypes.ts` 不重复定义
- **delete 返回类型**：`ApiResponse<{ id: number; name: string; label: string }>`（与 Field/Template 一致）
- **版本冲突后刷新**：`handleToggle` 的 catch 中检测 `VERSION_CONFLICT` 后调 `fetchList()` 刷新列表数据
- **错误码命名常量**：所有 catch 分支使用 `EVENT_TYPE_ERR.xxx`，禁止魔法数字
- **EventTypeForm version**：使用独立 `ref(0)` 存储版本号（与 FieldForm 一致），不用 `detail.value!.version` 非空断言
