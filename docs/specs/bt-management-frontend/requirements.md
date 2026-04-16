# bt-management-frontend — 需求分析

## 动机

后端行为树管理（`bt-management-backend`）已完整实现，提供了 `bt_trees` 和 `bt_node_types` 两张表的完整 CRUD API。目前没有前端入口，运营人员无法通过 ADMIN 平台创建和管理 NPC 行为树配置。

不做的后果：
- NPC 行为逻辑无法通过 ADMIN 写入，导出接口 `GET /api/configs/bt_trees` 永远返回空列表
- 游戏服务端联调无法验证行为树配置的完整链路

## 优先级

**高**。后端已完成，这是最后一块缺失的 V3 核心配置模块前端。行为树是 NPC 行为逻辑的最终表达，策划填完字段→模板→FSM 后，必须有行为树才能完成一个可运行的 NPC 配置闭环。

## 预期效果

**场景 1 — 运营新建行为树**

运营进入"行为树管理"列表页，点击"新建行为树"，填写标识（如 `wolf/attack`）、中文名（如"狼攻击行为"），然后在树编辑器中：
1. 添加根节点，选择类型"序列 (sequence)"
2. 在序列节点下 Add Child，选"检查浮点 BB (check_bb_float)"，填写 key=`player_distance`、op=`<`、value=`5`
3. 再 Add Child，选"存根动作 (stub_action)"，填写 name=`melee_attack`、result=`success`
4. 保存，系统提示"创建成功，默认为禁用状态"

**场景 2 — 运营启用行为树**

列表页找到 `wolf/attack`，点 Switch 切换启用，确认弹窗提示"启用后游戏服务端可拉取该行为树"，确认后变为启用状态。

**场景 3 — 开发者添加自定义节点类型**

进入"系统设置 > 节点类型"，新建节点类型：type_name=`patrol_action`、category=`leaf`、label=`巡逻动作`，添加参数 `radius`（类型 float，必填）。保存后，树编辑器的节点选择器中出现"巡逻动作 (patrol_action)"选项。

**场景 4 — 查看模式只读**

对已启用行为树点击"查看"，树编辑器整体 disabled，所有节点参数只读，不显示 Add/Delete/Edit 按钮。

## 依赖分析

**依赖（已完成）**：
- `bt-management-backend`：提供全部 API 端点
- `BBKeySelector.vue`：树编辑器中 bb_key 类型参数复用此组件
- `EnabledGuardDialog.vue`：需扩展支持 `bt-tree` 和 `bt-node-type` 两个 entityType
- `list-layout.css` / `form-layout.css`：全局布局 CSS 已就绪
- `fields.ts`：`ListData<T>` / `CheckNameResult` 类型导出源

**被依赖**：
- NPC 管理（未实现）：NPC 的 `bt_refs` 字段需要从行为树列表动态获取，需要 `btTreeApi.listAll()` 或 `btTreeApi.list()` 的 enabled 筛选

## 改动范围

| 类型 | 文件 | 说明 |
|------|------|------|
| 新建 | `frontend/src/api/btTrees.ts` | 行为树 API 层 |
| 新建 | `frontend/src/api/btNodeTypes.ts` | 节点类型 API 层 |
| 新建 | `frontend/src/views/BtTreeList.vue` | 行为树列表页 |
| 新建 | `frontend/src/views/BtTreeForm.vue` | 行为树新建/编辑/查看 |
| 新建 | `frontend/src/views/BtNodeTypeList.vue` | 节点类型列表页 |
| 新建 | `frontend/src/views/BtNodeTypeForm.vue` | 节点类型新建/编辑/查看 |
| 新建 | `frontend/src/components/BtNodeEditor.vue` | 树编辑器核心组件 |
| 新建 | `frontend/src/components/BtNodeTypeSelector.vue` | 节点类型选择对话框 |
| 新建 | `frontend/src/components/BtParamSchemaEditor.vue` | param_schema 行编辑器 |
| 修改 | `frontend/src/components/EnabledGuardDialog.vue` | 新增 bt-tree / bt-node-type entityType |
| 修改 | `frontend/src/router/index.ts` | 注册 8 条路由 |

合计：9 新建 + 2 修改 = 11 个文件。

## 扩展轴检查

**新增配置类型（行为树作为一类新配置）**：本次新增行为树前端，完全独立的 api/views/components 层，**不修改任何已有模块代码**（除扩展 GuardDialog 的 entityType map），符合"新增配置类型只需加一组文件"的扩展轴。

**新增表单字段（BtNodeEditor 动态渲染）**：BtNodeEditor 内联表单按 `param_schema.params[]` 动态渲染，新增 param type 只需在 `BtNodeEditor` 的 `renderParamInput()` 里加一个 case，不改 API 层或 Views 层，符合"新增表单字段只需加组件/case"的扩展轴。

## 验收标准

### 行为树管理
- **R1** 列表页按 name 模糊、display_name 模糊、enabled 状态组合筛选，后端分页，20 条/页，`el-empty` 空态含"新建行为树"引导按钮
- **R2** 新建：填 name（`^[a-z][a-z0-9_/]*$`）+ display_name + description，name 失焦调 check-name，格式不合法/已占用显示红色提示
- **R3** 新建：树编辑器可从空树添加根节点，支持 composite/decorator/leaf 三类节点的嵌套构建，保存成功后提示"创建成功，默认为禁用状态"
- **R4** 编辑：已启用行为树点编辑触发 EnabledGuardDialog；禁用状态直接进编辑页，name 字段锁定，乐观锁版本冲突弹警告
- **R5** 查看：树编辑器整体只读，所有控件 disabled，不显示 Add/Delete/Edit 按钮，不显示 form-footer
- **R6** 启用/禁用：Switch 操作先弹确认弹窗，确认前先 detail 拿最新 version，VERSION_CONFLICT 弹警告并刷新
- **R7** 删除：启用中不可删除（GuardDialog）；禁用状态弹确认后 delete，禁用行 opacity 排除最后 3 列
- **R8** name 格式允许 `/` 作为命名空间分隔符（`^[a-z][a-z0-9_/]*$`），表单下方有灰色提示说明格式

### 节点类型管理
- **R9** 列表页支持按 type_name 模糊、label 模糊、category（composite/decorator/leaf）、enabled 筛选；category 列用 `el-tag` 展示
- **R10** 内置节点（is_builtin=1）操作列只显示"查看"，无编辑/删除按钮
- **R11** 新建自定义节点类型：填 type_name/category/label/description + param_schema 参数列表（name/label/type/required，select 类型可填 options）
- **R12** param_schema 编辑器支持增减参数行，type 为 select 时显示 options 输入（逗号分隔），`defineExpose({ validate })` 暴露校验方法
- **R13** 删除节点类型：后端检查 bt_tree 使用情况，前端 IN_USE 错误时 ElMessage.error 显示"该节点类型已被行为树使用，请先删除相关节点"
- **R14** 启用/禁用：与行为树相同的 Switch + 确认弹窗模式

### 树编辑器（BtNodeEditor）
- **R15** 编辑器打开时从 `GET /api/v1/bt-node-types?enabled=1` 加载节点类型列表
- **R16** 节点类型选择器按 composite/decorator/leaf 三组展示，显示格式"中文标签 (type_name)"
- **R17** composite 节点渲染 Add Child 按钮 + children 递归；decorator 节点渲染 Set Child 按钮 + 单子节点位；leaf 节点渲染内联参数表单
- **R18** bb_key 类型参数使用 BBKeySelector 组件；select 类型用 el-select（options 来自 param_schema）；其余用 el-input
- **R19** 节点卡片头部显示 `[category标签] label (type_name) — params摘要`，每层缩进 24px
- **R20** 删除节点同时删除所有子孙；view 模式下 disabled=true，不渲染 Add/Edit/Delete 控件
- **R21** 编辑器内部数据为 `BtNode` 递归结构，序列化时 params 展开到节点对象顶层（无 `params` 包装层），与后端 config JSON 格式完全一致

### 路由
- **R22** 注册 8 条路由：`/bt-trees`（list）、`/bt-trees/create`、`/bt-trees/:id/view`、`/bt-trees/:id/edit`，`/bt-node-types` 同结构；meta 含 `isCreate`/`isView` 双字段

## 不做什么

- 不做行为树复制/克隆（BT-D2，毕设后）
- 不做节点拖拽排序（BT-D3，毕设后）
- 不做运行时 BB Key 表管理（BT-D4，毕设后）
- 不做 NPC bt_refs 引用检查（BT-D1，等 NPC 管理实现后补充）
- 不做行为树可视化图形编辑器（缩进卡片已满足需求）
- 不做 param_schema 的 JSON 原文编辑模式（结构化行编辑器已足够）
