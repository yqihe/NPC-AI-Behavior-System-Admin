# 字段管理后端重构 — 任务拆解

## T1: 新增迁移脚本 + 错误码 (R9, R12)

**涉及文件**：
- `backend/migrations/004_refactor_field_refs.sql`（新建）
- `backend/internal/errcode/codes.go`

**做什么**：
1. 新建 `004_refactor_field_refs.sql`：DROP field_refs → CREATE field_refs (field_id BIGINT, ref_type, ref_id BIGINT)
2. 在 `codes.go` 中新增 `ErrFieldEditNotDisabled = 40015`，消息 `"请先停用该字段再编辑"`
3. 移除批量相关错误码的注释说明（如有）

**做完是什么样**：迁移脚本可执行，错误码常量可编译引用。

---

## T2: Model 层重写 (R1-R7, R9)

**涉及文件**：
- `backend/internal/model/field.go`

**做什么**：
1. `FieldRef` 结构体：`FieldName/RefName string` → `FieldID/RefID int64`
2. 新增 `IDRequest{ID int64}`，替代 `NameRequest`（保留 `CheckNameRequest`）
3. `UpdateFieldRequest`：去掉 `Name` 字段，加 `ID int64`
4. `ToggleEnabledRequest`：`Name` → `ID int64`
5. 移除：`BatchDeleteRequest`、`BatchDeleteResult`、`BatchDeleteSkipped`、`BatchCategoryRequest`、`BatchCategoryResponse`
6. `ReferenceDetail`：内部引用方标识改为 ID

**做完是什么样**：`go build` model 包无编译错误（其他包暂时报错正常）。

---

## T3: Redis 缓存 key + 缓存操作改用 ID (R10, R18)

**涉及文件**：
- `backend/internal/store/redis/keys.go`
- `backend/internal/store/redis/field.go`

**做什么**：
1. `keys.go`：`FieldDetailKey(name string)` → `FieldDetailKey(id int64)`；`FieldLockKey(name string)` → `FieldLockKey(id int64)`
2. `field.go`：所有调用 `FieldDetailKey`/`FieldLockKey` 的地方改传 `int64`
3. `GetDetail`/`SetDetail`/`DelDetail`/`TryLock`/`Unlock` 的参数从 `name string` 改为 `id int64`

**做完是什么样**：redis 包编译通过，key 格式为 `fields:detail:123`。

---

## T4: FieldStore 改用 ID (R1-R5, R11)

**涉及文件**：
- `backend/internal/store/mysql/field.go`

**做什么**：
1. 新增 `GetByID(ctx, id) (*Field, error)` — `SELECT * FROM fields WHERE id=? AND deleted=0`
2. 保留 `GetByName` 和 `ExistsByName`（check-name 用）
3. `Update`：WHERE 条件从 `name=? AND version=?` 改为 `id=? AND version=?`
4. `SoftDeleteTx`：WHERE 从 name 改为 id
5. `ToggleEnabled`：WHERE 从 name 改为 id
6. `IncrRefCountTx`/`DecrRefCountTx`：WHERE 从 name 改为 id
7. 新增 `GetByIDs(ctx, ids) ([]Field, error)` — IN 查询批量取 label
8. `Create` 返回值改为 `(int64, error)`，返回 `result.LastInsertId()`
9. 移除：`BatchUpdateCategory`、`GetByNames`

**做完是什么样**：field store 编译通过，所有 SQL WHERE 条件使用 id。

---

## T5: FieldRefStore 改用 ID (R9, R17)

**涉及文件**：
- `backend/internal/store/mysql/field_ref.go`

**做什么**：
1. `Add(tx, fieldID int64, refType string, refID int64)` — INSERT IGNORE 三列改 BIGINT
2. `Remove(tx, fieldID int64, refType string, refID int64)` — DELETE 改 BIGINT
3. `RemoveBySource(tx, refType string, refID int64) ([]int64, error)` — 返回被引用的 fieldID 列表
4. `GetByFieldID(ctx, fieldID int64) ([]FieldRef, error)` — 替代 GetByFieldName
5. `HasRefsTx(tx, fieldID int64) (bool, error)` — FOR SHARE 改 BIGINT
6. 移除 `GetByRefName`（功能合并到 RemoveBySource）

**做完是什么样**：field_ref store 编译通过，全部使用 BIGINT 参数。

---

## T6: Service 层重写 — 创建 + 唯一性校验 (R1, R6, R16, R17)

**涉及文件**：
- `backend/internal/service/field.go`（Create、CheckName、辅助函数部分）

**做什么**：
1. `getFieldOrNotFound` 改为接收 `id int64`，内部调 `fieldStore.GetByID`
2. `Create`：调用链不变，写入 field_refs 时传 ID（被引用字段的 ID 从 GetByID 获取）；循环引用检测 DFS 改用 ID
3. `CheckName`：逻辑不变（仍用 name）
4. 辅助函数 `syncFieldRefs` / `checkCyclicRef` 改用 ID 参数
5. 清理缓存：`fieldCache.InvalidateList()`

**做完是什么样**：Create 和 CheckName 逻辑可编译，引用关系维护使用 ID。

---

## T7: Service 层重写 — 编辑 + 约束收紧 (R2, R12, R15, R16, R17)

**涉及文件**：
- `backend/internal/service/field.go`（Update 部分）

**做什么**：
1. `Update` 入口按 ID 查字段
2. 新增 **enabled=0 校验**：`if field.Enabled { return 40015 }`
3. ref_count > 0 时：禁止改类型(40006)、约束收紧检查(40007)
4. reference 类型：diff 计算引用增减，事务内维护 field_refs + ref_count（全部用 ID）
5. 乐观锁更新（WHERE id=? AND version=?）
6. 清缓存：DEL detail:{id} + INCR version + 级联清被引用方 detail

**做完是什么样**：Update 逻辑可编译，未启用才能编辑的规则生效。

---

## T8: Service 层重写 — 删除 + 列表 + 切换 + 引用详情 (R3-R5, R7, R13, R14, R18)

**涉及文件**：
- `backend/internal/service/field.go`（Delete、List、ToggleEnabled、GetReferences 部分）

**做什么**：
1. `Delete`：按 ID 查 → enabled=0 校验 → 事务内 FOR SHARE 检查引用 → 软删除 → 清理 reference 类型引用 → 清缓存
2. `List`：逻辑不变（列表项已含 id 字段）
3. `ToggleEnabled`：按 ID 查 → 乐观锁更新 → 清缓存
4. `GetReferences`：按 ID 查字段 → 查 field_refs WHERE field_id=? → 按 ref_type 分组 → GetByIDs 拿 label
5. 移除：`BatchDelete`、`BatchUpdateCategory`

**做完是什么样**：剩余 4 个 Service 方法可编译，批量方法已移除。

---

## T9: Handler 层重写 (R1-R7)

**涉及文件**：
- `backend/internal/handler/field.go`

**做什么**：
1. `Create`：不变（请求体无 ID）
2. `Update`：校验 `req.ID > 0`（用 ErrBadRequest）；去掉 name 校验
3. `Get`（详情）：参数改为 `IDRequest`，校验 `id > 0`
4. `Delete`：参数改为 `IDRequest`，校验 `id > 0`
5. `ToggleEnabled`：校验 `req.ID > 0`
6. `GetReferences`：参数改为 `IDRequest`，校验 `id > 0`
7. `CheckName`：不变（仍用 name）
8. 移除：`BatchDelete`、`BatchUpdateCategory`

**做完是什么样**：Handler 层 8 个方法可编译，入参/出参使用 ID。

---

## T10: Router 层 + 编译验证 (R1-R8, R19, R20)

**涉及文件**：
- `backend/internal/router/router.go`

**做什么**：
1. 移除 `batch-delete` 和 `batch-category` 路由
2. 确认剩余 8 条路由指向正确的 Handler 方法
3. `go build ./...` 全量编译通过
4. 检查无未使用的 import / 变量

**做完是什么样**：整个 backend 编译通过，路由只有 8 条 + 1 条 health + 1 条 dictionaries。

---

## 依赖顺序

```
T1 (迁移+错误码)
 └→ T2 (Model)
     ├→ T3 (Redis keys/cache)
     ├→ T4 (FieldStore)
     └→ T5 (FieldRefStore)
         ├→ T6 (Service: Create+CheckName)
         ├→ T7 (Service: Update)
         └→ T8 (Service: Delete+List+Toggle+Refs)
             └→ T9 (Handler)
                 └→ T10 (Router + 编译验证)
```
