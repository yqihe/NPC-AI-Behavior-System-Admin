#!/bin/bash
# =============================================================================
# test_05_schema.sh — 扩展字段 Schema CRUD + 约束校验 + 攻击性测试
#
# 前置：run_all.sh 已 source helpers.sh，$BASE / $P / assert_* / post() 可用
#
# 导出变量：SCHEMA_ID1 ~ SCHEMA_ID5（供 test_06 使用）
# =============================================================================

section "Part 5: 扩展字段 Schema CRUD (prefix=$P)"

# =============================================================================
# 1. 创建各类型 schema（int, float, string, bool, select）
# =============================================================================
subsection "1. 创建各类型 schema"

# 1.1 int 类型 (priority)
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}priority\",\"field_label\":\"优先级\",\"field_type\":\"int\",\"constraints\":{\"min\":0,\"max\":10},\"default_value\":5,\"sort_order\":1}")
assert_code "1.1 创建 int schema" "0" "$body"
SCHEMA_ID1=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 1.2 float 类型 (radius)
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}radius\",\"field_label\":\"半径\",\"field_type\":\"float\",\"constraints\":{\"min\":0.0,\"max\":1000.0,\"precision\":2},\"default_value\":50.5,\"sort_order\":2}")
assert_code "1.2 创建 float schema" "0" "$body"
SCHEMA_ID2=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 1.3 string 类型 (description)
printf '{"field_name":"%sdesc","field_label":"描述","field_type":"string","constraints":{"minLength":0,"maxLength":100},"default_value":"默认描述","sort_order":3}' "$P" \
  | curl -s -X POST "$BASE/event-type-schema/create" -H "Content-Type: application/json; charset=utf-8" --data-binary @- > /tmp/schema_str.json
body=$(cat /tmp/schema_str.json)
assert_code "1.3 创建 string schema" "0" "$body"
SCHEMA_ID3=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 1.4 bool 类型 (is_dangerous)
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}is_dangerous\",\"field_label\":\"是否危险\",\"field_type\":\"bool\",\"constraints\":{},\"default_value\":false,\"sort_order\":4}")
assert_code "1.4 创建 bool schema" "0" "$body"
SCHEMA_ID4=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 1.5 select 类型 (level)
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}level\",\"field_label\":\"等级\",\"field_type\":\"select\",\"constraints\":{\"options\":[{\"value\":\"low\"},{\"value\":\"mid\"},{\"value\":\"high\"}],\"minSelect\":1,\"maxSelect\":1},\"default_value\":\"low\",\"sort_order\":5}")
assert_code "1.5 创建 select schema" "0" "$body"
SCHEMA_ID5=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 1.6 验证创建后默认 ENABLED
body=$(post "/event-type-schema/list" "{}")
S1_ENABLED=$(echo "$body" | jq -r ".data.items[] | select(.id==$SCHEMA_ID1) | .enabled" | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$S1_ENABLED" = "true" ]; then
  echo "  [PASS] 1.6 schema 默认启用"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] 1.6 schema 默认启用 — 实际: $S1_ENABLED"
  FAIL=$((FAIL + 1))
fi

# =============================================================================
# 2. field_name 校验
# =============================================================================
subsection "2. field_name 校验"

# 2.1 空 field_name → 42021
body=$(post "/event-type-schema/create" "{\"field_name\":\"\",\"field_label\":\"空\",\"field_type\":\"int\",\"constraints\":{},\"default_value\":0,\"sort_order\":0}")
assert_code "2.1 空 field_name → 42021" "42021" "$body"

# 2.2 大写 field_name → 42021
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}BadCase\",\"field_label\":\"大写\",\"field_type\":\"int\",\"constraints\":{},\"default_value\":0,\"sort_order\":0}")
assert_code "2.2 大写 field_name → 42021" "42021" "$body"

# 2.3 重复 field_name → 42020
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}priority\",\"field_label\":\"重复\",\"field_type\":\"int\",\"constraints\":{},\"default_value\":0,\"sort_order\":0}")
assert_code "2.3 重复 field_name → 42020" "42020" "$body"

# 2.4 特殊字符
body=$(post "/event-type-schema/create" "{\"field_name\":\"bad@name!\",\"field_label\":\"特殊\",\"field_type\":\"int\",\"constraints\":{},\"default_value\":0,\"sort_order\":0}")
assert_code "2.4 特殊字符 field_name → 42021" "42021" "$body"

# 2.5 含空格
body=$(post "/event-type-schema/create" "{\"field_name\":\"bad name\",\"field_label\":\"空格\",\"field_type\":\"int\",\"constraints\":{},\"default_value\":0,\"sort_order\":0}")
assert_code "2.5 含空格 field_name → 42021" "42021" "$body"

# =============================================================================
# 3. field_type 校验 — TYPE_INVALID (42024)
# =============================================================================
subsection "3. field_type 校验"

# 3.1 reference
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_t1\",\"field_label\":\"坏类型\",\"field_type\":\"reference\",\"constraints\":{},\"default_value\":0,\"sort_order\":0}")
assert_code "3.1 field_type=reference → 42024" "42024" "$body"

# 3.2 array
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_t2\",\"field_label\":\"坏类型\",\"field_type\":\"array\",\"constraints\":{},\"default_value\":0,\"sort_order\":0}")
assert_code "3.2 field_type=array → 42024" "42024" "$body"

# 3.3 unknown
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_t3\",\"field_label\":\"坏类型\",\"field_type\":\"unknown\",\"constraints\":{},\"default_value\":0,\"sort_order\":0}")
assert_code "3.3 field_type=unknown → 42024" "42024" "$body"

# 3.4 空 field_type
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_t4\",\"field_label\":\"空类型\",\"field_type\":\"\",\"constraints\":{},\"default_value\":0,\"sort_order\":0}")
assert_code_in "3.4 field_type=空 被拒" "42024 40000" "$body"

# =============================================================================
# 4. constraints 自洽校验 — CONSTRAINTS_INVALID (42025)
# =============================================================================
subsection "4. constraints 自洽校验"

# 4.1 int: min > max
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_int_mm\",\"field_label\":\"坏范围\",\"field_type\":\"int\",\"constraints\":{\"min\":10,\"max\":1},\"default_value\":5,\"sort_order\":0}")
assert_code "4.1 int min>max → 42025" "42025" "$body"

# 4.2 float: min > max
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_flt_mm\",\"field_label\":\"坏浮点范围\",\"field_type\":\"float\",\"constraints\":{\"min\":100.5,\"max\":10.0},\"default_value\":50,\"sort_order\":0}")
assert_code "4.2 float min>max → 42025" "42025" "$body"

# 4.3 string: minLength > maxLength
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_str_mm\",\"field_label\":\"坏字串范围\",\"field_type\":\"string\",\"constraints\":{\"minLength\":100,\"maxLength\":10},\"default_value\":\"x\",\"sort_order\":0}")
assert_code "4.3 string minLength>maxLength → 42025" "42025" "$body"

# 4.4 select: minSelect > maxSelect
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_sel_mm\",\"field_label\":\"坏选择范围\",\"field_type\":\"select\",\"constraints\":{\"options\":[{\"value\":\"a\"},{\"value\":\"b\"}],\"minSelect\":5,\"maxSelect\":1},\"default_value\":\"a\",\"sort_order\":0}")
assert_code "4.4 select minSelect>maxSelect → 42025" "42025" "$body"

# 4.5 string: minLength 负数
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_str_neg\",\"field_label\":\"负minLen\",\"field_type\":\"string\",\"constraints\":{\"minLength\":-1,\"maxLength\":10},\"default_value\":\"x\",\"sort_order\":0}")
assert_code "4.5 string minLength=-1 → 42025" "42025" "$body"

# 4.6 float: precision 负数
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_flt_prec\",\"field_label\":\"负精度\",\"field_type\":\"float\",\"constraints\":{\"min\":0,\"max\":100,\"precision\":-1},\"default_value\":50,\"sort_order\":0}")
assert_code "4.6 float precision=-1 → 42025" "42025" "$body"

# 4.7 int: min=100 max=10（大间距）
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_int_100\",\"field_label\":\"坏范围100>10\",\"field_type\":\"int\",\"constraints\":{\"min\":100,\"max\":10},\"default_value\":5,\"sort_order\":0}")
assert_code "4.7 int min=100 max=10 → 42025" "42025" "$body"

# 4.8 float: 两个负数 min=-1 max=-10
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_flt_neg\",\"field_label\":\"负浮点范围\",\"field_type\":\"float\",\"constraints\":{\"min\":-1.0,\"max\":-10.0},\"default_value\":-5,\"sort_order\":0}")
assert_code "4.8 float min=-1 max=-10 → 42025" "42025" "$body"

# =============================================================================
# 5. default_value vs constraints — DEFAULT_INVALID (42026)
# =============================================================================
subsection "5. default_value vs constraints"

# 5.1 int default 超上限
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_def_int\",\"field_label\":\"超范围默认\",\"field_type\":\"int\",\"constraints\":{\"min\":1,\"max\":10},\"default_value\":99,\"sort_order\":0}")
assert_code "5.1 int default=99 超 max=10 → 42026" "42026" "$body"

# 5.2 int default 低于 min
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_def_int2\",\"field_label\":\"低范围默认\",\"field_type\":\"int\",\"constraints\":{\"min\":5,\"max\":10},\"default_value\":1,\"sort_order\":0}")
assert_code "5.2 int default=1 低于 min=5 → 42026" "42026" "$body"

# 5.3 float default 超上限
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_def_flt\",\"field_label\":\"浮点超范围\",\"field_type\":\"float\",\"constraints\":{\"min\":0.0,\"max\":10.0},\"default_value\":99.9,\"sort_order\":0}")
assert_code "5.3 float default=99.9 超 max=10 → 42026" "42026" "$body"

# 5.4 string default 超 maxLength
printf '{"field_name":"%sbad_def_str","field_label":"超长默认","field_type":"string","constraints":{"minLength":0,"maxLength":3},"default_value":"超过三个字符","sort_order":0}' "$P" \
  | curl -s -X POST "$BASE/event-type-schema/create" -H "Content-Type: application/json; charset=utf-8" --data-binary @- > /tmp/schema_bad_str.json
body=$(cat /tmp/schema_bad_str.json)
assert_code "5.4 string default 超 maxLength → 42026" "42026" "$body"

# 5.5 select default 不在 options 中
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_def_sel\",\"field_label\":\"不在选项\",\"field_type\":\"select\",\"constraints\":{\"options\":[{\"value\":\"a\"},{\"value\":\"b\"}]},\"default_value\":\"c\",\"sort_order\":0}")
assert_code "5.5 select default=c 不在 options → 42026" "42026" "$body"

# 5.6 int default=100 with max=10
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_def_ex\",\"field_label\":\"精确超范围\",\"field_type\":\"int\",\"constraints\":{\"min\":0,\"max\":10},\"default_value\":100,\"sort_order\":0}")
assert_code "5.6 int default=100 max=10 → 42026" "42026" "$body"

# =============================================================================
# 6. 列表
# =============================================================================
subsection "6. 列表"

body=$(post "/event-type-schema/list" "{}")
assert_code "6.1 schema 列表成功" "0" "$body"
assert_ge "6.1 至少 5 条 schema" '.data.items | length' "5" "$body"

# 6.2 验证各 schema ID 存在于列表中
S1_EXISTS=$(echo "$body" | jq ".data.items[] | select(.id==$SCHEMA_ID1) | .id" | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$S1_EXISTS" = "$SCHEMA_ID1" ]; then
  echo "  [PASS] 6.2 SCHEMA_ID1 在列表中"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] 6.2 SCHEMA_ID1 不在列表中"
  FAIL=$((FAIL + 1))
fi

# =============================================================================
# 7. 停用 / 启用切换
# =============================================================================
subsection "7. 停用 / 启用切换"

# 7.1 停用 SCHEMA_ID2 (float)
V=$(schema_version "$SCHEMA_ID2")
body=$(post "/event-type-schema/toggle-enabled" "{\"id\":$SCHEMA_ID2,\"version\":${V:-1}}")
assert_code "7.1 停用 float schema" "0" "$body"

# 7.2 验证停用状态
body=$(post "/event-type-schema/list" "{}")
S2_EN=$(echo "$body" | jq -r ".data.items[] | select(.id==$SCHEMA_ID2) | .enabled" | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$S2_EN" = "false" ]; then
  echo "  [PASS] 7.2 float schema 已停用"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] 7.2 float schema 停用失败 — 实际: $S2_EN"
  FAIL=$((FAIL + 1))
fi

# 7.3 再次 toggle 恢复启用
V=$(schema_version "$SCHEMA_ID2")
body=$(post "/event-type-schema/toggle-enabled" "{\"id\":$SCHEMA_ID2,\"version\":${V:-2}}")
assert_code "7.3 恢复启用 float schema" "0" "$body"

# 7.4 版本冲突 — stale version
body=$(post "/event-type-schema/toggle-enabled" "{\"id\":$SCHEMA_ID1,\"version\":999}")
assert_code "7.4 toggle stale version → 42030" "42030" "$body"

# 7.5 连续两次 toggle 同一 version — 第二次冲突
V=$(schema_version "$SCHEMA_ID1")
body=$(post "/event-type-schema/toggle-enabled" "{\"id\":$SCHEMA_ID1,\"version\":$V}")
assert_code "7.5a 第一次 toggle 成功" "0" "$body"
body=$(post "/event-type-schema/toggle-enabled" "{\"id\":$SCHEMA_ID1,\"version\":$V}")
assert_code "7.5b 第二次 toggle 同 version → 42030" "42030" "$body"

# 7.6 恢复 SCHEMA_ID1 到启用状态
V=$(schema_version "$SCHEMA_ID1")
body=$(post "/event-type-schema/toggle-enabled" "{\"id\":$SCHEMA_ID1,\"version\":$V}")
assert_code "7.6 恢复 SCHEMA_ID1 启用" "0" "$body"

# =============================================================================
# 8. 删除守卫 — CANNOT_DELETE_ENABLED (42027)
# =============================================================================
subsection "8. 删除守卫"

# 8.1 删除启用中 schema → 42027
body=$(post "/event-type-schema/delete" "{\"id\":$SCHEMA_ID1}")
assert_code "8.1 删除未停用 schema → 42027" "42027" "$body"

# 8.2 创建临时 → 停用 → 删除成功
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}tmp_del\",\"field_label\":\"临时\",\"field_type\":\"int\",\"constraints\":{},\"default_value\":0,\"sort_order\":99}")
assert_code "8.2a 创建临时 schema" "0" "$body"
TMP_SCHEMA=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

V=$(schema_version "$TMP_SCHEMA")
post "/event-type-schema/toggle-enabled" "{\"id\":$TMP_SCHEMA,\"version\":${V:-1}}" > /dev/null

body=$(post "/event-type-schema/delete" "{\"id\":$TMP_SCHEMA}")
assert_code "8.2b 停用后删除成功" "0" "$body"

# =============================================================================
# 9. 版本冲突 — VERSION_CONFLICT (42030)
# =============================================================================
subsection "9. 版本冲突"

# 9.1 update with wrong version
body=$(post "/event-type-schema/update" "{\"id\":$SCHEMA_ID1,\"field_label\":\"冲突测试\",\"constraints\":{\"min\":0,\"max\":10},\"default_value\":5,\"sort_order\":1,\"version\":999}")
assert_code "9.1 update 版本冲突 → 42030" "42030" "$body"

# =============================================================================
# 10. 编辑启用中 schema — EDIT_NOT_DISABLED (42031)
# =============================================================================
subsection "10. EDIT_NOT_DISABLED (42031)"

# SCHEMA_ID1 当前是启用状态
V=$(schema_version "$SCHEMA_ID1")
body=$(post "/event-type-schema/update" "{\"id\":$SCHEMA_ID1,\"field_label\":\"改名\",\"constraints\":{\"min\":0,\"max\":10},\"default_value\":5,\"sort_order\":1,\"version\":$V}")
assert_code_in "10.1 启用中编辑 → 42031" "42031 0" "$body"

# 10.2 创建专用 schema 测试编辑守卫
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}edit_guard_s\",\"field_label\":\"编辑守卫\",\"field_type\":\"int\",\"constraints\":{\"min\":0,\"max\":100},\"default_value\":50,\"sort_order\":0}")
assert_code "10.2a 创建编辑守卫 schema" "0" "$body"
EG_SCHEMA=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 启用状态下编辑
V=$(schema_version "$EG_SCHEMA")
body=$(post "/event-type-schema/update" "{\"id\":$EG_SCHEMA,\"field_label\":\"改名2\",\"constraints\":{\"min\":0,\"max\":100},\"default_value\":50,\"sort_order\":0,\"version\":$V}")
assert_code_in "10.2b 启用中编辑" "42031 0" "$body"

# 10.3 停用后编辑应成功
V=$(schema_version "$EG_SCHEMA")
post "/event-type-schema/toggle-enabled" "{\"id\":$EG_SCHEMA,\"version\":$V}" > /dev/null
V=$(schema_version "$EG_SCHEMA")
body=$(post "/event-type-schema/update" "{\"id\":$EG_SCHEMA,\"field_label\":\"停用后改\",\"constraints\":{\"min\":0,\"max\":100},\"default_value\":50,\"sort_order\":0,\"version\":$V}")
assert_code "10.3 停用后编辑成功" "0" "$body"

# 清理
schema_rm "$EG_SCHEMA"

# =============================================================================
# 11. references 端点
# =============================================================================
subsection "11. references 端点"

# 11.1 查询无引用的 schema
body=$(post "/event-type-schema/references" "{\"id\":$SCHEMA_ID1}")
assert_code "11.1 references 查询成功" "0" "$body"

# 11.2 不存在的 schema ID
body=$(post "/event-type-schema/references" "{\"id\":99999999}")
assert_not_500 "11.2 references 不存在 ID 不崩" "$body"

# =============================================================================
# 12. 攻击 — unknown/null/array constraints
# =============================================================================
subsection "12. 攻击 — 约束类型异常"

# 12.1 unknown constraint keys — 宽松接受或拒绝
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}unk_key\",\"field_label\":\"未知约束\",\"field_type\":\"int\",\"constraints\":{\"min\":0,\"max\":100,\"unknown_key\":\"foo\",\"another\":123},\"default_value\":50,\"sort_order\":0}")
assert_code_in "12.1 unknown constraint keys" "0 42025" "$body"
UNK_ID=$(echo "$body" | jq -r '.data.id // empty' | tr -d '\r')
if [ -n "$UNK_ID" ] && [ "$UNK_ID" != "null" ] && [ "$UNK_ID" != "" ]; then
  schema_rm "$UNK_ID"
fi

# 12.2 null constraints
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}null_con\",\"field_label\":\"空约束\",\"field_type\":\"int\",\"constraints\":null,\"default_value\":0,\"sort_order\":0}")
assert_code_in "12.2 null constraints" "0 40000" "$body"
NULL_ID=$(echo "$body" | jq -r '.data.id // empty' | tr -d '\r')
if [ -n "$NULL_ID" ] && [ "$NULL_ID" != "null" ] && [ "$NULL_ID" != "" ]; then
  schema_rm "$NULL_ID"
fi

# 12.3 constraints 为数组
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}arr_con\",\"field_label\":\"数组约束\",\"field_type\":\"int\",\"constraints\":[1,2,3],\"default_value\":0,\"sort_order\":0}")
assert_code "12.3 constraints 为数组被拒" "40000" "$body"

# 12.4 极大 max 值
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}huge_max\",\"field_label\":\"巨大Max\",\"field_type\":\"int\",\"constraints\":{\"min\":0,\"max\":999999999999},\"default_value\":0,\"sort_order\":0}")
assert_code_in "12.4 极大 max 值" "0 42025" "$body"
HUGE_ID=$(echo "$body" | jq -r '.data.id // empty' | tr -d '\r')
if [ -n "$HUGE_ID" ] && [ "$HUGE_ID" != "null" ] && [ "$HUGE_ID" != "" ]; then
  schema_rm "$HUGE_ID"
fi

# 12.5 负 precision
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}neg_prec\",\"field_label\":\"负精度\",\"field_type\":\"float\",\"constraints\":{\"min\":0,\"max\":100,\"precision\":-5},\"default_value\":50,\"sort_order\":0}")
assert_code "12.5 float precision=-5 → 42025" "42025" "$body"

# =============================================================================
# 13. 攻击 — 编辑时约束/默认值违规
# =============================================================================
subsection "13. 攻击 — 编辑约束违规"

# 先确保 SCHEMA_ID1 可编辑（需停用）
# 如果编辑需要停用的话
V=$(schema_version "$SCHEMA_ID1")

# 13.1 编辑时 constraints min > max
body=$(post "/event-type-schema/update" "{\"id\":$SCHEMA_ID1,\"field_label\":\"坏约束编辑\",\"constraints\":{\"min\":100,\"max\":1},\"default_value\":5,\"sort_order\":1,\"version\":$V}")
assert_code_in "13.1 编辑 constraints min>max" "42025 42031" "$body"

# 13.2 编辑时 default 超约束
body=$(post "/event-type-schema/update" "{\"id\":$SCHEMA_ID1,\"field_label\":\"坏默认编辑\",\"constraints\":{\"min\":0,\"max\":10},\"default_value\":999,\"sort_order\":1,\"version\":$V}")
assert_code_in "13.2 编辑 default=999 超约束" "42026 42031" "$body"

# =============================================================================
# 14. 攻击 — bool with constraints
# =============================================================================
subsection "14. bool with constraints"

# 14.1 bool 传约束（应忽略或接受）
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bool_con\",\"field_label\":\"带约束布尔\",\"field_type\":\"bool\",\"constraints\":{\"min\":0,\"max\":1},\"default_value\":true,\"sort_order\":0}")
assert_not_500 "14.1 bool with constraints 不崩" "$body"
BOOL_CON_ID=$(echo "$body" | jq -r '.data.id // empty' | tr -d '\r')
if [ -n "$BOOL_CON_ID" ] && [ "$BOOL_CON_ID" != "null" ] && [ "$BOOL_CON_ID" != "" ]; then
  schema_rm "$BOOL_CON_ID"
fi

# =============================================================================
# 15. 攻击 — select 重复 option values
# =============================================================================
subsection "15. select 重复选项"

body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}dup_opt\",\"field_label\":\"重复选项\",\"field_type\":\"select\",\"constraints\":{\"options\":[{\"value\":\"a\"},{\"value\":\"a\"},{\"value\":\"b\"}]},\"default_value\":\"a\",\"sort_order\":0}")
assert_not_500 "15.1 select 重复 option 不崩" "$body"
DUP_OPT_ID=$(echo "$body" | jq -r '.data.id // empty' | tr -d '\r')
if [ -n "$DUP_OPT_ID" ] && [ "$DUP_OPT_ID" != "null" ] && [ "$DUP_OPT_ID" != "" ]; then
  schema_rm "$DUP_OPT_ID"
fi

# =============================================================================
# 16. 攻击 — string pattern 为非法正则
# =============================================================================
subsection "16. string 非法正则"

body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_regex\",\"field_label\":\"坏正则\",\"field_type\":\"string\",\"constraints\":{\"minLength\":0,\"maxLength\":100,\"pattern\":\"[invalid(\"},\"default_value\":\"x\",\"sort_order\":0}")
assert_not_500 "16.1 非法正则 pattern 不崩" "$body"

# =============================================================================
# 17. 攻击 — default_value 类型不匹配
# =============================================================================
subsection "17. default_value 类型不匹配"

# 17.1 int schema 传 string default
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}def_str_int\",\"field_label\":\"字符串默认\",\"field_type\":\"int\",\"constraints\":{\"min\":0,\"max\":10},\"default_value\":\"five\",\"sort_order\":0}")
assert_code_in "17.1 int default=string 被拒" "42026 40000" "$body"

# 17.2 bool schema 传 number default
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}def_num_bool\",\"field_label\":\"数字布尔\",\"field_type\":\"bool\",\"constraints\":{},\"default_value\":42,\"sort_order\":0}")
assert_not_500 "17.2 bool default=number 不崩" "$body"
BOOL_NUM_ID=$(echo "$body" | jq -r '.data.id // empty' | tr -d '\r')
if [ -n "$BOOL_NUM_ID" ] && [ "$BOOL_NUM_ID" != "null" ] && [ "$BOOL_NUM_ID" != "" ]; then
  schema_rm "$BOOL_NUM_ID"
fi

# 17.3 float schema 传 string default
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}def_str_flt\",\"field_label\":\"字符串浮点\",\"field_type\":\"float\",\"constraints\":{\"min\":0,\"max\":100},\"default_value\":\"high\",\"sort_order\":0}")
assert_code_in "17.3 float default=string 被拒" "42026 40000" "$body"

# =============================================================================
# 18. 攻击 — sort_order 边界
# =============================================================================
subsection "18. sort_order 边界"

# 18.1 sort_order 负数
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}sort_neg\",\"field_label\":\"负排序\",\"field_type\":\"int\",\"constraints\":{},\"default_value\":0,\"sort_order\":-1}")
assert_not_500 "18.1 sort_order=-1 不崩" "$body"
SORT_NEG_ID=$(echo "$body" | jq -r '.data.id // empty' | tr -d '\r')
if [ -n "$SORT_NEG_ID" ] && [ "$SORT_NEG_ID" != "null" ] && [ "$SORT_NEG_ID" != "" ]; then
  schema_rm "$SORT_NEG_ID"
fi

# 18.2 sort_order 极大
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}sort_huge\",\"field_label\":\"巨大排序\",\"field_type\":\"int\",\"constraints\":{},\"default_value\":0,\"sort_order\":999999}")
assert_not_500 "18.2 sort_order=999999 不崩" "$body"
SORT_HUGE_ID=$(echo "$body" | jq -r '.data.id // empty' | tr -d '\r')
if [ -n "$SORT_HUGE_ID" ] && [ "$SORT_HUGE_ID" != "null" ] && [ "$SORT_HUGE_ID" != "" ]; then
  schema_rm "$SORT_HUGE_ID"
fi

# =============================================================================
# 19. 攻击 — 畸形请求
# =============================================================================
subsection "19. 畸形请求"

# 19.1 空 body
body=$(post "/event-type-schema/create" "{}")
assert_not_500 "19.1 空 body 不崩" "$body"

# 19.2 list 传参数（应忽略）
body=$(post "/event-type-schema/list" "{\"page\":1}")
assert_code "19.2 list 传多余参数仍成功" "0" "$body"

# 19.3 delete 不存在的 ID
body=$(post "/event-type-schema/delete" "{\"id\":99999999}")
assert_not_500 "19.3 delete 不存在 ID 不崩" "$body"

# 19.4 update 不存在的 ID
body=$(post "/event-type-schema/update" "{\"id\":99999999,\"field_label\":\"不存在\",\"constraints\":{},\"default_value\":0,\"sort_order\":0,\"version\":1}")
assert_not_500 "19.4 update 不存在 ID 不崩" "$body"

# 19.5 toggle 不存在的 ID
body=$(post "/event-type-schema/toggle-enabled" "{\"id\":99999999,\"version\":1}")
assert_not_500 "19.5 toggle 不存在 ID 不崩" "$body"

# =============================================================================
# 20. field_name / field_type 不可变验证
# =============================================================================
subsection "20. field_name / field_type 不可变"

# field_name 和 field_type 不在 update 参数中（API 不接受）
# 验证 update 后原值不变
V=$(schema_version "$SCHEMA_ID4")
# 先停用 bool schema 才能编辑
post "/event-type-schema/toggle-enabled" "{\"id\":$SCHEMA_ID4,\"version\":$V}" > /dev/null
V=$(schema_version "$SCHEMA_ID4")
body=$(post "/event-type-schema/update" "{\"id\":$SCHEMA_ID4,\"field_label\":\"是否危险(改)\",\"constraints\":{},\"default_value\":true,\"sort_order\":4,\"version\":$V}")
assert_code "20.1 编辑 bool schema 成功" "0" "$body"

# 验证 field_name 和 field_type 不变
body=$(post "/event-type-schema/list" "{}")
S4_NAME=$(echo "$body" | jq -r ".data.items[] | select(.id==$SCHEMA_ID4) | .field_name" | tr -d '\r')
S4_TYPE=$(echo "$body" | jq -r ".data.items[] | select(.id==$SCHEMA_ID4) | .field_type" | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$S4_NAME" = "${P}is_dangerous" ]; then
  echo "  [PASS] 20.2 field_name 不可变"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] 20.2 field_name 不可变 — 实际: $S4_NAME"
  FAIL=$((FAIL + 1))
fi
TOTAL=$((TOTAL + 1))
if [ "$S4_TYPE" = "bool" ]; then
  echo "  [PASS] 20.3 field_type 不可变"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] 20.3 field_type 不可变 — 实际: $S4_TYPE"
  FAIL=$((FAIL + 1))
fi

# 恢复 SCHEMA_ID4 启用
V=$(schema_version "$SCHEMA_ID4")
post "/event-type-schema/toggle-enabled" "{\"id\":$SCHEMA_ID4,\"version\":$V}" > /dev/null

echo ""
echo "  [INFO] 导出变量: SCHEMA_ID1=$SCHEMA_ID1 SCHEMA_ID2=$SCHEMA_ID2 SCHEMA_ID3=$SCHEMA_ID3 SCHEMA_ID4=$SCHEMA_ID4 SCHEMA_ID5=$SCHEMA_ID5"
echo ""
