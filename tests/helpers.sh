#!/bin/bash
# =============================================================================
# 共享工具函数 + 断言 + 各模块辅助方法
#
# 被 run_all.sh source，所有 test_*.sh 共享。
# =============================================================================

# ---- 全局变量 ----
BASE="http://localhost:9821/api/v1"
EXPORT_BASE="http://localhost:9821/api/configs"
PASS=0
FAIL=0
TOTAL=0
BUGS=()
TS=$(date +%s)
P="t${TS}_"

# ---- 输出格式 ----
section() {
  echo ""
  echo "=============================================================="
  echo "  $1"
  echo "=============================================================="
}

subsection() {
  echo ""
  echo "--- $1 ---"
}

# ---- HTTP ----
post() {
  printf '%s' "$2" | curl -s -X POST "$BASE$1" -H "Content-Type: application/json; charset=utf-8" --data-binary @-
}

get_export() {
  curl -s "$EXPORT_BASE$1"
}

# ---- 断言 ----

assert_code() {
  local name="$1" expected="$2" body="$3"
  TOTAL=$((TOTAL + 1))
  local actual=$(echo "$body" | jq -r '.code // empty' 2>/dev/null | tr -d '\r')
  if [ "$actual" = "$expected" ]; then
    echo "  [PASS] $name"
    PASS=$((PASS + 1))
  else
    echo "  [FAIL] $name — 期望 code=$expected, 实际: $actual"
    echo "         响应: $(echo "$body" | head -c 200)"
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

assert_not_equal() {
  local name="$1" expr="$2" unexpected="$3" body="$4"
  TOTAL=$((TOTAL + 1))
  local actual=$(echo "$body" | jq -r "$expr" 2>/dev/null | tr -d '\r')
  if [ "$actual" != "$unexpected" ]; then
    echo "  [PASS] $name (=$actual)"
    PASS=$((PASS + 1))
  else
    echo "  [FAIL] $name — 不应为 $unexpected, 实际: $actual"
    FAIL=$((FAIL + 1))
  fi
}

assert_code_in() {
  local name="$1" allowed="$2" body="$3"
  TOTAL=$((TOTAL + 1))
  local actual=$(echo "$body" | jq -r '.code // empty' 2>/dev/null | tr -d '\r')
  for c in $allowed; do
    if [ "$actual" = "$c" ]; then
      echo "  [PASS] $name (code=$actual)"
      PASS=$((PASS + 1))
      return
    fi
  done
  echo "  [FAIL] $name — 期望 code ∈ {$allowed}, 实际: $actual"
  echo "         响应: $(echo "$body" | head -c 200)"
  FAIL=$((FAIL + 1))
}

assert_bug() {
  local name="$1" expected="$2" body="$3" bug_desc="$4"
  TOTAL=$((TOTAL + 1))
  local actual=$(echo "$body" | jq -r '.code // empty' 2>/dev/null | tr -d '\r')
  if [ "$actual" = "$expected" ]; then
    echo "  [PASS] $name"
    PASS=$((PASS + 1))
  else
    echo "  [BUG ] $name — 期望 code=$expected, 实际 code=$actual"
    echo "         bug: $bug_desc"
    echo "         响应: $(echo "$body" | head -c 200)"
    FAIL=$((FAIL + 1))
    BUGS+=("$name: $bug_desc")
  fi
}

# ---- 字段辅助 ----
fld_detail()     { post "/fields/detail" "{\"id\":$1}"; }
fld_version()    { fld_detail "$1" | jq -r '.data.version' | tr -d '\r'; }
fld_refcount()   { fld_detail "$1" | jq -r '.data.ref_count' | tr -d '\r'; }
fld_enabled()    { fld_detail "$1" | jq -r '.data.enabled' | tr -d '\r'; }
fld_enable()     { local ver=$(fld_version "$1"); post "/fields/toggle-enabled" "{\"id\":$1,\"enabled\":true,\"version\":${ver}}" > /dev/null; }
fld_disable()    { local ver=$(fld_version "$1"); post "/fields/toggle-enabled" "{\"id\":$1,\"enabled\":false,\"version\":${ver}}" > /dev/null; }
fld_rm()         { fld_disable "$1" 2>/dev/null; post "/fields/delete" "{\"id\":$1}" > /dev/null 2>&1; }

# ---- 模板辅助 ----
tpl_detail()     { post "/templates/detail" "{\"id\":$1}"; }
tpl_version()    { tpl_detail "$1" | jq -r '.data.version' | tr -d '\r'; }
tpl_refcount()   { tpl_detail "$1" | jq -r '.data.ref_count' | tr -d '\r'; }
tpl_enable()     { local ver=$(tpl_version "$1"); post "/templates/toggle-enabled" "{\"id\":$1,\"enabled\":true,\"version\":${ver}}" > /dev/null; }
tpl_disable()    { local ver=$(tpl_version "$1"); post "/templates/toggle-enabled" "{\"id\":$1,\"enabled\":false,\"version\":${ver}}" > /dev/null; }
tpl_rm()         { tpl_disable "$1" 2>/dev/null; post "/templates/delete" "{\"id\":$1}" > /dev/null 2>&1; }

# ---- 事件类型辅助 ----
et_detail()      { post "/event-types/detail" "{\"id\":$1}"; }
et_version()     { et_detail "$1" | jq -r '.data.version' | tr -d '\r'; }
et_enable()      { local ver=$(et_version "$1"); post "/event-types/toggle-enabled" "{\"id\":$1,\"enabled\":true,\"version\":${ver}}" > /dev/null; }
et_disable()     { local ver=$(et_version "$1"); post "/event-types/toggle-enabled" "{\"id\":$1,\"enabled\":false,\"version\":${ver}}" > /dev/null; }
et_rm()          { et_disable "$1" 2>/dev/null; post "/event-types/delete" "{\"id\":$1}" > /dev/null 2>&1; }

# ---- 状态机辅助 ----
fsm_detail()     { post "/fsm-configs/detail" "{\"id\":$1}"; }
fsm_version()    { fsm_detail "$1" | jq -r '.data.version' | tr -d '\r'; }
fsm_enable()     { local ver=$(fsm_version "$1"); post "/fsm-configs/toggle-enabled" "{\"id\":$1,\"enabled\":true,\"version\":${ver}}" > /dev/null; }
fsm_disable()    { local ver=$(fsm_version "$1"); post "/fsm-configs/toggle-enabled" "{\"id\":$1,\"enabled\":false,\"version\":${ver}}" > /dev/null; }
fsm_rm()         { fsm_disable "$1" 2>/dev/null; post "/fsm-configs/delete" "{\"id\":$1}" > /dev/null 2>&1; }

# ---- 汇总 ----
print_summary() {
  echo ""
  section "汇总"
  echo ""
  echo "  总计: $TOTAL   通过: $PASS   失败: $FAIL"
  echo ""
  if [ "${#BUGS[@]}" -gt 0 ]; then
    echo "--------- 攻击命中的可疑 bug ---------"
    for b in "${BUGS[@]}"; do
      echo "  * $b"
    done
    echo "-------------------------------------"
  fi
  echo ""
}
