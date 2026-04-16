# 前端统一约定

本文档描述 ADMIN 项目前端（Vue 3 + TypeScript + Element Plus）的**项目级约定**，面向所有新增模块开发和已有模块改造。约定来自对现有页面的横向对齐，以 `FieldList.vue` / `FieldForm.vue` 为基准参考，`EventTypeList.vue`、`FsmStateDictList.vue` 等为补充参考。

通用语言陷阱与 Element Plus 注意事项见 `../development/standards/dev-rules/frontend.md`，禁止红线见 `../development/standards/red-lines/frontend.md`。

---

## 一、布局骨架

### 列表页

所有列表页由四个区域从上到下堆叠：标题栏、筛选栏、表格区、分页区。整体容器撑满父级高度，overflow 由各区负责，不出现外层滚动条。CSS 类全部来自 `styles/list-layout.css`，不在 scoped style 中重复定义这些类。

- **标题栏**：左侧页面标题，右侧"新建 XXX"按钮。标题简洁，与侧边栏菜单文字一致。
- **筛选栏**：所有筛选控件水平排列，文字搜索框用宽比例（`filter-item-wide`），下拉筛选框用标准比例（`filter-item`），末尾固定放"搜索"和"重置"两个按钮。
- **表格区**：`el-table` 撑满剩余高度，列定义见第二节。
- **分页区**：左侧"共 N 条"文字，右侧 `el-pagination`，布局为 `prev, pager, next`，每页固定 20 条，不提供每页条数选择器。

### 表单页

所有表单页（新建、编辑、查看共用同一组件）由三个区域组成：顶部导航栏、滚动内容区、底部操作栏。CSS 类全部来自 `styles/form-layout.css`。

- **顶部导航栏**：左箭头图标 + "返回"文字（点击跳转到对应列表页，用具体路径而非 `router.back()`）+ 斜杠分隔符 + 当前操作名（"新建 XXX" / "编辑 XXX" / "查看 XXX"）。
- **滚动内容区**：灰色背景，内容居中限宽（标准 800px，复杂页面用 1200px），内容卡片为白色带圆角边框。卡片内若有多个分区，每个分区用带彩色竖条的标题（`card-title`）区隔。
- **底部操作栏**：右对齐，"取消"在左、"保存"在右。查看模式（`isView`）时整个底部栏隐藏，不占位。底部栏必须在滚动区外，保证内容再长按钮始终可见。

---

## 二、列表页列定义

### 列名统一规则

所有模块的列使用同一套命名约定，以字段管理（`FieldList.vue`）为基准：

| 列含义 | 统一列名 |
|---|---|
| 自增主键 | **ID** |
| 技术标识符（`name` / `type_name` 等） | **XX标识**（如"字段标识"、"状态机标识"、"节点标识"） |
| 中文显示名（`label` / `display_name` 等） | **中文标签** |
| 枚举分类 | **分类** |
| 枚举类型 | **类型** |
| 启用开关 | **启用** |
| 创建时间 | **创建时间** |
| 行内操作 | **操作** |

"XX标识"的前缀取模块实体名，不能简写为裸"标识"。"中文标签"是全项目统一叫法，不用"中文名称"、"中文名"或其他变体。

### 列顺序

固定顺序为：ID → 标识列 → 中文标签列 → 业务特有列（类型/分类/数值等） → 启用列 → 创建时间列 → 操作列。每个模块都必须有"创建时间"列（width 170，用 `formatTime` 格式化），没有可以豁免的理由。"操作"列固定 `fixed="right"`，width 160。

### 枚举列

分类、类型等枚举值不直接输出原始字符串，统一用 `el-tag`（size="small"）展示，并通过辅助函数映射颜色语义。参考 `FieldList.vue` 的 `typeBadgeType` 和 `BtNodeTypeList.vue` 的 `categoryTagType`。

### 禁用行视觉

所有含启用开关的表格，禁用行除"启用"、"创建时间"、"操作"三列外整体降低透明度（opacity 0.5）。实现方式统一用 `:row-class-name="rowClassName"` + scoped `:deep(.row-disabled td:not(:nth-last-child(-n+3)))` CSS 选择器。计算 `nth-last-child` 的 `-n+3` 根据操作区右侧列数调整，保证倒数三列（启用、创建时间、操作）不变暗。

### 操作列按钮

三个操作按钮固定用 `el-link`（不用 `el-button`），顺序始终是"查看 → 编辑 → 删除"，"查看"和"编辑"用 `type="primary"`，"删除"用 `type="danger"`，按钮间距统一 `style="margin-left: 12px"`。`:underline="false"` 是必须属性。内置或只读实体（如 BtNodeType 的内置节点）可有条件隐藏"编辑"和"删除"，但"查看"始终显示。

---

## 三、筛选栏约定

### 文字搜索框

搜索中文标签的输入框，placeholder 统一为"搜索中文标签"，绑定字段视 API 参数名而定（`label`、`display_name` 等），但界面文字保持一致。如果同时支持搜索技术标识（仅在必要时），placeholder 为"搜索 XXX 标识"。两个搜索框并存时文字框用 `filter-item-wide`，标识框用普通 `filter-item`。

### 下拉筛选

所有下拉选项从字典接口动态加载（`dictApi.list('xxx')`），不硬编码，但节点分类、感知模式等固定枚举值集合可例外硬编码（集合已在后端固定，不随配置变化）。启用状态下拉全项目统一："已启用" / "已禁用"，value 分别为 `true` / `false`。

### 搜索和重置行为

点击"搜索"必须先将 `query.page` 重置为 1 再请求，避免在第 N 页筛选后找不到结果。"重置"将所有筛选字段清空并重置页码，然后重新请求。

---

## 四、交互模式

### 启用/禁用

点击开关后弹 `ElMessageBox.confirm`，启用时 type 为 `success`，禁用时为 `warning`。确认后先调 `detail()` 获取最新 version，再调 `toggleEnabled()`。收到版本冲突错误时弹 `ElMessageBox.alert`（不是 `ElMessage`）提示刷新，并刷新列表。取消弹窗时直接 return，不做任何操作。参考 `EventTypeList.vue` 和 `FsmStateDictList.vue` 的 `handleToggle`。

### 编辑跳转

已启用实体不允许直接进入编辑页，统一通过 `EnabledGuardDialog` 弹窗拦截。该弹窗引导用户先禁用再编辑。组件挂在列表页模板末尾，通过 `ref` 调用 `open()`，接收 action / entityType / entity 三个参数。

### 删除

删除存在两种模式，选择依据是后端是否提供独立的引用查询接口：

**模式 A（有独立引用查询，如字段管理）**：先查引用，有引用则直接展示引用弹窗阻止删除，无引用再弹确认弹窗。后端删除接口失败时作为兜底再次拉引用展示。

**模式 B（无独立引用查询，如状态字典）**：直接调删除接口，成功则提示，失败且错误码为 IN_USE 时展示引用弹窗。禁用已作为一层保护，不需要前置确认弹窗。

两种模式均满足同一个 UX 原则：有引用时直接告诉用户"无法删除，原因是 XXX"，不让用户先经历一次无效确认。

**删除确认文案**统一格式为：`确认删除XXX「{label}」（{name}）？删除后无法恢复。`，confirmButtonText 为"确认删除"，type 为 `warning`。

### 版本冲突

Toggle 操作的版本冲突用 `ElMessageBox.alert`（提示刷新）；Delete 操作的版本冲突用 `ElMessage.warning`（提示重新操作）；Update 操作的版本冲突用 `ElMessageBox.alert`（提示返回列表刷新）。三种场景提示力度不同，不要混用。

### 空状态

`el-table` 的 `#empty` slot 统一放 `el-empty`，description 为"暂无 XXX 数据"，内嵌一个与顶部相同的"新建 XXX"按钮。

---

## 五、表单页约定

### 三模式共用

新建、编辑、查看三个路由共用同一个 Form 组件，通过 `route.meta.isCreate` 和 `route.meta.isView` 区分，不拆三个组件。组件标题根据模式动态显示。

### 标识字段

技术标识符（`name`、`type_name` 等）新建时可编辑，离焦后异步校验唯一性（showing "校验中" → "标识符可用" / "已被使用"）；编辑和查看时禁用输入框并显示 Lock 图标，下方提示"创建后不可修改"。校验逻辑参考 `FieldForm.vue` 的 `checkNameUnique`。

### 表单规格

`el-form` 统一 `label-width="120px"`、`label-position="right"`。整体在查看模式时设置 `:disabled="isView"` 而不是逐个字段控制。

### 不展示时间戳

表单页（查看模式）不展示创建时间、更新时间。时间戳只在列表页 `created_at` 列可见。

### 提交反馈

创建成功的提示应包含有效后续指引（如"创建成功，默认为禁用状态，确认无误后请手动启用"），不只是"创建成功"。更新成功统一提示"保存成功"，并跳回列表页。

---

## 六、API 层约定

每个资源对应一个 `src/api/xxx.ts` 文件，内部结构固定为：类型定义 → 业务错误码常量 → api 对象。详见任意已有模块，如 `api/fields.ts`。

错误处理约定：全局 axios 拦截器在 code !== 0 时统一 toast，组件 catch 块只处理需要定制行为的错误码，其余 catch 块加注释 `// 拦截器已 toast`，不重复弹错误。

---

## 七、CSS 模块化规则

`styles/list-layout.css` 和 `styles/form-layout.css` 由 `main.ts` 全局引入，所有列表页和表单页直接使用其中的 class，不在各组件 scoped style 中重复定义。scoped style 只写当前组件私有内容（如模块特有的弹窗样式、行内提示色）。

不得在 scoped style 中覆盖 `.page-header`、`.filter-bar`、`.filter-item`、`.table-wrap`、`.pagination-wrap`、`.form-header`、`.form-scroll`、`.form-body`、`.form-card`、`.form-footer` 等全局布局类，否则会影响该类在页面中的行为并制造样式孤岛。

---

## 八、参考文件索引

| 场景 | 参考文件 |
|---|---|
| 列表页完整实现（含引用弹窗） | `views/FieldList.vue` |
| 列表页标准实现（无引用查询） | `views/EventTypeList.vue`、`views/BtTreeList.vue` |
| 删除模式 B（后端直接返回引用） | `views/FsmStateDictList.vue` |
| 含内置/只读行的操作列 | `views/BtNodeTypeList.vue` |
| 表单页完整实现（含约束子组件） | `views/FieldForm.vue` |
| 表单页标准实现 | `views/EventTypeForm.vue`、`views/FsmStateDictForm.vue` |
| 全局布局 CSS | `styles/list-layout.css`、`styles/form-layout.css` |
| 时间格式化 | `utils/format.ts` |
| 启用守卫组件 | `components/EnabledGuardDialog.vue` |
