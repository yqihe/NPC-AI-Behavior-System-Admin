# ref-system-backend — 设计方案

## 一、场景组 A：ref_count 清理

### 方案描述

**已在 feature/ref-cleanup 分支完成**，此处记录设计决策。

#### 数据库

直接修改迁移文件 `001_create_fields.sql` 和 `003_create_templates.sql`，去掉 `ref_count` 列和覆盖索引中的 `ref_count`。DROP TABLE 重建，不保留旧数据。

#### Model

- `Field` struct：删除 `RefCount`，新增 `HasRefs bool (db:"-")`
- `FieldListItem`：删除 `RefCount`（列表不含引用信息）
- `Template`/`TemplateListItem`/`TemplateDetail`：删除 `RefCount`

#### Store

- `FieldStore`/`TemplateStore`：删除 `IncrRefCountTx`/`DecrRefCountTx`/`GetRefCountTx`
- 所有 SQL（Create/GetByID/GetByName/GetByIDs/List）去掉 `ref_count`
- `FieldRefStore`：新增 `HasRefs(ctx, fieldID) (bool, error)`（非事务版，走主键前缀）

#### Service

- `FieldService.GetByID`：返回前调 `fieldRefStore.HasRefs()` 填充 `field.HasRefs`（不缓存，每次实时查）
- `FieldService.Update`：`old.RefCount > 0` 改为 `fieldRefStore.HasRefs(ctx, req.ID)`
- `FieldService.AttachToTemplateTx`/`DetachFromTemplateTx`/`syncFieldRefs`/`Delete`：保留 field_refs 操作，删除 Incr/Decr 调用
- `TemplateService`：删除 `GetRefCountForDeleteTx`；`UpdateInTx` 删除 RefCount 检查

#### Handler

- `TemplateHandler.Delete`：删除 `GetRefCountForDeleteTx` 调用，停用后直接删除

### 备选方案

保留 ref_count 但隐藏 UI → **不选**：维护无用的跨模块事务逻辑，NPC 模块开发时负担更重。

---

## 二、场景组 B：扩展字段 → 事件类型 引用保护

### 方案描述

#### 数据库

新增迁移文件 `007_create_schema_refs.sql`：

```sql
CREATE TABLE IF NOT EXISTS schema_refs (
    schema_id   BIGINT       NOT NULL,
    ref_type    VARCHAR(16)  NOT NULL,   -- 'event_type'
    ref_id      BIGINT       NOT NULL,
    PRIMARY KEY (schema_id, ref_type, ref_id),
    INDEX idx_ref (ref_type, ref_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

结构与 `field_refs` 完全对齐，方便统一理解。

#### Store 层

新增 `store/mysql/schema_ref.go`：

```go
type SchemaRefStore struct { db *sqlx.DB }

// Add 事务内添加引用（事件类型创建/编辑时）
func (s *SchemaRefStore) Add(ctx, tx, schemaID, refType, refID)

// Remove 事务内删除单条引用
func (s *SchemaRefStore) Remove(ctx, tx, schemaID, refType, refID)

// RemoveByRef 事务内删除某个引用方的所有引用（事件类型删除时）
func (s *SchemaRefStore) RemoveByRef(ctx, tx, refType, refID) ([]int64, error)

// HasRefs 非事务检查（编辑保护）
func (s *SchemaRefStore) HasRefs(ctx, schemaID) (bool, error)

// HasRefsTx 事务内检查（删除保护，FOR SHARE）
func (s *SchemaRefStore) HasRefsTx(ctx, tx, schemaID) (bool, error)

// GetBySchemaID 查询引用方列表（references API）
func (s *SchemaRefStore) GetBySchemaID(ctx, schemaID) ([]model.SchemaRef, error)
```

#### Model 层

`event_type_schema.go` 新增：

```go
type SchemaRef struct {
    SchemaID int64  `json:"schema_id" db:"schema_id"`
    RefType  string `json:"ref_type" db:"ref_type"`
    RefID    int64  `json:"ref_id" db:"ref_id"`
}

// EventTypeSchema 结构新增
HasRefs bool `json:"has_refs" db:"-"`
```

新增 references 响应结构：

```go
type SchemaReferenceItem struct {
    RefType string `json:"ref_type"`
    RefID   int64  `json:"ref_id"`
    Label   string `json:"label"`
}

type SchemaReferenceDetail struct {
    SchemaID    int64                  `json:"schema_id"`
    FieldLabel  string                 `json:"field_label"`
    EventTypes  []SchemaReferenceItem  `json:"event_types"`
}
```

#### Service 层 — EventTypeService 维护 schema_refs

事件类型 Create/Update/Delete 需要维护 schema_refs。**关键**：事件类型当前不使用跨模块事务（Create/Update/Delete 都是单表操作）。schema_refs 维护应在同一事务内。

**方案**：事件类型 Create/Update/Delete 改为事务操作。

**Create 流程**：
1. 校验通过后开事务
2. `store.CreateTx(ctx, tx, req, configJSON)` 写 event_types
3. 对 `req.Extensions` 每个 key → `schemaCache.GetByFieldName(key)` 拿 schema_id → `schemaRefStore.Add(ctx, tx, schema_id, "event_type", id)`
4. 提交事务

**Update 流程**：
1. 拿旧 config_json，解析出旧 extension keys
2. 与新 `req.Extensions` keys diff → toAdd / toRemove
3. 开事务
4. `store.UpdateTx(ctx, tx, req, configJSON)`
5. toRemove → `schemaRefStore.Remove`；toAdd → `schemaRefStore.Add`
6. 提交事务

**Delete 流程**：
1. 开事务
2. `store.SoftDeleteTx(ctx, tx, id)`
3. `schemaRefStore.RemoveByRef(ctx, tx, "event_type", id)`
4. 提交事务

注意：事件类型 store 当前只有非事务版 Create/Update/SoftDelete，需要新增 Tx 版本。

#### Service 层 — EventTypeSchemaService 引用保护

**Update 保护**（编辑扩展字段定义时）：

```go
// 新增检查
hasRefs, err := s.schemaRefStore.HasRefs(ctx, req.ID)
if hasRefs {
    // 类型不可改（当前 Update 不含 field_type，已不可改 ✓）
    // 约束收紧检查
    if e := checkConstraintTightened(ets.FieldType, ets.Constraints, req.Constraints); e != nil {
        return e
    }
}
```

注意：`UpdateEventTypeSchemaRequest` 不含 `FieldType`（field_type 创建后不可变），所以类型变更保护天然满足。只需加约束收紧检查。

`checkConstraintTightened` 函数在 `service/field.go` 中，是私有函数。需要提取到共享位置（`service/constraint/` 包或独立函数文件），让 schema service 也能调用。

**Delete 保护**：

```go
hasRefs, err := s.schemaRefStore.HasRefsTx(ctx, tx, id)
if hasRefs {
    return errcode.New(errcode.ErrExtSchemaRefDelete)
}
```

需新增错误码 `ErrExtSchemaRefDelete`。

**GetReferences API**：

```go
func (s *EventTypeSchemaService) GetReferences(ctx, id) (*model.SchemaReferenceDetail, error)
```

查 `schema_refs WHERE schema_id = ?`，跨模块由 handler 补事件类型 label。

#### Handler 层

- `EventTypeHandler`：Create/Update/Delete 改为事务操作，维护 schema_refs
- `EventTypeSchemaHandler`：新增 `GetReferences` 端点
- 路由注册：`POST /event-type-schemas/references`

### 备选方案

隐式搜 config_json（`LIKE '%"field_name"%'`）→ **不选**：
- LIKE 搜 JSON 不精确（key 可能是 value 的子串）
- 无法 FOR SHARE 防 TOCTOU
- 每次检查全表扫描

---

## 三、场景组 C：BB Key → FSM 引用追踪

### 方案描述

#### 复用 field_refs

`field_refs` 新增 `ref_type='fsm'`：

```
field_refs(field_id=字段ID, ref_type='fsm', ref_id=FSM配置ID)
```

含义：该字段的 BB Key 被该 FSM 配置引用。

**不需要改表结构**，只是增加一种 ref_type 值。`util/const.go` 新增：

```go
const RefTypeFsm = "fsm"
```

#### BB Key 提取

新增辅助函数 `extractBBKeysFromTransitions`：

```go
func extractBBKeysFromTransitions(transitions []model.FsmTransition) map[string]bool {
    keys := make(map[string]bool)
    for _, tr := range transitions {
        collectConditionKeys(&tr.Condition, keys)
    }
    return keys
}

func collectConditionKeys(cond *model.FsmCondition, keys map[string]bool) {
    if cond.IsEmpty() { return }
    if cond.Key != "" { keys[cond.Key] = true }
    if cond.RefKey != "" { keys[cond.RefKey] = true }
    for i := range cond.And { collectConditionKeys(&cond.And[i], keys) }
    for i := range cond.Or { collectConditionKeys(&cond.Or[i], keys) }
}
```

#### BB Key name → field ID 解析

FSM 条件存的是 BB Key **名称**（字段标识符），需要解析为 field ID 才能写 field_refs。

流程：
1. 提取条件树中所有 BB Key names
2. 调 `fieldStore.GetByNames(ctx, names)` 批量查（需新增方法）
3. 只有在 fields 表中存在且 `expose_bb=true` 的才写入 field_refs
4. 运行时 Key（不来自字段）找不到对应记录，跳过

新增 `FieldStore.GetByNames`：
```go
func (s *FieldStore) GetByNames(ctx context.Context, names []string) ([]model.Field, error)
```

#### FsmConfigService 维护 field_refs

FsmConfigService 当前不持有 fieldStore/fieldRefStore。需要调整：
- **方案 A**：service 注入 fieldRefStore + fieldStore → service 内维护
- **方案 B**：handler 层编排（同模板模块模式）

按模板模块的跨模块编排模式，**选方案 B**：handler 层开事务，调 fieldService 暴露的方法维护 field_refs。

新增 `FieldService.SyncFsmBBKeyRefs`：
```go
func (s *FieldService) SyncFsmBBKeyRefs(ctx, tx, fsmID int64, oldKeys, newKeys map[string]bool) (affected []int64, err error)
```

内部逻辑：
1. diff oldKeys vs newKeys → toAdd / toRemove
2. toAdd keys → `GetByNames` 查 field ID → `fieldRefStore.Add(ctx, tx, fieldID, "fsm", fsmID)`
3. toRemove keys → 同理 Remove
4. 返回 affected field IDs（用于清缓存）

**Create**：oldKeys 为空，newKeys = 条件树提取
**Update**：oldKeys 从旧 config_json 提取，newKeys 从新 transitions 提取
**Delete**：`fieldRefStore.RemoveBySource(ctx, tx, "fsm", fsmID)` 清理全部

#### FsmConfig Handler 改造

FsmConfig 的 Create/Update/Delete 当前在 service 层完成。要加跨模块事务需改为 handler 编排：
- handler 开事务
- 调 service 的 Tx 版 Create/Update/Delete
- 调 fieldService.SyncFsmBBKeyRefs
- 提交事务
- 清缓存

需要 FsmConfigStore/FsmConfigService 新增 Tx 版方法。

#### expose_bb 取消保护

`FieldService.Update` 中，如果旧字段 `expose_bb=true` 且新字段 `expose_bb=false`：
- 检查 `field_refs WHERE field_id=? AND ref_type='fsm'` 是否有记录
- 有则拒绝，返回新错误码 `ErrFieldBBKeyInUse`（40008 已预留）

#### 字段 references API 扩展

`FieldService.GetReferences` 已返回 templates/fields 引用方。扩展：
- 查 `field_refs WHERE field_id=? AND ref_type='fsm'` → 返回 FSM 引用方
- handler 跨模块补 FSM display_name（调 `FsmConfigService.GetByIDsLite`）

`model.ReferenceDetail` 新增 `Fsms []ReferenceItem`。

### 备选方案

FSM service 内部维护 field_refs（注入 fieldRefStore）→ **不选**：
- 违反"跨模块操作在 handler 层编排"的统一模式
- FSM service 不应直接依赖字段模块的 store

---

## 四、共享逻辑提取

### checkConstraintTightened

当前在 `service/field.go` 中是私有函数。扩展字段 service 也需要调用。

**方案**：移到 `service/constraint/` 包：
```go
// constraint/tighten.go
func CheckConstraintTightened(fieldType string, oldConstraints, newConstraints json.RawMessage) *errcode.Error
```

字段 service 和扩展字段 service 都调用 `constraint.CheckConstraintTightened`。

---

## 红线检查

### 通用红线 (general.md)

| 红线 | 合规 | 说明 |
|---|---|---|
| 禁止静默降级 | OK | 所有引用检查失败都返回明确错误码 |
| 禁止过度工程 | OK | schema_refs 和 field_refs 结构对齐，不引入新框架 |

### Go 红线 (go.md)

| 红线 | 合规 | 说明 |
|---|---|---|
| 禁止 nil slice 输出 null | OK | 所有 make([]T, 0) 初始化 |
| 禁止错误码语义混用 | OK | 新增独立错误码 ErrExtSchemaRefDelete |
| 禁止层级倒置 | OK | handler 编排跨模块事务，service 不跨模块 |
| 禁止硬编码魔法字符串 | OK | ref_type 用 util 常量 |

### MySQL 红线 (mysql.md)

| 红线 | 合规 | 说明 |
|---|---|---|
| 禁止事务内混用 db 和 tx | OK | 所有 Tx 版方法使用 tx 参数 |
| 禁止 TOCTOU 不加锁 | OK | 删除前 HasRefsTx 使用 FOR SHARE |

### Cache 红线 (cache.md)

| 红线 | 合规 | 说明 |
|---|---|---|
| 禁止写操作后不清缓存 | OK | 事件类型 CRUD 后清 Redis 缓存；扩展字段 CRUD 后 Reload 内存缓存 |
| 禁止只清 list 不清 detail | OK | 两类都清 |

### ADMIN 红线 (admin/red-lines.md)

| 红线 | 合规 | 说明 |
|---|---|---|
| 禁止偏离跨模块代码模式 | OK | handler 编排事务，service Tx 版方法 |
| 禁止新模块 store Create 用展开位置参数 | OK | 用 Request struct |
| 禁止引用完整性破坏 | OK | 正是本 spec 要修复的 |

---

## 扩展性影响

**正面**：
- 新模块（如 NPC/BT）只需在 handler 层加跨模块事务编排，维护 field_refs/schema_refs
- 统一的 refs 表模式降低学习成本
- `CheckConstraintTightened` 提取为共享函数后可被任何模块复用

---

## 依赖方向

```
handler (event_type / fsm_config / field / event_type_schema)
  ├── service (event_type / fsm_config / field / event_type_schema)
  │     ├── store/mysql (event_type / fsm_config / field / field_ref / schema_ref)
  │     ├── store/redis (event_type / fsm_config / field)
  │     ├── cache (event_type_schema)
  │     └── service/constraint (CheckConstraintTightened)
  └── [跨模块] handler 编排多个 service
```

所有依赖单向向下，handler 是唯一跨模块编排点。

---

## 陷阱检查

### MySQL (dev-rules/mysql.md)

- **事务内用 tx**：所有新增 Tx 方法都接收 `*sqlx.Tx`，不混用 `s.db` ✓
- **覆盖索引**：schema_refs 体量极小（< 500 行），无需覆盖索引 ✓
- **migration 合并**：新增 `007_create_schema_refs.sql`，V3 预上线阶段可合并 ✓

### Go (dev-rules/go.md)

- **nil slice**：所有返回 slice 的方法用 `make([]T, 0)` ✓
- **context 超时**：field_refs/schema_refs 直查走 db/tx 连接池超时 ✓

### Cache (dev-rules/cache.md)

- **has_refs 不缓存**：Field.HasRefs 每次实时查 field_refs，不进 Redis ✓
- **事件类型 CRUD 后清缓存**：改为事务操作后，commit 后清缓存 ✓

---

## 配置变更

无。不涉及 config.yaml 或 JSON 配置文件变更。

---

## 测试策略

### 后端编译

- `go build ./...` 通过

### 集成测试

现有 `tests/integration_test.sh` 中涉及 ref_count 的断言需要更新。新增：

- 扩展字段 CRUD + 引用保护测试：
  - 创建扩展字段 → 创建事件类型引用 → 编辑扩展字段收紧约束 → 被拒绝
  - 删除扩展字段 → 被拒绝（有引用）→ 删除事件类型 → 删除扩展字段成功
- FSM BB Key 追踪测试：
  - 创建字段(expose_bb) → 创建 FSM 引用 BB Key → 删除字段 → 被拒绝
  - 取消 expose_bb → 被拒绝（FSM 引用）→ 删除 FSM → 取消成功

### 手动验证

- curl 测试各 API 端点
- 确认 schema_refs/field_refs 数据正确写入
