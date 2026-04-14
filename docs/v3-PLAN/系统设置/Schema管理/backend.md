# 事件扩展字段 Schema 管理 — 后端设计

> **实现状态**：已完成。代码位于事件类型模块内。

---

## 1. 目录结构

```
backend/internal/
├── handler/event_type_schema.go       # HTTP handler（6 个接口，含 references）
├── service/event_type_schema.go       # 业务逻辑（含引用保护、约束校验）
├── store/mysql/event_type_schema.go   # event_type_schema 表 CRUD
├── store/mysql/schema_ref.go          # schema_refs 表操作（Add/Remove/RemoveByRef/HasRefs/HasRefsTx/GetBySchemaID）
├── cache/event_type_schema_cache.go   # 内存缓存（启用的 Schema）
├── model/event_type_schema.go         # 数据模型（EventTypeSchema/EventTypeSchemaLite/SchemaRef/SchemaReferenceItem/SchemaReferenceDetail）
└── util/constraint.go                 # 约束校验工具（ValidateValue/ValidateConstraintsSelf/CheckConstraintTightened）— 与字段管理共用
```

**注意**：约束校验工具已从原 `service/constraint/` 子目录迁移到 `util/constraint.go`（package `util`）。

---

## 2. 数据表

### event_type_schema

```sql
CREATE TABLE IF NOT EXISTS event_type_schema (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    field_name      VARCHAR(64)  NOT NULL,              -- 扩展字段 key，符合 ^[a-z][a-z0-9_]*$
    field_label     VARCHAR(128) NOT NULL,              -- 中文名
    field_type      VARCHAR(16)  NOT NULL,              -- int / float / string / bool / select（不支持 reference）
    constraints     JSON         NOT NULL,              -- 按 type 的约束（min/max/pattern/options 等）
    default_value   JSON         NOT NULL,              -- 前端表单初始值提示（不回填历史数据）
    sort_order      INT          NOT NULL DEFAULT 0,    -- 表单展示顺序

    enabled         TINYINT(1)   NOT NULL DEFAULT 1,    -- 默认启用（和事件类型的 enabled=0 不同）
    version         INT          NOT NULL DEFAULT 1,    -- 乐观锁
    created_at      DATETIME     NOT NULL,
    updated_at      DATETIME     NOT NULL,
    deleted         TINYINT(1)   NOT NULL DEFAULT 0,    -- 软删除

    -- field_name 唯一约束不含 deleted：软删后 field_name 仍占唯一性
    UNIQUE KEY uk_field_name (field_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**设计要点：**
- 独立主键，不复用字段管理的 `fields.id`。`field_name` 命名空间也独立。
- 数据量极小（< 100 条），不建复杂索引。
- `default_value` 类型为 JSON：允许 int / bool / string / array 不同类型的默认值统一存储。
- 不支持 `reference` 类型。

### schema_refs

```sql
CREATE TABLE IF NOT EXISTS schema_refs (
    schema_id   BIGINT       NOT NULL,              -- 被引用的扩展字段定义 ID
    ref_type    VARCHAR(16)  NOT NULL,              -- 引用来源：'event_type'
    ref_id      BIGINT       NOT NULL,              -- 引用方 ID（事件类型 ID）

    PRIMARY KEY (schema_id, ref_type, ref_id),
    INDEX idx_ref (ref_type, ref_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**设计要点：**
- 结构与 `field_refs` 对齐。联合主键保证引用关系唯一，写入用 `INSERT IGNORE`。
- `idx_ref (ref_type, ref_id)` 支持按引用方反查（事件类型删除时批量清理）。
- `ref_type` 当前固定为 `"event_type"`（`util.RefTypeEventType`）。
- 该表由事件类型 Service 维护（Create/Update/Delete 事务），Schema 模块只读查询。

---

## 3. API 接口（6 个）

| 方法 | 路径 | Handler | 说明 |
|------|------|---------|------|
| POST | `/api/v1/event-type-schema/list` | `EventTypeSchemaHandler.List` | 列表（可按 enabled 过滤，含 has_refs 填充） |
| POST | `/api/v1/event-type-schema/create` | `EventTypeSchemaHandler.Create` | 创建 |
| POST | `/api/v1/event-type-schema/update` | `EventTypeSchemaHandler.Update` | 编辑（field_name/field_type 不可变，引用保护 + 乐观锁） |
| POST | `/api/v1/event-type-schema/delete` | `EventTypeSchemaHandler.Delete` | 软删除（须先停用 + 引用保护） |
| POST | `/api/v1/event-type-schema/toggle-enabled` | `EventTypeSchemaHandler.ToggleEnabled` | 启用/禁用切换（乐观锁） |
| POST | `/api/v1/event-type-schema/references` | `EventTypeSchemaHandler.GetReferences` | 引用详情（跨模块补 display_name） |

**Handler 层依赖注入：**

```go
type EventTypeSchemaHandler struct {
    schemaService    *service.EventTypeSchemaService
    eventTypeService *service.EventTypeService       // references 接口跨模块补齐 display_name
    etsCfg           *config.EventTypeSchemaConfig
}
```

**Handler 层校验：**
- `field_name`：非空 + `util.IdentPattern` 正则 + 长度 <= FieldNameMaxLength
- `field_label`：非空 + 字符数 <= FieldLabelMaxLength
- `field_type`：必须在 `util.ValidExtFieldTypes` 枚举中
- `constraints`：必须是 JSON 对象（`checkJSONObjectShape`）
- `default_value`：不能为空

---

## 4. 引用保护逻辑

### 4.1 列表 has_refs 填充

`EventTypeSchemaHandler.List` 调用 `schemaService.FillHasRefs(ctx, items)`，为每条记录查询 `schema_refs` 填充 `has_refs` 布尔值。前端据此展示引用状态标记。

### 4.2 Update 引用保护 — 约束收紧检查

```
1. getOrNotFound(id)
2. schemaRefStore.HasRefs(ctx, id) — 非事务查询
3. 如果 hasRefs=true:
   util.CheckConstraintTightened(ets.FieldType, ets.Constraints, req.Constraints, errcode.ErrExtSchemaRefTighten)
   → 收紧则返回 42028 拒绝
4. util.ValidateConstraintsSelf(ets.FieldType, req.Constraints, errcode.ErrExtSchemaConstraintsInvalid) — 约束自洽校验
5. util.ValidateValue(ets.FieldType, req.Constraints, req.DefaultValue) — 默认值校验
6. store.Update — 乐观锁
7. schemaCache.Reload
```

**约束自洽校验规则**（`util.ValidateConstraintsSelf`）：

| 字段类型 | 校验项 |
|----------|--------|
| int/integer | `min <= max` |
| float | `min <= max`, `precision > 0` |
| string | `minLength <= maxLength`, `minLength >= 0`, `maxLength >= 0` |
| select | `options` 非空, `value` 不重复, `minSelect <= maxSelect`, `minSelect >= 0` |
| bool/boolean | 无约束 |

**约束收紧检查规则**（`util.CheckConstraintTightened`）：

| 字段类型 | 收紧判定 |
|----------|----------|
| int/float | 新 min > 旧 min 或 新 max < 旧 max；float 额外检查 precision 不能降低 |
| string | 新 minLength > 旧 minLength 或 新 maxLength < 旧 maxLength；pattern 不能变更 |
| select | 旧 options 中的值被删除；minSelect 增大；maxSelect 减小 |
| bool | 无约束，不检查 |

### 4.3 Delete 引用保护

```
1. getOrNotFound(id)
2. ets.Enabled == true → 42027 拒绝
3. schemaRefStore.HasRefs(ctx, id) — 非事务查询
4. hasRefs=true → 42029 拒绝
5. store.SoftDelete
6. schemaCache.Reload
```

### 4.4 GetReferences 接口

```
1. getOrNotFound(id)
2. schemaRefStore.GetBySchemaID(ctx, id) — 查所有引用方
3. 过滤 ref_type='event_type'，构建 SchemaReferenceDetail
4. handler 层跨模块补齐：
   遍历 eventTypes → eventTypeService.GetByID(ctx, refID) → 填充 Label=DisplayName
5. 返回 SchemaReferenceDetail
```

**数据模型：**

```go
type SchemaReferenceDetail struct {
    SchemaID   int64                  `json:"schema_id"`
    FieldLabel string                 `json:"field_label"`
    EventTypes []SchemaReferenceItem  `json:"event_types"`
}

type SchemaReferenceItem struct {
    RefType string `json:"ref_type"`   // "event_type"
    RefID   int64  `json:"ref_id"`
    Label   string `json:"label"`      // handler 层跨模块填充 display_name
}
```

---

## 5. Service 层核心方法

### EventTypeSchemaService

```go
type EventTypeSchemaService struct {
    store          *storemysql.EventTypeSchemaStore
    schemaRefStore *storemysql.SchemaRefStore          // 引用关系查询
    schemaCache    *cache.EventTypeSchemaCache
    etsCfg         *config.EventTypeSchemaConfig
}
```

| 方法 | 签名 | 说明 |
|------|------|------|
| `List` | `(ctx, *EventTypeSchemaListQuery) ([]EventTypeSchema, error)` | 列表查询（直查 MySQL，不走 Redis） |
| `ListEnabled` | `() []EventTypeSchemaLite` | 返回所有启用的（内存缓存） |
| `ListAllLite` | `(ctx) ([]EventTypeSchemaLite, error)` | 返回所有未删除的（含禁用，给详情页合并用） |
| `Create` | `(ctx, *CreateEventTypeSchemaRequest) (int64, error)` | 创建 |
| `Update` | `(ctx, *UpdateEventTypeSchemaRequest) error` | 编辑（含引用保护） |
| `Delete` | `(ctx, id) error` | 软删除（含引用保护） |
| `ToggleEnabled` | `(ctx, id, version) error` | 启用/禁用 |
| `GetReferences` | `(ctx, id) (*SchemaReferenceDetail, error)` | 引用详情 |
| `FillHasRefs` | `(ctx, []EventTypeSchema)` | 为列表填充 has_refs |

### SchemaRefStore

```go
type SchemaRefStore struct {
    db *sqlx.DB
}
```

| 方法 | 签名 | 说明 |
|------|------|------|
| `Add` | `(ctx, tx, schemaID, refType, refID) error` | 事务内添加引用（INSERT IGNORE） |
| `Remove` | `(ctx, tx, schemaID, refType, refID) error` | 事务内移除单条引用 |
| `RemoveByRef` | `(ctx, tx, refType, refID) ([]int64, error)` | 事务内移除引用方的所有引用，返回 schema IDs |
| `HasRefs` | `(ctx, schemaID) (bool, error)` | 非事务检查是否被引用 |
| `HasRefsTx` | `(ctx, tx, schemaID) (bool, error)` | 事务内检查（FOR SHARE 防 TOCTOU） |
| `GetBySchemaID` | `(ctx, schemaID) ([]SchemaRef, error)` | 查询所有引用方（references API） |

---

## 6. 内存缓存策略

实现类 `cache.EventTypeSchemaCache`，与 `DictCache` 同构：

```go
type EventTypeSchemaCache struct {
    mu      sync.RWMutex
    store   *mysql.EventTypeSchemaStore
    schemas []model.EventTypeSchemaLite          // 启用的，按 sort_order 排好序
    byName  map[string]*model.EventTypeSchemaLite // field_name → Schema
}
```

| 方法 | 说明 |
|------|------|
| `Load(ctx)` | 全量拉 `WHERE deleted=0 AND enabled=1 ORDER BY sort_order ASC, id ASC`，启动时调用 |
| `Reload(ctx)` | 写操作后同步调用，重新全量加载 |
| `ListEnabled()` | 返回所有启用 Schema 的副本（sort_order 已排序） |
| `GetByFieldName(name)` | 按 field_name 查找，返回副本。只返回 enabled=1 的，找不到返回 nil, false |

**生命周期：**
- 启动时 `setup.InitMemCaches` 调用 `Load`
- Schema Create / Update / Delete / ToggleEnabled 完成后立即 `Reload`
- 运行时只读，读操作通过 `RWMutex` 保护

**多实例一致性**：本期单实例，不处理。长期方案：Redis Pub/Sub 广播 reload 信号。

---

## 7. 错误码（42020-42031）

| 错误码 | 常量 | 消息 | 触发场景 |
|--------|------|------|----------|
| 42020 | `ErrExtSchemaNameExists` | 扩展字段标识已存在 | field_name 已存在（含软删除） |
| 42021 | `ErrExtSchemaNameInvalid` | 扩展字段标识格式不合法，需小写字母开头，仅允许 a-z、0-9、下划线 | field_name 为空 / 不匹配正则 / 超长 |
| 42022 | `ErrExtSchemaNotFound` | 扩展字段定义不存在 | ID 对应记录不存在或已软删 |
| 42023 | `ErrExtSchemaDisabled` | 扩展字段已停用 | 扩展字段已停用不能被引用（事件类型校验时） |
| 42024 | `ErrExtSchemaTypeInvalid` | 扩展字段类型必须是 int / float / string / bool / select 之一 | field_type 不在枚举范围 |
| 42025 | `ErrExtSchemaConstraintsInvalid` | 约束配置不自洽 | 约束不自洽（如 min > max / minLength > maxLength / minSelect > maxSelect / precision < 0） |
| 42026 | `ErrExtSchemaDefaultInvalid` | 默认值不符合约束 | default_value 不符合 constraints 约束 |
| 42027 | `ErrExtSchemaDeleteNotDisabled` | 请先停用该扩展字段再删除 | 删除时 enabled=1 |
| 42028 | `ErrExtSchemaRefTighten` | 该扩展字段已被事件类型引用，约束只能放宽不能收紧 | 被引用时约束收紧（Update） |
| 42029 | `ErrExtSchemaRefDelete` | 该扩展字段正被事件类型引用，无法删除 | 被引用时删除（Delete） |
| 42030 | `ErrExtSchemaVersionConflict` | 该扩展字段已被其他人修改，请刷新后重试 | 乐观锁 version 不匹配（编辑 / toggle） |
| 42031 | `ErrExtSchemaEditNotDisabled` | 请先停用该扩展字段再编辑 | 编辑前必须先停用（占位） |

---

## 8. 与事件类型模块的数据流

```
事件类型 Create/Update/Delete
  └→ 事务内维护 schema_refs（写入/更新/清理引用关系）

Schema 管理 Update
  └→ schemaRefStore.HasRefs → 有引用？ → CheckConstraintTightened → 收紧则 42028 拒绝

Schema 管理 Delete
  └→ schemaRefStore.HasRefs → 有引用？ → 42029 拒绝

Schema 管理 GetReferences
  └→ schemaRefStore.GetBySchemaID → handler 跨模块调 eventTypeService.GetByID 补 Label

Schema 管理 List
  └→ FillHasRefs → 为每条记录查 schema_refs 填充 has_refs

事件类型 Detail
  └→ schemaCache.ListEnabled + schemaService.ListAllLite → 合并启用+禁用但有值的 Schema
```
