# code-review-fixes — 任务拆解

依赖顺序：T1 → T2（handler 依赖 service 签名）；T3/T4/T5 独立，可任意顺序。

---

## [x] T1：修复 BtNodeTypeService.Delete — 移除无用 version 形参 (R2)

**涉及文件**：`backend/internal/service/bt_node_type.go`

**改动**：
- `Delete(ctx context.Context, id int64, version int)` → `Delete(ctx context.Context, id int64)`
- 函数体内 `version` 变量全部删除（原来只传给 `s.store.SoftDelete` 但 SoftDelete 不接收 version，实际上是死参数）

**完成定义**：`BtNodeTypeService.Delete` 签名只有 `(ctx, id int64)`，`go build ./...` 通过。

---

## [x] T2：修复 BtNodeTypeHandler.Delete — IDVersionRequest → IDRequest (R1)

**涉及文件**：`backend/internal/handler/bt_node_type.go`

**改动**：
- 入参类型 `*model.IDVersionRequest` → `*model.IDRequest`
- 删除 `shared.CheckVersion(req.Version)` 调用及其 error return
- 调用 service 时由 `h.svc.Delete(ctx, req.ID, req.Version)` → `h.svc.Delete(ctx, req.ID)`

**完成定义**：Handler Delete 方法入参为 `*model.IDRequest`，无 CheckVersion 调用，`go build ./...` 通过。

**依赖**：T1 必须先完成（service 签名变更后 handler 才能正确编译）。

---

## [x] T3：IsNodeTypeUsed / GetNodeTypeUsages 改走 bt_node_type_refs 索引 (R3a/R3b)

**涉及文件**：`backend/internal/store/mysql/bt_tree.go`

**改动**：

`IsNodeTypeUsed`：
```go
// 原 JSON_SEARCH 全表扫
// 改为：
var count int
err := s.db.GetContext(ctx, &count,
    `SELECT COUNT(*) FROM bt_node_type_refs WHERE type_name = ?`, typeName)
return count > 0, err
```

`GetNodeTypeUsages`：
```go
// 原 JSON_SEARCH 全表扫
// 改为：
SELECT bt.name
FROM bt_trees bt
INNER JOIN bt_node_type_refs r ON r.bt_tree_id = bt.id
WHERE r.type_name = ? AND bt.deleted = 0
```

**完成定义**：两个方法 SQL 不含 `JSON_SEARCH`，改用 `bt_node_type_refs` 表；`go build ./...` 通过。

**注意**：`walkNodes`、`extractBBKeys`、`IsBBKeyUsed`、`GetBBKeyUsages`、`SyncNodeTypeRefsTx`、`DeleteNodeTypeRefsTx` 等其他方法一概不改。

---

## [x] T4：FieldService TryLock TTL 改用 rcfg.LockExpire 常量 (R5)

**涉及文件**：`backend/internal/service/field.go`

**改动**：
- `GetByID` 方法中：`s.fieldCache.TryLock(ctx, id, 3*time.Second)` → `s.fieldCache.TryLock(ctx, id, rcfg.LockExpire)`
- 确认 `rcfg` import alias 已存在（`rcfg "github.com/yqihe/npc-ai-admin/backend/internal/store/redis/shared"`）；若不存在则补加

**完成定义**：`field.go` 中无 `3*time.Second` 字面量用于 TryLock，`go build ./...` 通过。

---

## [x] T5：fsmConfigApi.delete 响应类型注释补充 (R4)

**涉及文件**：`frontend/src/api/fsmConfigs.ts`

**改动**：
- 在 `delete` 行的响应类型上补充注释，说明 `label` 字段实际来自后端 `fc.DisplayName`（`model.DeleteResult.Label` json tag 是 `"label"`，与 FSM 的 `display_name` 字段同义但字段名不同）：

```typescript
// 注意：响应中 label 来自后端 DeleteResult.Label，实为 FSM 的 display_name
delete: (id: number) =>
    request.post('/fsm-configs/delete', { id }) as Promise<ApiResponse<{ id: number; name: string; label: string }>>,
```

**完成定义**：`fsmConfigs.ts` delete 行有注释，`npx vue-tsc --noEmit` 通过。

---

## 执行顺序

```
T1 (service) → T2 (handler)   [串行，T2 依赖 T1]
T3 (store)                     [独立]
T4 (service/field)             [独立]
T5 (frontend)                  [独立]
```

T3/T4/T5 可与 T1→T2 并行，但建议按序执行，每个任务 commit 后继续下一个。

---

## 全部完成后

```bash
go build ./...
cd frontend && npx vue-tsc --noEmit
```

两条命令均通过则验收通过。
