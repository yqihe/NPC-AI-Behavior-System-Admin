#!/bin/bash
# =============================================================================
# test_04_event_type.sh — 事件类型 CRUD + 系统字段校验 + 攻击性测试
#
# 前置：run_all.sh 已 source helpers.sh，$BASE / $P / assert_* / post() 可用
#       test_05 创建的 schema 不在此文件依赖中（本文件不用扩展字段）
#
# 导出变量：ET_ID1, ET_ID2, ET_ID3, ET_ID4（供 test_06, test_08 使用）
# =============================================================================

section "Part 4: 事件类型 CRUD + 系统字段校验 (prefix=$P)"

# =============================================================================
# 1. 三种感知模式创建
# =============================================================================
subsection "1. 三种感知模式创建"

# 1.1 visual
body=$(post "/event-types/create" "{\"name\":\"${P}visual_evt\",\"display_name\":\"视觉事件\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":10,\"range\":200}")
assert_code "1.1 创建 visual 事件" "0" "$body"
ET_ID1=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 1.2 auditory
body=$(post "/event-types/create" "{\"name\":\"${P}auditory_evt\",\"display_name\":\"听觉事件\",\"perception_mode\":\"auditory\",\"default_severity\":75,\"default_ttl\":15,\"range\":300}")
assert_code "1.2 创建 auditory 事件" "0" "$body"
ET_ID2=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 1.3 global — 客户端传 range=999，后端应强制为 0
body=$(post "/event-types/create" "{\"name\":\"${P}global_evt\",\"display_name\":\"全局事件\",\"perception_mode\":\"global\",\"default_severity\":95,\"default_ttl\":30,\"range\":999}")
assert_code "1.3 创建 global 事件" "0" "$body"
ET_ID3=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 1.4 验证 global range 被强制置 0
body=$(et_detail "$ET_ID3")
assert_code  "1.4 global 详情成功" "0" "$body"
assert_field "1.4 global range=0（自动修正）" '.data.config.range' "0" "$body"
assert_field "1.4 severity=95" '.data.config.default_severity' "95" "$body"
assert_field "1.4 perception_mode=global" '.data.config.perception_mode' "global" "$body"

# =============================================================================
# 2. severity 边界 — SEVERITY_INVALID (42004)
# =============================================================================
subsection "2. severity 边界"

# 2.1 severity=0 合法
body=$(post "/event-types/create" "{\"name\":\"${P}sev_zero\",\"display_name\":\"零威胁\",\"perception_mode\":\"visual\",\"default_severity\":0,\"default_ttl\":5,\"range\":100}")
assert_code "2.1 severity=0 合法" "0" "$body"
SEV0_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')
body=$(et_detail "$SEV0_ID")
assert_field "2.1 config.default_severity=0" '.data.config.default_severity' "0" "$body"

# 2.2 severity=100 合法
body=$(post "/event-types/create" "{\"name\":\"${P}sev_hundred\",\"display_name\":\"满威胁\",\"perception_mode\":\"visual\",\"default_severity\":100,\"default_ttl\":5,\"range\":100}")
assert_code "2.2 severity=100 合法" "0" "$body"

# 2.3 severity=-1 拒绝 → SEVERITY_INVALID
body=$(post "/event-types/create" "{\"name\":\"${P}sev_neg\",\"display_name\":\"负威胁\",\"perception_mode\":\"visual\",\"default_severity\":-1,\"default_ttl\":5,\"range\":100}")
assert_code "2.3 severity=-1 → SEVERITY_INVALID 42004" "42004" "$body"

# 2.4 severity=101 拒绝 → SEVERITY_INVALID
body=$(post "/event-types/create" "{\"name\":\"${P}sev_over\",\"display_name\":\"超威胁\",\"perception_mode\":\"visual\",\"default_severity\":101,\"default_ttl\":5,\"range\":100}")
assert_code "2.4 severity=101 → SEVERITY_INVALID 42004" "42004" "$body"

# 2.5 severity=-100 边界攻击
body=$(post "/event-types/create" "{\"name\":\"${P}sev_neg100\",\"display_name\":\"极负威胁\",\"perception_mode\":\"visual\",\"default_severity\":-100,\"default_ttl\":5,\"range\":100}")
assert_code "2.5 severity=-100 → SEVERITY_INVALID 42004" "42004" "$body"

# 2.6 severity=999 边界攻击
body=$(post "/event-types/create" "{\"name\":\"${P}sev_999\",\"display_name\":\"超大威胁\",\"perception_mode\":\"visual\",\"default_severity\":999,\"default_ttl\":5,\"range\":100}")
assert_code "2.6 severity=999 → SEVERITY_INVALID 42004" "42004" "$body"

# =============================================================================
# 3. TTL 边界 — TTL_INVALID (42005)
# =============================================================================
subsection "3. TTL 边界"

# 3.1 ttl=0.1 合法
body=$(post "/event-types/create" "{\"name\":\"${P}ttl_small\",\"display_name\":\"小TTL\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":0.1,\"range\":100}")
assert_code "3.1 ttl=0.1 合法" "0" "$body"

# 3.2 ttl=0 拒绝 → TTL_INVALID
body=$(post "/event-types/create" "{\"name\":\"${P}ttl_zero\",\"display_name\":\"零TTL\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":0,\"range\":100}")
assert_code "3.2 ttl=0 → TTL_INVALID 42005" "42005" "$body"

# 3.3 ttl=-1 拒绝 → TTL_INVALID
body=$(post "/event-types/create" "{\"name\":\"${P}ttl_neg\",\"display_name\":\"负TTL\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":-1,\"range\":100}")
assert_code "3.3 ttl=-1 → TTL_INVALID 42005" "42005" "$body"

# 3.4 ttl=-100 极端负值
body=$(post "/event-types/create" "{\"name\":\"${P}ttl_neg100\",\"display_name\":\"极负TTL\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":-100,\"range\":100}")
assert_code "3.4 ttl=-100 → TTL_INVALID 42005" "42005" "$body"

# =============================================================================
# 4. range 边界 — RANGE_INVALID (42006)
# =============================================================================
subsection "4. range 边界"

# 4.1 range=0 合法（非 global 也可以）
body=$(post "/event-types/create" "{\"name\":\"${P}range_zero\",\"display_name\":\"零范围\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":0}")
assert_code "4.1 range=0 合法" "0" "$body"

# 4.2 range=-1 拒绝 → RANGE_INVALID
body=$(post "/event-types/create" "{\"name\":\"${P}range_neg\",\"display_name\":\"负范围\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":-1}")
assert_code "4.2 range=-1 → RANGE_INVALID 42006" "42006" "$body"

# 4.3 range=-999 极端负值
body=$(post "/event-types/create" "{\"name\":\"${P}range_neg999\",\"display_name\":\"极负范围\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":-999}")
assert_code "4.3 range=-999 → RANGE_INVALID 42006" "42006" "$body"

# =============================================================================
# 5. name 校验
# =============================================================================
subsection "5. name 校验"

# 5.1 空 name
body=$(post "/event-types/create" "{\"name\":\"\",\"display_name\":\"空名\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "5.1 空 name → 42002" "42002" "$body"

# 5.2 大写 name
body=$(post "/event-types/create" "{\"name\":\"${P}BadCase\",\"display_name\":\"大写\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "5.2 大写 name → 42002" "42002" "$body"

# 5.3 重复 name
body=$(post "/event-types/create" "{\"name\":\"${P}visual_evt\",\"display_name\":\"重复\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "5.3 重复 name → 42001" "42001" "$body"

# =============================================================================
# 6. 详情 + 列表 + 筛选
# =============================================================================
subsection "6. 详情 + 列表 + 筛选"

# 6.1 详情
body=$(et_detail "$ET_ID1")
assert_code  "6.1 详情成功" "0" "$body"
assert_field "6.1 name 正确" '.data.name' "${P}visual_evt" "$body"
assert_field "6.1 perception_mode" '.data.perception_mode' "visual" "$body"

# 6.2 列表
body=$(post "/event-types/list" "{\"page\":1,\"page_size\":20}")
assert_code "6.2 列表成功" "0" "$body"
assert_ge   "6.2 total >= 3" '.data.total' "3" "$body"

# 6.3 perception_mode 筛选 — global
body=$(post "/event-types/list" "{\"perception_mode\":\"global\",\"page\":1,\"page_size\":20}")
assert_code "6.3 列表 global 筛选" "0" "$body"
G_COUNT=$(echo "$body" | jq -r '.data.total' | tr -d '\r')
assert_ge "6.3 global 筛选 >= 1" '.data.total' "1" "$body"

# 6.4 perception_mode 筛选 — visual
body=$(post "/event-types/list" "{\"perception_mode\":\"visual\",\"page\":1,\"page_size\":20}")
assert_code "6.4 列表 visual 筛选" "0" "$body"
assert_ge   "6.4 visual 筛选 >= 1" '.data.total' "1" "$body"

# =============================================================================
# 7. 编辑 + 启用守卫 + 版本冲突
# =============================================================================
subsection "7. 编辑 + 启用守卫 + 版本冲突"

# 7.1 编辑成功（默认 enabled=false）
V=$(et_version "$ET_ID1")
body=$(post "/event-types/update" "{\"id\":$ET_ID1,\"display_name\":\"视觉事件(改)\",\"perception_mode\":\"visual\",\"default_severity\":60,\"default_ttl\":12,\"range\":220,\"version\":$V}")
assert_code "7.1 编辑成功" "0" "$body"

# 7.2 验证编辑结果
body=$(et_detail "$ET_ID1")
assert_field "7.2 severity=60" '.data.config.default_severity' "60" "$body"

# 7.3 启用
V=$(et_version "$ET_ID1")
body=$(post "/event-types/toggle-enabled" "{\"id\":$ET_ID1,\"enabled\":true,\"version\":$V}")
assert_code "7.3 启用" "0" "$body"

# 7.4 EDIT_NOT_DISABLED: 启用后编辑 → 42015
V=$(et_version "$ET_ID1")
body=$(post "/event-types/update" "{\"id\":$ET_ID1,\"display_name\":\"视觉事件(再改)\",\"perception_mode\":\"visual\",\"default_severity\":60,\"default_ttl\":12,\"range\":220,\"version\":$V}")
assert_code "7.4 启用后编辑 → EDIT_NOT_DISABLED 42015" "42015" "$body"

# 7.5 版本冲突（用错误 version）— 先停用
V=$(et_version "$ET_ID1")
post "/event-types/toggle-enabled" "{\"id\":$ET_ID1,\"enabled\":false,\"version\":$V}" > /dev/null
body=$(post "/event-types/update" "{\"id\":$ET_ID1,\"display_name\":\"冲突测试\",\"perception_mode\":\"visual\",\"default_severity\":60,\"default_ttl\":12,\"range\":220,\"version\":999}")
assert_code "7.5 版本冲突 → 42010" "42010" "$body"

# =============================================================================
# 7b. EDIT_NOT_DISABLED 全覆盖
# =============================================================================
subsection "7b. EDIT_NOT_DISABLED 全覆盖"

# 创建专用事件用于 EDIT_NOT_DISABLED 测试
body=$(post "/event-types/create" "{\"name\":\"${P}edit_guard_evt\",\"display_name\":\"编辑守卫事件\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":10,\"range\":100}")
assert_code "7b.0 创建编辑守卫事件" "0" "$body"
EG_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 启用
V=$(et_version "$EG_ID")
post "/event-types/toggle-enabled" "{\"id\":$EG_ID,\"enabled\":true,\"version\":$V}" > /dev/null

# 7b.1 启用后编辑 display_name
V=$(et_version "$EG_ID")
body=$(post "/event-types/update" "{\"id\":$EG_ID,\"display_name\":\"改名\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":10,\"range\":100,\"version\":$V}")
assert_code "7b.1 启用后改 display_name → 42015" "42015" "$body"

# 7b.2 启用后编辑 severity
body=$(post "/event-types/update" "{\"id\":$EG_ID,\"display_name\":\"编辑守卫事件\",\"perception_mode\":\"visual\",\"default_severity\":99,\"default_ttl\":10,\"range\":100,\"version\":$V}")
assert_code "7b.2 启用后改 severity → 42015" "42015" "$body"

# 7b.3 启用后编辑 perception_mode
body=$(post "/event-types/update" "{\"id\":$EG_ID,\"display_name\":\"编辑守卫事件\",\"perception_mode\":\"auditory\",\"default_severity\":50,\"default_ttl\":10,\"range\":100,\"version\":$V}")
assert_code "7b.3 启用后改 perception_mode → 42015" "42015" "$body"

# 7b.4 启用后编辑 ttl
body=$(post "/event-types/update" "{\"id\":$EG_ID,\"display_name\":\"编辑守卫事件\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":99,\"range\":100,\"version\":$V}")
assert_code "7b.4 启用后改 ttl → 42015" "42015" "$body"

# 停用后编辑应成功
V=$(et_version "$EG_ID")
post "/event-types/toggle-enabled" "{\"id\":$EG_ID,\"enabled\":false,\"version\":$V}" > /dev/null
V=$(et_version "$EG_ID")
body=$(post "/event-types/update" "{\"id\":$EG_ID,\"display_name\":\"停用后可改\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":10,\"range\":100,\"version\":$V}")
assert_code "7b.5 停用后编辑成功" "0" "$body"

et_rm "$EG_ID"

# =============================================================================
# 7c. Global 模式自动 range=0 全覆盖
# =============================================================================
subsection "7c. Global 模式自动 range=0"

# 创建 global 事件传不同 range 值，验证详情中 range 都是 0
body=$(post "/event-types/create" "{\"name\":\"${P}global_r1\",\"display_name\":\"全局1\",\"perception_mode\":\"global\",\"default_severity\":50,\"default_ttl\":5,\"range\":0}")
assert_code "7c.1 global range=0 创建成功" "0" "$body"
GR1_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')
body=$(et_detail "$GR1_ID")
assert_field "7c.1 详情 range=0" '.data.config.range' "0" "$body"

body=$(post "/event-types/create" "{\"name\":\"${P}global_r2\",\"display_name\":\"全局2\",\"perception_mode\":\"global\",\"default_severity\":50,\"default_ttl\":5,\"range\":500}")
assert_code "7c.2 global range=500 创建成功（自动修正）" "0" "$body"
GR2_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')
body=$(et_detail "$GR2_ID")
assert_field "7c.2 详情 range=0（500 被修正）" '.data.config.range' "0" "$body"

# 编辑 global 事件传 range=100，验证仍为 0
V=$(et_version "$GR1_ID")
body=$(post "/event-types/update" "{\"id\":$GR1_ID,\"display_name\":\"全局1改\",\"perception_mode\":\"global\",\"default_severity\":50,\"default_ttl\":5,\"range\":100,\"version\":$V}")
assert_code "7c.3 编辑 global range=100 成功（自动修正）" "0" "$body"
body=$(et_detail "$GR1_ID")
assert_field "7c.3 编辑后 range 仍为 0" '.data.config.range' "0" "$body"

# =============================================================================
# 8. 切换 + 删除 + 启用守卫
# =============================================================================
subsection "8. 切换 + 删除 + 启用守卫"

# 创建一个临时事件用于删除测试
body=$(post "/event-types/create" "{\"name\":\"${P}del_test\",\"display_name\":\"删除测试\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "8.0 创建删除测试事件" "0" "$body"
DEL_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 8.1 启用后删除 → 42012
V=$(et_version "$DEL_ID")
post "/event-types/toggle-enabled" "{\"id\":$DEL_ID,\"enabled\":true,\"version\":$V}" > /dev/null
body=$(post "/event-types/delete" "{\"id\":$DEL_ID}")
assert_code "8.1 启用后删除 → 42012" "42012" "$body"

# 8.2 停用后删除成功，返回 DeleteResult
V=$(et_version "$DEL_ID")
post "/event-types/toggle-enabled" "{\"id\":$DEL_ID,\"enabled\":false,\"version\":$V}" > /dev/null
body=$(post "/event-types/delete" "{\"id\":$DEL_ID}")
assert_code  "8.2 删除成功" "0" "$body"
assert_field "8.2 返回 id" ".data.id" "$DEL_ID" "$body"
assert_not_equal "8.2 返回 name" ".data.name" "null" "$body"
assert_not_equal "8.2 返回 label" ".data.label" "null" "$body"

# 8.3 软删后 name 不可复用
body=$(post "/event-types/create" "{\"name\":\"${P}del_test\",\"display_name\":\"复用\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "8.3 软删后 name 不可复用 → 42001" "42001" "$body"

# =============================================================================
# 9. check-name
# =============================================================================
subsection "9. check-name"

# 9.1 已存在
body=$(post "/event-types/check-name" "{\"name\":\"${P}visual_evt\"}")
assert_code  "9.1 check-name 已存在" "0" "$body"
assert_field "9.1 not available" '.data.available' "false" "$body"

# 9.2 可用
body=$(post "/event-types/check-name" "{\"name\":\"${P}unique_name\"}")
assert_code  "9.2 check-name 可用" "0" "$body"
assert_field "9.2 available" '.data.available' "true" "$body"

# 9.3 check-name 校验格式 — 大写拒绝
body=$(post "/event-types/check-name" '{"name":"BAD_NAME"}')
assert_code "9.3 check-name 大写拒绝" "42002" "$body"

# 9.4 check-name 空拒绝
body=$(post "/event-types/check-name" '{"name":""}')
assert_code "9.4 check-name 空拒绝" "42002" "$body"

# =============================================================================
# 10. 攻击性测试
# =============================================================================
subsection "10. 攻击性测试"

# 10.1 非法 perception_mode
body=$(post "/event-types/create" "{\"name\":\"${P}bad_mode\",\"display_name\":\"坏模式\",\"perception_mode\":\"invalid\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "10.1 非法 perception_mode → 42003" "42003" "$body"

# 10.2 severity=NaN（JSON 不支持 NaN，传字符串触发类型错误）
body=$(post "/event-types/create" "{\"name\":\"${P}nan_sev\",\"display_name\":\"NaN威胁\",\"perception_mode\":\"visual\",\"default_severity\":\"NaN\",\"default_ttl\":5,\"range\":100}")
assert_code_in "10.2 severity=NaN 被拒" "42004 40000" "$body"

# 10.3 极长 display_name
LONG_DN=$(printf 'A%.0s' {1..300})
body=$(post "/event-types/create" "{\"name\":\"${P}long_dn\",\"display_name\":\"$LONG_DN\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "10.3 极长 display_name 被拒" "40000" "$body"

# 10.4 不存在的 ID 详情
body=$(post "/event-types/detail" "{\"id\":99999999}")
assert_code "10.4 不存在 ID → 42011" "42011" "$body"

# 10.5 SQL 注入 display_name
body=$(post "/event-types/create" "{\"name\":\"${P}sqli\",\"display_name\":\"' OR 1=1 --\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "10.5 SQL 注入不崩" "0" "$body"

# 10.6 perception_mode="telekinesis"
body=$(post "/event-types/create" "{\"name\":\"${P}telekinesis\",\"display_name\":\"超能力\",\"perception_mode\":\"telekinesis\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "10.6 telekinesis → 42003" "42003" "$body"

# 10.7 中文 name
printf '{"name":"枪声","display_name":"中文名","perception_mode":"visual","default_severity":50,"default_ttl":5,"range":100}' | curl -s -X POST "$BASE/event-types/create" -H "Content-Type: application/json; charset=utf-8" --data-binary @- > /tmp/et_cjk.json
body=$(cat /tmp/et_cjk.json)
assert_code "10.7 中文 name → 42002" "42002" "$body"

# 10.8 name 含空格
body=$(post "/event-types/create" "{\"name\":\"bad name\",\"display_name\":\"空格\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "10.8 name 含空格 → 42002" "42002" "$body"

# 10.9 severity 字符串类型
body=$(post "/event-types/create" "{\"name\":\"${P}sev_str\",\"display_name\":\"字符串威胁\",\"perception_mode\":\"visual\",\"default_severity\":\"high\",\"default_ttl\":5,\"range\":100}")
assert_code_in "10.9 severity=string 被拒" "42004 40000" "$body"

# 10.10 ttl 字符串类型
body=$(post "/event-types/create" "{\"name\":\"${P}ttl_str\",\"display_name\":\"字符串TTL\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":\"forever\",\"range\":100}")
assert_code_in "10.10 ttl=string 被拒" "42005 40000" "$body"

# 10.11 range 字符串类型
body=$(post "/event-types/create" "{\"name\":\"${P}range_str\",\"display_name\":\"字符串范围\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":\"far\"}")
assert_code_in "10.11 range=string 被拒" "42006 40000" "$body"

# =============================================================================
# 10b. SEVERITY / TTL / RANGE 编辑时校验
# =============================================================================
subsection "10b. 编辑时系统字段校验"

# 创建一个干净的事件用于编辑校验
body=$(post "/event-types/create" "{\"name\":\"${P}edit_val\",\"display_name\":\"编辑校验\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":10,\"range\":100}")
assert_code "10b.0 创建编辑校验事件" "0" "$body"
EVAL_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 编辑时 severity=-1 拒绝
V=$(et_version "$EVAL_ID")
body=$(post "/event-types/update" "{\"id\":$EVAL_ID,\"display_name\":\"编辑校验\",\"perception_mode\":\"visual\",\"default_severity\":-1,\"default_ttl\":10,\"range\":100,\"version\":$V}")
assert_code "10b.1 编辑 severity=-1 → 42004" "42004" "$body"

# 编辑时 severity=101 拒绝
body=$(post "/event-types/update" "{\"id\":$EVAL_ID,\"display_name\":\"编辑校验\",\"perception_mode\":\"visual\",\"default_severity\":101,\"default_ttl\":10,\"range\":100,\"version\":$V}")
assert_code "10b.2 编辑 severity=101 → 42004" "42004" "$body"

# 编辑时 ttl=0 拒绝
body=$(post "/event-types/update" "{\"id\":$EVAL_ID,\"display_name\":\"编辑校验\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":0,\"range\":100,\"version\":$V}")
assert_code "10b.3 编辑 ttl=0 → 42005" "42005" "$body"

# 编辑时 ttl=-1 拒绝
body=$(post "/event-types/update" "{\"id\":$EVAL_ID,\"display_name\":\"编辑校验\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":-1,\"range\":100,\"version\":$V}")
assert_code "10b.4 编辑 ttl=-1 → 42005" "42005" "$body"

# 编辑时 range=-1 拒绝
body=$(post "/event-types/update" "{\"id\":$EVAL_ID,\"display_name\":\"编辑校验\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":10,\"range\":-1,\"version\":$V}")
assert_code "10b.5 编辑 range=-1 → 42006" "42006" "$body"

et_rm "$EVAL_ID"

# =============================================================================
# 11. 创建第四个事件（供 test_06 使用）
# =============================================================================
subsection "11. 额外事件（供后续测试）"

body=$(post "/event-types/create" "{\"name\":\"${P}fire_evt\",\"display_name\":\"火灾事件\",\"perception_mode\":\"visual\",\"default_severity\":70,\"default_ttl\":20,\"range\":150}")
assert_code "11.1 创建 fire_evt" "0" "$body"
ET_ID4=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

echo ""
echo "  [INFO] 导出变量: ET_ID1=$ET_ID1 ET_ID2=$ET_ID2 ET_ID3=$ET_ID3 ET_ID4=$ET_ID4"
echo ""
