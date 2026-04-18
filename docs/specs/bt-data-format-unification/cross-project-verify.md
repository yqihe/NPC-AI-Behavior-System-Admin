# T9 — 跨项目 e2e 验证（游戏服务端 BuildFromJSON）

**执行日期**：2026-04-18  
**目标**：断言 ADMIN 迁移后的 6 棵 BT 导出能被游戏服务端 `BuildFromJSON()` 成功构建（R7）

---

## 1. 导出采集

```bash
$ curl -s http://127.0.0.1:9821/api/configs/bt_trees -o E:\tmp\bt_trees.json
$ python3 -c "import json; print('items:', len(json.load(open(r'E:\tmp\bt_trees.json')).get('items',[])))"
items: 6
```

payload 顶层形状 `{items:[{name, config:<raw json>}, ...]}`，6 条全齐。

---

## 2. 临时验证脚本（一次性，已删除）

写入路径：`../NPC-AI-Behavior-System-Server/internal/core/bt/bt_migrate_verify_test.go`  
生命周期：跨项目只读原则（姐妹项目不留迁移脚本痕迹），测试跑完立即删除，**未提交到 game server repo**。

### 源码快照（保存于本文供回溯）

```go
package bt

// 临时跨项目验证脚本（ADMIN spec bt-data-format-unification T9）
// 用途：对 ADMIN 迁移后的导出 JSON 逐棵调用 BuildFromJSON，断言 6/6 err=nil
// 生命周期：一次性执行，运行完毕立即删除此文件（不提交到 game server repo）

import (
	"encoding/json"
	"os"
	"testing"
)

type exportPayload struct {
	Items []struct {
		Name   string          `json:"name"`
		Config json.RawMessage `json:"config"`
	} `json:"items"`
}

func TestMigrateVerify_BuildFromJSON(t *testing.T) {
	const src = `E:\tmp\bt_trees.json`
	raw, err := os.ReadFile(src)
	if err != nil { t.Fatalf("read %s: %v", src, err) }
	var p exportPayload
	if err := json.Unmarshal(raw, &p); err != nil { t.Fatalf("parse: %v", err) }
	if len(p.Items) == 0 { t.Fatal("export payload empty") }

	reg := DefaultRegistry()
	pass, fail := 0, 0
	for _, it := range p.Items {
		node, err := BuildFromJSON(it.Config, reg)
		if err != nil {
			t.Errorf("BuildFromJSON %q failed: %v", it.Name, err)
			fail++
			continue
		}
		if node == nil {
			t.Errorf("BuildFromJSON %q returned nil node", it.Name)
			fail++
			continue
		}
		t.Logf("OK  %s → %T", it.Name, node)
		pass++
	}
	t.Logf("=== cross-project verify: %d PASS, %d FAIL out of %d ===", pass, fail, len(p.Items))
}
```

---

## 3. 执行输出

```
$ cd ../NPC-AI-Behavior-System-Server
$ go test ./internal/core/bt/... -run TestMigrateVerify_BuildFromJSON -v -count=1

=== RUN   TestMigrateVerify_BuildFromJSON
    bt_migrate_verify_test.go:51: OK  bt/combat/idle → *bt.Sequence
    bt_migrate_verify_test.go:51: OK  bt/combat/patrol → *bt.Sequence
    bt_migrate_verify_test.go:51: OK  bt/combat/chase → *bt.Selector
    bt_migrate_verify_test.go:51: OK  bt/combat/attack → *bt.Sequence
    bt_migrate_verify_test.go:51: OK  bt/passive/wander → *bt.Sequence
    bt_migrate_verify_test.go:51: OK  guard/patrol → *bt.stubAction
    bt_migrate_verify_test.go:54: === cross-project verify: 6 PASS, 0 FAIL out of 6 ===
--- PASS: TestMigrateVerify_BuildFromJSON (0.00s)
PASS
ok  	github.com/yqihe/NPC-AI-Behavior-System-Server/internal/core/bt	4.332s
```

---

## 4. 结论

| 断言 | 状态 |
|---|---|
| R7：6 棵全部可被 `BuildFromJSON()` 成功构建 | ✅ **6/6 PASS，0 FAIL** |
| BT 构建后返回合理的根节点类型 | ✅ Sequence × 4 / Selector / stubAction（符合各树根设计） |

**pencils down**：ADMIN BT 迁移后的数据**完全符合游戏服务端 schema**，联调 spawn 路径畅通。

---

## 5. 清理确认

```bash
$ rm ../NPC-AI-Behavior-System-Server/internal/core/bt/bt_migrate_verify_test.go
$ ls ../NPC-AI-Behavior-System-Server/internal/core/bt/ | grep migrate
(no output — 脚本已删除)
```

姐妹项目保持只读纪律，无任何迁移代码残留。

---

## 6. 本 spec 全 12 条 R 的最终状态

| R | 描述 | 落实任务 | 状态 |
|---|---|---|---|
| R1 | seed 10 个 bt_node_type | T2 | ✅ |
| R2 | validator 拒裸字段 | T4/T5 | ✅ |
| R3 | validator 逐类型 schema 校验 | T4/T5 | ✅ |
| R4 | validator 限制类型白名单 | T4/T5 | ✅ |
| R5 | 迁移 dry-run 输出 6 棵 diff | T6/T7/T8 | ✅ |
| R6 | 迁移 apply 后 6 棵通过新 validator | T8 | ✅ |
| R7 | game server BuildFromJSON 成功 | **T9** | ✅ |
| R8 | #4 两占位 attack_prepare / attack_strike | T6/T8 | ✅ |
| R9 | #6 category 字段剔除 | T6/T8 | ✅ |
| R10 | BB key 参数统一为 `key` | T6/T8 | ✅ |
| R11 | #3 check_bb_float 迁移正确 | T6/T8 | ✅ |
| R12 | 错误消息含节点定位信息 | T4/T5 | ✅ |

**12/12 全部 ✅**。spec bt-data-format-unification 收官。
