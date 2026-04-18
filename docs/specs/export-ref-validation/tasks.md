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

## T2：新增 FSM `CheckEnabledByNames` 跨层 helper（store + service）

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

## T3：新增错误码 `ErrNPCExportDanglingRef`（45016）+ message

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

## T4：新增 `NPCExportDanglingRef` 结构 + 3 个常量

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

## T5：新增 `ExportDanglingRefError` typed error

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

## T6：实现 `NpcService.validateExportRefs` + 集成到 `ExportAll`

**关联**：R1, R2, R5, R6
**文件**：[backend/internal/service/npc_service.go](backend/internal/service/npc_service.go)

**做什么**：
1. 新增私有方法（紧邻既有 `ExportAll`）：
   ```go
   // validateExportRefs 批量复核所有 NPC 的 FSM/BT 引用是否存在且 enabled
   //
   // 全部正常返回 nil。任一悬空返回 *errcode.ExportDanglingRefError，details 含全部悬空条目。
   // SQL 恒为 2 次（FSM 1 次 + BT 1 次批量），与 NPC 数量无关。
   func (s *NpcService) validateExportRefs(ctx context.Context, rows []model.NPC) (*errcode.ExportDanglingRefError, error) { ... }
   ```
2. 实现策略：
   - 第一遍扫 rows：聚合 `fsmSet := map[string]bool` (fsm_ref 非空) 和 `btMap := map[string]string` (bt name → 它来自哪个 state，仅取最近一条用于 details 反查)
   - 实际反查 details 时需要"哪些 NPC 用了这个名"，所以 btMap 应是 `map[btName][]struct{NPC, State}`，fsmSet 类似 `map[fsmName][]string{NPC names}`
   - 调 `fsmConfigService.CheckEnabledByNames(ctx, fsmNames)` 拿 notOK
   - 调 `btTreeService.CheckEnabledByNames(ctx, btNames)` 拿 notOK
   - 把 notOK × 反查 map 展开成 `[]NPCExportDanglingRef`
   - 任意非空 → 返回 `*ExportDanglingRefError`
   - infra 错误（SQL 失败等）走第二个返回值
3. 修改 [npc_service.go:591](backend/internal/service/npc_service.go#L591) `ExportAll`：
   - `store.ExportAll` 后插入 `if dangling, err := s.validateExportRefs(ctx, rows); err != nil { return nil, err } else if dangling != nil { return nil, dangling }`
   - 后续 assembleExportItem 循环不变

**做完了是什么样**：
- `go build ./internal/...` 通过
- 调用 `npcService.ExportAll(ctx)` 在 0 NPC 时不发起 FSM/BT 任何 SQL（短路）
- 1 个 NPC 引用 1 个 FSM + 2 个 BT 时：FSM helper 收到 names=[fsmRef]，BT helper 收到 names=[bt1,bt2]（去重后）
- 5 个 NPC 全引用同一个 FSM 时：FSM helper 收到 names 长度=1（验证去重）
- 触发 `/verify`：跨模块一致性 + N+1 防护检查

---

## T7：修改 `ExportHandler.NPCTemplates` 处理 typed error + 修 5xx 格式

**关联**：R3, R4, R5, R6, R7
**文件**：[backend/internal/handler/export.go](backend/internal/handler/export.go)

**做什么**：
按 design §1.6 改写 `NPCTemplates` 函数：
- `errors.As(err, &dangling)` 命中 → 5xx + `gin.H{"code":45016, "message":..., "details": dangling.Details}` + slog.Error 输出 details
- 通用错误 → 5xx + `gin.H{"code":errcode.ErrInternal, "message":"导出失败，请查看服务端日志"}`，**不再返回 `{"items":[]}`**（修既有 admin red-line #14 违规）
- 200 + items 路径不变
- 200 + empty 路径不变（`{"items":[]}`）
- 不动其他三个 export handler（EventTypes / FsmConfigs / BTTrees）

**做完了是什么样**：
- `go build ./internal/...` 通过
- 用 panic + middleware 模拟通用错误：响应 body 是 `{"code":<ErrInternal>, "message":"导出失败..."}`，**不含** `items` 字段
- 触发 typed error：响应 body 含 `code=45016` + `message` + `details` 数组（手动 mock service 注入悬空 NPC 即可）
- 其他三个端点 handler 行为完全不变（diff 显示只动了 NPCTemplates 函数体）
- 触发 `/verify`：HTTP 响应格式红线 #14 应通过

---

## T8：单元测试 6 用例

**关联**：R8
**文件**：[backend/internal/service/npc_service_test.go](backend/internal/service/npc_service_test.go)（如不存在则新建；如存在则追加）

**做什么**：
1. 先看 npc_service 当前是否有测试文件 + mock 模式（`grep -l "MockFsmConfigService\|interface.*Fsm" backend/internal/`）。
2. 如无 mock 框架：用最简方式——给 NpcService 注入接口（FsmRefChecker / BtRefChecker），测试时传 stub 实现。**不引入 mockery / gomock 等新依赖**（admin red-line "禁止引入没有使用场景的依赖"）。
3. 写以下 6 用例：

| 用例 | 数据 | 期望 |
|---|---|---|
| TestExportAll_AllValid | 1 NPC, FSM/BT 都 enabled | items 长度=1，无错误 |
| TestExportAll_FsmMissing | NPC.fsm_ref 不在 enabledSet | err is `*ExportDanglingRefError`, details 长度=1, ref_type=fsm_ref |
| TestExportAll_FsmDisabled | 同上（CheckEnabledByNames 视角分不出，复用 Missing 测试逻辑即可） | 与 FsmMissing 同 |
| TestExportAll_BtMissing | NPC.bt_refs[patrol]=不存在的 BT | details[0].ref_type=bt_ref, state=patrol |
| TestExportAll_BtDisabled | 同上语义 | 与 BtMissing 同 |
| TestExportAll_BatchSqlCount | mock store + checker，10 NPC 引用 5 FSM 3 BT | FsmRefChecker.CheckEnabledByNames 被调 1 次, names 长度=5；BtRefChecker 被调 1 次, names 长度=3（去重） |

**做完了是什么样**：
- `go test -race ./internal/service/...` 全绿
- 6 个用例全部 PASS（输出 `ok backend/internal/service`）
- 测试文件无未使用变量、无空断言（red-line "禁止测试质量低下"）
- 触发 `/verify`：测试质量红线应通过

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
