# regions-module — 需求分析

## 动机

2026-04-20 双边联调（`/integration`）中，Server CC 主动递 regions 模块契约触发 scope 重启。Server 侧已有完整 zone 运行时能力：数据模型（[`internal/runtime/zone/zone.go:16-38`](../../../../NPC-AI-Behavior-System-Server/internal/runtime/zone/zone.go#L16)）+ ZoneManager（Add/Get/Sleep/Wake/IsActive）+ Scheduler zone 过滤（tick 跳过 sleep zone + cross-zone 事件过滤）+ event.Envelope.ZoneID 传播 + Metrics.ZoneActiveCounts。但 `HTTPSource.LoadRegionConfig / LoadAllRegionConfigs`（[`internal/config/http_source.go:194-200`](../../../../NPC-AI-Behavior-System-Server/internal/config/http_source.go#L194)）是 stub，返 `not yet implemented` / 空 map。Server 当前只能走 JSONSource 读本地 `configs/regions/meadow.json`（butterfly_01 × 3 本地 fixture，红线保留不迁），生产模式缺 HTTP config 源。

ADMIN 侧 regions 模块 Phase 0 — 前后端零实现。`docs/deferred-features.md` 外 memory 记为"延后到毕设后"，因 Server HTTPSource stub 的激活条件 + 毕设答辩需要端到端 zone 链路演示（策划可视化配置 → NPC spawn 带 zone_id → 跨 zone 事件过滤），本轮联调决定重启 scope。

**不做的后果**：
- Server HTTPSource 永久 stub，生产模式强制走 JSONSource 本地 JSON 文件，双机部署时配置同步 ad hoc
- 毕设答辩无法完整演示 zone 链路（meadow 是 Server 本地 fixture，Admin 无 UI 入口展示"策划可视化定义 spawn region"）
- 未来做 sleep/wake 运行时 API 时 config 静态层缺位

## 优先级

🟡 **中**。双边联调触发的 scope 重启，非硬性 gate。毕设核心承诺已在 theft_alarm 闭环交付，regions 是增值演示面。

- Server CC 本期同 PR 配合：HTTPSource 接入 + e2e（HTTPSource mock server + zone_id 注入断言）+ 清理 `configs/schemas/region.json` 遗留
- ADMIN 侧是方案起点，Server 等 `GET /api/configs/regions` shape 稳定后一把撸

## 预期效果

**场景 1：策划 UI 配置**  
策划登录 ADMIN → 区域管理页 → 新建"野外"region，`region_id=village_outskirts` / `region_type=wilderness` → spawn_table 添加 `villager_guard × 2`（spawn_points 2 点 + wander_radius=5m + respawn_seconds=60）→ 保存 → toggle-enabled 激活。

**场景 2：R13 引用完整性**  
策划保存 region 时 `template_ref` 指向不存在/未启用的 NPC template → 后端返 45017 + details 指向具体 spawn_entry index + 提示问题 template 名。

**场景 3：Server HTTPSource e2e**  
Server 本地 `docker compose up --build` 带 `NPC_ADMIN_API=http://localhost:9821` → `HTTPSource.LoadAllRegionConfigs` → 收到 `village_outskirts` → `ZoneManager.AddZone(z)` + `z.Active=true` + `z.Spawn(ctx)` → villager_guard × 2 spawn → `PositionComponent.ZoneID="village_outskirts"` → 跨 zone 事件按 `scheduler.go:324` 过滤生效 → tick ≥30s 无 WARN/ERROR。

**场景 4：导出契约**  
`GET /api/configs/regions` 返 `{"items":[{"name":"village_outskirts","config":{region_id,name,region_type,spawn_table}}]}`，`enabled` / `version` 剥离，对齐现有 `npc_templates` / `fsm_configs` / `bt_trees` / `event_types` 统一 export shape。

## 依赖分析

**依赖（均已完成）**：
- `util/const.go` DictGroup pattern — 新增 `DictGroupRegionType`
- errcode 45000 段可扩展（45016 已被 export-ref-validation 占用，45017 空闲）
- MySQL migration pattern（见 bb-key-runtime-registry T1 / seed-fsm-bt-coverage）
- store/mysql + store/redis 双层 cache pattern（对齐 bt_tree / fsm_config）
- R13 引用启用校验 pattern（[`service/bt_tree.go`](../../../backend/internal/service/bt_tree.go) `CheckEnabledByNames`）
- Handler 5 步编排 + 5xx 错误契约（export-ref-validation）
- 前端选择器第 3 组范式（bb-key-runtime-registry T13-T16）— NPC template 选择器复用
- 写入 API 3 反直觉语义（memory `feedback_admin_write_api_quirks.md`）— toggle-enabled / disable→update→enable / value+ref_key 互斥

**被依赖**：
- Server CC 本 PR HTTPSource 接入（`/api/configs/regions` shape 稳定后撸）
- 未来运行时 sleep/wake API（本期只打 `enabled` 静态位，Active 不暴露）

## 改动范围

**后端（~12 新文件 + ~7 改动）**：

| 层 | 新增 | 改动 |
|----|------|------|
| migration | `NNN_regions.up.sql` / `.down.sql` | — |
| util/const | — | `+DictGroupRegionType` |
| errcode | — | 新增 47xxx 段 11 条（CRUD + 引用校验 + 导出悬空，详见 design.md §1.3） |
| model | `region.go` | — |
| store/mysql | `region.go` | — |
| store/redis | `region_cache.go` | — |
| service | `region.go` | — |
| handler | `region.go` | `export.go` +5 步编排 `Regions()` |
| router | — | `register region routes` |
| setup | — | 注册 wiring |
| seed | `region_seed.go` | `main.go` +调用 / 字典 seed +region_type 两枚举 |

**前端（~4 新文件 + ~2 改动）**：

| 类型 | 新增 | 改动 |
|------|------|------|
| api | `regions.ts` | — |
| views | `RegionList.vue` + `RegionForm.vue` | — |
| router | — | `+/regions` 路由 |
| layout | — | `+侧栏菜单项` |

## 扩展轴检查

✅ **扩展轴 1（新增配置类型）**：regions 是教科书级"加一组 handler/service/store/validator"，additive 扩展。既有模块代码零侵入（errcode/util/const/seed main/dict seed/export handler router 是 additive diff，不改语义）。

✅ **扩展轴 2（新增表单字段）**：SpawnEntry 嵌套 array（策划需动态增删 spawn_entries + 二级 spawn_points array）。对齐 BtTreeForm / FsmConfigForm 先例用 RegionForm.vue 自定义字段块，**不动** SchemaForm 核心。

## 验收标准

| ID | 条目 | 可验证方式 |
|----|------|------------|
| R1 | `regions` 表含 region_id(unique)/name/region_type/spawn_table(JSON)/enabled/version/deleted/created_at/updated_at | migration up 后 `DESCRIBE regions` 匹配 schema |
| R2 | POST /api/v1/regions/create 合法载荷 → 200 + 新记录 version=1 + enabled=false | curl + `jq '.data.version==1 and .data.enabled==false'` |
| R3 | POST /api/v1/regions/update 对 enabled=true 记录直接拒绝 → 43010（复用 FSM 错误码语义） | curl enabled 态直接 update，断言 code=43010 |
| R4 | POST /api/v1/regions/toggle-enabled 用 `{id, version, enabled}` 目标值语义（非翻转） | 连发两次 enabled=true，version +2，enabled 始终 true |
| R5 | POST /api/v1/regions/list 支持 region_type + enabled 条件过滤 + 分页 | 查 wilderness 返 1，查 town 返 0 |
| R6 | POST /api/v1/regions/detail 返完整 config 含 spawn_table | 响应含 spawn_table 长度 ≥1 |
| R7 | POST /api/v1/regions/delete 软删（deleted=1）+ enabled=true 记录拒删（43xxx） | 已启用记录删除失败，disable 后删除成功，list 不再返 |
| R8 | create/update 时 `spawn_table[].template_ref` 指向不存在返 47006 / 指向未启用返 47007 + details 含 spawn_entry index + ref_value + reason | 构造悬空 ref，断言 `details[0].ref_type == "npc_template_ref"` |
| R9 | GET /api/configs/regions 返 `{items:[{name, config}]}`，config 含 `{region_id, name, region_type, spawn_table}`，**enabled/version 剥离** | curl + jq 断言 `.items[0].config` 无 enabled/version key |
| R10 | GET /api/configs/regions 导出期同 NPCTemplates handler 做引用复核：region enable 后被引 template 被 disable → 500 + 47011 + details | disable 被引 template，再 curl export，断言 code=47011 |
| R11 | `village_outskirts` seed 幂等：首次写 1 region + 2 dict 枚举；第二次跳过 | seed 二次跑，stdout 含 "跳过" 文本 |
| R12 | 前端 RegionList：分页 + region_type 筛选 + enabled 开关（toggle 调 API，乐观锁 version） | 手测分页切 / 筛选 / 启停 |
| R13 | 前端 RegionForm：SpawnEntry 数组增删 + 二级 spawn_points 数组增删 + template_ref 走 NPC 选择器（复用第 3 组范式） | 手测增删 + 选择器回填 |
| R14 | 前端 respawn_seconds 字段带 help-text "Server v3+ 生效，当前仅保存不调度" | 手测 UI 可见 |
| R15 | list/detail 经 Redis 缓存，create/update/toggle/delete 后失效（对齐 bt_tree pattern） | 代码 review + Redis MONITOR 验证 |
| R16 | `go test ./internal/service/...` + `make verify` 全绿 | 本地 / CI |
| R17 | `GET /api/configs/regions` 空 items 合法：Server 互斥模式下若 ADMIN 无任何 region，返 `{"items":[]}` 不报错 | 清空 regions 表后 curl，断言 200 + `items` 长度=0 |

## 不做什么

- ❌ **sleep/wake 运行时 API** — Server 统一 `Active=true` 启动态，运行时激活走未来 WS handler / admin 侧边通道，本期 ADMIN 只管 `enabled`（config 激活），两层正交
- ❌ **boundary / polygon / weather 字段** — Server 不消费（grep 0 引用），Admin 不收
- ❌ **坐标 y 维度** — Server `Position{X,Z float64}` 硬约束，Admin 前端表单禁止加 y 输入
- ❌ **region_type 扩 `dungeon` / `safezone` / `boss_arena` 枚举** — 本期锁 wilderness / town，未来按策划需求扩
- ❌ **迁移 Server `configs/regions/meadow.json`** — butterfly_01 本地 fixture 红线（`feedback_server_local_fixtures_protected.md`），Server HTTPSource 互斥模式下自然旁路，by design
- ❌ **respawn 运行时调度** — Server v3 roadmap，Admin 本期只存不调；前端 help-text 警示
- ❌ **spawn_table 外键子表** — 用 JSON 列对齐 NPC `bt_refs` 先例，避免 N+1 + 写放大
- ❌ **导入/批量新建** — 毕设阶段单条够用
- ❌ **跨 region 的 NPC 迁移工具** — 运行时范畴
- ❌ **region 级 spawn 权重 / 概率** — 本期固定 count，future 可加

---

**→ 停下，等用户审批后进入 Phase 2 设计**
