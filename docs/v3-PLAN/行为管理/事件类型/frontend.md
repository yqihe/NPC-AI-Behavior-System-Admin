# 事件类型管理 — 前端设计

> **实现状态**：规划中，前端代码尚未落地。以下内容基于后端已实现的 API 和现有设计文档规划。
> 通用前端规范见 `docs/development/standards/red-lines/frontend.md` 和 `dev-rules/frontend.md`。

---

## 1. 目录结构（规划中）

```
frontend/src/
├── api/
│   └── event-types.ts                    # 类型定义 + EVENT_TYPE_ERR(42001-42015) + EVENT_TYPE_SCHEMA_ERR(42020-42030) + API 函数
├── views/
│   ├── EventTypeList.vue                 # 列表页：筛选(display_name / perception_mode / enabled) + 分页 + toggle + 编辑删除守卫
│   ├── EventTypeForm.vue                 # 新建/编辑共用：系统字段 + 扩展字段(SchemaForm) + dirty 追踪
│   └── SchemaManagement.vue              # Schema 管理页主容器（含多 tab，事件类型扩展字段为其中一个）
├── components/
│   ├── EventTypeSchemaTab.vue            # 事件类型扩展字段 tab（列表 + 新建/编辑弹窗）
│   ├── SchemaForm.vue                    # 通用 Schema 驱动表单：接受 schema 数组 + 值对象 + dirty 追踪
│   ├── FormFieldInt.vue                  # SchemaForm 子组件：整数输入
│   ├── FormFieldFloat.vue               # SchemaForm 子组件：浮点输入
│   ├── FormFieldString.vue              # SchemaForm 子组件：字符串输入
│   ├── FormFieldBool.vue                # SchemaForm 子组件：布尔开关
│   ├── FormFieldSelect.vue              # SchemaForm 子组件：选择框
│   ├── ConstraintPanel.vue              # 按 field_type 动态渲染约束编辑器（复用字段管理的 FieldConstraint*.vue）
│   └── EnabledGuardDialog.vue           # 启用守卫（复用，entityType 泛型扩展 'event_type'）
├── stores/
│   ├── eventType.ts                     # 列表查询 / 详情 / 当前编辑对象 / 提交态
│   └── eventTypeSchema.ts              # 扩展字段 schema 列表 + 按 enabled 过滤 + reload 动作
└── router/index.ts                      # 新增 /event-types, /event-types/new, /event-types/:id/edit, /schema-management
```

复用已有组件：`EnabledGuardDialog.vue`、`FieldConstraintInteger.vue`、`FieldConstraintString.vue`、`FieldConstraintSelect.vue`。不复用 `FieldConstraintReference.vue`（扩展字段不支持 reference 类型）。

---

## 2. 页面路由（规划中）

| 路径 | 组件 | 说明 |
|---|---|---|
| `/event-types` | `EventTypeList.vue` | 列表页 |
| `/event-types/new` | `EventTypeForm.vue` | 新建页 |
| `/event-types/:id/edit` | `EventTypeForm.vue` | 编辑页（与新建共用，mode 区分） |
| `/schema-management` | `SchemaManagement.vue` | Schema 管理页（多 tab，事件类型扩展字段是其中一个） |

---

## 3. 组件树（规划中）

```
EventTypeList.vue
  └─ EnabledGuardDialog.vue               (复用，entityType: 'event_type')

EventTypeForm.vue (mode: 'create' | 'edit')
  ├─ 系统字段区域（硬编码表单项）
  │   name / display_name / perception_mode / range / default_severity / default_ttl
  └─ 扩展字段区域
      └─ SchemaForm.vue                    (通用组件，接受 schema 数组)
          ├─ FormFieldInt.vue
          ├─ FormFieldFloat.vue
          ├─ FormFieldString.vue
          ├─ FormFieldBool.vue
          └─ FormFieldSelect.vue

SchemaManagement.vue
  └─ EventTypeSchemaTab.vue
      ├─ EventTypeSchemaList.vue           (扩展字段列表)
      ├─ EventTypeSchemaForm.vue           (新建/编辑弹窗)
      └─ ConstraintPanel.vue              (复用 FieldConstraint*.vue)
```

---

## 4. 类型契约（规划中）

```ts
// --- api/event-types.ts ---

// 事件类型错误码（42001-42015，与 backend/internal/errcode/codes.go 对齐）
const EVENT_TYPE_ERR: Record<number, string> = {
  42001: '事件标识已存在（含已删除记录）',
  42002: '事件标识格式不合法，必须小写字母开头，只含小写字母/数字/下划线',
  42003: '感知模式必须是 visual / auditory / global 之一',
  42004: '默认威胁必须在 0-100 之间',
  42005: '默认 TTL 必须大于 0',
  42006: '传播范围不能小于 0',
  42007: '扩展字段的值不符合约束',
  42008: '当前事件类型仍被引用，不能删除',
  42010: '数据已被其他用户修改，请刷新后重试',
  42011: '事件类型不存在',
  42012: '请先停用此事件类型才能删除',
  42015: '请先停用此事件类型才能编辑',
}

// 扩展字段 Schema 错误码（42020-42030）
const EVENT_TYPE_SCHEMA_ERR: Record<number, string> = {
  42020: '扩展字段标识已存在',
  42021: '扩展字段标识格式不合法',
  42022: '扩展字段定义不存在',
  42023: '扩展字段已停用',
  42024: '扩展字段类型非法',
  42025: '约束配置不自洽',
  42026: '默认值不符合约束',
  42027: '请先停用此扩展字段才能删除',
  42030: '扩展字段已被其他用户修改，请刷新后重试',
}

// SchemaForm 扩展字段交互的核心类型
interface ExtensionFieldState {
  schema: EventTypeSchemaItem  // 从 extension_schema 获取
  value: unknown               // 当前值
  dirty: boolean               // dirty=false 使用默认值（浅灰占位），dirty=true 运营主动填写（黑色）
}
```

**SchemaForm 核心设计**：

- `dirty=false` 的扩展字段在提交时不进 payload，服务端使用自己的默认值
- `dirty=true` 的扩展字段写入 `config_json`，代表运营明确配置过
- UI 上 dirty=false 显示"默认: {value}"浅灰占位文字，dirty=true 显示黑色值 + "重置为默认"按钮

---

## 5. API 调用映射（规划中）

| UI 操作 | API 函数 | 后端端点 |
|---|---|---|
| 列表加载 / 筛选 / 翻页 | `eventTypeApi.list(params)` | `POST /api/v1/event-types/list` |
| 新建事件类型 | `eventTypeApi.create(data)` | `POST /api/v1/event-types/create` |
| 查看详情 | `eventTypeApi.detail(id)` | `POST /api/v1/event-types/detail` |
| 编辑事件类型 | `eventTypeApi.update(data)` | `POST /api/v1/event-types/update` |
| 删除事件类型 | `eventTypeApi.delete(id)` | `POST /api/v1/event-types/delete` |
| 标识符唯一性校验 | `eventTypeApi.checkName(name)` | `POST /api/v1/event-types/check-name` |
| 切换启用/停用 | `eventTypeApi.toggleEnabled(id, enabled, version)` | `POST /api/v1/event-types/toggle-enabled` |
| Schema 列表 | `eventTypeSchemaApi.list()` | `POST /api/v1/event-type-schemas/list` |
| Schema 新建 | `eventTypeSchemaApi.create(data)` | `POST /api/v1/event-type-schemas/create` |
| Schema 编辑 | `eventTypeSchemaApi.update(data)` | `POST /api/v1/event-type-schemas/update` |
| Schema 删除 | `eventTypeSchemaApi.delete(id)` | `POST /api/v1/event-type-schemas/delete` |

---

## 6. 错误码处理（规划中）

### 事件类型错误码

| 错误码 | UI 反馈 |
|---|---|
| 42001 事件标识已存在 | form 内联红字（同字段/模板的 nameStatus 模式） |
| 42002 标识格式不合法 | form 内联红字 |
| 42003 感知模式非法 | `ElMessage.error` toast（前端 select 已限制，兜底） |
| 42004 威胁值越界 | `ElMessage.error` toast（前端 slider 已限制，兜底） |
| 42005 TTL 非法 | `ElMessage.error` toast |
| 42006 范围非法 | `ElMessage.error` toast |
| 42007 扩展字段值违反约束 | `ElMessage.error` toast |
| 42008 被引用不能删除 | 列表删除时 toast 提示 |
| 42010 版本冲突 | `ElMessageBox.alert` 提示刷新 |
| 42011 不存在 | `ElMessage.error` + `router.push('/event-types')` |
| 42012 删除须先停用 | `EnabledGuardDialog` 前端拦截，兜底 toast |
| 42015 编辑须先停用 | `EnabledGuardDialog` 前端拦截，兜底 toast |

### 扩展字段 Schema 错误码

| 错误码 | UI 反馈 |
|---|---|
| 42020 标识已存在 | 弹窗 form 内联红字 |
| 42021 标识格式不合法 | 弹窗 form 内联红字 |
| 42022 定义不存在 | `ElMessage.error` + 关闭弹窗刷新列表 |
| 42023 已停用 | `ElMessage.error` toast |
| 42024 类型非法 | `ElMessage.error` toast（前端 select 已限制，兜底） |
| 42025 约束不自洽 | `ElMessage.error` toast |
| 42026 默认值违反约束 | `ElMessage.error` toast |
| 42027 删除须先停用 | `ElMessage.warning` toast |
| 42030 版本冲突 | `ElMessageBox.alert` 提示刷新 |
