# /integration — ADMIN 与游戏服务端联调

新增 NPC 类型、事件类型时，ADMIN 平台与游戏服务端的协作流程。分三步，每步必须等确认后才能继续。

## Usage
```
/integration <简要描述要新增什么>
```

例：`/integration 新增 guard 守卫 NPC + earthquake 地震事件`

---

## 第一步：方案确认（ADMIN ↔ 服务端对齐）

ADMIN 平台这边提出新增方案，用自然语言描述（不写 JSON），发给游戏服务端确认。

### 产出格式

```
## 联调方案：<标题>

### 新增事件类型
- <事件名>：<一句话描述>。威胁等级 <数值>，持续 <秒数> 秒，传播方式 <视觉/听觉/全局>，范围 <米数>

### 新增状态机
- <状态机名>：状态有 <状态1>、<状态2>、<状态3>
  - <状态1> → <状态2>：当 <条件描述>
  - <状态2> → <状态3>：当 <条件描述>
  - <状态3> → <状态1>：当 <条件描述>
  - 初始状态：<状态名>

### 新增行为树
- <NPC名/状态名>：<这棵树做什么>
  （列出每棵树）

### 新增 NPC 类型
- <NPC名>：使用状态机 <名>，视觉范围 <米>，听觉范围 <米>
  - <状态1> → <行为树名1>
  - <状态2> → <行为树名2>
  - <状态3> → <行为树名3>
```

### 需要服务端确认的内容
- 状态名大小写是否正确（bt_refs 的 key 必须和 FSM states 完全一致）
- 行为树节点用到的 BB Key 是否都在服务端白名单内
- 事件的传播方式和参数是否合理
- 其他你觉得需要对齐的点

**→ 发给服务端，等确认。确认后进入第二步。**

---

## 第二步：页面操作（ADMIN 写入 MongoDB）

服务端确认后，指导用户在 ADMIN 网页上操作。

### 产出格式

按创建顺序（事件 → 状态机 → 行为树 → NPC）逐步说明：

```
### 操作 1：创建事件类型 <名称>
1. 点击左侧「1. 事件管理」
2. 点击「新建事件」
3. 填写：
   - 事件名称：<值>
   - 威胁等级：<值>
   - 持续时间：<值>
   - 传播方式：<选项>
   - 传播范围：<值>
4. 点击「保存」

### 操作 2：创建状态机 <名称>
...（同样格式）

### 操作 3-N：创建行为树 <名称>
...

### 操作 N+1：创建 NPC 类型 <名称>
...
```

### 注意事项
- 严格按顺序操作（NPC 依赖状态机和行为树）
- 每步保存后确认列表页能看到新记录
- 行为树的节点编辑步骤要详细到每个子节点怎么加

### 操作完成后：验证 MongoDB 数据

用户操作完成后，ADMIN 这边先自行验证数据已正确写入 MongoDB，再通知服务端。

验证方法：用 mongosh 或 ADMIN 后端日志确认：
```bash
# 连接 MongoDB 检查数据
docker exec -it <mongo容器名> mongosh npc_ai --eval "
  print('=== 事件类型 ===');
  db.event_types.find({}, {_id:0, name:1}).forEach(d => print('  ' + d.name));
  print('=== 状态机 ===');
  db.fsm_configs.find({}, {_id:0, name:1}).forEach(d => print('  ' + d.name));
  print('=== 行为树 ===');
  db.bt_trees.find({}, {_id:0, name:1}).forEach(d => print('  ' + d.name));
  print('=== NPC 类型 ===');
  db.npc_types.find({}, {_id:0, name:1}).forEach(d => print('  ' + d.name));
"
```

确认新创建的记录都在列表中后，通知服务端：**「配置已写入 MongoDB `npc_ai` 库，你那边重启验证。」**

**→ 进入第三步。**

---

## 第三步：交接验证（服务端接手）

ADMIN 这边的工作到此结束。剩下是服务端的事：

1. 服务端设置 `NPC_MONGO_URI` 指向同一个 MongoDB，重启
2. 服务端检查启动日志，确认新配置加载成功
3. 服务端通过 WebSocket API 测试（spawn_npc、publish_event、query_npc）
4. 如果发现 ADMIN 写入的数据格式有问题，反馈给 ADMIN 修复

### ADMIN 只在以下情况需要介入
- 服务端反馈数据格式不对（ADMIN 的校验器或表单有 bug）
- 服务端反馈某个字段值不合理（需要在页面上重新编辑）

**→ 服务端验证通过 = 联调完成。**

---

## 检查清单（每次联调前过一遍）

- [ ] 事件类型的字段类型：default_severity 和 default_ttl 是浮点数，range 是浮点数
- [ ] FSM 状态名首字母大写（与服务端现有 civilian/police 保持一致）
- [ ] bt_refs 的 key 和 FSM states 的 name 完全一致（大小写敏感）
- [ ] 行为树名格式：`<npc类型>/<状态小写>`，如 `guard/patrol`
- [ ] 行为树中 set_bb_value / check_bb_float / check_bb_string 的 key 在白名单内
- [ ] stub_action 的 result 只能是 success / failure / running
- [ ] condition 中的 op 只能是 == / != / > / >= / < / <= / in
- [ ] NPC 的每个状态都绑定了行为树（不能有遗漏）
- [ ] ADMIN 的 MongoDB 和服务端是同一个实例（database: npc_ai）
