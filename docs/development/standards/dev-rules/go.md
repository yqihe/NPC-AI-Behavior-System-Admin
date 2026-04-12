# Go 语言开发规范

禁止红线见 `../red-lines/go.md`。

## JSON / BSON 序列化

- **nil slice → `null`**：`var s []T` 序列化为 `null`，前端 `v-for` 报错。必须 `make([]T, 0)`
- **nil map → `null`**：同理，必须 `make(map[string]T)`
- **json.Number 精度**：`json.Unmarshal` 到 `any` 时数字变 `float64`。存 MongoDB 用 `bson.UnmarshalExtJSON` 保留类型
- **struct tag**：`json:"name"` 和 `bson:"name"` 必须同时写
- **omitempty 吞零值**：`severity: 0` 是合法值，带 `omitempty` 会丢失
- **config 用 Raw 类型**：`{name, config}` 中 config 不反序列化为具体 struct，用 `bson.Raw`/`json.RawMessage` 透传
- **bson canonical 模式**：`bson.MarshalExtJSON(raw, true, false)` 的 `canonical=true` 输出 `{"$numberInt":"80"}`，前端无法解析。给前端必须 `canonical=false`
- **`json.RawMessage` 对 `null` 不变 nil**：客户端传 `"field": null` 时 `req.Field` 是 `[]byte("null")`（长度 4），**不是 nil**。`if req.Field == nil` 会漏掉 `null` / `[]` / `"foo"` / `123` / `true` 等所有非对象形状。对"必须是 JSON 对象"的 RawMessage 字段（典型如字段管理 `properties`），必须在 handler 层用 `bytes.TrimSpace` + 首字符判 `{` 的方式拦截，或在 service 层 `json.Unmarshal` 到具体结构后判空。`field-constraint-hardening` spec 的 atk11.6 就是踩的这个坑——`properties=[]` 原本能直接落库成 `[]` 坏数据

## HTTP Handler

- **响应后忘 return**：`writeError` 后必须 `return`，否则继续执行导致二次写入或空指针
- **响应写入后继续写**：`WriteHeader` 只能调一次
- **goroutine 中写 ResponseWriter**：handler return 后 ResponseWriter 失效
- **请求体**：`http.Request.Body` 读一次解析一次，不要二次读取

## 错误处理

- **error 不要忽略**：`result, _ := InsertOne(...)` 插入失败后用 result 会 panic
- **errors.Is/As**：比较 error 用 `errors.Is`，不用 `==`
- **typed nil**：返回 error 直接 `return nil`，不返回 typed nil 指针

## 数据结构

- **nil map 写入 panic**：`var m map[string]string; m["a"] = "b"` panic。必须 `make` 初始化

## 字符串

- **`len()` 不是字符数**：`len("中文标签")` 返回 12（UTF-8 字节数），不是 4。校验中文字符串长度必须用 `utf8.RuneCountInString()`。纯 ASCII 字段名（如 `^[a-z][a-z0-9_]*$`）用 `len()` 没问题

## sqlx / database/sql

- **`json.RawMessage` 不能 scan NULL**：MySQL 列可能为 NULL，`json.RawMessage` 是 `[]byte` 的别名，driver 无法将 nil 存入非指针类型。必须用 `*json.RawMessage`。否则启动加载时 `sql: Scan error on column "extra": unsupported Scan, storing driver.Value type <nil> into type *json.RawMessage`

## 测试

- **-race**：`go test -race ./...` 必须通过
- **测试清理**：每个用例用独立数据或 `TestMain` 清库

## 包设计

- **依赖方向不能倒置**：`store`（数据访问层）不应 import `cache`（缓存层）。如果 store 需要 cache 的东西（如 key 函数），说明那些东西放错了位置，应该移到 store 中
- **导出可见性最小化**：只在包内使用的常量用小写（unexported）。比如 Redis version key 只在 `store/redis` 内部使用，不需要导出给外部

## 业务校验规约

- **新增 constraint key 必须同步更新 `checkConstraintTightened`**：字段管理的 `service/field.go:checkConstraintTightened` 实现了"被引用字段的约束只能放宽不能收紧"这个不变式，按字段类型分 case 校验。若字段类型新增 constraint 字段（比如未来加 `date` 类型的 `minDate/maxDate`，或给现有类型加 `step`/`precision`/`pattern` 之外的新 key），**必须同步在对应 case 中补一条"收紧判定"规则**。漏写会导致被引用字段可以静默收紧新约束，已有数据突然变非法——`field-constraint-hardening` spec 专门堵过 5 个这样的漏洞：`float.precision`、`string.pattern`、`select.minSelect/maxSelect`。判断流程：
  1. 这个 key 代表"范围"还是"枚举"还是"格式"？
  2. 放宽方向是什么？（数值变大/变小、集合增加、`pattern` 置空）
  3. 在 `checkConstraintTightened` 对应 case 里写判定 + 返回 `ErrFieldRefTighten`
  4. 在 `tests/api_test.sh` 的攻击段加一条 atk 用例，确认现在能拦

---

*踩到新坑时追加到对应分类下。*
