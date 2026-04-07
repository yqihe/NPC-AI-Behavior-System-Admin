# 需求 6：设计方案

## 方案描述

### BT 编辑器（BtNodeEditor.vue）

递归组件，每个节点渲染：

```
BtNodeEditor
├─ 节点类型选择（el-select，从 node-type-schemas API 获取）
├─ 参数表单（SchemaForm，按所选类型的 params_schema 渲染）
├─ 子节点区域（仅 composite / decorator）
│   ├─ BtNodeEditor（递归）
│   ├─ BtNodeEditor（递归）
│   └─ "添加子节点"按钮
└─ 删除按钮
```

**数据结构（与 V2 一致）：**
```json
{
  "type": "sequence",
  "children": [
    { "type": "check_bb_float", "key": "threat_level", "op": ">", "value": 50 },
    { "type": "stub_action", "name": "flee", "result": "running" }
  ]
}
```

**节点分类逻辑：**
- 从 node-type-schemas API 获取全部节点
- `category === "composite"` → children 数组（多子节点）
- `category === "decorator"` → child 单对象
- `category === "leaf"` → 无子节点

**参数处理：**
- 节点的 params_schema 传入 SchemaForm 渲染
- 参数数据直接展平在节点对象上（如 `{type: "check_bb_float", key: "...", op: "...", value: ...}`）
- SchemaForm 的 formFooter.show = false

### FSM 编辑器（FsmConfigForm.vue + ConditionEditor.vue）

**FsmConfigForm 结构：**
```
FsmConfigForm.vue
├─ name 输入框
├─ states 列表（el-tag 动态增删）
├─ initial_state 选择（el-select，选项来自 states）
└─ transitions 列表（动态增删）
    ├─ from（el-select）
    ├─ to（el-select）
    ├─ priority（el-input-number）
    └─ condition（ConditionEditor 递归组件）
```

**ConditionEditor 递归结构：**
```
ConditionEditor
├─ 类型选择：leaf / and / or
├─ leaf 模式 → 参数表单（key / op / value|ref_key）
└─ and/or 模式 → 子条件列表
    ├─ ConditionEditor（递归）
    └─ "添加条件"按钮
```

leaf 条件的 op 列表从 condition-type-schemas 的 leaf.params_schema 动态获取。

### 专用表单页

BtTreeForm.vue 和 FsmConfigForm.vue 替代 GenericForm，路由更新。

## 方案对比

### A（选定）：递归组件 + SchemaForm 渲染参数
节点类型和参数全部从 schema 驱动。

### B（不选）：硬编码节点类型和参数
回到 V2 模式，节点类型写死在前端。违背 V3"消灭硬编码"目标。

## 红线检查

| 红线 | 合规 |
|------|------|
| 禁止暴露技术细节 | ✅ 中文 display_name + 英文括注 |
| 禁止让策划手写 JSON | ✅ 全部表单化 |
| 禁止放行无效输入 | ✅ 节点类型/op 用 el-select |
| 装饰节点 != 复合节点 | ✅ decorator 用 child，composite 用 children |

## 前端陷阱检查

| 陷阱 | 处理 |
|------|------|
| 递归组件 v-for key | 用 index（递归中无稳定 id） |
| 深层响应式 | 递归数据整体用 ref，子组件 emit 更新 |
| 类型切换清理 | composite→leaf 清空 children，反之清空 params |

## 测试策略

手动测试：创建 3 层深度 BT → 保存 → 编辑回显。创建含 and/or 条件的 FSM transition → 保存 → 回显。
