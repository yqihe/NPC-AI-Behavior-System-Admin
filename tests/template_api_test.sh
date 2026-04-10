#!/bin/bash
# =============================================================================
# 模板管理 API 全方位集成测试
# 运行前提：docker compose up -d --build admin-backend && seed 已执行
# 用法：bash tests/template_api_test.sh
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

# 字段辅助
field_get_version() {
  post "/fields/detail" "{\"id\":$1}" | jq -r '.data.version' | tr -d '\r'
}

field_enable() {
  local id="$1"
  local ver=$(field_get_version "$id")
  post "/fields/toggle-enabled" "{\"id\":${id},\"enabled\":true,\"version\":${ver}}" > /dev/null
}

field_disable() {
  local id="$1"
  local ver=$(field_get_version "$id")
  post "/fields/toggle-enabled" "{\"id\":${id},\"enabled\":false,\"version\":${ver}}" > /dev/null
}

field_get_refcount() {
  post "/fields/detail" "{\"id\":$1}" | jq -r '.data.ref_count' | tr -d '\r'
}

field_disable_then_delete() {
  local id="$1"
  field_disable "$id"
  post "/fields/delete" "{\"id\":${id}}" > /dev/null 2>&1
}

# 模板辅助
tpl_get_version() {
  post "/templates/detail" "{\"id\":$1}" | jq -r '.data.version' | tr -d '\r'
}

tpl_get_refcount() {
  post "/templates/detail" "{\"id\":$1}" | jq -r '.data.ref_count' | tr -d '\r'
}

tpl_enable() {
  local id="$1"
  local ver=$(tpl_get_version "$id")
  post "/templates/toggle-enabled" "{\"id\":${id},\"enabled\":true,\"version\":${ver}}" > /dev/null
}

tpl_disable() {
  local id="$1"
  local ver=$(tpl_get_version "$id")
  post "/templates/toggle-enabled" "{\"id\":${id},\"enabled\":false,\"version\":${ver}}" > /dev/null
}

tpl_disable_then_delete() {
  local id="$1"
  tpl_disable "$id"
  post "/templates/delete" "{\"id\":${id}}" > /dev/null 2>&1
}

# =============================================================================
echo "=========================================="
echo "  模板管理 API 全方位集成测试 (prefix=$P)"
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
# 准备：创建 3 个启用字段供模板使用
# =============================================================================
echo ""
echo "[准备测试字段]"

R=$(post "/fields/create" "{\"name\":\"${P}f_hp\",\"label\":\"测试HP\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"HP\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}}}")
F_HP=$(echo "$R" | jq -r '.data.id')
field_enable "$F_HP"
assert_not_equal "prep.1 创建并启用 f_hp" ".data.id" "null" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}f_atk\",\"label\":\"测试ATK\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"ATK\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":999}}}")
F_ATK=$(echo "$R" | jq -r '.data.id')
field_enable "$F_ATK"
assert_not_equal "prep.2 创建并启用 f_atk" ".data.id" "null" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}f_name\",\"label\":\"测试名称\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{\"description\":\"name\",\"expose_bb\":false,\"constraints\":{\"minLength\":1,\"maxLength\":50}}}")
F_NAME=$(echo "$R" | jq -r '.data.id')
field_enable "$F_NAME"
assert_not_equal "prep.3 创建并启用 f_name" ".data.id" "null" "$R"

# 一个停用字段，用于测 41005
R=$(post "/fields/create" "{\"name\":\"${P}f_disabled\",\"label\":\"停用字段\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"disabled\",\"expose_bb\":false}}")
F_DISABLED=$(echo "$R" | jq -r '.data.id')
# 不启用

# =============================================================================
# 功能 5：模板名唯一性校验
# =============================================================================
echo ""
echo "[功能5: 模板名唯一性校验]"

R=$(post "/templates/check-name" "{\"name\":\"${P}npc_combat\"}")
assert_code "5.1 未用过的名字" "0" "$R"
assert_field "5.1 available=true" ".data.available" "true" "$R"

R=$(post "/templates/check-name" '{"name":""}')
assert_code "5.2 空名返回 41002" "41002" "$R"

R=$(post "/templates/check-name" '{"name":"BAD-NAME"}')
assert_code "5.3 非法格式返回 41002" "41002" "$R"

R=$(post "/templates/check-name" '{"name":"123start"}')
assert_code "5.4 数字开头返回 41002" "41002" "$R"

# =============================================================================
# 功能 2：新建模板（默认未启用）
# =============================================================================
echo ""
echo "[功能2: 新建模板]"

R=$(post "/templates/create" "{\"name\":\"${P}npc_combat\",\"label\":\"战斗生物模板\",\"description\":\"战斗用 NPC\",\"fields\":[{\"field_id\":${F_HP},\"required\":true},{\"field_id\":${F_ATK},\"required\":true},{\"field_id\":${F_NAME},\"required\":false}]}")
assert_code "2.1 创建成功" "0" "$R"
TPL_COMBAT=$(echo "$R" | jq -r '.data.id')
assert_not_equal "2.1 返回 id > 0" ".data.id" "null" "$R"
assert_field "2.1 返回 name" ".data.name" "${P}npc_combat" "$R"

# 默认未启用 + 字段 ref_count 已 +1
R=$(post "/templates/detail" "{\"id\":${TPL_COMBAT}}")
assert_code "2.2 详情成功" "0" "$R"
assert_field "2.2 默认 enabled=false" ".data.enabled" "false" "$R"
assert_field "2.2 初始 version=1" ".data.version" "1" "$R"
assert_field "2.2 初始 ref_count=0" ".data.ref_count" "0" "$R"
assert_field "2.2 fields 长度 3" ".data.fields | length" "3" "$R"
assert_field "2.2 fields[0] 是 f_hp" ".data.fields[0].name" "${P}f_hp" "$R"
assert_field "2.2 fields[0] required=true" ".data.fields[0].required" "true" "$R"
assert_field "2.2 fields[2] required=false" ".data.fields[2].required" "false" "$R"
assert_field "2.2 category_label 已翻译" ".data.fields[0].category_label" "战斗属性" "$R"

# 字段方 ref_count 已 +1
HP_REF=$(field_get_refcount "$F_HP")
TOTAL=$((TOTAL + 1))
if [ "$HP_REF" = "1" ]; then
  echo "  [PASS] 2.3 f_hp.ref_count=1（创建模板时 +1）"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] 2.3 f_hp.ref_count 期望 1，实际 $HP_REF"
  FAIL=$((FAIL + 1))
fi

# 重复名字
R=$(post "/templates/create" "{\"name\":\"${P}npc_combat\",\"label\":\"重复\",\"description\":\"\",\"fields\":[{\"field_id\":${F_HP},\"required\":true}]}")
assert_code "2.4 重复名字返回 41001" "41001" "$R"

# 非法名字
R=$(post "/templates/create" "{\"name\":\"BAD\",\"label\":\"坏\",\"description\":\"\",\"fields\":[{\"field_id\":${F_HP},\"required\":true}]}")
assert_code "2.5 大写名字返回 41002" "41002" "$R"

# 空 fields
R=$(post "/templates/create" "{\"name\":\"${P}empty\",\"label\":\"空\",\"description\":\"\",\"fields\":[]}")
assert_code "2.6 空 fields 返回 41004" "41004" "$R"

# 字段不存在
R=$(post "/templates/create" "{\"name\":\"${P}not_exist\",\"label\":\"不存在字段\",\"description\":\"\",\"fields\":[{\"field_id\":999999,\"required\":true}]}")
assert_code "2.7 不存在字段返回 41006" "41006" "$R"

# 字段停用
R=$(post "/templates/create" "{\"name\":\"${P}disabled_field\",\"label\":\"停用字段\",\"description\":\"\",\"fields\":[{\"field_id\":${F_DISABLED},\"required\":true}]}")
assert_code "2.8 停用字段返回 41005" "41005" "$R"

# 重复 field_id
R=$(post "/templates/create" "{\"name\":\"${P}dup\",\"label\":\"重复字段\",\"description\":\"\",\"fields\":[{\"field_id\":${F_HP},\"required\":true},{\"field_id\":${F_HP},\"required\":false}]}")
assert_code "2.9 重复 field_id 返回 40000" "40000" "$R"

# 空 label
R=$(post "/templates/create" "{\"name\":\"${P}nolabel\",\"label\":\"\",\"description\":\"\",\"fields\":[{\"field_id\":${F_HP},\"required\":true}]}")
assert_code "2.10 空 label 返回 40000" "40000" "$R"

# =============================================================================
# 功能 5 续：check-name 含软删除
# =============================================================================
R=$(post "/templates/check-name" "{\"name\":\"${P}npc_combat\"}")
assert_code "5.5 已存在的名字" "0" "$R"
assert_field "5.5 available=false" ".data.available" "false" "$R"

# =============================================================================
# 功能 1：模板列表
# =============================================================================
echo ""
echo "[功能1: 模板列表]"

R=$(post "/templates/list" '{"page":1,"page_size":20}')
assert_code "1.1 列表成功" "0" "$R"
assert_ge "1.1 至少 1 条" ".data.total" "1" "$R"
assert_field "1.1 items 是数组" ".data.items | type" "array" "$R"
assert_not_equal "1.1 items[0] 有 id" ".data.items[0].id" "null" "$R"

# 按 label 模糊
R=$(post "/templates/list" '{"label":"战斗生物","page":1,"page_size":20}')
assert_code "1.2 模糊搜索" "0" "$R"
assert_ge "1.2 至少 1 条" ".data.total" "1" "$R"

# 不传 enabled (管理页) → 看到未启用
R=$(post "/templates/list" '{"page":1,"page_size":20}')
assert_code "1.3 管理页查询" "0" "$R"

# enabled=true (NPC 管理页) → 当前模板未启用，不应在列表中
R=$(post "/templates/list" '{"enabled":true,"page":1,"page_size":20}')
assert_code "1.4 NPC 管理页查询" "0" "$R"

# 启用后再查 enabled=true
tpl_enable "$TPL_COMBAT"
R=$(post "/templates/list" '{"enabled":true,"page":1,"page_size":20}')
assert_code "1.5 启用后 enabled=true 查询" "0" "$R"
assert_ge "1.5 至少 1 条启用" ".data.total" "1" "$R"

# 分页边界
R=$(post "/templates/list" '{"page":0,"page_size":0}')
assert_code "1.6 page=0 自动校正" "0" "$R"
assert_field "1.6 page 校正为 1" ".data.page" "1" "$R"

# 空结果
R=$(post "/templates/list" '{"label":"绝对不存在zzz","page":1,"page_size":20}')
assert_code "1.7 空结果" "0" "$R"
assert_field "1.7 items 空数组" ".data.items | length" "0" "$R"

# =============================================================================
# 功能 7：启用/停用切换
# =============================================================================
echo ""
echo "[功能7: 启用/停用切换]"

# 已经在 1.5 启用了，先停用
tpl_disable "$TPL_COMBAT"
R=$(post "/templates/detail" "{\"id\":${TPL_COMBAT}}")
assert_field "7.1 已停用" ".data.enabled" "false" "$R"

# 再启用
tpl_enable "$TPL_COMBAT"
R=$(post "/templates/detail" "{\"id\":${TPL_COMBAT}}")
assert_field "7.2 重新启用" ".data.enabled" "true" "$R"

# 乐观锁冲突
R=$(post "/templates/toggle-enabled" "{\"id\":${TPL_COMBAT},\"enabled\":false,\"version\":999}")
assert_code "7.3 乐观锁冲突 41011" "41011" "$R"

# 不存在的 ID
R=$(post "/templates/toggle-enabled" '{"id":999999,"enabled":true,"version":1}')
assert_code "7.4 不存在 ID 返回 41003" "41003" "$R"

# id=0
R=$(post "/templates/toggle-enabled" '{"id":0,"enabled":true,"version":1}')
assert_code "7.5 id=0 返回 40000" "40000" "$R"

# =============================================================================
# 功能 3：编辑模板
# =============================================================================
echo ""
echo "[功能3: 编辑模板]"

# 启用中编辑 → 41010
TPL_VER=$(tpl_get_version "$TPL_COMBAT")
R=$(post "/templates/update" "{\"id\":${TPL_COMBAT},\"label\":\"不应成功\",\"description\":\"\",\"fields\":[{\"field_id\":${F_HP},\"required\":true}],\"version\":${TPL_VER}}")
assert_code "3.1 启用中编辑返回 41010" "41010" "$R"

# 停用后正常编辑
tpl_disable "$TPL_COMBAT"
TPL_VER=$(tpl_get_version "$TPL_COMBAT")

# 3.2 仅 label/description 改动
R=$(post "/templates/update" "{\"id\":${TPL_COMBAT},\"label\":\"战斗生物模板（改）\",\"description\":\"修改后的描述\",\"fields\":[{\"field_id\":${F_HP},\"required\":true},{\"field_id\":${F_ATK},\"required\":true},{\"field_id\":${F_NAME},\"required\":false}],\"version\":${TPL_VER}}")
assert_code "3.2 仅改 label/description 成功" "0" "$R"

R=$(post "/templates/detail" "{\"id\":${TPL_COMBAT}}")
assert_field "3.2 label 已更新" ".data.label" "战斗生物模板（改）" "$R"
assert_field "3.2 description 已更新" ".data.description" "修改后的描述" "$R"

# 3.3 字段顺序变化
TPL_VER=$(tpl_get_version "$TPL_COMBAT")
R=$(post "/templates/update" "{\"id\":${TPL_COMBAT},\"label\":\"战斗生物模板（改）\",\"description\":\"修改后的描述\",\"fields\":[{\"field_id\":${F_NAME},\"required\":false},{\"field_id\":${F_HP},\"required\":true},{\"field_id\":${F_ATK},\"required\":true}],\"version\":${TPL_VER}}")
assert_code "3.3 顺序变化成功（无 NPC 引用）" "0" "$R"

R=$(post "/templates/detail" "{\"id\":${TPL_COMBAT}}")
assert_field "3.3 fields[0] 现在是 f_name" ".data.fields[0].name" "${P}f_name" "$R"
assert_field "3.3 fields[1] 现在是 f_hp" ".data.fields[1].name" "${P}f_hp" "$R"

# 3.4 移除字段 + 新增字段
R=$(post "/fields/create" "{\"name\":\"${P}f_def\",\"label\":\"测试DEF\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"DEF\",\"expose_bb\":false}}")
F_DEF=$(echo "$R" | jq -r '.data.id')
field_enable "$F_DEF"

TPL_VER=$(tpl_get_version "$TPL_COMBAT")
R=$(post "/templates/update" "{\"id\":${TPL_COMBAT},\"label\":\"战斗生物模板（改）\",\"description\":\"\",\"fields\":[{\"field_id\":${F_HP},\"required\":true},{\"field_id\":${F_DEF},\"required\":true}],\"version\":${TPL_VER}}")
assert_code "3.4 集合变更成功" "0" "$R"

# 验证 ref_count 联动：F_HP 仍是 1, F_ATK 应回到 0, F_NAME 应回到 0, F_DEF 应是 1
HP_REF=$(field_get_refcount "$F_HP")
ATK_REF=$(field_get_refcount "$F_ATK")
NAME_REF=$(field_get_refcount "$F_NAME")
DEF_REF=$(field_get_refcount "$F_DEF")
TOTAL=$((TOTAL + 4))
[ "$HP_REF" = "1" ] && { echo "  [PASS] 3.5a F_HP ref_count=1"; PASS=$((PASS+1)); } || { echo "  [FAIL] 3.5a F_HP ref_count 期望 1 实际 $HP_REF"; FAIL=$((FAIL+1)); }
[ "$ATK_REF" = "0" ] && { echo "  [PASS] 3.5b F_ATK ref_count=0 (移除)"; PASS=$((PASS+1)); } || { echo "  [FAIL] 3.5b F_ATK ref_count 期望 0 实际 $ATK_REF"; FAIL=$((FAIL+1)); }
[ "$NAME_REF" = "0" ] && { echo "  [PASS] 3.5c F_NAME ref_count=0 (移除)"; PASS=$((PASS+1)); } || { echo "  [FAIL] 3.5c F_NAME ref_count 期望 0 实际 $NAME_REF"; FAIL=$((FAIL+1)); }
[ "$DEF_REF" = "1" ] && { echo "  [PASS] 3.5d F_DEF ref_count=1 (新增)"; PASS=$((PASS+1)); } || { echo "  [FAIL] 3.5d F_DEF ref_count 期望 1 实际 $DEF_REF"; FAIL=$((FAIL+1)); }

# 3.6 编辑加入停用字段 → 41005
TPL_VER=$(tpl_get_version "$TPL_COMBAT")
R=$(post "/templates/update" "{\"id\":${TPL_COMBAT},\"label\":\"战斗生物模板（改）\",\"description\":\"\",\"fields\":[{\"field_id\":${F_HP},\"required\":true},{\"field_id\":${F_DEF},\"required\":true},{\"field_id\":${F_DISABLED},\"required\":false}],\"version\":${TPL_VER}}")
assert_code "3.6 加入停用字段返回 41005" "41005" "$R"

# 3.7 加入不存在字段 → 41006
TPL_VER=$(tpl_get_version "$TPL_COMBAT")
R=$(post "/templates/update" "{\"id\":${TPL_COMBAT},\"label\":\"战斗生物模板（改）\",\"description\":\"\",\"fields\":[{\"field_id\":${F_HP},\"required\":true},{\"field_id\":999999,\"required\":false}],\"version\":${TPL_VER}}")
assert_code "3.7 加入不存在字段返回 41006" "41006" "$R"

# 3.8 乐观锁冲突
R=$(post "/templates/update" "{\"id\":${TPL_COMBAT},\"label\":\"锁测试\",\"description\":\"\",\"fields\":[{\"field_id\":${F_HP},\"required\":true}],\"version\":999}")
assert_code "3.8 乐观锁冲突返回 41011" "41011" "$R"

# 3.9 不存在的 ID
R=$(post "/templates/update" '{"id":999999,"label":"无","description":"","fields":[{"field_id":1,"required":true}],"version":1}')
assert_code "3.9 不存在 ID 返回 41003" "41003" "$R"

# 3.10 空 fields
TPL_VER=$(tpl_get_version "$TPL_COMBAT")
R=$(post "/templates/update" "{\"id\":${TPL_COMBAT},\"label\":\"x\",\"description\":\"\",\"fields\":[],\"version\":${TPL_VER}}")
assert_code "3.10 空 fields 返回 41004" "41004" "$R"

# =============================================================================
# 功能 6：模板引用详情（NPC 占位）
# =============================================================================
echo ""
echo "[功能6: 引用详情（NPC 占位）]"

R=$(post "/templates/references" "{\"id\":${TPL_COMBAT}}")
assert_code "6.1 引用详情成功" "0" "$R"
assert_field "6.1 template_id 正确" ".data.template_id" "$TPL_COMBAT" "$R"
assert_field "6.1 npcs 空数组（NPC 未上线）" ".data.npcs | length" "0" "$R"

R=$(post "/templates/references" '{"id":999999}')
assert_code "6.2 不存在 ID 返回 41003" "41003" "$R"

# =============================================================================
# 字段引用详情：补 template label（T9 验证）
# =============================================================================
echo ""
echo "[T9: 字段引用详情补 template label]"

R=$(post "/fields/references" "{\"id\":${F_HP}}")
assert_code "T9.1 查询 F_HP 引用详情" "0" "$R"
assert_ge "T9.1 至少 1 个模板引用" ".data.templates | length" "1" "$R"
# 第一个 template label 应该是真实的模板 label，不是 "模板#xxx"
TPL_LABEL=$(echo "$R" | jq -r '.data.templates[0].label' | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$TPL_LABEL" = "战斗生物模板（改）" ]; then
  echo "  [PASS] T9.2 template label 已正确补全"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] T9.2 template label 期望 '战斗生物模板（改）'，实际 '$TPL_LABEL'"
  FAIL=$((FAIL + 1))
fi

# =============================================================================
# 功能 4：删除模板
# =============================================================================
echo ""
echo "[功能4: 删除模板]"

# 启用中删除 → 41009
tpl_enable "$TPL_COMBAT"
R=$(post "/templates/delete" "{\"id\":${TPL_COMBAT}}")
assert_code "4.1 启用中删除返回 41009" "41009" "$R"

# 停用后删除成功
tpl_disable "$TPL_COMBAT"
R=$(post "/templates/delete" "{\"id\":${TPL_COMBAT}}")
assert_code "4.2 停用后删除成功" "0" "$R"
assert_field "4.2 返回 id" ".data.id" "$TPL_COMBAT" "$R"

# 字段 ref_count 应回退（F_HP 和 F_DEF 都应该回到 0）
HP_REF=$(field_get_refcount "$F_HP")
DEF_REF=$(field_get_refcount "$F_DEF")
TOTAL=$((TOTAL + 2))
[ "$HP_REF" = "0" ] && { echo "  [PASS] 4.3a F_HP ref_count=0 (删除模板回退)"; PASS=$((PASS+1)); } || { echo "  [FAIL] 4.3a F_HP ref_count 期望 0 实际 $HP_REF"; FAIL=$((FAIL+1)); }
[ "$DEF_REF" = "0" ] && { echo "  [PASS] 4.3b F_DEF ref_count=0 (删除模板回退)"; PASS=$((PASS+1)); } || { echo "  [FAIL] 4.3b F_DEF ref_count 期望 0 实际 $DEF_REF"; FAIL=$((FAIL+1)); }

# 已删除的查不到
R=$(post "/templates/detail" "{\"id\":${TPL_COMBAT}}")
assert_code "4.4 已删除查不到" "41003" "$R"

# 已删除的 name 不能复用
R=$(post "/templates/check-name" "{\"name\":\"${P}npc_combat\"}")
assert_field "4.5 已删除 name 不可复用" ".data.available" "false" "$R"

# 不存在的 ID
R=$(post "/templates/delete" '{"id":999999}')
assert_code "4.6 不存在 ID 返回 41003" "41003" "$R"

# id=0
R=$(post "/templates/delete" '{"id":0}')
assert_code "4.7 id=0 返回 40000" "40000" "$R"

# =============================================================================
# 攻击性测试
# =============================================================================
echo ""
echo "[攻击性测试]"

# 字段 ID 0
R=$(post "/templates/create" "{\"name\":\"${P}atk1\",\"label\":\"x\",\"description\":\"\",\"fields\":[{\"field_id\":0,\"required\":true}]}")
assert_code "ATK.1 field_id=0 返回 40000" "40000" "$R"

# 负 field_id
R=$(post "/templates/create" "{\"name\":\"${P}atk2\",\"label\":\"x\",\"description\":\"\",\"fields\":[{\"field_id\":-1,\"required\":true}]}")
assert_code "ATK.2 field_id=-1 返回 40000" "40000" "$R"

# 畸形 JSON
R=$(curl -s -X POST "$BASE/templates/create" -H "Content-Type: application/json" -d '{bad json}')
assert_code "ATK.3 畸形 JSON 返回 40000" "40000" "$R"

# 空 body
R=$(curl -s -X POST "$BASE/templates/create" -H "Content-Type: application/json" -d '')
assert_code "ATK.4 空 body 返回 40000" "40000" "$R"

# 超长 name
LONG_NAME="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
R=$(post "/templates/create" "{\"name\":\"${LONG_NAME}\",\"label\":\"x\",\"description\":\"\",\"fields\":[{\"field_id\":${F_HP},\"required\":true}]}")
assert_code "ATK.5 超长 name 返回 41002" "41002" "$R"

# =============================================================================
# 清理测试数据
# =============================================================================
echo ""
echo "[清理测试数据]"

# 模板已删除，清字段
for ID in $F_HP $F_ATK $F_NAME $F_DEF $F_DISABLED; do
  if [ -n "$ID" ] && [ "$ID" != "null" ]; then
    field_disable_then_delete "$ID"
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
