# field-template-frontend-sync — 需求分析

## 动机

后端「字段管理」+「模板管理」两个模块的全部 API 已经实现并跑通 199/199 集成测试（含字段段 17 个错误码 40001-40017 + 模板段 12 个错误码 41001-41012），但前端目前的状态是：

| 模块 | 后端 | 前端 |
|---|---|---|
| 字段管理 | 完成 + 集成测试通过 | **存在但落后**：8 个 API 已对接、UI 完整，但 `field-constraint-hardening` 引入的新错误码（40016 嵌套 / 40017 空 refs）和新校验语义（reference 下拉必须排除其他 reference 字段）尚未在前端体现 |
| 模板管理 | 完成 + 集成测试通过（含错误码 41012 `ErrTemplateFieldIsReference`）| **完全空白**：0 个 vue 文件、0 个 `api/templates.ts`、0 个路由、菜单里没有入口 |

不修的后果：

- **字段管理体验失真**：reference 字段创建/编辑时，前端「被引用字段」下拉列表当前还会展示其他 reference 字段，用户能选中后会被后端 40016 拒绝——属于「前端允许的操作后端却拒绝」的反向分裂，是最差的 UX 模式之一；同理 `refs=[]` 的 40017 兜底提示也缺失。
- **模板管理无法使用**：策划无法在 ADMIN 里新建/编辑模板，必须直接调 REST API。等于这个核心功能上线但不可达。
- **后续 NPC 管理无法启动**：NPC 管理依赖「选模板填值」的入口，模板前端不上线，NPC 模块的前端连第一步都迈不出去。
- **mockup 资产闲置**：`docs/v3-PLAN/mockups-template.pen` 已经画好 9 个高保真帧（列表 / 新建 / 编辑·未引用 / 编辑·已引用 / 删除确认 / reference 弹层 / 引用详情 / 启用下禁止编辑 / 启用下禁止删除），不落地等于白画。

## 优先级

**P0 — 当前阶段最高优先级**。

依据：

1. 后端两个模块已 100% ready，前端是唯一卡点；
2. NPC 管理是项目的核心可演示功能，依赖模板前端走通；
3. 模板前端的 mockup 已完成，避免了「前端规划期」，可以直接进入实现期；
4. 字段管理前端的对齐工作量很小（约 5 处改动），但不修的话每天都在产生反向 UX 分裂的 bug。

## 预期效果

### 字段管理前端的对齐（FA）

| 场景 | 当前 | 期望 |
|---|---|---|
| FA-1：在创建/编辑 reference 字段时打开「被引用字段」下拉 | 列表展示所有启用字段，包含其他 reference 字段 | **过滤掉其他 reference 字段**，下拉只看到 leaf 字段（integer / float / string / boolean / select）|
| FA-2：API 返回 40016 `ErrFieldRefNested` | 走通用拦截器 toast | 显示明确的本地化提示「不能引用 reference 类型字段，禁止嵌套」，表单状态保持可继续编辑 |
| FA-3：API 返回 40017 `ErrFieldRefEmpty` | 走通用拦截器 toast | 显示「reference 字段必须至少选择一个目标字段」 |
| FA-4：提交时 `properties` 形状校验（handler 层 40000）| 当前实现已保证传对象 | **仅需 verify 不退化** |
| FA-5：约束子组件 select 的 `minSelect`/`maxSelect` | 已实现 | **保持，用 vue-tsc 验证类型不退化** |

### 模板管理前端从 0 到 1（TA）

借鉴 mockup 的 9 个帧，但**风格沿用现有 `FieldList.vue` / `FieldForm.vue` 的视觉系统**（Element Plus 默认 + scoped CSS + 现有色板）。

| 场景 | 实现 |
|---|---|
| TA-1：左侧菜单 | 「模板管理」菜单项加在「字段管理」下方，路由 `/templates` |
| TA-2：列表页 `/templates` | 表格列：ID / 模板标识 / 中文标签 / 被引用数（蓝色 link 可点）/ 启用 switch / 创建时间 / 操作（编辑/删除）；顶部中文标签模糊搜索 + 启用状态 select；筛选走后端 `enabled` 参数；分页底部；**停用模板除操作列外整行 opacity 0.5**，操作列保持高亮；**不展示描述和字段数列**（features.md 功能 1 的约束）|
| TA-3：新建页 `/templates/create` | 三段式：基本信息卡（标识 + 中文标签 + 描述，描述 ≤ 512 字符）+ 字段选择卡（按 `category` 折叠分组，每行 3 个字段 cell，普通字段普通边框、reference 字段紫色边框 + chevron 触发 popover）+ 已选字段配置卡（每行：标签 / 标识 / 类型 tag / 必填 checkbox / 上下移动按钮）；保存按钮文案就是 `保存`；标识 blur 触发唯一性校验 |
| TA-4：字段分类分组标题 | **必须用 `/api/v1/dictionaries` 返回的 `field_category` 字典中文标签**，禁止硬编码「基础属性 / 战斗数值 / 行为配置」等自造词；分组顺序跟随字典顺序 |
| TA-5：reference popover | 单击 reference 字段 cell 弹出浮层，列出该 reference 的子字段（必是 leaf，一层），多选 + 全选 / 全不选 + 底部「已选 X / N」计数；勾选/取消同步到下方「已选字段配置」区，自动去重；**reference 字段本身永远不写入 `req.fields`，模板存的是展开后的扁平 leaf `field_id` 数组**（对齐后端 features 功能 8 + 集成回顾第 7 条）|
| TA-6：编辑页 `/templates/:id/edit` | 复用 `TemplateForm.vue` + mode prop；`name` 灰底 readonly + lock 图标 + hint「模板标识创建后不可修改」 |
| TA-7：ref_count > 0 编辑锁定 | 顶部黄色警告条「该模板已被 N 个 NPC 引用，字段勾选与必填配置不可修改」；字段选择卡 + 已选字段配置卡 opacity 0.55 + 卡片标题加 `🔒 已锁定` tag；reference popover 仍可打开但只读浏览（确定按钮禁用）；label / 描述可改 |
| TA-8：停用字段的视觉标注 | 详情接口返回的 `TemplateFieldItem.enabled=false` 字段，在已选字段配置卡中**整行 opacity 0.55 + 左侧 ⚠ 警告图标**，提示运营「字段已停用，但引用关系保留」|
| TA-9：列表点编辑启用中模板 | 前端拦截，弹「无法编辑模板」对话框（橙色警告 + 操作步骤），按钮「知道了」和「立即停用」；「立即停用」点击后调 `toggle-enabled` 后跳转到编辑页 |
| TA-10：列表点删除启用中模板 | 前端拦截，弹「无法删除模板」对话框（类似 TA-9），删除前置条件区列出 ✗ 模板已停用 + ✓ 没有 NPC 在使用 |
| TA-11：列表点蓝色「被引用数」链接 | 弹「模板引用详情」对话框，调 `/templates/references` API；NPC 模块未上线时后端返回 `{npcs: []}`（`make` 生成空数组），前端展示 `el-empty "暂无 NPC 引用"` 占位 |
| TA-12：列表删除已停用模板 | 二次确认后调 `/templates/delete`；后端返回 41007（被 NPC 引用）时自动打开引用详情对话框 |
| TA-13：表单切换字段勾选 | 已选字段配置区实时同步增删；上下移动按钮调整数组顺序（允许跨分类）；`ref_count > 0` 时整列灰化禁用 |
| TA-14：错误码处理 | 41001-41012 全部本地化文案；41009/41010 走 TA-9/TA-10 弹窗；41008（被引用编辑字段列表）作为后端兜底，前端 UI 已禁用理论到不了；41003（不存在）跳回列表；**41012（勾选了 reference 类型字段）作为后端兜底**，前端 UI 本就不把 reference 写入 `req.fields`，理论到不了，到了就弹「reference 字段必须先展开子字段再加入模板」|

## 依赖分析

**依赖已完成的工作**：

- 后端 8 个字段 API + 8 个模板 API 已上线（`router.go`）；
- 后端跨模块事务编排已就绪：`FieldService.ValidateFieldsForTemplate` / `AttachToTemplateTx` / `DetachFromTemplateTx` / `GetByIDsLite`；
- 字段管理前端 `FieldList.vue` / `FieldForm.vue` / `FieldConstraint*.vue` / `api/fields.ts` / `api/request.ts` 已存在；
- 字典 `field_type` / `field_category` 由 `/api/v1/dictionaries` 提供；
- mockup 帧已在 `mockups-template.pen`；
- 前端基建：Vue 3.5 + TypeScript strict + Element Plus + Vite + axios，已走通。

**谁依赖本需求**：

- **NPC 管理前端**：NPC 创建必须先选模板，没有模板列表/详情接口的前端，NPC 前端无法启动；
- **运营人员**：策划要在 ADMIN 里实际创建模板，不能再走 REST 调试工具；
- **联调演示**：毕设答辩演示流程要「创建字段 → 组装模板 → 创建 NPC → 游戏服务端启动拉取」，模板前端是中间的不可绕过点。

## 改动范围

**字段管理前端对齐（FA）**：

| 文件 | 性质 | 预估行数 |
|---|---|---|
| `frontend/src/api/fields.ts` | 改：追加 `FIELD_ERR` 常量表（40001-40017）| +20 |
| `frontend/src/components/FieldConstraintReference.vue` | 改：`enabledFields` 加载后追加 `f.type !== 'reference'` 过滤 | +3 |
| `frontend/src/views/FieldForm.vue` | 改：`handleSubmit.catch` 追加 40016 / 40017 分支 | +12 |

**模板管理前端 0→1（TA）**：

| 文件 | 性质 | 预估行数 |
|---|---|---|
| `frontend/src/api/templates.ts` | 新建：类型定义 + `TEMPLATE_ERR` 常量（含 41012）+ 8 个 API 函数 | +190 |
| `frontend/src/views/TemplateList.vue` | 新建：列表页 | +400 |
| `frontend/src/views/TemplateForm.vue` | 新建：新建/编辑共用表单 | +700 |
| `frontend/src/components/TemplateFieldPicker.vue` | 新建：字段选择卡（按 category 分组 + reference popover 触发）| +350 |
| `frontend/src/components/TemplateRefPopover.vue` | 新建：reference 子字段勾选弹层 | +200 |
| `frontend/src/components/TemplateSelectedFields.vue` | 新建：已选字段配置卡（必填 + 上下移动 + 停用字段标灰警告）| +240 |
| `frontend/src/components/TemplateReferencesDialog.vue` | 新建：模板被引用详情弹窗 | +120 |
| `frontend/src/components/EnabledGuardDialog.vue` | 新建：启用下禁止编辑/删除引导弹窗 | +140 |
| `frontend/src/router/index.ts` | 改：加 3 条路由 | +18 |
| `frontend/src/components/AppLayout.vue` | 改：菜单加「模板管理」项 | +5 |

**总计**：3 个字段端文件改动 + 8 个模板端新文件 + 2 个挂载点改动 = 13 个 .vue/.ts 文件。

**文档同步**：

- `docs/v3-PLAN/配置管理/字段管理/features.md` 加一行「前端 reference 下拉过滤已生效（与后端 40016 形成双重防御）」；
- `docs/v3-PLAN/配置管理/模板管理/features.md` 实现状态从「后端完成 / 前端实现中」更新为「全部完成」；
- `docs/v3-PLAN/配置管理/模板管理/frontend.md` 补齐内容（当前只有一行「待定义」）；
- `docs/development/frontend-pitfalls.md` 执行期发现新坑时追加。

## 扩展轴检查

**对「新增配置类型」方向**（加一组 handler/service/store/validator）：

- **正向**：模板前端的「按 category 分组 + popover 选项」模式可被未来的 FSM/BT 前端直接借鉴（「按字段类型分组的多选选择器」是通用模式）；
- **正向**：抽出的 `EnabledGuardDialog.vue` 是通用的「启用守卫」组件，未来 NPC/FSM/BT 列表都需要「启用中不能删除/编辑」的引导，可以直接复用；
- **正向**：`api/templates.ts` 的类型层 + 错误码常量层 + API 函数层是清晰的三段式，新增配置类型只需复制此模式。

**对「新增表单字段」方向**（加一个组件）：

- **正向**：字段管理前端的对齐巩固了 `FieldConstraint*.vue` 的契约，未来加 `FieldConstraintDate.vue` 等只需复制现有模式；
- **中性**：模板表单的 fields 列表是 `{field_id, required}` 扁平结构，对 SchemaForm 没有依赖；
- **正向**：`TemplateFieldPicker.vue` 抽成独立组件后，未来 NPC 表单要「为这个 NPC 选额外字段」也能复用。

## 验收标准

**FA — 字段管理前端对齐**：

- **R1**：在创建/编辑 reference 字段时，「被引用字段」下拉列表中，前端**不展示** `type === 'reference'` 的字段。下拉只展示 `type ∈ {integer, float, string, boolean, select}` 且 `enabled === true` 的字段。
- **R2**：当 API 返回 `code === 40016` 时，前端不走通用 toast，显示中文提示「不能引用 reference 类型字段（禁止嵌套），请选择普通字段」，表单状态保持可继续编辑。
- **R3**：当 API 返回 `code === 40017` 时，前端显示「reference 字段必须至少选择一个目标字段」。
- **R4**：手动验证「创建 reference 字段且 `ref_fields` 清空后提交」→ 返回 R3 的提示。
- **R5**：手动验证「编辑已有 reference 字段并尝试加入另一个 reference 字段」——下拉里看不到（R1），如果通过 devtools 直连 API 绕过前端，前端能正确处理 R2。

**TA — 模板管理前端 0→1**：

- **R6**：左侧菜单「字段管理」下方有「模板管理」项，点击进入 `/templates` 列表页。
- **R7**：列表页能展示后端返回的模板，列顺序与 mockup 一致（ID / 模板标识 / 中文标签 / 被引用数 / 启用 / 创建时间 / 操作）；**停用模板除操作列外整行 opacity 0.5**，操作列保持高亮可点。
- **R8**：列表页支持中文标签模糊搜索 + 启用状态三态筛选（全部 / 启用 / 停用）+ 后端分页。
- **R9**：列表点「被引用数」链接弹出引用详情对话框，调 `/templates/references` API；后端返回 `{npcs: []}` 空数组时显示 `el-empty "暂无 NPC 引用"` 占位。
- **R10**：列表点启用中模板的「编辑」按钮，前端**不发请求**，直接弹「无法编辑模板」对话框（橙色警告 + 操作步骤），「立即停用」点击后调 `toggle-enabled` 后跳转到编辑页。
- **R11**：列表点启用中模板的「删除」按钮，前端弹「无法删除模板」对话框（删除前置条件区：✗ 模板已停用 / ✓ 没有 NPC 在使用）。
- **R12**：列表点已停用模板的「删除」按钮，二次确认后调 delete API；后端返回 41007 时自动打开引用详情弹窗。
- **R13**：新建页有三段式布局：基本信息卡 / 字段选择卡 / 已选字段配置卡；描述输入框 maxlength 512。
- **R14**：字段选择卡按字段 `category` 折叠分组，分组标题**必须用**字典 `field_category` 返回的中文标签（不硬编码），分组顺序按字典顺序；每行 3 个字段 cell，每个 cell 显示字段标签 + `name · type`，可勾选。
- **R15**：reference 字段在字段选择卡中显示为紫色边框 + chevron 视觉，**点击不勾选 cell**，而是弹出 popover；popover 列出 reference 引用的子字段（子字段必然是 leaf，一层结构），多选 + 全选 / 全不选 + 底部「已选 X / N」计数 + 取消 / 确定按钮。
- **R16**：popover 内勾选/取消同步到下方「已选字段配置」区；同一字段被多个 reference 包含或被直接勾选时**自动去重**；**模板提交的 `req.fields` 只包含扁平的 leaf `field_id`，永远不包含 reference 字段本身的 ID**（对齐后端 `ValidateFieldsForTemplate` 的 41012 拦截）。
- **R17**：已选字段配置卡每行显示：字段标签 / `name` / 类型 tag / 必填 checkbox / 上下移动按钮；首行 ↑ 灰、末行 ↓ 灰；点击移动按钮在数组里 splice，UI 重渲染；允许跨分类移动。
- **R18**：已选字段配置卡中 `enabled === false` 的字段整行 opacity 0.55 + 左侧 ⚠ 警告图标（来自 `TemplateFieldItem.enabled`），提示运营「字段已停用」。
- **R19**：保存按钮文案就是 `保存`；不在按钮里写「默认未启用」等副文案。
- **R20**：标识 input 有 blur 唯一性校验，调 `/templates/check-name`，状态机 `idle` / `checking` / `available` / `taken`；`available` 时下方绿色对勾提示，`taken` 时红色提示。
- **R21**：编辑页与新建页用同一个 `TemplateForm.vue` 文件 + `mode` prop 切换；`mode === 'edit'` 时 `name` input 灰底 + lock 图标 + readonly + hint「模板标识创建后不可修改」。
- **R22**：编辑页 `ref_count > 0` 时：顶部加黄色警告条「该模板已被 N 个 NPC 引用，字段勾选与必填配置不可修改」；字段选择卡 + 已选字段配置卡 opacity 0.55 + 标题加 `🔒 已锁定` tag；上下移动按钮全部禁用；reference popover 仍可打开但只读浏览（确定按钮禁用或不存在）；`label` / 描述仍可改。
- **R23**：保存编辑后 41011（版本冲突）弹「该模板已被其他人修改，请刷新后重试」对话框，点击确认后跳回列表。
- **R24**：所有错误码（41001-41012）有本地化中文映射，存在 `api/templates.ts` 的常量表里；41012（兜底）文案「reference 字段必须先展开子字段再加入模板」。
- **R25**：`npm run build` 通过；`npx vue-tsc --noEmit` **零错误**（memory: `feedback_vue_tsc_required`）。
- **R26**：手动 e2e 流程：创建字段 a/b/c → 创建模板 t1 引用 a/b → 编辑 t1 加入 c → 保存 → 列表展示 → 启用 → 启用中点编辑被拦截 → 停用 → 再编辑 → 保存 → 停用 → 删除 → 列表消失。整个流程不报错。

## 不做什么

- **不做单元测试**（项目当前前端无测试基建；e2e 靠 R26 手动验证 + 后端 `tests/api_test.sh` 已覆盖 API 层）。
- **不做拖拽排序**（mockup 是上下移动按钮，不引入 dnd 库）。
- **不做模板字段的「默认值覆盖」**（毕设后扩展，已知限制）。
- **不做 reference popover 的「+N 新子字段差异提示」**（features.md 已知限制里写明）。
- **不做 NPC 列表搜索/分页交互**（NPC 模块未上线，引用详情对话框只展示空占位）。
- **不动 `mockups-template.pen` 文件**（只读借鉴）。
- **不引入 Pinia / VueUse / 任何新的运行时依赖**（保持现有 axios + Element Plus + scoped CSS 体系）。
- **不重写 `FieldList.vue` / `FieldForm.vue`**（只做对齐补丁 R1-R5，视觉和交互不变）。
- **不动后端任何代码**（所有改动限制在 `frontend/`）。
- **不实现「克隆模板」/「导入导出」**（毕设后延后功能）。
- **不动菜单项以外的 `AppLayout.vue` 内容**（不重构 sidebar 主题色或 layout）。

---

**Phase 1 完成，停下等待审批**。审批通过后进入 Phase 2 设计方案。
