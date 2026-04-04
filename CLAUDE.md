## CLAUDE.md

本文件为 Claude Code 提供项目上下文和开发指导。

## 项目概述

**项目名称**：NPC AI 行为系统 — 运营管理平台
**项目性质**：毕业设计配套工具
**当前状态**：初始开发

**定位**：为策划/运营人员提供可视化的配置管理界面，无需接触代码或 JSON 即可创建和管理 NPC 类型、事件类型、状态机、行为树等游戏配置。配置写入 MongoDB，游戏服务端下次启动时自动加载。

**姐妹项目**：
- 游戏服务端：`../NPC-AI-Behavior-System-Server/`（Go，已完成）
- Unity 客户端：`../NPC-AI-Behavior-System-Client/`（Unity C#，开发中）

## 用户角色

| 角色 | 技术水平 | 使用方式 |
|------|---------|---------|
| 策划/运营 | 非技术 | 通过 UI 组件（下拉框、滑块、单选、拖拽）创建配置 |
| 服务端开发 | 技术 | 通过表单化编辑器添加高级配置（有模板、校验、自动补全） |

## 技术栈

**后端**：
- 语言：Go
- 数据库：MongoDB（ADMIN 独占，游戏服务端通过导出接口获取配置）
- 缓存：Redis（缓存配置列表查询，减少 MongoDB 压力）
- API：RESTful HTTP
- 日志：Go 标准库 `log/slog`

**前端**：
- 框架：Vue 3 + Element Plus（表单组件丰富，适合管理后台）
- 构建：Vite
- 通信：Axios → REST API

**容器化**：Docker Compose（admin-backend + admin-frontend + Redis）

## 核心功能模块

### 1. 事件类型管理
- 列表展示所有事件类型
- 创建/编辑事件：名称输入、威胁等级滑块(0-100)、持续时间滑块(1-60s)、传播方式单选(视觉/听觉/全局)、传播范围滑块(0-1000m)
- 删除事件（需确认）
- 配置预览（JSON 预览，只读）

### 2. NPC 类型管理
- 列表展示所有 NPC 类型
- 创建/编辑 NPC：名称输入、感知范围滑块(视觉/听觉)、状态机选择(下拉已有 FSM)、各状态行为树选择(下拉已有 BT)
- 删除 NPC 类型（需确认）

### 3. FSM 状态机编辑
- 状态列表管理（添加/删除状态）
- 转换规则表单：来源状态(下拉) → 目标状态(下拉)、优先级(数字输入)、触发条件(条件构造器)
- 条件构造器：BB Key(下拉) + 运算符(下拉) + 值(输入/下拉)，支持 AND/OR 组合
- 初始状态选择（下拉）

### 4. BT 行为树编辑
- 节点类型选择（复合节点：顺序/选择/并行；装饰节点：反转inverter；叶子节点：检查BB/设置BB/动作占位）
- 复合节点使用 `children`（数组），装饰节点使用 `child`（单个子节点对象）——与游戏服务端 TreeConfig 结构对齐
- 参数表单（每种节点类型对应不同参数表单）
- 树结构可视化（树形图展示父子关系）
- 从已有模板创建

### 5. 配置校验
- 提交前自动检查：
  - FSM：状态引用合法性、状态名不重复、条件 op 操作符枚举校验（==/!=/>/>=/</<=/in）、condition 叶子与组合互斥
  - BT：节点类型和结构校验、BB key 白名单校验（防止未注册 key 导致服务端 panic）、stub_action result 枚举校验（success/failure/running）、parallel policy 校验
- 校验用结构体字段类型必须与游戏服务端一致（如 `default_severity` 为 float64，不是 int）
- BB Key 白名单与游戏服务端 `blackboard/keys.go` 对齐，前端用下拉选择器（不允许手动输入）
- 错误提示友好化（不暴露技术术语）

## 架构概览

```
┌─────────────────────┐     ┌─────────────────────┐
│   Vue 3 前端         │     │   游戏服务端          │
│   (Element Plus UI)  │     │   (启动时拉取配置)    │
│   端口: 3000         │     └─────────┬───────────┘
└─────────┬───────────┘               │ GET /api/configs/*
          │ REST API                   │
┌─────────▼───────────────────────────▼┐     ┌─────────┐
│   Go 后端                             │────→│  Redis  │ 缓存
│   端口: 9821                          │     └─────────┘
└─────────┬────────────────────────────┘
          │ 读写
┌─────────▼───────────┐
│     MongoDB          │
│   端口: 27017        │
└─────────────────────┘
```

## MongoDB 数据模型

与游戏服务端共享同一个 database（`npc_ai`），同一套 collection：

| Collection | 文档结构 | 说明 |
|------------|---------|------|
| `event_types` | `{name, config}` | 事件类型配置 |
| `npc_types` | `{name, config}` | NPC 类型配置 |
| `fsm_configs` | `{name, config}` | FSM 状态机配置 |
| `bt_trees` | `{name, config}` | BT 行为树配置 |

`name` 是唯一键。`config` 内容与游戏服务端的 JSON 配置格式完全一致。

详细字段定义见游戏服务端文档：`../NPC-AI-Behavior-System-Server/docs/protocol.md` 和各 `configs/*.json` 示例文件。

## 开发指令

```bash
# Docker Compose 启动全部服务（代码改动后加 --build）
docker compose up --build

# 后台启动
docker compose up --build -d

# 仅启动后端依赖（MongoDB + Redis）
docker compose up -d mongo redis

# 后端开发（本地运行）
cd backend && go run main.go

# 前端开发（本地运行）
cd frontend && npm run dev

# 运行后端测试
cd backend && go test ./...

# 停止
docker compose down
```

## 目录结构

```
backend/                   # Go 后端
  cmd/admin/               #   程序入口
  internal/
    handler/               #   HTTP handler（REST API）
    service/               #   业务逻辑（CRUD + 校验）
    store/                 #   MongoDB 数据访问
    cache/                 #   Redis 缓存
    validator/             #   配置校验器
  go.mod
frontend/                  # Vue 3 前端
  src/
    views/                 #   页面（事件管理/NPC管理/FSM编辑/BT编辑）
    components/            #   通用组件（条件构造器/树编辑器）
    api/                   #   REST API 调用
  package.json
configs/                   # 示例配置（从游戏服务端复制，用于参考）
docs/                      # 文档
  architecture/
  development/
  specs/
Dockerfile.backend         # 后端多阶段构建
Dockerfile.frontend        # 前端 nginx 构建
docker-compose.yml         # 服务编排
```

## 与游戏服务端的关系

- **配置导出**：游戏服务端启动时通过 HTTP API 从 ADMIN 拉取全量配置（不直接连 MongoDB）
- **单向依赖**：游戏服务端依赖 ADMIN 的导出接口，ADMIN 不依赖游戏服务端
- **更新流程**：运营平台改配置 → 发停服公告 → 到点停服 → 重启游戏服务端（自动拉取最新配置）→ 新配置生效
- **Schema 约定**：游戏服务端定义配置格式（collection 结构、字段含义），运营平台遵循

### 配置导出接口

供游戏服务端启动时一次性拉取全量配置，只读、不分页、不认证。

| 接口 | 返回格式 |
|------|---------|
| `GET /api/configs/event_types` | `{"items": [{"name": "...", "config": {...}}, ...]}` |
| `GET /api/configs/npc_types` | 同上 |
| `GET /api/configs/fsm_configs` | 同上 |
| `GET /api/configs/bt_trees` | 同上 |

游戏服务端用环境变量 `NPC_ADMIN_API=http://<admin地址>:9821` 配置 ADMIN 地址。

## 命名约定

- Go 文件名：`snake_case.go`
- Go 包名：小写单词
- Vue 组件：`PascalCase.vue`
- REST API：`/api/v1/event-types`（kebab-case 复数）
- CSS 类名：`kebab-case`

## 详细文档

详见 `docs/INDEX.md`，按需查阅。