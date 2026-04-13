# 状态机管理 — 后端设计

> 通用架构/规范见 `docs/architecture/overview.md` 和 `docs/development/`。
> 本文档只记录状态机管理模块的实现事实与特有约束。

---

## 1. 目录结构

```
backend/
  internal/
    handler/
      fsm_config.go                    # 状态机 CRUD + 列表 + 详情 + toggle + check-name + 跨模块事务编排
    service/
      fsm_config.go                    # 状态机业务逻辑（含配置完整性校验、条件树递归校验、config_json 拼装、BB Key 提取、事务版方法）
      field.go                         # SyncFsmBBKeyRefs / CleanFsmBBKeyRefs / InvalidateDetails（FSM BB Key 引用维护）
    store/
      mysql/
        fsm_config.go                  # fsm_configs 表 CRUD（含事务版 CreateTx/UpdateTx/SoftDeleteTx）
      redis/
        fsm_config_cache.go            # 状态机 Redis 缓存（detail + list + 分布式锁）
        config/keys.go                 # key 前缀 & 构造函数（fsm_configs:detail/list/lock）
    model/
      fsm_config.go                    # FsmConfig / FsmConfigListItem / FsmConfigListData / FsmConfigDetail / FsmConfigExportItem / 请求体 / FsmState / FsmTransition / FsmCondition
    errcode/
      codes.go                         # 43001-43012 错误码
    router/
      router.go                        # /api/v1/fsm-configs/* + /api/configs/fsm_configs
    util/
      const.go                         # RefTypeFsm = "fsm"（field_refs ref_type 常量）
  migrations/
    006_create_fsm_configs.sql
```

---

## 2. 数据表

### fsm_configs

```sql
CREATE TABLE IF NOT EXISTS fsm_configs (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    name            VARCHAR(64)  NOT NULL,              -- FSM 唯一标识（如 wolf_fsm），创建后不可变
    display_name    VARCHAR(128) NOT NULL,              -- 中文名（搜索用）
    config_json     JSON         NOT NULL,              -- {initial_state, states, transitions} 完整配置，导出 API 原样输出

    enabled         TINYINT(1)   NOT NULL DEFAULT 0,    -- 启用状态（创建默认 0，给"配置窗口期"）
    version         INT          NOT NULL DEFAULT 1,    -- 乐观锁
    created_at      DATETIME     NOT NULL,
    updated_at      DATETIME     NOT NULL,
    deleted         TINYINT(1)   NOT NULL DEFAULT 0,    -- 软删除

    -- name 唯一约束不含 deleted：软删后 name 仍占唯一性，不可复用
    UNIQUE KEY uk_name (name),
    -- 覆盖索引：列表分页查询（id DESC 排序，含 enabled 用于筛选）
    INDEX idx_list (deleted, enabled, id DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**设计要点：**
- `config_json` 是 `{initial_state, states, transitions}` 的完整 JSON，导出 API 直接原样输出，不经过 Go struct 中转。Service 层创建/编辑时用 `buildConfigJSON()` 组装，列表时 unmarshal 抽展示字段。
- `uk_name` 不含 `deleted` 列：软删后 name 永久不可复用。`ExistsByName` 查询不带 `deleted` 过滤。
- `enabled` 默认 0：创建后给"配置窗口期"，编辑/删除要求先停用。
- `idx_list (deleted, enabled, id DESC)`：列表分页覆盖索引，支持 enabled 筛选 + id 倒序分页。

### field_refs（关联表，由 FieldService 维护）

FSM 条件中引用的 BB Key 通过 `field_refs` 表追踪反向引用关系：

| 字段 | 说明 |
|------|------|
| `field_id` | 被引用的字段 ID |
| `ref_type` | `"fsm"`（`util.RefTypeFsm` 常量） |
| `source_id` | FSM 配置的 ID |

---

## 3. API 接口

### 状态机管理（7 个接口）

| 方法 | 路径 | Handler | 说明 |
|------|------|---------|------|
| POST | `/api/v1/fsm-configs/list` | `FsmConfigHandler.List` | 分页列表，支持 label 模糊搜索 + enabled 筛选 |
| POST | `/api/v1/fsm-configs/create` | `FsmConfigHandler.Create` | 创建状态机 + BB Key 引用追踪（跨模块事务） |
| POST | `/api/v1/fsm-configs/detail` | `FsmConfigHandler.Get` | 详情，返回 config_json 展开 |
| POST | `/api/v1/fsm-configs/update` | `FsmConfigHandler.Update` | 编辑（必须先停用）+ BB Key 引用 diff（跨模块事务） |
| POST | `/api/v1/fsm-configs/delete` | `FsmConfigHandler.Delete` | 软删除（必须先停用）+ BB Key 引用清理（跨模块事务） |
| POST | `/api/v1/fsm-configs/check-name` | `FsmConfigHandler.CheckName` | name 完整格式校验（正则+长度）+ 唯一性校验 |
| POST | `/api/v1/fsm-configs/toggle-enabled` | `FsmConfigHandler.ToggleEnabled` | 启用/停用切换（调用方指定目标状态 `enabled`），乐观锁。返回 `"操作成功"` |

### 导出接口

| 方法 | 路径 | Handler | 说明 |
|------|------|---------|------|
| GET | `/api/configs/fsm_configs` | `ExportHandler.FsmConfigs` | 导出所有已启用状态机，`{items: [{name, config}]}` |

导出查询：`SELECT name, config_json AS config FROM fsm_configs WHERE deleted = 0 AND enabled = 1 ORDER BY id`，config_json 原样输出。

---

## 4. Handler 层

### 结构体

```go
type FsmConfigHandler struct {
    db               *sqlx.DB               // 跨模块事务用
    fsmConfigService *service.FsmConfigService
    fieldService     *service.FieldService   // BB Key 引用维护
    fsmCfg           *config.FsmConfigConfig
}

func NewFsmConfigHandler(
    db *sqlx.DB,
    fsmConfigService *service.FsmConfigService,
    fieldService *service.FieldService,
    fsmCfg *config.FsmConfigConfig,
) *FsmConfigHandler
```

Handler 直接持有 `*sqlx.DB` 和 `*service.FieldService`，用于编排跨模块事务。这是与 Field/Template/EventType 等单模块 handler 的关键区别——单模块 handler 不持有 `db`，事务由 service 内部管理；FSM handler 因需跨模块协调（fsm_configs + field_refs），由 handler 开事务统一协调。

### 前置校验

- `checkName(name)`：非空 + `util.IdentPattern`（`^[a-z][a-z0-9_]*$`）+ 长度 <= NameMaxLength
- `checkDisplayName(displayName)`：非空 + 字符数 <= DisplayNameMaxLength（`utf8.RuneCountInString`）
- 统一使用共享 `util.CheckID()` / `util.CheckVersion()`
- slog Debug 日志在校验之后打印，格式为中文点分（如 `"handler.创建状态机"`）

### 跨模块事务编排

**Create 流程：**

```
handler.Create
  ├─ checkName + checkDisplayName（格式校验）
  ├─ db.BeginTxx(ctx, nil)
  │   defer tx.Rollback()
  ├─ fsmConfigService.CreateInTx(ctx, tx, req) → (id, configJSON, err)
  ├─ service.ExtractBBKeys(req.Transitions) → newKeys
  ├─ fieldService.SyncFsmBBKeyRefs(ctx, tx, id, emptyKeys, newKeys) → affected
  ├─ tx.Commit()
  ├─ fsmConfigService.InvalidateList(ctx)
  └─ fieldService.InvalidateDetails(ctx, affected)
```

**Update 流程：**

```
handler.Update
  ├─ CheckID + CheckVersion + checkDisplayName
  ├─ db.BeginTxx(ctx, nil)
  │   defer tx.Rollback()
  ├─ fsmConfigService.UpdateInTx(ctx, tx, req) → oldFc（旧 FsmConfig）
  ├─ service.ExtractBBKeysFromConfigJSON(oldFc.ConfigJSON) → oldKeys
  ├─ service.ExtractBBKeys(req.Transitions) → newKeys
  ├─ fieldService.SyncFsmBBKeyRefs(ctx, tx, req.ID, oldKeys, newKeys) → affected
  ├─ tx.Commit()
  ├─ fsmConfigService.InvalidateDetail(ctx, req.ID)
  ├─ fsmConfigService.InvalidateList(ctx)
  └─ fieldService.InvalidateDetails(ctx, affected)
```

**Delete 流程：**

```
handler.Delete
  ├─ CheckID
  ├─ db.BeginTxx(ctx, nil)
  │   defer tx.Rollback()
  ├─ fsmConfigService.SoftDeleteInTx(ctx, tx, id) → fc（旧 FsmConfig）
  ├─ fieldService.CleanFsmBBKeyRefs(ctx, tx, id) → affected
  ├─ tx.Commit()
  ├─ fsmConfigService.InvalidateDetail(ctx, id)
  ├─ fsmConfigService.InvalidateList(ctx)
  └─ fieldService.InvalidateDetails(ctx, affected)
```

**关键设计**：`defer tx.Rollback()` 保证任何 panic/error 路径自动回滚。缓存失效全部在 commit 之后执行，避免"缓存已清但事务回滚"的不一致窗口。

---

## 5. Service 层

### 结构体

```go
type FsmConfigService struct {
    store  *storemysql.FsmConfigStore
    cache  *storeredis.FsmConfigCache
    pagCfg *config.PaginationConfig
    fsmCfg *config.FsmConfigConfig
}

func NewFsmConfigService(
    store *storemysql.FsmConfigStore,
    cache *storeredis.FsmConfigCache,
    pagCfg *config.PaginationConfig,
    fsmCfg *config.FsmConfigConfig,
) *FsmConfigService
```

Service 只持有自身的 store/cache，不持有其他模块的 store/service（分层职责硬规则）。

### 标准方法（独立使用，内含缓存管理）

| 方法签名 | 说明 |
|----------|------|
| `List(ctx, *model.FsmConfigListQuery) (*model.ListData, error)` | 分页列表：校正分页 → Redis 缓存 → miss → MySQL → unmarshal config_json 抽 initial_state/state_count → 写缓存 |
| `Create(ctx, *model.CreateFsmConfigRequest) (int64, error)` | 创建：name 唯一性 → validateConfig → buildConfigJSON → store.Create → 清列表缓存 |
| `GetByID(ctx, id) (*model.FsmConfig, error)` | 详情：Cache-Aside + 分布式锁防击穿 + 空标记防穿透 |
| `Update(ctx, *model.UpdateFsmConfigRequest) error` | 编辑：存在性 → 必须已停用 → validateConfig → buildConfigJSON → 乐观锁更新 → 清缓存 |
| `Delete(ctx, id) (*model.DeleteResult, error)` | 软删除：存在性 → 必须已停用 → 软删 → 清缓存 |
| `CheckName(ctx, name) (*model.CheckNameResult, error)` | 唯一性校验 |
| `ToggleEnabled(ctx, *model.ToggleEnabledRequest) error` | 启用/停用切换：幂等安全 → 乐观锁更新 → 清缓存 |
| `ExportAll(ctx) ([]model.FsmConfigExportItem, error)` | 导出已启用配置 |

### 事务版方法（handler 跨模块编排用，不清缓存）

| 方法签名 | 说明 |
|----------|------|
| `CreateInTx(ctx, tx *sqlx.Tx, req *model.CreateFsmConfigRequest) (int64, json.RawMessage, error)` | name 唯一性 → validateConfig → buildConfigJSON → store.CreateTx。返回 (id, configJSON)，不清缓存 |
| `UpdateInTx(ctx, tx *sqlx.Tx, req *model.UpdateFsmConfigRequest) (*model.FsmConfig, error)` | 存在性 → 必须已停用 → validateConfig → buildConfigJSON → store.UpdateTx 乐观锁。返回旧 FsmConfig（handler 用于提取旧 BB Keys diff），不清缓存 |
| `SoftDeleteInTx(ctx, tx *sqlx.Tx, id int64) (*model.FsmConfig, error)` | 存在性 → 必须已停用 → store.SoftDeleteTx。返回旧 FsmConfig，不清缓存。TODO: NPC 管理上线后加引用检查 |

### 缓存失效方法（handler commit 后调用）

| 方法签名 | 说明 |
|----------|------|
| `InvalidateDetail(ctx, id int64)` | `cache.DelDetail(ctx, id)` |
| `InvalidateList(ctx)` | `cache.InvalidateList(ctx)` |

### BB Key 提取（包级函数）

```go
// ExtractBBKeys 从 transitions 中提取 BB Key name 集合
func ExtractBBKeys(transitions []model.FsmTransition) map[string]bool

// ExtractBBKeysFromConfigJSON 从 config_json 中提取 BB Key name 集合
func ExtractBBKeysFromConfigJSON(configJSON json.RawMessage) map[string]bool
```

两个函数都是包级函数（非方法），handler 直接用 `service.ExtractBBKeys(...)` 调用。

内部调用 `collectConditionKeys` 递归遍历条件树：

```go
func collectConditionKeys(cond *model.FsmCondition, keys map[string]bool) {
    if cond.IsEmpty() { return }
    if cond.Key != "" { keys[cond.Key] = true }
    if cond.RefKey != "" { keys[cond.RefKey] = true }
    for i := range cond.And { collectConditionKeys(&cond.And[i], keys) }
    for i := range cond.Or  { collectConditionKeys(&cond.Or[i], keys) }
}
```

**算法说明**：递归遍历条件树的所有节点，收集叶节点的 `Key`（BB Key 标识）和 `RefKey`（引用另一个 BB Key）。空条件节点直接跳过。最终返回去重的 BB Key name 集合。

`ExtractBBKeysFromConfigJSON` 先将 `json.RawMessage` unmarshal 到只含 `transitions` 的临时结构体，再调用 `ExtractBBKeys`。unmarshal 失败返回空 map（容错）。

### 配置完整性校验

`validateConfig(initialState, states, transitions)` 按以下顺序校验：

1. states 不能为空（43004）
2. 状态数不超上限 `MaxStates`（43004）
3. 状态名非空且不重复（43005）
4. initial_state 必须在 states 中（43006）
5. 转换数不超上限 `MaxTransitions`（43007）
6. 每条转换的 from/to 在 states 中 + priority >= 0（43007）
7. 每条转换的 condition 递归校验（43008）

`validateCondition(cond, depth, maxDepth)` 递归校验条件树节点。

### 条件操作符白名单

```go
var validConditionOps = map[string]bool{
    "==": true, "!=": true,
    ">": true, ">=": true,
    "<": true, "<=": true,
    "in": true,
}
```

对齐游戏服务端 `rule.validOps`。

---

## 6. Store 层

### 结构体

```go
type FsmConfigStore struct {
    db *sqlx.DB
}

func NewFsmConfigStore(db *sqlx.DB) *FsmConfigStore
```

### 标准方法

| 方法签名 | SQL |
|----------|-----|
| `Create(ctx, req, configJSON) (int64, error)` | `INSERT INTO fsm_configs (name, display_name, config_json, enabled, version, created_at, updated_at, deleted) VALUES (?, ?, ?, 0, 1, ?, ?, 0)` |
| `GetByID(ctx, id) (*model.FsmConfig, error)` | `SELECT ... FROM fsm_configs WHERE id = ? AND deleted = 0` |
| `ExistsByName(ctx, name) (bool, error)` | `SELECT COUNT(*) FROM fsm_configs WHERE name = ?`（不过滤 deleted） |
| `List(ctx, q) ([]model.FsmConfig, int64, error)` | 动态 WHERE（deleted=0 + label LIKE + enabled 筛选）+ COUNT + 分页 SELECT |
| `Update(ctx, req, configJSON) error` | `UPDATE ... SET display_name=?, config_json=?, version=version+1, updated_at=? WHERE id=? AND version=? AND deleted=0`。rows=0 → `errcode.ErrVersionConflict` |
| `SoftDelete(ctx, id) error` | `UPDATE ... SET deleted=1, updated_at=? WHERE id=? AND deleted=0`。rows=0 → `errcode.ErrNotFound` |
| `ToggleEnabled(ctx, id, enabled, version) error` | `UPDATE ... SET enabled=?, version=version+1, updated_at=? WHERE id=? AND version=? AND deleted=0`。rows=0 → `errcode.ErrVersionConflict` |
| `ExportAll(ctx) ([]model.FsmConfigExportItem, error)` | `SELECT name, config_json AS config FROM fsm_configs WHERE deleted=0 AND enabled=1 ORDER BY id` |

### 事务版方法（handler 跨模块编排用）

| 方法签名 | 说明 |
|----------|------|
| `CreateTx(ctx, tx *sqlx.Tx, req, configJSON) (int64, error)` | 同 Create，但在 tx 上执行 |
| `UpdateTx(ctx, tx *sqlx.Tx, req, configJSON) error` | 同 Update，但在 tx 上执行。rows=0 → `errcode.ErrVersionConflict` |
| `SoftDeleteTx(ctx, tx *sqlx.Tx, id) error` | 同 SoftDelete，但在 tx 上执行。rows=0 → `errcode.ErrNotFound` |

事务版方法的 SQL 与标准方法完全相同，唯一区别是在 `*sqlx.Tx` 上执行而非 `*sqlx.DB`。

### DB() 方法

```go
func (s *FsmConfigStore) DB() *sqlx.DB
```

暴露数据库连接，handler 层可用于开跨模块事务。但当前实现中 handler 直接持有 `*sqlx.DB`，此方法保留为兼容用途。

---

## 7. BB Key 引用维护（FieldService 侧）

### SyncFsmBBKeyRefs

```go
func (s *FieldService) SyncFsmBBKeyRefs(
    ctx context.Context,
    tx *sqlx.Tx,
    fsmID int64,
    oldKeys, newKeys map[string]bool,
) ([]int64, error)
```

**算法**：
1. 计算 diff：`toAdd = newKeys - oldKeys`，`toRemove = oldKeys - newKeys`
2. 若 diff 为空直接返回 `nil, nil`
3. 合并所有涉及的 BB Key name，批量查字段表 `fieldStore.GetByNames` → `nameToID` map
4. 遍历 toAdd：若 name 在 `nameToID` 中（即来自字段表），`fieldRefStore.Add(ctx, tx, fieldID, "fsm", fsmID)`；不在则跳过（运行时 Key）
5. 遍历 toRemove：同理，`fieldRefStore.Remove(ctx, tx, fieldID, "fsm", fsmID)`
6. 返回所有受影响的 field IDs（用于清缓存）

### CleanFsmBBKeyRefs

```go
func (s *FieldService) CleanFsmBBKeyRefs(
    ctx context.Context,
    tx *sqlx.Tx,
    fsmID int64,
) ([]int64, error)
```

删除时调用，`fieldRefStore.RemoveBySource(ctx, tx, "fsm", fsmID)` 一次性清除该 FSM 的所有引用。返回被引用的 field IDs。

### InvalidateDetails

```go
func (s *FieldService) InvalidateDetails(ctx context.Context, fieldIDs []int64)
```

批量清字段详情缓存。缓存清理失败仅 `slog.Error`，不阻塞业务。

---

## 8. 缓存策略

### fsm_configs 缓存（Redis）

| Key 模式 | 含义 | TTL |
|----------|------|-----|
| `fsm_configs:detail:{id}` | 单条详情（含空标记防穿透） | 5min + 30s jitter |
| `fsm_configs:list:v{ver}:{label}:{enabled}:{page}:{pageSize}` | 列表分页缓存（带版本号） | 1min + 10s jitter |
| `fsm_configs:list:version` | 列表缓存版本号（INCR 使旧 key 自然过期） | 永久 |
| `fsm_configs:lock:{id}` | 分布式锁（SETNX 防缓存击穿） | 3s（可配置） |

**失效规则：**

| 操作 | 失效动作 | 执行时机 |
|------|----------|----------|
| Create | `InvalidateList` + `fieldService.InvalidateDetails(affected)` | handler commit 后 |
| Update | `InvalidateDetail(id)` + `InvalidateList` + `fieldService.InvalidateDetails(affected)` | handler commit 后 |
| Delete | `InvalidateDetail(id)` + `InvalidateList` + `fieldService.InvalidateDetails(affected)` | handler commit 后 |
| ToggleEnabled | `DelDetail(id)` + `InvalidateList` | service 内部 |
| List（标准方法，非事务） | `InvalidateList` | service 内部 Create |

列表失效采用版本号递增方式（`INCR fsm_configs:list:version`），禁止 SCAN+DEL。

**详情读取流程（Cache-Aside + 分布式锁 + 空标记，与 Field/Template/EventType 完全一致）：**
1. 查 Redis 缓存：`err == nil && hit` 才使用缓存结果（Redis 错误降级直查 MySQL）
2. 未命中 → SETNX 获取分布式锁（锁失败 `slog.Warn` 记录后继续）→ double-check 缓存
3. 查 MySQL → 写缓存（nil 写空标记防穿透）

---

## 9. 错误码

### 状态机管理（43001-43012）

| 错误码 | 常量 | 默认消息 | 触发场景 |
|--------|------|----------|----------|
| 43001 | `ErrFsmConfigNameExists` | 状态机标识已存在 | 创建时 name 已存在（含软删除） |
| 43002 | `ErrFsmConfigNameInvalid` | 状态机标识格式不合法，需小写字母开头，仅允许 a-z、0-9、下划线 | name 为空 / 不匹配正则 / 超长 |
| 43003 | `ErrFsmConfigNotFound` | 状态机配置不存在 | ID 对应记录不存在或已软删 |
| 43004 | `ErrFsmConfigStatesEmpty` | 请至少定义一个状态 | 未定义任何状态 / 状态数超上限 |
| 43005 | `ErrFsmConfigStateNameInvalid` | 状态名不能为空且不能重复 | 状态名为空或重复 |
| 43006 | `ErrFsmConfigInitialInvalid` | 初始状态必须是已定义的状态之一 | 初始状态不在状态列表中 |
| 43007 | `ErrFsmConfigTransitionInvalid` | 转换规则引用了不存在的状态 | from/to 不在 states 中 / priority < 0 / 转换数超上限 |
| 43008 | `ErrFsmConfigConditionInvalid` | 条件表达式不合法 | 嵌套超深 / 叶组合混用 / and+or 共存 / 非法操作符 / value+ref_key 冲突或同时为空 |
| 43009 | `ErrFsmConfigDeleteNotDisabled` | 请先停用该状态机再删除 | 删除前必须先停用 |
| 43010 | `ErrFsmConfigEditNotDisabled` | 请先停用该状态机再编辑 | 编辑前必须先停用 |
| 43011 | `ErrFsmConfigVersionConflict` | 该状态机已被其他人修改，请刷新后重试 | 版本冲突（乐观锁，编辑 / toggle） |
| 43012 | `ErrFsmConfigRefDelete` | 当前状态机仍被引用，不能删除 | 被 NPC 引用，无法删除（占位，本期 ref_count 恒 0） |

---

## 10. 配置项

`config.yaml` 中 `fsm_config` 段：

```yaml
fsm_config:
  name_max_length: 64            # name 最大字符长度
  display_name_max_length: 128   # display_name 最大字符长度（按 rune 计）
  max_states: 50                 # 单个 FSM 最大状态数
  max_transitions: 200           # 单个 FSM 最大转换规则数
  condition_max_depth: 10        # 条件树最大嵌套深度
  cache_detail_ttl: 10m          # 详情缓存 TTL（base，实际加 jitter）
  cache_list_ttl: 5m             # 列表缓存 TTL（base，实际加 jitter）
  cache_lock_ttl: 3s             # 分布式锁 TTL
```

Go 结构体 `config.FsmConfigConfig`：

```go
type FsmConfigConfig struct {
    NameMaxLength        int           `yaml:"name_max_length"`
    DisplayNameMaxLength int           `yaml:"display_name_max_length"`
    MaxStates            int           `yaml:"max_states"`
    MaxTransitions       int           `yaml:"max_transitions"`
    ConditionMaxDepth    int           `yaml:"condition_max_depth"`
    CacheDetailTTL       time.Duration `yaml:"cache_detail_ttl"`
    CacheListTTL         time.Duration `yaml:"cache_list_ttl"`
    CacheLockTTL         time.Duration `yaml:"cache_lock_ttl"`
}
```

各层使用：
- Handler 层：`NameMaxLength` / `DisplayNameMaxLength` 做格式校验
- Service 层：`MaxStates` / `MaxTransitions` / `ConditionMaxDepth` 做业务上限校验；`CacheLockTTL` 做分布式锁超时
- Cache 层：`CacheDetailTTL` / `CacheListTTL` 做缓存 TTL
