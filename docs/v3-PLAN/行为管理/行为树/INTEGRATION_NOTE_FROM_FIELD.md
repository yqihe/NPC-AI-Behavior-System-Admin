# 行为树 — 来自字段管理的集成注意事项

> 字段管理模块已实现，行为树模块开发时需注意以下集成点。

---

## BB Key 引用检查

### 背景

字段定义中有 `expose_bb` 属性（properties JSON 中），标记该字段是否暴露为行为树黑板 Key。

当运营人员编辑字段想关闭 BB Key（`expose_bb: true → false`）时，如果该 Key 正在被行为树引用，必须阻止此操作。

### 行为树模块需要提供的能力

```go
// 查询某个 BB Key 是否被任何行为树引用
// key 的格式即字段的 name（如 "max_hp"、"move_speed"）
func (s *BTStore) IsBBKeyUsed(ctx context.Context, bbKey string) (bool, error)

// 可选：返回引用该 Key 的行为树名称列表（用于前端提示）
func (s *BTStore) GetBBKeyUsages(ctx context.Context, bbKey string) ([]string, error)
```

### 字段管理侧的对接

行为树模块实现上述接口后，字段管理的 `FieldService.Update` 需要补充：

```go
// 在 Update 方法中，检查 BB Key 关闭
if oldExposeBB && !newExposeBB {
    used, _ := btStore.IsBBKeyUsed(ctx, name)
    if used {
        return errcode.New(errcode.ErrFieldBBKeyInUse) // 40008
    }
}
```

错误码 `40008 ErrFieldBBKeyInUse` 已定义，消息为"该 Key 正被行为树使用，无法关闭"。

### BB Key 来源

ADMIN 的 BB Key 来源有两个（CLAUDE.md 中已明确）：
1. **字段标识**：字段 `expose_bb=true` 的 name
2. **运行时 Key 表**：独立管理（非字段系统）

行为树模块引用 BB Key 时应同时考虑这两个来源。
