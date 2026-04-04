# 任务拆解：前端表格自适应布局

## [x] T1: 清理 main.css 脚手架残留样式 (R3)

**文件**：`frontend/src/assets/main.css`

**做什么**：
- 移除 `#app` 的 `max-width`、`margin`、`padding`
- 移除 `@media (min-width: 1024px)` 整个媒体查询块（body flex + #app grid）
- 保留 `@import './base.css'`、`#app { font-weight: normal; }`、`a` 样式

**做完是什么样**：`#app` 不再限制最大宽度，AppLayout 的 `el-container` 可以全屏展开。

## [x] T2: EventTypeList + FsmConfigList 表格列自适应 (R1, R2)

**文件**：
- `frontend/src/views/EventTypeList.vue`
- `frontend/src/views/FsmConfigList.vue`

**做什么**：数据列 `width` → `min-width`，操作列 `width="180" fixed="right"` 不变。

**做完是什么样**：两个列表页表格铺满内容区，宽屏无右侧留白。

## [x] T3: NpcTypeList + BtTreeList 表格列自适应 (R1, R2)

**文件**：
- `frontend/src/views/NpcTypeList.vue`
- `frontend/src/views/BtTreeList.vue`

**做什么**：数据列 `width` → `min-width`，操作列 `width="180" fixed="right"` 不变。

**做完是什么样**：两个列表页表格铺满内容区，宽屏无右侧留白。

## [x] T4: 全局验证 (R4, R5)

**文件**：无代码改动

**做什么**：`docker compose up --build`，浏览器访问 4 个列表页，验证：
- 宽屏（≥1440px）：表格铺满，无大面积留白
- 窄屏：操作列固定右侧，可横向滚动
- AppLayout 布局正常，侧边栏 + 内容区无异常

**做完是什么样**：4 个列表页全部通过验收标准 R1–R5。

## 依赖顺序

```
T1 → T2 → T3 → T4
     （T2、T3 可并行，但 T1 必须先做）
```
