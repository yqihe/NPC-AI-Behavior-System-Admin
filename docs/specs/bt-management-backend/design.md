# 行为树管理 — 设计方案（后端）

> 对应需求：[requirements.md](./requirements.md)

---

## 1. 方案描述

### 1.1 数据模型

#### MySQL 表：`bt_trees`

```sql
CREATE TABLE IF NOT EXISTS bt_trees (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    name            VARCHAR(128) NOT NULL,              -- 唯一标识（如 wolf/attack），创建后不可变
    display_name    VARCHAR(128) NOT NULL,              -- 中文名（列表搜索用）
    description     TEXT,                              -- 描述（可空）
    config          JSON         NOT NULL,              -- 根节点 JSON，导出 API 原样输出

    enabled         TINYINT(1)   NOT NULL DEFAULT 0,    -- 启用状态（创建默认 0）
    version         INT          NOT NULL DEFAULT 1,    -- 乐观锁
    created_at      DATETIME     NOT NULL,
    updated_at      DATETIME     NOT NULL,
    deleted         TINYINT(1)   NOT NULL DEFAULT 0,    -- 软删除

    -- name 唯一约束不含 deleted：软删后 name 仍占唯一性，不可复用
    UNIQUE KEY uk_name (name),
    -- 覆盖索引：列表分页查询
    INDEX idx_list (deleted, enabled, id DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**设计决策**：
- `config` 列名不加 `_json` 后缀（api-contract 字段名就是 `config`，保持语义一致）
- `name` 格式 `^[a-z][a-z0-9_/]*$`，斜杠对 MySQL 无意义，仅前端分组用
- 搜索维度：`name`（前缀匹配）+ `display_name`（模糊）+ `enabled`（精确）

#### config 内部结构（对齐 api-contract §6）

```json
{
  "type": "sequence",
  "children": [
    {
      "type": "check_bb_float",
      "key": "player_distance",
      "op": "<",
      "value": 5
    },
    {
      "type": "stub_action",
      "name": "melee_attack",
      "result": "success"
    }
  ]
}
```

节点参数展开到顶层（无 `params` 包装层），与服务端消费格式一致。

#### MySQL 表：`bt_node_types`

```sql
CREATE TABLE IF NOT EXISTS bt_node_types (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    type_name       VARCHAR(64)  NOT NULL,              -- 节点类型标识（与导出 JSON 的 type 字段一致）
    category        VARCHAR(16)  NOT NULL,              -- composite / decorator / leaf
    label           VARCHAR(128) NOT NULL,              -- 中文名（如 "序列"）
    description     TEXT,                              -- 描述（可空）
    param_schema    JSON         NOT NULL,              -- 参数定义，编辑器用于动态渲染表单

    is_builtin      TINYINT(1)   NOT NULL DEFAULT 0,    -- 1=内置种子，不可删除/编辑
    enabled         TINYINT(1)   NOT NULL DEFAULT 1,    -- 内置类型默认启用
    version         INT          NOT NULL DEFAULT 1,    -- 乐观锁
    created_at      DATETIME     NOT NULL,
    updated_at      DATETIME     NOT NULL,
    deleted         TINYINT(1)   NOT NULL DEFAULT 0,    -- 软删除

    UNIQUE KEY uk_type_name (type_name),
    INDEX idx_list (deleted, enabled, category, id DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**设计决策**：
- `category` 用 VARCHAR(16) 而非 ENUM：Go 层做枚举校验，DB 层保持灵活
- `is_builtin` 独立列而非靠 `version=1` 推断：语义明确，防误删种子数据
- 索引含 `category`：节点类型列表常按分类筛选

#### param_schema 结构定义

```json
{
  "params": [
    {
      "name": "key",
      "label": "BB Key",
      "type": "bb_key",
      "required": true
    },
    {
      "name": "op",
      "label": "操作符",
      "type": "select",
      "options": ["<", "<=", ">", ">=", "==", "!="],
      "required": true
    },
    {
      "name": "value",
      "label": "比较值",
      "type": "float",
      "required": true
    }
  ]
}
```

param `type` 合法值：`bb_key` / `string` / `float` / `integer` / `bool` / `select`
- `select` 类型必须有非空 `options` 数组
- composite / decorator 节点的 `param_schema` 为 `{"params": []}`（空数组）

---

### 1.2 Go 数据结构

#### `model/bt_tree.go`

```go
// BtTree DB 行结构体
type BtTree struct {
    ID          int64           `json:"id"           db:"id"`
    Name        string          `json:"name"         db:"name"`
    DisplayName string          `json:"display_name" db:"display_name"`
    Description string          `json:"description"  db:"description"`
    Config      json.RawMessage `json:"config"       db:"config"`
    Enabled     bool            `json:"enabled"      db:"enabled"`
    Version     int             `json:"version"      db:"version"`
    CreatedAt   time.Time       `json:"created_at"   db:"created_at"`
    UpdatedAt   time.Time       `json:"updated_at"   db:"updated_at"`
    Deleted     bool            `json:"-"            db:"deleted"`
}

// BtTreeListItem 列表页展示项（不含 config，减少传输量）
type BtTreeListItem struct {
    ID          int64     `json:"id"           db:"id"`
    Name        string    `json:"name"         db:"name"`
    DisplayName string    `json:"display_name" db:"display_name"`
    Enabled     bool      `json:"enabled"      db:"enabled"`
    CreatedAt   time.Time `json:"created_at"   db:"created_at"`
}

// BtTreeDetail 详情（含 config + version）
type BtTreeDetail struct {
    ID          int64           `json:"id"`
    Name        string          `json:"name"`
    DisplayName string          `json:"display_name"`
    Description string          `json:"description"`
    Config      json.RawMessage `json:"config"`
    Enabled     bool            `json:"enabled"`
    Version     int             `json:"version"`
}

// CreateBtTreeRequest 创建请求
type CreateBtTreeRequest struct {
    Name        string          `json:"name"`
    DisplayName string          `json:"display_name"`
    Description string          `json:"description"`
    Config      json.RawMessage `json:"config"`
}

// UpdateBtTreeRequest 更新请求
type UpdateBtTreeRequest struct {
    ID          int64           `json:"id"`
    Version     int             `json:"version"`
    DisplayName string          `json:"display_name"`
    Description string          `json:"description"`
    Config      json.RawMessage `json:"config"`
}
```

#### `model/bt_node_type.go`

```go
// BtNodeType DB 行结构体
type BtNodeType struct {
    ID          int64           `json:"id"           db:"id"`
    TypeName    string          `json:"type_name"    db:"type_name"`
    Category    string          `json:"category"     db:"category"`
    Label       string          `json:"label"        db:"label"`
    Description string          `json:"description"  db:"description"`
    ParamSchema json.RawMessage `json:"param_schema" db:"param_schema"`
    IsBuiltin   bool            `json:"is_builtin"   db:"is_builtin"`
    Enabled     bool            `json:"enabled"      db:"enabled"`
    Version     int             `json:"version"      db:"version"`
    CreatedAt   time.Time       `json:"created_at"   db:"created_at"`
    UpdatedAt   time.Time       `json:"updated_at"   db:"updated_at"`
    Deleted     bool            `json:"-"            db:"deleted"`
}

// BtNodeTypeListItem 列表展示项
type BtNodeTypeListItem struct {
    ID        int64  `json:"id"        db:"id"`
    TypeName  string `json:"type_name" db:"type_name"`
    Category  string `json:"category"  db:"category"`
    Label     string `json:"label"     db:"label"`
    IsBuiltin bool   `json:"is_builtin" db:"is_builtin"`
    Enabled   bool   `json:"enabled"   db:"enabled"`
}

// BtNodeTypeDetail 详情（含 param_schema + version）
type BtNodeTypeDetail struct {
    ID          int64           `json:"id"`
    TypeName    string          `json:"type_name"`
    Category    string          `json:"category"`
    Label       string          `json:"label"`
    Description string          `json:"description"`
    ParamSchema json.RawMessage `json:"param_schema"`
    IsBuiltin   bool            `json:"is_builtin"`
    Enabled     bool            `json:"enabled"`
    Version     int             `json:"version"`
}

// CreateBtNodeTypeRequest 创建请求
type CreateBtNodeTypeRequest struct {
    TypeName    string          `json:"type_name"`
    Category    string          `json:"category"`
    Label       string          `json:"label"`
    Description string          `json:"description"`
    ParamSchema json.RawMessage `json:"param_schema"`
}

// UpdateBtNodeTypeRequest 更新请求
type UpdateBtNodeTypeRequest struct {
    ID          int64           `json:"id"`
    Version     int             `json:"version"`
    Label       string          `json:"label"`
    Description string          `json:"description"`
    ParamSchema json.RawMessage `json:"param_schema"`
}
```

---

### 1.3 API 接口（路由设计）

遵循 backend-conventions §八：POST 为主，GET 仅 List。

```
# bt_trees
POST   /api/v1/bt-trees               → BtTreeHandler.Create
GET    /api/v1/bt-trees               → BtTreeHandler.List
POST   /api/v1/bt-trees/detail        → BtTreeHandler.Detail
POST   /api/v1/bt-trees/update        → BtTreeHandler.Update
POST   /api/v1/bt-trees/delete        → BtTreeHandler.Delete
POST   /api/v1/bt-trees/toggle-enabled → BtTreeHandler.ToggleEnabled
POST   /api/v1/bt-trees/check-name    → BtTreeHandler.CheckName

# bt_node_types
POST   /api/v1/bt-node-types               → BtNodeTypeHandler.Create
GET    /api/v1/bt-node-types               → BtNodeTypeHandler.List
POST   /api/v1/bt-node-types/detail        → BtNodeTypeHandler.Detail
POST   /api/v1/bt-node-types/update        → BtNodeTypeHandler.Update
POST   /api/v1/bt-node-types/delete        → BtNodeTypeHandler.Delete
POST   /api/v1/bt-node-types/toggle-enabled → BtNodeTypeHandler.ToggleEnabled
POST   /api/v1/bt-node-types/check-name    → BtNodeTypeHandler.CheckName

# 导出（已有 export.go 追加）
GET    /api/configs/bt_trees               → ExportHandler.BTTrees
```

**List 查询参数**：

`BtTreeListQuery`：
```go
type BtTreeListQuery struct {
    Name        string `json:"name"`         // 前缀匹配
    DisplayName string `json:"display_name"` // 模糊匹配
    Enabled     *bool  `json:"enabled"`      // nil=全部
    Page        int    `json:"page"`
    PageSize    int    `json:"page_size"`
}
```

`BtNodeTypeListQuery`：
```go
type BtNodeTypeListQuery struct {
    TypeName  string `json:"type_name"`  // 前缀匹配
    Category  string `json:"category"`   // 精确匹配（composite/decorator/leaf/""）
    Enabled   *bool  `json:"enabled"`    // nil=全部
    Page      int    `json:"page"`
    PageSize  int    `json:"page_size"`
}
```

---

### 1.4 节点树校验逻辑

`service/bt_tree.go` 中递归校验函数（`validateBtNode`）：

```go
// validateBtNode 递归校验节点结构合法性
// nodeTypes: type_name → category（从 BtNodeTypeStore 预加载 enabled 且 not deleted 的类型）
func validateBtNode(node map[string]any, nodeTypes map[string]string, depth int) error {
    if depth > 20 {
        return errcode.New(errcode.ErrBtTreeNodeDepthExceeded)
    }
    typeName, ok := node["type"].(string)
    if !ok || typeName == "" {
        return errcode.New(errcode.ErrBtTreeConfigInvalid)
    }
    category, exists := nodeTypes[typeName]
    if !exists {
        return errcode.Newf(errcode.ErrBtTreeNodeTypeNotFound, "节点类型 %q 不存在或已禁用", typeName)
    }
    switch category {
    case "composite":
        children, ok := node["children"].([]any)
        if !ok || len(children) == 0 {
            return errcode.Newf(errcode.ErrBtTreeConfigInvalid, "composite 节点 %q 必须有非空 children", typeName)
        }
        if _, hasChild := node["child"]; hasChild {
            return errcode.Newf(errcode.ErrBtTreeConfigInvalid, "composite 节点不应有 child 字段")
        }
        for _, c := range children {
            child, ok := c.(map[string]any)
            if !ok {
                return errcode.New(errcode.ErrBtTreeConfigInvalid)
            }
            if err := validateBtNode(child, nodeTypes, depth+1); err != nil {
                return err
            }
        }
    case "decorator":
        childRaw, ok := node["child"]
        if !ok || childRaw == nil {
            return errcode.Newf(errcode.ErrBtTreeConfigInvalid, "decorator 节点 %q 必须有 child", typeName)
        }
        child, ok := childRaw.(map[string]any)
        if !ok {
            return errcode.New(errcode.ErrBtTreeConfigInvalid)
        }
        if _, hasChildren := node["children"]; hasChildren {
            return errcode.Newf(errcode.ErrBtTreeConfigInvalid, "decorator 节点不应有 children 字段")
        }
        if err := validateBtNode(child, nodeTypes, depth+1); err != nil {
            return err
        }
    case "leaf":
        if _, hasChildren := node["children"]; hasChildren {
            return errcode.Newf(errcode.ErrBtTreeConfigInvalid, "leaf 节点不能有 children 字段")
        }
        if _, hasChild := node["child"]; hasChild {
            return errcode.Newf(errcode.ErrBtTreeConfigInvalid, "leaf 节点不能有 child 字段")
        }
    }
    return nil
}
```

**节点类型预加载**：Create/Update 时先调 `BtNodeTypeStore.ListEnabledTypes(ctx)` 返回 `map[string]string`（type_name → category），再传入 `validateBtNode`。避免每个节点单独查 DB。

---

### 1.5 BB Key 引用扫描

```go
// BtTreeStore.IsBBKeyUsed 扫描所有未删除行为树的 config，提取所有 bb_key 类型参数值
// 实现：SELECT id, config FROM bt_trees WHERE deleted=0
//   → JSON_EXTRACT 递归提取 key 字段
// 方案：Go 层递归遍历（不用 JSON_EXTRACT），避免 MySQL JSON 函数跨版本差异
func (s *BtTreeStore) IsBBKeyUsed(ctx context.Context, bbKey string) (bool, error)
func (s *BtTreeStore) GetBBKeyUsages(ctx context.Context, bbKey string) ([]string, error)

// extractBBKeys 递归提取节点树中所有 bb_key 类型参数值
// node: json.Unmarshal 后的 map
// nodeParamTypes: type_name → bb_key 参数名列表（从 bt_node_types 预加载）
func extractBBKeys(node map[string]any, nodeParamTypes map[string][]string) []string
```

扫描策略：

1. `SELECT id, name, config FROM bt_trees WHERE deleted=0`（全量扫描，BT 总数可控）
2. Go 层 `json.Unmarshal` 每棵树的 config
3. 递归遍历节点，查 `nodeParamTypes` 中该 type 的 `bb_key` 参数名，提取对应值
4. 与目标 bbKey 比对

`nodeParamTypes` 从 `BtNodeTypeStore` 预加载：只加载 `type` 为 `bb_key` 的参数位置。

> 注意 general.md 红线：`json.Unmarshal` 失败时**不** continue 跳过，返回 error 阻止操作。

---

### 1.6 种子数据

```go
// cmd/seed/bt_node_type_seed.go
var builtinNodeTypes = []model.CreateBtNodeTypeRequest{
    {
        TypeName: "sequence", Category: "composite", Label: "序列",
        Description: "顺序执行子节点，全部成功才成功，任一失败立即停止",
        ParamSchema: mustJSON(`{"params":[]}`),
    },
    {
        TypeName: "selector", Category: "composite", Label: "选择器",
        Description: "顺序执行子节点，第一个成功即返回成功，全部失败才失败",
        ParamSchema: mustJSON(`{"params":[]}`),
    },
    {
        TypeName: "parallel", Category: "composite", Label: "并行",
        Description: "同时执行全部子节点",
        ParamSchema: mustJSON(`{"params":[]}`),
    },
    {
        TypeName: "inverter", Category: "decorator", Label: "取反",
        Description: "翻转子节点的执行结果（成功↔失败）",
        ParamSchema: mustJSON(`{"params":[]}`),
    },
    {
        TypeName: "check_bb_float", Category: "leaf", Label: "检查浮点 BB",
        Description: "读取 Blackboard 浮点值并与阈值比较",
        ParamSchema: mustJSON(`{"params":[
            {"name":"key","label":"BB Key","type":"bb_key","required":true},
            {"name":"op","label":"操作符","type":"select","options":["<","<=",">",">=","==","!="],"required":true},
            {"name":"value","label":"比较值","type":"float","required":true}
        ]}`),
    },
    {
        TypeName: "check_bb_string", Category: "leaf", Label: "检查字符串 BB",
        Description: "读取 Blackboard 字符串值并与目标值比较",
        ParamSchema: mustJSON(`{"params":[
            {"name":"key","label":"BB Key","type":"bb_key","required":true},
            {"name":"op","label":"操作符","type":"select","options":["==","!="],"required":true},
            {"name":"value","label":"比较值","type":"string","required":true}
        ]}`),
    },
    {
        TypeName: "set_bb_value", Category: "leaf", Label: "设置 BB 值",
        Description: "向 Blackboard 写入指定 Key 的值",
        ParamSchema: mustJSON(`{"params":[
            {"name":"key","label":"BB Key","type":"bb_key","required":true},
            {"name":"value","label":"设定值","type":"string","required":true}
        ]}`),
    },
    {
        TypeName: "stub_action", Category: "leaf", Label: "存根动作",
        Description: "占位动作节点，返回固定结果（调试/占位用）",
        ParamSchema: mustJSON(`{"params":[
            {"name":"name","label":"动作名","type":"string","required":true},
            {"name":"result","label":"返回结果","type":"select","options":["success","failure","running"],"required":true}
        ]}`),
    },
}
```

种子脚本在 `cmd/seed/main.go` 中调用，幂等（按 `type_name` upsert，`is_builtin=1`），不删除已有数据。

---

### 1.7 错误码

```go
// errcode/codes.go 新增段位 440xx

// --- 行为树管理 440xx ---
const (
    ErrBtTreeNameExists        = 44001 // 行为树标识已存在（含软删除）
    ErrBtTreeNameInvalid       = 44002 // 行为树标识格式不合法
    ErrBtTreeNotFound          = 44003 // 行为树不存在
    ErrBtTreeConfigInvalid     = 44004 // 树结构不合法
    ErrBtTreeNodeTypeNotFound  = 44005 // 节点类型不存在或已禁用
    ErrBtTreeNodeDepthExceeded = 44006 // 节点嵌套深度超过 20 层
    ErrBtTreeDeleteNotDisabled = 44009 // 删除前必须先停用
    ErrBtTreeEditNotDisabled   = 44010 // 编辑前必须先停用
    ErrBtTreeVersionConflict   = 44011 // 版本冲突（乐观锁）
    ErrBtTreeRefDelete         = 44012 // 被 NPC 引用，无法删除（占位）
    // 44013-44015 预留
)

// --- 节点类型管理 44016-44025 ---
const (
    ErrBtNodeTypeNameExists        = 44016 // 节点类型标识已存在（含软删除）
    ErrBtNodeTypeNameInvalid       = 44017 // 节点类型标识格式不合法
    ErrBtNodeTypeNotFound          = 44018 // 节点类型不存在
    ErrBtNodeTypeCategoryInvalid   = 44019 // category 枚举非法
    ErrBtNodeTypeDeleteNotDisabled = 44020 // 删除前必须先停用
    ErrBtNodeTypeEditNotDisabled   = 44021 // 编辑前必须先停用
    ErrBtNodeTypeRefDelete         = 44022 // 被行为树引用，无法删除（携带引用树名列表）
    ErrBtNodeTypeBuiltinDelete     = 44023 // 内置类型不可删除
    ErrBtNodeTypeBuiltinEdit       = 44024 // 内置类型不可编辑
    ErrBtNodeTypeParamSchemaInvalid = 44025 // param_schema 不合法
)
```

---

### 1.8 Redis 缓存 Key

`store/redis/config/keys.go` 新增：

```go
const (
    prefixBtTreeList   = "bt_trees:list:"
    prefixBtTreeDetail = "bt_trees:detail:"
    prefixBtTreeLock   = "bt_trees:lock:"

    prefixBtNodeTypeList   = "bt_node_types:list:"
    prefixBtNodeTypeDetail = "bt_node_types:detail:"
    prefixBtNodeTypeLock   = "bt_node_types:lock:"

    BtTreeListVersionKey     = "bt_trees:list:version"
    BtNodeTypeListVersionKey = "bt_node_types:list:version"
)

func BtTreeListKey(version int64, name, displayName string, enabled *bool, page, pageSize int) string
func BtTreeDetailKey(id int64) string
func BtTreeLockKey(id int64) string

func BtNodeTypeListKey(version int64, typeName, category string, enabled *bool, page, pageSize int) string
func BtNodeTypeDetailKey(id int64) string
func BtNodeTypeLockKey(id int64) string
```

---

### 1.9 config.go 新增配置

```go
// AppConfig 中追加
BtTree     BtTreeConfig     `yaml:"bt_tree"`
BtNodeType BtNodeTypeConfig `yaml:"bt_node_type"`

type BtTreeConfig struct {
    NameMaxLength        int           `yaml:"name_max_length"`
    DisplayNameMaxLength int           `yaml:"display_name_max_length"`
    CacheDetailTTL       time.Duration `yaml:"cache_detail_ttl"`
    CacheListTTL         time.Duration `yaml:"cache_list_ttl"`
}

type BtNodeTypeConfig struct {
    NameMaxLength        int           `yaml:"name_max_length"`
    LabelMaxLength       int           `yaml:"label_max_length"`
    CacheDetailTTL       time.Duration `yaml:"cache_detail_ttl"`
    CacheListTTL         time.Duration `yaml:"cache_list_ttl"`
}
```

`config.yaml` 示例值：
```yaml
bt_tree:
  name_max_length: 128
  display_name_max_length: 128
  cache_detail_ttl: 10m
  cache_list_ttl: 5m
bt_node_type:
  name_max_length: 64
  label_max_length: 128
  cache_detail_ttl: 30m
  cache_list_ttl: 15m
```

---

## 2. 方案对比

### 备选方案 A：行为树节点拆表存储

将树结构拆为 `bt_nodes` 关系表（每个节点一行，parent_id 外键），而非存 JSON。

**优点**：可直接 SQL 查询某节点类型被哪些树使用。

**不选原因**：
1. 导出 API 需要原样输出 JSON，拆表存储则每次导出都要递归重组，性能差
2. 树深度不固定，递归查询在 MySQL 中用 CTE 实现，复杂且慢
3. 节点编辑必然是整棵树重建（前端树编辑器输出整体 JSON），拆表无法增量更新
4. `bt_node_type` 删除时的引用检查可以全量扫 config JSON（BT 总数可控），不需要关系表

结论：JSON 单列存储（对齐 FSM config_json 模式）是正确选择。

### 备选方案 B：节点类型硬编码在前端

前端写死已知节点类型（check_bb_float 等），不建 `bt_node_types` 表。

**优点**：开发量少。

**不选原因**：
1. features.md 明确要求可扩展（服务端注册新节点 → ADMIN 出现新选项）
2. 硬编码违反 CLAUDE.md "所有下拉选项从数据库动态获取" 原则
3. 节点类型的 param_schema 需要持久化，供编辑器渲染动态表单

---

## 3. 红线检查

### ADMIN 专属红线

| # | 红线 | 状态 | 说明 |
|---|------|------|------|
| 1.4 | 禁止装饰节点归类为复合节点 | ✅ | `inverter` 归 decorator，用 `child`（单对象）；composite 用 `children`（数组）；`validateBtNode` switch 分支强制区分 |
| 1.5 | 禁止放行服务端不支持的枚举值 | ✅ | `op` 枚举、`result` 枚举由 param_schema options 约束，前端选择器不允许手填 |
| 2.1 | 禁止删除被引用配置 | ✅ | bt_node_type 删除前扫描所有 bt_tree.config |
| 4.1-4.8 | 禁止硬编码 | ✅ | 错误码用常量、Redis key 用函数、分页用 config |
| 10.1 | handler 响应格式一致 | ✅ | Update→SuccessMsg、Delete→DeleteResult、ToggleEnabled→SuccessMsg |
| 10.2 | ToggleEnabled 接收目标状态 | ✅ | 用 `ToggleEnabledRequest` 包含目标 enabled 值 |
| 10.3 | 缓存读取模式 | ✅ | `err == nil && hit` 双判断 |
| 11.8 | Redis cache 文件命名 | ✅ | `bt_tree_cache.go` / `bt_node_type_cache.go` |
| 16 | Commit 前清缓存 | ✅ | 缓存失效在 `tx.Commit()` 后执行（BT 无事务跨表，直接 MySQL 写后清缓存） |
| 17 | LuaUnlock | ✅ | 沿用已有分布式锁工具，不自行实现 |
| 18 | 事务内不绕过 tx | ✅ | bt_node_type 删除检查（扫 BT config）不在事务内，不涉及此红线 |

### Go 红线

| 红线 | 状态 | 说明 |
|------|------|------|
| nil slice 序列化为 null | ✅ | BtTreeListItem 数组用 `make([]model.BtTreeListItem, 0)` |
| JSON 可空列用 `*json.RawMessage` | ✅ | `description` 是 TEXT 可空，但不用 RawMessage；`config`/`param_schema` 不可空，用 `json.RawMessage` |
| `json.Unmarshal` 失败不 continue | ✅ | IsBBKeyUsed 中 Unmarshal 失败返回 error |
| 中文字符串长度用 `utf8.RuneCountInString` | ✅ | display_name / label 长度校验用 RuneCount |

### MySQL 红线

| 红线 | 状态 | 说明 |
|------|------|------|
| LIKE 转义通配符 | ✅ | name / display_name 模糊查询用 `shared.EscapeLike` |
| 事务内不混用 s.db 和 tx | ✅ | 无跨表事务（BT 独立写，node_type 删除检查不在事务内） |

### 缓存红线

| 红线 | 状态 | 说明 |
|------|------|------|
| 写操作成功后清缓存 | ✅ | Create/Update/Delete/ToggleEnabled 均清 detail + INCR list version |
| 缓存无 TTL | ✅ | detail/list 均设置 TTL（带 jitter） |
| 热 key 并发控制 | ✅ | detail 缓存用分布式锁 TryLock + double-check |

---

## 4. 扩展性影响

- **新增配置类型**：BT 模块完全独立，不改已有模块代码（唯一例外是 `service/field.go` 注入 BTTreeStore，这是既定的跨模块引用检查模式，不是改已有逻辑）。✅ 正面
- **新增表单字段 / 新增节点类型**：策划在系统设置 > 节点类型页新增记录即可，编辑器自动出现新选项，无需改代码。✅ 显著正面

---

## 5. 依赖方向

```
handler/bt_tree.go
handler/bt_node_type.go
        ↓
service/bt_tree.go          service/bt_node_type.go
        ↓                           ↓
store/mysql/bt_tree.go      store/mysql/bt_node_type.go
store/redis/bt_tree_cache.go store/redis/bt_node_type_cache.go
        ↓                           ↓
             model/bt_tree.go
             model/bt_node_type.go
             errcode/codes.go
             util/const.go

service/field.go ──注入──→ store/mysql/bt_tree.go   (跨模块引用检查，单向)
handler/export.go ──调用──→ service/bt_tree.go
setup/services.go ──装配──→ 所有上述组件
```

所有依赖单向向下，handler → service → store → model，无环。

---

## 6. 陷阱检查

查阅 `docs/development/standards/dev-rules/go.md` 和 `docs/development/admin/dev-rules.md`：

1. **JSON NULL 列**：`description` 是 TEXT 可空，sqlx scan 时用 `sql.NullString`，序列化输出空字符串（而非 null）
2. **IsBBKeyUsed 全量扫描性能**：BT 数量预期 < 500，单次扫描 < 1ms，无需缓存。若未来规模增大可加 param_schema 反向索引表，当前不过度设计
3. **param_schema 中 `options` 字段为 `[]string`**：JSON 反序列化后是 `[]interface{}`，Go 校验时需逐个类型断言
4. **node type 预加载时机**：Create/Update BT 时从 DB 加载 enabled 节点类型，不用内存缓存（防止节点类型刚禁用但缓存未刷新时仍通过校验）
5. **`name` 含斜杠的唯一性**：MySQL `UNIQUE KEY uk_name (name)` 对斜杠无特殊处理，`wolf/attack` 和 `wolf/idle` 是两个独立的唯一值，符合预期

---

## 7. 配置变更

新增 `config.yaml` 字段（见 §1.9），向后兼容（有默认值兜底）。

无 DDL 破坏性变更，新增两张表，migration 010/011。

---

## 8. 测试策略

### 单元测试（Go）

- `validateBtNode`：覆盖 composite/decorator/leaf 各分支，超深嵌套，未知 type，错误 children/child 字段
- `extractBBKeys`：覆盖多层嵌套树，多个 bb_key 参数，无 bb_key 参数的节点

### 集成测试（curl/bash）

参考 `docs/development/admin/test-lifecycle-guard-npc.md` 风格，覆盖：

1. bt_node_type CRUD 完整流程（创建/详情/列表/更新/删除）
2. 内置节点类型不可删除/编辑
3. bt_tree CRUD（使用内置节点类型构建树）
4. 节点类型禁用后创建 BT 时校验失败
5. 节点类型被 BT 引用时删除失败（携带引用列表）
6. 删除 BT → 再删节点类型成功
7. 导出 API 返回正确格式
8. BB Key 引用检查（字段关闭 expose_bb 被 BT 使用时失败）
9. 乐观锁冲突场景
10. 软删除后 name 不可复用
