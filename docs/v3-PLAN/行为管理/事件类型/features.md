# 事件类型管理 — 功能清单

> 通用架构/规范见 `docs/architecture/overview.md` 和 `docs/development/`。
> 后端实现细节见同目录 `backend.md`，前端设计见 `frontend.md`。

---

## 状态模型

| 状态 | 事件类型页看到 | FSM/BT 条件编辑器看到 | 能被新引用 | 已有引用 |
|---|---|---|---|---|
| 启用 | 可见，正常显示 | 可见可选 | 允许 | 正常 |
| 停用 | 可见，整行灰 | 不可见 | 拒绝 | 保持不动 |
| 已删除 | 不可见 | 不可见 | 不可能 | 已清理 |

核心原则和字段/模板同构：**停用 = 存量不动增量拦截，删除才真正清理引用关系**。

**关于引用计数**：FSM/BT 尚未开发，本期**不建** `event_type_refs` 表和 `ref_count` 列。所有引用相关错误码（`42008`）标记"占位未接入"，Service 层检查永远放行。等 FSM/BT 上线时再做一次迁移加列 + 补反向接口。

---

## 前置依赖（跨项目协调事项）

事件类型扩展字段机制要求游戏服务端 `EventTypeConfig` 有能力容纳未知字段。具体契约：

- 系统字段硬解析（缺失 → 启动 reject）
- 未知字段捕获到 `Extensions map[string]json.RawMessage`
- 业务代码消费扩展字段时**自带默认值**（默认值归服务端所有，ADMIN 不回填历史数据）

**ADMIN 侧开发范围与依赖关系**：
- 功能 1-7（事件类型系统字段 CRUD）**不依赖**服务端改造，可先行开发与自测
- 功能 8-11（扩展字段 Schema 管理）**依赖**服务端改造完成，联调阶段必须对齐
- 功能 12（导出 API）**依赖**服务端能消费扩展字段，否则 ADMIN 即使导出也没意义

---

## 字段分层（本模块最重要的概念）

| 类别 | 字段 | 定义方 | 演进方式 |
|---|---|---|---|
| 系统字段 | `name` / `display_name` / `default_severity` / `default_ttl` / `perception_mode` / `range` | 后端代码硬编码 | 需要 ADMIN 后端 + 前端 + 游戏服务端三方同步发版 |
| 扩展字段 | 运营自定义，如 `priority` / `category` / `cooldown` / `stackable` ... | `event_type_schema` 表 | 运营在 Schema 管理页自助增删，无需发版 |

**两类字段都存在同一个 `event_types.config_json` 列里**，对外表现完全一致——导出 API 扁平展开，游戏服务端用 `Extensions map[string]json.RawMessage` 捕获未知 key。区别只在"表单怎么渲染"：系统字段硬编码输入控件，扩展字段通过 `SchemaForm` 动态渲染。

**默认值的归属**：扩展字段的默认值**归游戏服务端所有**，不归 ADMIN。游戏服务端在 `internal/runtime/event/defaults.go` 维护一份集中的 `extensionDefaults` map 作为单一事实来源。ADMIN 的 `event_type_schema.default_value` **仅用于新建事件类型时前端表单的初始值提示**，不回填历史数据，不影响导出。运营没填过的扩展字段不进 `config_json`，导出时也就没这个 key，游戏服务端消费时按 `defaults.go` 里的值兜底。

运营在 ADMIN Schema 管理页填写 `default_value` 时可以参考服务端 `defaults.go` 的值作为"合理起点"——两者可以相同也可以不同（前者是"运营建议新事件类型这么填"，后者是"运营什么都不填时服务端怎么算"）。

---

## 与游戏服务端的契约承诺

经与服务端 CC 协商确认，ADMIN 侧对扩展字段值做类型和约束校验后才落库。服务端运行时会对类型不匹配做"退化到 `defaults.go` 默认值 + slog.Warn"的兜底，但**不作为常规路径**——触发 warn 意味着 ADMIN 侧有 bug。

**硬性要求**（功能 2 / 功能 4 的 Service 层必须满足）：

- `extensions` 里每个 key 对应的值必须通过 `constraint.ValidateValue(schema.field_type, schema.constraints, value)` 校验后才能进 `config_json`，不通过返回 `42007 ErrEventTypeExtValueInvalid`
- 系统字段在 Handler 层做强类型 + 边界校验（`default_severity ∈ [0, 100]`、`default_ttl > 0` 等），Service 层兜底一次
- 约束不自洽的 Schema（比如 `int` 类型 `max < min`）在 Schema 管理页创建/编辑时被 `42025 ErrExtSchemaConstraintsInvalid` 拦截
- Schema 的 `default_value` 必须符合自身 `constraints`，不符合返回 `42026 ErrExtSchemaDefaultInvalid`

服务端那边遇到类型不匹配时不会崩溃，但日志 warn 会甩锅给 ADMIN——排查的第一步永远是"校验链条哪里漏了"。

详见 [api-contract.md](../../api-contract.md) 的 "### 2. 事件类型" 段的"服务端实现契约"和"责任划分"表。

---

## 模块职责边界

`EventTypeService` 只持有自身的 `EventTypeStore` / `EventTypeCache` / `EventTypeSchemaStore` / `EventTypeSchemaCache`，**不持有**任何其他模块的 store/service。

未来 FSM/BT 上线时会新增跨模块对外接口，本期**只保留方法签名 stub**（返回 `nil` 或空切片，保证调用方编译通过）：

```
ValidateEventTypesForFSM / AttachToFSMTx / DetachFromFSMTx /
ValidateEventTypesForBT  / AttachToBTTx  / DetachFromBTTx  /
GetByIDsLite / InvalidateDetails
```

---

## 功能 1：事件类型列表

### 场景描述

**场景 A — 在事件类型管理页，管理员浏览所有事件。** 不传 `enabled` 筛选，启用和停用的都展示出来，管理员才能对停用条目做重新启用或删除操作。

**场景 B**（未来）— 在 FSM/BT 条件编辑器里，策划从下拉框选一个事件类型作为触发条件。传 `enabled=true`，只展示启用的。

两个场景走同一接口，靠 `EventTypeListQuery.Enabled (*bool)` 的三态区分：`nil` 不筛选、`true` 仅启用、`false` 仅停用。支持按 `display_name` 模糊搜索（`Label`）、按 `perception_mode` 精确筛选、后端分页（Service 层按 `pagCfg` 校正上下界）。

**列表项字段**：`id / name / display_name / perception_mode / enabled / created_at`，以及从 `config_json` 中抽取的展示值 `default_severity / default_ttl / range_meters`（Service 层 unmarshal 后按需取字段，不做索引筛选）。

### 校验规则

无 Handler 层校验（直接透传 query）。Service 层校正分页参数上下界。

### 调用链

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/event-types/list` |
| Handler | `EventTypeHandler.List` — 直接透传 query |
| Service | `EventTypeService.List` — 分页校正 → Redis 列表缓存 → miss → MySQL → 内存 unmarshal `config_json` 挑展示字段 → 写缓存 |
| Store | `EventTypeCache.GetList` → `EventTypeStore.List`（覆盖索引 `idx_list`）→ `EventTypeCache.SetList` |

### 错误码

无专属错误码。

### 边界 case

- Redis 挂了跳过缓存，降级直查 MySQL。
- `config_json` 中的展示字段在 Service 层 unmarshal 抽取。

---

## 功能 2：新建事件类型

### 场景描述

管理员定义一个新事件（"枪声"、"地震"）。填写系统字段 + 扩展字段后提交，默认**未启用**。

新建的事件类型默认未启用。这是和字段/模板一致的"配置窗口期"：管理员可能还要反复调约束和默认值，如果创建即启用，FSM/BT 条件编辑器会立刻出现这个半成品事件类型。

`name` 一经创建不可改（唯一键），含软删除记录也不可复用。

**表单结构**：
- **系统字段区**（硬编码）：`name` / `display_name` / `perception_mode`（radio）/ `range`（数字，global 模式禁用）/ `default_severity`（slider 0-100）/ `default_ttl`（数字）
- **扩展字段区**（SchemaForm 渲染）：读取 `event_type_schema` 里所有 `enabled=1` 的字段定义，按 `sort_order` 渲染。每个字段有"脏标记"追踪，未被交互过的字段不进最终 payload

### 校验规则

- **Handler**（`EventTypeHandler.Create`）做格式/必填校验：
  - `name` 符合 `identPattern = ^[a-z][a-z0-9_]*$`，长度 ≤ `etCfg.NameMaxLength`
  - `display_name` 非空，长度 ≤ `etCfg.DisplayNameMaxLength`
  - `perception_mode ∈ {visual, auditory, global}`
  - `default_severity` ∈ [0, 100]，`default_ttl > 0`，`range >= 0`
  - `perception_mode == "global"` 时 `range` 强制置 0（前端已禁用，后端兜底）
  - `extensions` 必须是 JSON 对象形状（拦 `null` / 数组 / 标量），空 key 检查
- **Service**（`EventTypeService.Create`）做业务校验：
  - `name` 唯一性（含软删除）→ `42001`
  - 对 `extensions` 里每个 key 查 `EventTypeSchemaCache`：
    - 扩展字段必须存在且 `enabled=1`，否则 `42022 ErrExtSchemaNotFound` / `42023 ErrExtSchemaDisabled`
    - 值符合 schema 定义的 `constraints`，不符合 `42007 ErrEventTypeExtValueInvalid`（复用 `constraint.ValidateValue`）
  - 拼 `config_json`：系统字段 + 运营实际填过的扩展字段（未交互过的扩展字段**不进** JSON）
  - 写 MySQL → 清列表缓存

**为什么不把 schema 的 default_value 自动塞进 config_json**：保持"运营定义过 vs 未定义"的语义区分，避免历史配置和新字段默认值耦合。运营没填就是没填，服务端用自己的默认值兜底。

### 调用链

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/event-types/create` |
| Handler | `EventTypeHandler.Create` — 格式校验 |
| Service | `EventTypeService.Create` — 唯一性 → 扩展字段约束 → `config_json` 拼装 → `EventTypeStore.Create` → 清缓存 |
| Store | `EventTypeStore.Create` 返回 `lastInsertId` |

### 错误码

| 错误码 | 常量 | 触发场景 |
|--------|------|---------|
| 42001 | `ErrEventTypeNameExists` | `name` 已存在（含软删除） |
| 42002 | `ErrEventTypeNameInvalid` | `name` 格式不合法或长度超限 |
| 42003 | `ErrEventTypeModeInvalid` | `perception_mode` 枚举非法 |
| 42004 | `ErrEventTypeSeverityInvalid` | `default_severity` 不在 0-100 |
| 42005 | `ErrEventTypeTTLInvalid` | `default_ttl` <= 0 |
| 42006 | `ErrEventTypeRangeInvalid` | `range` < 0 |
| 42007 | `ErrEventTypeExtValueInvalid` | 扩展字段值不符合 schema 约束 |
| 42022 | `ErrExtSchemaNotFound` | 扩展字段定义不存在 |
| 42023 | `ErrExtSchemaDisabled` | 扩展字段已停用 |
| 40000 | `ErrBadRequest` | `display_name` / `extensions` 格式错误 |

### 边界 case

- `name` 含软删除记录也不可复用。
- `perception_mode == "global"` 时 `range` 强制置 0。
- 未交互过的扩展字段不进 `config_json`。

---

## 功能 3：事件类型详情

### 场景描述

**场景 A — 编辑入口。** 管理员点击某个事件类型查看或准备编辑。

**场景 B**（未来）— FSM/BT 条件编辑器选中事件类型后查看完整 config 内容。

无论启用还是停用都能查——已经被 FSM/BT 引用的停用事件类型，策划仍然要能看到它的配置内容。

### 校验规则

- **Handler**：`id > 0`。

### 调用链

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/event-types/detail` |
| Handler | `EventTypeHandler.Get` — 校验 `id > 0` → Service 拿事件类型 → 调 `EventTypeSchemaService.ListEnabled` 拼 extension_schema |
| Service | `EventTypeService.GetByID`（Cache-Aside + 防击穿防穿透）+ `EventTypeSchemaService.ListEnabled`（内存缓存）|
| Store | `EventTypeCache.GetDetail` → `TryLock` → `EventTypeStore.GetByID` → `EventTypeCache.SetDetail` |

Service 层使用 Cache-Aside + 分布式锁 + 空标记三件套（和字段/模板同构）：
1. 先查 `EventTypeCache.GetDetail`，命中即返回（命中空标记时返回 `ErrEventTypeNotFound`）
2. miss 时 `TryLock(id, 3s)` 防击穿；获得锁后再 double-check 一次缓存
3. 锁失败不阻塞，降级直查 MySQL
4. 查到（或查不到）都写 Redis；`et=nil` 时写空标记防穿透

**Handler 层额外做一件事**：调 `EventTypeSchemaService.ListEnabled` 拿当前启用的扩展字段定义，和事件类型数据一起返回。前端据此渲染"哪些扩展字段该显示、以什么控件形态显示"。

**响应体：**

```
{
  id, name, display_name, enabled, version, created_at, updated_at,
  config: { display_name, default_severity, default_ttl, perception_mode, range, <扩展字段...> },
  extension_schema: [
    { field_name, field_label, field_type, constraints, default_value, sort_order },
    ...
  ]
}
```

**为什么 extension_schema 放在详情响应里**：前端渲染表单时需要知道"当前这个事件类型有没有某个扩展字段"和"表单该渲染哪些输入控件"，两者一起拉一次接口比分开两次请求更自然。schema 数据来自内存缓存，几乎零开销。

### 错误码

| 错误码 | 常量 | 触发场景 |
|--------|------|---------|
| 42011 | `ErrEventTypeNotFound` | 事件类型不存在（或命中空标记） |
| 40000 | `ErrBadRequest` | `id` 不合法 |

### 边界 case

- 停用事件类型也能查详情。
- extension_schema 来自内存缓存，零开销。

---

## 功能 4：编辑事件类型

### 场景描述

管理员修改某个事件类型的系统字段或扩展字段值。

编辑规则分三档：

| 状态 | 可改字段 | 不可改字段 |
|---|---|---|
| 启用中 | 无 | 全部（必须先停用）→ `42015 ErrEventTypeEditNotDisabled` |
| 未启用 + 未引用 | 全部（除 `name`） | `name`（唯一键，永久锁定） |
| 未启用 + 已引用（本期不生效） | — | 等 FSM/BT 上线后定义 |

**为什么不像字段那样硬拦截"收紧约束"**：FSM/BT 按 `name` 引用事件类型，不按属性引用；`default_severity` 调低、`range` 调小这类变动是**运营语义**层面的（已部署 FSM 的阈值可能失效），不是**数据格式**层面的。本期用**前端二次确认弹窗**提示风险，后端不做硬拦截。

**扩展字段编辑语义**：和功能 2 同构，每次请求全量替换 `config_json` 里的扩展字段部分。未在请求 `extensions` 里出现的扩展字段会从 `config_json` 被移除（表示"运营明确取消了这个字段的值"），下次导出服务端就拿不到这个 key，按默认值走。

写入使用乐观锁 `UPDATE ... WHERE id=? AND version=?`，rows=0 返回 `storemysql.ErrVersionConflict`，Service 层转 `42010 ErrEventTypeVersionConflict`。

### 校验规则

- **Handler**：`id > 0` / `display_name` / `perception_mode` / `severity` / `ttl` / `range` / `extensions` 形状 / `version > 0`（与创建同构，额外 `id > 0` + `version > 0`）
- **Service**：`getOrNotFound` → enabled 必须为 false（42015）→ 扩展字段约束校验（同功能 2）→ 乐观锁写入

### 调用链

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/event-types/update` |
| Handler | `EventTypeHandler.Update` — 格式校验 + `version > 0` |
| Service | `EventTypeService.Update` — `getOrNotFound` → enabled 必须为 false (42015) → 扩展字段约束 → 乐观锁写入 → 清自身 detail + 列表缓存 |
| Store | `EventTypeStore.GetByID` → `EventTypeStore.Update`（WHERE id=? AND version=?）|

### 错误码

| 错误码 | 常量 | 触发场景 |
|--------|------|---------|
| 42003 | `ErrEventTypeModeInvalid` | `perception_mode` 枚举非法 |
| 42004 | `ErrEventTypeSeverityInvalid` | `default_severity` 不在 0-100 |
| 42005 | `ErrEventTypeTTLInvalid` | `default_ttl` <= 0 |
| 42006 | `ErrEventTypeRangeInvalid` | `range` < 0 |
| 42007 | `ErrEventTypeExtValueInvalid` | 扩展字段值不符合 schema 约束 |
| 42010 | `ErrEventTypeVersionConflict` | 乐观锁版本冲突 |
| 42011 | `ErrEventTypeNotFound` | 事件类型不存在 |
| 42015 | `ErrEventTypeEditNotDisabled` | 启用中不允许编辑 |
| 42022 | `ErrExtSchemaNotFound` | 扩展字段定义不存在 |
| 42023 | `ErrExtSchemaDisabled` | 扩展字段已停用 |
| 40000 | `ErrBadRequest` | 格式/必填校验失败 |

### 边界 case

- `name` 不可修改。
- 扩展字段全量替换：未出现的 key 被移除。
- `perception_mode == "global"` 时 `range` 强制置 0。

---

## 功能 5：删除事件类型

### 场景描述

管理员彻底移除一个不再需要的事件类型。

两道门槛：
1. **必须先停用** → `42012 ErrEventTypeDeleteNotDisabled`
2. **不能有引用** → `42008 ErrEventTypeRefDelete`（本期 ref_count 恒为 0，规则占位但永远放行）

软删除（`deleted=1`），不是物理删除。软删后 `name` 仍然占用唯一性，不可复用（与字段/模板一致）。

### 校验规则

- **Handler**：`id > 0`
- **Service**：`getOrNotFound` → `enabled=false` 校验 → 软删

### 调用链

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/event-types/delete` |
| Handler | `EventTypeHandler.Delete` — 校验 `id > 0` |
| Service | `EventTypeService.Delete` — `getOrNotFound` → `enabled=false` 校验 → `SoftDelete` → 清缓存 |
| Store | `EventTypeStore.SoftDelete` |

### 错误码

| 错误码 | 常量 | 触发场景 |
|--------|------|---------|
| 42008 | `ErrEventTypeRefDelete` | 被引用无法删除（本期占位，永远放行） |
| 42011 | `ErrEventTypeNotFound` | 事件类型不存在 |
| 42012 | `ErrEventTypeDeleteNotDisabled` | 删除前必须先停用 |
| 40000 | `ErrBadRequest` | `id` 不合法 |

### 边界 case

- 本期无 TOCTOU 防护（无 `event_type_refs` 表），等 FSM/BT 上线后补 `FOR SHARE` 锁。
- 软删后 `name` 仍占用唯一性。

---

## 功能 6：name 唯一性校验

### 场景描述

在事件类型管理页新建时，管理员输入 `name` 后失焦，前端实时告知这个名字能不能用。

含软删除记录都视为已占用。理由和字段管理功能 6 同构：`name` 会进 `config_json` 并导出给游戏服务端，历史配置里可能有这个 key 的引用，复用会导致语义错乱。

`EventTypeStore.ExistsByName` 查询**不过滤 `deleted` 列**。

### 校验规则

- **Handler**：`name` 非空。

### 调用链

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/event-types/check-name` |
| Handler | `EventTypeHandler.CheckName` — 校验 `name` 非空 |
| Service | `EventTypeService.CheckName` — `EventTypeStore.ExistsByName` → 返回 `{available, message}` |

### 错误码

| 错误码 | 常量 | 触发场景 |
|--------|------|---------|
| 40000 | `ErrBadRequest` | `name` 为空 |

### 边界 case

- 含软删除记录也视为已占用。

---

## 功能 7：启用/停用切换

### 场景描述

**场景 A — 管理员新建完事件类型、确认配置无误后启用它。** 启用后 FSM/BT 条件编辑器才能看到。

**场景 B — 管理员下线一个事件类型，先停用它。** 停用后：
- FSM/BT 条件编辑器下拉列表立刻看不到它了
- 已经引用它的 FSM/BT 不受影响（未来实现，本期无引用）
- 如果确认不再需要，后续再执行删除

停用一个被引用的事件类型**允许**——这是"存量不动增量拦截"的标准姿势。

切换使用乐观锁，版本冲突返回 `42010`。

### 校验规则

- **Handler**：`id > 0`、`version > 0`。
- **Service**：`getOrNotFound` → 乐观锁更新。

### 调用链

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/event-types/toggle-enabled` |
| Handler | `EventTypeHandler.ToggleEnabled` — 校验 `id > 0`、`version > 0` |
| Service | `EventTypeService.ToggleEnabled` — `getOrNotFound` → 乐观锁更新 → 清自身 detail + 列表缓存 |
| Store | `EventTypeStore.ToggleEnabled(id, enabled, version)` |

### 错误码

| 错误码 | 常量 | 触发场景 |
|--------|------|---------|
| 42010 | `ErrEventTypeVersionConflict` | 乐观锁版本冲突 |
| 42011 | `ErrEventTypeNotFound` | 事件类型不存在 |
| 40000 | `ErrBadRequest` | `id` / `version` 不合法 |

### 边界 case

- 被引用的事件类型也可以停用（本期无引用，规则占位）。

---

## 功能 8：扩展字段 Schema 列表

### 场景描述

运营/管理员在 Schema 管理页的"事件类型扩展字段"tab 浏览所有已定义的扩展字段。

**展示列**：`id / field_name / field_label / field_type / enabled / sort_order / created_at`。

支持按 `enabled` 筛选，按 `sort_order ASC, id ASC` 排序。

### 校验规则

无。

### 调用链

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/event-type-schema/list` |
| Handler | `EventTypeSchemaHandler.List` |
| Service | `EventTypeSchemaService.List` |
| Store | `EventTypeSchemaStore.List` |

**两个独立接口**：

1. `EventTypeSchemaService.List` — 给 Schema 管理页的表格用，走 MySQL 查询（量小直查，不走 Redis）
2. `EventTypeSchemaService.ListEnabled` — 给事件类型详情/表单用，走内存缓存 `EventTypeSchemaCache`，启动时全量加载，写后 invalidate + reload

### 错误码

无专属错误码。

### 边界 case

- Schema 数据量小，直查 MySQL 不走 Redis。
- 内存缓存 `EventTypeSchemaCache` 用于事件类型详情/表单场景。

---

## 功能 9：扩展字段 Schema 新增

### 场景描述

运营想给所有事件类型加一个新字段 `priority (int, 1-10)`，让今后新建事件时表单多一个输入框。

**填写项：**
- `field_name`：扩展字段 key，符合 `^[a-z][a-z0-9_]*$`，唯一（含软删除）
- `field_label`：中文名，长度 ≤ 128
- `field_type`：`int / float / string / bool / select`（**不支持 reference**，扩展字段不嵌套）
- `constraints`：按 type 提供约束（min/max/pattern/options/...），复用字段管理的 `FieldConstraint*.vue` 组件（reference 除外）
- `default_value`：**仅用于新建事件类型时前端表单的初始值提示**，**不回填历史事件类型**，**不进导出**
- `sort_order`：表单展示顺序

### 校验规则

- **Handler**：`field_name` 正则 + 长度 / `field_label` 长度 / `field_type` 枚举 / `constraints` JSON 对象形状 / `default_value` 非空
- **Service**：
  - `field_name` 唯一性（含软删除；与字段管理的 `fields.name` **独立命名空间**，同名 `priority` 在两处含义不同）→ `42020`
  - `constraints` 自洽校验（比如 int 的 `max >= min`）→ `42025`
  - `default_value` 必须符合 `constraints` → `42026`
  - 写 MySQL → `EventTypeSchemaCache.Reload()`

### 调用链

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/event-type-schema/create` |
| Handler | `EventTypeSchemaHandler.Create` — 格式校验 |
| Service | `EventTypeSchemaService.Create` — 唯一性 → constraints 自洽 → default 符合 constraints → 写 MySQL → `EventTypeSchemaCache.Reload` |
| Store | `EventTypeSchemaStore.Create` |

### 错误码

| 错误码 | 常量 | 触发场景 |
|--------|------|---------|
| 42020 | `ErrExtSchemaNameExists` | `field_name` 已存在（含软删除） |
| 42021 | `ErrExtSchemaNameInvalid` | `field_name` 格式不合法 |
| 42024 | `ErrExtSchemaTypeInvalid` | `field_type` 枚举非法 |
| 42025 | `ErrExtSchemaConstraintsInvalid` | constraints 不自洽 |
| 42026 | `ErrExtSchemaDefaultInvalid` | default_value 不符合 constraints |
| 40000 | `ErrBadRequest` | `field_label` / `constraints` 格式错误 |

### 边界 case

- **与历史数据的关系**：
  - **完全不触碰 `event_types` 表任何一行**
  - 已有事件类型的 `config_json` 里没这个 key → 导出给服务端时也没有 → 服务端 `Extensions map` 里没这个 key → 消费代码返回默认值
  - 这是扩展字段设计的核心优势：**零回填、强一致、无漂移**
- `field_name` 与字段管理的 `fields.name` 是独立命名空间。

---

## 功能 10：扩展字段 Schema 编辑

### 场景描述

运营修改某个扩展字段的 label / constraints / default / sort_order。

### 校验规则

**编辑规则：**

| 项 | 可改吗 | 备注 |
|---|---|---|
| `field_name` | 不可 | 唯一键，和 `config_json` 里已存的值绑死 |
| `field_type` | 不可 | 改类型等于旧数据全错 |
| `field_label` | 可 | 纯展示 |
| `constraints` | 可，**不做收紧拦截** | 前端弹二次确认；历史值可能不符合新约束，服务端消费时按默认值兜底 |
| `default_value` | 可 | 只影响今后新建事件类型的表单初始值 |
| `sort_order` | 可 | 纯展示 |
| `enabled` | 走功能 11 | — |

写入使用乐观锁，冲突 `42030 ErrExtSchemaVersionConflict`。

### 调用链

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/event-type-schema/update` |
| Handler | `EventTypeSchemaHandler.Update` — 格式校验 + `version > 0` |
| Service | `EventTypeSchemaService.Update` — `getOrNotFound` → enabled 必须为 false (42031) → constraints 自洽 → default 符合 constraints → 乐观锁写入 → `EventTypeSchemaCache.Reload` |
| Store | `EventTypeSchemaStore.Update`（WHERE id=? AND version=?）|

### 错误码

| 错误码 | 常量 | 触发场景 |
|--------|------|---------|
| 42022 | `ErrExtSchemaNotFound` | 扩展字段定义不存在 |
| 42025 | `ErrExtSchemaConstraintsInvalid` | constraints 不自洽 |
| 42026 | `ErrExtSchemaDefaultInvalid` | default_value 不符合 constraints |
| 42030 | `ErrExtSchemaVersionConflict` | 乐观锁版本冲突 |
| 42031 | `ErrExtSchemaEditNotDisabled` | 编辑前必须先停用 |
| 40000 | `ErrBadRequest` | 格式校验失败 |

### 边界 case

- **为什么不拦截收紧**：字段管理能拦截是因为 `ref_count` 能精确反映引用方数量；这里"被引用"意味着"某条事件类型的 config_json 里有这个 key"，查询代价是全表 JSON 扫描。本期选放行 + 前端弹窗提示的务实方案。
- `field_name` 和 `field_type` 不可修改。

---

## 功能 11：扩展字段 Schema 启用/停用/删除

### 场景描述

**启用/停用**：乐观锁切换。

- **停用后**：新建/编辑事件类型的表单不再显示这个字段；**已有事件类型的 `config_json` 不动**，下次导出仍然携带这个字段的值给服务端。服务端消费代码不感知 schema 禁用状态，按原逻辑处理。这保证了停用是纯 ADMIN 侧的表单过滤动作，不触发数据变更。

**删除**：两道门槛（和事件类型删除同构）：

1. 必须先停用 → `42027 ErrExtSchemaDeleteNotDisabled`
2. 软删（`deleted=1`），**不对 `event_types.config_json` 做 `JSON_REMOVE`**

### 校验规则

- **启用/停用**：`id > 0`、`version > 0`、乐观锁更新
- **删除**：`id > 0`、`enabled=false` 校验

### 调用链

**启用/停用：**

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/event-type-schema/toggle-enabled` |
| Handler | `EventTypeSchemaHandler.ToggleEnabled` — 校验 `id > 0`、`version > 0` |
| Service | `EventTypeSchemaService.ToggleEnabled` — 乐观锁更新 → `EventTypeSchemaCache.Reload` |
| Store | `EventTypeSchemaStore.ToggleEnabled` |

**删除：**

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/event-type-schema/delete` |
| Handler | `EventTypeSchemaHandler.Delete` — 校验 `id > 0` |
| Service | `EventTypeSchemaService.Delete` — `getOrNotFound` → `enabled=false` 校验 → 软删 → `EventTypeSchemaCache.Reload` |
| Store | `EventTypeSchemaStore.SoftDelete` |

### 错误码

| 错误码 | 常量 | 触发场景 |
|--------|------|---------|
| 42022 | `ErrExtSchemaNotFound` | 扩展字段定义不存在 |
| 42027 | `ErrExtSchemaDeleteNotDisabled` | 删除前必须先停用 |
| 42030 | `ErrExtSchemaVersionConflict` | 乐观锁版本冲突（启用/停用） |
| 40000 | `ErrBadRequest` | `id` / `version` 不合法 |

### 边界 case

- **为什么软删不清 JSON：**
  - 清了不可恢复，误删风险高
  - 服务端已经能靠 `Extensions map` + 默认值兜底，留在 JSON 里不构成危害
  - 代价：`config_json` 会累积历史字段，需要手工 SQL + 审计清理（放到毕设后优化）

---

## 功能 12：导出 API

### 场景描述

游戏服务端启动时拉取全部事件类型配置。

- 路径：`GET /api/configs/event_types`
- 实现：`SELECT name, config_json FROM event_types WHERE deleted=0 AND enabled=1 ORDER BY id`
- 返回：`{"items": [{"name": "...", "config": <config_json 原样展开>}, ...]}`

### 校验规则

无（内网信任域，不鉴权）。

### 调用链

| 层 | 入口 |
|---|---|
| Router | `GET /api/configs/event_types` |
| Handler | `ExportHandler.EventTypes` |
| Service | `EventTypeService.ExportAll` |
| Store | `EventTypeStore.ExportAll` — `SELECT name, config_json FROM event_types WHERE deleted=0 AND enabled=1` |

### 错误码

无专属错误码。

### 边界 case

- `config` 字段把 `config_json` 列的 JSON **原样**塞进响应体，**不做 struct 中转**。这样任何扩展字段都自动透传，ADMIN 后端代码无需 per-field 处理。
- 只导 `enabled=1 AND deleted=0` 的记录。
- 不分页、不鉴权（内网信任域）、本期不缓存响应体。未来量上来后加 Redis 整包缓存。

---

## 横切关注点

| 关注点 | 实现方式 |
|---|---|
| 操作标识 | 主键 ID (BIGINT)，`name` 仅用于创建和唯一性校验 |
| 统一响应格式 | `handler.WrapCtx` 泛型包装 |
| 错误码段位 | 事件类型 `42001-42019`，扩展字段 schema `42020-42039` |
| 缓存穿透防护 | 空值标记，`EventTypeCache.SetDetail` 对 `nil` 也写 |
| 缓存击穿防护 | `GetByID` 使用 `TryLock(id, 3s)` + double-check |
| 缓存雪崩防护 | TTL 加随机 jitter |
| 列表缓存 | 版本号批量失效 |
| 扩展 Schema 内存缓存 | 启动时全量加载到 `EventTypeSchemaCache`，写后 invalidate + reload |
| 乐观锁 | `UPDATE ... WHERE id=? AND version=?`，rows=0 → `42010` / `42030` |
| 软删除 | `deleted=1`，所有查询过滤 `WHERE deleted=0`；`name` / `field_name` 唯一性不过滤 deleted |
| 输入校验分层 | Handler 格式校验，Service 业务校验 |
| 编辑限制 | 只有未启用状态才能编辑（`42015`） |
| 跨模块边界 | `EventTypeService` 只持有自身 store/cache；未来 FSM/BT 对接时通过 `*Tx` 方法加入跨模块事务 |
| 约束校验复用 | 抽出 `service/constraint/validate.go`，字段管理和事件类型扩展字段共用 |

---

## 错误码速查

**事件类型段 42001-42019：**

| 错误码 | 常量 | 含义 |
|---|---|---|
| 42001 | `ErrEventTypeNameExists` | name 已存在（含软删除） |
| 42002 | `ErrEventTypeNameInvalid` | name 格式不合法 |
| 42003 | `ErrEventTypeModeInvalid` | perception_mode 枚举非法 |
| 42004 | `ErrEventTypeSeverityInvalid` | severity 不在 0-100 |
| 42005 | `ErrEventTypeTTLInvalid` | ttl <= 0 |
| 42006 | `ErrEventTypeRangeInvalid` | range < 0 |
| 42007 | `ErrEventTypeExtValueInvalid` | 扩展字段值不符合 schema 约束 |
| 42008 | `ErrEventTypeRefDelete` | 被引用无法删除（本期占位，永远放行） |
| 42010 | `ErrEventTypeVersionConflict` | 乐观锁版本冲突 |
| 42011 | `ErrEventTypeNotFound` | 事件类型不存在 |
| 42012 | `ErrEventTypeDeleteNotDisabled` | 删除前必须先停用 |
| 42015 | `ErrEventTypeEditNotDisabled` | 编辑前必须先停用 |

**扩展字段 Schema 段 42020-42039：**

| 错误码 | 常量 | 含义 |
|---|---|---|
| 42020 | `ErrExtSchemaNameExists` | field_name 已存在（含软删除） |
| 42021 | `ErrExtSchemaNameInvalid` | field_name 格式不合法 |
| 42022 | `ErrExtSchemaNotFound` | 扩展字段定义不存在 |
| 42023 | `ErrExtSchemaDisabled` | 扩展字段已停用，不能被引用 |
| 42024 | `ErrExtSchemaTypeInvalid` | field_type 枚举非法 |
| 42025 | `ErrExtSchemaConstraintsInvalid` | constraints JSON 不自洽 |
| 42026 | `ErrExtSchemaDefaultInvalid` | default_value 不符合 constraints |
| 42027 | `ErrExtSchemaDeleteNotDisabled` | 删除前必须先停用 |
| 42030 | `ErrExtSchemaVersionConflict` | 版本冲突（乐观锁） |
| 42031 | `ErrExtSchemaEditNotDisabled` | 编辑前必须先停用 |

---

## 本期不做（待 FSM/BT 上线后补足）

| 项 | 现状 | 计划 |
|---|---|---|
| `event_type_refs` 表 | 不建 | FSM/BT 模块开发时创建 + 补反向引用接口 |
| `ref_count` 字段 | 不存 | 同上 |
| 删除 TOCTOU 防护 | 无 | 同上，事务内 `FOR SHARE` 检查 `event_type_refs` |
| 跨模块对外接口 | 只留签名 stub | 同上，实现 `ValidateEventTypesFor* / AttachTo*Tx / DetachFrom*Tx / GetByIDsLite / InvalidateDetails` |
| Schema 编辑收紧检查 | 不做 | 视运营反馈决定 |
| `config_json` 历史字段清理 | 无 | 放到毕设后优化 |
| 导出响应 Redis 整包缓存 | 无 | 量上来后再加 |
| 引用详情接口 | 不做 | FSM/BT 上线后补 |
| 系统字段热更新 | 无，服务端启动时一次性加载 | 服务端未来加热加载接口 |
| 服务端默认值对照 API | 无 | 未来可加 `GET /api/runtime/event-type-defaults` |
