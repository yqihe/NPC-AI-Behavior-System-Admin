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

## T9：seed 31 条 runtime_bb_keys  `[ ]`

**关联**：R3 / design §1.6

**文件**：
- `backend/cmd/seed/runtime_bb_key_seed.go`（新增 ~150 行）
- `backend/cmd/seed/main.go`（+1 行调用 `seedRuntimeBbKeys`）

**做什么**：
1. 硬编码 31 条 fixture，逐条与 [`Server keys.go`](../../../NPC-AI-Behavior-System-Server-v1/internal/core/blackboard/keys.go) 对齐：
   - name 与 `NewKey[T]("...")` 第一参数字节对齐
   - type 按 Go 泛型参数映射：`float64→float` / `int64→integer` / `string→string` / `bool→bool`
   - group_name 与 `keys.go` 的 `// --- xxx ---` 分节注释对齐：threat/event/fsm/npc/action/need/emotion/memory/social/decision/move 共 11 组
   - label / description 从 keys.go 注释提炼
2. `INSERT IGNORE` 语句 + `fmt.Printf("  [跳过] runtime_bb_key %s（已存在）\n", name)` 对齐其他 seed
3. main.go 在 `seedFields` 之后调用

**做完了是什么样**：
- `docker compose down -v && docker compose up -d && go run ./backend/cmd/seed` 成功
- `SELECT COUNT(*) FROM runtime_bb_keys WHERE deleted=0` = 31
- `SELECT COUNT(DISTINCT group_name) FROM runtime_bb_keys` = 11
- `SELECT name, type, group_name FROM runtime_bb_keys ORDER BY id` 逐条匹配 keys.go

---

## T10：verify-seed.sh 冷启断言扩容  `[ ]`

**关联**：R3 / design §8.2

**文件**：`scripts/verify-seed.sh`（+~15 行）

**做什么**：
1. 在 Step 1（seed 首跑）后新增块：`RUNTIME_KEY_COUNT=$(mysql -e "...") ; [ "$RUNTIME_KEY_COUNT" = "31" ] || exit 1`
2. 在 Step 4（API export 检查）新增：`curl /api/v1/runtime-bb-keys/list -d '{"page":1,"page_size":100}' | jq '.data.total == 31'`
3. Step 5 幂等重跑断言追加一条：`grep "运行时 Key 写入完成：新增 0 条，跳过 31 条"`
4. R7 输出行更新为 `(字段 16 + 模板 4 + NPC 6 + FSM 3 + BT 6 + Event 5 + RuntimeKey 31)`

**做完了是什么样**：
- `bash scripts/verify-seed.sh` 冷启 + 重跑双绿

---

## T11：FSM / BT handler 编排集成 runtime key sync  `[ ]`

**关联**：R7, R8 / design §1.5, §1.7

**文件**：
- `backend/internal/handler/fsm_config.go`（+~15 行）
- `backend/internal/handler/bt_tree.go`（+~15 行）

**做什么**：
1. Create / Update 编排中，在既有 `fieldService.SyncFsmBBKeyRefs` 调用**之后**、`tx.Commit()` **之前**，新增 `runtimeBbKeyService.SyncFsmRefs(ctx, tx, fsmID, oldKeys, newKeys)`
2. Delete 编排中，在既有 `fieldService.DeleteFieldRefsByFsmID` 之后新增 `runtimeBbKeyService.DeleteRefsByFsmID(ctx, tx, fsmID)`
3. Commit 后清缓存：原字段缓存清 + 新增 `runtimeBbKeyCache.DelDetail(affectedKeyIDs...)`
4. BT 对称

**做完了是什么样**：
- 手动创建 FSM 含 `condition.key="threat_level"` → `SELECT * FROM runtime_bb_key_refs` 命中 1 行
- 更新 FSM 把 `threat_level` 改为 `max_hp`（字段）→ runtime_bb_key_refs 减 1 行，field_refs 增 1 行
- 删除 FSM → runtime_bb_key_refs 相关行消失

---

## T12：field handler 反向冲突码集成  `[ ]`

**关联**：R6 / design §1.3

**文件**：`backend/internal/service/field.go`（+~10 行）

**做什么**：
1. `FieldService.Create` / `Update` 的 name 校验链路加一步：`runtimeBbKeyStore.GetByName(ctx, name)` 命中则返回 `ErrFieldNameConflictWithRuntimeBBKey`（41020）
2. **FieldService 新持 `runtimeBbKeyStore`**（peer store，非 peer service），`NewFieldService` 签名加一参数；setup 层同步

**做完了是什么样**：
- 先建 runtime_bb_key `foo_key` → 再 POST field `{"name":"foo_key",...}` → 409 + code 41020

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

## T17：单元测试  `[ ]`

**关联**：R4, R5, R7, R9, R11, R13

**文件**：`backend/internal/service/runtime_bb_key_test.go`（新增 ~200 行）

**做什么**：
1. 用 sqlmock 覆盖 8 个用例（见 design §8.1 表）
2. mock 策略：store 用 sqlmock 构造 rows + expect；cache 用 miniredis 或接口 mock
3. 断言覆盖验收标准 R4/R5/R7/R9/R11/R13

**做完了是什么样**：
- `go test ./internal/service/ -run TestRuntimeBbKey -v` 全绿
- `go test -cover` 新模块覆盖率 ≥70%（对齐 field 模块现状）

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
