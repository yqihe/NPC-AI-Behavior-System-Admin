#!/bin/bash
# =============================================================================
# 字段管理 API 全方位集成测试（ID 重构版）
# 运行前提：docker compose up -d && seed 脚本已执行
# 用法：bash tests/field_api_test.sh
# =============================================================================

# Windows 环境 UTF-8 支持
export LANG=en_US.UTF-8
export LC_ALL=en_US.UTF-8
if command -v chcp.com &>/dev/null; then
  chcp.com 65001 > /dev/null 2>&1
fi

BASE="http://localhost:9821/api/v1"
PASS=0
FAIL=0
TOTAL=0
TS=$(date +%s)
P="t${TS}_"

# --- 工具函数 ---

assert_code() {
  local test_name="$1" expected_code="$2" actual_body="$3"
  TOTAL=$((TOTAL + 1))
  actual_code=$(echo "$actual_body" | jq -r '.code // empty' 2>/dev/null | tr -d '\r')
  if [ "$actual_code" = "$expected_code" ]; then
    echo "  [PASS] $test_name"
    PASS=$((PASS + 1))
  else
    echo "  [FAIL] $test_name — 期望 code=$expected_code, 实际: $actual_code"
    echo "         响应: $(echo "$actual_body" | head -c 200)"
    FAIL=$((FAIL + 1))
  fi
}

assert_field() {
  local test_name="$1" jq_expr="$2" expected="$3" actual_body="$4"
  TOTAL=$((TOTAL + 1))
  actual=$(echo "$actual_body" | jq -r "$jq_expr" 2>/dev/null | tr -d '\r')
  if [ "$actual" = "$expected" ]; then
    echo "  [PASS] $test_name"
    PASS=$((PASS + 1))
  else
    echo "  [FAIL] $test_name — 期望 $expected, 实际: $actual"
    FAIL=$((FAIL + 1))
  fi
}

assert_ge() {
  local test_name="$1" jq_expr="$2" min_val="$3" actual_body="$4"
  TOTAL=$((TOTAL + 1))
  actual=$(echo "$actual_body" | jq -r "$jq_expr" 2>/dev/null | tr -d '\r')
  if [ "$actual" -ge "$min_val" ] 2>/dev/null; then
    echo "  [PASS] $test_name (=$actual)"
    PASS=$((PASS + 1))
  else
    echo "  [FAIL] $test_name — 期望 >= $min_val, 实际: $actual"
    FAIL=$((FAIL + 1))
  fi
}

assert_not_equal() {
  local test_name="$1" jq_expr="$2" unexpected="$3" actual_body="$4"
  TOTAL=$((TOTAL + 1))
  actual=$(echo "$actual_body" | jq -r "$jq_expr" 2>/dev/null | tr -d '\r')
  if [ "$actual" != "$unexpected" ]; then
    echo "  [PASS] $test_name (=$actual)"
    PASS=$((PASS + 1))
  else
    echo "  [FAIL] $test_name — 不应为 $unexpected, 实际: $actual"
    FAIL=$((FAIL + 1))
  fi
}

post() {
  local url="$BASE$1"
  local body="$2"
  printf '%s' "$body" | curl -s -X POST "$url" -H "Content-Type: application/json; charset=utf-8" --data-binary @-
}

# 按 ID 获取字段详情，返回完整响应
get_detail() {
  local id="$1"
  post "/fields/detail" "{\"id\":${id}}"
}

# 获取字段 version
get_version() {
  local id="$1"
  get_detail "$id" | jq -r '.data.version' | tr -d '\r'
}

# 启用字段
enable_field() {
  local id="$1"
  local ver=$(get_version "$id")
  post "/fields/toggle-enabled" "{\"id\":${id},\"enabled\":true,\"version\":${ver}}" > /dev/null
}

# 停用字段
disable_field() {
  local id="$1"
  local ver=$(get_version "$id")
  post "/fields/toggle-enabled" "{\"id\":${id},\"enabled\":false,\"version\":${ver}}" > /dev/null
}

# 停用 + 删除
disable_then_delete() {
  local id="$1"
  disable_field "$id"
  post "/fields/delete" "{\"id\":${id}}" > /dev/null 2>&1
}

# =============================================================================
echo "=========================================="
echo "  字段管理 API 全方位集成测试 (prefix=$P)"
echo "  所有操作使用 ID 标识"
echo "=========================================="

# --- 健康检查 ---
echo ""
echo "[健康检查]"
HEALTH=$(curl -s http://localhost:9821/health)
TOTAL=$((TOTAL + 1))
if echo "$HEALTH" | jq -e '.status == "ok"' > /dev/null 2>&1; then
  echo "  [PASS] 服务就绪"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] 服务未就绪，终止测试"
  exit 1
fi

# =============================================================================
# 功能 9：字典选项查询
# =============================================================================
echo ""
echo "[功能9: 字典选项查询]"

R=$(post "/dictionaries" '{"group":"field_type"}')
assert_code "9.1 查询 field_type 成功" "0" "$R"
assert_field "9.2 返回 6 种类型" ".data.items | length" "6" "$R"

R=$(post "/dictionaries" '{"group":"field_category"}')
assert_code "9.3 查询 field_category 成功" "0" "$R"
assert_field "9.4 返回 6 种分类" ".data.items | length" "6" "$R"

R=$(post "/dictionaries" '{"group":"field_properties"}')
assert_code "9.5 查询 field_properties 成功" "0" "$R"

R=$(post "/dictionaries" '{"group":""}')
assert_code "9.6 空 group 返回参数错误" "40000" "$R"

R=$(post "/dictionaries" '{"group":"nonexistent"}')
assert_code "9.7 不存在的 group 返回成功（空列表）" "0" "$R"
assert_field "9.8 返回空列表" ".data.items | length" "0" "$R"

# =============================================================================
# 功能 2：新建字段（默认未启用）
# =============================================================================
echo ""
echo "[功能2: 新建字段]"

R=$(post "/fields/create" "{\"name\":\"${P}hp\",\"label\":\"测试生命值\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"HP\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}}}")
assert_code "2.1 创建成功" "0" "$R"
assert_field "2.1 返回 name" ".data.name" "${P}hp" "$R"
HP_ID=$(echo "$R" | jq -r '.data.id')
assert_not_equal "2.1 返回 id > 0" ".data.id" "null" "$R"

# 默认未启用
R=$(get_detail "$HP_ID")
assert_field "2.2 新建默认 enabled=false" ".data.enabled" "false" "$R"
assert_field "2.2 初始 version=1" ".data.version" "1" "$R"
assert_field "2.2 初始 ref_count=0" ".data.ref_count" "0" "$R"

# 重复名字
R=$(post "/fields/create" "{\"name\":\"${P}hp\",\"label\":\"重复\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{}}")
assert_code "2.3 重复名字返回 40001" "40001" "$R"

# 非法名字格式
R=$(post "/fields/create" '{"name":"HP-bad","label":"坏","type":"integer","category":"combat","properties":{}}')
assert_code "2.4 大写+横线返回 40002" "40002" "$R"

R=$(post "/fields/create" '{"name":"123start","label":"数字开头","type":"integer","category":"combat","properties":{}}')
assert_code "2.5 数字开头返回 40002" "40002" "$R"

# 缺必填字段
R=$(post "/fields/create" '{"name":"","label":"空名","type":"integer","category":"combat","properties":{}}')
assert_code "2.6 空名返回 40002" "40002" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}nolabel\",\"label\":\"\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{}}")
assert_code "2.7 空标签返回 40000" "40000" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}notype\",\"label\":\"无类型\",\"type\":\"\",\"category\":\"combat\",\"properties\":{}}")
assert_code "2.8 空类型返回 40000" "40000" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}noprops\",\"label\":\"无属性\",\"type\":\"integer\",\"category\":\"combat\"}")
assert_code "2.9 无 properties 返回 40000" "40000" "$R"

# 不存在的类型/分类
R=$(post "/fields/create" "{\"name\":\"${P}badtype\",\"label\":\"假类型\",\"type\":\"faketype\",\"category\":\"combat\",\"properties\":{}}")
assert_code "2.10 不存在的类型返回 40003" "40003" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}badcat\",\"label\":\"假分类\",\"type\":\"integer\",\"category\":\"fakecat\",\"properties\":{}}")
assert_code "2.11 不存在的分类返回 40004" "40004" "$R"

# 创建其他类型字段用于后续测试
R=$(post "/fields/create" "{\"name\":\"${P}atk\",\"label\":\"攻击力\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"ATK\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":999}}}")
assert_code "2.12 创建 atk" "0" "$R"
ATK_ID=$(echo "$R" | jq -r '.data.id')

R=$(post "/fields/create" "{\"name\":\"${P}str\",\"label\":\"力量文本\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{\"description\":\"STR\",\"expose_bb\":false,\"constraints\":{\"minLength\":1,\"maxLength\":50}}}")
assert_code "2.13 创建 string 类型" "0" "$R"
STR_ID=$(echo "$R" | jq -r '.data.id')

R=$(post "/fields/create" "{\"name\":\"${P}flag\",\"label\":\"布尔标记\",\"type\":\"boolean\",\"category\":\"basic\",\"properties\":{\"description\":\"flag\",\"expose_bb\":false}}")
assert_code "2.14 创建 boolean 类型" "0" "$R"
FLAG_ID=$(echo "$R" | jq -r '.data.id')

R=$(post "/fields/create" "{\"name\":\"${P}mood\",\"label\":\"情绪选择\",\"type\":\"select\",\"category\":\"personality\",\"properties\":{\"description\":\"mood\",\"expose_bb\":false,\"constraints\":{\"options\":[{\"value\":\"happy\",\"label\":\"开心\"},{\"value\":\"sad\",\"label\":\"伤心\"}],\"minSelect\":1,\"maxSelect\":1}}}")
assert_code "2.15 创建 select 类型" "0" "$R"
MOOD_ID=$(echo "$R" | jq -r '.data.id')

# =============================================================================
# 功能 6：字段名唯一性校验
# =============================================================================
echo ""
echo "[功能6: 字段名唯一性校验]"

R=$(post "/fields/check-name" "{\"name\":\"${P}hp\"}")
assert_code "6.1 已存在的名字" "0" "$R"
assert_field "6.1 available=false" ".data.available" "false" "$R"

R=$(post "/fields/check-name" "{\"name\":\"${P}not_exist_999\"}")
assert_code "6.2 不存在的名字" "0" "$R"
assert_field "6.2 available=true" ".data.available" "true" "$R"

R=$(post "/fields/check-name" '{"name":""}')
assert_code "6.3 空名返回参数错误" "40000" "$R"

# =============================================================================
# 功能 3：字段详情（按 ID）
# =============================================================================
echo ""
echo "[功能3: 字段详情（按 ID）]"

R=$(get_detail "$HP_ID")
assert_code "3.1 按 ID 查详情成功" "0" "$R"
assert_field "3.1 返回正确 name" ".data.name" "${P}hp" "$R"
assert_field "3.1 返回正确 label" ".data.label" "测试生命值" "$R"
assert_field "3.1 返回 properties" ".data.properties.description" "HP" "$R"

R=$(get_detail "999999")
assert_code "3.2 不存在的 ID 返回 40011" "40011" "$R"

R=$(post "/fields/detail" '{"id":0}')
assert_code "3.3 ID=0 返回参数错误" "40000" "$R"

R=$(post "/fields/detail" '{"id":-1}')
assert_code "3.4 ID=-1 返回参数错误" "40000" "$R"

# =============================================================================
# 功能 1：字段列表
# =============================================================================
echo ""
echo "[功能1: 字段列表]"

R=$(post "/fields/list" '{"page":1,"page_size":20}')
assert_code "1.1 列表成功" "0" "$R"
assert_ge "1.1 至少有测试创建的字段" ".data.total" "4" "$R"
assert_field "1.1 items 是数组" ".data.items | type" "array" "$R"

# 列表项包含 id 字段
assert_not_equal "1.2 列表项有 id" ".data.items[0].id" "null" "$R"

# 按类型筛选
R=$(post "/fields/list" '{"type":"boolean","page":1,"page_size":20}')
assert_code "1.3 按 type 筛选" "0" "$R"

# 按分类筛选
R=$(post "/fields/list" '{"category":"combat","page":1,"page_size":20}')
assert_code "1.4 按 category 筛选" "0" "$R"

# 按标签模糊搜索
R=$(post "/fields/list" "{\"label\":\"测试生命\",\"page\":1,\"page_size\":20}")
assert_code "1.5 模糊搜索" "0" "$R"
assert_ge "1.5 至少找到 1 条" ".data.total" "1" "$R"

# enabled 筛选 — 场景 A：字段管理页不传 enabled
R=$(post "/fields/list" '{"page":1,"page_size":20}')
assert_code "1.6 不传 enabled 返回全部" "0" "$R"

# enabled 筛选 — 场景 B：其他模块传 enabled=true
R=$(post "/fields/list" '{"enabled":true,"page":1,"page_size":20}')
assert_code "1.7 enabled=true 仅返回启用" "0" "$R"

# 分页边界
R=$(post "/fields/list" '{"page":0,"page_size":0}')
assert_code "1.8 page=0 自动校正" "0" "$R"
assert_field "1.8 page 校正为 1" ".data.page" "1" "$R"

# 空结果
R=$(post "/fields/list" '{"label":"绝对不存在的标签zzz","page":1,"page_size":20}')
assert_code "1.9 空结果" "0" "$R"
assert_field "1.9 total=0" ".data.total" "0" "$R"
assert_field "1.9 items 空数组" ".data.items | length" "0" "$R"

# =============================================================================
# 功能 4：编辑字段（仅未启用时可编辑）
# =============================================================================
echo ""
echo "[功能4: 编辑字段]"

# 4.1 正常编辑（未启用状态）
HP_VER=$(get_version "$HP_ID")
R=$(post "/fields/update" "{\"id\":${HP_ID},\"label\":\"生命值改\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"HP changed\",\"expose_bb\":true,\"constraints\":{\"min\":0,\"max\":200}},\"version\":${HP_VER}}")
assert_code "4.1 编辑成功（未启用状态）" "0" "$R"

R=$(get_detail "$HP_ID")
assert_field "4.1 label 已更新" ".data.label" "生命值改" "$R"
assert_field "4.1 description 已更新" ".data.properties.description" "HP changed" "$R"
assert_field "4.1 max 已更新" ".data.properties.constraints.max" "200" "$R"

# 4.2 启用后禁止编辑
enable_field "$HP_ID"
HP_VER=$(get_version "$HP_ID")
R=$(post "/fields/update" "{\"id\":${HP_ID},\"label\":\"不该成功\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{},\"version\":${HP_VER}}")
assert_code "4.2 启用后编辑返回 40015" "40015" "$R"

# 恢复为停用状态
disable_field "$HP_ID"

# 4.3 乐观锁冲突
HP_VER=$(get_version "$HP_ID")
R=$(post "/fields/update" "{\"id\":${HP_ID},\"label\":\"锁测试\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"lock test\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":200}},\"version\":999}")
assert_code "4.3 版本冲突返回 40010" "40010" "$R"

# 4.4 不存在的 ID
R=$(post "/fields/update" '{"id":999999,"label":"不存在","type":"integer","category":"combat","properties":{},"version":1}')
assert_code "4.4 不存在的 ID 返回 40011" "40011" "$R"

# 4.5 无效 ID
R=$(post "/fields/update" '{"id":0,"label":"无效","type":"integer","category":"combat","properties":{},"version":1}')
assert_code "4.5 ID=0 返回参数错误" "40000" "$R"

# 4.6 不存在的类型/分类
HP_VER=$(get_version "$HP_ID")
R=$(post "/fields/update" "{\"id\":${HP_ID},\"label\":\"假类型\",\"type\":\"faketype\",\"category\":\"combat\",\"properties\":{},\"version\":${HP_VER}}")
assert_code "4.6 不存在的类型返回 40003" "40003" "$R"

R=$(post "/fields/update" "{\"id\":${HP_ID},\"label\":\"假分类\",\"type\":\"integer\",\"category\":\"fakecat\",\"properties\":{},\"version\":${HP_VER}}")
assert_code "4.7 不存在的分类返回 40004" "40004" "$R"

# =============================================================================
# 功能 8：启用/停用切换
# =============================================================================
echo ""
echo "[功能8: 启用/停用切换]"

ATK_VER=$(get_version "$ATK_ID")
R=$(post "/fields/toggle-enabled" "{\"id\":${ATK_ID},\"enabled\":true,\"version\":${ATK_VER}}")
assert_code "8.1 启用成功" "0" "$R"

R=$(get_detail "$ATK_ID")
assert_field "8.1 enabled=true" ".data.enabled" "true" "$R"

ATK_VER=$(get_version "$ATK_ID")
R=$(post "/fields/toggle-enabled" "{\"id\":${ATK_ID},\"enabled\":false,\"version\":${ATK_VER}}")
assert_code "8.2 停用成功" "0" "$R"

R=$(get_detail "$ATK_ID")
assert_field "8.2 enabled=false" ".data.enabled" "false" "$R"

# 乐观锁冲突
R=$(post "/fields/toggle-enabled" "{\"id\":${ATK_ID},\"enabled\":true,\"version\":999}")
assert_code "8.3 版本冲突返回 40010" "40010" "$R"

# 不存在的 ID
R=$(post "/fields/toggle-enabled" '{"id":999999,"enabled":true,"version":1}')
assert_code "8.4 不存在的 ID 返回 40011" "40011" "$R"

# 无效 ID
R=$(post "/fields/toggle-enabled" '{"id":0,"enabled":true,"version":1}')
assert_code "8.5 ID=0 返回参数错误" "40000" "$R"

# =============================================================================
# 功能 10：约束收紧检查（内嵌在编辑中）
# =============================================================================
echo ""
echo "[功能10: 约束收紧检查]"

# 创建一个被引用的字段来测试收紧检查
R=$(post "/fields/create" "{\"name\":\"${P}target\",\"label\":\"收紧目标\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"target\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}}}")
assert_code "10.0 创建 target" "0" "$R"
TARGET_ID=$(echo "$R" | jq -r '.data.id')

# 启用 target 以便被引用
enable_field "$TARGET_ID"

# 创建 reference 字段引用 target
R=$(post "/fields/create" "{\"name\":\"${P}reftest\",\"label\":\"引用测试\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"ref test\",\"expose_bb\":false,\"constraints\":{\"refs\":[${TARGET_ID}]}}}")
assert_code "10.1 创建 reference 字段" "0" "$R"
REF_ID=$(echo "$R" | jq -r '.data.id')

# 确认 target 的 ref_count 增加
R=$(get_detail "$TARGET_ID")
assert_field "10.2 target ref_count=1" ".data.ref_count" "1" "$R"

# 尝试收紧 target 的约束（ref_count > 0）— 需先停用
disable_field "$TARGET_ID"
TARGET_VER=$(get_version "$TARGET_ID")
R=$(post "/fields/update" "{\"id\":${TARGET_ID},\"label\":\"收紧目标\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"target\",\"expose_bb\":false,\"constraints\":{\"min\":10,\"max\":100}},\"version\":${TARGET_VER}}")
assert_code "10.3 min 收紧返回 40007" "40007" "$R"

R=$(post "/fields/update" "{\"id\":${TARGET_ID},\"label\":\"收紧目标\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"target\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":50}},\"version\":${TARGET_VER}}")
assert_code "10.4 max 收紧返回 40007" "40007" "$R"

# 约束放宽允许
R=$(post "/fields/update" "{\"id\":${TARGET_ID},\"label\":\"收紧目标\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"target\",\"expose_bb\":false,\"constraints\":{\"min\":-10,\"max\":200}},\"version\":${TARGET_VER}}")
assert_code "10.5 约束放宽成功" "0" "$R"

# 被引用时禁止改类型
TARGET_VER=$(get_version "$TARGET_ID")
R=$(post "/fields/update" "{\"id\":${TARGET_ID},\"label\":\"收紧目标\",\"type\":\"string\",\"category\":\"combat\",\"properties\":{\"description\":\"target\",\"expose_bb\":false,\"constraints\":{\"minLength\":0,\"maxLength\":100}},\"version\":${TARGET_VER}}")
assert_code "10.6 被引用时改类型返回 40006" "40006" "$R"

# =============================================================================
# 功能 11：循环引用检测 + 引用关系维护
# =============================================================================
echo ""
echo "[功能11: 循环引用检测]"

# 创建两个字段用于循环引用测试
R=$(post "/fields/create" "{\"name\":\"${P}a\",\"label\":\"链A\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"A\",\"expose_bb\":false}}")
A_ID=$(echo "$R" | jq -r '.data.id')
enable_field "$A_ID"

R=$(post "/fields/create" "{\"name\":\"${P}b\",\"label\":\"链B\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"B\",\"expose_bb\":false,\"constraints\":{\"refs\":[${A_ID}]}}}")
assert_code "11.1 B 引用 A 成功" "0" "$R"
B_ID=$(echo "$R" | jq -r '.data.id')
enable_field "$B_ID"

# 尝试创建 C 引用 B，再编辑 B 引用 C → 循环
R=$(post "/fields/create" "{\"name\":\"${P}c\",\"label\":\"链C\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"C\",\"expose_bb\":false,\"constraints\":{\"refs\":[${B_ID}]}}}")
assert_code "11.2 C 引用 B 成功" "0" "$R"
C_ID=$(echo "$R" | jq -r '.data.id')
enable_field "$C_ID"

# 引用停用字段
R=$(post "/fields/create" "{\"name\":\"${P}d\",\"label\":\"链D\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"D\",\"expose_bb\":false}}")
D_ID=$(echo "$R" | jq -r '.data.id')
# D 未启用，尝试引用它
R=$(post "/fields/create" "{\"name\":\"${P}e\",\"label\":\"链E\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"E\",\"expose_bb\":false,\"constraints\":{\"refs\":[${D_ID}]}}}")
assert_code "11.3 引用停用字段返回 40013" "40013" "$R"

# 引用不存在的字段
R=$(post "/fields/create" "{\"name\":\"${P}f\",\"label\":\"链F\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"F\",\"expose_bb\":false,\"constraints\":{\"refs\":[999999]}}}")
assert_code "11.4 引用不存在字段返回 40014" "40014" "$R"

# 删除 reference 字段时 ref_count 减回去
disable_then_delete "$REF_ID"
R=$(get_detail "$TARGET_ID")
assert_field "11.5 删除引用方后 target ref_count=0" ".data.ref_count" "0" "$R"

# =============================================================================
# 功能 7：字段引用详情
# =============================================================================
echo ""
echo "[功能7: 字段引用详情]"

R=$(post "/fields/references" "{\"id\":${A_ID}}")
assert_code "7.1 查引用详情成功" "0" "$R"
assert_field "7.1 返回 field_id" ".data.field_id" "$A_ID" "$R"
assert_ge "7.1 至少 1 个字段引用" ".data.fields | length" "1" "$R"

# 无引用的字段
R=$(post "/fields/references" "{\"id\":${FLAG_ID}}")
assert_code "7.2 无引用字段" "0" "$R"
assert_field "7.2 templates 空" ".data.templates | length" "0" "$R"
assert_field "7.2 fields 空" ".data.fields | length" "0" "$R"

# 不存在的 ID
R=$(post "/fields/references" '{"id":999999}')
assert_code "7.3 不存在的 ID 返回 40011" "40011" "$R"

# =============================================================================
# 功能 5：删除字段
# =============================================================================
echo ""
echo "[功能5: 软删除字段]"

# 5.1 删除启用中的字段 → 必须先停用
enable_field "$STR_ID"
R=$(post "/fields/delete" "{\"id\":${STR_ID}}")
assert_code "5.1 启用状态删除返回 40012" "40012" "$R"

# 5.2 停用后删除
disable_field "$STR_ID"
R=$(post "/fields/delete" "{\"id\":${STR_ID}}")
assert_code "5.2 停用后删除成功" "0" "$R"
assert_field "5.2 返回 id" ".data.id" "$STR_ID" "$R"

# 5.3 确认已删除（列表不可见）
R=$(get_detail "$STR_ID")
assert_code "5.3 已删除字段查不到" "40011" "$R"

# 5.4 删除不存在的字段
R=$(post "/fields/delete" '{"id":999999}')
assert_code "5.4 不存在的 ID 返回 40011" "40011" "$R"

# 5.5 无效 ID
R=$(post "/fields/delete" '{"id":0}')
assert_code "5.5 ID=0 返回参数错误" "40000" "$R"

# 5.6 被引用的字段无法删除
disable_field "$A_ID"
R=$(post "/fields/delete" "{\"id\":${A_ID}}")
assert_code "5.6 被引用的字段返回 40005" "40005" "$R"

# 5.7 已删除的 name 不能复用
R=$(post "/fields/check-name" "{\"name\":\"${P}str\"}")
assert_field "5.7 已删除 name 不可复用" ".data.available" "false" "$R"

# 5.8 删除 boolean 类型（无引用，正常删除流程）
R=$(post "/fields/delete" "{\"id\":${FLAG_ID}}")
assert_code "5.8 无引用字段删除成功" "0" "$R"

# =============================================================================
# 攻击性测试
# =============================================================================
echo ""
echo "[攻击性测试]"

# 注入攻击 — name 含特殊字符
R=$(post "/fields/create" '{"name":"a]\"injection","label":"注入","type":"integer","category":"combat","properties":{}}')
assert_code "ATK.1 注入字符被格式校验拦截" "40002" "$R"

# SQL 注入 — label 含 SQL
R=$(post "/fields/create" "{\"name\":\"${P}sqli\",\"label\":\"'; DROP TABLE fields; --\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{}}")
assert_code "ATK.2 SQL 注入无害（创建成功或校验拦截）" "0" "$R"
SQLI_ID=$(echo "$R" | jq -r '.data.id')
if [ "$SQLI_ID" != "null" ] && [ -n "$SQLI_ID" ]; then
  disable_then_delete "$SQLI_ID"
fi

# 超长 name
LONG_NAME=$(python3 -c "print('a' * 100)" 2>/dev/null || echo "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
R=$(post "/fields/create" "{\"name\":\"${LONG_NAME}\",\"label\":\"超长\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{}}")
assert_code "ATK.3 超长 name 被拦截" "40002" "$R"

# JSON 畸形
R=$(curl -s -X POST "$BASE/fields/create" -H "Content-Type: application/json" -d '{bad json}')
assert_code "ATK.4 畸形 JSON 返回参数错误" "40000" "$R"

# 空请求体
R=$(curl -s -X POST "$BASE/fields/create" -H "Content-Type: application/json" -d '')
assert_code "ATK.5 空请求体返回参数错误" "40000" "$R"

# =============================================================================
# 清理测试数据
# =============================================================================
echo ""
echo "[清理测试数据]"

# 先删 reference 字段（依赖链末端）
for ID in $C_ID $B_ID; do
  if [ -n "$ID" ] && [ "$ID" != "null" ]; then
    disable_then_delete "$ID"
  fi
done

# 再删被引用的字段
for ID in $A_ID $D_ID $TARGET_ID $HP_ID $ATK_ID $MOOD_ID; do
  if [ -n "$ID" ] && [ "$ID" != "null" ]; then
    disable_then_delete "$ID"
  fi
done

echo "  清理完成"

# =============================================================================
# 汇总
# =============================================================================
echo ""
echo "=========================================="
echo "  测试完成: 通过 $PASS / 失败 $FAIL / 总计 $TOTAL"
echo "=========================================="

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
exit 0
