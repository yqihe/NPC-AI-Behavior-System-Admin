# 事件类型管理 — 任务拆解（后端）

> 对应需求：[requirements.md](requirements.md)
> 对应设计：[design.md](design.md)
>
> **范围**：仅后端 T1-T18。前端另起 spec。
> **状态**：全部完成 ✅

---

## 第一阶段：后端基础设施

### [x] T1：DDL 迁移脚本 (R7, R9, R17)

- `backend/migrations/004_create_event_types.sql`（新建）
- `backend/migrations/005_create_event_type_schema.sql`（新建）

### [x] T2：Model 定义 (R1, R4, R16)

- `backend/internal/model/event_type.go`（新建）
- `backend/internal/model/event_type_schema.go`（新建）

### [x] T3：错误码定义 (R3)

- `backend/internal/errcode/codes.go`（改动：42001-42031）

### [x] T4：Redis Key 定义 (R22-R24)

- `backend/internal/store/redis/keys.go`（改动）

### [x] T5：配置项新增 (R22-R23)

- `backend/internal/config/config.go`（改动）
- `backend/config.yaml` + `config.docker.yaml`（改动）

---

## 第二阶段：后端 Store 层

### [x] T6：EventTypeStore (R1, R4-R11)

- `backend/internal/store/mysql/event_type.go`（新建）

### [x] T7：EventTypeSchemaStore (R16-R20)

- `backend/internal/store/mysql/event_type_schema.go`（新建）

### [x] T8：EventTypeCache — Redis (R22-R26)

- `backend/internal/store/redis/event_type_cache.go`（新建）

### [x] T9：EventTypeSchemaCache — 内存 (R27)

- `backend/internal/cache/event_type_schema_cache.go`（新建）

---

## 第三阶段：后端 Service 层

### [x] T10：constraint 包抽离 (R14-R15)

- `backend/internal/service/constraint/validate.go`（新建）
- `backend/internal/service/field.go`（改动）

### [x] T11：EventTypeService (R1, R4-R13)

- `backend/internal/service/event_type.go`（新建）

### [x] T12：EventTypeSchemaService (R16-R21)

- `backend/internal/service/event_type_schema.go`（新建）

---

## 第四阶段：后端 Handler + Router

### [x] T13：EventTypeHandler — 7 个接口 (R1-R3, R8, R11-R13)

- `backend/internal/handler/event_type.go`（新建）

### [x] T14：EventTypeSchemaHandler — 5 个接口 (R16-R19)

- `backend/internal/handler/event_type_schema.go`（新建）

### [x] T15：ExportHandler — 导出 API (R28-R31)

- `backend/internal/handler/export.go`（新建）

### [x] T16：路由注册 + main.go 装配 (R1, R16, R27-R28)

- `backend/internal/router/router.go`（改动）
- `backend/cmd/admin/main.go`（改动）

---

## 第五阶段：后端测试

### [x] T17：集成测试 (R1-R31)

- `tests/integration_test.sh`（合并进统一脚本）

### [x] T18：constraint 包验证 (R14-R15)

- 通过集成测试中的 CJK 攻击测试 + 约束校验 case 覆盖

---

## 任务依赖图

```
T1-T5 (基础设施)
    ↓
T6-T9 (Store)
    ↓
T10-T12 (Service)
    ↓
T13-T16 (Handler + Router)
    ↓
T17-T18 (测试)
```

---

## 总结

| 阶段 | 任务数 | 状态 |
|---|---|---|
| 后端基础设施 | T1-T5 | ✅ |
| 后端 Store | T6-T9 | ✅ |
| 后端 Service | T10-T12 | ✅ |
| 后端 Handler + Router | T13-T16 | ✅ |
| 后端测试 | T17-T18 | ✅ |
| **合计** | **18 个任务** | **全部完成** |
