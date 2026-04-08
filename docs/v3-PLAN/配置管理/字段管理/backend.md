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
    version         INT          NOT NULL DEFAULT 1,
    deleted         TINYINT(1)   NOT NULL DEFAULT 0,
    created_at      DATETIME     NOT NULL,
    updated_at      DATETIME     NOT NULL,

    UNIQUE KEY uk_name (name),
    INDEX idx_list (deleted, id, name, label, type, category, ref_count, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**固定列选择依据：**

| 列 | 为什么做固定列 |
|----|--------------|
| name | 唯一标识，uk_name 唯一性校验 + 单条详情 |
| label | 列表搜索 `LIKE '%xxx%'` |
| type | 筛选条件 `WHERE type = 'integer'` |
| category | 筛选条件 `WHERE category = 'combat'` |

**properties JSON 内容：**

```json
{
  "description": "NPC 的血量，降为 0 时死亡",
  "expose_bb": true,
  "default_value": 100,
  "constraints": { "min": 0, "max": 10000, "step": 1 }
}
```

未来新增属性直接加 key，旧数据缺失的 key 读取为 null，前端显示空/默认值，不用回填不改表。

### field_refs

```sql
CREATE TABLE field_refs (
    field_name      VARCHAR(64)  NOT NULL,
    ref_type        VARCHAR(16)  NOT NULL,     -- 'template' / 'field'
    ref_name        VARCHAR(64)  NOT NULL,

    PRIMARY KEY (field_name, ref_type, ref_name),
    INDEX idx_ref (ref_type, ref_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**两种引用来源：**
- `ref_type = 'template'`：模板引用了该字段
- `ref_type = 'field'`：其他 reference 类型字段引用了该字段

**ref_count 维护**（事务内）：

```go
tx.Exec("INSERT INTO field_refs VALUES (?, ?, ?)", fieldName, refType, refName)
tx.Exec("UPDATE fields SET ref_count = ref_count + 1 WHERE name = ?", fieldName)
```

### dictionaries（字段管理依赖的 group）

| group_name | 用途 | 示例 |
|------------|------|------|
| `field_type` | 字段类型下拉 + constraint_schema | integer, float, string, boolean, select, reference |
| `field_category` | 标签分类下拉 | basic, combat, perception, movement, interaction, personality |
| `field_properties` | 动态表单属性定义 | description, expose_bb, default_value, constraints |

表结构和缓存策略见 [backend-guide.md](../../backend-guide.md)。

---

## API

```
POST   /api/v1/fields                        创建字段
GET    /api/v1/fields                        列表（?label=&type=&category=&page=&page_size=）
GET    /api/v1/fields/:name                   详情
PUT    /api/v1/fields/:name                   编辑
DELETE /api/v1/fields/:name                   删除
POST   /api/v1/fields/check-name              唯一性校验
GET    /api/v1/fields/:name/references         引用详情
POST   /api/v1/fields/batch-delete            批量删除
PUT    /api/v1/fields/batch-category           批量修改分类
```

### 错误码

| 错误码 | 含义 |
|--------|------|
| 40001 | 字段标识已存在 |
| 40002 | 字段标识格式不合法 |
| 40003 | 字段类型不存在 |
| 40004 | 标签分类不存在 |
| 40005 | 被引用无法删除 |
| 40006 | 被引用无法修改类型 |
| 40007 | 被引用无法收紧约束 |
| 40008 | BB Key 被行为树引用无法关闭 |
| 40009 | 循环引用 |
| 40010 | 版本冲突（乐观锁） |
| 40011 | 引用的字段不存在 |

---

## 关键查询

### 列表（覆盖索引，不回表）

```sql
SELECT id, name, label, type, category, ref_count, created_at
FROM fields
WHERE deleted = 0
ORDER BY id DESC
LIMIT 20 OFFSET 0;
```

带筛选时加 `AND type = ? AND category = ?`，idx_list 索引列内过滤。

带搜索时加 `AND label LIKE '%xxx%'`，千级数据量全表扫描可接受。

type_label / category_label 由内存 map 翻译，不查表不 JOIN。

### 详情（唯一索引，回表取 properties）

```sql
SELECT * FROM fields WHERE name = ? AND deleted = 0;
```

### 引用详情（两次独立查询，不 JOIN）

```sql
-- 1. 查关系（主键索引前缀）
SELECT ref_type, ref_name FROM field_refs WHERE field_name = ?;

-- 2. 查模板信息（IN 查询，走主键）
SELECT name, label, category FROM templates WHERE name IN (...);

-- 3. 查字段信息（IN 查询，走 uk_name）
SELECT name, label FROM fields WHERE name IN (...) AND deleted = 0;
```

### 删除检查（主键索引前缀）

```sql
SELECT ref_type, ref_name FROM field_refs WHERE field_name = ?;
-- 有结果 → 禁止删除，返回引用列表
```

---

## 业务逻辑

### 循环引用检测

创建/编辑 reference 字段时，递归检查引用链：

```go
func checkCyclicRef(fieldName string, refs []string, visited map[string]bool) error {
    if visited[fieldName] {
        return fmt.Errorf("循环引用：%s", buildRefChain(visited, fieldName))
    }
    visited[fieldName] = true
    for _, ref := range refs {
        field, _ := store.GetField(ref)
        if field.Type == "reference" {
            subRefs := field.Properties.Constraints.Refs
            if err := checkCyclicRef(ref, subRefs, visited); err != nil {
                return err
            }
        }
    }
    delete(visited, fieldName)
    return nil
}
```

### 编辑限制检查

```go
func checkEditConstraints(old, new *Field) error {
    if old.Type != new.Type && old.RefCount > 0 {
        return ErrTypeChangeWithRefs    // 40006
    }
    if isConstraintTightened(old, new) && old.RefCount > 0 {
        return ErrConstraintTightened   // 40007
    }
    if old.ExposeBB && !new.ExposeBB {
        btRefs := store.GetBTRefsForBBKey(old.Name)
        if len(btRefs) > 0 {
            return ErrBBKeyInUse        // 40008
        }
    }
    return nil
}
```

### 写入流程

| 操作 | MySQL | MongoDB | Redis |
|------|-------|---------|-------|
| 创建 | INSERT fields | — | DEL 分页缓存 |
| 编辑 | UPDATE fields（乐观锁） | — | DEL 分页缓存 + DEL 单条缓存 |
| 删除 | UPDATE SET deleted=1 | — | DEL 分页缓存 + DEL 单条缓存 |
| 查列表 | SELECT（覆盖索引） | — | 先查分页缓存 |
| 查详情 | SELECT（uk_name） | — | 先查单条缓存 |
