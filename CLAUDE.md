## CLAUDE.md

本文件为 Claude Code 提供项目上下文和开发指导。

## 项目概述

**项目名称**：NPC AI 行为系统 — 运营管理平台
**当前状态**：开发中
**项目性质**：毕业设计配套工具

**定位**：为策划/运营人员提供可视化配置管理界面，无需接触代码或 JSON 即可创建和管理 NPC 类型、事件类型、状态机、行为树等游戏配置。配置写入 MongoDB，游戏服务端下次启动时通过 HTTP API 拉取。

**姐妹项目**：
- 游戏服务端：`../NPC-AI-Behavior-System-Server/`（Go，已完成）
- Unity 客户端：`../NPC-AI-Behavior-System-Client/`（Unity C#，开发中）

## 技术栈

**后端**（Go）：
- 数据库：MongoDB（ADMIN 独占，游戏服务端通过导出接口获取配置）
- 缓存：Redis（缓存配置列表查询）
- API：RESTful HTTP
- 日志：Go 标准库 `log/slog`

**前端**：
- 框架：Vue 3 + Element Plus
- 构建：Vite
- 通信：Axios → REST API

**容器化**：Docker Compose（admin-backend + admin-frontend + Redis + MongoDB）

## 开发指令

```bash
# Docker Compose 启动全部服务（代码改动后加 --build）
docker compose up --build

# 后台启动
docker compose up --build -d

# 仅启动后端依赖（MongoDB + Redis）
docker compose up -d mongo redis

# 后端开发（本地运行）
cd backend && go run ./cmd/admin/

# 前端开发（本地运行）
cd frontend && npm run dev

# 运行后端测试
cd backend && go test ./...

# 停止
docker compose down
```

## 架构和约束

### 目录结构

```
backend/                   # Go 后端
  cmd/admin/               #   程序入口
  internal/
    handler/               #   HTTP handler（REST API）
    service/               #   业务逻辑（CRUD + 校验）
    store/                 #   MongoDB 数据访问
    cache/                 #   Redis 缓存
    validator/             #   配置校验器
frontend/                  # Vue 3 前端
  src/
    views/                 #   页面（事件管理/NPC管理/FSM编辑/BT编辑）
    components/            #   通用组件（条件构造器/树编辑器）
    api/                   #   REST API 调用
configs/                   # 参考配置（仅供参考，实际数据在 MongoDB）
docs/                      # 文档
  standards/               #   通用标准（跨项目复用）
  architecture/            #   项目架构约束
  development/             #   开发规范与陷阱
  specs/                   #   功能 Spec
Dockerfile.backend
Dockerfile.frontend
docker-compose.yml
```

### 命名约定

- Go 文件名：`snake_case.go`
- Go 包名：小写单词
- Vue 组件：`PascalCase.vue`
- REST API：`/api/v1/event-types`（kebab-case 复数）
- CSS 类名：`kebab-case`
- 配置文件：`snake_case.json`

### 代码风格

- `{name, config}` 文档结构由游戏服务端定义，ADMIN 不得修改
- 校验用结构体字段类型必须与游戏服务端一致（如 `default_severity` 为 `float64`）
- BB Key 白名单与游戏服务端 `blackboard/keys.go` 对齐，前端用下拉选择器

## 环境配置

### 开发环境（Docker Compose）
- **启动**：`docker compose up --build`
- **后端端口**：9821
- **前端端口**：3000
- **MongoDB**：localhost:27017，数据库 `npc_ai`
- **Redis**：localhost:6379

### 与游戏服务端联调
- 游戏服务端设置 `NPC_ADMIN_API=http://<admin地址>:9821`
- 配置导出接口：`GET /api/configs/{event_types,npc_types,fsm_configs,bt_trees}`
- 返回格式：`{"items": [{"name": "...", "config": {...}}, ...]}`

## Git 工作流

- **主分支**：`main`
- **功能分支**：`feature/<spec-name>`
- commit message 格式：`类型(范围): 描述`
  - 类型：`feat` / `fix` / `test` / `refactor` / `docs` / `chore`
  - 范围：`backend/handler`、`frontend/views`、`backend/store` 等

## 详细文档

详见 `docs/INDEX.md`，按需查阅。
