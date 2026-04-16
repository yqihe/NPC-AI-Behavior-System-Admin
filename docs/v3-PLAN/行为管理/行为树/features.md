# 行为树管理 — 功能定义

> 定义 NPC 在每个状态下具体做什么。行为树通过节点组合描述 NPC 的决策逻辑。

---

## 概览

行为树管理分两个页面：

| 页面 | 路由 | 说明 |
|------|------|------|
| 行为树列表 | 行为管理 > 行为树 | 行为树的 CRUD，含树编辑器 |
| 节点类型管理 | 系统设置 > 节点类型 | 注册节点类型和参数 Schema（开发者工具） |

---

## 数据模型

### bt_tree（行为树）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT | 主键 |
| name | VARCHAR(128) | 唯一标识，`^[a-z][a-z0-9_/]*$`，如 `wolf/attack` |
| display_name | VARCHAR(128) | 中文名 |
| description | TEXT | 描述（可空） |
| config | JSON | 根节点 JSON（树结构） |
| enabled | TINYINT | 1=启用 0=禁用 |
| deleted | TINYINT | 软删除 |
| version | INT | 乐观锁 |
| created_at / updated_at | DATETIME | 时间戳 |

### bt_node_type（节点类型）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGINT | 主键 |
| type_name | VARCHAR(64) | 唯一标识，与导出 JSON 的 `type` 字段一致 |
| category | ENUM | `composite` / `decorator` / `leaf` |
| label | VARCHAR(128) | 中文名 |
| description | TEXT | 描述（可空） |
| param_schema | JSON | 参数定义列表，编辑器用于动态渲染表单 |
| is_builtin | TINYINT | 1=内置（种子），不可删除 |
| enabled | TINYINT | 1=启用 0=禁用（禁用后编辑器不显示该类型） |
| deleted | TINYINT | 软删除 |
| version | INT | 乐观锁 |
| created_at / updated_at | DATETIME | 时间戳 |

### param_schema 格式

```json
{
  "params": [
    {
      "name": "key",
      "label": "BB Key",
      "type": "bb_key",
      "required": true
    },
    {
      "name": "op",
      "label": "操作符",
      "type": "select",
      "options": ["<", "<=", ">", ">=", "==", "!="],
      "required": true
    },
    {
      "name": "value",
      "label": "比较值",
      "type": "float",
      "required": true
    }
  ]
}
```

param `type` 取值：`bb_key` / `string` / `float` / `integer` / `bool` / `select`

`bb_key` 类型触发 BBKeySelector 组件（显示 NPC 字段 + 事件扩展字段两组）。

---

## 内置节点类型（种子数据）

### Composite（组合节点）

| type_name | label | children 字段 | 说明 |
|-----------|-------|--------------|------|
| `sequence` | 序列 | `children: [...]` | 顺序执行，全部成功才成功 |
| `selector` | 选择器 | `children: [...]` | 顺序执行，第一个成功即成功 |
| `parallel` | 并行 | `children: [...]` | 同时执行全部子节点 |

Composite 节点无 param_schema，子节点通过编辑器 Add/Remove 管理。

### Decorator（装饰节点）

| type_name | label | child 字段 | 说明 |
|-----------|-------|-----------|------|
| `inverter` | 取反 | `child: {...}` | 翻转子节点结果 |

Decorator 节点无 param_schema，只有一个子节点位。

### Leaf（叶子节点，V1 范围）

| type_name | label | param_schema 核心字段 |
|-----------|-------|-----------------------|
| `check_bb_float` | 检查浮点 BB | key(bb_key), op(select: <,<=,>,>=,==,!=), value(float) |
| `check_bb_string` | 检查字符串 BB | key(bb_key), op(select: ==,!=), value(string) |
| `set_bb_value` | 设置 BB 值 | key(bb_key), value(string，运行时由服务端解析类型) |
| `stub_action` | 存根动作 | name(string), result(select: success,failure,running) |

---

## 功能清单

### 行为树管理

| # | 功能 | 说明 |
|---|------|------|
| BT-01 | 列表分页 | 20 条/页，后端分页 |
| BT-02 | 组合搜索 | 按 name 模糊、display_name 模糊、enabled 状态筛选 |
| BT-03 | 新建 | 填基本信息（name/display_name/description）+ 树编辑器构建节点树 |
| BT-04 | 编辑 | 同上，乐观锁防冲突 |
| BT-05 | 详情查看 | 只读模式展示树结构 |
| BT-06 | 启用/禁用 | 切换 enabled，EnabledGuardDialog 拦截编辑/删除 |
| BT-07 | 删除 | 软删除，启用中不可删除；禁用状态直接删除（当前无 NPC 引用检查，NPC 管理完成后补充） |
| BT-08 | 名称唯一性 | name 全局唯一（含软删除记录），`^[a-z][a-z0-9_/]*$` 校验 |

### 节点类型管理（系统设置）

| # | 功能 | 说明 |
|---|------|------|
| NT-01 | 列表分页 | 按 category 分组展示（composite / decorator / leaf），可按 type_name / label / category / enabled 筛选 |
| NT-02 | 查看详情 | 展示 param_schema 结构（JSON 格式化只读） |
| NT-03 | 新建自定义节点类型 | 填 type_name / category / label / description / param_schema |
| NT-04 | 编辑自定义节点类型 | 同上，乐观锁；内置类型不可编辑 |
| NT-05 | 删除 | 检查是否被任何 bt_tree 使用（扫描 config JSON）；内置类型不可删除 |
| NT-06 | 启用/禁用 | 禁用后树编辑器不显示该类型（已有节点不受影响） |

---

## 树编辑器（BtNodeEditor 组件）

前端核心组件，用于 BT-03 / BT-04。

### 数据结构（前端内部）

```ts
interface BtNode {
  type: string            // 对应 bt_node_type.type_name
  category: 'composite' | 'decorator' | 'leaf'
  params: Record<string, unknown>  // 参数值，key 对应 param_schema.params[].name
  children?: BtNode[]    // composite 用
  child?: BtNode         // decorator 用
}
```

### 渲染规则

- 树以**缩进卡片**形式展示，每级缩进 24px
- 每个节点卡片头部显示：`[category 标签] type_name — label（params 摘要）`
- Composite 节点：头部 + Add Child 按钮 + 子节点列表
- Decorator 节点：头部 + Set Child 按钮 + 单个子节点位（空时显示占位）
- Leaf 节点：头部 + 内联表单（按 param_schema 动态渲染）
- 每个节点右侧有 Edit（展开内联参数）/ Delete 按钮

### 节点操作

| 操作 | 触发 | 说明 |
|------|------|------|
| 添加根节点 | 树为空时 | 选择节点类型，创建根节点 |
| Add Child | Composite 节点的按钮 | 选择节点类型，追加到 children 末尾 |
| Set Child | Decorator 节点的按钮 | 选择节点类型，设置 child |
| 删除节点 | 节点的 Delete 按钮 | 同时删除其所有子孙 |
| 编辑参数 | 节点的 Edit 按钮 / 直接展开 | 按 param_schema 渲染内联表单，实时写入 params |

### 节点类型选择器

- 从后端 `GET /api/v1/bt-node-types?enabled=1` 加载（编辑器打开时）
- 按 category 分三组展示
- 选中后渲染对应 param_schema 的表单

---

## 导出格式（与 api-contract 对齐）

`GET /api/configs/bt_trees`

```json
{
  "items": [
    {
      "name": "wolf/attack",
      "config": {
        "type": "sequence",
        "children": [
          {
            "type": "check_bb_float",
            "key": "player_distance",
            "op": "<",
            "value": 5
          },
          {
            "type": "stub_action",
            "name": "melee_attack",
            "result": "success"
          }
        ]
      }
    }
  ]
}
```

导出规则：
- 只导出 `enabled=1` 且 `deleted=0` 的行为树
- `config` 字段直接输出 DB 存储的 JSON（服务端透传）
- 节点 params 展开到节点对象顶层（无 `params` 包装层）

---

## 跨模块集成

### 字段管理 ← 行为树（BB Key 引用检查）

当运营关闭字段的 `expose_bb`（true → false）时，`FieldService.Update` 需要查询行为树是否引用了该 BB Key。

行为树模块需提供：
```go
// BTTreeStore
IsBBKeyUsed(ctx, bbKey string) (bool, error)
GetBBKeyUsages(ctx, bbKey string) ([]string, error)  // 返回引用该 Key 的行为树 name 列表
```

实现方式：扫描所有 `deleted=0` 的 bt_tree.config，提取所有带 `key` 字段（param_schema type=bb_key）的节点。

错误码 `40008 ErrFieldBBKeyInUse`（"该 Key 正被行为树使用，无法关闭"）已定义。

### NPC 管理 → 行为树（bt_refs）

NPC 的 `bt_refs` 值（如 `wolf/attack`）必须精确匹配 bt_tree.name（大小写敏感）。  
NPC 管理实现时，bt_refs 的 value 用下拉选择器（从 bt_tree 列表动态获取），不允许手填。

---

## 延后功能

| # | 功能 | 说明 |
|---|------|------|
| BT-D1 | 删除时引用 NPC 检查 | NPC 管理完成后补充 |
| BT-D2 | 行为树复制/克隆 | 毕设后 |
| BT-D3 | 节点拖拽排序 | 毕设后 |
| BT-D4 | 运行时 BB Key 表管理 | 毕设后（当前 Key 由开发者维护） |
