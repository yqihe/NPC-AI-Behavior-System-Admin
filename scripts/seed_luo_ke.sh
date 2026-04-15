#!/usr/bin/env bash
# ─────────────────────────────────────────────────────────────────────────────
# 野生宠物 测试数据生成脚本（洛克王国世界题材）
#
# 用途：清空业务数据，写入野生宠物题材测试数据，供 CRUD 功能验证
#
# 使用方式（Git Bash）：
#   bash scripts/seed_luo_ke.sh
#
# 前置条件：
#   - 后端已启动（http://localhost:9821）
#   - MySQL root:root@127.0.0.1:3306/npc_ai_admin 可访问
#   - Redis 127.0.0.1:6379 可访问
#   - 已安装：jq、mysql-client、redis-cli
#
# 编码说明：
#   所有含中文的 JSON 均通过 heredoc 写入临时文件，再 curl --data @file，
#   完全绕开 Windows 命令行参数的编码转换，保证 UTF-8 原样送达。
# ─────────────────────────────────────────────────────────────────────────────
set -euo pipefail

BASE="http://localhost:9821/api/v1"
BODY="/tmp/_seed_body.json"   # 临时请求体文件

# ─── 颜色输出 ─────────────────────────────────────────────────────────────────
red()   { printf '\033[31m%s\033[0m\n' "$*"; }
green() { printf '\033[32m%s\033[0m\n' "$*"; }
blue()  { printf '\033[1;34m%s\033[0m\n' "$*"; }

die() { red "✗ $*"; exit 1; }

# ─── API 调用（从临时文件读 body）────────────────────────────────────────────
# 用法：post <path>  （body 已提前写入 $BODY）
post() {
  curl -sf -X POST "$BASE/$1" \
    -H "Content-Type: application/json" \
    --data "@$BODY"
}

# 检查响应 code==0，返回 .data.id
extract_id() {
  local resp="$1" label="$2" code
  code=$(printf '%s' "$resp" | tr -d '\r' | jq -r '.code')
  [ "$code" = "0" ] || die "$label 失败: $resp"
  printf '%s' "$resp" | tr -d '\r' | jq -r '.data.id'
}

# 检查响应 code==0，只打印
assert_ok() {
  local resp="$1" label="$2" code
  code=$(printf '%s' "$resp" | tr -d '\r' | jq -r '.code')
  [ "$code" = "0" ] || die "$label 失败: $resp"
  green "  ✓ $label"
}

# MySQL 快捷（通过 Docker 容器执行）
my() { docker exec -i npc-admin-mysql mysql -u root -proot -s npc_ai_admin 2>/dev/null; }

# ═══════════════════════════════════════════════════════════════════════════════
# Phase 0 — 环境重置
# ═══════════════════════════════════════════════════════════════════════════════
blue "┌─ Phase 0: 清空业务表 + Redis"

my <<'SQL'
SET FOREIGN_KEY_CHECKS = 0;
TRUNCATE TABLE field_refs;
TRUNCATE TABLE schema_refs;
TRUNCATE TABLE event_type_schema;
TRUNCATE TABLE fields;
TRUNCATE TABLE templates;
TRUNCATE TABLE event_types;
TRUNCATE TABLE fsm_configs;
SET FOREIGN_KEY_CHECKS = 1;
SQL

docker exec npc-admin-redis redis-cli FLUSHALL >/dev/null
green "└─ ✓ 清空完成（字典表 / fsm_state_dicts 保留）"
echo ""

# ═══════════════════════════════════════════════════════════════════════════════
# Phase 1 — 字段（16 个）
#   全部属于：basic / combat / perception / movement / interaction
# ═══════════════════════════════════════════════════════════════════════════════
blue "┌─ Phase 1: 创建字段（16 个）"

# ── basic ──────────────────────────────────────────────────────────────────

cat > "$BODY" <<'EOF'
{"name":"pet_name","label":"宠物名称","type":"string","category":"basic","properties":{"description":"野生宠物的种族名称，如《火焰狐狸》《雷霆鹰》","expose_bb":false,"constraints":{"maxLength":32}}}
EOF
R=$(post "fields/create"); F_PET_NAME=$(extract_id "$R" "pet_name")
green "  ✓ pet_name (id=$F_PET_NAME)"

cat > "$BODY" <<'EOF'
{"name":"level","label":"等级","type":"integer","category":"basic","properties":{"description":"野生宠物出现时的等级，影响各项属性强度","expose_bb":true,"constraints":{"min":1,"max":100,"step":1}}}
EOF
R=$(post "fields/create"); F_LEVEL=$(extract_id "$R" "level")
green "  ✓ level (id=$F_LEVEL)"

cat > "$BODY" <<'EOF'
{"name":"hp","label":"生命值","type":"integer","category":"basic","properties":{"description":"宠物最大生命值，降至 0 则进入濒死状态","expose_bb":true,"constraints":{"min":1,"max":9999,"step":1}}}
EOF
R=$(post "fields/create"); F_HP=$(extract_id "$R" "hp")
green "  ✓ hp (id=$F_HP)"

cat > "$BODY" <<'EOF'
{"name":"element","label":"元素属性","type":"select","category":"basic","properties":{"description":"宠物的元素克制关系，决定技能加成","expose_bb":false,"constraints":{"options":[{"value":"fire","label":"火"},{"value":"water","label":"水"},{"value":"grass","label":"草"},{"value":"electric","label":"电"},{"value":"ice","label":"冰"},{"value":"dark","label":"暗"}],"minSelect":1,"maxSelect":1}}}
EOF
R=$(post "fields/create"); F_ELEMENT=$(extract_id "$R" "element")
green "  ✓ element (id=$F_ELEMENT)"

cat > "$BODY" <<'EOF'
{"name":"rarity","label":"稀有度","type":"select","category":"basic","properties":{"description":"宠物的稀有程度，影响出现概率和属性成长","expose_bb":false,"constraints":{"options":[{"value":"common","label":"普通"},{"value":"rare","label":"稀有"},{"value":"epic","label":"史诗"},{"value":"legend","label":"传说"}],"minSelect":1,"maxSelect":1}}}
EOF
R=$(post "fields/create"); F_RARITY=$(extract_id "$R" "rarity")
green "  ✓ rarity (id=$F_RARITY)"

# ── combat ─────────────────────────────────────────────────────────────────

cat > "$BODY" <<'EOF'
{"name":"atk","label":"攻击力","type":"integer","category":"combat","properties":{"description":"物理攻击基础值","expose_bb":true,"constraints":{"min":1,"max":999,"step":1}}}
EOF
R=$(post "fields/create"); F_ATK=$(extract_id "$R" "atk")
green "  ✓ atk (id=$F_ATK)"

cat > "$BODY" <<'EOF'
{"name":"def","label":"防御力","type":"integer","category":"combat","properties":{"description":"物理防御基础值","expose_bb":true,"constraints":{"min":1,"max":999,"step":1}}}
EOF
R=$(post "fields/create"); F_DEF=$(extract_id "$R" "def")
green "  ✓ def (id=$F_DEF)"

cat > "$BODY" <<'EOF'
{"name":"sp_atk","label":"特殊攻击","type":"integer","category":"combat","properties":{"description":"特殊技能攻击力，影响元素技能伤害","expose_bb":true,"constraints":{"min":1,"max":999,"step":1}}}
EOF
R=$(post "fields/create"); F_SP_ATK=$(extract_id "$R" "sp_atk")
green "  ✓ sp_atk (id=$F_SP_ATK)"

cat > "$BODY" <<'EOF'
{"name":"speed","label":"速度","type":"integer","category":"combat","properties":{"description":"决定宠物在战斗中的行动顺序","expose_bb":true,"constraints":{"min":1,"max":999,"step":1}}}
EOF
R=$(post "fields/create"); F_SPEED=$(extract_id "$R" "speed")
green "  ✓ speed (id=$F_SPEED)"

cat > "$BODY" <<'EOF'
{"name":"crit_rate","label":"暴击率","type":"float","category":"combat","properties":{"description":"造成双倍伤害的概率（0.0~1.0）","expose_bb":true,"constraints":{"min":0,"max":1,"precision":2}}}
EOF
R=$(post "fields/create"); F_CRIT=$(extract_id "$R" "crit_rate")
green "  ✓ crit_rate (id=$F_CRIT)"

# ── perception ─────────────────────────────────────────────────────────────

cat > "$BODY" <<'EOF'
{"name":"sight_range","label":"视野范围","type":"float","category":"perception","properties":{"description":"宠物能感知玩家的最大距离（格），超出则无感知","expose_bb":true,"constraints":{"min":1,"max":30,"precision":1}}}
EOF
R=$(post "fields/create"); F_SIGHT=$(extract_id "$R" "sight_range")
green "  ✓ sight_range (id=$F_SIGHT)"

cat > "$BODY" <<'EOF'
{"name":"is_nocturnal","label":"夜行性","type":"boolean","category":"perception","properties":{"description":"是否仅在夜晚活跃，白天处于休眠状态","expose_bb":false,"constraints":{}}}
EOF
R=$(post "fields/create"); F_NOCTURNAL=$(extract_id "$R" "is_nocturnal")
green "  ✓ is_nocturnal (id=$F_NOCTURNAL)"

# ── movement ───────────────────────────────────────────────────────────────

cat > "$BODY" <<'EOF'
{"name":"move_speed","label":"移动速度","type":"float","category":"movement","properties":{"description":"宠物在地图上的移动速度（格/秒）","expose_bb":true,"constraints":{"min":0.5,"max":8,"precision":1}}}
EOF
R=$(post "fields/create"); F_MOVE=$(extract_id "$R" "move_speed")
green "  ✓ move_speed (id=$F_MOVE)"

cat > "$BODY" <<'EOF'
{"name":"can_fly","label":"可飞行","type":"boolean","category":"movement","properties":{"description":"是否可以飞越障碍物，影响寻路和逃跑路径","expose_bb":false,"constraints":{}}}
EOF
R=$(post "fields/create"); F_FLY=$(extract_id "$R" "can_fly")
green "  ✓ can_fly (id=$F_FLY)"

# ── interaction ────────────────────────────────────────────────────────────

cat > "$BODY" <<'EOF'
{"name":"tameable","label":"可驯服","type":"boolean","category":"interaction","properties":{"description":"是否允许玩家使用道具尝试驯服","expose_bb":false,"constraints":{}}}
EOF
R=$(post "fields/create"); F_TAMEABLE=$(extract_id "$R" "tameable")
green "  ✓ tameable (id=$F_TAMEABLE)"

cat > "$BODY" <<'EOF'
{"name":"hostility","label":"敌意程度","type":"select","category":"interaction","properties":{"description":"宠物对玩家的默认行为倾向","expose_bb":true,"constraints":{"options":[{"value":"passive","label":"温顺"},{"value":"neutral","label":"中立"},{"value":"aggressive","label":"凶猛"},{"value":"territorial","label":"领地型"}],"minSelect":1,"maxSelect":1}}}
EOF
R=$(post "fields/create"); F_HOSTILITY=$(extract_id "$R" "hostility")
green "  ✓ hostility (id=$F_HOSTILITY)"

green "└─ ✓ 16 个字段创建完成"
echo ""

# ─── Phase 1.5: 批量启用所有字段（模板引用需字段处于启用状态）──────────────────
blue "┌─ Phase 1.5: 启用所有字段"
for FID in $F_PET_NAME $F_LEVEL $F_HP $F_ELEMENT $F_RARITY \
           $F_ATK $F_DEF $F_SP_ATK $F_SPEED $F_CRIT \
           $F_SIGHT $F_NOCTURNAL $F_MOVE $F_FLY $F_TAMEABLE $F_HOSTILITY; do
  cat > "$BODY" <<EOF
{"id":$FID,"enabled":true,"version":1}
EOF
  R=$(post "fields/toggle-enabled")
  code=$(printf '%s' "$R" | tr -d '\r' | jq -r '.code')
  [ "$code" = "0" ] || die "启用字段 id=$FID 失败: $R"
done
green "└─ ✓ 16 个字段已全部启用"
echo ""

# ═══════════════════════════════════════════════════════════════════════════════
# Phase 2 — 模板（4 个）
# ═══════════════════════════════════════════════════════════════════════════════
blue "┌─ Phase 2: 创建模板（4 个）"

# 野生走兽
cat > "$BODY" <<EOF
{
  "name": "wild_beast",
  "label": "野生走兽",
  "description": "在草地、森林等陆地区域游荡的野生宠物模板，侧重物理攻防。",
  "fields": [
    {"field_id": $F_LEVEL,    "required": true},
    {"field_id": $F_HP,       "required": true},
    {"field_id": $F_ELEMENT,  "required": true},
    {"field_id": $F_RARITY,   "required": true},
    {"field_id": $F_ATK,      "required": true},
    {"field_id": $F_DEF,      "required": true},
    {"field_id": $F_SPEED,    "required": true},
    {"field_id": $F_SIGHT,    "required": true},
    {"field_id": $F_MOVE,     "required": true},
    {"field_id": $F_HOSTILITY,"required": true},
    {"field_id": $F_TAMEABLE, "required": true}
  ]
}
EOF
R=$(post "templates/create"); T_BEAST=$(extract_id "$R" "wild_beast")
green "  ✓ 野生走兽 wild_beast (id=$T_BEAST)"

# 野生飞禽
cat > "$BODY" <<EOF
{
  "name": "wild_bird",
  "label": "野生飞禽",
  "description": "活跃于天空的飞行类宠物，速度快、视野广，夜行性个体在暗处攻击力提升。",
  "fields": [
    {"field_id": $F_LEVEL,     "required": true},
    {"field_id": $F_HP,        "required": true},
    {"field_id": $F_ELEMENT,   "required": true},
    {"field_id": $F_RARITY,    "required": true},
    {"field_id": $F_ATK,       "required": true},
    {"field_id": $F_SPEED,     "required": true},
    {"field_id": $F_CRIT,      "required": false},
    {"field_id": $F_SIGHT,     "required": true},
    {"field_id": $F_NOCTURNAL,  "required": false},
    {"field_id": $F_FLY,       "required": true},
    {"field_id": $F_MOVE,      "required": true},
    {"field_id": $F_HOSTILITY, "required": true},
    {"field_id": $F_TAMEABLE,  "required": true}
  ]
}
EOF
R=$(post "templates/create"); T_BIRD=$(extract_id "$R" "wild_bird")
green "  ✓ 野生飞禽 wild_bird (id=$T_BIRD)"

# 水栖宠物
cat > "$BODY" <<EOF
{
  "name": "aquatic_pet",
  "label": "水栖宠物",
  "description": "生活在河流、湖泊、海洋中的宠物，特殊攻击强，对水系技能有天然抗性。",
  "fields": [
    {"field_id": $F_LEVEL,    "required": true},
    {"field_id": $F_HP,       "required": true},
    {"field_id": $F_ELEMENT,  "required": true},
    {"field_id": $F_RARITY,   "required": true},
    {"field_id": $F_ATK,      "required": true},
    {"field_id": $F_DEF,      "required": true},
    {"field_id": $F_SP_ATK,   "required": true},
    {"field_id": $F_SPEED,    "required": true},
    {"field_id": $F_SIGHT,    "required": true},
    {"field_id": $F_MOVE,     "required": true},
    {"field_id": $F_HOSTILITY,"required": true},
    {"field_id": $F_TAMEABLE, "required": true}
  ]
}
EOF
R=$(post "templates/create"); T_AQUA=$(extract_id "$R" "aquatic_pet")
green "  ✓ 水栖宠物 aquatic_pet (id=$T_AQUA)"

# 稀有头领宠物（全属性）
cat > "$BODY" <<EOF
{
  "name": "rare_boss_pet",
  "label": "稀有头领",
  "description": "极低概率出现的区域头领，集齐所有稀有属性，驯服后可作为强力战斗伙伴。",
  "fields": [
    {"field_id": $F_PET_NAME, "required": true},
    {"field_id": $F_LEVEL,    "required": true},
    {"field_id": $F_HP,       "required": true},
    {"field_id": $F_ELEMENT,  "required": true},
    {"field_id": $F_RARITY,   "required": true},
    {"field_id": $F_ATK,      "required": true},
    {"field_id": $F_DEF,      "required": true},
    {"field_id": $F_SP_ATK,   "required": true},
    {"field_id": $F_SPEED,    "required": true},
    {"field_id": $F_CRIT,     "required": true},
    {"field_id": $F_SIGHT,    "required": true},
    {"field_id": $F_MOVE,     "required": true},
    {"field_id": $F_FLY,      "required": false},
    {"field_id": $F_HOSTILITY,"required": true},
    {"field_id": $F_TAMEABLE, "required": true}
  ]
}
EOF
R=$(post "templates/create"); T_BOSS=$(extract_id "$R" "rare_boss_pet")
green "  ✓ 稀有头领 rare_boss_pet (id=$T_BOSS)"

green "└─ ✓ 4 个模板创建完成"
echo ""

# ═══════════════════════════════════════════════════════════════════════════════
# Phase 3 — 事件类型扩展字段 Schema（2 个）
# ═══════════════════════════════════════════════════════════════════════════════
blue "┌─ Phase 3: 创建扩展字段 Schema（2 个）"

cat > "$BODY" <<'EOF'
{"field_name":"encounter_xp","field_label":"遭遇经验值","field_type":"int","constraints":{"min":0,"max":9999},"default_value":10,"sort_order":1}
EOF
R=$(post "event-type-schema/create"); assert_ok "$R" "扩展字段 encounter_xp（遭遇经验值）"

cat > "$BODY" <<'EOF'
{"field_name":"escape_rate","field_label":"逃跑概率","field_type":"float","constraints":{"min":0,"max":1},"default_value":0.3,"sort_order":2}
EOF
R=$(post "event-type-schema/create"); assert_ok "$R" "扩展字段 escape_rate（逃跑概率）"

green "└─ ✓ 2 个扩展字段创建完成"
echo ""

# ═══════════════════════════════════════════════════════════════════════════════
# Phase 4 — 事件类型（6 个）
# ═══════════════════════════════════════════════════════════════════════════════
blue "┌─ Phase 4: 创建事件类型（6 个）"

cat > "$BODY" <<'EOF'
{"name":"wild_pet_appear","display_name":"野生宠物现身","perception_mode":"visual","default_severity":0.4,"default_ttl":5,"range":12,"extensions":{"encounter_xp":10,"escape_rate":0.5}}
EOF
R=$(post "event-types/create"); assert_ok "$R" "野生宠物现身 wild_pet_appear"

cat > "$BODY" <<'EOF'
{"name":"pet_attack_player","display_name":"宠物袭击玩家","perception_mode":"visual","default_severity":0.7,"default_ttl":20,"range":8,"extensions":{"encounter_xp":30,"escape_rate":0.1}}
EOF
R=$(post "event-types/create"); assert_ok "$R" "宠物袭击玩家 pet_attack_player"

cat > "$BODY" <<'EOF'
{"name":"pet_escape","display_name":"宠物成功逃跑","perception_mode":"visual","default_severity":0.2,"default_ttl":5,"range":5,"extensions":{"encounter_xp":5,"escape_rate":1.0}}
EOF
R=$(post "event-types/create"); assert_ok "$R" "宠物成功逃跑 pet_escape"

cat > "$BODY" <<'EOF'
{"name":"rare_pet_spawn","display_name":"稀有宠物降临","perception_mode":"global","default_severity":0.9,"default_ttl":60,"range":0,"extensions":{"encounter_xp":200,"escape_rate":0.8}}
EOF
R=$(post "event-types/create"); assert_ok "$R" "稀有宠物降临 rare_pet_spawn"

cat > "$BODY" <<'EOF'
{"name":"pack_call","display_name":"群体召唤","perception_mode":"auditory","default_severity":0.5,"default_ttl":15,"range":20,"extensions":{"encounter_xp":20,"escape_rate":0.3}}
EOF
R=$(post "event-types/create"); assert_ok "$R" "群体召唤 pack_call"

cat > "$BODY" <<'EOF'
{"name":"pet_captured","display_name":"宠物被捕获","perception_mode":"global","default_severity":0.3,"default_ttl":5,"range":0,"extensions":{"encounter_xp":50,"escape_rate":0.0}}
EOF
R=$(post "event-types/create"); assert_ok "$R" "宠物被捕获 pet_captured"

green "└─ ✓ 6 个事件类型创建完成"
echo ""

# ═══════════════════════════════════════════════════════════════════════════════
# Phase 5 — FSM 配置（3 个）
# ═══════════════════════════════════════════════════════════════════════════════
blue "┌─ Phase 5: 创建 FSM 状态机配置（3 个）"

# ── 温顺宠物（只会逃跑）────────────────────────────────────────────────────────
cat > "$BODY" <<'EOF'
{
  "name": "passive_pet_fsm",
  "display_name": "温顺宠物状态机",
  "initial_state": "idle",
  "states": [
    {"name": "idle"},
    {"name": "flee"},
    {"name": "dead"}
  ],
  "transitions": [
    {
      "from": "idle", "to": "flee", "priority": 10,
      "condition": {"key": "threat_level", "op": ">", "value": 0.2}
    },
    {
      "from": "flee", "to": "idle", "priority": 10,
      "condition": {"key": "threat_level", "op": "<", "value": 0.05}
    },
    {
      "from": "idle", "to": "dead", "priority": 30,
      "condition": {"key": "hp", "op": "<=", "value": 0}
    },
    {
      "from": "flee", "to": "dead", "priority": 30,
      "condition": {"key": "hp", "op": "<=", "value": 0}
    }
  ]
}
EOF
R=$(post "fsm-configs/create"); assert_ok "$R" "温顺宠物状态机 passive_pet_fsm"

# ── 凶猛宠物（主动攻击）────────────────────────────────────────────────────────
cat > "$BODY" <<'EOF'
{
  "name": "aggressive_pet_fsm",
  "display_name": "凶猛宠物状态机",
  "initial_state": "wander",
  "states": [
    {"name": "wander"},
    {"name": "alert"},
    {"name": "engage"},
    {"name": "attack_melee"},
    {"name": "flee"},
    {"name": "dead"}
  ],
  "transitions": [
    {
      "from": "wander", "to": "alert", "priority": 10,
      "condition": {"key": "threat_level", "op": ">", "value": 0.1}
    },
    {
      "from": "alert", "to": "engage", "priority": 10,
      "condition": {"and": [
        {"key": "hp_percent", "op": ">=", "value": 0.3},
        {"key": "threat_level", "op": ">=", "value": 0.5}
      ]}
    },
    {
      "from": "alert", "to": "wander", "priority": 5,
      "condition": {"key": "threat_level", "op": "<", "value": 0.1}
    },
    {
      "from": "engage", "to": "attack_melee", "priority": 10,
      "condition": {"key": "dist_to_target", "op": "<=", "value": 1.5}
    },
    {
      "from": "attack_melee", "to": "engage", "priority": 5,
      "condition": {}
    },
    {
      "from": "engage", "to": "flee", "priority": 20,
      "condition": {"key": "hp_percent", "op": "<", "value": 0.2}
    },
    {
      "from": "flee", "to": "wander", "priority": 10,
      "condition": {"key": "threat_level", "op": "<", "value": 0.05}
    },
    {
      "from": "engage",       "to": "dead", "priority": 30,
      "condition": {"key": "hp", "op": "<=", "value": 0}
    },
    {
      "from": "attack_melee", "to": "dead", "priority": 30,
      "condition": {"key": "hp", "op": "<=", "value": 0}
    },
    {
      "from": "flee",         "to": "dead", "priority": 30,
      "condition": {"key": "hp", "op": "<=", "value": 0}
    }
  ]
}
EOF
R=$(post "fsm-configs/create"); assert_ok "$R" "凶猛宠物状态机 aggressive_pet_fsm"

# ── 领地型宠物（巡逻+返回原点）──────────────────────────────────────────────────
cat > "$BODY" <<'EOF'
{
  "name": "territorial_pet_fsm",
  "display_name": "领地型宠物状态机",
  "initial_state": "patrol",
  "states": [
    {"name": "patrol"},
    {"name": "alert"},
    {"name": "engage"},
    {"name": "attack_melee"},
    {"name": "return_home"},
    {"name": "dead"}
  ],
  "transitions": [
    {
      "from": "patrol", "to": "alert", "priority": 10,
      "condition": {"key": "threat_level", "op": ">", "value": 0.3}
    },
    {
      "from": "alert", "to": "engage", "priority": 10,
      "condition": {"key": "threat_level", "op": ">=", "value": 0.6}
    },
    {
      "from": "alert", "to": "patrol", "priority": 5,
      "condition": {"key": "threat_level", "op": "<", "value": 0.1}
    },
    {
      "from": "engage", "to": "attack_melee", "priority": 10,
      "condition": {"key": "dist_to_target", "op": "<=", "value": 1.5}
    },
    {
      "from": "attack_melee", "to": "engage", "priority": 5,
      "condition": {}
    },
    {
      "from": "engage", "to": "return_home", "priority": 15,
      "condition": {"or": [
        {"key": "threat_level", "op": "<", "value": 0.1},
        {"key": "dist_from_home", "op": ">", "value": 15}
      ]}
    },
    {
      "from": "return_home", "to": "patrol", "priority": 10,
      "condition": {"key": "at_home_pos", "op": "==", "value": true}
    },
    {
      "from": "patrol",       "to": "dead", "priority": 30,
      "condition": {"key": "hp", "op": "<=", "value": 0}
    },
    {
      "from": "engage",       "to": "dead", "priority": 30,
      "condition": {"key": "hp", "op": "<=", "value": 0}
    },
    {
      "from": "attack_melee", "to": "dead", "priority": 30,
      "condition": {"key": "hp", "op": "<=", "value": 0}
    }
  ]
}
EOF
R=$(post "fsm-configs/create"); assert_ok "$R" "领地型宠物状态机 territorial_pet_fsm"

green "└─ ✓ 3 个 FSM 配置创建完成"
echo ""

# ═══════════════════════════════════════════════════════════════════════════════
# 汇总
# ═══════════════════════════════════════════════════════════════════════════════
blue "════ 数据生成完成 ════"
echo ""
printf "  %-20s %s\n" "字段"          "16 个  (basic×5 / combat×5 / perception×2 / movement×2 / interaction×2)"
printf "  %-20s %s\n" "模板"          "4 个   (野生走兽 / 野生飞禽 / 水栖宠物 / 稀有头领)"
printf "  %-20s %s\n" "扩展字段"      "2 个   (encounter_xp / escape_rate)"
printf "  %-20s %s\n" "事件类型"      "6 个   (wild_pet_appear / pet_attack_player / ...)"
printf "  %-20s %s\n" "FSM 配置"      "3 个   (passive / aggressive / territorial)"
printf "  %-20s %s\n" "FSM 状态字典"  "31 条  (保留 seed 数据)"
echo ""
green "可以打开管理后台验证 CRUD 了！"

rm -f "$BODY"
