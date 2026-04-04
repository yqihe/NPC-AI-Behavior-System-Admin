# 开发规则

## 协作方请求处理流程

收到姐妹项目（游戏服务端/Unity 客户端）的需求或架构变更请求时：

1. **先回复**：确认收到、表明可行性、说明计划
2. **同步文档**：将架构决策写入 red-lines / dev-rules / CLAUDE.md / spec
3. **提交当前代码**：保证干净的工作区
4. **走正式流程**：/spec-create 规划 → /spec-execute 实现

## 日志格式

后端统一使用结构化日志：

```go
slog.Info("handler.create_event", "name", name, "severity", severity)
slog.Warn("validator.error", "collection", "fsm_configs", "name", name, "err", err)
```

## 文档同步

**强制规则：代码改动和文档更新必须在同一步骤完成。**

改代码时同步：当前 spec 的 `requirements.md` / `design.md` / `tasks.md`。

改完代码检查：`CLAUDE.md`、`red-lines.md`、`dev-rules.md`、`INDEX.md`。

## Git 规则

- 每个需求创建 feature 分支：`feature/<spec-name>`
- commit message 格式：`类型(范围): 描述`
  - 类型：`feat` / `fix` / `test` / `refactor` / `docs` / `chore`
  - 范围：`backend/handler`、`frontend/views`、`backend/store` 等

### 提交即推送

**每次 commit 后必须考虑是否推送到远端。** 默认行为：commit 完成后立即 `git push`。以下情况例外：
- 在 feature 分支上且尚未准备好 review → 可以暂缓
- 明确有后续 commit 要一起推 → 可以攒几个一起推

在 main 分支上直接工作时，commit 后必须立即推送，不允许积压本地 commit。

## CRUD 通用规则

### Name 唯一性

`name` 是各 collection 的业务主键。创建时用 MongoDB unique index 保证，不能"先查后插"（竞态）。

### 写操作

UPDATE 使用 `ReplaceOne` 整体替换（PUT 语义）。

### 空值处理

| Go 类型 | JSON | 要求 |
|---------|------|------|
| `[]T(nil)` | `null` | 必须 `make([]T, 0)` → `[]` |
| `map[string]T(nil)` | `null` | 必须 `make(map[string]T)` → `{}` |

### 列表查询

配置数量有限（每类 < 100），不分页。返回格式：`{"items": [...]}`，空列表 `{"items": []}`。

### 错误响应

统一 `{"error": "中文描述"}`。状态码：400 参数错误 / 404 不存在 / 409 重复 / 422 校验失败 / 500 内部错误。

### 请求体大小

HTTP body 上限 1MB。

## Docker 构建与运行

```bash
docker compose up --build       # 启动全部
docker compose up --build -d    # 后台启动
docker compose down             # 停止
```

## 经验沉淀指引

发现新规则/新坑时，按类型添加到对应文档：

| 发现类型 | 添加到 |
|----------|--------|
| 通用禁令 | `docs/standards/red-lines.md` |
| Go 语言禁令 | `docs/standards/go-red-lines.md` |
| 前端禁令 | `docs/standards/frontend-red-lines.md` |
| ADMIN 架构禁令 | `docs/architecture/red-lines.md` |
| Go 陷阱 | `docs/development/go-pitfalls.md` |
| 前端陷阱 | `docs/development/frontend-pitfalls.md` |
| Skill 流程缺陷 | 对应的 `.claude/commands/*.md` |
