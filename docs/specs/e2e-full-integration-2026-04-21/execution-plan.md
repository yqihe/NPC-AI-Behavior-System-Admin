# e2e-full-integration-2026-04-21 — 执行计划

配套 [requirements.md](requirements.md)。双边流程，Admin 侧手顺自动化，Server 侧由 Server CC 提供 runbook。

## 前置

| 工具 | 检查方式 |
|---|---|
| docker compose | `docker compose version` |
| curl | `curl --version` |
| jq | `jq --version` |
| mysql client | 通过 `docker compose exec mysql` 进入，无需本机装 |

Admin 侧容器 `npc-admin-backend` + `npc-admin-mysql` + `npc-admin-redis` + `npc-admin-frontend` 必须已起且 healthy（端口 9821 / 3306 / 6379 / 3000）。

Server CC 侧 `docker-compose.yml`（`Server-v1/`）配置 `NPC_ADMIN_API=http://npc-admin-backend:9821`（或宿主机映射对应地址），确保能通过 docker 网络或宿主网络访问 Admin。

## 第一轮：happy path + disable fan-out

### Step 1：Admin 侧清空 + 灌 base + 灌 e2e

```bash
# 从仓库根目录执行
bash scripts/e2e/reset.sh       # 清 npcs + npc_bt_refs + regions；保留 fields/templates/FSM/BT/events/bb_keys/dicts
bash scripts/e2e/seed.sh        # API 灌 1 template + 5 NPC + 2 region
```

`reset.sh` 最终会跑 `cmd/seed`（幂等）确保 base 数据在，然后对 `npcs` + `npc_bt_refs` + `regions` 三张表做 TRUNCATE（**只清这 3 张**，其他 cmd/seed 内容完整保留）。

`seed.sh` 走 HTTP API：
1. GET `/fields/list` 解析 12 个 field_id
2. POST `/templates/create` 建 `e2e_template_full`
3. POST `/npcs/create` × 5 建 5 NPC
4. POST `/npcs/toggle-enabled` 把 `e2e_disabled` 停用
5. POST `/regions/create` × 2 建 2 region

每步 assert HTTP 200 + `code=0`；任一失败立刻 exit 1。

### Step 2：Admin 侧自检 `/api/configs/*` 响应

`seed.sh` 末尾 5 次 curl + jq 检查条数：
- `/api/configs/event_types` → `items | length == 5`
- `/api/configs/fsm_configs` → `items | length == 3`
- `/api/configs/bt_trees` → `items | length == 6`
- `/api/configs/npc_templates` → `items | length == 4`（验证 disable fan-out）
- `/api/configs/regions` → `items | length == 2`

### Step 3：Server 侧清空 + 冷启动

Server CC 执行（参考 Server-v1 runbook）：
```bash
docker compose down
docker compose up --build -d
sleep 5   # 等 HTTPSource 拉取完成 + scheduler 开始 tick
```

### Step 4：Admin 侧对账

```bash
SERVER_COMPOSE_DIR=../NPC-AI-Behavior-System-Server-v1 \
SERVER_CONTAINER=server-v1-server-1 \
bash scripts/e2e/verify.sh first-round
```

`verify.sh` 从 Server 容器 stdout 取日志（`docker compose logs`）+ 从 Server `/metrics` 端点取 npc_active_count，按 [requirements.md §R3](requirements.md) 的预期锚点表逐行 assert。

输出：PASS / FAIL 矩阵。全 PASS 则第一轮通过。

### Step 5：前端 UI 薄层抽查（手测）

开 http://localhost:3000，对每个模块做最小 smoke：

| 模块 | List 页 | Form 抽查 |
|---|---|---|
| Fields | 打开，过滤 enabled=true，确认列出 ≥11 条（不含 hp）| 随机点一条进详情，不编辑 |
| Templates | 打开，确认列出 5 条（4 老 + `e2e_template_full`）| 点 `e2e_template_full` 详情，确认 12 字段显示 |
| NPCs | 打开，确认列出 5 条 e2e_* | 点 `e2e_full` 详情，确认 5 opt-in bool 全 true + group_id/social_role 正确 |
| FSM | 打开 | 点 `fsm_combat_basic` 详情，确认 4 状态 + transitions 可视化 |
| BT | 打开 | 点 `bt/combat/attack` 详情，确认节点树渲染 |
| Event Types | 打开 | 点 `gunshot` 详情，确认 default_severity=90 等字段 |
| Regions | 打开，确认列出 2 条 | 点 `e2e_village` 详情，确认 spawn_table 嵌套编辑器渲染 |

每项有一处渲染异常即为 UI 侧 FAIL，记录到 joint-report.md。

### Step 6：第一轮结果归档

对账 + UI 抽查全 PASS → 填 [joint-report.md](joint-report.md) 第一轮段。任何 FAIL → 定位根因（Admin 侧 / Server 侧 / 契约不一致），修复后重跑 Step 1-4。

## 第二轮：故障注入 × 2

### R4.1 dangling region

```bash
# 1. Admin 侧直接 SQL 写入 dangling ref（绕过写入校验；e2e 场景认可）
docker compose exec -T mysql mysql -uroot -proot npc_ai_admin <<'SQL'
UPDATE regions
SET spawn_table = JSON_SET(spawn_table, '$[0].template_ref', 'missing_npc_xxx')
WHERE region_id = 'e2e_village' AND deleted = 0;
SQL

# 2. Admin 侧清缓存
docker compose exec -T redis redis-cli FLUSHDB

# 3. Server CC 重启
docker compose restart server-v1-server-1   # 由 Server CC 执行

# 4. 对账
bash scripts/e2e/verify.sh dangling-region
```

预期：verify.sh 检出 `config.http.regions.dangling region_id=e2e_village ref_value=missing_npc_xxx` ≥1 行 + `config.http_error ... code=47011` 1 行 + RestartCount ≥2。全 PASS 记入 joint-report。

恢复：

```bash
docker compose exec -T mysql mysql -uroot -proot npc_ai_admin <<'SQL'
UPDATE regions
SET spawn_table = JSON_SET(spawn_table, '$[0].template_ref', 'e2e_bare')
WHERE region_id = 'e2e_village' AND deleted = 0;
SQL
docker compose exec -T redis redis-cli FLUSHDB
```

### R4.2 dangling fsm_ref

```bash
# 1. Admin 侧直接 SQL 改 e2e_full 的 fsm_ref
docker compose exec -T mysql mysql -uroot -proot npc_ai_admin <<'SQL'
UPDATE npcs
SET fsm_ref = 'missing_fsm_xxx'
WHERE name = 'e2e_full' AND deleted = 0;
SQL

# 2. 清缓存
docker compose exec -T redis redis-cli FLUSHDB

# 3. Server 重启 + 对账
bash scripts/e2e/verify.sh dangling-fsm
```

预期：verify.sh 检出 `config.http_error err=".*status 500.*"` 1 行 + RestartCount ≥2 + `regions` / `zones.loaded` / `admin_spawn.done` 不出现。全 PASS 记入 joint-report。

恢复：
```bash
docker compose exec -T mysql mysql -uroot -proot npc_ai_admin <<'SQL'
UPDATE npcs SET fsm_ref = 'fsm_combat_basic' WHERE name = 'e2e_full' AND deleted = 0;
SQL
docker compose exec -T redis redis-cli FLUSHDB
```

## 异常与回滚

- 任何轮次 FAIL 不覆盖清除 DB 状态；保留现场便于排查
- Server 容器 RestartCount 失控时：`docker compose stop server`（Server CC 侧）
- Admin 侧状态污染时：重跑 `reset.sh + seed.sh` 一键恢复

## 产出归档

- Admin 侧 → 本目录 `joint-report.md`
- Server 侧 → `Server-v1/docs/specs/e2e-full-integration-2026-04-21/joint-report.md`
- Memory 长期参照：`project_e2e_full_integration_2026-04-21.md` —— 记录本次结果 + 暴露的任何 Server/Admin 侧 issue
