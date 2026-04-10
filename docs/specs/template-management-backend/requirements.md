# 模板管理后端 — 需求分析

> 对应文档：
> - 功能：[features.md](../../v3-PLAN/配置管理/模板管理/features.md)
> - 后端设计：[backend.md](../../v3-PLAN/配置管理/模板管理/backend.md)

---

## 动机

模板是 ADMIN 平台 V3 三层数据模型（字段 → 模板 → NPC）的中间层。字段管理已完成（含 `field_refs` 表与 `IncrRefCountTx/DecrRefCountTx` 钩子），但**模板管理后端尚未实现**，导致：

1. **NPC 模块无法启动**：NPC 必须依附模板创建（"选模板填值"），没有模板就没法做 NPC 管理。
2. **字段管理的引用详情不完整**：`backend/internal/service/field.go:493-499` 当前用 `fmt.Sprintf("模板#%d", tid)` 占位 label，无法显示真实的模板中文名。
3. **字段管理已建好的 `field_refs` 写入通路被闲置**：`ref_type='template'` 这一半的写入方还不存在，引用计数链路只跑通了 reference 字段对字段的引用部分。

不做的代价：V3 重写阻塞在第二层，所有依赖"选模板"的功能（NPC 管理、行为树/状态机绑定模板等）都没法启动。

---

## 优先级

**当前阶段最高优先级**。理由：

- V3 重写计划是"字段 → 模板 → NPC → 行为配置"线性推进，模板是当前的关键路径节点
- 字段管理已经在等模板管理回填两个集成点（GetByIDs 补 label、缓存级联 hook）
- 模板的设计文档（features.md + backend.md）已经详尽到接口/字段/SQL/错误码级别，没有未决议题
- 数据表 DDL 已经在 `backend/migrations/003_create_templates.sql` 落地，等代码实现

---

## 预期效果

完成后系统行为：

1. **模板 CRUD 闭环**：管理员通过 ADMIN 前端可以新建模板、编辑模板、停用/启用、删除模板，所有操作通过 8 个 REST 接口完成。
2. **字段引用关系自动维护**：创建/编辑/删除模板时，事务内同步写 `field_refs(ref_type='template')` 与 `fields.ref_count`，保证字段管理的"被引用数"和引用详情准确。
3. **三态生命周期严格执行**：启用中模板拒绝编辑/删除，未启用模板可改可删，已被 NPC 引用的模板字段集合/顺序/必填整体锁死。
4. **字段管理引用详情补全**：`field.go` 的占位 `fmt.Sprintf("模板#%d", tid)` 替换成真实 label。
5. **缓存策略与字段管理一致**：列表用版本号失效，详情用分布式锁防击穿，空值标记防穿透，TTL 加 jitter 防雪崩。
6. **接口幂等且并发安全**：乐观锁拦截编辑冲突，FOR SHARE 防删除 TOCTOU。

具体场景：

- **场景 A**：管理员创建"战斗生物模板"，勾选 5 个字段并标记 hp/atk 必填 → 接口返回新模板 ID，模板默认 enabled=0，字段表对应 5 个字段的 ref_count 各 +1。
- **场景 B**：管理员编辑一个无 NPC 引用的模板，移除 1 个字段、新增 2 个字段、调整顺序 → 接口成功，被移除字段 ref_count -1，新增字段 ref_count +1，fields JSON 顺序变更。
- **场景 C**：管理员尝试编辑一个 ref_count=3 的模板的字段列表 → 返回 41008，仅允许改 label/description。
- **场景 D**：管理员尝试删除启用中的模板 → 返回 41009；停用后再删除 → 成功，所有 field_refs 清理，字段 ref_count 回退。
- **场景 E**：在字段管理点击"引用详情"，看到引用方"模板：战斗生物模板（id=12）"而非"模板#12"。

---

## 依赖分析

### 依赖的已完成工作

| 依赖项 | 位置 | 用途 |
|---|---|---|
| `fields` 表与 FieldStore | `backend/internal/store/mysql/field.go` | 字段存在性/启用校验、批量补全详情 |
| `field_refs` 表与 FieldRefStore | `backend/internal/store/mysql/field_ref.go` | 引用关系写入/查询 |
| `FieldStore.IncrRefCountTx/DecrRefCountTx` | 同上 | 维护字段被引用数 |
| 错误码体系 | `backend/internal/errcode/codes.go` | 41001-41011 段位 |
| Handler `WrapCtx` 泛型包装 | `backend/internal/handler/wrap.go` | 统一响应格式 |
| 字典缓存 `DictCache` | `backend/internal/cache/` | category_label 翻译 |
| 配置（PaginationConfig/ValidationConfig） | `backend/internal/config/` | 分页默认值/校验长度 |
| `templates` 表 DDL | `backend/migrations/003_create_templates.sql` | 已就绪 |

### 谁依赖这个需求

| 依赖方 | 需要的内容 | 紧迫度 |
|---|---|---|
| **NPC 模块**（未来） | TemplateStore.GetByID/GetByIDs/IncrRefCountTx/DecrRefCountTx | 阻塞 |
| **字段管理 GetReferences**（已存在） | TemplateStore.GetByIDs 补 label | 立即 |
| 模板管理前端（未来 spec） | 8 个 REST 接口 | 立即 |
| **行为配置模块**（未来） | 引用模板 ID 关联 FSM/BT | 阻塞 |

---

## 改动范围

预估 **新增 7 个文件 + 改动 3 个文件**，**全在 backend/internal/**：

### 新增文件

| 文件 | 作用 |
|---|---|
| `internal/model/template.go` | Template / TemplateListItem / TemplateDetail / FieldEntry / 各请求响应结构 |
| `internal/store/mysql/template.go` | TemplateStore：CRUD + 列表 + 乐观锁 + ref_count 维护 |
| `internal/store/redis/template.go` | TemplateCache：列表/详情/锁/版本号 |
| `internal/service/template.go` | TemplateService：业务逻辑 + 字段引用同步 + 缓存协调 |
| `internal/handler/template.go` | TemplateHandler：8 个接口 + 校验 |
| `internal/service/template_ref_diff.go`（可选） | fields diff 算法（如 service 文件过大则拆出） |
| `internal/store/redis/keys.go` 内的常量补充（不算新文件） | 模板 key 前缀 |

### 改动文件

| 文件 | 改动内容 |
|---|---|
| `internal/errcode/codes.go` | 新增 41001-41011 模板错误码 + 消息 |
| `internal/router/router.go` | 注册 `/api/v1/templates/*` 8 个路由，Setup 签名追加 `*handler.TemplateHandler` |
| `cmd/admin/main.go` | 装配 TemplateStore/Cache/Service/Handler 注入链 |
| `internal/store/redis/keys.go` | 新增 `TemplateDetailKey/TemplateListKey/TemplateLockKey` 与版本号常量 |
| `internal/service/field.go` | 注入 TemplateStore，把第 493-499 行的占位 label 替换成真实查询 |

预估 **5 个新文件 + 4 个改动文件**，约 1500-1800 行新增 Go 代码（参考字段管理 service.go ~750 行 + store ~260 行 + cache ~200 行 + handler ~210 行 + model ~165 行 = ~1585 行；模板管理略简单：无 reference 嵌套校验、无约束收紧检查、无 BB Key）。

---

## 扩展轴检查

ADMIN 平台两个扩展方向：

1. **新增配置类型只需加一组 handler/service/store/validator**：
   - ✅ 本需求**就是**新增一个配置类型（templates），完全套用字段管理已建立的"四件套"模式（handler/service/store/cache）
   - 不会侵入字段管理代码（除集成点 GetByIDs 注入外，字段管理 service 改动局限于一个 TODO 替换）
   - 路由注册、main.go 装配走相同模式，没有破坏对称性
   - **正面**：进一步沉淀"配置类型四件套"的标准模板，后续 NPC/事件/FSM/BT 模块可以更确定地复用

2. **新增表单字段只需加组件**：
   - 本需求是后端，不直接涉及前端字段渲染
   - 但数据结构上：`fields JSON [{field_id, required}]` 是面向"字段引用"的，新增字段类型时模板侧不需要任何改动（模板永远只存 field_id 引用）
   - **中性**

---

## 验收标准

> 编号化、可验证。每条要么有明确的接口/数据库后果，要么有可观测的副作用。

### 接口契约

- **R1**：实现 8 个 REST 接口，路径与 features.md 一致：`/api/v1/templates/{list,create,detail,update,delete,check-name,references,toggle-enabled}`
- **R2**：所有接口走 `handler.WrapCtx` 包装，统一 `{Code, Data, Message}` 响应格式
- **R3**：所有写接口在请求异常时返回对应错误码（41001-41011），错误码常量加入 `errcode/codes.go`
- **R4**：所有"按 ID"请求复用 `model.IDRequest`，不另定义

### 数据一致性

- **R5**：创建模板时，事务内完成 `INSERT templates` + `INSERT field_refs(ref_type='template')` * N + `UPDATE fields SET ref_count = ref_count + 1` * N，全部成功才提交，任一失败回滚
- **R6**：删除模板时，事务内完成 `SELECT ref_count FOR SHARE` + `UPDATE templates SET deleted=1` + `DELETE field_refs WHERE ref_type='template' AND ref_id=?` + 字段 ref_count 回退
- **R7**：编辑模板（ref_count=0）字段集合/顺序变更时，事务内 diff 出 toAdd/toRemove，分别 Add/Remove field_refs 并增减字段 ref_count
- **R8**：编辑模板（ref_count>0）尝试改 fields（含集合、顺序、required 任一变化）时返回 41008，不写库
- **R9**：模板 `ref_count > 0` 时拒绝删除（41007）；`enabled=1` 时拒绝删除（41009）和编辑（41010）
- **R10**：所有 update/toggle-enabled 走 `WHERE id=? AND version=?` 乐观锁，rows=0 返回 41011

### 业务校验

- **R11**：创建/编辑模板时，`fields` 数组为空返回 41004
- **R12**：创建/编辑模板时，`fields` 中存在重复 field_id 返回 `ErrBadRequest`（防御性，前端已去重）
- **R13**：创建/编辑模板时，新增字段引用必须存在（41006）且 enabled=1（41005）
- **R14**：模板 `name` 创建后不可修改，UpdateTemplateRequest 不包含 name 字段
- **R15**：`name` 全局唯一（含软删除），check-name 接口和创建接口都拦截

### 缓存

- **R16**：列表缓存使用版本号 `templates:list:version`，写操作 INCR 即失效
- **R17**：详情缓存 key `templates:detail:{id}`，TTL 5min ± jitter；存的是已补全字段精简列表的 `TemplateDetail`
- **R18**：详情查询使用分布式锁 `templates:lock:{id}` 防击穿，锁后 double-check 缓存
- **R19**：缓存空值用 `{"_null":true}` 标记防穿透
- **R20**：所有写操作清自身 detail（如有）+ INCR list version
- **R21**：Redis 不可用时全部降级到 MySQL 直查，slog.Warn 但不阻塞业务

### 字段管理集成

- **R22**：实现 `TemplateStore.GetByIDs(ctx, ids) ([]TemplateListItem, error)` 暴露给字段管理使用
- **R23**：`field.go:493-499` 的占位 label 替换为真实的 `templateStore.GetByIDs` 查询，字段引用详情显示"模板：<真实 label>"
- **R24**：注入方向是"FieldService 依赖 TemplateStore"，不要反过来（避免循环依赖）

### 并发安全

- **R25**：删除前 `SELECT ref_count FOR SHARE` 在事务内执行，防 TOCTOU
- **R26**：`field_refs` 写入沿用 `INSERT IGNORE` 幂等
- **R27**：详情接口的分布式锁失败时降级直查 MySQL（不阻塞）

### 可观测性

- **R28**：所有 service 入口加 slog.Debug，写操作成功加 slog.Info，错误用 slog.Error 记录上下文（id/name/error）
- **R29**：handler 入口 slog.Debug 记录请求关键字段

### 占位实现

- **R30**：`/templates/references` 接口在 NPC 模块未上线前返回 `{template_id, template_label, npcs: []}`，但接口签名和 service 方法已定义好，NPC 模块上线只需注入 npcStore 实现而无需改 handler/service 入口

---

## 不做什么

明确排除：

1. **NPC 表的创建和 NPC ↔ 模板的引用维护**：NPC 模块的事，本 spec 不碰
2. **模板管理前端（Vue 页面、组件）**：另起 spec
3. **缓存级联失效 hook**（字段编辑/停用时清模板 detail 缓存）：作为后续小型补丁，不在本 spec
4. **模板默认值覆盖**：features.md "已知限制" 已声明延后到毕设后
5. **导入导出/克隆模板**：memory 中的"毕设后延后功能"已确认延后
6. **审计日志**：CLAUDE.md 提到了，但字段管理也尚未实现，本 spec 不开新分支
7. **MongoDB 写入**：模板永远不写 MongoDB（NPC 创建时才会写 npc_templates 集合）
8. **RabbitMQ 同步**：模板没有跨库同步需求
9. **新增 SQL migration**：003_create_templates.sql 已存在且符合 backend.md 设计

---

## 待审批确认事项

请审批以下三个细节后进入 Phase 2：

1. **错误码段位 41001-41011 是否符合规划**？字段管理用 4xxxx 中的 40001-40016，模板管理用 41001-41011 是否需要预留更大段位（比如留给"字段+模板共用"的）？
2. **TemplateStore 是否暴露 `IncrRefCountTx/DecrRefCountTx` 给未来 NPC 模块使用**？现在不会被调用，但接口先放出来便于 NPC 模块对接。倾向：放出来。
3. **`templates:detail:{id}` 缓存内容**：确认存"已补全字段精简列表"的完整 `TemplateDetail`，而不是裸 templates 行。这意味着字段被编辑/停用时模板 detail 缓存会过期。短期内不做缓存级联 hook，靠 5min TTL 自然过期，可接受？
