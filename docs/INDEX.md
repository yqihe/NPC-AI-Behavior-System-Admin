# 文档索引

## V3 规划 — v3-PLAN/

| 文档 | 内容概括 | 何时查阅 |
|------|----------|----------|
| [README.md](v3-PLAN/README.md) | V3 功能总览（页面清单 + 通用功能 + 通用延后功能） | 了解全局规划时 |
| [api-contract.md](v3-PLAN/api-contract.md) | ADMIN 与游戏服务端的导出 API 契约（5 个接口 + JSON 格式） | 开发导出接口 / 联调时 |

### 配置管理
| 页面 | features | frontend | backend |
|------|----------|----------|---------|
| 字段管理 | [features.md](v3-PLAN/配置管理/字段管理/features.md) | [frontend.md](v3-PLAN/配置管理/字段管理/frontend.md) | [backend.md](v3-PLAN/配置管理/字段管理/backend.md) |
| 模板管理 | [features.md](v3-PLAN/配置管理/模板管理/features.md) | [frontend.md](v3-PLAN/配置管理/模板管理/frontend.md) | [backend.md](v3-PLAN/配置管理/模板管理/backend.md) |
| NPC 管理 | [features.md](v3-PLAN/配置管理/NPC管理/features.md) | [frontend.md](v3-PLAN/配置管理/NPC管理/frontend.md) | [backend.md](v3-PLAN/配置管理/NPC管理/backend.md) |

### 行为管理
| 页面 | features | frontend | backend |
|------|----------|----------|---------|
| 事件类型 | [features.md](v3-PLAN/行为管理/事件类型/features.md) | [frontend.md](v3-PLAN/行为管理/事件类型/frontend.md) | [backend.md](v3-PLAN/行为管理/事件类型/backend.md) |
| 状态机 | [features.md](v3-PLAN/行为管理/状态机/features.md) | [frontend.md](v3-PLAN/行为管理/状态机/frontend.md) | [backend.md](v3-PLAN/行为管理/状态机/backend.md) |
| 行为树 | [features.md](v3-PLAN/行为管理/行为树/features.md) | [frontend.md](v3-PLAN/行为管理/行为树/frontend.md) | [backend.md](v3-PLAN/行为管理/行为树/backend.md) |

### 世界管理
| 页面 | features | frontend | backend |
|------|----------|----------|---------|
| 区域管理 | [features.md](v3-PLAN/世界管理/区域管理/features.md) | [frontend.md](v3-PLAN/世界管理/区域管理/frontend.md) | [backend.md](v3-PLAN/世界管理/区域管理/backend.md) |

### 系统设置
| 页面 | features | frontend | backend |
|------|----------|----------|---------|
| Schema 管理 | [features.md](v3-PLAN/系统设置/Schema管理/features.md) | [frontend.md](v3-PLAN/系统设置/Schema管理/frontend.md) | [backend.md](v3-PLAN/系统设置/Schema管理/backend.md) |
| 导出管理 | [features.md](v3-PLAN/系统设置/导出管理/features.md) | [frontend.md](v3-PLAN/系统设置/导出管理/frontend.md) | [backend.md](v3-PLAN/系统设置/导出管理/backend.md) |

---

## standards/ — 通用标准（跨项目复用）

按技术领域拆分的禁止红线：

| 文档 | 技术领域 | 何时查阅 |
|------|----------|----------|
| [red-lines.md](standards/red-lines.md) | 通用（静默降级、安全、测试、过度设计、协作） | 所有开发活动 |
| [go-red-lines.md](standards/go-red-lines.md) | Go 语言（资源泄漏、序列化、错误处理、字符串、包设计、shutdown） | 编写 Go 代码时 |
| [mysql-red-lines.md](standards/mysql-red-lines.md) | MySQL（事务一致性、LIKE 注入） | 编写 MySQL 查询时 |
| [redis-red-lines.md](standards/redis-red-lines.md) | Redis（SCAN 禁用、DEL 错误检查） | 编写 Redis 操作时 |
| [cache-red-lines.md](standards/cache-red-lines.md) | 缓存模式（Cache-Aside、失效策略、穿透/雪崩） | 设计缓存逻辑时 |
| [frontend-red-lines.md](standards/frontend-red-lines.md) | 前端（数据源污染、无效输入、URL 编码） | 编写前端代码时 |

## architecture/ — ADMIN 项目专属约束

| 文档 | 内容概括 | 何时查阅 |
|------|----------|----------|
| [backend-red-lines.md](architecture/backend-red-lines.md) | 后端架构（数据格式、引用完整性、REST API、硬编码、过度设计） | 编写或审查后端代码时 |
| [ui-red-lines.md](architecture/ui-red-lines.md) | UI/UX（技术暴露、表单友好） | 编写或审查前端页面时 |

## development/ — 开发规范与陷阱

| 文档 | 技术领域 | 何时查阅 |
|------|----------|----------|
| [dev-rules.md](development/dev-rules.md) | 协作流程、Git、CRUD、Docker | 所有开发活动 |
| [go-pitfalls.md](development/go-pitfalls.md) | Go 语言（JSON/BSON、HTTP、错误处理、数据结构、字符串、包设计） | 编写 Go 代码时 |
| [mysql-pitfalls.md](development/mysql-pitfalls.md) | MySQL（事务与锁、查询优化） | 编写 MySQL 查询时 |
| [mongodb-pitfalls.md](development/mongodb-pitfalls.md) | MongoDB（连接、操作、集成测试） | 编写 MongoDB 操作时 |
| [redis-pitfalls.md](development/redis-pitfalls.md) | Redis（操作注意事项、分布式锁） | 编写 Redis 操作时 |
| [cache-pitfalls.md](development/cache-pitfalls.md) | 缓存模式（Cache-Aside、穿透、雪崩、失效策略） | 设计缓存逻辑时 |
| [frontend-pitfalls.md](development/frontend-pitfalls.md) | 前端（JS 基础、Vue 3、Element Plus、Axios、Router） | 编写前端代码时 |

## 游戏服务端参考文档
| 文档 | 位置 | 何时查阅 |
|------|------|----------|
| BB Key 定义 | `../NPC-AI-Behavior-System-Server/internal/core/blackboard/keys.go` | BB Key 同步时 |
| BT 节点类型 | `../NPC-AI-Behavior-System-Server/internal/core/bt/registry.go` | 节点类型同步时 |
