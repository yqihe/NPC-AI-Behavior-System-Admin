# seed-fsm-bt-coverage — 设计方案（Phase 2）

## 0. Phase 2 定位

本 design.md **不是** Phase 3 tasks 的前置设计（Phase 3 不存在 —— seed 只是数据注入，没有值得单独设计的代码架构）。它是**冻结窗口关闭后的决议落盘**：Phase 1 `requirements.md` 遗留的 OQ1（FSM/BT fixture 数据源）经两轮 Admin↔Server 对话收敛后，把"锁下的决议 + 翻译路径 + 交付边界"凝固为可审计文档。

- Phase 1 产出：[requirements.md](requirements.md)（commit `869dc64` + meta fix `7931300`）
- Phase 2(a)(b) 代码落地：commit `b9b0be5`（4 态 combat FSM + BT 升级 + 2 string 字段 + 3 demo NPC opt-in）
- Phase 2(e) snapshot + verify：commit `401cf75`
- Phase 2(c) 契约附录：见 [docs/architecture/api-contract.md](../../architecture/api-contract.md) v1.1.3
- Phase 2(d) 本文件

---

## 1. 现象与起点：冻结关闭后的 4 项 Server findings

2026-04-20 晚，双边契约冻结（`project_contract_freeze_2026-04-19`）关闭、R15 live smoke 起步时，Server CC 对 ADMIN HEAD 冷启一轮 smoke，触发 4 条 findings：

| # | 现象 | 根因 | 归属 |
|---|---|---|---|
| 1 | `fsm_combat_basic.transitions` 空 → 6 NPC 起在 Idle 永不进战斗循环 | Phase 1 fixture 占位（requirements.md OQ1 未决）| Admin |
| 2 | 5 个 opt-in bool 全部 demo NPC 都 false → 记忆/情绪/社交组件不触发 | seed 没分配，api-contract v1.1 opt-in 未进入 e2e 路径 | Admin |
| 3 | `guard_basic.fields.hp` 而非 `max_hp`（历史数据噪声）| 41008 硬约束未解封前无法原地改 | 双边已 ack（零动作）|
| 4 | 战斗数值字段（`attack_power`/`defense`/`is_boss`/`loot_table`/`max_hp`）未进契约附录 | api-contract v1.1 只形式化了 top-level shape + opt-in bool，`fields` 内字段无独立说明表 | Admin |

Finding #3 已在 api-contract.md v1.1 §已知数据噪声 ack，留 41008 解封时一次性改；**本 spec 不处理**。Findings #1/#2/#4 合并为 Phase 2 一次性 atomic 交付。

---

## 2. 决议概览

### 2.1 combat FSM 锁 4 态（非 6 态）

**states**：`Idle` / `Patrol` / `Chase` / `Attack`

**砍掉 Flee / Dead 的原因**：Server runtime `blackboard/keys.go` **无血量系统** —— 没有 `KeyHP`、没有 damage 事件、没有 die 事件。Flee/Dead 的 transition 条件（`current_hp < threshold` / `current_hp == 0`）无法在现阶段评估。若硬保留空 transition 的 2 态，策划层会对"看到状态却跳不进去"产生歧义，契约污染。

Flee/Dead 挂后续独立 HP 系统 spec（非本轮）；待 HP 系统落地后由该 spec 扩 6 态。

### 2.2 翻译蓝本：police FSM/BT → combat

Server 侧有成熟的 police FSM/BT（`Idle`/`Alarmed`/`Engage`），结构与 combat 4 态同构（`Idle`/`Patrol`/`Chase`/`Attack`）。

| Server police 态 | Admin combat 态 | 迁移 |
|---|---|---|
| Idle | Idle | 同义直接 byte-copy |
| Idle | Patrol | police 无 patrol 态；从 `bt/combat/idle` 衍生 `bt/combat/patrol`（写 `current_action=patrol_move` + `patrol_waypoints` stub）|
| Alarmed | Chase | 同义；transition 条件 `threat_level>=30 && threat_expire_at>current_time`（双 key 比较，走 condition DSL 的 `ref_key`）|
| Engage | Attack | 同义；transition 条件 `threat_level>=60`（进入）/`threat_level<40`（退回 Chase）|

**condition DSL 双 key 比较**走 [`backend/internal/model/fsm_config.go`](../../../backend/internal/model/fsm_config.go) 的 `ref_key` 字段（`{"key":"threat_expire_at","op":">","value":"","ref_key":"current_time"}`）。Server 在 `condition.go:79-80` 已确认支持 runtime 解析。

**guard BT**：Server 从 `police/alarmed` 翻译一版 `bt/guard/patrol`（写 `current_action=guard_patrol` + `patrol_post` stub）。

### 2.3 3 demo NPC 的 opt-in 分配

| NPC | 启用组件 | 额外字段 | 展示场景 |
|---|---|---|---|
| `wolf_alpha`（boss）| `enable_memory=true` + `enable_emotion=true` | — | Boss 记仇：memory.threat_value 累积 → emotion.fear 驱动 decision 权重 |
| `villager_merchant`（商人）| `enable_social=true` | `group_id="merchant_guild"` + `social_role="trader"` | Group 可见性 + 自由 role（验 Server PR #32 白名单放宽）|
| `villager_guard`（村民）| `enable_personality=true` | — | 复用已有 `aggression="neutral"` 驱动 decision_weights |

**`enable_needs` 不覆盖**：模拟游戏循环（饥饿 / 疲劳周期），毕设场景用不到；留 default_value=false。

**`wolf_common` 保持 3 bool=false**：作为对照组，证明"absent ≡ false" 契约不因 demo 扩展被破坏。

### 2.4 group_id / social_role 的语义与归属

**是 Admin 字段（category=component），不是纯 BB runtime key**。Server 的 [`admin_template.go:296-300`](../../../../NPC-AI-Behavior-System-Server/internal/runtime/npc/admin_template.go) 从 `config.fields.group_id` / `config.fields.social_role` 读，不从 blackboard 读，不参与 BB key 注册表。

- **字段类型**：`string`
- **约束**：无（无 enum、无正则、无长度上限 —— 运营手填自由值）
- **默认值**：空串（absent 时服务端视作"未分配 group"，`GroupManager` 对该 NPC 不可见）
- **Role 自由化**：Server PR #32 放宽 `SocialFactory` 的 `validRoles` 白名单（原 `{"leader","follower"}`），role 接任意 string。`group_manager` 内 `role == "leader"` / `role != "follower"` 分支保留，未知 role 静默 skip（不被 leader 驱动也不做 follower 跟随）

---

## 3. 命名约定：`bt/guard/patrol` 非 `guard/patrol`

原 Phase 1 seed 命名 `guard/patrol`（无 domain 前缀），与其他 BT 的 `bt/<domain>/<leaf>` 约定（`bt/combat/*`、`bt/passive/*`）不一致。

Phase 2(a) 顺手改：`guard/patrol` → `bt/guard/patrol`。影响点：
- seed 数据：[`backend/cmd/seed/fsm_bt_seed.go:88`](../../../backend/cmd/seed/fsm_bt_seed.go#L88)
- 模板引用：[`backend/cmd/seed/field_template_npc_seed.go`](../../../backend/cmd/seed/field_template_npc_seed.go) 的 `btTreeGuardPatrol` 常量 + `tpl_guard` 模板 + `guard_basic` NPC bt_refs
- 冷启断言：[`scripts/verify-seed.sh`](../../../scripts/verify-seed.sh) BT_COUNT 检查
- 需求文档：本目录 requirements.md §R2 / §改动范围

**决定原因**：Phase 2 是唯一能便宜清理命名债的窗口（spec 还未 push 到 Server 消费层）。延后改会涉及 Server 配置同步，成本不划算。

---

## 4. 不做的事（与 Phase 1 要求一致）

- **不改** `api-contract.md` v1.1 核心 shape（v1.1.3 为 doc-only append，不 bump minor）
- **不改** `snapshot-section-4.json` 字段基线 —— 新增字段走 export-ref-validation 已有的 oh-fixture 机制
- **不加** Flee/Dead 态（等 HP 系统 spec）
- **不加** Faction / FollowTarget 字段翻译（Server 本轮提议，Admin 明确拒绝：超出 Phase 2 必要范围，毕设场景用不上）
- **不做** 跨字段联动校验（`enable_emotion=true && enable_memory=false` 的非法组合由 Server 启动 fatal 校验兜底，不新增 Admin 侧 UI 校验）

---

## 5. 落地记录（何处 / 何时 / 何种校验）

| 节点 | commit | 校验方式 |
|---|---|---|
| Phase 2(a)：4 态 FSM + BT 升级 + 2 string 字段 + 3 demo opt-in | `b9b0be5`（已 push） | 冷启 `curl /api/configs/npc_templates` 200 + jq 校验字段值 |
| Phase 2(a) 顺手 bug：[`loadFieldIDMap`](../../../backend/cmd/seed/field_template_npc_seed.go) 硬编码 9 字段名（不修则 templates 阶段报 `unknown field "enable_memory"`）| 同 `b9b0be5` | 冷启 seed 零错 |
| Phase 2(b)：Server `SocialFactory` 放宽 Role 白名单 | Server PR #32（合并前即可引 URL，hash 不稳）| Server 单元测试 + R15 smoke |
| Phase 2(e)：snapshot regenerate + verify-seed R7 计数 14→16 | `401cf75`（已 push） | `bash scripts/verify-seed.sh` 全绿（R1–R13.2 + R7 + R1/R2 + batch2）|
| Phase 2(c)：契约附录 v1.1.3 | TBD（下轮）| 手审 |
| Phase 2(d)：本文件 | TBD（下轮） | 手审 |

**R15 live smoke 解锁**：Admin `origin/main` HEAD `401cf75`，Server 拉此 tag 跑 `docker compose up --build`。

---

## 6. 扩展性影响

**扩展轴 1（新增配置类型）**：⚪ 无影响。未增新类型。

**扩展轴 2（新增表单字段）**：✅ 走既有 seed 流程。`group_id` / `social_role` 走与其他 string 字段完全相同的注册链路（`seedFields` → `loadFieldIDMap` → 模板 `FieldNames` → NPC `FieldValues`）；**唯一特殊点**是 `category=component`，由 UI 层决定是否与 5 opt-in bool 同组渲染。

**本 spec 未引入新的扩展轴设计负担**。

---

## 7. 跨仓依赖与版本锚

- Admin 侧 HEAD：`origin/main` `401cf75`（5 commits 一次性 push）
- Server 侧 Phase 2(b)：PR #32 open 后 URL 永久有效（合不合并都作为可引用锚），合并后 hash 按 Server 侧 merge 策略变化 —— 契约附录**引 PR URL 不引 hash**
- 契约版本：`api-contract.md` v1.1.3（doc-only append，无 shape 变动）

---

## 8. 后续 Phase 3 / tasks.md

**不存在**。Phase 2 结束即本 spec 关闭。

理由：Phase 2 交付 6 NPC 冷启 + R15 smoke 通过 + 契约附录落盘 + 设计凝固 == 满足 requirements.md §验收标准 R1–R10 全部。后续"行为语义完整（Flee/Dead / 真实血量驱动）"属独立 HP 系统 spec 范畴。
