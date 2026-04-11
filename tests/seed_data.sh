#!/usr/bin/env bash
# 批量生成联调数据：覆盖所有 6 个 category + 所有字段类型 + 若干模板
# 依赖：curl + jq
# 用法：bash tests/seed_data.sh [API_BASE]
set -euo pipefail

API="${1:-http://localhost:9821/api/v1}"

post() {
  curl -sS -X POST "$API$1" -H "Content-Type: application/json" -d "$2"
}

create_field() {
  local name="$1" label="$2" type="$3" category="$4" props="$5"
  local body
  body=$(post "/fields/create" "{\"name\":\"$name\",\"label\":\"$label\",\"type\":\"$type\",\"category\":\"$category\",\"properties\":$props}")
  local code id
  code=$(echo "$body" | jq -r '.code')
  if [[ "$code" == "0" ]]; then
    id=$(echo "$body" | jq -r '.data.id')
    echo "$id"
  elif [[ "$code" == "40001" ]]; then
    # 已存在，查 id
    post "/fields/list" "{\"label\":\"$label\",\"page\":1,\"page_size\":50}" \
      | jq -r ".data.items[] | select(.name==\"$name\") | .id" | head -n1
  else
    echo "  [WARN] create field $name failed: $body" >&2
    return 1
  fi
}

enable_field() {
  local id="$1"
  local ver
  ver=$(post "/fields/detail" "{\"id\":$id}" | jq -r '.data.version')
  post "/fields/toggle-enabled" "{\"id\":$id,\"enabled\":true,\"version\":$ver}" > /dev/null
}

create_template() {
  local name="$1" label="$2" desc="$3" fields_json="$4"
  local body code
  body=$(post "/templates/create" "{\"name\":\"$name\",\"label\":\"$label\",\"description\":\"$desc\",\"fields\":$fields_json}")
  code=$(echo "$body" | jq -r '.code')
  if [[ "$code" == "0" ]]; then
    echo "  [OK] template $name created id=$(echo "$body" | jq -r '.data.id')"
  elif [[ "$code" == "41001" ]]; then
    echo "  [SKIP] template $name already exists"
  else
    echo "  [WARN] create template $name failed: $body" >&2
  fi
}

echo ">> seeding fields..."

# ============================================================
# 基础属性 basic
# ============================================================
ID_LEVEL=$(create_field "level" "等级" "integer" "basic" \
  '{"description":"NPC 等级","expose_bb":true,"default_value":1,"constraints":{"min":1,"max":100,"step":1}}')
ID_NPC_NAME=$(create_field "npc_name" "NPC 名称" "string" "basic" \
  '{"description":"显示用名称","expose_bb":false,"default_value":"无名氏","constraints":{"minLength":1,"maxLength":32}}')
ID_FACTION=$(create_field "faction" "阵营" "select" "basic" \
  '{"description":"所属阵营","expose_bb":true,"default_value":"neutral","constraints":{"minSelect":1,"maxSelect":1,"options":[{"value":"ally","label":"友方"},{"value":"neutral","label":"中立"},{"value":"enemy","label":"敌对"}]}}')
ID_UNIQUE=$(create_field "is_unique" "是否唯一" "boolean" "basic" \
  '{"description":"唯一 NPC 标识","expose_bb":false,"default_value":false}')

# ============================================================
# 战斗属性 combat （health 已存在）
# ============================================================
ID_ATTACK=$(create_field "attack" "攻击力" "integer" "combat" \
  '{"description":"物理攻击","expose_bb":true,"default_value":10,"constraints":{"min":0,"max":9999,"step":1}}')
ID_DEFENSE=$(create_field "defense" "防御力" "integer" "combat" \
  '{"description":"物理防御","expose_bb":true,"default_value":5,"constraints":{"min":0,"max":9999,"step":1}}')
ID_ATTACK_RANGE=$(create_field "attack_range" "攻击范围" "float" "combat" \
  '{"description":"攻击距离（米）","expose_bb":true,"default_value":1.5,"constraints":{"min":0.5,"max":50,"precision":2}}')
ID_DAMAGE_TYPE=$(create_field "damage_type" "伤害类型" "select" "combat" \
  '{"description":"主要伤害类型","expose_bb":false,"default_value":"physical","constraints":{"minSelect":1,"maxSelect":2,"options":[{"value":"physical","label":"物理"},{"value":"magic","label":"法术"},{"value":"fire","label":"火焰"},{"value":"ice","label":"冰霜"}]}}')

# ============================================================
# 感知属性 perception
# ============================================================
ID_SIGHT=$(create_field "sight_range" "视野范围" "float" "perception" \
  '{"description":"视野半径（米）","expose_bb":true,"default_value":10,"constraints":{"min":0,"max":100,"precision":1}}')
ID_HEARING=$(create_field "hearing_range" "听觉范围" "float" "perception" \
  '{"description":"听觉半径（米）","expose_bb":true,"default_value":15,"constraints":{"min":0,"max":100,"precision":1}}')
ID_ALERT=$(create_field "alert_level" "警戒等级" "select" "perception" \
  '{"description":"默认警戒状态","expose_bb":true,"default_value":"normal","constraints":{"minSelect":1,"maxSelect":1,"options":[{"value":"calm","label":"平静"},{"value":"normal","label":"正常"},{"value":"alert","label":"警戒"},{"value":"combat","label":"战斗"}]}}')

# ============================================================
# 移动属性 movement
# ============================================================
ID_MOVE_SPEED=$(create_field "move_speed" "移动速度" "float" "movement" \
  '{"description":"基础移动速度","expose_bb":true,"default_value":3.5,"constraints":{"min":0,"max":20,"precision":2}}')
ID_PATROL_RADIUS=$(create_field "patrol_radius" "巡逻半径" "float" "movement" \
  '{"description":"巡逻范围","expose_bb":false,"default_value":8,"constraints":{"min":0,"max":200,"precision":1}}')
ID_CAN_FLY=$(create_field "can_fly" "能否飞行" "boolean" "movement" \
  '{"description":"是否具备飞行能力","expose_bb":true,"default_value":false}')

# ============================================================
# 交互属性 interaction
# ============================================================
ID_DIALOGUE=$(create_field "dialogue_id" "对话 ID" "string" "interaction" \
  '{"description":"关联对话表 ID","expose_bb":false,"default_value":"","constraints":{"minLength":0,"maxLength":64,"pattern":"^[a-z0-9_]*$"}}')
ID_SHOP=$(create_field "shop_id" "商店 ID" "string" "interaction" \
  '{"description":"关联商店配置","expose_bb":false,"default_value":"","constraints":{"minLength":0,"maxLength":64}}')
ID_CAN_TRADE=$(create_field "can_trade" "可交易" "boolean" "interaction" \
  '{"description":"是否可交易","expose_bb":true,"default_value":false}')

# ============================================================
# 个性属性 personality
# ============================================================
ID_AGGRESSION=$(create_field "aggression" "攻击倾向" "integer" "personality" \
  '{"description":"0-100 攻击倾向","expose_bb":true,"default_value":50,"constraints":{"min":0,"max":100,"step":1}}')
ID_FRIENDLINESS=$(create_field "friendliness" "友善度" "integer" "personality" \
  '{"description":"0-100 友善度","expose_bb":true,"default_value":50,"constraints":{"min":0,"max":100,"step":1}}')
ID_GREED=$(create_field "greed" "贪婪度" "float" "personality" \
  '{"description":"0-1 贪婪度","expose_bb":false,"default_value":0.5,"constraints":{"min":0,"max":1,"precision":2}}')

echo ">> enabling all new fields..."
for ID in $ID_LEVEL $ID_NPC_NAME $ID_FACTION $ID_UNIQUE \
          $ID_ATTACK $ID_DEFENSE $ID_ATTACK_RANGE $ID_DAMAGE_TYPE \
          $ID_SIGHT $ID_HEARING $ID_ALERT \
          $ID_MOVE_SPEED $ID_PATROL_RADIUS $ID_CAN_FLY \
          $ID_DIALOGUE $ID_SHOP $ID_CAN_TRADE \
          $ID_AGGRESSION $ID_FRIENDLINESS $ID_GREED; do
  enable_field "$ID" || true
done

# ============================================================
# reference 字段 — 把基础块打包成"快捷选择器"
# ============================================================
echo ">> seeding reference fields..."
ID_REF_BASIC=$(create_field "npc_basic_ref" "基础信息包" "reference" "basic" \
  "{\"description\":\"一次性选入 NPC 基础四件套\",\"expose_bb\":false,\"constraints\":{\"refs\":[$ID_LEVEL,$ID_NPC_NAME,$ID_FACTION,$ID_UNIQUE]}}")
enable_field "$ID_REF_BASIC" || true

ID_REF_COMBAT=$(create_field "npc_combat_ref" "战斗套件" "reference" "combat" \
  "{\"description\":\"攻击/防御/攻击范围/伤害类型\",\"expose_bb\":false,\"constraints\":{\"refs\":[$ID_ATTACK,$ID_DEFENSE,$ID_ATTACK_RANGE,$ID_DAMAGE_TYPE]}}")
enable_field "$ID_REF_COMBAT" || true

ID_REF_PERCEPTION=$(create_field "npc_perception_ref" "感知套件" "reference" "perception" \
  "{\"description\":\"视野/听觉/警戒\",\"expose_bb\":false,\"constraints\":{\"refs\":[$ID_SIGHT,$ID_HEARING,$ID_ALERT]}}")
enable_field "$ID_REF_PERCEPTION" || true

echo ">> seeding templates..."

# 哥布林巡逻兵 — 基础 + 战斗 + 感知 + 移动
create_template "goblin_patroller" "哥布林巡逻兵" "低级敌对单位，用于野外遭遇战" \
  "[
    {\"field_id\":$ID_LEVEL,\"required\":true},
    {\"field_id\":$ID_NPC_NAME,\"required\":true},
    {\"field_id\":$ID_FACTION,\"required\":true},
    {\"field_id\":16,\"required\":true},
    {\"field_id\":$ID_ATTACK,\"required\":true},
    {\"field_id\":$ID_DEFENSE,\"required\":false},
    {\"field_id\":$ID_SIGHT,\"required\":true},
    {\"field_id\":$ID_ALERT,\"required\":false},
    {\"field_id\":$ID_MOVE_SPEED,\"required\":true},
    {\"field_id\":$ID_PATROL_RADIUS,\"required\":true}
  ]"

# 村庄商人 — 基础 + 交互 + 个性
create_template "village_merchant" "村庄商人" "友善 NPC，提供交易" \
  "[
    {\"field_id\":$ID_LEVEL,\"required\":true},
    {\"field_id\":$ID_NPC_NAME,\"required\":true},
    {\"field_id\":$ID_FACTION,\"required\":true},
    {\"field_id\":$ID_UNIQUE,\"required\":false},
    {\"field_id\":$ID_DIALOGUE,\"required\":true},
    {\"field_id\":$ID_SHOP,\"required\":true},
    {\"field_id\":$ID_CAN_TRADE,\"required\":true},
    {\"field_id\":$ID_FRIENDLINESS,\"required\":false},
    {\"field_id\":$ID_GREED,\"required\":false}
  ]"

# 精英守卫 — 全属性覆盖
create_template "elite_guard" "精英守卫" "高级防守型 NPC，全属性覆盖" \
  "[
    {\"field_id\":$ID_LEVEL,\"required\":true},
    {\"field_id\":$ID_NPC_NAME,\"required\":true},
    {\"field_id\":$ID_FACTION,\"required\":true},
    {\"field_id\":16,\"required\":true},
    {\"field_id\":$ID_ATTACK,\"required\":true},
    {\"field_id\":$ID_DEFENSE,\"required\":true},
    {\"field_id\":$ID_ATTACK_RANGE,\"required\":true},
    {\"field_id\":$ID_DAMAGE_TYPE,\"required\":false},
    {\"field_id\":$ID_SIGHT,\"required\":true},
    {\"field_id\":$ID_HEARING,\"required\":true},
    {\"field_id\":$ID_ALERT,\"required\":true},
    {\"field_id\":$ID_MOVE_SPEED,\"required\":true},
    {\"field_id\":$ID_CAN_FLY,\"required\":false},
    {\"field_id\":$ID_AGGRESSION,\"required\":false},
    {\"field_id\":$ID_FRIENDLINESS,\"required\":false}
  ]"

# 飞龙 Boss — 飞行 + 战斗 + 个性
create_template "dragon_boss" "飞龙 Boss" "飞行型 Boss，强攻击 + 高警戒" \
  "[
    {\"field_id\":$ID_LEVEL,\"required\":true},
    {\"field_id\":$ID_NPC_NAME,\"required\":true},
    {\"field_id\":$ID_FACTION,\"required\":true},
    {\"field_id\":$ID_UNIQUE,\"required\":true},
    {\"field_id\":16,\"required\":true},
    {\"field_id\":$ID_ATTACK,\"required\":true},
    {\"field_id\":$ID_ATTACK_RANGE,\"required\":true},
    {\"field_id\":$ID_DAMAGE_TYPE,\"required\":true},
    {\"field_id\":$ID_SIGHT,\"required\":true},
    {\"field_id\":$ID_MOVE_SPEED,\"required\":true},
    {\"field_id\":$ID_CAN_FLY,\"required\":true},
    {\"field_id\":$ID_AGGRESSION,\"required\":true}
  ]"

# 安静型模板（停用演示用）— 只有基础信息
create_template "silent_statue" "静默雕像" "仅基础字段，用于场景装饰" \
  "[
    {\"field_id\":$ID_LEVEL,\"required\":false},
    {\"field_id\":$ID_NPC_NAME,\"required\":true},
    {\"field_id\":$ID_FACTION,\"required\":false}
  ]"

echo ""
echo "=================================================="
echo "seed 完成，字段统计："
post "/fields/list" '{"page":1,"page_size":100}' | jq -r '.data.total as $t | "total = \($t)"'
echo "模板统计："
post "/templates/list" '{"page":1,"page_size":100}' | jq -r '.data.total as $t | "total = \($t)"'
echo "=================================================="
