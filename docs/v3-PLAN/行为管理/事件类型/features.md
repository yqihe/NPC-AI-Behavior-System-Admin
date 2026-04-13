# 事件类型管理 — 功能定义

> **实现状态**：已完成（后端 + 前端）。
> **路径前缀**：`/api/v1/event-types/*`
> **导出接口**：`GET /api/configs/event_types`

---

## 1. 概述

事件类型是"游戏世界里会发生什么事"的元数据登记。策划在 ADMIN 平台通过 UI 配置事件标识、系统字段和扩展字段，配置存入 MySQL（`config_json` 列），游戏服务端启动时通过导出 API 一次性拉取。

---

## 2. 状态模型

```
创建 → 停用态（enabled=0）
         ↓ toggle-enabled
       启用态（enabled=1）
         ↓ toggle-enabled
       停用态（enabled=0）
         ↓ delete
       软删除（deleted=1）
```

| 状态 | 事件类型页看到 | FSM/BT 条件编辑器看到 | 能被新引用 | 已有引用 |
|---|---|---|---|---|
| 启用 | 可见，正常显示 | 可见可选 | 允许 | 正常 |
| 停用 | 可见，整行灰 | 不可见 | 拒绝 | 保持不动 |
| 已删除 | 不可见 | 不可见 | 不可能 | 已清理 |

- 创建后默认**停用**（`enabled=0`），给"配置窗口期"。
- **编辑**和**删除**要求必须处于停用态（`enabled=0`），否则拒绝（42015 / 42012）。
- `name`（事件标识）创建后不可变，软删后 name 不可复用。
- 乐观锁 `version`：编辑和 toggle 操作均需携带当前 version，冲突返回 42010。

---

## 3. 字段分层

| 类别 | 字段 | 定义方 | 演进方式 |
|---|---|---|---|
| 系统字段 | `name` / `display_name` / `default_severity` / `default_ttl` / `perception_mode` / `range` | 后端代码硬编码 | 需要 ADMIN 后端 + 前端 + 游戏服务端三方同步发版 |
| 扩展字段 | 运营自定义，如 `priority` / `category` / `cooldown` / `stackable` | `event_type_schema` 表 | 运营在 Schema 管理页自助增删，无需发版 |

两类字段都存在同一个 `event_types.config_json` 列里，对外表现完全一致——导出 API 扁平展开，游戏服务端用 `Extensions map[string]json.RawMessage` 捕获未知 key。区别只在"表单怎么渲染"：系统字段硬编码输入控件，扩展字段通过 `SchemaForm` 动态渲染。

**默认值的归属**：扩展字段的默认值归游戏服务端所有，不归 ADMIN。ADMIN 的 `event_type_schema.default_value` 仅用于新建事件类型时前端表单的初始值提示，不回填历史数据，不影响导出。

---

## 4. 功能清单

### 4.1 列表

- 后端分页（默认 page=1, pageSize=20）
- 支持筛选条件：
  - `label`：display_name 模糊搜索
  - `perception_mode`：精确筛选（visual / auditory / global）
  - `enabled`：null 不筛选 / true 仅启用 / false 仅停用
- 列表展示字段：id, name, display_name, perception_mode, default_severity, default_ttl, range, enabled, created_at
- 其中 default_severity / default_ttl / range 从 config_json unmarshal 后由 service 层填充

### 4.2 创建

请求字段：
- `name`：事件标识，`^[a-z][a-z0-9_]*$`，不可重复（含软删除记录）
- `display_name`：中文名称
- `perception_mode`：visual / auditory / global
- `default_severity`：默认威胁值，0-100
- `default_ttl`：默认存活时间，> 0
- `range`：传播范围，>= 0；global 模式后端强制置 0
- `extensions`：扩展字段 key-value 对象（可选）

**事务编排**：`tx → store.CreateTx + attachSchemaRefs → commit`，在同一事务内写 event_types 行 + schema_refs 引用关系。

### 4.3 详情

返回 `EventTypeDetail`：
- 基础字段：id, name, display_name, perception_mode, enabled, version, created_at, updated_at
- `config`：config_json 展开为 map（系统字段 + 扩展字段值）
- `extension_schema`：当前启用的扩展字段 Schema 定义列表（`EventTypeSchemaLite`）。如果 config 中包含已禁用 Schema 的 key，额外拉 `ListAllLite` 补齐（前端灰显 + "已禁用" tag）。

### 4.4 编辑

- 必须先停用（否则 42015）。name 不可变。
- 乐观锁更新。
- 扩展字段值需通过 Schema 约束校验（`validateExtensions`）。
- **事务编排**：解析旧 config_json 的扩展字段 key（排除系统字段） → diff 新旧 key → `tx → store.UpdateTx + syncSchemaRefs → commit`。

### 4.5 删除

- 必须先停用（否则 42012）。
- 软删除（`deleted=1`）。
- **事务编排**：`tx → store.SoftDeleteTx + schemaRefStore.RemoveByRef → commit`，清理该事件类型的所有 schema_refs 记录。
- 返回 `{id, name, label}`。

### 4.6 标识唯一性校验

- 完整格式校验（正则 + 长度）+ MySQL 唯一性查询（含软删除记录）。
- 返回 `{available, message}`。

### 4.7 启用/停用切换

- 调用方指定目标状态 `enabled`（幂等安全），与 Field/Template 模式一致。
- 乐观锁保护。

---

## 5. config_json 结构

config_json 是系统字段与扩展字段值的合并 JSON，导出 API 直接原样输出给游戏服务端。

```json
{
  "display_name": "发现敌人",
  "default_severity": 80,
  "default_ttl": 30,
  "perception_mode": "visual",
  "range": 15,
  "priority": 5,
  "cooldown": 10
}
```

**系统字段**（固定 5 个）：`display_name`, `default_severity`, `default_ttl`, `perception_mode`, `range`

**扩展字段**（动态）：由 `event_type_schema` 定义，key-value 合并到 config_json 同一层级。运营没填过的扩展字段不进 config_json，导出时也没有这个 key，游戏服务端按自己的 `defaults.go` 兜底。

---

## 6. 扩展字段交互

### 创建/编辑事件类型时

1. 前端从 Schema 管理的内存缓存获取所有启用的扩展字段定义（`ListEnabled`）
2. 根据 field_type + constraints 渲染对应输入控件（SchemaForm 动态渲染）
3. 用户填写的扩展字段值放入 `extensions` 字段提交
4. 后端按 field_name 查 Schema 缓存，用 `util.ValidateValue` 校验每个扩展字段值

### 详情页

1. handler 层 unmarshal config_json，拿到所有 key-value
2. 拉启用的 Schema 列表（`ListEnabled`），检查 config 中是否有禁用 Schema 的 key
3. 如果有禁用但有值的 key，额外拉 `ListAllLite`（含禁用 Schema）补齐
4. 返回 `extension_schema` 供前端渲染（禁用的灰显 + "已禁用" tag）

### schema_refs 维护

事件类型的 CRUD 操作通过事务维护 `schema_refs` 表，追踪每个事件类型使用了哪些扩展字段 Schema：
- **Create**：为 extensions 中的每个 key 写入 `(schema_id, 'event_type', event_type_id)` 引用（`attachSchemaRefs`）
- **Update**：diff 旧/新扩展字段 key，增加新引用、移除不再使用的引用（`syncSchemaRefs`）
- **Delete**：`schemaRefStore.RemoveByRef('event_type', id)` 清理该事件类型的所有引用

这些引用关系供 Schema 管理模块查询"是否被引用"（`HasRefs`）、"被谁引用"（`GetReferences`），实现引用保护（约束收紧检查 + 删除保护）。

---

## 7. API 端点

### 事件类型管理（7 个接口）

| 方法 | 路径 | Handler | 说明 |
|------|------|---------|------|
| POST | `/api/v1/event-types/list` | `EventTypeHandler.List` | 分页列表，支持 label + perception_mode + enabled 筛选 |
| POST | `/api/v1/event-types/create` | `EventTypeHandler.Create` | 创建（含 schema_refs 事务） |
| POST | `/api/v1/event-types/detail` | `EventTypeHandler.Get` | 详情（含扩展字段 Schema 合并） |
| POST | `/api/v1/event-types/update` | `EventTypeHandler.Update` | 编辑（含 schema_refs diff 事务，乐观锁） |
| POST | `/api/v1/event-types/delete` | `EventTypeHandler.Delete` | 软删除（含 schema_refs 清理事务） |
| POST | `/api/v1/event-types/check-name` | `EventTypeHandler.CheckName` | 标识唯一性校验 |
| POST | `/api/v1/event-types/toggle-enabled` | `EventTypeHandler.ToggleEnabled` | 启用/停用切换（乐观锁） |

### 导出 API

| 方法 | 路径 | Handler | 说明 |
|------|------|---------|------|
| GET | `/api/configs/event_types` | `ExportHandler.EventTypes` | 导出所有已启用事件类型 |

返回格式：`{"items": [{"name": "enemy_spotted", "config": {...}}, ...]}`

查询：`SELECT name, config_json AS config FROM event_types WHERE deleted = 0 AND enabled = 1 ORDER BY id`，config_json 原样输出。

---

## 8. 感知模式

| 模式 | 常量 | 含义 | range 行为 |
|------|------|------|-----------|
| `visual` | `util.PerceptionModeVisual` | 视觉感知 | 用户输入，>= 0 |
| `auditory` | `util.PerceptionModeAuditory` | 听觉感知 | 用户输入，>= 0 |
| `global` | `util.PerceptionModeGlobal` | 全局感知 | 后端强制置 0，前端禁用 range 输入 |

global 模式下 `range=0` 的含义：CalcThreat 对 range=0 的 global 事件直接返回 severity，不做距离衰减。

---

## 9. 前置依赖（跨项目协调事项）

事件类型扩展字段机制要求游戏服务端 `EventTypeConfig` 有能力容纳未知字段：
- 系统字段硬解析（缺失 → 启动 reject）
- 未知字段捕获到 `Extensions map[string]json.RawMessage`
- 业务代码消费扩展字段时自带默认值（默认值归服务端所有，ADMIN 不回填历史数据）
