# 事件类型管理 — 任务拆解

> 对应需求：[requirements.md](requirements.md)
> 对应设计：[design.md](design.md)
> 按依赖顺序排列，每个任务 1-3 个文件。

---

## 第一阶段：后端基础设施

### T1：DDL 迁移脚本 (R7, R9, R17)

**涉及文件**：
- `backend/migrations/004_create_event_types.sql`（新建）
- `backend/migrations/005_create_event_type_schema.sql`（新建）

**做完标准**：两张表在 MySQL 中创建成功，`uk_name` / `uk_field_name` 唯一约束生效，`idx_list` / `idx_perception` 索引可用。手动执行 `docker exec -i` 验证。

---

### T2：Model 定义 (R1, R4, R16)

**涉及文件**：
- `backend/internal/model/event_type.go`（新建）
- `backend/internal/model/event_type_schema.go`（新建）

**做完标准**：定义完整的请求/响应/DB 结构体。包含：
- `EventType`（db struct）、`EventTypeListItem`（列表展示）、`EventTypeDetail`（详情响应含 config + extension_schema）
- `CreateEventTypeRequest`、`UpdateEventTypeRequest`（含 extensions map）
- `EventTypeSchema`（db struct）、`EventTypeSchemaLite`（精简版给详情接口）
- `CreateEventTypeSchemaRequest`、`UpdateEventTypeSchemaRequest`
- perception_mode 枚举常量（`PerceptionModeVisual` / `Auditory` / `Global`）
- 所有 slice 字段默认 `make([]T, 0)` 防 null

---

### T3：错误码定义 (R3)

**涉及文件**：
- `backend/internal/errcode/codes.go`（改动：追加 42001-42031）

**做完标准**：42001-42015（事件类型）+ 42020-42031（扩展字段 Schema）全部定义为常量，`messages` map 有对应中文消息。编译通过。

---

### T4：Redis Key 定义 (R22, R23, R24)

**涉及文件**：
- `backend/internal/store/redis/keys.go`（改动：追加 event_types key 函数）

**做完标准**：新增 `EventTypeDetailKey(id)`、`EventTypeListKey(version, hash)`、`EventTypeLockKey(id)`、`EventTypeListVersionKey` 四个函数/常量，命名风格与 fields/templates 一致。

---

### T5：配置项新增 (R22, R23)

**涉及文件**：
- `backend/internal/config/config.go`（改动：追加 EventTypeConfig / EventTypeSchemaConfig）
- `config.yaml`（改动：追加 event_type / event_type_schema 段）

**做完标准**：`Config` struct 有 `EventType EventTypeConfig` 和 `EventTypeSchema EventTypeSchemaConfig` 字段，YAML 能正确加载。

---

## 第二阶段：后端 Store 层

### T6：EventTypeStore — CRUD + 列表 (R1, R4, R5, R6, R7, R9, R10, R11)

**涉及文件**：
- `backend/internal/store/mysql/event_type.go`（新建）

**做完标准**：实现以下方法：
- `Create(ctx, et) (int64, error)` — INSERT 返回 lastInsertId
- `GetByID(ctx, id) (*model.EventType, error)` — 单条查询（含 config_json）
- `List(ctx, query) ([]model.EventType, int64, error)` — 分页列表（覆盖索引 + display_name LIKE 转义 + perception_mode 精确筛选）
- `Update(ctx, id, fields, version) (int64, error)` — 乐观锁更新
- `SoftDelete(ctx, id) error` — 软删除
- `ToggleEnabled(ctx, id, enabled, version) (int64, error)` — 乐观锁切换
- `ExistsByName(ctx, name) (bool, error)` — 唯一性校验（不过滤 deleted）
- `ExportAll(ctx) ([]model.EventTypeExportItem, error)` — 导出用：`SELECT name, config_json WHERE deleted=0 AND enabled=1`

---

### T7：EventTypeSchemaStore — CRUD + 列表 (R16, R17, R18, R20)

**涉及文件**：
- `backend/internal/store/mysql/event_type_schema.go`（新建）

**做完标准**：实现以下方法：
- `Create(ctx, s) (int64, error)` — INSERT
- `GetByID(ctx, id) (*model.EventTypeSchema, error)` — 单条
- `List(ctx, query) ([]model.EventTypeSchema, error)` — 按 enabled 筛选
- `ListEnabled(ctx) ([]model.EventTypeSchema, error)` — 全量拉启用的（给内存缓存 Load 用）
- `Update(ctx, id, fields, version) (int64, error)` — 乐观锁
- `SoftDelete(ctx, id) error`
- `ToggleEnabled(ctx, id, enabled, version) (int64, error)`
- `ExistsByFieldName(ctx, fieldName) (bool, error)` — 唯一性

---

### T8：EventTypeCache — Redis 缓存 (R22, R23, R24, R25, R26)

**涉及文件**：
- `backend/internal/store/redis/event_type_cache.go`（新建）

**做完标准**：实现以下方法（和 FieldCache / TemplateCache 同构）：
- `GetDetail(ctx, id) (*model.EventType, error)` — 含空标记识别
- `SetDetail(ctx, id, et, ttl)` — et=nil 时写空标记
- `DelDetail(ctx, id)`
- `TryLock(ctx, id, ttl) (bool, error)` — 分布式锁
- `Unlock(ctx, id)`
- `GetList(ctx, version, hash) (*model.EventTypeListData, error)` — 类型安全
- `SetList(ctx, version, hash, data, ttl)`
- `InvalidateList(ctx)` — INCR version key
- Redis 不可用时方法返回缓存 miss（降级到 MySQL），slog.Warn

---

### T9：EventTypeSchemaCache — 内存缓存 (R27)

**涉及文件**：
- `backend/internal/cache/event_type_schema_cache.go`（新建）

**做完标准**：
- `Load(ctx) error` — 启动时从 MySQL 全量加载 `ListEnabled`，按 `sort_order` 排序存入 `[]EventTypeSchemaLite`
- `Reload(ctx) error` — 写后同步调用
- `ListEnabled() []EventTypeSchemaLite` — 返回预排序内存切片（返回副本防外部修改）
- `GetByFieldName(fieldName) (*EventTypeSchemaLite, bool)` — 按 field_name 查找（供 Service 校验用）
- sync.RWMutex 保护并发读写

---

## 第三阶段：后端 Service 层

### T10：constraint 包抽离 (R14, R15)

**涉及文件**：
- `backend/internal/service/constraint/validate.go`（新建）
- `backend/internal/service/field.go`（改动：抽出值级校验辅助函数）

**做完标准**：
- `constraint.ValidateValue(fieldType, constraints, value) *errcode.Error` — 覆盖 int/float/string/bool/select 五种类型
- `constraint.ValidateConstraintsSelf(fieldType, constraints) *errcode.Error` — min<=max / options 非空等自洽检查
- `field.go` 的 `checkConstraintTightened` 改为调 constraint 包的辅助函数（`getFloat`/`getString`/`getOptions` 等），但收紧逻辑本身保留在 field.go
- **字段管理现有 199/199 集成测试全部通过**（这是硬性验收条件）

---

### T11：EventTypeService (R1, R4, R5, R6, R7, R8, R10, R11, R12, R13)

**涉及文件**：
- `backend/internal/service/event_type.go`（新建）

**做完标准**：实现以下方法：
- `List(ctx, query) (*model.ListData, error)` — 分页 + Cache-Aside + unmarshal config_json 挑展示字段
- `Create(ctx, req) (int64, error)` — name 唯一性 → 扩展字段约束校验（调 constraint.ValidateValue）→ 拼 config_json → Store.Create → 清缓存
- `GetByID(ctx, id) (*model.EventType, error)` — Cache-Aside + TryLock + double-check + 空标记
- `Update(ctx, req) error` — getOrNotFound → enabled=false 校验 → 扩展字段约束 → 拼 config_json → 乐观锁更新 → 清缓存
- `Delete(ctx, id) error` — getOrNotFound → enabled=false → SoftDelete → 清缓存
- `CheckName(ctx, name) (*model.NameCheckResult, error)` — ExistsByName
- `ToggleEnabled(ctx, id, version) error` — 乐观锁 → 清缓存
- `ExportAll(ctx) ([]model.EventTypeExportItem, error)` — Store.ExportAll 透传
- `buildConfigJSON(req, schemas) (json.RawMessage, error)` — 内部方法：合并系统字段 + 校验后的扩展字段
- `validateExtensions(extensions, schemas) *errcode.Error` — 内部方法：遍历 extensions key 校验

---

### T12：EventTypeSchemaService (R16, R17, R18, R19, R20, R21)

**涉及文件**：
- `backend/internal/service/event_type_schema.go`（新建）

**做完标准**：实现以下方法：
- `List(ctx, query) ([]model.EventTypeSchema, error)` — 直查 MySQL（量小不走 Redis）
- `Create(ctx, req) (int64, error)` — field_name 唯一性 → field_type 枚举 → constraints 自洽（constraint.ValidateConstraintsSelf）→ default_value 符合 constraints（constraint.ValidateValue）→ Store.Create → SchemaCache.Reload
- `Update(ctx, req) error` — getOrNotFound → field_name/field_type 不可改 → constraints 自洽 → default_value 校验 → 乐观锁 → SchemaCache.Reload
- `Delete(ctx, id) error` — getOrNotFound → enabled=false → SoftDelete → SchemaCache.Reload
- `ToggleEnabled(ctx, id, version) error` — 乐观锁 → SchemaCache.Reload
- `ListEnabled() []model.EventTypeSchemaLite` — 直接代理 SchemaCache.ListEnabled（给 handler 调）

---

## 第四阶段：后端 Handler + Router

### T13：EventTypeHandler — 7 个接口 (R1, R2, R3, R8, R11, R12, R13)

**涉及文件**：
- `backend/internal/handler/event_type.go`（新建）

**做完标准**：
- `List` — WrapCtx，透传 query
- `Create` — WrapCtx，Handler 校验：name 正则 + display_name 长度（utf8.RuneCountInString）+ perception_mode 枚举 + severity ∈ [0,100] + ttl>0 + range>=0 + global 时 range 置 0 + extensions JSON 对象形状
- `Get` — WrapCtx，校验 id>0 → Service.GetByID → SchemaService.ListEnabled → 拼 EventTypeDetail 响应
- `Update` — WrapCtx，Handler 校验（同 Create 但无 name）+ version>0
- `Delete` — WrapCtx，校验 id>0
- `CheckName` — WrapCtx，校验 name 非空
- `ToggleEnabled` — WrapCtx，校验 id>0 + version>0

---

### T14：EventTypeSchemaHandler — 5 个接口 (R16, R17, R18, R19)

**涉及文件**：
- `backend/internal/handler/event_type_schema.go`（新建）

**做完标准**：
- `List` — WrapCtx
- `Create` — WrapCtx，Handler 校验：field_name 正则 + field_label 长度 + field_type 枚举（int/float/string/bool/select，拒绝 reference）+ constraints JSON 对象形状 + default_value 非空
- `Update` — WrapCtx，校验 id>0 + version>0 + 不含 field_name/field_type
- `Delete` — WrapCtx，校验 id>0
- `ToggleEnabled` — WrapCtx，校验 id>0 + version>0

---

### T15：ExportHandler — 导出 API (R28, R29, R30, R31)

**涉及文件**：
- `backend/internal/handler/export.go`（新建）

**做完标准**：
- `EventTypes(c *gin.Context)` — 调 Service.ExportAll → 遍历结果 → 每条 `json.RawMessage(config_json)` 原样展开到 `{name, config}` → 返回 `{items: [...]}`
- 空数据返回 `{items: []}`
- 不走 WrapCtx（导出 API 格式与 CRUD 不同），直接 `c.JSON(200, ...)`

---

### T16：路由注册 + main.go 装配 (R1, R16, R27, R28)

**涉及文件**：
- `backend/internal/router/router.go`（改动：新增路由组 + Setup 签名追加参数）
- `backend/cmd/admin/main.go`（改动：装配注入链 + SchemaCache.Load）

**做完标准**：
- `router.Setup` 签名追加 `eth *handler.EventTypeHandler, etsh *handler.EventTypeSchemaHandler, exh *handler.ExportHandler`
- 注册 `/api/v1/event-types/*` 7 个路由 + `/api/v1/event-type-schema/*` 5 个路由 + `GET /api/configs/event_types`
- `main.go` 按依赖顺序装配：Store → Cache → SchemaCache.Load(ctx) → Service → Handler
- 服务启动成功，所有接口可达（curl 返回 200）

---

## 第五阶段：后端测试

### T17：集成测试脚本 — 正向 + 错误路径 (R1-R31)

**涉及文件**：
- `tests/event_type_test.sh`（新建）

**做完标准**：
- 覆盖事件类型 CRUD 全流程 + 扩展字段 Schema CRUD 全流程 + 导出 API
- 覆盖所有错误码路径（42001-42027 至少各一条 case）
- 含攻击性测试：name 特殊字符 / SQL 注入 / severity=0 零值 / global+range=0 / 扩展字段塞不存在 key / 塞已停用 key
- **全部通过**

### T18：constraint 包单元测试 (R14, R15)

**涉及文件**：
- `backend/internal/service/constraint/validate_test.go`（新建）

**做完标准**：
- `ValidateValue`：5 种类型 × (合法值 + 违反 min + 违反 max + 类型错误) = ~20 case
- `ValidateConstraintsSelf`：min>max / options 为空 / pattern 非法正则 = ~10 case
- `go test ./internal/service/constraint/ -v` 全部通过

---

## 第六阶段：前端 API + Store

### T19：前端 API 层 (R32)

**涉及文件**：
- `frontend/src/api/event-types.ts`（新建）
- `frontend/src/api/event-type-schema.ts`（新建）

**做完标准**：
- 定义所有请求/响应 TypeScript 接口
- 封装 12 个 API 函数 + 1 个导出 API 函数（如果前端需要）
- `EVENT_TYPE_ERR` / `EVENT_TYPE_SCHEMA_ERR` 错误码映射中文消息
- `npx vue-tsc --noEmit` 通过

---

### T20：前端 Pinia Store (R32, R35)

**涉及文件**：
- `frontend/src/stores/eventType.ts`（新建）
- `frontend/src/stores/eventTypeSchema.ts`（新建）

**做完标准**：
- `eventType` store：列表查询 / 详情 / 当前编辑对象
- `eventTypeSchema` store：schema 列表 + `listEnabled` 过滤 + `reload` action（App 启动后 fetch 一次）
- `npx vue-tsc --noEmit` 通过

---

## 第七阶段：前端页面 — 事件类型

### T21：事件类型列表页 (R32, R33, R38)

**涉及文件**：
- `frontend/src/views/EventTypeList.vue`（新建）
- `frontend/src/router/index.ts`（改动：新增 `/event-types` 路由）

**做完标准**：
- 后端分页 + display_name 模糊搜索 + perception_mode facet 筛选（el-select，选项从字典接口拉或前端常量）+ enabled 三态筛选
- 停用行整行 opacity 0.5，操作列保持高亮
- perception_mode 用中文 el-tag（视觉=绿 / 听觉=蓝 / 全局=紫）
- severity 列展示数字 + 色带
- 操作列：编辑 / 删除
- 新建按钮跳 `/event-types/new`
- EnabledGuardDialog 泛型复用（追加 `entityType: 'event_type'`）
- `npx vue-tsc --noEmit` 通过

---

### T22：事件类型系统字段组件 (R34, R36, R37, R39)

**涉及文件**：
- `frontend/src/components/EventTypeSystemFields.vue`（新建）
- `frontend/src/components/SeverityBar.vue`（新建）

**做完标准**：
- name 输入框（新建时可编辑 + blur 调 check-name，编辑时只读）
- display_name 输入框
- perception_mode radio group（视觉 / 听觉 / 全局，中文标签）
- range 数字输入框，global 模式时禁用并置 0（watch perception_mode）
- SeverityBar：el-slider 0-100 + 左侧数字输入 + 色带（0-30 绿 / 30-70 黄 / 70-100 红）
- default_ttl 数字输入框（> 0）
- 顶部常驻 el-alert warning "修改感知参数后需通知运维重启游戏服务端才能生效"
- `npx vue-tsc --noEmit` 通过

---

### T23：SchemaForm 通用组件 (R35)

**涉及文件**：
- `frontend/src/components/SchemaForm.vue`（新建）

**做完标准**：
- 接受 `schemas: SchemaFieldDef[]` + `values: Record<string, any>` + `defaults: Record<string, any>`
- 按 sort_order 渲染每个 schema 定义的输入控件（int→InputNumber / float→InputNumber step=0.01 / string→Input / bool→Switch / select→Select）
- 内部维护 `dirtyMap`：config 里已有 key → dirty=true；无 key → dirty=false，显示 default 作暗示值（灰色占位）
- 用户交互后 dirty=true
- emit `update` 事件只含 dirty=true 的字段
- `npx vue-tsc --noEmit` 通过

---

### T24：事件类型表单页 (R34, R38, R39, R41)

**涉及文件**：
- `frontend/src/views/EventTypeForm.vue`（新建）
- `frontend/src/router/index.ts`（改动：新增 `/event-types/new` + `/event-types/:id/edit` 路由）

**做完标准**：
- 新建模式：调 SchemaStore.listEnabled 拿 schema → 渲染 SystemFields + SchemaForm
- 编辑模式：先调 detail 接口拿完整数据 + extension_schema → 拆分系统字段和扩展字段 → 渲染
- 启用中进入编辑页：全部只读 + 顶部 banner "请先停用才能编辑"
- 保存按钮提交：merge 系统字段 + dirty 扩展字段 → 调 create/update API
- 离开路由前未保存弹确认（onBeforeRouteLeave）
- `npx vue-tsc --noEmit` 通过

---

## 第八阶段：前端页面 — Schema 管理

### T25：Schema 管理页 + 事件类型扩展字段 Tab (R40)

**涉及文件**：
- `frontend/src/views/SchemaManagement.vue`（新建）
- `frontend/src/components/EventTypeSchemaTab.vue`（新建）
- `frontend/src/router/index.ts`（改动：新增 `/schema-management` 路由）

**做完标准**：
- SchemaManagement 作为 tab 容器，当前只有"事件类型扩展字段"一个 tab
- EventTypeSchemaTab：schema 列表（field_name / field_label / field_type / enabled / sort_order / 操作）
- 新建/编辑按钮拉起弹窗
- 启用/停用 toggle + 确认弹窗
- 删除 + 确认弹窗（"必须先停用"引导）
- `npx vue-tsc --noEmit` 通过

---

### T26：扩展字段 Schema 新建/编辑弹窗 (R17, R18, R19, R40)

**涉及文件**：
- `frontend/src/components/EventTypeSchemaForm.vue`（新建）
- `frontend/src/components/ConstraintPanel.vue`（新建）

**做完标准**：
- 弹窗内表单：field_name（新建可编辑，编辑只读）/ field_label / field_type（新建可选，编辑只读）/ constraints（ConstraintPanel 按 field_type 渲染）/ default_value / sort_order
- ConstraintPanel：按 field_type 动态 import 对应的 FieldConstraint{Integer,Float,String,Boolean,Select}.vue（复用字段管理已有组件，reference 不渲染）
- default_value 输入控件和 field_type 联动
- field_name blur 调 check API（如有，或前端本地校验正则）
- `npx vue-tsc --noEmit` 通过

---

## 第九阶段：侧边栏 + 收尾

### T27：侧边栏菜单注册 (R32, R40)

**涉及文件**：
- `frontend/src/layout/Sidebar.vue` 或等价侧边栏文件（改动）

**做完标准**：
- "行为管理"el-sub-menu 下新增"事件类型"菜单项，跳转 `/event-types`
- "系统设置"el-sub-menu 下新增"Schema 管理"菜单项，跳转 `/schema-management`
- 折叠/展开正常
- 高亮态和 hover 态正确

---

### T28：前端最终验证 (R41, R42, R43)

**涉及文件**：
- 无新文件，验证已有代码

**做完标准**：
- `npx vue-tsc --noEmit` 通过
- 浏览器手动测试全流程：新建/编辑/启停/删除事件类型 + Schema 管理 CRUD + SchemaForm dirty 追踪 + perception_mode 联动 + severity slider + 启用中编辑拦截 + 导出 API curl 验证
- 后端 slog 日志输出正确（Debug/Info/Error 层级）
- `tests/event_type_test.sh` 全部通过

---

## 任务依赖图

```
T1 (DDL) ──┐
T2 (Model) ─┤
T3 (ErrCode)┤
T4 (Keys)  ─┤
T5 (Config) ┘
     │
     ▼
T6 (EventTypeStore) ──┐
T7 (SchemaStore) ──────┤
T8 (RedisCache) ───────┤
T9 (MemCache) ─────────┘
     │
     ▼
T10 (constraint 包) ──► T11 (EventTypeService) ──┐
                        T12 (SchemaService) ──────┘
                              │
                              ▼
                    T13 (EventTypeHandler) ──┐
                    T14 (SchemaHandler) ─────┤
                    T15 (ExportHandler) ─────┤
                    T16 (Router + main) ─────┘
                              │
                              ▼
                    T17 (集成测试) ──┐
                    T18 (单元测试) ──┘
                              │
                              ▼
                    T19 (前端 API) ──┐
                    T20 (前端 Store)─┘
                              │
                              ▼
              T21 (列表页) ──────────┐
              T22 (系统字段组件) ────┤
              T23 (SchemaForm) ─────┤
              T24 (表单页) ─────────┘
                              │
                              ▼
              T25 (Schema 管理页) ──┐
              T26 (Schema 弹窗) ───┘
                              │
                              ▼
              T27 (侧边栏) ──► T28 (最终验证)
```

---

## 总结

| 阶段 | 任务数 | 预估新增/改动文件 |
|---|---|---|
| 后端基础设施 | T1-T5 | 4 新建 + 3 改动 |
| 后端 Store | T6-T9 | 4 新建 |
| 后端 Service | T10-T12 | 3 新建 + 1 改动 |
| 后端 Handler + Router | T13-T16 | 3 新建 + 2 改动 |
| 后端测试 | T17-T18 | 2 新建 |
| 前端 API + Store | T19-T20 | 4 新建 |
| 前端页面（事件类型）| T21-T24 | 5 新建 + 1 改动 |
| 前端页面（Schema）| T25-T26 | 4 新建 + 1 改动 |
| 收尾 | T27-T28 | 1 改动 |
| **合计** | **28 个任务** | **~29 新建 + ~8 改动** |
