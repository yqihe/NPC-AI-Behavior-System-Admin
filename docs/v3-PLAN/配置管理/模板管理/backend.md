# 模板管理 — 后端设计

> **实现状态**：已全部落地（集成测试 199/199 通过），与 `backend/internal/{handler,service,store,cache,model,errcode}/template*.go` 完全对齐。
> 通用技术选型、分层硬规则、跨模块事务规则见 `docs/backend-guide.md` 与 `docs/development/dev-rules.md`。
> 本文档只记录模板管理特有的实现事实与跨模块编排细节，不重复通用规则。

---

## 存储范围

模板是 ADMIN 内部的字段组合方案，NPC 创建时选一个模板填值，创建后 NPC 独立于模板（字段列表 + 值已快照）。

- **MySQL**：唯一写入目标
- **Redis**：`*model.Template` 裸行 detail 缓存 + 列表缓存 + 分布式锁
- **MongoDB / RabbitMQ**：不涉及（模板自身不产生导出数据，NPC 配置层才产生 `npc_templates` 集合）

## 操作标识

所有操作使用主键 ID（BIGINT）。`name` 只出现在：

1. 创建请求体 + 创建响应返回值
2. `/check-name` 唯一性校验
3. 跨模块 `GetByIDsLite` 返回 `TemplateLite.Name` 供字段引用详情补 label 展示

---

## 目录结构

```
backend/internal/
├── handler/template.go         HTTP 入口 + 跨模块事务编排 + 拼装 TemplateDetail
├── service/template.go         业务逻辑 + Cache-Aside (only Template 裸行) + 对外接口
├── store/mysql/template.go     templates 表 CRUD + 覆盖索引 + 乐观锁
├── store/redis/template.go     TemplateCache (Detail / List / Lock)
├── model/template.go           Template / TemplateFieldEntry / TemplateListItem / TemplateDetail / TemplateFieldItem / TemplateLite / DTO
├── errcode/codes.go            41001-41012
└── router/router.go            POST /api/v1/templates/* 路由注册
```

## 数据表

```sql
CREATE TABLE templates (
  id              BIGINT AUTO_INCREMENT PRIMARY KEY,
  name            VARCHAR(64)  NOT NULL,
  label           VARCHAR(128) NOT NULL,
  description     VARCHAR(512) NOT NULL DEFAULT '',
  fields          JSON         NOT NULL,           -- [{field_id, required}, ...] 顺序=NPC 表单展示顺序
  ref_count       INT          NOT NULL DEFAULT 0, -- 被 NPC 引用数（NPC 模块未上线前恒为 0）
  enabled         TINYINT(1)   NOT NULL DEFAULT 0,
  version         INT          NOT NULL DEFAULT 1,
  deleted         TINYINT(1)   NOT NULL DEFAULT 0,
  created_at      DATETIME     NOT NULL,
  updated_at      DATETIME     NOT NULL,
  UNIQUE KEY uk_name (name),                        -- 含软删除记录
  INDEX idx_list (deleted, id, name, label, ref_count, enabled, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**关键设计**：

- `fields` JSON 数组的**顺序就是 NPC 表单展示顺序**——前端「上下移动」直接修改此数组顺序
- `idx_list` 是覆盖索引（不含 `fields / description`），列表 SQL `ORDER BY id DESC` 不回表
- `templates` 表**不持有 field_refs**，模板对字段的引用关系由 `field_refs(ref_type='template', ref_id=<template_id>)` 记录，跨模块维护

---

## API 接口

| Method | Path | 用途 |
|---|---|---|
| POST | `/api/v1/templates/list` | 列表（label 模糊 + enabled 三态 + 分页，按 id DESC） |
| POST | `/api/v1/templates/create` | 创建（跨模块事务：写 templates + field_refs + fields.ref_count）|
| POST | `/api/v1/templates/detail` | 详情（handler 拼装，不进缓存）|
| POST | `/api/v1/templates/update` | 编辑（跨模块事务：条件 Detach/Attach）|
| POST | `/api/v1/templates/delete` | 软删除（跨模块事务：FOR SHARE 防 TOCTOU + 批量 detach）|
| POST | `/api/v1/templates/check-name` | 唯一性校验（含软删除记录） |
| POST | `/api/v1/templates/references` | 被引用 NPC 详情（NPC 模块未上线前返回 `npcs: make([]...)` 空数组） |
| POST | `/api/v1/templates/toggle-enabled` | 启用 / 停用（单模块乐观锁） |

---

## 跨模块事务编排（Handler 层）

`TemplateService` **不持有** `FieldStore / FieldRefStore / FieldCache`，也不调 `FieldService`。所有跨模块操作——字段存在性校验、`field_refs` 维护、`fields.ref_count` 维护、字段侧缓存清理——都由 `TemplateHandler` 作为"用例编排者"显式调 `FieldService` 的对外方法。

### Create 流程

```
1. 格式校验
   - name 正则 + 长度
   - label 非空 + 长度
   - description ≤ 512
   - fields 非空且 field_id > 0 不重复

2. 事务外预校验
   - templateService.ExistsByName(name)                          → 41001
   - fieldService.ValidateFieldsForTemplate(fieldIDs)
       # 存在性 41006 / 启用 41005 / 非 reference 41012

3. tx := db.BeginTxx(ctx, nil); defer tx.Rollback()

4. templateService.CreateTx(tx, req)
   # Service 内再校验 fields 非空 41004 + name 唯一兜底 41001
   # INSERT templates (ref_count=0, enabled=false, version=1, deleted=0)

5. fieldService.AttachToTemplateTx(tx, tplID, fieldIDs)
   # 对每个 fieldID 写 field_refs(field_id, 'template', tplID) + IncrRefCountTx

6. tx.Commit()

7. Commit 后分别清缓存：
   - templateService.InvalidateList(ctx)
   - fieldService.InvalidateDetails(ctx, affected)
```

### Update 流程（字段变更时额外增量事务）

```
1. 格式校验（id / label / description / fields / version > 0）

2. 拿旧状态
   - old := templateService.GetByID(id)                           # 自身 Cache-Aside
   - oldEntries := templateService.ParseFieldEntries(old.Fields)

3. 事务外预校验（仅新增字段）
   - toAddPre := diffNewFieldIDs(oldEntries, req.Fields)
   - fieldService.ValidateFieldsForTemplate(toAddPre)             → 41005 / 41006 / 41012
   # toRemove 不校验启用或类型（存量不动）

4. tx := db.BeginTxx(ctx, nil); defer tx.Rollback()

5. fieldsChanged, toAdd, toRemove, err := templateService.UpdateTx(tx, req, old, oldEntries)
   # Service 内:
   #   - fields 基础校验 41004
   #   - old.Enabled != false                        → 41010
   #   - isFieldsChanged(oldEntries, req.Fields)     → bool
   #   - old.RefCount > 0 && fieldsChanged           → 41008
   #   - 计算 toAdd / toRemove (fieldsChanged=true 但 diff 为空时两者均空切片)
   #   - 序列化新 fields JSON + UpdateTx(WHERE id=? AND version=?)
   #   - 乐观锁 0 行 → 41011

6. if fieldsChanged && (len(toAdd) > 0 || len(toRemove) > 0):
   - fieldService.DetachFromTemplateTx(tx, id, toRemove)
   - fieldService.AttachToTemplateTx(tx, id, toAdd)
   # 纯排序 / 纯 required 变更时跳过这一步（fields JSON 已更新足矣，不操作 field_refs）

7. tx.Commit()

8. 清缓存
   - templateService.InvalidateDetail(id)
   - templateService.InvalidateList
   - 若 Detach: fieldService.InvalidateDetails(detachAffected)
   - 若 Attach: fieldService.InvalidateDetails(attachAffected)
```

### `isFieldsChanged` 粗粒度语义

```go
// service/template.go
func isFieldsChanged(old, new []model.TemplateFieldEntry) bool {
    if len(old) != len(new) { return true }
    for i := range old {
        if old[i].FieldID != new[i].FieldID { return true }    // 集合 or 顺序
        if old[i].Required != new[i].Required { return true }  // required
    }
    return false
}
```

**集合 / 顺序 / `required` 任一不同都算「变更」**。意味着 `ref_count > 0` 时，**纯调整 required 或纯重排序也会被 41008 拒绝**。这是有意设计：排序决定 NPC 表单展示顺序，required 决定 NPC 创建校验，两者虽对存量 NPC 无直接影响但语义上属于模板配置，被 NPC 引用后统一锁死。

### Delete 流程（FOR SHARE 防 TOCTOU）

```
1. Handler 校验 id > 0
2. tpl := templateService.GetByID(id)
   # tpl.Enabled != false                                      → 41009
3. fieldIDs := templateService.ParseFieldEntries(tpl.Fields)
4. tx := db.BeginTxx(ctx, nil); defer tx.Rollback()
5. refCount := templateService.GetRefCountForDeleteTx(tx, id)
   # SELECT ref_count FROM templates WHERE id=? AND deleted=0 FOR SHARE
   # 防 "前面查无引用后面被 NPC 创建引用" 竞态
   # refCount > 0                                              → 41007
6. templateService.SoftDeleteTx(tx, id)                          # UPDATE deleted=1
7. fieldService.DetachFromTemplateTx(tx, id, fieldIDs)           # 批量删 field_refs + Decr
8. tx.Commit()
9. 清两方缓存
```

### Detail 流程（Cache-Aside 裸行 + Handler 拼装）

```
handler.Get
  → service.GetByID(id)
      # 只缓存 *model.Template 裸行（未解析 fields JSON）
      # Cache-Aside + TryLock 分布式锁 + double-check + 空标记
  → service.ParseFieldEntries(tpl.Fields)   # []TemplateFieldEntry
  → fieldIDs := entries.map(e => e.FieldID)
  → lite := fieldService.GetByIDsLite(fieldIDs)
      # 按 fieldIDs 顺序对齐返回 []FieldLite
      # 内部用 DictCache 翻译 CategoryLabel
      # 缺失 ID 用零值占位（handler 识别后 slog.Warn 跳过）
  → 按 entries 顺序组装 []TemplateFieldItem
      # FieldID / Name / Label / Type / Category / CategoryLabel / Enabled / Required
  → 包装 TemplateDetail 返回
```

**为什么 `TemplateDetail` 不进缓存**：`FieldLite.Enabled` 反映字段**当前**状态，如果把拼装后的详情缓存到模板方，字段被停用时就得同时清模板详情缓存，耦合链太长。分层做法是：模板方缓存裸行（受字段写影响小），字段方有自己的 detail 缓存（受模板写影响大），拼装每次在 handler 层发生，两边命中各自的 cache，拼装开销极小。

---

## 缓存策略

| 层 | Key 形态 | TTL | 防护机制 |
|---|---|---|---|
| detail | `templates:detail:{id}` | 5min + 0-30s jitter | 分布式锁 `templates:lock:{id}`（3s）+ double-check + 空标记 |
| list | `templates:list:v{N}:{label}:{enabled}:{page}:{ps}` | 1min + 0-10s jitter | 版本号 `templates:list:version`，INCR 一次所有变体失效 |
| — | **TemplateDetail（拼装后）** | 不缓存 | handler 每次重新拼装 |

---

## Service 方法清单

| 方法 | 签名 | 说明 |
|---|---|---|
| `List` | `(ctx, q) → (*ListData, error)` | Cache-Aside + 分页参数校正 |
| `GetByID` | `(ctx, id) → (*Template, error)` | 裸行 Cache-Aside + 分布式锁防击穿 |
| `CreateTx` | `(ctx, tx, req) → (id, error)` | 事务内写 templates 行 + 校验 41001/41004 |
| `UpdateTx` | `(ctx, tx, req, old, oldEntries) → (fieldsChanged, toAdd, toRemove, error)` | 事务内更新 + 41010/41008/41011 + 计算 diff |
| `SoftDeleteTx` | `(ctx, tx, id) → error` | 事务内标记 deleted=1，不存在返回 41003 |
| `GetRefCountForDeleteTx` | `(ctx, tx, id) → (int, error)` | FOR SHARE 读锁查 `ref_count`，防 TOCTOU |
| `ToggleEnabled` | `(ctx, id, enabled, version) → error` | 单模块乐观锁 + 清自身缓存 |
| `CheckName` | `(ctx, name) → (*CheckNameResult, error)` | 封装 `ExistsByName` |
| `ExistsByName` | `(ctx, name) → (bool, error)` | 直查 store，含软删除 |
| `ParseFieldEntries` | `(raw json.RawMessage) → ([]TemplateFieldEntry, error)` | 解析 fields JSON（公开工具方法）|
| `GetByIDsLite` | `(ctx, ids) → ([]TemplateLite, error)` | 给字段引用详情跨模块补 label |
| `InvalidateDetail` / `InvalidateList` | `(ctx, ...)` | 缓存清理 |

---

## Store 方法清单

| 方法 | 说明 |
|---|---|
| `GetByID` | `SELECT ... WHERE id=? AND deleted=0` |
| `ExistsByName` | `SELECT COUNT(*) FROM templates WHERE name=?`（含软删除）|
| `List` | 覆盖索引 `ORDER BY id DESC`，不回表 |
| `CreateTx` | `INSERT` 初始化 `ref_count=0, enabled=0, version=1, deleted=0` |
| `UpdateTx` | `UPDATE ... WHERE id=? AND version=?`，rows=0 返回 `ErrVersionConflict` |
| `SoftDeleteTx` | `UPDATE deleted=1`，rows=0 返回 `ErrNotFound` |
| `ToggleEnabled` | 乐观锁 `UPDATE enabled=?, version=version+1 WHERE id=? AND version=?` |
| `GetRefCountTx` | `SELECT ref_count FROM templates WHERE id=? FOR SHARE` |
| `IncrRefCountTx` / `DecrRefCountTx` | 由 NPC 模块写 NPC 时调用（NPC 未上线前为空洞），事务内原子递增递减 |
| `GetByIDs` | 批量 `SELECT id, name, label` for `TemplateLite` |

---

## 不变量与陷阱

- **软删除 name 不复用**：`ExistsByName` 不过滤 `deleted`，曾经存在的 name 永远被占用（防 NPC 快照语义混乱）。
- **FOR SHARE 防 TOCTOU**：`Delete` 的 `GetRefCountForDeleteTx` 用共享锁防止"前面查 ref_count=0 后面被 NPC 创建引用"的竞态。
- **编辑限制**：`41010` 启用中禁止编辑 + `41008` 被引用时字段列表 / 顺序 / required 锁死（两个闸）。
- **乐观锁**：`Update` / `ToggleEnabled` `WHERE id=? AND version=?`，rows=0 → `41011`。
- **41012 的位置**：由 `FieldService.ValidateFieldsForTemplate` 抛出，但归在**模板段位**，因为它由模板管理页消费。这是跨模块错误码归属的约定。
- **fields 数组顺序即业务顺序**：纯排序变更 fieldsChanged=true，但 diff 为空切片（不操作 `field_refs`），Service 只更新 `fields` JSON，handler 跳过 Detach/Attach。
- **跨模块事务由模板 handler 开启**：ADMIN 是 HTTP 单体，单 `*sqlx.DB`，跨模块事务就是一次普通的 `BEGIN ... COMMIT`，不需要 2PC / TCC / Saga。详见 `dev-rules.md` 「跨模块事务的成本」。
- **引用详情占位**：NPC 模块未上线前 handler 返回 `npcs: make([]TemplateReferenceItem, 0)`，**用 `make` 而不是 `nil`**，避免 JSON 序列化成 `null`。

---

## 错误码

| 错误码 | 常量 | 抛出层 | 含义 |
|---|---|---|---|
| 41001 | `ErrTemplateNameExists` | Service.CreateTx | 模板标识已存在（含软删除） |
| 41002 | `ErrTemplateNameInvalid` | Handler | 模板标识格式不合法 |
| 41003 | `ErrTemplateNotFound` | Service.GetByID / SoftDeleteTx | 模板不存在 |
| 41004 | `ErrTemplateNoFields` | Service.CreateTx / UpdateTx | 未勾选任何字段 |
| 41005 | `ErrTemplateFieldDisabled` | FieldService.ValidateFieldsForTemplate | 勾选了停用字段 |
| 41006 | `ErrTemplateFieldNotFound` | 同上 | 勾选的字段不存在 |
| 41007 | `ErrTemplateRefDelete` | Handler.Delete（GetRefCountForDeleteTx 后）| 被 NPC 引用无法删除 |
| 41008 | `ErrTemplateRefEditFields` | Service.UpdateTx | 被 NPC 引用无法编辑字段列表（含排序 / required） |
| 41009 | `ErrTemplateDeleteNotDisabled` | Handler.Delete（GetByID 后）| 删除前必须先停用 |
| 41010 | `ErrTemplateEditNotDisabled` | Service.UpdateTx | 编辑前必须先停用 |
| 41011 | `ErrTemplateVersionConflict` | Service.UpdateTx / ToggleEnabled | 乐观锁版本冲突 |
| 41012 | `ErrTemplateFieldIsReference` | FieldService.ValidateFieldsForTemplate | 勾选了 reference 类型字段（必须展开为 leaf 子字段后加入）|

---

## 详细功能说明

每个 API 的场景、校验分层、完整调用链、NPC 引用详情的未来对接方案见同目录下 `features.md`（按"功能 X"编号展开）。本文档只覆盖架构层面的事实。
