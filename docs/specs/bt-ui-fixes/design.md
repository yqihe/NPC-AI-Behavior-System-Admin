# Design — bt-ui-fixes

## 前置阅读已完成
- `docs/architecture/frontend-conventions.md` ✓
- `docs/development/standards/red-lines/frontend.md` ✓
- `docs/development/standards/red-lines/general.md` ✓
- `docs/development/admin/red-lines.md` ✓
- `docs/development/standards/dev-rules/frontend.md` ✓

---

## 方案描述

### F1 — Docker 健康检查

**问题**：`docker-compose.yml` 里 `admin-frontend` 的 `depends_on` 只等 `admin-backend` 容器启动，不等它能处理请求。Nginx 启动时后端可能还在连 MySQL/Redis，导致首次打开页面 API 请求失败。

**方案**：
1. 给 `admin-backend` 新增 `healthcheck`，用 `wget` 探测 `/health` 接口（该接口已注册，返回 `{"status":"ok"}`）。
2. `admin-frontend` 的 `depends_on` 加 `condition: service_healthy`。

```yaml
# docker-compose.yml 改动
admin-backend:
  healthcheck:
    test: ["CMD", "wget", "-qO-", "http://localhost:9821/health"]
    interval: 5s
    timeout: 3s
    retries: 12
    start_period: 15s

admin-frontend:
  depends_on:
    admin-backend:
      condition: service_healthy
```

`start_period: 15s` 给后端足够的冷启动时间（连接 MySQL + 加载字典 + 预热缓存），期间健康检查失败不计入 retries。

**备选方案**：在 nginx `entrypoint` 加 shell 循环等待后端响应 → 与 Docker Compose 健康检查机制重复，维护成本高，排除。

---

### F2 — 搜索栏精简

**问题**：`BtTreeList.vue` 有"搜索行为树标识"和"搜索中文标签"两个输入框，标识是技术 key，策划/运营用不到。

**方案**：前后端同步删除 `name` 过滤字段。

**前端**：
- `BtTreeList.vue`：删除 `name` 输入框，删除 `query.name` 字段，删除 `handleReset` 里的 `query.name = ''`。
- `api/btTrees.ts`：`BtTreeListQuery` 删除 `name?: string`。

**后端**（3 处联动）：
- `model/bt_tree.go`：`BtTreeListQuery.Name` 字段删除。
- `store/mysql/bt_tree.go`：`List()` 删除 `if q.Name != ""` 分支。
- `handler/bt_tree.go`：`List()` 的 slog.Debug 删除 `name` 字段。

Redis 缓存 key 依赖 `BtTreeListQuery` 序列化。删掉 `Name` 字段后旧缓存 key 不再被命中，会自然过期（无需主动清除）。

**备选方案**：仅前端隐藏，后端保留字段 → 留死代码，不干净，排除。

---

### F3 — 表单宽度修正

**问题**：`BtTreeForm.vue` 用了 `form-body-wide`（max-width 1200px），树编辑器是纵向嵌套结构，宽度浪费。

**方案**：`BtTreeForm.vue` 里把 `form-body-wide` 改为 `form-body`（max-width 800px），与字段管理、事件类型等模块一致。

树结构嵌套深时，`form-body` 内 `.bt-node-card` 用 `marginLeft: depth * 24px` 缩进，800px 宽度足够显示 10+ 层缩进（10×24=240px，剩余 560px 给 header/param 内容）。

**备选方案**：自定义一个 950px 宽度 → 破坏统一性，排除。

---

### F3b — BtNodeTypeForm 格式对齐（"统一格式"要求）

检查发现 `BtNodeTypeForm.vue` 有两处偏离全局约定：

1. **卡片类名**：用了 `.card` 而非 `.form-card`（`form-layout.css` 全局类），`.card` 未在全局样式定义，等于 scoped 内重复定义或由别处污染。
2. **返回导航**：`@click="router.back()"` 违反约定（"用具体路径而非 `router.back()`"），应改为 `router.push('/bt-node-types')`。
3. **el-form 无 `:disabled="isView"`**：各字段单独写 `:disabled="isView || isBuiltinLocked"`，违反"整体 `isView` 时设置 `el-form :disabled`"约定。正确写法：`el-form :disabled="isView || isBuiltinLocked"` 统一控制，单个字段只追加 builtin 特有条件。

同类问题在 `BtTreeForm.vue` 也存在（`router.back()`），一并修复。

---

### F4 — 节点类型选择器重做

**问题**：当前是 Dialog + el-radio-group + "确认"按钮，两步操作（先选中 radio，再点确认）；类型列表多时扫描困难；radio 排版密集。

**方案**：保留 Dialog 外框，内部改为**卡片列表单击即选**：

- 每个节点类型渲染为一个可点击 Card：左侧 category 色块标签（复用 `categoryTagType`），主区域显示"中文标签"+ 小字 `(type_name)`，右侧无额外按钮。
- 单击卡片立即 emit `select` 并关闭 Dialog，无需"确认"按钮。
- 按 category 分组，组标题用与现在相同的 `category-title` 样式。
- 不需要搜索（节点类型数量有限，当前设计已有中文标签，视觉扫描够用）。

视觉规格：卡片 `border: 1px solid #e4e7ed`，hover `background: #f5f7fa`，`border-color: #409EFF`；选中态不需要，因为单击直接确认。Dialog `width="560px"` 以容纳双列布局（用 CSS grid `grid-template-columns: 1fr 1fr`，gap 8px）。

**备选方案**：改为 Popover 内联显示 → Popover 内容受到 `overflow` 父容器裁切，在 `.form-scroll` 内可能被截断；Dialog 更稳定。排除。

**组件改动**：只改 `BtNodeTypeSelector.vue` 内部，对外接口（`v-model: boolean`，`nodeTypes: BtNodeTypeMeta[]`，`@select`）不变，调用方 `BtNodeEditor.vue` 无需改动。

---

### F5 — 节点参数字段可交互修复

**根因分析**（静态代码 + 架构推导）：

**Bug A（已确认）—— `BBKeySelector` 使用 `@change` 而非 `@update:model-value`**

`BBKeySelector.vue` 内部 `el-select` 绑定了 `@change="handleChange"`。Element Plus `el-select` 的 `change` 事件仅在 **选定值真正改变后** 才触发（含 `allow-create` 的 Enter 确认）；而 `update:modelValue` 事件与 Vue v-model 契约对齐，在值变更时立即触发。两者在正常选择场景下一致，但 `@change` 在以下情形会静默失败：用户打开下拉并在过滤框里输入文字后点击空白处离开（没按 Enter），`change` 不触发，输入内容丢失。

另外，`BBKeySelector` 使用 `allow-create`，但红线 `frontend.md` § "禁止放行无效输入" 明确要求 **BB Key 必须用 `el-select`，不允许手动输入**。`allow-create` 允许策划手写任意字符串，违反红线。

**修复 A**：
1. `BBKeySelector.vue`：移除 `allow-create`（不再允许自由输入），`@change` 改为 `@update:model-value`，同步更新 `handleChange` 为 `handleSelect`。
2. 当选项列表为空时（API 加载失败），显示占位提示"暂无可用 BB Key，请先在字段管理中添加字段并暴露 BB"而非空白下拉。

**Bug B（已确认）—— `select` 类型参数初始值为 `undefined`，`el-select` 不显示 placeholder**

`BtNodeEditor.vue` 里：
```html
:model-value="modelValue.params[paramDef.name]"
```
新建节点时 `params = {}`，`params[name] = undefined`，`el-select` 接收 `undefined` 作为 model-value 时不显示 placeholder，且在 Element Plus 某些版本中点击无响应。

**修复 B**：
```html
:model-value="modelValue.params[paramDef.name] ?? null"
```
`null` 是 Element Plus el-select 的合法"无选中"状态，会正确显示 placeholder 并允许交互。

**Bug C（已确认）—— `el-input-number` `undefined` 初始值警告**

`BtNodeEditor.vue` 里：
```html
:model-value="(modelValue.params[paramDef.name] as number) ?? undefined"
```
`el-input-number` 接收 `undefined` 时内部报 Vue warn，且在某些版本的 Element Plus 中输入后光标异常跳动。改为 `?? null`。

**修复 C**：`float`/`integer` 的 `el-input-number` 用 `?? null` 代替 `?? undefined`。

**Bug D（已确认）—— BtNodeTypeForm 内各字段各自传 `:disabled="isView || isBuiltinLocked"`，违反红线 §12**

当 `isBuiltinLocked = false`（非内置节点的编辑模式）时：
- `el-form` 没有 `:disabled`
- 各字段显式传 `:disabled="isView || false"` = `:disabled="isView"`

这在非内置节点编辑时无问题，但在内置节点查看时（`isView = true`，`isBuiltinLocked = true`），子组件的 `disabled` 从 form-level 取不到任何注入，只靠各自字段传递。如果有新增字段遗漏这个 `:disabled`，该字段会变成可编辑。修复见 F3b。

---

## 红线检查

| 红线 | 检查结果 |
|------|---------|
| 前端禁止放行无效输入（BB Key 必须 el-select，不允许手动输入） | **违反**：BBKeySelector 有 `allow-create`。F5 修复中移除 |
| 前端禁止 el-form disabled 被子组件覆盖 | **违反**：BtNodeTypeForm 未使用 el-form 级 disabled。F3b 修复 |
| 前端 scoped 不覆盖全局布局类 | BtNodeTypeForm 用 `.card` 而非 `.form-card`，需对齐 |
| 前端 vue-tsc 必跑 | 修复后在 Phase 3 每任务完成后验证 |
| 通用 — 禁止静默降级 | BBKeySelector 空选项时改为显式提示，不再静默无响应 |
| go 红线 | F2 后端改动仅删字段，无新增逻辑，不涉及 |
| ADMIN 禁止硬编码 | 无新增硬编码 |
| ADMIN 禁止暴露技术细节给策划 | F4 卡片继续显示"中文标签 (type_name)"格式，符合 §6.5 |

---

## 扩展性影响

- **F4**（选择器重做）对 BtNodeEditor 外部接口零影响，后续新增节点类型自动出现在卡片列表，正面影响。
- **F5**（参数修复）让 BtNodeEditor 的参数编辑框架更健壮，后续若新增 `json_object` 等参数类型，框架更稳固，正面影响。
- 其余修复（F1、F2、F3）均为局部改动，不影响扩展轴。

---

## 依赖方向

```
docker-compose.yml        (F1, 独立)
frontend/views/BtTreeList.vue  → api/btTrees.ts  (F2 前端)
backend model → store → service → handler         (F2 后端，单向向下)
frontend/views/BtTreeForm.vue                     (F3, 独立)
frontend/views/BtNodeTypeForm.vue                 (F3b, 独立)
frontend/components/BtNodeTypeSelector.vue        (F4, 对外接口不变)
frontend/components/BBKeySelector.vue             (F5-A, 对外接口不变)
frontend/components/BtNodeEditor.vue              (F5-B/C, 内部修复)
```

所有依赖均单向向下，无循环依赖。

---

## 陷阱检查（frontend dev-rules）

| 陷阱 | 相关性 | 应对 |
|------|--------|------|
| `el-select` v-model 类型必须与选项 value 一致 | F5-B：`select` 类型 options 是 `string[]`，`null` 作初始值 OK，选中后是 string | 无问题 |
| `el-input-number` step 默认 1，小数需显式设置 | `float` 类型已有 `:precision="4" :step="0.1"` | 已有 |
| `el-form :disabled` 与子组件 `:disabled` 交互 | F3b 修复 BtNodeTypeForm | 修复中处理 |
| `el-dialog` 关闭再打开数据残留 | F4：BtNodeTypeSelector 新卡片方案，重新打开时 selectedTypeName 已在 `handleClose` 清空 | 无需改动 |
| vue-tsc 必跑 | 每任务完成后验证 | Phase 3 每 T 结束验证 |

---

## 配置变更

`docker-compose.yml` 新增 `healthcheck` 字段，不需要新的配置文件或环境变量。

---

## 测试策略

- **F1**：`docker compose down && docker compose up`，等待约 30s，观察浏览器首次打开 `/bt-trees` 是否有数据，无需刷新。
- **F2**：页面只剩一个搜索框；后端接收 `{}` 不含 `name` 字段的请求，列表正常返回。
- **F3**：视觉对比 BtTreeForm 与 FieldForm 宽度，应一致。
- **F3b**：BtNodeTypeForm 查看模式下所有字段不可编辑。
- **F4**：单击节点卡片后 Dialog 立即关闭，节点被添加到树中，无需确认按钮。
- **F5**：创建含 bb_key + select + float 参数的 leaf 节点；在编辑模式下：BBKeySelector 下拉可选/搜索；operator el-select 可打开并选择；value el-input-number 可输入数字；保存后重新打开，数值正确回填。
- **R6（回归）**：`npx vue-tsc --noEmit` 零错误。
