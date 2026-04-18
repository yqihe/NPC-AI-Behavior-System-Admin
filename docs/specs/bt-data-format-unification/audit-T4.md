# T4 — 设计决策备忘（/backend-design-audit）

**任务**：validator 接入 param_schema 数据驱动（`backend/internal/service/bt_tree.go`）  
**审视日期**：2026-04-18  
**前置**：T1（errcode）/ T2（seed）/ T3（store.ListParamSchemas）已落地

---

## 1. 需求摘要

- **做什么**：把现有 `validateBtNode` 从"只校结构"升级为"数据驱动的结构+params 全量校验"，消费 `bt_node_types.param_schema` 列定义的参数规格
- **为什么**：6 棵存量 BT 有 3 种混乱 params 形态（裸字段 / 半裸 / 正确嵌套），使游戏服务端 `BuildFromJSON()` 失败。硬化后新写入数据不会再漂；配合 T6-T8 可对齐存量
- **范围边界**：
  - 在内：`validateConfig` + `validateBtNode` 签名/实现扩展；+2 私有类型（paramSpec / nodeParamSchema）；+2 helper（validateNodeParams / validateParamValue）
  - 不在内：JSON path 完整索引（tasks.md R12 已显式降级到 type 名）；array 类型 params 支持（现有 10 类型的 schema 零 array）；前端改造（spec 已排除）

---

## 2. 七层激活分布

**重点审视**：
- **L3 可推理**：`validateBtNode` 签名从 3 参扩到 4 参，签名要"说人话"——读者看一眼就能推断每参作用
- **L4 可保证**：BT 节点合法性要进入类型系统可表达的最强程度；**param_schema 损坏时的处理**是红线边界
- **L5 可观测**：validator 失败时能否在 3 a.m. 定位到哪棵树、哪个节点、哪个参数

**轻度审视**：
- **L1 看得懂**：paramSpec / nodeParamSchema 是 JSON 映射，形状跟 `param_schema` 列一致即可
- **L2 可外推**：`validateBtNode` 是 unexported，服务 BtTreeService 内部使用

**本次不涉及**：
- **L6 可负担**：design §6 扩展轴分析已证 ROI 正面，这里不重开话题
- **L7 可删除**：validator 是永久设施，不适用

理由：这是"领域模型（校验规则）+ 数据驱动配置消费"的组合，L4 是硬核（所有非法状态必须在类型系统里被挡）；L3 紧随其后（签名不能撒谎）；L5 用于定位 3 a.m. 现场。

---

## 3. 决策表

| 引入的东西 | 解决的具体问题 | 替代方案 + 为什么不选 | 所在层 |
|---|---|---|---|
| `paramSpec` / `nodeParamSchema` 私有类型（首字母小写，不导出） | store 返回的 `json.RawMessage` 反序列化后需要强类型访问 `Name/Type/Required/Options` 四字段，便于 validator 按字段分发 | (A) `map[string]any` 直接遍历：每处都要类型断言，失去编译期检查 / (B) 导出到 model 层：service 以外零使用方，徒增 API 表面积 | L3、L4 |
| `validateNodeParams(typeName, node, schema)` helper | 把"params 对象整体校验"和"params 单值校验"分层——前者判"该有没有、结构对不对"，后者判"值的类型和枚举" | 全部塞进 validateBtNode：100+ 行单函数，category switch 之后继续分 params schema 的 switch，嵌套过深 | L1 |
| `validateParamValue(typeName, p, val)` helper | 对单个 param 的 `p.Type`（bb_key / string / float / select）做类型断言 + 枚举检查 | 在 validateNodeParams 内 inline switch：四个 case 在一个函数里，对应 4 个不同错误码，行数爆炸 | L1、L4 |
| 顶层字段白名单（inline check：只允许 `type` / `params` / `children` / `child`） | 拦截旧格式裸字段如 `{type:"stub_action", action:"wait_idle"}`（设计 R2 要求） | (A) 改用 `struct{Type,Params,Children,Child}` unmarshal 拒绝未知字段：要定义嵌套 struct + 递归每级都用 struct；复杂度成本远大于一个 4-key set / (B) 不做白名单，依赖下游 params schema 校验发现缺失：旧 `action` 裸字段会被"无 params 对象"错误顺带拦到，但错误消息指向 params 缺失，掩盖了根因 | L4 |
| validateBtNode 签名扩到 4 参（加 `paramSchemas map[string]*nodeParamSchema`） | paramSchemas 是递归路径上需要透传的数据，显式参数 > 闭包捕获 > 包级变量 | (A) 闭包捕获 validateConfig 本地变量：让 validateBtNode 变成闭包或方法，签名意图模糊 / (B) 挂到 BtTreeService receiver 并转函数为方法：paramSchemas 本质是单次调用的 snapshot 不是服务状态，强行挂 receiver 让生命周期语义错位 | L3 |
| param_schema 损坏时：`slog.Error` + 返回 500 级错误，**不静默跳过** | seed 数据损坏是 ADMIN 自己的 bug，任何 BT 校验都不应"凑合能用"；静默降级违反 red-lines/general.md "lookup 失败 silent continue 必须记日志" | 部分降级（损坏条目跳过，其它仍校验）：让 seed bug 隐形；参考邻居 `ListBBKeyParamNames` 采用此策略 — 那是历史选择的静默降级 bug，不要复制 | L5 |
| `validateParamValue` 遇未知 `param.Type` 显式 fail（非默认通过） | schema 里若出现如 `"int"`（未在 switch 覆盖），默认通过就等于静默放行非法参数 | default case 直接通过：静默降级风险；未来为某节点加新 param.Type 会失校 | L4 |
| **不** 缓存 paramSchemas（每次 validateConfig 调用都走 DB） | 延续 `ListEnabledTypes` 现有模式（邻居同样每次查），保持对齐；10 行 SELECT 非瓶颈 | 启动时加载到 Service struct 字段：引入缓存失效问题（seed 重跑后需重启）、跨请求共享可变状态（需考虑并发） | L6 |

---

## 4. 反模式拒绝记录

| 被要求 / 本能想做的 | 为什么拒绝 | 替代方案 |
|---|---|---|
| 把 `paramSpec` 改成可导出 `ParamSpec` 放 model 层 | 当前零跨包使用方（单一使用方 validator），强拉到 model = 过早抽象 | 保持 unexported；等第二个使用方（如前端下发 schema 接口、或迁移脚本复用）出现时再升级 |
| 把 `validateParamValue` 抽成支持多态扩展的接口（`ParamValidator interface { Validate() }`） | 4 个 type 的 switch 完全够用；接口会让每次加新 type 都要新建文件 + 注册 | 普通 switch 语句；新类型直接加 case |
| param_schema JSON 解析失败降级为"只校结构不校 params" | 静默降级，掩盖 seed 损坏；red-lines/general.md 第 1 条硬禁 | fail-fast 返回 500 + slog.Error(type_name, raw_bytes) |
| validateBtNode 加完整 JSON path 参数（`$.children[1].params.key`） | tasks.md R12 已承认本期用 type 名代替，完整路径非硬需求；每级递归要构造字符串、消耗 CPU | 本期只带 type 名；留 TODO 注释标记未来升级 |
| 本期就支持 array 类型 param（如 `options: [...]` 作为参数值） | 现有 10 个节点类型的 schema 零 array 参数，超前设计 | 本期不做；validateParamValue 的 default case 抛错，未来新增节点类型带 array 时显式失败可见 |
| 把 `{type/params/children/child}` 白名单抽成包级 var | 4 个字符串 literal 只在 validateBtNode 一处使用，过度抽象 | inline `switch k { case "type","params","children","child": ... }` 或 `if k != "type" && k != "params" ...` |
| 给 validateBtNode 加 feature flag（`ENABLE_STRICT_BT_VALIDATE`） | 新 validator 要对齐游戏服务端 schema；没有"逐步放量"的需求，硬化就是目的 | 直接生效；存量 6 棵 dirty 由 T7 迁移脚本负责对齐 |

---

## 5. 未决问题（需人类决定）

虽然我对每条都有 leaning，但按显式化原则必须让人类点头：

- [ ] **Q1**：错误消息定位用 type 名（如 `"节点 check_bb_float 缺少必填参数 key"`）够不够？还是必须带节点路径（`"$.children[1].params.key"`）？  
  **我的建议**：接受 type 名。6 棵树深度都 ≤ 3，人眼一眼能定位；复杂路径字符串构造非零成本。**R12 已写"本期用 type 名代替路径索引"** — 沿用此决策
- [ ] **Q2**：`validateParamValue` 遇未知 `param.Type` 时 fail vs 通过？  
  **我的建议**：fail（返回 errcode.ErrBtNodeParamType + "未知参数类型 X"）。静默通过 = 未来某 seed 手滑写了 `"type":"integer"` 全链路无感知
- [ ] **Q3**：未来若加 array 类型 param 的节点类型，当前 T4 代码会在 `validateParamValue` default case 抛错。**是否接受这个"未来 TODO"的半衰期**？  
  **我的建议**：接受。现有 10 类型零 array；真要加时改 1 个 case 即可；留 `// TODO: 未来 array 类型 param 支持` 注释即可
- [ ] **Q4**：paramSchemas 和 nodeTypes 两个 map 都用 `WHERE enabled=1 AND deleted=0` 过滤，**是否合并成单次 DB 查询**？  
  **我的建议**：本期不合并。两个方法的调用方不完全重合（ListEnabledTypes 被 SyncNodeTypeRefs 也用），合并会引入跨用途耦合；单棵 BT 保存多 1 次 SELECT（10 行）非性能瓶颈。如后续 profiling 显示是瓶颈再开独立 spec

**给人类的最小决策题**：Q1/Q2/Q3/Q4 是否全部 **接受我的建议**？任一不接受请指出 → 我修改 design.md 并回到本节。

---

## 6. 落地代码门禁

门禁检查：
- [x] **1. 空字段检查**：决策表 7 行全部三列填满
- [x] **2. 敷衍检查**：每行"解决的具体问题"都是具体场景（"拦截 action 裸字段" / "反序列化 RawMessage" 等），无"为可扩展/最佳实践"
- [x] **3. 反模式拒绝检查**：第 4 节 7 条非空（paramSpec 导出 / validateParamValue 接口化 / 降级静默 / JSON path / array 超前 / 白名单抽 var / feature flag）
- [x] **4. 组织前提检查**：第 5 节 4 个未决问题均有主人（本项目单人，我 = 执行者 / 用户 = 决策者）
- [x] **5. 场景感检查**：第 2 节按"领域模型+数据驱动消费"倾斜（L3/L4/L5 重点，L1/L2 轻度，L6/L7 不涉及），非同等权重
- [x] **6. 硬拒绝诚实检查**：需求未要求任何硬拒绝清单事项；第 4 节主动记录了 7 条我自己拒绝的"本能想做但不该做"的事

**六道门禁全部通过** ✅

---

## 下一步

- 如果 Q1-Q4 全部采纳我的建议 → 直接 `/spec-execute T4 bt-data-format-unification`
- 如果任一需要调整 → 指出我调整 design.md + 本备忘后再执行
