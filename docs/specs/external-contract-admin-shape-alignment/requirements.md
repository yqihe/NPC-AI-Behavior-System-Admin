# external-contract-admin-shape-alignment — 需求分析

## 动机

**触发事件**：服务端侧（2026-04-19）审视"路径 A：外部契约统一到 ADMIN 形状"的可行性时发现，原以为是"冻结未用回退代码"的**组件化路径 `{name, preset, components}` 实际是主力**：

| 路径 | 谁在用 |
|---|---|
| 组件化 `{name, preset, components}` | **主路径**：zones 批量 spawn、gateway WS handler、所有 e2e 测试、`configs/*.json` |
| ADMIN `{template_ref, fields, behavior}` | **回退路径**：仅在无 zones 时激活（`main.go:116`），smoke test 走这条 |

简单删组件化会干掉 zones + handler + e2e + 13 个 component factory；**路径 A 的可行版本是"外部契约统一到 ADMIN 形状，内部组件化保留"**，由服务端 `admin_template.go` 升级为唯一入口，把 `{fields, behavior}` 翻译成内部组件实例。

**ADMIN 侧本 spec 的职责**：锚定外部契约的**数据侧**——把联调 snapshot §4 里已冻结的 6 NPC / 4 模板 / 8 字段 seed 到 ADMIN 数据库，并把导出接口的返回形态写进 `docs/architecture/api-contract.md`，作为服务端翻译层实现时反向依赖的 schema 权威。

**不做的后果**：
- 服务端翻译层改造（`admin_template.go` 升级）**无法启动**——没有固化 schema 可锚定
- 毕设答辩演示时 ADMIN 是"空仓库 + 手工造数据"状态，无法展现"ADMIN seed 支撑 NPC 端到端跑通"完整链路
- snapshot §4 数据停留在一次性联调现场，未进 SSOT（single source of truth），后续变更无 diff 基线

## 优先级

🟡 **中高**。

- 是服务端"路径 A 可行版"的**前置依赖**：翻译层要锚定 ADMIN 导出的确切 schema，schema 没固化翻译层无从写起
- 不是 🔴 阻断：毕设 e2e 当前在 commit `747b0c3`（BT 格式统一）+ `32a4539`（event_type name 注入）后已能跑通 6/6 NPC spawn，运营手建数据能维持现状
- 但是服务端 spec 的**硬依赖**，越早做服务端越早能启动

## 预期效果

**场景 1：新 clone ADMIN 的开发者跑 `cmd/seed` 得到可用数据库**
开发者执行 `go run ./cmd/seed -config config.yaml`。除当前已有的 dictionaries / fsm_state_dicts / bt_node_types 外，新增打印：`字段写入完成：8 条 / 模板写入完成：4 条 / NPC 写入完成：6 条`。随后 `GET /api/configs/npc_templates` 返回与 snapshot §4 逐字段一致的 6 NPC JSON。

**场景 2：服务端开发按 ADMIN 契约文档写翻译层**
服务端工程师打开 `docs/architecture/api-contract.md` "npc_templates 导出"段落，看到明确的返回 schema：`{items: [{name, config: {template_ref, fields: {k:v}, behavior: {fsm_ref, bt_refs}}}]}`，字段类型/可选性标注清晰，据此在 `admin_template.go` 写反序列化和组件翻译，无需再问 ADMIN 侧"你到底返什么"。

**场景 3：运营通过 UI 看到 seed 的 8 个字段/4 个模板**
运营登录 ADMIN，进入字段管理页面，看到 seed 的 8 个字段（max_hp / move_speed / aggression / …），带正确的 type 和 constraints。进入模板管理页面，看到 warrior_base / ranger_base / passive_npc / tpl_guard 四个模板，前三个字段集合正确，tpl_guard 显式为空并标记"历史遗留占位模板"。

**场景 4：运营尝试新建同名字段 `max_hp`**
运营点"新建字段"填 `name=max_hp`——触发 ADMIN 已有的 name 唯一约束，返回 409 + `ErrFieldNameConflict`，UI 提示"max_hp 已存在"。运营不能意外覆盖 seed 字段。

**场景 5：运营删除/编辑 seed 字段**
运营尝试编辑 seed 字段 `max_hp` 的 constraints——当该字段被 warrior_base / ranger_base 等模板引用时，ADMIN 已有硬约束 41008 生效，禁止编辑（这是 memory `project_guard_basic_hp_deferred.md` 里描述的约束）。该行为**本 spec 不改变**，只对齐。

## 依赖分析

**上游依赖（已完成）**：
- 联调 snapshot `../NPC-AI-Behavior-System-Server/docs/integration/admin-snapshot-2026-04-18.md` §4：本 spec 的权威数据源
- 现有 seed 骨架 `backend/cmd/seed/main.go`：已有 dictionary / fsm_state_dict / bt_node_type seed 逻辑可复用模式
- migration `001_create_fields.sql` / `003_create_templates.sql` / `013_create_npcs.sql`：表结构已就绪
- 已收口的阻断修复 commit `747b0c3`（BT 格式）+ `32a4539`（event_type name）：本 spec seed 的 `behavior.bt_refs` 和 `fsm_ref` 引用能正确被导出

**下游被依赖（本 spec 解锁）**：
- 服务端 "路径 A 可行版" spec：`admin_template.go` 升级为唯一入口 + `TemplateConfig` 旧 schema 删除 + `configs/*.json` 重写 + zones/handler/e2e 切换，全部锚定本 spec 固化的 api-contract.md

**不依赖**：
- **不依赖** `bb-key-runtime-registry` spec：该 spec 治理的是 FSM 条件里静态字段当 BB key 的语义正确性，与外部契约 schema 无关
- **不依赖** `export-ref-validation` spec：该 spec 做导出时的引用完整性校验，是 seed 完成之后才有意义的增量工作
- **不依赖**服务端代码改动：本 spec 纯 ADMIN 侧数据落地，服务端侧的翻译层改造是下游独立工作

## 改动范围

**新增文件（预估 3 个）**：
- `backend/cmd/seed/field_template_npc_seed.go`（或分三个文件，设计阶段定）：字段/模板/NPC seed 逻辑
- `docs/specs/external-contract-admin-shape-alignment/design.md`（Phase 2 产出）
- `docs/specs/external-contract-admin-shape-alignment/tasks.md`（Phase 3 产出）

**修改文件（预估 2 个）**：
- `backend/cmd/seed/main.go`：在现有 dictionary / fsm_state / bt_node_type seed 之后追加调用
- `docs/architecture/api-contract.md`：新增/更新 "npc_templates 导出"段落，固化 schema

**不新建表**：所有数据落现有表。

## 扩展轴检查

ADMIN 有两个预设扩展方向：（1）新增配置类型；（2）新增表单字段组件。

**本需求与两个扩展轴均不相关**。理由：本 spec 是**一次性数据对齐**，不引入新的配置类型（字段/模板/NPC 均已存在），不引入新的表单字段组件，只在 seed 层写数据、在文档层固化契约。

**中性判断**：既不推进扩展性，也不伤害扩展性——不改 handler / service / store / validator / SchemaForm 任何已有模块代码，新增的只有 seed 代码和文档。符合扩展轴检查的"可接受中性改动"。

## 验收标准

- **R1**：执行 `go run ./cmd/seed -config <config>`，成功后控制台输出包含 "字段写入完成：8 条"、"模板写入完成：4 条"、"NPC 写入完成：6 条"（或等价信息），幂等——重跑一次新增数均为 0、跳过数为 8/4/6
- **R2**：`SELECT count(*) FROM fields` ≥ 8，且 8 个字段的 (name, type, constraints) 与 spec 表 1 的 8 行逐行匹配（JSON 序列化后字节级一致）
- **R3**：`SELECT count(*) FROM templates` ≥ 4，且 warrior_base / ranger_base / passive_npc / tpl_guard 四个模板的 field_ids 集合与 spec 表 2 的每行 fields 集合一一对应（顺序无要求）
- **R4**：`SELECT count(*) FROM npcs` ≥ 6，且 `GET /api/configs/npc_templates` 返回的 JSON items 数组与 snapshot §4 的 JSON items 数组**逐 NPC 逐字段相等**（除 `guard_basic.fields.hp` 的处理见 R5）
- **R5**：`guard_basic` 导出的 `fields` 对象包含且仅包含 `{hp: 100}`（或设计阶段决定的等价过渡方案），且在 `docs/architecture/api-contract.md` 的"已知数据噪声"小节明确标注此例外和延期原因（引用 memory `project_guard_basic_hp_deferred.md`）
- **R6**：`docs/architecture/api-contract.md` 的 "`GET /api/configs/npc_templates`" 段落存在且包含：返回 JSON schema、字段类型表、"双边外部契约，服务端 `admin_template.go` 反向依赖此 schema"的显式声明
- **R7**：Seed 执行失败场景——当字段 `max_hp` 已存在（运营先手建过）时，seed 不报错终止，而是跳过该条并继续，最终打印"跳过 N 条（已存在）"
- **R8**：`loot_table` 字段 seed 时 `expose_bb=false`（按 snapshot 它是外部 loot 表 ref，服务端 `SetDynamic` 写 BB 但不被任何 BT 消费，无需暴露给策划选 BB key）

## 不做什么（Out of Scope）

- ❌ **FSM 静态字段当 BB key 的治理**：独立 spec `bb-key-runtime-registry` 已规划，保持独立
- ❌ **`guard_basic.hp → max_hp` 修复**：按 memory `project_guard_basic_hp_deferred.md`，撞 ADMIN 41008 硬约束，延期至毕设后；本 spec 只"保留现状、文档标注"而不"修复"
- ❌ **服务端翻译层任何改动**：`admin_template.go` 升级、`configs/*.json` 重写、zones/handler/e2e 切换，全部在服务端仓库的独立 spec，不在本 spec
- ❌ **新建 handler/service/store 代码**：本 spec 只动 seed 和文档，不碰业务代码
- ❌ **UI 层变更**：运营是否看到"seed 标记"、能否"重置 seed"等交互增强，本 spec 不做
- ❌ **字段 constraints 的精细化调优**：本 spec 取 spec 正文表 1 的"基于 6 NPC 观测值 + 合理余量"默认值，上线后若运营发现不合理可改，但不在本 spec 范围
- ❌ **BT 节点格式 / event_types name 缺失**：已由 commit `747b0c3` 和 `32a4539` 收口，不重做

## 开放问题（设计阶段必须解决）

以下问题影响设计决策，Phase 2 `design.md` 必须给出结论后才能进入 Phase 3：

1. **当前 npcs / fields / templates 表是否已有数据**：需先查 DB。若运营已手建过 6 NPC 之一，seed 冲突策略是 `INSERT IGNORE`（跳过）还是 `ON DUPLICATE KEY UPDATE`（覆盖）。本 spec 倾向前者（保守，幂等），但需确认没有"运营手建值优于 snapshot 值"的场景
2. **seed 落点**：是走 `cmd/seed` 脚本（可重入、需手动触发）还是走 migration（一次性、启动即执行）。本 spec 倾向前者（与现有 dictionary seed 同构、可显式控制是否播 NPC 数据），但 trade-off 需在 design.md 展开
3. **`guard_basic.fields.hp` 过渡方案**：R5 占位为"保留现状 + 文档标注"。但导出接口实际返回需决定——是"按 DB 原样返回 `{hp: 100}`"，还是"过滤掉未被模板声明的字段"。两种方案对服务端翻译层影响不同：前者服务端要容忍"模板外字段"，后者需要 tpl_guard 声明 hp 字段（但 hp 不进 catalog 本身就冲突）。design.md 给结论
4. **`tpl_guard` 占位的 FK 约束实际是否存在**：需读 `003_create_templates.sql` 和 `013_create_npcs.sql`，确认 `npcs.template_id` 是否真对 `templates.id` 有外键。若无 FK，tpl_guard 可以不 seed；若有 FK，必须 seed 占位

## 决策日志（Phase 1 review 2026-04-19 锁定）

- **R7 幂等策略**：保守占位"跳过不报错"进 design.md，最终方案 design 阶段定。若 design 选"覆盖"语义，需明确**不覆盖运营在 UI 手改的 constraints**（seed 只管初值、不夺运营控制权）
- **R8 `loot_table.expose_bb=false`**：确认。loot_table 是死亡掉落查询 ref、非 AI 决策输入，放进 BB key 下拉是噪声；运营若真需要一键改 true 即可
- **开放问题 4 `tpl_guard` 占位**：无论 migration 里 FK 是否存在都 seed。有 FK 则强制，无 FK 则"推荐而非必须"——理由是运营在 ADMIN UI 看 guard_basic 详情时 template_ref 应指向真实存在的模板（发现性/一致性）
- **开放问题 3 `guard_basic.hp` 导出形态**：选 (a) DB 原样返回 `{hp: 100}`。理由：保 snapshot §4 观测形态不破、保服务端 `SetDynamic` 兼容、不把"严格按模板过滤"这种大架构决策塞进本 spec

---

**Phase 1 结束并通过审批。进入 Phase 2（design.md）**。
