# reference-integrity-guards — 设计方案

## 方案描述

### 核心思路

将所有「HasRefs 检查 + 状态修改」的路径包裹在同一个 MySQL 事务中，用 `FOR SHARE` 锁定 `field_refs` / `schema_refs` 行，消除 TOCTOU 窗口。

这是已有的正确示范模式（`FieldService.Delete`）在遗漏路径上的扩展，无新概念引入。

### 需要新增的 Store 方法

`HasRefsTx`（FOR SHARE 版本）三个 Store 都已有，无需新增。缺少的是事务版写操作：

| Store | 新增方法 | 说明 |
|---|---|---|
| `EventTypeSchemaStore` | `SoftDeleteTx(ctx, tx, id)` | 事务内软删除 |
| `EventTypeSchemaStore` | `UpdateTx(ctx, tx, req)` | 事务内乐观锁更新 |
| `FieldStore` | `UpdateTx(ctx, tx, req)` | 事务内乐观锁更新 |

实现与已有的非事务版完全相同，仅将 `s.db.ExecContext` 替换为 `tx.ExecContext`。

### R1：EventTypeSchemaService.Delete 修复

**修复前**（TOCTOU）：
```
HasRefs(ctx, id)        ← 无锁查询
[窗口：并发可写 schema_refs]
SoftDelete(ctx, id)     ← 无事务
```

**修复后**：
```go
func (s *EventTypeSchemaService) Delete(ctx context.Context, id int64) error {
    ets, err := s.getOrNotFound(ctx, id)
    if ets.Enabled {
        return errcode.New(errcode.ErrExtSchemaDeleteNotDisabled)
    }

    tx, err := s.store.DB().BeginTxx(ctx, nil)
    if err != nil { return fmt.Errorf("begin tx: %w", err) }
    defer func() {
        if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
            slog.Warn("service.删除扩展字段事务回滚失败", "error", rbErr)
        }
    }()

    hasRefs, err := s.schemaRefStore.HasRefsTx(ctx, tx, id)   // FOR SHARE
    if err != nil {
        slog.Error("service.查询扩展字段引用失败", "error", err, "id", id)
        return fmt.Errorf("check schema refs: %w", err)
    }
    if hasRefs {
        return errcode.New(errcode.ErrExtSchemaRefDelete)
    }

    if err := s.store.SoftDeleteTx(ctx, tx, id); err != nil {  // 事务内删
        if errors.Is(err, errcode.ErrNotFound) {
            return errcode.New(errcode.ErrExtSchemaNotFound)
        }
        return err
    }

    if err := tx.Commit(); err != nil {
        return fmt.Errorf("commit: %w", err)
    }

    // 内存缓存必须在 Commit 成功后 Reload（Reload 是全量 DB 查询，Commit 前读到旧数据）
    if err := s.schemaCache.Reload(ctx); err != nil {
        slog.Error("service.删除扩展字段-重载缓存失败", "error", err)
    }
    slog.Info("service.删除扩展字段成功", "id", id)
    return nil
}
```

> **注**：`schemaCache` 是内存缓存，`Reload` 是全量重查 DB，必须在 `Commit` 后执行（Commit 前查到的是旧数据）。这与 Redis cache 的"清除在 Commit 前"规则不冲突——Redis 清的是缓存 key（失效即可），内存缓存做的是全量重建（需要已提交的数据）。

### R2：FieldService.Update 修复

**修复前**（TOCTOU）：
```
HasRefs(ctx, id)           ← 无锁
[窗口：并发可写 field_refs]
fieldStore.Update(ctx, req) ← 无事务
```

**修复后**（只展示关键变化，其余逻辑不变）：
```go
func (s *FieldService) Update(ctx context.Context, req *model.UpdateFieldRequest) error {
    // ... 同原有：类型/分类检查、properties 校验、getFieldOrNotFound、Enabled 检查 ...
    // ... expose_bb 检查（读操作，较低 TOCTOU 风险，保持现有非事务查询）...

    // ── 新：开事务，锁定 field_refs，原子执行 HasRefs + Update ──
    tx, err := s.fieldStore.DB().BeginTxx(ctx, nil)
    if err != nil { return fmt.Errorf("begin tx: %w", err) }
    defer func() {
        if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
            slog.Warn("service.编辑字段事务回滚失败", "error", rbErr)
        }
    }()

    hasRefs, err := s.fieldRefStore.HasRefsTx(ctx, tx, req.ID)  // FOR SHARE
    if err != nil {
        slog.Error("service.查询字段引用失败", "error", err, "id", req.ID)
        return fmt.Errorf("check field refs: %w", err)
    }
    if old.Type != req.Type && hasRefs {
        return errcode.New(errcode.ErrFieldRefChangeType)
    }
    if hasRefs && old.Type == req.Type {
        // ... CheckConstraintTightened（逻辑不变）...
    }

    // ... reference 类型：validateReferenceRefs（只读，在事务内调用无问题）...

    // 乐观锁 UPDATE（事务内）
    if err = s.fieldStore.UpdateTx(ctx, tx, req); err != nil {
        if errors.Is(err, errcode.ErrVersionConflict) {
            return errcode.New(errcode.ErrFieldVersionConflict)
        }
        return fmt.Errorf("update field: %w", err)
    }

    // ── 缓存清除在 Commit 前（R3 规则）──
    s.fieldCache.DelDetail(ctx, req.ID)
    s.fieldCache.InvalidateList(ctx)

    if err := tx.Commit(); err != nil {
        return fmt.Errorf("commit: %w", err)
    }

    // ── Commit 后：同步引用关系（syncFieldRefs 自带事务，与主事务独立）──
    // ... syncFieldRefs / clear ref-affected caches（逻辑不变）...
    return nil
}
```

> **关键**：`syncFieldRefs` 自带独立事务，放在主事务提交后调用，不产生嵌套事务问题。`expose_bb` 检查使用非事务读（`GetByFieldID`），风险低（仅可能过度拒绝，不会放行非法操作），维持现状。

### R3：EventTypeSchemaService.Update 修复

与 R2 同构，只展示差异：

```go
func (s *EventTypeSchemaService) Update(ctx context.Context, req *model.UpdateEventTypeSchemaRequest) error {
    ets, err := s.getOrNotFound(ctx, req.ID)
    ...

    tx, err := s.store.DB().BeginTxx(ctx, nil)
    defer error-aware-rollback

    hasRefs, err := s.schemaRefStore.HasRefsTx(ctx, tx, req.ID)   // FOR SHARE
    if hasRefs {
        if e := CheckConstraintTightened(...); e != nil { return e }
    }
    // ... ValidateConstraintsSelf, ValidateValue（纯计算，与事务无关）...

    if err := s.store.UpdateTx(ctx, tx, req); err != nil {  // 事务内 UPDATE
        if errors.Is(err, errcode.ErrVersionConflict) {
            return errcode.New(errcode.ErrExtSchemaVersionConflict)
        }
        return err
    }

    if err := tx.Commit(); err != nil {
        return fmt.Errorf("commit: %w", err)
    }

    // Commit 后重载内存缓存
    if err := s.schemaCache.Reload(ctx); err != nil {
        slog.Error("service.编辑扩展字段-重载缓存失败", "error", err)
    }
    slog.Info("service.编辑扩展字段成功", "id", req.ID)
    return nil
}
```

## 方案对比

### 备选方案 A：Redis 分布式锁保护 check-then-act

```go
lockID, _ := s.cache.TryLock(ctx, id, ttl)
if lockID == "" { return ErrConflict }
defer s.cache.Unlock(ctx, id, lockID)

hasRefs, _ := s.refStore.HasRefs(ctx, id)   // 仍然无事务锁
... 操作 ...
```

**不选原因**：
1. Redis 锁与 MySQL 事务是两个系统，不能保证原子性。Redis 锁到期或宕机时，MySQL 层仍无保护。
2. 本项目已明确规则：DB 状态一致性用 MySQL FOR SHARE，不用 Redis 锁（Redis 锁用于热点资源竞争保护，不用于 ACID 替代）。
3. 代码更复杂，锁超时处理容易出错。

### 备选方案 B：乐观锁 + 回滚（更新后重检 hasRefs）

```go
hasRefs := HasRefs(id)   // 无锁读
update(id)
// 更新后再次检查 hasRefs
if HasRefs(id) && !hasRefsBefore { rollback... }
```

**不选原因**：
1. "先做后检查" 逻辑违背防御性设计原则；update 可能已触发部分副作用。
2. 两次 HasRefs 查询之间仍有窗口，且回滚逻辑复杂。
3. MySQL 乐观锁已处理版本冲突，不需要在应用层再实现一套。

**选定方案**：MySQL 事务 + `FOR SHARE`，与项目现有 `FieldService.Delete` 完全同构，零新概念引入。

## 红线检查

对照 `docs/development/admin/red-lines.md`（截至本 spec 最新版）：

| 红线 | 检查结果 |
|---|---|
| §16：缓存清除在 `tx.Commit()` 之前 | ✓ `DelDetail`/`InvalidateList` 在 Commit 前；`schemaCache.Reload` 是内存重建必须在 Commit 后，符合备注 |
| §17：Unlock 必须传 lockID | ✓ 本 spec 不涉及分布式锁 |
| §18：`defer tx.Rollback()` error-aware | ✓ 三处均用 `!errors.Is(rbErr, sql.ErrTxDone)` 过滤 |
| §4b：被引用时约束只能放宽（`CheckConstraintTightened`）| ✓ 逻辑不变，仅锁住执行窗口 |
| §4b：被引用时类型不可变 | ✓ 逻辑不变，仅锁住执行窗口 |
| MySQL 红线：事务内查询用 tx | ✓ `HasRefsTx` / `UpdateTx` / `SoftDeleteTx` 均使用 tx |
| MySQL 红线：乐观锁 rows==0 返回 ErrVersionConflict | ✓ `UpdateTx` 继承原有 rows check |
| MySQL 红线：TOCTOU 防护用 FOR SHARE | ✓ 这正是本 spec 的修复目标 |
| Go 红线：error 不忽略 | ✓ 所有 err 均有处理 |
| ADMIN 专属红线：禁止外键约束 | ✓ 不涉及 DDL |

## 扩展性影响

- **新增配置类型**：正面影响。修复后的事务 + FOR SHARE 模式是新配置类型应遵循的标准模板，`FieldService.Delete` 和本 spec 后的 `EventTypeSchemaService.Delete` 共同形成完整的参考示例。
- **新增表单字段**：不涉及。

## 依赖方向

```
handler
  └── service/event_type_schema.go   (修改 Delete / Update)
  └── service/field.go               (修改 Update)
        └── store/mysql/event_type_schema.go  (新增 SoftDeleteTx / UpdateTx)
        └── store/mysql/field.go              (新增 UpdateTx)
              └── store/mysql/schema_ref.go   (HasRefsTx 已有)
              └── store/mysql/field_ref.go    (HasRefsTx 已有)
```

依赖单向向下，无循环。

## 陷阱检查

### MySQL
- `UpdateTx` 与 `Update` SQL 完全相同，只是从 `s.db` 换成 `tx`。乐观锁 `rows==0` 语义：返回 `errcode.ErrVersionConflict`（哨兵错误），service 层 `errors.Is` 转为业务码。✓
- FOR SHARE 锁住 schema_refs / field_refs 的行，并发写同一行会阻塞直到事务结束——不会死锁，因为写方（attachSchemaRefs / syncFieldRefs）不持有 DELETE 方持有的行锁。✓

### 内存缓存 vs Redis 缓存的 Commit 时序
- **Redis 缓存**（FieldCache）：`DelDetail` / `InvalidateList` 在 Commit 前执行（R3 规则）。Commit 失败时缓存已清无害，下次读重建；Commit 成功后不会有脏读窗口。
- **内存缓存**（EventTypeSchemaCache）：`Reload` 是全量重查 DB，必须在 Commit 后执行。Commit 前 Reload 会读到旧数据，导致缓存与 DB 不一致。这是两类缓存在时序上唯一的差异点，代码中需显式注释说明。

### Go
- `sql.ErrTxDone` 过滤：三处 defer rollback 均加此过滤，避免正常 Commit 后的"假回滚失败"日志。✓
- `syncFieldRefs` 自带独立事务，在主事务 Commit 后调用，无嵌套事务问题。✓

## 配置变更

无。不涉及 JSON 配置文件修改。

## 测试策略

**单元测试**：无法用 miniredis/内存 MySQL 模拟 FOR SHARE 并发，不做并发测试。

**编译验证**：`go build ./...` 通过（类型安全的 Tx 方法签名可捕获所有调用错误）。

**代码审查**：阅读修改后的三个方法，逐行确认：
1. 事务在 HasRefsTx 之前开启
2. HasRefsTx 在事务内调用（不是 HasRefs）
3. SoftDeleteTx / UpdateTx 在事务内调用（不是非 Tx 版本）
4. Redis 缓存清除在 Commit 前
5. 内存缓存 Reload 在 Commit 后
6. defer rollback 是 error-aware 模式

**e2e 验证**（curl）：
- curl 测试删除被引用的扩展字段返回 `ErrExtSchemaRefDelete`（而非删除成功）
- curl 测试更新被引用字段的 type 返回 `ErrFieldRefChangeType`
- curl 测试收紧被引用扩展字段约束返回 `ErrExtSchemaRefTighten`
