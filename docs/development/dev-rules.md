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

## 经验沉淀（从游戏服务端继承）

| 教训 | 来源 | 应用到运营平台 |
|------|------|--------------|
| 路径穿越 | 游戏服务端客户端输入拼文件路径 | 所有用户输入必须校验，不直接用于查询构造 |
| 死配置 | mongo_uri 存在但代码不用 | 添加配置项时必须有对应实现 |
| nil slice → JSON null | Go nil slice 序列化为 null | API 响应中 slice 必须 `make` 初始化 |
| JSON int/float 丢失 | `json.Unmarshal` 到 `any` | 写入 MongoDB 时用 `bson.UnmarshalExtJSON` 保留类型 |
| 构建期校验 > 运行时 panic | BT key 运行时才报错 | 配置保存时立即校验，不等游戏服务端启动才发现错误 |
