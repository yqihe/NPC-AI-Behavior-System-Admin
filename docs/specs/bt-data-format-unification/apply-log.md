# T8 — dry-run + apply 端到端执行日志

**执行日期**：2026-04-18  
**执行环境**：docker compose（admin-backend / mysql / redis），rebuild admin-backend 确保新 validator 生效  
**执行者**：Claude（在 spec-execute T8 驱动下）

---

## 0. 前置准备

```bash
$ docker compose up -d --build admin-backend
# admin-backend rebuilt + healthy ✓
# mysql/redis healthy ✓

$ curl -s http://127.0.0.1:9821/health
{"status":"ok"}

$ docker exec npc-admin-mysql mysql ... -e "SELECT COUNT(*) FROM bt_node_types WHERE deleted=0"
10  # T2 seed 已落库（8 旧 + move_to + flee_from）
```

### 初始 BT 状态（全部 enabled=1）
```
id  name                version  enabled
1   bt/combat/idle      2        1
2   bt/combat/patrol    2        1
3   bt/combat/chase     2        1
4   bt/combat/attack    5        1
5   bt/passive/wander   4        1
6   guard/patrol        4        1
```

---

## 1. 关键修复：T7 API 端点纠正

**T8 执行中发现 T7 bug**：main.go 原假设 ADMIN 用 RESTful `PUT /api/v1/bt-trees/:id`，实际 ADMIN 所有写路由是 `POST /api/v1/bt-trees/update`。

**修复**：`backend/cmd/bt-migrate/main.go` URL 改为 `POST /api/v1/bt-trees/update`，body 保持 `UpdateBtTreeRequest`（ID/Version/DisplayName/Description/Config）。

**为何 T7 verify 未捕获**：T7 verify 只跑 dry-run（零 PUT），apply 路径没执行。T8 apply 首次触发了真 HTTP 调用，bug 显现。经验：**编码中引入的未验证网络调用假设，必须在 verify 阶段跑至少一次真实请求**。

---

## 2. 预处理：禁用 6 棵 BT

ADMIN 红线"编辑前必须停用"。apply 路径依赖 PUT 成功，故先调 toggle-enabled 逐棵禁用。

```bash
$ for id_ver in "1:2" "2:2" "3:2" "4:5" "5:4" "6:4"; do
    curl -s -X POST http://127.0.0.1:9821/api/v1/bt-trees/toggle-enabled \
      -H 'Content-Type: application/json' \
      -d "{\"id\":$id,\"version\":$ver,\"enabled\":false}"
  done
```

全部 6 棵返回 `{"code":0,"data":"操作成功","message":"success"}`；version 各 +1（3/3/3/6/5/5）。

---

## 3. Dry-run 输出（BT #4 示例）

完整 276 行日志存 `/tmp/t8/dry-run.log`（临时路径）。关键示例：

```
=== Tree #4  bt/combat/attack  (version=6, enabled=false) ===
[BEFORE]
  {
    "type": "sequence",
    "children": [
      { "op": ">", "key": "perception_range", "type": "check_bb_float", "value": 0 },
      { "type": "stub_action" },
      { "type": "stub_action" }
    ]
  }
[AFTER]
  {
    "children": [
      { "params": { "key": "perception_range", "op": "\u003e", "value": 0 }, "type": "check_bb_float" },
      { "params": { "name": "attack_prepare", "result": "success" }, "type": "stub_action" },
      { "params": { "name": "attack_strike", "result": "success" }, "type": "stub_action" }
    ],
    "type": "sequence"
  }
[CHANGES] 2
  - $.children[1]: bt/combat/attack 空 stub_action 填占位 name=attack_prepare, result=success
  - $.children[2]: bt/combat/attack 空 stub_action 填占位 name=attack_strike, result=success
```

**Summary**：`6/6 trees transformed (dry-run — 未写入 DB)`。

**人眼审阅签字**：审阅通过。#4 两个占位 attack_prepare / attack_strike 位置精确；其它 5 棵的 action → params.name + 补 default result 迁移正确；#6 category 字段被剔除。

---

## 4. Apply 输出

```
[APPLIED] tree #1 写入成功
[APPLIED] tree #2 写入成功
[APPLIED] tree #3 写入成功
[APPLIED] tree #4 写入成功
[APPLIED] tree #5 写入成功
[APPLIED] tree #6 写入成功

=== Summary ===
6/6 trees transformed, 6 applied
```

exit=0；6/6 全部走通新 validator 并落库。

---

## 5. 后置断言（R6 / R8 / R9 / R10）

### R9：`$.category` 全部 NULL ✅
```sql
SELECT id, JSON_EXTRACT(config, '$.category') FROM bt_trees WHERE deleted=0;
-- 1/2/3/4/5/6 全部返回 NULL
```

### R10：`"target_key"`（右引号避免误匹配 `target_key_x/z`）零命中 ✅
```bash
docker exec ... mysql -e "SELECT config FROM bt_trees WHERE deleted=0" | grep -E '"target_key"' | wc -l
# 0
```

### R8：BT #4 两个占位精确 ✅
```sql
SELECT JSON_EXTRACT(config, '$.children[1].params.name'),
       JSON_EXTRACT(config, '$.children[2].params.name')
FROM bt_trees WHERE id=4;
-- "attack_prepare"  "attack_strike"
```

### R6：GET detail 全部 200 ✅
```
tree #1: HTTP 200
tree #2: HTTP 200
tree #3: HTTP 200
tree #4: HTTP 200
tree #5: HTTP 200
tree #6: HTTP 200
```

---

## 6. 后处理：重新启用 6 棵

apply 把每棵 version 再 +1（4/4/4/7/6/6）。用 toggle-enabled 恢复 enabled=1。

```bash
$ for id_ver in "1:4" "2:4" "3:4" "4:7" "5:6" "6:6"; do
    curl ... toggle-enabled ... {"enabled":true}
  done
```

全部 6 棵返回 `{"code":0,"data":"操作成功","message":"success"}`。

### 最终状态
```
id  name                version  enabled
1   bt/combat/idle      5        1
2   bt/combat/patrol    5        1
3   bt/combat/chase     5        1
4   bt/combat/attack    8        1
5   bt/passive/wander   7        1
6   guard/patrol        7        1
```

6/6 enabled=1，config 全部规范化，游戏服务端导出 API (`/api/configs/bt_trees`) 可消费。

---

## 7. 结论

| 验收 | 状态 |
|---|---|
| R5（dry-run diff 输出） | ✅ 276 行日志覆盖 6 棵完整 BEFORE/AFTER/CHANGES |
| R6（apply 后 GET 全 200） | ✅ 6/6 tree detail 接口返回 200 |
| R8（#4 两占位 name 精确） | ✅ attack_prepare + attack_strike |
| R9（无 `category` 游离字段） | ✅ 6/6 JSON_EXTRACT 为 NULL |
| R10（无 `target_key` 旧名） | ✅ grep 零命中 |

**T8 整体 PASS**。剩余 T9（游戏服务端跨项目 BuildFromJSON 验证）。

## 8. 经验沉淀

- **教训 1**：编码中未验证的网络调用假设必须在 verify 阶段跑真请求，不能仅靠 dry-run（T7 main.go 的 URL 错误直到 T8 apply 才暴露）
- **教训 2**：文档中"6 棵 dirty BT"的实际 enabled=1 状态 spec 中未显式记录，导致 T7 设计时未预见 apply 前需要 disable；后续类似运维类 spec 应把数据状态（enabled/version 等）当作前置条件明确列出
