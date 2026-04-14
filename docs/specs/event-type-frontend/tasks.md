# 事件类型管理 — 前端页面开发 · 任务拆解

> 按依赖顺序排列。每个任务 1-3 个文件，单一明确产出。

---

## [x] T1: API 模块 — 类型定义 + 错误码 + 请求函数 (R1-R31 基础)

**涉及文件**：
- 新增 `frontend/src/api/eventTypes.ts`

**做完了是什么样**：
- 导出 `EventTypeListQuery`、`EventTypeListItem`、`ExtensionSchemaItem`、`EventTypeDetail`、`CreateEventTypeRequest`、`UpdateEventTypeRequest`、`CheckNameResult` 类型
- 导出 `EVENT_TYPE_ERR` 错误码常量（42001-42015）
- 导出 `eventTypeApi` 对象（list / create / detail / update / delete / checkName / toggleEnabled），全部带显式返回类型
- 复用 `ListData<T>` 从 `fields.ts` 或在本文件重新定义（不跨文件 import 列表泛型，保持模块独立）
- `npx vue-tsc --noEmit` 通过

---

## [x] T2: 路由注册 + 侧栏菜单项 (R1, R30)

**涉及文件**：
- 修改 `frontend/src/router/index.ts`
- 修改 `frontend/src/components/AppLayout.vue`

**做完了是什么样**：
- 新增 3 条路由：`/event-types`（list）、`/event-types/create`（form, isCreate=true）、`/event-types/:id/edit`（form, isCreate=false）
- AppLayout 侧栏 `group-config` 下新增「事件类型」菜单项，排在"字段管理"之后
- 使用合适的 Element Plus 图标（如 `Histogram` 或其他闪电/信号类图标）
- 点击菜单项可导航到 `/event-types`，activeMenu 正确高亮
- `npx vue-tsc --noEmit` 通过

---

## [x] T3: 列表页 — 表格 + 筛选 + 分页 (R1-R10)

**涉及文件**：
- 新增 `frontend/src/views/EventTypeList.vue`

**做完了是什么样**：
- 页面结构：page-header + filter-bar + table-wrap + pagination
- 表格列：ID / 事件标识 / 中文名称 / 感知模式(Tag) / 严重度 / TTL / 范围 / 启用(Switch) / 创建时间 / 操作(编辑·删除)
- 感知模式 Tag 颜色区分：visual=success(绿), auditory=默认(蓝), global=info(灰)
- 筛选栏：中文标签输入 + 感知模式 Select（硬编码 3 个选项）+ 启用状态 Select + 搜索/重置
- 后端分页：el-pagination 联动 query.page
- 已停用行 `row-disabled` + opacity 0.5（操作列除外）
- Toggle：confirm → detail 拿 version → toggleEnabled → 刷新，版本冲突弹 alert
- 编辑/删除：启用中 → 暂时 `ElMessage.warning` 占位（T5 接入 Guard）；已停用 → 跳转/确认删除
- 空数据：el-empty + 「新建事件类型」引导按钮
- v-loading 加载态
- `npx vue-tsc --noEmit` 通过

---

## [x] T4: 表单页 — 基本信息 + 扩展字段 + 新建/编辑 (R11-R25)

**涉及文件**：
- 新增 `frontend/src/views/EventTypeForm.vue`

**做完了是什么样**：
- form-header：返回按钮 + 分隔线 + 标题（新建/编辑）
- **基本信息卡片**（蓝色竖条标题）：
  - 事件标识：新建时 el-input + blur 校验 check-name（checking/available/taken 状态），编辑时 disabled + Lock 图标
  - 中文名称：el-input，必填
  - 感知模式：el-select（Visual/Auditory/Global），必填
  - 默认严重度：el-input-number，controls=false，0-100 范围校验
  - 默认 TTL：el-input-number，controls=false，>0 校验
  - 感知范围：el-input-number，controls=false，>=0 校验，Global 模式 disabled + 自动置 0 + 提示
- **扩展字段卡片**（橙色竖条标题 + "可选" Tag）：
  - 条件渲染：`extension_schema` 非空时显示
  - info 提示框说明扩展字段来源
  - v-for 遍历 schema，按 field_type 渲染 int/float/string/bool/select 控件
  - dirty 跟踪：`Set<string>` 记录交互过的字段，提交时只收集 dirty 字段
  - 编辑模式：从 config 中提取扩展字段值并标记 dirty
- **FormFooter**：取消 + 保存（loading 态）
- el-form 校验规则：name/display_name/perception_mode/default_severity/default_ttl/range 必填
- 错误处理：NAME_EXISTS/NAME_INVALID → 内联红字，VERSION_CONFLICT → alert，NOT_FOUND → 跳列表
- `npx vue-tsc --noEmit` 通过

---

## [x] T5: EnabledGuardDialog 扩展 — 支持 event-type (R26-R29)

**涉及文件**：
- 修改 `frontend/src/components/EnabledGuardDialog.vue`

**做完了是什么样**：
- `EntityType` 联合类型新增 `'event-type'`
- `entityTypeLabel`：event-type → '事件类型'
- `refTargetLabel`：event-type → 'FSM 或 BT'
- `reasonText`（edit）：事件类型专属文案 '已启用的事件类型对 FSM/BT 可见，任意修改可能导致引用方看到不稳定的配置。请先停用，再进入编辑。'
- `onActOnce`：新增 `event-type` 分支，import `eventTypeApi` + `EVENT_TYPE_ERR`，调 detail → toggleEnabled
- 路由跳转：`/event-types/${id}/edit`
- 版本冲突码：`EVENT_TYPE_ERR.VERSION_CONFLICT`
- EventTypeList.vue 中将 T3 的占位 warning 替换为 `guardRef.open({ action, entityType: 'event-type', entity })`
- `npx vue-tsc --noEmit` 通过

---

## [x] T6: 集成验证 — vue-tsc + 全流程手动测试 (R31)

**涉及文件**：
- 无新增/修改文件（纯验证）

**做完了是什么样**：
- `npx vue-tsc --noEmit` 零错误
- 启动后端 + 前端 dev server，验证以下场景：
  - 侧栏菜单项可见、可点击、高亮正确
  - 列表页：加载数据、筛选、分页、空数据引导
  - 新建：完整提交流程、标识符校验、Global 范围联动、扩展字段
  - 编辑：detail 回填、标识符只读、版本冲突
  - Toggle：启用/停用、confirm 弹窗、版本冲突
  - Guard 弹窗：编辑/删除启用中事件类型、立即停用、跳转/刷新
  - 删除：确认弹窗、删除成功刷新
- 发现的问题在本任务内修复

---

## 依赖关系

```
T1 (API 模块)
 ↓
T2 (路由 + 侧栏)
 ↓
T3 (列表页)  ←── T1
 ↓
T4 (表单页)  ←── T1
 ↓
T5 (Guard 扩展) ←── T1, T3
 ↓
T6 (集成验证) ←── T1-T5
```

T3 和 T4 可以并行（都只依赖 T1），但 T5 依赖 T3（需要在列表页中替换占位代码）。
