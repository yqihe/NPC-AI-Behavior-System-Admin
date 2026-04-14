## CLAUDE.md

本文件为 Claude Code 提供项目上下文和开发指导。

## 项目概述

**项目名称**：NPC AI 行为系统 — 运营管理平台
**当前状态**：V3 重写中（V2 代码已归档到 `v2-archive` 分支）
**项目性质**：毕业设计配套工具，但**对标企业级工程标准**

**定位**：为策划/运营人员提供可视化配置管理界面，无需接触代码或 JSON 即可创建和管理 NPC 字段、模板、事件类型、状态机、行为树、区域等游戏配置。所有配置写入 MySQL，游戏服务端启动时通过 HTTP 导出 API 一次性拉取。

**姐妹项目**：
- 游戏服务端：`../NPC-AI-Behavior-System-Server/`（Go）
- Unity 客户端：`../NPC-AI-Behavior-System-Client/`（Unity C#）

## 技术栈

**后端**（Go）：
- 数据库：MySQL（配置数据源、搜索索引、元数据、审计日志）
- 缓存：Redis（分页/单条/distinct 缓存 + 分布式锁）
- API：RESTful HTTP
- 日志：Go 标准库 `log/slog`

**前端**：
- 框架：Vue 3 + TypeScript + Element Plus
- 构建：Vite
- 通信：Axios → REST API
- 表单渲染：自研 SchemaForm（不使用第三方 JSON Schema 表单库）

**容器化**：Docker Compose（admin-backend + admin-frontend + MySQL + Redis）

## 企业级标准

- 数据量支持 1000+ 配置
- QPS 支持千级
- 后端无状态，支持多实例水平扩展
- 所有列表后端分页，不做前端全量过滤
- 所有下拉选项从数据库动态获取，不硬编码
- 组合搜索（多字段筛选 + 后端 MySQL 查询）
- 数据同步双层保障（同步写入 + 抽样校验）
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
    handler/               #   HTTP handler（纯业务 + wrap.go 泛型包装）
      shared/              #     层内工具（package shared，import alias shared）
        validate.go        #       请求校验辅助（CheckID/CheckName/CheckLabel/SuccessMsg）
    service/               #   业务逻辑（纯业务）
      shared/              #     层内工具（package shared，import alias shared）
        validate.go        #       值/约束校验（ValidateValue/ValidateConstraintsSelf/NormalizePagination）
        jsonutil.go        #       JSON 提取辅助（ParseConstraintsMap/GetFloat/GetString/...）
    store/
      mysql/               #   MySQL 操作（纯业务 CRUD）
        shared/            #     层内工具（package shared，import alias shared）
          sqlutil.go       #       SQL 辅助（EscapeLike/Is1062）
      redis/               #   Redis 缓存操作（纯业务 *_cache.go）
        shared/            #   Redis 专属常量 + key 管理（package shared，import alias rcfg）
    cache/                 #   内存缓存（字典/Schema 启动加载）
    config/                #   配置加载
    errcode/               #   错误码（业务码 + store 哨兵错误）
    model/                 #   数据模型
    router/                #   路由注册
    setup/                 #   统一聚合初始化（连接 + 分层注册）
    util/                  #   跨层共享常量（每层都可能引用）
      const.go             #     枚举/ref_type/字典组名（PerceptionMode/FieldType/RefType/DictGroup）
  migrations/              #   SQL DDL 脚本
frontend/
  src/
    views/                 #   页面
    components/            #   通用组件（SchemaForm/BtNodeEditor/ConditionEditor 等）
    api/                   #   REST API 调用
docs/                      # 文档
  architecture/            #   架构总览 + 游戏服务端 API 契约
  development/
    standards/             #   通用标准（跨项目）：red-lines/ + dev-rules/
    admin/                 #   ADMIN 项目专属：red-lines + dev-rules
  v3-PLAN/                 #   V3 需求规划文档（按模块分 features/backend/frontend）
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

### BB Key 同步

ADMIN 和游戏服务端各存各的 BB Key，不走 API 互拉。ADMIN 的 Key 来自字段标识（标记暴露的）+ 运行时 Key 表。

## 环境配置

### 开发环境（Docker Compose）
- **后端端口**：9821
- **前端端口**：3000
- **MySQL**：localhost:3306，数据库 `npc_ai_admin`
- **Redis**：localhost:6379

### 与游戏服务端联调
- 游戏服务端设置 `NPC_ADMIN_API=http://<admin地址>:9821`
- 导出接口：`GET /api/configs/{npc_templates,event_types,fsm_configs,bt_trees,regions}`
- 返回格式：`{"items": [{"name": "...", "config": {...}}, ...]}`
- 详见 `docs/architecture/api-contract.md`

## Git 工作流

- **主分支**：`main`
- **V2 归档**：`v2-archive`
- **功能分支**：`feature/<spec-name>`
- commit message 格式：`类型(范围): 描述`
  - 类型：`feat` / `fix` / `test` / `refactor` / `docs` / `chore`
  - 范围：`backend/handler`、`frontend/views`、`backend/store` 等

## 详细文档

详见 `docs/INDEX.md`，按需查阅。
