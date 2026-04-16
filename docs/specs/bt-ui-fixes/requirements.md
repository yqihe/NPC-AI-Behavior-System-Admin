# Requirements — bt-ui-fixes

行为树管理页面的多项修复与 UI 改进。

---

## 动机

行为树是 V3 最后一块完整落地的核心模块，目前存在以下问题：

1. **Docker 启动竞态**：首次打开 `localhost:3000` 有时拿不到数据，因为 nginx 容器不等后端就绪。
2. **搜索栏冗余字段**：行为树标识是内部英文 key，策划/运营无需按它搜索，留着只增加界面噪音。
3. **新建/编辑表单过宽**：树编辑器是纵向嵌套结构，用 `form-body-wide`（max-width 1200px）导致两侧有大量空白，视觉臃肿。
4. **节点类型选择器体验差**：当前是 Dialog + Radio 列表，类型多时难以扫描，"确认"按钮多一步操作。
5. **节点参数字段不可交互**：BtNodeEditor 里 BB Key / select / 数值等控件在编辑模式下无法选择或填写，行为树结构无法正常编辑，是功能性缺陷。

不做以上修复，行为树模块对策划/运营基本不可用。

---

## 优先级

**高**。行为树是毕业答辩演示的核心路径，P5 以上问题直接影响可演示性。

---

## 预期效果（场景描述）

**场景 A（Docker 启动）**：`docker compose up` 后打开浏览器，前端容器等待后端健康检查通过后再启动，首页直接有数据，无需 F5。

**场景 B（搜索栏）**：`BtTreeList` 只有"搜索中文标签"一个输入框，输入"巡逻"可过滤出对应行。

**场景 C（表单宽度）**：新建行为树表单基本信息 + 树结构区域宽度约 800px，内容居中，与其他模块（字段管理、事件类型等）视觉一致。

**场景 D（节点类型选择器）**：点击"添加根节点"/"添加子节点"，弹出面板（或改为 Popover/内联）按 category 分组显示卡片，单击即选中，无需二次确认。

**场景 E（节点参数可交互）**：为 Condition leaf 节点添加后，"展开参数"区域显示 BB Key 下拉可搜索可选可输入、operator select 可选、value 数值框可填写，保存后数据正确序列化到后端。

---

## 依赖分析

- 依赖已完成：BtTreeList、BtTreeForm、BtNodeEditor、BtNodeTypeSelector、BBKeySelector 均已实现。
- 依赖已完成：docker-compose.yml、nginx.conf 均已就位。
- 无下游依赖：这些都是纯前端/部署层修复，不影响后端 API。

---

## 改动范围

| # | 改动内容 | 涉及文件 | 文件数 |
|---|---------|---------|--------|
| F1 | Docker 健康检查 | `docker-compose.yml` | 1 |
| F2 | 搜索栏精简（前端 + 后端 query struct） | `BtTreeList.vue`, `api/btTrees.ts`, `backend/.../bt_tree.go` (handler/service/store) | ~5 |
| F3 | 表单宽度 | `BtTreeForm.vue` | 1 |
| F4 | 节点类型选择器重做 | `BtNodeTypeSelector.vue`, `BtNodeEditor.vue` | 2 |
| F5 | 节点参数可交互修复 | `BtNodeEditor.vue`, `BBKeySelector.vue` | 2 |

估计文件数：8-10 个。

---

## 扩展轴检查

- **新增配置类型**：不涉及。
- **新增表单字段**：F4/F5 改善 BtNodeEditor 的参数渲染，间接有利于后续新增参数类型（框架更稳健）。
- 其余修复与扩展轴无直接关联，属于稳定性投资。

---

## 验收标准

**R1**（Docker）：`docker compose down && docker compose up` 后等待约 30s，在浏览器打开 `localhost:3000` 并导航到行为树管理，列表有数据，无需手动刷新。

**R2**（搜索栏）：BtTreeList 页面只有一个搜索输入框（中文标签），无行为树标识搜索框，后端 list handler 不再接受 `name` 参数（或忽略它）。

**R3**（表单宽度）：`BtTreeForm` 新建/编辑页面，基本信息表单内容区 `max-width` 不超过 880px，与 `FieldForm`、`EventTypeForm` 宽度视觉一致。

**R4**（节点类型选择器）：点击"添加根节点"/"添加子节点"/"设置子节点"/"编辑"，展示改进后的选择 UI，按 category 分组，可快速点选，交互步骤 ≤ 2 步（点击按钮 → 点击节点类型）。

**R5**（节点参数交互）：在编辑模式下，leaf 节点展开参数后，bb_key 类型控件可选可输入，select 类型控件可下拉选择，float/integer 类型控件可输入数值，string 类型控件可输入文本，修改后父组件 `rootNode` 数据即时更新。

**R6**（回归）：`npx vue-tsc --noEmit` 无新增类型错误。

---

## 不做什么

- 不修改后端 bt-trees list 的分页、缓存、排序逻辑，只移除 `name` 过滤字段。
- 不新增节点参数类型（如 `enum`、`json_object`），只修复现有类型渲染。
- 不做行为树可视化拖拽编辑（毕设后延后）。
- 不做节点类型的搜索过滤（节点数量有限，无需）。
- 不改 `BBKeySelector` 的数据来源逻辑，只修复交互 bug。
