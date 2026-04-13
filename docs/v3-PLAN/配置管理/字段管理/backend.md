# 字段管理 — 后端设计

## 目录结构

```
handler/field.go          请求校验 + 跨模块编排（GetReferences 补模板/FSM label）
service/field.go          业务逻辑 + Cache-Aside + 引用保护 + 循环引用检测 + 跨模块对外方法
store/mysql/field.go      fields 表 CRUD + 覆盖索引 + 乐观锁
store/mysql/field_ref.go  field_refs 关联表 Add/Remove/RemoveBySource/HasRefs/HasRefsTx/GetByFieldID
store/redis/field_cache   Detail/List 缓存 + 分布式锁
model/field.go            Field/FieldLite/FieldListItem + DTO
errcode/codes.go          40001-40017
```

## 数据库

**fields 表**：id, name(uk), label, type, category, properties(JSON), enabled, version, deleted, created_at, updated_at

覆盖索引：`idx_list(deleted, id, name, label, type, category, enabled, created_at)`

**field_refs 表**：`PRIMARY KEY(field_id, ref_type, ref_id)`，`INDEX idx_ref(ref_type, ref_id)`

ref_type 取值：`template` / `field` / `fsm`

## 缓存策略

- **Detail**：Redis Cache-Aside + 分布式锁防击穿 + 空标记防穿透，TTL 5min+30s 抖动
- **List**：Redis 版本号方案（`INCR version` 使旧 key 失效），TTL 1min+10s 抖动
- **has_refs**：不缓存，每次实时查 `field_refs`（引用关系随其他模块操作变化）

## 核心逻辑

### 编辑保护（service.Update）

1. 必须先禁用（`enabled=false`）
2. `fieldRefStore.HasRefs(ctx, id)` → 有引用时：
   - 类型不可改（40006）
   - `util.CheckConstraintTightened` 约束只能放宽（40007）
3. reference 类型校验：非空 + 目标存在 + 新增 ref 必须启用 + 非 reference + 无循环

### expose_bb 取消保护（service.Update）

旧 `expose_bb=true` → 新 `expose_bb=false` 时，检查 `field_refs WHERE ref_type='fsm'`，有引用返回 40008。

### 删除保护（service.Delete）

1. 必须先禁用
2. 事务内 `HasRefsTx(FOR SHARE)` 防 TOCTOU → 有引用返回 40005
3. reference 类型字段删除时 `RemoveBySource` 清理它对子字段的引用

### 跨模块对外方法

| 方法 | 调用方 |
|------|--------|
| `ValidateFieldsForTemplate(ctx, fieldIDs)` | 模板 handler |
| `AttachToTemplateTx(ctx, tx, tplID, fieldIDs)` | 模板 handler |
| `DetachFromTemplateTx(ctx, tx, tplID, fieldIDs)` | 模板 handler |
| `GetByIDsLite(ctx, fieldIDs)` | 模板 handler |
| `InvalidateDetails(ctx, fieldIDs)` | 模板/FSM handler |
| `SyncFsmBBKeyRefs(ctx, tx, fsmID, oldKeys, newKeys)` | FSM handler |
| `CleanFsmBBKeyRefs(ctx, tx, fsmID)` | FSM handler |

## 错误码

40001 NameExists / 40002 NameInvalid / 40003 TypeNotFound / 40004 CategoryNotFound / 40005 RefDelete / 40006 RefChangeType / 40007 RefTighten / 40008 BBKeyInUse / 40009 CyclicRef / 40010 VersionConflict / 40011 NotFound / 40012 DeleteNotDisabled / 40013 RefDisabled / 40014 RefNotFound / 40015 EditNotDisabled / 40016 RefNested / 40017 RefEmpty
