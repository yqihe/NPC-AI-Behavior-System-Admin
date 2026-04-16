# 行为树管理 — 需求分析（后端）

> 对应文档：
> - 功能：[features.md](../../v3-PLAN/行为管理/行为树/features.md)
> - 导出契约：[api-contract.md](../../architecture/api-contract.md) "6. 行为树"段
> - 集成注意：[INTEGRATION_NOTE_FROM_FIELD.md](../../v3-PLAN/行为管理/行为树/INTEGRATION_NOTE_FROM_FIELD.md)
>
> **范围**：仅后端（handler/service/store/model/errcode/router/migrations）。前端另起 spec。

---

## 动机

行为树定义 NPC 在每个 FSM 状态下具体执行什么逻辑，是行为系统的**行为层**。

不做的代价：

1. **NPC 管理阻塞**：NPC 配置 `bt_refs`（状态 → 行为树名）需要行为树 name 列表做下拉选项和引用校验。
2. **导出 API 缺口**：游戏服务端启动拉取 `GET /api/configs/bt_trees`，当前端点不存在。
3. **字段 BB Key 引用检查不完整**：`FieldService.Update` 在关闭 `expose_bb` 时须调用 `BTTreeStore.IsBBKeyUsed`，否则运营可以悄无声息地删除行为树正在使用的 BB Key，导致服务端运行时读到 nil 值。
4. **节点类型扩展机制缺失**：服务端新增节点类型后，ADMIN 无法管理其参数 schema，编辑器无法动态渲染表单。

---

## 优先级

**当前阶段次高优先级**（仅次于已完成的 FSM）。

行为树是 FSM 之后的直接下游——FSM 定义有哪些状态，行为树定义每个状态做什么。NPC 管理依赖行为树完成后才能设计 bt_refs 部分。

---

## 预期效果

### 场景 1：策划配置狼的攻击行为树

1. 在节点类型管理中确认 `check_bb_float`、`stub_action` 已注册（内置种子）。
2. 新建行为树 `wolf/attack`，用树编辑器构建：
   ```
   sequence
   ├── check_bb_float (key=player_distance, op=<, value=5)
   └── stub_action (name=melee_attack, result=success)
   ```
3. 保存，启用，游戏服务端拉取 `/api/configs/bt_trees` 获得完整树 JSON。

### 场景 2：服务端注册新节点类型 `wait_seconds`

1. 开发者在节点类型管理页新建：type_name=`wait_seconds`, category=`leaf`, param_schema 描述 `duration(float)` 参数。
2. 策划打开任意行为树编辑器，Add Child 时节点类型列表自动出现 `wait_seconds`。
3. 旧行为树中不含 `wait_seconds` 节点，不受影响。

### 场景 3：关闭字段 BB Key 被行为树引用

1. 字段 `player_distance` 有 `expose_bb=true`。
2. 运营在字段管理页尝试将 `expose_bb` 改为 false。
3. `FieldService.Update` 调用 `BTTreeStore.IsBBKeyUsed(ctx, "player_distance")`，返回 true。
4. 接口返回 40008 `该 Key 正被行为树使用，无法关闭`，操作被阻止。

---

## 依赖分析

### 依赖的已完成工作

| 依赖项 | 位置 | 用途 |
|---|---|---|
| Handler `WrapCtx` 泛型包装 | `handler/wrap.go` | 统一响应格式 |
| 错误码体系框架 | `errcode/codes.go` | 44001+ 段位 |
| 配置加载 | `config/` | 分页 / 名称长度限制 |
| Router 注册模式 | `router/router.go` | 沿用已有模式 |
| 导出 Handler | `handler/export.go` | 追加 BT 导出方法 |
| 字段模块 `FieldService` | `service/field.go` | 需在 Update 中注入 BTTreeStore |
| 错误码 `ErrFieldBBKeyInUse=40008` | `errcode/codes.go` | 已定义，BT 完成后激活使用 |

### 谁依赖这个需求

| 依赖方 | 需要的内容 | 紧迫度 |
|---|---|---|
| **NPC 管理** | bt_tree name 列表（`bt_refs` 下拉 + 引用校验） | 阻塞 |
| **游戏服务端** | `GET /api/configs/bt_trees` | 联调阻塞 |
| **字段管理** | `BTTreeStore.IsBBKeyUsed` | 功能补全 |

### 不依赖

| 项 | 说明 |
|---|---|
| MongoDB / RabbitMQ | MySQL 单存储 |
| Schema 管理（事件类型扩展字段） | BT 节点类型与事件扩展字段是独立系统 |
| FSM 配置内容 | BT 只引用 FSM 状态名（字符串），不查 FSM 表 |

---

## 改动范围

新增 ~10 个后端文件 + 改动 ~4 个文件。

### 后端新增文件

| 文件 | 作用 |
|---|---|
| `model/bt_tree.go` | BtTree / BtTreeListItem / BtTreeDetail / DTO |
| `model/bt_node_type.go` | BtNodeType / BtNodeTypeListItem / BtNodeTypeDetail / DTO |
| `store/mysql/bt_tree.go` | BtTreeStore CRUD + IsBBKeyUsed + GetBBKeyUsages |
| `store/mysql/bt_node_type.go` | BtNodeTypeStore CRUD |
| `store/redis/bt_tree_cache.go` | BtTreeCache（列表 + 详情） |
| `service/bt_tree.go` | BtTreeService 业务逻辑 + 节点树校验 |
| `service/bt_node_type.go` | BtNodeTypeService 业务逻辑 |
| `handler/bt_tree.go` | 7 个接口 |
| `handler/bt_node_type.go` | 7 个接口 |
| `migrations/009_create_bt_tables.sql` | DDL（bt_tree + bt_node_type 两张表） |
| `cmd/seed/bt_node_type_seed.go` | 内置节点类型种子数据 |

### 后端改动文件

| 文件 | 改动内容 |
|---|---|
| `errcode/codes.go` | 新增 44001-44025 |
| `router/router.go` | 注册 BT 路由（14 个 CRUD + 1 导出） |
| `handler/export.go` | 追加 `BTTrees()` 方法 |
| `service/field.go` | `Update` 中注入 BTTreeStore，激活 `IsBBKeyUsed` 检查 |
| `setup/services.go` | 装配注入链（新增 BtTreeStore、BtNodeTypeStore、注入到 FieldService） |

---

## 扩展轴检查

- **新增配置类型只需加一组 handler/service/store/validator**：✅ BT 完全遵循此模式，不改已有模块代码（唯一例外是 `service/field.go` 注入 BTTreeStore，这是跨模块引用检查的既定模式，不是"改已有模块逻辑"）。
- **新增表单字段只需加组件**：不涉及（BT 节点参数由 `bt_node_type.param_schema` 驱动，已是最大化可扩展设计）。

---

## 验收标准

### bt_tree CRUD 接口 (R1-R10)

- **R1**：7 个 REST 接口：`list` / `create` / `detail` / `update` / `delete` / `check-name` / `toggle-enabled`
- **R2**：统一 `{Code, Data, Message}` 响应格式（WrapCtx）
- **R3**：错误码 44001-44015 定义在 `errcode/codes.go`
- **R4**：`config` 字段存完整根节点 JSON，节点 params 展开到节点对象顶层（无 `params` 包装层）
- **R5**：编辑时全量替换 `config`
- **R6**：`name` 创建后不可修改
- **R7**：`name` 全局唯一含软删除，格式 `^[a-z][a-z0-9_/]*$`
- **R8**：`enabled=1` 时拒绝编辑 (44010) 和删除 (44009)
- **R9**：软删除 `deleted=1`
- **R10**：乐观锁 `WHERE version=?`，冲突返回 409

### bt_node_type CRUD 接口 (R11-R20)

- **R11**：7 个 REST 接口：`list` / `create` / `detail` / `update` / `delete` / `check-name` / `toggle-enabled`
- **R12**：错误码 44016-44025 定义在 `errcode/codes.go`
- **R13**：`is_builtin=1` 的节点类型不可删除 (44023)，不可编辑 (44024)
- **R14**：删除前检查是否被任何 bt_tree 使用（扫描 config JSON），被引用时返回 44022（携带引用树名列表）
- **R15**：`param_schema` 合法性校验：是合法 JSON 对象，`params` 是数组，每个 param 有 `name`/`label`/`type`，`type` 枚举合法（`bb_key`/`string`/`float`/`integer`/`bool`/`select`），`select` 类型必须有非空 `options`
- **R16**：`enabled=1` 时拒绝编辑 (44021) 和删除 (44020)
- **R17**：软删除 `deleted=1`
- **R18**：乐观锁，冲突返回 409
- **R19**：`type_name` 全局唯一含软删除，格式 `^[a-z][a-z0-9_]*$`
- **R20**：启动时执行种子脚本，内置 7 种节点类型（sequence/selector/parallel/inverter/check_bb_float/check_bb_string/set_bb_value/stub_action）

### 节点树校验 (R21-R26)

- **R21**：config 不能为空（根节点必须存在）
- **R22**：每个节点的 `type` 必须存在于 bt_node_type 表（`deleted=0`）
- **R23**：composite 节点（sequence/selector/parallel）必须有 `children` 数组且非空
- **R24**：decorator 节点（inverter）必须有 `child` 对象
- **R25**：leaf 节点不能有 `children` 或 `child` 字段
- **R26**：嵌套深度不超过 20 层

> 注意：节点 params 字段值的合法性（如 `op` 枚举、`key` 是否真实存在于 BB Key 表）**不在本期校验范围**，服务端运行时负责。

### 导出 API (R27-R29)

- **R27**：`GET /api/configs/bt_trees` 返回 `{items: [{name, config}]}`
- **R28**：仅导出 `enabled=1 AND deleted=0` 的记录
- **R29**：空数据返回 `{items: []}`

### BB Key 引用检查 (R30-R31)

- **R30**：`BTTreeStore.IsBBKeyUsed(ctx, bbKey)` 扫描所有 `deleted=0` 的 bt_tree.config，提取所有 `type=bb_key` 参数位置的值，判断是否包含该 key
- **R31**：`FieldService.Update` 在 `expose_bb true→false` 时调用 `IsBBKeyUsed`，命中则返回 40008

### 缓存 (R32-R35)

- **R32**：列表缓存使用版本号方案（写操作 INCR 版本号）
- **R33**：详情缓存使用分布式锁防击穿，空查询缓存空标记
- **R34**：TTL 添加 jitter 防雪崩
- **R35**：Redis 不可用时降级到 MySQL 直查

### 可观测性 (R36-R37)

- **R36**：关键路径 `slog.Debug`（list/detail/create/update/delete/export）
- **R37**：错误路径 `slog.Error`（含 error 上下文）

---

## 不做什么

1. **NPC 引用检查**：BT 删除时不检查 NPC 是否引用（NPC 管理完成后补充，占位错误码 44012）
2. **节点 params 值校验**：op 枚举、bb_key 是否真实存在等，服务端运行时负责
3. **MongoDB / RabbitMQ**：MySQL 单存储
4. **bt_tree 之间的引用**：BT 不支持子树引用（每棵树独立完整）
5. **param_schema 版本迁移**：节点类型 schema 变更不自动修复已有行为树中的旧参数
6. **前端页面**：另起 spec
7. **导出响应 Redis 整包缓存**
8. **节点顺序拖拽**：毕设后
