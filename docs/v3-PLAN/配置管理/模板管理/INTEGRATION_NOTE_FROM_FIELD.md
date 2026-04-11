# 模板管理 — 来自字段管理的集成注意事项（历史存档）

> **⚠ 本文档已历史存档**，不再作为开发参考。模板管理的跨模块集成在 V3 实现期已全部落地，与字段管理之间的所有约定都在 service/handler 层通过明确的接口固化。

---

## 历史背景

V3 重写期间，字段管理模块先落地（2026-Q1），随后模板管理进入实现期（2026-Q2）。为了让两个模块解耦地并行开发，当时写下了一份「集成注意事项」说明模板管理开发时必须遵守的跨模块约定（field_refs 维护、enabled 约束、reference 展开等）。

模板管理完成后，这些约定都已通过代码落实，不再需要 prose 文档作为开发提醒。

---

## 当前真实文档来源

| 关注点 | 当前权威文档 |
|---|---|
| 字段管理对外接口清单（`ValidateFieldsForTemplate` / `AttachToTemplateTx` / `DetachFromTemplateTx` / `GetByIDsLite` / `InvalidateDetails`） | `../字段管理/backend.md` 「跨模块对外接口」章节 + `../字段管理/features.md` 功能 12 |
| 模板侧跨模块事务编排完整步骤（Create / Update / Delete 的 tx 流程） | `backend.md` 「跨模块事务编排（Handler 层）」章节 |
| 字段 `ref_count` 维护语义 | `features.md` 功能 2 / 功能 4 / 功能 5 的「后端跨模块事务流程」 |
| enabled 状态约束（停用字段的"存量不动、增量拦截"）| `features.md` 功能 4 「字段启用/停用状态约束」+ 「与字段管理的集成回顾」第 3 条 |
| reference 类型字段禁嵌套 + 模板扁平化约束 | `features.md` 功能 8 + `features.md` 「与字段管理的集成回顾」第 7 条 + 错误码 41012 |
| 引用详情补模板 label 的跨模块路径 | `features.md` 功能 11 「跨模块对外接口」+ `../字段管理/features.md` 功能 7 |
| 错误码归属约定（41005 / 41006 / 41012 归在模板段位）| `features.md` 「错误码」表 + 「与字段管理的集成回顾」第 6 条 + `backend.md` 错误码抛出层 |
| 分层硬规则（Service 之间零依赖，跨模块由 Handler 编排）| `../../../development/dev-rules.md` 「分层职责」章节 |

---

## 当初的三条核心约定 + 落地验证

| 当初约定 | 实际落地 |
|---|---|
| 创建 / 编辑模板时事务内维护 `field_refs` 与 `fields.ref_count` | `TemplateHandler.Create / Update / Delete` 通过 `h.db.BeginTxx` 开事务，传 `*sqlx.Tx` 给 `templateService.*Tx` + `fieldService.Attach/Detach*Tx`，commit 后分别清两方缓存 |
| 字段引用详情的 template label 补全 | `FieldHandler.GetReferences` 调 `templateService.GetByIDsLite(templateIDs)` 批量补 label，`FieldService.GetReferences` 内部只填 RefID 留 Label 空 |
| reference 类型字段的展开语义 | `FieldService.ValidateFieldsForTemplate` 直接拒绝 `f.Type == FieldTypeReference` → 41012，模板 `req.fields` 永远只含 leaf ID；前端 `TemplateRefPopover` 负责把 reference 字段 → 子字段 ID 展平后去重合并到 `selectedIds` |

---

## 历史遗留 TODO 已全部清零

- ✅ `FieldStore.GetByNames / IncrRefCount / DecrRefCount` → 已重构为 ID 版本（`GetByIDs / IncrRefCountTx / DecrRefCountTx`）
- ✅ `field_refs` schema 用 BIGINT 代替 VARCHAR（见 `../字段管理/backend.md` 数据表定义）
- ✅ `FieldService.GetReferences` 中的 `// TODO 补模板 label` 已通过 handler 层跨模块调用消除
- ✅ reference 字段展开契约已由字段管理 feature 11 + 错误码 41016 (禁嵌套) / 41017 (refs 非空) + 40009 (循环检测) 明确固化

---

**维护约定**：如果未来字段管理 / 模板管理之间新增跨模块接口，**不要**再新建一份 INTEGRATION_NOTE 文档。直接：

1. 更新 `../字段管理/backend.md` 或 `backend.md` 的「跨模块对外接口」章节
2. 更新对应 features.md 的调用链说明
3. 在 `docs/development/dev-rules.md` 的「分层职责」补一条例外（如果有）

单一权威胜过散落 prose。
