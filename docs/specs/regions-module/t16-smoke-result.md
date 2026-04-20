# regions-module T16 后端 e2e smoke 结果

> 执行日期：2026-04-20
> admin-backend：镜像 5h+20min 内的 T9-T12 版本（localhost:9821）
> 场景编号对齐 [design.md §8.2](design.md) 表

§8.2 共 7 场景，其中 #5/#6 为前端手测（T17 scope）；T16 跑脚本化的 5 个后端场景。

## # 1：正常路径 /api/configs/regions

```bash
$ curl http://localhost:9821/api/configs/regions
{"items":[{"name":"village_outskirts","config":{"region_id":"village_outskirts","name":"村庄外围","region_type":"wilderness","spawn_table":[{"count":2,"spawn_points":[{"x":10,"z":20},{"x":15,"z":20}],"template_ref":"villager_guard","wander_radius":5,"respawn_seconds":60}]}}]}
HTTP 200
```

**结论 ✅**：envelope.name=`village_outskirts`（业务键）/ config.name=`村庄外围`（显示名）正确双层解耦；spawn_table 原样透传含 respawn_seconds=60。

## # 2：悬空路径（villager_guard 被禁用）

```bash
$ curl -X POST /api/v1/npcs/toggle-enabled -d '{"id":5,"enabled":false,"version":1}'
{"code":0,"data":"操作成功"}

$ curl http://localhost:9821/api/configs/regions
{"code":47011,"details":[{"npc_name":"village_outskirts","ref_type":"npc_template_ref","ref_value":"villager_guard","reason":"missing_or_disabled"}],"message":"区域导出失败：存在悬空的 NPC 模板引用，请按 details 修复"}
HTTP 500
```

**结论 ✅**：code=47011、details[0].ref_type=`npc_template_ref`、ref_value=`villager_guard`、reason=`missing_or_disabled`、`npc_name` 字段此处承载 region_id=`village_outskirts`（T8 约定 — Details 复用 []NPCExportDanglingRef 类型）。jq 等效断言 `.code==47011 and (.details|length)>0` 通过。

## # 3：隔离性（同一 disabled 状态下其他 4 export 端点）

```bash
/api/configs/event_types → HTTP 200
/api/configs/fsm_configs → HTTP 200
/api/configs/bt_trees    → HTTP 200
/api/configs/npc_templates → HTTP 200
```

**结论 ✅**：regions 导出的悬空引用不污染其他 4 端点；隔离性达标。

> 还原：`POST /npcs/toggle-enabled {"id":5,"enabled":true,"version":2}` → /api/configs/regions 回 200。

## # 4：CRUD 9 步矩阵（region_id=`ctrl_region`，引 villager_guard 已恢复）

| 步骤 | 请求 | 响应 |
|------|------|------|
| 1 create | POST /regions/create | `code=0 data.id=5 region_id=ctrl_region` ✅ |
| 2 toggle enable v=1 | POST /regions/toggle-enabled | `code=0 data="操作成功"` ✅ |
| 3 update while enabled | POST /regions/update v=2 | `code=47009 message="请先停用该区域再编辑"` ✅ |
| 4 toggle disable v=2 | POST /regions/toggle-enabled | `code=0` ✅ |
| 5 update while disabled | POST /regions/update v=3 | `code=0 data="保存成功"` ✅ |
| 6 toggle enable v=4 | POST /regions/toggle-enabled | `code=0` ✅ |
| 7 delete while enabled | POST /regions/delete | `code=47008 message="请先停用该区域再删除"` ✅ |
| 8 toggle disable v=5 | POST /regions/toggle-enabled | `code=0` ✅ |
| 9 delete while disabled | POST /regions/delete | `code=0 data={id:5,name:ctrl_region,...}` ✅ |

**结论 ✅**：
- 47009 / 47008 两个启用态前置拦截正常
- 乐观锁 version 从 1→5 稳定递增
- 软删除 endpoint 返 DeleteResult 承载 id/name/label 供 UI 确认展示
- 测试 region 已物理清理（软删除后不再影响导出）

## # 7：seed 幂等（连跑两次 `go run ./cmd/seed`）

```
1st run (region 段)：
  [跳过] region_type 字典 wilderness（已存在）
  [跳过] region_type 字典 town（已存在）
  区域类型字典写入完成：新增 0 条，跳过 2 条（已存在）
  [跳过] region village_outskirts（已存在）
  区域种子写入完成：新增 0 条，跳过 1 条（已存在）

2nd run (region 段)：同 1st — 全部 [跳过]（INSERT IGNORE 幂等）
```

**结论 ✅**：2 字典枚举 + 1 region fixture 三条均命中 UNIQUE 约束被静默跳过；`新增 0 条` 计数器真实反映；与 014/015 既有 seed 文件的幂等模式一致。

> 首次跑（fresh DB）对应 T12 commit 期历史验证，本次 1st run 是叠加在已跑过的 DB 上，所以等同 2nd run 的 [跳过] 文本。

---

## 汇总

| # | 场景 | 结果 |
|---|------|------|
| 1 | 正常路径导出 | ✅ |
| 2 | 悬空 47011 | ✅ |
| 3 | 跨端点隔离性 | ✅ |
| 4 | CRUD 9 步 + 47008/47009/乐观锁 | ✅ |
| 5 | 前端红点（47006/47007）| T17 覆盖 |
| 6 | 乐观锁弹窗（47010）| T17 覆盖 |
| 7 | seed 幂等 | ✅ |

T16 后端 5/5 PASS。剩 T17（前端 2 场景 + 前端 e2e）由 UI 手测覆盖。
