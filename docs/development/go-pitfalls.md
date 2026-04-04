# Go 常见陷阱

编写 Go 代码时主动检查。禁止红线见 `../standards/go-red-lines.md`。

## JSON / BSON 序列化

- **nil slice → `null`**：`var s []T` 序列化�� `null`，前端 `v-for` 报错。必须 `make([]T, 0)`
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
- **请求体**：`http.Request.Body` 读一次解析一��，不要二次读取

## MongoDB 操作

- **连接泄漏**：`mongo.Client` 必须在 shutdown 时 `Disconnect`
- **Context 超时**：所有操作带 `context.WithTimeout`
- **FindOne 无结果**：返回 `mongo.ErrNoDocuments`，用 `errors.Is` 判断
- **UpdateOne 无匹配**：`result.MatchedCount == 0` 返��� 404
- **Duplicate key**：检查 `WriteErrors[i].Code == 11000` 转 409
- **bson.M key 顺序**：`bson.M` 是 map 无序，需要有序用 `bson.D`

## Redis 操作

- **Get 返回 redis.Nil**：key 不存在时返回 `redis.Nil`，用 `errors.Is(err, redis.Nil)` 判断
- **序列化一致**：存 `json.Marshal`，取 `json.Unmarshal` 到相同类型
- **key 命名**：统一前缀 `admin:`，避免与游戏服务端冲���

## 错误处理

- **error 不要忽略**：`result, _ := InsertOne(...)` 插入失败后用 result 会 panic
- **errors.Is/As**：比较 error 用 `errors.Is`，不用 `==`
- **typed nil**：返回 error 直接 `return nil`，不返回 typed nil 指针

## 数据结构

- **nil map 写入 panic**：`var m map[string]string; m["a"] = "b"` panic。必须 `make` 初始化

## 测试

- **-race**：`go test -race ./...` 必须通过
- **集成测试连真实 MongoDB**：不 mock，用 Docker 起测试库
- **测试清理**：每个用例用独立 collection 或 `TestMain` 清库

---

*踩到新坑时追加到对应分类下。*
