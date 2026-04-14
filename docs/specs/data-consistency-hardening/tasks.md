# 数据一致性硬化 — Tasks

## 依赖图

```
T1 (LuaUnlock常量) ──┬── T3 (field/template cache签名)
                     └── T4 (event_type/fsm cache签名)
                              └── T8  service/field.go
                              └── T9  service/event_type.go
                              └── T10 service/template.go
                              └── T11 service/fsm_config.go
T2 (Is1062) ─────────┬── T5 (field/template store)
                     ├── T6 (event_type/schema store)
                     └── T7 (fsm_config store)
                              └── T8..T12 (service 层兜底)
T8-T12 ──────────────┬── T13 handler/template.go  (R3+R5)
                     └── T14 handler/fsm_config.go (R3+R5)
T1..T14 ─────────────── T15 unit tests
T1..T15 ─────────────── T16 docs
```

---

## 任务列表

### [x] T1：LuaUnlock 常量 `(R1)`

**文件**：`backend/internal/store/redis/config/common.go`

**改动**：
- 新增常量 `LuaUnlock`，值为原子解锁 Lua 脚本：
  ```go
  const LuaUnlock = `if redis.call('get',KEYS[1])==ARGV[1] then return redis.call('del',KEYS[1]) else return 0 end`
  ```

**做完是什么样**：`grep LuaUnlock backend/internal/store/redis/config/common.go` 有 1 匹配；`go build ./...` 通过。

---

### T2：`Is1062` helper `(R2)`

**文件**：`backend/internal/util/store.go`

**改动**：
- 在 `// ========== store 层 ==========` 分节下新增：
  ```go
  // Is1062 判断 err 是否为 MySQL duplicate entry (1062)
  func Is1062(err error) bool {
      var me *mysql.MySQLError
      return errors.As(err, &me) && me.Number == 1062
  }
  ```
- 文件头 import 补充 `github.com/go-sql-driver/mysql`

**做完是什么样**：`grep Is1062 backend/internal/util/store.go` 有匹配；`go vet ./...` 通过。

---

### T3：field / template Cache — TryLock/Unlock 改签名 `(R1)`

**文件**：
- `backend/internal/store/redis/field_cache.go`
- `backend/internal/store/redis/template_cache.go`

**改动**（两文件相同模式）：

```go
// TryLock: (bool, error) → (lockID string, error)
// 返回空串表示未获锁
func (c *FieldCache) TryLock(ctx context.Context, id int64, expire time.Duration) (string, error) {
    key := rcfg.FieldLockKey(id)
    lockID := fmt.Sprintf("%d-%d", id, time.Now().UnixNano())
    ok, err := c.rdb.SetNX(ctx, key, lockID, expire).Result()
    if err != nil {
        return "", fmt.Errorf("field try lock: %w", err)
    }
    if !ok {
        return "", nil
    }
    return lockID, nil
}

// Unlock: 新增 lockID 参数，走 Lua 脚本
func (c *FieldCache) Unlock(ctx context.Context, id int64, lockID string) {
    key := rcfg.FieldLockKey(id)
    if err := c.rdb.Eval(ctx, rcfg.LuaUnlock, []string{key}, lockID).Err(); err != nil {
        slog.Error("cache.字段释放锁失败", "error", err, "key", key)
    }
}
```

**做完是什么样**：`go build ./...` 通过；两文件 TryLock 返回值从 `(bool, error)` 变为 `(string, error)`；`grep 'rdb\.Del(ctx, key)' backend/internal/store/redis/` 无匹配（旧 DEL 消失）。

---

### T4：event_type / fsm_config Cache — TryLock/Unlock 改签名 `(R1)`

**文件**：
- `backend/internal/store/redis/event_type_cache.go`
- `backend/internal/store/redis/fsm_config_cache.go`

**改动**：与 T3 相同模式，key 前缀分别用 `rcfg.EventTypeLockKey` / `rcfg.FsmConfigLockKey`，slog key 文案对应改为 "事件类型" / "状态机"。

**做完是什么样**：`go build ./...` 通过；4 个 Cache 文件的 TryLock 签名全部一致。

---

### T5：field / template Store — Create 检测 1062 `(R2)`

**文件**：
- `backend/internal/store/mysql/field.go`
- `backend/internal/store/mysql/template.go`

**改动**（两文件 Create 方法同模式）：

```go
// field.go Create
result, err := s.db.ExecContext(ctx, `INSERT INTO fields ...`, ...)
if err != nil {
    if util.Is1062(err) {
        return 0, errcode.ErrDuplicate
    }
    return 0, fmt.Errorf("insert field: %w", err)
}
```

template 的 `CreateTx` 方法同理（对 `tx.ExecContext` 的错误也加 1062 检测）。

**做完是什么样**：`go build ./...` 通过；两个 Create 方法在 INSERT 出错时均检测 1062。

---

### T6：event_type / event_type_schema Store — Create 检测 1062 `(R2)`

**文件**：
- `backend/internal/store/mysql/event_type.go`
- `backend/internal/store/mysql/event_type_schema.go`

**改动**：与 T5 相同模式，`CreateTx` / `Create` 方法的 INSERT 错误路径加 1062 检测。

**做完是什么样**：`go build ./...` 通过。

---

### T7：fsm_config Store — Create 检测 1062 `(R2)`

**文件**：`backend/internal/store/mysql/fsm_config.go`

**改动**：`Create`（普通 Create 和 CreateInTx）INSERT 出错时加 1062 检测，返回 `errcode.ErrDuplicate`。

**做完是什么样**：`go build ./...` 通过。

---

### T8：`service/field.go` — R1 调用侧 + R2 兜底 + R3 缓存时序 + R5 `(R1+R2+R3+R5)`

**文件**：`backend/internal/service/field.go`

**改动**：

**R1 — `GetByID` TryLock 调用侧**：
```go
// Before
locked, lockErr := s.fieldCache.TryLock(ctx, id, 3*time.Second)
if lockErr == nil && locked { defer s.fieldCache.Unlock(ctx, id) }

// After
lockID, lockErr := s.fieldCache.TryLock(ctx, id, 3*time.Second)
if lockErr == nil && lockID != "" { defer s.fieldCache.Unlock(ctx, id, lockID) }
```

**R2 — `Create` ErrDuplicate 兜底**（在 `s.fieldStore.Create(ctx, req)` 返回后）：
```go
id, err := s.fieldStore.Create(ctx, req)
if err != nil {
    if errors.Is(err, errcode.ErrDuplicate) {
        return 0, errcode.Newf(errcode.ErrFieldNameExists, "字段标识 '%s' 已存在", req.Name)
    }
    slog.Error(...)
    return 0, fmt.Errorf("create field: %w", err)
}
```

**R3 — `Delete` 缓存移到 Commit 前**（`field.go:399` 附近的 tx 路径）：
```go
// Before
if err := tx.Commit(); err != nil { ... }
s.fieldCache.DelDetail(ctx, id)
s.fieldCache.InvalidateList(ctx)

// After
s.fieldCache.DelDetail(ctx, id)
for _, affectedID := range affectedIDs { s.fieldCache.DelDetail(ctx, affectedID) }
s.fieldCache.InvalidateList(ctx)
if err := tx.Commit(); err != nil { ... }
```

**R5 — `defer tx.Rollback()` 改为 error-aware**（2 处：`field.go:399`、`field.go:836`）：
```go
defer func() {
    if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
        slog.Warn("事务回滚失败", "error", rbErr)
    }
}()
```

**做完是什么样**：`go build ./...` 通过；`grep 'defer tx.Rollback()' backend/internal/service/field.go` 无匹配。

---

### T9：`service/event_type.go` — R1+R2+R3+R4+R5 `(全部)`

**文件**：`backend/internal/service/event_type.go`

**改动**：

**R1 — `GetByID` TryLock 调用侧**（与 T8 相同模式）。

**R2 — `Create` ErrDuplicate 兜底**（`s.store.CreateTx` 返回后）。

**R3 — 三处缓存时序调整**：
- `Create`（line ~225）：`InvalidateList` 移到 `tx.Commit()` 前
- `Update`（line ~341）：`DelDetail + InvalidateList` 移到 `tx.Commit()` 前
- `Delete`（line ~389）：`DelDetail + InvalidateList` 移到 `tx.Commit()` 前
- `ToggleEnabled` 如有 tx 同理

**R4 — `syncSchemaRefs` 加 tx 参数**：
```go
// 签名改为
func (s *EventTypeService) syncSchemaRefs(ctx context.Context, tx *sqlx.Tx, ...) error

// 内部
// Before: allSchemas, err := s.store.DB().QueryContext(ctx, ...)
// After:  allSchemas, err := tx.QueryContext(ctx, ...)
// 并紧跟:  if err != nil { ... continue }
//          defer allSchemas.Close()  ← 新增
```
调用方 `Update` 把 `tx` 传入：`s.syncSchemaRefs(ctx, tx, ...)`.

**R5 — 三处 `defer tx.Rollback()` 改 error-aware**（lines 212, 326, 374）。

**做完是什么样**：`grep 'store\.DB()' backend/internal/service/event_type.go` 在 syncSchemaRefs 函数体内 0 匹配；`grep 'defer tx.Rollback()' backend/internal/service/event_type.go` 无匹配；`go build ./...` 通过。

---

### T10：`service/template.go` — R1 调用侧 + R2 兜底 `(R1+R2)`

**文件**：`backend/internal/service/template.go`

**改动**：
- R1：`GetByID` 中 TryLock/Unlock 调用侧改签名（与 T8 相同模式）。
- R2：`CreateTx` 中 `s.store.CreateTx(ctx, tx, req)` 返回后加 `ErrDuplicate` 兜底 → `ErrTemplateNameExists`。

**做完是什么样**：`go build ./...` 通过。

---

### T11：`service/fsm_config.go` — R1 调用侧 + R2 兜底 `(R1+R2)`

**文件**：`backend/internal/service/fsm_config.go`

**改动**：
- R1：`GetByID` TryLock/Unlock 调用侧改签名（line ~309）。
- R2：`Create` 和 `CreateInTx` 的 `s.store.Create`/`s.store.CreateInTx` 返回后加 `ErrDuplicate` 兜底 → `ErrFsmConfigNameExists`。

**做完是什么样**：`go build ./...` 通过。

---

### T12：`service/event_type_schema.go` — R2 兜底 `(R2)`

**文件**：`backend/internal/service/event_type_schema.go`

**改动**：`Create` 中 `s.store.Create()` 返回后加 `ErrDuplicate` 兜底 → `ErrExtSchemaNameExists`。

**做完是什么样**：`go build ./...` 通过。

---

### T13：`handler/template.go` — R3 缓存时序 + R5 `(R3+R5)`

**文件**：`backend/internal/handler/template.go`

**改动**：

**R3**：3 个带 tx 的 handler 方法（Create/Update/Delete），将 `InvalidateList`/`InvalidateDetail`/`InvalidateDetails` 移到 `tx.Commit()` 之前：
```go
// Before
if err := tx.Commit(); err != nil { ... }
h.templateService.InvalidateList(ctx)
h.fieldService.InvalidateDetails(ctx, affected)

// After
h.templateService.InvalidateList(ctx)
h.fieldService.InvalidateDetails(ctx, affected)
if err := tx.Commit(); err != nil { ... }
```

**R5**：3 处 `defer tx.Rollback()` 改为 error-aware（lines 181, 327, 410）。

**做完是什么样**：`grep 'defer tx.Rollback()' backend/internal/handler/template.go` 无匹配；`go build ./...` 通过。

---

### T14：`handler/fsm_config.go` — R3 缓存时序 + R5 `(R3+R5)`

**文件**：`backend/internal/handler/fsm_config.go`

**改动**：

**R3**：3 个带 tx 的 handler（Create/Update/Delete），缓存清理移到 `tx.Commit()` 前（参见 lines 80-86, 165-172, 204-211）。

**R5**：3 处 `defer tx.Rollback()` 改为 error-aware（lines 65, 150, 191）。

**做完是什么样**：`grep 'defer tx.Rollback()' backend/internal/handler/fsm_config.go` 无匹配；`go build ./...` 通过。

---

### T15：单元测试 `(R6)`

**文件**：
- `backend/internal/store/redis/field_cache_test.go`（新建）
- `backend/internal/util/store_test.go`（新建）

**field_cache_test.go**（用 `github.com/alicebob/miniredis/v2`）：
- 场景：A 获锁 → 锁自然过期 → B 获相同 key 锁 → A 调 Unlock → B 的锁仍存在（`GET` 不为空）
- 验证 `TryLock` 返回非空 lockID；`Unlock` 传错 lockID 时 key 不被删除

**store_test.go**：
- `Is1062` 对 `*mysql.MySQLError{Number: 1062}` 返回 true
- `Is1062` 对 `*mysql.MySQLError{Number: 1045}` 返回 false
- `Is1062` 对 `fmt.Errorf("wrap: %w", &mysql.MySQLError{Number: 1062})` 返回 true（测试 errors.As 穿透）
- `Is1062` 对 `errors.New("other")` 返回 false

**做完是什么样**：`go test ./backend/internal/store/redis/... ./backend/internal/util/...` 全部 PASS。

---

### T16：文档更新 `(R7)`

**文件**：
- `docs/development/admin/dev-rules.md`
- `docs/development/admin/red-lines.md`（或 `docs/development/standards/red-lines/redis.md`）

**dev-rules.md 新增**（在缓存/Redis 分节）：
1. **缓存清除顺序**：有事务的写路径，`DelDetail`/`InvalidateList` 必须在 `tx.Commit()` 之前调用。Commit 失败时缓存已清无害（下次读走 DB）；Commit 成功后不清缓存则有脏读窗口。
2. **分布式锁使用规范**：`TryLock` 返回 `lockID`，`Unlock` 必须传入同一 `lockID`。Lua 脚本保证只删自己的锁，防止 TTL 超时后误删他人锁。
3. **事务内跨表查询纪律**：在已开启的事务路径中，所有 DB 查询必须走 `tx.QueryContext`/`tx.GetContext`，禁止调 `store.DB().QueryContext`（绕过事务隔离）。

**red-lines.md 新增**（在 Redis 或事务分节）：
- 禁止 `Commit 后清缓存`：有事务的写路径，缓存失效必须在 `tx.Commit()` 调用前完成。
- 禁止 `Unlock` 不传 lockID：分布式锁解锁必须携带 lockID，使用 Lua 脚本原子判断，禁止直接 `DEL`。

**做完是什么样**：两个文档各新增对应条目；`git diff --stat` 确认只改了这两个文档文件。

---

## 执行顺序

```
T1 → T3 → T4           (Redis 基础设施)
T2 → T5 → T6 → T7      (MySQL 基础设施)
T3+T4+T5+T6+T7 → T8 → T9 → T10 → T11 → T12  (Service 层)
T8-T12 → T13 → T14     (Handler 层)
T1-T14 → T15            (测试)
独立 → T16              (文档，可最后单独提交)
```

**建议分组提交**：
- Commit 1：T1+T2（基础设施，无业务影响）
- Commit 2：T3+T4+T5+T6+T7（Store 层，向下兼容）
- Commit 3：T8+T9+T10+T11+T12（Service 层）
- Commit 4：T13+T14（Handler 层）
- Commit 5：T15（测试）
- Commit 6：T16（文档）
