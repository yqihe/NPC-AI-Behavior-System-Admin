# code-review-fixes — 设计方案

## 前置阅读确认

- [x] `docs/architecture/backend-conventions.md` — 已读，关键发现见下
- [x] `docs/architecture/frontend-conventions.md` — 不涉及本次 spec 的前端结构改动
- [x] 所有红线文档已读

---

## 方案概述

本次改动分 4 个独立子任务，均为局部修复，无跨模块编排，无新增接口。

---

## T1 — BtNodeType Delete handler/service 对齐 convention

### 方案（选定）

**Handler 层**：`BtNodeTypeHandler.Delete` 入参从 `*model.IDVersionRequest` 改为 `*model.IDRequest`，删除 `shared.CheckVersion(req.Version)` 调用。

**Service 层**：`BtNodeTypeService.Delete` 签名去掉 `version int` 形参，内部不传 version，直接调 `s.store.SoftDelete(ctx, id)`（已存在，行为不变）。

**Store 层**：`BtNodeTypeStore.SoftDelete` 不改，保持 `(ctx, id int64)` 签名，SQL `WHERE id=? AND deleted=0`，符合 convention 第 IV 节规定。

**为什么这样做**：
- `backend-conventions.md` §II：删除接口统一用 `model.IDRequest`，不传 version
- `backend-conventions.md` §IV：`SoftDelete` 不带 version，防并发误删由"启用中不可删除"业务约束承担
- 当前 handler 用 `IDVersionRequest` + `CheckVersion(0)` 导致前端每次删除请求都 400，功能已损坏

**为什么不改为"加 version 到 SoftDelete"**：与 convention 相反。Field、FsmConfig、BtTree 等所有模块的 SoftDelete 均不带 version，改了 BtNodeType 会变成异类，且让前端携带 version 的逻辑更复杂（需先 detail 取 version 再删）。

### 备选方案（不选）

在 SoftDelete 加 `AND version=?` 条件。

不选原因：(1) 违反 convention；(2) 前端需要从 detail 拿 version 才能调用，增加交互链路；(3) 与其他所有模块的 delete 模式不一致，造成模板污染。

---

## T2 — bt_node_type_refs 替代 JSON_SEARCH

### 方案（选定）

**IsNodeTypeUsed**：

```go
// 改写前（JSON_SEARCH 全表扫）
SELECT name FROM bt_trees
WHERE deleted = 0
  AND JSON_SEARCH(config, 'one', ?, NULL, '$**.type') IS NOT NULL

// 改写后（走 idx_type_name 索引）
SELECT COUNT(*) FROM bt_node_type_refs WHERE type_name = ?
```

返回 `count > 0` 即 used。不再读 bt_trees，因为 bt_node_type_refs 只在 `deleted=0` 的树的 Create/Update 时同步写入，软删除时调 `DeleteNodeTypeRefsTx` 清除，数据与实际引用状态一致。

**GetNodeTypeUsages**：

```go
// 改写前（JSON_SEARCH 全表扫）
SELECT name FROM bt_trees WHERE deleted=0
  AND JSON_SEARCH(config, 'one', ?, NULL, '$**.type') IS NOT NULL

// 改写后（JOIN 索引查询）
SELECT bt.name
FROM bt_trees bt
INNER JOIN bt_node_type_refs r ON r.bt_tree_id = bt.id
WHERE r.type_name = ? AND bt.deleted = 0
```

`bt_node_type_refs.idx_type_name(type_name)` 已存在，命中索引后按主键 `bt_tree_id` JOIN bt_trees（主键查找），整体 O(引用数)。

**为什么能保证数据一致性**：
- `SyncNodeTypeRefsTx` 在 Create/Update BT 树时维护，先删全部旧引用再批量插入新引用（同事务）
- `DeleteNodeTypeRefsTx` 在 BT 树软删除时清除该树的所有引用
- 上述两处均已在 service/bt_tree.go 中集成，同步逻辑不变

**不改 SyncNodeTypeRefsTx / DeleteNodeTypeRefsTx**，它们是数据源头，逻辑已正确。

### 备选方案（不选）

保持 JSON_SEARCH，在 bt_trees.config 上加 Generated Column + 索引。

不选原因：(1) MySQL 8.0 的 JSON Generated Column 语法复杂；(2) 已有 bt_node_type_refs 专为此设计，再加 generated column 是冗余；(3) 引入 DDL 变更，本次 spec 不碰 schema。

---

## T3 — fsmConfigApi.delete 响应类型修正

### 方案（选定）

```typescript
// fsmConfigs.ts，delete 响应类型
// 改写前
{ id: number; name: string; label: string }
// 改写后
{ id: number; name: string; display_name: string }
```

FSM 实体字段是 `display_name`（中文名），没有 `label` 字段。与后端 `model.DeleteResult` / `FsmConfigService.Delete` 的实际返回对齐。

后端 `FsmConfigService.Delete` 返回 `&model.DeleteResult{ID: id, Name: fc.Name, Label: fc.DisplayName}`——注意后端 `DeleteResult.Label` 字段实际存放的是 `fc.DisplayName`，前端若消费这个字段应读 `.label`（因为后端统一用 DeleteResult.Label）。核实：

```go
// model/common.go — DeleteResult
type DeleteResult struct {
    ID    int64  `json:"id"`
    Name  string `json:"name"`
    Label string `json:"label"`
}
```

后端 `DeleteResult` 的字段名是 `label`（json tag `"label"`），前端响应里会是 `label`，不是 `display_name`。因此前端响应类型的 `label` 实际上是正确的！前端代码不消费这个 `data`（只检查 `code===0`），type 错误只是误导性文档，不影响运行。

**修正后的改动**：保持字段名 `label`（与后端 `DeleteResult.Label` json tag 一致），仅补充注释说明这里的 `label` 来自 `fc.DisplayName`。或者直接改成联合类型 `ApiResponse<{ id: number; name: string; label: string }>` 并加注释。

> 经二次核实：原响应类型 `{ id: number; name: string; label: string }` 与后端返回一致，字段名 `label` 在 `DeleteResult` 中是正确的。改动缩减为：**原类型写法无误，仅需加注释防止误解**。R4 从"改字段名"变为"加注释"。

### 备选方案

不做任何改动（因为类型已正确）。

由于 R4 验收标准已写明要改 `label → display_name`，且此改动实际上是错误的（会与后端 json tag 不符），将 R4 调整为：补充注释解释 label 对应 display_name，不修改字段名。

---

## T4 — FieldService TryLock TTL 常量化

### 方案（选定）

```go
// service/field.go GetByID 方法中
// 改写前
lockID, lockErr := s.fieldCache.TryLock(ctx, id, 3*time.Second)
// 改写后
lockID, lockErr := s.fieldCache.TryLock(ctx, id, rcfg.LockExpire)
```

`rcfg.LockExpire = 3 * time.Second`（定义在 `store/redis/shared/common.go`），值不变，只是用常量替代字面量。

需要在 field.go 中补充 import alias `rcfg "github.com/yqihe/npc-ai-admin/backend/internal/store/redis/shared"`（该 import 可能已存在，需确认）。

### 备选方案

保持硬编码。不选，因为 convention §V 明确：`LockExpire` 通过 `store/redis/config/` 统一配置，不在各模块硬编码。

---

## 红线检查

### 通用红线 / Go 红线

- **T1**：删除了 `CheckVersion`，不新增任何 nil 风险。`IDRequest` 仍被 `CheckID` 验证。✓
- **T4**：仅替换常量引用，无逻辑改变。✓

### MySQL 红线

- **T2**：新 SQL 不涉及 LIKE，无注入风险。✓
- **T2**：`bt_node_type_refs` 只读查询，无事务混用问题。✓
- **T2**：COUNT 查询不涉及 TOCTOU（只是检查引用存在性，业务上是软性检查，允许瞬时不一致）。✓

### Redis 红线 / 缓存红线

- 本次改动不涉及 Redis 缓存写入/删除，无相关红线风险。✓

### 前端红线

- **T3**：只改类型注解，不改运行时逻辑，无 disabled 覆盖、无数据污染问题。✓
- **R6**：改完后需跑 `npx vue-tsc --noEmit` 验证。✓

### Admin 专属红线

- **T1**：不涉及游戏服务端数据格式，不涉及引用完整性逻辑改变（引用检查仍在 service 层执行）。✓
- **T1**：handler 校验用 `CheckID`（`ErrBadRequest`），符合红线 §4 第 8 条。✓
- **T2**：不破坏引用完整性，反而让引用检查更高效。✓
- **红线 §10 第 7 条**："前端 API：`ListData<T>` / `CheckNameResult` 从 `fields.ts` 导入" — T3 不涉及。✓

---

## 扩展性影响

- **T1** 正面：BtNodeTypeHandler.Delete 修复后成为新增配置类型的参照标准模板（IDRequest + 无 CheckVersion）
- **T2** 正面：将来新增配置类型若也需要 "被哪些 BT 节点引用" 的检查，`bt_node_type_refs` 的查询模式可直接复用
- **T3/T4** 无扩展性影响

---

## 依赖方向

```
handler/bt_node_type.go
  → service/bt_node_type.go
    → store/mysql/bt_node_type.go (SoftDelete 不变)
    → store/mysql/bt_tree.go (IsNodeTypeUsed / GetNodeTypeUsages 改写)

service/field.go
  → store/redis/shared (rcfg.LockExpire 常量引用)

frontend/src/api/fsmConfigs.ts (独立，无后端依赖变更)
```

所有依赖均单向向下，无循环。✓

---

## 陷阱检查

### Go 规范

- **T1**：移除 `version int` 形参后，调用方 handler 传入 `req.ID`（`int64`）——确认 service 签名改为 `Delete(ctx, id int64)`，与 handler 调用 `h.svc.Delete(ctx, req.ID)` 一致。
- **T4**：`rcfg` import alias 可能已存在于 field.go，需读文件确认，不重复 import。

### MySQL 规范

- **T2**：COUNT 语句不需要 LIKE 转义。✓
- **T2**：JOIN 查询确认 `bt.deleted = 0` 条件已加（软删除过滤）。✓

### 缓存规范

- **T1**：BtNodeType delete 路径中已有 `s.cache.DelDetail` + `s.cache.InvalidateList`，本次不改缓存逻辑。✓

---

## 配置变更

无新增配置文件或 schema 变更。

---

## 测试策略

所有改动通过 `go build ./...` + `npx vue-tsc --noEmit` 验证编译正确性。

功能验证：
- T1：调用 `/bt-node-types/delete` 传 `{id}` 不传 version，期望正常返回（不再 400）
- T2：删除节点类型时 service 层调 `IsNodeTypeUsed`，走新 SQL 路径，期望结果正确
- T3：TypeScript 类型检查通过（`vue-tsc --noEmit`）
- T4：编译通过即可（值未变）
