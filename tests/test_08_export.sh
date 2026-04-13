#!/bin/bash
# =============================================================================
# test_08_export.sh — 导出 API 跨模块测试
#
# 前置：run_all.sh 已 source helpers.sh，$BASE / $P / assert_* / post() /
#       et_* / fsm_* / get_export() 可用
#
# 覆盖：事件类型导出 + FSM 导出 + 格式验证 + 启用/停用/删除联动
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
subsection "8-A: 事件类型导出"
# =============================================================================

# A1: 创建事件类型并启用
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

# A2: 创建一个未启用的事件类型
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

# A3: 导出 — 只包含已启用项
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

# A5: 导出格式验证 — {items: [{name, config}, ...]}
assert_field "A5a 导出有 items 字段" '.items | type' "array" "$body"

TOTAL=$((TOTAL + 1))
ET_FIRST_NAME=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_ET_NAME1\") | .name" | tr -d '\r')
ET_FIRST_CFG=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_ET_NAME1\") | .config | type" | tr -d '\r')
if [ -n "$ET_FIRST_NAME" ] && [ "$ET_FIRST_CFG" = "object" ]; then
  echo "  [PASS] A5b 导出格式 {name, config} 正确"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] A5b 导出格式异常 name=$ET_FIRST_NAME config_type=$ET_FIRST_CFG"
  FAIL=$((FAIL + 1))
fi

# A6: 事件类型 config 内含必要字段: display_name, default_severity, default_ttl, perception_mode, range
ET_CFG_ITEM=$(echo "$body" | jq ".items[] | select(.name==\"$EXP_ET_NAME1\") | .config")
TOTAL=$((TOTAL + 1))
ET_HAS_DN=$(echo "$ET_CFG_ITEM" | jq -r '.display_name // empty' | tr -d '\r')
ET_HAS_SEV=$(echo "$ET_CFG_ITEM" | jq -r '.default_severity // empty' | tr -d '\r')
ET_HAS_TTL=$(echo "$ET_CFG_ITEM" | jq -r '.default_ttl // empty' | tr -d '\r')
ET_HAS_PM=$(echo "$ET_CFG_ITEM" | jq -r '.perception_mode // empty' | tr -d '\r')
ET_HAS_RNG=$(echo "$ET_CFG_ITEM" | jq -r '.range' | tr -d '\r')
if [ -n "$ET_HAS_DN" ] && [ -n "$ET_HAS_SEV" ] && [ -n "$ET_HAS_TTL" ] && [ -n "$ET_HAS_PM" ] && [ "$ET_HAS_RNG" != "null" ]; then
  echo "  [PASS] A6 event config 含 display_name/severity/ttl/perception_mode/range"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] A6 event config 缺少字段 (dn=$ET_HAS_DN sev=$ET_HAS_SEV ttl=$ET_HAS_TTL pm=$ET_HAS_PM rng=$ET_HAS_RNG)"
  FAIL=$((FAIL + 1))
fi

# A7: Disable -> verify NOT in export
et_disable "$EXP_ET_ID1"
body=$(get_export "/event_types")
TOTAL=$((TOTAL + 1))
ET_AFTER_DISABLE=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_ET_NAME1\") | .name" | tr -d '\r')
if [ -z "$ET_AFTER_DISABLE" ]; then
  echo "  [PASS] A7 停用后 earthquake 不在导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] A7 停用后 earthquake 仍在导出"
  FAIL=$((FAIL + 1))
fi

# A8: Re-enable -> verify back in export
et_enable "$EXP_ET_ID1"
body=$(get_export "/event_types")
TOTAL=$((TOTAL + 1))
ET_REENABLE=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_ET_NAME1\") | .name" | tr -d '\r')
if [ "$ET_REENABLE" = "$EXP_ET_NAME1" ]; then
  echo "  [PASS] A8 重新启用后 earthquake 回到导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] A8 重新启用后 earthquake 未回到导出"
  FAIL=$((FAIL + 1))
fi

# A9: Delete -> verify NOT in export
et_disable "$EXP_ET_ID1"
post "/event-types/delete" "{\"id\":$EXP_ET_ID1}" > /dev/null
body=$(get_export "/event_types")
TOTAL=$((TOTAL + 1))
ET_AFTER_DEL=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_ET_NAME1\") | .name" | tr -d '\r')
if [ -z "$ET_AFTER_DEL" ]; then
  echo "  [PASS] A9 删除后 earthquake 不在导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] A9 删除后 earthquake 仍在导出"
  FAIL=$((FAIL + 1))
fi

# =============================================================================
subsection "8-B: FSM 导出"
# =============================================================================

# B1: 创建 FSM 并启用
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

# B2: 创建未启用 FSM
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"$EXP_FSM_NAME3"'",
  "display_name":"Disabled FSM",
  "initial_state":"s1",
  "states":[{"name":"s1"}],
  "transitions":[]
}')")
assert_code "B2 创建未启用 FSM" "0" "$body"
EXP_FSM_ID3=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# B3: 导出 — 只包含已启用项
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

# B5: 导出格式验证
assert_field "B5a 导出有 items 字段" '.items | type' "array" "$body"

TOTAL=$((TOTAL + 1))
FSM_FIRST_NAME=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_FSM_NAME1\") | .name" | tr -d '\r')
FSM_FIRST_CFG=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_FSM_NAME1\") | .config | type" | tr -d '\r')
if [ -n "$FSM_FIRST_NAME" ] && [ "$FSM_FIRST_CFG" = "object" ]; then
  echo "  [PASS] B5b FSM 导出格式 {name, config} 正确"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] B5b FSM 导出格式异常 name=$FSM_FIRST_NAME config_type=$FSM_FIRST_CFG"
  FAIL=$((FAIL + 1))
fi

# B6: FSM config 内含必要字段: initial_state, states, transitions
FSM_CFG_ITEM=$(echo "$body" | jq ".items[] | select(.name==\"$EXP_FSM_NAME1\") | .config")
TOTAL=$((TOTAL + 1))
FSM_HAS_INIT=$(echo "$FSM_CFG_ITEM" | jq -r '.initial_state // empty' | tr -d '\r')
FSM_HAS_STATES=$(echo "$FSM_CFG_ITEM" | jq -r '.states | length' | tr -d '\r')
FSM_HAS_TRANS_TYPE=$(echo "$FSM_CFG_ITEM" | jq -r '.transitions | type' | tr -d '\r')
if [ -n "$FSM_HAS_INIT" ] && [ "$FSM_HAS_STATES" -gt 0 ] 2>/dev/null && [ "$FSM_HAS_TRANS_TYPE" = "array" ]; then
  echo "  [PASS] B6 FSM config 含 initial_state/states/transitions"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] B6 FSM config 缺少字段 (init=$FSM_HAS_INIT states=$FSM_HAS_STATES trans_type=$FSM_HAS_TRANS_TYPE)"
  FAIL=$((FAIL + 1))
fi

# B7: Verify FSM config.initial_state matches what was set
TOTAL=$((TOTAL + 1))
if [ "$FSM_HAS_INIT" = "idle" ]; then
  echo "  [PASS] B7 FSM config.initial_state=idle"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] B7 FSM config.initial_state 期望 idle, 实际 $FSM_HAS_INIT"
  FAIL=$((FAIL + 1))
fi

# B8: Verify FSM config.states count
TOTAL=$((TOTAL + 1))
if [ "$FSM_HAS_STATES" = "3" ]; then
  echo "  [PASS] B8 FSM config.states 数量=3"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] B8 FSM config.states 期望 3, 实际 $FSM_HAS_STATES"
  FAIL=$((FAIL + 1))
fi

# B9: Disable -> verify NOT in export
fsm_disable "$EXP_FSM_ID1"
body=$(get_export "/fsm_configs")
TOTAL=$((TOTAL + 1))
FSM_AFTER_DISABLE=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_FSM_NAME1\") | .name" | tr -d '\r')
if [ -z "$FSM_AFTER_DISABLE" ]; then
  echo "  [PASS] B9 停用后 wolf FSM 不在导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] B9 停用后 wolf FSM 仍在导出"
  FAIL=$((FAIL + 1))
fi

# B10: Re-enable -> verify back in export
fsm_enable "$EXP_FSM_ID1"
body=$(get_export "/fsm_configs")
TOTAL=$((TOTAL + 1))
FSM_REENABLE=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_FSM_NAME1\") | .name" | tr -d '\r')
if [ "$FSM_REENABLE" = "$EXP_FSM_NAME1" ]; then
  echo "  [PASS] B10 重新启用后 wolf FSM 回到导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] B10 重新启用后 wolf FSM 未回到导出"
  FAIL=$((FAIL + 1))
fi

# B11: Delete -> verify NOT in export
fsm_disable "$EXP_FSM_ID1"
post "/fsm-configs/delete" "{\"id\":$EXP_FSM_ID1}" > /dev/null
body=$(get_export "/fsm_configs")
TOTAL=$((TOTAL + 1))
FSM_AFTER_DEL=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_FSM_NAME1\") | .name" | tr -d '\r')
if [ -z "$FSM_AFTER_DEL" ]; then
  echo "  [PASS] B11 删除后 wolf FSM 不在导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] B11 删除后 wolf FSM 仍在导出"
  FAIL=$((FAIL + 1))
fi

# =============================================================================
subsection "8-C: 空导出 + 跨模块一致性"
# =============================================================================

# C1: 导出 items 始终是数组（即使所有数据都已清除）
body=$(get_export "/event_types")
assert_field "C1a event_types items 是 array" '.items | type' "array" "$body"

body=$(get_export "/fsm_configs")
assert_field "C1b fsm_configs items 是 array" '.items | type' "array" "$body"

# C2: 全面清理后导出 — 验证 disabled/deleted 的都不在
# (EXP_ET_ID1 已删, EXP_ET_ID3 未启用, EXP_FSM_ID1 已删, EXP_FSM_ID3 未启用)
body=$(get_export "/event_types")
TOTAL=$((TOTAL + 1))
ET_STALE1=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_ET_NAME1\") | .name" | tr -d '\r')
ET_STALE3=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_ET_NAME3\") | .name" | tr -d '\r')
if [ -z "$ET_STALE1" ] && [ -z "$ET_STALE3" ]; then
  echo "  [PASS] C2a 已删/未启用事件不在导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] C2a 过期事件仍在导出 (del=$ET_STALE1, dis=$ET_STALE3)"
  FAIL=$((FAIL + 1))
fi

body=$(get_export "/fsm_configs")
TOTAL=$((TOTAL + 1))
FSM_STALE1=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_FSM_NAME1\") | .name" | tr -d '\r')
FSM_STALE3=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_FSM_NAME3\") | .name" | tr -d '\r')
if [ -z "$FSM_STALE1" ] && [ -z "$FSM_STALE3" ]; then
  echo "  [PASS] C2b 已删/未启用FSM不在导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] C2b 过期FSM仍在导出 (del=$FSM_STALE1, dis=$FSM_STALE3)"
  FAIL=$((FAIL + 1))
fi

# C3: 仍启用的项还在（fire event + guard FSM）
body=$(get_export "/event_types")
TOTAL=$((TOTAL + 1))
ET_STILL=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_ET_NAME2\") | .name" | tr -d '\r')
if [ "$ET_STILL" = "$EXP_ET_NAME2" ]; then
  echo "  [PASS] C3a fire event 仍在导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] C3a fire event 不在导出"
  FAIL=$((FAIL + 1))
fi

body=$(get_export "/fsm_configs")
TOTAL=$((TOTAL + 1))
FSM_STILL=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_FSM_NAME2\") | .name" | tr -d '\r')
if [ "$FSM_STILL" = "$EXP_FSM_NAME2" ]; then
  echo "  [PASS] C3b guard FSM 仍在导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] C3b guard FSM 不在导出"
  FAIL=$((FAIL + 1))
fi

# C4: Create -> Enable -> Verify in export -> Disable -> Verify NOT in export (full cycle)
EXP_CYCLE_NAME="${P}exp_cycle_fsm"
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"$EXP_CYCLE_NAME"'",
  "display_name":"Cycle Test",
  "initial_state":"a",
  "states":[{"name":"a"},{"name":"b"}],
  "transitions":[{"from":"a","to":"b","priority":0,"condition":{}}]
}')")
assert_code "C4a 创建 cycle FSM" "0" "$body"
EXP_CYCLE_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# Not in export before enable
body=$(get_export "/fsm_configs")
TOTAL=$((TOTAL + 1))
CYCLE_PRE=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_CYCLE_NAME\") | .name" | tr -d '\r')
if [ -z "$CYCLE_PRE" ]; then
  echo "  [PASS] C4b 启用前不在导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] C4b 启用前已在导出"
  FAIL=$((FAIL + 1))
fi

# Enable -> in export
fsm_enable "$EXP_CYCLE_ID"
body=$(get_export "/fsm_configs")
TOTAL=$((TOTAL + 1))
CYCLE_EN=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_CYCLE_NAME\") | .name" | tr -d '\r')
if [ "$CYCLE_EN" = "$EXP_CYCLE_NAME" ]; then
  echo "  [PASS] C4c 启用后出现在导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] C4c 启用后未出现在导出"
  FAIL=$((FAIL + 1))
fi

# Disable -> not in export
fsm_disable "$EXP_CYCLE_ID"
body=$(get_export "/fsm_configs")
TOTAL=$((TOTAL + 1))
CYCLE_DIS=$(echo "$body" | jq -r ".items[] | select(.name==\"$EXP_CYCLE_NAME\") | .name" | tr -d '\r')
if [ -z "$CYCLE_DIS" ]; then
  echo "  [PASS] C4d 再次停用后不在导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] C4d 再次停用后仍在导出"
  FAIL=$((FAIL + 1))
fi

# ---- 清理测试数据 ----
et_rm "$EXP_ET_ID2" 2>/dev/null
et_rm "$EXP_ET_ID3" 2>/dev/null
fsm_rm "$EXP_FSM_ID2" 2>/dev/null
fsm_rm "$EXP_FSM_ID3" 2>/dev/null
fsm_rm "$EXP_CYCLE_ID" 2>/dev/null
