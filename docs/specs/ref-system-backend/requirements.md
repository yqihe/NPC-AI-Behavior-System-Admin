# ref-system-backend — 需求分析

## 动机

平台现有五个模块，跨模块引用保护水平参差不齐：

| 关联 | 引用追踪 | 新建过滤 | 存量保留 | 编辑保护 | 删除保护 |
|---|---|---|---|---|---|
| 字段 → 模板 | ✓ field_refs | ✓ | ✓ | ✓ ref_count 驱动 | ✓ |
| 扩展字段 → 事件类型 | ✗ 无 | ✓ | ✓ | ✗ | ✗ |
| 字段(BB Key) → FSM | ✗ 无 | ⚠️ 部分 | ✗ | ✗ | ✗ |

问题：
1. **ref_count 冗余**：字段/模板的 `ref_count` 列是 `field_refs` 表的冗余计数，需要跨模块事务维护 Incr/Decr，增加复杂度。实际删除保护已由 `HasRefsTx` 独立支撑，ref_count 仅用于列表展示（已决定移除展示）和编辑锁定。
2. **扩展字段无保护**：策划可以随意修改已被事件类型使用的扩展字段的类型/约束，甚至直接删除。已有事件类型的 config_json 中存着按旧约束写入的值，下次编辑时可能过不了新约束校验。
3. **BB Key 无追踪**：字段 `expose_bb=true` 暴露的 BB Key 被 FSM 条件引用（`key`/`ref_key` 字符串），但无引用关系追踪。字段被禁用/删除后 FSM 条件中的 BB Key 成为悬空引用。

不做的话：
- 扩展字段被随意修改导致事件类型编辑报错，策划不知道哪些事件类型受影响
- BB Key 字段被删除后 FSM 配置导出给游戏服务端，加载时才发现 Key 不存在
- ref_count 维护逻辑持续增加 NPC 模块开发成本

## 优先级

**高**。ref_count 清理直接影响当前开发体验；扩展字段保护和 BB Key 追踪是数据完整性问题，越晚修复，存量脏数据越多。

## 预期效果

### 场景组 A：ref_count 清理（字段/模板模块）

**A1**：`fields` 表和 `templates` 表无 `ref_count` 列。

**A2**：后端 `IncrRefCountTx`/`DecrRefCountTx`/`GetRefCountTx` 方法全部删除。模板创建/编辑/删除不再调这些方法。

**A3**：字段详情 API 响应新增 `has_refs: boolean`（实时查 field_refs，不缓存）。

**A4**：字段编辑时，被引用判断改用 `field_refs.HasRefs()` —— 有引用则类型不可改、约束只能放宽。逻辑不变，数据源从 ref_count 改为 field_refs。

**A5**：模板删除不再调 `GetRefCountForDeleteTx`（NPC 未上线，无模板引用）。停用后直接删除。

### 场景组 B：扩展字段 → 事件类型 引用保护

**B1**：新建 `schema_refs` 表，结构与 `field_refs` 对齐：
```sql
schema_refs (
    schema_id   BIGINT NOT NULL,       -- 被引用的扩展字段 ID
    ref_type    VARCHAR(16) NOT NULL,   -- 'event_type'
    ref_id      BIGINT NOT NULL,        -- 事件类型 ID
    PRIMARY KEY (schema_id, ref_type, ref_id),
    INDEX idx_ref (ref_type, ref_id)
)
```

**B2**：事件类型创建时，对 config_json 中每个扩展字段 key 写入 `schema_refs(schema_id, 'event_type', event_type_id)`。

**B3**：事件类型编辑时，diff 旧/新扩展字段 key，增删对应的 schema_refs 记录。

**B4**：事件类型删除时，清理该事件类型的所有 schema_refs 记录。

**B5**：扩展字段编辑时，若有引用（schema_refs 中有记录）→ 类型不可改、约束只能放宽（复用字段模块的 `checkConstraintTightened` 逻辑）。

**B6**：扩展字段删除时，检查 schema_refs → 有引用则拒绝删除，返回引用方列表。

**B7**：新增扩展字段 references API：返回哪些事件类型在使用该扩展字段。

**B8**：扩展字段详情（或列表项）新增 `has_refs: boolean`（实时查 schema_refs）。

### 场景组 C：BB Key → FSM 引用追踪

**C1**：`field_refs` 表扩展 `ref_type='fsm'`。FSM 条件中引用的 BB Key 对应字段 ID 写入 `field_refs(field_id, 'fsm', fsm_config_id)`。

**C2**：FSM 创建时，解析条件树提取所有 `key`/`ref_key` → 通过字段 name 查找 field ID → 写入 field_refs。仅追踪来自字段的 BB Key（`expose_bb=true` 的字段），运行时 Key 不追踪。

**C3**：FSM 编辑时，diff 旧/新 BB Key 集合，增删对应 field_refs。

**C4**：FSM 删除时，清理 `field_refs WHERE ref_type='fsm' AND ref_id=fsm_id`。

**C5**：字段 references API 扩展：除返回模板和字段引用方外，新增 FSM 引用方（`ref_type='fsm'`）。

**C6**：字段删除检查自动覆盖 FSM 引用（`HasRefsTx` 查 field_refs 不区分 ref_type，已有逻辑无需改动）。

**C7**：字段取消 `expose_bb` 时（编辑保存），若有 FSM 引用该 BB Key → 拒绝，提示先移除 FSM 引用。

## 依赖分析

**依赖**：
- 字段/模板/事件类型/扩展字段/FSM 五个模块全部已完成
- `field_refs` 表和 `HasRefsTx`/`GetByFieldID` 等方法已存在
- `checkConstraintTightened` + `constraint.ValidateConstraintsSelf` 已存在

**被依赖**：
- 前端 spec（ref-system-frontend）依赖本 spec 的 API 变更
- NPC 模块（未开发）——ref_count 清理降低其开发复杂度
- BT 模块（未上线）——未来可复用 `ref_type='bt'` 追踪 BT 的 BB Key 引用

## 改动范围

### 场景组 A（约 8 文件，已在 feature/ref-cleanup 分支上完成大部分）

| 包 | 文件 | 改动 | 状态 |
|---|---|---|---|
| migrations | `001_create_fields.sql` | 去掉 ref_count 列 + 索引 | ✅ 已完成 |
| migrations | `003_create_templates.sql` | 去掉 ref_count 列 + 索引 | ✅ 已完成 |
| model | `field.go` | 删 RefCount，加 HasRefs | ✅ 已完成 |
| model | `template.go` | 删 RefCount | ✅ 已完成 |
| store/mysql | `field.go` | 删 Incr/Decr/GetRefCount，更新 SQL | ✅ 已完成 |
| store/mysql | `template.go` | 删 Incr/Decr/GetRefCount，更新 SQL | ✅ 已完成 |
| store/mysql | `field_ref.go` | 新增 HasRefs（非事务版） | ✅ 已完成 |
| service | `field.go` | 用 HasRefs 替代 RefCount；GetByID 填充 has_refs | ✅ 已完成 |
| service | `template.go` | 删 GetRefCountForDeleteTx | ✅ 已完成 |
| handler | `template.go` | 删 refCount 检查 | ✅ 已完成 |

### 场景组 B（约 8 文件，全部新增）

| 包 | 文件 | 改动 |
|---|---|---|
| migrations | 新增 `007_create_schema_refs.sql` | 建表 |
| model | `event_type_schema.go` | 加 HasRefs 字段 |
| store/mysql | 新增 `schema_ref.go` | Add/Remove/RemoveBySource/HasRefs/HasRefsTx/GetBySchemaID |
| service | `event_type_schema.go` | Delete 加引用检查；Update 加类型/约束保护；新增 GetReferences |
| service | `event_type.go` | Create/Update/Delete 维护 schema_refs |
| handler | `event_type.go` | Create/Update/Delete 调 schema_refs 维护；新增 event-type-schema references 路由 |
| handler | `event_type_schema.go` | 新增 GetReferences handler |
| router | `router.go` | 注册 references 路由 |

### 场景组 C（约 5 文件）

| 包 | 文件 | 改动 |
|---|---|---|
| service | `fsm_config.go` | Create/Update/Delete 解析条件树提取 BB Key → 维护 field_refs |
| handler | `fsm_config.go` | Create/Update/Delete 后清字段缓存 |
| handler | `field.go` | GetReferences 扩展返回 FSM 引用方 |
| service | `field.go` | GetReferences 扩展查 ref_type='fsm' |
| service | `field.go` | Update 时检查 expose_bb 取消是否有 FSM 引用 |

## 扩展轴检查

- **新增配置类型**：正面。新模块只需选择复用 `field_refs`（新增 ref_type）或新建 `xxx_refs` 表，即可获得完整的引用追踪+编辑保护+删除保护。不需要维护 ref_count。
- **新增表单字段**：不涉及。

## 验收标准

### 场景组 A：ref_count 清理

- **R1**：`fields` 表和 `templates` 表无 `ref_count` 列
- **R2**：后端无 `IncrRefCountTx`/`DecrRefCountTx`/`GetRefCountTx` 方法
- **R3**：字段详情 API 返回 `has_refs: true/false`，值与 field_refs 表一致
- **R4**：字段编辑：有引用时类型不可改(40006)、约束收紧返回(40007)，驱动源为 field_refs
- **R5**：模板删除：停用后直接删除成功（无 ref_count 检查）
- **R6**：`go build ./...` 通过，无编译错误

### 场景组 B：扩展字段引用保护

- **R7**：`schema_refs` 表存在，结构正确
- **R8**：创建事件类型后，`schema_refs` 中有对应记录（每个扩展字段 key 一条）
- **R9**：编辑事件类型增减扩展字段后，`schema_refs` 记录正确增减
- **R10**：删除事件类型后，`schema_refs` 中对应记录全部清除
- **R11**：扩展字段有引用时，编辑改类型 → 被拒绝
- **R12**：扩展字段有引用时，编辑收紧约束（如 min 调大）→ 被拒绝
- **R13**：扩展字段有引用时，删除 → 被拒绝，返回引用方信息
- **R14**：扩展字段无引用时，删除成功
- **R15**：扩展字段 references API 返回引用方事件类型列表
- **R16**：扩展字段详情/列表包含 `has_refs` 字段

### 场景组 C：BB Key 引用追踪

- **R17**：创建 FSM 后，条件树中的 BB Key 对应字段在 `field_refs` 中有 `ref_type='fsm'` 记录
- **R18**：编辑 FSM 增减条件中的 BB Key 后，field_refs 记录正确增减
- **R19**：删除 FSM 后，`field_refs WHERE ref_type='fsm' AND ref_id=fsm_id` 全部清除
- **R20**：字段 references API 返回结果包含 FSM 引用方（ref_type='fsm'）
- **R21**：字段有 FSM 引用时，删除字段 → 被拒绝（`HasRefsTx` 自动覆盖，无需新增逻辑）
- **R22**：字段编辑取消 `expose_bb` 时，若有 FSM 引用 → 被拒绝
- **R23**：FSM 条件中的运行时 Key（不来自字段）不写入 field_refs（找不到对应字段 ID 则跳过）
- **R24**：`go build ./...` 通过

## 不做什么

- **不改前端**：前端改动在 ref-system-frontend spec 中
- **不做 BT 模块的 BB Key 追踪**：BT 未上线，上线后复用 `ref_type='bt'`
- **不做事件类型 → FSM 的引用追踪**：FSM 条件中的事件类型以 name 字符串存储，且 FSM 不校验事件类型存在性（设计如此），后续如需追踪另开 spec
- **不做模板 → NPC 引用追踪**：NPC 未上线
- **不改 reference 字段的 refs 缩减保护**：reference 字段的 refs 变化不影响已有模板（模板存展开后的 leaf 字段 ID）
- **不做 BB Key 后端校验**：FSM 条件中的 BB Key 存在性校验是游戏服务端加载时的职责（设计文档明确）
- **不改 field_refs 表结构**：只扩展 ref_type 取值，不加列
- **不改 schema_refs 的 Redis 缓存**：扩展字段体量小（< 50），引用检查走 MySQL 直查即可
