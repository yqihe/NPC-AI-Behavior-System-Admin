# 字段管理前端 — 设计方案

## 方案描述

### 整体架构

基于 V2 项目骨架模式，从零搭建 V3 前端。V3 不再使用 V2 的 GenericList/GenericForm 通用组件（那是为 MongoDB `{name, config}` 动态 schema 设计的），改为每个配置模块写专用页面，原因：

- 字段管理的三态生命周期（启用/禁用/删除）、引用计数、乐观锁等逻辑是字段特有的
- 约束面板按类型动态切换，通用组件反而增加复杂度
- 专用页面更容易满足 UI 红线（非技术用户友好的表单、灰色提示文字等）

### 目录结构

```
frontend/
├── index.html
├── package.json
├── vite.config.js
├── nginx.conf
├── src/
│   ├── main.js                          # 入口
│   ├── App.vue                          # 根组件
│   ├── router/
│   │   └── index.js                     # 路由定义
│   ├── api/
│   │   ├── request.js                   # Axios 实例 + 拦截器
│   │   ├── fields.js                    # 字段 API
│   │   └── dictionaries.js             # 字典 API
│   ├── components/
│   │   ├── AppLayout.vue                # 侧边栏 + 主内容区布局
│   │   ├── FieldConstraintInteger.vue   # integer/float 约束面板
│   │   ├── FieldConstraintString.vue    # string 约束面板
│   │   ├── FieldConstraintSelect.vue    # select 约束面板
│   │   └── FieldConstraintReference.vue # reference 约束面板
│   └── views/
│       ├── FieldList.vue                # 字段列表页
│       └── FieldForm.vue                # 新建/编辑字段页（共用）
```

### API 层设计

**Axios 实例（`request.js`）**

```javascript
const request = axios.create({
  baseURL: import.meta.env.VITE_API_BASE || '/api/v1',
  timeout: 10000,
})

// 响应拦截器：统一业务错误处理
request.interceptors.response.use(
  (response) => {
    const { code, message } = response.data
    if (code !== 0) {
      // 业务错误：弹 toast，reject 让调用方感知
      ElMessage.error(message || '操作失败')
      return Promise.reject(new Error(message))
    }
    return response.data // 直接返回 { code, data, message }
  },
  (error) => {
    const msg = error.response?.data?.message || '网络错误，请检查后端服务'
    ElMessage.error(msg)
    return Promise.reject(error)
  }
)
```

关键改进（对比 V2）：
- V2 拦截器只处理 HTTP 错误，不处理 `code !== 0` 业务错误 → V3 在 success 分支统一检查 code
- V2 拦截器 `return response` 但不 reject → 调用方 `.then()` 收到 undefined。V3 明确 reject
- V3 直接 `return response.data`，调用方拿到的是 `{ code, data, message }` 不需要 `.data.data`

**字段 API（`fields.js`）**

所有接口均为 POST，请求体 JSON：

```javascript
export const fieldApi = {
  list: (params) => request.post('/fields/list', params),
  create: (data) => request.post('/fields/create', data),
  detail: (id) => request.post('/fields/detail', { id }),
  update: (data) => request.post('/fields/update', data),
  delete: (id) => request.post('/fields/delete', { id }),
  checkName: (name) => request.post('/fields/check-name', { name }),
  toggleEnabled: (id, enabled, version) =>
    request.post('/fields/toggle-enabled', { id, enabled, version }),
  references: (id) => request.post('/fields/references', { id }),
}
```

**字典 API（`dictionaries.js`）**

```javascript
export const dictApi = {
  list: (group) => request.post('/dictionaries', { group }),
}
```

### 路由设计

```javascript
routes: [
  {
    path: '/',
    redirect: '/fields',
  },
  {
    path: '/fields',
    name: 'field-list',
    component: () => import('@/views/FieldList.vue'),
    meta: { title: '字段管理' },
  },
  {
    path: '/fields/create',
    name: 'field-create',
    component: () => import('@/views/FieldForm.vue'),
    meta: { title: '新建字段', isCreate: true },
  },
  {
    path: '/fields/:id/edit',
    name: 'field-edit',
    component: () => import('@/views/FieldForm.vue'),
    meta: { title: '编辑字段', isCreate: false },
  },
]
```

`:id` 使用数字 ID（V3 已全面改用 ID 标识），不需要 V2 的 `:name(.*)` catch-all。

### 页面组件设计

#### FieldList.vue

**数据状态**：
```javascript
const loading = ref(false)
const tableData = ref([])
const total = ref(0)
const query = reactive({
  label: '',
  type: '',
  category: '',
  enabled: null,     // null=全部, true=启用, false=禁用
  page: 1,
  page_size: 20,
})
const typeOptions = ref([])      // 从字典 API 获取
const categoryOptions = ref([])  // 从字典 API 获取
```

**生命周期**：
- `onMounted`：并行调用 `fieldApi.list` + `dictApi.list('field_type')` + `dictApi.list('field_category')`
- 筛选/翻页：重新调用 `fieldApi.list`，query 直接传给后端

**表格列**：ID | 标识符 | 中文标签 | 类型（badge） | 分类（badge） | 引用计数（蓝色链接） | 启用（switch） | 创建时间 | 操作（编辑/删除）

**禁用行样式**：`el-table` `:row-class-name` 返回 `'row-disabled'`，CSS 设 `opacity: 0.5`

**引用详情弹窗**：内嵌 `el-dialog`，点击引用计数时调用 `fieldApi.references(id)`，展示模板引用表和字段引用表

**删除逻辑**：
```
点击删除 →
  if (row.enabled) → ElMessageBox「请先禁用该字段」
  else if (row.ref_count > 0) → 弹窗「该字段被引用中」+ 显示引用列表 + 红色警告
  else → ElMessageBox.confirm 确认删除（显示字段名和标签）→ fieldApi.delete
```

**Toggle 逻辑**：
```
switch 切换 → fieldApi.toggleEnabled(id, newValue, version)
  成功 → 刷新列表
  40010 → ElMessageBox「数据已被其他用户修改，请刷新页面后重试」
```

#### FieldForm.vue

**区分新建/编辑**：通过 `route.meta.isCreate` 判断

**数据状态**：
```javascript
const form = reactive({
  name: '',
  label: '',
  type: '',
  category: '',
  properties: {
    description: '',
    expose_bb: false,
    default_value: null,
    constraints: {},
  },
})
const version = ref(0)           // 编辑时的乐观锁版本
const refCount = ref(0)          // 编辑时的引用计数
const nameStatus = ref('')       // '' | 'checking' | 'available' | 'taken'
const submitting = ref(false)
```

**编辑模式加载**：`onMounted` 调用 `fieldApi.detail(route.params.id)`，`Object.assign(form, ...)` 填充

**标识符校验**：
- `@blur` 触发：先本地 `/^[a-z][a-z0-9_]*$/` 校验，通过后调用 `fieldApi.checkName(name)`
- 三态反馈：checking（灰色旋转图标）、available（绿色勾 + "可用"）、taken（红色叉 + "已被使用"）
- 编辑模式标识符只读，跳过校验

**约束面板动态切换**：
```vue
<FieldConstraintInteger v-if="form.type === 'integer' || form.type === 'float'"
  v-model="form.properties.constraints" :restricted="refCount > 0" />
<FieldConstraintString v-else-if="form.type === 'string'"
  v-model="form.properties.constraints" :restricted="refCount > 0" />
<!-- boolean 无约束，显示灰色提示文字 -->
<div v-else-if="form.type === 'boolean'" class="constraint-empty">
  布尔类型无需约束配置
</div>
<FieldConstraintSelect v-else-if="form.type === 'select'"
  v-model="form.properties.constraints" :restricted="refCount > 0" />
<FieldConstraintReference v-else-if="form.type === 'reference'"
  v-model="form.properties.constraints" :restricted="refCount > 0" />
```

**编辑受限模式**（`refCount > 0`）：
- 类型字段：`el-select` disabled + 锁图标 + 黄色警告「已被引用，无法更改类型」
- 约束面板：通过 `restricted` prop 传入，面板内部显示黄色警告「已被引用，约束只能放宽不能收紧」
- BB Key toggle：若后端返回 40008，显示黄色警告

**提交逻辑**：
```
新建：fieldApi.create({ name, label, type, category, properties })
  成功 → ElMessage.success + 跳转列表
编辑：fieldApi.update({ id, label, type, category, properties, version })
  成功 → ElMessage.success + 跳转列表
  40010 → ElMessageBox「数据已被修改，请刷新后重试」
```

#### 约束面板组件

**FieldConstraintInteger.vue**（integer/float 共用）：
- Props：`modelValue`（constraints 对象）、`restricted`（是否受限）
- 三个 `el-input-number`：最小值、最大值、步长
- 水平排列（el-row + el-col）

**FieldConstraintString.vue**：
- 最小长度、最大长度（水平排列）
- 正则校验模式（单独一行，选填，placeholder 示例）

**FieldConstraintSelect.vue**：
- 选项列表：动态表格，每行 value + label + 删除按钮
- 「添加选项」按钮
- 最少/最多选择数（水平排列）
- 绿色提示「min=1, max=1 为单选；max>1 为多选」

**FieldConstraintReference.vue**：
- 引用字段列表：每行拖拽手柄 + 类型 badge + name + label + 删除
- 「添加引用」按钮 → 下拉选择（仅 enabled 字段，排除自身和已选）
- 展开预览：显示扁平化字段列表
- 黄色警告：引用 reference 字段时系统自动检测循环引用

### 错误码 → UI 映射

| 错误码 | 场景 | UI 处理 |
|--------|------|---------|
| 40001 | 标识符已存在 | 标识符输入框下方红字 |
| 40002 | 标识符格式错误 | 标识符输入框下方红字 |
| 40003 | 类型不存在 | 全局 toast（拦截器自动处理） |
| 40004 | 分类不存在 | 全局 toast |
| 40005 | 有引用无法删除 | 弹窗 + 自动打开引用详情 |
| 40006 | 已引用无法改类型 | 编辑页类型字段黄色警告（预防性，不应触发） |
| 40007 | 约束只能放宽 | 全局 toast |
| 40008 | BB Key 被行为树使用 | 编辑页 BB Key 黄色警告 |
| 40009 | 循环引用 | 全局 toast |
| 40010 | 版本冲突 | ElMessageBox「数据已变更，请刷新重试」 |
| 40011 | 字段不存在 | 跳转列表 + toast |
| 40012 | 未禁用不能删除 | ElMessageBox「请先禁用」 |
| 40013 | 不能引用禁用字段 | 全局 toast |
| 40014 | 引用字段不存在 | 全局 toast |
| 40015 | 未禁用不能编辑 | ElMessageBox「请先禁用」 |

**特殊处理**：40001、40002、40005、40010、40011、40012、40015 需要在调用方 `.catch()` 中判断 code 做特殊 UI，其余由拦截器 toast 兜底即可。

实现方式：拦截器统一 toast 后 reject，错误对象携带 code。调用方在 `.catch(err)` 中检查 `err.code`，匹配特殊码则做额外 UI 处理（弹窗/红字），不匹配则已有 toast 足够。

```javascript
// 拦截器中 reject 时携带 code
if (code !== 0) {
  ElMessage.error(message || '操作失败')
  const err = new Error(message)
  err.code = code
  return Promise.reject(err)
}
```

### Docker 部署

**Dockerfile.frontend**：复用 V2 的多阶段构建模式（Node 20 Alpine 构建 + Nginx Alpine 部署）

**nginx.conf**：
- `/api/` → `proxy_pass http://admin-backend:9821`
- `/` → `try_files $uri $uri/ /index.html`（SPA 路由）

**docker-compose.yml**：新增 `admin-frontend` 服务，端口 `3000:80`

## 方案对比

### 备选方案 A：复用 V2 的 GenericList + GenericForm + SchemaForm

V2 使用 GenericList/GenericForm/SchemaForm 三个通用组件处理所有配置类型，通过 `route.meta` 传入 API 和 schema 驱动渲染。

**不选原因**：
1. V3 字段管理有三态生命周期（启用/禁用/删除），GenericList 不支持 toggle/引用计数/条件删除
2. V3 表单有 5 种约束面板动态切换、标识符实时校验、受限编辑模式，SchemaForm 的 JSON Schema 驱动无法表达这些交互逻辑
3. V2 的 SchemaForm 依赖 `@lljj/vue3-form-element`，V3 明确不使用第三方 JSON Schema 表单库
4. 通用组件为了适配字段管理需要大量 `if/else` 特判，比专用组件更难维护

### 备选方案 B：使用 Pinia 全局状态管理

将字段列表、字典选项、当前编辑字段等放入 Pinia store。

**不选原因**：
1. 字段管理是典型 CRUD 页面，数据流为「页面加载 → API 请求 → 局部状态 → 渲染」，无跨页面共享状态需求
2. 字典选项在列表页和表单页各自加载（页面级 `onMounted`），无需全局缓存
3. 引入 Pinia 增加依赖但不减少复杂度，违反「不引入无用例的依赖」红线
4. 后续模板管理若需要字段选择器，通过 API 调用获取即可，不需要 store 中转

## 红线检查

### 前端红线（`standards/frontend-red-lines.md`）

| 红线 | 合规 | 说明 |
|------|------|------|
| 禁止直接 filter/splice 原始数据 | ✅ | 所有筛选走后端分页，前端不做本地过滤 |
| 枚举值必须 el-select | ✅ | 类型/分类/状态均用 el-select，标识符用 el-input 但有 blur 校验 |
| 同一数据源下拉一致 | ✅ | 字典选项从 API 获取存入 ref，列表页和表单页各自独立加载 |
| name 字段必须格式校验 | ✅ | blur 时本地 `/^[a-z][a-z0-9_]*$/` + 后端 check-name |
| URL 编码 | ✅ | V3 改用数字 ID 路由 `/fields/:id/edit`，无特殊字符问题 |

### UI/UX 红线（`architecture/ui-red-lines.md`）

| 红线 | 合规 | 说明 |
|------|------|------|
| 禁止暴露 BB Key 原始名 | ✅ | 表格展示中文标签（label），标识符仅在详情/编辑页展示 |
| 禁止让策划写 JSON | ✅ | 约束配置全部用表单组件，properties 在提交时组装为 JSON |
| 禁止展示技术错误信息 | ✅ | 后端错误 message 已为中文，拦截器直接 toast |
| 表单必须有灰色提示文字 | ✅ | 所有表单项用 el-form-item + 灰色 placeholder/描述 |
| 空列表用 el-empty + 按钮 | ✅ | 表格空状态显示 el-empty + 「新建字段」按钮 |
| 删除确认必须说明影响 | ✅ | 三态删除，有引用时显示具体引用列表 |
| 新建必须 blur 查重 | ✅ | 标识符 blur 调用 check-name API |

### 通用红线（`standards/red-lines.md`）

| 红线 | 合规 | 说明 |
|------|------|------|
| 禁止信任前端校验 | ✅ | 前端校验仅为 UX，后端完整校验 |
| 禁止引入无用例依赖 | ✅ | 不引入 Pinia、不引入 JSON Schema 表单库 |
| 禁止暴露内部错误 | ✅ | 拦截器统一中文 toast |

### 后端架构红线（`architecture/backend-red-lines.md`，前端相关）

| 红线 | 合规 | 说明 |
|------|------|------|
| 禁止 ADMIN 过度设计 | ✅ | 不做认证/权限、不做版本回滚、不做协同编辑、不做审批 |
| 禁止硬编码 | ✅ | 下拉选项从字典 API 动态获取 |

## 扩展性影响

**新增配置类型**（正面）：
- 项目骨架（`request.js`、`AppLayout.vue`、路由结构、Vite 配置、Docker 配置）直接复用
- 新模块只需加：`api/<module>.js` + `views/<Module>List.vue` + `views/<Module>Form.vue`
- 侧边栏 `AppLayout.vue` 加一个 `el-menu-item`

**新增表单字段类型**（正面）：
- 约束面板独立组件化，新类型加一个 `FieldConstraint<Type>.vue`
- `FieldForm.vue` 的 `v-if/v-else-if` 链加一个分支
- 字典表新增一行记录即可

## 依赖方向

```
views/FieldList.vue  ──→  api/fields.js      ──→  api/request.js
views/FieldForm.vue  ──→  api/dictionaries.js ──→  api/request.js
                     ──→  components/FieldConstraint*.vue

components/AppLayout.vue  ──→  vue-router

router/index.js  ──→  views/*
```

单向向下，无循环依赖。约束面板组件不依赖 API 层（数据由父组件 props 传入）。

## 陷阱检查（`development/frontend-pitfalls.md`）

| 陷阱 | 设计应对 |
|------|----------|
| ref 忘记 .value | `<script setup>` 中访问 ref 用 .value，模板自动解包 |
| reactive 不能整体替换 | 编辑页加载数据用 `Object.assign(form, apiData)`，不用 `form = reactive(newData)` |
| el-form prop 必须匹配 model 字段名 | 表单 `:model="form"` + `prop="name"` 严格对应 |
| el-dialog 表单残留 | 引用详情弹窗关闭时重置数据 |
| el-select v-model 类型匹配 | enabled 筛选 v-model 为 `null/true/false`，不混用字符串 |
| 拦截器吞错误 | 明确 `return Promise.reject(err)`，调用方 `.catch()` 能感知 |
| 双击重复提交 | `submitting` ref 控制按钮 loading + disabled |
| 同组件多路由不刷新 | `<router-view :key="route.fullPath" />` |
| 异步返回后组件已卸载 | API 回调中不做复杂状态修改（CRUD 页面简单赋值 Vue 自动处理） |
| v-for 必须 key | 选项列表/引用列表使用 value 或 id 作为 key，不用 index |
| computed 无副作用 | computed 仅用于派生显示数据，不在 computed 中发请求 |
| deep watcher 死循环 | 约束面板用 v-model emit 单向数据流，不用双向 deep watch |

## 配置变更

无新增配置文件。`vite.config.js` 中 `server.proxy` 已覆盖开发环境代理，`nginx.conf` 覆盖生产环境。

环境变量：`VITE_API_BASE`（可选，默认 `/api/v1`）。

## 测试策略

**本阶段不写自动化测试**（理由：前端为 CRUD 表单页面，交互逻辑简单；毕设阶段人工验证+后端 API 测试已覆盖核心逻辑）。

**人工验证清单**（对应验收标准 R1-R20）：
1. `npm run dev` 启动 → 页面正常加载
2. 列表筛选/分页 → 数据正确
3. 新建字段 → 保存成功，列表可见
4. 编辑字段 → 数据预填充，保存成功
5. 删除字段 → 三态确认正确
6. Toggle 启禁用 → 状态切换正确
7. 引用详情弹窗 → 数据正确
8. 约束面板切换 → 类型匹配
9. 标识符 blur 校验 → 三态反馈
10. Docker 构建 → `docker compose up --build` 前端可访问
