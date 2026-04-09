# 字段管理 — 未实现功能清单

> 后端 API 全部实现（见 features.md）。
> 前端已全部实现（Vue 3 + TypeScript + Element Plus）。
> 批量操作已移除（不在 UI 暴露）。
> 以下为剩余未实现功能。

---

## 依赖其他模块

### 未实现 1：关闭 BB Key 时行为树引用检查

**需求**：被行为树引用时禁止关闭 BB Key（expose_bb: true → false），提示"该 Key 正被 N 棵行为树使用，无法关闭"。

**现状**：错误码 `40008 ErrFieldBBKeyInUse` 已定义，但 `FieldService.Update` 中未实现 BB Key 变更检查。前端已预留 40008 错误码处理（编辑页黄色警告）。

**依赖**：行为树（BT）模块需提供 `IsBBKeyUsed(ctx, bbKey) (bool, error)` 接口。

**已通知**：见 `docs/v3-PLAN/行为管理/行为树/INTEGRATION_NOTE_FROM_FIELD.md`

---

## 跨模块集成点（已通知）

| 模块 | 集成点 | 通知文档 |
|------|--------|---------|
| 模板管理 | 勾选字段时维护 field_refs（用 field_id/ref_id BIGINT）+ ref_count；enabled 状态约束；引用详情补全模板 label；reference 展开 | `配置管理/模板管理/INTEGRATION_NOTE_FROM_FIELD.md` |
| 行为树 | 提供 BB Key 引用查询接口，供字段管理关闭 BB Key 时校验 | `行为管理/行为树/INTEGRATION_NOTE_FROM_FIELD.md` |

---

## 延后功能（毕设后）

| 功能 | 说明 |
|------|------|
| 字段导入/导出 | CSV/Excel 批量导入导出 |
| 列头排序 | 点击列头切换排序方式 |
| 字段克隆/复制 | 待定 |
