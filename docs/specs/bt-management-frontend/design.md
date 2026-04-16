# bt-management-frontend — 设计方案

> 对应需求：[requirements.md](./requirements.md)

---

## 1. 方案描述

### 1.1 总体结构

```
frontend/src/
  api/
    btTrees.ts              # 行为树 CRUD API + 类型 + 错误码
    btNodeTypes.ts          # 节点类型 CRUD API + 类型 + 错误码
  views/
    BtTreeList.vue          # 行为树列表页
    BtTreeForm.vue          # 行为树新建/编辑/查看（宽布局）
    BtNodeTypeList.vue      # 节点类型列表页（系统设置）
    BtNodeTypeForm.vue      # 节点类型新建/编辑/查看
  components/
    BtNodeEditor.vue        # 树编辑器（接收 BtNodeInternal | null，emit update:modelValue）
    BtNodeTypeSelector.vue  # 节点类型选择对话框（BtNodeEditor 的子组件）
    BtParamSchemaEditor.vue # param_schema 行编辑器（BtNodeTypeForm 的子组件）
    EnabledGuardDialog.vue  # [修改] 新增 bt-tree / bt-node-type entityType
  router/index.ts           # [修改] 追加 8 条路由
```

---

### 1.2 API 层类型定义

#### `btTrees.ts`

```typescript
// ─── 内部树结构（编辑器用） ───

/** 编辑器内部节点表示（params 单独存，序列化时展开到顶层） */
export interface BtNodeInternal {
  type: string
  category: 'composite' | 'decorator' | 'leaf'
  params: Record<string, unknown>          // 参数值，key 对应 param_schema.params[].name
  children?: BtNodeInternal[]             // composite 用
  child?: BtNodeInternal | null           // decorator 用
}

// ─── 列表 ───

export interface BtTreeListQuery {
  name?: string
  display_name?: string
  enabled?: boolean | null
  page: number
  page_size: number
}

export interface BtTreeListItem {
  id: number
  name: string
  display_name: string
  enabled: boolean
  created_at: string
}

// ─── 详情 ───

export interface BtTreeDetail {
  id: number
  name: string
  display_name: string
  description: string
  config: unknown                          // 原始 JSON，由 deserializeBtNode 转换
  enabled: boolean
  version: number
}

// ─── 请求 ───

export interface CreateBtTreeRequest {
  name: string
  display_name: string
  description: string
  config: unknown                          // serializeBtNode 序列化后的对象
}

export interface UpdateBtTreeRequest {
  id: number
  version: number
  display_name: string
  description: string
  config: unknown
}

// ─── 错误码（对应 errcode/codes.go 44001–44012） ───

export const BT_TREE_ERR = {
  NAME_EXISTS:         44001,
  NAME_INVALID:        44002,
  NOT_FOUND:           44003,
  CONFIG_INVALID:      44004,
  NODE_TYPE_NOT_FOUND: 44005,
  DEPTH_EXCEEDED:      44006,
  DELETE_NOT_DISABLED: 44009,
  EDIT_NOT_DISABLED:   44010,
  VERSION_CONFLICT:    44011,
} as const

// ─── API 对象 ───

export const btTreeApi = {
  list: (params: BtTreeListQuery) =>
    request.get('/bt-trees', { params }) as Promise<ApiResponse<ListData<BtTreeListItem>>>,

  create: (data: CreateBtTreeRequest) =>
    request.post('/bt-trees', data) as Promise<ApiResponse<{ id: number; name: string }>>,

  detail: (id: number) =>
    request.post('/bt-trees/detail', { id }) as Promise<ApiResponse<BtTreeDetail>>,

  update: (data: UpdateBtTreeRequest) =>
    request.post('/bt-trees/update', data) as Promise<ApiResponse<string>>,

  delete: (id: number) =>
    request.post('/bt-trees/delete', { id }) as Promise<ApiResponse<{ id: number; name: string; label: string }>>,

  checkName: (name: string) =>
    request.post('/bt-trees/check-name', { name }) as Promise<ApiResponse<CheckNameResult>>,

  toggleEnabled: (id: number, enabled: boolean, version: number) =>
    request.post('/bt-trees/toggle-enabled', { id, enabled, version }) as Promise<ApiResponse<string>>,
}
```

#### `btNodeTypes.ts`

```typescript
// ─── 节点类型元信息（树编辑器用） ───

/** param_schema 中单条参数定义 */
export interface BtParamDef {
  name: string
  label: string
  type: 'bb_key' | 'string' | 'float' | 'integer' | 'bool' | 'select'
  required: boolean
  options?: string[]           // select 类型时有值
}

/** 节点类型完整元信息（树编辑器加载用） */
export interface BtNodeTypeMeta {
  id: number
  type_name: string
  category: 'composite' | 'decorator' | 'leaf'
  label: string
  params: BtParamDef[]         // 从 param_schema.params 解析
}

// ─── 列表 ───

export interface BtNodeTypeListQuery {
  type_name?: string
  category?: string            // '' | 'composite' | 'decorator' | 'leaf'
  enabled?: boolean | null
  page: number
  page_size: number
}

export interface BtNodeTypeListItem {
  id: number
  type_name: string
  category: string
  label: string
  is_builtin: boolean
  enabled: boolean
}

// ─── 详情 ───

export interface BtNodeTypeDetail {
  id: number
  type_name: string
  category: string
  label: string
  description: string
  param_schema: { params: BtParamDef[] }
  is_builtin: boolean
  enabled: boolean
  version: number
}

// ─── 请求 ───

export interface CreateBtNodeTypeRequest {
  type_name: string
  category: string
  label: string
  description: string
  param_schema: { params: BtParamDef[] }
}

export interface UpdateBtNodeTypeRequest {
  id: number
  version: number
  label: string
  description: string
  param_schema: { params: BtParamDef[] }
}

// ─── 错误码（对应 errcode/codes.go 44016–44026） ───

export const BT_NODE_TYPE_ERR = {
  NAME_EXISTS:          44016,
  NAME_INVALID:         44017,
  NOT_FOUND:            44018,
  CATEGORY_INVALID:     44019,
  DELETE_NOT_DISABLED:  44020,
  EDIT_NOT_DISABLED:    44021,
  REF_DELETE:           44022,
  BUILTIN_DELETE:       44023,
  BUILTIN_EDIT:         44024,
  PARAM_SCHEMA_INVALID: 44025,
  VERSION_CONFLICT:     44026,
} as const
```

---

### 1.3 序列化 / 反序列化

`BtNodeInternal`（编辑器内部）↔ 后端 config JSON（params 展开到顶层）的转换逻辑放在 `btTrees.ts`。

```typescript
/** 将编辑器内部结构序列化为后端 config JSON */
export function serializeBtNode(node: BtNodeInternal): Record<string, unknown> {
  const out: Record<string, unknown> = { type: node.type, ...node.params }
  if (node.category === 'composite') {
    out.children = (node.children ?? []).map(serializeBtNode)
  } else if (node.category === 'decorator') {
    out.child = node.child ? serializeBtNode(node.child) : null
  }
  return out
}

/** 将后端 config JSON 反序列化为编辑器内部结构 */
export function deserializeBtNode(
  json: Record<string, unknown>,
  typeMap: Map<string, BtNodeTypeMeta>,
): BtNodeInternal {
  const { type, children, child, ...rest } = json
  const meta = typeMap.get(type as string)
  const category = meta?.category ?? 'leaf'
  const paramNames = new Set(meta?.params.map((p) => p.name) ?? [])
  const params: Record<string, unknown> = {}
  for (const [k, v] of Object.entries(rest)) {
    if (paramNames.has(k)) params[k] = v
  }
  const node: BtNodeInternal = { type: type as string, category, params }
  if (category === 'composite' && Array.isArray(children)) {
    node.children = children.map((c) => deserializeBtNode(c as Record<string, unknown>, typeMap))
  }
  if (category === 'decorator' && child) {
    node.child = deserializeBtNode(child as Record<string, unknown>, typeMap)
  }
  return node
}
```

---

### 1.4 BtNodeEditor 组件设计

**职责分离**：

- `BtNodeEditor.vue`：顶层容器，**不做 API 调用**。接收 `nodeTypes: BtNodeTypeMeta[]`（由 BtTreeForm 在 onMounted 加载后传入），以及 `modelValue: BtNodeInternal | null`，对外 emit `update:modelValue`。
- `BtNodeTypeSelector.vue`：节点类型选择对话框，父组件传入 `nodeTypes`，按 category 三组展示，emit `select`。

```typescript
// BtNodeEditor props
interface Props {
  modelValue: BtNodeInternal | null
  nodeTypes: BtNodeTypeMeta[]
  disabled?: boolean
  depth?: number               // 缩进层级，顶层默认 0
}
```

**渲染逻辑（伪代码）**：

```
if depth===0 && !modelValue && !disabled:
  → 显示"添加根节点"按钮，点击打开 BtNodeTypeSelector

if modelValue:
  渲染节点卡片：
    头部：[category标签] label (type_name) — params摘要
    if !disabled: 节点右侧 Edit + Delete 按钮

  if category === 'composite':
    if !disabled: Add Child 按钮（打开 BtNodeTypeSelector，追加到 children）
    v-for child in children:
      <BtNodeEditor :modelValue="child" :depth="depth+1" ... />

  if category === 'decorator':
    if !disabled: Set Child 按钮（打开 BtNodeTypeSelector，覆盖 child）
    if child:
      <BtNodeEditor :modelValue="child" :depth="depth+1" ... />
    else:
      占位提示"暂未设置子节点"

  if category === 'leaf':
    内联参数表单（按 param_schema 动态渲染）
    编辑展开由节点 Edit 按钮控制（每节点独立 showParams ref）
```

**Vue 3 自递归**：`<script setup>` 文件名即为组件名，BtNodeEditor.vue 可直接在模板中使用 `<BtNodeEditor>` 自引用，与 FsmConditionEditor.vue 相同模式。

**内联参数表单 param type 映射**：

| param.type | 渲染组件 | 注意 |
|------------|----------|------|
| `bb_key` | `BBKeySelector` | `:disabled="isView \|\| false"` 显式写法 |
| `select` | `el-select`（选项来自 `param.options`） | 枚举值必须用 select，不允许手输 |
| `float` | `el-input-number :precision="2"` | 不随父容器拉伸，style="width: 200px" |
| `integer` | `el-input-number :precision="0" :step="1"` | 同上 |
| `bool` | `el-select`（true/false 两项） | 枚举用 select，不用 checkbox |
| `string` | `el-input` | |

**节点缩进**：每层 `margin-left: 24px`，用 inline style 绑定 `:style="{ marginLeft: depth * 24 + 'px' }"`，不用嵌套 class。

---

### 1.5 BtParamSchemaEditor 设计

```typescript
interface ParamRow {
  name: string
  label: string
  type: 'bb_key' | 'string' | 'float' | 'integer' | 'bool' | 'select'
  required: boolean
  optionsText: string          // select 类型时，逗号分隔的选项字符串（UI 友好）
}

// 对外
defineExpose({
  validate(): string | null    // 通过返回 null，失败返回错误描述
})

// 对外传入 / 传出
defineProps<{ modelValue: BtParamDef[]; disabled?: boolean }>()
defineEmits<{ 'update:modelValue': [BtParamDef[]] }>()
```

`validate()` 检查：
- 每行 name 和 label 非空
- name 格式 `^[a-z][a-z0-9_]*$`
- name 不重复
- select 类型的 options 非空

---

### 1.6 路由追加（8 条）

```typescript
{ path: '/bt-trees',             meta: { title: '行为树管理' }, ... }
{ path: '/bt-trees/create',      meta: { title: '新建行为树', isCreate: true,  isView: false }, ... }
{ path: '/bt-trees/:id/view',    meta: { title: '查看行为树', isCreate: false, isView: true  }, ... }
{ path: '/bt-trees/:id/edit',    meta: { title: '编辑行为树', isCreate: false, isView: false }, ... }
{ path: '/bt-node-types',        meta: { title: '节点类型管理' }, ... }
{ path: '/bt-node-types/create', meta: { title: '新建节点类型', isCreate: true,  isView: false }, ... }
{ path: '/bt-node-types/:id/view', meta: { title: '查看节点类型', isCreate: false, isView: true }, ... }
{ path: '/bt-node-types/:id/edit', meta: { title: '编辑节点类型', isCreate: false, isView: false }, ... }
```

路由参数 `id` 为数字型，无含 `/` 的 URL 段，**不存在 URL 编码问题**（name 含 `/` 只在 POST body 传输）。

---

### 1.7 EnabledGuardDialog 修改点

在 `EntityType` union 追加：`'bt-tree' | 'bt-node-type'`

新增映射：
- `entityTypeLabel`：`'bt-tree' → '行为树'`，`'bt-node-type' → '节点类型'`
- `reasonText`（edit）：行为树："已启用的行为树对游戏服务端可见…"；节点类型："已启用的节点类型被树编辑器使用…"
- `onActOnce`：调 `btTreeApi.toggleEnabled` / `btNodeTypeApi.toggleEnabled`
- edit 路由跳转：`/bt-trees/${id}/edit` / `/bt-node-types/${id}/edit`
- `conflictCode`：`BT_TREE_ERR.VERSION_CONFLICT` / `BT_NODE_TYPE_ERR.VERSION_CONFLICT`

---

### 1.8 删除行为说明

| 实体 | 启用中 | 禁用中 |
|------|--------|--------|
| 行为树 | EnabledGuardDialog | ElMessageBox 确认 → delete，VERSION_CONFLICT 刷新 |
| 节点类型（自定义） | EnabledGuardDialog | 直接 delete；REF_DELETE(44022) → 引用弹窗（列出树名）；BUILTIN_DELETE 不会触发（列表不渲染内置的删除按钮） |
| 节点类型（内置） | 操作列只有"查看"，无删除按钮 | 同左 |

节点类型删除**对齐 FieldList / EventTypeSchemaList 的引用弹窗模式**：
- 无独立 `/references` 接口，直接调 delete
- 44022 时读 `(bizErr.data as { referenced_by: string[] }).referenced_by` 弹 `el-dialog`，列出引用该类型的行为树 name 列表
- 弹窗结构与 FieldList 引用详情弹窗一致（标题 + el-table 列名称列）

---

## 2. 备选方案及对比

### 方案 B：可视化图形编辑器（节点拖拽画布）

用 Vue Flow / AntV G6 等图形库实现树形拖拽编辑，每个节点是可拖动的卡片，连线表示父子关系。

**不选原因**：
1. 引入大型图形库，打包体积增加 200KB+，超出项目规模
2. BT 深度一般 3–5 层，缩进卡片完全可读，图形化收益边际
3. 开发成本是缩进卡片的 3–5 倍，不符合毕设时间约束
4. 图形编辑器对 `param_schema` 动态表单的嵌入支持不佳

### 方案 C：原文 JSON 编辑器

直接提供 `el-input type="textarea"` 让运营手写 config JSON。

**不选原因**：
1. 违反 red-lines.md §6.2"禁止让策划手写 JSON"
2. 违反 red-lines.md §6.5"节点类型用'中文标签 (english)'格式"——JSON 中必须写 `type_name` 英文标识

---

## 3. 红线检查

### 通用红线 (`standards/red-lines/general.md`)

无违反：不引入外部大依赖，不硬编码配置。

### Go 红线

不涉及（纯前端）。

### 前端红线 (`standards/red-lines/frontend.md`)

| 条目 | 检查结果 |
|------|----------|
| 禁止枚举值用自由文本输入 | ✓ bb_key 用 BBKeySelector，op 用 el-select，result 用 el-select，category 用 el-select |
| 禁止 `el-form :disabled` 被子组件覆盖 | ✓ BtNodeEditor 所有子组件写 `:disabled="isView \|\| condition"` |
| 禁止跳过 vue-tsc | ✓ 提交前必跑 `npx vue-tsc --noEmit` |
| 禁止 reactive 不带显式泛型 | ✓ 所有 reactive 写显式接口泛型 |
| BT name 含 `/` 的 URL 编码 | ✓ name 只在 POST body 传输，无 URL 路径段，不存在编码问题 |
| 装饰节点 ≠ 复合节点 | ✓ decorator 用 `child`（单对象），composite 用 `children`（数组） |

### ADMIN 专属红线 (`admin/red-lines.md`)

| 条目 | 检查结果 |
|------|----------|
| §1.4 禁止将 decorator 归类为 composite | ✓ BtNodeInternal.category 严格区分，渲染分支独立 |
| §1.5 禁止放行服务端不支持的枚举（op/result） | ✓ op/result 均用 el-select，选项来自 param_schema，不允许手输 |
| §6.2 禁止让策划手写 JSON | ✓ 结构化树编辑器 |
| §6.4 表单字段必须有提示文字 | ✓ 每个字段下方加灰色 hint |
| §6.5 节点类型用"中文标签 (english)"格式 | ✓ 选择器和卡片头均显示"序列 (sequence)" |
| §7.3 删除确认必须明确对象名 | ✓ 行为树删除弹窗含 display_name + name |
| §7.4 启用/禁用必须弹确认弹窗 | ✓ Switch onChange 先弹 ElMessageBox.confirm |
| §7.5 乐观锁必须先 detail 拿 version | ✓ Toggle 操作先调 detail |
| §8.1 启用中编辑/删除走 EnabledGuardDialog | ✓ GuardDialog 扩展支持 bt-tree / bt-node-type |
| §12 form disabled 子组件必须显式写 | ✓ 见上 |
| §13 业务错误码必须逐一处理 | ✓ BtTreeForm catch 处理 44001/44002/44003/44004/44005/44006/44011；BtNodeTypeForm 处理 44016/44017/44018/44019/44024/44025/44026 |

---

## 4. 扩展性影响

**正面**：
- `BtNodeEditor` 通过 `param_schema` 动态渲染表单，新增 leaf 节点类型（如 `patrol_action`）只需在"节点类型管理"页添加记录，**前端零代码改动**
- 如果新增 param type（如 `json`），只需在 `BtNodeEditor` 的 `renderParamInput` 加一个 case，不影响 API 层或 Views 层

**无负面影响**：
- 本次改动均为新增文件，只在 `EnabledGuardDialog.vue` 和 `router/index.ts` 追加内容，不修改已有功能逻辑

---

## 5. 依赖方向

```
BtTreeList.vue      → btTrees.ts
BtTreeForm.vue      → btTrees.ts, btNodeTypes.ts, BtNodeEditor.vue
BtNodeEditor.vue    → (自递归), BtNodeTypeSelector.vue, BBKeySelector.vue, btTrees.ts(类型)
BtNodeTypeSelector  → (纯 UI，无 API 调用)
BtNodeTypeList.vue  → btNodeTypes.ts
BtNodeTypeForm.vue  → btNodeTypes.ts, BtParamSchemaEditor.vue
BtParamSchemaEditor → (纯 UI，无 API 调用)
EnabledGuardDialog  → btTrees.ts, btNodeTypes.ts (追加)
router/index.ts     → 各 views
```

单向：views → api，components 无直接 api 调用（BtNodeEditor 纯受控组件）。

---

## 6. 陷阱检查

### 前端开发规范 dev-rules/frontend.md

| 陷阱 | 应对 |
|------|------|
| §2.6 v-for 必须稳定 key | BtNodeEditor 中 children 使用节点 index+type 组合 key（`${index}-${node.type}`），**不用纯 index** |
| §2.3 reactive 不能整体替换 | BtTreeForm 加载 detail 用 `Object.assign(formState, ...)` |
| §3.3 el-dialog 关闭数据残留 | BtNodeTypeSelector 在 `@close` 重置 selectedType |
| §3.4 el-select v-model 类型一致 | enabled 三态用 `boolean \| null`，与 el-select option value 类型匹配 |
| §9 装饰节点用 child 单对象 | ✓ 在 BtNodeEditor.vue §9 已明确 |
| §10 同组件多路由不刷新 | BtTreeForm 和 BtNodeTypeForm 用 `watch(() => route.fullPath, reload)` |

### BtNodeEditor 特有陷阱

1. **BtNodeInternal 深拷贝**：删除节点、添加节点时必须返回新对象而非 mutate，保持响应式追踪。用 `structuredClone()` 深拷贝后修改。

2. **deserializeBtNode 处理未知类型**：后端可能存有已被禁用的节点类型（view / edit 模式加载已有配置时）。`typeMap` 中找不到 type_name 时，category 降级为 `'leaf'`，params 保留全部 key-value 原始数据（不丢数据）。节点卡片正常渲染，显示 `type_name` 即可，无需标注"未知"——对齐 BBKeySelector + FsmConditionEditor 的**静默降级**模式（`allow-create` 保留已有值，`el-input` 兜底，不报错不阻止）。edit 模式下保存时 `serializeBtNode` 照原样输出 params，数据不丢失。

3. **BtTreeForm 宽布局**：树编辑器区的 `form-card` 内用 `.form-body-wide`（max-width: 1200px），不用默认 800px。

4. **param_schema 解析时机**：`BtNodeTypeMeta.params` 从 `param_schema.params` 解析，在 `btNodeTypeApi.list` 返回后统一做一次，不在每次渲染时重新解析。

5. **节点类型列表不含 label 模糊搜索**：后端 `BtNodeTypeListQuery` 无 label 字段（仅 type_name/category/enabled），对齐 `EventTypeSchemaList` 模式——前端搜索项严格对齐后端 query 能力，后端没有就不放。节点类型列表只提供 type_name 前缀匹配 + category 精确 + enabled 三个筛选项。

---

## 7. 配置变更

无。不涉及 `config.yaml` / `docker-compose.yml` 改动。

---

## 8. 测试策略

测试方式：手动 E2E（bash curl 脚本 + 浏览器操作），与其他前端模块一致，不写 Vitest 单元测试（项目没有前端测试框架）。

**E2E 验证矩阵**：

| # | 操作 | 预期 |
|---|------|------|
| E1 | 新建行为树（空 config）→ 保存 | 后端报 ErrBtTreeConfigInvalid，前端显示后端消息 |
| E2 | 新建行为树（sequence → check_bb_float + stub_action）→ 保存 | 201 成功，列表出现新行，默认禁用 |
| E3 | 编辑已禁用行为树，修改 display_name + 树结构 → 保存 | 200 成功，VERSION_CONFLICT 弹窗测试（另标签页提前改 version） |
| E4 | 对已启用行为树点编辑 | 弹 EnabledGuardDialog；点"立即禁用"后跳编辑页 |
| E5 | 删除已禁用行为树 | 弹确认后软删除，列表消失 |
| E6 | 查看模式 | 树编辑器 disabled，无 Add/Delete/Edit 按钮 |
| E7 | 新建自定义节点类型（leaf，含 bb_key 和 select 参数）→ 树编辑器中使用该类型 | 选择器出现新类型，保存后导出包含该节点 |
| E8 | 删除被引用的节点类型（先在树中使用）| 44022 错误，ElMessage.error 显示被引用的树名 |
| E9 | 启用行为树 → GET /api/configs/bt_trees | 响应 items 中含该树，config 字段 params 展开到顶层 |
