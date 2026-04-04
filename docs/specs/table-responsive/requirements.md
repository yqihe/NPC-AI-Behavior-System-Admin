# 需求：前端表格自适应布局

## 动机

当前 4 个列表页面（事件类型、NPC 类型、状态机、行为树）的 `el-table` 所有列均使用固定像素 `width`，总宽度 650–830px。而内容区（`el-main`）宽度随浏览器窗口变化，宽屏下表格右侧留出大片空白，窄屏下可能触发横向滚动条。

此外 `main.css` 残留 Vue 脚手架默认样式（`#app { max-width: 1280px; }` + 媒体查询 grid 布局），与 AppLayout 的 `el-container` 全屏布局冲突，进一步限制了可用宽度。

不做的话：表格在宽屏上浪费空间，视觉效果差；窄屏下可能出现不必要的横向滚动。

## 优先级

中。属于 UI 体验优化，不阻塞功能开发，但作为运营管理平台的基础列表页，视觉合理性直接影响使用体验。当前阶段功能已基本成型，适合做这类打磨。

## 预期效果

- **宽屏场景**（≥1440px）：表格铺满内容区，各数据列按比例分配宽度，操作列固定在右侧
- **中等屏幕**（1024–1440px）：表格仍铺满，列宽按比例缩放但不小于最小宽度
- **窄屏场景**（<1024px）：当内容区不足以容纳所有列的最小宽度时，操作列 `fixed="right"` 生效，出现横向滚动

## 依赖分析

- 依赖：无，纯前端样式调整
- 被依赖：无

## 改动范围

涉及 5 个前端文件：

| 文件 | 改动 |
|------|------|
| `frontend/src/views/EventTypeList.vue` | 列 `width` → `min-width` |
| `frontend/src/views/NpcTypeList.vue` | 列 `width` → `min-width` |
| `frontend/src/views/FsmConfigList.vue` | 列 `width` → `min-width` |
| `frontend/src/views/BtTreeList.vue` | 列 `width` → `min-width` |
| `frontend/src/assets/main.css` | 移除脚手架残留样式 |

## 扩展轴检查

- **新增配置类型**：不影响。新列表页按同样模式使用 `min-width` 即可。
- **新增表单字段**：不涉及。

本需求不涉及两条扩展轴，属于 UI 层面的统一优化。

## 验收标准

- **R1**：4 个列表页的 `el-table` 数据列（非操作列）全部使用 `min-width` 而非 `width`，表格自动铺满父容器宽度
- **R2**：操作列保持 `width`（固定宽度）+ `fixed="right"` 不变
- **R3**：`main.css` 中 `#app` 的 `max-width: 1280px`、脚手架 grid 布局、多余 padding 被清理，不再限制 AppLayout 全屏展开
- **R4**：宽屏（≥1440px）下表格无右侧大面积留白
- **R5**：窄屏下操作列仍可固定右侧，表格可横向滚动

## 不做什么

- 不做列宽可拖拽调整
- 不做移动端适配（< 768px）
- 不做表格虚拟滚动
- 不改表格的数据列数量或内容
