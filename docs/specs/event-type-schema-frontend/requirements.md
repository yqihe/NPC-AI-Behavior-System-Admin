# 扩展字段 Schema 管理前端 — 需求分析

## 动机

事件类型的扩展字段 Schema 后端已完成（5 个 API），但策划只能通过 curl 管理扩展字段定义。这意味着：

- 策划无法自助增删改扩展字段，每次变更都需要开发介入
- 事件类型模块"后端完成但前端缺一块"，模块未闭环
- 后续 FSM/BT 模块也会引用事件类型，扩展字段不完整会影响下游

## 优先级

**高** — 这是事件类型模块的最后一块拼图，工作量小（纯前端，后端零改动），完成后事件类型模块彻底闭环，可以干净地进入 FSM 开发。

## 预期效果

### 场景 1：策划新增扩展字段

策划进入"事件类型 > 扩展字段"页面 → 点击"新建" → 填写字段标识 `custom_range`、中文名"自定义范围"、类型 `int`、约束 `{min: 0, max: 1000}`、默认值 `100`、排序 `10` → 提交 → 列表刷新，新字段出现。之后在事件类型创建/编辑页面，表单底部自动出现"自定义范围"输入框。

### 场景 2：策划编辑扩展字段

策划在列表点击"编辑" → 字段标识和类型灰显不可改 → 修改中文名或约束 → 提交成功。

### 场景 3：策划禁用后删除

策划先点击"禁用"开关 → 状态变灰 → 再点击"删除" → 确认对话框说明影响 → 删除成功。若直接删除启用状态的字段，提示"需先禁用"。

### 场景 4：策划查看只读详情

策划在列表点击"查看" → 进入只读详情页，所有字段禁用，无提交按钮。

## 依赖分析

### 上游依赖（已完成）

- 后端 5 个 API：list / create / update / delete / toggle-enabled
- 前端 API 层：`eventTypeApi.schemaListEnabled()` 已存在（需补充完整 CRUD 函数）
- 约束渲染组件：EventTypeForm.vue 中已有按 field_type 渲染约束的逻辑（可参考复用）
- 字段管理前端：FieldList.vue / FieldForm.vue 提供完整 CRUD 模式参照

### 下游被依赖

- EventTypeForm.vue 已在使用 `schemaListEnabled()` 动态渲染扩展字段，本需求不影响该逻辑
- FSM 模块开发前，事件类型模块需完整闭环

## 改动范围

**纯前端改动，后端零改动。**

| 类型 | 文件 | 说明 |
|------|------|------|
| 新增 | `frontend/src/views/EventTypeSchemaList.vue` | 列表页 |
| 新增 | `frontend/src/views/EventTypeSchemaForm.vue` | 创建/编辑/查看表单页 |
| 修改 | `frontend/src/api/eventTypes.ts` | 补充 schema CRUD API 函数 |
| 修改 | `frontend/src/router/index.ts` | 新增路由 |
| 修改 | `frontend/src/components/AppLayout.vue` | 侧边栏菜单新增入口 |

预估 **2 新增 + 3 修改 = 5 个文件**。

## 扩展轴检查

- **新增配置类型**：不涉及（Schema 是事件类型的附属，不是独立配置类型）
- **新增表单字段**：正面影响 — Schema 本身就是"动态表单字段"的管理入口，完成后策划可以自助定义新的事件类型表单字段

## 验收标准

| 编号 | 标准 | 验证方式 |
|------|------|----------|
| R1 | 列表页展示所有扩展字段定义，包含：字段标识、中文名、类型、启用状态、排序、操作 | 页面加载后目视确认 |
| R2 | 列表支持按启用状态筛选（全部/启用/禁用） | 切换筛选下拉后列表数据变化 |
| R3 | 创建表单包含：field_name（blur 唯一性校验）、field_label、field_type（下拉）、动态约束面板、default_value、sort_order | 填写并提交成功 |
| R4 | 约束面板按 field_type 动态切换：int/float 显示 min/max，string 显示 minLength/maxLength，select 显示 options 列表，bool 无约束 | 切换类型后约束面板变化 |
| R5 | 编辑表单中 field_name 和 field_type 不可修改（灰显） | 进入编辑页确认字段 disabled |
| R6 | 查看页所有字段只读，无提交按钮 | 进入查看页确认 |
| R7 | 启用/禁用切换通过列表操作完成，使用乐观锁（version） | 切换后刷新确认状态 |
| R8 | 删除操作需先禁用；删除前弹出确认框，明确说明对象名称 | 尝试删除启用的字段，提示需先禁用 |
| R9 | 侧边栏在"事件类型"下方新增"扩展字段"菜单项 | 侧边栏目视确认 |
| R10 | 路由：/event-type-schemas（列表）、/event-type-schemas/create、/event-type-schemas/:id/view、/event-type-schemas/:id/edit | URL 导航确认 |
| R11 | `npx vue-tsc --noEmit` 类型检查通过 | 命令行执行通过 |
| R12 | 版本冲突（42030）时提示用户刷新页面 | 模拟并发编辑 |

## 不做什么

- **不做后端改动** — 5 个 API 已就绪
- **不做批量操作** — 不做批量启用/禁用/删除
- **不做拖拽排序** — sort_order 通过输入框手填，不做拖拽
- **不做 field_name 唯一性的后端新接口** — 直接用 create API 的 42020 错误码判断（或在 submit 时处理），不额外加 checkName 接口
- **不做导入导出** — 延后功能
