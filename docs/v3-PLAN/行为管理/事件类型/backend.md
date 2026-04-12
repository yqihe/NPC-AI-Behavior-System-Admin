# 事件类型管理 — 后端设计

> 通用架构/规范见 `docs/architecture/overview.md` 和 `docs/development/`。
> 本文档只记录事件类型管理模块的实现事实与特有约束。

---

## 1. 目录结构

```
backend/
  internal/
    handler/
      event_type.go                   # 事件类型 CRUD + 列表 + 详情 + toggle + check-name
      event_type_schema.go            # 扩展字段 Schema CRUD + toggle
    service/
      event_type.go                   # 事件类型业务逻辑（含扩展字段值校验、config_json 拼装）
      event_type_schema.go            # 扩展字段 Schema 业务逻辑（含约束自洽校验）
      constraint/
        validate.go                   # 公共约束校验工具（ValidateValue + ValidateConstraintsSelf）
    store/
      mysql/
        event_type.go                 # event_types 表 CRUD
        event_type_schema.go          # event_type_schema 表 CRUD
      redis/
        event_type_cache.go           # 事件类型 Redis 缓存（detail + list + 分布式锁）
        keys.go                       # key 前缀 & 构造函数（event_types:detail/list/lock）
    cache/
      event_type_schema_cache.go      # 扩展字段 Schema 内存缓存（启动 Load + 写后 Reload）
    model/
      event_type.go                   # EventType / EventTypeListItem / EventTypeListData / EventTypeDetail / EventTypeExportItem / 请求体
      event_type_schema.go            # EventTypeSchema / EventTypeSchemaLite / 请求体
    errcode/
      codes.go                        # 42001-42031 错误码
    router/
      router.go                       # /api/v1/event-types/* + /api/v1/event-type-schema/* + /api/configs/event_types
  migrations/
    004_create_event_types.sql
    005_create_event_type_schema.sql
```

`service/constraint/validate.go` 是从字段管理抽出的公共约束校验工具，字段管理和事件类型扩展字段共用。支持的字段类型：`int` / `integer` / `float` / `string` / `bool` / `select`。

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
- `config_json` 是系统字段（`display_name` / `default_severity` / `default_ttl` / `perception_mode` / `range`）与扩展字段值的合并 JSON，导出 API 直接原样输出，不经过 Go struct 中转。
- `uk_name` 不含 `deleted` 列：软删后 name 永久不可复用。`ExistsByName` 查询不带 `deleted` 过滤。
- `enabled` 默认 0：创建后给"配置窗口期"，编辑/删除要求先停用。

### event_type_schema

```sql
CREATE TABLE IF NOT EXISTS event_type_schema (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    field_name      VARCHAR(64)  NOT NULL,              -- 扩展字段 key，^[a-z][a-z0-9_]*$
    field_label     VARCHAR(128) NOT NULL,              -- 中文名
    field_type      VARCHAR(16)  NOT NULL,              -- int / float / string / bool / select
    constraints     JSON         NOT NULL,              -- 按 type 的约束（min/max/options 等）
    default_value   JSON         NOT NULL,              -- 前端表单初始值
    sort_order      INT          NOT NULL DEFAULT 0,    -- 表单展示顺序

    enabled         TINYINT(1)   NOT NULL DEFAULT 1,    -- 默认启用（与事件类型的 enabled=0 相反）
    version         INT          NOT NULL DEFAULT 1,    -- 乐观锁
    created_at      DATETIME     NOT NULL,
    updated_at      DATETIME     NOT NULL,
    deleted         TINYINT(1)   NOT NULL DEFAULT 0,    -- 软删除

    UNIQUE KEY uk_field_name (field_name)               -- 不含 deleted：软删后 field_name 不可复用
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**设计要点：**
- 独立主键，不复用字段管理的 `fields.id`。`field_name` 命名空间也独立。
- 数据量极小（< 100 条），不建复杂索引。
- `default_value` 类型为 JSON：允许 int / bool / string / array 不同类型的默认值统一存储。
- 不支持 `reference` 类型。

---

## 3. API 接口

### 事件类型管理（7 个接口）

| 方法 | 路径 | Handler | 说明 |
|------|------|---------|------|
| POST | `/api/v1/event-types/list` | `EventTypeHandler.List` | 分页列表，支持 label 模糊搜索 + perception_mode 精确筛选 + enabled 筛选 |
| POST | `/api/v1/event-types/create` | `EventTypeHandler.Create` | 创建事件类型，校验 name/displayName/perceptionMode/severity/ttl/range + 扩展字段值 |
| POST | `/api/v1/event-types/detail` | `EventTypeHandler.Get` | 详情，返回 config_json 展开 + 当前启用的 extension_schema |
| POST | `/api/v1/event-types/update` | `EventTypeHandler.Update` | 编辑（必须先停用），乐观锁，name 不可变。返回 `"保存成功"` |
| POST | `/api/v1/event-types/delete` | `EventTypeHandler.Delete` | 软删除（必须先停用）。返回 `{id, name, label}` |
| POST | `/api/v1/event-types/check-name` | `EventTypeHandler.CheckName` | name 完整格式校验（正则+长度）+ 唯一性校验 |
| POST | `/api/v1/event-types/toggle-enabled` | `EventTypeHandler.ToggleEnabled` | 启用/停用切换（调用方指定目标状态 `enabled`），乐观锁。返回 `"操作成功"` |

**系统字段校验规则（Handler 层）：**
- `name`：非空 + `^[a-z][a-z0-9_]*$` + 长度 <= NameMaxLength
- `display_name`：非空 + 字符数 <= DisplayNameMaxLength
- `perception_mode`：必须是 `visual` / `auditory` / `global` 之一
- `default_severity`：0-100
- `default_ttl`：> 0
- `range`：>= 0；`global` 模式后端强制置 0

**Handler 层校验（与 Field/Template 一致模式）：**
- 统一使用共享 `checkID()` / `checkVersion()`（定义在 field.go，错误消息中文：`"ID 不合法"` / `"版本号不合法"`）
- slog Debug 日志在校验**之后**打印，格式为中文点分（如 `"handler.创建事件类型"`），与 Field/Template 一致
- CheckName 调用完整 `h.checkName()` 做正则 + 长度校验，不仅是空值检查

**业务规则（Service 层）：**
- Create：name 唯一性检查（含软删除）→ 扩展字段值校验 → 拼 config_json → store.Create(req, configJSON) → 清列表缓存
- Update：查存在性 → 必须已停用 → 扩展字段值校验 → 拼 config_json → store.Update(req, configJSON) 乐观锁 → 清缓存
- Delete：查存在性 → 必须已停用 → 软删除 → 清缓存。返回 `*model.DeleteResult{ID, Name, Label(=DisplayName)}`
- ToggleEnabled：接收 `*model.ToggleEnabledRequest`（调用方指定目标 `enabled` 状态，幂等安全），与 Field/Template 一致
- GetByID：Cache-Aside + 分布式锁防击穿 + 空标记防穿透。缓存错误处理使用 `err == nil && hit` 模式（Redis 错误降级直查 MySQL），与 Field/Template 一致
- CheckName：成功时返回 `{available: true, message: "该标识可用"}`，与 Field/Template 一致
- 所有 store 错误统一 `slog.Error` + `fmt.Errorf("xxx: %w", err)` 包装，与 Field/Template 一致
- 扩展字段值校验（`validateExtensions`）：遍历 extensions map，按 field_name 查内存缓存拿 schema，调 `constraint.ValidateValue` 校验

**Store 参数风格（与 Field/Template 一致）：**
- `Create(ctx, *model.CreateEventTypeRequest, configJSON)` — 用请求结构体，不用展开的位置参数
- `Update(ctx, *model.UpdateEventTypeRequest, configJSON)` — 同上

### 扩展字段 Schema 管理（5 个接口）

| 方法 | 路径 | Handler | 说明 |
|------|------|---------|------|
| POST | `/api/v1/event-type-schema/list` | `EventTypeSchemaHandler.List` | 列表（可按 enabled 筛选，sort_order ASC, id ASC） |
| POST | `/api/v1/event-type-schema/create` | `EventTypeSchemaHandler.Create` | 创建扩展字段定义 |
| POST | `/api/v1/event-type-schema/update` | `EventTypeSchemaHandler.Update` | 编辑（field_name / field_type 不可变），乐观锁 |
| POST | `/api/v1/event-type-schema/delete` | `EventTypeSchemaHandler.Delete` | 软删除（必须先停用） |
| POST | `/api/v1/event-type-schema/toggle-enabled` | `EventTypeSchemaHandler.ToggleEnabled` | 启用/停用切换，乐观锁 |

**业务规则（Service 层）：**
- Create：field_name 唯一性（含软删除）→ field_type 枚举校验 → `constraint.ValidateConstraintsSelf` → `constraint.ValidateValue(default_value)` → 数量上限检查 → 写 MySQL → Reload 内存缓存
- Update：查存在性 → 约束自洽校验 → default_value 符合新约束 → 乐观锁更新 → Reload 内存缓存
- Delete：查存在性 → 必须已停用 → 软删除 → Reload 内存缓存

### 导出接口

| 方法 | 路径 | Handler | 说明 |
|------|------|---------|------|
| GET | `/api/configs/event_types` | `ExportHandler.EventTypes` | 导出所有已启用事件类型，`{items: [{name, config}]}` |

导出查询：`SELECT name, config_json AS config FROM event_types WHERE deleted = 0 AND enabled = 1 ORDER BY id`，config_json 原样输出。

---

## 4. 缓存策略

### event_types 缓存（Redis）

| Key 模式 | 含义 | TTL |
|----------|------|-----|
| `event_types:detail:{id}` | 单条详情（含空标记防穿透） | 5min + 30s jitter |
| `event_types:list:v{ver}:{label}:{mode}:{enabled}:{page}:{pageSize}` | 列表分页缓存（带版本号） | 1min + 10s jitter |
| `event_types:list:version` | 列表缓存版本号（INCR 使旧 key 自然过期） | 永久 |
| `event_types:lock:{id}` | 分布式锁（SETNX 防缓存击穿） | 3s（可配置） |

**失效规则：**
- 单条写操作（Create / Update / Delete / ToggleEnabled）：`DEL event_types:detail:{id}` + `INCR event_types:list:version`
- 列表失效采用版本号递增方式，禁止 SCAN+DEL

**详情读取流程（Cache-Aside + 分布式锁 + 空标记，与 Field/Template 完全一致）：**
1. 查 Redis 缓存：`err == nil && hit` 才使用缓存结果（Redis 错误降级直查 MySQL）
2. 未命中 → SETNX 获取分布式锁（锁失败 `slog.Warn` 记录后继续）→ double-check 缓存
3. 查 MySQL → 写缓存（nil 写空标记防穿透）

### event_type_schema 缓存（内存）

**采用内存缓存，不走 Redis。** 原因：数据量极小（< 100 条）、命中频率极高（每次事件类型 CRUD 都要读）、写频率极低（运营月级别偶尔改一次）。

实现类 `cache.EventTypeSchemaCache`，与 `DictCache` 同构：
- **启动时** `Load(ctx)` 全量拉 `event_type_schema WHERE deleted=0 AND enabled=1`，按 sort_order ASC 排序
- **写后同步** `Reload(ctx)`：Create / Update / Delete / ToggleEnabled 完成后立即调用
- **运行时只读**：`ListEnabled()` 返回副本，`GetByFieldName(name)` 按 name 查找

**多实例一致性**：本期单实例，不处理。长期方案：Redis Pub/Sub 广播 reload 信号。

---

## 5. 错误码

### 事件类型管理（42001-42015）

| 错误码 | 常量 | 触发场景 |
|--------|------|----------|
| 42001 | `ErrEventTypeNameExists` | 创建时 name 已存在（含软删除） |
| 42002 | `ErrEventTypeNameInvalid` | name 为空 / 不匹配 `^[a-z][a-z0-9_]*$` / 超长 |
| 42003 | `ErrEventTypeModeInvalid` | perception_mode 不是 visual/auditory/global |
| 42004 | `ErrEventTypeSeverityInvalid` | default_severity 不在 0-100 范围 |
| 42005 | `ErrEventTypeTTLInvalid` | default_ttl <= 0 |
| 42006 | `ErrEventTypeRangeInvalid` | range < 0 |
| 42007 | `ErrEventTypeExtValueInvalid` | 扩展字段值不符合 schema 约束（key 不存在 / 值校验失败） |
| 42008 | `ErrEventTypeRefDelete` | 被 FSM/BT 引用无法删除（占位，本期 ref_count 恒 0） |
| 42010 | `ErrEventTypeVersionConflict` | 乐观锁 version 不匹配（编辑 / toggle） |
| 42011 | `ErrEventTypeNotFound` | ID 对应记录不存在或已软删 |
| 42012 | `ErrEventTypeDeleteNotDisabled` | 删除时 enabled=1，必须先停用 |
| 42015 | `ErrEventTypeEditNotDisabled` | 编辑时 enabled=1，必须先停用 |

### 扩展字段 Schema（42020-42031）

| 错误码 | 常量 | 触发场景 |
|--------|------|----------|
| 42020 | `ErrExtSchemaNameExists` | field_name 已存在（含软删除） |
| 42021 | `ErrExtSchemaNameInvalid` | field_name 为空 / 不匹配标识格式 / 超长 |
| 42022 | `ErrExtSchemaNotFound` | 扩展字段定义 ID 不存在或已软删 |
| 42023 | `ErrExtSchemaDisabled` | 扩展字段已停用，不能被事件类型引用 |
| 42024 | `ErrExtSchemaTypeInvalid` | field_type 不是 int/float/string/bool/select |
| 42025 | `ErrExtSchemaConstraintsInvalid` | constraints 不自洽（如 min > max / minLength > maxLength / minSelect > maxSelect） |
| 42026 | `ErrExtSchemaDefaultInvalid` | default_value 不符合 constraints 约束 |
| 42027 | `ErrExtSchemaDeleteNotDisabled` | 删除时 enabled=1，必须先停用 |
| 42030 | `ErrExtSchemaVersionConflict` | 乐观锁 version 不匹配（编辑 / toggle） |
| 42031 | `ErrExtSchemaEditNotDisabled` | 编辑前必须先停用（占位，当前 Update 未检查 enabled） |
