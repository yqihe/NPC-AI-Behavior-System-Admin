# bt-management-frontend — 任务拆解

> 对应设计：[design.md](./design.md)

---

## 依赖顺序说明

```
T1(API层) → T2(路由+GuardDialog) → T3(BtTreeList) → T4(BtNodeTypeSelector)
                                                    → T5(BtNodeEditor)
                                                    → T6(BtTreeForm)      依赖 T4,T5
         → T7(BtNodeTypeList)
         → T8(BtParamSchemaEditor) → T9(BtNodeTypeForm) 依赖 T7,T8
```

---

## 任务列表

### [x] T1：API 层 — btTrees.ts + btNodeTypes.ts (R1–R3, R8, R11, R22)

**文件**：
- `frontend/src/api/btTrees.ts`（新建）
- `frontend/src/api/btNodeTypes.ts`（新建）

**产出**：
- `btTrees.ts`：`BtNodeInternal` 接口、`BtTreeListQuery/ListItem/Detail/Create/UpdateRequest`、`BT_TREE_ERR` 错误码常量、`btTreeApi` 对象（list/create/detail/update/delete/checkName/toggleEnabled）、`serializeBtNode` / `deserializeBtNode` 序列化函数
- `btNodeTypes.ts`：`BtParamDef`、`BtNodeTypeMeta`、`BtNodeTypeListQuery/ListItem/Detail/Create/UpdateRequest`、`BT_NODE_TYPE_ERR` 错误码常量（44016–44026）、`btNodeTypeApi` 对象

**做完是什么样**：两个文件无 TS 编译错误，`btTreeApi.list` 等调用路径与后端路由一致（`/bt-trees`、`/bt-node-types`）。

---

### T2：路由注册 + EnabledGuardDialog 扩展 (R22)

**文件**：
- `frontend/src/router/index.ts`（修改）
- `frontend/src/components/EnabledGuardDialog.vue`（修改）

**产出**：
- `router/index.ts`：追加 8 条路由（`/bt-trees`、`/bt-trees/create`、`/bt-trees/:id/view`、`/bt-trees/:id/edit`、`/bt-node-types`、`/bt-node-types/create`、`/bt-node-types/:id/view`、`/bt-node-types/:id/edit`），meta 含 `isCreate`/`isView` 双字段，懒加载对应 view 组件
- `EnabledGuardDialog.vue`：`EntityType` union 追加 `'bt-tree' | 'bt-node-type'`；`entityTypeLabel` / `reasonText` / `onActOnce`（调 btTreeApi/btNodeTypeApi toggleEnabled）/ edit 跳转路径 / `conflictCode` 全部补充

**做完是什么样**：访问 `/bt-trees` 路由不报 404，GuardDialog 传入 `entityType: 'bt-tree'` 能正常渲染文案并执行禁用操作。

---

### T3：行为树列表页 BtTreeList.vue (R1, R6, R7)

**文件**：
- `frontend/src/views/BtTreeList.vue`（新建）

**产出**：
- 顶部标题栏 + 新建按钮
- 筛选栏：name 模糊输入（`filter-item-wide`）+ display_name 模糊输入 + enabled 三态 select
- el-table：ID / name / display_name / enabled(Switch) / created_at / 操作（查看/编辑/删除）
- 分页：`el-pagination`，20 条/页
- `handleToggle`：先 `detail()` 拿 version，弹确认，`toggleEnabled`，VERSION_CONFLICT 弹 alert
- `handleEdit`：enabled → GuardDialog；disabled → 路由跳转
- `handleDelete`：enabled → GuardDialog；disabled → `ElMessageBox.confirm` → delete
- `rowClassName`：禁用行 opacity，排除最后 3 列
- `EnabledGuardDialog` 挂载，`entityType: 'bt-tree'`

**做完是什么样**：列表页正常展示、分页、筛选、启用/禁用/删除流程完整走通。

---

### T4：节点类型选择对话框 BtNodeTypeSelector.vue (R15, R16)

**文件**：
- `frontend/src/components/BtNodeTypeSelector.vue`（新建）

**产出**：
- Props：`modelValue: boolean`（visible）、`nodeTypes: BtNodeTypeMeta[]`
- Emits：`update:modelValue`、`select: [BtNodeTypeMeta]`
- `el-dialog` 内按 composite / decorator / leaf 三组展示，每组一个 `el-radio-group`
- 选项显示格式：`中文标签 (type_name)`（对齐 red-lines §6.5）
- 确认按钮 emit `select`，关闭重置选中状态（`@close` 重置）

**做完是什么样**：父组件传入 nodeTypes，打开弹窗可按分类选择节点类型，点确认回调正确节点。

---

### T5：树编辑器核心组件 BtNodeEditor.vue (R15–R21)

**文件**：
- `frontend/src/components/BtNodeEditor.vue`（新建）

**产出**：
- Props：`modelValue: BtNodeInternal | null`、`nodeTypes: BtNodeTypeMeta[]`、`disabled?: boolean`、`depth?: number`（默认 0）
- Emits：`update:modelValue: [BtNodeInternal | null]`
- `depth===0 && !modelValue && !disabled`：显示"添加根节点"按钮，点击打开 BtNodeTypeSelector
- 节点卡片：头部显示 `[category标签] label (type_name)`，右侧 Edit/Delete 按钮（`v-if="!disabled"`）
- composite：Add Child 按钮 + children 递归 `<BtNodeEditor>`（自引用，depth+1）
- decorator：Set Child 按钮 + 单子节点位（无 child 时占位文字）+ 子节点递归
- leaf：内联参数表单（`showParams` ref 控制展开/收起），param type 映射（bb_key → BBKeySelector，select → el-select，float/integer → el-input-number，bool → el-select，string → el-input），所有子控件 `:disabled="disabled || !showParams"`（view 模式外壳已 disabled，但 Edit 按钮需 `v-if="!disabled"`）
- 节点操作：添加子节点 / 设置子节点 / 删除节点（同时删除子孙），均用 `structuredClone` 深拷贝后修改，emit 新对象
- 未知节点类型（typeMap 找不到）：category 降级 `'leaf'`，params 保留原始 key-value，卡片正常显示 type_name，静默降级不报错
- 缩进：`:style="{ marginLeft: depth * 24 + 'px' }"`

**做完是什么样**：可以从空树添加根节点，构建 sequence → check_bb_float + stub_action 的三节点树，查看模式下所有控件 disabled，Add/Delete/Edit 按钮隐藏。

---

### T6：行为树表单页 BtTreeForm.vue (R2–R5, R8)

**文件**：
- `frontend/src/views/BtTreeForm.vue`（新建）

**产出**：
- `route.meta.isCreate / isView` 模式判断
- `form-body-wide`（max-width: 1200px）宽布局
- **卡片一：基本信息**（`title-bar-blue`）
  - `name`：create 模式可编辑，正则 `^[a-z][a-z0-9_/]*$`，blur 调 `checkName`，showing 状态（idle/checking/available/taken）；edit/view 模式 `disabled` + Lock 图标 + "创建后不可修改"提示；字段下方灰色提示说明格式含 `/`
  - `display_name`：必填
  - `description`：非必填，`el-input type="textarea"`
- **卡片二：行为树结构**（`title-bar-green`）
  - `onMounted` 调 `btNodeTypeApi.list({ enabled: true, page: 1, page_size: 200 })` 加载 nodeTypes，解析 `param_schema` 构建 `BtNodeTypeMeta[]`
  - edit/view 模式：`onMounted` 调 `btTreeApi.detail(id)`，`deserializeBtNode(config, typeMap)` → `rootNode`
  - `<BtNodeEditor v-model="rootNode" :nodeTypes="nodeTypes" :disabled="isView" />`
- 提交：`rootNode === null` 时阻止提交并提示"请构建行为树结构"；`serializeBtNode(rootNode)` 序列化为 config；处理错误码 44001/44002/44003/44004/44005/44006/44011
- `form-footer`：取消 + 保存（`v-if="!isView"`）

**做完是什么样**：新建行为树完整流程（填基本信息 + 构建树 + 保存成功提示）；编辑加载已有 config 并能修改保存；查看模式表单 + 树全部只读。

---

### T7：节点类型列表页 BtNodeTypeList.vue (R9, R10, R13, R14)

**文件**：
- `frontend/src/views/BtNodeTypeList.vue`（新建）

**产出**：
- 筛选栏：type_name 前缀输入 + category el-select（composite/decorator/leaf）+ enabled 三态
- el-table：ID / type_name / category（el-tag）/ label / 内置（el-tag "内置"/"自定义"）/ enabled(Switch) / 创建时间 / 操作
- **操作列**：`is_builtin` 时只显示"查看"；非内置显示"查看/编辑/删除"
- `handleToggle`：先 `detail()` 拿 version，弹确认，VERSION_CONFLICT 刷新
- `handleEdit`：enabled → GuardDialog（`entityType: 'bt-node-type'`）；disabled → 路由跳转
- `handleDelete`：enabled → GuardDialog；disabled → 直接 delete；44022 → 弹引用弹窗（读 `bizErr.data.referenced_by` 展示树名列表，`el-dialog` + `el-table`，对齐 FieldList 引用弹窗结构）
- `EnabledGuardDialog` 挂载，`entityType: 'bt-node-type'`

**做完是什么样**：内置节点类型只能查看，自定义节点类型可编辑/删除；删除被引用时弹窗列出引用树名。

---

### T8：param_schema 行编辑器 BtParamSchemaEditor.vue (R11, R12)

**文件**：
- `frontend/src/components/BtParamSchemaEditor.vue`（新建）

**产出**：
- Props：`modelValue: BtParamDef[]`、`disabled?: boolean`
- Emits：`update:modelValue: [BtParamDef[]]`
- 参数行列表：每行 name（el-input）/ label（el-input）/ type（el-select，6 种）/ required（el-switch）/ options（type=select 时出现，逗号分隔 el-input）
- 行末删除按钮（`v-if="!disabled"`），底部添加行按钮
- `defineExpose({ validate })`：validate 检查每行 name/label 非空、name 格式 `^[a-z][a-z0-9_]*$`、name 不重复、select 类型 options 非空，返回 `string | null`
- `disabled` 时整体只读，隐藏添加/删除按钮

**做完是什么样**：可增删参数行，select 类型行出现 options 输入，`validate()` 能检出空 name、格式错误、重复 name、select 空 options。

---

### T9：节点类型表单页 BtNodeTypeForm.vue (R10, R11, R12, R14)

**文件**：
- `frontend/src/views/BtNodeTypeForm.vue`（新建）

**产出**：
- `route.meta.isCreate / isView` 模式判断，额外检测 `is_builtin`（detail 加载后置 `isBuiltinLocked`）
- **卡片一：基本信息**（`title-bar-blue`）
  - `type_name`：create 可编辑（格式 `^[a-z][a-z0-9_]*$`，blur 调 checkName），edit/view disabled + Lock 图标
  - `category`：create 可选 composite/decorator/leaf，edit/view disabled（`title-bar-orange`）
  - `label`：必填
  - `description`：非必填 textarea
  - `is_builtin` 时顶部显示蓝色 info alert"内置节点类型，只可查看"，整体表单 disabled
- **卡片二：参数定义**（`title-bar-orange`）
  - `<BtParamSchemaEditor v-model="paramDefs" :disabled="isView || isBuiltinLocked" />`
- 提交前调 `paramSchemaEditorRef.value?.validate()`，不通过则阻止
- 错误码处理：44016/44017/44018/44019/44024/44025/44026

**做完是什么样**：新建自定义节点类型填 type_name + category + label + 参数列表，保存成功；内置节点类型查看页整体 disabled；edit 模式 type_name/category 锁定。

---

## 文件清单汇总

| 任务 | 文件 | 新建/修改 |
|------|------|----------|
| T1 | `src/api/btTrees.ts` | 新建 |
| T1 | `src/api/btNodeTypes.ts` | 新建 |
| T2 | `src/router/index.ts` | 修改 |
| T2 | `src/components/EnabledGuardDialog.vue` | 修改 |
| T3 | `src/views/BtTreeList.vue` | 新建 |
| T4 | `src/components/BtNodeTypeSelector.vue` | 新建 |
| T5 | `src/components/BtNodeEditor.vue` | 新建 |
| T6 | `src/views/BtTreeForm.vue` | 新建 |
| T7 | `src/views/BtNodeTypeList.vue` | 新建 |
| T8 | `src/components/BtParamSchemaEditor.vue` | 新建 |
| T9 | `src/views/BtNodeTypeForm.vue` | 新建 |
