# 通用开发规则

## 协作方请求处理流程（硬性规则）

收到姐妹项目（游戏服务端/Unity 客户端）的需求或架构变更请求时，**必须按以下顺序执行**：

1. **先回复**：确认收到、表明可行性、说明计划。不回复就动手是禁止行为（见 red-lines）
2. **同步文档**：将架构决策、新规则、新约定写入 red-lines / dev-rules / CLAUDE.md / spec
3. **提交当前代码**：如有未提交的改动，先 commit，保证干净的工作区
4. **走正式流程实现**：用 /spec-create 规划需求，再用 /spec-execute 逐步实现

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

## 经验沉淀（从游戏服务端继承）

| 教训 | 来源 | 应用到运营平台 |
|------|------|--------------|
| 路径穿越 | 游戏服务端客户端输入拼文件路径 | 所有用户输入必须校验，不直接用于查询构造 |
| 死配置 | mongo_uri 存在但代码不用 | 添加配置项时必须有对应实现 |
| nil slice → JSON null | Go nil slice 序列化为 null | API 响应中 slice 必须 `make` 初始化 |
| JSON int/float 丢失 | `json.Unmarshal` 到 `any` | 写入 MongoDB 时用 `bson.UnmarshalExtJSON` 保留类型 |
| 构建期校验 > 运行时 panic | BT key 运行时才报错 | 配置保存时立即校验，不等游戏服务端启动才发现错误 |
| typed nil ≠ nil interface | 返回 typed nil 导致 err!=nil 判断异常 | error 返回值直接 `return nil`，不返回 typed nil 指针 |
| MongoDB 操作必须带超时 | 网络异常时无超时 context 导致 handler 永久挂起 | 所有 DB 操作用 `context.WithTimeout`，统一 5s |
| handler 写错误后忘记 return | 错误响应后继续执行导致二次写入或空指针 | `writeError` 后必须紧跟 `return` |
| omitempty 吞零值 | `severity: 0` 是合法值但被 omitempty 丢弃 | config 字段不加 omitempty，只在明确需要时使用 |
| bson tag 漏写 | 缺 bson tag 导致 MongoDB 字段名变大写开头 | struct 字段必须同时写 `json` 和 `bson` tag |

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

## 经验沉淀（UX 审查反馈）

来源：以非技术用户（策划/运营）视角审查前端页面，发现多处"看不懂"或缺乏防错机制。

| 教训 | 来源 | 应用到运营平台 |
|------|------|--------------|
| 同一数据源的下拉列表必须全局一致 | ConditionEditor 用手动输入 BB Key，BtNodeEditor 用下拉框，二者不一致 | 提取公共常量或共享数组，所有使用 BB Key 的地方统一引用同一来源 |
| 名称含 "/" 会炸路由和 API | BT 名称 `civilian/idle` 导致 Vue Router 匹配失败、后端只拿到 `idle` | API 层 `encodeURIComponent`、路由用 `:name(.*)`、后端 `url.PathUnescape` |
| Go http.ServeMux 自动解码 %2F | 前端 encodeURIComponent 修复后，后端仍报"记录不存在"。Go 标准库 `http.ServeMux` 会把 `%2F` 解码回 `/` 写入 `r.URL.Path`，导致 `Split("/")` 分割错误 | 含 `/` 的资源（如 BT）不能用 `pathName`（取最后一段），必须用 `pathNameAfterPrefix`（从 `r.URL.RawPath` 按固定前缀截取再解码）。修复 URL 编码问题必须端到端验证（前端路由 → API → 后端解析 → MongoDB 查询） |
| 行为树列表无分类 = 找不到 | 所有行为树平铺列表，civilian 和 police 混在一起，数量多了根本找不到 | 按名称中 `/` 前的 NPC 类型分组展示，每组带标题和数量标签 |
| 重复检测不能只靠后端 409 | 用户输入重名后提交，后端返回"名称已存在"但提示位置不明确 | 前端 name 字段 blur 时异步调 list API 检查重复，在输入框下方红字提示 |
| 名称格式必须前端限定 | 用户输入中文、空格、大写字母等，后端返回技术性校验错误 | 正则 `/^[a-z][a-z0-9_]*$/`（BT 允许 `/`），在 blur 时即时校验 |
| FSM 添加重复状态名无提示 | `states.includes(s)` 静默不添加，用户以为操作失败 | 重复时显示红字 `状态 "xxx" 已存在，不能重复添加` |
| 空列表无引导 = 用户卡死 | NPC 表单的 FSM/BT 下拉框为空，用户不知道为什么选不了 | 下拉框为空时显示 `el-alert` + 创建链接，列表页空数据用 `el-empty` + 引导按钮 |
| 删除确认太简略 | "确认删除？"没说删什么、有什么后果 | 明确对象名 + 影响说明，如 `确认删除状态机「civilian」？使用此状态机的 NPC 将受影响。` |
| 技术标签策划看不懂 | `sequence`、`selector`、`check_bb_float` 对非技术用户无意义 | 节点类型用中文标签 + 英文括注，如 `顺序执行 (sequence)` |
| 表单字段缺少说明 | 策划不理解"威胁等级"的数值范围和影响 | 每个字段下方加灰色 hint 文字，用自然语言解释 |
| NPC 未绑定行为树可以保存 | 3 个状态只绑了 2 棵树，保存后游戏服务端加载报错 | 提交前前端检查所有状态是否已绑定行为树，未绑定则列出状态名警告 |

## 经验沉淀（Mailbox 代码审查反哺）

来源：审查姐妹项目 Mailbox 时发现的通用问题，提炼为 ADMIN 平台的预防性规则。

| 教训 | 来源 | 应用到运营平台 |
|------|------|--------------|
| channel close 必须幂等 | Mailbox Hub.Close() 与 Unregister 并发 double-close channel 导致 panic | 如果未来引入 channel 通信，关闭操作必须用 `sync.Once` 保护，或由单一 goroutine 负责关闭 |
| 过滤条件下推到数据库 | Mailbox `since` 参数先全量拉取再内存过滤，随数据增长必然超时 | 所有查询过滤条件（时间范围、状态等）必须在 MongoDB 查询层面完成，不在 Go 层做二次过滤。如果需要新的过滤维度，同步添加对应索引 |
| 前端 filter 不能修改源数据 | Mailbox 前端 `applyClientFilter` 直接覆盖 messages 数组，切换过滤器后数据永久丢失 | Vue 组件中过滤/排序必须使用 `computed` 派生，永远不修改 `ref`/`reactive` 中的原始数据源 |
| ID 生成器避免回绕碰撞 | Mailbox `seq%1000` 在高并发下同一毫秒内回绕导致重复 ID | 如果自行生成 ID（非 MongoDB ObjectId），序号部分不做取模截断，或使用足够大的空间（如 UUID） |
| graceful shutdown 必须捕获 SIGTERM | Mailbox 只监听 os.Interrupt，Docker 发 SIGTERM 时走不到优雅关闭 | ADMIN 已正确使用 `syscall.SIGINT, syscall.SIGTERM`（已验证），此条作为 checklist 保留 |
| 测试数据库必须每次清理 | Mailbox handler 测试未 Drop collection，多次运行导致数据累积影响断言 | 集成测试 setup 中必须清理测试数据库，用独立的 database 名（如 `npc_ai_test`），每个 TestXxx 开头 Drop collection |
| 标准库优先于手写工具函数 | Mailbox 手写 `contains()`，功能等同 `strings.Contains` | 使用标准库函数，除非有明确的性能或功能理由。自写工具函数容易遗漏边界情况（参见已有教训：itoa 不处理负数） |
