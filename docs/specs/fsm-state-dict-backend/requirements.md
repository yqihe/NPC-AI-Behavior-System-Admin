# 状态字典管理 — 需求分析（后端）

> 对应 mockup：
> - [mockups-fsm-statedict-list.png](../../v3-PLAN/mockups-fsm-statedict-list.png)
> - [mockups-fsm-statedict-form.png](../../v3-PLAN/mockups-fsm-statedict-form.png)
> - FSM 前端字典选择器 [mockups-fsm-dict-selector.png](../../v3-PLAN/mockups-fsm-dict-selector.png)
>
> **范围**：仅后端（handler/service/store/cache/model/errcode/router + seed）。前端另起 spec。
> 姐妹 spec：[fsm-management](../fsm-management/requirements.md)（已完成）
>
> **一致性原则**：handler/service/store/model/errcode/router 命名和分层严格对齐 `fsm_config` / `event_type` 模块，不引入新风格。

---

## 动机

FSM 转换的状态名（`idle` / `attack` / `patrol` ...）目前由策划在每个 FSM 表单中手输。问题：

1. **拼写漂移**：同一个概念被写成 `idle` / `Idle` / `idling` / `idel`，游戏服务端拿到时匹配不上
2. **中文名缺失**：策划看到的是裸英文标识，不利于快速理解和审核
3. **无分类**：战斗态/移动态/社交态混在一起，规模上来后难以管理
4. **无复用**：每个 FSM 单独定义状态，无法沉淀企业级配置资产

不做的代价：
- FSM 规模上到 50+ 时，策划难以排查引用不一致的 bug
- 行为树后续接入时没有统一的状态语义基准
- 毕设答辩时"企业级标准"站不住脚

---

## 优先级

**P0（FSM 前端的前置依赖）**。

原因：
- FSM 表单的「从字典添加」按钮直接依赖字典数据源
- 字典不做，FSM 前端的状态选择只能退化为自由输入，违反 mockup 设计
- 后端无状态字典意味着 Seed 无预置数据，开发环境无法跑通 FSM 完整链路

---

## 预期效果

1. **字典 CRUD 闭环**：8 个 REST 接口完成新建、编辑、启用/停用、删除、查名唯一性、分类列表。
2. **删除引用保护**：删除字典条目前扫描 `fsm_configs.config_json.states`，若有 FSM 在用，返回 `referenced_by: [fsm_names]` 并拒绝删除。
3. **企业级预置数据**：Seed 脚本预置 30+ 条常用状态，按战斗/移动/社交/活动/通用分类。
4. **启用/停用机制**：字典条目复用现有 `enabled` 字段（enabled=0 对应 mockup 中的"废弃"状态），不出现在 FSM 字典选择器中，但历史 FSM 引用仍可查看。
5. **分层对齐**：handler/service/store/cache/model 分层严格复用 `fsm_config` 和 `event_type` 模块命名与模式，代码风格差异 ≈ 0。
6. **缓存策略与已有模块一致**：版本号失效 + 分布式锁防击穿 + 降级 MySQL。

---

## 依赖分析

### 依赖的已完成工作

| 依赖项 | 位置 | 用途 |
|---|---|---|
| Handler `WrapCtx` 泛型包装 | `handler/wrap.go` | 统一响应格式 |
| util 通用校验 | `util/handler.go` `util/service.go` | CheckName / CheckLabel / Pagination |
| 通用 DTO | `model/common.go`（或现有位置） | `IDRequest` / `CheckNameRequest` / `CheckNameResult` / `ToggleEnabledRequest` / `DeleteResult` |
| 错误码体系框架 | `errcode/codes.go` | 43013-43024 段位（FSM 43012 之后顺延） |
| 配置加载 | `config/config.go` | 分页 / 校验长度 |
| Router 注册模式 | `router/router.go` | 沿用 `/api/v1/xxx/{list,create,detail,update,delete,check-name,toggle-enabled}` |
| Redis 缓存模式 | `store/redis/fsm_config_cache.go` | 版本号 + 分布式锁模式直接 clone |
| FSM store 存在 | `store/mysql/fsm_config.go` | 删除引用扫描依赖其 JSON 查询方法 |
| Seed 脚本框架 | `cmd/seed/main.go` | 追加字典数据 |

### 谁依赖这个需求

| 依赖方 | 需要的内容 | 紧迫度 |
|---|---|---|
| **FSM 前端** | 字典列表 API + 分类 API（供选择器） | 阻塞前端开发 |
| **状态字典管理前端**（同期 spec） | 字典 CRUD + 删除引用检查 | 直接依赖 |
| **BT 模块（未来）** | 字典条目可被 BT 节点复用 | 非阻塞 |

### 不依赖

| 项 | 说明 |
|---|---|
| MongoDB / RabbitMQ | 字典配置 MySQL 单存储，无需异步同步 |
| field_refs 表 | 字典条目不是字段，不参与 BB Key 引用体系 |
| 游戏服务端导出 API | 字典仅供 ADMIN 内部使用，游戏服务端不消费 |

---

## 改动范围

新增 ~8 个后端文件 + 改动 ~5 个文件。

### 后端新增文件

| 文件 | 作用 |
|---|---|
| `model/fsm_state_dict.go` | `FsmStateDict` / `FsmStateDictListItem` / `FsmStateDictListData` / `FsmStateDictDetail` / `FsmStateDictListQuery` / `CreateFsmStateDictRequest` / `CreateFsmStateDictResponse` / `UpdateFsmStateDictRequest` / `FsmStateDictDeleteResult`（含 `ReferencedBy` 字段） |
| `store/mysql/fsm_state_dict.go` | `FsmStateDictStore` CRUD + `ListCategories` |
| `store/redis/fsm_state_dict_cache.go` | `FsmStateDictCache` |
| `service/fsm_state_dict.go` | `FsmStateDictService` 业务逻辑 + 删除引用扫描协调 |
| `handler/fsm_state_dict.go` | 8 个接口方法（`List/Create/Get/Update/Delete/CheckName/ToggleEnabled/ListCategories`） |
| `migrations/007_create_fsm_state_dicts.sql` | DDL |
| `config/config.go`（追加） | `FsmStateDictConfig` 结构（长度限制 + TTL） |
| `cmd/seed/main.go`（扩展） | 追加字典 seed 数据（30+ 条） |

### 后端改动文件

| 文件 | 改动内容 |
|---|---|
| `errcode/codes.go` | 新增 43013-43024 |
| `router/router.go` | 注册 `/api/v1/fsm-state-dicts/*` 8 个路由 |
| `store/redis/config/keys.go` | 新增 `fsm_state_dict:*` 的 4 个 key 常量 |
| `store/mysql/fsm_config.go` | **新增** `ListFsmConfigsReferencingState(ctx, stateName, limit)` 方法（供 service 删除引用检查） |
| `cmd/admin/main.go` | 装配注入链 |

---

## 扩展轴检查

- **新增配置类型只需加一组 handler/service/store/validator**：✅ 字典模块完全遵循此模式，对字段管理/事件类型/FSM 模块零入侵（只新增一个公开方法 `ListFsmConfigsReferencingState` 到 `fsm_config` store，不改已有签名）。
- **新增表单字段只需加组件**：不涉及（字典无扩展字段机制）。

---

## 验收标准

### 字典 CRUD 接口 (R1-R10)

- **R1**：8 个 REST 接口（POST，统一 kebab-case 复数）
  - `POST /api/v1/fsm-state-dicts/list` — 分页列表（支持 name/display_name 模糊 + category + enabled 筛选）
  - `POST /api/v1/fsm-state-dicts/create`
  - `POST /api/v1/fsm-state-dicts/detail`
  - `POST /api/v1/fsm-state-dicts/update`
  - `POST /api/v1/fsm-state-dicts/delete`
  - `POST /api/v1/fsm-state-dicts/check-name`
  - `POST /api/v1/fsm-state-dicts/toggle-enabled`
  - `POST /api/v1/fsm-state-dicts/list-categories` — 返回所有分类 distinct 列表（`[]string`），供前端分类筛选下拉
- **R2**：统一 `{Code, Data, Message}` 响应格式（handler.WrapCtx）
- **R3**：错误码 43013-43024 定义在 `errcode/codes.go`，命名前缀 `ErrFsmStateDict*`
- **R4**：Handler 方法命名对齐 `FsmConfigHandler`：`List / Create / Get / Update / Delete / CheckName / ToggleEnabled / ListCategories`
- **R5**：Service 公开方法命名对齐：`List / Create / GetByID / Update / Delete / CheckName / ToggleEnabled / ListCategories`
- **R6**：Store 方法命名对齐：`List / Create / GetByID / Update / SoftDelete / ExistsByName / ToggleEnabled / ListCategories / ListFsmConfigsReferencingState`（最后一个加在 `fsm_config` store）
- **R7**：`name` 全局唯一含软删除
- **R8**：`name` 创建后不可修改
- **R9**：软删除 `deleted=1`
- **R10**：乐观锁 `WHERE version=?`，冲突返回 43021（`ErrFsmStateDictVersionConflict`）

### 输入校验 (R11-R16)

- **R11**：`name` 非空，正则 `^[a-z][a-z0-9_]*$`，长度取自 `FsmStateDictConfig.NameMaxLength`（默认 64）
- **R12**：`display_name` 非空，UTF-8 字符数 `[1, FsmStateDictConfig.DisplayNameMaxLength]`（默认 32）
- **R13**：`category` 非空，UTF-8 字符数 `[1, FsmStateDictConfig.CategoryMaxLength]`（默认 16）
  - 字符集不强制白名单，允许任何运营自定义分类；distinct 查询供下拉参考
- **R14**：`description` 可空，UTF-8 字符数 `≤ FsmStateDictConfig.DescriptionMaxLength`（默认 200）
- **R15**：所有文本字段 trim 后校验，拒绝纯空白
- **R16**：复用 `util.CheckName` / `util.CheckLabel` 校验函数，不重新实现

### 删除引用保护 (R17-R19)

- **R17**：Service 层 `Delete` 前调用 `FsmConfigStore.ListFsmConfigsReferencingState(ctx, stateName, 20)` 扫描所有未软删的 FSM 配置，匹配 `config_json.states[].name == stateName`
- **R18**：若命中 ≥ 1 条 FSM，返回 `ErrFsmStateDictInUse` (43020) 错误，并在 `data` 中携带 `referenced_by: [{id, name, display_name, enabled}]`（最多 20 条，避免响应过大）
- **R19**：命中 0 条时允许软删除

### 启用/停用机制 (R20-R22)

- **R20**：`enabled=0`（对应 mockup 中"废弃"tag）的条目：管理端 `list` 默认返回，FSM 前端字典选择器通过 `enabled=true` 过滤
- **R21**：停用不影响已存在的 FSM 引用（历史数据兼容）
- **R22**：删除已停用的条目仍需走 R17 引用检查

### Seed 数据 (R23-R25)

- **R23**：Seed 脚本预置 30+ 条状态，按以下分类至少各含 4 条：
  - **通用**：idle / moving / interacting / busy
  - **战斗**：alert / engage / attack_melee / attack_ranged / cast_spell / dodge / stagger / dying / dead / flee / revive
  - **移动**：patrol / wander / chase / return_home / follow / escort
  - **社交**：greet / talk / trade / quest_offer / farewell
  - **活动**：sleep / eat / sit / craft / gather
- **R24**：Seed 幂等，重复执行不报错、不重复插入（`name` 唯一约束 + "已存在则跳过"逻辑）
- **R25**：Seed 初始数据 `enabled=1 version=1 deleted=0`

### 缓存 (R26-R29)

- **R26**：列表缓存使用版本号方案（`InvalidateList` INCR 版本号键），写操作触发，旧缓存自然过期
- **R27**：详情缓存使用分布式锁防击穿（`TryLock` + `NullMarker`）
- **R28**：空查询结果缓存空标记（防缓存穿透）
- **R29**：Redis 不可用时降级到 MySQL 直查，不阻塞请求

### 可观测性 (R30-R31)

- **R30**：关键路径 `slog.Debug`（list / detail / create / update / delete / toggle）
- **R31**：错误路径 `slog.Error`（含 error + 字典 name 上下文）

---

## 不做什么

1. **`by-names` 批量查询接口**：FSM 前端通过一次 `list?enabled=true` 拿全量字典（数据量 < 300 条），本地 map 映射 `name → display_name`，无需额外批量接口
2. **分类独立实体**：category 仅作为字符串字段 + distinct 查询，不单独建 `fsm_state_categories` 表
3. **字典条目被 FSM 引用计数**：不维护 `ref_count` 字段，每次删除时实时扫描 FSM 表
4. **`field_refs` / `ref_registry` 扩展**：字典引用关系通过 JSON 扫描即时查询，不持久化到 ref 表
5. **停用条目自动清理任务**：无定时 GC，停用后靠人工删除
6. **字典导入/导出**：企业级延后，毕设后再做
7. **字典历史版本**：乐观锁仅防并发冲突，不存历史
8. **前端页面**：另起 spec（`fsm-state-dict-frontend`）
9. **FSM 条件中 key 是否是合法 BB Key 的后端语义校验**：归属另一个 spec
10. **多语言 display_name**：仅中文
11. **字典条目权限控制**：本期所有运营人员权限相同，后续引入 RBAC 再做
