# FSM 状态字典前端管理页面 — 设计方案

## 方案描述

### 总体策略：克隆 EventTypeList/Form 模式

fsm-state-dict 不涉及动态表单（无 SchemaForm）、无嵌套子资源（无扩展字段 Schema），是最简单的 CRUD 页。
完全复刻 `EventTypeList.vue + EventTypeForm.vue` 模式，差异点只有：

1. **分类过滤**：category 下拉从 `list-categories` 动态加载（EventType 用静态枚举 perception_mode）
2. **删除被引用（43020）**：返回 `referenced_by` 列表，弹独立 dialog 展示（EventType 删除有被引用保护但走的是另一套）
3. **name 校验 pattern**：`^[a-z][a-z0-9_]*$`（EventType name 同规则）
4. **无编辑前置禁用要求**：fsm-state-dict 的 Update API 不要求先禁用，直接提交 version 即可

---

### 文件结构

```
frontend/src/
  api/
    fsmStateDicts.ts          # 新增：类型 + 错误码常量 + API 函数
  views/
    FsmStateDictList.vue      # 新增：列表页（搜索/分页/toggle/delete）
    FsmStateDictForm.vue      # 新增：创建/编辑/查看表单
  router/
    index.ts                  # 修改：追加 4 条路由
  components/
    AppLayout.vue             # 修改：新增 group-fsm 子菜单
    EnabledGuardDialog.vue    # 修改：EntityType 扩展 'fsm-state-dict'
```

---

### API 类型设计（fsmStateDicts.ts）

```typescript
// 列表查询
interface FsmStateDictListQuery {
  name?: string
  category?: string
  enabled?: boolean | null
  page: number
  page_size: number
}

// 列表项
interface FsmStateDictListItem {
  id: number
  name: string
  display_name: string
  category: string
  enabled: boolean
  created_at: string
}

// 完整详情（detail 接口返回）
interface FsmStateDict {
  id: number
  name: string
  display_name: string
  category: string
  description: string
  enabled: boolean
  version: number
  created_at: string
  updated_at: string
}

// 创建请求
interface CreateFsmStateDictRequest {
  name: string
  display_name: string
  category: string
  description?: string
}

// 编辑请求
interface UpdateFsmStateDictRequest {
  id: number
  display_name: string
  category: string
  description: string
  version: number
}

// FSM 引用信息（delete 43020 时返回）
interface FsmConfigRef {
  id: number
  name: string
  display_name: string
  enabled: boolean
}

// 删除结果
interface FsmStateDictDeleteResult {
  id: number
  name: string
  display_name: string
  referenced_by: FsmConfigRef[]
}

// 错误码常量
export const FSM_STATE_DICT_ERR = {
  NAME_EXISTS:        43013,
  NAME_INVALID:       43014,
  NOT_FOUND:          43015,
  DELETE_NOT_DISABLED: 43016,
  VERSION_CONFLICT:   43017,
  IN_USE:             43020,
} as const

// API 函数（路径对齐 T7 router）
export const fsmStateDictApi = {
  list:           (params) => POST('/fsm-state-dicts/list', params),
  create:         (data)   => POST('/fsm-state-dicts/create', data),
  detail:         (id)     => POST('/fsm-state-dicts/detail', { id }),
  update:         (data)   => POST('/fsm-state-dicts/update', data),
  delete:         (id)     => POST('/fsm-state-dicts/delete', { id }),
  checkName:      (name)   => POST('/fsm-state-dicts/check-name', { name }),
  toggleEnabled:  (id, enabled, version) => POST('/fsm-state-dicts/toggle-enabled', { id, enabled, version }),
  listCategories: ()       => POST('/fsm-state-dicts/list-categories', {}),
}
```

---

### FsmStateDictList.vue 结构

| 区域 | 内容 |
|------|------|
| 搜索栏 | name 模糊搜索 + category 下拉（动态 listCategories）+ enabled 三态下拉 + 搜索/重置 |
| 操作栏 | 「新建状态」按钮 |
| 表格 | 5 列：name / display_name / category / enabled（Switch）/ created_at + 操作列（查看/编辑/删除） |
| 分页 | 对齐 EventTypeList：default 20，total 后端返回 |

**Toggle 流程**（同 EventTypeList 精确实现）：
```
二次确认弹框 →
  确认 → detail(id) 取最新 version →
  toggleEnabled(id, !current, version) →
  成功：刷新该行；版本冲突(43017)：fetchList
```

**Delete 流程**（扩展 43020 分支）：
```
row.enabled → EnabledGuardDialog.open({ action: 'delete', entityType: 'fsm-state-dict', entity })
row.disabled → ElMessageBox.confirm →
  确认 → delete(id) →
    成功：fetchList
    43020 (InUse) → refDeleteDialog.open(referencedBy)  // 独立 dialog
    其他错误：拦截器 toast
```

---

### FsmStateDictForm.vue 结构

路由 meta 区分 `isCreate / isView / isEdit`（同 EventTypeForm）。

| 字段 | 类型 | 约束 | 查看态 |
|------|------|------|--------|
| name | el-input | 格式 `^[a-z][a-z0-9_]*$`，失焦 check-name；编辑/查看时只读 | 只读 |
| display_name | el-input | 必填，后端 128 字符上限 | 只读 |
| category | el-input + datalist | 必填，可从已有分类下拉选择或自由输入 | 只读 |
| description | el-input type=textarea | 可选，512 字符上限 | 只读 |

**name 唯一性检查**（同 EventTypeForm nameStatus 状态机）：
```
'' → 'checking' → 'available' | 'taken'
el-form-item show-message 条件渲染
```

**版本冲突处理（43017）**：
```
catch (43017) → ElMessageBox.alert('数据已更新，请刷新后重试') → 不跳转
```

---

### AppLayout.vue 修改

在 `group-event` 的 `el-sub-menu` 之后追加：

```html
<el-sub-menu index="group-fsm">
  <template #title>
    <el-icon class="group-icon"><Cpu /></el-icon>
    <span class="group-title">状态机管理</span>
  </template>
  <el-menu-item index="/fsm-state-dicts">
    <el-icon><Collection /></el-icon>
    <span>状态字典</span>
  </el-menu-item>
</el-sub-menu>
```

`defaultOpeneds` 追加 `'group-fsm'`。

图标从 `@element-plus/icons-vue` 导入 `Cpu`、`Collection`（已包含在 Element Plus，不新增依赖）。

---

### EnabledGuardDialog.vue 修改

三处：
1. `type EntityType` 联合类型追加 `'fsm-state-dict'`
2. `entityTypeLabel` computed 追加 `if (entityType.value === 'fsm-state-dict') return '状态字典'`
3. `onActOnce()` 追加分支：
   ```typescript
   } else if (entityType.value === 'fsm-state-dict') {
     const detail = await fsmStateDictApi.detail(id)
     await fsmStateDictApi.toggleEnabled(id, false, detail.data.version)
   }
   ```
   以及 `conflictCode` 分支赋值 `FSM_STATE_DICT_ERR.VERSION_CONFLICT`。

---

### 路由设计（router/index.ts）

```typescript
{ path: '/fsm-state-dicts',          component: FsmStateDictList,
  meta: { requiresAuth: false } },
{ path: '/fsm-state-dicts/create',   component: FsmStateDictForm,
  meta: { isCreate: true } },
{ path: '/fsm-state-dicts/:id/edit', component: FsmStateDictForm,
  meta: { isEdit: true } },
{ path: '/fsm-state-dicts/:id/view', component: FsmStateDictForm,
  meta: { isView: true } },
```

---

### 43020 InUse Dialog

FsmStateDictList 内含一个 `ref-delete-dialog`（el-dialog，非独立组件），展示：
- 标题：`无法删除「${currentDict.display_name}」`
- 提示文字：`以下 FSM 配置引用了此状态，请先修改再删除`
- 表格：name / display_name / enabled（tag 标签：启用绿 / 停用灰）
- 底部：「知道了」关闭按钮

不提取为独立组件（只有一处使用）。

---

## 方案对比

### 备选方案：提取 StateDictRefDialog 为独立组件

**优点**：复用性高；若 BT/FSM 配置页也需要引用弹窗可复用。

**不选理由**：当前只有一处使用，提前抽象属于过度设计。若未来 fsm-management-frontend 需要，届时再提取。

### 备选方案：category 使用 el-select + allow-create

**优点**：交互更清晰，明确区分"选已有"和"新建"。

**不选理由**：Element Plus `el-select allow-create` 存在 v-model 绑定边缘情况（allow-create + filterable 时 v-model 在未选中时可能为 undefined）。使用原生 `<datalist>` + `el-input` 更稳定可控，且无需额外处理边界。

---

## 红线检查

### 前端红线（docs/development/standards/red-lines/frontend.md）

| 红线 | 状态 |
|------|------|
| el-form 必须通过 validate() 提交 | ✅ FsmStateDictForm 统一走 formRef.validate() |
| reactive 解构必须用 toRefs | ✅ 使用 ref<T>() 存列表/详情，无 reactive 解构 |
| el-form :disabled 用 `:disabled="isView"` 整体禁用 | ✅ form 级别禁用；name 字段额外 `:disabled="isView \|\| isEdit"` |
| 所有子字段 `:disabled` 保持 `isView \|\| 条件` 格式 | ✅ name 字段仅在 create 时可编辑 |
| 所有下拉从后端动态加载 | ✅ category 走 listCategories API |
| 不硬编码 baseURL | ✅ 继承 request.ts 的 baseURL env 配置 |

### ADMIN 红线（docs/development/admin/red-lines.md）

| 红线 | 状态 |
|------|------|
| 写操作后必须刷新缓存对应的列表 | N/A（纯前端） |
| 版本冲突必须提示用户，不自动重试 | ✅ 43017 → ElMessageBox.alert，不跳转 |
| Toggle enabled 前必须 detail() 拉取最新 version | ✅ handleToggle 走 detail → toggleEnabled |
| delete 已启用资源→ EnabledGuardDialog，不直接删除 | ✅ |

---

## 扩展性影响

- **正面**：新增一种独立配置管理类型，完全遵循「只加一组 views/api/router/sidebar 条目」扩展方向，不改动任何已有业务逻辑。
- **未来 fsm-management-frontend**：届时在 `group-fsm` 追加 `<el-menu-item index="/fsm-configs">` 即可，不影响本次改动。

---

## 依赖方向

```
FsmStateDictList.vue ──→ fsmStateDicts.ts (api)
FsmStateDictForm.vue ──→ fsmStateDicts.ts (api)
AppLayout.vue        ──→ (无新依赖，仅添加菜单项)
EnabledGuardDialog.vue → fsmStateDicts.ts (api, 新增 import)
router/index.ts      ──→ FsmStateDictList, FsmStateDictForm (懒加载)
```

所有新依赖单向向下（views → api），无循环。

---

## 陷阱检查（frontend.md）

| 检查项 | 结论 |
|--------|------|
| reactive 整体替换 bug | 不用 reactive，全用 ref<T>，安全 |
| el-form-item prop 与 model 对齐 | 表单字段少（4个），手动对齐，不涉及动态渲染 |
| dialog 关闭时重置表单 | Form 是独立路由页，不用 dialog，无此问题 |
| v-for :key 稳定 | 列表用 `:key="row.id"` |
| 按钮 loading 防重复提交 | submitLoading ref，提交时设 true |
| 后端 null 返回不 crash | `referenced_by ?? []` 防御 |
| category datalist 边界 | 输入值直接绑 v-model，无 undefined 风险 |
| `npx vue-tsc --noEmit` 通过 | 所有 API 返回类型显式声明，不用 any |

---

## 配置变更

无需修改任何配置文件。后端 `config.yaml` 中 `fsm_state_dict` 节已在 `fsm-state-dict-backend` 写入。

---

## 测试策略

### 构建验证（自动）
- `npx vue-tsc --noEmit` — TypeScript 无错误
- `npm run build` — Vite 构建通过

### 浏览器手动验证（按 R1–R13）
| 场景 | 验证步骤 |
|------|----------|
| R1 侧边栏 | 看到「状态机管理 → 状态字典」，点击跳转到 /fsm-state-dicts |
| R2 列表展示 | 31 条种子数据分页显示，5 列正确 |
| R3 组合过滤 | name 模糊搜索、category 下拉精确、enabled 三态各自验证 |
| R4 新建表单校验 | name 格式错误提示、display_name 空提示、失焦 check-name 重复提示 |
| R5 创建成功 | 跳回列表，43013 留在表单 |
| R6 编辑 | name 只读，version 携带正确 |
| R7 版本冲突 | 手动在 DB 改 version 后提交，弹窗提示 |
| R8 Toggle | Switch 点击二次确认，成功刷新该行 |
| R9 删除（无引用）| 已停用状态删除成功 |
| R10 删除（启用态）| EnabledGuardDialog 弹出 |
| R11 删除（有引用）| 43020 弹窗显示 referenced_by 表格 |
| R12 查看页 | 所有字段只读，含时间戳 |
| R13 构建 | vue-tsc + build 双通过 |
