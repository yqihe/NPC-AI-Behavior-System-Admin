# 事件扩展字段 Schema 管理 — 前端设计

> **实现状态**：已完成。删除流程已接入反向引用追踪（`schemaReferences`），列表项包含 `has_refs` 聚合标志。
> 本文档专注 Schema 管理自身（List + Form + 引用详情弹窗）。事件类型主页面见 `docs/v3-PLAN/行为管理/事件类型/frontend.md`。
> 通用前端规范见 `docs/development/standards/red-lines/frontend.md` 和 `dev-rules/frontend.md`。

---

## 1. 目录结构

```
frontend/src/
├── api/
│   └── eventTypes.ts                    # 类型定义 + EXT_SCHEMA_ERR + API 函数
│                                         # 含 EventTypeSchemaFull (带 has_refs)
│                                         # 含 SchemaReferenceItem / SchemaReferenceDetail
├── views/
│   ├── EventTypeSchemaList.vue          # 列表页：筛选 + 排序切换 + toggle + 守卫 + 引用详情弹窗
│   └── EventTypeSchemaForm.vue          # 新建/编辑/查看表单页
├── components/
│   ├── EnabledGuardDialog.vue           # 启用守卫（复用，entityType 含 'event-type-schema'）
│   ├── FieldConstraintInteger.vue       # 约束组件（复用，typeName 支持 'int'/'float'）
│   ├── FieldConstraintString.vue        # 约束组件（复用）
│   └── FieldConstraintSelect.vue        # 约束组件（复用，支持 disabled prop）
└── router/index.ts                      # 4 条 Schema 路由
```

Schema 管理的 API 入口复用 `eventTypeApi`（`schemaList` / `schemaCreate` / `schemaUpdate` / `schemaDelete` / `schemaToggleEnabled` / `schemaReferences`），没有独立的 `api/eventTypeSchemas.ts` — Schema 与事件类型在契约上强耦合，共用一个 API 模块更便于维护。

---

## 2. 页面路由

| 路径 | 组件 | route meta | 说明 |
|---|---|---|---|
| `/event-type-schemas` | EventTypeSchemaList.vue | — | 列表页 |
| `/event-type-schemas/create` | EventTypeSchemaForm.vue | `isCreate: true` | 新建页 |
| `/event-type-schemas/:id/view` | EventTypeSchemaForm.vue | `isCreate: false, isView: true` | 查看页（只读） |
| `/event-type-schemas/:id/edit` | EventTypeSchemaForm.vue | `isCreate: false` | 编辑页 |

侧边栏菜单：「配置管理」→「事件扩展字段」。

---

## 3. 组件树

```
EventTypeSchemaList.vue
  ├─ 筛选栏：启用状态 (el-select) + 搜索/重置 + 排序切换 (el-button-group: ID 倒序 | 排序正序)
  ├─ el-table
  │   列：ID / 字段标识 / 中文标签 / 类型 tag / 排序 / 启用 switch / 创建时间 / 操作(查看/编辑/删除)
  │   禁用行：opacity 0.5（最后三列操作保持不透明）
  ├─ EnabledGuardDialog (entityType: 'event-type-schema')
  │   传入 entity: { id, name: field_name, label: field_label }
  │   ← 不传 ref_count，Guard 只判断「已禁用」条件
  └─ 引用详情 el-dialog (refDialog.visible, width 500px)
      标题：`引用详情 — ${fieldLabel}`
      body：一个 section，展示"事件类型引用"
        - 若 event_types.length > 0 → el-table (size small)：label + ref_type
        - 若 = 0 → <p class="ref-empty">暂无事件类型引用</p>

EventTypeSchemaForm.vue (三模式：create / edit / view)
  ├─ field_name (创建可编辑 + blur 正则校验；编辑/查看锁定 + Lock icon + 警告提示)
  ├─ field_label (el-input, maxlength)
  ├─ field_type (create：el-select 五选项 int/float/string/bool/select；edit：锁定)
  ├─ 约束配置
  │   - int/float → FieldConstraintInteger (typeName='int' / 'float')
  │   - string   → FieldConstraintString
  │   - select   → FieldConstraintSelect (disabled prop)
  │   - bool     → 无约束
  │   constraintRef.value?.validate() 提交前调用
  ├─ default_value (按 field_type 动态渲染 el-input / el-input-number / el-switch / el-select)
  │   提交前本地校验：是否落在约束 min/max 范围内（CheckConstraintSelf 自洽）
  └─ sort_order (el-input-number, 默认 0)
```

---

## 4. 类型契约

```ts
// --- api/eventTypes.ts ---

/** Schema 列表查询参数 */
export interface ExtSchemaListQuery {
  enabled?: boolean
}

/** 扩展字段 Schema 完整信息（list 接口返回） */
export interface EventTypeSchemaFull {
  id: number
  field_name: string
  field_label: string
  field_type: string                     // 'int' | 'float' | 'string' | 'bool' | 'select'
  constraints: Record<string, unknown>
  default_value: unknown
  sort_order: number
  enabled: boolean
  has_refs: boolean                      // ← 聚合标志：是否被任何 event_type 引用
                                         //   由后端基于 schema_refs 反向索引计算
                                         //   列表页直接消费，无需再发 schemaReferences
                                         //   删除决策仍以实时 schemaReferences 结果为准（has_refs 可能陈旧）
  version: number
  created_at: string
  updated_at: string
}

/** 创建请求 */
export interface CreateExtSchemaRequest {
  field_name: string
  field_label: string
  field_type: string
  constraints: Record<string, unknown>
  default_value: unknown
  sort_order: number
}

/** 编辑请求（field_name / field_type 不可变，不在 payload 中） */
export interface UpdateExtSchemaRequest {
  id: number
  field_label: string
  constraints: Record<string, unknown>
  default_value: unknown
  sort_order: number
  version: number
}

/** 引用详情中的单条引用方 */
export interface SchemaReferenceItem {
  ref_type: string    // 当前固定为 'event_type'（未来可扩展到其他实体）
  ref_id: number
  label: string       // 事件类型的 display_name
}

/** 引用详情聚合 */
export interface SchemaReferenceDetail {
  schema_id: number
  field_label: string
  event_types: SchemaReferenceItem[]
}
```

### 错误码

```ts
export const EXT_SCHEMA_ERR = {
  NAME_EXISTS:         42020,
  NAME_INVALID:        42021,
  NOT_FOUND:           42022,
  DISABLED:            42023,
  TYPE_INVALID:        42024,
  CONSTRAINTS_INVALID: 42025,
  DEFAULT_INVALID:     42026,
  DELETE_NOT_DISABLED: 42027,
  REF_TIGHTEN:         42028,   // ← 新增：有引用时不允许收紧约束
  REF_DELETE:          42029,   // ← 新增：有引用时不允许删除（后端兜底）
  VERSION_CONFLICT:    42030,
  EDIT_NOT_DISABLED:   42031,
} as const
```

---

## 5. API 调用映射

| UI 操作 | API 函数 | 后端端点 |
|---|---|---|
| 列表 / 筛选 | `eventTypeApi.schemaList(params?)` | `POST /api/v1/event-type-schema/list` |
| 列表（仅启用，事件类型表单用） | `eventTypeApi.schemaListEnabled()` | `POST /api/v1/event-type-schema/list` (enabled=true) |
| 新建 | `eventTypeApi.schemaCreate(data)` | `POST /api/v1/event-type-schema/create` |
| 编辑 | `eventTypeApi.schemaUpdate(data)` | `POST /api/v1/event-type-schema/update` |
| 删除 | `eventTypeApi.schemaDelete(id)` | `POST /api/v1/event-type-schema/delete` |
| 启用/禁用 | `eventTypeApi.schemaToggleEnabled(id, enabled, version)` | `POST /api/v1/event-type-schema/toggle-enabled` |
| **引用详情** | **`eventTypeApi.schemaReferences(id)`** | **`POST /api/v1/event-type-schema/references`** |

**注意**：Schema **无 detail 接口**，编辑/查看页、EnabledGuardDialog 均通过 `schemaList()` 全量获取后按 ID 查找。**无 checkName 接口**，标识符重复靠提交时的 42020 错误码反馈。共 6 个专属 API + 1 个 enabled 过滤便捷封装。

---

## 6. 错误码处理

| 错误码 | 常量名 | UI 反馈 |
|---|---|---|
| 42020 | NAME_EXISTS | `nameStatus='taken'` + form 内联红字 |
| 42021 | NAME_INVALID | `nameStatus='taken'` + form 内联红字 |
| 42022 | NOT_FOUND | `ElMessage.error('扩展字段不存在')` + 跳转列表 |
| 42023 | DISABLED | `ElMessage.error` 提示 Schema 已禁用 |
| 42024 | TYPE_INVALID | `ElMessage.error` 提示字段类型不合法（前端 select 已限制，兜底） |
| 42025 | CONSTRAINTS_INVALID | `ElMessage.error` 提示约束参数不合法 |
| 42026 | DEFAULT_INVALID | `ElMessage.error` 提示默认值不符合约束条件 |
| 42027 | DELETE_NOT_DISABLED | `ElMessage.warning('请先禁用该扩展字段后再删除')` |
| **42028** | **REF_TIGHTEN** | **`ElMessage.error` 提示「该扩展字段被事件类型引用，不允许收紧约束范围」** |
| **42029** | **REF_DELETE** | **后端兜底。触发后调 `loadAndShowRefs(row)` 重新拉引用详情展示**（正常情况不会触发，因前端删除前已主动调 `schemaReferences`） |
| 42030 | VERSION_CONFLICT | `ElMessageBox.alert('数据已被其他用户修改，请刷新页面后重试')` + `fetchList()` |
| 42031 | EDIT_NOT_DISABLED | `EnabledGuardDialog` 前端拦截；提交兜底 `ElMessage.warning` |
| 其他 | — | 请求拦截器统一 toast |

---

## 7. 关键实现细节

### 7.1 列表页排序与数据量假设

- 数据量 < 100，**不做分页**（后端 list 无分页参数）
- 默认按 `ID DESC` 排列（新增在顶）
- 筛选栏右侧 `el-button-group` 提供两种排序模式切换：
  - `ID 倒序`：`b.id - a.id`
  - `排序正序`：`a.sort_order - b.sort_order || a.id - b.id`（sort_order 相同则 ID 升序次排）
- 切换排序 **不重新请求后端**，只本地 `applySorting(tableData.value)`

### 7.2 `has_refs` 列表展示

- 列表项包含 `has_refs: boolean`，由后端基于 `schema_refs` 反向索引聚合计算
- 当前 UI **未在表头直接展示此标志**（避免列数过多），但 `has_refs=true` 时：
  - 可作为"先启用/先禁用"决策的参考
  - 未来可在操作列加 badge 提示（延后）
- **删除决策不依赖 `has_refs`**：`has_refs` 是列表快照，可能与实时不一致。删除前必须主动调 `schemaReferences` 获取最新结果

### 7.3 删除流程（核心：先查引用 → 再确认）

```
handleDelete(row)
├─ if row.enabled → 弹 EnabledGuardDialog，return
└─ else (disabled)
   ├─ await eventTypeApi.schemaReferences(row.id)
   ├─ if res.data.event_types.length > 0
   │   ├─ showRefDialog(row, eventTypes)             ← 弹引用详情
   │   ├─ ElMessage.warning(`该扩展字段被 N 个事件类型使用，无法删除`)
   │   └─ return
   └─ else (无引用)
      ├─ ElMessageBox.confirm("确认删除...")
      ├─ await eventTypeApi.schemaDelete(row.id)
      ├─ on success → ElMessage.success + fetchList()
      └─ on error:
         ├─ cancel → return
         ├─ 42027 DELETE_NOT_DISABLED → ElMessage.warning
         ├─ 42029 REF_DELETE → await loadAndShowRefs(row)   ← 后端兜底场景
         └─ other → 拦截器 toast
```

**核心原则**：
1. **只有禁用 + 无引用** 的 Schema 才能真正删除
2. **前端主动查询** 引用是第一道闸门（用户体验好）
3. **后端 REF_DELETE** 是兜底（避免前后端状态不一致的竞态）
4. 引用详情展示采用 **详情弹窗** 而非 alert，因为需要列出具体事件类型列表

### 7.4 引用详情弹窗状态

```ts
const refDialog = reactive({
  visible: false,
  loading: false,
  fieldLabel: '',
  eventTypes: [] as SchemaReferenceItem[],
})

function resetRefDialog() {   // @close 钩子调用
  refDialog.loading = false
  refDialog.fieldLabel = ''
  refDialog.eventTypes = []
}
```

两个 populate 函数：

- `showRefDialog(row, eventTypes)`：已有数据直接塞入（同步路径，删除前查到有引用）
- `loadAndShowRefs(row)`：异步拉取（42029 兜底路径），期间 `loading=true`

### 7.5 CSS 类名

引用详情区块专用 class：

```css
.ref-section { margin-bottom: 8px; }
.ref-subtitle { font-size: 13px; color: #909399; margin: 0 0 8px 0; }
.ref-empty { font-size: 13px; color: #C0C4CC; margin: 4px 0; }
```

### 7.6 Toggle 预取版本号（乐观锁保护）

`handleToggle` 在调用 `schemaToggleEnabled` 前，先调 `schemaList()` 重新获取最新列表，从中取出目标 Schema 的当前 `version`。避免列表页陈旧 version 与实际不一致导致的乐观锁冲突。

```ts
const freshRes = await eventTypeApi.schemaList()
const freshRow = (freshRes.data?.items || []).find((s) => s.id === row.id)
if (!freshRow) { /* 已被删除 */ fetchList(); return }
await eventTypeApi.schemaToggleEnabled(row.id, val, freshRow.version)
```

### 7.7 EnabledGuardDialog 的 Schema 适配

- 传入 `entity: { id, name: field_name, label: field_label }` —— 字段名到通用 `{id, name, label}` 的映射
- **不传 ref_count**（已从 GuardEntity 中移除）
- Guard 内部对 `event-type-schema` 的禁用分支：
  ```ts
  const listRes = await eventTypeApi.schemaList()
  const target = listRes.data?.items.find(s => s.id === id)
  await eventTypeApi.schemaToggleEnabled(id, false, target.version)
  ```
- 版本冲突错误码映射到 `EXT_SCHEMA_ERR.VERSION_CONFLICT`

### 7.8 约束组件 `validate()` 校验

`EventTypeSchemaForm.vue` 持有 `constraintRef`（模板引用），提交前调用 `constraintRef.value?.validate()` 进行约束级前端校验。同时表单提交前额外校验默认值是否落在 min/max 约束范围内（CheckConstraintSelf 自洽校验），失败直接 `ElMessage.error` 提示，不提交后端。

**自洽校验的两个维度**：

1. **约束参数本身合法**（如 min ≤ max、select options 非空、pattern 正则可编译）— 由约束子组件 `validate()` 负责
2. **默认值符合约束**（如 int 的 default 在 [min, max] 内、string 的 default 满足 pattern）— 由表单提交前的额外校验负责

两层校验保障前端提交给后端的数据已经是内部自洽的，后端再做一次权威校验（防绕过）。

### 7.9 约束组件 `disabled` prop

`FieldConstraintSelect` 接受 `disabled` prop，在查看模式下传入 `true`，禁用所有内部控件。`FieldConstraintInteger` / `FieldConstraintString` 遵循同样约定。

### 7.10 field_name / field_type 不可变

- **field_name**：创建后不可改（属于外部契约，一改满盘皆动）
- **field_type**：创建后不可改（改类型会让历史 default_value 和 config 值语义漂移）
- 编辑请求 `UpdateExtSchemaRequest` 中根本不带这两个字段，从源头阻断误改
- UI 层：编辑态显示为 disabled + Lock 图标 + 「不可修改」warning 提示
