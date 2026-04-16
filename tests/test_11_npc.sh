#!/bin/bash
# =============================================================================
# NPC 管理 — 集成测试
#
# 验收标准：R1–R17（后端范围）
# =============================================================================

# ---- NPC 辅助 ----
npc_detail()  { post "/npcs/detail"        "{\"id\":$1}"; }
npc_version() { npc_detail "$1" | jq -r '.data.version' | tr -d '\r'; }
npc_enable()  { local ver=$(npc_version "$1"); post "/npcs/toggle-enabled" "{\"id\":$1,\"enabled\":true,\"version\":${ver}}"  > /dev/null; }
npc_disable() { local ver=$(npc_version "$1"); post "/npcs/toggle-enabled" "{\"id\":$1,\"enabled\":false,\"version\":${ver}}" > /dev/null; }
npc_rm()      { npc_disable "$1" 2>/dev/null; post "/npcs/delete" "{\"id\":$1}" > /dev/null 2>&1; }

bt_detail()   { post "/bt-trees/detail"       "{\"id\":$1}"; }
bt_version()  { bt_detail "$1" | jq -r '.data.version' | tr -d '\r'; }
bt_enable()   { local ver=$(bt_version "$1"); post "/bt-trees/toggle-enabled" "{\"id\":$1,\"enabled\":true,\"version\":${ver}}"  > /dev/null; }
bt_disable()  { local ver=$(bt_version "$1"); post "/bt-trees/toggle-enabled" "{\"id\":$1,\"enabled\":false,\"version\":${ver}}" > /dev/null; }
bt_rm()       { bt_disable "$1" 2>/dev/null; post "/bt-trees/delete" "{\"id\":$1}" > /dev/null 2>&1; }

section "NPC 管理 (R1-R17)"

# =============================================================================
# 环境搭建：字段 + 模板 + FSM + BT（全部启用）
# =============================================================================
subsection "测试前置数据创建"

# -- 字段 --
# display_name: string, required
FLD_DNAME_R=$(post "/fields/create" "{\"name\":\"${P}dname\",\"label\":\"显示名\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"maxLength\":64}}}")
FLD_DNAME_ID=$(echo "$FLD_DNAME_R" | jq -r '.data.id' | tr -d '\r')
fld_enable "$FLD_DNAME_ID"

# hp: integer
FLD_HP_R=$(post "/fields/create" "{\"name\":\"${P}hp\",\"label\":\"生命值\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":9999}}}")
FLD_HP_ID=$(echo "$FLD_HP_R" | jq -r '.data.id' | tr -d '\r')
fld_enable "$FLD_HP_ID"

# attack: float
FLD_ATK_R=$(post "/fields/create" "{\"name\":\"${P}atk\",\"label\":\"攻击力\",\"type\":\"float\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{}}}")
FLD_ATK_ID=$(echo "$FLD_ATK_R" | jq -r '.data.id' | tr -d '\r')
fld_enable "$FLD_ATK_ID"

echo "  字段 IDs: dname=${FLD_DNAME_ID}, hp=${FLD_HP_ID}, atk=${FLD_ATK_ID}"

# -- 模板：dname(required=true), hp(required=false), atk(required=false) --
TPL_R=$(post "/templates/create" "{\"name\":\"${P}creature\",\"label\":\"生物模板\",\"description\":\"\",\"fields\":[{\"field_id\":${FLD_DNAME_ID},\"required\":true},{\"field_id\":${FLD_HP_ID},\"required\":false},{\"field_id\":${FLD_ATK_ID},\"required\":false}]}")
TPL_ID=$(echo "$TPL_R" | jq -r '.data.id' | tr -d '\r')
tpl_enable "$TPL_ID"
echo "  模板 ID: ${TPL_ID}"

# -- FSM: states=[idle, chase] --
FSM_R=$(post "/fsm-configs/create" "{\"name\":\"${P}wolf_fsm\",\"display_name\":\"狼FSM\",\"initial_state\":\"idle\",\"states\":[{\"name\":\"idle\",\"display_name\":\"空闲\"},{\"name\":\"chase\",\"display_name\":\"追逐\"}],\"transitions\":[{\"from\":\"idle\",\"to\":\"chase\",\"priority\":1,\"condition\":{\"type\":\"always\"}}]}")
FSM_ID=$(echo "$FSM_R" | jq -r '.data.id' | tr -d '\r')
fsm_enable "$FSM_ID"
echo "  FSM ID: ${FSM_ID}"

# -- BT 树: idle, chase（使用 stub_action 作为叶子节点使配置合法） --
BT_IDLE_R=$(post "/bt-trees/create" "{\"name\":\"${P}wolf/idle\",\"display_name\":\"狼空闲BT\",\"description\":\"\",\"config\":{\"type\":\"sequence\",\"children\":[{\"type\":\"stub_action\"}]}}")
BT_IDLE_ID=$(echo "$BT_IDLE_R" | jq -r '.data.id' | tr -d '\r')
bt_enable "$BT_IDLE_ID"

BT_CHASE_R=$(post "/bt-trees/create" "{\"name\":\"${P}wolf/chase\",\"display_name\":\"狼追逐BT\",\"description\":\"\",\"config\":{\"type\":\"sequence\",\"children\":[{\"type\":\"stub_action\"}]}}")
BT_CHASE_ID=$(echo "$BT_CHASE_R" | jq -r '.data.id' | tr -d '\r')
bt_enable "$BT_CHASE_ID"

echo "  BT idle ID: ${BT_IDLE_ID}, chase ID: ${BT_CHASE_ID}"

# 校验前置数据
if [ -z "$FLD_DNAME_ID" ] || [ "$FLD_DNAME_ID" = "null" ] ||
   [ -z "$TPL_ID" ]       || [ "$TPL_ID" = "null" ]       ||
   [ -z "$FSM_ID" ]       || [ "$FSM_ID" = "null" ]       ||
   [ -z "$BT_IDLE_ID" ]   || [ "$BT_IDLE_ID" = "null" ]; then
  echo "  [FATAL] 前置数据创建失败，终止测试"
  return 1 2>/dev/null || exit 1
fi

# =============================================================================
# R7 — 名称校验
# =============================================================================
subsection "R7: check-name"

R=$(post "/npcs/check-name" '{"name":""}')
assert_code "R7a: 空 name → 45002" "45002" "$R"

R=$(post "/npcs/check-name" '{"name":"INVALID NAME"}')
assert_code "R7b: 含空格 name → 45002" "45002" "$R"

R=$(post "/npcs/check-name" "{\"name\":\"${P}wolf_common\"}")
assert_code "R7c: 新 name → 0" "0" "$R"
assert_field "R7d: available=true" '.data.available' 'true' "$R"

# =============================================================================
# R2 — NPC 创建
# =============================================================================
subsection "R2: 创建 NPC"

NPC_R=$(post "/npcs/create" "{\"name\":\"${P}wolf_common\",\"label\":\"普通灰狼\",\"description\":\"测试\",\"template_id\":${TPL_ID},\"field_values\":[{\"field_id\":${FLD_DNAME_ID},\"value\":\"普通灰狼\"},{\"field_id\":${FLD_HP_ID},\"value\":100},{\"field_id\":${FLD_ATK_ID},\"value\":15.5}],\"fsm_ref\":\"${P}wolf_fsm\",\"bt_refs\":{\"idle\":\"${P}wolf/idle\",\"chase\":\"${P}wolf/chase\"}}")
NPC_ID=$(echo "$NPC_R" | jq -r '.data.id' | tr -d '\r')

assert_code "R2a: 创建成功 → 0" "0" "$NPC_R"
assert_exists "R2b: 返回 id" '.data.id' "$NPC_R"
assert_field  "R2c: 返回 name" '.data.name' "${P}wolf_common" "$NPC_R"
echo "  NPC ID: ${NPC_ID}"

# =============================================================================
# R7 follow-up — 已有 name
# =============================================================================
R=$(post "/npcs/check-name" "{\"name\":\"${P}wolf_common\"}")
assert_code  "R7e: 已存在 name → 0" "0" "$R"
assert_field "R7f: available=false" '.data.available' 'false' "$R"

# =============================================================================
# R1 — 列表分页
# =============================================================================
subsection "R1: 列表分页"

R=$(post "/npcs/list" '{}')
assert_code "R1a: list code=0" "0" "$R"
assert_ge   "R1b: total >= 1" '.data.total' "1" "$R"
assert_exists "R1c: page 存在" '.data.page' "$R"
assert_exists "R1d: page_size 存在" '.data.page_size' "$R"

R=$(post "/npcs/list" '{"label":"普通"}')
assert_ge "R1e: label 模糊筛选有结果" '.data.total' "1" "$R"

R=$(post "/npcs/list" "{\"template_name\":\"${P}creature\"}")
assert_ge "R1f: template_name 精确筛选有结果" '.data.total' "1" "$R"

R=$(post "/npcs/list" '{"name":"wolf"}')
assert_code "R1g: name 模糊筛选 → 0" "0" "$R"

# =============================================================================
# R3 — 详情
# =============================================================================
subsection "R3: 详情"

D=$(npc_detail "$NPC_ID")
assert_code  "R3a: 详情 code=0" "0" "$D"
assert_field "R3b: name 正确" '.data.name' "${P}wolf_common" "$D"
assert_field "R3c: label 正确" '.data.label' "普通灰狼" "$D"
assert_exists "R3d: fields 非空" '.data.fields' "$D"
assert_field  "R3e: fsm_ref 正确" '.data.fsm_ref' "${P}wolf_fsm" "$D"
assert_exists "R3f: bt_refs 非空" '.data.bt_refs' "$D"
assert_field  "R3g: enabled=true（默认启用）" '.data.enabled' 'true' "$D"

# =============================================================================
# R6 — toggle-enabled
# =============================================================================
subsection "R6: toggle-enabled"

VER=$(npc_version "$NPC_ID")

R=$(post "/npcs/toggle-enabled" "{\"id\":${NPC_ID},\"enabled\":false,\"version\":${VER}}")
assert_code "R6a: 停用 → 0" "0" "$R"

# 旧 version 冲突 → 45014
R=$(post "/npcs/toggle-enabled" "{\"id\":${NPC_ID},\"enabled\":true,\"version\":${VER}}")
assert_code "R6b: 乐观锁冲突 → 45014" "45014" "$R"

npc_enable "$NPC_ID"

# =============================================================================
# R4 — 编辑（无需先停用）
# =============================================================================
subsection "R4: 编辑"

VER=$(npc_version "$NPC_ID")
R=$(post "/npcs/update" "{\"id\":${NPC_ID},\"version\":${VER},\"label\":\"灰狼v2\",\"description\":\"已更新\",\"field_values\":[{\"field_id\":${FLD_DNAME_ID},\"value\":\"灰狼v2\"},{\"field_id\":${FLD_HP_ID},\"value\":200}],\"fsm_ref\":\"${P}wolf_fsm\",\"bt_refs\":{\"idle\":\"${P}wolf/idle\",\"chase\":\"${P}wolf/chase\"}}")
assert_code "R4a: 编辑成功 → 0" "0" "$R"

D=$(npc_detail "$NPC_ID")
assert_field "R4b: label 已更新" '.data.label' "灰狼v2" "$D"

# 旧 version 冲突
R=$(post "/npcs/update" "{\"id\":${NPC_ID},\"version\":${VER},\"label\":\"x\",\"description\":\"\",\"field_values\":[{\"field_id\":${FLD_DNAME_ID},\"value\":\"x\"}],\"fsm_ref\":\"\",\"bt_refs\":{}}")
assert_code "R4c: 编辑乐观锁冲突 → 45014" "45014" "$R"

# =============================================================================
# R5 — 删除前置停用
# =============================================================================
subsection "R5: 删除前置停用"

# 启用中删除 → 45013
R=$(post "/npcs/delete" "{\"id\":${NPC_ID}}")
assert_code "R5a: 启用中删除 → 45013" "45013" "$R"

# 停用后删除 → 0
npc_disable "$NPC_ID"
R=$(post "/npcs/delete" "{\"id\":${NPC_ID}}")
assert_code "R5b: 停用后删除 → 0" "0" "$R"

# 删除后 detail → 45003
R=$(npc_detail "$NPC_ID")
assert_code "R5c: 删除后 detail → 45003" "45003" "$R"

# =============================================================================
# R8 — 必填字段校验
# =============================================================================
subsection "R8: 必填字段"

# dname required=true，传 null
R=$(post "/npcs/create" "{\"name\":\"${P}req_test\",\"label\":\"必填测试\",\"description\":\"\",\"template_id\":${TPL_ID},\"field_values\":[{\"field_id\":${FLD_DNAME_ID},\"value\":null},{\"field_id\":${FLD_HP_ID},\"value\":50}],\"fsm_ref\":\"\",\"bt_refs\":{}}")
assert_code "R8a: 必填字段 null → 45007" "45007" "$R"

# =============================================================================
# R9 — 字段值类型约束
# =============================================================================
subsection "R9: 字段值类型约束"

# integer 字段传浮点
R=$(post "/npcs/create" "{\"name\":\"${P}type_test\",\"label\":\"类型测试\",\"description\":\"\",\"template_id\":${TPL_ID},\"field_values\":[{\"field_id\":${FLD_DNAME_ID},\"value\":\"x\"},{\"field_id\":${FLD_HP_ID},\"value\":3.14}],\"fsm_ref\":\"\",\"bt_refs\":{}}")
assert_code "R9a: integer 传浮点 → 45006" "45006" "$R"

# integer 传字符串
R=$(post "/npcs/create" "{\"name\":\"${P}type_test2\",\"label\":\"类型测试2\",\"description\":\"\",\"template_id\":${TPL_ID},\"field_values\":[{\"field_id\":${FLD_DNAME_ID},\"value\":\"x\"},{\"field_id\":${FLD_HP_ID},\"value\":\"notnum\"}],\"fsm_ref\":\"\",\"bt_refs\":{}}")
assert_code "R9b: integer 传字符串 → 45006" "45006" "$R"

# string 超 max_length (64)
LONG_STR=$(printf 'x%.0s' {1..70})
R=$(post "/npcs/create" "{\"name\":\"${P}str_test\",\"label\":\"超长字符串\",\"description\":\"\",\"template_id\":${TPL_ID},\"field_values\":[{\"field_id\":${FLD_DNAME_ID},\"value\":\"${LONG_STR}\"}],\"fsm_ref\":\"\",\"bt_refs\":{}}")
assert_code "R9c: string 超 max_length → 45006" "45006" "$R"

# =============================================================================
# R10 — 行为配置校验
# =============================================================================
subsection "R10: 行为配置校验"

# bt_refs 非空但 fsm_ref 为空 → 45015
R=$(post "/npcs/create" "{\"name\":\"${P}bt_nofsm\",\"label\":\"无FSM\",\"description\":\"\",\"template_id\":${TPL_ID},\"field_values\":[{\"field_id\":${FLD_DNAME_ID},\"value\":\"x\"}],\"fsm_ref\":\"\",\"bt_refs\":{\"idle\":\"${P}wolf/idle\"}}")
assert_code "R10a: bt_refs 非空 fsm_ref 空 → 45015" "45015" "$R"

# fsm_ref 不存在 → 45008
R=$(post "/npcs/create" "{\"name\":\"${P}badfsm\",\"label\":\"坏FSM\",\"description\":\"\",\"template_id\":${TPL_ID},\"field_values\":[{\"field_id\":${FLD_DNAME_ID},\"value\":\"x\"}],\"fsm_ref\":\"nonexistent_fsm\",\"bt_refs\":{}}")
assert_code "R10b: fsm_ref 不存在 → 45008" "45008" "$R"

# bt_refs state 不在 FSM 状态列表中 → 45012
R=$(post "/npcs/create" "{\"name\":\"${P}badstate\",\"label\":\"坏状态\",\"description\":\"\",\"template_id\":${TPL_ID},\"field_values\":[{\"field_id\":${FLD_DNAME_ID},\"value\":\"x\"}],\"fsm_ref\":\"${P}wolf_fsm\",\"bt_refs\":{\"INVALID_STATE\":\"${P}wolf/idle\"}}")
assert_code "R10c: bt_refs state 不在 FSM 中 → 45012" "45012" "$R"

# bt_refs 中 BT 树不存在 → 45010 or 45011
R=$(post "/npcs/create" "{\"name\":\"${P}badbt\",\"label\":\"坏BT\",\"description\":\"\",\"template_id\":${TPL_ID},\"field_values\":[{\"field_id\":${FLD_DNAME_ID},\"value\":\"x\"}],\"fsm_ref\":\"${P}wolf_fsm\",\"bt_refs\":{\"idle\":\"nonexistent_bt_tree\"}}")
assert_code_in "R10d: bt_refs BT 不存在 → 45010/45011" "45010 45011" "$R"

# =============================================================================
# R11 — 模板停用校验
# =============================================================================
subsection "R11: 模板停用"

# 停用模板
TPL2_R=$(post "/templates/create" "{\"name\":\"${P}disabled_tpl\",\"label\":\"停用模板\",\"description\":\"\",\"fields\":[{\"field_id\":${FLD_DNAME_ID},\"required\":false}]}")
TPL2_ID=$(echo "$TPL2_R" | jq -r '.data.id' | tr -d '\r')
# 不启用，默认 enabled=false

R=$(post "/npcs/create" "{\"name\":\"${P}npc_dis_tpl\",\"label\":\"停用模板NPC\",\"description\":\"\",\"template_id\":${TPL2_ID},\"field_values\":[],\"fsm_ref\":\"\",\"bt_refs\":{}}")
assert_code "R11a: 模板未启用 → 45005" "45005" "$R"

R=$(post "/npcs/create" "{\"name\":\"${P}npc_notpl\",\"label\":\"无模板\",\"description\":\"\",\"template_id\":99999,\"field_values\":[],\"fsm_ref\":\"\",\"bt_refs\":{}}")
assert_code "R11b: 模板不存在 → 45004" "45004" "$R"

# =============================================================================
# 重建 NPC 用于跨模块引用测试 (R12-R16)
# =============================================================================
subsection "R12-R16: 跨模块引用 - 准备"

NPC2_R=$(post "/npcs/create" "{\"name\":\"${P}wolf_ref\",\"label\":\"引用狼\",\"description\":\"\",\"template_id\":${TPL_ID},\"field_values\":[{\"field_id\":${FLD_DNAME_ID},\"value\":\"引用狼\"},{\"field_id\":${FLD_HP_ID},\"value\":100}],\"fsm_ref\":\"${P}wolf_fsm\",\"bt_refs\":{\"idle\":\"${P}wolf/idle\"}}")
NPC2_ID=$(echo "$NPC2_R" | jq -r '.data.id' | tr -d '\r')
assert_code "引用 NPC 创建成功" "0" "$NPC2_R"
echo "  引用 NPC ID: ${NPC2_ID}"

# =============================================================================
# R12 — 模板删除引用检查
# =============================================================================
subsection "R12: 模板删除 → 41007"

tpl_disable "$TPL_ID"
R=$(post "/templates/delete" "{\"id\":${TPL_ID}}")
assert_code "R12a: 被 NPC 引用的模板删除 → 41007" "41007" "$R"
tpl_enable "$TPL_ID"

# =============================================================================
# R13 — 模板字段编辑引用检查
# =============================================================================
subsection "R13: 模板字段编辑 → 41008"

TPL_VER=$(tpl_version "$TPL_ID")
R=$(post "/templates/update" "{\"id\":${TPL_ID},\"version\":${TPL_VER},\"label\":\"生物模板2\",\"description\":\"\",\"fields\":[{\"field_id\":${FLD_DNAME_ID},\"required\":false},{\"field_id\":${FLD_HP_ID},\"required\":true}]}")
assert_code "R13a: 字段 required 变更 → 41008" "41008" "$R"

# 只改 label（fields 不变）→ 应成功（模板需先禁用才可编辑）
tpl_disable "$TPL_ID"
TPL_VER=$(tpl_version "$TPL_ID")
R=$(post "/templates/update" "{\"id\":${TPL_ID},\"version\":${TPL_VER},\"label\":\"生物模板\",\"description\":\"\",\"fields\":[{\"field_id\":${FLD_DNAME_ID},\"required\":true},{\"field_id\":${FLD_HP_ID},\"required\":false},{\"field_id\":${FLD_ATK_ID},\"required\":false}]}")
assert_code "R13b: 只改 label fields 不变 → 0" "0" "$R"
tpl_enable "$TPL_ID"

# =============================================================================
# R14 — BT 树删除引用检查
# =============================================================================
subsection "R14: BT 树删除 → 44012"

bt_disable "$BT_IDLE_ID"
R=$(post "/bt-trees/delete" "{\"id\":${BT_IDLE_ID}}")
assert_code "R14a: 被 NPC 引用 BT 树删除 → 44012" "44012" "$R"
bt_enable "$BT_IDLE_ID"

# =============================================================================
# R15 — FSM 删除引用检查
# =============================================================================
subsection "R15: FSM 删除 → 43012"

fsm_disable "$FSM_ID"
R=$(post "/fsm-configs/delete" "{\"id\":${FSM_ID}}")
assert_code "R15a: 被 NPC 引用 FSM 删除 → 43012" "43012" "$R"
fsm_enable "$FSM_ID"

# =============================================================================
# R16 — 模板引用详情填充
# =============================================================================
subsection "R16: 模板引用详情"

R=$(post "/templates/references" "{\"id\":${TPL_ID}}")
assert_code "R16a: references code=0" "0" "$R"
assert_ge   "R16b: npcs 数组 >= 1" '.data.npcs | length' "1" "$R"

# =============================================================================
# R17 — 导出格式
# =============================================================================
subsection "R17: 导出格式"

EXP=$(get_export "/npc_templates")
# 导出接口直接返回 {"items":[...]}，没有 code 字段
assert_exists "R17a: items 字段存在" '.items' "$EXP"
assert_ge     "R17b: items 有数据" '.items | length' "1" "$EXP"

ITEM=$(echo "$EXP" | jq -c --arg n "${P}wolf_ref" '.items[] | select(.name==$n)' 2>/dev/null | tr -d '\r')
assert_exists "R17c: wolf_ref 在导出中" '.' "$ITEM"

assert_field  "R17d: template_ref 正确" '.config.template_ref' "${P}creature" "$ITEM"
assert_ge     "R17e: fields 有内容" '.config.fields | keys | length' "1" "$ITEM"
assert_field  "R17f: behavior.fsm_ref 正确" '.config.behavior.fsm_ref' "${P}wolf_fsm" "$ITEM"
assert_exists "R17g: behavior.bt_refs 存在" '.config.behavior.bt_refs' "$ITEM"

# NPC 无 behavior 时 behavior = {}（不省略 behavior key）
NPC3_R=$(post "/npcs/create" "{\"name\":\"${P}no_behavior\",\"label\":\"无行为\",\"description\":\"\",\"template_id\":${TPL_ID},\"field_values\":[{\"field_id\":${FLD_DNAME_ID},\"value\":\"无行为NPC\"}],\"fsm_ref\":\"\",\"bt_refs\":{}}")
NPC3_ID=$(echo "$NPC3_R" | jq -r '.data.id' | tr -d '\r')

EXP2=$(get_export "/npc_templates")
ITEM3=$(echo "$EXP2" | jq -c --arg n "${P}no_behavior" '.items[] | select(.name==$n)' 2>/dev/null | tr -d '\r')
assert_exists "R17h: 无行为 NPC 在导出中" '.' "$ITEM3"

BEHAVIOR_VAL=$(echo "$ITEM3" | jq -r '.config.behavior' 2>/dev/null | tr -d '\r')
assert_not_equal "R17i: behavior 键不省略" '.config.behavior' "null" "$ITEM3"

# behavior 为 {} 时，fsm_ref/bt_refs key 不存在
FSM_KEY=$(echo "$ITEM3" | jq -r '.config.behavior | keys | length' 2>/dev/null | tr -d '\r')
assert_field "R17j: 空 behavior 没有多余 key" '.config.behavior | keys | length' "0" "$ITEM3"

# =============================================================================
# 攻击性测试
# =============================================================================
subsection "攻击性测试"

# 重复 name
R=$(post "/npcs/create" "{\"name\":\"${P}wolf_ref\",\"label\":\"重复\",\"description\":\"\",\"template_id\":${TPL_ID},\"field_values\":[{\"field_id\":${FLD_DNAME_ID},\"value\":\"x\"}],\"fsm_ref\":\"\",\"bt_refs\":{}}")
assert_code "ATK1: 重复 name → 45001" "45001" "$R"

# template_id = 0
R=$(post "/npcs/create" "{\"name\":\"${P}atk2\",\"label\":\"atk2\",\"description\":\"\",\"template_id\":0,\"field_values\":[],\"fsm_ref\":\"\",\"bt_refs\":{}}")
assert_not_500 "ATK2: template_id=0 不是 500" "$R"

# 不存在的 NPC
R=$(npc_detail "9999999")
assert_code "ATK3: 不存在 NPC → 45003" "45003" "$R"

# id=0
R=$(post "/npcs/detail" '{"id":0}')
assert_not_500 "ATK4: id=0 detail 不 panic" "$R"

# 删除启用的 FSM（无 NPC 引用时应成功）—— 先创建独立 FSM
FSM2_R=$(post "/fsm-configs/create" "{\"name\":\"${P}lonely_fsm\",\"display_name\":\"独立FSM\",\"initial_state\":\"a\",\"states\":[{\"name\":\"a\"}],\"transitions\":[]}")
FSM2_ID=$(echo "$FSM2_R" | jq -r '.data.id' | tr -d '\r')
fsm_disable "$FSM2_ID"
R=$(post "/fsm-configs/delete" "{\"id\":${FSM2_ID}}")
assert_code "ATK5: 无 NPC 引用的 FSM 可以删除 → 0" "0" "$R"

# BT 树无引用时可以删除
BT3_R=$(post "/bt-trees/create" "{\"name\":\"${P}lonely_bt\",\"display_name\":\"独立BT\",\"description\":\"\",\"config\":{\"type\":\"stub_action\"}}")
BT3_ID=$(echo "$BT3_R" | jq -r '.data.id' | tr -d '\r')
bt_disable "$BT3_ID"
R=$(post "/bt-trees/delete" "{\"id\":${BT3_ID}}")
assert_code "ATK6: 无 NPC 引用的 BT 树可以删除 → 0" "0" "$R"

# =============================================================================
# 清理
# =============================================================================
subsection "清理测试数据"
npc_rm "$NPC2_ID"  2>/dev/null
npc_rm "$NPC3_ID"  2>/dev/null
post "/templates/delete" "{\"id\":${TPL2_ID}}" > /dev/null 2>&1
echo "  清理完成（保留字段/模板/FSM/BT 供后续测试使用）"
