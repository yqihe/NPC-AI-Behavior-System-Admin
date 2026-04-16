# code-review-fixes — 需求分析

## 动机

对全量前后端代码做系统性审查后，发现若干健壮性和一致性问题。按优先级分为：

- **P0（当前删除功能损坏）**：BtNodeType 删除接口 handler 使用 `IDVersionRequest` 并调 `CheckVersion`，而前端只发 `{id}`，version 默认为 0，`CheckVersion(0)` 返回 400——删除功能目前完全不可用。
- **P1（性能/一致性）**：维护了 `bt_node_type_refs` 索引表却不用，引用检查走全表 JSON_SEARCH。
- **P2（类型错误）**：前端响应类型定义有误（`fsmConfigApi.delete` 响应写了不存在的 `label` 字段）。
- **P4（一致性）**：一个 service 中锁超时硬编码数字，其余模块均用常量。

注：审查发现的 `getOrNotFound` nil 解引用（P0 panic）已在 commit `48016ff` 中修复，不再列入本 spec。

**设计澄清（Phase 2 读完 backend-conventions.md 后更新）**：
初始审查将"BtNodeType delete 无版本锁"定为 P1 TOCTOU 问题，但 `backend-conventions.md` 明确规定：delete 接口统一用 `model.IDRequest`，版本锁不用于保护删除，业务约束（enabled=false 才能删）承担并发保护。正确的 bug 是 handler 偏离了这条规范，引入了 `IDVersionRequest` + `CheckVersion`，导致当前删除功能彻底损坏。

不修则：
- BtNodeType 删除功能当前完全不可用（前端每次调用都 400）
- 已维护的 `bt_node_type_refs` 表白白承担写入成本，引用检查退化为 O(N·JSON解析)
- 前端 `fsmConfigApi.delete` 的错误类型定义误导后续开发

---

## 优先级

**P0/P1**：高，BtNodeType 删除已损坏，应立即修复。

**P2/P4**：低，可与 P0/P1 同批次修复，不阻塞进行中功能开发。

---

## 预期效果

### 场景 1：BtNodeType 删除功能恢复正常（R1/R2）

**修复前**：用户点击删除节点类型 → 前端调 `btNodeTypeApi.delete(row.id)` → 请求 body `{id: 1}` → handler 执行 `CheckVersion(0)` → 返回 400 "版本号不合法"。删除操作完全无法完成。

**修复后**：
- Handler 改用 `model.IDRequest`，移除 `CheckVersion` 调用
- Service.Delete 移除无用的 `version` 形参
- 前端无需改动（已只发 `{id}`）
- 用户正常点删除 → 检查 enabled=false、未被引用 → 成功删除，列表刷新

### 场景 2：bt_node_type_refs 引用检查走索引（R3）

**修复前**：删除节点类型前调 `IsNodeTypeUsed`，底层跑 JSON_SEARCH 全表扫描，O(N)。

**修复后**：查 `bt_node_type_refs` 表走 `idx_type_name` 索引，O(1)。`GetNodeTypeUsages` 同理改为 JOIN 查询。

### 场景 3：fsmConfigApi.delete 类型正确（R4）

**修复前**：响应类型有 `label` 字段（FSM 没有 label）。

**修复后**：响应类型改为 `{ id: number; name: string; display_name: string }`。

### 场景 4：FieldService 锁 TTL 统一（R5）

**修复前**：`s.fieldCache.TryLock(ctx, id, 3*time.Second)` — 数字字面量。

**修复后**：`s.fieldCache.TryLock(ctx, id, rcfg.LockExpire)` — 与其他所有 service 一致。

---

## 依赖分析

### 依赖的已完成工作

- `bt_node_type_refs` 表（migration 012）：已建表，`SyncNodeTypeRefsTx` 已在 BT 树 Create/Update 时写入
- `errcode.ErrVersionConflict`：已存在（不在 delete 路径用，仅确认存在不影响本 spec）
- `rcfg.LockExpire`：已在 `store/redis/shared/common.go` 定义
- `model.IDRequest`：已存在，所有其他模块 delete handler 已在用

### 谁依赖这个需求

- 无下游 spec 依赖本次修复
- 游戏服务端的导出 API 不涉及

---

## 改动范围

| 文件 | 改动内容 | 行数预估 |
|------|---------|---------|
| `backend/internal/handler/bt_node_type.go` | Delete 入参 `IDVersionRequest` → `IDRequest`，移除 `CheckVersion` | ~4 行 |
| `backend/internal/service/bt_node_type.go` | Delete 签名去掉 `version int` 形参，内部调用同步 | ~3 行 |
| `backend/internal/store/mysql/bt_tree.go` | 重写 `IsNodeTypeUsed` / `GetNodeTypeUsages` | ~25 行（改写） |
| `frontend/src/api/fsmConfigs.ts` | delete 响应类型 `label` → `display_name` | 1 行 |
| `backend/internal/service/field.go` | `TryLock` 参数 `3*time.Second` → `rcfg.LockExpire` | 1 行 |

预估：5 个文件，净改动 < 35 行。

---

## 扩展轴检查

- **新增配置类型**：R1/R2 正面影响——修复后 BtNodeType delete handler 成为标准模板（IDRequest，convention 对齐），新增类型参照此写法即可。
- **新增表单字段**：不涉及。

---

## 验收标准

**R1**：`BtNodeTypeHandler.Delete` 入参改为 `*model.IDRequest`，移除 `shared.CheckVersion` 调用。

**R2**：`BtNodeTypeService.Delete` 签名改为 `Delete(ctx context.Context, id int64) (*model.BtNodeTypeDeleteResult, error)`，去掉 `version int` 形参。BtNodeTypeStore.SoftDelete 不变。

**R3a**：`BtTreeStore.IsNodeTypeUsed` 改为 `SELECT COUNT(*) FROM bt_node_type_refs WHERE type_name = ?`，不再使用 `JSON_SEARCH`。

**R3b**：`BtTreeStore.GetNodeTypeUsages` 改为 JOIN `bt_node_type_refs + bt_trees` 查询，不再使用 `JSON_SEARCH`。

**R4**：`fsmConfigs.ts` 中 `fsmConfigApi.delete` 响应类型改为 `{ id: number; name: string; display_name: string }`，`label` 字段消失。

**R5**：`service/field.go` 中 `TryLock` 调用第三个参数为 `rcfg.LockExpire`，不含 `time.Second` 字面量。

**R6**：`go build ./...` 通过；`npx vue-tsc --noEmit` 通过。

---

## 不做什么

1. **不给 SoftDelete 加 version 参数**——convention 明确：删除防并发用业务约束，不用版本锁
2. **不改 bt_node_type_refs 的写入逻辑**——`SyncNodeTypeRefsTx` / `DeleteNodeTypeRefsTx` 维护逻辑不动
3. **不改其他模块的 delete handler**——Field、Template、FSM 等已符合 IDRequest 规范，不扩大范围
4. **不改前端删除调用侧**——`btNodeTypeApi.delete(id)` 已只发 id，无需改动
5. **不修 Redis key 转义问题**——毕设阶段受控场景，留后续版本
6. **不统一 handler Get/Detail 命名**——纯风格，不动运行中代码
