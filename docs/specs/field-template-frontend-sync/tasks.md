# field-template-frontend-sync — 任务拆解

13 个原子任务。FA 三个任务先做（小、独立、收尾快），然后 TA 按依赖顺序：API 层 → 独立子组件 → 大组件 → 列表/表单页 → 路由+菜单挂载 → 文档。

**每个任务完成后强制**：

1. `cd frontend && npx vue-tsc --noEmit` — **必须零错误**（memory: `feedback_vue_tsc_required`）
2. `cd frontend && npm run build` — 必须通过
3. 浏览器 smoke：必须能看到任务的实际效果（「做完是什么样」里列出）
4. `git commit` 当前改动（commit message 格式 `feat(frontend/<scope>): T<N> <短描述>`）
5. 出现 vue-tsc 错误或 smoke 失败 → **立刻停下排查**，不进入下一个 T

---

## Part FA — 字段管理对齐

### T1：`api/fields.ts` 追加 `FIELD_ERR` 常量表（FA 前置）

**文件**：

- `frontend/src/api/fields.ts`（唯一）

**改动**：在文件末尾追加：

```ts
// 字段管理段错误码（40001-40017，与 backend/internal/errcode/codes.go 保持一致）
export const FIELD_ERR = {
  NAME_EXISTS:         40001,
  NAME_INVALID:        40002,
  TYPE_NOT_FOUND:      40003,
  CATEGORY_NOT_FOUND:  40004,
  REF_DELETE:          40005,
  REF_CHANGE_TYPE:     40006,
  REF_TIGHTEN:         40007,
  BB_KEY_IN_USE:       40008,
  CYCLIC_REF:          40009,
  VERSION_CONFLICT:    40010,
  NOT_FOUND:           40011,
  DELETE_NOT_DISABLED: 40012,
  REF_DISABLED:        40013,
  REF_NOT_FOUND:       40014,
  EDIT_NOT_DISABLED:   40015,
  REF_NESTED:          40016,
  REF_EMPTY:           40017,
} as const
```

**做完是什么样**：

- `vue-tsc / build` 通过
- 在浏览器 console 里 `import('./api/fields').then((m) => console.log(m.FIELD_ERR.REF_NESTED))` 输出 `40016`

**依赖**：无

---

### T2：`FieldConstraintReference.vue` 过滤 reference 类型字段（R1 → FA-1）

**文件**：

- `frontend/src/components/FieldConstraintReference.vue`（唯一）

**改动**：找到 `loadEnabledFields` 函数，在拿到接口结果后**追加一道** `f.type !== 'reference'` 过滤：

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

**做完是什么样**：

- `vue-tsc / build` 通过
- 浏览器进入 `/fields/create` → 选 `type=reference` → 点「添加引用」→ 下拉里**看不到**已有的 reference 字段（如果 seed 里没有可先去创建一个 test reference 字段并启用）
- 普通字段（integer / float / string / boolean / select）正常出现

**依赖**：无

---

### T3：`FieldForm.vue` 捕获 40016 / 40017 给定向提示（R2 / R3 → FA-2 / FA-3）

**文件**：

- `frontend/src/views/FieldForm.vue`（唯一）

**改动**：

1. `import { fieldApi, FIELD_ERR } from '@/api/fields'`（保留原 `fieldApi` 导入，追加 `FIELD_ERR`）
2. 在 `handleSubmit` 的 catch 里，把现有的 `bizErr.code === 40010` / `40001` / `40002` 替换为 `FIELD_ERR.VERSION_CONFLICT` / `FIELD_ERR.NAME_EXISTS` / `FIELD_ERR.NAME_INVALID` 常量写法，并追加两个新分支：

```ts
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

**做完是什么样**：

- `vue-tsc / build` 通过
- 用 devtools 直接发一个 `refs=[]` 的 reference 字段 create 请求 → 前端弹 `ElMessage "reference 字段必须至少选择一个目标字段"`
- 用 devtools 发一个 `refs` 包含另一个 reference 的 update → 弹 `ElMessage "不能引用 reference 类型字段..."`
- 不破坏现有 40010 / 40001 / 40002 错误处理

**依赖**：T1（需要 `FIELD_ERR` 常量）

---

## Part TA — 模板管理 0→1

### T4：`api/templates.ts` 完整 API 层（R24）

**文件**：

- `frontend/src/api/templates.ts`（新建，唯一）

**改动**：新建文件，包含四段：

1. **类型定义**：`TemplateFieldEntry` / `TemplateListItem` / `TemplateFieldItem` / `TemplateDetail` / `TemplateListQuery` / `CreateTemplateRequest` / `UpdateTemplateRequest` / `TemplateReferenceItem` / `TemplateReferenceDetail`
2. **错误码常量** `TEMPLATE_ERR`（41001-41012，**包含 41012 `FIELD_IS_REFERENCE`**）
3. **错误码文案映射** `TEMPLATE_ERR_MSG: Record<number, string>`（12 条中文文案，对齐 `backend/internal/errcode/codes.go` 的 `errMsgs`）
4. **8 个 API 函数** `templateApi.{list, create, detail, update, delete, checkName, references, toggleEnabled}`

每个类型字段名严格对齐 `backend/internal/model/template.go` 的 JSON tag（snake_case）。API 函数复用 `./request` 的 axios 实例。

**做完是什么样**：

- `vue-tsc / build` 通过
- 浏览器 devtools 里手动 `import('@/api/templates').then((m) => m.templateApi.list({ page: 1, page_size: 20 }))` 能拉到数据（前提是后端有数据，否则空数组）
- 类型导入 / 导出无报错

**依赖**：无（依赖现有 `api/request.ts`）

---

### T5：`TemplateReferencesDialog.vue` 引用详情弹窗（R9）

**文件**：

- `frontend/src/components/TemplateReferencesDialog.vue`（新建，唯一）

**改动**：

- 新建组件，`defineExpose({ open(template: TemplateListItem) })`
- 模板：`el-dialog` 包含 header（`template_label` + 总数）+ `el-table` 或 `el-empty`
- 内部 `open` 时调 `templateApi.references(template.id)` 拿数据
- `@close` 时清 `data.value = null`

**做完是什么样**：

- `vue-tsc / build` 通过
- 暂时挂到任一现有页面做手工测试：`open` 一个模板能弹窗，后端返回 `npcs: []` 时显示「暂无 NPC 引用」占位

**依赖**：T4

---

### T6：`EnabledGuardDialog.vue` 启用守卫弹窗（R10 / R11）

**文件**：

- `frontend/src/components/EnabledGuardDialog.vue`（新建，唯一）

**改动**：

- 新建组件，`defineExpose({ open(action: 'edit' | 'delete', template: TemplateListItem) })`
- 模板：`el-dialog`（橙色警告 icon + title + 原因文本 + 操作步骤区 + 知道了 / 立即停用 两个按钮）
- 文案按 features.md 功能 9：
  - `edit`：标题「无法编辑模板」，正文解释启用中对 NPC 管理页可见的风险
  - `delete`：标题「无法删除模板」，删除前置条件区列出 ✗ 模板已停用 / ✓ 没有 NPC 在使用
- 「立即停用」按钮：
  - `edit`：调 `templateApi.toggleEnabled(id, false, version)` → 成功后 `router.push('/templates/${id}/edit')`
  - `delete`：调 `toggleEnabled(false)` → 关闭对话框 + emit `refresh` 事件让父组件刷新列表（**不自动触发删除**）
- emit：`refresh: []`

**做完是什么样**：

- `vue-tsc / build` 通过
- 组件本身可独立挂载测试
- 文案符合 mockup（橙色警告、操作步骤区、按钮）

**依赖**：T4

---

### T7：`TemplateRefPopover.vue` reference 子字段勾选弹层（R15 / R16）

**文件**：

- `frontend/src/components/TemplateRefPopover.vue`（新建，唯一）

**改动**：

- 新建组件，`defineExpose({ open(refField: FieldListItem, currentSelectedIds: number[]) })`
- 模板：`el-dialog` 包含：
  - 标题：reference 字段的中文标签 + 标识 + reference 紫色徽章
  - 蓝色信息条「勾选的子字段会扁平地写入模板，与其他来源去重」
  - 工具栏：左侧「子字段 (N)」计数，右侧 `全选` `全不选` 两个快捷按钮
  - 子字段列表：每行 checkbox + 字段标签 + 字段标识 + 类型徽章
  - 底部：左侧「已选 X / N」计数 + 右侧 `取消` `确定` 按钮
- 内部状态：
  - `tempSelected = ref<number[]>([])` ——**不直接改父组件的 selectedIds**
  - `subFields = ref<RefFieldItem[]>([])`
- 数据加载：`open` 时调 `fieldApi.detail(refField.id)` 拿 `properties.constraints.ref_fields`（富对象数组，带 `id / name / label / type`），直接填 `subFields`，**不**二次调 `fieldApi.list`
- 初始化 `tempSelected`：`currentSelectedIds.filter((id) => subFields.some((f) => f.id === id))`，把外部已选的子字段回勾
- 全选 / 全不选：只操作 `tempSelected`
- 确认按钮 emit：

```ts
emit('confirm', {
  allSubIds: subFields.value.map((f) => f.id),
  selectedSubIds: tempSelected.value.slice(),
})
```

- 取消按钮：直接关闭，不 emit
- 只读 prop：`readonly?: boolean`，为 true 时禁用所有 checkbox 和全选按钮，隐藏「确定」按钮只留「关闭」

**做完是什么样**：

- `vue-tsc / build` 通过
- 单独挂载测试：传一个 reference 字段（需要先在字段管理里创建一个 reference）→ 弹窗显示其子字段 → 勾选 → 确认 → emit 出的 `{allSubIds, selectedSubIds}` 正确
- 取消不 emit；再次打开勾选状态从父组件重新计算

**依赖**：T4（但本组件直接用 `fieldApi` 不依赖 `templateApi`，只是放在 TA 段位执行）

---

### T8：`TemplateSelectedFields.vue` 已选字段配置卡（R17 / R18）

**文件**：

- `frontend/src/components/TemplateSelectedFields.vue`（新建，唯一）

**改动**：

- 新建组件，props：`selectedFields: TemplateFieldItem[]`、`disabled?: boolean`
- emits：
  - `update:order: [number[]]` — 新的 `field_id` 顺序
  - `update:required: [fieldId: number, required: boolean]`
- 模板：`el-table` 5 列（标签 / `name` / 类型 tag / 必填 checkbox / 上下移动）
- **停用字段标灰 + 警告图标**：
  - `:row-class-name` 返回 `row.enabled ? '' : 'row-field-disabled'`
  - 字段标签列在 `row.enabled === false` 时左侧加 `<el-icon class="warn-icon"><WarningFilled /></el-icon>`
  - scoped CSS：`:deep(.row-field-disabled) { opacity: 0.55; }` + `.warn-icon { color: #e6a23c; margin-right: 4px; }`
- 上下移动：纯前端 splice 一份 `field_id` 数组，emit `update:order`
- 必填 checkbox：`@change` → emit `update:required`（不要在组件内修改 `row.required`，保持 props 单向下行）
- 首行 `↑` 灰、末行 `↓` 灰、`disabled` 时整列灰

**做完是什么样**：

- `vue-tsc / build` 通过
- 单独测试：传一个 mock 字段数组（含一个 `enabled=false` 的字段）→ 上下移动能改顺序、必填能勾选、停用字段行整行灰 + 左侧警告图标、`disabled` 时按钮全灰

**依赖**：T4

---

### T9：`TemplateFieldPicker.vue` 简化版字段选择卡（R13 / R14 / R15 / R16）

**文件**：

- `frontend/src/components/TemplateFieldPicker.vue`（新建，唯一）

**改动**：

- 新建组件，使用 `defineModel<number[]>('selectedIds', { required: true })` 双向绑定
- props：`fieldPool: FieldListItem[]`、`disabled?: boolean`
- 内部 `groupedFields` computed：按 `f.category` 分组，**分组标题用 `f.category_label`**（来自字段接口，不硬编码），分组顺序按 `fieldPool` 中第一次出现 category 的顺序
- 模板：`v-for` 分组 → grid `repeat(3, 1fr)` → 每个 cell 用 design.md 的 scoped CSS（普通 cell 普通边框，reference cell 紫色边框 + chevron）
- 点击交互：
  - `disabled` → return
  - `f.type === 'reference'` → `popoverRef.value?.open(f, selectedIds.value)`（不打钩）
  - 否则 → toggle：`selectedIds.value = [...selectedIds.value, f.id]` 或 `selectedIds.value.filter(...)`
- popover 确认 → 收到 `{allSubIds, selectedSubIds}` 后：

```ts
function onPopoverConfirm(payload: { allSubIds: number[]; selectedSubIds: number[] }) {
  const { allSubIds, selectedSubIds } = payload
  // 先从 selectedIds 里移除本 reference 负责的所有子字段
  const withoutSubs = selectedIds.value.filter((id) => !allSubIds.includes(id))
  // 再合并本次勾选的（自动去重）
  selectedIds.value = Array.from(new Set([...withoutSubs, ...selectedSubIds]))
}
```

- 内嵌 `<TemplateRefPopover ref="popoverRef" :readonly="disabled" @confirm="onPopoverConfirm" />`

**做完是什么样**：

- `vue-tsc / build` 通过
- 在简单测试页面挂载，传 mock 数据 → 能看到按字典分组、3 列网格、复选框打钩
- 点击 reference 字段不打钩，弹出 T7 的 popover；确定后外部 `selectedIds` 合并去重
- 再次打开 reference popover 时「已勾选的子字段」自动回勾（通过 `currentSelectedIds` 传递）
- 整体 `disabled` 时所有 cell 不能点击，popover 进只读模式

**依赖**：T4 + T7

---

### T10：`TemplateForm.vue` 新建/编辑共用表单页（R13 / R19-R24）

**文件**：

- `frontend/src/views/TemplateForm.vue`（新建，唯一）

**改动**：

- 新建组件，props `mode: 'create' | 'edit'` + 可选 `id: number`
- 三段式布局：
  1. 基本信息卡（标识 `name` + 中文标签 `label` + 描述 `description` maxlength 512）
  2. 字段选择卡（嵌入 `<TemplateFieldPicker v-model:selectedIds="selectedIds" :field-pool="fieldPool" :disabled="isLocked" />`）
  3. 已选字段配置卡（嵌入 `<TemplateSelectedFields :selected-fields="selectedFieldsView" :disabled="isLocked" @update:order="onOrderChange" @update:required="onRequiredChange" />`）
- 数据流见 design.md TA-6：
  - `selectedIds: Ref<number[]>`（扁平 ID 数组，顺序即 `fields` JSON 顺序）
  - `requiredMap: Ref<Record<number, boolean>>`
  - `selectedFieldsView: ComputedRef<TemplateFieldItem[]>`（编辑模式优先从 `template.fields` 取停用字段元数据）
- 进入页面：
  - create：构造空 state + `fieldApi.list({ enabled: true, page_size: 1000 })` 拉字段池
  - edit：`Promise.all` 并发拉 `templateApi.detail(id)` + `fieldApi.list({ enabled: true, page_size: 1000 })` → 回填 `selectedIds` + `requiredMap`
- `onOrderChange(newOrder)`：`selectedIds.value = newOrder`
- `onRequiredChange(id, required)`：`requiredMap.value = { ...requiredMap.value, [id]: required }`
- **name 唯一性校验**（仅 create 模式）：blur 时调 `templateApi.checkName`，状态机 `idle / checking / available / taken`
- **name 编辑模式**：灰底 + `Lock` 图标 + `readonly` + hint「模板标识创建后不可修改」
- **ref_count > 0 锁定**：`isLocked = mode === 'edit' && (template.value?.ref_count ?? 0) > 0` → 顶部 `<el-alert type="warning">` 黄色警告条 + picker disabled + selectedFields disabled + 卡片标题加 `🔒 已锁定` tag
- **提交**：合并 `selectedIds` + `requiredMap` 成 `fields: TemplateFieldEntry[]` → 调 `create` 或 `update`
- **错误处理**：按 design.md 的错误码表逐条捕获，**必须包含 41012 `FIELD_IS_REFERENCE` 兜底分支**

**做完是什么样**：

- `vue-tsc / build` 通过
- 浏览器手动测试 create 流程：填表 → 勾字段 → 移动顺序 → 标必填 → 保存 → 成功跳列表
- 测试 edit 流程（路由还没注册，可以通过 import 直接挂载或者在 T11 之前临时手写 `/templates/1/edit` 路由试水）
- 41012 兜底：devtools 构造一个 `fields` 里含 reference 字段 ID 的 create 请求 → 弹「reference 字段必须先展开子字段再加入模板」

**依赖**：T4 + T8 + T9

---

### T11：`TemplateList.vue` 列表页（R6-R12）

**文件**：

- `frontend/src/views/TemplateList.vue`（新建，唯一）

**改动**：

- 新建组件，参照 `FieldList.vue` 的视觉系统
- 顶部 page-header（标题 + 新建按钮）
- filter-bar（`label` 模糊搜索 + `enabled` 三态 select + 搜索 / 重置按钮）
- `el-table` 7 列（ID / `name` / `label` / `ref_count` 蓝色 link / `enabled` switch / `created_at` / 操作）
- 底部 `el-pagination`
- 嵌入 `<EnabledGuardDialog ref="guardRef" @refresh="loadList">` 和 `<TemplateReferencesDialog ref="refsRef">`
- 行交互：
  - **编辑**：`row.enabled === true` → `guardRef.value.open('edit', row)`（**不发请求**）；否则 `router.push('/templates/${row.id}/edit')`
  - **删除**：`row.enabled === true` → `guardRef.value.open('delete', row)`；否则 `ElMessageBox.confirm` → `templateApi.delete(row.id)` → 捕获 `TEMPLATE_ERR.REF_DELETE`（41007）时自动调 `refsRef.value.open(row)`
  - **ref_count 链接**：`refsRef.value.open(row)`
  - **enabled switch**：`templateApi.toggleEnabled(row.id, !row.enabled, row.version)`，捕获 41011 时弹冲突提示并重新 `loadList`
- **停用模板整行变灰**：`:row-class-name` 返回 `row-disabled`，操作列单独用 `:cell-class-name` 排除

**做完是什么样**：

- `vue-tsc / build` 通过
- 进入 `/templates`（路由还没挂，可以临时注释 `router.push`，或者先做 T12）→ 列表能拉到模板
- 启用中点编辑 / 删除弹守卫窗
- 已停用点删除走二次确认
- 删除引用中的模板会自动弹引用详情

**依赖**：T4 + T5 + T6

---

### T12：路由注册 + 菜单挂载（R6）

**文件**：

- `frontend/src/router/index.ts`（改）
- `frontend/src/components/AppLayout.vue`（改）

**改动**：

1. `router/index.ts` 追加 3 条路由：

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

2. `AppLayout.vue` 在「字段管理」`el-menu-item` 下方插入：

```vue
<el-menu-item index="/templates" @click="$router.push('/templates')">
  <el-icon><Files /></el-icon>
  <span>模板管理</span>
</el-menu-item>
```

**做完是什么样**：

- `vue-tsc / build` 通过
- 浏览器刷新 `/` → 左侧菜单出现「模板管理」项
- 点击进入 `/templates`，看到 T11 的列表页
- 进入 `/templates/create` 看到 T10 的新建表单
- 进入 `/templates/123/edit`（如果该 id 存在）看到编辑表单

**依赖**：T10 + T11

---

### T13：文档同步

**文件**：

- `docs/v3-PLAN/配置管理/字段管理/features.md`（改）
- `docs/v3-PLAN/配置管理/模板管理/features.md`（改）
- `docs/v3-PLAN/配置管理/模板管理/frontend.md`（改，当前仅一行「待定义」）

**改动**：

1. **字段管理 `features.md`**：在功能 11「reference 字段引用校验」或横切关注点处补一句：「**前端**reference 下拉在 `FieldConstraintReference.vue` 中已排除 `type='reference'` 字段，与后端 40016 形成双重防御」。

2. **模板管理 `features.md`**：把首行「实现状态」从「后端 API 全部实现（8 个接口 + 跨模块事务编排 + 12 个错误码）；前端 UI 实现中」更新为「**后端 + 前端全部实现**（后端 199/199 + 前端 13 个文件 ~2400 行，参见 `docs/specs/field-template-frontend-sync`）」。

3. **模板管理 `frontend.md`**：替换掉「待定义」，写入组件树 + 依赖方向 + 关键状态流的简要描述（直接摘要 design.md TA-总览 + TA-6 核心数据流，不重复完整方案）。

**做完是什么样**：

- 文档与实际代码状态一致
- features.md 实现状态更新到最新
- frontend.md 有实质性内容

**依赖**：T1-T12 全部完成

---

## 执行顺序总览

```
T1 (FIELD_ERR)
  ↓
T2 (filter)        T3 (catch errors)   ← 都依赖 T1
  ↓
T4 (api/templates.ts)
  ↓
T5 (refs dialog)   T6 (guard dialog)   ← 都依赖 T4
  ↓
T7 (ref popover)   T8 (selected)        ← T7/T8 都依赖 T4，可并行
  ↓
T9 (field picker)  ← 依赖 T4 + T7
  ↓
T10 (TemplateForm)  ← 依赖 T4 + T8 + T9
  ↓
T11 (TemplateList)  ← 依赖 T4 + T5 + T6
  ↓
T12 (router + menu)  ← 依赖 T10 + T11
  ↓
T13 (docs sync)
```

实际串行执行顺序：**T1 → T2 → T3 → T4 → T5 → T6 → T7 → T8 → T9 → T10 → T11 → T12 → T13**。

## 每步验证协议

每个 T 完成后：

1. `cd frontend && npx vue-tsc --noEmit` — **必须零错误**
2. `cd frontend && npm run build` — 必须通过
3. **针对该任务的浏览器 smoke**（每个任务在「做完是什么样」里都列了）
4. `git commit` 当前改动（格式 `feat(frontend/<scope>): T<N> <短描述>`）
5. 出现 vue-tsc 错误或 smoke 失败 → **立刻停下排查**，不进入下一个 T

## 终态验收

完成 T13 后达到终态：

- 字段管理的 reference 下拉自动排除 reference 字段（FA-1）
- 字段管理的 40016 / 40017 错误有定向中文提示（FA-2 / FA-3）
- 模板管理菜单可见，**列表 → 新建 → 编辑 → 启停 → 删除 → 被引用守卫 → reference popover → 停用字段警告 → 41012 兜底** 全流程可走
- `tests/api_test.sh` 后端集成测试仍 199/199 通过（不动后端，理论上必然通过）
- 文档与代码完全同步

## 文档同步确认

- [ ] 字段管理 `features.md` 加 reference 下拉过滤的横切说明（T13）
- [ ] 模板管理 `features.md` 实现状态更新（T13）
- [ ] 模板管理 `frontend.md` 填入实质内容（T13）
- [ ] 错误码表与 `api/templates.ts` 的 `TEMPLATE_ERR`（含 41012）完全一致（T4 时双向核对）
- [ ] 无遗漏的跨模块文档（`mockups-template.pen` 不动，`CLAUDE.md` 不动）

---

**Phase 3 完成，停下等待审批**。

审批通过后：

1. 当前在 `feature/template-management-backend` 分支，需要从 `main` 切新 feature 分支：`git checkout main && git pull && git checkout -b feature/field-template-frontend-sync`
2. 开始 `/spec-execute T1 field-template-frontend-sync`
