# Go 语言禁止红线

适用于所有 Go 项目。

## 禁止资源泄漏

- **禁止** `mongo.Client`、`redis.Client` 等外部连接在 shutdown 时不关闭。必须在 graceful shutdown 中显式调用 `Close()`/`Disconnect()`
- **禁止** `cursor.Close()` 复用查询超时 context。查询耗时接近超时时 Close 会因 context 到期而失败，导致游标泄漏。用 `context.Background()` 关闭
- **禁止**使用 `context.Background()` 或 `context.TODO()` 直接调用数据库/缓存。所有外部 IO 必须带超时 context
- **禁止**在多个 goroutine 中对同一 channel 执行 `close()`。关闭权归属唯一 owner，或用 `sync.Once` 保护

## 禁止序列化陷阱

- **禁止** nil slice/map 输出到 JSON API。`var s []T` 序列化为 `null`，前端 `v-for` 报错。必须 `make([]T, 0)` 初始化
- **禁止** struct 字段只写 `json` tag 不写 `bson` tag。漏写导致 MongoDB 字段名变大写开头
- **禁止**对合法零值字段加 `omitempty`。`severity: 0` 是合法值，`omitempty` 会丢弃它
- **禁止** `json.Unmarshal` 到 `any` 后假设数字是 int。所有数字变 `float64`

## 禁止错误处理不当

- **禁止** `writeError` / `http.Error` 后不 `return`。错误响应后继续执行会二次写入或空指针
- **禁止**返回 typed nil 指针作为 error。`var e *MyError = nil; return e` 导致 `err != nil` 为 true。直接 `return nil`
- **禁止** 500 响应暴露 Go error 原文。对外返回预定义中文提示，原始 error 写 slog

## 禁止 graceful shutdown 顺序错误

- **禁止**在清理资源前不先停止事件循环。正确顺序：停止接受新请求 → 等待进行中请求完成 → 关闭数据库/缓存连接
