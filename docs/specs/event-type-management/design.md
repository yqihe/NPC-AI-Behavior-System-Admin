# 事件类型管理 — 设计方案

> 对应需求：[requirements.md](requirements.md)
> 对应设计文档：[backend.md](../../v3-PLAN/行为管理/事件类型/backend.md) / [frontend.md](../../v3-PLAN/行为管理/事件类型/frontend.md)

---

## 方案描述

### 存储架构

**MySQL 单存储**，不使用 MongoDB。两张新表：

```sql
-- migrations/004_create_event_types.sql
CREATE TABLE event_types (
  id              BIGINT       NOT NULL AUTO_INCREMENT,
  name            VARCHAR(64)  NOT NULL,
  display_name    VARCHAR(128) NOT NULL,
  perception_mode VARCHAR(16)  NOT NULL,
  config_json     JSON         NOT NULL,
  enabled         TINYINT      NOT NULL DEFAULT 0,
  version         INT          NOT NULL DEFAULT 1,
  created_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at      DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  deleted         TINYINT      NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  UNIQUE KEY uk_name (name),
  KEY idx_list (deleted, enabled, id DESC),
  KEY idx_perception (deleted, perception_mode)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- migrations/005_create_event_type_schema.sql
CREATE TABLE event_type_schema (
  id            BIGINT       NOT NULL AUTO_INCREMENT,
  field_name    VARCHAR(64)  NOT NULL,
  field_label   VARCHAR(128) NOT NULL,
  field_type    VARCHAR(16)  NOT NULL,
  constraints   JSON         NOT NULL,
  default_value JSON         NOT NULL,
  sort_order    INT          NOT NULL DEFAULT 0,
  enabled       TINYINT      NOT NULL DEFAULT 1,
  version       INT          NOT NULL DEFAULT 1,
  created_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at    DATETIME     NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  deleted       TINYINT      NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  UNIQUE KEY uk_field_name (field_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**uk_name 不含 deleted 列**：和字段/模板的 `uk_name (name, deleted)` 不同。字段/模板用含 deleted 的复合唯一键允许同名软删记录并存，但 `ExistsByName` 不过滤 deleted 来保证名字不可复用。事件类型直接用不含 deleted 的唯一键，效果一样——软删时 `UPDATE SET deleted=1` 不改 name，唯一键继续占着。后续如果需要"软删后重新创建同名记录"再改成含 deleted 的复合键。

### 接口定义

**事件类型 CRUD（7 个）：**

| 方法 | 路径 | Handler | 请求体关键字段 |
|---|---|---|---|
| POST | `/api/v1/event-types/list` | `EventTypeHandler.List` | `{label?, perception_mode?, enabled?, page, page_size}` |
| POST | `/api/v1/event-types/create` | `EventTypeHandler.Create` | `{name, display_name, perception_mode, default_severity, default_ttl, range, extensions?}` |
| POST | `/api/v1/event-types/detail` | `EventTypeHandler.Get` | `{id}` |
| POST | `/api/v1/event-types/update` | `EventTypeHandler.Update` | `{id, display_name, perception_mode, default_severity, default_ttl, range, extensions?, version}` |
| POST | `/api/v1/event-types/delete` | `EventTypeHandler.Delete` | `{id}` |
| POST | `/api/v1/event-types/check-name` | `EventTypeHandler.CheckName` | `{name}` |
| POST | `/api/v1/event-types/toggle-enabled` | `EventTypeHandler.ToggleEnabled` | `{id, version}` |

**扩展字段 Schema（5 个）：**

| 方法 | 路径 | Handler | 请求体关键字段 |
|---|---|---|---|
| POST | `/api/v1/event-type-schema/list` | `EventTypeSchemaHandler.List` | `{enabled?}` |
| POST | `/api/v1/event-type-schema/create` | `EventTypeSchemaHandler.Create` | `{field_name, field_label, field_type, constraints, default_value, sort_order}` |
| POST | `/api/v1/event-type-schema/update` | `EventTypeSchemaHandler.Update` | `{id, field_label, constraints, default_value, sort_order, version}` |
| POST | `/api/v1/event-type-schema/toggle-enabled` | `EventTypeSchemaHandler.ToggleEnabled` | `{id, version}` |
| POST | `/api/v1/event-type-schema/delete` | `EventTypeSchemaHandler.Delete` | `{id}` |

**导出 API（1 个）：**

| 方法 | 路径 | Handler |
|---|---|---|
| GET | `/api/configs/event_types` | `ExportHandler.EventTypes` |

### config_json 拼装规则

创建/编辑时，Service 层拼装 `config_json`：

```go
configMap := map[string]any{
    "display_name":     req.DisplayName,
    "default_severity": req.DefaultSeverity,
    "default_ttl":      req.DefaultTTL,
    "perception_mode":  req.PerceptionMode,
    "range":            req.Range,
}
// 只有运营实际填过的扩展字段才进 configMap
for key, val := range req.Extensions {
    configMap[key] = val
}
configJSON, _ := json.Marshal(configMap)
```

导出时 `config_json` **原样输出**到 HTTP 响应，不经过 Go struct 中转。

### constraint 包抽离

从 `service/field.go` 抽出到 `service/constraint/validate.go`：

```go
package constraint

// ValidateValue 校验单个值是否符合 (fieldType, constraints) 约束
// 复用字段管理的 integer min/max、float min/max/precision、string minLength/maxLength/pattern、select options/minSelect/maxSelect 校验逻辑
func ValidateValue(fieldType string, constraints json.RawMessage, value json.RawMessage) *errcode.Error

// ValidateConstraintsSelf 校验约束自身是否自洽（min <= max 等）
func ValidateConstraintsSelf(fieldType string, constraints json.RawMessage) *errcode.Error
```

`service/field.go` 的 `checkConstraintTightened` 保留在原处（只有字段管理用它），但其内部的值级校验辅助函数（`getFloat`、`getString`、`getOptions` 等）移到 constraint 包作为公共工具。

### 缓存策略

**event_types 缓存（Redis）**：和字段/模板完全同构。

| Key | 含义 | TTL |
|---|---|---|
| `event_types:detail:{id}` | 单条详情（含空标记防穿透） | 10min ± jitter |
| `event_types:list:v{ver}:q={hash}` | 列表分页缓存 | 5min ± jitter |
| `event_types:list:version` | 列表缓存版本号 | 永久 |
| `event_types:lock:{id}` | 分布式锁（防击穿） | 3s |

**event_type_schema 缓存（内存）**：和 `DictCache` 同构。

```go
type EventTypeSchemaCache struct {
    mu      sync.RWMutex
    store   *mysql.EventTypeSchemaStore
    schemas []model.EventTypeSchemaLite   // 启用的，按 sort_order 排好序
}

func (c *EventTypeSchemaCache) Load(ctx context.Context) error   // 启动时调
func (c *EventTypeSchemaCache) Reload(ctx context.Context) error // 写后调
func (c *EventTypeSchemaCache) ListEnabled() []model.EventTypeSchemaLite // 直接返回内存
```

### 前端核心组件

**SchemaForm.vue**（通用组件，不绑定事件类型业务逻辑）：

```typescript
interface SchemaFormProps {
  schemas: SchemaFieldDef[]     // 扩展字段定义数组
  values: Record<string, any>   // 当前值（从 config 里抽出的扩展字段部分）
  defaults: Record<string, any> // schema 定义的 default_value（用于暗示值显示）
}

interface SchemaFormEmits {
  (e: 'update', payload: Record<string, any>): void  // 只含 dirty=true 的字段
}
```

内部维护 `dirtyMap: Record<string, boolean>`，每个字段的交互状态。

---

## 方案对比

### 方案 A（选用）：MySQL 单存储 + config_json 列

- 写路径：单次 MySQL 事务
- 读路径：MySQL → Redis Cache-Aside
- 导出路径：`SELECT name, config_json FROM event_types WHERE ...`
- 优点：零同步问题、单事务原子性、和字段/模板同架构
- 缺点：MySQL JSON 列不适合 config 内字段筛选（但我们只筛 perception_mode 提列）

### 方案 B（拒绝）：MySQL + MongoDB 双写 + Transactional Outbox

- 写路径：MySQL 事务（含 outbox 行）→ async worker → Mongo upsert
- 读路径：MySQL（索引）+ Mongo（config 详情）
- 导出路径：Mongo find
- 优点：Mongo 是原始设计里的"配置数据源"
- 缺点：
  1. 引入双写一致性问题，需要 outbox worker + reconcile job + DLQ 面板
  2. 部署多一个 Mongo + worker 进程
  3. 游戏服务端只通过 HTTP 拉取，不在乎后端用什么存储
  4. 事件类型数据量 < 1000 条，MySQL JSON 列完全满足
  5. 事务性（乐观锁 + 引用计数未来接入）在 MySQL 里天然支持

**拒绝理由**：方案 B 的复杂度远高于收益。HTTP 导出 API 对游戏服务端来说是黑盒，MySQL 直出和 Mongo 直出没有可观测差异。

---

## 红线检查

### `docs/standards/red-lines.md` — 通用红线

| 红线 | 检查结果 |
|---|---|
| 禁止静默降级 | ✅ `config_json` unmarshal 失败 → slog.Error + 返回 500；扩展字段校验失败 → 返回 42007 |
| 禁止安全隐患 | ✅ 所有查询参数化（sqlx `?` 占位），`display_name` 模糊搜索用 `escapeLike()`；所有外部 IO 带超时 ctx |
| 禁止信任前端校验 | ✅ Handler 格式校验 + Service 业务校验双层 |
| 禁止测试质量低下 | ✅ 集成测试覆盖所有接口 + 错误路径 |
| 禁止过度设计 | ✅ 不引入 Mongo/MQ；不做 ref_count（等 FSM/BT）；不做导出缓存 |
| 禁止协作失序 | ✅ api-contract.md 已更新并与服务端 CC 确认 |

### `docs/standards/go-red-lines.md` — Go 红线

| 红线 | 检查结果 |
|---|---|
| 禁止资源泄漏 | ✅ 不新增 Client，复用已有 db/redis |
| nil slice/map → null | ✅ 列表返回 `make([]T, 0)`；config_json 拼装用 `map[string]any` 初始化 |
| 禁止 omitempty 吞零值 | ✅ `default_severity: 0` 是合法值（global 事件），config_json 用 `json.Marshal(configMap)` 不加 omitempty |
| json.RawMessage scan NULL | ✅ `config_json JSON NOT NULL`，不会是 NULL |
| 禁止 len() 算中文 | ✅ `name` 是 ASCII（`^[a-z][a-z0-9_]*$`）用 `len()` OK；`display_name` 用 `utf8.RuneCountInString()` |
| 禁止错误码语义混用 | ✅ 42xxx 段位独立，42001-42015 事件类型 / 42020-42031 Schema |
| 禁止硬编码魔术字符串 | ✅ perception_mode 枚举值定义为 `model.PerceptionModeVisual` 等常量 |
| 禁止缓存反序列化类型丢失 | ✅ 使用类型安全结构体 `EventTypeListData` 缓存，不用 `ListData{Items: any}` |
| 禁止分层倒置 | ✅ store/redis 不 import cache 包；Redis key 函数在 `store/redis/keys.go` |

### `docs/standards/mysql-red-lines.md` — MySQL 红线

| 红线 | 检查结果 |
|---|---|
| 事务内不混用 s.db 和 tx | ✅ 本期无跨模块事务，service 内部单表操作用 `s.db`；未来 FSM/BT 接入时 `*Tx` 方法接受外部 tx |
| TOCTOU 用 FOR SHARE | ⚠️ 本期删除不做 FOR SHARE（因为 event_type_refs 不建），在 features.md "本期不做" 已明确声明。FSM/BT 上线时补 |
| LIKE 转义 | ✅ `display_name` 模糊搜索用 `escapeLike()` |

### `docs/standards/redis-red-lines.md` — Redis 红线

| 红线 | 检查结果 |
|---|---|
| 禁止 SCAN+DEL | ✅ 列表缓存用版本号方案 `INCR event_types:list:version` |
| DEL/Unlock 检查 error | ✅ 和字段/模板缓存同实现 |

### `docs/standards/cache-red-lines.md` — 缓存红线

| 红线 | 检查结果 |
|---|---|
| 写后必须清缓存 | ✅ 所有写操作：DEL detail + INCR list version |
| 缓存无 TTL | ✅ detail 10min ± jitter，list 5min ± jitter |
| 分布式锁防击穿 | ✅ `TryLock(id, 3s)` + double-check |
| TOCTOU 遗漏 | ⚠️ 同 mysql-red-lines，本期不做 ref 检查，已声明 |
| 高频路径禁止 sort.Slice | ✅ EventTypeSchemaCache.ListEnabled 返回预排序结果 |

### `docs/standards/frontend-red-lines.md` — 前端红线

| 红线 | 检查结果 |
|---|---|
| 禁止数据源污染 | ✅ 列表过滤用 computed 派生 |
| 枚举用 el-select | ✅ perception_mode 用 radio group（三选一），field_type 用 el-select |
| name blur 格式校验 | ✅ blur 时调 check-name 接口 |
| JSON key 单一权威 | ✅ 扩展字段 key 由 `event_type_schema.field_name` 定义，前端 SchemaForm 按此渲染，不自造 key。系统字段 key 和 api-contract.md 对齐 |
| vue-tsc --noEmit | ✅ R41 验收标准 |
| reactive 显式泛型 | ✅ 所有 reactive 表单对象带 `reactive<EventTypeFormState>(...)` |
| @update:model-value 参数类型 | ✅ 所有回调注解参数类型 |

### `docs/architecture/backend-red-lines.md` — 后端架构红线

| 红线 | 检查结果 |
|---|---|
| 禁止破坏游戏服务端数据格式 | ✅ config_json 原样导出 `{name, config}` 格式；扩展字段由服务端 Extensions map 接收，已有契约 |
| 禁止 config 加服务端不认识的字段 | ✅ 扩展字段不是"ADMIN 私有"而是"运营定义的通用配置"，服务端通过 Extensions map 显式消费。ADMIN 私有元数据（enabled/version/id）不进 config_json |
| 校验结构体字段类型一致 | ⚠️ 注意：服务端 `DefaultSeverity float64`，ADMIN 校验 `∈ [0, 100]` 按整数。JSON wire format `90` 兼容两端，不存在实际不一致。但如果运营输入 `85.5` 也能通过（被 JSON number 存入），服务端按 float64 消费也没问题。如需严格限整数，Handler 校验时做 `math.Floor(v) == v` 判断 |
| 禁止硬编码 | ✅ 错误码用常量；Redis key 用 keys.go 函数；分页/长度限制从 config 读 |
| 禁止绕过 REST API | ✅ 所有数据变更走接口 |
| 禁止 ADMIN 过度设计 | ✅ 不做用户认证/版本控制/实时协作/工作流审批 |

### `docs/architecture/ui-red-lines.md` — UI/UX 红线

| 红线 | 检查结果 |
|---|---|
| 不暴露技术细节 | ✅ perception_mode 用中文 tag（视觉/听觉/全局），错误码映射中文提示 |
| 不让策划手写 JSON | ✅ 系统字段有专用控件，扩展字段通过 SchemaForm 渲染 |
| EnabledGuardDialog 泛型复用 | ✅ 新增 `entityType: 'event_type'` case，不新建组件 |
| Toggle 乐观锁先获详情 | ✅ 列表不返回 version，toggle 前先调 detail 拿 version |
| 停用确认弹窗说明影响 | ✅ 启用说明"启用后 FSM/BT 条件编辑器可见"，停用说明"已有引用不受影响" |
| 删除确认明确对象名 | ✅ "确认删除事件类型「枪声 (gunshot)」？此操作不可恢复" |
| 侧栏用 el-sub-menu | ✅ "行为管理"和"系统设置"都用 el-sub-menu 可折叠 |

---

## 扩展性影响

**正面**：

1. **新增配置类型的标准模式确立**：事件类型建立了"MySQL 单存储 + config_json 列 + 导出 API 直出"的架构模板，后续 FSM / BT / Region 直接复用
2. **约束校验复用包**：`service/constraint/validate.go` 抽出后，任何新增配置类型的扩展字段校验直接调用
3. **SchemaForm.vue 通用组件**：运营在 Schema 管理页添加扩展字段后，表单自动渲染新控件——零前端代码改动
4. **ExportHandler 统一入口**：`handler/export.go` 承载所有 `/api/configs/*` 导出接口，后续添加 FSM/BT/Region 导出只需加方法

**中性**：

- `EventTypeSchemaCache` 内存缓存模式与 `DictCache` 同构，不引入新的基础设施
- 前端 ConstraintPanel 复用已有 5 个 FieldConstraint*.vue，不新增约束面板

**无负面影响**：不侵入字段/模板管理代码（除 constraint 包抽离这一次性重构外）。

---

## 依赖方向

```
                ┌─────────────────┐
                │   router.go     │  路由注册
                └──────┬──────────┘
                       │
        ┌──────────────┼──────────────┐
        │              │              │
        ▼              ▼              ▼
┌──────────────┐ ┌───────────────┐ ┌─────────────┐
│EventTypeHandler│ │EventTypeSchema│ │ExportHandler│
│              │ │Handler        │ │             │
└──────┬───────┘ └──────┬────────┘ └──────┬──────┘
       │                │                  │
       │   ┌────────────┘                  │
       ▼   ▼                               │
┌──────────────────┐  ┌──────────────────┐ │
│EventTypeService  │  │EventTypeSchema   │ │
│                  │  │Service           │ │
│ ┌──────────────┐ │  │ ┌─────────────┐  │ │
│ │constraint/   │ │  │ │constraint/  │  │ │
│ │validate.go   │ │  │ │validate.go  │  │ │
│ └──────────────┘ │  │ └─────────────┘  │ │
└──────┬───────────┘  └──────┬───────────┘ │
       │                     │             │
       ▼                     ▼             │
┌──────────────┐  ┌────────────────────┐   │
│EventTypeStore│  │EventTypeSchemaStore│   │
│(MySQL)       │  │(MySQL)             │   │
└──────────────┘  └────────────────────┘   │
       │                     │             │
       ▼                     ▼             │
┌──────────────┐  ┌────────────────────┐   │
│EventTypeCache│  │EventTypeSchema     │   │
│(Redis)       │  │Cache (内存)         │   │
└──────────────┘  └────────────────────┘   │
                                           │
ExportHandler ──► EventTypeService ──► EventTypeStore
```

**关键约束**：
- `EventTypeService` 和 `EventTypeSchemaService` 之间**无横向依赖**
- `EventTypeHandler.Get` 跨调两个 Service（拿事件类型 + 拿 schema），符合 dev-rules 的 handler 编排模式
- `constraint/validate.go` 是无状态工具包，被两个 Service 调用但不持有任何 store/cache
- `service/field.go` 也调 `constraint/validate.go`，依赖方向单向向下

---

## 陷阱检查

### `docs/development/go-pitfalls.md`

| 陷阱 | 应对 |
|---|---|
| nil slice → null | 列表返回 `make([]EventTypeListItem, 0)`；detail 响应的 extension_schema 同理 |
| json.RawMessage 对 null | `config_json` 列 `NOT NULL`，不会遇到；`constraints` / `default_value` 列也是 `NOT NULL` |
| json.Number 精度 | config_json 拼装用 `map[string]any`，int/float 按 Go 类型序列化，不走 any 反序列化 |
| struct tag 双写 | 本模块不涉及 MongoDB，只需 `json` 和 `db` tag |
| len() 不是字符数 | `name` 用 `len()` 正确（ASCII）；`display_name` / `field_label` 用 `utf8.RuneCountInString()` |
| writeError 后忘 return | WrapCtx 泛型包装器自动处理，handler 内部不直接写响应 |
| checkConstraintTightened 新增 key | constraint 包抽离后，事件类型扩展字段的校验由 `ValidateValue` 覆盖；字段管理的收紧检查保持原样 |

### `docs/development/mysql-pitfalls.md`

| 陷阱 | 应对 |
|---|---|
| 事务内用 tx 不用 s.db | 本期无跨模块事务。service 内部方法全用 `s.db`。未来 `*Tx` 方法接受 `sqlx.Ext` 参数 |
| LIKE 转义 | `display_name` 模糊搜索：`WHERE display_name LIKE ?`, 值用 `"%" + escapeLike(label) + "%"` |
| Docker initdb.d | 新增 004/005 迁移文件需手动执行或重建数据卷 |
| 乐观锁 rows==0 语义 | Service 层先 `getOrNotFound` 预检查存在性，再乐观锁更新。rows=0 则确认是版本冲突 |

### `docs/development/redis-pitfalls.md`

| 陷阱 | 应对 |
|---|---|
| Get 返回 redis.Nil | `EventTypeCache.GetDetail` 用 `errors.Is(err, redis.Nil)` 判断缓存 miss |
| key 用 ID 不用 name | `event_types:detail:{id}`、`event_types:lock:{id}` |
| SetNX 锁设 expire | `TryLock(id, 3s)` 和字段/模板一致 |

### `docs/development/cache-pitfalls.md`

| 陷阱 | 应对 |
|---|---|
| 写后清缓存 | Create/Update/Delete/ToggleEnabled 都 DEL detail + INCR list version |
| 空值标记 | detail miss + DB miss → 写空标记（TTL 短于正常缓存）|
| TTL 加 jitter | `ttl(base, jitter)` 复用字段/模板已有的 jitter 函数 |
| 版本号方案 | 列表缓存 key 带 `v{version}`，写操作 INCR version key |

### `docs/development/frontend-pitfalls.md`

| 陷阱 | 应对 |
|---|---|
| reactive 不写泛型 | `reactive<EventTypeFormState>({...})` 显式泛型 |
| el-form prop 匹配 | prop 对齐 :model 字段名，嵌套用点号路径 |
| el-dialog 残留 | Schema 管理弹窗在 @open 时重置表单 |
| el-slider 精度 | severity slider step=1（整数），不需要小数 |
| computed 无副作用 | 列表过滤/排序用 computed，不在 computed 里发请求 |
| 双向 deep watcher 死循环 | SchemaForm dirty 追踪用事件监听（@change/@input），不用 deep watch |
| ElMessage 样式缺失 | main.ts 确认已手动 import 样式 |
| 列表接口缺 version | toggle 前先调 detail 拿 version |
| 同组件多路由 | 事件类型 list/form 是不同组件，不需要 :key。Schema 管理页只有一个路由，也不需要 |

---

## 配置变更

在 `config.yaml` 新增：

```yaml
event_type:
  name_max_length: 64
  display_name_max_length: 128
  cache_detail_ttl: 10m
  cache_list_ttl: 5m
  cache_lock_ttl: 3s

event_type_schema:
  field_name_max_length: 64
  field_label_max_length: 128
  max_schemas: 100
```

对应 `config.go` 新增：

```go
type EventTypeConfig struct {
    NameMaxLength        int           `yaml:"name_max_length"`
    DisplayNameMaxLength int           `yaml:"display_name_max_length"`
    CacheDetailTTL       time.Duration `yaml:"cache_detail_ttl"`
    CacheListTTL         time.Duration `yaml:"cache_list_ttl"`
    CacheLockTTL         time.Duration `yaml:"cache_lock_ttl"`
}

type EventTypeSchemaConfig struct {
    FieldNameMaxLength  int `yaml:"field_name_max_length"`
    FieldLabelMaxLength int `yaml:"field_label_max_length"`
    MaxSchemas          int `yaml:"max_schemas"`
}
```

---

## 测试策略

### 后端集成测试

沿用字段/模板管理的 `tests/api_test.sh` 模式，新建 `tests/event_type_test.sh`：

**正向用例**：
- 事件类型 CRUD 全流程：create → detail → update → toggle-enabled → list → delete
- 扩展字段 Schema CRUD 全流程
- 带扩展字段的事件类型创建/编辑/导出
- 导出 API 格式验证（`{items: [{name, config}]}`）
- check-name 唯一性（含软删除）
- global 模式 range 自动置 0

**错误路径**：
- 42001 name 重复
- 42002 name 格式非法
- 42003 perception_mode 非法枚举
- 42004 severity 超范围
- 42007 扩展字段值不符合约束
- 42010 乐观锁冲突
- 42012 删除未停用
- 42015 编辑未停用
- 42020-42027 Schema 各错误码

**攻击性测试**（参考字段管理的 atk 系列）：
- name 含特殊字符 / 中文 / 大写 / 空格
- extensions 里塞不存在的 schema key
- extensions 里塞已停用的 schema key
- severity=0（合法零值，不被 omitempty 吞）
- display_name 含 SQL 注入字符 `' OR 1=1 --`
- 超长 config_json（堆大量扩展字段到接近 1MB）
- 并发创建同名事件类型

### 后端单元测试

- `service/constraint/validate.go`：每种类型 + 每种约束条件的正/反 case
- `service/event_type.go`：mock store，测 config_json 拼装逻辑、扩展字段校验分支
- `service/event_type_schema.go`：mock store，测约束自洽校验、default_value 校验

### 前端

- `npx vue-tsc --noEmit` 通过
- 手动测试：新建/编辑/删除/启停全流程 + SchemaForm dirty 追踪 + perception_mode 联动 range 禁用 + severity slider
- Schema 管理页：新增/编辑/启停/删除扩展字段 + 约束编辑面板

---

## 待审批确认

以上方案符合所有红线和陷阱检查，无违规项需要修改红线。

⚠️ 一个观察点需你确认：`default_severity` 在服务端 struct 里是 `float64`，ADMIN 侧按整数 0-100 校验。JSON wire format 兼容，但是否允许运营输入 `85.5` 这样的小数？
- **选项 A**：Handler 校验只检查 `∈ [0, 100]`，允许小数（和服务端 float64 完全一致）
- **选项 B**：Handler 额外检查 `math.Floor(v) == v`，只允许整数

我倾向 A（宽松），因为服务端是 float64 能处理。

审批通过后进入 Phase 3（任务拆解）。
