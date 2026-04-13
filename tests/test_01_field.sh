#!/bin/bash
# =============================================================================
# test_01_field.sh — 字段管理 CRUD + 约束收紧 + 引用验证 + 攻击性测试
#
# 前置：run_all.sh 已 source helpers.sh，$BASE / $P / assert_* / post() / fld_* 可用
# 导出变量：HP_ID ATK_ID STR_ID FLAG_ID MOOD_ID FLOAT_ID 等供后续测试使用
# =============================================================================

section "Part 1: 字段管理 — CRUD (prefix=$P)"

# =============================================================================
# 功能 2：新建字段
# =============================================================================
subsection "功能 2: 新建字段"

R=$(post "/fields/create" "{\"name\":\"${P}hp\",\"label\":\"测试生命值\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"HP\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}}}")
assert_code         "f2.1 创建成功"             "0" "$R"
assert_field        "f2.1 返回 name"            ".data.name" "${P}hp" "$R"
HP_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
assert_not_equal    "f2.1 id > 0"               ".data.id" "null" "$R"

R=$(fld_detail "$HP_ID")
assert_field  "f2.2 默认 enabled=false"   ".data.enabled"   "false" "$R"
assert_field  "f2.2 初始 version=1"       ".data.version"   "1"     "$R"
assert_field  "f2.2 初始 ref_count=0"     ".data.ref_count" "0"     "$R"

# ---- 名称校验 ----
R=$(post "/fields/create" "{\"name\":\"${P}hp\",\"label\":\"重复\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{}}")
assert_code "f2.3 重复 name 返回 40001" "40001" "$R"

R=$(post "/fields/create" '{"name":"HP-bad","label":"坏","type":"integer","category":"combat","properties":{}}')
assert_code "f2.4 大写+横线 40002" "40002" "$R"

R=$(post "/fields/create" '{"name":"123start","label":"数字开头","type":"integer","category":"combat","properties":{}}')
assert_code "f2.5 数字开头 40002" "40002" "$R"

R=$(post "/fields/create" '{"name":"","label":"空名","type":"integer","category":"combat","properties":{}}')
assert_code "f2.6 空 name 40002" "40002" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}nolabel\",\"label\":\"\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{}}")
assert_code "f2.7 空 label 40000" "40000" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}notype\",\"label\":\"无类型\",\"type\":\"\",\"category\":\"combat\",\"properties\":{}}")
assert_code "f2.8 空 type 40000" "40000" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}noprops\",\"label\":\"无属性\",\"type\":\"integer\",\"category\":\"combat\"}")
assert_code "f2.9 缺 properties 40000" "40000" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}badtype\",\"label\":\"假类型\",\"type\":\"faketype\",\"category\":\"combat\",\"properties\":{}}")
assert_code "f2.10 不存在的 type 40003" "40003" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}badcat\",\"label\":\"假分类\",\"type\":\"integer\",\"category\":\"fakecat\",\"properties\":{}}")
assert_code "f2.11 不存在的 category 40004" "40004" "$R"

R=$(post "/fields/create" '{"name":"a]\"injection","label":"注入","type":"integer","category":"combat","properties":{}}')
assert_code "f2.12 特殊字符 name 40002" "40002" "$R"

# ---- 字段池（供后续测试使用） ----
R=$(post "/fields/create" "{\"name\":\"${P}atk\",\"label\":\"攻击力\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"ATK\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":999}}}")
ATK_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
assert_code "f2.13 创建 atk (integer)" "0" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}str\",\"label\":\"名字文本\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{\"description\":\"STR\",\"expose_bb\":false,\"constraints\":{\"minLength\":1,\"maxLength\":50}}}")
STR_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
assert_code "f2.14 创建 str (string)" "0" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}flag\",\"label\":\"布尔标记\",\"type\":\"boolean\",\"category\":\"basic\",\"properties\":{\"description\":\"flag\",\"expose_bb\":false}}")
FLAG_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
assert_code "f2.15 创建 flag (boolean)" "0" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}mood\",\"label\":\"情绪选择\",\"type\":\"select\",\"category\":\"personality\",\"properties\":{\"description\":\"mood\",\"expose_bb\":false,\"constraints\":{\"options\":[{\"value\":\"happy\",\"label\":\"开心\"},{\"value\":\"sad\",\"label\":\"伤心\"}],\"minSelect\":1,\"maxSelect\":1}}}")
MOOD_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
assert_code "f2.16 创建 mood (select)" "0" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}fnum\",\"label\":\"浮点字段\",\"type\":\"float\",\"category\":\"combat\",\"properties\":{\"description\":\"fl\",\"expose_bb\":false,\"constraints\":{\"min\":0.0,\"max\":100.0,\"precision\":2}}}")
FLOAT_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
assert_code "f2.17 创建 fnum (float)" "0" "$R"

# =============================================================================
# 功能 6：唯一性校验 (check-name)
# =============================================================================
subsection "功能 6: 字段名唯一性校验"

R=$(post "/fields/check-name" "{\"name\":\"${P}hp\"}")
assert_code  "f6.1 已存在名字" "0" "$R"
assert_field "f6.1 available=false" ".data.available" "false" "$R"

R=$(post "/fields/check-name" "{\"name\":\"${P}notexist_xxx\"}")
assert_field "f6.2 不存在 available=true" ".data.available" "true" "$R"

R=$(post "/fields/check-name" '{"name":""}')
assert_code  "f6.3 空名 40000" "40000" "$R"

# =============================================================================
# 功能 3：字段详情
# =============================================================================
subsection "功能 3: 字段详情"

R=$(fld_detail "$HP_ID")
assert_code  "f3.1 详情成功" "0" "$R"
assert_field "f3.1 name 正确" ".data.name" "${P}hp" "$R"
assert_field "f3.1 label 正确" ".data.label" "测试生命值" "$R"
assert_field "f3.1 properties.description" ".data.properties.description" "HP" "$R"
assert_field "f3.1 constraints.min" ".data.properties.constraints.min" "0" "$R"
assert_field "f3.1 constraints.max" ".data.properties.constraints.max" "100" "$R"

R=$(fld_detail 999999)
assert_code "f3.2 不存在 ID 40011" "40011" "$R"

R=$(post "/fields/detail" '{"id":0}')
assert_code "f3.3 ID=0 40000" "40000" "$R"

R=$(post "/fields/detail" '{"id":-1}')
assert_code "f3.4 负 ID 40000" "40000" "$R"

# 停用中的字段详情也能查
fld_disable "$HP_ID" 2>/dev/null
R=$(fld_detail "$HP_ID")
assert_code  "f3.5 停用字段详情可查" "0" "$R"
assert_field "f3.5 enabled=false"   ".data.enabled" "false" "$R"

# =============================================================================
# 功能 1：字段列表
# =============================================================================
subsection "功能 1: 字段列表"

R=$(post "/fields/list" '{"page":1,"page_size":20}')
assert_code  "f1.1 列表成功" "0" "$R"
assert_ge    "f1.1 至少 6 条" ".data.total" "6" "$R"
assert_field "f1.1 items 数组" ".data.items | type" "array" "$R"
assert_not_equal "f1.2 items[0] 有 id" ".data.items[0].id" "null" "$R"

R=$(post "/fields/list" '{"type":"boolean","page":1,"page_size":20}')
assert_code "f1.3 按 type 筛选" "0" "$R"
assert_ge   "f1.3 >= 1 个 boolean" ".data.total" "1" "$R"

R=$(post "/fields/list" '{"category":"combat","page":1,"page_size":20}')
assert_code "f1.4 按 category 筛选" "0" "$R"
assert_ge   "f1.4 >= 2 个 combat" ".data.total" "2" "$R"

R=$(post "/fields/list" "{\"label\":\"测试生命\",\"page\":1,\"page_size\":20}")
assert_ge "f1.5 模糊搜索 >= 1" ".data.total" "1" "$R"

R=$(post "/fields/list" '{"enabled":true,"page":1,"page_size":20}')
assert_code "f1.6 enabled=true" "0" "$R"

R=$(post "/fields/list" '{"enabled":false,"page":1,"page_size":20}')
assert_code "f1.6b enabled=false" "0" "$R"

R=$(post "/fields/list" '{"page":0,"page_size":0}')
assert_field "f1.7 page=0 自动校正" ".data.page" "1" "$R"
assert_not_equal "f1.7 page_size=0 被校正" ".data.page_size" "0" "$R"

R=$(post "/fields/list" '{"label":"绝对不存在zzz","page":1,"page_size":20}')
assert_field "f1.8 空结果 total=0" ".data.total" "0" "$R"
assert_field "f1.8 空结果 items=[]" ".data.items | length" "0" "$R"

R=$(post "/fields/list" '{"page":999999,"page_size":20}')
assert_code  "f1.9 极大 page 成功" "0" "$R"
assert_field "f1.9 极大 page items=[]" ".data.items | length" "0" "$R"

R=$(post "/fields/list" '{"page":1,"page_size":10000}')
assert_code "f1.10 超大 page_size 成功（自动截断）" "0" "$R"

# category_label / type_label 翻译
R=$(post "/fields/list" '{"label":"攻击力","page":1,"page_size":20}')
assert_field "f1.11 type_label 已翻译" ".data.items[0].type_label" "整数" "$R"
assert_field "f1.11 category_label 已翻译" ".data.items[0].category_label" "战斗属性" "$R"

# =============================================================================
# 功能 4：编辑字段
# =============================================================================
subsection "功能 4: 编辑字段"

HP_VER=$(fld_version "$HP_ID")
R=$(post "/fields/update" "{\"id\":${HP_ID},\"label\":\"生命值改\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"HP changed\",\"expose_bb\":true,\"constraints\":{\"min\":0,\"max\":200}},\"version\":${HP_VER}}")
assert_code "f4.1 编辑成功（未启用）" "0" "$R"

R=$(fld_detail "$HP_ID")
assert_field "f4.1 label 已更新"          ".data.label" "生命值改" "$R"
assert_field "f4.1 max 已更新"            ".data.properties.constraints.max" "200" "$R"
assert_field "f4.1 expose_bb 已更新"      ".data.properties.expose_bb" "true" "$R"

# 缓存一致性：连续读两次应该都拿到新数据
R=$(fld_detail "$HP_ID")
assert_field "f4.1b 缓存一致（读 2 次仍是新数据）" ".data.label" "生命值改" "$R"

# 启用后禁止编辑
fld_enable "$HP_ID"
HP_VER=$(fld_version "$HP_ID")
R=$(post "/fields/update" "{\"id\":${HP_ID},\"label\":\"x\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{},\"version\":${HP_VER}}")
assert_code "f4.2 启用中编辑 40015" "40015" "$R"
fld_disable "$HP_ID"

# 乐观锁冲突
HP_VER=$(fld_version "$HP_ID")
R=$(post "/fields/update" "{\"id\":${HP_ID},\"label\":\"锁\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"lock\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":200}},\"version\":999}")
assert_code "f4.3 version 冲突 40010" "40010" "$R"

R=$(post "/fields/update" '{"id":999999,"label":"x","type":"integer","category":"combat","properties":{},"version":1}')
assert_code "f4.4 不存在 ID 40011" "40011" "$R"

R=$(post "/fields/update" '{"id":0,"label":"x","type":"integer","category":"combat","properties":{},"version":1}')
assert_code "f4.5 ID=0 40000" "40000" "$R"

HP_VER=$(fld_version "$HP_ID")
R=$(post "/fields/update" "{\"id\":${HP_ID},\"label\":\"x\",\"type\":\"faketype\",\"category\":\"combat\",\"properties\":{},\"version\":${HP_VER}}")
assert_code "f4.6 不存在 type 40003" "40003" "$R"

R=$(post "/fields/update" "{\"id\":${HP_ID},\"label\":\"x\",\"type\":\"integer\",\"category\":\"fakecat\",\"properties\":{},\"version\":${HP_VER}}")
assert_code "f4.7 不存在 category 40004" "40004" "$R"

R=$(post "/fields/update" "{\"id\":${HP_ID},\"label\":\"x\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{},\"version\":0}")
assert_code "f4.8 version=0 40000" "40000" "$R"

# noop 编辑应成功
HP_VER=$(fld_version "$HP_ID")
R=$(post "/fields/update" "{\"id\":${HP_ID},\"label\":\"生命值改\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"HP changed\",\"expose_bb\":true,\"constraints\":{\"min\":0,\"max\":200}},\"version\":${HP_VER}}")
assert_code "f4.9 noop 编辑成功" "0" "$R"

# =============================================================================
# 功能 8：启用/停用
# =============================================================================
subsection "功能 8: 启用/停用"

ATK_VER=$(fld_version "$ATK_ID")
R=$(post "/fields/toggle-enabled" "{\"id\":${ATK_ID},\"enabled\":true,\"version\":${ATK_VER}}")
assert_code "f8.1 启用成功" "0" "$R"
assert_field "f8.1 enabled=true" ".data.enabled" "true" "$(fld_detail $ATK_ID)"

ATK_VER=$(fld_version "$ATK_ID")
R=$(post "/fields/toggle-enabled" "{\"id\":${ATK_ID},\"enabled\":false,\"version\":${ATK_VER}}")
assert_code "f8.2 停用成功" "0" "$R"

R=$(post "/fields/toggle-enabled" "{\"id\":${ATK_ID},\"enabled\":true,\"version\":999}")
assert_code "f8.3 version 冲突 40010" "40010" "$R"

R=$(post "/fields/toggle-enabled" '{"id":999999,"enabled":true,"version":1}')
assert_code "f8.4 不存在 ID 40011" "40011" "$R"

R=$(post "/fields/toggle-enabled" '{"id":0,"enabled":true,"version":1}')
assert_code "f8.5 ID=0 40000" "40000" "$R"

# =============================================================================
# 功能 10/11：约束收紧 + 引用关系
# =============================================================================
section "Part 1b: 字段管理 — 约束收紧 + 引用关系"

fld_enable "$ATK_ID"

subsection "功能 10: 约束收紧检查"

# ---- integer 收紧 ----
R=$(post "/fields/create" "{\"name\":\"${P}tgt\",\"label\":\"收紧目标\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"tgt\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}}}")
TGT_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
fld_enable "$TGT_ID"

R=$(post "/fields/create" "{\"name\":\"${P}refone\",\"label\":\"引用一\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"ref\",\"expose_bb\":false,\"constraints\":{\"refs\":[${TGT_ID}]}}}")
assert_code "f10.1 创建 reference 字段" "0" "$R"
REFONE_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

R=$(fld_detail "$TGT_ID")
assert_field "f10.2 target ref_count=1" ".data.ref_count" "1" "$R"

fld_disable "$TGT_ID"
TGT_VER=$(fld_version "$TGT_ID")

R=$(post "/fields/update" "{\"id\":${TGT_ID},\"label\":\"t\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"t\",\"expose_bb\":false,\"constraints\":{\"min\":10,\"max\":100}},\"version\":${TGT_VER}}")
assert_code "f10.3 integer min 收紧 40007" "40007" "$R"

R=$(post "/fields/update" "{\"id\":${TGT_ID},\"label\":\"t\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"t\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":50}},\"version\":${TGT_VER}}")
assert_code "f10.4 integer max 收紧 40007" "40007" "$R"

R=$(post "/fields/update" "{\"id\":${TGT_ID},\"label\":\"t\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"t\",\"expose_bb\":false,\"constraints\":{\"min\":-10,\"max\":200}},\"version\":${TGT_VER}}")
assert_code "f10.5 放宽成功" "0" "$R"

# 被引用时禁止改类型
TGT_VER=$(fld_version "$TGT_ID")
R=$(post "/fields/update" "{\"id\":${TGT_ID},\"label\":\"t\",\"type\":\"string\",\"category\":\"combat\",\"properties\":{\"description\":\"t\",\"expose_bb\":false,\"constraints\":{\"minLength\":0,\"maxLength\":100}},\"version\":${TGT_VER}}")
assert_code "f10.6 被引用改 type 40006" "40006" "$R"

# ---- float 收紧 ----
R=$(post "/fields/create" "{\"name\":\"${P}ftgt\",\"label\":\"浮点目标\",\"type\":\"float\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"min\":0.0,\"max\":100.0,\"precision\":4}}}")
FTGT_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$FTGT_ID"
R=$(post "/fields/create" "{\"name\":\"${P}fholder\",\"label\":\"浮点持有\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"refs\":[${FTGT_ID}]}}}")
FHOLDER_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
fld_disable "$FTGT_ID"
FTGT_VER=$(fld_version "$FTGT_ID")

R=$(post "/fields/update" "{\"id\":${FTGT_ID},\"label\":\"浮点目标\",\"type\":\"float\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"min\":0.0,\"max\":100.0,\"precision\":2}},\"version\":${FTGT_VER}}")
assert_code "f10.7 float precision 4->2 40007" "40007" "$R"

FTGT_VER=$(fld_version "$FTGT_ID")
R=$(post "/fields/update" "{\"id\":${FTGT_ID},\"label\":\"浮点目标\",\"type\":\"float\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"min\":0.0,\"max\":100.0,\"precision\":6}},\"version\":${FTGT_VER}}")
assert_code "f10.8 float precision 4->6 放宽 ok" "0" "$R"

# ---- string 收紧 ----
R=$(post "/fields/create" "{\"name\":\"${P}stgt\",\"label\":\"字符目标\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"minLength\":0,\"maxLength\":100}}}")
STGT_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$STGT_ID"
R=$(post "/fields/create" "{\"name\":\"${P}sholder\",\"label\":\"字符持\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"refs\":[${STGT_ID}]}}}")
SHOLDER_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
fld_disable "$STGT_ID"
STGT_VER=$(fld_version "$STGT_ID")

R=$(post "/fields/update" "{\"id\":${STGT_ID},\"label\":\"字符目标\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"minLength\":5,\"maxLength\":100}},\"version\":${STGT_VER}}")
assert_code "f10.9 string minLength 0->5 40007" "40007" "$R"

R=$(post "/fields/update" "{\"id\":${STGT_ID},\"label\":\"字符目标\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"minLength\":0,\"maxLength\":50}},\"version\":${STGT_VER}}")
assert_code "f10.10 string maxLength 100->50 40007" "40007" "$R"

R=$(post "/fields/update" "{\"id\":${STGT_ID},\"label\":\"字符目标\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"minLength\":0,\"maxLength\":100,\"pattern\":\"^[a-z]+$\"}},\"version\":${STGT_VER}}")
assert_code "f10.11 string 新增 pattern 40007" "40007" "$R"

# ---- select 收紧 ----
R=$(post "/fields/create" "{\"name\":\"${P}seltgt\",\"label\":\"选择目标\",\"type\":\"select\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"options\":[{\"value\":\"a\",\"label\":\"A\"},{\"value\":\"b\",\"label\":\"B\"},{\"value\":\"c\",\"label\":\"C\"}],\"minSelect\":1,\"maxSelect\":3}}}")
SELTGT_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$SELTGT_ID"
R=$(post "/fields/create" "{\"name\":\"${P}selholder\",\"label\":\"选持\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"refs\":[${SELTGT_ID}]}}}")
SELHOLDER_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
fld_disable "$SELTGT_ID"
SELTGT_VER=$(fld_version "$SELTGT_ID")

R=$(post "/fields/update" "{\"id\":${SELTGT_ID},\"label\":\"选择目标\",\"type\":\"select\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"options\":[{\"value\":\"a\",\"label\":\"A\"},{\"value\":\"b\",\"label\":\"B\"}],\"minSelect\":1,\"maxSelect\":2}},\"version\":${SELTGT_VER}}")
assert_code "f10.12 select 删除 option 40007" "40007" "$R"

R=$(post "/fields/update" "{\"id\":${SELTGT_ID},\"label\":\"选择目标\",\"type\":\"select\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"options\":[{\"value\":\"a\",\"label\":\"A\"},{\"value\":\"b\",\"label\":\"B\"},{\"value\":\"c\",\"label\":\"C\"}],\"minSelect\":2,\"maxSelect\":3}},\"version\":${SELTGT_VER}}")
assert_code "f10.13 select minSelect 1->2 40007" "40007" "$R"

R=$(post "/fields/update" "{\"id\":${SELTGT_ID},\"label\":\"选择目标\",\"type\":\"select\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"options\":[{\"value\":\"a\",\"label\":\"A\"},{\"value\":\"b\",\"label\":\"B\"},{\"value\":\"c\",\"label\":\"C\"}],\"minSelect\":1,\"maxSelect\":2}},\"version\":${SELTGT_VER}}")
assert_code "f10.14 select maxSelect 3->2 40007" "40007" "$R"

R=$(post "/fields/update" "{\"id\":${SELTGT_ID},\"label\":\"选择目标\",\"type\":\"select\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"options\":[{\"value\":\"a\",\"label\":\"A\"},{\"value\":\"b\",\"label\":\"B\"},{\"value\":\"c\",\"label\":\"C\"},{\"value\":\"d\",\"label\":\"D\"}],\"minSelect\":1,\"maxSelect\":3}},\"version\":${SELTGT_VER}}")
assert_code "f10.15 select 追加 option ok" "0" "$R"

# ---- boolean 无约束 ----
R=$(post "/fields/create" "{\"name\":\"${P}btgt\",\"label\":\"布尔目标\",\"type\":\"boolean\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false}}")
BTGT_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$BTGT_ID"
R=$(post "/fields/create" "{\"name\":\"${P}bholder\",\"label\":\"布持\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"refs\":[${BTGT_ID}]}}}")
BHOLDER_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
fld_disable "$BTGT_ID"
BTGT_VER=$(fld_version "$BTGT_ID")
R=$(post "/fields/update" "{\"id\":${BTGT_ID},\"label\":\"布尔目标\",\"type\":\"boolean\",\"category\":\"basic\",\"properties\":{\"description\":\"boolean 编辑\",\"expose_bb\":false},\"version\":${BTGT_VER}}")
assert_code "f10.16 boolean 编辑 ok（无约束）" "0" "$R"

# =============================================================================
# 功能 11：reference 引用校验
# =============================================================================
subsection "功能 11: reference 引用校验"

R=$(post "/fields/create" "{\"name\":\"${P}cyc_a\",\"label\":\"A\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"A\",\"expose_bb\":false}}")
CA=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$CA"

R=$(post "/fields/create" "{\"name\":\"${P}cyc_b\",\"label\":\"B\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"B\",\"expose_bb\":false,\"constraints\":{\"refs\":[${CA}]}}}")
assert_code "f11.1 B refs [A] 成功" "0" "$R"
CB=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$CB"

# 嵌套 reference 应 40016
R=$(post "/fields/create" "{\"name\":\"${P}cyc_c\",\"label\":\"C\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"C\",\"expose_bb\":false,\"constraints\":{\"refs\":[${CB}]}}}")
assert_code "f11.2 C refs [B](reference 嵌套) 40016" "40016" "$R"

# 引用停用字段
R=$(post "/fields/create" "{\"name\":\"${P}cyc_d\",\"label\":\"D\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"D\",\"expose_bb\":false}}")
CD=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
R=$(post "/fields/create" "{\"name\":\"${P}cyc_e\",\"label\":\"E\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"E\",\"expose_bb\":false,\"constraints\":{\"refs\":[${CD}]}}}")
assert_code "f11.3 引用停用字段 40013" "40013" "$R"

# 引用不存在字段
R=$(post "/fields/create" "{\"name\":\"${P}cyc_f\",\"label\":\"F\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"F\",\"expose_bb\":false,\"constraints\":{\"refs\":[999999]}}}")
assert_code "f11.4 引用不存在字段 40014" "40014" "$R"

# 空 refs
R=$(post "/fields/create" "{\"name\":\"${P}cyc_g\",\"label\":\"G\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"G\",\"expose_bb\":false,\"constraints\":{\"refs\":[]}}}")
assert_code "f11.5 空 refs 40017" "40017" "$R"

# 删除 reference 字段后 target.ref_count 回退
fld_rm "$REFONE_ID"
R=$(fld_detail "$TGT_ID")
assert_field "f11.6 删除引用方后 target ref_count=0" ".data.ref_count" "0" "$R"

# =============================================================================
# 功能 7：字段引用详情
# =============================================================================
subsection "功能 7: 字段引用详情"

R=$(post "/fields/references" "{\"id\":${CA}}")
assert_code       "f7.1 查 A 引用详情" "0" "$R"
assert_field      "f7.1 field_id 正确" ".data.field_id" "$CA" "$R"
assert_ge         "f7.1 至少 1 个字段引用（B 引用 A）" ".data.fields | length" "1" "$R"
assert_field      "f7.1 fields[0] 有 label" ".data.fields[0].label" "B" "$R"

R=$(post "/fields/references" "{\"id\":${FLAG_ID}}")
assert_field "f7.2 无引用 templates=[]" ".data.templates | length" "0" "$R"
assert_field "f7.2 无引用 fields=[]"    ".data.fields | length" "0" "$R"

R=$(post "/fields/references" '{"id":999999}')
assert_code "f7.3 不存在 ID 40011" "40011" "$R"

# =============================================================================
# 功能 5：软删除字段
# =============================================================================
subsection "功能 5: 软删除字段"

fld_enable "$STR_ID"
R=$(post "/fields/delete" "{\"id\":${STR_ID}}")
assert_code "f5.1 启用中删除 40012" "40012" "$R"

fld_disable "$STR_ID"
R=$(post "/fields/delete" "{\"id\":${STR_ID}}")
assert_code "f5.2 停用后删除成功" "0" "$R"
assert_field "f5.2 返回 id" ".data.id" "$STR_ID" "$R"

R=$(fld_detail "$STR_ID")
assert_code "f5.3 已删除查不到 40011" "40011" "$R"

R=$(post "/fields/delete" '{"id":999999}')
assert_code "f5.4 不存在 40011" "40011" "$R"

R=$(post "/fields/delete" '{"id":0}')
assert_code "f5.5 ID=0 40000" "40000" "$R"

# 被引用字段无法删
fld_disable "$CA"
R=$(post "/fields/delete" "{\"id\":${CA}}")
assert_code "f5.6 被引用 40005" "40005" "$R"

# 软删除的 name 不可复用
R=$(post "/fields/check-name" "{\"name\":\"${P}str\"}")
assert_field "f5.7 软删 name 不可复用" ".data.available" "false" "$R"

R=$(post "/fields/delete" "{\"id\":${FLAG_ID}}")
assert_code "f5.8 无引用字段删除成功" "0" "$R"

# 已删除字段 toggle-enabled / references 应 40011
R=$(post "/fields/toggle-enabled" "{\"id\":${STR_ID},\"enabled\":true,\"version\":1}")
assert_code "f5.9 删除字段 toggle 40011" "40011" "$R"

R=$(post "/fields/references" "{\"id\":${STR_ID}}")
assert_code "f5.10 删除字段 references 40011" "40011" "$R"

# =============================================================================
# 攻击性测试
# =============================================================================
section "Part 1c: 字段管理 — 攻击性测试"

# ---- properties 形状校验 ----
subsection "ATK: properties 形状校验"

R=$(post "/fields/create" "{\"name\":\"${P}atk_p_null\",\"label\":\"x\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":null}")
assert_code "atk.1 properties=null 40000" "40000" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}atk_p_arr\",\"label\":\"x\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":[]}")
assert_code "atk.2 properties=[] 40000" "40000" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}atk_p_num\",\"label\":\"x\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":123}")
assert_code "atk.3 properties=123 40000" "40000" "$R"

# ---- SQL 注入 ----
subsection "ATK: SQL 注入 / XSS / LIKE 通配"

R=$(post "/fields/create" "{\"name\":\"${P}sqli\",\"label\":\"'; DROP TABLE fields; --\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{}}")
CODE=$(echo "$R" | jq -r '.code' | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$CODE" = "0" ]; then
  echo "  [PASS] atk.4 SQL-like label 被安全处理"
  PASS=$((PASS+1))
  SQLI_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
  fld_rm "$SQLI_ID"
else
  echo "  [FAIL] atk.4 意外 code=$CODE"; FAIL=$((FAIL+1))
fi

# XSS in label
R=$(post "/fields/create" "{\"name\":\"${P}xss\",\"label\":\"<script>alert(1)</script>\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{}}")
CODE=$(echo "$R" | jq -r '.code' | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$CODE" = "0" ]; then
  echo "  [PASS] atk.5 XSS label 被安全处理（存储型 XSS 不影响 API 层）"
  PASS=$((PASS+1))
  XSS_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
  fld_rm "$XSS_ID"
else
  echo "  [FAIL] atk.5 意外 code=$CODE"; FAIL=$((FAIL+1))
fi

# LIKE wildcard % in search
R=$(post "/fields/list" '{"label":"%","page":1,"page_size":20}')
assert_code "atk.6 LIKE % 搜索不报错" "0" "$R"

# ---- CJK 字符长度计数 ----
subsection "ATK: CJK 字符长度"

LONG_NAME=$(printf 'a%.0s' $(seq 1 100))
R=$(post "/fields/create" "{\"name\":\"${LONG_NAME}\",\"label\":\"超长\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{}}")
assert_code "atk.7 超长 name 40002" "40002" "$R"

# ---- 缓存穿透 ----
subsection "ATK: 缓存穿透"

for i in 1 2 3; do
  R=$(fld_detail 999999)
  CODE=$(echo "$R" | jq -r '.code' | tr -d '\r')
  TOTAL=$((TOTAL + 1))
  if [ "$CODE" = "40011" ]; then
    echo "  [PASS] atk.8.${i} 不存在 ID 第 ${i} 次返回 40011"
    PASS=$((PASS+1))
  else
    echo "  [FAIL] atk.8.${i} code=$CODE"
    FAIL=$((FAIL+1))
  fi
done

# ---- 缓存一致性 ----
subsection "ATK: 缓存一致性"

R=$(post "/fields/create" "{\"name\":\"${P}atk_cache\",\"label\":\"初始\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"v1\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}}}")
CACHE_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

R=$(fld_detail "$CACHE_ID")
assert_field "atk.9a 初始值" ".data.label" "初始" "$R"

V=$(fld_version "$CACHE_ID")
R=$(post "/fields/update" "{\"id\":${CACHE_ID},\"label\":\"已改\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"v2\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}},\"version\":${V}}")
assert_code "atk.9b 立即编辑成功" "0" "$R"

R=$(fld_detail "$CACHE_ID")
assert_field "atk.9c 编辑后立即读 label=已改" ".data.label" "已改" "$R"

# ---- 列表缓存一致性 ----
subsection "ATK: 列表缓存一致性"

R=$(post "/fields/list" '{"label":"原子操作","page":1,"page_size":20}')
BEFORE_TOTAL=$(echo "$R" | jq -r '.data.total' | tr -d '\r')

R=$(post "/fields/create" "{\"name\":\"${P}atk_atomic\",\"label\":\"原子操作\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false}}")
ATOMIC_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

R=$(post "/fields/list" '{"label":"原子操作","page":1,"page_size":20}')
AFTER_TOTAL=$(echo "$R" | jq -r '.data.total' | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$((AFTER_TOTAL - BEFORE_TOTAL))" = "1" ]; then
  echo "  [PASS] atk.10 创建后列表立即反映 ($BEFORE_TOTAL -> $AFTER_TOTAL)"
  PASS=$((PASS+1))
else
  echo "  [BUG ] atk.10 列表未反映新建字段 ($BEFORE_TOTAL -> $AFTER_TOTAL)"
  FAIL=$((FAIL+1))
  BUGS+=("atk.10: 创建字段后列表缓存未正确失效")
fi

fld_rm "$ATOMIC_ID"

# ---- 版本号负值 ----
subsection "ATK: 版本号负值"

R=$(post "/fields/toggle-enabled" "{\"id\":${HP_ID},\"enabled\":false,\"version\":-1}")
assert_code "atk.11 version=-1 toggle 40000" "40000" "$R"

R=$(post "/fields/update" "{\"id\":${HP_ID},\"label\":\"x\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{},\"version\":-999}")
assert_code "atk.12 update version=-999 40000" "40000" "$R"

# ---- 生命周期：删除后 list 不可见 ----
subsection "ATK: 生命周期"

R=$(post "/fields/create" "{\"name\":\"${P}atk_lifecycle\",\"label\":\"生命周期\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false}}")
LIFE_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

R=$(post "/fields/list" '{"label":"生命周期","page":1,"page_size":20}')
assert_ge "atk.13a 创建后 list 可见" ".data.total" "1" "$R"

fld_rm "$LIFE_ID"

R=$(post "/fields/list" '{"label":"生命周期","page":1,"page_size":20}')
assert_field "atk.13b 删除后 list 不可见 total=0" ".data.total" "0" "$R"
