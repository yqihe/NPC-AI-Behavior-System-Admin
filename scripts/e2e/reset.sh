#!/usr/bin/env bash
#
# scripts/e2e/reset.sh — Admin 侧 e2e 清空 + base seed
#
# 动作顺序（配套 docs/specs/e2e-full-integration-2026-04-21/execution-plan.md Step 1）：
#   1. Redis FLUSHDB
#   2. 全量 TRUNCATE 业务表（migration 定义的全部业务表）
#   3. cmd/seed 播 base 数据（fields + templates + NPCs + FSM + BT + events + bb_keys + dicts）
#   4. 再 TRUNCATE npcs / npc_bt_refs / regions 三张表（清掉 cmd/seed 默认 NPC + region，为 e2e 腾空间）
#
# 执行后：base 数据齐全，NPCs/regions 为空，等待 seed.sh 灌 e2e 数据。
#
# Windows Git Bash 适配：所有 jq 抽取经 `tr -d '\r'`。

set -euo pipefail

REPO_ROOT=$(git rev-parse --show-toplevel)
cd "$REPO_ROOT"

# ──────────────────────────────────────────────
# 前置：docker compose 就绪
# ──────────────────────────────────────────────

for svc in admin-backend mysql redis; do
	docker compose ps --services --filter "status=running" \
		| grep -qxF "$svc" \
		|| { echo "✗ $svc 未运行（docker compose up -d 后重试）"; exit 1; }
done

echo "=== [前置] docker compose 三件套就绪 ==="

# ──────────────────────────────────────────────
# Step 1：Redis FLUSHDB
# ──────────────────────────────────────────────

echo
echo "=== Step 1: Redis FLUSHDB ==="
docker compose exec -T redis redis-cli FLUSHDB >/dev/null
echo "[✓] Redis 已清空"

# ──────────────────────────────────────────────
# Step 2：全量 TRUNCATE 业务表
#
# 清单对齐 backend/migrations/*.sql。
# 未列入：schema_migrations（migration 自身状态表）。
# 顺序：子表 → 父表（FK 友好；但表都用软删或无 FK 也无所谓）。
# ──────────────────────────────────────────────

echo
echo "=== Step 2: 全量 TRUNCATE 业务表 ==="
docker compose exec -T mysql mysql -uroot -proot npc_ai_admin <<'SQL'
SET FOREIGN_KEY_CHECKS = 0;
TRUNCATE TABLE field_refs;
TRUNCATE TABLE schema_refs;
TRUNCATE TABLE npc_bt_refs;
TRUNCATE TABLE bt_node_type_refs;
TRUNCATE TABLE runtime_bb_key_refs;
TRUNCATE TABLE npcs;
TRUNCATE TABLE templates;
TRUNCATE TABLE fields;
TRUNCATE TABLE fsm_configs;
TRUNCATE TABLE fsm_state_dicts;
TRUNCATE TABLE bt_trees;
TRUNCATE TABLE bt_node_types;
TRUNCATE TABLE event_types;
TRUNCATE TABLE event_type_schema;
TRUNCATE TABLE runtime_bb_keys;
TRUNCATE TABLE regions;
TRUNCATE TABLE dictionaries;
SET FOREIGN_KEY_CHECKS = 1;
SQL
echo "[✓] 全部业务表已 TRUNCATE"

# ──────────────────────────────────────────────
# Step 3：cmd/seed 播 base 数据
# ──────────────────────────────────────────────

echo
echo "=== Step 3: cmd/seed 播 base 数据 ==="
(cd backend && go run ./cmd/seed -config config.yaml) 2>&1 | tee /tmp/e2e_seed.log

# 必须齐的 7 段输出
for pattern in "字段写入完成" "模板写入完成" "NPC 写入完成" "FSM 配置写入完成" "行为树写入完成" "事件类型写入完成" "运行时 BB Key 写入完成"; do
	grep -q "$pattern" /tmp/e2e_seed.log \
		|| { echo "✗ cmd/seed 缺 '$pattern' 段"; exit 1; }
done
echo "[✓] cmd/seed 7 段输出齐全"

# ──────────────────────────────────────────────
# Step 4：二次 TRUNCATE —— 只清 cmd/seed 默认 NPCs + regions
#
# base 保留：fields / templates / FSM / BT / events / bb_keys / dicts
# e2e 腾空：npcs / npc_bt_refs / regions
# ──────────────────────────────────────────────

echo
echo "=== Step 4: 清 cmd/seed 默认 NPCs + regions（为 e2e 腾位）==="
docker compose exec -T mysql mysql -uroot -proot npc_ai_admin <<'SQL'
SET FOREIGN_KEY_CHECKS = 0;
TRUNCATE TABLE npcs;
TRUNCATE TABLE npc_bt_refs;
TRUNCATE TABLE regions;
SET FOREIGN_KEY_CHECKS = 1;
SQL
echo "[✓] npcs / npc_bt_refs / regions 已清空"

# Redis 再清一次（cmd/seed 运行时可能回填了列表缓存）
docker compose exec -T redis redis-cli FLUSHDB >/dev/null
echo "[✓] Redis 二次清空"

# ──────────────────────────────────────────────
# 自检：base 数据齐 + e2e 表空
# ──────────────────────────────────────────────

echo
echo "=== [自检] base 数据齐 + e2e 表空 ==="

mysql_q() {
	docker compose exec -T mysql mysql -uroot -proot npc_ai_admin -N -B -e "$1" 2>/dev/null | tr -d '\r'
}

FIELD_N=$(mysql_q "SELECT COUNT(*) FROM fields WHERE deleted=0")
TEMPLATE_N=$(mysql_q "SELECT COUNT(*) FROM templates WHERE deleted=0")
FSM_N=$(mysql_q "SELECT COUNT(*) FROM fsm_configs WHERE deleted=0")
BT_N=$(mysql_q "SELECT COUNT(*) FROM bt_trees WHERE deleted=0")
EVENT_N=$(mysql_q "SELECT COUNT(*) FROM event_types WHERE deleted=0")
RBK_N=$(mysql_q "SELECT COUNT(*) FROM runtime_bb_keys WHERE deleted=0")
NPC_N=$(mysql_q "SELECT COUNT(*) FROM npcs WHERE deleted=0")
REGION_N=$(mysql_q "SELECT COUNT(*) FROM regions WHERE deleted=0")

echo "  fields=$FIELD_N  templates=$TEMPLATE_N  fsm_configs=$FSM_N  bt_trees=$BT_N  event_types=$EVENT_N  runtime_bb_keys=$RBK_N"
echo "  npcs=$NPC_N  regions=$REGION_N  （应为 0）"

[ "$FIELD_N" -ge "9" ] || { echo "✗ fields <9"; exit 1; }
[ "$TEMPLATE_N" = "4" ] || { echo "✗ templates != 4"; exit 1; }
[ "$FSM_N" = "3" ] || { echo "✗ fsm_configs != 3"; exit 1; }
[ "$BT_N" = "6" ] || { echo "✗ bt_trees != 6"; exit 1; }
[ "$EVENT_N" = "5" ] || { echo "✗ event_types != 5"; exit 1; }
[ "$RBK_N" = "31" ] || { echo "✗ runtime_bb_keys != 31"; exit 1; }
[ "$NPC_N" = "0" ] || { echo "✗ npcs 非空 ($NPC_N)"; exit 1; }
[ "$REGION_N" = "0" ] || { echo "✗ regions 非空 ($REGION_N)"; exit 1; }

echo
echo "=== reset 完成：base 就绪 + e2e 腾空，可运行 seed.sh ==="
