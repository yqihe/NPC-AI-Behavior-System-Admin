# fsm-state-dict-polish — 任务列表

## 状态

- [ ] T1: 后端 — 新增字典常量 + seed + 删除 ListCategories 接口
- [ ] T2: 前端 — 列表页对齐企业标准（ID列/列头/搜索字段/分类下拉）
- [ ] T3: 前端 — 表单页分类改 el-select + 按钮内嵌

---

## T1：后端 — 新增字典常量 + seed + 删除 ListCategories 接口 (R1, R3)

**涉及文件**：
- `backend/internal/util/const.go`（修改）
- `backend/cmd/seed/main.go`（修改）
- `backend/internal/router/router.go`（修改）
- `backend/internal/handler/fsm_state_dict.go`（修改）
- `backend/internal/service/fsm_state_dict.go`（修改）

**做什么**：

1. `util/const.go` 的「字典组名」分节追加：
   ```go
   DictGroupFsmStateCategory = "fsm_state_category"
   ```

2. `seed/main.go` 追加 `fsm_state_category` 字典数据（在 `all` 合并前追加）：
   ```go
   fsmStateCategories := []model.Dictionary{
       {GroupName: util.DictGroupFsmStateCategory, Name: "general",  Label: "通用", SortOrder: 1},
       {GroupName: util.DictGroupFsmStateCategory, Name: "combat",   Label: "战斗", SortOrder: 2},
       {GroupName: util.DictGroupFsmStateCategory, Name: "movement", Label: "移动", SortOrder: 3},
       {GroupName: util.DictGroupFsmStateCategory, Name: "social",   Label: "社交", SortOrder: 4},
       {GroupName: util.DictGroupFsmStateCategory, Name: "activity", Label: "活动", SortOrder: 5},
   }
   all = append(all, fsmStateCategories...)
   ```

3. `router/router.go` 删除：
   ```go
   fsmStateDicts.POST("/list-categories", handler.WrapCtx(h.FsmStateDict.ListCategories))
   ```

4. `handler/fsm_state_dict.go` 删除 `ListCategories` 方法（含注释）

5. `service/fsm_state_dict.go` 删除 `ListCategories` 方法（含注释）

> store 层的 `ListCategories` SQL 方法保留。

**做完是什么样**：
- `go build ./...` 通过（删除方法后无编译错误）
- 重跑 seed 后 `curl POST /api/v1/dictionaries {"group":"fsm_state_category"}` 返回 5 条
- `curl POST /api/v1/fsm-state-dicts/list-categories` 返回 404

---

## T2：前端 — 列表页对齐企业标准 (R2, R4, R5)

**涉及文件**：
- `frontend/src/api/fsmStateDicts.ts`（修改）
- `frontend/src/views/FsmStateDictList.vue`（修改）

**做什么**：

### fsmStateDicts.ts
1. 删除 `listCategories()` 方法
2. `FsmStateDictListQuery` 的 `name?: string` 改为 `display_name?: string`

### FsmStateDictList.vue
1. 加 ID 列（第一列，`prop="id"` `width="70"`）
2. 列头调整：
   - `display_name` 列头改为「中文标签」
   - `category` 列头改为「状态分类」
3. 搜索字段调整：
   - `query.name` 全部改为 `query.display_name`
   - 搜索框 placeholder 改为「搜索中文标签」
4. 分类过滤下拉：
   - 删除 `<datalist>` 实现
   - 改为 `el-select` + `dictApi.list('fsm_state_category')` 加载选项
   - `el-option :key="item.name" :label="item.label" :value="item.name"`
   - `onMounted` 时加载分类选项（替换原 `fsmStateDictApi.listCategories()` 调用）
5. 删除原 `loadCategories` 函数，改为 `loadCategoryOptions` 调用 `dictApi`

**做完是什么样**：
- 列表页第一列为 ID
- 列头依次：ID / 状态标识 / 中文标签 / 状态分类 / 启用 / 创建时间 / 操作
- 搜索框输入「空闲」可过滤出对应记录
- 分类下拉展示「通用/战斗/移动/社交/活动」
- `npx vue-tsc --noEmit` 通过

---

## T3：前端 — 表单页分类改 el-select + 按钮内嵌 (R2, R6, R7, R8)

**涉及文件**：
- `frontend/src/views/FsmStateDictForm.vue`（修改）

**做什么**：

1. 引入 `dictApi` 和 `DictionaryItem`：
   ```typescript
   import { dictApi } from '@/api/dictionaries'
   import type { DictionaryItem } from '@/api/dictionaries'
   ```

2. 将 `categories: ref<string[]>([])` 改为 `categoryOptions: ref<DictionaryItem[]>([])`

3. `loadCategories` 函数改为：
   ```typescript
   async function loadCategories() {
     try {
       const res = await dictApi.list('fsm_state_category')
       categoryOptions.value = res.data?.items ?? []
     } catch {
       // 非关键，静默失败
     }
   }
   ```

4. 模板中分类字段从 `el-input + datalist` 改为：
   ```html
   <el-select
     v-model="form.category"
     placeholder="请选择状态分类"
     style="width: 100%"
   >
     <el-option
       v-for="item in categoryOptions"
       :key="item.name"
       :label="item.label"
       :value="item.name"
     />
   </el-select>
   ```

5. 表单标签「分类」改为「状态分类」

6. 保存/取消按钮：从独立 `form-card` 移入 `el-form` 内部底部，与 FieldForm 一致：
   - 删除 `<div v-if="!isView" class="form-card form-actions">...</div>`（独立 card）
   - 在 `</el-form>` 前追加（仍在 form-card 内）：
     ```html
     <div v-if="!isView" class="form-actions">
       <el-button @click="$router.push('/fsm-state-dicts')">取消</el-button>
       <el-button type="primary" :loading="submitting" @click="handleSubmit">保存</el-button>
     </div>
     ```

7. 删除 `fsmStateDictApi.listCategories` 的 import 引用（fsmStateDicts.ts 已无此方法）

**做完是什么样**：
- 新建/编辑时分类为 el-select，展示「通用/战斗/移动/社交/活动」，不可手动输入
- 保存/取消按钮在表单卡片内底部
- `npx vue-tsc --noEmit` 0 errors
- `npm run build` 成功

---

## 执行顺序

T1 → T2 → T3

- T1 先行（后端删路由/前端删 listCategories 调用必须同步，否则 TS 类型对不上）
- T2、T3 均只改前端，可在 T1 完成后顺序执行
