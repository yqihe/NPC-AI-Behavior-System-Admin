# 字段管理 — 功能清单

> 权威参考文档，与代码一一对齐。最后同步：2026-04-14

---

## 1. 状态模型

| 状态 | enabled | deleted | 本模块列表 | 其他模块下拉 | 能被新引用 | 已有引用 |
|------|---------|---------|-----------|------------|----------|---------|
| 停用（草稿） | 0 | 0 | 正常显示 | 不可见（`enabled=true` 筛选过滤） | 拒绝 | 保持不动 |
| 启用 | 1 | 0 | 正常显示 | 可见可选 | 允许 | 正常 |
| 已删除 | - | 1 | 不可见 | 不可见 | 不可能 | 删除前已确认无引用 |

**核心原则**：停用 = 对新隐藏、对旧保留；删除 = 确认无引用后软删除。

- 新建字段默认 `enabled=0`（草稿），策划确认无误后手动启用。
- 启用中的字段不可编辑（40015），不可删除（40012）。必须先停用，再操作。
- 停用不影响已引用该字段的模板/reference 字段/FSM，它们继续保留引用关系。
- 删除前后端用 `HasRefsTx(FOR SHARE)` 在事务内做最终引用检查，防 TOCTOU。

---

## 2. 模块职责边界

### 本模块拥有

| 层 | 文件 | 职责 |
|----|------|------|
| handler | `handler/field.go` | 请求参数校验（name/label/type/category/properties 格式） + 跨模块编排（GetReferences 补 template label 和 FSM display_name） |
| service | `service/field.go` | 业务逻辑 + Cache-Aside + 引用保护 + 循环引用检测 + 跨模块对外方法 |
| store | `store/mysql/field.go` | `fields` 表 CRUD、覆盖索引查询、乐观锁写入、事务内软删除 |
| store | `store/mysql/field_ref.go` | `field_refs` 关联表全部操作（Add/Remove/RemoveBySource/GetByFieldID/HasRefs/HasRefsTx） |
| cache | `store/redis/field_cache.go` | Detail/List Redis 缓存 + 分布式锁防击穿 |
| model | `model/field.go` | Field/FieldLite/FieldListItem/FieldRef/FieldProperties + 全部 DTO |
| errcode | `errcode/codes.go` | 40001-40017 字段模块错误码 |
| util | `util/constraint.go` | `CheckConstraintTightened` 约束收紧检查（字段模块和事件类型扩展字段共用） |

### 跨模块对外暴露的方法（service 层）

| 方法 | 消费方 | 说明 |
|------|--------|------|
| `ValidateFieldsForTemplate(ctx, fieldIDs)` | 模板 handler | 校验字段全部存在 + 启用 + 非 reference |
| `AttachToTemplateTx(ctx, tx, tplID, fieldIDs)` | 模板 handler | 事务内写入 field_refs(template) |
| `DetachFromTemplateTx(ctx, tx, tplID, fieldIDs)` | 模板 handler | 事务内删除 field_refs(template) |
| `GetByIDsLite(ctx, fieldIDs)` | 模板 handler | 批量查字段精简信息（含 CategoryLabel 翻译） |
| `InvalidateDetails(ctx, fieldIDs)` | 模板/FSM handler | 批量清字段详情缓存 |
| `SyncFsmBBKeyRefs(ctx, tx, fsmID, oldKeys, newKeys)` | FSM handler | 同步 FSM 条件中 BB Key 对字段的引用关系 |
| `CleanFsmBBKeyRefs(ctx, tx, fsmID)` | FSM handler | FSM 删除时清理所有 BB Key 引用 |

---

## 3. 功能清单

### 3.1 基础 CRUD

1. **创建字段**：填写 name（标识符，创建后不可改）、label（中文标签）、type（字段类型）、category（标签分类）、properties（JSON 动态属性）。创建默认 `enabled=0`。
2. **查看详情**：按 ID 查询完整字段信息，含 `has_refs` 实时计算值（不缓存）。
3. **编辑字段**：必须先停用。name 不可改，其余均可改。乐观锁防并发。
4. **软删除字段**：必须先停用 + 无引用。事务内 FOR SHARE 兜底。reference 类型删除时自动清理它对子字段的引用。
5. **分页列表**：支持 label 模糊搜索 + type/category/enabled 精确筛选，覆盖索引不回表。
6. **标识唯一性校验**：创建前前端实时调 check-name 接口（含软删除记录也算占用）。

### 3.2 类型系统

7. **类型枚举**：`integer` / `float` / `string` / `bool` / `select` / `reference`。类型值来自字典表 `field_type` 组，由 `DictCache` 内存缓存。
8. **类型校验**：创建/编辑时通过 `dictCache.Exists(field_type, typ)` 校验类型存在性。

### 3.3 约束配置

9. **按类型约束**：
   - `integer` / `float`：`min` / `max`（float 额外支持 `precision > 0`）
   - `string`：`minLength` / `maxLength` / `pattern`
   - `select`：`options`（选项数组） / `minSelect` / `maxSelect`
   - `reference`：`refs`（子字段 ID 列表）
   - `bool` / `boolean`：无约束
10. **约束自洽校验**：Create 和 Update 均在写 DB **之前**调 `util.ValidateConstraintsSelf(type, constraints, errcode.ErrBadRequest)`，任一项违反即返回 40000。reference 类型的 refs 由 `validateReferenceRefs` 单独处理，不走此函数。覆盖的规则：
    - `integer` / `float`：`min <= max`
    - `float`：`precision > 0`（0 和负数都拒绝）
    - `string`：`minLength <= maxLength`，`minLength >= 0`，`maxLength >= 0`
    - `select`：`options` 非空，所有 `option.value` 不重复且非空字符串，`minSelect <= maxSelect`，`minSelect >= 0`
    - `bool` / `boolean`：无任何约束

### 3.4 引用追踪与保护

11. **引用追踪**：通过 `field_refs` 表记录所有引用关系（联合主键 `field_id + ref_type + ref_id`，INSERT IGNORE 幂等）。
12. **编辑保护 — 类型不可改**：`HasRefs` 为 true 且类型变更时，返回 40006。
13. **编辑保护 — 约束只能放宽**：`HasRefs` 为 true 且类型未变时，调 `util.CheckConstraintTightened` 检查，收紧返回 40007。
14. **删除保护**：事务内 `HasRefsTx(FOR SHARE)` 检查，有引用返回 40005。
15. **引用详情 API**：返回三类引用方（template/field/fsm），handler 层跨模块补齐 label。

### 3.5 reference 字段特殊逻辑

16. **refs 非空校验**：reference 类型字段的 `constraints.refs` 不能为空（40017）。
17. **目标存在性**：每个 refID 对应的字段必须存在（40014）。
18. **新增 ref 限制**：新增的 ref 必须启用（40013）+ 非 reference 类型（40016）。存量 ref 不重新校验（"存量不动"）。
19. **循环引用检测**：DFS 遍历引用链，检测到环返回 40009。
20. **引用关系同步**：创建/编辑时 diff 新旧 refs，事务内增删 field_refs 记录。
21. **删除时清理**：reference 字段删除时 `RemoveBySource(field, selfID)` 清理它对子字段的引用。

### 3.6 BB Key 暴露与保护

22. **BB Key 暴露**：`properties.expose_bb=true` 的字段，其 name 成为黑板 Key，可被 FSM 条件树引用。
23. **BB Key 取消保护**：编辑时若 `expose_bb` 从 true 变 false，检查 `field_refs WHERE ref_type='fsm'`，有引用返回 40008。
24. **FSM BB Key 同步**：FSM 创建/编辑时由 FSM handler 调 `SyncFsmBBKeyRefs` 维护引用关系；FSM 删除时调 `CleanFsmBBKeyRefs` 清理。

### 3.7 启用/停用

25. **切换启用状态**：乐观锁写入，版本号不匹配返回 40010。
26. **停用不断引用**：停用只影响"能否被新引用"，已有引用保持不动。

---

## 4. 引用关系

| 引用方 | ref_type | 触发写入时机 | 触发删除时机 |
|--------|----------|-------------|-------------|
| 模板 | `template` | 模板创建/编辑时勾选字段 → `AttachToTemplateTx` | 模板编辑移除字段/模板删除 → `DetachFromTemplateTx` |
| reference 字段 | `field` | 字段创建/编辑设置 refs → `syncFieldRefs` | 字段编辑移除 ref / 字段删除 → `syncFieldRefs` / `RemoveBySource` |
| FSM 条件 | `fsm` | FSM 创建/编辑条件树中引用 BB Key → `SyncFsmBBKeyRefs` | FSM 编辑移除 Key / FSM 删除 → `SyncFsmBBKeyRefs` / `CleanFsmBBKeyRefs` |

**引用方向**：`field_refs.field_id` = 被引用的字段 ID，`ref_id` = 引用方 ID。即 "谁引用了我"。

---

## 5. API 端点（8 个）

所有端点均为 `POST /api/v1/fields/<action>`，请求/响应体均为 JSON。

### 5.1 POST /fields/list

分页列表，支持组合筛选。

**请求**：
```json
{
  "label": "生命",        // 可选，模糊搜索
  "type": "integer",      // 可选，精确筛选
  "category": "basic",    // 可选，精确筛选
  "enabled": true,        // 可选，null=不筛选
  "page": 1,
  "page_size": 20
}
```

**响应**：
```json
{
  "code": 0,
  "data": {
    "items": [
      {
        "id": 1, "name": "hp", "label": "生命值",
        "type": "integer", "category": "basic",
        "enabled": true, "created_at": "...",
        "type_label": "整数", "category_label": "基础属性"
      }
    ],
    "total": 42, "page": 1, "page_size": 20
  }
}
```

### 5.2 POST /fields/create

创建字段，默认 `enabled=0`。

**请求**：
```json
{
  "name": "hp",
  "label": "生命值",
  "type": "integer",
  "category": "basic",
  "properties": { "description": "NPC生命值", "expose_bb": true, "default_value": 100, "constraints": { "min": 0, "max": 9999 } }
}
```

**响应**：
```json
{ "code": 0, "data": { "id": 1, "name": "hp" } }
```

**错误码**：40001 标识已存在 / 40002 标识格式错误 / 40003 类型不存在 / 40004 分类不存在 / 40017 refs 为空 / 40014 引用字段不存在 / 40013 引用已停用字段 / 40016 嵌套引用 / 40009 循环引用

### 5.3 POST /fields/detail

字段详情，含 `has_refs` 实时计算。

**请求**：`{ "id": 1 }`

**响应**：
```json
{
  "code": 0,
  "data": {
    "id": 1, "name": "hp", "label": "生命值",
    "type": "integer", "category": "basic",
    "properties": { ... },
    "enabled": true, "has_refs": true,
    "version": 3, "created_at": "...", "updated_at": "..."
  }
}
```

**错误码**：40011 字段不存在

### 5.4 POST /fields/update

编辑字段，乐观锁。name 不可改。

**请求**：
```json
{
  "id": 1,
  "label": "生命值（改名）",
  "type": "integer",
  "category": "basic",
  "properties": { ... },
  "version": 3
}
```

**响应**：`{ "code": 0, "data": "保存成功" }`

**错误码**：40015 未停用不可编辑 / 40006 被引用不可改类型 / 40007 被引用不可收紧约束 / 40008 BB Key 被 FSM 引用不可关闭暴露 / 40010 版本冲突 / 40003 类型不存在 / 40004 分类不存在 / 40017/40014/40013/40016/40009（reference 类型校验）

### 5.5 POST /fields/delete

软删除，必须先停用 + 无引用。

**请求**：`{ "id": 1 }`

**响应**：
```json
{ "code": 0, "data": { "id": 1, "name": "hp", "label": "生命值" } }
```

**错误码**：40012 未停用不可删除 / 40005 被引用不可删除 / 40011 字段不存在

### 5.6 POST /fields/check-name

标识唯一性校验（含软删除记录）。**先在 handler 层 `checkName()` 做格式/长度校验，再到 service 层查 DB**，确保非法 name（大写、空格、`BAD_FORMAT` 之类）直接返回 40002，不会被误判为"可用"。

**请求**：`{ "name": "hp" }`

**响应**：
```json
{ "code": 0, "data": { "available": true, "message": "该标识可用" } }
```

**错误码**：40002 标识格式不合法（空字符串、大写字母、超 64 字符、含特殊字符都命中）

### 5.7 POST /fields/toggle-enabled

切换启用/停用。

**请求**：`{ "id": 1, "enabled": true, "version": 3 }`

**响应**：`{ "code": 0, "data": "操作成功" }`

**错误码**：40010 版本冲突 / 40011 字段不存在

### 5.8 POST /fields/references

字段引用详情，返回所有引用方。

**请求**：`{ "id": 1 }`

**响应**：
```json
{
  "code": 0,
  "data": {
    "field_id": 1,
    "field_label": "生命值",
    "templates": [
      { "ref_type": "template", "ref_id": 10, "label": "战士模板" }
    ],
    "fields": [
      { "ref_type": "field", "ref_id": 5, "label": "战斗属性组" }
    ],
    "fsms": [
      { "ref_type": "fsm", "ref_id": 20, "label": "巡逻状态机" }
    ]
  }
}
```

**错误码**：40011 字段不存在

---

## 6. "对新隐藏、对旧保留"生命周期

### 场景 1：字段被模板引用后停用

- 模板详情页继续显示该字段（灰色标记停用）。
- 新建/编辑模板时，字段下拉选项不显示该字段（`enabled=true` 筛选）。
- 该模板的 NPC 已填的值保持不动。

### 场景 2：字段被 reference 字段引用后停用

- reference 字段的 refs 列表中继续包含该字段。
- 编辑 reference 字段时，新增 ref 不允许选择已停用字段（40013）；但若该 ref 是存量（旧 refs 中已有），保持不动不重新校验。

### 场景 3：expose_bb 的字段被 FSM 条件引用后停用

- FSM 条件中已配置的 BB Key 不受影响，继续生效。
- 取消 expose_bb 时若存在 FSM 引用，返回 40008 阻止。
- 必须先在 FSM 编辑中移除该 Key 的条件引用，再取消暴露。

### 场景 4：尝试删除被引用的字段

- 前端先调 references API 展示引用详情，用户确认是否继续。
- 后端 `HasRefsTx(FOR SHARE)` 在事务内做最终判断：有引用返回 40005。
- 确保前端展示和后端判断之间不存在 TOCTOU 竞态。
