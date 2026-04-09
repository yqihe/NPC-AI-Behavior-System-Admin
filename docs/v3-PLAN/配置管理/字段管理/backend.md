# 字段管理 — 后端设计

> 通用技术选型、架构分层、存储原则、缓存策略、常见问题见 [backend-guide.md](../../backend-guide.md)。
> 本文档只记录字段管理特有的设计。

---

## 存储范围

字段是 ADMIN 内部的管理概念，游戏服务端不需要字段定义（导出的 5 个接口不含 fields）。字段值最终通过「模板 → NPC → 导出」打平写入 `npc_templates.config.fields`。

因此：
- **MySQL**：唯一写入目标
- **MongoDB**：不操作
- **RabbitMQ**：不需要（无跨库同步）

---

## 操作标识

**所有操作使用主键 ID (BIGINT)**，不使用 name。name 仅在两个场景使用：
1. 创建时写入（请求体含 name）
2. check-name 唯一性校验时传入

这是企业级 CRUD 系统的标准做法：主键 ID 做操作标识，name 只用于展示和唯一性校验。INT 比较快于 VARCHAR，field_refs 的 JOIN/IN 查询更高效。

---

## 数据表

### fields

```sql
CREATE TABLE fields (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    name            VARCHAR(64)  NOT NULL,
    label           VARCHAR(128) NOT NULL,
    type            VARCHAR(32)  NOT NULL,
    category        VARCHAR(32)  NOT NULL,
    properties      JSON         NOT NULL,

    ref_count       INT          NOT NULL DEFAULT 0,
    enabled         TINYINT(1)   NOT NULL DEFAULT 0,
    version         INT          NOT NULL DEFAULT 1,
    deleted         TINYINT(1)   NOT NULL DEFAULT 0,
    created_at      DATETIME     NOT NULL,
    updated_at      DATETIME     NOT NULL,

    UNIQUE KEY uk_name (name),
    INDEX idx_list (deleted, id, name, label, type, category, ref_count, enabled, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### field_refs（ID 关联）

```sql
CREATE TABLE field_refs (
    field_id    BIGINT       NOT NULL,              -- 被引用的字段 ID
    ref_type    VARCHAR(16)  NOT NULL,              -- 'template' / 'field'
    ref_id      BIGINT       NOT NULL,              -- 引用方 ID

    PRIMARY KEY (field_id, ref_type, ref_id),
    INDEX idx_ref (ref_type, ref_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**两种引用来源：**
- `ref_type = 'template'`：模板引用了该字段
- `ref_type = 'field'`：其他 reference 类型字段引用了该字段

**ref_count 维护**（事务内）：

```go
fieldRefStore.Add(ctx, tx, targetFieldID, model.RefTypeField, sourceFieldID)
fieldStore.IncrRefCountTx(ctx, tx, targetFieldID)
```

### dictionaries（字段管理依赖的 group）

| group_name | 用途 | 示例 |
|------------|------|------|
| `field_type` | 字段类型下拉 + constraint_schema | integer, float, string, boolean, select, reference |
| `field_category` | 标签分类下拉 | basic, combat, perception, movement, interaction, personality |
| `field_properties` | 动态表单属性定义 | description, expose_bb, default_value, constraints |

---

## API（8 个接口）

```
POST   /api/v1/fields/list             列表（label/type/category/enabled/page/page_size）
POST   /api/v1/fields/create           创建字段（默认 enabled=0）
POST   /api/v1/fields/detail           详情（id）
POST   /api/v1/fields/update           编辑（id + version，仅未启用时可编辑）
POST   /api/v1/fields/delete           软删除（id，仅未启用且无引用）
POST   /api/v1/fields/check-name       唯一性校验（name，含已删除）
POST   /api/v1/fields/references       引用详情（id）
POST   /api/v1/fields/toggle-enabled   启用/停用切换（id + version 乐观锁）
POST   /api/v1/dictionaries            字典选项列表
```

### 请求/响应结构

| 接口 | 请求体 | 响应体 |
|------|--------|--------|
| list | `FieldListQuery { label, type, category, enabled?, page, page_size }` | `ListData { items: FieldListItem[], total, page, page_size }` |
| create | `CreateFieldRequest { name, label, type, category, properties }` | `CreateFieldResponse { id, name }` |
| detail | `IDRequest { id }` | `Field { 全量字段 }` |
| update | `UpdateFieldRequest { id, label, type, category, properties, version }` | `string "保存成功"` |
| delete | `IDRequest { id }` | `DeleteResult { id, name, label }` |
| check-name | `CheckNameRequest { name }` | `CheckNameResult { available, message }` |
| references | `IDRequest { id }` | `ReferenceDetail { field_id, field_label, templates[], fields[] }` |
| toggle-enabled | `ToggleEnabledRequest { id, enabled, version }` | `string "操作成功"` |

### 错误码

| 错误码 | 常量 | 含义 |
|--------|------|------|
| 40001 | ErrFieldNameExists | 字段标识已存在 |
| 40002 | ErrFieldNameInvalid | 字段标识格式不合法 |
| 40003 | ErrFieldTypeNotFound | 字段类型不存在 |
| 40004 | ErrFieldCategoryNotFound | 标签分类不存在 |
| 40005 | ErrFieldRefDelete | 该字段正被引用，无法删除 |
| 40006 | ErrFieldRefChangeType | 该字段已被引用，无法修改类型 |
| 40007 | ErrFieldRefTighten | 已有数据可能超出新约束范围 |
| 40008 | ErrFieldBBKeyInUse | BB Key 被行为树引用无法关闭（待对接） |
| 40009 | ErrFieldCyclicRef | 检测到循环引用 |
| 40010 | ErrFieldVersionConflict | 版本冲突（乐观锁） |
| 40011 | ErrFieldNotFound | 字段不存在 |
| 40012 | ErrFieldDeleteNotDisabled | 请先停用该字段再删除 |
| 40013 | ErrFieldRefDisabled | 不能引用已停用的字段 |
| 40014 | ErrFieldRefNotFound | 引用的字段不存在 |
| 40015 | ErrFieldEditNotDisabled | 请先停用该字段再编辑 |

### Handler 层校验规则

| 字段 | 规则 |
|------|------|
| id | > 0（detail/update/delete/references/toggle-enabled），用 ErrBadRequest |
| name | 非空 + `^[a-z][a-z0-9_]*$` + 长度 ≤ 64（create/check-name），用 ErrFieldNameInvalid |
| label | 非空 + UTF-8 字符数 ≤ 128 |
| type | 非空（存在性由 service 查字典校验） |
| category | 非空 |
| properties | 非 null JSON 对象 |
| version | > 0（update/toggle-enabled） |

---

## 三态生命周期

| 状态 | enabled | deleted | 列表可见 | 可编辑 | 可被新引用 | 已有引用 |
|------|---------|---------|---------|--------|-----------|---------|
| **启用** | 1 | 0 | 正常展示 | 禁止（40015） | 可以 | 正常使用 |
| **停用** | 0 | 0 | 灰色展示 | 可以 | 禁止（40013） | 保留不动 |
| **已删除** | - | 1 | 不可见 | 不可能 | 不可能 | 删前已清理 |

状态转换：
- 创建 → 停用（新建默认 enabled=0）
- 停用 ↔ 启用（toggle-enabled，乐观锁）
- 停用 → 已删除（必须先停用 + 无引用才能删除）
- 启用 → 已删除（❌ 不允许直接删除）
- 启用 → 编辑（❌ 不允许，必须先停用）

---

## 关键查询

### 列表（覆盖索引，不回表）

```sql
SELECT id, name, label, type, category, ref_count, enabled, created_at
FROM fields WHERE deleted = 0 ORDER BY id DESC LIMIT 20 OFFSET 0;
```

### 详情（主键查询）

```sql
SELECT * FROM fields WHERE id = ? AND deleted = 0;
```

### 引用详情

```sql
SELECT field_id, ref_type, ref_id FROM field_refs WHERE field_id = ?;
SELECT id, name, label FROM fields WHERE id IN (?) AND deleted = 0;
```

### 删除检查（事务内 FOR SHARE）

```sql
SELECT COUNT(*) FROM field_refs WHERE field_id = ? FOR SHARE;
```

---

## 业务逻辑

### 编辑限制检查

```
service.Update 检查链:
1. 按 ID 查字段 → 不存在返回 40011
2. 校验 enabled=0 → 否则返回 40015
3. 字典校验 type/category
4. ref_count > 0 时: 禁止改类型(40006)、禁止收紧约束(40007)
5. reference 类型: 引用存在性+启用检查+循环引用检测+引用关系维护
6. 乐观锁写入 → 版本不匹配则 40010
7. 清缓存: DEL detail:{id} + INCR version + 级联清被引用方 detail
```

### 删除流程

```
service.Delete:
1. 按 ID 查字段 → 不存在返回 40011
2. 校验 enabled=0 → 否则返回 40012
3. 事务内:
   a. FOR SHARE 检查引用 → 有引用返回 40005
   b. 软删除 (UPDATE SET deleted=1 WHERE id=?)
   c. reference 类型: RemoveBySource + DecrRefCountTx
4. 清缓存: DEL detail:{id} + 级联清被引用方 + INCR version
```

### 循环引用检测（DFS，使用 ID）

```go
func detectCyclicRef(ctx, currentID int64, refIDs []int64) error {
    visited := map[int64]bool{currentID: true}  // 新建时 currentID=0
    dfs(refIDs, visited)  // 递归展开 reference 类型字段的引用链
}
```

### 引用关系同步（diff 计算，事务内）

```go
func syncFieldRefs(ctx, sourceFieldID int64, oldRefIDs, newRefIDs []int64) {
    toAdd = newRefIDs - oldRefIDs    // 新增引用: Add + IncrRefCountTx
    toRemove = oldRefIDs - newRefIDs // 移除引用: Remove + DecrRefCountTx
}
```

---

## 缓存策略

### Key 设计

| Key 模式 | TTL | 用途 |
|----------|-----|------|
| `fields:detail:{id}` | 5min + jitter | 单条详情缓存 |
| `fields:list:v{ver}:{type}:{category}:{label}:{enabled}:{page}:{pageSize}` | 1min + jitter | 分页列表缓存 |
| `fields:list:version` | 无 TTL | 列表缓存版本号 |
| `fields:lock:{id}` | 3s | 分布式锁（防缓存击穿） |

### 写入失效

| 操作 | 缓存动作 |
|------|---------|
| 创建 | INCR version + 级联 DEL 被引用方 detail |
| 编辑 | DEL detail:{id} + INCR version + 级联 DEL 被引用方 detail |
| 删除 | DEL detail:{id} + INCR version + 级联 DEL 被引用方 detail |
| 切换启用 | DEL detail:{id} + INCR version |

### 降级策略

Redis 不可用时全部降级到 MySQL 直查，不阻塞业务。

---

## 并发安全

| 场景 | 机制 |
|------|------|
| 编辑冲突 | 乐观锁 `WHERE id=? AND version=?` |
| 缓存击穿 | 分布式锁 `SetNX fields:lock:{id}` + double-check |
| 缓存穿透 | 空标记 `{"_null":true}` |
| 删除 TOCTOU | 事务内 `FOR SHARE` 锁住 field_refs |
| 启用/停用冲突 | 乐观锁 |

---

## Store 层方法清单

### FieldStore

| 方法 | 说明 |
|------|------|
| `Create(ctx, req) (int64, error)` | INSERT，返回 lastInsertId |
| `GetByID(ctx, id) (*Field, error)` | 主键查询 |
| `GetByName(ctx, name) (*Field, error)` | uk_name 查询（check-name 用） |
| `ExistsByName(ctx, name) (bool, error)` | 含软删除检查 |
| `List(ctx, query) ([]FieldListItem, int64, error)` | 覆盖索引 |
| `Update(ctx, req) error` | 乐观锁 WHERE id=? AND version=? |
| `SoftDeleteTx(tx, id) error` | 事务内软删除 |
| `ToggleEnabled(ctx, id, enabled, version) error` | 乐观锁切换 |
| `IncrRefCountTx(tx, id) error` | 事务内 +1 |
| `DecrRefCountTx(tx, id) error` | 事务内 -1 |
| `GetByIDs(ctx, ids) ([]Field, error)` | IN 查询批量取 |
| `GetRefCountTx(tx, id) (int, error)` | FOR SHARE |

### FieldRefStore

| 方法 | 说明 |
|------|------|
| `Add(tx, fieldID, refType, refID) error` | INSERT IGNORE |
| `Remove(tx, fieldID, refType, refID) error` | DELETE |
| `RemoveBySource(tx, refType, refID) ([]int64, error)` | 清理引用方的所有引用，返回被引用字段 ID |
| `GetByFieldID(ctx, fieldID) ([]FieldRef, error)` | 查某字段的所有引用方 |
| `HasRefsTx(tx, fieldID) (bool, error)` | FOR SHARE 检查 |

---

## 跨模块集成点

| 对接模块 | 集成内容 | 通知文档 |
|---------|---------|---------|
| 模板管理 | 勾选字段时调用 `FieldRefStore.Add(tx, fieldID, "template", templateID)` + `FieldStore.IncrRefCountTx(tx, fieldID)`；enabled 状态约束；引用详情补全模板 label | `配置管理/模板管理/INTEGRATION_NOTE_FROM_FIELD.md` |
| 行为树 | 提供 `IsBBKeyUsed(ctx, bbKey) bool`，供字段管理关闭 BB Key 时校验（错误码 40008 已定义，对接逻辑待补） | `行为管理/行为树/INTEGRATION_NOTE_FROM_FIELD.md` |
