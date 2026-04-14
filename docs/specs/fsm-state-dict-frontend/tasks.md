# fsm-state-dict-frontend — 任务列表

## 状态

- [x] T1: API 封装 + TypeScript 类型（fsmStateDicts.ts）
- [x] T2: 路由 + 侧边栏 + EnabledGuardDialog 扩展
- [x] T3: FsmStateDictList.vue — 列表页
- [x] T4: FsmStateDictForm.vue — 创建/编辑/查看表单

---

## T1：API 封装 + TypeScript 类型 (R4, R5, R6, R7, R8, R9, R10, R11)

**涉及文件**：
- `frontend/src/api/fsmStateDicts.ts`（新增）

**做什么**：

新建 `fsmStateDicts.ts`，内容：

1. **类型定义**：
   - `FsmStateDictListQuery`：name?/category?/enabled?（boolean|null）/page/page_size
   - `FsmStateDictListItem`：id/name/display_name/category/enabled/created_at
   - `FsmStateDict`（详情）：上述字段 + description/version/updated_at
   - `CreateFsmStateDictRequest`：name/display_name/category/description?
   - `UpdateFsmStateDictRequest`：id/display_name/category/description/version
   - `FsmConfigRef`：id/name/display_name/enabled
   - `FsmStateDictDeleteResult`：id/name/display_name/referenced_by: FsmConfigRef[]

2. **错误码常量**：
   ```typescript
   export const FSM_STATE_DICT_ERR = {
     NAME_EXISTS:          43013,
     NAME_INVALID:         43014,
     NOT_FOUND:            43015,
     DELETE_NOT_DISABLED:  43016,
     VERSION_CONFLICT:     43017,
     IN_USE:               43020,
   } as const
   ```

3. **API 函数**（`fsmStateDictApi` 对象，8 个方法）：
   - `list(params: FsmStateDictListQuery)` → `ApiResponse<ListData<FsmStateDictListItem>>`
   - `create(data: CreateFsmStateDictRequest)` → `ApiResponse<{ id: number; name: string }>`
   - `detail(id: number)` → `ApiResponse<FsmStateDict>`
   - `update(data: UpdateFsmStateDictRequest)` → `ApiResponse<string>`
   - `delete(id: number)` → `ApiResponse<FsmStateDictDeleteResult>`
   - `checkName(name: string)` → `ApiResponse<CheckNameResult>`（从 `./fields` 复用类型）
   - `toggleEnabled(id, enabled, version)` → `ApiResponse<string>`
   - `listCategories()` → `ApiResponse<string[]>`

   路径前缀：`/fsm-state-dicts/`，全部用 `request.post`。

**做完是什么样**：`npx vue-tsc --noEmit` 通过；`fsmStateDictApi.list` 可从其他文件 import 使用。

---

## T2：路由 + 侧边栏 + EnabledGuardDialog 扩展 (R1)

**涉及文件**：
- `frontend/src/router/index.ts`（修改）
- `frontend/src/components/AppLayout.vue`（修改）
- `frontend/src/components/EnabledGuardDialog.vue`（修改）

**做什么**：

### router/index.ts
在 `event-type-schemas` 路由组之后、`not-found` 路由之前追加 4 条路由：
```typescript
{
  path: '/fsm-state-dicts',
  name: 'fsm-state-dict-list',
  component: () => import('@/views/FsmStateDictList.vue'),
  meta: { title: '状态字典' },
},
{
  path: '/fsm-state-dicts/create',
  name: 'fsm-state-dict-create',
  component: () => import('@/views/FsmStateDictForm.vue'),
  meta: { title: '新建状态', isCreate: true },
},
{
  path: '/fsm-state-dicts/:id/view',
  name: 'fsm-state-dict-view',
  component: () => import('@/views/FsmStateDictForm.vue'),
  meta: { title: '查看状态', isCreate: false, isView: true },
},
{
  path: '/fsm-state-dicts/:id/edit',
  name: 'fsm-state-dict-edit',
  component: () => import('@/views/FsmStateDictForm.vue'),
  meta: { title: '编辑状态', isCreate: false },
},
```

### AppLayout.vue
1. 导入新图标：`Cpu, Collection`（加入 import from `@element-plus/icons-vue`）
2. 在 `group-event` 的 `</el-sub-menu>` 之后追加：
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
3. `defaultOpeneds` 数组追加 `'group-fsm'`

### EnabledGuardDialog.vue
1. `type EntityType` 联合类型追加 `| 'fsm-state-dict'`
2. `entityTypeLabel` computed 追加分支：
   `if (entityType.value === 'fsm-state-dict') return '状态字典'`
3. 导入 `fsmStateDictApi, FSM_STATE_DICT_ERR` from `@/api/fsmStateDicts`
4. `onActOnce()` 中追加分支（在 `} else {` 模板 fallback 之前）：
   ```typescript
   } else if (entityType.value === 'fsm-state-dict') {
     const detail = await fsmStateDictApi.detail(id)
     await fsmStateDictApi.toggleEnabled(id, false, detail.data.version)
   }
   ```
5. `conflictCode` 赋值追加分支：
   ```typescript
   } else if (entityType.value === 'fsm-state-dict') {
     conflictCode = FSM_STATE_DICT_ERR.VERSION_CONFLICT
   }
   ```
6. 删除场景不需要跳转路由，EnabledGuardDialog 的编辑跳转路径也追加：
   ```typescript
   } else if (entityType.value === 'fsm-state-dict') {
     path = `/fsm-state-dicts/${id}/edit`
   }
   ```

**做完是什么样**：侧边栏出现「状态机管理 → 状态字典」；访问 `/fsm-state-dicts` 不 404（页面组件文件尚不存在时可先创建空壳）；`npx vue-tsc --noEmit` 通过。

---

## T3：FsmStateDictList.vue — 列表页 (R1, R2, R3, R8, R9, R10, R11)

**涉及文件**：
- `frontend/src/views/FsmStateDictList.vue`（新增）

**做什么**：

克隆 `EventTypeList.vue` 结构，按以下差异实现：

**搜索区**：
- name 文字输入框（模糊搜索）
- category 下拉：`el-select`，选项从 `fsmStateDictApi.listCategories()` 在 `onMounted` 加载，追加「全部」选项
- enabled 三态下拉：全部 / 启用 / 停用（同 EventTypeList）

**表格 5 列**：
- name（宽度 160）
- display_name（宽度 160）
- category（宽度 120）
- enabled：`el-switch`，`@change` 触发 `handleToggle(row)`（二次确认 + detail + toggle）
- created_at（宽度 180）
- 操作列：查看 / 编辑 / 删除（`el-button link`）

**Toggle 逻辑**（精确实现）：
```typescript
async function handleToggle(row: FsmStateDictListItem) {
  try {
    await ElMessageBox.confirm(...)
    const detailRes = await fsmStateDictApi.detail(row.id)
    await fsmStateDictApi.toggleEnabled(row.id, !row.enabled, detailRes.data.version)
    fetchList()
  } catch (err) {
    if (err === 'cancel') { fetchList(); return }
    const bizErr = err as BizError
    if (bizErr.code === FSM_STATE_DICT_ERR.VERSION_CONFLICT) {
      fetchList()
    }
  }
}
```

**Delete 逻辑**：
```typescript
async function handleDelete(row: FsmStateDictListItem) {
  if (row.enabled) {
    guardRef.value?.open({ action: 'delete', entityType: 'fsm-state-dict',
      entity: { id: row.id, name: row.name, label: row.display_name } })
    return
  }
  await ElMessageBox.confirm(`确定删除「${row.display_name}」？`, '删除确认', { type: 'warning' })
  try {
    await fsmStateDictApi.delete(row.id)
    fetchList()
  } catch (err) {
    const bizErr = err as BizError
    if (bizErr.code === FSM_STATE_DICT_ERR.IN_USE) {
      refDeleteResult.value = bizErr.data as FsmStateDictDeleteResult
      refDeleteVisible.value = true
    }
    // 其他错误由拦截器 toast
  }
}
```

**43020 引用弹窗**（内联在 template 中）：
- `el-dialog` v-model="refDeleteVisible"
- 标题：`无法删除「{{refDeleteResult?.display_name}}」`
- 提示：「以下 FSM 配置引用了此状态，请先修改再删除」
- `el-table` 展示 `refDeleteResult?.referenced_by`，3 列：name / display_name / enabled（`el-tag` 启用绿/停用灰）
- 底部：「知道了」关闭

**组件引用**：`<EnabledGuardDialog ref="guardRef" @refresh="fetchList" />`

**做完是什么样**：列表页完整可用；31 条种子数据分页显示；搜索/过滤/toggle/删除全部场景可操作；`npx vue-tsc --noEmit` 通过。

---

## T4：FsmStateDictForm.vue — 创建/编辑/查看表单 (R4, R5, R6, R7, R12)

**涉及文件**：
- `frontend/src/views/FsmStateDictForm.vue`（新增）

**做什么**：

克隆 `EventTypeForm.vue` 结构，4 个字段：

| 字段 | el 组件 | 约束 |
|------|---------|------|
| name | el-input | pattern `^[a-z][a-z0-9_]*$`；isCreate 时可编辑，其余只读；失焦 checkName |
| display_name | el-input | 必填 |
| category | el-input + datalist | 必填；onMounted 加载 categories 填充 datalist |
| description | el-input type=textarea | 可选，autosize |

**name 唯一性检查**（nameStatus 状态机，同 EventTypeForm）：
- `nameStatus: '' | 'checking' | 'available' | 'taken'`
- 失焦 → `checkName(name)` → 更新 nameStatus
- `el-form-item` error 显示「标识已存在」当 nameStatus === 'taken'

**提交处理**：
```typescript
// Create
await fsmStateDictApi.create({ name, display_name, category, description })
router.push('/fsm-state-dicts')

// Update
await fsmStateDictApi.update({ id, display_name, category, description, version })
router.push('/fsm-state-dicts')
```

**错误处理**：
```typescript
catch (err) {
  const bizErr = err as BizError
  if (bizErr.code === FSM_STATE_DICT_ERR.NAME_EXISTS) {
    nameStatus.value = 'taken'
  } else if (bizErr.code === FSM_STATE_DICT_ERR.VERSION_CONFLICT) {
    await ElMessageBox.alert('数据已更新，请刷新后重试', '版本冲突', { type: 'warning' })
    // 不跳转
  }
  // 其他错误拦截器 toast
}
```

**查看态**：`el-form :disabled="isView"`；「返回」按钮跳回 `/fsm-state-dicts`。

**onMounted**（编辑/查看时）：
```typescript
const res = await fsmStateDictApi.detail(id)
form.name = res.data.name
form.display_name = res.data.display_name
form.category = res.data.category
form.description = res.data.description
version.value = res.data.version
// 加载 categories 供 datalist
categories.value = await fsmStateDictApi.listCategories().then(r => r.data ?? [])
```

**做完是什么样**：新建/编辑/查看三种路由均正常渲染；name 校验 + 唯一性检查工作正常；版本冲突提示弹窗；`npx vue-tsc --noEmit` 通过；`npm run build` 构建通过。

---

## 执行顺序

T1 → T2 → T3 → T4

- T1 必须最先（T2 的 EnabledGuardDialog 和 T3/T4 的 views 都依赖它）
- T2 的路由声明了 FsmStateDictList/Form 的懒加载路径，但这两个文件在 T3/T4 才创建；T2 执行时先建空壳文件避免 vue-tsc 报错
- T3、T4 可在 T2 完成后并行开发，但建议顺序执行
