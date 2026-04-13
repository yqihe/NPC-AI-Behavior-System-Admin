#!/bin/bash
# =============================================================================
# test_06_event_schema.sh — 事件类型 x 扩展字段 Schema 跨模块集成测试
#
# 前置：run_all.sh 已 source helpers.sh，$BASE / $P / assert_* / post() 可用
#       test_04 导出: ET_ID1 ~ ET_ID4
#       test_05 导出: SCHEMA_ID1 ~ SCHEMA_ID5
# =============================================================================

section "Part 6: 事件类型 x 扩展字段 Schema 集成 (prefix=$P)"

# =============================================================================
# 1. 创建带扩展字段的事件类型
# =============================================================================
subsection "1. 创建带扩展字段"

# 1.1 正常 — 扩展字段 priority=8（int schema: min=0, max=10）
body=$(post "/event-types/create" "{\"name\":\"${P}ext_evt1\",\"display_name\":\"扩展事件1\",\"perception_mode\":\"visual\",\"default_severity\":60,\"default_ttl\":10,\"range\":100,\"extensions\":{\"${P}priority\":8}}")
assert_code "1.1 创建带 int 扩展" "0" "$body"
EXT_ET1=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 1.2 正常 — 多个扩展字段
body=$(post "/event-types/create" "{\"name\":\"${P}ext_evt2\",\"display_name\":\"扩展事件2\",\"perception_mode\":\"auditory\",\"default_severity\":40,\"default_ttl\":8,\"range\":200,\"extensions\":{\"${P}priority\":3,\"${P}is_dangerous\":true}}")
assert_code "1.2 创建带多个扩展" "0" "$body"
EXT_ET2=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 1.3 详情验证扩展字段值
body=$(et_detail "$EXT_ET1")
assert_code  "1.3 详情成功" "0" "$body"
assert_field "1.3 扩展 priority=8" ".data.config.${P}priority" "8" "$body"

# 1.4 详情包含 extension_schema 列表
SCHEMA_COUNT=$(echo "$body" | jq -r '.data.extension_schema | length' | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$SCHEMA_COUNT" -ge "1" ] 2>/dev/null; then
  echo "  [PASS] 1.4 extension_schema 非空 (=$SCHEMA_COUNT)"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] 1.4 extension_schema 应非空, 实际: $SCHEMA_COUNT"
  FAIL=$((FAIL + 1))
fi

# =============================================================================
# 2. 扩展字段值校验 (42007)
# =============================================================================
subsection "2. 扩展字段值约束校验"

# 2.1 int 超 max (priority max=10, 传 99)
body=$(post "/event-types/create" "{\"name\":\"${P}ext_bad_int\",\"display_name\":\"超范围\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100,\"extensions\":{\"${P}priority\":99}}")
assert_code "2.1 int 超 max → 42007" "42007" "$body"

# 2.2 int 低于 min (priority min=0, 传 -5)
body=$(post "/event-types/create" "{\"name\":\"${P}ext_bad_int2\",\"display_name\":\"低范围\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100,\"extensions\":{\"${P}priority\":-5}}")
assert_code "2.2 int 低于 min → 42007" "42007" "$body"

# 2.3 string 超 maxLength (desc maxLength=100, 传超长)
LONG_STR=$(printf 'X%.0s' {1..150})
printf '{"name":"%sext_bad_str","display_name":"超长描述","perception_mode":"visual","default_severity":50,"default_ttl":5,"range":100,"extensions":{"%sdesc":"%s"}}' "$P" "$P" "$LONG_STR" | curl -s -X POST "$BASE/event-types/create" -H "Content-Type: application/json; charset=utf-8" --data-binary @- > /tmp/ext_bad_str.json
body=$(cat /tmp/ext_bad_str.json)
assert_code "2.3 string 超 maxLength → 42007" "42007" "$body"

# 2.4 不存在的扩展字段 key → 42022
body=$(post "/event-types/create" "{\"name\":\"${P}ext_bad_key\",\"display_name\":\"不存在的key\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100,\"extensions\":{\"nonexistent_field\":1}}")
assert_code "2.4 不存在的扩展 key → 42022" "42022" "$body"

# 2.5 bool 类型传非 bool 值
body=$(post "/event-types/create" "{\"name\":\"${P}ext_bad_bool\",\"display_name\":\"坏布尔\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100,\"extensions\":{\"${P}is_dangerous\":\"not_bool\"}}")
assert_code "2.5 bool 传字符串 → 42007" "42007" "$body"

# =============================================================================
# 3. 禁用 schema 但已有事件类型引用该字段值
# =============================================================================
subsection "3. 禁用 schema + 已有值场景"

# 3.1 停用 priority schema（SCHEMA_ID1）
V=$(echo "$(post "/event-type-schema/list" "{}")" | jq -r ".data.items[] | select(.id==$SCHEMA_ID1) | .version" | tr -d '\r')
body=$(post "/event-type-schema/toggle-enabled" "{\"id\":$SCHEMA_ID1,\"version\":${V:-1}}")
assert_code "3.1 停用 priority schema" "0" "$body"

# 3.2 详情仍然展示已停用 schema 的值（灰色标注行为）
body=$(et_detail "$EXT_ET1")
assert_code  "3.2 详情仍然成功" "0" "$body"
assert_field "3.2 扩展 priority=8 仍在 config" ".data.config.${P}priority" "8" "$body"

# 3.3 extension_schema 列表应包含已停用的（因为 config 里有值）
HAS_DISABLED=$(echo "$body" | jq "[.data.extension_schema[] | select(.field_name==\"${P}priority\")] | length" | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$HAS_DISABLED" -ge "1" ] 2>/dev/null; then
  echo "  [PASS] 3.3 extension_schema 包含已停用的 priority"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] 3.3 extension_schema 应包含已停用 schema, 实际: $HAS_DISABLED"
  FAIL=$((FAIL + 1))
fi

# 3.4 停用后新创建的事件不能再用该字段 → 42022（因为缓存只有 enabled 的）
body=$(post "/event-types/create" "{\"name\":\"${P}ext_disabled\",\"display_name\":\"用停用字段\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100,\"extensions\":{\"${P}priority\":3}}")
assert_code "3.4 用停用 schema 创建 → 42022" "42022" "$body"

# 3.5 恢复启用 priority schema
V=$(echo "$(post "/event-type-schema/list" "{}")" | jq -r ".data.items[] | select(.id==$SCHEMA_ID1) | .version" | tr -d '\r')
body=$(post "/event-type-schema/toggle-enabled" "{\"id\":$SCHEMA_ID1,\"version\":${V:-1}}")
assert_code "3.5 恢复启用 priority schema" "0" "$body"

# =============================================================================
# 4. 无扩展字段的事件类型
# =============================================================================
subsection "4. 无扩展字段的事件类型"

# 4.1 不传 extensions 字段
body=$(post "/event-types/create" "{\"name\":\"${P}no_ext\",\"display_name\":\"无扩展\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "4.1 不传 extensions — 合法" "0" "$body"
NO_EXT_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 4.2 空 extensions map
body=$(post "/event-types/create" "{\"name\":\"${P}empty_ext\",\"display_name\":\"空扩展\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100,\"extensions\":{}}")
assert_code "4.2 空 extensions={} — 合法" "0" "$body"
EMPTY_EXT_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 4.3 验证无扩展事件详情 — config 只包含系统字段
body=$(et_detail "$NO_EXT_ID")
assert_code  "4.3 无扩展详情成功" "0" "$body"
assert_field "4.3 有 perception_mode" '.data.config.perception_mode' "visual" "$body"
assert_field "4.3 severity=50" '.data.config.default_severity' "50" "$body"

# =============================================================================
# 5. 编辑事件类型中的扩展字段
# =============================================================================
subsection "5. 编辑事件类型扩展字段"

# 5.1 编辑 EXT_ET1 修改扩展字段值
V=$(et_version "$EXT_ET1")
body=$(post "/event-types/update" "{\"id\":$EXT_ET1,\"display_name\":\"扩展事件1(改)\",\"perception_mode\":\"visual\",\"default_severity\":60,\"default_ttl\":10,\"range\":100,\"extensions\":{\"${P}priority\":2},\"version\":$V}")
assert_code "5.1 编辑扩展字段值" "0" "$body"

# 5.2 验证编辑后的值
body=$(et_detail "$EXT_ET1")
assert_field "5.2 priority 变为 2" ".data.config.${P}priority" "2" "$body"

# 5.3 编辑时扩展字段值超约束 → 42007
V=$(et_version "$EXT_ET1")
body=$(post "/event-types/update" "{\"id\":$EXT_ET1,\"display_name\":\"扩展事件1(再改)\",\"perception_mode\":\"visual\",\"default_severity\":60,\"default_ttl\":10,\"range\":100,\"extensions\":{\"${P}priority\":999},\"version\":$V}")
assert_code "5.3 编辑时扩展值超约束 → 42007" "42007" "$body"

echo ""
echo "  [INFO] 跨模块集成测试完成"
echo ""
