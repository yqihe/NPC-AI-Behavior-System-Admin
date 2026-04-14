# code-conformance-fixes — 任务列表

## 状态

- [x] T1: EventTypeSchema handler 返回类型修复 + model.Empty 删除

---

## T1：EventTypeSchema handler 返回类型修复 + model.Empty 删除 (R1–R5)

**涉及文件**：
- `backend/internal/handler/event_type_schema.go`
- `backend/internal/model/response.go`

**做什么**：

1. **`event_type_schema.go`**：
   - `Update` 签名 `*model.Empty` → `*string`，返回值 `&model.Empty{}` → `shared.SuccessMsg("保存成功")`
   - `ToggleEnabled` 签名 `*model.Empty` → `*string`，返回值 `&model.Empty{}` → `shared.SuccessMsg("操作成功")`
   - `Delete` 签名 `*model.Empty` → `*model.DeleteResult`；方法体在调用 `h.schemaService.Delete` 前，先调 `h.schemaService.GetByID(ctx, req.ID)`，若 nil 返回 `ErrExtSchemaNotFound`；成功后返回 `&model.DeleteResult{ID: req.ID, Name: ets.FieldName, DisplayName: ets.FieldLabel}`

2. **`response.go`**：
   - 删除 `Empty struct{}` 定义及其注释（确认 `go build ./...` 无其他引用后再删）

**做完是什么样**：`go build ./...` 通过，无 `model.Empty` 残留引用；`go test ./...` 通过。

---

## 执行顺序

T1（唯一任务）
