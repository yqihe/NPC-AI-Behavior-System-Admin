# Go 语言禁止红线

适用于所有 Go 项目。纯 Go 语言层面的禁令。MySQL 见 `mysql.md`，Redis 见 `redis.md`，缓存模式见 `cache.md`。开发规范见 `../dev-rules/go.md`。

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

- **禁止** MySQL 可空 JSON 列用 `json.RawMessage`（非指针）接收。NULL 值无法 scan 进 `[]byte`，必须用 `*json.RawMessage`

## 禁止错误处理不当

- **禁止** `writeError` / `http.Error` 后不 `return`。错误响应后继续执行会二次写入或空指针
- **禁止**返回 typed nil 指针作为 error。`var e *MyError = nil; return e` 导致 `err != nil` 为 true。直接 `return nil`
- **禁止** 500 响应暴露 Go error 原文。对外返回预定义中文提示，原始 error 写 slog

## 禁止字符串长度计算错误

- **禁止**用 `len()` 校验包含中文的字符串长度。`len("中文")` 返回 6（UTF-8 字节数）而非 2（字符数）。必须用 `utf8.RuneCountInString()` 计算字符数

## 禁止嵌套循环 continue/break 跳转错误

- **禁止**在嵌套 for 循环中使用 `continue`/`break` 时不确认跳转目标。Go 的 `continue`/`break` 只作用于最内层循环。如果需要跳出外层循环，必须使用 labeled break/continue 或 flag 变量。典型场景：事务回滚后 `continue` 到的是内层循环而非外层，导致在已回滚事务上继续操作

## 禁止错误码语义混用

- **禁止**同一个错误码用于不同语义场景。例如 `ErrFieldRefNotFound`（引用的字段不存在）不能也用于"字段本身不存在"，即使通过 `Newf` 覆盖了消息。前端按错误码分支处理逻辑，语义混用会导致前端行为异常

## 禁止缓存反序列化类型丢失

- **禁止** `json.Unmarshal` 到 `any` 类型的字段后假设类型正确。`ListData{Items: any}` 存入 Redis 后反序列化回来，Items 变为 `[]interface{}` 而非 `[]FieldListItem`。缓存层必须使用类型安全的结构体

## 禁止硬编码魔术字符串

- **禁止**在多处使用同一个字符串字面量（如 `"reference"`）。必须定义为常量（如 `model.FieldTypeReference`），避免拼写错误和重构遗漏

## 禁止分层倒置

- **禁止** `store` 层依赖 `cache` 层。Redis key 管理属于 `store/redis` 的职责，不应放在上层 `cache` 包中
- **禁止**高层包导出只被低层包使用的常量。如 Redis key 常量只在 `store/redis` 内部使用，应为包内可见（小写）

## 禁止 graceful shutdown 顺序错误

- **禁止**在清理资源前不先停止事件循环。正确顺序：停止接受新请求 → 等待进行中请求完成 → 关闭数据库/缓存连接
