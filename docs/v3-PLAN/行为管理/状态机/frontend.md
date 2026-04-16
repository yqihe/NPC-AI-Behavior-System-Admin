# 状态机管理 — 前端页面设计

> 通用架构/规范见 `docs/architecture/overview.md` 和 `docs/development/`。
> 后端实现细节见同目录 `backend.md`，功能清单见 `features.md`。
> 本文档记录状态机管理模块的前端设计事实与特有约束。
>
> **实现状态**：状态机前端模块尚未实现，本文档为设计蓝图（待实现）。字段管理 / 模板管理 / 事件类型管理的已有实现是本模块的参照基线，命名、布局、交互、错误处理必须与它们保持一致。

---

## 1. 目录结构

```
frontend/src/
├─ views/
│  ├─ FsmConfigList.vue               # 状态机列表页（待实现）
│  └─ FsmConfigForm.vue               # 新建/查看/编辑三合一表单页（待实现）
├─ components/
│  ├─ FsmStateListEditor.vue          # 状态列表编辑器（状态增删改 + initial_state 选择）（待实现）
│  ├─ FsmTransitionListEditor.vue     # 转换规则列表编辑器（from/to/priority/condition）（待实现）
│  ├─ FsmConditionTreeEditor.vue      # 条件树递归编辑器（支持叶节点 + and/or 组合节点）（待实现）
│  ├─ FsmConditionLeafRow.vue         # 条件树叶节点一行（key/op/value or ref_key）（待实现）
│  └─ BBKeySelector.vue               # BB Key 下拉（来源：字段表 + 运行时 Key 表，可选自输入）（待实现）
├─ api/
│  └─ fsmConfigs.ts                   # FsmConfig REST 调用 + 类型契约 + FSM_ERR 错误码常量（待实现）
└─ router/
   └─ index.ts                        # 新增 /fsm-configs/* 路由（待追加）
```

**与既有模块的对应关系**

| 既有模块 | 状态机模块 |
|---|---|
| `FieldList.vue` / `EventTypeList.vue` | `FsmConfigList.vue`（同构：搜索 + 启用三态筛选 + 分页 + EnabledGuardDialog） |
| `EventTypeForm.vue` / `TemplateForm.vue` | `FsmConfigForm.vue`（同构：mode=create/view/edit 三态 + `isView` computed） |
| `TemplateSelectedFields.vue`（列表增删） | `FsmStateListEditor.vue`（列表增删 + 选中） |
| —（首次引入） | `FsmConditionTreeEditor.vue`（递归组件） |

---

## 2. 页面路由

在 `frontend/src/router/index.ts` 追加：

| path | name | component | meta | 说明 |
|------|------|-----------|------|------|
| `/fsm-configs` | `fsm-config-list` | `FsmConfigList.vue` | `{ title: '状态机管理' }` | 列表页 |
| `/fsm-configs/create` | `fsm-config-create` | `FsmConfigForm.vue` | `{ title: '新建状态机', isCreate: true }` | 新建 |
| `/fsm-configs/:id/view` | `fsm-config-view` | `FsmConfigForm.vue` | `{ title: '查看状态机', isCreate: false, isView: true }` | 查看（只读） |
| `/fsm-configs/:id/edit` | `fsm-config-edit` | `FsmConfigForm.vue` | `{ title: '编辑状态机', isCreate: false }` | 编辑 |

路由风格与 `/event-types/*` 完全一致（kebab-case 复数 + 四条子路由）。

---

## 3. 组件树

### 3.1 列表页 `FsmConfigList.vue`

```
FsmConfigList.vue
├─ el-input（display_name 模糊搜索，带 debounce 300ms）
├─ el-select（enabled 三态筛选：全部 / 仅启用 / 仅停用）
├─ el-button type="primary"（新建状态机 → /fsm-configs/create）
├─ el-table
│   ├─ 列：ID / name（等宽字体）/ display_name / initial_state / state_count / enabled（el-switch）/ created_at / 操作
│   └─ 操作列：查看 / 编辑 / 删除
├─ el-pagination（page + page_size，后端分页）
└─ EnabledGuardDialog ref="guardRef"
    └─ entityType="fsm-config"（需在 EnabledGuardDialog 中新增该分支）
```

### 3.2 表单页 `FsmConfigForm.vue`

```
FsmConfigForm.vue
├─ el-form（labelPosition="top"）
│   ├─ 基础信息区块
│   │   ├─ name（el-input + check-name 实时校验 + 仅新建可编辑）
│   │   └─ display_name（el-input）
│   ├─ 状态定义区块
│   │   ├─ initial_state（el-select，选项来自当前 states 列表）
│   │   └─ FsmStateListEditor
│   │       ├─ el-button「添加状态」
│   │       └─ v-for state in states
│   │           └─ el-input（state.name）+ 删除按钮
│   └─ 转换规则区块
│       └─ FsmTransitionListEditor
│           ├─ el-button「添加转换规则」
│           └─ v-for transition in transitions
│               ├─ el-select from（选项来自 states）
│               ├─ el-select to  （选项来自 states）
│               ├─ el-input-number priority（min=0）
│               ├─ FsmConditionTreeEditor v-model="transition.condition"
│               │   └─ FsmConditionNode（递归组件）
│               │       ├─ 叶节点分支 → FsmConditionLeafRow
│               │       │   ├─ BBKeySelector（key）
│               │       │   ├─ el-select（op：==, !=, >, >=, <, <=, in）
│               │       │   ├─ 比较值模式切换（value | ref_key）
│               │       │   ├─ 值输入（JSON 编辑器或类型推导输入框）
│               │       │   └─ BBKeySelector（ref_key）
│               │       └─ 组合节点分支
│               │           ├─ 组合方式 radio（and | or）
│               │           ├─ v-for child → FsmConditionNode 递归
│               │           └─ 「添加子条件」按钮
│               └─ 删除按钮
└─ 底部固定操作栏
    ├─ el-button「取消」
    └─ el-button type="primary"「保存」（view 模式下隐藏）
```

### 3.3 `EnabledGuardDialog` 复用

FSM 列表的「编辑/删除已启用项」场景必须走 `EnabledGuardDialog`。需要在 `EnabledGuardDialog.vue` 中扩展：

- `EntityType` 联合类型追加 `'fsm-config'`
- `entityTypeLabel` 追加 `if (entityType.value === 'fsm-config') return '状态机'`
- `reasonText` 追加 FSM 专属文案：「已启用的状态机对 NPC 管理页可见，任意修改可能导致引用方看到不稳定的配置。请先禁用，再进入编辑。」
- `onActOnce` 追加 `fsm-config` 分支：`fsmConfigApi.detail(id)` + `fsmConfigApi.toggleEnabled(id, false, version)`
- 冲突码分支追加 `FSM_ERR.VERSION_CONFLICT`

---

## 4. 类型契约

### 4.1 通用类型（复用 `api/fields.ts` 导出）

```ts
import type { ApiResponse, ListData, CheckNameResult } from '@/api/fields'
```

### 4.2 FSM 专属类型（`api/fsmConfigs.ts`）

```ts
/** 列表查询参数 */
export interface FsmConfigListQuery {
  label?: string             // display_name 模糊
  enabled?: boolean | null   // 三态：undefined/null=全部，true/false=精确
  page: number
  page_size: number
}

/** 列表项（从 config_json 抽 initial_state / state_count） */
export interface FsmConfigListItem {
  id: number
  name: string
  display_name: string
  initial_state: string
  state_count: number
  enabled: boolean
  version: number
  created_at: string
}

/** 单个状态 */
export interface FsmState {
  name: string
}

/** 条件树节点（叶节点与组合节点共用） */
export interface FsmCondition {
  // 叶节点字段
  key?: string
  op?: '==' | '!=' | '>' | '>=' | '<' | '<=' | 'in'
  value?: unknown         // JSON 任意类型，与 ref_key 二选一
  ref_key?: string        // 引用另一个 BB Key，与 value 二选一
  // 组合节点字段（and / or 互斥）
  and?: FsmCondition[]
  or?: FsmCondition[]
}

/** 转换规则 */
export interface FsmTransition {
  from: string
  to: string
  priority: number
  condition: FsmCondition  // 空对象表示「无条件转换」
}

/** 详情响应中的 config 部分（config_json unmarshal 展开） */
export interface FsmConfigBody {
  initial_state: string
  states: FsmState[]
  transitions: FsmTransition[]
}

/** 详情响应 */
export interface FsmConfigDetail {
  id: number
  name: string
  display_name: string
  enabled: boolean
  version: number
  created_at: string
  updated_at: string
  config: FsmConfigBody
}

/** 创建请求 */
export interface CreateFsmConfigRequest {
  name: string
  display_name: string
  initial_state: string
  states: FsmState[]
  transitions: FsmTransition[]
}

/** 编辑请求 */
export interface UpdateFsmConfigRequest {
  id: number
  display_name: string
  initial_state: string
  states: FsmState[]
  transitions: FsmTransition[]
  version: number
}

/** 删除响应 */
export interface DeleteFsmConfigResult {
  id: number
  name: string
  label: string
}
```

### 4.3 错误码常量

```ts
// 与 backend/internal/errcode/codes.go 43001-43012 保持一致
export const FSM_ERR = {
  NAME_EXISTS:           43001,
  NAME_INVALID:          43002,
  NOT_FOUND:             43003,
  STATES_EMPTY:          43004,
  STATE_NAME_INVALID:    43005,
  INITIAL_INVALID:       43006,
  TRANSITION_INVALID:    43007,
  CONDITION_INVALID:     43008,
  DELETE_NOT_DISABLED:   43009,
  EDIT_NOT_DISABLED:     43010,
  VERSION_CONFLICT:      43011,
  REF_DELETE:            43012,
} as const
```

---

## 5. API 调用映射

`api/fsmConfigs.ts` 暴露 `fsmConfigApi`，全部走 `request.post`（导出 API 除外）：

| 方法 | HTTP | 路径 | 请求体 | 响应体 |
|------|------|------|--------|--------|
| `list(params)` | POST | `/fsm-configs/list` | `FsmConfigListQuery` | `ListData<FsmConfigListItem>` |
| `create(data)` | POST | `/fsm-configs/create` | `CreateFsmConfigRequest` | `{ id: number; name: string }` |
| `detail(id)` | POST | `/fsm-configs/detail` | `{ id }` | `FsmConfigDetail` |
| `update(data)` | POST | `/fsm-configs/update` | `UpdateFsmConfigRequest` | `string`（`"保存成功"`） |
| `delete(id)` | POST | `/fsm-configs/delete` | `{ id }` | `DeleteFsmConfigResult` |
| `checkName(name)` | POST | `/fsm-configs/check-name` | `{ name }` | `CheckNameResult` |
| `toggleEnabled(id, enabled, version)` | POST | `/fsm-configs/toggle-enabled` | `{ id, enabled, version }` | `string`（`"操作成功"`） |
| `exportAll()` | GET | `/api/configs/fsm_configs` | — | `{ items: [{ name, config }] }` |

```ts
export const fsmConfigApi = {
  list: (params: FsmConfigListQuery) =>
    request.post('/fsm-configs/list', params) as Promise<ApiResponse<ListData<FsmConfigListItem>>>,
  create: (data: CreateFsmConfigRequest) =>
    request.post('/fsm-configs/create', data) as Promise<ApiResponse<{ id: number; name: string }>>,
  detail: (id: number) =>
    request.post('/fsm-configs/detail', { id }) as Promise<ApiResponse<FsmConfigDetail>>,
  update: (data: UpdateFsmConfigRequest) =>
    request.post('/fsm-configs/update', data) as Promise<ApiResponse<string>>,
  delete: (id: number) =>
    request.post('/fsm-configs/delete', { id }) as Promise<ApiResponse<DeleteFsmConfigResult>>,
  checkName: (name: string) =>
    request.post('/fsm-configs/check-name', { name }) as Promise<ApiResponse<CheckNameResult>>,
  toggleEnabled: (id: number, enabled: boolean, version: number) =>
    request.post('/fsm-configs/toggle-enabled', { id, enabled, version }) as Promise<ApiResponse<string>>,
}
```

**关于跨模块事务**：后端 Create/Update/Delete 在同一 MySQL 事务中维护 `fsm_configs` + `field_refs(ref_type='fsm')` BB Key 反向引用。前端不感知事务细节，但务必注意：

- 保存成功即意味着 BB Key 引用已同步入库，BB Key 来源字段的详情缓存已被服务端清理。
- 若保存操作因事务中任何一步失败而回滚，前端会收到对应错误码（43001 / 43004-43008 / 43010 / 43011），需照常走错误分支提示。

---

## 6. 错误码处理

FSM 错误码范围 `43001-43012`，前端 `api/fsmConfigs.ts` 导出 `FSM_ERR` 常量，所有 catch 分支按码分派。

| 错误码 | 常量 | UI 反馈 | 触发操作 |
|--------|------|---------|----------|
| 43001 | `NAME_EXISTS` | 在 name 表单项下方显示红字「该状态机标识已存在」；同时 `checkName` 轮询失败态 | 创建 |
| 43002 | `NAME_INVALID` | 表单项下方红字「格式不合法：需小写字母开头，仅允许 a-z、0-9、下划线」 | 创建 / check-name |
| 43003 | `NOT_FOUND` | `ElMessage.error('状态机配置不存在')` + 返回列表页 | 详情 / 编辑 / 删除 / 切换 |
| 43004 | `STATES_EMPTY` | 状态列表区块标题下方红字「至少定义一个状态，且不超过 50 个」；禁用保存按钮 | 创建 / 编辑 |
| 43005 | `STATE_NAME_INVALID` | 状态名输入框红框 + 行内提示「状态名不能为空且不能重复」 | 创建 / 编辑 |
| 43006 | `INITIAL_INVALID` | initial_state 下拉红框 + 提示「初始状态必须是已定义的状态之一」 | 创建 / 编辑 |
| 43007 | `TRANSITION_INVALID` | 对应 transition 行红框 + 提示「转换规则引用了不存在的状态，或优先级无效」 | 创建 / 编辑 |
| 43008 | `CONDITION_INVALID` | 条件树节点高亮 + 提示「条件表达式不合法，请检查嵌套层数、操作符、value/ref_key 设置」 | 创建 / 编辑 |
| 43009 | `DELETE_NOT_DISABLED` | 不弹 `ElMessage`，改弹 `EnabledGuardDialog(action='delete', entityType='fsm-config')` | 删除 |
| 43010 | `EDIT_NOT_DISABLED` | 弹 `EnabledGuardDialog(action='edit', entityType='fsm-config')` | 编辑入口点击 |
| 43011 | `VERSION_CONFLICT` | `ElMessage.warning('该状态机已被其他人修改，请刷新后重试')` + 触发列表刷新 | 编辑 / 切换启用 |
| 43012 | `REF_DELETE` | `ElMessage.error('当前状态机仍被引用，不能删除')`（本期 ref_count 恒 0，实际不会触发） | 删除 |

**分派模板**（与 `EventTypeList.vue` 的 `handleError` 同构）：

```ts
try {
  await fsmConfigApi.update(payload)
} catch (err) {
  const bizErr = err as BizError
  switch (bizErr.code) {
    case FSM_ERR.EDIT_NOT_DISABLED:
      guardRef.value?.open({ action: 'edit', entityType: 'fsm-config', entity: row })
      return
    case FSM_ERR.VERSION_CONFLICT:
      ElMessage.warning('该状态机已被其他人修改，请刷新后重试')
      refresh()
      return
    case FSM_ERR.STATES_EMPTY:
    case FSM_ERR.STATE_NAME_INVALID:
    case FSM_ERR.INITIAL_INVALID:
    case FSM_ERR.TRANSITION_INVALID:
    case FSM_ERR.CONDITION_INVALID:
      // 由表单校验态处理，不再 toast
      return
    // 其他错误由 request 拦截器统一 toast
  }
}
```

---

## 7. 关键实现细节

### 7.1 条件树编辑器（`FsmConditionTreeEditor` + 递归子组件）

**模型形态三选一**（与后端 `FsmCondition.IsEmpty()` / 叶节点 / 组合节点三态对齐）：

| UI 表现 | 数据形态 |
|---|---|
| 「无条件转换」开关（默认关） | 所有字段为零值 → 序列化为 `{}` |
| 「叶条件」单行 | `{ key, op, value? | ref_key? }` |
| 「组合条件」多行 | `{ and: [...] }` 或 `{ or: [...] }` |

**组件契约**：

```vue
<FsmConditionTreeEditor
  v-model="transition.condition"
  :bb-keys="bbKeys"
  :max-depth="10"
  :current-depth="0"
  :readonly="isView"
/>
```

`v-model` 绑定单个 `FsmCondition` 节点；组件内部根据节点形态切分渲染：

- 若 `and?.length || or?.length` → 渲染组合节点分支，`v-for` 每个子节点递归 `<FsmConditionTreeEditor :current-depth="currentDepth+1" v-model="child" />`
- 否则 → 渲染 `FsmConditionLeafRow`
- 顶层节点提供「转为组合 / 转为叶节点 / 清空（无条件转换）」切换按钮

**前端预校验**（与后端 service 层 `validateCondition` 规则对齐，错误码 43008 的本地拦截）：

| 规则 | 触发 |
|---|---|
| 嵌套深度 > `max_depth`（10） | 递归组件 `currentDepth >= maxDepth` 时禁用「转为组合」按钮 |
| 叶节点 `op` 必须在白名单 | `el-select` 选项即白名单 |
| `value` 与 `ref_key` 不能同时非空 | 比较值模式 radio 切换，二选一 |
| `value` 与 `ref_key` 不能同时为空 | 保存前扫描所有叶节点；任一为空则在该叶节点标红 |
| `and` / `or` 不能共存 | 组合方式 radio 切换，一次只写一个 key |
| 叶节点与组合节点互斥 | 切换「转为组合」时清空 key/op/value；切换「转为叶」时清空 and/or |

**序列化要求**：保存时提交的 JSON 必须精确反映三种形态。具体实现：

```ts
function serializeCondition(c: FsmCondition): Record<string, unknown> {
  if (!c.key && !c.and?.length && !c.or?.length) return {}  // 空条件
  if (c.and?.length) return { and: c.and.map(serializeCondition) }
  if (c.or?.length) return { or: c.or.map(serializeCondition) }
  const leaf: Record<string, unknown> = { key: c.key, op: c.op }
  if (c.ref_key) leaf.ref_key = c.ref_key
  else leaf.value = c.value
  return leaf
}
```

### 7.2 状态列表管理（`FsmStateListEditor`）

**交互要点**：

- 「添加状态」按钮在末尾追加 `{ name: '' }`，自动聚焦新行
- 每行可删除，但若该状态为 `initial_state` 或被任一 transition 的 `from`/`to` 引用，删除前弹 `ElMessageBox.confirm('该状态被引用，删除后将同时清空相关引用，是否继续？')`
- 状态名编辑时实时检测重复，红框提示
- 状态总数接近 `max_states`（50）时，顶部显示黄色横条「接近状态数上限 (45/50)」；到达上限时禁用「添加状态」按钮

**与 initial_state 的联动**：

- `initial_state` 下拉的选项始终来自当前 `states.map(s => s.name).filter(Boolean)`
- 删除当前 `initial_state` 对应状态时，自动将 `initial_state` 重置为 `states[0]?.name ?? ''`
- 状态改名时，若等于当前 `initial_state`，同步刷新 `initial_state` 值

**与 transitions 的联动**：

- 状态改名时，扫描所有 transition.from/to，若命中旧名则弹确认「同步更新转换规则引用？」，确认则批量改
- 状态删除时，清空 transitions 中引用该状态的 from/to（赋空字符串，由保存时校验报 43007）

### 7.3 转换规则编辑器（`FsmTransitionListEditor`）

- 每行固定结构：`from | to | priority | condition`
- 「添加转换规则」按钮在末尾追加 `{ from: '', to: '', priority: 0, condition: {} }`
- `from` / `to` 下拉选项来自 `states`，实时响应状态列表变化
- `priority` 使用 `el-input-number`，`:min="0"` `:step="1"`，显示「数字越大越优先」提示
- 右侧折叠按钮展开/收起 `FsmConditionTreeEditor`；收起时仅显示条件摘要（「无条件」/ 「叶：key op value」/ 「组合：and/or (N 项)」）
- 总数接近 `max_transitions`（200）时，顶部黄色横条提示
- 支持拖拽排序（仅视觉排序，后端按数组顺序保存）—— **待实现**

### 7.4 BB Key 选择（`BBKeySelector`）

**已实现**。条件树的 `key` 和 `ref_key` 都是 BB Key 标识，`BBKeySelector` 同时从两个数据源加载选项：

| 来源 | 获取方式 | 分组标签 |
|------|----------|---------|
| 字段管理中「标记为 BB 暴露」的字段 | `fieldApi.list({ bb_exposed: true, enabled: true, page_size: 200 })` | `NPC 字段` |
| 事件扩展字段 Schema | `eventTypeApi.schemaList({ enabled: true })` | `事件扩展字段` |

两类在下拉中用 `el-option-group` 分组区分。允许自由输入（`allow-create`），支持运行时 Key 手动填写。

**类型系统**（`BBKeyField` 接口，由 `BBKeySelector.vue` 导出）：

```ts
export interface BBKeyField {
  name: string
  label: string
  /** 规范化类型：integer / float / string / bool / select / reference */
  type: string
}
```

`field-selected` 事件 emit `BBKeyField | null`（自由输入时为 `null`，降级为文本框）。

**类型名规范化**：两类来源的原始类型名不一致，`BBKeySelector` 内部统一转换后再 emit：

| 原始值 | 来源 | 规范化为 |
|--------|------|---------|
| `boolean` | NPC 字段 | `bool` |
| `int` | 事件扩展字段 | `integer` |
| 其他 | 任意 | 保持原值 |

`FsmConditionEditor` 根据 `selectedFieldType` 渲染值输入控件（`bool` → el-select，`integer` → 整数输入框，`float` → 浮点输入框，其余 → 文本框）。

**后端写入 `field_refs`**：保存时后端自动扫描 condition 中所有 `key` / `ref_key`，命中字段表的写入 `field_refs(ref_type='fsm', source_id=fsm_id)`。前端无需手动组装引用列表。

### 7.5 启用/停用状态管理

**列表页启用开关**（`el-switch`）：

```ts
async function onToggleEnabled(row: FsmConfigListItem) {
  const next = !row.enabled
  try {
    await fsmConfigApi.toggleEnabled(row.id, next, row.version)
    ElMessage.success(next ? '已启用' : '已禁用')
    refresh()
  } catch (err) {
    const bizErr = err as BizError
    if (bizErr.code === FSM_ERR.VERSION_CONFLICT) {
      ElMessage.warning('该状态机已被其他人修改，请刷新后重试')
      refresh()
    }
    // 其他错误由拦截器处理
  }
}
```

**「编辑」「删除」按钮的启用态守卫**：

- 列表页的「编辑」/「删除」按钮在 `row.enabled === true` 时**不禁用**（保持可点击），点击后：
  - 编辑：若后端返回 43010 → 弹 `EnabledGuardDialog(action='edit')`；否则跳转 `/fsm-configs/:id/edit`
  - 删除：若后端返回 43009 → 弹 `EnabledGuardDialog(action='delete')`
- 表单页 `FsmConfigForm.vue` 的保存按钮在编辑模式下不做前端启用态拦截，一律以后端返回为准

**mode 三态**（与 `EventTypeForm.vue` 一致）：

```ts
const isCreate = computed(() => route.meta.isCreate === true)
const isView = computed(() => route.meta.isView === true)
const isEdit = computed(() => !isCreate.value && !isView.value)

// name 只在 isCreate 下可编辑
// 所有表单项在 isView 下 disabled
// 底部「保存」按钮在 isView 下隐藏
```

### 7.6 name 实时校验

- `el-input` 失焦或 debounce 500ms 后调用 `fsmConfigApi.checkName(name)`
- 响应 `{ available: true }` → 绿色对钩；`{ available: false, message }` → 红色叉 + 消息
- 同时前端本地正则 `^[a-z][a-z0-9_]*$` + 长度 <= 64 预校验，失败直接红字，不调后端
- 仅在 `isCreate` 下启用，编辑态 name 只读

### 7.7 与后端跨模块事务的契合

后端 Create/Update/Delete 是跨模块事务（handler 在同一 `*sqlx.Tx` 上编排 fsm_configs + field_refs 的读写）。前端的契合要求：

1. **一次请求即全部完成**：前端不需要分两次调用写 FSM 和写 field_refs，单一 `create/update/delete` 接口保证原子性。
2. **错误边界一致**：任何 BB Key 处理异常（例如字段被软删或禁用）都会连同 FSM 写入一并回滚，前端收到统一错误码即可。
3. **缓存一致性**：保存成功后，FSM 详情 / 列表缓存 + 受影响字段的详情缓存已由后端自动清理，前端无需额外操作；但列表页保存返回后仍需触发一次 `refresh()` 以拉最新版本号。

---

## 8. 样式与交互规范

- 表格行高、按钮间距、列宽与 `EventTypeList.vue` 保持像素级一致
- 表单分区块使用 `el-divider + 小节标题`，与 `EventTypeForm.vue` 一致
- 条件树节点缩进：每递归一层左边距 `+16px`，使用 `border-left: 2px solid #EBEEF5` 可视化层级
- 叶节点的 `op` 下拉统一宽度 72px；`value` 输入框根据 BB Key 字段类型自适应（字符串 → input；数字 → input-number；布尔 → switch；in → JSON 数组编辑器）— **待实现基于字段 Schema 的类型推导**
- 所有删除操作统一走 `ElMessageBox.confirm`
- 保存成功后按 `ElMessage.success('保存成功')` + `router.push('/fsm-configs')`
- 错误码 toast 与其他模块共用 `request.ts` 拦截器

---

## 9. 待实现清单

| 项 | 说明 |
|---|---|
| `FsmConfigList.vue` | 列表页 |
| `FsmConfigForm.vue` | 表单三合一 |
| `FsmStateListEditor.vue` | 状态列表编辑器 |
| `FsmTransitionListEditor.vue` | 转换规则列表编辑器 |
| `FsmConditionTreeEditor.vue` | 递归条件树编辑器 |
| `FsmConditionLeafRow.vue` | 叶节点行组件 |
| `BBKeySelector.vue` | ✅ 已实现。双数据源（NPC 字段 + 事件扩展字段），el-option-group 分组，类型名规范化，支持自由输入运行时 Key |
| `api/fsmConfigs.ts` | REST 调用 + 类型 + `FSM_ENT_ERR` |
| `router/index.ts` 追加 `/fsm-configs/*` 四条路由 | — |
| `EnabledGuardDialog.vue` 追加 `entityType='fsm-config'` 分支 | 新增分支 + FSM_ERR 导入 |
| `AppLayout.vue` 侧边栏菜单追加「行为管理 → 状态机管理」入口 | — |
| 基于字段 Schema 的条件值类型推导 | 依赖字段详情的 `field_type` / `constraints` |
| 转换规则拖拽排序 | 体验增强项 |
| 运行时 Key 表接口 | 待后端规划 |

---

## 10. 依赖关系

- **依赖字段管理**：BB Key 下拉需读字段列表；保存后后端维护 `field_refs(ref_type='fsm')` 反向引用
- **被事件类型 / 行为树 / NPC 管理依赖**：状态机 name 作为被引用项；BT 管理按「状态名 → bt_ref」挂接；NPC.behavior.fsm_ref 指向 FSM name
- **与后端契约**：以 `backend.md` 的 API / 错误码 / config_json 结构为唯一真实来源，前端类型 (`FsmConfig*` / `FSM_ERR`) 与后端保持逐项一致
