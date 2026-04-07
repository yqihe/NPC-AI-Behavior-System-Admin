# V3 架构升级 — 需求路线图

> 本文件是 V3 所有需求的总览，清空上下文后阅读此文件即可恢复全局视角。

## 背景

参考《洛克王国：世界》，对 ADMIN 运营平台做架构升级。核心目标：**从硬编码字段改为 Schema 驱动动态表单 + 组件化 NPC 模板**，聚焦游戏 AI 角色系统，不做商店/任务/战斗等非 AI 系统。

与游戏服务端 CC 已完成架构对齐（详见 memory: `project_v3_architecture_alignment.md`）。

## 技术决策

- **中间件不变**：MongoDB + Redis，不引入 MySQL / ES
- **Schema 格式**：JSON Schema Draft 7
- **分工**：Schema 由游戏服务端定义 → ADMIN 存储和渲染
- **非 AI 系统**（权限、审批、商店、战斗等）：预留接口，测试时 mock

## 需求清单

| # | 需求 | Spec 目录 | 状态 | 依赖 |
|---|------|-----------|------|------|
| 0 | 旧代码清理 + V3 基础准备 | `docs/specs/v3-foundation/` | **已完成** | 无 |
| 1 | Schema 驱动动态表单 | `docs/specs/schema-driven-form/` | **已完成** | 需求 0 |
| 2 | NPC 模板组件化 | `docs/specs/npc-component/` | **已完成** | 需求 0, 1 |
| 3 | 区域管理 | `docs/specs/region-management/` | **已完成** | 需求 0 |
| 4 | 关键字搜索 | `docs/specs/keyword-search/` | **已完成** | 需求 0 |
| 5 | 侧边栏重构 | 待创建 | 未开始 | 需求 0（骨架在 0 中完成） |
| 6 | FSM/BT 编辑器 Schema 化 | 待创建 | 未开始 | 需求 0, 1 |

## 依赖链

```
需求 0（基础准备）
  ├─→ 需求 1（动态表单）
  │     ├─→ 需求 2（NPC 组件化）
  │     └─→ 需求 6（FSM/BT Schema 化）
  ├─→ 需求 3（区域管理）
  ├─→ 需求 4（关键字搜索）
  └─→ 需求 5（侧边栏，骨架在 0 中完成）
```

## 各需求简述

### 需求 0：旧代码清理 + V3 基础准备
- 清除 4 个硬编码 validator/handler/service
- 搭建注册制通用 CRUD 框架
- 引入 JSON Schema 校验库（后端）和表单渲染库（前端）
- 新增 component_schemas / npc_presets 只读 API
- 侧边栏新分组骨架 + 占位页

### 需求 1：Schema 驱动动态表单
- 通用动态表单组件（读 JSON Schema → 渲染 Element Plus 表单）
- 支持 if/then 条件字段
- 支持字段分组/折叠
- 消灭所有硬编码字段

### 需求 2：NPC 模板组件化
- 选预设（simple/reactive/autonomous/social）→ 自动勾选组件
- 可手动增删组件 → 按已勾选组件渲染动态表单
- 10 个 AI 组件：identity, position, behavior, perception, movement, personality, needs, emotion, memory, social
- 动态黑板 Key（BT 编辑器 key 下拉 = 已启用组件的 keys 并集）

### 需求 3：区域管理
- regions CRUD（区域名、类型、边界、天气、NPC 刷怪表）
- 边界坐标预留给 Unity 客户端，现阶段手填
- 与 NPC 模板的 position.zone_id 关联

### 需求 4：关键字搜索
- 每个列表页支持按名称 + 关键字段搜索
- 后端 MongoDB regex / text index
- 前端搜索框 + 防抖

### 需求 5：侧边栏重构
- 配置管理（NPC 模板 / 事件类型 / 状态机 / 行为树）
- 世界管理（区域管理）
- 系统设置（Schema 管理 / 导出管理）
- 骨架在需求 0 中完成，需求 5 做细化和交互优化

### 需求 6：FSM/BT 编辑器 Schema 化
- BT 节点类型从硬编码改为读取 node-type-schemas
- FSM 条件类型从硬编码改为读取 condition-type-schemas
- 新增节点类型双方都不改代码

## 与游戏服务端的接口约定

### API 路径
| 路径 | 方法 | 用途 |
|------|------|------|
| `/api/v1/npc-templates` | CRUD | 替代旧 /api/v1/npc-types |
| `/api/v1/event-types` | CRUD | 不变 |
| `/api/v1/fsm-configs` | CRUD | 不变 |
| `/api/v1/bt-trees` | CRUD | 不变 |
| `/api/v1/regions` | CRUD | 新增 |
| `/api/v1/component-schemas` | GET（只读） | 组件 Schema |
| `/api/v1/npc-presets` | GET（只读） | NPC 预设定义 |
| `/api/v1/node-type-schemas` | GET（只读） | BT 节点类型 Schema |
| `/api/v1/condition-type-schemas` | GET（只读） | FSM 条件类型 Schema |
| `/api/configs/{collection}` | GET | 游戏服务端配置导出 |

### NPC 模板导出格式
```json
{
  "items": [{
    "name": "wolf_common",
    "config": {
      "preset": "creature",
      "components": {
        "identity": {"name": "普通灰狼", "model_id": "wolf_01"},
        "movement": {"move_type": "wander", "move_speed": 3.0, "wander_radius": 50}
      }
    }
  }]
}
```

### Schema 交付流程
1. 游戏服务端 CC 输出 schema JSON 文件
2. ADMIN 通过种子脚本导入 MongoDB component_schemas 集合
3. 前端读取 schema → 动态渲染表单
