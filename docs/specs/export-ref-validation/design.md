# 设计：NPC 导出端点引用完整性校验

> 本设计对应 [requirements.md](requirements.md)。

## 0. 对 requirements 的修订

**错误码段位修正**：requirements 写的 `45010` 已被 `ErrNPCBtNotFound` 占用（[errcode/codes.go:163](backend/internal/errcode/codes.go#L163)）。NPC 段当前用到 45015，本 spec 取下一空位 **45016 `ErrNPCExportDanglingRef`**。R3 验收标准的 JSON 示例 code 字段同步改 45016。

## 1. 方案描述

### 1.1 整体调用链

```
GET /api/configs/npc_templates
  → ExportHandler.NPCTemplates
    → NpcService.ExportAll(ctx)
        ├─ store.ExportAll(ctx)              // 既有：捞 NPC 行（不变）
        ├─ collectRefs(rows)                 // 新增：扫一遍收集 fsm/bt name 集合
        ├─ validateExportRefs(ctx, refs)    // 新增：批量复核
        │     ├─ fsmConfigService.CheckEnabledByNames(ctx, fsmNames)
        │     └─ btTreeService.CheckEnabledByNames(ctx, btNames)
        │   → 任一悬空 → 返回 *ExportDanglingRefError（含 details）
        └─ assembleExportItem 循环（不变）
  → handler 用 errors.As 解出 *ExportDanglingRefError
        → 命中：HTTP 500 + {code:45016, msg, details:[...]}
        → 未命中：原样 500 + 通用错误（既有逻辑）
```

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

### 1.4 Service 层签名

[backend/internal/service/npc_service.go](backend/internal/service/npc_service.go) 修改：

```go
// ExportAll 导出所有已启用且未删除的 NPC（带导出期引用复核）
func (s *NpcService) ExportAll(ctx context.Context) ([]model.NPCExportItem, error) {
    rows, err := s.store.ExportAll(ctx)
    if err != nil { ... }

    // 新增：先复核引用，悬空直接返回结构化错误
    if dangling := s.validateExportRefs(ctx, rows); dangling != nil {
        return nil, dangling   // *errcode.ExportDanglingRefError
    }

    items := make([]model.NPCExportItem, 0, len(rows))
    for _, n := range rows {
        item, err := assembleExportItem(n)
        ...
    }
    return items, nil
}

// validateExportRefs 批量校验所有 NPC 的 fsm_ref / bt_refs.value，悬空返回结构化错误
//
// 全部正常返回 nil。SQL 查询恒为 2 次（FSM/BT 各一次批量），与 N 无关。
func (s *NpcService) validateExportRefs(ctx context.Context, rows []model.NPC) *errcode.ExportDanglingRefError { ... }
```

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

### 1.6 Handler 层修改

[backend/internal/handler/export.go:115-131](backend/internal/handler/export.go#L115) 的 `NPCTemplates` 改为：

```go
func (h *ExportHandler) NPCTemplates(c *gin.Context) {
    items, err := h.npcService.ExportAll(c.Request.Context())
    if err != nil {
        var dangling *errcode.ExportDanglingRefError
        if errors.As(err, &dangling) {
            slog.Error("handler.export.npc_templates.dangling_refs",
                "count", len(dangling.Details), "details", dangling.Details)
            c.JSON(http.StatusInternalServerError, gin.H{
                "code":    errcode.ErrNPCExportDanglingRef,
                "message": errcode.Message(errcode.ErrNPCExportDanglingRef),
                "details": dangling.Details,
            })
            return
        }
        slog.Error("handler.export.npc_templates.error", "error", err)
        c.JSON(http.StatusInternalServerError, gin.H{
            "code":    errcode.ErrInternal,    // 既有通用 500 码
            "message": "导出失败，请查看服务端日志",
        })
        return
    }
    if len(items) == 0 {
        c.JSON(http.StatusOK, exportResponse{Items: make([]interface{}, 0)})
        return
    }
    c.JSON(http.StatusOK, exportResponse{Items: items})
}
```

> **顺便修了一个既有 bug**：原代码 500 返回 `{"items":[]}` 没有 code 字段，违反 admin red-line #14。本 spec 范围内顺手修正（属于"为达成 R3 必须做的事"，不算 scope creep）。

## 2. 方案对比

### 2.1 校验位置：Service 层 vs Handler 层

| 维度 | Service 层（**选**） | Handler 层 |
|---|---|---|
| 单测难度 | 低（直接测 service） | 高（需要 mock gin context） |
| 可复用性 | 未来 CLI/Job 调用同样校验 | 只能 HTTP 用 |
| 跨模块一致性 | service 层已是其他校验逻辑（`ValidateBehaviorRefs`）的位置 | handler 现在很薄 |

**选 Service 层**：与 admin red-line #10「跨模块代码模式」一致——校验/业务规则放 service，handler 仅做 HTTP 适配。

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
| admin/red-lines.md | #10「跨模块代码模式」 | service 错误用 `slog.Error + fmt.Errorf("%w")` | 见 1.4 |
| admin/red-lines.md | #11「文件职责」 | helper 加在已有 `npc_service.go` 同包；新错误类型加在 errcode 包 | — |
| admin/red-lines.md | #14「HTTP 5xx 必须 JSON 含 code」 | 顺手修复既有违规 | 见 1.6 |

**全部检查通过。** 顺手修了一处既有 #14 违规。

## 4. 扩展性影响

**新增配置类型轴**：本 spec 沉淀的 pattern（`store.GetEnabledByNames` 批量 → `service.CheckEnabledByNames` 包装 → service 层 `validateExportRefs` 复核 → handler `errors.As` 解 `ExportDanglingRefError`）可直接套用到未来给 fsm_configs / bt_trees 端点加导出期校验（如 FSM condition 引用 BB Key、BT `set_bb_value.key` 引用 BB Key）。**正面影响**。

**新增表单字段轴**：不涉及。

## 5. 依赖方向

```
handler/export.go
   ├─ service/npc_service.go
   │     ├─ store/mysql/npc.go           (既有)
   │     ├─ service/fsm_config.go         → store/mysql/fsm_config.go (新方法)
   │     └─ service/bt_tree.go            → store/mysql/bt_tree.go    (既有)
   ├─ errcode (新增 ExportDanglingRefError, ErrNPCExportDanglingRef)
   └─ model (既有)
```

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
- ✅ handler 用 `errors.As` 而非 `==` 比较 typed error

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
