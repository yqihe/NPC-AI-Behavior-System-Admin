# 扩展字段 Schema 管理前端 — 任务拆解

## T1: API 层补充 + 错误码常量 (R3, R7, R8, R12)

**涉及文件：**
- `frontend/src/api/eventTypes.ts`

**做什么：**
1. 新增 `EXT_SCHEMA_ERR` 错误码常量（42020-42031）
2. 新增 `CreateExtSchemaRequest` / `UpdateExtSchemaRequest` 接口定义
3. 在 `eventTypeApi` 对象中补充 5 个 schema CRUD 函数：`schemaList`、`schemaCreate`、`schemaUpdate`、`schemaDelete`、`schemaToggleEnabled`
4. 保留已有的 `schemaListEnabled()` 不变

**做完了是什么样：**
- `eventTypeApi.schemaList()` 可以调用后端 `/event-type-schema/list`
- 其余 4 个 API 函数均可正确调用对应后端接口
- `EXT_SCHEMA_ERR` 常量可被 List/Form 页面引用
- `npx vue-tsc --noEmit` 通过

---

## T2: 路由注册 + 侧边栏菜单 (R9, R10)

**涉及文件：**
- `frontend/src/router/index.ts`
- `frontend/src/components/AppLayout.vue`

**做什么：**
1. 在 router 中新增 4 条路由：list / create / view / edit，遵循 route meta flags 模式
2. 在 AppLayout.vue 侧边栏"配置管理"分组内，"事件类型"下方新增"扩展字段"菜单项（`index="/event-type-schemas"`）
3. 导入合适的图标（如 `Tickets` 或 `Collection`）

**做完了是什么样：**
- 浏览器访问 `/event-type-schemas` 能加载页面（即使组件尚未创建，路由不报错即可先用空组件占位——但因为 T3 紧随其后，可以先创建再注册）
- 侧边栏显示"扩展字段"菜单项，点击跳转正确
- 菜单高亮在 `/event-type-schemas` 路径下正确

---

## T3: 列表页 EventTypeSchemaList.vue (R1, R2, R7, R8)

**涉及文件：**
- `frontend/src/views/EventTypeSchemaList.vue`

**做什么：**
1. 创建列表页，遵循 FieldList.vue 模式
2. 表格列：ID、字段标识（field_name）、中文名（field_label）、类型（field_type，el-tag）、排序（sort_order）、启用状态（el-switch）、创建时间、操作
3. 筛选栏：仅启用状态筛选（el-select：全部/启用/禁用）— 数据量 < 100，无需文本搜索和分页
4. 操作列：查看 / 编辑 / 删除
5. 启用/禁用 toggle：ElMessageBox.confirm 确认 → 调用 `schemaToggleEnabled`（list 返回 version 可直接用）→ 刷新
6. 编辑拦截：启用状态走 EnabledGuardDialog
7. 删除拦截：启用状态走 EnabledGuardDialog；已禁用 → 确认框 → 调用 `schemaDelete` → 刷新；后端返回 42027 则提示需先禁用
8. 空数据：`el-empty` + "新建扩展字段"引导按钮
9. 无分页（后端不分页，全量返回）

**做完了是什么样：**
- 列表加载并显示所有 schema 数据
- 筛选切换后端生效
- toggle / 编辑 / 删除操作正常
- 空数据有引导

---

## T4: EnabledGuardDialog 扩展 (R8)

**涉及文件：**
- `frontend/src/components/EnabledGuardDialog.vue`

**做什么：**
1. `EntityType` 类型新增 `'event-type-schema'`
2. `entityTypeLabel` 新增映射："扩展字段"
3. `refTargetLabel` 新增映射：扩展字段无引用关系，可写"其他配置"
4. `reasonText` 新增 `event-type-schema` 分支文案
5. `onActOnce` 新增 `event-type-schema` 分支：调用 `eventTypeApi.schemaList()` 按 ID 找到目标项获取 version → 调用 `schemaToggleEnabled`
6. 编辑跳转路径：`/event-type-schemas/${id}/edit`
7. 版本冲突码：`EXT_SCHEMA_ERR.VERSION_CONFLICT`

**做完了是什么样：**
- 列表页点击启用状态的"编辑/删除"时弹出正确的守卫弹窗
- "立即停用"按钮正常工作
- 停用后编辑场景跳转到编辑页，删除场景刷新列表

---

## T5: 表单页 EventTypeSchemaForm.vue (R3, R4, R5, R6, R12)

**涉及文件：**
- `frontend/src/views/EventTypeSchemaForm.vue`

**做什么：**
1. 创建表单页，遵循 FieldForm.vue 模式（header + card + form）
2. 三种模式：isCreate / isView / edit（通过 route meta 判断）
3. 表单字段：
   - field_name：创建时可编辑 + blur 格式校验；编辑/查看时禁用 + Lock 图标
   - field_label：必填
   - field_type：创建时 el-select（int/float/string/bool/select 五选项）；编辑时禁用
   - 约束配置：按 field_type 动态渲染 FieldConstraintInteger / FieldConstraintString / FieldConstraintSelect（bool 显示"无需约束"）
   - default_value：按 field_type 动态渲染输入控件
   - sort_order：el-input-number
4. 数据加载（编辑/查看）：调用 `schemaList()` 全量获取 → 按路由 ID 找到目标项 → 填充 form
5. 提交（创建）：调用 `schemaCreate` → 成功 toast + 跳转列表
6. 提交（编辑）：调用 `schemaUpdate`（带 version）→ 成功 toast + 跳转列表
7. 错误处理：42020 → nameStatus='taken'；42030 → 版本冲突弹窗；42031 → 提示需先禁用
8. 查看模式：form disabled，无提交按钮

**做完了是什么样：**
- 创建表单填写完整信息后提交成功
- 编辑表单 field_name/field_type 灰显不可改，其他字段可编辑
- 查看模式全部只读
- 约束面板按类型切换正常
- 错误处理到位

---

## T6: 验证 + 类型检查 (R11)

**涉及文件：**
- 无新增文件，对前 5 个 task 的产出做整体验证

**做什么：**
1. 运行 `npx vue-tsc --noEmit` 确认类型检查通过
2. 启动开发服务器，手工验证全部 10 个测试场景（design.md 测试策略部分）
3. 修复发现的问题

**做完了是什么样：**
- 类型检查零错误
- 10 个手工测试场景全部通过
- 验收标准 R1-R12 全部达成
