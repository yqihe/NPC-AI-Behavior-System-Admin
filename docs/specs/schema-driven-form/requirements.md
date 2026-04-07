# 需求 1：Schema 驱动动态表单

## 动机

需求 0 搭建了通用 CRUD 框架，但所有页面还是占位页（"待接入动态表单"），运营人员无法创建任何配置。服务端 CC 已交付完整的 Schema 契约（25 个 JSON 文件），现在需要：

1. 把 Schema 导入 MongoDB，让后端 API 能返回 schema 数据
2. 前端读取 schema 并渲染为可交互的 Element Plus 表单
3. 运营人员可以通过表单创建/编辑/删除所有实体配置

**不做会怎样**：ADMIN 平台无法使用，所有页面都是空白占位页，后续需求（NPC 组件化、区域管理、FSM/BT 编辑器）全部无法推进。

## 优先级

**最高**。是需求 0 之后的第一优先级，阻塞所有后续需求。

## 预期效果

### 场景 1：导入 Schema 并通过 API 访问
- 运行种子脚本后，`GET /api/v1/component-schemas` 返回 10 个组件 schema
- `GET /api/v1/npc-presets` 返回 4 个预设定义
- `GET /api/v1/node-type-schemas` 返回 8 个 BT 节点类型
- `GET /api/v1/condition-type-schemas` 返回 2 个 FSM 条件类型

### 场景 2：事件类型 CRUD
- 运营人员点击"事件类型"→ 看到列表页（表格展示 name + 关键字段）
- 点击"新建"→ 表单动态渲染（name 输入框 + config 字段按 schema 渲染）
- 填写保存 → 后端校验 → 写入 MongoDB → 列表刷新
- 点击编辑 → 表单回显已有数据 → 修改保存
- 点击删除 → 确认 → 删除

### 场景 3：条件字段动态显示
- 创建 movement 组件数据时，选择 `move_type=wander` → `wander_radius` 字段出现且必填
- 选择 `move_type=patrol` → `patrol_waypoints` 字段出现且必填，`wander_radius` 消失
- 选择 `move_type=follow` → 两个条件字段都消失

### 场景 4：Schema 管理页面
- 点击侧边栏"Schema 管理"→ 看到所有 schema 列表（组件/预设/节点/条件分类展示）
- 只读查看，不可编辑（schema 由服务端定义）

## 依赖分析

### 前置依赖
- **需求 0**（已完成）：通用 CRUD 框架、只读 API、JSON Schema 校验器、占位页
- **服务端 Schema 契约**（已交付）：25 个 JSON 文件

### 谁依赖本需求
- 需求 2（NPC 模板组件化）：依赖动态表单组件渲染组件 schema
- 需求 3（区域管理）：依赖动态表单渲染区域 schema
- 需求 4（关键字搜索）：依赖真实的列表页
- 需求 6（FSM/BT 编辑器 Schema 化）：依赖 node-type-schemas / condition-type-schemas API

## 改动范围

### 后端（Go）— 约 6 个文件

| 包 | 改动 |
|---|------|
| `cmd/admin/` | 新增种子脚本入口或启动时自动导入 |
| `internal/handler/router.go` | 注册 node-type-schemas / condition-type-schemas 只读路由 |
| `internal/store/mongo.go` | Collections 列表新增 node_type_schemas / condition_type_schemas |
| `cmd/admin/main.go` | 注册新的只读 handler |

新增文件：
- `cmd/seed/main.go` — Schema 种子脚本（读 JSON 文件 → 写入 MongoDB）

### 前端（Vue 3）— 约 8 个文件

| 目录 | 改动 |
|------|------|
| `src/views/` | 通用列表页 + 通用表单页替换占位页 |
| `src/components/` | 动态表单组件（JSON Schema → Element Plus） |
| `src/router/` | 路由指向真实页面 |
| `src/api/` | 新增 node-type-schemas / condition-type-schemas API |

## 扩展轴检查

- **新增配置类型**：✅ 有利。新增实体类型只需在种子脚本中加一个 schema JSON，前端自动适配
- **新增表单字段**：✅ 有利。修改 schema JSON 即可，前端自动渲染新字段

## 验收标准

### 后端
- **R1**：种子脚本运行后，MongoDB `component_schemas` 集合包含 10 个文档
- **R2**：种子脚本运行后，MongoDB `npc_presets` 集合包含 4 个文档
- **R3**：`GET /api/v1/node-type-schemas` 返回 8 个节点类型 schema
- **R4**：`GET /api/v1/condition-type-schemas` 返回 2 个条件类型 schema
- **R5**：种子脚本幂等——多次运行结果一致，不产生重复数据
- **R6**：`go test ./...` 全部通过
- **R7**：`docker compose up --build` 启动成功

### 前端
- **R8**：事件类型页面支持完整 CRUD（新建/编辑/删除/列表）
- **R9**：状态机页面支持完整 CRUD
- **R10**：行为树页面支持完整 CRUD
- **R11**：NPC 模板页面支持完整 CRUD
- **R12**：区域页面支持完整 CRUD
- **R13**：表单根据 JSON Schema 动态渲染字段（类型、标题、描述、必填标记）
- **R14**：条件字段正确工作（movement 的 wander_radius、personality 的 aggro_range）
- **R15**：Schema 管理页面展示所有 schema（只读）
- **R16**：`npm run build` 构建通过
- **R17**：列表页展示 name + config 中的关键字段

### 工程质量
- **R18**：所有表单字段展示中文标题和描述提示（来自 schema 的 title / description）
- **R19**：表单校验错误以中文提示展示

## 不做什么

- ❌ 不做 NPC 模板的组件勾选流程（需求 2 做）——本需求 NPC 模板页面做基础 CRUD，config 以 JSON Schema 表单渲染
- ❌ 不做 FSM/BT 的可视化编辑器（需求 6 做）——本需求 FSM/BT 以 JSON Schema 表单渲染（能创建/编辑，但不是树形可视化）
- ❌ 不做关键字搜索（需求 4 做）
- ❌ 不做 schema 的增删改（schema 由服务端定义，ADMIN 只读）
- ❌ 不做字段分组折叠（需求 2 的组件化表单做）
