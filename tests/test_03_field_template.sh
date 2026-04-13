#!/bin/bash
# =============================================================================
# test_03_field_template.sh — 跨模块集成：字段 <-> 模板
#
# 前置：run_all.sh 已 source helpers.sh + test_01 + test_02
#       可用变量：F_HP F_ATK F_NAME F_DEF F_DISABLED 等
#       可用函数：post() tpl_* fld_* assert_*
# =============================================================================

section "Part 3: 跨模块集成 — 字段 x 模板 (prefix=$P)"

# =============================================================================
# 准备独立字段池（避免被前面测试的删除操作污染）
# =============================================================================
subsection "准备跨模块测试字段池"

R=$(post "/fields/create" "{\"name\":\"${P}x_hp\",\"label\":\"X_HP\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"HP\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}}}")
X_HP=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$X_HP"

R=$(post "/fields/create" "{\"name\":\"${P}x_atk\",\"label\":\"X_ATK\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"ATK\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":999}}}")
X_ATK=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$X_ATK"

R=$(post "/fields/create" "{\"name\":\"${P}x_def\",\"label\":\"X_DEF\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"DEF\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":500}}}")
X_DEF=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$X_DEF"

R=$(post "/fields/create" "{\"name\":\"${P}x_name\",\"label\":\"X_NAME\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{\"description\":\"name\",\"expose_bb\":false,\"constraints\":{\"minLength\":1,\"maxLength\":50}}}")
X_NAME=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$X_NAME"

# =============================================================================
# 1. 模板创建递增字段 ref_count
# =============================================================================
subsection "跨模块 1: 模板创建递增 ref_count"

# 创建前确认 ref_count=0
TOTAL=$((TOTAL + 2))
HP_RC=$(fld_refcount "$X_HP")
ATK_RC=$(fld_refcount "$X_ATK")
[ "$HP_RC" = "0" ] && { echo "  [PASS] x1.0a X_HP 初始 ref_count=0"; PASS=$((PASS+1)); } || { echo "  [FAIL] x1.0a 期望 0 实际 $HP_RC"; FAIL=$((FAIL+1)); }
[ "$ATK_RC" = "0" ] && { echo "  [PASS] x1.0b X_ATK 初始 ref_count=0"; PASS=$((PASS+1)); } || { echo "  [FAIL] x1.0b 期望 0 实际 $ATK_RC"; FAIL=$((FAIL+1)); }

R=$(post "/templates/create" "{\"name\":\"${P}x_tpl1\",\"label\":\"跨模块模板1\",\"description\":\"\",\"fields\":[{\"field_id\":${X_HP},\"required\":true},{\"field_id\":${X_ATK},\"required\":true}]}")
assert_code "x1.1 创建模板成功" "0" "$R"
X_TPL1=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

TOTAL=$((TOTAL + 2))
HP_RC=$(fld_refcount "$X_HP")
ATK_RC=$(fld_refcount "$X_ATK")
[ "$HP_RC" = "1" ] && { echo "  [PASS] x1.2a X_HP ref_count=1"; PASS=$((PASS+1)); } || { echo "  [FAIL] x1.2a 期望 1 实际 $HP_RC"; FAIL=$((FAIL+1)); }
[ "$ATK_RC" = "1" ] && { echo "  [PASS] x1.2b X_ATK ref_count=1"; PASS=$((PASS+1)); } || { echo "  [FAIL] x1.2b 期望 1 实际 $ATK_RC"; FAIL=$((FAIL+1)); }

# 第二个模板也引用 X_HP，ref_count 应为 2
R=$(post "/templates/create" "{\"name\":\"${P}x_tpl2\",\"label\":\"跨模块模板2\",\"description\":\"\",\"fields\":[{\"field_id\":${X_HP},\"required\":false},{\"field_id\":${X_DEF},\"required\":true}]}")
assert_code "x1.3 创建第二个模板成功" "0" "$R"
X_TPL2=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

TOTAL=$((TOTAL + 1))
HP_RC=$(fld_refcount "$X_HP")
[ "$HP_RC" = "2" ] && { echo "  [PASS] x1.4 X_HP ref_count=2（被两个模板引用）"; PASS=$((PASS+1)); } || { echo "  [FAIL] x1.4 期望 2 实际 $HP_RC"; FAIL=$((FAIL+1)); }

# =============================================================================
# 2. 模板删除递减字段 ref_count
# =============================================================================
subsection "跨模块 2: 模板删除递减 ref_count"

tpl_disable "$X_TPL2"
R=$(post "/templates/delete" "{\"id\":${X_TPL2}}")
assert_code "x2.1 删除模板2成功" "0" "$R"

TOTAL=$((TOTAL + 2))
HP_RC=$(fld_refcount "$X_HP")
DEF_RC=$(fld_refcount "$X_DEF")
[ "$HP_RC" = "1" ] && { echo "  [PASS] x2.2a X_HP ref_count=1（回退）"; PASS=$((PASS+1)); } || { echo "  [FAIL] x2.2a 期望 1 实际 $HP_RC"; FAIL=$((FAIL+1)); }
[ "$DEF_RC" = "0" ] && { echo "  [PASS] x2.2b X_DEF ref_count=0（回退）"; PASS=$((PASS+1)); } || { echo "  [FAIL] x2.2b 期望 0 实际 $DEF_RC"; FAIL=$((FAIL+1)); }

# =============================================================================
# 3. 模板 update（add/remove 字段）调整 ref_count
# =============================================================================
subsection "跨模块 3: 模板 update 调整 ref_count"

# 当前 X_TPL1 包含 [X_HP, X_ATK]
# 改为 [X_HP, X_DEF]（移除 X_ATK，新增 X_DEF）
V=$(tpl_version "$X_TPL1")
R=$(post "/templates/update" "{\"id\":${X_TPL1},\"label\":\"跨模块模板1\",\"description\":\"\",\"fields\":[{\"field_id\":${X_HP},\"required\":true},{\"field_id\":${X_DEF},\"required\":true}],\"version\":${V}}")
assert_code "x3.1 update 字段集合变化成功" "0" "$R"

TOTAL=$((TOTAL + 3))
HP_RC=$(fld_refcount "$X_HP")
ATK_RC=$(fld_refcount "$X_ATK")
DEF_RC=$(fld_refcount "$X_DEF")
[ "$HP_RC" = "1" ]  && { echo "  [PASS] x3.2a X_HP ref_count=1（保持）"; PASS=$((PASS+1)); }  || { echo "  [FAIL] x3.2a 期望 1 实际 $HP_RC"; FAIL=$((FAIL+1)); }
[ "$ATK_RC" = "0" ] && { echo "  [PASS] x3.2b X_ATK ref_count=0（被移除）"; PASS=$((PASS+1)); } || { echo "  [FAIL] x3.2b 期望 0 实际 $ATK_RC"; FAIL=$((FAIL+1)); }
[ "$DEF_RC" = "1" ] && { echo "  [PASS] x3.2c X_DEF ref_count=1（被新增）"; PASS=$((PASS+1)); } || { echo "  [FAIL] x3.2c 期望 1 实际 $DEF_RC"; FAIL=$((FAIL+1)); }

# 纯排序 / required 变化不应影响 ref_count
V=$(tpl_version "$X_TPL1")
R=$(post "/templates/update" "{\"id\":${X_TPL1},\"label\":\"跨模块模板1\",\"description\":\"\",\"fields\":[{\"field_id\":${X_DEF},\"required\":false},{\"field_id\":${X_HP},\"required\":false}],\"version\":${V}}")
assert_code "x3.3 纯排序+required 变化成功" "0" "$R"

TOTAL=$((TOTAL + 2))
HP_RC=$(fld_refcount "$X_HP")
DEF_RC=$(fld_refcount "$X_DEF")
[ "$HP_RC" = "1" ]  && { echo "  [PASS] x3.4a X_HP ref_count 不变"; PASS=$((PASS+1)); }  || { echo "  [FAIL] x3.4a 期望 1 实际 $HP_RC"; FAIL=$((FAIL+1)); }
[ "$DEF_RC" = "1" ] && { echo "  [PASS] x3.4b X_DEF ref_count 不变"; PASS=$((PASS+1)); } || { echo "  [FAIL] x3.4b 期望 1 实际 $DEF_RC"; FAIL=$((FAIL+1)); }

# =============================================================================
# 4. 删除被模板引用的字段 -> 应失败 (40005)
# =============================================================================
subsection "跨模块 4: 被模板引用的字段不可删除"

fld_disable "$X_HP"
R=$(post "/fields/delete" "{\"id\":${X_HP}}")
assert_code "x4.1 被模板引用字段删除 40005" "40005" "$R"

# 恢复启用状态
fld_enable "$X_HP"

# =============================================================================
# 5. 停用被模板引用的字段 -> 允许
# =============================================================================
subsection "跨模块 5: 停用被模板引用的字段（允许）"

fld_disable "$X_HP"
TOTAL=$((TOTAL + 1))
EN=$(fld_enabled "$X_HP")
if [ "$EN" = "false" ]; then
  echo "  [PASS] x5.1 允许停用被模板引用的字段"
  PASS=$((PASS+1))
else
  echo "  [FAIL] x5.1 应能停用 实际 $EN"
  FAIL=$((FAIL+1))
fi

# 模板详情中字段应显示 enabled=false
R=$(tpl_detail "$X_TPL1")
# X_HP 现在在 fields[1]（因为上面排序变了）
TOTAL=$((TOTAL + 1))
# 找到 X_HP 在 fields 中的 enabled 状态
HP_EN=$(echo "$R" | jq -r --arg id "$X_HP" '.data.fields[] | select(.field_id == ($id | tonumber)) | .enabled' | tr -d '\r')
if [ "$HP_EN" = "false" ]; then
  echo "  [PASS] x5.2 模板详情反映 X_HP.enabled=false"
  PASS=$((PASS+1))
else
  echo "  [FAIL] x5.2 期望 false 实际 $HP_EN"
  FAIL=$((FAIL+1))
fi

fld_enable "$X_HP"

# =============================================================================
# 6. 被模板引用的字段禁止修改类型 (40006)
# =============================================================================
subsection "跨模块 6: 被引用字段禁止改类型"

fld_disable "$X_HP"
V=$(fld_version "$X_HP")
R=$(post "/fields/update" "{\"id\":${X_HP},\"label\":\"X_HP\",\"type\":\"string\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"minLength\":0,\"maxLength\":100}},\"version\":${V}}")
assert_code "x6.1 被引用改 type 40006" "40006" "$R"

# 但纯 label / description 修改应允许
V=$(fld_version "$X_HP")
R=$(post "/fields/update" "{\"id\":${X_HP},\"label\":\"X_HP_Renamed\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"HP renamed\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}},\"version\":${V}}")
assert_code "x6.2 纯 label 修改允许" "0" "$R"

fld_enable "$X_HP"

# =============================================================================
# 7. 缓存一致性：模板操作后字段 detail 反映正确 ref_count
# =============================================================================
subsection "跨模块 7: 缓存一致性"

# 当前 X_TPL1 包含 [X_DEF, X_HP]
# 先读一次字段 detail（回填缓存）
R=$(fld_detail "$X_NAME")
assert_field "x7.0 X_NAME ref_count=0（初始）" ".data.ref_count" "0" "$R"

# 编辑模板加入 X_NAME
V=$(tpl_version "$X_TPL1")
R=$(post "/templates/update" "{\"id\":${X_TPL1},\"label\":\"跨模块模板1\",\"description\":\"\",\"fields\":[{\"field_id\":${X_DEF},\"required\":false},{\"field_id\":${X_HP},\"required\":false},{\"field_id\":${X_NAME},\"required\":true}],\"version\":${V}}")
assert_code "x7.1 模板加入 X_NAME 成功" "0" "$R"

# 立即读字段 detail，ref_count 应为 1（缓存已失效）
R=$(fld_detail "$X_NAME")
assert_field "x7.2 模板操作后 X_NAME ref_count=1" ".data.ref_count" "1" "$R"

# 再读一次确认缓存稳定
R=$(fld_detail "$X_NAME")
assert_field "x7.3 第二次读 ref_count 仍为 1" ".data.ref_count" "1" "$R"

# 字段引用详情应包含模板
R=$(post "/fields/references" "{\"id\":${X_NAME}}")
assert_code  "x7.4 字段引用详情成功" "0" "$R"
assert_ge    "x7.4 至少 1 个模板引用" ".data.templates | length" "1" "$R"

# 模板引用详情中有 template label
TOTAL=$((TOTAL + 1))
TPL_LABEL=$(echo "$R" | jq -r '.data.templates[0].label' | tr -d '\r')
if [ "$TPL_LABEL" = "跨模块模板1" ]; then
  echo "  [PASS] x7.5 template label 正确补全"
  PASS=$((PASS+1))
else
  echo "  [FAIL] x7.5 期望 '跨模块模板1' 实际 '$TPL_LABEL'"
  FAIL=$((FAIL+1))
fi

# =============================================================================
# 8. 大模板创建 + 删除 ref_count 全量联动
# =============================================================================
subsection "跨模块 8: 大模板（50字段）ref_count 联动"

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
assert_code "x8.1 50 字段模板创建成功" "0" "$R"
BIG_TPL=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

R=$(tpl_detail "$BIG_TPL")
assert_field "x8.2 fields 长度=50" ".data.fields | length" "50" "$R"

# 抽样验证 ref_count（检查第 1、25、50 个）
TOTAL=$((TOTAL + 3))
for idx in 0 24 49; do
  ID="${BIG_IDS[$idx]}"
  RC=$(fld_refcount "$ID")
  if [ "$RC" = "1" ]; then
    echo "  [PASS] x8.3 big_$((idx+1)) ref_count=1"
    PASS=$((PASS+1))
  else
    echo "  [FAIL] x8.3 big_$((idx+1)) 期望 1 实际 $RC"
    FAIL=$((FAIL+1))
  fi
done

# 删除大模板 -> 所有字段 ref_count 清零
tpl_disable "$BIG_TPL"
R=$(post "/templates/delete" "{\"id\":${BIG_TPL}}")
assert_code "x8.4 大模板删除成功" "0" "$R"

TOTAL=$((TOTAL + 3))
for idx in 0 24 49; do
  ID="${BIG_IDS[$idx]}"
  RC=$(fld_refcount "$ID")
  if [ "$RC" = "0" ]; then
    echo "  [PASS] x8.5 big_$((idx+1)) ref_count=0"
    PASS=$((PASS+1))
  else
    echo "  [FAIL] x8.5 big_$((idx+1)) 期望 0 实际 $RC"
    FAIL=$((FAIL+1))
  fi
done
