# 设计：NPC 导出端点引用完整性校验

> 本设计对应 [requirements.md](requirements.md)。

## 0. 对 requirements 的修订

**错误码段位修正**：requirements 写的 `45010` 已被 `ErrNPCBtNotFound` 占用（[errcode/codes.go:163](backend/internal/errcode/codes.go#L163)）。NPC 段当前用到 45015，本 spec 取下一空位 **45016 `ErrNPCExportDanglingRef`**。R3 验收标准的 JSON 示例 code 字段同步改 45016。

**T6 实施前二次修订（2026-04-18，T6 实施前发现 + 用户确认）**：

原 design §1.4 让 `NpcService.validateExportRefs` 直接调 `s.fsmConfigService.CheckEnabledByNames` —— **违反 [npc_service.go:22-24](backend/internal/service/npc_service.go#L22) 明文硬约束**：

> `NpcService 严格遵守"分层职责"硬规则：只持有自身的 store/cache，不持有 templateService / fieldService / fsmService / btService。跨模块校验由 handler 层负责。`

按既有 NPC Create/Update pattern（[handler/npc.go:170-186](backend/internal/handler/npc.go#L170)）改为 **handler 编排，service 拆 4 个纯方法**：

- service 不再持有跨服务依赖
- 业务验证在 service（纯输入输出，易测）
- 跨模块 IO 在 handler（编排）

§1.1 / §1.4 / §1.6 全部按此重写，§2 新增对比，§3 补 admin #10 检查，§5 依赖图简化。

## 1. 方案描述

### 1.1 整体调用链（handler 5 步编排）

```
GET /api/configs/npc_templates
  → ExportHandler.NPCTemplates(c)
    │
    │ Step 1：取原始 rows（service 纯查 DB，无校验）
    ├─ rows, err = npcService.ExportRows(ctx)
    │     └─ store.ExportAll(ctx)              （既有 store 方法，不变）
    │
    │ Step 2：收集引用反查索引（service 纯函数，无 IO）
    ├─ refs, err = npcService.CollectExportRefs(rows)
    │     → refs.FsmIndex: map[fsmName][]npcName
    │     → refs.BtIndex:  map[btName][]{npcName, state}
    │
    │ Step 3：跨模块校验（handler 直接调 fsm/bt service，2 次 SQL）
    ├─ fsmNotOK, err = fsmConfigService.CheckEnabledByNames(ctx, keysOf(refs.FsmIndex))
    ├─ btNotOK,  err = btTreeService.CheckEnabledByNames(ctx, keysOf(refs.BtIndex))
    │
    │ Step 4：构建 dangling error（service 纯函数，无 IO）
    ├─ dangling = npcService.BuildExportDanglingError(refs, fsmNotOK, btNotOK)
    │     if dangling != nil → HTTP 500 + {code:45016, msg, details:[...]}, return
    │
    │ Step 5：装配 items（service 纯函数，无 IO）
    └─ items, err = npcService.AssembleExportItems(rows)
          → HTTP 200 + {items}
```

**关键点**：handler 持有跨模块编排责任，service 4 个新方法全部"纯输入输出"——不调用其他 service、不发起 IO（除 ExportRows 直查自己 store）。这与 NPC Create/Update 既有 pattern 完全一致。

### 1.2 数据结构

新增到 [backend/internal/model/npc.go](backend/internal/model/npc.go)（导出区段下方）：

```go
// NPCExportDanglingRef 单条悬空引用记录
type NPCExportDanglingRef struct {
    NPCName  string `json:"npc_name"`
    RefType  string `json:"ref_type"`            // "fsm_ref" | "bt_ref"
    RefValue string `json:"ref_value"`           // 引用的 FSM/BT name
    Reason   string `json:"reason"`              // "not_found" | "disabled"
    State    string `json:"state,omitempty"`     // 仅 ref_type=bt_ref 时有值（FSM 状态名）
}
```

> **不区分 not_found 和 disabled 的 reason**：当前 `CheckEnabledByNames` 返回 `notOK` 列表时无法区分这两种情形（都不在 enabledSet 里）。统一标记为 `"missing_or_disabled"`，避免假信息。

修订后：

```go
type NPCExportDanglingRef struct {
    NPCName  string `json:"npc_name"`
    RefType  string `json:"ref_type"`            // "fsm_ref" | "bt_ref"
    RefValue string `json:"ref_value"`
    Reason   string `json:"reason"`              // 当前实现统一 "missing_or_disabled"
    State    string `json:"state,omitempty"`     // 仅 ref_type=bt_ref 时有值
}
```

新增到 [backend/internal/errcode/error.go](backend/internal/errcode/error.go)（或新建 `errcode/export_error.go`，按层职责命名）：

```go
// ExportDanglingRefError NPC 导出期发现悬空引用
//
// 实现 error 接口，handler 用 errors.As 提取 Details。
type ExportDanglingRefError struct {
    Details []model.NPCExportDanglingRef
}

func (e *ExportDanglingRefError) Error() string {
    return fmt.Sprintf("npc export found %d dangling refs", len(e.Details))
}
```

> 这里 `errcode` 包 import `model` 包是新依赖方向。验证：`model` 已不 import `errcode`（grep 确认），方向单向，OK。

### 1.3 错误码新增

[backend/internal/errcode/codes.go](backend/internal/errcode/codes.go) NPC 段追加：

```go
ErrNPCExportDanglingRef = 45016 // 导出 NPC 时发现悬空 FSM/BT 引用
```

messages map 同文件追加：

```go
ErrNPCExportDanglingRef: "NPC 导出失败：存在悬空的状态机/行为树引用，请按 details 修复",
```

### 1.4 Service 层签名（4 个纯方法 + 删除 ExportAll）

[backend/internal/service/npc_service.go](backend/internal/service/npc_service.go) 修改：

**新增反查索引类型**（也加到 model 或 service 内部，按内聚就近原则放 service 内部）：

```go
// NPCExportRefs 导出引用反查索引
//
// CollectExportRefs 产物，BuildExportDanglingError 输入。
// FsmIndex: fsmName → 引用它的 NPC 名列表
// BtIndex:  btName  → 引用它的 (npcName, state) 列表
type NPCExportRefs struct {
    FsmIndex map[string][]string
    BtIndex  map[string][]NPCExportBtUsage
}

type NPCExportBtUsage struct {
    NPCName string
    State   string
}
```

**4 个新方法**：

```go
// ExportRows 直查 MySQL 取所有已启用未删除 NPC 原始行
//
// 替代既有 ExportAll 的"取数"职责。返回原始 model.NPC，
// 由调用方编排引用校验和最终装配。
func (s *NpcService) ExportRows(ctx context.Context) ([]model.NPC, error)

// CollectExportRefs 纯函数：扫 rows 构建反查索引
//
// 解析每行 BtRefs JSON 失败立即返回 error（数据损坏不能放行）。
// 空 fsm_ref / 空 bt_refs 不进入 Index（视为合法的"无行为配置"）。
func (s *NpcService) CollectExportRefs(rows []model.NPC) (*NPCExportRefs, error)

// BuildExportDanglingError 纯函数：notOK 名列表 + 反查索引 → 结构化错误
//
// 全部正常返回 nil。任一非空时返回 *ExportDanglingRefError，
// Details 按 (fsm 全部) → (bt 全部) 顺序展开。
//
// 一个 NPC 多次引用同一悬空 BT 不会发生（bt_refs 是 map[state]btName，
// 同 NPC 内 state 唯一），所以 Details 不需要去重。
func (s *NpcService) BuildExportDanglingError(
    refs *NPCExportRefs,
    fsmNotOK []string,
    btNotOK  []string,
) *errcode.ExportDanglingRefError

// AssembleExportItems 纯函数：rows → []NPCExportItem
//
// 抽自既有 ExportAll 的装配段。任一行解析失败立即返回 error，
// 不允许部分装配。
func (s *NpcService) AssembleExportItems(rows []model.NPC) ([]model.NPCExportItem, error)
```

**删除既有 `ExportAll`**：调用方只有 [export.go:118](backend/internal/handler/export.go#L118) 一处，明确切换到上面 4 步编排。删除而非保留 dead code。

**返回签名设计决策**：
- `BuildExportDanglingError` 返回**值类型** `*ExportDanglingRefError`（不是 error），因为 handler 直接用 nil 检查即可，无需 errors.As 解构链路（service 不再走"业务错混 infra 错"的单 error 通道）。
- `ExportRows` / `CollectExportRefs` / `AssembleExportItems` 返回 `(T, error)`，error 都是 infra/数据损坏类，handler 一律走通用 500 通道。

**这是对 T6 audit §6 的二次修订**：audit 写的 `validateExportRefs(ctx, rows) error` 在新拆分下不存在，它的功能被 `CollectExportRefs + BuildExportDanglingError` 替代，且**handler 持有 fsm/bt 调用**而非 service 持有。

### 1.5 新增 store/service helper（对齐 BT 模式）

[backend/internal/store/mysql/fsm_config.go](backend/internal/store/mysql/fsm_config.go) 追加（**完全镜像** [bt_tree.go:195-220](backend/internal/store/mysql/bt_tree.go#L195)）：

```go
// GetEnabledByNames 批量查询指定 name 中已启用且未删除的 FSM 配置名集合
func (s *FsmConfigStore) GetEnabledByNames(ctx context.Context, names []string) (map[string]bool, error) {
    result := make(map[string]bool)
    if len(names) == 0 {
        return result, nil
    }
    query, args, err := sqlx.In(
        `SELECT name FROM fsm_configs WHERE name IN (?) AND enabled = 1 AND deleted = 0`, names)
    if err != nil { ... }
    query = s.db.Rebind(query)

    rows := make([]string, 0)
    if err := s.db.SelectContext(ctx, &rows, query, args...); err != nil { ... }
    for _, name := range rows { result[name] = true }
    return result, nil
}
```

[backend/internal/service/fsm_config.go](backend/internal/service/fsm_config.go) 追加（镜像 [bt_tree.go:387](backend/internal/service/bt_tree.go#L387)）：

```go
func (s *FsmConfigService) CheckEnabledByNames(ctx context.Context, names []string) (notOK []string, err error) {
    if len(names) == 0 { return nil, nil }
    enabledSet, err := s.store.GetEnabledByNames(ctx, names)
    ...
}
```

### 1.6 Handler 层修改（5 步编排，不再用 errors.As）

**前置**：[ExportHandler](backend/internal/handler/export.go#L15) 已注入 `fsmConfigService` 和 `btTreeService`，**不需改 setup**。

[backend/internal/handler/export.go:115-131](backend/internal/handler/export.go#L115) 的 `NPCTemplates` 改为：

```go
func (h *ExportHandler) NPCTemplates(c *gin.Context) {
    ctx := c.Request.Context()
    slog.Debug("handler.export.npc_templates")

    // Step 1: 取 rows
    rows, err := h.npcService.ExportRows(ctx)
    if err != nil {
        h.respondInternalErr(c, "export_rows", err)
        return
    }
    if len(rows) == 0 {
        c.JSON(http.StatusOK, exportResponse{Items: make([]interface{}, 0)})
        return
    }

    // Step 2: 收集引用
    refs, err := h.npcService.CollectExportRefs(rows)
    if err != nil {
        h.respondInternalErr(c, "collect_refs", err)
        return
    }

    // Step 3: 跨模块校验（key 集合空时 helper 自动短路）
    fsmNames := make([]string, 0, len(refs.FsmIndex))
    for name := range refs.FsmIndex { fsmNames = append(fsmNames, name) }
    fsmNotOK, err := h.fsmConfigService.CheckEnabledByNames(ctx, fsmNames)
    if err != nil {
        h.respondInternalErr(c, "check_fsm", err)
        return
    }
    btNames := make([]string, 0, len(refs.BtIndex))
    for name := range refs.BtIndex { btNames = append(btNames, name) }
    btNotOK, err := h.btTreeService.CheckEnabledByNames(ctx, btNames)
    if err != nil {
        h.respondInternalErr(c, "check_bt", err)
        return
    }

    // Step 4: dangling error
    if dangling := h.npcService.BuildExportDanglingError(refs, fsmNotOK, btNotOK); dangling != nil {
        slog.Error("handler.export.npc_templates.dangling_refs",
            "count", len(dangling.Details), "details", dangling.Details)
        c.JSON(http.StatusInternalServerError, gin.H{
            "code":    errcode.ErrNPCExportDanglingRef,
            "message": errcode.Msg(errcode.ErrNPCExportDanglingRef),  // 注意是 Msg 不是 Message
            "details": dangling.Details,
        })
        return
    }

    // Step 5: 装配
    items, err := h.npcService.AssembleExportItems(rows)
    if err != nil {
        h.respondInternalErr(c, "assemble", err)
        return
    }
    c.JSON(http.StatusOK, exportResponse{Items: items})
}

// 通用 500 响应辅助（消除 5 处 ifErr 重复）
func (h *ExportHandler) respondInternalErr(c *gin.Context, stage string, err error) {
    slog.Error("handler.export.npc_templates.error", "stage", stage, "error", err)
    c.JSON(http.StatusInternalServerError, gin.H{
        "code":    errcode.ErrInternal,
        "message": "导出失败，请查看服务端日志",
    })
}
```

> **顺便修了一个既有 bug**：原代码 500 返回 `{"items":[]}` 没有 code 字段，违反 admin red-line #14。本 spec 范围内顺手修正（属于"为达成 R3 必须做的事"，不算 scope creep）。

> **errors.As 不再需要**：编排式实现下 dangling 是 `BuildExportDanglingError` 直接返回值，handler nil 检查即可。`ExportDanglingRefError` 仍实现 `error` 接口（Go 惯例 + T5 已实现），但本 spec 内无消费者依赖该接口路径——保留为未来潜在扩展点。

## 2. 方案对比

### 2.1 编排位置：Service 持有跨服务依赖 vs Handler 编排

| 维度 | Service 持有 fsm/bt（原 design） | **Handler 编排（采用，T6 audit 后修订）** |
|---|---|---|
| 项目硬约束 | **违反** [npc_service.go:22-24](backend/internal/service/npc_service.go#L22) 注释明文「不持有跨服务依赖」 | 符合 |
| 跨模块一致性（feedback memory） | 偏离 NPC Create/Update 既有 pattern | 完全对齐 |
| 单测难度 | 需要 mock fsmConfigService + btTreeService | 测 4 个纯函数（输入输出），**更简单** |
| handler 厚度 | 1 行调 ExportAll | 5 步编排（约 40 行） |
| 可复用性 | service 内部封装，CLI/Job 直调 | CLI/Job 也得照搬 5 步——略折损 |

**选 Handler 编排**：项目硬约束 > 单点便利。可复用性折损可接受（毕设阶段没有 CLI/Job）。

**service 内部纯方法**与 NPC `ValidateBehaviorRefs`（[npc_service.go:431](backend/internal/service/npc_service.go#L431)）风格完全一致——pure 输入输出。

### 2.2 错误码策略：新增 vs 复用现有

| 方案 | 优点 | 缺点 |
|---|---|---|
| 新增 1 个 + details 字段（**选**） | 单一错误码语义清晰；details 携带任意条数 | 需要新增 1 个 code |
| 复用 4 个现有（45008/45009/45010/45011）按悬空类型分别返 | 不增 code | 违反 red-line「禁止错误码语义混用」——这 4 个是"NPC 创建/更新时引用错误"，导出场景语义不同（系统状态而非用户输入） |
| 新增 4 个（export-fsm-missing / disabled / bt-missing / disabled） | 最细粒度 | 1 个失败即整端点 500，前端无法分支处理；4 个 code 都对应同一动作（运营去修引用）→ 过度设计 |

**选方案 1**：新增 1 个 code + 结构化 details，参照 admin red-line #15 的"用结构化字段 > 数字字段"思路。

### 2.3 失败策略：fail-fast vs skip-and-continue

requirements 已定 fail-fast。design 不再纠结。

### 2.4 引用收集时机：assembleExportItem 内 vs 单独前置

| 方案 | 优点 | 缺点 |
|---|---|---|
| 前置 collectRefs（**选**） | 校验早于装配，失败时不浪费 CPU；引用收集只需读 `n.FsmRef` 和 `n.BtRefs`（已在 NPC 行里），不需要解析 fields | 多遍历一次 rows |
| 装配时顺带收集 | 单次循环 | 控制流复杂：装配中途遇悬空要不要继续装配下一个？容易出 bug |

多遍历一次 N 行（纯内存）相比省 1 次扫描带来的可读性损失，前置明显更优。

## 3. 红线检查

逐条核对（所有红线文档已确认存在）：

| 文档 | 相关条款 | 本设计是否触发？ | 应对 |
|---|---|---|---|
| [general.md](docs/development/standards/red-lines/general.md) | 「禁止配置错误延迟到运行时暴露。引用关系必须在加载/保存阶段校验」 | ✅ 本 spec 正是落实此条 | — |
| general.md | 「禁止 lookup 失败时 silent return」 | 不触发——校验失败 → 显式 500 | — |
| general.md | 「禁止 4xx/5xx 混用 JSON 和非 JSON」 | 顺手修了既有 500 缺 code 字段 bug | 见 1.6 |
| [go.md](docs/development/standards/red-lines/go.md) | 「nil slice/map 输出 JSON」 | details 用 `make([]model.NPCExportDanglingRef, 0)` 初始化 | 编码时确保 |
| go.md | 「writeError 后不 return」 | handler 每个分支都 `return` | 见 1.6 代码 |
| go.md | 「500 响应暴露 Go error 原文」 | 通用 500 路径返回中文「导出失败」+ slog 原文 | 见 1.6 |
| go.md | 「错误码语义混用」 | 新建 45016 而非复用 45008-45011 | 见 2.2 |
| go.md | 「硬编码魔术字符串」 | `"fsm_ref"`/`"bt_ref"`/`"missing_or_disabled"` 都定义为 model 包常量 | 详见 §6 |
| go.md | 「分层倒置」 | errcode 新增 import model，需校验单向 | grep 已确认 model 不 import errcode ✓ |
| [mysql.md](docs/development/standards/red-lines/mysql.md) | 「LIKE 不转义」 | 不涉及 LIKE | — |
| mysql.md | 「事务一致性」 | 本路径只读，不在事务中 | — |
| mysql.md | 「乐观锁 rows==0」 | 本路径不写 | — |
| [redis.md](docs/development/standards/red-lines/redis.md) | 全部条款 | ExportAll 直查 MySQL，不走 Redis | — |
| [cache.md](docs/development/standards/red-lines/cache.md) | 全部条款 | 同上 | — |
| [frontend.md](docs/development/standards/red-lines/frontend.md) | 全部条款 | 本 spec 无前端改动 | — |
| [admin/red-lines.md](docs/development/admin/red-lines.md) | #1.5「禁止放行服务端不支持的枚举」 | 本 spec 是落实「不放行悬空引用」 | — |
| admin/red-lines.md | #2.2「禁止创建时引用不存在的配置」 | 已有；本 spec 补「导出时引用必须仍有效」 | — |
| admin/red-lines.md | #4 「禁止硬编码」#1-2「错误码数字/消息进 codes.go」 | 45016 + message 进 codes.go | 见 1.3 |
| admin/red-lines.md | #10「跨模块代码模式」 | service 错误用 `slog.Error + fmt.Errorf("%w")`；**NpcService 不持有跨服务依赖**（T6 二次修订核心） | 见 1.4 / §0 |
| admin/red-lines.md | #11「文件职责」 | helper 加在已有 `npc_service.go` 同包；新错误类型加在 errcode 包 | — |
| admin/red-lines.md | #14「HTTP 5xx 必须 JSON 含 code」 | 顺手修复既有违规 | 见 1.6 |

**全部检查通过。** 顺手修了一处既有 #14 违规。

## 4. 扩展性影响

**新增配置类型轴**：本 spec 沉淀的 pattern（`store.GetEnabledByNames` 批量 → `service.CheckEnabledByNames` 包装 → service 层 `validateExportRefs` 复核 → handler `errors.As` 解 `ExportDanglingRefError`）可直接套用到未来给 fsm_configs / bt_trees 端点加导出期校验（如 FSM condition 引用 BB Key、BT `set_bb_value.key` 引用 BB Key）。**正面影响**。

**新增表单字段轴**：不涉及。

## 5. 依赖方向（T6 二次修订后）

```
handler/export.go (ExportHandler — 已注入 4 service)
   ├─ npcService.ExportRows         → store/mysql/npc.go (既有 ExportAll)
   ├─ npcService.CollectExportRefs  纯函数
   ├─ fsmConfigService.CheckEnabledByNames → store/mysql/fsm_config.go (T2 新增)
   ├─ btTreeService.CheckEnabledByNames    → store/mysql/bt_tree.go    (既有)
   ├─ npcService.BuildExportDanglingError  纯函数
   ├─ npcService.AssembleExportItems       纯函数
   ├─ errcode (ErrNPCExportDanglingRef T3 + ExportDanglingRefError T5)
   └─ model (NPCExportDanglingRef 等 T4)
```

**npcService 不依赖 fsmConfigService / btTreeService**——这是 T6 二次修订的核心。

新增的依赖：`errcode → model`（用 `model.NPCExportDanglingRef` 作为 details 元素类型）。验证 `model` 不反向 import `errcode`：

**T1 (2026-04-18) 已核实**：`grep -rn "internal/errcode" backend/internal/model/` 零匹配，model 包不 import errcode。单向依赖成立，按方案 A 推进：
- `NPCExportDanglingRef` 留 [backend/internal/model/npc.go](backend/internal/model/npc.go)（T4 实施）
- `ExportDanglingRefError` 新建 [backend/internal/errcode/export_error.go](backend/internal/errcode/export_error.go)（T5 实施），import model

方案 B（结构改放 errcode 包，命名 `DanglingRefDetail`）作为反向依赖出现时的逃生路径，本 spec 不启用。

## 6. 陷阱检查

按 [dev-rules](docs/development/standards/dev-rules/) 涉及技术领域：

### Go ([dev-rules/go.md](docs/development/standards/dev-rules/go.md))
- ✅ details 切片用 `make([]model.NPCExportDanglingRef, 0)` 初始化（避免 nil → null）
- ✅ store 错误用 `fmt.Errorf("get enabled fsm_configs by names: %w", err)`（既有规约）
- ✅ 字符串字面量 `"fsm_ref"` / `"bt_ref"` / `"missing_or_disabled"` 提取为 model 包常量：
  ```go
  const (
      ExportRefTypeFsm  = "fsm_ref"
      ExportRefTypeBt   = "bt_ref"
      ExportRefReasonMissingOrDisabled = "missing_or_disabled"
  )
  ```
- ⚠️ ~~handler 用 `errors.As` 而非 `==` 比较 typed error~~ — T6 二次修订后不再走 typed error 通道，handler 直接拿 `BuildExportDanglingError` 返回值做 nil 检查

### MySQL ([dev-rules/mysql.md](docs/development/standards/dev-rules/mysql.md))
- ✅ `IN` 查询用 `sqlx.In + Rebind`（既有 BT/Field 模式）
- ✅ 复核为只读，无事务/锁问题
- ⚠️ **N+1 检查**：`validateExportRefs` 必须**先聚合**所有 NPC 的 fsm/bt name 到两个 set，再各发起 1 次 `IN` 查询。**禁止**循环里逐 NPC 调 `GetEnabledByName`（那就是 N+1）。
- ⚠️ **空集短路**：`CheckEnabledByNames(ctx, nil)` 返回 `nil, nil` 不发 SQL（已有约定）

### 其他领域
- Redis / Cache / Frontend / Mongo：本 spec 不涉及。

## 7. 配置变更

**无**。无新配置文件、无 env var、无 schema 变更、无 migration。

## 8. 测试策略

### 8.1 单元测试（新增 `backend/internal/service/npc_service_test.go` 或追加到既有同名文件）

5 个测试用例覆盖 R8：

| 测试名 | 数据 | 期望 |
|---|---|---|
| TestExportAll_AllValid | 1 NPC 引用启用的 FSM + BT | 返回 1 item，无错误 |
| TestExportAll_FsmMissing | NPC.fsm_ref 指向不存在的 FSM | 返回 `*ExportDanglingRefError`，details 含 1 条 fsm_ref |
| TestExportAll_FsmDisabled | NPC.fsm_ref 指向 enabled=0 的 FSM | 同上 |
| TestExportAll_BtMissing | NPC.bt_refs[s]=不存在的 BT | details 含 1 条 bt_ref + state=s |
| TestExportAll_BtDisabled | NPC.bt_refs[s]=禁用的 BT | 同上 |
| TestExportAll_BatchSql | mock store，10 NPC 引用 5 FSM 3 BT | 断言 GetEnabledByNames 各被调用恰好 1 次（覆盖 R2） |

依赖：用接口或 mock 框架隔离 store。当前 service 是否已 mock？需要看 npc_service 现状（如未 mock，本 spec 评估为 +1 文件 mock）。

### 8.2 e2e（手动 curl，不进 CI）

R3-R7 验收：

```bash
# 准备：seed 1 个 NPC 引用 FSM=guard + BT=guard/patrol
# 1. 全部启用 → 200 + items 长度=1
curl http://localhost:9821/api/configs/npc_templates

# 2. 禁用 BT guard/patrol → 500 + code=45016 + details=[{npc_name,ref_type:bt_ref,state:patrol,...}]
curl -X PUT http://localhost:9821/api/v1/bt-trees/<id>/toggle-enabled -d '{"enabled":false,"version":N}'
curl http://localhost:9821/api/configs/npc_templates

# 3. 不影响其他 3 端点
curl http://localhost:9821/api/configs/event_types     # 仍 200
curl http://localhost:9821/api/configs/fsm_configs     # 仍 200
curl http://localhost:9821/api/configs/bt_trees        # 仍 200
```

## 9. 经验沉淀候选

完成本 spec 后，以下规则候选追加到对应文档：

- **dev-rules/go.md**：「跨模块批量校验 helper 必须返回 `map[name]bool` 而非 `[]string` 命中列表」（已是事实标准，未文档化）
- **admin/red-lines.md #14 补充**：「export 端点的 5xx 也必须含 code 字段，不能因为响应 schema 是 `{items}` 就省略」
- **admin/dev-rules.md**：「跨集合引用配置的端点：创建/更新校验 + 导出/读取复核，两次都做」

> 上述沉淀**不在本 spec 范围内**，作为 spec 完成后的 followup。
