# ADMIN 与游戏服务端 — 导出 API 契约

> 本文档是 ADMIN 运营平台与游戏服务端之间的接口规范。
> 所有配置由 ADMIN 写入 MongoDB，游戏服务端启动时通过 HTTP 一次性拉取。

---

## 1. 双方定位

| 角色 | 职责 |
|------|------|
| **ADMIN 运营平台** | 配置的唯一数据源。提供可视化 CRUD 界面 + HTTP 导出 API。不执行游戏逻辑，不与客户端通信 |
| **游戏服务端** | 启动时调用导出 API 拉取全部配置加载到内存。运行时驱动 NPC AI 行为，通过 WebSocket 推送客户端 |

**BB Key 同步**：各存各的。ADMIN 存字段标识 + 运行时 Key 表，服务端 `keys.go` 注册。新增 Key 通过文档/对话同步，不走 API 互拉。

---

## 2. 通用规范

**基础路径**：`GET /api/configs/{collection}`

**统一返回格式**：

```json
{"items": [{...}, {...}, ...]}
```

空数据返回 `{"items": []}`。

**MongoDB 文档格式**：所有导出配置统一 `{name, config}` 格式，`config` 内部结构由游戏服务端定义。

---

## 3. NPC 配置

`GET /api/configs/npc_templates`

```json
{
  "items": [
    {
      "name": "wolf_common",
      "config": {
        "template_ref": "combat_creature",
        "fields": {
          "display_name": "普通灰狼",
          "hp": 100,
          "attack": 15,
          "move_speed": 3.0,
          "visual_range": 200
        },
        "behavior": {
          "fsm_ref": "wolf_fsm",
          "bt_refs": {
            "idle": "wolf/idle",
            "chase": "wolf/chase"
          }
        }
      }
    }
  ]
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | NPC 唯一标识，区域刷怪表引用 |
| `config.template_ref` | string | ADMIN 内部模板名，服务端可忽略 |
| `config.fields` | object | 扁平 key-value，所有 NPC 属性。key = 字段英文标识，value 类型由 JSON 区分 |
| `config.behavior.fsm_ref` | string | 状态机名 |
| `config.behavior.bt_refs` | object | 状态名 → 行为树名 |

新增字段 = `fields` 里多一个 key-value，服务端不需要改代码。

---

## 4. 事件类型

`GET /api/configs/event_types`

```json
{
  "items": [
    {
      "name": "player_nearby",
      "config": {
        "display_name": "玩家靠近",
        "default_severity": 50,
        "default_ttl": 5.0,
        "perception_mode": "visual",
        "range": 80
      }
    }
  ]
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `name` | string | 事件唯一标识 |
| `config.display_name` | string | 中文名，服务端可忽略 |
| `config.default_severity` | float64 | 威胁等级 0-100 |
| `config.default_ttl` | float64 | 存活时间（秒） |
| `config.perception_mode` | string | `visual` / `auditory` / `global` |
| `config.range` | float64 | 传播范围（米），global 模式下为 0 |

### 4.1 系统字段 vs 扩展字段

config 字段集合是**动态的**：

- **系统字段**（永远存在）：`display_name` / `default_severity` / `default_ttl` / `perception_mode` / `range`
- **扩展字段**（运营自定义）：通过 Schema 管理页定义，如 `priority` / `category` / `cooldown` / `stackable`。未填的字段不进 `config`

### 4.2 责任划分

| 关注点 | ADMIN 侧 | 服务端侧 |
|--------|----------|---------|
| 系统字段格式 | Handler 强校验 + Service 兜底 | struct 硬解析，缺失 reject |
| 扩展字段类型 | Service 层按 Schema 校验后才落库 | 运行时访问器退化 + warn |
| 扩展字段默认值 | Schema `default_value` 仅做表单初始值提示 | `defaults.go` 单一事实来源 |
| 扩展字段增删 | Schema 管理页自助，不清存量 | 透明，`Extensions` map 自动承载 |

### 4.3 服务端实现契约

1. 系统字段硬解析到 struct，缺失则 reject
2. 未知字段用 `Extensions map[string]json.RawMessage` 接收
3. 扩展字段默认值集中在 `internal/runtime/event/defaults.go`
4. 类型错误退化到默认值 + `slog.Warn`，不 panic
5. 扩展字段增删对服务端透明

---

## 5. 状态机

`GET /api/configs/fsm_configs`

```json
{
  "items": [
    {
      "name": "wolf_fsm",
      "config": {
        "initial_state": "idle",
        "states": [
          {"name": "idle"},
          {"name": "chase"},
          {"name": "attack"}
        ],
        "transitions": [
          {
            "from": "idle",
            "to": "chase",
            "priority": 2,
            "condition": {
              "key": "player_distance",
              "op": "<",
              "value": 80
            }
          }
        ]
      }
    }
  ]
}
```

| 字段 | 说明 |
|------|------|
| `initial_state` | 初始状态名 |
| `states[].name` | 状态名 |
| `transitions[].from/to` | 转换起止状态 |
| `transitions[].priority` | 优先级（数字越大越优先） |
| `transitions[].condition` | 条件表达式 |

**条件格式**：
- 叶节点：`{"key": "...", "op": "...", "value": ...}` 或 `{"key": "...", "op": "...", "ref_key": "..."}`
- 组合：`{"and": [...]}` 或 `{"or": [...]}`（可嵌套）
- 操作符：`==` / `!=` / `>` / `>=` / `<` / `<=` / `in`
- `key` 来源：NPC fields 中暴露的字段标识 + 服务端运行时 Key

---

## 6. 行为树

`GET /api/configs/bt_trees`

```json
{
  "items": [
    {
      "name": "wolf/attack",
      "config": {
        "type": "sequence",
        "children": [
          {
            "type": "check_bb_float",
            "key": "player_distance",
            "op": "<",
            "value": 5
          },
          {
            "type": "stub_action",
            "name": "melee_attack",
            "result": "success"
          }
        ]
      }
    }
  ]
}
```

**节点格式（扁平，无 params 包装）**：

| 分类 | 类型 | 子节点字段 |
|------|------|-----------|
| composite | `sequence` / `selector` / `parallel` | `children: [...]` |
| decorator | `inverter` | `child: {...}` |
| leaf | `check_bb_float` / `check_bb_string` / `set_bb_value` / `stub_action` / ... | 无 |

节点类型可扩展：服务端注册新节点 → ADMIN 导入 schema → 编辑器自动出现新选项。

---

## 7. 区域

`GET /api/configs/regions`

```json
{
  "items": [
    {
      "name": "grassland",
      "config": {
        "display_name": "风语草原",
        "region_type": "wilderness",
        "boundary": {
          "type": "polygon",
          "points": [{"x": 0, "z": 0}, {"x": 1000, "z": 0}]
        },
        "weather": {
          "default": "sunny",
          "cycle": [{"type": "sunny", "weight": 60}]
        },
        "spawn_table": [
          {
            "template_ref": "wolf_common",
            "count": 5,
            "spawn_points": [{"x": 100, "z": 200}],
            "wander_radius": 50,
            "respawn_seconds": 120
          }
        ],
        "properties": {
          "level_range": {"min": 1, "max": 10},
          "bgm": "grassland_theme"
        }
      }
    }
  ]
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| `boundary` | object | 区域边界多边形，服务端做 NPC 越界检查 |
| `spawn_table` | array | 刷怪表，启动时据此创建 NPC |
| `spawn_table[].template_ref` | string | 对应 npc_templates 的 name |
| `weather` / `properties` | object | 天气和扩展属性，透传给客户端 |
