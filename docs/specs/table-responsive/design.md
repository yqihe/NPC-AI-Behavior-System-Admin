# 设计方案：前端表格自适应布局

## 方案描述

### 1. 表格列宽：`width` → `min-width`

Element Plus 的 `el-table` 布局规则：
- 有 `width` 的列：固定该宽度，不参与剩余空间分配
- 有 `min-width` 的列：保证最小宽度，剩余空间按 `min-width` 比例分配

将 4 个列表页的数据列从 `width` 改为 `min-width`，操作列保持 `width` + `fixed="right"` 不变。

改动对照：

| 文件 | 列 | 原值 | 新值 |
|------|----|------|------|
| EventTypeList.vue | 名称 | `width="150"` | `min-width="150"` |
| | 威胁等级 | `width="100"` | `min-width="100"` |
| | 持续时间(s) | `width="120"` | `min-width="120"` |
| | 传播方式 | `width="100"` | `min-width="100"` |
| | 传播范围(m) | `width="120"` | `min-width="120"` |
| | **操作** | **`width="180"` 不变** | |
| NpcTypeList.vue | 名称 | `width="150"` | `min-width="150"` |
| | 状态机 | `width="150"` | `min-width="150"` |
| | 行为树数 | `width="100"` | `min-width="100"` |
| | 视觉范围 | `width="100"` | `min-width="100"` |
| | 听觉范围 | `width="100"` | `min-width="100"` |
| | **操作** | **`width="180"` 不变** | |
| FsmConfigList.vue | 名称 | `width="150"` | `min-width="150"` |
| | 状态数 | `width="100"` | `min-width="100"` |
| | 转换数 | `width="100"` | `min-width="100"` |
| | 初始状态 | `width="120"` | `min-width="120"` |
| | **操作** | **`width="180"` 不变** | |
| BtTreeList.vue | 名称 | `width="200"` | `min-width="200"` |
| | 完整路径 | `width="200"` | `min-width="200"` |
| | 根节点类型 | `width="150"` | `min-width="150"` |
| | 子节点数 | `width="100"` | `min-width="100"` |
| | **操作** | **`width="180"` 不变** | |

### 2. 清理 main.css 脚手架残留

当前 `main.css` 内容：

```css
#app {
  max-width: 1280px;    /* 限制了全屏布局 */
  margin: 0 auto;
  padding: 2rem;        /* 与 AppLayout el-main padding 冲突 */
  font-weight: normal;
}

@media (min-width: 1024px) {
  body { display: flex; place-items: center; }
  #app { display: grid; grid-template-columns: 1fr 1fr; padding: 0 2rem; }
}
```

这是 Vue `create-vue` 脚手架的默认样式，与 AppLayout（`el-container` + `el-aside` + `el-main` 全屏布局）完全冲突。清理为：

```css
#app {
  font-weight: normal;
}
```

仅保留 `font-weight`（全局字重设定），移除 `max-width`、`margin`、`padding`、媒体查询 grid 布局。`@import './base.css'` 和 `a` 样式保留不动。

## 方案对比

| | 方案 A：`min-width`（选用） | 方案 B：百分比 `width` |
|--|--|--|
| 做法 | 列 `width` → `min-width`，保留原数值 | 列 `width` 改为百分比（如 `20%`） |
| 优点 | 改动最小，保证最小可读宽度，宽屏自动扩展 | 严格等比例 |
| 缺点 | 无 | 窄屏下百分比可能小于内容所需宽度导致截断；需手工计算百分比且难以统一；Element Plus 对百分比 width 支持不如 min-width 自然 |
| 结论 | **选用** | 不选 |

## 红线检查

逐条对照：

- `docs/standards/red-lines.md`：无涉及（纯样式改动，不涉及数据/API/Go）
- `docs/standards/go-red-lines.md`：无涉及（不改后端）
- `docs/standards/frontend-red-lines.md`：无涉及（不改数据源、不改输入组件、不改 URL）
- `docs/architecture/red-lines.md`：无涉及（不改数据格式、不改 UI 标签文案、不改空状态处理）

**无违反。**

## 扩展性影响

- **新增配置类型**：正面。新列表页参照已有模式使用 `min-width` 即可自动适配
- **新增表单字段**：不涉及

## 依赖方向

本次改动全部在 `frontend/src/views/` 和 `frontend/src/assets/` 中，不涉及跨包依赖。

## Go 陷阱检查

不涉及后端，跳过。

## 前端陷阱检查

- **CSS / 布局 — flex 溢出**（`frontend-pitfalls.md`）：`el-main` 是 flex 子元素，但表格本身处理溢出靠 `fixed="right"` + 内部横向滚动，不受 `min-width: auto` 影响。无风险。
- **scoped 穿透**：本次不新增 scoped 样式，无风险。

## 配置变更

无。不涉及 JSON 配置。

## 测试策略

纯 CSS/属性改动，无逻辑变更：

- **手动验证**：`docker compose up --build` 后在浏览器分别访问 4 个列表页，调整窗口宽度观察表格铺满效果
- **验收点**：
  - 宽屏下表格铺满内容区，无右侧大面积留白
  - 窄屏下操作列固定右侧，可横向滚动
  - main.css 清理后 AppLayout 布局不受 max-width 限制
