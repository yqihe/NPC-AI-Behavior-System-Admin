# 后端统一约定

本文档描述 ADMIN 后端（Go + Gin + MySQL + Redis）的分层模式与模块内约定。目标是让每一层的每个模块读起来都像同一个人写的——同类操作结构相同，差异只来自业务本身，不来自风格分歧。

通用 Go 规范见 `../development/standards/dev-rules/go.md`，禁止红线见 `../development/standards/red-lines/go.md`，Admin 项目专属规则见 `../development/admin/dev-rules.md`。

---

## 一、分层职责边界

四层各司其职，单向依赖，不可跨层调用：

**Handler**：HTTP 入口。唯一职责是格式校验（非空、长度、枚举合法性）+ 跨模块编排 + 响应包装。不含业务规则判断，不直接访问 store。

**Service**：业务逻辑层。负责分页规范化、缓存读写、业务约束校验（启用中不可编辑、被引用不可删除等）、存在性检查、store 哨兵错误翻译。

**Store/MySQL**：单表 CRUD。纯 SQL，不含任何业务判断，不了解"启用"、"软删除以外"等业务概念，只返回哨兵错误（见 `errcode/store_errors.go`）。

**Store/Redis**：Cache-Aside 缓存操作。Key 管理、分布式锁、空标记，不含业务逻辑。

目录结构与初始化链路详见 `CLAUDE.md` 目录结构一节，依赖装配详见 `backend/internal/setup/`。

---

## 二、Handler 层

### 方法命名

每个模块的 handler 方法固定为：`List`、`Create`、`Get`、`Update`、`Delete`、`CheckName`、`ToggleEnabled`，有引用查询的加 `GetReferences`。方法名与业务语义一一对应，不使用 `Detail` 或其他别名。

### 请求校验顺序

同一个方法内，校验严格按以下顺序：先校验 ID（`shared.CheckID`），再校验 Version（`shared.CheckVersion`），再校验其余格式字段（名称、标签、枚举），最后才打 `slog.Debug` 并调用 service。Debug 日志必须在所有校验通过之后才记录——如果记在前面，校验失败时也会打出无意义的日志。见 `handler/bt_tree.go` Create 方法的结构。

### shared 校验辅助

handler 层不手写格式判断逻辑，统一使用 `handler/shared/validate.go` 提供的 `CheckID`、`CheckVersion`、`CheckName`、`CheckLabel`、`SuccessMsg`。每个辅助函数的错误码语义固定，不同模块传入不同的业务错误码常量。

### 跨模块编排与事务

需要同时操作多个模块的写操作（模板写 field_refs、状态机写 BB Key refs），事务由 handler 负责开启和提交，每个 service 提供 `XxxInTx` 变体供 handler 调用。缓存失效必须在 `tx.Commit()` 之前执行，顺序不可颠倒，原因见 `handler/template.go` 和 `handler/fsm_config.go` 的注释。

单模块内部的事务（如 EventTypeSchema 的约束收紧保护）由 service 层自行开启，handler 不感知事务细节。

### Delete 请求类型

删除接口统一使用 `model.IDRequest`，不在删除请求中要求客户端传 `version`。删除前 service 通过 `getOrNotFound` 确认记录存在，防并发误删由业务约束（启用中不可删除）而非乐观锁承担。

---

## 三、Service 层

### `getOrNotFound` 模式

所有需要先查记录再操作的方法，统一通过私有方法 `getOrNotFound` 封装存在性检查，其实现如下逻辑：调 `s.store.GetByID`，若 `err != nil` 直接透传；若 `d == nil` 返回模块专属的 NotFound 错误。这个约定来自 store 层 `GetByID` 返回 `(nil, nil)` 表示不存在，而不是返回哨兵错误。见 `service/fsm_state_dict.go` 和 `service/bt_node_type.go`。

### List 方法结构

List 的执行顺序固定：① 调 `shared.NormalizePagination` 规范化分页参数（必须是第一步，store 层直接使用这些参数，不做二次规范化）；② 查 Redis，命中则直接返回；③ 查 MySQL；④ 写 Redis。Redis 故障时静默跳过，不阻断主流程。见 `service/bt_tree.go` List 方法。

### GetByID 方法结构

遵循 Cache-Aside + 分布式锁 + 空标记三层防护：先查缓存，缺失时加分布式锁（防击穿），加锁后 double-check，然后查 MySQL，结果（含 nil）写入缓存。MySQL 返回 `nil` 时缓存空标记，同样返回 NotFound，防止下次再穿透到 MySQL。见 `service/fsm_state_dict.go` GetByID 方法。

### 缓存失效时机

写操作的缓存失效时机有两种情况：

有事务的路径，缓存失效必须在 `tx.Commit()` 之前调用。原因：Commit 前失效，若 Commit 失败缓存已清空，下次请求会穿透到 MySQL 读到一致的旧数据，安全；若 Commit 后才失效，Commit 成功与失效之间存在窗口期，其他协程可能读到旧值并写回缓存，造成脏数据存活。

无事务的路径，MySQL 写成功后立即失效缓存。

所有写操作同时失效 List 缓存（`InvalidateList`）和 Detail 缓存（`DelDetail`）。

### store 哨兵错误翻译

store 层只返回三种哨兵错误：`ErrNotFound`、`ErrVersionConflict`、`ErrDuplicate`（均定义在 `errcode/store_errors.go`）。service 层用 `errors.Is` 捕获后翻译为模块专属业务码，例如 `ErrVersionConflict` → `ErrXxxVersionConflict`。业务码携带中文描述，供前端展示。

---

## 四、Store/MySQL 层

### `GetByID` 返回约定

未找到记录时返回 `(nil, nil)`，不返回哨兵错误。调用方（service 层）通过检查 `d == nil` 判断不存在。这是全库统一约定，见 `store/mysql/field.go`、`store/mysql/fsm_state_dict.go`、`store/mysql/bt_node_type.go` 等所有模块。

### List WHERE 构建

WHERE 条件统一用 `[]string` slice 积累，最后 `strings.Join(where, " AND ")` 拼接。不使用字符串直接拼接（容易漏空格、难以阅读）。初始元素始终是 `"deleted = 0"`，其余条件按查询参数按需追加。见 `store/mysql/bt_node_type.go` List 方法。

### 分页参数

store 的 List 方法直接使用 `q.Page` 和 `q.PageSize` 计算 OFFSET，不在 store 层做默认值 fallback。service 层的 `NormalizePagination` 已经保证进入 store 的分页参数合法，store 层不重复处理。

### 软删除

所有实体软删除：`UPDATE xxx SET deleted=1, updated_at=? WHERE id=? AND deleted=0`，0 rows 时返回 `ErrNotFound`。方法名统一为 `SoftDelete(ctx context.Context, id int64) error`，不带 version 参数。所有查询语句加 `AND deleted=0` 过滤。唯一性检查（`ExistsByName`）不加 `deleted=0`，保证已删除的标识不可复用。

### `ToggleEnabled` 签名

所有模块的 `ToggleEnabled` 方法统一接受 `*model.ToggleEnabledRequest` 结构体，不拆散为 `(id, enabled, version)` 散参数。这与 Update、Delete 等其他操作的风格一致。见 `store/mysql/bt_node_type.go` 和 `store/mysql/fsm_state_dict.go`。

### `Update` 乐观锁

所有 Update 语句带版本条件：`WHERE id=? AND version=? AND deleted=0`，同时 `SET version=version+1`。0 rows 时返回 `ErrVersionConflict`。

### List 列选择

List 查询只返回列表展示需要的核心列（通常是 id、name、label、enabled、created_at），不返回大字段（config、properties、fields 等 JSON 列）。GetByID 返回完整列。这是性能约定，不是风格约定。

### Tx 变体

需要在事务内被 handler 调用的方法提供 Tx 版本，命名为 `CreateInTx`、`UpdateInTx`、`SoftDeleteTx`，接受 `*sqlx.Tx` 作为第二参数。非事务版本和事务版本逻辑相同，区别只是使用 `s.db` 还是传入的 `tx`。

---

## 五、Redis 缓存层

### 缓存适用范围

除 EventTypeSchema 外，所有主要业务模块（字段、模板、事件类型、状态机、状态字典、行为树、节点类型）都使用 Redis Cache-Aside。EventTypeSchema 是全局小表，启动时全量加载到内存缓存（`cache/event_type_schema_cache.go`），每次写操作后调 `Reload()` 重建，不走 Redis。

### Key 管理

所有 Redis key 通过 `store/redis/config/keys.go` 中的函数生成，不在业务代码里拼字符串。List key 基于查询参数的哈希生成，Detail key 基于 ID。

### 分布式锁

`GetByID` 的缓存穿透防护使用分布式锁，锁的 TTL 通过 `store/redis/shared/common.go` 中的 `LockExpire` 常量统一配置，不在各模块硬编码。

---

## 六、Model 层

### DB 结构体公共字段

所有持久化实体包含：`id`（int64）、`enabled`（bool）、`version`（int）、`created_at`、`updated_at`、`deleted`（bool，json tag 为 `"-"` 不对外暴露）。

### 列表 vs 详情结构体

List 查询返回专用的轻量结构体（`XxxListItem`），只含列表展示需要的字段。GetByID 返回完整实体结构体。两者分开定义，不共用。

### 通用请求/响应结构体

以下结构体所有模块共用，不各自重复定义：`IDRequest`、`ToggleEnabledRequest`、`CheckNameRequest`、`CheckNameResult`、`DeleteResult`、`ListData`。仅当某模块的删除响应需要携带额外信息（如 `ReferencedBy` 列表）时才定义专属结构体，见 `model/bt_node_type.go` 的 `BtNodeTypeDeleteResult`。

---

## 七、错误码体系

错误分三层流动：store 只返回哨兵错误，service 翻译为模块业务码，handler 的 `WrapCtx` 将 `*errcode.Error` 写入 HTTP 响应。

所有错误码定义在 `errcode/codes.go`，按模块分段，每段包含固定含义的几类错误：NameExists、NotFound、VersionConflict、DeleteNotDisabled、EditNotDisabled，以及各模块特有的业务约束错误。每个错误码在 `messages` map 中对应一条中文提示。

store 哨兵错误定义在 `errcode/store_errors.go`，与业务码隔离。

---

## 八、日志规范

**Debug**：记录每次请求的关键入参，在所有格式校验通过之后、调用 service 之前打印。handler 层和 service 层都可以有 Debug 日志，分别标注来源前缀（`handler.xxx` / `service.xxx`）。

**Info**：记录成功的写操作，包含操作结果的关键字段（id、name 等）。仅在写操作成功后记录，不在读操作上记录。

**Warn**：记录非致命异常，包括 Redis 故障、跨模块补全失败、事务回滚失败。这类情况不阻断请求，但需要留痕。

**Error**：记录系统级错误，包括 MySQL 写失败、事务 commit 失败、无法解释的业务异常。出现 Error 日志意味着请求大概率失败。

---

## 九、参考实现

以下模块覆盖了所有模式的典型用法，开发新模块时以这些为对照：

**字段管理**（`handler/field.go` + `service/field.go` + `store/mysql/field.go`）：标准单模块 CRUD，包含 properties JSON 校验、BB Key 引用、GetByID 完整 Cache-Aside 流程。

**模板管理**（`handler/template.go` + `service/template.go` + `store/mysql/template.go`）：跨模块事务编排范本，handler 层开事务、调多个 service、commit 前失效多模块缓存。

**状态机配置**（`handler/fsm_config.go` + `service/fsm_config.go`）：跨模块事务 + BB Key diff 追踪，与模板管理并列作为跨模块编排的参考。

**状态字典**（`handler/fsm_state_dict.go` + `service/fsm_state_dict.go`）：Delete 带引用检查并返回 `ReferencedBy` 列表的模式，以及 `getOrNotFound` 的 `d==nil` 判断模式。

**节点类型**（`handler/bt_node_type.go` + `service/bt_node_type.go`）：同上，Delete 被引用时返回携带引用列表的专属结构体，结合 `WrapCtx` 的错误携带 data 机制。

**扩展字段 Schema**（`handler/event_type_schema.go` + `service/event_type_schema.go`）：内存缓存（非 Redis）模式、service 层内部事务、约束收紧保护（Update 带 FOR SHARE 锁）。
