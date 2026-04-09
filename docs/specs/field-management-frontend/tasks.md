# 字段管理前端 — 任务拆解

## T1: Vue 3 项目骨架搭建 (R16) [x]

**涉及文件**：
- `frontend/package.json`（新建）
- `frontend/vite.config.js`（新建）
- `frontend/index.html`（新建）

**做完的样子**：`cd frontend && npm install && npm run dev` 能启动，浏览器访问 `localhost:3000` 看到空白页面（无报错），Vite proxy 配置 `/api` → `localhost:9821`。

---

## T2: 入口文件 + 路由 + 布局组件 (R18) [x]

**涉及文件**：
- `frontend/src/main.js`（新建）
- `frontend/src/App.vue`（新建）
- `frontend/src/router/index.js`（新建）

**做完的样子**：浏览器访问 `localhost:3000` 看到侧边栏布局（深色侧边栏 + 浅色主内容区），路由 `/fields`、`/fields/create`、`/fields/:id/edit` 已注册（组件暂为占位 div），`<router-view :key="route.fullPath" />`。

---

## T3: AppLayout 侧边栏组件 (R18) [x]

**涉及文件**：
- `frontend/src/components/AppLayout.vue`（新建）

**做完的样子**：侧边栏包含 logo「ADMIN 运营平台」、分组菜单（配置管理→字段管理），深色背景 `#1D2B3A`，当前路由高亮，点击菜单项跳转对应路由。与 mockup 中的侧边栏设计一致。

---

## T4: API 层 — Axios 实例 + 字段 API + 字典 API (R17, R20) [x]

**涉及文件**：
- `frontend/src/api/request.js`（新建）
- `frontend/src/api/fields.js`（新建）
- `frontend/src/api/dictionaries.js`（新建）

**做完的样子**：Axios 实例带响应拦截器（`code !== 0` 时 ElMessage.error + reject 携带 code），fieldApi 暴露 8 个方法（list/create/detail/update/delete/checkName/toggleEnabled/references），dictApi 暴露 list 方法。

---

## T5: 字段列表页 — 表格 + 筛选 + 分页 (R1, R2, R3, R4, R20) [x]

**涉及文件**：
- `frontend/src/views/FieldList.vue`（新建）

**做完的样子**：`/fields` 页面展示字段表格（ID/标识符/标签/类型badge/分类badge/引用计数/启用switch/创建时间/操作列），顶部筛选栏（标签搜索+类型/分类/状态下拉+搜索/重置按钮），底部分页组件。下拉选项从字典 API 加载。禁用行 50% 透明度。空状态 el-empty + 新建按钮。

---

## T6: 字段列表页 — Toggle + 编辑 + 删除交互 (R5, R11, R12, R13, R14) [x]

**涉及文件**：
- `frontend/src/views/FieldList.vue`（修改，追加交互逻辑）

**做完的样子**：
- Toggle：调用 toggleEnabled API，40010 冲突弹窗提示
- 编辑：已启用→弹窗「请先禁用」，禁用→跳转编辑页
- 删除：三态确认（启用/有引用/无引用），有引用时显示引用列表
- 引用详情弹窗：点击引用计数蓝色链接弹出 el-dialog，分模板引用和字段引用两个表格

---

## T7: 新建字段页 — 基础表单 (R6, R7, R15) [x]

**涉及文件**：
- `frontend/src/views/FieldForm.vue`（新建）

**做完的样子**：`/fields/create` 页面展示表单（标识符+标签+描述+类型下拉+分类下拉+暴露BB Key开关），顶部「返回」链接 + 页面标题。标识符 blur 时三态校验（checking/available/taken）。类型/分类下拉从字典 API 加载。保存按钮 loading 防重复提交。约束面板区域暂为空占位。

---

## T8: 编辑字段页 — 数据加载 + 受限模式 (R9, R10, R14) [x]

**涉及文件**：
- `frontend/src/views/FieldForm.vue`（修改，追加编辑模式逻辑）

**做完的样子**：`/fields/:id/edit` 复用 FieldForm，标识符只读（锁图标+灰色+黄色警告），数据从 detail API 预填充。ref_count > 0 时类型字段锁定+黄色警告，BB Key 受限提示。提交带 version 乐观锁，40010 冲突弹窗。40011 跳转列表。

---

## T9: 约束面板 — Integer/String/Boolean (R8) [x]

**涉及文件**：
- `frontend/src/components/FieldConstraintInteger.vue`（新建）
- `frontend/src/components/FieldConstraintString.vue`（新建）

**做完的样子**：Integer 面板（最小值+最大值+步长水平排列，带类型badge标题），String 面板（最小长度+最大长度+正则校验），Boolean 显示灰色「布尔类型无需约束配置」。`restricted` prop 控制黄色警告显示。v-model 双向绑定 constraints 对象。与 mockup 设计一致。

---

## T10: 约束面板 — Select + Reference (R8)

**涉及文件**：
- `frontend/src/components/FieldConstraintSelect.vue`（新建）
- `frontend/src/components/FieldConstraintReference.vue`（新建）

**做完的样子**：Select 面板（选项列表动态增删+最少/最多选择数+绿色提示），Reference 面板（引用字段列表+添加引用下拉仅显示enabled字段+展开预览+循环引用黄色警告）。v-model 双向绑定，restricted 控制警告。与 mockup 设计一致。

---

## T11: FieldForm 集成约束面板 + 默认值控件 (R8)

**涉及文件**：
- `frontend/src/views/FieldForm.vue`（修改，集成约束面板）

**做完的样子**：类型下拉切换时动态渲染对应约束面板，默认值控件根据类型切换（数字输入/文本输入/开关/选择器）。类型变更时清空旧约束数据。新建和编辑模式均正常工作。

---

## T12: Docker 部署配置 (R19)

**涉及文件**：
- `frontend/nginx.conf`（新建）
- `Dockerfile.frontend`（新建）
- `docker-compose.yml`（修改，新增 admin-frontend 服务）

**做完的样子**：`docker compose up --build` 后前端服务在 `localhost:3000` 可访问，nginx 反代 `/api/` 到后端，SPA 路由 fallback 到 index.html。
