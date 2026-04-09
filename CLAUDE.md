## CLAUDE.md

本文件为 Claude Code 提供项目上下文和开发指导。

## 项目概述

**项目名称**：NPC AI 行为系统 — 运营管理平台
**当前状态**：V3 重写中（V2 代码已归档到 `v2-archive` 分支）
**项目性质**：毕业设计配套工具，但**对标企业级工程标准**

**定位**：为策划/运营人员提供可视化配置管理界面，无需接触代码或 JSON 即可创建和管理 NPC 字段、模板、事件类型、状态机、行为树、区域等游戏配置。所有配置写入 MongoDB，游戏服务端启动时通过 HTTP 导出 API 一次性拉取。

**姐妹项目**：
- 游戏服务端：`../NPC-AI-Behavior-System-Server/`（Go）
- Unity 客户端：`../NPC-AI-Behavior-System-Client/`（Unity C#）

## 技术栈

**后端**（Go）：
- 数据库：MongoDB（配置数据源，游戏服务端通过导出 API 获取）+ MySQL（搜索索引、元数据、审计日志）
- 缓存：Redis（分页/单条/distinct 缓存 + 分布式锁）
- 消息队列：RabbitMQ（MongoDB→MySQL 异步同步）
- API：RESTful HTTP
- 日志：Go 标准库 `log/slog`

**前端**：
- 框架：Vue 3 + TypeScript + Element Plus
- 构建：Vite
- 通信：Axios → REST API
- 表单渲染：自研 SchemaForm（不使用第三方 JSON Schema 表单库）

**容器化**：Docker Compose（admin-backend + admin-frontend + MongoDB + MySQL + Redis + RabbitMQ）

## 企业级标准

- 数据量支持 1000+ 配置
- QPS 支持千级
- 后端无状态，支持多实例水平扩展
- 所有列表后端分页，不做前端全量过滤
- 所有下拉选项从数据库动态获取，不硬编码
- 组合搜索（多字段筛选 + 后端 MySQL 查询）
- 数据同步三层保障（同步写入 + MQ 重试 + 抽样校验）
- 乐观锁防并发冲突
- 审计日志（谁改了什么）

## 架构和约束

### 目录结构（V3 规划）

```
backend/
  cmd/
    admin/                 #   API 服务入口
    seed/                  #   字典种子脚本
  internal/
    handler/               #   HTTP handler（REST API）
    service/               #   业务逻辑
    store/
      mysql/               #   MySQL 操作（字段/字典/引用关系）
      redis/               #   Redis 缓存 + key 管理
    cache/                 #   内存缓存（字典启动加载）
    config/                #   配置加载
    errcode/               #   错误码定义
    model/                 #   数据模型
    router/                #   路由注册
  migrations/              #   SQL DDL 脚本
frontend/
  src/
    views/                 #   页面
    components/            #   通用组件（SchemaForm/BtNodeEditor/ConditionEditor 等）
    api/                   #   REST API 调用
docs/                      # 文档
  standards/               #   通用标准红线（Go/MySQL/Redis/缓存/前端）
  architecture/            #   ADMIN 项目专属约束（后端架构/UI-UX）
  development/             #   开发规范与陷阱（Go/MySQL/MongoDB/Redis/缓存/前端）
  v3-PLAN/                 #   V3 需求规划文档
Dockerfile.backend
Dockerfile.frontend
docker-compose.yml
```

### 命名约定

- Go 文件名：`snake_case.go`
- Go 包名：小写单词
- Vue 组件：`PascalCase.vue`
- REST API：`/api/v1/fields`（kebab-case 复数）
- CSS 类名：`kebab-case`

### 核心数据模型

三层配置模型：
- **字段**（原子单位）：定义 NPC 可以有什么属性
- **模板**（字段组合）：把字段组合成可复用的模板
- **NPC**（模板实例）：选模板填值，创建具体 NPC

行为配置独立于字段系统：
- **事件类型**：游戏世界中的事件定义
- **状态机（FSM）**：NPC 状态和转换条件
- **行为树（BT）**：NPC 在每个状态下的行为逻辑
- **区域**：游戏场景配置

### MongoDB 数据格式

所有配置文档统一 `{name, config}` 格式。详见 `docs/api-contract.md`。

### BB Key 同步

ADMIN 和游戏服务端各存各的 BB Key，不走 API 互拉。ADMIN 的 Key 来自字段标识（标记暴露的）+ 运行时 Key 表。

## 环境配置

### 开发环境（Docker Compose）
- **后端端口**：9821
- **前端端口**：3000
- **MongoDB**：localhost:27017，数据库 `npc_ai`
- **MySQL**：localhost:3306，数据库 `npc_ai_admin`
- **Redis**：localhost:6379
- **RabbitMQ**：localhost:5672（AMQP），15672（Management UI）

### 与游戏服务端联调
- 游戏服务端设置 `NPC_ADMIN_API=http://<admin地址>:9821`
- 导出接口：`GET /api/configs/{npc_templates,event_types,fsm_configs,bt_trees,regions}`
- 返回格式：`{"items": [{"name": "...", "config": {...}}, ...]}`
- 详见 `docs/api-contract.md`

## Git 工作流

- **主分支**：`main`
- **V2 归档**：`v2-archive`
- **功能分支**：`feature/<spec-name>`
- commit message 格式：`类型(范围): 描述`
  - 类型：`feat` / `fix` / `test` / `refactor` / `docs` / `chore`
  - 范围：`backend/handler`、`frontend/views`、`backend/store` 等

## Claude Code 权限模式

每个 SKILL 对应推荐的权限模式，Claude 在调用 SKILL 前应提醒用户切换：

| SKILL | 推荐模式 | 原因 |
|-------|----------|------|
| `/spec-create` | `plan` | 只读分析，不该写代码 |
| `/spec-execute` | `auto` | 写代码，allow 列表自动执行 |
| `/verify` | `auto` | 跑构建/测试命令 |
| `/debug` | `auto` | 需要读写代码修复 |
| `/integration` | `ask` | 跨项目操作，需确认每步 |
| 普通对话 | `ask` | 讨论功能，避免误操作 |

切换方式：`/mode auto` / `/mode plan` / `/mode ask`

## 详细文档

详见 `docs/INDEX.md`，按需查阅。
