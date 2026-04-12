# 事件类型管理 — 后端架构

> 本文档补充 [features.md](features.md) 未覆盖的架构层信息：文件组织、存储设计、缓存策略、配置项、与功能点的对应关系。
> **实现状态**：规划中。前置依赖见 features.md 顶部说明。

---

## 目录结构

```
backend/
  cmd/admin/
    main.go                           # 新增路由注册 + EventTypeSchemaCache 启动装载
  internal/
    handler/
      event_type.go                   # CRUD + 列表 + 详情 + toggle
      event_type_schema.go             # 扩展字段 Schema CRUD
      export.go                        # 新增 EventTypes 分支
    service/
      event_type.go                    # 业务逻辑 + 扩展字段约束校验
      event_type_schema.go             # 扩展字段 Schema 业务逻辑
      constraint/
        validate.go                    # 【新增】从字段管理抽出的约束校验工具，两模块共用
    store/
      mysql/
        event_type.go                  # event_types 表
        event_type_schema.go           # event_type_schema 表
      redis/
        event_type_cache.go            # detail + list 缓存
    cache/
      event_type_schema_cache.go       # 启动时全量内存缓存
    model/
      event_type.go                    # EventType / EventTypeListItem / EventTypeDetail
      event_type_schema.go             # EventTypeSchema
    errcode/
      event_type.go                    # 42001-42039
  migrations/
    202604xx_create_event_types.sql
    202604xx_create_event_type_schema.sql
```

**注意复用点**：`service/constraint/validate.go` 是在做本模块时**从字段管理抽出的**独立约束校验工具。字段管理原来的 `service/field.go::checkConstraintTightened` 和 `validateReferenceRefs` 里的值级校验逻辑需要重构到这个包，字段管理自己也切换到用这个包。这是本期的一项辅助重构，落在"扩展字段值约束校验"这条路径上顺带做掉。

---

## 存储设计

### event_types 表

```sql
CREATE TABLE event_types (
  id              BIGINT       NOT NULL AUTO_INCREMENT,
  name            VARCHAR(64)  NOT NULL,
  display_name    VARCHAR(128) NOT NULL,
  perception_mode VARCHAR(16)  NOT NULL,
  config_json     JSON         NOT NULL,
  enabled         TINYINT      NOT NULL DEFAULT 0,
  version         INT          NOT NULL DEFAULT 1,
  created_at      DATETIME     NOT NULL,
  updated_at      DATETIME     NOT NULL,
  deleted         TINYINT      NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  UNIQUE KEY uk_name (name, deleted),
  KEY idx_list (deleted, enabled, id DESC),
  KEY idx_perception (deleted, perception_mode)
);
```

**设计要点：**

1. **只提 `perception_mode` 一列做 facet 筛选**。`default_severity` / `default_ttl` / `range` 留在 `config_json` 里，不做索引筛选（未来如需再加列 + 回填）。这是决策点"不提"的实现
2. **`config_json` 是系统字段 + 扩展字段的完整合并**，导出 API 直接原样输出到 HTTP 响应体，**不经过 Go struct 中转**。这样新增任意扩展字段 ADMIN 后端零代码改动
3. **`uk_name (name, deleted)` 含 deleted 列**：软删后 `deleted=1`，理论允许同名新记录和旧软删记录并存；**但** `ExistsByName` 查询**不带 `deleted` 过滤**，确保软删 name 不可复用（和字段/模板同构）
4. **`idx_list` 覆盖索引**满足列表分页：主查询只需要 `id / name / display_name / perception_mode / enabled / created_at`，从 `config_json` 抽展示值的 `default_severity` / `default_ttl` / `range` 在 Service 层 unmarshal 时取——覆盖索引命中后再回表一次拿 `config_json`，可以接受（量小）。如果未来量上来，`config_json` 可以独立放一个"延迟加载"列（MySQL 8.0 的 invisible column / 或拆二级表）

### event_type_schema 表

```sql
CREATE TABLE event_type_schema (
  id            BIGINT       NOT NULL AUTO_INCREMENT,
  field_name    VARCHAR(64)  NOT NULL,
  field_label   VARCHAR(128) NOT NULL,
  field_type    VARCHAR(16)  NOT NULL,
  constraints   JSON         NOT NULL,
  default_value JSON         NOT NULL,
  sort_order    INT          NOT NULL DEFAULT 0,
  enabled       TINYINT      NOT NULL DEFAULT 1,
  version       INT          NOT NULL DEFAULT 1,
  created_at    DATETIME     NOT NULL,
  updated_at    DATETIME     NOT NULL,
  deleted       TINYINT      NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  UNIQUE KEY uk_field_name (field_name, deleted)
);
```

**设计要点：**

1. **独立主键**，不复用字段管理的 `fields.id`（作用域隔离方案 B）。`field_name` 命名空间也独立——同名 `priority` 在字段管理和事件类型扩展中语义不同
2. **数据量小**（预计常态 < 50 条，绝对上限 100）：不建复杂索引，ORM 直查 MySQL 即可
3. **默认 `enabled=1`**：与事件类型的 `enabled=0` 相反，扩展字段定义默认立即生效，因为 schema 本身不会"半配置"（filled 完就 filled）
4. **`default_value` 类型为 JSON** 而非 VARCHAR：允许 int/bool/string/array 不同类型的默认值统一存储

---

## 缓存策略

### event_types 缓存（Redis）

| Key | 含义 | TTL |
|---|---|---|
| `event_types:detail:{id}` | 单条详情（含空标记防穿透） | 10min + jitter |
| `event_types:list:v{ver}:q={hash}` | 列表分页缓存 | 5min + jitter |
| `event_types:list:version` | 列表缓存版本号 | 永久 |
| `event_types:lock:{id}` | 分布式锁（防击穿） | 3s |

**失效规则：**
- 单条写操作（Create/Update/Delete/ToggleEnabled）：`DEL event_types:detail:{id}` + `INCR event_types:list:version`
- schema 写操作：不清 event_types 缓存（因为 detail 响应里的 extension_schema 来自另一个独立缓存）

### event_type_schema 缓存（内存）

**采用内存缓存，不走 Redis**。原因：

- 数据量极小（< 100 条），全量放内存 O(N) 遍历足够快
- 命中频率极高（每次事件类型新建/编辑/详情都要读）
- 和 `DictCache` 同构，服务启动时 `Load()` 一次性灌入
- 写频率极低（运营月级别偶尔改一次）

**失效规则**：`EventTypeSchemaService.Create` / `Update` / `ToggleEnabled` / `Delete` 完成后**同步调用** `EventTypeSchemaCache.Reload()` 重新全量加载。

**多实例部署的一致性问题**：单实例下 `Reload()` 就够；多实例下一个实例写入后其他实例的内存缓存会陈旧几秒。解决方案延后处理：
- 短期：接受陈旧窗口（运营写后等 30s 再验证）
- 长期：通过 Redis Pub/Sub 广播 reload 信号，所有实例收到后各自 `Reload()`

本期单实例开发，不处理多实例一致性。

---

## 扩展字段值校验（约束复用）

事件类型新建/编辑功能 2/4 里，Service 层需要校验运营提交的扩展字段值是否符合 `event_type_schema.constraints` 定义的约束。**复用字段管理的约束校验代码**。

计划重构：

1. 把 `service/field.go::checkConstraintTightened` 里的值级校验逻辑抽出到 `service/constraint/validate.go`：
   ```go
   package constraint
   
   // ValidateValue 检查 value 是否符合 (fieldType, constraints) 定义的约束
   func ValidateValue(fieldType string, constraints json.RawMessage, value json.RawMessage) error
   
   // ValidateConstraintsSelf 检查 constraints 自身是否自洽（比如 int 的 min <= max）
   func ValidateConstraintsSelf(fieldType string, constraints json.RawMessage) error
   ```

2. 字段管理的 `Create` / `Update` 改调 `constraint.ValidateValue`（在约束收紧检查之前先做值级校验）
3. 事件类型的 `EventTypeService.validateExtensions` 也调 `constraint.ValidateValue`
4. `EventTypeSchemaService.Create` 调 `constraint.ValidateConstraintsSelf` + `constraint.ValidateValue(type, constraints, default_value)`

**这项重构落在本期"事件类型管理"模块里**，是一次性成本。字段管理的原有功能不变，只是内部抽了层。

**不复用的部分**：
- `reference` 类型的校验（`validateReferenceRefs` / `detectCyclicRef`）留在字段管理自己那里，因为事件类型扩展字段不支持 reference
- 字段管理的"收紧检查"（`checkConstraintTightened`）留在字段管理自己那里，因为事件类型扩展字段编辑不做收紧拦截

---

## 配置项

在 `config.yaml` / `config.go` 新增：

```yaml
event_type:
  name_max_length: 64
  display_name_max_length: 128
  cache_detail_ttl: 10m
  cache_list_ttl: 5m
  cache_lock_ttl: 3s

event_type_schema:
  field_name_max_length: 64
  field_label_max_length: 128
  max_schemas: 100          # 扩展字段数量上限，防失控
```

---

## 功能与代码入口对应

| feature | handler | service | store |
|---|---|---|---|
| 1. 列表 | `EventTypeHandler.List` | `EventTypeService.List` | `EventTypeStore.List` |
| 2. 新建 | `EventTypeHandler.Create` | `EventTypeService.Create` | `EventTypeStore.Create` |
| 3. 详情 | `EventTypeHandler.Get` | `EventTypeService.GetByID` + `EventTypeSchemaService.ListEnabled` | `EventTypeStore.GetByID` |
| 4. 编辑 | `EventTypeHandler.Update` | `EventTypeService.Update` | `EventTypeStore.Update` |
| 5. 删除 | `EventTypeHandler.Delete` | `EventTypeService.Delete` | `EventTypeStore.SoftDelete` |
| 6. 唯一性 | `EventTypeHandler.CheckName` | `EventTypeService.CheckName` | `EventTypeStore.ExistsByName` |
| 7. 启/停 | `EventTypeHandler.ToggleEnabled` | `EventTypeService.ToggleEnabled` | `EventTypeStore.ToggleEnabled` |
| 8. schema 列表 | `EventTypeSchemaHandler.List` | `EventTypeSchemaService.List` | `EventTypeSchemaStore.List` |
| 9. schema 新建 | `EventTypeSchemaHandler.Create` | `EventTypeSchemaService.Create` | `EventTypeSchemaStore.Create` |
| 10. schema 编辑 | `EventTypeSchemaHandler.Update` | `EventTypeSchemaService.Update` | `EventTypeSchemaStore.Update` |
| 11. schema 启停删 | `EventTypeSchemaHandler.{ToggleEnabled,Delete}` | `EventTypeSchemaService.{ToggleEnabled,Delete}` | `EventTypeSchemaStore.{ToggleEnabled,SoftDelete}` |
| 12. 导出 | `ExportHandler.EventTypes` | `EventTypeService.ExportAll` | `EventTypeStore.ExportAll` |

---

## 数据模型（Go struct 骨架）

```go
// model/event_type.go

type EventType struct {
    ID             int64           `db:"id"`
    Name           string          `db:"name"`
    DisplayName    string          `db:"display_name"`
    PerceptionMode string          `db:"perception_mode"`
    ConfigJSON     json.RawMessage `db:"config_json"`
    Enabled        bool            `db:"enabled"`
    Version        int             `db:"version"`
    CreatedAt      time.Time       `db:"created_at"`
    UpdatedAt      time.Time       `db:"updated_at"`
}

type EventTypeListItem struct {
    ID              int64     `json:"id"`
    Name            string    `json:"name"`
    DisplayName     string    `json:"display_name"`
    PerceptionMode  string    `json:"perception_mode"`
    DefaultSeverity int       `json:"default_severity"`  // 从 config_json 抽
    DefaultTTL      float64   `json:"default_ttl"`       // 从 config_json 抽
    Range           float64   `json:"range"`             // 从 config_json 抽
    Enabled         bool      `json:"enabled"`
    CreatedAt       time.Time `json:"created_at"`
}

type EventTypeDetail struct {
    ID              int64                  `json:"id"`
    Name            string                 `json:"name"`
    DisplayName     string                 `json:"display_name"`
    Enabled         bool                   `json:"enabled"`
    Version         int                    `json:"version"`
    Config          map[string]interface{} `json:"config"`            // 整个 config_json 解包
    ExtensionSchema []EventTypeSchemaLite  `json:"extension_schema"`  // 当前启用的扩展字段定义
    CreatedAt       time.Time              `json:"created_at"`
    UpdatedAt       time.Time              `json:"updated_at"`
}
```

```go
// model/event_type_schema.go

type EventTypeSchema struct {
    ID           int64           `db:"id"`
    FieldName    string          `db:"field_name"`
    FieldLabel   string          `db:"field_label"`
    FieldType    string          `db:"field_type"`
    Constraints  json.RawMessage `db:"constraints"`
    DefaultValue json.RawMessage `db:"default_value"`
    SortOrder    int             `db:"sort_order"`
    Enabled      bool            `db:"enabled"`
    Version      int             `db:"version"`
    CreatedAt    time.Time       `db:"created_at"`
    UpdatedAt    time.Time       `db:"updated_at"`
}

type EventTypeSchemaLite struct {
    FieldName    string          `json:"field_name"`
    FieldLabel   string          `json:"field_label"`
    FieldType    string          `json:"field_type"`
    Constraints  json.RawMessage `json:"constraints"`
    DefaultValue json.RawMessage `json:"default_value"`
    SortOrder    int             `json:"sort_order"`
}
```
