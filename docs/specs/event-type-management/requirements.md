# 事件类型管理 — 需求分析

> 对应文档：
> - 功能：[features.md](../../v3-PLAN/行为管理/事件类型/features.md)
> - 后端设计：[backend.md](../../v3-PLAN/行为管理/事件类型/backend.md)
> - 前端设计：[frontend.md](../../v3-PLAN/行为管理/事件类型/frontend.md)
> - 导出契约：[api-contract.md](../../v3-PLAN/api-contract.md) 的 "### 2. 事件类型" 段

---

## 动机

事件类型是 V3 推荐创建顺序中第 4 步（字段 → 模板 → **事件类型** → 状态机 → 行为树 → NPC → 区域），是**行为管理分类的第一个模块**，也是 ADMIN 侧第一个"导出给游戏服务端消费的配置"类型。

不做的代价：

1. **FSM/BT 模块阻塞**：状态机转移条件用 `last_event_type == "gunshot"` 引用事件类型 name，行为树叶节点用 `check_bb_string key=last_event_type value=gunshot`。没有事件类型管理，FSM/BT 的条件编辑器无法提供事件类型下拉选项。
2. **导出 API 缺口**：游戏服务端启动时拉取 `GET /api/configs/event_types`，当前该端点不存在，服务端如果开启会直接启动失败。
3. **扩展字段机制无处落地**：事件类型是第一个引入"系统字段 + 运营自定义扩展字段"架构的模块，该机制需要在事件类型上验证后才能推广到 FSM/BT/Region。
4. **约束校验无法复用**：字段管理的约束校验逻辑（`checkConstraintTightened` 内的值校验部分）需要在本模块中被抽出为 `service/constraint/validate.go` 独立包，后续所有新增配置类型都依赖此包。

---

## 优先级

**当前阶段最高优先级**。理由：

- V3 推荐创建顺序的下一个节点
- FSM/BT（顺序 5/6）直接依赖事件类型管理的 name 列表和导出 API
- 游戏服务端 Phase 1 的"需求 7 EventType 扩展字段"改造正在并行进行，联调窗口即将打开
- 设计文档（features.md + backend.md + frontend.md + api-contract.md）已完整定稿并提交，与服务端 CC 的扩展字段契约已确认，没有未决议题

---

## 预期效果

完成后系统行为：

1. **事件类型 CRUD 闭环**：管理员通过 ADMIN 前端可以新建、编辑、停用/启用、删除事件类型，所有操作通过 7 个 REST 接口完成。
2. **扩展字段 Schema 管理**：运营通过 Schema 管理页的"事件类型扩展字段"tab 可以新增/编辑/启用/停用/删除扩展字段定义，无需后端发版即可在事件类型表单上多出一个输入控件。
3. **Schema 驱动的动态表单**：事件类型新建/编辑页由"硬编码系统字段区 + SchemaForm 渲染的扩展字段区"组成。SchemaForm 从 `event_type_schema` 表的启用字段定义动态渲染。
4. **导出 API 到位**：`GET /api/configs/event_types` 从 MySQL `event_types.config_json` 列直接输出 `{items: [{name, config}]}` 格式，游戏服务端可以启动时一次性拉取。
5. **三态生命周期严格执行**：启用中事件类型拒绝编辑/删除；停用后可改可删；引用机制（ref_count/event_type_refs）本期不接入但 Service 层预留 stub。
6. **约束校验复用包落地**：`service/constraint/validate.go` 抽出后，字段管理和事件类型扩展字段共用同一套值级校验逻辑。
7. **缓存策略与字段/模板管理一致**：列表版本号失效、详情分布式锁防击穿、空标记防穿透、TTL+jitter 防雪崩。

具体场景：

- **场景 A**：管理员创建"枪声"事件类型（name=gunshot, perception_mode=auditory, default_severity=90, range=300）→ 接口返回新 ID，默认 enabled=0，config_json 包含完整系统字段。
- **场景 B**：运营在 Schema 管理页添加扩展字段 `priority (int, min=1, max=10, default=5)` → schema 表新增一行。下次新建事件类型时表单多出 "优先级" 输入框，初始值 5。
- **场景 C**：管理员编辑"枪声"，填写扩展字段 priority=8 → config_json 里出现 `"priority": 8`。另一个事件类型"地震"从未填 priority → config_json 里没有 priority key。导出 API 两条都返回，游戏服务端对"枪声"用 8，对"地震"按 defaults.go 的 50 兜底。
- **场景 D**：管理员尝试编辑启用中的事件类型 → 返回 42015；停用后编辑 → 成功。
- **场景 E**：管理员尝试删除未停用的事件类型 → 返回 42012；停用后删除 → 软删成功，name 仍占唯一性。
- **场景 F**：游戏服务端调用 `GET /api/configs/event_types` → 返回所有 `enabled=1 AND deleted=0` 的事件类型，config 字段是 config_json 原样展开。

---

## 依赖分析

### 依赖的已完成工作

| 依赖项 | 位置 | 用途 |
|---|---|---|
| Handler `WrapCtx` 泛型包装 | `backend/internal/handler/wrap.go` | 统一响应格式 |
| 错误码体系框架 | `backend/internal/errcode/` | 42001-42039 段位 |
| 字典缓存 `DictCache` | `backend/internal/cache/` | perception_mode label 翻译（如需） |
| 配置（PaginationConfig/ValidationConfig）| `backend/internal/config/` | 分页默认值/校验长度 |
| Router 注册模式 | `backend/internal/router/router.go` | 沿用已有路由注册模式 |
| 字段管理的 FieldConstraint*.vue 组件 | `frontend/src/components/` | Schema 管理页复用约束编辑面板 |
| EnabledGuardDialog.vue | `frontend/src/components/` | 复用"必须先停用"引导弹窗 |
| 字段管理的约束校验逻辑 | `backend/internal/service/field.go` | 抽出到 `service/constraint/validate.go` 复用 |

### 谁依赖这个需求

| 依赖方 | 需要的内容 | 紧迫度 |
|---|---|---|
| **FSM 模块**（下一步） | 事件类型 name 列表（条件编辑器下拉）+ `ValidateEventTypesForFSM` | 阻塞 |
| **BT 模块**（下一步） | 同上 | 阻塞 |
| **游戏服务端 Phase 1**（并行） | `GET /api/configs/event_types` 导出 API | 联调阻塞 |
| **游戏服务端 Phase 3** | Source 接口适配 v3 端点 | 后续 |
| **后续配置类型（FSM/BT/Region）** | `service/constraint/validate.go` 约束校验复用包 | 通用依赖 |
| **后续配置类型** | SchemaForm.vue 通用组件 | 通用依赖 |

### 不依赖（确认无关）

| 项 | 说明 |
|---|---|
| MongoDB | 本模块**不使用 MongoDB**，MySQL 单存储 |
| RabbitMQ | 本模块**不使用 MQ**，无跨库同步需求 |
| `field_refs` 表 | 事件类型不通过字段引用关系表——它有自己独立的 `event_type_refs`（本期不建） |
| `fields` 表 | 事件类型的字段不来自字段管理，而是系统字段硬编码 + event_type_schema 扩展 |

---

## 改动范围

预估 **新增 ~15 个文件 + 改动 ~5 个文件**，横跨后端和前端。

### 后端新增文件

| 文件 | 作用 |
|---|---|
| `internal/model/event_type.go` | EventType / EventTypeListItem / EventTypeDetail / 请求响应结构 |
| `internal/model/event_type_schema.go` | EventTypeSchema / EventTypeSchemaLite |
| `internal/store/mysql/event_type.go` | EventTypeStore：CRUD + 列表 + 乐观锁 + 导出 |
| `internal/store/mysql/event_type_schema.go` | EventTypeSchemaStore：CRUD + 列表 |
| `internal/store/redis/event_type_cache.go` | EventTypeCache：列表/详情/锁/版本号 |
| `internal/cache/event_type_schema_cache.go` | EventTypeSchemaCache：启动时全量内存缓存 |
| `internal/service/event_type.go` | EventTypeService：业务逻辑 + 扩展字段约束校验 |
| `internal/service/event_type_schema.go` | EventTypeSchemaService：扩展字段 Schema CRUD |
| `internal/service/constraint/validate.go` | **从字段管理抽出的**约束校验复用包 |
| `internal/handler/event_type.go` | EventTypeHandler：7 个接口 |
| `internal/handler/event_type_schema.go` | EventTypeSchemaHandler：5 个接口 |
| `internal/handler/export.go` | ExportHandler：导出 API（或在已有 handler 追加方法） |
| `migrations/004_create_event_types.sql` | event_types 表 DDL |
| `migrations/005_create_event_type_schema.sql` | event_type_schema 表 DDL |

### 后端改动文件

| 文件 | 改动内容 |
|---|---|
| `internal/errcode/codes.go` | 新增 42001-42039 事件类型 + 扩展字段 Schema 错误码 |
| `internal/router/router.go` | 注册 `/api/v1/event-types/*` 7 个路由 + `/api/v1/event-type-schema/*` 5 个路由 + `GET /api/configs/event_types` |
| `internal/store/redis/keys.go` | 新增 event_types 缓存 key 前缀常量 |
| `cmd/admin/main.go` | 装配 EventTypeStore/Cache/Service/Handler + EventTypeSchemaStore/Cache/Service/Handler + ExportHandler 注入链 + EventTypeSchemaCache.Load() |
| `internal/service/field.go` | 抽出 `checkConstraintTightened` 内的值级校验逻辑到 `constraint/validate.go`，原方法调改为调新包 |

### 前端新增文件

| 文件 | 作用 |
|---|---|
| `src/views/EventTypeList.vue` | 事件类型列表页 |
| `src/views/EventTypeForm.vue` | 事件类型新建/编辑页 |
| `src/components/EventTypeSystemFields.vue` | 系统字段硬编码区（name/display_name/perception_mode/range/severity/ttl）|
| `src/components/EventTypeExtensionFields.vue` | 扩展字段 SchemaForm 包装 |
| `src/components/SchemaForm.vue` | **通用组件**：接受 schema 数组 + 值对象 + dirty 追踪，动态渲染表单 |
| `src/components/SeverityBar.vue` | default_severity 0-100 slider 配色带 |
| `src/views/SchemaManagement.vue` | Schema 管理页主容器（多 tab） |
| `src/components/EventTypeSchemaTab.vue` | 事件类型扩展字段 tab |
| `src/components/EventTypeSchemaForm.vue` | 扩展字段 Schema 新建/编辑弹窗 |
| `src/api/event-types.ts` | 事件类型 REST API 调用 + 错误码映射 |
| `src/api/event-type-schema.ts` | 扩展字段 Schema REST API 调用 + 错误码映射 |
| `src/stores/eventType.ts` | Pinia store：列表/详情/编辑状态 |
| `src/stores/eventTypeSchema.ts` | Pinia store：schema 列表 + 启用过滤 + reload |

### 前端改动文件

| 文件 | 改动内容 |
|---|---|
| `src/router/index.ts` | 新增 `/event-types` / `/event-types/new` / `/event-types/:id/edit` / `/schema-management` 路由 |
| `src/layout/Sidebar.vue`（或等价） | 侧边栏"行为管理"分组 + "事件类型"菜单项；"系统设置"分组 + "Schema 管理"菜单项 |

预估后端 ~2000 行新增 Go 代码 + 前端 ~2500 行新增 Vue/TS 代码。

---

## 扩展轴检查

ADMIN 平台两个扩展方向：

1. **新增配置类型只需加一组 handler/service/store/validator**：
   - ✅ 本需求**就是**新增两个配置类型（event_types + event_type_schema），完全套用字段/模板管理已建立的"四件套"模式
   - 约束校验抽出为独立 `service/constraint/` 包后，后续 FSM/BT/Region 的扩展字段直接调 `constraint.ValidateValue`，不重复编码
   - 路由注册、main.go 装配走相同模式，不破坏对称性
   - **正面**：建立"MySQL 单存储 + config_json 列 + 导出 API 直出"的新标准模式（区别于字段/模板的"纯 MySQL 内部概念"模式），后续 FSM/BT/Region 可直接复用

2. **新增表单字段只需加组件**：
   - ✅ SchemaForm.vue 是本需求引入的通用组件。运营在 Schema 管理页添加新扩展字段后，SchemaForm 根据 schema 数组自动渲染新的输入控件——**零前端代码改动**
   - FieldConstraint*.vue 被 Schema 管理页的 ConstraintPanel 复用，约束编辑能力免费获得
   - **正面**：首次实现"运营自助添加表单字段"的能力，是运营平台核心价值的落地

---

## 验收标准

### 事件类型 CRUD 接口

- **R1**：实现 7 个 REST 接口：`POST /api/v1/event-types/{list,create,detail,update,delete,check-name,toggle-enabled}`
- **R2**：所有接口走 `handler.WrapCtx` 包装，统一 `{Code, Data, Message}` 响应格式
- **R3**：所有写接口在请求异常时返回对应错误码（42001-42015），常量加入 `errcode/codes.go`

### 数据一致性

- **R4**：创建事件类型时，`config_json` = 系统字段 + 运营填过的扩展字段值（未填的扩展字段不进 JSON）
- **R5**：编辑事件类型时，全量替换 `config_json`。未在请求 `extensions` 里出现的扩展字段从 `config_json` 移除
- **R6**：`name` 一旦创建不可修改，UpdateRequest 不包含 name 字段
- **R7**：`name` 全局唯一含软删除，check-name 接口和创建接口都拦截
- **R8**：`enabled=1` 时拒绝编辑（42015）和删除（42012 前置，需先停用）
- **R9**：删除是软删除 `deleted=1`，name 仍占唯一性
- **R10**：所有 update/toggle-enabled 走 `WHERE id=? AND version=?` 乐观锁，rows=0 返回 42010
- **R11**：`perception_mode == "global"` 时 `range` 强制置 0（后端兜底）

### 扩展字段约束校验（契约承诺）

- **R12**：创建/编辑事件类型时，扩展字段值必须通过 `constraint.ValidateValue(schema.field_type, schema.constraints, value)` 校验，不符合返回 42007
- **R13**：扩展字段 key 必须在 `event_type_schema` 中存在且 `enabled=1`，否则 42022/42023
- **R14**：`constraint.ValidateValue` 从字段管理的值级校验逻辑抽出，字段管理原有功能不受影响
- **R15**：`constraint.ValidateConstraintsSelf` 校验约束自洽（如 int min <= max），在 Schema 创建/编辑时调用，不符合返回 42025

### 扩展字段 Schema 管理

- **R16**：实现 5 个 REST 接口：`POST /api/v1/event-type-schema/{list,create,update,toggle-enabled,delete}`
- **R17**：`field_name` 唯一含软删除，格式符合 `^[a-z][a-z0-9_]*$`
- **R18**：`field_type ∈ {int, float, string, bool, select}`，不支持 reference
- **R19**：`default_value` 必须符合自身 `constraints`，不符合返回 42026
- **R20**：删除前必须先停用（42027）；软删不对 `event_types.config_json` 做 `JSON_REMOVE`
- **R21**：启用/停用不触碰已有 event_types 行（存量不动增量拦截）

### 缓存

- **R22**：事件类型列表缓存使用版本号 `event_types:list:version`，写操作 INCR 即失效
- **R23**：事件类型详情缓存 key `event_types:detail:{id}`，TTL 10min ± jitter
- **R24**：详情查询使用分布式锁 `event_types:lock:{id}` 防击穿，锁后 double-check 缓存
- **R25**：缓存空值标记防穿透
- **R26**：Redis 不可用时降级到 MySQL 直查，slog.Warn 但不阻塞业务
- **R27**：扩展字段 Schema 使用内存缓存（`EventTypeSchemaCache`），启动时 Load，写后同步 Reload

### 导出 API

- **R28**：`GET /api/configs/event_types` 返回 `{"items": [{"name": "...", "config": ...}]}`
- **R29**：`config` 字段直接把 `config_json` 列原样展开，不经过 Go struct 中转
- **R30**：只导出 `enabled=1 AND deleted=0` 的记录
- **R31**：空数据返回 `{"items": []}`

### 前端

- **R32**：事件类型列表页：后端分页 + display_name 模糊搜索 + perception_mode facet 筛选 + enabled 三态筛选
- **R33**：停用行整行 opacity 0.5，操作列保持高亮
- **R34**：事件类型表单页由"系统字段区 + 扩展字段区"组成。系统字段硬编码，扩展字段通过 SchemaForm 动态渲染
- **R35**：SchemaForm 维护 dirty 状态：未被交互过的扩展字段不进提交 payload
- **R36**：`perception_mode == global` 时 range 输入框禁用并置 0
- **R37**：`default_severity` 使用 slider 0-100 配色带组件
- **R38**：启用中的事件类型进入编辑页，全部字段只读 + 顶部 banner "请先停用才能编辑"
- **R39**：编辑页顶部常驻 warning "修改感知参数后需通知运维重启游戏服务端才能生效"
- **R40**：Schema 管理页的"事件类型扩展字段"tab：列表 + 新建/编辑弹窗 + 约束编辑面板复用 FieldConstraint*.vue
- **R41**：前端 `npx vue-tsc --noEmit` 通过

### 可观测性

- **R42**：所有 service 入口加 slog.Debug，写操作成功加 slog.Info，错误用 slog.Error 记录上下文
- **R43**：handler 入口 slog.Debug 记录请求关键字段

---

## 不做什么

明确排除：

1. **`event_type_refs` 表和 `ref_count` 列**：等 FSM/BT 上线时再做迁移加列 + 补反向引用接口
2. **引用详情接口**（`/event-types/references`）：同上
3. **删除 TOCTOU 防护**（`FOR SHARE` 检查 `event_type_refs`）：等 FSM/BT
4. **Schema 编辑收紧拦截**（被引用时拒绝收紧约束）：不做硬拦截，前端二次确认即可
5. **`config_json` 历史字段清理**：放到毕设后
6. **导出响应 Redis 整包缓存**：量上来后再加
7. **系统字段热更新**：服务端启动时一次性加载，改动需重启
8. **服务端默认值对照 API**（`GET /api/runtime/event-type-defaults`）：延后
9. **MongoDB / RabbitMQ**：本模块不使用
10. **多实例 EventTypeSchemaCache 一致性**（Redis Pub/Sub 广播 reload）：本期单实例开发
11. **NPC 管理模块**：另起 spec
12. **FSM / BT / Region**：另起 spec
13. **审计日志**：全局延后功能

---

## 待审批确认事项

请审批以下细节后进入 Phase 2：

1. **错误码段位 42001-42039**：事件类型用 42001-42015，扩展字段 Schema 用 42020-42031。是否需要调整段位或预留间隔？
2. **字段管理约束校验抽包**：计划把 `service/field.go` 里 `checkConstraintTightened` 的值级校验逻辑抽到 `service/constraint/validate.go`，字段管理 service 改为调新包。这是一次辅助重构，字段管理的 199/199 测试必须继续通过。可接受？
3. **Schema 管理页位置**：计划放在侧边栏"系统设置"分组下的"Schema 管理"菜单项，页面内用 tab 切换不同 entity 的扩展字段。当前只有"事件类型"一个 tab，后续 FSM/BT/Region 上线时追加 tab。可接受？
4. **ExportHandler 位置**：新建 `handler/export.go` 统一放导出 API，还是每个配置类型的 handler 各自放一个导出方法？倾向前者（统一），因为导出 API 路径 `/api/configs/{collection}` 和 CRUD 路径 `/api/v1/event-types/*` 是不同的前缀。
