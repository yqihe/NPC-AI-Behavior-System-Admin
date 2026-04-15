# field-expose-bb-column — 设计方案

## 方案描述

### 核心思路

在 `fields` 表新增独立列 `expose_bb TINYINT(1) NOT NULL DEFAULT 0`，带普通索引。
写路径（Create / Update / UpdateTx）从 `req.Properties` 中解析 `ExposeBB` 并同步写入独立列；
读路径（List）在 `ExposesBB != nil` 时追加 `WHERE expose_bb = ?` 走索引过滤。

### 数据结构变更

**migration 009（Drop + 重建 `fields` 表，`field_refs` 不变）**

```sql
DROP TABLE IF EXISTS fields;
CREATE TABLE fields (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    name            VARCHAR(64)  NOT NULL,
    label           VARCHAR(128) NOT NULL,
    type            VARCHAR(32)  NOT NULL,
    category        VARCHAR(32)  NOT NULL,
    properties      JSON         NOT NULL,
    expose_bb       TINYINT(1)   NOT NULL DEFAULT 0,   -- ← 新增

    enabled         TINYINT(1)   NOT NULL DEFAULT 0,
    version         INT          NOT NULL DEFAULT 1,
    deleted         TINYINT(1)   NOT NULL DEFAULT 0,
    created_at      DATETIME     NOT NULL,
    updated_at      DATETIME     NOT NULL,

    UNIQUE KEY uk_name (name),
    INDEX idx_list (deleted, id, name, label, type, category, enabled, created_at),
    INDEX idx_expose_bb (expose_bb)                    -- ← 新增索引
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

> `field_refs` 与 `expose_bb` 无关，不重建。

**`model/field.go` 新增字段**

```go
// Field（全量结构体，用于 GetByID / GetByIDs 等返回）
type Field struct {
    // ... 现有字段 ...
    ExposeBB  bool   `db:"expose_bb"  json:"expose_bb"`
}

// FieldListQuery 新增过滤参数
type FieldListQuery struct {
    // ... 现有字段 ...
    ExposesBB *bool `json:"bb_exposed,omitempty" form:"bb_exposed"`
}
```

> `FieldListItem`（列表行）不加 `expose_bb`——列表页不需要展示此列，保持覆盖索引不回表原则。

### 各层改动

**store/mysql/field.go**

| 方法 | 改动 |
|------|------|
| `Create` | INSERT 语句加 `expose_bb` 列，值取 `req.Properties` 中解析的 `ExposeBB` |
| `Update` / `UpdateTx` | UPDATE SET 加 `expose_bb = ?`，值取 `req.Properties` 中解析的 `ExposeBB` |
| `GetByID` / `GetByName` / `GetByIDs` / `GetByNames` | SELECT 列表加 `expose_bb` |
| `List` | `if q.ExposesBB != nil { WHERE expose_bb = ? }` |

`expose_bb` 值提取：在 `Create`/`Update` 内做一次 `json.Unmarshal` 到 `model.FieldProperties`，取 `.ExposeBB`。已有 `parseProperties` 工具函数在 `service` 层，store 层直接内联一行 json.Unmarshal（与 service 层无耦合，store 不依赖 service）。

**store/redis/shared/keys.go**

```go
func FieldListKey(version int64, typ, category, label string,
    enabled *bool, exposesBB *bool, page, pageSize int) string {
    return fmt.Sprintf("%sv%d:%s:%s:%s:%s:%s:%d:%d",
        prefixFieldList, version, typ, category, label,
        boolStr(enabled), boolStr(exposesBB), page, pageSize)
}
```

**store/redis/field_cache.go**

`GetList` / `SetList` 调用 `FieldListKey` 时额外传 `q.ExposesBB`。

**service/field.go**

无需修改——`List` 已直接把 `q` 传给 `fieldStore.List` 和 `fieldCache.GetList/SetList`，
新参数随结构体透传，零改动。

**handler/field.go**

无需修改——`FieldListQuery` 已通过 Gin `ShouldBindQuery` 绑定，新增 `form:"bb_exposed"` tag 后
Gin 自动解析 `?bb_exposed=true` 查询参数，无需手写绑定代码。

## 方案对比

| 维度 | A（独立列 + 索引，本方案） | B（JSON_EXTRACT WHERE 子句） |
|------|--------------------------|------------------------------|
| 查询性能 | O(log N) 索引扫描 | O(N) 全表扫（无法走索引） |
| 改动量 | 4 个文件，约 30 行 | 2 个文件，约 5 行 |
| 数据一致性 | 写路径显式同步 | 天然一致（单一 JSON 源） |
| 企业级合规 | 符合「后端分页、索引过滤」原则 | 违反高并发下性能标准 |
| 迁移成本 | Drop+重建（开发阶段无历史数据） | 无迁移 |

**选 A**：B 方案违反项目「1000+ 数据/千级 QPS」企业级要求，且随字段量增长无法横向扩展。

## 红线检查

| 红线文档 | 条目 | 检查结果 |
|---------|------|---------|
| admin/red-lines.md §10 | store: Create/Update 用 `*model.XxxRequest` 结构体 | ✓ 保持不变 |
| admin/red-lines.md §10 | service: 缓存读取 `err == nil && hit` | ✓ 无改动 |
| admin/red-lines.md §16 | DelDetail/InvalidateList 必须在 Commit 前 | ✓ 无改动 |
| admin/red-lines.md §11 | store 层不依赖 service 层（分层倒置禁止） | ✓ store 内联 json.Unmarshal，不 import service |
| go.md | nil slice → `make([]T, 0)` | ✓ List 现有模式不变 |
| mysql.md | LIKE 走 EscapeLike | ✓ 不涉及 LIKE |
| mysql.md | 禁止事务内混用 tx/db | ✓ UpdateTx 内已全程用 tx |
| go.md | 禁止 `json.Unmarshal` 到 `any` 后假设数字类型 | ✓ 解析目标是具体的 `model.FieldProperties` |
| admin/red-lines.md §4 | 禁止硬编码，DB 字段统一 `*sqlx.DB` | ✓ 无新硬编码 |

**无红线违反。**

## 扩展性影响

- 「新增配置类型」扩展轴：不涉及，本次只改 field 模块自身列表查询
- 「新增表单字段」扩展轴：正面示范——新增一个可过滤列只改 4 个文件，handler 零感知

## 依赖方向

```
handler/field.go
    ↓
service/field.go
    ↓              ↘
store/mysql/       store/redis/
field.go           field_cache.go
                       ↓
                   store/redis/shared/keys.go

model/field.go  ← 所有层共用
```

单向向下，无循环。`store/mysql` 不 import `service`（store 内联一行 json.Unmarshal，无需 parseProperties）。

## 陷阱检查

参照 `docs/development/standards/dev-rules/` 中 Go / MySQL 规范：

1. **store 内 json.Unmarshal 失败处理**：`Create`/`Update` 若 `json.Unmarshal` 失败，`ExposeBB` 默认为 `false`（Go 零值），不影响主流程但 expose_bb 列会写 0。此行为与 service 层 `parseProperties` 一致（失败时用零值继续），acceptable。

2. **列表覆盖索引**：现有 `idx_list` 覆盖索引包含 `(deleted, id, name, label, type, category, enabled, created_at)`，加 `expose_bb` 过滤时会先走 `idx_expose_bb` 再回行或走 MySQL 优化器选择。`expose_bb` 选择性低（仅 0/1），优化器在大部分场景优先 `idx_list`；需测试确认，但不影响正确性。

3. **FieldListKey 签名变更**：只有一处调用方（`field_cache.go`），编译器会直接报错强制更新，无遗漏风险。

4. **Drop+重建顺序**：`field_refs` 有 `field_id` 列但无外键约束（物理 FK 已放弃，用应用层保证），DROP `fields` 表不会级联影响 `field_refs`，安全。

## 配置变更

无。不涉及 `config.yaml` 任何字段。

## 测试策略

**构建验证（必做）**：`go build ./...` 零错误。

**手动 API 测试**（migration 执行后）：

1. 创建一个 `expose_bb=true` 的字段，调 `GET /api/v1/fields?bb_exposed=true`，确认该字段出现在结果中
2. 创建一个 `expose_bb=false` 的字段，调 `GET /api/v1/fields?bb_exposed=true`，确认该字段不出现
3. 调 `GET /api/v1/fields`（不传 `bb_exposed`），确认两个字段均出现
4. 调 `GET /api/v1/fields?bb_exposed=true&enabled=true`，确认仅返回启用且暴露 BB 的字段

> 单元测试留待行为树/NPC 模块再统一补全（毕设阶段，测试资源集中在集成测试）。
