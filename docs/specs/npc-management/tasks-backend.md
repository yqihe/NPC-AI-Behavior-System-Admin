# NPC 管理 — 后端任务拆解

> spec-create Phase 3 产出（后端）。
> 审批后从 main 拉分支：`git checkout -b feature/npc-management-backend`
>
> **一致性锚点**：`design.md` 第二节 API Contract。
> **前端任务**：`tasks-frontend.md`（独立分支，依赖本 spec 的 API Contract，不依赖后端代码）。
>
> 每个任务完成后执行 `/verify` 再继续。

---

## 依赖顺序

```
T1 → T2 → T3 → T4 → T5
                    ↑
              T6 → T7
T5 + T7 → T8
T5 → T9、T10、T12
T8 + T9 + T10 → T11 → T12
```

---

## ✅ T1：DDL + 错误码基础

**关联需求**：所有 R 的前置条件

**涉及文件**：
- `backend/migrations/013_create_npcs.sql`
- `backend/internal/errcode/codes.go`

**做完后是什么样**：
1. `013_create_npcs.sql` 包含 `npcs` 表和 `npc_bt_refs` 表的完整 DDL（含索引），表结构与 design.md §1.1 一致
2. `errcode/codes.go` 新增 `// --- NPC 管理 450xx ---` 段，包含 `ErrNPCNameExists`~`ErrNPCBtWithoutFsm`（45001-45015）共 15 个常量 + `messages` map 对应中文提示
3. 在 Docker MySQL 中执行 SQL 无报错

---

## ✅ T2：NPC 数据模型

**关联需求**：R1–R6（模型是所有后续层的基础）

**涉及文件**：
- `backend/internal/model/npc.go`

**做完后是什么样**：
文件包含以下全部结构体（对齐 design.md §1.2）：

| 结构体 | 用途 |
|--------|------|
| `NPC` | DB 整行，含 `Fields json.RawMessage`、`BtRefs json.RawMessage` |
| `NPCFieldEntry` | `fields` JSON 元素：`{field_id, name, required, value}` |
| `NPCListItem` | 列表项（覆盖索引列 + `template_label` 补全字段）|
| `NPCListData` | 列表缓存数据，带 `ToListData()` 方法 |
| `NPCDetail` | 详情响应（handler 组装，不进缓存）|
| `NPCDetailField` | 详情字段项（含当前 `label/type/enabled` + 快照 `required` + `value`）|
| `NPCLite` | 跨模块调用精简结构 `{id, name, label}` |
| `NPCExportItem` | 导出单条 `{name, config json.RawMessage}` |
| `NPCListQuery` | 列表查询参数（含 `template_name` 精确筛选）|
| `CreateNPCRequest` | 创建请求（含 `FieldValues []NPCFieldValue`、`BtRefs map[string]string`）|
| `NPCFieldValue` | `{field_id int64, value json.RawMessage}` |
| `UpdateNPCRequest` | 编辑请求（无 `template_id`）|
| `CreateNPCResponse` | `{id, name}` |

---

## T3：NPC MySQL Store

**关联需求**：R1–R7、R12–R16

**涉及文件**：
- `backend/internal/store/mysql/npc_store.go`

**做完后是什么样**：
实现以下方法（均遵循全库 store 约定）：

**读操作：**
- `GetByID(ctx, id) (*model.NPC, nil)` — 未找到返回 `(nil, nil)`
- `ExistsByName(ctx, name) (bool, error)` — 含软删除记录
- `List(ctx, q) ([]model.NPCListItem, int64, error)` — 覆盖索引，WHERE 条件用 slice 拼接

**写操作（事务变体）：**
- `CreateInTx(ctx, tx, req) (int64, error)` — INSERT INTO npcs
- `InsertBtRefsInTx(ctx, tx, npcID, btRefs map[string]string) error` — 批量 INSERT INTO npc_bt_refs
- `UpdateInTx(ctx, tx, req) error` — 乐观锁 WHERE id=? AND version=? AND deleted=0，0 rows → `ErrVersionConflict`
- `DeleteBtRefsInTx(ctx, tx, npcID) error` — DELETE FROM npc_bt_refs WHERE npc_id=?
- `SoftDeleteInTx(ctx, tx, id) error` — UPDATE npcs SET deleted=1
- `ToggleEnabled(ctx, req *model.ToggleEnabledRequest) error` — 乐观锁

**跨模块引用计数（供其他 handler 调用）：**
- `CountByTemplateID(ctx, templateID) (int64, error)` — `WHERE template_id=? AND deleted=0`
- `CountByBtTreeName(ctx, btName) (int64, error)` — `SELECT COUNT(*) FROM npc_bt_refs WHERE bt_tree_name=? AND npc_id IN (SELECT id FROM npcs WHERE deleted=0)`
- `CountByFsmRef(ctx, fsmName) (int64, error)` — `WHERE fsm_ref=? AND deleted=0`
- `ListByTemplateID(ctx, templateID, page, pageSize) ([]model.NPCLite, int64, error)` — 分页

**导出：**
- `ExportAll(ctx) ([]model.NPCExportItem, error)` — `WHERE enabled=1 AND deleted=0`，直接返回 `name + config json`（export handler 负责组装最终 config 结构）

---

## T4：NPC Redis Cache

**关联需求**：R1、R3、R4、R6（列表/详情缓存）

**涉及文件**：
- `backend/internal/store/redis/npc_cache.go`

**做完后是什么样**：
文件包含：
- Key 函数（在 `store/redis/config/keys.go` 补充 `NPCDetailKey(id)`、`NPCListKey(hash)`、`NPCLockKey(id)`，或在 npc_cache.go 内部私有函数生成，保持与已有模块一致的方式）
- `GetDetail / SetDetail / DelDetail` — Cache-Aside 详情缓存（`*model.NPC` 裸行，不含拼装后的 NPCDetail）
- `GetList / SetList / InvalidateList` — 列表缓存（`*model.NPCListData`）
- `TryLock / Unlock` — 分布式锁，Unlock 用 `LuaUnlock` 携带 lockID（red-lines §17）
- Redis 故障时静默跳过（不阻断主流程）

---

## T5：NPC Service

**关联需求**：R1–R9、R12–R17

**涉及文件**：
- `backend/internal/service/npc_service.go`

**做完后是什么样**：
`NpcService` 只持有自身的 `store *NpcStore` + `cache *NpcCache`，不持有其他 service。

**标准 CRUD 方法（遵循 backend-conventions.md 约定）：**
- `List(ctx, q) (*model.ListData, error)` — NormalizePagination → Redis → MySQL → 写缓存
- `GetByID(ctx, id) (*model.NPC, error)` — Cache-Aside + TryLock + double-check + 空标记
- `Create(ctx, req *model.CreateNPCRequest) (int64, error)` — 自行开事务：CreateInTx + InsertBtRefsInTx；Commit 前失效缓存（red-lines §16）
- `Update(ctx, req *model.UpdateNPCRequest) error` — 自行开事务：UpdateInTx + DeleteBtRefsInTx + InsertBtRefsInTx；Commit 前失效缓存
- `SoftDelete(ctx, id) (*model.DeleteResult, error)` — 自行开事务：SoftDeleteInTx + DeleteBtRefsInTx；Commit 前失效缓存
- `ToggleEnabled(ctx, req) error` — 乐观锁，ErrVersionConflict → 45014；失效缓存
- `CheckName(ctx, name) (*model.CheckNameResult, error)` — ExistsByName（含软删除）
- `getOrNotFound(ctx, id)` — 私有，存在性检查，nil → ErrNPCNotFound（45003）

**跨模块对外接口（供其他 handler 调用，不暴露 store 细节）：**
- `CountByTemplateID(ctx, templateID) (int64, error)`
- `CountByBtTreeName(ctx, btName) (int64, error)`
- `CountByFsmRef(ctx, fsmName) (int64, error)`
- `ListByTemplateID(ctx, templateID, page, pageSize) ([]model.NPCLite, int64, error)`
- `ExportAll(ctx) ([]model.NPCExportItem, error)`

---

## T6：扩展已有 Store 层

**关联需求**：R9、R10（FSM/BT 校验所需 store 方法）

**涉及文件**：
- `backend/internal/store/mysql/fsm_config.go`
- `backend/internal/store/mysql/bt_tree.go`

**做完后是什么样**：
- `FsmConfigStore.GetByName(ctx, name) (*model.FsmConfig, error)` — `WHERE name=? AND deleted=0`，未找到返回 `(nil, nil)`
- `BtTreeStore.GetEnabledByNames(ctx, names []string) (map[string]bool, error)` — `SELECT name FROM bt_trees WHERE name IN (?) AND enabled=1 AND deleted=0`；返回 `map[name → true]`，不在 map 中的 name 表示不存在或未启用；`names` 为空时直接返回空 map

---

## T7：扩展已有 Service 层

**关联需求**：R9、R10（FSM/BT service 对外接口）

**涉及文件**：
- `backend/internal/service/fsm_config.go`
- `backend/internal/service/bt_tree.go`

**做完后是什么样**：
- `FsmConfigService.GetEnabledByName(ctx, name) (*model.FsmConfig, error)` — 调 `store.GetByName`；nil → `ErrNPCFsmNotFound(45008)`；`!fsm.Enabled` → `ErrNPCFsmDisabled(45009)`（注：错误码属于 NPC 段，由 NPC handler 调用场景决定，service 方法直接返回对应 error）
- `BtTreeService.CheckEnabledByNames(ctx, names []string) (notOK []string, err error)` — 调 `store.GetEnabledByNames`；返回不存在或未启用的 name 列表（`notOK`）；`names` 为空时返回 `nil, nil`

---

## T8：NPC Handler

**关联需求**：R1–R9、R11

**涉及文件**：
- `backend/internal/handler/npc.go`

**做完后是什么样**：
`NpcHandler` 持有 `npcService`、`templateService`、`fieldService`、`fsmService`、`btService`、`db *sqlx.DB`（模板字段校验无需跨模块事务，`db` 本期可不注入）。

实现 7 个 handler 方法，均遵循 `handler/validate.go` 校验顺序（CheckID → CheckVersion → CheckName/CheckLabel → 其他格式 → slog.Debug → service 调用）：

**`List`**：透传 query → `npcService.List`

**`Create`**（跨模块编排，无事务）：
1. `CheckName(name)` + `CheckLabel(label)` + `template_id > 0` + `field_values` 非空
2. `templateService.GetByID` → enabled 校验 → 45004/45005
3. `templateService.ParseFieldEntries` → 拿 `[{field_id, required}]`
4. `fieldService.GetByIDsLite(fieldIDs)` → 拿字段元数据 map
5. 按模板字段顺序遍历：`value` 为 null + `required=true` → 45007；非 null → `service/shared.ValidateValue` → 45006
6. `fsm_ref` 非空：`fsmService.GetEnabledByName` → 45008/45009；解 `config_json` 拿 states 列表
7. `bt_refs` 非空且 `fsm_ref` 为空 → 45015；`btService.CheckEnabledByNames(treeNames)` → 45010/45011；校验每个 state key 在 states 列表中 → 45012
8. 构造 `CreateNPCRequest`（含 fields 快照）→ `npcService.Create`
9. 返回 `CreateNPCResponse{id, name}`

**`Get`**：`CheckID` → `npcService.GetByID` → `fieldService.GetByIDsLite` + `templateService.GetByIDsLite` 补全 labels → 组装 `NPCDetail` 返回

**`Update`**（跨模块编排）：
1. `CheckID` → `CheckVersion` → `CheckLabel`
2. `npcService.getOrNotFound` 拿旧 NPC（通过 `GetByID`）
3. 解析旧 `fields` 快照拿 `[{field_id, name, required}]` + 值 map
4. `fieldService.GetByIDsLite(snapshotFieldIDs)` 拿当前元数据
5. 按快照字段列表遍历新 `field_values`：missing 字段 `slog.Warn` 并保留旧值；其余校验同 Create
6. 重新校验 `fsm_ref` / `bt_refs`（复用私有方法）
7. 构造 `UpdateNPCRequest` → `npcService.Update`
8. 返回 `SuccessMsg("保存成功")`

**`Delete`**：`CheckID` → `npcService.GetByID` → `enabled=true` → 45013 → `npcService.SoftDelete` → `DeleteResult`

**`CheckName`**：`CheckName(name)` → `npcService.CheckName`

**`ToggleEnabled`**：`CheckID` → `CheckVersion` → `npcService.ToggleEnabled` → `SuccessMsg`

---

## T9：跨模块激活 — 模板

**关联需求**：R12、R13、R16

**涉及文件**：
- `backend/internal/handler/template.go`

**做完后是什么样**（仅修改以下 3 处，不改其他逻辑）：

1. **`TemplateHandler.Delete`**（激活 `41007`）：  
   在 `tpl.Enabled == false` 校验通过后、开事务前，加：
   ```go
   if count, err := h.npcService.CountByTemplateID(ctx, id); err != nil {
       return err
   } else if count > 0 {
       return errcode.New(errcode.ErrTemplateRefDelete)
   }
   ```

2. **`TemplateHandler.Update`**（激活 `41008`）：  
   在 `isFieldsChanged` 为 true 时（事务外预校验新增字段之后），加：
   ```go
   if count, err := h.npcService.CountByTemplateID(ctx, req.ID); err != nil {
       return err
   } else if count > 0 {
       return errcode.New(errcode.ErrTemplateRefEditFields)
   }
   ```

3. **`TemplateHandler.GetReferences`**（填充真实数据）：  
   将 `NPCs: make([]model.TemplateReferenceItem, 0)` 占位替换为：
   ```go
   npcs, _, _ := h.npcService.ListByTemplateID(ctx, req.ID, 1, 50)
   // 组装 TemplateReferenceItem 列表
   ```

`NewTemplateHandler` 函数签名新增 `npcService *service.NpcService` 参数。

---

## T10：跨模块激活 — BT 树 + FSM

**关联需求**：R14、R15

**涉及文件**：
- `backend/internal/handler/bt_tree.go`
- `backend/internal/handler/fsm_config.go`

**做完后是什么样**：

**`BtTreeHandler.Delete`**（激活 `44012`）：  
在 `btree.Enabled == false` 校验通过后、执行软删前，加：
```go
if count, _ := h.npcService.CountByBtTreeName(ctx, btree.Name); count > 0 {
    return errcode.New(errcode.ErrBtTreeRefDelete)
}
```
`NewBtTreeHandler` 签名新增 `npcService *service.NpcService`。

**`FsmConfigHandler.Delete`**（激活 `43012`）：  
在 `fsm.Enabled == false` 校验通过后、执行软删前，加：
```go
if count, _ := h.npcService.CountByFsmRef(ctx, fsm.Name); count > 0 {
    return errcode.New(errcode.ErrFsmConfigRefDelete)
}
```
`NewFsmConfigHandler` 签名新增 `npcService *service.NpcService`。

---

## T11：Setup 依赖注入

**关联需求**：所有 R 的运行前提

**涉及文件**：
- `backend/internal/setup/stores.go`
- `backend/internal/setup/caches.go`
- `backend/internal/setup/services.go`

**做完后是什么样**：

- `stores.go`：`Stores` 结构体新增 `Npc *storemysql.NpcStore`；`NewStores` 中初始化 `storemysql.NewNpcStore(db)`
- `caches.go`：`Caches` 结构体新增 `Npc *storeredis.NpcCache`；`NewCaches` 中初始化 `storeredis.NewNpcCache(rdb)`
- `services.go`：`Services` 结构体新增 `Npc *service.NpcService`；`NewServices` 中初始化 `service.NewNpcService(st.Npc, ca.Npc)`

---

## T12：路由 + Handler 注入 + 导出接口

**关联需求**：R1–R17（整体可跑）

**涉及文件**：
- `backend/internal/handler/export.go`
- `backend/internal/setup/handlers.go`
- `backend/internal/router/router.go`

**做完后是什么样**：

**`handler/export.go`**：新增 `NPCTemplates` handler 方法，调 `npcService.ExportAll`，组装 `{name, config: {template_ref, fields: {k:v}, behavior: {fsm_ref, bt_refs}}}` 格式；`behavior` 中 `fsm_ref` 为空串时省略该键，`bt_refs` 为空 map 时省略该键（两者均空时 `behavior = {}`）。`NewExportHandler` 签名新增 `npcService`。

**`setup/handlers.go`**：
- `Handlers` 结构体新增 `Npc *handler.NpcHandler`
- `NewHandlers` 中初始化 `handler.NewNpcHandler(svc.Npc, svc.Template, svc.Field, svc.FsmConfig, svc.BtTree)`
- 修改 `NewTemplateHandler` 调用：补传 `svc.Npc`
- 修改 `NewBtTreeHandler` 调用：补传 `svc.Npc`
- 修改 `NewFsmConfigHandler` 调用：补传 `svc.Npc`
- 修改 `NewExportHandler` 调用：补传 `svc.Npc`

**`router/router.go`**：
- 新增 `/api/v1/npcs` 路由组，注册 7 个接口（list/create/detail/update/delete/check-name/toggle-enabled）
- 在 configs 路由组补充 `configs.GET("/npc_templates", h.Export.NPCTemplates)`
- 服务编译通过，`go build ./...` 无报错

---

## 验收检查清单

后端所有任务完成后执行 `/verify`，确认：

- [ ] `go build ./...` 零报错、零 warning
- [ ] `npx vue-tsc --noEmit`（前端类型不应因后端改动破坏，但前端 API 类型已与 Contract 对齐时才执行）
- [ ] curl 脚本验证（参考 `docs/development/admin/test-lifecycle-guard-npc.md`）：
  - [ ] NPC CRUD 完整链路（create→detail→update→toggle→delete）
  - [ ] 45007 必填字段拦截
  - [ ] 45006 字段值约束拦截
  - [ ] 45005 模板未启用拦截
  - [ ] 45012 bt_refs 状态名错误拦截
  - [ ] 45015 bt_refs 非空但 fsm_ref 为空拦截
  - [ ] 41007 模板被 NPC 引用后无法删除
  - [ ] 41008 模板被 NPC 引用后字段不可改
  - [ ] 44012 BT 树被 NPC 引用后无法删除
  - [ ] 43012 FSM 被 NPC 引用后无法删除
  - [ ] GET /api/configs/npc_templates 返回正确格式
