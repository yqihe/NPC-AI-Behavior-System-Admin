# 模板管理 — 后端设计

> 通用架构/规范见 `docs/architecture/overview.md` 和 `docs/development/`。
> 本文档只记录模板管理模块的实现事实与特有约束。

---

## 1. 目录结构

```
backend/internal/
├── handler/template.go          # HTTP 入口 + 跨模块事务编排 + 拼装 TemplateDetail
├── service/template.go          # 业务逻辑 + Cache-Aside（只缓存 *model.Template 裸行）+ 对外接口
├── store/
│   ├── mysql/template.go        # templates 表 CRUD + 覆盖索引 + 乐观锁
│   └── redis/template.go        # TemplateCache（Detail / List / Lock）
├── model/template.go            # Template / TemplateFieldEntry / TemplateListItem / TemplateDetail / TemplateFieldItem / TemplateLite / DTO
├── errcode/codes.go             # 41001-41012
└── router/router.go             # POST /api/v1/templates/* 路由注册
```

**存储范围**：

- **MySQL**：唯一写入目标
- **Redis**：`*model.Template` 裸行 detail 缓存 + 列表缓存 + 分布式锁
- **MongoDB / RabbitMQ**：不涉及（模板自身不产生导出数据，NPC 配置层才产生 `npc_templates` 集合）

---

## 2. 数据表

### templates

```sql
CREATE TABLE IF NOT EXISTS templates (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    name            VARCHAR(64)  NOT NULL,              -- 模板标识，唯一，创建后不可变
    label           VARCHAR(128) NOT NULL,              -- 中文标签（搜索用）
    description     VARCHAR(512) NOT NULL DEFAULT '',   -- 描述（可选）
    fields          JSON         NOT NULL,              -- [{field_id, required}, ...] 数组顺序=NPC 表单展示顺序

    ref_count       INT          NOT NULL DEFAULT 0,    -- 被 NPC 引用数（冗余计数，事务内维护）
    enabled         TINYINT(1)   NOT NULL DEFAULT 0,    -- 启用状态（创建默认 0）
    version         INT          NOT NULL DEFAULT 1,    -- 乐观锁
    deleted         TINYINT(1)   NOT NULL DEFAULT 0,    -- 软删除
    created_at      DATETIME     NOT NULL,
    updated_at      DATETIME     NOT NULL,

    UNIQUE KEY uk_name (name),
    INDEX idx_list (deleted, id, name, label, ref_count, enabled, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**索引说明**：

| 索引 | 用途 |
|---|---|
| `uk_name (name)` | 唯一约束，**不带 deleted**——已删除的 name 永久不可复用，防历史 NPC 引用混乱 |
| `idx_list (deleted, id, name, label, ref_count, enabled, created_at)` | 覆盖索引，列表 SQL `ORDER BY id DESC` 不回表（不含 `fields / description`） |

**字段引用关系**：

`templates` 表不持有 `field_refs`。模板对字段的引用关系由 `field_refs(ref_type='template', ref_id=<template_id>)` 记录，跨模块事务内由 handler 调 `FieldService.AttachToTemplateTx / DetachFromTemplateTx` 维护。

---

## 3. API 接口

所有操作使用主键 ID（BIGINT）。`name` 只出现在创建请求体/响应、`/check-name` 校验、跨模块 `GetByIDsLite` 返回值。

| Method | Path | 请求体 | 响应体 | 错误码 |
|---|---|---|---|---|
| POST | `/api/v1/templates/list` | `{label?, enabled?, page, page_size}` | `{items: TemplateListItem[], total, page, page_size}` | — |
| POST | `/api/v1/templates/create` | `{name, label, description?, fields: [{field_id, required}]}` | `{id, name}` | 41001, 41002, 41004, 41005, 41006, 41012 |
| POST | `/api/v1/templates/detail` | `{id}` | `TemplateDetail` | 41003 |
| POST | `/api/v1/templates/update` | `{id, label, description?, fields, version}` | `"保存成功"` | 41004, 41005, 41006, 41008, 41010, 41011, 41012 |
| POST | `/api/v1/templates/delete` | `{id}` | `{id, name, label}` | 41003, 41007, 41009 |
| POST | `/api/v1/templates/check-name` | `{name}` | `{available, message}` | 41002 |
| POST | `/api/v1/templates/references` | `{id}` | `{template_id, template_label, npcs: []}` | 41003 |
| POST | `/api/v1/templates/toggle-enabled` | `{id, enabled, version}` | `"操作成功"` | 41003, 41011 |

**跨模块事务编排**（handler 层开启事务，调 templateService + fieldService 协同完成）：

- **Create**：格式校验 -> `ExistsByName` -> `fieldService.ValidateFieldsForTemplate` -> tx: `CreateTx` + `AttachToTemplateTx` -> commit -> 清两方缓存
- **Update**：格式校验 -> `GetByID` + `ParseFieldEntries` -> 事务前预校验新增字段 -> tx: `UpdateTx`（enabled/ref_count/diff/写 templates）+ 条件 `DetachFromTemplateTx` / `AttachToTemplateTx` -> commit -> 清两方缓存
- **Delete**：`GetByID` + enabled 校验 -> `ParseFieldEntries` -> tx: `GetRefCountForDeleteTx`（FOR SHARE）+ `SoftDeleteTx` + `DetachFromTemplateTx` -> commit -> 清两方缓存
- **Detail**：`GetByID`（裸行 cache-aside）-> `ParseFieldEntries` -> `fieldService.GetByIDsLite` -> handler 拼装 `TemplateDetail`（不缓存拼装结果）

---

## 4. 缓存策略

| 层 | Key 模式 | TTL | 防护机制 |
|---|---|---|---|
| detail | `templates:detail:{id}` | 5min + 0-30s jitter | 分布式锁 `templates:lock:{id}`（3s）+ double-check + 空标记 `{"_null":true}` |
| list | `templates:list:v{N}:{label}:{enabled}:{page}:{ps}` | 1min + 0-10s jitter | 版本号 `templates:list:version`（INCR 一次所有变体失效） |
| 拼装后 TemplateDetail | **不缓存** | — | handler 每次从两方 cache 分别取裸行 + 字段精简后拼装 |

**不缓存 TemplateDetail 的原因**：`FieldLite.Enabled` 反映字段当前状态，如果缓存拼装后的详情到模板方，字段被停用时就得同时清模板详情缓存，耦合链太长。分层做法：模板方缓存裸行，字段方有自己的 detail 缓存，拼装在 handler 层每次发生。

**失效时机**：

| 操作 | 清模板 detail | 清模板 list | 清字段 details |
|---|---|---|---|
| Create | — | INCR version | affected fieldIDs |
| Update | DEL detail:{id} | INCR version | detach + attach affected |
| Delete | DEL detail:{id} | INCR version | affected fieldIDs |
| ToggleEnabled | DEL detail:{id} | INCR version | — |

---

## 5. 错误码

| 错误码 | 常量 | 触发场景 |
|---|---|---|
| 41001 | `ErrTemplateNameExists` | 创建时 name 已存在（含软删除记录，`ExistsByName` 不过滤 deleted） |
| 41002 | `ErrTemplateNameInvalid` | name 为空 / 不匹配 `^[a-z][a-z0-9_]*$` / 超长 |
| 41003 | `ErrTemplateNotFound` | `GetByID` / `SoftDeleteTx` / `GetRefCountForDeleteTx` 查不到未删除记录 |
| 41004 | `ErrTemplateNoFields` | 创建或编辑时 fields 数组为空 |
| 41005 | `ErrTemplateFieldDisabled` | `fieldService.ValidateFieldsForTemplate` 发现勾选了停用字段 |
| 41006 | `ErrTemplateFieldNotFound` | `fieldService.ValidateFieldsForTemplate` 发现勾选的字段不存在 |
| 41007 | `ErrTemplateRefDelete` | 删除时 `GetRefCountForDeleteTx`（FOR SHARE）发现 ref_count > 0 |
| 41008 | `ErrTemplateRefEditFields` | 编辑时 ref_count > 0 且 fields 变更（集合/顺序/required 任一不同） |
| 41009 | `ErrTemplateDeleteNotDisabled` | 删除时 `tpl.Enabled == true`，必须先停用 |
| 41010 | `ErrTemplateEditNotDisabled` | 编辑时 `old.Enabled == true`，必须先停用 |
| 41011 | `ErrTemplateVersionConflict` | `UpdateTx` / `ToggleEnabled` 乐观锁 `WHERE version=?` 命中 0 行 |
| 41012 | `ErrTemplateFieldIsReference` | `fieldService.ValidateFieldsForTemplate` 发现勾选了 reference 类型字段（必须展开为 leaf 子字段后加入） |
