# fsm-state-dict-backend — 设计方案

## 方案描述

### 核心思路

FSM 状态字典是一个标准配置管理模块，结构与 `fsm_config`（模块 6）完全同构。分层命名、缓存模式、乐观锁、错误处理全部复用已有模式，零新概念引入。

唯一特殊点：**删除引用保护**需要扫描 `fsm_configs.config_json`（MySQL JSON 列）查找 FSM 是否引用了该状态名。扫描结果（referenced_by 列表）随错误响应一起返回。`WrapCtx` 已原生支持此模式（`writeError(c, err, resp)` 在 error 时也传入 resp 作为 data）——无需修改任何基础设施。

---

### 数据模型

#### MySQL 表 `fsm_state_dicts`

```sql
CREATE TABLE IF NOT EXISTS fsm_state_dicts (
    id           BIGINT       AUTO_INCREMENT PRIMARY KEY,
    name         VARCHAR(64)  NOT NULL,         -- 机器标识，正则 ^[a-z][a-z0-9_]*$，创建后不可改
    display_name VARCHAR(128) NOT NULL,         -- 中文显示名
    category     VARCHAR(64)  NOT NULL,         -- 分类（战斗/移动/社交/活动/通用）
    description  VARCHAR(512) NOT NULL DEFAULT '', -- 可选说明
    enabled      TINYINT(1)   NOT NULL DEFAULT 1,
    version      INT          NOT NULL DEFAULT 1,  -- 乐观锁
    created_at   DATETIME     NOT NULL,
    updated_at   DATETIME     NOT NULL,
    deleted      TINYINT(1)   NOT NULL DEFAULT 0,  -- 软删除

    UNIQUE KEY uk_name (name),
    INDEX idx_list (deleted, enabled, id DESC),
    INDEX idx_category (deleted, category)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**字段说明**：
- `name` 唯一（含软删除），创建后不可改（UpdateRequest 不含 name 字段）
- `category` 为自由字符串（不建独立表），`ListCategories` 做 DISTINCT 查询
- `description` 默认空字符串（非 NULL），避免 omitempty 问题

#### Model 结构

```go
// DB 行结构
type FsmStateDict struct {
    ID          int64     `json:"id" db:"id"`
    Name        string    `json:"name" db:"name"`
    DisplayName string    `json:"display_name" db:"display_name"`
    Category    string    `json:"category" db:"category"`
    Description string    `json:"description" db:"description"`
    Enabled     bool      `json:"enabled" db:"enabled"`
    Version     int       `json:"version" db:"version"`
    CreatedAt   time.Time `json:"created_at" db:"created_at"`
    UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
    Deleted     bool      `json:"-" db:"deleted"`
}

// 列表 item
type FsmStateDictListItem struct {
    ID          int64     `json:"id" db:"id"`
    Name        string    `json:"name" db:"name"`
    DisplayName string    `json:"display_name" db:"display_name"`
    Category    string    `json:"category" db:"category"`
    Enabled     bool      `json:"enabled" db:"enabled"`
    CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// 列表查询参数
type FsmStateDictListQuery struct {
    Name        string `json:"name"`         // 模糊搜索（name/display_name LIKE）
    Category    string `json:"category"`     // 精确过滤
    Enabled     *bool  `json:"enabled,omitempty"`
    Page        int    `json:"page"`
    PageSize    int    `json:"page_size"`
}

// 删除结果（含引用信息，WrapCtx 在 error 时原样传入 data）
type FsmStateDictDeleteResult struct {
    ID           int64          `json:"id,omitempty"`
    Name         string         `json:"name,omitempty"`
    DisplayName  string         `json:"display_name,omitempty"`
    ReferencedBy []FsmConfigRef `json:"referenced_by,omitempty"` // 仅在 ErrFsmStateDictInUse 时有值
}

// 引用详情（供 DeleteResult 使用）
type FsmConfigRef struct {
    ID          int64  `json:"id" db:"id"`
    Name        string `json:"name" db:"name"`
    DisplayName string `json:"display_name" db:"display_name"`
    Enabled     bool   `json:"enabled" db:"enabled"`
}
```

---

### 接口定义

8 个 REST 接口，全部 POST，路径 `/api/v1/fsm-state-dicts/`：

| 路径 | 功能 |
|---|---|
| `POST /list` | 分页列表，支持 name/display_name 模糊 + category + enabled 筛选 |
| `POST /create` | 创建（name 唯一） |
| `POST /detail` | 按 ID 查询详情 |
| `POST /update` | 编辑（display_name/category/description，乐观锁） |
| `POST /delete` | 软删除（先引用检查，blocked 时携带 referenced_by） |
| `POST /check-name` | 名称唯一性校验（含软删除） |
| `POST /toggle-enabled` | 启用/停用（乐观锁） |
| `POST /list-categories` | 返回 `[]string` 所有分类，供筛选下拉 |

---

### 富错误响应模式（Delete 被引用时）

`WrapCtx` 签名：
```go
resp, err := fn(c.Request.Context(), &req)
if err != nil {
    writeError(c, err, resp)  // ← resp 在 error 时仍作为 data 传入
    return
}
```

因此 Service.Delete 返回：
- 成功：`(&FsmStateDictDeleteResult{ID, Name, DisplayName}, nil)`
- 被引用：`(&FsmStateDictDeleteResult{ReferencedBy: [...]}, errcode.New(ErrFsmStateDictInUse))`

响应体：
```json
{
  "code": 43020,
  "message": "状态字典条目被 FSM 引用，无法删除",
  "data": {
    "referenced_by": [
      {"id": 1, "name": "combat_fsm", "display_name": "战斗状态机", "enabled": true}
    ]
  }
}
```

**不需要修改 WrapCtx**，现有实现已完全支持。

---

### 引用扫描 SQL

在 `FsmConfigStore` 新增方法 `ListFsmConfigsReferencingState`：

```sql
SELECT id, name, display_name, enabled
FROM fsm_configs
WHERE deleted = 0
  AND JSON_SEARCH(config_json, 'one', ?, NULL, '$.states[*].name') IS NOT NULL
LIMIT ?
```

`JSON_SEARCH(col, 'one', searchStr, NULL, path)` 精确匹配 `config_json.states[].name == stateName`，返回 NULL（无命中）或路径字符串（命中）。`LIMIT 20` 避免响应过大。

> **注**：此方法加在 `FsmConfigStore`（fsm_configs 表的 store），不放在 `FsmStateDictStore`。属于"FsmConfig 表的查询方法"，分层正确，不产生反向依赖。

---

### 缓存方案

与 `FsmConfigCache` 完全同构：

| 缓存类型 | 方案 | 实现 |
|---|---|---|
| 列表 | 版本号失效（INCR key） | 写操作调用 `InvalidateList` |
| 详情 | Cache-Aside + 分布式锁防击穿 + 空标记 | `TryLock + GetDetail + SetDetail` |
| 降级 | Redis 不可用直查 MySQL | error 时 log + fallback |

Redis key 前缀（新增到 `store/redis/shared/keys.go`）：

```go
const (
    FsmStateDictListVersionKey = "fsm_state_dicts:list:version"
)

func FsmStateDictListKey(version int64, name, category string, enabled *bool, page, pageSize int) string
func FsmStateDictDetailKey(id int64) string
func FsmStateDictLockKey(id int64) string
```

---

### 错误码（43013–43024）

```go
ErrFsmStateDictNameExists       = 43013  // 标识已存在（含软删除）
ErrFsmStateDictNameInvalid      = 43014  // 标识格式不合法
ErrFsmStateDictNotFound         = 43015  // 条目不存在
ErrFsmStateDictDeleteNotDisabled = 43016 // 删除前必须先停用
ErrFsmStateDictVersionConflict  = 43017  // 版本冲突
ErrFsmStateDictInUse            = 43020  // 被 FSM 引用，无法删除（携带 referenced_by）
// 43018-43019、43021-43024 预留
```

---

### 配置

新增 `FsmStateDictConfig` 到 `config/config.go`：

```yaml
fsm_state_dict:
  name_max_length: 64
  display_name_max_length: 128
  category_max_length: 64
  description_max_length: 512
```

Go 结构：
```go
type FsmStateDictConfig struct {
    NameMaxLength        int `yaml:"name_max_length"`
    DisplayNameMaxLength int `yaml:"display_name_max_length"`
    CategoryMaxLength    int `yaml:"category_max_length"`
    DescriptionMaxLength int `yaml:"description_max_length"`
}
```

TTL 复用 `store/redis/shared/common.go` 中的 `DetailTTLBase`/`ListTTLBase` 全局常量，不单独配置。

---

## 方案对比

### 备选方案 A：分类独立建表

把 category 单独建 `fsm_state_categories` 表，字典条目外键关联。

**不选原因**：
1. 分类数量极少（< 10 个），不值得增加一层实体管理
2. 独立表意味着新增分类也需要 CRUD 接口，开发量翻倍
3. 项目明确 "禁止外键约束"（ADMIN 专属红线），且自由字符串 + DISTINCT 查询已满足所有需求

### 备选方案 B：引用计数字段 ref_count

维护 `ref_count` 字段，FSM 每次引用/解引用时 +1/-1。

**不选原因**：
1. requirements.md 明确 "不做字典条目被 FSM 引用计数"
2. ref_count 与 fsm_configs.config_json 之间的一致性难以保证（FSM 直接 JSON 编辑时绕过计数）
3. 实时扫描（20 条 LIMIT）完全满足性能需求，响应时间 < 10ms

### 备选方案 C：删除时只返回错误码，不携带 referenced_by

出错时只返回 43020，不携带 referenced_by 列表，前端另行查询。

**不选原因**：
1. requirements.md R18 明确要求携带 referenced_by
2. WrapCtx 已原生支持（resp + error 同时非 nil 时 data = resp），实现成本为零
3. 前端可直接显示"被哪些 FSM 引用"，用户体验更好

---

## 红线检查

| 红线 | 检查结果 |
|---|---|
| §4（禁止外键约束）| ✓ 纯软删除，无 FK |
| §10（handler 模式一致）| ✓ Update→SuccessMsg，Delete→DeleteResult，Toggle→SuccessMsg |
| §11（Redis key 在 shared/）| ✓ 新增到 `store/redis/shared/keys.go` |
| §16（缓存清除在 Commit 前）| ✓ DelDetail/InvalidateList 在 Commit 前 |
| §17（Unlock 必须传 lockID）| ✓ 复用 FsmConfigCache 模式 |
| §18（事务内用 tx）| ✓ 不涉及多步事务（无 ref 表写入） |
| MySQL：LIKE 转义 | ✓ `shared.EscapeLike` |
| MySQL：乐观锁 rows==0 | ✓ 返回 ErrFsmStateDictVersionConflict |
| Go：nil slice 初始化 | ✓ `make([]FsmStateDictListItem, 0)` |
| Go：len() 不用于中文 | ✓ `utf8.RuneCountInString` |
| Go：error 不忽略 | ✓ 所有 err 均有处理 |
| Redis：不用 SCAN+DEL | ✓ 版本号方案 |
| 缓存：击穿防护 | ✓ TryLock 分布式锁 |
| 缓存：空标记防穿透 | ✓ NullMarker |
| 缓存：TTL 有抖动 | ✓ TTL(base, jitter) |

---

## 扩展性影响

- **新增配置类型**：正面。FSM 状态字典完全遵循"加一组 handler/service/store/model"的模式，对字段/模板/FSM 零入侵（FsmConfigStore 新增一个查询方法，不改已有签名）。
- **新增表单字段**：不涉及。

---

## 依赖方向

```
router
  └── handler/fsm_state_dict.go
        └── service/fsm_state_dict.go
              ├── store/mysql/fsm_state_dict.go     (新增)
              ├── store/mysql/fsm_config.go          (新增 ListFsmConfigsReferencingState 方法)
              └── store/redis/fsm_state_dict_cache.go (新增)
                    └── store/redis/shared/keys.go   (新增 key 常量)
```

依赖单向向下，无循环。`FsmStateDictService` 依赖 `FsmConfigStore`（读取引用），但 `FsmConfigService` 不依赖 `FsmStateDictStore`——方向正确。

---

## 陷阱检查

### MySQL

- `JSON_SEARCH` 精确匹配：第二参数 `'one'` 找到第一个即返回，语义正确（只需知道"是否存在"）；不用 `'all'` 避免返回数组。✓
- `description` 默认 `NOT NULL DEFAULT ''`，避免 `*string` 指针 + omitempty 吞零值问题。✓
- `category` 不是枚举字段，DISTINCT 查询，`ListCategories` 仅返回 `deleted=0` 的 category。✓
- `LIKE` 搜索同时匹配 `name` 和 `display_name`（两个条件 OR 关系）：`(name LIKE ? OR display_name LIKE ?)`，转义通配符。✓

### Redis

- `ListCategories` 结果是否缓存：**不缓存**。分类数量 < 10，每次直查 MySQL 一条 DISTINCT 语句，响应时间 < 1ms，无需缓存复杂度。✓
- 版本号 key 与列表 key 的 TTL：列表 TTL 1min，版本号 key 永不过期（不设 TTL），INCR 后旧 key 自然超时。✓

### Go

- `FsmStateDictDeleteResult.ReferencedBy` 为 `[]FsmConfigRef`，初始化用 `make([]FsmConfigRef, 0)`，避免 JSON 序列化为 null。✓
- 分类长度用 `utf8.RuneCountInString`（分类名可能含中文，如"战斗"）。✓
- `WrapCtx` 已在 error 时传 resp 给 data，不需要修改基础设施。✓

---

## 配置变更

新增 `config/config.go` 字段：
```go
FsmStateDict FsmStateDictConfig `yaml:"fsm_state_dict"`
```

`config.yaml` 追加：
```yaml
fsm_state_dict:
  name_max_length: 64
  display_name_max_length: 128
  category_max_length: 64
  description_max_length: 512
```

---

## 测试策略

**编译验证**：`go build ./...` 通过。

**单元测试**：无（同 fsm_config 模块，无数据库依赖的纯计算逻辑极少）。

**e2e 验证（curl）**：
1. 创建 3 条字典条目（idle/attack/patrol），验证 R1/R7/R23
2. `list?category=战斗` 过滤，验证 R1
3. `check-name?name=idle` 返回 `exists: true`，验证 R7
4. 停用 idle，删除 idle（无引用），验证 R19
5. 在 FSM 配置中添加 attack 状态，删除 attack 字典条目 → 应返回 43020 + referenced_by，验证 R17/R18
6. `list-categories` 返回 `["战斗","移动",...]`，验证 R1
7. Seed 重复执行不报错，验证 R24
8. 重复 name 创建返回 43013，验证 R7
9. 更新 version 过期返回 43017，验证 R10
