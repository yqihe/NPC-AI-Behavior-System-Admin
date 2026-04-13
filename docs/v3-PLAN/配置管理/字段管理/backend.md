# 字段管理 — 后端设计

> 权威参考文档，与代码一一对齐。最后同步：2026-04-13

---

## 1. 目录结构

```
backend/internal/
  handler/field.go            # HTTP handler：请求参数校验 + 跨模块编排
  service/field.go            # 业务逻辑：Cache-Aside + 引用保护 + 循环检测 + 跨模块对外方法
  store/mysql/field.go        # fields 表 CRUD + 覆盖索引 + 乐观锁 + 事务内软删除
  store/mysql/field_ref.go    # field_refs 关联表全部操作
  store/redis/field_cache.go  # Detail/List Redis 缓存 + 版本号方案 + 分布式锁
  store/redis/config/keys.go  # Redis key 模式定义
  store/redis/config/common.go # TTL 常量 + 抖动函数
  model/field.go              # Field/FieldLite/FieldListItem/FieldRef/FieldProperties + 全部 DTO
  errcode/codes.go            # 40001-40017 字段模块错误码
  util/constraint.go          # CheckConstraintTightened + ValidateValue + ValidateConstraintsSelf（共用）
  util/const.go               # FieldTypeReference / RefTypeTemplate / RefTypeField / RefTypeFsm 常量
```

### handler 依赖

```go
type FieldHandler struct {
    fieldService     *service.FieldService
    templateService  *service.TemplateService  // 跨模块：GetReferences 补 template label
    fsmConfigService *service.FsmConfigService // 跨模块：GetReferences 补 FSM display_name
    valCfg           *config.ValidationConfig
}
```

### service 依赖

```go
type FieldService struct {
    fieldStore    *storemysql.FieldStore
    fieldRefStore *storemysql.FieldRefStore
    fieldCache    *storeredis.FieldCache
    dictCache     *cache.DictCache
    pagCfg        *config.PaginationConfig
}
```

---

## 2. 数据表

### 2.1 fields 表

```sql
CREATE TABLE IF NOT EXISTS fields (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    name            VARCHAR(64)  NOT NULL,              -- 字段标识，唯一，创建后不可变
    label           VARCHAR(128) NOT NULL,              -- 中文标签（搜索用）
    type            VARCHAR(32)  NOT NULL,              -- 字段类型（筛选用）
    category        VARCHAR(32)  NOT NULL,              -- 标签分类（筛选用）
    properties      JSON         NOT NULL,              -- 动态属性（描述/BB Key/默认值/约束等）

    enabled         TINYINT(1)   NOT NULL DEFAULT 0,    -- 启用状态（0=停用，1=启用）
    version         INT          NOT NULL DEFAULT 1,    -- 乐观锁
    deleted         TINYINT(1)   NOT NULL DEFAULT 0,    -- 软删除
    created_at      DATETIME     NOT NULL,
    updated_at      DATETIME     NOT NULL,

    UNIQUE KEY uk_name (name),
    INDEX idx_list (deleted, id, name, label, type, category, enabled, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**索引说明**：

| 索引 | 类型 | 用途 |
|------|------|------|
| `PRIMARY KEY (id)` | 聚簇索引 | 主键查询、GetByID、GetByIDs |
| `uk_name (name)` | 唯一索引 | name 唯一性校验（ExistsByName）、GetByName、GetByNames |
| `idx_list (deleted, id, name, label, type, category, enabled, created_at)` | 覆盖索引 | 列表查询不回表。`deleted` 在最前确保分区。包含 `enabled` 列支持 `Enabled` 筛选 |

**关键设计**：

- 无 `ref_count` 列。引用计数完全由 `field_refs` 表管理。
- `HasRefs bool (db:"-")`：Field 结构体中有此字段，但不映射 DB 列，由 service 层实时查 field_refs 填充。
- `name` 创建后不可变，因此 `UpdateFieldRequest` 中无 name 字段。
- `ExistsByName` 查询不带 `deleted=0` 条件，软删除的 name 也占用（防止重名导致数据混淆）。

### 2.2 field_refs 表

```sql
CREATE TABLE IF NOT EXISTS field_refs (
    field_id    BIGINT       NOT NULL,              -- 被引用的字段 ID
    ref_type    VARCHAR(16)  NOT NULL,              -- 引用来源：'template' / 'field' / 'fsm'
    ref_id      BIGINT       NOT NULL,              -- 引用方 ID（模板 ID / 字段 ID / FSM ID）

    PRIMARY KEY (field_id, ref_type, ref_id),
    INDEX idx_ref (ref_type, ref_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**索引说明**：

| 索引 | 类型 | 用途 |
|------|------|------|
| `PRIMARY KEY (field_id, ref_type, ref_id)` | 联合主键 | 天然去重（INSERT IGNORE 幂等）+ HasRefs/HasRefsTx 按 field_id 前缀扫描 + GetByFieldID |
| `idx_ref (ref_type, ref_id)` | 二级索引 | RemoveBySource 按引用方反查（如删除模板时清理该模板的所有字段引用） |

**ref_type 取值**：

| 值 | 含义 | 写入方 |
|----|------|--------|
| `template` | 模板引用字段 | `AttachToTemplateTx` / `DetachFromTemplateTx` |
| `field` | reference 字段引用字段 | `syncFieldRefs`（service 内部） |
| `fsm` | FSM 条件引用 BB Key 对应字段 | `SyncFsmBBKeyRefs` / `CleanFsmBBKeyRefs` |

---

## 3. API 接口

所有端点前缀：`POST /api/v1/fields/<action>`

| # | 方法 | 路径 | 请求体 | 响应 data | 可能错误码 |
|---|------|------|--------|-----------|-----------|
| 1 | POST | `/fields/list` | `FieldListQuery{label, type, category, enabled, page, page_size}` | `ListData{items: FieldListItem[], total, page, page_size}` | - |
| 2 | POST | `/fields/create` | `CreateFieldRequest{name, label, type, category, properties}` | `CreateFieldResponse{id, name}` | 40001, 40002, 40003, 40004, 40017, 40014, 40013, 40016, 40009 |
| 3 | POST | `/fields/detail` | `IDRequest{id}` | `Field{..., has_refs}` | 40011 |
| 4 | POST | `/fields/update` | `UpdateFieldRequest{id, label, type, category, properties, version}` | `"保存成功"` | 40015, 40006, 40007, 40008, 40010, 40003, 40004, 40017, 40014, 40013, 40016, 40009 |
| 5 | POST | `/fields/delete` | `IDRequest{id}` | `DeleteResult{id, name, label}` | 40012, 40005, 40011 |
| 6 | POST | `/fields/check-name` | `CheckNameRequest{name}` | `CheckNameResult{available, message}` | - |
| 7 | POST | `/fields/toggle-enabled` | `ToggleEnabledRequest{id, enabled, version}` | `"操作成功"` | 40010, 40011 |
| 8 | POST | `/fields/references` | `IDRequest{id}` | `ReferenceDetail{field_id, field_label, templates[], fields[], fsms[]}` | 40011 |

**handler 层校验清单**（纯格式校验，不查 DB）：

| 端点 | 校验项 |
|------|--------|
| create | name 非空 + `IdentPattern` 正则 + 长度上限；label 非空 + 长度上限；type 非空；category 非空；properties 必须是 JSON 对象 |
| update | id > 0；label 非空 + 长度上限；type 非空；category 非空；properties 必须是 JSON 对象；version > 0 |
| delete / detail / references | id > 0 |
| check-name | name 非空 |
| toggle-enabled | id > 0；version > 0 |

---

## 4. 跨模块事务编排

### 4.1 GetReferences 补 label（handler 层编排）

`FieldService.GetReferences` 只返回字段模块内的数据：
- `fields[]` 中的 label 由 service 通过 `fieldStore.GetByIDs` 补齐
- `templates[]` 和 `fsms[]` 中的 label 留空

`FieldHandler.GetReferences` 负责跨模块补齐：
1. 提取 `detail.Templates` 中所有 `RefID` → 调 `templateService.GetByIDsLite(ctx, ids)` → 按 ID 映射 label → 写回
2. 遍历 `detail.Fsms` → 逐个调 `fsmConfigService.GetByID(ctx, id)` → 取 `DisplayName` → 写回
3. 若跨模块查询失败，模板侧返回错误，FSM 侧 `slog.Warn` 后继续（降级为空 label）

### 4.2 reference 字段的引用同步（service 层内部事务）

`syncFieldRefs` 方法：
1. Diff oldRefIDs vs newRefIDs → 计算 toAdd / toRemove
2. 开事务 → 对 toAdd 调 `fieldRefStore.Add`，对 toRemove 调 `fieldRefStore.Remove`
3. Commit → 返回 affected field IDs → 外层清这些字段的 detail 缓存

### 4.3 删除事务编排

```
service.Delete:
  1. getFieldOrNotFound
  2. 检查 enabled=false（否则 40012）
  3. BeginTxx
  4. HasRefsTx(FOR SHARE) → 有引用返回 40005
  5. SoftDeleteTx(tx, id)
  6. 若 type=reference → RemoveBySource(tx, 'field', id)
  7. Commit
  8. 清缓存（自身 + affected）
```

---

## 5. 缓存策略

### 5.1 Detail 缓存（单条）

| 项 | 值 |
|----|-----|
| Key 模式 | `fields:detail:{id}` |
| TTL | 5min + 0~30s 随机抖动 |
| 读路径 | Redis hit → 返回；Redis miss → 分布式锁 → double-check → MySQL → 写 Redis |
| 写路径 | Create/Update/Delete/ToggleEnabled 后 `DelDetail` |
| 空值保护 | `field=nil` 时缓存 `{"_null":true}` 标记，防缓存穿透 |
| 击穿保护 | `SetNX` 分布式锁，key=`fields:lock:{id}`，expire=3s |

### 5.2 List 缓存（分页）

| 项 | 值 |
|----|-----|
| Key 模式 | `fields:list:v{version}:{type}:{category}:{label}:{enabled}:{page}:{pageSize}` |
| 版本号 Key | `fields:list:version`（全局单一） |
| TTL | 1min + 0~10s 随机抖动 |
| 失效方式 | 任何写操作后 `INCR fields:list:version`，旧版本 key 自然过期（无需 SCAN） |

### 5.3 has_refs 不缓存

`HasRefs` 每次实时查 `field_refs` 表。原因：引用关系随模板/reference 字段/FSM 的操作频繁变化，缓存一致性维护成本高于直接查表。`COUNT(*)` 走联合主键前缀扫描，性能可接受。

---

## 6. 核心逻辑

### 6.1 编辑保护（service.Update 完整流程）

```
1. checkTypeExists(req.Type) → 40003
2. checkCategoryExists(req.Category) → 40004
3. getFieldOrNotFound(req.ID) → 40011
4. old.Enabled == true → 40015 ErrFieldEditNotDisabled
5. fieldRefStore.HasRefs(ctx, req.ID) → hasRefs
6. hasRefs && old.Type != req.Type → 40006 ErrFieldRefChangeType
7. hasRefs && old.Type == req.Type → util.CheckConstraintTightened → 40007 ErrFieldRefTighten
8. req.Type == "reference" → validateReferenceRefs(ctx, req.ID, newRefIDs, oldRefSet)
   → 40017/40014/40013/40016/40009
9. expose_bb: true→false 且 field_refs 中存在 ref_type='fsm' → 40008 ErrFieldBBKeyInUse
10. fieldStore.Update(ctx, req) → 乐观锁，失败 40010
11. type=reference → syncFieldRefs 同步引用关系
    旧 type=reference 新 type 非 reference → syncFieldRefs 清除所有引用
12. 清缓存：DelDetail(self) + DelDetail(affected) + InvalidateList
```

### 6.2 expose_bb 取消保护

```go
// service.Update 中的检查逻辑
oldProps, _ := parseProperties(old.Properties)
newProps, _ := parseProperties(req.Properties)
if oldProps != nil && newProps != nil && oldProps.ExposeBB && !newProps.ExposeBB {
    refs, err := s.fieldRefStore.GetByFieldID(ctx, req.ID)
    // ...
    for _, r := range refs {
        if r.RefType == util.RefTypeFsm {
            return errcode.New(errcode.ErrFieldBBKeyInUse) // 40008
        }
    }
}
```

### 6.3 删除保护（HasRefsTx FOR SHARE）

```go
// service.Delete 中的事务保护
tx, err := s.fieldStore.DB().BeginTxx(ctx, nil)
// ...
hasRefs, err := s.fieldRefStore.HasRefsTx(ctx, tx, id)  // SELECT COUNT(*) ... FOR SHARE
if hasRefs {
    return nil, errcode.New(errcode.ErrFieldRefDelete) // 40005
}
if err := s.fieldStore.SoftDeleteTx(ctx, tx, id); err != nil { ... }
// reference 类型额外清理
if field.Type == util.FieldTypeReference {
    affectedIDs, err = s.fieldRefStore.RemoveBySource(ctx, tx, util.RefTypeField, id)
}
tx.Commit()
```

`FOR SHARE` 的作用：在当前事务提交前，阻止其他事务对匹配行执行 INSERT/DELETE，防止"查询时无引用 → 并发写入引用 → 删除"的 TOCTOU 竞态。

### 6.4 约束收紧检查（util.CheckConstraintTightened）

签名：
```go
func CheckConstraintTightened(fieldType string, oldConstraints, newConstraints json.RawMessage, errCode int) *errcode.Error
```

字段模块调用时 `errCode = errcode.ErrFieldRefTighten (40007)`。

检查规则：

| 类型 | 收紧条件 |
|------|---------|
| integer / int / float | newMin > oldMin，或 newMax < oldMax |
| float 额外 | newPrecision < oldPrecision |
| string | newMinLength > oldMinLength，或 newMaxLength < oldMaxLength，或 pattern 变更 |
| select | 已有 option 被删除，或 newMinSelect > oldMinSelect，或 newMaxSelect < oldMaxSelect |
| bool | 无约束，不检查 |

### 6.5 循环引用检测（detectCyclicRef）

DFS 算法，visited 集合 + 递归：
1. 将 currentID（编辑中的字段）加入 visited
2. 对每个 refID：若在 visited 中 → 40009 循环引用
3. 标记 visited，查该字段 → 若 type=reference 则递归其 constraints.refs
4. 创建时 currentID=0，不加入 visited

### 6.6 reference 字段 refs 校验（validateReferenceRefs）

```go
func (s *FieldService) validateReferenceRefs(ctx context.Context, currentID int64, newRefIDs []int64, oldRefSet map[int64]bool) error
```

规则链：
1. `len(newRefIDs) == 0` → 40017 ErrFieldRefEmpty
2. 每个 refID 查 `fieldStore.GetByID` → nil → 40014 ErrFieldRefNotFound
3. 若 `oldRefSet[refID]` 为 true → 跳过（存量不动）
4. `!f.Enabled` → 40013 ErrFieldRefDisabled
5. `f.Type == "reference"` → 40016 ErrFieldRefNested
6. `detectCyclicRef(ctx, currentID, newRefIDs)` → 40009 ErrFieldCyclicRef

---

## 7. 跨模块对外方法

### FieldService 对外方法表

| 方法签名 | 调用方 | 说明 |
|----------|--------|------|
| `ValidateFieldsForTemplate(ctx context.Context, fieldIDs []int64) error` | 模板 handler | 校验字段全部存在（41006）+ 启用（41005）+ 非 reference（41012）。错误码归模板段位 |
| `AttachToTemplateTx(ctx context.Context, tx *sqlx.Tx, templateID int64, fieldIDs []int64) ([]int64, error)` | 模板 handler | 事务内写 field_refs(template)，返回 affected IDs |
| `DetachFromTemplateTx(ctx context.Context, tx *sqlx.Tx, templateID int64, fieldIDs []int64) ([]int64, error)` | 模板 handler | 事务内删 field_refs(template)，返回 affected IDs |
| `GetByIDsLite(ctx context.Context, fieldIDs []int64) ([]model.FieldLite, error)` | 模板 handler | 批量查精简信息，含 CategoryLabel 翻译。缺失位返回 zero FieldLite{ID:0} |
| `InvalidateDetails(ctx context.Context, fieldIDs []int64)` | 模板/FSM handler | 批量清 detail 缓存。失败仅 slog.Error |
| `SyncFsmBBKeyRefs(ctx context.Context, tx *sqlx.Tx, fsmID int64, oldKeys, newKeys map[string]bool) ([]int64, error)` | FSM handler | 同步 FSM 条件中 BB Key 引用关系。内部调 `fieldStore.GetByNames` 解析 name→ID，运行时 Key 跳过 |
| `CleanFsmBBKeyRefs(ctx context.Context, tx *sqlx.Tx, fsmID int64) ([]int64, error)` | FSM handler | FSM 删除时清理所有 BB Key 引用。委托 `fieldRefStore.RemoveBySource(tx, 'fsm', fsmID)` |

### FieldStore 对外方法

| 方法签名 | 用途 |
|----------|------|
| `DB() *sqlx.DB` | service 层开事务 |
| `Create(ctx, req) (int64, error)` | 插入 |
| `GetByID(ctx, id) (*Field, error)` | 主键查询 |
| `GetByName(ctx, name) (*Field, error)` | UK 查询 |
| `GetByNames(ctx, names) ([]Field, error)` | 批量 UK 查询（FSM BB Key name→ID 解析） |
| `GetByIDs(ctx, ids) ([]Field, error)` | 批量主键查询 |
| `ExistsByName(ctx, name) (bool, error)` | 唯一性检查（含软删除） |
| `List(ctx, q) ([]FieldListItem, int64, error)` | 分页列表 |
| `Update(ctx, req) error` | 乐观锁更新 |
| `SoftDeleteTx(ctx, tx, id) error` | 事务内软删除 |
| `ToggleEnabled(ctx, id, enabled, version) error` | 乐观锁切换启用 |

### FieldRefStore 方法

| 方法签名 | 用途 |
|----------|------|
| `Add(ctx, tx, fieldID, refType, refID) error` | 事务内添加引用（INSERT IGNORE） |
| `Remove(ctx, tx, fieldID, refType, refID) error` | 事务内删除引用 |
| `RemoveBySource(ctx, tx, refType, refID) ([]int64, error)` | 按引用方删除所有引用，返回被引用字段 IDs |
| `GetByFieldID(ctx, fieldID) ([]FieldRef, error)` | 非事务查某字段的所有引用方 |
| `HasRefs(ctx, fieldID) (bool, error)` | 非事务检查引用（编辑前） |
| `HasRefsTx(ctx, tx, fieldID) (bool, error)` | 事务内检查引用（FOR SHARE，删除前） |

---

## 8. 错误码

| 错误码 | 常量名 | 默认消息 | 触发场景 |
|--------|--------|---------|---------|
| 40001 | `ErrFieldNameExists` | 字段标识已存在 | Create 时 name 重复（含软删除） |
| 40002 | `ErrFieldNameInvalid` | 字段标识格式不合法，需小写字母开头，仅允许 a-z、0-9、下划线 | Create handler 校验 name 格式/长度 |
| 40003 | `ErrFieldTypeNotFound` | 字段类型不存在 | Create/Update 时 type 不在字典表中 |
| 40004 | `ErrFieldCategoryNotFound` | 标签分类不存在 | Create/Update 时 category 不在字典表中 |
| 40005 | `ErrFieldRefDelete` | 该字段正被引用，无法删除 | Delete 时 HasRefsTx 检测到引用 |
| 40006 | `ErrFieldRefChangeType` | 该字段已被引用，无法修改类型 | Update 时 HasRefs=true 且 type 变更 |
| 40007 | `ErrFieldRefTighten` | 已有数据可能超出新约束范围，请先移除引用 | Update 时 HasRefs=true 且约束被收紧 |
| 40008 | `ErrFieldBBKeyInUse` | 该 BB Key 正被 FSM/BT 引用，无法关闭暴露 | Update 时 expose_bb true→false 且存在 ref_type='fsm' |
| 40009 | `ErrFieldCyclicRef` | 检测到循环引用 | reference 字段的 refs 形成环 |
| 40010 | `ErrFieldVersionConflict` | 该字段已被其他人修改，请刷新后重试 | Update/ToggleEnabled 乐观锁失败 |
| 40011 | `ErrFieldNotFound` | 字段不存在 | 任何按 ID 查询未找到 |
| 40012 | `ErrFieldDeleteNotDisabled` | 请先停用该字段再删除 | Delete 时 enabled=true |
| 40013 | `ErrFieldRefDisabled` | 不能引用已停用的字段 | reference 字段新增 ref 目标已停用 |
| 40014 | `ErrFieldRefNotFound` | 引用的字段不存在 | reference 字段 ref 目标不存在 |
| 40015 | `ErrFieldEditNotDisabled` | 请先停用该字段再编辑 | Update 时 enabled=true |
| 40016 | `ErrFieldRefNested` | 不能引用 reference 类型字段，禁止嵌套 | reference 字段新增 ref 目标也是 reference |
| 40017 | `ErrFieldRefEmpty` | reference 字段必须至少引用一个目标字段 | reference 字段 refs 为空 |
