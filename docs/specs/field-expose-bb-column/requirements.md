# field-expose-bb-column — 需求分析

> **来源**：从 `docs/specs/fsm-config-frontend/requirements.md` R22 分出。
> FSM Config 前端的 BBKeySelector 需要按 `expose_bb=true` 过滤字段列表，但 `expose_bb` 当前
> 埋在 `properties` JSON 列中，无法走索引过滤，因此先做此后端预处理，完成后再回 fsm-config-frontend。

## 动机

字段表 `fields` 中每条记录的 `properties` 列是 JSON blob，其中 `expose_bb` 布尔值标记该字段
是否暴露给游戏服务端 BB（Blackboard）系统。当前后端 `FieldListQuery` 不支持按 `expose_bb`
筛选，只能把所有字段全量拉出来再在内存/前端过滤——违反「所有列表后端分页，不做前端全量过滤」红线。

不做的后果：
- BBKeySelector 无法高效拉取 bb_exposed 字段，FSM Config 前端 R22 被阻塞
- 随着字段数量增长，全量拉取性能劣化，且前端内存过滤规则分散难维护

## 优先级

**高**。是 fsm-config-frontend（当前最高优先级模块）的直接后端前置条件。

## 预期效果

**场景 1：BBKeySelector 拉取候选 Key**
1. 前端调 `GET /api/v1/fields?bb_exposed=true&enabled=true&page=1&page_size=100`
2. 后端在 MySQL 走索引过滤 `expose_bb=1 AND enabled=1`，返回分页结果
3. BBKeySelector 展示这些字段的 `name` 作为候选 Key

**场景 2：字段创建/更新时同步**
1. 策划在字段表单中勾选「暴露给 BB」
2. 后端 Create/Update 把 `properties.expose_bb` 同时写入独立列 `expose_bb`
3. 两处保持一致，`properties` 保留完整 JSON 供其他用途

## 依赖分析

**依赖（已完成）：**
- `fields` 表及全套 CRUD（已实现）
- migration 体系（001–008 已有，本次建 009）
- `backend/internal/model/field.go`、`store/mysql/field.go`、`service/field.go`

**被依赖（尚未开始）：**
- `docs/specs/fsm-config-frontend` BBKeySelector（R22）

## 改动范围

| 文件 | 类型 | 说明 |
|------|------|------|
| `backend/migrations/009_fields_expose_bb.sql` | 新建 | Drop + 重建 `fields` 表，新增 `expose_bb` 列及索引 |
| `backend/internal/model/field.go` | 修改 | `Field` 增加 `ExposeBB` 列；`FieldListQuery` 增加 `ExposesBB *bool` |
| `backend/internal/store/mysql/field.go` | 修改 | `Create`/`Update` 写入 `expose_bb`；`List` 支持 `expose_bb` WHERE + 查询列 |
| `backend/internal/service/field.go` | 修改 | 透传 `ExposesBB` 参数（如有缓存 key，需加入维度） |

共 4 个文件，1 新建 3 修改。handler 层无需改动（参数透传已有机制）。

## 扩展轴检查

- **新增配置类型**：不涉及
- **新增表单字段**：本次在已有的 `fields` 表新增一个可查询列，属于「新增字段」扩展轴——只改 model/store/service 三层，handler 透传，符合单向扩展原则

## 验收标准

- R1：`fields` 表新增 `expose_bb TINYINT(1) NOT NULL DEFAULT 0`，有索引 `idx_fields_expose_bb`
- R2：`Field` 结构体新增 `ExposeBB bool \`db:"expose_bb" json:"expose_bb"\``
- R3：`FieldListQuery` 新增 `ExposesBB *bool`，`List` 接口支持 `?bb_exposed=true/false` 查询参数
- R4：`store.Create` / `store.Update` 把 `properties.ExposeBB` 同步写入 `expose_bb` 列
- R5：`store.List` 在 `ExposesBB != nil` 时追加 `WHERE expose_bb = ?`，走索引
- R6：`go build ./...` 零错误；现有字段管理相关接口行为不变（无 `bb_exposed` 参数时返回全部）
- R7：migration 脚本可幂等执行（DROP TABLE IF EXISTS → CREATE TABLE）

## 不做什么

- **不做**前端改动（BBKeySelector 在 fsm-config-frontend spec 中实现）
- **不做** `expose_bb` 缓存独立维度（缓存 key 已按 query 全参数散列，自动兼容）
- **不做**历史数据回填脚本（migration DROP+重建即重置，开发阶段无历史数据）
- **不做** handler 层改动（`FieldListQuery` binding 已支持任意新增字段透传）
