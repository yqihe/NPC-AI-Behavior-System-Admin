# 文档索引

## architecture/ — 架构与约束
| 文档 | 内容概括 | 何时查阅 |
|------|----------|----------|
| [red-lines.md](architecture/red-lines.md) | 禁止事项（暴露技术细节、破坏数据格式、安全隐患、过度设计） | 编写或审查代码时 |

## development/ — 开发规范
| 文档 | 内容概括 | 何时查阅 |
|------|----------|----------|
| [dev-rules.md](development/dev-rules.md) | 日志格式、文档同步、Git 规则、前后端协作、继承的经验教训 | 所有开发活动 |

## 游戏服务端参考文档
| 文档 | 位置 | 何时查阅 |
|------|------|----------|
| 配置字段定义 | `../NPC-AI-Behavior-System-Server/docs/protocol.md` | 理解配置 JSON 结构时 |
| MongoDB 数据模型 | `../NPC-AI-Behavior-System-Server/docs/specs/mongo-source/design.md` | 理解 collection 结构时 |
| 配置文件示例 | `../NPC-AI-Behavior-System-Server/configs/` | 了解各类配置的具体格式时 |
| BB Key 定义 | `../NPC-AI-Behavior-System-Server/internal/core/blackboard/keys.go` | 构建条件编辑器的 Key 下拉列表时 |
| BT 节点类型 | `../NPC-AI-Behavior-System-Server/internal/core/bt/registry.go` | 构建 BT 编辑器的节点类型列表时 |
