# e2e-full-integration-2026-04-21 — 联调基线报告（Admin 侧）

执行日期：2026-04-21
Admin 执行人：Admin CC
Server 执行人：Server CC
对照：[requirements.md](requirements.md) · [execution-plan.md](execution-plan.md)

## 结论

| 轮次 | 结果 |
|---|---|
| 第一轮（happy path + disable fan-out）| **PASS ✓** |
| 第二轮 R4.1（dangling region）| **PASS ✓** |
| 第二轮 R4.2（dangling fsm_ref）| **PASS ✓** |

三轮全绿。Admin ↔ Server HTTPSource 契约全链路验证通过。

## 第一轮：happy path + disable fan-out

### Admin 侧执行

```
bash scripts/e2e/reset.sh   # base 数据齐 + e2e 表空
bash scripts/e2e/seed.sh    # 1 template + 5 NPC + 2 region
```

### Admin 侧 `/api/configs/*` 自检（seed.sh Step 6）

| 端点 | 实际 count | 预期 | 结果 |
|---|---|---|---|
| `/api/configs/event_types` | 5 | 5 | ✓ |
| `/api/configs/fsm_configs` | 3 | 3 | ✓ |
| `/api/configs/bt_trees` | 6 | 6 | ✓ |
| `/api/configs/npc_templates` | 4 | 4（e2e_disabled 已滤）| ✓ **disable fan-out PASS** |
| `/api/configs/regions` | 2 | 2 | ✓ |

### Server 侧对账（verify.sh first-round）

启动锚点 9/9 命中：
- `config.source type=http` ≥1 行
- `config.http.loaded` 五端点 count 匹配（5/3/6/4/2）
- `events.loaded count=5`
- `zones.loaded count=2`
- `admin_spawn.done spawned=4 template_count=4`

禁区断言 5/5 全 0：
- `cascade.violations` / `zones.spawn_error` / `admin_spawn.parse_error` / `admin_spawn.instance_error` / `config.http_error`

`/metrics` 活跃数：**`npc_active_count Σ = 6`**（4 模板路径 + 2 zone 路径）。其中 `e2e_bare` 3 个实例（1 模板 + 2 zone），符合 R2 双路径 spawn 设计。

Server 容器 `RestartCount = 0`。

### Server CC 单方观察（记录）

- `config.source` 日志顺序出现在 5 个 `config.http.loaded` 之后（HTTPSource 构造完后才打源标注），非 runbook 描述的"启动阶段第 2 步"。不影响对账（正则存在性+count，非顺序判定），Server CC 将同步 spec 实际顺序。

### 前端 UI 薄层抽查

因 e2e 自动化已 100% 覆盖启动锚点 + 导出端点 + 日志对账，UI smoke 留作下一轮单独走，本次不阻塞。

## 第二轮 R4.1：dangling region

### Admin 侧注入

```sql
UPDATE regions
SET spawn_table = JSON_SET(spawn_table, '$[0].template_ref', 'missing_npc_xxx')
WHERE region_id = 'e2e_village' AND deleted = 0;
```
+ `redis-cli FLUSHDB`。

### Admin 侧端点响应

```
GET /api/configs/regions
→ HTTP 500
→ body: {"code":47011,"details":[{"npc_name":"e2e_village","ref_type":"npc_template_ref","ref_value":"missing_npc_xxx","reason":"missing_or_disabled"}],"message":"区域导出失败：存在悬空的 NPC 模板引用，请按 details 修复"}
```

### Server 侧对账（verify.sh dangling-region）

- `config.http.regions.dangling region_id=e2e_village ref_type=npc_template_ref ref_value=missing_npc_xxx reason=missing_or_disabled` ≥1 行 ✓
- `config.http_error code=47011` ≥1 行 ✓
- Server 容器 `RestartCount = 11`（≥2）✓
- `zones.loaded` / `admin_spawn.done` 后续阶段行 0 ✓

### 恢复

```sql
UPDATE regions SET spawn_table = JSON_SET(spawn_table, '$[0].template_ref', 'e2e_bare') WHERE region_id = 'e2e_village' AND deleted = 0;
```

## 第二轮 R4.2：dangling fsm_ref

### Admin 侧注入

```sql
UPDATE npcs SET fsm_ref = 'missing_fsm_xxx' WHERE name = 'e2e_full' AND deleted = 0;
```
+ `redis-cli FLUSHDB`。

### Admin 侧端点响应

```
GET /api/configs/npc_templates
→ HTTP 500
→ body: {"code":45016,"details":[{"npc_name":"e2e_full","ref_type":"fsm_ref","ref_value":"missing_fsm_xxx","reason":"missing_or_disabled"}],...}
```

### Server 侧对账（verify.sh dangling-fsm）

- `config.http_error err=".*api/configs/npc_templates: status 500.*"` 1 行 ✓
- 前 3 端点（event_types / fsm_configs / bt_trees）loaded ✓（各 count 正确）
- `config.http.loaded endpoint=/api/configs/npc_templates` 0 行 ✓（fail-fast 卡在第 4 端点）
- `config.http.loaded endpoint=/api/configs/regions` 0 行 ✓（未到）
- `zones.loaded` / `admin_spawn.done` 0 行 ✓
- Server 容器 `RestartCount = 12`（≥2）✓

### 恢复

```sql
UPDATE npcs SET fsm_ref = 'fsm_combat_basic' WHERE name = 'e2e_full' AND deleted = 0;
```

恢复后 `/api/configs/npc_templates` → 200 / 4 条（e2e_bare, e2e_social, e2e_memo_emo, e2e_full），状态清洁。

## 暴露的 Issue

### Admin 侧

- **verify.sh 日志窗口 defect（已就地修）**：故障注入轮用 `docker compose logs --tail=500` 会把第一轮 happy 的 `config.source / zones.loaded / admin_spawn.done / server.start` 残留行带入判定（R4.1 首跑 `zones.loaded=1 / admin_spawn.done=1` 误判 FAIL）。修法：非 first-round 模式改用 `docker compose logs --since $(docker inspect --format '{{.State.StartedAt}}')`，把日志窗口缩到容器本次生命周期。已在 [scripts/e2e/verify.sh](../../../scripts/e2e/verify.sh) 中合入。
- **template / region 默认 enabled=0 需 toggle**：seed.sh 被迫在 create 后追加 toggle-enabled（NPCs 默认 enabled=1 不受影响）。非 bug，属已知反直觉语义（记在 [feedback_admin_write_api_quirks.md](../../../../../Users/Lenovo/.claude/projects/.../memory/feedback_admin_write_api_quirks.md)）。

### Server 侧（Server CC 确认）

- **fetchEndpoint 通用路径不解码业务码细节**：R4.2 日志只显 `status 500`，不显 `code=45016 / ref_value=missing_fsm_xxx`。R4.1 走 `fetchRegionsEndpoint` 特化路径能解 47011 details，对称覆盖 45016 需独立 PR 扩展 4 硬失败端点的 fetch 路径。不在本轮范围，由 Server CC 评估后续 PR。
- **日志顺序落差**：`config.source` 出现在 5 个 `config.http.loaded` 之后（非 runbook 描述的启动第 2 步），Server CC 将同步 spec。

### 双边

- **e2e 灌数走 API 被迫绕过故障注入**：Admin 写时校验（47006/47007/45016 等）正确拦截 dangling ref，测不了 Server 侧导出期 fail-fast。R4 用 SQL UPDATE 绕过写时校验是合理妥协，本次 spec 已明确认可。

## 性能 / 噪音观察

- reset.sh 耗时约 6s（`cmd/seed` 占大头）
- seed.sh 耗时约 3s（10+ HTTP POST）
- Server 首轮启动到稳定 tick ≈1s
- 故障注入轮 Server fail-fast 循环约 2s/次，11-12 次后总计约 20-25s

## 产出归档

- Admin 侧报告：本文件
- Server 侧报告：`../NPC-AI-Behavior-System-Server-v1/docs/specs/e2e-full-integration-2026-04-21/joint-report.md`（待 Server CC 填写）
- 配套脚本：[scripts/e2e/reset.sh](../../../scripts/e2e/reset.sh) · [scripts/e2e/seed.sh](../../../scripts/e2e/seed.sh) · [scripts/e2e/verify.sh](../../../scripts/e2e/verify.sh)

## 下一步

- Server CC 填 Server 侧 joint-report
- Admin 侧 git commit：spec + scripts + report 一并入 main
- memory 留档：`project_e2e_full_integration_2026-04-21.md` 记录双边结果 + verify.sh 日志窗口 defect 修法
