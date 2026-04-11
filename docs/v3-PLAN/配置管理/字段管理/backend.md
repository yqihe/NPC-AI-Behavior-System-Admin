# 字段管理 — 后端设计

> **实现状态**：已全部落地，与 `backend/internal/{handler,service,store,cache,model,errcode}/field*.go` 完全对齐。
> 通用技术选型（Go + gin + sqlx + slog + Redis）、分层硬规则、跨模块事务规则见 `docs/backend-guide.md` 与 `docs/development/dev-rules.md`。
> 本文档只记录字段管理模块的实现事实与特有约束，不重复通用规则。

---

## 存储范围

字段是 ADMIN 内部的管理概念，游戏服务端不需要字段定义（导出的 5 个接口都不含 fields）。字段值最终通过「模板 → NPC → 导出」打平写入 `npc_templates.config.fields`。

- **MySQL**：唯一写入目标
- **Redis**：detail / list 缓存 + 分布式锁
- **MongoDB / RabbitMQ**：不涉及（无跨库同步）

## 操作标识

**所有操作使用主键 ID (BIGINT)**，不使用 name。name 只在两个场景出现：

1. 创建请求体 + 创建响应返回值
2. `/check-name` 唯一性校验

企业级 CRUD 系统的标准做法：主键 ID 做操作标识，`field_refs` 的 JOIN/IN 查询更高效，name 只用于展示和 uniqueness 校验。

---

## 目录结构

```
backend/internal/
├── handler/field.go            HTTP 入口 + 请求格式校验 + 跨模块拼装
├── service/field.go            业务逻辑 + Cache-Aside + 跨模块对外方法
├── store/mysql/
│   ├── field.go                fields 表 CRUD + 覆盖索引 List
│   └── field_ref.go            field_refs 关联表 CRUD + FOR SHARE 读
├── store/redis/
│   ├── field.go                FieldCache (Detail/List/Lock)
│   └── keys.go                 Redis key 生成器
├── cache/dictionary.go         进程内 DictCache（启动一次性加载）
├── model/field.go              Field / FieldLite / FieldListItem / Properties / DTO
├── errcode/codes.go            40001-40017
└── router/router.go            POST /api/v1/fields/* 路由注册
```

## 数据表

### fields

```sql
CREATE TABLE fields (
  id              BIGINT AUTO_INCREMENT PRIMARY KEY,
  name            VARCHAR(64)  NOT NULL,
  label           VARCHAR(128) NOT NULL,
  type            VARCHAR(32)  NOT NULL,
  category        VARCHAR(32)  NOT NULL,
  properties      JSON         NOT NULL,         -- {description, expose_bb, default_value, constraints}
  ref_count       INT          NOT NULL DEFAULT 0,
  enabled         TINYINT(1)   NOT NULL DEFAULT 0,
  version         INT          NOT NULL DEFAULT 1,
  deleted         TINYINT(1)   NOT NULL DEFAULT 0,
  created_at      DATETIME     NOT NULL,
  updated_at      DATETIME     NOT NULL,
  UNIQUE KEY uk_name (name),                     -- 含软删除记录，name 永不复用
  INDEX idx_list (deleted, id, name, label, type, category, ref_count, enabled, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### field_refs

```sql
CREATE TABLE field_refs (
  field_id    BIGINT       NOT NULL,              -- 被引用的字段 ID
  ref_type    VARCHAR(16)  NOT NULL,              -- 'template' 或 'field'
  ref_id      BIGINT       NOT NULL,              -- 引用方 ID（模板 ID 或 reference 字段 ID）
  PRIMARY KEY (field_id, ref_type, ref_id),
  INDEX idx_ref (ref_type, ref_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**两种引用来源**：

- `ref_type = 'template'`：模板引用了该字段（由 `TemplateHandler` 在跨模块事务内写入 / 删除）
- `ref_type = 'field'`：某 reference 类型字段引用了该字段（由 `FieldService` 在 Create/Update reference 字段时写入）

**ref_count 维护**：事务内 `IncrRefCountTx` / `DecrRefCountTx` 原子递增递减，与 `field_refs` 行保持一致。

---

## API 接口

| Method | Path | 用途 |
|---|---|---|
| POST | `/api/v1/fields/list` | 列表（按 label 模糊 + type/category/enabled 三态） |
| POST | `/api/v1/fields/create` | 创建（默认 `enabled=false`） |
| POST | `/api/v1/fields/detail` | 详情（不过滤 enabled，模板引用的停用字段仍能查） |
| POST | `/api/v1/fields/update` | 编辑（要求 `enabled=false` + 乐观锁） |
| POST | `/api/v1/fields/delete` | 软删除（要求 `enabled=false` + `ref_count=0`） |
| POST | `/api/v1/fields/check-name` | 唯一性校验（含软删除记录） |
| POST | `/api/v1/fields/references` | 引用详情（templates + 其他 reference 字段两类） |
| POST | `/api/v1/fields/toggle-enabled` | 启用 / 停用（乐观锁） |

---

## 核心调用链

### 列表 — `FieldService.List`

```
handler.List → service.List
  → cache.GetList(key)                        # Redis 命中直接返回
  → store.List(q)                             # miss 走 MySQL 覆盖索引
  → 内存翻译 type_label / category_label      # via DictCache
  → cache.SetList(key, data, ttl+jitter)
```

列表缓存 key 含 `version` 号（`fields:list:v{N}:...`），任何写操作通过 `InvalidateList`（INCR 版本号）让所有变体一次失效，无需 SCAN 删 key。

### 详情 — `FieldService.GetByID`

Cache-Aside + 分布式锁 + 空标记三件套，防击穿防穿透：

```
service.GetByID(id)
  → cache.GetDetail(id)                       # 命中直接返回（含空标记情形）
  → cache.TryLock(id, 3s)                     # miss 时抢分布式锁防击穿
      ├─ 锁成功  → double-check 缓存 → miss → store.GetByID → cache.SetDetail
      └─ 锁失败  → 降级直查 MySQL（不阻塞）
  → field=nil 时写空标记防穿透
```

### 创建 — `FieldService.Create`

```
handler.Create → 格式校验（name 正则 / label 长度 / properties 形状）
  → service.Create
    → checkDictExists(type) + checkDictExists(category)    # DictCache
    → ExistsByName(name)                                   # 40001（含软删）
    → if type == reference: validateReferenceRefs
        # 非空 40017 / 存在 40014 / 启用 40013 / 非嵌套 40016 / 无循环 40009
    → store.Create(tx)                                      # 主记录 INSERT
    → syncFieldRefs(id, nil, refIDs)                        # 单独 tx：写 field_refs + IncrRefCountTx
    → cache.InvalidateList
```

> **已知小瑕疵**：主记录 INSERT 和 `syncFieldRefs` 不在同一个 tx 里（Create/Update 同源）。极端场景主成功 + 引用失败会不一致。待后续统一事务重构。

### 编辑 — `FieldService.Update`

乐观锁 + 引用后约束收紧 + reference refs 增量校验：

```
handler.Update → 格式校验 + version > 0
  → service.Update
    → checkDictExists(type/category)
    → getFieldOrNotFound                                    # 40011
    → enabled != false                       → 40015       # 启用中禁止编辑
    → ref_count > 0 且 type 变 → 40006                       # 被引用禁改类型
    → ref_count > 0 且 type 未变 → checkConstraintTightened
        # integer min↓/max↑、float 含 precision 单调、
        # string minLength↓/maxLength↑/pattern 只能移除、
        # select options 只增不删 + minSelect↓/maxSelect↑ → 40007
    → if type == reference:
        → oldRefSet vs newRefSet 差集
        → 只对"新增"目标校验 40013/40014/40016/40009
        → "已有"的目标即使变停用或变嵌套也保留（存量不动）
    → store.Update (WHERE id=? AND version=?)  → 0 行 → 40010
    → syncFieldRefs(id, oldRefIDs, newRefIDs)
    → type 从 reference 改为其他 → RemoveBySource + DecrRefCountTx
    → InvalidateDetails(affected) + InvalidateList
```

### 删除 — `FieldService.Delete`

软删 + FOR SHARE 防 TOCTOU：

```
handler.Delete → 格式校验
  → service.Delete
    → getFieldOrNotFound
    → enabled != false → 40012
    → 开 tx
    → FieldRefStore.HasRefsTx(tx, id)    # FOR SHARE，防止"前面查无引用后面被插入"
        → true → 40005
    → FieldStore.SoftDeleteTx(tx, id)
    → if type == reference:
        → FieldRefStore.RemoveBySource(tx, 'field', id)
        → 对每个被引用方 DecrRefCountTx
    → Commit
    → InvalidateDetails(affected) + InvalidateList
```

### 引用详情 — `FieldService.GetReferences` + Handler 跨模块补齐

```
handler.GetReferences
  → service.GetReferences(id)
      # 字段模块内只查 field_refs + 按 ref_type 分组
      # Fields 类的 label 用 FieldStore.GetByIDs 批量补
      # Templates 类只填 RefID，Label 留空
  → templateService.GetByIDsLite(templateIDs)   # 跨模块补模板 label
  → 按 ref_type 分组的两个数组 + 补齐的 label 返回给前端
```

**分层边界**：`FieldService` 只持有 `FieldStore / FieldRefStore / FieldCache / DictCache`，不认识 TemplateStore/Cache/Service。跨模块编排发生在 Handler 层（见 `dev-rules.md` 「分层职责」）。

### 启用切换 — `FieldService.ToggleEnabled`

纯单模块乐观锁写 + detail/list 缓存清理。版本冲突 → 40010。

---

## 缓存策略

| 层 | Key 形态 | TTL | 防护机制 |
|---|---|---|---|
| detail | `fields:detail:{id}` | 5min + 0-30s jitter | 分布式锁 `fields:lock:{id}`（3s）+ double-check + 空标记 `{"_null":true}` |
| list | `fields:list:v{N}:{type}:{category}:{label}:{enabled}:{page}:{ps}` | 1min + 0-10s jitter | 版本号 `fields:list:version`，INCR 一次所有变体失效 |
| DictCache | 进程内 `map[group][name] → label` | 永不过期 | 启动一次 Load()，不支持运行时热更，改字典需重启进程 |

**降级**：Redis 不可用时所有路径穿透到 MySQL，不阻塞；日志记录降级事件。

---

## 跨模块对外接口（给 TemplateHandler 调用）

| 方法 | 用途 | 事务归属 | 错误码段位 |
|---|---|---|---|
| `ValidateFieldsForTemplate(ctx, ids)` | 校验目标字段全部存在 + 启用 + 非 reference 类型 | 事务外（预校验）| 41005 / 41006 / 41012 |
| `AttachToTemplateTx(ctx, tx, tplID, ids)` | 事务内写 `field_refs(ref_type=template)` + IncrRefCountTx，返回受影响 fieldIDs | 外部 tx | — |
| `DetachFromTemplateTx(ctx, tx, tplID, ids)` | 事务内删 `field_refs(ref_type=template)` + DecrRefCountTx，返回受影响 fieldIDs | 外部 tx | — |
| `GetByIDsLite(ctx, ids)` | 给模板详情拼装用。按 `ids` 顺序对齐返回 `[]FieldLite`，缺失位用零值占位 + `CategoryLabel` DictCache 翻译 | 无 tx | — |
| `InvalidateDetails(ctx, ids)` | 模板写完成后由 handler 调用（模板写改了这些字段的 ref_count），批量清 detail 缓存 | 无 tx | — |

**重要约定**：`41005/41006/41012` 归在模板段位而非字段段位，因为这三个错误码由**模板管理页**消费，语义上属于「模板要求字段的前置条件」，不混用字段段 `40011/40013/40016`。

---

## 约束 key 契约

`FieldProperties.Constraints` 是 `json.RawMessage`，DB 层不校验结构，命名靠前后端约定。**单一权威**是 `backend/cmd/seed/main.go` 里 `field_type` 字典的 `constraint_schema` 字段。

| 类型 | 约束 key | 收紧检查（`ref_count > 0 && type 未变` 时） |
|---|---|---|
| integer | `min` / `max` / `step` | min↓ / max↑（step 不检查）|
| float | `min` / `max` / `precision` | 同上 + precision 只可增（截断已存数据） |
| string | `minLength` / `maxLength` / `pattern` | minLength↓ / maxLength↑ / pattern 只可移除不可新增或变更 |
| boolean | — | 无约束 |
| select | `options` / `minSelect` / `maxSelect` | options 只可新增 / minSelect↓ / maxSelect↑ |
| reference | `refs` | 由 `validateReferenceRefs` 单独处理（非空 / 存在 / 启用 / 非嵌套 / 无循环）|

**reference 持久化契约**：`properties.constraints.refs` 是 `number[]`（被引用字段的 ID 数组）。**前端 `FieldForm.vue` 的 `ref_fields` 富对象 `[{id,name,label,type}]` 只是 UI 本地状态**，`loadFieldDetail` 从 `refs` 转入、`buildSubmitProperties` 转回 `refs` 提交，后端永远只见到 `refs`。其他任何组件（如模板的 reference popover）读 detail 时必须读 `refs`，不能假设后端返回富对象。

---

## 不变量与陷阱

- **软删除 name 不复用**：`ExistsByName` 不过滤 `deleted` 列，曾经存在的 name 永远被占用。保证历史 NPC/模板快照里的 name 不会对应一个语义完全不同的新字段。
- **FOR SHARE 防 TOCTOU**：`Delete` 的 `HasRefsTx` 用共享锁重新确认 `field_refs` 空，防止"前面查无引用后面被插入"的竞态。
- **启用中禁止编辑 / 删除**：`40015` / `40012` 双闸——启用中的字段对模板/其他 reference 字段可见，允许修改会让引用方看到不稳定的配置。
- **乐观锁**：`UPDATE ... WHERE id=? AND version=?` rows=0 → `storemysql.ErrVersionConflict` → Service 转 `40010`。
- **reference 禁止嵌套**：`refB.refs = [refA]` 直接拒绝（40016），因为模板的 popover 假设子字段必是 leaf，一层展开即完整。
- **跨模块事务打开位置**：Create/Update/Delete 的跨模块路径（写 field_refs + 改 ref_count）都由 **TemplateHandler** 开 tx 后传给 `FieldService.*Tx` 方法，FieldService 本身不认识 TemplateStore。
- **DictCache 是只读基础设施**：所有模块可直接读它做 label 翻译，这是分层红线的例外（基础设施不算跨模块）。

---

## 错误码

| 错误码 | 常量 | 含义 |
|---|---|---|
| 40001 | `ErrFieldNameExists` | 字段标识已存在（含软删除） |
| 40002 | `ErrFieldNameInvalid` | 标识格式不合法 |
| 40003 | `ErrFieldTypeNotFound` | 字段类型字典不存在 |
| 40004 | `ErrFieldCategoryNotFound` | 标签分类字典不存在 |
| 40005 | `ErrFieldRefDelete` | 被引用无法删除 |
| 40006 | `ErrFieldRefChangeType` | 被引用无法修改类型 |
| 40007 | `ErrFieldRefTighten` | 被引用无法收紧约束 |
| 40008 | `ErrFieldBBKeyInUse` | BB Key 被行为树引用（预留，未接入） |
| 40009 | `ErrFieldCyclicRef` | reference 循环引用 |
| 40010 | `ErrFieldVersionConflict` | 乐观锁版本冲突 |
| 40011 | `ErrFieldNotFound` | 字段不存在 |
| 40012 | `ErrFieldDeleteNotDisabled` | 删除前必须先停用 |
| 40013 | `ErrFieldRefDisabled` | 不能引用已停用的字段 |
| 40014 | `ErrFieldRefNotFound` | 引用目标字段不存在 |
| 40015 | `ErrFieldEditNotDisabled` | 编辑前必须先停用 |
| 40016 | `ErrFieldRefNested` | reference 字段禁止嵌套引用 |
| 40017 | `ErrFieldRefEmpty` | reference 字段 refs 不能为空 |

---

## 详细功能说明

每个 API 的场景、校验分层、完整调用链、已知限制见同目录下 `features.md`（按"功能 X"编号展开）。本文档只覆盖架构层面的事实。
