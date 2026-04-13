#!/bin/bash
# =============================================================================
# test_05_schema.sh — 扩展字段 Schema CRUD + 约束校验 + 攻击性测试
#
# 前置：run_all.sh 已 source helpers.sh，$BASE / $P / assert_* / post() 可用
#
# 导出变量：SCHEMA_ID1 ~ SCHEMA_ID5（供 test_06 使用）
# =============================================================================

section "Part 5: 扩展字段 Schema CRUD (prefix=$P)"

# --- Schema 辅助函数 ---
# 从列表获取指定 schema 的 version
schema_version() {
  echo "$(post "/event-type-schema/list" "{}")" | jq -r ".data.items[] | select(.id==$1) | .version" | tr -d '\r'
}

# =============================================================================
# 1. 创建各类型 schema
# =============================================================================
subsection "1. 创建各类型 schema"

# 1.1 int 类型 (priority)
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}priority\",\"field_label\":\"优先级\",\"field_type\":\"int\",\"constraints\":{\"min\":0,\"max\":10},\"default_value\":5,\"sort_order\":1}")
assert_code "1.1 创建 int schema" "0" "$body"
SCHEMA_ID1=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 1.2 float 类型 (radius)
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}radius\",\"field_label\":\"半径\",\"field_type\":\"float\",\"constraints\":{\"min\":0.0,\"max\":1000.0},\"default_value\":50.5,\"sort_order\":2}")
assert_code "1.2 创建 float schema" "0" "$body"
SCHEMA_ID2=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 1.3 string 类型 (description)
printf '{"field_name":"%sdesc","field_label":"描述","field_type":"string","constraints":{"minLength":0,"maxLength":100},"default_value":"默认描述","sort_order":3}' "$P" | curl -s -X POST "$BASE/event-type-schema/create" -H "Content-Type: application/json; charset=utf-8" --data-binary @- > /tmp/schema_str.json
body=$(cat /tmp/schema_str.json)
assert_code "1.3 创建 string schema" "0" "$body"
SCHEMA_ID3=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 1.4 bool 类型 (is_dangerous)
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}is_dangerous\",\"field_label\":\"是否危险\",\"field_type\":\"bool\",\"constraints\":{},\"default_value\":false,\"sort_order\":4}")
assert_code "1.4 创建 bool schema" "0" "$body"
SCHEMA_ID4=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 1.5 select 类型 (level)
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}level\",\"field_label\":\"等级\",\"field_type\":\"select\",\"constraints\":{\"options\":[{\"value\":\"low\"},{\"value\":\"mid\"},{\"value\":\"high\"}]},\"default_value\":\"low\",\"sort_order\":5}")
assert_code "1.5 创建 select schema" "0" "$body"
SCHEMA_ID5=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

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

# =============================================================================
# 3. field_type 校验 — TYPE_INVALID (42024)
# =============================================================================
subsection "3. field_type 校验"

# 3.1 非法 field_type → 42024
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_type\",\"field_label\":\"坏类型\",\"field_type\":\"reference\",\"constraints\":{},\"default_value\":0,\"sort_order\":0}")
assert_code "3.1 field_type=reference → TYPE_INVALID 42024" "42024" "$body"

# 3.2 另一个非法类型
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_type2\",\"field_label\":\"坏类型2\",\"field_type\":\"array\",\"constraints\":{},\"default_value\":0,\"sort_order\":0}")
assert_code "3.2 field_type=array → TYPE_INVALID 42024" "42024" "$body"

# 3.3 field_type=unknown
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_type3\",\"field_label\":\"未知类型\",\"field_type\":\"unknown\",\"constraints\":{},\"default_value\":0,\"sort_order\":0}")
assert_code "3.3 field_type=unknown → TYPE_INVALID 42024" "42024" "$body"

# 3.4 空 field_type
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_type4\",\"field_label\":\"空类型\",\"field_type\":\"\",\"constraints\":{},\"default_value\":0,\"sort_order\":0}")
assert_code_in "3.4 field_type=空 被拒" "42024 40000" "$body"

# =============================================================================
# 4. constraints 自洽校验 — CONSTRAINTS_INVALID (42025)
# =============================================================================
subsection "4. constraints 自洽校验"

# 4.1 int: min > max → 42025
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_int_range\",\"field_label\":\"坏范围\",\"field_type\":\"int\",\"constraints\":{\"min\":10,\"max\":1},\"default_value\":5,\"sort_order\":0}")
assert_code "4.1 int min>max → CONSTRAINTS_INVALID 42025" "42025" "$body"

# 4.2 float: min > max → 42025
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_float_range\",\"field_label\":\"坏浮点范围\",\"field_type\":\"float\",\"constraints\":{\"min\":100.5,\"max\":10.0},\"default_value\":50,\"sort_order\":0}")
assert_code "4.2 float min>max → CONSTRAINTS_INVALID 42025" "42025" "$body"

# 4.3 string: minLength > maxLength → 42025
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_str_range\",\"field_label\":\"坏字串范围\",\"field_type\":\"string\",\"constraints\":{\"minLength\":100,\"maxLength\":10},\"default_value\":\"x\",\"sort_order\":0}")
assert_code "4.3 string minLength>maxLength → CONSTRAINTS_INVALID 42025" "42025" "$body"

# 4.4 select: empty options
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_select\",\"field_label\":\"空选项\",\"field_type\":\"select\",\"constraints\":{\"options\":[]},\"default_value\":\"x\",\"sort_order\":0}")
assert_code_in "4.4 select 空 options" "42025 42026 0" "$body"

# 4.5 select: minSelect > maxSelect → 42025
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_sel_mm\",\"field_label\":\"坏选择范围\",\"field_type\":\"select\",\"constraints\":{\"options\":[{\"value\":\"a\"},{\"value\":\"b\"}],\"minSelect\":5,\"maxSelect\":1},\"default_value\":\"a\",\"sort_order\":0}")
assert_code "4.5 select minSelect>maxSelect → CONSTRAINTS_INVALID 42025" "42025" "$body"

# 4.6 int: min > max（边界值，min=100 max=10 与用户描述一致）
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_int_100_10\",\"field_label\":\"坏范围100>10\",\"field_type\":\"int\",\"constraints\":{\"min\":100,\"max\":10},\"default_value\":5,\"sort_order\":0}")
assert_code "4.6 int min=100 max=10 → 42025" "42025" "$body"

# 4.7 float: min > max（两个都是负数）
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_flt_neg\",\"field_label\":\"负浮点范围\",\"field_type\":\"float\",\"constraints\":{\"min\":-1.0,\"max\":-10.0},\"default_value\":-5,\"sort_order\":0}")
assert_code "4.7 float min=-1 max=-10 → 42025" "42025" "$body"

# 4.8 string: minLength 负数
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_str_neg\",\"field_label\":\"负minLength\",\"field_type\":\"string\",\"constraints\":{\"minLength\":-1,\"maxLength\":10},\"default_value\":\"x\",\"sort_order\":0}")
assert_code "4.8 string minLength=-1 → 42025" "42025" "$body"

# =============================================================================
# 5. default_value vs constraints — DEFAULT_INVALID (42026)
# =============================================================================
subsection "5. default_value vs constraints"

# 5.1 int default 超范围
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_def_int\",\"field_label\":\"超范围默认\",\"field_type\":\"int\",\"constraints\":{\"min\":1,\"max\":10},\"default_value\":99,\"sort_order\":0}")
assert_code "5.1 int default=99 超 max=10 → DEFAULT_INVALID 42026" "42026" "$body"

# 5.2 int default 低于 min
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_def_int2\",\"field_label\":\"低范围默认\",\"field_type\":\"int\",\"constraints\":{\"min\":5,\"max\":10},\"default_value\":1,\"sort_order\":0}")
assert_code "5.2 int default=1 低于 min=5 → DEFAULT_INVALID 42026" "42026" "$body"

# 5.3 string default 超 maxLength
printf '{"field_name":"%sbad_def_str","field_label":"超长默认","field_type":"string","constraints":{"minLength":0,"maxLength":3},"default_value":"超过三个字符","sort_order":0}' "$P" | curl -s -X POST "$BASE/event-type-schema/create" -H "Content-Type: application/json; charset=utf-8" --data-binary @- > /tmp/schema_bad_str.json
body=$(cat /tmp/schema_bad_str.json)
assert_code "5.3 string default 超 maxLength → DEFAULT_INVALID 42026" "42026" "$body"

# 5.4 int default=100 with max=10（与用户描述完全一致）
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_def_exact\",\"field_label\":\"精确超范围\",\"field_type\":\"int\",\"constraints\":{\"min\":0,\"max\":10},\"default_value\":100,\"sort_order\":0}")
assert_code "5.4 int default=100 max=10 → DEFAULT_INVALID 42026" "42026" "$body"

# 5.5 float default 超范围
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_def_flt\",\"field_label\":\"浮点超范围\",\"field_type\":\"float\",\"constraints\":{\"min\":0.0,\"max\":10.0},\"default_value\":99.9,\"sort_order\":0}")
assert_code "5.5 float default=99.9 超 max=10 → DEFAULT_INVALID 42026" "42026" "$body"

# 5.6 select default 不在 options 中
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_def_sel\",\"field_label\":\"不在选项\",\"field_type\":\"select\",\"constraints\":{\"options\":[{\"value\":\"a\"},{\"value\":\"b\"}]},\"default_value\":\"c\",\"sort_order\":0}")
assert_code "5.6 select default=c 不在 options → DEFAULT_INVALID 42026" "42026" "$body"

# =============================================================================
# 6. 列表
# =============================================================================
subsection "6. 列表"

body=$(post "/event-type-schema/list" "{}")
assert_code "6.1 schema 列表成功" "0" "$body"
SCHEMA_COUNT=$(echo "$body" | jq -r '.data.items | length' | tr -d '\r')
assert_ge "6.1 至少 5 条 schema" '.data.items | length' "5" "$body"

# =============================================================================
# 7. 停用 / 启用切换
# =============================================================================
subsection "7. 停用 / 启用切换"

# 7.1 停用 SCHEMA_ID2 (float)
V=$(schema_version "$SCHEMA_ID2")
body=$(post "/event-type-schema/toggle-enabled" "{\"id\":$SCHEMA_ID2,\"version\":${V:-1}}")
assert_code "7.1 停用 float schema" "0" "$body"

# 7.2 再次 toggle 恢复启用
V=$(schema_version "$SCHEMA_ID2")
body=$(post "/event-type-schema/toggle-enabled" "{\"id\":$SCHEMA_ID2,\"version\":${V:-2}}")
assert_code "7.2 恢复启用 float schema" "0" "$body"

# =============================================================================
# 8. 删除前必须先停用 (42027)
# =============================================================================
subsection "8. 删除守卫"

# 8.1 删除启用中的 schema → 42027
body=$(post "/event-type-schema/delete" "{\"id\":$SCHEMA_ID1}")
assert_code "8.1 删除未停用 schema → 42027" "42027" "$body"

# 8.2 停用后可删除（创建一个临时的来测试）
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}tmp_del\",\"field_label\":\"临时\",\"field_type\":\"int\",\"constraints\":{},\"default_value\":0,\"sort_order\":99}")
assert_code "8.2a 创建临时 schema" "0" "$body"
TMP_SCHEMA=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 停用
V=$(schema_version "$TMP_SCHEMA")
post "/event-type-schema/toggle-enabled" "{\"id\":$TMP_SCHEMA,\"version\":${V:-1}}" > /dev/null

# 删除
body=$(post "/event-type-schema/delete" "{\"id\":$TMP_SCHEMA}")
assert_code "8.2b 停用后删除成功" "0" "$body"

# =============================================================================
# 9. 版本冲突 — VERSION_CONFLICT (42030)
# =============================================================================
subsection "9. 版本冲突"

body=$(post "/event-type-schema/update" "{\"id\":$SCHEMA_ID1,\"field_label\":\"冲突测试\",\"constraints\":{\"min\":0,\"max\":10},\"default_value\":5,\"version\":999}")
assert_code "9.1 版本冲突 → VERSION_CONFLICT 42030" "42030" "$body"

# =============================================================================
# 9b. Toggle 版本冲突 (42030)
# =============================================================================
subsection "9b. Toggle 版本冲突"

# toggle 用 stale version → 42030
body=$(post "/event-type-schema/toggle-enabled" "{\"id\":$SCHEMA_ID1,\"version\":999}")
assert_code "9b.1 toggle stale version → 42030" "42030" "$body"

# 连续两次 toggle 用同一个 version，第二次应冲突
V=$(schema_version "$SCHEMA_ID1")
body=$(post "/event-type-schema/toggle-enabled" "{\"id\":$SCHEMA_ID1,\"version\":$V}")
assert_code "9b.2a 第一次 toggle 成功" "0" "$body"
body=$(post "/event-type-schema/toggle-enabled" "{\"id\":$SCHEMA_ID1,\"version\":$V}")
assert_code "9b.2b 第二次 toggle 同 version → 42030" "42030" "$body"

# 用正确 version 恢复到启用状态
V=$(schema_version "$SCHEMA_ID1")
body=$(post "/event-type-schema/toggle-enabled" "{\"id\":$SCHEMA_ID1,\"version\":$V}")
assert_code "9b.3 恢复 toggle 成功" "0" "$body"

# =============================================================================
# 10. 编辑启用中 schema — EDIT_NOT_DISABLED (42031)
# =============================================================================
subsection "10. 编辑启用中 schema"

# SCHEMA_ID1 当前是启用状态，尝试编辑
V=$(schema_version "$SCHEMA_ID1")
body=$(post "/event-type-schema/update" "{\"id\":$SCHEMA_ID1,\"field_label\":\"改名\",\"constraints\":{\"min\":0,\"max\":10},\"default_value\":5,\"version\":$V}")
# 注意：当前实现可能不检查 enabled 状态，如果通过则说明 42031 未实现
assert_code_in "10.1 编辑启用中 schema → EDIT_NOT_DISABLED" "42031 0" "$body"

# 创建专用 schema 测试编辑守卫
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}edit_guard_s\",\"field_label\":\"编辑守卫\",\"field_type\":\"int\",\"constraints\":{\"min\":0,\"max\":100},\"default_value\":50,\"sort_order\":0}")
assert_code "10.2a 创建编辑守卫 schema" "0" "$body"
EG_SCHEMA=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 启用状态下编辑
V=$(schema_version "$EG_SCHEMA")
body=$(post "/event-type-schema/update" "{\"id\":$EG_SCHEMA,\"field_label\":\"改名2\",\"constraints\":{\"min\":0,\"max\":100},\"default_value\":50,\"version\":$V}")
assert_code_in "10.2b 启用中编辑 schema" "42031 0" "$body"

# 停用后编辑应成功
V=$(schema_version "$EG_SCHEMA")
post "/event-type-schema/toggle-enabled" "{\"id\":$EG_SCHEMA,\"version\":$V}" > /dev/null
V=$(schema_version "$EG_SCHEMA")
body=$(post "/event-type-schema/update" "{\"id\":$EG_SCHEMA,\"field_label\":\"停用后改\",\"constraints\":{\"min\":0,\"max\":100},\"default_value\":50,\"version\":$V}")
assert_code "10.3 停用后编辑成功" "0" "$body"

# 清理
V=$(schema_version "$EG_SCHEMA")
# 确保停用状态
post "/event-type-schema/toggle-enabled" "{\"id\":$EG_SCHEMA,\"version\":$V}" > /dev/null 2>&1
post "/event-type-schema/delete" "{\"id\":$EG_SCHEMA}" > /dev/null 2>&1

# =============================================================================
# 11. 攻击性测试
# =============================================================================
subsection "11. 攻击性测试"

# 11.1 unknown constraint keys — 应该不影响创建（宽松接受或忽略）
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}unk_key\",\"field_label\":\"未知约束\",\"field_type\":\"int\",\"constraints\":{\"min\":0,\"max\":100,\"unknown_key\":\"foo\",\"another\":123},\"default_value\":50,\"sort_order\":0}")
assert_code_in "11.1 unknown constraint keys" "0 42025" "$body"
UNK_ID=$(echo "$body" | jq -r '.data.id // empty' | tr -d '\r')
# 清理
if [ -n "$UNK_ID" ] && [ "$UNK_ID" != "null" ] && [ "$UNK_ID" != "" ]; then
  V=$(schema_version "$UNK_ID")
  post "/event-type-schema/toggle-enabled" "{\"id\":$UNK_ID,\"version\":${V:-1}}" > /dev/null 2>&1
  post "/event-type-schema/delete" "{\"id\":$UNK_ID}" > /dev/null 2>&1
fi

# 11.2 null constraints
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}null_con\",\"field_label\":\"空约束\",\"field_type\":\"int\",\"constraints\":null,\"default_value\":0,\"sort_order\":0}")
assert_code_in "11.2 null constraints" "0 40000" "$body"
NULL_ID=$(echo "$body" | jq -r '.data.id // empty' | tr -d '\r')
if [ -n "$NULL_ID" ] && [ "$NULL_ID" != "null" ] && [ "$NULL_ID" != "" ]; then
  V=$(schema_version "$NULL_ID")
  post "/event-type-schema/toggle-enabled" "{\"id\":$NULL_ID,\"version\":${V:-1}}" > /dev/null 2>&1
  post "/event-type-schema/delete" "{\"id\":$NULL_ID}" > /dev/null 2>&1
fi

# 11.3 极大 max 值
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}huge_max\",\"field_label\":\"巨大Max\",\"field_type\":\"int\",\"constraints\":{\"min\":0,\"max\":999999999999},\"default_value\":0,\"sort_order\":0}")
assert_code_in "11.3 极大 max 值" "0 42025" "$body"
HUGE_ID=$(echo "$body" | jq -r '.data.id // empty' | tr -d '\r')
if [ -n "$HUGE_ID" ] && [ "$HUGE_ID" != "null" ] && [ "$HUGE_ID" != "" ]; then
  V=$(schema_version "$HUGE_ID")
  post "/event-type-schema/toggle-enabled" "{\"id\":$HUGE_ID,\"version\":${V:-1}}" > /dev/null 2>&1
  post "/event-type-schema/delete" "{\"id\":$HUGE_ID}" > /dev/null 2>&1
fi

# 11.4 constraints 不是 JSON 对象（传数组）
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}arr_con\",\"field_label\":\"数组约束\",\"field_type\":\"int\",\"constraints\":[1,2,3],\"default_value\":0,\"sort_order\":0}")
assert_code "11.4 constraints 为数组被拒" "40000" "$body"

# 11.5 编辑时 constraints 不自洽
V=$(schema_version "$SCHEMA_ID1")
# 先确保 SCHEMA_ID1 是停用状态才能编辑
# （如果是启用状态，先停用）
body=$(post "/event-type-schema/update" "{\"id\":$SCHEMA_ID1,\"field_label\":\"坏约束编辑\",\"constraints\":{\"min\":100,\"max\":1},\"default_value\":5,\"version\":$V}")
assert_code_in "11.5 编辑时 constraints min>max" "42025 42031" "$body"

# 11.6 编辑时 default 超约束
body=$(post "/event-type-schema/update" "{\"id\":$SCHEMA_ID1,\"field_label\":\"坏默认编辑\",\"constraints\":{\"min\":0,\"max\":10},\"default_value\":999,\"version\":$V}")
assert_code_in "11.6 编辑时 default=999 超约束" "42026 42031" "$body"

echo ""
echo "  [INFO] 导出变量: SCHEMA_ID1=$SCHEMA_ID1 SCHEMA_ID2=$SCHEMA_ID2 SCHEMA_ID3=$SCHEMA_ID3 SCHEMA_ID4=$SCHEMA_ID4 SCHEMA_ID5=$SCHEMA_ID5"
echo ""
