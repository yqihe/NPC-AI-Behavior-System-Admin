# 字段管理 — 未实现功能清单

> 约束收紧检查和循环引用检测已实现（见 features.md 功能 10、11）。
> 批量操作已移除（不在 UI 暴露，需要时由后端人员直接操作）。
> 以下为剩余未实现功能。

---

## 依赖其他模块

### 未实现 1：关闭 BB Key 时行为树引用检查

**需求**：被行为树引用时禁止关闭 BB Key（expose_bb: true → false），提示"该 Key 正被 N 棵行为树使用，无法关闭"。

**现状**：错误码 `40008 ErrFieldBBKeyInUse` 已定义，但 `FieldService.Update` 中未实现 BB Key 变更检查。

**依赖**：行为树（BT）模块需提供 `IsBBKeyUsed(ctx, bbKey) (bool, error)` 接口。

**已通知**：见 `docs/v3-PLAN/行为管理/行为树/INTEGRATION_NOTE_FROM_FIELD.md`

---

## 跨模块集成点（已通知）

| 模块 | 集成点 | 通知文档 |
|------|--------|---------|
| 模板管理 | 勾选字段时维护 field_refs（用 field_id/ref_id BIGINT）+ ref_count；enabled 状态约束；引用详情补全模板 label；reference 展开 | `配置管理/模板管理/INTEGRATION_NOTE_FROM_FIELD.md` |
| 行为树 | 提供 BB Key 引用查询接口，供字段管理关闭 BB Key 时校验 | `行为管理/行为树/INTEGRATION_NOTE_FROM_FIELD.md` |

---

## 未实现 2：前端功能

以下前端功能在需求文档中已规划，但尚未开发：

| 功能 | 说明 |
|------|------|
| 字段列表页 | 表格展示、搜索筛选、分页、toggle 启用/停用开关、操作按钮（编辑/删除） |
| 新建/编辑页 | 固定区 + 动态区表单、SchemaForm 渲染、约束配置区、表单填充右侧空间 |
| 删除确认弹窗 | 未停用时：amber 警告+禁用删除按钮；已停用无引用：绿色提示；已停用有引用：红色警告+引用列表 |
| 启用/停用确认弹窗 | 启用确认（绿色）、停用确认（amber，提示已有引用不受影响） |
| 字段名实时校验 | 输入框失焦触发 check-name 接口、三色状态反馈 |
| 引用详情弹窗 | 点击引用数弹出、分类展示模板引用/字段引用 |
| 动态表单渲染 | dictionaries(field_properties) 驱动、按 sort_order 渲染 |
| 约束配置区 | 按字段类型动态展示约束项（integer/float/string/select/reference） |

---

## 延后功能（毕设后）

| 功能 | 说明 |
|------|------|
| 字段导入/导出 | CSV/Excel 批量导入导出 |
| 列头排序 | 点击列头切换排序方式 |
| 字段克隆/复制 | 待定 |
