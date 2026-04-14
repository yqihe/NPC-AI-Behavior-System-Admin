# util 架构分层重组 — Tasks

按依赖顺序拆解为 11 个原子任务。每任务 1-3 文件，单一产出。**每任务完成后必须 `/verify` 通过才能进下一任务**（auto_verify 规则）。

---

## T1：下沉 CheckConstraintTightened 到 service 层 (R4) [x]

**文件**（3）：
- `backend/internal/service/constraint_check.go`（新建）
- `backend/internal/service/field.go`（改调用点）
- `backend/internal/service/event_type_schema.go`（改调用点）

**做什么**：
1. 新建 `service/constraint_check.go`，`package service`，把 `util/constraint.go:289-371` 的 `CheckConstraintTightened` 函数**一字不改**搬过来（包括 doc comment + 内部对 `util.ParseConstraintsMap`/`GetFloat`/`GetString`/`ParseSelectOptions` 的调用——这些工具此刻还在 util/ 里，依赖成立）
2. `service/field.go:282`：`util.CheckConstraintTightened(...)` → `CheckConstraintTightened(...)`（drop 前缀，同包调用）
3. `service/event_type_schema.go:140`：同上

**做完了是什么样**：
- `grep -rn "util.CheckConstraintTightened" backend/` 结果为空
- `util/constraint.go` 仍定义 `CheckConstraintTightened`，但无人调用（orphan，T3 会删除）
- `go build ./...` 通过

---

## T2：util/handler.go 建立 + validation.go/strings.go 剥离 (R1, R2) [x]

**文件**（3）：
- `backend/internal/util/handler.go`（新建）
- `backend/internal/util/validation.go`（**删除**）
- `backend/internal/util/strings.go`（编辑：仅删除 `IdentPattern` 相关内容，保留 `EscapeLike`）

**做什么**：
1. 新建 `util/handler.go`，`package util`，内容：
   - 分节「ID / Version / 必填校验」：搬 `CheckID` / `CheckVersion` / `CheckRequired`（from validation.go）
   - 分节「成功响应辅助」：搬 `SuccessMsg`（from validation.go）
   - 分节「标识符正则」：搬 `IdentPattern`（from strings.go）
   - 分节「名称格式校验」：**新增** `CheckName(name string, maxLen int, errCode int, subject string) *errcode.Error`（见 design.md 签名）
   - 分节「标签格式校验」：**新增** `CheckLabel(label string, maxLen int, subject string) *errcode.Error`
   - import: `"regexp"`, `"unicode/utf8"`, `errcode` 包
2. 删除 `util/validation.go`
3. `util/strings.go`：只保留 `EscapeLike` + 其 import（`"strings"`），移除 `IdentPattern` 及 `"regexp"` import

**做完了是什么样**：
- `ls backend/internal/util/` 输出：`const.go constraint.go handler.go pagination.go strings.go`（老的 constraint/pagination/strings 尚存，T3/T4 清理）
- `grep -rn "util.CheckID\|util.CheckVersion\|util.CheckRequired\|util.IdentPattern\|util.SuccessMsg" backend/` 结果全部 hit `util/handler.go`，无调用方编译错
- `go build ./...` 通过

---

## T3：util/service.go 建立 + constraint.go/pagination.go 删除 (R1) [x]

**文件**（3）：
- `backend/internal/util/service.go`（新建）
- `backend/internal/util/constraint.go`（**删除**）
- `backend/internal/util/pagination.go`（**删除**）

**做什么**：
1. 新建 `util/service.go`，内容：
   - 分节「分页规范化」：搬 `NormalizePagination`（from pagination.go）
   - 分节「约束 JSON 解析」：搬 `ParseConstraintsMap` / `GetFloat` / `GetString` / `GetBool` / `ParseSelectOptions`（from constraint.go）
   - 分节「单值校验」：搬 `ValidateValue` + `validateInt/Float/String/Bool/Select`
   - 分节「约束自洽校验」：搬 `ValidateConstraintsSelf` + `selfCheckMinMax/LengthRange/Select`
   - 不搬 `CheckConstraintTightened`（T1 已下沉到 service 层）
   - import: `"encoding/json"`, `"fmt"`, `"unicode/utf8"`, errcode 包
2. 删除 `util/constraint.go`
3. 删除 `util/pagination.go`

**做完了是什么样**：
- `ls backend/internal/util/` 输出：`const.go handler.go service.go strings.go`
- `grep -rn "util.ValidateValue\|util.ValidateConstraintsSelf\|util.ParseConstraintsMap\|util.NormalizePagination" backend/` 所有调用点继续工作
- `grep -rn "util.CheckConstraintTightened" backend/` 仍为空
- `go build ./...` 通过

---

## T4：util/store.go 建立 + strings.go 删除 (R1) [x]

**文件**（2）：
- `backend/internal/util/store.go`（新建）
- `backend/internal/util/strings.go`（**删除**）

**做什么**：
1. 新建 `util/store.go`，内容：
   - 分节「SQL LIKE 转义」：搬 `EscapeLike`
   - import: `"strings"`
2. 删除 `util/strings.go`

**做完了是什么样**：
- `ls backend/internal/util/` 输出精确为：`const.go handler.go service.go store.go` ✓（R1 达成）
- `grep -rn "util.EscapeLike" backend/` 所有调用点继续工作
- `go build ./...` 通过

---

## T5：field + template handler 改用 util.CheckName / CheckLabel (R3)

**文件**（2）：
- `backend/internal/handler/field.go`
- `backend/internal/handler/template.go`

**做什么**：
1. `field.go`：
   - 删除私有方法 `checkName` / `checkLabel`（第 43-64 行附近）
   - 3 个调用点（Create `field.go:91-94`、Update `field.go:133`、CheckName RPC `field.go:172`）改为：
     - `util.CheckName(req.Name, h.valCfg.FieldNameMaxLength, errcode.ErrFieldNameInvalid, "字段标识")`
     - `util.CheckLabel(req.Label, h.valCfg.FieldLabelMaxLength, "中文标签")`
   - 清理不再用的 import `"unicode/utf8"`
2. `template.go`：
   - 删除私有方法 `checkName` / `checkLabel`
   - 调用点改为 `util.CheckName(..., TemplateNameMaxLength, ErrTemplateNameInvalid, "模板标识")` / `util.CheckLabel(..., FieldLabelMaxLength, "中文标签")`
   - 清理 `unicode/utf8` import（如仅此处使用）

**做完了是什么样**：
- `grep -n "func (h \*FieldHandler) check\|func (h \*TemplateHandler) check" backend/internal/handler/{field,template}.go` 无结果
- 业务错误码未变：非法 name 仍返回 `ErrFieldNameInvalid` / `ErrTemplateNameInvalid`
- `go build ./...` + 相关单测通过

---

## T6：event_type + fsm_config handler 改用 util.CheckName / CheckLabel (R3)

**文件**（2）：
- `backend/internal/handler/event_type.go`
- `backend/internal/handler/fsm_config.go`

**做什么**：
1. `event_type.go`：
   - 删除私有方法 `checkName`（第 38-49 行）/ `checkDisplayName`（第 51-59 行）
   - 调用点（Create `:111-114`、Update `:234`、CheckName RPC `:277`）改为：
     - `util.CheckName(req.Name, h.etCfg.NameMaxLength, errcode.ErrEventTypeNameInvalid, "事件标识")`
     - `util.CheckLabel(req.DisplayName, h.etCfg.DisplayNameMaxLength, "中文名称")`
   - 保留业务前置校验 `checkPerceptionMode` / `checkSeverity`（**不搬**，R3 边界）
   - 清理 `unicode/utf8` import（如仅此处使用）
2. `fsm_config.go`：
   - 删除私有方法 `checkName` / `checkDisplayName`
   - 调用点改为 `util.CheckName(..., NameMaxLength, ErrFsmConfigNameInvalid, "状态机标识")` / `util.CheckLabel(..., DisplayNameMaxLength, "中文名称")`
   - 清理 import

**做完了是什么样**：
- `grep -n "func (h \*EventTypeHandler) checkName\|func (h \*EventTypeHandler) checkDisplayName\|func (h \*FsmConfigHandler) checkName\|func (h \*FsmConfigHandler) checkDisplayName" backend/` 无结果
- `checkPerceptionMode` / `checkSeverity` 等业务校验仍在
- `go build ./...` + 相关单测通过

---

## T7：event_type_schema handler 改用 util.CheckName / CheckLabel (R3)

**文件**（1）：
- `backend/internal/handler/event_type_schema.go`

**做什么**：
1. 删除私有方法 `checkFieldName`（第 38-46 行）/ `checkLabel`（第 51-57 行）
2. 调用点（Create/Update 各处）改为：
   - `util.CheckName(req.FieldName, h.etsCfg.FieldNameMaxLength, errcode.ErrExtSchemaNameInvalid, "扩展字段标识")`
   - `util.CheckLabel(req.Label, h.etsCfg.FieldLabelMaxLength, "扩展字段中文名")`
3. 保留 `checkFieldType` 业务前置校验（不搬）
4. 清理 `unicode/utf8` import

**做完了是什么样**：
- `grep -rn "func (h \*.*Handler) check" backend/internal/handler/` 只剩业务前置校验（`checkSeverity` / `checkPerceptionMode` / `checkPropertiesShape` / `checkFieldType`）
- `grep -rn "util.CheckConstraintTightened" backend/` 仍为空
- `go build ./...` + 相关集成测试通过

---

## T8：util/handler_test.go 新增单测 (R6)

**文件**（1）：
- `backend/internal/util/handler_test.go`（新建）

**做什么**：
- `TestCheckName`：
  - case `empty` → `errCode` 透传 + "XXX不能为空"
  - case `BadFormat`（大写/数字开头/特殊字符 3 子 case）→ `errCode` + errcode 默认消息
  - case `TooLong`（len>maxLen）→ `errCode` + "XXX长度不能超过 N 个字符"
  - case `Valid` → nil
- `TestCheckLabel`：
  - case `empty` → `ErrBadRequest` + "XXX不能为空"
  - case `UTF8TooLong`（10 个中文 vs maxLen=9）→ `ErrBadRequest` + "XXX长度不能超过 N 个字符"（**关键**：验证 `utf8.RuneCountInString` 而非 byte len）
  - case `Valid` → nil

**做完了是什么样**：
- `go test ./backend/internal/util/` 通过，新增测试全绿
- 覆盖率工具显示 `CheckName` / `CheckLabel` 分支 100%

---

## T9：service/constraint_check_test.go 新增单测 (R6)

**文件**（1）：
- `backend/internal/service/constraint_check_test.go`（新建）

**做什么**：
- `TestCheckConstraintTightened`：
  - integer：`min 10→20`（收紧）→ 错；`min 20→10`（放宽）→ nil
  - integer：`max 100→50`（收紧）→ 错；`max 50→100`（放宽）→ nil
  - float：`precision 2→1`（降低）→ 错
  - string：`minLength 3→5` → 错；`maxLength 20→10` → 错；`pattern 变更` → 错
  - select：`options 删除 "foo"` → 错；`minSelect 收紧` → 错；`maxSelect 收紧` → 错
  - errCode 透传：用 `errcode.ErrFieldRefTighten` 测试时响应体 Code 一致

**做完了是什么样**：
- `go test ./backend/internal/service/` 通过
- 覆盖 integer/float/string/select 各类型的收紧/放宽路径

---

## T10：更新文档 CLAUDE.md + admin red-lines + admin dev-rules (R5)

**文件**（3）：
- `CLAUDE.md`
- `docs/development/admin/red-lines.md`
- `docs/development/admin/dev-rules.md`

**做什么**：
1. `CLAUDE.md` 目录结构段落（第 68 行附近）：将 `util/ # 通用工具（校验/分页/转义/常量）` 改为明确按层组织的说明：
   ```
   util/                      # 跨模块共享工具（按架构层分文件）
     handler.go               #   handler 层通用（ID/版本/必填/名称/标签校验、响应辅助）
     service.go               #   service 层通用（分页/约束解析/值校验）
     store.go                 #   store 层通用（SQL LIKE 转义）
     const.go                 #   跨层共享常量
   ```
2. `docs/development/admin/red-lines.md` §11（禁止文件职责混放）：
   - 新增一条：`util/ 按架构层分文件（handler.go / service.go / store.go / const.go），业务规则不进 util/——放对应 service/*_check.go 等`
   - 新增一条：`service/ 根目录只允许业务模块聚合文件和跨模块业务规则文件（命名必须带业务语义，禁止 helpers.go/common.go）`
3. `docs/development/admin/dev-rules.md`：
   - Handler 权威模式表格保留 `util.CheckID/CheckVersion/CheckRequired`，**新增** `util.CheckName` / `util.CheckLabel` 两行
   - §4b 引用 `util.CheckConstraintTightened` 的位置改为 `service.CheckConstraintTightened`（或"同包 `CheckConstraintTightened`"，视上下文）
   - 新增「service 根目录共享文件纪律」小节（design.md 中的原文）

**做完了是什么样**：
- CLAUDE.md 目录结构段与实际 util/ 文件一致
- red-lines.md §11 加入新条目
- dev-rules.md 的 `util.CheckConstraintTightened` 引用全部改为 service 层路径
- `grep -rn "util.CheckConstraintTightened" docs/` 无结果

---

## T11：全量回归验证 (R6, R7)

**文件**：无改动（仅运行验证命令）

**做什么**：
1. `go build ./...` → 通过
2. `go test ./...` → 全部绿（含 T8/T9 新增单测）
3. 集成测试（test/integration/ 下的 5 模块 e2e）全部通过
4. `npx vue-tsc --noEmit` → 前端无编译错（前端未改，但项目规则要求）
5. 硬门槛 grep：
   - `ls backend/internal/util/` → 精确为 `const.go handler.go service.go store.go`
   - `grep -rn "util.CheckConstraintTightened" backend/` → 空
   - `grep -rn "func (h \*.*Handler) check" backend/internal/handler/` → 只剩 `checkSeverity` / `checkPerceptionMode` / `checkPropertiesShape` / `checkFieldType`
6. 手动验证（以 curl 或已有集成测试脚本）：
   - `POST /api/v1/fields/check-name` with `{"name": "INVALID"}` → `code: ErrFieldNameInvalid`
   - `POST /api/v1/event-types/check-name` with `{"name": ""}` → `code: ErrEventTypeNameInvalid, message: "事件标识不能为空"`
   - 字段被引用后修改 min 收紧 → `code: ErrFieldRefTighten`

**做完了是什么样**：
- 所有验证步骤通过截图或命令输出记录
- 准备合并到 main，commit 消息按 `refactor(backend/util): 架构分层重组 + handler 校验统一 + 业务规则下沉` 格式

---

## 任务依赖图

```
T1 (service/constraint_check.go)  →  独立，最先做（util 还完整时搬业务规则）
      ↓
T2 (util/handler.go + 删 validation.go + 改 strings.go)
      ↓
T3 (util/service.go + 删 constraint.go + pagination.go)
      ↓
T4 (util/store.go + 删 strings.go)  ← R1 达成
      ↓
T5 (field + template handler)  ─┐
T6 (event_type + fsm_config)   ─┼─ 三任务可串行，handler 改造
T7 (event_type_schema)         ─┘
      ↓
T8 (util/handler_test.go)  ─┐
T9 (constraint_check_test) ─┴─ 两任务测试，可并行但建议串行便于 /verify
      ↓
T10 (文档三件)
      ↓
T11 (回归验证)  ← 最终门槛
```

**估算**：11 任务，每个含 `/verify` 约 10-20 分钟，总时 ≈ 3-4 小时。

---

## 验收总表

| R | 任务 |
|---|---|
| R1 | T2 + T3 + T4（util 结构最终达成） |
| R2 | T2（CheckName/CheckLabel 定义） |
| R3 | T5 + T6 + T7（handler 全部改造） |
| R4 | T1（CheckConstraintTightened 下沉） |
| R5 | T10（文档三件） |
| R6 | T8 + T9 + T11（测试门槛） |
| R7 | T11（可编译验证） |
