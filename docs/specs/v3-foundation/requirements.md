# 需求 0：旧代码清理 + V3 基础准备

## 动机

项目从 V2（硬编码字段）升级到 V3（Schema 驱动 + 组件化 NPC）。旧数据已清空（MongoDB 4 个集合已 drop，configs/ 下 16 个 JSON 已删），但旧的硬编码代码仍在——4 个 validator、4 个 handler、4 个 service 全部绑定固定字段结构。

**不做会怎样**：后续 6 个需求（动态表单、组件化 NPC、区域管理、搜索、侧边栏重构、FSM/BT Schema 化）全部无法启动，因为现有代码骨架不支持动态 schema，改任何一个需求都要先拆旧代码。

## 优先级

**最高**。这是所有 V3 需求的前置依赖，阻塞全部后续开发。

## 预期效果

完成后系统状态：

1. **后端**：一个通用的 schema 驱动的 CRUD 框架已就位。新增实体类型只需往 MongoDB 插入一条 schema 文档 + 注册一个路由前缀，不用写新的 handler/service/validator 代码
2. **前端**：旧的硬编码页面已清除，新的路由骨架和布局结构已搭建，准备好接入动态表单组件
3. **依赖库**：Go JSON Schema 校验库已引入并验证可用；前端 JSON Schema 表单渲染库已引入并验证可用
4. **Docker 环境**：无变化，`docker compose up --build` 正常启动，各服务健康
5. **空白但可运行**：系统可以启动、侧边栏可以看到新的分组结构、页面为空白占位状态，API 返回空列表

### 具体场景

- 运营人员打开 ADMIN 平台 → 看到新的侧边栏分组（配置管理 / 世界管理 / 系统设置）→ 点击任何页面 → 看到空白占位页（"暂无数据，待接入动态表单"）
- 开发者调用 `GET /api/v1/component-schemas` → 返回 `{"items": []}` 空列表
- 开发者调用 `GET /api/v1/npc-presets` → 返回 `{"items": []}` 空列表
- 后端启动日志显示 schema 校验库加载成功
- `go test ./...` 全部通过

## 依赖分析

### 前置依赖
- 无。旧数据已清空，可直接开工

### 谁依赖本需求
- 需求 1（Schema 驱动动态表单）：依赖本需求搭建的通用校验器和动态表单骨架
- 需求 2（NPC 模板组件化）：依赖 component_schemas 和 npc_presets 集合
- 需求 3（区域管理）：依赖通用 CRUD 框架
- 需求 4（关键字搜索）：依赖新的列表页骨架
- 需求 5（侧边栏重构）：**本需求直接完成侧边栏骨架**
- 需求 6（FSM/BT 编辑器 Schema 化）：依赖通用校验器

## 改动范围

### 后端（Go） — 约 22 个生产文件

| 包 | 文件数 | 改动 |
|---|--------|------|
| `cmd/admin/` | 1 | 重写 main.go 的依赖注入，适配新模块 |
| `internal/handler/` | 7 | 清除 4 个实体 handler，重构为通用 handler + schema/preset 只读 handler |
| `internal/service/` | 4 | 清除 4 个实体 service，重构为通用 service |
| `internal/validator/` | 6 | 清除 4 个硬编码 validator，引入 JSON Schema 校验器 |
| `internal/store/` | 3 | 保留通用 MongoStore，扩展支持新集合 |
| `internal/cache/` | 3 | 保留 RedisCache，不改 |
| `internal/model/` | 2 | 扩展 Document 模型，支持组件化结构 |

**新增依赖**：Go JSON Schema 校验库（如 `github.com/santhosh-tekuri/jsonschema/v6`）

### 前端（Vue 3） — 约 21 个文件

| 目录 | 文件数 | 改动 |
|------|--------|------|
| `src/router/` | 1 | 重写路由结构 |
| `src/components/` | 3 | 清除旧编辑器组件（暂不替代，需求 1/6 再实现） |
| `src/api/` | 5 | 清除 4 个实体 API，新增通用 API + schema/preset API |
| `src/views/` | 9 | 清除 9 个旧页面，新建占位页 |
| `src/utils/` | 1 | 保留 nameRules，可能需调整 |

**新增依赖**：JSON Schema 表单渲染库（如 `@lljj/vue3-form-element`）

## 扩展轴检查

- **新增配置类型**：✅ 有利。重构后新增类型只需注册路由 + 插入 schema，不改已有代码
- **新增表单字段**：✅ 有利。字段由 schema 定义，ADMIN 代码无需改动

这正是本需求的核心价值。

## 验收标准

### 后端
- **R1**：`docker compose up --build` 成功启动，无报错
- **R2**：`go test ./...` 全部通过
- **R3**：`GET /api/v1/component-schemas` 返回 200 + `{"items": []}`
- **R4**：`GET /api/v1/npc-presets` 返回 200 + `{"items": []}`
- **R5**：旧的 4 个实体 CRUD API（`/api/v1/event-types` 等）已移除，请求返回 404
- **R6**：JSON Schema 校验库已引入，`go.mod` 中可见
- **R7**：通用 CRUD handler 支持对任意已注册集合执行 List/Get/Create/Update/Delete
- **R8**：通用 schema 校验器可读取 `component_schemas` 集合中的 schema 并对 config 执行校验
- **R9**：配置导出接口（`/api/configs/*`）保留并适配新结构

### 前端
- **R10**：`npm run dev` 正常启动，无编译错误
- **R11**：侧边栏展示新分组结构：配置管理（NPC 模板 / 事件类型 / 状态机 / 行为树）、世界管理（区域管理）、系统设置（Schema 管理 / 导出管理）
- **R12**：点击任何侧边栏菜单进入对应占位页，显示"暂无数据"
- **R13**：旧的硬编码表单和列表页已删除，无残留代码
- **R14**：JSON Schema 表单渲染库已引入，`package.json` 中可见
- **R15**：API 层重构为通用模式，支持对任意实体类型执行 CRUD

### 工程质量
- **R16**：无硬编码的实体字段定义残留（validator 中无 `eventConfig`/`npcConfig` 等结构体）
- **R17**：代码中无 TODO 占位（占位页面可以有 UI 占位，但代码不留 TODO）

## 不做什么

- ❌ 不实现动态表单渲染逻辑（需求 1 做）
- ❌ 不实现 NPC 模板的组件勾选流程（需求 2 做）
- ❌ 不实现区域管理的业务逻辑（需求 3 做）
- ❌ 不实现关键字搜索（需求 4 做）
- ❌ 不填充任何 schema 数据（等服务端 CC 提供）
- ❌ 不实现 BT/FSM 编辑器的新版本（需求 6 做）
- ❌ 不做用户权限、审批流、版本控制等非 AI 角色系统功能
- ❌ 不修改 Docker Compose 的服务组成（MongoDB + Redis 不变）
