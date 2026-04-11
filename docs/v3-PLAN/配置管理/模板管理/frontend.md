# 模板管理 — 前端设计

> **实现状态**：已全部落地，Vue 3.5 + TypeScript strict + Element Plus + Vite。
> 代码位于 `frontend/src/{views,components,api}/template*.ts|vue`，与字段管理共享 `api/request.ts` / `EnabledGuardDialog.vue` / `AppLayout.vue`。
> spec 历史（需求 / 设计 / 任务拆解）详见 [`docs/specs/field-template-frontend-sync`](../../../specs/field-template-frontend-sync)。
> 通用前端规范 / 陷阱 / 红线见 `docs/development/frontend-pitfalls.md` 与 `docs/standards/frontend-red-lines.md`。
> 本文档只记录模板管理特有的设计与实现事实。

---

## 文件清单

```
frontend/src/
├── api/
│   └── templates.ts                      # 类型定义 + TEMPLATE_ERR(41001-41012 含 41012) + TEMPLATE_ERR_MSG + 8 个 API 函数
├── views/
│   ├── TemplateList.vue                  # 列表页：筛选 / 分页 / guard 拦截 / refs 详情 / toggle / 删除
│   └── TemplateForm.vue                  # 新建 + 编辑共用（mode prop 切换）
├── components/
│   ├── TemplateFieldPicker.vue           # 字段选择卡：按 category 分组 + 3 列网格 + reference popover 触发 + 差集清理
│   ├── TemplateRefPopover.vue            # reference 子字段勾选弹层（el-dialog 形态）
│   ├── TemplateSelectedFields.vue        # 已选字段配置卡：必填 / 上下移动 / 停用字段标灰警告
│   ├── TemplateReferencesDialog.vue      # 被引用 NPC 详情弹窗（NPC 未上线显示 el-empty 占位）
│   └── EnabledGuardDialog.vue            # 启用守卫（与字段管理共用，entityType 泛型）
└── router/index.ts                       # /templates, /templates/create, /templates/:id/edit
```

**挂载点**：

- `router/index.ts`：3 条路由，`/templates/create` 注入 `mode: 'create'`，`/templates/:id/edit` 通过 `props: (route) => ({ mode: 'edit', id: Number(route.params.id) })` 注入
- `components/AppLayout.vue`：侧栏改用 `el-sub-menu` 分级分组「配置管理」，「模板管理」菜单项在「字段管理」上方（策划最常用的入口放前）

---

## 组件树

```
TemplateList.vue
  ├─ EnabledGuardDialog.vue           (启用守卫，与字段管理泛型复用)
  └─ TemplateReferencesDialog.vue     (引用详情)

TemplateForm.vue (mode: 'create' | 'edit', id?: number)
  ├─ TemplateFieldPicker.vue
  │   └─ TemplateRefPopover.vue       (内嵌，picker 持有 ref 并 open)
  └─ TemplateSelectedFields.vue
```

依赖方向单向向下：`views → components → api → request`。Service 层无，错误码走 `TEMPLATE_ERR` 常量。

---

## 类型契约（与后端对齐）

```ts
interface TemplateListItem {
  id: number
  name: string
  label: string
  ref_count: number
  enabled: boolean
  created_at: string
  // 注意：列表接口不返回 version，toggle 前需调 detail 拉 version
}

interface TemplateFieldItem {
  field_id: number
  name: string
  label: string
  type: string
  category: string
  category_label: string   // 后端 DictCache 翻译后返回，前端不重复查字典
  enabled: boolean         // 字段当前是否启用（false → 已选字段配置卡整行灰 + ⚠ 图标）
  required: boolean        // 模板里的必填配置
}

interface TemplateDetail {
  id, name, label, description, enabled, version, ref_count
  created_at, updated_at
  fields: TemplateFieldItem[]   // 顺序即 templates.fields JSON 数组顺序
}

const TEMPLATE_ERR = {
  NAME_EXISTS: 41001, NAME_INVALID: 41002, NOT_FOUND: 41003, NO_FIELDS: 41004,
  FIELD_DISABLED: 41005, FIELD_NOT_FOUND: 41006, REF_DELETE: 41007,
  REF_EDIT_FIELDS: 41008, DELETE_NOT_DISABLED: 41009, EDIT_NOT_DISABLED: 41010,
  VERSION_CONFLICT: 41011, FIELD_IS_REFERENCE: 41012,
} as const

const TEMPLATE_ERR_MSG: Record<number, string> = { ... }  // 12 条中文文案
```

---

## TemplateList 交互

### 列表视觉

| 列 | 字段 | 渲染 |
|---|---|---|
| ID | `id` | 纯文本 |
| 模板标识 | `name` | 等宽 |
| 中文标签 | `label` | 主信息，列宽自适应 |
| 被引用数 | `ref_count` | 蓝色 `el-link`，点击弹 `TemplateReferencesDialog` |
| 启用 | `enabled` | `el-switch`，变化时先 `detail` 拉 version 再 `toggleEnabled` |
| 创建时间 | `created_at` | `YYYY-MM-DD HH:mm:ss` |
| 操作 | — | `编辑` / `删除` 文字链接 |

**停用行视觉**：`:row-class-name` 返回 `row-disabled`，scoped CSS `:deep(.row-disabled td:not(:last-child))` 让除操作列外整行 `opacity: 0.5`，操作列保持高亮。

### 行交互

| 操作 | 逻辑 |
|---|---|
| 编辑 | `row.enabled === true` → `guardRef.open({action:'edit', entityType:'template', entity:row})`；否则 `router.push('/templates/${id}/edit')` |
| 删除 | `row.enabled === true` → 守卫弹窗同上；已停用且 `ref_count > 0` → 自动弹引用详情 + warning toast；已停用无引用 → `ElMessageBox.confirm` → `delete API` → 刷新；捕获 41007 时自动弹引用详情（兜底 race condition）|
| 被引用数链接 | `refsRef.open(row)` 调 `templateApi.references(id)` 展示 `npcs`（NPC 未上线时后端返回空数组，前端显 `el-empty "暂无 NPC 引用"`）|
| toggle | `detail` 拉 version → `toggleEnabled(id, !enabled, version)`；41011 → `ElMessageBox.alert` + refetch |

---

## TemplateForm 状态流

### 核心状态

```ts
const selectedIds = ref<number[]>([])                     // 扁平 field_id 数组，顺序即 templates.fields JSON 顺序
const requiredMap = ref<Record<number, boolean>>({})      // 按 field_id 索引的必填
const fieldPool = ref<FieldListItem[]>([])                // 启用字段池，id ASC 排序
const template = ref<TemplateDetail | null>(null)         // 编辑模式原始数据，提供 version / ref_count / 停用字段元数据
```

### selectedFieldsView computed：stopped-field 元数据优先策略

```ts
const selectedFieldsView = computed<TemplateFieldItem[]>(() => {
  const detailMap = new Map()
  if (isEdit.value && template.value) {
    for (const f of template.value.fields) detailMap.set(f.field_id, f)
  }
  return selectedIds.value.map(id => {
    // 编辑模式优先用 template.fields 的元数据（含停用字段 enabled=false）
    const fromDetail = detailMap.get(id)
    if (fromDetail) return { ...fromDetail, required: requiredMap.value[id] ?? fromDetail.required }
    // 否则（create 模式或编辑时新增的字段）从 fieldPool 构造
    const fromPool = fieldPool.value.find(f => f.id === id)
    return fromPool ? { field_id: id, ..., required: requiredMap.value[id] ?? false } : null
  }).filter(Boolean)
})
```

**为什么优先 `template.fields`**：字段停用后不再出现在 `fieldPool`（`fieldApi.list({enabled:true})`），但 `templateApi.detail` 仍然返回该字段的元数据（因为后端 `GetByIDsLite` 不过滤 enabled）。编辑模式下必须用 detail 的元数据才能显示「已停用字段」的警告图标。

### fieldPool 排序

`onMounted` 拉到字段池后排一次 `id ASC`：

```ts
fieldPool.value = [...(fieldsRes.data?.items || [])].sort((a, b) => a.id - b.id)
```

后端列表接口默认 `id DESC`（新在前），字段选择卡需要**按创建顺序正序**展示（最早创建的在前，符合直觉——基础属性通常最先创建）。

### TemplateFieldPicker 的差集清理语义

Popover 确认时 emit `{ allSubIds: number[], selectedSubIds: number[] }`，picker 收到后：

```ts
function onPopoverConfirm({ allSubIds, selectedSubIds }) {
  // 先从 selectedIds 里移除本 reference 负责的所有子字段（无论之前是否在选中状态）
  const withoutSubs = selectedIds.value.filter(id => !allSubIds.includes(id))
  // 再合并本次确认的选中子字段
  selectedIds.value = Array.from(new Set([...withoutSubs, ...selectedSubIds]))
  // 记忆这个 reference 的子字段集合，供下次开 popover 时 "has-sub-selected" 高亮
  refFieldSubIdsMap.value[pendingRefFieldId.value!] = allSubIds
}
```

这种"差集清理 + 合并"而非"单边合并"的写法，才能正确表达「用户之前在 popover 里勾了 A B C，这次取消了 B」的语义。

### ref_count > 0 锁定

```ts
const isLocked = computed(() => isEdit.value && (template.value?.ref_count ?? 0) > 0)
```

- 顶部黄色 `el-alert` 警告条「该模板已被 N 个 NPC 引用，字段勾选与必填配置不可修改」
- `TemplateFieldPicker :disabled="isLocked"` → 所有 cell 不可点，`picker-card.disabled` 整体 `opacity: 0.55 + pointer-events: none`
- `TemplateSelectedFields :disabled="isLocked"` → 必填 checkbox + 上下移动按钮全部禁用
- `TemplateRefPopover` 内部 `:readonly="disabled"` → 只读浏览，所有 checkbox 禁用、全选按钮不显示、「确定」按钮替换为「关闭」

---

## reference popover：读 `refs` 而非 `ref_fields`

**这是 spec 实现期的核心踩坑，值得单独说明**。

后端 `properties.constraints` 的持久化格式是 `refs: number[]`（纯 ID 数组），`FieldForm.vue` 的 `ref_fields: [{id,name,label,type}]` **只是该表单组件的 UI 本地状态**（load 时从 `refs` 转入、submit 前转回 `refs`）。

`TemplateRefPopover.vue` 首版假设 `fieldApi.detail(refField.id)` 返回 `ref_fields`，结果 popover 永远空白。修复是读 `refs` 后并发 `Promise.all` 调 `fieldApi.detail(subId)` 拿每个子字段的 `name / label / type` 元数据。reference 禁嵌套保证子字段必是 leaf 且数量通常 < 10，N+1 代价可接受。

**红线**：见 `docs/standards/frontend-red-lines.md` 「禁止在非表单组件里假设字段 detail 返回富对象」。

---

## 停用字段的视觉标注

在「已选字段配置卡」（`TemplateSelectedFields.vue`）中：

- `row.enabled === false` → `:row-class-name` 返回 `row-field-disabled`，CSS `:deep(.row-field-disabled) { opacity: 0.55 }`
- 标签列左侧加 `<el-icon class="warn-icon"><WarningFilled /></el-icon>`（橙色 `#E6A23C`）+ 「已停用」`el-tag type="warning"`

**停用字段不会出现在「字段选择卡」**——picker 只渲染 `fieldPool`（`enabled=true`），停用字段只通过「已选字段配置卡」从 `template.fields` 元数据源进入视图，保留现有引用关系（存量不动，增量拦截）。

---

## 字段分类分组

`TemplateFieldPicker` 的分组标题用 `fieldPool[*].category_label`（后端 Service 层用 `DictCache` 翻译后返回），**不调用字典 API**、**不硬编码**中文文案。这是 `features.md` 功能 2 的强制约束（字段管理新增 category 时模板管理无需改前端）。

分组顺序按 `fieldPool` 中第一次出现 category 的顺序。因为 pool 是 id ASC 排序，所以基础属性（最先创建）通常最先出现。

---

## 排序按钮视觉（mockup `oE1Hj` / `ylI4t`）

`TemplateSelectedFields` 的「排序」列不用 `el-button text` 包 Unicode `↑↓`，而是用纯 `el-icon` + `ArrowUp` / `ArrowDown`：

- 禁用态 `#C0C4CC` 灰（首行 ↑ / 末行 ↓ / `disabled` 状态）
- 可点态 `#409EFF` 蓝
- hover 浅蓝底 `#ECF5FF`
- 两个图标 `gap: 14`，包裹 `sort-btn` 宽高 22 居中

见 `docs/architecture/ui-red-lines.md` 「禁止表格排序按钮用 el-button text + Unicode 箭头」。

---

## 启用守卫弹窗（字段 / 模板共用）

`EnabledGuardDialog.vue` 是**泛型组件**，通过 `entityType: 'field' | 'template'` 切换文案与 API 调度：

```ts
guardRef.value?.open({
  action: 'edit' | 'delete',
  entityType: 'template',
  entity: row,   // { id, name, label, ref_count }
})
```

**视觉基线**（对齐 mockup `5aRMF` / `ka8Xu`）：

- Header: 24×24 橙色圆角小图标（`#FDF6EC` 底 + `#E6A23C` `WarningFilled`）+ 16/600 加粗标题
- Body: 加粗 lead 句（14/500） + 灰色 reason 段（13/normal/1.6）+ 灰底 `#F5F7FA` 边框步骤 / 条件区
- Footer: 「知道了」outline + 「立即停用」橙底主按钮（`SwitchButton` 图标）

**行为约定**：

- `edit` 场景：「立即停用」先 `detail` 拉 version → `toggleEnabled(id, false, version)` → `router.push('/templates/${id}/edit')`
- `delete` 场景：停用后 emit `refresh` 让父组件 refetch 列表，**不自动触发删除**（避免连锁误操作）
- 41011 版本冲突：warning toast + emit refresh + 关闭弹窗

**红线**：所有危险操作守卫必须走同一个 `EnabledGuardDialog`，禁止每个列表页用 `ElMessageBox.alert` 简陋提示各自重复写一份。见 `docs/architecture/ui-red-lines.md` 「禁止危险操作引导不一致」。

---

## 错误码处理

| 错误码 | 处理 |
|---|---|
| 41001 `NAME_EXISTS` / 41002 `NAME_INVALID` | `nameStatus='taken'` + 红字提示下方，不跳转 |
| 41003 `NOT_FOUND` | `ElMessage.error` + `router.push('/templates')` |
| 41004 `NO_FIELDS` | 提交前已前端拦截（兜底） |
| 41005 `FIELD_DISABLED` / 41006 `FIELD_NOT_FOUND` | `ElMessage.error` + `reloadFieldPool()` 重拉字段池 |
| 41007 `REF_DELETE` | 列表删除路径自动打开 `TemplateReferencesDialog` |
| 41008 `REF_EDIT_FIELDS` | UI 已禁用字段变更（`ref_count > 0` 锁定）理论到不了，兜底走拦截器默认 toast |
| 41009 `DELETE_NOT_DISABLED` / 41010 `EDIT_NOT_DISABLED` | 列表 `EnabledGuardDialog` 前置拦截，理论到不了 |
| 41011 `VERSION_CONFLICT` | `ElMessageBox.alert` → 确认后 `router.push('/templates')` |
| **41012 `FIELD_IS_REFERENCE`** | **兜底**：`ElMessage.error('reference 字段必须先展开子字段再加入模板')` + `reloadFieldPool()`（前端 picker 本就不把 reference 写入 `req.fields`，理论到不了，但留一条兜底分支）|

---

## 手动 e2e 验收

见 `docs/specs/field-template-frontend-sync/design.md` 「测试策略 / 手动 e2e」16 步脚本（含字段池创建、reference 字段展开、停用字段标灰、41012 devtools 兜底等完整场景）。
