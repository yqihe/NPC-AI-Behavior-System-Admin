# 文档索引

## V3 规划
| 文档 | 内容概括 | 何时查阅 |
|------|----------|----------|
| [V3_PLAN.md](V3_PLAN.md) | V2 经验总结 + V3 架构设计（中间件/分层/同步/缓存） | V3 开发前 |
| [v3-pages.md](v3-pages.md) | V3 页面清单（9 个页面的第一级广度需求） | 确定每个页面深入功能时 |
| [api-contract.md](api-contract.md) | ADMIN 与游戏服务端的导出 API 契约（5 个接口 + JSON 格式） | 开发导出接口 / 联调时 |

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
