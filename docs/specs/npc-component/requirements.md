# 需求 2：NPC 模板组件化

## 动机

需求 1 实现了通用动态表单，但 NPC 模板页面目前和其他实体一样——一个扁平的 JSON 编辑器。这不是 V3 的目标。

V3 的核心特性是**组件化 NPC 模板**：运营人员选预设 → 勾选组件 → 填写各组件字段 → 保存。这是"AI 角色系统"区别于普通配置管理工具的关键功能，也是毕设答辩的核心展示点。

**不做会怎样**：NPC 模板页面无法体现组件化设计，运营人员需要手写 JSON 来配置 NPC，完全违背"消灭硬编码"的目标。

## 优先级

**最高**。需求 0 和 1 是基础设施，本需求才是业务核心。

## 预期效果

### 场景 1：新建 NPC 模板
1. 运营点击"NPC 模板" → "新建"
2. 输入名称 `wolf_common`
3. 选择预设 → 下拉选 `reactive（反应型）`
4. 自动勾选：identity ✅ position ✅ behavior ✅ perception ✅ movement ✅ personality ✅
5. 可选组件列表显示：needs / emotion / memory / social（均未勾选）
6. 运营勾选 `emotion`，emotion 的表单区域出现
7. 每个已勾选组件展开为独立表单区域（折叠面板），各组件字段按 schema 渲染
8. 填写各组件字段 → 点击保存
9. 后端接收 `{name: "wolf_common", config: {preset: "reactive", components: {identity: {...}, position: {...}, ...}}}`
10. 返回列表，看到新建的模板

### 场景 2：编辑 NPC 模板
1. 点击已有模板 `wolf_common` → 进入编辑页
2. 预设显示为 `reactive`（不可修改——改预设等于重建）
3. 各组件数据回显到对应表单区域
4. 运营修改 movement 的 `move_speed` → 保存

### 场景 3：列表展示
- 列表每行显示：name、预设名、已启用组件标签列表（如 `identity position behavior perception movement personality`）

### 场景 4：条件字段
- movement 组件选 `wander` → `wander_radius` 出现
- personality 组件选 `aggressive` → `aggro_range` 出现

### 场景 5：组件面板折叠
- 10 个组件不会同时展开，默认全部折叠，运营点击展开需要编辑的组件
- 必选组件（identity/position）标记为"必选"，不可取消

## 依赖分析

### 前置依赖
- **需求 0**（已完成）：通用 CRUD 框架
- **需求 1**（已完成）：SchemaForm 组件、种子脚本已导入 10 个组件 schema + 4 个预设

### 谁依赖本需求
- 需求 6（FSM/BT 编辑器 Schema 化）：BT 编辑器的黑板 Key 下拉框需要读取已启用组件的 blackboard_keys

## 改动范围

### 后端（Go）— 约 2 个文件
- `backend/internal/service/generic.go` 或新文件 — NPC 模板保存时校验 preset + components 合法性（可选，后端兜底）
- 无新增 API — 使用现有 npc-templates CRUD + component-schemas / npc-presets 只读 API

### 前端（Vue 3）— 约 4 个文件

| 文件 | 说明 |
|------|------|
| `src/views/NpcTemplateForm.vue` | NPC 模板专用表单页（替代 GenericForm） |
| `src/views/NpcTemplateList.vue` | NPC 模板专用列表页（替代 GenericList） |
| `src/components/ComponentPanel.vue` | 单个组件的折叠面板（标题 + SchemaForm） |
| `src/router/index.js` | NPC 模板路由指向专用页面 |

## 扩展轴检查

- **新增配置类型**：不涉及（本需求是 NPC 模板专用逻辑）
- **新增表单字段**：✅ 有利。服务端新增组件 schema → 种子脚本重新导入 → 前端自动渲染新组件

## 验收标准

### 核心流程
- **R1**：新建 NPC 模板时可选择预设（simple/reactive/autonomous/social），选择后自动勾选对应组件
- **R2**：必选组件（identity/position）标记为"必选"且不可取消
- **R3**：默认组件可取消勾选，可选组件可添加勾选
- **R4**：每个已勾选组件展示为折叠面板，面板内按组件 schema 渲染 SchemaForm
- **R5**：条件字段正确工作（movement 的 wander_radius、personality 的 aggro_range）
- **R6**：保存时 config 格式为 `{preset: "...", components: {identity: {...}, ...}}`
- **R7**：编辑模式下数据正确回显，预设锁定，组件勾选状态和表单数据还原

### 列表页
- **R8**：NPC 模板列表展示 name、预设名、已启用组件标签
- **R9**：列表支持编辑跳转和删除

### 工程质量
- **R10**：`npm run build` 构建通过
- **R11**：`docker compose up --build` 启动成功
- **R12**：组件面板展示中文名称（来自 schema 的 display_name）

## 不做什么

- ❌ 不做后端组件组合校验（后端只做基础 JSON 格式校验，组件合法性校验留给未来）
- ❌ 不做 BT 编辑器的黑板 Key 联动（需求 6 做）——本需求只预留 blackboard_keys 数据
- ❌ 不做 NPC 模板间的引用检查（如 behavior 组件的 fsm_ref 是否存在）
- ❌ 不做拖拽排序组件顺序
- ❌ 不做预设自定义（预设由服务端定义，ADMIN 只读）
