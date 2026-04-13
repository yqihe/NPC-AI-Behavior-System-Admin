# 状态机管理 — 设计方案（后端）

> 对应需求：[requirements.md](./requirements.md)

---

## 1. 方案描述

### 1.1 数据模型

#### MySQL 表：`fsm_configs`

```sql
CREATE TABLE IF NOT EXISTS fsm_configs (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    name            VARCHAR(64)  NOT NULL,              -- FSM 唯一标识（如 wolf_fsm），创建后不可变
    display_name    VARCHAR(128) NOT NULL,              -- 中文名（列表搜索用）
    config_json     JSON         NOT NULL,              -- {initial_state, states, transitions} 完整配置

    enabled         TINYINT(1)   NOT NULL DEFAULT 0,    -- 启用状态（创建默认 0）
    version         INT          NOT NULL DEFAULT 1,    -- 乐观锁
    created_at      DATETIME     NOT NULL,
    updated_at      DATETIME     NOT NULL,
    deleted         TINYINT(1)   NOT NULL DEFAULT 0,    -- 软删除

    UNIQUE KEY uk_name (name),
    INDEX idx_list (deleted, enabled, id DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**设计决策**：
- 无 `perception_mode` 类 facet 列，FSM 没有分类维度，索引比事件类型更简单
- `config_json` 存完整配置，导出 API 原样输出，不拆多张表
- 搜索维度：`display_name`（模糊）+ `enabled`（精确），与事件类型模式对齐

#### config_json 内部结构（对齐 API 契约 §5）

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
    }
  ]
}
```

#### Go 数据结构 (`model/fsm_config.go`)

```go
// FsmConfig DB 行结构体
type FsmConfig struct {
    ID          int64           `json:"id" db:"id"`
    Name        string          `json:"name" db:"name"`
    DisplayName string          `json:"display_name" db:"display_name"`
    ConfigJSON  json.RawMessage `json:"config_json" db:"config_json"`
    Enabled     bool            `json:"enabled" db:"enabled"`
    Version     int             `json:"version" db:"version"`
    CreatedAt   time.Time       `json:"created_at" db:"created_at"`
    UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
    Deleted     bool            `json:"-" db:"deleted"`
}

// FsmConfigListItem 列表页展示项
type FsmConfigListItem struct {
    ID           int64     `json:"id" db:"id"`
    Name         string    `json:"name" db:"name"`
    DisplayName  string    `json:"display_name" db:"display_name"`
    Enabled      bool      `json:"enabled" db:"enabled"`
    CreatedAt    time.Time `json:"created_at" db:"created_at"`
    // 以下字段从 config_json unmarshal 后由 service 层填充
    InitialState string `json:"initial_state" db:"-"`
    StateCount   int    `json:"state_count" db:"-"`
}

// FsmConfigListData 类型安全的列表缓存结构
type FsmConfigListData struct {
    Items    []FsmConfigListItem `json:"items"`
    Total    int64               `json:"total"`
    Page     int                 `json:"page"`
    PageSize int                 `json:"page_size"`
}

// FsmConfigDetail 详情接口响应
type FsmConfigDetail struct {
    ID          int64                  `json:"id"`
    Name        string                 `json:"name"`
    DisplayName string                 `json:"display_name"`
    Enabled     bool                   `json:"enabled"`
    Version     int                    `json:"version"`
    CreatedAt   time.Time              `json:"created_at"`
    UpdatedAt   time.Time              `json:"updated_at"`
    Config      map[string]interface{} `json:"config"` // config_json 展开
}

// FsmConfigExportItem 导出 API 单条
type FsmConfigExportItem struct {
    Name   string          `json:"name"`
    Config json.RawMessage `json:"config"`
}

// ---- 请求结构 ----

// FsmConfigListQuery 列表查询参数
type FsmConfigListQuery struct {
    Label    string `json:"label"`              // display_name 模糊搜索
    Enabled  *bool  `json:"enabled,omitempty"`  // nil=不筛选
    Page     int    `json:"page"`
    PageSize int    `json:"page_size"`
}

// CreateFsmConfigRequest 创建请求
type CreateFsmConfigRequest struct {
    Name         string          `json:"name"`
    DisplayName  string          `json:"display_name"`
    InitialState string          `json:"initial_state"`
    States       []FsmState      `json:"states"`
    Transitions  []FsmTransition `json:"transitions"`
}

// UpdateFsmConfigRequest 编辑请求
type UpdateFsmConfigRequest struct {
    ID           int64           `json:"id"`
    DisplayName  string          `json:"display_name"`
    InitialState string          `json:"initial_state"`
    States       []FsmState      `json:"states"`
    Transitions  []FsmTransition `json:"transitions"`
    Version      int             `json:"version"`
}

// FsmState 状态定义
type FsmState struct {
    Name string `json:"name"`
}

// FsmTransition 转换规则
type FsmTransition struct {
    From      string       `json:"from"`
    To        string       `json:"to"`
    Priority  int          `json:"priority"`
    Condition FsmCondition `json:"condition"`
}

// FsmCondition 条件树（对齐游戏服务端 rule.Condition）
type FsmCondition struct {
    Key    string          `json:"key,omitempty"`
    Op     string          `json:"op,omitempty"`
    Value  json.RawMessage `json:"value,omitempty"`
    RefKey string          `json:"ref_key,omitempty"`
    And    []FsmCondition  `json:"and,omitempty"`
    Or     []FsmCondition  `json:"or,omitempty"`
}
```

### 1.2 接口定义

#### CRUD 接口（7 个）

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/fsm-configs/list` | 分页列表 |
| POST | `/api/v1/fsm-configs/create` | 创建 |
| POST | `/api/v1/fsm-configs/detail` | 详情 |
| POST | `/api/v1/fsm-configs/update` | 编辑（全量替换 config_json） |
| POST | `/api/v1/fsm-configs/delete` | 删除（软删除） |
| POST | `/api/v1/fsm-configs/check-name` | 名称唯一性检查 |
| POST | `/api/v1/fsm-configs/toggle-enabled` | 启用/停用 |

#### 导出接口

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/configs/fsm_configs` | 导出所有已启用 FSM 给游戏服务端 |

### 1.3 分层职责

```
handler/fsm_config.go
  ├─ 参数绑定 + 前置校验（name 格式、displayName 非空、ID/version 有效）
  ├─ 调 service 层
  └─ 统一 WrapCtx 响应

service/fsm_config.go
  ├─ 业务逻辑（启用拦截、乐观锁、config_json 校验与组装）
  ├─ 条件树递归校验（validateCondition）
  ├─ 配置完整性校验（states 非空/不重复、initial_state ∈ states、from/to ∈ states）
  ├─ 调 store 层读写 MySQL
  └─ 调 cache 层管理 Redis

store/mysql/fsm_config.go
  ├─ fsm_configs 表 CRUD
  └─ ExportAll（enabled=1 AND deleted=0）

store/redis/fsm_config_cache.go
  ├─ 详情 Cache-Aside + 分布式锁 + 空标记
  └─ 列表版本号
```

### 1.4 条件树校验逻辑

Service 层 `validateCondition(cond *FsmCondition, depth int) error`：

1. **空条件**（key="" 且无 and/or）→ 合法（无条件转换，始终 true）
2. **叶/组合互斥**：key 非空时不允许有 and/or；有 and/or 时不允许有 key
3. **叶节点校验**：
   - `op` 必须在 `== != > >= < <= in` 中
   - `value` 和 `ref_key` 不能同时为空（除非 op 允许）
   - `value` 和 `ref_key` 不能同时非空
4. **组合节点校验**：递归校验子条件
5. **深度限制**：depth > 10 报错，防无限嵌套

此逻辑对齐游戏服务端 `rule.Condition.Validate()`，但**不校验 key 是否在 BB Key 注册表中**（那是服务端加载时的职责）。

### 1.5 config_json 组装

Service 层 `buildConfigJSON`：

```go
func (s *FsmConfigService) buildConfigJSON(req *CreateFsmConfigRequest) (json.RawMessage, error) {
    config := map[string]interface{}{
        "initial_state": req.InitialState,
        "states":        req.States,
        "transitions":   req.Transitions,
    }
    return json.Marshal(config)
}
```

Handler 层接收结构化的 `states` / `transitions`，Service 层校验后组装为 `config_json`。导出时原样输出 `config_json` 列，不经 Go struct 中转。

### 1.6 缓存策略

完全复用已有模式（与 EventTypeCache 同构）：

| 维度 | 方案 |
|------|------|
| 列表缓存 | 版本号方案：key = `fsm_configs:list:v{version}:{label}:{enabled}:{page}:{pageSize}`，写操作 INCR 版本号 |
| 详情缓存 | Cache-Aside：key = `fsm_configs:detail:{id}`，分布式锁防击穿 |
| 空标记 | 不存在的 ID 缓存 `{"_null":true}`，短 TTL |
| TTL | base + jitter 防雪崩 |
| 降级 | Redis 不可用时直查 MySQL |
| 失效 | Create/Update/Delete/ToggleEnabled → INCR 版本号 + DEL 详情 key |

### 1.7 错误码

```go
// --- 状态机管理 430xx ---
const (
    ErrFsmConfigNameExists        = 43001 // FSM 标识已存在（含软删除）
    ErrFsmConfigNameInvalid       = 43002 // FSM 标识格式不合法
    ErrFsmConfigNotFound          = 43003 // FSM 配置不存在
    ErrFsmConfigStatesEmpty       = 43004 // 未定义任何状态
    ErrFsmConfigStateNameInvalid  = 43005 // 状态名为空或重复
    ErrFsmConfigInitialInvalid    = 43006 // 初始状态不在状态列表中
    ErrFsmConfigTransitionInvalid = 43007 // 转换规则引用了不存在的状态
    ErrFsmConfigConditionInvalid  = 43008 // 条件表达式不合法
    ErrFsmConfigDeleteNotDisabled = 43009 // 删除前必须先停用
    ErrFsmConfigEditNotDisabled   = 43010 // 编辑前必须先停用
    ErrFsmConfigVersionConflict   = 43011 // 版本冲突（乐观锁）
    ErrFsmConfigRefDelete         = 43012 // 被 NPC 引用，无法删除（占位，本期 ref_count 恒 0）
)
```

### 1.8 配置项

```yaml
# config.yaml 新增
fsm_config:
  name_max_length: 64
  display_name_max_length: 128
  max_states: 50                     # 单个 FSM 最大状态数
  max_transitions: 200               # 单个 FSM 最大转换规则数
  condition_max_depth: 10            # 条件树最大嵌套深度
  cache_detail_ttl: 300s
  cache_list_ttl: 300s
  cache_lock_ttl: 5s
```

---

## 2. 方案对比

### 方案 A（采用）：MySQL 单表 + config_json

- `config_json` 存完整 `{initial_state, states, transitions}`
- 列表搜索列（name、display_name、enabled）拎出来做独立列
- 导出直接输出 `config_json`

### 方案 B（不采用）：MySQL 多表（fsm_configs + fsm_states + fsm_transitions）

- 把 states 和 transitions 拆成独立表，外键关联 fsm_configs
- **不选原因**：
  1. 违背已有模式——事件类型用 `config_json` 一列存完整配置，FSM 拆多表会导致模式不一致
  2. 导出 API 需要 JOIN 三张表重组 JSON，复杂度高
  3. 游戏服务端消费的就是一个 JSON 对象，拆开存再拼回去是无意义的循环
  4. states/transitions 不会被其他表引用（NPC 引用的是 FSM name，不是单个状态 ID）
  5. 增加事务复杂度：创建/编辑需要跨表事务

---

## 3. 红线检查

### 通用红线 (`general.md`)

| 条目 | 检查结果 |
|------|---------|
| 禁止静默降级 | ✅ 条件校验失败明确报错，不 fallback |
| 禁止安全隐患 | ✅ LIKE 转义、参数化查询、不暴露内部错误 |
| 禁止测试质量低下 | ✅ 测试策略见 §8 |
| 禁止过度设计 | ✅ 不引入无用依赖，不为 FSM 建 Schema 管理 |
| 禁止协作失序 | ✅ spec 先行 |

### Go 红线 (`go.md`)

| 条目 | 检查结果 |
|------|---------|
| nil slice → null | ✅ `make([]T, 0)` 初始化 |
| config_json 用 RawMessage | ✅ `json.RawMessage` 透传 |
| 错误码语义不混用 | ✅ 每个校验场景独立错误码 |
| 缓存反序列化类型安全 | ✅ `FsmConfigListData` 类型安全结构体 |
| 分层不倒置 | ✅ store 不依赖 cache |
| slog 不暴露 Go error | ✅ 500 返回中文提示 |

### MySQL 红线 (`mysql.md`)

| 条目 | 检查结果 |
|------|---------|
| 事务内用 tx 不用 db | ✅ 本期无跨模块事务，单表操作不需要显式事务 |
| LIKE 转义 | ✅ `escapeLike()` |

### Redis 红线 (`redis.md`)

| 条目 | 检查结果 |
|------|---------|
| 禁止 SCAN 批量删除 | ✅ 版本号方案 |
| DEL/Unlock 检查 error | ✅ 对齐已有模式 |

### 缓存红线 (`cache.md`)

| 条目 | 检查结果 |
|------|---------|
| 写后清缓存 | ✅ INCR 版本号 + DEL 详情 |
| 缓存无 TTL | ✅ 所有 key 带 TTL + jitter |
| 缓存击穿 | ✅ 详情用分布式锁 |

### 前端红线 (`frontend.md`)

不涉及（本 spec 仅后端）。

### ADMIN 专属红线 (`admin/red-lines.md`)

| 条目 | 检查结果 |
|------|---------|
| 禁止破坏游戏服务端数据格式 | ✅ config_json 结构对齐 API 契约 §5 |
| 禁止放行不支持的枚举值 | ✅ condition.op 校验对齐服务端 `validOps` |
| 禁止硬编码 | ✅ 错误码/Redis key/配置参数统一管理 |
| 禁止偏离跨模块代码模式 | ✅ 见下文逐条对齐 |

**跨模块代码模式逐条对齐**：

| 红线 | FSM 方案 |
|------|---------|
| Update 返回 `*string("保存成功")` | ✅ |
| Delete 返回 `*DeleteResult{ID, Name, Label}` | ✅ Label = DisplayName |
| ToggleEnabled 返回 `*string("操作成功")` | ✅ |
| ToggleEnabled 接收 `*model.ToggleEnabledRequest` | ✅ |
| 缓存读取用 `err == nil && hit` | ✅ |
| store 错误 `slog.Error` + `fmt.Errorf` 包装 | ✅ |
| store Create/Update 用 Request 结构体 | ✅ |
| handler 用共享 `checkID()` / `checkVersion()` | ✅ |
| handler 校验通过后才打 slog Debug | ✅ |

---

## 4. 扩展性影响

- **新增配置类型**：FSM 完全遵循"加一组 handler/service/store"模式，不改已有模块。**正面影响**。
- **新增表单字段**：不涉及（FSM 无扩展字段机制）。

---

## 5. 依赖方向

```
cmd/admin/main.go
  └─ handler/fsm_config.go
       └─ service/fsm_config.go
            ├─ store/mysql/fsm_config.go
            └─ store/redis/fsm_config_cache.go

handler/export.go
  └─ service/fsm_config.go
```

单向向下，无循环依赖。handler 可持有多个 service（export handler 持有 eventTypeService + fsmConfigService），service 间无横向依赖。

---

## 6. 陷阱检查

### Go (`dev-rules/go.md`)

- **json.RawMessage 对 null 不变 nil**：`config_json` 列是 `JSON NOT NULL`，不会出现 NULL scan 问题。但前端提交的 `condition.value` 可能是 `null`，需在 handler 层处理。
- **`len()` 不是字符数**：`display_name` 用 `utf8.RuneCountInString()` 校验；`name` 纯 ASCII 用 `len()` 无问题。

### MySQL (`dev-rules/mysql.md`)

- **乐观锁 rows==0 语义模糊**：沿用已有模式，service 层预检查（getOrNotFound）后再 UPDATE。
- **Docker initdb.d 只在首次初始化**：迁移文件需要手动执行或重建数据卷。

### Redis (`dev-rules/redis.md`)

- **Get 返回 redis.Nil**：用 `errors.Is(err, redis.Nil)` 判断，对齐已有模式。

### Cache (`dev-rules/cache.md`)

- **写后清缓存顺序**：先写 DB 成功，后 INCR 版本号 + DEL 详情。
- **级联清缓存**：本期 FSM 无被其他模块缓存引用的场景，不需要级联。

---

## 7. 配置变更

### config.yaml 新增

```yaml
fsm_config:
  name_max_length: 64
  display_name_max_length: 128
  max_states: 50
  max_transitions: 200
  condition_max_depth: 10
  cache_detail_ttl: 300s
  cache_list_ttl: 300s
  cache_lock_ttl: 5s
```

### 迁移文件

`migrations/006_create_fsm_configs.sql`：见 §1.1。

---

## 8. 测试策略

### API 测试（`tests/api_test.sh` 追加 FSM 段）

沿用已有 curl 测试模式，覆盖：

1. **正常 CRUD 流程**：create → detail → list → update → toggle-enabled → delete
2. **name 唯一性**：重复 name 创建返回 43001
3. **name 格式校验**：非法字符返回 43002
4. **配置校验**：
   - states 为空 → 43004
   - 状态名重复 → 43005
   - initial_state 不在 states → 43006
   - transition from/to 不在 states → 43007
   - condition op 不合法 → 43008
   - condition 嵌套超深 → 43008
5. **生命周期**：
   - 启用中编辑 → 43010
   - 启用中删除 → 43009
6. **乐观锁**：过期 version 更新 → 43011
7. **导出 API**：`GET /api/configs/fsm_configs` 返回正确格式
