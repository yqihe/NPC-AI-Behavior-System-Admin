# external-contract-admin-shape-alignment — 任务拆解

按 requirements.md 的 R1–R13 和 design.md §3/§10/§12 的方案组织，共 8 个原子任务。依赖顺序：T1 → T2 → T3 → T4 →（T5 / T6 可并行）→ T7 → T8。

---

## T1：补齐 `util/const.go` 字段 category 常量  `[x]` 完成 2026-04-19

**关联需求**：R12（禁止裸字符串）

**涉及文件**：`backend/internal/util/const.go`

**产出**：
- 若文件中尚无 `FieldCategoryBasic` / `Combat` / `Perception` / `Movement` / `Interaction` / `Personality` 这组常量，新增一节（对齐现有 `DictGroupXxx` / `RefTypeXxx` 风格）
- 值与 `cmd/seed/main.go` 里 `fieldCategories` 切片定义的 name 字段严格一致（`"basic"` / `"combat"` / `"perception"` / `"movement"` / `"interaction"` / `"personality"`）
- 追加分节注释 `// ========== 字段 category ==========`

**做完了是什么样**：
- `grep -n "FieldCategoryBasic" backend/internal/util/const.go` 有匹配且值为 `"basic"`
- `go build ./...` 通过
- 若常量已存在则本任务直接关闭，标注"无需变更"

---

## T2：新建 seed 数据文件 + 字段 seed 函数（9 个字段含 hp 孤儿）

**关联需求**：R2、R7、R8、R12、OQ3 方案 A

**涉及文件**：`backend/cmd/seed/field_template_npc_seed.go`（新建）

**产出**：
- 文件顶部 `package main` + import 块（对齐 `main.go` 风格）
- 导出函数 `SeedFieldsTemplatesNPCs(ctx context.Context, db *sqlx.DB) error`（聚合入口，T3 追加其内部步骤）
- 本任务内实现第一步 `seedFields(ctx, db) error`：
  - 9 个字段切片字面量（按 design.md §3.1 表）含 **hp 孤儿字段 `enabled=0`**
  - 每个字段的 `properties` 用 `mustRawJSON`（复用 `main.go` 里已有 helper，若非 export 则在本文件复制一份同名 unexported）
  - `INSERT IGNORE INTO fields (...) VALUES (...)`，逐条插入，统计 `inserted` / `skipped`
  - `loot_table` 的 `expose_bb=false`（R8），`hp` 的 `expose_bb=false` 且 `enabled=0`（OQ3-A）
  - 字段 name / category 走 T1 的常量，无裸字符串

**做完了是什么样**：
- `go build ./cmd/seed` 通过
- 在干净 MySQL 上跑 `go run ./cmd/seed -config <config>` 成功，stdout 有 `"字段写入完成：新增 9 条，跳过 0 条"`
- 再跑一次，stdout 有 `"字段写入完成：新增 0 条，跳过 9 条（已存在）"`（幂等 R7）
- `SELECT name, type, enabled FROM fields WHERE name IN ('max_hp','hp','loot_table',...) ORDER BY name` 返回 9 行，hp 的 enabled=0，其余 enabled=1
- `SELECT JSON_EXTRACT(properties,'$.expose_bb') FROM fields WHERE name='loot_table'` 返回 `false`（R8）

---

## T3：追加模板 seed + NPC 实例 seed + field_refs 维护

**关联需求**：R3、R4、R5、R7、R11（snapshot 对齐）

**涉及文件**：`backend/cmd/seed/field_template_npc_seed.go`（追加）

**产出**：
- 函数 `seedTemplates(ctx, db) error`：
  - 先 `SELECT id, name FROM fields WHERE name IN (...)` 取 9 个字段的 name→id 映射
  - 构造 4 个模板：warrior_base / ranger_base / passive_npc / tpl_guard（fields=`[]` 用 `make([]TemplateFieldRef, 0)`）
  - `INSERT IGNORE INTO templates` 写入
  - 对新插入模板（affected=1）的每个 field_id 追写 `INSERT IGNORE INTO field_refs (field_id, ref_type='template', ref_id=<new_template_id>)`
  - `ref_type` 走 `util.RefTypeTemplate`
- 函数 `seedNPCs(ctx, db) error`：
  - 先拿 template name→id + hp 字段 id
  - 构造 6 条 NPC 记录（按 design.md §3.4 表）
  - guard_basic 的 `fields` JSON 为 `[{field_id:<hp_id>, name:"hp", required:false, value:100}]`
  - 其余 NPC 的 `fields` 按其模板 fields 顺序组装（field_id + name + required=false + snapshot §4 原值）
  - `INSERT IGNORE INTO npcs`，对新插入 NPC 的 bt_refs 展开写 `npc_bt_refs`
- `SeedFieldsTemplatesNPCs` 聚合函数依次调用 `seedFields` → `seedTemplates` → `seedNPCs`
- 每步打印 `新增 X 条，跳过 Y 条`

**做完了是什么样**：
- `go build ./cmd/seed` 通过
- `go run ./cmd/seed` 成功，stdout 三行：字段/模板/NPC 各自的新增/跳过计数
- `SELECT count(*) FROM templates WHERE name IN ('warrior_base','ranger_base','passive_npc','tpl_guard')` = 4
- `SELECT count(*) FROM field_refs WHERE ref_type='template'` ≥ 19（warrior_base 8 + ranger_base 7 + passive_npc 4 + tpl_guard 0 = 19）
- `SELECT count(*) FROM npcs WHERE name IN ('wolf_common',...,'guard_basic')` = 6
- `SELECT JSON_EXTRACT(fields,'$') FROM npcs WHERE name='guard_basic'` 含 `"name": "hp"` 且 `"value": 100`
- 再跑一次幂等（R7），三步均打印新增 0

---

## T4：main.go 接入 + 缓存清理提示

**关联需求**：R1、R9

**涉及文件**：`backend/cmd/seed/main.go`

**产出**：
- 在 `seedBtNodeTypes` 调用之后、`os.Exit` 之前追加：
  ```go
  if err := SeedFieldsTemplatesNPCs(ctx, db); err != nil {
      slog.Error("seed.外部契约数据写入失败", "error", err)
      os.Exit(1)
  }
  ```
- 在所有 seed 步骤完成后打印：
  ```
  ⚠️  若 admin-backend 已启动，请重启或清 Redis 缓存以同步新数据
  ```
  （建议独立函数 `printPostSeedWarning()` 让文案集中管理）

**做完了是什么样**：
- `go build ./cmd/seed` 通过
- `go run ./cmd/seed` 成功运行完整流程：dictionary → fsm_state_dict → bt_node_type → fields/templates/npcs → 终端输出缓存清理提示（R9）
- 输出尾部包含 `⚠️` 或等价 WARN 级字样，肉眼可见

---

## T5：新建 `docs/architecture/api-contract.md`

**关联需求**：R6、R10

**涉及文件**：
- `docs/architecture/`（新建目录）
- `docs/architecture/api-contract.md`（新建）

**产出**：
- 文件开头声明 "ADMIN 为权威源，人工同步" + 本 commit 作为 v1 版本 hash
- `## GET /api/configs/npc_templates` 段落包含：
  - 完整 JSON schema（`items[].name: string`, `items[].config.template_ref: string`, `items[].config.fields: object<string, any>`, `items[].config.behavior.fsm_ref: string`, `items[].config.behavior.bt_refs: object<string,string>`）
  - 字段说明表（每字段的语义 + 允许类型 + 空值语义）
  - **"双边外部契约，服务端 `admin_template.go` 反向依赖此 schema"** 显式声明（R6）
  - "## 已知数据噪声" 小节：guard_basic.fields.hp 是孤儿字段，仅为兼容 snapshot，41008 解封后清除（引用 memory `project_guard_basic_hp_deferred.md`）
- 其他配置导出段落（event_types / fsm_configs / bt_trees / regions）**不在本任务范围**，可用 "## 待补充：...（见 xxx spec）" 占位

**做完了是什么样**：
- `ls docs/architecture/api-contract.md` 存在
- `grep -n "双边外部契约" docs/architecture/api-contract.md` 命中
- `grep -n "guard_basic" docs/architecture/api-contract.md` 命中（已知数据噪声段落）

---

## T6：提取 snapshot §4 基线 JSON

**关联需求**：R11

**涉及文件**：
- `docs/integration/`（新建目录）
- `docs/integration/snapshot-section-4.json`（新建）

**产出**：
- 从 `../NPC-AI-Behavior-System-Server/docs/integration/admin-snapshot-2026-04-18.md` §4 的 JSON 代码块完整拷贝，去掉 markdown 代码围栏
- 文件首行不加注释（需保证纯 JSON 可被 `jq` 解析）
- 内容为 6 个 NPC 的 items 数组完整结构
- 可选：在同目录新建 `README.md` 一句话标注来源 + 用途（"供 seed verify 脚本 diff 用，更新 snapshot 时两边同步"）

**做完了是什么样**：
- `jq '.items | length' docs/integration/snapshot-section-4.json` 返回 `6`
- `jq '.items[] | select(.name=="guard_basic") | .config.fields.hp' docs/integration/snapshot-section-4.json` 返回 `100`
- `jq '.items | map(.name) | sort' docs/integration/snapshot-section-4.json` 返回排序后的 6 个 NPC 名称数组

---

## T7：verify 脚本 + ops runbook（guard_basic 重建路径）

**关联需求**：R1、R2、R3、R4、R5、R13.1（用户 tasks 层追加的 runbook 要求）

**涉及文件**：
- `scripts/verify-seed.sh`（新建）
- `docs/specs/external-contract-admin-shape-alignment/ops-runbook.md`（新建）

**产出**：

**verify-seed.sh**：
- 按 design.md §10.2 的流程：docker compose 启动 → 跑 seed → 验字段数 → 验模板数 → 验 NPC 导出 → 跑 `diff` 对比 T6 的 JSON
- 用 `set -euo pipefail`，失败即退出
- 最后一步：再跑一次 seed，grep stdout 包含"跳过 9 条（已存在）"验证幂等（R7）
- 脚本头注释标明：Windows Git Bash 下 jq 输出可能带 CRLF，用 `tr -d '\r'` 处理（遵循 memory `feedback_bash_utf8_curl.md` 的平台差异警告）

**ops-runbook.md**：
- 标题：`## 操作手册：种子数据管理`
- 段落 1 "seed 脚本作用"：一句话说明它 seed 了什么
- 段落 2 **"guard_basic 恢复流程"**（用户明示）：
  - 明示"**唯一重建路径是重跑 seed**（`go run ./cmd/seed -config <config>`）"
  - 禁止在 UI 新建 NPC 时选 tpl_guard 尝试手建——tpl_guard.fields=[] 无 hp 字段引用
  - 指向 design.md §2 OQ3 和 R13.3 作为技术解释入口
- 段落 3 "41008 解封后的清理步骤"：一句话占位（"届时新建 spec 处理"）

**做完了是什么样**：
- `bash scripts/verify-seed.sh` 在干净 docker compose 环境下退出码 0
- `grep -n "唯一重建路径是重跑 seed" docs/specs/external-contract-admin-shape-alignment/ops-runbook.md` 命中
- 故意删除 guard_basic 后重跑 `scripts/verify-seed.sh` 再次退出码 0（恢复验证）

---

## T8：执行完整验证 + 记录结果

**关联需求**：R1、R2、R3、R4、R5、R6、R7、R8、R9、R10、R11、R12、R13.1、R13.2、R13.3

**涉及文件**：
- `docs/specs/external-contract-admin-shape-alignment/verify-report.md`（新建）

**产出**：
- 在干净环境执行 T7 的 `verify-seed.sh`，记录每一步输出到 verify-report.md
- R13 三条手动验证并截图/文字记录：
  - **R13.1**：`curl http://localhost:9821/api/configs/npc_templates | jq '.items[] | select(.name=="guard_basic").config.fields'` 应返回 `{"hp": 100}`
  - **R13.2**：`curl 'http://localhost:9821/api/v1/fields?enabled=true' | jq '.data.items | map(.name) | contains(["hp"])'` 返回 `false`；`curl 'http://localhost:9821/api/v1/fields?enabled=false' | jq '.data.items | map(.name) | contains(["hp"])'` 返回 `true`
  - **R13.3**：打开 ADMIN UI → 进"NPC 管理" → "新建 NPC" → 选 tpl_guard 模板，截屏字段表单区域，证明**无 hp 字段栏**（该任务是人工 UI 验证，不能脚本化，但必须做）
- R12 审查：`grep -E '(^|[^A-Za-z])"(basic|combat|perception|movement|interaction|personality|template|field|fsm|event_type)"' backend/cmd/seed/field_template_npc_seed.go | grep -v const` 应仅返回注释或 JSON 字面量（不应有散落的裸 string）
- 每条 R 的验证结论：✅ 通过 / ❌ 失败（含错误输出）/ ⚠️ 有条件通过
- 任何 ❌ 必须回溯到对应任务修复，不带病合并

**做完了是什么样**：
- `verify-report.md` 文件存在，13 条 R 每条都有明确结论
- 至少 12 条 ✅（R13.3 如无 UI 截图可暂标 ⚠️，说明原因）
- 0 条 ❌

---

## 任务依赖图

```
T1 util 常量
  ↓
T2 字段 seed
  ↓
T3 模板 + NPC seed
  ↓
T4 main.go 接入
  ↓
T5 api-contract.md     T6 snapshot 基线
         ↓                    ↓
         └──────┬─────────────┘
                ↓
         T7 verify 脚本 + runbook
                ↓
         T8 执行验证 + 报告
```

T5 / T6 可并行；其余严格串行。

---

**Phase 3 结束。等待用户审批后：**
1. **创建 feature 分支**：`git checkout -b feature/external-contract-admin-shape-alignment`
2. **T1 判断密度评估**：T1 是"补 util/const.go 字段 category 常量"——**轻执行场景**（单点常量补齐，无 API 契约 / schema 变更 / 跨模块依赖 / 新抽象层）→ 建议**直接 `/spec-execute T1 external-contract-admin-shape-alignment`**，不需要 `/backend-design-audit`
