# 需求 6：任务拆解

## T1: BtNodeEditor 递归组件 (R1, R2, R3, R4, R12)

**新增文件：**
- `frontend/src/components/BtNodeEditor.vue`

**职责：**
- 从 node-type-schemas API 加载节点类型列表（传入 props 避免重复请求）
- 节点类型 el-select（中文 display_name + 英文括注）
- 选择类型后按 params_schema 渲染 SchemaForm（formFooter.show=false）
- composite → children 数组 + "添加子节点"
- decorator → child 单对象
- leaf → 无子节点
- 类型切换时清理 children/child/params
- 递归渲染子节点
- 删除按钮

**做完了是什么样：** `<BtNodeEditor v-model="node" :nodeTypes="types" />` 渲染递归树形编辑器。

---

## T2: BtTreeForm 专用表单页 (R5, R10)

**新增文件：**
- `frontend/src/views/BtTreeForm.vue`

**职责：**
- name 输入框（allowSlash，BT 名称含 `/`）
- onMounted 加载 node-type-schemas
- BtNodeEditor 渲染根节点（默认 sequence）
- 保存时组装 `{name, config: rootNode}` → API create/update
- 编辑模式回显

**做完了是什么样：** 新建 BT → 树形编辑 → 保存 → 编辑回显。

---

## T3: ConditionEditor 递归组件 (R8)

**新增文件：**
- `frontend/src/components/ConditionEditor.vue`

**职责：**
- 类型选择：leaf / and / or
- leaf → key(el-input) + op(el-select，从 condition-type-schemas leaf.params_schema 获取) + value/ref_key
- and/or → 子条件列表 + "添加条件" + 递归
- 删除按钮

**做完了是什么样：** `<ConditionEditor v-model="condition" :conditionSchemas="schemas" />` 渲染递归条件树。

---

## T4: FsmConfigForm 专用表单页 (R6, R7, R9, R10)

**新增文件：**
- `frontend/src/views/FsmConfigForm.vue`

**职责：**
- name 输入框
- states 列表（el-tag 动态增删）
- initial_state（el-select，选项来自 states）
- transitions 列表（from/to/priority/condition）
- condition 用 ConditionEditor
- 保存组装 `{name, config: {initial_state, states, transitions}}`
- 编辑回显

**做完了是什么样：** 新建 FSM → 添加 states → 添加 transitions with conditions → 保存 → 编辑回显。

---

## T5: 路由更新 + 构建 + 文档 (R11)

**修改文件：**
- `frontend/src/router/index.js` — BT/FSM 路由指向专用页面
- `docs/specs/v3-roadmap.md` — 需求 6 状态更新

**做完了是什么样：** `npm run build` 通过。所有 7 个需求标记完成。

---

## 任务依赖

```
T1(BtNodeEditor) → T2(BtTreeForm)
T3(ConditionEditor) → T4(FsmConfigForm)
T2 + T4 → T5(路由 + 文档)
```

T1/T3 可并行。
