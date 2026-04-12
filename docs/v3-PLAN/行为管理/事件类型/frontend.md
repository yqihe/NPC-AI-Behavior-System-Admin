# 事件类型管理 — 前端架构

> 补充 [features.md](features.md) 未覆盖的前端层信息：页面路由、组件树、状态流、Schema 驱动表单渲染。
> **实现状态**：规划中。

---

## 页面路由

```
/event-types                       # 列表页
/event-types/new                   # 新建页
/event-types/:id/edit              # 编辑/详情页（一个页面承担两种角色，和模板管理同构）
/schema-management                 # Schema 管理页（含多个 tab，事件类型扩展字段是其中一个）
```

---

## 组件树

```
EventTypeListView.vue              # 列表 + 筛选 + 分页
  ├─ EventTypeFilterBar.vue        # 搜索框(display_name) + perception_mode facet + enabled 筛选
  ├─ EventTypeTable.vue            # el-table 渲染
  └─ EnabledGuardDialog.vue        # 复用字段管理的"必须先停用"引导弹窗

EventTypeFormView.vue              # 新建/编辑
  ├─ EventTypeSystemFields.vue     # 系统字段硬编码 (name, display_name, perception_mode, range, severity, ttl)
  ├─ EventTypeExtensionFields.vue  # 扩展字段 SchemaForm 包装
  │   └─ SchemaForm.vue            # 通用组件，接受 schema 数组 + 值对象 + dirty 追踪
  │       ├─ FormFieldInt.vue
  │       ├─ FormFieldFloat.vue
  │       ├─ FormFieldString.vue
  │       ├─ FormFieldBool.vue
  │       └─ FormFieldSelect.vue
  └─ (ReferenceWarning.vue 占位)   # 被引用时的警告 banner，本期不渲染

SchemaManagementView.vue           # Schema 管理页主容器
  └─ EventTypeSchemaTab.vue        # 事件类型扩展字段 tab
      ├─ EventTypeSchemaList.vue   # 列表
      ├─ EventTypeSchemaForm.vue   # 新建/编辑弹窗或子页
      └─ ConstraintPanel.vue       # 按 field_type 动态渲染约束编辑器
          └─ 复用 FieldConstraintInteger / Float / String / Boolean / Select.vue（字段管理已有）
```

---

## 扩展字段的 SchemaForm 渲染（核心交互）

这是本模块在前端最复杂的部分。

### 加载阶段

```typescript
// 进入 /event-types/:id/edit
const { event_type, extension_schema } = await getEventTypeDetail(id)

// 系统字段：从 config 里直接抽出已知 key
const systemValues = {
  display_name: event_type.config.display_name,
  perception_mode: event_type.config.perception_mode,
  range: event_type.config.range,
  default_severity: event_type.config.default_severity,
  default_ttl: event_type.config.default_ttl,
}

// 扩展字段：按 schema 顺序初始化，同时记录 dirty 状态
const extensionFields = extension_schema
  .filter(s => s.enabled)
  .sort((a, b) => a.sort_order - b.sort_order)
  .map(s => {
    const hasValue = s.field_name in event_type.config
    return {
      schema: s,
      value: hasValue ? event_type.config[s.field_name] : s.default_value,
      dirty: hasValue,  // config 里有 → 运营曾经填过 → dirty
    }
  })
```

### 交互阶段

用户在扩展字段输入框里改值 → 该字段的 `dirty` 变成 `true`。

**重要**：展示 `default_value` 作为"暗示值"，但在用户没真的改之前不算"填过"。UI 上区分两种状态：

- `dirty=false`：输入框有占位文字"默认: 50"（浅灰），值显示 `default_value`，但视觉上能看出这是默认
- `dirty=true`：值显示运营填的值（黑色），有一个"重置为默认"小按钮

这样运营一眼能看出"哪些字段是我主动配置的，哪些是系统默认的"。

### 提交阶段

```typescript
const config = {
  ...systemValues,
  // 只把 dirty=true 的扩展字段写进 payload
  ...Object.fromEntries(
    extensionFields
      .filter(ef => ef.dirty)
      .map(ef => [ef.schema.field_name, ef.value])
  )
}

await updateEventType({ id, version, ...basicFields, extensions: pickExtensions(config) })
```

### 为什么这么设计

- **未 dirty 的扩展字段不进 payload** → 不进 `config_json` → 导出给游戏服务端时没这个 key → 服务端按自己的默认值处理
- 如果前端无脑把 default_value 也提交上去，`config_json` 会存一堆"默认值重复"，浪费存储，还让"运营明确填过 vs 运营从未管过"失去语义区分
- 未来如果游戏服务端修改了某个默认值（把 priority 默认从 50 改成 80），所有"从未被运营填过"的事件类型立刻跟着变，不需要 ADMIN 做任何迁移

---

## 状态流（Pinia store）

```
stores/eventType.ts          # 列表查询 / 详情 / 当前编辑对象 / 提交态
stores/eventTypeSchema.ts    # 扩展字段 schema 列表 + 按 enabled 过滤 + reload 动作
```

**`eventTypeSchema` 初始化**：
- App 启动后（在 `main.ts` 或路由进入事件类型页时）主动 fetch 一次
- 事件类型表单直接从 store 读，不重复请求
- Schema 管理页有写操作后调 `eventTypeSchema.reload()`

**`eventType` store 的 detail 字段**：
- 直接存后端返回的完整 detail 对象（含 `extension_schema`）
- 路由离开时清空，避免返回列表后再进入时看到旧数据

---

## UI 关键交互细节

| 交互 | 实现 |
|---|---|
| `perception_mode == global` 时 `range` 禁用并置 0 | `watch(perception_mode, ...)` |
| 启用中的事件类型进入编辑页，全部字段只读 + 顶部 banner "请先停用" | 路由 meta 检查 + 表单组件 `readonly` prop |
| 停用行整行 opacity 0.5，但**操作列保持高亮**（与模板页一致） | el-table `row-class-name` |
| `default_severity` 0-100 slider 配色带：0-30 绿 / 30-70 黄 / 70-100 红 | 自定义 SeverityBar.vue |
| 扩展字段数值收紧时弹二次确认 | `ElMessageBox.confirm` + 文案 "此操作可能影响已部署的 FSM/BT 条件" |
| 扩展字段删除前展示影响估算（本期不做） | 未来加 `POST /api/v1/event-type-schema/impact-estimate` |
| 表单有未保存变更时离开路由弹确认 | `onBeforeRouteLeave` |
| 编辑页顶部常驻提示：修改 `perception_mode` / `range` / `default_severity` / `default_ttl` 后需通知运维重启游戏服务端才能生效 | 静态 `el-alert type="warning"` 横条，文案固定 |

---

## 与字段管理 / 模板管理的复用

| 组件 | 复用方式 |
|---|---|
| `EnabledGuardDialog.vue` | 直接复用，泛型化（目前已经是泛型） |
| `FieldConstraint{Integer,Float,String,Boolean,Select}.vue` | 直接复用。Schema 管理页的 `ConstraintPanel` 按 `field_type` 动态 `import()` |
| 字段标识正则校验工具 | 复用 `utils/identifier.ts` |
| 错误码 → 中文映射 | 照 `api/fields.ts` 的 `FIELD_ERR` 模式，新建 `api/event-types.ts::EVENT_TYPE_ERR` |
| 分页组件 / 列表工具 | 复用 |
| 乐观锁冲突提示 | 复用 |

**不复用**：
- `FieldConstraintReference.vue`：扩展字段不支持 reference 类型
- 字段管理的 "停用字段下拉自动隐藏" 逻辑：事件类型暂时没有"被引用"语义

---

## 错误码本地化

在 `api/event-types.ts` 定义：

```typescript
export const EVENT_TYPE_ERR: Record<number, string> = {
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

export const EVENT_TYPE_SCHEMA_ERR: Record<number, string> = {
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
```

---

## 前端开发顺序建议

1. 先做 features 1-7（事件类型系统字段 CRUD），不涉及 SchemaForm，独立可测
2. 抽出 `SchemaForm.vue` 通用组件（空壳子即可，先不接 schema 源）
3. 做 features 8-11（Schema 管理页），涉及 ConstraintPanel 复用
4. 把 SchemaForm 接入事件类型编辑页的扩展字段区（features 2/3/4 的扩展字段部分）
5. 最后做 features 12 的联调（服务端改造完成后才能真正跑通）
