#!/bin/bash
# =============================================================================
# test_03_field_template.sh — 跨模块集成：字段 <-> 模板
#
# 前置：run_all.sh 已 source helpers.sh + test_01 + test_02
#       可用变量：F_HP F_ATK F_NAME F_DEF F_DISABLED CB 等
#       可用函数：post() tpl_* fld_* assert_*
# =============================================================================

section "Part 3: 跨模块集成 — 字段 x 模板 (prefix=$P)"

# =============================================================================
# 3.0 准备独立字段池（避免前面测试的删除操作污染）
# =============================================================================
subsection "3.0 准备跨模块测试字段池"

R=$(post "/fields/create" "{\"name\":\"${P}x_hp\",\"label\":\"X_HP\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"HP\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}}}")
X_HP=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$X_HP"

R=$(post "/fields/create" "{\"name\":\"${P}x_atk\",\"label\":\"X_ATK\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"ATK\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":999}}}")
X_ATK=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$X_ATK"

R=$(post "/fields/create" "{\"name\":\"${P}x_def\",\"label\":\"X_DEF\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"DEF\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":500}}}")
X_DEF=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$X_DEF"

R=$(post "/fields/create" "{\"name\":\"${P}x_name\",\"label\":\"X_NAME\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{\"description\":\"name\",\"expose_bb\":false,\"constraints\":{\"minLength\":1,\"maxLength\":50}}}")
X_NAME=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$X_NAME"

R=$(post "/fields/create" "{\"name\":\"${P}x_spd\",\"label\":\"X_SPD\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"SPD\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}}}")
X_SPD=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$X_SPD"

# =============================================================================
# 3.1 模板创建 -> 字段 ref_count 递增
# =============================================================================
subsection "3.1 模板创建递增 ref_count"

# 创建前确认 ref_count=0
TOTAL=$((TOTAL + 2))
HP_RC=$(fld_refcount "$X_HP")
ATK_RC=$(fld_refcount "$X_ATK")
[ "$HP_RC" = "0" ] && { echo "  [PASS] x1.0a X_HP 初始 ref_count=0"; PASS=$((PASS+1)); } || { echo "  [FAIL] x1.0a 期望 0 实际 $HP_RC"; FAIL=$((FAIL+1)); }
[ "$ATK_RC" = "0" ] && { echo "  [PASS] x1.0b X_ATK 初始 ref_count=0"; PASS=$((PASS+1)); } || { echo "  [FAIL] x1.0b 期望 0 实际 $ATK_RC"; FAIL=$((FAIL+1)); }

R=$(post "/templates/create" "{\"name\":\"${P}x_tpl1\",\"label\":\"跨模块模板1\",\"description\":\"\",\"fields\":[{\"field_id\":${X_HP},\"required\":true},{\"field_id\":${X_ATK},\"required\":true}]}")
assert_code "x1.1 创建模板1成功" "0" "$R"
X_TPL1=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

TOTAL=$((TOTAL + 2))
HP_RC=$(fld_refcount "$X_HP")
ATK_RC=$(fld_refcount "$X_ATK")
[ "$HP_RC" = "1" ] && { echo "  [PASS] x1.2a X_HP ref_count=1"; PASS=$((PASS+1)); } || { echo "  [FAIL] x1.2a 期望 1 实际 $HP_RC"; FAIL=$((FAIL+1)); }
[ "$ATK_RC" = "1" ] && { echo "  [PASS] x1.2b X_ATK ref_count=1"; PASS=$((PASS+1)); } || { echo "  [FAIL] x1.2b 期望 1 实际 $ATK_RC"; FAIL=$((FAIL+1)); }

# =============================================================================
# 3.2 第二个模板引用同一字段 -> ref_count=2
# =============================================================================
subsection "3.2 第二个模板引用同字段 ref_count=2"

R=$(post "/templates/create" "{\"name\":\"${P}x_tpl2\",\"label\":\"跨模块模板2\",\"description\":\"\",\"fields\":[{\"field_id\":${X_HP},\"required\":false},{\"field_id\":${X_DEF},\"required\":true}]}")
assert_code "x2.1 创建模板2成功" "0" "$R"
X_TPL2=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

TOTAL=$((TOTAL + 2))
HP_RC=$(fld_refcount "$X_HP")
DEF_RC=$(fld_refcount "$X_DEF")
[ "$HP_RC" = "2" ] && { echo "  [PASS] x2.2a X_HP ref_count=2"; PASS=$((PASS+1)); } || { echo "  [FAIL] x2.2a 期望 2 实际 $HP_RC"; FAIL=$((FAIL+1)); }
[ "$DEF_RC" = "1" ] && { echo "  [PASS] x2.2b X_DEF ref_count=1"; PASS=$((PASS+1)); } || { echo "  [FAIL] x2.2b 期望 1 实际 $DEF_RC"; FAIL=$((FAIL+1)); }

# =============================================================================
# 3.3 模板删除 -> 字段 ref_count 递减
# =============================================================================
subsection "3.3 模板删除递减 ref_count"

tpl_disable "$X_TPL2"
R=$(post "/templates/delete" "{\"id\":${X_TPL2}}")
assert_code "x3.1 删除模板2成功" "0" "$R"

TOTAL=$((TOTAL + 2))
HP_RC=$(fld_refcount "$X_HP")
DEF_RC=$(fld_refcount "$X_DEF")
[ "$HP_RC" = "1" ] && { echo "  [PASS] x3.2a X_HP ref_count=1（回退）"; PASS=$((PASS+1)); } || { echo "  [FAIL] x3.2a 期望 1 实际 $HP_RC"; FAIL=$((FAIL+1)); }
[ "$DEF_RC" = "0" ] && { echo "  [PASS] x3.2b X_DEF ref_count=0（回退）"; PASS=$((PASS+1)); } || { echo "  [FAIL] x3.2b 期望 0 实际 $DEF_RC"; FAIL=$((FAIL+1)); }

# =============================================================================
# 3.4 模板 update (add/remove 字段) -> ref_count 调整
# =============================================================================
subsection "3.4 模板 update 调整 ref_count"

# 当前 X_TPL1 包含 [X_HP, X_ATK]
# 改为 [X_HP, X_DEF]（移除 X_ATK，新增 X_DEF）
V=$(tpl_version "$X_TPL1")
R=$(post "/templates/update" "{\"id\":${X_TPL1},\"label\":\"跨模块模板1\",\"description\":\"\",\"fields\":[{\"field_id\":${X_HP},\"required\":true},{\"field_id\":${X_DEF},\"required\":true}],\"version\":${V}}")
assert_code "x4.1 update 字段集合变化" "0" "$R"

TOTAL=$((TOTAL + 3))
HP_RC=$(fld_refcount "$X_HP")
ATK_RC=$(fld_refcount "$X_ATK")
DEF_RC=$(fld_refcount "$X_DEF")
[ "$HP_RC" = "1" ]  && { echo "  [PASS] x4.2a X_HP ref_count=1（保持）"; PASS=$((PASS+1)); }  || { echo "  [FAIL] x4.2a 期望 1 实际 $HP_RC"; FAIL=$((FAIL+1)); }
[ "$ATK_RC" = "0" ] && { echo "  [PASS] x4.2b X_ATK ref_count=0（被移除）"; PASS=$((PASS+1)); } || { echo "  [FAIL] x4.2b 期望 0 实际 $ATK_RC"; FAIL=$((FAIL+1)); }
[ "$DEF_RC" = "1" ] && { echo "  [PASS] x4.2c X_DEF ref_count=1（被新增）"; PASS=$((PASS+1)); } || { echo "  [FAIL] x4.2c 期望 1 实际 $DEF_RC"; FAIL=$((FAIL+1)); }

# =============================================================================
# 3.5 纯顺序/required 变化 -> ref_count 不变
# =============================================================================
subsection "3.5 纯顺序+required 变化 ref_count 不变"

V=$(tpl_version "$X_TPL1")
R=$(post "/templates/update" "{\"id\":${X_TPL1},\"label\":\"跨模块模板1\",\"description\":\"\",\"fields\":[{\"field_id\":${X_DEF},\"required\":false},{\"field_id\":${X_HP},\"required\":false}],\"version\":${V}}")
assert_code "x5.1 纯排序+required 变化成功" "0" "$R"

TOTAL=$((TOTAL + 2))
HP_RC=$(fld_refcount "$X_HP")
DEF_RC=$(fld_refcount "$X_DEF")
[ "$HP_RC" = "1" ]  && { echo "  [PASS] x5.2a X_HP ref_count 不变"; PASS=$((PASS+1)); }  || { echo "  [FAIL] x5.2a 期望 1 实际 $HP_RC"; FAIL=$((FAIL+1)); }
[ "$DEF_RC" = "1" ] && { echo "  [PASS] x5.2b X_DEF ref_count 不变"; PASS=$((PASS+1)); } || { echo "  [FAIL] x5.2b 期望 1 实际 $DEF_RC"; FAIL=$((FAIL+1)); }

# =============================================================================
# 3.6 删除被模板引用的字段 -> 40005
# =============================================================================
subsection "3.6 被引用字段不可删除"

fld_disable "$X_HP"
R=$(post "/fields/delete" "{\"id\":${X_HP}}")
assert_code "x6.1 被引用字段删除 40005" "40005" "$R"
fld_enable "$X_HP"

# =============================================================================
# 3.7 停用被引用字段 -> 允许，模板 detail 反映 enabled=false
# =============================================================================
subsection "3.7 停用被引用字段（允许）"

fld_disable "$X_HP"
TOTAL=$((TOTAL + 1))
EN=$(fld_enabled "$X_HP")
if [ "$EN" = "false" ]; then
  echo "  [PASS] x7.1 允许停用被引用字段"
  PASS=$((PASS+1))
else
  echo "  [FAIL] x7.1 应能停用 实际 $EN"
  FAIL=$((FAIL+1))
fi

R=$(tpl_detail "$X_TPL1")
TOTAL=$((TOTAL + 1))
HP_EN=$(echo "$R" | jq -r --arg id "$X_HP" '.data.fields[] | select(.field_id == ($id | tonumber)) | .enabled' | tr -d '\r')
if [ "$HP_EN" = "false" ]; then
  echo "  [PASS] x7.2 模板详情 X_HP.enabled=false"
  PASS=$((PASS+1))
else
  echo "  [FAIL] x7.2 期望 false 实际 $HP_EN"
  FAIL=$((FAIL+1))
fi

fld_enable "$X_HP"

# =============================================================================
# 3.8 被引用字段禁止改类型 (40006)
# =============================================================================
subsection "3.8 被引用字段禁止改类型"

fld_disable "$X_HP"
V=$(fld_version "$X_HP")
R=$(post "/fields/update" "{\"id\":${X_HP},\"label\":\"X_HP\",\"type\":\"string\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"minLength\":0,\"maxLength\":100}},\"version\":${V}}")
assert_code "x8.1 被引用改 type 40006" "40006" "$R"

# =============================================================================
# 3.9 被引用字段纯 label 修改 -> 允许
# =============================================================================
subsection "3.9 被引用字段纯 label 修改（允许）"

V=$(fld_version "$X_HP")
R=$(post "/fields/update" "{\"id\":${X_HP},\"label\":\"X_HP_Renamed\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"HP renamed\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}},\"version\":${V}}")
assert_code "x9.1 纯 label 修改允许" "0" "$R"
fld_enable "$X_HP"

# =============================================================================
# 3.10 缓存一致性：字段 detail -> 模板编辑 -> 字段 detail
# =============================================================================
subsection "3.10 缓存一致性"

# 先读 X_NAME（回填缓存），当前 ref_count=0
R=$(fld_detail "$X_NAME")
assert_field "x10.0 X_NAME ref_count=0（初始）" ".data.ref_count" "0" "$R"

# 编辑模板加入 X_NAME
V=$(tpl_version "$X_TPL1")
R=$(post "/templates/update" "{\"id\":${X_TPL1},\"label\":\"跨模块模板1\",\"description\":\"\",\"fields\":[{\"field_id\":${X_DEF},\"required\":false},{\"field_id\":${X_HP},\"required\":false},{\"field_id\":${X_NAME},\"required\":true}],\"version\":${V}}")
assert_code "x10.1 模板加入 X_NAME 成功" "0" "$R"

# 立即读字段 detail，ref_count 应为 1（缓存已失效）
R=$(fld_detail "$X_NAME")
assert_field "x10.2 X_NAME ref_count=1" ".data.ref_count" "1" "$R"

# 再读一次确认缓存稳定
R=$(fld_detail "$X_NAME")
assert_field "x10.3 第二次读 ref_count 仍为 1" ".data.ref_count" "1" "$R"

# =============================================================================
# 3.11 字段 references API 显示模板引用
# =============================================================================
subsection "3.11 字段 references API"

R=$(post "/fields/references" "{\"id\":${X_NAME}}")
assert_code "x11.1 字段引用详情成功" "0" "$R"
assert_ge   "x11.1 至少 1 个模板" ".data.templates | length" "1" "$R"

TOTAL=$((TOTAL + 1))
TPL_LABEL=$(echo "$R" | jq -r '.data.templates[0].label' | tr -d '\r')
if [ "$TPL_LABEL" = "跨模块模板1" ]; then
  echo "  [PASS] x11.2 template label 正确"
  PASS=$((PASS+1))
else
  echo "  [FAIL] x11.2 期望 '跨模块模板1' 实际 '$TPL_LABEL'"
  FAIL=$((FAIL+1))
fi

# 未被引用的字段 references 应为空
R=$(post "/fields/references" "{\"id\":${X_SPD}}")
assert_code  "x11.3 未引用字段 references" "0" "$R"
assert_field "x11.3 templates 空数组" ".data.templates | length" "0" "$R"

# =============================================================================
# 3.12 大模板 (50 字段) 创建+删除 ref_count 联动
# =============================================================================
subsection "3.12 大模板（50字段）ref_count 联动"

BIG_FIELDS=""
BIG_IDS=()
for i in $(seq 1 50); do
  R=$(post "/fields/create" "{\"name\":\"${P}big_${i}\",\"label\":\"big${i}\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}}}")
  ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
  fld_enable "$ID"
  BIG_IDS+=("$ID")
  if [ -z "$BIG_FIELDS" ]; then
    BIG_FIELDS="{\"field_id\":${ID},\"required\":false}"
  else
    BIG_FIELDS="${BIG_FIELDS},{\"field_id\":${ID},\"required\":false}"
  fi
done

R=$(post "/templates/create" "{\"name\":\"${P}x_big_tpl\",\"label\":\"大模板\",\"description\":\"\",\"fields\":[${BIG_FIELDS}]}")
assert_code "x12.1 50 字段模板创建成功" "0" "$R"
BIG_TPL=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

R=$(tpl_detail "$BIG_TPL")
assert_field "x12.2 fields 长度=50" ".data.fields | length" "50" "$R"

# 抽样验证 ref_count（第 1、25、50 个）
TOTAL=$((TOTAL + 3))
for idx in 0 24 49; do
  ID="${BIG_IDS[$idx]}"
  RC=$(fld_refcount "$ID")
  if [ "$RC" = "1" ]; then
    echo "  [PASS] x12.3 big_$((idx+1)) ref_count=1"
    PASS=$((PASS+1))
  else
    echo "  [FAIL] x12.3 big_$((idx+1)) 期望 1 实际 $RC"
    FAIL=$((FAIL+1))
  fi
done

# 删除大模板 -> 全部归零
tpl_disable "$BIG_TPL"
R=$(post "/templates/delete" "{\"id\":${BIG_TPL}}")
assert_code "x12.4 大模板删除成功" "0" "$R"

TOTAL=$((TOTAL + 3))
for idx in 0 24 49; do
  ID="${BIG_IDS[$idx]}"
  RC=$(fld_refcount "$ID")
  if [ "$RC" = "0" ]; then
    echo "  [PASS] x12.5 big_$((idx+1)) ref_count=0"
    PASS=$((PASS+1))
  else
    echo "  [FAIL] x12.5 big_$((idx+1)) 期望 0 实际 $RC"
    FAIL=$((FAIL+1))
  fi
done

# =============================================================================
# 3.13 ATTACK: 快速创建/删除模板循环 -> ref_count 一致性
# =============================================================================
subsection "3.13 ATTACK: 快速创建/删除循环"

# 使用 X_SPD 作为测试字段（当前 ref_count=0）
TOTAL=$((TOTAL + 1))
SPD_RC=$(fld_refcount "$X_SPD")
[ "$SPD_RC" = "0" ] && { echo "  [PASS] x13.0 X_SPD 初始 ref_count=0"; PASS=$((PASS+1)); } || { echo "  [FAIL] x13.0 期望 0 实际 $SPD_RC"; FAIL=$((FAIL+1)); }

for cycle in $(seq 1 5); do
  R=$(post "/templates/create" "{\"name\":\"${P}cycle_${cycle}\",\"label\":\"cycle${cycle}\",\"description\":\"\",\"fields\":[{\"field_id\":${X_SPD},\"required\":true}]}")
  CYC_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
  tpl_rm "$CYC_ID" 2>/dev/null
done

TOTAL=$((TOTAL + 1))
SPD_RC=$(fld_refcount "$X_SPD")
if [ "$SPD_RC" = "0" ]; then
  echo "  [PASS] x13.1 5轮创建/删除后 X_SPD ref_count=0"
  PASS=$((PASS+1))
else
  echo "  [FAIL] x13.1 5轮后期望 0 实际 $SPD_RC"
  FAIL=$((FAIL+1))
fi

# 创建两个不删除，再删一个
R=$(post "/templates/create" "{\"name\":\"${P}cycle_a\",\"label\":\"cycleA\",\"description\":\"\",\"fields\":[{\"field_id\":${X_SPD},\"required\":true}]}")
CYC_A=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
R=$(post "/templates/create" "{\"name\":\"${P}cycle_b\",\"label\":\"cycleB\",\"description\":\"\",\"fields\":[{\"field_id\":${X_SPD},\"required\":true}]}")
CYC_B=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

TOTAL=$((TOTAL + 1))
SPD_RC=$(fld_refcount "$X_SPD")
[ "$SPD_RC" = "2" ] && { echo "  [PASS] x13.2 两模板后 ref_count=2"; PASS=$((PASS+1)); } || { echo "  [FAIL] x13.2 期望 2 实际 $SPD_RC"; FAIL=$((FAIL+1)); }

tpl_rm "$CYC_A"
TOTAL=$((TOTAL + 1))
SPD_RC=$(fld_refcount "$X_SPD")
[ "$SPD_RC" = "1" ] && { echo "  [PASS] x13.3 删一个后 ref_count=1"; PASS=$((PASS+1)); } || { echo "  [FAIL] x13.3 期望 1 实际 $SPD_RC"; FAIL=$((FAIL+1)); }

tpl_rm "$CYC_B"
TOTAL=$((TOTAL + 1))
SPD_RC=$(fld_refcount "$X_SPD")
[ "$SPD_RC" = "0" ] && { echo "  [PASS] x13.4 全删后 ref_count=0"; PASS=$((PASS+1)); } || { echo "  [FAIL] x13.4 期望 0 实际 $SPD_RC"; FAIL=$((FAIL+1)); }

# =============================================================================
# 3.14 ATTACK: Reference 字段 (CB) 通过模板挂载 -> 41012
# =============================================================================
subsection "3.14 ATTACK: Reference 字段通过模板挂载"

R=$(post "/templates/create" "{\"name\":\"${P}atk_ref_tpl\",\"label\":\"ref挂载\",\"description\":\"\",\"fields\":[{\"field_id\":${CB},\"required\":true}]}")
assert_code "x14.1 reference 字段挂模板 41012" "41012" "$R"

# 混合正常+reference 字段
R=$(post "/templates/create" "{\"name\":\"${P}atk_ref_mix\",\"label\":\"混合ref\",\"description\":\"\",\"fields\":[{\"field_id\":${X_HP},\"required\":true},{\"field_id\":${CB},\"required\":false}]}")
assert_code "x14.2 混合含 reference 41012" "41012" "$R"

# 确认 X_HP ref_count 未被污染
TOTAL=$((TOTAL + 1))
HP_RC=$(fld_refcount "$X_HP")
if [ "$HP_RC" = "1" ]; then
  echo "  [PASS] x14.3 失败创建未污染 X_HP ref_count"
  PASS=$((PASS+1))
else
  echo "  [FAIL] x14.3 X_HP ref_count 期望 1 实际 $HP_RC"
  FAIL=$((FAIL+1))
fi

echo ""
echo "  [INFO] test_03 完成"
