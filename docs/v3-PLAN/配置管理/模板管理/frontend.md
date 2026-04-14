# 模板管理 — 前端设计

> **实现状态**：已全部落地，Vue 3.5 + TypeScript strict + Element Plus + Vite。
> 通用前端规范见 `docs/development/standards/red-lines/frontend.md` 和 `dev-rules/frontend.md`。
>
> **本次重构（ref-cleanup 分支）**：
> - 列表页和详情页类型都移除了 `ref_count`（后端不再计算模板级引用数）。
> - 列表表格移除「被引用数」列；`TemplateReferencesDialog.vue` 组件整体删除。
> - 编辑表单移除 `isLocked` 计算态、header 上的「被 N 个 NPC 引用」Tag、以及顶部 `<el-alert>` 锁定提示；字段选择/已选字段卡片的禁用态只看 `isView`。
> - 删除流程在 NPC 模块上线前简化为「停用 → 二次确认 → 直接调 delete」，`REF_DELETE(41007)` catch 分支保留为占位。
> - reference 子字段选择器新增 `mode` 传递链：`TemplateForm → TemplateFieldPicker → TemplateRefPopover`，create 模式过滤停用子字段，edit/view 模式保留并标灰。

---

## 1. 目录结构

```
frontend/src/
├── api/
│   └── templates.ts                      # 类型定义 + TEMPLATE_ERR(41001-41012) + TEMPLATE_ERR_MSG + 8 个 API 函数
├── views/
│   ├── TemplateList.vue                  # 列表页：筛选 / 分页 / guard 拦截 / toggle / 删除
│   └── TemplateForm.vue                  # 新建 + 编辑 + 查看共用（mode: 'create' | 'edit' | 'view'）
├── components/
│   ├── TemplateFieldPicker.vue           # 字段选择卡：按 category 分组 + 3 列网格 + reference popover 触发（接收 mode prop）
│   ├── TemplateRefPopover.vue            # reference 子字段勾选弹层（el-dialog 形态，接收 filterDisabled）
│   ├── TemplateSelectedFields.vue        # 已选字段配置卡：必填 / 上下移动 / 停用字段标灰警告
│   └── EnabledGuardDialog.vue            # 启用守卫（与字段管理共用，entityType 泛型）
└── router/index.ts                       # /templates, /templates/create, /templates/:id/edit, /templates/:id/view
```

> 历史上的 `TemplateReferencesDialog.vue`（模板被引用 NPC 列表弹窗）已在 ref-cleanup 重构中整体删除。列表页不再承担展示模板级引用 NPC 的职责，后续 NPC 模块上线后若需此能力，将以独立的「NPC 按模板分组」视图形式在 NPC 管理页提供。

共享依赖：`api/fields.ts`（FieldListItem 类型 + fieldApi.list 拉字段池 + fieldApi.detail 拉 reference 子字段）、`api/request.ts`、`components/AppLayout.vue`。

---

## 2. 页面路由

| 路径 | 组件 | route name | route meta / props |
|---|---|---|---|
| `/templates` | `TemplateList.vue` | `template-list` | `{ title: '模板管理' }` |
| `/templates/create` | `TemplateForm.vue` | `template-create` | `props: { mode: 'create' }` |
| `/templates/:id/edit` | `TemplateForm.vue` | `template-edit` | `props: (route) => ({ mode: 'edit', id: Number(route.params.id) })` |
| `/templates/:id/view` | `TemplateForm.vue` | `template-view` | `props: (route) => ({ mode: 'view', id: Number(route.params.id) })` |

`TemplateForm.vue` 通过 `defineProps<{ mode: 'create' | 'edit' | 'view'; id?: number }>()` 接收路由注入的 props，内部派生：

```ts
const isEdit = computed(() => props.mode === 'edit' || props.mode === 'view')
const isView = computed(() => props.mode === 'view')
```

`isEdit` 用于判断「是否需要预加载详情」，`isView` 用于禁用所有交互（包括字段选择、必填切换、排序、弹层确认按钮）。

---

## 3. 组件树

```
TemplateList.vue
  └─ EnabledGuardDialog.vue              (启用守卫，与字段管理泛型复用)

TemplateForm.vue (mode: 'create' | 'edit' | 'view', id?: number)
  ├─ TemplateFieldPicker.vue             (接收 :mode="mode"，用于 popover filterDisabled)
  │   └─ TemplateRefPopover.vue          (picker 持有 ref，open(refField, selectedIds, isCreateMode) 调用)
  └─ TemplateSelectedFields.vue
```

依赖方向单向向下：`views -> components -> api -> request`。`TemplateRefPopover` 不直接读 `mode`，而是通过 `open()` 第三参数 `filterDisabled` 接收「是否过滤停用子字段」的布尔值，保持组件纯粹。

---

## 4. 类型契约

```ts
// --- api/templates.ts ---

/** 模板字段单元（templates.fields JSON 数组的元素） */
interface TemplateFieldEntry {
  field_id: number
  required: boolean
}

/** 列表项（覆盖索引返回，不含 fields/description/version，toggle 前需调 detail 拿 version） */
interface TemplateListItem {
  id: number
  name: string
  label: string
  enabled: boolean
  created_at: string
  // 注意：ref-cleanup 重构后移除 ref_count
  // 注意：列表接口不返回 version/description/fields
}

/** 详情中的字段精简信息（顺序即 templates.fields JSON 数组顺序） */
interface TemplateFieldItem {
  field_id: number
  name: string
  label: string
  type: string
  category: string
  category_label: string   // 后端 DictCache 翻译后返回
  enabled: boolean         // 字段当前是否启用（停用字段在 UI 整行灰 + 警告图标）
  required: boolean        // 模板里的必填配置
}

/** 详情（handler 层拼装，不进缓存） */
interface TemplateDetail {
  id: number
  name: string
  label: string
  description: string
  enabled: boolean
  version: number
  created_at: string
  updated_at: string
  fields: TemplateFieldItem[]
  // 注意：ref-cleanup 重构后移除 ref_count
}

interface TemplateListQuery {
  label?: string
  enabled?: boolean | null
  page: number
  page_size: number
}

interface CreateTemplateRequest {
  name: string
  label: string
  description: string
  fields: TemplateFieldEntry[]
}

/** 编辑请求（无 name，name 创建后不可变） */
interface UpdateTemplateRequest {
  id: number
  label: string
  description: string
  fields: TemplateFieldEntry[]
  version: number
}

/** NPC 引用方（NPC 模块未上线前 npcs 恒为空数组 make 生成） */
interface TemplateReferenceItem {
  npc_id: number
  npc_name: string
}

interface TemplateReferenceDetail {
  template_id: number
  template_label: string
  npcs: TemplateReferenceItem[]
}

const TEMPLATE_ERR = {
  NAME_EXISTS: 41001, NAME_INVALID: 41002, NOT_FOUND: 41003,
  NO_FIELDS: 41004, FIELD_DISABLED: 41005, FIELD_NOT_FOUND: 41006,
  REF_DELETE: 41007, REF_EDIT_FIELDS: 41008, DELETE_NOT_DISABLED: 41009,
  EDIT_NOT_DISABLED: 41010, VERSION_CONFLICT: 41011, FIELD_IS_REFERENCE: 41012,
} as const

const TEMPLATE_ERR_MSG: Record<number, string> = {
  41001: '模板标识已存在',
  41002: '模板标识格式不合法，需小写字母开头，仅允许 a-z、0-9、下划线',
  41003: '模板不存在',
  41004: '请至少勾选一个字段',
  41005: '勾选的字段已停用，请先在字段管理中启用',
  41006: '勾选的字段不存在',
  41007: '该模板正被 NPC 引用，无法删除',
  41008: '该模板已被 NPC 引用，字段勾选与必填配置不可修改',
  41009: '请先停用该模板再删除',
  41010: '请先停用该模板再编辑',
  41011: '该模板已被其他人修改，请刷新后重试',
  41012: 'reference 字段必须先展开子字段再加入模板',
}
```

**ref_count 移除说明**：
- 后端在 ref-cleanup 重构中不再维护模板级引用计数（原有的 `ref_count` 列与同步逻辑已从 MySQL 表和 service 层剥离）。
- 前端所有依赖 `ref_count` 的 UI（表格列、header Tag、`<el-alert>`、`isLocked`）同步移除。
- `TemplateReferenceItem` / `TemplateReferenceDetail` 类型暂时保留（`templateApi.references` 目前也保留），但在 NPC 模块上线前无实际调用点。

**TemplateRefPopover 内部类型（ref-cleanup 新增 `enabled` 字段）**：

```ts
// --- components/TemplateRefPopover.vue ---

/** reference 子字段的 UI 展示对象（后端持久化格式是 constraints.refs: number[]，此处是本组件 open 时回填的富对象） */
interface RefFieldItem {
  id: number
  name: string
  label: string
  type: string
  type_label?: string
  enabled: boolean   // 新增：子字段当前是否启用，用于 edit/view 模式标灰展示
}
```

**关键设计**：`TemplateRefPopover.vue` 读字段 detail 的 `constraints.refs: number[]`（后端权威格式），不假设 `ref_fields` 富对象存在。读到 id 列表后并发 `fieldApi.detail(subId)` 拿子字段元数据（包括 `enabled`），再由 `open()` 的第三参数 `filterDisabled` 决定是否跳过停用子字段。

---

## 5. API 调用映射

| UI 操作 | API 函数 | 后端端点 |
|---|---|---|
| 列表加载 / 筛选 / 翻页 | `templateApi.list(params)` | `POST /api/v1/templates/list` |
| 新建模板 | `templateApi.create(data)` | `POST /api/v1/templates/create` |
| 查看详情（编辑/查看页加载 / toggle 取 version） | `templateApi.detail(id)` | `POST /api/v1/templates/detail` |
| 编辑模板 | `templateApi.update(data)` | `POST /api/v1/templates/update` |
| 删除模板 | `templateApi.delete(id)` | `POST /api/v1/templates/delete` |
| 标识符唯一性校验（blur） | `templateApi.checkName(name)` | `POST /api/v1/templates/check-name` |
| 引用详情（NPC 模块未上线，占位保留） | `templateApi.references(id)` | `POST /api/v1/templates/references` |
| 切换启用/停用 | `templateApi.toggleEnabled(id, enabled, version)` | `POST /api/v1/templates/toggle-enabled` |
| 拉取启用字段池（表单页） | `fieldApi.list({ enabled: true, page: 1, page_size: 1000 })` | `POST /api/v1/fields/list` |
| 拉取 reference 子字段详情（popover） | `fieldApi.detail(subId)` 并发 | `POST /api/v1/fields/detail` |

**共 8 个模板域接口 + 2 个字段域接口复用**。其中 `templateApi.references` 本轮重构后无 UI 调用点（弹窗已删除），保留为「NPC 模块上线占位」，后续在 NPC 管理侧以独立视图形式消费。

---

## 6. 错误码处理

| 错误码 | 常量名 | UI 反馈 |
|---|---|---|
| 41001 | `NAME_EXISTS` | form 内联红字：`nameStatus='taken'` + `nameMessage` |
| 41002 | `NAME_INVALID` | form 内联红字：同上 |
| 41003 | `NOT_FOUND` | `ElMessage.error` + `router.push('/templates')` |
| 41004 | `NO_FIELDS` | 提交前已前端拦截（`selectedIds.length === 0` 检查），兜底 toast |
| 41005 | `FIELD_DISABLED` | `ElMessage.error` + `reloadFieldPool()` 重拉字段池 |
| 41006 | `FIELD_NOT_FOUND` | `ElMessage.error` + `reloadFieldPool()` 重拉字段池 |
| 41007 | `REF_DELETE` | **占位分支**：`handleDelete` catch 中保留 `BizError` 解包注释，当前 NPC 模块未上线，后端不会返回此码；NPC 上线后此分支启用，再弹 `TemplateReferencesDialog` 或跳转 NPC 管理 |
| 41008 | `REF_EDIT_FIELDS` | NPC 模块未上线前后端不会返回，保留为协议兼容位；走默认 toast |
| 41009 | `DELETE_NOT_DISABLED` | 列表 `EnabledGuardDialog` 前置拦截（启用态不允许删除），理论不到此分支 |
| 41010 | `EDIT_NOT_DISABLED` | 列表 `EnabledGuardDialog` 前置拦截（启用态不允许编辑），理论不到此分支 |
| 41011 | `VERSION_CONFLICT` | `ElMessageBox.alert` 提示后 `router.push('/templates')` |
| 41012 | `FIELD_IS_REFERENCE` | `ElMessage.error('reference 字段必须先展开子字段再加入模板')` + `reloadFieldPool()` |

> 注：`REF_DELETE / REF_EDIT_FIELDS` 在 ref-cleanup 后端重构中也从模板域 service 的「写前检查」中移除，仅作为错误码常量与协议位保留。等 NPC 模块引入「NPC 引用模板」约束时，会以新的检查路径（可能在 NPC 域或 ref-system 中间层）重新产出这两个码。

---

## 7. 关键实现细节

### 7.1 「对新隐藏、对旧保留」模式

这是本模块贯穿字段选择卡、已选字段卡、reference 子字段弹层的统一策略：

- **新建模式（`mode === 'create'`）**：
  - 字段池接口 `fieldApi.list({ enabled: true, ... })` 天然只返回启用字段，字段选择卡不会出现停用项。
  - reference 子字段弹层 `TemplateRefPopover.open(refField, selectedIds, filterDisabled=true)`，内部跳过 `!d.enabled` 的子字段，用户完全看不到停用子字段。
  - 设计意图：新建场景鼓励用户只选当下有效的配置，减少无效组合。

- **编辑模式（`mode === 'edit'`）/ 查看模式（`mode === 'view'`）**：
  - 字段池仍然只拉启用字段，但 `TemplateForm` 的 `selectedFieldsView` 计算态会**优先从 `template.fields`（详情返回值）取字段元数据**，只有详情中不存在的 id 才退回到 `fieldPool.find()`。这保证了「模板里存的某个字段后来被停用」这种历史数据仍然能正常展示（带 `enabled: false` + 警告图标 + 「已禁用」Tag）。
  - reference 子字段弹层 `filterDisabled=false`，停用子字段保留在列表中，渲染为灰色、显示「已停用」Tag、checkbox `disabled`。
  - 设计意图：旧数据不能因为字段停用而「消失」，否则用户无法感知历史配置、也无法主动清理。

### 7.2 reference 子字段 mode 传递链

完整的数据流向：

```
TemplateForm.vue (mode: 'create' | 'edit' | 'view')
  └─ <TemplateFieldPicker :mode="mode" ...>
       │
       ├─ (computed) isCreateMode = props.mode === 'create'
       │
       └─ onCellClick(f):
            if (f.type === 'reference')
              popoverRef.value?.open(f, selectedIds.value, isCreateMode.value)
                                                           ^^^^^^^^^^^^^^^^^
                                                           第三参数即 filterDisabled
  
TemplateRefPopover.vue
  └─ async open(refField, currentSelectedIds, filterDisabled = false)
       ├─ 并发拉 refIds.map(id => fieldApi.detail(id))
       └─ for each sub:
            if (filterDisabled && !d.enabled) continue   // create 模式过滤
            items.push({ ..., enabled: d.enabled })      // edit/view 保留
```

**为什么不把 `mode` 字符串一路透传到 popover？**
`TemplateRefPopover` 是展示组件，它只关心「是否过滤停用」这一纯布尔语义，不关心业务模式。把「`mode → filterDisabled`」的语义翻译收敛到 `TemplateFieldPicker`，让 popover 保持职责纯粹、易于单测。这也与 popover 已有的 `readonly?: boolean` 参数风格一致——都是纯 UI 层的布尔开关。

### 7.3 禁用字段在已选列表的灰显

`TemplateSelectedFields.vue` 已原生支持（本轮重构无改动）：

- 行级：`rowClassName` 返回 `'row-field-disabled'`，配合 `:deep(.row-field-disabled) { opacity: 0.55; }` 使整行变灰。
- 标签列：左侧显示橙色 `WarningFilled` 图标；标签文字右侧追加 `<el-tag type="warning">已禁用</el-tag>`。
- 行内交互：`<el-checkbox :disabled="disabled">` 和排序按钮依然响应父级 `disabled`（`isView`），但由于整行灰度使得视觉上「看得见、不建议改」。

`TemplateFieldPicker.vue` 和 `TemplateSelectedFields.vue` 的 `:disabled` 现在只绑定 `isView`，不再像旧版本拼 `isView || isLocked`——因为 `isLocked` 已被彻底删除，编辑模式下任何已选字段（包括因后端停用而变灰的历史字段）都可以取消勾选、调整必填、改顺序。

### 7.4 字段池 enabled=true 过滤

表单页初始化和 `reloadFieldPool()` 均调用：

```ts
fieldApi.list({ enabled: true, page: 1, page_size: 1000 })
```

并对返回结果做一次 `id ASC` 排序（后端返回 `id DESC`），保证字段选择卡始终按「旧在前、新在后」的稳定顺序展示。

**为何必须前端再排一次**：后端列表接口遵循「最新在前」的通用约定（id DESC），但模板字段选择 UI 需要稳定顺序以避免每次新增字段后旧字段位置跳动。这条排序在两次加载路径（`onMounted` 和 `reloadFieldPool`）必须一致。

### 7.5 删除流程（ref-cleanup 简化）

```ts
async function handleDelete(row: TemplateListItem) {
  if (row.enabled) {
    // 启用态走守卫弹窗，提示先停用
    guardRef.value?.open({ action: 'delete', entityType: 'template', entity: row })
    return
  }
  try {
    await ElMessageBox.confirm(
      `确认删除模板「${row.label}」（${row.name}）？删除后无法恢复，模板标识也不可再复用。`,
      '删除确认',
      { confirmButtonText: '确认删除', cancelButtonText: '取消', type: 'warning' },
    )
    await templateApi.delete(row.id)
    ElMessage.success('删除成功')
    fetchList()
  } catch (err: unknown) {
    if (err === 'cancel') return
    const bizErr = err as BizError
    // REF_DELETE(41007) 占位：NPC 上线后启用
    //   if (bizErr.code === TEMPLATE_ERR.REF_DELETE) { ... 弹引用详情 ... }
    // 其他错误拦截器已 toast
  }
}
```

对比旧版：不再有「先调 `templateApi.references` 预检、有 NPC 引用则直接 `TemplateReferencesDialog.open()`」的前置步骤，直接进入确认→delete 流程。NPC 模块上线后，该前置预检可能以「在守卫弹窗里展示引用列表」或「NPC 模块主动维护反向索引」两种形式回归，具体方案延后至 NPC spec 阶段再定。

### 7.6 Toggle 启用/停用的 version 获取

列表接口不返回 `version`（覆盖索引优化），`handleToggle` 中先 `templateApi.detail(row.id)` 拿最新版本再调 toggle：

```ts
const detail = await templateApi.detail(row.id)
await templateApi.toggleEnabled(row.id, val, detail.data.version)
```

如返回 `VERSION_CONFLICT(41011)`，`ElMessageBox.alert` 提示「该模板已被其他人修改」后 `fetchList()` 刷新列表。

### 7.7 version 存储（TemplateForm.vue）

使用独立 `const version = ref(0)` 存储版本号（在 `onMounted` 加载详情时赋值），提交时用 `version.value`。不使用 `template.value!.version` 非空断言，避免加载失败时 `TypeError`。

### 7.8 reloadFieldPool 排序

`reloadFieldPool()` 和初始加载一样，必须对字段池按 `id ASC` 排序（`.sort((a, b) => a.id - b.id)`），保证字段选择卡的展示顺序在「41005 字段停用」「41006 字段被删」「41012 reference 字段未展开」三类错误后重拉时仍然稳定。

### 7.9 ListData 共享

`templates.ts` 从 `fields.ts` 导入 `ListData<T>` 和 `CheckNameResult`，不重复定义。列表接口返回 `ListData<TemplateListItem>`，结构与字段列表完全一致（`items + total`）。

### 7.10 View 模式 disabled 统一

ref-cleanup 之前：`:disabled="isView || isLocked"`（`isLocked` 基于 `ref_count > 0`）。
ref-cleanup 之后：`:disabled="isView"`（`isLocked` 已删除）。

编辑模式下任何场景都不再锁字段勾选；如果后续 NPC 模块上线要求「有 NPC 引用时字段勾选不可改」，将以 backend 41008 错误码 + 前端在提交失败时回滚并 toast 的方式处理，而非前置 UI 锁。这样避免了「前端算的锁」和「后端算的锁」两套状态同步问题。
