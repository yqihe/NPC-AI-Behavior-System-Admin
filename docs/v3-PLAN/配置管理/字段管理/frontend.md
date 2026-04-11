# 字段管理 — 前端设计

> **实现状态**：已全部落地，Vue 3.5 + TypeScript strict + Element Plus + Vite。
> 代码位于 `frontend/src/{views,components,api}/`。
> 通用前端规范 / 陷阱 / 红线见 `docs/development/frontend-pitfalls.md` 与 `docs/standards/frontend-red-lines.md`。
> 本文档只记录字段管理特有的 UI 设计与实现事实，不重复通用规范。

---

## 文件清单

```
frontend/src/
├── api/
│   ├── fields.ts                        # 8 个 API 函数 + FIELD_ERR 常量表（40001-40017）+ 全部类型定义
│   ├── dictionaries.ts                  # DictionaryItem 列表接口（field_type / field_category）
│   └── request.ts                       # axios 实例 + 响应拦截器 + BizError 语义
├── views/
│   ├── FieldList.vue                    # 列表页：筛选 / 分页 / toggle / 编辑删除守卫 / 引用详情
│   └── FieldForm.vue                    # 新建/编辑共用：类型切换 / 约束面板动态加载 / refs↔ref_fields 转换
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

## 组件树

```
FieldList.vue
  ├─ EnabledGuardDialog.vue        (启用守卫，与模板管理共用同一组件)
  └─ el-dialog 内嵌引用详情        (templates + fields 两个表格)

FieldForm.vue
  ├─ FieldConstraintInteger.vue    (按 form.type 动态加载)
  ├─ FieldConstraintString.vue
  ├─ FieldConstraintSelect.vue
  └─ FieldConstraintReference.vue
```

依赖方向单向向下：`views → components → api → request`；components 之间通过 `v-model` / `update:modelValue` 传递，不互相 import。

---

## 类型契约（与后端对齐）

```ts
interface FieldListItem {
  id: number
  name: string
  label: string
  type: string
  category: string
  ref_count: number
  enabled: boolean
  created_at: string
  type_label: string       // 由 DictCache 翻译，前端直接渲染
  category_label: string   // 同上
  version: number          // 列表接口实际不返回（见 FieldList 的 toggle 先 detail 模式）
}

interface FieldProperties {
  description?: string
  expose_bb: boolean
  default_value?: unknown
  constraints?: Record<string, unknown>   // 无 schema RawMessage，前端按 type 解释
}

interface FieldDetail {
  id, name, label, type, category
  properties: FieldProperties
  ref_count, enabled, version
  created_at, updated_at
}

const FIELD_ERR = { NAME_EXISTS: 40001, ..., REF_NESTED: 40016, REF_EMPTY: 40017 } as const
```

**约束 key 单一权威**：所有 `FieldConstraint*.vue` 写入的 `constraints[key]` 必须严格对齐后端 seed `constraint_schema`（见 `backend.md` 约束 key 契约表）。前端改 key 名会让后端 `checkConstraintTightened` **静默失效**。

---

## FieldList 交互

### 筛选栏

| 控件 | 类型 | API 参数 | 选项来源 |
|---|---|---|---|
| 中文标签 | `el-input` | `label` (模糊) | — |
| 字段类型 | `el-select` | `type` | `dictApi.list('field_type')` 启动时拉一次 |
| 标签分类 | `el-select` | `category` | `dictApi.list('field_category')` 同上 |
| 状态 | `el-select` | `enabled` (三态) | 固定 `[启用, 停用]`，`null` = 全部 |

### 表格列

| 列 | 字段 | 渲染 |
|---|---|---|
| ID | `id` | 纯文本 |
| 标识符 | `name` | 加粗文本 |
| 中文标签 | `label` | 纯文本 |
| 类型 | `type_label` | `el-tag`，颜色由类型映射（reference 用 danger 红）|
| 分类 | `category_label` | `el-tag` type=info 灰 |
| 被引用数 | `ref_count` | 蓝色 `el-link`，点击拉引用详情弹窗（value=0 时灰色不可点）|
| 启用 | `enabled` | `el-switch` |
| 创建时间 | `created_at` | `YYYY-MM-DD HH:mm:ss` |
| 操作 | — | `编辑` `删除` 文字链接 |

**停用行视觉**：`:row-class-name` 返回 `row-disabled`，CSS `:deep(.row-disabled td:not(:nth-last-child(-n+3)))` 让除 启用/创建时间/操作 三列外整行 `opacity: 0.5`，保证操作列高亮可点。

### 行交互

| 操作 | 逻辑 |
|---|---|
| **toggle 启用** | `ElMessageBox.confirm` → **先 `fieldApi.detail(id)` 拿最新 version**（列表接口不返回）→ `toggleEnabled(id, val, version)` → 成功 refetch；40010 版本冲突 `ElMessageBox.alert` 提示刷新 |
| **编辑** | `row.enabled === true` → `guardRef.open({action:'edit', entityType:'field', entity:row})`（**前端拦截，不发请求**）；已停用直接 `router.push('/fields/${id}/edit')` |
| **删除** | `row.enabled === true` → 守卫弹窗同上；已停用且 `ref_count > 0` → 自动打开引用详情 + warning toast；已停用且 `ref_count === 0` → `ElMessageBox.confirm` → `delete API` → 成功 refetch；捕获 40005 时自动打开引用详情（兜底 race condition） |
| **被引用数链接** | `ref_count > 0` 时打开引用详情对话框，渲染 `templates[]` + `fields[]` 两张表格（后者是 reference 类型字段引用它时的反向关系） |

### 引用详情弹窗

- 调 `fieldApi.references(id)` 拿 `ReferenceDetail { templates, fields }`
- 两个 `el-table`：
  - **模板引用**：展示 `label` / ref_type（后端 handler 跨模块调 `templateService.GetByIDsLite` 补齐 label）
  - **字段引用**：展示 reference 类型字段的 `label`
- 任一为空时显示 `<p class="ref-empty">暂无XX引用</p>`

---

## FieldForm 状态流

### 核心状态

```ts
const form = reactive({
  name, label, type, category,
  properties: {
    description: '',
    expose_bb: false,
    default_value: null as unknown,
    constraints: {} as Record<string, unknown>,
  },
})
const version = ref(1)
const refCount = ref(0)
const nameStatus = ref<'' | 'checking' | 'available' | 'taken'>('')
```

### 加载编辑页：`loadFieldDetail`

1. `fieldApi.detail(id)` 拿 `FieldDetail`
2. 基本字段回填 `form.{name,label,type,category}`
3. `form.properties.{description,expose_bb,default_value}` 回填
4. **reference 类型的关键转换**：后端 `constraints.refs: number[]` → UI `constraints.ref_fields: [{id,name,label,type,type_label}]`（对每个 refID 调 `fieldApi.detail` 拿元数据，失败则 fallback `{id, name: 'field_${id}', label: '字段${id}', type: 'unknown'}`）
5. 非 reference 类型：`constraints` 原样回填
6. `version.value = data.version`，`refCount.value = data.ref_count`

### 提交：`buildSubmitProperties` + `handleSubmit`

```ts
function buildSubmitProperties() {
  const props = { ...form.properties, constraints: { ...form.properties.constraints } }
  if (form.type === 'reference') {
    // UI 富对象 ref_fields → 后端 refs: number[]
    const refFields = props.constraints.ref_fields as Array<{id:number}> | undefined
    if (refFields) {
      props.constraints.refs = refFields.map(f => f.id)
      delete props.constraints.ref_fields   // 不发给后端
    }
  }
  return props
}
```

**这是 `ref_fields` ↔ `refs` 双向转换的唯一边界**。其他任何组件读字段 detail 时**必须读 `refs`**（后端权威），绝不能假设 API 返回 `ref_fields`（见 `frontend-pitfalls.md` 的明文红线）。

### 类型切换：`handleTypeChange`

类型变更时清空 `form.properties.constraints = {}` + `form.properties.default_value = null`，防止旧类型的约束数据污染新类型的面板（例如从 integer 切到 string 后残留 `min/max`）。

### 唯一性校验：`checkNameUnique`

blur 触发，仅 create 模式有效：

```
'' → 'checking' → 'available' | 'taken'
```

失败时 `nameStatus='taken'` + `nameMessage` 红字提示下方。

### 错误码处理：`handleSubmit.catch`

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
  // 其他错误走 request 拦截器默认 toast
}
```

> 40015（启用中禁止编辑）不会走到 form 这一层——列表的 `EnabledGuardDialog` 已经前端拦截。40006/40007（被引用禁改类型/收紧约束）和 40011（不存在）走默认 toast。

---

## FieldConstraintReference.vue 特殊性

### 双重过滤

`loadEnabledFields()` 从 `fieldApi.list({enabled: true, page_size: 1000})` 拿到全部启用字段后**追加一道过滤**：

```ts
enabledFields.value = (res.data?.items || []).filter(f => f.type !== 'reference')
```

用户看到的下拉列表永远不包含其他 reference 字段，和后端 `validateReferenceRefs` 的 40016 形成**双重防御**（前端过滤做 UX，后端兜底做真校验）。

### 富对象列表模式

内部状态：`modelValue.ref_fields: RefFieldItem[]`，每项带 `{id, name, label, type, type_label}` 供展示。

```
添加引用 → el-select 从 availableFields（enabled 字段 - 自身 - 已选）选一个
         → 深拷贝该字段的元数据到 ref_fields 尾部
         → emit('update:modelValue', { ...constraints, ref_fields: newRefFields })

移除引用 → splice 并 emit
```

`availableFields` 计算属性：排除 `currentFieldId`（防自引用）+ 排除已选 ID（防重复）+ 已经过类型过滤（只有 leaf）。

> **只有这个组件持有富对象格式**。提交时 `FieldForm.buildSubmitProperties` 转回 `refs: number[]`。

---

## 错误码前端映射

`api/fields.ts` 的 `FIELD_ERR` 常量表（40001-40017）是所有 catch 分支的引用源。常量表从 `backend/internal/errcode/codes.go` 逐行复制，避免硬编码数字。手工校对一致性，不做自动生成。

---

## 与 EnabledGuardDialog 的集成

字段和模板共用一个 `EnabledGuardDialog` 组件（`components/EnabledGuardDialog.vue`），通过 `entityType` 参数切换 API 调度与文案：

```ts
guardRef.value?.open({
  action: 'edit' | 'delete',
  entityType: 'field',
  entity: row,   // { id, name, label, ref_count }
})
```

组件内部：

- `edit` 场景：`立即停用` 按钮先 `fieldApi.detail(id)` 拿 version，再 `fieldApi.toggleEnabled(id, false, version)`，成功后 `router.push('/fields/${id}/edit')`
- `delete` 场景：停用后 emit `refresh` 让父组件 refetch 列表，**不自动触发删除**（避免连锁误操作）
- 40010 版本冲突：warning toast + emit refresh + 关闭弹窗

视觉统一：橙色 24×24 圆角图标 header + 加粗 lead + 灰色 reason + 灰底 `#F5F7FA` 步骤/条件区 + 「知道了」outline + 「立即停用」橙底主按钮（SwitchButton 图标）。详见 `docs/architecture/ui-red-lines.md` 「禁止危险操作引导不一致」红线。

---

## 字段选择卡复用（给模板管理）

字段管理本身不需要字段选择卡。但 `FieldConstraintReference` 的"从启用字段里多选"模式和模板管理的 `TemplateFieldPicker` 是同类问题的两种解法：

- `FieldConstraintReference`：单个 `el-select` + 添加列表，适合 reference 字段少量（1-10 个）选择
- `TemplateFieldPicker`：按 category 分组的 3 列网格 + popover，适合模板选 20+ 字段

模板的实现见 `../模板管理/frontend.md`。

---

## 详细 UI 描述

每个页面 / 组件的完整 mockup 对照（字段顺序、间距、颜色）见同目录下 `features.md` 的"功能 X"章节。本文档只覆盖实现事实与状态流。
