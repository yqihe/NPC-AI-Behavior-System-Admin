# ADMIN 项目开发规则

通用开发规范见 `../standards/dev-rules/`。架构总览见 `../../architecture/overview.md`。

## 1. 分层职责（硬性规定）

| 层 | 职责 | 禁止 |
|---|---|---|
| **store** | 单张表 CRUD（一个 store 文件 = 一张表） | 读写其他模块的表 |
| **service** | 同模块业务逻辑、事务、缓存 | 调用其他模块的 store/cache/service |
| **handler** | 请求校验 + **跨模块编排** | 写业务逻辑（handler 只做编排） |

**模块边界**：每个模块拥有自己的表。字段模块拥有 fields + field_refs，模板模块拥有 templates，事件类型模块拥有 event_types，扩展字段模块拥有 event_type_schema + schema_refs，FSM 模块拥有 fsm_configs。

**跨模块事务处理**：
- handler 调 `db.BeginTxx` → 传 `*sqlx.Tx` 给多个 service 的 Tx 版方法 → handler 统一 Commit/Rollback → commit 后清两个模块缓存
- ADMIN 是 HTTP 单体，跨模块事务就是普通 MySQL `BEGIN...COMMIT`，不需要分布式事务

**典型编排场景**：

| 场景 | handler 做什么 |
|---|---|
| 模板创建 | TemplateService.CreateTx + FieldService.AttachToTemplateTx |
| 事件类型创建 | EventTypeService.Create（内部事务写 event_types + schema_refs） |
| FSM 创建 | FsmConfigService.CreateInTx + FieldService.SyncFsmBBKeyRefs |
| 字段引用详情 | FieldService.GetReferences + TemplateService.GetByIDsLite + FsmConfigService.GetByID 补 label |

**例外**：`DictCache`、`EventTypeSchemaCache` 是只读基础设施，可被任意 service 直接调用。

**层内文件夹**：每层文件夹下不允许子文件夹。通用函数放 `util/` 包（`store/redis/config/` 例外，是红线规定的 key 管理子包）。

## 2. 引用系统通用模式

### 2.1 "对新隐藏、对旧保留"原则

所有可被引用的配置都遵循统一的生命周期：

```
创建(disabled) → 启用(对外可见) → 禁用(对新不可见,对旧保留) → 删除(无引用时)
```

| 环节 | 行为 |
|---|---|
| **新建页** | 选择池只展示 enabled=true 的配置 |
| **已有配置编辑页** | 被引用的已禁用配置保留展示（标灰 + "已禁用" tag） |
| **编辑保护** | 有引用时：类型不可改，约束只能放宽（`util.CheckConstraintTightened`） |
| **删除保护** | 必须先禁用 → 检查引用关系表 → 有引用弹详情阻止 → 无引用才允许 |

### 2.2 引用追踪表

用关系表追踪引用，不用冗余计数器（ref_count 已移除）：

| 关系表 | 追踪 | ref_type 值 |
|---|---|---|
| `field_refs(field_id, ref_type, ref_id)` | 字段被谁引用 | `template` / `field` / `fsm` |
| `schema_refs(schema_id, ref_type, ref_id)` | 扩展字段被谁引用 | `event_type` |

ref_type 常量定义在 `util/const.go`。

### 2.3 has_refs

`Field.HasRefs` / `EventTypeSchema.HasRefs` 是运行时实时查 `xxx_refs` 表的 bool 字段（`db:"-"`），**不进缓存**——引用关系随其他模块操作变化。

### 2.4 前端落地规范

前端对齐引用系统的统一做法：

| 场景 | 权威做法 |
|---|---|
| **列表展示** | 不显示"被引用数"列（避免数字频繁查询）。引用信息在删除时才查 |
| **删除流程** | 已禁用 → 先调 `{entity}Api.references(id)` → 有引用弹详情弹窗阻止 → 无引用 ElMessageBox 确认 → delete API；后端 `REF_DELETE` 错误码做兜底重新拉引用 |
| **引用详情弹窗** | 按 ref_type 分区展示（模板/字段/FSM 三块），每块 `<el-table>` + 空态 `<p class="ref-empty">暂无 XX 引用</p>` |
| **表单锁定驱动** | 用后端返回的 `has_refs: boolean` 驱动 UI（不用数字），如 `:disabled="isView \|\| (!isCreate && hasRefs)"` |
| **reference 子字段过滤** | 新建模式过滤停用子字段（`filterDisabled=true`），编辑/查看保留停用（标灰 + "已停用" tag）。通过父组件 `mode` prop 传 `isCreateMode` 控制 |
| **存量标灰** | 模板已选字段/事件类型扩展字段中，若后端返回 `enabled=false`，整行 opacity 0.55 + "已禁用" tag + 控件 disabled |

**EnabledGuardDialog 职责**：只展示"已禁用"一个前置条件，不塞业务特有的引用检查（引用检查在调用方的 handleDelete 里做）。

## 3. 需求处理流程

任何新需求必须先走 `/spec-create` 规划（requirements → design → tasks），不允许直接写代码。协作方请求也要先回复确认再走正式流程。

## 4. 日志格式

```go
slog.Debug("handler.创建事件类型", "name", name)     // handler 层，校验之后打印
slog.Info("service.创建事件类型成功", "id", id)       // service 写操作成功
slog.Error("service.创建事件类型失败", "error", err)  // store 错误（+ fmt.Errorf 包装）
slog.Warn("service.获取锁失败，降级直查MySQL", ...)   // 降级场景
```

中文点分格式，禁止英文 snake_case（~~`service.event_type.create`~~）。

## 5. 跨模块一致性

新模块必须逐层对齐已有模块的代码模式。

### Handler 层

| 维度 | 权威模式 |
|---|---|
| ID/Version/Required 校验 | `util.CheckID()` / `util.CheckVersion()` / `util.CheckRequired()` |
| 标识符正则 | `util.IdentPattern` |
| slog Debug | 校验通过**之后**打印 |
| Update 返回 | `*string` → `util.SuccessMsg("保存成功")` |
| Delete 返回 | `*model.DeleteResult{ID, Name, Label}` |
| ToggleEnabled 返回 | `*string` → `util.SuccessMsg("操作成功")` |

### Service 层

| 维度 | 权威模式 |
|---|---|
| 分页 | `util.NormalizePagination(...)` |
| 缓存读取 | `if cached, hit, err := cache.GetXxx(...); err == nil && hit` |
| Store 错误 | `slog.Error + fmt.Errorf("xxx: %w", err)`，禁止 raw `return err` |
| ToggleEnabled | `(ctx, *model.ToggleEnabledRequest) error` |
| CheckName 成功 | `{Available: true, Message: "该标识可用"}` |

### Store 层

| 维度 | 权威模式 |
|---|---|
| db 字段 | 统一 `*sqlx.DB` |
| Create/Update | `*model.CreateXxxRequest` 结构体，禁止位置参数 |
| 哨兵错误 | `errcode.ErrVersionConflict` / `errcode.ErrNotFound` |
| LIKE | `util.EscapeLike()` |

### Redis Cache 层

文件命名 `{module}_cache.go` → 常量从 `store/redis/config` 导入 → 方法集：GetDetail/SetDetail/DelDetail/GetList/SetList/InvalidateList/TryLock/Unlock

### 前端一致性

- `ListData<T>` / `CheckNameResult` 从 `fields.ts` 导入
- 错误码用命名常量（`XXX_ERR.VERSION_CONFLICT`）
- version 用独立 `ref(0)` 存储，禁止 `detail.value!.version`

## 6. Git 规则

- main 只能 PR Squash Merge，禁止直接 push / force push
- 分支命名 `feature/<spec-name>`
- commit 格式 `类型(范围): 描述`（类型：feat/fix/test/refactor/docs/chore）
- 完成后 push + 创建 PR

## 7. CRUD 通用规则

- **Name**：`UNIQUE KEY uk_name(name)`，含软删除（已删除 name 不可复用）
- **创建**：`version=1, deleted=0, enabled=0`（默认禁用）
- **更新**：乐观锁 `WHERE version = ?`，`version = version + 1`
- **删除**：软删除 `SET deleted=1`
- **空值**：`[]T` 必须 `make([]T, 0)` → `[]`，`map` 同理
- **列表**：后端分页，返回 `{items, total, page, page_size}`
- **响应**：`{code: 0, data, message: "OK"}`，错误码在 code 字段
- **404/405**：router 注册 `NoRoute/NoMethod` 返回统一 JSON（`code=40000`），不让 Gin 默认纯文本兜底

## 7b. 约束自洽校验（约束写入前的统一前置）

字段/扩展字段的 Create/Update 都必须先做 constraints 自洽校验，**在类型存在性校验之后、在 DB 操作之前**：

```go
// service 层（字段模块示例）
if err := s.checkTypeExists(req.Type); err != nil { return err }
if err := s.checkCategoryExists(req.Category); err != nil { return err }
if err := s.validatePropertiesConstraints(req.Type, req.Properties); err != nil { return err }
// ↓ 后续：name 唯一性、reference refs 校验、写 DB...
```

`util.ValidateConstraintsSelf(fieldType, constraints, errCode)` 是唯一入口。errCode 按模块传：

| 模块 | errCode |
|---|---|
| 字段模块 | `errcode.ErrBadRequest`（40000） |
| 扩展字段模块 | `errcode.ErrExtSchemaConstraintsInvalid`（42025） |

覆盖的校验项（全类型）：

| 类型 | 检查 |
|---|---|
| int/integer | min ≤ max |
| float | min ≤ max, precision > 0 |
| string | minLength ≤ maxLength, 非负 |
| bool/boolean | 无约束 |
| select | options 非空, value 不重复, minSelect ≤ maxSelect, minSelect ≥ 0 |
| reference | 不走此函数，走 `validateReferenceRefs`（refs 非空/目标启用/非嵌套/无循环） |

## 7c. check-name 接口前置校验模式

所有 check-name 接口必须先跑 handler 内部 `checkName()`（空/正则/长度），再查 DB：

```go
func (h *XxxHandler) CheckName(ctx, req) (*CheckNameResult, error) {
    if err := h.checkName(req.Name); err != nil { return nil, err }  // 格式校验
    return h.xxxService.CheckName(ctx, req.Name)                      // 查存在性
}
```

**禁止**只做 `util.CheckRequired(req.Name)` 就进 service——会让 `BAD_FORMAT` 这类非法 name 返回"可用"假结果。

## 8. 测试脚本（Windows 环境）

1. 中文传参：`printf '%s' "$body" | curl --data-binary @-`（Windows Git Bash 的 `curl -d` 会把 UTF-8 转 cp936）
2. jq 提取：所有 `jq -r` 后接 `| tr -d '\r'`（Windows CRLF）
3. Phase 0 重置：Redis FLUSHALL → DROP+CREATE 业务表 → 保留字典表 → seed → 重启后端 → 轮询 /health
4. 断言错误码对准 `errcode/codes.go`，测试 ID 不跨 section 复用

## 9. 前端 UI 一致性

- **术语**：禁用（非停用）、XX标识、中文标签、启用状态
- **列表页**：ID 倒序、禁用行 opacity 排除最后 3 列、`el-empty` 空态、创建成功提示"默认为禁用状态"
- **EnabledGuardDialog**：泛型组件，通过 `entityType` 切换文案/API
- **Toggle 操作**：必须先 detail 拿最新 version，不用列表 row.version
- **约束组件**：`defineExpose({ validate })` 暴露 `validate(): string | null`
- **CSS 类名**：`.form-actions`（非 `.form-footer`）、`.field-warn`
- **排序按钮**：纯 `el-icon` ArrowUp/ArrowDown，禁止 `el-button text` + Unicode 箭头

## 10. 文档同步

代码改动和文档更新必须同步。改完检查：spec 文档、CLAUDE.md、red-lines.md、dev-rules.md。

### 两类文档的写作标准

| 类别 | 路径 | 定位 | 写作要求 |
|------|------|------|---------|
| **开发规范** | `docs/development/` | 跨模块通用规则，写代码时按编号查阅 | 表达完整但精简，每个文件控制在 15KB 以内。按编号分点，方便定位单条规则 |
| **模块设计** | `docs/v3-PLAN/{分组}/{模块}/` | 单模块的权威参考，features/backend/frontend 三件套 | 详细不限大小。包含完整 SQL、方法签名、请求响应示例、错误码表、事务流程图等。与代码一一对齐 |

**开发规范**写给"正在写任意模块代码的人"看——需要快速查到某条规则。
**模块设计**写给"正在开发或维护这个具体模块的人"看——需要理解全部细节。
