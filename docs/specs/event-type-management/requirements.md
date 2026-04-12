# 事件类型管理 — 需求分析（后端）

> 对应文档：
> - 功能：[features.md](../../v3-PLAN/行为管理/事件类型/features.md)
> - 后端设计：[backend.md](../../v3-PLAN/行为管理/事件类型/backend.md)
> - 导出契约：[api-contract.md](../../architecture/api-contract.md) "4. 事件类型"段
>
> **范围**：仅后端（handler/service/store/cache/model/errcode/router/test）。前端另起 spec。

---

## 动机

事件类型是 V3 推荐创建顺序中第 4 步（字段 → 模板 → **事件类型** → 状态机 → 行为树 → NPC → 区域），是**行为管理分类的第一个模块**，也是 ADMIN 侧第一个"导出给游戏服务端消费的配置"类型。

不做的代价：

1. **FSM/BT 模块阻塞**：状态机转移条件用 `last_event_type == "gunshot"` 引用事件类型 name，行为树叶节点用 `check_bb_string key=last_event_type value=gunshot`。没有事件类型管理，FSM/BT 的条件编辑器无法提供事件类型下拉选项。
2. **导出 API 缺口**：游戏服务端启动时拉取 `GET /api/configs/event_types`，当前该端点不存在，服务端如果开启会直接启动失败。
3. **扩展字段机制无处落地**：事件类型是第一个引入"系统字段 + 运营自定义扩展字段"架构的模块，该机制需要在事件类型上验证后才能推广到 FSM/BT/Region。
4. **约束校验无法复用**：字段管理的约束校验逻辑需要在本模块中被抽出为 `service/constraint/validate.go` 独立包，后续所有新增配置类型都依赖此包。

---

## 优先级

**当前阶段最高优先级**。FSM/BT 直接依赖事件类型管理的 name 列表和导出 API。

---

## 预期效果

1. **事件类型 CRUD 闭环**：7 个 REST 接口完成新建、编辑、停用/启用、删除。
2. **扩展字段 Schema 管理**：5 个 REST 接口完成扩展字段定义的 CRUD。
3. **导出 API 到位**：`GET /api/configs/event_types` 从 MySQL `config_json` 列直接输出 `{items: [{name, config}]}` 格式。
4. **三态生命周期严格执行**：启用中拒绝编辑/删除；停用后可改可删。
5. **约束校验复用包落地**：`service/constraint/validate.go` 抽出后，字段管理和事件类型扩展字段共用同一套值级校验逻辑。
6. **缓存策略与字段/模板管理一致**。

---

## 依赖分析

### 依赖的已完成工作

| 依赖项 | 位置 | 用途 |
|---|---|---|
| Handler `WrapCtx` 泛型包装 | `handler/wrap.go` | 统一响应格式 |
| 错误码体系框架 | `errcode/` | 42001-42039 段位 |
| 字典缓存 `DictCache` | `cache/` | perception_mode label 翻译 |
| 配置 | `config/` | 分页/校验长度 |
| Router 注册模式 | `router/router.go` | 沿用已有模式 |
| 字段管理约束校验逻辑 | `service/field.go` | 抽出到 `service/constraint/validate.go` 复用 |

### 谁依赖这个需求

| 依赖方 | 需要的内容 | 紧迫度 |
|---|---|---|
| **FSM 模块** | 事件类型 name 列表 + 导出 API | 阻塞 |
| **BT 模块** | 同上 | 阻塞 |
| **游戏服务端** | `GET /api/configs/event_types` | 联调阻塞 |
| **后续配置类型** | `service/constraint/validate.go` 约束校验复用包 | 通用依赖 |

### 不依赖

| 项 | 说明 |
|---|---|
| MongoDB / RabbitMQ | 本模块不使用 |
| `field_refs` 表 | 事件类型有独立引用体系（本期不建） |
| `fields` 表 | 事件类型字段来自系统字段 + event_type_schema |

---

## 改动范围

新增 ~14 个后端文件 + 改动 ~5 个文件。

### 后端新增文件

| 文件 | 作用 |
|---|---|
| `model/event_type.go` | EventType / EventTypeListItem / EventTypeDetail / DTO |
| `model/event_type_schema.go` | EventTypeSchema / EventTypeSchemaLite / DTO |
| `store/mysql/event_type.go` | EventTypeStore CRUD |
| `store/mysql/event_type_schema.go` | EventTypeSchemaStore CRUD |
| `store/redis/event_type_cache.go` | EventTypeCache |
| `cache/event_type_schema_cache.go` | EventTypeSchemaCache 内存缓存 |
| `service/event_type.go` | EventTypeService 业务逻辑 |
| `service/event_type_schema.go` | EventTypeSchemaService |
| `service/constraint/validate.go` | 约束校验复用包 |
| `handler/event_type.go` | 7 个接口 |
| `handler/event_type_schema.go` | 5 个接口 |
| `handler/export.go` | 导出 API |
| `migrations/004_create_event_types.sql` | DDL |
| `migrations/005_create_event_type_schema.sql` | DDL |

### 后端改动文件

| 文件 | 改动内容 |
|---|---|
| `errcode/codes.go` | 新增 42001-42039 |
| `router/router.go` | 注册 13 个路由 |
| `store/redis/keys.go` | 新增 event_types key |
| `cmd/admin/main.go` | 装配注入链 + SchemaCache.Load |
| `service/field.go` | 抽出值级校验到 constraint 包 |

---

## 验收标准

### 事件类型 CRUD 接口 (R1-R11)

- **R1**：7 个 REST 接口：list/create/detail/update/delete/check-name/toggle-enabled
- **R2**：统一 `{Code, Data, Message}` 响应格式
- **R3**：错误码 42001-42015 定义在 `errcode/codes.go`
- **R4**：`config_json` = 系统字段 + 运营填过的扩展字段值
- **R5**：编辑时全量替换 `config_json`
- **R6**：`name` 创建后不可修改
- **R7**：`name` 全局唯一含软删除
- **R8**：`enabled=1` 时拒绝编辑 (42015) 和删除 (42012)
- **R9**：软删除 `deleted=1`
- **R10**：乐观锁 `WHERE version=?`
- **R11**：`perception_mode==global` 时 `range` 强制置 0

### 扩展字段约束校验 (R12-R15)

- **R12**：扩展字段值通过 `constraint.ValidateValue` 校验
- **R13**：扩展字段 key 必须在 schema 中存在且 enabled
- **R14**：`constraint.ValidateValue` 从字段管理抽出，字段管理不受影响
- **R15**：`constraint.ValidateConstraintsSelf` 校验约束自洽

### 扩展字段 Schema 管理 (R16-R21)

- **R16**：5 个 REST 接口：list/create/update/toggle-enabled/delete
- **R17**：`field_name` 唯一含软删除
- **R18**：`field_type ∈ {int, float, string, bool, select}`
- **R19**：`default_value` 必须符合 `constraints`
- **R20**：删除前必须先停用
- **R21**：启用/停用不触碰已有 event_types 行

### 缓存 (R22-R27)

- **R22-R26**：列表版本号失效、详情分布式锁、空标记、TTL+jitter、Redis 降级
- **R27**：Schema 使用内存缓存

### 导出 API (R28-R31)

- **R28-R31**：`GET /api/configs/event_types` 返回 `{items: [{name, config}]}`

### 可观测性 (R42-R43)

- **R42-R43**：slog.Debug/Info/Error 日志

---

## 不做什么

1. `event_type_refs` 表和 `ref_count`（等 FSM/BT）
2. 引用详情接口
3. 删除 TOCTOU 防护（等 FSM/BT）
4. Schema 编辑收紧拦截
5. `config_json` 历史字段清理
6. 导出响应 Redis 整包缓存
7. 系统字段热更新
8. MongoDB / RabbitMQ
9. **前端页面**（另起 spec）
