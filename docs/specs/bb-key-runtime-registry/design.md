# 设计：运行时 BB Key 注册表

> 本设计对应 [requirements.md](requirements.md)。

## 0. 对 requirements 的修订

**补 §R11 type 规范化枚举**：requirements 说前端 emit `'integer' / 'float' / 'string' / 'bool'` 四枚举，但没说数据库存什么。本 design 锁定：

| Server `keys.go` Go 类型 | ADMIN `runtime_bb_keys.type` 存值 | 分布 |
|---|---|---|
| `float64` | `"float"` | 13 个 |
| `int64` | `"integer"` | 4 个 |
| `string` | `"string"` | 12 个 |
| `bool` | `"bool"` | 2 个 |

**13 + 4 + 12 + 2 = 31**，与 Server [`blackboard/keys.go`](../../../../NPC-AI-Behavior-System-Server-v1/internal/core/blackboard/keys.go) 锁定一致。

type 字段在 DB 走 enum CHECK 约束（`CHECK (type IN ('integer','float','string','bool'))`）防止 seed / UI 写入非法值。

**补 §R3 grouping 机制**：requirements §场景 1 要求下拉"分节呈现"按 11 组，但没说存哪。本 design 锁定：

- `runtime_bb_keys.group_name` 字段（VARCHAR(32) NOT NULL），存组名（如 `threat` / `event` / `fsm` / `npc` / `action` / `need` / `emotion` / `memory` / `social` / `decision` / `move`）
- seed 时硬编码 11 组映射（与 Server `keys.go` 分节注释逐字对齐）
- 前端 `BBKeySelector` 下拉按 `group_name` 分组渲染
- **不**做 `runtime_key_groups` 独立表：11 组规模小、变动低频、无管理 UI 需求，过度规范化违反 [red-lines/general.md §禁止过度设计](../../development/standards/red-lines/general.md)

**补 §R13 toggle 语义**：requirements 说"停用仅阻断新建引用，不影响历史数据"。本 design 锁定：

- `enabled=0` 时 `BBKeySelector` 下拉**隐藏**该 key（策划看不到）
- `enabled=0` 时 `RuntimeBbKeyService.CheckByNames` 返回该 key 为 `notOK`（新建 FSM/BT 引用它时 400）
- 但 `runtime_bb_key_refs` 表中**既有**引用不级联删除（历史 FSM 导出保留该 key 引用）
- 对称现有 `field` 模块的 `enabled=0` 语义（见 [field.go:checkFieldEnabled](../../backend/internal/service/field.go) 模式）

---

## 1. 方案描述

### 1.1 整体架构：新模块（非侵入现有模块）

```
┌─────────────────────────────────────────────────────────────┐
│ 前端                                                          │
│   ┌─────────────────────────────────────────┐               │
│   │ BBKeySelector.vue                       │               │
│   │   ├─ 字段 group（既有）                   │               │
│   │   ├─ 事件扩展字段 group（既有）            │               │
│   │   └─ 运行时 Key group（新增第三路）        │               │
│   └─────────────────────────────────────────┘               │
│   ┌─────────────────────────────────────────┐               │
│   │ RuntimeBbKeyList.vue + Form.vue（新增页面）│               │
│   └─────────────────────────────────────────┘               │
└─────────────────────────────────────────────────────────────┘
         ↓                       ↓
┌─────────────────────────────────────────────────────────────┐
│ 后端                                                          │
│   ┌─────────────────────────────────────────┐               │
│   │ handler/runtime_bb_key.go（新增）         │               │
│   │   → /api/v1/runtime-bb-keys/*            │               │
│   └─────────────────────────────────────────┘               │
│            ↓                                                 │
│   ┌─────────────────────────────────────────┐               │
│   │ service/runtime_bb_key.go（新增）         │               │
│   │   ├─ CRUD（复用 shared.Pagination/Validate）│            │
│   │   ├─ CheckByNames（FSM/BT Create 时用）   │               │
│   │   ├─ CheckNameConflictWithField          │               │
│   │   └─ SyncFsmRefs / SyncBtRefs（新增，平行于 field.go 既有）│  │
│   └─────────────────────────────────────────┘               │
│            ↓                                                 │
│   ┌─────────────────────────────────────────┐               │
│   │ store/mysql/runtime_bb_key.go（新增）     │               │
│   │ store/mysql/runtime_bb_key_ref.go（新增） │               │
│   │ store/redis/runtime_bb_key_cache.go（新增）│               │
│   └─────────────────────────────────────────┘               │
│            ↓                                                 │
│   ┌─────────────────────────────────────────┐               │
│   │ MySQL: runtime_bb_keys + runtime_bb_key_refs │           │
│   └─────────────────────────────────────────┘               │
└─────────────────────────────────────────────────────────────┘
```

**现有模块改动点**（3 处，非侵入式）：

1. [`handler/fsm_config.go`](../../backend/internal/handler/fsm_config.go) Create/Update：多一行 `runtimeBbKeyService.SyncFsmRefs(ctx, tx, fsmID, oldKeys, newKeys)`，不改现有 `fieldService.SyncFsmBBKeyRefs` 调用
2. [`handler/bt_tree.go`](../../backend/internal/handler/bt_tree.go) Create/Update：同上
3. [`components/BBKeySelector.vue`](../../frontend/src/components/BBKeySelector.vue)：下拉数据源从 2 路扩到 3 路

**不改**：[`service/field.go:898-1000`](../../backend/internal/service/field.go#L898) 的 `SyncFsmBBKeyRefs` / `SyncBtBBKeyRefs` 既有行为保留，内部注释 `"内部解析 name→field ID，只追踪来自字段表的 Key（运行时 Key 跳过）"` 早已埋下对称接入点。

### 1.2 数据结构

**表 1：`runtime_bb_keys`**

```sql
CREATE TABLE runtime_bb_keys (
    id           BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    name         VARCHAR(64)  NOT NULL COMMENT 'BB Key 名，对齐服务端 keys.go',
    type         VARCHAR(16)  NOT NULL COMMENT 'integer|float|string|bool',
    label        VARCHAR(64)  NOT NULL COMMENT '中文标签（UI 展示）',
    description  VARCHAR(255) NOT NULL DEFAULT '' COMMENT '中文描述（UI 展示）',
    group_name   VARCHAR(32)  NOT NULL COMMENT '分组（threat/event/fsm/npc/action/need/emotion/memory/social/decision/move）',
    enabled      TINYINT(1)   NOT NULL DEFAULT 1,
    version      INT UNSIGNED NOT NULL DEFAULT 1,
    deleted      TINYINT(1)   NOT NULL DEFAULT 0,
    created_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_name (name, deleted),
    KEY idx_list (deleted, enabled, group_name, id),
    CHECK (type IN ('integer','float','string','bool'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**表 2：`runtime_bb_key_refs`**

```sql
CREATE TABLE runtime_bb_key_refs (
    id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    runtime_key_id  BIGINT UNSIGNED NOT NULL,
    ref_type        VARCHAR(16) NOT NULL COMMENT 'fsm|bt',
    ref_id          BIGINT UNSIGNED NOT NULL,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uk_ref (runtime_key_id, ref_type, ref_id),
    KEY idx_reverse (ref_type, ref_id),
    CHECK (ref_type IN ('fsm','bt'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**对称决策**：结构与 [`field_refs`](../../backend/migrations/) 并列（三元组 + 反向覆盖索引 + CHECK 约束），策划动作感知一致。

**不复用 `field_refs` 表的原因**（见 §2.1 详述）：两表语义不同（一个指向 fields 表，一个指向 runtime_bb_keys 表），共用需要 `ref_kind` 区分列 + 外键限定，反而复杂化。

**model 层**（[`backend/internal/model/runtime_bb_key.go`](../../backend/internal/model/runtime_bb_key.go)）：

```go
type RuntimeBbKey struct {
    ID          int64     `db:"id"          json:"id"`
    Name        string    `db:"name"        json:"name"`
    Type        string    `db:"type"        json:"type"`
    Label       string    `db:"label"       json:"label"`
    Description string    `db:"description" json:"description"`
    GroupName   string    `db:"group_name"  json:"group_name"`
    Enabled     bool      `db:"enabled"     json:"enabled"`
    Version     int       `db:"version"     json:"version"`
    Deleted     bool      `db:"deleted"     json:"-"`
    CreatedAt   time.Time `db:"created_at"  json:"created_at"`
    UpdatedAt   time.Time `db:"updated_at"  json:"updated_at"`

    HasRefs     bool  `db:"-" json:"has_refs,omitempty"`    // 仅 detail 填充
    RefCount    int   `db:"-" json:"ref_count,omitempty"`   // 仅 detail 填充
}

type RuntimeBbKeyRef struct {
    ID           int64     `db:"id"`
    RuntimeKeyID int64     `db:"runtime_key_id"`
    RefType      string    `db:"ref_type"`   // "fsm" | "bt"
    RefID        int64     `db:"ref_id"`
    CreatedAt    time.Time `db:"created_at"`
}
```

### 1.3 错误码新增（6 个）

[`backend/internal/errcode/codes.go`](../../backend/internal/errcode/codes.go) RuntimeBbKey 段新开一个编号块（避免与 field/fsm/bt 段冲突，取 **47000 段**）：

```go
// RuntimeBbKey 段 47000-47099
ErrRuntimeBBKeyNotFound               = 47001 // 运行时 Key 不存在
ErrRuntimeBBKeyNameRequired           = 47002 // name 必填
ErrRuntimeBBKeyNameInvalid            = 47003 // name 格式非法（对齐 ^[a-z][a-z0-9_]*$）
ErrRuntimeBBKeyNameConflictWithField  = 47004 // name 与 fields 冲突
ErrRuntimeBBKeyTypeInvalid            = 47005 // type 不在 4 枚举内
ErrRuntimeBBKeyHasRefs                = 47006 // 删除时有 FSM/BT 引用
```

[`errcode/messages.go`](../../backend/internal/errcode/messages.go) 同文件追加中文提示。

**反向冲突码**在 field 段追加（复用既有 4101x 段）：

```go
ErrFieldNameConflictWithRuntimeBBKey = 41020 // 新增，name 与 runtime_bb_keys 冲突
```

### 1.4 Service 层签名

[`backend/internal/service/runtime_bb_key.go`](../../backend/internal/service/runtime_bb_key.go) 新增：

```go
type RuntimeBbKeyService struct {
    store      *storemysql.RuntimeBbKeyStore
    refStore   *storemysql.RuntimeBbKeyRefStore
    cache      *storeredis.RuntimeBbKeyCache
    fieldStore *storemysql.FieldStore   // 用于 CheckNameConflictWithField
    pagCfg     *config.PaginationConfig
}

// CRUD ---------------------------------------------------------------

func (s *RuntimeBbKeyService) List(ctx context.Context, req ListReq) (*ListResp, error)
func (s *RuntimeBbKeyService) GetByID(ctx context.Context, id int64) (*model.RuntimeBbKey, error)
func (s *RuntimeBbKeyService) Create(ctx context.Context, req CreateReq) (int64, error)
func (s *RuntimeBbKeyService) Update(ctx context.Context, req UpdateReq) error
func (s *RuntimeBbKeyService) Delete(ctx context.Context, id int64, version int) error
func (s *RuntimeBbKeyService) Toggle(ctx context.Context, id int64, enabled bool) error
func (s *RuntimeBbKeyService) CheckName(ctx context.Context, name string) (conflict bool, source string, err error)

// 跨模块校验（FSM/BT Create 时 handler 调用） -----------------------------

// CheckByNames 给定一组 key name，返回其中"非运行时 key 或已停用"的名字列表
// （用于 FSM 条件校验：未识别 key → 400）
func (s *RuntimeBbKeyService) CheckByNames(ctx context.Context, names []string) (notOK []string, err error)

// 引用同步（FSM/BT Create/Update 时 handler 在事务内调用） -----------------

// SyncFsmRefs 同步 FSM 条件树中对运行时 key 的引用
// 与 fieldService.SyncFsmBBKeyRefs 并行执行，互不干扰
func (s *RuntimeBbKeyService) SyncFsmRefs(
    ctx context.Context, tx *sqlx.Tx, fsmID int64,
    oldKeys, newKeys map[string]bool,
) (affectedKeyIDs []int64, err error)

func (s *RuntimeBbKeyService) SyncBtRefs(
    ctx context.Context, tx *sqlx.Tx, btTreeID int64,
    oldKeys, newKeys map[string]bool,
) (affectedKeyIDs []int64, err error)

// DeleteRefsByFsmID 级联：FSM 被删除时清 refs（handler 编排）
func (s *RuntimeBbKeyService) DeleteRefsByFsmID(ctx context.Context, tx *sqlx.Tx, fsmID int64) error
func (s *RuntimeBbKeyService) DeleteRefsByBtID(ctx context.Context, tx *sqlx.Tx, btID int64) error
```

**分层原则**（对齐 [red-lines/go.md §禁止分层倒置](../../development/standards/red-lines/go.md#禁止分层倒置)）：

- `RuntimeBbKeyService` **不持有** `FsmConfigService` / `BtTreeService`（跨模块调用归 handler）
- **持有** `FieldStore`（用于 name 冲突检测的反向查询；这是同层 peer 资源访问，非倒置）
- CheckByNames 返回纯数据（`[]string`），调用方（FSM handler）决定如何响应

### 1.5 Handler 层 + 路由

[`backend/internal/handler/runtime_bb_key.go`](../../backend/internal/handler/runtime_bb_key.go)：

```
POST   /api/v1/runtime-bb-keys/list        → 分页列表
GET    /api/v1/runtime-bb-keys/:id         → 详情（含 has_refs / ref_count）
POST   /api/v1/runtime-bb-keys             → 创建
PUT    /api/v1/runtime-bb-keys/:id         → 更新
DELETE /api/v1/runtime-bb-keys/:id         → 删除（存在引用 409）
POST   /api/v1/runtime-bb-keys/:id/toggle  → 启用/停用
POST   /api/v1/runtime-bb-keys/check-name  → 冲突检测（跨 fields + runtime_bb_keys）
GET    /api/v1/runtime-bb-keys/:id/references → 引用详情（FSM name 列表 + BT name 列表）
```

模式与 [`handler/field.go`](../../backend/internal/handler/field.go) 完全一致（POST list / wrap.go 包装 / shared.SuccessMsg）。

**现有 handler 的改动**（不新增函数，只插入调用）：

[`handler/fsm_config.go`](../../backend/internal/handler/fsm_config.go) Create/Update 编排流程扩展：

```go
// 既有流程（保留不动）：
tx.Begin()
fsmConfigService.Create/Update(tx, ...)         // 主操作
fieldService.SyncFsmBBKeyRefs(tx, fsmID, oldKeys, newKeys)  // 字段 key 同步

// 新增一行（在 field sync 后，tx commit 前）：
runtimeBbKeyService.SyncFsmRefs(tx, fsmID, oldKeys, newKeys)  // 运行时 key 同步

tx.Commit()
// 清缓存（新增 runtime_bb_key_cache.InvalidateByIDs + 原字段 cache）
```

**两路 sync 是幂等并行的**：同一个 `newKeys` 集合给两个 service，分别筛出自己能识别的 name，对应表分别增删。未匹配任何一方的 name 是非法（400 由 FSM validator 前置兜底，见 §6.2）。

### 1.6 Seed 策略：31 条硬编码对齐 keys.go

[`backend/cmd/seed/runtime_bb_key_seed.go`](../../backend/cmd/seed/runtime_bb_key_seed.go)：

```go
var runtimeBbKeyFixtures = []runtimeBbKeySeed{
    // --- 威胁相关 ---
    {Name: "threat_level",       Type: "float",   GroupName: "threat",  Label: "威胁等级",       Desc: "当前威胁等级 0~100，决策中心写入"},
    {Name: "threat_source",      Type: "string",  GroupName: "threat",  Label: "威胁来源",       Desc: "威胁来源 ID，决策中心写入"},
    {Name: "threat_expire_at",   Type: "integer", GroupName: "threat",  Label: "威胁过期时间",   Desc: "威胁过期时间戳（毫秒）"},
    // --- 事件相关 ---
    {Name: "last_event_type",    Type: "string",  GroupName: "event",   Label: "最近事件类型",   Desc: "最近一次感知到的事件类型"},
    {Name: "current_time",       Type: "integer", GroupName: "event",   Label: "当前时间戳",     Desc: "当前时间戳（毫秒），Runtime 每 Tick 更新"},
    // ... 完整 31 条
}

// INSERT IGNORE 幂等写入；name 已存在则跳过（不覆盖运营手改）
func seedRuntimeBbKeys(ctx context.Context, db *sqlx.DB) error { ... }
```

**对齐规则**：每条 seed 的 name / type 与 Server [`keys.go`](../../../../NPC-AI-Behavior-System-Server-v1/internal/core/blackboard/keys.go) 逐字对齐。GroupName 与 `keys.go` 的分节注释（`// --- 威胁相关 ---`）对齐，Label/Desc 从注释提炼。

**冷启断言**（加到 `scripts/verify-seed.sh`）：`SELECT COUNT(*) FROM runtime_bb_keys WHERE deleted=0` = 31。

**漂移监控**（非本 spec 实现，留 TODO）：未来可写 `make verify-runtime-keys` 对比 seed 常量与服务端 keys.go 的 `go list -f ...`，Phase 3 tasks 考虑。

### 1.7 引用同步：FSM/BT 生命周期钩子

FSM 生命周期下 4 个位置要调 `SyncFsmRefs` / `DeleteRefsByFsmID`：

| FSM 生命周期 | 既有调用 | 新增调用 |
|---|---|---|
| Create | `fieldSvc.SyncFsmBBKeyRefs(tx, fsmID, {}, newKeys)` | `runtimeBbKeySvc.SyncFsmRefs(tx, fsmID, {}, newKeys)` |
| Update | `fieldSvc.SyncFsmBBKeyRefs(tx, fsmID, oldKeys, newKeys)` | `runtimeBbKeySvc.SyncFsmRefs(tx, fsmID, oldKeys, newKeys)` |
| Delete | `fieldSvc.DeleteFieldRefsByFsmID(tx, fsmID)` | `runtimeBbKeySvc.DeleteRefsByFsmID(tx, fsmID)` |
| Toggle | 无 | 无（toggle 不改 refs） |

BT 对称。

**幂等性**：两路都用 add/remove diff 算法（见 field.go:898 的 SyncFsmBBKeyRefs 现有实现）+ `UNIQUE KEY uk_ref` 兜底，重入安全。

### 1.8 前端 BBKeySelector 三组数据源并行加载

[`frontend/src/components/BBKeySelector.vue`](../../frontend/src/components/BBKeySelector.vue) 现有结构：

```typescript
// 既有（2 组）
const exposedFields = ref<BBKeyField[]>([])        // 来自 /api/v1/fields?expose_bb=true
const eventExtraFields = ref<BBKeyField[]>([])     // 来自 /api/v1/event-type-schemas

// 新增第 3 组
const runtimeKeys = ref<BBKeyField[]>([])          // 来自 /api/v1/runtime-bb-keys/list?enabled=true
```

三个请求 `Promise.all` 并行加载。渲染用 `<el-option-group label="运行时 Key">` 按 `group_name` 次级分组（威胁/事件/FSM/.../移动）。

**类型规范化（§R11）**：

```typescript
// 现有 2 组：从 field.type 映射到 BBKeyField.type（已有逻辑）
// 新增：runtime_bb_key.type 已经是 'integer'|'float'|'string'|'bool' 四值，直接透传
const runtimeToBBKey = (k: RuntimeBbKey): BBKeyField => ({
    name: k.name,
    type: k.type,           // 已规范化，直接用
    label: k.label,
    source: 'runtime',      // 新枚举值，与 'field' / 'event_extra' 并列
})
```

`source` 字段给下游消费者（如 `FsmConditionEditor`）做运算符过滤时区分来源。

---

## 2. 方案对比

### 2.1 Ref 表策略：独立 `runtime_bb_key_refs` vs 共用 `field_refs`

**选择**：独立 `runtime_bb_key_refs` 表。

**方案 A（独立表，采纳）**：
- ✅ 语义干净：ref 表 FK 指向确切的源表
- ✅ 同步代码与 `fieldService.SyncFsmBBKeyRefs` 完全对称，新手可读
- ✅ 未来新增配置类型（如 V3 `component_schemas.blackboard_keys`）可继续加 `component_bb_key_refs` 表，扩展轴清晰

**方案 B（共用 field_refs，加 ref_kind 列）**：
- ❌ `field_refs.ref_kind='field'|'runtime'` 列承担多重语义，FK 无法限定到唯一父表
- ❌ 现有 `SyncFsmBBKeyRefs` 要扩 `WHERE ref_kind=?` 分支，侵入性强
- ❌ 跨类型 query 反而更复杂（`JOIN fields OR JOIN runtime_bb_keys` 需要 UNION）

**方案 C（前端动态分派到一个统一 /bb-keys 端点）**：完全违反"ADMIN 和服务端各存各的" + 现有模块化架构。不考虑。

### 2.2 Name 冲突检测位置：Service 单查 vs MySQL 联合唯一

**选择**：Service 单查（应用层）。

**方案 A（采纳，service 单查）**：
```go
func (s *RuntimeBbKeyService) CheckNameConflictWithField(ctx context.Context, name string) error {
    _, err := s.fieldStore.GetByName(ctx, name)
    if err == nil {
        return errcode.New(errcode.ErrRuntimeBBKeyNameConflictWithField, ...)
    }
    if errors.Is(err, sql.ErrNoRows) { return nil }
    return err
}
```
- ✅ 实现简单，错误码可精准返回"与字段 'xxx' 冲突"
- ✅ 业务校验全在应用层，调试直观
- ⚠️ 非事务保护下理论有 TOCTOU（两边同时新建同名）—— 实测 runtime key 新建频率极低（31 条初始 + 运营偶发补录），TOCTOU 概率可忽略；即使撞上也只是一方 409，无数据损坏

**方案 B（MySQL 联合唯一，假设 CREATE VIEW + UNIQUE 或 trigger）**：
- ❌ MySQL 不支持跨表 UNIQUE INDEX；方案要用 trigger（BEFORE INSERT 查对方表）—— 违反 [red-lines/mysql.md §禁止业务逻辑下沉到 trigger](../../development/standards/red-lines/mysql.md)（推断存在，无则本 design 追加一条）
- ❌ 维护成本高：trigger 调试困难、跨环境部署易漏

### 2.3 Grouping 数据存储：表列 vs seed 常量 vs 独立表

**选择**：表列 `group_name` VARCHAR(32)。

**方案对比**：

| 方案 | 优点 | 缺点 |
|---|---|---|
| A 表列（采纳） | 运营可改 label / 新增 key 时自选 group；前端下拉分组直接 `GROUP BY group_name` | 11 个 group 组名是魔术字符串（缓解：seed 写死 + CHECK enum 约束可后续加） |
| B seed 常量（硬编码 name→group 映射） | 前端拉 list 后用常量表映射 | 运营新增 key 时映射缺失 → 无 group 显示 "other"；运营层动作受限 |
| C 独立 `runtime_key_groups` 表 | 规范化到 3NF | 11 行数据 + 无管理 UI，违反 §禁止过度设计 |

### 2.4 Type 规范化：ADMIN 层 vs 前端层

**选择**：ADMIN 层（seed 时规范化）。

- seed 时直接写 `"float" / "integer" / "string" / "bool"`（不存 Go 原始类型名）
- DB CHECK 约束拒绝非枚举值
- 前端拿到数据直接用，无二次转换

**拒绝方案**：存 Go 类型名（`"float64" / "int64" / "string" / "bool"`）前端映射 —— 前端要维护 Go↔Admin 类型映射，耦合服务端实现细节。

### 2.5 Toggle 语义：隐藏下拉 + 禁新建引用 vs 其他方案

**选择**：`enabled=0` 时下拉隐藏 + `CheckByNames` 返回 notOK + 历史 refs 保留。

**理由**：对称 field 模块的 enabled=0 语义，策划心智一致；历史 FSM 仍可导出（`GET /api/configs/fsm_configs` 返回包含该 key 的 condition），服务端视 enabled 与否，只按 name 解析。

**拒绝方案**：
- "enabled=0 时级联清 refs"：会导致停用 = 批量改历史配置，运营风险高
- "enabled=0 时阻断导出"：违反 §R13 验收标准（"历史数据仍可导出"）

---

## 3. 红线检查

逐项比对 [`docs/development/standards/red-lines/*.md`](../../development/standards/red-lines/)：

### 3.1 general.md

| 红线 | 本 spec 如何规避 |
|---|---|
| 禁止静默降级 | seed 失败 → 错误向上冒；`CheckByNames` 返回 notOK 不静默 skip；前端加载失败整体报错 |
| 禁止安全隐患 | `name` 正则白名单（`^[a-z][a-z0-9_]*$`）+ MySQL prepared statement；无路径拼接 |
| 禁止 HTTP 响应格式割裂 | 复用 `wrap.go` 统一 JSON（含 code 字段） |
| 禁止测试质量低下 | e2e 不硬编码引用计数（用 `> 0` 断言） |
| 禁止过度设计 | 拒绝独立 group 表（§2.3）；拒绝 trigger（§2.2） |
| 禁止协作失序 | 本 spec 不改服务端 keys.go；契约 v1.1.3 不动 |

### 3.2 go.md

| 红线 | 本 spec 如何规避 |
|---|---|
| 禁止资源泄漏 | `store.ListRuntimeBbKeys` 用 `context.WithTimeout`（继承项目既有模式）；sqlx 自动 Close |
| 禁止序列化陷阱 | `ListResp.Items` 必 `make([]RuntimeBbKey, 0)`；`*json.RawMessage` 无用（本表无 JSON 列）|
| 禁止错误处理不当 | handler 统一 `wrap.go` 包装；error 消息经 errcode 映射，不暴露 Go 原文 |
| 禁止错误码语义混用 | 新段 47001-47006 独立，不复用其他段 |
| 禁止硬编码魔术字符串 | group_name 的 11 组走 `util/const.go` 新增常量（`RuntimeKeyGroupThreat` 等） |
| **禁止分层倒置** | Service 不持 peer service；跨模块 IO 归 handler（§1.4 明示） |

### 3.3 mysql.md

| 红线 | 本 spec 如何规避 |
|---|---|
| 禁止事务一致性破坏 | `SyncFsmRefs` / `SyncBtRefs` 接 `tx *sqlx.Tx` 参数，全程事务内；DELETE refs 与主删在同 tx |
| 禁止查询注入风险 | 所有 LIKE 用 `shared.EscapeLike`（既有 util）；name 正则拦截 $ 开头 |
| TOCTOU 防护 | Delete 前 `SELECT ... FOR SHARE` 获取当前读（对齐 field 模块的 `deleteFieldChecked` 模式）|

### 3.4 cache.md / redis.md

| 红线 | 本 spec 如何规避 |
|---|---|
| 禁止缓存与数据库不一致 | 写路径 `tx.Commit()` **后**清缓存（`DelDetail(id) → InvalidateList()`）；顺序对齐 field_cache.go 既有模式 |
| 禁止缓存失效遗漏 | toggle / delete / update 三路都清 list 缓存；写入后 detail 缓存置空（不回填，避免 TOCTOU） |
| 禁止缓存击穿不设防 | detail 读路径用既有 `redis/shared.WithLock`（分布式锁）|
| 禁止 TOCTOU 遗漏 | 级联删 refs 在 FSM/BT delete tx 内完成，非独立异步任务 |

### 3.5 frontend.md

| 红线 | 本 spec 如何规避 |
|---|---|
| 禁止数据源污染 | runtimeKeys 与 exposedFields / eventExtraFields 三路数据源独立，`source` 字段标记来源 |
| 禁止放行无效输入 | `RuntimeBbKeyForm` name 校验 regex 对齐后端；type 下拉只 4 枚举 |
| 禁止 JSON 子结构 key 各写各的 | `RuntimeBbKey` TS 接口与 Go struct json tag 对齐 |
| 禁止跳过类型检查就上线 | R14 要求 `vue-tsc --noEmit` 无报错 |

---

## 4. 扩展性影响

**扩展轴 1（新增配置类型）**：🟢 **正面验证**

本 spec 新增了完整技术栈（migration + model + store + service + handler + seed + frontend view + BBKeySelector 数据源扩展），**不动任何现有模块的 handler/service/store 内部实现**，仅在 3 个明确的集成点（FSM handler 2 行 / BT handler 2 行 / BBKeySelector 1 个 Promise）做加法式接入。

**证明扩展轴 1 可用**：未来 V3 `component_schemas.blackboard_keys` 或其他第 4 种 BB Key 来源，套本 spec 模式照搬即可，边际成本线性。

**扩展轴 2（新增表单字段）**：⚪ 无影响

本 spec 的 `RuntimeBbKeyForm` 是静态 schema（4 字段固定），不触达 SchemaForm 扩展轴；相对地，未来若要让 runtime key 支持"自定义属性"，**那才是另一条扩展轴的新 spec**。

---

## 5. 依赖方向

```
              ┌──────────────────┐
              │ handler/runtime  │
              └────────┬─────────┘
                       │
           ┌───────────┴─────────┐
           ↓                     ↓
┌──────────────────┐   ┌─────────────────┐
│ service/runtime  │   │ store/mysql     │
│ _bb_key          │   │ redis           │
└──────────────────┘   └─────────────────┘
         │
         │ 仅读
         ↓
┌──────────────────┐
│ store/mysql/     │
│ field（既有）      │
└──────────────────┘
```

- `service/runtime_bb_key` 依赖：`store/mysql/runtime_bb_key` + `runtime_bb_key_ref` + `fieldStore`（仅读）+ `store/redis/runtime_bb_key_cache` + `config`
- **不依赖**：`service/fsm_config` / `service/bt_tree` / `service/field`（对称避免层级循环）
- `handler/fsm_config` / `handler/bt_tree` **新增依赖** `service/runtime_bb_key`（编排层依赖业务 service 合法）

无循环依赖；新增依赖方向单向从上到下。

---

## 6. 陷阱检查

### 6.1 Go（[dev-rules/go.md](../../development/standards/dev-rules/go.md)）

1. **slog 结构化日志**：seed 错误、冲突检测、ref 同步的增/删计数都走 `slog.Info/Warn` + 结构化 key（`slog.Int64("fsm_id", ...)`）
2. **context 传播**：所有 store 方法首参数 `ctx context.Context`，不起 `context.Background()` 内嵌
3. **errors.Is 用法**：判断 `sql.ErrNoRows` 用 `errors.Is(err, sql.ErrNoRows)` 不用 `err == sql.ErrNoRows`（对齐既有模式）
4. **defer tx.Rollback 习惯**：`defer func() { if err != nil { tx.Rollback() } }()`；commit 后 Rollback 是 no-op，安全

### 6.2 MySQL（[dev-rules/mysql.md](../../development/standards/dev-rules/mysql.md)）

1. **`IN (?, ?, ...)` 展开**：`sqlx.In(query, slice)` 展开占位符；不手工字符串拼
2. **LIKE 转义**：`EscapeLike(req.Name) + "%"`（既有 shared util）
3. **软删 + unique 复合**：`UNIQUE KEY uk_name (name, deleted)` 保证启用+软删各一条共存（对齐 fields 表）
4. **FK 不加**：本项目不用 DB 外键（对齐既有表设计）；完整性靠 service 层

### 6.3 Redis / cache（[dev-rules/redis.md](../../development/standards/dev-rules/redis.md) + [cache.md](../../development/standards/dev-rules/cache.md)）

1. **key 管理集中化**：新增 `rcfg.RuntimeBbKeyDetailKey(id)` / `RuntimeBbKeyListKey(req)` 常量到 [`store/redis/shared/`](../../backend/internal/store/redis/shared/)
2. **TTL 策略**：detail 5 分钟 / list 1 分钟（对齐 field_cache.go 既有策略）
3. **分布式锁**：热点 detail 缓存击穿用 `WithLock(ctx, lockKey, 3s)`（既有 pattern）
4. **commit 前清缓存是错误的**：必须 `commit → 清缓存`，反之会短窗口读到旧数据；对齐 [cache.md §写后清缓存顺序]

### 6.4 前端（[dev-rules/frontend.md](../../development/standards/dev-rules/frontend.md)）

1. **Element Plus `<el-option-group>`**：每 group 需要唯一 label + items 非空否则渲染空组 —— `v-if="runtimeKeys.length"` 兜底
2. **Composition API ref unwrap**：模板内 `runtimeKeys` 不需 `.value`；JS 逻辑内必须 `.value`
3. **错误码 i18n**：47001-47006 中文提示走既有 `errMessageMap`，不硬编码到组件

---

## 7. 配置变更

无。本 spec 不引入新的环境变量 / 配置文件字段 / Docker Compose 改动。

seed 入口需要在 [`backend/cmd/seed/main.go`](../../backend/cmd/seed/main.go) 的主流程加一行 `seedRuntimeBbKeys(ctx, db)`，插在 `seedFields` 之后（runtime_bb_keys 与 fields 是对等的配置源，顺序不严格）。

---

## 8. 测试策略

### 8.1 单元测试

[`backend/internal/service/runtime_bb_key_test.go`](../../backend/internal/service/runtime_bb_key_test.go)（新增，~200 行）：

| 测试用例 | 覆盖验收标准 |
|---|---|
| `TestCreate_Success` | R4 |
| `TestCreate_NameConflictWithField` | R5 |
| `TestCreate_TypeInvalid` | 47005 |
| `TestList_WithFilter_Pagination` | R4 |
| `TestDelete_HasRefs_Rejected` | R9 |
| `TestToggle_DisabledKeyHiddenFromCheckByNames` | R13 |
| `TestSyncFsmRefs_AddAndRemove` | R7 |
| `TestSyncFsmRefs_IgnoresFieldKeys` | R7 反向（field key 不误入） |

Mock 策略：`store/mysql` / `store/redis` 走既有 `mocks.MockStore` pattern（若无则以 sqlmock）。

### 8.2 e2e（手动 curl，不进 CI）

```bash
# R4: 基础 CRUD
curl -X POST http://localhost:9821/api/v1/runtime-bb-keys -d '{"name":"test_key","type":"float","label":"测试","group_name":"threat"}'
curl -X POST http://localhost:9821/api/v1/runtime-bb-keys/list -d '{"page":1,"page_size":10}'

# R5: 命名冲突（跨 field）
curl -X POST http://localhost:9821/api/v1/fields -d '{"name":"conflict_name",...}'
curl -X POST http://localhost:9821/api/v1/runtime-bb-keys -d '{"name":"conflict_name",...}'   # 预期 409 + 47004

# R7: FSM 引用同步
# 创建 FSM 含 condition.key="threat_level" → 检 runtime_bb_key_refs
curl -X POST http://localhost:9821/api/v1/fsm-configs -d '{"name":"...","config":{"transitions":[{"condition":{"key":"threat_level","op":">","value":50}}]}}'
mysql> SELECT * FROM runtime_bb_key_refs WHERE ref_type='fsm';   # 预期 1 行

# R13: 停用 key 后尝试新建引用
curl -X POST http://localhost:9821/api/v1/runtime-bb-keys/1/toggle -d '{"enabled":false}'
curl -X POST http://localhost:9821/api/v1/fsm-configs -d '{"...condition.key":"threat_level"...}'  # 预期 400
```

**seed 冷启断言**（加到 [`scripts/verify-seed.sh`](../../scripts/verify-seed.sh) Step 3）：

```bash
RUNTIME_KEY_COUNT=$(mysql -e "SELECT COUNT(*) FROM runtime_bb_keys WHERE deleted=0" | tail -1)
[ "$RUNTIME_KEY_COUNT" = "31" ] || { echo "✗ runtime_bb_keys 应为 31 条，实际 $RUNTIME_KEY_COUNT"; exit 1; }
echo "[✓] runtime_bb_keys 表含 31 条对齐服务端 keys.go"
```

---

## 9. 经验沉淀候选

以下候选在本 spec 完成后考虑沉淀到 [`docs/development/standards/`](../../development/standards/) 或 memory：

1. **"三路 BB Key 数据源"模式**：字段 expose_bb / 事件扩展字段 / 运行时注册表；前端 `BBKeySelector` 的三 Promise 加载范式可作为"同类选择器的标准模式"写入 `dev-rules/frontend.md`
2. **跨模块引用同步的"平行 sync + handler 编排"**：两个独立 service 各管各的 ref 表、handler 里串接 → 可沉淀为"添加第 N 种引用源"的设计模板
3. **Go 类型 → ADMIN type 规范化表**：float64→float / int64→integer / string→string / bool→bool；未来新增来源（如组件 schema）沿用；可写入 `api-contract.md`
4. **seed 漂移监控**（挂 Phase 3 tasks）：seed 与服务端 `keys.go` 的逐条对齐监控方案
