# ref-system-frontend — 需求分析

## 动机

后端已完成引用系统重构（ref-system-backend spec）：移除 ref_count 列、新增 field_refs(fsm)/schema_refs 引用追踪、Field detail 返回 has_refs、扩展字段新增 references API。前端需要同步对齐：

1. **ref_count 展示残留**：列表页仍有"被引用数"列（字段/模板），表单页仍有基于 ref_count 的锁定逻辑
2. **删除流程不一致**：字段删除仍依赖 row.ref_count 前端预检查，应改为调 references API
3. **停用字段泄露**：模板新建时 reference 子字段选择器展示停用字段
4. **扩展字段缺保护**：扩展字段列表删除时无引用检查
5. **FSM 引用不展示**：字段引用详情弹窗不展示 FSM 引用方

不做的话：前端与后端 API 不一致，TS 类型报错（ref_count 字段已从响应中移除），用户体验不连贯。

## 优先级

**高**。后端已移除 ref_count 字段，前端 TS 类型不匹配会导致构建警告和运行时 undefined 访问。

## 预期效果

### 场景 A：字段管理

**A1**：字段列表无"被引用数"列。
**A2**：字段删除流程：点删除 → 已启用走 EnabledGuardDialog → 已禁用调 `fieldApi.references(id)` → 有引用弹引用详情弹窗 + 提示无法删除 → 无引用弹确认删除 → 调 delete API。
**A3**：字段引用详情弹窗新增"FSM 引用"区域，展示引用该 BB Key 的 FSM 配置列表。
**A4**：字段编辑页：类型选择器根据 `has_refs`（非 ref_count）控制 disabled + 警告；约束组件 `restricted` 根据 `has_refs` 控制。

### 场景 B：模板管理

**B1**：模板列表无"被引用数"列，无 `handleShowRefs` 和 `TemplateReferencesDialog`。
**B2**：模板删除流程：停用后确认删除 → 调 delete API（NPC 未上线，无引用检查）。
**B3**：模板编辑页无 `isLocked` 逻辑（无"被 N 个 NPC 引用" tag、无锁定 alert、字段选择/配置不因引用而锁定）。

### 场景 C：模板新建 reference 子字段过滤

**C1**：模板新建时点 reference 字段弹出子字段选择器，只展示**启用的**子字段。
**C2**：模板编辑/查看时，子字段选择器展示全部子字段，停用的标灰 + "已停用" tag。

### 场景 D：EnabledGuardDialog

**D1**：`GuardEntity` 接口无 `ref_count` 字段。
**D2**：删除场景只显示"已禁用"一个前置条件，不显示引用数条件。

### 场景 E：事件类型 / 扩展字段

**E1**：EventTypeList / EventTypeSchemaList 传给 EnabledGuardDialog 的 entity 无 `ref_count: 0` 占位。
**E2**：扩展字段列表展示 `has_refs` 信息（列表 API 已返回）。
**E3**：扩展字段删除流程：点删除 → 已启用走 Guard → 已禁用调 schema references API → 有引用弹详情 → 无引用确认删除。
**E4**：事件类型表单中禁用的扩展字段区域展示对齐（已实现标灰只读，确认无需改动即可）。

### 场景 F：API 类型对齐

**F1**：`FieldListItem` 无 `ref_count` 字段（已完成）。
**F2**：`FieldDetail` 新增 `has_refs: boolean`（已完成）。
**F3**：`TemplateListItem`、`TemplateDetail` 无 `ref_count` 字段。
**F4**：`ReferenceDetail` 新增 `fsms` 数组。
**F5**：新增 `SchemaReferenceDetail` 类型 + `eventTypeSchemaApi.references(id)` 方法。

## 依赖分析

**依赖**：后端 ref-system-backend spec 全部完成（已完成并推送）。

**被依赖**：无。

## 改动范围

### API 层（约 3 文件）

| 文件 | 改动 |
|------|------|
| `api/fields.ts` | 确认 ref_count 已移除 + has_refs 已加（✅ 已完成） |
| `api/templates.ts` | 移除 ref_count 字段 |
| `api/eventTypes.ts` | 新增 SchemaReferenceDetail 类型 + references 方法 |

### Views 层（约 6 文件）

| 文件 | 改动 |
|------|------|
| `views/FieldList.vue` | 删除引用详情弹窗内联 dialog → 改为调 references API 驱动删除 |
| `views/FieldForm.vue` | ref_count → has_refs 驱动 |
| `views/TemplateList.vue` | 移除引用数列 + 引用详情弹窗 |
| `views/TemplateForm.vue` | 移除 isLocked |
| `views/EventTypeList.vue` | 移除 ref_count 占位 |
| `views/EventTypeSchemaList.vue` | 移除 ref_count 占位 + 新增删除引用检查 |

### Components 层（约 3 文件）

| 文件 | 改动 |
|------|------|
| `components/EnabledGuardDialog.vue` | 移除 ref_count |
| `components/TemplateRefPopover.vue` | 新建过滤停用 + 编辑保留标灰 |
| `components/TemplateReferencesDialog.vue` | 可能删除（模板引用详情 NPC 未上线无用） |

## 扩展轴检查

- **新增配置类型**：正面。移除 ref_count 后 EnabledGuardDialog 更简洁，新模块只需加 entityType case
- **新增表单字段**：不涉及

## 验收标准

### API 类型

- **R1**：`TemplateListItem` 和 `TemplateDetail` 无 `ref_count` 字段
- **R2**：`fields.ts` 的 `ReferenceDetail` 新增 `fsms: ReferenceItem[]`
- **R3**：`eventTypes.ts` 新增 `SchemaReferenceDetail` 类型和 `eventTypeSchemaApi.references(id)` 方法

### 字段管理

- **R4**：FieldList 无"被引用数"列
- **R5**：FieldList 删除流程：禁用字段点删除 → 调 references API → 有引用弹详情 → 无引用确认删除
- **R6**：FieldForm 类型选择器根据 `has_refs` 控制 disabled
- **R7**：FieldForm 约束组件 `restricted` 根据 `has_refs` 控制
- **R8**：字段引用详情弹窗展示 FSM 引用方区域

### 模板管理

- **R9**：TemplateList 无"被引用数"列
- **R10**：TemplateList 无 TemplateReferencesDialog 和 handleShowRefs
- **R11**：TemplateForm 无 isLocked 逻辑（无引用 tag、无锁定 alert、字段不因引用锁定）

### TemplateRefPopover

- **R12**：模板新建时 reference 子字段选择器只展示启用子字段
- **R13**：模板编辑时 reference 子字段选择器展示全部子字段，停用标灰

### EnabledGuardDialog

- **R14**：GuardEntity 接口无 ref_count
- **R15**：删除场景只显示"已禁用"前置条件

### 事件类型 / 扩展字段

- **R16**：EventTypeList entity 无 ref_count 占位
- **R17**：EventTypeSchemaList entity 无 ref_count 占位
- **R18**：扩展字段删除流程：调 schema references API → 有引用弹详情 → 无引用确认删除

### 构建

- **R19**：`npx vue-tsc --noEmit` 通过
- **R20**：浏览器手动验证全部场景无 JS 错误

## 不做什么

- **不改后端**：后端已在 ref-system-backend spec 完成
- **不做扩展字段编辑页的约束锁定 UI**：后端已有保护（42028），前端暂不做 restricted 展示（扩展字段编辑页结构与字段编辑页不同，后续按需补）
- **不做 FSM 条件编辑器的 BB Key 下拉过滤**：FSM 前端条件编辑器当前用自由输入，BB Key 下拉选择是独立需求
- **不做事件类型删除时的引用检查 UI**：FSM/BT 引用事件类型的追踪后端未实现
