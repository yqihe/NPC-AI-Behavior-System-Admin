# 事件扩展字段 Schema 管理 — 后端设计

> **实现状态**：已完成。代码位于事件类型模块内。

---

## 目录结构

```
backend/internal/
├── handler/event_type_schema.go       # HTTP handler（5 个接口）
├── service/event_type_schema.go       # 业务逻辑 + 约束校验
├── store/mysql/event_type_schema.go   # MySQL CRUD
├── cache/event_type_schema_cache.go   # 内存缓存（启用的 Schema）
├── model/event_type_schema.go         # 数据模型
└── service/constraint/validate.go     # 约束校验（与字段管理共用）
```

## 数据表

```sql
CREATE TABLE event_type_schema (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    field_name      VARCHAR(64)  NOT NULL,
    field_label     VARCHAR(128) NOT NULL,
    field_type      VARCHAR(16)  NOT NULL,  -- int/float/string/bool/select
    constraints     JSON         NOT NULL,
    default_value   JSON         NOT NULL,
    sort_order      INT          NOT NULL DEFAULT 0,
    enabled         TINYINT(1)   NOT NULL DEFAULT 1,
    version         INT          NOT NULL DEFAULT 1,
    created_at      DATETIME     NOT NULL,
    updated_at      DATETIME     NOT NULL,
    deleted         TINYINT(1)   NOT NULL DEFAULT 0,
    UNIQUE KEY uk_field_name (field_name)
);
```

field_name 唯一性含软删除记录。

## API 接口

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/event-type-schema/list` | 列表（可按 enabled 过滤） |
| POST | `/api/v1/event-type-schema/create` | 创建 |
| POST | `/api/v1/event-type-schema/update` | 编辑（field_name/field_type 不可变，乐观锁） |
| POST | `/api/v1/event-type-schema/delete` | 软删除（须先禁用） |
| POST | `/api/v1/event-type-schema/toggle-enabled` | 启用/禁用切换（乐观锁） |

## 缓存策略

- **内存缓存**（非 Redis）：数据量 < 100，启动加载 + 写操作后 Reload
- `ListEnabled()` 返回所有启用 Schema（sort_order ASC）
- `GetByFieldName()` 按 field_name 查找（事件类型校验用）
- `ListAllLite()` 返回所有未删除 Schema（事件类型详情合并用）

## 错误码

| 错误码 | 常量 | 场景 |
|--------|------|------|
| 42020 | ErrExtSchemaNameExists | field_name 已存在（含软删除） |
| 42021 | ErrExtSchemaNameInvalid | field_name 格式不合法 |
| 42022 | ErrExtSchemaNotFound | Schema 不存在 |
| 42023 | ErrExtSchemaDisabled | Schema 已禁用 |
| 42024 | ErrExtSchemaTypeInvalid | field_type 不在枚举范围 |
| 42025 | ErrExtSchemaConstraintsInvalid | 约束不自洽 |
| 42026 | ErrExtSchemaDefaultInvalid | 默认值违反约束 |
| 42027 | ErrExtSchemaDeleteNotDisabled | 删除须先禁用 |
| 42030 | ErrExtSchemaVersionConflict | 乐观锁冲突 |
| 42031 | ErrExtSchemaEditNotDisabled | 编辑须先禁用 |

## 与事件类型的集成

- 事件类型 detail 接口（handler/event_type.go `Get` 方法）：返回 `extension_schema` 包含启用 Schema + config 中有值但 Schema 已禁用的
- 事件类型 create/update：通过 `schemaCache.GetByFieldName()` 校验扩展字段值
- Schema 的 `enabled` 字段在 Lite 结构体中暴露给前端，用于禁用展示
