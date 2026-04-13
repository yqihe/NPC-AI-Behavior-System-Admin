# 事件类型管理 — 后端设计

> 通用架构/规范见 `docs/architecture/overview.md` 和 `docs/development/`。
> 本文档只记录事件类型管理模块的实现事实与特有约束。

---

## 1. 目录结构

```
backend/
  internal/
    handler/
      event_type.go                   # 事件类型 CRUD + 列表 + 详情 + toggle + check-name（7 个接口）
    service/
      event_type.go                   # 事件类型业务逻辑（含事务编排、schema_refs 维护、扩展字段值校验、config_json 拼装）
    store/
      mysql/
        event_type.go                 # event_types 表 CRUD（含 CreateTx/UpdateTx/SoftDeleteTx 事务版）
        schema_ref.go                 # schema_refs 表操作（Add/Remove/RemoveByRef/HasRefs/HasRefsTx/GetBySchemaID）
      redis/
        event_type_cache.go           # 事件类型 Redis 缓存（detail + list + 分布式锁）
        config/                       # Redis 缓存共享配置子包
          common.go                   # 共享常量（DetailTTLBase/ListTTLBase/LockExpire/NullMarker）+ TTL() / Available()
          keys.go                     # key 构造函数（EventTypeDetailKey / EventTypeListKey / EventTypeLockKey + EventTypeListVersionKey）
    cache/
      event_type_schema_cache.go      # 扩展字段 Schema 内存缓存（启动 Load + 写后 Reload）
    model/
      event_type.go                   # EventType / EventTypeListItem / EventTypeListData / EventTypeDetail / EventTypeExportItem / 请求体
      event_type_schema.go            # SchemaRef / SchemaReferenceItem / SchemaReferenceDetail
    errcode/
      codes.go                        # 42001-42015 错误码
      store_errors.go                 # Store 层哨兵错误（ErrNotFound / ErrVersionConflict / ErrDuplicate）
    util/
      constraint.go                   # 公共约束校验工具（ValidateValue / ValidateConstraintsSelf / CheckConstraintTightened）
      const.go                        # 感知模式常量 + 扩展字段类型常量 + RefTypeEventType
    router/
      router.go                       # /api/v1/event-types/* 路由注册
  migrations/
    004_create_event_types.sql
    007_create_schema_refs.sql
```

**注意**：约束校验工具已从原 `service/constraint/` 子目录迁移到 `util/constraint.go`（package `util`），被字段管理和事件类型扩展字段共用。支持的字段类型：`int` / `integer` / `float` / `string` / `bool` / `select`。

---

## 2. 数据表

### event_types

```sql
CREATE TABLE IF NOT EXISTS event_types (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    name            VARCHAR(64)  NOT NULL,              -- 事件标识，唯一，创建后不可变
    display_name    VARCHAR(128) NOT NULL,              -- 中文名（搜索用）
    perception_mode VARCHAR(16)  NOT NULL,              -- 感知模式：visual / auditory / global
    config_json     JSON         NOT NULL,              -- 系统字段 + 扩展字段的完整合并，导出 API 原样输出

    enabled         TINYINT(1)   NOT NULL DEFAULT 0,    -- 创建默认停用
    version         INT          NOT NULL DEFAULT 1,    -- 乐观锁
    created_at      DATETIME     NOT NULL,
    updated_at      DATETIME     NOT NULL,
    deleted         TINYINT(1)   NOT NULL DEFAULT 0,    -- 软删除

    UNIQUE KEY uk_name (name),                          -- 不含 deleted：软删后 name 不可复用
    INDEX idx_list (deleted, enabled, id DESC),          -- 列表分页覆盖索引
    INDEX idx_perception (deleted, perception_mode)      -- facet 筛选索引
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**设计要点：**
- `config_json` 是系统字段（`display_name` / `default_severity` / `default_ttl` / `perception_mode` / `range`）与扩展字段值的合并 JSON，导出 API 直接原样输出。
- `uk_name` 不含 `deleted` 列：软删后 name 永久不可复用。`ExistsByName` 查询不带 `deleted` 过滤。
- `enabled` 默认 0：创建后给"配置窗口期"，编辑/删除要求先停用。

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
- 结构与 `field_refs` 对齐：被引用方 ID + 引用来源类型 + 引用方 ID。
- 联合主键 `(schema_id, ref_type, ref_id)` 保证引用关系唯一，写入用 `INSERT IGNORE`。
- `idx_ref (ref_type, ref_id)` 索引支持按引用方反查（事件类型删除时用 `RemoveByRef`）。
- `ref_type` 当前固定为 `"event_type"`（`util.RefTypeEventType`），预留未来其他模块引用扩展字段。

---

## 3. API 接口

### 事件类型管理（7 个接口）

| 方法 | 路径 | Handler | 说明 |
|------|------|---------|------|
| POST | `/api/v1/event-types/list` | `EventTypeHandler.List` | 分页列表，支持 label 模糊搜索 + perception_mode 精确筛选 + enabled 筛选 |
| POST | `/api/v1/event-types/create` | `EventTypeHandler.Create` | 创建事件类型（含 schema_refs 事务） |
| POST | `/api/v1/event-types/detail` | `EventTypeHandler.Get` | 详情，返回 config_json 展开 + 当前启用的 extension_schema |
| POST | `/api/v1/event-types/update` | `EventTypeHandler.Update` | 编辑（必须先停用），乐观锁，name 不可变 |
| POST | `/api/v1/event-types/delete` | `EventTypeHandler.Delete` | 软删除（必须先停用），返回 `{id, name, label}` |
| POST | `/api/v1/event-types/check-name` | `EventTypeHandler.CheckName` | name 完整格式校验（正则+长度）+ 唯一性校验 |
| POST | `/api/v1/event-types/toggle-enabled` | `EventTypeHandler.ToggleEnabled` | 启用/停用切换（调用方指定目标状态 `enabled`），乐观锁 |

**Handler 层依赖注入：**

```go
type EventTypeHandler struct {
    eventTypeService       *service.EventTypeService
    eventTypeSchemaService *service.EventTypeSchemaService  // 详情接口需要拉 Schema 列表
    etCfg                  *config.EventTypeConfig
}
```

**系统字段校验规则（Handler 层）：**
- `name`：非空 + `^[a-z][a-z0-9_]*$` + 长度 <= NameMaxLength
- `display_name`：非空 + 字符数 <= DisplayNameMaxLength
- `perception_mode`：必须是 `visual` / `auditory` / `global` 之一
- `default_severity`：0-100
- `default_ttl`：> 0
- `range`：>= 0；`global` 模式后端强制置 0

**Store 参数风格（与 Field/Template 一致）：**
- `Create(ctx, *model.CreateEventTypeRequest, configJSON)` — 用请求结构体
- `CreateTx(ctx, tx, *model.CreateEventTypeRequest, configJSON)` — 事务版
- `Update(ctx, *model.UpdateEventTypeRequest, configJSON)` — 用请求结构体
- `UpdateTx(ctx, tx, *model.UpdateEventTypeRequest, configJSON)` — 事务版

### 导出接口

| 方法 | 路径 | Handler | 说明 |
|------|------|---------|------|
| GET | `/api/configs/event_types` | `ExportHandler.EventTypes` | 导出所有已启用事件类型 |

查询：`SELECT name, config_json AS config FROM event_types WHERE deleted = 0 AND enabled = 1 ORDER BY id`，config_json 原样输出。

---

## 4. 事务编排（Create/Update/Delete with schema_refs）

EventTypeService 持有 `schemaRefStore *storemysql.SchemaRefStore`，在 CRUD 时通过事务维护 schema_refs 引用关系。

### 4.1 Create 事务

```
1. name 唯一性检查（含软删除）
2. 扩展字段值校验（validateExtensions → schemaCache + util.ValidateValue）
3. global 模式 range=0 兜底
4. 拼 config_json（buildConfigJSON）
5. BEGIN TX
   5a. store.CreateTx(ctx, tx, req, configJSON) → 得到 id
   5b. attachSchemaRefs(ctx, tx, id, extensions)
       遍历 extensions keys → schemaCache.GetByFieldName → schemaRefStore.Add(tx, schemaID, 'event_type', id)
6. COMMIT
7. cache.InvalidateList
```

### 4.2 Update 事务

```
1. getOrNotFound(id)
2. 检查 enabled=0（否则 42015）
3. 扩展字段值校验
4. global 模式 range=0 兜底
5. 拼 config_json
6. 解析旧 config_json 的扩展字段 key（extractExtensionKeys：排除 5 个系统字段 key）
7. 构建新扩展字段 key 集合
8. BEGIN TX
   8a. store.UpdateTx(ctx, tx, req, configJSON) — 乐观锁
   8b. syncSchemaRefs(ctx, tx, id, oldKeys, newKeys)
       toAdd: newKeys 中有但 oldKeys 没有 → schemaRefStore.Add
       toRemove: oldKeys 中有但 newKeys 没有 → 查 schema_id → schemaRefStore.Remove
9. COMMIT
10. cache.DelDetail + cache.InvalidateList
```

### 4.3 Delete 事务

```
1. getOrNotFound(id)
2. 检查 enabled=0（否则 42012）
3. BEGIN TX
   3a. store.SoftDeleteTx(ctx, tx, id)
   3b. schemaRefStore.RemoveByRef(ctx, tx, 'event_type', id) — 清理所有引用
4. COMMIT
5. cache.DelDetail + cache.InvalidateList
```

---

## 5. 缓存策略

### event_types 缓存（Redis）

| Key 模式 | 含义 | TTL |
|----------|------|-----|
| `event_types:detail:{id}` | 单条详情（含空标记防穿透） | 5min + 30s jitter |
| `event_types:list:v{ver}:{label}:{mode}:{enabled}:{page}:{pageSize}` | 列表分页缓存（带版本号） | 1min + 10s jitter |
| `event_types:list:version` | 列表缓存版本号（INCR 使旧 key 自然过期） | 永久 |
| `event_types:lock:{id}` | 分布式锁（SETNX 防缓存击穿） | 3s（可配置 `etCfg.CacheLockTTL`） |

**失效规则：**
- 单条写操作（Update / Delete / ToggleEnabled）：`DEL event_types:detail:{id}` + `INCR event_types:list:version`
- Create：仅 `INCR event_types:list:version`（新记录无 detail 缓存）
- 列表失效采用版本号递增方式，禁止 SCAN+DEL

**详情读取流程（Cache-Aside + 分布式锁 + 空标记，与 Field/Template 完全一致）：**
1. 查 Redis 缓存：`err == nil && hit` 才使用缓存结果（Redis 错误降级直查 MySQL）
2. 未命中 → SETNX 获取分布式锁（锁失败 `slog.Warn` 记录后继续）→ double-check 缓存
3. 查 MySQL → 写缓存（nil 写空标记防穿透）

### event_type_schema 缓存（内存）

采用内存缓存，不走 Redis。原因：数据量极小（< 100 条）、命中频率极高（每次事件类型 CRUD 都要读）、写频率极低（运营月级别偶尔改一次）。

实现类 `cache.EventTypeSchemaCache`，与 `DictCache` 同构：
- **启动时** `Load(ctx)` 全量拉 `event_type_schema WHERE deleted=0 AND enabled=1`，按 sort_order ASC 排序
- **写后同步** `Reload(ctx)`：Create / Update / Delete / ToggleEnabled 完成后立即调用
- **运行时只读**：`ListEnabled()` 返回副本，`GetByFieldName(name)` 按 name 查找

**多实例一致性**：本期单实例，不处理。长期方案：Redis Pub/Sub 广播 reload 信号。

---

## 6. 错误码

### 事件类型管理（42001-42015）

| 错误码 | 常量 | 消息 | 触发场景 |
|--------|------|------|----------|
| 42001 | `ErrEventTypeNameExists` | 事件标识已存在 | 创建时 name 已存在（含软删除） |
| 42002 | `ErrEventTypeNameInvalid` | 事件标识格式不合法，需小写字母开头，仅允许 a-z、0-9、下划线 | name 为空 / 不匹配正则 / 超长 |
| 42003 | `ErrEventTypeModeInvalid` | 感知模式必须是 visual / auditory / global 之一 | perception_mode 枚举非法 |
| 42004 | `ErrEventTypeSeverityInvalid` | 默认威胁必须在 0-100 之间 | default_severity 不在 0-100 范围 |
| 42005 | `ErrEventTypeTTLInvalid` | 默认 TTL 必须大于 0 | default_ttl <= 0 |
| 42006 | `ErrEventTypeRangeInvalid` | 传播范围不能小于 0 | range < 0 |
| 42007 | `ErrEventTypeExtValueInvalid` | 扩展字段的值不符合约束 | 扩展字段 key 不存在或值校验失败 |
| 42008 | `ErrEventTypeRefDelete` | 当前事件类型仍被引用，不能删除 | 被 FSM/BT 引用无法删除（占位，FSM/BT 上线后接入） |
| 42010 | `ErrEventTypeVersionConflict` | 该事件类型已被其他人修改，请刷新后重试 | 乐观锁 version 不匹配（编辑 / toggle） |
| 42011 | `ErrEventTypeNotFound` | 事件类型不存在 | ID 对应记录不存在或已软删 |
| 42012 | `ErrEventTypeDeleteNotDisabled` | 请先停用该事件类型再删除 | 删除时 enabled=1 |
| 42015 | `ErrEventTypeEditNotDisabled` | 请先停用该事件类型再编辑 | 编辑时 enabled=1 |

---

## 7. Service 层核心方法

### EventTypeService

```go
type EventTypeService struct {
    store          *storemysql.EventTypeStore
    schemaRefStore *storemysql.SchemaRefStore     // schema_refs 维护
    cache          *storeredis.EventTypeCache
    schemaCache    *cache.EventTypeSchemaCache    // 扩展字段 Schema 内存缓存
    pagCfg         *config.PaginationConfig
    etCfg          *config.EventTypeConfig
}
```

| 方法 | 签名 | 说明 |
|------|------|------|
| `List` | `(ctx, *EventTypeListQuery) (*ListData, error)` | 分页列表，Redis 缓存 |
| `Create` | `(ctx, *CreateEventTypeRequest) (int64, error)` | 创建 + 事务写 schema_refs |
| `GetByID` | `(ctx, id) (*EventType, error)` | Cache-Aside + 锁 + 空标记 |
| `Update` | `(ctx, *UpdateEventTypeRequest) error` | 编辑 + 事务 diff schema_refs |
| `Delete` | `(ctx, id) (*DeleteResult, error)` | 软删 + 事务清理 schema_refs |
| `CheckName` | `(ctx, name) (*CheckNameResult, error)` | 唯一性校验 |
| `ToggleEnabled` | `(ctx, *ToggleEnabledRequest) error` | 启用/停用 |
| `ExportAll` | `(ctx) ([]EventTypeExportItem, error)` | 导出 |
| `validateExtensions` | 私有 | 遍历 extensions → schemaCache.GetByFieldName → util.ValidateValue |
| `buildConfigJSON` | 私有 | 合并系统字段 + 扩展字段为 JSON |
| `extractExtensionKeys` | 私有 | 从 config_json 提取非系统字段 key 集合 |
| `attachSchemaRefs` | 私有 | 为新建事件类型写 schema_refs（事务内） |
| `syncSchemaRefs` | 私有 | diff 旧/新扩展字段 key，增删 schema_refs（事务内） |

### EventTypeStore 事务版方法

| 方法 | 说明 |
|------|------|
| `CreateTx(ctx, tx, req, configJSON) (int64, error)` | 事务内 INSERT |
| `UpdateTx(ctx, tx, req, configJSON) error` | 事务内 UPDATE + 乐观锁 |
| `SoftDeleteTx(ctx, tx, id) error` | 事务内软删除 |
| `DB() *sqlx.DB` | 暴露数据库连接，service 层开事务用 |

非事务版方法（`Create` / `Update` / `SoftDelete`）保留，供不需要跨表事务的场景使用。
