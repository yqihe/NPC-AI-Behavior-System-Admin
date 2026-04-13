# 字段管理 — 后端设计

> 通用架构/规范见 `docs/architecture/overview.md` 和 `docs/development/`。
> 本文档只记录字段管理模块的实现事实与特有约束。

---

## 1. 目录结构

```
backend/internal/
├── handler/field.go              HTTP 入口 + 请求格式校验 + 跨模块拼装（GetReferences 补 template label）
├── service/
│   ├── field.go                  业务逻辑 + Cache-Aside + 约束收紧检查 + 循环引用检测 + 跨模块对外方法
│   └── constraint/               约束解析工具（parseConstraintsMap / GetFloat / GetString / ParseSelectOptions）
├── store/mysql/
│   ├── field.go                  fields 表 CRUD + 覆盖索引 List + 乐观锁 Update + 事务内 IncrRefCountTx / DecrRefCountTx
│   └── field_ref.go              field_refs 关联表 Add / Remove / RemoveBySource / HasRefsTx(FOR SHARE) / GetByFieldID
├── store/redis/
│   ├── field.go                  FieldCache — Detail(Get/Set/Del) + List(Get/Set/InvalidateList) + TryLock/Unlock
│   └── config/                   Redis 缓存共享配置子包
│       ├── common.go             TTL / Ping / Available / NullMarker 等共享常量与工具函数
│       └── keys.go               Redis key 生成器（FieldDetailKey / FieldListKey / FieldLockKey + FieldListVersionKey）
├── cache/dictionary.go           进程内 DictCache（启动一次性加载 field_type / field_category 字典，运行时只读）
├── model/field.go                Field / FieldLite / FieldListItem / FieldProperties / FieldRef / DTO（请求/响应/查询）
├── errcode/
│   ├── codes.go                  字段错误码 40001-40017
│   └── store_errors.go           Store 层哨兵错误（ErrNotFound / ErrVersionConflict / ErrDuplicate）
└── router/router.go              POST /api/v1/fields/* 路由注册
```

---

## 2. 数据表

### 2.1 fields

```sql
CREATE TABLE IF NOT EXISTS fields (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    name            VARCHAR(64)  NOT NULL,              -- 字段标识，唯一，创建后不可变
    label           VARCHAR(128) NOT NULL,              -- 中文标签（搜索用）
    type            VARCHAR(32)  NOT NULL,              -- 字段类型（筛选用）
    category        VARCHAR(32)  NOT NULL,              -- 标签分类（筛选用）
    properties      JSON         NOT NULL,              -- 动态属性（描述/BB Key/默认值/约束等）

    ref_count       INT          NOT NULL DEFAULT 0,    -- 被引用数（冗余计数，事务内维护）
    enabled         TINYINT(1)   NOT NULL DEFAULT 0,    -- 启用状态（0=停用，1=启用）
    version         INT          NOT NULL DEFAULT 1,    -- 乐观锁
    deleted         TINYINT(1)   NOT NULL DEFAULT 0,    -- 软删除
    created_at      DATETIME     NOT NULL,
    updated_at      DATETIME     NOT NULL,

    UNIQUE KEY uk_name (name),
    INDEX idx_list (deleted, id, name, label, type, category, ref_count, enabled, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**索引说明**：

| 索引 | 类型 | 用途 |
|---|---|---|
| `uk_name (name)` | UNIQUE | name 全局唯一（含软删除记录，name 永不复用），`ExistsByName` / `GetByName` 走此索引 |
| `idx_list (deleted, id, name, label, type, category, ref_count, enabled, created_at)` | 覆盖索引 | 列表查询 `List` 不回表，WHERE deleted=0 + type/category/enabled/label LIKE 筛选 + ORDER BY id DESC + LIMIT/OFFSET 全部命中此索引 |

**约束说明**：

- `name` 含软删除记录不复用：`ExistsByName` 不过滤 `deleted` 列，保证历史快照中的 name 不会对应一个语义不同的新字段
- `ref_count` 冗余计数：事务内通过 `IncrRefCountTx` / `DecrRefCountTx` 原子维护，与 `field_refs` 行数保持一致
- `version` 乐观锁：`UPDATE ... WHERE id=? AND version=?` rows=0 时 store 返回 `errcode.ErrVersionConflict`（哨兵错误定义在 `errcode/store_errors.go`），service 转为 `40010`
- `enabled` 启用闸门：启用中禁止编辑（40015）和删除（40012），防止引用方看到不稳定配置

### 2.2 field_refs

```sql
CREATE TABLE IF NOT EXISTS field_refs (
    field_id    BIGINT       NOT NULL,              -- 被引用的字段 ID
    ref_type    VARCHAR(16)  NOT NULL,              -- 引用来源：'template' / 'field'
    ref_id      BIGINT       NOT NULL,              -- 引用方 ID（模板 ID 或字段 ID）

    PRIMARY KEY (field_id, ref_type, ref_id),
    INDEX idx_ref (ref_type, ref_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**索引说明**：

| 索引 | 类型 | 用途 |
|---|---|---|
| `PRIMARY KEY (field_id, ref_type, ref_id)` | 联合主键 | `GetByFieldID` 走前缀 `field_id`；`HasRefsTx(FOR SHARE)` 走前缀 `field_id` |
| `idx_ref (ref_type, ref_id)` | 二级索引 | `RemoveBySource` 按引用方查询并批量删除 |

**两种引用来源**：

- `ref_type = 'template'`：模板引用了该字段（由 `TemplateHandler` 在跨模块事务内通过 `AttachToTemplateTx` / `DetachFromTemplateTx` 写入/删除）
- `ref_type = 'field'`：某 reference 类型字段引用了该字段（由 `FieldService.syncFieldRefs` 在 Create/Update reference 字段时写入）

---

## 3. API 接口

### POST `/api/v1/fields/list` — 分页列表

| 项 | 值 |
|---|---|
| **Request** | `{ label?: string, type?: string, category?: string, enabled?: bool, page: int, page_size: int }` |
| **Response** | `{ items: FieldListItem[], total: int, page: int, page_size: int }` |
| **调用链** | handler.List -> service.List -> cache.GetList (Redis hit 直返) -> store.List (覆盖索引) -> DictCache 翻译 type_label/category_label -> cache.SetList |
| **ErrorCode** | 无业务错误码，仅通用 40000 |

### POST `/api/v1/fields/create` — 创建字段

| 项 | 值 |
|---|---|
| **Request** | `{ name: string, label: string, type: string, category: string, properties: JSON }` |
| **Response** | `{ id: int, name: string }` |
| **Handler 校验** | name 正则 + 长度、label 非空 + 长度、type/category 非空、properties 必须是 JSON 对象 |
| **Service 校验** | type/category 字典存在性 (DictCache)、name 唯一性 (含软删除)、reference 类型: refs 非空 + 目标存在 + 目标启用 + 非嵌套 + 无循环 |
| **ErrorCode** | 40001(name 已存在)、40002(name 格式非法)、40003(type 不存在)、40004(category 不存在)、40009(循环引用)、40013(引用已停用字段)、40014(引用目标不存在)、40016(reference 嵌套)、40017(refs 为空) |

### POST `/api/v1/fields/detail` — 字段详情

| 项 | 值 |
|---|---|
| **Request** | `{ id: int }` |
| **Response** | `Field` 完整结构（含 properties） |
| **调用链** | handler.Get -> service.GetByID -> cache.GetDetail (含空标记) -> TryLock (分布式锁 3s) -> double-check -> store.GetByID -> cache.SetDetail (nil 时写空标记防穿透) |
| **ErrorCode** | 40011(字段不存在) |

### POST `/api/v1/fields/update` — 编辑字段

| 项 | 值 |
|---|---|
| **Request** | `{ id: int, label: string, type: string, category: string, properties: JSON, version: int }` |
| **Response** | `"保存成功"` |
| **Handler 校验** | id > 0、label 非空 + 长度、type/category 非空、properties JSON 对象、version > 0 |
| **Service 校验** | type/category 字典存在性、字段存在性、enabled 必须 false (40015)、被引用禁改 type (40006)、被引用禁收紧约束 (40007)、reference 类型增量校验 |
| **ErrorCode** | 40003、40004、40006(被引用改类型)、40007(被引用收紧约束)、40009、40010(版本冲突)、40011、40013、40014、40015(启用中禁编辑)、40016、40017 |

### POST `/api/v1/fields/delete` — 软删除

| 项 | 值 |
|---|---|
| **Request** | `{ id: int }` |
| **Response** | `{ id: int, name: string, label: string }` |
| **调用链** | service.Delete -> getFieldOrNotFound -> enabled 必须 false -> 开 tx -> HasRefsTx(FOR SHARE 防 TOCTOU) -> SoftDeleteTx -> reference 类型: RemoveBySource + DecrRefCountTx -> Commit -> 清缓存 |
| **ErrorCode** | 40005(被引用无法删除)、40011(字段不存在)、40012(删除前必须先停用) |

### POST `/api/v1/fields/check-name` — 标识唯一性校验

| 项 | 值 |
|---|---|
| **Request** | `{ name: string }` |
| **Response** | `{ available: bool, message: string }` |
| **说明** | `ExistsByName` 不过滤 `deleted`，已软删除的 name 也返回不可用 |
| **ErrorCode** | 无业务错误码 |

### POST `/api/v1/fields/references` — 引用详情

| 项 | 值 |
|---|---|
| **Request** | `{ id: int }` |
| **Response** | `{ field_id: int, field_label: string, templates: ReferenceItem[], fields: ReferenceItem[] }` |
| **调用链** | handler.GetReferences -> service.GetReferences (查 field_refs + 按 ref_type 分组, 字段 label 用 GetByIDs 补) -> handler 跨模块调 templateService.GetByIDsLite 补 template label |
| **ErrorCode** | 40011(字段不存在) |

### POST `/api/v1/fields/toggle-enabled` — 启用/停用切换

| 项 | 值 |
|---|---|
| **Request** | `{ id: int, enabled: bool, version: int }` |
| **Response** | `"操作成功"` |
| **调用链** | service.ToggleEnabled -> getFieldOrNotFound -> store.ToggleEnabled (乐观锁) -> 清 detail + InvalidateList |
| **ErrorCode** | 40010(版本冲突)、40011(字段不存在) |

---

## 4. 缓存策略

### 4.1 Detail 缓存

| 项 | 值 |
|---|---|
| **Key** | `fields:detail:{id}` |
| **TTL** | 5min + 0~30s 随机抖动（防雪崩） |
| **防击穿** | 分布式锁 `fields:lock:{id}`（3s expire），TryLock + double-check |
| **防穿透** | 空标记 `{"_null":true}`，field=nil 时也写缓存 |
| **失效** | 写操作（Update / Delete / ToggleEnabled / 跨模块 ref_count 变化）后 `DelDetail(id)` + 受影响 ID |
| **降级** | Redis 不可用时 TryLock 失败，降级直查 MySQL，不阻塞 |

### 4.2 List 缓存

| 项 | 值 |
|---|---|
| **Key** | `fields:list:v{N}:{type}:{category}:{label}:{enabled}:{page}:{pageSize}` |
| **TTL** | 1min + 0~10s 随机抖动 |
| **版本号** | `fields:list:version`（Redis INCR），所有写操作调 `InvalidateList` 递增版本号，旧版本 key 自然过期，无需 SCAN |
| **降级** | Redis 不可用时跳过缓存，直查 MySQL |

### 4.3 DictCache（进程内）

| 项 | 值 |
|---|---|
| **存储** | 进程内 `map[group][name] -> label` |
| **TTL** | 永不过期，启动时一次 Load() |
| **热更** | 不支持运行时热更，改字典需重启进程 |
| **用途** | List 翻译 type_label / category_label、GetByIDsLite 翻译 CategoryLabel、Create/Update 校验 type/category 存在性 |

---

## 5. 错误码

字段管理错误码范围 40001-40017：

| 错误码 | 常量 | 触发场景 |
|---|---|---|
| 40001 | `ErrFieldNameExists` | Create 时 `ExistsByName` 返回 true（含软删除记录） |
| 40002 | `ErrFieldNameInvalid` | Handler 校验 name 为空、不匹配 `^[a-z][a-z0-9_]*$` 正则、或超长 |
| 40003 | `ErrFieldTypeNotFound` | Create/Update 时 DictCache 中 `field_type` 组无该值 |
| 40004 | `ErrFieldCategoryNotFound` | Create/Update 时 DictCache 中 `field_category` 组无该值 |
| 40005 | `ErrFieldRefDelete` | Delete 事务内 `HasRefsTx(FOR SHARE)` 发现 field_refs 非空 |
| 40006 | `ErrFieldRefChangeType` | Update 时 `old.Type != req.Type && old.RefCount > 0` |
| 40007 | `ErrFieldRefTighten` | Update 时 `ref_count > 0 && type 未变`，且 `checkConstraintTightened` 检测到约束收紧（integer/float: min 增大/max 减小/precision 减小；string: minLength 增大/maxLength 减小/pattern 新增或变更；select: options 删除/minSelect 增大/maxSelect 减小） |
| 40008 | `ErrFieldBBKeyInUse` | BB Key 被行为树引用（预留，本期未接入） |
| 40009 | `ErrFieldCyclicRef` | Create/Update reference 字段时 `detectCyclicRef` DFS 检测到循环 |
| 40010 | `ErrFieldVersionConflict` | Update / ToggleEnabled 时 `UPDATE ... WHERE version=?` rows=0，store 返回 `errcode.ErrVersionConflict` |
| 40011 | `ErrFieldNotFound` | Detail / Update / Delete / References / ToggleEnabled 时 `GetByID` 返回 nil |
| 40012 | `ErrFieldDeleteNotDisabled` | Delete 时 `field.Enabled == true` |
| 40013 | `ErrFieldRefDisabled` | Create/Update reference 字段时新增的 ref 目标 `Enabled == false` |
| 40014 | `ErrFieldRefNotFound` | Create/Update reference 字段时 ref 目标 `GetByID` 返回 nil |
| 40015 | `ErrFieldEditNotDisabled` | Update 时 `old.Enabled == true` |
| 40016 | `ErrFieldRefNested` | Create/Update reference 字段时新增的 ref 目标 `Type == "reference"`（禁止嵌套） |
| 40017 | `ErrFieldRefEmpty` | Create/Update reference 字段时 `refs` 数组为空 |
