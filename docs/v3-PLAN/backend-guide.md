# V3 后端通用指南

> 所有页面后端开发共用的技术选型、架构规范、存储设计原则、缓存策略、常见问题解决方案。
> 各页面独有的表设计、API、业务逻辑见各自的 backend.md。

---

## 技术选型

### 框架：Gin

不用 Go-zero、Kratos 等微服务框架。原因：

- ADMIN 是**单体应用**，不需要服务发现、RPC、熔断等微服务基础设施
- 水平扩展靠 Docker 多实例 + Nginx 负载均衡，不需要微服务注册中心
- Gin 轻量、高性能、生态成熟，满足千级 QPS
- CLAUDE.md 已确定 RESTful HTTP 架构

### 存储职责划分

| 组件 | 职责 | 什么数据存这里 |
|------|------|--------------|
| **MongoDB** | 游戏配置数据源 | 只存需要导出给游戏服务端的配置（npc_templates / event_types / fsm_configs / bt_trees / regions） |
| **MySQL** | ADMIN 管理数据 | 运营看到的一切 + 搜索筛选索引 + 元数据 + 审计日志 |
| **Redis** | 缓存 + 分布式锁 | dictionaries 缓存、分页缓存、单条缓存、防并发锁 |
| **RabbitMQ** | 异步同步 | 需要跨库同步的场景（MySQL → MongoDB 或反向） |

**判断数据存哪里的原则：**

- 问自己："游戏服务端启动时需要拉这个数据吗？"
  - 需要 → MongoDB（`{name, config}` 格式）+ MySQL（管理字段）
  - 不需要 → 只存 MySQL

---

## 架构分层

```
handler/         ← HTTP 入口：参数绑定、校验、调 service、返回响应
service/         ← 业务逻辑：引用检查、权限校验、事务编排
store/
  mongo/         ← MongoDB 操作（只有需要导出的实体用）
  mysql/         ← MySQL 操作（所有实体都用）
cache/           ← Redis 缓存操作
model/           ← 数据模型定义（struct）
validator/       ← 校验规则
mq/              ← RabbitMQ 生产者/消费者（跨库同步用）
```

**职责边界：**

- handler 不写业务逻辑，只做参数校验 + 调 service + 返回
- service 不写 SQL/MongoDB 查询，只调 store
- store 不做业务判断，只封装数据操作
- cache 被 service 调用，store 不直接操作缓存

---

## MySQL 设计原则

### 建表原则

**1. 固定列 vs JSON 列**

```
能搜索/筛选的 → 固定列（走索引）
只存取不搜索的 → JSON 列（properties）
```

固定列是表的骨架，不会频繁变更。JSON 列承载动态属性，新增属性 = JSON 里多一个 key，不改表结构。

**2. 覆盖索引**

列表查询返回的字段全部放在一个联合索引里，MySQL 直接从索引返回数据，不回表读数据页。

```sql
-- 列表查询只需要这些列
SELECT id, name, label, type, category, ref_count, created_at FROM xxx;

-- 覆盖索引包含所有这些列
INDEX idx_list (deleted, id, name, label, type, category, ref_count, created_at)
```

单条详情需要 JSON 列（properties），走唯一索引回表，单条回表无影响。

**3. 禁止 JOIN**

所有关联查询拆成多次独立查询 + 代码层拼装：

```go
// ✗ 禁止
SELECT f.*, t.label FROM fields f JOIN templates t ON ...

// ✓ 正确
refs := store.GetFieldRefs("health")                    // 查引用关系
templates := store.GetTemplatesByNames(refs.TemplateNames) // IN 查模板
// 代码里拼装返回结果
```

每次查询都走索引，代码层 map 拼装，性能比 JOIN 更可控。

**4. 冗余计数**

列表页需要显示的关联计数（如"被引用数"），冗余到主表做固定列。修改时事务内维护。避免列表查询时 N 次额外 COUNT。

**5. 关系表替代 JSON_CONTAINS**

实体间引用关系用独立的关系表 + 联合主键，不用在 JSON 列上做 `JSON_CONTAINS` 查询。

```sql
-- ✗ JSON_CONTAINS 无法走索引，全表扫描
WHERE JSON_CONTAINS(properties->'$.refs', '"health"')

-- ✓ 关系表，联合主键索引
SELECT ref_name FROM field_refs WHERE field_name = 'health'
```

**6. 软删除**

所有实体用 `deleted TINYINT(1)` 软删除。唯一索引不含 deleted 列（软删除的名称仍占用唯一性）。

**7. 乐观锁**

所有可编辑实体加 `version INT`，更新时 `WHERE version = ? AND ... SET version = version + 1`。`affected_rows = 0` 时返回版本冲突错误。

### MySQL 常见问题

| 问题 | 解决方案 |
|------|---------|
| **慢查询** | 覆盖索引避免回表；LIKE '%xx%' 在千级数据量可接受，万级以上迁移 ES |
| **JSON 列查询慢** | 不在 WHERE 中使用 JSON 函数。JSON 列只做存取，筛选走固定列 |
| **大事务锁表** | 批量操作逐条处理（每条独立事务），不在一个大事务里锁多行 |
| **乐观锁 ABA** | version 只增不减，用 `UPDATE ... WHERE version = ?` |
| **连接池耗尽** | `SetMaxOpenConns(50)` + `SetMaxIdleConns(10)` + `SetConnMaxLifetime(5min)` |
| **deadlock** | 批量操作逐条事务；同一事务内按固定顺序锁行 |
| **字符集** | 全部 `utf8mb4`，支持 emoji 和特殊字符 |

---

## MongoDB 设计原则

### 文档格式

所有导出给游戏服务端的配置统一 `{name, config}` 格式：

```json
{
  "name": "wolf_common",
  "config": { ... }
}
```

`config` 内部结构由游戏服务端定义，ADMIN 不擅自添加字段。ADMIN 私有数据用独立 collection。

### MongoDB 常见问题

| 问题 | 解决方案 |
|------|---------|
| **连接泄漏** | shutdown 时显式 `client.Disconnect()`；cursor 用 `defer cursor.Close(context.Background())` |
| **cursor.Close context** | 不复用查询超时 context，用 `context.Background()` 关闭。查询耗时接近超时时 Close 会因 context 到期失败 |
| **数字类型** | `json.Unmarshal` 到 `any` 后数字是 `float64` 不是 `int`。配置中 `severity: 0` 不加 `omitempty` |
| **bson tag** | struct 同时写 `json` 和 `bson` tag，漏写 bson 导致字段名变大写开头 |
| **nil slice** | `var s []T` 序列化为 JSON `null`，前端 `v-for` 报错。必须 `make([]T, 0)` |
| **操作符注入** | 查询条件禁止接受用户输入的 `$` 开头 key |
| **超时** | 所有操作带超时 context，禁止 `context.Background()` 直接查库 |

---

## Redis 缓存策略

### 三类缓存

**1. dictionaries 缓存（长期）**

```
Key:     dict:{group_name}
Type:    Hash
TTL:     24h（兜底），变更时手动刷新
用途:    所有下拉选项、类型翻译
```

同时在后端维护内存 map（`typeMap["integer"] = "整数"`），Redis 挂了也能用。

**2. 分页缓存（短期）**

```
Key:     {entity}:list:{筛选条件哈希}:{page}:{page_size}
Type:    String (JSON)
TTL:     60s + rand(0,10)s（防雪崩）
刷新:    该实体 CRUD 时 DEL {entity}:list:*
```

**3. 单条缓存（中期）**

```
Key:     {entity}:detail:{name}
Type:    String (JSON)
TTL:     300s + rand(0,30)s
刷新:    该实体更新/删除时 DEL
```

### 缓存模式：Cache-Aside

```
读: 先查 Redis → 命中返回 → 未命中查 MySQL → 写入 Redis → 返回
写: 先写 MySQL → 成功后删 Redis → 删失败记日志告警
```

### Redis 常见问题

| 问题 | 解决方案 |
|------|---------|
| **缓存穿透**（查不存在的 key） | 空结果也缓存 60s，写入 `{"null":true}` |
| **缓存击穿**（热点 key 过期瞬间并发） | 分布式锁 `SETNX {entity}:lock:{name}`，拿到锁的查 DB，其他等待 |
| **缓存雪崩**（大量 key 同时过期） | TTL 加随机抖动 |
| **缓存一致性** | Cache-Aside，先写 DB 后删缓存。删缓存失败必须记日志告警 |
| **大 Key** | 分页缓存每页独立 key，不缓存全量列表 |
| **热 Key** | dictionaries 用本地内存 map 兜底 |
| **连接泄漏** | shutdown 时 `client.Close()`；所有操作带超时 context |
| **批量删除** | Pipeline 减少 RTT |
| **TTL 必设** | 所有 key 必须有 TTL，即使是"永久"缓存也设 24h 兜底 |

---

## 统一响应格式

```json
// 成功
{ "code": 0, "data": { ... }, "message": "success" }

// 失败
{ "code": 40001, "data": null, "message": "该字段标识已存在" }

// 列表
{
  "code": 0,
  "data": {
    "items": [...],
    "total": 100,
    "page": 1,
    "page_size": 20
  }
}
```

- 错误码 4xxxx 为业务错误，5xxxx 为系统错误
- 对外返回中文提示，Go error 原文写 slog（不暴露给前端）
- 各页面的错误码在各自 backend.md 中定义，编号不冲突

---

## Graceful Shutdown

```
1. 停止接受新 HTTP 请求（server.Shutdown）
2. 等待进行中请求完成（context 超时 30s）
3. 关闭 RabbitMQ 连接（停止消费）
4. 关闭 Redis 连接
5. 关闭 MySQL 连接池
6. 关闭 MongoDB 连接
```

顺序不可颠倒。先停流量，再关依赖。

---

## 企业级保障

| 指标 | 设计保障 |
|------|---------|
| **1000+ 配置** | 覆盖索引不回表，列表查询 < 1ms |
| **千级 QPS** | Redis 分页缓存 + 内存 map 翻译，热路径不查 MySQL |
| **多实例扩展** | 后端无状态，Redis 分布式锁，乐观锁防冲突 |
| **数据安全** | 软删除不丢数据，乐观锁防覆盖 |
| **可观测** | slog 结构化日志，错误必记日志 |
