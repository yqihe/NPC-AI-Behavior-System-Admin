# Go 常见陷阱（运营平台版）

继承自游戏服务端 `go-pitfalls.md`，针对 HTTP CRUD + MongoDB + Redis 场景裁剪和补充。编写代码时主动检查。

## JSON / BSON 序列化

- **nil slice → JSON `null`**：`var s []T` 序列化为 `null`，前端 `v-for` 直接报错。所有 API 响应中的 slice 必须 `make([]T, 0)` 初始化，序列化为 `[]`
- **nil map → JSON `null`**：同理，`var m map[string]T` 序列化为 `null`。必须 `make(map[string]T)` 初始化
- **json.Number 精度问题**：`json.Unmarshal` 到 `any` 时，所有数字变 `float64`。`priority: 10` 变成 `10.0`。存 MongoDB 时用 `bson.UnmarshalExtJSON` 保留原始类型，或在 model 中定义明确的 Go 类型（`int`/`float64`）避免 `any`
- **struct tag 拼写**：`json:"name"` 和 `bson:"name"` 标签必须同时写。漏写 bson tag 会导致 MongoDB 字段名变成大写开头（Go 默认导出名）
- **omitempty 陷阱**：`omitempty` 会吞掉零值（`0`、`""`、`false`）。事件的 `default_severity: 0` 是合法值，带 omitempty 会丢失。只在确实需要省略空值时使用
- **config 字段是 `bson.Raw` / `json.RawMessage`**：`{name, config}` 中 config 字段不应反序列化为具体 Go struct（各 collection 的 config 结构不同）。用 Raw 类型透传，校验时按需解析
- **bson.MarshalExtJSON canonical 模式**：`bson.MarshalExtJSON(raw, true, false)` 的 `canonical=true` 会把数字输出为 `{"$numberInt":"80"}`，前端无法解析。从 MongoDB 读取后转 JSON 给前端时必须用 `canonical=false`（relaxed 模式）。同理，`bson.UnmarshalExtJSON` 接收普通 JSON 时必须用 `canonical=false`

## HTTP Handler

- **请求体未关闭**：`http.Request.Body` 是 `io.ReadCloser`，标准库会自动关闭，但如果用 `io.ReadAll` 读取后又想二次读取，需要重新包装。最佳实践：读一次，解析一次
- **响应写入后继续写**：`w.WriteHeader(400)` 后再 `w.WriteHeader(200)` 会 superfluous warning 且不生效。`WriteHeader` 只能调一次，`w.Write()` 会隐式调 `WriteHeader(200)`
- **handler 中忘记 return**：写完错误响应后必须 `return`，否则继续执行后续逻辑，可能导致二次写入或空指针
  ```go
  // ❌ 错误
  if err != nil {
      writeError(w, 400, "参数错误")
      // 忘记 return，继续往下执行
  }
  
  // ✅ 正确
  if err != nil {
      writeError(w, 400, "参数错误")
      return
  }
  ```
- **goroutine 中使用 http.ResponseWriter**：handler return 后 ResponseWriter 失效。不要在 go func() 里写响应

## MongoDB 操作

- **连接泄漏**：`mongo.Client` 必须在程序退出时 `Disconnect`。用 `defer client.Disconnect(ctx)` 或 graceful shutdown
- **Context 超时**：所有 MongoDB 操作必须带超时 context（如 `context.WithTimeout(ctx, 5*time.Second)`），否则网络异常时 handler 永久挂起
- **FindOne 无结果**：`FindOne` 找不到文档时返回 `mongo.ErrNoDocuments`，不是 nil。必须用 `errors.Is(err, mongo.ErrNoDocuments)` 判断，不能用 `err != nil` 一刀切当成 500
- **UpdateOne/ReplaceOne 无匹配**：`result.MatchedCount == 0` 表示目标不存在，应返回 404，不能静默成功
- **Duplicate key error**：unique index 冲突时 MongoDB 返回 `mongo.WriteException`，需要检查 `writeErr.WriteErrors[i].Code == 11000` 转为 409 响应
- **bson.M key 顺序**：`bson.M` 是 map，key 顺序不确定。如果需要有序文档（如创建复合索引），用 `bson.D`

## Redis 操作

- **连接池耗尽**：默认连接池大小有限，高并发下 `Get`/`Set` 会阻塞等待。确保配置合理的 `PoolSize`（admin 平台并发低，默认值足够，但需要知道这个坑）
- **序列化/反序列化不一致**：存入 Redis 用 `json.Marshal`，取出必须用 `json.Unmarshal` 到相同类型。不要存 Go struct 取 map[string]any
- **缓存 key 命名冲突**：统一前缀 `admin:`，如 `admin:event_types:list`。避免与游戏服务端（如果也用 Redis）key 冲突
- **Get 返回 redis.Nil**：key 不存在时返回 `redis.Nil` 错误，不是空字符串。用 `errors.Is(err, redis.Nil)` 判断缓存未命中，然后回源 MongoDB

## 错误处理

- **error 不要忽略**：`result, _ := collection.InsertOne(ctx, doc)` — 插入失败后用 result 会 panic
- **errors.Is / errors.As**：比较 error 用 `errors.Is`，不用 `==`。MongoDB 和 Redis 的 error 都是包装过的
- **错误信息不暴露给前端**：`err.Error()` 可能包含 MongoDB 连接串、集合名等敏感信息。对外只返回预定义的中文提示，原始 error 写 slog

## 数据结构

- **nil map 写入 panic**：`var m map[string]string; m["a"] = "b"` 直接 panic。所有 map 类型字段在构造时必须 `make` 初始化
- **typed nil vs nil interface**：`var e *MyError = nil; var err error = e; err != nil` 为 true。返回 error 时直接 `return nil`，不要返回一个 typed nil 指针

## 测试

- **-race 标志**：`go test -race ./...` 必须通过。HTTP handler 虽然是同步的，但 MongoDB/Redis client 内部有 goroutine
- **测试用 MongoDB**：集成测试连真实 MongoDB（可用 Docker），不 mock。原因：游戏服务端教训——mock 测试通过但 prod 迁移失败
- **测试清理**：每个测试用例用独立的 collection 名（如加随机后缀），或在 `TestMain` 中清库，避免测试间互相污染

---

*在开发过程中踩到新坑时追加到本文档对应分类下。*
