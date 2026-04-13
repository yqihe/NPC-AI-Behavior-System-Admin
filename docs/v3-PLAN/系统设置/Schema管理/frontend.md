# 事件扩展字段 Schema 管理 — 前端设计

> **实现状态**：已完成。
> 详细前端设计（含事件类型主页面）见 `docs/v3-PLAN/行为管理/事件类型/frontend.md`。

---

## 页面结构

| 路径 | 组件 | 说明 |
|---|---|---|
| `/event-type-schemas` | EventTypeSchemaList.vue | 列表页 |
| `/event-type-schemas/create` | EventTypeSchemaForm.vue | 新建页 |
| `/event-type-schemas/:id/view` | EventTypeSchemaForm.vue | 查看页（只读） |
| `/event-type-schemas/:id/edit` | EventTypeSchemaForm.vue | 编辑页 |

侧边栏菜单：「配置管理」→「事件扩展字段」。

## 列表页

- 无分页（数据量 < 100），默认按 ID 倒序排列
- 排序切换：筛选栏右侧有 `el-button-group`（`ID 倒序` / `排序正序`），切换后前端本地重新排序（`sort_order ASC, id ASC`），不重新请求
- 筛选：启用状态（全部/启用/禁用）
- 表格列：ID / 字段标识 / 中文标签 / 类型（中文 tag）/ 排序 / 启用（switch）/ 创建时间 / 操作
- 操作：查看 / 编辑 / 删除
- 启用态编辑/删除走 EnabledGuardDialog
- 空数据走 `el-empty` + 引导按钮

## 表单页

- 三模式：create / edit / view（route meta 区分）
- field_name：创建可编辑 + blur 格式校验；编辑/查看禁用 + Lock 图标
- field_type：创建可选（int/float/string/bool/select 五选项，中文括注）；编辑禁用
- 约束配置：复用 FieldConstraintInteger / FieldConstraintString / FieldConstraintSelect
- 默认值：按 field_type 动态渲染
- sort_order：el-input-number

## API 调用

无 detail 接口，编辑/查看通过 `schemaList()` 全量获取后按 ID 查找。无 checkName 接口，标识符重复在提交时通过 42020 错误码处理。

## 错误码处理

| 错误码 | UI 反馈 |
|---|---|
| 42020 标识已存在 | nameStatus='taken' + form 内联红字 |
| 42021 标识格式非法 | nameStatus='taken' + form 内联红字 |
| 42022 不存在 | `ElMessage.error` + 跳转列表 |
| 42027 删除须先禁用 | `ElMessage.warning` |
| 42030 版本冲突 | `ElMessageBox.alert` 提示刷新 |
| 42031 编辑须先禁用 | `ElMessage.warning` |
| 42024 类型非法 | `ElMessage.error` 提示字段类型不合法 |
| 42025 约束非法 | `ElMessage.error` 提示约束参数不合法 |
| 42026 默认值非法 | `ElMessage.error` 提示默认值不符合约束条件 |
| 其他校验错误 | 拦截器 toast |

---

## 关键实现细节

### Toggle 预取版本号

`EventTypeSchemaList.vue` 的 `handleToggle` 在调用 `schemaToggleEnabled` 前，先调用 `schemaList()` 重新获取最新列表，从中取出目标 Schema 的当前 `version`。这避免了列表页缓存的 version 与实际不一致导致的乐观锁冲突（race condition）。

### 约束组件 `validate()` 校验

`EventTypeSchemaForm.vue` 持有 `constraintRef`（模板引用），提交前调用 `constraintRef.value?.validate()` 进行约束级前端校验。同时表单提交前额外校验默认值是否在 min/max 约束范围内，校验失败直接 `ElMessage.error` 提示，不提交后端。

### 约束组件 `disabled` prop

`FieldConstraintSelect` 接受 `disabled` prop，在查看模式下传入 `true`，禁用所有内部控件。
