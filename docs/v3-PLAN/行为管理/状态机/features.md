# 状态机管理 — 功能清单

> 通用架构/规范见 `docs/architecture/overview.md` 和 `docs/development/`。
> 后端实现细节见同目录 `backend.md`，前端设计见 `frontend.md`。

---

## 模块定位

定义 NPC 有哪些状态、什么条件下在状态间切换。状态机是 NPC 行为系统的顶层调度器——每个 NPC 在任意时刻处于某个状态（如 idle / chase / attack），转换规则（transition）描述"在什么条件下从 A 跳到 B"，条件表达式引用黑板（BB）Key 做比较运算。

在系统中的角色：
- **上游依赖**：条件表达式的 Key 来自 BB Key（字段标识 + 运行时 Key 表）
- **下游消费**：被 NPC 管理模块的 `behavior.fsm_ref` 引用；行为树模块按 `状态名 → bt_ref` 挂接到状态机的每个状态上
- **导出 API**：`GET /api/configs/fsm_configs` 输出 `{items: [{name, config}]}`，游戏服务端启动时一次性拉取
- **BB Key 引用追踪**：Create/Update/Delete 通过跨模块事务维护 `field_refs`（`ref_type='fsm'`），实现条件中 BB Key 对字段的反向引用

---

## 状态模型（生命周期）

| 状态 | 管理页表现 | NPC/BT 编辑器看到 | 能被新引用 | 已有引用 |
|---|---|---|---|---|
| 停用（默认） | 可见，可编辑/删除 | 不可见 | 拒绝 | 保持不动 |
| 启用 | 可见，禁止编辑/删除 | 可见可选 | 允许 | 正常 |
| 已删除 | 不可见 | 不可见 | 不可能 | 已清理 |

核心原则与字段/模板/事件类型同构：**创建默认停用（给"配置窗口期"）→ 停用态可编辑可删除 → 启用态禁止编辑禁止删除**。

**关于 NPC 引用计数**：NPC 管理尚未开发，本期**不建** `fsm_config_refs` 表和 `ref_count` 列。错误码 `43012 ErrFsmConfigRefDelete` 标记"占位未接入"，Service 层检查永远放行。等 NPC 管理上线时再做迁移加列 + 补反向接口。

---

## 数据模型

### 顶层字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | FSM 唯一标识（如 `wolf_fsm`），`^[a-z][a-z0-9_]*$`，创建后不可变，软删后不可复用 |
| `display_name` | string | 中文名，列表搜索用 |
| `config_json` | JSON | 完整配置，导出 API 原样输出 |
| `enabled` | bool | 启用状态，创建默认 `false` |
| `version` | int | 乐观锁版本号 |

### config_json 内部结构

```json
{
  "initial_state": "idle",
  "states": [
    {"name": "idle"},
    {"name": "chase"},
    {"name": "attack"}
  ],
  "transitions": [
    {
      "from": "idle",
      "to": "chase",
      "priority": 2,
      "condition": {
        "key": "player_distance",
        "op": "<",
        "value": 80
      }
    },
    {
      "from": "chase",
      "to": "attack",
      "priority": 1,
      "condition": {
        "and": [
          {"key": "player_distance", "op": "<", "value": 10},
          {"key": "stamina", "op": ">", "ref_key": "attack_threshold"}
        ]
      }
    }
  ]
}
```

#### states

状态列表，每项只有 `name` 字段。状态名在同一 FSM 内唯一、不能为空。

Go 结构体：

```go
type FsmState struct {
    Name string `json:"name"`
}
```

#### transitions

转换规则列表，每项包含：

| 字段 | 类型 | 说明 |
|------|------|------|
| `from` | string | 起始状态，必须在 states 中 |
| `to` | string | 目标状态，必须在 states 中 |
| `priority` | int | 优先级 >= 0，数字越大越优先 |
| `condition` | object | 条件树（见下文） |

Go 结构体：

```go
type FsmTransition struct {
    From      string       `json:"from"`
    To        string       `json:"to"`
    Priority  int          `json:"priority"`
    Condition FsmCondition `json:"condition"`
}
```

#### condition（条件树）

对齐游戏服务端 `rule.Condition` 结构，支持三种形态：

**1. 空条件（所有字段为零值）**：无条件转换，始终 true。`FsmCondition.IsEmpty()` 判定：`Key == "" && len(And) == 0 && len(Or) == 0`。

**2. 叶节点**：

| 字段 | 类型 | JSON tag | 说明 |
|------|------|----------|------|
| `key` | string | `json:"key,omitempty"` | BB Key 标识 |
| `op` | string | `json:"op,omitempty"` | 操作符：`==` `!=` `>` `>=` `<` `<=` `in` |
| `value` | json.RawMessage | `json:"value,omitempty"` | 比较值（JSON 任意类型），与 `ref_key` 二选一 |
| `ref_key` | string | `json:"ref_key,omitempty"` | 引用另一个 BB Key 作为比较值，与 `value` 二选一 |

**3. 组合节点**：

| 字段 | 类型 | JSON tag | 说明 |
|------|------|----------|------|
| `and` | []FsmCondition | `json:"and,omitempty"` | 子条件数组，全部为 true 才通过 |
| `or` | []FsmCondition | `json:"or,omitempty"` | 子条件数组，任一为 true 即通过 |

Go 结构体：

```go
type FsmCondition struct {
    Key    string          `json:"key,omitempty"`
    Op     string          `json:"op,omitempty"`
    Value  json.RawMessage `json:"value,omitempty"`
    RefKey string          `json:"ref_key,omitempty"`
    And    []FsmCondition  `json:"and,omitempty"`
    Or     []FsmCondition  `json:"or,omitempty"`
}
```

**互斥约束**：
- `key` 与 `and/or` 不能共存（叶节点与组合节点互斥）
- `and` 与 `or` 不能共存（同一层只能选一种组合方式）
- `value` 与 `ref_key` 不能同时设置（叶节点比较值二选一）
- `value` 与 `ref_key` 不能同时为空（叶节点必须有比较值）

---

## BB Key 引用追踪

条件树中的 `key` 和 `ref_key` 引用的是黑板（BB）Key，这些 Key 可能来自字段表（字段标识暴露为 BB Key）或运行时 Key 表。为维护字段的反向引用关系，Create/Update/Delete 操作通过跨模块事务写入 `field_refs` 表。

### 追踪规则

- **只追踪来自字段表的 Key**：`ExtractBBKeys` 提取条件树中所有 `key` 和 `ref_key`，`SyncFsmBBKeyRefs` 内部按 name 批量查字段表，查不到的（运行时 Key）自动跳过
- **ref_type = 'fsm'**：`field_refs` 表中 `ref_type` 为 `util.RefTypeFsm`（值 `"fsm"`）
- **diff 策略**：Update 时对比旧/新 BB Key 集合，只操作差异部分（toAdd / toRemove）
- **事务保证**：BB Key 引用写入与 FSM 配置写入在同一个 MySQL 事务中，要么全成功要么全回滚

### 事务编排（handler 层）

| 操作 | 流程 |
|------|------|
| Create | handler 开 tx → `CreateInTx` → `ExtractBBKeys(transitions)` → `SyncFsmBBKeyRefs(emptyOld, newKeys)` → commit → 清缓存 |
| Update | handler 开 tx → `UpdateInTx`（返回旧 FsmConfig）→ `ExtractBBKeysFromConfigJSON(old.ConfigJSON)` + `ExtractBBKeys(new.Transitions)` → `SyncFsmBBKeyRefs(oldKeys, newKeys)` → commit → 清缓存 |
| Delete | handler 开 tx → `SoftDeleteInTx` → `CleanFsmBBKeyRefs(fsmID)` → commit → 清缓存 |

### 缓存失效

事务 commit 后，handler 调用：
- `fsmConfigService.InvalidateDetail(id)` + `fsmConfigService.InvalidateList()`：清 FSM 缓存
- `fieldService.InvalidateDetails(affectedFieldIDs)`：清受影响字段的详情缓存

---

## 校验规则

### Handler 层（格式校验）

| 规则 | 说明 |
|------|------|
| name 非空 | 不能为空字符串 |
| name 格式 | 匹配 `^[a-z][a-z0-9_]*$` |
| name 长度 | <= `fsm_config.name_max_length`（默认 64） |
| display_name 非空 | 不能为空 |
| display_name 长度 | 字符数 <= `fsm_config.display_name_max_length`（默认 128） |
| ID 合法 | 正整数 |
| version 合法 | 正整数 |

### Service 层（业务校验）

| 规则编号 | 规则 | 错误码 |
|----------|------|--------|
| R10 | name 唯一性（含软删除，永久不可复用） | 43001 |
| R11 | states 不能为空 | 43004 |
| R11b | 状态数量 <= `max_states`（默认 50） | 43004 |
| R12 | 状态名非空且不重复 | 43005 |
| R13 | `initial_state` 必须是 states 中的某个 | 43006 |
| R14 | transition 的 `from`/`to` 必须在 states 中 | 43007 |
| R14b | 转换规则数量 <= `max_transitions`（默认 200） | 43007 |
| R15 | `priority` >= 0 | 43007 |
| R16 | condition 递归校验（见下文） | 43008 |

### 条件树递归校验（R16）

- 空条件 → 通过（无条件转换）
- 嵌套深度 <= `condition_max_depth`（默认 10）
- 叶节点与组合节点互斥（`key` 和 `and/or` 不能共存）
- `and` 与 `or` 不能共存
- 叶节点：`op` 必须在白名单中（`==` `!=` `>` `>=` `<` `<=` `in`）
- `value` 和 `ref_key` 不能同时为空
- `value` 和 `ref_key` 不能同时非空

### 生命周期约束

| 操作 | 前置条件 | 错误码 |
|------|----------|--------|
| 编辑 | 必须已停用 | 43010 |
| 删除 | 必须已停用 | 43009 |
| 启用/停用切换 | 乐观锁 version 匹配 | 43011 |

---

## 功能列表

### 功能 1：状态机列表

分页列表，支持 `display_name` 模糊搜索 + `enabled` 三态筛选（nil=全部 / true=仅启用 / false=仅停用）。

列表项字段：`id / name / display_name / enabled / created_at`，以及从 `config_json` 中 Service 层 unmarshal 抽取的 `initial_state` 和 `state_count`。

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fsm-configs/list` |
| Handler | `FsmConfigHandler.List` — 直接透传 query |
| Service | `FsmConfigService.List` — 分页校正 → Redis 列表缓存 → miss → MySQL → unmarshal config_json 抽展示字段 → 写缓存 |
| Store | `FsmConfigCache.GetList` → `FsmConfigStore.List`（覆盖索引 `idx_list`）→ `FsmConfigCache.SetList` |

### 功能 2：新建状态机

创建状态机配置，默认停用。**跨模块事务**：在同一 MySQL 事务中写 `fsm_configs` + 维护 `field_refs` BB Key 引用。

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fsm-configs/create` |
| Handler | `FsmConfigHandler.Create` — name/displayName 格式校验 → 开事务 → CreateInTx → ExtractBBKeys → SyncFsmBBKeyRefs → commit → 清缓存 |
| Service | `FsmConfigService.CreateInTx` — name 唯一性 → 配置完整性校验 → 组装 config_json → store.CreateTx（不清缓存，由 handler commit 后清） |

请求体：`{name, display_name, initial_state, states, transitions}`
响应体：`{id, name}`

### 功能 3：状态机详情

返回完整配置，config_json unmarshal 展开到 `config` 字段。

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fsm-configs/detail` |
| Handler | `FsmConfigHandler.Get` — ID 校验 → unmarshal config_json |
| Service | `FsmConfigService.GetByID` — Cache-Aside + 分布式锁防击穿 + 空标记防穿透 |

响应体：`{id, name, display_name, enabled, version, created_at, updated_at, config: {initial_state, states, transitions}}`

### 功能 4：编辑状态机

必须已停用才能编辑，乐观锁防并发。name 创建后不可变。**跨模块事务**：在同一 MySQL 事务中更新 `fsm_configs` + diff BB Key 引用。

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fsm-configs/update` |
| Handler | `FsmConfigHandler.Update` — ID/version/displayName 格式校验 → 开事务 → UpdateInTx（返回旧 FsmConfig）→ 提取 old/new BB Keys → SyncFsmBBKeyRefs → commit → 清缓存 |
| Service | `FsmConfigService.UpdateInTx` — 查存在性 → 必须已停用 → 配置完整性校验 → 组装 config_json → 乐观锁更新 → 返回旧 FsmConfig（handler 用于 BB Key diff） |

请求体：`{id, display_name, initial_state, states, transitions, version}`
响应体：`"保存成功"`

### 功能 5：删除状态机

软删除，必须先停用。**跨模块事务**：在同一 MySQL 事务中软删 `fsm_configs` + 清理所有 BB Key 引用。

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fsm-configs/delete` |
| Handler | `FsmConfigHandler.Delete` — ID 校验 → 开事务 → SoftDeleteInTx → CleanFsmBBKeyRefs → commit → 清缓存 |
| Service | `FsmConfigService.SoftDeleteInTx` — 查存在性 → 必须已停用 → 软删除 → 返回旧 FsmConfig |

响应体：`{id, name, label}`

### 功能 6：标识唯一性校验

前端实时校验 name 是否可用（含格式 + 唯一性）。

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fsm-configs/check-name` |
| Handler | `FsmConfigHandler.CheckName` — 完整 `checkName()` 做正则 + 长度校验 |
| Service | `FsmConfigService.CheckName` — 查 MySQL 唯一性 |

响应体：`{available: true/false, message: "该标识可用" / "该状态机标识已存在"}`

### 功能 7：启用/停用切换

调用方指定目标 `enabled` 状态，幂等安全，乐观锁。

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fsm-configs/toggle-enabled` |
| Handler | `FsmConfigHandler.ToggleEnabled` — ID/version 校验 |
| Service | `FsmConfigService.ToggleEnabled` — 查存在性 → 乐观锁更新 → 清缓存 |

请求体：`{id, enabled, version}`
响应体：`"操作成功"`

### 功能 8：导出 API

导出所有已启用且未删除的状态机配置，供游戏服务端启动时拉取。

| 层 | 入口 |
|---|---|
| Router | `GET /api/configs/fsm_configs` |
| Store | `FsmConfigStore.ExportAll` — `SELECT name, config_json AS config WHERE deleted=0 AND enabled=1 ORDER BY id` |

响应格式：`{items: [{name: "wolf_fsm", config: {...}}]}`

---

## 依赖关系

```
BB Key（字段标识 + 运行时 Key 表）
  ↓ condition 中的 key / ref_key
  ↓ field_refs (ref_type='fsm') 反向追踪
状态机 (FSM)
  ↓ NPC.behavior.fsm_ref
  ↓ BT 按状态名挂接
NPC / 行为树
```

- **上游**：condition 表达式中的 `key` 和 `ref_key` 来自黑板（Blackboard）Key。Create/Update/Delete 通过跨模块事务写 `field_refs` 维护反向引用
- **下游**：NPC 管理模块通过 `behavior.fsm_ref` 字段引用状态机 name；行为树按 `状态名 → bt_ref` 映射挂接到状态机的每个状态
- 本期因 NPC/BT 管理未上线，`ref_count` 恒为 0，删除不做 NPC 引用检查（TODO: NPC 管理上线后加引用检查）
