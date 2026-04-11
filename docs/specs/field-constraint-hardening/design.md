# field-constraint-hardening — 设计方案

## 方案描述

本次改动全部在 `backend/internal/{handler,service,errcode}/` 内，无新增包、无跨模块 API 变动。按三块分开描述。

### 块 1：`properties` 形状校验（R5 → atk11.6）

**定位**：`backend/internal/handler/field.go`，新增一个包内工具函数 `checkPropertiesShape`，在 `Create` / `Update` 的前置校验段调用。

**函数签名**：

```go
// checkPropertiesShape 校验 properties 原始字节必须是 JSON 对象（首字符 '{'）
// 目的：防御 properties=[] / "foo" / 123 / true / null 这类形状错误
// 在 handler 层做形状拦截，service 层拿到的 json.RawMessage 就可以放心按 object 解
func checkPropertiesShape(raw json.RawMessage) *errcode.Error
```

**实现**：

```go
func checkPropertiesShape(raw json.RawMessage) *errcode.Error {
    trimmed := bytes.TrimSpace(raw)
    if len(trimmed) == 0 {
        return errcode.Newf(errcode.ErrBadRequest, "properties 不能为空")
    }
    if trimmed[0] != '{' {
        return errcode.Newf(errcode.ErrBadRequest, "properties 必须是 JSON 对象")
    }
    return nil
}
```

**调用点改动**：

```go
// Handler.Create 现有：
if req.Properties == nil {
    return nil, errcode.Newf(errcode.ErrBadRequest, "properties 不能为空")
}
// 改为：
if err := checkPropertiesShape(req.Properties); err != nil {
    return nil, err
}
// Handler.Update 同理
```

**关键点**：

- `json.RawMessage` 是 `[]byte` 别名。客户端传 `"properties": null` 反序列化后 `req.Properties` 是 `[]byte("null")`（长度 4），**不是 nil**，所以原来的 `req.Properties == nil` 校验漏掉了所有非对象形状。
- `bytes.TrimSpace` 处理前置空白（客户端可能发 `" { ... }"`）。
- 不用 `json.Unmarshal` 反序列化判断类型，因为那等于把 service 层的工作在 handler 又做一遍；只看首字符最轻量。

### 块 2：reference 字段的 refs 校验统一化（R1/R2/R3/R4 → atk1.2, atk7.1）

**定位**：`backend/internal/service/field.go`，新增 `validateReferenceRefs` 替换 Create / Update 中两处散落的逻辑。

**函数签名**：

```go
// validateReferenceRefs 校验 reference 字段的 refs 业务规则
//
// currentID: 正在创建/编辑的字段 ID（Create 传 0）
// newRefIDs: 新的 refs 列表
// oldRefSet: 旧 refs 集合（Create 传 nil）。用于区分"新增引用"与"已有引用"——
//            只有新增的 ref 才检查启用状态和嵌套，已有的 ref 即使后来变成 reference
//            类型或被停用也保留（存量不动原则）。
//
// 规则：
//   1. newRefIDs 不能为空 → 40014 ErrFieldRefNotFound（保持向前兼容的错误码段）
//      注意：改用 ErrFieldRefEmpty (40017) 语义更精确，但会让"40014 含义"分裂。
//      最终选 ErrFieldRefEmpty (见后文 errcode 改动)
//   2. 每个 refID 必须存在 → 40014
//   3. 新增的 refID 必须启用 → 40013
//   4. 新增的 refID 不能是 reference 类型 → 40016 ErrFieldRefNested（新增）
//   5. 不能形成循环引用 → 40009（复用现有）
func (s *FieldService) validateReferenceRefs(
    ctx context.Context,
    currentID int64,
    newRefIDs []int64,
    oldRefSet map[int64]bool,
) error
```

**实现骨架**：

```go
func (s *FieldService) validateReferenceRefs(ctx context.Context, currentID int64, newRefIDs []int64, oldRefSet map[int64]bool) error {
    if len(newRefIDs) == 0 {
        return errcode.New(errcode.ErrFieldRefEmpty)
    }
    for _, refID := range newRefIDs {
        f, err := s.fieldStore.GetByID(ctx, refID)
        if err != nil {
            return fmt.Errorf("check ref field %d: %w", refID, err)
        }
        if f == nil {
            return errcode.Newf(errcode.ErrFieldRefNotFound, "引用的字段 ID=%d 不存在", refID)
        }
        if oldRefSet[refID] {
            continue // 存量不动：已有的 ref 不做启用/嵌套检查
        }
        if !f.Enabled {
            return errcode.Newf(errcode.ErrFieldRefDisabled, "字段 '%s' 已停用，不能引用", f.Name)
        }
        if f.Type == model.FieldTypeReference {
            return errcode.Newf(errcode.ErrFieldRefNested, "字段 '%s' 是 reference 类型，不允许嵌套引用", f.Name)
        }
    }
    return s.detectCyclicRef(ctx, currentID, newRefIDs)
}
```

**Create 调用**（替换 field.go:134-158 的那段）：

```go
var refFieldIDs []int64
if req.Type == model.FieldTypeReference {
    props, _ := parseProperties(req.Properties)
    if props != nil {
        refFieldIDs = parseRefFieldIDs(props.Constraints)
    }
    if err := s.validateReferenceRefs(ctx, 0, refFieldIDs, nil); err != nil {
        return 0, err
    }
}
```

**Update 调用**（替换 field.go:270-304 的那段）：

```go
if req.Type == model.FieldTypeReference {
    newProps, _ := parseProperties(req.Properties)
    var newRefIDs []int64
    if newProps != nil {
        newRefIDs = parseRefFieldIDs(newProps.Constraints)
    }
    // 算旧 ref 集合（仅当旧类型也是 reference 时）
    oldRefSet := make(map[int64]bool)
    if old.Type == model.FieldTypeReference {
        oldProps, _ := parseProperties(old.Properties)
        if oldProps != nil {
            for _, rid := range parseRefFieldIDs(oldProps.Constraints) {
                oldRefSet[rid] = true
            }
        }
    }
    if err := s.validateReferenceRefs(ctx, req.ID, newRefIDs, oldRefSet); err != nil {
        return err
    }
}
```

**关键点**：

- Create 时 `oldRefSet` 传 `nil`（不是 empty map），`nil` map 读取返回零值 `false`，所以所有 ref 都会走"新增"路径，符合预期。
- Update 时若旧类型是 reference 且旧 refs 里已经有嵌套引用（因为历史上没有嵌套检查），保留原有嵌套不拦截，这是 R2 要的"存量不动"。但若用户**移除了某个旧嵌套然后又加回来**，`oldRefSet` 里没它（因为 parseRefFieldIDs 是从新请求解析的），会被识别为"新增"然后拦截——这是合理的，用户在那一刻主动做出了"把嵌套加回来"的动作，拦截符合直觉。
- `detectCyclicRef` 不动，继续用。现有实现已经正确处理循环。
- `parseRefFieldIDs` 不动，但本次顺带补一个去重（见"块 4 顺带处理"）。

### 块 3：`checkConstraintTightened` 补齐覆盖面（R6/R7/R8 → atk3/4/6）

**定位**：`backend/internal/service/field.go`，扩展现有的 `checkConstraintTightened` 函数的 `case` 分支。

**当前代码结构不变**，只是每个 case 追加规则：

```go
switch fieldType {
case "integer", "float":
    // 现有：min/max 检查
    // 新增：float 专属的 precision
    if fieldType == "float" {
        if oldPrec, ok := getFloat(oldMap["precision"]); ok {
            if newPrec, ok2 := getFloat(newMap["precision"]); ok2 && newPrec < oldPrec {
                return errcode.Newf(errcode.ErrFieldRefTighten, "precision 从 %v 降低为 %v，请先移除引用", oldPrec, newPrec)
            }
        }
    }

case "string":
    // 现有：minLength/maxLength 检查
    // 新增：pattern 检查
    oldPat := getStringFromRaw(oldMap["pattern"])
    newPat := getStringFromRaw(newMap["pattern"])
    if newPat != "" && newPat != oldPat {
        return errcode.Newf(errcode.ErrFieldRefTighten, "pattern 从 %q 变更为 %q，可能使已有数据失效，请先移除引用", oldPat, newPat)
    }

case "select":
    // 现有：options 删除检查
    // 新增：minSelect 只能变小、maxSelect 只能变大
    if oldMinSel, ok := getFloat(oldMap["minSelect"]); ok {
        if newMinSel, ok2 := getFloat(newMap["minSelect"]); ok2 && newMinSel > oldMinSel {
            return errcode.Newf(errcode.ErrFieldRefTighten, "minSelect 从 %v 收紧为 %v，请先移除引用", oldMinSel, newMinSel)
        }
    }
    if oldMaxSel, ok := getFloat(oldMap["maxSelect"]); ok {
        if newMaxSel, ok2 := getFloat(newMap["maxSelect"]); ok2 && newMaxSel < oldMaxSel {
            return errcode.Newf(errcode.ErrFieldRefTighten, "maxSelect 从 %v 收紧为 %v，请先移除引用", oldMaxSel, newMaxSel)
        }
    }
}
```

**新增辅助函数**：

```go
// getStringFromRaw 从 json.RawMessage 提取字符串值，失败返回空串
func getStringFromRaw(raw json.RawMessage) string {
    if len(raw) == 0 {
        return ""
    }
    var s string
    if err := json.Unmarshal(raw, &s); err != nil {
        return ""
    }
    return s
}
```

**pattern 语义约定**（同步写进 features 功能 10）：

| old | new | 判定 | 原因 |
|---|---|---|---|
| "" | "" | 允许 | 未变 |
| "" | "^x$" | 拒绝 40007 | 新增 pattern → 旧数据可能不匹配 |
| "^x$" | "^x$" | 允许 | 未变 |
| "^x$" | "^y$" | 拒绝 40007 | pattern 变化 → 旧数据可能不匹配新 |
| "^x$" | "" | 允许 | 移除 pattern = 放宽 |

判定逻辑：`newPat != "" && newPat != oldPat`。

**不做**：不做 `integer.step` 变更检查（atk5 当前放行，留给产品决策）。

### 块 4：errcode 新增 2 条

**定位**：`backend/internal/errcode/codes.go`。

```go
const (
    // ...existing...
    ErrFieldRefNested = 40016 // reference 字段禁止嵌套引用
    ErrFieldRefEmpty  = 40017 // reference 字段必须至少引用一个目标
)

var messages = map[int]string{
    // ...existing...
    ErrFieldRefNested: "不能引用 reference 类型字段，禁止嵌套",
    ErrFieldRefEmpty:  "reference 字段必须至少引用一个目标字段",
}
```

### 块 5：tests/api_test.sh 的微调

`atk1.2` 目前的 `assert_bug` 期望 `code=40009`。改完以后实际会返回 `40016`，需要把期望也改到 `40016`。其他攻击测试的断言已经使用宽松的"40007 / 40000 / 40014"分支，无需改。

```bash
# atk1.2 修改前
assert_bug "atk1.2 reference 嵌套应被拒绝" "40009" "$R" "..."
# 修改后
assert_bug "atk1.2 reference 嵌套应被拒绝" "40016" "$R" "..."
```

同理 atk7.1（refs=[]）目前在 `if CODE = 40000 or 40014` 的分支判断里，需要加上 40017：

```bash
if [ "$CODE" = "40000" ] || [ "$CODE" = "40014" ] || [ "$CODE" = "40017" ]; then
```

---

## 方案对比

### 备选方案 A：在 service 层做 `properties` 形状校验

**做法**：`properties` 形状检查放在 `FieldService.Create/Update` 的业务校验段，handler 只管格式。

**为什么不选**：违反本项目已经建立的"handler 做格式校验、service 做业务校验"分层（见 `backend-red-lines.md` 第 4 节"禁止 handler 层校验使用错误的错误码"、以及 handler/field.go 现有的 `checkName/checkLabel` 等函数组）。`properties` 形状属于请求体格式错误，归 handler 拦最符合现状。

### 备选方案 B：引入完整的 JSON Schema 校验库

**做法**：引入 `github.com/xeipuuv/gojsonschema` 对 `properties` 做基于 schema 的完整校验。seed 文件里已经有 `constraint_schema` 字段，可以直接驱动校验。

**为什么不选**：

1. 新引入依赖，过度设计。本次只是想挡 7 个具体 bug，不是为 constraints 做一套完整的 schema 运行时；
2. schema 要在启动时加载并维护与字典表的同步，牵扯缓存失效策略；
3. 违反"禁止引入没有使用场景的依赖"（见 `red-lines.md`）；
4. 未来如果真要做，应该走独立 spec，和字段管理前端的 `SchemaForm` 统一设计。

### 备选方案 C：reference 嵌套拦截复用 40009 `ErrFieldCyclicRef`

**做法**：不新增 `ErrFieldRefNested`，直接复用 40009，靠消息文案区分。

**为什么不选**：违反 `go-red-lines.md` 的"禁止错误码语义混用"。前端在 `Create` / `Update` 的错误处理里会按 `code` 走不同分支——循环引用的提示是"你选的字段链条里有环"，嵌套的提示是"你不能选另一个 reference"，混用会让前端逻辑反复判断消息字符串。

### 备选方案 D：reference.refs=[] 复用 40000 `ErrBadRequest`

**做法**：不新增 `ErrFieldRefEmpty`，refs=[] 时返回通用 40000，消息说清楚。

**为什么不选**：

- 通用 40000 的前端行为通常是"显示 message 字段"，不会做针对性的字段高亮或清空提示；
- 引入一个专用码的成本极低（2 行代码 + 1 行 message），收益是前端可以把这个错误和"reference 下拉框为空"绑定 UI 反馈；
- 但本决策也可以接受备选方案 D，用户如果觉得段位膨胀，可以随时退回。**首选新增，列为可接受的降级选项。**

### 选定方案

采用主方案。关键决策：

- handler 形状校验（A 的反面）；
- 不引 JSON Schema 库（不选 B）；
- 为 reference 嵌套新增 `ErrFieldRefNested` 40016（不选 C）；
- 为 refs=[] 新增 `ErrFieldRefEmpty` 40017（不选 D，但允许退化）。

---

## 红线检查

| 红线文档 | 相关条目 | 本方案是否触及 | 说明 |
|---|---|---|---|
| `standards/red-lines.md` - 禁止静默降级 | "lookup 失败时 silent return/continue" | 不触及 | 新代码在校验失败时返回明确 error code，不 silent continue |
| 同上 - 禁止过度设计 | "不引入没有使用场景的依赖" | 不触及 | 不引新依赖，只用现有 `encoding/json` / `bytes`（标准库） |
| 同上 - 禁止安全隐患 | "禁止信任前端校验" | 正向满足 | 本次就是在加强后端校验 |
| `standards/go-red-lines.md` - 禁止序列化陷阱 | "`json.RawMessage` 不能 scan NULL" | 不触及 | 本次不动 store 层，字段 `properties` 列仍然是 `json.RawMessage`（非指针），现状 | 
| 同上 - 禁止错误码语义混用 | ✓ | **主动遵守** | 这正是不选备选 C 的理由，新增 40016 / 40017 专用码而非复用 |
| 同上 - 禁止硬编码魔术字符串 | "reference" 应用 `FieldTypeReference` 常量 | 主动遵守 | 新代码用 `model.FieldTypeReference`，不出现字面量 |
| 同上 - 禁止错误处理不当 | "writeError 后不 return" | 不触及 | 无直接写 ResponseWriter 的代码 |
| `standards/mysql-red-lines.md` | — | 不触及 | 不动 SQL |
| `standards/redis-red-lines.md` | — | 不触及 | 不动 Redis |
| `standards/cache-red-lines.md` | "修改 detail 字段必须 DelDetail" | 现状已满足 | Update 现有路径已经清缓存，本次不动该部分 |
| `standards/frontend-red-lines.md` | — | 不触及 | 无前端改动 |
| `architecture/backend-red-lines.md` - 禁止硬编码 | "错误码统一定义在 errcode/codes.go" | 主动遵守 | 新增 40016/40017 放在 codes.go |
| 同上 - "name 校验用 ErrFieldNameInvalid" | 不触及 | 不动 name 校验 | |
| 同上 - "使用常量" | 主动遵守 | `model.FieldTypeReference` |
| `architecture/ui-red-lines.md` | — | 不触及 | 无 UI 改动 |

**结论：无红线违反，无需申请例外。**

---

## 扩展性影响

**对"新增配置类型"方向**（加一组 handler/service/store/validator）：

- **正向**：抽象出的 `validateReferenceRefs` 是一个可复用的模板，未来若状态机配置也要"配置 A 可引用配置 B"的校验时，可以参考这个形状；
- **中性**：新增 40016/40017 是字段段位内部的扩展，不影响其他配置类型的错误码段位。

**对"新增表单字段"方向**（加一个组件）：

- **正向**：`checkPropertiesShape` 为前端动态表单提供了一个稳定的基线契约——后端拿到的 `properties` 一定是对象，前端 `SchemaForm` 组件可以放心假设；
- **正向**：`checkConstraintTightened` 补齐 select/float/string 的收紧检查后，新 constraint key 加入时开发者有一个可对照的模式——任何新 key 都应该问"被引用时能不能收紧"，并在此函数里补规则；
- **为此沉淀规约**：在 `docs/development/go-pitfalls.md` 新增陷阱条目"新增 constraint key 时必须同步更新 checkConstraintTightened"，让未来加字段类型的人能读到。

**不引入过度抽象**：不为 `checkConstraintTightened` 改成策略模式或 interface 注册表，因为 case 数量有限（6 个字段类型），直接扩展 switch 最轻量。

---

## 依赖方向

涉及的包：

```
cmd/admin (main.go)  ← 不变
      ↓
router (router.go)   ← 不变
      ↓
handler              ← 改动 (field.go: checkPropertiesShape + 调用点)
      ↓
service              ← 改动 (field.go: validateReferenceRefs + checkConstraintTightened 扩展)
      ↓
store (mysql/redis)  ← 不变
      ↓
model                ← 不变
      ↓
errcode              ← 改动 (codes.go: +2 常量 +2 消息)
```

**依赖关系单向向下，无逆向依赖**。handler 依赖 service、service 依赖 store、全体依赖 errcode —— 符合 `go-red-lines.md` 的"禁止分层倒置"。

---

## 陷阱检查

### Go 陷阱（`go-pitfalls.md`）

- **nil map 读取**：`oldRefSet[refID]` 在 `oldRefSet` 为 nil 时读取返回零值 `false`，Go 规范保证，不 panic。**已确认**。
- **nil map 写入 panic**：本方案 `oldRefSet` 在 Update 路径用 `make(map[int64]bool)` 初始化，Create 路径不写，只读。**安全**。
- **`json.RawMessage` 的空值**：客户端传 `"properties": null` 时 `req.Properties = []byte("null")`（长度 4，非 nil）。**已针对性处理**，`checkPropertiesShape` 判首字符。
- **`json.Unmarshal` 到 `any` 类型**：本方案不在这条陷阱上，`parseConstraintsMap` 现在就是 `map[string]json.RawMessage`（类型安全）。
- **UTF-8 字符数**：本方案不动字符串长度校验，沿用 handler 现有 `utf8.RuneCountInString`。
- **错误响应后忘 return**：本方案 handler 改动都是 `if err { return nil, err }` 形状，**安全**。
- **typed nil**：本方案不返回 typed nil 指针。

### 缓存陷阱（`cache-pitfalls.md`）

- **写后必须清缓存**：本方案**不改变 DB 写入路径**，清缓存逻辑沿用现有 `Update` / `Create`。新增的校验全部在写 DB 之前，失败时不会落库也不会污染缓存。
- **级联清缓存**：本方案不新增级联写操作。

### MySQL 陷阱

- 本方案不动 SQL，不触发 MySQL 陷阱。

### Redis 陷阱

- 本方案不动 Redis，不触发 Redis 陷阱。

### 前端陷阱

- 本方案不动前端。

---

## 配置变更

**无**。本次不新增 / 修改任何 YAML / JSON 配置文件，不改 seed 数据，不改 `config.yaml` 的 `Validation` / `Pagination` 段。

`backend/cmd/seed/main.go` 的 `constraint_schema` 字段**不动**——它只是给前端看的字段类型元数据，本次在后端校验上加规则不等于要改前端可见的 schema 定义。

---

## 测试策略

### 单元测试

**不新增单元测试**。理由：

1. 现有的 `tests/api_test.sh` 已经作为集成测试覆盖全部 7 个攻击场景，从 HTTP 入口跑到 MySQL 落库，端到端保真度更高；
2. 本次改的 `validateReferenceRefs` / `checkConstraintTightened` 需要 `fieldStore` / `dictCache` 等依赖，单元测试要 mock 大半个 store 层，收益低；
3. 项目当前 Go 单元测试基建还不成熟（`backend/internal/` 下没有 `*_test.go`），为本次改动现场搭 fixture 和任务范围不符。

**若后续加测试基建**，优先为 `checkConstraintTightened` 写表格驱动测试（纯函数，无依赖，最容易写）。

### 集成测试

**直接用 `tests/api_test.sh`**，修复后重跑一遍，预期：

| 断言 | 修复前 | 修复后 |
|---|---|---|
| atk1.2 reference 嵌套 | BUG (code=0) | PASS (code=40016) |
| atk3.1 minSelect 收紧 | BUG (code=0) | PASS (code=40007) |
| atk3.2 maxSelect 收紧 | BUG (code=0) | PASS (code=40007) |
| atk4.1 precision 收紧 | BUG (code=0) | PASS (code=40007) |
| atk6.1 pattern 新增 | BUG (code=0) | PASS (code=40007) |
| atk7.1 refs=[] 空数组 | BUG (code=0) | PASS (code=40017) |
| atk11.6 properties=[] | BUG (code=0) | PASS (code=40000) |
| 其余 192 项 | PASS | PASS（无退化） |

**注意 Part 1-4 必须继续全 PASS**。特别是：

- Part 3 f10.3/f10.4 现有的 min/max 收紧拦截不能因为 case 结构扩展而回归；
- Part 3 f11.1 `B refs [A]` 中 A 是 `integer`，B 是 `reference`，**不是嵌套**，不应被新规则拦截。
- Part 3 f11.2 stopped 字段拦截（40013）依然有效。
- 字段引用/循环引用的所有现有用例不受影响。

### 额外的回归风险点

1. **模板管理测试 Part 4**：模板管理依赖字段管理的 `ValidateFieldsForTemplate` 等跨模块对外方法，本次**不改这些方法**，只改 Create/Update 内部。预期 Part 4 全部继续 PASS。
2. **字段引用详情 x.1/x.2**：依赖 `GetByIDsLite` 和跨模块 label 补全，**不改**，预期继续 PASS。
3. **`parseProperties` 失败的兼容性**：现有代码在 parseProperties 失败时返回 `nil, err`，上层代码用 `props, _ := ...` 忽略 err 然后判 `props != nil`。本次改动后 handler 已经挡住了形状错误，service 层 `parseProperties` 失败概率极低，兼容性不变。

### 执行步骤（落到 /spec-execute 各 T）

每个 T 完成后立刻跑 `bash tests/api_test.sh`（memory feedback: "写完必须先验证"），FAIL 计数必须只减不增。

---

**Phase 2 完成，停下等待审批**。审批通过后进入 Phase 3 任务拆解。
