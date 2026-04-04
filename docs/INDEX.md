# 文档索引

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
| [frontend-pitfalls.md](development/frontend-pitfalls.md) | 前端陷阱清单（JS 基础、Vue 3 响应式、Element Plus、Axios、Vite） | 编写前端代码时 |

## specs/ — 功能 Spec（需求 -> 设计 -> 任务）
| 目录 | 状态 | 内容概括 |
|------|------|----------|
| [specs/overall-architecture/](specs/overall-architecture/) | 已完成 | 整体架构设计 |
| [specs/config-export/](specs/config-export/) | 已完成 | 配置导出接口（供游戏服务端拉取） |

## 游戏服务端参考文档
| 文档 | 位置 | 何时查阅 |
|------|------|----------|
| 配置字段定义 | `../NPC-AI-Behavior-System-Server/docs/protocol.md` | 理解配置 JSON 结构时 |
| BB Key 定义 | `../NPC-AI-Behavior-System-Server/internal/core/blackboard/keys.go` | 构建 Key 下拉列表时 |
| BT 节点类型 | `../NPC-AI-Behavior-System-Server/internal/core/bt/registry.go` | 构建节点类型列表时 |
