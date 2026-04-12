# 扩展字段 Schema 管理前端 — 设计方案

## 方案描述

### 整体思路

复用现有 FieldList / FieldForm 的 CRUD 模式，新增 `EventTypeSchemaList.vue` 和 `EventTypeSchemaForm.vue` 两个页面。侧边栏在"事件类型"下方新增"扩展字段"菜单项。

### 关键设计决策

#### 1. 无 detail API —— 用 list 充当 detail

后端只有 5 个 API，**没有 detail 接口**。但 list 接口返回完整 `EventTypeSchemaFull`（含 version），且数据量 < 100 条无分页。

- **列表页 toggle**：list 返回 version，可直接使用（不违反红线——红线说"列表接口**可能**不返回 version"，此处确认返回）
- **编辑/查看页加载**：调用 `schemaList()`（不传 enabled 过滤）获取全量，按 `id` 找到目标项填充表单
- 若后续数据量增长或需要独立 detail 接口，是向前兼容的改动

#### 2. 无 checkName API —— 提交时处理 42020

字段管理和事件类型有 `checkName` 接口做 blur 唯一性校验。Schema 后端没有该接口。

- **blur 时**：只做本地格式校验（`/^[a-z][a-z0-9_]*$/`）
- **提交时**：若后端返回 42020（`ErrExtSchemaNameExists`），将 `nameStatus` 设为 `taken` 并显示错误
- 不新增后端接口，符合"纯前端改动"原则

#### 3. 约束面板复用方式

EventTypeForm.vue 已有按 `field_type` 渲染扩展字段值的逻辑，但那是**使用**扩展字段（填值）。Schema 表单需要的是**定义**约束（设置 min/max/options 等）。

现有 FieldForm.vue 中的约束组件（`FieldConstraintInteger`、`FieldConstraintString`、`FieldConstraintSelect`）是为字段管理设计的，但约束结构一致（同用 `service/constraint/validate.go`）。直接复用这些组件：

| field_type | 约束组件 | 说明 |
|-----------|---------|------|
| `int` | `FieldConstraintInteger` | min/max，typeName="int" |
| `float` | `FieldConstraintInteger` | min/max，typeName="float" |
| `string` | `FieldConstraintString` | minLength/maxLength |
| `bool` | 无约束面板 | 灰色提示"布尔类型无需约束配置" |
| `select` | `FieldConstraintSelect` | options 列表 |

注意：字段管理的类型名是 `integer`/`float`，而 Schema 的类型名是 `int`/`float`。`FieldConstraintInteger` 组件通过 `typeName` prop 区分，需确认兼容性。

#### 4. 默认值输入

默认值控件与 EventTypeForm.vue 中扩展字段值渲染逻辑一致，按 `field_type` 动态渲染：
- `int`：`el-input-number`（整数，min/max 来自约束）
- `float`：`el-input-number`（step=0.1，min/max 来自约束）
- `string`：`el-input`
- `bool`：`el-switch`
- `select`：`el-select`（options 来自约束面板的实时值）

#### 5. 路由和菜单

路由结构遵循现有模式（route meta flags）：

```
/event-type-schemas                → EventTypeSchemaList
/event-type-schemas/create         → EventTypeSchemaForm (isCreate=true)
/event-type-schemas/:id/view       → EventTypeSchemaForm (isView=true)
/event-type-schemas/:id/edit       → EventTypeSchemaForm (isCreate=false)
```

侧边栏 `activeMenu` 逻辑当前按 `route.path.split('/')[1]` 取一级路径。`event-type-schemas` 的一级路径是 `event-type-schemas`，与 `event-types` 不同，菜单高亮不会冲突。

#### 6. EnabledGuardDialog 扩展

现有 `EnabledGuardDialog` 支持 `'field' | 'template' | 'event-type'` 三种 entityType。需新增 `'event-type-schema'` 类型：
- 需要在 `EnabledGuardDialog.vue` 中新增 `event-type-schema` 的文案、API 调度和路由跳转
- 需要在 `eventTypes.ts` 中补充 schema 的 detail-like 和 toggleEnabled API

### API 层补充

在 `frontend/src/api/eventTypes.ts` 中补充：

```typescript
// 错误码
export const EXT_SCHEMA_ERR = {
  NAME_EXISTS:         42020,
  NAME_INVALID:        42021,
  NOT_FOUND:           42022,
  TYPE_INVALID:        42024,
  CONSTRAINTS_INVALID: 42025,
  DEFAULT_INVALID:     42026,
  DELETE_NOT_DISABLED: 42027,
  VERSION_CONFLICT:    42030,
  EDIT_NOT_DISABLED:   42031,
} as const

// API 函数
schemaList: (params?: { enabled?: boolean }) =>
  request.post('/event-type-schema/list', params || {})

schemaCreate: (data: CreateSchemaRequest) =>
  request.post('/event-type-schema/create', data)

schemaUpdate: (data: UpdateSchemaRequest) =>
  request.post('/event-type-schema/update', data)

schemaDelete: (id: number) =>
  request.post('/event-type-schema/delete', { id })

schemaToggleEnabled: (id: number, enabled: boolean, version: number) =>
  request.post('/event-type-schema/toggle-enabled', { id, enabled, version })
```

### 数据流

```
列表页                          表单页（创建/编辑/查看）
┌────────────┐                ┌────────────────────┐
│ schemaList │──table─────────│ schemaList → 按 id  │
│ (全量)     │  toggle用version│ 找到目标项填充 form │
│            │                │                    │
│ 筛选: enabled│               │ 提交: create/update│
│ 操作: toggle│               │ 约束面板: 复用      │
│       delete│               │ FieldConstraint*   │
│       view  │               │                    │
│       edit  │               └────────────────────┘
└────────────┘
```

## 方案对比

### 方案 A（选定）：用 list 充当 detail

- 优点：零后端改动，数据量 < 100 性能无影响
- 缺点：每次进入编辑页多拉了不需要的数据

### 方案 B（不选）：新增后端 detail 接口

- 优点：标准 CRUD 模式，编辑页只拉一条数据
- 缺点：需要后端改动（新增 handler/service/store 方法、注册路由），违反"纯前端"原则
- **不选理由**：数据量 < 100，list 充当 detail 完全可接受，不值得为此增加后端改动

### 方案 C（不选）：router state 传递数据

- 优点：不需要额外 API 调用
- 缺点：页面刷新丢失数据，用户体验差
- **不选理由**：红线要求数据一致性，刷新后必须能正常工作

## 红线检查

### 前端红线 (`docs/development/standards/red-lines/frontend.md`)

| 红线 | 合规 | 说明 |
|------|------|------|
| 禁止数据源污染 | ✅ | 列表筛选不变，list API 带 enabled 参数做后端过滤 |
| 禁止放行无效输入 | ✅ | field_type 用 el-select；field_name blur 校验格式 |
| 禁止 URL 编码遗漏 | ✅ | field_name 不含特殊字符（`/^[a-z][a-z0-9_]*$/`），ID 为纯数字 |
| 禁止 JSON key 各写各的 | ✅ | constraints 的 key 名沿用 FieldConstraint* 组件，与 constraint_schema 一致 |
| 禁止跳过类型检查 | ✅ | 提交前跑 `npx vue-tsc --noEmit` |
| 禁止 reactive 不带泛型 | ✅ | FormState 接口显式标注 |
| 禁止事件回调省略类型 | ✅ | el-switch @change 标注 `(val: string | number | boolean)` |

### 通用红线 (`docs/development/standards/red-lines/general.md`)

| 红线 | 合规 | 说明 |
|------|------|------|
| 禁止静默降级 | ✅ | API 错误由拦截器 toast，业务错误按 code 处理 |
| 禁止过度设计 | ✅ | 无新抽象层，复用现有约束组件 |

### ADMIN 专属红线 (`docs/development/admin/red-lines.md`)

| 红线 | 合规 | 说明 |
|------|------|------|
| 禁止暴露技术细节给策划 | ✅ | 表单显示中文标签 + 灰色提示文字 |
| 禁止表单对非技术用户不友好 | ✅ | 空数据 el-empty + 引导按钮；删除确认明确对象名；blur 校验名称格式 |
| 禁止启用/禁用不弹确认 | ✅ | toggle 弹 ElMessageBox.confirm |
| 禁止 Toggle 直接用列表 version | ✅ | Schema list 确认返回 version，可直接使用（不同于字段/模板列表不返回 version 的情况） |
| 禁止危险操作引导不一致 | ✅ | 启用状态编辑/删除走 EnabledGuardDialog |
| 禁止侧栏用不可折叠容器 | ✅ | 新菜单项用 el-menu-item 放在现有 el-sub-menu 内 |
| 禁止硬编码错误码 | ✅ | 定义 EXT_SCHEMA_ERR 常量 |

## 扩展性影响

**正面**：Schema 管理 UI 让策划能自助定义事件类型的扩展字段，是"新增表单字段"扩展轴的管理入口。

**无负面影响**：不改动已有模块代码的业务逻辑，只是新增页面 + 修改路由/菜单/API 层。

## 依赖方向

```
EventTypeSchemaList.vue ──→ api/eventTypes.ts ──→ request.ts
EventTypeSchemaForm.vue ──→ api/eventTypes.ts
                        ──→ components/FieldConstraint*.vue
                        ──→ components/EnabledGuardDialog.vue

AppLayout.vue（仅新增一行 el-menu-item）
router/index.ts（仅新增 4 条路由）
```

方向单一向下，无循环依赖。

## 陷阱检查 (`docs/development/standards/dev-rules/frontend.md`)

| 检查项 | 结论 |
|--------|------|
| el-select value 类型匹配 | field_type 用 string v-model，options value 也是 string ✅ |
| el-form-item prop 匹配 model | 所有 prop 严格对应 form 字段名 ✅ |
| route-switch stale state | AppLayout 已用 `:key="route.fullPath"` ✅ |
| reactive toRefs | 不需要解构 reactive ✅ |
| v-for unique key | 列表用 `row.id`，schema 用 `ext.field_name` ✅ |
| ElMessage 手动导入 CSS | 已在 main.ts 全量引入 element-plus CSS ✅ |

## 配置变更

无。不需要新增/修改配置文件。

## 测试策略

**手工测试**（前端页面无单元测试框架）：

1. 列表页：加载 → 筛选切换 → 重置 → 空数据引导
2. 创建：填写表单 → 切换类型约束面板变化 → 提交 → 列表刷新
3. 创建重名：提交返回 42020 → 显示"标识符已存在"
4. 编辑：field_name/field_type 灰显 → 修改 label/约束 → 提交
5. 查看：所有字段禁用 → 无提交按钮
6. 启用/禁用：确认弹窗 → toggle → 刷新
7. 删除启用中：EnabledGuardDialog 拦截
8. 删除已禁用：确认弹窗 → 删除 → 刷新
9. 版本冲突：模拟并发编辑 → 42030 提示
10. `npx vue-tsc --noEmit` 通过
