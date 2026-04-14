# util 架构分层重组 — Design

## 方案描述

### 1. util/ 新文件结构

拆现有 5 文件到按架构层划分的 4 文件。**每个函数位置按"主调用层"确定**（多层都调时按最主要的 caller 层）。

```
backend/internal/util/
  const.go     # 跨层共享常量（保留，内容不变）
  handler.go   # handler 层通用：ID/Version/必填校验、标识符正则、名称/标签格式校验、响应辅助
  service.go   # service 层通用：分页规范化、约束 JSON 解析、值/约束自洽校验
  store.go     # store 层通用：SQL LIKE 转义
```

**文件内分节约定**（Go 风格，IDE 大纲友好）：

```go
// ============================================================
// 分节标题（2-4 字概括）
// ============================================================
```

### 2. 函数归位表

| 当前位置 | 函数/常量 | 归位 | 调用点数 |
|---|---|---|---|
| `validation.go` | `CheckID` / `CheckVersion` / `CheckRequired` / `SuccessMsg` | `handler.go` | handler 各处 |
| `strings.go` | `IdentPattern` | `handler.go` | handler 名称校验 |
| `strings.go` | `EscapeLike` | `store.go` | mysql 各模糊查询 |
| `pagination.go` | `NormalizePagination` | `service.go` | service 各 List |
| `constraint.go` | `ParseConstraintsMap` / `GetFloat` / `GetString` / `GetBool` / `ParseSelectOptions` | `service.go` | service 层消费 |
| `constraint.go` | `ValidateValue` / `validateInt/Float/String/Bool/Select` | `service.go` | service 层消费 |
| `constraint.go` | `ValidateConstraintsSelf` / `selfCheck*` | `service.go` | service 层消费 |
| `constraint.go` | **`CheckConstraintTightened`** | **`service/constraint_check.go`** | service 层业务规则 |
| `const.go` | 所有枚举常量 | `const.go`（不动） | 跨层 |

### 3. handler 统一校验函数

新增到 `util/handler.go`：

```go
// CheckName 校验标识符名称（小写+数字+下划线，a-z 开头，有长度上限）
//
// 所有配置类型的 name/field_name 共用。subject 用于错误消息（"字段标识"/"模板标识"/...）。
// errCode 由调用方传入（各模块独立：ErrFieldNameInvalid / ErrTemplateNameInvalid 等）。
func CheckName(name string, maxLen int, errCode int, subject string) *errcode.Error {
    if name == "" {
        return errcode.Newf(errCode, "%s不能为空", subject)
    }
    if !IdentPattern.MatchString(name) {
        return errcode.New(errCode) // 走 errcode 默认消息
    }
    if len(name) > maxLen {
        return errcode.Newf(errCode, "%s长度不能超过 %d 个字符", subject, maxLen)
    }
    return nil
}

// CheckLabel 校验中文标签/展示名（非空 + UTF-8 字符数上限）
//
// 所有配置类型的 label / display_name 共用。subject 是字段描述（"中文标签"/"中文名称"/"扩展字段中文名"）。
// 统一返回 ErrBadRequest（所有模块当前都用此码，符合 admin red-lines §4.8）。
func CheckLabel(label string, maxLen int, subject string) *errcode.Error {
    if label == "" {
        return errcode.Newf(errcode.ErrBadRequest, "%s不能为空", subject)
    }
    if utf8.RuneCountInString(label) > maxLen {
        return errcode.Newf(errcode.ErrBadRequest, "%s长度不能超过 %d 个字符", subject, maxLen)
    }
    return nil
}
```

### 4. handler 改造示例（field）

**Before**（`handler/field.go:43-64`）：
```go
func (h *FieldHandler) checkName(name string) *errcode.Error {
    if name == "" { return errcode.Newf(errcode.ErrFieldNameInvalid, "字段标识不能为空") }
    if !util.IdentPattern.MatchString(name) { return errcode.New(errcode.ErrFieldNameInvalid) }
    if len(name) > h.valCfg.FieldNameMaxLength {
        return errcode.Newf(errcode.ErrFieldNameInvalid, "字段标识长度不能超过 %d 个字符", h.valCfg.FieldNameMaxLength)
    }
    return nil
}
func (h *FieldHandler) checkLabel(label string) *errcode.Error { /* 类似 */ }
```

**After**（调用点内联，私有方法删除）：
```go
// field.go:91
if err := util.CheckName(req.Name, h.valCfg.FieldNameMaxLength, errcode.ErrFieldNameInvalid, "字段标识"); err != nil {
    return nil, err
}
if err := util.CheckLabel(req.Label, h.valCfg.FieldLabelMaxLength, "中文标签"); err != nil {
    return nil, err
}
```

**5 个 handler 改造映射**：

| handler | name 调用参数 | label/displayName 调用参数 |
|---|---|---|
| `field.go` | `FieldNameMaxLength, ErrFieldNameInvalid, "字段标识"` | `FieldLabelMaxLength, "中文标签"` |
| `template.go` | `TemplateNameMaxLength, ErrTemplateNameInvalid, "模板标识"` | `FieldLabelMaxLength, "中文标签"` |
| `event_type.go` | `NameMaxLength, ErrEventTypeNameInvalid, "事件标识"` | `DisplayNameMaxLength, "中文名称"` |
| `fsm_config.go` | `NameMaxLength, ErrFsmConfigNameInvalid, "状态机标识"` | `DisplayNameMaxLength, "中文名称"` |
| `event_type_schema.go` | `FieldNameMaxLength, ErrExtSchemaNameInvalid, "扩展字段标识"` | `FieldLabelMaxLength, "扩展字段中文名"` |

### 5. CheckConstraintTightened 迁移

**新文件**：`backend/internal/service/constraint_check.go`

```go
package service

import (
    "encoding/json"

    "github.com/yqihe/npc-ai-admin/backend/internal/errcode"
    "github.com/yqihe/npc-ai-admin/backend/internal/util"
)

// CheckConstraintTightened 检查约束是否被收紧
//
// 业务规则：被引用的字段/扩展字段编辑时，约束只能放宽不能收紧。
// 此函数是跨模块业务规则（字段 + 扩展字段 Schema 共用），放 service 层根目录。
// errCode 由调用方传入（字段模块用 ErrFieldRefTighten，扩展字段用 ErrExtSchemaRefTighten）。
//
// 实现复用 util.ParseConstraintsMap / GetFloat / GetString / ParseSelectOptions（这些是纯 JSON 解析工具）。
func CheckConstraintTightened(fieldType string, oldConstraints, newConstraints json.RawMessage, errCode int) *errcode.Error {
    // ... 70 行原封不动从 util/constraint.go:297-371 搬过来
}
```

**调用点改动**：

- `service/field.go:282`：`util.CheckConstraintTightened(...)` → `CheckConstraintTightened(...)`（同包调用，drop 前缀）
- `service/event_type_schema.go:140`：同上

---

## 方案对比

### 备选方案 A：保留功能点分文件（现状）

**优点**：零改动，零风险  
**缺点**：
1. 读者拿到工具函数无法从**文件名**判断归属层，必须 grep 调用点
2. 新工具函数加入时没有归位规则（按名字？按主题？）容易随意塞
3. 跨层工具和业务规则混放（`constraint.go` 里既有纯解析也有业务规则）——本次审查的触发问题

**不选理由**：不解决本次审查的核心问题。

### 备选方案 B：每层下建子包（`handler/util/`、`service/util/`、`store/util/`）

**优点**：物理隔离最强，import 路径一眼识别层  
**缺点**：
1. 违反 admin red-lines §11.6「每层文件夹下不允许子文件夹」
2. 增加 3 个包，调用点需要 `handlerutil` / `serviceutil` 之类别名解决冲突
3. `ParseConstraintsMap` 这类被多层可能都用的函数不知道该放哪层
4. 与用户「所有工具包集中在 util/ 下」的明确要求冲突

**不选理由**：违反红线 + 用户要求。

### 备选方案 C：保留 CheckConstraintTightened 在 util/service.go

**优点**：少一个文件，util 更"完整"  
**缺点**：
1. 破坏"util 无业务"的规则（这条规则本次重构要立起来）
2. 后续类似的跨模块业务规则没有归属地，会继续塞进 util 让它退化
3. 审查直接指出的问题（#14）原样保留

**不选理由**：与重构意图矛盾。

**最终选定**：正文方案（util 4 文件 + CheckConstraintTightened 下沉 service）。

---

## 红线检查

| 红线 | 条目 | 方案是否违反 | 说明 |
|---|---|---|---|
| standards/general.md | 禁止过度设计 | ❌ 不违反 | 新增函数有 5 个 handler 调用点（3+ 场景），不是为单点抽象 |
| standards/general.md | 禁止信息泄漏 | ❌ 不违反 | 错误消息均为业务中文，不含内部细节 |
| standards/go.md | 禁止字符串长度计算错误 | ❌ 不违反 | `CheckLabel` 用 `utf8.RuneCountInString`，`CheckName` 用 `len`（ASCII-only）✓ |
| standards/go.md | 禁止硬编码魔术字符串 | ❌ 不违反 | errCode 参数化，subject 作为参数传入（不是字面量） |
| standards/go.md | 禁止分层倒置 | ❌ 不违反 | util 不反依赖 store/cache；service/constraint_check.go 依赖 util（单向下行） |
| standards/mysql.md | — | ✅ 无关 | 本次无 SQL 改动 |
| standards/redis.md | — | ✅ 无关 | 本次无 Redis 改动 |
| standards/cache.md | — | ✅ 无关 | 本次无缓存改动 |
| standards/frontend.md | — | ✅ 无关 | 本次不动前端 |
| admin/red-lines.md §4.8 | name 校验用 ErrXxxNameInvalid 其他用 ErrBadRequest | ❌ 不违反 | `CheckIdentName` 收 errCode 参数，各模块传入 `ErrXxxNameInvalid`；`CheckDisplayLabel` 硬编码 `ErrBadRequest` ✓ |
| admin/red-lines.md §4b.5 | check-name 先走 handler 内部格式校验再查 DB | ❌ 不违反 | 迁移后 check-name handler 依旧先调 `util.CheckName` 再调 service；语义零变化 |
| admin/red-lines.md §10.6 | handler 用 `util.CheckID/CheckVersion/CheckRequired` | ❌ 不违反 | 函数名保留，只换家到 `util/handler.go` |
| admin/red-lines.md §11.1 | 共享常量/工具函数 → util/ | ✅ 强化 | 本重构就是落实这条 |
| admin/red-lines.md §11.6 | 每层文件夹下不允许子文件夹 | ❌ 不违反 | `service/constraint_check.go` 是 service 根的**文件**，不是子文件夹 |

**新增 1 条纪律**（本 spec 顺便立）：

> **service 根目录共享文件纪律**（`dev-rules.md` 新增）：  
> service/ 根目录只允许：  
> (1) 各业务模块的聚合文件（`field.go` / `event_type.go` 等）  
> (2) 被 2 个及以上 service 模块调用的**业务规则共享文件**，命名**必须带业务语义**（如 `constraint_check.go`，禁止 `helpers.go` / `common.go` 这类泛化名）

---

## 扩展性影响

**正面影响扩展轴 1（新增配置类型）**：

- 新配置类型的 handler 无需抄 `checkName/checkLabel/checkDisplayName` 样板（不会再出现"EventType 模块偏离"这种问题）
- 新开发者读 `util/handler.go` 就能理解 handler 层应该调哪些通用校验，不用 grep 多个文件
- 新加跨模块业务规则（如"BB Key 引用完整性"）时，有明确落脚点 `service/xxx_check.go`

**不影响扩展轴 2（新增表单字段）**：本次不动前端。

---

## 依赖方向

```
handler ──▶ service ──▶ store
   │           │           │
   └───────────┴───────────┴──▶ util （单向向下）
                   │
                   └──▶ service/constraint_check.go（service 同层调用，非依赖）
```

- util/ 不依赖任何业务层 ✓
- service/constraint_check.go 依赖 util（向下）✓
- 各 service 模块调用 service/constraint_check.go（同层调用，Go 同包直接调函数）✓
- 无环 ✓

---

## 陷阱检查

查阅相关 `docs/development/standards/dev-rules/`：

**go.md 陷阱**：
- ✅ 函数签名保留 `*errcode.Error` 返回类型，不返回 typed nil
- ✅ `utf8.RuneCountInString` 正确使用
- ✅ 不新增 goroutine / channel / 全局状态，无并发陷阱
- ✅ 不涉及 JSON tag，序列化陷阱不适用

**其他**：mysql/redis/cache/mongodb/frontend 均无改动，dev-rules 不触发。

---

## 配置变更

**无**。本次重构不新增/修改任何 JSON 配置文件。`config.yaml` 中 `FieldNameMaxLength` / `TemplateNameMaxLength` 等字段保留，只是读取它们的代码从 handler 搬到 util 函数参数传入。

---

## 测试策略

### 单元测试

**新增 `backend/internal/util/handler_test.go`**：
- `TestCheckName`：覆盖空串 / 非法正则（大写/数字开头/特殊字符）/ 超长 / 合法。验证错误码透传正确（用 `errcode.ErrFieldNameInvalid` 等）
- `TestCheckLabel`：覆盖空串 / UTF-8 中文超长（含 10 个中文 vs `maxLen=9`）/ 合法。验证返回的是 `ErrBadRequest`

**新增 `backend/internal/service/constraint_check_test.go`**：
- 把原 `util/constraint_test.go`（如存在）的 `CheckConstraintTightened` 部分迁移过来
- 覆盖 integer/float/string/select 每种类型收紧各 1 case + 放宽各 1 case

**原有测试**：
- `util/constraint_test.go`（如存在）保留非 tightened 部分；文件名改为 `util/service_test.go`
- 其他 util 函数已有测试全部随搬迁保留

### 集成测试

**回归验证**：
- `test/integration/` 下的所有模块 e2e（field / template / event_type / fsm_config / event_type_schema）全部复跑
- 重点关注：
  - field check-name 接口：传非法 name 应返回 `ErrFieldNameInvalid`（不是 ErrBadRequest）
  - event_type check-name：同上模块错误码
  - 字段被模板引用后，修改约束收紧 → 应返回 `ErrFieldRefTighten`
  - 扩展字段 Schema 被事件类型引用后，修改约束收紧 → 应返回 `ErrExtSchemaRefTighten`

### 验证门槛

1. `go build ./...` 通过
2. `go test ./...` 通过（所有单测 + 集成测）
3. `grep -rn "func (h \*.*Handler) check" backend/internal/handler/` 只剩业务前置校验（`checkSeverity` / `checkPerceptionMode` / `checkPropertiesShape` / `checkFieldType`）
4. `grep -rn "util.CheckConstraintTightened" backend/` 结果为空
5. `ls backend/internal/util/` 输出只有 `const.go handler.go service.go store.go`
6. `npx vue-tsc --noEmit`（前端回归，确保无间接破坏）

### 手动验证点

- 启动后端，调 `POST /api/v1/fields/check-name`，传 `{"name": "INVALID"}`（大写非法），应返回 `code: ErrFieldNameInvalid`（非 `ErrBadRequest`）
- 调 `POST /api/v1/event-types/check-name`，传 `{"name": ""}`，应返回 `code: ErrEventTypeNameInvalid, message: "事件标识不能为空"`（验证消息模板正确）
- 调 `POST /api/v1/fields/update`，label 传 100 个中文字符（超 `FieldLabelMaxLength`），应返回 `code: ErrBadRequest, message: "中文标签长度不能超过 N 个字符"`
