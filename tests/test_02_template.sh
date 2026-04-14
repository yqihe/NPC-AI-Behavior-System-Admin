#!/bin/bash
# =============================================================================
# test_02_template.sh — 模板管理 CRUD + 引用字段阻断 + 攻击性测试
#
# 前置：run_all.sh 已 source helpers.sh + test_01_field.sh
#       可用变量：HP_ID ATK_ID STR_ID FLAG_ID MOOD_ID FLOAT_ID CA CB
#       HP_ID/ATK_ID/STR_ID/CA/CB 已 enabled，FLAG_ID/MOOD_ID/FLOAT_ID 已 disabled
# 导出变量：TPL_ID F_HP F_ATK F_NAME F_DISABLED F_DEF
# =============================================================================

section "Part 2: 模板管理 (prefix=$P)"

# =============================================================================
# 2.0 准备模板用字段池
# =============================================================================
subsection "2.0 准备模板用字段池"

R=$(post "/fields/create" "{\"name\":\"${P}f_hp\",\"label\":\"T_HP\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"HP\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}}}")
F_HP=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$F_HP"
assert_code "t0.1 创建 f_hp" "0" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}f_atk\",\"label\":\"T_ATK\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"ATK\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":999}}}")
F_ATK=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$F_ATK"
assert_code "t0.2 创建 f_atk" "0" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}f_name\",\"label\":\"T_NAME\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{\"description\":\"name\",\"expose_bb\":false,\"constraints\":{\"minLength\":1,\"maxLength\":50}}}")
F_NAME=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$F_NAME"
assert_code "t0.3 创建 f_name" "0" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}f_disabled\",\"label\":\"T_DIS\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"dis\",\"expose_bb\":false}}")
F_DISABLED=$(echo "$R" | jq -r '.data.id' | tr -d '\r')   # 保持停用
assert_code "t0.4 创建 f_disabled" "0" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}f_def\",\"label\":\"T_DEF\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"DEF\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":500}}}")
F_DEF=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$F_DEF"
assert_code "t0.5 创建 f_def" "0" "$R"

# 确保 CB (reference) 可用
if [ -z "$CB" ] || [ "$CB" = "null" ]; then
  R=$(post "/fields/create" "{\"name\":\"${P}ref_leaf\",\"label\":\"ref_leaf\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false}}")
  REF_LEAF=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$REF_LEAF"
  R=$(post "/fields/create" "{\"name\":\"${P}ref_holder\",\"label\":\"ref_holder\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"refs\":[${REF_LEAF}]}}}")
  CB=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$CB"
fi

# =============================================================================
# 2.1 check-name
# =============================================================================
subsection "2.1 模板名唯一性校验"

R=$(post "/templates/check-name" "{\"name\":\"${P}npc_combat\"}")
assert_code  "t1.1 可用名" "0" "$R"
assert_field "t1.1 available=true" ".data.available" "true" "$R"

R=$(post "/templates/check-name" '{"name":""}')
assert_code "t1.2 空名 41002" "41002" "$R"

R=$(post "/templates/check-name" '{"name":"BAD"}')
assert_code "t1.3 大写 41002" "41002" "$R"

R=$(post "/templates/check-name" '{"name":"123abc"}')
assert_code "t1.4 数字开头 41002" "41002" "$R"

# soft-deleted name 测试放在 delete 之后

# =============================================================================
# 2.2 创建模板
# =============================================================================
subsection "2.2 创建模板"

R=$(post "/templates/create" "{\"name\":\"${P}npc_combat\",\"label\":\"战斗生物模板\",\"description\":\"战斗用\",\"fields\":[{\"field_id\":${F_HP},\"required\":true},{\"field_id\":${F_ATK},\"required\":true},{\"field_id\":${F_NAME},\"required\":false}]}")
assert_code "t2.1 创建成功" "0" "$R"
TPL_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
assert_not_equal "t2.1 id > 0" ".data.id" "null" "$R"

# 详情验证
R=$(tpl_detail "$TPL_ID")
assert_field "t2.2 enabled=false"          ".data.enabled"   "false" "$R"
assert_field "t2.2 version=1"              ".data.version"   "1"     "$R"
assert_field "t2.2 ref_count=0"            ".data.ref_count" "0"     "$R"
assert_field "t2.2 fields 数=3"            ".data.fields | length" "3" "$R"
assert_field "t2.2 fields[0] required"     ".data.fields[0].required" "true" "$R"
assert_field "t2.2 fields[2] required=false" ".data.fields[2].required" "false" "$R"
assert_exists "t2.2 category_label 存在"   ".data.fields[0].category_label" "$R"
assert_exists "t2.2 字段有 type"           ".data.fields[0].type"           "$R"
assert_exists "t2.2 字段有 enabled"        ".data.fields[0].enabled"        "$R"

# 验证字段 ref_count 已递增
TOTAL=$((TOTAL + 1))
HP_RC=$(fld_refcount "$F_HP")
if [ "$HP_RC" = "1" ]; then echo "  [PASS] t2.3 f_hp ref_count=1"; PASS=$((PASS+1)); else echo "  [FAIL] t2.3 期望 1 实际 $HP_RC"; FAIL=$((FAIL+1)); fi

# 重复名
R=$(post "/templates/create" "{\"name\":\"${P}npc_combat\",\"label\":\"重\",\"description\":\"\",\"fields\":[{\"field_id\":${F_HP},\"required\":true}]}")
assert_code "t2.4 重复 name 41001" "41001" "$R"

# 非法 name
R=$(post "/templates/create" "{\"name\":\"BAD-name\",\"label\":\"坏\",\"description\":\"\",\"fields\":[{\"field_id\":${F_HP},\"required\":true}]}")
assert_code "t2.5 非法 name 41002" "41002" "$R"

# 空字段
R=$(post "/templates/create" "{\"name\":\"${P}empty_f\",\"label\":\"x\",\"description\":\"\",\"fields\":[]}")
assert_code "t2.6 空 fields 41004" "41004" "$R"

# 不存在字段
R=$(post "/templates/create" "{\"name\":\"${P}nofield\",\"label\":\"x\",\"description\":\"\",\"fields\":[{\"field_id\":999999,\"required\":true}]}")
assert_code "t2.7 字段不存在 41006" "41006" "$R"

# disabled 字段
R=$(post "/templates/create" "{\"name\":\"${P}dis_f\",\"label\":\"x\",\"description\":\"\",\"fields\":[{\"field_id\":${F_DISABLED},\"required\":true}]}")
assert_code "t2.8 disabled 字段 41005" "41005" "$R"

# 重复 field_id
R=$(post "/templates/create" "{\"name\":\"${P}dup_f\",\"label\":\"x\",\"description\":\"\",\"fields\":[{\"field_id\":${F_HP},\"required\":true},{\"field_id\":${F_HP},\"required\":false}]}")
assert_code "t2.9 重复 field_id 40000" "40000" "$R"

# 空 label
R=$(post "/templates/create" "{\"name\":\"${P}no_label\",\"label\":\"\",\"description\":\"\",\"fields\":[{\"field_id\":${F_HP},\"required\":true}]}")
assert_code "t2.10 空 label 40000" "40000" "$R"

# description 超长 (>512)
LONG_DESC=$(printf 'a%.0s' $(seq 1 513))
R=$(post "/templates/create" "{\"name\":\"${P}longdesc\",\"label\":\"x\",\"description\":\"${LONG_DESC}\",\"fields\":[{\"field_id\":${F_HP},\"required\":true}]}")
assert_code "t2.11 description>512 40000" "40000" "$R"

# description = 512 OK
DESC_512=$(printf 'a%.0s' $(seq 1 512))
R=$(post "/templates/create" "{\"name\":\"${P}okdesc\",\"label\":\"x\",\"description\":\"${DESC_512}\",\"fields\":[{\"field_id\":${F_HP},\"required\":true}]}")
assert_code "t2.12 description=512 OK" "0" "$R"
DESC_OK_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
[ -n "$DESC_OK_ID" ] && [ "$DESC_OK_ID" != "null" ] && tpl_rm "$DESC_OK_ID" 2>/dev/null

# =============================================================================
# 2.3 Reference 类型字段阻断 (41012)
# =============================================================================
subsection "2.3 Reference 字段阻断"

R=$(post "/templates/create" "{\"name\":\"${P}ref_tpl\",\"label\":\"含引用\",\"description\":\"\",\"fields\":[{\"field_id\":${CB},\"required\":true}]}")
assert_code "t3.1 创建含 reference 字段 41012" "41012" "$R"

# ref_count 不应被污染
TOTAL=$((TOTAL + 1))
CB_RC=$(fld_refcount "$CB")
if [ "$CB_RC" = "0" ]; then echo "  [PASS] t3.1b ref_count 未污染"; PASS=$((PASS+1)); else echo "  [FAIL] t3.1b ref_count=$CB_RC（应 0）"; FAIL=$((FAIL+1)); fi

# 编辑路径也应拒绝
R=$(post "/fields/create" "{\"name\":\"${P}ref_leaf3\",\"label\":\"叶3\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}}}")
LEAF3=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$LEAF3"
R=$(post "/templates/create" "{\"name\":\"${P}ref_edit_tpl\",\"label\":\"编辑引用\",\"description\":\"\",\"fields\":[{\"field_id\":${LEAF3},\"required\":true}]}")
REF_EDIT_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
V=$(tpl_version "$REF_EDIT_ID")
R=$(post "/templates/update" "{\"id\":${REF_EDIT_ID},\"label\":\"编辑引用\",\"description\":\"\",\"fields\":[{\"field_id\":${LEAF3},\"required\":true},{\"field_id\":${CB},\"required\":false}],\"version\":${V}}")
assert_code "t3.2 编辑时加 reference 字段 41012" "41012" "$R"
tpl_rm "$REF_EDIT_ID"

# =============================================================================
# 2.4 详情
# =============================================================================
subsection "2.4 模板详情"

R=$(tpl_detail "$TPL_ID")
assert_code  "t4.1 详情成功"     "0" "$R"
assert_field "t4.1 name"         ".data.name" "${P}npc_combat" "$R"
assert_field "t4.1 label"        ".data.label" "战斗生物模板" "$R"
assert_field "t4.1 description"  ".data.description" "战斗用" "$R"
assert_exists "t4.1 enriched type" ".data.fields[0].type" "$R"

R=$(tpl_detail 999999)
assert_code "t4.2 不存在 41003" "41003" "$R"

R=$(post "/templates/detail" '{"id":0}')
assert_code "t4.3 ID=0 40000" "40000" "$R"

# =============================================================================
# 2.5 列表
# =============================================================================
subsection "2.5 模板列表"

R=$(post "/templates/list" '{"page":1,"page_size":20}')
assert_code "t5.1 列表成功" "0" "$R"
assert_ge   "t5.1 total>=1" ".data.total" "1" "$R"

# label 搜索
R=$(post "/templates/list" '{"label":"战斗生物","page":1,"page_size":20}')
assert_ge "t5.2 label 搜索 >=1" ".data.total" "1" "$R"

# enabled 筛选
R=$(post "/templates/list" '{"enabled":true,"page":1,"page_size":20}')
assert_code  "t5.3 enabled=true" "0" "$R"
assert_field "t5.3 total=0 (全部 disabled)" ".data.total" "0" "$R"

# 页面纠正
R=$(post "/templates/list" '{"page":0,"page_size":0}')
assert_field "t5.4 page 校正" ".data.page" "1" "$R"

# 空结果
R=$(post "/templates/list" '{"label":"zzz_nonexistent_xyz","page":1,"page_size":20}')
assert_code  "t5.5 空结果" "0" "$R"
assert_field "t5.5 total=0" ".data.total" "0" "$R"

# 列表项无 fields/description
R=$(post "/templates/list" '{"page":1,"page_size":20}')
assert_field "t5.6 列表项无 fields" ".data.items[0].fields" "null" "$R"

# =============================================================================
# 2.6 Toggle 启用/禁用
# =============================================================================
subsection "2.6 Toggle 启停"

tpl_disable "$TPL_ID"
R=$(tpl_detail "$TPL_ID")
assert_field "t6.1 已停用" ".data.enabled" "false" "$R"

tpl_enable "$TPL_ID"
R=$(tpl_detail "$TPL_ID")
assert_field "t6.2 重新启用" ".data.enabled" "true" "$R"

# version conflict
R=$(post "/templates/toggle-enabled" "{\"id\":${TPL_ID},\"enabled\":false,\"version\":999}")
assert_code "t6.3 版本冲突 41011" "41011" "$R"

# not found
R=$(post "/templates/toggle-enabled" '{"id":999999,"enabled":true,"version":1}')
assert_code "t6.4 不存在 41003" "41003" "$R"

# id=0
R=$(post "/templates/toggle-enabled" '{"id":0,"enabled":true,"version":1}')
assert_code "t6.5 ID=0 40000" "40000" "$R"

# =============================================================================
# 2.7 编辑模板
# =============================================================================
subsection "2.7 编辑模板"

# 启用中编辑 -> 41010
V=$(tpl_version "$TPL_ID")
R=$(post "/templates/update" "{\"id\":${TPL_ID},\"label\":\"x\",\"description\":\"\",\"fields\":[{\"field_id\":${F_HP},\"required\":true}],\"version\":${V}}")
assert_code "t7.1 启用中编辑 41010" "41010" "$R"

tpl_disable "$TPL_ID"

# label + description 变更
V=$(tpl_version "$TPL_ID")
R=$(post "/templates/update" "{\"id\":${TPL_ID},\"label\":\"战斗生物模板（改）\",\"description\":\"改后\",\"fields\":[{\"field_id\":${F_HP},\"required\":true},{\"field_id\":${F_ATK},\"required\":true},{\"field_id\":${F_NAME},\"required\":false}],\"version\":${V}}")
assert_code "t7.2 label+desc 变更成功" "0" "$R"
R=$(tpl_detail "$TPL_ID")
assert_field "t7.2 label 已变" ".data.label" "战斗生物模板（改）" "$R"
assert_field "t7.2 desc 已变"  ".data.description" "改后" "$R"

# 纯顺序变化
V=$(tpl_version "$TPL_ID")
R=$(post "/templates/update" "{\"id\":${TPL_ID},\"label\":\"战斗生物模板（改）\",\"description\":\"改后\",\"fields\":[{\"field_id\":${F_NAME},\"required\":false},{\"field_id\":${F_HP},\"required\":true},{\"field_id\":${F_ATK},\"required\":true}],\"version\":${V}}")
assert_code "t7.3 顺序变化成功" "0" "$R"
R=$(tpl_detail "$TPL_ID")
assert_field "t7.3 fields[0]=f_name" ".data.fields[0].name" "${P}f_name" "$R"

# 字段集合变更：加 F_DEF
V=$(tpl_version "$TPL_ID")
R=$(post "/templates/update" "{\"id\":${TPL_ID},\"label\":\"战斗生物模板（改）\",\"description\":\"\",\"fields\":[{\"field_id\":${F_HP},\"required\":true},{\"field_id\":${F_DEF},\"required\":true}],\"version\":${V}}")
assert_code "t7.4 字段集合变更成功" "0" "$R"
R=$(tpl_detail "$TPL_ID")
assert_field "t7.4 fields 数=2" ".data.fields | length" "2" "$R"

# disabled 字段拒绝
V=$(tpl_version "$TPL_ID")
R=$(post "/templates/update" "{\"id\":${TPL_ID},\"label\":\"x\",\"description\":\"\",\"fields\":[{\"field_id\":${F_HP},\"required\":true},{\"field_id\":${F_DISABLED},\"required\":false}],\"version\":${V}}")
assert_code "t7.5 disabled 字段 41005" "41005" "$R"

# 不存在字段
V=$(tpl_version "$TPL_ID")
R=$(post "/templates/update" "{\"id\":${TPL_ID},\"label\":\"x\",\"description\":\"\",\"fields\":[{\"field_id\":${F_HP},\"required\":true},{\"field_id\":999999,\"required\":false}],\"version\":${V}}")
assert_code "t7.6 不存在字段 41006" "41006" "$R"

# version conflict
R=$(post "/templates/update" "{\"id\":${TPL_ID},\"label\":\"x\",\"description\":\"\",\"fields\":[{\"field_id\":${F_HP},\"required\":true}],\"version\":999}")
assert_code "t7.7 version conflict 41011" "41011" "$R"

# 不存在 id
R=$(post "/templates/update" '{"id":999999,"label":"x","description":"","fields":[{"field_id":1,"required":true}],"version":1}')
assert_code "t7.8 不存在 id 41003" "41003" "$R"

# 空 fields
V=$(tpl_version "$TPL_ID")
R=$(post "/templates/update" "{\"id\":${TPL_ID},\"label\":\"x\",\"description\":\"\",\"fields\":[],\"version\":${V}}")
assert_code "t7.9 空 fields 41004" "41004" "$R"

# =============================================================================
# 2.8 References
# =============================================================================
subsection "2.8 模板引用 (references)"

R=$(post "/templates/references" "{\"id\":${TPL_ID}}")
assert_code  "t8.1 references 成功" "0" "$R"
assert_field "t8.1 npcs 空数组" ".data.npcs | length" "0" "$R"
assert_field "t8.1 npcs 类型" ".data.npcs | type" "array" "$R"

R=$(post "/templates/references" '{"id":999999}')
assert_code "t8.2 不存在 41003" "41003" "$R"

# =============================================================================
# 2.9 删除
# =============================================================================
subsection "2.9 删除模板"

# enabled 时删除
tpl_enable "$TPL_ID"
R=$(post "/templates/delete" "{\"id\":${TPL_ID}}")
assert_code "t9.1 enabled 时删除 41009" "41009" "$R"

# disabled 后删除
tpl_disable "$TPL_ID"
R=$(post "/templates/delete" "{\"id\":${TPL_ID}}")
assert_code "t9.2 停用后删除成功" "0" "$R"
assert_field "t9.2 返回 id" ".data.id" "$TPL_ID" "$R"

# 软删除名不可复用
R=$(post "/templates/check-name" "{\"name\":\"${P}npc_combat\"}")
assert_field "t9.3 软删除名不可用" ".data.available" "false" "$R"

# deleted -> detail 404
R=$(tpl_detail "$TPL_ID")
assert_code "t9.4 deleted detail 41003" "41003" "$R"

# deleted -> toggle 404
R=$(post "/templates/toggle-enabled" "{\"id\":${TPL_ID},\"enabled\":true,\"version\":1}")
assert_code "t9.5 deleted toggle 41003" "41003" "$R"

# deleted -> references 404
R=$(post "/templates/references" "{\"id\":${TPL_ID}}")
assert_code "t9.6 deleted references 41003" "41003" "$R"

# deleted -> delete again
R=$(post "/templates/delete" "{\"id\":${TPL_ID}}")
assert_code "t9.7 重复删除 41003" "41003" "$R"

# delete id=0
R=$(post "/templates/delete" '{"id":0}')
assert_code "t9.8 ID=0 40000" "40000" "$R"

# =============================================================================
# 2.10 重建主模板供后续测试使用
# =============================================================================
subsection "2.10 重建主模板"

R=$(post "/templates/create" "{\"name\":\"${P}npc_combat_v2\",\"label\":\"战斗生物模板V2\",\"description\":\"后续测试用\",\"fields\":[{\"field_id\":${F_HP},\"required\":true},{\"field_id\":${F_ATK},\"required\":true},{\"field_id\":${F_NAME},\"required\":false}]}")
assert_code "t10.1 重建成功" "0" "$R"
TPL_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

# =============================================================================
# 2.11 ATTACK 攻击性测试
# =============================================================================
subsection "2.11 攻击性测试"

# 特殊字符 name
R=$(post "/templates/create" '{"name":"a]\"injection","label":"xss","description":"","fields":[]}')
assert_not_500 "t11.1 特殊字符 name" "$R"

# 超长 name (>64)
LONG_TPL=$(printf 'a%.0s' $(seq 1 100))
R=$(post "/templates/create" "{\"name\":\"${LONG_TPL}\",\"label\":\"x\",\"description\":\"\",\"fields\":[{\"field_id\":${F_ATK},\"required\":true}]}")
assert_not_500 "t11.2 超长 name(>64)" "$R"
assert_code    "t11.2 应为 41002" "41002" "$R"

# 畸形 JSON
R=$(raw_post "/templates/create" "{bad json}")
assert_not_500 "t11.3 畸形 JSON" "$R"

# 空 body
R=$(raw_post "/templates/create" "")
assert_not_500 "t11.4 空 body" "$R"

# field_id=0
R=$(post "/templates/create" "{\"name\":\"${P}atk_fz\",\"label\":\"x\",\"description\":\"\",\"fields\":[{\"field_id\":0,\"required\":true}]}")
assert_not_500 "t11.5 field_id=0" "$R"
assert_code    "t11.5 field_id=0 40000" "40000" "$R"

# field_id=-1
R=$(post "/templates/create" "{\"name\":\"${P}atk_fn\",\"label\":\"x\",\"description\":\"\",\"fields\":[{\"field_id\":-1,\"required\":true}]}")
assert_not_500 "t11.6 field_id=-1" "$R"

# field_id=999999999
R=$(post "/templates/create" "{\"name\":\"${P}atk_fb\",\"label\":\"x\",\"description\":\"\",\"fields\":[{\"field_id\":999999999,\"required\":true}]}")
assert_not_500 "t11.7 field_id=999999999" "$R"
assert_code    "t11.7 应为 41006" "41006" "$R"

echo ""
echo "  [INFO] test_02 完成, TPL_ID=$TPL_ID, F_HP=$F_HP, F_ATK=$F_ATK, F_NAME=$F_NAME, F_DEF=$F_DEF"
