# 开发规则

## 分层职责（硬性规定）

**三层各司其职，禁止越界。**

| 层 | 职责 | 禁止 |
|---|---|---|
| **store** | 只对自己管理的**单张表**做 CRUD。一个 store 文件 = 一张表 | 不允许在自己的方法里读写其它模块的表 |
| **service** | 编排同模块内的多个 store/cache，处理同模块内的业务逻辑、事务、缓存 | 不允许直接调用其它模块的 store / cache / service |
| **handler** | 校验请求格式 + **跨模块编排**：当一个接口需要协调多个模块（比如模板创建要写 templates + 写 field_refs + 改 fields.ref_count）时，handler 层负责调用多个 service 协同完成 | 不允许把业务逻辑写在 handler 里（handler 只做编排和校验） |

**模块边界定义**：

- **字段管理模块**：拥有 `fields`、`field_refs` 表，对应 `FieldStore`、`FieldRefStore`、`FieldService`
- **模板管理模块**：拥有 `templates` 表，对应 `TemplateStore`、`TemplateService`
- **字典模块**：拥有 `dictionaries` 表，对应 `DictionaryStore`、`DictCache`（只读查询，所有模块都可调用，视为基础设施）

**典型违规与正确做法**：

| 场景 | ❌ 违规 | ✅ 正确 |
|---|---|---|
| 模板创建要写 field_refs 和改 fields.ref_count | TemplateService 直接调 FieldRefStore.Add 和 FieldStore.IncrRefCountTx | Handler 调 TemplateService.Create + FieldService.AttachToTemplate（或类似的"模块对外接口"），跨模块事务在 handler 层开启 |
| 详情接口要补字段精简列表 | TemplateService 直接调 FieldStore.GetByIDs | Handler 先调 TemplateService.GetByID 拿 fields JSON，再调 FieldService.GetByIDsLite 拿字段信息，handler 拼装结果返回 |
| 字段引用详情要展示模板 label | FieldService 直接调 TemplateStore.GetByIDs | Handler 调 FieldService.GetReferences 拿 ID 列表，再调 TemplateService.GetByIDsLite 补 label |
| 模板写操作要清字段方缓存 | TemplateService 持有 FieldCache 引用 | Handler 调 TemplateService 写完后再调 FieldService.InvalidateDetail(fieldIDs) |

**跨模块事务处理**：

handler 层负责跨模块事务时：

- 由 handler 调用 `db.BeginTxx` 开启事务
- 把 `*sqlx.Tx` 作为参数传给两个 service 的对应方法（service 方法签名要支持接受外部 tx）
- handler 统一 `Commit` / `Rollback`
- service 方法既要支持"自己开事务"也要支持"接受外部 tx"，前者用于纯单模块写，后者用于被 handler 编排

**为什么这么严格**：

1. **模块解耦**：service 互相不感知，删除一个模块只需删自己目录，不会破坏其它模块
2. **测试单元化**：每个 service 单测只 mock 自己的 store，不需要 mock 其它模块
3. **依赖方向清晰**：依赖图永远是 handler → service → store，不会出现 service ↔ service 的横向依赖
4. **职责单一**：handler 是"用例编排者"，service 是"模块业务专家"，store 是"表 DAO"，每层只做一件事

> **例外**：`DictCache` 是只读基础设施，可以被任意 service 直接调用（label 翻译是公共能力，不算业务跨模块）。

**关于跨模块事务的成本**：

ADMIN 是 HTTP 单体（不是微服务），所有模块在同一进程、同一 `*sqlx.DB`。handler 层开的跨模块事务在物理上就是一次普通的 MySQL `BEGIN ... COMMIT`，与单 service 内开事务的物理行为完全相同：

- ❌ **不需要** 2PC / TCC / Saga / 补偿事务
- ❌ **不存在** 协调者宕机 / in-doubt 事务 / 网络分区问题
- ✅ `tx.Rollback()` 一行搞定失败回滚
- ⚠️ 仅需注意两点（与单 service 事务一样）：
  1. 事务内不做慢操作（HTTP 调用、长循环、跨库查询）
  2. 多个 handler 路径锁多张表时保持一致的加锁顺序，防死锁

**只有在跨进程 RPC 调用时**才需要分布式事务方案。我们这里不会出现。

## 需求处理流程（硬性规定）

**任何新需求都必须先走 `/spec-create` 规划，不允许直接写代码。**

当用户提出新需求（如"能不能做 XX"、"加个功能"、"改一下 XX"等），Claude 必须：

1. **提醒用户**：先调用 `/spec-create` 进行需求规划
2. **等待规划完成**：spec 产出 requirements / design / tasks 文档
3. **再走执行**：用户调用 `/spec-execute` 后才开始写代码

即使用户没有显式调用 skill，也必须主动提醒。跳过规划直接写代码属于违规操作。

## 协作方请求处理流程

收到姐妹项目（游戏服务端/Unity 客户端）的需求或架构变更请求时：

1. **先回复**：确认收到、表明可行性、说明计划
2. **同步文档**：将架构决策写入 red-lines / dev-rules / CLAUDE.md / spec
3. **提交当前代码**：保证干净的工作区
4. **走正式流程**：/spec-create 规划 → /spec-execute 实现

## Claude Code 权限模式

每个 SKILL 对应推荐的权限模式，Claude 在调用 SKILL 前应提醒用户切换：

| SKILL | 推荐模式 | 原因 |
|-------|----------|------|
| `/spec-create` | `plan` | 只读分析，不该写代码 |
| `/spec-execute` | `auto` | 写代码，allow 列表自动执行 |
| `/verify` | `auto` | 跑构建/测试命令 |
| `/debug` | `auto` | 需要读写代码修复 |
| `/integration` | `ask` | 跨项目操作，需确认每步 |
| 普通对话 | `ask` | 讨论功能，避免误操作 |

切换方式：`/mode auto` / `/mode plan` / `/mode ask`

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

### main 分支保护

- main 分支**禁止直接 push**，只能通过 PR 合并
- main 分支**禁止 force push**
- 所有代码变更必须走 feature 分支 → PR → Squash Merge 流程

### 合并策略

- **仅允许 Squash Merge**（merge commit 和 rebase 已禁用）
- 每个 PR 合并后在 main 上产生一条干净的 commit
- PR 合并后远端分支**自动删除**

### 分支与提交

- 每个需求创建 feature 分支：`feature/<spec-name>`
- commit message 格式：`类型(范围): 描述`
  - 类型：`feat` / `fix` / `test` / `refactor` / `docs` / `chore`
  - 范围：`backend/handler`、`frontend/views`、`backend/store` 等

### 提交即推送

**每次 commit 后必须考虑是否推送到远端。** 默认行为：commit 完成后立即 `git push`。以下情况例外：
- 在 feature 分支上且尚未准备好 review → 可以暂缓
- 明确有后续 commit 要一起推 → 可以攒几个一起推

在 feature 分支上完成所有任务后，必须 push 并创建 PR。

## CRUD 通用规则

### Name 唯一性

`name` 是各表的业务主键。MySQL 通过 `UNIQUE KEY uk_name (name)` 保证唯一性。创建前在 service 层检查是否已存在（含软删除）。

### 写操作

- **创建**：INSERT，初始 `version=1`、`ref_count=0`、`deleted=0`
- **更新**：UPDATE + 乐观锁（`WHERE version = ?`），`version = version + 1`
- **删除**：软删除（`UPDATE SET deleted=1`），不物理删除

### 空值处理

| Go 类型 | JSON | 要求 |
|---------|------|------|
| `[]T(nil)` | `null` | 必须 `make([]T, 0)` → `[]` |
| `map[string]T(nil)` | `null` | 必须 `make(map[string]T)` → `{}` |

### 列表查询

所有列表后端分页（MySQL LIMIT/OFFSET），不做前端全量过滤。返回格式：

```json
{"code": 0, "data": {"items": [...], "total": 100, "page": 1, "page_size": 20}, "message": "OK"}
```

### 统一响应格式

```json
{"code": 0, "data": {...}, "message": "OK"}
```

- `code=0` 成功，`code=40xxx` 业务错误，`code=50000` 内部错误
- HTTP 状态码统一 200，业务错误码在 `code` 字段中
- 错误码定义在 `errcode/codes.go`

### 请求体大小

HTTP body 上限 1MB。

## Docker 构建与运行

```bash
docker compose up --build       # 启动全部
docker compose up --build -d    # 后台启动
docker compose down             # 停止
```

## 经验沉淀指引

发现新规则/新坑时，按技术领域添加到对应文档：

| 发现类型 | 红线（禁令） | 陷阱（踩坑） |
|----------|-------------|-------------|
| 通用 | `standards/red-lines.md` | — |
| Go 语言 | `standards/go-red-lines.md` | `development/go-pitfalls.md` |
| MySQL | `standards/mysql-red-lines.md` | `development/mysql-pitfalls.md` |
| Redis | `standards/redis-red-lines.md` | `development/redis-pitfalls.md` |
| MongoDB | — | `development/mongodb-pitfalls.md` |
| 缓存模式 | `standards/cache-red-lines.md` | `development/cache-pitfalls.md` |
| 前端 | `standards/frontend-red-lines.md` | `development/frontend-pitfalls.md` |
| 后端架构 | `architecture/backend-red-lines.md` | — |
| UI/UX | `architecture/ui-red-lines.md` | — |
| Skill 流程 | — | 对应的 `.claude/commands/*.md` |

## Bash 集成测试脚本（Windows 环境）

### 中文编码

Windows 上 Git Bash 的 `curl -d "$var"` 在变量展开时会破坏 UTF-8 中文字节。必须用管道传输：

```bash
# 错误：中文会乱码
curl -d "$body" ...

# 正确：通过 stdin 管道传输，避免 shell 展开
printf '%s' "$body" | curl --data-binary @- -H "Content-Type: application/json; charset=utf-8" ...
```

### jq 输出 CRLF

Windows 上 `jq -r` 输出带 `\r`（CR），导致 bash 字符串比较失败。所有 assert 函数的 jq 输出必须 `| tr -d '\r'`。

### Docker initdb.d

`docker-entrypoint-initdb.d` 只在数据卷首次初始化时执行。修改迁移文件后必须手动执行或重建数据卷。
