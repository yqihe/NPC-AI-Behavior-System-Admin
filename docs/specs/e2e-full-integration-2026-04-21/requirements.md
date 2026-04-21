# e2e-full-integration-2026-04-21 — 需求

**目标**：双边清空 → Admin 从零配置 → Server 冷启动拉取 → 对账预期 → 覆盖 happy path + disable fan-out + 2 条 fail-fast 故障注入，生成联调基线报告。

**双边 owner**：Admin CC（本仓）+ Server CC（`NPC-AI-Behavior-System-Server-v1`）。产出 joint-report.md 双边各存一份。

**验收深度**：L2（启动 + 加载级；不深入 tick 行为验证）。

## R1 清理契约

| 侧 | 操作 |
|---|---|
| Admin | truncate 所有业务表 + Redis FLUSHDB + 跑 cmd/seed（含 SQL 补删 npcs + regions + npc_bt_refs 三张表，其他保留） |
| Server | `docker compose down && up --build -d`；configs/ 目录**不动**（HTTPSource 模式不读；是离线兜底）；本地 3 fixture 文件保留 |

**关键**：Admin 侧 cmd/seed 默认会播 6 NPCs + 1 region（`village_outskirts`），这些在 e2e 数据灌入前必须被清掉，否则 `/api/configs/npc_templates` 和 `/api/configs/regions` 响应条数会混入。具体：`reset.sh` 在 cmd/seed 后追加 `TRUNCATE npcs, npc_bt_refs, regions`。

## R2 数据矩阵

### 基础层（cmd/seed 产出，不动）

| 层 | 条数 | 来源 |
|---|---|---|
| fields | 16 | cmd/seed（含 5 opt-in + group_id/social_role + hp 孤儿 + 8 核心字段）|
| event_types | 5 | `earthquake` / `explosion` / `fire` / `gunshot` / `shout` |
| FSM | 3 | `fsm_combat_basic` / `fsm_passive` / `guard` |
| BT | 6 | `bt/combat/{idle,patrol,chase,attack}` + `bt/passive/wander` + `bt/guard/patrol` |
| runtime_bb_keys | 31 | 内置覆盖 Server `blackboard/keys.go` |
| templates | 4 | `warrior_base` / `ranger_base` / `passive_npc` / `tpl_guard`（cmd/seed 的基础模板）|

### e2e 层（API 灌入）

| 资源 | 条数 | 说明 |
|---|---|---|
| **模板** | +1 | `e2e_template_full`：POST /templates/create，含 5 opt-in bool + group_id + social_role + 5 战斗字段（max_hp/attack_power/defense/is_boss/loot_table），共 12 字段 |
| **NPC 实例** | 5 | 全部用 `e2e_template_full` + `fsm_combat_basic`，bt_refs 覆盖 Idle/Patrol/Attack 三态（Chase 留空，验证 bt_refs 部分映射合法）|
| **region** | 2 | `e2e_village`（引 `e2e_bare` × 2）+ `e2e_empty`（空 spawn_table）|

### 5 个 NPC 的差异设计

| NPC name | 用途 | enable_memory | enable_emotion | enable_needs | enable_personality | enable_social | group_id | social_role | enabled |
|---|---|---|---|---|---|---|---|---|---|
| `e2e_bare` | 覆盖 5 opt-in 全 false（v1.1 absent≡false 语义）| false | false | false | false | false | "" | "" | true |
| `e2e_social` | 覆盖 social 单开（独立合法）| false | false | false | false | **true** | `"e2e_group"` | `"follower"` | true |
| `e2e_memo_emo` | 覆盖 emotion+memory 合法级联 | **true** | **true** | false | false | false | "" | "" | true |
| `e2e_full` | 覆盖 5 opt-in 全开 | **true** | **true** | **true** | **true** | **true** | `"e2e_group"` | `"leader"` | true |
| `e2e_disabled` | 覆盖 disable fan-out（两端点都不出现）| false | false | false | false | false | "" | "" | **false** |

**战斗字段统一值**（所有 NPC 一致，降低对账噪音）：
- max_hp=100 / attack_power=15 / defense=8 / is_boss=false / loot_table="e2e_loot"
- move_speed=5 / perception_range=20 / aggression="neutral"

## R3 预期导出响应

### 第一轮（happy path + disable fan-out）

| 端点 | items.count | 关键 items.name |
|---|---|---|
| `/api/configs/event_types` | 5 | earthquake, explosion, fire, gunshot, shout |
| `/api/configs/fsm_configs` | 3 | fsm_combat_basic, fsm_passive, guard |
| `/api/configs/bt_trees` | 6 | bt/combat/{idle,patrol,chase,attack}, bt/passive/wander, bt/guard/patrol |
| `/api/configs/npc_templates` | **4** | e2e_bare, e2e_social, e2e_memo_emo, e2e_full（**e2e_disabled 不出现** → disable fan-out PASS）|
| `/api/configs/regions` | 2 | e2e_village, e2e_empty |

### Server 侧预期（来自 Server CC 模式表）

| 锚点 | 预期值 |
|---|---|
| `config.source type=http base_url=...` | 必须出现 1 行 |
| `config.http.loaded endpoint=/api/configs/event_types count=` | `count=5` |
| `config.http.loaded endpoint=/api/configs/fsm_configs count=` | `count=3` |
| `config.http.loaded endpoint=/api/configs/bt_trees count=` | `count=6` |
| `config.http.loaded endpoint=/api/configs/npc_templates count=` | `count=4` |
| `config.http.loaded endpoint=/api/configs/regions count=` | `count=2` |
| `events.loaded count=` | `count=5` |
| `zones.loaded count=` | `count=2` |
| `admin_spawn.done spawned=4 template_count=4` | 必须出现 1 行 |
| `cascade.violations` / `zones.spawn_error` / `admin_spawn.parse_error` / `admin_spawn.instance_error` / `config.http_error` | **各 0 行** |
| `npc_active_count` Σ（/metrics 取，≥1s 后）| **6**（4 模板路径 + 2 zone 路径）|

### 双路径 spawn 总和（Server CC 敲定）

```
模板路径：admin_spawn.done spawned=4（4 个 enabled NPC 各实例化一份）
zone 路径：e2e_village 从 spawn_table[0] spawn e2e_bare × 2
e2e_empty 空 spawn_table → zone.Spawn 零次不报错
───────────────────────────────────────
实例总数 = 4 + 2 = 6
其中 e2e_bare 有 3 个实例（1 模板 + 2 zone）
```

## R4 故障注入矩阵（第二轮）

### R4.1 dangling region

- 操作：PATCH `e2e_village.spawn_table[0].template_ref = "missing_npc_xxx"`（需先 toggle-disable region → update → toggle-enable；Admin 写入时会触发 47006/47007 写时拦截 → 所以必须 bypass 直接改 DB 写入）
- 另一种简化做法：不改现有 region，新建一个 `e2e_dangling_region` 引用不存在模板 —— 但 Admin 写入 API 会拦截。
- **最终做法**：直接 SQL UPDATE `regions.spawn_table` 写入 dangling ref（绕过写时校验），保留对 Server 侧导出期校验 + fail-fast 的验证目的。
- 预期：
  - `/api/configs/regions` 返 500 + code=47011
  - Server 日志 `config.http.regions.dangling region_id=e2e_village ref_type=\w+ ref_value=missing_npc_xxx reason=...` ≥1 行
  - Server 日志 `config.http_error err=".*code=47011.*"` 1 行
  - Server 容器 `RestartCount >= 2`
  - `zones.loaded` / `admin_spawn.done` 不出现

### R4.2 dangling fsm_ref

- 操作：新建一条 NPC `e2e_dangling_fsm`，fsm_ref 指向不存在 FSM `missing_fsm_xxx` —— 同上，需 SQL bypass 写入（Admin 写入 API 会拦截）
- 预期：
  - `/api/configs/npc_templates` 返 500 + code=45016
  - Server 日志 `config.http_error err=".*api/configs/npc_templates: status 500.*"` 1 行
  - **不解码 details**（Server 现状 fetchEndpoint 通用路径不解码业务码；Server CC 确认此为已知落差）
  - Server 容器 `RestartCount >= 2`
  - `regions` / `zones.loaded` / `admin_spawn.done` 不出现

## R5 产出物

- `requirements.md`（本文件）—— 数据矩阵 + 预期响应 + 故障注入矩阵
- `execution-plan.md` —— reset/seed/verify 流程 + UI 抽查 checklist
- `scripts/e2e/reset.sh` —— Admin 侧 wipe
- `scripts/e2e/seed.sh` —— HTTP API 灌 e2e 数据
- `scripts/e2e/verify.sh` —— 日志 grep + /metrics 对账
- `joint-report.md` —— 第一轮 + 第二轮合并结果（双边各存一份）

## R6 非目标

- **L3 运行级**（FSM 转换 / BT 节点执行 / perception 事件分发）不在本轮范围
- **Server 侧 45016 details 补强**（4 端点对称）不在本轮；Server CC 视跑完后决定是否补 PR
- **前端 UI 完整回归**不做；仅做 List + 1 条 Form 的薄层抽查（覆盖渲染正常性）
