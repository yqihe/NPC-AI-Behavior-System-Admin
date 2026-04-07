# 需求 6：FSM/BT 编辑器 Schema 化

## 动机

FSM 和 BT 页面目前使用 GenericForm 的 JSON 编辑器模式，运营人员需要手写 JSON 来配置状态机和行为树。V2 有专用的可视化编辑器（ConditionEditor / BtNodeEditor），但在需求 0 中被清除了。

现在 node-type-schemas（8 个 BT 节点类型）和 condition-type-schemas（2 个 FSM 条件类型）已导入 MongoDB 并有只读 API，需要基于这些 schema 重建编辑器。

**不做会怎样**：FSM/BT 页面无法使用，运营人员需手写递归 JSON 结构，不可接受。

## 优先级

**最高**。最后一个需求，完成后 V3 全部就绪。

## 预期效果

### 场景 1：BT 编辑器
1. 运营点击"行为树"→"新建"→ 输入名称
2. 看到树形编辑器：根节点默认 sequence
3. 点击"添加子节点"→ 下拉选择节点类型（从 node-type-schemas 动态获取）
4. 选择类型后，该节点的参数表单按 params_schema 动态渲染
5. composite 节点（sequence/selector/parallel）可添加多个子节点
6. decorator 节点（inverter）只能有一个子节点
7. leaf 节点（check_bb_float 等）无子节点，只有参数
8. 保存为递归 JSON 结构

### 场景 2：FSM 编辑器
1. 运营点击"状态机"→"新建"→ 输入名称
2. 填写 initial_state、states 列表
3. 添加 transitions：from / to / priority / condition
4. condition 使用递归条件编辑器（leaf / and / or）
5. leaf 条件的参数按 condition-type-schemas 的 leaf.params_schema 渲染

### 场景 3：动态节点类型
- BT 编辑器的"添加节点"下拉列表从 API 动态获取，不硬编码
- 服务端新增节点类型 → 种子脚本重新导入 → 编辑器自动出现新选项

## 依赖分析

- 前置：需求 0（CRUD 框架）+ 需求 1（种子脚本导入 schema + SchemaForm）
- 被依赖：无

## 改动范围

### 前端 — 约 5 个文件

| 文件 | 说明 |
|------|------|
| `src/components/BtNodeEditor.vue` | BT 节点递归编辑器（重建） |
| `src/components/ConditionEditor.vue` | FSM 条件递归编辑器（重建） |
| `src/views/BtTreeForm.vue` | BT 专用表单页 |
| `src/views/FsmConfigForm.vue` | FSM 专用表单页 |
| `src/router/index.js` | BT/FSM 路由指向专用页面 |

### 后端 — 无改动

## 扩展轴检查

- 新增配置类型：不涉及
- 新增表单字段：✅ 有利。新增 BT 节点类型 → 种子脚本导入 → 编辑器自动出现新选项

## 验收标准

### BT 编辑器
- **R1**：节点类型下拉从 node-type-schemas API 动态获取
- **R2**：选择节点类型后按 params_schema 渲染参数表单（SchemaForm）
- **R3**：composite 节点可添加多个子节点，decorator 只能一个，leaf 无子节点
- **R4**：支持递归嵌套（至少 3 层深度）
- **R5**：保存为正确的递归 JSON 结构

### FSM 编辑器
- **R6**：states 列表可增删
- **R7**：transitions 可增删，condition 为递归编辑器
- **R8**：condition leaf 节点参数从 condition-type-schemas 动态获取
- **R9**：保存格式正确（initial_state / states / transitions）

### 通用
- **R10**：编辑模式下数据正确回显
- **R11**：`npm run build` 通过
- **R12**：节点类型展示中文名（display_name）+ 英文括注

## 不做什么

- ❌ 不做拖拽排序节点
- ❌ 不做可视化连线图（纯表单树形结构）
- ❌ 不做 BT 黑板 Key 与 NPC 模板组件联动（预留数据，不做 UI 联动）
- ❌ 不做运行时调试/预览
