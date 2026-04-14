# 事件类型管理 — 前端页面开发 · 需求分析

> 对应文档：
> - 功能：[features.md](../../v3-PLAN/行为管理/事件类型/features.md)
> - 前端设计：[frontend.md](../../v3-PLAN/行为管理/事件类型/frontend.md)
> - 后端 spec：[../event-type-management/](../event-type-management/)
> - UI Mockup：[mockups-event-type.pen](../../v3-PLAN/mockups-event-type.pen)
>
> **范围**：仅前端（views / api / components / router）。后端已完成并合入 main。

---

## 动机

事件类型是行为管理的基础配置——FSM 转换条件和 BT 节点都需要引用事件类型。后端 7 个 API 端点已完成（list / create / detail / update / delete / check-name / toggle-enabled），但**没有前端页面**，策划无法可视化管理事件类型，只能通过 curl 操作。

不做的后果：
- 行为管理链路（事件类型 → 状态机 → 行为树）的第一环缺失，后续模块前端无法推进
- 策划无法验证后端已实现的事件类型配置功能

## 优先级

**高**。当前处于 V3 前端按模块推进阶段，字段管理和模板管理前端已完成。事件类型是下一个必须完成的模块，阻塞 FSM/BT 前端开发。

## 预期效果

### 场景 1：列表页浏览与筛选
策划打开「事件类型」菜单项，看到事件类型列表，支持：
- 按中文名称模糊搜索
- 按感知模式（Visual / Auditory / Global）下拉筛选
- 按启用状态下拉筛选
- 后端分页浏览
- 感知模式用彩色 Tag 区分（蓝=Sound/Auditory, 绿=Visual, 橙=Proximity(预留), 灰=Global）
- 已停用行整行半透明（opacity 0.5，操作列除外）
- 启用开关可直接点击切换（走乐观锁，先 detail 拿 version 再 toggle）

### 场景 2：新建事件类型
策划点击「新建事件类型」按钮，进入表单页：
- **基本信息卡片**：事件标识（blur 校验唯一性）、中文名称、感知模式（Select）、默认严重度（0-100）、默认 TTL（>0）、感知范围（>=0，Global 模式提示自动置 0）
- **扩展字段卡片**：根据 detail 接口返回的 `extension_schema` 动态渲染表单控件（int/float/string/bool/select），未交互过的扩展字段不进提交 payload
- 保存后跳回列表页

### 场景 3：编辑事件类型
策划在列表点「编辑」：
- 如果事件类型启用中 → 弹 EnabledGuardDialog，可「立即停用」后跳编辑页
- 如果已停用 → 直接跳编辑页
- 编辑页与新建共用组件，区别：标识符只读、标题为"编辑事件类型"、加载 detail 回填表单
- 乐观锁保存，版本冲突弹 alert 提示刷新

### 场景 4：删除事件类型
策划在列表点「删除」：
- 如果启用中 → 弹 EnabledGuardDialog（delete 模式）
- 如果已停用 → 弹 ElMessageBox 确认（明确显示事件名称）
- 删除成功刷新列表

## 依赖分析

### 已完成的依赖
- ✅ 后端 7 个 API 端点（list / create / detail / update / delete / check-name / toggle-enabled）
- ✅ `EnabledGuardDialog.vue` 共享组件（需扩展支持 `event-type`）
- ✅ `frontend/src/api/request.ts` 请求模块
- ✅ 前端路由体系（`router/index.ts`）
- ✅ `AppLayout.vue` 侧栏菜单（需新增事件类型菜单项）
- ✅ UI Mockup（`mockups-event-type.pen`）

### 谁依赖本需求
- FSM 前端（转换条件需要选择事件类型）
- BT 前端（节点配置可能引用事件类型）

## 改动范围

| 变更类型 | 文件路径 | 说明 |
|---------|---------|------|
| 新增 | `frontend/src/api/eventTypes.ts` | API 类型定义 + 错误码 + 请求函数 |
| 新增 | `frontend/src/views/EventTypeList.vue` | 列表页 |
| 新增 | `frontend/src/views/EventTypeForm.vue` | 新建/编辑表单页 |
| 修改 | `frontend/src/components/EnabledGuardDialog.vue` | 扩展 EntityType 支持 `event-type` |
| 修改 | `frontend/src/router/index.ts` | 新增 3 条路由 |
| 修改 | `frontend/src/components/AppLayout.vue` | 侧栏新增事件类型菜单项 |

预计 3 个新增文件 + 3 个修改文件，共 6 个文件。

## 扩展轴检查

**新增配置类型扩展轴**：✅ 正面。事件类型是行为管理分类的第一个前端模块，验证"新增配置类型只需加一组 views/api + 修改 router/sidebar"的模式。与字段/模板模块完全同构，不需要改已有模块代码（除 EnabledGuardDialog 泛型扩展和 sidebar 菜单项）。

**新增表单字段扩展轴**：✅ 正面。扩展字段通过 `extension_schema` 动态渲染，验证 SchemaForm 模式——新增表单字段只需在 Schema 管理页定义，无需修改事件类型前端代码。

## 验收标准

### 列表页
- **R1**：`/event-types` 路由可达，侧栏「事件类型」菜单项高亮
- **R2**：列表展示 ID / 事件标识 / 中文名称 / 感知模式(Tag) / 严重度 / TTL / 范围 / 启用开关 / 创建时间 / 操作列
- **R3**：感知模式 Tag 颜色区分——Visual 绿底绿字、Auditory 蓝底蓝字、Global 灰底灰字
- **R4**：搜索框输入中文标签 + 搜索按钮 → 列表按 `display_name` 模糊过滤（后端分页）
- **R5**：感知模式下拉筛选 + 启用状态下拉筛选 → 精确过滤
- **R6**：重置按钮清空所有筛选并重新加载
- **R7**：分页组件联动后端分页
- **R8**：已停用行 opacity 0.5（操作列除外）
- **R9**：点击启用开关 → 先 detail 拿 version → 调 toggleEnabled → 刷新列表
- **R10**：启用/停用 toggle 版本冲突 → 提示刷新

### 新建页
- **R11**：`/event-types/create` 路由可达，SubHeader 显示「返回 | 新建事件类型」
- **R12**：事件标识输入框 blur 时异步调 check-name，显示"校验中 / 标识符可用 / 该事件标识已存在"
- **R13**：事件标识格式校验 `^[a-z][a-z0-9_]*$`
- **R14**：感知模式 Select 选项从后端 perception_mode 枚举（visual / auditory / global）
- **R15**：默认严重度 input-number，0-100 范围校验
- **R16**：默认 TTL input-number，>0 校验
- **R17**：感知范围 input-number，>=0 校验，选择 Global 时提示"自动置为 0"
- **R18**：扩展字段卡片根据 `extension_schema` 动态渲染（int/float/string/bool/select），schema 为空时不显示卡片
- **R19**：未交互过的扩展字段不进提交 payload
- **R20**：保存成功 → ElMessage.success + 跳转列表页
- **R21**：所有必填字段为空时阻止提交

### 编辑页
- **R22**：`/event-types/:id/edit` 路由可达，SubHeader 显示「返回 | 编辑事件类型」
- **R23**：加载 detail 回填表单，事件标识字段只读（disabled + Lock 图标）
- **R24**：编辑保存携带 version 字段，版本冲突 → ElMessageBox.alert 提示刷新
- **R25**：扩展字段值从 config 中回填，已有值标记为 dirty

### EnabledGuardDialog
- **R26**：列表页点击已启用事件类型的「编辑」→ 弹 EnabledGuardDialog（edit 模式），文案适配事件类型
- **R27**：列表页点击已启用事件类型的「删除」→ 弹 EnabledGuardDialog（delete 模式）
- **R28**：「立即停用」成功后 edit 模式 → 跳编辑页，delete 模式 → 仅刷新列表（不自动删除）
- **R29**：停用时版本冲突 → 提示刷新

### 集成
- **R30**：`AppLayout.vue` 侧栏新增「事件类型」菜单项，在「配置管理」分组下
- **R31**：`npx vue-tsc --noEmit` 通过，无类型错误

## 不做什么

- ❌ **不做**扩展字段 Schema 管理页（SchemaManagement.vue）—— 另起 spec
- ❌ **不做**事件类型被 FSM/BT 引用的关联显示（ref_count 本期恒 0）
- ❌ **不做**事件类型导入/导出 UI
- ❌ **不做**列排序功能
- ❌ **不做**事件类型克隆功能
- ❌ **不做**扩展字段约束面板编辑（属于 Schema 管理 spec 范围）
