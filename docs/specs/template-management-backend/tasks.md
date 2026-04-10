# 模板管理后端 — 任务拆解

> 对应 [requirements.md](requirements.md) 验收标准 R1-R30，[design.md](design.md) 设计方案。
> 严格遵守 [dev-rules.md "分层职责"](../../development/dev-rules.md#分层职责硬性规定) 硬性规则。

每个任务原则：1-3 个文件、单一明确产出、按依赖顺序排列、做完即可 `/verify`。

---

## 任务依赖关系

```
T1 (errcode) ──┐
T2 (model)   ──┤
T3 (keys)    ──┼──→ T4 (TemplateStore) ──→ T5 (TemplateCache) ──→ T6 (TemplateService)
                │                                                       │
                └───────────────────────→ T7 (FieldService 扩展) ───────┤
                                                                        ↓
                                                              T8 (TemplateHandler)
                                                                        ↓
                                                              T9 (FieldHandler 改造)
                                                                        ↓
                                                              T10 (router + main.go 装配)
                                                                        ↓
                                                              T11 (集成 smoke 测试)
```

每完成一个任务，跑 `/verify` 通过后才能启动下一个。

---

## T1：定义模板错误码 ✅

**目标**：在 `errcode/codes.go` 追加 11 个模板管理错误码常量与对应中文消息。

**文件**：
- `backend/internal/errcode/codes.go`（改动）

**做完了是什么样**：
- 文件里追加 `// --- 模板管理 410xx ---` 段落，定义 `ErrTemplateNameExists` ~ `ErrTemplateVersionConflict` 共 11 个常量（41001-41011）
- `messages` map 追加 11 条对应中文消息
- `go build ./backend/...` 通过

**验收**：R3 部分

---

## T2：定义模板数据模型 + 字段精简结构 ✅

**目标**：在 `model/` 下定义模板相关结构体，并为字段管理追加 `FieldLite` 给跨模块用。

**文件**：
- `backend/internal/model/template.go`（新增）
- `backend/internal/model/field.go`（追加 `FieldLite` 结构体）

**做完了是什么样**：
- `model/template.go` 包含：`Template`、`TemplateFieldEntry`、`TemplateListItem`、`TemplateLite`、`TemplateListData`（含 `ToListData` 方法）、`TemplateDetail`、`TemplateFieldItem`、`TemplateListQuery`、`CreateTemplateRequest`、`CreateTemplateResponse`、`UpdateTemplateRequest`、`TemplateReferenceItem`、`TemplateReferenceDetail`
- `model/field.go` 追加 `FieldLite` 结构体
- 复用现有的 `IDRequest` / `CheckNameRequest` / `CheckNameResult` / `DeleteResult` / `ToggleEnabledRequest` / `ListData`，**不重复定义**
- `templates.fields` 字段在 `Template` 结构体里用 `json.RawMessage`（NOT NULL，安全）
- 所有字段带 `db` tag（用于 sqlx 扫描）和 `json` tag
- `go build ./backend/...` 通过

**验收**：R2 部分

---

## T3：模板 Redis Key 函数 ✅

**目标**：在 `store/redis/keys.go` 追加模板相关 key 前缀和生成函数。

**文件**：
- `backend/internal/store/redis/keys.go`（改动）

**做完了是什么样**：
- 追加常量 `prefixTemplateList` / `prefixTemplateDetail` / `prefixTemplateLock` / `templateListVersionKey`
- `templateListVersionKey` 用小写（包内可见，不导出）
- 追加导出函数 `TemplateListKey(version, label, enabled, page, pageSize)` / `TemplateDetailKey(id)` / `TemplateLockKey(id)`
- `TemplateListKey` 处理 `enabled *bool` 三态：`nil → "*"`、`true → "1"`、`false → "0"`
- `go build ./backend/...` 通过

**验收**：R16 R17 R18 部分

---

## T4：TemplateStore（MySQL 表 DAO） ✅

**目标**：实现 `templates` 表的所有 CRUD 方法。**只操作 templates 表，不读写其它表。**

**文件**：
- `backend/internal/store/mysql/template.go`（新增）

**做完了是什么样**：
- `TemplateStore` 结构体 + `NewTemplateStore(db)` 构造函数
- `DB() *sqlx.DB` 暴露连接（service 层用）
- 实现所有方法（签名见 design.md 第 5 节）：
  - `CreateTx(ctx, tx, req, fieldsJSON) (int64, error)`
  - `GetByID(ctx, id) (*model.Template, error)`
  - `ExistsByName(ctx, name) (bool, error)`
  - `List(ctx, q) ([]TemplateListItem, int64, error)` —— 走覆盖索引 + escapeLike
  - `UpdateTx(ctx, tx, req, fieldsJSON) error` —— 乐观锁，rows=0 返回 `ErrVersionConflict`
  - `SoftDeleteTx(ctx, tx, id) error`
  - `ToggleEnabled(ctx, id, enabled, version) error` —— 乐观锁
  - `IncrRefCountTx(ctx, tx, id) error`
  - `DecrRefCountTx(ctx, tx, id) error`
  - `GetRefCountTx(ctx, tx, id) (int, error)` —— FOR SHARE
  - `GetByIDs(ctx, ids) ([]TemplateLite, error)` —— sqlx.In
- 复用现有的 `escapeLike` / `ErrVersionConflict` / `ErrNotFound`
- `go build ./backend/...` 通过

**验收**：R5 R6 R10 R22 R25 部分

---

## T5：TemplateCache（Redis 缓存层） ✅

**目标**：实现模板的 Redis 缓存，**只缓存 templates 裸行**，不掺字段补全。

**文件**：
- `backend/internal/store/redis/template.go`（新增）

**做完了是什么样**：
- `TemplateCache` 结构体 + `NewTemplateCache(rdb)` 构造函数
- 包内常量：TTL base/jitter（沿用字段管理 5min/30s, 1min/10s 数值），`nullMarker`，`lockExpire`
- 实现方法（签名见 design.md 第 6 节）：
  - `GetDetail(ctx, id) (*model.Template, bool, error)` —— 返回裸行
  - `SetDetail(ctx, id, tpl)` —— 含 nil 写空标记
  - `DelDetail(ctx, id)`
  - `GetList(ctx, q) (*TemplateListData, bool, error)` —— 走 version key
  - `SetList(ctx, q, data)`
  - `InvalidateList(ctx)` —— INCR version key
  - `TryLock(ctx, id, expire) (bool, error)`
  - `Unlock(ctx, id)`
- DEL/Unlock 必须检查 error 并 slog 告警
- `go build ./backend/...` 通过

**验收**：R16 R17 R18 R19 R21 部分

---

## T6：TemplateService（模板模块业务逻辑）

**目标**：实现 TemplateService。**只调用自己模块的 store/cache，不持有任何字段管理组件**。

**文件**：
- `backend/internal/service/template.go`（新增）

**做完了是什么样**：
- 结构体只持有 `templateStore` / `templateCache` / `pagCfg`，**不含 fieldStore/fieldRefStore/fieldCache/dictCache**
- 构造函数 `NewTemplateService(store, cache, pagCfg)`
- 实现单模块方法：
  - `List(ctx, q)` —— Cache-Aside，`make([]TemplateListItem, 0)` 防 nil
  - `GetByID(ctx, id) (*model.Template, error)` —— TryLock + double-check + 空标记防穿透，**返回裸行**
  - `ExistsByName(ctx, name)`
  - `CheckName(ctx, name)`
  - `ToggleEnabled(ctx, req)` —— 乐观锁错误转换
- 实现对外方法（供 handler 跨模块编排）：
  - `CreateTx(ctx, tx, req, fieldsJSON) (int64, error)` —— **service 层做模板自身的业务校验**：fields 非空 41004、field_id 去重防御性校验、name 唯一性 41001
  - `UpdateTx(ctx, tx, req, fieldsJSON, oldVersion) error` —— **service 层做**：fields 非空 / field_id 去重 / 乐观锁错误转换；**不做** ref_count > 0 时的 fields 锁死校验（这个由 handler 层用 GetByID 拿到 ref_count 后判断）
  - 等等 —— 这里要重新考虑：ref_count > 0 时锁死字段变更属于"模板自身的业务规则"，应该在 service 层。但 service 拿不到对比新旧 fields 所需的"diff 是否变更"判断，这个 diff 是模板自身的逻辑。**结论**：把 diff 算法和 41008 校验放在 TemplateService 里，UpdateTx 接收 oldVersion 和 oldFields 作为参数，由 handler 在事务前查好传入。
  - 修订签名：`UpdateTx(ctx, tx, req, oldEntries, fieldsJSON) error` —— oldEntries 由 handler 从 GetByID 拿到的 tpl 解析后传入，service 内部做 diff + 41008 校验
- 实现：
  - `SoftDeleteTx(ctx, tx, id) error`
  - `GetRefCountForDeleteTx(ctx, tx, id) (int, error)` —— 内部调 store.GetRefCountTx
  - `ParseFieldEntries(raw json.RawMessage) ([]TemplateFieldEntry, error)` —— 公开方法供 handler 解 fields JSON
  - `InvalidateDetail(ctx, id)`
  - `InvalidateList(ctx)`
  - `GetByIDsLite(ctx, ids) ([]TemplateLite, error)`
- 所有 slog Debug/Info/Error 一致风格
- `go build ./backend/...` 通过

**验收**：R5-R11 R14 R16-R21 R28 部分

> ⚠️ 任务边界：T6 完成后 service 层是孤立的，**没有调用方**，因此无法跑业务测试。先 build 通过即可，端到端测试在 T11。

---

## T7：FieldService 扩展 5 个对外方法

**目标**：在 `service/field.go` 追加 5 个跨模块方法 + 删除 GetReferences 内的占位 label 逻辑（已由用户手动完成）。

**文件**：
- `backend/internal/service/field.go`（改动）

**做完了是什么样**：
- 追加方法（签名见 design.md 第 8 节）：
  - `ValidateFieldsForTemplate(ctx, fieldIDs []int64) error`
    - 批量调 `fieldStore.GetByIDs(fieldIDs)`
    - 校验全部存在 → 否则返回 `errcode.ErrTemplateFieldNotFound (41006)`
    - 校验全部 enabled=1 → 否则返回 `errcode.ErrTemplateFieldDisabled (41005)`
  - `AttachToTemplateTx(ctx, tx, templateID, fieldIDs) ([]int64, error)`
    - 对每个 fieldID 调 `fieldRefStore.Add(tx, fieldID, RefTypeTemplate, templateID)` + `fieldStore.IncrRefCountTx(tx, fieldID)`
    - 返回 fieldIDs 副本（用于 handler 清缓存）
  - `DetachFromTemplateTx(ctx, tx, templateID, fieldIDs) ([]int64, error)`
    - 对每个 fieldID 调 `fieldRefStore.Remove(tx, ...)` + `fieldStore.DecrRefCountTx(tx, fieldID)`
    - 返回 fieldIDs 副本
  - `GetByIDsLite(ctx, fieldIDs) ([]model.FieldLite, error)`
    - 调 `fieldStore.GetByIDs(fieldIDs)`，转 `[]FieldLite`
    - 用 `dictCache.GetLabel("field_category", category)` 翻译 `CategoryLabel`
    - **保持 fieldIDs 顺序对齐**：缺失的位置补 zero `FieldLite{ID: 0}` 占位（handler 拼装时识别 ID=0 跳过）
  - `InvalidateDetails(ctx, fieldIDs []int64)`
    - 循环调 `fieldCache.DelDetail(ctx, fieldID)`
- **不改 GetReferences 内部逻辑**（用户已改完，service 层只返回 ID 不补 label）
- `go build ./backend/...` 通过

**验收**：R5 R6 R7 R12 R13 R23 R26 部分

---

## T8：TemplateHandler（8 个接口 + 跨模块编排）

**目标**：实现模板管理的 HTTP handler，跨模块事务在这里开启。

**文件**：
- `backend/internal/handler/template.go`（新增）
- `backend/internal/handler/field.go`（小改动：把 `namePattern` 改名为 `identPattern`，供 templates 复用）

**做完了是什么样**：
- 把 `handler/field.go` 顶部的 `var namePattern` 重命名为 `var identPattern`，全文件引用同步替换
- 新建 `handler/template.go`：
  - 结构体 `TemplateHandler { db, templateService, fieldService, valCfg }`
  - 构造函数 `NewTemplateHandler(db, ts, fs, vc)`
  - 校验 helper：`checkTemplateName` / `checkTemplateLabel` / `checkDescription` / `checkFields`（fields 非空 + field_id > 0 + 去重）
  - 实现 8 个 handler 方法：
    - `List(ctx, q) (*ListData, error)` —— 单模块直转
    - `Create(ctx, req) (*CreateTemplateResponse, error)` —— 跨模块事务（流程见 design.md 第 9 节 handler.Create）
    - `Get(ctx, req) (*TemplateDetail, error)` —— 跨模块拼装：调 `templateService.GetByID` 拿裸行 → `ParseFieldEntries` → `fieldService.GetByIDsLite` 拿字段 → handler 拼 `TemplateDetail`
    - `Update(ctx, req) (*string, error)` —— 跨模块事务（流程见 design.md 第 9 节 handler.Update）
    - `Delete(ctx, req) (*DeleteResult, error)` —— 跨模块事务（流程见 design.md 第 9 节 handler.Delete）
    - `CheckName(ctx, req) (*CheckNameResult, error)` —— 单模块直转
    - `GetReferences(ctx, req) (*TemplateReferenceDetail, error)` —— 单模块（NPC 占位返回空数组）
    - `ToggleEnabled(ctx, req) (*string, error)` —— 单模块直转
- 所有 handler 内：
  - 输入校验失败立即 return，不混调 service
  - 跨模块事务路径：`tx, err := h.db.BeginTxx; defer tx.Rollback(); ...; tx.Commit(); 清缓存`
  - 详情拼装：fieldLites 中 ID=0 的位置 slog.Warn + 跳过（不能 silent skip）
  - slog.Debug 入参，slog.Info 成功，slog.Error 失败
- `go build ./backend/...` 通过

**验收**：R1 R2 R3 R4 R5-R29 大部分

---

## T9：FieldHandler.GetReferences 替换占位 label

**目标**：把用户当前用 `fmt.Sprintf("模板#%d", ...)` 兜底的占位逻辑替换成调 `templateService.GetByIDsLite`。

**文件**：
- `backend/internal/handler/field.go`（改动）

**做完了是什么样**：
- `FieldHandler` 结构体追加 `templateService *service.TemplateService` 字段
- `NewFieldHandler` 构造函数追加 `templateService` 参数
- `GetReferences` 方法体改造：
  - 调用 `fieldService.GetReferences` 拿 detail
  - 提取 `templateIDs := []int64{}`（来自 `detail.Templates`）
  - 若 `len(templateIDs) > 0`：调 `templateService.GetByIDsLite(ctx, templateIDs)`，拼 labelMap，回填 `detail.Templates[i].Label`
  - 删除 `fmt.Sprintf("模板#%d", ...)` 占位逻辑
  - 删除文件顶部 import "fmt"（如果不再使用）
  - 删除文件顶部的 TODO 注释（已经不再 TODO）
- `go build ./backend/...` 通过

**验收**：R23

---

## T10：装配链路（router + main.go）

**目标**：把所有新组件接入 main.go 启动流程，注册路由。

**文件**：
- `backend/internal/router/router.go`（改动）
- `backend/cmd/admin/main.go`（改动）

**做完了是什么样**：
- `router/router.go`：
  - `Setup` 函数签名追加 `th *handler.TemplateHandler`
  - 在 v1 下注册 `/api/v1/templates/{list,create,detail,update,delete,check-name,references,toggle-enabled}` 8 个 POST 路由
- `cmd/admin/main.go`：
  - 装配 `templateStore := storemysql.NewTemplateStore(db)`
  - 装配 `templateCache := storeredis.NewTemplateCache(rdb)`
  - 装配 `templateService := service.NewTemplateService(templateStore, templateCache, &cfg.Pagination)`
  - 装配 `templateHandler := handler.NewTemplateHandler(db, templateService, fieldService, &cfg.Validation)`
  - 修改 `fieldHandler := handler.NewFieldHandler(fieldService, templateService, &cfg.Validation)` —— 注入 templateService
  - 修改 `router.Setup(r, fieldHandler, dictHandler, templateHandler)`
- `go build ./backend/...` 通过
- `docker compose up --build admin-backend` 启动成功，`/health` 返回 ok

**验收**：R1 装配验证

---

## T11：集成 smoke 测试

**目标**：用 bash 脚本端到端跑通 8 个接口，覆盖关键失败路径。

**文件**：
- `backend/scripts/smoke_template.sh`（新增）

**做完了是什么样**：
- 脚本依赖 `backend/scripts/` 已有的 curl helper（参考字段管理的 smoke 脚本风格）
- 顺序覆盖：
  1. 准备：先创建 3 个启用字段（用 fields/create），记录 ID
  2. `templates/check-name` 一个未用过的 name → available=true
  3. `templates/create` 用上述 3 个字段创建模板 → 拿 templateID
  4. `templates/check-name` 同一个 name → available=false（含软删除前提）
  5. `templates/create` 用一个不存在字段 ID → 41006
  6. `templates/create` 用一个停用字段 → 41005
  7. `templates/list` 默认查询 → 命中刚创建的模板
  8. `templates/list` enabled=true → 不命中（默认 enabled=0）
  9. `templates/detail` 查 templateID → fields 数组顺序 = 创建时顺序，category_label 已翻译
  10. `templates/update` 启用中模板 → 41010（先 toggle 启用再 update）
  11. `templates/toggle-enabled` 启用 → 检查 fields.ref_count 已经在创建时各 +1
  12. `templates/toggle-enabled` 停用 → 然后 update fields 顺序 + required → 200
  13. `templates/update` 一个不存在字段 → 41006
  14. `templates/delete` 启用中 → 41009
  15. `templates/delete` 停用后 → 200，检查 fields.ref_count 已减回去
  16. `fields/references` 查询其中一个字段 → 模板 label 已正确回填（**T9 验证**）
  17. 清理：删除测试字段
- 脚本可以 `bash backend/scripts/smoke_template.sh` 一键跑通，全部 assert 通过 → 退出码 0
- 脚本失败时 echo 出错点 + 服务端 log

**验收**：R1-R30 端到端

> ⚠️ 此任务依赖 docker compose 起好的开发环境（admin-backend + MySQL + Redis）。

---

## 任务清单总览

| 任务 | 文件数 | 文件 | 验收 R# |
|------|------|------|--------|
| T1 错误码 | 1 | `errcode/codes.go` | R3 |
| T2 模型 | 2 | `model/template.go`, `model/field.go` | R2 |
| T3 Redis Key | 1 | `store/redis/keys.go` | R16-R18 |
| T4 TemplateStore | 1 | `store/mysql/template.go` | R5 R6 R10 R22 R25 |
| T5 TemplateCache | 1 | `store/redis/template.go` | R16-R21 |
| T6 TemplateService | 1 | `service/template.go` | R5-R11 R14 R16-R21 R28 |
| T7 FieldService 扩展 | 1 | `service/field.go` | R5 R6 R7 R12 R13 R23 R26 |
| T8 TemplateHandler | 2 | `handler/template.go`, `handler/field.go`(rename) | R1-R4 R5-R29 |
| T9 FieldHandler 改造 | 1 | `handler/field.go` | R23 |
| T10 装配 | 2 | `router/router.go`, `cmd/admin/main.go` | R1 |
| T11 集成测试 | 1 | `scripts/smoke_template.sh` | R1-R30 |

**总计**：14 处文件改动，11 个原子任务，每个任务 ≤ 3 个文件。

---

## 执行顺序硬约束

```
T1 ──→ T2 ──→ T3 ──→ T4 ──→ T5 ──→ T6 ─┐
                                          ├──→ T8 ──→ T9 ──→ T10 ──→ T11
                                T7 ───────┘
```

- T1-T3 是底层类型/常量定义，可以并行做但建议顺序避免 git 冲突
- T4 依赖 T1 T2 T3
- T5 依赖 T1 T2 T3
- T6 依赖 T4 T5
- T7 与 T1-T6 并行，但要等 T1（错误码）完成（用到 41005/41006）
- T8 依赖 T6 + T7
- T9 依赖 T6（templateService.GetByIDsLite）
- T10 依赖 T8 T9
- T11 依赖 T10 + 完整服务启动

每完成一个任务，跑 `/verify` 通过才能开下一个（按 memory 中的"写完必须先验证"规则）。

---

## 启动 spec-execute 前

完成本 spec-create 流程后，按 `/spec-create` Phase 3 末尾约定：

1. **创建 feature 分支**：`git checkout -b feature/template-management-backend`（从当前 main 拉出）
2. **检查 main 干净**：当前有未提交的字段管理小改动（service/field.go GetReferences 占位删除 + handler/field.go 占位 fallback），要么先 commit 到 main 再切分支，要么连同分支一起带过去
3. 确认 docker compose 开发环境可启动
4. 调用 `/spec-execute T1 template-management-backend` 开始第一个任务
