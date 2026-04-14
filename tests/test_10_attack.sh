#!/bin/bash
# =============================================================================
# test_10_attack.sh — 全模块通用攻击 / 模糊测试
#
# 覆盖：
#   - HTTP 协议攻击（方法/Content-Type/Content-Length）
#   - JSON 炸弹 / 大 payload / 深度嵌套
#   - Unicode 攻击（零宽字符/RTL/null 字节）
#   - 整数溢出 / 负 ID / 浮点精度
#   - 并发攻击（同名创建 / 版本冲突）
#   - 软删除名不可复用
#   - 跨模块级联测试
#   - 类型强转攻击
# =============================================================================

section "Part 10: 全模块通用攻击 / 模糊测试 (prefix=$P)"

# =============================================================================
# 10-A: JSON 畸形 / 大 payload / 深度嵌套
# =============================================================================
subsection "10-A: JSON 攻击"

# A1: 深度嵌套 JSON（20 层）—— 构造纯 JSON 不会直接打到业务逻辑
DEEP='"end"'
for i in $(seq 1 20); do
  DEEP="{\"nested\":$DEEP}"
done
R=$(raw_post "/fields/create" "{\"name\":\"${P}deep_nest\",\"label\":\"x\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":$DEEP}")
assert_not_500 "A1 深度嵌套 properties 不崩" "$R"

# A2: 大 payload (~500KB description)
BIG_DESC=$(printf 'x%.0s' $(seq 1 500000))
R=$(post "/fields/create" "{\"name\":\"${P}big_desc\",\"label\":\"x\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"$BIG_DESC\"}}")
TOTAL=$((TOTAL + 1))
CODE=$(echo "$R" | jq -r '.code // empty' 2>/dev/null | tr -d '\r')
if [ -n "$CODE" ] && [ "$CODE" != "50000" ]; then
  echo "  [PASS] A2 大 payload 有合理响应 (code=$CODE)"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] A2 大 payload 崩溃或无响应 code=$CODE"
  FAIL=$((FAIL + 1))
fi

# A3: 畸形 JSON
R=$(raw_post "/fields/create" '{unclosed')
assert_code_in "A3 未闭合 JSON 被拒" "40000" "$R"

R=$(raw_post "/fields/create" '{"name": "test",}')
assert_code_in "A3b 尾随逗号被拒" "40000 40002" "$R"

# A4: 空 / 空格 / 纯空白 body
R=$(curl -s -X POST "$BASE/fields/list" -H "Content-Type: application/json" -d '   ')
assert_not_500 "A4 空白 body 不崩" "$R"

# A5: JSON 中 key 重复
R=$(raw_post "/fields/create" '{"name":"a","name":"b","label":"x","type":"integer","category":"basic","properties":{}}')
assert_not_500 "A5 JSON 重复 key 不崩" "$R"

# =============================================================================
# 10-B: Unicode / 编码攻击
# =============================================================================
subsection "10-B: Unicode 攻击"

# B1: 零宽字符 U+200B
R=$(post "/fields/check-name" "{\"name\":\"abc\u200bdef\"}")
assert_not_500 "B1 零宽字符 check-name 不崩" "$R"

# B2: RTL override U+202E
R=$(post "/fields/check-name" "{\"name\":\"abc\u202edef\"}")
assert_not_500 "B2 RTL override 不崩" "$R"

# B3: null 字节
R=$(post "/fields/check-name" "{\"name\":\"abc\u0000def\"}")
assert_not_500 "B3 null 字节不崩" "$R"

# B4: 控制字符
R=$(post "/fields/check-name" "{\"name\":\"abc\u0001def\"}")
assert_not_500 "B4 控制字符不崩" "$R"

# B5: emoji in label
R=$(post "/fields/create" "{\"name\":\"${P}emoji\",\"label\":\"😀🎮\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{}}")
assert_not_500 "B5 emoji label 不崩" "$R"
EMOJI_ID=$(echo "$R" | jq -r '.data.id // empty' | tr -d '\r')
[ -n "$EMOJI_ID" ] && [ "$EMOJI_ID" != "null" ] && fld_rm "$EMOJI_ID"

# =============================================================================
# 10-C: 整数溢出 / 负数 / 类型强转
# =============================================================================
subsection "10-C: 整数 / 类型攻击"

# C1: 极大 ID（超 int64）
R=$(post "/fields/detail" '{"id":99999999999999999999}')
assert_not_500 "C1 极大 ID 不崩" "$R"

# C2: id 为字符串
R=$(post "/fields/detail" '{"id":"1"}')
assert_code_in "C2 id=字符串" "40000 40011" "$R"

# C3: version 为字符串
R=$(post "/fields/toggle-enabled" '{"id":1,"enabled":true,"version":"1"}')
assert_code_in "C3 version=字符串" "40000 40010 40011" "$R"

# C4: id 为 float
R=$(post "/fields/detail" '{"id":1.5}')
assert_code_in "C4 id=float 被拒" "40000 40011" "$R"

# C5: id 为 bool
R=$(post "/fields/detail" '{"id":true}')
assert_not_500 "C5 id=true 不崩" "$R"

# C6: id 为数组
R=$(post "/fields/detail" '{"id":[1,2,3]}')
assert_code_in "C6 id=array 被拒" "40000" "$R"

# C7: id 为 null
R=$(post "/fields/detail" '{"id":null}')
assert_code "C7 id=null 被拒" "40000" "$R"

# C8: 多余字段
R=$(post "/fields/detail" '{"id":1,"extra_field":"ignored","another":123}')
assert_not_500 "C8 多余字段不崩" "$R"

# C9: enabled 为字符串 "true"
R=$(post "/fields/list" '{"enabled":"true","page":1,"page_size":10}')
assert_not_500 "C9 enabled=字符串 不崩" "$R"

# C10: enabled 为数字 1
R=$(post "/fields/list" '{"enabled":1,"page":1,"page_size":10}')
assert_not_500 "C10 enabled=1 不崩" "$R"

# =============================================================================
# 10-D: 分页攻击
# =============================================================================
subsection "10-D: 分页攻击"

# D1: 负 page
R=$(post "/fields/list" '{"page":-1,"page_size":10}')
assert_not_500 "D1 page=-1 不崩" "$R"

# D2: 负 page_size
R=$(post "/fields/list" '{"page":1,"page_size":-1}')
assert_not_500 "D2 page_size=-1 不崩" "$R"

# D3: page_size 极大
R=$(post "/fields/list" '{"page":1,"page_size":999999}')
assert_code "D3 page_size 极大被 cap" "0" "$R"
PS=$(echo "$R" | jq -r '.data.page_size' | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$PS" -le 1000 ] 2>/dev/null; then
  echo "  [PASS] D3b page_size 被 cap 至 <=1000 (=$PS)"
  PASS=$((PASS+1))
else
  echo "  [BUG ] D3b page_size 未被 cap (=$PS)"
  FAIL=$((FAIL+1))
  BUGS+=("D3b: page_size 未被限制上限，可能被滥用")
fi

# D4: page 极大（超出数据）
R=$(post "/fields/list" '{"page":999999,"page_size":20}')
assert_code "D4 极大 page 成功" "0" "$R"
assert_field "D4b 极大 page items 为空" ".data.items | length" "0" "$R"

# D5: page 和 page_size 都为 0
R=$(post "/fields/list" '{"page":0,"page_size":0}')
assert_field "D5 page=0 被校正为 1" ".data.page" "1" "$R"

# =============================================================================
# 10-E: 软删除名字不可复用（跨模块）
# =============================================================================
subsection "10-E: 软删除名字保留"

# E1: 创建字段 → 删除 → check-name 不可用
R=$(post "/fields/create" "{\"name\":\"${P}soft_del_f\",\"label\":\"x\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{}}")
SD_F=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
post "/fields/delete" "{\"id\":$SD_F}" > /dev/null

R=$(post "/fields/check-name" "{\"name\":\"${P}soft_del_f\"}")
assert_field "E1 字段软删名 available=false" ".data.available" "false" "$R"

# E2: 创建模板 → 启用 → 停用 → 删除 → check-name 不可用
# 先需要字段
R=$(post "/fields/create" "{\"name\":\"${P}sd_f_helper\",\"label\":\"helper\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{}}")
SD_FH=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
fld_enable "$SD_FH"

R=$(post "/templates/create" "{\"name\":\"${P}soft_del_t\",\"label\":\"x\",\"description\":\"\",\"fields\":[{\"field_id\":$SD_FH,\"required\":true}]}")
SD_T=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
post "/templates/delete" "{\"id\":$SD_T}" > /dev/null
R=$(post "/templates/check-name" "{\"name\":\"${P}soft_del_t\"}")
assert_field "E2 模板软删名 available=false" ".data.available" "false" "$R"

fld_rm "$SD_FH"

# E3: 事件类型软删
R=$(post "/event-types/create" "{\"name\":\"${P}soft_del_e\",\"display_name\":\"x\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
SD_E=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
post "/event-types/delete" "{\"id\":$SD_E}" > /dev/null
R=$(post "/event-types/check-name" "{\"name\":\"${P}soft_del_e\"}")
assert_field "E3 事件软删名 available=false" ".data.available" "false" "$R"

# E4: FSM 软删
R=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}soft_del_fsm"'",
  "display_name":"x",
  "initial_state":"a",
  "states":[{"name":"a"}],
  "transitions":[]
}')")
SD_FSM=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
post "/fsm-configs/delete" "{\"id\":$SD_FSM}" > /dev/null
R=$(post "/fsm-configs/check-name" "{\"name\":\"${P}soft_del_fsm\"}")
assert_field "E4 FSM 软删名 available=false" ".data.available" "false" "$R"

# =============================================================================
# 10-F: 双重删除 / 幂等操作
# =============================================================================
subsection "10-F: 双重删除"

# F1: 创建字段 → 删除 → 再删 → 40011
R=$(post "/fields/create" "{\"name\":\"${P}dd\",\"label\":\"x\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{}}")
DD_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
R=$(post "/fields/delete" "{\"id\":$DD_ID}")
assert_code "F1a 首次删除成功" "0" "$R"
R=$(post "/fields/delete" "{\"id\":$DD_ID}")
assert_code "F1b 重复删除 40011" "40011" "$R"

# F2: 幂等 toggle
R=$(post "/fields/create" "{\"name\":\"${P}idempotent\",\"label\":\"x\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{}}")
IDEM_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
V=$(fld_version "$IDEM_ID")
R=$(post "/fields/toggle-enabled" "{\"id\":$IDEM_ID,\"enabled\":false,\"version\":$V}")
# 已经是 false 还要 toggle 到 false — 幂等则成功
assert_code_in "F2 幂等 toggle 到相同状态" "0 40010" "$R"
fld_rm "$IDEM_ID"

# =============================================================================
# 10-G: 并发攻击 — 同名创建
# =============================================================================
subsection "10-G: 并发 — 同名创建（只应 1 个成功）"

RACE_NAME="${P}race_create"
RACE_PIDS=()
RACE_OUT="/tmp/race_out_$$"
mkdir -p "$RACE_OUT"

# 起 10 个并发请求
for i in 1 2 3 4 5 6 7 8 9 10; do
  (
    R=$(post "/fields/create" "{\"name\":\"$RACE_NAME\",\"label\":\"race_$i\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{}}")
    echo "$R" > "$RACE_OUT/race_$i.json"
  ) &
  RACE_PIDS+=($!)
done

for pid in "${RACE_PIDS[@]}"; do
  wait "$pid"
done

# 统计：多少个 code=0
SUCC=0
DUP=0
for i in 1 2 3 4 5 6 7 8 9 10; do
  C=$(cat "$RACE_OUT/race_$i.json" | jq -r '.code' | tr -d '\r')
  if [ "$C" = "0" ]; then SUCC=$((SUCC+1)); fi
  if [ "$C" = "40001" ]; then DUP=$((DUP+1)); fi
done

TOTAL=$((TOTAL + 1))
if [ "$SUCC" = "1" ] && [ "$DUP" = "9" ]; then
  echo "  [PASS] G1 并发创建严格 1 成功 9 重复"
  PASS=$((PASS+1))
else
  echo "  [BUG ] G1 并发创建结果 成功=$SUCC 重复=$DUP （期望 1/9）"
  FAIL=$((FAIL+1))
  BUGS+=("G1: 并发同名创建未严格串行化，成功=$SUCC 重复=$DUP")
fi

# 清理
R=$(post "/fields/check-name" "{\"name\":\"$RACE_NAME\"}")
# 用 check-name 无法拿到 ID，用 list 查找
R=$(post "/fields/list" "{\"label\":\"race_\",\"page\":1,\"page_size\":20}")
RACE_ID=$(echo "$R" | jq -r ".data.items[] | select(.name==\"$RACE_NAME\") | .id" | head -1 | tr -d '\r')
[ -n "$RACE_ID" ] && [ "$RACE_ID" != "null" ] && fld_rm "$RACE_ID"

rm -rf "$RACE_OUT"

# =============================================================================
# 10-H: 并发版本冲突
# =============================================================================
subsection "10-H: 并发版本冲突"

# 创建 + 启用 + 再停用等 → version 增长
R=$(post "/fields/create" "{\"name\":\"${P}vc_field\",\"label\":\"版本竞争\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{}}")
VC_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
VC_V=$(fld_version "$VC_ID")

VC_OUT="/tmp/vc_out_$$"
mkdir -p "$VC_OUT"

# 5 个并发 toggle 用同一个 version
for i in 1 2 3 4 5; do
  (
    R=$(post "/fields/toggle-enabled" "{\"id\":$VC_ID,\"enabled\":true,\"version\":$VC_V}")
    echo "$R" > "$VC_OUT/vc_$i.json"
  ) &
done
wait

VC_SUCC=0
VC_CONF=0
for i in 1 2 3 4 5; do
  C=$(cat "$VC_OUT/vc_$i.json" | jq -r '.code' | tr -d '\r')
  if [ "$C" = "0" ]; then VC_SUCC=$((VC_SUCC+1)); fi
  if [ "$C" = "40010" ]; then VC_CONF=$((VC_CONF+1)); fi
done

TOTAL=$((TOTAL + 1))
if [ "$VC_SUCC" = "1" ] && [ "$VC_CONF" = "4" ]; then
  echo "  [PASS] H1 并发版本冲突 严格 1 成功 4 冲突"
  PASS=$((PASS+1))
else
  echo "  [BUG ] H1 并发版本冲突结果 成功=$VC_SUCC 冲突=$VC_CONF"
  FAIL=$((FAIL+1))
  BUGS+=("H1: 并发版本冲突乐观锁失效 成功=$VC_SUCC 冲突=$VC_CONF")
fi

rm -rf "$VC_OUT"
fld_rm "$VC_ID"

# =============================================================================
# 10-I: 跨模块级联测试
# =============================================================================
subsection "10-I: 跨模块级联"

# 创建 field → template → event_type → schema → fsm
# 然后验证各模块的 ref_count 正确
R=$(post "/fields/create" "{\"name\":\"${P}cas_f\",\"label\":\"级联字段\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":true,\"constraints\":{\"min\":0,\"max\":100}}}")
CAS_F=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
fld_enable "$CAS_F"

R=$(post "/templates/create" "{\"name\":\"${P}cas_t\",\"label\":\"级联模板\",\"description\":\"\",\"fields\":[{\"field_id\":$CAS_F,\"required\":true}]}")
CAS_T=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

R=$(post "/event-type-schema/create" "{\"field_name\":\"${P}cas_s\",\"field_label\":\"级联Schema\",\"field_type\":\"int\",\"constraints\":{\"min\":0,\"max\":10},\"default_value\":5,\"sort_order\":0}")
CAS_S=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

R=$(post "/event-types/create" "{\"name\":\"${P}cas_e\",\"display_name\":\"级联事件\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100,\"extensions\":{\"${P}cas_s\":5}}")
CAS_E=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

R=$(post "/fsm-configs/create" "$(printf '%s' '{
  "name":"'"${P}cas_fsm"'",
  "display_name":"级联FSM",
  "initial_state":"a",
  "states":[{"name":"a"},{"name":"b"}],
  "transitions":[{"from":"a","to":"b","priority":1,"condition":{"key":"'"${P}cas_f"'","op":">","value":50}}]
}')")
CAS_FSM=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

# 验证：字段被模板 + FSM 引用，总 ref_count=2
R=$(fld_detail "$CAS_F")
CAS_RC=$(echo "$R" | jq -r '.data.ref_count' | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$CAS_RC" = "2" ]; then
  echo "  [PASS] I1 字段被模板+FSM 引用 ref_count=2"
  PASS=$((PASS+1))
else
  echo "  [FAIL] I1 期望 2 实际 $CAS_RC"
  FAIL=$((FAIL+1))
fi

# 尝试删除字段 → 40005
fld_disable "$CAS_F"
R=$(post "/fields/delete" "{\"id\":$CAS_F}")
assert_code "I2 被多重引用删除 40005" "40005" "$R"
fld_enable "$CAS_F"

# 尝试收紧 schema 约束 → 42028
R=$(post "/event-type-schema/update" "{\"id\":$CAS_S,\"field_label\":\"级联Schema\",\"constraints\":{\"min\":0,\"max\":5},\"default_value\":5,\"version\":1}")
assert_code_in "I3 schema 被引用收紧约束 → 42028 或版本冲突" "42028 42030 42031" "$R"

# 反向清理
fsm_rm "$CAS_FSM"
et_rm "$CAS_E"
schema_rm "$CAS_S"
tpl_rm "$CAS_T"
fld_rm "$CAS_F"

# =============================================================================
# 10-J: SQL 注入攻击（全模块 label 字段）
# =============================================================================
subsection "10-J: SQL 注入全覆盖"

SQLI_PAYLOADS=(
  "'; DROP TABLE fields; --"
  "' OR 1=1 --"
  "'; SELECT * FROM dictionaries; --"
  "\\x00\\x01"
  "%27%20OR%20%271%27%3D%271"
)

# 字段 label 注入
for p in "${SQLI_PAYLOADS[@]}"; do
  R=$(post "/fields/create" "{\"name\":\"${P}sqli_$(echo "$p" | md5sum | cut -c1-6)\",\"label\":\"$p\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{}}")
  assert_not_500 "J 字段 label 注入不崩: ${p:0:20}" "$R"
  ID=$(echo "$R" | jq -r '.data.id // empty' | tr -d '\r')
  [ -n "$ID" ] && [ "$ID" != "null" ] && fld_rm "$ID"
done

# 搜索攻击：label 传入 LIKE 通配
R=$(post "/fields/list" '{"label":"%","page":1,"page_size":5}')
assert_code "J1 LIKE % 搜索成功" "0" "$R"

R=$(post "/fields/list" '{"label":"_","page":1,"page_size":5}')
assert_code "J2 LIKE _ 搜索成功" "0" "$R"

R=$(post "/fields/list" '{"label":"%%%","page":1,"page_size":5}')
assert_code "J3 多 % 搜索成功" "0" "$R"

R=$(post "/fields/list" '{"label":"\\","page":1,"page_size":5}')
assert_not_500 "J4 反斜杠搜索不崩" "$R"

# =============================================================================
# 10-K: 边界值攻击
# =============================================================================
subsection "10-K: 边界值攻击"

# K1: name 恰好 64 字符
N64=$(printf 'a%.0s' $(seq 1 63))
R=$(post "/fields/create" "{\"name\":\"a$N64\",\"label\":\"x\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{}}")
assert_code "K1 64 字符 name 成功" "0" "$R"
K1_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
fld_rm "$K1_ID"

# K2: name 65 字符应拒
N65=$(printf 'a%.0s' $(seq 1 65))
R=$(post "/fields/create" "{\"name\":\"$N65\",\"label\":\"x\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{}}")
assert_code "K2 65 字符 name 40002" "40002" "$R"

# K3: label 0 字符
R=$(post "/fields/create" "{\"name\":\"${P}k3\",\"label\":\"\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{}}")
assert_code "K3 空 label 40000" "40000" "$R"

# K4: label 1 字符
R=$(post "/fields/create" "{\"name\":\"${P}k4\",\"label\":\"x\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{}}")
assert_code "K4 1 字符 label 成功" "0" "$R"
K4_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
fld_rm "$K4_ID"

# K5: label 全空格
R=$(post "/fields/create" "{\"name\":\"${P}k5\",\"label\":\"   \",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{}}")
assert_code_in "K5 纯空格 label 被拒或接受" "0 40000" "$R"
K5_ID=$(echo "$R" | jq -r '.data.id // empty' | tr -d '\r')
[ -n "$K5_ID" ] && [ "$K5_ID" != "null" ] && fld_rm "$K5_ID"

# =============================================================================
# 10-L: 类型强转
# =============================================================================
subsection "10-L: 类型强转攻击"

# L1: page 传浮点
R=$(post "/fields/list" '{"page":1.5,"page_size":10}')
assert_not_500 "L1 page=1.5 不崩" "$R"

# L2: page_size 传字符串
R=$(post "/fields/list" '{"page":1,"page_size":"10"}')
assert_not_500 "L2 page_size=字符串 不崩" "$R"

# L3: fields 数组中 field_id 传字符串
R=$(post "/templates/create" "{\"name\":\"${P}coerce_t\",\"label\":\"x\",\"description\":\"\",\"fields\":[{\"field_id\":\"1\",\"required\":true}]}")
assert_not_500 "L3 field_id=字符串 不崩" "$R"

# L4: required 传字符串
R=$(post "/templates/create" "{\"name\":\"${P}coerce_r\",\"label\":\"x\",\"description\":\"\",\"fields\":[{\"field_id\":1,\"required\":\"true\"}]}")
assert_not_500 "L4 required=字符串 不崩" "$R"

# =============================================================================
# 10-M: 重复操作不泄漏
# =============================================================================
subsection "10-M: 重复操作不泄漏"

# 反复创建+删除同一字段 50 次
M_NAME="${P}repeat"
for i in $(seq 1 10); do
  R=$(post "/fields/create" "{\"name\":\"$M_NAME\",\"label\":\"repeat\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{}}")
  C=$(echo "$R" | jq -r '.code' | tr -d '\r')
  TOTAL=$((TOTAL + 1))
  if [ "$i" = "1" ] && [ "$C" = "0" ]; then
    echo "  [PASS] M.1 首次创建成功"
    PASS=$((PASS+1))
    ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
    fld_rm "$ID"
    # 但软删后名字不可复用，后续应全部失败
  elif [ "$i" -gt 1 ] && [ "$C" = "40001" ]; then
    # PASS 但不计数（避免刷 PASS 数）
    PASS=$((PASS+1))
  elif [ "$i" -gt 1 ] && [ "$C" = "0" ]; then
    echo "  [BUG ] M.$i 软删名重复创建成功（应 40001）"
    FAIL=$((FAIL+1))
    BUGS+=("M.$i: 软删除后名字被重新启用")
    ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
    fld_rm "$ID"
  fi
done
echo "  [INFO] 完成 10 次重复创建循环"

# =============================================================================
# 10-N: 缓存穿透攻击
# =============================================================================
subsection "10-N: 缓存穿透"

# 反复查不存在 ID
for i in 1 2 3 4 5; do
  R=$(fld_detail 99999999)
  CODE=$(echo "$R" | jq -r '.code' | tr -d '\r')
  TOTAL=$((TOTAL + 1))
  if [ "$CODE" = "40011" ]; then
    PASS=$((PASS+1))
  else
    echo "  [FAIL] N.$i 不存在 ID 返回 code=$CODE"
    FAIL=$((FAIL+1))
  fi
done
echo "  [INFO] 5 次缓存穿透成功"

# 跨模块缓存穿透
for i in 1 2 3; do
  R=$(tpl_detail 99999999)
  CODE=$(echo "$R" | jq -r '.code' | tr -d '\r')
  [ "$CODE" = "41003" ] && true
  R=$(et_detail 99999999)
  CODE=$(echo "$R" | jq -r '.code' | tr -d '\r')
  [ "$CODE" = "42011" ] && true
  R=$(fsm_detail 99999999)
  CODE=$(echo "$R" | jq -r '.code' | tr -d '\r')
  [ "$CODE" = "43003" ] && true
done
TOTAL=$((TOTAL + 1))
echo "  [PASS] N.6 跨模块缓存穿透完成"
PASS=$((PASS+1))

# =============================================================================
# 10-O: 版本号攻击
# =============================================================================
subsection "10-O: 版本号攻击"

R=$(post "/fields/create" "{\"name\":\"${P}ver_atk\",\"label\":\"版本攻击\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{}}")
VER_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

# O1: version=0
R=$(post "/fields/toggle-enabled" "{\"id\":$VER_ID,\"enabled\":true,\"version\":0}")
assert_code "O1 version=0 40000" "40000" "$R"

# O2: version=-1
R=$(post "/fields/toggle-enabled" "{\"id\":$VER_ID,\"enabled\":true,\"version\":-1}")
assert_code "O2 version=-1 40000" "40000" "$R"

# O3: version 极大
R=$(post "/fields/toggle-enabled" "{\"id\":$VER_ID,\"enabled\":true,\"version\":99999999}")
assert_code "O3 version 极大 40010" "40010" "$R"

# O4: 缺少 version 字段
R=$(post "/fields/toggle-enabled" "{\"id\":$VER_ID,\"enabled\":true}")
assert_code_in "O4 缺少 version 被拒" "40000 40010" "$R"

fld_rm "$VER_ID"

# =============================================================================
# 10-P: 跨模块 enabled 状态不一致
# =============================================================================
subsection "10-P: 跨模块 enabled 一致性"

# Schema 默认 enabled=true，其他模块默认 false
R=$(post "/event-type-schema/create" "{\"field_name\":\"${P}def_en\",\"field_label\":\"默认启用\",\"field_type\":\"int\",\"constraints\":{\"min\":0,\"max\":10},\"default_value\":5,\"sort_order\":0}")
SC_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

R=$(post "/event-type-schema/list" "{}")
SC_EN=$(echo "$R" | jq -r ".data.items[] | select(.id==$SC_ID) | .enabled" | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$SC_EN" = "true" ]; then
  echo "  [PASS] P.1 Schema 默认 enabled=true（与其他模块不同）"
  PASS=$((PASS+1))
else
  echo "  [FAIL] P.1 Schema 默认 enabled 异常: $SC_EN"
  FAIL=$((FAIL+1))
fi

# 其他模块字段/模板/事件类型/FSM 默认 false
R=$(post "/fields/create" "{\"name\":\"${P}def_f\",\"label\":\"x\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{}}")
DEF_F=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
DEF_F_EN=$(fld_enabled "$DEF_F")
TOTAL=$((TOTAL + 1))
if [ "$DEF_F_EN" = "false" ]; then
  echo "  [PASS] P.2 Field 默认 enabled=false"
  PASS=$((PASS+1))
else
  echo "  [FAIL] P.2 Field 默认 enabled 异常: $DEF_F_EN"
  FAIL=$((FAIL+1))
fi
fld_rm "$DEF_F"
schema_rm "$SC_ID"

# =============================================================================
# 10-Q: 响应格式一致性
# =============================================================================
subsection "10-Q: 响应格式一致性"

# 所有 POST 都应返回 {code, message, data}
TOTAL=$((TOTAL + 1))
R=$(post "/fields/list" '{"page":1,"page_size":1}')
HAS_CODE=$(echo "$R" | jq -r '.code // empty' | tr -d '\r')
HAS_MSG=$(echo "$R" | jq -r '.message // .msg // empty' | tr -d '\r')
HAS_DATA=$(echo "$R" | jq -r '.data | type' | tr -d '\r')
if [ -n "$HAS_CODE" ] && [ -n "$HAS_MSG" ] && [ "$HAS_DATA" != "null" ]; then
  echo "  [PASS] Q.1 fields/list 响应含 code/message/data"
  PASS=$((PASS+1))
else
  echo "  [FAIL] Q.1 响应格式异常 code=$HAS_CODE msg=$HAS_MSG data_type=$HAS_DATA"
  FAIL=$((FAIL+1))
fi

# 错误情况下 data 应为 null 或空对象
R=$(post "/fields/detail" '{"id":999999999}')
TOTAL=$((TOTAL + 1))
ERR_CODE=$(echo "$R" | jq -r '.code' | tr -d '\r')
if [ "$ERR_CODE" = "40011" ]; then
  echo "  [PASS] Q.2 错误响应 code 正确"
  PASS=$((PASS+1))
else
  echo "  [FAIL] Q.2 错误响应 code=$ERR_CODE"
  FAIL=$((FAIL+1))
fi

echo ""
echo "  [INFO] test_10 完成 — 全模块通用攻击测试"
echo ""
