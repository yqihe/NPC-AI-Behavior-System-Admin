# 字段管理 — 未实现功能

> **现状**：字段管理模块（后端 + 前端）已全部落地，`features.md` 覆盖 12 个功能。本文档只列出明确延后或依赖其他模块的点。

---

## 依赖其他模块

### BB Key 引用检查（错误码 `40008` 已预留）

**需求**：字段编辑时若关闭 `expose_bb`（`true → false`），需检查该 BB Key 是否被行为树引用；若被引用则拒绝关闭，提示「该 Key 正被 N 棵行为树使用，无法关闭」。

**现状**：

- `errcode/codes.go` 中 `ErrFieldBBKeyInUse = 40008` 已定义
- `FieldService.Update` 中**没有**实际的 BB Key 变更检查逻辑（空洞预留）
- 前端 `FieldForm.vue` 没有为 40008 写 catch 分支（依赖通用拦截器默认 toast）

**依赖**：行为树（BT）模块需提供以下对外接口：

```go
// BT Service 或跨模块 helper
func IsBBKeyUsed(ctx context.Context, bbKey string) (bool, error)
```

**接入点**：`FieldService.Update` 在拿到 `old` 与 `req` 后，如果 `old.Properties.ExposeBB == true && req.Properties.ExposeBB == false`，调用 `btService.IsBBKeyUsed(old.Name)`；返回 `true` 则 `return 40008`。

**跨模块通知**：`行为管理/行为树/INTEGRATION_NOTE_FROM_FIELD.md`（当 BT 模块进入实现期时查此文档）。

---

## 毕设后延后功能

见 memory `project_deferred_features.md`。字段管理相关：

| 功能 | 说明 |
|---|---|
| 字段导入 / 导出 | CSV / Excel 批量导入导出，便于运营跨项目迁移配置 |
| 列头排序 | 列表页点击列头切换 ASC / DESC 排序（目前固定 id DESC） |
| 字段克隆 / 复制 | 基于现有字段快速创建同类型同约束的新字段 |
| 字段批量操作 | 批量删除、批量改分类、批量启用停用 |

**延后原因**：毕设阶段数据量小（< 50 个字段），这些功能不在核心演示路径上；企业级上线前再按需补齐。

---

## 已在其他位置处理的"曾经未实现"

以下项在早期版本是 `unimplemented.md` 的内容，现在已全部落地：

- ✅ reference 类型字段（40016 禁嵌套 / 40017 refs 非空 / 40009 循环检测 DFS）
- ✅ 启用/停用切换（40015 编辑限制）
- ✅ 被引用字段的约束收紧检查（40007 按类型分支）
- ✅ 引用详情接口（跨模块补 template label）
- ✅ 字段管理前端全套（FieldList + FieldForm + 5 个约束面板 + 引用详情弹窗 + EnabledGuardDialog 集成）
- ✅ 前端 reference 下拉过滤（双重防御，40016 兜底）
- ✅ 前端 40016 / 40017 定向提示
- ✅ 与模板管理的跨模块集成（`ValidateFieldsForTemplate` / `AttachToTemplateTx` / `DetachFromTemplateTx` / `GetByIDsLite` / `InvalidateDetails`）

→ 跨模块已落地的集成点详见 `../模板管理/features.md` 功能 11 「跨模块对外接口」。
