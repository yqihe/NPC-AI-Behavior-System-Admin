# form-width-consistency — 需求分析

## 动机

各模块的新建/编辑/查看表单页宽度不一致：

| 页面 | 表单最大宽度 | 内边距 | 卡片边框 | 滚动区背景 |
|------|------------|--------|---------|-----------|
| FieldForm（基准） | **800px** (`card-inner max-width`) | 24px 32px | 1px #E4E7ED | 透明 |
| EventTypeSchemaForm | **800px** (`card-inner max-width`) | 24px 32px | 1px #E4E7ED | 透明 |
| EventTypeForm | **无限制**（充满容器） | 24px 32px | 1px #E4E7ED | 透明 |
| FsmStateDictForm | **无限制**（充满容器） | 24px | 无边框 | #F5F7FA |

`FieldForm` 和 `EventTypeSchemaForm` 已对齐（均有 `card-inner max-width: 800px`）。`EventTypeForm` 和 `FsmStateDictForm` 缺少宽度约束，在宽屏上输入框会拉伸到很宽，影响可读性和视觉一致性。

不修复的话：同一套管理平台内不同页面视觉差异明显，宽屏下 EventType/FsmStateDict 表单输入框过宽，阅读体验差。

## 优先级

中。纯视觉一致性修复，无功能影响，修改量小。

## 预期效果

打开「事件类型 → 新建」和「状态字典 → 新建状态」，表单卡片宽度与「字段管理 → 新建字段」完全一致：
- 卡片内容区最大宽度 800px，居中显示
- 滚动区 padding 24px 32px
- 卡片有 1px #E4E7ED 边框
- 滚动区背景透明（去掉 FsmStateDictForm 的 #F5F7FA 灰底）

## 依赖分析

- **依赖**：无特殊依赖，纯 CSS/HTML 结构调整
- **谁依赖本需求**：无下游依赖

## 改动范围

| 文件 | 改动类型 |
|------|---------|
| `frontend/src/views/EventTypeForm.vue` | 修改——补 `card-inner` 结构 + CSS |
| `frontend/src/views/FsmStateDictForm.vue` | 修改——补 `card-inner` 结构 + CSS，去灰底 |

共 2 个文件，纯前端 CSS/模板调整。

## 扩展轴检查

- 不影响新增配置类型（只改 CSS，handler/service/store 不变）
- 正面：统一 `card-inner` 模式后，新增表单模块只需参考同一套结构，降低不一致风险

## 验收标准

| # | 标准 | 验证方法 |
|---|------|---------|
| R1 | EventTypeForm 表单卡片内容区最大宽度 800px，宽屏下不充满 | 浏览器截图，拖宽窗口验证 |
| R2 | FsmStateDictForm 表单卡片内容区最大宽度 800px，宽屏下不充满 | 浏览器截图，拖宽窗口验证 |
| R3 | FsmStateDictForm 滚动区背景为透明（不再是 #F5F7FA 灰色） | 浏览器截图 |
| R4 | FsmStateDictForm 表单卡片有 1px solid #E4E7ED 边框 | 浏览器截图 |
| R5 | EventTypeForm 与 FieldForm 视觉宽度目测一致 | 两页面对比截图 |
| R6 | `npx vue-tsc --noEmit` 0 errors；`npm run build` 成功 | 命令输出 |

## 不做什么

- 不改 FieldForm 和 EventTypeSchemaForm（已对齐，不动）
- 不改列表页、header 区等非表单区域
- 不修改 label-width、字段顺序、输入框类型等表单逻辑
- 不抽取公共 CSS 类（两处改动，不值得过度抽象）
