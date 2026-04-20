# 操作手册：种子数据管理

本手册是 `external-contract-admin-shape-alignment` spec 的运维配套，面向 ADMIN 的运营/联调/开发人员。

## seed 脚本的作用

`backend/cmd/seed` 是 ADMIN 的种子数据写入工具，涵盖：

1. **字典数据**：字段类型 / 分类 / 属性 / FSM 状态分类
2. **FSM 状态字典**：31 条
3. **内置行为树节点类型**：10 条
4. **FSM 配置（seed-fsm-bt-coverage 新增）**：3 条（`fsm_combat_basic` / `fsm_passive` / `guard`），enabled=1
5. **行为树（seed-fsm-bt-coverage 新增）**：6 棵（`bt/combat/{idle,patrol,chase,attack}` / `bt/passive/wander` / `guard/patrol`），enabled=1
6. **事件类型（seed-fsm-bt-coverage batch2 新增）**：5 条（`earthquake` / `explosion` / `fire` / `gunshot` / `shout`），enabled=1。服务端 HTTPSource 对空 items 硬失败，冷启动必须非空
7. **外部契约数据**：14 字段（9 seeded + 5 opt-in bool，api-contract v1.1）+ 4 模板 + 6 NPC（对齐联调 snapshot §4）

执行方式：

```bash
cd backend
go run ./cmd/seed -config config.yaml
```

Seed 采用 `INSERT IGNORE` 语义——**name 已存在则跳过，不覆盖运营手改的 label/constraints**。

运行完成后会打印：

```
⚠️  若 admin-backend 已启动，请重启或清 Redis 缓存以同步新数据
```

新数据对运行中的 admin-backend 不可见（缓存），必须按提示重启或 FLUSHALL。

## guard_basic 恢复流程

### 背景

`guard_basic` 是联调 snapshot §4 冻结的守卫 NPC，其 fields 快照包含历史遗留的 `hp=100`（应为 `max_hp=100`，属数据噪声）。为保 smoke test 回归不破，本 spec 采用**方案 A（孤儿字段）**：

- `hp` 是 `fields` 表的一行，`enabled=0`，**不在任何模板的 fields 数组里**
- `guard_basic` 的 fields JSON 直接引用 `hp` 字段的 id + value=100
- 导出 `GET /api/configs/npc_templates` 返回 `{"hp":100}` 保持 snapshot 形态不变

### ✅ **唯一重建路径是重跑 seed**

若 `guard_basic` 被误删需要恢复：

```bash
cd backend && go run ./cmd/seed -config config.yaml
```

### ❌ 不要尝试的路径

**不要** 在 UI "NPC 管理" 页面新建 NPC 并选 `tpl_guard` 模板试图手建 hp：
- 技术原因：`tpl_guard.fields=[]`，UI 表单按模板 fields 数组渲染，**看不到 hp 字段栏**
- 操作路径：即使选了 tpl_guard，表单里没有 hp 输入框可填

**不要** 在 UI "模板管理" 页面把 `hp` 字段加到 `tpl_guard.fields` 数组：
- 合规原因：这等价于"合法化 hp 字段为模板的一等字段"，**违反本 spec 方案 A** 的核心承诺（hp 不应作为 catalog 里可被策划选择的字段）
- 参考：`docs/specs/external-contract-admin-shape-alignment/design.md` §2 OQ3、R13.3

### 若运营方确实需要新建携带 hp 字段的 NPC

本 spec **不支持此操作**。`hp` 是**一次性历史遗留**，不应扩散。

如果需求真实且无法绕过，走新 spec 路线：讨论是否把 `hp` 合法化进 catalog（等价于方案 D 的退路），或先把 `guard_basic.hp` 清理掉（等待 41008 解封）。

## 41008 解封后的清理

41008 是 ADMIN 的硬约束：**模板被 NPC 引用时字段不可编辑**。这导致当前无法通过 UI 把 `guard_basic` 的 fields 从 `{hp}` 改为 `{max_hp}`。等此约束解封（后续 spec 处理）后：

1. 修改 `guard_basic` 实例：fields 从 `hp` → `max_hp`（直接编辑 NPC 的 fields JSON 或走新 API）
2. 从 `fields` 表删除 `hp` 孤儿行
3. 更新 seed 代码：移除 `fieldNameHp` 常量 + hp fieldSeed + guard_basic 的 hp FieldValues
4. 更新 snapshot 基线 JSON：`docs/integration/snapshot-section-4.json` 对应字段
5. 更新 `docs/architecture/api-contract.md` 的"已知数据噪声"段落（移除 hp 条目）

此流程属**后续 spec 范围**，当前记录为占位指引。相关 memory：`project_guard_basic_hp_deferred.md`。

## 验证工具

`scripts/verify-seed.sh`：端到端验收 seed 产出，自动跑 + DB 计数核对 + 基线 diff + 幂等 + R13.1/R13.2 UI 过滤语义。

```bash
bash scripts/verify-seed.sh
```

失败退出码非 0 并打印原因。**R13.3（UI tpl_guard 表单不含 hp 字段栏）无法脚本化**，需手动在浏览器验证（脚本尾部提示）。

## 相关资料

- spec：`docs/specs/external-contract-admin-shape-alignment/`
- 契约权威：`docs/architecture/api-contract.md`
- 测试基线：`docs/integration/snapshot-section-4.json`（及同目录 README.md）
