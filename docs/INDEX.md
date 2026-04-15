# 文档索引

## architecture/ — 架构总览

| 文档 | 内容概括 | 何时查阅 |
|------|----------|----------|
| [overview.md](architecture/overview.md) | 架构分层 + 各层职责 + 技术栈 + 中间件 + 企业级保障 | 了解后端架构时 |
| [api-contract.md](architecture/api-contract.md) | 与游戏服务端的导出 API 契约（5 个接口 + JSON 格式 + 责任划分） | 开发导出接口 / 联调时 |
| [frontend-conventions.md](architecture/frontend-conventions.md) | 前端项目级约定：目录结构、布局 CSS、API 层、列表页/表单页代码模式、路由 | 新建前端模块时 |

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

## V3 规划 — v3-PLAN/

| 文档 | 内容概括 | 何时查阅 |
|------|----------|----------|
| [README.md](v3-PLAN/README.md) | V3 功能总览（页面清单 + 通用功能 + 通用延后功能） | 了解全局规划时 |

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

## 游戏服务端参考文档

| 文档 | 位置 | 何时查阅 |
|------|------|----------|
| BB Key 定义 | `../NPC-AI-Behavior-System-Server/internal/core/blackboard/keys.go` | BB Key 同步时 |
| BT 节点类型 | `../NPC-AI-Behavior-System-Server/internal/core/bt/registry.go` | 节点类型同步时 |

---

## 测试

| 文件 | 内容 | 用法 |
|------|------|------|
| [integration_test.sh](../tests/integration_test.sh) | 全方位集成测试（字段+模板+事件类型+攻击） | `bash tests/integration_test.sh` |
