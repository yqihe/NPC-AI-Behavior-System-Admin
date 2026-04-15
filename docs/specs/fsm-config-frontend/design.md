# FSM Config Frontend — 设计文档

## 整体架构

### 路由结构

```
/fsm-configs              → FsmConfigList.vue   (列表页)
/fsm-configs/create       → FsmConfigForm.vue   (新建)
/fsm-configs/:id/view     → FsmConfigForm.vue   (查看，isView=true)
/fsm-configs/:id/edit     → FsmConfigForm.vue   (编辑，isCreate=false)
```

### 文件清单

| 文件 | 类型 | 说明 |
|------|------|------|
| `frontend/src/api/fsmConfigs.ts` | 新建 | 类型 + FSM_ERR + fsmConfigApi |
| `frontend/src/views/FsmConfigList.vue` | 新建 | 列表页 |
| `frontend/src/views/FsmConfigForm.vue` | 新建 | 新建/编辑/查看表单页 |
| `frontend/src/components/FsmStateListEditor.vue` | 新建 | 状态列表编辑器 |
| `frontend/src/components/FsmTransitionListEditor.vue` | 新建 | 转换规则列表编辑器 |
| `frontend/src/components/FsmConditionEditor.vue` | 新建 | 条件编辑器（递归） |
| `frontend/src/components/BBKeySelector.vue` | 新建 | BB Key 下拉 |
| `frontend/src/components/EnabledGuardDialog.vue` | 修改 | 追加 fsm-config 分支 |
| `frontend/src/components/AppLayout.vue` | 修改 | 追加「状态机管理」菜单项 |
| `frontend/src/router/index.ts` | 修改 | 追加 4 条路由 |

---

## API 设计（fsmConfigs.ts）

### 后端接口对照

| 接口 | 路径 | 请求 |
|------|------|------|
| 列表 | POST `/fsm-configs/list` | `{ label?, enabled?, page, page_size }` |
| 创建 | POST `/fsm-configs/create` | `{ name, display_name, initial_state, states, transitions }` |
| 详情 | POST `/fsm-configs/detail` | `{ id }` |
| 编辑 | POST `/fsm-configs/update` | `{ id, display_name, initial_state, states, transitions, version }` |
| 删除 | POST `/fsm-configs/delete` | `{ id }` |
| 校验名 | POST `/fsm-configs/check-name` | `{ name }` |
| 切换启用 | POST `/fsm-configs/toggle-enabled` | `{ id, enabled, version }` |

### 错误码（backend errcode 43001–43012）

```ts
FSM_ERR = {
  NAME_EXISTS:         43001,
  NAME_INVALID:        43002,
  NOT_FOUND:           43003,
  STATES_EMPTY:        43004,
  STATE_NAME_INVALID:  43005,
  INITIAL_INVALID:     43006,
  TRANSITION_INVALID:  43007,
  CONDITION_INVALID:   43008,
  DELETE_NOT_DISABLED: 43009,
  EDIT_NOT_DISABLED:   43010,
  VERSION_CONFLICT:    43011,
}
```

### 关键数据结构

```ts
// 条件节点（递归）
interface FsmConditionNode {
  // 叶节点
  key?: string
  op?: string
  value?: unknown          // 直接值
  ref_key?: string         // 引用 BB Key
  // 组合节点
  and?: FsmConditionNode[]
  or?: FsmConditionNode[]
}

// 转换规则
interface FsmTransition {
  from: string
  to: string
  priority: number
  condition: FsmConditionNode
}

// 列表项（含 initial_state/state_count，由后端 service 填充）
interface FsmConfigListItem {
  id: number
  name: string
  display_name: string
  initial_state: string
  state_count: number
  enabled: boolean
  created_at: string
}

// 详情（config 字段展开）
interface FsmConfigDetail {
  id: number
  name: string
  display_name: string
  enabled: boolean
  version: number
  created_at: string
  updated_at: string
  config: {
    initial_state: string
    states: { name: string }[]
    transitions: FsmTransition[]
  }
}
```

---

## 组件设计

### BBKeySelector.vue

- Props: `modelValue: string`, `allowCreate: boolean = true`, `fieldType?: string`（外部传入，用于反查）
- Emits: `update:modelValue`, `select`（携带完整 FieldListItem 供外层获取 type）
- 内部：调 `fieldApi.list({ bb_exposed: true, enabled: true, page: 1, page_size: 200 })` 初始化候选列表
- El-select + `filterable` + `allow-create`（自由输入运行时 Key）
- 显示格式：`name (label)`

### FsmConditionEditor.vue（递归）

- Props: `modelValue: FsmConditionNode`, `depth: number = 0`, `disabled: boolean = false`
- Emits: `update:modelValue`
- 条件类型三选一（radio-group）：`none` / `leaf` / `group`
  - `none`：空对象 `{}`
  - `leaf`：`{ key, op, value/ref_key }`
  - `group`：`{ and/or: [...] }`
- 组合节点：AND / OR radio + 递归渲染子节点（`v-for` + `FsmConditionEditor`）
- depth >= 10 时禁用「添加子条件」按钮
- 叶节点 value 控件根据 BBKeySelector 返回的 fieldType 自适应

### FsmStateListEditor.vue

- Props: `modelValue: string[]`（state name 数组）, `initialState: string`, `disabled: boolean`
- Emits: `update:modelValue`, `update:initialState`
- 动态列表 + 重名实时检测（行内红色提示）
- initial_state 下拉与 states 数组保持同步

### FsmTransitionListEditor.vue

- Props: `modelValue: FsmTransition[]`, `states: string[]`, `disabled: boolean`
- Emits: `update:modelValue`
- 可折叠面板（el-collapse）：每条规则可折叠
- 收起态摘要：`from → to | 优先级N | 已配置条件 / 无条件`
- 嵌入 FsmConditionEditor

### FsmConfigList.vue

- 列：id / name / display_name / initial_state / state_count / enabled / created_at / 操作
- 搜索：display_name 模糊 + enabled 三态
- 操作：查看 / 编辑（已启用→ GuardDialog）/ 删除（已启用→ GuardDialog）
- el-switch 切换：先弹 confirm，成功后刷新

### FsmConfigForm.vue

- 共享 isCreate / isView 两个 meta flag
- 基本信息卡：name（新建可编辑+checkName，编辑只读）+ display_name
- 状态卡：FsmStateListEditor（含 initial_state）
- 转换规则卡：FsmTransitionListEditor
- 保存前校验：states 非空（前端拦截）
- 提交成功：跳 /fsm-configs，显示对应 toast

---

## 数据流

```
FsmConfigForm
  ├─ FsmStateListEditor  (v-model:states, v-model:initialState)
  └─ FsmTransitionListEditor (v-model:transitions)
       └─ [每条规则] FsmConditionEditor (v-model:condition)
                         └─ BBKeySelector (v-model:key / ref_key)
```

---

## EnabledGuardDialog 扩展

追加 `entityType = 'fsm-config'`：

```ts
type EntityType = 'field' | 'template' | 'event-type' | 'event-type-schema' | 'fsm-state-dict' | 'fsm-config'
```

- `entityTypeLabel`：`'状态机'`
- `reasonText` (edit)：`'已启用的状态机对游戏服务端可见，任意修改可能导致服务端拉取到不稳定配置。请先禁用，再进入编辑。'`
- `onActOnce`：调 `fsmConfigApi.detail(id)` 获取 version，再调 `fsmConfigApi.toggleEnabled(id, false, version)`
- 跳转路径：`/fsm-configs/${id}/edit`

---

## AppLayout 扩展

在 `group-fsm` sub-menu 下追加：

```html
<el-menu-item index="/fsm-configs">
  <el-icon><Operation /></el-icon>
  <span>状态机管理</span>
</el-menu-item>
```

图标使用 Element Plus 内置 `Operation`（CPU-like，已在导入中可用或追加）。

---

## 关键实现注意事项

1. **条件树序列化**：FsmConditionEditor 直接操作 `FsmConditionNode` 对象，提交时原样传给后端，后端负责校验语义
2. **value 类型**：叶节点 value 可能是 string / number / boolean，序列化到 JSON 时直接使用 JSON.stringify 的默认行为，后端 `json.RawMessage` 接收
3. **运行时 Key**：BBKeySelector `allow-create` 返回自由输入字符串，fieldType 未知，value 控件降级为文本框（el-input）
4. **reactive 解构**：FsmTransitionListEditor 内使用 `computed` getter/setter 操作 props，不直接修改 prop
5. **el-collapse-item key**：使用 index，因为转换规则无固定 id
