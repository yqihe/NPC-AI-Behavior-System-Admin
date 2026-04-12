# 模板管理 — 前端设计

> **实现状态**：已全部落地，Vue 3.5 + TypeScript strict + Element Plus + Vite。
> 通用前端规范见 `docs/development/standards/red-lines/frontend.md` 和 `dev-rules/frontend.md`。

---

## 1. 目录结构

```
frontend/src/
├── api/
│   └── templates.ts                      # 类型定义 + TEMPLATE_ERR(41001-41012) + TEMPLATE_ERR_MSG + 8 个 API 函数
├── views/
│   ├── TemplateList.vue                  # 列表页：筛选 / 分页 / guard 拦截 / refs 详情 / toggle / 删除
│   └── TemplateForm.vue                  # 新建 + 编辑共用（mode prop 切换）
├── components/
│   ├── TemplateFieldPicker.vue           # 字段选择卡：按 category 分组 + 3 列网格 + reference popover 触发
│   ├── TemplateRefPopover.vue            # reference 子字段勾选弹层（el-dialog 形态）
│   ├── TemplateSelectedFields.vue        # 已选字段配置卡：必填 / 上下移动 / 停用字段标灰警告
│   ├── TemplateReferencesDialog.vue      # 被引用 NPC 详情弹窗（NPC 未上线显示 el-empty 占位）
│   └── EnabledGuardDialog.vue            # 启用守卫（与字段管理共用，entityType 泛型）
└── router/index.ts                       # /templates, /templates/create, /templates/:id/edit
```

共享依赖：`api/fields.ts`（FieldListItem 类型 + fieldApi.list 拉字段池）、`api/request.ts`、`components/AppLayout.vue`。

---

## 2. 页面路由

| 路径 | 组件 | route name | route meta / props |
|---|---|---|---|
| `/templates` | `TemplateList.vue` | `template-list` | `{ title: '模板管理' }` |
| `/templates/create` | `TemplateForm.vue` | `template-create` | `props: { mode: 'create' }` |
| `/templates/:id/edit` | `TemplateForm.vue` | `template-edit` | `props: (route) => ({ mode: 'edit', id: Number(route.params.id) })` |

`TemplateForm.vue` 通过 `defineProps<{ mode: 'create' | 'edit'; id?: number }>()` 接收路由注入的 props。

---

## 3. 组件树

```
TemplateList.vue
  ├─ EnabledGuardDialog.vue              (启用守卫，与字段管理泛型复用)
  └─ TemplateReferencesDialog.vue        (引用详情，NPC 未上线时 el-empty 占位)

TemplateForm.vue (mode: 'create' | 'edit', id?: number)
  ├─ TemplateFieldPicker.vue
  │   └─ TemplateRefPopover.vue          (内嵌，picker 持有 ref 并 open)
  └─ TemplateSelectedFields.vue
```

依赖方向单向向下：`views -> components -> api -> request`。

---

## 4. 类型契约

```ts
// --- api/templates.ts ---

interface TemplateFieldEntry {
  field_id: number
  required: boolean
}

interface TemplateListItem {
  id: number
  name: string
  label: string
  ref_count: number
  enabled: boolean
  created_at: string
  // 注意：列表接口不返回 version/description/fields
}

interface TemplateFieldItem {
  field_id: number
  name: string
  label: string
  type: string
  category: string
  category_label: string   // 后端 DictCache 翻译后返回
  enabled: boolean         // 字段停用时 UI 整行灰 + 警告图标
  required: boolean        // 模板里的必填配置
}

interface TemplateDetail {
  id: number
  name: string
  label: string
  description: string
  enabled: boolean
  version: number
  ref_count: number
  created_at: string
  updated_at: string
  fields: TemplateFieldItem[]   // 顺序即 templates.fields JSON 数组顺序
}

interface TemplateListQuery {
  label?: string
  enabled?: boolean | null
  page: number
  page_size: number
}

interface CreateTemplateRequest {
  name: string
  label: string
  description: string
  fields: TemplateFieldEntry[]
}

interface UpdateTemplateRequest {
  id: number
  label: string
  description: string
  fields: TemplateFieldEntry[]
  version: number
}

interface TemplateReferenceItem {
  npc_id: number
  npc_name: string
}

interface TemplateReferenceDetail {
  template_id: number
  template_label: string
  npcs: TemplateReferenceItem[]
}

const TEMPLATE_ERR = {
  NAME_EXISTS: 41001, NAME_INVALID: 41002, NOT_FOUND: 41003,
  NO_FIELDS: 41004, FIELD_DISABLED: 41005, FIELD_NOT_FOUND: 41006,
  REF_DELETE: 41007, REF_EDIT_FIELDS: 41008, DELETE_NOT_DISABLED: 41009,
  EDIT_NOT_DISABLED: 41010, VERSION_CONFLICT: 41011, FIELD_IS_REFERENCE: 41012,
} as const

const TEMPLATE_ERR_MSG: Record<number, string> = {
  41001: '模板标识已存在',
  41002: '模板标识格式不合法，需小写字母开头，仅允许 a-z、0-9、下划线',
  41003: '模板不存在',
  41004: '请至少勾选一个字段',
  41005: '勾选的字段已停用，请先在字段管理中启用',
  41006: '勾选的字段不存在',
  41007: '该模板正被 NPC 引用，无法删除',
  41008: '该模板已被 NPC 引用，字段勾选与必填配置不可修改',
  41009: '请先停用该模板再删除',
  41010: '请先停用该模板再编辑',
  41011: '该模板已被其他人修改，请刷新后重试',
  41012: 'reference 字段必须先展开子字段再加入模板',
}
```

**关键设计**：`TemplateRefPopover.vue` 读字段 detail 的 `constraints.refs`（后端权威格式），不假设 `ref_fields` 存在。读到 `refs: number[]` 后并发 `fieldApi.detail(subId)` 拿子字段元数据。

---

## 5. API 调用映射

| UI 操作 | API 函数 | 后端端点 |
|---|---|---|
| 列表加载 / 筛选 / 翻页 | `templateApi.list(params)` | `POST /api/v1/templates/list` |
| 新建模板 | `templateApi.create(data)` | `POST /api/v1/templates/create` |
| 查看详情（编辑页加载 / toggle 取 version） | `templateApi.detail(id)` | `POST /api/v1/templates/detail` |
| 编辑模板 | `templateApi.update(data)` | `POST /api/v1/templates/update` |
| 删除模板 | `templateApi.delete(id)` | `POST /api/v1/templates/delete` |
| 标识符唯一性校验（blur） | `templateApi.checkName(name)` | `POST /api/v1/templates/check-name` |
| 引用详情弹窗 | `templateApi.references(id)` | `POST /api/v1/templates/references` |
| 切换启用/停用 | `templateApi.toggleEnabled(id, enabled, version)` | `POST /api/v1/templates/toggle-enabled` |
| 拉取启用字段池（表单页） | `fieldApi.list({ enabled: true, page: 1, page_size: 1000 })` | `POST /api/v1/fields/list` |

---

## 6. 错误码处理

| 错误码 | 常量名 | UI 反馈 |
|---|---|---|
| 41001 | `NAME_EXISTS` | form 内联红字：`nameStatus='taken'` + `nameMessage` |
| 41002 | `NAME_INVALID` | form 内联红字：同上 |
| 41003 | `NOT_FOUND` | `ElMessage.error` + `router.push('/templates')` |
| 41004 | `NO_FIELDS` | 提交前已前端拦截（`selectedIds.length === 0` 检查），兜底 |
| 41005 | `FIELD_DISABLED` | `ElMessage.error` + `reloadFieldPool()` 重拉字段池 |
| 41006 | `FIELD_NOT_FOUND` | `ElMessage.error` + `reloadFieldPool()` 重拉字段池 |
| 41007 | `REF_DELETE` | 列表删除时自动打开 `TemplateReferencesDialog`（兜底 race condition） |
| 41008 | `REF_EDIT_FIELDS` | UI 已通过 `isLocked` 禁用字段变更，理论不到此分支，走默认 toast |
| 41009 | `DELETE_NOT_DISABLED` | 列表 `EnabledGuardDialog` 前置拦截，理论不到此分支 |
| 41010 | `EDIT_NOT_DISABLED` | 列表 `EnabledGuardDialog` 前置拦截，理论不到此分支 |
| 41011 | `VERSION_CONFLICT` | `ElMessageBox.alert` 提示后 `router.push('/templates')` |
| 41012 | `FIELD_IS_REFERENCE` | `ElMessage.error('reference 字段必须先展开子字段再加入模板')` + `reloadFieldPool()` |
