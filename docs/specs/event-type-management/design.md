# 事件类型管理 — 设计方案（后端）

> 对应需求：[requirements.md](requirements.md)
> 对应实现文档：[backend.md](../../v3-PLAN/行为管理/事件类型/backend.md)
>
> **范围**：仅后端。前端另起 spec。

---

## 方案描述

### 存储架构

**MySQL 单存储**，不使用 MongoDB。两张新表 DDL 见 `migrations/004_create_event_types.sql` 和 `005_create_event_type_schema.sql`。

### 接口定义

**事件类型 CRUD（7 个）：**

| 方法 | 路径 | Handler |
|---|---|---|
| POST | `/api/v1/event-types/list` | `EventTypeHandler.List` |
| POST | `/api/v1/event-types/create` | `EventTypeHandler.Create` |
| POST | `/api/v1/event-types/detail` | `EventTypeHandler.Get` |
| POST | `/api/v1/event-types/update` | `EventTypeHandler.Update` |
| POST | `/api/v1/event-types/delete` | `EventTypeHandler.Delete` |
| POST | `/api/v1/event-types/check-name` | `EventTypeHandler.CheckName` |
| POST | `/api/v1/event-types/toggle-enabled` | `EventTypeHandler.ToggleEnabled` |

**扩展字段 Schema（5 个）：**

| 方法 | 路径 | Handler |
|---|---|---|
| POST | `/api/v1/event-type-schema/list` | `EventTypeSchemaHandler.List` |
| POST | `/api/v1/event-type-schema/create` | `EventTypeSchemaHandler.Create` |
| POST | `/api/v1/event-type-schema/update` | `EventTypeSchemaHandler.Update` |
| POST | `/api/v1/event-type-schema/toggle-enabled` | `EventTypeSchemaHandler.ToggleEnabled` |
| POST | `/api/v1/event-type-schema/delete` | `EventTypeSchemaHandler.Delete` |

**导出 API（1 个）：**

| 方法 | 路径 | Handler |
|---|---|---|
| GET | `/api/configs/event_types` | `ExportHandler.EventTypes` |

### config_json 拼装规则

Service 层合并系统字段 + 校验后的扩展字段 → `json.Marshal(configMap)` → 存入 `config_json` 列。导出时原样输出，不经过 Go struct 中转。

### constraint 包抽离

`service/constraint/validate.go` 从 `service/field.go` 抽出值级校验辅助函数，提供 `ValidateValue` 和 `ValidateConstraintsSelf` 两个公共入口。字段管理的 `checkConstraintTightened` 保留原处。

### 缓存策略

- **event_types（Redis）**：详情 `event_types:detail:{id}` TTL 10min ± jitter，列表版本号失效，分布式锁防击穿，空标记防穿透
- **event_type_schema（内存）**：启动时 `Load`，写后同步 `Reload`

---

## 方案对比

**选用**：MySQL 单存储 + config_json 列。零同步问题、单事务原子性、和字段/模板同架构。

**拒绝**：MySQL + MongoDB 双写。复杂度远高于收益，HTTP 导出 API 对游戏服务端是黑盒。

---

## 红线检查

详见原始 design.md 的完整红线检查表。所有红线通过，无需修改红线本身。

关键确认：
- `default_severity` 允许小数（服务端 float64 兼容）
- 本期删除不做 FOR SHARE（无 event_type_refs），已在 features.md 声明

---

## 依赖方向

```
router.go → EventTypeHandler / EventTypeSchemaHandler / ExportHandler
                    ↓                       ↓
         EventTypeService      EventTypeSchemaService
           ↓        ↓                  ↓
   constraint/   EventTypeStore   EventTypeSchemaStore
   validate.go   (MySQL)          (MySQL)
                    ↓                  ↓
              EventTypeCache    EventTypeSchemaCache
              (Redis)           (内存)
```

关键约束：两个 Service 之间无横向依赖。Handler.Get 跨调两个 Service 拼装详情。

---

## 配置变更

`config.yaml` 新增 `event_type` 和 `event_type_schema` 段。详见 `config.go` 的 `EventTypeConfig` 和 `EventTypeSchemaConfig`。

---

## 测试策略

### 后端集成测试

统一在 `tests/integration_test.sh` 中（已合并）：
- 正向：CRUD 全流程 + Schema 全流程 + 带扩展字段创建/编辑/导出
- 错误路径：42001-42027 各错误码
- 攻击：name 特殊字符 / SQL 注入 / severity=0 / global+range=0 / 不存在扩展字段 key

---

## 实现状态

**已全部落地**。后端 T1-T18 全部完成。集成测试 315/315 通过。
