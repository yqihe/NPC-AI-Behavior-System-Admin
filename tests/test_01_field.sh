#!/bin/bash
# =============================================================================
# test_01_field.sh — 字段管理 CRUD + 约束收紧 + 引用验证 + 约束校验 + 攻击性测试
#
# 前置：run_all.sh 已 source helpers.sh，$BASE / $P / assert_* / post() / fld_* 可用
# 导出变量：HP_ID ATK_ID STR_ID FLAG_ID MOOD_ID FLOAT_ID CA CB 供后续测试使用
# =============================================================================

# =============================================================================
# 1. CRUD — 创建所有类型
# =============================================================================
section "Part 1: 字段管理 — CRUD (prefix=$P)"

subsection "1.1 创建 — 全类型覆盖"

R=$(post "/fields/create" "{\"name\":\"${P}hp\",\"label\":\"测试生命值\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"HP\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}}}")
assert_code         "f1.1 创建 integer 成功"     "0" "$R"
assert_field        "f1.1 返回 name"              ".data.name" "${P}hp" "$R"
HP_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
assert_not_equal    "f1.1 id > 0"                 ".data.id" "null" "$R"

R=$(fld_detail "$HP_ID")
assert_field  "f1.2 默认 enabled=false"    ".data.enabled"   "false" "$R"
assert_field  "f1.2 初始 version=1"        ".data.version"   "1"     "$R"
assert_field  "f1.2 初始 ref_count=0"      ".data.ref_count" "0"     "$R"

R=$(post "/fields/create" "{\"name\":\"${P}atk\",\"label\":\"攻击力\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"ATK\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":999}}}")
ATK_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
assert_code "f1.3 创建 atk (integer)" "0" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}str\",\"label\":\"名字文本\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{\"description\":\"STR\",\"expose_bb\":false,\"constraints\":{\"minLength\":1,\"maxLength\":50}}}")
STR_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
assert_code "f1.4 创建 str (string)" "0" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}flag\",\"label\":\"布尔标记\",\"type\":\"boolean\",\"category\":\"basic\",\"properties\":{\"description\":\"flag\",\"expose_bb\":false}}")
FLAG_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
assert_code "f1.5 创建 flag (boolean)" "0" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}mood\",\"label\":\"情绪选择\",\"type\":\"select\",\"category\":\"personality\",\"properties\":{\"description\":\"mood\",\"expose_bb\":false,\"constraints\":{\"options\":[{\"value\":\"happy\",\"label\":\"开心\"},{\"value\":\"sad\",\"label\":\"伤心\"}],\"minSelect\":1,\"maxSelect\":1}}}")
MOOD_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
assert_code "f1.6 创建 mood (select)" "0" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}fnum\",\"label\":\"浮点字段\",\"type\":\"float\",\"category\":\"combat\",\"properties\":{\"description\":\"fl\",\"expose_bb\":false,\"constraints\":{\"min\":0.0,\"max\":100.0,\"precision\":2}}}")
FLOAT_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
assert_code "f1.7 创建 fnum (float)" "0" "$R"

# =============================================================================
# 2. 名称校验
# =============================================================================
subsection "1.2 创建 — 名称校验"

R=$(post "/fields/create" "{\"name\":\"${P}hp\",\"label\":\"重复\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{}}")
assert_code "f2.1 重复 name 40001" "40001" "$R"

R=$(post "/fields/create" '{"name":"HP-bad","label":"x","type":"integer","category":"combat","properties":{}}')
assert_code "f2.2 大写+横线 40002" "40002" "$R"

R=$(post "/fields/create" '{"name":"123start","label":"x","type":"integer","category":"combat","properties":{}}')
assert_code "f2.3 数字开头 40002" "40002" "$R"

R=$(post "/fields/create" '{"name":"","label":"x","type":"integer","category":"combat","properties":{}}')
assert_code "f2.4 空 name 40002" "40002" "$R"

R=$(post "/fields/create" '{"name":"has space","label":"x","type":"integer","category":"combat","properties":{}}')
assert_code "f2.5 含空格 40002" "40002" "$R"

R=$(post "/fields/create" '{"name":"has@special","label":"x","type":"integer","category":"combat","properties":{}}')
assert_code "f2.6 特殊字符 40002" "40002" "$R"

R=$(post "/fields/create" "{\"name\":\"中文名\",\"label\":\"x\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{}}")
assert_code "f2.7 中文 name 40002" "40002" "$R"

R=$(post "/fields/create" '{"name":"a]\"injection","label":"x","type":"integer","category":"combat","properties":{}}')
assert_code "f2.8 JSON 特殊字符 40002" "40002" "$R"

R=$(post "/fields/create" '{"name":"_leading","label":"x","type":"integer","category":"combat","properties":{}}')
assert_code "f2.9 下划线开头 40002" "40002" "$R"

# 64 字符边界 — 刚好 64 个小写字母应该成功
NAME64=$(printf 'a%.0s' $(seq 1 64))
R=$(post "/fields/create" "{\"name\":\"${NAME64}\",\"label\":\"64字符\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{}}")
assert_code "f2.10 64 字符 name 成功" "0" "$R"
NAME64_ID=$(echo "$R" | jq -r '.data.id // empty' | tr -d '\r')
if [ -n "$NAME64_ID" ] && [ "$NAME64_ID" != "null" ]; then fld_rm "$NAME64_ID"; fi

# 65 字符 — 超出限制
NAME65=$(printf 'a%.0s' $(seq 1 65))
R=$(post "/fields/create" "{\"name\":\"${NAME65}\",\"label\":\"65字符\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{}}")
assert_code "f2.11 65 字符 name 40002" "40002" "$R"

# 100 字符超长
LONG_NAME=$(printf 'a%.0s' $(seq 1 100))
R=$(post "/fields/create" "{\"name\":\"${LONG_NAME}\",\"label\":\"超长\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{}}")
assert_code "f2.12 超长 name 40002" "40002" "$R"

# 空 label / 空 type / 缺 properties
R=$(post "/fields/create" "{\"name\":\"${P}nolabel\",\"label\":\"\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{}}")
assert_code "f2.13 空 label 40000" "40000" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}notype\",\"label\":\"x\",\"type\":\"\",\"category\":\"combat\",\"properties\":{}}")
assert_code "f2.14 空 type 40000" "40000" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}noprops\",\"label\":\"x\",\"type\":\"integer\",\"category\":\"combat\"}")
assert_code "f2.15 缺 properties 40000" "40000" "$R"

# 字典不存在
R=$(post "/fields/create" "{\"name\":\"${P}badtype\",\"label\":\"x\",\"type\":\"faketype\",\"category\":\"combat\",\"properties\":{}}")
assert_code "f2.16 不存在的 type 40003" "40003" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}badcat\",\"label\":\"x\",\"type\":\"integer\",\"category\":\"fakecat\",\"properties\":{}}")
assert_code "f2.17 不存在的 category 40004" "40004" "$R"

# =============================================================================
# 3. check-name 唯一性校验
# =============================================================================
subsection "1.3 check-name 唯一性校验"

R=$(post "/fields/check-name" "{\"name\":\"${P}hp\"}")
assert_code  "f3.1 已存在 name code=0" "0" "$R"
assert_field "f3.1 available=false"     ".data.available" "false" "$R"

R=$(post "/fields/check-name" "{\"name\":\"${P}notexist_xxx\"}")
assert_field "f3.2 不存在 available=true" ".data.available" "true" "$R"

R=$(post "/fields/check-name" '{"name":""}')
assert_code  "f3.3 空名 40000" "40000" "$R"

R=$(post "/fields/check-name" '{"name":"BAD_FORMAT"}')
assert_code  "f3.4 格式不合法 40002" "40002" "$R"

# =============================================================================
# 4. 字段详情
# =============================================================================
subsection "1.4 字段详情"

R=$(fld_detail "$HP_ID")
assert_code  "f4.1 详情成功"                   "0" "$R"
assert_field "f4.1 name 正确"                  ".data.name" "${P}hp" "$R"
assert_field "f4.1 label 正确"                 ".data.label" "测试生命值" "$R"
assert_field "f4.1 properties.description"     ".data.properties.description" "HP" "$R"
assert_field "f4.1 constraints.min"            ".data.properties.constraints.min" "0" "$R"
assert_field "f4.1 constraints.max"            ".data.properties.constraints.max" "100" "$R"

R=$(fld_detail 999999)
assert_code "f4.2 不存在 ID 40011" "40011" "$R"

R=$(post "/fields/detail" '{"id":0}')
assert_code "f4.3 ID=0 40000" "40000" "$R"

R=$(post "/fields/detail" '{"id":-1}')
assert_code "f4.4 负 ID 40000" "40000" "$R"

# 停用中的字段详情也能查
fld_disable "$HP_ID" 2>/dev/null
R=$(fld_detail "$HP_ID")
assert_code  "f4.5 停用字段详情可查" "0" "$R"
assert_field "f4.5 enabled=false"   ".data.enabled" "false" "$R"

# created_at / updated_at 存在
assert_exists "f4.6 created_at 存在" ".data.created_at" "$R"
assert_exists "f4.7 updated_at 存在" ".data.updated_at" "$R"

# =============================================================================
# 5. 字段列表 — 全过滤器 + 分页边界
# =============================================================================
subsection "1.5 字段列表"

R=$(post "/fields/list" '{"page":1,"page_size":20}')
assert_code  "f5.1 列表成功" "0" "$R"
assert_ge    "f5.1 至少 6 条" ".data.total" "6" "$R"
assert_field "f5.1 items 数组" ".data.items | type" "array" "$R"
assert_not_equal "f5.1 items[0] 有 id" ".data.items[0].id" "null" "$R"

# type 筛选
R=$(post "/fields/list" '{"type":"boolean","page":1,"page_size":20}')
assert_code "f5.2 按 type=boolean 筛选" "0" "$R"
assert_ge   "f5.2 >= 1 个 boolean" ".data.total" "1" "$R"

# category 筛选
R=$(post "/fields/list" '{"category":"combat","page":1,"page_size":20}')
assert_code "f5.3 按 category=combat 筛选" "0" "$R"
assert_ge   "f5.3 >= 2 个 combat" ".data.total" "2" "$R"

# label 模糊搜索
R=$(post "/fields/list" "{\"label\":\"测试生命\",\"page\":1,\"page_size\":20}")
assert_ge "f5.4 模糊搜索 >= 1" ".data.total" "1" "$R"

# enabled 筛选
R=$(post "/fields/list" '{"enabled":true,"page":1,"page_size":20}')
assert_code "f5.5 enabled=true" "0" "$R"

R=$(post "/fields/list" '{"enabled":false,"page":1,"page_size":20}')
assert_code "f5.5b enabled=false" "0" "$R"

# 分页边界：page=0, page_size=0
R=$(post "/fields/list" '{"page":0,"page_size":0}')
assert_field "f5.6 page=0 自动校正" ".data.page" "1" "$R"
assert_not_equal "f5.6 page_size=0 被校正" ".data.page_size" "0" "$R"

# 空结果
R=$(post "/fields/list" '{"label":"绝对不存在zzz","page":1,"page_size":20}')
assert_field "f5.7 空结果 total=0" ".data.total" "0" "$R"
assert_field "f5.7 空结果 items=[]" ".data.items | length" "0" "$R"

# 极大 page
R=$(post "/fields/list" '{"page":999999,"page_size":20}')
assert_code  "f5.8 极大 page 成功" "0" "$R"
assert_field "f5.8 极大 page items=[]" ".data.items | length" "0" "$R"

# page_size > 100 自动截断
R=$(post "/fields/list" '{"page":1,"page_size":10000}')
assert_code "f5.9 超大 page_size 成功（自动截断）" "0" "$R"

# 组合筛选
R=$(post "/fields/list" '{"type":"integer","category":"combat","page":1,"page_size":20}')
assert_code "f5.10 组合筛选 type+category" "0" "$R"
assert_ge   "f5.10 >= 2 条" ".data.total" "2" "$R"

# type_label / category_label 翻译
R=$(post "/fields/list" '{"label":"攻击力","page":1,"page_size":20}')
assert_field "f5.11 type_label 已翻译" ".data.items[0].type_label" "整数" "$R"
assert_field "f5.11 category_label 已翻译" ".data.items[0].category_label" "战斗属性" "$R"

# 负数 page
R=$(post "/fields/list" '{"page":-1,"page_size":20}')
assert_code "f5.12 负 page 不报 500" "0" "$R"

# =============================================================================
# 6. 编辑字段
# =============================================================================
subsection "1.6 编辑字段"

HP_VER=$(fld_version "$HP_ID")
R=$(post "/fields/update" "{\"id\":${HP_ID},\"label\":\"生命值改\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"HP changed\",\"expose_bb\":true,\"constraints\":{\"min\":0,\"max\":200}},\"version\":${HP_VER}}")
assert_code "f6.1 编辑成功（未启用）" "0" "$R"

R=$(fld_detail "$HP_ID")
assert_field "f6.1 label 已更新"         ".data.label" "生命值改" "$R"
assert_field "f6.1 max 已更新"           ".data.properties.constraints.max" "200" "$R"
assert_field "f6.1 expose_bb 已更新"     ".data.properties.expose_bb" "true" "$R"

# 缓存一致性：连续读两次应该都拿到新数据
R=$(fld_detail "$HP_ID")
assert_field "f6.1b 缓存一致（读 2 次仍是新数据）" ".data.label" "生命值改" "$R"

# 启用后禁止编辑 — 40015
fld_enable "$HP_ID"
HP_VER=$(fld_version "$HP_ID")
R=$(post "/fields/update" "{\"id\":${HP_ID},\"label\":\"x\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{},\"version\":${HP_VER}}")
assert_code "f6.2 启用中编辑 40015" "40015" "$R"
fld_disable "$HP_ID"

# 乐观锁冲突
HP_VER=$(fld_version "$HP_ID")
R=$(post "/fields/update" "{\"id\":${HP_ID},\"label\":\"锁\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"lock\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":200}},\"version\":999}")
assert_code "f6.3 version 冲突 40010" "40010" "$R"

R=$(post "/fields/update" '{"id":999999,"label":"x","type":"integer","category":"combat","properties":{},"version":1}')
assert_code "f6.4 不存在 ID 40011" "40011" "$R"

R=$(post "/fields/update" '{"id":0,"label":"x","type":"integer","category":"combat","properties":{},"version":1}')
assert_code "f6.5 ID=0 40000" "40000" "$R"

HP_VER=$(fld_version "$HP_ID")
R=$(post "/fields/update" "{\"id\":${HP_ID},\"label\":\"x\",\"type\":\"faketype\",\"category\":\"combat\",\"properties\":{},\"version\":${HP_VER}}")
assert_code "f6.6 不存在 type 40003" "40003" "$R"

R=$(post "/fields/update" "{\"id\":${HP_ID},\"label\":\"x\",\"type\":\"integer\",\"category\":\"fakecat\",\"properties\":{},\"version\":${HP_VER}}")
assert_code "f6.7 不存在 category 40004" "40004" "$R"

R=$(post "/fields/update" "{\"id\":${HP_ID},\"label\":\"x\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{},\"version\":0}")
assert_code "f6.8 version=0 40000" "40000" "$R"

# noop 编辑应成功
HP_VER=$(fld_version "$HP_ID")
R=$(post "/fields/update" "{\"id\":${HP_ID},\"label\":\"生命值改\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"HP changed\",\"expose_bb\":true,\"constraints\":{\"min\":0,\"max\":200}},\"version\":${HP_VER}}")
assert_code "f6.9 noop 编辑成功" "0" "$R"

# version 递增确认
HP_VER_AFTER=$(fld_version "$HP_ID")
TOTAL=$((TOTAL + 1))
if [ "$HP_VER_AFTER" -gt "$HP_VER" ] 2>/dev/null; then
  echo "  [PASS] f6.10 noop 编辑 version 递增 ($HP_VER -> $HP_VER_AFTER)"
  PASS=$((PASS+1))
else
  echo "  [FAIL] f6.10 version 应递增 ($HP_VER -> $HP_VER_AFTER)"
  FAIL=$((FAIL+1))
fi

# =============================================================================
# 7. 启用/停用
# =============================================================================
subsection "1.7 启用/停用"

ATK_VER=$(fld_version "$ATK_ID")
R=$(post "/fields/toggle-enabled" "{\"id\":${ATK_ID},\"enabled\":true,\"version\":${ATK_VER}}")
assert_code "f7.1 启用成功" "0" "$R"
assert_field "f7.1 enabled=true" ".data.enabled" "true" "$(fld_detail $ATK_ID)"

ATK_VER=$(fld_version "$ATK_ID")
R=$(post "/fields/toggle-enabled" "{\"id\":${ATK_ID},\"enabled\":false,\"version\":${ATK_VER}}")
assert_code "f7.2 停用成功" "0" "$R"

R=$(post "/fields/toggle-enabled" "{\"id\":${ATK_ID},\"enabled\":true,\"version\":999}")
assert_code "f7.3 version 冲突 40010" "40010" "$R"

R=$(post "/fields/toggle-enabled" '{"id":999999,"enabled":true,"version":1}')
assert_code "f7.4 不存在 ID 40011" "40011" "$R"

R=$(post "/fields/toggle-enabled" '{"id":0,"enabled":true,"version":1}')
assert_code "f7.5 ID=0 40000" "40000" "$R"

# 幂等：双重 toggle
ATK_VER=$(fld_version "$ATK_ID")
R=$(post "/fields/toggle-enabled" "{\"id\":${ATK_ID},\"enabled\":true,\"version\":${ATK_VER}}")
assert_code "f7.6 第一次启用成功" "0" "$R"
ATK_VER=$(fld_version "$ATK_ID")
R=$(post "/fields/toggle-enabled" "{\"id\":${ATK_ID},\"enabled\":true,\"version\":${ATK_VER}}")
assert_code "f7.7 幂等第二次启用成功" "0" "$R"

# 幂等停用
ATK_VER=$(fld_version "$ATK_ID")
R=$(post "/fields/toggle-enabled" "{\"id\":${ATK_ID},\"enabled\":false,\"version\":${ATK_VER}}")
assert_code "f7.8 停用" "0" "$R"
ATK_VER=$(fld_version "$ATK_ID")
R=$(post "/fields/toggle-enabled" "{\"id\":${ATK_ID},\"enabled\":false,\"version\":${ATK_VER}}")
assert_code "f7.9 幂等第二次停用成功" "0" "$R"

# =============================================================================
# 8. 软删除
# =============================================================================
subsection "1.8 软删除"

fld_enable "$STR_ID"
R=$(post "/fields/delete" "{\"id\":${STR_ID}}")
assert_code "f8.1 启用中删除 40012" "40012" "$R"

fld_disable "$STR_ID"
R=$(post "/fields/delete" "{\"id\":${STR_ID}}")
assert_code "f8.2 停用后删除成功" "0" "$R"
assert_field "f8.2 返回 id" ".data.id" "$STR_ID" "$R"

R=$(fld_detail "$STR_ID")
assert_code "f8.3 已删除查不到 40011" "40011" "$R"

R=$(post "/fields/delete" '{"id":999999}')
assert_code "f8.4 不存在 40011" "40011" "$R"

R=$(post "/fields/delete" '{"id":0}')
assert_code "f8.5 ID=0 40000" "40000" "$R"

# 软删除 name 不可复用
R=$(post "/fields/check-name" "{\"name\":\"${P}str\"}")
assert_field "f8.6 软删 name 不可复用" ".data.available" "false" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}str\",\"label\":\"试重建\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{}}")
assert_code "f8.7 软删 name 重建 40001" "40001" "$R"

# 删除无引用字段成功
R=$(post "/fields/delete" "{\"id\":${FLAG_ID}}")
assert_code "f8.8 无引用字段删除成功" "0" "$R"

# 已删除字段 toggle / references 应 40011
R=$(post "/fields/toggle-enabled" "{\"id\":${STR_ID},\"enabled\":true,\"version\":1}")
assert_code "f8.9 删除字段 toggle 40011" "40011" "$R"

R=$(post "/fields/references" "{\"id\":${STR_ID}}")
assert_code "f8.10 删除字段 references 40011" "40011" "$R"

# 已删除字段 update 应 40011
R=$(post "/fields/update" "{\"id\":${STR_ID},\"label\":\"x\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{},\"version\":1}")
assert_code "f8.11 删除字段 update 40011" "40011" "$R"

# =============================================================================
# 9. Reference 字段引用校验
# =============================================================================
section "Part 1b: 字段管理 — 引用关系"

fld_enable "$ATK_ID"

subsection "1.9 reference 引用校验"

R=$(post "/fields/create" "{\"name\":\"${P}cyc_a\",\"label\":\"A\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"A\",\"expose_bb\":false}}")
CA=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$CA"

R=$(post "/fields/create" "{\"name\":\"${P}cyc_b\",\"label\":\"B\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"B\",\"expose_bb\":false,\"constraints\":{\"refs\":[${CA}]}}}")
assert_code "f9.1 B refs [A] 成功" "0" "$R"
CB=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$CB"

# 嵌套 reference 40016
R=$(post "/fields/create" "{\"name\":\"${P}cyc_c\",\"label\":\"C\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"C\",\"expose_bb\":false,\"constraints\":{\"refs\":[${CB}]}}}")
assert_code "f9.2 嵌套 reference 40016" "40016" "$R"

# 引用停用字段 40013
R=$(post "/fields/create" "{\"name\":\"${P}cyc_d\",\"label\":\"D\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"D\",\"expose_bb\":false}}")
CD=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
R=$(post "/fields/create" "{\"name\":\"${P}cyc_e\",\"label\":\"E\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"E\",\"expose_bb\":false,\"constraints\":{\"refs\":[${CD}]}}}")
assert_code "f9.3 引用停用字段 40013" "40013" "$R"

# 引用不存在字段 40014
R=$(post "/fields/create" "{\"name\":\"${P}cyc_f\",\"label\":\"F\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"F\",\"expose_bb\":false,\"constraints\":{\"refs\":[999999]}}}")
assert_code "f9.4 引用不存在字段 40014" "40014" "$R"

# 空 refs 40017
R=$(post "/fields/create" "{\"name\":\"${P}cyc_g\",\"label\":\"G\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"G\",\"expose_bb\":false,\"constraints\":{\"refs\":[]}}}")
assert_code "f9.5 空 refs 40017" "40017" "$R"

# 多目标引用
R=$(post "/fields/create" "{\"name\":\"${P}multi_tgt\",\"label\":\"多目标\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false}}")
MULTI_TGT=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$MULTI_TGT"

R=$(post "/fields/create" "{\"name\":\"${P}multi_ref\",\"label\":\"多目标引用\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"refs\":[${CA},${MULTI_TGT}]}}}")
assert_code "f9.6 多目标引用成功" "0" "$R"
MULTI_REF_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

# ref_count 追踪
R=$(fld_detail "$CA")
assert_ge "f9.7 A.ref_count >= 2 (B+multi)" ".data.ref_count" "2" "$R"

R=$(fld_detail "$MULTI_TGT")
assert_field "f9.8 multi_tgt.ref_count=1" ".data.ref_count" "1" "$R"

# 删除引用方后 ref_count 回退
fld_rm "$MULTI_REF_ID"
R=$(fld_detail "$MULTI_TGT")
assert_field "f9.9 删除引用方后 ref_count=0" ".data.ref_count" "0" "$R"
fld_disable "$MULTI_TGT"; fld_rm "$MULTI_TGT"

# 缺少 refs 字段（reference 无 constraints）
R=$(post "/fields/create" "{\"name\":\"${P}ref_noc\",\"label\":\"无约束ref\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false}}")
assert_code_in "f9.10 reference 无 constraints 40017/40000" "40017 40000" "$R"

# 混合引用含停用字段
R=$(post "/fields/create" "{\"name\":\"${P}dis_mix_a\",\"label\":\"停用A\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false}}")
DIS_MIX_A=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
R=$(post "/fields/create" "{\"name\":\"${P}dis_mix_ref\",\"label\":\"混合引用\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"refs\":[${CA},${DIS_MIX_A}]}}}")
assert_code "f9.11 混合引用含停用 40013" "40013" "$R"
fld_rm "$DIS_MIX_A"

# =============================================================================
# 字段引用详情 (/fields/references)
# =============================================================================
subsection "1.9b 引用详情"

R=$(post "/fields/references" "{\"id\":${CA}}")
assert_code  "f9r.1 查 A 引用详情" "0" "$R"
assert_field "f9r.1 field_id 正确" ".data.field_id" "$CA" "$R"
assert_ge    "f9r.1 至少 1 个字段引用" ".data.fields | length" "1" "$R"
assert_field "f9r.1 fields[0] 有 label" ".data.fields[0].label" "B" "$R"

R=$(post "/fields/references" "{\"id\":${MOOD_ID}}")
assert_field "f9r.2 无引用 templates=[]" ".data.templates | length" "0" "$R"
assert_field "f9r.2 无引用 fields=[]"    ".data.fields | length" "0" "$R"

R=$(post "/fields/references" '{"id":999999}')
assert_code "f9r.3 不存在 ID 40011" "40011" "$R"

# 被引用字段无法删
fld_disable "$CA"
R=$(post "/fields/delete" "{\"id\":${CA}}")
assert_code "f9r.4 被引用 40005" "40005" "$R"
fld_enable "$CA"

# =============================================================================
# 10. 约束收紧 (40007) — 被引用字段约束不可收紧
# =============================================================================
section "Part 1c: 字段管理 — 约束收紧 + 类型变更守卫"

subsection "1.10 约束收紧检查"

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

R=$(post "/fields/update" "{\"id\":${TGT_ID},\"label\":\"t\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"t\",\"expose_bb\":false,\"constraints\":{\"min\":10,\"max\":50}},\"version\":${TGT_VER}}")
assert_code "f10.4b integer min+max 同时收紧 40007" "40007" "$R"

R=$(post "/fields/update" "{\"id\":${TGT_ID},\"label\":\"t\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"t\",\"expose_bb\":false,\"constraints\":{\"min\":-10,\"max\":200}},\"version\":${TGT_VER}}")
assert_code "f10.5 放宽成功" "0" "$R"

# type change blocked when referenced (40006)
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
assert_code "f10.7 float precision 4->2 收紧 40007" "40007" "$R"

R=$(post "/fields/update" "{\"id\":${FTGT_ID},\"label\":\"浮点目标\",\"type\":\"float\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"min\":5.0,\"max\":100.0,\"precision\":4}},\"version\":${FTGT_VER}}")
assert_code "f10.7b float min 收紧 40007" "40007" "$R"

R=$(post "/fields/update" "{\"id\":${FTGT_ID},\"label\":\"浮点目标\",\"type\":\"float\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"min\":0.0,\"max\":50.0,\"precision\":4}},\"version\":${FTGT_VER}}")
assert_code "f10.7c float max 收紧 40007" "40007" "$R"

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
assert_code "f10.9 string minLength 0->5 收紧 40007" "40007" "$R"

R=$(post "/fields/update" "{\"id\":${STGT_ID},\"label\":\"字符目标\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"minLength\":0,\"maxLength\":50}},\"version\":${STGT_VER}}")
assert_code "f10.10 string maxLength 100->50 收紧 40007" "40007" "$R"

R=$(post "/fields/update" "{\"id\":${STGT_ID},\"label\":\"字符目标\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"minLength\":0,\"maxLength\":100,\"pattern\":\"^[a-z]+$\"}},\"version\":${STGT_VER}}")
assert_code "f10.11 string 新增 pattern 收紧 40007" "40007" "$R"

# string 放宽 pattern: 删除 pattern
R=$(post "/fields/update" "{\"id\":${STGT_ID},\"label\":\"字符目标\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"minLength\":0,\"maxLength\":200}},\"version\":${STGT_VER}}")
assert_code "f10.11b string 放宽 maxLength 成功" "0" "$R"

# ---- select 收紧 ----
R=$(post "/fields/create" "{\"name\":\"${P}seltgt\",\"label\":\"选择目标\",\"type\":\"select\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"options\":[{\"value\":\"a\",\"label\":\"A\"},{\"value\":\"b\",\"label\":\"B\"},{\"value\":\"c\",\"label\":\"C\"}],\"minSelect\":1,\"maxSelect\":3}}}")
SELTGT_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$SELTGT_ID"
R=$(post "/fields/create" "{\"name\":\"${P}selholder\",\"label\":\"选持\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"refs\":[${SELTGT_ID}]}}}")
SELHOLDER_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
fld_disable "$SELTGT_ID"
SELTGT_VER=$(fld_version "$SELTGT_ID")

R=$(post "/fields/update" "{\"id\":${SELTGT_ID},\"label\":\"选择目标\",\"type\":\"select\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"options\":[{\"value\":\"a\",\"label\":\"A\"},{\"value\":\"b\",\"label\":\"B\"}],\"minSelect\":1,\"maxSelect\":2}},\"version\":${SELTGT_VER}}")
assert_code "f10.12 select 删除 option 收紧 40007" "40007" "$R"

R=$(post "/fields/update" "{\"id\":${SELTGT_ID},\"label\":\"选择目标\",\"type\":\"select\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"options\":[{\"value\":\"a\",\"label\":\"A\"},{\"value\":\"b\",\"label\":\"B\"},{\"value\":\"c\",\"label\":\"C\"}],\"minSelect\":2,\"maxSelect\":3}},\"version\":${SELTGT_VER}}")
assert_code "f10.13 select minSelect 1->2 收紧 40007" "40007" "$R"

R=$(post "/fields/update" "{\"id\":${SELTGT_ID},\"label\":\"选择目标\",\"type\":\"select\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"options\":[{\"value\":\"a\",\"label\":\"A\"},{\"value\":\"b\",\"label\":\"B\"},{\"value\":\"c\",\"label\":\"C\"}],\"minSelect\":1,\"maxSelect\":2}},\"version\":${SELTGT_VER}}")
assert_code "f10.14 select maxSelect 3->2 收紧 40007" "40007" "$R"

R=$(post "/fields/update" "{\"id\":${SELTGT_ID},\"label\":\"选择目标\",\"type\":\"select\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"options\":[{\"value\":\"a\",\"label\":\"A\"},{\"value\":\"b\",\"label\":\"B\"},{\"value\":\"c\",\"label\":\"C\"},{\"value\":\"d\",\"label\":\"D\"}],\"minSelect\":1,\"maxSelect\":3}},\"version\":${SELTGT_VER}}")
assert_code "f10.15 select 追加 option 放宽 ok" "0" "$R"

# ---- boolean 无约束 ----
R=$(post "/fields/create" "{\"name\":\"${P}btgt\",\"label\":\"布尔目标\",\"type\":\"boolean\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false}}")
BTGT_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$BTGT_ID"
R=$(post "/fields/create" "{\"name\":\"${P}bholder\",\"label\":\"布持\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"refs\":[${BTGT_ID}]}}}")
BHOLDER_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
fld_disable "$BTGT_ID"
BTGT_VER=$(fld_version "$BTGT_ID")
R=$(post "/fields/update" "{\"id\":${BTGT_ID},\"label\":\"布尔目标\",\"type\":\"boolean\",\"category\":\"basic\",\"properties\":{\"description\":\"boolean 编辑\",\"expose_bb\":false},\"version\":${BTGT_VER}}")
assert_code "f10.16 boolean 编辑 ok（无约束）" "0" "$R"

# 清理 refone
fld_rm "$REFONE_ID"
R=$(fld_detail "$TGT_ID")
assert_field "f10.17 删除引用方后 target ref_count=0" ".data.ref_count" "0" "$R"

# =============================================================================
# 11. 类型变更守卫 (40006) — 所有类型转换
# =============================================================================
subsection "1.11 REF_CHANGE_TYPE: 被引用字段改类型"

R=$(post "/fields/create" "{\"name\":\"${P}rct_tgt\",\"label\":\"改类型目标\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}}}")
RCT_TGT_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
fld_enable "$RCT_TGT_ID"

R=$(post "/fields/create" "{\"name\":\"${P}rct_holder\",\"label\":\"改类型持有\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"refs\":[${RCT_TGT_ID}]}}}")
RCT_HOLDER_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

fld_disable "$RCT_TGT_ID"
RCT_VER=$(fld_version "$RCT_TGT_ID")

R=$(post "/fields/update" "{\"id\":${RCT_TGT_ID},\"label\":\"x\",\"type\":\"float\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}},\"version\":${RCT_VER}}")
assert_code "f11.1 被引用 integer->float 40006" "40006" "$R"

R=$(post "/fields/update" "{\"id\":${RCT_TGT_ID},\"label\":\"x\",\"type\":\"boolean\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":false},\"version\":${RCT_VER}}")
assert_code "f11.2 被引用 integer->boolean 40006" "40006" "$R"

R=$(post "/fields/update" "{\"id\":${RCT_TGT_ID},\"label\":\"x\",\"type\":\"select\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"options\":[{\"value\":\"a\",\"label\":\"A\"}],\"minSelect\":1,\"maxSelect\":1}},\"version\":${RCT_VER}}")
assert_code "f11.3 被引用 integer->select 40006" "40006" "$R"

R=$(post "/fields/update" "{\"id\":${RCT_TGT_ID},\"label\":\"x\",\"type\":\"string\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"minLength\":0,\"maxLength\":100}},\"version\":${RCT_VER}}")
assert_code "f11.4 被引用 integer->string 40006" "40006" "$R"

R=$(post "/fields/update" "{\"id\":${RCT_TGT_ID},\"label\":\"x\",\"type\":\"reference\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"refs\":[${CA}]}},\"version\":${RCT_VER}}")
assert_code "f11.5 被引用 integer->reference 40006" "40006" "$R"

# 清理
fld_rm "$RCT_HOLDER_ID"
fld_rm "$RCT_TGT_ID"

# =============================================================================
# 12. 循环引用检测
# =============================================================================
subsection "1.12 循环引用检测"

# reference 嵌套引用先被 REF_NESTED (40016) 拦截
# A(int), B refs [A], C refs [B] -> 40016（已在 f9.2 测试）

# 多字段共同引用检测
R=$(post "/fields/create" "{\"name\":\"${P}cyc_x\",\"label\":\"CycX\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false}}")
CYC_X=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$CYC_X"

R=$(post "/fields/create" "{\"name\":\"${P}cyc_y\",\"label\":\"CycY\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false}}")
CYC_Y=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$CYC_Y"

R=$(post "/fields/create" "{\"name\":\"${P}cyc_z\",\"label\":\"CycZ\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"refs\":[${CYC_X},${CYC_Y}]}}}")
assert_code "f12.1 多目标引用 Z refs [X,Y] 成功" "0" "$R"
CYC_Z=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

R=$(fld_detail "$CYC_X")
assert_field "f12.2 X ref_count=1" ".data.ref_count" "1" "$R"
R=$(fld_detail "$CYC_Y")
assert_field "f12.3 Y ref_count=1" ".data.ref_count" "1" "$R"

# W refs [Z] — Z 是 reference 类型 -> 40016
R=$(post "/fields/create" "{\"name\":\"${P}cyc_w\",\"label\":\"CycW\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"refs\":[${CYC_Z}]}}}")
assert_code_in "f12.4 引用 reference 字段 40016/40013" "40016 40013" "$R"

fld_rm "$CYC_Z"
fld_disable "$CYC_X"; fld_rm "$CYC_X"
fld_disable "$CYC_Y"; fld_rm "$CYC_Y"

# =============================================================================
# 13. ATTACK: 约束自洽校验 — assert_bug()
# =============================================================================
section "Part 1d: 字段管理 — 攻击性测试"

subsection "1.13 约束自洽校验（assert_bug 探测）"

# integer: min > max
R=$(post "/fields/create" "{\"name\":\"${P}bug_int_mm\",\"label\":\"坏整数\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"min\":100,\"max\":10}}}")
assert_bug "bug.1 integer min>max 应拒绝" "40000" "$R" "字段模块不校验 constraint 自洽: integer min>max"
BUG_ID=$(echo "$R" | jq -r '.data.id // empty' | tr -d '\r')
if [ -n "$BUG_ID" ] && [ "$BUG_ID" != "null" ]; then fld_rm "$BUG_ID"; fi

# float: min > max
R=$(post "/fields/create" "{\"name\":\"${P}bug_flt_mm\",\"label\":\"坏浮点\",\"type\":\"float\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"min\":99.9,\"max\":1.1,\"precision\":2}}}")
assert_bug "bug.2 float min>max 应拒绝" "40000" "$R" "字段模块不校验 constraint 自洽: float min>max"
BUG_ID=$(echo "$R" | jq -r '.data.id // empty' | tr -d '\r')
if [ -n "$BUG_ID" ] && [ "$BUG_ID" != "null" ]; then fld_rm "$BUG_ID"; fi

# string: minLength > maxLength
R=$(post "/fields/create" "{\"name\":\"${P}bug_str_mm\",\"label\":\"坏字串\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"minLength\":100,\"maxLength\":10}}}")
assert_bug "bug.3 string minLength>maxLength 应拒绝" "40000" "$R" "字段模块不校验 constraint 自洽: string minLen>maxLen"
BUG_ID=$(echo "$R" | jq -r '.data.id // empty' | tr -d '\r')
if [ -n "$BUG_ID" ] && [ "$BUG_ID" != "null" ]; then fld_rm "$BUG_ID"; fi

# select: minSelect > maxSelect
R=$(post "/fields/create" "{\"name\":\"${P}bug_sel_mm\",\"label\":\"坏选择\",\"type\":\"select\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"options\":[{\"value\":\"a\",\"label\":\"A\"},{\"value\":\"b\",\"label\":\"B\"}],\"minSelect\":5,\"maxSelect\":1}}}")
assert_bug "bug.4 select minSelect>maxSelect 应拒绝" "40000" "$R" "字段模块不校验 constraint 自洽: select minSel>maxSel"
BUG_ID=$(echo "$R" | jq -r '.data.id // empty' | tr -d '\r')
if [ -n "$BUG_ID" ] && [ "$BUG_ID" != "null" ]; then fld_rm "$BUG_ID"; fi

# float: precision <= 0
R=$(post "/fields/create" "{\"name\":\"${P}bug_flt_p0\",\"label\":\"零精度\",\"type\":\"float\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100,\"precision\":0}}}")
assert_bug "bug.5 float precision=0 应拒绝" "40000" "$R" "字段模块不校验 constraint 自洽: float precision=0"
BUG_ID=$(echo "$R" | jq -r '.data.id // empty' | tr -d '\r')
if [ -n "$BUG_ID" ] && [ "$BUG_ID" != "null" ]; then fld_rm "$BUG_ID"; fi

# float: negative precision
R=$(post "/fields/create" "{\"name\":\"${P}bug_flt_pn\",\"label\":\"负精度\",\"type\":\"float\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100,\"precision\":-2}}}")
assert_bug "bug.6 float precision=-2 应拒绝" "40000" "$R" "字段模块不校验 constraint 自洽: float precision<0"
BUG_ID=$(echo "$R" | jq -r '.data.id // empty' | tr -d '\r')
if [ -n "$BUG_ID" ] && [ "$BUG_ID" != "null" ]; then fld_rm "$BUG_ID"; fi

# select: 空 options
R=$(post "/fields/create" "{\"name\":\"${P}bug_sel_empty\",\"label\":\"空选项\",\"type\":\"select\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"options\":[],\"minSelect\":1,\"maxSelect\":1}}}")
assert_bug "bug.7 select 空 options 应拒绝" "40000" "$R" "字段模块不校验 constraint 自洽: select 空 options"
BUG_ID=$(echo "$R" | jq -r '.data.id // empty' | tr -d '\r')
if [ -n "$BUG_ID" ] && [ "$BUG_ID" != "null" ]; then fld_rm "$BUG_ID"; fi

# select: 重复 option value
R=$(post "/fields/create" "{\"name\":\"${P}bug_sel_dup\",\"label\":\"重复选项\",\"type\":\"select\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"options\":[{\"value\":\"a\",\"label\":\"A\"},{\"value\":\"a\",\"label\":\"A2\"}],\"minSelect\":1,\"maxSelect\":1}}}")
assert_bug "bug.8 select 重复 option value 应拒绝" "40000" "$R" "字段模块不校验 constraint 自洽: select 重复 option"
BUG_ID=$(echo "$R" | jq -r '.data.id // empty' | tr -d '\r')
if [ -n "$BUG_ID" ] && [ "$BUG_ID" != "null" ]; then fld_rm "$BUG_ID"; fi

# string: negative minLength
R=$(post "/fields/create" "{\"name\":\"${P}bug_str_neg\",\"label\":\"负最小\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"minLength\":-1,\"maxLength\":50}}}")
assert_bug "bug.9 string minLength=-1 应拒绝" "40000" "$R" "字段模块不校验 constraint 自洽: string minLength<0"
BUG_ID=$(echo "$R" | jq -r '.data.id // empty' | tr -d '\r')
if [ -n "$BUG_ID" ] && [ "$BUG_ID" != "null" ]; then fld_rm "$BUG_ID"; fi

# =============================================================================
# 14. ATTACK: properties 形状
# =============================================================================
subsection "1.14 properties 形状校验"

R=$(post "/fields/create" "{\"name\":\"${P}atk_p_null\",\"label\":\"x\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":null}")
assert_code "atk.1 properties=null 40000" "40000" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}atk_p_arr\",\"label\":\"x\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":[]}")
assert_code "atk.2 properties=[] 40000" "40000" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}atk_p_num\",\"label\":\"x\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":123}")
assert_code "atk.3 properties=123 40000" "40000" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}atk_p_str\",\"label\":\"x\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":\"hello\"}")
assert_code "atk.4 properties=\"hello\" 40000" "40000" "$R"

# =============================================================================
# 15. ATTACK: SQL 注入 / XSS / LIKE 通配
# =============================================================================
subsection "1.15 SQL 注入 / XSS / LIKE 通配"

R=$(post "/fields/create" "{\"name\":\"${P}sqli\",\"label\":\"'; DROP TABLE fields; --\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{}}")
CODE=$(echo "$R" | jq -r '.code' | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$CODE" = "0" ]; then
  echo "  [PASS] atk.5 SQL-like label 被安全处理"
  PASS=$((PASS+1))
  SQLI_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
  # 确认数据正确存储
  R2=$(fld_detail "$SQLI_ID")
  assert_field "atk.5b SQL label 原样存储" ".data.label" "'; DROP TABLE fields; --" "$R2"
  fld_rm "$SQLI_ID"
else
  echo "  [FAIL] atk.5 意外 code=$CODE"; FAIL=$((FAIL+1))
fi

R=$(post "/fields/create" "{\"name\":\"${P}xss\",\"label\":\"<script>alert(1)</script>\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{}}")
CODE=$(echo "$R" | jq -r '.code' | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$CODE" = "0" ]; then
  echo "  [PASS] atk.6 XSS label 被安全处理"
  PASS=$((PASS+1))
  XSS_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
  fld_rm "$XSS_ID"
else
  echo "  [FAIL] atk.6 意外 code=$CODE"; FAIL=$((FAIL+1))
fi

# LIKE wildcard % in search
R=$(post "/fields/list" '{"label":"%","page":1,"page_size":20}')
assert_code "atk.7 LIKE % 搜索不报错" "0" "$R"

# LIKE wildcard _ in search
R=$(post "/fields/list" '{"label":"_","page":1,"page_size":20}')
assert_code "atk.8 LIKE _ 搜索不报错" "0" "$R"

# SQL injection in label search
R=$(post "/fields/list" "{\"label\":\"' OR 1=1 --\",\"page\":1,\"page_size\":20}")
assert_code "atk.9 SQL 注入搜索不报错" "0" "$R"

# =============================================================================
# 16. ATTACK: 缓存一致性
# =============================================================================
subsection "1.16 缓存一致性"

# 创建 -> 读
R=$(post "/fields/create" "{\"name\":\"${P}atk_cache\",\"label\":\"初始\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"v1\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}}}")
CACHE_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

R=$(fld_detail "$CACHE_ID")
assert_field "atk.10a 创建后立即可读" ".data.label" "初始" "$R"

# 编辑 -> 读
V=$(fld_version "$CACHE_ID")
R=$(post "/fields/update" "{\"id\":${CACHE_ID},\"label\":\"已改\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"v2\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}},\"version\":${V}}")
assert_code "atk.10b 编辑成功" "0" "$R"

R=$(fld_detail "$CACHE_ID")
assert_field "atk.10c 编辑后立即读 label=已改" ".data.label" "已改" "$R"

# 列表缓存一致性: 创建前后 total 变化
R=$(post "/fields/list" '{"label":"原子操作","page":1,"page_size":20}')
BEFORE_TOTAL=$(echo "$R" | jq -r '.data.total' | tr -d '\r')

R=$(post "/fields/create" "{\"name\":\"${P}atk_atomic\",\"label\":\"原子操作\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false}}")
ATOMIC_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

R=$(post "/fields/list" '{"label":"原子操作","page":1,"page_size":20}')
AFTER_TOTAL=$(echo "$R" | jq -r '.data.total' | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$((AFTER_TOTAL - BEFORE_TOTAL))" = "1" ]; then
  echo "  [PASS] atk.10d 创建后列表立即反映 ($BEFORE_TOTAL -> $AFTER_TOTAL)"
  PASS=$((PASS+1))
else
  echo "  [BUG ] atk.10d 列表未反映新建字段 ($BEFORE_TOTAL -> $AFTER_TOTAL)"
  FAIL=$((FAIL+1))
  BUGS+=("atk.10d: 创建字段后列表缓存未正确失效")
fi

fld_rm "$ATOMIC_ID"

# toggle-enabled -> detail 一致
V=$(fld_version "$CACHE_ID")
R=$(post "/fields/toggle-enabled" "{\"id\":${CACHE_ID},\"enabled\":true,\"version\":${V}}")
assert_code "atk.10e toggle 成功" "0" "$R"
R=$(fld_detail "$CACHE_ID")
assert_field "atk.10f toggle 后 detail enabled=true" ".data.enabled" "true" "$R"

fld_disable "$CACHE_ID"
fld_rm "$CACHE_ID"

# =============================================================================
# 17. ATTACK: 缓存穿透
# =============================================================================
subsection "1.17 缓存穿透"

for i in 1 2 3; do
  R=$(fld_detail 999999)
  CODE=$(echo "$R" | jq -r '.code' | tr -d '\r')
  TOTAL=$((TOTAL + 1))
  if [ "$CODE" = "40011" ]; then
    echo "  [PASS] atk.11.${i} 不存在 ID 第 ${i} 次返回 40011"
    PASS=$((PASS+1))
  else
    echo "  [FAIL] atk.11.${i} code=$CODE"
    FAIL=$((FAIL+1))
  fi
done

# 不同的不存在 ID
for ID_VAL in 888888 777777 666666; do
  R=$(fld_detail $ID_VAL)
  assert_code "atk.11b ID=$ID_VAL 不存在 40011" "40011" "$R"
done

# =============================================================================
# 18. ATTACK: 版本号负值 / version=0
# =============================================================================
subsection "1.18 版本号异常"

R=$(post "/fields/toggle-enabled" "{\"id\":${HP_ID},\"enabled\":false,\"version\":-1}")
assert_code "atk.12a version=-1 toggle 40000" "40000" "$R"

R=$(post "/fields/toggle-enabled" "{\"id\":${HP_ID},\"enabled\":false,\"version\":0}")
assert_code "atk.12b version=0 toggle 40000" "40000" "$R"

R=$(post "/fields/update" "{\"id\":${HP_ID},\"label\":\"x\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{},\"version\":-999}")
assert_code "atk.12c update version=-999 40000" "40000" "$R"

R=$(post "/fields/update" "{\"id\":${HP_ID},\"label\":\"x\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{},\"version\":0}")
assert_code "atk.12d update version=0 40000" "40000" "$R"

# 极大 version（不应 500）
R=$(post "/fields/toggle-enabled" "{\"id\":${HP_ID},\"enabled\":true,\"version\":2147483647}")
assert_not_500 "atk.12e 极大 version 不应 500" "$R"

# =============================================================================
# 19. ATTACK: default_value 校验探测
# =============================================================================
subsection "1.19 default_value 校验探测"

# integer default_value 为字符串
R=$(post "/fields/create" "{\"name\":\"${P}dv_int_str\",\"label\":\"x\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"default_value\":\"not_a_number\",\"constraints\":{\"min\":0,\"max\":100}}}")
assert_not_500 "atk.13a integer default_value=string 不应 500" "$R"
DV_ID=$(echo "$R" | jq -r '.data.id // empty' | tr -d '\r')
if [ -n "$DV_ID" ] && [ "$DV_ID" != "null" ]; then fld_rm "$DV_ID"; fi

# boolean default_value 为数字
R=$(post "/fields/create" "{\"name\":\"${P}dv_bool_num\",\"label\":\"x\",\"type\":\"boolean\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"default_value\":42}}")
assert_not_500 "atk.13b boolean default_value=42 不应 500" "$R"
DV_ID=$(echo "$R" | jq -r '.data.id // empty' | tr -d '\r')
if [ -n "$DV_ID" ] && [ "$DV_ID" != "null" ]; then fld_rm "$DV_ID"; fi

# integer default_value 超出范围
R=$(post "/fields/create" "{\"name\":\"${P}dv_int_oor\",\"label\":\"x\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"default_value\":999,\"constraints\":{\"min\":0,\"max\":100}}}")
assert_not_500 "atk.13c integer default_value 超范围不应 500" "$R"
DV_ID=$(echo "$R" | jq -r '.data.id // empty' | tr -d '\r')
if [ -n "$DV_ID" ] && [ "$DV_ID" != "null" ]; then fld_rm "$DV_ID"; fi

# select default_value 不在 options 中
R=$(post "/fields/create" "{\"name\":\"${P}dv_sel_bad\",\"label\":\"x\",\"type\":\"select\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"default_value\":\"nonexist\",\"constraints\":{\"options\":[{\"value\":\"a\",\"label\":\"A\"}],\"minSelect\":1,\"maxSelect\":1}}}")
assert_not_500 "atk.13d select default_value 非选项不应 500" "$R"
DV_ID=$(echo "$R" | jq -r '.data.id // empty' | tr -d '\r')
if [ -n "$DV_ID" ] && [ "$DV_ID" != "null" ]; then fld_rm "$DV_ID"; fi

# =============================================================================
# 20. ATTACK: expose_bb toggle 行为
# =============================================================================
subsection "1.20 expose_bb 行为"

# 创建 expose_bb=true 字段
R=$(post "/fields/create" "{\"name\":\"${P}bb_test\",\"label\":\"BB测试\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":true,\"constraints\":{\"min\":0,\"max\":100}}}")
assert_code "atk.14a 创建 expose_bb=true 成功" "0" "$R"
BB_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

R=$(fld_detail "$BB_ID")
assert_field "atk.14b expose_bb=true 已设置" ".data.properties.expose_bb" "true" "$R"

# 编辑关闭 expose_bb（无 FSM 引用时应成功）
V=$(fld_version "$BB_ID")
R=$(post "/fields/update" "{\"id\":${BB_ID},\"label\":\"BB测试\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}},\"version\":${V}}")
assert_code "atk.14c 无 FSM 引用时关闭 expose_bb 成功" "0" "$R"

R=$(fld_detail "$BB_ID")
assert_field "atk.14d expose_bb=false 已更新" ".data.properties.expose_bb" "false" "$R"

# 再开启
V=$(fld_version "$BB_ID")
R=$(post "/fields/update" "{\"id\":${BB_ID},\"label\":\"BB测试\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":true,\"constraints\":{\"min\":0,\"max\":100}},\"version\":${V}}")
assert_code "atk.14e 重新开启 expose_bb 成功" "0" "$R"

fld_rm "$BB_ID"

# =============================================================================
# EDIT_NOT_DISABLED 全覆盖
# =============================================================================
subsection "1.20b EDIT_NOT_DISABLED 全覆盖"

R=$(post "/fields/create" "{\"name\":\"${P}edit_guard\",\"label\":\"编辑守卫\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"guard\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}}}")
GUARD_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
fld_enable "$GUARD_ID"

GUARD_VER=$(fld_version "$GUARD_ID")
R=$(post "/fields/update" "{\"id\":${GUARD_ID},\"label\":\"改名\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"guard\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}},\"version\":${GUARD_VER}}")
assert_code "edit_guard.1 启用后编辑 label 40015" "40015" "$R"

R=$(post "/fields/update" "{\"id\":${GUARD_ID},\"label\":\"编辑守卫\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"changed\",\"expose_bb\":true,\"constraints\":{\"min\":0,\"max\":200}},\"version\":${GUARD_VER}}")
assert_code "edit_guard.2 启用后改 properties 40015" "40015" "$R"

R=$(post "/fields/update" "{\"id\":${GUARD_ID},\"label\":\"编辑守卫\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"guard\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}},\"version\":${GUARD_VER}}")
assert_code "edit_guard.3 启用后改 category 40015" "40015" "$R"

fld_disable "$GUARD_ID"
fld_rm "$GUARD_ID"

# =============================================================================
# REF_DISABLED 引用停用字段全覆盖
# =============================================================================
subsection "1.20c REF_DISABLED 全覆盖"

R=$(post "/fields/create" "{\"name\":\"${P}dis_a\",\"label\":\"停用A\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false}}")
DIS_A=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

R=$(post "/fields/create" "{\"name\":\"${P}dis_ref_a\",\"label\":\"引用停用A\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"refs\":[${DIS_A}]}}}")
assert_code "ref_dis.1 引用停用字段 40013" "40013" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}dis_b\",\"label\":\"启用B\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false}}")
DIS_B=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$DIS_B"

R=$(post "/fields/create" "{\"name\":\"${P}dis_ref_mix\",\"label\":\"混合引用\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"refs\":[${DIS_B},${DIS_A}]}}}")
assert_code "ref_dis.2 混合引用含停用 40013" "40013" "$R"

fld_rm "$DIS_A"
fld_disable "$DIS_B"; fld_rm "$DIS_B"

# =============================================================================
# 生命周期：删除后 list 不可见
# =============================================================================
subsection "1.20d 生命周期"

R=$(post "/fields/create" "{\"name\":\"${P}atk_lifecycle\",\"label\":\"生命周期\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false}}")
LIFE_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

R=$(post "/fields/list" '{"label":"生命周期","page":1,"page_size":20}')
assert_ge "atk.15a 创建后 list 可见" ".data.total" "1" "$R"

fld_rm "$LIFE_ID"

R=$(post "/fields/list" '{"label":"生命周期","page":1,"page_size":20}')
assert_field "atk.15b 删除后 list 不可见 total=0" ".data.total" "0" "$R"

# =============================================================================
# HTTP 方法攻击
# =============================================================================
subsection "1.20e HTTP 方法攻击"

# GET 请求不应被处理
R=$(raw_get "/fields/list")
assert_not_500 "atk.16a GET /fields/list 不应 500" "$R"

# PUT 请求
R=$(raw_put "/fields/create" '{"name":"put_test","label":"x","type":"integer","category":"combat","properties":{}}')
assert_not_500 "atk.16b PUT /fields/create 不应 500" "$R"

# DELETE 请求
R=$(raw_delete "/fields/create")
assert_not_500 "atk.16c DELETE /fields/create 不应 500" "$R"

# =============================================================================
# 空 body / 畸形 JSON
# =============================================================================
subsection "1.20f 空 body / 畸形 JSON"

R=$(raw_post "/fields/create" "")
assert_not_500 "atk.17a 空 body 不应 500" "$R"

R=$(raw_post "/fields/create" "{invalid json}")
assert_not_500 "atk.17b 畸形 JSON 不应 500" "$R"

R=$(raw_post "/fields/create" "null")
assert_not_500 "atk.17c body=null 不应 500" "$R"

R=$(raw_post "/fields/list" "[]")
assert_not_500 "atk.17d body=[] 不应 500" "$R"

# =============================================================================
# 并发版本冲突模拟
# =============================================================================
subsection "1.20g 并发版本冲突模拟"

R=$(post "/fields/create" "{\"name\":\"${P}conc_test\",\"label\":\"并发测试\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}}}")
CONC_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
CONC_V=$(fld_version "$CONC_ID")

# 两个"并发"更新用同一个 version
R1=$(post "/fields/update" "{\"id\":${CONC_ID},\"label\":\"并发A\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"A\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}},\"version\":${CONC_V}}")
assert_code "conc.1 第一次更新成功" "0" "$R1"

R2=$(post "/fields/update" "{\"id\":${CONC_ID},\"label\":\"并发B\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"B\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}},\"version\":${CONC_V}}")
assert_code "conc.2 第二次更新 version 冲突 40010" "40010" "$R2"

fld_rm "$CONC_ID"

# =============================================================================
# 大字段值探测
# =============================================================================
subsection "1.20h 大值探测"

# 超长 label（200 字符）
LONG_LABEL=$(printf '测%.0s' $(seq 1 200))
R=$(post "/fields/create" "{\"name\":\"${P}long_label\",\"label\":\"${LONG_LABEL}\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{}}")
assert_not_500 "atk.18a 超长 label 不应 500" "$R"
LL_ID=$(echo "$R" | jq -r '.data.id // empty' | tr -d '\r')
if [ -n "$LL_ID" ] && [ "$LL_ID" != "null" ]; then fld_rm "$LL_ID"; fi

# 超长 description
LONG_DESC=$(printf 'desc%.0s' $(seq 1 500))
R=$(post "/fields/create" "{\"name\":\"${P}long_desc\",\"label\":\"x\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"${LONG_DESC}\",\"expose_bb\":false}}")
assert_not_500 "atk.18b 超长 description 不应 500" "$R"
LD_ID=$(echo "$R" | jq -r '.data.id // empty' | tr -d '\r')
if [ -n "$LD_ID" ] && [ "$LD_ID" != "null" ]; then fld_rm "$LD_ID"; fi

# =============================================================================
# 确保导出变量可用
# =============================================================================
subsection "1.21 导出变量最终状态确认"

# 恢复 HP_ID 为停用态（后续测试可能需要）
fld_disable "$HP_ID" 2>/dev/null

# 确认关键变量都有值
TOTAL=$((TOTAL + 1))
if [ -n "$HP_ID" ] && [ -n "$ATK_ID" ] && [ -n "$MOOD_ID" ] && [ -n "$FLOAT_ID" ] && [ -n "$CA" ] && [ -n "$CB" ]; then
  echo "  [PASS] export 变量就绪: HP_ID=$HP_ID ATK_ID=$ATK_ID MOOD_ID=$MOOD_ID FLOAT_ID=$FLOAT_ID CA=$CA CB=$CB"
  PASS=$((PASS+1))
else
  echo "  [FAIL] 缺少导出变量: HP_ID=$HP_ID ATK_ID=$ATK_ID MOOD_ID=$MOOD_ID FLOAT_ID=$FLOAT_ID CA=$CA CB=$CB"
  FAIL=$((FAIL+1))
fi
