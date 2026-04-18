# bt-data-format-unification — 需求分析

## 动机

联调 4 项阻塞项之一（🔴 最高优先级）。游戏服务端 `BuildFromJSON()` 严格期望每个 BT 节点形如 `{type, params, children|child}`，而 ADMIN 现存 6 棵 BT 的 `config_json` 存在三种混乱形态（裸字段 / 半裸字段 / 正确嵌套），导致 game server 构建失败 → NPC 无法 spawn。

**不做的后果**：
- `GET /api/configs/bt_trees` 虽然通过 export-ref-validation，但 game server 端 `BuildFromJSON()` 在解析节点 params 时返回 error → 6/6 NPC spawn 失败 → 毕设联调阻塞
- 未来通过 ADMIN UI 新建的 BT 仍会继续漂移（validator 不严），脏数据持续累积

**根因**：
1. ADMIN validator 过度宽松，只校验 `type` + `children|child` 结构，不校验 `params` 形态
2. ADMIN seed 仅注册 8 种 `bt_node_type`，缺 game server 已有的 `move_to` / `flee_from`
3. 历史数据（6 棵）通过手工 UI/API 写入 + docker volume 持久，无迁移路径

## 优先级

🔴 **最高**。在当前联调 4 项阻塞中：
- 🔴 BT 格式统一（本 spec）—— 唯一真正阻断 spawn 的项
- 🟡 event_type name 注入 —— 15 min 修复，不阻断 spawn，只影响 event 语义
- 🟢 hp→max_hp —— 5 min 数据修复
- ⚪ BB key 运行时注册表 —— 不阻断 spawn，只影响 transition 触发，延后独立 spec

本 spec 是"6/6 NPC spawn 成功"的必要且唯一的非平凡工程。

## 预期效果

**场景 1：策划通过 UI 编辑 BT 节点**
策划提交节点 `{"type": "check_bb_float", "op": ">", "key": "hp", "value": 30}`（裸字段，旧格式），后端返回 400 错误 + 明确消息 `"params must be nested under 'params' field"`。提交 `{"type": "check_bb_float", "params": {"key": "hp", "op": ">", "value": 30}}` → 200 成功。

**场景 2：策划使用未知节点类型**
策划提交 `{"type": "unknown_action", "params": {...}}`，后端返回 400 + `"unknown node type: unknown_action; allowed: [sequence, selector, parallel, inverter, check_bb_float, check_bb_string, set_bb_value, stub_action, move_to, flee_from]"`。

**场景 3：策划配置 move_to 动作**
策划从节点类型下拉框选 `move_to`（此前不存在），配置 `{params: {target_key_x: "npc_target_x", target_key_z: "npc_target_z", speed: 3.0}}` → 保存成功 → 导出给 game server → `BuildFromJSON()` 成功。

**场景 4：存量迁移**
管理员执行 `go run cmd/bt-migrate/main.go --dry-run` → 控制台输出 6 棵树的 before/after diff + BT #4 两个空 stub_action 的占位化提醒。确认无误后执行 `go run cmd/bt-migrate/main.go --apply` → 6 棵全部对齐 + 通过新 validator。

## 依赖分析

**上游依赖（已完成）**：
- 本会话反推出的 game server BT schema（见 `design.md` 附录）
- bt_node_types 表 DDL（`migrations/010_create_bt_node_types.sql`）
- BtTreeStore / BtTreeService / BtTreeHandler 全链路（commit `8003c67`、`2025ac5`、`5bd3f1c`）
- export-ref-validation spec（commit `8127657`）—— 导出链路已就位

**下游被依赖（本 spec 解锁）**：
- 联调 6/6 NPC spawn 全通
- 游戏服务端 BT tick 正常运转
- 🟡 event_type + 🟢 hp→max_hp 串行 e2e（本 spec 收尾时顺手完成）

**无依赖"重构"**：本 spec 不接续未合入的 V3 重构 WIP（`feature/v3-refactor-wip`），亦不为其铺路。

## 改动范围

**新增 / 修改文件（预估 5 个）**：

| 文件 | 动作 | 预估行数 |
|---|---|---|
| `backend/internal/service/bt_tree.go` | 修改：validator 硬化（params 嵌套 + 逐类型 schema） | +200 / -80 |
| `backend/cmd/seed/bt_node_type_seed.go` | 修改：补 `move_to` + `flee_from` | +40 |
| `backend/cmd/bt-migrate/main.go` | 新增：一次性迁移脚本 | +250 |
| `backend/internal/errcode/codes.go` | 修改：新增 BT 校验细分错误码 | +5 |
| `backend/internal/service/bt_tree_test.go` | 修改：补新 validator 单测 | +150 |

**总计约 ~645 行变更（+645 / -80）**。无前端改动、无 migration SQL、无 API 契约变更。

## 扩展轴检查

**扩展轴 1（新增配置类型）**：🟢 **正面影响**  
本 spec 向 `bt_node_types` 字典扩展 2 个类型，属字典类配置扩展；不改 handler/service/store/validator 的架构，仅扩展数据。证明"新增 bt_node_type 配置"这条扩展路径可用，**不需要改已有模块代码**。

**扩展轴 2（新增表单字段）**：⚪ **无影响**  
本 spec 不涉及前端 SchemaForm；后端 validator 硬化对现有前端透明（若前端提交不合法数据，后端返回 400，错误在 UI 层可见即可）。

## 验收标准

| 编号 | 标准 | 验证方式 |
|---|---|---|
| R1 | ADMIN seed 注册 10 个 bt_node_type（sequence/selector/parallel/inverter/check_bb_float/check_bb_string/set_bb_value/stub_action/move_to/flee_from），与 game server `registry.go` 一致 | 运行 `cmd/seed` 后 `SELECT COUNT(*) FROM bt_node_types` = 10 |
| R2 | validator 强制 params 嵌套：节点字段除 `type`/`params`/`children`/`child` 外一律报错 | POST `/api/v1/bt-trees` 带 `{"type":"stub_action","action":"wait_idle"}` 返回 400 + 错误码 `ErrBTNodeBareFields` |
| R3 | validator 逐类型校验 params 必填字段 | `check_bb_float` 缺 `params.key` 返回 400；`stub_action` 缺 `params.name` 返回 400 |
| R4 | validator 限定 type 在 10 个注册类型内 | POST 带 `{"type":"foobar",...}` 返回 400 + `ErrBTUnknownNodeType` |
| R5 | 迁移脚本 dry-run 输出 6 棵树的 before/after diff（JSON 片段级），不写 DB | `go run cmd/bt-migrate/main.go --dry-run` 控制台输出含 6 个 tree 段，`SELECT config_json FROM bt_trees` 未变 |
| R6 | 迁移脚本 apply 后 6 棵全部通过新 validator | `--apply` 后对 6 个 ID 循环 `PUT /api/v1/bt-trees/:id`（不改内容），全部 200 |
| R7 | game server 可 `BuildFromJSON()` 成功构建每棵迁移后的树 | 在 game server 单测或 e2e 中对 6 棵树逐一 BuildFromJSON，返回 nil error |
| R8 | BT #4 (`bt/combat/attack`) 两个空 stub_action 迁移为占位：`{name:"attack_prepare", result:"success"}` + `{name:"attack_strike", result:"success"}` | `--apply` 后 `SELECT config_json FROM bt_trees WHERE id=4` 含两个命名节点 |
| R9 | BT #6 (`guard/patrol`) 迁移后无 `category` 字段 | `JSON_EXTRACT(config_json, '$.category') IS NULL` |
| R10 | BB key 参数名统一为 `key`（check_bb_float / check_bb_string / set_bb_value），不保留 `target_key` 旧名 | 6 棵迁移后的 JSON 全文搜 `"target_key"` 零命中 |
| R11 | BT #3 (`bt/combat/chase`) 中 `check_bb_float` 原 `target_key: "perception_range"` 迁移为 `params: {key: "perception_range", op: ">", value: 0}` | JSON_EXTRACT 断言 |
| R12 | 新 validator 的错误消息包含具体节点路径（如 `$.children[1]`）便于 UI 定位 | 单测断言 error.Error() 含路径 |

## 不做什么

- **不做运行时 BB key 注册表**（⚪ 阻塞项第 4 项）—— 独立 spec
- **不重写 BT 前端编辑器**（SchemaForm）—— 后端 validator 变严对前端 API 调用透明；前端若提交不合法数据，后端返回 400 即可；前端优化（如提交前自我校验）留给下轮
- **不在本 spec 修 🟡 event_type name 注入、🟢 hp→max_hp** —— e2e 联调阶段顺手做，不塞进本 spec
- **不引入审计日志** —— 迁移脚本是一次性运维操作，非业务 API
- **不改 bt_node_types 表结构**（`migrations/010_...sql`）—— 仅往表里插 2 条新数据
- **不改 export handler** —— export-ref-validation spec 已就绪，本 spec 不动 `handler/export.go`
- **不做批量回滚机制** —— 迁移脚本在 apply 前已有 dry-run 审阅，失败可人工逐棵修复

## 一个 spec 还是三个？（合规性说明）

`/spec-create` 红线："不准把多个独立功能塞进一个 spec"。本 spec 含三件事：seed 扩展 + validator 硬化 + 存量迁移。判定它们**紧耦合、非独立**，理由：

- 若只做 **seed + validator 硬化**，存量 6 棵立即无法通过新 validator → 导出 500 / UI 编辑卡死 → 线上即坏
- 若只做 **迁移脚本**，未来通过 UI 写入的新数据仍会漂移（validator 旧） → 本次迁移成果几天内归零
- 若只做 **validator 硬化**（不含 seed 补齐），合法使用 `move_to` / `flee_from` 的 BT 全被打回

三者共同构成"BT 数据对齐 game server schema"的**最小可行完整交付**。单独任一件均会让系统处于更坏的中间态，故合为一 spec。
