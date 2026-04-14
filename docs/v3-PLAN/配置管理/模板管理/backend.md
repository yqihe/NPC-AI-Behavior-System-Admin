# 模板管理 — 后端设计

> 通用架构/规范见 `docs/architecture/overview.md` 和 `docs/development/`。
> 本文档只记录模板管理模块的实现事实与特有约束。

---

## 1. 目录结构

```
backend/internal/
├── handler/template.go          # HTTP 入口 + 跨模块事务编排 + 拼装 TemplateDetail
├── service/template.go          # 业务逻辑 + Cache-Aside（只缓存 *model.Template 裸行）+ 对外接口
├── store/
│   ├── mysql/template.go        # templates 表 CRUD + 覆盖索引 + 乐观锁
│   └── redis/template_cache.go  # TemplateCache（Detail / List / Lock）
├── model/template.go            # Template / TemplateFieldEntry / TemplateListItem / TemplateDetail / TemplateFieldItem / TemplateLite / DTO
├── errcode/codes.go             # 41001-41012
└── router/router.go             # POST /api/v1/templates/* 路由注册
```

**存储范围**：

- **MySQL**：唯一写入目标
- **Redis**：`*model.Template` 裸行 detail 缓存 + 列表缓存 + 分布式锁
- **MongoDB / RabbitMQ**：不涉及（模板自身不产生导出数据，NPC 配置层才产生 `npc_templates` 集合）

---

## 2. 数据表

### templates

```sql
CREATE TABLE IF NOT EXISTS templates (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    name            VARCHAR(64)  NOT NULL,              -- 模板标识，唯一，创建后不可变
    label           VARCHAR(128) NOT NULL,              -- 中文标签（搜索用）
    description     VARCHAR(512) NOT NULL DEFAULT '',   -- 描述（可选）
    fields          JSON         NOT NULL,              -- [{field_id, required}, ...] 数组顺序=NPC 表单展示顺序

    enabled         TINYINT(1)   NOT NULL DEFAULT 0,    -- 启用状态（创建默认 0，给"配置窗口期"）
    version         INT          NOT NULL DEFAULT 1,    -- 乐观锁
    deleted         TINYINT(1)   NOT NULL DEFAULT 0,    -- 软删除
    created_at      DATETIME     NOT NULL,
    updated_at      DATETIME     NOT NULL,

    -- name 唯一约束不带 deleted：已删除的标识也不能复用，
    -- 防止历史 NPC 引用混乱（详见 features.md 功能 5）
    UNIQUE KEY uk_name (name),
    -- 覆盖索引：列表查询按 id DESC 扫描，含 enabled / label 用于筛选
    INDEX idx_list (deleted, id, name, label, enabled, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**索引说明**：

| 索引 | 用途 |
|---|---|
| `uk_name (name)` | 唯一约束，**不带 deleted**——已删除的 name 永久不可复用，防历史 NPC 引用混乱 |
| `idx_list (deleted, id, name, label, enabled, created_at)` | 覆盖索引，列表 SQL `ORDER BY id DESC` 不回表（不含 `fields / description`） |

**字段引用关系**：

`templates` 表不持有 `field_refs`。模板对字段的引用关系由 `field_refs(ref_type='template', ref_id=<template_id>)` 记录，跨模块事务内由 handler 调 `FieldService.AttachToTemplateTx / DetachFromTemplateTx` 维护。

---

## 3. 数据模型

### Template（templates 表整行）

```go
type Template struct {
    ID          int64           `json:"id" db:"id"`
    Name        string          `json:"name" db:"name"`
    Label       string          `json:"label" db:"label"`
    Description string          `json:"description" db:"description"`
    Fields      json.RawMessage `json:"fields" db:"fields"` // [{field_id, required}, ...]
    Enabled     bool            `json:"enabled" db:"enabled"`
    Version     int             `json:"version" db:"version"`
    Deleted     bool            `json:"-" db:"deleted"`
    CreatedAt   time.Time       `json:"created_at" db:"created_at"`
    UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
}
```

### TemplateFieldEntry（fields JSON 数组单元）

```go
type TemplateFieldEntry struct {
    FieldID  int64 `json:"field_id"`
    Required bool  `json:"required"`
}
```

### TemplateListItem（列表项，覆盖索引返回）

```go
type TemplateListItem struct {
    ID        int64     `json:"id" db:"id"`
    Name      string    `json:"name" db:"name"`
    Label     string    `json:"label" db:"label"`
    Enabled   bool      `json:"enabled" db:"enabled"`
    CreatedAt time.Time `json:"created_at" db:"created_at"`
}
```

### TemplateLite（跨模块精简结构）

```go
type TemplateLite struct {
    ID    int64  `json:"id" db:"id"`
    Name  string `json:"name" db:"name"`
    Label string `json:"label" db:"label"`
}
```

### TemplateDetail（详情响应，handler 层拼装，不进缓存）

```go
type TemplateDetail struct {
    ID          int64               `json:"id"`
    Name        string              `json:"name"`
    Label       string              `json:"label"`
    Description string              `json:"description"`
    Enabled     bool                `json:"enabled"`
    Version     int                 `json:"version"`
    CreatedAt   time.Time           `json:"created_at"`
    UpdatedAt   time.Time           `json:"updated_at"`
    Fields      []TemplateFieldItem `json:"fields"` // 顺序与 templates.fields JSON 数组一致
}
```

### TemplateFieldItem（详情中的字段精简信息）

```go
type TemplateFieldItem struct {
    FieldID       int64  `json:"field_id"`
    Name          string `json:"name"`
    Label         string `json:"label"`
    Type          string `json:"type"`
    Category      string `json:"category"`
    CategoryLabel string `json:"category_label"` // dictionary 翻译
    Enabled       bool   `json:"enabled"`        // 字段当前是否启用（用于前端标灰停用字段）
    Required      bool   `json:"required"`       // 模板里的必填配置
}
```

### DTO（请求/响应）

```go
type TemplateListQuery struct {
    Label    string `json:"label"`
    Enabled  *bool  `json:"enabled,omitempty"` // nil=不筛选, true=仅启用
    Page     int    `json:"page"`
    PageSize int    `json:"page_size"`
}

type CreateTemplateRequest struct {
    Name        string               `json:"name"`
    Label       string               `json:"label"`
    Description string               `json:"description"`
    Fields      []TemplateFieldEntry `json:"fields"`
}

type CreateTemplateResponse struct {
    ID   int64  `json:"id"`
    Name string `json:"name"`
}

type UpdateTemplateRequest struct {
    ID          int64                `json:"id"`
    Label       string               `json:"label"`
    Description string               `json:"description"`
    Fields      []TemplateFieldEntry `json:"fields"`
    Version     int                  `json:"version"`
}

type TemplateReferenceDetail struct {
    TemplateID    int64                   `json:"template_id"`
    TemplateLabel string                  `json:"template_label"`
    NPCs          []TemplateReferenceItem `json:"npcs"`
}

type TemplateReferenceItem struct {
    NPCID   int64  `json:"npc_id"`
    NPCName string `json:"npc_name"`
}
```

---

## 4. API 接口

所有操作使用主键 ID（BIGINT）。`name` 只出现在创建请求体/响应、`/check-name` 校验、跨模块 `GetByIDsLite` 返回值。

| Method | Path | 请求体 | 响应体 | 错误码 |
|---|---|---|---|---|
| POST | `/api/v1/templates/list` | `{label?, enabled?, page, page_size}` | `{items: TemplateListItem[], total, page, page_size}` | -- |
| POST | `/api/v1/templates/create` | `{name, label, description?, fields: [{field_id, required}]}` | `{id, name}` | 41001, 41002, 41004, 41005, 41006, 41012 |
| POST | `/api/v1/templates/detail` | `{id}` | `TemplateDetail` | 41003 |
| POST | `/api/v1/templates/update` | `{id, label, description?, fields, version}` | `"保存成功"` | 41004, 41005, 41006, 41010, 41011, 41012 |
| POST | `/api/v1/templates/delete` | `{id}` | `{id, name, label}` | 41003, 41009 |
| POST | `/api/v1/templates/check-name` | `{name}` | `{available, message}` | 41002 |
| POST | `/api/v1/templates/references` | `{id}` | `{template_id, template_label, npcs: []}` | 41003 |
| POST | `/api/v1/templates/toggle-enabled` | `{id, enabled, version}` | `"操作成功"` | 41003, 41011 |

**跨模块事务编排**（handler 层开启事务，调 templateService + fieldService 协同完成）：

- **Create**：格式校验 -> `ExistsByName` -> `fieldService.ValidateFieldsForTemplate` -> tx: `CreateTx` + `AttachToTemplateTx` -> commit -> 清两方缓存
- **Update**：格式校验 -> `GetByID` + `ParseFieldEntries` -> 事务前预校验新增字段 -> tx: `UpdateTx`（enabled/diff/写 templates）+ 条件 `DetachFromTemplateTx` / `AttachToTemplateTx` -> commit -> 清两方缓存
- **Delete**：`GetByID` + enabled 校验 -> `ParseFieldEntries` -> tx: `SoftDeleteTx` + `DetachFromTemplateTx` -> commit -> 清两方缓存
- **Detail**：`GetByID`（裸行 cache-aside）-> `ParseFieldEntries` -> `fieldService.GetByIDsLite` -> handler 拼装 `TemplateDetail`（不缓存拼装结果）

---

## 5. 方法签名

### TemplateHandler

```go
type TemplateHandler struct {
    db              *sqlx.DB
    templateService *service.TemplateService
    fieldService    *service.FieldService
    valCfg          *config.ValidationConfig
}

func NewTemplateHandler(db *sqlx.DB, templateService *service.TemplateService, fieldService *service.FieldService, valCfg *config.ValidationConfig) *TemplateHandler

// 单模块路径
func (h *TemplateHandler) List(ctx context.Context, q *model.TemplateListQuery) (*model.ListData, error)
func (h *TemplateHandler) CheckName(ctx context.Context, req *model.CheckNameRequest) (*model.CheckNameResult, error)
func (h *TemplateHandler) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) (*string, error)
func (h *TemplateHandler) GetReferences(ctx context.Context, req *model.IDRequest) (*model.TemplateReferenceDetail, error)

// 跨模块路径（含事务编排）
func (h *TemplateHandler) Create(ctx context.Context, req *model.CreateTemplateRequest) (*model.CreateTemplateResponse, error)
func (h *TemplateHandler) Get(ctx context.Context, req *model.IDRequest) (*model.TemplateDetail, error)
func (h *TemplateHandler) Update(ctx context.Context, req *model.UpdateTemplateRequest) (*string, error)
func (h *TemplateHandler) Delete(ctx context.Context, req *model.IDRequest) (*model.DeleteResult, error)
```

### TemplateService

```go
type TemplateService struct {
    store  *storemysql.TemplateStore
    cache  *storeredis.TemplateCache
    pagCfg *config.PaginationConfig
}

func NewTemplateService(store *storemysql.TemplateStore, cache *storeredis.TemplateCache, pagCfg *config.PaginationConfig) *TemplateService

// 单模块路径
func (s *TemplateService) List(ctx context.Context, q *model.TemplateListQuery) (*model.ListData, error)
func (s *TemplateService) GetByID(ctx context.Context, id int64) (*model.Template, error)
func (s *TemplateService) ExistsByName(ctx context.Context, name string) (bool, error)
func (s *TemplateService) CheckName(ctx context.Context, name string) (*model.CheckNameResult, error)
func (s *TemplateService) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) error
func (s *TemplateService) ParseFieldEntries(raw json.RawMessage) ([]model.TemplateFieldEntry, error)

// 跨模块路径（接收外部 tx）
func (s *TemplateService) CreateTx(ctx context.Context, tx *sqlx.Tx, req *model.CreateTemplateRequest) (int64, error)
func (s *TemplateService) UpdateTx(ctx context.Context, tx *sqlx.Tx, req *model.UpdateTemplateRequest, old *model.Template, oldEntries []model.TemplateFieldEntry) (fieldsChanged bool, toAdd []int64, toRemove []int64, err error)
func (s *TemplateService) SoftDeleteTx(ctx context.Context, tx *sqlx.Tx, id int64) error

// 缓存失效（commit 后由 handler 调用）
func (s *TemplateService) InvalidateDetail(ctx context.Context, id int64)
func (s *TemplateService) InvalidateList(ctx context.Context)

// 跨模块对外查询接口
func (s *TemplateService) GetByIDsLite(ctx context.Context, ids []int64) ([]model.TemplateLite, error)
func (s *TemplateService) DB() *sqlx.DB
```

### TemplateStore

```go
type TemplateStore struct {
    db *sqlx.DB
}

func NewTemplateStore(db *sqlx.DB) *TemplateStore

func (s *TemplateStore) CreateTx(ctx context.Context, tx *sqlx.Tx, req *model.CreateTemplateRequest, fieldsJSON []byte) (int64, error)
func (s *TemplateStore) GetByID(ctx context.Context, id int64) (*model.Template, error)
func (s *TemplateStore) ExistsByName(ctx context.Context, name string) (bool, error)
func (s *TemplateStore) List(ctx context.Context, q *model.TemplateListQuery) ([]model.TemplateListItem, int64, error)
func (s *TemplateStore) UpdateTx(ctx context.Context, tx *sqlx.Tx, req *model.UpdateTemplateRequest, fieldsJSON []byte) error
func (s *TemplateStore) SoftDeleteTx(ctx context.Context, tx *sqlx.Tx, id int64) error
func (s *TemplateStore) ToggleEnabled(ctx context.Context, id int64, enabled bool, version int) error
func (s *TemplateStore) GetByIDs(ctx context.Context, ids []int64) ([]model.TemplateLite, error)
func (s *TemplateStore) DB() *sqlx.DB
```

### TemplateCache

```go
type TemplateCache struct {
    rdb *redis.Client
}

func NewTemplateCache(rdb *redis.Client) *TemplateCache

func (c *TemplateCache) GetDetail(ctx context.Context, id int64) (*model.Template, bool, error)
func (c *TemplateCache) SetDetail(ctx context.Context, id int64, tpl *model.Template)
func (c *TemplateCache) DelDetail(ctx context.Context, id int64)
func (c *TemplateCache) GetList(ctx context.Context, q *model.TemplateListQuery) (*model.TemplateListData, bool, error)
func (c *TemplateCache) SetList(ctx context.Context, q *model.TemplateListQuery, list *model.TemplateListData)
func (c *TemplateCache) InvalidateList(ctx context.Context)
func (c *TemplateCache) TryLock(ctx context.Context, id int64, expire time.Duration) (bool, error)
func (c *TemplateCache) Unlock(ctx context.Context, id int64)
```

---

## 6. 跨模块事务编排详细流程

### 6.1 Create（创建模板）

```
Handler.Create(ctx, req)
│
├─ 1. 格式校验: checkTemplateName / checkTemplateLabel / checkDescription / checkTemplateFields
│
├─ 2. 事务外预检
│   ├─ templateService.ExistsByName(ctx, name)        → 41001 if exists
│   └─ fieldService.ValidateFieldsForTemplate(ctx, fieldIDs) → 41005/41006/41012
│
├─ 3. h.db.BeginTxx(ctx, nil)  +  defer tx.Rollback()
│   ├─ templateService.CreateTx(ctx, tx, req)
│   │   ├─ validateFieldsBasic(req.Fields)            → 41004 / ErrBadRequest
│   │   ├─ store.ExistsByName(ctx, name)              → 41001 (兜底)
│   │   ├─ json.Marshal(req.Fields)
│   │   └─ store.CreateTx(ctx, tx, req, fieldsJSON)   → INSERT templates
│   │
│   └─ fieldService.AttachToTemplateTx(ctx, tx, templateID, fieldIDs)
│       └─ fieldRefStore.Add(ctx, tx, ...)             → INSERT field_refs
│
├─ 4. tx.Commit()
│
└─ 5. 清缓存
    ├─ templateService.InvalidateList(ctx)              → INCR templates:list:version
    └─ fieldService.InvalidateDetails(ctx, affected)    → DEL fields:detail:{id}...
```

### 6.2 Update（编辑模板）

```
Handler.Update(ctx, req)
│
├─ 1. 格式校验: checkID / checkTemplateLabel / checkDescription / checkTemplateFields / checkVersion
│
├─ 2. 拿旧状态
│   ├─ templateService.GetByID(ctx, req.ID)            → *Template (cache-aside)
│   └─ templateService.ParseFieldEntries(old.Fields)   → []TemplateFieldEntry
│
├─ 3. 事务外预校验新增字段
│   ├─ diffNewFieldIDs(oldEntries, req.Fields)         → toAddPre
│   └─ fieldService.ValidateFieldsForTemplate(ctx, toAddPre) → 41005/41006/41012
│
├─ 4. h.db.BeginTxx(ctx, nil)  +  defer tx.Rollback()
│   ├─ templateService.UpdateTx(ctx, tx, req, old, oldEntries)
│   │   ├─ validateFieldsBasic(req.Fields)             → 41004 / ErrBadRequest
│   │   ├─ old.Enabled == true?                        → 41010
│   │   ├─ isFieldsChanged(oldEntries, req.Fields)     → fieldsChanged
│   │   ├─ diffFieldIDs(oldEntries, req.Fields)        → toAdd, toRemove
│   │   ├─ json.Marshal(req.Fields)
│   │   └─ store.UpdateTx(ctx, tx, req, fieldsJSON)   → UPDATE WHERE id=? AND version=?
│   │       └─ rows == 0?                              → 41011
│   │
│   └─ if fieldsChanged && (len(toAdd) > 0 || len(toRemove) > 0):
│       ├─ fieldService.DetachFromTemplateTx(ctx, tx, req.ID, toRemove)
│       └─ fieldService.AttachToTemplateTx(ctx, tx, req.ID, toAdd)
│
├─ 5. tx.Commit()
│
└─ 6. 清缓存
    ├─ templateService.InvalidateDetail(ctx, req.ID)   → DEL templates:detail:{id}
    ├─ templateService.InvalidateList(ctx)              → INCR templates:list:version
    ├─ fieldService.InvalidateDetails(ctx, detachAffected)
    └─ fieldService.InvalidateDetails(ctx, attachAffected)
```

### 6.3 Delete（删除模板）

```
Handler.Delete(ctx, req)
│
├─ 1. 校验: checkID
│
├─ 2. 前置校验
│   ├─ templateService.GetByID(ctx, req.ID)            → *Template
│   ├─ tpl.Enabled == true?                            → 41009
│   └─ templateService.ParseFieldEntries(tpl.Fields)   → fieldIDs
│
├─ 3. h.db.BeginTxx(ctx, nil)  +  defer tx.Rollback()
│   ├─ templateService.SoftDeleteTx(ctx, tx, req.ID)
│   │   └─ store.SoftDeleteTx: UPDATE templates SET deleted=1 WHERE id=? AND deleted=0
│   │       └─ rows == 0?                              → ErrNotFound → 41003
│   │
│   └─ fieldService.DetachFromTemplateTx(ctx, tx, req.ID, fieldIDs)
│       └─ fieldRefStore.Remove(ctx, tx, ...)          → DELETE field_refs
│
├─ 4. tx.Commit()
│
└─ 5. 清缓存
    ├─ templateService.InvalidateDetail(ctx, req.ID)   → DEL templates:detail:{id}
    ├─ templateService.InvalidateList(ctx)              → INCR templates:list:version
    └─ fieldService.InvalidateDetails(ctx, affected)   → DEL fields:detail:{id}...
```

### 6.4 Detail（模板详情，跨模块拼装）

```
Handler.Get(ctx, req)
│
├─ 1. 校验: checkID
│
├─ 2. templateService.GetByID(ctx, req.ID)             → *Template (cache-aside)
│   ├─ cache.GetDetail(ctx, id)                        → hit? return
│   ├─ cache.TryLock(ctx, id, 3s) + double-check      → 防击穿
│   ├─ store.GetByID(ctx, id)                          → MySQL
│   └─ cache.SetDetail(ctx, id, tpl)                   → nil 写空标记防穿透
│
├─ 3. templateService.ParseFieldEntries(tpl.Fields)    → []TemplateFieldEntry
│
├─ 4. fieldService.GetByIDsLite(ctx, fieldIDs)         → []FieldLite (按顺序对齐)
│
└─ 5. 拼装 TemplateDetail
    └─ 按 entries 顺序: FieldLite + Required → []TemplateFieldItem
       缺失字段 slog.Warn 跳过
```

---

## 7. 关键 SQL

### CreateTx

```sql
INSERT INTO templates (name, label, description, fields, enabled, version, deleted, created_at, updated_at)
VALUES (?, ?, ?, ?, 0, 1, 0, ?, ?)
```

### GetByID

```sql
SELECT id, name, label, description, fields, enabled, version, deleted, created_at, updated_at
FROM templates WHERE id = ? AND deleted = 0
```

### ExistsByName

```sql
-- 含软删除记录，已删除的 name 不可复用
SELECT COUNT(*) FROM templates WHERE name = ?
```

### List

```sql
-- 计数
SELECT COUNT(*) FROM templates WHERE deleted = 0 [AND label LIKE ?] [AND enabled = ?]

-- 分页（覆盖索引 idx_list，不回表）
SELECT id, name, label, enabled, created_at
FROM templates WHERE deleted = 0 [AND label LIKE ?] [AND enabled = ?]
ORDER BY id DESC LIMIT ? OFFSET ?
```

### UpdateTx

```sql
UPDATE templates SET label = ?, description = ?, fields = ?, version = version + 1, updated_at = ?
WHERE id = ? AND version = ? AND deleted = 0
```

### SoftDeleteTx

```sql
UPDATE templates SET deleted = 1, updated_at = ?
WHERE id = ? AND deleted = 0
```

### ToggleEnabled

```sql
UPDATE templates SET enabled = ?, version = version + 1, updated_at = ?
WHERE id = ? AND version = ? AND deleted = 0
```

### GetByIDs（跨模块精简查询）

```sql
SELECT id, name, label FROM templates WHERE id IN (?) AND deleted = 0
```

---

## 8. 缓存策略

| 层 | Key 模式 | TTL | 防护机制 |
|---|---|---|---|
| detail | `templates:detail:{id}` | 5min + 0-30s jitter | 分布式锁 `templates:lock:{id}`（3s）+ double-check + 空标记 `{"_null":true}` |
| list | `templates:list:v{N}:{label}:{enabled}:{page}:{ps}` | 1min + 0-10s jitter | 版本号 `templates:list:version`（INCR 一次所有变体失效） |
| 拼装后 TemplateDetail | **不缓存** | -- | handler 每次从两方 cache 分别取裸行 + 字段精简后拼装 |

**不缓存 TemplateDetail 的原因**：`FieldLite.Enabled` 反映字段当前状态，如果缓存拼装后的详情到模板方，字段被停用时就得同时清模板详情缓存，耦合链太长。分层做法：模板方缓存裸行，字段方有自己的 detail 缓存，拼装在 handler 层每次发生。

**失效时机**：

| 操作 | 清模板 detail | 清模板 list | 清字段 details |
|---|---|---|---|
| Create | -- | INCR version | affected fieldIDs |
| Update | DEL detail:{id} | INCR version | detach + attach affected |
| Delete | DEL detail:{id} | INCR version | affected fieldIDs |
| ToggleEnabled | DEL detail:{id} | INCR version | -- |

---

## 9. 错误码

| 错误码 | 常量 | 触发场景 |
|---|---|---|
| 41001 | `ErrTemplateNameExists` | 创建时 name 已存在（含软删除记录，`ExistsByName` 不过滤 deleted） |
| 41002 | `ErrTemplateNameInvalid` | name 为空 / 不匹配 `^[a-z][a-z0-9_]*$` / 超长 |
| 41003 | `ErrTemplateNotFound` | `GetByID` / `SoftDeleteTx` 查不到未删除记录 |
| 41004 | `ErrTemplateNoFields` | 创建或编辑时 fields 数组为空 |
| 41005 | `ErrTemplateFieldDisabled` | `fieldService.ValidateFieldsForTemplate` 发现勾选了停用字段 |
| 41006 | `ErrTemplateFieldNotFound` | `fieldService.ValidateFieldsForTemplate` 发现勾选的字段不存在 |
| 41007 | `ErrTemplateRefDelete` | 被 NPC 引用，无法删除（错误码已注册，NPC 模块上线后启用） |
| 41008 | `ErrTemplateRefEditFields` | 被 NPC 引用，字段列表不可修改（错误码已注册，NPC 模块上线后启用） |
| 41009 | `ErrTemplateDeleteNotDisabled` | 删除时 `tpl.Enabled == true`，必须先停用 |
| 41010 | `ErrTemplateEditNotDisabled` | 编辑时 `old.Enabled == true`，必须先停用 |
| 41011 | `ErrTemplateVersionConflict` | `UpdateTx` / `ToggleEnabled` 乐观锁 `WHERE version=?` 命中 0 行 |
| 41012 | `ErrTemplateFieldIsReference` | `fieldService.ValidateFieldsForTemplate` 发现勾选了 reference 类型字段（必须展开为 leaf 子字段后加入） |

---

## 10. 内部 diff 算法

### isFieldsChanged

集合 + 顺序 + required 任一不同都视为变更：

```go
func isFieldsChanged(old, new []model.TemplateFieldEntry) bool {
    if len(old) != len(new) { return true }
    for i := range old {
        if old[i].FieldID != new[i].FieldID { return true }
        if old[i].Required != new[i].Required { return true }
    }
    return false
}
```

### diffFieldIDs

计算字段集合的增删（顺序变化但集合相同时返回空切片）：

```go
func diffFieldIDs(old, new []model.TemplateFieldEntry) (toAdd, toRemove []int64) {
    oldSet := make(map[int64]bool, len(old))
    for _, e := range old { oldSet[e.FieldID] = true }
    newSet := make(map[int64]bool, len(new))
    for _, e := range new { newSet[e.FieldID] = true }

    toAdd = make([]int64, 0)
    for _, e := range new {
        if !oldSet[e.FieldID] { toAdd = append(toAdd, e.FieldID) }
    }
    toRemove = make([]int64, 0)
    for _, e := range old {
        if !newSet[e.FieldID] { toRemove = append(toRemove, e.FieldID) }
    }
    return toAdd, toRemove
}
```

**关键语义**：`isFieldsChanged=true` 但 `toAdd` + `toRemove` 都为空（纯排序或纯 required 变化）时，Service 仍写 `fields` JSON（顺序有业务语义），但 handler 不调 Detach/Attach，不打扰字段方缓存。
