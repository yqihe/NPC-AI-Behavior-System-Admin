# form-width-consistency — 设计方案

## 方案描述

### 基准结构（FieldForm）

```
.form-card  (flex:1; padding:24px 32px; overflow-y:auto)
  └── .card-inner  (max-width:800px; margin:0 auto; bg:#fff; border:1px #E4E7ED; border-radius:8px; padding:32px)
        └── el-form + .form-actions
```

### 当前问题

**EventTypeForm**：`.form-scroll` 作为滚动容器，多个 `.form-card` 直接铺满宽度，无 800px 约束。

**FsmStateDictForm**：单个 `.form-card` 无宽度约束；`.form-scroll` 有 `#F5F7FA` 灰底；`.form-card` 无边框；padding 偏小（24px）。

### 修复方案

#### EventTypeForm.vue

在 `.form-card` CSS 中追加：
```css
.form-card {
  /* 已有：background, border, border-radius, padding */
  max-width: 800px;
  margin-left: auto;
  margin-right: auto;
}
```

效果：`.form-scroll` 内所有卡片（基本信息、扩展字段、操作栏）均居中且最宽 800px，与 FieldForm 视觉一致。模板不需要改动。

#### FsmStateDictForm.vue

CSS 变更：

```css
/* 变更前 */
.form-scroll {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
  background: #F5F7FA;   /* ← 删除灰底 */
}

.form-card {
  background: #fff;
  border-radius: 8px;
  padding: 24px;          /* ← 改为 32px */
  margin-bottom: 16px;    /* ← 删除（单卡片无需间距） */
  /* 缺少：border, max-width, margin auto */
}

/* 变更后 */
.form-scroll {
  flex: 1;
  overflow-y: auto;
  padding: 24px 32px;     /* ← 与 FieldForm 一致 */
}

.form-card {
  max-width: 800px;
  margin: 0 auto;
  background: #fff;
  border: 1px solid #E4E7ED;
  border-radius: 8px;
  padding: 32px;
}
```

模板不需要改动。

---

## 方案对比

### 备选方案：复制 FieldForm 的 card-inner 模式（在模板中加一层 div）

即为 EventTypeForm 和 FsmStateDictForm 各加一层 `<div class="card-inner">` 包裹内容，CSS 与 FieldForm 完全一致。

**不选原因**：
- EventTypeForm 有多个 `.form-card`，每个都要加 `<div class="card-inner">` 且改名，改动量大
- 直接给 `.form-card` 加 `max-width + margin: auto` 达到相同视觉效果，模板零改动
- 两页面不需要嵌套层级，结构更扁平

---

## 红线检查

**前端红线**（`docs/development/standards/red-lines/frontend.md`）：
- 无涉及数据源、枚举输入、URL 编码、JSON key、form disabled 等规则
- **vue-tsc 必跑**：纯 CSS 改动不影响类型，但仍需 build 验证 ✅

**ADMIN 专属红线**（`docs/development/admin/red-lines.md`）：
- 无涉及游戏服务端数据格式
- 无涉及引用完整性
- 无涉及硬编码、错误码等

无违反任何红线。

---

## 扩展性影响

**正面**：EventTypeForm 有 `card-title` 分节标题，其他模块未来可参考此模式。统一宽度后，新增表单模块有明确的视觉基线可参照。

---

## 依赖方向

```
frontend/src/views/EventTypeForm.vue    ← CSS only
frontend/src/views/FsmStateDictForm.vue ← CSS only
```

纯前端修改，无跨层依赖。

---

## 陷阱检查

- **CSS scoped 穿透**：修改均在本组件 scoped CSS 内，不影响其他组件 ✅
- **flex 溢出**：`.form-card` 设 `max-width: 800px` + `margin: auto`，不影响父容器 flex 布局 ✅
- **EventTypeForm 的 form-actions 卡片**：也有 `.form-card` 类，设 max-width 后同样居中，与上方卡片对齐，视觉效果更好 ✅

---

## 配置变更

无。纯 CSS 修改。

---

## 测试策略

- **R1–R5**：浏览器截图，拖宽窗口验证 800px 约束生效
- **R6**：`npx vue-tsc --noEmit && npm run build`
