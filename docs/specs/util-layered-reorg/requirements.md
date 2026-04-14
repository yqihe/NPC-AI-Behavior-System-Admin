# util 架构分层重组 — Requirements

## 动机

企业级代码审查发现两个相关问题：

1. **util 包缺乏组织原则**：当前按"功能点"分文件（`const.go` / `constraint.go` / `pagination.go` / `strings.go` / `validation.go`）。读者拿到一个工具函数无法立刻判断"谁在用、该放哪里"。`pagination.go` 里的 `NormalizePagination` 实际是 service 层用；`strings.go` 里的 `EscapeLike` 是 store 层用；`validation.go` 里的 `CheckID/CheckVersion` 是 handler 层用——但分布散乱。
2. **handler 重复格式校验**：`field.go` / `event_type.go` / `fsm_config.go` 三个 handler 各自手写 `checkName` / `checkLabel` / `checkDisplayName`（结构几乎相同——非空 + 正则/长度校验 + 错误码），当前已有 3 处重复，后续 `template` / `event_type_schema` / `region` 等模块加入还会继续抄。

不做会怎样：
- 新模块开发继续抄手写校验，偏离风险扩大（`EventType` 模块曾因最后开发偏离，见项目记忆）
- util 组织随功能增加继续膨胀，读者需要全文扫才能定位工具归属
- `CheckConstraintTightened` 作为明显的跨模块业务规则（"约束只能放宽"）放在 util/ 里，破坏了"util 无业务"的分层原则

## 优先级

**中等偏高**。非功能性重构，不阻塞 FSM 联调。但：
- 所有新模块（BT / 区域 / NPC）都会走 handler 三段校验，继续抄会把坏样板固化
- util 分层规则一旦敲定，后续所有模块遵循；拖得越久，迁移成本越大

## 预期效果

改完后：

**util 组织**（`backend/internal/util/`）：
- 只剩 4 个文件：`handler.go` / `service.go` / `store.go` / `const.go`
- 每个文件内按功能分节（Go 风格大区块注释 `// ==========`），读者 IDE 大纲能直接看到分节
- CLAUDE.md 和 admin 的 red-lines/dev-rules 明确"util 按层分文件 + 业务规则不进 util"

**handler 校验**：
- 所有 handler 的名称类前置校验调 `util.CheckIdentName(name, maxLen, errCode)` / `util.CheckDisplayName(label, maxLen, errCode, fieldLabel)` 之类的统一函数
- 各 handler 不再有 `checkName` / `checkLabel` / `checkDisplayName` 私有方法

**业务规则下沉**：
- `CheckConstraintTightened`（约束收紧检查）从 util/ 搬出，归位到 service 层（具体放法见 design 阶段讨论）

## 依赖分析

**依赖**：
- 字段管理（`service/field.go`）、扩展字段 Schema（`service/event_type_schema.go`）已完成，是 `CheckConstraintTightened` 的两个唯一调用点
- handler 三段校验的现有模块（field / event_type / fsm_config / template / event_type_schema）已完成

**谁依赖**：
- 后续模块（BT / 区域 / NPC）直接受益于统一校验入口
- `handler 禁止文件职责混放`（admin red-lines §11）的条目会因此更清晰

## 改动范围

**Backend**：
- `backend/internal/util/` — 全盘重组（拆解 5 文件 → 4 文件）
- `backend/internal/handler/` — 5 个 handler 的 `checkName/checkLabel/checkDisplayName` 调用点替换（field / event_type / fsm_config / template / event_type_schema）
- `backend/internal/service/` — `CheckConstraintTightened` 归位点（最多新增 1 文件 `service/constraint_check.go` 或复用）

**Docs**：
- `CLAUDE.md` — util 目录规则描述更新
- `docs/development/admin/red-lines.md` — §11 文件职责混放新增 util 分层规则 + service 根目录纪律
- `docs/development/admin/dev-rules.md` — util 分层实例 + `CheckConstraintTightened` 新路径 + service 根目录共享文件规则

**预估文件数**：util 4 文件重写 + 5 handler 改动 + 1 service 新增或改动 + 3 文档更新 ≈ **13 文件**

## 扩展轴检查

项目两个扩展方向：
1. **新增配置类型**：只需加一组 handler/service/store/validator
2. **新增表单字段**：只需加组件

本 spec **正面影响扩展轴 1**：统一 handler 校验入口后，新配置类型的 handler 不再需要抄 `checkName/checkLabel` 样板，直接调 util。util 分层清晰后，开发者知道通用函数该归在哪个文件里（handler.go / service.go / store.go），不会随机往 util 里塞。

不涉及扩展轴 2（前端组件）。

## 验收标准

- **R1**：`backend/internal/util/` 只包含 4 个 Go 文件：`handler.go`、`service.go`、`store.go`、`const.go`。不存在 `constraint.go` / `pagination.go` / `strings.go` / `validation.go`。
- **R2**：`util/handler.go` 导出统一的名称/标签格式校验函数（函数签名在 design 阶段敲定），覆盖原 `field.go` / `event_type.go` / `fsm_config.go` 的 `checkName/checkLabel/checkDisplayName` 的全部语义（非空、正则 `IdentPattern`、byte 长度上限、UTF-8 rune 长度上限）。
- **R3**：5 个 handler 模块（field / event_type / fsm_config / template / event_type_schema）不再定义或调用私有 `checkName/checkLabel/checkDisplayName`；全部改调 util 统一函数。**范围仅限名称类格式校验**；模块专属业务前置校验（`checkSeverity` / `checkPerceptionMode` / `checkPropertiesShape` 等）**不搬**，留在各 handler 内。
- **R4**：`CheckConstraintTightened` 迁移到 `backend/internal/service/constraint_check.go`（service 层根目录新文件）。字段 service 和扩展字段 Schema service 的两个原调用点改为直接调用同包函数（无 import 变化或仅 drop `util` 前缀）。`grep -rn "util.CheckConstraintTightened" backend/` 结果为空。
- **R5**：`CLAUDE.md` 目录结构段落、`docs/development/admin/red-lines.md` §11、`docs/development/admin/dev-rules.md` util 权威模式表格全部同步更新到新分层规则。
- **R6**：`go build ./...` 通过；所有现有单测 + 集成测试通过；`npx vue-tsc --noEmit` 通过（前端无改动，但项目规则要求任何 commit 前跑一遍）。
- **R7**：新工作流可重复验证——新建一个最小 handler demo（仅用于本地验证，不 commit），调 `util.CheckXxx` 三函数能编译通过。

## 不做什么

- **不重写 util 函数的内部实现**——只搬运 + 统一签名。`ValidateValue` / `ValidateConstraintsSelf` / `ParseConstraintsMap` 等函数代码**一行不改**，只换家。
- **不改 util 函数的大小写命名**——`CheckID`、`CheckVersion`、`IdentPattern`、`EscapeLike`、`NormalizePagination` 等导出名保持不变，避免打爆所有调用点。
- **不引入泛型缓存读穿模板**（`ReadThrough[T]`）——那是另一个独立优化话题，不纳入本 spec。
- **不合并 errcode**——各模块的 `ErrFieldNameInvalid` / `ErrEventTypeNameInvalid` / `ErrFsmConfigNameInvalid` 保留；统一函数通过参数接收 errCode。
- **不处理 B 组小硬化**（前端 404 / Password 脱敏 / reactive→ref）——走独立流程。
- **不改前端任何代码**。
