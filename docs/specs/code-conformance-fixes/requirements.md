# code-conformance-fixes — 需求分析

## 动机

项目在推进过程中积累了若干偏离 red-lines 的小缺陷。这些缺陷不影响功能正确性，但会在代码审查时引起疑问，也可能在未来扩展时造成混乱。本 spec 集中清理这批缺陷，一次性对齐规范，之后新增模块直接参照修复后的代码作为模板。

不修则：随着模块增多，"参考这个老模块的写法"会把偏差传播到新代码；审查新模块时也难以分辨哪些是刻意偏差、哪些是笔误。

---

## 优先级

**中**。不阻塞任何功能开发，但 FSM 状态字典等新模块上线前修完更好（避免用脏模板）。

---

## 预期效果

### 场景 1：EventTypeSchema handler 响应类型对齐

修复前，`EventTypeSchemaHandler` 的三个写操作返回 `*model.Empty`：

```go
func (h *EventTypeSchemaHandler) Update(...) (*model.Empty, error)
func (h *EventTypeSchemaHandler) Delete(...) (*model.Empty, error)
func (h *EventTypeSchemaHandler) ToggleEnabled(...) (*model.Empty, error)
```

修复后，与全平台所有模块对齐：

```go
func (h *EventTypeSchemaHandler) Update(...) (*string, error)           // shared.SuccessMsg("保存成功")
func (h *EventTypeSchemaHandler) Delete(...) (*model.DeleteResult, error)
func (h *EventTypeSchemaHandler) ToggleEnabled(...) (*string, error)    // shared.SuccessMsg("操作成功")
```

`model.Empty` 在整个 handler 层只有这三处使用，修复后可以删除该类型。

---

## 依赖分析

### 依赖的已完成工作

- `handler/shared/validate.go`：`SuccessMsg` 函数已存在
- `model.DeleteResult`：已存在
- `EventTypeSchemaService.Delete`：已返回 `errcode.ErrExtSchemaNotFound` 等，handler 层直接透传即可

### 谁依赖这个需求

- **fsm-state-dict-backend** Phase 3：新模块 handler 以修复后的模式为参照
- 无运行时依赖（响应 `data` 字段变化：`{}` → `"保存成功"` / `{id,name,...}`；前端不消费这些字段，只检查 `code === 0`）

---

## 改动范围

| 文件 | 改动 |
|---|---|
| `backend/internal/handler/event_type_schema.go` | `Update/Delete/ToggleEnabled` 返回类型 + 返回值 |
| `backend/internal/model/common.go` | 删除 `Empty` 结构体（确认无其他引用后删除） |

预估：2 个文件，净改动 < 15 行。

---

## 扩展轴检查

- **新增配置类型**：正面影响。修复后 EventTypeSchema handler 成为标准模板，新模块直接复用。
- **新增表单字段**：不涉及。

---

## 验收标准

**R1**：`EventTypeSchemaHandler.Update` 签名改为 `(*string, error)`，返回 `shared.SuccessMsg("保存成功")`。

**R2**：`EventTypeSchemaHandler.Delete` 签名改为 `(*model.DeleteResult, error)`，返回 `&model.DeleteResult{ID: id, Name: ets.FieldName, DisplayName: ets.FieldLabel}`。

**R3**：`EventTypeSchemaHandler.ToggleEnabled` 签名改为 `(*string, error)`，返回 `shared.SuccessMsg("操作成功")`。

**R4**：`model.Empty` 结构体在整个后端无其他引用后删除。

**R5**：`go build ./...` 通过；`go test ./...` 通过。

验证方法：代码审查 + 编译。

---

## 不做什么

1. **不修改其他模块的 handler**——其他模块已经符合规范，不扩大范围
2. **不改 service/store 层**——纯 handler 层返回类型修正
3. **不修改前端**——前端不消费 `data` 字段，行为不变
4. **不清理其他 `model.Empty` 使用**（如果有的话）——只处理 handler 层
