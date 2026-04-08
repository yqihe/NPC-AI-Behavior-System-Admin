# 文档索引

## V3 规划 — v3-PLAN/

| 文档 | 内容概括 | 何时查阅 |
|------|----------|----------|
| [README.md](v3-PLAN/README.md) | V3 功能总览（页面清单 + 通用功能 + 通用延后功能） | 了解全局规划时 |
| [backend-guide.md](v3-PLAN/backend-guide.md) | 通用后端指南（Gin + MySQL/MongoDB/Redis 设计原则 + 常见问题） | 所有后端开发 |
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
| 文档 | 内容概括 | 何时查阅 |
|------|----------|----------|
| [red-lines.md](standards/red-lines.md) | 通用禁止红线（静默降级、安全、测试、过度设计、协作） | 所有开发活动 |
| [go-red-lines.md](standards/go-red-lines.md) | Go 语言禁止红线（资源泄漏、序列化、错误处理、shutdown） | 编写 Go 代码时 |
| [frontend-red-lines.md](standards/frontend-red-lines.md) | 前端禁止红线（数据源污染、无效输入、URL 编码） | 编写前端代码时 |

## architecture/ — 项目架构与约束
| 文档 | 内容概括 | 何时查阅 |
|------|----------|----------|
| [red-lines.md](architecture/red-lines.md) | ADMIN 项目禁止红线（技术暴露、数据格式、缓存、引用完整性、UX） | 编写或审查 ADMIN 代码时 |

## development/ — 开发规范
| 文档 | 内容概括 | 何时查阅 |
|------|----------|----------|
| [dev-rules.md](development/dev-rules.md) | 协作流程、日志格式、文档同步、Git 规则、CRUD 规则、Docker | 所有开发活动 |
| [go-pitfalls.md](development/go-pitfalls.md) | Go 陷阱清单（JSON/BSON、HTTP Handler、MongoDB/Redis、错误处理） | 编写 Go 代码时 |
| [frontend-pitfalls.md](development/frontend-pitfalls.md) | 前端陷阱清单（JS 基础、Vue 3 响应式、Element Plus、Axios、Router） | 编写前端代码时 |

## 游戏服务端参考文档
| 文档 | 位置 | 何时查阅 |
|------|------|----------|
| BB Key 定义 | `../NPC-AI-Behavior-System-Server/internal/core/blackboard/keys.go` | BB Key 同步时 |
| BT 节点类型 | `../NPC-AI-Behavior-System-Server/internal/core/bt/registry.go` | 节点类型同步时 |
