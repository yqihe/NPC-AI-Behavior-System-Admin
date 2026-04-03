# 整体架构规划 — 需求分析

## 动机

运营管理平台是毕业设计的核心交付物之一。游戏服务端已完成，但配置（事件类型、NPC 类型、FSM、BT）目前只能通过手动编辑 JSON 文件写入 MongoDB，对策划/运营人员极不友好。不做此平台，策划无法独立管理游戏配置，毕设演示也缺少可视化操作环节。

## 优先级

**最高** — 这是项目当前阶段唯一的开发任务。前端和后端骨架搭建是所有后续功能的前置条件，必须先完成。

## 预期效果

做完后系统应达到以下状态：

1. **策划场景**：策划打开浏览器访问 `http://localhost:3000`，看到左侧导航菜单（事件管理 / NPC 管理 / 状态机管理 / 行为树管理），点击任一模块，看到配置列表页，可以新建、编辑、删除配置，所有操作通过表单完成，不接触 JSON。
2. **数据链路**：前端表单提交 → Go 后端 REST API → 校验通过 → 写入 MongoDB（`npc_ai` 库，`{name, config}` 格式） → 游戏服务端下次启动时自动加载生效。
3. **缓存链路**：列表查询走 Redis 缓存，写操作后自动失效对应缓存。
4. **容器化**：`docker compose up --build` 一键拉起 admin-backend + admin-frontend + Redis（MongoDB 由外部提供或 compose 中包含）。

## 依赖分析

| 依赖方向 | 依赖项 | 状态 |
|---------|--------|------|
| 本需求依赖 | MongoDB 数据格式（由游戏服务端定义） | ✅ 已确定，见 `configs/` 示例 |
| 本需求依赖 | 游戏服务端 collection 结构 `{name, config}` | ✅ 已确定 |
| 依赖本需求 | 后续所有功能模块的具体 UI 交互细化 | 等待本需求完成 |

## 改动范围

本需求从零搭建，涉及以下新增文件：

| 目录 | 预估文件数 | 说明 |
|------|-----------|------|
| `backend/cmd/admin/` | 1 | 程序入口 main.go |
| `backend/internal/handler/` | 5 | HTTP handler（路由 + 4 个模块） |
| `backend/internal/service/` | 4 | 业务逻辑层（4 个模块） |
| `backend/internal/store/` | 2 | MongoDB 数据访问（通用 store + 初始化） |
| `backend/internal/cache/` | 1 | Redis 缓存 |
| `backend/internal/validator/` | 4 | 配置校验器（4 个模块） |
| `backend/internal/model/` | 4 | 数据模型定义 |
| `backend/go.mod` | 1 | Go 模块 |
| `frontend/src/views/` | 4-8 | 页面组件 |
| `frontend/src/components/` | 3-5 | 通用组件 |
| `frontend/src/api/` | 4 | API 调用层 |
| `frontend/src/router/` | 1 | 路由配置 |
| `frontend/` | 3 | package.json, vite.config, index.html |
| 根目录 | 3 | docker-compose.yml, Dockerfile.backend, Dockerfile.frontend |

总计约 **40-50 个文件**。

## 扩展轴检查

`docs/architecture/extension-axes.md` 不存在（扩展轴是游戏服务端的概念）。运营管理平台的扩展性体现在：

- **新增配置类型**：当前架构应支持未来新增 collection 类型时，只需新增 handler/service/store/validator 一组文件，不改已有代码。
- **新增表单字段**：当游戏服务端在 config 中新增字段时，前端只需在对应表单中添加组件。

本需求的设计将确保这两个方向的扩展不需要修改已有模块。

## 验收标准

| 编号 | 验收标准 | 验证方式 |
|------|---------|---------|
| R1 | `docker compose up --build` 可一键启动 admin-backend、admin-frontend、redis 三个容器，启动后无报错 | 运行命令，检查容器状态 |
| R2 | 后端提供 4 组 REST API（event-types / npc-types / fsm-configs / bt-trees），每组支持 LIST / GET / CREATE / UPDATE / DELETE | curl 测试每个端点 |
| R3 | API 请求/响应格式：列表返回 `{items: [...]}` 数组，单条返回 `{name, config}` 对象，错误返回 `{error: "中文描述"}` | curl 检查响应格式 |
| R4 | 写入 MongoDB 的文档格式为 `{name: string, config: object}`，与游戏服务端完全兼容 | 通过 mongosh 查询验证 |
| R5 | 列表查询走 Redis 缓存，CREATE/UPDATE/DELETE 操作后对应缓存自动失效 | 检查 Redis 键存在/缺失 |
| R6 | 前端可访问（`http://localhost:3000`），展示左侧导航 + 4 个模块的列表页 | 浏览器打开验证 |
| R7 | 前端列表页可展示配置列表，支持新建、编辑、删除操作（通过表单，不暴露 JSON） | 浏览器操作验证 |
| R8 | 后端在保存时执行配置校验：FSM 转换引用的状态必须存在、BT 节点类型必须合法、必填字段不为空。校验失败返回中文错误提示 | 提交非法配置，检查错误响应 |
| R9 | API 响应中不暴露 MongoDB ObjectId，不暴露 Go error 堆栈 | 检查所有 API 响应 |
| R10 | Go nil slice 序列化为 `[]` 而非 `null` | 检查空列表 API 响应 |

## 不做什么

- **不做**用户认证 / 权限管理（红线：禁止过度设计）
- **不做**配置版本控制 / 回滚功能（红线：禁止过度设计）
- **不做**实时协作编辑（红线：禁止过度设计）
- **不做**审批工作流（红线：保存即生效）
- **不做**行为树可视化拖拽编辑器（后续需求，本期仅做表单化树结构编辑）
- **不做**条件构造器的高级 AND/OR 嵌套 UI（本期用简化的条件表单，后续迭代增强）
- **不做**国际化（仅中文界面）
- **不做**游戏服务端的 WebSocket 实时推送通知
