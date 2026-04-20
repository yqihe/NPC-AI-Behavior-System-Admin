# regions-module — 任务拆解

> 对应 [requirements.md](requirements.md) / [design.md](design.md)。每 task 完成后触发 `/verify` 通过（user memory `feedback_auto_verify.md`）。

## 任务依赖图

```
T1 migration ──┐
T2 errcode+const ─├──▶ T3 model ──▶ T4 store/mysql ──┬──▶ T6 service CRUD ──▶ T7 service validate ──▶ T8 service export 方法 ──┬──▶ T9 handler CRUD + 注册 ──▶ T10 handler export ──┬──▶ T12 seed ──▶ T16 后端 e2e
                                                     └──▶ T5 store/redis      (改 T6 Create/Update hook)                         │                                                   │
                                                                                                                                 └──▶ T11 service 单测                                │
                                                                                                                                                                                     ▼
                                                                                                                                                           T13 api ──▶ T14 RegionList + 路由 + 菜单
                                                                                                                                                                    └──▶ T15 RegionForm（嵌套 array）
                                                                                                                                                                                     │
                                                                                                                                                                                     ▼
                                                                                                                                                                                  T17 前端手测（跑完 R12-R14）
```

并行机会：
- T1/T2/T3 互不依赖 → 可并行
- T4/T5 可并行（同依赖 T3）
- T10/T11/T12 可并行（同依赖 T9）
- T13 解锁后 T14/T15 可并行

---

## T1：migration 016_create_regions （R1）  `[x]` 完成 2026-04-20

**关联需求**：R1
**文件**：`backend/migrations/016_create_regions.sql`（单文件对齐既有 014/015 先例）

**做什么**：按 design.md §1.1 落 DDL。单文件含 `CREATE TABLE IF NOT EXISTS regions (...) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;` — index 为 `uk_region_id` + `idx_list(deleted, enabled, id DESC)` + `idx_region_type(region_type, deleted)`。`spawn_table JSON NOT NULL`，seed 侧保证写 `'[]'` 起始。

**做完了是什么样**：
- MySQL 内 `DESCRIBE regions;` 字段匹配 design §1.1 的 10 列
- `SHOW INDEX FROM regions;` 含 3 个 index（PRIMARY + uk_region_id + idx_list + idx_region_type = 4 个 index name，若 PRIMARY 单独计则共 4）
- 文件注释 3-5 行说明 regions 表用途 + 与 Server Zone 对应关系（对齐 014 先例）
- 二次 `go run ./cmd/admin migrate`（或等价入口）幂等（IF NOT EXISTS 保护）

---

## T2：errcode 47xxx 段 + util/const.go DictGroupRegionType （R8, R10）  `[x]` 完成 2026-04-20

**关联需求**：R8, R10 + 间接支撑 R2-R7
**文件**：`backend/internal/errcode/codes.go` + `backend/internal/util/const.go`

**做什么**：
1. codes.go 追加分节注释 `// ========== regions (470xx) ==========`，按 design §1.3 定义 11 个常量 + messages map 中文文案
2. const.go 追加：
   ```go
   // ========== 字典组：region_type ==========
   DictGroupRegionType = "region_type"
   RegionTypeWilderness = "wilderness"
   RegionTypeTown       = "town"
   ```

**做完了是什么样**：
- `go build ./...` 通过
- `grep "47001" backend/internal/errcode/codes.go` 命中 2 次（常量定义 + messages map）
- `errcode.Msg(errcode.ErrRegionIDExists)` 返回中文"区域标识已存在"
- `grep "DictGroupRegionType" backend/internal/util/const.go` 命中 1 次

---

## T3：model/region.go （R1, R9）  `[x]` 完成 2026-04-20

**关联需求**：R1, R9
**文件**：`backend/internal/model/region.go`（新建）

**做什么**：按 design §1.2 落 `Region` / `SpawnEntry` / `SpawnPoint` / `RegionListItem` / `RegionListData` / `RegionDetail` / `RegionExportItem` 结构。`SpawnTable` 用 `json.RawMessage`；`RegionExportItem` 形如 `{Name string "json:\"name\""; Config RegionConfigExport "json:\"config\""}`，`RegionConfigExport` 剥离 id/enabled/version 仅含 region_id/name/region_type/spawn_table。

**做完了是什么样**：
- `go build ./internal/...` 通过
- `json.Marshal(model.RegionExportItem{...})` 输出不含 enabled/version/id key
- `RegionListData.ToListData()` 实现 `*ListData` 转换（对齐 bt_tree 先例）

---

## T4：store/mysql/region.go CRUD + List + ExportAll （R1-R7, R15）  `[x]` 完成 2026-04-20

**关联需求**：R1-R7（支撑层），R15（缓存 invalidate 对接点）
**文件**：`backend/internal/store/mysql/region.go`（新建）

**做什么**：镜像 [store/mysql/bt_tree.go](../../../backend/internal/store/mysql/bt_tree.go) 结构，提供：
- `Create(ctx, r *model.Region) (int64, error)` — UNIQUE 冲突用 `shared.Is1062` 返 `errcode.ErrRegionIDExists`
- `Update(ctx, r *model.Region) error` — WHERE version=? 乐观锁，version +=1；0 rows 返 `errcode.ErrRegionVersionConflict`
- `GetByID(ctx, id) (*Region, error)` / `GetByRegionID(ctx, regionID)` — `errcode.ErrRegionNotFound` on not found
- `SoftDelete(ctx, id, version) error`
- `ToggleEnabled(ctx, id, version, target bool) error` — version +=1 幂等（对齐 bt_tree）
- `List(ctx, filter RegionListFilter) ([]RegionListItem, int64, error)` — keyword 走 `shared.EscapeLike`
- `ExportAll(ctx) ([]Region, error)` — `WHERE enabled=1 AND deleted=0 ORDER BY id ASC`

**做完了是什么样**：
- `go build ./internal/...` 通过
- 手测 MySQL：`INSERT → SELECT → UPDATE version=1 → SELECT version=2`
- List `keyword="%25_test%25"` 不被 LIKE 通配符注入（逐字匹配）
- Export 排除 enabled=0 / deleted=1 行

---

## T5：store/redis/region_cache.go （R15）

**关联需求**：R15
**文件**：`backend/internal/store/redis/region_cache.go`（新建）

**做什么**：镜像 [store/redis/bt_tree_cache.go](../../../backend/internal/store/redis/bt_tree_cache.go)：
- `CacheList(ctx, hash, data *RegionListData) / GetList(ctx, hash) (*RegionListData, bool, error)`
- `CacheDetail(ctx, id, data *RegionDetail) / GetDetail(ctx, id)`
- `InvalidateAll(ctx) error` — 写操作用，DEL list 前缀 + detail 前缀（用预知 key 不用 SCAN）
- `InvalidateDetail(ctx, id)` — 单记录失效
- TTL 900s ± jitter 60s，hash 编码 filter

**做完了是什么样**：
- `go build ./internal/...` 通过
- 本地 Redis：set → get hit → invalidate → get miss
- pipeline 失败返 err（不 swallow）

---

## T6：service/region.go — CRUD 骨架 （R2-R7）

**关联需求**：R2, R3, R4, R5, R6, R7
**文件**：`backend/internal/service/region.go`（新建）

**做什么**：`RegionService` struct 持有 `store *mysql.RegionStore` + `cache *redis.RegionCache` + `templateService *TemplateService`（构造函数注入）。实现 CRUD：
- `Create` / `Update` / `SoftDelete` / `ToggleEnabled` / `GetDetail` / `List`
- `Update` 首先 check 记录 enabled → `ErrRegionEditNotDisabled`
- `SoftDelete` 首先 check 记录 enabled → `ErrRegionDeleteNotDisabled`
- `Create` / `Update` **暂不 hook** validateSpawnTable（T7 接入），先过 JSON 基础格式校验 `json.Valid(spawnTable)`
- 所有写操作成功后调 `cache.InvalidateAll`

**做完了是什么样**：
- `go build ./internal/...` 通过
- 本 task 用临时 curl：create → get detail → list 命中 → toggle enable → update (43xxx 拒) → toggle disable → update OK → delete (43xxx 拒) → toggle disable → delete OK
- TODO 注释标记 T7 接入点：`// TODO(T7): validateSpawnTable hook`

---

## T7：service/region.go — validateSpawnTable + Create/Update hook （R8）

**关联需求**：R8
**文件**：`backend/internal/service/region.go`（同 T6 文件，扩展）

**做什么**：
1. 新增方法 `(s *RegionService) validateSpawnTable(ctx, raw json.RawMessage) error`：
   - `json.Unmarshal` 到 `[]SpawnEntry`，失败返 `ErrRegionSpawnEntryInvalid`
   - 遍历断言 `TemplateRef != ""` / `Count >= 1` / `len(SpawnPoints) >= Count` / `WanderRadius >= 0` / `RespawnSeconds >= 0`，任一违反 → `ErrRegionSpawnEntryInvalid` + 详细 err message
   - 收集所有 `TemplateRef` → 调 `s.templateService.CheckEnabledByNames(ctx, names)` 取 notOK
   - notOK 非空：调 `s.templateService.GetByNames(ctx, notOK)` 分类"不存在" vs "存在但 disabled"
   - 按分类返 `ErrRegionTemplateRefNotFound` / `ErrRegionTemplateRefDisabled`（两类混合时按先遇到的分类返）
2. Create/Update 起始调 `validateSpawnTable`，删 T6 遗留的 TODO

**做完了是什么样**：
- `go build ./internal/...` 通过
- 单测用例（T11 覆盖）已就绪占位
- 手测：create 引 villager_guard（enabled=1）→ 200；引 thief_never_exist → 47006；create 先 disable villager_guard 后 create → 47007

---

## T8：service/region.go — Export 4 纯方法 （R9, R10）

**关联需求**：R9, R10
**文件**：`backend/internal/service/region.go`（同文件，扩展）

**做什么**：对齐 [npc_service.go](../../../backend/internal/service/npc_service.go) export-ref-validation 5 步编排的 4 个纯方法：
1. `ExportRows(ctx) ([]Region, error)` — 直查 store.ExportAll
2. `CollectExportRefs(rows) (*RegionExportRefs, error)` — 纯函数，扫 rows + unmarshal spawn_table，构建 `TemplateIndex map[string][]string`（template_ref → region_id[]）；空 spawn_table 不入索引
3. `BuildExportDanglingError(refs, notOK) *errcode.ExportDanglingRefError` — 纯函数，遍历 notOK × 反查 index 拼 details（`ref_type="npc_template_ref"`, `reason="missing_or_disabled"`）；全部正常返 nil
4. `AssembleExportItems(rows) ([]RegionExportItem, error)` — 纯函数，装配 `{name: region_id, config: {...}}`

注：`ExportDanglingRefError` 既有类型（`backend/internal/errcode/export_error.go`）复用，`Details` 字段泛用 `[]model.NPCExportDanglingRef` 已经满足（字段名 `NPCName` 这里装填 region_id，`RefType` 填 `"npc_template_ref"`）。**不新建 RegionExportDanglingRef**，对齐既有 pattern + 避免重复结构。

**做完了是什么样**：
- `go build ./internal/...` 通过
- 纯方法可单测（输入 []Region → 确定性输出）
- 既有 ExportAll 方法（如有）删除或薄 shim

---

## T9：handler/region.go CRUD 7 端点 + router + setup （R2-R7, R12）

**关联需求**：R2, R3, R4, R5, R6, R7（HTTP 层）
**文件**：`backend/internal/handler/region.go`（新建）+ `backend/internal/router/region.go`（新建或追加到主 router）+ `backend/internal/setup/setup.go`（wiring）

**做什么**：
1. `RegionHandler` 走 `WrapCtx[ReqType, RespType]` 泛型包装（对齐 bt_tree.go handler）
2. 7 端点：POST /api/v1/regions/{create, update, toggle-enabled, list, detail, delete}
3. router 注册：`rg := r.Group("/api/v1/regions"); ...`
4. setup.go 构造注入：`regionStore → regionCache → regionService(templateService 依赖)  → regionHandler`

**做完了是什么样**：
- `go build ./...` 通过
- `curl -XPOST /api/v1/regions/create -d '{"region_id":"test","display_name":"测试",...}'` 返 200 + data 含 id + version=1
- `curl -XPOST /api/v1/regions/list` 返分页结构
- 所有错误码经由 handler 正确映射到 HTTP code（4xx 语义错 / 5xx 系统错）

---

## T10：handler/export.go +Regions() 5 步编排 （R9, R10, R17）

**关联需求**：R9, R10, R17
**文件**：`backend/internal/handler/export.go`（扩展）

**做什么**：
1. `ExportHandler` struct 追加 `regionService *service.RegionService` 字段 + 构造函数参数
2. 新增方法 `Regions(c *gin.Context)` — 对齐 `NPCTemplates` 5 步编排（design §1.6 表）
3. 登记路由：`GET /api/configs/regions`
4. setup.go 补 regionService 注入

**做完了是什么样**：
- `go build ./...` 通过
- curl /api/configs/regions 正常路径 200 + items；空 regions 表 → items=[] 200（R17 覆盖）
- 构造悬空：curl → 500 + `.code==47011` + `.details[].ref_type=="npc_template_ref"` + slog ERROR 一条

---

## T11：service/region_test.go 10 用例 （R16）

**关联需求**：R16 + 覆盖 R8/R9/R10 逻辑单元
**文件**：`backend/internal/service/region_test.go`（新建）

**做什么**：table-driven 10 用例，按 design §8.1 表。TemplateService 用接口 stub（service 构造函数接受 interface）不引入 mock 框架。

**做完了是什么样**：
- `go test ./internal/service/... -run TestRegion` 全绿 10 个用例 PASS
- 不引入 mockery/gomock
- 覆盖 validate 4 + collect 2 + build 2 + assemble 2

---

## T12：seed region_type 字典 + village_outskirts fixture （R11）

**关联需求**：R11
**文件**：`backend/cmd/seed/region_seed.go`（新建）+ `backend/cmd/seed/main.go`（调用）+ `backend/cmd/seed/dict_seed.go` 或 同源（字典段）

**做什么**：
1. `seedRegionTypeDict(ctx, db)` — INSERT IGNORE 写 `dict_entries` 的 region_type 组 2 枚举（wilderness=野外, town=城镇）
2. `seedRegions(ctx, db)` — INSERT IGNORE 写 `village_outskirts`：
   - region_type=wilderness, enabled=1, version=1
   - spawn_table JSON: `[{"template_ref":"villager_guard","count":2,"spawn_points":[{"x":10,"z":20},{"x":15,"z":20}],"wander_radius":5,"respawn_seconds":60}]`
3. main.go 按序：`seedDicts → ... → seedFields → seedTemplates → seedNPCs → seedRegionTypeDict → seedRegions`（region 必在 templates 已写之后）

**做完了是什么样**：
- `go run ./cmd/seed` 首次跑：dict 新增 2 + region 新增 1
- 二次跑：全 "[跳过]"
- curl /api/configs/regions 返 200 + 1 items
- `make verify-seed` 通过（如既有 verify-seed 脚本不校验 region 先放宽；或本 task 追加断言到 verify-seed.sh）

---

## T13：frontend/src/api/regions.ts （R12, R13）

**关联需求**：R12, R13（API 层支撑）
**文件**：`frontend/src/api/regions.ts`（新建）

**做什么**：镜像 `btTrees.ts`，9 个方法：createRegion / updateRegion / toggleRegionEnabled / listRegions / getRegionDetail / deleteRegion + getRegionTypeOptions（调 `/api/v1/dictionaries/region_type`）+ 相关 TypeScript interface（Region / SpawnEntry / SpawnPoint / RegionListItem / RegionDetail）。

**做完了是什么样**：
- `npm run type-check` 通过
- interface 字段与后端 model 一致（含 respawn_seconds）

---

## T14：RegionList.vue + 路由 + 菜单 （R12）

**关联需求**：R12
**文件**：`frontend/src/views/RegionList.vue`（新建）+ `frontend/src/router/index.ts`（追加 `/regions`）+ layout 菜单组件（新增"区域管理"菜单项）

**做什么**：镜像 `BtTreeList.vue` 结构 — 分页表格 + region_type 筛选 + enabled Switch + 关键字搜索 + 行操作（详情 / 编辑 / 启停 / 删除）。toggle 调 API 带 version 乐观锁。

**做完了是什么样**：
- 浏览器访问 `/regions` 显示已 seed 的 village_outskirts
- region_type 下拉筛选生效（wilderness 返 1，town 返 0）
- 启停切换 API 调用 + 表格刷新
- 删除启用中记录弹 error toast（47008 中文）

---

## T15：RegionForm.vue 含嵌套 array 编辑器 （R13, R14）

**关联需求**：R13, R14
**文件**：`frontend/src/views/RegionForm.vue`（新建）

**做什么**：
1. 基础字段区：region_id（创建可编辑，编辑禁用）/ display_name / region_type（Select）
2. SpawnTable 编辑器（**自定义组件或 inline template**）：
   - 外层 `el-card` 数组 v-for，每张卡片头含"删除此 entry"按钮 + 底部"添加 entry"按钮
   - 卡片内字段：
     - `template_ref`：复用 NPC template 选择器组件（bb-key-runtime-registry T13-T16 第 3 组范式）
     - `count`：`el-input-number` min=1
     - `wander_radius`：`el-input-number` min=0 step=0.1 + 后缀"米"
     - `respawn_seconds`：`el-input-number` min=0 step=1 + Form Item `help` 属性 **"Server v3+ 生效，当前仅保存不调度"**
     - `spawn_points`：二级嵌套数组（`el-table` 或 flat div 列表），每行两栏 x/z（`el-input-number` step=0.1）+ 行增删
3. 提交前前端校验：`len(spawn_points) >= count`，否则红点阻塞提交
4. 保存：JSON 化 `spawn_table` → POST create/update；409（47010）弹"版本冲突，请刷新"

**做完了是什么样**：
- 浏览器 /regions/create：动态增删 2 个 entry，每个 2 个 spawn_points，template_ref 从选择器选 villager_guard，保存后跳回列表看到新记录
- 编辑已有 region：表单预填 + version 字段存在 + 改 respawn_seconds 保存成功
- spawn_points 数 < count 时提交按钮 disabled + 错误文案
- respawn_seconds 字段 help-text 可见

---

## T16：后端 e2e 手动验证 （R16 的 e2e 验证）

**关联需求**：贯穿全部 R
**文件**：0 改动，结果贴到 PR description

**做什么**：按 design.md §8.2 表跑 7 个场景（正常/悬空/隔离/CRUD/乐观锁/seed 幂等等），jq 断言关键字段。

**做完了是什么样**：
- 7 步 curl 输出全贴
- 步骤 2 响应 jq 校验 `.code==47011 and (.details|length)>0`
- 步骤 3 其他 export 端点全 200
- 步骤 7 seed 二次跑 "跳过" 文本

---

## T17：前端手测（R12-R14 e2e）

**关联需求**：R12, R13, R14
**文件**：0 改动，结果贴到 PR description

**做什么**：按 design.md §8.2 #5-6 前端场景手测：
1. 新建 region 含 2 spawn_entry（一引启用 template 一引 disable template）→ 后端返 47006/47007，前端对应 entry 红点显示
2. 两 tab 同编辑 region，第二 tab 提交返 47010 → 弹"版本冲突"
3. respawn_seconds help-text 截图入 PR

**做完了是什么样**：
- 3 个前端场景皆按预期
- 截图入 PR
- `/verify` 通过

---

## 不做的（显式确认）

- ❌ sleep/wake 运行时 API / WS handler
- ❌ Active 字段暴露
- ❌ boundary / polygon / weather / y 坐标字段
- ❌ dungeon / safezone 等 region_type 扩展
- ❌ Server configs/regions/meadow.json 迁移
- ❌ respawn 运行时调度逻辑
- ❌ spawn_table 外键子表
- ❌ 导入 / 批量新建
- ❌ 跨 region NPC 迁移
- ❌ SchemaForm 核心改动以支持嵌套 array
- ❌ 自动化 e2e 进 CI

## 经验沉淀

spec-execute 收尾后若发现新模式/陷阱，追加到：
- admin 专属：`docs/development/admin/dev-rules.md`
- 跨项目：`docs/development/standards/dev-rules/*.md`
- 红线：`docs/development/standards/red-lines/*.md` 或 `docs/development/admin/red-lines.md`

---

**→ 停下，等用户审批**

审批通过后：
1. `git checkout -b feature/regions-module`（从 main 拉）
2. T1 判断密度评估：T1 是明确的 migration DDL，属"轻执行场景"（CRUD 级精确），**不建议** `/backend-design-audit`，直接 `/spec-execute T1 regions-module` 即可
3. 后续 T2/T3 同为轻执行；T6/T7 属中等判断密度（service 层与 TemplateService 交互 + 错误码分类），可视情况 audit；T15 RegionForm 嵌套 array 编辑器属前端重判断（UI 设计 + 状态管理），建议 `/backend-design-audit T15`（虽名叫 backend 但对前端重判断也适用）— 本意是"开工前沉淀决策"，不拘前后端
