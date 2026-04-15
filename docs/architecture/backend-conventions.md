# 后端统一约定

本文档描述 ADMIN 项目后端（Go + Gin + MySQL + Redis）的**项目级约定**：分层职责、各层代码模式、错误码体系、乐观锁、软删除、分页。

通用 Go 规范见 `../development/standards/dev-rules/go.md`，禁止红线见 `../development/standards/red-lines/go.md`，Admin 项目专属规则见 `../development/admin/dev-rules.md`。

---

## 一、分层职责

```
handler/        HTTP 入口：JSON 解析 → 请求校验 → 调 service → 写响应
  shared/         handler 层工具：CheckID / CheckVersion / CheckName / CheckLabel / SuccessMsg
service/        业务逻辑：分页规范化 → 缓存查询 → MySQL 读写 → 缓存失效
  shared/         service 层工具：NormalizePagination / ValidateValue / ValidateConstraintsSelf
store/mysql/    MySQL CRUD：纯 SQL，不含业务判断，返回哨兵错误
store/redis/    Redis 缓存：Cache-Aside 读写，分布式锁，key 统一管理
  shared/         Redis key 生成、连接辅助（package shared，import alias rcfg）
cache/          内存缓存：字典 / Schema 启动全量加载，变更后 Reload()
model/          数据模型：DB 结构体 + 请求/响应结构体
errcode/        错误码：业务码（codes.go）+ store 哨兵错误（store_errors.go）
router/         路由注册：按模块分组，RESTful 路径
setup/          初始化聚合：DB 连接 → Store → Cache → Service → Handler → Router
config/         配置加载
util/const.go   跨层枚举常量（FieldType / RefType / DictGroup 等）
```

**铁律：层之间单向依赖，禁止跨层调用。handler 不直接访问 store，service 不直接写 HTTP 响应。**

---

## 二、Handler 层模式

### 2.1 请求包装器

所有 handler 方法用 `WrapCtx` 包装，不手写 JSON 解析和响应：

```go
// router 注册
r.POST("/api/v1/fields", handler.WrapCtx(h.fieldHandler.Create))
r.GET("/api/v1/fields", handler.WrapCtx(h.fieldHandler.List))

// handler 方法签名统一为：
func (h *XxxHandler) Create(ctx context.Context, req *model.XxxCreateReq) (*model.XxxCreateResp, error)
func (h *XxxHandler) List(ctx context.Context, req *model.XxxListQuery) (*model.ListData, error)
func (h *XxxHandler) Detail(ctx context.Context, req *model.IDRequest) (*model.XxxDetail, error)
func (h *XxxHandler) Update(ctx context.Context, req *model.XxxUpdateReq) (*string, error)
func (h *XxxHandler) Delete(ctx context.Context, req *model.IDRequest) (*model.DeleteResult, error)
func (h *XxxHandler) ToggleEnabled(ctx context.Context, req *model.ToggleEnabledRequest) (*string, error)
func (h *XxxHandler) CheckName(ctx context.Context, req *model.CheckNameRequest) (*model.CheckNameResult, error)
```

`WrapCtx` 自动处理：`ShouldBindJSON` → 调 fn → 业务错误写对应 code → 系统错误写 50000 + slog.Error。

### 2.2 请求校验顺序

```go
func (h *XxxHandler) Update(ctx context.Context, req *model.XxxUpdateReq) (*string, error) {
    // 1. ID / Version（用 shared helper，不手写 if req.ID <= 0）
    if err := shared.CheckID(req.ID); err != nil {
        return nil, err
    }
    if err := shared.CheckVersion(req.Version); err != nil {
        return nil, err
    }

    // 2. 格式校验（名称/标签/枚举）
    if e := shared.CheckName(req.Name, h.cfg.NameMaxLength, errcode.ErrXxxNameInvalid, "字段标识"); e != nil {
        return nil, e
    }
    if e := shared.CheckLabel(req.Label, h.cfg.LabelMaxLength, "中文标签"); e != nil {
        return nil, e
    }

    // 3. debug 日志（校验通过后再记录）
    slog.Debug("handler.xxx.update", "id", req.ID)

    // 4. 调 service
    if err := h.svc.Update(ctx, req); err != nil {
        return nil, err
    }
    return shared.SuccessMsg("保存成功"), nil
}
```

### 2.3 shared 辅助函数

| 函数 | 用途 |
|------|------|
| `shared.CheckID(id int64)` | id <= 0 返回 40000 |
| `shared.CheckVersion(v int)` | v <= 0 返回 40000 |
| `shared.CheckName(name, maxLen, errCode, subject)` | 正则 + 长度，错误码由调用方传入 |
| `shared.CheckLabel(label, maxLen, subject)` | 非空 + UTF-8 字符数上限，统一返回 40000 |
| `shared.SuccessMsg(msg)` | 构造 `*string`，Update/ToggleEnabled 返回值用 |

---

## 三、Service 层模式

### 3.1 结构体与构造函数

```go
type XxxService struct {
    store    *storemysql.XxxStore
    cache    *storeredis.XxxCache   // Redis 缓存（可选）
    pagCfg   *config.PaginationConfig
    xxxCfg   *config.XxxConfig     // 模块专属配置
}

func NewXxxService(store *storemysql.XxxStore, cache *storeredis.XxxCache,
    pagCfg *config.PaginationConfig, xxxCfg *config.XxxConfig) *XxxService {
    return &XxxService{store: store, cache: cache, pagCfg: pagCfg, xxxCfg: xxxCfg}
}
```

### 3.2 List 方法

```go
func (s *XxxService) List(ctx context.Context, q *model.XxxListQuery) (*model.ListData, error) {
    // 1. 分页规范化（必须第一步）
    shared.NormalizePagination(&q.Page, &q.PageSize,
        s.pagCfg.DefaultPage, s.pagCfg.DefaultPageSize, s.pagCfg.MaxPageSize)

    // 2. 查 Redis（挂了跳过，降级直查 MySQL）
    if cached, hit, err := s.cache.GetList(ctx, q); err == nil && hit {
        return cached.ToListData(), nil
    }

    // 3. 查 MySQL
    items, total, err := s.store.List(ctx, q)
    if err != nil {
        return nil, err
    }

    // 4. 写 Redis
    listData := &model.XxxListData{Items: items, Total: total, Page: q.Page, PageSize: q.PageSize}
    s.cache.SetList(ctx, q, listData)

    return listData.ToListData(), nil
}
```

### 3.3 store 哨兵错误翻译

store 层只返回哨兵错误，service 层 `errors.Is()` 捕获后翻译为模块业务码：

```go
// store 返回哨兵错误（errcode/store_errors.go）：
// errcode.ErrNotFound / errcode.ErrVersionConflict / errcode.ErrDuplicate

// service 翻译：
if errors.Is(err, errcode.ErrVersionConflict) {
    return errcode.New(errcode.ErrXxxVersionConflict)
}
if errors.Is(err, errcode.ErrDuplicate) {
    return errcode.Newf(errcode.ErrXxxNameExists, "标识 '%s' 已存在", req.Name)
}
```

### 3.4 乐观锁写操作

```go
func (s *XxxService) Update(ctx context.Context, req *model.XxxUpdateReq) error {
    // 1. 存在性校验（也可内联）
    if _, err := s.getOrNotFound(ctx, req.ID); err != nil {
        return err
    }

    // 2. 业务规则校验（启用中不可编辑等）

    // 3. 写 MySQL（store 层做乐观锁：WHERE id=? AND version=?，0 rows → ErrVersionConflict）
    if err := s.store.Update(ctx, req); err != nil {
        if errors.Is(err, errcode.ErrVersionConflict) {
            return errcode.New(errcode.ErrXxxVersionConflict)
        }
        return err
    }

    // 4. 缓存失效（Commit 成功后）
    s.cache.DelDetail(ctx, req.ID)
    s.cache.InvalidateList(ctx)

    return nil
}
```

### 3.5 事务写操作

需要跨表原子操作时开事务，统一用 defer Rollback：

```go
tx, err := s.store.DB().BeginTxx(ctx, nil)
if err != nil {
    return fmt.Errorf("begin tx: %w", err)
}
defer func() {
    if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
        slog.Warn("service.xxx事务回滚失败", "error", rbErr)
    }
}()

// ... 事务内操作 ...

if err := tx.Commit(); err != nil {
    return fmt.Errorf("commit: %w", err)
}

// 缓存失效必须在 Commit 成功后
s.cache.Reload(ctx)  // 或 DelDetail + InvalidateList
```

### 3.6 shared 辅助函数

| 函数 | 用途 |
|------|------|
| `shared.NormalizePagination(page, pageSize, ...)` | 修正负数/超限分页参数 |
| `shared.ValidateValue(fieldType, constraints, value)` | 字段值是否符合 constraints |
| `shared.ValidateConstraintsSelf(fieldType, constraints, errCode)` | constraints 内部自洽校验 |

---

## 四、Store/MySQL 层模式

### 4.1 List 方法（带分页）

```go
func (s *XxxStore) List(ctx context.Context, q *model.XxxListQuery) ([]model.XxxListItem, int64, error) {
    where := []string{"deleted = 0"}
    args := make([]any, 0, 4)

    if q.Name != "" {
        where = append(where, "name LIKE ?")
        args = append(args, shared.EscapeLike(q.Name)+"%")
    }
    if q.Enabled != nil {
        where = append(where, "enabled = ?")
        args = append(args, *q.Enabled)
    }
    whereClause := strings.Join(where, " AND ")

    // COUNT
    var total int64
    if err := s.db.GetContext(ctx, &total,
        "SELECT COUNT(*) FROM xxx WHERE "+whereClause, args...); err != nil {
        return nil, 0, fmt.Errorf("count xxx: %w", err)
    }

    // LIST
    listSQL := fmt.Sprintf(
        `SELECT id, name, ... FROM xxx WHERE %s ORDER BY id DESC LIMIT ? OFFSET ?`,
        whereClause,
    )
    offset := (q.Page - 1) * q.PageSize
    items := make([]model.XxxListItem, 0)
    if err := s.db.SelectContext(ctx, &items, listSQL, append(args, q.PageSize, offset)...); err != nil {
        return nil, 0, fmt.Errorf("list xxx: %w", err)
    }
    return items, total, nil
}
```

### 4.2 乐观锁 Update

```go
func (s *XxxStore) Update(ctx context.Context, req *model.XxxUpdateReq) error {
    result, err := s.db.ExecContext(ctx,
        `UPDATE xxx SET col=?, version=version+1, updated_at=? WHERE id=? AND version=? AND deleted=0`,
        req.Col, time.Now(), req.ID, req.Version,
    )
    if err != nil {
        return fmt.Errorf("update xxx: %w", err)
    }
    rows, _ := result.RowsAffected()
    if rows == 0 {
        return errcode.ErrVersionConflict  // 哨兵错误，service 层翻译
    }
    return nil
}
```

### 4.3 软删除

所有配置实体用软删除，不物理删除：

```go
// 软删除
UPDATE xxx SET deleted=1, updated_at=? WHERE id=? AND deleted=0

// 所有查询加 AND deleted=0
SELECT ... FROM xxx WHERE deleted = 0 AND ...

// 唯一索引包含软删除行（含 field_name + deleted）避免标识复用问题时使用全量检查：
SELECT COUNT(*) FROM xxx WHERE name = ?  // 不加 deleted=0
```

### 4.4 store 层只返回哨兵错误

```
errcode.ErrNotFound        // GetByID 返回 nil 时调用方判断，或 soft-delete 0 rows 时
errcode.ErrVersionConflict // Update 0 rows 时
errcode.ErrDuplicate       // 捕获 MySQL 1062（shared.Is1062(err)）
```

---

## 五、Store/Redis 层模式（Cache-Aside）

### 5.1 读：先查缓存，缺失穿透到 MySQL

```go
// service 层
if cached, hit, err := s.cache.GetDetail(ctx, id); err == nil && hit {
    return cached, nil
}
// 查 MySQL，写缓存
```

### 5.2 写：先更新 MySQL，Commit 成功后失效缓存

```go
// 顺序不可颠倒：Commit 前失效 → 其他协程穿透到旧数据写入缓存 → 更新丢失
if err := tx.Commit(); err != nil { ... }
s.cache.DelDetail(ctx, id)
s.cache.InvalidateList(ctx)
```

### 5.3 Key 统一管理

所有 Redis key 通过 `store/redis/config/keys.go` 中的函数生成，不在业务代码里拼字符串：

```go
// keys.go
func FieldDetailKey(id int64) string { return fmt.Sprintf("field:detail:%d", id) }
func FieldListKey(hash string) string { return fmt.Sprintf("field:list:%s", hash) }
```

### 5.4 Redis 故障降级

cache 方法遇到 Redis 故障（`err != nil`）时静默跳过，不阻断主流程：

```go
if cached, hit, err := s.cache.GetList(ctx, q); err == nil && hit {
    // 命中
}
// err != nil 或 !hit：直接走 MySQL，不 return err
```

---

## 六、Model 层模式

### 6.1 DB 结构体公共字段

```go
type Xxx struct {
    ID        int64     `json:"id"         db:"id"`
    Name      string    `json:"name"       db:"name"`
    // ... 业务字段 ...
    Enabled   bool      `json:"enabled"    db:"enabled"`
    Version   int       `json:"version"    db:"version"`
    CreatedAt time.Time `json:"created_at" db:"created_at"`
    UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
    Deleted   bool      `json:"-"          db:"deleted"`   // 软删除，json 不暴露
}
```

### 6.2 请求结构体命名

| 用途 | 命名 |
|------|------|
| 列表查询 | `XxxListQuery` |
| 详情查询 | 直接用 `model.IDRequest` |
| 创建请求 | `CreateXxxRequest` |
| 更新请求 | `UpdateXxxRequest`（含 ID + Version） |
| 启用切换 | 共用 `model.ToggleEnabledRequest`（id + enabled + version） |
| 名称检查 | 共用 `model.CheckNameRequest` |

### 6.3 通用响应结构体

```go
// 列表响应（所有 List 接口统一）
type ListData struct {
    Items    any   `json:"items"`
    Total    int64 `json:"total"`
    Page     int   `json:"page"`
    PageSize int   `json:"page_size"`
}

// 删除响应
type DeleteResult struct {
    ID    int64  `json:"id"`
    Name  string `json:"name"`
    Label string `json:"label"`
}

// 名称可用性检查响应
type CheckNameResult struct {
    Available bool   `json:"available"`
    Message   string `json:"message"`
}
```

---

## 七、错误码体系

### 7.1 分层错误

```
store 层  →  哨兵错误（errors.New，不含业务含义）
service 层 →  errcode.New(ErrXxxVersionConflict) 翻译为业务码
handler 层 →  WrapCtx 将 *errcode.Error 的 code 写入响应
```

### 7.2 错误码分段

| 模块 | 范围 |
|------|------|
| 通用 | 40000 / 50000 |
| 字段管理 | 400xx（40001–40017） |
| 模板管理 | 410xx（41001–41012） |
| 事件类型 | 420xx（42001–42015） |
| 扩展字段 Schema | 420[20-39]（42020–42031） |
| 状态机配置 | 430xx（43001–43012） |
| 状态字典 | 430[13-24]（43013–43020） |

### 7.3 错误码定义要求

每个新模块在 `errcode/codes.go` 中声明自己的分段，并同步在 `messages` map 里写中文提示：

```go
const (
    ErrXxxNameExists       = 4yyyy1
    ErrXxxNotFound         = 4yyyy2
    ErrXxxVersionConflict  = 4yyyy3
    ErrXxxDeleteNotDisabled = 4yyyy4
    ErrXxxEditNotDisabled  = 4yyyy5
    // ...
)
```

---

## 八、路由约定

```
POST   /api/v1/xxx           → Create
GET    /api/v1/xxx           → List（query 参数用 ShouldBindJSON 从 body 读）
POST   /api/v1/xxx/detail    → Detail（body: {id}）
POST   /api/v1/xxx/update    → Update
POST   /api/v1/xxx/delete    → Delete
POST   /api/v1/xxx/toggle-enabled → ToggleEnabled
POST   /api/v1/xxx/check-name    → CheckName
```

GET 的 query 参数统一从 JSON body 读（通过 `ShouldBindJSON`），不用 URL query string，保持前后端一致。

---

## 九、日志规范

```go
// debug：入参（校验通过后记录，不记录敏感字段）
slog.Debug("handler.xxx.create", "name", req.Name)

// info：成功写操作
slog.Info("service.xxx创建成功", "id", id, "name", req.Name)

// warn：非致命异常（缓存失败、跨模块补字段失败）
slog.Warn("service.xxx缓存失败", "error", err)

// error：系统错误（DB 写失败、事务失败）
slog.Error("service.xxx写DB失败", "error", err)
```

---

## 十、已实现模块参考

| 模块 | handler | service | store/mysql |
|------|---------|---------|-------------|
| 字段管理 | `handler/field.go` | `service/field.go` | `store/mysql/field.go` |
| 模板管理 | `handler/template.go` | `service/template.go` | `store/mysql/template.go` |
| 事件类型 | `handler/event_type.go` | `service/event_type.go` | `store/mysql/event_type.go` |
| 扩展字段 Schema | `handler/event_type_schema.go` | `service/event_type_schema.go` | `store/mysql/event_type_schema.go` |
| 状态机配置 | `handler/fsm_config.go` | `service/fsm_config.go` | `store/mysql/fsm_config.go` |
| 状态字典 | `handler/fsm_state_dict.go` | `service/fsm_state_dict.go` | `store/mysql/fsm_state_dict.go` |
