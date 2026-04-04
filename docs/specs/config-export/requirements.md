# 需求分析：配置导出 API（config-export）

## 动机

游戏服务端架构调整——不再直接连 MongoDB 读取配置，改为启动时通过 HTTP API 从 ADMIN 平台一次性拉取全量配置。这样 MongoDB 只存在于 ADMIN 服务内部，游戏服务端零数据库依赖，部署拓扑更简单（服务端只需知道 ADMIN 的 HTTP 地址，不需要 MongoDB 连接串）。

**不做会怎样**：游戏服务端必须继续维护 MongoDB 连接配置，部署时 ADMIN 和服务端必须共享同一个 MongoDB 实例，增加了运维耦合。

## 优先级

**高** — 游戏服务端已在等这个接口才能完成 HTTPSource 的开发。这是两个项目之间的阻塞依赖。

## 预期效果

游戏服务端启动时，用环境变量 `NPC_ADMIN_API=http://<admin地址>:9821` 配置 ADMIN 地址，然后依次调用 4 个 GET 接口，获取全部配置加载到内存：

| 接口 | 返回 |
|------|------|
| `GET /api/configs/event_types` | `{"items": [{"name": "explosion", "config": {...}}, ...]}` |
| `GET /api/configs/npc_types` | `{"items": [{"name": "civilian", "config": {...}}, ...]}` |
| `GET /api/configs/fsm_configs` | `{"items": [{"name": "civilian", "config": {...}}, ...]}` |
| `GET /api/configs/bt_trees` | `{"items": [{"name": "guard/patrol", "config": {...}}, ...]}` |

**具体场景**：
1. 运营人员在 ADMIN 网页上创建/编辑配置（已有功能）
2. 游戏服务端重启时调用 `GET /api/configs/event_types` 等接口，拿到全量配置
3. 服务端将配置加载到内存，开始正常运行
4. 全程不需要 MongoDB 连接串

## 依赖分析

- **依赖已有功能**：Store 层的 `List(ctx, collection)` 方法（已实现），直接复用
- **谁依赖这个**：游戏服务端的 HTTPSource（正在开发，等这个接口）

## 改动范围

仅涉及后端，预估 2 个文件：

| 文件 | 改动 |
|------|------|
| `backend/internal/handler/config_export.go` | 新增文件，4 个 GET handler |
| `backend/internal/handler/router.go` | 注册 4 条新路由 |

不需要新增 service/validator/cache 层——导出接口是纯只读，直接调用 Store.List 即可，不涉及校验和缓存失效。现有管理接口（`/api/v1/`）的 List 已经走 Redis 缓存，但导出接口面向服务端启动（低频、一次性），不需要缓存。

## 扩展轴检查

- **新增配置类型**：如果未来新增配置类型（如 `dialogue_configs`），只需在 `config_export.go` 中复制一个 handler 函数 + router 加一行注册。符合"加一组 handler"的扩展模式 ✓
- **新增表单字段**：不涉及，导出接口透传 `{name, config}` 原始结构，字段增减对导出接口透明 ✓

## 验收标准

- **R1**：`GET /api/configs/event_types` 返回 HTTP 200，body 为 `{"items": [{name, config}, ...]}` 格式，包含 MongoDB 中所有事件类型
- **R2**：`GET /api/configs/npc_types` 同上，返回所有 NPC 类型
- **R3**：`GET /api/configs/fsm_configs` 同上，返回所有 FSM 状态机
- **R4**：`GET /api/configs/bt_trees` 同上，返回所有行为树
- **R5**：空 collection 返回 `{"items": []}`（不是 null）
- **R6**：config 字段为 JSON 对象，不是序列化后的字符串
- **R7**：接口支持 CORS（与现有管理接口一致），游戏服务端可能从不同主机调用
- **R8**：与现有管理接口（`/api/v1/`）互不影响，路径前缀用 `/api/configs/` 区分

## 不做什么

- **不做分页** — 配置量不大（每类 < 100 条），全量返回
- **不做认证** — 内网服务间调用，后续有需要再加 token
- **不做缓存** — 服务端只在启动时调一次，不会高频轮询
- **不做写入接口** — 配置管理仍然通过现有 `/api/v1/` 管理接口
- **不做前端改动** — 纯后端接口，前端无需感知
- **不做 WebSocket/推送通知** — 配置变更通知不在本需求范围内
