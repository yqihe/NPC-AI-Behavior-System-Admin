#!/bin/bash
# =============================================================================
# test_07_fsm.sh — FSM (状态机) 管理全方位测试
#
# 前置：run_all.sh 已 source helpers.sh，$BASE / $P / assert_* / post() / fsm_* 可用
#
# 覆盖：CRUD + 全错误码 43001-43012 + 条件树深度攻击 + 生命周期 + 攻击性测试
# =============================================================================

section "Part 7: 状态机管理 (FSM)  (prefix=$P)"

# ---- 测试数据变量 ----
FSM_NAME1="${P}wolf_fsm"
FSM_NAME2="${P}guard_fsm"
FSM_NAME5="${P}lifecycle_fsm"

# ---- 条件测试通用模板 ----
fsm_cond_test() {
  local test_name="$1" cond="$2" expect_code="$3"
  local unique_name="${P}cond_$(echo "$test_name" | tr ' ' '_' | tr -cd 'a-z0-9_' | head -c 40)"
  local body
  body=$(post "/fsm-configs/create" "$(printf '%s' '{
    "name":"'"$unique_name"'",
    "display_name":"cond test",
    "initial_state":"a",
    "states":[{"name":"a"},{"name":"b"}],
    "transitions":[{"from":"a","to":"b","priority":0,"condition":'"$cond"'}]
  }')")
  assert_code "$test_name" "$expect_code" "$body"
  if [ "$expect_code" = "0" ]; then
    local created_id=$(echo "$body" | jq -r '.data.id // empty' | tr -d '\r')
    [ -n "$created_id" ] && [ "$created_id" != "null" ] && fsm_rm "$created_id"
  fi
}

# ---- 攻击辅助：创建+清理 ----
fsm_atk() {
  local name="$1" body_in="$2"
  local R=$(post "/fsm-configs/create" "$body_in")
  local id=$(echo "$R" | jq -r '.data.id // empty' | tr -d '\r')
  [ -n "$id" ] && [ "$id" != "null" ] && fsm_rm "$id" 2>/dev/null
  echo "$R"
}

# =============================================================================
subsection "7-A: FSM CRUD 基本操作"
# =============================================================================

# A1: 创建带 3 个状态、2 个转换(含条件)的 FSM
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"$FSM_NAME1"'",
  "display_name":"'"${P}"'Wolf FSM",
  "initial_state":"idle",
  "states":[{"name":"idle"},{"name":"patrol"},{"name":"chase"}],
  "transitions":[
    {"from":"idle","to":"patrol","priority":1,"condition":{"key":"time_of_day","op":"==","value":"\"night\""}},
    {"from":"patrol","to":"chase","priority":2,"condition":{"key":"enemy_distance","op":"<","value":100}}
  ]
}')")
assert_code "A1 创建 FSM 成功" "0" "$body"
FSM_ID1=$(echo "$body" | jq -r '.data.id' | tr -d '\r')
assert_not_equal "A1 返回有效 ID" '.data.id' "null" "$body"
assert_field "A1 返回 name" '.data.name' "$FSM_NAME1" "$body"

# A2: 详情返回正确结构
body=$(fsm_detail "$FSM_ID1")
assert_code  "A2 detail 成功" "0" "$body"
assert_field "A2 name" '.data.name' "$FSM_NAME1" "$body"
assert_field "A2 enabled=false (创建默认停用)" '.data.enabled' "false" "$body"
assert_field "A2 version=1" '.data.version' "1" "$body"
assert_field "A2 config.initial_state" '.data.config.initial_state' "idle" "$body"
assert_field "A2 states 数量=3" '.data.config.states | length' "3" "$body"
assert_field "A2 transitions 数量=2" '.data.config.transitions | length' "2" "$body"

# A3: 列表包含新 FSM + enrichment
body=$(post "/fsm-configs/list" '{"page":1,"page_size":50}')
assert_code "A3 list 成功" "0" "$body"
assert_not_equal "A3 列表 total > 0" '.data.total' "0" "$body"

# A4: 列表 enrichment (initial_state, state_count)
FSM_LIST_INITIAL=$(echo "$body" | jq -r ".data.items[] | select(.name==\"$FSM_NAME1\") | .initial_state" | tr -d '\r')
FSM_LIST_SC=$(echo "$body" | jq -r ".data.items[] | select(.name==\"$FSM_NAME1\") | .state_count" | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$FSM_LIST_INITIAL" = "idle" ]; then
  echo "  [PASS] A4a 列表 initial_state=idle"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] A4a 列表 initial_state — 期望 idle, 实际: $FSM_LIST_INITIAL"
  FAIL=$((FAIL + 1))
fi
TOTAL=$((TOTAL + 1))
if [ "$FSM_LIST_SC" = "3" ]; then
  echo "  [PASS] A4b 列表 state_count=3"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] A4b 列表 state_count — 期望 3, 实际: $FSM_LIST_SC"
  FAIL=$((FAIL + 1))
fi

# A5: 更新 FSM（添加状态 + 改转换）
FSM_V=$(fsm_version "$FSM_ID1")
body=$(post "/fsm-configs/update" "$(printf '%s' '{
  "id":'"$FSM_ID1"',
  "display_name":"'"${P}"'Wolf FSM v2",
  "initial_state":"idle",
  "states":[{"name":"idle"},{"name":"patrol"},{"name":"chase"},{"name":"flee"}],
  "transitions":[
    {"from":"idle","to":"patrol","priority":1,"condition":{"key":"time_of_day","op":"==","value":"\"night\""}},
    {"from":"patrol","to":"chase","priority":2,"condition":{"key":"enemy_distance","op":"<","value":100}},
    {"from":"chase","to":"flee","priority":3,"condition":{"key":"hp","op":"<","value":20}}
  ],
  "version":'"$FSM_V"'
}')")
assert_code "A5 更新 FSM 成功" "0" "$body"

# A6: 验证更新生效
body=$(fsm_detail "$FSM_ID1")
assert_field "A6 states 数量=4" '.data.config.states | length' "4" "$body"
assert_field "A6 transitions 数量=3" '.data.config.transitions | length' "3" "$body"
assert_field "A6 display_name 更新" '.data.display_name' "${P}Wolf FSM v2" "$body"

# A7: check-name（可用 / 不可用）
body=$(post "/fsm-configs/check-name" "{\"name\":\"${P}unused_name\"}")
assert_code  "A7a check-name 可用" "0" "$body"
assert_field "A7a available=true" '.data.available' "true" "$body"

body=$(post "/fsm-configs/check-name" "{\"name\":\"$FSM_NAME1\"}")
assert_code  "A7b check-name 已占用" "0" "$body"
assert_field "A7b available=false" '.data.available' "false" "$body"

# A8: toggle enabled/disabled
fsm_enable "$FSM_ID1"
body=$(fsm_detail "$FSM_ID1")
assert_field "A8a 启用后 enabled=true" '.data.enabled' "true" "$body"

fsm_disable "$FSM_ID1"
body=$(fsm_detail "$FSM_ID1")
assert_field "A8b 停用后 enabled=false" '.data.enabled' "false" "$body"

# A9: 软删除（必须先停用）— 创建第二个 FSM 用于删除测试
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"$FSM_NAME2"'",
  "display_name":"Guard FSM",
  "initial_state":"stand",
  "states":[{"name":"stand"},{"name":"alert"}],
  "transitions":[{"from":"stand","to":"alert","priority":1,"condition":{"key":"noise","op":">","value":50}}]
}')")
assert_code "A9a 创建第二个 FSM" "0" "$body"
FSM_ID2=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

body=$(post "/fsm-configs/delete" "{\"id\":$FSM_ID2}")
assert_code "A9b 停用状态下删除成功" "0" "$body"

# A10: 验证删除后不在列表/详情
body=$(fsm_detail "$FSM_ID2")
assert_code "A10a 已删 FSM 详情返回 43003" "43003" "$body"

body=$(post "/fsm-configs/list" '{"page":1,"page_size":50}')
FSM_DELETED_HIT=$(echo "$body" | jq -r ".data.items[] | select(.id==$FSM_ID2) | .id" | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ -z "$FSM_DELETED_HIT" ]; then
  echo "  [PASS] A10b 已删 FSM 不出现在列表"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] A10b 已删 FSM 仍出现在列表 id=$FSM_DELETED_HIT"
  FAIL=$((FAIL + 1))
fi

# =============================================================================
subsection "7-B: FSM 校验 — 全错误码覆盖 (43001-43012)"
# =============================================================================

# B1: 43001 Name already exists
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"$FSM_NAME1"'",
  "display_name":"Dup",
  "initial_state":"idle",
  "states":[{"name":"idle"}],
  "transitions":[]
}')")
assert_code "B1 name 重复 -> 43001" "43001" "$body"

# B2: 43002 Name format invalid — 多种变体
body=$(post "/fsm-configs/create" '{"name":"","display_name":"x","initial_state":"a","states":[{"name":"a"}],"transitions":[]}')
assert_code "B2a 空 name -> 43002" "43002" "$body"

body=$(post "/fsm-configs/create" '{"name":"UPPERCASE","display_name":"x","initial_state":"a","states":[{"name":"a"}],"transitions":[]}')
assert_code "B2b 大写 name -> 43002" "43002" "$body"

body=$(post "/fsm-configs/create" '{"name":"bad name","display_name":"x","initial_state":"a","states":[{"name":"a"}],"transitions":[]}')
assert_code "B2c 含空格 -> 43002" "43002" "$body"

body=$(post "/fsm-configs/create" '{"name":"123abc","display_name":"x","initial_state":"a","states":[{"name":"a"}],"transitions":[]}')
assert_code "B2d 数字开头 -> 43002" "43002" "$body"

body=$(post "/fsm-configs/create" '{"name":"a@b#c","display_name":"x","initial_state":"a","states":[{"name":"a"}],"transitions":[]}')
assert_code "B2e 特殊字符 -> 43002" "43002" "$body"

# 100-char name exceeds 64
FSM_LONG_NAME=$(printf 'a%.0s' {1..100})
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"$FSM_LONG_NAME"'",
  "display_name":"long",
  "initial_state":"a",
  "states":[{"name":"a"}],
  "transitions":[]
}')")
assert_code "B2f 100 字符 name > 64 -> 43002" "43002" "$body"

# B3: 43003 Not found — detail, update, delete, toggle
body=$(post "/fsm-configs/detail" '{"id":999999}')
assert_code "B3a detail 不存在 -> 43003" "43003" "$body"

body=$(post "/fsm-configs/update" '{"id":999999,"display_name":"x","initial_state":"a","states":[{"name":"a"}],"transitions":[],"version":1}')
assert_code "B3b update 不存在 -> 43003" "43003" "$body"

body=$(post "/fsm-configs/delete" '{"id":999999}')
assert_code "B3c delete 不存在 -> 43003" "43003" "$body"

body=$(post "/fsm-configs/toggle-enabled" '{"id":999999,"enabled":true,"version":1}')
assert_code "B3d toggle 不存在 -> 43003" "43003" "$body"

# B4: 43004 Empty states array
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}empty_states"'",
  "display_name":"x",
  "initial_state":"idle",
  "states":[],
  "transitions":[]
}')")
assert_code "B4 空 states -> 43004" "43004" "$body"

# B5a: 43005 Duplicate state name
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}dup_state"'",
  "display_name":"x",
  "initial_state":"idle",
  "states":[{"name":"idle"},{"name":"idle"}],
  "transitions":[]
}')")
assert_code "B5a 重复 state name -> 43005" "43005" "$body"

# B5b: 43005 Empty state name
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}empty_state_name"'",
  "display_name":"x",
  "initial_state":"idle",
  "states":[{"name":"idle"},{"name":""}],
  "transitions":[]
}')")
assert_code "B5b 空 state name -> 43005" "43005" "$body"

# B6: 43006 initial_state not in states
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}bad_initial"'",
  "display_name":"x",
  "initial_state":"nonexistent",
  "states":[{"name":"idle"},{"name":"run"}],
  "transitions":[]
}')")
assert_code "B6 initial_state 不在 states -> 43006" "43006" "$body"

# B7a: 43007 Transition from unknown state
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}bad_from"'",
  "display_name":"x",
  "initial_state":"idle",
  "states":[{"name":"idle"},{"name":"run"}],
  "transitions":[{"from":"ghost","to":"run","priority":0,"condition":{}}]
}')")
assert_code "B7a from 不存在 -> 43007" "43007" "$body"

# B7b: 43007 Transition to unknown state
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}bad_to"'",
  "display_name":"x",
  "initial_state":"idle",
  "states":[{"name":"idle"},{"name":"run"}],
  "transitions":[{"from":"idle","to":"ghost","priority":0,"condition":{}}]
}')")
assert_code "B7b to 不存在 -> 43007" "43007" "$body"

# B7c: 43007 Negative priority
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}neg_pri"'",
  "display_name":"x",
  "initial_state":"idle",
  "states":[{"name":"idle"},{"name":"run"}],
  "transitions":[{"from":"idle","to":"run","priority":-1,"condition":{}}]
}')")
assert_code "B7c 负 priority -> 43007" "43007" "$body"

# B8a: 43008 Invalid condition operator
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}bad_op"'",
  "display_name":"x",
  "initial_state":"idle",
  "states":[{"name":"idle"},{"name":"run"}],
  "transitions":[{"from":"idle","to":"run","priority":0,"condition":{"key":"x","op":"LIKE","value":1}}]
}')")
assert_code "B8a 非法 op LIKE -> 43008" "43008" "$body"

body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}bad_op2"'",
  "display_name":"x",
  "initial_state":"idle",
  "states":[{"name":"idle"},{"name":"run"}],
  "transitions":[{"from":"idle","to":"run","priority":0,"condition":{"key":"x","op":"BETWEEN","value":1}}]
}')")
assert_code "B8a2 非法 op BETWEEN -> 43008" "43008" "$body"

# B8b: 43008 Condition with both value AND ref_key
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}both_val_ref"'",
  "display_name":"x",
  "initial_state":"idle",
  "states":[{"name":"idle"},{"name":"run"}],
  "transitions":[{"from":"idle","to":"run","priority":0,"condition":{"key":"x","op":"==","value":1,"ref_key":"y"}}]
}')")
assert_code "B8b value+ref_key 同时 -> 43008" "43008" "$body"

# B8c: 43008 Condition with neither value NOR ref_key
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}no_val_ref"'",
  "display_name":"x",
  "initial_state":"idle",
  "states":[{"name":"idle"},{"name":"run"}],
  "transitions":[{"from":"idle","to":"run","priority":0,"condition":{"key":"x","op":"=="}}]
}')")
assert_code "B8c 无 value 无 ref_key -> 43008" "43008" "$body"

# B8d: 43008 Condition with both key AND and/or (leaf+composite)
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}leaf_composite"'",
  "display_name":"x",
  "initial_state":"idle",
  "states":[{"name":"idle"},{"name":"run"}],
  "transitions":[{"from":"idle","to":"run","priority":0,"condition":{"key":"x","op":"==","value":1,"and":[{"key":"y","op":"==","value":2}]}}]
}')")
assert_code "B8d key+and 同时 -> 43008" "43008" "$body"

# B8e: 43008 Condition with both and AND or
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}and_or_both"'",
  "display_name":"x",
  "initial_state":"idle",
  "states":[{"name":"idle"},{"name":"run"}],
  "transitions":[{"from":"idle","to":"run","priority":0,"condition":{"and":[{"key":"a","op":"==","value":1}],"or":[{"key":"b","op":"==","value":2}]}}]
}')")
assert_code "B8e and+or 同时 -> 43008" "43008" "$body"

# B8f: 43008 Condition nesting too deep (> 10 levels)
FSM_DEEP_COND='{"key":"x","op":"==","value":1}'
for i in $(seq 1 11); do
  FSM_DEEP_COND="{\"and\":[$FSM_DEEP_COND]}"
done
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}deep_cond"'",
  "display_name":"x",
  "initial_state":"idle",
  "states":[{"name":"idle"},{"name":"run"}],
  "transitions":[{"from":"idle","to":"run","priority":0,"condition":'"$FSM_DEEP_COND"'}]
}')")
assert_code "B8f 嵌套深度 >10 -> 43008" "43008" "$body"

# B9: 43009 Delete while enabled
fsm_enable "$FSM_ID1"
body=$(post "/fsm-configs/delete" "{\"id\":$FSM_ID1}")
assert_code "B9 启用中删除 -> 43009" "43009" "$body"

# B10: 43010 Edit while enabled
FSM_V=$(fsm_version "$FSM_ID1")
body=$(post "/fsm-configs/update" "$(printf '%s' '{
  "id":'"$FSM_ID1"',
  "display_name":"try edit",
  "initial_state":"idle",
  "states":[{"name":"idle"},{"name":"patrol"},{"name":"chase"},{"name":"flee"}],
  "transitions":[],
  "version":'"$FSM_V"'
}')")
assert_code "B10 启用中编辑 -> 43010" "43010" "$body"

# 恢复停用
fsm_disable "$FSM_ID1"

# B11a: 43011 Version conflict on update
FSM_V=$(fsm_version "$FSM_ID1")
FSM_V_STALE=$((FSM_V - 1))
body=$(post "/fsm-configs/update" "$(printf '%s' '{
  "id":'"$FSM_ID1"',
  "display_name":"conflict",
  "initial_state":"idle",
  "states":[{"name":"idle"},{"name":"patrol"},{"name":"chase"},{"name":"flee"}],
  "transitions":[],
  "version":'"$FSM_V_STALE"'
}')")
assert_code "B11a 版本冲突 update -> 43011" "43011" "$body"

# B11b: 43011 Version conflict on toggle
body=$(post "/fsm-configs/toggle-enabled" "{\"id\":$FSM_ID1,\"enabled\":true,\"version\":$FSM_V_STALE}")
assert_code "B11b 版本冲突 toggle -> 43011" "43011" "$body"

# B12: 43012 placeholder — ref_count 恒 0，此错误码本期不会触发
# 验证错误码已定义（通过尝试删除存在且停用的 FSM 成功来间接确认 ref_count=0 逻辑正确）
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}ref_del_test"'",
  "display_name":"ref del",
  "initial_state":"a",
  "states":[{"name":"a"}],
  "transitions":[]
}')")
FSM_REF_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')
body=$(post "/fsm-configs/delete" "{\"id\":$FSM_REF_ID}")
assert_code "B12 无引用时可正常删除(43012 本期不触发)" "0" "$body"

# =============================================================================
subsection "7-C: 条件树深度攻击测试"
# =============================================================================

# C1: 空条件 = 无条件转换
fsm_cond_test "C1 空条件(无条件转换)" '{}' "0"

# C2: 每种合法操作符
fsm_cond_test "C2a op==" '{"key":"x","op":"==","value":1}' "0"
fsm_cond_test "C2b op!=" '{"key":"x","op":"!=","value":1}' "0"
fsm_cond_test "C2c op>"  '{"key":"x","op":">","value":1}' "0"
fsm_cond_test "C2d op>=" '{"key":"x","op":">=","value":1}' "0"
fsm_cond_test "C2e op<"  '{"key":"x","op":"<","value":1}' "0"
fsm_cond_test "C2f op<=" '{"key":"x","op":"<=","value":1}' "0"
fsm_cond_test "C2g op=in" '{"key":"x","op":"in","value":[1,2,3]}' "0"

# C3: ref_key instead of value
fsm_cond_test "C3 ref_key 代替 value" '{"key":"x","op":"==","ref_key":"y"}' "0"

# C4: AND composite
fsm_cond_test "C4 and composite" '{"and":[{"key":"a","op":">","value":0},{"key":"b","op":"<","value":100}]}' "0"

# C5: OR composite
fsm_cond_test "C5 or composite" '{"or":[{"key":"a","op":"==","value":1},{"key":"b","op":"==","value":2}]}' "0"

# C6: Nested: and -> or -> leaf (3 levels)
fsm_cond_test "C6 and_or_leaf 3层嵌套" '{"and":[{"or":[{"key":"x","op":"==","value":1},{"key":"y","op":"!=","value":2}]}]}' "0"

# C7: Deeper: and -> or -> and -> leaf (4 levels)
fsm_cond_test "C7 4层嵌套" '{"and":[{"or":[{"and":[{"key":"x","op":"==","value":1}]}]}]}' "0"

# C8: Exactly 10 levels deep — should succeed (at limit)
FSM_DEPTH10='{"key":"x","op":"==","value":1}'
for i in $(seq 1 10); do
  FSM_DEPTH10="{\"and\":[$FSM_DEPTH10]}"
done
fsm_cond_test "C8 恰好10层深度(极限)" "$FSM_DEPTH10" "0"

# C9: 11 levels deep — should fail 43008
FSM_DEPTH11='{"key":"x","op":"==","value":1}'
for i in $(seq 1 11); do
  FSM_DEPTH11="{\"and\":[$FSM_DEPTH11]}"
done
fsm_cond_test "C9 11层深度(超限)" "$FSM_DEPTH11" "43008"

# C10: 9 levels — well within limit
FSM_DEPTH9='{"key":"x","op":"==","value":1}'
for i in $(seq 1 9); do
  FSM_DEPTH9="{\"and\":[$FSM_DEPTH9]}"
done
fsm_cond_test "C10 9层深度(安全)" "$FSM_DEPTH9" "0"

# C11: Empty and array {"and":[]}
# len(And)==0, Key=="", len(Or)==0 => IsEmpty()=true => valid
fsm_cond_test "C11 空and数组" '{"and":[]}' "0"

# C12: {"and":[{}]} — and with one empty child; child is empty => valid
fsm_cond_test "C12 and含一个空子节点" '{"and":[{}]}' "0"

# C13: value types — integer
fsm_cond_test "C13a value=integer" '{"key":"hp","op":">","value":42}' "0"

# C14: value types — float
fsm_cond_test "C14 value=float" '{"key":"speed","op":">=","value":3.14}' "0"

# C15: value types — string (quoted in JSON)
fsm_cond_test "C15 value=string" '{"key":"name","op":"==","value":"\"wolf\""}' "0"

# C16: value types — boolean
fsm_cond_test "C16 value=bool" '{"key":"alive","op":"==","value":true}' "0"

# C17: value types — array (for in op)
fsm_cond_test "C17 value=array(for in)" '{"key":"type","op":"in","value":["a","b","c"]}' "0"

# C18: value=null — treated as no value; ref_key also empty => 43008
# In Go: len(cond.Value)>0 && string(cond.Value)!="null" => false for null
fsm_cond_test "C18 value=null 无 ref_key" '{"key":"x","op":"==","value":null}' "43008"

# C19: value=null but ref_key set — should succeed (ref_key provides the value)
fsm_cond_test "C19 value=null 有 ref_key" '{"key":"x","op":"==","value":null,"ref_key":"y"}' "0"

# C20: Leaf with op but no key: {"op":"==","value":1}
# Key="" => isLeaf=false, hasAnd=false, hasOr=false => IsEmpty()=true => valid (treated as empty)
# This is suspicious but the code allows it. Test and document.
fsm_cond_test "C20 op+value 无 key(IsEmpty=true)" '{"op":"==","value":1}' "0"

# C21: Leaf with key="" but op and value set — same as C20
fsm_cond_test "C21 key空字符串+op+value(IsEmpty=true)" '{"key":"","op":"==","value":1}' "0"

# C22: Very large numeric value
fsm_cond_test "C22 超大数值" '{"key":"x","op":"==","value":99999999999999}' "0"

# C23: Negative value
fsm_cond_test "C23 负数值" '{"key":"x","op":"<","value":-999}' "0"

# C24: Empty string value
fsm_cond_test "C24 空字符串value" '{"key":"x","op":"==","value":"\"\""}' "0"

# C25: Multiple children in AND
fsm_cond_test "C25 and含5个子节点" '{"and":[{"key":"a","op":"==","value":1},{"key":"b","op":"==","value":2},{"key":"c","op":"==","value":3},{"key":"d","op":"==","value":4},{"key":"e","op":"==","value":5}]}' "0"

# C26: Mixed nesting: or -> and -> leaf, or -> leaf
fsm_cond_test "C26 混合嵌套" '{"or":[{"and":[{"key":"a","op":">","value":0}]},{"key":"b","op":"<","value":10}]}' "0"

# C27: Invalid op inside nested AND (should still be caught)
fsm_cond_test "C27 嵌套内非法op" '{"and":[{"key":"x","op":"NOPE","value":1}]}' "43008"

# C28: value+ref_key both set inside nested OR (should still be caught)
fsm_cond_test "C28 嵌套内value+ref_key" '{"or":[{"key":"x","op":"==","value":1,"ref_key":"y"}]}' "43008"

# C29: Neither value nor ref_key inside nested AND (should still be caught)
fsm_cond_test "C29 嵌套内无value无ref_key" '{"and":[{"key":"x","op":"=="}]}' "43008"

# C30: key+or mixed inside nested (leaf+composite)
fsm_cond_test "C30 嵌套内key+or混合" '{"and":[{"key":"x","op":"==","value":1,"or":[{"key":"y","op":"==","value":2}]}]}' "43008"

# C31: value = 0 (zero is a valid value, not null)
# In Go: json.RawMessage for 0 is []byte("0"), len=1 >0, string!="null" => hasValue=true
fsm_cond_test "C31 value=0(零值非null)" '{"key":"x","op":"==","value":0}' "0"

# C32: value = false (boolean false is a valid value)
fsm_cond_test "C32 value=false" '{"key":"x","op":"==","value":false}' "0"

# C33: value = "" (empty string in JSON is "\"\"" which is len>0 and !="null")
fsm_cond_test "C33 value=空JSON字符串" '{"key":"x","op":"==","value":""}' "0"

# =============================================================================
subsection "7-D: 生命周期 & 状态守卫"
# =============================================================================

# D1: 创建后默认 disabled
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"$FSM_NAME5"'",
  "display_name":"Lifecycle FSM",
  "initial_state":"idle",
  "states":[{"name":"idle"},{"name":"run"}],
  "transitions":[{"from":"idle","to":"run","priority":0,"condition":{}}]
}')")
assert_code "D1 创建生命周期 FSM" "0" "$body"
FSM_ID5=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

body=$(fsm_detail "$FSM_ID5")
assert_field "D1 创建后 enabled=false" '.data.enabled' "false" "$body"

# D2: disabled -> 可编辑
FSM_V5=$(fsm_version "$FSM_ID5")
body=$(post "/fsm-configs/update" "$(printf '%s' '{
  "id":'"$FSM_ID5"',
  "display_name":"Lifecycle v2",
  "initial_state":"idle",
  "states":[{"name":"idle"},{"name":"run"},{"name":"sleep"}],
  "transitions":[{"from":"idle","to":"sleep","priority":0,"condition":{}}],
  "version":'"$FSM_V5"'
}')")
assert_code "D2a disabled 可编辑" "0" "$body"

# D3: disabled -> 可删除 (创建临时 FSM)
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}life_del"'",
  "display_name":"Del Test",
  "initial_state":"a",
  "states":[{"name":"a"}],
  "transitions":[]
}')")
FSM_ID_LIFE_DEL=$(echo "$body" | jq -r '.data.id' | tr -d '\r')
body=$(post "/fsm-configs/delete" "{\"id\":$FSM_ID_LIFE_DEL}")
assert_code "D3 disabled 可删除" "0" "$body"

# D4: enabled -> 不可编辑 (43010) -> 不可删除 (43009)
fsm_enable "$FSM_ID5"
FSM_V5=$(fsm_version "$FSM_ID5")
body=$(post "/fsm-configs/update" "$(printf '%s' '{
  "id":'"$FSM_ID5"',
  "display_name":"try",
  "initial_state":"idle",
  "states":[{"name":"idle"},{"name":"run"},{"name":"sleep"}],
  "transitions":[],
  "version":'"$FSM_V5"'
}')")
assert_code "D4a 启用中编辑 -> 43010" "43010" "$body"

body=$(post "/fsm-configs/delete" "{\"id\":$FSM_ID5}")
assert_code "D4b 启用中删除 -> 43009" "43009" "$body"

# D5: enable -> disable -> edit -> re-enable 完整循环
fsm_disable "$FSM_ID5"
FSM_V5=$(fsm_version "$FSM_ID5")
body=$(post "/fsm-configs/update" "$(printf '%s' '{
  "id":'"$FSM_ID5"',
  "display_name":"Lifecycle v3",
  "initial_state":"idle",
  "states":[{"name":"idle"},{"name":"run"},{"name":"sleep"},{"name":"eat"}],
  "transitions":[{"from":"idle","to":"eat","priority":0,"condition":{}}],
  "version":'"$FSM_V5"'
}')")
assert_code "D5a 停用后可编辑" "0" "$body"

fsm_enable "$FSM_ID5"
body=$(fsm_detail "$FSM_ID5")
assert_field "D5b 重新启用后 enabled=true" '.data.enabled' "true" "$body"
assert_field "D5b 编辑生效 state 4个" '.data.config.states | length' "4" "$body"
fsm_disable "$FSM_ID5"

# D6: 软删除的 name 不可复用 (43001)
body=$(post "/fsm-configs/check-name" "{\"name\":\"$FSM_NAME2\"}")
assert_field "D6a 已删 name check-name=false" '.data.available' "false" "$body"

body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"$FSM_NAME2"'",
  "display_name":"Reuse Attempt",
  "initial_state":"a",
  "states":[{"name":"a"}],
  "transitions":[]
}')")
assert_code "D6b 复用已删 name -> 43001" "43001" "$body"

# D7: Version increments on each operation
body=$(fsm_detail "$FSM_ID5")
FSM_V_BEFORE=$(echo "$body" | jq -r '.data.version' | tr -d '\r')
fsm_enable "$FSM_ID5"
body=$(fsm_detail "$FSM_ID5")
FSM_V_AFTER=$(echo "$body" | jq -r '.data.version' | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$FSM_V_AFTER" -gt "$FSM_V_BEFORE" ] 2>/dev/null; then
  echo "  [PASS] D7 toggle 后 version 递增 ($FSM_V_BEFORE -> $FSM_V_AFTER)"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] D7 toggle 后 version 未递增 ($FSM_V_BEFORE -> $FSM_V_AFTER)"
  FAIL=$((FAIL + 1))
fi
fsm_disable "$FSM_ID5"

# =============================================================================
subsection "7-E: 攻击性测试"
# =============================================================================

# E1: SQL 注入 in name
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"wolf_fsm'"'"'; DROP TABLE fsm_configs; --",
  "display_name":"sql inject",
  "initial_state":"a",
  "states":[{"name":"a"}],
  "transitions":[]
}')")
assert_code_in "E1 SQL 注入 name -> 格式拒绝" "43002 40000" "$body"

# E2: XSS in display_name — should create or reject, never 500
body=$(fsm_atk "${P}xss_test" "$(printf '%s' '{
  "name":"'"${P}xss_test"'",
  "display_name":"<script>alert(1)</script>",
  "initial_state":"a",
  "states":[{"name":"a"}],
  "transitions":[]
}')")
FSM_XSS_CODE=$(echo "$body" | jq -r '.code // empty' | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$FSM_XSS_CODE" = "0" ] || [ "$FSM_XSS_CODE" = "40000" ]; then
  echo "  [PASS] E2 XSS display_name 不报 500 (code=$FSM_XSS_CODE)"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] E2 XSS display_name 返回异常 code=$FSM_XSS_CODE"
  FAIL=$((FAIL + 1))
fi

# E3: Unicode/emoji in state names
body=$(fsm_atk "${P}unicode_st" "$(printf '%s' '{
  "name":"'"${P}unicode_st"'",
  "display_name":"unicode test",
  "initial_state":"idle",
  "states":[{"name":"idle"},{"name":"\u7a7a\u95f2"}],
  "transitions":[]
}')")
FSM_UNI_CODE=$(echo "$body" | jq -r '.code // empty' | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$FSM_UNI_CODE" = "0" ] || [ "$FSM_UNI_CODE" = "40000" ] || [ "$FSM_UNI_CODE" = "43005" ]; then
  echo "  [PASS] E3a 中文 state name 不报 500 (code=$FSM_UNI_CODE)"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] E3a 中文 state name 返回异常 code=$FSM_UNI_CODE"
  FAIL=$((FAIL + 1))
fi

# E3b: Emoji in state name
body=$(fsm_atk "${P}emoji_st" "$(printf '%s' '{
  "name":"'"${P}emoji_st"'",
  "display_name":"emoji test",
  "initial_state":"idle",
  "states":[{"name":"idle"},{"name":"\ud83d\udc3a"}],
  "transitions":[]
}')")
FSM_EMO_CODE=$(echo "$body" | jq -r '.code // empty' | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$FSM_EMO_CODE" != "" ]; then
  echo "  [PASS] E3b emoji state name 不报 500 (code=$FSM_EMO_CODE)"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] E3b emoji state name 无响应"
  FAIL=$((FAIL + 1))
fi

# E4: null states
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}null_states"'",
  "display_name":"x",
  "initial_state":"a",
  "states":null,
  "transitions":[]
}')")
assert_code_in "E4a states=null -> 43004 or 40000" "43004 40000" "$body"

# E5: null transitions (should be treated as empty array => success)
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}null_trans"'",
  "display_name":"x",
  "initial_state":"a",
  "states":[{"name":"a"}],
  "transitions":null
}')")
FSM_NULL_TRANS_CODE=$(echo "$body" | jq -r '.code // empty' | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$FSM_NULL_TRANS_CODE" = "0" ] || [ "$FSM_NULL_TRANS_CODE" = "40000" ]; then
  echo "  [PASS] E5 transitions=null 不报 500 (code=$FSM_NULL_TRANS_CODE)"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] E5 transitions=null 返回异常 code=$FSM_NULL_TRANS_CODE"
  FAIL=$((FAIL + 1))
fi
FSM_NT_ID=$(echo "$body" | jq -r '.data.id // empty' | tr -d '\r')
[ -n "$FSM_NT_ID" ] && [ "$FSM_NT_ID" != "null" ] && fsm_rm "$FSM_NT_ID"

# E6: Missing initial_state field entirely
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}no_initial"'",
  "display_name":"x",
  "states":[{"name":"a"}],
  "transitions":[]
}')")
# initial_state="" not in states => 43006, or 40000 if handler catches
assert_code_in "E6 缺少 initial_state -> 43006 or 40000" "43006 43004 40000" "$body"

# E7: Missing states field entirely
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}no_states_field"'",
  "display_name":"x",
  "initial_state":"a"
}')")
assert_code_in "E7 缺少 states 字段 -> 43004 or 40000" "43004 40000" "$body"

# E8: Duplicate transitions same from/to different priority (should be ALLOWED)
body=$(fsm_atk "${P}dup_trans" "$(printf '%s' '{
  "name":"'"${P}dup_trans"'",
  "display_name":"dup trans",
  "initial_state":"a",
  "states":[{"name":"a"},{"name":"b"}],
  "transitions":[
    {"from":"a","to":"b","priority":1,"condition":{"key":"x","op":">","value":10}},
    {"from":"a","to":"b","priority":2,"condition":{"key":"x","op":">","value":20}}
  ]
}')")
assert_code "E8 相同 from/to 不同 priority 应允许" "0" "$body"

# E9: State name with spaces
body=$(fsm_atk "${P}space_state" "$(printf '%s' '{
  "name":"'"${P}space_state"'",
  "display_name":"space test",
  "initial_state":"idle state",
  "states":[{"name":"idle state"},{"name":"run state"}],
  "transitions":[]
}')")
FSM_SPACE_CODE=$(echo "$body" | jq -r '.code // empty' | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$FSM_SPACE_CODE" != "" ]; then
  echo "  [PASS] E9 state name 含空格不报 500 (code=$FSM_SPACE_CODE)"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] E9 state name 含空格无响应"
  FAIL=$((FAIL + 1))
fi

# E10: State name with dots and slashes
body=$(fsm_atk "${P}dot_state" "$(printf '%s' '{
  "name":"'"${P}dot_state"'",
  "display_name":"dot test",
  "initial_state":"state.v1",
  "states":[{"name":"state.v1"},{"name":"state/v2"}],
  "transitions":[]
}')")
FSM_DOT_CODE=$(echo "$body" | jq -r '.code // empty' | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$FSM_DOT_CODE" != "" ]; then
  echo "  [PASS] E10 state name 含点/斜杠不报 500 (code=$FSM_DOT_CODE)"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] E10 state name 含点/斜杠无响应"
  FAIL=$((FAIL + 1))
fi

# E11: 100+ states exceeding max_states=50
FSM_MANY_STATES='['
for i in $(seq 1 100); do
  [ $i -gt 1 ] && FSM_MANY_STATES+=','
  FSM_MANY_STATES+="{\"name\":\"s$i\"}"
done
FSM_MANY_STATES+=']'
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}many_states"'",
  "display_name":"many",
  "initial_state":"s1",
  "states":'"$FSM_MANY_STATES"',
  "transitions":[]
}')")
assert_code "E11 100 states > max 50 -> 43004" "43004" "$body"

# E12: Exactly 50 states — at limit, should succeed
FSM_50_STATES='['
for i in $(seq 1 50); do
  [ $i -gt 1 ] && FSM_50_STATES+=','
  FSM_50_STATES+="{\"name\":\"s$i\"}"
done
FSM_50_STATES+=']'
body=$(fsm_atk "${P}fifty_states" "$(printf '%s' '{
  "name":"'"${P}fifty_states"'",
  "display_name":"fifty",
  "initial_state":"s1",
  "states":'"$FSM_50_STATES"',
  "transitions":[]
}')")
assert_code "E12 恰好 50 states -> 成功" "0" "$body"

# E13: Many transitions (49 valid — should succeed)
FSM_MANY_TRANS='['
for i in $(seq 1 49); do
  [ $i -gt 1 ] && FSM_MANY_TRANS+=','
  FSM_MANY_TRANS+="{\"from\":\"s1\",\"to\":\"s2\",\"priority\":$i,\"condition\":{}}"
done
FSM_MANY_TRANS+=']'
body=$(fsm_atk "${P}many_trans" "$(printf '%s' '{
  "name":"'"${P}many_trans"'",
  "display_name":"many trans",
  "initial_state":"s1",
  "states":[{"name":"s1"},{"name":"s2"}],
  "transitions":'"$FSM_MANY_TRANS"'
}')")
assert_code "E13 49 transitions 不超限 -> 成功" "0" "$body"

# E14: Non-object condition types
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}cond_str"'",
  "display_name":"x",
  "initial_state":"a",
  "states":[{"name":"a"},{"name":"b"}],
  "transitions":[{"from":"a","to":"b","priority":0,"condition":"string"}]
}')")
assert_code_in "E14a condition=string -> 400/43008" "40000 43008" "$body"

body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}cond_num"'",
  "display_name":"x",
  "initial_state":"a",
  "states":[{"name":"a"},{"name":"b"}],
  "transitions":[{"from":"a","to":"b","priority":0,"condition":123}]
}')")
assert_code_in "E14b condition=number -> 400/43008" "40000 43008" "$body"

body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}cond_arr"'",
  "display_name":"x",
  "initial_state":"a",
  "states":[{"name":"a"},{"name":"b"}],
  "transitions":[{"from":"a","to":"b","priority":0,"condition":[1,2]}]
}')")
assert_code_in "E14c condition=array -> 400/43008" "40000 43008" "$body"

# E15: empty display_name
body=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}no_display"'",
  "display_name":"",
  "initial_state":"a",
  "states":[{"name":"a"}],
  "transitions":[]
}')")
assert_code "E15 空 display_name -> 40000" "40000" "$body"

# E16: Self-transition (from == to) — should be allowed
body=$(fsm_atk "${P}self_trans" "$(printf '%s' '{
  "name":"'"${P}self_trans"'",
  "display_name":"self",
  "initial_state":"a",
  "states":[{"name":"a"}],
  "transitions":[{"from":"a","to":"a","priority":0,"condition":{"key":"x","op":"==","value":1}}]
}')")
assert_code "E16 自转换 from==to 应允许" "0" "$body"

# E17: Priority=0 (edge case, should be valid)
body=$(fsm_atk "${P}pri_zero" "$(printf '%s' '{
  "name":"'"${P}pri_zero"'",
  "display_name":"zero pri",
  "initial_state":"a",
  "states":[{"name":"a"},{"name":"b"}],
  "transitions":[{"from":"a","to":"b","priority":0,"condition":{}}]
}')")
assert_code "E17 priority=0 应合法" "0" "$body"

# E18: Very large priority
body=$(fsm_atk "${P}big_pri" "$(printf '%s' '{
  "name":"'"${P}big_pri"'",
  "display_name":"big pri",
  "initial_state":"a",
  "states":[{"name":"a"},{"name":"b"}],
  "transitions":[{"from":"a","to":"b","priority":999999,"condition":{}}]
}')")
assert_code "E18 超大 priority 应合法" "0" "$body"

# ---- 清理剩余测试数据 ----
fsm_rm "$FSM_ID1" 2>/dev/null
fsm_rm "$FSM_ID5" 2>/dev/null
