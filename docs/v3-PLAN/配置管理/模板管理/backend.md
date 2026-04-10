# 模板管理 — 后端设计

> 通用技术选型、架构分层、存储原则、缓存策略、常见问题见 [backend-guide.md](../../backend-guide.md)。
> 字段管理的设计详见 [字段管理/backend.md](../字段管理/backend.md) —— 模板管理大量复用其惯例。
> 本文档只记录模板管理特有的设计。

---

## 存储范围

模板是 ADMIN 内部的"字段组合方案"，游戏服务端不需要模板定义本身（导出的 5 个接口只含 npc_templates，那是 NPC 创建后的快照，不是这里的 templates 表）。

因此：
- **MySQL**：唯一写入目标（templates 表 + 复用 field_refs）
- **MongoDB**：不操作（NPC 模块才会写）
- **RabbitMQ**：不需要（无跨库同步）

---

## 操作标识

**所有操作使用主键 ID (BIGINT)**，与字段管理保持一致。`name` 仅在两个场景使用：
1. 创建时写入（请求体含 name）
2. check-name 唯一性校验时传入

NPC 模块未来引用模板时也用 `template_id` 而非 name，理由同字段管理：INT 比较快、JOIN/IN 高效。

---

## 数据表

### templates

```sql
CREATE TABLE templates (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    name            VARCHAR(64)  NOT NULL,              -- 模板标识，唯一，创建后不可变
    label           VARCHAR(128) NOT NULL,              -- 中文标签（搜索用）
    description     VARCHAR(512) NOT NULL DEFAULT '',   -- 描述（可选，仅展示）
    fields          JSON         NOT NULL,              -- [{field_id, required}, ...] 数组顺序=NPC 表单展示顺序

    ref_count       INT          NOT NULL DEFAULT 0,    -- 被 NPC 引用数（冗余计数，事务内维护）
    enabled         TINYINT(1)   NOT NULL DEFAULT 0,    -- 启用状态，创建默认 0（配置窗口期）
    version         INT          NOT NULL DEFAULT 1,    -- 乐观锁
    deleted         TINYINT(1)   NOT NULL DEFAULT 0,    -- 软删除
    created_at      DATETIME     NOT NULL,
    updated_at      DATETIME     NOT NULL,

    UNIQUE KEY uk_name (name),
    INDEX idx_list (deleted, id, name, label, ref_count, enabled, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**几个关键决策：**

| 决策 | 选择 | 原因 |
|---|---|---|
| `name` 唯一约束 | 不带 `deleted` | 已删除的 name 永久不可复用，防止历史 NPC 引用混乱 |
| `description` | NOT NULL DEFAULT `''` | Go 端不用 `sql.NullString`，统一空串语义 |
| `fields` 存储 | JSON 列 | 模板和字段是"快照式"关系，不需要 JOIN；数组顺序天然=展示顺序，**排序功能零额外成本** |
| `enabled` 默认 | 0 | 与字段表一致，给"配置窗口期" |
| 索引 | 单一 idx_list | 列表查询的两个变体（全部 / `enabled=1`）均通过覆盖索引扫描满足 |

### fields JSON 结构

```json
[
  {"field_id": 1, "required": true},
  {"field_id": 5, "required": false},
  {"field_id": 8, "required": true}
]
```

**重要约定：**
- **数组顺序就是展示顺序** —— NPC 创建表单按这个顺序渲染字段，前端的"上下移动"按钮就是修改这个数组顺序。
- **没有 reference 字段的痕迹** —— 弹层勾选 reference 时已经在前端展平为 leaf 字段 ID 列表，后端只看到打平后的结果。
- **去重在前端做** —— 同一个 field_id 不能在数组里出现两次，前端勾选时即时去重；后端在 service 层做最终防御性校验。

### field_refs（复用字段管理已有表）

```sql
-- 字段管理已建（见 字段管理/backend.md）
CREATE TABLE field_refs (
    field_id    BIGINT       NOT NULL,
    ref_type    VARCHAR(16)  NOT NULL,              -- 'template' / 'field'
    ref_id      BIGINT       NOT NULL,
    PRIMARY KEY (field_id, ref_type, ref_id),
    INDEX idx_ref (ref_type, ref_id)
);
```

**模板管理使用方式：** 永远以 `ref_type = 'template'` + `ref_id = templateID` 写入/查询/删除。常量定义在 `backend/internal/model/field.go` 的 `RefTypeTemplate`，不硬编码字符串。

**写入时机（事务内）：**

```go
// 创建模板 / 编辑模板新增字段
fieldRefStore.Add(ctx, tx, fieldID, model.RefTypeTemplate, templateID)
fieldStore.IncrRefCountTx(ctx, tx, fieldID)

// 删除模板 / 编辑模板移除字段
fieldRefStore.Remove(ctx, tx, fieldID, model.RefTypeTemplate, templateID)
fieldStore.DecrRefCountTx(ctx, tx, fieldID)
```

---

## API（8 个接口）

```
POST   /api/v1/templates/list              列表（label/enabled/page/page_size）
POST   /api/v1/templates/create            创建模板（默认 enabled=0）
POST   /api/v1/templates/detail            详情（id），返回模板 + 字段精简列表
POST   /api/v1/templates/update            编辑（id + version，仅未启用时可编辑）
POST   /api/v1/templates/delete            软删除（id，仅未启用且无 NPC 引用）
POST   /api/v1/templates/check-name        唯一性校验（name，含已删除）
POST   /api/v1/templates/references        引用详情（id，列出哪些 NPC 在用）
POST   /api/v1/templates/toggle-enabled    启用/停用切换（id + version 乐观锁）
```

> 注意：**没有独立的"详情页接口"** —— `detail` 接口同时服务于编辑页（管理员查看 + 修改）和 NPC 管理页（拉字段列表渲染表单）。features.md 功能 3 已说明编辑页承担了双重角色。

### 请求/响应结构

| 接口 | 请求体 | 响应体 |
|------|--------|--------|
| list | `TemplateListQuery { label, enabled?, page, page_size }` | `ListData { items: TemplateListItem[], total, page, page_size }` |
| create | `CreateTemplateRequest { name, label, description, fields }` | `CreateTemplateResponse { id, name }` |
| detail | `IDRequest { id }` | `TemplateDetail { 模板字段 + fields[]：精简字段信息 }` |
| update | `UpdateTemplateRequest { id, label, description, fields, version }` | `string "保存成功"` |
| delete | `IDRequest { id }` | `DeleteResult { id, name, label }` |
| check-name | `CheckNameRequest { name }` | `CheckNameResult { available, message }` |
| references | `IDRequest { id }` | `TemplateReferenceDetail { template_id, template_label, npcs[] }` |
| toggle-enabled | `ToggleEnabledRequest { id, enabled, version }` | `string "操作成功"` |

### TemplateListItem（覆盖索引返回，不含 fields）

```go
type TemplateListItem struct {
    ID        int64     `json:"id"`
    Name      string    `json:"name"`
    Label     string    `json:"label"`
    RefCount  int       `json:"ref_count"`
    Enabled   bool      `json:"enabled"`
    CreatedAt time.Time `json:"created_at"`
}
```

不返回 `description` 和 `fields` —— 列表场景用不到，纯粹减少网络传输。

### TemplateDetail（详情接口返回）

```go
type TemplateDetail struct {
    ID          int64                `json:"id"`
    Name        string               `json:"name"`
    Label       string               `json:"label"`
    Description string               `json:"description"`
    Enabled     bool                 `json:"enabled"`
    Version     int                  `json:"version"`
    RefCount    int                  `json:"ref_count"`
    CreatedAt   time.Time            `json:"created_at"`
    UpdatedAt   time.Time            `json:"updated_at"`
    Fields      []TemplateFieldItem  `json:"fields"` // 顺序与 templates.fields JSON 数组一致
}

// 详情接口返回的字段精简信息（service 层批量补全）
type TemplateFieldItem struct {
    FieldID       int64  `json:"field_id"`
    Name          string `json:"name"`
    Label         string `json:"label"`
    Type          string `json:"type"`
    Category      string `json:"category"`
    CategoryLabel string `json:"category_label"` // dictionary 翻译
    Enabled       bool   `json:"enabled"`        // 字段当前是否启用（用于前端标灰停用字段）
    Required      bool   `json:"required"`      // 模板里的必填配置
}
```

**详情接口流程：**
1. 查 templates 主表 → 拿到 fields JSON
2. 解出 `[{field_id, required}, ...]`
3. `FieldStore.GetByIDs(fieldIDs)` 批量拉字段精简信息
4. 把 required 标记拼回去，生成 `[]TemplateFieldItem`
5. **保持原数组顺序**（不要按 ID 或字典序重排，前端依赖这个顺序渲染表单）
6. CategoryLabel 走 `dictCache.GetLabel("field_category", category)` 翻译

### 错误码

| 错误码 | 常量 | 含义 |
|--------|------|------|
| 41001 | ErrTemplateNameExists | 模板标识已存在（含软删除） |
| 41002 | ErrTemplateNameInvalid | 模板标识格式不合法 |
| 41003 | ErrTemplateNotFound | 模板不存在 |
| 41004 | ErrTemplateNoFields | 未勾选任何字段 |
| 41005 | ErrTemplateFieldDisabled | 勾选了停用字段 |
| 41006 | ErrTemplateFieldNotFound | 勾选的字段不存在 |
| 41007 | ErrTemplateRefDelete | 被 NPC 引用，无法删除 |
| 41008 | ErrTemplateRefEditFields | 被 NPC 引用，无法编辑字段列表（含顺序变更）|
| 41009 | ErrTemplateDeleteNotDisabled | 删除前必须先停用 |
| 41010 | ErrTemplateEditNotDisabled | 编辑前必须先停用 |
| 41011 | ErrTemplateVersionConflict | 乐观锁版本冲突 |

### Handler 层校验规则

| 字段 | 规则 |
|------|------|
| id | > 0（detail/update/delete/references/toggle-enabled），用 ErrBadRequest |
| name | 非空 + `^[a-z][a-z0-9_]{2,49}$` + 长度 ≤ 64（create/check-name），用 ErrTemplateNameInvalid |
| label | 非空 + UTF-8 字符数 ≤ 128 |
| description | 长度 ≤ 512（可空） |
| fields | 非空数组（create/update），每项必须 `field_id > 0`，**禁止重复 field_id** |
| version | > 0（update/toggle-enabled） |

---

## 三态生命周期

| 状态 | enabled | deleted | 模板列表可见 | NPC 管理可见 | 可编辑 | 可删除 | 可被新 NPC 选 |
|------|---------|---------|-------------|-------------|--------|--------|--------------|
| **启用** | 1 | 0 | 正常展示 | 可选 | 禁止（41010） | 禁止（41009）| 允许 |
| **停用** | 0 | 0 | 灰色展示 | 不可见 | 可以 | 可以（无 NPC 引用时） | 拒绝 |
| **已删除** | - | 1 | 不可见 | 不可见 | 不可能 | 已删 | 不可能 |

状态转换：
- 创建 → 停用（新建默认 enabled=0）
- 停用 ↔ 启用（toggle-enabled，乐观锁）
- 停用 → 已删除（必须先停用 + 无 NPC 引用才能删除）
- 启用 → 已删除（❌ 不允许直接删除）
- 启用 → 编辑（❌ 不允许，必须先停用）

**编辑权限按引用数二档：**

| ref_count | label / 描述 | fields 集合 | fields 顺序 | 必填配置 |
|---|---|---|---|---|
| `= 0` | 可改 | 可加可减 | 可调 | 可改 |
| `> 0` | 可改 | **锁死（41008）**| **锁死（41008）**| **锁死（41008）**|

> 顺序变更和集合变更使用同一个错误码 —— 它们共属于"字段勾选"语义。Service 层 diff 时按"集合 + 数组逐位"双重比较，集合相同但顺序不同也视为变更。

---

## 关键查询

### 列表（覆盖索引，不回表）

```sql
-- 模板管理页（不传 enabled）
SELECT id, name, label, ref_count, enabled, created_at
FROM templates WHERE deleted = 0
  AND label LIKE CONCAT('%', ?, '%')   -- 可选
ORDER BY id DESC LIMIT 20 OFFSET 0;

-- NPC 管理页（enabled=true）
SELECT id, name, label, ref_count, enabled, created_at
FROM templates WHERE deleted = 0 AND enabled = 1
ORDER BY id DESC LIMIT 20 OFFSET 0;
```

两个查询都被 `idx_list (deleted, id, name, label, ref_count, enabled, created_at)` 覆盖。

### 详情（主键查询 + 批量字段补全）

```sql
SELECT * FROM templates WHERE id = ? AND deleted = 0;
-- 解 fields JSON 拿 field_ids
SELECT id, name, label, type, category, enabled FROM fields WHERE id IN (?) AND deleted = 0;
```

### 引用计数检查（事务内 FOR SHARE）

```sql
SELECT ref_count FROM templates WHERE id = ? AND deleted = 0 FOR SHARE;
```

> 模板的 `ref_count` 是 NPC 模块维护的（NPC 创建/删除时增减）。模板管理只读这个字段做"能不能删"的判断，不直接操作 NPC 表。

### 按 ID 批量查模板（给字段管理用）

```sql
SELECT id, name, label FROM templates WHERE id IN (?) AND deleted = 0;
```

字段管理的 `GetReferences` 返回引用方时，模板 label 通过这个方法补全（解 features.md 里集成项 #2 的 TODO）。

---

## 业务逻辑

### 创建流程

```
service.Create:
1. Handler 层校验通过后进入
2. 业务校验:
   a. name 唯一性（含软删除）→ 存在则返回 41001
   b. fields 非空 → 否则返回 41004
   c. 批量查 fields 表确认所有 field_id 存在 → 不存在返回 41006
   d. 所有勾选字段必须 enabled=1 → 否则返回 41005
3. 事务内:
   a. INSERT templates 行（enabled=0, version=1）
   b. 对每个字段: field_refs.Add(tx, field_id, "template", template_id)
   c. 对每个字段: fields.IncrRefCountTx(tx, field_id)
4. 清缓存: INCR templates 列表版本号 + 级联清字段方 detail
5. 返回 lastInsertId
```

### 编辑流程

```
service.Update:
1. 按 ID 查模板 → 不存在返回 41003
2. 校验 enabled=0 → 否则返回 41010
3. 业务校验:
   a. fields 非空 → 否则返回 41004
   b. fields 内 field_id 不重复（防御性，前端已去重）
   c. ref_count > 0 时:
      - diff(oldFields, newFields)
      - 若集合或顺序变更 → 返回 41008
      - 仅 label/description 变化 → 允许
   d. ref_count = 0 时:
      - 批量查新增字段确认存在 → 否则 41006
      - 新增字段必须 enabled=1 → 否则 41005
4. 事务内:
   a. UPDATE templates SET ... WHERE id=? AND version=?
      → rows=0 返回 41011
   b. ref_count = 0 且字段集合变更时:
      - toRemove: field_refs.Remove + fields.DecrRefCountTx
      - toAdd:    field_refs.Add    + fields.IncrRefCountTx
5. 清缓存: DEL detail:{id} + INCR list version + 级联清字段方 detail
```

**fields diff 算法：**

```go
type FieldEntry struct {
    FieldID  int64
    Required bool
}

// 集合或顺序变化都视为字段变更
func isFieldsChanged(old, new []FieldEntry) bool {
    if len(old) != len(new) {
        return true
    }
    for i := range old {
        if old[i].FieldID != new[i].FieldID {
            return true  // 顺序或集合变化
        }
    }
    return false
}

// 必填变化不算字段变更（被引用模板也允许改必填? 不允许 — required 也属于 fields，
// 与顺序、集合一起被 41008 拦截）
func isRequiredChanged(old, new []FieldEntry) bool {
    // 长度和顺序已经相同时才会进入这个比较
    for i := range old {
        if old[i].Required != new[i].Required {
            return true
        }
    }
    return false
}
```

> **必填标记的变更也归到 41008** —— 被 NPC 引用的模板，整个 fields JSON 都锁死，包括 required。features.md 的"字段勾选完全锁死"是广义的"fields 列锁死"。

### 删除流程

```
service.Delete:
1. 按 ID 查模板 → 不存在返回 41003
2. 校验 enabled=0 → 否则返回 41009
3. 事务内:
   a. SELECT ref_count FROM templates WHERE id=? FOR SHARE
      → ref_count > 0 返回 41007
   b. UPDATE templates SET deleted=1 WHERE id=?
   c. 解出旧 fields 的 field_id 列表
   d. 对每个字段:
      - field_refs.Remove(tx, field_id, "template", template_id)
      - fields.DecrRefCountTx(tx, field_id)
4. 清缓存: DEL detail:{id} + INCR list version + 级联清字段方 detail
```

**FOR SHARE 防 TOCTOU**：删除前先在事务内锁住 templates 行的 ref_count，避免"读时无引用 → 写前 NPC 模块刚好新建了一个引用 → 仍然删除"的竞态。

### 启用/停用切换

```
service.ToggleEnabled:
1. 按 ID 查模板 → 不存在返回 41003
2. UPDATE templates SET enabled=?, version=version+1, updated_at=NOW()
   WHERE id=? AND version=? AND deleted=0
   → rows=0 返回 41011
3. 清缓存: DEL detail:{id} + INCR list version
```

> 启用/停用没有"先校验引用数"的约束 —— 停用一个被 NPC 引用的模板是允许的（"存量不动，增量拦截"）。

### 引用详情（NPC 模块未上线前占位）

```
service.GetReferences:
1. 按 ID 查模板 → 不存在返回 41003
2. NPC 模块上线前: 直接返回 { template_id, template_label, npcs: [] }
3. NPC 模块上线后: SELECT id, name FROM npcs WHERE template_id=? AND deleted=0
```

实现要点：在 `TemplateService` 里预留一个 `npcStore NPCStoreInterface`（接口），当前版本注入空实现，NPC 模块上线时替换成真实实现。这样模板管理代码不用改。

---

## 缓存策略

### Key 设计

| Key 模式 | TTL | 用途 |
|----------|-----|------|
| `templates:detail:{id}` | 5min + jitter | 单条详情缓存（含字段精简列表） |
| `templates:list:v{ver}:{label}:{enabled}:{page}:{pageSize}` | 1min + jitter | 分页列表缓存 |
| `templates:list:version` | 无 TTL | 列表缓存版本号 |
| `templates:lock:{id}` | 3s | 分布式锁（防缓存击穿） |

### 写入失效

| 操作 | 缓存动作 |
|------|---------|
| 创建 | INCR list version + 级联 DEL 字段方 detail |
| 编辑 | DEL detail:{id} + INCR list version + 级联 DEL 字段方 detail |
| 删除 | DEL detail:{id} + INCR list version + 级联 DEL 字段方 detail |
| 切换启用 | DEL detail:{id} + INCR list version |

### 详情缓存的特殊性

`templates:detail:{id}` 缓存的不是裸 templates 行，而是已经补全了字段精简信息的 `TemplateDetail`。这意味着：

- **字段被编辑/停用时也要清模板缓存**：字段管理在编辑/停用字段时，需要查 `field_refs WHERE field_id=? AND ref_type='template'` 拿到所有引用该字段的模板 ID，然后清这些模板的 detail 缓存。
- **简化方案**：字段管理目前已经做了"清字段方 detail"的级联失效，对应到模板管理就是清 `templates:detail:{id}`。需要在字段管理 service 里增加一个 hook 或者让模板 store 暴露一个 `InvalidateDetailByFieldID` 方法。

### 降级策略

Redis 不可用时全部降级到 MySQL 直查，不阻塞业务。

---

## 并发安全

| 场景 | 机制 |
|------|------|
| 编辑冲突 | 乐观锁 `WHERE id=? AND version=?` |
| 缓存击穿 | 分布式锁 `SetNX templates:lock:{id}` + double-check |
| 缓存穿透 | 空标记 `{"_null":true}` |
| 删除 TOCTOU | 事务内 `SELECT ref_count FOR SHARE` |
| 启用/停用冲突 | 乐观锁 |
| 同时编辑同一模板 | 乐观锁 → 后到者收到 41011 |
| field_refs 写入 | 字段管理已用 INSERT IGNORE，模板写入沿用同样的幂等性 |

---

## Store 层方法清单

### TemplateStore

| 方法 | 说明 |
|------|------|
| `Create(ctx, req) (int64, error)` | INSERT，返回 lastInsertId |
| `GetByID(ctx, id) (*Template, error)` | 主键查询（不补全字段信息）|
| `GetByName(ctx, name) (*Template, error)` | uk_name 查询（check-name 用） |
| `ExistsByName(ctx, name) (bool, error)` | 含软删除检查 |
| `List(ctx, query) ([]TemplateListItem, int64, error)` | 覆盖索引 |
| `Update(ctx, req) error` | 乐观锁 WHERE id=? AND version=? |
| `SoftDeleteTx(tx, id) error` | 事务内软删除 |
| `ToggleEnabled(ctx, id, enabled, version) error` | 乐观锁切换 |
| `IncrRefCountTx(tx, id) error` | 事务内 +1（NPC 模块创建时调用） |
| `DecrRefCountTx(tx, id) error` | 事务内 -1（NPC 模块删除时调用） |
| `GetRefCountTx(tx, id) (int, error)` | FOR SHARE，删除前防 TOCTOU |
| `GetByIDs(ctx, ids) ([]TemplateListItem, error)` | IN 查询，**给字段管理 GetReferences 补 label 用** |

### TemplateService

| 方法 | 说明 |
|------|------|
| `List(ctx, query) (*ListData, error)` | 列表 + 缓存 |
| `Create(ctx, req) (int64, error)` | 创建 + 维护 field_refs |
| `GetByID(ctx, id) (*TemplateDetail, error)` | 详情 + 批量补全字段 + 缓存 |
| `Update(ctx, req) error` | 编辑 + diff fields + 维护 field_refs |
| `Delete(ctx, id) (*DeleteResult, error)` | 软删除 + 清理 field_refs |
| `CheckName(ctx, name) (*CheckNameResult, error)` | 唯一性校验 |
| `GetReferences(ctx, id) (*TemplateReferenceDetail, error)` | NPC 引用列表（占位实现） |
| `ToggleEnabled(ctx, req) error` | 启用/停用切换 |

### TemplateCache

| 方法 | 说明 |
|------|------|
| `GetList(ctx, query) (*TemplateListData, hit, error)` | 列表缓存 |
| `SetList(ctx, query, data) error` | 写列表缓存 |
| `GetDetail(ctx, id) (*TemplateDetail, hit, error)` | 详情缓存 |
| `SetDetail(ctx, id, detail) error` | 写详情缓存 |
| `DelDetail(ctx, id) error` | 清单条详情 |
| `InvalidateList(ctx) error` | INCR list version |
| `TryLock(ctx, id, ttl) (locked, error)` | 分布式锁 |
| `Unlock(ctx, id) error` | 释放锁 |

---

## 跨模块集成点

| 对接模块 | 集成内容 | 状态 |
|---------|---------|------|
| **字段管理** | 调用 `FieldRefStore.Add/Remove` 维护引用关系；调用 `FieldStore.IncrRefCountTx/DecrRefCountTx` 维护字段的 ref_count；详情接口调用 `FieldStore.GetByIDs` 批量补全字段信息 | ✅ 字段管理已实现 |
| **字段管理（反向）** | 暴露 `TemplateStore.GetByIDs(ctx, ids)`，让字段管理的 `GetReferences` 接口能补全模板 label，解 `backend/internal/service/field.go:331-337` 的 TODO | ⏳ 模板管理实现时一起做 |
| **字段管理（缓存级联）** | 字段管理在编辑/停用字段时，需要清所有引用该字段的模板 detail 缓存（避免模板详情显示过期的字段信息）。需要约定一个 hook 或共享 invalidator | ⏳ 模板管理上线后补 |
| **NPC 管理（未来）** | NPC 创建/删除时调用 `TemplateStore.IncrRefCountTx/DecrRefCountTx`；NPC 表存 `template_id` 而非 name；导出 `npc_templates` 时按模板的 fields 顺序写 MongoDB；支持反查 `NPCStore.GetByTemplateID(template_id)` 给引用详情接口 | ⏳ NPC 模块上线时对接 |
| **dictionaries** | 详情接口的 `category_label` 走 `DictCache.GetLabel("field_category", category)` 翻译，与字段管理保持一致 | ✅ 已就绪 |

---

## 关键差异（与字段管理对照）

| 维度 | 字段管理 | 模板管理 |
|---|---|---|
| 唯一表 | `fields` | `templates` |
| 引用关系 | 写 `field_refs (ref_type='field')` | 写 `field_refs (ref_type='template')` |
| ref_count 含义 | 被多少模板/reference 字段引用 | 被多少 NPC 引用 |
| ref_count 维护方 | 字段管理自己（创建/编辑/删除模板时）+ 字段管理自己（创建/编辑/删除 reference 字段时）| **NPC 模块**（NPC 创建/删除时增减），模板管理只读不写 |
| 详情缓存内容 | 裸字段行 | 模板行 + 字段精简列表（已展开） |
| 子表查询 | 无（properties 是 JSON） | 详情时查 fields 表批量补全 |
| 数组顺序 | 无意义 | **fields JSON 数组顺序 = NPC 表单展示顺序** |
| 编辑限制 | 启用中禁编辑 + 引用中禁改类型/收紧 | 启用中禁编辑 + 引用中禁改 fields（含顺序+必填）|

---

## 已知限制（实现时确认）

| 限制 | 说明 | 计划 |
|------|------|------|
| 引用详情占位 | NPC 模块未上线前，`/templates/references` 永远返回空数组 | NPC 模块上线时填充 |
| 缓存级联 hook 待约定 | 字段编辑/停用时清模板 detail 缓存的机制，需要字段管理改一行调用 | 模板管理实现时同步加 |
| 字段管理的 TemplateStore 注入 | `field.go:331-337` 当前用占位 label，模板管理上线后注入真实 store | 模板管理实现时一并解决 |
