# ADMIN 项目开发规则

通用开发规范见 `../standards/dev-rules/`。架构总览见 `../../architecture/overview.md`。

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

后端统一使用中文点分结构化日志：

```go
slog.Debug("handler.创建事件类型", "name", name, "mode", mode)    // handler 层
slog.Info("service.创建事件类型成功", "id", id, "name", name)     // service 层成功
slog.Error("service.创建事件类型失败", "error", err, "name", name) // service 层失败
slog.Warn("service.获取锁失败，降级直查MySQL", "error", lockErr)   // 降级
```

- handler 层 Debug 日志在**校验之后**打印（校验失败的请求不产生日志噪音）
- service 层写操作成功打 `slog.Info`，store 错误打 `slog.Error` + `fmt.Errorf` 包装
- 禁止使用英文 snake_case 日志 key（如 ~~`"service.event_type.create"`~~）

## 跨模块一致性规则

**新模块必须逐层对齐已有模块的代码模式。** EventType 曾因最后开发偏离 Field/Template 模式，导致一次性修 13 处。

### Handler 层一致性

| 维度 | 权威模式 |
|---|---|
| ID 校验 | 调 `util.CheckID(id)`（定义在 `util/validation.go`），消息 `"ID 不合法"` |
| Version 校验 | 调 `util.CheckVersion(version)`，消息 `"版本号不合法"` |
| Required 校验 | 调 `util.CheckRequired(value, fieldName)` |
| 标识符正则 | 引用 `util.IdentPattern`（定义在 `util/strings.go`） |
| 成功消息 | `util.SuccessMsg("保存成功")` / `util.SuccessMsg("操作成功")` |
| slog Debug 时机 | 校验**之后**打印，不在校验前 |
| slog 格式 | 中文点分 `"handler.创建字段"` |
| Update 返回 | `*string` → `util.SuccessMsg("保存成功")` |
| Delete 返回 | `*model.DeleteResult{ID, Name, Label}` |
| ToggleEnabled 返回 | `*string` → `util.SuccessMsg("操作成功")` |
| CheckName 校验 | 调完整的 `h.checkName()` 做正则+长度，不仅检查空 |

### Service 层一致性

| 维度 | 权威模式 |
|---|---|
| 分页校正 | `util.NormalizePagination(&q.Page, &q.PageSize, cfg.DefaultPage, cfg.DefaultPageSize, cfg.MaxPageSize)` |
| List 缓存检查 | `if cached, hit, err := cache.GetList(ctx, q); err == nil && hit` |
| GetByID 缓存检查 | `if cached, hit, err := cache.GetDetail(ctx, id); err == nil && hit` |
| 分布式锁错误 | `slog.Warn("service.获取锁失败，降级直查MySQL", ...)` |
| Store 哨兵错误 | `errors.Is(err, errcode.ErrVersionConflict)` / `errcode.ErrNotFound`（定义在 `errcode/store_errors.go`） |
| Store 错误处理 | `slog.Error` + `fmt.Errorf("xxx: %w", err)`，禁止 raw `return err` |
| ToggleEnabled 签名 | `(ctx, *model.ToggleEnabledRequest) error`（调用方指定目标状态） |
| Delete 返回 | `(*model.DeleteResult, error)` |
| CheckName 成功 | `{Available: true, Message: "该标识可用"}` |
| 日志格式 | 中文动词 `"service.创建状态机"`，禁止英文 dot notation `"service.fsm_config.create"` |

### Store 层一致性

| 维度 | 权威模式 |
|---|---|
| db 字段类型 | 统一 `*sqlx.DB`，禁止用 interface |
| Create 参数 | `(ctx, *model.CreateXxxRequest, ...)` 用请求结构体 |
| Update 参数 | `(ctx, *model.UpdateXxxRequest, ...)` 用请求结构体 |
| 禁止位置参数 | ~~`Create(ctx, name, displayName, mode string, ...)`~~ |
| LIKE 转义 | 调 `util.EscapeLike()`，不在 store 包内定义 wrapper |
| 哨兵错误 | 返回 `errcode.ErrVersionConflict` / `errcode.ErrNotFound`（不在 store 包内定义） |

### Redis Cache 层一致性

| 维度 | 权威模式 |
|---|---|
| 文件命名 | `{module}_cache.go`（如 `field_cache.go`、`fsm_config_cache.go`） |
| 常量/key 引用 | 从 `store/redis/config` 子包导入（`rcfg.NullMarker`、`rcfg.TTL()`、`rcfg.FieldDetailKey()` 等） |
| 日志前缀 | `"cache.{模块中文名}{动作}"`（如 `"cache.字段详情未命中"`、`"cache.状态机释放锁失败"`） |
| TryLock 错误 | `fmt.Errorf("{module} try lock: %w", err)`（如 `"field try lock"`） |
| 方法集 | GetDetail / SetDetail / DelDetail / getListVersion / GetList / SetList / InvalidateList / TryLock / Unlock |

### Frontend API 层一致性

| 维度 | 权威模式 |
|---|---|
| ListData 类型 | 从 `fields.ts` 导入 `ListData<T>`，不重复定义 |
| CheckNameResult | 从 `fields.ts` 导入，不重复定义 |
| delete 返回 | `ApiResponse<{ id: number; name: string; label: string }>` |
| toggleEnabled 参数 | `(id, enabled, version)` 显式传 `enabled` |
| 错误码 | 全部使用命名常量（`FIELD_ERR.xxx` / `EVENT_TYPE_ERR.xxx`），禁止魔法数字 |

### Frontend 列表页一致性

| 维度 | 权威模式 |
|---|---|
| 版本冲突后 | `ElMessageBox.alert` + `fetchList()` 刷新列表 |
| 版本冲突检测 | 用命名常量 `XXX_ERR.VERSION_CONFLICT`，不用 `40010` |

### Frontend 表单页一致性

| 维度 | 权威模式 |
|---|---|
| version 存储 | `const version = ref(0)`，`onMounted` 赋值 |
| version 读取 | `version.value`，禁止 `detail.value!.version` 非空断言 |
| reloadPool 排序 | 必须与初始加载保持相同排序 |

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

Windows 上 `jq -r` 输出带 `\r`（CR），导致 bash 字符串比较失败。**所有** jq 提取（不仅是 assert 内，包括 ID 提取、version 提取等）都必须 `| tr -d '\r'`。遗漏一处就会导致 JSON 拼接出 `{"id":3\r}`，curl 报 `40000 请求参数格式错误`，且在首次运行（无缓存干扰）时才暴露。

### 测试环境重置（Phase 0）

**每次运行测试前必须完成以下重置**（脚本已自动执行）：

1. **Redis FLUSHALL**：清除上一次运行残留的 detail/list 缓存。不清会导致 MySQL 已重建但 Redis 返回旧数据（enabled=true, version=2 等脏数据）
2. **DROP + CREATE 业务表**：fields、field_refs、templates、event_types、event_type_schema — 这些表测试会写入数据，必须每次重建以重置 AUTO_INCREMENT
3. **保留字典表**：dictionaries 只有种子数据，CREATE IF NOT EXISTS 后检查行数 > 0 即跳过 seed，加速二次运行
4. **先 seed 后重启后端**：DictCache / SchemaCache 在后端启动时一次性加载。如果先启动后端再 seed，缓存为空，所有类型校验返回 `40003 字段类型不存在`
5. **等待后端就绪**：重启后轮询 `/health` 直到返回 `{"status":"ok"}`

### 测试脚本编写规范

- **所有 ID 提取必须 `| tr -d '\r'`**：`ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')`，不能省略
- **辅助函数内联定义时注意转义**：bash `cat << 'EOF'` 和 `echo "..."` 对 `$` 的处理不同。直接写入文件用 heredoc，动态拼接用 `echo`
- **helper 函数错误不要吞**：`fld_enable` / `tpl_disable` 等 helper 的 `> /dev/null` 只压输出不压逻辑，如果 toggle 失败后续断言会级联失败且无日志。调试时去掉 `> /dev/null`
- **断言错误码要对准 errcode/codes.go**：`41001` 是 NameExists，`41002` 是 NameInvalid，搞混会导致测试假绿或假红
- **测试 ID 不要跨 section 复用**：每个 section 用自己创建的 ID，不依赖其他 section 的残留状态

### Docker initdb.d

`docker-entrypoint-initdb.d` 只在数据卷首次初始化时执行。修改迁移文件后必须手动执行或重建数据卷。

## 前端 UI 一致性规则

**以下规则在扩展字段 Schema 前端开发中总结，适用于所有列表页和表单页。**

### 术语统一

| 概念 | 统一用语 | 禁止变体 |
|------|----------|----------|
| 禁用操作 | 「禁用」 | ~~停用~~ |
| 标识符列名 | 「XX标识」（字段标识 / 模板标识 / 事件标识） | ~~标识符~~（无前缀） |
| 中文名列名 | 「中文标签」 | ~~中文名~~ ~~中文名称~~ |
| 筛选启用状态 | placeholder=「启用状态」 | ~~状态~~ |
| 类型 tag | 中文（整数 / 浮点数 / 文本 / 布尔 / 选择 / 视觉 / 听觉 / 全局） | ~~English raw values~~ |

### 列表页一致性

- **排序**：所有列表按 ID 倒序（后端分页 `ORDER BY id DESC`；无分页列表前端 `.sort((a, b) => b.id - a.id)`）
- **禁用行透明度**：`:deep(.row-disabled td:not(:nth-last-child(-n+3)))` — 排除最后 3 列（启用开关 + 创建时间 + 操作），使启用开关可交互
- **空数据**：统一用 `el-empty` + 引导按钮
- **创建成功提示**：「创建成功，XX默认为禁用状态，确认无误后请手动启用」（Schema 默认启用除外）

### EnabledGuardDialog 扩展模式

新增配置类型时，在 `EnabledGuardDialog.vue` 中：
1. `EntityType` union 加新类型
2. `entityTypeLabel` / `refTargetLabel` / `reasonText` 各加一个分支
3. `onActOnce` 加 API 调度（获取 version → toggle）
4. 编辑跳转路径加新分支
5. 版本冲突码加新分支

### 无 detail API 的列表模块

数据量 < 100 且无分页的模块（如扩展字段 Schema），可以不提供 detail 接口：
- 编辑/查看页通过 `list()` 全量获取 → 按 ID 找目标项
- 无 checkName 接口时，blur 只做本地格式校验，提交时处理 42020 错误码

### 禁用但有值的扩展字段

事件类型 detail 接口必须返回「启用的 schema + config 中有值但 schema 已禁用的」。前端对禁用字段整行变灰 + 「已禁用」tag + 控件 disabled。

### 自定义约束组件 validate() 模式

自定义约束组件（如 NumberConstraint、SelectConstraint）必须通过 `defineExpose({ validate })` 暴露 `validate(): string | null` 方法。父表单在提交前调用 `constraintRef.value?.validate()`，返回非 null 则阻止提交并显示错误。此模式用于捕获 el-form rules 无法表达的逻辑错误（min > max、空选项列表、重复值等）。

### Toggle 操作预取版本号

所有列表页的 toggleEnabled 操作必须在调用 API 前先 `getDetail(id)` 获取最新 `version`，不能直接使用列表行数据的 `row.version`。列表数据可能在加载后被其他操作修改，使用过期 version 会导致 VERSION_CONFLICT。

### CSS 类名跨模块一致性

所有表单页必须使用统一的 CSS 类名：表单底部操作区用 `.form-actions`（不是 `.form-footer`），警告提示用 `.field-warn`（`margin-top: 4px`）。新增页面前先审查已有模块的命名，禁止引入同义不同名的 class。
