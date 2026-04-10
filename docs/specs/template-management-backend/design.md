# 模板管理后端 — 设计方案

> 对应 [requirements.md](requirements.md) 的 R1-R30 验收标准。
> **本设计严格遵守 [dev-rules.md "分层职责"](../../development/dev-rules.md#分层职责硬性规定)**：store 只管自己的表，service 只管自己模块，跨模块编排在 handler 层。

---

## 总体方案

新增 5 个文件 + 改动 5 个文件。沿用字段管理四件套结构，但**严格执行模块边界**：

```
HTTP 请求
   ↓
TemplateHandler (handler/template.go)
   │
   ├── 校验格式
   ├── 单模块路径：调 TemplateService
   └── 跨模块路径：开 tx → 调 TemplateService.XxxTx + FieldService.YyyTx → Commit → 清缓存
   ↓                       ↓
TemplateService          FieldService（字段管理已存在，扩展 5 个对外方法）
   ↓                       ↓
TemplateStore            FieldStore + FieldRefStore
TemplateCache            FieldCache
```

**模块归属**：

| 模块 | 拥有的表 | 拥有的 store/cache |
|---|---|---|
| 模板管理 | `templates` | TemplateStore / TemplateCache |
| 字段管理 | `fields`、`field_refs` | FieldStore / FieldRefStore / FieldCache |
| 字典 | `dictionaries`（只读基础设施） | DictCache（任意模块可调） |

**单模块路径**（service 自己处理）：
- `/templates/list` —— 只读 templates
- `/templates/check-name` —— 只读 templates
- `/templates/toggle-enabled` —— 只写 templates
- `/templates/detail` 的 templates 部分 —— 只读 templates（字段补全在 handler 层）

**跨模块路径**（handler 编排两个 service + 跨模块事务）：
- `/templates/create` —— 写 templates + 写 field_refs + 改 fields.ref_count
- `/templates/update` —— 写 templates + 增删 field_refs + 增减 fields.ref_count
- `/templates/delete` —— 写 templates + 删 field_refs + 减 fields.ref_count
- `/templates/detail` 的字段补全部分 —— 调 TemplateService 拿 templates 行，再调 FieldService 拿字段精简列表
- `/templates/references` —— 单模块（只读 templates，NPC 部分占位）
- `/fields/references`（已存在，需改造）—— 调 FieldService 拿引用列表，再调 TemplateService 补 label

---

## 数据结构

### 1. model/template.go（新增）

```go
package model

import (
    "encoding/json"
    "time"
)

// Template 模板定义（templates 表整行）
type Template struct {
    ID          int64           `json:"id" db:"id"`
    Name        string          `json:"name" db:"name"`
    Label       string          `json:"label" db:"label"`
    Description string          `json:"description" db:"description"`
    Fields      json.RawMessage `json:"fields" db:"fields"` // [{field_id, required}]
    RefCount    int             `json:"ref_count" db:"ref_count"`
    Enabled     bool            `json:"enabled" db:"enabled"`
    Version     int             `json:"version" db:"version"`
    Deleted     bool            `json:"-" db:"deleted"`
    CreatedAt   time.Time       `json:"created_at" db:"created_at"`
    UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
}

// TemplateFieldEntry templates.fields JSON 单元
// 数组顺序 = NPC 表单展示顺序
type TemplateFieldEntry struct {
    FieldID  int64 `json:"field_id"`
    Required bool  `json:"required"`
}

// TemplateListItem 列表项（覆盖索引返回，不含 fields/description）
type TemplateListItem struct {
    ID        int64     `json:"id" db:"id"`
    Name      string    `json:"name" db:"name"`
    Label     string    `json:"label" db:"label"`
    RefCount  int       `json:"ref_count" db:"ref_count"`
    Enabled   bool      `json:"enabled" db:"enabled"`
    CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// TemplateLite 给跨模块调用的精简结构（id/name/label）
// 用于字段引用详情补 template label
type TemplateLite struct {
    ID    int64  `json:"id" db:"id"`
    Name  string `json:"name" db:"name"`
    Label string `json:"label" db:"label"`
}

// TemplateListData 列表缓存数据
type TemplateListData struct {
    Items    []TemplateListItem `json:"items"`
    Total    int64              `json:"total"`
    Page     int                `json:"page"`
    PageSize int                `json:"page_size"`
}

func (d *TemplateListData) ToListData() *ListData {
    return &ListData{Items: d.Items, Total: d.Total, Page: d.Page, PageSize: d.PageSize}
}

// TemplateDetail 详情接口最终响应（handler 层组装，不进缓存）
type TemplateDetail struct {
    ID          int64               `json:"id"`
    Name        string              `json:"name"`
    Label       string              `json:"label"`
    Description string              `json:"description"`
    Enabled     bool                `json:"enabled"`
    Version     int                 `json:"version"`
    RefCount    int                 `json:"ref_count"`
    CreatedAt   time.Time           `json:"created_at"`
    UpdatedAt   time.Time           `json:"updated_at"`
    Fields      []TemplateFieldItem `json:"fields"`
}

// TemplateFieldItem 详情中的字段精简信息
// 由 handler 把 FieldLite + required 拼装而成
type TemplateFieldItem struct {
    FieldID       int64  `json:"field_id"`
    Name          string `json:"name"`
    Label         string `json:"label"`
    Type          string `json:"type"`
    Category      string `json:"category"`
    CategoryLabel string `json:"category_label"`
    Enabled       bool   `json:"enabled"`
    Required      bool   `json:"required"`
}

// TemplateListQuery 列表查询参数
type TemplateListQuery struct {
    Label    string `json:"label"`
    Enabled  *bool  `json:"enabled,omitempty"`
    Page     int    `json:"page"`
    PageSize int    `json:"page_size"`
}

// CreateTemplateRequest / UpdateTemplateRequest / CreateTemplateResponse / TemplateReferenceItem / TemplateReferenceDetail
// 同上一版（不变），略
```

### 2. model/field.go（追加 1 个结构）

```go
// FieldLite 给跨模块调用的字段精简结构
// 用于模板详情接口拼装 TemplateFieldItem
type FieldLite struct {
    ID            int64  `json:"id" db:"id"`
    Name          string `json:"name" db:"name"`
    Label         string `json:"label" db:"label"`
    Type          string `json:"type" db:"type"`
    Category      string `json:"category" db:"category"`
    CategoryLabel string `json:"category_label" db:"-"` // service 层翻译
    Enabled       bool   `json:"enabled" db:"enabled"`
}
```

---

### 3. errcode/codes.go（追加）

```go
// --- 模板管理 410xx ---

const (
    ErrTemplateNameExists        = 41001
    ErrTemplateNameInvalid       = 41002
    ErrTemplateNotFound          = 41003
    ErrTemplateNoFields          = 41004
    ErrTemplateFieldDisabled     = 41005
    ErrTemplateFieldNotFound     = 41006
    ErrTemplateRefDelete         = 41007
    ErrTemplateRefEditFields     = 41008
    ErrTemplateDeleteNotDisabled = 41009
    ErrTemplateEditNotDisabled   = 41010
    ErrTemplateVersionConflict   = 41011
)

// messages map 追加对应中文消息
```

> **注意**：41005 (`ErrTemplateFieldDisabled`) 和 41006 (`ErrTemplateFieldNotFound`) 由 **FieldService.ValidateFieldsForTemplate** 返回。这两个错误码定义在模板段位是因为它们的语义是"模板侧的字段校验失败"，前端按这两个码分支处理"勾选了停用字段"和"勾选了不存在的字段"。**字段管理自身的同类错误码（40011/40013）不可混用**（go-red-lines）。

---

### 4. store/redis/keys.go（追加）

```go
const (
    prefixTemplateList   = "templates:list:"
    prefixTemplateDetail = "templates:detail:"
    prefixTemplateLock   = "templates:lock:"

    templateListVersionKey = "templates:list:version" // 包内可见
)

func TemplateListKey(version int64, label string, enabled *bool, page, pageSize int) string { ... }
func TemplateDetailKey(id int64) string { ... }
func TemplateLockKey(id int64) string { ... }
```

---

## 5. store/mysql/template.go（新增）

**只对 `templates` 表 CRUD，不读写其他模块的表。**

```go
type TemplateStore struct {
    db *sqlx.DB
}

func NewTemplateStore(db *sqlx.DB) *TemplateStore
func (s *TemplateStore) DB() *sqlx.DB

// CRUD —— 全部只操作 templates 表
func (s *TemplateStore) CreateTx(ctx, tx, req, fieldsJSON) (int64, error)
func (s *TemplateStore) GetByID(ctx, id) (*model.Template, error)
func (s *TemplateStore) ExistsByName(ctx, name) (bool, error)
func (s *TemplateStore) List(ctx, q) ([]model.TemplateListItem, int64, error)
func (s *TemplateStore) UpdateTx(ctx, tx, req, fieldsJSON) error
func (s *TemplateStore) SoftDeleteTx(ctx, tx, id) error
func (s *TemplateStore) ToggleEnabled(ctx, id, enabled, version) error

// ref_count（NPC 模块预留）
func (s *TemplateStore) IncrRefCountTx(ctx, tx, id) error
func (s *TemplateStore) DecrRefCountTx(ctx, tx, id) error
func (s *TemplateStore) GetRefCountTx(ctx, tx, id) (int, error) // FOR SHARE

// 给跨模块用
func (s *TemplateStore) GetByIDs(ctx, ids) ([]model.TemplateLite, error)
```

> **CreateTx/UpdateTx 只接受 tx**：因为模板写操作永远是跨模块事务的一部分（除了 ToggleEnabled），不存在 service 自己开 tx 的场景。
> **ToggleEnabled 走 db**：纯单模块写，不需要 tx。

SQL 范式见上一版（不变）。

---

## 6. store/redis/template.go（新增）

**只缓存 templates 行数据。** 不缓存任何 fields 补全后的复合数据。

```go
type TemplateCache struct {
    rdb *redis.Client
}

func NewTemplateCache(rdb *redis.Client) *TemplateCache

// 详情缓存（缓存 *model.Template 裸行）
func (c *TemplateCache) GetDetail(ctx, id) (*model.Template, bool, error)
func (c *TemplateCache) SetDetail(ctx, id, tpl *model.Template)
func (c *TemplateCache) DelDetail(ctx, id)

// 列表（版本号方案）
func (c *TemplateCache) GetList(ctx, q) (*model.TemplateListData, bool, error)
func (c *TemplateCache) SetList(ctx, q, data)
func (c *TemplateCache) InvalidateList(ctx)

// 锁
func (c *TemplateCache) TryLock(ctx, id, expire) (bool, error)
func (c *TemplateCache) Unlock(ctx, id)
```

> **缓存内容是 `*model.Template` 而非 `*model.TemplateDetail`**：字段补全是 handler 层的拼装动作，不进缓存。这样字段被编辑/停用时不会污染模板缓存。

---

## 7. service/template.go（新增）

**只调用自己模块的 store/cache + DictCache 基础设施。** 不持有 FieldStore/FieldRefStore/FieldCache 的任何引用。

```go
type TemplateService struct {
    store  *storemysql.TemplateStore
    cache  *storeredis.TemplateCache
    pagCfg *config.PaginationConfig
}

func NewTemplateService(store, cache, pagCfg) *TemplateService
```

**对内方法（service 自己用）**：

```go
// 单模块路径
func (s *TemplateService) List(ctx, q) (*model.ListData, error)
func (s *TemplateService) GetByID(ctx, id) (*model.Template, error)  // 返回裸行 + 缓存
func (s *TemplateService) ExistsByName(ctx, name) (bool, error)
func (s *TemplateService) CheckName(ctx, name) (*model.CheckNameResult, error)
func (s *TemplateService) ToggleEnabled(ctx, req) error
```

**对外方法（供 handler 跨模块编排调用）**：

```go
// 跨模块事务参与（接收外部 tx）
func (s *TemplateService) CreateTx(ctx, tx, req, fieldsJSON) (int64, error)
func (s *TemplateService) UpdateTx(ctx, tx, req, fieldsJSON, oldVersion int) error
func (s *TemplateService) SoftDeleteTx(ctx, tx, id) error
func (s *TemplateService) GetRefCountForDeleteTx(ctx, tx, id) (int, error) // FOR SHARE
func (s *TemplateService) ParseFieldEntries(raw json.RawMessage) ([]TemplateFieldEntry, error)

// 缓存失效（跨模块编排 commit 后由 handler 调用）
func (s *TemplateService) InvalidateDetail(ctx, id)
func (s *TemplateService) InvalidateList(ctx)

// 给字段管理跨模块调用
func (s *TemplateService) GetByIDsLite(ctx, ids) ([]model.TemplateLite, error)
```

**TemplateService 不再做的事**（与上一版的差异）：
- ❌ 不调 fieldStore.GetByIDs
- ❌ 不调 fieldRefStore.Add/Remove
- ❌ 不调 fieldStore.IncrRefCountTx/DecrRefCountTx
- ❌ 不持有 fieldCache
- ❌ 不开跨模块事务

**TemplateService 仍然做的事**：
- ✅ 校验 fields 数组格式（非空、无重复 field_id）—— 这是模板自身的输入校验
- ✅ 解析 fields JSON
- ✅ 维护自身缓存
- ✅ 乐观锁错误转换
- ✅ slog 日志

### TemplateService.GetByID 实现（详情接口的"模板部分"）

```go
func (s *TemplateService) GetByID(ctx, id) (*model.Template, error) {
    // 1. 查 templateCache.GetDetail(id)
    //    命中且非空 → 返回
    //    命中空标记 → 返回 41003
    // 2. TryLock 防击穿 + double-check
    // 3. templateStore.GetByID(id)
    // 4. nil → SetDetail(id, nil) + 返回 41003
    // 5. SetDetail(id, tpl)
    // 6. 返回 tpl
}
```

> 注意返回的是 `*model.Template`（裸行），不是 `*model.TemplateDetail`。字段补全是 handler 层的责任。

---

## 8. service/field.go（改动 — 扩展 5 个对外方法）

**FieldService 新增的对外方法**：

```go
// ValidateFieldsForTemplate 校验字段列表对模板的可用性
// 用途：handler 在创建/编辑模板时调用，校验勾选的 field_ids 全部存在 + 启用
// 返回：errcode 41005 / 41006 / nil
func (s *FieldService) ValidateFieldsForTemplate(ctx, fieldIDs []int64) error

// AttachToTemplateTx 把字段列表挂到模板上（事务内）
// 用途：handler 在创建模板/编辑模板时调用
// 行为：对每个 fieldID 写 field_refs(template, templateID) + IncrRefCount
// 返回：受影响的 fieldID 列表（用于 handler commit 后清缓存）
func (s *FieldService) AttachToTemplateTx(ctx, tx, templateID int64, fieldIDs []int64) ([]int64, error)

// DetachFromTemplateTx 把字段列表从模板上卸下（事务内）
// 用途：handler 在删除模板/编辑模板移除字段时调用
// 行为：对每个 fieldID 删 field_refs(template, templateID) + DecrRefCount
// 返回：受影响的 fieldID 列表
func (s *FieldService) DetachFromTemplateTx(ctx, tx, templateID int64, fieldIDs []int64) ([]int64, error)

// GetByIDsLite 批量查字段精简信息
// 用途：handler 在拼装 TemplateDetail 时调用
// 行为：从 fieldStore 批量取，service 层做 category_label 翻译
// 返回：[]FieldLite，按 fieldIDs 顺序对齐（不存在的位置返回 zero FieldLite）
func (s *FieldService) GetByIDsLite(ctx, fieldIDs []int64) ([]model.FieldLite, error)

// InvalidateDetails 批量清字段详情缓存
// 用途：handler 在跨模块事务 commit 后调用
func (s *FieldService) InvalidateDetails(ctx, fieldIDs []int64)
```

**FieldService.GetReferences 改造**（删除占位 label 的 TODO）：

```go
// 改造前（service 调 templateStore，违规）
result.Templates = append(result.Templates, model.ReferenceItem{
    Label: fmt.Sprintf("模板#%d", tid),
})

// 改造后（service 只返回 ID 列表，handler 补 label）
// service 层去掉对 templates label 的拼装，把 templateIDs 透传给上层
// handler/field.go GetReferences 编排：先调 fieldService.GetReferences，
// 再调 templateService.GetByIDsLite 补 label
```

具体改造步骤见下文 handler 层。

---

## 9. handler/template.go（新增）

**handler 持有 db + 两个 service**，跨模块事务在 handler 开启。

```go
type TemplateHandler struct {
    db              *sqlx.DB
    templateService *service.TemplateService
    fieldService    *service.FieldService
    dictCache       *cache.DictCache
    valCfg          *config.ValidationConfig
}

func NewTemplateHandler(db, ts, fs, dc, vc) *TemplateHandler
```

> **handler 持有 db**：因为跨模块事务的边界在 handler。这是新规则下的必然选择。
> **handler 持有 dictCache**：因为详情接口的字段补全（category_label 翻译）发生在 handler 层。或者把翻译放在 FieldService.GetByIDsLite 里——选 **后者**，handler 不感知字典。修订：去掉 dictCache。

修订后：

```go
type TemplateHandler struct {
    db              *sqlx.DB
    templateService *service.TemplateService
    fieldService    *service.FieldService
    valCfg          *config.ValidationConfig
}
```

### handler.Create 流程（跨模块事务）

```go
func (h *TemplateHandler) Create(ctx, req) (*CreateTemplateResponse, error) {
    // 1. 格式校验（name 正则 / label 长度 / fields 非空 / field_id 去重）
    // 2. 模板自身业务校验（service 同步调用）
    if exists, _ := h.templateService.ExistsByName(ctx, req.Name); exists {
        return nil, errcode.New(ErrTemplateNameExists)
    }
    // 3. 跨模块校验：字段必须存在 + 启用
    fieldIDs := extractFieldIDs(req.Fields)
    if err := h.fieldService.ValidateFieldsForTemplate(ctx, fieldIDs); err != nil {
        return nil, err  // 41005 / 41006
    }
    // 4. 序列化 fields JSON
    fieldsJSON, _ := json.Marshal(req.Fields)
    // 5. 开跨模块事务
    tx, err := h.db.BeginTxx(ctx, nil)
    if err != nil { ... }
    defer tx.Rollback()
    // 6. 写 templates 行
    templateID, err := h.templateService.CreateTx(ctx, tx, req, fieldsJSON)
    if err != nil { return nil, err }
    // 7. 写 field_refs + IncrRefCount
    affected, err := h.fieldService.AttachToTemplateTx(ctx, tx, templateID, fieldIDs)
    if err != nil { return nil, err }
    // 8. 提交
    if err := tx.Commit(); err != nil { ... }
    // 9. 清缓存（service 各自管自己）
    h.templateService.InvalidateList(ctx)
    h.fieldService.InvalidateDetails(ctx, affected)
    // 10. 返回
    return &CreateTemplateResponse{ID: templateID, Name: req.Name}, nil
}
```

### handler.Update 流程

```go
1. 格式校验
2. h.templateService.GetByID(id) → 旧 tpl
3. enabled 校验 (41010)
4. ParseFieldEntries(tpl.Fields) → oldEntries
5. diff(oldEntries, req.Fields) → fieldsChanged / toAdd / toRemove
6. ref_count > 0 + fieldsChanged → 41008
7. 若有 toAdd:
   - h.fieldService.ValidateFieldsForTemplate(ctx, toAdd) → 41005/41006
8. 序列化 newFieldsJSON
9. tx := h.db.BeginTxx
10. h.templateService.UpdateTx(ctx, tx, req, newFieldsJSON, req.Version) → 41011
11. 若 ref_count == 0 + fieldsChanged 中"集合"变了:
    - removedAffected := h.fieldService.DetachFromTemplateTx(ctx, tx, id, toRemove)
    - addedAffected := h.fieldService.AttachToTemplateTx(ctx, tx, id, toAdd)
12. tx.Commit
13. 清缓存:
    - h.templateService.InvalidateDetail(ctx, id)
    - h.templateService.InvalidateList(ctx)
    - h.fieldService.InvalidateDetails(ctx, removedAffected ∪ addedAffected)
```

### handler.Delete 流程

```go
1. 格式校验
2. h.templateService.GetByID(id) → tpl (41003)
3. enabled 校验 (41009)
4. ParseFieldEntries(tpl.Fields) → fieldEntries → fieldIDs
5. tx := h.db.BeginTxx
6. refCount, _ := h.templateService.GetRefCountForDeleteTx(tx, id) -- FOR SHARE
7. refCount > 0 → 41007
8. h.templateService.SoftDeleteTx(tx, id)
9. affected := h.fieldService.DetachFromTemplateTx(tx, id, fieldIDs)
10. tx.Commit
11. 清缓存:
    - h.templateService.InvalidateDetail(ctx, id)
    - h.templateService.InvalidateList(ctx)
    - h.fieldService.InvalidateDetails(ctx, affected)
12. 返回 DeleteResult
```

### handler.Get（详情）流程

```go
1. id 校验
2. tpl, err := h.templateService.GetByID(ctx, id)  -- 内部走 cache + lock + db
3. if err → return  (41003)
4. ParseFieldEntries(tpl.Fields) → entries
5. fieldIDs := extract(entries)
6. fieldLites, err := h.fieldService.GetByIDsLite(ctx, fieldIDs)
   -- FieldService 走自己的 cache + db + dictCache 翻译 category_label
7. handler 拼装 TemplateDetail:
   - 用 fieldLites 按 entries 顺序 zip 成 []TemplateFieldItem
   - 缺失的 fieldID 用 zero value 填充 + slog.Warn
   - 注入 required (来自 entries) 和 enabled (来自 fieldLites)
8. 返回 *TemplateDetail
```

### handler.ToggleEnabled / handler.CheckName / handler.GetReferences / handler.List

单模块路径，handler 只校验请求格式后转发：

```go
func (h *TemplateHandler) List(ctx, q) (*ListData, error) {
    return h.templateService.List(ctx, q)
}

func (h *TemplateHandler) ToggleEnabled(ctx, req) (*string, error) {
    if err := checkID(req.ID); ...
    if err := checkVersion(req.Version); ...
    if err := h.templateService.ToggleEnabled(ctx, req); ...
    return successMsg("操作成功"), nil
}
```

---

## 10. handler/field.go（改动 — GetReferences 跨模块编排）

```go
type FieldHandler struct {
    fieldService    *service.FieldService
    templateService *service.TemplateService  // 新增
    valCfg          *config.ValidationConfig
}

// 改造 GetReferences
func (h *FieldHandler) GetReferences(ctx, req) (*model.ReferenceDetail, error) {
    if err := checkID(req.ID); err != nil { ... }

    // 1. 调字段管理拿原始引用列表（含 field 和 template 两类，template 的 label 为空）
    detail, err := h.fieldService.GetReferences(ctx, req.ID)
    if err != nil { return nil, err }

    // 2. 提取 templateIDs
    templateIDs := extractTemplateIDs(detail.Templates)
    if len(templateIDs) == 0 {
        return detail, nil
    }

    // 3. 跨模块调模板管理补 label
    tplLites, err := h.templateService.GetByIDsLite(ctx, templateIDs)
    if err != nil {
        slog.Error("handler.补模板label失败", "error", err)
        return nil, fmt.Errorf("get template lites: %w", err)
    }

    // 4. handler 层拼装
    labelMap := make(map[int64]string, len(tplLites))
    for _, t := range tplLites {
        labelMap[t.ID] = t.Label
    }
    for i := range detail.Templates {
        detail.Templates[i].Label = labelMap[detail.Templates[i].RefID]
    }

    return detail, nil
}
```

**FieldService.GetReferences 内部改造**：删除当前 `fmt.Sprintf("模板#%d", tid)` 占位逻辑，service 层只返回带 RefID 不带 Label 的 Templates 数组（Label 字段留空字符串）。handler 负责补 Label。

---

## 11. router/router.go（改动）

```go
func Setup(r, fh *handler.FieldHandler, dh *handler.DictionaryHandler, th *handler.TemplateHandler) {
    // fields 路由不变
    // templates 路由新增 8 个
    templates := v1.Group("/templates")
    {
        templates.POST("/list", handler.WrapCtx(th.List))
        templates.POST("/create", handler.WrapCtx(th.Create))
        templates.POST("/detail", handler.WrapCtx(th.Get))
        templates.POST("/update", handler.WrapCtx(th.Update))
        templates.POST("/delete", handler.WrapCtx(th.Delete))
        templates.POST("/check-name", handler.WrapCtx(th.CheckName))
        templates.POST("/references", handler.WrapCtx(th.GetReferences))
        templates.POST("/toggle-enabled", handler.WrapCtx(th.ToggleEnabled))
    }
}
```

---

## 12. cmd/admin/main.go（改动）

```go
// 新增装配
templateStore := storemysql.NewTemplateStore(db)
templateCache := storeredis.NewTemplateCache(rdb)
templateService := service.NewTemplateService(templateStore, templateCache, &cfg.Pagination)

// FieldService 构造签名不变（不依赖 templateService，避免循环）
fieldService := service.NewFieldService(fieldStore, fieldRefStore, fieldCache, dictCache, &cfg.Pagination)

// FieldHandler 构造签名追加 templateService（用于 GetReferences 补 label）
fieldHandler := handler.NewFieldHandler(fieldService, templateService, &cfg.Validation)

// TemplateHandler 持有 db + 两个 service
templateHandler := handler.NewTemplateHandler(db, templateService, fieldService, &cfg.Validation)

router.Setup(r, fieldHandler, dictHandler, templateHandler)
```

> **依赖方向**：FieldService 不依赖 TemplateService，TemplateService 不依赖 FieldService。两个 service 之间无直接调用关系。**所有跨模块协作通过 handler 层组合完成**。无循环依赖。

---

## 方案对比（备选）

### 备选 A：把 fields 列拆成单独的 template_fields 表

否决（同上一版理由：JOIN 多一层 + 顺序变更要 UPDATE 多行）。

### 备选 B：让 service 之间直接调用（违反新规则）

之前的设计是这样：TemplateService 持有 FieldStore/FieldRefStore/FieldCache 引用。

**为什么改掉**：违反 dev-rules.md 新增的"分层职责"硬性规定。模块解耦、测试单元化、依赖方向清晰是优先级更高的目标。代码量略增（handler 层多 ~50 行编排），但结构更清晰。

### 备选 C：新建一个"template_create_orchestrator" 中间层

在 service 和 handler 之间引入"用例"层专门处理跨模块编排。

**否决**：过度设计。当前阶段两层就够，handler 完全可以承担用例编排者的角色，不需要再加一层。引入中间层就是 over-engineering（违反 backend-red-lines "ADMIN 过度设计"）。

---

## 红线检查

| 红线文件 | 关键条目 | 本设计是否触发 | 处理 |
|---|---|---|---|
| dev-rules.md | 分层职责（store 单表/service 同模块/handler 跨模块）| ✅ 严格遵守 | 见上设计 |
| standards/red-lines.md | 禁止静默降级（lookup 失败 silent return） | ⚠️ 详情接口字段查不到时 | slog.Warn 告警 + zero value 兜底 |
| standards/red-lines.md | 禁止信任前端校验 | ✅ | service/handler 双层校验 |
| standards/red-lines.md | 禁止 API 暴露内部错误 | ✅ | errcode + 中文消息 + 原 error 入 slog |
| standards/go-red-lines.md | nil slice → null | ✅ | 所有返回 slice 用 make([]T, 0) |
| standards/go-red-lines.md | 禁止资源泄漏 | ✅ | tx 用 defer Rollback |
| standards/go-red-lines.md | json.RawMessage scan NULL | ✅ | templates.fields NOT NULL |
| standards/go-red-lines.md | 禁止 typed nil error | ✅ | return nil |
| standards/go-red-lines.md | 禁止错误码语义混用 | ✅ | 41001-41011 全部新建 |
| standards/go-red-lines.md | 禁止缓存反序列化类型丢失 | ✅ | TemplateListData/Template 类型安全 |
| standards/go-red-lines.md | 禁止硬编码魔术字符串 | ✅ | RefTypeTemplate 常量 |
| standards/go-red-lines.md | 禁止分层倒置 | ✅ | handler → service → store/cache 单向 |
| standards/mysql-red-lines.md | 事务连接不混用 | ✅ | Tx 方法只用 tx 参数 |
| standards/mysql-red-lines.md | TOCTOU 用 FOR SHARE | ✅ | GetRefCountForDeleteTx |
| standards/mysql-red-lines.md | LIKE 转义 | ✅ | escapeLike |
| standards/redis-red-lines.md | 禁止 SCAN+DEL | ✅ | 版本号方案 |
| standards/redis-red-lines.md | DEL/Unlock 检查 error | ✅ | slog.Error 但不阻塞 |
| standards/cache-red-lines.md | 写后清缓存 | ✅ | handler 在 commit 后调各 service 的 Invalidate |
| standards/cache-red-lines.md | 修改字段 ref_count 后清字段方 detail | ✅ | handler 调 fieldService.InvalidateDetails(affected) |
| standards/cache-red-lines.md | 缓存击穿用分布式锁 | ✅ | TemplateService.GetByID 内部 TryLock |
| standards/cache-red-lines.md | 缓存无 TTL | ✅ | TTL+jitter |
| architecture/backend-red-lines.md | 禁止硬编码错误码/key/类型 | ✅ | 全部走常量 |
| architecture/backend-red-lines.md | 禁止 ADMIN 过度设计 | ✅ | 不引入中间层 |
| architecture/backend-red-lines.md | 禁止破坏游戏服务端数据格式 | ✅ | 模板不写 MongoDB |

**触发但已处理**：详情接口字段查不到时不能 silent skip → 已加 slog.Warn 告警。

---

## 扩展性影响

| 扩展轴 | 影响方向 | 说明 |
|---|---|---|
| **新增配置类型只需加四件套** | ✅ 强正面 | 本 spec 严格执行了"模块边界"，未来 NPC 模块新增时只需添加自己的 store/service，跨模块编排在 handler 层即可，不会侵入 templates 或 fields 模块 |
| **新增表单字段只需加组件** | ⚪ 中性 | 后端不直接涉及 |

**额外收益**：模块解耦后，未来如果想把"模板管理"独立成一个微服务，只需要把 templates store/service 拆出去，handler 层换成 RPC 调用即可，FieldService 不需要任何改动。

---

## 依赖方向

```
handler/template.go  ──→  service/template.go  ──→  store/mysql/template.go
        │                                    └─→  store/redis/template.go
        ├──→  service/field.go (扩展)  ──→  store/mysql/field.go
        │                              └──→  store/mysql/field_ref.go
        │                              └──→  store/redis/field.go
        │                              └──→  cache/dict.go
        └──→  *sqlx.DB (跨模块事务)

handler/field.go (改动)  ──→  service/field.go
                          └──→  service/template.go (新增依赖：补 label)

service/template.go  ✗ 不依赖  service/field.go
service/field.go     ✗ 不依赖  service/template.go
```

**无循环依赖**。两个 service 完全解耦，所有交互都通过 handler 层。

---

## 陷阱检查

| 陷阱文件 | 关键陷阱 | 应对 |
|---|---|---|
| go-pitfalls.md | nil slice → JSON null | service/handler 所有返回 slice 用 `make([]T, 0)` |
| go-pitfalls.md | json.RawMessage scan NULL | templates.fields NOT NULL |
| go-pitfalls.md | utf8 字符数 | label 校验用 `utf8.RuneCountInString` |
| go-pitfalls.md | 嵌套 for continue/break | 本 spec 无嵌套循环里的跨层跳出 |
| mysql-pitfalls.md | 事务内必须用 tx 不用 db | Tx 方法签名强制接 tx |
| mysql-pitfalls.md | REPEATABLE READ TOCTOU | Delete 用 FOR SHARE |
| mysql-pitfalls.md | LIKE 转义 | escapeLike |
| mysql-pitfalls.md | 乐观锁 rows=0 语义 | service 层先 GetByID 预检查 |
| mysql-pitfalls.md | 操作标识用 ID | 全程 ID |
| redis-pitfalls.md | redis.Nil 判断 | errors.Is(err, redis.Nil) |
| redis-pitfalls.md | SetNX 锁必须 expire | TryLock 强制 expire |
| cache-pitfalls.md | 写后清缓存 | handler 在 commit 后清 |
| cache-pitfalls.md | 级联清关联方缓存 | handler 调 InvalidateDetails(affected) |
| cache-pitfalls.md | 空值标记 | nullMarker |
| cache-pitfalls.md | TTL jitter | ttl(base, jitter) |

---

## 配置变更

无。复用字段管理已有的 `pagination` 和 `validation` 配置项。

---

## 测试策略

### 单元测试范围

**TemplateStore 单元测试**（`store/mysql/template_test.go`）：
- CreateTx + GetByID + ExistsByName 闭环
- List 分页 + label 过滤 + enabled 过滤
- UpdateTx 乐观锁版本冲突
- SoftDeleteTx
- ToggleEnabled
- GetByIDs 批量
- GetRefCountTx FOR SHARE

**TemplateService 单元测试**（`service/template_test.go`）：
- List/GetByID 缓存命中/未命中
- ExistsByName/CheckName
- ToggleEnabled 乐观锁错误转换
- ParseFieldEntries 异常输入

**FieldService 新增方法单元测试**（追加到 `service/field_test.go`）：
- ValidateFieldsForTemplate 全部存在 + 启用 / 部分不存在 / 部分停用
- AttachToTemplateTx / DetachFromTemplateTx 事务内行为 + 返回 affected
- GetByIDsLite 顺序对齐 + category_label 翻译

**TemplateHandler 集成测试**（`handler/template_test.go`）：
- 跨模块事务：Create / Update（fields 变更）/ Delete 的端到端
- 失败回滚：模拟 fieldService.AttachToTemplateTx 返回错误，验证 templates 行没有写入
- 详情拼装：handler 层把 templates 行 + field lites 拼成 TemplateDetail
- GetReferences 编排（FieldHandler 改造后）

**集成测试**：testcontainers 起 MySQL+Redis，端到端跑 8 个接口。

---

## 已知遗留

| 遗留 | 处理时机 |
|---|---|
| 字段编辑/停用时清模板 detail 缓存的级联 hook | 后续小型补丁（缓存粒度变小后影响降低，因为模板 detail 缓存只存 templates 行，不存字段补全） |
| `/templates/references` NPC 占位 | NPC 模块上线时改 service 方法 |
| 模板默认值覆盖 | 毕设后 |
| 审计日志 | 统一时机 |

---

## 总结：与上一版的关键差异

| 维度 | 上一版（违规） | 本版（合规） |
|---|---|---|
| TemplateService 依赖 | fieldStore/fieldRefStore/fieldCache/dictCache | 仅 templateStore/templateCache（DictCache 通过 FieldService 间接使用） |
| 跨模块事务 | service 内开 | handler 内开 |
| 字段引用维护 | TemplateService 调 fieldRefStore.Add | FieldService.AttachToTemplateTx |
| 字段缓存清理 | TemplateService 调 fieldCache.DelDetail | FieldService.InvalidateDetails |
| TemplateDetail 缓存内容 | 完整复合（含字段精简列表） | 只存 templates 裸行，字段补全在 handler 层每次拼 |
| TemplateService 文件大小 | ~600 行 | ~350 行 |
| FieldService 文件大小 | 现状 ~750 行 | ~900 行（+150 行新增 5 个方法） |
| handler/template.go 文件大小 | ~300 行 | ~450 行（+150 行跨模块编排） |
| 总代码量 | 大致相同 | 大致相同（150 行从 service 转移到 handler） |
| 单元测试 | service 测试要 mock 字段管理多个组件 | service 测试只 mock 自己模块 |

---

## 待审批确认事项

进入 Phase 3 前请确认：

1. 41005/41006 错误码归在模板管理段位（41xxx），由 FieldService.ValidateFieldsForTemplate 返回。是否同意？
2. handler 持有 `*sqlx.DB` 用于跨模块事务，是否同意？
3. FieldService 新增 5 个对外方法的命名（ValidateFieldsForTemplate / AttachToTemplateTx / DetachFromTemplateTx / GetByIDsLite / InvalidateDetails）是否同意？
