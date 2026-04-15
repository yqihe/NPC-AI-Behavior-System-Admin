# 行为树管理 — 任务拆解（后端）

> 对应设计：[design.md](./design.md)
> 对应需求：[requirements.md](./requirements.md)

---

## T1: DDL — bt_node_types 和 bt_trees 表 (R7, R9, R19) [x]

**涉及文件**：
- `backend/migrations/010_create_bt_node_types.sql`（新增）
- `backend/migrations/011_create_bt_trees.sql`（新增）

**做完了是什么样**：
- `010_create_bt_node_types.sql` 可执行，含 `uk_type_name`、`idx_list (deleted, enabled, category, id DESC)` 索引，`is_builtin` 列默认 0，`enabled` 默认 1
- `011_create_bt_trees.sql` 可执行，含 `uk_name`、`idx_list (deleted, enabled, id DESC)` 索引，`enabled` 默认 0（创建默认禁用）
- 两个文件均有注释说明表用途和字段含义

---

## T2: 错误码 440xx (R3, R12) [x]

**涉及文件**：
- `backend/internal/errcode/codes.go`（改动）

**做完了是什么样**：
- `codes.go` 新增 `// --- 行为树管理 440xx ---` 段，定义 44001-44015（bt_tree）和 44016-44025（bt_node_type）共 20 个错误码常量
- `messages` map 新增对应中文消息
- 段间预留注释标注占位码（如 44012 `// 占位，NPC 管理完成后激活`）

---

## T3: 配置结构 BtTreeConfig + BtNodeTypeConfig (R20) [x]

**涉及文件**：
- `backend/internal/config/config.go`（改动）
- `backend/configs/config.yaml`（改动）

**做完了是什么样**：
- `config.go` 新增 `BtTreeConfig`（NameMaxLength、DisplayNameMaxLength、CacheDetailTTL、CacheListTTL）和 `BtNodeTypeConfig`（NameMaxLength、LabelMaxLength、CacheDetailTTL、CacheListTTL）结构体
- `AppConfig` 新增 `BtTree BtTreeConfig` 和 `BtNodeType BtNodeTypeConfig` 字段
- `config.yaml` 新增对应默认值（name_max_length: 128、display_name_max_length: 128、label_max_length: 128、cache_detail_ttl: 10m、cache_list_ttl: 5m）
- `go build ./...` 通过

---

## T4: Model — bt_node_type (R11-R19) [x]

**涉及文件**：
- `backend/internal/model/bt_node_type.go`（新增）

**做完了是什么样**：
- 包含结构体：`BtNodeType`（DB 行）、`BtNodeTypeListItem`（列表项）、`BtNodeTypeDetail`（详情）、`BtNodeTypeListData`（含 `ToListData()` 方法）、`CreateBtNodeTypeRequest`、`UpdateBtNodeTypeRequest`、`BtNodeTypeListQuery`
- 所有字段含 `json` 和 `db` tag，`Deleted` 字段 `json:"-"`
- `json.RawMessage` 用于 `ParamSchema`（非指针，列不可空）
- `go build ./...` 通过

---

## T5: Model — bt_tree (R1-R10, R27-R29) [x]

**涉及文件**：
- `backend/internal/model/bt_tree.go`（新增）

**做完了是什么样**：
- 包含结构体：`BtTree`（DB 行）、`BtTreeListItem`（列表项，不含 config）、`BtTreeDetail`（含 config + version）、`BtTreeListData`（含 `ToListData()` 方法）、`CreateBtTreeRequest`、`UpdateBtTreeRequest`、`BtTreeListQuery`、`BtTreeExportItem`（导出用 `{Name, Config}`）
- `Config` 字段类型为 `json.RawMessage`
- `go build ./...` 通过

---

## T6: Redis Key 管理 (R32-R35) [x]

**涉及文件**：
- `backend/internal/store/redis/config/keys.go`（改动）

**做完了是什么样**：
- 新增 `bt_trees` 和 `bt_node_types` 前缀常量（小写，包内可见）
- 新增公开常量：`BtTreeListVersionKey`、`BtNodeTypeListVersionKey`
- 新增函数：`BtTreeListKey(version, name, displayName, enabled, page, pageSize)`、`BtTreeDetailKey(id)`、`BtTreeLockKey(id)`、`BtNodeTypeListKey(version, typeName, category, enabled, page, pageSize)`、`BtNodeTypeDetailKey(id)`、`BtNodeTypeLockKey(id)`
- `go build ./...` 通过

---

## T7: BtNodeTypeStore — MySQL CRUD (R11-R19) [x]

**涉及文件**：
- `backend/internal/store/mysql/bt_node_type.go`（新增）

**做完了是什么样**：
- `BtNodeTypeStore` 实现：`Create`、`GetByID`、`ExistsByTypeName`、`List`（分页 + type_name 前缀 + category 精确 + enabled 筛选）、`Update`（乐观锁，只更新 label/description/param_schema）、`Delete`（软删除 + 乐观锁，is_builtin=1 由 service 层预检）、`ToggleEnabled`（乐观锁）
- `ListEnabledTypes(ctx) (map[string]string, error)`：返回所有 `enabled=1 AND deleted=0` 的 `type_name → category` map，供 BtTreeService 节点校验用
- List 查询 `type_name LIKE` 使用 `shared.EscapeLike()` 转义
- `go build ./...` 通过

---

## T8: BtTreeStore — MySQL CRUD (R1-R10) [x]

**涉及文件**：
- `backend/internal/store/mysql/bt_tree.go`（新增）

**做完了是什么样**：
- `BtTreeStore` 实现：`Create`、`GetByID`、`ExistsByName`、`List`（分页 + name 前缀 + display_name 模糊 + enabled 筛选）、`Update`（乐观锁，更新 display_name/description/config）、`Delete`（软删除 + 乐观锁）、`ToggleEnabled`（乐观锁）
- `ExportAll(ctx) ([]model.BtTreeExportItem, error)`：查 `enabled=1 AND deleted=0`，仅取 name + config 列
- name 前缀查询：`name LIKE ? AND name LIKE 'xxx%'`（不用 `%` 中缀，前缀匹配命中索引）
- `go build ./...` 通过

---

## T9: BtTreeStore — BB Key 扫描方法 (R30, R31) [x]

**涉及文件**：
- `backend/internal/store/mysql/bt_tree.go`（改动）

**前置**：T7（`ListEnabledTypes` 提供 nodeParamTypes）、T8

**做完了是什么样**：
- 新增私有函数 `extractBBKeys(node map[string]any, nodeParamTypes map[string][]string) []string`：递归遍历节点树，按 nodeParamTypes 找出所有 `type=bb_key` 参数位置的值
- 新增 `IsBBKeyUsed(ctx, bbKey string) (bool, error)`：全量扫描 `deleted=0` 的 bt_trees.config，调用 `extractBBKeys`，json.Unmarshal 失败时返回 error（不 continue 跳过）
- 新增 `GetBBKeyUsages(ctx, bbKey string) ([]string, error)`：返回引用该 key 的行为树 name 列表
- `nodeParamTypes` 由调用方从 `BtNodeTypeStore.ListEnabledTypes` 获取后传入（避免循环依赖）
- `go build ./...` 通过

---

## T10: BtNodeTypeCache — Redis (R32-R35) [x]

**涉及文件**：
- `backend/internal/store/redis/bt_node_type_cache.go`（新增）

**前置**：T6

**做完了是什么样**：
- `BtNodeTypeCache` 实现：`GetDetail / SetDetail / DelDetail`（含 TTL+jitter）、`TryLock / Unlock`（LuaUnlock，携带 lockID）、`GetList / SetList / IncrListVersion / GetListVersion`（版本号方案）
- 空标记处理（缓存 NOT_FOUND 防穿透）
- Redis 不可用时返回 error（service 层降级）
- `go build ./...` 通过

---

## T11: BtTreeCache — Redis (R32-R35) [x]

**涉及文件**：
- `backend/internal/store/redis/bt_tree_cache.go`（新增）

**前置**：T6

**做完了是什么样**：
- `BtTreeCache` 实现与 T10 同构
- `go build ./...` 通过

---

## T12: BtNodeTypeService (R11-R20, R36, R37) [x]

**涉及文件**：
- `backend/internal/service/bt_node_type.go`（新增）

**前置**：T4、T7、T10

**做完了是什么样**：
- `BtNodeTypeService` 实现：`List`、`Create`、`GetByID`（含 detail 缓存 TryLock + double-check）、`Update`、`Delete`、`ToggleEnabled`、`CheckName`
- `Create/Update` 前校验 param_schema：合法 JSON 对象、`params` 是数组、每个 param 含 name/label/type、type 枚举合法、`select` 类型必须有非空 options；category 枚举合法（composite/decorator/leaf）
- `Delete` 前检查：is_builtin=1 返回 44023；enabled=1 返回 44020；调用 `BtTreeStore.IsBBKeyUsed` 以外——实际是扫描 bt_tree.config 中 type_name 使用情况（需要 `BtTreeStore.IsNodeTypeUsed`）

> **注意**：T9 实现的是 BB Key 扫描，节点类型使用检查是另一个方向——扫描 bt_tree.config 中是否有 `type == typeName` 的节点。需在 T8/T9 中同步补充 `IsNodeTypeUsed(ctx, typeName string) (bool, error)` 和 `GetNodeTypeUsages(ctx, typeName string) ([]string, error)` 方法。

- store 错误 `slog.Error` + `fmt.Errorf` 包装
- 缓存读取用 `err == nil && hit` 模式
- `go build ./...` 通过

---

## T8b: BtTreeStore — NodeType 使用检查（T12 前置补充） (R14) [x]

**涉及文件**：
- `backend/internal/store/mysql/bt_tree.go`（改动）

**前置**：T8

**做完了是什么样**：
- 新增 `IsNodeTypeUsed(ctx, typeName string) (bool, error)`：扫描 `deleted=0` 的 bt_trees.config，递归遍历检查是否有 `type == typeName` 的节点；json.Unmarshal 失败返回 error
- 新增 `GetNodeTypeUsages(ctx, typeName string) ([]string, error)`：返回使用该 type 的行为树 name 列表
- 共用 T9 的递归遍历辅助逻辑（提取为 `walkNodes(node, visit func(map[string]any))`）
- `go build ./...` 通过

---

## T13: BtTreeService (R1-R10, R21-R26, R36, R37) [x]

**涉及文件**：
- `backend/internal/service/bt_tree.go`（新增）

**前置**：T5、T8、T8b、T11

**做完了是什么样**：
- `BtTreeService` 实现：`List`、`Create`、`GetByID`、`Update`、`Delete`、`ToggleEnabled`、`CheckName`、`ExportAll`
- `validateBtNode(node, nodeTypes, depth int) error`：递归校验，composite 强制 children 非空、decorator 强制单 child、leaf 禁止 children/child，depth > 20 返回 44006，type 不在 nodeTypes 返回 44005（admin 红线 1.4：decorator/composite 严格区分）
- `Create/Update` 调用 `BtNodeTypeStore.ListEnabledTypes` 预加载节点类型 map，再调 `validateBtNode`
- `enabled=1` 时 Update/Delete 返回 44010/44009
- 乐观锁冲突返回 44011
- 缓存模式与已有模块一致（TryLock + double-check、版本号方案）
- `go build ./...` 通过

---

## T14: BtNodeTypeHandler (R11-R20) [x]

**涉及文件**：
- `backend/internal/handler/bt_node_type.go`（新增）

**前置**：T2、T12

**做完了是什么样**：
- `BtNodeTypeHandler` 实现 7 个方法：`List`、`Create`、`Detail`、`Update`、`Delete`、`ToggleEnabled`、`CheckName`
- `CheckName` 先调 `shared.CheckName(req.Name, cfg.NameMaxLength, ErrBtNodeTypeNameInvalid, "节点类型标识")` 再查 DB（admin 红线 4b.5）
- `Update` 调 `shared.CheckID` + `shared.CheckVersion`；`Delete` 调 `shared.CheckID`
- `Delete` 响应为 `*model.DeleteResult{ID, Name: typeName, Label}`（admin 红线 10.1）
- `ToggleEnabled` 响应为 `shared.SuccessMsg("操作成功")`
- slog.Debug 在校验通过后记录
- `go build ./...` 通过

---

## T15: BtTreeHandler (R1-R10) [x]

**涉及文件**：
- `backend/internal/handler/bt_tree.go`（新增）

**前置**：T2、T13

**做完了是什么样**：
- `BtTreeHandler` 实现 7 个方法，模式与 T14 相同
- `CheckName` 正则 `^[a-z][a-z0-9_/]*$`（含斜杠，与字段标识不同），使用对应错误码 `ErrBtTreeNameInvalid`
- `go build ./...` 通过

---

## T16: ExportHandler — BTTrees (R27-R29) [x]

**涉及文件**：
- `backend/internal/handler/export.go`（改动）

**前置**：T13

**做完了是什么样**：
- `ExportHandler.BTTrees(c *gin.Context)` 调用 `BtTreeService.ExportAll`，返回 `{"items": [{"name": "...", "config": {...}}]}`
- 空数据返回 `{"items": []}` 而非 null（Go nil slice 用 `make([]T, 0)`）
- `go build ./...` 通过

---

## T17: Router 注册 BT 路由 (R1, R11) [x]

**涉及文件**：
- `backend/internal/router/router.go`（改动）

**前置**：T14、T15、T16

**做完了是什么样**：
- 注册 7 条 `/api/v1/bt-trees/*` 路由 + 7 条 `/api/v1/bt-node-types/*` 路由（均用 `handler.WrapCtx` 包装）
- 注册 `GET /api/configs/bt_trees` 导出路由（不用 WrapCtx，直接调 `ExportHandler.BTTrees`）
- `go build ./...` 通过

---

## T18: FieldService — BB Key 检查激活 (R30, R31) [ ]

**涉及文件**：
- `backend/internal/service/field.go`（改动）

**前置**：T9

**做完了是什么样**：
- `FieldService` 结构体新增 `btTreeStore *storemysql.BtTreeStore` 字段
- `NewFieldService` 签名新增 `btTreeStore *storemysql.BtTreeStore` 参数
- `Update` 方法中 `expose_bb true→false` 分支：调用 `s.btTreeStore.IsBBKeyUsed(ctx, name)`，返回 true 时返回 `errcode.New(errcode.ErrFieldBBKeyInUse)`
- `go build ./...` 通过

---

## T19: Setup — 全链路装配 (R1-R37) [ ]

**涉及文件**：
- `backend/internal/setup/services.go`（改动）

**前置**：T17、T18（所有 Store/Service/Handler 已就绪）

**做完了是什么样**：
- 实例化 `BtNodeTypeStore`、`BtTreeStore`、`BtNodeTypeCache`、`BtTreeCache`
- 实例化 `BtNodeTypeService`、`BtTreeService`（各注入对应 Store + Cache + Config）
- 实例化 `BtNodeTypeHandler`、`BtTreeHandler`（各注入对应 Service + Config）
- 将 `BtTreeStore` 作为新参数传入 `NewFieldService`（激活 BB Key 检查）
- 注册到 Router
- `go build ./...` 通过，`docker compose up` 后 `GET /api/configs/bt_trees` 返回 `{"items":[]}`

---

## T20: 种子数据 — 内置节点类型 (R20) [ ]

**涉及文件**：
- `backend/cmd/seed/bt_node_type_seed.go`（新增）

**前置**：T19（服务可运行）

**做完了是什么样**：
- 定义 8 种内置节点类型（sequence / selector / parallel / inverter / check_bb_float / check_bb_string / set_bb_value / stub_action）及其 param_schema
- 幂等执行：按 `type_name` 检查存在性，已存在则跳过，不存在则 INSERT（`is_builtin=1`）
- `go run ./cmd/seed` 可成功执行，执行后 `GET /api/v1/bt-node-types` 返回 8 条记录
- 重复执行不报错、不重复插入

---

## 任务依赖图

```
T1 ──────────────────────────────────────────────────────────────→ T19
T2 ──────────────────────────────────────────────────────────────→ T19
T3 ──────────────────────────────────────────────────────────────→ T19
T4 → T7 → T8 → T8b → T9 → T12 → T14 ──────────────────────────→ T17 → T19 → T20
T5 → T8 → T8b ──────────→ T13 → T15 ──────────────────────────→ T17
                              ↓         T16 ────────────────────→ T17
T6 → T10 ────────────────→ T12
T6 → T11 ────────────────→ T13
T9 → T18 ───────────────────────────────────────────────────────→ T19
```

**推荐执行顺序**（串行）：T1 → T2 → T3 → T4 → T5 → T6 → T7 → T8 → T8b → T9 → T10 → T11 → T12 → T13 → T14 → T15 → T16 → T17 → T18 → T19 → T20
