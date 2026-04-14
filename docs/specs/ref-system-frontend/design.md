# ref-system-frontend — 设计方案

## 方案描述

### A. API 类型对齐

**templates.ts**：
- `TemplateListItem`：删除 `ref_count: number`
- `TemplateDetail`：删除 `ref_count: number`

**fields.ts**（已部分完成）：
- `FieldListItem`：已删除 `ref_count`
- `FieldDetail`：已新增 `has_refs: boolean`
- `ReferenceDetail`：新增 `fsms: ReferenceItem[]`

**eventTypes.ts**：
- 新增 `SchemaReferenceItem` 和 `SchemaReferenceDetail` 类型
- 新增 `eventTypeApi.schemaReferences(id)` 方法

### B. FieldList 删除流程重构

当前：`row.ref_count > 0` → 弹引用详情 + 警告
改为：

```
handleDelete(row):
  1. row.enabled → EnabledGuardDialog
  2. else → fieldApi.references(row.id)
     → refs.templates.length + refs.fields.length + refs.fsms.length > 0
       → 弹引用详情弹窗 + 提示无法删除
     → 无引用 → ElMessageBox 确认 → fieldApi.delete(row.id)
       → 成功 → toast + fetchList
       → REF_DELETE(40005) → 弹引用详情（后端兜底）
```

引用详情弹窗保留（原有内联 dialog），新增 FSM 引用区域。

### C. FieldForm has_refs 驱动

- `refCount` ref 变量改名为 `hasRefs: ref(false)`
- `loadFieldDetail` 中 `hasRefs.value = data.has_refs`
- 类型选择器：`:disabled="isView || (!isCreate && hasRefs)"` + 警告 "被引用中，无法更改类型"
- 约束组件：`:restricted="hasRefs"`

### D. TemplateList 简化

- 删除"被引用数"列 (`<el-table-column label="被引用数">`)
- 删除 `TemplateReferencesDialog` 组件引用和 `refsRef`
- 删除 `handleShowRefs` 方法
- `handleDelete`：移除 `row.ref_count > 0` 分支，直接走确认删除 → API
- 保留 `REF_DELETE` catch 分支作为占位（NPC 上线后启用）

### E. TemplateForm 简化

- 删除 `isLocked` computed（`isEdit && template.ref_count > 0`）
- 删除 header 中"被 N 个 NPC 引用" tag
- 删除 `<el-alert>` 锁定提示
- 字段选择/已选字段配置卡片：`:disabled` 去掉 `isLocked`，保留 `isView`
- 删除"🔒 已锁定" tag

### F. TemplateRefPopover 过滤

**新增 props**：
- `filterDisabled: boolean`（默认 `false`）

**TemplateFieldPicker**：
- 新增 `mode: 'create' | 'edit' | 'view'` prop
- 传给 TemplateRefPopover：`:filter-disabled="mode === 'create'"`

**TemplateForm**：
- 传给 TemplateFieldPicker：`:mode="mode"`（mode 已有 prop）

**TemplateRefPopover.open 逻辑**：
- 拉子字段详情后，若 `filterDisabled && !detail.enabled` → 跳过
- 若 `!filterDisabled && !detail.enabled` → 保留但标记 `enabled: false`
- `RefFieldItem` 新增 `enabled: boolean` 字段
- 模板中停用子字段展示：标灰 + `<el-tag size="small" type="info">已停用</el-tag>`

### G. EnabledGuardDialog 简化

- `GuardEntity` 接口删除 `ref_count: number`
- 删除 `refCountPass` computed
- 删除"没有 XX 在使用该字段（当前被引用：N）"前置条件
- 删除场景的 `guard-box` 只显示"已禁用"一个条件

### H. EventType/Schema 列表

- `EventTypeList.vue`：entity 对象删除 `ref_count: 0`
- `EventTypeSchemaList.vue`：
  - entity 对象删除 `ref_count: 0`
  - 新增删除引用检查：已禁用 → 调 `eventTypeApi.schemaReferences(id)` → 有引用弹详情 → 无引用确认删除

### I. 字段引用详情弹窗扩展

FieldList 内联的引用详情 dialog 新增 FSM 区域：

```html
<!-- FSM 引用 -->
<div class="ref-section" style="margin-top: 16px">
  <p class="ref-subtitle">
    FSM 引用（{{ refDialog.fsms.length }} 个状态机引用了该 BB Key）：
  </p>
  <el-table v-if="refDialog.fsms.length > 0" :data="refDialog.fsms" size="small">
    <el-table-column prop="label" label="状态机名称" />
    <el-table-column prop="ref_type" label="类型" width="100" />
  </el-table>
  <p v-else class="ref-empty">暂无 FSM 引用</p>
</div>
```

## 备选方案

### 字段删除流程：后端驱动 vs 前端预检查

**备选**：不调 references API，直接 delete → 后端 40005 → 再拉引用详情

**不选**：用户先确认了删除，再被告知不能删，体验差。先查引用再决定是否弹确认更自然。

### TemplateRefPopover 过滤：前端过滤 vs 后端接口

**备选**：后端新增 `/fields/sub-fields?enabled=true` 接口

**不选**：detail 已返回 enabled 字段，前端一行 `if` 即可过滤。不必增加 API 面积。

## 红线检查

| 红线 | 合规 | 说明 |
|------|------|------|
| 禁止数据源污染 | OK | 不 filter ref 数据，用 computed 或条件渲染 |
| 禁止 el-form disabled 被覆盖 | OK | FieldForm `:disabled="isView \|\| (!isCreate && hasRefs)"` 包含 isView |
| 禁止跳过 vue-tsc | OK | 完成后执行 `npx vue-tsc --noEmit` |
| 禁止业务错误码漏处理 | OK | FieldList delete catch 处理 REF_DELETE；Schema delete catch 处理 ErrExtSchemaRefDelete |
| 禁止 EnabledGuardDialog 私有副本 | OK | 继续用泛型组件，只删 ref_count 相关 |
| 禁止 reactive 无显式泛型 | OK | 不新增 reactive |

## 扩展性影响

**正面**：EnabledGuardDialog 去掉 ref_count 后更简洁，新模块接入更轻。

## 陷阱检查（frontend dev-rules）

- reactive 泛型：不新增 reactive
- v-for key：引用详情弹窗 FSM 列表用 `ref_id` 作 key
- 列表 API 缺字段：ref_count 已从类型移除，不会 undefined 访问
- callback 类型注解：不新增需要类型的回调

## 配置变更

无。

## 测试策略

- `npx vue-tsc --noEmit` 通过
- 浏览器手动验证：
  - 字段列表无引用数列 + 删除被引用字段弹详情（含 FSM 区域）
  - 字段编辑页 has_refs 控制类型锁定 + restricted
  - 模板列表无引用数列 + 删除无引用检查
  - 模板编辑页无锁定逻辑
  - 模板新建 reference 子字段过滤停用
  - 模板编辑 reference 子字段停用标灰
  - EnabledGuardDialog 只显示"已禁用"条件
  - 扩展字段删除引用检查
