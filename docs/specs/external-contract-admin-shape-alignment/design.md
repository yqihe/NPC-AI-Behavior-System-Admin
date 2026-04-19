# external-contract-admin-shape-alignment — 设计方案

本阶段把 requirements.md 里的需求（R1–R8 + 4 条 Phase 1 决策）翻译成可执行的技术方案，并正面处理 Phase 1 留下的 4 个开放问题（OQ1–OQ4）。

---

## 1. 方案总览

本 spec 纯**数据 seed + 文档 + 轻量 schema 补齐**，不动业务代码。分四块：

| 块 | 产出 | 落点 |
|---|---|---|
| A | 字段 catalog seed（8 个字段 + 1 个孤儿字段 `hp`） | `backend/cmd/seed/field_template_npc_seed.go` |
| B | 模板 seed（4 个模板含 tpl_guard 占位） | 同上 |
| C | NPC 实例 seed（6 个 NPC，携带 snapshot §4 形态） | 同上 |
| D | 双边外部契约文档 | 新建 `docs/architecture/api-contract.md` |

调用时机：`backend/cmd/seed/main.go` 在现有 dictionary / fsm_state_dict / bt_node_type seed 完成后追加一步 `seedFieldsTemplatesNPCs(ctx, db)`。**幂等**：重复执行只跳过、不覆盖。

---

## 2. 开放问题的设计决策

### OQ1：当前 npcs / fields / templates 表是否已有数据

**结论**：假设**可能有运营手建数据**。seed 策略必须幂等且不覆盖运营手改。

- fields 表：若 `name` 已存在（UNIQUE KEY `uk_name`），seed 跳过（不改运营已改过的 label / properties / constraints）
- templates 表：同上，UNIQUE KEY 冲突跳过
- npcs 表：同上
- field_refs 表：模板 seed 产生的 template→field 引用走 `INSERT IGNORE`（`PRIMARY KEY (field_id, ref_type, ref_id)` 天然幂等）

**行动项**：seed 启动时先 `SELECT count(*)` 三张表并打印当前记录数，让运行者感知"seed 运行前的库状态"。不做自动分支逻辑——只打印、只用 INSERT IGNORE 语义。

### OQ2：seed 落点（cmd/seed vs migration）

**结论**：`cmd/seed` 脚本扩展（**不**走 migration）。

**对比**：

| 方案 | 可重入 | 可选择性执行 | 与现有惯例一致 | 业务数据 vs DDL |
|---|---|---|---|---|
| **cmd/seed 扩展** ✅ | 是 | 加 flag 控制（如 `-include-npcs`） | 对齐 seedFsmStateDicts / seedBtNodeTypes | 业务数据——匹配 |
| migration（014_seed_fields.sql） | 否（一次性） | 无（依 schema_migrations 跟踪）| migration 目前纯 DDL | DDL——不匹配 |

不选 migration 的关键理由：
- migration 是"版本演进"的 DDL 流，把 NPC 实例业务数据塞进去会让"DDL 版本"和"业务数据版本"耦合；将来 NPC 换形态时 DDL 永远不能再重放，也就没法回到干净环境
- 现有 `cmd/seed` 已是字典/引用表种子的规范位置，扩展它保持惯例

### OQ3：guard_basic.hp 的 DB 存储形态（本 spec 最大张力）

**问题**：snapshot §4 显示 `guard_basic.fields = {hp: 100}`，导出接口要原样返回以保 smoke test。但 `npcs.fields JSON` schema 要求每个 entry 有 `field_id`，hp 不在 fields 表就没 id。决策已锁定"hp 不进 catalog"且"DB 原样返回 {hp: 100}"——技术上必须二选一妥协。

**候选方案**：

| 方案 | 实现 | 与决策精神的张力 | 对服务端翻译层 | 选 / 不选 |
|---|---|---|---|---|
| **A：孤儿字段** | seed hp 进 fields 表，但 `enabled=0` + 不进任何模板 fields 数组 + label 带"历史遗留"标注 | 中等。技术上 hp 进了 catalog，但 UI 层默认隐藏、不可被模板引用——"不合法化"以"不可见"实现 | 透明（field_id 不进导出 payload） | ✅ **选** |
| B：允许 npcs.fields entry 的 field_id=0 | 改 model/service 容忍 field_id=0 | 低（hp 真的不进 catalog） | 透明 | ❌ 超 spec 范围（改业务代码） |
| C：导出接口特例注入 hp | handler 层 `if TemplateRef==tpl_guard then inject hp` | 高（违反"禁止静默降级"红线）| 透明 | ❌ 红线 |
| D：guard_basic.fields 存空数组 | DB 里 guard_basic 无 hp，snapshot §4 与 seed 输出偏差 | 低（hp 真的不进） | smoke test 回归破（hp 消失）| ❌ 与用户决策冲突 |

**选 A 的实施细节**：
- hp 字段：`name=hp`, `type=float`, `label=旧血量（历史遗留，请用 max_hp）`, `category=basic`, `enabled=0`, `properties.description=仅用于 guard_basic 兼容，后续 41008 约束解封后一次性清除`, `properties.expose_bb=false`, `properties.constraints={}`, `properties.default_value=100`
- 不进任何 template 的 fields 数组 → 运营建模板时选字段不会看到它
- 前端 FieldList 页面按 `enabled=1` 过滤（现有行为）→ 运营平时不被它打扰
- api-contract.md 里明确标注"hp 是孤儿字段，仅为兼容 snapshot，不建议引用"

**承认的妥协**：方案 A 把"hp 不进 catalog"从严格的字面承诺降为"不暴露给策划 / 不可被模板引用"。若用户认为这个妥协不可接受，回退到方案 D 并在 R4/R5 明示"guard_basic 的 fields 暂空、snapshot hp 形态留在 v1 观测文档里，seed 不复现"。

### OQ4：tpl_guard 占位的 FK 约束

**验证结果**：`013_create_npcs.sql` 的 `npcs.template_id` **没有 FOREIGN KEY** 约束，只有 `INDEX idx_template (template_id, deleted)`。应用层校验（`NpcService.Create`）要求传入的 `template_id` 能在 templates 表 `GetByID` 到，但 seed 走 store 层直写 DB，可以绕过这个校验。

**决策**：**仍 seed tpl_guard 占位**（按 Phase 1 用户立场"无 FK 也推荐"）。理由：
- 运营在 UI 查 guard_basic 详情时 `template_ref=tpl_guard`，若 templates 表无对应记录，UI 跨模块补全 `template_label` 会取空串，显示为"未知模板"
- 未来若加 FK 约束（DB 优化），占位已存在不需要补建
- 代价极低：1 行记录，fields=[]

---

## 3. 数据结构设计

### 3.1 字段 catalog（8 正常 + 1 孤儿）

统一 schema（对齐 `001_create_fields.sql`）：

```
{
  name, label, type, category, properties: {
    description, expose_bb, default_value, constraints
  }, enabled, version, deleted, created_at, updated_at
}
```

| name | label | type | category | expose_bb | default_value | constraints |
|---|---|---|---|---|---|---|
| max_hp | 最大生命值 | float | basic | true | 100 | `{min:1, max:10000}` |
| move_speed | 移动速度 | float | movement | true | 3.0 | `{min:0, max:20}` |
| perception_range | 感知范围 | float | perception | true | 20.0 | `{min:0, max:200}` |
| attack_power | 攻击力 | float | combat | true | 15.0 | `{min:0, max:9999}` |
| defense | 防御力 | float | combat | true | 5.0 | `{min:0, max:9999}` |
| aggression | 攻击性 | select | personality | true | `"neutral"` | `{options:[{value:aggressive,label:主动攻击},{value:neutral,label:中立},{value:passive,label:被动}], minSelect:1, maxSelect:1}` |
| is_boss | 是否 Boss | boolean | combat | true | false | `{}` |
| loot_table | 掉落表 | string | interaction | **false** | `""` | `{}` |
| **hp** (孤儿) | 旧血量（历史遗留，请用 max_hp） | float | basic | false | 100 | `{}` |

正常字段 `enabled=1`；**hp 字段 `enabled=0`**。

**label 与 category 取值**：基于 `dictionaries.field_category` 现有 6 类（`basic/combat/perception/movement/interaction/personality`），逐字段合理归类。label 全中文，符合 ADMIN red-lines §6.1（UI 中文标签）。

**aggression 的 constraints**：select 类型必须符合 `seed main.go` 里 `constraint_schema`（`options`/`minSelect`/`maxSelect`）。seed 写入的 options 用 `{value, label}` 对。

**为什么 max_hp 等 `expose_bb=true`**：与 snapshot 时代实际行为一致。memory 里的 `bb-key-runtime-registry` spec 认为"FSM 用静态字段当 BB key 是死条件"，但这是**独立 spec 的治理范围**，本 spec 不改 expose_bb 语义，按 snapshot 当时状态恢复。loot_table 特例按 R8。

### 3.2 模板 seed（4 个）

对齐 `003_create_templates.sql`。fields JSON 形态：`[{field_id, required}, ...]`。

| name | label | description | fields（name 顺序，required 默认 false） |
|---|---|---|---|
| warrior_base | 战士基础模板 | 战士类 NPC 的字段集合 | aggression, attack_power, defense, is_boss, loot_table, max_hp, move_speed, perception_range |
| ranger_base | 游侠基础模板 | 游侠类 NPC 字段集合（无 is_boss） | aggression, attack_power, defense, loot_table, max_hp, move_speed, perception_range |
| passive_npc | 被动 NPC 模板 | 非战斗 NPC 最小字段集 | aggression, max_hp, move_speed, perception_range |
| tpl_guard | 守卫历史模板 | 历史遗留占位模板，仅为兼容 guard_basic | `[]` |

全部 `enabled=1`（不给"配置窗口期"——因为是 seed 数据即最终态）。

**field_id 填值**：seed 执行时先拿各字段 name → id，再组装 template.fields JSON。若字段因 name 冲突已存在则复用其 id；若字段被手删（deleted=1）则 seed 失败并报错（明示数据不一致）。

**`required` 的默认值**：全部 `false`。snapshot §4 没有 required 信息，保守默认非必填。

### 3.3 field_refs 补建

每个模板 seed 成功后，对其 fields 数组里每个 field_id 插入 `field_refs`：

```sql
INSERT IGNORE INTO field_refs (field_id, ref_type, ref_id) VALUES (?, 'template', ?)
```

`ref_type='template'`（对齐 `util.RefTypeTemplate`），`ref_id=template_id`。tpl_guard 因为 fields=[]，不插任何 field_ref。

### 3.4 NPC seed（6 个）

对齐 `013_create_npcs.sql`。关键字段组装：

| name | label | template_name → template_id | fields 内容 | fsm_ref | bt_refs |
|---|---|---|---|---|---|
| wolf_common | 普通狼 | warrior_base | snapshot §4 数值 | fsm_combat_basic | snapshot §4 bt_refs |
| wolf_alpha | 头狼 | warrior_base | snapshot §4 数值 | fsm_combat_basic | 同上 |
| villager_guard | 村卫兵 | warrior_base | snapshot §4 数值 | fsm_combat_basic | 同上 |
| goblin_archer | 哥布林弓手 | ranger_base | snapshot §4 数值 | fsm_combat_basic | 同上 |
| villager_merchant | 村庄商人 | passive_npc | snapshot §4 数值 | fsm_passive | snapshot §4 bt_refs |
| **guard_basic** | 基础守卫 | **tpl_guard** | `[{field_id: <hp_id>, name: "hp", required: false, value: 100}]` | guard | `{"patrol": "guard/patrol"}` |

`enabled=1`（NPC 是成品）。`npc_bt_refs` 表按 `bt_refs` 展开对应行。

**Label 取值**：中文合理命名，由 seed 文件内定义，允许后续运营改（不覆盖）。

### 3.5 api-contract.md（新建）

目录和文件首次创建。最简骨架：

```
# 游戏服务端 ↔ ADMIN API 契约

本文档是双边外部契约的权威。服务端 admin_template.go 反向依赖此 schema。
**同步方式：人工同步**（ADMIN 为权威源，服务端侧对应 PR 需在 description 引用本文件 commit hash）。

## 1. GET /api/configs/npc_templates

**用途**：服务端启动或 `cmd/sync` 拉取 NPC 模板配置
**返回**：200 JSON，形态如下
**调用方**：服务端 admin_template.go（唯一反序列化点）

### Schema
...
### 字段说明
...
### 已知数据噪声
- guard_basic.fields.hp：历史遗留（见 project_guard_basic_hp_deferred.md），
  41008 解封后一次性清除。服务端 SetDynamic 写入 BB 但不被任何 BT 消费。
```

后续段落（`event_types` / `fsm_configs` / `bt_trees` / `regions`）按实际已有接口形态补全，但**本 spec 只做 `npc_templates` 那一段**——其他段落是 out-of-scope 的增量工作。

---

## 4. 方案对比（替代整体方案）

**替代方案**：把 seed 做在 `backend/migrations/014_seed_external_contract_data.sql`，纯 SQL。

**为什么不选**：
- migration 永久记录在 `schema_migrations`，重跑环境时 SQL 数据混在 DDL 里；想清空 NPC 重测就得回滚 migration
- SQL 不好表达"先查字段 name → id 映射、再组装 JSON"——要么 HARD-CODE field_id（跨环境不稳定），要么写 stored procedure（维护成本高）
- 无法优雅复用 Go 层已有的 `mustRawJSON` / `model.NPCFieldEntry` 结构体 → 写出的 JSON 字节级校对困难
- 运营未来变更 seed 默认值需要懂 SQL 的人改，cmd/seed Go 代码门槛更低

---

## 5. 红线检查（逐份对照）

### 5.1 `standards/red-lines/general.md`

| 红线 | 适用性 | 结论 |
|---|---|---|
| 禁止静默降级 | ⚠️ 需注意 | seed 遇冲突必须 `slog.Info("跳过 X: 已存在")`，不能 silent continue |
| 禁止过度设计 | ✅ 无违反 | 不引入新框架、不新增抽象层 |
| 禁止协作失序 | ✅ 无违反 | 配置变更走 DDL seed，不仅仅改本地 configs 文件 |
| 禁止测试质量低下 | ⚠️ 需注意 | Phase 3 测试任务必须清理外部状态；断言对齐实际 API 响应字段名 |

### 5.2 `standards/red-lines/go.md`

| 红线 | 适用性 | 结论 |
|---|---|---|
| 禁止资源泄漏 | ✅ | seed 用 context.WithTimeout（沿用 main.go 已有模式）|
| 禁止序列化陷阱（nil slice→null） | ⚠️ | tpl_guard.fields=[] 必须 `make([]TemplateFieldRef, 0)` 再 Marshal，避免 `null` |
| 禁止 `json.Unmarshal` 到 any | ✅ | 本 spec 无反序列化 |
| 禁止硬编码魔术字符串 | ⚠️ | `"template"` 必须用 `util.RefTypeTemplate`；`"basic"/"combat"` 等 category 值须引用 seed main.go 已有常量或新建 `util.FieldCategoryXxx` |
| 禁止字符串长度用 len() | ✅ | seed 不做长度校验 |
| 禁止分层倒置 | ✅ | cmd/seed 只 import store/mysql + model + util + config |

### 5.3 `standards/red-lines/mysql.md`

| 红线 | 适用性 | 结论 |
|---|---|---|
| 禁止事务一致性破坏 | ⚠️ | seed 每个实体独立事务或全局一个事务？**选无事务 + `INSERT IGNORE`**——幂等语义天然避免 TOCTOU；用事务反而在冲突时要回滚无意义 |
| 禁止 LIKE 不转义 | ✅ | seed 无 LIKE 查询 |

### 5.4 `standards/red-lines/redis.md` / `cache.md`

- **不涉及**：seed 直写 MySQL，不走 Redis 缓存。但**运行完 seed 后运营页面可能看到旧缓存**——需要 seed 完成后打印提示"请清 Redis 缓存或重启 admin-backend"。加一条验收项 R9（见 §12）

### 5.5 `standards/red-lines/frontend.md`

- **不涉及**：本 spec 纯后端 seed + 文档

### 5.6 `admin/red-lines.md`（ADMIN 专属）

| 红线 | 适用性 | 结论 |
|---|---|---|
| §1.1 禁改 MongoDB 文档结构 | ✅ | ADMIN 用 MySQL，与该项无关（红线文案是 V2 遗迹）|
| §1.6 禁写入不属于模板的 BB Key | ⚠️ | guard_basic.hp 在 tpl_guard（fields=[]）之外写入——**实际走的是 NPC.fields 快照路径而非模板路径**，该红线针对的是 FSM/BT 条件里的 BB key 选择，对 NPC 字段 seed 不适用。但需在 api-contract.md 解释清楚避免误读 |
| §2.1 禁删被引用配置 | ⚠️ | seed 本身只新建不删，无违反；field_refs 正确维护确保"删字段需先解引用"继续生效 |
| §2.5 禁止冗余计数器 | ✅ | 本 spec 不引入 ref_count 列 |
| §3.1 禁用 mongosh 直写 | ⚠️ | 现 seed 是 Go 代码直写 MySQL，不走 REST API。这是**现有 seed 惯例**（dictionary / fsm_state_dict / bt_node_type 都这么做），本 spec 对齐此惯例。memory `feedback_configs_must_hit_api.md` 针对的是"联调配置更新"场景，与 seed 不同——seed 是冷启动数据初始化，不是联调热更新 |
| §4 禁止硬编码 | ⚠️ | 错误码/消息/ref_type 等所有硬编码都走 errcode/util 常量；seed 内打印的 slog 消息用 i18n 格式（沿用现有 `"seed.加载配置失败"` 风格）|
| §4b 禁止跳过 constraints 自洽校验 | ⚠️ | seed 写入的 constraints 必须**先过 `ValidateConstraintsSelf`**（`service/validate.go`）。否则未来 service 层收紧校验时 seed 数据会就地爆炸。Phase 3 T2 必须包含此校验 |
| §10 禁止偏离跨模块代码模式 | ✅ | seed 不经 handler/service/store 的 CRUD，走直写不受此约束 |
| §11 禁止文件职责混放 | ✅ | 新文件 `cmd/seed/field_template_npc_seed.go` 是独立聚合 seed 文件，命名带业务语义，不塞入 util |
| §16 禁止 Commit 后清缓存 | ✅ | seed 无 Commit + 缓存场景（不经 service）|
| §18 禁止事务内绕过事务查询 | ✅ | seed 不开事务 |

**无红线违反** — 按 §5.3 的 INSERT IGNORE 无事务方案、§5.6 的 constraints 自洽校验前置即可通过。

---

## 6. 扩展性影响

ADMIN 有两个扩展轴（新增配置类型 / 新增表单字段）。本 spec **中性**：

- 不新建 handler/service/store/validator → 不影响扩展轴 1
- 不新建 SchemaForm 子组件 → 不影响扩展轴 2
- seed 代码在独立包 cmd/seed，未来加新配置类型的 seed 可以照搬本 spec 的模式（`seedXxx` 函数 + `INSERT IGNORE`）→ 反而**轻微正向**（建立了业务数据 seed 的模板）

---

## 7. 依赖方向

```
cmd/seed (本 spec 新增)
  ↓ imports
internal/model, internal/store/mysql, internal/util, internal/config
  ↓ 不 import service / handler
```

**单向向下** ✅ —— cmd/seed 作为工具入口，不反向依赖上层。现有 `cmd/seed/main.go` 已是此结构，本 spec 新增文件沿用。

唯一跨包新依赖：本 spec 可能需要引入 `internal/service` 的 `ValidateConstraintsSelf`（见 §5.6 §4b）。但该函数是纯函数、不依赖 service 上下文，技术上可以 pass。实现时评估：
- 优先选项：把 `ValidateConstraintsSelf` 相关逻辑提到一个不依赖 service 实例的位置（或直接在 cmd/seed 内本地复制校验逻辑——因为 seed 数据是静态已知，我们在代码评审时确保正确就行，不需要运行时校验）
- 保底选项：cmd/seed import service 只为调用这个纯函数——这会是本 spec 第一次打破"cmd 不 import service"的原则，需要审视

**决定**：Phase 3 T2 实现时采用**静态校验**——seed 数据是 Go 代码字面量，在 PR review 时人眼比对 constraints 合法性；运行时不再调 ValidateConstraintsSelf。这样不引入新依赖。风险：seed 代码后续修改时错过自洽校验。缓解：PR review checklist 里加一条"修改 seed constraints 时手动核对 ValidateConstraintsSelf 规则"。

**⚠️ 此决策的有效期**：当前 ADMIN 是"字段由运营动态创建 + seed 只有字典类型"的惯例。本 spec 首次 seed 字段 catalog 属一次性初始化，校验覆盖面可控。**若未来 ADMIN 扩展为"运营也能在 UI 上自定义字段 constraints"（目前由 `properties.constraints` 的 JSON 编辑能力支撑），必须启用运行时 `ValidateConstraintsSelf`，不能延用 PR 兜底**——因为运营输入不经过代码 PR。本 spec 的"静态校验"只对"seed 写入阶段"有效，不对"运营 UI 写入阶段"有效（后者已经在现有 service 层被强制校验，不受本 spec 影响）。

---

## 8. 陷阱检查

按涉及技术领域查 `standards/dev-rules/`：

- **`dev-rules/go.md`**：seed 里 `json.RawMessage` 初始化用 `&raw` 模式（现 main.go 已有 `mustRawJSON`），tpl_guard.fields=[] 必须 `make([]X, 0)` 再 Marshal，避免序列化成 null
- **`dev-rules/mysql.md`**：INSERT IGNORE 返回的 `rows_affected=0` 需区分"跳过"和"错误"——用 `err == nil && affected == 0` 判定跳过，打 info 日志
- **`dev-rules/cache.md`**：seed 完成后缓存不同步，需提示运行者清 Redis 或重启 admin-backend——加到 seed 运行完的打印
- **`dev-rules/frontend.md`**：不涉及（本 spec 不动前端）

---

## 9. 配置变更

无新 JSON 配置文件、无新环境变量、无 `config.yaml` 字段增减。

cmd/seed 可以加一个 flag 控制是否 seed NPC 实例（`-seed-npcs=true`，默认 true）——让后续只重播字典/模板时可以跳过 NPC。但这是 nice-to-have，Phase 3 T4 设计时决定是否纳入。

---

## 10. 测试策略

本 spec 是**数据 seed + 文档**，不涉及业务代码改动，测试分三层：

### 10.1 单元测试（可选 / 低价值）

seed 函数本身是 I/O + INSERT IGNORE，难以模拟且价值低。**不写单元测试**，通过 §10.2 的 integration 脚本兜底。

### 10.2 集成验证脚本（**必须**）

Phase 3 产出 `scripts/verify-seed.sh`（或内置在 /verify skill 里）：

```bash
# 1. 启动 docker compose 干净环境（admin-backend + MySQL + Redis）
docker compose up -d --build admin-backend mysql redis

# 2. 跑 seed
docker compose exec admin-backend /app/seed -config /app/config.yaml

# 3. 验证字段
curl -s http://localhost:9821/api/v1/fields?enabled=true | jq '.data.total' # 应 >= 8
curl -s http://localhost:9821/api/v1/fields?enabled=false | jq '.data.items[] | select(.name=="hp")' # 应非空

# 4. 验证模板
curl -s http://localhost:9821/api/v1/templates?enabled=true | jq '.data.total' # 应 >= 4

# 5. 导出 NPC 并与 snapshot §4 做逐字段对比
curl -s http://localhost:9821/api/configs/npc_templates > /tmp/export.json
diff <(jq -S '.items | sort_by(.name)' /tmp/export.json) \
     <(jq -S '.items | sort_by(.name)' docs/integration/snapshot-section-4.json)

# 6. 幂等验证：再跑一次 seed，断言所有 rows_affected=0
docker compose exec admin-backend /app/seed -config /app/config.yaml | grep "跳过 N 条"
```

snapshot-section-4.json 是从 `../NPC-AI-Behavior-System-Server/docs/integration/admin-snapshot-2026-04-18.md` §4 的 JSON 代码块人工提取到本仓 `docs/integration/snapshot-section-4.json`（作为本 spec 的**测试基线文件**，与 api-contract.md 同步维护）。

### 10.3 E2E（外部）

本 spec 不做 e2e——服务端侧用新的 admin 数据重新跑 NPC spawn 验证，属于服务端仓库独立 spec 的验收范围。ADMIN 侧只保 §10.2 的 API 层正确。

---

## 11. 回滚策略

本 spec 全部走 `INSERT IGNORE`，不改已有行，不删任何记录。**自然无需回滚**。

但若"seed 写错数据后发现"：
- 影响 NPC 实例：运营手动在 UI 删对应 NPC 即可
- 影响字段 / 模板：如果被引用，走 ADMIN 自身的"停用 → 解引用 → 删除"流程；若无引用，直接 UI 删
- 影响 field_refs：seed 重跑时幂等

---

## 12. 验收标准补丁（在 requirements.md R1–R8 基础上新增）

- **R9**：seed 完成后在 stdout 打印提示 `"⚠️ 若 admin-backend 已启动，请重启或清 Redis 缓存以同步新数据"`，确保运行者感知
- **R10**：`docs/architecture/` 目录 + `api-contract.md` 文件新建，至少包含 "GET /api/configs/npc_templates" 段落（其他配置导出段落为可选）
- **R11**：`docs/integration/snapshot-section-4.json` 存在，内容与 snapshot §4 的 JSON 字节一致（供 §10.2 diff 用）
- **R12**：seed 代码所有字面量（field name / template name / npc name / category / ref_type）走 util 常量或 seed 文件顶部的 const 声明，无裸字符串散落
- **R13**（Phase 2 review ① 挂起项）：Phase 3 必须包含**方案 A 三条语义验证任务**，在 seed 运行后执行：
  - **R13.1**：`GET /api/configs/npc_templates` 响应中 `items[].config.fields` 对 `guard_basic` 返回 `{hp: 100}`——证明 enabled=0 字段**不影响导出**
  - **R13.2**：`GET /api/v1/fields?enabled=true` 响应 items 中**不包含** `hp`；`GET /api/v1/fields?enabled=false` 响应 items 中**包含** `hp`——证明 UI 字段选择器按 enabled 过滤正确
  - **R13.3**：UI 建 NPC 选 tpl_guard 模板时，字段表单**不展示 hp 字段栏**（验证当前 `model.CreateNPCRequest.FieldValues` 按模板 fields 顺序的语义符合预期）——已知限制：**guard_basic 唯一重建路径是重跑 seed，不走 UI 新建 NPC 路径**（已用户 Phase 2 review 接受）

---

## 13. 双边契约同步方案（Phase 2 结论）

**方式**：**人工同步**（ADMIN 为权威源）。

流程：
1. ADMIN 改 `docs/architecture/api-contract.md`，commit + push
2. 在 commit message 体中显式列出"影响服务端仓库侧"的契约点
3. 对方发 PR 时在 description 引用 ADMIN 对应 commit hash 作为契约版本号
4. 若 ADMIN 改契约未通知服务端（协作失序），`general.md` "禁止协作失序"红线兜底

**不选 git submodule**：跨仓依赖引入运维成本，毕设体量不匹配
**不选 CI mirror**：需额外基础设施，毕设体量不匹配
**不选完全拷贝**：服务端侧保留副本会与 ADMIN 漂移；若确需本地副本，用 `git subtree pull` 手工同步

---

## 14. Phase 2 review 决策日志（2026-04-19 锁定）

| 决策点 | 结论 | 依据 |
|---|---|---|
| ① OQ3 方案 A（hp 孤儿字段） | **采纳**，附 Phase 3 三条验证（R13.1/.2/.3）| 方案 A 与 D 核心差异在导出层；方案 A 下 UI 新建 NPC 看不到 hp 是已知限制，guard_basic 唯一重建路径=重跑 seed，用户确认可接受 |
| ② aggression 中文 label | `aggressive=主动攻击 / neutral=中立 / passive=被动` | 用户命名，避免情绪化翻译（"侵略性/友善"），与 NPC 行为模式语义对齐 |
| ③ 静态 PR review 替代运行时校验 | **采纳**，附"有效期"条款（§7） | 仅对 seed-time 字面量有效；若未来运营 UI 能自定义 constraints，必须启用运行时 ValidateConstraintsSelf |
| ④ `docs/integration/snapshot-section-4.json` 落 ADMIN 仓 | 采纳 | 测试基线归属消费方（seed verify 在 ADMIN 跑） |

---

**Phase 2 结束并通过审批。进入 Phase 3（tasks.md）**。
