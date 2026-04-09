# 字段管理后端重构 — 设计方案

## 方案描述

### 核心变更：name → id

所有操作标识从 `name (VARCHAR 64)` 改为 `id (BIGINT PK)`。`name` 仅在两个场景使用：创建时写入、check-name 校验时传入。

### 数据表变更

#### fields 表（不变，仅确认索引）

```sql
-- 表结构不变，003 迁移已加 enabled 列和重建索引
-- idx_list 已包含 enabled：(deleted, id, name, label, type, category, ref_count, enabled, created_at)
```

#### field_refs 表（重建）

```sql
-- 004 迁移：重建 field_refs 表，VARCHAR → BIGINT
DROP TABLE IF EXISTS field_refs;

CREATE TABLE field_refs (
    field_id    BIGINT       NOT NULL,   -- 被引用的字段 ID
    ref_type    VARCHAR(16)  NOT NULL,   -- 'template' / 'field'
    ref_id      BIGINT       NOT NULL,   -- 引用方 ID（模板 ID 或字段 ID）

    PRIMARY KEY (field_id, ref_type, ref_id),
    INDEX idx_ref (ref_type, ref_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

当前 field_refs 表无生产数据（模板管理未开发），可直接 DROP + CREATE。

#### 新增错误码

```go
ErrFieldEditNotDisabled = 40015  // "请先停用该字段再编辑"
```

### 接口设计（8 个）

#### 1. 创建字段 `POST /api/v1/fields/create`

**请求体**：
```json
{
  "name": "health",
  "label": "生命值",
  "type": "integer",
  "category": "basic",
  "properties": {"description": "...", "expose_bb": false, "default_value": 100, "constraints": {"min": 0, "max": 10000}}
}
```

**响应体**：
```json
{"code": 0, "data": {"id": 1, "name": "health"}, "message": "OK"}
```

**各层职责**：

| 层 | 做什么 |
|---|--------|
| Router | 注册 `POST /api/v1/fields/create` → `handler.WrapCtx(fh.Create)` |
| Handler | 校验 name 非空 + 正则 `^[a-z][a-z0-9_]*$` + 长度 ≤ 64；label 非空 + UTF-8 长度 ≤ 128；type/category 非空；properties 非 null |
| Service | 校验 type/category 字典存在；校验 name 唯一（含已删除）；如果是 reference 类型：校验被引用字段存在且启用、循环引用检测；写入 MySQL（enabled=0, version=1）；如果是 reference 类型：写入 field_refs + IncrRefCount；清列表缓存 |
| Store | `FieldStore.Create(ctx, field)` INSERT 返回 lastInsertId；`FieldRefStore.Add(tx, fieldID, refType, refID)` INSERT IGNORE；`FieldStore.IncrRefCountTx(tx, fieldID)` |

#### 2. 编辑字段 `POST /api/v1/fields/update`

**请求体**：
```json
{
  "id": 1,
  "label": "生命值",
  "type": "integer",
  "category": "combat",
  "properties": {"description": "...", "expose_bb": true, "default_value": 100, "constraints": {"min": 0, "max": 20000}},
  "version": 1
}
```

**响应体**：
```json
{"code": 0, "data": "保存成功", "message": "OK"}
```

**各层职责**：

| 层 | 做什么 |
|---|--------|
| Router | 注册路由 |
| Handler | 校验 id > 0；label 非空 + 长度；type/category 非空；properties 非 null；version > 0 |
| Service | 按 ID 查字段 → 不存在返回 40011；**校验 enabled=0 → 否则返回 40015**；校验 type/category 字典存在；ref_count > 0 时：禁止改类型(40006)、禁止收紧约束(40007)；如果是 reference 类型：校验被引用字段存在且启用、循环引用检测、diff 计算引用增减 → 事务内维护 field_refs + ref_count；乐观锁更新；清缓存（detail + list 版本号） |
| Store | `FieldStore.GetByID(ctx, id)` SELECT WHERE id=? AND deleted=0；`FieldStore.Update(ctx, field)` UPDATE WHERE id=? AND version=?；`FieldRefStore.RemoveBySource(tx, "field", sourceFieldID)` 清旧引用；`FieldRefStore.Add(tx, ...)` 加新引用 |

#### 3. 字段列表 `POST /api/v1/fields/list`

**请求体**：
```json
{
  "label": "",
  "type": "",
  "category": "",
  "enabled": null,
  "page": 1,
  "page_size": 20
}
```

**响应体**：
```json
{"code": 0, "data": {"items": [{"id": 1, "name": "health", ...}], "total": 42, "page": 1, "page_size": 20}, "message": "OK"}
```

**各层职责**：

| 层 | 做什么 |
|---|--------|
| Router | 注册路由 |
| Handler | 分页参数默认值/上限校正（从 config 读） |
| Service | 查 Redis 版本化列表缓存 → 未命中查 MySQL → type_label/category_label 内存翻译 → 写缓存 |
| Store | `FieldStore.List(ctx, query)` 覆盖索引查询 + COUNT；`FieldCache.GetList / SetList` |

#### 4. 软删除字段 `POST /api/v1/fields/delete`

**请求体**：
```json
{"id": 1}
```

**响应体**：
```json
{"code": 0, "data": {"id": 1, "name": "health", "label": "生命值"}, "message": "OK"}
```

**各层职责**：

| 层 | 做什么 |
|---|--------|
| Router | 注册路由 |
| Handler | 校验 id > 0 |
| Service | 按 ID 查字段 → 不存在返回 40011；**校验 enabled=0** → 否则返回 40012；事务内：FOR SHARE 检查 field_refs 无引用 → 有引用返回 40005；软删除（deleted=1）；如果是 reference 类型：清理它对其他字段的引用 + DecrRefCount；清缓存 |
| Store | `FieldStore.GetByID(ctx, id)`；`FieldRefStore.HasRefsTx(tx, fieldID)` SELECT FOR SHARE；`FieldStore.SoftDeleteTx(tx, id)`；`FieldRefStore.RemoveBySource(tx, "field", fieldID)` + `FieldStore.DecrRefCountTx(tx, targetFieldID)` |

#### 5. 启用/停用切换 `POST /api/v1/fields/toggle-enabled`

**请求体**：
```json
{"id": 1, "enabled": true, "version": 2}
```

**各层职责**：

| 层 | 做什么 |
|---|--------|
| Router | 注册路由 |
| Handler | 校验 id > 0；version > 0 |
| Service | 按 ID 查字段 → 不存在返回 40011；乐观锁更新 enabled；清缓存 |
| Store | `FieldStore.ToggleEnabled(ctx, id, enabled, version)` UPDATE WHERE id=? AND version=? |

#### 6. 字段名唯一性校验 `POST /api/v1/fields/check-name`

**请求体**：`{"name": "health"}`（保留 name，创建前校验）

**各层职责**：

| 层 | 做什么 |
|---|--------|
| Handler | 校验 name 非空 |
| Service | `FieldStore.ExistsByName(ctx, name)` 含已删除记录 |

#### 7. 引用详情 `POST /api/v1/fields/references`

**请求体**：`{"id": 1}`

**各层职责**：

| 层 | 做什么 |
|---|--------|
| Handler | 校验 id > 0 |
| Service | 按 ID 查字段 → 不存在返回 40011；查 field_refs WHERE field_id=?；按 ref_type 分组；查引用方的 label |
| Store | `FieldRefStore.GetByFieldID(ctx, fieldID)` 返回 `[]FieldRef`；`FieldStore.GetByIDs(ctx, ids)` 批量查 label |

#### 8. 字典选项 `POST /api/v1/dictionaries`

不变。

### Store 层方法清单

#### FieldStore（改名函数 + 新增函数）

| 方法 | 改动 | SQL |
|------|------|-----|
| `Create(ctx, field) (int64, error)` | 返回值加 lastInsertId | `INSERT INTO fields (...) VALUES (...)` |
| `GetByID(ctx, id) (*Field, error)` | **新增**，替代 GetByName | `SELECT * FROM fields WHERE id=? AND deleted=0` |
| `GetByName(ctx, name) (*Field, error)` | **保留**，check-name 和内部用 | `SELECT * FROM fields WHERE name=? AND deleted=0` |
| `ExistsByName(ctx, name) (bool, error)` | 不变 | `SELECT 1 FROM fields WHERE name=?` |
| `List(ctx, query) (*FieldListData, error)` | 不变 | 覆盖索引查询 |
| `Update(ctx, field) error` | WHERE 改用 id | `UPDATE fields SET ... WHERE id=? AND version=? AND deleted=0` |
| `SoftDeleteTx(tx, id) error` | WHERE 改用 id | `UPDATE fields SET deleted=1 WHERE id=? AND deleted=0` |
| `ToggleEnabled(ctx, id, enabled, version) error` | WHERE 改用 id | `UPDATE fields SET enabled=? WHERE id=? AND version=? AND deleted=0` |
| `IncrRefCountTx(tx, id) error` | WHERE 改用 id | `UPDATE fields SET ref_count=ref_count+1 WHERE id=?` |
| `DecrRefCountTx(tx, id) error` | WHERE 改用 id | `UPDATE fields SET ref_count=ref_count-1 WHERE id=?` |
| `GetByIDs(ctx, ids) ([]Field, error)` | **新增**，批量查 label | `SELECT id,name,label FROM fields WHERE id IN (?) AND deleted=0` |
| `GetRefCountTx(tx, id) (int, error)` | WHERE 改用 id | `SELECT ref_count FROM fields WHERE id=? FOR SHARE` |

移除：`BatchUpdateCategory`、`GetByNames`。

#### FieldRefStore

| 方法 | 改动 | SQL |
|------|------|-----|
| `Add(tx, fieldID, refType, refID) error` | 全部改 BIGINT | `INSERT IGNORE INTO field_refs (field_id, ref_type, ref_id) VALUES (?,?,?)` |
| `Remove(tx, fieldID, refType, refID) error` | 全部改 BIGINT | `DELETE FROM field_refs WHERE field_id=? AND ref_type=? AND ref_id=?` |
| `RemoveBySource(tx, refType, refID) ([]int64, error)` | 返回受影响的 field_id 列表 | `SELECT field_id FROM field_refs WHERE ref_type=? AND ref_id=?` + `DELETE ...` |
| `GetByFieldID(ctx, fieldID) ([]FieldRef, error)` | **改名** | `SELECT * FROM field_refs WHERE field_id=?` |
| `HasRefsTx(tx, fieldID) (bool, error)` | 改用 BIGINT | `SELECT 1 FROM field_refs WHERE field_id=? FOR SHARE LIMIT 1` |

移除：`GetByRefName`（改为 `RemoveBySource` 内联）。

### Redis 缓存 Key 变更

```go
// keys.go
func FieldDetailKey(id int64) string { return fmt.Sprintf("%s%d", prefixFieldDetail, id) }
func FieldLockKey(id int64) string   { return fmt.Sprintf("%s%d", prefixFieldLock, id) }
// FieldListKey 不变（不含 name/id，按筛选条件+版本号）
```

### Model 变更

```go
// FieldRef 改用 ID
type FieldRef struct {
    FieldID int64  `json:"field_id" db:"field_id"`
    RefType string `json:"ref_type" db:"ref_type"`
    RefID   int64  `json:"ref_id"   db:"ref_id"`
}

// 请求体改用 ID
type IDRequest struct {
    ID int64 `json:"id"`
}

type UpdateFieldRequest struct {
    ID         int64           `json:"id"`
    Label      string          `json:"label"`
    Type       string          `json:"type"`
    Category   string          `json:"category"`
    Properties json.RawMessage `json:"properties"`
    Version    int             `json:"version"`
}

// CreateFieldRequest 不含 ID（不变）
type CreateFieldRequest struct {
    Name       string          `json:"name"`
    Label      string          `json:"label"`
    Type       string          `json:"type"`
    Category   string          `json:"category"`
    Properties json.RawMessage `json:"properties"`
}

// ToggleEnabledRequest 改用 ID
type ToggleEnabledRequest struct {
    ID      int64 `json:"id"`
    Enabled bool  `json:"enabled"`
    Version int   `json:"version"`
}
```

移除：`NameRequest`（用 `IDRequest` 替代）、`BatchDeleteRequest`、`BatchDeleteResult`、`BatchDeleteSkipped`、`BatchCategoryRequest`、`BatchCategoryResponse`。

保留：`CheckNameRequest`（仍用 name）。

---

## 方案对比

### 方案 A（采用）：全量重写，一次性迁移

- field_refs DROP + CREATE
- 所有 Store/Service/Handler 一次性改完
- 好处：干净彻底，无兼容包袱
- 可行原因：field_refs 无生产数据，模板管理未开发

### 方案 B（不采用）：渐进迁移，双主键过渡

- 保留 field_name 列，新增 field_id 列，逐步迁移
- 好处：可回滚
- 不选原因：field_refs 当前无数据，双列过渡是无意义的复杂度，违反"禁止过度设计"红线

---

## 红线检查

| 红线 | 是否违反 | 说明 |
|------|---------|------|
| **通用：禁止静默降级** | 否 | GetByID 找不到返回 40011，不静默 |
| **通用：禁止信任前端校验** | 否 | Handler 做格式校验，Service 做业务校验，双层保障 |
| **通用：禁止过度设计** | 否 | 方案 B 的双列过渡属于过度设计，已排除 |
| **MySQL：禁止事务混用 db/tx** | 否 | 所有事务内操作统一用 tx 参数 |
| **MySQL：引用检查用 FOR SHARE** | 否 | 删除前 HasRefsTx 使用 FOR SHARE |
| **MySQL：LIKE 转义** | 否 | label 模糊搜索保留 escapeLike |
| **Redis：禁止 SCAN+DEL** | 否 | 列表缓存用版本号方案 |
| **Redis：DEL/Unlock 检查 error** | 否 | 保留现有错误检查 |
| **架构：禁止硬编码错误码** | 否 | 新增 ErrFieldEditNotDisabled 常量 |
| **架构：禁止硬编码 Redis key** | 否 | key 函数在 keys.go 统一管理 |
| **架构：禁止硬编码引用类型** | 否 | 使用 model.RefTypeTemplate/RefTypeField 常量 |
| **架构：Handler 校验错误码** | 否 | id 校验用 ErrBadRequest，name 校验用 ErrFieldNameInvalid |

---

## 扩展性影响

**正面**：模板管理模块开发时，调用 `FieldRefStore.Add(tx, fieldID, "template", templateID)` + `FieldStore.IncrRefCountTx(tx, fieldID)`。BIGINT 关联比 VARCHAR 更高效，接口更规范。

---

## 依赖方向

```
handler → service → store/mysql
                  → store/redis
                  → cache (dictCache)
         → model  (数据结构)
         → errcode (错误码)
         → config  (配置)
```

单向向下，无循环依赖。store 不 import service，service 不 import handler。

---

## 陷阱检查

| 陷阱来源 | 检查项 | 应对 |
|---------|--------|------|
| go-pitfalls: nil slice → null | 列表返回空数组 | `make([]FieldListItem, 0)` |
| go-pitfalls: json.RawMessage NULL | properties 列 | 建表 `NOT NULL`，Go 层用 `json.RawMessage` 非指针 |
| go-pitfalls: len() 不是字符数 | label 长度校验 | `utf8.RuneCountInString(label)` |
| go-pitfalls: 响应后忘 return | WrapCtx 封装 | 统一封装已处理 |
| mysql-pitfalls: 乐观锁 rows==0 | Update/ToggleEnabled | Service 预查存在性，rows=0 即版本冲突 |
| mysql-pitfalls: LIKE 转义 | label 搜索 | escapeLike() |
| mysql-pitfalls: FOR SHARE vs FOR UPDATE | 删除引用检查 | HasRefsTx 用 FOR SHARE |
| redis-pitfalls: Get 返回 redis.Nil | 详情缓存 | `errors.Is(err, redis.Nil)` |
| cache-pitfalls: 写后必须清缓存 | 所有写操作 | Update/Delete/Toggle 后 DEL detail + INCR version |
| cache-pitfalls: 级联清缓存 | 引用关系变更时被引用方 | DecrRefCount 后 DEL 被引用方 detail |

---

## 配置变更

无新增配置项。现有 `config.yaml` 中的 `validation.field_name_max_length`、`validation.field_label_max_length`、`pagination` 等不变。

---

## 测试策略

本次为重构，不新增单元测试（毕设阶段）。验证方式：

1. 重写完成后 `go build` 编译通过
2. `docker compose up --build` 启动成功
3. 通过 curl/Postman 手动验证 8 个接口的正常路径和异常路径（对照 R1-R18 验收标准）
