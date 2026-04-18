# bt-data-format-unification — 任务拆解

9 个原子任务，按依赖顺序。每个任务 1-3 文件，产出具体可验证。

---

## T1：errcode 扩展 +4 个 BT params 错误码 (R2, R3, R4)  `[x]` 完成 2026-04-18

**文件**：`backend/internal/errcode/codes.go`

**做什么**：
- 新增 4 个常量：`ErrBtNodeBareFields`(44007) / `ErrBtNodeParamMissing`(44008) / `ErrBtNodeParamType`(44013) / `ErrBtNodeParamEnum`(44014)
  - **码段分配**：44001-44006、44009-44012 已占用，44007-44008 + 44013-44015 为预留段；本 task 用 44007/44008/44013/44014 共 4 个（留 44015 最后一个备用）
- `messages` map 补 4 条中文消息：
  - 44007 → `"节点字段结构非法"`
  - 44008 → `"节点缺少必填参数"`
  - 44013 → `"节点参数类型不匹配"`
  - 44014 → `"节点参数取值不在允许集合"`
- 常量插入位置：放在 `ErrBtTreeRefDelete = 44012` 后面，`// 44013-44015 预留` 注释替换为新三项

**做完了是什么样**：
- `grep -E 'ErrBtNode(BareFields|ParamMissing|ParamType|ParamEnum)' backend/internal/errcode/codes.go` 各返回 2 次（常量定义 + messages map）
- `go build ./backend/...` 通过，零新 warning
- 码值与现有常量无冲突（人工核对 44001-44015 整段清单）

---

## T2：Seed 补齐 `move_to` / `flee_from` (R1)  `[x]` 完成 2026-04-18

**文件**：`backend/cmd/seed/bt_node_type_seed.go`

**做什么**：
- 在 `builtinNodeTypes` 切片追加 2 条 `btNodeTypeSeed`（design §1.1 原文 JSON）
- `move_to` params：`target_key_x:bb_key:required` / `target_key_z:bb_key:required` / `speed:float:optional`
- `flee_from` params：`source_key_x:bb_key:required` / `source_key_z:bb_key:required` / `distance:float:optional` / `speed:float:optional`
- **字段名严格对齐游戏服务端**（见 requirements R10 + design 附录），禁止简化成 `target_key` 单字段

**做完了是什么样**：
- `builtinNodeTypes` 切片 `len` = 10
- 本地 docker 环境 `go run ./backend/cmd/seed` 执行成功，stdout 含 `新增 2 条，跳过 8 条`（假设 8 个旧类型已存在；首次运行则新增 10 条）
- `SELECT type_name, category FROM bt_node_types WHERE deleted=0 AND type_name IN ('move_to','flee_from')` 返回 2 行，category 均为 `'leaf'`
- `SELECT JSON_EXTRACT(param_schema, '$.params[*].name') FROM bt_node_types WHERE type_name='move_to'` 含 `"target_key_x"` + `"target_key_z"`

---

## T3：store 层 `ListParamSchemas` 方法 (支撑 R2-R4)  `[x]` 完成 2026-04-18

**文件**：`backend/internal/store/mysql/bt_node_type.go`

**做什么**：
- 新增 `ListParamSchemas(ctx context.Context) (map[string]json.RawMessage, error)`
- SQL：`SELECT type_name, param_schema FROM bt_node_types WHERE deleted=0 AND enabled=1`
- 返回 `map[string]json.RawMessage`（不解析业务类型，对应 design §1.2 修订分层）
- 空结果返回空 map + nil error（不是 nil map，遵循 `dev-rules/go.md` nil map 规约）

**做完了是什么样**：
- 方法签名精确为 `(ctx context.Context) (map[string]json.RawMessage, error)`
- `go build ./backend/...` 通过
- 若有现成的 store 单测文件，加一个 case：seed 后调用返回长度 ≥ 10；无则留待 T4 集成验证

---

## T4：service validator 接入 param_schema 数据驱动 (R2, R3, R4, R12)  `[x]` 完成 2026-04-18

**文件**：`backend/internal/service/bt_tree.go`

**做什么**：
- 新增本地类型（文件内小写首字母私有命名，遵循 admin 红线 11 文件职责）：
  ```go
  type paramSpec struct { Name, Label, Type string; Required bool; Options []string }
  type nodeParamSchema struct { Params []paramSpec }
  func (s *nodeParamSchema) hasParams() bool { return len(s.Params) > 0 }
  ```
- 新增 `validateNodeParams(typeName, node, schema)` 和 `validateParamValue(typeName, p, val)` helper（design §1.2 原文）
- 修改 `validateConfig`：
  - 调用 `nodeTypeStore.ListParamSchemas(ctx)` 得 `map[string]json.RawMessage`
  - 循环 unmarshal 到 `map[string]*nodeParamSchema`；任一解析失败 → `slog.Error` + 返回 500-级错误（不静默降级，对照 red-lines/general.md）
  - 把 `paramSchemas` 作为第 4 参传入 `validateBtNode`
- 修改 `validateBtNode` 签名：`validateBtNode(node, nodeTypes, paramSchemas, depth)`
- 在现有 category switch 之前加一段**顶层字段白名单检查**（只允许 `type` / `params` / `children` / `child`）
- 在 category switch 之后加一段 `validateNodeParams` 调用（仅当 schema.hasParams()）
- 错误消息含节点 type 名（便于 UI 定位；design R12 要求的"路径"在本期用 type 名代替，路径索引留后续 spec）

**做完了是什么样**：
- `go build ./backend/...` 通过
- 现有 `bt_tree_test.go` 所有用例继续绿（新 validator 对老合法数据无负面影响）
- validator 新增逻辑通过 T5 单测覆盖（T5 通过即本任务通过）
- `validateBtNode` 签名行在 [bt_tree.go](../../../backend/internal/service/bt_tree.go) 中改成 4 参版本（grep 验证）

---

## T5：validator 新行为单元测试 (R2, R3, R4, R12)

**文件**：`backend/internal/service/bt_tree_test.go`

**做什么**：
- 新增 1 个测试函数 `TestValidateBtNode_ParamsHardening`，含 10 个子用例（design §8.1 表格）：
  1. 顶层裸字段（`action` 在顶层） → `ErrBtNodeBareFields`
  2. 叶子缺 params → `ErrBtNodeBareFields`
  3. params 是数组非对象 → `ErrBtNodeBareFields`
  4. params 缺必填 `result` → `ErrBtNodeParamMissing`
  5. params.name 类型错（number 而非 string）→ `ErrBtNodeParamType`
  6. params.result 枚举非法 → `ErrBtNodeParamEnum`
  7. bb_key 空串 → `ErrBtNodeParamType`
  8. 最简合法 sequence+stub_action → nil
  9. `move_to` 合法 → nil
  10. `move_to` 缺 `target_key_z` → `ErrBtNodeParamMissing`
- 每个用例构造节点 JSON + 期望错误码常量；使用 `errors.Is(err, errcode.XXX)` 断言（遵循 dev-rules/go.md `errors.Is`）
- mock `nodeTypeStore` 返回预构造的 paramSchemas（用 testify mock 或手工 fake）

**做完了是什么样**：
- `go test ./backend/internal/service/... -run TestValidateBtNode_ParamsHardening -race -v` 全绿（10/10）
- `go test ./backend/internal/service/... -race` 全部绿（无旧用例回归）

---

## T6：迁移 transformer 纯函数 + 单测 (R5, R8, R9, R10, R11)

**文件**：
- `backend/cmd/bt-migrate/transform.go`（新增）
- `backend/cmd/bt-migrate/transform_test.go`（新增）

**做什么**：
- `transform.go` 导出核心函数：
  ```go
  // transformNode 规则化单节点+递归
  // treeName 用于 #4 特判；pathHint 用于 warning 定位子节点（"children[1]"）
  func transformNode(node map[string]any, treeName string, pathHint string) (newNode map[string]any, warnings []string, err error)
  ```
- 实现 design §1.4 规则表 6 条规则
- #4 特判：当 `treeName == "bt/combat/attack"` && 当前节点 `{type:"stub_action"}` 无其他字段时，按 `pathHint` 末段（`children[1]` / `children[2]`）映射到 `attack_prepare` / `attack_strike`
- `transform_test.go` 含 5 个用例（design §8.1 下半表格）+ 1 个完整 tree #4 用例（验证两个占位同时生效）

**做完了是什么样**：
- `go test ./backend/cmd/bt-migrate/... -race -v` 全绿（6/6）
- transformer 对"已经符合新格式"的节点是 identity 变换（idempotent 单测）—— 即再次跑迁移无副作用
- `go build ./backend/cmd/bt-migrate/...` 通过（单文件非 main 编译仅需 transform.go 自含）

---

## T7：迁移 CLI 主程序（读 MySQL + 写 HTTP） (R5, R6)

**文件**：`backend/cmd/bt-migrate/main.go`

**做什么**：
- flag 解析：`--dsn` / `--admin-url` / `--apply` (bool, default false) / `--tree-id` (int64, default 0)
- pipeline：
  1. `sqlx.Connect("mysql", *dsn)` → `defer db.Close()`
  2. `SELECT id, name, config, version, display_name, description FROM bt_trees WHERE deleted=0 [AND id=? if tree-id>0]`
  3. loop：对每棵调 `transformNode(root, name, "$")`；打印 `[BEFORE] / [AFTER] / [CHANGES] / [WARNING]` 块（design §1.4 示例）
  4. 若 `*apply`：构造 `UpdateBtTreeRequest{ID, Version, DisplayName, Description, Config: newConfigJSON}` → `http.Client{Timeout: 10s}` → `PUT {admin-url}/api/v1/bt-trees/{id}` → 检查 HTTP 200 + JSON `code==0`；任一失败 → `log.Fatal` 终止（不 continue 避免半迁移）
  5. 结尾 `=== Summary === N/6 trees transformed[, M applied]`
- 错误处理：HTTP 超时 10s；JSON 解析失败或 HTTP 非 2xx → 打印响应 body + 终止

**做完了是什么样**：
- `go build ./backend/cmd/bt-migrate` 成功产出二进制
- `go run ./backend/cmd/bt-migrate --help` 列出 4 个 flag
- `go run ./backend/cmd/bt-migrate --dry-run`（对一个空 / 不可达 DSN）失败退出码非零 + 清晰中文错误（不 panic stack）

---

## T8：dry-run + apply 端到端验证 (R5, R6, R8, R9, R10, R11)

**文件**：`docs/specs/bt-data-format-unification/apply-log.md`（新增）

**做什么**（执行类任务，产物是日志）：
1. `docker compose up -d --build admin-backend mysql redis` → 等 healthcheck ok
2. `docker exec -i admin-mysql mysql ... -e "SELECT COUNT(*) FROM bt_node_types"` → 若 < 10，容器内跑 seed：`docker exec admin-backend /app/seed`
3. 在项目根跑 `go run ./backend/cmd/bt-migrate --dry-run` → stdout 重定向到 `apply-log.md` 的 `## Dry-run 输出` 段落
4. 人眼审阅 6 棵 diff，确认符合预期；特别确认 #4 两个占位（`attack_prepare` / `attack_strike`）正确出现
5. `go run ./backend/cmd/bt-migrate --apply` → stdout 同样记录
6. 验证断言（逐条写入 apply-log.md）：
   - a) `SELECT id, JSON_EXTRACT(config, '$.category') FROM bt_trees` 全部 `NULL`（R9）
   - b) 全文搜 `target_key"`（带右引号，避免 `target_key_x` 误中）零命中（R10）
   - c) `SELECT JSON_EXTRACT(config, '$.children[1].params.name') FROM bt_trees WHERE id=<attack id>` = `"attack_prepare"`（R8）
   - d) `curl -s {admin-url}/api/v1/bt-trees/<id>` 6 次，全部 HTTP 200（R6）

**做完了是什么样**：
- `apply-log.md` 存在，含 5 段：dry-run 输出 / apply 输出 / 4 条 SQL/curl 断言结果 / 人眼审阅签字（写"审阅通过"或"发现问题 X"）
- 4 条断言全部 ✅

---

## T9：跨项目 e2e — game server BuildFromJSON 验证 (R7)

**文件**：`docs/specs/bt-data-format-unification/cross-project-verify.md`（新增）

**做什么**：
1. `curl -s http://localhost:9821/api/configs/bt_trees > /tmp/bt_trees.json`
2. 在姐妹项目 `../NPC-AI-Behavior-System-Server/` 新增一个临时单测文件（或一次性 `go run` script），内容：
   ```go
   // 读 /tmp/bt_trees.json 的每个 items[].config
   // 对每条 config 调 BuildFromJSON(configBytes, registry)
   // 打印 "tree[%d] %s: %v" tree id, name, err
   ```
3. 运行 script，预期 6 行全部 `err=nil`
4. 把 script 源码 + 运行输出粘贴到 `cross-project-verify.md`
5. 确认后**删除临时 script**（姐妹项目只读不写，脚本不留痕）—— 或者注明"仅本地执行未提交"

**做完了是什么样**：
- `cross-project-verify.md` 存在，含：script 源码 / 运行输出 6 行 / 6/6 `err=nil` 结论
- 若任一 tree `err != nil`：记录具体错误 + 回到 T6/T7 修 transformer（不视为 T9 完成）

---

## 依赖关系图

```
T1 errcode ─┐
            ├→ T4 validator ─┬→ T5 单测
T3 store ───┘                │
                             │
T2 seed ─────────────────────┴→ T8 dry-run+apply ─→ T9 e2e
                             ↑
T6 transformer ─→ T7 CLI ────┘
```

**关键顺序**：
- T1 / T2 / T3 可并行（无互依）
- T4 依赖 T1 + T3
- T5 依赖 T4
- T6 纯函数无依赖（仅需 design 规则表）
- T7 依赖 T6 + T1（errcode 消息可能反序列化时用到）
- T8 依赖 T2 + T4 + T7（需要 admin-backend 跑着新 validator + seed 数据就绪 + CLI 可用）
- T9 依赖 T8 完成

---

## 下一步建议

**T1 是判断密度评估对象**（按 spec-create skill 规则）。

T1 = errcode 加 4 个常量 + 4 条消息。**轻执行场景**（单点扩展，无设计决策），不需要 `/backend-design-audit`。

**建议直接 `/spec-execute T1 bt-data-format-unification`**。

**T4 是本 spec 唯一重判断任务**（新类型引入 / 跨层 JSON 透传 / 错误码语义四分 / 数据驱动改造），建议到 T4 之前（T3 完成后）主动 `/backend-design-audit T4`，产出决策备忘再执行。这个节点我会在当时再次提醒。
