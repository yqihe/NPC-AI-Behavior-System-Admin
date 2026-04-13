# 状态机管理 — 需求分析（后端）

> 对应文档：
> - 功能：[features.md](../../v3-PLAN/行为管理/状态机/features.md)
> - 导出契约：[api-contract.md](../../architecture/api-contract.md) "5. 状态机"段
> - 游戏服务端消费方：`NPC-AI-Behavior-System-Server/internal/core/fsm/fsm.go`
>
> **范围**：仅后端（handler/service/store/cache/model/errcode/router）。前端另起 spec。

---

## 动机

状态机定义 NPC 有哪些状态、什么条件下切换，是行为系统的**状态层**。

不做的代价：

1. **行为树阻塞**：NPC 配置的 `bt_refs` 映射 `状态名 → 行为树名`，状态名来自 FSM。没有 FSM 管理，BT 模块无法确定合法状态列表。
2. **NPC 管理阻塞**：NPC 配置引用 `fsm_ref`，需要 FSM name 列表做下拉选项和引用校验。
3. **导出 API 缺口**：游戏服务端启动拉取 `GET /api/configs/fsm_configs`，当前端点不存在。
4. **条件编辑器无处验证**：FSM 转换条件使用 `{key, op, value}` / `{and/or}` 树形结构，该条件系统是首次在 ADMIN 落地，BT 后续复用。

---

## 优先级

**当前阶段最高优先级**。事件类型已完成，FSM 是行为管理链路的下一环，BT、NPC 管理直接依赖。

---

## 预期效果

1. **FSM CRUD 闭环**：7 个 REST 接口完成新建、编辑、停用/启用、删除、查名唯一性。
2. **条件树校验**：Service 层递归校验 `condition` 结构合法性（叶/组合互斥、op 合法、嵌套深度限制）。
3. **配置完整性校验**：`initial_state ∈ states`、`from/to ∈ states`、状态名不重复、transition priority 不为负。
4. **导出 API 到位**：`GET /api/configs/fsm_configs` 从 MySQL `config_json` 列直接输出 `{items: [{name, config}]}` 格式。
5. **三态生命周期严格执行**：启用中拒绝编辑/删除；停用后可改可删。
6. **缓存策略与已有模块一致**。

---

## 依赖分析

### 依赖的已完成工作

| 依赖项 | 位置 | 用途 |
|---|---|---|
| Handler `WrapCtx` 泛型包装 | `handler/wrap.go` | 统一响应格式 |
| 错误码体系框架 | `errcode/` | 430xx 段位（字段 400xx / 模板 410xx / 事件类型 420xx，FSM 顺延） |
| 配置 | `config/` | 分页 / 校验长度 |
| Router 注册模式 | `router/router.go` | 沿用已有模式 |
| 导出 Handler | `handler/export.go` | 追加 FSM 导出方法 |

### 谁依赖这个需求

| 依赖方 | 需要的内容 | 紧迫度 |
|---|---|---|
| **BT 模块** | FSM 的状态名列表（BT 绑定到状态） | 阻塞 |
| **NPC 管理** | FSM name 列表（`fsm_ref` 下拉 + 引用校验） | 阻塞 |
| **游戏服务端** | `GET /api/configs/fsm_configs` | 联调阻塞 |

### 不依赖

| 项 | 说明 |
|---|---|
| MongoDB / RabbitMQ | FSM 配置 MySQL 单存储 |
| Schema 管理 | FSM 无扩展字段，结构固定 |
| 事件类型 `ref_count` | FSM 条件的 key 引用的是 BB Key，不是事件类型 ID，无需引用计数 |

---

## 改动范围

新增 ~8 个后端文件 + 改动 ~4 个文件。

### 后端新增文件

| 文件 | 作用 |
|---|---|
| `model/fsm_config.go` | FsmConfig / FsmConfigListItem / FsmConfigDetail / DTO |
| `store/mysql/fsm_config.go` | FsmConfigStore CRUD |
| `store/redis/fsm_config_cache.go` | FsmConfigCache |
| `service/fsm_config.go` | FsmConfigService 业务逻辑 + 条件树校验 |
| `handler/fsm_config.go` | 7 个接口 |
| `migrations/006_create_fsm_configs.sql` | DDL |
| `config/fsm_config.go` | FSM 模块配置（name/displayName 长度限制等） |

### 后端改动文件

| 文件 | 改动内容 |
|---|---|
| `errcode/codes.go` | 新增 43001-43015 |
| `router/router.go` | 注册 8 个路由（7 CRUD + 1 导出） |
| `store/redis/keys.go` | 新增 fsm_configs key |
| `handler/export.go` | 追加 `FsmConfigs()` 方法 |
| `cmd/admin/main.go` | 装配注入链 |

---

## 扩展轴检查

- **新增配置类型只需加一组 handler/service/store/validator**：✅ FSM 完全遵循此模式，不改已有模块代码。
- **新增表单字段只需加组件**：不涉及（FSM 无扩展字段机制）。

---

## 验收标准

### FSM CRUD 接口 (R1-R10)

- **R1**：7 个 REST 接口：`list` / `create` / `detail` / `update` / `delete` / `check-name` / `toggle-enabled`
- **R2**：统一 `{Code, Data, Message}` 响应格式（WrapCtx）
- **R3**：错误码 43001-43015 定义在 `errcode/codes.go`
- **R4**：`config_json` 存 `{initial_state, states, transitions}` 完整配置
- **R5**：编辑时全量替换 `config_json`
- **R6**：`name` 创建后不可修改
- **R7**：`name` 全局唯一含软删除
- **R8**：`enabled=1` 时拒绝编辑 (43010) 和删除 (43009)
- **R9**：软删除 `deleted=1`
- **R10**：乐观锁 `WHERE version=?`，冲突返回 409

### 配置完整性校验 (R11-R16)

- **R11**：`states` 不能为空
- **R12**：`states[].name` 不能为空、不能重复
- **R13**：`initial_state` 必须是 `states` 中的某个
- **R14**：`transitions[].from` 和 `to` 必须是 `states` 中的某个
- **R15**：`transitions[].priority` 必须 ≥ 0
- **R16**：`transitions[].condition` 递归校验：叶/组合互斥、`op` 合法（`== != > >= < <= in`）、嵌套深度 ≤ 10

### 导出 API (R17-R19)

- **R17**：`GET /api/configs/fsm_configs` 返回 `{items: [{name, config}]}`
- **R18**：仅导出 `enabled=1 AND deleted=0` 的记录
- **R19**：空数据返回 `{items: []}`

### 缓存 (R20-R24)

- **R20**：列表缓存使用版本号方案，写操作 INCR 版本号，旧缓存自然过期
- **R21**：详情缓存使用分布式锁防击穿
- **R22**：空查询结果缓存空标记
- **R23**：TTL 添加 jitter 防雪崩
- **R24**：Redis 不可用时降级到 MySQL 直查，不阻塞请求

### 可观测性 (R25-R26)

- **R25**：关键路径 `slog.Debug`（list/detail/create/update/delete/export）
- **R26**：错误路径 `slog.Error`（含 error 上下文）

---

## 不做什么

1. **`fsm_refs` 引用计数表**：等 NPC 管理模块建立引用关系时再加
2. **删除 TOCTOU 防护**：等引用表就位后加
3. **条件 key 来源校验**：condition 中的 `key` 不校验是否在 BB Key 注册表中（服务端自己做 Validate）
4. **MongoDB / RabbitMQ**：MySQL 单存储
5. **扩展字段 / Schema 管理**：FSM 结构固定
6. **前端页面**：另起 spec
7. **导出响应 Redis 整包缓存**
8. **transition 去重校验**：同 from-to 可以有多条不同条件的转换（按 priority 排序）
