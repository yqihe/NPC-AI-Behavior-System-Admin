# 游戏服务端 ↔ ADMIN API 契约

**ADMIN 为权威源**。本文件定义服务端启动时通过 HTTP 拉取 ADMIN 配置的导出接口形态；服务端侧 `internal/runtime/npc/admin_template.go` 等反序列化逻辑反向依赖此 schema。

**同步方式：人工同步**（毕设体量不引入 git submodule / CI mirror）：
- ADMIN 仓改本文件 → commit + push
- 契约变更必须在 commit message 显式标注"影响服务端"
- 服务端仓发 PR 时 description 引用 ADMIN 对应 commit hash 作为契约版本锚
- 若 ADMIN 改契约未通知服务端，由 `docs/development/standards/red-lines/general.md` "禁止协作失序"红线兜底

**当前版本**：v1.2（2026-04-21，doc-only：形式化 event_types / fsm_configs / bt_trees / regions 4 段契约；无 shape 变更）
- v1.2：补齐 4 个导出端点的 Schema + 字段说明 + 双边契约锚定；规范化 Server HTTPSource 4 硬失败端点 + regions 空 items[] 容忍的端点级行为差异
- v1.1.3：seed-fsm-bt-coverage Phase 2 doc-only append —— fields 内字段语义表（战斗数值 + 社交），覆盖 Server post-freeze smoke 暴露的契约附录缺口
- v1.1.2：seed-fsm-bt-coverage batch2 补齐 5 event_types 冷启动覆盖（服务端 HTTPSource 对空 items 硬失败，`earthquake` / `explosion` / `fire` / `gunshot` / `shout`）
- v1.1.1：seed-fsm-bt-coverage 补齐 3 FSM + 6 BT 冷启动覆盖；契约 shape 未变
- v1.1：2026-04-19，新增组件 opt-in 依赖矩阵；对齐服务端仓 spec `external-contract-server-adaptation` R17-R21

**覆盖范围**：5 个导出端点形式化完成 —— `GET /api/configs/{npc_templates,event_types,fsm_configs,bt_trees,regions}`。

**端点级行为差异（Server 侧）**：
- `event_types` / `fsm_configs` / `bt_trees` / `npc_templates`：**硬失败端点**（Server `HTTPSource.fetchEndpoint` 对空 items[] 返错，optional=false）—— ADMIN 侧冷启动必须有 seed 覆盖（至少 5 event_types + 3 FSM + 6 BT + N NPC templates）
- `regions`：**空 items[] 合法**（Server `fetchRegionsEndpoint` 专用通道；与 JSONSource 目录不存在语义一致）

---

## GET /api/configs/npc_templates

**用途**：服务端启动或 `cmd/sync` 拉取 NPC 模板配置到 configs/。

**调用方**：服务端 `internal/runtime/npc/admin_template.go`——**唯一的反序列化落点**，本 schema 的反向依赖源。

**ADMIN 实现位置**：`backend/internal/service/npc_service.go` 的 `assembleExportItem`（从 `npcs.fields` JSON 组装 `map[name]value`），`backend/internal/model/npc.go` 的 `NPCExportItem` / `NPCExportConfig` / `NPCExportBehavior`。

**返回状态**：始终 `200 OK`（失败时进通用错误响应，超出本契约范围）。

### Schema

```json
{
  "items": [
    {
      "name": "wolf_common",
      "config": {
        "template_ref": "warrior_base",
        "fields": {
          "aggression": "aggressive",
          "attack_power": 18.5,
          "defense": 8.0,
          "is_boss": false,
          "loot_table": "loot_wolf_common",
          "max_hp": 120,
          "move_speed": 5.5,
          "perception_range": 20.0
        },
        "behavior": {
          "fsm_ref": "fsm_combat_basic",
          "bt_refs": {
            "attack": "bt/combat/attack",
            "chase": "bt/combat/chase",
            "idle": "bt/combat/idle",
            "patrol": "bt/combat/patrol"
          }
        }
      }
    }
  ]
}
```

### 字段说明

| 路径 | 类型 | 语义 | 空值 / 可选 |
|---|---|---|---|
| `items` | array | NPC 列表，顺序无保证（服务端不得依赖序） | 空列表合法（无 NPC）|
| `items[].name` | string | NPC 唯一标识，小写 + 下划线，`^[a-z][a-z0-9_]*$` | 必填非空 |
| `items[].config` | object | 配置体 | 必填 |
| `items[].config.template_ref` | string | 模板标识（仅做名字引用，**服务端视为不透明字符串**，不要求预先声明）| 必填非空 |
| `items[].config.fields` | object<string, any> | 字段名 → 值映射；value 保留 JSON 原类型（number/string/bool/null）| 必填；可为 `{}`（如纯占位模板）|
| `items[].config.behavior` | object | 行为配置容器 | 必填（对象本身存在），内部键可能被 `omitempty` 省略 |
| `items[].config.behavior.fsm_ref` | string | FSM 配置 name；服务端按此名到 fsm_configs 集合查 | **可选**：空串时 JSON 中**整键省略** |
| `items[].config.behavior.bt_refs` | object<string, string> | FSM 状态 name → 行为树 name；value 是已启用的行为树标识 | **可选**：空 map 时 JSON 中**整键省略** |

**key 顺序**：`items[].config.fields` 和 `behavior.bt_refs` 的 key 由 Go `encoding/json` 按字典序输出（Go 1.12+ 稳定行为）；服务端解析时不要依赖业务顺序，应按 key 读。

**value 类型约定**：
- number 保留浮点形态（`8.0` 不归一化为 `8`）。ADMIN 用 `json.RawMessage` 存 `npcs.fields[].value` 字节，MySQL JSON 列不改写数值形态。服务端若做精确 diff 对比 snapshot 需注意此点。
- 枚举类字段（如 `aggression`）value 为 string；ADMIN 侧 constraint_schema 约束合法枚举值，但**服务端解析时不做再次校验**（ADMIN 为权威）。

### fields 内字段语义（v1.1.3 新增）

`items[].config.fields` 是 `object<string, any>`（top-level shape 详见上表），value 保留 JSON 原类型。本节形式化当前**有 Server 侧消费语义**的 fields 内字段。未在此列出的字段（如 `aggression` / `move_speed` / `perception_range`）由 ADMIN 侧 constraint_schema 约束，**服务端不做解析侧校验**（ADMIN 为权威）。

#### 战斗数值字段

| 字段 | 类型 | 服务端消费位置 | 语义 | default |
|---|---|---|---|---|
| `max_hp` | number | `admin_template.go` → NPC 实例初始血量上限 | 战斗/HP 系统基底。历史数据噪声 `guard_basic.fields.hp` 待 41008 解封后统一为 `max_hp`（见 §已知数据噪声）| 必填，无默认 |
| `attack_power` | number | `admin_template.go` → 战斗系统伤害基值 | 攻击伤害基数（浮点）| 必填，无默认 |
| `defense` | number | `admin_template.go` → 战斗系统减伤基值 | 防御减伤基数（浮点）| 必填，无默认 |
| `is_boss` | bool | `admin_template.go` → 影响 decision 权重 + 掉落逻辑 | 是否 boss 类 NPC | 必填，默认 `false` |
| `loot_table` | string | `admin_template.go` → 死亡掉落表 id | 掉落表字符串 id（服务端视为不透明字符串，不做预声明校验）| 必填非空 |

**note**：当前 Server runtime **无血量系统 / 无 damage 事件 / 无 die 事件**，上述字段写入 NPC 但未被 tick loop 消费。R15 smoke 仅校验"字段存在 + 类型正确"，不校验战斗行为。Flee/Dead 态 + 真实伤害闭环留给后续独立 HP 系统 spec。

#### 社交字段（opt-in `enable_social=true` 时启用）

| 字段 | 类型 | 服务端消费链路 | 语义 | default |
|---|---|---|---|---|
| `group_id` | string | `admin_template.go:298-299` 反序列化到 `SocialComponent.GroupID` → `admin_template.go:122-126` `blackboard.Set(bb, KeyGroupID, ...)` 镜像至 BB → `GroupManager` 按 `KeyGroupID` 聚合 | NPC 所属 group 自由标识（如 `"merchant_guild"` / `"village_guard"`）。空串时 `GroupManager` 对该 NPC 不可见（逐 NPC 跳过，不影响其他 NPC）| 可选（空串默认）|
| `social_role` | string | `admin_template.go:298-299` 反序列化到 `SocialComponent.Role` → `admin_template.go:122-126` `blackboard.Set(bb, KeySocialRole, ...)` 镜像至 BB | group 内角色（`"leader"` / `"follower"` 触发队形逻辑；其他自由值如 `"trader"` / `"guard"` 在 group 中但无队形行为，未知 role 静默 skip）| 可选（空串默认）|

**Role 白名单放宽锚点**：Server PR [#32](https://github.com/yqihe/npc-ai-behavior-system-server/pull/32) —— `SocialFactory` 去掉 `{"leader","follower"}` 硬限制；`group_manager` 内 `role == "leader"` / `role != "follower"` 分支保留。PR URL 稳定引用（不引 merge hash —— squash/rebase 后 hash 变，PR 号永远不变）。

**字段归属**：`group_id` / `social_role` 在 **Admin `fields` 表中 `category=component` 而非 `blackboard_key`**（Admin 字段模型层面不当做 runtime key 管理）。服务端实例化 `SocialComponent` 后会 **mirror 到 BB key**（`KeyGroupID` / `KeySocialRole` 由 `blackboard/keys.go` 定义），供 `GroupManager` 读取。

### 双边契约锚定

**服务端 admin_template.go 反向依赖此 schema**。任何以下改动都属于 breaking change，必须先改本文件再改代码：
- `items[].name` / `items[].config` 层级结构变动
- `template_ref` 语义变化（比如从"字符串"变成"对象"）
- `fields` 从 `object<string, any>` 变为 `array<object>`
- `behavior.fsm_ref` / `behavior.bt_refs` 的 omitempty 语义切换（必填化或删除）

**非 breaking change**（免通知）：新增 `items[].config.*` 下的可选字段（带 omitempty）、字段值类型在兼容子集内调整（如 int↔float 表示的同数值）。

### 组件 opt-in 依赖矩阵（v1.1 新增）

#### 5 个 opt-in bool 字段

ADMIN `items[].config.fields` 中**约定的 5 个 bool 字段**控制服务端侧能力组件实例化：

| 字段名 | 语义 | default_value | absent 时 |
|---|---|---|---|
| `enable_memory` | 记忆组件：写入威胁记忆 → 驱动 emotion | `false`（必填）| 等价 false |
| `enable_emotion` | 情绪组件：读记忆累积 fear → 驱动 decision | `false`（必填）| 等价 false |
| `enable_needs` | 需求组件：计算 lowest need → 驱动 decision | `false`（必填）| 等价 false |
| `enable_personality` | 性格组件：提供 decision weights 覆盖默认值 | `false`（必填）| 等价 false |
| `enable_social` | 社交组件：group/follower/leader 机制 | `false`（必填）| 等价 false |

**absent ≡ false 语义锁定**：字段缺失等价显式 `false`，避免"未声明 vs 显式关闭"歧义。

**ADMIN seed 强制要求**：5 个 bool field 的 `properties.default_value` 必须显式设为 `false`，确保新建 NPC 携带 false 而非 null。若存 null，导出时字段会变为 `null` 而非 `false`，服务端解析歧义。

#### 级联依赖（硬约束）

| 如启用 | 必须同时启用 | 校验位置 | 违规后果 |
|---|---|---|---|
| `enable_emotion` | `enable_memory` | 服务端启动 Registry 填充阶段，**逐 NPC** 校验 | **Fatal**：打印违规 NPC name 列表 + ADMIN UI 修正路径，**不跳过违规 NPC、不部分启动** |

**根因**：`emotion.Tick()` 读 `KeyMemoryThreatValue`（由 `memory.Tick()` 写入）；无 memory 则 emotion.fear 永不累积 → emotion 独立开启无意义。

**ADMIN 侧已知缺陷**：ADMIN 字段系统当前不支持跨字段联动校验（见 `deferred-features.md`），运营侧可以保存 `enable_emotion=true, enable_memory=false` 的非法组合。服务端兜底 fatal 校验承担此约束。

#### 其他组件的独立性

| 组合 | 级别 | 说明 |
|---|---|---|
| `enable_needs` 单开 | ⚠️ 弱耦合警告 | `personality.weights.Needs` 失去乘数意义，但不违法 |
| `enable_personality` 单开 | ✅ 合法 | 无 BB 链路，decision 用自定义 weights 但 NeedUrgency=0 |
| `enable_social` 单开 | ✅ 合法 | 完全独立，只影响 group 可见性 |

#### 组件缺席时的系统行为

服务端必须保证**任意组合缺席下 Tick 不崩溃**。当前实现已合规，契约要求不退化：

| 缺席组件 | 直接效果 | 可观测二阶效果 |
|---|---|---|
| memory | `KeyMemoryThreatValue` unset | emotion.fear 衰减到 0（若 emotion 开） |
| emotion | `KeyEmotionDominant/Val` unset | `scheduler.buildDecisionInput.EmotionValue=0` |
| needs | `KeyNeedLowest/Val` unset | `scheduler.calcNeedUrgency=0` → `decision.NeedUrgency=0` |
| personality | 无 BB 影响 | `decision.Weights` 用 `decision.DefaultWeights`（Threat/Needs/Emotion 各为 1）|
| social | `KeyGroupID/SocialRole` unset | `GroupManager` 对该 NPC 不可见（逐 NPC 跳过，不影响其他 NPC） |

**服务端实现准入模式**：所有组件访问点通过 `npc.GetComponent[T](inst, name)` 的 `(T, ok)` 返回值决策。**禁止**裸 nil 访问、类型断言 panic、或整体禁用系统。当前生产代码面 12 处访问全部合规（`scheduler.go` 7 处 + `group_manager.go` 4 处 + `gateway/handler.go` 1 处）。

#### 软/硬依赖契约

- **软依赖（允许）**：组件 X 读取组件 Y 写入的 BB key（如 emotion 读 `KeyMemoryThreatValue`）。Y 缺席时 X 必须降级到默认值，不得阻塞 tick 或 panic

- **硬依赖（禁止）**：**组件代码内部**出现 `GetComponent[Y]` 访问其他组件类型 Y。例如 `emotion.Tick()` 里调用 `GetComponent[*MemoryComponent](inst, "memory")` 属违规

- **编排层例外**：scheduler / gateway / group_manager 等**编排器**对组件的直接访问是组件化架构的合法协调机制——编排层的核心职责就是把组件粘合起来。此类访问**不计入硬依赖**，也不构成技术债

- **当前全部合规**：scheduler.go（7）+ group_manager.go（4）+ gateway/handler.go（1）共 **12 处组件访问全部位于编排层**，无组件间硬依赖违规。服务端代码已遵循"编排层协调组件 / 组件间只通过 BB 交互"的双层架构原则

### 已知数据噪声

#### `guard_basic.fields.hp`

- **现象**：`items[].name="guard_basic"` 的 `config.fields` 返回 `{"hp": 100}` 而非更规范的 `{"max_hp": 100}`
- **原因**：T9 建字段时没看到已存在的 `max_hp` 造成重复（属 ADMIN 侧数据治理遗留，非服务端 bug）
- **当前策略**：ADMIN 把 `hp` seed 为孤儿字段（`enabled=0` + 不进任何模板 `fields` 数组），仅被 `guard_basic` NPC 的字段快照引用；UI 层默认不暴露此字段给策划选择
- **服务端影响**：`SetDynamic` 把 `hp` 写入 BB，但无任何 BT 节点消费，**实际对行为无影响**
- **清除时机**：ADMIN 41008 硬约束（模板被 NPC 引用时字段不可编辑）解封后，一次性把 `guard_basic` 的字段改为 `max_hp`，同时删除 fields 表 hp 行
- **参考**：memory `project_guard_basic_hp_deferred.md`；本 spec design.md §2 OQ3 方案 A

---

## GET /api/configs/event_types

**用途**：服务端启动时一次性拉取所有已启用事件类型，供感知系统（visual / auditory / global）分类分发。

**调用方**：服务端 `internal/config/http_source.go` → `fetchEndpoint(/api/configs/event_types)` → 内存 map[name][]byte；运行期 `LoadEventConfig(name)` / `LoadAllEventConfigs()` 返回原始 JSON 给事件系统反序列化。

**ADMIN 实现位置**：`backend/internal/handler/export.go` 的 `EventTypes`；`backend/internal/service/event_type.go` 的 `ExportAll`；`backend/internal/model/event_type.go` 的 `EventTypeExportItem{Name, Config json.RawMessage}` —— **`config` 字段直接从 `event_types.config_json` 列原样透传**，不经过 Go struct 中转。

**返回状态**：始终 `200 OK`。空 items[] 在 ADMIN 侧合法，但 **Server 启动期硬失败**（HTTPSource.fetchEndpoint `optional=false` + 空 target 报错）。

### Schema

```json
{
  "items": [
    {
      "name": "gunshot",
      "config": {
        "display_name": "枪声",
        "default_severity": 90,
        "default_ttl": 10,
        "perception_mode": "auditory",
        "range": 300
      }
    }
  ]
}
```

### 字段说明

| 路径 | 类型 | 语义 | 空值 / 可选 |
|---|---|---|---|
| `items[].name` | string | 事件类型标识，`^[a-z][a-z0-9_]*$`（`IdentPattern`）| 必填非空 |
| `items[].config` | object | 事件配置体，`config_json` 列原样透传 | 必填 |
| `items[].config.display_name` | string | 中文展示名 | 必填非空 |
| `items[].config.default_severity` | number | 事件强度基值（浮点保留）| 必填 |
| `items[].config.default_ttl` | number | 事件在感知记忆中的有效期（秒）| 必填 |
| `items[].config.perception_mode` | string | 感知通道枚举：`global` / `auditory` / `visual`（`util.ValidPerceptionModes`）| 必填 |
| `items[].config.range` | number | 触发半径（米）；`global` 模式下 `range=0` 语义为全图| 必填 |
| `items[].config.<extension_key>` | any | 扩展字段键值（由 `event_type_schemas` 表定义）；扁平合并入 config 顶层，**不嵌套**在 extensions 对象下 | 可选 |

**扩展字段机制**：ADMIN 侧 `event_type_schemas` 表定义按 event_type 维度的扩展字段 schema（`field_type` + `constraints` 校验），service 层 `validateExtensions` 在写时保存前校验值合法性，最终通过 `buildConfigJSON` 平铺入 `config_json`。**服务端视扩展字段为不透明 JSON**，解析与否由事件消费代码决定。

**最小冷启动集**（v1.1.2 锚定）：5 条 event_types（`earthquake` / `explosion` / `fire` / `gunshot` / `shout`）由 `seedEventTypes` 保证 —— 少于这些 Server 启动期 fetchEndpoint 虽然不会因 count 失败（有至少一条就算通过），但 Server 侧具体事件消费代码对这 5 条有名字耦合。

### 双边契约锚定

破坏性变更（必须先改本文件再改代码）：
- `items[].config` 层级结构变动（扁平 → 嵌套 / 反向）
- `perception_mode` 枚举值新增/删除
- `default_severity` / `default_ttl` / `range` 的类型从 number 变 string
- 扩展字段语义从"扁平合并"变为"子对象"（影响现有消费代码）

非破坏变更（免通知）：新增 `perception_mode` 枚举值（服务端视为不透明字符串时）、新增扩展字段 key。

---

## GET /api/configs/fsm_configs

**用途**：服务端启动时一次性拉取所有已启用状态机配置，按 NPC 模板的 `behavior.fsm_ref` 字段查表实例化。

**调用方**：服务端 `internal/config/http_source.go` → `fetchEndpoint(/api/configs/fsm_configs)` → 内存 map[name][]byte；运行期 `LoadFSMConfig(name)` 反序列化成 `fsm.FSMConfig` 喂给 FSM 执行器。

**ADMIN 实现位置**：`backend/internal/handler/export.go` 的 `FsmConfigs`；`backend/internal/service/fsm_config.go` 的 `ExportAll`；`backend/internal/model/fsm_config.go` 的 `FsmConfigExportItem{Name, Config json.RawMessage}` —— **`config` 字段原样透传 `fsm_configs.config_json`**。

**返回状态**：始终 `200 OK`。空 items[] **Server 启动期硬失败**（同 event_types）。

### Schema

```json
{
  "items": [
    {
      "name": "fsm_combat_basic",
      "config": {
        "initial_state": "Idle",
        "states": [
          {"name": "Idle"},
          {"name": "Patrol"},
          {"name": "Chase"},
          {"name": "Attack"}
        ],
        "transitions": [
          {
            "from": "Idle",
            "to": "Patrol",
            "priority": 1,
            "condition": {}
          },
          {
            "from": "Patrol",
            "to": "Chase",
            "priority": 10,
            "condition": {
              "and": [
                {"key": "threat_level", "op": ">=", "value": 30},
                {"key": "threat_expire_at", "op": ">", "ref_key": "current_time"}
              ]
            }
          }
        ]
      }
    }
  ]
}
```

### 字段说明

| 路径 | 类型 | 语义 | 空值 / 可选 |
|---|---|---|---|
| `items[].name` | string | FSM 标识，`^[a-z][a-z0-9_]*$` | 必填非空 |
| `items[].config.initial_state` | string | 初始状态名（必须在 `states[].name` 集合内；ADMIN 写时校验）| 必填非空 |
| `items[].config.states` | array<object> | 状态列表；顺序无语义 | 必填非空（`ErrFsmConfigStatesEmpty` 拦截）|
| `items[].config.states[].name` | string | 状态名（NPC 运行期的活动状态标识）| 必填非空 |
| `items[].config.transitions` | array<object> | 转换规则列表；ADMIN 允许空数组（对应"单状态 FSM 无转换"场景，如 `fsm_passive`）| 可为 `[]` |
| `items[].config.transitions[].from` | string | 起点状态名；必须在 `states` 内 | 必填非空 |
| `items[].config.transitions[].to` | string | 目标状态名；必须在 `states` 内 | 必填非空 |
| `items[].config.transitions[].priority` | number | 多条同 from 转换竞争时的优先级排序，高者优先 | 必填（可为 0）|
| `items[].config.transitions[].condition` | object | 条件树；**空对象 `{}` 语义为"始终 true"**（无条件转换）| 必填（对象本身存在）|

**condition 条件树结构**（对齐 Server `core/rule.Condition`）：

| 形态 | 字段组合 | 语义 |
|---|---|---|
| 叶节点（立即值）| `{key, op, value}` | BB key 值与 `value` 比较 |
| 叶节点（引用值）| `{key, op, ref_key}` | BB key 值与另一 BB key (`ref_key`) 比较；**`value` 与 `ref_key` 互斥** |
| 组合 AND | `{and: [...]}` | 子条件全部 true |
| 组合 OR | `{or: [...]}` | 任一子条件 true |
| 空对象 | `{}` | 无条件转换，始终 true（诸如 `Idle → Patrol` 初始过渡）|

**op 枚举**（由 Server rule pkg 消费）：`==` / `!=` / `>` / `>=` / `<` / `<=`。

**value 类型**：`json.RawMessage`，保留 JSON 原类型（number / string / bool / null）。

**最小冷启动集**（v1.1.1 锚定）：3 条 FSM（`fsm_combat_basic` / `fsm_passive` / `guard`）由 `seedFsmConfigs` 保证。

### 双边契约锚定

破坏性变更：
- `states` 从 `array<object>` 变为 `array<string>`（扁平化）或 object map
- `transitions[].condition` 叶节点字段名变动（`key` / `op` / `value` / `ref_key`）
- `and` / `or` 语义交换或去掉组合能力
- `priority` 从 number 变枚举或删除

非破坏变更：新增 `op` 枚举值、新增 `transitions[]` 可选字段（带 omitempty）。

---

## GET /api/configs/bt_trees

**用途**：服务端启动时一次性拉取所有已启用行为树，按 NPC 模板的 `behavior.bt_refs[state]` 字段查表；BT 执行器用 `BuildFromJSON` 构造树结构。

**调用方**：服务端 `internal/config/http_source.go` → `fetchEndpoint(/api/configs/bt_trees)` → 内存 map[name][]byte；运行期 `LoadBTTree(name)` 返回**原始 JSON 字节**直接喂给 BT 构造器（不经过 ADMIN 侧 Go struct）。

**ADMIN 实现位置**：`backend/internal/handler/export.go` 的 `BTTrees`；`backend/internal/service/bt_tree.go` 的 `ExportAll`；`backend/internal/model/bt_tree.go` 的 `BtTreeExportItem{Name, Config json.RawMessage}` —— **`config` 直接透传 `bt_trees.config` 列**。

**返回状态**：始终 `200 OK`。空 items[] **Server 启动期硬失败**。

### Schema

```json
{
  "items": [
    {
      "name": "bt/combat/attack",
      "config": {
        "type": "sequence",
        "children": [
          {
            "type": "set_bb_value",
            "params": {"key": "current_action", "value": "draw_weapon"}
          },
          {
            "type": "stub_action",
            "params": {"name": "equip_weapon", "result": "success"}
          }
        ]
      }
    }
  ]
}
```

### 字段说明

| 路径 | 类型 | 语义 | 空值 / 可选 |
|---|---|---|---|
| `items[].name` | string | BT 标识，`^[a-z][a-z0-9_/]*$`（**允许斜杠**，如 `bt/combat/attack`；与其他导出端点 `name` 规则不同）| 必填非空 |
| `items[].config` | object | BT 树结构根节点，`bt_trees.config` 列原样透传 | 必填 |
| `items[].config.type` | string | 节点类型（`sequence` / `selector` / `set_bb_value` / `stub_action` / ... 由 `bt_node_types` 表登记）| 必填非空 |
| `items[].config.params` | object | 节点参数；**key 集合由 `bt_node_types.param_schema` 约束**；ADMIN 写时 validator 数据驱动校验 | 可选（叶节点可能无参数）|
| `items[].config.children` | array<object> | 子节点列表（仅 composite / decorator 节点）；每个子节点形如 `config` 结构（递归嵌套）| 可选 |

**节点类型注册机制**：BT 节点类型不是硬编码枚举，由 ADMIN 侧 `bt_node_types` 表管理（`name` + `param_schema` JSON），seed 初始化 `sequence` / `selector` / `set_bb_value` / `stub_action` 等基础节点。新增节点类型走 ADMIN UI 注册 → 自动在前端 `BtTreeForm` 节点编辑器可选。

**BB key 提取**（v1.1 后运行时 BB Key Registry 闭环）：ADMIN 在 `bt_tree` 写时从 `params.key` 子字段递归抽取 BB key 集合 → 写入 `runtime_bb_key_refs` 反查表 —— 禁用或删除运行时 key 时反向阻止。

**最小冷启动集**（v1.1.1 锚定）：6 棵 BT（`bt/combat/idle` / `patrol` / `chase` / `attack` / `bt/passive/wander` / `bt/guard/patrol`）由 `seedBtTrees` 保证。

### 双边契约锚定

破坏性变更：
- `config.type` 从 string 变 enum 对象或 int
- `config.params` 从 object 变 array
- `config.children` 语义变化（复合节点必要性 / 顺序语义）
- 节点类型 `param_schema` 字段集合**必须先在 ADMIN 注册**再由 Server 消费 —— Server 引入新节点类型的 PR 必须同步 ADMIN seed

非破坏变更：`params` 内新增可选 key（带合理默认）、新增节点类型（需 ADMIN seed 同步）。

---

## GET /api/configs/regions

**用途**：服务端启动时一次性拉取所有已启用区域配置；运行期按 `region_id` 查表，在 zone 范围内按 `spawn_table` spawn NPC 实例。

**调用方**：服务端 `internal/config/http_source.go` 的 **`fetchRegionsEndpoint`**（**regions 专用通道**，不走通用 `fetchEndpoint`）→ 内存 map[region_id][]byte；运行期 `LoadRegionConfig(region_id)` / `LoadAllRegionConfigs()` 返回原始 JSON 给 zone 系统。

**ADMIN 实现位置**：`backend/internal/handler/export.go` 的 `Regions`（5 步编排，同 NPCTemplates）；`backend/internal/service/region.go` 的 `ExportRows` / `CollectExportRefs` / `AssembleExportItems` / `BuildExportDanglingError`；`backend/internal/model/region.go` 的 `RegionExportItem{Name, Config RegionExportConfig}`。

**返回状态**：
- **`200 OK`**：装配成功（含空 items[] —— regions 唯一容忍空数据的端点）
- **`500 + code=47011`**：发现悬空 `template_ref`（spawn_table 引用的 NPC 模板不存在或未启用），**Server fail-fast 不启动**，读 details[] 定位

**500 错误响应体**（`errcode.ErrRegionExportDanglingRef`）：
```json
{
  "code": 47011,
  "message": "区域导出失败：存在悬空的 NPC 模板引用，请按 details 修复",
  "details": [
    {
      "npc_name": "village_outskirts",
      "ref_type": "npc_template_ref",
      "ref_value": "missing_template",
      "reason": "missing_or_disabled"
    }
  ]
}
```

**`details[].npc_name` 字段名字段复用警告**：regions 端点复用了 npc_templates 的 `NPCExportDanglingRef` 结构，字段名保留 `npc_name`，但**此语境下实际承载 `region_id`**（Server 侧 `regionsDangling.RegionID` 重命名做运维可读性修正）。这是已锚定的跨端点字段复用妥协，不修。

### Schema

```json
{
  "items": [
    {
      "name": "village_outskirts",
      "config": {
        "region_id": "village_outskirts",
        "name": "村庄外围",
        "region_type": "wilderness",
        "spawn_table": [
          {
            "template_ref": "villager_guard",
            "count": 2,
            "spawn_points": [
              {"x": 10, "z": 20},
              {"x": 15, "z": 20}
            ],
            "wander_radius": 5,
            "respawn_seconds": 60
          }
        ]
      }
    }
  ]
}
```

### 字段说明

| 路径 | 类型 | 语义 | 空值 / 可选 |
|---|---|---|---|
| `items[].name` | string | **envelope 层：区域业务键** = `region_id`（Server HTTPSource 路由用）| 必填非空 |
| `items[].config.region_id` | string | **config 层：冗余回写**的 region_id（消费方便 snapshot 对比；与 envelope.name 值相同）| 必填非空 |
| `items[].config.name` | string | **config 层：display_name**（Server `Zone.Name` 显示名语义）；**与 envelope.name 语义分层不冲突** | 必填非空 |
| `items[].config.region_type` | string | 区域类型枚举（由 ADMIN 字典组 `DictGroupRegionType` 定义，如 `wilderness` / `town` / `dungeon`）| 必填非空 |
| `items[].config.spawn_table` | array<object> | spawn 规则列表；**ADMIN 允许空数组**（占位区域，无 NPC spawn）| 可为 `[]` |
| `items[].config.spawn_table[].template_ref` | string | NPC 模板名；**ADMIN 写时 + 导出时双重校验 enabled 状态**（悬空 → 47011）| 必填非空 |
| `items[].config.spawn_table[].count` | number | spawn 数量 | 必填 >= 1 |
| `items[].config.spawn_table[].spawn_points` | array<object> | 候选 spawn 坐标；count 小于点数时 Server 侧随机挑选 | 必填非空 |
| `items[].config.spawn_table[].spawn_points[].x` | number | X 坐标 | 必填 |
| `items[].config.spawn_table[].spawn_points[].z` | number | Z 坐标（**对齐 Server zone.go `Position{X, Z float64}`，不含 y 维度**）| 必填 |
| `items[].config.spawn_table[].wander_radius` | number | spawn 点周围游荡半径（米）| 必填 |
| `items[].config.spawn_table[].respawn_seconds` | number | 死亡后重刷延迟（秒）；**v3 roadmap 占位字段，本期 Server 不消费**，前端 help-text 已标注 | 必填（可为 0）|

**双层 name 分层语义**（重要）：
- `items[].name` —— **business key**，Server HTTPSource 用作 `regions[region_id][]byte` 的 map key
- `items[].config.name` —— **display name**，装填进 Server `Zone.Name` 做运行期日志/UI 展示
- 两者写入同一条记录但来自 ADMIN 不同列：envelope 来自 `regions.region_id`，config.name 来自 `regions.display_name`

**NPC disable fan-out 契约**：NPC 模板 `enabled=false` 会同步影响 `/npc_templates` **和** `/regions` 两个端点 —— ADMIN 侧 single source of truth，Server 不会出现某端点看到 enabled 另一端点看到 disabled 的中间态。详见 memory `project_admin_single_source_disabled_fanout.md`。

### 双边契约锚定

破坏性变更：
- envelope 层 `name` 与 config 层 `name` 语义交换（`region_id` 与 `display_name` 互换）
- `spawn_table` 从 array 变 object（key 是 template_ref）
- `spawn_points` 坐标新增 `y` 维度（目前 Server zone.go 明确 2D）
- 500 错误 `details[].npc_name` 字段名修正为 `region_id`（需 Server 侧同步改 `regionsDangling` 字段）—— 记入 v2.0 breaking，不在本期动
- 错误码 47011 语义变动

非破坏变更：`spawn_table[]` 内新增可选字段（带 omitempty）、`region_type` 字典新增枚举值、`respawn_seconds` 被 Server 激活消费（纯添加行为）。

### 已知数据契约遗留

- **`details[].npc_name` 字段名错位**：regions 端点 500 错误体的 `npc_name` 实际承载 `region_id`。原因：service 复用 `NPCExportDanglingRef` 结构未拆独立类型。当前 Server 侧 `regionsDangling` 手动重命名规避。清理路径：下次双边契约 breaking 窗口一次性拆独立 `RegionExportDanglingRef` 类型 + 字段名改 `region_id`。
