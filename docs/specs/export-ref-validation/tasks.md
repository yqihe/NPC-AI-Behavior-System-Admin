# 任务拆解：NPC 导出端点引用完整性校验

> 对应 [requirements.md](requirements.md) / [design.md](design.md)。
> 每个 task 完成后必须 `/verify` 通过（user memory `feedback_auto_verify.md`）。

## 任务依赖图

```
T1 (探索) ──┬─→ T5 (errcode 包归属定案后才能写)
           │
T2 (FSM helper) ──┐
T3 (errcode 码)   ├─→ T6 (validateExportRefs + ExportAll) ─→ T7 (handler) ─→ T8 (单测) ─→ T9 (e2e)
T4 (model 结构)   │                          ↑
T5 (typed error) ─┘                          │ 所有上游就绪后实施
```

并行机会：T2 / T3 / T4 三者互不依赖，可并行；T5 依赖 T1 结论 + T4 结构。

---

## T1：核实依赖方向，锁定 ExportDanglingRefError 归属包  `[x]` 完成 2026-04-18

**关联**：design §0、§5
**文件**：0 改动（只探索 + 在 design.md §5 落定结论）

**结论**：`grep -rn "internal/errcode" backend/internal/model/` 零匹配 → 方案 A 成立（errcode → model 单向）。`ExportDanglingRefError` 放 errcode/export_error.go（T5），`NPCExportDanglingRef` 放 model/npc.go（T4）。详见 design.md §5 修订。

**做什么**：
1. `grep -r "internal/errcode" backend/internal/model/` 确认 model 包是否 import errcode
2. 如果 **未 import**（预期）：方案不变，`ExportDanglingRefError` 放 [backend/internal/errcode/](backend/internal/errcode/)（新文件 `export_error.go`），其 `Details []model.NPCExportDanglingRef` 字段引用 model 包
3. 如果 **已 import**（反向依赖已存在）：把 `NPCExportDanglingRef` 移到 errcode 包内（更名 `ExportDanglingRefDetail`），T4 改为不在 model 加结构
4. 在 design.md §5「<待 grep>」处替换为最终结论 + 一句决策说明

**做完了是什么样**：
- design.md §5 不再含「<待 grep>」字样，明确写出方向和归属包
- 控制台输出 grep 结果（粘贴到 spec-execute 报告里）
- 后续 T4/T5 文件路径基于此结论确定

---

## T2：新增 FSM `CheckEnabledByNames` 跨层 helper（store + service）  `[x]` 完成 2026-04-18

**关联**：R1, R2
**文件**：
- [backend/internal/store/mysql/fsm_config.go](backend/internal/store/mysql/fsm_config.go) （+1 新方法）
- [backend/internal/service/fsm_config.go](backend/internal/service/fsm_config.go) （+1 新方法）

**做什么**：
1. **store 层**：新增 `(s *FsmConfigStore) GetEnabledByNames(ctx, names []string) (map[string]bool, error)`，**完全镜像** [store/mysql/bt_tree.go:195-220](backend/internal/store/mysql/bt_tree.go#L195) 的 `BtTreeStore.GetEnabledByNames`：
   - 空 names → 返回空 map，不发 SQL
   - 非空 → `sqlx.In + Rebind` + `SELECT name FROM fsm_configs WHERE name IN (?) AND enabled=1 AND deleted=0`
   - godoc 与 BT 版本同结构（用「FSM 配置」替换「行为树」）
2. **service 层**：新增 `(s *FsmConfigService) CheckEnabledByNames(ctx, names []string) (notOK []string, err error)`，镜像 [service/bt_tree.go:387-401](backend/internal/service/bt_tree.go#L387)：
   - 空 names → 直接 `nil, nil`
   - 调 `s.store.GetEnabledByNames`，错误用 `fmt.Errorf("get enabled fsm_configs by names: %w", err)` wrap
   - 遍历 names 比对 enabledSet，不在的塞进 notOK

**做完了是什么样**：
- `go build ./internal/...` 通过
- 两个方法的 godoc 字数与 BT 版本相差不超过 ±2 行（强制对齐）
- 在本地 MySQL 上手动 smoke：`SELECT name FROM fsm_configs WHERE name IN ('guard','nonexistent') AND enabled=1 AND deleted=0` 返回符合预期（启用的 1 行 + 不在的 0 行）
- 触发 `/verify`：跨模块一致性检查应通过（与 BT 同名 helper 一致）

---

## T3：新增错误码 `ErrNPCExportDanglingRef`（45016）+ message  `[x]` 完成 2026-04-18

**关联**：R3, R4
**文件**：[backend/internal/errcode/codes.go](backend/internal/errcode/codes.go)

**做什么**：
1. `grep -n "45016" backend/internal/errcode/codes.go` 确认 45016 未被占用（design 已确认 45001-45015 用尽）
2. NPC 段（45001-45015 之后）追加常量：
   ```go
   ErrNPCExportDanglingRef = 45016 // 导出 NPC 时发现悬空 FSM/BT 引用
   ```
3. messages map NPC 段追加：
   ```go
   ErrNPCExportDanglingRef: "NPC 导出失败：存在悬空的状态机/行为树引用，请按 details 修复",
   ```

**做完了是什么样**：
- `go build ./internal/...` 通过
- `grep "45016" backend/internal/errcode/codes.go` 命中且仅命中 1 次（常量定义）
- `grep "ErrNPCExportDanglingRef" backend/internal/errcode/codes.go` 命中 2 次（常量 + message）
- 调用 `errcode.Message(errcode.ErrNPCExportDanglingRef)` 返回上述中文

---

## T4：新增 `NPCExportDanglingRef` 结构 + 3 个常量  `[x]` 完成 2026-04-18

**关联**：R3, R4
**文件**：[backend/internal/model/npc.go](backend/internal/model/npc.go)
> 注：若 T1 结论是反向依赖存在，本 task 取消，结构搬到 T5。

**做什么**：
在 [model/npc.go](backend/internal/model/npc.go) 导出区段（NPCExportBehavior 之后）追加：

```go
// NPCExportDanglingRef 导出期发现的单条悬空引用（fsm_ref 或 bt_ref）
//
// Reason 当前实现统一为 ExportRefReasonMissingOrDisabled——
// 因 service.CheckEnabledByNames 仅返回"不在 enabled 集合"列表，
// 无法区分"不存在"和"存在但已禁用"。如未来 helper 增强可分别细化。
type NPCExportDanglingRef struct {
    NPCName  string `json:"npc_name"`
    RefType  string `json:"ref_type"`
    RefValue string `json:"ref_value"`
    Reason   string `json:"reason"`
    State    string `json:"state,omitempty"`
}

const (
    ExportRefTypeFsm                 = "fsm_ref"
    ExportRefTypeBt                  = "bt_ref"
    ExportRefReasonMissingOrDisabled = "missing_or_disabled"
)
```

**做完了是什么样**：
- `go build ./internal/...` 通过
- 结构序列化 `json.Marshal(NPCExportDanglingRef{NPCName:"a",RefType:"fsm_ref",RefValue:"b",Reason:"missing_or_disabled"})` 输出**不含** state 字段（`omitempty` 验证）；带 `State:"patrol"` 时含 state
- 3 个常量值与 design 一致，无拼写错误

---

## T5：新增 `ExportDanglingRefError` typed error  `[x]` 完成 2026-04-18

**关联**：R3
**文件**：按 T1 结论：
- T1 通过 → 新建 [backend/internal/errcode/export_error.go](backend/internal/errcode/export_error.go)
- T1 未通过 → 追加到 [backend/internal/errcode/error.go](backend/internal/errcode/error.go)，并把 T4 的结构一起搬过来

**做什么**：

```go
package errcode

import (
    "fmt"
    "github.com/yqihe/npc-ai-admin/backend/internal/model"
)

// ExportDanglingRefError NPC 导出期发现悬空引用的结构化错误
//
// handler 用 errors.As 提取 Details，渲染为 {code:45016, message, details:[...]} 的 5xx JSON。
type ExportDanglingRefError struct {
    Details []model.NPCExportDanglingRef
}

func (e *ExportDanglingRefError) Error() string {
    return fmt.Sprintf("npc export found %d dangling refs", len(e.Details))
}
```

**做完了是什么样**：
- `go build ./internal/...` 通过
- `var _ error = (*ExportDanglingRefError)(nil)` 编译通过（接口实现验证）
- `errors.As(&ExportDanglingRefError{Details: ...}, &dst)` 在 handler 测试里能解出 dst（手动 REPL/单测均可）

---

## T6：拆分 `NpcService.ExportAll` 为 4 个纯方法

**关联**：R1, R2, R5, R6
**文件**：[backend/internal/service/npc_service.go](backend/internal/service/npc_service.go)

> **2026-04-18 二次修订**：原 T6 让 service 直接调 fsm/bt service，违反 [npc_service.go:22-24](backend/internal/service/npc_service.go#L22) 明文「不持有跨服务依赖」硬约束。改为 handler 编排（T7）+ service 4 个纯方法（本任务）。详见 design §0、§1.4、§2.1。

**做什么**：
1. 新增类型（紧邻既有 ExportAll 上方）：
   ```go
   type NPCExportRefs struct {
       FsmIndex map[string][]string         // fsmName → npc names
       BtIndex  map[string][]NPCExportBtUsage  // btName → (npc, state) list
   }
   type NPCExportBtUsage struct {
       NPCName string
       State   string
   }
   ```
2. 新增 4 个方法（替代既有 `ExportAll`）：
   - `ExportRows(ctx) ([]model.NPC, error)` — 直查 store.ExportAll
   - `CollectExportRefs(rows) (*NPCExportRefs, error)` — 纯函数，扫 rows + json.Unmarshal bt_refs，构建反查索引；空 fsm_ref / 空 bt_refs 不入索引（合法的"无行为配置"）
   - `BuildExportDanglingError(refs, fsmNotOK, btNotOK) *errcode.ExportDanglingRefError` — 纯函数，遍历两个 notOK × 反查 index 拼 details；全部正常返 nil
   - `AssembleExportItems(rows) ([]model.NPCExportItem, error)` — 纯函数，抽自既有 ExportAll 的装配段
3. **删除** 既有 `ExportAll` 方法（调用方只有 export.go 一处，T7 切到新 4 步编排）
4. 注意：本任务**不动** NpcService struct（不新增字段，因为不持有任何跨服务依赖）

**做完了是什么样**：
- `go build ./internal/...` 通过
- grep 确认 `ExportAll` 只剩 1 个命中（store 层 `NpcStore.ExportAll`，service 层旧方法已删）
- 4 个新方法都是 receiver 纯方法（无 ctx 参数除 ExportRows）
- 触发 `/verify`：单元测起来更容易（输入 model.NPC 切片、输出确定结构），N+1 防护通过 T7 实施时验证

---

## T7：handler 5 步编排 + 修 5xx 格式

**关联**：R1, R2, R3, R4, R5, R6, R7
**文件**：[backend/internal/handler/export.go](backend/internal/handler/export.go)

> **2026-04-18 二次修订**：handler 接管跨模块编排（NpcService 不持有 fsm/bt service，遵守硬约束）。ExportHandler 已注入 fsmConfigService + btTreeService（[export.go:15](backend/internal/handler/export.go#L15)），无需改 setup。

**做什么**：
按 design §1.6 改写 `NPCTemplates` 函数为 5 步编排：

| Step | 调用 | 失败处理 |
|---|---|---|
| 1 | `npcService.ExportRows(ctx)` | 通用 500 |
| - | `len(rows)==0` 短路 | 200 + `{"items":[]}` |
| 2 | `npcService.CollectExportRefs(rows)` | 通用 500 |
| 3a | `fsmConfigService.CheckEnabledByNames(ctx, keysOf(refs.FsmIndex))` | 通用 500 |
| 3b | `btTreeService.CheckEnabledByNames(ctx, keysOf(refs.BtIndex))` | 通用 500 |
| 4 | `npcService.BuildExportDanglingError(refs, fsmNotOK, btNotOK)` | 非 nil → 5xx + `{code:45016, message, details}` + slog.Error 输出 details |
| 5 | `npcService.AssembleExportItems(rows)` | 通用 500 |
| - | success | 200 + `{"items": items}` |

通用 500 抽 `respondInternalErr(c, stage, err)` 辅助，含 stage 标签（`"export_rows"` / `"collect_refs"` / `"check_fsm"` / `"check_bt"` / `"assemble"`）。

**不动**其他三个 export handler（EventTypes / FsmConfigs / BTTrees）。

**做完了是什么样**：
- `go build ./internal/...` 通过
- diff 显示只动了 `NPCTemplates` + 新增 `respondInternalErr`，其他 handler 函数零变更
- handler 不依赖 `errors.As`（因为 BuildExportDanglingError 直接返指针）
- 用 `errcode.Msg(...)` 而非 `errcode.Message(...)`（design 原 §1.6 笔误已订正）
- 触发 `/verify`：跨模块一致性 + 5xx 格式红线 #14

---

---

## T8：单元测试覆盖 4 个纯方法

**关联**：R8
**文件**：[backend/internal/service/npc_service_test.go](backend/internal/service/npc_service_test.go)（如不存在则新建；如存在则追加）

> **2026-04-18 二次修订**：T6 拆分后测试目标从 1 个 hybrid 方法变成 4 个纯方法，**不再需要 mock fsm/bt service**（service 不持有它们）。store mock 仅 `ExportRows` 测试需要。

**做什么**：

3 个纯函数（无 ctx，纯输入输出）直接 table-driven test，零 mock：

| 用例 | 测对象 | 数据 | 期望 |
|---|---|---|---|
| TestCollectExportRefs_Empty | CollectExportRefs | rows=[] | refs 两个 index 都是空 map（非 nil） |
| TestCollectExportRefs_AllRefs | 同 | 3 NPC: A 引 fsm=g + bt={patrol→p1}; B 引 fsm=g + bt={patrol→p1, alert→a1}; C 无 fsm 仅 bt={idle→i1} | FsmIndex={g:[A,B]}; BtIndex={p1:[(A,patrol),(B,patrol)], a1:[(B,alert)], i1:[(C,idle)]} |
| TestCollectExportRefs_BadJSON | 同 | 1 NPC bt_refs="not json" | 返 error |
| TestBuildExportDanglingError_AllValid | BuildExportDanglingError | refs 非空，fsmNotOK=[]，btNotOK=[] | nil |
| TestBuildExportDanglingError_FsmMissing | 同 | FsmIndex={g:[A,B]}, fsmNotOK=[g] | details=2 条，都 ref_type=fsm_ref + ref_value=g + npc_name 各为 A/B |
| TestBuildExportDanglingError_BtMissing | 同 | BtIndex={p1:[(A,patrol),(B,patrol)]}, btNotOK=[p1] | details=2 条，都 ref_type=bt_ref + state=patrol |
| TestBuildExportDanglingError_FsmAndBt | 同 | 同时 fsmNotOK + btNotOK | details=多条，FSM 在前 BT 在后 |
| TestAssembleExportItems_Empty | AssembleExportItems | rows=[] | items=[] (非 nil) |
| TestAssembleExportItems_OneRow | 同 | 1 NPC 含 fields/fsm_ref/bt_refs | items 长度=1，与既有 ExportAll 装配语义一致 |

ExportRows 是 store passthrough，不单独测（store 层覆盖；e2e 在 T9 验证）。

**做完了是什么样**：
- `go test ./internal/service/...` 全绿
- 9 个用例全部 PASS
- 不引入新 mock 框架（admin red-line "禁止引入没有使用场景的依赖"）
- 测试文件无未使用变量、无空断言
- 触发 `/verify`：测试质量红线应通过

> **N+1 防护和"3 SQL 总数"** 通过 T9 e2e 验证（用 SQL 慢查日志或 EXPLAIN），不在单测范围。

---

## T9：e2e 手动验证

**关联**：R3, R6, R7
**文件**：0 改动（验证清单）

**做什么**：
按 design §8.2 跑 4 步 curl，结果贴到 spec-execute 完成报告或 PR description：

1. **正常路径**：seed 1 NPC 引用启用的 FSM+BT → `curl /api/configs/npc_templates` → 期望 200 + `items` 长度=1，结构符合契约 §3
2. **悬空路径**：禁用一个被引用的 BT → 同 curl → 期望 **HTTP 500** + body `{code:45016, message, details:[{npc_name,ref_type:bt_ref,ref_value,reason:missing_or_disabled,state}]}`
3. **审计/日志**：步骤 2 后查后端 stdout，确认 slog ERROR 一条 `handler.export.npc_templates.dangling_refs` 含 details
4. **隔离性**：在步骤 2 状态下分别 curl `/api/configs/event_types`、`/api/configs/fsm_configs`、`/api/configs/bt_trees` → 期望 3 个端点全部 200（不受 NPC 端点影响）

**做完了是什么样**：
- 4 步 curl 输出全部贴到完成报告（含 HTTP code + body）
- 步骤 2 的响应 body 通过 jq 校验 `.code == 45016 and (.details | length) > 0`
- 步骤 4 三个端点都返回 200
- 触发最终 `/verify` 完成 spec 收尾

---

## 不做的（再次显式确认）

- regions 端点
- FSM condition Key 注册校验 / BT 节点 schema 校验 / NPC fields 跨模板类型一致性
- fsm_configs / bt_trees 端点的导出期校验（pattern 已沉淀，未来 spec 复用）
- 任何前端改动
- 引入测试 mock 框架（mockery / gomock）—— 用接口注入 stub 即可
- 自动化 e2e 进 CI（暂手动验证）

## 经验沉淀（spec 完成后追加，不在任务范围）

见 design §9 候选清单。spec-execute 收尾后另起一个轻量 PR 处理。
