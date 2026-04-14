# 数据一致性硬化 — Design

## 方案描述

### R1：分布式锁带 lockID + Lua 解锁

**问题**：`TryLock` 存入固定值 `"1"`，`Unlock` 直接 `DEL`。锁超时后另一持有者 B 已获锁，A 的 Unlock 仍能删掉 B 的锁。

**方案**：

1. `TryLock` 签名改为 `(ctx, id, expire) → (lockID string, err error)`。
   - `lockID = ""` 表示未获锁（SetNX 失败）
   - `lockID != ""` 表示获锁成功，value 即 lockID
   - lockID 生成：`fmt.Sprintf("%d-%d", id, time.Now().UnixNano())` — 无外部依赖，足够唯一

2. `Unlock` 签名改为 `(ctx, id, lockID string)`，用 Lua 脚本原子解锁：
   ```lua
   if redis.call('get', KEYS[1]) == ARGV[1] then
     return redis.call('del', KEYS[1])
   else
     return 0
   end
   ```
   脚本作为常量 `LuaUnlock` 放到 `store/redis/config/common.go`。

3. 调用侧改动（以 field service 为例）：
   ```go
   // Before
   locked, lockErr := s.fieldCache.TryLock(ctx, id, 3*time.Second)
   if lockErr == nil && locked {
       defer s.fieldCache.Unlock(ctx, id)
   }

   // After
   lockID, lockErr := s.fieldCache.TryLock(ctx, id, 3*time.Second)
   if lockErr == nil && lockID != "" {
       defer s.fieldCache.Unlock(ctx, id, lockID)
   }
   ```

涉及文件：`store/redis/{field,template,event_type,fsm_config}_cache.go`（4 个）+ `store/redis/config/common.go` + `service/{field,template,event_type,fsm_config}.go`（4 个调用侧）。

---

### R2：Create 1062 安全网

**问题**：`Create` 先调 `ExistsByName`（check），再 INSERT（act）。并发两个相同 name 请求同时通过 check → 都执行 INSERT → 一个撞 MySQL 1062 唯一键 → 返回 500 而非业务错误码。

**方案**：

1. `util/store.go` 新增 `Is1062(err error) bool` helper：
   ```go
   func Is1062(err error) bool {
       var me *mysql.MySQLError
       return errors.As(err, &me) && me.Number == 1062
   }
   ```
   依赖 `github.com/go-sql-driver/mysql`（已在 go.mod 中作为间接依赖，需显式引入）。

2. 各模块 store 的 `Create` 方法在 INSERT 出错时检测 1062，返回 `errcode.ErrDuplicate`：
   ```go
   if err != nil {
       if util.Is1062(err) {
           return 0, errcode.ErrDuplicate
       }
       return 0, fmt.Errorf("insert xxx: %w", err)
   }
   ```

3. service 的 `Create` 在调用 `store.Create` 之后追加兜底：
   ```go
   id, err := s.xxxStore.Create(ctx, req)
   if err != nil {
       if errors.Is(err, errcode.ErrDuplicate) {
           return 0, errcode.Newf(errcode.ErrXxxNameExists, "标识 '%s' 已存在", req.Name)
       }
       slog.Error("...", "error", err)
       return 0, fmt.Errorf("create xxx: %w", err)
   }
   ```
   原 `ExistsByName` 预检保留（提前返回友好错误），1062 只是安全网。

涉及文件：`util/store.go` + `store/mysql/{field,template,event_type,fsm_config,event_type_schema}.go`（5 个）+ `service/{field,template,event_type,fsm_config,event_type_schema}.go`（5 个）。

---

### R3：缓存失效移到 Commit 之前

**问题**：当前 `Delete/Update(带tx)` 路径先 Commit 再清缓存，存在 1ms 窗口并发读命中旧缓存。

**方案**：对有事务的写路径，将 `DelDetail` + `InvalidateList` 移到 `tx.Commit()` 调用之前：

```go
// Before
if err := tx.Commit(); err != nil { ... }
s.cache.DelDetail(ctx, id)
s.cache.InvalidateList(ctx)

// After
s.cache.DelDetail(ctx, id)      // ← 先清缓存
s.cache.InvalidateList(ctx)
if err := tx.Commit(); err != nil {
    // Commit 失败时缓存已清，下次读走 DB，DB 未变，数据仍一致
    return fmt.Errorf("commit: %w", err)
}
```

无事务的写路径（如 `field.Update` 直接乐观锁 UPDATE）：DB 写是原子操作，清缓存在写成功之后，顺序不变。

涉及文件：`service/{event_type,field,fsm_config,template}.go`（4 个，含 tx 的 Delete/ToggleEnabled/Update 路径）。

---

### R4：syncSchemaRefs 改走事务内查询

**问题**：`syncSchemaRefs` 在事务外调用 `s.store.DB().QueryContext(...)` 查 schema ID，并发禁用 Schema 时可能查不到 → 跳过 Remove → `schema_refs` 残留脏数据。

**方案**：
1. `syncSchemaRefs` 签名加 `tx *sqlx.Tx` 参数。
2. 内部 `s.store.DB().QueryContext(...)` 改为 `tx.QueryContext(...)`（读到事务一致快照）。
3. 同时加 `defer rows.Close()` 防行游标泄漏：
   ```go
   rows, err := tx.QueryContext(ctx, `SELECT id FROM event_type_schema WHERE field_name = ? AND deleted = 0`, key)
   if err != nil { ... continue }
   defer rows.Close()   // ← 新增
   ```
4. 调用方 `Update` 将 `tx` 传入：`s.syncSchemaRefs(ctx, tx, ...)`.

涉及文件：`service/event_type.go`（1 个）。

---

### R5：defer Rollback 改为 error-aware

**问题**：`defer tx.Rollback()` 无条件执行，Commit 成功后触发 Rollback 返回 `sql.ErrTxDone`，虽被静默吞掉，但如有日志也是噪音。

**方案**：统一改为：
```go
defer func() {
    if rbErr := tx.Rollback(); rbErr != nil && !errors.Is(rbErr, sql.ErrTxDone) {
        slog.Warn("事务回滚失败", "error", rbErr)
    }
}()
```
- Commit 成功时：Rollback 返回 `sql.ErrTxDone`，被 `errors.Is` 过滤，不记日志。
- Commit 失败/未到达时：Rollback 正常执行，失败则记 Warn 日志。
- 不需要命名返回值，最小侵入。

涉及 11 处：`service/event_type.go`（3）、`service/field.go`（2）、`handler/fsm_config.go`（3）、`handler/template.go`（3）。

---

## 方案对比

### R1 备选：引入 redsync 库

redsync 提供完整的 Redlock 算法（多节点锁），适合多 Redis 实例场景。

**不选原因**：
- 本项目单 Redis 实例，Redlock 的跨实例共识无意义
- 引入新依赖（`github.com/go-redsync/redsync`），增加维护成本
- Lua 脚本 6 行解决问题，无需过度工程化

### R2 备选：改 Create 为事务内 SELECT FOR UPDATE + INSERT

在事务内先 `SELECT FOR UPDATE WHERE name=?`，不存在再 INSERT，彻底消灭竞态。

**不选原因**：
- `FOR UPDATE` 在 name 不存在时锁行不存在，MySQL 会对 gap 加 next-key lock，影响并发
- 保留 `ExistsByName` 预检本身就是用户友好设计，加 1062 兜底已足够

### R3 备选：先清缓存再开事务（双删）

先清缓存 → 执行业务事务 → Commit → 延迟再清一次缓存（双删防止并发读回填脏数据）。

**不选原因**：
- 引入延迟时间硬编码
- 本项目 QPS 不在需要双删的量级
- 移到 Commit 前已消灭主要窗口，权衡合理

---

## 红线检查

| 红线文档 | 涉及条目 | 结论 |
|----------|---------|------|
| `redis.md` | "禁止 DEL/Unlock 不检查 error" | ✅ R1 Lua 方案保留错误检查 |
| `redis.md` | "Unlock 用 DEL，高并发下需 Lua" | ✅ R1 正是解决此问题 |
| `cache.md` | "禁止写操作成功后不清缓存" | ✅ R3 确保清缓存，仅调整时序 |
| `go.md` | "禁止 500 响应暴露 Go error 原文" | ✅ R2 1062 翻译为业务码，不透传 |
| `go.md` | "禁止 context.Background() 直接调用 DB" | ✅ R4 tx 继承 request context |
| `admin/red-lines.md` §10.3 | "service 缓存读取 err==nil && hit，禁止丢弃 error" | ✅ 不涉及 |
| `admin/red-lines.md` §11.2 | "跨 store 共享工具 → util/" | ✅ R2 `Is1062` 放 `util/store.go` |
| `admin/red-lines.md` §4.1 | "错误码数字 → errcode/codes.go 常量" | ✅ 使用现有常量，不硬编码 |

无红线违反。

---

## 扩展性影响

**正面影响扩展轴 1（新增配置类型）**：
- `util/store.go: Is1062` 新模块直接调用，无需重复实现
- `LuaUnlock` 脚本作常量共享，新 Cache 结构体直接用
- Rollback 语义修正形成代码模板，新模块复制即正确

**不影响扩展轴 2（新增表单字段）**：纯后端改动。

---

## 依赖方向

```
handler
  └── service
        ├── store/mysql   (CRUD)
        ├── store/redis   (Cache — TryLock/Unlock/SetDetail/DelDetail)
        │     └── store/redis/config  (key 生成 + LuaUnlock 常量)
        └── util          (Is1062, EscapeLike, NormalizePagination …)
              └── (仅标准库 + go-sql-driver/mysql)
errcode  (被 service/store/handler 引用，无向上依赖)
```

改动不引入新的依赖方向，`util/store.go` 新增 `Is1062` 依赖 `go-sql-driver/mysql`（已是间接依赖，显式化）。

---

## 陷阱检查（dev-rules）

参考 `docs/development/standards/dev-rules/redis.md`、`go.md`、`cache.md`：

| 陷阱 | 本方案处置 |
|------|---------|
| SetNX 锁必须设 expire | TryLock 调用保留 expire 参数，无变化 |
| Lua 脚本键和参数分离 | `KEYS[1]` 传 key，`ARGV[1]` 传 lockID，符合 Redis cluster 要求 |
| `errors.As` vs `errors.Is` 解 MySQL 错误 | 使用 `errors.As(err, &me *mysql.MySQLError)` 穿透 fmt.Errorf wrap |
| `defer rows.Close()` 应在 err 检查之后 | R4 先判 err，再 `defer rows.Close()`，符合规范 |
| `sql.ErrTxDone` 判断需 `errors.Is` | R5 用 `errors.Is(rbErr, sql.ErrTxDone)` 正确判断 |
| 清缓存失败不应阻断业务 | Del/Invalidate 失败只记日志，不 return error，保持现有行为 |

---

## 配置变更

无新增配置项。`LuaUnlock` 是代码常量，不在 config.yaml。

---

## 测试策略

**单元测试（新增）**：
- `store/redis/field_cache_test.go`：用 `miniredis` 测试 Lua 解锁场景（A 超时后 B 持锁，A 调 Unlock，B 的锁不受影响）
- `util/store_test.go`：测试 `Is1062` 正确识别 MySQL 1062 错误和非 1062 错误

**现有测试回归**：
- `go test ./...`（含 `service/constraint_check_test.go`）
- `npx vue-tsc --noEmit`（前端无改动，确认无回归）

**集成测试（升级）**：
- `tests/test_10_attack.sh` 并发 Create 同 name 场景：两请求返回 1 个 success + 1 个业务错误码（不含 500）
