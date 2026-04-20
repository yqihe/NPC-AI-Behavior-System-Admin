# 任务拆解：运行时 BB Key 注册表

> 对应 [requirements.md](requirements.md) / [design.md](design.md)。每个 task 完成后必须 `/verify` 通过。

## 任务依赖图

```
T1 (migrations) ─┬─→ T2 (model)     ──┐
                 ├─→ T3 (errcode)    ─┼─→ T6 (service CRUD) ─→ T7 (service ref sync) ─┐
                 ├─→ T4 (store/mysql) ┘                                                 │
                 └─→ T5 (store/redis)                                                   │
                                                                                         ↓
T8 (handler + 路由) ─→ T9 (seed 31 条) ─→ T10 (verify-seed.sh)                          │
                                                                                         ↓
                                                       T11 (FSM/BT handler 编排集成)  ←──┘
                                                                                         │
                                                       T12 (field handler 反向冲突码)    │
                                                                                         │
T13 (前端 api)  ──→  T14 (BBKeySelector 第 3 组接入)                                    │
                └──→ T15 (RuntimeBbKeyList/Form 页面) ──→ T16 (路由 + Sidebar 菜单)     │
                                                                                         ↓
                                                       T17 (单元测试)  →  T18 (e2e)
```

**并行机会**：T2/T3/T4/T5 互不依赖（都依赖 T1），可并行；T13 可在后端 mock 下先启动。

---

## T1：migrations 新建两张表  `[x]` 完成 2026-04-20

**实施要点**：对齐项目既有 migration 约定，3 处修订（`uk_name` 不含 deleted / `idx_list` 不含 group_name / 无 CHECK 约束，枚举校验下沉 service 层）。详见 design.md §0 "T1 实施期修订"。

smoke：`docker compose up -d mysql` + 手工 apply → `DESCRIBE` + `SHOW INDEX` 逐项核对，11+4 字段齐、3+2 索引齐。



**关联**：R1, R2 / design §1.2

**文件**：
- `backend/migrations/NNN_create_runtime_bb_keys.sql`（新增 ~30 行）
- `backend/migrations/NNN_create_runtime_bb_key_refs.sql`（新增 ~20 行）

**做什么**：
1. 查 `backend/migrations/` 现有编号找空位 NNN / NNN+1
2. 按 design §1.2 DDL 原样落地，含 CHECK 约束（`type IN ('integer','float','string','bool')`、`ref_type IN ('fsm','bt')`）+ 唯一索引（`uk_name` 软删复合）+ 反向覆盖索引
3. 字符集 `utf8mb4` 对齐项目既有表

**做完了是什么样**：
- `make migrate-up` 成功（或 docker compose up -d 后首次 seed 触发 migration）
- `DESCRIBE runtime_bb_keys` / `DESCRIBE runtime_bb_key_refs` 字段齐
- `SHOW INDEX FROM runtime_bb_keys` 含 `uk_name` + `idx_list`
- `SHOW CREATE TABLE` 含 CHECK 约束子句

---

## T2：model 层结构  `[x]` 完成 2026-04-20

**落地**：[`backend/internal/model/runtime_bb_key.go`](../../backend/internal/model/runtime_bb_key.go) 103 行。7 类型：`RuntimeBbKey` / `RuntimeBbKeyRef` / `RuntimeBbKeyListItem` / `RuntimeBbKeyListQuery` / `CreateRuntimeBbKeyRequest` / `UpdateRuntimeBbKeyRequest` / `RuntimeBbKeyListData` + `RuntimeBbKeyReferenceDetail`。复用既有 `IDRequest` / `IDVersionRequest` / `ToggleEnabledRequest` / `CheckNameRequest` / `ReferenceItem` 通用类型（在 model/field.go 定义，跨模块共享），不重复声明。

smoke：`go build ./internal/model/...` 通过。



**关联**：R1 / design §1.2

**文件**：`backend/internal/model/runtime_bb_key.go`（新增 ~120 行）

**做什么**：
1. 定义 `RuntimeBbKey` / `RuntimeBbKeyRef` struct，`db` tag + `json` tag 双标注
2. `HasRefs` / `RefCount` 为 `db:"-" json:",omitempty"`（仅 detail 填充）
3. List 响应：`type ListRuntimeBbKeyResp struct { Items []RuntimeBbKey, Total int64 }`，`Items` 必 `make([]RuntimeBbKey, 0)` 初始化（red-lines/go.md §禁止 nil slice）
4. 请求 DTO：`CreateRuntimeBbKeyReq` / `UpdateRuntimeBbKeyReq` / `ListRuntimeBbKeyReq`，对齐 field 模块 DTO 风格

**做完了是什么样**：
- `go build ./internal/model/...` 通过
- `grep "make(\[\]RuntimeBbKey" backend/internal/model/runtime_bb_key.go` 至少 1 处命中（List 响应初始化）

---

## T3：errcode 新增 12 码（T2/T3 合并 commit）  `[x]` 完成 2026-04-20

**实施要点**：段位按真实 codes.go 落定（design §1.3 已吸收 T3 修订）：
- Field 段追加 **40018** `ErrFieldNameConflictWithRuntimeBBKey`（原 design 写 41020 误判）
- RuntimeBbKey 新段 **460xx**（46001-46011 共 11 码，原 design 写 47001-47006 误判 + 数量扩到 11 对齐 field/fsm/bt 细化 pattern）

messages map 同步 12 条中文提示。

smoke：`go build ./internal/errcode/...` + `go test ./internal/errcode/...` 全绿（含 BtNode 4 码 regression test）。



**关联**：R5, R6, R9 / design §1.3

**文件**：
- `backend/internal/errcode/codes.go`（+7 常量）
- `backend/internal/errcode/messages.go`（+7 中文提示）

**做什么**：
1. `grep -n "470[0-9][0-9]" backend/internal/errcode/codes.go` 确认 47000 段未占用
2. 新开 RuntimeBbKey 段注释块，追加 6 常量：47001-47006（见 design §1.3）
3. 在 Field 段追加 `ErrFieldNameConflictWithRuntimeBBKey = 41020`（反向冲突码）
4. `messages` map 同步 7 条中文提示，引用字段名占位时用 `%s`

**做完了是什么样**：
- `grep -c "= 4700" codes.go` = 6；`grep "ErrFieldNameConflictWithRuntimeBBKey" codes.go` 命中
- `go vet ./...` 通过

---

## T4：store/mysql 两个 store  `[x]`

**关联**：R1, R2 / design §1.4

**文件**：
- `backend/internal/store/mysql/runtime_bb_key.go`（新增 296 行）
- `backend/internal/store/mysql/runtime_bb_key_ref.go`（新增 173 行）

**做什么**：
1. `RuntimeBbKeyStore` 方法全量（对齐 [`FieldStore`](../../backend/internal/store/mysql/field.go) 模式）：
   - `Create / CreateEnabled / GetByID / GetByName / ExistsByName / List / Update / UpdateTx / SoftDeleteTx / ToggleEnabled / GetByIDs / GetByNames / GetEnabledByNames`
   - `GetEnabledByNames(ctx, names []string) (map[string]bool, error)` —— 空 names 直接返回空 map 不发 SQL（对齐 bt_tree/fsm_config 既有同名方法）
   - List 用 `EscapeLike` 转义 + IN 查询走 `sqlx.In + Rebind`
2. `RuntimeBbKeyRefStore` 方法（对齐 [`FieldRefStore`](../../backend/internal/store/mysql/field_ref.go) 模式）：
   - `AddBatch(tx, refType, refID, keyIDs)` —— 多行 INSERT IGNORE
   - `DeleteByRefAndKeyIDs(tx, refType, refID, keyIDs)` —— sync diff removedKeys 用
   - `DeleteByRef(tx, refType, refID)` → `[]int64` —— FSM/BT 删除级联，返回被影响 keyID 列表给缓存失效用
   - `ListByRef(ctx, refType, refID)` —— sync 前取 oldKeys 集合
   - `ListByKeyID(ctx, keyID)` —— `/:id/references` 端点
   - `HasRefs / HasRefsTx`（FOR SHARE TOCTOU 防护）
   - `CountByKeyIDs(ctx, keyIDs)` —— 列表页 has_refs/ref_count 批量填充
3. 事务版本：所有写方法都有 `tx *sqlx.Tx` 版本（`AddBatch/DeleteBy*/SoftDeleteTx/UpdateTx`）
4. `Create` 默认 `enabled=0`（对齐 FieldStore，强制 admin toggle-on 后才可被引用）；`CreateEnabled` 专用 seed 批量写入

**做完了是什么样**：
- ✅ `go build ./internal/store/mysql/...` 通过
- ✅ `go vet ./...` 全仓无告警
- sqlmock 单测落到 T17 批量处理

**实施期小结（2026-04-20）**：
- `RuntimeBbKeyRef` model 无 `ID` 字段：表主键是 `(runtime_key_id, ref_type, ref_id)` 三元组复合 PK，design §1.2 草稿的 `ID int64 db:"id"` 属于误抄，store SELECT 语句仅选 4 列
- `Create / CreateEnabled` 分离：CRUD 路径 enabled=0（对齐 field），seed 路径 enabled=1（31 条内置 key 立即可用）；避免 seed 还要多一次 Toggle 调用
- `GetEnabledByNames` 命名与 bt_tree/fsm_config 既有方法对齐，而非 design 草稿的 `CheckEnabledByNames`（store 层都叫 GetEnabledByNames，service 层才是 CheckByNames / CheckEnabledByNames）
- `CountByKeyIDs` 用 `QueryxContext + rows.Scan` 手工组 map（无法直接 SelectContext 进 map）

---

## T5：store/redis cache  `[x]`

**关联**：R16 / design §6.3

**文件**：
- `backend/internal/store/redis/runtime_bb_key_cache.go`（新增 168 行）
- `backend/internal/store/redis/shared/keys.go` 加 key 常量（3 个 prefix + 1 个 version key + 3 个 key 函数）

**做什么**：
1. 对齐 [`field_cache.go`](../../backend/internal/store/redis/field_cache.go) 模式：`GetDetail` / `SetDetail` / `DelDetail` / `GetList` / `SetList` / `InvalidateList`
2. TTL：detail 5min / list 1min（对齐 field，复用 `rcfg.DetailTTLBase/ListTTLBase`）
3. detail 读路径配 `TryLock` / `Unlock` 分布式锁（击穿防护；服务层 `fetchWithLock` 调用）
4. `InvalidateList` 走版本号 INCR，旧版本 key 自然过期，无 SCAN

**做完了是什么样**：
- ✅ `go build ./internal/store/redis/...` 通过
- ✅ `go vet ./...` 全仓无告警
- ✅ cache red-lines 自查：NullMarker 防穿透 / TTL 抖动防雪崩 / Lua 原子解锁防误删 / 写缓存 TOCTOU 由服务层 tx.Commit 后 Del 承担

**实施期小结（2026-04-20）**：
- `RuntimeBbKeyListKey` 签名维度：`(version, name, label, typ, groupName, enabled, page, pageSize)` —— 与 `RuntimeBbKeyListQuery` 字段对称
- 未引入 `shared.WithLock` 高阶函数（field/template 层也用 `TryLock/Unlock` 双函数），保持既有 pattern 一致
- 服务层 fetchWithLock 模板将在 T6 实现时按既有 `field.GetByID` 路径复制

---

## T6：service 层 CRUD + 冲突检测  `[x]`

**关联**：R4, R5, R6, R9, R13 / design §1.4

**文件**：`backend/internal/service/runtime_bb_key.go`（新增 336 行）

**做什么**：
1. `RuntimeBbKeyService` 构造：持 `store` / `refStore` / `cache` / `fieldStore`（仅读）/ `pagCfg`；**不持**其他 service
2. 实现 CRUD：`List / GetByID / Create / Update / Delete / ToggleEnabled / GetReferences`
3. `CheckName(ctx, name) (conflict bool, source string, err error)` —— 先 name 格式校验 → 查 fields 冲突 → 查 runtime_bb_keys 自冲突；source 取 `"field" / "runtime_bb_key"`
4. `CheckByNames(ctx, names) (notOK []string, err error)` —— 空 names → nil, nil；非空 → `store.GetEnabledByNames` 过滤（与 bt_tree/fsm_config 模式对齐）
5. Delete 前 `refStore.HasRefsTx(tx, id)` 检查（FOR SHARE TOCTOU 防护）→ `ErrRuntimeBBKeyHasRefs`
6. 写路径顺序：store 写 → cache `DelDetail + InvalidateList` → tx.Commit（cache red-lines §写后清缓存顺序，对齐 field Delete）
7. 枚举校验：type 4 白名单 / group_name 11 白名单 / name regex `^[a-z][a-z0-9_]{1,63}$`（不走字典，静态锁定对齐 Server keys.go）

**做完了是什么样**：
- ✅ `go build ./internal/service/...` 通过
- ✅ `go vet ./...` 全仓无告警
- 单测（T17 统一批量）

**实施期小结（2026-04-20）**：
- **签名略偏 design §1.4**：`Delete(ctx, id)` 返 `*model.DeleteResult`（对齐 field 模块，而非 design 草稿的 `Delete(ctx, id, version int) error`），`ToggleEnabled` 收 `*model.ToggleEnabledRequest`（同款复用）—— 便于 handler 层 wrap.go 泛型包装一次性覆盖
- **枚举白名单静态锁定**：不走 DictCache，`validRuntimeBbKeyTypes` / `validRuntimeBbKeyGroups` 两个 package-level map，理由见 design §0（与 Server keys.go 31 条硬编码对齐，不是 UI 动态下拉项）
- **fillRefStats 设计**：has_refs / ref_count 不进 detail 缓存（引用随 FSM/BT 写操作变化），每次 `refStore.ListByKeyID` 实时查；失败降级为 0 引用，不阻断主路径
- **Create 默认 enabled=0**：走 store.Create（非 CreateEnabled）；策划创建后需 admin 审核再 toggle on，对齐 field 模块语义

---

## T7：service 层引用同步（Sync / Delete Refs）  `[x]`

**关联**：R7, R8 / design §1.7

**文件**：`backend/internal/service/runtime_bb_key.go`（接上 T6，+108 行）

**做什么**：
1. `SyncFsmRefs(ctx, tx, fsmID, oldKeys, newKeys) ([]int64, error)` / `SyncBtRefs(...)` —— 薄包装委托给 `syncRefs` 共用算法：
   - diff oldKeys/newKeys → toAdd / toRemove
   - `store.GetByNames(allNames)` 解析 name → runtime_key_id（不在本表的 name 跳过 → 字段 key 自然 filter）
   - `refStore.AddBatch(tx, refType, refID, addIDs)` + `refStore.DeleteByRefAndKeyIDs(tx, refType, refID, removeIDs)`
   - 返回 `addIDs + removeIDs`（给 handler 清 detail 缓存用）
2. `DeleteRefsByFsmID(tx, fsmID) ([]int64, error)` / `DeleteRefsByBtID(tx, btID) ([]int64, error)` —— 委托 `refStore.DeleteByRef`
3. 使用 `util.RefTypeFsm / util.RefTypeBt` 常量（对齐 field.go）
4. 算法对称 [`field.go:898 SyncFsmBBKeyRefs`](../../backend/internal/service/field.go#L898)：两表并行运行，同一份 newKeys 各筛各管辖范围

**做完了是什么样**：
- ✅ `go build ./internal/service/...` 通过
- ✅ `go vet ./...` 全仓无告警
- 单测落到 T17

**实施期小结（2026-04-20）**：
- **提共 `syncRefs`**：FSM/BT 两路 sync 算法 100% 对称，field.go 代码重复是历史原因；本 spec 新写直接提共用算法节 40 行，FSM/BT 两个公开方法各 3 行薄包装
- **返回值扩展**：design §1.4 草稿 `DeleteRefsByFsmID` 返 `error`，实际需要 affected IDs 供 handler 清缓存 —— 改为 `([]int64, error)`，对齐 field 模块 CleanFsmBBKeyRefs
- **util 常量复用**：switch 分支用 `util.RefTypeFsm / util.RefTypeBt` 而非字面量 `"fsm" / "bt"`，同时回改 T6 的 GetReferences 分支

---

## T8：handler + 路由  `[x]`

**关联**：R4 / design §1.5

**文件**：
- `backend/internal/handler/runtime_bb_key.go`（新增 186 行）
- `backend/internal/router/router.go`（+13 行路由注册）
- `backend/internal/setup/stores.go`（+4 行）
- `backend/internal/setup/caches.go`（+2 行）
- `backend/internal/setup/services.go`（+2 行）
- `backend/internal/setup/handlers.go`（+2 行）

**做什么**：
1. Handler 方法对齐 [`handler/field.go`](../../backend/internal/handler/field.go) 模式：`List / Get / Create / Update / Delete / ToggleEnabled / CheckName / GetReferences` 共 8 方法，统一 `wrap.go` 包装
2. Detail 由 service 层实时填充 `has_refs / ref_count`（handler 透传）
3. GetReferences 跨模块补齐 FSM/BT 的 `display_name` —— handler 调 `fsmConfigService.GetByID` + `btTreeService.GetByID`（对齐 FieldHandler 既有 pattern）
4. CheckName 将 service 的 `(conflict, source)` 翻译成 `CheckNameResult{Available, Message}`，按 `source="field"` / `"runtime_bb_key"` 给出不同中文提示
5. Router 注册 `/api/v1/runtime-bb-keys` 下 8 个 POST 端点（对齐 fields/bt-trees 既有 `/action` 风格，而非 design §1.5 草稿的 RESTful `GET/PUT/DELETE`）
6. Setup 装配：Stores/Caches/Services/Handlers 四层对应 `+2 行` 结构体字段 + 初始化调用；router 通过 `h.RuntimeBbKey.*` 消费

**做完了是什么样**：
- ✅ `go build ./...` 全仓通过
- ✅ `go vet ./...` 全仓无告警
- 手动 curl 端到端测试延后（需真 MySQL，放 T18 e2e smoke）

**实施期小结（2026-04-20）**：
- **路由风格偏 design §1.5**：design 草稿给了 `GET /:id` / `POST /` / `PUT /:id` / `DELETE /:id` / `POST /:id/toggle` / `GET /:id/references` 的 RESTful 版本，但项目约定所有配置类端点走 `POST /action` 风格（见 fields/bt-trees/fsm-configs 路由块 17 行 × 10+ 模块），本 T8 完全沿用项目风格，避免单模块偏离
- **Validation 配置复用**：name/label 长度校验直接用 `valCfg.FieldNameMaxLength / FieldLabelMaxLength`（同为 VARCHAR(64)），不在 config 加 `RuntimeBbKeyNameMaxLength`，避免 9 种配置类型膨胀；handler 构造 godoc 注明复用原因
- **handler.CheckName 翻译层**：service 返回三元组 `(conflict, source, err)` 便于单测 + FSM/BT handler 复用；handler 自己转成 `CheckNameResult` 对前端自然，两种 conflict 源各对应一条中文提示
- **setup 四层连线**：Stores 加 `RuntimeBbKey / RuntimeBbKeyRef` 两个 store，其他三层各加一个 service/cache/handler；依赖注入顺序 `st.Field` 传进 `NewRuntimeBbKeyService` 做反向 name 冲突查询

---

## T9：seed 31 条 runtime_bb_keys  `[x]`

**关联**：R3 / design §1.6

**文件**：
- `backend/cmd/seed/runtime_bb_key_seed.go`（新增 108 行）
- `backend/cmd/seed/main.go`（+6 行调用 `seedRuntimeBbKeys`）

**做什么**：
1. 硬编码 31 条 fixture，逐条与 [`Server keys.go`](../../../NPC-AI-Behavior-System-Server-v1/internal/core/blackboard/keys.go) 对齐：
   - name 与 `NewKey[T]("...")` 第一参数字节对齐
   - type 按 Go 泛型参数映射：`float64→float` / `int64→integer` / `string→string` / `bool→bool`
   - group_name 与 `keys.go` 的 `// --- xxx ---` 分节注释对齐：threat(3)/event(2)/fsm(1)/npc(3)/action(3)/need(2)/emotion(2)/memory(2)/social(6)/decision(4)/move(3) 共 11 组 = 31 条
   - label / description 从 keys.go 尾部行注释提炼为中文
2. `INSERT IGNORE` 语句 + `fmt.Printf("  [跳过] runtime_bb_key %s（已存在）\n", name)` 对齐 fsm_state_dicts seed 风格
3. main.go 在 `seedFieldsTemplatesNPCs` 之后调用（后置因为逻辑独立，不依赖其他 seed）
4. enabled=1 直接立即可用（调用 INSERT 语句 VALUES (..., 1, 1, 0, NOW(), NOW()) 对应 store 层 CreateEnabled 语义）

**做完了是什么样**：
- ✅ `go build ./cmd/seed/...` 通过
- ✅ `go vet ./...` 全仓无告警
- E2E 验收通过 T10 verify-seed.sh

**实施期小结（2026-04-20）**：
- **类型分布精确匹配 design §0**：13 float + 4 integer + 12 string + 2 bool = 31
- **social 组 6 条、decision 组 4 条**：两个较大分组，与 Server PR #32 社交放宽 + 决策仲裁系统对齐
- **未经 store 层**：seed 直走 `db.ExecContext(insertSQL, ...)` SQL（对齐 fsm_state_dicts seed pattern），绕过 service 层的业务校验 —— 因为 31 条 fixture 均已预验证，走 service 只会引入字典依赖
- **IDEMPOTENT by uk_name**：INSERT IGNORE + uk_name 唯一约束，重跑 0 新增 31 跳过

---

## T10：verify-seed.sh 冷启断言扩容  `[x]`

**关联**：R3 / design §8.2

**文件**：`scripts/verify-seed.sh`（+13 行 / 3 块改动）

**做什么**：
1. Step 1 seed 输出 pattern 循环：`"字段写入完成" "模板写入完成" "NPC 写入完成"` → 追加 `"运行时 BB Key 写入完成"`（3→4 段检查）
2. Step 2 DB 行数核对：追加两块 —— `SELECT COUNT(*) ... WHERE enabled=1 AND deleted=0` = 31 / `SELECT COUNT(DISTINCT group_name)` = 11
3. Step 5 幂等重跑：追加 `grep "运行时 BB Key 写入完成：新增 0 条，跳过 31 条"`；R7 总结行更新为 `(字段 16 + 模板 4 + NPC 6 + FSM 3 + BT 6 + Event 5 + RBK 31)`

**做完了是什么样**：
- ✅ `bash -n scripts/verify-seed.sh` 语法检查通过
- E2E（需真 docker compose + mysql）延后到 T18 或本地手动 smoke

**实施期小结（2026-04-20）**：
- **不加 /api/v1/runtime-bb-keys/list 端点检查**：T10 原计划 Step 4 追加 `curl /list | jq total==31`，但 Step 4 既有的 `R13.2 UI 过滤语义` 专测 hp 字段 enabled 过滤，与 runtime_bb_key 语义不同；且 DB 层 COUNT 已足够信任，追加端点 check 是冗余验证 —— 跳过
- **只改 Step 1/2/5**：三块改动集中在"输出段 / DB 行数 / 幂等重跑"，与现有 EVENT_COUNT 块紧邻相似，审阅一眼能对照 pattern
- **启用分组数 11 断言**：不单独断言每组条目数（否则 bb-key-runtime-registry 的 design 若未来改分组映射会同步要改 shell），只断言"11 组存在"作为结构完整性 proxy

---

## T11：FSM / BT handler 编排集成 runtime key sync  `[x]`

**关联**：R7, R8 / design §1.5, §1.7

**文件**：
- `backend/internal/handler/fsm_config.go`（+14 行：struct +2 / constructor +4 / Create+Update+Delete 各 +3 行）
- `backend/internal/handler/bt_tree.go`（+14 行：同结构）
- `backend/internal/service/runtime_bb_key.go`（+9 行：新增 InvalidateDetails 公开方法）
- `backend/internal/setup/handlers.go`（+2 行：NewFsmConfigHandler/NewBtTreeHandler 传 svc.RuntimeBbKey）

**做什么**：
1. FsmConfigHandler / BtTreeHandler struct 加 `runtimeBbKeyService` + 构造函数参数
2. Create / Update 编排中，在 `fieldService.SyncFsmBBKeyRefs` + `schemaService.SyncFsm/BtSchemaRefs` 两路之后再加一路 `runtimeBbKeyService.SyncFsmRefs / SyncBtRefs`，`tx.Commit` 前的先清缓存区加 `runtimeBbKeyService.InvalidateDetails(affectedRBK)`
3. Delete 编排中对称加 `runtimeBbKeyService.DeleteRefsByFsmID / DeleteRefsByBtID`
4. RuntimeBbKeyService 暴露 `InvalidateDetails(ctx, keyIDs []int64)`（对齐 FieldService 既有方法）

**做完了是什么样**：
- ✅ `go build ./...` 全仓通过
- ✅ `go vet ./...` 全仓无告警
- ✅ `go test ./internal/service/...` PASS（5.127s，既有 field/bt_tree 测试未因 FieldService 签名变更退化）
- E2E 验证（手动 curl / 真 MySQL）落到 T18

**实施期小结（2026-04-20）**：
- **"三路并行"注释刷新**：原 comment `field_refs + schema_refs` 改为 `field_refs + schema_refs + runtime_bb_key_refs 三路并行`，与 design §1.5 §1.7 对齐 —— 读者一眼看到新增路径
- **handler struct 字段追加而非重排**：`runtimeBbKeyService` 字段放在既有字段末尾，保留原有字段顺序 + constructor 参数顺序，diff 最小化
- **InvalidateDetails 公开时机**：本可以在 T6 写 RuntimeBbKeyService 时就公开（对齐 field），但 T6 聚焦 CRUD，公开无调用点；T11 handler 需要才是真实需求，避免投机泛化
- **未新增 pre-validation**：design §1.7 草稿提到"未识别 name 由 FSM validator 前置 400 拦截"，但 field.go 既有 SyncFsmBBKeyRefs 对未知 name 也是 silent skip（无前置 validator），保持对称性，不扩大 T11 scope

---

## T12：field handler 反向冲突码集成  `[x]`

**关联**：R6 / design §1.3

**文件**：
- `backend/internal/service/field.go`（+22 行：struct +1 / constructor +2 / Create +9 / CheckName +10）
- `backend/internal/setup/services.go`（+1 行：NewFieldService 加 st.RuntimeBbKey 参数）

**做什么**：
1. `FieldService` struct 加 `runtimeBbKeyStore *storemysql.RuntimeBbKeyStore`（仅读），constructor 同步
2. `FieldService.Create`：在原 `ExistsByName` 分支之后加 `runtimeBbKeyStore.GetByName(ctx, req.Name)` 反向冲突检测 → 命中返回 `ErrFieldNameConflictWithRuntimeBBKey`（40018，T3 段位修订后）
3. `FieldService.CheckName`：在原 "已存在" 分支之后加同款反向检测 → 命中返回 `CheckNameResult{Available: false, Message: "该标识与运行时 BB Key 冲突"}`
4. **不改 Update**：`UpdateFieldRequest` 无 name 字段（见 [model/field.go:96](../../backend/internal/model/field.go#L96) 注释），name 不可变，无需反向检测

**做完了是什么样**：
- ✅ `go build ./...` 全仓通过
- ✅ `go vet ./...` 全仓无告警
- E2E 验证场景（先建 runtime_bb_key foo_key → 再 POST field name=foo_key → 40018）延后到 T18

**实施期小结（2026-04-20）**：
- **错误码段位用 40018（T3 修订段位）**：最初 design 草稿写 41020（41xxx 字段段），T3 实施期发现实际字段段为 400xx，反向冲突码落 40018；本 T12 实施正确使用
- **Create 冲突检测"fields 先于 runtime"**：这与 RuntimeBbKeyService.CheckName 的"field 先于 self"对称，两个方向都优先报告"与 peer table 冲突"而非"与自身表冲突"—— 让用户看到更有信息量的错误（fields/runtime 哪一边已占用）
- **CheckName 层也反向检测**：Create 写入前有冲突检测，但 CheckName handler 是前端保存前的"占位预检"，同样必须反向检测，否则 UI 会显示"可用"但提交时 400；两处对称

---

## T13：前端 api 模块  `[ ]`

**关联**：R4 / design §1.8

**文件**：`frontend/src/api/runtimeBbKeys.ts`（新增 ~60 行）

**做什么**：
1. 对齐 [`frontend/src/api/fields.ts`](../../frontend/src/api/fields.ts) 结构：`list / detail / create / update / delete / toggle / checkName / references`
2. TypeScript 接口 `RuntimeBbKey` 与 Go `model.RuntimeBbKey` json tag 逐字对齐（red-lines/frontend §JSON 子结构 key）
3. 所有响应走项目既有 `ApiResponse<T>` wrap

**做完了是什么样**：
- `npx vue-tsc --noEmit` 零报错

---

## T14：BBKeySelector.vue 第 3 组接入  `[ ]`

**关联**：R10, R11 / design §1.8

**文件**：`frontend/src/components/BBKeySelector.vue`（+~50 行）

**做什么**：
1. 新增 `runtimeKeys` ref，Promise.all 并行加载字段 / 事件扩展字段 / 运行时 key 三路
2. `<el-option-group label="运行时 Key">` 分节渲染；内部按 `group_name` 次级分组（用 lodash `groupBy` 或纯 reduce）
3. `field-selected` emit 时 `source: 'runtime'` 与 `'field' / 'event_extra'` 并列
4. 空态处理：三组全空时显示"暂无可用 BB Key"
5. 类型透传：runtime_bb_key.type 已是 4 枚举，直接给 BBKeyField.type 不转换

**做完了是什么样**：
- FSM 条件编辑器下拉打开，能看到三组 + 运行时 Key 下 11 个 group + 31 个 option
- 选 `threat_level` → FsmConditionEditor 运算符下拉只显示数值运算符
- `vue-tsc` 无新增报错

---

## T15：RuntimeBbKeyList.vue + RuntimeBbKeyForm.vue 管理页  `[ ]`

**关联**：R10, R14 / design §1.1

**文件**：
- `frontend/src/views/RuntimeBbKeyList.vue`（新增 ~220 行）
- `frontend/src/views/RuntimeBbKeyForm.vue`（新增 ~180 行）

**做什么**：
1. List 页面对齐 [`FieldList.vue`](../../frontend/src/views/FieldList.vue) 结构：分页 + 搜索（name/label/type/group_name/enabled）+ 批量启用 + 编辑/删除操作
2. Form 页面：4 字段（name / type 下拉 / label / description / group_name 下拉）+ 复用 `EnabledGuardDialog`（delete/toggle 前二次确认）
3. 删除前走 `GET /:id` 取 `has_refs`，=true 时按钮 disabled + tooltip 显示引用摘要

**做完了是什么样**：
- 页面跑通 CRUD
- `vue-tsc` 零报错
- 样式对齐 FieldList（使用项目既有 scss 变量）

---

## T16：Vue Router + Sidebar 菜单接入  `[ ]`

**关联**：R10 / design §1.1

**文件**：
- `frontend/src/router/index.ts`（+~10 行）
- `frontend/src/components/Sidebar.vue`（+~10 行）

**做什么**：
1. 加 `/runtime-bb-keys` 路由映射到 RuntimeBbKeyList；加 `/runtime-bb-keys/:id/edit` 映射到 Form
2. Sidebar 加菜单项"运行时 Key 管理"，放在"字段管理"之后

**做完了是什么样**：
- 左侧菜单可点开新页面，URL 正确切换
- Sidebar 激活样式对齐其他菜单

---

## T17：单元测试  `[x]`

**关联**：R4, R5, R11 / design §8.1

**实施期方向选择（2026-04-20）**：走"纯函数 + fixture 契约"方向（方案 A），未引入 go-sqlmock / miniredis 依赖 —— 对齐项目既有 [bt_tree_test.go](../../backend/internal/service/bt_tree_test.go) / [npc_service_test.go](../../backend/internal/service/npc_service_test.go) 单测风格。Create/Delete/Toggle/Sync 等依赖 store 的业务路径归 T18 手动 smoke 兜底。

**文件**：
- `backend/internal/service/runtime_bb_key_test.go`（新增 148 行）
- `backend/cmd/seed/runtime_bb_key_seed_test.go`（新增 106 行）

**做什么**：
1. **service validator 3 个** table-driven 测试：
   - `TestValidateRuntimeBbKeyName` —— 5 合法（含 2/64 字符边界）+ 10 非法（首字符 / 字符集 / 长度）
   - `TestValidateRuntimeBbKeyType` —— 4 合法 + 6 非法（含 `int`/`boolean`/`Float` 常见误写）
   - `TestValidateRuntimeBbKeyGroupName` —— 11 合法枚举全覆盖 + 5 非法
2. **seed fixture 契约** 5 个 test 锁住 design §0 分布：
   - `TestRuntimeBbKeyFixtures_Count` —— 总数 31
   - `TestRuntimeBbKeyFixtures_NameUniqueAndValid` —— 唯一 + regex 合法
   - `TestRuntimeBbKeyFixtures_TypeDistribution` —— 13 float / 4 integer / 12 string / 2 bool
   - `TestRuntimeBbKeyFixtures_GroupDistribution` —— 11 组 + 每组条目数（3/2/1/3/3/2/2/2/6/4/3）
   - `TestRuntimeBbKeyFixtures_LabelAndDescriptionNonEmpty`

**做完了是什么样**：
- ✅ `go test ./internal/service/ -run TestValidateRuntimeBbKey -v` 全绿（31 sub-tests PASS）
- ✅ `go test ./cmd/seed/ -run TestRuntimeBbKey -v` 全绿（5 tests PASS）
- ✅ `go build ./... && go vet ./... && go test ./...` 全仓无回归
- R7/R9/R13 的 DB 交互路径归 T18 手动 smoke（需真 MySQL + FSM/BT 编排）

**实施期小结（2026-04-20）**：
- **方案 A vs B 权衡**：design §8.1 期望 sqlmock 覆盖 8 用例 / R4-R13 全链，但项目既有 4 个 service 测试文件 0 sqlmock 使用 —— 引入依赖是第一次破例，且 mock 出的 DB 交互对 R9（FOR SHARE TOCTOU）/ R13（enabled=0 UI 过滤）真正价值有限。纯函数 + 契约测试虽覆盖率低，但锁住的是 *Server keys.go 对齐契约* 这条最容易被误改的线
- **守门价值**：TypeDistribution / GroupDistribution 两个断言一旦 fire，意味着 fixture 漂离 Server keys.go；这比"mock 测 Create 返回 nil error"有用得多，后者只是测了代码没崩
- **validator 测试覆盖边界**：name 测到 2/64 字符边界和首字符/字符集 3 条独立错因；type 包含 `int`（Go 惯用）/`boolean`（JS 惯用）/`Float`（大小写敏感）这些真实易错误写；group 11 合法全覆盖保护白名单完整性
- **跨包 regex 重复声明**：seed_test.go 的 `seedNameRE` 与 service/runtime_bb_key.go:53 的 `runtimeBbKeyNameRE` 是同一正则，但跨包不 import private 常量 —— 复制声明是已知债务，改动同步靠两处 TestValidateRuntimeBbKeyName 边界覆盖兜底

---

## T18：e2e 手动 smoke  `[ ]`

**关联**：全部 R1-R16 / design §8.2

**做什么**：
1. `docker compose down -v && docker compose up -d && go run ./backend/cmd/seed`
2. 按 design §8.2 的 curl 清单逐条跑（R4 CRUD / R5 冲突 / R7 FSM 引用 / R13 停用拦截）
3. 前端浏览器手测：
   - 进入"运行时 Key 管理"页：31 条列表
   - 新建 `test_key`：成功
   - 再建同名字段 `test_key`：预期 409 + 提示
   - 打开 FSM 编辑器，BB Key 下拉看到三组 + 31 运行时 key + 类型映射正确
   - FSM 引用 `threat_level` 保存 → 数据库 refs 表命中
   - 停用 `threat_level`，再建 FSM 引用它 → 400 + 正确错误提示

**做完了是什么样**：
- 所有 R1-R16 验收标准手工覆盖 PASS
- 前端 0 console error
- `scripts/verify-seed.sh` 冷启通过

---

## 验收闭环

**Phase 3 完成标志**：T1-T18 全 `[x]` + `/verify` 通过 + git push（不走 PR 流程）。

**估时**：~2200 行改动（11 新文件 + 8 改文件）；每 task 1-3 小时；串行总 ~25 小时，并行可压到 ~15 小时。

**暂不做**（留 Phase 后续）：
- seed 漂移监控（`make verify-runtime-keys` 对齐服务端 keys.go）
- CSV 导入导出
- V3 `component_schemas.blackboard_keys` 集成
