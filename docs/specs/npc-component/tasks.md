# 需求 2：任务拆解

> **状态：全部完成** — 2026-04-07

## [x] T1: ComponentPanel 折叠面板组件 (R4, R5, R12)

**新增文件：**
- `frontend/src/components/ComponentPanel.vue`

**职责：**
- Props: `componentName`, `displayName`, `schema`, `modelValue`, `required`
- el-collapse-item 包装，标题显示中文名 + 英文括注 + 必选标记
- 内部渲染 SchemaForm（formFooter.show = false，不显示独立保存按钮）
- v-model 双向绑定组件数据

**做完了是什么样：** `<ComponentPanel name="movement" displayName="移动组件" :schema="movementSchema" v-model="data" />` 渲染为折叠面板，展开后显示 movement 的表单字段。

---

## [x] T2: NpcTemplateForm — 预设选择 + 组件勾选 (R1, R2, R3)

**新增文件：**
- `frontend/src/views/NpcTemplateForm.vue`

**职责（本任务只做上半部分）：**
- name 输入框（新建可编辑，编辑锁定）
- 预设选择器（el-select，加载 npc-presets API）
- 选择预设后：解析 required/default/optional → 更新 enabledComponents
- 组件勾选区（el-checkbox-group）：必选 disabled、默认勾选、可选未勾选
- 编辑模式：预设锁定，组件勾选状态从已有数据还原

**不做：** 组件面板区（T3）、保存逻辑（T3）

**做完了是什么样：** 新建页面选预设 → checkbox 自动勾选对应组件，必选项灰色不可取消。编辑页面预设和勾选状态正确回显。

---

## [x] T3: NpcTemplateForm — 组件面板区 + 保存逻辑 (R4, R5, R6, R7)

**修改文件：**
- `frontend/src/views/NpcTemplateForm.vue` — 添加组件面板区 + 保存

**职责：**
- 加载已勾选组件的 schema（从 component-schemas API）
- 为每个已勾选组件渲染 ComponentPanel（el-collapse 包裹）
- 组件数据双向绑定到 `componentData[componentName]`
- 保存时组装 `{name, config: {preset, components: {...}}}` → 调用 API
- 编辑模式：从已有 config.components 回显各组件数据
- 条件字段（movement/personality）由 SchemaForm 内部处理（T5 已实现）

**做完了是什么样：** 完整的新建/编辑流程可用。选预设 → 勾选组件 → 填写字段 → 保存 → API 返回 201。编辑时数据回显。

---

## [x] T4: NpcTemplateList 专用列表页 (R8, R9)

**新增文件：**
- `frontend/src/views/NpcTemplateList.vue`

**职责：**
- 表格列：name / 预设名 / 已启用组件标签列表
- 操作列：编辑 / 删除
- 预设名从 `config.preset` 读取
- 组件列表从 `Object.keys(config.components || {})` 读取，用 el-tag 展示
- 空状态 + 新建按钮

**做完了是什么样：** 访问 `/npc-templates` → 看到表格，每行显示 name + preset + 组件 tag 列表。

---

## [x] T5: 路由更新 + 构建验证 (R10, R11)

**修改文件：**
- `frontend/src/router/index.js` — NPC 模板路由指向专用页面

**变更：**
```js
// 替换
...entityRoutes('npc-templates', 'NPC 模板', npcTemplateApi)
// 为
{ path: '/npc-templates', component: NpcTemplateList, meta: {...} },
{ path: '/npc-templates/new', component: NpcTemplateForm, meta: {...} },
{ path: '/npc-templates/:name(.*)', component: NpcTemplateForm, meta: {...} },
```

**做完了是什么样：** `npm run build` 通过。`docker compose up --build` 启动成功。NPC 模板页面使用专用组件。

---

## [x] T6: 文档更新 (R10)

**修改文件：**
- `docs/specs/v3-roadmap.md` — 需求 2 状态更新
- `docs/specs/npc-component/tasks.md` — 标记所有任务完成

**做完了是什么样：** Roadmap 反映当前状态。

---

## 任务依赖顺序

```
T1（ComponentPanel）
  → T2（NpcTemplateForm 上半：预设 + 勾选）
    → T3（NpcTemplateForm 下半：面板 + 保存）
      → T4（NpcTemplateList）
        → T5（路由 + 构建）
          → T6（文档）
```

严格串行，每个任务依赖前一个。

---

## 任务 × 验收标准映射

| 验收标准 | 覆盖任务 |
|----------|----------|
| R1: 预设选择自动勾选组件 | T2 |
| R2: 必选组件不可取消 | T2 |
| R3: 默认可取消，可选可添加 | T2 |
| R4: 折叠面板按 schema 渲染 | T1, T3 |
| R5: 条件字段正确 | T1, T3（依赖需求 1 的 SchemaForm） |
| R6: 保存格式 {preset, components} | T3 |
| R7: 编辑回显 | T2, T3 |
| R8: 列表展示 preset + 组件标签 | T4 |
| R9: 列表编辑/删除 | T4 |
| R10: npm run build 通过 | T5 |
| R11: docker compose up 成功 | T5 |
| R12: 中文 display_name | T1 |
