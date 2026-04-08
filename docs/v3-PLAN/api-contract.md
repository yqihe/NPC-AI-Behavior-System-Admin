# ADMIN 运营平台 — 导出 API 契约

## 双方定位

**ADMIN 运营平台**：配置的唯一数据源。提供可视化界面供运营/策划 CRUD 各种配置，提供 HTTP 导出 API 供游戏服务端拉取。不执行任何游戏逻辑，不与客户端通信。

**游戏服务端**：启动时调用 ADMIN 导出 API 拉取全部配置加载到内存（拉不到则启动失败）。运行时驱动 NPC AI 行为（事件总线、感知过滤、决策中心、FSM、BT、边界校验），通过 WebSocket 推送状态给客户端。

**BB Key 同步方式**：各存各的。ADMIN 存字段标识 + 运行时 Key 表，服务端 `keys.go` 注册。新增 Key 通过文档/对话同步，不走 API 互拉。

---

## 导出 API

基础路径：`GET /api/configs/{collection}`

统一返回格式：

```json
{"items": [{...}, {...}, ...]}
```

空数据返回 `{"items": []}`。

---

### 1. NPC 配置

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
          "model_id": "wolf_01",
          "hp": 100,
          "attack": 15,
          "move_speed": 3.0,
          "wander_radius": 50,
          "chase_range": 120,
          "visual_range": 200,
          "auditory_range": 500,
          "spawn_x": 100,
          "spawn_z": 200,
          ...
        },
        "behavior": {
          "fsm_ref": "wolf_fsm",
          "bt_refs": {
            "idle": "wolf/idle",
            "walk": "wolf/walk",
            ...
          }
        }
      }
    },
    ...
  ]
}
```

| 字段 | 说明 | 服务端处理 |
|------|------|-----------|
| `name` | NPC 唯一标识 | 区域刷怪表引用 |
| `template_ref` | ADMIN 内部模板名 | 可忽略 |
| `fields` | 扁平 key-value，所有 NPC 属性 | 遍历写入黑板 `RegisterDynamic(k, type) + Set(k, v)` |
| `behavior.fsm_ref` | 状态机名 | 加载对应 FSM 配置 |
| `behavior.bt_refs` | 状态名 → 行为树名 | 当前状态对应哪棵 BT |

`fields` 的 key 是字段英文标识，value 类型由 JSON 区分（string/number/boolean/array）。新增字段只是多一个 key-value，服务端不需要改代码。

---

### 2. 事件类型

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
        "range": 80,
        ...
      }
    },
    ...
  ]
}
```

| 字段 | 说明 |
|------|------|
| `default_severity` | 威胁等级 0-100 |
| `default_ttl` | 存活时间（秒） |
| `perception_mode` | `visual` / `auditory` / `global` |
| `range` | 传播范围（米），global 模式下无效 |
| `display_name` | 中文名，服务端可忽略 |

---

### 3. 状态机

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
          {"name": "walk"},
          {"name": "chase"},
          {"name": "attack"},
          ...
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
          },
          {
            "from": "attack",
            "to": "idle",
            "priority": 1,
            "condition": {
              "and": [
                {"key": "player_distance", "op": ">", "value": 120},
                ...
              ]
            }
          },
          ...
        ]
      }
    },
    ...
  ]
}
```

条件（condition）格式：
- 叶节点：`{"key": "...", "op": "...", "value": ...}` 或 `{"key": "...", "op": "...", "ref_key": "..."}`
- 组合：`{"and": [...]}` 或 `{"or": [...]}`
- 可嵌套

操作符：`==`、`!=`、`>`、`>=`、`<`、`<=`、`in`

`key` 来源：NPC fields 中标记为暴露的字段标识 + 服务端运行时 Key。

---

### 4. 行为树

`GET /api/configs/bt_trees`

```json
{
  "items": [
    {
      "name": "wolf/idle",
      "config": {
        "type": "stub_action",
        "name": "stand_idle",
        "result": "success"
      }
    },
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
          },
          ...
        ]
      }
    },
    ...
  ]
}
```

节点格式（**扁平，无 params 包装**）：

| 分类 | 节点类型 | 子节点 |
|------|---------|--------|
| composite | `sequence` / `selector` / `parallel` | `children: [...]` |
| decorator | `inverter` | `child: {...}` |
| leaf | `check_bb_float` / `check_bb_string` / `set_bb_value` / `stub_action` / ... | 无子节点 |

节点类型可扩展：服务端注册新节点 → ADMIN 导入对应 schema → 编辑器自动出现新选项。

---

### 5. 区域

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
          "points": [
            {"x": 0, "z": 0},
            {"x": 1000, "z": 0},
            ...
          ]
        },
        "weather": {
          "default": "sunny",
          "cycle": [
            {"type": "sunny", "weight": 60},
            {"type": "rainy", "weight": 25},
            ...
          ]
        },
        "spawn_table": [
          {
            "template_ref": "wolf_common",
            "count": 5,
            "spawn_points": [{"x": 100, "z": 200}, ...],
            "wander_radius": 50,
            "respawn_seconds": 120
          },
          ...
        ],
        "properties": {
          "level_range": {"min": 1, "max": 10},
          "bgm": "grassland_theme",
          ...
        }
      }
    },
    ...
  ]
}
```

| 字段 | 说明 | 服务端处理 |
|------|------|-----------|
| `boundary` | 区域边界多边形 | NPC 越界检查 |
| `spawn_table` | 刷怪表 | 启动时根据此表创建 NPC |
| `spawn_table[].template_ref` | 对应 npc_templates 的 name | 加载 NPC 配置 |
| `weather` / `properties` | 天气和扩展属性 | 透传给客户端，服务端可不处理 |
