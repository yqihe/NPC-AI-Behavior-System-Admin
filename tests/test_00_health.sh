#!/bin/bash
# =============================================================================
# test_00_health.sh — 健康检查 + 字典查询
#
# 前置：run_all.sh 已 source helpers.sh，$BASE / $P / assert_* / post() 可用
# =============================================================================

section "Part 0: 健康检查 + 字典查询 (prefix=$P)"

# ---- 健康检查 ----
subsection "健康检查"

HEALTH=$(curl -s http://localhost:9821/health)
TOTAL=$((TOTAL + 1))
if echo "$HEALTH" | jq -e '.status == "ok"' > /dev/null 2>&1; then
  echo "  [PASS] 服务就绪"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] 服务未就绪，终止测试"
  exit 1
fi

# ---- 字典查询 ----
subsection "字典查询"

R=$(post "/dictionaries" '{"group":"field_type"}')
assert_code   "dict.1 field_type 成功"     "0" "$R"
assert_field  "dict.1 返回 6 种类型"       ".data.items | length" "6" "$R"

R=$(post "/dictionaries" '{"group":"field_category"}')
assert_code   "dict.2 field_category 成功" "0" "$R"
assert_field  "dict.2 返回 6 种分类"       ".data.items | length" "6" "$R"

R=$(post "/dictionaries" '{"group":"field_properties"}')
assert_code   "dict.3 field_properties 成功" "0" "$R"

R=$(post "/dictionaries" '{"group":""}')
assert_code   "dict.4 空 group 返回参数错误" "40000" "$R"

R=$(post "/dictionaries" '{"group":"nonexistent"}')
assert_code   "dict.5 不存在 group 返回成功（空列表）" "0" "$R"
assert_field  "dict.5 空列表"              ".data.items | length" "0" "$R"

# 验证字典返回的结构完整性（每项 {name, label}）
R=$(post "/dictionaries" '{"group":"field_category"}')
assert_not_equal "dict.6 category items[0].name 非空" ".data.items[0].name" "null" "$R"
assert_not_equal "dict.6 category items[0].label 非空" ".data.items[0].label" "null" "$R"
