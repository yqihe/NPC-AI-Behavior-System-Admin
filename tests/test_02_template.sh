#!/bin/bash
# =============================================================================
# test_02_template.sh — 模板管理 CRUD + 攻击性测试
#
# 前置：run_all.sh 已 source helpers.sh + test_01_field.sh
#       可用变量：HP_ID ATK_ID MOOD_ID FLOAT_ID CA CB 等
#       可用函数：post() tpl_* fld_* assert_*
# 导出变量：TPL_ID F_HP F_ATK F_NAME F_DISABLED F_DEF 供 test_03 使用
# =============================================================================

section "Part 2: 模板管理 (prefix=$P)"

# =============================================================================
# 准备模板用字段池（独立于 test_01 的字段）
# =============================================================================
subsection "准备模板用字段池"

R=$(post "/fields/create" "{\"name\":\"${P}f_hp\",\"label\":\"T_HP\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"HP\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}}}")
F_HP=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$F_HP"

R=$(post "/fields/create" "{\"name\":\"${P}f_atk\",\"label\":\"T_ATK\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"ATK\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":999}}}")
F_ATK=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$F_ATK"

R=$(post "/fields/create" "{\"name\":\"${P}f_name\",\"label\":\"T_NAME\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{\"description\":\"name\",\"expose_bb\":false,\"constraints\":{\"minLength\":1,\"maxLength\":50}}}")
F_NAME=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$F_NAME"

R=$(post "/fields/create" "{\"name\":\"${P}f_disabled\",\"label\":\"T_DIS\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"dis\",\"expose_bb\":false}}")
F_DISABLED=$(echo "$R" | jq -r '.data.id' | tr -d '\r')   # 保持停用

# =============================================================================
# 功能 10：唯一性校验 (check-name)
# =============================================================================
subsection "模板 功能 10: 名唯一性校验"

R=$(post "/templates/check-name" "{\"name\":\"${P}npc_combat\"}")
assert_field "t10.1 未用 available=true" ".data.available" "true" "$R"

R=$(post "/templates/check-name" '{"name":""}')
assert_code "t10.2 空名 41002" "41002" "$R"

R=$(post "/templates/check-name" '{"name":"BAD"}')
assert_code "t10.3 大写 41002" "41002" "$R"

R=$(post "/templates/check-name" '{"name":"123abc"}')
assert_code "t10.4 数字开头 41002" "41002" "$R"

# =============================================================================
# 功能 2：新建模板
# =============================================================================
subsection "模板 功能 2: 新建"

R=$(post "/templates/create" "{\"name\":\"${P}npc_combat\",\"label\":\"战斗生物模板\",\"description\":\"战斗用\",\"fields\":[{\"field_id\":${F_HP},\"required\":true},{\"field_id\":${F_ATK},\"required\":true},{\"field_id\":${F_NAME},\"required\":false}]}")
assert_code "t2.1 创建成功" "0" "$R"
TPL_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

R=$(tpl_detail "$TPL_ID")
assert_field "t2.2 enabled=false"                ".data.enabled" "false" "$R"
assert_field "t2.2 version=1"                    ".data.version" "1" "$R"
assert_field "t2.2 ref_count=0"                  ".data.ref_count" "0" "$R"
assert_field "t2.2 fields 数 3"                  ".data.fields | length" "3" "$R"
assert_field "t2.2 fields[0]=f_hp"               ".data.fields[0].name" "${P}f_hp" "$R"
assert_field "t2.2 fields[0] required=true"      ".data.fields[0].required" "true" "$R"
assert_field "t2.2 fields[2] required=false"     ".data.fields[2].required" "false" "$R"
assert_field "t2.2 category_label 已翻译"        ".data.fields[0].category_label" "战斗属性" "$R"
assert_field "t2.2 enabled 回传"                 ".data.fields[0].enabled" "true" "$R"

# 字段方 ref_count 已同步
TOTAL=$((TOTAL + 1))
HP_RC=$(fld_refcount "$F_HP")
if [ "$HP_RC" = "1" ]; then echo "  [PASS] t2.3 f_hp.ref_count=1"; PASS=$((PASS+1)); else echo "  [FAIL] t2.3 期望 1 实际 $HP_RC"; FAIL=$((FAIL+1)); fi

# ---- 异常场景 ----
R=$(post "/templates/create" "{\"name\":\"${P}npc_combat\",\"label\":\"重\",\"description\":\"\",\"fields\":[{\"field_id\":${F_HP},\"required\":true}]}")
assert_code "t2.4 重复 name 41001" "41001" "$R"

R=$(post "/templates/create" "{\"name\":\"BAD\",\"label\":\"x\",\"description\":\"\",\"fields\":[{\"field_id\":${F_HP},\"required\":true}]}")
assert_code "t2.5 非法 name 41002" "41002" "$R"

R=$(post "/templates/create" "{\"name\":\"${P}empty\",\"label\":\"x\",\"description\":\"\",\"fields\":[]}")
assert_code "t2.6 空 fields 41004" "41004" "$R"

R=$(post "/templates/create" "{\"name\":\"${P}n_exist\",\"label\":\"x\",\"description\":\"\",\"fields\":[{\"field_id\":999999,\"required\":true}]}")
assert_code "t2.7 不存在字段 41006" "41006" "$R"

R=$(post "/templates/create" "{\"name\":\"${P}n_disabled\",\"label\":\"x\",\"description\":\"\",\"fields\":[{\"field_id\":${F_DISABLED},\"required\":true}]}")
assert_code "t2.8 停用字段 41005" "41005" "$R"

R=$(post "/templates/create" "{\"name\":\"${P}n_dup\",\"label\":\"x\",\"description\":\"\",\"fields\":[{\"field_id\":${F_HP},\"required\":true},{\"field_id\":${F_HP},\"required\":false}]}")
assert_code "t2.9 重复 field_id 40000" "40000" "$R"

R=$(post "/templates/create" "{\"name\":\"${P}n_nolabel\",\"label\":\"\",\"description\":\"\",\"fields\":[{\"field_id\":${F_HP},\"required\":true}]}")
assert_code "t2.10 空 label 40000" "40000" "$R"

# description 超长
LONG_DESC=$(printf 'a%.0s' $(seq 1 513))
R=$(post "/templates/create" "{\"name\":\"${P}n_desc\",\"label\":\"x\",\"description\":\"${LONG_DESC}\",\"fields\":[{\"field_id\":${F_HP},\"required\":true}]}")
assert_code "t2.11 description 513 字 40000" "40000" "$R"

# 512 字刚好允许
LONG_OK=$(printf 'a%.0s' $(seq 1 512))
R=$(post "/templates/create" "{\"name\":\"${P}n_desc_ok\",\"label\":\"x\",\"description\":\"${LONG_OK}\",\"fields\":[{\"field_id\":${F_HP},\"required\":true}]}")
assert_code "t2.12 description 512 字 ok" "0" "$R"
DESC_OK_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
if [ -n "$DESC_OK_ID" ] && [ "$DESC_OK_ID" != "null" ]; then
  tpl_rm "$DESC_OK_ID" 2>/dev/null
fi

# ---- reference 类型字段禁止挂模板 (41012) ----
subsection "ATK: 模板挂载 reference 类型字段"

# 用 test_01 中的 CB（reference 类型，已启用）
# 如果 CB 不可用，先创建一个
if [ -z "$CB" ] || [ "$CB" = "null" ]; then
  R=$(post "/fields/create" "{\"name\":\"${P}ref_leaf\",\"label\":\"ref_leaf\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false}}")
  REF_LEAF=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$REF_LEAF"
  R=$(post "/fields/create" "{\"name\":\"${P}ref_holder\",\"label\":\"ref_holder\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"refs\":[${REF_LEAF}]}}}")
  CB=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$CB"
fi

R=$(post "/templates/create" "{\"name\":\"${P}atk_tpl_ref\",\"label\":\"含 reference 的模板\",\"description\":\"\",\"fields\":[{\"field_id\":${CB},\"required\":true}]}")
assert_code "atk_tpl.1 模板挂 reference 被拒绝 41012" "41012" "$R"

# 确认 ref_count 未被污染
TOTAL=$((TOTAL + 1))
CB_RC=$(fld_refcount "$CB")
if [ "$CB_RC" = "0" ]; then
  echo "  [PASS] atk_tpl.2 ref_count 保持 0（未被污染）"
  PASS=$((PASS+1))
else
  echo "  [FAIL] atk_tpl.2 ref_count=$CB_RC（应为 0）"
  FAIL=$((FAIL+1))
fi

# 编辑路径也应拒绝
R=$(post "/fields/create" "{\"name\":\"${P}atk_leaf3\",\"label\":\"攻击叶3\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}}}")
LEAF3=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$LEAF3"

R=$(post "/templates/create" "{\"name\":\"${P}atk_tpl_ref2\",\"label\":\"模板\",\"description\":\"\",\"fields\":[{\"field_id\":${LEAF3},\"required\":true}]}")
TPL_REF2=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
V=$(tpl_version "$TPL_REF2")
R=$(post "/templates/update" "{\"id\":${TPL_REF2},\"label\":\"模板\",\"description\":\"\",\"fields\":[{\"field_id\":${LEAF3},\"required\":true},{\"field_id\":${CB},\"required\":false}],\"version\":${V}}")
assert_code "atk_tpl.3 编辑时加入 reference 字段 41012" "41012" "$R"
tpl_rm "$TPL_REF2"

# =============================================================================
# 功能 3：模板详情（字段富化）
# =============================================================================
subsection "模板 功能 3: 详情"

R=$(tpl_detail "$TPL_ID")
assert_code  "t3.1 详情成功" "0" "$R"
assert_field "t3.1 name" ".data.name" "${P}npc_combat" "$R"
assert_field "t3.1 label" ".data.label" "战斗生物模板" "$R"
assert_field "t3.1 description" ".data.description" "战斗用" "$R"
assert_field "t3.1 fields enriched name" ".data.fields[0].name" "${P}f_hp" "$R"

R=$(tpl_detail 999999)
assert_code "t3.2 不存在 41003" "41003" "$R"

R=$(post "/templates/detail" '{"id":0}')
assert_code "t3.3 ID=0 40000" "40000" "$R"

# =============================================================================
# 功能 1：模板列表
# =============================================================================
subsection "模板 功能 1: 列表"

R=$(post "/templates/list" '{"page":1,"page_size":20}')
assert_code  "t1.1 列表成功" "0" "$R"
assert_ge    "t1.1 total >= 1" ".data.total" "1" "$R"

R=$(post "/templates/list" '{"label":"战斗生物","page":1,"page_size":20}')
assert_ge "t1.2 模糊搜索 >= 1" ".data.total" "1" "$R"

R=$(post "/templates/list" '{"enabled":true,"page":1,"page_size":20}')
assert_code "t1.3 enabled=true 查询" "0" "$R"

tpl_enable "$TPL_ID"
R=$(post "/templates/list" '{"enabled":true,"page":1,"page_size":20}')
assert_ge "t1.4 启用后 >= 1" ".data.total" "1" "$R"

R=$(post "/templates/list" '{"page":0,"page_size":0}')
assert_field "t1.5 page 校正 1" ".data.page" "1" "$R"

R=$(post "/templates/list" '{"label":"不存在zzz","page":1,"page_size":20}')
assert_field "t1.6 空结果" ".data.items | length" "0" "$R"

# 列表项不应含 fields / description
R=$(post "/templates/list" '{"page":1,"page_size":20}')
assert_field "t1.7 列表项无 fields 字段" ".data.items[0].fields" "null" "$R"

# =============================================================================
# 功能 7：启停切换
# =============================================================================
subsection "模板 功能 7: 启停切换"

tpl_disable "$TPL_ID"
R=$(tpl_detail "$TPL_ID")
assert_field "t7.1 已停用" ".data.enabled" "false" "$R"

tpl_enable "$TPL_ID"
R=$(tpl_detail "$TPL_ID")
assert_field "t7.2 重新启用" ".data.enabled" "true" "$R"

R=$(post "/templates/toggle-enabled" "{\"id\":${TPL_ID},\"enabled\":false,\"version\":999}")
assert_code "t7.3 版本冲突 41011" "41011" "$R"

R=$(post "/templates/toggle-enabled" '{"id":999999,"enabled":true,"version":1}')
assert_code "t7.4 不存在 ID 41003" "41003" "$R"

R=$(post "/templates/toggle-enabled" '{"id":0,"enabled":true,"version":1}')
assert_code "t7.5 ID=0 40000" "40000" "$R"

# =============================================================================
# 功能 4：编辑模板
# =============================================================================
subsection "模板 功能 4: 编辑"

# 启用中编辑 -> 41010
V=$(tpl_version "$TPL_ID")
R=$(post "/templates/update" "{\"id\":${TPL_ID},\"label\":\"x\",\"description\":\"\",\"fields\":[{\"field_id\":${F_HP},\"required\":true}],\"version\":${V}}")
assert_code "t4.1 启用中编辑 41010" "41010" "$R"

tpl_disable "$TPL_ID"
V=$(tpl_version "$TPL_ID")

# 纯 label/description 修改
R=$(post "/templates/update" "{\"id\":${TPL_ID},\"label\":\"战斗生物模板（改）\",\"description\":\"改后\",\"fields\":[{\"field_id\":${F_HP},\"required\":true},{\"field_id\":${F_ATK},\"required\":true},{\"field_id\":${F_NAME},\"required\":false}],\"version\":${V}}")
assert_code "t4.2 label/desc 改动成功" "0" "$R"

R=$(tpl_detail "$TPL_ID")
assert_field "t4.2 label 更新" ".data.label" "战斗生物模板（改）" "$R"
assert_field "t4.2 desc 更新" ".data.description" "改后" "$R"

# 纯字段顺序变化
V=$(tpl_version "$TPL_ID")
R=$(post "/templates/update" "{\"id\":${TPL_ID},\"label\":\"战斗生物模板（改）\",\"description\":\"改后\",\"fields\":[{\"field_id\":${F_NAME},\"required\":false},{\"field_id\":${F_HP},\"required\":true},{\"field_id\":${F_ATK},\"required\":true}],\"version\":${V}}")
assert_code "t4.3 顺序变化成功" "0" "$R"

R=$(tpl_detail "$TPL_ID")
assert_field "t4.3 fields[0]=f_name" ".data.fields[0].name" "${P}f_name" "$R"
assert_field "t4.3 fields[1]=f_hp"    ".data.fields[1].name" "${P}f_hp" "$R"

# 集合变化：加新字段 + 移除旧字段
R=$(post "/fields/create" "{\"name\":\"${P}f_def\",\"label\":\"T_DEF\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"DEF\",\"expose_bb\":false}}")
F_DEF=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$F_DEF"

V=$(tpl_version "$TPL_ID")
R=$(post "/templates/update" "{\"id\":${TPL_ID},\"label\":\"战斗生物模板（改）\",\"description\":\"\",\"fields\":[{\"field_id\":${F_HP},\"required\":true},{\"field_id\":${F_DEF},\"required\":true}],\"version\":${V}}")
assert_code "t4.4 集合变化成功" "0" "$R"

# 加入停用字段
V=$(tpl_version "$TPL_ID")
R=$(post "/templates/update" "{\"id\":${TPL_ID},\"label\":\"战斗生物模板（改）\",\"description\":\"\",\"fields\":[{\"field_id\":${F_HP},\"required\":true},{\"field_id\":${F_DEF},\"required\":true},{\"field_id\":${F_DISABLED},\"required\":false}],\"version\":${V}}")
assert_code "t4.6 加入停用字段 41005" "41005" "$R"

# 加入不存在字段
V=$(tpl_version "$TPL_ID")
R=$(post "/templates/update" "{\"id\":${TPL_ID},\"label\":\"战斗生物模板（改）\",\"description\":\"\",\"fields\":[{\"field_id\":${F_HP},\"required\":true},{\"field_id\":999999,\"required\":false}],\"version\":${V}}")
assert_code "t4.7 加入不存在字段 41006" "41006" "$R"

# 乐观锁
R=$(post "/templates/update" "{\"id\":${TPL_ID},\"label\":\"x\",\"description\":\"\",\"fields\":[{\"field_id\":${F_HP},\"required\":true}],\"version\":999}")
assert_code "t4.8 version 冲突 41011" "41011" "$R"

R=$(post "/templates/update" '{"id":999999,"label":"x","description":"","fields":[{"field_id":1,"required":true}],"version":1}')
assert_code "t4.9 不存在 ID 41003" "41003" "$R"

V=$(tpl_version "$TPL_ID")
R=$(post "/templates/update" "{\"id\":${TPL_ID},\"label\":\"x\",\"description\":\"\",\"fields\":[],\"version\":${V}}")
assert_code "t4.10 空 fields 41004" "41004" "$R"

# =============================================================================
# 功能 6：模板引用详情
# =============================================================================
subsection "模板 功能 6: 引用详情"

R=$(post "/templates/references" "{\"id\":${TPL_ID}}")
assert_code  "t6.1 成功" "0" "$R"
assert_field "t6.1 template_id" ".data.template_id" "$TPL_ID" "$R"
assert_field "t6.1 npcs 空（NPC 未上线）" ".data.npcs | length" "0" "$R"
assert_field "t6.1 npcs 是数组（非 null）" ".data.npcs | type" "array" "$R"

R=$(post "/templates/references" '{"id":999999}')
assert_code "t6.2 不存在 41003" "41003" "$R"

# =============================================================================
# 功能 5：删除模板
# =============================================================================
subsection "模板 功能 5: 删除"

tpl_enable "$TPL_ID"
R=$(post "/templates/delete" "{\"id\":${TPL_ID}}")
assert_code "t5.1 启用中删除 41009" "41009" "$R"

tpl_disable "$TPL_ID"
R=$(post "/templates/delete" "{\"id\":${TPL_ID}}")
assert_code "t5.2 停用后删除成功" "0" "$R"
assert_field "t5.2 返回 id" ".data.id" "$TPL_ID" "$R"

R=$(tpl_detail "$TPL_ID")
assert_code "t5.4 已删除 41003" "41003" "$R"

R=$(post "/templates/check-name" "{\"name\":\"${P}npc_combat\"}")
assert_field "t5.5 软删 name 不可复用" ".data.available" "false" "$R"

R=$(post "/templates/delete" '{"id":999999}')
assert_code "t5.6 不存在 41003" "41003" "$R"

R=$(post "/templates/delete" '{"id":0}')
assert_code "t5.7 ID=0 40000" "40000" "$R"

# 删除后再 toggle / references 应 41003
R=$(post "/templates/toggle-enabled" "{\"id\":${TPL_ID},\"enabled\":true,\"version\":1}")
assert_code "t5.8 删除模板 toggle 41003" "41003" "$R"

R=$(post "/templates/references" "{\"id\":${TPL_ID}}")
assert_code "t5.9 删除模板 references 41003" "41003" "$R"

# =============================================================================
# 攻击性测试：名称验证
# =============================================================================
subsection "ATK: 模板名称注入"

R=$(post "/templates/create" '{"name":"a]\"injection","label":"注入","description":"","fields":[]}')
assert_code_in "atk_tpl.4 特殊字符 name" "41002 41004" "$R"

LONG_TPL=$(printf 'a%.0s' $(seq 1 100))
R=$(post "/templates/create" "{\"name\":\"${LONG_TPL}\",\"label\":\"x\",\"description\":\"\",\"fields\":[{\"field_id\":${F_ATK},\"required\":true}]}")
assert_code "atk_tpl.5 超长模板 name 41002" "41002" "$R"

# 畸形 JSON
R=$(curl -s -X POST "$BASE/templates/create" -H "Content-Type: application/json" -d '{bad json}')
assert_code "atk_tpl.6 畸形 JSON 40000" "40000" "$R"

R=$(curl -s -X POST "$BASE/templates/create" -H "Content-Type: application/json" -d '')
assert_code "atk_tpl.7 空 body 40000" "40000" "$R"

# field_id 边界
R=$(post "/templates/create" "{\"name\":\"${P}atk_fz\",\"label\":\"x\",\"description\":\"\",\"fields\":[{\"field_id\":0,\"required\":true}]}")
assert_code "atk_tpl.8 field_id=0 40000" "40000" "$R"

R=$(post "/templates/create" "{\"name\":\"${P}atk_fn\",\"label\":\"x\",\"description\":\"\",\"fields\":[{\"field_id\":-1,\"required\":true}]}")
assert_code "atk_tpl.9 field_id=-1 40000" "40000" "$R"
