# regions-module — 设计方案

## 0. 术语与正交关系

| 术语 | 含义 | 作用域 |
|------|------|--------|
| region（Admin 侧） | 配置侧区域定义 | ADMIN 数据库 |
| Zone（Server 侧） | 运行时区域实例 | Server ZoneManager |
| region_id | 跨边界业务键 | 两侧一致，snake_case |
| `enabled`（Admin） | 配置激活位 | 仅 Admin 导出过滤用；禁用 region 不出现在 `/api/configs/regions` items |
| `Active`（Server） | 运行时激活位 | Server 启动期统一 `true`，sleep/wake 走未来 WS handler，**本 spec 不碰** |

两层正交：Admin `enabled=false` → 配置静默；Server `Active=false` → 运行时静默。本 spec 只管 `enabled`。

---

## 1. 方案描述

### 1.1 数据模型（MySQL）

**migration 016_create_regions.sql**：

```sql
CREATE TABLE IF NOT EXISTS regions (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    region_id       VARCHAR(64)  NOT NULL,              -- 业务键，snake_case，^[a-z][a-z0-9_]*$，创建后不可变
    display_name    VARCHAR(128) NOT NULL,              -- 中文名（列表搜索用）
    region_type     VARCHAR(32)  NOT NULL,              -- wilderness / town（dict 枚举）
    spawn_table     JSON         NOT NULL,              -- SpawnEntry[] JSON，可为空数组 '[]'

    enabled         TINYINT(1)   NOT NULL DEFAULT 0,    -- 启用位（创建默认 0）
    version         INT          NOT NULL DEFAULT 1,    -- 乐观锁
    created_at      DATETIME     NOT NULL,
    updated_at      DATETIME     NOT NULL,
    deleted         TINYINT(1)   NOT NULL DEFAULT 0,

    UNIQUE KEY uk_region_id (region_id),
    INDEX idx_list (deleted, enabled, id DESC),
    INDEX idx_region_type (region_type, deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**`down.sql`**：`DROP TABLE IF EXISTS regions;`

### 1.2 Go model

```go
// backend/internal/model/region.go

type Region struct {
    ID          int64           `json:"id"           db:"id"`
    RegionID    string          `json:"region_id"    db:"region_id"`
    DisplayName string          `json:"display_name" db:"display_name"`
    RegionType  string          `json:"region_type"  db:"region_type"`
    SpawnTable  json.RawMessage `json:"spawn_table"  db:"spawn_table"`
    Enabled     bool            `json:"enabled"      db:"enabled"`
    Version     int             `json:"version"      db:"version"`
    CreatedAt   time.Time       `json:"created_at"   db:"created_at"`
    UpdatedAt   time.Time       `json:"updated_at"   db:"updated_at"`
    Deleted     bool            `json:"-"            db:"deleted"`
}

// SpawnEntry — 解析 spawn_table JSON 用，service 层校验引用完整性
type SpawnEntry struct {
    TemplateRef    string       `json:"template_ref"`
    Count          int          `json:"count"`
    SpawnPoints    []SpawnPoint `json:"spawn_points"`
    WanderRadius   float64      `json:"wander_radius"`
    RespawnSeconds float64      `json:"respawn_seconds"`
}

type SpawnPoint struct {
    X float64 `json:"x"`
    Z float64 `json:"z"`
}

// RegionListItem / RegionListData / RegionDetail / RegionExportItem 按 bt_tree 先例同构
```

**关键选择**：`SpawnTable json.RawMessage` — 存储层直透 JSON，service 层按需解码（对齐 fsm_configs.ConfigJSON 先例，避免三层转译丢类型）。

### 1.3 错误码（47xxx 新段）

> **修订**：requirements.md "改动范围"列写的 45017 实为笔误。regions 是独立配置类别，应开新段；`45000-45016` 保留 NPC 专属。本 spec 用 **47xxx** 段。

```go
// backend/internal/errcode/codes.go

ErrRegionIDExists            = 47001 // region_id 已存在（含软删除）
ErrRegionIDInvalid           = 47002 // region_id 格式非法（^[a-z][a-z0-9_]*$）
ErrRegionNotFound            = 47003 // region 不存在
ErrRegionTypeInvalid         = 47004 // region_type 不在 dict 枚举内
ErrRegionSpawnEntryInvalid   = 47005 // spawn_entry 自洽性校验失败（count<1 / spawn_points<count 等）
ErrRegionTemplateRefNotFound = 47006 // spawn_entry.template_ref 指向不存在的 template
ErrRegionTemplateRefDisabled = 47007 // spawn_entry.template_ref 指向未启用 template
ErrRegionDeleteNotDisabled   = 47008 // 删除前必须先停用
ErrRegionEditNotDisabled     = 47009 // 编辑前必须先停用
ErrRegionVersionConflict     = 47010 // 版本冲突（乐观锁）
ErrRegionExportDanglingRef   = 47011 // 导出期发现悬空 template 引用
```

**messages map 同步追加中文**。

### 1.4 Store 层

`backend/internal/store/mysql/region.go`（对齐 bt_tree.go 结构）：
- `Create / Update / GetByID / GetByRegionID / SoftDelete / ToggleEnabled`
- `List(ctx, filter RegionListFilter) ([]RegionListItem, int64, error)` — 支持 region_type/enabled/keyword 筛选
- `ExportAll(ctx) ([]Region, error)` — WHERE enabled=1 AND deleted=0
- `GetByRegionIDs(ctx, ids []string) (map[string]bool, error)` — ⚠️ 不是本 spec 必需（regions 不被其他资源反查）

**Redis cache** `backend/internal/store/redis/region_cache.go` — list + detail 双层缓存，写操作失效（对齐 bt_tree_cache.go）。

### 1.5 Service 层

`backend/internal/service/region.go`：
- `Create / Update / SoftDelete / ToggleEnabled / GetDetail / List / ExportAll`
- **关键校验方法** `validateSpawnTable(ctx, rawJSON) error`：
  1. `json.Unmarshal` 到 `[]SpawnEntry`（格式错误 → ErrRegionSpawnEntryInvalid）；空数组合法
  2. 遍历：`TemplateRef` 非空 / `Count >= 1` / `len(SpawnPoints) >= Count` / `WanderRadius >= 0` / `RespawnSeconds >= 0`
  3. 收集去重后的 `TemplateRef`，调 **`NpcService.LookupByNames(ctx, names) → map[name]enabled`**（T7 新增 helper）
  4. 遍历分类：
     - 不在 map 中 → `ErrRegionTemplateRefNotFound`
     - 在 map 中但 enabled=false → `ErrRegionTemplateRefDisabled`
     - 两类同时发生时按"不存在"优先返第一类（未来如需 details 数组再开 spec）
- `Create / Update` 均前置调 `validateRegionType` + `validateSpawnTable`
- `Update` 首先校验 `enabled=true` → 返 `ErrRegionEditNotDisabled`（对齐 43010 语义）
- `SoftDelete` 首先校验 `enabled=true` → 返 `ErrRegionDeleteNotDisabled`

**T7 修订**：原设计写 `TemplateService.CheckEnabledByNames`，经 T7 实施核对：Server 视角的 "NPC template"（`template_ref`）对应 ADMIN `/api/configs/npc_templates` 端点即 ADMIN npcs 表（不是 ADMIN templates 表）。故引用校验走 `NpcService.LookupByNames`，依赖注入从 `TemplateService` 改为 `NpcService`。

### 1.6 Handler 层

`backend/internal/handler/region.go` — 走既有 `WrapCtx` 泛型包装（对齐 bt_tree.go handler）。CRUD 7 个端点：create / update / toggle-enabled / list / detail / delete。

`backend/internal/handler/export.go` 追加 `Regions(c *gin.Context)` — 5 步编排（对齐 NPCTemplates）：

| Step | 调用 | 失败处理 |
|------|------|---------|
| 1 | `regionService.ExportRows(ctx)` | 通用 500 |
| - | `len(rows)==0` 短路 | 200 + `{"items":[]}` |
| 2 | `regionService.CollectExportRefs(rows)` → 构建 `templateRef → []RegionName` 反查索引 | 通用 500 |
| 3 | `templateService.CheckEnabledByNames(ctx, keys)` | 通用 500 |
| 4 | `regionService.BuildExportDanglingError(refs, notOK)` | 非 nil → 500 + 47011 + details |
| 5 | `regionService.AssembleExportItems(rows)` | 通用 500 |
| - | success | 200 + `{"items": items}` |

导出 shape：
```json
{"items": [{
  "name": "village_outskirts",
  "config": {
    "region_id": "village_outskirts",
    "name": "村外野地",
    "region_type": "wilderness",
    "spawn_table": [{"template_ref": "villager_guard", "count": 2, "spawn_points": [{"x":10,"z":20},{"x":15,"z":20}], "wander_radius": 5, "respawn_seconds": 60}]
  }
}]}
```

**"name" 字段**：导出 envelope 的 key 用 `region_id` 值（对齐既有 `event_types` / `fsm_configs` / `bt_trees` 导出封装的 "name" = 资源业务键，而不是 display_name）。这样 Server HTTPSource 反序列化 `Zone` 直命中 `region_id` tag。

### 1.7 Seed

`backend/cmd/seed/region_seed.go`：
- `seedRegionTypeDict`：写 `dict_entries` 表 `DictGroupRegionType` 组下 2 枚举（wilderness=野外 / town=城镇），INSERT IGNORE 幂等
- `seedRegions`：写 `village_outskirts`（region_type=wilderness, enabled=1, spawn_table=villager_guard × 2）

`backend/cmd/seed/main.go` 按序追加调用：字典 → regions（在 templates/NPC 之后，因需校验 template 存在性）。

### 1.8 前端

**`frontend/src/api/regions.ts`** — 9 个方法（对齐 btTrees.ts）：create / update / toggleEnabled / list / detail / deleteRegion + regionTypeOptions（从字典拉）

**`RegionList.vue`**：
- 分页表格（columns: region_id / display_name / region_type / enabled / created_at）
- 筛选：region_type Select + enabled Switch + region_id/display_name 关键字
- 行操作：详情 / 编辑 / 启停 / 删除
- 样式对齐 BtTreeList.vue

**`RegionForm.vue`**（本 spec 最大前端重点）：
- 基础字段：region_id（仅创建时可编辑）/ display_name / region_type（字典选择器）
- `spawn_table` 嵌套数组编辑器（**不走 SchemaForm**，自定义组件）：
  - 外层数组：增/删 SpawnEntry 卡片
  - 每张卡片内：
    - `template_ref`：NPC template 选择器（**复用** bb-key-runtime-registry T13-T16 确立的第 3 组选择器范式，过滤 enabled=1）
    - `count`：number input
    - `wander_radius`：number input + 后缀"米"
    - `respawn_seconds`：number input + help-text **"Server v3+ 生效，当前仅保存不调度"**
    - `spawn_points`：二级嵌套数组编辑器，每点两栏 x/z（number + step=0.1）+ 行增删
- 保存 → 拼 JSON → 调 create/update API；提交前前端预校验 `len(spawn_points) >= count`（红点提示）
- 受乐观锁保护：编辑时带 `version`，409 时弹窗提示刷新

**路由**：`/regions` 列表 + `/regions/create` + `/regions/:id/edit`。**菜单**：主侧栏追加"区域管理"。

---

## 2. 方案对比

### 2.1 spawn_table 存储：JSON 列 vs 子表

| | JSON 列（选） | 子表 `region_spawn_entries` |
|---|---|---|
| 读性能 | 单 SELECT | region + entries 两次 SELECT 或 JOIN |
| 写原子性 | 单行 UPDATE | 事务 + DELETE-ALL-INSERT 或增量同步 |
| 结构校验 | service 层 Unmarshal 校验 | DDL 强制 schema |
| 对齐先例 | NPC `bt_refs` / fsm `config_json` | 无先例 |
| 复杂查询 | 无需跨 entry 查询 | 若未来要"查所有引用 villager_guard 的 region" 方便 |

**选 JSON 列**。理由：
- 本 spec 无跨 entry 查询需求（引用完整性靠 service 层遍历，规模 region×entry 都是两位数）
- 对齐既有 JSON 列先例，学习成本 0
- Server 侧 Unmarshal 直收 `spawn_table: []SpawnEntry`，JSON 透传 zero-copy

**反悔成本**：未来若需加跨 region 反查索引，开独立 spec 做 `spawn_table_refs` 物化表，不改 regions 表本身。

### 2.2 `name` 导出 envelope vs 直接返 region_id 作 key

| | 选 | 不选 |
|---|---|---|
| A：`{name, config:{...}}` 封装（选） | 对齐既有 4 个端点 envelope | — |
| B：`{region_id: {...}}` map | Server Unmarshal 要改 | 破坏契约一致性 |

**选 A**。Server CC 已确认 shape `{items:[{name, config}]}`，无需辩论。

### 2.3 悬空引用：导出期 500 vs 过滤跳过

| | 500 + 47011（选） | 过滤跳过 + warning log |
|---|---|---|
| 对齐 NPCTemplates 既有 pattern | ✅ | ❌ 两个端点行为分歧 |
| 让运维看见问题 | ✅ 立刻硬失败 | ❌ 可能静默跑坏 |
| Server 侧影响 | HTTPSource fail 阻塞启动（预期行为）| zones.loaded 数字静默少 |
| 联调反馈 | Server CC 明确支持 500（"现有契约就是 ADMIN 校验通过才落库"）| — |

**选 500 + 47011**。硬失败是正确信号。

---

## 3. 红线检查

逐条对照（文件存在性已确认：`ls docs/development/standards/red-lines/` → general/go/mysql/redis/cache/frontend 全部存在，admin/red-lines.md 存在）。

### 3.1 general.md
- ✅ 无静默降级：悬空引用 500 硬失败、JSON 格式错误明确错误码、seed 失败返 err 而非 warn
- ✅ 无过度设计：不引入子表 / 不抽象 registry / 不加 feature flag
- ✅ 测试策略：service 纯方法单测 + e2e curl（见 §8）

### 3.2 go.md
- ✅ 资源释放：store 层 rows/close 对齐既有 bt_tree.go pattern
- ✅ JSON：`spawn_table` 走 `json.RawMessage` 透传 + service 层 Unmarshal 校验，不做三层转译
- ✅ 错误处理：service 层 `fmt.Errorf("...: %w", err)` wrap，handler 层 `errors.Is` 判 sentinel
- ✅ 字符串：`region_id` 校验用 regexp，不用裸 strings 判断

### 3.3 mysql.md
- ✅ 事务一致性：create/update 单行操作无需事务；toggle-enabled 和 version 同步用乐观锁 `WHERE version=?`
- ✅ LIKE 注入：list 关键字查询用 `EscapeLike`（复用 store/mysql/shared/sqlutil.go）
- ✅ UNIQUE 约束：`uk_region_id` 捕获冲突，store 层检 `Is1062` 返 ErrRegionIDExists
- ✅ 索引：`idx_list (deleted, enabled, id DESC)` 覆盖分页；`idx_region_type` 覆盖筛选

### 3.4 redis.md
- ✅ 无 SCAN：cache 用明确 key（`region:list:<hash>` / `region:detail:<id>`），TTL 15min 对齐 bt_tree
- ✅ DEL 错误检查：写操作后失效 key 用 MULTI 或 pipeline，err 检查不跳过

### 3.5 cache.md
- ✅ Cache-Aside 模式：读先 Redis 后 MySQL，写先 MySQL 后 invalidate Redis
- ✅ 穿透防护：list/detail miss 返回空结果不缓存空值（对齐 bt_tree）；或缓存空值 1min（TBD，沿用既有选择）
- ✅ 雪崩防护：TTL 15min ± 随机抖动 60s（对齐既有）

### 3.6 frontend.md
- ✅ 无数据源污染：region_type 选项从 `/api/v1/dictionaries/region_type` 拉，非硬编码
- ✅ 无效输入：spawn_points 数组 x/z 必填 + number 类型强制；count<1 前端 disable 提交
- ✅ URL 编码：详情 `/regions/:id/edit` id 是 int64 无编码问题

### 3.7 admin/red-lines.md
- ✅ 数据格式：导出 envelope 对齐既有；无裸字符串（region_type 字典 / 错误码常量）
- ✅ 引用完整性：R13 pattern 在 create/update/export 三处校验
- ✅ 硬编码：region_type 枚举走字典表，不写入代码 switch
- ✅ 表单友好：respawn_seconds 带 help-text，spawn_points x/z 分两栏（不逼策划写 JSON）
- ✅ 所有 4xx/5xx 响应含 code 字段（对齐 #14.2）

**全部红线通过**。

---

## 4. 扩展性影响

### 4.1 扩展轴 1（新增配置类型）
- ✅ 本 spec 即走此轴的教科书实现
- ✅ 既有模块零侵入（errcode / util/const / seed main / dict seed / export handler / router 均 additive）
- ✅ 后续加 `dungeon` / `safezone` 等枚举只需在字典表 INSERT，不改代码

### 4.2 扩展轴 2（新增表单字段）
- ⚠️ SchemaForm 核心不动，但 regions 嵌套 array 走 RegionForm 自定义块（对齐 BtTreeForm / FsmConfigForm 先例）
- 💡 未来若 SchemaForm 支持嵌套 array（另开 spec），RegionForm 可平迁；本 spec 不阻塞
- ✅ SpawnEntry 自身加字段（如 `priority`）只改 RegionForm 单文件

---

## 5. 依赖方向

```
handler/region ──┬──▶ service/region ──▶ store/mysql/region + store/redis/region_cache
                 │                    └──▶ service/template (CheckEnabledByNames + GetByNames)
handler/export ──┤
                 ▼
               service/region (ExportRows/CollectExportRefs/BuildExportDanglingError/AssembleExportItems)

service/region ──▶ model/region
              └──▶ errcode
              └──▶ util/const (DictGroupRegionType)

migrations/016 (独立)
seed/region_seed (依赖 template seed 先行)
frontend api ──▶ handler HTTP
frontend views ──▶ api + template 选择器（既有）
```

**单向向下**：handler → service → store；service 间无环（region → template 单向，template 不反引 region）。

---

## 6. 陷阱检查

### 6.1 go.md dev-rules
- ⚠️ `json.RawMessage` 存储 null 时写入 DB 的是 `null` 字符串而非 SQL NULL — Migration `NOT NULL` + 默认值 `'[]'` + service 层拒绝传 `null` spawn_table
- ⚠️ `float64` 精度：`wander_radius=0.1+0.2 != 0.3` — 本 spec 不做浮点比较，仅存储 + 传递，无陷阱
- ⚠️ `time.Now()` 写 UTC vs Local — 对齐既有 store 层用 `time.Now()`（MySQL DATETIME 无时区）

### 6.2 mysql.md dev-rules
- ⚠️ JSON 列索引：MySQL 8.0+ 支持 generated column + index，本 spec 不建 JSON 索引（不按 spawn_table 内字段查询）
- ⚠️ 软删除 + UNIQUE：`uk_region_id` 不含 deleted，软删后 region_id 占位不可复用（对齐 bt_tree 选择，策划感知"软删永久"）
- ⚠️ `DATETIME` 精度：秒级即可，不用 DATETIME(3)

### 6.3 redis.md dev-rules
- ⚠️ 连接泄漏：go-redis 自带连接池，无需手动 close
- ⚠️ pipeline 错误：invalidate list + detail 两 key 用 `pipeline.Del`，err != nil 时 log + 返回（不阻塞主流程，对齐既有 cache 行为）

### 6.4 cache.md dev-rules
- ⚠️ 缓存一致性：写操作 commit 后才 invalidate，避免"MySQL 写失败但 Redis 已失效"倒挂
- ⚠️ list 缓存 key 含所有筛选维度 hash（region_type + enabled + keyword + page + page_size）

### 6.5 frontend.md dev-rules
- ⚠️ Element Plus `el-input-number` step=0.1 浮点显示：前端不做四舍五入，原样传后端
- ⚠️ 动态数组 `v-for` key：SpawnEntry 用 `:key="'entry-'+index"` 避免索引抖动
- ⚠️ 提交前 JSON 化 `spawn_table` 用 `JSON.stringify`，后端用 `json.RawMessage` 收

---

## 7. 配置变更

**无新增独立 JSON 配置文件**。
- regions 数据全走 MySQL
- region_type 字典走 `dict_entries` 表（既有 table）
- Server 侧的 `configs/regions/meadow.json` 是 Server 本地 fixture，**ADMIN 不动**（`feedback_server_local_fixtures_protected.md` 红线）

---

## 8. 测试策略

### 8.1 单元测试

**service/region_test.go**（纯方法优先 — 对齐 export-ref-validation T8 先例）：

| 用例 | 测对象 | 期望 |
|------|--------|------|
| TestValidateSpawnTable_Empty | validateSpawnTable | `[]` 合法 |
| TestValidateSpawnTable_NegativeCount | 同 | count=0 / 负数 → ErrRegionSpawnEntryInvalid |
| TestValidateSpawnTable_PointsLessThanCount | 同 | spawn_points=1 count=2 → ErrRegionSpawnEntryInvalid |
| TestValidateSpawnTable_BadJSON | 同 | 非数组 JSON → ErrRegionSpawnEntryInvalid |
| TestCollectExportRefs_Empty | CollectExportRefs | rows=[] → map 非 nil 空 |
| TestCollectExportRefs_Multi | 同 | 2 region 各引 1 template → 反查索引正确聚合 |
| TestBuildExportDanglingError_AllValid | BuildExportDanglingError | notOK=[] → nil |
| TestBuildExportDanglingError_SomeMissing | 同 | notOK=[villager_guard] → details 含所有引用该 template 的 region_id |
| TestAssembleExportItems_Empty | AssembleExportItems | rows=[] → items=[] 非 nil |
| TestAssembleExportItems_OneRow | 同 | 单行装配 shape 符合契约 |

**覆盖目标**：service 层纯方法 100%，store 层跟既有走 integration（可延后）。

### 8.2 e2e 手动验证（按 design §1.6 shape）

| # | 场景 | 期望 |
|---|------|------|
| 1 | 正常路径：seed 后 curl /api/configs/regions | 200 + items 含 village_outskirts |
| 2 | 悬空路径：禁用 villager_guard template 后 curl | 500 + `code=47011` + `details[0].ref_type=npc_template_ref` + slog ERROR 一条 |
| 3 | 隔离性：步骤 2 状态下 curl /api/configs/{event_types,fsm_configs,bt_trees,npc_templates} | 4 端点全 200（`npc_templates` 端点校验自己 fsm/bt，不涉 template 引用——如果涉及也在 45016 路径) |
| 4 | CRUD 全路径：create → toggle enable → update (应 43010) → disable → update → enable → delete (应 47008) → disable → delete | 每步 HTTP code 符合预期 |
| 5 | 前端手测：新建 region 含 2 spawn_entry（一引启用 template 一引禁用 template）| 后端返 47006/47007，前端红点显示在对应 entry |
| 6 | 乐观锁：两 tab 同编辑 region，第二提交返 47010 | 弹窗"版本冲突" |
| 7 | seed 幂等：`go run ./cmd/seed` 连跑两次 | 第二次 stdout "跳过" 文本 |

### 8.3 跨项目 e2e（Server CC 配合）

Server CC 本期承诺：
- HTTPSource.LoadAllRegionConfigs 接入后跑 `docker compose up --build` + `NPC_ADMIN_API=http://localhost:9821`
- 断言 villager_guard × 2 spawn 时 `PositionComponent.ZoneID="village_outskirts"`
- 跨 zone 事件过滤验证（触发 meadow zone 内部事件，确认 `village_outskirts` 的 guard 不响应）

---

**→ 停下，等用户审批后进入 Phase 3 任务拆解**
