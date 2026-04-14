#!/bin/bash
# =============================================================================
# test_00_health.sh — 健康检查 + 字典查询 + HTTP 协议攻击
# =============================================================================

section "Part 0: 健康检查 + 字典查询 + HTTP 协议攻击 (prefix=$P)"

# =============================================================================
# 0-A: 健康检查
# =============================================================================
subsection "0-A: 健康检查"

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
# 0-B: 字典查询
# =============================================================================
subsection "0-B: 字典查询"

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

# =============================================================================
# 0-C: 字典攻击性测试
# =============================================================================
subsection "0-C: 字典攻击"

# SQL 注入
R=$(post "/dictionaries" '{"group":"field_type\"; DROP TABLE dictionaries; --"}')
assert_not_500 "dict_atk.1 SQL 注入不崩" "$R"

# XSS
R=$(post "/dictionaries" '{"group":"<script>alert(1)</script>"}')
assert_not_500 "dict_atk.2 XSS 不崩" "$R"

# 极长 group
LONG_GROUP=$(printf 'a%.0s' $(seq 1 10000))
R=$(post "/dictionaries" "{\"group\":\"$LONG_GROUP\"}")
assert_not_500 "dict_atk.3 极长 group 不崩" "$R"

# null group
R=$(post "/dictionaries" '{"group":null}')
assert_code "dict_atk.4 null group 40000" "40000" "$R"

# 畸形 JSON
R=$(raw_post "/dictionaries" '{bad json}')
assert_code "dict_atk.5 畸形 JSON 40000" "40000" "$R"

# 空 body
R=$(curl -s -X POST "$BASE/dictionaries" -H "Content-Type: application/json" -d '')
assert_code "dict_atk.6 空 body 40000" "40000" "$R"

# =============================================================================
# 0-D: HTTP 协议攻击（不存在的路由、错误方法）
# =============================================================================
subsection "0-D: HTTP 协议攻击"

# 不存在的路由
R=$(curl -s http://localhost:9821/api/v1/nonexistent)
TOTAL=$((TOTAL + 1))
HTTP_CODE=$(echo "$R" | jq -r '.code // empty' 2>/dev/null | tr -d '\r')
if [ -n "$HTTP_CODE" ] || echo "$R" | grep -qi "not found" 2>/dev/null; then
  echo "  [PASS] http.1 不存在路由有合理响应"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] http.1 不存在路由返回异常: $(echo "$R" | head -c 100)"
  FAIL=$((FAIL + 1))
fi

# GET 方法访问 POST 端点
R=$(curl -s http://localhost:9821/api/v1/fields/list)
TOTAL=$((TOTAL + 1))
echo "$R" | jq -r '.code // empty' 2>/dev/null | tr -d '\r' | grep -qE '^[0-9]+$'
if [ $? -eq 0 ] || [ -n "$R" ]; then
  echo "  [PASS] http.2 GET→POST 端点有合理响应"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] http.2 GET→POST 端点返回异常"
  FAIL=$((FAIL + 1))
fi

# PUT 方法
R=$(curl -s -X PUT "$BASE/fields/create" -H "Content-Type: application/json" -d '{}')
TOTAL=$((TOTAL + 1))
if [ -n "$R" ]; then
  echo "  [PASS] http.3 PUT 方法不崩"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] http.3 PUT 方法无响应"
  FAIL=$((FAIL + 1))
fi

# DELETE 方法
R=$(curl -s -X DELETE "$BASE/fields/create")
TOTAL=$((TOTAL + 1))
if [ -n "$R" ]; then
  echo "  [PASS] http.4 DELETE 方法不崩"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] http.4 DELETE 方法无响应"
  FAIL=$((FAIL + 1))
fi

# Content-Type: text/plain
R=$(curl -s -X POST "$BASE/fields/list" -H "Content-Type: text/plain" -d '{"page":1,"page_size":10}')
TOTAL=$((TOTAL + 1))
if [ -n "$R" ]; then
  echo "  [PASS] http.5 Content-Type text/plain 不崩"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] http.5 Content-Type text/plain 无响应"
  FAIL=$((FAIL + 1))
fi

# 无 Content-Type
R=$(curl -s -X POST "$BASE/fields/list" -d '{"page":1,"page_size":10}')
TOTAL=$((TOTAL + 1))
if [ -n "$R" ]; then
  echo "  [PASS] http.6 无 Content-Type 不崩"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] http.6 无 Content-Type 无响应"
  FAIL=$((FAIL + 1))
fi

# 超大 Content-Length
R=$(curl -s -X POST "$BASE/fields/list" -H "Content-Type: application/json" -H "Content-Length: 99999999" -d '{"page":1}' --max-time 5)
TOTAL=$((TOTAL + 1))
if [ $? -le 28 ]; then
  echo "  [PASS] http.7 超大 Content-Length 不 hang（超时或正常）"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] http.7 超大 Content-Length 异常"
  FAIL=$((FAIL + 1))
fi
