# 字段管理 — 前端设计

> **实现状态**：已全部落地，Vue 3.5 + TypeScript strict + Element Plus + Vite。
> 通用前端规范见 `docs/development/standards/red-lines/frontend.md` 和 `dev-rules/frontend.md`。

---

## 1. 目录结构

```
frontend/src/
├── api/
│   ├── fields.ts                        # 8 个 API 函数 + FIELD_ERR 常量表（40001-40017）+ 全部类型定义
│   ├── dictionaries.ts                  # DictionaryItem 列表接口（field_type / field_category）
│   └── request.ts                       # axios 实例 + 响应拦截器 + BizError 语义
├── views/
│   ├── FieldList.vue                    # 列表页：筛选 / 分页 / toggle / 编辑删除守卫 / 引用详情
│   └── FieldForm.vue                    # 新建/编辑共用：类型切换 / 约束面板动态加载 / refs<->ref_fields 转换
├── components/
│   ├── FieldConstraintInteger.vue       # integer / float 共用：min / max / step / precision
│   ├── FieldConstraintString.vue        # string：minLength / maxLength / pattern 正则
│   ├── FieldConstraintSelect.vue        # select：options + minSelect / maxSelect
│   ├── FieldConstraintReference.vue     # reference：ref_fields 富对象列表 + 过滤掉 reference 类型
│   ├── EnabledGuardDialog.vue           # 启用守卫（字段 + 模板共用，entityType: 'field' | 'template'）
│   └── AppLayout.vue                    # 侧栏（el-sub-menu 可折叠分组）+ router-view
└── router/index.ts                      # /fields / /fields/create / /fields/:id/edit
```

---

## 2. 页面路由

| 路径 | 组件 | route name | route meta |
|---|---|---|---|
| `/fields` | `FieldList.vue` | `field-list` | `{ title: '字段管理' }` |
| `/fields/create` | `FieldForm.vue` | `field-create` | `{ title: '新建字段', isCreate: true }` |
| `/fields/:id/edit` | `FieldForm.vue` | `field-edit` | `{ title: '编辑字段', isCreate: false }` |

`FieldForm.vue` 通过 `route.meta.isCreate` 区分新建/编辑模式。`/` 重定向到 `/fields`。

---

## 3. 组件树

```
AppLayout.vue
  └─ <router-view>

FieldList.vue
  ├─ EnabledGuardDialog.vue         (启用守卫，与模板管理共用同一组件)
  └─ el-dialog 内嵌引用详情         (templates + fields 两个 el-table)

FieldForm.vue
  ├─ FieldConstraintInteger.vue     (type === 'integer' || 'float' 时动态加载)
  ├─ FieldConstraintString.vue      (type === 'string')
  ├─ FieldConstraintSelect.vue      (type === 'select')
  └─ FieldConstraintReference.vue   (type === 'reference')
```

依赖方向单向向下：`views -> components -> api -> request`；Constraint 组件之间通过 `v-model` / `update:modelValue` 传递，不互相 import。

---

## 4. 类型契约

```ts
// --- api/fields.ts ---

interface FieldListQuery {
  label?: string
  type?: string
  category?: string
  enabled?: boolean | null
  page: number
  page_size: number
}

interface FieldListItem {
  id: number
  name: string
  label: string
  type: string
  category: string
  ref_count: number
  enabled: boolean
  created_at: string
  type_label: string       // 后端 DictCache 翻译后返回
  category_label: string   // 同上
  version: number
}

interface FieldProperties {
  description?: string
  expose_bb: boolean
  default_value?: unknown
  constraints?: Record<string, unknown>
}

interface FieldDetail {
  id: number
  name: string
  label: string
  type: string
  category: string
  properties: FieldProperties
  ref_count: number
  enabled: boolean
  version: number
  created_at: string
  updated_at: string
}

interface ListData<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}

interface ReferenceItem {
  ref_type: string
  ref_id: number
  label: string
}

interface ReferenceDetail {
  field_id: number
  field_label: string
  templates: ReferenceItem[]
  fields: ReferenceItem[]
}

interface CheckNameResult {
  available: boolean
  message: string
}

const FIELD_ERR = {
  NAME_EXISTS: 40001, NAME_INVALID: 40002, TYPE_NOT_FOUND: 40003,
  CATEGORY_NOT_FOUND: 40004, REF_DELETE: 40005, REF_CHANGE_TYPE: 40006,
  REF_TIGHTEN: 40007, BB_KEY_IN_USE: 40008, CYCLIC_REF: 40009,
  VERSION_CONFLICT: 40010, NOT_FOUND: 40011, DELETE_NOT_DISABLED: 40012,
  REF_DISABLED: 40013, REF_NOT_FOUND: 40014, EDIT_NOT_DISABLED: 40015,
  REF_NESTED: 40016, REF_EMPTY: 40017,
} as const
```

**约束 key 单一权威**：所有 `FieldConstraint*.vue` 写入的 `constraints[key]` 必须严格对齐后端 seed `constraint_schema`。前端改 key 名会让后端 `checkConstraintTightened` 静默失效。

**`ref_fields` <-> `refs` 转换边界**：`FieldForm.vue` 是唯一转换点。加载时 `refs: number[]` -> `ref_fields: [{id,name,label,type}]`；提交时反转回 `refs`。其他组件读字段 detail 必须读 `refs`（后端权威）。

---

## 5. API 调用映射

| UI 操作 | API 函数 | 后端端点 |
|---|---|---|
| 列表加载 / 筛选 / 翻页 | `fieldApi.list(params)` | `POST /api/v1/fields/list` |
| 新建字段 | `fieldApi.create(data)` | `POST /api/v1/fields/create` |
| 查看详情（编辑页加载 / toggle 取 version） | `fieldApi.detail(id)` | `POST /api/v1/fields/detail` |
| 编辑字段 | `fieldApi.update(data)` | `POST /api/v1/fields/update` |
| 删除字段 | `fieldApi.delete(id)` | `POST /api/v1/fields/delete` |
| 标识符唯一性校验（blur） | `fieldApi.checkName(name)` | `POST /api/v1/fields/check-name` |
| 切换启用/停用 | `fieldApi.toggleEnabled(id, enabled, version)` | `POST /api/v1/fields/toggle-enabled` |
| 引用详情弹窗 | `fieldApi.references(id)` | `POST /api/v1/fields/references` |
| 字典下拉（类型/分类） | `dictApi.list('field_type')` / `dictApi.list('field_category')` | `POST /api/v1/dictionaries/list` |

---

## 6. 错误码处理

| 错误码 | 常量名 | UI 反馈 |
|---|---|---|
| 40001 | `NAME_EXISTS` | form 内联红字：`nameStatus='taken'` + `nameMessage` |
| 40002 | `NAME_INVALID` | form 内联红字：同上 |
| 40005 | `REF_DELETE` | 列表删除时自动打开引用详情弹窗（兜底 race condition） |
| 40010 | `VERSION_CONFLICT` | `ElMessageBox.alert` 提示"数据已被其他用户修改，请刷新" + `fetchList()` 刷新列表 |
| 40011 | `NOT_FOUND` | 编辑页加载失败时 `router.push('/fields')` |
| 40015 | `EDIT_NOT_DISABLED` | 列表 `EnabledGuardDialog` 前端拦截，理论不会到 form |
| 40016 | `REF_NESTED` | `ElMessage.error('不能引用 reference 类型字段')` |
| 40017 | `REF_EMPTY` | `ElMessage.error('reference 字段必须至少选择一个目标字段')` |
| 40006 | `REF_CHANGE_TYPE` | `ElMessage.error` 提示被引用字段不允许改类型 |
| 40007 | `REF_TIGHTEN` | `ElMessage.error` 提示约束收紧会影响引用方 |
| 40009 | `CYCLIC_REF` | `ElMessage.error` 提示循环引用 |
| 40013 | `REF_DISABLED` | `ElMessage.error` 提示引用的目标字段已停用 |
| 40014 | `REF_NOT_FOUND` | `ElMessage.error` 提示引用的目标字段不存在 |
| 其他 | — | 走 request 拦截器默认 toast |

---

## 7. 关键实现细节

### 约束组件 `validate()` 模式

所有约束组件（`FieldConstraintInteger`、`FieldConstraintString`、`FieldConstraintSelect`）通过 `defineExpose({ validate })` 暴露校验方法。`FieldForm.vue` 持有 `constraintRef`（模板引用），提交前调用 `constraintRef.value?.validate()` 进行约束级校验（如 min ≤ max、minLength ≤ maxLength 等）。校验失败返回 `false` 并内联提示，不走后端。

### View 模式 disabled 修复

`FieldForm.vue` 字段类型下拉的 disabled 条件为 `isView || (!isCreate && refCount > 0)`，而非仅 `!isCreate && refCount > 0`。原因：Element Plus 的 `el-select` 在 `disabled` 为 `undefined ?? false` 时会覆盖为 `false`，导致查看模式下类型下拉可操作。显式加 `isView ||` 前缀确保查看模式始终禁用。

### 约束组件 `disabled` prop

`FieldConstraintSelect` 和 `FieldConstraintReference` 接受 `disabled` prop，在查看模式下传入 `true`，禁用所有内部控件（选项列表、添加/删除按钮等）。
