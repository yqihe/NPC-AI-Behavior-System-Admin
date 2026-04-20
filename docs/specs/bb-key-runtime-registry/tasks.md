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

## T2：model 层结构  `[ ]`

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

## T3：errcode 新增 7 码  `[ ]`

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

## T4：store/mysql 两个 store  `[ ]`

**关联**：R1, R2 / design §1.4

**文件**：
- `backend/internal/store/mysql/runtime_bb_key.go`（新增 ~200 行）
- `backend/internal/store/mysql/runtime_bb_key_ref.go`（新增 ~80 行）

**做什么**：
1. `RuntimeBbKeyStore` 方法全量（对齐 [`FieldStore`](../../backend/internal/store/mysql/field.go) 模式）：
   - `GetByID / GetByName / List / Create / Update / Delete / Toggle`
   - `CheckEnabledByNames(ctx, names []string) (map[string]bool, error)` —— 空 names 直接返回空 map 不发 SQL
   - List 用 `sqlx.In + Rebind` 展开 IN 查询；LIKE 走 `shared.EscapeLike`
2. `RuntimeBbKeyRefStore` 方法（对齐 [`FieldRefStore`](../../backend/internal/store/mysql/field_ref.go) 模式）：
   - `CreateBatch(tx, keyID int64, refs []RuntimeBbKeyRef) error`
   - `DeleteByKeyIDAndRefIDs(tx, keyID int64, refType string, refIDs []int64) error`
   - `CountByKeyIDs(ctx, keyIDs []int64) (map[int64]int, error)` —— 给 has_refs 判断用
   - `ListByKeyID(ctx, keyID int64) ([]RuntimeBbKeyRef, error)` —— 给 /:id/references 端点用
   - `DeleteByRefTypeAndRefID(tx, refType string, refID int64) error` —— FSM/BT 删除级联
3. 事务版本：所有写方法首参数 `tx *sqlx.Tx`（无 tx 版本走 `db.ExecContext`）
4. Delete 前 `SELECT ... FOR SHARE`（TOCTOU 防护，对齐 mysql red-lines）

**做完了是什么样**：
- `go build ./internal/store/...` 通过
- sqlmock 单测覆盖 `CheckEnabledByNames` 空输入 / 多值展开 / 软删过滤 3 场景

---

## T5：store/redis cache  `[ ]`

**关联**：R16 / design §6.3

**文件**：
- `backend/internal/store/redis/runtime_bb_key_cache.go`（新增 ~150 行）
- `backend/internal/store/redis/shared/` 加 key 常量（`RuntimeBbKeyDetailKey(id)` / `RuntimeBbKeyListKey(req)`）

**做什么**：
1. 对齐 [`field_cache.go`](../../backend/internal/store/redis/field_cache.go) 模式：`GetDetail` / `SetDetail` / `DelDetail` / `GetList` / `SetList` / `InvalidateList`
2. TTL：detail 5min / list 1min（对齐 field）
3. detail 读路径用 `shared.WithLock` 分布式锁（击穿防护）
4. `InvalidateList` 清 list 分页缓存（pattern 匹配）

**做完了是什么样**：
- `go build ./internal/store/redis/...` 通过
- cache red-lines 自查通过（commit 前清缓存、TOCTOU 保护、nil slice 问题）

---

## T6：service 层 CRUD + 冲突检测  `[ ]`

**关联**：R4, R5, R6, R9, R13 / design §1.4

**文件**：`backend/internal/service/runtime_bb_key.go`（新增 ~180 行）

**做什么**：
1. `RuntimeBbKeyService` 构造：持 `store` / `refStore` / `cache` / `fieldStore`（仅读）/ `pagCfg`；**不持**其他 service
2. 实现 CRUD：`List / GetByID / Create / Update / Delete / Toggle`
3. `CheckName(ctx, name)` —— 先查 fields 冲突 → 再查 runtime_bb_keys 自冲突 → 返回 `(conflict, source, err)`；source 在 `"field" / "runtime_bb_key"` 两值之间
4. `CheckByNames(ctx, names []string) (notOK []string, err error)` —— 空 names → nil, nil；非空 → `store.CheckEnabledByNames` 过滤
5. Delete 前 has_refs 检查：`refStore.CountByKeyIDs([id]) > 0` → `ErrRuntimeBBKeyHasRefs`
6. 写路径顺序：tx.Begin → store 写 → **tx.Commit** → cache `DelDetail + InvalidateList`（cache red-lines §写后清缓存顺序）

**做完了是什么样**：
- `go build ./internal/service/...` 通过
- `service/runtime_bb_key_test.go` 覆盖 TestCheckName_FieldConflict / TestCheckName_SelfConflict / TestDelete_HasRefs_Rejected

---

## T7：service 层引用同步（Sync / Delete Refs）  `[ ]`

**关联**：R7, R8 / design §1.7

**文件**：`backend/internal/service/runtime_bb_key.go`（接上 T6，+80 行）

**做什么**：
1. `SyncFsmRefs(ctx, tx, fsmID, oldKeys, newKeys map[string]bool) (affectedKeyIDs []int64, err error)`：
   - diff oldKeys/newKeys → toAdd / toRemove
   - 解析 name → runtime_key_id（走 `store.CheckEnabledByNames` 或新建 `GetIDsByNames`，跳过非 runtime key name）
   - 批量 `refStore.CreateBatch` + `refStore.DeleteByKeyIDAndRefIDs`
   - 返回受影响 keyID 列表（调用方用于清 detail 缓存）
2. `SyncBtRefs` 对称
3. `DeleteRefsByFsmID(tx, fsmID)` / `DeleteRefsByBtID(tx, btID)` —— 走 `refStore.DeleteByRefTypeAndRefID`
4. 算法完全对称 [`field.go:898 SyncFsmBBKeyRefs`](../../backend/internal/service/field.go#L898)，便于未来阅读

**做完了是什么样**：
- 4 个新方法 godoc 与 field.go 对称版本字数差 ±3 行内
- 单测覆盖：TestSyncFsmRefs_AddAndRemove / TestSyncFsmRefs_IgnoresFieldKeys（field key 混入 newKeys 时不误建 ref）

---

## T8：handler + 路由  `[ ]`

**关联**：R4 / design §1.5

**文件**：
- `backend/internal/handler/runtime_bb_key.go`（新增 ~180 行）
- `backend/internal/router/router.go`（+10 行路由注册）
- `backend/internal/setup/*.go`（+30 行装配 store/service/handler）

**做什么**：
1. Handler 方法对齐 [`handler/field.go`](../../backend/internal/handler/field.go) 模式：`List / Detail / Create / Update / Delete / Toggle / CheckName / References`，用 `wrap.go` 统一包装
2. Detail 响应填充 `has_refs` / `ref_count`
3. References 响应：`{items: [{ref_type, ref_id, ref_name}]}`，ref_name 通过 join `fsm_configs` / `bt_trees` 得到（handler 编排，调对应 store 方法查 name）
4. Router 新增 `/api/v1/runtime-bb-keys` 下 8 个端点（见 design §1.5）
5. Setup 装配：`NewRuntimeBbKeyStore` / `NewRuntimeBbKeyRefStore` / `NewRuntimeBbKeyCache` / `NewRuntimeBbKeyService` / `NewRuntimeBbKeyHandler` + 注入 router

**做完了是什么样**：
- `go build ./...` 通过
- 手动 curl 8 端点 200 / 400 / 409 按 design §1.5 返回
- 响应 JSON 含 `code` 字段（red-lines/general §HTTP 响应格式）

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
