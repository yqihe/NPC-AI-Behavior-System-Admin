# field-expose-bb-column — 任务拆解

## 执行顺序

```
T1 → T2 → T3 → T4 → T5（验收）
```

---

## T1：migration — Drop+重建 fields 表

**涉及文件**（1 个）：
- `backend/migrations/009_fields_expose_bb.sql`（新建）

**做什么**：
- DROP TABLE IF EXISTS fields
- 重建 fields 表，新增 `expose_bb TINYINT(1) NOT NULL DEFAULT 0` 列
- 新增 `INDEX idx_expose_bb (expose_bb)`
- 保持其余列定义与 001_create_fields.sql 完全一致

**做完是什么样**：
- SQL 文件存在，可在 MySQL 中执行且无报错
- `SHOW CREATE TABLE fields` 包含 `expose_bb` 列和 `idx_expose_bb` 索引

**关联需求**：R1

---

## T2：model — 新增 ExposeBB 字段和 ExposesBB 过滤参数

**涉及文件**（1 个）：
- `backend/internal/model/field.go`（修改）

**做什么**：
- `Field` 结构体加 `ExposeBB bool \`db:"expose_bb" json:"expose_bb"\``（放在 `Enabled` 之前）
- `FieldListQuery` 加 `ExposesBB *bool \`json:"bb_exposed,omitempty" form:"bb_exposed"\``

**做完是什么样**：
- `go build ./...` 零错误（此步不会引入编译错误，下游 T3/T4 会）

**关联需求**：R2, R3

---

## T3：store/mysql — 同步写 expose_bb、读 SELECT 列、List 过滤

**涉及文件**（1 个）：
- `backend/internal/store/mysql/field.go`（修改）

**做什么**：

1. `Create`：INSERT 列表加 `expose_bb`，从 `req.Properties` 内联 Unmarshal 取 `ExposeBB`（失败时默认 false）
2. `Update` / `UpdateTx`：UPDATE SET 加 `expose_bb = ?`，同样内联取值
3. `GetByID` / `GetByName` / `GetByIDs` / `GetByNames`：SELECT 列表加 `expose_bb`
4. `List`：`if q.ExposesBB != nil { WHERE expose_bb = ? }`，args 追加 `*q.ExposesBB`

**做完是什么样**：
- `go build ./...` 零错误（T4 的 FieldListKey 签名改动会导致编译失败，T3 完成后先确认 store 本身无语法错误）

**关联需求**：R4, R5

---

## T4：cache key — FieldListKey 加 exposesBB 维度

**涉及文件**（2 个）：
- `backend/internal/store/redis/shared/keys.go`（修改）
- `backend/internal/store/redis/field_cache.go`（修改）

**做什么**：

1. `keys.go`：`FieldListKey` 函数签名加 `exposesBB *bool` 参数（放在 `enabled` 之后），格式串加 `:%s`
2. `field_cache.go`：`GetList` / `SetList` 调用 `FieldListKey` 时额外传 `q.ExposesBB`

**做完是什么样**：
- `go build ./...` 零错误
- `FieldListKey` 函数签名和所有调用点对齐

**关联需求**：R5（缓存 key 隔离，防止不同 bb_exposed 参数命中同一缓存条目）

---

## T5：验收 — 构建 + 手动 API 测试

**涉及文件**（0 个，仅执行命令）

**做什么**：

1. 执行 migration 009（在 MySQL 中执行 SQL 或通过 docker-compose 重建）
2. 运行 `go build ./...`，确认零错误
3. 启动后端，依次执行：

```
# 建两个字段：一个 expose_bb=true，一个 expose_bb=false
POST /api/v1/fields  {"name":"hp","label":"血量","type":"int","category":"combat",
                      "properties":{"expose_bb":true}}
POST /api/v1/fields  {"name":"level","label":"等级","type":"int","category":"combat",
                      "properties":{"expose_bb":false}}

# 验证过滤
GET /api/v1/fields?bb_exposed=true        → 仅返回 hp
GET /api/v1/fields?bb_exposed=false       → 仅返回 level
GET /api/v1/fields                        → 返回 hp 和 level
GET /api/v1/fields?bb_exposed=true&enabled=true  → 启用且 bb_exposed=true（hp 默认停用，应为空）
```

**做完是什么样**：
- `go build ./...` 零错误
- 过滤行为与预期完全一致

**关联需求**：R1–R7（全部）
