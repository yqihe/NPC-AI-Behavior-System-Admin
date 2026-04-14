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
│   └── FieldForm.vue                    # 新建/编辑/查看共用：类型切换 / 约束面板动态加载 / refs<->ref_fields 转换
├── components/
│   ├── FieldConstraintInteger.vue       # integer / float 共用：min / max / step / precision
│   ├── FieldConstraintString.vue        # string：minLength / maxLength / pattern 正则
│   ├── FieldConstraintSelect.vue        # select：options + minSelect / maxSelect
│   ├── FieldConstraintReference.vue     # reference：ref_fields 富对象列表 + 过滤掉 reference 类型
│   ├── EnabledGuardDialog.vue           # 启用守卫（字段 / 模板 / 事件类型 / 扩展字段共用）
│   └── AppLayout.vue                    # 侧栏（el-sub-menu 可折叠分组）+ router-view
└── router/index.ts                      # /fields / /fields/create / /fields/:id/edit / /fields/:id/view
```

---

## 2. 页面路由

| 路径 | 组件 | route name | route meta |
|---|---|---|---|
| `/fields` | `FieldList.vue` | `field-list` | `{ title: '字段管理' }` |
| `/fields/create` | `FieldForm.vue` | `field-create` | `{ title: '新建字段', isCreate: true }` |
| `/fields/:id/edit` | `FieldForm.vue` | `field-edit` | `{ title: '编辑字段', isCreate: false }` |
| `/fields/:id/view` | `FieldForm.vue` | `field-view` | `{ title: '查看字段', isCreate: false, isView: true }` |

`FieldForm.vue` 通过 `route.meta.isCreate` 区分新建/编辑模式，通过 `route.meta.isView` 判定只读模式。`/` 重定向到 `/fields`。

---

## 3. 组件树

```
AppLayout.vue
  └─ <router-view>

FieldList.vue
  ├─ EnabledGuardDialog.vue         (启用中字段点「编辑」或「删除」触发)
  └─ el-dialog 内嵌引用详情         (模板引用 / 字段引用 / FSM 引用 三个 el-table)

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

// 列表项不再包含 ref_count（V3 清理：列表页不展示"被引用数"列）
interface FieldListItem {
  id: number
  name: string
  label: string
  type: string
  category: string
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

// 详情用 has_refs: boolean 代替原 ref_count: number
// 前端只关心"有没有引用"，无需具体数字驱动锁定逻辑
interface FieldDetail {
  id: number
  name: string
  label: string
  type: string
  category: string
  properties: FieldProperties
  enabled: boolean
  has_refs: boolean        // 新增：替代 ref_count，布尔语义更清晰
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

// 引用详情新增 fsms（通过 BB Key 间接引用的 FSM）
interface ReferenceDetail {
  field_id: number
  field_label: string
  templates: ReferenceItem[]   // 引用了该字段的模板
  fields: ReferenceItem[]      // 通过 reference 类型引用该字段的其他字段
  fsms: ReferenceItem[]        // 新增：通过 BB Key 追踪到的 FSM 引用
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

**`ref_fields` <-> `refs` 转换边界**：`FieldForm.vue` 是唯一转换点。加载详情时 `refs: number[]` -> `ref_fields: [{id,name,label,type}]`（通过旁路拉取候选字段列表 join label）；提交时反转回 `refs`。其他组件读字段 detail 必须读 `refs`（后端权威），禁止在其他位置重复这层转换。

**`has_refs` 语义**：只要存在任何一类引用（templates / fields / fsms）后端即置 `true`。前端据此锁定「类型」下拉与约束组件可编辑范围，不再关心具体计数。

---

## 5. API 调用映射

| UI 操作 | API 函数 | 后端端点 |
|---|---|---|
| 列表加载 / 筛选 / 翻页 | `fieldApi.list(params)` | `POST /api/v1/fields/list` |
| 新建字段 | `fieldApi.create(data)` | `POST /api/v1/fields/create` |
| 查看详情（编辑页加载 / toggle 取 version / 查看模式） | `fieldApi.detail(id)` | `POST /api/v1/fields/detail` |
| 编辑字段 | `fieldApi.update(data)` | `POST /api/v1/fields/update` |
| 删除字段 | `fieldApi.delete(id)` | `POST /api/v1/fields/delete` |
| 标识符唯一性校验（blur） | `fieldApi.checkName(name)` | `POST /api/v1/fields/check-name` |
| 切换启用/停用 | `fieldApi.toggleEnabled(id, enabled, version)` | `POST /api/v1/fields/toggle-enabled` |
| 引用详情弹窗 / 删除前置检查 | `fieldApi.references(id)` | `POST /api/v1/fields/references` |
| 字典下拉（类型/分类） | `dictApi.list('field_type')` / `dictApi.list('field_category')` | `POST /api/v1/dictionaries/list` |

共 8 个字段 API + 1 个字典 API。`references` 被列表删除流程（前置检查）和后端 REF_DELETE 兜底（race fallback）共享调用。

---

## 6. 错误码处理

| 错误码 | 常量名 | UI 反馈 |
|---|---|---|
| 40001 | `NAME_EXISTS` | form 内联红字：`nameStatus='taken'` + `nameMessage` |
| 40002 | `NAME_INVALID` | form 内联红字：同上 |
| 40003 | `TYPE_NOT_FOUND` | `ElMessage.error` 字段类型不存在；通常字典加载失败才会触发 |
| 40004 | `CATEGORY_NOT_FOUND` | `ElMessage.error` 字段分类不存在 |
| 40005 | `REF_DELETE` | 列表删除时后端兜底：调用 `loadAndShowRefs(row)` 重新拉 references 并打开详情弹窗（解决删除前置检查与后端持久化之间的 race condition） |
| 40006 | `REF_CHANGE_TYPE` | `ElMessage.error` 提示被引用字段不允许改类型；前端已通过 `hasRefs` 锁定类型下拉，此码作为后端兜底 |
| 40007 | `REF_TIGHTEN` | `ElMessage.error` 提示约束收紧会影响引用方；前端已通过 `:restricted="hasRefs"` 锁定收紧类控件，此码作为后端兜底 |
| 40008 | `BB_KEY_IN_USE` | `ElMessage.error` 提示 BB Key 被 FSM/BT 引用，无法变更 expose_bb 或 name |
| 40009 | `CYCLIC_REF` | `ElMessage.error` 提示循环引用（reference 字段选到了间接指回自己的字段） |
| 40010 | `VERSION_CONFLICT` | `ElMessageBox.alert` 提示"数据已被其他用户修改，请刷新" + `fetchList()` 刷新列表；form 内同样兜底 |
| 40011 | `NOT_FOUND` | 编辑/查看页加载失败时 `router.push('/fields')` |
| 40012 | `DELETE_NOT_DISABLED` | 正常前端通过 `EnabledGuardDialog` 拦截，若后端收到说明 race，提示并刷新 |
| 40013 | `REF_DISABLED` | `ElMessage.error` 提示引用的目标字段已停用 |
| 40014 | `REF_NOT_FOUND` | `ElMessage.error` 提示引用的目标字段不存在 |
| 40015 | `EDIT_NOT_DISABLED` | 正常前端通过 `EnabledGuardDialog` 拦截；若后端收到说明 race，提示并回退 |
| 40016 | `REF_NESTED` | `ElMessage.error('不能引用 reference 类型字段')`；`FieldConstraintReference` 已前端过滤 `type !== 'reference'`，此码作为兜底 |
| 40017 | `REF_EMPTY` | `ElMessage.error('reference 字段必须至少选择一个目标字段')`；submit 前约束组件 `validate()` 已拦截 |
| 其他 | — | 走 `request.ts` 拦截器默认 `ElMessage.error(message)` |

`request.ts` 拦截器把后端 `{code, message, data}` 非零 code 封装成 `BizError` throw 出来。页面层用 `(err as BizError).code === FIELD_ERR.XXX` 精准分支；未命中分支的错误保持拦截器默认 toast，不重复提示。

---

## 7. 关键实现细节

### 7.1 列表页「被引用数」列已移除

V3 清理：`FieldListItem` 去掉 `ref_count`，列表表格对应列一并删除。原因：
- 列表页带计数需要后端 list 接口额外跑 N+1 聚合（或 join），QPS 成本高。
- "具体被几处引用"对列表决策并无信息增量，用户需要的是「能不能删」的布尔答案。
- 实际删除决策下沉到点击删除时按需调用 `fieldApi.references(id)`，单条查询代价可接受。

列表列顺序（从左至右）：ID / 字段标识 / 中文标签 / 类型 / 分类 / 启用 / 创建时间 / 操作。

### 7.2 删除流程（列表页 `handleDelete`）

启用状态分两路：

1. **启用中** → `guardRef.open({ action: 'delete', entityType: 'field', entity: row })`，由 `EnabledGuardDialog` 引导先禁用。
2. **已禁用** → 删除前置检查：
   1. 立即 `await fieldApi.references(row.id)`，取 `templates / fields / fsms` 三个数组。
   2. 若 `tpls.length + flds.length + fsms.length > 0`：调用 `showRefDialog(row, tpls, flds, fsms)` 展示三分区弹窗 + `ElMessage.warning` 提示被引用数，**阻止删除**，直接 return。
   3. 若全部为空：`ElMessageBox.confirm` 二次确认 → `await fieldApi.delete(row.id)` → toast 成功 + `fetchList()`。
   4. 确认 / 删除阶段如果后端返回 `REF_DELETE (40005)`（说明前置检查与最终删除之间出现了新引用的 race），调用 `loadAndShowRefs(row)` 重新拉 references 并打开弹窗。
   5. `references` API 调用本身失败时**保守处理**：不继续走确认删除，直接 return（避免在未知引用状态下误删）。

### 7.3 `handleShowRefs` 拆分为 `showRefDialog` + `loadAndShowRefs`

按数据来源拆成两个函数，职责明确：

- `showRefDialog(row, templates, fields, fsms)`：**同步**设置弹窗状态，数据已从调用方（前置 `fieldApi.references`）预加载，`loading=false` 直接展示。用于主流程。
- `loadAndShowRefs(row)`：**异步**先打开弹窗 + `loading=true`，清空三个数组，然后 `await fieldApi.references(row.id)` 重新拉数据。用于 `REF_DELETE` 兜底路径和任何需要重新拉取的场景。

两个入口共享同一个 `refDialog` reactive 对象和同一个 `el-dialog` 模板实例，关闭时通过 `@close="resetRefDialog"` 统一清理。

### 7.4 引用详情弹窗：三分区布局

```
引用详情 — {label} ({name})
├─ 模板引用（N 个模板引用了该字段）
│    el-table: 模板名称 / 类型    或   "暂无模板引用"
├─ 字段引用（N 个 reference 字段引用了该字段）
│    el-table: 字段名 / 类型      或   "暂无字段引用"
└─ FSM 引用（N 个状态机引用了该 BB Key）    [V3 新增]
     el-table: 状态机名称 / 类型  或   "暂无 FSM 引用"
```

`refDialog` reactive 结构：

```ts
const refDialog = reactive({
  visible: false,
  loading: false,
  name: '',
  label: '',
  templates: [] as ReferenceItem[],
  fields:    [] as ReferenceItem[],
  fsms:      [] as ReferenceItem[],   // V3 新增
})
```

FSM 引用的来源与前两类不同：**templates / fields 由 MySQL 引用表直接存储**，**fsms 则由后端扫描 FSM 配置中使用的 BB Key，反查该字段（expose_bb=true 且 name 匹配）得到**。前端对这种差异无感知，统一按 `ReferenceItem[]` 渲染。

### 7.5 `has_refs` 驱动 FieldForm 锁定

`FieldForm.vue` 用 `const hasRefs = ref(false)` 替代原 `refCount`，`loadFieldDetail` 中 `hasRefs.value = data.has_refs || false`。

- **字段类型下拉 disabled**：
  ```vue
  :disabled="isView || (!isCreate && hasRefs)"
  ```
  - `isView`：查看模式始终禁用。显式加这前缀是因为 Element Plus `el-select` 的 `disabled` 为 `undefined ?? false` 时会覆盖成 `false`，导致查看模式下类型下拉仍可操作。
  - `!isCreate && hasRefs`：编辑模式下，一旦存在引用（任何一类），类型不可改。

- **警告文案**：
  ```vue
  <div v-if="!isCreate && hasRefs" class="field-warn">
    该字段被引用中，无法更改类型
  </div>
  ```
  从原先"已被 N 处引用"改为布尔语义，不再展示具体数字。

- **约束组件 restricted**：
  ```vue
  <FieldConstraintInteger :restricted="hasRefs" ... />
  <FieldConstraintString  :restricted="hasRefs" ... />
  <FieldConstraintSelect  :restricted="hasRefs" ... />
  <FieldConstraintReference :restricted="hasRefs" ... />
  ```
  约束组件内部据此屏蔽"收紧类"操作（例如 integer 的 min 变大 / max 变小、string 的 maxLength 变小、select 删除已有 option 等），仅允许"放宽类"变更。

### 7.6 约束组件 `validate()` 模式

所有约束组件（`FieldConstraintInteger` / `FieldConstraintString` / `FieldConstraintSelect` / `FieldConstraintReference`）通过 `defineExpose({ validate })` 暴露校验方法。`FieldForm.vue` 持有 `constraintRef`（模板引用），提交前调用 `constraintRef.value?.validate()` 进行约束级校验：

- `FieldConstraintInteger`：min ≤ max、step > 0、precision ≥ 0。
- `FieldConstraintString`：minLength ≤ maxLength、pattern 可编译。
- `FieldConstraintSelect`：options 非空 + minSelect ≤ maxSelect ≤ options.length。
- `FieldConstraintReference`：`ref_fields.length >= 1`（对应 40017 前端拦截）+ 所有条目 `type !== 'reference'`（对应 40016 前端拦截）。

校验失败返回 `false` 并内联错误提示，不发起 update 请求。后端同样做这些校验作为最终兜底，前端负责在提交前给出快速反馈。

### 7.7 约束组件 `disabled` prop（查看模式）

`FieldConstraintSelect` 和 `FieldConstraintReference` 除 `restricted` 外还接受 `disabled` prop：

- `disabled = isView`：查看模式下禁用所有内部控件（选项列表编辑、添加/删除按钮、el-input、el-input-number、操作图标等）。
- `restricted = hasRefs`：编辑模式下仅屏蔽收紧类操作，不影响放宽类操作。

两者组合完成三态：新建 / 编辑（hasRefs=true 部分锁） / 查看（全锁）。

### 7.8 `EnabledGuardDialog` 去 ref_count 化

V3 同步清理：

- `GuardEntity` 接口删除 `ref_count` 字段，只保留 `{id, name, label}`。
- `refCountPass`（检查 ref_count===0）计算属性删除。
- `refTargetLabel`（根据 entityType 拼"引用目标"文案）计算属性删除。
- 删除场景的「前置条件」区域简化为**单一条件**：
  ```
  删除前置条件
  [X] {字段/模板/事件类型/扩展字段}已禁用
  ```
  不再展示"无引用"那条红/绿 check，因为删除前的引用检查已经下沉到 `FieldList.vue.handleDelete` 的前置 `references` 查询（以及 `TemplateList.vue` 的对应流程），不再由守卫弹窗承担。
- 守卫弹窗的职责收敛为单一语义：**启用中的实体不能直接编辑/删除，先禁用**。`action === 'edit'` 展示操作步骤，`action === 'delete'` 展示唯一前置条件。
- 「立即禁用」按钮逻辑不变：`fieldApi.detail → fieldApi.toggleEnabled(false, version)`；成功后编辑跳转到编辑页，删除仅 emit('refresh') 让父组件刷新列表由用户再点一次「删除」（删除场景不自动替用户执行删除，因为删除不可恢复）。

### 7.9 `ref_fields` <-> `refs` 转换边界

**唯一转换点**：`FieldForm.vue`，其余位置都读写后端权威的 `refs: number[]`。

加载流程（`loadFieldDetail`）：

```
1. const detail = await fieldApi.detail(id)   // 拿到 constraints.refs: [3,7,9]
2. 拉取候选字段列表（可引用的字段，过滤 type!=='reference' && enabled===true）
3. join: [3,7,9] -> [{id:3, name, label, type}, {id:7, ...}, {id:9, ...}]
4. 作为 FieldConstraintReference 的 v-model:ref_fields
```

提交流程（`handleSubmit`）：

```
1. ref_fields: [{id,name,label,type}, ...] -> refs: ids.map(x => x.id)
2. 写入 properties.constraints.refs
3. 移除 properties.constraints.ref_fields（前端 UI 专用结构，后端不认）
4. fieldApi.update / fieldApi.create
```

其他任何组件或业务流读字段详情时，都应直接读 `properties.constraints.refs`，**禁止重复做这层 id→富对象的转换**，避免口径分裂。`ReferenceDetail.fields` 返回的是"引用我的字段"，与 `constraints.refs`（"我引用谁"）方向相反，不可混用。

### 7.10 启用/禁用 toggle 的 version 获取

列表接口 `FieldListItem` 虽含 `version`，但考虑到分页列表可能跨用户存在毫秒级陈旧，`handleToggle` 仍显式 `await fieldApi.detail(row.id)` 拿最新 `version` 再调 `toggleEnabled`，换一次额外请求换"最小化 VERSION_CONFLICT 概率"。命中 40010 时走统一的"数据被修改请刷新"提示 + `fetchList()` 刷新。
