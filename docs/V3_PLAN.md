# V3 完全重写规划

> 本文档是 V2 → V3 的完整经验总结和架构规划。V3 将从零开始，不保留 V2 代码。

---

## 一、V2 做了什么

### 已实现功能
1. 通用注册制 CRUD 框架（5 个实体，一行注册）
2. JSON Schema 种子脚本（25 个 schema 导入 MongoDB）
3. Schema 驱动动态表单（SchemaForm 自研渲染器）
4. NPC 模板组件化（预设选择 + 组件勾选 + 折叠面板）
5. FSM 编辑器（状态增删 + 转换规则 + 递归条件编辑器）
6. BT 编辑器（递归节点编辑器，节点类型从 API 动态获取）
7. 区域管理（Schema 驱动表单）
8. 列表关键字搜索（前端过滤）
9. 侧边栏三组分类
10. Schema 管理页（只读查看）
11. 导出管理页
12. 配置导出 API（供游戏服务端拉取）

### 代码量
- 后端 Go：20 个文件，1,587 行
- 前端 Vue：21 个文件，2,787 行
- 总计：4,374 行

### 中间件
- MongoDB 7（数据存储）
- Redis 7（列表缓存，5 分钟 TTL）
- Docker Compose（4 个服务）

---

## 二、V2 失败的原因

### 架构层面
1. **没有 MySQL**：组合搜索、分页、排序全在前端做，不可扩展
2. **没有消息队列**：数据同步只能同步写，没有异步保障
3. **缓存设计粗糙**：只缓存列表，没有单条缓存、没有分页缓存、没有穿透/雪崩防护
4. **假设数据量 < 100**：所有设计都基于"毕设够用"，不支持 1000+ 数据
5. **单体部署假设**：没有考虑多实例水平扩展

### 前端层面
6. **第三方表单库不可靠**：`@lljj/vue3-form-element` 与 Element Plus 不兼容，渲染空白，浪费时间
7. **双向 deep watcher 死循环**：SchemaForm/ComponentPanel 的 prop↔local 双向 watch 导致页面卡死
8. **Vue Router 组件复用不刷新**：同组件多路由切换时 onMounted 不重新执行
9. **没有用户引导**：非技术用户完全不知道该做什么
10. **枚举没有中文标签**：所有下拉选项都是英文
11. **JSON 编辑器暴露给运营**：违反"禁止让策划手写 JSON"红线

### 后端层面
12. **Schema 校验器只有骨架**：ValidateAll 是空实现，没有真正校验
13. **没有引用完整性检查**：删除 FSM 不检查是否被 NPC 引用
14. **没有审计日志**：谁改了什么完全无记录
15. **没有数据版本号**：无法判断数据新旧

### 流程层面
16. **先做了 7 个需求再发现架构有问题**：应该在第一个需求前就做好技术选型
17. **Spec 流程走得太快**：Phase 1/2/3 每次都直接"继续"，没有足够的讨论
18. **Bug 修复靠用户发现**：没有自己跑通完整的用户流程

---

## 三、V3 技术栈

```
┌─────────────────────────────────────────────────────┐
│                    前端 (Vue 3)                       │
│  Element Plus + 自研 SchemaForm + 组合筛选 + 分页     │
└────────────────────────┬────────────────────────────┘
                         │ HTTP REST API
┌────────────────────────▼────────────────────────────┐
│                  API 网关层                           │
│  CORS + 限流 + 请求日志 + Body 限制                   │
├─────────┬──────────┬──────────┬────────┬────────────┤
│ MongoDB │  MySQL   │  Redis   │RabbitMQ│            │
│         │          │          │        │            │
│ 配置原文 │ 搜索索引  │ 缓存     │ 异步同步│            │
│ Schema  │ 元数据    │ 分布式锁  │ 审计事件│            │
│ (数据源) │ 审计日志  │ 降级保障  │ 变更通知│            │
└─────────┴──────────┴──────────┴────────┴────────────┘
```

### 各中间件职责

| 中间件 | 角色 | 数据 |
|--------|------|------|
| **MongoDB** | 唯一数据源 | 配置 JSON 原文（{name, config}）、Schema 定义 |
| **MySQL** | 搜索索引 + 关系型数据 | 实体元数据、可搜索字段、审计日志、枚举标签 |
| **Redis** | 缓存 + 分布式锁 | 分页列表缓存、单条缓存、distinct 缓存、锁 |
| **RabbitMQ** | 异步消息 | MongoDB→MySQL 同步事件、审计事件、变更通知 |

---

## 四、V3 功能清单

### 核心功能（必须实现）
1. NPC 模板组件化 CRUD（预设 + 组件勾选 + Schema 驱动表单）
2. 事件类型 CRUD
3. 状态机 CRUD（状态 + 转换 + 递归条件编辑器）
4. 行为树 CRUD（递归节点编辑器，节点类型动态获取）
5. 区域管理 CRUD
6. 配置导出 API（供游戏服务端拉取）
7. Schema 管理页（只读查看所有 Schema）
8. 导出管理页

### 企业级基础设施（必须实现）
9. **组合搜索**：多字段筛选（名称输入 + 类型下拉 + 标签多选），后端 MySQL 查询
10. **分页**：后端分页 + 前端 el-pagination
11. **枚举中文标签**：Schema 层面 enumNames，动态渲染
12. **审计日志**：谁在什么时间改了什么，MySQL 存储，按月分区
13. **数据同步**：MongoDB→MySQL 通过 RabbitMQ 异步同步，幂等 + 去重 + DLQ
14. **引用完整性**：删除 FSM 前检查 NPC 是否引用
15. **缓存体系**：分页缓存 + 单条缓存 + distinct 缓存 + 穿透防护 + 雪崩防护 + 降级
16. **数据版本号**：MongoDB 文档 _version 自增，防乱序覆盖

### 用户体验（必须实现）
17. **新手引导**：Dashboard 步骤引导，每个页面顶部操作说明
18. **可折叠侧边栏**：el-sub-menu 分级折叠
19. **所有选项动态化**：下拉框选项全部从数据库读取，不硬编码
20. **中文友好**：所有字段中文标题 + 描述，枚举中文标签
21. **操作反馈**：成功/失败提示，loading 防重复，删除确认

---

## 五、V3 架构设计要点

### 后端分层

```
cmd/
  admin/main.go        程序入口
  seed/main.go         Schema 种子脚本
  worker/main.go       Sync Worker（MQ 消费者，可独立部署）

internal/
  handler/             HTTP Handler（路由 + 请求解析 + 响应）
  service/             业务逻辑（校验 + 编排 + 事务）
  store/
    mongo.go           MongoDB 操作
    mysql.go           MySQL 操作
  cache/               Redis 缓存
  mq/                  RabbitMQ 生产者 + 消费者
  validator/           JSON Schema 校验
  model/               数据模型
  sync/                MongoDB→MySQL 同步逻辑
```

### 写入流程

```
API Handler
  → Service.Create()
    → Validator.Validate()（JSON Schema 校验）
    → MongoStore.Create()（写数据源，_version++）
    → MQ.Publish("config.created")（异步同步 MySQL）
    → Cache.Invalidate()（精准清缓存）
    → return success
```

### 查询流程

```
API Handler（搜索请求）
  → Cache.Get(query_hash)
    → hit: return
    → miss:
      → MySQLStore.Search(filters, page, limit)
      → Cache.Set(query_hash, result)
      → return
```

### Sync Worker

```
MQ.Consume("sync_queue")
  → Redis 去重（MessageId）
  → MongoStore.Get(name)（读最新数据）
  → 提取元数据
  → MySQLStore.Upsert(metadata, version)（版本号防乱序）
  → ACK
  → 失败: NACK → 延迟重试 → DLQ → 报警
```

### MySQL 表设计原则

- 每个实体类型一张元数据表（不混在一起）
- 关键搜索字段提取为独立列（不查 JSON 列）
- 布尔字段代替 LIKE 查询（has_behavior, has_social）
- 组合索引覆盖高频查询
- 审计日志按月分区
- 版本号列（mongo_version）用于同步校验

### Redis Key 设计

```
admin:cache:{collection}:list:{query_hash}    分页列表     TTL 5min + random(60s)
admin:cache:{collection}:item:{name}          单条缓存     TTL 10min + random(60s)
admin:cache:{collection}:distinct:{field}     下拉选项     TTL 10min
admin:cache:{collection}:total:{query_hash}   分页总数     TTL 5min
admin:lock:{task_name}                        分布式锁     TTL 30s 自动续期
admin:mq:dedup:{message_id}                   MQ 消息去重  TTL 1h
admin:null:{collection}:{name}                空值缓存     TTL 1min
```

### RabbitMQ 设计

- Exchange: `admin.config`（direct 类型）
- Queue: `sync_queue`（持久化，手动 ACK）
- DLQ: `sync_queue.dlq`（死信队列）
- Routing Key: `config.created` / `config.updated` / `config.deleted`
- 消息格式: `{event, collection, name, version, timestamp, instance_id}`
- 消息 ID 去重 + 版本号防乱序 + UPSERT 幂等

---

## 六、V3 前端设计要点

### 不使用第三方表单库

V2 教训：`@lljj/vue3-form-element` 与 Element Plus 不兼容。V3 自研 SchemaForm，直接用 Element Plus 组件。

### SchemaForm 渲染规则

| JSON Schema type | Element Plus 组件 |
|-----------------|------------------|
| string + enum + enumNames | el-select（中文标签） |
| string | el-input |
| number + min/max | el-slider + show-input |
| number/integer | el-input-number |
| boolean | el-switch |
| array + enum items | el-checkbox-group |
| array + string items | el-tag 动态输入 |
| array + object items | 动态表格（el-table 可增删行） |
| object | el-card 嵌套渲染 |
| allOf + if/then | 监听触发字段，动态显隐 |

### 避免 V2 的 watcher 死循环

```js
// 错误（V2）：双向 deep watch 互相触发
watch(prop, set local, { deep: true })
watch(local, emit, { deep: true })

// 正确（V3）：单向数据流，子组件不维护 local copy
// 方案 1：直接 emit 每个字段的变更
function updateField(name, value) {
  emit('update:modelValue', { ...props.modelValue, [name]: value })
}

// 方案 2：如果必须 local copy，用 JSON 比较 + 标志位
let isInternalUpdate = false
watch(local, () => { if (!isInternalUpdate) emit(...) })
watch(prop, () => { isInternalUpdate = true; local = ...; nextTick(() => isInternalUpdate = false) })
```

### 侧边栏：el-sub-menu 可折叠

```vue
<el-sub-menu index="config">
  <template #title>配置管理</template>
  <el-menu-item index="/npc-templates">NPC 模板</el-menu-item>
  <el-menu-item index="/event-types">事件类型</el-menu-item>
  ...
</el-sub-menu>
```

### 组合搜索筛选面板

```vue
<el-form inline>
  <el-form-item label="名称">
    <el-input v-model="filters.name" placeholder="模糊搜索" />
  </el-form-item>
  <el-form-item label="感知方式">
    <el-select v-model="filters.perception_mode">
      <!-- 选项从 GET /api/v1/event-types/distinct/perception_mode 动态获取 -->
    </el-select>
  </el-form-item>
  <el-button @click="search">搜索</el-button>
  <el-button @click="reset">重置</el-button>
</el-form>

<el-table :data="items" />
<el-pagination :total="total" :page-size="20" @current-change="onPageChange" />
```

---

## 七、V2 经验 → V3 红线补充

### 新增到 `docs/standards/red-lines.md`

- **禁止假设数据量小于 100**。所有列表必须后端分页，前端不做全量过滤
- **禁止前端硬编码枚举选项**。所有下拉框选项必须从数据库动态获取
- **禁止使用未经验证的第三方 UI 库**。表单渲染器自研，不依赖第三方 JSON Schema 表单库

### 新增到 `docs/standards/frontend-red-lines.md`

- **禁止双向 deep watcher**。父子组件数据传递用单向数据流（props down, events up），不在子组件维护 local copy + emit 的双向 watch
- **禁止忘记 router-view key**。多路由共用组件时必须 `:key="route.fullPath"`

### 新增到 `docs/development/dev-rules.md`

- **技术选型在需求 0 之前完成**。中间件栈（MySQL/Redis/MQ）必须在第一行代码之前确定
- **每个页面必须自己走一遍完整用户流程**。不能只看代码不测页面
- **非技术用户视角审查**。每个页面问"运营人员看到这个页面知道该做什么吗？"

---

## 八、V3 开发顺序

```
Phase 0: 基础设施
  ├─ Docker Compose（MongoDB + MySQL + Redis + RabbitMQ）
  ├─ 后端骨架（handler/service/store/cache/mq 分层）
  ├─ MySQL 表结构（所有实体元数据表 + 审计日志表 + 枚举标签表）
  ├─ Redis Key 规范
  ├─ RabbitMQ Exchange/Queue 定义
  ├─ Sync Worker 骨架
  └─ 前端骨架（路由 + 侧边栏 + 空页面）

Phase 1: 数据层
  ├─ MongoDB CRUD（通用 store）
  ├─ MySQL 元数据同步（MQ 生产 + 消费 + 幂等）
  ├─ Redis 缓存体系（分页/单条/distinct/穿透/雪崩）
  ├─ JSON Schema 校验器
  └─ Schema 种子脚本

Phase 2: API 层
  ├─ 通用 CRUD API
  ├─ 组合搜索 API（MySQL 多字段筛选 + 分页）
  ├─ Distinct API（下拉选项动态获取）
  ├─ 引用完整性检查
  ├─ 配置导出 API
  └─ 审计日志 API

Phase 3: 前端核心
  ├─ SchemaForm（自研，支持所有字段类型 + 条件字段 + enumNames）
  ├─ 组合筛选面板 + 分页
  ├─ 通用列表页 + 通用表单页
  └─ 可折叠侧边栏 + Dashboard 引导

Phase 4: 专用页面
  ├─ NPC 模板（预设 + 组件勾选 + 折叠面板）
  ├─ FSM 编辑器（状态 + 转换 + 条件）
  ├─ BT 编辑器（递归节点 + 动态类型）
  ├─ 区域管理
  ├─ Schema 管理页
  └─ 导出管理页

Phase 5: 企业级加固
  ├─ 审计日志页面
  ├─ 抽样校验 Worker
  ├─ 监控指标（Prometheus）
  └─ 部署文档（多实例 + nginx 负载均衡）
```

---

## 九、与游戏服务端 CC 需要对齐的事项

1. **Schema 格式升级**：所有 enum 加 `enumNames` 中文标签
2. **配置导出 API 格式**：是否需要变更（V2 的 `{items: [...]}` 是否保留）
3. **数据版本号**：MongoDB 文档加 `_version` 字段，服务端是否需要读取
4. **新增实体类型的 Schema**：事件类型的 schema 由谁定义（V2 中 ADMIN 自己定义了 `_event_type`）
5. **服务端也打算重写**：底层数据格式、API 契约需要重新商定
