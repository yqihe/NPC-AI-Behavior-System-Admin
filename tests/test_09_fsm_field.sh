#!/bin/bash
# =============================================================================
# test_09_fsm_field.sh — FSM BB Key ↔ Field 跨模块集成（全新覆盖）
#
# 重点攻击：
#   - FSM 条件中的 BB Key 是否正确写入 field_refs(ref_type='fsm')
#   - 字段 expose_bb 关闭守卫 (40008) — 被 FSM 引用时禁止关闭
#   - FSM update 时新旧 BB Key diff 同步
#   - FSM delete 时清理 BB Key refs
#   - 多 FSM 引用同一 BB Key 的 ref_count 叠加
#   - 运行时 Key（非字段来源）不应写入 field_refs
# =============================================================================

section "Part 9: FSM BB Key ↔ Field 跨模块集成 (prefix=$P)"

# =============================================================================
# 9-A: 准备字段池（expose_bb=true 的字段）
# =============================================================================
subsection "9-A: 准备 BB Key 字段池"

# bb_hp：暴露为 BB Key 的 integer 字段
R=$(post "/fields/create" "{\"name\":\"${P}bb_hp\",\"label\":\"BB_HP\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"HP BB\",\"expose_bb\":true,\"constraints\":{\"min\":0,\"max\":100}}}")
assert_code "9A.1 创建 bb_hp (expose_bb=true)" "0" "$R"
BB_HP=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
fld_enable "$BB_HP"

# bb_distance：float BB Key
R=$(post "/fields/create" "{\"name\":\"${P}bb_distance\",\"label\":\"距离\",\"type\":\"float\",\"category\":\"perception\",\"properties\":{\"description\":\"\",\"expose_bb\":true,\"constraints\":{\"min\":0.0,\"max\":1000.0}}}")
assert_code "9A.2 创建 bb_distance" "0" "$R"
BB_DIST=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
fld_enable "$BB_DIST"

# bb_energy：expose_bb=true
R=$(post "/fields/create" "{\"name\":\"${P}bb_energy\",\"label\":\"能量\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":true,\"constraints\":{\"min\":0,\"max\":100}}}")
assert_code "9A.3 创建 bb_energy" "0" "$R"
BB_ENERGY=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
fld_enable "$BB_ENERGY"

# no_bb：普通字段 expose_bb=false，不应被 FSM 追踪
R=$(post "/fields/create" "{\"name\":\"${P}no_bb\",\"label\":\"非BB字段\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false}}")
assert_code "9A.4 创建 no_bb (expose_bb=false)" "0" "$R"
NO_BB=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
fld_enable "$NO_BB"

# 确认初始 ref_count=0
R=$(fld_detail "$BB_HP")
assert_field "9A.5 bb_hp 初始 ref_count=0" ".data.ref_count" "0" "$R"

# =============================================================================
# 9-B: FSM 创建 → 写入 BB Key ref
# =============================================================================
subsection "9-B: FSM 创建写入 field_refs"

# B1: 创建 FSM 引用 bb_hp（key）
R=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}fsm_b1"'",
  "display_name":"FSM B1",
  "initial_state":"idle",
  "states":[{"name":"idle"},{"name":"flee"}],
  "transitions":[
    {"from":"idle","to":"flee","priority":1,"condition":{"key":"'"${P}bb_hp"'","op":"<","value":20}}
  ]
}')")
assert_code "9B.1 创建 FSM 引用 bb_hp" "0" "$R"
FSM_B1=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

# B2: bb_hp ref_count 应为 1
R=$(fld_detail "$BB_HP")
assert_field "9B.2 bb_hp ref_count=1（被 FSM 引用）" ".data.ref_count" "1" "$R"

# B3: 通过 /fields/references 验证 FSM 出现在引用列表
R=$(post "/fields/references" "{\"id\":${BB_HP}}")
assert_code  "9B.3 references 成功" "0" "$R"
# FSM 可能出现在 fsms 或 fsm_configs 字段中
TOTAL=$((TOTAL + 1))
FSM_LIST=$(echo "$R" | jq -r '.data | to_entries[] | .value | if type=="array" then .[] | select(.id=='"$FSM_B1"') | .id else empty end' 2>/dev/null | head -1 | tr -d '\r')
if [ "$FSM_LIST" = "$FSM_B1" ]; then
  echo "  [PASS] 9B.3b FSM 出现在 bb_hp 引用列表"
  PASS=$((PASS + 1))
else
  echo "  [BUG ] 9B.3b FSM 不在 bb_hp 引用列表中"
  FAIL=$((FAIL + 1))
  BUGS+=("9B.3b: FSM BB Key 引用未在 /fields/references 返回")
fi

# B4: 不暴露 BB 的字段不被追踪 — no_bb ref_count 仍为 0
R=$(fld_detail "$NO_BB")
assert_field "9B.4 no_bb ref_count=0（未被追踪）" ".data.ref_count" "0" "$R"

# =============================================================================
# 9-C: FSM update → BB Key diff 同步
# =============================================================================
subsection "9-C: FSM update BB Key diff"

# FSM_B1 当前引用 bb_hp。改为引用 bb_distance，bb_hp ref_count 应减，bb_distance ref_count 应增
V=$(fsm_version "$FSM_B1")
R=$(post "/fsm-configs/update" "$(printf '%s' '{
  "id":'"$FSM_B1"',
  "display_name":"FSM B1 v2",
  "initial_state":"idle",
  "states":[{"name":"idle"},{"name":"flee"}],
  "transitions":[
    {"from":"idle","to":"flee","priority":1,"condition":{"key":"'"${P}bb_distance"'","op":">","value":100}}
  ],
  "version":'"$V"'
}')")
assert_code "9C.1 FSM update 切换 BB Key" "0" "$R"

R=$(fld_detail "$BB_HP")
assert_field "9C.2 bb_hp ref_count 回退到 0" ".data.ref_count" "0" "$R"

R=$(fld_detail "$BB_DIST")
assert_field "9C.3 bb_distance ref_count=1（新增）" ".data.ref_count" "1" "$R"

# C4: update 同时使用两个 BB Key
V=$(fsm_version "$FSM_B1")
R=$(post "/fsm-configs/update" "$(printf '%s' '{
  "id":'"$FSM_B1"',
  "display_name":"FSM B1 v3",
  "initial_state":"idle",
  "states":[{"name":"idle"},{"name":"flee"}],
  "transitions":[
    {"from":"idle","to":"flee","priority":1,"condition":{"and":[{"key":"'"${P}bb_hp"'","op":"<","value":20},{"key":"'"${P}bb_distance"'","op":">","value":100}]}}
  ],
  "version":'"$V"'
}')")
assert_code "9C.4 FSM update 使用两个 BB Key" "0" "$R"

R=$(fld_detail "$BB_HP")
assert_field "9C.5 bb_hp ref_count=1" ".data.ref_count" "1" "$R"
R=$(fld_detail "$BB_DIST")
assert_field "9C.6 bb_distance ref_count=1" ".data.ref_count" "1" "$R"

# =============================================================================
# 9-D: expose_bb 关闭守卫 (40008)
# =============================================================================
subsection "9-D: expose_bb 关闭守卫 40008"

# 此时 bb_hp 被 FSM_B1 引用，尝试停用字段并关闭 expose_bb → 40008
fld_disable "$BB_HP"
V=$(fld_version "$BB_HP")
R=$(post "/fields/update" "{\"id\":${BB_HP},\"label\":\"BB_HP\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"HP BB\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}},\"version\":${V}}")
assert_code "9D.1 关闭 expose_bb 被 FSM 引用 → 40008" "40008" "$R"

# D2: 但 expose_bb 保持 true 的其他编辑应允许（如 label）
V=$(fld_version "$BB_HP")
R=$(post "/fields/update" "{\"id\":${BB_HP},\"label\":\"BB_HP 改名\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"HP BB\",\"expose_bb\":true,\"constraints\":{\"min\":0,\"max\":100}},\"version\":${V}}")
assert_code "9D.2 保持 expose_bb=true 的编辑应允许" "0" "$R"

# D3: 尝试改类型 → 40006（被引用）
V=$(fld_version "$BB_HP")
R=$(post "/fields/update" "{\"id\":${BB_HP},\"label\":\"BB_HP\",\"type\":\"float\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":true,\"constraints\":{\"min\":0.0,\"max\":100.0}},\"version\":${V}}")
assert_code "9D.3 被 FSM 引用改类型 → 40006" "40006" "$R"

# D4: 尝试收紧约束 → 40007
V=$(fld_version "$BB_HP")
R=$(post "/fields/update" "{\"id\":${BB_HP},\"label\":\"BB_HP\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":true,\"constraints\":{\"min\":10,\"max\":100}},\"version\":${V}}")
assert_code "9D.4 被 FSM 引用收紧约束 → 40007" "40007" "$R"

# D5: 尝试删除 → 40005
R=$(post "/fields/delete" "{\"id\":${BB_HP}}")
assert_code "9D.5 被 FSM 引用删除字段 → 40005" "40005" "$R"

fld_enable "$BB_HP"

# =============================================================================
# 9-E: 多 FSM 引用同一 BB Key
# =============================================================================
subsection "9-E: 多 FSM 引用叠加 ref_count"

# 创建第二个 FSM 也引用 bb_hp
R=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}fsm_e2"'",
  "display_name":"FSM E2",
  "initial_state":"s1",
  "states":[{"name":"s1"},{"name":"s2"}],
  "transitions":[
    {"from":"s1","to":"s2","priority":1,"condition":{"key":"'"${P}bb_hp"'","op":">","value":50}}
  ]
}')")
assert_code "9E.1 创建第二个 FSM 引用 bb_hp" "0" "$R"
FSM_E2=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

R=$(fld_detail "$BB_HP")
assert_field "9E.2 bb_hp ref_count=2（两个 FSM）" ".data.ref_count" "2" "$R"

# 删除一个 FSM，ref_count 减到 1
fsm_rm "$FSM_E2"
R=$(fld_detail "$BB_HP")
assert_field "9E.3 删除一个 FSM 后 bb_hp ref_count=1" ".data.ref_count" "1" "$R"

# =============================================================================
# 9-F: 运行时 Key（非字段来源）
# =============================================================================
subsection "9-F: 运行时 Key 不追踪"

# 创建 FSM 使用一个不存在的字段名作为 key → 应该成功，不写 field_refs
R=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}fsm_runtime"'",
  "display_name":"FSM Runtime Key",
  "initial_state":"a",
  "states":[{"name":"a"},{"name":"b"}],
  "transitions":[
    {"from":"a","to":"b","priority":1,"condition":{"key":"some_runtime_key_xyz","op":"==","value":true}}
  ]
}')")
assert_code "9F.1 FSM 使用运行时 Key 创建成功" "0" "$R"
FSM_RT=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

# 没有可比对的字段，但至少 bb_hp ref_count 未被污染
R=$(fld_detail "$BB_HP")
assert_field "9F.2 bb_hp ref_count 未被污染（仍为 1）" ".data.ref_count" "1" "$R"

fsm_rm "$FSM_RT"

# =============================================================================
# 9-G: ref_key 追踪
# =============================================================================
subsection "9-G: ref_key BB Key 追踪"

# 创建 FSM 使用 ref_key 引用 bb_energy
R=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}fsm_refkey"'",
  "display_name":"FSM ref_key",
  "initial_state":"a",
  "states":[{"name":"a"},{"name":"b"}],
  "transitions":[
    {"from":"a","to":"b","priority":1,"condition":{"key":"'"${P}bb_hp"'","op":">","ref_key":"'"${P}bb_energy"'"}}
  ]
}')")
assert_code "9G.1 FSM 使用 ref_key 创建" "0" "$R"
FSM_RK=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

# bb_energy 应该被追踪（如果 ref_key 也被扫描）
R=$(fld_detail "$BB_ENERGY")
TOTAL=$((TOTAL + 1))
ENERGY_RC=$(echo "$R" | jq -r '.data.ref_count' | tr -d '\r')
if [ "$ENERGY_RC" = "1" ]; then
  echo "  [PASS] 9G.2 bb_energy 被 ref_key 追踪（ref_count=1）"
  PASS=$((PASS + 1))
else
  echo "  [BUG ] 9G.2 bb_energy ref_key 未被追踪（ref_count=$ENERGY_RC）"
  FAIL=$((FAIL + 1))
  BUGS+=("9G.2: FSM 的 ref_key 引用可能未写入 field_refs")
fi

# 清理
fsm_rm "$FSM_RK"

# =============================================================================
# 9-H: 复合条件多 Key 追踪
# =============================================================================
subsection "9-H: 复合条件 AND/OR 多 Key 追踪"

# 创建 FSM，OR 条件中包含三个不同 BB Key
R=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}fsm_composite"'",
  "display_name":"复合条件",
  "initial_state":"a",
  "states":[{"name":"a"},{"name":"b"}],
  "transitions":[
    {"from":"a","to":"b","priority":1,"condition":{"or":[
      {"key":"'"${P}bb_hp"'","op":"<","value":10},
      {"key":"'"${P}bb_distance"'","op":">","value":500},
      {"key":"'"${P}bb_energy"'","op":"==","value":0}
    ]}}
  ]
}')")
assert_code "9H.1 创建复合条件 FSM" "0" "$R"
FSM_CP=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

R=$(fld_detail "$BB_HP")
HP_RC=$(echo "$R" | jq -r '.data.ref_count' | tr -d '\r')
R=$(fld_detail "$BB_DIST")
DIST_RC=$(echo "$R" | jq -r '.data.ref_count' | tr -d '\r')
R=$(fld_detail "$BB_ENERGY")
EN_RC=$(echo "$R" | jq -r '.data.ref_count' | tr -d '\r')

TOTAL=$((TOTAL + 3))
# bb_hp 原来 ref=1（FSM_B1），+1 = 2
# bb_distance 原来 ref=1（FSM_B1），+1 = 2
# bb_energy 原来 ref=0，+1 = 1
if [ "$HP_RC" = "2" ]; then
  echo "  [PASS] 9H.2 bb_hp ref_count=2（FSM_B1+FSM_CP）"
  PASS=$((PASS+1))
else
  echo "  [FAIL] 9H.2 bb_hp 期望 2 实际 $HP_RC"
  FAIL=$((FAIL+1))
fi
if [ "$DIST_RC" = "2" ]; then
  echo "  [PASS] 9H.3 bb_distance ref_count=2（FSM_B1+FSM_CP）"
  PASS=$((PASS+1))
else
  echo "  [FAIL] 9H.3 bb_distance 期望 2 实际 $DIST_RC"
  FAIL=$((FAIL+1))
fi
if [ "$EN_RC" = "1" ]; then
  echo "  [PASS] 9H.4 bb_energy ref_count=1（新增 FSM_CP）"
  PASS=$((PASS+1))
else
  echo "  [FAIL] 9H.4 bb_energy 期望 1 实际 $EN_RC"
  FAIL=$((FAIL+1))
fi

# =============================================================================
# 9-I: 深度嵌套条件的 Key 追踪
# =============================================================================
subsection "9-I: 深度嵌套 BB Key 追踪"

# 7 层 AND 嵌套，最底层 key
DEEP_COND='{"key":"'"${P}bb_hp"'","op":"<","value":5}'
for i in 1 2 3 4 5; do
  DEEP_COND="{\"and\":[$DEEP_COND]}"
done

R=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}fsm_deep"'",
  "display_name":"深度嵌套",
  "initial_state":"a",
  "states":[{"name":"a"},{"name":"b"}],
  "transitions":[{"from":"a","to":"b","priority":1,"condition":'"$DEEP_COND"'}]
}')")
assert_code "9I.1 深度嵌套 FSM 创建" "0" "$R"
FSM_DEEP=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

R=$(fld_detail "$BB_HP")
HP_RC=$(echo "$R" | jq -r '.data.ref_count' | tr -d '\r')
TOTAL=$((TOTAL + 1))
# 原来 ref=2，+1 = 3
if [ "$HP_RC" = "3" ]; then
  echo "  [PASS] 9I.2 深度嵌套 Key 也被追踪 (ref_count=3)"
  PASS=$((PASS+1))
else
  echo "  [BUG ] 9I.2 深度嵌套 Key 未被追踪（ref=$HP_RC，期望 3）"
  FAIL=$((FAIL+1))
  BUGS+=("9I.2: 深度嵌套条件中的 BB Key 可能未递归扫描")
fi

fsm_rm "$FSM_DEEP"
fsm_rm "$FSM_CP"

# =============================================================================
# 9-J: 无条件 FSM（空 condition）
# =============================================================================
subsection "9-J: 无条件 FSM 不写 refs"

R=$(fld_detail "$BB_HP")
BEFORE=$(echo "$R" | jq -r '.data.ref_count' | tr -d '\r')

R=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}fsm_empty"'",
  "display_name":"无条件",
  "initial_state":"a",
  "states":[{"name":"a"},{"name":"b"}],
  "transitions":[{"from":"a","to":"b","priority":1,"condition":{}}]
}')")
assert_code "9J.1 无条件 FSM 创建" "0" "$R"
FSM_EMPTY=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

R=$(fld_detail "$BB_HP")
AFTER=$(echo "$R" | jq -r '.data.ref_count' | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$BEFORE" = "$AFTER" ]; then
  echo "  [PASS] 9J.2 无条件 FSM 未新增 ref ($BEFORE==$AFTER)"
  PASS=$((PASS+1))
else
  echo "  [FAIL] 9J.2 空条件却新增了 ref ($BEFORE→$AFTER)"
  FAIL=$((FAIL+1))
fi

fsm_rm "$FSM_EMPTY"

# =============================================================================
# 9-K: FSM delete 清理 refs
# =============================================================================
subsection "9-K: FSM delete 清理 refs"

# 当前 FSM_B1 仍在引用 bb_hp + bb_distance
R=$(fld_detail "$BB_HP")
BEFORE_HP=$(echo "$R" | jq -r '.data.ref_count' | tr -d '\r')
R=$(fld_detail "$BB_DIST")
BEFORE_DIST=$(echo "$R" | jq -r '.data.ref_count' | tr -d '\r')

fsm_rm "$FSM_B1"

R=$(fld_detail "$BB_HP")
AFTER_HP=$(echo "$R" | jq -r '.data.ref_count' | tr -d '\r')
R=$(fld_detail "$BB_DIST")
AFTER_DIST=$(echo "$R" | jq -r '.data.ref_count' | tr -d '\r')

TOTAL=$((TOTAL + 2))
if [ "$AFTER_HP" = "$((BEFORE_HP - 1))" ]; then
  echo "  [PASS] 9K.1 FSM 删除后 bb_hp ref_count 减 1 ($BEFORE_HP→$AFTER_HP)"
  PASS=$((PASS+1))
else
  echo "  [FAIL] 9K.1 ref_count 未正确回退 ($BEFORE_HP→$AFTER_HP)"
  FAIL=$((FAIL+1))
fi
if [ "$AFTER_DIST" = "$((BEFORE_DIST - 1))" ]; then
  echo "  [PASS] 9K.2 bb_distance ref_count 减 1 ($BEFORE_DIST→$AFTER_DIST)"
  PASS=$((PASS+1))
else
  echo "  [FAIL] 9K.2 bb_distance ref_count 未正确回退"
  FAIL=$((FAIL+1))
fi

# =============================================================================
# 9-L: 删除所有 FSM 后字段 expose_bb 可关闭
# =============================================================================
subsection "9-L: 清理所有 FSM 后 expose_bb 可关闭"

# 确认 bb_hp 无引用
R=$(fld_detail "$BB_HP")
FINAL_HP=$(echo "$R" | jq -r '.data.ref_count' | tr -d '\r')

if [ "$FINAL_HP" = "0" ]; then
  fld_disable "$BB_HP"
  V=$(fld_version "$BB_HP")
  R=$(post "/fields/update" "{\"id\":${BB_HP},\"label\":\"BB_HP\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"HP BB\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}},\"version\":${V}}")
  assert_code "9L.1 无 FSM 引用后 expose_bb=false 成功" "0" "$R"

  # 验证 expose_bb 已关闭
  R=$(fld_detail "$BB_HP")
  assert_field "9L.2 expose_bb=false 生效" ".data.properties.expose_bb" "false" "$R"
else
  echo "  [SKIP] 9L.1 bb_hp 仍有 ref_count=$FINAL_HP，跳过关闭测试"
fi

# =============================================================================
# 9-M: 缓存一致性：FSM 操作后字段立即反映
# =============================================================================
subsection "9-M: 缓存一致性"

# 启用一个新字段并创建 FSM
R=$(post "/fields/create" "{\"name\":\"${P}cache_bb\",\"label\":\"缓存测试BB\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":true}}")
CACHE_BB=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
fld_enable "$CACHE_BB"

# 先读一次进入缓存
R=$(fld_detail "$CACHE_BB")
assert_field "9M.0 初始 ref_count=0" ".data.ref_count" "0" "$R"

# 立即创建 FSM 引用
R=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}fsm_cache"'",
  "display_name":"缓存测试FSM",
  "initial_state":"a",
  "states":[{"name":"a"},{"name":"b"}],
  "transitions":[{"from":"a","to":"b","priority":1,"condition":{"key":"'"${P}cache_bb"'","op":">","value":0}}]
}')")
FSM_CACHE=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

# 立即再读字段详情
R=$(fld_detail "$CACHE_BB")
assert_field "9M.1 FSM 创建后立即读 ref_count=1（缓存已失效）" ".data.ref_count" "1" "$R"

# 连续读两次验证稳定
R=$(fld_detail "$CACHE_BB")
assert_field "9M.2 第二次读仍为 1" ".data.ref_count" "1" "$R"

# FSM 立即删除后
fsm_rm "$FSM_CACHE"
R=$(fld_detail "$CACHE_BB")
assert_field "9M.3 FSM 删除后立即读 ref_count=0" ".data.ref_count" "0" "$R"

fld_rm "$CACHE_BB"

# =============================================================================
# 9-N: 攻击：快速创建删除 FSM，ref_count 应最终一致
# =============================================================================
subsection "9-N: 攻击 — 快速循环 ref_count 稳定"

R=$(fld_detail "$BB_ENERGY")
INIT_EN=$(echo "$R" | jq -r '.data.ref_count' | tr -d '\r')

# 创建 5 个 FSM 引用 bb_energy，再全部删除
FSM_BATCH=()
for i in 1 2 3 4 5; do
  BATCH_NAME="${P}fsm_batch_${i}"
  BATCH_BODY="{\"name\":\"${BATCH_NAME}\",\"display_name\":\"batch${i}\",\"initial_state\":\"a\",\"states\":[{\"name\":\"a\"},{\"name\":\"b\"}],\"transitions\":[{\"from\":\"a\",\"to\":\"b\",\"priority\":1,\"condition\":{\"key\":\"${P}bb_energy\",\"op\":\"==\",\"value\":${i}}}]}"
  R=$(post "/fsm-configs/create" "$BATCH_BODY")
  FID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
  FSM_BATCH+=("$FID")
done

R=$(fld_detail "$BB_ENERGY")
MID=$(echo "$R" | jq -r '.data.ref_count' | tr -d '\r')
TOTAL=$((TOTAL + 1))
EXPECTED=$((INIT_EN + 5))
if [ "$MID" = "$EXPECTED" ]; then
  echo "  [PASS] 9N.1 批量创建后 ref_count=$MID"
  PASS=$((PASS+1))
else
  echo "  [FAIL] 9N.1 期望 $EXPECTED 实际 $MID"
  FAIL=$((FAIL+1))
fi

# 全部删除
for fid in "${FSM_BATCH[@]}"; do
  fsm_rm "$fid"
done

R=$(fld_detail "$BB_ENERGY")
FINAL=$(echo "$R" | jq -r '.data.ref_count' | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$FINAL" = "$INIT_EN" ]; then
  echo "  [PASS] 9N.2 全部删除后 ref_count 回退到 $INIT_EN"
  PASS=$((PASS+1))
else
  echo "  [FAIL] 9N.2 期望 $INIT_EN 实际 $FINAL（refs 泄漏）"
  FAIL=$((FAIL+1))
  BUGS+=("9N.2: FSM 删除后 field_refs 未完全清理，ref_count 泄漏")
fi

# =============================================================================
# 9-O: 攻击：FSM update 变体（部分 Key 重叠）
# =============================================================================
subsection "9-O: 攻击 — update 部分 Key 重叠"

# 创建 FSM 使用 {bb_hp, bb_distance}
R=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}fsm_overlap"'",
  "display_name":"重叠",
  "initial_state":"a",
  "states":[{"name":"a"},{"name":"b"}],
  "transitions":[{"from":"a","to":"b","priority":1,"condition":{"and":[
    {"key":"'"${P}bb_hp"'","op":"<","value":10},
    {"key":"'"${P}bb_distance"'","op":">","value":10}
  ]}}]
}')")
FSM_OL=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

R=$(fld_detail "$BB_HP")
HP_PRE=$(echo "$R" | jq -r '.data.ref_count' | tr -d '\r')
R=$(fld_detail "$BB_DIST")
DIST_PRE=$(echo "$R" | jq -r '.data.ref_count' | tr -d '\r')

# update 改为 {bb_hp, bb_energy} — bb_hp 保留，bb_distance 移除，bb_energy 新增
V=$(fsm_version "$FSM_OL")
R=$(post "/fsm-configs/update" "$(printf '%s' '{
  "id":'"$FSM_OL"',
  "display_name":"重叠v2",
  "initial_state":"a",
  "states":[{"name":"a"},{"name":"b"}],
  "transitions":[{"from":"a","to":"b","priority":1,"condition":{"and":[
    {"key":"'"${P}bb_hp"'","op":">","value":20},
    {"key":"'"${P}bb_energy"'","op":"<","value":50}
  ]}}],
  "version":'"$V"'
}')")
assert_code "9O.1 update 部分重叠成功" "0" "$R"

R=$(fld_detail "$BB_HP")
HP_POST=$(echo "$R" | jq -r '.data.ref_count' | tr -d '\r')
R=$(fld_detail "$BB_DIST")
DIST_POST=$(echo "$R" | jq -r '.data.ref_count' | tr -d '\r')

TOTAL=$((TOTAL + 2))
# bb_hp 应保持不变（仍在 FSM_OL 里）
if [ "$HP_POST" = "$HP_PRE" ]; then
  echo "  [PASS] 9O.2 bb_hp ref_count 保持 $HP_POST（未变）"
  PASS=$((PASS+1))
else
  echo "  [BUG ] 9O.2 bb_hp ref_count 错误 ($HP_PRE→$HP_POST)"
  FAIL=$((FAIL+1))
  BUGS+=("9O.2: update BB Key 保留时 ref_count 波动，可能先删后加")
fi
# bb_distance 应该减 1
if [ "$DIST_POST" = "$((DIST_PRE - 1))" ]; then
  echo "  [PASS] 9O.3 bb_distance ref_count 减 1 ($DIST_PRE→$DIST_POST)"
  PASS=$((PASS+1))
else
  echo "  [FAIL] 9O.3 bb_distance 期望 $((DIST_PRE - 1)) 实际 $DIST_POST"
  FAIL=$((FAIL+1))
fi

fsm_rm "$FSM_OL"

# =============================================================================
# 9-P: 攻击：同一条件中重复使用同一 Key
# =============================================================================
subsection "9-P: 攻击 — 重复 Key 去重"

R=$(fld_detail "$BB_HP")
DUP_PRE=$(echo "$R" | jq -r '.data.ref_count' | tr -d '\r')

R=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}fsm_dup_key"'",
  "display_name":"重复Key",
  "initial_state":"a",
  "states":[{"name":"a"},{"name":"b"}],
  "transitions":[{"from":"a","to":"b","priority":1,"condition":{"and":[
    {"key":"'"${P}bb_hp"'","op":">","value":0},
    {"key":"'"${P}bb_hp"'","op":"<","value":100}
  ]}}]
}')")
FSM_DK=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

R=$(fld_detail "$BB_HP")
DUP_POST=$(echo "$R" | jq -r '.data.ref_count' | tr -d '\r')
TOTAL=$((TOTAL + 1))
# 应该只 +1，不是 +2（同一 FSM 同一 Key 不应叠加）
if [ "$DUP_POST" = "$((DUP_PRE + 1))" ]; then
  echo "  [PASS] 9P.1 同 FSM 同 Key 只计 1 次引用"
  PASS=$((PASS+1))
else
  echo "  [BUG ] 9P.1 同一 Key 重复引用被重复计数 ($DUP_PRE→$DUP_POST)"
  FAIL=$((FAIL+1))
  BUGS+=("9P.1: 同一 FSM 中重复出现的 BB Key 被重复计入 ref_count")
fi

fsm_rm "$FSM_DK"

# ---- 清理 ----
fld_rm "$BB_HP"
fld_rm "$BB_DIST"
fld_rm "$BB_ENERGY"
fld_rm "$NO_BB"

echo ""
echo "  [INFO] test_09 完成 — FSM BB Key ↔ Field 跨模块测试"
echo ""
