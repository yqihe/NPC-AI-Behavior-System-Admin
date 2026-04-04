# 通用开发规则

## 日志格式

后端统一使用结构化日志：

```go
slog.Info("handler.create_event", "name", name, "severity", severity)
slog.Warn("validator.error", "collection", "fsm_configs", "name", name, "err", err)
```

## 文档同步

**强制规则：代码改动和文档更新必须在同一步骤完成。**

来源：游戏服务端开发中多次出现代码改了但文档没同步的问题。

### 改代码时必须同步的文档

- 当前 spec 的 `requirements.md` / `design.md` / `tasks.md`

### 改完代码后检查的文档

- `docs/specs/<当前层>/` — 实现偏离了 spec 时同步更新
- `CLAUDE.md` — 目录结构、技术栈、开发指令是否变化
- `docs/architecture/red-lines.md` — 是否发现新的禁令
- `docs/development/dev-rules.md` — 是否有新规则

## Git 规则

- 每个需求创建 feature 分支：`feature/<spec-name>`
- commit message 格式：`类型(范围): 描述`
  - 类型：`feat` / `fix` / `test` / `refactor` / `docs` / `chore`
  - 范围：`backend/handler`、`frontend/views`、`backend/store` 等

## 前后端协作规则

- API 接口先定义（OpenAPI/接口文档），再分别实现
- 前端不直接操作 MongoDB——所有数据操作通过后端 API
- 前端校验是 UX 优化，后端校验是安全保障——两者都要做

## Docker 构建与运行

```bash
# 启动全部服务
docker compose up --build

# 后台启动
docker compose up --build -d

# 停止
docker compose down
```

## CRUD 通用规则

### Name 唯一性

`name` 是各 collection 的业务主键。创建时必须用 MongoDB unique index 保证唯一性，不能用"先查后插"的方式（存在并发竞态）。收到 duplicate key error 后，返回友好中文提示"名称已存在"。

### 写操作幂等性

UPDATE 使用 `ReplaceOne` 整体替换 `{name, config}` 文档（PUT 语义），不做部分字段 PATCH。这与游戏服务端的配置加载方式一致——每次读取完整文档。

### 空值处理

| Go 类型 | JSON 序列化 | 要求 |
|---------|------------|------|
| `[]T(nil)` | `null` ❌ | 必须初始化为 `[]T{}` → `[]` |
| `map[string]T(nil)` | `null` ❌ | 必须初始化为 `map[string]T{}` → `{}` |
| `*string(nil)` | `null` | 允许，表示字段缺失 |

前端收到 `null` 数组/对象会导致 `.length` / `v-for` 报错，必须从后端根源解决。

### 列表查询

- 本项目配置数量有限（每类 < 100 条），列表接口**不做分页**，一次返回全部
- 列表接口返回格式统一为 `{"items": [...]}`，空列表返回 `{"items": []}`（不是 `null`）

### 错误响应格式

统一返回 `{"error": "中文描述"}`，HTTP 状态码语义：

| 场景 | 状态码 |
|------|--------|
| 参数缺失 / 格式错误 | 400 |
| 资源不存在 | 404 |
| 名称重复 | 409 |
| 校验失败（引用不存在、条件非法） | 422 |
| 服务端内部错误 | 500 |

500 响应 body 中**禁止**包含 Go error 原文，只返回"服务器内部错误，请联系开发人员"。原始 error 写入 slog。

### 请求体大小限制

HTTP body 上限 1MB。防止恶意或误操作提交超大 JSON 导致内存问题。

## 经验沉淀（从游��服务端继承）

| 教训 | 来源 | 应用��运营平台 |
|------|------|--------------|
| 路径穿越 | ���戏服务端客户端输���拼文件路径 | 所有用户输入必须���验，不直接用于查询构造 |
| 死配置 | mongo_uri 存在但代码不用 | 添��配置���时必须有对应实��� |
| nil slice → JSON null | Go nil slice 序列化为 null | API 响应中 slice 必须 `make` 初始化 |
| JSON int/float 丢失 | `json.Unmarshal` 到 `any` | 写入 MongoDB 时用 `bson.UnmarshalExtJSON` 保留类型 |
| 构建期校验 > 运行时 panic | BT key 运行时才报错 | ��置保存时立即校���，不等���戏服务端启动才发现错误 |
| typed nil ≠ nil interface | 返�� typed nil 导致 err!=nil 判断���常 | error 返回值直接 `return nil`，不���回 typed nil ��针 |
| MongoDB 操作必须带超时 | 网络异常时无超时 context 导致 handler 永久挂起 | 所有 DB 操作用 `context.WithTimeout`，��一 5s |
| handler 写错误后忘记 return | 错误响应后继续执行导致二���写入或空指针 | `writeError` 后必须紧跟 `return` |
| omitempty 吞零值 | `severity: 0` 是合法值但被 omitempty 丢弃 | config 字段���加 omitempty，只在明��需要时使用 |
| bson tag 漏写 | 缺 bson tag 导致 MongoDB ��段名变大写开头 | struct 字段必须同��写 `json` 和 `bson` tag |

## 经验沉淀（联调反馈）

| 教训 | 来源 | 应用到运营平台 |
|------|------|--------------|
| 装饰节点 ≠ 复合节点 | inverter 用 `child` 不是 `children`，产出的 JSON 游戏服务端无法加载 | BT 节点必须区分三类：复合（children 数组）、装饰（child 单对象）、叶子（params） |
| 字段类型必须对齐 | `default_severity` 服务端是 float64，运营平台用 int 导致浮点值反序列化失败 | 校验用结构体字段类型必须查游戏服务端源码确认，不能凭猜测 |
| 枚举值必须校验 | FSM op 操作符不校验，无效 op 在服务端静默返回 false，状态转换永远不触发 | 所有枚举型字段（op、policy 等）必须在校验层白名单拦截 |
| 静默降级比报错更危险 | parallel policy 无效值被服务端默认 require_all 兜底，行为与策划预期不一致 | 凡是服务端有默认值兜底的参数，运营平台更应主动校验，而非依赖兜底 |
| Store/Cache 需要 Close() | 优雅关闭时需断开 MongoDB/Redis 连接，否则连接泄漏 | 外部资源的 struct 必须保存 client 引用并暴露 Close() 方法 |
| BB key 未注册 = 生产事故 | set_bb_value 写了未注册 key，服务端加载直接 panic | 后端白名单校验 + 前端下拉选择器，双重拦截 |
| stub_action result 三个值 | result 只支持 success/failure/running，无效值静默降级 | 后端枚举校验 + 前端下拉限定 |
| FSM 状态名可以重复 | states 列表里两个同名状态，stateSet 覆盖不报错，服务端加载报 duplicate | 显式检测 `if stateSet[s.Name]` 重复并报错 |
| condition 字段互斥 | 一个 condition 同时有 key 和 and，key 被静默忽略，属无效配置 | 进入 and/or 分支前检测是否同时有 key，有则拒绝 |
| 自定义工具函数有盲区 | itoa 不处理负数返回空串 | 用标准库 strconv.Itoa 代替自行实现 |
| Docker mongo 端口冲突 | Admin 和 Server 各自 compose 启 mongo，端口都映射 27017 | 文档说明三种共享策略，注释加到 docker-compose.yml |
