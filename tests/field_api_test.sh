#!/bin/bash
# =============================================================================
# 字段管理 API 全方位集成测试
# 运行前提：docker compose up -d && seed 脚本已执行
# 用法：bash tests/field_api_test.sh
# =============================================================================

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
  actual_code=$(echo "$actual_body" | jq -r '.code // empty' 2>/dev/null)
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
  actual=$(echo "$actual_body" | jq -r "$jq_expr" 2>/dev/null)
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
  actual=$(echo "$actual_body" | jq -r "$jq_expr" 2>/dev/null)
  if [ "$actual" -ge "$min_val" ] 2>/dev/null; then
    echo "  [PASS] $test_name (=$actual)"
    PASS=$((PASS + 1))
  else
    echo "  [FAIL] $test_name — 期望 >= $min_val, 实际: $actual"
    FAIL=$((FAIL + 1))
  fi
}

assert_le() {
  local test_name="$1" jq_expr="$2" max_val="$3" actual_body="$4"
  TOTAL=$((TOTAL + 1))
  actual=$(echo "$actual_body" | jq -r "$jq_expr" 2>/dev/null)
  if [ "$actual" -le "$max_val" ] 2>/dev/null; then
    echo "  [PASS] $test_name (=$actual)"
    PASS=$((PASS + 1))
  else
    echo "  [FAIL] $test_name — 期望 <= $max_val, 实际: $actual"
    FAIL=$((FAIL + 1))
  fi
}

post() {
  curl -s -X POST "$BASE$1" -H "Content-Type: application/json" -d "$2"
}

get_version() {
  local name="$1"
  post "/fields/detail" "{\"name\":\"${name}\"}" | jq -r '.data.version'
}

enable_field() {
  local name="$1"
  local ver=$(get_version "$name")
  post "/fields/toggle-enabled" "{\"name\":\"${name}\",\"enabled\":true,\"version\":${ver}}" > /dev/null
}

disable_field() {
  local name="$1"
  local ver=$(get_version "$name")
  post "/fields/toggle-enabled" "{\"name\":\"${name}\",\"enabled\":false,\"version\":${ver}}" > /dev/null
}

disable_then_delete() {
  local name="$1"
  disable_field "$name"
  post "/fields/delete" "{\"name\":\"${name}\"}" > /dev/null 2>&1
}

# =============================================================================
echo "=========================================="
echo "  字段管理 API 全方位集成测试 (prefix=$P)"
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
# 功能 11：字典选项查询
# 场景：字段管理页新建/编辑字段时，下拉选项从后端动态获取
# =============================================================================
echo ""
echo "[功能11: 字典选项查询]"

R=$(post "/dictionaries" '{"group":"field_type"}')
assert_code "11.1 查询 field_type 成功" "0" "$R"
assert_field "11.2 返回 6 种类型" ".data.items | length" "6" "$R"

R=$(post "/dictionaries" '{"group":"field_category"}')
assert_code "11.3 查询 field_category 成功" "0" "$R"
assert_field "11.4 返回 6 种分类" ".data.items | length" "6" "$R"

R=$(post "/dictionaries" '{"group":"field_properties"}')
assert_code "11.5 查询 field_properties 成功" "0" "$R"

R=$(post "/dictionaries" '{"group":""}')
assert_code "11.6 空 group 返回参数错误" "40000" "$R"

R=$(post "/dictionaries" '{"group":"nonexistent"}')
assert_code "11.7 不存在的 group 返回成功（空列表）" "0" "$R"
assert_field "11.7 返回空列表" ".data.items | length" "0" "$R"

# =============================================================================
# 功能 2：新建字段
# 场景：管理员定义新的 NPC 属性，默认未启用
# =============================================================================
echo ""
echo "[功能2: 新建字段（默认未启用）]"

R=$(post "/fields/create" "{\"name\":\"${P}hp\",\"label\":\"测试生命值\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"HP\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}}}")
assert_code "2.1 创建成功" "0" "$R"
assert_field "2.1 返回 name" ".data.name" "${P}hp" "$R"

# 默认未启用 — 新建字段处于配置窗口期
R=$(post "/fields/detail" "{\"name\":\"${P}hp\"}")
assert_field "2.2 新建默认 enabled=false" ".data.enabled" "false" "$R"
assert_field "2.2 初始 version=1" ".data.version" "1" "$R"
assert_field "2.2 初始 ref_count=0" ".data.ref_count" "0" "$R"

# 重复名字
R=$(post "/fields/create" "{\"name\":\"${P}hp\",\"label\":\"重复\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{}}")
assert_code "2.3 重复名字返回 40001" "40001" "$R"

# 格式校验
R=$(post "/fields/create" '{"name":"TestBad","label":"x","type":"integer","category":"combat","properties":{}}')
assert_code "2.4 大写名字返回 40002" "40002" "$R"

R=$(post "/fields/create" '{"name":"123start","label":"x","type":"integer","category":"combat","properties":{}}')
assert_code "2.5 数字开头返回 40002" "40002" "$R"

# 字典校验
R=$(post "/fields/create" "{\"name\":\"${P}bad1\",\"label\":\"x\",\"type\":\"nonexistent\",\"category\":\"combat\",\"properties\":{}}")
assert_code "2.6 不存在类型返回 40003" "40003" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}bad2\",\"label\":\"x\",\"type\":\"integer\",\"category\":\"nonexistent\",\"properties\":{}}")
assert_code "2.7 不存在分类返回 40004" "40004" "$R"

# 必填校验
R=$(post "/fields/create" '{"name":"","label":"x","type":"integer","category":"combat","properties":{}}')
assert_code "2.8 空 name 返回参数错误" "40002" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}bad3\",\"label\":\"\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{}}")
assert_code "2.9 空 label 返回参数错误" "40000" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}bad4\",\"label\":\"x\",\"type\":\"integer\",\"category\":\"combat\"}")
assert_code "2.10 无 properties 返回参数错误" "40000" "$R"

# 创建其他测试字段
post "/fields/create" "{\"name\":\"${P}speed\",\"label\":\"测试速度\",\"type\":\"float\",\"category\":\"movement\",\"properties\":{\"expose_bb\":true,\"constraints\":{\"min\":0,\"max\":50}}}" > /dev/null
post "/fields/create" "{\"name\":\"${P}name_str\",\"label\":\"测试名称\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{\"constraints\":{\"minLength\":1,\"maxLength\":20}}}" > /dev/null
post "/fields/create" "{\"name\":\"${P}alive\",\"label\":\"是否存活\",\"type\":\"boolean\",\"category\":\"basic\",\"properties\":{}}" > /dev/null
post "/fields/create" "{\"name\":\"${P}mood\",\"label\":\"情绪\",\"type\":\"select\",\"category\":\"personality\",\"properties\":{\"constraints\":{\"options\":[{\"value\":\"happy\"},{\"value\":\"sad\"},{\"value\":\"angry\"}]}}}" > /dev/null
post "/fields/create" "{\"name\":\"${P}batch_a\",\"label\":\"批量A\",\"type\":\"boolean\",\"category\":\"basic\",\"properties\":{}}" > /dev/null
post "/fields/create" "{\"name\":\"${P}batch_b\",\"label\":\"批量B\",\"type\":\"boolean\",\"category\":\"basic\",\"properties\":{}}" > /dev/null
post "/fields/create" "{\"name\":\"${P}batch_c\",\"label\":\"批量C\",\"type\":\"boolean\",\"category\":\"basic\",\"properties\":{}}" > /dev/null
echo "  [INFO] 额外创建 speed, name_str, alive, mood, batch_a, batch_b, batch_c（均未启用）"

# =============================================================================
# 功能 10：启用/停用切换
# 场景 A：管理员确认配置无误后启用，其他模块才能看到
# 场景 B：管理员下线字段先停用，存量不动增量拦截
# =============================================================================
echo ""
echo "[功能10: 启用/停用切换]"

R=$(post "/fields/detail" "{\"name\":\"${P}hp\"}")
VER=$(echo "$R" | jq -r '.data.version')

R=$(post "/fields/toggle-enabled" "{\"name\":\"${P}hp\",\"enabled\":true,\"version\":${VER}}")
assert_code "10.1 启用成功" "0" "$R"

R=$(post "/fields/detail" "{\"name\":\"${P}hp\"}")
assert_field "10.2 enabled=true" ".data.enabled" "true" "$R"
VER=$(echo "$R" | jq -r '.data.version')

R=$(post "/fields/toggle-enabled" "{\"name\":\"${P}hp\",\"enabled\":false,\"version\":${VER}}")
assert_code "10.3 停用成功" "0" "$R"

R=$(post "/fields/detail" "{\"name\":\"${P}hp\"}")
assert_field "10.4 enabled=false" ".data.enabled" "false" "$R"

# 乐观锁
R=$(post "/fields/toggle-enabled" "{\"name\":\"${P}hp\",\"enabled\":true,\"version\":1}")
assert_code "10.5 版本冲突返回 40010" "40010" "$R"

# 不合法版本号
R=$(post "/fields/toggle-enabled" "{\"name\":\"${P}hp\",\"enabled\":true,\"version\":0}")
assert_code "10.6 version=0 返回参数错误" "40000" "$R"

# 不存在字段
R=$(post "/fields/toggle-enabled" '{"name":"nonexistent_xyz","enabled":true,"version":1}')
assert_code "10.7 不存在字段返回 40011" "40011" "$R"

# 把 hp 和部分字段启用，后续测试要用
enable_field "${P}hp"
enable_field "${P}speed"
enable_field "${P}alive"
enable_field "${P}mood"

# =============================================================================
# 功能 6：字段名唯一性校验
# 场景：新建字段时输入框失焦实时校验，含软删除也不可复用
# =============================================================================
echo ""
echo "[功能6: 字段名唯一性校验]"

R=$(post "/fields/check-name" "{\"name\":\"${P}unique_xyz\"}")
assert_code "6.1 可用名称" "0" "$R"
assert_field "6.1 available=true" ".data.available" "true" "$R"

R=$(post "/fields/check-name" "{\"name\":\"${P}hp\"}")
assert_field "6.2 已存在 available=false" ".data.available" "false" "$R"

R=$(post "/fields/check-name" '{"name":""}')
assert_code "6.3 空名称返回参数错误" "40000" "$R"

# =============================================================================
# 功能 1：字段列表
# 场景 A：字段管理页浏览全部字段（不传 enabled）
# 场景 B：其他模块选字段（传 enabled=true 只看启用的）
# =============================================================================
echo ""
echo "[功能1: 字段列表]"

# 场景 A：字段管理页看全部
R=$(post "/fields/list" '{}')
assert_code "1.1 默认分页成功" "0" "$R"
assert_ge "1.2 total >= 8（含启用+未启用）" ".data.total" "8" "$R"

# 验证列表包含 enabled 字段
assert_field "1.3 列表含 enabled 字段" '.data.items[0] | has("enabled")' "true" "$R"
# 验证列表包含 type_label 翻译
assert_field "1.4 列表含 type_label" '.data.items[0] | has("type_label")' "true" "$R"

# 场景 B：其他模块选字段 — 只看启用的
R=$(post "/fields/list" '{"enabled":true}')
assert_code "1.5 enabled=true 筛选成功" "0" "$R"
ENABLED_TOTAL=$(echo "$R" | jq -r '.data.total')

R=$(post "/fields/list" '{"enabled":false}')
assert_code "1.6 enabled=false 筛选成功" "0" "$R"
DISABLED_TOTAL=$(echo "$R" | jq -r '.data.total')

R=$(post "/fields/list" '{}')
ALL_TOTAL=$(echo "$R" | jq -r '.data.total')

TOTAL=$((TOTAL + 1))
SUM=$((ENABLED_TOTAL + DISABLED_TOTAL))
if [ "$SUM" = "$ALL_TOTAL" ]; then
  echo "  [PASS] 1.7 启用($ENABLED_TOTAL) + 未启用($DISABLED_TOTAL) = 全部($ALL_TOTAL)"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] 1.7 启用($ENABLED_TOTAL) + 未启用($DISABLED_TOTAL) != 全部($ALL_TOTAL)"
  FAIL=$((FAIL + 1))
fi

# 组合筛选
R=$(post "/fields/list" '{"type":"integer"}')
assert_code "1.8 按 type 筛选" "0" "$R"

R=$(post "/fields/list" '{"category":"combat"}')
assert_code "1.9 按 category 筛选" "0" "$R"

R=$(post "/fields/list" "{\"label\":\"测试\"}")
assert_code "1.10 按 label 模糊搜索" "0" "$R"

R=$(post "/fields/list" '{"type":"integer","enabled":true}')
assert_code "1.11 type + enabled 组合筛选" "0" "$R"

# 分页
R=$(post "/fields/list" '{"page":1,"page_size":2}')
assert_le "1.12 分页 page_size=2 返回条数 <= 2" ".data.items | length" "2" "$R"
assert_field "1.12 page_size=2 响应" ".data.page_size" "2" "$R"

# =============================================================================
# 功能 3：字段详情
# 场景 A：管理员查看/编辑字段
# 场景 B：模板管理页获取字段配置（停用字段也能查）
# =============================================================================
echo ""
echo "[功能3: 字段详情]"

R=$(post "/fields/detail" "{\"name\":\"${P}hp\"}")
assert_code "3.1 查询成功" "0" "$R"
assert_field "3.1 name 正确" ".data.name" "${P}hp" "$R"
assert_field "3.1 type 正确" ".data.type" "integer" "$R"
assert_field "3.1 properties 含 description" ".data.properties.description" "HP" "$R"

# 未启用的字段也能查详情（模板里已有的停用字段仍需看配置）
R=$(post "/fields/detail" "{\"name\":\"${P}name_str\"}")
assert_code "3.2 未启用字段也能查详情" "0" "$R"
assert_field "3.2 enabled=false" ".data.enabled" "false" "$R"

# 不存在字段 — 返回 40011 ErrFieldNotFound
R=$(post "/fields/detail" '{"name":"nonexistent_xyz"}')
assert_code "3.3 不存在返回 40011" "40011" "$R"

R=$(post "/fields/detail" '{"name":""}')
assert_code "3.4 空 name 返回参数错误" "40000" "$R"

# =============================================================================
# 功能 4：编辑字段
# 场景 A：管理员修改字段配置（启用/停用都能编辑）
# 场景 B：reference 类型改成其他类型，自动清理引用
# =============================================================================
echo ""
echo "[功能4: 编辑字段]"

VER=$(get_version "${P}hp")
R=$(post "/fields/update" "{\"name\":\"${P}hp\",\"label\":\"生命值-改\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"expose_bb\":true,\"constraints\":{\"min\":0,\"max\":100}},\"version\":${VER}}")
assert_code "4.1 启用字段编辑成功" "0" "$R"

R=$(post "/fields/detail" "{\"name\":\"${P}hp\"}")
# 中文在 Git Bash 的 jq 输出可能乱码，只验证 label 非原值
LABEL_CHANGED=$(echo "$R" | jq -r '.data.label != "测试生命值"' 2>/dev/null)
TOTAL=$((TOTAL + 1))
if [ "$LABEL_CHANGED" = "true" ]; then
  echo "  [PASS] 4.2 label 已更新"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] 4.2 label 未更新"
  FAIL=$((FAIL + 1))
fi

# 未启用字段也能编辑
VER=$(get_version "${P}name_str")
R=$(post "/fields/update" "{\"name\":\"${P}name_str\",\"label\":\"名称-改\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{\"constraints\":{\"minLength\":1,\"maxLength\":30}},\"version\":${VER}}")
assert_code "4.3 未启用字段也能编辑" "0" "$R"

# 版本冲突
R=$(post "/fields/update" "{\"name\":\"${P}hp\",\"label\":\"冲突\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{},\"version\":1}")
assert_code "4.4 版本冲突返回 40010" "40010" "$R"

# 不存在字段 — 40011
R=$(post "/fields/update" '{"name":"nonexistent_xyz","label":"x","type":"integer","category":"combat","properties":{},"version":1}')
assert_code "4.5 不存在返回 40011" "40011" "$R"

# =============================================================================
# 功能 9：批量修改分类
# 场景：管理员把一批字段从一个分类移到另一个分类
# =============================================================================
echo ""
echo "[功能9: 批量修改分类]"

R=$(post "/fields/batch-category" "{\"names\":[\"${P}batch_a\",\"${P}batch_b\"],\"category\":\"combat\"}")
assert_code "9.1 批量改分类成功" "0" "$R"
assert_field "9.1 affected=2" ".data.affected" "2" "$R"

# 验证确实改了
R=$(post "/fields/detail" "{\"name\":\"${P}batch_a\"}")
assert_field "9.2 batch_a category 已变" ".data.category" "combat" "$R"

# 不存在分类
R=$(post "/fields/batch-category" "{\"names\":[\"${P}batch_c\"],\"category\":\"nonexistent\"}")
assert_code "9.3 不存在分类返回 40004" "40004" "$R"

# 空列表
R=$(post "/fields/batch-category" '{"names":[],"category":"combat"}')
assert_code "9.4 空 names 返回参数错误" "40000" "$R"

# =============================================================================
# 功能 5：删除字段
# 场景：管理员彻底移除不需要的字段，必须先停用、无引用
# =============================================================================
echo ""
echo "[功能5: 删除字段]"

# 启用状态直接删除 → 被拒绝（必须先停用）
R=$(post "/fields/delete" "{\"name\":\"${P}alive\"}")
assert_code "5.1 启用状态删除返回 40012" "40012" "$R"

# 停用后删除成功
disable_field "${P}alive"
R=$(post "/fields/delete" "{\"name\":\"${P}alive\"}")
assert_code "5.2 停用后删除成功" "0" "$R"
assert_field "5.2 deleted=true" ".data.deleted" "true" "$R"

# 已删除查不到
R=$(post "/fields/detail" "{\"name\":\"${P}alive\"}")
assert_code "5.3 已删除查不到，返回 40011" "40011" "$R"

# 软删除后名字仍然被占用
R=$(post "/fields/check-name" "{\"name\":\"${P}alive\"}")
assert_field "5.4 软删除后名字仍占用" ".data.available" "false" "$R"

# 不存在字段
R=$(post "/fields/delete" '{"name":"nonexistent_xyz"}')
assert_code "5.5 不存在返回 40011" "40011" "$R"

# =============================================================================
# 功能 7：字段引用详情
# 场景：管理员停用/删除前先看谁在用这个字段
# =============================================================================
echo ""
echo "[功能7: 字段引用详情]"

R=$(post "/fields/references" "{\"name\":\"${P}hp\"}")
assert_code "7.1 查询成功" "0" "$R"
assert_field "7.1 field_name" ".data.field_name" "${P}hp" "$R"
assert_field "7.1 无引用 templates 空" ".data.templates | length" "0" "$R"
assert_field "7.1 无引用 fields 空" ".data.fields | length" "0" "$R"

R=$(post "/fields/references" '{"name":"nonexistent_xyz"}')
assert_code "7.2 不存在返回 40011" "40011" "$R"

# =============================================================================
# 功能 8：批量删除
# 场景：管理员勾选多个字段一起删除，能删的删、不能删的跳过
# =============================================================================
echo ""
echo "[功能8: 批量删除]"

# batch_a 是启用的，batch_b 和 batch_c 是未启用的
enable_field "${P}batch_a"

R=$(post "/fields/batch-delete" "{\"names\":[\"${P}batch_a\",\"${P}batch_b\",\"${P}batch_c\"]}")
assert_code "8.1 批量删除成功" "0" "$R"
assert_field "8.1 删除 2 个（未启用的）" ".data.deleted | length" "2" "$R"
assert_field "8.1 跳过 1 个（已启用的）" ".data.skipped | length" "1" "$R"

# 跳过原因
assert_field "8.2 跳过原因含停用提示" ".data.skipped[0].reason" "请先停用再删除" "$R"

# 不存在的字段也能处理
R=$(post "/fields/batch-delete" '{"names":["nonexistent_a","nonexistent_b"]}')
assert_code "8.3 不存在的字段被跳过" "0" "$R"
assert_field "8.3 全部跳过" ".data.deleted | length" "0" "$R"

# 空列表
R=$(post "/fields/batch-delete" '{"names":[]}')
assert_code "8.4 空 names 返回参数错误" "40000" "$R"

# 清理 batch_a
disable_then_delete "${P}batch_a"

# =============================================================================
# 功能 12：约束收紧检查
# 场景：被引用的字段编辑约束时，只能放宽不能收紧
# =============================================================================
echo ""
echo "[功能12: 约束收紧检查]"

# 创建字段并让它被引用
post "/fields/create" "{\"name\":\"${P}cstr\",\"label\":\"约束测试\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"constraints\":{\"min\":0,\"max\":100}}}" > /dev/null
enable_field "${P}cstr"
post "/fields/create" "{\"name\":\"${P}ref_c\",\"label\":\"约束引用者\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"constraints\":{\"refs\":[\"${P}cstr\"]}}}" > /dev/null
echo "  [INFO] 创建 ${P}cstr(启用) 被 ${P}ref_c(reference) 引用"

# 收紧 min → 拒绝
VER=$(get_version "${P}cstr")
R=$(post "/fields/update" "{\"name\":\"${P}cstr\",\"label\":\"约束测试\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"constraints\":{\"min\":10,\"max\":100}},\"version\":${VER}}")
assert_code "12.1 收紧 min(0→10) 返回 40007" "40007" "$R"

# 收紧 max → 拒绝
R=$(post "/fields/update" "{\"name\":\"${P}cstr\",\"label\":\"约束测试\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"constraints\":{\"min\":0,\"max\":50}},\"version\":${VER}}")
assert_code "12.2 收紧 max(100→50) 返回 40007" "40007" "$R"

# 放宽 → 允许
R=$(post "/fields/update" "{\"name\":\"${P}cstr\",\"label\":\"约束放宽\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"constraints\":{\"min\":-10,\"max\":200}},\"version\":${VER}}")
assert_code "12.3 放宽(min↓max↑) 允许" "0" "$R"

# 被引用时禁止改类型
VER=$(get_version "${P}cstr")
R=$(post "/fields/update" "{\"name\":\"${P}cstr\",\"label\":\"改类型\",\"type\":\"string\",\"category\":\"combat\",\"properties\":{},\"version\":${VER}}")
assert_code "12.4 被引用时改类型返回 40006" "40006" "$R"

# 删引用者后无引用，可随意修改
disable_then_delete "${P}ref_c"
VER=$(get_version "${P}cstr")
R=$(post "/fields/update" "{\"name\":\"${P}cstr\",\"label\":\"无引用收紧\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"constraints\":{\"min\":50,\"max\":60}},\"version\":${VER}}")
assert_code "12.5 无引用后可收紧" "0" "$R"

# --- string 类型约束 ---
echo "  [INFO] 测试 string/select 约束"
enable_field "${P}name_str"
post "/fields/create" "{\"name\":\"${P}ref_s\",\"label\":\"字符串引用\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"constraints\":{\"refs\":[\"${P}name_str\"]}}}" > /dev/null

VER=$(get_version "${P}name_str")
R=$(post "/fields/update" "{\"name\":\"${P}name_str\",\"label\":\"名称\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{\"constraints\":{\"minLength\":5,\"maxLength\":30}},\"version\":${VER}}")
assert_code "12.6 string 收紧 minLength(1→5) 返回 40007" "40007" "$R"

R=$(post "/fields/update" "{\"name\":\"${P}name_str\",\"label\":\"名称\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{\"constraints\":{\"minLength\":1,\"maxLength\":10}},\"version\":${VER}}")
assert_code "12.7 string 收紧 maxLength(30→10) 返回 40007" "40007" "$R"

disable_then_delete "${P}ref_s"

# --- select 类型约束 ---
enable_field "${P}mood"
post "/fields/create" "{\"name\":\"${P}ref_m\",\"label\":\"选项引用\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"constraints\":{\"refs\":[\"${P}mood\"]}}}" > /dev/null

VER=$(get_version "${P}mood")
R=$(post "/fields/update" "{\"name\":\"${P}mood\",\"label\":\"情绪\",\"type\":\"select\",\"category\":\"personality\",\"properties\":{\"constraints\":{\"options\":[{\"value\":\"happy\"},{\"value\":\"angry\"}]}},\"version\":${VER}}")
assert_code "12.8 select 删除选项(sad) 返回 40007" "40007" "$R"

R=$(post "/fields/update" "{\"name\":\"${P}mood\",\"label\":\"情绪\",\"type\":\"select\",\"category\":\"personality\",\"properties\":{\"constraints\":{\"options\":[{\"value\":\"happy\"},{\"value\":\"sad\"},{\"value\":\"angry\"},{\"value\":\"calm\"}]}},\"version\":${VER}}")
assert_code "12.9 select 新增选项(calm) 允许" "0" "$R"

disable_then_delete "${P}ref_m"

# =============================================================================
# 功能 13：循环引用检测 + 引用关系维护
# 场景 A：创建 reference 字段选择引用（只能引用启用字段）
# 场景 B：编辑 reference 字段，已有的停用引用允许保持
# 场景 C：删除 reference 字段，引用计数自动恢复
# =============================================================================
echo ""
echo "[功能13: 循环引用检测 + 引用关系]"

post "/fields/create" "{\"name\":\"${P}fa\",\"label\":\"基础A\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{}}" > /dev/null
post "/fields/create" "{\"name\":\"${P}fb\",\"label\":\"基础B\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{}}" > /dev/null
enable_field "${P}fa"
enable_field "${P}fb"

# 13.1 创建 reference 引用启用字段
R=$(post "/fields/create" "{\"name\":\"${P}rx\",\"label\":\"引用X\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"constraints\":{\"refs\":[\"${P}fa\",\"${P}fb\"]}}}")
assert_code "13.1 创建 reference 成功" "0" "$R"

# 13.2 被引用字段的 ref_count 增加
R=$(post "/fields/detail" "{\"name\":\"${P}fa\"}")
assert_field "13.2 fa.ref_count=1" ".data.ref_count" "1" "$R"
R=$(post "/fields/detail" "{\"name\":\"${P}fb\"}")
assert_field "13.2 fb.ref_count=1" ".data.ref_count" "1" "$R"

# 13.3 引用详情能看到引用方
R=$(post "/fields/references" "{\"name\":\"${P}fa\"}")
assert_field "13.3 fa 被 rx 引用" ".data.fields | length" "1" "$R"

# 13.4 链式引用（rx → fa,fb; ry → rx）
enable_field "${P}rx"
R=$(post "/fields/create" "{\"name\":\"${P}ry\",\"label\":\"引用Y\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"constraints\":{\"refs\":[\"${P}rx\"]}}}")
assert_code "13.4 链式引用成功" "0" "$R"

# 13.5 循环引用检测（rx 不能反过来引用 ry）
enable_field "${P}ry"
VER=$(get_version "${P}rx")
R=$(post "/fields/update" "{\"name\":\"${P}rx\",\"label\":\"引用X\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"constraints\":{\"refs\":[\"${P}fa\",\"${P}ry\"]}},\"version\":${VER}}")
assert_code "13.5 循环引用返回 40009" "40009" "$R"

# 13.6 引用不存在的字段 — 返回 40014 ErrFieldRefNotFound（不是 40011）
R=$(post "/fields/create" "{\"name\":\"${P}rbad\",\"label\":\"坏引用\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"constraints\":{\"refs\":[\"nonexistent_field\"]}}}")
assert_code "13.6 引用不存在字段返回 40014" "40014" "$R"

# 13.7 引用停用字段 → 新增引用被拒绝
disable_field "${P}fb"
R=$(post "/fields/create" "{\"name\":\"${P}rdis\",\"label\":\"引用停用\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"constraints\":{\"refs\":[\"${P}fb\"]}}}")
assert_code "13.7 新增引用停用字段返回 40013" "40013" "$R"

# 13.8 编辑 reference 字段时，已有的停用引用允许保持
# rx 引用了 fa 和 fb，fb 现在已停用
# 编辑 rx 保留 fb 引用（不新增），应允许
VER=$(get_version "${P}rx")
R=$(post "/fields/update" "{\"name\":\"${P}rx\",\"label\":\"引用X改\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"constraints\":{\"refs\":[\"${P}fa\",\"${P}fb\"]}},\"version\":${VER}}")
assert_code "13.8 保留已有停用引用允许" "0" "$R"

# 13.9 但新增停用字段的引用会被拒绝
# 创建一个新的停用字段
post "/fields/create" "{\"name\":\"${P}fc\",\"label\":\"基础C\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{}}" > /dev/null
VER=$(get_version "${P}rx")
R=$(post "/fields/update" "{\"name\":\"${P}rx\",\"label\":\"引用X\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"constraints\":{\"refs\":[\"${P}fa\",\"${P}fb\",\"${P}fc\"]}},\"version\":${VER}}")
assert_code "13.9 新增停用字段引用返回 40013" "40013" "$R"

# 13.10 被引用字段禁止删除
disable_field "${P}fa"
R=$(post "/fields/delete" "{\"name\":\"${P}fa\"}")
assert_code "13.10 被引用禁止删除返回 40005" "40005" "$R"
# 返回引用详情
assert_field "13.10 返回 deleted=false" ".data.deleted" "false" "$R"
enable_field "${P}fa"

# 13.11 删 reference 字段后 ref_count 恢复
disable_then_delete "${P}ry"
R=$(post "/fields/detail" "{\"name\":\"${P}rx\"}")
assert_field "13.11 rx.ref_count 减为 0" ".data.ref_count" "0" "$R"

disable_then_delete "${P}rx"
R=$(post "/fields/detail" "{\"name\":\"${P}fa\"}")
assert_field "13.11 fa.ref_count 恢复为 0" ".data.ref_count" "0" "$R"
R=$(post "/fields/detail" "{\"name\":\"${P}fb\"}")
assert_field "13.11 fb.ref_count 恢复为 0" ".data.ref_count" "0" "$R"

# =============================================================================
# 功能 4 场景 B：reference 类型改成其他类型，自动清理引用
# =============================================================================
echo ""
echo "[功能4-B: reference 改类型自动清理引用]"

# 用独立字段，不依赖前面功能 13 的数据
post "/fields/create" "{\"name\":\"${P}tgt_x\",\"label\":\"目标X\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{}}" > /dev/null
post "/fields/create" "{\"name\":\"${P}tgt_y\",\"label\":\"目标Y\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{}}" > /dev/null
enable_field "${P}tgt_x"
enable_field "${P}tgt_y"
post "/fields/create" "{\"name\":\"${P}rz\",\"label\":\"将改类型\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"constraints\":{\"refs\":[\"${P}tgt_x\",\"${P}tgt_y\"]}}}" > /dev/null
echo "  [INFO] 创建 ${P}rz(reference) 引用 tgt_x 和 tgt_y"

R=$(post "/fields/detail" "{\"name\":\"${P}tgt_x\"}")
assert_field "4B.1 tgt_x.ref_count=1（被 rz 引用）" ".data.ref_count" "1" "$R"

# 把 rz 从 reference 改成 string — 引用关系自动清理
VER=$(get_version "${P}rz")
R=$(post "/fields/update" "{\"name\":\"${P}rz\",\"label\":\"已改类型\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{},\"version\":${VER}}")
assert_code "4B.2 改类型成功" "0" "$R"

R=$(post "/fields/detail" "{\"name\":\"${P}tgt_x\"}")
assert_field "4B.3 tgt_x.ref_count 恢复为 0" ".data.ref_count" "0" "$R"
R=$(post "/fields/detail" "{\"name\":\"${P}tgt_y\"}")
assert_field "4B.3 tgt_y.ref_count 恢复为 0" ".data.ref_count" "0" "$R"

R=$(post "/fields/detail" "{\"name\":\"${P}rz\"}")
assert_field "4B.4 rz 类型已变为 string" ".data.type" "string" "$R"

# =============================================================================
# 边界场景补充
# =============================================================================
echo ""
echo "[边界场景]"

# 大页码（超出数据量）
R=$(post "/fields/list" '{"page":9999,"page_size":20}')
assert_code "E.1 大页码返回成功（空列表）" "0" "$R"
assert_field "E.1 items 为空" ".data.items | length" "0" "$R"

# page_size 超限（应被裁剪到 max_page_size=100）
R=$(post "/fields/list" '{"page":1,"page_size":999}')
assert_code "E.2 超大 page_size 返回成功" "0" "$R"
assert_le "E.2 page_size 被裁剪" ".data.page_size" "100" "$R"

# =============================================================================
# 清理
# =============================================================================
echo ""
echo "[清理测试数据]"
disable_then_delete "${P}fa"
disable_then_delete "${P}fb"
disable_then_delete "${P}fc"
disable_then_delete "${P}tgt_x"
disable_then_delete "${P}tgt_y"
disable_then_delete "${P}rz"
disable_then_delete "${P}cstr"
disable_then_delete "${P}hp"
disable_then_delete "${P}speed"
disable_then_delete "${P}name_str"
disable_then_delete "${P}mood"
echo "  [INFO] 已清理所有测试数据"

# =============================================================================
echo ""
echo "=========================================="
echo "  测试完成: $TOTAL 个用例"
echo "  通过: $PASS"
echo "  失败: $FAIL"
echo "=========================================="

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
