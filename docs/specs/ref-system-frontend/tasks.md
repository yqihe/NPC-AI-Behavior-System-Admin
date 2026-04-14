# ref-system-frontend — 任务拆解

## [x] T1: API 类型对齐 — templates.ts + fields.ts + eventTypes.ts (R1, R2, R3)

**文件**：
- `frontend/src/api/templates.ts`
- `frontend/src/api/fields.ts`
- `frontend/src/api/eventTypes.ts`

**做完是什么样**：
- `TemplateListItem`、`TemplateDetail` 无 `ref_count` 字段
- `ReferenceDetail` 新增 `fsms: ReferenceItem[]`
- `eventTypes.ts` 新增 `SchemaReferenceItem`、`SchemaReferenceDetail` 类型
- `eventTypeApi.schemaReferences(id)` 方法可用
- `npx vue-tsc --noEmit` 无类型错误（可能有其他文件因 ref_count 移除报错，暂不管）

---

## [x] T2: EnabledGuardDialog 移除 ref_count (R14, R15)

**文件**：
- `frontend/src/components/EnabledGuardDialog.vue`

**做完是什么样**：
- `GuardEntity` 接口无 `ref_count` 字段
- `refCountPass` computed 删除
- 删除场景 `guard-box` 只显示"已禁用"条件（不显示引用数条件）
- `refTargetLabel` computed 可保留（后续可能被引用详情弹窗复用）

---

## [x] T3: EventTypeList + EventTypeSchemaList 移除 ref_count 占位 (R16, R17)

**文件**：
- `frontend/src/views/EventTypeList.vue`
- `frontend/src/views/EventTypeSchemaList.vue`

**做完是什么样**：
- 传给 EnabledGuardDialog 的 entity 对象无 `ref_count: 0` 属性
- 编译通过（EnabledGuardDialog 已不要求 ref_count）

---

## [x] T4: TemplateList 移除引用数列 + 引用详情弹窗 (R9, R10)

**文件**：
- `frontend/src/views/TemplateList.vue`

**做完是什么样**：
- 无"被引用数"列
- 无 `TemplateReferencesDialog` import 和 `refsRef`
- 无 `handleShowRefs` 方法
- `handleDelete` 无 `row.ref_count > 0` 分支，直接确认删除 → API
- `REF_DELETE` catch 保留作为占位

---

## [x] T5: TemplateForm 移除 isLocked (R11)

**文件**：
- `frontend/src/views/TemplateForm.vue`

**做完是什么样**：
- 无 `isLocked` computed
- header 无"被 N 个 NPC 引用" tag
- 无 `<el-alert>` 锁定提示
- 字段选择/配置卡 `:disabled` 只有 `isView`
- 无"🔒 已锁定" tag

---

## [x] T6: FieldList 删除流程重构 + 引用详情扩展 FSM (R4, R5, R8)

**文件**：
- `frontend/src/views/FieldList.vue`

**做完是什么样**：
- 无"被引用数"列（已完成）
- `handleDelete`：禁用字段 → 调 `fieldApi.references(row.id)` → 有引用弹详情 → 无引用确认删除
- 引用详情弹窗新增 FSM 引用区域（`refDialog.fsms`）
- `handleShowRefs` 加载 `res.data.fsms`
- 删除后 `REF_DELETE` catch 也弹引用详情

---

## [x] T7: FieldForm has_refs 驱动 (R6, R7)

**文件**：
- `frontend/src/views/FieldForm.vue`

**做完是什么样**：
- `refCount` ref 改为 `hasRefs = ref(false)`
- `loadFieldDetail` 中 `hasRefs.value = data.has_refs`
- 类型选择器 `:disabled="isView || (!isCreate && hasRefs)"`
- 警告文字 "被引用中，无法更改类型"（条件 `!isCreate && hasRefs`）
- 约束组件 `:restricted="hasRefs"`

---

## [x] T8: TemplateRefPopover 过滤停用子字段 (R12, R13)

**文件**：
- `frontend/src/components/TemplateRefPopover.vue`
- `frontend/src/components/TemplateFieldPicker.vue`
- `frontend/src/views/TemplateForm.vue`

**做完是什么样**：
- TemplateRefPopover 新增 `filterDisabled` prop
- `RefFieldItem` 新增 `enabled` 字段
- `filterDisabled=true` 时跳过 `enabled=false` 子字段
- `filterDisabled=false` 时停用子字段标灰 + "已停用" tag
- TemplateFieldPicker 新增 `mode` prop，据此传 `filterDisabled`
- TemplateForm 传 `mode` 给 TemplateFieldPicker

---

## T9: EventTypeSchemaList 删除引用检查 (R18)

**文件**：
- `frontend/src/views/EventTypeSchemaList.vue`

**做完是什么样**：
- `handleDelete`：已禁用 → 调 `eventTypeApi.schemaReferences(row.id)` → 有引用弹详情弹窗 → 无引用确认删除
- 新增引用详情弹窗（内联 dialog 或独立组件），展示引用该扩展字段的事件类型列表
- `ErrExtSchemaRefDelete`(42029) catch 也弹引用详情

---

## T10: 构建验证 (R19, R20)

**文件**：无新文件

**做完是什么样**：
- `npx vue-tsc --noEmit` 通过
- 浏览器手动验证全部场景无 JS 错误
