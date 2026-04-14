#!/bin/bash
# =============================================================================
# test_04_event_type.sh — 事件类型 CRUD + 系统字段校验 + 攻击性测试
#
# 前置：run_all.sh 已 source helpers.sh，$BASE / $P / assert_* / post() 可用
#       本文件不依赖 test_05 schema（不用扩展字段）
#
# 导出变量：ET_ID1, ET_ID2, ET_ID3, ET_ID4（供 test_06, test_08 使用）
# =============================================================================

section "Part 4: 事件类型 CRUD + 系统字段校验 (prefix=$P)"

# =============================================================================
# 1. 三种感知模式创建 + 验证
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

# 1.4 验证 visual 详情
body=$(et_detail "$ET_ID1")
assert_code  "1.4 visual 详情成功" "0" "$body"
assert_field "1.4 name" '.data.name' "${P}visual_evt" "$body"
assert_field "1.4 perception_mode=visual" '.data.perception_mode' "visual" "$body"
assert_field "1.4 severity=50" '.data.config.default_severity' "50" "$body"
assert_field "1.4 ttl=10" '.data.config.default_ttl' "10" "$body"
assert_field "1.4 range=200" '.data.config.range' "200" "$body"

# 1.5 验证 auditory 详情
body=$(et_detail "$ET_ID2")
assert_field "1.5 perception_mode=auditory" '.data.perception_mode' "auditory" "$body"
assert_field "1.5 severity=75" '.data.config.default_severity' "75" "$body"

# 1.6 验证 global 详情 — range 被强制为 0
body=$(et_detail "$ET_ID3")
assert_code  "1.6 global 详情成功" "0" "$body"
assert_field "1.6 global range=0（自动修正）" '.data.config.range' "0" "$body"
assert_field "1.6 severity=95" '.data.config.default_severity' "95" "$body"
assert_field "1.6 perception_mode=global" '.data.config.perception_mode' "global" "$body"

# 1.7 所有新建默认 enabled=false
body=$(et_detail "$ET_ID1")
assert_field "1.7 visual 默认 disabled" '.data.enabled' "false" "$body"
body=$(et_detail "$ET_ID3")
assert_field "1.7 global 默认 disabled" '.data.enabled' "false" "$body"

# =============================================================================
# 2. Global range 强制 0 — 创建 + 编辑全覆盖
# =============================================================================
subsection "2. Global range 强制 0"

# 2.1 global range=0 原值
body=$(post "/event-types/create" "{\"name\":\"${P}global_r0\",\"display_name\":\"全局R0\",\"perception_mode\":\"global\",\"default_severity\":50,\"default_ttl\":5,\"range\":0}")
assert_code "2.1 global range=0 创建成功" "0" "$body"
GR0_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')
body=$(et_detail "$GR0_ID")
assert_field "2.1 range=0" '.data.config.range' "0" "$body"

# 2.2 global range=500 被修正
body=$(post "/event-types/create" "{\"name\":\"${P}global_r500\",\"display_name\":\"全局R500\",\"perception_mode\":\"global\",\"default_severity\":50,\"default_ttl\":5,\"range\":500}")
assert_code "2.2 global range=500 创建成功（自动修正）" "0" "$body"
GR500_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')
body=$(et_detail "$GR500_ID")
assert_field "2.2 range=0（500 被修正）" '.data.config.range' "0" "$body"

# 2.3 编辑 global 事件传 range=100 — 仍为 0
V=$(et_version "$GR0_ID")
body=$(post "/event-types/update" "{\"id\":$GR0_ID,\"display_name\":\"全局R0改\",\"perception_mode\":\"global\",\"default_severity\":50,\"default_ttl\":5,\"range\":100,\"version\":$V}")
assert_code "2.3 编辑 global range=100 成功（自动修正）" "0" "$body"
body=$(et_detail "$GR0_ID")
assert_field "2.3 编辑后 range 仍为 0" '.data.config.range' "0" "$body"

# 清理临时 global
et_rm "$GR0_ID"
et_rm "$GR500_ID"

# =============================================================================
# 3. severity 边界 — SEVERITY_INVALID (42004)
# =============================================================================
subsection "3. severity 边界"

# 3.1 severity=0 合法
body=$(post "/event-types/create" "{\"name\":\"${P}sev_zero\",\"display_name\":\"零威胁\",\"perception_mode\":\"visual\",\"default_severity\":0,\"default_ttl\":5,\"range\":100}")
assert_code "3.1 severity=0 合法" "0" "$body"
SEV0_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')
body=$(et_detail "$SEV0_ID")
assert_field "3.1 config.default_severity=0" '.data.config.default_severity' "0" "$body"

# 3.2 severity=100 合法
body=$(post "/event-types/create" "{\"name\":\"${P}sev_hundred\",\"display_name\":\"满威胁\",\"perception_mode\":\"visual\",\"default_severity\":100,\"default_ttl\":5,\"range\":100}")
assert_code "3.2 severity=100 合法" "0" "$body"
SEV100_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 3.3 severity=-1 拒绝
body=$(post "/event-types/create" "{\"name\":\"${P}sev_neg1\",\"display_name\":\"负威胁\",\"perception_mode\":\"visual\",\"default_severity\":-1,\"default_ttl\":5,\"range\":100}")
assert_code "3.3 severity=-1 → 42004" "42004" "$body"

# 3.4 severity=101 拒绝
body=$(post "/event-types/create" "{\"name\":\"${P}sev_101\",\"display_name\":\"超威胁\",\"perception_mode\":\"visual\",\"default_severity\":101,\"default_ttl\":5,\"range\":100}")
assert_code "3.4 severity=101 → 42004" "42004" "$body"

# 3.5 severity=-100 极端负
body=$(post "/event-types/create" "{\"name\":\"${P}sev_neg100\",\"display_name\":\"极负威胁\",\"perception_mode\":\"visual\",\"default_severity\":-100,\"default_ttl\":5,\"range\":100}")
assert_code "3.5 severity=-100 → 42004" "42004" "$body"

# 3.6 severity=999 极端正
body=$(post "/event-types/create" "{\"name\":\"${P}sev_999\",\"display_name\":\"超大威胁\",\"perception_mode\":\"visual\",\"default_severity\":999,\"default_ttl\":5,\"range\":100}")
assert_code "3.6 severity=999 → 42004" "42004" "$body"

# 清理 sev 成功的
et_rm "$SEV0_ID"
et_rm "$SEV100_ID"

# =============================================================================
# 4. TTL 边界 — TTL_INVALID (42005)
# =============================================================================
subsection "4. TTL 边界"

# 4.1 ttl=0.1 合法（float ok）
body=$(post "/event-types/create" "{\"name\":\"${P}ttl_small\",\"display_name\":\"小TTL\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":0.1,\"range\":100}")
assert_code "4.1 ttl=0.1 合法" "0" "$body"
TTL_SMALL_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')
et_rm "$TTL_SMALL_ID"

# 4.2 ttl=0 拒绝（must be >0）
body=$(post "/event-types/create" "{\"name\":\"${P}ttl_zero\",\"display_name\":\"零TTL\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":0,\"range\":100}")
assert_code "4.2 ttl=0 → 42005" "42005" "$body"

# 4.3 ttl=-1 拒绝
body=$(post "/event-types/create" "{\"name\":\"${P}ttl_neg1\",\"display_name\":\"负TTL\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":-1,\"range\":100}")
assert_code "4.3 ttl=-1 → 42005" "42005" "$body"

# 4.4 ttl=-100 极端负
body=$(post "/event-types/create" "{\"name\":\"${P}ttl_neg100\",\"display_name\":\"极负TTL\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":-100,\"range\":100}")
assert_code "4.4 ttl=-100 → 42005" "42005" "$body"

# =============================================================================
# 5. range 边界 — RANGE_INVALID (42006)
# =============================================================================
subsection "5. range 边界"

# 5.1 range=0 合法（非 global 也可以）
body=$(post "/event-types/create" "{\"name\":\"${P}range_zero\",\"display_name\":\"零范围\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":0}")
assert_code "5.1 range=0 合法" "0" "$body"
RNG0_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')
et_rm "$RNG0_ID"

# 5.2 range=-1 拒绝
body=$(post "/event-types/create" "{\"name\":\"${P}range_neg1\",\"display_name\":\"负范围\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":-1}")
assert_code "5.2 range=-1 → 42006" "42006" "$body"

# 5.3 range=-999 极端负
body=$(post "/event-types/create" "{\"name\":\"${P}range_neg999\",\"display_name\":\"极负范围\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":-999}")
assert_code "5.3 range=-999 → 42006" "42006" "$body"

# =============================================================================
# 6. name 校验 — NAME_FORMAT_INVALID (42002) / NAME_EXISTS (42001)
# =============================================================================
subsection "6. name 校验"

# 6.1 空 name
body=$(post "/event-types/create" "{\"name\":\"\",\"display_name\":\"空名\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "6.1 空 name → 42002" "42002" "$body"

# 6.2 大写 name
body=$(post "/event-types/create" "{\"name\":\"${P}BadCase\",\"display_name\":\"大写\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "6.2 大写 name → 42002" "42002" "$body"

# 6.3 重复 name
body=$(post "/event-types/create" "{\"name\":\"${P}visual_evt\",\"display_name\":\"重复\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "6.3 重复 name → 42001" "42001" "$body"

# 6.4 中文 name
printf '{"name":"枪声事件","display_name":"中文","perception_mode":"visual","default_severity":50,"default_ttl":5,"range":100}' \
  | curl -s -X POST "$BASE/event-types/create" -H "Content-Type: application/json; charset=utf-8" --data-binary @- > /tmp/et_cjk.json
body=$(cat /tmp/et_cjk.json)
assert_code "6.4 中文 name → 42002" "42002" "$body"

# 6.5 name 含空格
body=$(post "/event-types/create" "{\"name\":\"bad name\",\"display_name\":\"空格\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "6.5 name 含空格 → 42002" "42002" "$body"

# 6.6 name 含特殊字符
body=$(post "/event-types/create" "{\"name\":\"bad@name!\",\"display_name\":\"特殊字符\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "6.6 name 特殊字符 → 42002" "42002" "$body"

# =============================================================================
# 7. 详情 + 列表 + 筛选
# =============================================================================
subsection "7. 详情 + 列表 + 筛选"

# 7.1 详情
body=$(et_detail "$ET_ID1")
assert_code  "7.1 详情成功" "0" "$body"
assert_field "7.1 name 正确" '.data.name' "${P}visual_evt" "$body"
assert_field "7.1 perception_mode" '.data.perception_mode' "visual" "$body"

# 7.2 列表
body=$(post "/event-types/list" "{\"page\":1,\"page_size\":20}")
assert_code "7.2 列表成功" "0" "$body"
assert_ge   "7.2 total >= 3" '.data.total' "3" "$body"

# 7.3 perception_mode 筛选 — global
body=$(post "/event-types/list" "{\"perception_mode\":\"global\",\"page\":1,\"page_size\":20}")
assert_code "7.3 列表 global 筛选" "0" "$body"
assert_ge   "7.3 global >= 1" '.data.total' "1" "$body"

# 7.4 perception_mode 筛选 — visual
body=$(post "/event-types/list" "{\"perception_mode\":\"visual\",\"page\":1,\"page_size\":20}")
assert_code "7.4 列表 visual 筛选" "0" "$body"
assert_ge   "7.4 visual >= 1" '.data.total' "1" "$body"

# 7.5 perception_mode 筛选 — auditory
body=$(post "/event-types/list" "{\"perception_mode\":\"auditory\",\"page\":1,\"page_size\":20}")
assert_code "7.5 列表 auditory 筛选" "0" "$body"
assert_ge   "7.5 auditory >= 1" '.data.total' "1" "$body"

# 7.6 label 筛选
body=$(post "/event-types/list" "{\"label\":\"视觉\",\"page\":1,\"page_size\":20}")
assert_code "7.6 label 筛选" "0" "$body"

# 7.7 enabled 筛选（全部未启用）
body=$(post "/event-types/list" "{\"enabled\":false,\"page\":1,\"page_size\":20}")
assert_code "7.7 enabled=false 筛选" "0" "$body"
assert_ge   "7.7 disabled >= 3" '.data.total' "3" "$body"

# 7.8 enabled=true 筛选（暂无启用的）
body=$(post "/event-types/list" "{\"enabled\":true,\"page\":1,\"page_size\":20}")
assert_code "7.8 enabled=true 筛选" "0" "$body"

# =============================================================================
# 8. 编辑 — 成功 + 启用守卫 + 版本冲突 + 全字段可编辑
# =============================================================================
subsection "8. 编辑 + 启用守卫 + 版本冲突"

# 8.1 编辑成功（默认 enabled=false）
V=$(et_version "$ET_ID1")
body=$(post "/event-types/update" "{\"id\":$ET_ID1,\"display_name\":\"视觉事件(改)\",\"perception_mode\":\"visual\",\"default_severity\":60,\"default_ttl\":12,\"range\":220,\"version\":$V}")
assert_code "8.1 编辑成功" "0" "$body"

# 8.2 验证编辑结果
body=$(et_detail "$ET_ID1")
assert_field "8.2 severity=60" '.data.config.default_severity' "60" "$body"
assert_field "8.2 ttl=12" '.data.config.default_ttl' "12" "$body"
assert_field "8.2 range=220" '.data.config.range' "220" "$body"

# 8.3 启用
V=$(et_version "$ET_ID1")
body=$(post "/event-types/toggle-enabled" "{\"id\":$ET_ID1,\"enabled\":true,\"version\":$V}")
assert_code "8.3 启用" "0" "$body"

# 8.4 EDIT_NOT_DISABLED: 启用后编辑 → 42015
V=$(et_version "$ET_ID1")
body=$(post "/event-types/update" "{\"id\":$ET_ID1,\"display_name\":\"视觉事件(再改)\",\"perception_mode\":\"visual\",\"default_severity\":60,\"default_ttl\":12,\"range\":220,\"version\":$V}")
assert_code "8.4 启用后编辑 → 42015" "42015" "$body"

# 8.5 版本冲突（用错误 version）— 先停用
V=$(et_version "$ET_ID1")
post "/event-types/toggle-enabled" "{\"id\":$ET_ID1,\"enabled\":false,\"version\":$V}" > /dev/null
body=$(post "/event-types/update" "{\"id\":$ET_ID1,\"display_name\":\"冲突测试\",\"perception_mode\":\"visual\",\"default_severity\":60,\"default_ttl\":12,\"range\":220,\"version\":999}")
assert_code "8.5 版本冲突 → 42010" "42010" "$body"

# =============================================================================
# 8b. EDIT_NOT_DISABLED 全字段覆盖
# =============================================================================
subsection "8b. EDIT_NOT_DISABLED 全字段覆盖"

body=$(post "/event-types/create" "{\"name\":\"${P}edit_guard_evt\",\"display_name\":\"编辑守卫事件\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":10,\"range\":100}")
assert_code "8b.0 创建编辑守卫事件" "0" "$body"
EG_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 启用
V=$(et_version "$EG_ID")
post "/event-types/toggle-enabled" "{\"id\":$EG_ID,\"enabled\":true,\"version\":$V}" > /dev/null

# 8b.1 启用后改 display_name
V=$(et_version "$EG_ID")
body=$(post "/event-types/update" "{\"id\":$EG_ID,\"display_name\":\"改名\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":10,\"range\":100,\"version\":$V}")
assert_code "8b.1 启用后改 display_name → 42015" "42015" "$body"

# 8b.2 启用后改 severity
body=$(post "/event-types/update" "{\"id\":$EG_ID,\"display_name\":\"编辑守卫事件\",\"perception_mode\":\"visual\",\"default_severity\":99,\"default_ttl\":10,\"range\":100,\"version\":$V}")
assert_code "8b.2 启用后改 severity → 42015" "42015" "$body"

# 8b.3 启用后改 perception_mode
body=$(post "/event-types/update" "{\"id\":$EG_ID,\"display_name\":\"编辑守卫事件\",\"perception_mode\":\"auditory\",\"default_severity\":50,\"default_ttl\":10,\"range\":100,\"version\":$V}")
assert_code "8b.3 启用后改 perception_mode → 42015" "42015" "$body"

# 8b.4 启用后改 ttl
body=$(post "/event-types/update" "{\"id\":$EG_ID,\"display_name\":\"编辑守卫事件\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":99,\"range\":100,\"version\":$V}")
assert_code "8b.4 启用后改 ttl → 42015" "42015" "$body"

# 8b.5 停用后编辑应成功
V=$(et_version "$EG_ID")
post "/event-types/toggle-enabled" "{\"id\":$EG_ID,\"enabled\":false,\"version\":$V}" > /dev/null
V=$(et_version "$EG_ID")
body=$(post "/event-types/update" "{\"id\":$EG_ID,\"display_name\":\"停用后可改\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":10,\"range\":100,\"version\":$V}")
assert_code "8b.5 停用后编辑成功" "0" "$body"

et_rm "$EG_ID"

# =============================================================================
# 9. 编辑时字段校验 — severity/ttl/range 边界
# =============================================================================
subsection "9. 编辑时系统字段校验"

body=$(post "/event-types/create" "{\"name\":\"${P}edit_val\",\"display_name\":\"编辑校验\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":10,\"range\":100}")
assert_code "9.0 创建编辑校验事件" "0" "$body"
EVAL_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')
V=$(et_version "$EVAL_ID")

# 9.1 severity=-1
body=$(post "/event-types/update" "{\"id\":$EVAL_ID,\"display_name\":\"编辑校验\",\"perception_mode\":\"visual\",\"default_severity\":-1,\"default_ttl\":10,\"range\":100,\"version\":$V}")
assert_code "9.1 编辑 severity=-1 → 42004" "42004" "$body"

# 9.2 severity=101
body=$(post "/event-types/update" "{\"id\":$EVAL_ID,\"display_name\":\"编辑校验\",\"perception_mode\":\"visual\",\"default_severity\":101,\"default_ttl\":10,\"range\":100,\"version\":$V}")
assert_code "9.2 编辑 severity=101 → 42004" "42004" "$body"

# 9.3 ttl=0
body=$(post "/event-types/update" "{\"id\":$EVAL_ID,\"display_name\":\"编辑校验\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":0,\"range\":100,\"version\":$V}")
assert_code "9.3 编辑 ttl=0 → 42005" "42005" "$body"

# 9.4 ttl=-1
body=$(post "/event-types/update" "{\"id\":$EVAL_ID,\"display_name\":\"编辑校验\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":-1,\"range\":100,\"version\":$V}")
assert_code "9.4 编辑 ttl=-1 → 42005" "42005" "$body"

# 9.5 range=-1
body=$(post "/event-types/update" "{\"id\":$EVAL_ID,\"display_name\":\"编辑校验\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":10,\"range\":-1,\"version\":$V}")
assert_code "9.5 编辑 range=-1 → 42006" "42006" "$body"

# 9.6 编辑 severity=0 合法
body=$(post "/event-types/update" "{\"id\":$EVAL_ID,\"display_name\":\"编辑校验\",\"perception_mode\":\"visual\",\"default_severity\":0,\"default_ttl\":10,\"range\":100,\"version\":$V}")
assert_code "9.6 编辑 severity=0 合法" "0" "$body"

# 9.7 编辑 severity=100 合法
V=$(et_version "$EVAL_ID")
body=$(post "/event-types/update" "{\"id\":$EVAL_ID,\"display_name\":\"编辑校验\",\"perception_mode\":\"visual\",\"default_severity\":100,\"default_ttl\":10,\"range\":100,\"version\":$V}")
assert_code "9.7 编辑 severity=100 合法" "0" "$body"

et_rm "$EVAL_ID"

# =============================================================================
# 10. 切换 + 删除生命周期
# =============================================================================
subsection "10. 切换 + 删除 + 启用守卫"

body=$(post "/event-types/create" "{\"name\":\"${P}del_test\",\"display_name\":\"删除测试\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "10.0 创建删除测试事件" "0" "$body"
DEL_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 10.1 启用后删除 → 42012
V=$(et_version "$DEL_ID")
post "/event-types/toggle-enabled" "{\"id\":$DEL_ID,\"enabled\":true,\"version\":$V}" > /dev/null
body=$(post "/event-types/delete" "{\"id\":$DEL_ID}")
assert_code "10.1 启用后删除 → 42012" "42012" "$body"

# 10.2 停用后删除成功
V=$(et_version "$DEL_ID")
post "/event-types/toggle-enabled" "{\"id\":$DEL_ID,\"enabled\":false,\"version\":$V}" > /dev/null
body=$(post "/event-types/delete" "{\"id\":$DEL_ID}")
assert_code  "10.2 删除成功" "0" "$body"
assert_field "10.2 返回 id" ".data.id" "$DEL_ID" "$body"
assert_not_equal "10.2 返回 name" ".data.name" "null" "$body"
assert_not_equal "10.2 返回 label" ".data.label" "null" "$body"

# 10.3 软删后 name 不可复用
body=$(post "/event-types/create" "{\"name\":\"${P}del_test\",\"display_name\":\"复用\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "10.3 软删后 name 不可复用 → 42001" "42001" "$body"

# 10.4 不存在的 ID 删除
body=$(post "/event-types/delete" "{\"id\":99999999}")
assert_code "10.4 删除不存在 ID → 42011" "42011" "$body"

# =============================================================================
# 11. check-name
# =============================================================================
subsection "11. check-name"

# 11.1 已存在
body=$(post "/event-types/check-name" "{\"name\":\"${P}visual_evt\"}")
assert_code  "11.1 check-name 已存在" "0" "$body"
assert_field "11.1 not available" '.data.available' "false" "$body"

# 11.2 可用
body=$(post "/event-types/check-name" "{\"name\":\"${P}unique_name_xyz\"}")
assert_code  "11.2 check-name 可用" "0" "$body"
assert_field "11.2 available" '.data.available' "true" "$body"

# 11.3 大写拒绝
body=$(post "/event-types/check-name" '{"name":"BAD_NAME"}')
assert_code "11.3 check-name 大写拒绝 → 42002" "42002" "$body"

# 11.4 空拒绝
body=$(post "/event-types/check-name" '{"name":""}')
assert_code "11.4 check-name 空拒绝 → 42002" "42002" "$body"

# 11.5 特殊字符
body=$(post "/event-types/check-name" '{"name":"foo@bar!"}')
assert_code "11.5 check-name 特殊字符 → 42002" "42002" "$body"

# =============================================================================
# 12. 攻击性测试 — 类型异常 + 非法值 + 注入
# =============================================================================
subsection "12. 攻击性测试"

# 12.1 非法 perception_mode
body=$(post "/event-types/create" "{\"name\":\"${P}bad_mode\",\"display_name\":\"坏模式\",\"perception_mode\":\"invalid\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "12.1 非法 perception_mode → 42003" "42003" "$body"

# 12.2 severity=NaN（字符串）
body=$(post "/event-types/create" "{\"name\":\"${P}nan_sev\",\"display_name\":\"NaN威胁\",\"perception_mode\":\"visual\",\"default_severity\":\"NaN\",\"default_ttl\":5,\"range\":100}")
assert_code_in "12.2 severity=NaN 被拒" "42004 40000" "$body"

# 12.3 severity=string
body=$(post "/event-types/create" "{\"name\":\"${P}str_sev\",\"display_name\":\"字符串威胁\",\"perception_mode\":\"visual\",\"default_severity\":\"high\",\"default_ttl\":5,\"range\":100}")
assert_code_in "12.3 severity=string 被拒" "42004 40000" "$body"

# 12.4 ttl=string
body=$(post "/event-types/create" "{\"name\":\"${P}str_ttl\",\"display_name\":\"字符串TTL\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":\"forever\",\"range\":100}")
assert_code_in "12.4 ttl=string 被拒" "42005 40000" "$body"

# 12.5 range=string
body=$(post "/event-types/create" "{\"name\":\"${P}str_range\",\"display_name\":\"字符串范围\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":\"far\"}")
assert_code_in "12.5 range=string 被拒" "42006 40000" "$body"

# 12.6 极长 display_name (300 chars)
LONG_DN=$(printf 'A%.0s' {1..300})
body=$(post "/event-types/create" "{\"name\":\"${P}long_dn\",\"display_name\":\"$LONG_DN\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "12.6 极长 display_name 被拒" "40000" "$body"

# 12.7 不存在的 ID 详情
body=$(post "/event-types/detail" "{\"id\":99999999}")
assert_code "12.7 不存在 ID → 42011" "42011" "$body"

# 12.8 SQL 注入 display_name（不应崩）
body=$(post "/event-types/create" "{\"name\":\"${P}sqli\",\"display_name\":\"' OR 1=1 --\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "12.8 SQL 注入不崩" "0" "$body"
SQLI_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')
et_rm "$SQLI_ID"

# 12.9 perception_mode="telekinesis"
body=$(post "/event-types/create" "{\"name\":\"${P}telekinesis\",\"display_name\":\"超能力\",\"perception_mode\":\"telekinesis\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "12.9 telekinesis → 42003" "42003" "$body"

# 12.10 severity float (50.5) — 探测是否接受小数
body=$(post "/event-types/create" "{\"name\":\"${P}sev_float\",\"display_name\":\"浮点威胁\",\"perception_mode\":\"visual\",\"default_severity\":50.5,\"default_ttl\":5,\"range\":100}")
assert_not_500 "12.10 severity=50.5 不崩" "$body"

# 12.11 very large severity (999999)
body=$(post "/event-types/create" "{\"name\":\"${P}sev_huge\",\"display_name\":\"巨大威胁\",\"perception_mode\":\"visual\",\"default_severity\":999999,\"default_ttl\":5,\"range\":100}")
assert_code "12.11 severity=999999 → 42004" "42004" "$body"

# 12.12 very large range (999999)
body=$(post "/event-types/create" "{\"name\":\"${P}range_huge\",\"display_name\":\"巨大范围\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":999999}")
assert_not_500 "12.12 range=999999 不崩" "$body"

# 12.13 very large ttl (999999)
body=$(post "/event-types/create" "{\"name\":\"${P}ttl_huge\",\"display_name\":\"巨大TTL\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":999999,\"range\":100}")
assert_not_500 "12.13 ttl=999999 不崩" "$body"

# =============================================================================
# 13. 攻击 — perception_mode 变更 + global range=0 编辑
# =============================================================================
subsection "13. perception_mode 变更攻击"

# 创建 visual，编辑为 global — range 应被强制 0
body=$(post "/event-types/create" "{\"name\":\"${P}mode_change\",\"display_name\":\"模式变更\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":500}")
assert_code "13.1 创建 visual range=500" "0" "$body"
MC_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 编辑 visual → global，range=100
V=$(et_version "$MC_ID")
body=$(post "/event-types/update" "{\"id\":$MC_ID,\"display_name\":\"模式变更\",\"perception_mode\":\"global\",\"default_severity\":50,\"default_ttl\":5,\"range\":100,\"version\":$V}")
assert_code "13.2 visual→global 编辑成功" "0" "$body"
body=$(et_detail "$MC_ID")
assert_field "13.2 变更后 range=0" '.data.config.range' "0" "$body"
assert_field "13.2 perception_mode=global" '.data.config.perception_mode' "global" "$body"

# 编辑 global → auditory，range 不应再被强制
V=$(et_version "$MC_ID")
body=$(post "/event-types/update" "{\"id\":$MC_ID,\"display_name\":\"模式变更\",\"perception_mode\":\"auditory\",\"default_severity\":50,\"default_ttl\":5,\"range\":250,\"version\":$V}")
assert_code "13.3 global→auditory 编辑成功" "0" "$body"
body=$(et_detail "$MC_ID")
assert_field "13.3 auditory range=250（不被修正）" '.data.config.range' "250" "$body"

et_rm "$MC_ID"

# =============================================================================
# 14. 攻击 — extensions 字段类型异常
# =============================================================================
subsection "14. extensions 类型异常攻击"

# 14.1 extensions 传字符串
body=$(post "/event-types/create" "{\"name\":\"${P}ext_str\",\"display_name\":\"字符串扩展\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100,\"extensions\":\"bad\"}")
assert_not_500 "14.1 extensions=string 不崩" "$body"

# 14.2 extensions 传数组
body=$(post "/event-types/create" "{\"name\":\"${P}ext_arr\",\"display_name\":\"数组扩展\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100,\"extensions\":[1,2,3]}")
assert_not_500 "14.2 extensions=array 不崩" "$body"

# 14.3 extensions 传数字
body=$(post "/event-types/create" "{\"name\":\"${P}ext_num\",\"display_name\":\"数字扩展\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100,\"extensions\":42}")
assert_not_500 "14.3 extensions=number 不崩" "$body"

# 14.4 extensions key 指向不存在的 schema → 42022
body=$(post "/event-types/create" "{\"name\":\"${P}ext_ghost\",\"display_name\":\"幽灵扩展\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100,\"extensions\":{\"nonexistent_schema_key\":42}}")
assert_code_in "14.4 extensions 不存在 schema key" "42022 42007 40000 0" "$body"

# =============================================================================
# 15. 攻击 — 畸形请求
# =============================================================================
subsection "15. 畸形请求攻击"

# 15.1 空 body
body=$(post "/event-types/create" "{}")
assert_not_500 "15.1 空 body 不崩" "$body"

# 15.2 GET 请求到 POST 端点
body=$(raw_get "/event-types/create")
assert_not_500 "15.2 GET 到 create 不崩" "$body"

# 15.3 PUT 请求
body=$(raw_put "/event-types/create" "{\"name\":\"${P}put_test\"}")
assert_not_500 "15.3 PUT 到 create 不崩" "$body"

# 15.4 DELETE 请求
body=$(raw_delete "/event-types/create")
assert_not_500 "15.4 DELETE 到 create 不崩" "$body"

# 15.5 detail 负数 ID
body=$(post "/event-types/detail" "{\"id\":-1}")
assert_code "15.5 detail 负数 ID → 42011" "42011" "$body"

# 15.6 detail ID=0
body=$(post "/event-types/detail" "{\"id\":0}")
assert_code_in "15.6 detail ID=0" "42011 40000" "$body"

# 15.7 toggle 不传 enabled
body=$(post "/event-types/toggle-enabled" "{\"id\":$ET_ID2,\"version\":1}")
assert_not_500 "15.7 toggle 不传 enabled 不崩" "$body"

# =============================================================================
# 16. 创建第四个事件 + 确保导出变量
# =============================================================================
subsection "16. 额外事件（供后续测试）"

body=$(post "/event-types/create" "{\"name\":\"${P}fire_evt\",\"display_name\":\"火灾事件\",\"perception_mode\":\"visual\",\"default_severity\":70,\"default_ttl\":20,\"range\":150}")
assert_code "16.1 创建 fire_evt" "0" "$body"
ET_ID4=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 确保 ET_ID1 恢复到 disabled 状态（被 8.5 停用了）
body=$(et_detail "$ET_ID1")
EN=$(echo "$body" | jq -r '.data.enabled' | tr -d '\r')
if [ "$EN" = "true" ]; then
  V=$(et_version "$ET_ID1")
  post "/event-types/toggle-enabled" "{\"id\":$ET_ID1,\"enabled\":false,\"version\":$V}" > /dev/null
fi

echo ""
echo "  [INFO] 导出变量: ET_ID1=$ET_ID1 ET_ID2=$ET_ID2 ET_ID3=$ET_ID3 ET_ID4=$ET_ID4"
echo ""
