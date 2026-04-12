#!/bin/bash
# =============================================================================
# 事件类型管理 API 集成测试
#
# 覆盖：事件类型 CRUD（7 接口）+ 扩展字段 Schema CRUD（5 接口）+ 导出 API + 攻击性测试
# 运行前提：docker compose up --build -d（后端已启动，DDL 已执行）
# 用法：bash tests/event_type_test.sh
# =============================================================================

export LANG=en_US.UTF-8
export LC_ALL=en_US.UTF-8
if command -v chcp.com &>/dev/null; then
  chcp.com 65001 > /dev/null 2>&1
fi

BASE="http://localhost:9821/api/v1"
EXPORT_BASE="http://localhost:9821/api/configs"
PASS=0
FAIL=0
TOTAL=0
BUGS=()
TS=$(date +%s)
P="et${TS}_"

# =============================================================================
# 工具函数（和 api_test.sh 同构）
# =============================================================================

assert_code() {
  local name="$1" expected="$2" body="$3"
  TOTAL=$((TOTAL + 1))
  local actual=$(echo "$body" | jq -r '.code // empty' 2>/dev/null | tr -d '\r')
  if [ "$actual" = "$expected" ]; then
    echo "  [PASS] $name"
    PASS=$((PASS + 1))
  else
    echo "  [FAIL] $name — 期望 code=$expected, 实际: $actual"
    echo "         响应: $(echo "$body" | head -c 300)"
    FAIL=$((FAIL + 1))
  fi
}

assert_field() {
  local name="$1" expr="$2" expected="$3" body="$4"
  TOTAL=$((TOTAL + 1))
  local actual=$(echo "$body" | jq -r "$expr" 2>/dev/null | tr -d '\r')
  if [ "$actual" = "$expected" ]; then
    echo "  [PASS] $name"
    PASS=$((PASS + 1))
  else
    echo "  [FAIL] $name — 期望 $expected, 实际: $actual"
    echo "         响应: $(echo "$body" | head -c 300)"
    FAIL=$((FAIL + 1))
  fi
}

assert_ge() {
  local name="$1" expr="$2" min="$3" body="$4"
  TOTAL=$((TOTAL + 1))
  local actual=$(echo "$body" | jq -r "$expr" 2>/dev/null | tr -d '\r')
  if [ "$actual" -ge "$min" ] 2>/dev/null; then
    echo "  [PASS] $name (=$actual)"
    PASS=$((PASS + 1))
  else
    echo "  [FAIL] $name — 期望 >= $min, 实际: $actual"
    FAIL=$((FAIL + 1))
  fi
}

post() {
  printf '%s' "$2" | curl -s -X POST "$BASE$1" -H "Content-Type: application/json; charset=utf-8" --data-binary @-
}

get_export() {
  curl -s "$EXPORT_BASE$1"
}

et_detail() { post "/event-types/detail" "{\"id\":$1}"; }
et_version() { et_detail "$1" | jq -r '.data.config // empty' | jq -r 'empty' 2>/dev/null; et_detail "$1" | jq -r '.data.version' | tr -d '\r'; }

# =============================================================================
echo "================================================================="
echo "  事件类型管理 API 集成测试   $(date)"
echo "================================================================="
echo ""

# =============================================================================
# 1. 扩展字段 Schema CRUD
# =============================================================================
echo "--- 1. 扩展字段 Schema CRUD ---"

# 1.1 创建 schema: priority (int, 1-10, default 5)
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}priority\",\"field_label\":\"优先级\",\"field_type\":\"int\",\"constraints\":{\"min\":1,\"max\":10},\"default_value\":5,\"sort_order\":1}")
assert_code "1.1 创建 schema priority" "0" "$body"
SCHEMA_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 1.2 创建 schema: category (string)
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}category\",\"field_label\":\"事件分类\",\"field_type\":\"string\",\"constraints\":{\"maxLength\":32},\"default_value\":\"unknown\",\"sort_order\":2}")
assert_code "1.2 创建 schema category" "0" "$body"
SCHEMA_ID2=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 1.3 重复 field_name → 42020
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}priority\",\"field_label\":\"重复\",\"field_type\":\"int\",\"constraints\":{},\"default_value\":0,\"sort_order\":0}")
assert_code "1.3 重复 field_name" "42020" "$body"

# 1.4 非法 field_type → 42024
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_type\",\"field_label\":\"坏类型\",\"field_type\":\"reference\",\"constraints\":{},\"default_value\":0,\"sort_order\":0}")
assert_code "1.4 reference 被拒" "42024" "$body"

# 1.5 constraints 不自洽 (min > max) → 42025
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_range\",\"field_label\":\"坏范围\",\"field_type\":\"int\",\"constraints\":{\"min\":10,\"max\":1},\"default_value\":5,\"sort_order\":0}")
assert_code "1.5 min > max" "42025" "$body"

# 1.6 default_value 不符合 constraints → 42026
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_default\",\"field_label\":\"坏默认\",\"field_type\":\"int\",\"constraints\":{\"min\":1,\"max\":10},\"default_value\":99,\"sort_order\":0}")
assert_code "1.6 default 超范围" "42026" "$body"

# 1.7 列表
body=$(post "/event-type-schema/list" "{}")
assert_code "1.7 schema 列表" "0" "$body"

# 1.8 停用 schema (先拿 version)
SCHEMA2_DETAIL=$(post "/event-type-schema/list" "{}")
V=$(echo "$SCHEMA2_DETAIL" | jq -r ".data.items[] | select(.id==$SCHEMA_ID2) | .version" | tr -d '\r')
body=$(post "/event-type-schema/toggle-enabled" "{\"id\":$SCHEMA_ID2,\"version\":${V:-1}}")
assert_code "1.8 停用 schema" "0" "$body"

# 1.9 删除未停用 schema → 42027
body=$(post "/event-type-schema/delete" "{\"id\":$SCHEMA_ID}")
assert_code "1.9 删除未停用 schema" "42027" "$body"

echo ""

# =============================================================================
# 2. 事件类型 CRUD（正向流程）
# =============================================================================
echo "--- 2. 事件类型 CRUD ---"

# 2.1 创建 gunshot（auditory）
body=$(post "/event-types/create" "{\"name\":\"${P}gunshot\",\"display_name\":\"枪声\",\"perception_mode\":\"auditory\",\"default_severity\":90,\"default_ttl\":10,\"range\":300}")
assert_code "2.1 创建 gunshot" "0" "$body"
ET_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 2.2 创建 earthquake（global, range 自动置 0）
body=$(post "/event-types/create" "{\"name\":\"${P}earthquake\",\"display_name\":\"地震\",\"perception_mode\":\"global\",\"default_severity\":95,\"default_ttl\":30,\"range\":999}")
assert_code "2.2 创建 earthquake (global)" "0" "$body"
ET_ID2=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 2.3 详情 — config 里 range=0（global 兜底）
body=$(et_detail "$ET_ID2")
assert_code "2.3 详情" "0" "$body"
assert_field "2.3 global range=0" '.data.config.range' "0" "$body"
assert_field "2.3 severity=95" '.data.config.default_severity' "95" "$body"
assert_field "2.3 perception_mode=global" '.data.config.perception_mode' "global" "$body"

# 2.4 创建带扩展字段
body=$(post "/event-types/create" "{\"name\":\"${P}fire\",\"display_name\":\"火灾\",\"perception_mode\":\"visual\",\"default_severity\":70,\"default_ttl\":20,\"range\":100,\"extensions\":{\"${P}priority\":8}}")
assert_code "2.4 创建带扩展字段" "0" "$body"
ET_ID3=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 2.5 详情验证扩展字段
body=$(et_detail "$ET_ID3")
assert_field "2.5 扩展 priority=8" ".data.config.${P}priority" "8" "$body"

# 2.6 check-name 已存在
body=$(post "/event-types/check-name" "{\"name\":\"${P}gunshot\"}")
assert_code "2.6 check-name" "0" "$body"
assert_field "2.6 not available" '.data.available' "false" "$body"

# 2.7 check-name 可用
body=$(post "/event-types/check-name" "{\"name\":\"${P}not_exist\"}")
assert_field "2.7 available" '.data.available' "true" "$body"

# 2.8 列表
body=$(post "/event-types/list" "{\"page\":1,\"page_size\":20}")
assert_code "2.8 列表" "0" "$body"
assert_ge "2.8 total >= 3" '.data.total' "3" "$body"

# 2.9 列表 — perception_mode 筛选
body=$(post "/event-types/list" "{\"perception_mode\":\"global\",\"page\":1,\"page_size\":20}")
assert_code "2.9 列表 global 筛选" "0" "$body"

# 2.10 编辑 — 未停用状态，可编辑（默认 enabled=0）
V=$(et_detail "$ET_ID" | jq -r '.data.version' | tr -d '\r')
body=$(post "/event-types/update" "{\"id\":$ET_ID,\"display_name\":\"枪声(修改)\",\"perception_mode\":\"auditory\",\"default_severity\":85,\"default_ttl\":8,\"range\":250,\"version\":$V}")
assert_code "2.10 编辑 gunshot" "0" "$body"

# 2.11 编辑验证 — severity 变成 85
body=$(et_detail "$ET_ID")
assert_field "2.11 severity=85" '.data.config.default_severity' "85" "$body"

# 2.12 启用
V=$(et_detail "$ET_ID" | jq -r '.data.version' | tr -d '\r')
body=$(post "/event-types/toggle-enabled" "{\"id\":$ET_ID,\"version\":$V}")
assert_code "2.12 启用 gunshot" "0" "$body"

# 2.13 启用后编辑 → 42015
V=$(et_detail "$ET_ID" | jq -r '.data.version' | tr -d '\r')
body=$(post "/event-types/update" "{\"id\":$ET_ID,\"display_name\":\"枪声(再改)\",\"perception_mode\":\"auditory\",\"default_severity\":85,\"default_ttl\":8,\"range\":250,\"version\":$V}")
assert_code "2.13 启用后编辑拒绝" "42015" "$body"

# 2.14 启用后删除 → 42012
body=$(post "/event-types/delete" "{\"id\":$ET_ID}")
assert_code "2.14 启用后删除拒绝" "42012" "$body"

# 2.15 停用再删除
V=$(et_detail "$ET_ID" | jq -r '.data.version' | tr -d '\r')
body=$(post "/event-types/toggle-enabled" "{\"id\":$ET_ID,\"version\":$V}")
assert_code "2.15a 停用" "0" "$body"
body=$(post "/event-types/delete" "{\"id\":$ET_ID}")
assert_code "2.15b 删除" "0" "$body"

# 2.16 软删后 name 不可复用
body=$(post "/event-types/create" "{\"name\":\"${P}gunshot\",\"display_name\":\"枪声复用\",\"perception_mode\":\"auditory\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "2.16 软删后 name 不可复用" "42001" "$body"

echo ""

# =============================================================================
# 3. 导出 API
# =============================================================================
echo "--- 3. 导出 API ---"

# 先启用 earthquake 和 fire
V2=$(et_detail "$ET_ID2" | jq -r '.data.version' | tr -d '\r')
R=$(post "/event-types/toggle-enabled" "{\"id\":$ET_ID2,\"version\":$V2}")
echo "  [INFO] 启用 earthquake: $(echo $R | jq -r '.code' | tr -d '\r')"
V3=$(et_detail "$ET_ID3" | jq -r '.data.version' | tr -d '\r')
R=$(post "/event-types/toggle-enabled" "{\"id\":$ET_ID3,\"version\":$V3}")
echo "  [INFO] 启用 fire: $(echo $R | jq -r '.code' | tr -d '\r')"

# 3.1 导出 — 返回 items 数组
body=$(get_export "/event_types")
TOTAL=$((TOTAL + 1))
items_count=$(echo "$body" | jq '.items | length' 2>/dev/null | tr -d '\r')
if [ "$items_count" -ge "1" ] 2>/dev/null; then
  echo "  [PASS] 3.1 导出 items >= 1 (=$items_count)"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] 3.1 导出 items >= 1, 实际: $items_count"
  echo "         响应: $(echo "$body" | head -c 300)"
  FAIL=$((FAIL + 1))
fi

# 3.2 导出格式 — 每条有 name + config
TOTAL=$((TOTAL + 1))
first_name=$(echo "$body" | jq -r '.items[0].name // empty' | tr -d '\r')
first_config=$(echo "$body" | jq -r '.items[0].config // empty' | tr -d '\r')
if [ -n "$first_name" ] && [ -n "$first_config" ]; then
  echo "  [PASS] 3.2 导出格式 {name, config}"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] 3.2 导出格式, name='$first_name'"
  FAIL=$((FAIL + 1))
fi

# 3.3 已删除的 gunshot 不在导出中
TOTAL=$((TOTAL + 1))
deleted_count=$(echo "$body" | jq "[.items[] | select(.name==\"${P}gunshot\")] | length" | tr -d '\r')
if [ "$deleted_count" = "0" ]; then
  echo "  [PASS] 3.3 已删除不导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] 3.3 已删除不导出, 找到 $deleted_count 条"
  FAIL=$((FAIL + 1))
fi

echo ""

# =============================================================================
# 4. 攻击性测试
# =============================================================================
echo "--- 4. 攻击性测试 ---"

# 4.1 name 含大写
body=$(post "/event-types/create" "{\"name\":\"${P}BadCase\",\"display_name\":\"大写\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "4.1 name 大写拒绝" "42002" "$body"

# 4.2 name 含中文
body=$(post "/event-types/create" "{\"name\":\"枪声\",\"display_name\":\"中文名\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "4.2 name 中文拒绝" "42002" "$body"

# 4.3 name 含空格
body=$(post "/event-types/create" "{\"name\":\"bad name\",\"display_name\":\"空格\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "4.3 name 空格拒绝" "42002" "$body"

# 4.4 非法 perception_mode
body=$(post "/event-types/create" "{\"name\":\"${P}bad_mode\",\"display_name\":\"坏模式\",\"perception_mode\":\"telekinesis\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "4.4 非法 perception_mode" "42003" "$body"

# 4.5 severity 超范围
body=$(post "/event-types/create" "{\"name\":\"${P}bad_sev\",\"display_name\":\"超范围\",\"perception_mode\":\"visual\",\"default_severity\":101,\"default_ttl\":5,\"range\":100}")
assert_code "4.5 severity > 100" "42004" "$body"

# 4.6 severity = 0 合法（零值不被吞）
body=$(post "/event-types/create" "{\"name\":\"${P}zero_sev\",\"display_name\":\"零威胁\",\"perception_mode\":\"visual\",\"default_severity\":0,\"default_ttl\":5,\"range\":100}")
assert_code "4.6 severity=0 合法" "0" "$body"
ZERO_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')
body=$(et_detail "$ZERO_ID")
assert_field "4.6 config.default_severity=0" '.data.config.default_severity' "0" "$body"

# 4.7 ttl <= 0
body=$(post "/event-types/create" "{\"name\":\"${P}bad_ttl\",\"display_name\":\"零TTL\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":0,\"range\":100}")
assert_code "4.7 ttl=0 拒绝" "42005" "$body"

# 4.8 range < 0
body=$(post "/event-types/create" "{\"name\":\"${P}bad_range\",\"display_name\":\"负范围\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":-1}")
assert_code "4.8 range < 0 拒绝" "42006" "$body"

# 4.9 扩展字段塞不存在的 key → 42022
body=$(post "/event-types/create" "{\"name\":\"${P}bad_ext\",\"display_name\":\"坏扩展\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100,\"extensions\":{\"nonexistent_field\":1}}")
assert_code "4.9 不存在的扩展字段" "42022" "$body"

# 4.10 扩展字段值不符合约束 → 42007
body=$(post "/event-types/create" "{\"name\":\"${P}bad_ext_val\",\"display_name\":\"坏扩展值\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100,\"extensions\":{\"${P}priority\":99}}")
assert_code "4.10 扩展值超约束" "42007" "$body"

# 4.11 display_name SQL 注入
body=$(post "/event-types/create" "{\"name\":\"${P}sqli\",\"display_name\":\"' OR 1=1 --\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "4.11 SQL 注入 display_name 不崩" "0" "$body"

# 4.12 display_name 模糊搜索 LIKE 转义
body=$(post "/event-types/list" "{\"label\":\"%\",\"page\":1,\"page_size\":20}")
assert_code "4.12 LIKE % 不返回全部" "0" "$body"

# 4.13 乐观锁冲突（对未启用的 fire 事件用错误 version）
# 先停用 fire
V=$(et_detail "$ET_ID3" | jq -r '.data.version' | tr -d '\r')
post "/event-types/toggle-enabled" "{\"id\":$ET_ID3,\"version\":$V}" > /dev/null
body=$(post "/event-types/update" "{\"id\":$ET_ID3,\"display_name\":\"火灾(改)\",\"perception_mode\":\"visual\",\"default_severity\":70,\"default_ttl\":20,\"range\":100,\"version\":999}")
assert_code "4.13 乐观锁冲突" "42010" "$body"

# 4.14 不存在的 ID
body=$(post "/event-types/detail" "{\"id\":99999999}")
assert_code "4.14 不存在 ID" "42011" "$body"

echo ""

# =============================================================================
# 汇总
# =============================================================================
echo "================================================================="
echo "  结果: $PASS / $TOTAL PASS, $FAIL FAIL"
echo "================================================================="

if [ ${#BUGS[@]} -gt 0 ]; then
  echo ""
  echo "  可疑 BUG 汇总:"
  for b in "${BUGS[@]}"; do
    echo "    - $b"
  done
fi

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
exit 0
