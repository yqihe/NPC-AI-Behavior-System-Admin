# ADMIN 运营平台 — 架构总览

> 本文档描述 ADMIN 后端的架构分层、各层职责、技术栈与中间件、企业级保障。
> 中间件使用原则/策略/常见问题见 `docs/development/` 下对应的 red-lines 和 dev-rules。
> 各模块的表设计/API/缓存/错误码见 `docs/v3-PLAN/{模块}/backend.md`。

---

## 1. 架构分层

```
handler/         ← HTTP 入口
service/         ← 业务逻辑
store/
  mysql/         ← MySQL 操作
  redis/         ← Redis 缓存操作
cache/           ← 进程内内存缓存
model/           ← 数据模型
errcode/         ← 错误码定义
router/          ← 路由注册
config/          ← 配置加载
util/            ← 通用常量与工具
```

---

## 2. 各层职责

| 层 | 职责 | 禁止 |
|---|---|---|
| **handler** | 请求参数绑定 + 格式校验 + **跨模块编排**（开事务、调多个 service、commit/rollback）+ 返回响应 | 不写业务逻辑 |
| **service** | 编排同模块内的 store/cache，处理业务逻辑、Cache-Aside、乐观锁映射 | 不调用其他模块的 store/cache/service |
| **store/mysql** | 单张表的 CRUD + 覆盖索引查询 | 不做业务判断 |
| **store/redis** | Redis 缓存读写 + key 生成 + 分布式锁 | 不做业务判断 |
| **cache** | 进程内内存缓存（DictCache、EventTypeSchemaCache），启动加载，运行时只读 | 只读基础设施，任意 service 可调用 |
| **model** | 数据模型定义（请求/响应/数据库行 struct） | 无逻辑 |
| **errcode** | 错误码常量 + 默认消息 + Error 类型 | 无逻辑 |

**跨模块编排规则**：

- handler 是"用例编排者"：当接口涉及多个模块（如模板创建要写 templates + 写 field_refs + 改 fields.ref_count）时，handler 调 `db.BeginTxx` 开事务，把 `*sqlx.Tx` 传给多个 service 的 Tx 方法，统一 commit/rollback
- service 之间零依赖：TemplateService 不知道 FieldStore/FieldCache 的存在
- 例外：`DictCache` 是只读基础设施，不算跨模块

**物理上**：ADMIN 是 HTTP 单体（非微服务），所有模块共享同一 `*sqlx.DB`。handler 层开的跨模块事务就是普通的 MySQL `BEGIN ... COMMIT`，不需要 2PC/TCC/Saga。

---

## 3. 技术栈

| 组件 | 选型 | 版本 |
|------|------|------|
| 语言 | Go | 1.21+ |
| HTTP 框架 | Gin | — |
| MySQL 驱动 | sqlx + go-sql-driver/mysql | — |
| Redis 客户端 | go-redis/v9 | — |
| 配置 | config.yaml + 环境变量覆盖 | — |
| 日志 | Go 标准库 `log/slog` | — |
| 前端 | Vue 3 + TypeScript + Element Plus + Vite | — |

---

## 4. 中间件职责

| 中间件 | 职责 | 什么数据存这里 |
|--------|------|--------------|
| **MySQL** | ADMIN 管理数据 + 搜索/筛选索引 + 元数据 | 运营看到的一切：字段、模板、事件类型、引用关系、字典 |
| **Redis** | 缓存 + 分布式锁 | detail 缓存、list 缓存、dict 缓存、防并发锁 |
| **MongoDB** | 游戏配置数据源 | 只存需要导出给游戏服务端的配置（npc_templates / event_types / fsm_configs / bt_trees / regions） |
| **RabbitMQ** | 异步跨库同步 | MySQL → MongoDB 的变更同步（需要跨库的场景） |

**判断数据存哪里**：游戏服务端启动时需要拉取 → MongoDB（`{name, config}` 格式）+ MySQL；不需要 → 只存 MySQL。

---

## 5. 统一响应格式

```json
{ "code": 0, "data": { ... }, "message": "success" }
{ "code": 40001, "data": null, "message": "该字段标识已存在" }
```

- `code=0` 成功，`code=4xxxx` 业务错误，`code=50000` 系统错误
- HTTP 状态码统一 200，业务错误码在 `code` 字段
- 列表返回：`{"code": 0, "data": {"items": [...], "total": N, "page": P, "page_size": S}}`
- 错误码定义在 `errcode/codes.go`，各模块码段见模块 backend.md

---

## 6. 企业级保障

| 指标 | 设计保障 |
|------|---------|
| **1000+ 配置** | 覆盖索引不回表，列表查询 < 1ms |
| **千级 QPS** | Redis 分页缓存 + 内存 map 翻译，热路径不查 MySQL |
| **多实例扩展** | 后端无状态，Redis 分布式锁，乐观锁防冲突 |
| **数据安全** | 软删除不丢数据，乐观锁防覆盖，version 只增不减 |
| **可观测** | slog 结构化日志，错误必记日志 |
| **优雅关闭** | 停止接受请求 → 等待进行中请求（30s）→ 关闭 MQ → 关闭 Redis → 关闭 MySQL → 关闭 MongoDB |

---

## 7. MySQL 设计原则

| 原则 | 说明 |
|------|------|
| 固定列 vs JSON 列 | 能搜索/筛选的 → 固定列（走索引）；只存取不搜索的 → JSON 列 |
| 覆盖索引 | 列表查询字段全部放联合索引，不回表 |
| 禁止 JOIN | 多次独立查询 + 代码层拼装 |
| 冗余计数 | `ref_count` 等冗余到主表，事务内维护 |
| 关系表 | 实体间引用用独立关系表 + 联合主键，不用 JSON_CONTAINS |
| 软删除 | `deleted TINYINT(1)`，唯一索引不含 deleted（软删名称仍占用唯一性） |
| 乐观锁 | `version INT`，`WHERE version = ?`，`affected_rows=0` 返回冲突错误 |

---

## 8. Redis 缓存架构

三类缓存：

| 类型 | Key 模式 | TTL | 失效方式 |
|------|---------|-----|---------|
| 字典（内存+Redis） | `dict:{group}` | 24h | 启动加载，变更时手动刷新 |
| 列表 | `{entity}:list:v{N}:{filters}:{page}:{ps}` | 60s + jitter | `INCR {entity}:list:version` |
| 详情 | `{entity}:detail:{id}` | 300s + jitter | 写后 DEL |

模式：Cache-Aside（先查 Redis → 未命中查 MySQL → 写 Redis；写 MySQL → 删 Redis）

防护：穿透（null marker）、击穿（分布式锁 `TryLock` + double-check）、雪崩（TTL + jitter）
