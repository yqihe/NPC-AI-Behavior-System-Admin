# NPC 管理 — 前端任务拆解

> spec-create Phase 3 产出（前端）。
> 审批后从 main 拉分支：`git checkout -b feature/npc-management-frontend`
>
> **一致性锚点**：`design.md` 第二节 API Contract。
> **后端任务**：`tasks-backend.md`（独立分支）。
> 前端依赖 API Contract 定义的类型和接口，不依赖后端代码；后端 API 就绪前可用 mock 数据联调。
>
> 每个任务完成后执行 `/verify` 再继续。

---

## 参考文件

开始前必读：

| 文件 | 目的 |
|------|------|
| `docs/architecture/frontend-conventions.md` | 列表页/表单页骨架、列定义、交互模式 |
| `docs/development/standards/red-lines/frontend.md` | 前端禁止红线 |
| `docs/development/admin/red-lines.md` §6、§7、§8、§12、§13 | ADMIN 专属前端红线 |
| `frontend/src/views/TemplateList.vue` | 列表页参考（含 EnabledGuardDialog）|
| `frontend/src/views/TemplateForm.vue` | 复杂表单参考（动态字段渲染）|
| `frontend/src/views/FsmConfigForm.vue` | FSM 配置表单参考 |
| `frontend/src/api/templates.ts` | API 文件结构参考 |
| `design.md §2` | **API Contract**：所有接口的 TypeScript 类型定义 |

---

## 依赖顺序

```
F1 → F2
F1 → F3 → F4
F4 + F2 → F5
```

---

## ✅ F1：API 层（`api/npc.ts`）

**关联需求**：R1–R7、R17（前端所有接口调用的基础）

**涉及文件**：
- `frontend/src/api/npc.ts`

**做完后是什么样**：
文件结构与已有 `api/templates.ts` 一致：**类型定义区** → **错误码常量区** → **api 对象区**。

**类型定义**（严格对齐 design.md §2 的 TypeScript 类型）：

```ts
// 列表
export interface NPCListQuery { name?, label?, template_name?, enabled?, page, page_size }
export interface NPCListItem { id, name, label, template_name, template_label, fsm_ref, enabled, created_at }

// 详情
export interface NPCDetail { id, name, label, description, template_id, template_name, template_label, enabled, version, fields: NPCDetailField[], fsm_ref, bt_refs }
export interface NPCDetailField { field_id, name, label, type, category, category_label, enabled, required, value }

// 创建/编辑
export interface NPCFieldValue { field_id: number, value: number | string | boolean | null }
export interface CreateNPCRequest { name, label, description?, template_id, field_values: NPCFieldValue[], fsm_ref?, bt_refs? }
export interface UpdateNPCRequest { id, label, description?, field_values: NPCFieldValue[], fsm_ref?, bt_refs?, version }
export interface CreateNPCResponse { id: number, name: string }
```

**错误码常量**：
```ts
export const NPC_ERRORS = {
  NAME_EXISTS: 45001, NAME_INVALID: 45002, NOT_FOUND: 45003,
  TEMPLATE_NOT_FOUND: 45004, TEMPLATE_DISABLED: 45005,
  FIELD_VALUE_INVALID: 45006, FIELD_REQUIRED: 45007,
  FSM_NOT_FOUND: 45008, FSM_DISABLED: 45009,
  BT_NOT_FOUND: 45010, BT_DISABLED: 45011,
  BT_STATE_INVALID: 45012, DELETE_NOT_DISABLED: 45013,
  VERSION_CONFLICT: 45014, BT_WITHOUT_FSM: 45015,
} as const
```

**api 对象**：
```ts
export const npcApi = {
  list(q: NPCListQuery): Promise<ListData<NPCListItem>>
  create(req: CreateNPCRequest): Promise<CreateNPCResponse>
  detail(id: number): Promise<NPCDetail>
  update(req: UpdateNPCRequest): Promise<void>
  delete(id: number): Promise<DeleteResult>
  checkName(name: string): Promise<CheckNameResult>
  toggleEnabled(id: number, enabled: boolean, version: number): Promise<void>
}
```

`ListData<T>` / `CheckNameResult` / `DeleteResult` 从 `api/fields.ts` 导入（red-lines §10.7）。

---

## ✅ F2：NPC 列表页（`NPCList.vue`）

**关联需求**：R18（含启用/停用/删除交互）

**涉及文件**：
- `frontend/src/views/NPCList.vue`

**做完后是什么样**：

**列定义**（按约定顺序，`frontend-conventions.md §二`）：

| 列名 | 字段 | 宽度 | 备注 |
|------|------|------|------|
| ID | id | 80 | |
| NPC 标识 | name | 160 | 等宽字体 |
| 中文标签 | label | — | 自适应 |
| 所用模板 | template_label | 160 | 空则灰色"—"|
| 行为状态机 | fsm_ref | 140 | 空串显示灰色"—" |
| 启用 | enabled | 80 | el-switch |
| 创建时间 | created_at | 170 | `formatTime` |
| 操作 | — | 160 | `fixed="right"` |

**筛选栏**：搜索中文标签（`filter-item-wide`）+ 搜索NPC标识（`filter-item`）+ 所用模板标识（精确，`filter-item`）+ 启用状态下拉 + 搜索/重置

**停用行视觉**：`row-disabled`，倒数 3 列（启用/创建时间/操作）不降透明度

**操作列**：`编辑`（primary）+ `删除`（danger），均为 `el-link`，`:underline="false"`

**启用/停用交互**（`frontend-conventions.md §四`）：
- 点击开关 → `ElMessageBox.confirm`（启用 type=success，停用 type=warning）
- 确认后先 `npcApi.detail(id)` 拿最新 version → `npcApi.toggleEnabled`
- 版本冲突 → `ElMessageBox.alert`（提示刷新）

**删除交互（模式 B）**：
- **启用中**：直接弹 `EnabledGuardDialog`（action='delete'），不发删除请求
- **停用中**：弹确认弹窗 → 调 `npcApi.delete(id)` → 收到 `45013` 则再弹 `EnabledGuardDialog` 作兜底
- `EnabledGuardDialog` 的"立即禁用"只停用，停用后刷新列表让用户再点删除（red-lines §8.3）

**空状态**：`el-empty` + "暂无 NPC 数据" + "新建 NPC" 按钮

---

## ✅ F3：行为配置面板（`BehaviorConfigPanel.vue`）

**关联需求**：R20（FSM 选择 + bt_refs 动态行）

**涉及文件**：
- `frontend/src/components/BehaviorConfigPanel.vue`

> 独立为子组件，NPCForm.vue 引用它（F4 依赖 F3）。

**做完后是什么样**：

**Props**：
```ts
props: {
  modelValue: { fsm_ref: string, bt_refs: Record<string, string> },
  disabled: boolean,          // 查看模式
  fsmList: FsmListItem[],     // 父组件已加载的 FSM 下拉选项
  btList: BtListItem[],       // 父组件已加载的 BT 下拉选项
}
emits: ['update:modelValue']
```

> 下拉数据由 NPCForm.vue 加载并传入，Panel 只负责渲染 + emit，不自己调 API。

**FSM 选择区**：
- `el-select` 展示 `fsmList`（已启用 FSM），`display_name (name)` 格式
- 下拉为空时警告 + 文字链接跳转 `/fsm-configs`（red-lines §7.1）
- 选中 FSM 后：emit 更新 `fsm_ref`；清空并重新渲染 bt_refs 行

**BT 引用动态表**（FSM 选中后显示）：
- 每行：`{state_name} (FSM 状态)` 标签 + BT 下拉（`btList` 选项）
- BT 下拉允许清空（`clearable`），置空表示该状态不绑 BT
- FSM 的 states 列表由父组件传入（NPCForm.vue 在 FSM detail 拿到后注入）
- 整体 `v-if="modelValue.fsm_ref"` —— 无 FSM 时不显示 bt 表

**查看模式（`disabled=true`）**：
- `el-select` 全部禁用（注意 red-lines §12：子组件 `:disabled="disabled || condition"`）
- bt_refs 空时显示灰色"未配置行为"

---

## ✅ F4：NPC 表单页（`NPCForm.vue`）

**关联需求**：R19、R20（新建/编辑共用）

**涉及文件**：
- `frontend/src/views/NPCForm.vue`

**做完后是什么样**：

**路由模式**（与 TemplateForm.vue 一致）：
- `route.meta.isCreate`：新建模式
- `route.meta.isView`：查看模式（当前不使用，但预留接口）
- 其余：编辑模式

**三卡片区块布局**（`form-layout.css`）：

**卡片 A — 基本信息**：
| 字段 | 新建 | 编辑 |
|------|------|------|
| NPC 标识 | 可编辑 + 异步唯一性校验（checkNameUnique，防抖 400ms）| 禁用 + lock 图标 |
| 中文标签 | 必填 | 必填 |
| 描述 | textarea，可选 | textarea |

**卡片 B — 字段值配置**：
1. **模板选择**（新建时）：
   - `el-select` 调 `templateApi.list({enabled: true})`
   - 选项格式：`{label} ({name})`
   - 下拉为空时 → 警告 + "去模板管理" 链接（red-lines §7.1）
   - 选中后：调 `templateApi.detail(id)` → 拿 `fields` 列表 → 渲染动态字段区；同时清空 `field_values`

2. **模板展示**（编辑时）：
   - 灰底 `el-input` 只读，展示 `template_label (template_name)`；下方提示"模板选择后不可更改"

3. **动态字段区**（模板选定后）：
   - 按 `templateDetail.fields` 数组顺序逐一渲染
   - 每个字段调用已有 `SchemaForm` 组件（`components/SchemaForm.vue`），传入 `field.properties`、当前 `value`、`disabled=isView`
   - 必填字段（`required=true`）：label 前红星 `*`；提交时前端校验非空（value ≠ null/""）
   - 编辑时停用字段（`field.enabled=false`）：灰色 ⚠️ 图标 + "此字段已停用，值保留但不可修改" → `disabled=true`

**卡片 C — 行为配置**：
- 引用 `<BehaviorConfigPanel>` 组件（F3）
- 父组件在 `onMounted` / 模板选定后一次性加载 `fsmList` + `btList`
- 编辑回填：从 `NPCDetail.fsm_ref` + `NPCDetail.bt_refs` 初始化
- FSM 选中时调 `fsmConfigApi.detail(id)` 拿 states 列表，传入 Panel

**提交逻辑**：
- 新建：
  1. 前端校验（name 唯一性结果 + 必填字段 + template_id）
  2. 调 `npcApi.create`
  3. `.catch` 逐一处理：45001→"NPC标识已存在"；45005→"模板未启用，请刷新"；45007→"必填字段未填"；45006→"字段值不符合约束，请检查输入"；45008/45009→"状态机不可用"；45010/45011→"行为树不可用"；45012→"行为树绑定的状态名与状态机不匹配"；45015→"选择行为树前请先选择状态机"（`// 拦截器已 toast` for others）
  4. 成功 → `ElMessage.success("创建成功，NPC 已默认启用")` → 跳转 `/npcs`

- 编辑：
  1. 前端校验（必填字段 + 非空）
  2. 调 `npcApi.update`
  3. `.catch`：45014→`ElMessageBox.alert`（提示版本冲突，返回列表刷新）；其余同创建
  4. 成功 → `ElMessage.success("保存成功")` → 跳转 `/npcs`

**加载回填（编辑）**：
- `onMounted`：`npcApi.detail(id)` → 回填 label/description/field_values/fsm_ref/bt_refs
- 字段 value 从 `NPCDetail.fields[i].value` 读取；建 `Map<field_id, value>` 供 SchemaForm 使用

---

## ✅ F5：路由注册 + 侧边栏入口

**关联需求**：R18–R20（页面可访问）

**涉及文件**：
- `frontend/src/router/index.ts`
- `frontend/src/components/AppLayout.vue`

**做完后是什么样**：

**`router/index.ts`** 新增 4 条路由：
```ts
{ path: '/npcs',        component: () => import('@/views/NPCList.vue') },
{ path: '/npcs/create', component: () => import('@/views/NPCForm.vue'), meta: { isCreate: true } },
{ path: '/npcs/:id/edit', component: () => import('@/views/NPCForm.vue') },
{ path: '/npcs/:id/view', component: () => import('@/views/NPCForm.vue'), meta: { isView: true } },
```

**`AppLayout.vue`** 在 `group-npc` el-sub-menu 的现有列表页链接中，在"模板管理"之前插入：
```html
<el-menu-item index="/npcs">
  <el-icon><Grid /></el-icon>
  <span>NPC 管理</span>
</el-menu-item>
```
（位置：模板管理 `/templates` 之前，保持"NPC→模板→字段"的配置层级顺序）

---

## 验收检查清单

前端所有任务完成后执行 `/verify`，确认：

- [ ] `npx vue-tsc --noEmit` 零类型错误
- [ ] 侧边栏可见"NPC 管理"菜单项，点击跳转 `/npcs`
- [ ] NPC 列表正常加载、分页、筛选（名称/标签/模板/启用状态）
- [ ] 停用行 opacity 0.5，操作列保持高亮
- [ ] 新建 NPC：选模板 → 字段动态渲染 → 必填标星 → 提交成功 → 列表出现新行
- [ ] 新建 NPC 提交成功提示包含"已默认启用"字样
- [ ] 编辑 NPC：字段值/FSM/BT 正确回填 → 修改 → 保存成功
- [ ] 停用字段在编辑页显示 ⚠️ 图标 + 禁用输入
- [ ] 删除启用中 NPC → EnabledGuardDialog 弹出
- [ ] 删除停用中 NPC → 确认弹窗 → 删除成功
- [ ] 行为配置：FSM 下拉为空时展示警告 + 跳转链接
- [ ] FSM 切换后 bt_refs 行刷新（旧状态名清除）
- [ ] 所有表单错误码 catch 块均有定向处理（不只依赖全局 toast）
