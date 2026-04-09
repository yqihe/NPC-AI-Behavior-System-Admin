# 字段管理前端 — 需求分析

## 动机

V3 重写后后端字段管理 API 已全部就绪（8 个接口 + 字典接口），但前端代码在 `c2d3d90` 清除 V2 后**完全不存在**。`frontend/` 目录为空，无 Dockerfile、无 Docker Compose 前端服务。

不做的后果：后端 API 无法被策划/运营人员使用，字段作为三层配置模型（字段→模板→NPC）的原子底层，阻塞模板管理和 NPC 管理的全部后续工作。

## 优先级

**P0 — 阻塞性**。字段是整个配置体系的基础层，模板管理依赖字段列表（`enabled=true` 筛选），NPC 管理依赖模板。不完成字段前端，后续所有配置模块无法开工。

## 预期效果

### 场景 1：策划浏览字段列表
策划打开 `/fields`，看到带筛选栏（标签搜索 + 类型/分类/状态下拉）的表格，支持后端分页。禁用字段行半透明。点击引用计数蓝色链接弹出引用详情。

### 场景 2：策划新建字段
策划点击「+ 新建字段」进入 `/fields/create`，填写标识符（blur 时实时校验唯一性）、中文标签、描述、类型（下拉选择后动态渲染对应约束面板）、分类、默认值、是否暴露 BB Key。保存后字段默认禁用，需手动启用。

### 场景 3：策划编辑字段
策划在列表点击「编辑」，若字段已启用则弹窗提示「请先禁用」；禁用状态进入 `/fields/:id/edit`，标识符只读。若 ref_count > 0，类型字段锁定并显示黄色警告，约束只能放宽不能收紧。提交带 version 乐观锁。

### 场景 4：策划删除字段
点击「删除」：若已启用→弹窗「请先禁用」；若禁用但有引用→弹窗显示引用列表 + 红色警告；若禁用且无引用→确认后软删除。

### 场景 5：策划启禁用字段
点击 toggle 开关，带 version 乐观锁调用 API。冲突时弹窗提示「数据已变更，请刷新重试」。

## 依赖分析

### 上游依赖（已完成）
- 后端 8 个字段 API：`fields/{list,create,detail,update,delete,check-name,references,toggle-enabled}`
- 后端字典 API：`POST /api/v1/dictionaries`（提供 field_type、field_category 下拉选项）
- 后端错误码体系：40001-40015 + 40000/50000

### 下游被依赖
- 模板管理前端：需要字段列表（`enabled=true`）作为字段选择器数据源
- NPC 管理前端：间接依赖（通过模板）

### 外部依赖
- BB Key 校验（40008）：依赖行为树模块，当前后端已预留错误码但未实装 → 前端正常处理该错误码即可，不阻塞

## 改动范围

| 区域 | 新增文件 | 改动文件 |
|------|----------|----------|
| frontend/ 项目骨架 | ~15 文件（package.json, vite.config.js, index.html, App.vue, router, api/index.js 等） | 0 |
| frontend/src/api/ | 2 文件（fields.js, dictionaries.js） | 0 |
| frontend/src/views/ | 2 文件（FieldList.vue, FieldForm.vue） | 0 |
| frontend/src/components/ | 5 文件（AppLayout.vue, FieldConstraintInteger.vue, FieldConstraintString.vue, FieldConstraintSelect.vue, FieldConstraintReference.vue） | 0 |
| Docker 配置 | 2 文件（Dockerfile.frontend, nginx.conf） | 1 文件（docker-compose.yml 加前端服务） |
| **合计** | **~26 文件** | **1 文件** |

## 扩展轴检查

- **新增配置类型**：本次建立的 Vue 项目骨架（路由、API 层、布局组件）直接复用于模板/NPC/事件/FSM/BT/区域等后续模块，只需加 view + api 文件。**正面影响**。
- **新增表单字段**：约束面板按类型拆分为独立组件，新增字段类型只需加一个 `FieldConstraint<Type>.vue`。字典驱动下拉，不硬编码。**正面影响**。

## 验收标准

### 列表页
- **R1**：`/fields` 页面展示字段表格，包含 ID、标识符、中文标签、类型（badge）、分类（badge）、引用计数（蓝色链接）、启用开关、创建时间、操作列（编辑/删除）
- **R2**：筛选栏支持标签模糊搜索 + 类型/分类/状态下拉，搜索和重置按钮，所有筛选走后端分页
- **R3**：分页组件显示总数 + 页码导航，默认 page_size=20
- **R4**：禁用字段行以 50% 透明度显示
- **R5**：点击引用计数弹出引用详情弹窗，分「模板引用」和「字段引用」两个表格

### 新建/编辑页
- **R6**：`/fields/create` 表单包含：标识符、中文标签、描述、字段类型、分类、默认值、暴露 BB Key、约束配置
- **R7**：标识符输入框 blur 时调用 `check-name` API，红/绿反馈
- **R8**：字段类型下拉切换时动态渲染对应约束面板（integer/float→数值约束、string→文本约束、boolean→无约束提示、select→选项列表、reference→引用列表）
- **R9**：`/fields/:id/edit` 复用新建表单，标识符只读（锁定图标+灰色+黄色警告），数据从 detail API 预填充，提交带 version 乐观锁
- **R10**：编辑页 ref_count > 0 时：类型字段锁定+黄色警告(40006)、约束面板显示「只能放宽」警告(40007)

### 交互与错误处理
- **R11**：启用/禁用 toggle 调用 `toggle-enabled` API（带 version），40010 冲突弹窗提示
- **R12**：编辑按钮：已启用→弹窗「请先禁用」；禁用→跳转编辑页
- **R13**：删除按钮三态：已启用→弹窗「请先禁用」；禁用有引用→弹窗显示引用列表+警告；禁用无引用→确认删除
- **R14**：所有 15 个错误码（40001-40015）正确映射到 UI 反馈（输入框红字/全局 toast/弹窗/黄色警告）
- **R15**：表单提交按钮在请求期间 loading 禁用，防重复提交

### 基础设施
- **R16**：Vue 3 + Element Plus + Vite 项目骨架可 `npm run dev` 启动，Vite proxy 代理 `/api` 到 `localhost:9821`
- **R17**：Axios 响应拦截器统一处理 `code !== 0` 的业务错误（中文 message toast），并 `return Promise.reject()`
- **R18**：AppLayout 组件含侧边栏导航 + 主内容区，`<router-view :key="route.fullPath" />`
- **R19**：Dockerfile.frontend 多阶段构建 + nginx.conf 反代配置 + docker-compose.yml 前端服务
- **R20**：下拉选项（字段类型、分类）全部从字典 API 动态获取，不硬编码

## 不做什么

- **不做** SchemaForm 通用组件：V2 的 SchemaForm 是为 MongoDB 动态 schema 设计的，V3 字段管理的表单结构固定，直接用 Element Plus 原生表单组件
- **不做**导入导出、列排序、字段克隆（已明确延后到毕设后）
- **不做**批量操作（后端已移除批量接口）
- **不做**仪表盘/Dashboard 页（后续独立 spec）
- **不做**其他配置模块的页面（模板/NPC/事件/FSM/BT/区域各自独立 spec）
- **不做** Pinia/Vuex 全局状态管理（CRUD 页面用组件级 ref/reactive 即可）
- **不做** i18n 国际化（面向国内策划，界面全中文）
