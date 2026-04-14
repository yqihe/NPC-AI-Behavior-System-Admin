#!/bin/bash
# =============================================================================
# test_08_export.sh — 导出 API 跨模块测试
#
# 前置：run_all.sh 已 source helpers.sh，$BASE / $P / assert_* / post() /
#       et_* / fsm_* / get_export() 可用
#
# 覆盖：事件类型导出 + FSM 导出 + 格式验证 + 启用/停用/删除联动
#       + 全局事件 range=0 + 扩展字段导出 + 空导出 + 生命周期
#       + 多次启停循环 + 快速 toggle 攻击 + 跨模块一致性
# =============================================================================

section "Part 8: 导出 API (prefix=$P)"

# ---- 测试数据名称 ----
EXP_ET_NAME1="${P}exp_earthquake"
EXP_ET_NAME2="${P}exp_fire"
EXP_ET_NAME3="${P}exp_disabled_evt"
EXP_FSM_NAME1="${P}exp_wolf_fsm"
EXP_FSM_NAME2="${P}exp_guard_fsm"
EXP_FSM_NAME3="${P}exp_disabled_fsm"

# =============================================================================
subsection "8-A: 事件类型导出 — 启用/未启用过滤"
# =============================================================================

# A1: 创建 2 个启用的事件类型
body=$(post "/event-types/create" "$(printf '%s' '{
  "name":"'"$EXP_ET_NAME1"'",
  "display_name":"Earthquake",
  "perception_mode":"auditory",
  "default_severity":80,
  "default_ttl":10,
  "range":200
}')")
assert_code "A1a 创建事件类型 earthquake" "0" "$body"
EXP_ET_ID1=$(echo "$body" | jq -r '.data.id' | tr -d '\r')
et_enable "$EXP_ET_ID1"

body=$(post "/event-types/create" "$(printf '%s' '{
  "name":"'"$EXP_ET_NAME2"'",
  "display_name":"Fire",
  "perception_mode":"visual",
  "default_severity":60,
  "default_ttl":15,
  "range":150
}')")
assert_code "A1b 创建事件类型 fire" "0" "$body"
EXP_ET_ID2=$(echo "$body" | jq -r '.data.id' | tr -d '\r')
et_enable "$EXP_ET_ID2"

# A2: 创建 1 个未启用的事件类型
body=$(post "/event-types/create" "$(printf '%s' '{
  "name":"'"$EXP_ET_NAME3"'",
  "display_name":"Disabled Event",
  "perception_mode":"global",
  "default_severity":10,
  "default_ttl":5,
  "range":0
}')")
assert_code "A2 创建未启用事件类型" "0" "$body"
EXP_ET_ID3=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# A3: 导出 — 已启用项出现
body=$(get_export "/event_types")
TOTAL=$((TOTAL + 1))
ET_EXP_HIT1=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_ET_NAME1\") | .name" | tr -d '\r')
if [ "$ET_EXP_HIT1" = "$EXP_ET_NAME1" ]; then
  echo "  [PASS] A3a 启用的 earthquake 出现在导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] A3a 启用的 earthquake 未出现在导出"
  FAIL=$((FAIL + 1))
fi

TOTAL=$((TOTAL + 1))
ET_EXP_HIT2=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_ET_NAME2\") | .name" | tr -d '\r')
if [ "$ET_EXP_HIT2" = "$EXP_ET_NAME2" ]; then
  echo "  [PASS] A3b 启用的 fire 出现在导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] A3b 启用的 fire 未出现在导出"
  FAIL=$((FAIL + 1))
fi

# A4: 未启用项 NOT in export
TOTAL=$((TOTAL + 1))
ET_EXP_DISABLED=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_ET_NAME3\") | .name" | tr -d '\r')
if [ -z "$ET_EXP_DISABLED" ]; then
  echo "  [PASS] A4 未启用事件类型不在导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] A4 未启用事件类型出现在导出"
  FAIL=$((FAIL + 1))
fi

# =============================================================================
subsection "8-B: FSM 导出 — 启用/未启用过滤"
# =============================================================================

# B1: 创建 2 个启用的 FSM
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"$EXP_FSM_NAME1"'",
  "display_name":"Export Wolf",
  "initial_state":"idle",
  "states":[{"name":"idle"},{"name":"patrol"},{"name":"chase"}],
  "transitions":[
    {"from":"idle","to":"patrol","priority":1,"condition":{"key":"energy","op":">","value":50}},
    {"from":"patrol","to":"chase","priority":2,"condition":{"key":"enemy_near","op":"==","value":true}}
  ]
}')")
assert_code "B1a 创建 FSM wolf" "0" "$body"
EXP_FSM_ID1=$(echo "$body" | jq -r '.data.id' | tr -d '\r')
fsm_enable "$EXP_FSM_ID1"

body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"$EXP_FSM_NAME2"'",
  "display_name":"Export Guard",
  "initial_state":"stand",
  "states":[{"name":"stand"},{"name":"alert"}],
  "transitions":[{"from":"stand","to":"alert","priority":1,"condition":{"key":"noise","op":">","value":30}}]
}')")
assert_code "B1b 创建 FSM guard" "0" "$body"
EXP_FSM_ID2=$(echo "$body" | jq -r '.data.id' | tr -d '\r')
fsm_enable "$EXP_FSM_ID2"

# B2: 创建 1 个未启用 FSM
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"$EXP_FSM_NAME3"'",
  "display_name":"Disabled FSM",
  "initial_state":"s1",
  "states":[{"name":"s1"}],
  "transitions":[]
}')")
assert_code "B2 创建未启用 FSM" "0" "$body"
EXP_FSM_ID3=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# B3: 导出 — 已启用项出现
body=$(get_export "/fsm_configs")
TOTAL=$((TOTAL + 1))
FSM_EXP_HIT1=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_FSM_NAME1\") | .name" | tr -d '\r')
if [ "$FSM_EXP_HIT1" = "$EXP_FSM_NAME1" ]; then
  echo "  [PASS] B3a 启用的 wolf FSM 出现在导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] B3a 启用的 wolf FSM 未出现在导出"
  FAIL=$((FAIL + 1))
fi

TOTAL=$((TOTAL + 1))
FSM_EXP_HIT2=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_FSM_NAME2\") | .name" | tr -d '\r')
if [ "$FSM_EXP_HIT2" = "$EXP_FSM_NAME2" ]; then
  echo "  [PASS] B3b 启用的 guard FSM 出现在导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] B3b 启用的 guard FSM 未出现在导出"
  FAIL=$((FAIL + 1))
fi

# B4: 未启用项 NOT in export
TOTAL=$((TOTAL + 1))
FSM_EXP_DISABLED=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_FSM_NAME3\") | .name" | tr -d '\r')
if [ -z "$FSM_EXP_DISABLED" ]; then
  echo "  [PASS] B4 未启用 FSM 不在导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] B4 未启用 FSM 出现在导出"
  FAIL=$((FAIL + 1))
fi

# =============================================================================
subsection "8-C: 导出格式验证"
# =============================================================================

# C1: items 是 array
body=$(get_export "/event_types")
assert_field "C1a event_types items 是 array" '.items | type' "array" "$body"

body=$(get_export "/fsm_configs")
assert_field "C1b fsm_configs items 是 array" '.items | type' "array" "$body"

# C2: 每个 item 有 name + config(object)
body=$(get_export "/event_types")
TOTAL=$((TOTAL + 1))
ET_FIRST_NAME=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_ET_NAME1\") | .name" | tr -d '\r')
ET_FIRST_CFG=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_ET_NAME1\") | .config | type" | tr -d '\r')
if [ -n "$ET_FIRST_NAME" ] && [ "$ET_FIRST_CFG" = "object" ]; then
  echo "  [PASS] C2a 事件导出格式 {name, config} 正确"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] C2a 事件导出格式异常 name=$ET_FIRST_NAME config_type=$ET_FIRST_CFG"
  FAIL=$((FAIL + 1))
fi

body=$(get_export "/fsm_configs")
TOTAL=$((TOTAL + 1))
FSM_FIRST_NAME=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_FSM_NAME1\") | .name" | tr -d '\r')
FSM_FIRST_CFG=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_FSM_NAME1\") | .config | type" | tr -d '\r')
if [ -n "$FSM_FIRST_NAME" ] && [ "$FSM_FIRST_CFG" = "object" ]; then
  echo "  [PASS] C2b FSM 导出格式 {name, config} 正确"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] C2b FSM 导出格式异常 name=$FSM_FIRST_NAME config_type=$FSM_FIRST_CFG"
  FAIL=$((FAIL + 1))
fi

# =============================================================================
subsection "8-D: 事件类型 config 字段完整性"
# =============================================================================

body=$(get_export "/event_types")
ET_CFG_ITEM=$(echo "$body" | jq ".items[] | select(.name==\"$EXP_ET_NAME1\") | .config")

TOTAL=$((TOTAL + 1))
ET_HAS_DN=$(echo "$ET_CFG_ITEM" | jq -r '.display_name // empty' | tr -d '\r')
ET_HAS_SEV=$(echo "$ET_CFG_ITEM" | jq -r '.default_severity // empty' | tr -d '\r')
ET_HAS_TTL=$(echo "$ET_CFG_ITEM" | jq -r '.default_ttl // empty' | tr -d '\r')
ET_HAS_PM=$(echo "$ET_CFG_ITEM" | jq -r '.perception_mode // empty' | tr -d '\r')
ET_HAS_RNG=$(echo "$ET_CFG_ITEM" | jq -r '.range' | tr -d '\r')
if [ -n "$ET_HAS_DN" ] && [ -n "$ET_HAS_SEV" ] && [ -n "$ET_HAS_TTL" ] && [ -n "$ET_HAS_PM" ] && [ "$ET_HAS_RNG" != "null" ]; then
  echo "  [PASS] D1 event config 含 display_name/severity/ttl/perception_mode/range"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] D1 event config 缺少字段 (dn=$ET_HAS_DN sev=$ET_HAS_SEV ttl=$ET_HAS_TTL pm=$ET_HAS_PM rng=$ET_HAS_RNG)"
  FAIL=$((FAIL + 1))
fi

# D2: 各字段值正确
assert_field "D2a display_name=Earthquake" '.display_name' "Earthquake" "$ET_CFG_ITEM"
assert_field "D2b default_severity=80" '.default_severity' "80" "$ET_CFG_ITEM"
assert_field "D2c perception_mode=auditory" '.perception_mode' "auditory" "$ET_CFG_ITEM"

# =============================================================================
subsection "8-E: FSM config 字段完整性"
# =============================================================================

body=$(get_export "/fsm_configs")
FSM_CFG_ITEM=$(echo "$body" | jq ".items[] | select(.name==\"$EXP_FSM_NAME1\") | .config")

TOTAL=$((TOTAL + 1))
FSM_HAS_INIT=$(echo "$FSM_CFG_ITEM" | jq -r '.initial_state // empty' | tr -d '\r')
FSM_HAS_STATES=$(echo "$FSM_CFG_ITEM" | jq -r '.states | length' | tr -d '\r')
FSM_HAS_TRANS_TYPE=$(echo "$FSM_CFG_ITEM" | jq -r '.transitions | type' | tr -d '\r')
if [ -n "$FSM_HAS_INIT" ] && [ "$FSM_HAS_STATES" -gt 0 ] 2>/dev/null && [ "$FSM_HAS_TRANS_TYPE" = "array" ]; then
  echo "  [PASS] E1 FSM config 含 initial_state/states/transitions"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] E1 FSM config 缺少字段 (init=$FSM_HAS_INIT states=$FSM_HAS_STATES trans_type=$FSM_HAS_TRANS_TYPE)"
  FAIL=$((FAIL + 1))
fi

# E2: initial_state 值匹配
TOTAL=$((TOTAL + 1))
if [ "$FSM_HAS_INIT" = "idle" ]; then
  echo "  [PASS] E2 FSM config.initial_state=idle"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] E2 FSM config.initial_state 期望 idle, 实际 $FSM_HAS_INIT"
  FAIL=$((FAIL + 1))
fi

# E3: states 数量
TOTAL=$((TOTAL + 1))
if [ "$FSM_HAS_STATES" = "3" ]; then
  echo "  [PASS] E3 FSM config.states 数量=3"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] E3 FSM config.states 期望 3, 实际 $FSM_HAS_STATES"
  FAIL=$((FAIL + 1))
fi

# E4: transitions 是 array
assert_field "E4 transitions type=array" '.transitions | type' "array" "$FSM_CFG_ITEM"

# =============================================================================
subsection "8-F: Disable -> NOT in export, Re-enable -> back in export"
# =============================================================================

# F1: 停用 earthquake -> 不在导出
et_disable "$EXP_ET_ID1"
body=$(get_export "/event_types")
TOTAL=$((TOTAL + 1))
ET_AFTER_DISABLE=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_ET_NAME1\") | .name" | tr -d '\r')
if [ -z "$ET_AFTER_DISABLE" ]; then
  echo "  [PASS] F1 停用后 earthquake 不在导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] F1 停用后 earthquake 仍在导出"
  FAIL=$((FAIL + 1))
fi

# F2: 重新启用 -> 回到导出
et_enable "$EXP_ET_ID1"
body=$(get_export "/event_types")
TOTAL=$((TOTAL + 1))
ET_REENABLE=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_ET_NAME1\") | .name" | tr -d '\r')
if [ "$ET_REENABLE" = "$EXP_ET_NAME1" ]; then
  echo "  [PASS] F2 重新启用后 earthquake 回到导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] F2 重新启用后 earthquake 未回到导出"
  FAIL=$((FAIL + 1))
fi

# =============================================================================
subsection "8-G: Delete -> NOT in export"
# =============================================================================

et_disable "$EXP_ET_ID1"
post "/event-types/delete" "{\"id\":$EXP_ET_ID1}" > /dev/null
body=$(get_export "/event_types")
TOTAL=$((TOTAL + 1))
ET_AFTER_DEL=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_ET_NAME1\") | .name" | tr -d '\r')
if [ -z "$ET_AFTER_DEL" ]; then
  echo "  [PASS] G1 删除后 earthquake 不在导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] G1 删除后 earthquake 仍在导出"
  FAIL=$((FAIL + 1))
fi

# FSM 删除同理
fsm_disable "$EXP_FSM_ID1"
post "/fsm-configs/delete" "{\"id\":$EXP_FSM_ID1}" > /dev/null
body=$(get_export "/fsm_configs")
TOTAL=$((TOTAL + 1))
FSM_AFTER_DEL=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_FSM_NAME1\") | .name" | tr -d '\r')
if [ -z "$FSM_AFTER_DEL" ]; then
  echo "  [PASS] G2 删除后 wolf FSM 不在导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] G2 删除后 wolf FSM 仍在导出"
  FAIL=$((FAIL + 1))
fi

# =============================================================================
subsection "8-H: 全生命周期 — create -> not in export -> enable -> in -> disable -> not in"
# =============================================================================

EXP_CYCLE_NAME="${P}exp_cycle_fsm"
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"$EXP_CYCLE_NAME"'",
  "display_name":"Cycle Test",
  "initial_state":"a",
  "states":[{"name":"a"},{"name":"b"}],
  "transitions":[{"from":"a","to":"b","priority":0,"condition":{}}]
}')")
assert_code "H1a 创建 cycle FSM" "0" "$body"
EXP_CYCLE_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# H1b: Not in export before enable
body=$(get_export "/fsm_configs")
TOTAL=$((TOTAL + 1))
CYCLE_PRE=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_CYCLE_NAME\") | .name" | tr -d '\r')
if [ -z "$CYCLE_PRE" ]; then
  echo "  [PASS] H1b 创建后(未启用)不在导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] H1b 创建后(未启用)已在导出"
  FAIL=$((FAIL + 1))
fi

# H1c: Enable -> in export
fsm_enable "$EXP_CYCLE_ID"
body=$(get_export "/fsm_configs")
TOTAL=$((TOTAL + 1))
CYCLE_EN=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_CYCLE_NAME\") | .name" | tr -d '\r')
if [ "$CYCLE_EN" = "$EXP_CYCLE_NAME" ]; then
  echo "  [PASS] H1c 启用后出现在导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] H1c 启用后未出现在导出"
  FAIL=$((FAIL + 1))
fi

# H1d: Disable -> not in export
fsm_disable "$EXP_CYCLE_ID"
body=$(get_export "/fsm_configs")
TOTAL=$((TOTAL + 1))
CYCLE_DIS=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_CYCLE_NAME\") | .name" | tr -d '\r')
if [ -z "$CYCLE_DIS" ]; then
  echo "  [PASS] H1d 停用后不在导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] H1d 停用后仍在导出"
  FAIL=$((FAIL + 1))
fi

# =============================================================================
subsection "8-I: Global 事件导出 — range=0"
# =============================================================================

EXP_GLOBAL_NAME="${P}exp_global_evt"
body=$(post "/event-types/create" "$(printf '%s' '{
  "name":"'"$EXP_GLOBAL_NAME"'",
  "display_name":"Global Export",
  "perception_mode":"global",
  "default_severity":90,
  "default_ttl":20,
  "range":999
}')")
assert_code "I1 创建 global 事件(range=999,应修正为0)" "0" "$body"
EXP_GLOBAL_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')
et_enable "$EXP_GLOBAL_ID"

body=$(get_export "/event_types")
TOTAL=$((TOTAL + 1))
GLOBAL_RANGE=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_GLOBAL_NAME\") | .config.range" | tr -d '\r')
if [ "$GLOBAL_RANGE" = "0" ]; then
  echo "  [PASS] I2 global 导出 range=0"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] I2 global 导出 range 期望 0, 实际 $GLOBAL_RANGE"
  FAIL=$((FAIL + 1))
fi

# =============================================================================
subsection "8-J: 扩展字段值在导出 config 中"
# =============================================================================

# 先创建扩展字段 schema（如果之前 test_05 已经创建了也无妨，用新名字）
EXP_SCHEMA_NAME="${P}exp_ext_priority"
body=$(post "/event-type-schema/create" "$(printf '%s' '{
  "field_name":"'"$EXP_SCHEMA_NAME"'",
  "field_label":"Export Priority",
  "field_type":"integer",
  "constraints":{"min":0,"max":10},
  "default_value":5
}')")
EXP_SCHEMA_OK=$(echo "$body" | jq -r '.code' | tr -d '\r')
if [ "$EXP_SCHEMA_OK" = "0" ]; then
  EXP_SCHEMA_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')
  # 启用 schema
  schema_enable "$EXP_SCHEMA_ID"

  # 创建带扩展的事件
  EXP_EXT_EVT_NAME="${P}exp_ext_evt"
  body=$(post "/event-types/create" "$(printf '%s' '{
    "name":"'"$EXP_EXT_EVT_NAME"'",
    "display_name":"Ext Export Event",
    "perception_mode":"visual",
    "default_severity":50,
    "default_ttl":10,
    "range":100,
    "extensions":{"'"$EXP_SCHEMA_NAME"'":8}
  }')")
  assert_code "J1 创建带扩展字段的事件" "0" "$body"
  EXP_EXT_EVT_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')
  et_enable "$EXP_EXT_EVT_ID"

  # 导出并验证扩展字段值
  body=$(get_export "/event_types")
  TOTAL=$((TOTAL + 1))
  EXT_VAL=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_EXT_EVT_NAME\") | .config.${EXP_SCHEMA_NAME}" | tr -d '\r')
  if [ "$EXT_VAL" = "8" ]; then
    echo "  [PASS] J2 扩展字段值 ${EXP_SCHEMA_NAME}=8 出现在导出 config"
    PASS=$((PASS + 1))
  else
    echo "  [FAIL] J2 扩展字段值 期望 8, 实际 $EXT_VAL"
    FAIL=$((FAIL + 1))
  fi

  et_rm "$EXP_EXT_EVT_ID" 2>/dev/null
  schema_rm "$EXP_SCHEMA_ID" 2>/dev/null
else
  echo "  [SKIP] J 扩展 schema 创建失败(code=$EXP_SCHEMA_OK)，跳过扩展导出测试"
fi

# =============================================================================
subsection "8-K: 空导出 — items 始终是 array"
# =============================================================================

# 无论数据多少，items 都应是 array（从不为 null）
body=$(get_export "/event_types")
assert_field "K1 event_types items 是 array" '.items | type' "array" "$body"

body=$(get_export "/fsm_configs")
assert_field "K2 fsm_configs items 是 array" '.items | type' "array" "$body"

# =============================================================================
subsection "8-L: 多次启停循环"
# =============================================================================

EXP_TOGGLE_NAME="${P}exp_toggle_evt"
body=$(post "/event-types/create" "$(printf '%s' '{
  "name":"'"$EXP_TOGGLE_NAME"'",
  "display_name":"Toggle Test",
  "perception_mode":"visual",
  "default_severity":50,
  "default_ttl":10,
  "range":100
}')")
assert_code "L0 创建 toggle 测试事件" "0" "$body"
EXP_TOGGLE_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 循环 3 次
for cycle in 1 2 3; do
  et_enable "$EXP_TOGGLE_ID"
  body=$(get_export "/event_types")
  TOTAL=$((TOTAL + 1))
  HIT=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_TOGGLE_NAME\") | .name" | tr -d '\r')
  if [ "$HIT" = "$EXP_TOGGLE_NAME" ]; then
    echo "  [PASS] L${cycle}a 第${cycle}次启用 -> 在导出"
    PASS=$((PASS + 1))
  else
    echo "  [FAIL] L${cycle}a 第${cycle}次启用 -> 不在导出"
    FAIL=$((FAIL + 1))
  fi

  et_disable "$EXP_TOGGLE_ID"
  body=$(get_export "/event_types")
  TOTAL=$((TOTAL + 1))
  HIT=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_TOGGLE_NAME\") | .name" | tr -d '\r')
  if [ -z "$HIT" ]; then
    echo "  [PASS] L${cycle}b 第${cycle}次停用 -> 不在导出"
    PASS=$((PASS + 1))
  else
    echo "  [FAIL] L${cycle}b 第${cycle}次停用 -> 仍在导出"
    FAIL=$((FAIL + 1))
  fi
done

# =============================================================================
subsection "8-M: ATTACK — 快速启停 toggle 后导出一致性"
# =============================================================================

EXP_RAPID_NAME="${P}exp_rapid_fsm"
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"$EXP_RAPID_NAME"'",
  "display_name":"Rapid Toggle",
  "initial_state":"x",
  "states":[{"name":"x"},{"name":"y"}],
  "transitions":[{"from":"x","to":"y","priority":0,"condition":{}}]
}')")
assert_code "M1 创建 rapid toggle FSM" "0" "$body"
EXP_RAPID_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 快速: enable -> disable -> enable -> disable -> enable
fsm_enable "$EXP_RAPID_ID"
fsm_disable "$EXP_RAPID_ID"
fsm_enable "$EXP_RAPID_ID"
fsm_disable "$EXP_RAPID_ID"
fsm_enable "$EXP_RAPID_ID"

# 最终是 enabled，应该在导出
body=$(get_export "/fsm_configs")
TOTAL=$((TOTAL + 1))
RAPID_HIT=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_RAPID_NAME\") | .name" | tr -d '\r')
if [ "$RAPID_HIT" = "$EXP_RAPID_NAME" ]; then
  echo "  [PASS] M2 快速toggle后(最终enabled)出现在导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] M2 快速toggle后(最终enabled)未出现在导出"
  FAIL=$((FAIL + 1))
fi

# 再停用
fsm_disable "$EXP_RAPID_ID"
body=$(get_export "/fsm_configs")
TOTAL=$((TOTAL + 1))
RAPID_HIT2=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_RAPID_NAME\") | .name" | tr -d '\r')
if [ -z "$RAPID_HIT2" ]; then
  echo "  [PASS] M3 快速toggle后(最终disabled)不在导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] M3 快速toggle后(最终disabled)仍在导出"
  FAIL=$((FAIL + 1))
fi

# =============================================================================
subsection "8-N: 跨模块一致性 — disabled/deleted 不在导出"
# =============================================================================

# 验证之前删除/未启用的都不在导出
body=$(get_export "/event_types")
TOTAL=$((TOTAL + 1))
ET_STALE1=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_ET_NAME1\") | .name" | tr -d '\r')
ET_STALE3=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_ET_NAME3\") | .name" | tr -d '\r')
if [ -z "$ET_STALE1" ] && [ -z "$ET_STALE3" ]; then
  echo "  [PASS] N1a 已删/未启用事件不在导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] N1a 过期事件仍在导出 (del=$ET_STALE1, dis=$ET_STALE3)"
  FAIL=$((FAIL + 1))
fi

body=$(get_export "/fsm_configs")
TOTAL=$((TOTAL + 1))
FSM_STALE1=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_FSM_NAME1\") | .name" | tr -d '\r')
FSM_STALE3=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_FSM_NAME3\") | .name" | tr -d '\r')
if [ -z "$FSM_STALE1" ] && [ -z "$FSM_STALE3" ]; then
  echo "  [PASS] N1b 已删/未启用FSM不在导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] N1b 过期FSM仍在导出 (del=$FSM_STALE1, dis=$FSM_STALE3)"
  FAIL=$((FAIL + 1))
fi

# N2: 仍启用的项还在
body=$(get_export "/event_types")
TOTAL=$((TOTAL + 1))
ET_STILL=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_ET_NAME2\") | .name" | tr -d '\r')
if [ "$ET_STILL" = "$EXP_ET_NAME2" ]; then
  echo "  [PASS] N2a fire event 仍在导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] N2a fire event 不在导出"
  FAIL=$((FAIL + 1))
fi

body=$(get_export "/fsm_configs")
TOTAL=$((TOTAL + 1))
FSM_STILL=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_FSM_NAME2\") | .name" | tr -d '\r')
if [ "$FSM_STILL" = "$EXP_FSM_NAME2" ]; then
  echo "  [PASS] N2b guard FSM 仍在导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] N2b guard FSM 不在导出"
  FAIL=$((FAIL + 1))
fi

# ---- 清理测试数据 ----
et_rm "$EXP_ET_ID2" 2>/dev/null
et_rm "$EXP_ET_ID3" 2>/dev/null
et_rm "$EXP_GLOBAL_ID" 2>/dev/null
et_rm "$EXP_TOGGLE_ID" 2>/dev/null
fsm_rm "$EXP_FSM_ID2" 2>/dev/null
fsm_rm "$EXP_FSM_ID3" 2>/dev/null
fsm_rm "$EXP_CYCLE_ID" 2>/dev/null
fsm_rm "$EXP_RAPID_ID" 2>/dev/null
