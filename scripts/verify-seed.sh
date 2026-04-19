#!/usr/bin/env bash
#
# scripts/verify-seed.sh — 外部契约数据 seed 端到端验收
# 对齐 docs/specs/external-contract-admin-shape-alignment/ 的 R1-R11 + R13.1-R13.2
#
# 本脚本非破坏性：不 wipe DB；依赖 docker compose 已启动（admin-backend + mysql + redis）。
# 首次运行（空 DB）会看到"新增 9 条"；后续运行（含数据）看到"跳过 9 条"——均为 PASS。
#
# Windows Git Bash 适配：所有 jq 提取必须经 `tr -d '\r'` 去除 CRLF 污染
#   （对齐 memory feedback_bash_utf8_curl.md 精神）
#
# 使用：
#   bash scripts/verify-seed.sh
# 环境变量（可选）：
#   ADMIN_API  默认 http://localhost:9821

set -euo pipefail

ADMIN_API=${ADMIN_API:-http://localhost:9821}
BASELINE=docs/integration/snapshot-section-4.json

REPO_ROOT=$(git rev-parse --show-toplevel)
cd "$REPO_ROOT"

# ──────────────────────────────────────────────
# 前置检查
# ──────────────────────────────────────────────

for tool in docker curl jq diff; do
	command -v "$tool" >/dev/null || { echo "✗ 缺少工具: $tool"; exit 1; }
done
[ -f "$BASELINE" ] || { echo "✗ 找不到 snapshot 基线: $BASELINE"; exit 1; }

# docker compose 运行状态（服务列表走精确匹配，不依赖 grep 子串）
for svc in admin-backend mysql; do
	docker compose ps --services --filter "status=running" \
		| grep -qxF "$svc" \
		|| { echo "✗ $svc 未在运行（docker compose up -d 后重试）"; exit 1; }
done

echo "=== [前置] 工具链 + docker compose 就绪 ==="

# ──────────────────────────────────────────────
# Step 1：首次 / 当前 seed 运行
# ──────────────────────────────────────────────

echo
echo "=== Step 1: 运行 seed ==="
(cd backend && go run ./cmd/seed -config config.yaml) 2>&1 | tee /tmp/verify_seed_run1.log

# 三段输出必须齐全
for pattern in "字段写入完成" "模板写入完成" "NPC 写入完成"; do
	grep -q "$pattern" /tmp/verify_seed_run1.log \
		|| { echo "✗ seed 输出缺 '$pattern' 段"; exit 1; }
done
# R9：缓存清理 ⚠️ 提示
grep -q "⚠️" /tmp/verify_seed_run1.log \
	|| { echo "✗ R9 失败：无 ⚠️ 缓存清理提示"; exit 1; }
echo "[✓] seed 三段输出齐全 + R9 ⚠️ 提示存在"

# ──────────────────────────────────────────────
# Step 2：DB 行数核对（R2 / R3 / R4）
# ──────────────────────────────────────────────

echo
echo "=== Step 2: DB 行数核对 ==="

mysql_q() {
	docker compose exec -T mysql mysql -uroot -proot npc_ai_admin -N -B -e "$1" 2>/dev/null \
		| tr -d '\r'
}

FIELD_COUNT=$(mysql_q "SELECT COUNT(*) FROM fields WHERE name IN ('max_hp','move_speed','perception_range','attack_power','defense','aggression','is_boss','loot_table','hp') AND deleted=0")
[ "$FIELD_COUNT" = "9" ] \
	|| { echo "✗ R2 失败：期望 9 字段，实际 $FIELD_COUNT"; exit 1; }
echo "[✓] R2: fields 表含 9 目标字段"

HP_ENABLED=$(mysql_q "SELECT enabled FROM fields WHERE name='hp' AND deleted=0")
[ "$HP_ENABLED" = "0" ] \
	|| { echo "✗ OQ3-A 失败：hp 字段 enabled 应为 0，实际 $HP_ENABLED"; exit 1; }
echo "[✓] OQ3-A: hp 孤儿字段 enabled=0"

LOOT_EXPOSE=$(mysql_q "SELECT expose_bb FROM fields WHERE name='loot_table' AND deleted=0")
[ "$LOOT_EXPOSE" = "0" ] \
	|| { echo "✗ R8 失败：loot_table.expose_bb 应为 0，实际 $LOOT_EXPOSE"; exit 1; }
echo "[✓] R8: loot_table.expose_bb=false"

TEMPLATE_COUNT=$(mysql_q "SELECT COUNT(*) FROM templates WHERE name IN ('warrior_base','ranger_base','passive_npc','tpl_guard') AND deleted=0")
[ "$TEMPLATE_COUNT" = "4" ] \
	|| { echo "✗ R3 失败：期望 4 模板，实际 $TEMPLATE_COUNT"; exit 1; }
echo "[✓] R3: templates 表含 4 目标模板"

NPC_COUNT=$(mysql_q "SELECT COUNT(*) FROM npcs WHERE name IN ('wolf_common','wolf_alpha','goblin_archer','villager_merchant','villager_guard','guard_basic') AND deleted=0")
[ "$NPC_COUNT" = "6" ] \
	|| { echo "✗ R4 失败：期望 6 NPC，实际 $NPC_COUNT"; exit 1; }
echo "[✓] R4: npcs 表含 6 目标 NPC"

# 模板引用应 ≥ 19（warrior_base 8 + ranger_base 7 + passive_npc 4 + tpl_guard 0 = 19）
TEMPLATE_FIELD_REFS=$(mysql_q "SELECT COUNT(*) FROM field_refs WHERE ref_type='template'")
[ "$TEMPLATE_FIELD_REFS" -ge "19" ] \
	|| { echo "✗ field_refs (template) 期望 ≥19，实际 $TEMPLATE_FIELD_REFS"; exit 1; }
echo "[✓] field_refs(template) ≥19 (实际 $TEMPLATE_FIELD_REFS)"

# ──────────────────────────────────────────────
# Step 3：导出契约对齐基线（R4 / R5 / R11 / R13.1）
# ──────────────────────────────────────────────

echo
echo "=== Step 3: 导出契约 vs snapshot §4 基线 ==="

curl -sf "$ADMIN_API/api/configs/npc_templates" > /tmp/verify_export.json \
	|| { echo "✗ /api/configs/npc_templates 请求失败"; exit 1; }

jq -S '.items | sort_by(.name)' /tmp/verify_export.json  > /tmp/verify_export_sorted.json
jq -S '.items | sort_by(.name)' "$BASELINE"              > /tmp/verify_baseline_sorted.json

if diff /tmp/verify_export_sorted.json /tmp/verify_baseline_sorted.json > /tmp/verify_diff.txt; then
	echo "[✓] R4/R11: 导出与 snapshot §4 基线逐字节一致"
else
	echo "✗ R4/R11: 导出与基线偏离："
	head -50 /tmp/verify_diff.txt
	exit 1
fi

# R13.1：guard_basic.config.fields.hp = 100
HP_VAL=$(jq -r '.items[] | select(.name=="guard_basic") | .config.fields.hp' /tmp/verify_export.json | tr -d '\r')
[ "$HP_VAL" = "100" ] \
	|| { echo "✗ R13.1 失败：guard_basic.hp 应为 100，实际 $HP_VAL"; exit 1; }
echo "[✓] R13.1: 导出 guard_basic.hp=100（方案 A 路径打通）"

# ──────────────────────────────────────────────
# Step 4：UI 过滤语义（R13.2）
#
# 说明：
#   • 字段列表接口是 POST /api/v1/fields/list（body JSON 带 enabled 过滤），
#     不是 GET 查询参数；ADMIN 全部 list 接口统一走 POST（router.go 约定）
#   • 用 `any(. == "hp")` 做精确匹配而非 `contains(["hp"])`——后者在字符串数组上
#     退化为子串匹配，`"max_hp"` 里含 `"hp"` 会假阳
# ──────────────────────────────────────────────

echo
echo "=== Step 4: R13.2 UI enabled 过滤语义 ==="

HP_IN_ENABLED=$(curl -sf -X POST "$ADMIN_API/api/v1/fields/list" \
	-H "Content-Type: application/json" \
	-d '{"enabled": true, "page": 1, "page_size": 100}' \
	| jq -r '.data.items | map(.name) | any(. == "hp")' | tr -d '\r')
[ "$HP_IN_ENABLED" = "false" ] \
	|| { echo "✗ R13.2 失败：enabled=true 列表不应含 hp"; exit 1; }

HP_IN_DISABLED=$(curl -sf -X POST "$ADMIN_API/api/v1/fields/list" \
	-H "Content-Type: application/json" \
	-d '{"enabled": false, "page": 1, "page_size": 100}' \
	| jq -r '.data.items | map(.name) | any(. == "hp")' | tr -d '\r')
[ "$HP_IN_DISABLED" = "true" ] \
	|| { echo "✗ R13.2 失败：enabled=false 列表应含 hp"; exit 1; }

echo "[✓] R13.2: enabled=true 隐藏 hp，enabled=false 暴露 hp"

# ──────────────────────────────────────────────
# Step 5：幂等重跑（R7）
# ──────────────────────────────────────────────

echo
echo "=== Step 5: 幂等重跑 ==="
(cd backend && go run ./cmd/seed -config config.yaml) 2>&1 | tee /tmp/verify_seed_run2.log

grep -q "字段写入完成：新增 0 条，跳过 9 条" /tmp/verify_seed_run2.log \
	|| { echo "✗ R7 失败：字段重跑非全跳过"; exit 1; }
grep -q "模板写入完成：新增 0 条，跳过 4 条" /tmp/verify_seed_run2.log \
	|| { echo "✗ R7 失败：模板重跑非全跳过"; exit 1; }
grep -q "NPC 写入完成：新增 0 条，跳过 6 条" /tmp/verify_seed_run2.log \
	|| { echo "✗ R7 失败：NPC 重跑非全跳过"; exit 1; }
echo "[✓] R7: 幂等重跑全跳过（9+4+6）"

# ──────────────────────────────────────────────
# 收尾
# ──────────────────────────────────────────────

echo
echo "=== 自动化验收 PASS ==="
echo
echo "⚠️  手动项（无法脚本化）："
echo "  • R13.3: 在 ADMIN UI（http://localhost:3000）新建 NPC → 选 tpl_guard 模板 →"
echo "    确认字段表单区域**不含 hp 字段栏**"
echo "  • 若上述预期破了，参考 docs/specs/external-contract-admin-shape-alignment/ops-runbook.md"
