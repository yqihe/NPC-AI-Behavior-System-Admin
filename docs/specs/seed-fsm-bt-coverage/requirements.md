# seed-fsm-bt-coverage — 需求分析

## 动机

**2026-04-20 R15 live smoke 起步时暴露的冷启动缺陷**：ADMIN HEAD `0aa77b2` 执行 `go run ./cmd/seed` 后，`GET /api/configs/npc_templates` 返回 `code=45016`（export-ref-validation 拒绝 25 条悬空引用）。6 NPC 的 `fsm_ref` + `bt_refs` 硬引用了 3 个 FSM 配置 + 6 棵 BT，但 seed **从不创建这些 FSM/BT**。

**历史假阳**：`external-contract-admin-shape-alignment` 的 T8 验收（commit `61fe8eb` 2026-04-19 13R 全 PASS）在富环境下跑 `verify-seed.sh`——当时的 `npc-ai-behavior-system-admin_mysql-data` 卷里有开发期累积的 FSM/BT，脚本"非破坏性不 wipe"的设计让检查顺过。冷启动从未真正走过，`export-ref-validation` spec（commit `8127657` 合并后新增的严格校验）与遗漏的 FSM/BT seed 合起来**注定在干净环境下 500**。

**不做的后果**：
- Server CC R15 live smoke（docker compose up --build 拉 ADMIN `0aa77b2`）100% FAIL
- CI / 新人本地复现 / 毕设答辩评委自己起环境 → 全部卡 500
- `verify-seed.sh` 承诺"首次运行（空 DB）→ PASS" 是谎言

## 优先级

🔴 **高**。R15 smoke 唯一阻塞项。延期 R15 直到本 spec 交付。

- Server CC 侧 Phase 3 已完成 R1–R14 + R16（T10 smoke 文档 commit `b19639b`），仅 R15 待 ADMIN 就绪
- 毕设联调最终闭环依赖 R15 PASS
- 对方无法继续推进（R15 是他们 Phase 3 最后一道 gate）

## 预期效果

**场景 1：干净环境冷启动**  
`docker compose up -d --build` → `go run ./cmd/seed` → `curl http://localhost:9821/api/configs/npc_templates` 直接返回 200 + 6 NPC items，零悬空引用。

**场景 2：Server CC R15 smoke 通过**  
Server CC 本地 `docker compose up --build` 拉 ADMIN `0aa77b2` live → 6 ADMIN NPC + 3 zone butterfly = 9 NPC spawn → tick ≥30s 无 WARN/ERROR。

**场景 3：verify-seed.sh 真正测冷启动**  
`docker compose down -v` + `docker compose up -d` + `bash scripts/verify-seed.sh` 全绿。脚本头部注释"首次运行（空 DB）→ PASS"名实相符。

**场景 4：新人/评委本地起环境**  
`README` 的 3 步 quickstart 可复现；不需要任何"先手建 FSM/BT"的隐性前置。

## 依赖分析

**上游依赖（阻塞本 spec）**：
- **Server CC 提供 3 FSM + 6 BT 的权威结构**：本 spec **最大**的跨项目依赖。ADMIN 侧 seed 硬编码的 FSM/BT 必须与 Server CC `NewInstanceFromADMIN` 的消费期望对齐，否则 spawn 即使成功行为也跑偏。需要 Server CC 出：
  - 3 个 FSM 的 states / transitions 完整 JSON（`fsm_combat_basic` / `fsm_passive` / `guard`）
  - 6 棵 BT 的节点树完整 JSON（`bt/combat/{idle,chase,attack,patrol}` / `bt/passive/wander` / `guard/patrol`）
  - 来源建议：服务端仓之前的开发期 snapshot / v2 fixture 翻译层 / 或现编一个最小可 spawn 行为集（足以支持 R15 tick ≥30s 无 WARN）
- `bt-data-format-unification` (`747b0c3`)：BT 节点格式规范已统一，本 spec 注入的 BT JSON 走严格 validator
- `export-ref-validation` (`8127657`)：ref-validation 本身工作正常，本 spec 只是给它**正确的数据**

**下游被依赖**：
- R15 live smoke 通过 → Server CC Phase 3 收尾
- `bb-key-runtime-registry` Phase 2 design 解冻后开工（时序上本 spec 先于 bb-key-runtime-registry）
- `verify-seed.sh` 在 CI 里的可行性

**不依赖**：
- **不依赖**契约演进：本 spec **不改** `api-contract.md v1.1`，**不改** `snapshot-section-4.json`。FSM/BT 走独立导出端点 `/api/configs/{fsm_configs,bt_trees}`；NPC 导出 JSON 内只含 `fsm_ref` / `bt_refs` 字符串不嵌入定义。所以契约冻结窗口即使已解除，本 spec 也**不触发 v1.2**
- **不依赖** ADMIN 前端：seed 走 CLI + DB，前端无关

## 改动范围

**新增文件（预估 3 个）**：

| 类别 | 文件 | 预估行数 |
|---|---|---|
| seed | `backend/cmd/seed/fsm_bt_seed.go` | ~250 |
| fixture | `backend/cmd/seed/fixtures/fsm_combat_basic.json` + 2 | ~300（3 FSM JSON） |
| fixture | `backend/cmd/seed/fixtures/bt_*.json` | ~400（6 BT JSON） |

**修改文件（预估 4 个）**：

| 文件 | 动作 | 预估行数 |
|---|---|---|
| `backend/cmd/seed/main.go` | 在 seedFieldsTemplatesNPCs 前调用 seedFsmConfigs + seedBtTrees | +20 |
| `backend/cmd/seed/field_template_npc_seed.go` | 无改动（保持 hp 孤儿等历史包袱） | 0 |
| `scripts/verify-seed.sh` | 新增 3 FSM + 6 BT 存在性断言；移除"非破坏性"承诺改为"可 wipe 可不 wipe"；首次运行断言新增"FSM 3 条 / BT 6 棵"行 | +30 |
| `docs/specs/external-contract-admin-shape-alignment/ops-runbook.md` | §seed 覆盖范围 从"字段+模板+NPC"扩到"字段+模板+NPC+FSM+BT"；新增"本次 spec 补齐"备注引用本 spec | +20 |

**总计约 ~1000 行（新增+修改，JSON fixture 占大头）**。

## 扩展轴检查

**扩展轴 1（新增配置类型）**：⚪ **无影响**  
本 spec 不新增配置类型，只是补齐已有 FSM/BT 类型的 seed 覆盖。

**扩展轴 2（新增表单字段）**：⚪ **无影响**  
同上。

## 验收标准

| 编号 | 标准 | 验证方式 |
|---|---|---|
| R1 | `docker compose down -v` + `docker compose up -d` + `go run ./cmd/seed` 后，fsm_configs 表有 3 条（`fsm_combat_basic` / `fsm_passive` / `guard`），全部 enabled=1 / deleted=0 | `SELECT name, enabled FROM fsm_configs` 断言 |
| R2 | 同上，bt_trees 表有 6 棵（`bt/combat/{idle,chase,attack,patrol}` / `bt/passive/wander` / `guard/patrol`），全部 enabled=1 / deleted=0 | `SELECT name, enabled FROM bt_trees` 断言 |
| R3 | 干净环境冷启动 `curl /api/configs/npc_templates` 返回 200 + 6 items，零 45016 | curl + http code + items count 断言 |
| R4 | 干净环境冷启动 `curl /api/configs/fsm_configs` 返回 200 + 3 items；`curl /api/configs/bt_trees` 返回 200 + 6 items | 同上 |
| R5 | `GET /api/configs/npc_templates` 导出与 `docs/integration/snapshot-section-4.json` 逐字节一致（不因本 spec 回归） | 沿用 verify-seed.sh Step 3 diff |
| R6 | seed 幂等重跑：fsm/bt 段输出"新增 0 条，跳过 N 条" | 沿用 R7 模式 |
| R7 | 3 FSM + 6 BT 的 JSON fixture 内容来源**已与 Server CC 对齐**（PR 合入前需要 Server CC review fixture 语义，确认与 `NewInstanceFromADMIN` 消费期望一致） | fixture 文件头注释引用 Server CC commit hash + PR review 记录 |
| R8 | `scripts/verify-seed.sh` 在干净环境冷启动通过；脚本头部注释改为明确"首次运行/重跑均 PASS"，不再有"依赖富 DB"的隐性前置 | `docker compose down -v && docker compose up -d && bash scripts/verify-seed.sh` 退出码 0 |
| R9 | `docs/specs/external-contract-admin-shape-alignment/ops-runbook.md` seed 覆盖范围文档更新（§seed 脚本的作用 新增 "FSM 配置：3 条" + "行为树：6 棵"） | 手读文档 |
| R10 | Server CC R15 live smoke 通过：docker compose up --build 拉 ADMIN cold-start live → 6 ADMIN NPC + 3 zone butterfly spawn + tick ≥30s 无 WARN/ERROR | 由 Server CC 跑（跨项目验收，非本仓脚本化） |

## 不做什么

- **不改** `api-contract.md v1.1`（不进 v1.2）
- **不改** `docs/integration/snapshot-section-4.json`（R5 守住零偏差）
- **不扩** NPC 数量 / 不改 NPC fields（snapshot §4 全部保留）
- **不做** FSM/BT 的前端可视化编辑改动（既有编辑器继续工作即可）
- **不重构** seed 结构为模块化注入（主入口加 2 行调用即可，过度设计不接入）
- **不做** FSM/BT 导出引用反向校验（`export-ref-validation` 已覆盖）
- **不做** FSM/BT 的版本管理/审计日志（与其他配置模块一致的延后策略）

## 一个 spec 还是多个？

本 spec 含：FSM seed + BT seed + verify-seed.sh 更新 + ops-runbook 更新 + **跨项目 fixture 对齐**（R7）。判定**紧耦合、不可拆**：

- 若只做 FSM seed 不做 BT seed：export 仍有 5 条 BT 悬空
- 若只做 BT seed 不做 FSM seed：export 仍有 3 条 FSM 悬空
- 若只做 seed 不更新 verify-seed.sh / ops-runbook：下次运营/新人踩同样的假阳坑
- 若 fixture 不对齐 Server CC：spawn 成功但行为与 R15 tick expect 不符

四件共同构成"让干净环境冷启动真正能跑通 R15 smoke"的最小可行完整交付。

## 一个关键未决问题（需 Server CC 回复后才能进 Phase 2 design）

**OQ1：FSM/BT fixture 数据源**

三种候选路径，等 Server CC 侧决定：

1. **从服务端仓 v2 fixture 翻译**：`NPC-AI-Behavior-System-Server/configs/npc_types/*.json` 或 `internal/experiment/` 下的 fixture 是否可翻译为 ADMIN shape 的 FSM/BT JSON？若可行最快，但需要 Server CC 出翻译映射说明
2. **从服务端仓开发期 snapshot 抽取**：服务端有没有之前从 ADMIN 抓的完整导出（不仅 `npc_templates`，还含 `fsm_configs` + `bt_trees`）？若有直接 byte-copy
3. **双方合作现编最小行为集**：只保证 R15 tick ≥30s 无 WARN，不承诺行为语义完整（后续 `bb-key-runtime-registry` 上线后策划会重写条件）。最快但行为最简陋，毕设答辩展示效果打折

**推荐路径 2 > 路径 1 > 路径 3**——零数据冲突风险。

## 阶段产出物一览

- `requirements.md`（本文件，Phase 1 产出，**未 commit**）
- `design.md`（Phase 2 产出，等 OQ1 答复 + Server CC 提供 fixture 原始数据后开工）
- `tasks.md`（Phase 3 产出）
