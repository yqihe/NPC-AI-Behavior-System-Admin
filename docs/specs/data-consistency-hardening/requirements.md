# 数据一致性硬化 — Requirements

## 动机

企业级代码审查发现 5 项**并发与事务安全风险**。这些缺陷在单请求顺序场景下完全正常，但在以下生产场景会暴露：

| 缺陷 | 触发场景 | 暴露形式 |
|---|---|---|
| 分布式锁用 Del 解锁 | A 业务超时+B 获得同 key 锁 | B 的临界区锁被 A 误删，多线程同时进入临界区 |
| 唯一性 check-then-act | 两个并发 Create 同 name | 双双通过 ExistsByName 检查，撞 MySQL 唯一键 → 500 而非业务错误 |
| Commit 后清缓存 | 并发读/写同一行 | 读命中已删除/旧值的缓存，数据与 DB 不一致 |
| TOCTOU 事务外查询 | Schema 被并发禁用 | schema_refs 残留脏数据（查不到 id 跳过 Remove） |
| defer 无条件 Rollback | 所有 tx 写路径 | Commit 成功后仍触发 Rollback 调用（虽容错但语义错 + 日志噪音） |

**为什么打包一个 spec**：5 项同属"并发/事务边界"主题，修复动作有耦合（例如 Delete 流程同时涉及缓存顺序 #4 和 Rollback 语义 #6；锁抽取 #2 与唯一性修复 #3 共享 `errcode.Is1062` 等工具）。分散改比合并改多走 5 次 spec 流程开销、且中间态存在不完整性（修了锁没修顺序）。

不做会怎样：
- 上线后低频并发即可产生数据不一致（缓存脏、引用残留），debug 成本极高（难复现）
- 唯一性撞键返回 500 让前端/测试脚本无法分支处理，产生假失败
- 锁误删是"静默"bug，不会 panic，但会导致多客户端进入应做互斥的临界区

## 优先级

**高**。5 项中 4 项 CRITICAL、1 项 HIGH。在后续 NPC 模块开发和集成测试前必须修复——否则：
- 集成测试的并发攻击测试（`test_10_attack.sh`）会间歇性红
- 新模块继续复制现有锁/缓存模式，问题横向扩散
- 上线后暴露成本是开发期修复成本的 10 倍以上

## 预期效果

### 场景 A：并发创建同 name
- **Before**：两请求同时调 CheckName → 都返回 available → 都调 Create → 一个成功、一个撞唯一键返回 500 暴露 `duplicate entry` 原始错误
- **After**：两请求同时调 Create → 一个成功、一个返回 `code: ErrXxxNameExists, message: "XX 标识已存在"`

### 场景 B：缓存脏读
- **Before**：A 调 Delete → Commit → 清缓存（1ms 窗口期）；B 在窗口期内读 → 命中旧缓存 → 返回已删除记录
- **After**：A 调 Delete → **先清缓存** → Commit → B 读 → 走 DB 查不到 → 返回 404

### 场景 C：锁超时误删
- **Before**：A 持 FieldLock:42 (TTL 5s)，阻塞 10s 后恢复 → A 调 Unlock → 但此时 B 已持同 key → B 的锁被删掉
- **After**：A 调 Unlock 时带自己的 lockID；Lua 脚本发现 key 的 value ≠ A 的 lockID → 拒绝删除

### 场景 D：引用残留
- **Before**：EventType 更新时，某个 old ext key 对应的 Schema 已被并发禁用；syncSchemaRefs 事务外查该 key → 查不到 id → 跳过 Remove → schema_refs 残留
- **After**：查询走 tx（读到事务一致快照）→ 查到 id → Remove 成功

### 场景 E：Rollback 语义
- **Before**：所有 tx 路径 `defer tx.Rollback()` + 后续 `tx.Commit()`，Commit 成功后 defer 触发 Rollback 返回 `sql: transaction has already been committed` 错误（容错吞掉但日志产生噪音）
- **After**：仅在 err != nil 时 Rollback；Commit 成功时 defer 不做事

## 依赖分析

**依赖**（已完成）：
- PR #22 util 分层重组（util/store.go 已建立，锁工具适合放这里）
- PR #22 CheckConstraintTightened 下沉（后续可能跟进把其他共享规则也下沉）
- errcode 包（用于添加 errcode.Is1062 辅助或直接用 mysql.MySQLError 类型断言）

**谁依赖**：
- 集成测试 `test_10_attack.sh` 的并发场景断言会从"接受 500"改为"必须 409/业务码"
- 新模块（BT / 区域 / NPC）直接沿用本次建立的锁/缓存/事务规范
- `docs/development/admin/dev-rules.md` 的缓存读写模式条目会升级

## 改动范围

**Backend**：
- `store/redis/{event_type,field,template,fsm_config}_cache.go` — 4 个文件的 TryLock/Unlock 改签名（带 lockID）
- `util/store.go` 或新增 `util/redis_lock.go` 分节 — 统一锁工具 + Lua 脚本
- `store/mysql/{field,event_type,event_type_schema,template,fsm_config}.go` — 5 个 store 的 Create 错误处理（1062 识别）
- `errcode/error.go` 或 `errcode/mysql.go` — 新增 `Is1062(err)` 辅助（可选）
- `service/*.go` × 5 — Create/Update/Delete/ToggleEnabled 流程：
  - 唯一性改事务内 INSERT + 1062 捕获
  - 缓存清除顺序调整（Commit 前清）
  - Rollback 语义修正
  - TryLock/Unlock 调用点改签名
- `service/event_type.go` 的 syncSchemaRefs — 改走 tx
- `handler/{template,fsm_config}.go` — Rollback 语义修正（handler 层的 defer）

**Docs**：
- `docs/development/admin/dev-rules.md` — 缓存写-清顺序、锁语义、tx 查询纪律
- `docs/development/admin/red-lines.md` — 新增相关禁令（如"禁止 Commit 后清缓存"）

**测试**：
- `backend/internal/store/redis/redis_lock_test.go` 新建（锁误删单测，用 miniredis 或 fakeredis）
- `tests/test_10_attack.sh` 现有并发攻击断言升级

**预估文件**：~20 文件（backend 15 + docs 2 + 测试 3）

## 扩展轴检查

**正面影响扩展轴 1（新增配置类型）**：
- 统一锁工具抽到 util 后，新模块直接调用，无需每次实现 TryLock/Unlock
- 1062 错误处理抽到 store 层基础设施后，新模块 Create 不用重复写 if errors.Is(err, dupKeyErr)
- 缓存清除顺序规范化成 dev-rules 权威模式，新模块按模板写即可

**不影响扩展轴 2（新增表单字段）**：本次后端改动，不动前端。

## 验收标准

- **R1**：分布式锁带 value 签名，解锁走 Lua 脚本（`if get == value then del`）。`miniredis` 或集成测试验证场景 C（A 超时后 B 持锁，A 调 Unlock 不影响 B）。
- **R2**：5 个模块的 Create 路径：INSERT 在事务内，捕获 MySQL 1062 错误转 `ErrXxxNameExists` 业务错误码。`tests/test_10_attack.sh` 并发 Create 同 name 场景：两请求返回合计 1 个 success + 1 个业务错误码（不含 500）。
- **R3**：5 个模块的 Delete/Update/ToggleEnabled：缓存失效调用（`DelDetail` + `InvalidateList`）发生在 `tx.Commit()` **之前**。grep 扫描 service 层无"Commit 后清缓存"模式。
- **R4**：`service/event_type.go` 的 `syncSchemaRefs` 查询改走 tx，`store.DB()` 调用从该函数消失。`grep -n "store.DB()" backend/internal/service/event_type.go` 在 syncSchemaRefs 函数内 0 匹配。
- **R5**：所有 `defer tx.Rollback()` 改为 error-aware 模式（`defer func(){ if err != nil { tx.Rollback() } }()` 或等价）。Commit 成功后无 Rollback 调用日志噪音。grep `defer tx.Rollback()` 数量从当前 11 处降为 0。
- **R6**：`go build ./...` + `go vet ./...` + `go test ./...` 全部通过（含 T8/T9 单测 + 新增锁单测）。`npx vue-tsc --noEmit` 通过（前端无改动回归确认）。
- **R7**：`docs/development/admin/dev-rules.md` 和 `red-lines.md` 新增"缓存写-清顺序"+ "分布式锁使用规范"+ "事务内跨表查询"三条权威模式/禁令。
- **R8**：现有集成测试全部通过（含 5 模块 e2e）。`test_10_attack.sh` 升级后的并发断言通过。

## 不做什么

- **不引入外部分布式锁库**（如 redsync）——简单 Lua 脚本足够，避免依赖膨胀
- **不改 Redis key 命名**——保留现有 `rcfg.XxxLockKey(id)`
- **不统一 TryLock/Unlock 为自由函数**——保留各 cache 结构体方法形式，只让内部调用共享 util 函数（与 T5 handler 改造风格一致）
- **不新增审计日志字段**——审查 punch list 中的日志规范项（HIGH #7-#8）走独立 spec
- **不改 handler wrap.go**（HIGH #7/#8 范围）——留给 🅱 组 spec
- **不动前端**
- **不补齐 EventType.Delete 引用检查**（HIGH #9 另议）
- **不重写 List 缓存 Cache-Aside 共享模板**（MEDIUM #16 另议）
- **不改 TTL jitter / 空列表缓存**（MEDIUM #20/#21 另议）
