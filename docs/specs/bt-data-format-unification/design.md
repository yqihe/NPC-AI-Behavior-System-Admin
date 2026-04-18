# bt-data-format-unification — 设计方案

## 0. 关键发现与核心思路

### 发现 1：validator 欠了数据驱动的一半
现有 `validateBtNode`（[bt_tree.go:83-143](../../../backend/internal/service/bt_tree.go#L83-L143)）**只校验 category 结构**（composite→children、decorator→child、leaf→无子），**不校验 params**。但 `bt_node_types.param_schema` 列已经以 JSON 描述每个节点类型的 params 规格（seed 中已有 8 条），即**schema 数据已就位，消费侧缺失**。本 spec 把 validator 接入 param_schema 即可，无需新建 schema 体系。

### 发现 2：迁移只是"对齐真相"，不是"重新定义"
目标格式由游戏服务端 `BuildFromJSON()` 定义（见 requirements 附录引用）。ADMIN 已有 param_schema **大体上已对齐**（8 个 type 的 schema 与游戏服务端 params 字段名几乎一致，只差 `move_to` / `flee_from` 两个缺失类型）。迁移不是重新设计数据，是把 6 棵存量树按 param_schema 规格化。

### 核心思路
1. **seed 补 2 个类型**（`move_to` / `flee_from`），param_schema 以游戏服务端 `registry.go` / `leaves.go` 为准
2. **validator 数据驱动**：新增 `validateNodeParams(node, paramSchemas)` 消费 `param_schema`
3. **迁移脚本** `cmd/bt-migrate/main.go`：直连 MySQL 读 → 规则化转换 → 通过 REST API PUT 写回（受新 validator 保护）

---

## 1. 方案描述

### 1.1 Seed 扩展（`backend/cmd/seed/bt_node_type_seed.go`）

追加 2 个 `btNodeTypeSeed`：

```go
{
    TypeName:    "move_to",
    Category:    "leaf",
    Label:       "移动到",
    Description: "向 Blackboard 中指定坐标移动（读 target_key_x / target_key_z），到达返回 success",
    ParamSchema: json.RawMessage(`{"params":[
        {"name":"target_key_x","label":"目标X坐标BB Key","type":"bb_key","required":true},
        {"name":"target_key_z","label":"目标Z坐标BB Key","type":"bb_key","required":true},
        {"name":"speed","label":"移动速度(单位/秒)","type":"float","required":false}
    ]}`),
},
{
    TypeName:    "flee_from",
    Category:    "leaf",
    Label:       "逃离",
    Description: "从 Blackboard 中指定威胁源逃离（读 source_key_x / source_key_z），达到安全距离返回 success",
    ParamSchema: json.RawMessage(`{"params":[
        {"name":"source_key_x","label":"威胁源X坐标BB Key","type":"bb_key","required":true},
        {"name":"source_key_z","label":"威胁源Z坐标BB Key","type":"bb_key","required":true},
        {"name":"distance","label":"安全距离","type":"float","required":false},
        {"name":"speed","label":"逃离速度(单位/秒)","type":"float","required":false}
    ]}`),
},
```

字段命名严格对齐游戏服务端 `target_key_x` / `source_key_x`（**不是** `target_key` 单字段），原因见 requirements R10 迁移决策。

### 1.2 Validator 硬化（`backend/internal/service/bt_tree.go`）

**修改点 1**：`validateConfig` 签名不变；`validateBtNode` 新增第 4 参 `paramSchemas map[string]*NodeParamSchema`（提前加载）。

**修改点 2**：在 `validateBtNode` 的 category 分支后追加：

```go
// a) 拒绝顶层未知字段（仅允许 type/params/children/child）
for k := range node {
    if k != "type" && k != "params" && k != "children" && k != "child" {
        return errcode.Newf(errcode.ErrBtNodeBareFields,
            "节点 %q 含未知字段 %q（仅允许 type/params/children/child）", typeName, k)
    }
}

// b) 消费 param_schema 校验 params
schema, ok := paramSchemas[typeName]
if ok && schema.HasParams() {
    if err := validateNodeParams(typeName, node, schema); err != nil {
        return err
    }
}
```

**新增函数 `validateNodeParams`**：

```go
func validateNodeParams(typeName string, node map[string]any, schema *NodeParamSchema) error {
    paramsRaw, hasParams := node["params"]
    if !hasParams {
        return errcode.Newf(errcode.ErrBtNodeBareFields,
            "节点 %q 缺少 params 字段", typeName)
    }
    params, ok := paramsRaw.(map[string]any)
    if !ok {
        return errcode.Newf(errcode.ErrBtNodeBareFields,
            "节点 %q 的 params 必须是对象", typeName)
    }
    for _, p := range schema.Params {
        val, exists := params[p.Name]
        if p.Required && !exists {
            return errcode.Newf(errcode.ErrBtNodeParamMissing,
                "节点 %q 缺少必填参数 %q", typeName, p.Name)
        }
        if exists {
            if err := validateParamValue(typeName, p, val); err != nil {
                return err
            }
        }
    }
    return nil
}

func validateParamValue(typeName string, p ParamSpec, val any) error {
    switch p.Type {
    case "bb_key", "string":
        s, ok := val.(string)
        if !ok || s == "" {
            return errcode.Newf(errcode.ErrBtNodeParamType,
                "节点 %q 参数 %q 必须是非空字符串", typeName, p.Name)
        }
    case "float":
        if _, ok := val.(float64); !ok {
            return errcode.Newf(errcode.ErrBtNodeParamType,
                "节点 %q 参数 %q 必须是数字", typeName, p.Name)
        }
    case "select":
        s, ok := val.(string)
        if !ok {
            return errcode.Newf(errcode.ErrBtNodeParamType,
                "节点 %q 参数 %q 必须是字符串枚举", typeName, p.Name)
        }
        if len(p.Options) > 0 && !slices.Contains(p.Options, s) {
            return errcode.Newf(errcode.ErrBtNodeParamEnum,
                "节点 %q 参数 %q 取值 %q 不在允许集合 %v", typeName, p.Name, s, p.Options)
        }
    }
    return nil
}
```

**新增类型**（在 `service/bt_tree.go` 本文件内，不新开文件 — 按 admin 红线 10 跨模块代码模式 + 11 文件职责，小规模私有类型放本文件即可）：

```go
type ParamSpec struct {
    Name     string   `json:"name"`
    Label    string   `json:"label"`
    Type     string   `json:"type"`
    Required bool     `json:"required"`
    Options  []string `json:"options,omitempty"`
}
type NodeParamSchema struct {
    Params []ParamSpec `json:"params"`
}
func (s *NodeParamSchema) HasParams() bool { return len(s.Params) > 0 }
```

**nodeTypeStore 新增方法**（`backend/internal/store/mysql/bt_node_type.go`）：

```go
// ListParamSchemas 返回 type_name → param_schema 原始 JSON（仅 enabled 且 not deleted）
// store 层不解析业务类型，保持单向分层（service 消费时自行 unmarshal 成 NodeParamSchema）
func (s *BtNodeTypeStore) ListParamSchemas(ctx context.Context) (map[string]json.RawMessage, error) { ... }
```

`validateConfig` 在调用 `validateBtNode` 前一次性加载 RawMessage → 解析成 `map[string]*NodeParamSchema` → 传入递归。若某条 schema JSON 损坏，`slog.Error` + 返回 500（属数据层损坏，不做静默降级，对照 red-lines/general.md "禁止静默降级"）。

### 1.3 错误码扩展（`backend/internal/errcode/codes.go`）

| 新常量 | 码 | 消息 |
|---|---|---|
| `ErrBtNodeBareFields` | 44007 | `"节点字段结构非法"` |
| `ErrBtNodeParamMissing` | 44008 | `"节点缺少必填参数"` |
| `ErrBtNodeParamType` | 44013 | `"节点参数类型不匹配"` |
| `ErrBtNodeParamEnum` | 44014 | `"节点参数取值不在允许集合"` |

码段分配说明：BT 段 44001-44015，其中 44001-44006、44009-44012 已用；44007-44008、44013-44015 为预留段。本 spec 使用 44007/44008（相邻）+ 44013/44014（相邻），留 44015 作 BT 段最后一个预留。消息为后端返回前端的中文短语（见 admin 红线 6.3）；`Newf` 可拼具体细节。

### 1.4 迁移脚本（`backend/cmd/bt-migrate/main.go`）

**结构**：

```go
package main

// Flags
var (
    dsn      = flag.String("dsn", "root:root@tcp(localhost:3306)/npc_ai_admin", "...")
    adminURL = flag.String("admin-url", "http://localhost:9821", "...")
    apply    = flag.Bool("apply", false, "false=dry-run，true=写入")
    treeID   = flag.Int64("tree-id", 0, "0=所有，>0=单棵")
)

// Pipeline
// 1. SELECT id,name,config,version FROM bt_trees WHERE deleted=0
// 2. For each tree:
//    a. transformTree(config) → newConfig + report
//    b. if *apply: PUT /api/v1/bt-trees/:id with {version, config: newConfig, display_name, description}
// 3. Print summary

// Core transformer
func transformNode(node map[string]any) (map[string]any, []string) {
    // 返回 (新节点, warnings列表)
    // 规则：
    //   - 归一化 BB key 参数名 (target_key → key, 但 move_to 保持 target_key_x/z 原样)
    //   - 非法顶层字段（action, op, value, key, target_key, category）收集到 params
    //   - stub_action 裸 action → params.name, 补 params.result="success"
    //   - check_bb_float/string 裸字段 → 收进 params
    //   - #4 空 stub_action → 按位置填占位 (attack_prepare/attack_strike)
    //   - 递归 children/child
}
```

**规则表**（`transformNode` 的核心逻辑）：

| 旧形态 | 新形态 | 备注 |
|---|---|---|
| `{type:"stub_action", action:"X"}` | `{type:"stub_action", params:{name:"X", result:"success"}}` | 补 result 默认值 |
| `{type:"stub_action"}`（#4 两个空节点） | 按 tree name + 出现顺序填占位 `attack_prepare` / `attack_strike` | 硬编码 #4 `bt/combat/attack` 定制逻辑 |
| `{type:"check_bb_float", op:">", value:0, target_key:"X"}` | `{type:"check_bb_float", params:{key:"X", op:">", value:0}}` | `target_key` → `key` |
| `{type:"check_bb_float", op:">", value:0, key:"X"}` | `{type:"check_bb_float", params:{key:"X", op:">", value:0}}` | 裸字段收进 params |
| `{type:"stub_action", params:{...}, category:"leaf"}`（#6） | `{type:"stub_action", params:{...}}` | 剔除 category |
| `{type:"sequence", children:[...]}` | 不变，递归子节点 | |

**dry-run 输出示例**（stdout，人类可读）：

```
=== Tree #1  bt/combat/idle ===
[BEFORE]
{"type":"sequence","children":[
  {"type":"stub_action","action":"wait_idle"},
  ...
]}
[AFTER]
{"type":"sequence","children":[
  {"type":"stub_action","params":{"name":"wait_idle","result":"success"}},
  ...
]}
[CHANGES] 2 nodes: stub_action action→params.name (wait_idle, look_around)

=== Tree #4  bt/combat/attack ===
[BEFORE] ...
[AFTER] ...
[CHANGES] 1 node: check_bb_float bare→params
[WARNING] 2 empty stub_action nodes filled with placeholders:
  children[1] → params: {name:"attack_prepare", result:"success"}
  children[2] → params: {name:"attack_strike", result:"success"}

=== Summary ===
6/6 trees transformed. Apply with --apply.
```

**apply 路径**：对每棵树调 `PUT /api/v1/bt-trees/:id`，body `{version, display_name, description, config: newConfigJSON}`。HTTP 401/409/400 任一失败 → 立即终止（不 continue 避免半迁移状态），打印失败树 ID 供人工介入。

**并发**：串行处理 6 棵；无需并发（<10 次网络调用）。

**权限**：脚本不走 auth（ADMIN 无 auth 系统，见 admin 红线 5）。

---

## 2. 方案对比

### 备选 A：SQL 一次性迁移（JSON_SET）
```sql
UPDATE bt_trees SET config_json = JSON_SET(...) WHERE id=1;
```

**不选**，三点不可行：
1. 旧数据 3 种形态混杂，每种需不同 `JSON_SET`/`JSON_REMOVE` 组合；维护性差
2. 递归节点树（#3 有嵌套 selector→sequence）无法用 SQL JSON 函数递归下探（MySQL 8.0 支持 `JSON_TABLE` 但改写成 UPDATE 非常绕）
3. #4 的两个空 `stub_action` 需要"按出现位置填不同占位"的语义，SQL 表达不出

### 备选 B：硬编码 switch 在 validator 里（不走 param_schema 数据驱动）
```go
switch typeName {
case "stub_action":
    // 校验 params.name + params.result
case "check_bb_float":
    ...
}
```

**不选**，两点劣势：
1. **违反扩展轴 1**：`bt_node_types` 新增一个类型（比如将来加 `wait`）要改 service 代码，不能靠只改 seed + DB
2. **权威源分裂**：`param_schema` 列和 validator switch 必须保持一致，否则 seed 说 X 必填但 validator 不校验（或反之）。红线"禁止硬编码魔术字符串"（go.md）的延伸精神

### 备选 C：UI 手工重建 6 棵
**不选**：30+ 次编辑，且**前端改造不在本 spec 范围**（requirements 明确），UI 当前没有严格 validator 配合，编辑过程无保护，极易产生新脏数据。

### 备选 D：迁移脚本直连 service 层（不走 HTTP）
```go
svc := service.NewBtTreeService(...)
svc.UpdateInTx(ctx, tx, req)
```

**不选**：
1. 需复刻完整的 setup 依赖图（config、redis、mysql 连接、缓存对象），大量样板代码
2. **违反 admin 红线 3.1 的精神**（"所有数据变更必须通过 REST API 保证缓存同步"；本项目虽用 MySQL 非 MongoDB，但红线逻辑依然生效 —— 走同一代码路径 = 自动过 validator + 同步清缓存）
3. 跳过 HTTP = 跳过 handler 层预校验（CheckID / CheckVersion），掩盖潜在问题

---

## 3. 红线检查

逐条对照。✅ = 不违反、⚠ = 需注意、❌ = 违反（本 spec 无）。

### `red-lines/general.md`
- **静默降级**：✅ validator 全部显式 `errcode.Newf` 返回 400；迁移脚本失败即 abort
- **配置错误延迟到运行时**：✅ validator 在 Create/Update 时生效，落库前阻断
- **信任前端校验**：✅ 后端独立 validator（前端改造不在本 spec，后端自行严守）
- **HTTP 响应格式**：✅ 新错误码继续走 `errcode` 统一 JSON 响应
- **测试质量**：✅ 单测用具体值，无外部数据依赖
- **过度设计**：✅ 未引入新抽象层（ParamSpec/NodeParamSchema 直接服务 validator）
- **协作失序**：✅ 本 spec 即为设计落文档

### `red-lines/go.md`
- **资源泄漏**：✅ 迁移脚本使用 `defer db.Close()` + `http.Client{Timeout}`
- **序列化陷阱**：✅ `config` 继续用 `json.RawMessage`（见 model.BtTree），非可空列无需指针
- **错误处理**：✅ 迁移脚本出错即 log + 非零退出；不返回 typed nil
- **字符串长度**：✅ 纯 ASCII 标识符（type_name）用 `len()`；label 中文字段本 spec 不新增校验
- **嵌套循环跳转**：✅ validator 递归无 break/continue
- **错误码语义混用**：✅ 4 个新错误码语义互斥（bare/missing/type/enum）
- **缓存反序列化类型丢失**：✅ validator 用 `map[string]any` 解析节点（本就动态），不涉及缓存层类型
- **硬编码魔术字符串**：✅ 节点类型名通过 `paramSchemas` map lookup，不写死在 switch
- **分层倒置**：✅ service 不 import cache（validator 在 service/bt_tree.go）

### `red-lines/mysql.md`
- **事务一致性**：✅ 迁移脚本读直连 `db.QueryContext`、写走 HTTP（handler 内部自管事务）；无事务内混用
- **LIKE 转义**：✅ 本 spec 无 LIKE 查询

### `red-lines/redis.md`
- **SCAN + DEL**：✅ 迁移走 HTTP PUT，handler 层在 commit 前 `DelDetail` + `InvalidateList`（现有实现）
- **DEL 不检查 error**：✅ 现有 cache 代码已处理

### `red-lines/cache.md`
- **写后清缓存**：✅ 走 HTTP PUT → service `UpdateInTx` → handler `InvalidateDetail` + `InvalidateList`（复用现有流程，不改缓存代码）
- **批量修改清 detail 缓存**：✅ 每棵 PUT 独立清 detail + 清 list，不存在批量
- **TTL**：✅ 不改 TTL
- **TOCTOU**：✅ 迁移串行 + 版本号乐观锁

### `red-lines/frontend.md`
- ✅ 不涉及前端

### `admin/red-lines.md`
- **1. 破坏游戏服务端数据格式**：✅ 本 spec 即"**对齐**游戏服务端格式"；1.4（装饰节点用 `child`）、1.5（op/result 枚举）由新 validator 强制
- **2. 引用完整性**：✅ 未改引用逻辑
- **3. 绕过 REST API**：✅ 写路径走 HTTP PUT；**读走直连 SQL 不算"数据变更"**，符合红线字面与精神
- **4. 硬编码**：✅ 新错误码走 `errcode/codes.go`；节点类型名由 seed + DB 权威
- **4b. constraints 自洽校验**：⚪ 不相关（字段 constraints vs BT 节点 params 是两套）
- **5. 过度设计**：✅ 不做 auth/版本回滚/审批
- **6. 暴露技术细节**：✅ 错误消息中文，含具体节点路径（如 `节点 "stub_action" 缺少必填参数 "name"`）
- **7. 表单友好**：⚪ 不涉及前端
- **10. 跨模块代码模式**：✅ service Update 签名不变；错误码命名沿用 `ErrXxxYyy` 规则
- **11. 文件职责**：✅ validator 扩展仍在 `service/bt_tree.go`（小规模私有类型同文件，符合"文件纪律"b）；迁移脚本独立 `cmd/bt-migrate/`（沿用 `cmd/seed` 模式）
- **13. 业务错误码**：⚪ 本 spec 不动前端；新错误码留给后续 BT 编辑器硬化 spec 更新 catch
- **16. Commit 后清缓存**：✅ 复用现有 handler 流程，顺序正确
- **17. Unlock 不传 lockID**：✅ 不新增锁
- **18. 事务内绕过事务查询**：✅ validator 内无事务

**结论**：零违反，零红线修改申请。

---

## 4. 扩展性影响

### 扩展轴 1：新增 bt_node_type — 🟢 正面强化
本 spec 之前 validator 硬编码只认 category（structure），本 spec 之后 validator 完全由 `bt_node_types.param_schema` 驱动。未来新增一个 type（如 `wait`）只需改 seed + 重跑，**零代码改动**。本 spec 即为该扩展轴的一次真实演练（加 `move_to` / `flee_from`）。

### 扩展轴 2：新增表单字段 — ⚪ 无影响
后端 validator 变严对 SchemaForm 透明；前端若未同步，提交不合法数据会收到 400，由前端自行演进。

---

## 5. 依赖方向

```
cmd/bt-migrate (新增)
      │
      │ HTTP PUT
      ▼
 handler/bt_tree (现有)
      │
      ▼
 service/bt_tree (本 spec 改)    ←←← cmd/seed/bt_node_type_seed (本 spec 改，向下写)
      │                                     │
      ▼                                     ▼
 store/mysql/bt_tree            store/mysql/bt_node_type
 store/mysql/bt_node_type       (两者都被 service 读, seed 直接 INSERT)
      │
      ▼
 errcode (本 spec 加 4 个常量)
```

**单向向下无环**。`cmd/bt-migrate` 不 import 任何 service/store；通过 HTTP 与 admin-backend 对话，架构上是独立进程。`cmd/seed` 只向下写 DB，不依赖 service。

---

## 6. 陷阱检查（对照 `dev-rules/`）

### `dev-rules/go.md`
- **nil slice → null**：✅ `ParamSpec.Options` 用 `make([]string, 0)` 或 `json:"options,omitempty"`（枚举为空时无意义）
- **omitempty 吞零值**：⚠ `Required bool` 不加 `omitempty`（false 是合法值，不要丢弃）
- **json.Unmarshal 到 any 数字变 float64**：✅ `validateParamValue` 中 `float` case 断言 `float64`（不是 `int`）
- **writeError 后必须 return**：✅ 迁移脚本有显式 `os.Exit(1)` / return
- **typed nil**：✅ 错误直接 `return nil`
- **len() 中文**：✅ 本 spec 不校验中文长度（label 不验）
- **包设计**：✅ service 不 import cache

### `dev-rules/mysql.md`
- **事务内用 tx 不用 s.db**：✅ 迁移脚本无事务（单条 UPDATE 走 handler 现有事务）
- **LIKE 转义**：✅ 不涉及
- **主键 ID**：✅ 迁移脚本按 ID 处理（SELECT + PUT 都用 ID）

### `dev-rules/redis.md`
- **redis.Nil**：✅ 不改缓存读
- **分布式锁 expire**：✅ 不新增锁

### `dev-rules/cache.md`
- **Cache-Aside 顺序**：✅ 沿用现有 handler 实现

---

## 7. 配置变更

### 7.1 config.yaml
- **无变更**。migration 脚本通过命令行 flag 传 DSN 和 admin URL。

### 7.2 新增 SQL 迁移文件
- **无新增 migration SQL**。seed 通过幂等 INSERT 插入 2 条 bt_node_types 数据；不改表结构。

### 7.3 Schema 变更总览
- 数据库 schema：零变更（bt_node_types.param_schema 列已存在）
- bt_node_types 数据：+2 条（`move_to` / `flee_from`）
- bt_trees 数据：6 条 config_json 规格化（迁移脚本 apply 后）
- API 契约：无新增路由；新错误码 44007~44010 在现有 `/api/v1/bt-trees` 路径下可能出现

---

## 8. 测试策略

### 8.1 单元测试（`backend/internal/service/bt_tree_test.go`）

**validator 新增用例**（配合新 param_schema 消费）：

| 用例 | 输入节点 | 期望错误码 |
|---|---|---|
| 顶层裸字段 | `{type:"stub_action", action:"X"}` | `ErrBtNodeBareFields` |
| params 缺失 | `{type:"check_bb_float"}` | `ErrBtNodeBareFields` |
| params 非对象 | `{type:"stub_action", params:[]}` | `ErrBtNodeBareFields` |
| 必填缺失 | `{type:"stub_action", params:{name:"X"}}` （缺 result） | `ErrBtNodeParamMissing` |
| 参数类型错 | `{type:"stub_action", params:{name:123, result:"success"}}` | `ErrBtNodeParamType` |
| 枚举非法 | `{type:"stub_action", params:{name:"X", result:"maybe"}}` | `ErrBtNodeParamEnum` |
| bb_key 空串 | `{type:"check_bb_float", params:{key:"", op:">", value:1}}` | `ErrBtNodeParamType` |
| 合法最简 | `{type:"sequence", children:[{type:"stub_action", params:{name:"X",result:"success"}}]}` | nil |
| move_to 合法 | `{type:"move_to", params:{target_key_x:"a", target_key_z:"b"}}` | nil |
| move_to 缺必填 | `{type:"move_to", params:{target_key_x:"a"}}` | `ErrBtNodeParamMissing` |

**迁移 transformer 单测**（`backend/cmd/bt-migrate/transform_test.go`）：

| 用例 | 输入 | 期望输出 |
|---|---|---|
| stub_action 裸 action | `{type:"stub_action", action:"wait"}` | `{type:"stub_action", params:{name:"wait", result:"success"}}` |
| check_bb_float 裸字段 | `{type:"check_bb_float", op:">", value:0, key:"hp"}` | `{type:"check_bb_float", params:{key:"hp", op:">", value:0}}` |
| target_key → key | `{type:"check_bb_float", target_key:"hp", op:">", value:0}` | `{type:"check_bb_float", params:{key:"hp", op:">", value:0}}` |
| 剔除 category | `{type:"stub_action", params:{name:"X",result:"success"}, category:"leaf"}` | `{type:"stub_action", params:{name:"X",result:"success"}}` |
| 递归 children | `{type:"sequence", children:[<裸>, <裸>]}` | `{type:"sequence", children:[<规范>, <规范>]}` |

### 8.2 集成测试
- **e2e-01**：docker compose up --build → 跑 seed → `SELECT COUNT(*) FROM bt_node_types WHERE type_name IN ('move_to','flee_from') AND deleted=0` = 2
- **e2e-02**：`cmd/bt-migrate --dry-run` → 人工 diff 审阅 6 棵输出；**不写库**
- **e2e-03**：`cmd/bt-migrate --apply` → 对每棵 `GET /api/v1/bt-trees/:id` 验证新 config_json 通过 validator（200 OK）
- **e2e-04**：迁移后 `GET /api/configs/bt_trees` → 人工样本 2 棵，喂给游戏服务端单测 `BuildFromJSON()` 验证 nil error（跨项目验证，在游戏服务端仓库执行）

### 8.3 回归
- 现有 BT CRUD e2e（若有）必须全绿 —— 新 validator 对已合法数据无侵害
- 现有 BT 单测集合必须全绿 —— 新逻辑不改老测试预期

---

## 附：目标格式（来自游戏服务端反推）

节点基本形态：
```json
{
  "type": "<10 个合法类型之一>",
  "params": { ... 按类型不同 ... },       // leaf 必填，composite/decorator 可省
  "children": [ ... ],                     // composite 必填
  "child": { ... }                         // decorator 必填
}
```

合法类型与 params（详见游戏服务端 [`builder.go`](../../../../NPC-AI-Behavior-System-Server/internal/core/bt/builder.go) + [`leaves.go`](../../../../NPC-AI-Behavior-System-Server/internal/core/bt/leaves.go)）：
- `sequence` / `selector`：无 params，必 children
- `parallel`：params `{policy: "require_all"|"require_one"}` 可省（默 `require_all`）
- `inverter`：无 params，必 child
- `check_bb_float`：`{key, op∈[==,!=,>,>=,<,<=], value:float}`
- `check_bb_string`：`{key, op∈[==,!=], value:string}`
- `set_bb_value`：`{key, value}`
- `stub_action`：`{name, result∈[success,failure,running] 默 success}`
- `move_to`：`{target_key_x, target_key_z, speed:float?}`
- `flee_from`：`{source_key_x, source_key_z, distance:float?, speed:float?}`
