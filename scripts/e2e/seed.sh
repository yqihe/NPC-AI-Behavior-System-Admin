#!/usr/bin/env bash
#
# scripts/e2e/seed.sh — Admin 侧 e2e 数据 API 灌入（基于 HTTP API，不走 SQL）
#
# 配套 docs/specs/e2e-full-integration-2026-04-21/execution-plan.md Step 1。
# 前置：reset.sh 已完成（base 就绪 + npcs/regions 空）。
#
# 本脚本动作：
#   1. GET /fields/list 解析 12 个必需 field 的 id
#   2. POST /templates/create 建 e2e_template_full
#   3. POST /npcs/create × 5 建 e2e_{bare,social,memo_emo,full,disabled}
#   4. POST /npcs/toggle-enabled 停用 e2e_disabled
#   5. POST /regions/create × 2 建 e2e_{village,empty}
#   6. 5 次 curl /api/configs/* 核对导出条数
#
# 任一步失败立即 exit 1；log 全程输出到 stderr。

set -euo pipefail

REPO_ROOT=$(git rev-parse --show-toplevel)
cd "$REPO_ROOT"

API=${ADMIN_API:-http://localhost:9821}
API_V1="$API/api/v1"

# ──────────────────────────────────────────────
# 通用辅助
# ──────────────────────────────────────────────

# 发 POST 请求，校验 code=0，返回 .data（流）。code!=0 exit 1。
# 用法：api_post /path '{"key":"value"}' label_for_log
api_post() {
	local path=$1
	local body=$2
	local label=$3

	local resp
	resp=$(curl -sf -X POST "$API_V1$path" \
		-H "Content-Type: application/json" \
		-d "$body") || { echo "✗ [$label] HTTP 请求失败 path=$path"; exit 1; }

	local code
	code=$(echo "$resp" | jq -r '.code' | tr -d '\r')
	if [ "$code" != "0" ]; then
		local msg
		msg=$(echo "$resp" | jq -r '.message' | tr -d '\r')
		echo "✗ [$label] 业务错误 code=$code message=$msg" >&2
		echo "  req: $body" >&2
		exit 1
	fi
	echo "$resp" | jq '.data'
}

# 封装：把字段名列表转成 [{field_id:N, required:false}] JSON
# 依赖已加载的 FIELD_ID_MAP 数组
field_entries_json() {
	local names=("$@")
	local entries="[]"
	for name in "${names[@]}"; do
		local id=${FIELD_IDS[$name]:-}
		[ -n "$id" ] || { echo "✗ unknown field name: $name" >&2; exit 1; }
		entries=$(echo "$entries" | jq ". += [{\"field_id\": $id, \"required\": false}]")
	done
	echo "$entries"
}

# 封装：把 field_name → value 的 KV 对转成 [{field_id:N, value:V}] JSON
# 参数：交替的 name value 对
# value 必须是 JSON 字面量（"string" / true / 100 / ...）
field_values_json() {
	local entries="[]"
	while [ $# -gt 0 ]; do
		local name=$1
		local val=$2
		shift 2
		local id=${FIELD_IDS[$name]:-}
		[ -n "$id" ] || { echo "✗ unknown field name: $name" >&2; exit 1; }
		entries=$(echo "$entries" | jq ". += [{\"field_id\": $id, \"value\": $val}]")
	done
	echo "$entries"
}

# ──────────────────────────────────────────────
# Step 1: 加载全部 fields → field_id 映射
# ──────────────────────────────────────────────

echo "=== Step 1: 加载 fields → id 映射 ==="
FIELDS_RESP=$(curl -sf -X POST "$API_V1/fields/list" \
	-H "Content-Type: application/json" \
	-d '{"page":1,"page_size":100}') || { echo "✗ /fields/list 请求失败"; exit 1; }

# 解析成 bash 关联数组 FIELD_IDS[name]=id
declare -A FIELD_IDS
while IFS=$'\t' read -r name id; do
	FIELD_IDS[$name]=$id
done < <(echo "$FIELDS_RESP" | jq -r '.data.items[] | "\(.name)\t\(.id)"' | tr -d '\r')

# 校验必需字段齐全
REQUIRED_FIELDS=(
	max_hp attack_power defense is_boss loot_table
	move_speed perception_range aggression
	enable_memory enable_emotion enable_needs enable_personality enable_social
	group_id social_role
)
for name in "${REQUIRED_FIELDS[@]}"; do
	[ -n "${FIELD_IDS[$name]:-}" ] || { echo "✗ 缺少字段: $name"; exit 1; }
done
echo "[✓] 15 个必需 field 已全部载入 field_id"

# ──────────────────────────────────────────────
# Step 2: 建 e2e_template_full（12 字段模板）
# ──────────────────────────────────────────────

echo
echo "=== Step 2: 建模板 e2e_template_full ==="
TEMPLATE_FIELDS=$(field_entries_json \
	max_hp attack_power defense is_boss loot_table \
	enable_memory enable_emotion enable_needs enable_personality enable_social \
	group_id social_role)

TEMPLATE_BODY=$(jq -n \
	--arg name "e2e_template_full" \
	--arg label "e2e 全量模板" \
	--arg desc "e2e-full-integration 2026-04-21：含 5 opt-in bool + group_id/social_role + 5 战斗字段" \
	--argjson fields "$TEMPLATE_FIELDS" \
	'{name: $name, label: $label, description: $desc, fields: $fields}')

TEMPLATE_DATA=$(api_post /templates/create "$TEMPLATE_BODY" "create-template")
TEMPLATE_ID=$(echo "$TEMPLATE_DATA" | jq -r '.id' | tr -d '\r')
[ -n "$TEMPLATE_ID" ] && [ "$TEMPLATE_ID" != "null" ] || { echo "✗ template_id 解析失败"; exit 1; }
echo "[✓] e2e_template_full 创建成功 id=$TEMPLATE_ID（创建即 enabled=0，下一步需 toggle-enable）"

# 反直觉点：模板创建默认 enabled=0，NPC 引用前必须先 toggle-enable
# 对齐 memory feedback_admin_write_api_quirks.md
TOGGLE_TPL_BODY=$(jq -n --argjson id "$TEMPLATE_ID" '{id:$id, enabled:true, version:1}')
api_post /templates/toggle-enabled "$TOGGLE_TPL_BODY" "toggle-template" >/dev/null
echo "[✓] e2e_template_full 已 toggle 为 enabled=true"

# ──────────────────────────────────────────────
# Step 3: 建 5 个 e2e NPC
#
# 公共战斗字段值：max_hp=100 attack=15 defense=8 is_boss=false loot="e2e_loot"
# 差异：5 opt-in bool + group_id + social_role
# ──────────────────────────────────────────────

echo
echo "=== Step 3: 建 5 个 e2e NPC ==="

# 公共 bt_refs：覆盖 Idle/Patrol/Attack 三态（Chase 留空验证部分映射合法）
BT_REFS='{"Idle":"bt/combat/idle","Patrol":"bt/combat/patrol","Attack":"bt/combat/attack"}'

# create_npc name label mem emo need pers soc group role
create_npc() {
	local name=$1 label=$2 mem=$3 emo=$4 need=$5 pers=$6 soc=$7 gid=$8 role=$9
	local values
	values=$(field_values_json \
		max_hp 100 attack_power 15 defense 8 is_boss false loot_table '"e2e_loot"' \
		enable_memory "$mem" enable_emotion "$emo" enable_needs "$need" \
		enable_personality "$pers" enable_social "$soc" \
		group_id "\"$gid\"" social_role "\"$role\"")

	local body
	body=$(jq -n \
		--arg name "$name" --arg label "$label" \
		--argjson tpl_id "$TEMPLATE_ID" \
		--argjson values "$values" \
		--arg fsm_ref "fsm_combat_basic" \
		--argjson bt_refs "$BT_REFS" \
		'{name:$name, label:$label, description:"", template_id:$tpl_id,
		  field_values:$values, fsm_ref:$fsm_ref, bt_refs:$bt_refs}')

	local data
	data=$(api_post /npcs/create "$body" "create-npc-$name")
	local npc_id
	npc_id=$(echo "$data" | jq -r '.id' | tr -d '\r')
	echo "[✓] $name 创建成功 id=$npc_id" >&2
	echo "$npc_id"
}

NPC_ID_BARE=$(create_npc e2e_bare     "e2e-bare 全 false"       false false false false false ""           "")
NPC_ID_SOCIAL=$(create_npc e2e_social   "e2e-social 独开"         false false false false true  "e2e_group"  "follower")
NPC_ID_MEMO=$(create_npc e2e_memo_emo "e2e-memory+emotion"      true  true  false false false ""           "")
NPC_ID_FULL=$(create_npc e2e_full     "e2e-5 opt-in 全开"       true  true  true  true  true  "e2e_group"  "leader")
NPC_ID_DISABLED=$(create_npc e2e_disabled "e2e-disabled fan-out" false false false false false ""           "")

# ──────────────────────────────────────────────
# Step 4: 停用 e2e_disabled
# ──────────────────────────────────────────────

echo
echo "=== Step 4: 停用 e2e_disabled ==="
TOGGLE_BODY=$(jq -n --argjson id "$NPC_ID_DISABLED" '{id:$id, enabled:false, version:1}')
api_post /npcs/toggle-enabled "$TOGGLE_BODY" "toggle-e2e_disabled" >/dev/null
echo "[✓] e2e_disabled 已停用"

# ──────────────────────────────────────────────
# Step 5: 建 2 个 region
# ──────────────────────────────────────────────

echo
echo "=== Step 5: 建 2 个 region ==="

REGION_VILLAGE_BODY=$(jq -n '{
	region_id:"e2e_village", display_name:"e2e 村庄", region_type:"wilderness",
	spawn_table:[{
		template_ref:"e2e_bare", count:2,
		spawn_points:[{x:10,z:20},{x:15,z:20}],
		wander_radius:5, respawn_seconds:60
	}]
}')
REGION_VILLAGE_DATA=$(api_post /regions/create "$REGION_VILLAGE_BODY" "create-region-village")
REGION_VILLAGE_ID=$(echo "$REGION_VILLAGE_DATA" | jq -r '.id' | tr -d '\r')
echo "[✓] e2e_village 创建成功 id=$REGION_VILLAGE_ID（enabled=0，下一步 toggle）"
TOGGLE_RV_BODY=$(jq -n --argjson id "$REGION_VILLAGE_ID" '{id:$id, enabled:true, version:1}')
api_post /regions/toggle-enabled "$TOGGLE_RV_BODY" "toggle-e2e_village" >/dev/null
echo "[✓] e2e_village 已 toggle enabled=true"

REGION_EMPTY_BODY=$(jq -n '{
	region_id:"e2e_empty", display_name:"e2e 空广场", region_type:"town",
	spawn_table:[]
}')
REGION_EMPTY_DATA=$(api_post /regions/create "$REGION_EMPTY_BODY" "create-region-empty")
REGION_EMPTY_ID=$(echo "$REGION_EMPTY_DATA" | jq -r '.id' | tr -d '\r')
echo "[✓] e2e_empty 创建成功 id=$REGION_EMPTY_ID（enabled=0，下一步 toggle）"
TOGGLE_RE_BODY=$(jq -n --argjson id "$REGION_EMPTY_ID" '{id:$id, enabled:true, version:1}')
api_post /regions/toggle-enabled "$TOGGLE_RE_BODY" "toggle-e2e_empty" >/dev/null
echo "[✓] e2e_empty 已 toggle enabled=true"

# ──────────────────────────────────────────────
# Step 6: 自检 5 个导出端点
# ──────────────────────────────────────────────

echo
echo "=== Step 6: 自检 /api/configs/* 响应 ==="

assert_count() {
	local ep=$1 expected=$2
	local resp actual
	resp=$(curl -sf "$API/api/configs/$ep") || { echo "✗ /api/configs/$ep 请求失败"; exit 1; }
	actual=$(echo "$resp" | jq '.items | length' | tr -d '\r')
	[ "$actual" = "$expected" ] || { echo "✗ /api/configs/$ep: 期望 $expected 条，实际 $actual"; exit 1; }
	echo "[✓] /api/configs/$ep: $actual 条"
}

assert_count event_types  5
assert_count fsm_configs  3
assert_count bt_trees     6
assert_count npc_templates 4
assert_count regions      2

# 额外校验：e2e_disabled 不在 npc_templates 响应中（disable fan-out）
NPC_EXPORT=$(curl -sf "$API/api/configs/npc_templates")
HAS_DISABLED=$(echo "$NPC_EXPORT" | jq -r '.items | map(.name) | any(. == "e2e_disabled")' | tr -d '\r')
[ "$HAS_DISABLED" = "false" ] || { echo "✗ e2e_disabled 出现在 /api/configs/npc_templates 中（disable fan-out 破损）"; exit 1; }
echo "[✓] disable fan-out: e2e_disabled 已被过滤"

# ──────────────────────────────────────────────
# 收尾
# ──────────────────────────────────────────────

echo
echo "=== seed 完成：5 NPC + 2 region 就绪，导出端点自检 PASS ==="
echo "下一步：Server 侧冷启动，然后 bash scripts/e2e/verify.sh first-round"
