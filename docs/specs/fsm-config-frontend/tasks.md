# FSM Config Frontend — 任务列表

## 任务状态

| ID | 描述 | 状态 |
|----|------|------|
| T1 | 创建 `frontend/src/api/fsmConfigs.ts` | [x] |
| T2 | 创建 `frontend/src/components/BBKeySelector.vue` | [x] |
| T3 | 创建 `frontend/src/components/FsmConditionEditor.vue` | [x] |
| T4 | 创建 `frontend/src/components/FsmStateListEditor.vue` | [x] |
| T5 | 创建 `frontend/src/components/FsmTransitionListEditor.vue` | [x] |
| T6 | 创建 `frontend/src/views/FsmConfigList.vue` | [x] |
| T7 | 创建 `frontend/src/views/FsmConfigForm.vue` | [x] |
| T8 | 修改 `router/index.ts`、`AppLayout.vue`、`EnabledGuardDialog.vue` | [x] |

## 详细任务

### T1 — fsmConfigs.ts

**目标**：创建 API 文件，包含所有类型定义、错误码常量、fsmConfigApi 对象。

**涉及文件**：
- 新建 `frontend/src/api/fsmConfigs.ts`

**验收**：
- 类型与后端 model/fsm_config.go 对齐
- FSM_ERR 错误码与 errcode/codes.go 43001–43012 对齐
- fsmConfigApi 包含 list/create/detail/update/delete/checkName/toggleEnabled 七个方法

---

### T2 — BBKeySelector.vue

**目标**：BB Key 下拉选择器，调 `fieldApi.list({ bb_exposed: true, enabled: true })` 获取候选，支持 `allow-create` 自由输入。

**涉及文件**：
- 新建 `frontend/src/components/BBKeySelector.vue`

**验收**：
- 挂载时拉取 bb_exposed 字段列表
- 选项显示 `name (label)` 格式
- 支持自由输入（运行时 Key）
- emit `update:modelValue`（key name）和 `field-selected`（携带完整 FieldListItem，null=自由输入）

---

### T3 — FsmConditionEditor.vue

**目标**：递归条件编辑器，支持无条件 / 单条件 / 组合条件三态，组合条件可嵌套。

**涉及文件**：
- 新建 `frontend/src/components/FsmConditionEditor.vue`

**验收**：
- 三态 radio 切换时清空对应字段
- 组合条件支持 AND / OR，最多 10 层（超出禁用「添加子条件」）
- 叶节点 value 控件根据 fieldType 自适应（integer/float/boolean/string）
- 叶节点 value / ref_key 二选一，切换时清空另一字段
- op 枚举白名单：`== != > >= < <= in`
- disabled prop 透传给所有子控件

---

### T4 — FsmStateListEditor.vue

**目标**：状态列表动态编辑器，含重名检测和 initial_state 联动。

**涉及文件**：
- 新建 `frontend/src/components/FsmStateListEditor.vue`

**验收**：
- 可添加/删除状态行
- 实时检测重名（行内红色提示）
- initial_state 下拉与 states 同步
- 删除 initial_state 对应状态时，自动重置为 states[0]（若存在）

---

### T5 — FsmTransitionListEditor.vue

**目标**：转换规则列表编辑器，每条规则可折叠，嵌入 FsmConditionEditor。

**涉及文件**：
- 新建 `frontend/src/components/FsmTransitionListEditor.vue`

**验收**：
- 可添加/删除规则
- from/to 下拉来自 states prop
- el-collapse 折叠，收起态摘要：`from → to | 优先级N | 已配置条件/无条件`
- priority el-input-number，min=0，默认 0
- 嵌入 FsmConditionEditor（v-model:condition）

---

### T6 — FsmConfigList.vue

**目标**：状态机列表页，含搜索筛选、启用切换、编辑/删除守卫。

**涉及文件**：
- 新建 `frontend/src/views/FsmConfigList.vue`

**验收（对应需求 R1–R5）**：
- 七列展示（R1）
- display_name 模糊 + enabled 三态筛选（R2）
- el-switch 切换前 confirm，成功刷新（R3）
- 已启用时编辑/删除调 EnabledGuardDialog（R4）
- 空列表 el-empty + 「新建状态机」快捷按钮（R5）

---

### T7 — FsmConfigForm.vue

**目标**：新建/编辑/查看三合一表单页，含 name 校验、状态编辑器、转换规则编辑器、全套错误处理。

**涉及文件**：
- 新建 `frontend/src/views/FsmConfigForm.vue`

**验收（对应需求 R6–R27）**：
- route.meta 区分三种模式（R6）
- 新建 name 失焦 checkName（R7）
- 编辑/查看 name 只读+Lock 图标（R8）
- 查看模式 el-form disabled，footer 隐藏（R9）
- 保存成功 toast + 跳转（R10）
- 状态/转换/条件编辑器联动（R11–R23）
- 错误码处理（R24–R27）

---

### T8 — router + AppLayout + EnabledGuardDialog

**目标**：接入路由、侧边栏菜单、EnabledGuardDialog 的 fsm-config 分支。

**涉及文件**：
- 修改 `frontend/src/router/index.ts`
- 修改 `frontend/src/components/AppLayout.vue`
- 修改 `frontend/src/components/EnabledGuardDialog.vue`

**验收**：
- 4 条路由（list/create/view/edit）已注册
- 侧边栏 group-fsm 下有「状态机管理」菜单项，点击跳转 /fsm-configs
- EnabledGuardDialog EntityType 联合类型追加 'fsm-config'
- onActOnce 处理 fsm-config 分支：detail→toggleEnabled→跳转 /fsm-configs/:id/edit
