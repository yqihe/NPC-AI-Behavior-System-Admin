# 事件扩展字段 Schema 管理 — 功能定义

> **实现状态**：已完成（后端 + 前端）。
> **归属**：属于事件类型模块的子功能，后端 API 路径 `/api/v1/event-type-schema/*`。

---

## 1. 概述

让策划在 UI 上定义事件类型的可选附加字段，无需修改代码。每个 Schema 定义一个字段的标识、类型、约束和默认值，创建事件类型时表单自动渲染这些扩展字段。

---

## 2. 支持的字段类型

| 类型 | 中文标签 | 约束 | 默认值 |
|------|----------|------|--------|
| `int` | 整数 | min / max / step | 数值 |
| `float` | 浮点数 | min / max / step / precision | 数值 |
| `string` | 文本 | minLength / maxLength / pattern | 字符串 |
| `bool` | 布尔 | 无 | true/false |
| `select` | 选择 | options / minSelect / maxSelect | 选项值 |

不支持 `reference` 类型（`util.ValidExtFieldTypes` 枚举排除了 reference）。

---

## 3. 状态模型

```
创建 → 启用态（enabled=1）
         ↓ toggle-enabled
       停用态（enabled=0）
         ↓ toggle-enabled
       启用态（enabled=1）
         ↓ delete（须先停用 + 无引用）
       软删除（deleted=1）
```

- 创建后默认**启用**（`enabled=1`），与事件类型的默认停用（`enabled=0`）相反。
- 删除前必须先停用（否则 42027），且不能被事件类型引用（否则 42029）。
- `field_name` / `field_type` 创建后不可变。
- `field_name` 软删后不可复用（唯一约束不含 deleted 列）。
- 乐观锁 `version`：编辑和 toggle 操作均需携带当前 version，冲突返回 42030。

---

## 4. 功能清单

### 4.1 列表

- 全量展示（数据量极小，无分页），按 sort_order ASC, id ASC 排序。
- 支持 `enabled` 筛选（null=不筛选）。
- 每条记录包含 `has_refs` 字段（通过 `schema_refs` 表动态查询填充），前端可据此展示引用状态。

### 4.2 创建

请求字段：
- `field_name`：扩展字段标识，`^[a-z][a-z0-9_]*$`，不可重复（含软删除记录）
- `field_label`：中文名
- `field_type`：int / float / string / bool / select
- `constraints`：JSON 对象，按 type 的约束（min/max/options 等）
- `default_value`：JSON 值，必须符合 constraints
- `sort_order`：表单展示顺序

校验流程：field_name 唯一性 → field_type 枚举校验 → 约束自洽校验（`util.ValidateConstraintsSelf`） → 默认值校验（`util.ValidateValue`） → 数量上限检查 → 写 MySQL → Reload 内存缓存。

### 4.3 编辑

- `field_name` / `field_type` 不可变（UpdateRequest 不含这两个字段）。
- 可修改：`field_label`、`constraints`、`default_value`、`sort_order`。
- **引用保护**：如果该 Schema 被事件类型引用（`schemaRefStore.HasRefs`），则调用 `util.CheckConstraintTightened` 检查约束是否被收紧。收紧则拒绝（42028），放宽或不变则允许。
- 约束自洽校验 + 默认值符合新约束。
- 乐观锁更新 → Reload 内存缓存。

### 4.4 删除

- 必须先停用（否则 42027）。
- **引用保护**：如果被事件类型引用（`schemaRefStore.HasRefs`），拒绝删除（42029）。
- 软删除 → Reload 内存缓存。

### 4.5 启用/禁用

- toggle 方式（读当前状态取反），乐观锁保护。
- 启用态走 EnabledGuardDialog 确认弹窗。
- 切换后 Reload 内存缓存。

### 4.6 引用查询（GetReferences）

- 查询某个扩展字段被哪些事件类型引用。
- service 层返回 `SchemaReferenceDetail`：包含 `schema_id`、`field_label`、`event_types` 列表。
- handler 层跨模块补齐：调用 `EventTypeService.GetByID` 为每个引用方补充 `display_name`（Label）。
- 前端用于引用保护提示（展示"被哪些事件类型使用"）。

---

## 5. API 端点（6 个接口）

| 方法 | 路径 | Handler | 说明 |
|------|------|---------|------|
| POST | `/api/v1/event-type-schema/list` | `EventTypeSchemaHandler.List` | 列表（可按 enabled 筛选，含 has_refs 填充） |
| POST | `/api/v1/event-type-schema/create` | `EventTypeSchemaHandler.Create` | 创建扩展字段定义 |
| POST | `/api/v1/event-type-schema/update` | `EventTypeSchemaHandler.Update` | 编辑（field_name/field_type 不可变，引用保护 + 乐观锁） |
| POST | `/api/v1/event-type-schema/delete` | `EventTypeSchemaHandler.Delete` | 软删除（须先停用 + 引用保护） |
| POST | `/api/v1/event-type-schema/toggle-enabled` | `EventTypeSchemaHandler.ToggleEnabled` | 启用/禁用切换（乐观锁） |
| POST | `/api/v1/event-type-schema/references` | `EventTypeSchemaHandler.GetReferences` | 引用详情（含跨模块 display_name 补齐） |

---

## 6. 与事件类型的关系

### 引用关系通过 schema_refs 表追踪

- 事件类型 Create/Update/Delete 时，在同一事务内维护 schema_refs 记录。
- Schema 管理模块通过 `SchemaRefStore.HasRefs` 判断是否被引用，决定是否允许约束收紧或删除。

### 详情页合并

- 事件类型 detail 接口返回 `extension_schema`：启用的 Schema + config 中有值但 Schema 已禁用的。
- `EventTypeSchemaLite` 结构体包含 `id`、`field_name`、`field_label`、`field_type`、`constraints`、`default_value`、`sort_order`、`enabled` 字段。

### 值校验

- 事件类型 create/update 时，通过 `schemaCache.GetByFieldName()` 拿 Schema 定义，用 `util.ValidateValue()` 校验扩展字段值。
- 只有 `enabled=1` 的 Schema 在内存缓存中可查到（GetByFieldName 只返回启用的）。

### 约束收紧检查（CheckConstraintTightened）

当 Schema 被引用时，编辑约束只能放宽不能收紧。检查逻辑在 `util/constraint.go`：
- **int/float**：新 min 不能大于旧 min，新 max 不能小于旧 max；float 额外检查 precision 不能降低
- **string**：新 minLength 不能大于旧 minLength，新 maxLength 不能小于旧 maxLength，pattern 不能变更
- **select**：旧 options 的每个值必须在新 options 中存在（不能删选项），minSelect 不能增大，maxSelect 不能减小
- **bool**：无约束，不检查

错误码使用调用方传入的 `errCode` 参数（Schema 模块传 42028）。
