# 需求：NPC 导出端点引用完整性校验

## 动机

游戏服务端 v3 API 契约规定：`/api/configs/npc_templates` 返回的每条 NPC 中，`behavior.fsm_ref` 与 `behavior.bt_refs.value` 必须指向真实存在且 enabled 的 FSM/BT 配置。**任何悬空引用都会让游戏服务端启动 fatal 退出**（契约原文）。

ADMIN 现状：
- NPC 创建/更新时校验过引用（[npc_service.go:425](backend/internal/service/npc_service.go#L425) `ValidateBehaviorRefs`）
- 但 `ExportAll`（[npc_service.go:591](backend/internal/service/npc_service.go#L591)）**完全不复核**——只查 NPC 表后直接拼装，FSM/BT 之后被禁用、删除、改名都不会拦截

不做的后果：运营在 ADMIN 禁用一条 FSM/BT 后，游戏服端下次启动直接 fatal，且错误信息只在游戏服端日志里，运营不知道是 ADMIN 数据问题，要靠跨人沟通定位。

## 优先级

**P0（阻塞）**。这是游戏服务端联调的硬契约，未修复则联调随时可能 fatal。

依据：
- 契约里**唯一被显式标注"会让服务端启动 fatal 退出"**的约束之一
- regions 已确认延后，本 spec 是当前阶段对齐契约的**主要剩余工作**
- 工程量小（预估 2-4 文件，前置 helper 已在 T6/T7 提交里就绪），无理由延后

## 预期效果

**正常路径**：所有 NPC 的 FSM/BT 引用都有效 → `GET /api/configs/npc_templates` 返回 200 + 完整 items 列表（与现状一致）。

**异常路径**：任一 NPC 的 `fsm_ref` 或某个 `bt_refs.value` 悬空（不存在或已禁用） → 端点返回 **500**，响应 body 明确列出**所有**悬空引用（不只是第一个），格式：

```json
{
  "code": 45010,
  "msg": "NPC 导出引用悬空",
  "details": [
    {"npc_name": "guard_basic",  "ref_type": "fsm_ref", "ref_value": "guard_v2",   "reason": "not_found"},
    {"npc_name": "merchant",     "ref_type": "bt_ref",  "ref_value": "trade/sell", "reason": "disabled", "state": "selling"}
  ]
}
```

同时 slog 输出 ERROR 级别日志，含全部悬空条目，便于排查。

**审计日志**：每次导出失败都记一条审计日志（操作者 = 调用者 IP / token，操作 = export_npc_templates，结果 = failed_dangling_ref，detail = 上述 details）。

**性能**：N 个 NPC 引用 M 个 FSM、K 个 BT，校验只发起 **2 次 SQL**（FSM 一次批量、BT 一次批量），与现有 `CheckEnabledByNames` 模式一致。

## 依赖分析

**依赖（已就绪）**：
- [FsmConfigService.GetEnabledByName](backend/internal/service/fsm_config.go#L584)（T7 已实现）→ 本 spec 改用批量版（见下）
- [BtTreeService.CheckEnabledByNames](backend/internal/service/bt_tree.go#L387)（T7 已实现）→ 直接复用
- [FsmConfigStore.GetByName](backend/internal/store/mysql/fsm_config.go)（T6 已实现）→ 本 spec 需新增批量版 `GetEnabledByNames`

**新增依赖**：
- `FsmConfigStore.GetEnabledByNames(ctx, names)`（仿 `BtTreeStore.GetEnabledByNames`，避免 N+1 查询）
- `FsmConfigService.CheckEnabledByNames`（service 层包装，对齐 BT 的 helper 命名）

**谁依赖此 spec**：
- 游戏服务端（联调主用例）
- 未来 fsm_configs / bt_trees 端点若也加导出期校验，会复用本 spec 沉淀的"导出前置校验"模式

## 改动范围

预估 **3-5 文件，0 新包**：

| 文件 | 改动 |
|---|---|
| `backend/internal/store/mysql/fsm_config.go` | 新增 `GetEnabledByNames(ctx, names) (map[string]bool, error)` |
| `backend/internal/service/fsm_config.go` | 新增 `CheckEnabledByNames(ctx, names) (notOK []string, err error)`，对齐 BT |
| `backend/internal/service/npc_service.go` | `ExportAll` 内插入引用复核；新增私有 helper `validateExportRefs` |
| `backend/internal/errcode/` | 新增错误码 `ErrNPCExportDanglingRef`（45010 段） |
| `backend/internal/service/npc_service_test.go` | 单测 5 场景 |

**不动**：handler / model / router / migration / 前端 / 其他 3 个 export 端点。

## 扩展轴检查

- **新增配置类型轴**：本 spec 不增/不减配置类型，**中性**。但沉淀的"导出前置校验" pattern（service 层私有 helper + 批量 store 查询 + 统一错误码段）将是后续给 fsm_configs/bt_trees 加同类校验的样板。
- **新增表单字段轴**：纯后端校验，**不涉及**。

**为什么两个轴都没有正面收益仍要做**：本 spec 是**契约对齐的缺陷修复**，不是功能扩展。扩展轴检查是为了避免"加一个功能要改十处"，本 spec 只修一个 API 端点的内部行为，扩展轴不适用。

## 验收标准

| 编号 | 描述 | 验证方式 |
|---|---|---|
| R1 | `NpcService.ExportAll` 在拼装前对每条 NPC 校验 `fsm_ref` 与 `bt_refs.value` 在 MySQL 中存在且 `enabled = true` | 单测：注入禁用 FSM 后调用 ExportAll，断言返回 errcode |
| R2 | 校验产生的 SQL 不超过 **2 次**（FSM 批量 + BT 批量），与 NPC 数量无关 | 单测：mock store，断言 GetEnabledByNames 各被调用一次 |
| R3 | 任一引用悬空 → 整端点返回 HTTP 500，body 含错误码 `ErrNPCExportDanglingRef` (45010) + `details` 数组列出**所有**悬空条目（不止第一个） | e2e：构造 2 条 NPC 各引用一个禁用 FSM/BT，curl 验证 details 长度=2 |
| R4 | `details` 每项包含 `npc_name` / `ref_type` (`fsm_ref` 或 `bt_ref`) / `ref_value` / `reason` (`not_found` 或 `disabled`)；BT 项额外含 `state`（FSM 状态名） | e2e + 单测：检查 JSON 字段 |
| R5 | 校验失败时 slog 输出 ERROR 日志，包含完整 details；审计日志记一条 `export_npc_templates / failed_dangling_ref` | 单测 + 手测查日志 |
| R6 | 全部引用有效时 ExportAll 行为与现状完全一致（200 + items）；空 NPC 列表时返回 `{"items":[]}` | e2e：seed 一条正常 NPC，curl 验证 |
| R7 | 不影响 `event_types` / `fsm_configs` / `bt_trees` 三个 export 端点 | e2e：分别 curl 三个端点，行为与本 spec 前一致 |
| R8 | 单测覆盖 5 个场景：全部正常 / FSM 不存在 / FSM 禁用 / BT 不存在 / BT 禁用 | go test ./internal/service/... 全绿 |

## 不做什么

明确排除：

1. **regions 端点**——已确认延后到毕设后（见 memory `project_deferred_features.md`）
2. **FSM `condition` 引用的 BB Key 注册校验**（Gap 3）——独立 spec
3. **BT 节点结构 schema 校验**（Gap 4，`set_bb_value.key` 注册、`parallel.policy` 取值等）——独立 spec
4. **NPC `fields` 跨模板类型一致性**（Gap 5）——独立 spec
5. **fsm_configs / bt_trees 端点的导出期校验**——本 spec 只动 NPC，但留下可复用 pattern
6. **前端改动**——失败时 ADMIN 内部如何展示由后续 UI spec 处理；本次仅保证 API 行为正确
7. **NPC 创建/更新时的校验逻辑**——已有 `ValidateBehaviorRefs` 不动
8. **导出端点鉴权 / 限流改造**——若现状无鉴权，本 spec 不补
9. **失败策略改成"跳过悬空 NPC"的宽容模式**——本 spec 选 fail-fast，理由见下

## 失败策略决策（待用户确认）

候选两案：

**A. fail-fast，整个端点 500**（**本 spec 默认**）
- 优点：与契约"任何悬空引用 fatal 退出"对齐，让运营立刻看到所有问题；显式 > 隐式（用户偏好"显式化优先"）；游戏服端反正会 fatal，提前 500 比让运营误以为部署成功更安全
- 缺点：一个错引用阻塞所有 NPC 同步

**B. 跳过悬空 NPC + 200**
- 优点：其他正常 NPC 仍能下发
- 缺点：违反"显式化优先"；游戏服端拿到不完整列表可能出现"应该有但缺了"的诡异 bug；运营需主动查响应头才知道有 NPC 被跳过

**默认选 A**。如果用户坚持 B，需要先回答："运营漏看跳过列表导致游戏世界 NPC 缺失，谁负责？"
