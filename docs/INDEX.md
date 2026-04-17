# 文档索引

## development/ — 开发规范

### standards/ — 通用标准（跨项目复用）

#### red-lines/（禁止红线）

| 文档 | 技术领域 | 何时查阅 |
|------|----------|----------|
| [general.md](development/standards/red-lines/general.md) | 通用（静默降级、安全、测试、过度设计、协作） | 所有开发活动 |
| [go.md](development/standards/red-lines/go.md) | Go 语言（资源泄漏、序列化、错误处理、字符串） | 编写 Go 代码时 |
| [mysql.md](development/standards/red-lines/mysql.md) | MySQL（事务一致性、LIKE 注入） | 编写 MySQL 查询时 |
| [redis.md](development/standards/red-lines/redis.md) | Redis（SCAN 禁用、DEL 错误检查） | 编写 Redis 操作时 |
| [cache.md](development/standards/red-lines/cache.md) | 缓存模式（Cache-Aside、失效策略、穿透/雪崩） | 设计缓存逻辑时 |
| [frontend.md](development/standards/red-lines/frontend.md) | 前端（数据源污染、无效输入、URL 编码） | 编写前端代码时 |

#### dev-rules/（开发规范 + 常见陷阱）

| 文档 | 技术领域 | 何时查阅 |
|------|----------|----------|
| [go.md](development/standards/dev-rules/go.md) | Go 语言（JSON/BSON、HTTP、错误处理、字符串、包设计） | 编写 Go 代码时 |
| [mysql.md](development/standards/dev-rules/mysql.md) | MySQL（事务与锁、查询优化、迁移管理） | 编写 MySQL 查询时 |
| [mongodb.md](development/standards/dev-rules/mongodb.md) | MongoDB（连接、操作、集成测试） | 编写 MongoDB 操作时 |
| [redis.md](development/standards/dev-rules/redis.md) | Redis（操作注意事项、分布式锁） | 编写 Redis 操作时 |
| [cache.md](development/standards/dev-rules/cache.md) | 缓存模式（Cache-Aside、穿透、雪崩、失效策略） | 设计缓存逻辑时 |
| [frontend.md](development/standards/dev-rules/frontend.md) | 前端（JS 基础、Vue 3、Element Plus、Axios、Router） | 编写前端代码时 |

### admin/ — ADMIN 项目专属

| 文档 | 内容概括 | 何时查阅 |
|------|----------|----------|
| [red-lines.md](development/admin/red-lines.md) | 后端架构禁令 + UI/UX 禁令（数据格式、引用完整性、硬编码、表单友好） | 编写或审查代码时 |
| [dev-rules.md](development/admin/dev-rules.md) | 分层职责、需求流程、Git、CRUD、Docker、**测试环境重置 + 测试脚本编写规范** | 所有开发活动 |

---