# ref-system-backend — 任务拆解

> 场景组 A（ref_count 清理）大部分已在分支上完成，标记为 ✅ done。
> 从 T6 开始是新增任务。

---

## ✅ T1: 迁移文件去 ref_count (R1)

**文件**：`backend/migrations/001_create_fields.sql`、`backend/migrations/003_create_templates.sql`

**已完成**：fields/templates 表定义无 ref_count 列，idx_list 索引无 ref_count。

---

## ✅ T2: 后端 model 去 RefCount (R1)

**文件**：`backend/internal/model/field.go`、`backend/internal/model/template.go`

**已完成**：Field/FieldListItem/Template/TemplateListItem/TemplateDetail 无 RefCount。Field 新增 HasRefs。

---

## ✅ T3: 后端 store 删 ref_count 方法 + 更新 SQL (R1, R2)

**文件**：`backend/internal/store/mysql/field.go`、`backend/internal/store/mysql/template.go`

**已完成**：Incr/Decr/GetRefCount 删除，所有 SQL 无 ref_count。

---

## ✅ T4: FieldRefStore 新增 HasRefs + service/field 改用 field_refs (R2, R3, R4)

**文件**：`backend/internal/store/mysql/field_ref.go`、`backend/internal/service/field.go`

**已完成**：HasRefs 非事务版新增；GetByID 填充 has_refs；Update 用 HasRefs 驱动类型锁定+约束收紧；Attach/Detach/syncFieldRefs/Delete 无 Incr/Decr。

---

## ✅ T5: service/template + handler/template 移除 ref_count (R2, R5)

**文件**：`backend/internal/service/template.go`、`backend/internal/handler/template.go`

**已完成**：GetRefCountForDeleteTx 删除；Delete 无 ref_count 检查；UpdateInTx 无 RefCount 检查。

---

## [x] T6: 提取 CheckConstraintTightened 到 constraint 包 (R4, R12)

**文件**：
- `backend/internal/service/constraint/tighten.go`（新增）
- `backend/internal/service/field.go`（改为调用 constraint 包）

**做完是什么样**：
- `constraint.CheckConstraintTightened(fieldType, oldConstraints, newConstraints)` 公开函数可用
- 相关辅助函数（`getStringFromRaw`、`parseSelectOptions`）也移到 constraint 包
- `field.go` 中的 `checkConstraintTightened` 改为调用 `constraint.CheckConstraintTightened`
- `go build ./...` 通过

---

## [x] T7: schema_refs 迁移文件 + model + store (R7)

**文件**：
- `backend/migrations/007_create_schema_refs.sql`（新增）
- `backend/internal/model/event_type_schema.go`（加 SchemaRef + HasRefs + ReferenceDetail）

**做完是什么样**：
- `schema_refs` 表迁移文件存在，结构与 field_refs 对齐
- model 中 `SchemaRef`、`SchemaReferenceItem`、`SchemaReferenceDetail` 结构定义完整
- `EventTypeSchema` struct 新增 `HasRefs bool (db:"-")`
- `go build ./...` 通过

---

## [x] T8: SchemaRefStore 实现 (R7)

**文件**：
- `backend/internal/store/mysql/schema_ref.go`（新增）
- `backend/internal/setup/stores.go`（注册）

**做完是什么样**：
- `SchemaRefStore` 包含 Add/Remove/RemoveByRef/HasRefs/HasRefsTx/GetBySchemaID 六个方法
- setup 中注册 `SchemaRef: storemysql.NewSchemaRefStore(db)`
- `go build ./...` 通过

---

## [x] T9: EventType store 新增 Tx 版方法 (R8, R9, R10)

**文件**：
- `backend/internal/store/mysql/event_type.go`

**做完是什么样**：
- 新增 `CreateTx`、`UpdateTx`、`SoftDeleteTx` 三个事务版方法（签名与非事务版一致，接收 `*sqlx.Tx`）
- 原有非事务版保留（List/GetByID 等不需要 Tx）
- `go build ./...` 通过

---

## [x] T10: EventTypeService 改为事务 + 维护 schema_refs (R8, R9, R10)

**文件**：
- `backend/internal/service/event_type.go`

**做完是什么样**：
- `EventTypeService` 新增持有 `schemaRefStore` 和 `fieldStore`（DB 引用，用于开事务）
- `Create`：开事务 → CreateTx → 写 schema_refs → 提交 → 清缓存
- `Update`：解析旧 config_json 提取旧 extension keys → diff → 开事务 → UpdateTx → 增删 schema_refs → 提交 → 清缓存
- `Delete`：开事务 → SoftDeleteTx → RemoveByRef schema_refs → 提交 → 清缓存
- setup 中 NewEventTypeService 注入 schemaRefStore
- `go build ./...` 通过

---

## [x] T11: EventTypeSchemaService 引用保护 (R11, R12, R13, R14, R15, R16)

**文件**：
- `backend/internal/service/event_type_schema.go`
- `backend/internal/errcode/codes.go`（新增错误码）

**做完是什么样**：
- `EventTypeSchemaService` 新增持有 `schemaRefStore`
- `Update`：有引用时调 `constraint.CheckConstraintTightened` 检查约束收紧
- `Delete`：有引用时拒绝删除（新错误码 `ErrExtSchemaRefDelete`）
- 新增 `GetReferences(ctx, id)` 方法
- List 方法填充 `has_refs`（或新增独立方法）
- setup 中注入 schemaRefStore
- `go build ./...` 通过

---

## [x] T12: EventTypeSchema references handler + 路由 (R15)

**文件**：
- `backend/internal/handler/event_type_schema.go`
- `backend/internal/router/router.go`

**做完是什么样**：
- `EventTypeSchemaHandler` 新增 `GetReferences` 方法，跨模块补事件类型 label
- 路由 `POST /event-type-schemas/references` 注册
- `go build ./...` 通过

---

## [x] T13: util 常量 + FieldStore.GetByNames (R17)

**文件**：
- `backend/internal/util/const.go`（新增 `RefTypeFsm`）
- `backend/internal/store/mysql/field.go`（新增 `GetByNames`）

**做完是什么样**：
- `util.RefTypeFsm = "fsm"` 常量存在
- `FieldStore.GetByNames(ctx, names []string) ([]model.Field, error)` 批量按 name 查字段（走 uk_name）
- `go build ./...` 通过

---

## [x] T14: FieldService.SyncFsmBBKeyRefs (R17, R18, R19)

**文件**：
- `backend/internal/service/field.go`

**做完是什么样**：
- 新增 `SyncFsmBBKeyRefs(ctx, tx, fsmID, oldKeys, newKeys map[string]bool) ([]int64, error)`
- 内部：diff keys → GetByNames 解析 name→ID → Add/Remove field_refs(ref_type='fsm')
- 新增 `CleanFsmBBKeyRefs(ctx, tx, fsmID) ([]int64, error)`（删除 FSM 时用）
- `go build ./...` 通过

---

## [x] T15: FsmConfig store 新增 Tx 版方法 (R17, R18, R19)

**文件**：
- `backend/internal/store/mysql/fsm_config.go`

**做完是什么样**：
- 新增 `CreateTx`、`UpdateTx`、`SoftDeleteTx` 三个事务版方法
- `go build ./...` 通过

---

## T16: FsmConfig handler 改为事务编排 + BB Key 追踪 (R17, R18, R19)

**文件**：
- `backend/internal/handler/fsm_config.go`
- `backend/internal/setup/handlers.go`（注入 fieldService + db）

**做完是什么样**：
- `FsmConfigHandler` 新增持有 `fieldService` 和 `db`（开事务用）
- `Create`：提取 BB Keys → 开事务 → service.CreateTx → fieldService.SyncFsmBBKeyRefs → 提交 → 清缓存
- `Update`：提取新旧 BB Keys → 开事务 → service.UpdateTx → SyncFsmBBKeyRefs → 提交 → 清缓存
- `Delete`：开事务 → service.SoftDeleteTx → fieldService.CleanFsmBBKeyRefs → 提交 → 清缓存
- setup 注入正确
- `go build ./...` 通过

---

## T17: 字段 references API 扩展 + expose_bb 保护 (R5, R20, R21, R22)

**文件**：
- `backend/internal/service/field.go`（GetReferences 扩展 + Update expose_bb 检查）
- `backend/internal/model/field.go`（ReferenceDetail 新增 Fsms）
- `backend/internal/handler/field.go`（GetReferences 补 FSM label）

**做完是什么样**：
- `ReferenceDetail` 新增 `Fsms []ReferenceItem`
- `GetReferences` 返回 ref_type='fsm' 的引用方，handler 跨模块补 FSM display_name
- `Update` 中 expose_bb 从 true→false 时，检查 field_refs ref_type='fsm'，有则返回 `ErrFieldBBKeyInUse`(40008)
- handler 注入 fsmConfigService（或 GetByIDsLite 方法）
- `go build ./...` 通过

---

## T18: 编译验证 + 数据库重建 (R6, R24)

**文件**：无新文件

**做完是什么样**：
- `go build ./...` 通过
- DROP TABLE fields/field_refs/templates/event_types/event_type_schema/fsm_configs + schema_refs 新建 → 重跑全部迁移成功
- 后端启动无报错
