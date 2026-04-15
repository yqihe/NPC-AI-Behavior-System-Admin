# 前端统一约定

本文档描述 ADMIN 项目前端（Vue 3 + TypeScript + Element Plus）的**项目级约定**：目录结构、布局规范、API 层模式、列表页/表单页代码结构。

通用语言陷阱与 Element Plus 注意事项见 `../development/standards/dev-rules/frontend.md`，禁止红线见 `../development/standards/red-lines/frontend.md`。

---

## 一、目录结构

```
frontend/src/
  api/              # 每个资源一个文件：类型 + 错误码 + api 对象
  components/       # 跨页面复用组件
  styles/
    list-layout.css # 列表页共用布局（全局引入）
    form-layout.css # 表单页共用布局（全局引入）
  utils/
    format.ts       # formatTime 等工具函数
  views/            # 页面组件，每个资源 XxxList.vue + XxxForm.vue
  main.ts           # 全局 CSS 统一在此 import
  router/
```

---

## 二、布局标准（CSS）

### 2.1 列表页结构（`list-layout.css`）

```
<div class="xxx-list">          ← display: flex; flex-direction: column; height: 100%
  <div class="page-header">     ← 标题（左） + 新建按钮（右）
  <div class="filter-bar">      ← 筛选输入框 + 搜索/重置按钮
  <div class="table-wrap">      ← el-table
    <div class="pagination-wrap"> ← "共 N 条" + el-pagination
```

筛选输入框用 `.filter-item`（等宽），文字搜索框用 `.filter-item-wide`（1.5x）。

### 2.2 表单页结构（`form-layout.css`）

```
<div class="xxx-form">          ← display: flex; flex-direction: column; height: 100%
  <div class="form-header">     ← 返回箭头 + 返回文字 + / + 页面标题
  <div class="form-scroll">     ← flex: 1; overflow-y: auto; 灰色背景
    <div class="form-body">     ← max-width: 800px; margin: 0 auto
      <div class="form-card">   ← 白色卡片，padding: 32px
        <div class="card-title"> ← 彩色竖条 + 标题文字
        <el-form>
  <div class="form-footer">     ← 取消 + 保存（v-if="!isView"，滚动区外）
```

- **宽布局**（TemplateForm 等需要 > 800px）：用 `.form-body-wide`（max-width: 1200px）
- **卡片竖条颜色**：`.title-bar-blue / -orange / -green / -red`
- **`.form-footer` 必须在 `.form-scroll` 外**，保证按钮始终可见不随内容滚动

### 2.3 页面级 scoped style 原则

只写当前组件私有样式。以下 class 已由全局 CSS 覆盖，**不得在 scoped 中重复定义**：

`.form-header` `.form-scroll` `.form-body` `.form-body-wide` `.form-card` `.card-title` `.title-bar*` `.title-text` `.form-footer` `.page-header` `.page-title` `.filter-bar` `.filter-item*` `.table-wrap` `.pagination-wrap` `.total-text`

---

## 三、API 层模式（`src/api/*.ts`）

每个资源模块的文件结构固定如下：

```typescript
// 1. 类型定义
export interface XxxListQuery { ... }
export interface XxxListItem  { ... }
export interface XxxDetail    { ... }
export interface XxxCreateReq { ... }
export interface XxxUpdateReq { ... }

// 2. 业务错误码（对应后端 errcode 包）
export const XXX_ERR = {
  NAME_EXISTS:      40001,
  VERSION_CONFLICT: 40009,
  IN_USE:           40010,
  // ...
} as const

// 3. API 对象
export const xxxApi = {
  list(params: XxxListQuery):       Promise<ApiResponse<ListData<XxxListItem>>>
  detail(id: number):               Promise<ApiResponse<XxxDetail>>
  create(req: XxxCreateReq):        Promise<ApiResponse<void>>
  update(req: XxxUpdateReq):        Promise<ApiResponse<void>>
  delete(id: number):               Promise<ApiResponse<void>>
  checkName(name: string):          Promise<ApiResponse<CheckNameResult>>
  toggleEnabled(id, val, version):  Promise<ApiResponse<void>>
}
```

**拦截器约定**：`request.ts` 拦截器在 code !== 0 时自动 `ElMessage.error()`，组件 catch 块不重复 toast，用注释 `// 拦截器已 toast` 标注。

---

## 四、列表页代码结构

### 4.1 状态

```typescript
const loading        = ref(false)
const tableData      = ref<XxxListItem[]>([])
const total          = ref(0)
const guardRef       = ref<InstanceType<typeof EnabledGuardDialog> | null>(null)
const categoryOptions = ref<DictionaryItem[]>([])  // 下拉字典选项

const query = reactive<XxxListQuery>({
  // 文字筛选字段默认 ''
  // 枚举/ID 筛选默认 ''
  // enabled 三态：true / false / null
  enabled: null,
  page: 1,
  page_size: 20,
})
```

### 4.2 数据加载

```typescript
async function fetchList() {
  loading.value = true
  try {
    const params: XxxListQuery = { page: query.page, page_size: query.page_size }
    if (query.xxx) params.xxx = query.xxx          // 只传非空字段
    if (query.enabled !== null) params.enabled = query.enabled
    const res = await xxxApi.list(params)
    tableData.value = res.data?.items || []
    total.value     = res.data?.total || 0
  } catch {
    // 拦截器已 toast
  } finally {
    loading.value = false
  }
}

async function loadCategoryOptions() {
  try {
    const res = await dictApi.list('xxx_category')
    categoryOptions.value = res.data?.items ?? []
  } catch { /* 非关键，静默失败 */ }
}

onMounted(() => { fetchList(); loadCategoryOptions() })
```

### 4.3 筛选

```typescript
function handleSearch() { query.page = 1; fetchList() }
function handleReset()  { query.xxx = ''; query.enabled = null; query.page = 1; fetchList() }
```

### 4.4 启用/禁用 Toggle

列表接口**不返回 version**，切换前必须先 `detail()` 拿版本号：

```typescript
async function handleToggle(row: XxxListItem, val: boolean) {
  const action = val ? '启用' : '禁用'
  try {
    await ElMessageBox.confirm(`确认${action}「${row.display_name}」？`, `${action}确认`, {
      confirmButtonText: `确认${action}`,
      cancelButtonText: '取消',
      type: val ? 'success' : 'warning',
    })
    const detail = await xxxApi.detail(row.id)
    await xxxApi.toggleEnabled(row.id, val, detail.data.version)
    ElMessage.success(`已${action}`)
    fetchList()
  } catch (err) {
    if (err === 'cancel') return
    if ((err as BizError).code === XXX_ERR.VERSION_CONFLICT) {
      ElMessageBox.alert('数据已被其他用户修改，请刷新页面后重试。', '版本冲突', { type: 'warning' })
      fetchList()
      return
    }
    // 其他错误拦截器已 toast
  }
}
```

### 4.5 编辑 / 删除

```typescript
function handleEdit(row: XxxListItem) {
  if (row.enabled) {
    guardRef.value?.open({ action: 'edit', entityType: 'xxx', entity: { id: row.id, name: row.name, label: row.display_name } })
    return
  }
  router.push(`/xxx/${row.id}/edit`)
}

async function handleDelete(row: XxxListItem) {
  if (row.enabled) {
    guardRef.value?.open({ action: 'delete', entityType: 'xxx', entity: { id: row.id, name: row.name, label: row.display_name } })
    return
  }
  try {
    await ElMessageBox.confirm(`确认删除「${row.display_name}」？删除后无法恢复。`, '删除确认', {
      confirmButtonText: '确认删除', cancelButtonText: '取消', type: 'warning',
    })
    await xxxApi.delete(row.id)
    ElMessage.success('删除成功')
    fetchList()
  } catch (err: unknown) {
    if (err === 'cancel') return
    const bizErr = err as BizError
    if (bizErr.code === XXX_ERR.IN_USE) {
      // 展示被引用详情弹窗
      return
    }
    // 其他错误拦截器已 toast
  }
}
```

### 4.6 辅助

```typescript
function rowClassName({ row }: { row: XxxListItem }) {
  return row.enabled ? '' : 'row-disabled'
}
```

模板中 `:row-class-name="rowClassName"`，配合全局 CSS：

```css
:deep(.row-disabled td:not(:nth-last-child(-n+3))) {
  opacity: 0.5;   /* 操作列、开关列不变暗 */
}
```

分类/状态等枚举列用 `<el-tag>` 展示，不直接输出原始值。

---

## 五、表单页代码结构

### 5.1 模式判断

单个 Form 组件承接新建/编辑/查看三种模式，通过 `route.meta` 区分：

```typescript
// router 配置
{ path: '/xxx/create',    component: XxxForm, meta: { isCreate: true,  isView: false } }
{ path: '/xxx/:id/view',  component: XxxForm, meta: { isCreate: false, isView: true  } }
{ path: '/xxx/:id/edit',  component: XxxForm, meta: { isCreate: false, isView: false } }

// 组件内
const isCreate = route.meta.isCreate as boolean
const isView   = route.meta.isView   as boolean
const isEdit   = !isCreate
```

### 5.2 核心状态

```typescript
const formRef    = ref<FormInstance | null>(null)
const loading    = ref(false)
const submitting = ref(false)
const version    = ref(0)            // 乐观锁版本

const formState = reactive<XxxFormState>({
  name: '',   // 标识字段，create 时可编辑，edit/view 锁定
  display_name: '',
  // ...
})

type NameStatus = 'idle' | 'checking' | 'available' | 'taken'
const nameStatus  = ref<NameStatus>('idle')
const nameMessage = ref('')
const namePattern = /^[a-z][a-z0-9_]*$/
```

### 5.3 标识字段校验（新建模式）

```typescript
async function onNameBlur() {
  if (isEdit) return
  const name = formState.name.trim()
  if (!name) { nameStatus.value = 'idle'; return }
  if (!namePattern.test(name)) {
    nameStatus.value = 'taken'
    nameMessage.value = '格式不合法（小写字母开头，a-z / 0-9 / 下划线）'
    return
  }
  nameStatus.value = 'checking'
  try {
    const res = await xxxApi.checkName(name)
    nameStatus.value  = res.data.available ? 'available' : 'taken'
    nameMessage.value = res.data.message
  } catch {
    nameStatus.value = 'idle'
  }
}
```

模板中标识字段两种态：
- **edit/view**：`<el-input disabled>` + Lock 图标 + 警告提示"创建后不可修改"
- **create**：`<el-input @blur="onNameBlur">` + 状态提示（checking / available / taken）

### 5.4 详情加载

```typescript
onMounted(async () => {
  if (!isEdit) return
  loading.value = true
  try {
    const res = await xxxApi.detail(props.id!)
    const d = res.data
    version.value        = d.version
    formState.name       = d.name
    formState.display_name = d.display_name
    // ...
  } catch (err) {
    if ((err as BizError).code === XXX_ERR.NOT_FOUND) {
      ElMessage.error('记录不存在')
      router.push('/xxx')
    }
  } finally {
    loading.value = false
  }
})
```

### 5.5 提交

```typescript
async function onSubmit() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  if (isCreate && nameStatus.value === 'taken') {
    ElMessage.warning('标识不可用，请更换')
    return
  }

  submitting.value = true
  try {
    if (isCreate) {
      await xxxApi.create({ name: formState.name, ... })
      ElMessage.success('创建成功')
    } else {
      await xxxApi.update({ id: props.id!, version: version.value, ... })
      ElMessage.success('保存成功')
    }
    router.push('/xxx')
  } catch (err) {
    const bizErr = err as BizError
    switch (bizErr.code) {
      case XXX_ERR.NAME_EXISTS:
      case XXX_ERR.NAME_INVALID:
        nameStatus.value  = 'taken'
        nameMessage.value = bizErr.message
        return
      case XXX_ERR.VERSION_CONFLICT:
        ElMessageBox.alert('数据已被其他人修改，请刷新后重试。', '版本冲突', { type: 'warning' })
        return
      case XXX_ERR.NOT_FOUND:
        router.push('/xxx')
        return
      default:
        // 其他错误拦截器已 toast
    }
  } finally {
    submitting.value = false
  }
}
```

### 5.6 查看模式

- `<el-form :disabled="isView">` 整体禁用
- `<div v-if="!isView" class="form-footer">` 隐藏按钮栏
- 不展示创建时间 / 更新时间（时间戳仅在列表页 `created_at` 列可见，详情页不展示）

---

## 六、通用组件

### EnabledGuardDialog

所有列表页必须挂载此组件，处理"已启用实体不可直接编辑/删除"的拦截逻辑：

```html
<!-- 列表模板末尾 -->
<EnabledGuardDialog ref="guardRef" @refresh="fetchList" />
```

```typescript
guardRef.value?.open({
  action:     'edit' | 'delete',
  entityType: 'field' | 'template' | 'event-type' | 'event-type-schema' | 'fsm-state-dict',
  entity:     { id: row.id, name: row.name, label: row.display_name },
})
```

组件内部会先禁用实体（调 toggleEnabled），edit action 自动跳转编辑页，delete action 完成后 emit `@refresh`。

### formatTime

```typescript
import { formatTime } from '@/utils/format'
// 所有时间字段统一用此函数格式化，不直接输出 ISO 字符串
```

---

## 七、路由约定

| 路径 | meta | 说明 |
|------|------|------|
| `/resource` | — | 列表页 |
| `/resource/create` | `{ isCreate: true, isView: false }` | 新建 |
| `/resource/:id/view` | `{ isCreate: false, isView: true }` | 查看 |
| `/resource/:id/edit` | `{ isCreate: false, isView: false }` | 编辑 |

路由 name 用 kebab-case：`field-list`、`field-create`、`field-view`、`field-edit`。

---

## 八、数值输入规范

- `el-input-number` 统一 `style="width: 200px"`，不随父容器拉伸
- 数字输入带边界（`:min` / `:max`）时配合后端 constraint 定义

---

## 九、当前已实现模块

以下模块完整实现了上述约定，可作为参考：

| 模块 | List | Form |
|------|------|------|
| 字段管理 | `FieldList.vue` | `FieldForm.vue` |
| 模板管理 | `TemplateList.vue` | `TemplateForm.vue` |
| 事件类型 | `EventTypeList.vue` | `EventTypeForm.vue` |
| EventTypeSchema | `EventTypeSchemaList.vue` | `EventTypeSchemaForm.vue` |
| FSM 状态字典 | `FsmStateDictList.vue` | `FsmStateDictForm.vue` |
