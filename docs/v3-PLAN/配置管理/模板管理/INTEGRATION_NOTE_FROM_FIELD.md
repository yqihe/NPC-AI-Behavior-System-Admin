# 模板管理 — 来自字段管理的集成注意事项

> 字段管理模块已实现，模板管理开发时需注意以下集成点。

---

## 1. field_refs 引用关系维护

模板勾选字段时，必须在 `field_refs` 表中写入引用关系，同时维护字段的 `ref_count` 冗余计数。

### 创建/编辑模板（勾选字段变更时）

```
事务内操作：
1. FieldRefStore.RemoveByRef(tx, "template", templateName)  // 清除旧引用
2. 对每个新勾选的字段:
   FieldRefStore.Add(tx, {FieldName, RefType: "template", RefName: templateName})
3. 维护 ref_count:
   - 被移除的字段: FieldStore.DecrRefCount(tx, fieldName)
   - 被新增的字段: FieldStore.IncrRefCount(tx, fieldName)
```

### 删除模板

```
事务内操作：
1. affectedFields := FieldRefStore.RemoveByRef(tx, "template", templateName)
2. 对每个 affectedField:
   FieldStore.DecrRefCount(tx, fieldName)
```

### 关键约束

- `ref_count > 0` 的字段**禁止修改类型**（字段管理已实现此检查）
- `ref_count > 0` 的字段**禁止删除**（字段管理已实现此检查）
- 模板管理只需正确维护引用关系，约束由字段管理自动执行

### 字段启用/禁用状态约束

字段管理新增了三态生命周期（启用 / 禁用 / 已删除），模板管理必须遵守：

- **新增引用**：只能引用 `enabled=1` 的字段，否则字段管理返回 `40013 ErrFieldRefDisabled`
- **已有引用**：模板已引用的字段被禁用后，**引用关系保留不动**（"存量不变、增量阻断"）
- **模板展示**：列出模板字段时，建议标注已禁用字段的状态（灰色 / 警告图标），提示运营人员
- **导出过滤**：导出给游戏服务端时，需判断字段是否仍处于启用状态

---

## 2. 引用详情中的模板 label

字段管理的 `GetReferences` API 返回引用方信息时，模板引用的 label 当前是占位值（直接用 name）。

模板管理完成后需要：
- 提供 `TemplateStore.GetByNames(ctx, names)` 批量查 label 的方法
- 或者在字段 service 中注入模板 store，补全模板的中文标签

**代码位置**：`backend/internal/service/field.go` 第 331-337 行，有 `// TODO` 标记。

---

## 3. reference 类型字段展开

模板勾选 reference 类型字段时，需要递归展开为实际字段列表：
- reference 字段本身不产生数据
- 其引用的字段直接打平到模板的字段列表中
- 展开时自动去重

reference 字段的引用列表存储在 `properties.constraints` 的 JSON 中（待字段管理补全循环引用检测后确定具体格式）。
