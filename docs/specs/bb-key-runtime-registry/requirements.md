# bb-key-runtime-registry — 需求分析

## 动机

**联调快照 2026-04-18 的 ⚠️ 问题 #3 的正面收拾**：策划在 FSM 条件里写 `max_hp < 50`、`perception_range > 0` 等条件，行为不符合预期——这些是 NPC 静态字段，创建时写入后运行时不再变化，`max_hp < 50` 永假、`perception_range > 0` 永真，FSM 转移等价于死代码（详见 `docs/integration/admin-snapshot-2026-04-18.md` 第 2 节分歧点）。

**根因**：ADMIN 把 BB Key 等同于"字段"（`fields.expose_bb=true`），没有"运行时变量"的概念。而游戏服务端在 `internal/core/blackboard/keys.go` 静态声明了 31 个运行时 key（`threat_level` / `fsm_state` / `current_time` / `npc_pos_x` / `move_state` / `emotion_dominant_val` 等），这些是 FSM 转移和 BT 条件应当引用的"真正的动态信号"。策划在 `BBKeySelector` 下拉里看不到它们，只能将就用静态字段 → 写出死条件。

**不做的后果**：
- 毕设联调里 FSM 转移逻辑大部分是死代码（snapshot 中 `fsm_combat_basic` 的 12 条 transitions 只有空条件的那几条真正生效），展示效果差
- 新建 FSM 的策划会延续错误用法——每多一棵 FSM，多一批死转移条件；发现时治理成本成倍增加
- BT `check_bb_float` / `check_bb_string` 同样存在此坑，但因为 `bt-data-format-unification` 已经把 `params.key` 接入了严格 validator，后续策划配 BT 时选错 key 只会报 400，不会产生错误运行时行为——所以 BT 那头损失小于 FSM

## 优先级

🟡 **中**。

- 已完成的 🔴 `bt-data-format-unification` 解决了"NPC spawn 不起来"的致命问题，6/6 NPC 能启动
- 本 spec 解决的是"FSM/BT 条件语义正确性"——NPC 能跑但行为不合理
- 不做这个不阻断 e2e 跑通，但毕设**答辩展示效果**明显打折（FSM 演示只能看到 `idle↔patrol` 死循环，看不到血量低触发 flee、感知到敌人触发 chase 等真正展示智能行为的转移）
- 在"联调 4 项阻塞"的原分级里标为 ⚪，当时只考虑 spawn 能否成功；现在 spawn 已通，需重估为 🟡——因为演示价值直接依赖它

## 预期效果

**场景 1：策划在 FSM 条件编辑器里选"运行时 Key"**  
策划打开 `FsmConditionEditor.vue`，点 BB Key 下拉框，除了现有的"NPC 字段"（`max_hp` 等）和"事件扩展字段"（`range` 等）两组，多出第三组"运行时 Key"。展开后看到 31 条分节呈现的运行时 key（分组按：威胁 / 事件 / FSM / NPC / 行为 / 需求 / 情绪 / 记忆 / 社交 / 决策 / 移动），每项含中文标签 + name + 类型 + 一行描述。选 `threat_level`，条件编辑器自动识别为 `float64` 类型，运算符下拉只允许数值运算符。

**场景 2：策划保存引用了运行时 key 的 FSM 配置**  
策划提交 `{from: "patrol", to: "chase", condition: {op: ">=", key: "threat_level", value: 30}}`。后端接收、持久化、成功，无"未知字段"报错。`GET /api/configs/fsm_configs` 导出后服务端 `fsm.go` 读取到该转移条件，`current_tick` 时若 `blackboard.Get(bb, threat_level) >= 30` 则触发转移。

**场景 3：策划删除被引用的运行时 key**  
管理员试图在"运行时 Key 管理"页面删除 `threat_level`，前端 detail 发现 `has_refs=true`，UI 锁定删除按钮，引用详情显示"被 FSM `fsm_combat_basic` 引用（1 处）"。不让删。

**场景 4：策划编辑已存在的 FSM，把 `max_hp < 50` 改为 `threat_level > 50`**  
打开 `FsmList.vue` → 编辑 `fsm_combat_basic` → 定位到 `idle→flee` 转移 → BB Key 从 `max_hp`（NPC 字段）改选 `threat_level`（运行时 Key）→ 保存。保存路径正确同步引用关系：`field_refs` 里对 `max_hp` 的 FSM→Field 引用被移除，`runtime_bb_key_refs`（新表）里对 `threat_level` 的 FSM→RuntimeKey 引用被添加。

**场景 5：运营新增一个运行时 key**  
未来服务端新增了 `current_hp`（比如服务端下一版本加动态血量）。运营在 ADMIN"运行时 Key 管理"页面新建一条：`name=current_hp, type=float, label=当前血量, description=实时变化的 HP`。保存后立即出现在 BB Key 下拉，策划可在 FSM/BT 中引用。**无需改 ADMIN 代码**。

**场景 6：命名冲突保护**  
策划想新增运行时 key `max_hp`——但 `max_hp` 已经是 fields 表里的字段。后端返回 `409 + ErrRuntimeBBKeyNameConflictWithField`，前端提示"name 已被字段 'max_hp' 占用"。反之亦然：新增字段 `threat_level` 会撞已有运行时 key。

## 依赖分析

**上游依赖（已完成）**：
- `bt-data-format-unification`（commit `747b0c3`）：BT validator 已经基于 `params.key` 做严格校验，后续扩大 key 白名单时不需要再动 BT validator 结构
- `fields` 表 + `field_refs` 表 + `SyncFsmBBKeyRefs` / `SyncBtBBKeyRefs`（commit 历史中已就绪）：现有 BB Key 引用同步机制成熟，可借鉴模式
- 服务端 `internal/core/blackboard/keys.go`：31 个 runtime key 的**权威定义源**（本 spec 的种子数据来自此文件逐一对齐）

**下游被依赖（本 spec 解锁）**：
- 下一次"FSM 条件语义审查 + 批量改条件"运维动作（把 6 棵现有 FSM 的死条件改对，属于数据治理，不在本 spec 范围）
- 答辩演示里的"智能 NPC"故事（血量低触发 flee / 感知到敌人触发 chase 等真实转移）

**不依赖**：
- **不依赖** V3 组件化架构（`component_schemas.blackboard_keys`）：那个是更长远的"每个组件声明自己的 BB Key 集合"路线，与本 spec 的"运行时注册表"是**两种不同来源**的 key，可并存。V3 落地后，BB Key 来源升级为 (a) 字段 expose_bb (b) 运行时注册表 (c) 已启用组件声明的 key——本 spec 做 (b)，不动 (a)(c)
- **不依赖**服务端做任何改动：服务端 `keys.go` 是权威源，ADMIN 只做**镜像**（手工 seed 对齐 + 运营可后续通过 CRUD 增量跟进），CLAUDE.md 明确规定"不走 API 互拉"

## 改动范围

**新增文件（预估 12 个）**：

| 类别 | 文件 | 预估行数 |
|---|---|---|
| migration | `backend/migrations/XXX_create_runtime_bb_keys.sql` | ~30 |
| migration | `backend/migrations/XXX_create_runtime_bb_key_refs.sql` | ~20 |
| model | `backend/internal/model/runtime_bb_key.go` | ~120 |
| store mysql | `backend/internal/store/mysql/runtime_bb_key.go` | ~200 |
| store mysql | `backend/internal/store/mysql/runtime_bb_key_ref.go` | ~80 |
| store redis | `backend/internal/store/redis/runtime_bb_key_cache.go` | ~150 |
| service | `backend/internal/service/runtime_bb_key.go` | ~250 |
| handler | `backend/internal/handler/runtime_bb_key.go` | ~180 |
| seed | `backend/cmd/seed/runtime_bb_key_seed.go`（种 31 条） | ~150 |
| frontend api | `frontend/src/api/runtimeBbKeys.ts` | ~60 |
| frontend view | `frontend/src/views/RuntimeBbKeyList.vue` + `RuntimeBbKeyForm.vue` | ~400 |
| test | `backend/internal/service/runtime_bb_key_test.go` | ~200 |

**修改文件（预估 8 个）**：

| 文件 | 动作 | 预估行数 |
|---|---|---|
| `backend/internal/service/field.go` | `SyncFsmBBKeyRefs`/`SyncBtBBKeyRefs` 分叉处理"字段 key"与"运行时 key"两类 | +80 / -10 |
| `backend/internal/service/fsm_config.go` | Create/Update/Delete 同步调用 runtimeBbKeyService 的 ref 同步 | +40 |
| `backend/internal/service/bt_tree.go` | 同上 | +40 |
| `backend/internal/errcode/codes.go` | 新增 ~6 个错误码 | +20 |
| `backend/internal/router/router.go` | 注册 `/api/v1/runtime-bb-keys` | +10 |
| `backend/internal/setup/*` | 装配 store/service/handler | +30 |
| `frontend/src/components/BBKeySelector.vue` | 新增第三组"运行时 Key" + 规范化类型 | +50 |
| `frontend/src/router/index.ts` + `Sidebar.vue` | 菜单接入 | +20 |

**总计约 ~2200 行（新增 + 修改）**。

## 扩展轴检查

**扩展轴 1（新增配置类型）**：🟢 **正面影响**  
本 spec 就是"新增一种配置类型"的典范——新增 `runtime_bb_keys` 完整技术栈（migration + model + store + service + handler + frontend），**完全不改 fields / fsm / bt 等已有模块的 handler/service/store**，仅在 `fsm_config.go` / `bt_tree.go` 的 ref 同步调用点、以及 `BBKeySelector.vue` 下拉数据源处，新增一条平行的分支（非侵入式扩展）。证明"新增配置类型"这条扩展路径可用。

**扩展轴 2（新增表单字段）**：⚪ **无影响**  
本 spec 涉及的 `RuntimeBbKeyForm` 是一个新建的独立表单（字段固定：name / type / label / description），不使用 SchemaForm 动态渲染（运行时 key 的定义是**静态 schema**，无扩展需求）。因此既不验证也不破坏 SchemaForm 的扩展性。

## 验收标准

| 编号 | 标准 | 验证方式 |
|---|---|---|
| R1 | 新建 `runtime_bb_keys` 表：id/name/type/label/description/enabled/version/deleted/created_at/updated_at，唯一索引 uk_name，覆盖索引 idx_list | `DESCRIBE runtime_bb_keys` 字段一致 + 索引核对 |
| R2 | 新建 `runtime_bb_key_refs` 表：runtime_key_id/ref_type/ref_id 三元组，唯一索引 | `DESCRIBE runtime_bb_key_refs` |
| R3 | seed 写入 31 条与游戏服务端 `internal/core/blackboard/keys.go` 严格对齐的运行时 key（逐条对照 name + type） | `SELECT COUNT(*) FROM runtime_bb_keys` = 31；逐条 name/type 与 keys.go 一致（测试里打 assertion） |
| R4 | 完整 REST API：`GET /api/v1/runtime-bb-keys`（分页 + 搜索 name/label/type/enabled）、`POST`、`PUT`、`DELETE`、`POST /check-name`、`GET /:id`、`GET /:id/references`、`POST /:id/toggle` | e2e curl 脚本按上述 endpoints 全通，返回格式对齐 handler/shared 包装 |
| R5 | `POST /api/v1/runtime-bb-keys` 传入与 fields.name 冲突的 name，返回 409 + `ErrRuntimeBBKeyNameConflictWithField` | 先创建 field `hp_foo` → 再 POST 运行时 key `hp_foo` → 响应码校验 |
| R6 | 反向：`POST /api/v1/fields` 传入与 runtime_bb_keys.name 冲突的 name，返回 409 + `ErrFieldNameConflictWithRuntimeBBKey`（新增错误码） | 同上对称测试 |
| R7 | FSM 条件里引用运行时 key，保存 FSM 时同步写 `runtime_bb_key_refs`（ref_type=fsm, ref_id=fsmID），删除 FSM 时级联清 | Create FSM 含 `key=threat_level` → `SELECT * FROM runtime_bb_key_refs` 命中；删除 FSM → 该行消失 |
| R8 | BT 节点里引用运行时 key，同上 | 同上针对 BT |
| R9 | 删除被 FSM/BT 引用的运行时 key 返回 409 + `ErrRuntimeBBKeyHasRefs`，detail 响应含 `has_refs: true` | `DELETE /api/v1/runtime-bb-keys/:id` 返回 409；`GET /:id` 响应含 `has_refs=true` |
| R10 | `BBKeySelector.vue` 下拉新增第三组"运行时 Key"；三组数据源独立并行加载；全部为空时显示"暂无可用 BB Key" | 前端 e2e：FSM 编辑页下拉含 "运行时 Key" 分组 + 31 条 + 类型图标 |
| R11 | `BBKeySelector` 选中运行时 key 时，`field-selected` emit 的 BBKeyField.type 规范化为 `'integer' / 'float' / 'string' / 'bool'`（与 FSM 条件编辑器期望的类型名一致） | 单测 / 组件测试：选 `threat_level` → `type='float'`；选 `leader_lost` → `type='bool'` |
| R12 | 导出接口 `GET /api/configs/*` **不包含** runtime_bb_keys——运行时 key 的权威定义在服务端 `keys.go`，ADMIN 只是 UI 辅助表 | grep export handler 代码无 runtime_bb_key 字样；服务端 HTTPSource 端点列表保持 4 个不变 |
| R13 | `runtime-bb-keys/:id/toggle` 停用后，引用该 key 的 FSM/BT 仍可导出（停用仅阻断"新建引用"，不影响历史数据） | 停用 `threat_level` → 再 Create 引用它的 FSM 返回 400；已存在的引用保留；`GET /api/configs/fsm_configs` 仍输出该 key | 
| R14 | 前端 `RuntimeBbKeyList.vue` + `RuntimeBbKeyForm.vue` 跑通 vue-tsc 类型检查无报错 | `npx vue-tsc --noEmit` 无输出 |
| R15 | 所有 runtime_bb_key CRUD 路径走与其他模块一致的模式（乐观锁 version、软删 deleted、`has_refs` 返回、`EnabledGuardDialog` 整合） | 代码审查：model 字段齐、handler/service 调用对齐其他模块、详情响应 `has_refs` 布尔 |
| R16 | Redis 缓存：detail 单条缓存 + list 分页缓存 + distinct 缓存；Commit 前清缓存（红线 16） | 代码审查 service 内 DelDetail/InvalidateList 调用顺序在 `tx.Commit()` 之前 |

## 不做什么

- **不做服务端运行时 key 的自动同步**：CLAUDE.md 明确"ADMIN 和游戏服务端各存各的，不走 API 互拉"。服务端 `keys.go` 是权威定义源，ADMIN seed 手工对齐 31 条，后续服务端新增时由运营在 ADMIN UI 手工补录（或下一轮 seed）。**不实现 HTTP 拉取/推送 / 文件同步脚本**
- **不改造 V3 `component_schemas.blackboard_keys` 集成**：组件化 schema 是独立路径，与本 spec 并行存在，延后到 V3 重写
- **不做存量 FSM 数据治理**：本 spec 只**启用**策划把 `max_hp<50` 改为 `threat_level>50` 的能力；实际改数据是运维操作，不进本 spec
- **不做服务端 BBKey 类型的强校验**：运行时 key 的类型（float/int/string/bool）在 ADMIN 作为字符串字段存储，不接入"编译期类型安全"。类型一致性是**软校验**——用于 FSM 条件编辑器的运算符下拉约束，以及导出后由游戏服务端 `NewKey[T]` 二次校验。ADMIN 不复现 Go 泛型类型系统
- **不做 runtime key 的 blackboard "namespace"/"scope"**：所有 runtime key 是**全局名字**（与服务端 keys.go 一致），不分"每个 NPC 独立 namespace"；即使是 `current_action` 这种每 NPC 独立的状态，其 **key 名**仍然全局唯一
- **不做 runtime key 的 default value / constraints**：运行时 key 不是 NPC 字段，没有"创建 NPC 时填什么"的语义，运行时由服务端写入
- **不做审计日志**：与其他配置模块一致（按 CLAUDE.md 过度设计红线，审计延后到毕设后）
- **不做运行时 key 的导入导出**（CSV 等）：按已有项目策略（`project_deferred_features.md`）延后到毕设后

## 一个 spec 还是多个？（合规性说明）

本 spec 含：runtime_bb_keys 表 + CRUD + seed + 引用同步 + 前端下拉接入 + 前端管理页。判定**紧耦合、非独立**，理由：

- 若只做 **表 + CRUD + seed**，没有前端下拉接入，策划在 FSM 编辑器里选不到新 key，价值零
- 若只做 **seed 脚本 + 前端下拉只读**（不 CRUD），那 seed 写错或服务端后续新增时运营无工具修正，必须回去改 seed 代码重跑，违背"新增配置类型不改代码"的扩展轴原则
- 若只做 **CRUD + seed**（不做引用同步），那策划改 FSM 条件时，被引用的 runtime key 可被自由删除 → 导致导出后服务端 `ValidateKeyName` 失败 → 死循环回滚

三者共同构成"让策划能在 FSM/BT 中稳定引用运行时 BB Key"的最小可行完整交付。单独任一件都会让系统处于更坏的中间态。

## 阶段产出物一览

- `requirements.md`（本文件，Phase 1 产出）
- `design.md`（Phase 2 产出，前置 `/backend-design-audit`——用户指定七层审视靶子在 Phase 2 做）
- `tasks.md`（Phase 3 产出）
