# Tasks — bt-ui-fixes

依赖顺序：F1 → F2（后端先于前端） → F3/F3b → F4 → F5（A先于B/C）

---

## T1 — Docker healthcheck (R1)

**涉及文件**：`docker-compose.yml`（1 个）

**做什么**：
- `admin-backend` 服务新增 `healthcheck`（wget 探测 `/health`，interval 5s，retries 12，start_period 15s）
- `admin-frontend` 的 `depends_on.admin-backend` 加 `condition: service_healthy`

**做完是什么样**：`docker compose config` 验证无报错；`docker compose up` 后 `admin-frontend` 容器等待 `admin-backend` healthy 后才启动。

---

## T2 — 后端删除 name 过滤字段 (R2)

**涉及文件**：
1. `backend/internal/model/bt_tree.go`
2. `backend/internal/store/mysql/bt_tree.go`
3. `backend/internal/handler/bt_tree.go`

**做什么**：
- `model/bt_tree.go`：`BtTreeListQuery` 删除 `Name string` 字段
- `store/mysql/bt_tree.go`：`List()` 删除 `if q.Name != ""` 分支（共 2 行）
- `handler/bt_tree.go`：`List()` 的 `slog.Debug` 删除 `"name", req.Name` 参数

**做完是什么样**：`go build ./...` 无报错；`BtTreeListQuery` 只有 `DisplayName`、`Enabled`、`Page`、`PageSize` 四个字段。

---

## T3 — 前端删除 name 搜索框 (R2)

**涉及文件**：
1. `frontend/src/views/BtTreeList.vue`
2. `frontend/src/api/btTrees.ts`

**做什么**：
- `btTrees.ts`：`BtTreeListQuery` 删除 `name?: string`
- `BtTreeList.vue`：
  - 删除"搜索行为树标识"的 `el-input`（template 中）
  - 删除 `query.name` 字段（`reactive` 初始化中）
  - `handleReset` 删除 `query.name = ''`
  - `fetchList` 中 `params.name` 赋值逻辑删除

**做完是什么样**：列表页只剩"搜索中文标签"一个文字输入框；`npx vue-tsc --noEmit` 无错。

---

## T4 — BtTreeForm 宽度 + 返回路径修正 (R3)

**涉及文件**：`frontend/src/views/BtTreeForm.vue`（1 个）

**做什么**：
- `form-body-wide` → `form-body`
- 顶部导航栏两处 `@click="router.back()"` 改为 `@click="router.push('/bt-trees')"`

**做完是什么样**：新建行为树页面内容区宽度与字段管理表单一致（视觉对比）；点击返回直接跳列表页。

---

## T5 — BtNodeTypeForm 格式对齐 (F3b)

**涉及文件**：`frontend/src/views/BtNodeTypeForm.vue`（1 个）

**做什么**：
1. 所有 `.card` 改为 `.form-card`（scoped style 中同步删除重复的 `.card` 定义，若有）
2. `el-form` 加 `:disabled="isView || isBuiltinLocked"`，各子字段的 `:disabled` 逻辑对应简化：
   - 纯 `isView` 控制的字段：删除单独的 `:disabled`（由 form 接管）
   - 仍需 `isBuiltinLocked` 单独控制的字段（如 category 在非创建模式也要锁）：保留 `:disabled="!isCreate || isBuiltinLocked"`（移除 isView 因 form 级已覆盖）
3. 返回导航 `router.back()` → `router.push('/bt-node-types')`

**做完是什么样**：查看模式下所有字段不可编辑；`npx vue-tsc --noEmit` 无错。

---

## T6 — BtNodeTypeSelector 重做为卡片式 (R4)

**涉及文件**：`frontend/src/components/BtNodeTypeSelector.vue`（1 个）

**做什么**：
- 保留 Dialog 外框、`v-model`、`@select` emit，对外接口不变
- 内部替换 `el-radio-group` 为双列卡片网格（`display: grid; grid-template-columns: 1fr 1fr; gap: 8px`）
- 每个节点类型渲染为可点击 div（`cursor: pointer`）：
  - 左：`el-tag`（复用 `categoryTagType` 颜色，size="small"）
  - 中：`{{ t.label }}` 加粗 + `({{ t.type_name }})` 灰色小字
  - hover：`background: #f5f7fa; border-color: #409EFF`
- 单击直接调用 `handleConfirm(t)`（emit select + close），**删除"确认"按钮**
- Dialog footer 只保留"取消"按钮
- 删除 `selectedTypeName` ref（不再需要中间选中态）

**做完是什么样**：点击"添加根节点"后，Dialog 弹出，节点类型以双列卡片展示，单击任一卡片 Dialog 立即关闭并将节点添加到树。

---

## T7 — BBKeySelector 修复：移除 allow-create，修正事件绑定 (R5)

**涉及文件**：`frontend/src/components/BBKeySelector.vue`（1 个）

**做什么**：
1. `el-select` 移除 `allow-create` 属性（红线：BB Key 是有限集合，禁止手动输入）
2. `@change="handleChange"` 改为 `@update:model-value="handleChange"`
3. 当 `npcOptions` 和 `schemaOptions` 均为空时，在 `el-select` 内部添加提示 option（禁用态，文字"暂无 BB Key，请先在字段管理中添加字段并开启暴露 BB"）；有选项时不显示该提示

**做完是什么样**：BBKeySelector 只能从列表选择；选择某个 BB Key 后 `@update:model-value` 正确触发更新；空选项时有友好提示。`npx vue-tsc --noEmit` 无错。

---

## T8 — BtNodeEditor 参数控件初始值修复 (R5)

**涉及文件**：`frontend/src/components/BtNodeEditor.vue`（1 个）

**做什么**：
- `select` 类型 `el-select`：`:model-value="modelValue.params[paramDef.name] ?? null"`
- `bool` 类型 `el-select`：`:model-value="modelValue.params[paramDef.name] ?? null"`
- `float` 类型 `el-input-number`：`:model-value="(modelValue.params[paramDef.name] as number | null) ?? null"`
- `integer` 类型 `el-input-number`：同上
- `string` 类型 `el-input`：已有 `|| ''` 兜底，无需改动
- `bb_key` 类型 BBKeySelector：已有 `|| ''` 兜底，无需改动
- `@update:model-value` 回调的参数类型注解补全（`(v: unknown) =>` 等，防 vue-tsc any 报错）

**做完是什么样**：新建 leaf 节点后，展开参数面板，`select`/`bool` 显示 placeholder 可点击下拉；`float`/`integer` 显示空数值框可输入；修改后 `rootNode` 数据同步更新；`npx vue-tsc --noEmit` 无错。

---

## 任务顺序

```
T1 (docker)
T2 (后端 name 删除)  →  T3 (前端 name 删除)
T4 (BtTreeForm 宽度+路径)
T5 (BtNodeTypeForm 格式)
T6 (选择器重做)
T7 (BBKeySelector 修复)  →  T8 (BtNodeEditor 参数修复)
```

T2 必须先于 T3（前端类型依赖后端 model，虽然不直接 import，但接口契约对齐要求先改后端再改前端）。T7 先于 T8（T8 验证时依赖 BBKeySelector 已修复）。其余任务可独立执行。
