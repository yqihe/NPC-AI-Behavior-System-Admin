# field-template-frontend-sync — 设计方案

## 方案描述

本方案分两块：FA（字段管理对齐）+ TA（模板管理 0→1），全部限制在 `frontend/` 内，**不动任何后端代码**。

FA 是补丁式对齐（3 个文件 ~35 行）；TA 是从 0 到 1 的新建（8 个新文件 + 2 个挂载点改动 ~2400 行）。FA 能独立上线、独立验证，所以放在执行顺序的最前面快速收尾，然后进入 TA 的大工程。

---

## FA — 字段管理前端对齐

### FA-1：`FieldConstraintReference.vue` 下拉过滤 reference 类型字段

**位置**：`frontend/src/components/FieldConstraintReference.vue`

**当前行为**：`loadEnabledFields` 调 `fieldApi.list({ enabled: true, page_size: 1000 })` 后把全部启用字段塞进 `enabledFields`，`availableFields` computed 只排除自身和已选。

**改动**：在 `loadEnabledFields` 里拿到结果后追加一道 `f.type !== 'reference'` 的过滤，让下拉从源头上看不到其他 reference 字段：

```ts
async function loadEnabledFields() {
  try {
    const res = await fieldApi.list({ enabled: true, page: 1, page_size: 1000 })
    // 过滤 reference 类型字段，与后端 validateReferenceRefs 的禁止嵌套保持一致
    enabledFields.value = (res.data?.items || []).filter((f) => f.type !== 'reference')
  } catch {
    // 拦截器已 toast
  }
}
```

**为什么前端过滤而不是后端加 `excludeType=reference` 参数**：

- 字段列表 API 是通用的，加偏门参数会污染列表契约；
- 启用字段总数预期 < 200，前端 O(n) 过滤代价极小；
- 这只是 UX 优化——后端 `validateReferenceRefs` 在新增引用路径上仍然兜底 40016。

### FA-2：`api/fields.ts` 追加 `FIELD_ERR` 常量表

**位置**：`frontend/src/api/fields.ts`

**改动**：在文件末尾追加常量表（与后端 `backend/internal/errcode/codes.go` 字段段一一对应）：

```ts
// 字段管理段错误码（40001-40017，与 backend/internal/errcode/codes.go 保持一致）
export const FIELD_ERR = {
  NAME_EXISTS:        40001,
  NAME_INVALID:       40002,
  TYPE_NOT_FOUND:     40003,
  CATEGORY_NOT_FOUND: 40004,
  REF_DELETE:         40005,
  REF_CHANGE_TYPE:    40006,
  REF_TIGHTEN:        40007,
  BB_KEY_IN_USE:      40008,
  CYCLIC_REF:         40009,
  VERSION_CONFLICT:   40010,
  NOT_FOUND:          40011,
  DELETE_NOT_DISABLED: 40012,
  REF_DISABLED:       40013,
  REF_NOT_FOUND:      40014,
  EDIT_NOT_DISABLED:  40015,
  REF_NESTED:         40016,
  REF_EMPTY:          40017,
} as const
```

### FA-3：`FieldForm.vue` 捕获 40016 / 40017 定向提示

**位置**：`frontend/src/views/FieldForm.vue`

**现状**：`handleSubmit.catch` 已经处理 `40010`（版本冲突弹 MessageBox）和 `40001 / 40002`（name 状态置 taken）。**不处理** 40016 / 40017——当前走通用拦截器默认 toast，文案与业务场景脱节。

**改动**：import `FIELD_ERR`，在 catch 分支追加：

```ts
import { fieldApi, FIELD_ERR } from '@/api/fields'
// ...
} catch (err: unknown) {
  const bizErr = err as BizError
  if (bizErr.code === FIELD_ERR.VERSION_CONFLICT) {
    ElMessageBox.alert('数据已被其他用户修改，请返回列表刷新后重试。', '版本冲突', { type: 'warning' })
    return
  }
  if (bizErr.code === FIELD_ERR.NAME_EXISTS || bizErr.code === FIELD_ERR.NAME_INVALID) {
    nameStatus.value = 'taken'
    nameMessage.value = bizErr.message
    return
  }
  if (bizErr.code === FIELD_ERR.REF_NESTED) {
    ElMessage.error('不能引用 reference 类型字段（禁止嵌套），请选择普通字段')
    return
  }
  if (bizErr.code === FIELD_ERR.REF_EMPTY) {
    ElMessage.error('reference 字段必须至少选择一个目标字段')
    return
  }
}
```

**注意**：不要把原来的 `bizErr.code === 40010` 行直接改成常量但漏掉其他分支——一并替换为 `FIELD_ERR.*` 常量，让整个 catch 读起来一致。

---

## TA — 模板管理前端 0→1

### TA-总览：组件树

```
TemplateList.vue           (列表页)
  ├─ EnabledGuardDialog.vue       (启用守卫弹窗 — 编辑/删除两条路径共用)
  └─ TemplateReferencesDialog.vue (引用详情弹窗)

TemplateForm.vue           (新建/编辑共用，mode prop 切换)
  ├─ TemplateFieldPicker.vue      (字段选择卡，按 category 分组 + 3 列网格)
  │   └─ TemplateRefPopover.vue   (reference 子字段勾选弹层)
  └─ TemplateSelectedFields.vue   (已选字段配置卡 — 必填 / 排序 / 停用字段标灰)
```

**依赖方向单向向下**：views → components → api → request；components 之间通过 props/emit 通信，不互相 import（除了 `TemplateFieldPicker` 内嵌 `TemplateRefPopover`）。

### TA-1：API 层 `frontend/src/api/templates.ts`

**类型定义**（字段与 `backend/internal/model/template.go` 的 JSON tag 严格对齐）：

```ts
export interface TemplateFieldEntry {
  field_id: number
  required: boolean
}

export interface TemplateListItem {
  id: number
  name: string
  label: string
  ref_count: number
  enabled: boolean
  created_at: string
}

export interface TemplateFieldItem {
  field_id: number
  name: string
  label: string
  type: string
  category: string
  category_label: string
  enabled: boolean   // 字段当前是否启用（停用字段在 UI 标灰 + 警告图标）
  required: boolean  // 模板里的必填配置
}

export interface TemplateDetail {
  id: number
  name: string
  label: string
  description: string
  enabled: boolean
  version: number
  ref_count: number
  created_at: string
  updated_at: string
  fields: TemplateFieldItem[]  // 顺序即 templates.fields JSON 数组顺序
}

export interface TemplateListQuery {
  label?: string
  enabled?: boolean | null
  page: number
  page_size: number
}

export interface CreateTemplateRequest {
  name: string
  label: string
  description: string
  fields: TemplateFieldEntry[]
}

export interface UpdateTemplateRequest {
  id: number
  label: string
  description: string
  fields: TemplateFieldEntry[]
  version: number
}

export interface TemplateReferenceItem {
  npc_id: number
  npc_name: string
}

export interface TemplateReferenceDetail {
  template_id: number
  template_label: string
  npcs: TemplateReferenceItem[]  // NPC 未上线时后端返回空数组（make 生成，不是 null）
}
```

**错误码常量**（含 **41012**，backend features.md 集成回顾第 7 条引入）：

```ts
export const TEMPLATE_ERR = {
  NAME_EXISTS:         41001,
  NAME_INVALID:        41002,
  NOT_FOUND:           41003,
  NO_FIELDS:           41004,
  FIELD_DISABLED:      41005,
  FIELD_NOT_FOUND:     41006,
  REF_DELETE:          41007,
  REF_EDIT_FIELDS:     41008,
  DELETE_NOT_DISABLED: 41009,
  EDIT_NOT_DISABLED:   41010,
  VERSION_CONFLICT:    41011,
  FIELD_IS_REFERENCE:  41012, // 勾选了 reference 类型字段（兜底）
} as const

export const TEMPLATE_ERR_MSG: Record<number, string> = {
  [TEMPLATE_ERR.NAME_EXISTS]:         '模板标识已存在',
  [TEMPLATE_ERR.NAME_INVALID]:        '模板标识格式不合法（需小写字母开头，仅 a-z / 0-9 / 下划线）',
  [TEMPLATE_ERR.NOT_FOUND]:           '模板不存在',
  [TEMPLATE_ERR.NO_FIELDS]:           '请至少勾选一个字段',
  [TEMPLATE_ERR.FIELD_DISABLED]:      '勾选的字段已停用，请先在字段管理中启用',
  [TEMPLATE_ERR.FIELD_NOT_FOUND]:     '勾选的字段不存在',
  [TEMPLATE_ERR.REF_DELETE]:          '该模板正被 NPC 引用，无法删除',
  [TEMPLATE_ERR.REF_EDIT_FIELDS]:     '该模板已被 NPC 引用，字段勾选与必填配置不可修改',
  [TEMPLATE_ERR.DELETE_NOT_DISABLED]: '请先停用该模板再删除',
  [TEMPLATE_ERR.EDIT_NOT_DISABLED]:   '请先停用该模板再编辑',
  [TEMPLATE_ERR.VERSION_CONFLICT]:    '该模板已被其他人修改，请刷新后重试',
  [TEMPLATE_ERR.FIELD_IS_REFERENCE]:  'reference 字段必须先展开子字段再加入模板',
}
```

**8 个 API 函数**：

```ts
export const templateApi = {
  list: (q: TemplateListQuery) =>
    request.post('/templates/list', q) as Promise<ApiResponse<ListData<TemplateListItem>>>,
  create: (req: CreateTemplateRequest) =>
    request.post('/templates/create', req) as Promise<ApiResponse<{ id: number; name: string }>>,
  detail: (id: number) =>
    request.post('/templates/detail', { id }) as Promise<ApiResponse<TemplateDetail>>,
  update: (req: UpdateTemplateRequest) =>
    request.post('/templates/update', req) as Promise<ApiResponse<string>>,
  delete: (id: number) =>
    request.post('/templates/delete', { id }) as Promise<ApiResponse<{ id: number; name: string; label: string }>>,
  checkName: (name: string) =>
    request.post('/templates/check-name', { name }) as Promise<ApiResponse<CheckNameResult>>,
  references: (id: number) =>
    request.post('/templates/references', { id }) as Promise<ApiResponse<TemplateReferenceDetail>>,
  toggleEnabled: (id: number, enabled: boolean, version: number) =>
    request.post('/templates/toggle-enabled', { id, enabled, version }) as Promise<ApiResponse<string>>,
}
```

### TA-2：列表页 `TemplateList.vue`

**视觉沿用** `FieldList.vue` 的 page-header / filter-bar / table-wrap / pagination 布局。

**关键列定义**（对齐 features.md 功能 1）：

| 列 | prop | 宽度 | 备注 |
|---|---|---|---|
| ID | `id` | 60 | 倒序排序（覆盖索引 `ORDER BY id DESC`）|
| 模板标识 | `name` | 200 | 等宽 |
| 中文标签 | `label` | min-width 200 | 列宽自适应 |
| 被引用数 | `ref_count` | 100 | `el-link type="primary"` 点击弹引用详情 |
| 启用 | `enabled` | 80 | `el-switch` 绑定 `toggleEnabled` |
| 创建时间 | `created_at` | 180 | |
| 操作 | — | 140 `fixed="right"` | `编辑` / `删除` 两个文字按钮 |

**关键交互**：

- **停用模板整行变灰**：用 `:row-class-name` 返回 `row-disabled`，scoped CSS 对 `.row-disabled` 设 `opacity: 0.5`；但操作列不能跟着灰——用 `:cell-class-name` 给操作列的 cell 追加 `action-cell` class，CSS 上加 `.row-disabled .action-cell { opacity: 1 }` 覆盖（或直接写 `.row-disabled :deep(td:not(.action-cell)) { opacity: 0.5 }`，CSS 选择器细节 smoke 时再调）。
- **handleEdit**：先判断 `row.enabled === true`，是则调 `guardRef.value.open('edit', row)` 弹守卫窗，**不发请求**；否则 `router.push('/templates/${row.id}/edit')`。
- **handleDelete**：同理，启用中弹守卫窗；已停用走 `ElMessageBox.confirm` → `templateApi.delete`，捕获 `TEMPLATE_ERR.REF_DELETE`（41007）时自动打开引用详情弹窗。
- **showRefs**：调 `templateApi.references(row.id)` 拿数据，传给 `refsRef.value.open(data)`。
- **handleToggle**：直接调 `toggleEnabled(id, !enabled, version)`，捕获 41011 时 `ElMessage.warning + refresh`。
- **搜索 / 筛选**：`label` 模糊匹配，`enabled` 三态（`null / true / false`），变化时触发 `loadList`；分页走 `page` / `page_size`，后端分页。

### TA-3：字段选择卡 `TemplateFieldPicker.vue`

**Props / Emits**：

```ts
const props = defineProps<{
  fieldPool: FieldListItem[]              // 父组件拉好的启用字段池
  disabled?: boolean                      // ref_count > 0 时整体禁用
}>()
const selectedIds = defineModel<number[]>('selectedIds', { required: true })
```

用 Vue 3.4+ 的 `defineModel`（项目 `vue ^3.5.31` 已支持）简化双向绑定。

**分组逻辑**：

```ts
interface FieldGroup {
  category: string
  label: string          // 来自 fieldPool[0..n].category_label，避免硬编码
  fields: FieldListItem[]
  selectedCount: number
}

const groupedFields = computed<FieldGroup[]>(() => {
  const map = new Map<string, FieldGroup>()
  for (const f of props.fieldPool) {
    let g = map.get(f.category)
    if (!g) {
      g = { category: f.category, label: f.category_label, fields: [], selectedCount: 0 }
      map.set(f.category, g)
    }
    g.fields.push(f)
    if (selectedIds.value.includes(f.id)) g.selectedCount++
  }
  return Array.from(map.values())
})
```

**关键点**：`category_label` **来自 `fieldPool` 里字段对象自带的 `category_label`**（后端 Service 层用 `DictCache` 翻译后返回），**不调用字典 API**，也**不硬编码**。分组顺序就是第一次出现的顺序，与字段列表接口的排序一致。

**点击交互**：

```ts
function onCellClick(f: FieldListItem) {
  if (props.disabled) return
  if (f.type === 'reference') {
    popoverRef.value?.open(f, selectedIds.value)
    return
  }
  const idx = selectedIds.value.indexOf(f.id)
  if (idx === -1) selectedIds.value = [...selectedIds.value, f.id]
  else selectedIds.value = selectedIds.value.filter((id) => id !== f.id)
}

function onPopoverConfirm(subFieldIds: number[]) {
  // 合并去重，subFieldIds 里都是 leaf 字段 ID
  const merged = Array.from(new Set([...selectedIds.value, ...subFieldIds]))
  // popover 取消勾选的也要从 selectedIds 里移除
  // 策略：把本次 popover 的 subFieldIds 作为"这个 reference 的新选择"，
  // 但 picker 不知道某个 id 是否来自这个 reference，所以 popover 直接返回
  // "popover 内的最终完整勾选集合"，picker 做：先移除所有属于这个 reference 的子字段，
  // 再合并回 subFieldIds。
  // → 更简单的做法：popover emit 时同时 emit 它负责的 refFieldId + 它的子字段全集 allSubIds + 确认后的 selected
  selectedIds.value = merged
}
```

**重要**：popover 确认时需要同时 emit「该 reference 的全部子字段 ID 集合」和「本次勾选的子字段 ID 集合」，picker 用「全集 - 勾选」做一次差集清理，再合并勾选，这样「某个 reference 先全选后来又取消某几个」的语义才对。详见 TA-4 的 emit 签名。

**CSS 要点**（scoped）：

```css
.picker-card { display: flex; flex-direction: column; gap: 24px;
               padding: 20px; background: #fff; border: 1px solid #ebeef5;
               border-radius: 6px; }
.group { display: flex; flex-direction: column; gap: 12px; }
.group-header { font-size: 13px; font-weight: 600; color: #303133; }
.group-count { margin-left: 8px; font-size: 12px; color: #909399; }
.grid { display: grid; grid-template-columns: repeat(3, 1fr); gap: 12px; }
.cell { display: flex; align-items: center; gap: 8px; padding: 0 12px;
        height: 32px; background: #fff; border: 1px solid #dcdfe6;
        border-radius: 4px; cursor: pointer; user-select: none; }
.cell.reference { border-color: #9575cd; background: #f7f3fd; }
.cell .checkbox { width: 16px; height: 16px; border: 1px solid #dcdfe6;
                  border-radius: 2px; display: flex; align-items: center;
                  justify-content: center; }
.cell .checkbox.checked { background: #409eff; border-color: #409eff;
                          color: #fff; font-size: 12px; }
.cell .chevron { color: #9575cd; font-size: 14px; margin-left: auto; }
.cell.disabled { cursor: not-allowed; opacity: 0.55; }
.picker-card.disabled { opacity: 0.55; pointer-events: none; }
```

### TA-4：reference popover `TemplateRefPopover.vue`

**形态**：独立 `el-dialog`（非 `el-popover`），原因是子字段可能 10+ 个需要可滚动 + 宽度可控 + 明确的取消/确定按钮。

**Props / Emits**：

```ts
defineExpose<{
  open: (refField: FieldListItem, currentSelectedIds: number[]) => void
}>()

const emit = defineEmits<{
  // 返回该 reference 负责的全部子字段 ID（作为差集清理依据）+ 本次勾选后的子字段 ID 集合
  confirm: [payload: { allSubIds: number[]; selectedSubIds: number[] }]
}>()
```

**数据加载**：

1. `open(refField, currentSelectedIds)` 触发时：
   - 先 `fieldApi.detail(refField.id)` 拿到 `properties.constraints.ref_fields`（注意：前端约束存的是富对象数组 `[{id, name, label, type}]`，不是平面 ID 数组——对齐 `FieldConstraintReference.vue` 的 `RefFieldItem` 类型）。
   - **最简策略**：直接用 `ref_fields` 数组里的 `id / name / label / type` 填充子字段列表，**不再二次调 `fieldApi.list`**。
   - 初始化 `tempSelected = currentSelectedIds.filter((id) => allSubIds.includes(id))`——把外部 selectedIds 中属于这个 reference 的子字段自动回勾。
2. popover 内操作 `tempSelected` 独立于父组件——**不直接修改父组件的 selectedIds**，避免取消时污染父状态。
3. 全选 / 全不选按钮只影响 `tempSelected`。
4. 确定按钮 emit `confirm` 附带 `allSubIds`（全部子字段 ID）和 `selectedSubIds`（本次勾选）。

**只读模式**（编辑页 `ref_count > 0`）：父组件传一个 `readonly` prop，popover 内禁用所有 checkbox + 取消全选按钮 + 隐藏或禁用确定按钮（只留「关闭」）。

### TA-5：已选字段配置卡 `TemplateSelectedFields.vue`

**Props / Emits**：

```ts
const props = defineProps<{
  selectedFields: TemplateFieldItem[]  // 由父组件 computed 出来（顺序即 fields 数组顺序）
  disabled?: boolean
}>()

const emit = defineEmits<{
  'update:order': [newOrder: number[]]      // 上下移动后的新 field_id 顺序
  'update:required': [fieldId: number, required: boolean]
}>()
```

**模板**（`el-table` 5 列）：

```vue
<el-table :data="selectedFields" :row-class-name="rowClassName">
  <el-table-column prop="label" label="字段标签" min-width="180">
    <template #default="{ row }">
      <el-icon v-if="!row.enabled" class="warn-icon"><WarningFilled /></el-icon>
      <span>{{ row.label }}</span>
    </template>
  </el-table-column>
  <el-table-column prop="name" label="字段标识" width="200" />
  <el-table-column label="类型" width="100">
    <template #default="{ row }"><el-tag size="small">{{ row.type }}</el-tag></template>
  </el-table-column>
  <el-table-column label="必填" width="80">
    <template #default="{ row }">
      <el-checkbox
        :model-value="row.required"
        :disabled="disabled"
        @change="(v) => emit('update:required', row.field_id, Boolean(v))"
      />
    </template>
  </el-table-column>
  <el-table-column label="排序" width="100">
    <template #default="{ $index }">
      <el-button text :disabled="disabled || $index === 0" @click="moveUp($index)">↑</el-button>
      <el-button text :disabled="disabled || $index === selectedFields.length - 1" @click="moveDown($index)">↓</el-button>
    </template>
  </el-table-column>
</el-table>
```

**停用字段标灰**：

```ts
function rowClassName({ row }: { row: TemplateFieldItem }) {
  return row.enabled ? '' : 'row-field-disabled'
}
```

```css
:deep(.row-field-disabled) { opacity: 0.55; }
.warn-icon { color: #e6a23c; margin-right: 4px; }
```

**move 实现**：纯前端 splice → emit `update:order` 新 `field_id` 数组。父组件收到后同步更新自己的 `selectedIds`。

### TA-6：表单页 `TemplateForm.vue`（新建 + 编辑共用）

**Props**：

```ts
const props = defineProps<{
  mode: 'create' | 'edit'
  id?: number  // edit 模式必传
}>()
```

**核心数据流**：

1. 本地状态：
   - `formState: reactive<{ name, label, description }>`
   - `selectedIds: Ref<number[]>`（字段选择的扁平 ID 数组，**顺序就是模板 `fields` JSON 顺序**）
   - `requiredMap: Ref<Record<number, boolean>>`（必填配置，按 `field_id` 索引）
   - `fieldPool: Ref<FieldListItem[]>`（启用字段池，用于 picker 分组）
   - `template: Ref<TemplateDetail | null>`（编辑模式下的原始模板，`version` / `ref_count` 从这里取）

2. 进入页面：
   - **create**：构造空 `formState` + 并发拉 `fieldApi.list({ enabled: true, page_size: 1000 })`；
   - **edit**：并发拉 `templateApi.detail(id)` + `fieldApi.list({ enabled: true })`；`detail` 返回的 `fields` 数组回填 `selectedIds` 和 `requiredMap`；**注意字段池只包含启用字段，但模板可能含停用字段**——停用字段需要额外加入 `fieldPool`（或者 selectedFieldsView 独立从 `detail.fields` 构造，不回查 pool）。

3. 给 picker 用的是 `fieldPool`，给已选列表用的是 computed：

```ts
const selectedFieldsView = computed<TemplateFieldItem[]>(() => {
  if (props.mode === 'edit' && template.value) {
    // 编辑模式：用 detail.fields 作为基础（含停用字段的 enabled=false 信息）
    const map = new Map(template.value.fields.map((f) => [f.field_id, f]))
    return selectedIds.value.map((id) => {
      const existing = map.get(id)
      if (existing) return { ...existing, required: requiredMap.value[id] ?? existing.required }
      // 新增字段（从 picker 勾选进来的，不在原模板里）→ 从 fieldPool 查
      const pool = fieldPool.value.find((p) => p.id === id)
      return pool
        ? { field_id: id, name: pool.name, label: pool.label, type: pool.type,
            category: pool.category, category_label: pool.category_label,
            enabled: pool.enabled, required: requiredMap.value[id] ?? false }
        : /* 不应发生，slog 级别 */ null
    }).filter(Boolean) as TemplateFieldItem[]
  }
  // create 模式：全从 fieldPool 构造
  return selectedIds.value
    .map((id) => fieldPool.value.find((f) => f.id === id))
    .filter(Boolean)
    .map((f) => ({
      field_id: f!.id, name: f!.name, label: f!.label, type: f!.type,
      category: f!.category, category_label: f!.category_label,
      enabled: f!.enabled, required: requiredMap.value[f!.id] ?? false,
    }))
})
```

**ref_count > 0 锁定**：

```ts
const isLocked = computed(() =>
  props.mode === 'edit' && (template.value?.ref_count ?? 0) > 0
)
```

模板里：

```vue
<el-alert v-if="isLocked" type="warning" :closable="false" show-icon>
  该模板已被 {{ template!.ref_count }} 个 NPC 引用，字段勾选与必填配置不可修改
</el-alert>

<TemplateFieldPicker v-model:selectedIds="selectedIds" :field-pool="fieldPool" :disabled="isLocked" />
<TemplateSelectedFields
  :selected-fields="selectedFieldsView"
  :disabled="isLocked"
  @update:order="onOrderChange"
  @update:required="onRequiredChange"
/>
```

**唯一性校验**（仅 create 模式）：

```ts
type NameStatus = 'idle' | 'checking' | 'available' | 'taken'
const nameStatus = ref<NameStatus>('idle')
const nameMessage = ref('')

async function onNameBlur() {
  if (props.mode !== 'create') return
  const name = formState.name.trim()
  if (!name) { nameStatus.value = 'idle'; return }
  if (!/^[a-z][a-z0-9_]*$/.test(name)) {
    nameStatus.value = 'taken'
    nameMessage.value = '格式不合法（小写字母开头，a-z / 0-9 / 下划线）'
    return
  }
  nameStatus.value = 'checking'
  try {
    const res = await templateApi.checkName(name)
    const { available, message } = res.data!
    nameStatus.value = available ? 'available' : 'taken'
    nameMessage.value = message
  } catch {
    nameStatus.value = 'idle'
  }
}
```

**提交**：

```ts
async function onSubmit() {
  if (selectedIds.value.length === 0) {
    ElMessage.error('请至少勾选一个字段')
    return
  }
  const payload = {
    name: formState.name,
    label: formState.label,
    description: formState.description,
    fields: selectedIds.value.map((id) => ({
      field_id: id,
      required: requiredMap.value[id] ?? false,
    })),
  }
  submitting.value = true
  try {
    if (props.mode === 'create') {
      await templateApi.create(payload)
    } else {
      await templateApi.update({
        ...payload,
        id: props.id!,
        version: template.value!.version,
      })
    }
    ElMessage.success('保存成功')
    router.push('/templates')
  } catch (err) {
    handleSubmitError(err as BizError)
  } finally {
    submitting.value = false
  }
}
```

**错误处理**（含 **41012**）：

| 错误码 | 常量 | 处理 |
|---|---|---|
| 41001 | `NAME_EXISTS` | `ElMessage.error` + `nameStatus='taken'` |
| 41002 | `NAME_INVALID` | `ElMessage.error` + `nameStatus='taken'` |
| 41003 | `NOT_FOUND` | `ElMessage.error` + `router.push('/templates')` |
| 41004 | `NO_FIELDS` | `ElMessage.error`（提交前已拦截，兜底）|
| 41005 | `FIELD_DISABLED` | `ElMessage.error` + 重新拉 `fieldPool` |
| 41006 | `FIELD_NOT_FOUND` | 同 41005 |
| 41008 | `REF_EDIT_FIELDS` | `ElMessage.error`（UI 已禁用，理论不到）|
| 41010 | `EDIT_NOT_DISABLED` | **不应到这里**——列表 UI 已拦截 |
| 41011 | `VERSION_CONFLICT` | `ElMessageBox.alert` → `router.push('/templates')` |
| **41012** | `FIELD_IS_REFERENCE` | `ElMessage.error('reference 字段必须先展开子字段再加入模板')` + 重新拉 `fieldPool`（兜底，UI 已确保不会把 reference 写入 `req.fields`）|

所有未命中上述分支的错误码走默认拦截器 toast。

### TA-7：启用守卫弹窗 `EnabledGuardDialog.vue`

**单组件复用 edit / delete 两个场景**。

```ts
defineExpose<{
  open: (action: 'edit' | 'delete', template: TemplateListItem) => void
}>()

const emit = defineEmits<{
  refresh: []   // 「立即停用」后请父组件刷新列表
}>()
```

**文案差异**（features.md 功能 9）：

| 场景 | 标题 | 正文 | 操作步骤区 |
|---|---|---|---|
| `edit` | 无法编辑模板 | 启用中模板对 NPC 管理页可见，允许任意修改可能导致策划在配置不稳定时选用 | 1. 停用该模板 2. 编辑完成后再启用 |
| `delete` | 无法删除模板 | 删除是不可恢复的操作，先停用可以提供一个观察期 | 列前置条件：✗ 模板已停用 / ✓ 没有 NPC 在使用（`ref_count === 0`）|

**「立即停用」按钮**：

- 调 `templateApi.toggleEnabled(id, false, version)`；捕获 41011 时弹版本冲突；
- 成功后：
  - `edit` → `router.push('/templates/${id}/edit')`
  - `delete` → emit `refresh` 让父组件刷新列表（父组件刷新后用户再自己点删除即可，不自动触发删除，避免连锁操作风险）

### TA-8：引用详情弹窗 `TemplateReferencesDialog.vue`

```ts
defineExpose<{
  open: (template: TemplateListItem) => void
}>()
```

`open` 时调 `templateApi.references(template.id)` 拉数据。NPC 模块未上线时后端返回 `{npcs: []}`（`make` 生成空数组而非 null），前端用 `el-empty description="暂无 NPC 引用"` 占位。

**布局**：

```vue
<el-dialog v-model="visible" title="模板引用详情" width="520px" @close="data = null">
  <div v-if="data" class="header">
    <span class="label">{{ data.template_label }} ({{ data.template_id }})</span>
    <span class="count">共 {{ data.npcs.length }} 个 NPC 在使用</span>
  </div>
  <el-empty v-if="data && data.npcs.length === 0" description="暂无 NPC 引用" />
  <el-table v-else-if="data" :data="data.npcs">
    <el-table-column prop="npc_id" label="ID" width="80" />
    <el-table-column prop="npc_name" label="NPC 名称" />
  </el-table>
</el-dialog>
```

### TA-9：路由 + 菜单挂载

**`router/index.ts`**：

```ts
{
  path: '/templates',
  name: 'template-list',
  component: () => import('../views/TemplateList.vue'),
  meta: { title: '模板管理' },
},
{
  path: '/templates/create',
  name: 'template-create',
  component: () => import('../views/TemplateForm.vue'),
  props: { mode: 'create' },
  meta: { title: '新建模板' },
},
{
  path: '/templates/:id/edit',
  name: 'template-edit',
  component: () => import('../views/TemplateForm.vue'),
  props: (route) => ({ mode: 'edit', id: Number(route.params.id) }),
  meta: { title: '编辑模板' },
},
```

**`AppLayout.vue` 菜单**：在「字段管理」`el-menu-item` 下方插入：

```vue
<el-menu-item index="/templates" @click="$router.push('/templates')">
  <el-icon><Files /></el-icon>
  <span>模板管理</span>
</el-menu-item>
```

---

## 方案对比

### 备选方案 A：用 `el-tree` 而非自写 grid 做字段选择

**做法**：每个 category 是 tree 的一个分支，字段是叶子，多选支持 checkbox。

**为什么不选**：

- features.md 明确要求「每行 3 列网格」，`el-tree` 只能单列展示；
- `el-tree` 的视觉权重远超需求（节点缩进、展开图标），跟「简朴」相反；
- reference 字段的 popover 触发用 tree 没有自然的交互位。

### 备选方案 B：把 `TemplateForm.vue` 拆成 `TemplateCreate.vue` + `TemplateEdit.vue` 两个文件

**为什么不选**：

- 新建和编辑的字段集合 95% 重合，拆开会大量复制粘贴；
- `mode` prop 切换是 Vue SFC 的常见模式，复杂度可控；
- features.md 明确说明「编辑页本身就承担查看 + 修改双重角色」，和新建页共用同一个布局结构是设计共识。

### 备选方案 C：reference popover 用 `el-popover` 而非 `el-dialog`

**为什么不选**：

- 子字段可能 10+ 个，popover 容易溢出屏幕；
- `el-popover` 的关闭交互（点击外部）在多选场景容易误触；
- `el-dialog` 提供居中 + 遮罩 + 明确的取消/确定按钮，更稳。

### 备选方案 D：用 Pinia 做模板/字段的全局状态

**为什么不选**：

- 当前项目无 Pinia 依赖（requirements 明确「不引入新运行时依赖」）；
- 状态只在两三个组件间共享，prop drilling 一层就够；
- 模板列表不需要全局缓存（每次进入页面重新拉取保证新鲜度）。

### 备选方案 E：编辑模式下把停用字段重新塞进 fieldPool

**做法**：`fieldApi.list({ enabled: true })` 之后再补一次 `fieldApi.list({ enabled: false })` 筛选出模板里用到的停用字段，合并进 pool。

**为什么不选**：

- 会污染字段选择卡的分组显示——停用字段出现在 picker 里会让用户误以为可以重新勾选；
- 实际只有「已选字段配置卡」需要展示停用字段（标灰 + 警告图标），picker 不需要；
- 改成：`selectedFieldsView` computed 里优先用 `template.fields` 作为元数据源（见 TA-6），picker 只管启用字段池，解耦干净。

### 选定方案

采用主方案。关键决策：

- 简化版字段选择卡（3 列网格 + 复选框 + 单击 cell = 勾选，reference 单击 cell = 开 popover）
- `TemplateForm.vue` 单文件 + `mode` prop（不选 B）
- `el-dialog` 做 reference popover（不选 C）
- 不引入 Pinia（不选 D）
- 自写 grid 而非 `el-tree`（不选 A）
- `selectedFieldsView` 优先从 `template.fields` 拿停用字段元数据（不选 E 的 pool 合并）

---

## 红线检查

| 红线文档 | 相关条目 | 触及 | 说明 |
|---|---|---|---|
| `standards/red-lines.md` — 禁止过度设计 | 不引入没有使用场景的依赖 | 不触及 | 不引入 Pinia / VueUse / dnd / 任何新依赖 |
| 同上 — 禁止安全隐患 | 禁止信任前端校验 | 主动遵守 | 前端的 enabled 拦截 / 字段类型过滤 / `req.fields` 扁平约束都是 UX 优化，后端 41005 / 41010 / 41012 兜底 |
| `standards/frontend-red-lines.md` | 全文 | 主动遵守 | 见下表 |
| `standards/go-red-lines.md` | — | 不触及 | 无 Go 改动 |
| `standards/mysql / redis / cache-red-lines.md` | — | 不触及 | 无后端改动 |
| `architecture/backend-red-lines.md` | — | 不触及 | 无后端改动 |
| `architecture/ui-red-lines.md` | 全文 | 主动遵守 | 见下表 |

### `standards/frontend-red-lines.md` 逐条对照

| 条目 | 本方案 |
|---|---|
| 禁止 `v-for` 不写 `:key` | 所有 `v-for` 都用 `f.id` / `g.category` 等稳定 key |
| 禁止 reactive 整体替换 | `formState` 用 `reactive`，更新走属性赋值；列表数据用 `ref` 整体替换 |
| 禁止表单关闭不重置 | guard dialog / refs dialog `@close` 时清 `data` |
| 禁止按钮无 loading 防重 | 提交按钮用 `:loading="submitting"`，submit 期间禁用 |
| 禁止响应式数据直接传非响应式 API | 所有传出去的对象都结构化或 `toRaw` |
| 禁止 `baseURL` 硬编码 | 复用现有 `request.ts` 的 `import.meta.env.VITE_API_BASE` |
| 禁止硬编码字典文案 | `category_label` 来自 `fieldPool.category_label`（后端字典翻译），不硬编码 |

### `architecture/ui-red-lines.md` 逐条对照

| 条目 | 本方案 |
|---|---|
| 禁止启用中配置的对外可见操作 | 列表点编辑/删除时前端拦截，不发请求 |
| 禁止写死分类标签 | 全部从 `category_label`（字典翻译后）取 |
| 禁止毕设阶段引入用户认证 | 不实现登录/权限 |
| 禁止页面级 dialog 嵌套 dialog | guard / refs dialog 和 popover 都是平级挂载，不嵌套 |

**结论**：无红线违反，无需申请例外。

---

## 扩展性影响

**对「新增配置类型」方向**：

- **正向**：`EnabledGuardDialog.vue` 是通用组件，未来 NPC / FSM / BT 列表都需要「启用守卫」，可直接复用；
- **正向**：`TemplateFieldPicker.vue` 的「按 category 分组 + 3 列网格 + 复选框 + popover」是通用模式；
- **正向**：`api/templates.ts` 的类型层 + 错误码常量层 + 文案映射层 + API 函数层是清晰的四段式，新增配置类型只需复制此模式。

**对「新增表单字段」方向**：

- **中性**：模板 `fields` 是 `{field_id, required}` 扁平结构，对 SchemaForm 没有依赖；
- **正向**：FA 部分巩固了 `FieldConstraint*.vue` 的契约（type 过滤 + 错误码处理），未来加 `FieldConstraintDate.vue` 等子组件有可循模式。

**反向风险**：无。本方案不引入新抽象，不改造现有共用代码。

---

## 依赖方向

```
views/TemplateList.vue
  ├→ api/templates.ts
  ├→ components/EnabledGuardDialog.vue
  └→ components/TemplateReferencesDialog.vue

views/TemplateForm.vue
  ├→ api/templates.ts
  ├→ api/fields.ts                     (拉字段池)
  ├→ components/TemplateFieldPicker.vue
  │   └→ components/TemplateRefPopover.vue
  └→ components/TemplateSelectedFields.vue

router/index.ts → views/Template*
components/AppLayout.vue → router (push)

api/templates.ts → api/request.ts
api/fields.ts    → api/request.ts
```

**依赖方向单向向下**：views → components → api → request。components 之间通过 props/emit 通信，不互相 import（`TemplateFieldPicker` 内嵌 `TemplateRefPopover` 是组合关系，不算横向依赖）。

**特别说明**：`TemplateRefPopover.vue` **不直接调 `fieldApi`**——从 `fieldApi.detail(refField.id)` 拿到的 `properties.constraints.ref_fields` 已经带了子字段的 `id / name / label / type`，直接渲染即可，不需要二次查字段列表。这也避免了「reference 引用一个停用字段」时 popover 拉不到数据的边界。

---

## 陷阱检查

### `frontend-pitfalls.md`

基于 Vue 3 + Element Plus 通用陷阱 + 本方案特有点：

- **响应式陷阱**：reactive 解构会丢响应式 → 用 `toRefs` 或保持 `props.xxx` 形式访问；ref 必须 `.value`。
- **el-form 校验**：`el-form-item` 的 `prop` 名必须与 `model` 字段名严格一致，否则 rule 不生效。
- **el-dialog 关闭复位**：关闭后 dialog 的内部 state 不会自动清，必须 `@close` 重置 form 数据。
- **el-table fixed 列**：`fixed="right"` 必须配 `width`，否则警告。
- **v-for 的 key 必须稳定**：用 ID 不用 index，删除中间项时避免渲染错位。
- **空值防御**：后端 `npcs: []`（`make` 生成的空数组）经过 JSON 反序列化仍是 `[]`，前端 `data.npcs.length` 不需要 null 检查；但 `data` 本身在 dialog 未加载时是 `null`，要 `v-if="data"`。

**针对本方案的具体陷阱预警**：

1. **`defineModel` 的 Vue 版本要求**：Vue 3.4+ 支持。项目 `package.json` 是 `vue ^3.5.31`，OK。
2. **popover 的 `tempSelected` 状态隔离**：popover 自己的 selected 是 `ref<number[]>`，**不直接修改父组件的 selectedIds**，只在确认时 emit。避免取消时已经污染父状态。
3. **`el-table` `row-class-name` 与 `cell-class-name` 同时使用**：操作列要保持高亮，用 `:cell-class-name` 给操作列追加 `action-cell`，CSS 写 `.row-disabled :deep(td):not(.action-cell) { opacity: 0.5 }`，CSS 选择器细节 smoke 时再调。
4. **`TemplateFieldPicker` 接收 picker `selectedIds` 变化的时机**：`defineModel` 触发的 update 是异步的，`groupedFields` 的 `selectedCount` 依赖 `selectedIds.value.includes(f.id)`——Vue 会自动响应式追踪，无需手动。
5. **编辑模式下 `selectedFieldsView` 的停用字段**：从 `template.fields` 取 `enabled=false` 的字段元数据，**不**从 `fieldPool` 取（pool 里没有这些字段）。见 TA-6 的 computed 实现。
6. **reference popover emit 的差集清理语义**：popover 需要同时 emit `allSubIds` + `selectedSubIds`，父组件做「先从 selectedIds 里移除 allSubIds 中出现的，再合并 selectedSubIds」，才能正确表达「取消勾选某几个子字段」。

### `go-pitfalls.md` / `mysql-pitfalls.md` / `redis-pitfalls.md` / `cache-pitfalls.md` / `mongodb-pitfalls.md`

**不触及**——本方案纯前端，无后端/存储/缓存改动。

---

## 配置变更

**无**。不新增/修改任何 YAML / JSON 配置文件。

`vite.config.js` 不动（auto-import 已覆盖 Element Plus 组件 + 图标）；`tsconfig.json` 不动（保持 strict）。

**唯一需要确认**：`Check` / `ArrowRight` / `WarningFilled` / `Files` / `Lock` / `Plus` / `Search` 等 Element Plus 图标如未被 `unplugin-auto-import` 覆盖，需在使用处 `import { Check } from '@element-plus/icons-vue'`。这是显式 import，不算配置变更。

---

## 测试策略

**项目当前没有前端测试基建**（无 Vitest / Cypress / Playwright），按 requirements R25 / R26 走两个层级：

### 静态检查

- `npx vue-tsc --noEmit`：**零错误**（memory: `feedback_vue_tsc_required`）。**每个任务完成后必跑**。
- `npm run build`：构建通过。

### 手动 e2e（R26 验收脚本）

按以下顺序在浏览器里跑一遍，全部通过即认为 spec 完成：

1. **菜单可见**：刷新 `/` → 左侧菜单出现「模板管理」项。
2. **空列表态**：进入 `/templates` → 有「暂无数据」或空表格。
3. **创建字段池**：先去 `/fields` 创建 3 个字段（`hp` / `atk` / `patrol_path`）并启用。
4. **创建模板**：去 `/templates/create` → 标识 `combat_npc`、中文标签「战斗 NPC」→ 字段池里勾上 `hp + atk` → 已选字段配置出现两行 → `atk` 必填勾上 → 上下移动一次 → 保存。
5. **列表展示**：跳回列表，新模板出现在第一行（id DESC）。
6. **启用模板**：点 `enabled` switch → 开关变绿。
7. **启用中编辑拦截**：点「编辑」→ **不发请求**，弹「无法编辑模板」对话框 → 点「立即停用」→ 跳到编辑页且模板已停用。
8. **编辑保存**：在编辑页改中文标签 → 保存 → 跳回列表 → 标签已更新。
9. **被引用数链接**：点 `ref_count` 链接（即使是 0）→ 弹引用详情对话框 → 显示「暂无 NPC 引用」。
10. **删除已停用**：点「删除」→ 二次确认 → 列表消失。
11. **软删 name 不可复用**：再次创建 `combat_npc` → 提示「模板标识已存在」（41001）。
12. **reference 字段流程**：创建 reference 字段 `combat_ext`（`refs` 包含 `hp` 和 `atk`）→ 启用 → 创建另一个模板 → 字段池里看到 `combat_ext`（紫色边框 + chevron）→ 点击它弹 popover → 勾选 `hp` → 确定 → 已选字段配置出现 `hp`（去重，`atk` 没有因为没勾）→ 保存 → 列表出现。
13. **字段管理 FA-1 验证**：编辑一个 reference 字段 → 打开「被引用字段」下拉 → 看不到其他 reference 字段（R1）。
14. **FA-2 / FA-3 验证**：用浏览器 devtools 直接发一个 reference `refs` 包含另一个 reference 的 `update` → 前端弹中文 `ElMessage "不能引用 reference 类型字段（禁止嵌套）…"`；同理发 `refs=[]` 的 update → 弹「reference 字段必须至少选择一个目标字段」。
15. **41012 兜底验证**：devtools 直接发一个 `/templates/create` 请求，`fields` 里放一个 reference 字段的 `field_id` → 前端弹「reference 字段必须先展开子字段再加入模板」，不进列表。
16. **停用字段警告**：把 `atk` 字段停用 → 编辑之前创建的模板 → 已选字段配置卡中 `atk` 整行灰 + 左侧警告图标。

### 集成测试

后端 `tests/api_test.sh` 已经覆盖所有 API 行为（199/199 通过），前端不重复测 API 层。

---

**Phase 2 完成，停下等待审批**。审批通过后进入 Phase 3 任务拆解。
