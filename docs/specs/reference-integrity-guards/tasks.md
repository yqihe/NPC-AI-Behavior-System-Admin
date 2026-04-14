# reference-integrity-guards — 任务列表

## 状态

- [x] T1: store 层新增 SoftDeleteTx / UpdateTx
- [x] T2: EventTypeSchemaService.Delete — 事务 + FOR SHARE
- [x] T3: EventTypeSchemaService.Update — 事务 + FOR SHARE
- [x] T4: FieldService.Update — 事务 + FOR SHARE

---

## T1：store 层新增 SoftDeleteTx / UpdateTx (R4)

**涉及文件**：
- `backend/internal/store/mysql/event_type_schema.go`
- `backend/internal/store/mysql/field.go`

**做什么**：
在两个 store 文件各新增事务版方法，紧贴现有非 Tx 版本放置：

1. `EventTypeSchemaStore.SoftDeleteTx(ctx context.Context, tx *sqlx.Tx, id int64) error`
   — 与 `SoftDelete` SQL 相同，`s.db` → `tx`
2. `EventTypeSchemaStore.UpdateTx(ctx context.Context, tx *sqlx.Tx, req *model.UpdateEventTypeSchemaRequest) error`
   — 与 `Update` SQL 相同，`s.db` → `tx`
3. `FieldStore.UpdateTx(ctx context.Context, tx *sqlx.Tx, req *model.UpdateFieldRequest) error`
   — 与 `Update` SQL 相同，`s.db` → `tx`

**做完是什么样**：`go build ./...` 通过；三个方法可被 service 层调用。

---

## T2：EventTypeSchemaService.Delete — 事务 + FOR SHARE (R1)

**涉及文件**：
- `backend/internal/service/event_type_schema.go`

**做什么**：
将 `Delete` 方法的 `HasRefs + SoftDelete` 包裹在一个事务中：
- `HasRefs(ctx, id)` → `HasRefsTx(ctx, tx, id)`（FOR SHARE）
- `s.store.SoftDelete(ctx, id)` → `s.store.SoftDeleteTx(ctx, tx, id)`
- 新增 `tx` 的 error-aware defer rollback
- `schemaCache.Reload` 保持在 `tx.Commit()` 之后（内存缓存必须 Commit 后重建）
- 新增 `"database/sql"` 和 `"errors"` import（如缺少）

**做完是什么样**：Delete 方法体内有显式 `tx.BeginTxx` + `defer rollback` + `HasRefsTx` + `SoftDeleteTx` + `tx.Commit`；`go build ./...` 通过。

---

## T3：EventTypeSchemaService.Update — 事务 + FOR SHARE (R3)

**涉及文件**：
- `backend/internal/service/event_type_schema.go`

**做什么**：
将 `Update` 方法的 `HasRefs + store.Update` 包裹在一个事务中：
- `HasRefs(ctx, req.ID)` → `HasRefsTx(ctx, tx, req.ID)`（FOR SHARE）
- `s.store.Update(ctx, req)` → `s.store.UpdateTx(ctx, tx, req)`
- 新增 `tx` 的 error-aware defer rollback
- `schemaCache.Reload` 保持在 `tx.Commit()` 之后
- `ValidateConstraintsSelf` / `ValidateValue` 保持在事务内（纯计算，无副作用）

**做完是什么样**：Update 方法体内有显式 `tx.BeginTxx` + `defer rollback` + `HasRefsTx` + `UpdateTx` + `tx.Commit`；`go build ./...` 通过。

---

## T4：FieldService.Update — 事务 + FOR SHARE (R2)

**涉及文件**：
- `backend/internal/service/field.go`

**做什么**：
将 `Update` 方法的 `HasRefs + fieldStore.Update` 包裹在一个事务中：
- `s.fieldRefStore.HasRefs(ctx, req.ID)` → `s.fieldRefStore.HasRefsTx(ctx, tx, req.ID)`
- `s.fieldStore.Update(ctx, req)` → `s.fieldStore.UpdateTx(ctx, tx, req)`
- 新增 `tx` 的 error-aware defer rollback
- `s.fieldCache.DelDetail(ctx, req.ID)` + `s.fieldCache.InvalidateList(ctx)` 移到 `tx.Commit()` 之前（R3 规则）
- `syncFieldRefs` 保持在 `tx.Commit()` 之后（syncFieldRefs 自带独立事务）
- ref-affected 的 `fieldCache.DelDetail` 保持在 `syncFieldRefs` 之后
- `expose_bb` 检查（`GetByFieldID`）保持在事务外（事务开启之前）

**做完是什么样**：Update 方法体内有显式 `tx.BeginTxx` + `defer rollback` + `HasRefsTx` + `UpdateTx` + Commit 前 Redis 缓存清除 + `tx.Commit` + Commit 后 `syncFieldRefs`；`go build ./...` 通过。

---

## 执行顺序

T1 → T2 → T3 → T4

T1 必须先完成（后续 task 依赖新增的 store 方法）。T2/T3 同文件可连续执行，T4 独立。
