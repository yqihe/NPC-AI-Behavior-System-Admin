# Go 语言常见陷阱

纯 Go 语言层面的陷阱。MySQL 见 `mysql-pitfalls.md`，MongoDB 见 `mongodb-pitfalls.md`，Redis 见 `redis-pitfalls.md`，缓存模式见 `cache-pitfalls.md`。禁止红线见 `../standards/go-red-lines.md`。

## JSON / BSON 序列化

- **nil slice → `null`**：`var s []T` 序列化为 `null`，前端 `v-for` 报错。必须 `make([]T, 0)`
- **nil map → `null`**：同理，必须 `make(map[string]T)`
- **json.Number 精度**：`json.Unmarshal` 到 `any` 时数字变 `float64`。存 MongoDB 用 `bson.UnmarshalExtJSON` 保留类型
- **struct tag**：`json:"name"` 和 `bson:"name"` 必须同时写
- **omitempty 吞零值**：`severity: 0` 是合法值，带 `omitempty` 会丢失
- **config 用 Raw 类型**：`{name, config}` 中 config 不反序列化为具体 struct，用 `bson.Raw`/`json.RawMessage` 透传
- **bson canonical 模式**：`bson.MarshalExtJSON(raw, true, false)` 的 `canonical=true` 输出 `{"$numberInt":"80"}`，前端无法解析。给前端必须 `canonical=false`

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

---

*踩到新坑时追加到对应分类下。*
