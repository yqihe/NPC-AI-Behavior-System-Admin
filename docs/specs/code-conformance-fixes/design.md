# code-conformance-fixes — 设计方案

## 方案描述

### 核心思路

直接替换三个返回值类型，不改 service/store 层逻辑。Delete 需要提前调一次 `GetByID` 来构造 `DeleteResult`（service 内部也会调，但为了最小化改动，不改 service 签名）。

### 具体改动

#### Update

```go
// Before
func (h *EventTypeSchemaHandler) Update(...) (*model.Empty, error) {
    ...
    return &model.Empty{}, nil
}

// After
func (h *EventTypeSchemaHandler) Update(...) (*string, error) {
    ...
    return shared.SuccessMsg("保存成功"), nil
}
```

#### ToggleEnabled

```go
// Before
func (h *EventTypeSchemaHandler) ToggleEnabled(...) (*model.Empty, error) {
    ...
    return &model.Empty{}, nil
}

// After
func (h *EventTypeSchemaHandler) ToggleEnabled(...) (*string, error) {
    ...
    return shared.SuccessMsg("操作成功"), nil
}
```

#### Delete

Delete 需要在删除前获取 `field_name` / `field_label` 来填充 `DeleteResult`。最小改动：handler 层预先调 `schemaService.GetByID`（service 内部 Delete 也会调一次，接受这个轻微冗余）：

```go
// After
func (h *EventTypeSchemaHandler) Delete(ctx context.Context, req *model.IDRequest) (*model.DeleteResult, error) {
    slog.Debug("handler.event_type_schema.delete", "id", req.ID)

    if req.ID <= 0 {
        return nil, errcode.Newf(errcode.ErrBadRequest, "ID 必须 > 0")
    }

    // 预先获取用于构造 DeleteResult（service.Delete 内也有 getOrNotFound，轻微冗余但避免改 service 签名）
    ets, err := h.schemaService.GetByID(ctx, req.ID)
    if err != nil {
        return nil, err
    }
    if ets == nil {
        return nil, errcode.New(errcode.ErrExtSchemaNotFound)
    }

    if err := h.schemaService.Delete(ctx, req.ID); err != nil {
        return nil, err
    }
    return &model.DeleteResult{ID: req.ID, Name: ets.FieldName, DisplayName: ets.FieldLabel}, nil
}
```

#### model.Empty 删除

`model.Empty` 定义在 `backend/internal/model/response.go`，仅被 `event_type_schema.go` handler 使用。修复后删除该类型定义，`go build` 确认无其他引用。

---

## 方案对比

### 备选方案：修改 service.Delete 返回 `(*model.EventTypeSchema, error)`

```go
func (s *EventTypeSchemaService) Delete(ctx context.Context, id int64) (*model.EventTypeSchema, error) {
    ets, err := s.getOrNotFound(ctx, id)
    ...
    return ets, nil  // 返回被删对象供 handler 构造 DeleteResult
}
```

**不选原因**：service 层接口变更会影响 service 测试和文档，改动范围更大。requirements 明确"不改 service/store 层"。handler 内多一次主键 GetByID 是可接受的轻微冗余（走主键索引，< 1ms）。

---

## 红线检查

| 红线 | 检查结果 |
|---|---|
| §10：handler Update → `*string`，Delete → `*DeleteResult`，Toggle → `*string` | ✓ 本次修复的目标 |
| §10：service.ToggleEnabled 接收 `*ToggleEnabledRequest` | ✓ 不涉及，已符合 |
| Go：nil slice/map 初始化 | ✓ 不涉及 |
| Go：error 不忽略 | ✓ 不涉及 |

---

## 扩展性影响

- **新增配置类型**：正面。`EventTypeSchema` handler 修复后成为完整的参照模板，无偏差模式可供新模块参考。

---

## 依赖方向

```
handler/event_type_schema.go
  └── service/event_type_schema.go  (GetByID + Delete，不改签名)
  └── model.DeleteResult            (已存在)
  └── handler/shared.SuccessMsg     (已存在)
```

---

## 陷阱检查

### Go
- `GetByID` 返回 `(nil, nil)` 表示不存在；handler 需判断 `ets == nil` 返回 `ErrExtSchemaNotFound`。✓
- `DeleteResult.Name` 对应 `ets.FieldName`，`DeleteResult.DisplayName` 对应 `ets.FieldLabel`，字段名不同需显式映射。✓
- 删除 `model.Empty` 后，`go build ./...` 是唯一验证手段——类型检查比 grep 可靠。✓

---

## 配置变更

无。

---

## 测试策略

`go build ./...` 通过即可。无纯计算逻辑变化，无需新增单元测试。
