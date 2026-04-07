# 需求 2：设计方案

## 方案描述

### 总体思路

NPC 模板页面从通用 GenericList/GenericForm **独立出来**，实现专用的 NpcTemplateList + NpcTemplateForm。核心是 NpcTemplateForm 中的**预设驱动组件选择 + 折叠面板组件表单**。

后端不改——现有 npc-templates CRUD + component-schemas / npc-presets 只读 API 完全够用。

### 数据结构

NPC 模板存储在 `npc_templates` 集合，文档格式：

```json
{
  "name": "wolf_common",
  "config": {
    "preset": "reactive",
    "components": {
      "identity": { "name": "普通灰狼", "model_id": "wolf_01", "tags": ["predator"] },
      "position": { "x": 0, "y": 0, "z": 0, "orientation": 0, "zone_id": "" },
      "behavior": { "fsm_ref": "wolf_fsm", "bt_refs": {"Idle": "wolf/idle"} },
      "perception": { "visual_range": 150, "auditory_range": 300, "attention_capacity": 5 },
      "movement": { "move_type": "wander", "move_speed": 3.0, "wander_radius": 50 },
      "personality": { "personality_type": "aggressive", "decision_weights": {"threat": 0.6, "needs": 0.2, "emotion": 0.2}, "aggro_range": 80 }
    }
  }
}
```

### 前端组件结构

```
NpcTemplateForm.vue（页面）
├─ name 输入框
├─ 预设选择器（el-select，编辑模式锁定）
├─ 组件勾选区（el-checkbox-group）
│   ├─ 必选组件：disabled + "必选"标记
│   ├─ 默认组件：默认勾选，可取消
│   └─ 可选组件：默认不勾选，可添加
└─ 组件面板区（el-collapse）
    ├─ ComponentPanel (identity) — 展开后内含 SchemaForm
    ├─ ComponentPanel (position)
    ├─ ComponentPanel (movement)
    └─ ...
```

#### NpcTemplateForm.vue

**状态管理：**

```js
const presetName = ref('')           // 选中的预设名
const presetDef = ref(null)          // 预设定义（从 API 加载）
const enabledComponents = ref([])    // 已勾选的组件名列表
const componentData = ref({})        // 各组件的表单数据 {identity: {...}, ...}
const componentSchemas = ref({})     // 各组件的 schema {identity: {schema}, ...}
```

**预设选择流程：**

```
选择预设 "reactive"
  → 加载预设定义（required_components + default_components + optional_components）
  → enabledComponents = [...required, ...default]
  → 对每个 enabled 组件，加载其 schema
  → componentData 初始化为空对象
```

**保存时组装：**

```js
const payload = {
  name: name.value,
  config: {
    preset: presetName.value,
    components: {}
  }
}
for (const comp of enabledComponents.value) {
  payload.config.components[comp] = componentData.value[comp] || {}
}
```

#### ComponentPanel.vue

简单的折叠面板包装：

```vue
<el-collapse-item :name="componentName">
  <template #title>
    {{ displayName }} ({{ componentName }})
    <el-tag v-if="required" type="danger" size="small">必选</el-tag>
  </template>
  <SchemaForm v-model="localData" :schema="schema" :form-footer="{show: false}" />
</el-collapse-item>
```

Props: `componentName`, `displayName`, `schema`, `modelValue`, `required`

**SchemaForm 的 formFooter 设为 `{show: false}`**——不在每个组件里显示保存按钮，统一由 NpcTemplateForm 顶层保存。

#### NpcTemplateList.vue

专用列表页，比 GenericList 增加：

- preset 列（显示预设中文名）
- 已启用组件列（el-tag 列表）

#### 路由更新

```js
// NPC 模板使用专用页面
{ path: '/npc-templates', component: NpcTemplateList, ... },
{ path: '/npc-templates/new', component: NpcTemplateForm, ... },
{ path: '/npc-templates/:name(.*)', component: NpcTemplateForm, ... },
```

---

## 方案对比

### 方案 A（选定）：专用 NpcTemplateForm + ComponentPanel

如上所述。NPC 模板有独立的表单页，不复用 GenericForm。

**优点**：
- 组件勾选 + 折叠面板 + 预设逻辑无法用 GenericForm 表达
- 代码清晰，NPC 模板的特殊逻辑集中在一个文件
- 不污染通用组件

**缺点**：
- 增加了两个专用页面（不影响其他实体）

### 方案 B（不选）：扩展 GenericForm 支持"组件化模式"

在 GenericForm 中加 `if (isComponentMode)` 分支处理组件化逻辑。

**优点**：
- 复用 GenericForm，少一个文件

**缺点**：
- GenericForm 变得臃肿，NPC 模板逻辑和通用逻辑耦合
- 违反"不准为一个调用点创建抽象层"原则的精神——反过来也是，不应该把特殊逻辑塞进通用组件

**不选原因**：NPC 模板的交互模式与其他实体完全不同，不应该硬塞进通用框架。

---

## 红线检查

### `docs/standards/red-lines.md`

| 红线 | 合规 | 说明 |
|------|------|------|
| 禁止静默降级 | ✅ | 组件 schema 加载失败时显示错误提示 |
| 禁止过度设计 | ✅ | 只做预设驱动 + 组件勾选，不做拖拽排序等 |

### `docs/standards/frontend-red-lines.md`

| 红线 | 合规 | 说明 |
|------|------|------|
| 禁止数据源污染 | ✅ | componentData 用 ref，不直接修改源数据 |
| 禁止放行无效输入 | ✅ | 预设用 el-select，组件用 checkbox |
| 禁止 URL 编码遗漏 | ✅ | 沿用 encodeURIComponent |

### `docs/architecture/red-lines.md`

| 红线 | 合规 | 说明 |
|------|------|------|
| 禁止暴露技术细节 | ✅ | 组件用中文 display_name + 英文括注 |
| 禁止 {name,config} 破坏 | ✅ | config 内部结构为 {preset, components}，外层仍是 {name, config} |
| 禁止表单不友好 | ✅ | 空状态引导、必选标记、折叠面板 |

---

## 扩展性影响

- **新增配置类型**：不影响（本需求只改 NPC 模板页面）
- **新增表单字段**：✅ 正面。新增组件 schema → 种子脚本导入 → NPC 模板表单自动出现新组件选项

---

## 依赖方向

```
NpcTemplateForm.vue
  → ComponentPanel.vue → SchemaForm.vue → @lljj/vue3-form-element
  → api/generic.js (npcTemplateApi)
  → api/schema.js (componentSchemaApi, npcPresetApi)

NpcTemplateList.vue
  → api/generic.js (npcTemplateApi)
```

单向向下。

---

## Go 陷阱检查

不涉及后端改动，跳过。

---

## 前端陷阱检查

| 陷阱 | 是否涉及 | 处理 |
|------|---------|------|
| 响应式：reactive 整体替换 | 是 | componentData 用 ref({})，更新单个 key 用 `componentData.value[name] = {...}` |
| watch deep | 是 | ComponentPanel 的 v-model 需要 deep watch |
| el-collapse 状态 | 是 | activeNames 用 ref([])，默认全部折叠 |
| Axios 竞态 | 是 | 预设选择后并行加载多个组件 schema，用 Promise.all |
| v-for key | 是 | 组件面板用 componentName 作为 key |

---

## 配置变更

无。不新增任何后端配置或 Docker 变更。

---

## 测试策略

| 测试类型 | 覆盖内容 |
|----------|----------|
| 前端构建 | `npm run build` 通过 |
| 手动测试 | 新建 NPC 模板全流程（选预设→勾选→填写→保存→列表→编辑→回显） |
| 条件字段 | movement 选 wander/patrol 切换 |
| Docker 集成 | `docker compose up --build` + 端到端 CRUD |
