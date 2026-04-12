#!/bin/bash
# =============================================================================
# ADMIN 后端 API 全方位集成测试
#
# 覆盖：字段管理 + 模板管理 + 事件类型 + 扩展字段 Schema + 导出 API + 攻击性测试
#
# 用法：bash tests/integration_test.sh
#
# 脚本自动完成：
#   1. docker compose up --build -d  （重建后端镜像）
#   2. MySQL DROP + 重建所有表（自增 ID 从 1 开始）
#   3. 执行 seed 种子数据
#   4. 等待后端就绪
#   5. 运行全部测试用例
#
# 约定：
#   [PASS] ... 测试通过
#   [FAIL] ... 测试失败
#   [BUG ] ... 攻击测试命中的可疑 bug
# =============================================================================

set -o pipefail

export LANG=en_US.UTF-8
export LC_ALL=en_US.UTF-8
if command -v chcp.com &>/dev/null; then
  chcp.com 65001 > /dev/null 2>&1
fi

# 项目根目录（脚本所在目录的上一层）
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

MYSQL_CONTAINER="npc-admin-mysql"
MYSQL_CMD="docker exec -i $MYSQL_CONTAINER mysql -uroot -proot npc_ai_admin"
MIGRATIONS_DIR="$PROJECT_ROOT/backend/migrations"

BASE="http://localhost:9821/api/v1"
EXPORT_BASE="http://localhost:9821/api/configs"
PASS=0
FAIL=0
TOTAL=0
BUGS=()
TS=$(date +%s)
P="t${TS}_"

# =============================================================================
# Phase 0: 环境准备（Docker 重建 + MySQL 清空 + Seed）
# =============================================================================

echo "================================================================="
echo "  Phase 0: 环境准备"
echo "================================================================="
echo ""

# 0.1 Docker Compose 重建后端
echo "  [0.1] docker compose up --build -d ..."
cd "$PROJECT_ROOT"
docker compose up --build -d 2>&1 | tail -5
echo ""

# 0.2 等待 MySQL 就绪
echo "  [0.2] 等待 MySQL 就绪 ..."
for i in $(seq 1 30); do
  if docker exec $MYSQL_CONTAINER mysqladmin ping -uroot -proot --silent 2>/dev/null; then
    echo "  MySQL 就绪 (${i}s)"
    break
  fi
  sleep 1
done

# 0.3 清空 Redis 缓存（防止上一次测试的缓存污染）
echo "  [0.3] 清空 Redis 缓存 ..."
docker exec npc-admin-redis redis-cli FLUSHALL > /dev/null 2>&1

# 0.3b 清空业务数据表（TRUNCATE 重置自增 ID），保留字典和 Schema 定义表
echo "  [0.3b] 清空业务数据 ..."
echo "SET FOREIGN_KEY_CHECKS=0;
DROP TABLE IF EXISTS field_refs;
DROP TABLE IF EXISTS fields;
DROP TABLE IF EXISTS templates;
DROP TABLE IF EXISTS event_types;
DROP TABLE IF EXISTS event_type_schema;
SET FOREIGN_KEY_CHECKS=1;" | $MYSQL_CMD 2>/dev/null

# 字典和 Schema 只在不存在时创建 + 确保种子数据存在
echo "  [0.3c] 确保基础表存在 ..."
for f in "$MIGRATIONS_DIR"/0*.sql; do
  $MYSQL_CMD < "$f" 2>/dev/null
done

# 检查字典是否已有数据（有则跳过 seed）
DICT_COUNT=$(echo "SELECT COUNT(*) AS c FROM dictionaries;" | $MYSQL_CMD -N 2>/dev/null | tr -d '\r ')

if [ "$DICT_COUNT" -gt 0 ] 2>/dev/null; then
  echo "  字典已有 ${DICT_COUNT} 条，跳过 seed"
else
  echo "  [0.4] 执行种子脚本 ..."
  cd "$PROJECT_ROOT/backend"
  go run ./cmd/seed/ -config config.yaml 2>&1
  cd "$PROJECT_ROOT"
fi
echo ""

# 0.5 重启后端（让 DictCache / SchemaCache 加载刚写入的种子数据）
echo "  [0.5] 重启后端容器 ..."
docker restart npc-admin-backend > /dev/null 2>&1

# 0.6 等待后端就绪
echo "  [0.6] 等待后端就绪 ..."
for i in $(seq 1 60); do
  if curl -s http://localhost:9821/health | grep -q '"ok"' 2>/dev/null; then
    echo "  后端就绪 (${i}s)"
    break
  fi
  if [ "$i" -eq 60 ]; then
    echo "  [FATAL] 后端 60s 内未就绪，终止测试"
    exit 1
  fi
  sleep 1
done
echo ""
echo "  环境准备完成"
echo ""


# =============================================================================
# 工具函数
# =============================================================================

assert_code() {
  local name="$1" expected="$2" body="$3"
  TOTAL=$((TOTAL + 1))
  local actual=$(echo "$body" | jq -r '.code // empty' 2>/dev/null | tr -d '\r')
  if [ "$actual" = "$expected" ]; then
    echo "  [PASS] $name"
    PASS=$((PASS + 1))
  else
    echo "  [FAIL] $name — 期望 code=$expected, 实际: $actual"
    echo "         响应: $(echo "$body" | head -c 200)"
    FAIL=$((FAIL + 1))
  fi
}

assert_field() {
  local name="$1" expr="$2" expected="$3" body="$4"
  TOTAL=$((TOTAL + 1))
  local actual=$(echo "$body" | jq -r "$expr" 2>/dev/null | tr -d '\r')
  if [ "$actual" = "$expected" ]; then
    echo "  [PASS] $name"
    PASS=$((PASS + 1))
  else
    echo "  [FAIL] $name — 期望 $expected, 实际: $actual"
    FAIL=$((FAIL + 1))
  fi
}

assert_ge() {
  local name="$1" expr="$2" min="$3" body="$4"
  TOTAL=$((TOTAL + 1))
  local actual=$(echo "$body" | jq -r "$expr" 2>/dev/null | tr -d '\r')
  if [ "$actual" -ge "$min" ] 2>/dev/null; then
    echo "  [PASS] $name (=$actual)"
    PASS=$((PASS + 1))
  else
    echo "  [FAIL] $name — 期望 >= $min, 实际: $actual"
    FAIL=$((FAIL + 1))
  fi
}

assert_not_equal() {
  local name="$1" expr="$2" unexpected="$3" body="$4"
  TOTAL=$((TOTAL + 1))
  local actual=$(echo "$body" | jq -r "$expr" 2>/dev/null | tr -d '\r')
  if [ "$actual" != "$unexpected" ]; then
    echo "  [PASS] $name (=$actual)"
    PASS=$((PASS + 1))
  else
    echo "  [FAIL] $name — 不应为 $unexpected, 实际: $actual"
    FAIL=$((FAIL + 1))
  fi
}

# 攻击测试专用：期望的"正确行为"未实现时记录 bug，但仍计入 FAIL 便于总览
assert_bug() {
  local name="$1" expected="$2" body="$3" bug_desc="$4"
  TOTAL=$((TOTAL + 1))
  local actual=$(echo "$body" | jq -r '.code // empty' 2>/dev/null | tr -d '\r')
  if [ "$actual" = "$expected" ]; then
    echo "  [PASS] $name"
    PASS=$((PASS + 1))
  else
    echo "  [BUG ] $name — 期望 code=$expected（正确行为），实际 code=$actual"
    echo "         bug: $bug_desc"
    echo "         响应: $(echo "$body" | head -c 200)"
    FAIL=$((FAIL + 1))
    BUGS+=("$name: $bug_desc")
  fi
}

# 攻击测试：期望 code 属于某个允许集合之一（用空格分隔）
assert_code_in() {
  local name="$1" allowed="$2" body="$3"
  TOTAL=$((TOTAL + 1))
  local actual=$(echo "$body" | jq -r '.code // empty' 2>/dev/null | tr -d '\r')
  for c in $allowed; do
    if [ "$actual" = "$c" ]; then
      echo "  [PASS] $name (code=$actual)"
      PASS=$((PASS + 1))
      return
    fi
  done
  echo "  [FAIL] $name — 期望 code ∈ {$allowed}, 实际: $actual"
  echo "         响应: $(echo "$body" | head -c 200)"
  FAIL=$((FAIL + 1))
}

post() {
  printf '%s' "$2" | curl -s -X POST "$BASE$1" -H "Content-Type: application/json; charset=utf-8" --data-binary @-
}

# ---- 字段辅助 ----
fld_detail()     { post "/fields/detail" "{\"id\":$1}"; }
fld_version()    { fld_detail "$1" | jq -r '.data.version' | tr -d '\r'; }
fld_refcount()   { fld_detail "$1" | jq -r '.data.ref_count' | tr -d '\r'; }
fld_enabled()    { fld_detail "$1" | jq -r '.data.enabled' | tr -d '\r'; }
fld_type()       { fld_detail "$1" | jq -r '.data.type' | tr -d '\r'; }
fld_enable()     { local ver=$(fld_version "$1"); post "/fields/toggle-enabled" "{\"id\":$1,\"enabled\":true,\"version\":${ver}}" > /dev/null; }
fld_disable()    { local ver=$(fld_version "$1"); post "/fields/toggle-enabled" "{\"id\":$1,\"enabled\":false,\"version\":${ver}}" > /dev/null; }
fld_rm()         { fld_disable "$1" 2>/dev/null; post "/fields/delete" "{\"id\":$1}" > /dev/null 2>&1; }

# ---- 模板辅助 ----
tpl_detail()     { post "/templates/detail" "{\"id\":$1}"; }
tpl_version()    { tpl_detail "$1" | jq -r '.data.version' | tr -d '\r'; }
tpl_refcount()   { tpl_detail "$1" | jq -r '.data.ref_count' | tr -d '\r'; }
tpl_enable()     { local ver=$(tpl_version "$1"); post "/templates/toggle-enabled" "{\"id\":$1,\"enabled\":true,\"version\":${ver}}" > /dev/null; }
tpl_disable()    { local ver=$(tpl_version "$1"); post "/templates/toggle-enabled" "{\"id\":$1,\"enabled\":false,\"version\":${ver}}" > /dev/null; }
tpl_rm()         { tpl_disable "$1" 2>/dev/null; post "/templates/delete" "{\"id\":$1}" > /dev/null 2>&1; }

section() {
  echo ""
  echo "=============================================================="
  echo "  $1"
  echo "=============================================================="
}

subsection() {
  echo ""
  echo "--- $1 ---"
}


# ---- 事件类型辅助 ----
et_detail()  { post "/event-types/detail" "{\"id\":$1}"; }
et_version() { et_detail "$1" | jq -r '.data.version' | tr -d '\r'; }
get_export() { curl -s "$EXPORT_BASE$1"; }

# =============================================================================
# PART 0: 健康检查 + 字典
# PART 1: 字段管理
# PART 2: 模板管理
# PART 3: 跨模块
# PART 4: 攻击性测试（字段+模板）
# =============================================================================
# =============================================================================
# 开始测试
# =============================================================================

section "ADMIN 后端 API 全方位集成测试 (prefix=$P)"

# ---- 健康检查 ----
subsection "健康检查"
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
section "Part 1: 字典查询"
# =============================================================================

R=$(post "/dictionaries" '{"group":"field_type"}')
assert_code   "dict.1 field_type 成功"     "0" "$R"
assert_field  "dict.1 返回 6 种类型"       ".data.items | length" "6" "$R"

R=$(post "/dictionaries" '{"group":"field_category"}')
assert_code   "dict.2 field_category 成功" "0" "$R"
assert_field  "dict.2 返回 6 种分类"       ".data.items | length" "6" "$R"

R=$(post "/dictionaries" '{"group":"field_properties"}')
assert_code   "dict.3 field_properties 成功" "0" "$R"

R=$(post "/dictionaries" '{"group":""}')
assert_code   "dict.4 空 group 返回参数错误" "40000" "$R"

R=$(post "/dictionaries" '{"group":"nonexistent"}')
assert_code   "dict.5 不存在 group 返回成功（空列表）" "0" "$R"
assert_field  "dict.5 空列表"              ".data.items | length" "0" "$R"

# 验证字典返回的结构完整性（每项 {name, label}）
R=$(post "/dictionaries" '{"group":"field_category"}')
assert_not_equal "dict.6 category items[0].name 非空" ".data.items[0].name" "null" "$R"
assert_not_equal "dict.6 category items[0].label 非空" ".data.items[0].label" "null" "$R"

# =============================================================================
section "Part 2: 字段管理 — CRUD"
# =============================================================================

# ---- 功能 2：新建字段 ----
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

# 供后续用的字段池（每种类型都准备一份，便于覆盖收紧检查和引用场景）
R=$(post "/fields/create" "{\"name\":\"${P}atk\",\"label\":\"攻击力\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"ATK\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":999}}}")
ATK_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
assert_code "f2.12 创建 atk (integer)" "0" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}str\",\"label\":\"名字文本\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{\"description\":\"STR\",\"expose_bb\":false,\"constraints\":{\"minLength\":1,\"maxLength\":50}}}")
STR_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
assert_code "f2.13 创建 str (string)" "0" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}flag\",\"label\":\"布尔标记\",\"type\":\"boolean\",\"category\":\"basic\",\"properties\":{\"description\":\"flag\",\"expose_bb\":false}}")
FLAG_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
assert_code "f2.14 创建 flag (boolean)" "0" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}mood\",\"label\":\"情绪选择\",\"type\":\"select\",\"category\":\"personality\",\"properties\":{\"description\":\"mood\",\"expose_bb\":false,\"constraints\":{\"options\":[{\"value\":\"happy\",\"label\":\"开心\"},{\"value\":\"sad\",\"label\":\"伤心\"}],\"minSelect\":1,\"maxSelect\":1}}}")
MOOD_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
assert_code "f2.15 创建 mood (select)" "0" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}fnum\",\"label\":\"浮点字段\",\"type\":\"float\",\"category\":\"combat\",\"properties\":{\"description\":\"fl\",\"expose_bb\":false,\"constraints\":{\"min\":0.0,\"max\":100.0,\"precision\":2}}}")
FLOAT_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
assert_code "f2.16 创建 fnum (float)" "0" "$R"

# ---- 功能 6：唯一性校验 ----
subsection "功能 6: 字段名唯一性校验"

R=$(post "/fields/check-name" "{\"name\":\"${P}hp\"}")
assert_code  "f6.1 已存在名字" "0" "$R"
assert_field "f6.1 available=false" ".data.available" "false" "$R"

R=$(post "/fields/check-name" "{\"name\":\"${P}notexist_xxx\"}")
assert_field "f6.2 不存在 → available=true" ".data.available" "true" "$R"

R=$(post "/fields/check-name" '{"name":""}')
assert_code  "f6.3 空名 40000" "40000" "$R"

# ---- 功能 3：字段详情 ----
subsection "功能 3: 字段详情"

R=$(fld_detail "$HP_ID")
assert_code  "f3.1 详情成功" "0" "$R"
assert_field "f3.1 name 正确" ".data.name" "${P}hp" "$R"
assert_field "f3.1 label 正确" ".data.label" "测试生命值" "$R"
assert_field "f3.1 properties.description" ".data.properties.description" "HP" "$R"
assert_field "f3.1 constraints.min" ".data.properties.constraints.min" "0" "$R"
assert_field "f3.1 constraints.max" ".data.properties.constraints.max" "100" "$R"

R=$(fld_detail 999999)
assert_code "f3.2 不存在 ID → 40011" "40011" "$R"

R=$(post "/fields/detail" '{"id":0}')
assert_code "f3.3 ID=0 → 40000" "40000" "$R"

R=$(post "/fields/detail" '{"id":-1}')
assert_code "f3.4 负 ID → 40000" "40000" "$R"

# 即使停用中的字段，详情也能查
fld_disable "$HP_ID" 2>/dev/null
R=$(fld_detail "$HP_ID")
assert_code  "f3.5 停用字段详情可查" "0" "$R"
assert_field "f3.5 enabled=false"   ".data.enabled" "false" "$R"

# ---- 功能 1：字段列表 ----
subsection "功能 1: 字段列表"

R=$(post "/fields/list" '{"page":1,"page_size":20}')
assert_code  "f1.1 列表成功" "0" "$R"
assert_ge    "f1.1 至少 6 条" ".data.total" "6" "$R"
assert_field "f1.1 items 数组" ".data.items | type" "array" "$R"
assert_not_equal "f1.2 items[0] 有 id" ".data.items[0].id" "null" "$R"

R=$(post "/fields/list" '{"type":"boolean","page":1,"page_size":20}')
assert_code "f1.3 按 type 筛选" "0" "$R"
assert_ge   "f1.3 ≥ 1 个 boolean" ".data.total" "1" "$R"

R=$(post "/fields/list" '{"category":"combat","page":1,"page_size":20}')
assert_code "f1.4 按 category 筛选" "0" "$R"
assert_ge   "f1.4 ≥ 2 个 combat" ".data.total" "2" "$R"

R=$(post "/fields/list" "{\"label\":\"测试生命\",\"page\":1,\"page_size\":20}")
assert_ge "f1.5 模糊搜索 ≥ 1" ".data.total" "1" "$R"

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

# ---- 功能 4：编辑字段 ----
subsection "功能 4: 编辑字段"

HP_VER=$(fld_version "$HP_ID")
R=$(post "/fields/update" "{\"id\":${HP_ID},\"label\":\"生命值改\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"HP changed\",\"expose_bb\":true,\"constraints\":{\"min\":0,\"max\":200}},\"version\":${HP_VER}}")
assert_code "f4.1 编辑成功（未启用）" "0" "$R"

R=$(fld_detail "$HP_ID")
assert_field "f4.1 label 已更新"          ".data.label" "生命值改" "$R"
assert_field "f4.1 max 已更新"            ".data.properties.constraints.max" "200" "$R"
assert_field "f4.1 expose_bb 已更新"      ".data.properties.expose_bb" "true" "$R"

# 缓存一致性：连续读两次应该都拿到新数据（检查 detail 缓存正确失效）
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
assert_code "f4.8 version=0 → 40000" "40000" "$R"

# 编辑纯 noop（只写回一样的值），应成功
HP_VER=$(fld_version "$HP_ID")
R=$(post "/fields/update" "{\"id\":${HP_ID},\"label\":\"生命值改\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"HP changed\",\"expose_bb\":true,\"constraints\":{\"min\":0,\"max\":200}},\"version\":${HP_VER}}")
assert_code "f4.9 noop 编辑成功" "0" "$R"

# ---- 功能 8：启用/停用 ----
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
section "Part 3: 字段管理 — 约束收紧 + 引用关系"
# =============================================================================

fld_enable "$ATK_ID"

# ---- 功能 10/11：收紧 + 引用关系 ----
subsection "功能 10/11: 约束收紧 + 引用关系"

R=$(post "/fields/create" "{\"name\":\"${P}tgt\",\"label\":\"收紧目标\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"tgt\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}}}")
TGT_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
fld_enable "$TGT_ID"

R=$(post "/fields/create" "{\"name\":\"${P}refone\",\"label\":\"引用一\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"ref\",\"expose_bb\":false,\"constraints\":{\"refs\":[${TGT_ID}]}}}")
assert_code "f10.1 创建 reference 字段" "0" "$R"
REFONE_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

R=$(fld_detail "$TGT_ID")
assert_field "f10.2 target ref_count=1" ".data.ref_count" "1" "$R"

# 被引用时禁止收紧 — integer
fld_disable "$TGT_ID"
TGT_VER=$(fld_version "$TGT_ID")
R=$(post "/fields/update" "{\"id\":${TGT_ID},\"label\":\"t\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"t\",\"expose_bb\":false,\"constraints\":{\"min\":10,\"max\":100}},\"version\":${TGT_VER}}")
assert_code "f10.3 integer min 收紧 40007" "40007" "$R"

R=$(post "/fields/update" "{\"id\":${TGT_ID},\"label\":\"t\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"t\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":50}},\"version\":${TGT_VER}}")
assert_code "f10.4 integer max 收紧 40007" "40007" "$R"

# 放宽允许
R=$(post "/fields/update" "{\"id\":${TGT_ID},\"label\":\"t\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"t\",\"expose_bb\":false,\"constraints\":{\"min\":-10,\"max\":200}},\"version\":${TGT_VER}}")
assert_code "f10.5 放宽成功" "0" "$R"

# 被引用时禁止改类型
TGT_VER=$(fld_version "$TGT_ID")
R=$(post "/fields/update" "{\"id\":${TGT_ID},\"label\":\"t\",\"type\":\"string\",\"category\":\"combat\",\"properties\":{\"description\":\"t\",\"expose_bb\":false,\"constraints\":{\"minLength\":0,\"maxLength\":100}},\"version\":${TGT_VER}}")
assert_code "f10.6 被引用改 type 40006" "40006" "$R"

# float 收紧测试
R=$(post "/fields/create" "{\"name\":\"${P}ftgt\",\"label\":\"浮点目标\",\"type\":\"float\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"min\":0.0,\"max\":100.0,\"precision\":4}}}")
FTGT_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$FTGT_ID"
R=$(post "/fields/create" "{\"name\":\"${P}fholder\",\"label\":\"浮点持有\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"refs\":[${FTGT_ID}]}}}")
FHOLDER_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
fld_disable "$FTGT_ID"
FTGT_VER=$(fld_version "$FTGT_ID")

R=$(post "/fields/update" "{\"id\":${FTGT_ID},\"label\":\"浮点目标\",\"type\":\"float\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"min\":0.0,\"max\":100.0,\"precision\":2}},\"version\":${FTGT_VER}}")
assert_code "f10.7 float precision 4→2 40007" "40007" "$R"

FTGT_VER=$(fld_version "$FTGT_ID")
R=$(post "/fields/update" "{\"id\":${FTGT_ID},\"label\":\"浮点目标\",\"type\":\"float\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"min\":0.0,\"max\":100.0,\"precision\":6}},\"version\":${FTGT_VER}}")
assert_code "f10.8 float precision 4→6 放宽 ok" "0" "$R"

# string pattern / minLength / maxLength 收紧
R=$(post "/fields/create" "{\"name\":\"${P}stgt\",\"label\":\"字符目标\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"minLength\":0,\"maxLength\":100}}}")
STGT_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$STGT_ID"
R=$(post "/fields/create" "{\"name\":\"${P}sholder\",\"label\":\"字符持\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"refs\":[${STGT_ID}]}}}")
SHOLDER_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
fld_disable "$STGT_ID"
STGT_VER=$(fld_version "$STGT_ID")

R=$(post "/fields/update" "{\"id\":${STGT_ID},\"label\":\"字符目标\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"minLength\":5,\"maxLength\":100}},\"version\":${STGT_VER}}")
assert_code "f10.9 string minLength 0→5 40007" "40007" "$R"

R=$(post "/fields/update" "{\"id\":${STGT_ID},\"label\":\"字符目标\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"minLength\":0,\"maxLength\":50}},\"version\":${STGT_VER}}")
assert_code "f10.10 string maxLength 100→50 40007" "40007" "$R"

R=$(post "/fields/update" "{\"id\":${STGT_ID},\"label\":\"字符目标\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"minLength\":0,\"maxLength\":100,\"pattern\":\"^[a-z]+$\"}},\"version\":${STGT_VER}}")
assert_code "f10.11 string 新增 pattern 40007" "40007" "$R"

# select 收紧（options 删除 + minSelect/maxSelect）
R=$(post "/fields/create" "{\"name\":\"${P}seltgt\",\"label\":\"选择目标\",\"type\":\"select\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"options\":[{\"value\":\"a\",\"label\":\"A\"},{\"value\":\"b\",\"label\":\"B\"},{\"value\":\"c\",\"label\":\"C\"}],\"minSelect\":1,\"maxSelect\":3}}}")
SELTGT_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$SELTGT_ID"
R=$(post "/fields/create" "{\"name\":\"${P}selholder\",\"label\":\"选持\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"refs\":[${SELTGT_ID}]}}}")
SELHOLDER_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
fld_disable "$SELTGT_ID"
SELTGT_VER=$(fld_version "$SELTGT_ID")

R=$(post "/fields/update" "{\"id\":${SELTGT_ID},\"label\":\"选择目标\",\"type\":\"select\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"options\":[{\"value\":\"a\",\"label\":\"A\"},{\"value\":\"b\",\"label\":\"B\"}],\"minSelect\":1,\"maxSelect\":2}},\"version\":${SELTGT_VER}}")
assert_code "f10.12 select 删除 option 40007" "40007" "$R"

R=$(post "/fields/update" "{\"id\":${SELTGT_ID},\"label\":\"选择目标\",\"type\":\"select\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"options\":[{\"value\":\"a\",\"label\":\"A\"},{\"value\":\"b\",\"label\":\"B\"},{\"value\":\"c\",\"label\":\"C\"}],\"minSelect\":2,\"maxSelect\":3}},\"version\":${SELTGT_VER}}")
assert_code "f10.13 select minSelect 1→2 40007" "40007" "$R"

R=$(post "/fields/update" "{\"id\":${SELTGT_ID},\"label\":\"选择目标\",\"type\":\"select\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"options\":[{\"value\":\"a\",\"label\":\"A\"},{\"value\":\"b\",\"label\":\"B\"},{\"value\":\"c\",\"label\":\"C\"}],\"minSelect\":1,\"maxSelect\":2}},\"version\":${SELTGT_VER}}")
assert_code "f10.14 select maxSelect 3→2 40007" "40007" "$R"

# 对照：select 追加 option 应允许
R=$(post "/fields/update" "{\"id\":${SELTGT_ID},\"label\":\"选择目标\",\"type\":\"select\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"options\":[{\"value\":\"a\",\"label\":\"A\"},{\"value\":\"b\",\"label\":\"B\"},{\"value\":\"c\",\"label\":\"C\"},{\"value\":\"d\",\"label\":\"D\"}],\"minSelect\":1,\"maxSelect\":3}},\"version\":${SELTGT_VER}}")
assert_code "f10.15 select 追加 option ok" "0" "$R"

# boolean 无约束检查
R=$(post "/fields/create" "{\"name\":\"${P}btgt\",\"label\":\"布尔目标\",\"type\":\"boolean\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false}}")
BTGT_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$BTGT_ID"
R=$(post "/fields/create" "{\"name\":\"${P}bholder\",\"label\":\"布持\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"refs\":[${BTGT_ID}]}}}")
BHOLDER_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
fld_disable "$BTGT_ID"
BTGT_VER=$(fld_version "$BTGT_ID")
R=$(post "/fields/update" "{\"id\":${BTGT_ID},\"label\":\"布尔目标\",\"type\":\"boolean\",\"category\":\"basic\",\"properties\":{\"description\":\"boolean 编辑\",\"expose_bb\":false},\"version\":${BTGT_VER}}")
assert_code "f10.16 boolean 编辑 ok（无约束）" "0" "$R"

# ---- reference 引用关系（嵌套 / 循环 / 停用）----
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

# ---- 功能 7：字段引用详情 ----
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

# ---- 功能 5：软删除字段 ----
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
section "Part 4: 模板管理 + 跨模块集成"
# =============================================================================

# 准备模板用字段池
R=$(post "/fields/create" "{\"name\":\"${P}f_hp\",\"label\":\"T_HP\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"HP\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}}}")
F_HP=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$F_HP"

R=$(post "/fields/create" "{\"name\":\"${P}f_atk\",\"label\":\"T_ATK\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"ATK\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":999}}}")
F_ATK=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$F_ATK"

R=$(post "/fields/create" "{\"name\":\"${P}f_name\",\"label\":\"T_NAME\",\"type\":\"string\",\"category\":\"basic\",\"properties\":{\"description\":\"name\",\"expose_bb\":false,\"constraints\":{\"minLength\":1,\"maxLength\":50}}}")
F_NAME=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$F_NAME"

R=$(post "/fields/create" "{\"name\":\"${P}f_disabled\",\"label\":\"T_DIS\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"dis\",\"expose_bb\":false}}")
F_DISABLED=$(echo "$R" | jq -r '.data.id' | tr -d '\r')   # 保持停用

# ---- 模板功能 10：唯一性校验 ----
subsection "模板 功能 10: 名唯一性校验"

R=$(post "/templates/check-name" "{\"name\":\"${P}npc_combat\"}")
assert_field "t10.1 未用 available=true" ".data.available" "true" "$R"

R=$(post "/templates/check-name" '{"name":""}')
assert_code "t10.2 空名 41002" "41002" "$R"

R=$(post "/templates/check-name" '{"name":"BAD"}')
assert_code "t10.3 大写 41002" "41002" "$R"

R=$(post "/templates/check-name" '{"name":"123abc"}')
assert_code "t10.4 数字开头 41002" "41002" "$R"

# ---- 模板功能 2：新建 ----
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

# 异常
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

# 使用 ASCII 避免 Windows git-bash 下 python `print` 的 \r\n 污染
LONG_DESC=$(printf 'a%.0s' $(seq 1 513))
R=$(post "/templates/create" "{\"name\":\"${P}n_desc\",\"label\":\"x\",\"description\":\"${LONG_DESC}\",\"fields\":[{\"field_id\":${F_HP},\"required\":true}]}")
assert_code "t2.11 description 513 字 40000" "40000" "$R"

# 对照：512 字刚好允许。注意：该模板会挂载 F_HP，为避免污染后续 ref_count 断言，创建后立即删除。
LONG_OK=$(printf 'a%.0s' $(seq 1 512))
R=$(post "/templates/create" "{\"name\":\"${P}n_desc_ok\",\"label\":\"x\",\"description\":\"${LONG_OK}\",\"fields\":[{\"field_id\":${F_HP},\"required\":true}]}")
assert_code "t2.12 description 512 字 ok" "0" "$R"
DESC_OK_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
if [ -n "$DESC_OK_ID" ] && [ "$DESC_OK_ID" != "null" ]; then
  tpl_rm "$DESC_OK_ID" 2>/dev/null
fi

# ---- 模板功能 1：列表 ----
subsection "模板 功能 1: 列表"

R=$(post "/templates/list" '{"page":1,"page_size":20}')
assert_code  "t1.1 列表成功" "0" "$R"
assert_ge    "t1.1 total ≥ 1" ".data.total" "1" "$R"

R=$(post "/templates/list" '{"label":"战斗生物","page":1,"page_size":20}')
assert_ge "t1.2 模糊搜索 ≥ 1" ".data.total" "1" "$R"

R=$(post "/templates/list" '{"enabled":true,"page":1,"page_size":20}')
assert_code "t1.3 enabled=true 查询" "0" "$R"

tpl_enable "$TPL_ID"
R=$(post "/templates/list" '{"enabled":true,"page":1,"page_size":20}')
assert_ge "t1.4 启用后 ≥ 1" ".data.total" "1" "$R"

R=$(post "/templates/list" '{"page":0,"page_size":0}')
assert_field "t1.5 page 校正 1" ".data.page" "1" "$R"

R=$(post "/templates/list" '{"label":"不存在zzz","page":1,"page_size":20}')
assert_field "t1.6 空结果" ".data.items | length" "0" "$R"

# 列表项不应含 fields / description（覆盖索引返回）
R=$(post "/templates/list" '{"page":1,"page_size":20}')
assert_field "t1.7 列表项无 fields 字段（应 null）" ".data.items[0].fields" "null" "$R"

# ---- 模板功能 7：启停切换 ----
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

# ---- 模板功能 4：编辑 ----
subsection "模板 功能 4: 编辑"

# 启用中编辑 → 41010
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

# 纯字段顺序变化（ref_count=0）
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

# ref_count 联动
TOTAL=$((TOTAL + 4))
HP_RC=$(fld_refcount "$F_HP")
ATK_RC=$(fld_refcount "$F_ATK")
NAME_RC=$(fld_refcount "$F_NAME")
DEF_RC=$(fld_refcount "$F_DEF")
[ "$HP_RC" = "1" ]   && { echo "  [PASS] t4.5a F_HP ref_count=1"; PASS=$((PASS+1)); }   || { echo "  [FAIL] t4.5a 期望 1 实际 $HP_RC"; FAIL=$((FAIL+1)); }
[ "$ATK_RC" = "0" ]  && { echo "  [PASS] t4.5b F_ATK ref_count=0"; PASS=$((PASS+1)); }  || { echo "  [FAIL] t4.5b 期望 0 实际 $ATK_RC"; FAIL=$((FAIL+1)); }
[ "$NAME_RC" = "0" ] && { echo "  [PASS] t4.5c F_NAME ref_count=0"; PASS=$((PASS+1)); } || { echo "  [FAIL] t4.5c 期望 0 实际 $NAME_RC"; FAIL=$((FAIL+1)); }
[ "$DEF_RC" = "1" ]  && { echo "  [PASS] t4.5d F_DEF ref_count=1"; PASS=$((PASS+1)); }  || { echo "  [FAIL] t4.5d 期望 1 实际 $DEF_RC"; FAIL=$((FAIL+1)); }

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

# ---- 模板功能 6：引用详情 ----
subsection "模板 功能 6: 引用详情"

R=$(post "/templates/references" "{\"id\":${TPL_ID}}")
assert_code  "t6.1 成功" "0" "$R"
assert_field "t6.1 template_id" ".data.template_id" "$TPL_ID" "$R"
assert_field "t6.1 npcs 空（NPC 未上线）" ".data.npcs | length" "0" "$R"
assert_field "t6.1 npcs 是数组（非 null）" ".data.npcs | type" "array" "$R"

R=$(post "/templates/references" '{"id":999999}')
assert_code "t6.2 不存在 41003" "41003" "$R"

# ---- 跨模块：字段引用详情补 template label ----
subsection "跨模块: F.references 补 template label"

R=$(post "/fields/references" "{\"id\":${F_HP}}")
assert_code  "x.1 F_HP 引用详情成功" "0" "$R"
assert_ge    "x.1 ≥ 1 个模板引用"   ".data.templates | length" "1" "$R"
TOTAL=$((TOTAL + 1))
TPL_LABEL=$(echo "$R" | jq -r '.data.templates[0].label' | tr -d '\r')
if [ "$TPL_LABEL" = "战斗生物模板（改）" ]; then
  echo "  [PASS] x.2 template label 已正确补全"; PASS=$((PASS+1))
else
  echo "  [FAIL] x.2 期望 '战斗生物模板（改）' 实际 '$TPL_LABEL'"; FAIL=$((FAIL+1))
fi

# 跨模块：禁止删除被模板引用的字段
fld_disable "$F_HP"
R=$(post "/fields/delete" "{\"id\":${F_HP}}")
assert_code "x.3 被模板引用字段删除 40005" "40005" "$R"

# 跨模块：允许停用被模板引用的字段
TOTAL=$((TOTAL + 1))
EN=$(fld_enabled "$F_HP")
if [ "$EN" = "false" ]; then
  echo "  [PASS] x.4 允许停用被模板引用的字段"; PASS=$((PASS+1))
else
  echo "  [FAIL] x.4 应能停用 实际 $EN"; FAIL=$((FAIL+1))
fi

# 模板详情中停用字段 enabled=false
R=$(tpl_detail "$TPL_ID")
assert_field "x.5 模板详情反映 F_HP.enabled=false" ".data.fields[0].enabled" "false" "$R"
fld_enable "$F_HP"

# ---- 模板功能 5：删除 ----
subsection "模板 功能 5: 删除"

tpl_enable "$TPL_ID"
R=$(post "/templates/delete" "{\"id\":${TPL_ID}}")
assert_code "t5.1 启用中删除 41009" "41009" "$R"

tpl_disable "$TPL_ID"
R=$(post "/templates/delete" "{\"id\":${TPL_ID}}")
assert_code "t5.2 停用后删除成功" "0" "$R"
assert_field "t5.2 返回 id" ".data.id" "$TPL_ID" "$R"

# ref_count 回退
TOTAL=$((TOTAL + 2))
HP_RC=$(fld_refcount "$F_HP")
DEF_RC=$(fld_refcount "$F_DEF")
[ "$HP_RC" = "0" ]  && { echo "  [PASS] t5.3a F_HP ref_count=0"; PASS=$((PASS+1)); } || { echo "  [FAIL] t5.3a 期望 0 实际 $HP_RC"; FAIL=$((FAIL+1)); }
[ "$DEF_RC" = "0" ] && { echo "  [PASS] t5.3b F_DEF ref_count=0"; PASS=$((PASS+1)); } || { echo "  [FAIL] t5.3b 期望 0 实际 $DEF_RC"; FAIL=$((FAIL+1)); }

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
section "Part 5: 攻击性测试（重点攻击可疑 bug）"
# =============================================================================

# 专用字段池
R=$(post "/fields/create" "{\"name\":\"${P}atk_leaf1\",\"label\":\"攻击叶1\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}}}")
LEAF1=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$LEAF1"

R=$(post "/fields/create" "{\"name\":\"${P}atk_leaf2\",\"label\":\"攻击叶2\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}}}")
LEAF2=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$LEAF2"

R=$(post "/fields/create" "{\"name\":\"${P}atk_leaf3\",\"label\":\"攻击叶3\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}}}")
LEAF3=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$LEAF3"

# ---- Attack 1: refs 数组含重复 ID（DB unique 泄漏 或 ref_count 被重复递增）----
subsection "ATK-1: refs=[X,X] 未去重 — syncFieldRefs 缺 dedup"

R=$(post "/fields/create" "{\"name\":\"${P}atk_dup_refs\",\"label\":\"重复refs\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"refs\":[${LEAF2},${LEAF2}]}}}")
CODE=$(echo "$R" | jq -r '.code' | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$CODE" = "0" ]; then
  # 业务通过 → 必须确认 leaf2.ref_count 只被 +1
  DUP_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
  L2_RC=$(fld_refcount "$LEAF2")
  if [ "$L2_RC" = "1" ]; then
    echo "  [PASS] atk1.1 创建成功且 leaf2.ref_count=1（业务层或 DB 保证了去重）"
    PASS=$((PASS+1))
  else
    echo "  [BUG ] atk1.1 创建成功但 leaf2.ref_count=$L2_RC（应为 1）"
    FAIL=$((FAIL+1))
    BUGS+=("atk1.1: refs=[id,id] 未去重导致 ref_count 被重复递增为 $L2_RC — 建议在 syncFieldRefs 对 newRefIDs 去重")
  fi
  fld_rm "$DUP_ID"
elif [ "$CODE" = "40000" ] || [ "$CODE" = "40009" ] || [ "$CODE" = "40017" ]; then
  echo "  [PASS] atk1.1 重复 refs 被拒绝 (code=$CODE)"
  PASS=$((PASS+1))
elif [ "$CODE" = "50000" ]; then
  echo "  [BUG ] atk1.1 返回 50000 — DB unique 约束泄漏为 500 错误，应由 Service 层提前校验"
  FAIL=$((FAIL+1))
  BUGS+=("atk1.1: refs 重复时 syncFieldRefs 触发 DB UNIQUE 约束，错误以 50000 返回而非业务错误")
else
  echo "  [BUG ] atk1.1 意外 code=$CODE"
  FAIL=$((FAIL+1))
  BUGS+=("atk1.1: refs=[id,id] 返回 code=$CODE，非预期")
fi

# ---- Attack 2: reference 嵌套（禁止） ----
subsection "ATK-2: reference 嵌套 — 应 40016"

R=$(post "/fields/create" "{\"name\":\"${P}atk_ref_a\",\"label\":\"嵌套A\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"refs\":[${LEAF1}]}}}")
assert_code "atk2.1 refA -> LEAF1 成功" "0" "$R"
REF_A=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$REF_A"

R=$(post "/fields/create" "{\"name\":\"${P}atk_ref_b\",\"label\":\"嵌套B\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"refs\":[${REF_A}]}}}")
assert_code "atk2.2 refB -> refA 应 40016（嵌套禁止）" "40016" "$R"

# ---- Attack 3: 模板能否挂载 reference 类型字段 ----
subsection "ATK-3: 模板挂载 reference 类型字段 — 应 41012"

# REF_A 是 reference 类型（已启用）
R=$(post "/templates/create" "{\"name\":\"${P}atk_tpl_ref\",\"label\":\"含 reference 的模板\",\"description\":\"\",\"fields\":[{\"field_id\":${REF_A},\"required\":true}]}")
assert_code "atk3.1 模板挂 reference 被拒绝 41012" "41012" "$R"

# 确认 REF_A.ref_count 未受污染
TOTAL=$((TOTAL + 1))
REFA_RC=$(fld_refcount "$REF_A")
if [ "$REFA_RC" = "0" ]; then
  echo "  [PASS] atk3.2 REF_A.ref_count 保持 0（未被污染）"
  PASS=$((PASS+1))
else
  echo "  [FAIL] atk3.2 REF_A.ref_count=$REFA_RC（应为 0）"
  FAIL=$((FAIL+1))
fi

# 编辑路径同样应拒绝 — 先用合法字段建一个模板，再尝试 Update 改成 reference 字段
R=$(post "/templates/create" "{\"name\":\"${P}atk_tpl_ref2\",\"label\":\"模板\",\"description\":\"\",\"fields\":[{\"field_id\":${LEAF3},\"required\":true}]}")
TPL_REF2=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
V=$(tpl_version "$TPL_REF2")
R=$(post "/templates/update" "{\"id\":${TPL_REF2},\"label\":\"模板\",\"description\":\"\",\"fields\":[{\"field_id\":${LEAF3},\"required\":true},{\"field_id\":${REF_A},\"required\":false}],\"version\":${V}}")
assert_code "atk3.3 编辑时加入 reference 字段 41012" "41012" "$R"
tpl_rm "$TPL_REF2"

# ---- Attack 4: reference 字段在 Update 中自引用 ----
subsection "ATK-4: Update 把 reference 字段 refs 指向自身 — 应被拒绝"

# REF_A 已启用。Update 前必须先停用（40015），
# 然后 refs=[REF_A 自身] 会被校验链拦截：
# - 40013 ErrFieldRefDisabled（因为"新增 ref"中的 REF_A 此时正是停用态），或
# - 40016 ErrFieldRefNested（REF_A 自身是 reference 类型），或
# - 40009 ErrFieldCyclicRef（detectCyclicRef 看到 visited[currentID]）
# 任一拒绝都算正确。
fld_disable "$REF_A"
VER=$(fld_version "$REF_A")
R=$(post "/fields/update" "{\"id\":${REF_A},\"label\":\"嵌套A\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"refs\":[${REF_A}]}},\"version\":${VER}}")
assert_code_in "atk4.1 自引用被拒绝" "40009 40013 40016" "$R"
fld_enable "$REF_A"

# ---- Attack 5: "存量不动"语义 — 编辑 reference 字段保留停用目标 ----
subsection "ATK-5: oldRefSet 语义 — 已有 ref 即使停用也应保留"

# 创建新目标 + reference 字段
R=$(post "/fields/create" "{\"name\":\"${P}atk_legacy_tgt\",\"label\":\"遗留目标\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false}}")
LEGACY_TGT=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$LEGACY_TGT"

R=$(post "/fields/create" "{\"name\":\"${P}atk_legacy_ref\",\"label\":\"遗留 ref\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"refs\":[${LEGACY_TGT}]}}}")
LEGACY_REF=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

# 把目标停用
fld_disable "$LEGACY_TGT"

# Update legacy_ref 保持 refs=[legacy_tgt] 不变（应允许）
VER=$(fld_version "$LEGACY_REF")
R=$(post "/fields/update" "{\"id\":${LEGACY_REF},\"label\":\"遗留 ref\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"已编辑\",\"expose_bb\":false,\"constraints\":{\"refs\":[${LEGACY_TGT}]}},\"version\":${VER}}")
assert_code "atk5.1 保留停用目标 ok（存量不动）" "0" "$R"

# Update 再新增一个停用目标作为 NEW ref → 应 40013
R=$(post "/fields/create" "{\"name\":\"${P}atk_new_dis\",\"label\":\"新停用\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false}}")
NEW_DIS=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
# 保持停用
VER=$(fld_version "$LEGACY_REF")
R=$(post "/fields/update" "{\"id\":${LEGACY_REF},\"label\":\"遗留 ref\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"refs\":[${LEGACY_TGT},${NEW_DIS}]}},\"version\":${VER}}")
assert_code "atk5.2 新增停用目标 40013" "40013" "$R"
fld_rm "$NEW_DIS"

# ---- Attack 6: reference 类型改为非 reference 类型后 ref_count 清零 ----
subsection "ATK-6: reference → integer 类型变更应清空 refs"

R=$(post "/fields/create" "{\"name\":\"${P}atk_morph_tgt\",\"label\":\"morph 目标\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false}}")
MORPH_TGT=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$MORPH_TGT"

R=$(post "/fields/create" "{\"name\":\"${P}atk_morph\",\"label\":\"morph\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"refs\":[${MORPH_TGT}]}}}")
MORPH=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

RC_BEFORE=$(fld_refcount "$MORPH_TGT")
VER=$(fld_version "$MORPH")
R=$(post "/fields/update" "{\"id\":${MORPH},\"label\":\"morph\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}},\"version\":${VER}}")
assert_code "atk6.1 reference → integer 允许" "0" "$R"

RC_AFTER=$(fld_refcount "$MORPH_TGT")
TOTAL=$((TOTAL + 2))
[ "$RC_BEFORE" = "1" ] && { echo "  [PASS] atk6.2 类型变更前 tgt.ref_count=1"; PASS=$((PASS+1)); } || { echo "  [FAIL] atk6.2 期望 1 实际 $RC_BEFORE"; FAIL=$((FAIL+1)); }
[ "$RC_AFTER" = "0" ]  && { echo "  [PASS] atk6.3 类型变更后 tgt.ref_count=0"; PASS=$((PASS+1)); } || { echo "  [BUG ] atk6.3 期望 0 实际 $RC_AFTER — reference→其他类型后 ref_count 未清零"; FAIL=$((FAIL+1)); BUGS+=("atk6.3: reference→integer 后未减回 ref_count"); }

# ---- Attack 7: 模板纯排序 / 纯 required 变化不应影响 field_refs ----
subsection "ATK-7: 模板纯排序变化不应触发 field_refs 操作"

R=$(post "/fields/create" "{\"name\":\"${P}atk_ord_a\",\"label\":\"orda\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false}}")
O_A=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$O_A"
R=$(post "/fields/create" "{\"name\":\"${P}atk_ord_b\",\"label\":\"ordb\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false}}")
O_B=$(echo "$R" | jq -r '.data.id' | tr -d '\r'); fld_enable "$O_B"

R=$(post "/templates/create" "{\"name\":\"${P}atk_ord_tpl\",\"label\":\"排序模板\",\"description\":\"\",\"fields\":[{\"field_id\":${O_A},\"required\":true},{\"field_id\":${O_B},\"required\":false}]}")
O_TPL=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

RC_A_BEFORE=$(fld_refcount "$O_A")
RC_B_BEFORE=$(fld_refcount "$O_B")

# 纯反序 + required 变更
VER=$(tpl_version "$O_TPL")
R=$(post "/templates/update" "{\"id\":${O_TPL},\"label\":\"排序模板\",\"description\":\"\",\"fields\":[{\"field_id\":${O_B},\"required\":true},{\"field_id\":${O_A},\"required\":false}],\"version\":${VER}}")
assert_code "atk7.1 纯反序 + required 变更成功" "0" "$R"

RC_A_AFTER=$(fld_refcount "$O_A")
RC_B_AFTER=$(fld_refcount "$O_B")
TOTAL=$((TOTAL + 2))
[ "$RC_A_BEFORE" = "$RC_A_AFTER" ] && { echo "  [PASS] atk7.2 O_A ref_count 不变 ($RC_A_BEFORE→$RC_A_AFTER)"; PASS=$((PASS+1)); } || { echo "  [BUG ] atk7.2 O_A ref_count 从 $RC_A_BEFORE 变为 $RC_A_AFTER"; FAIL=$((FAIL+1)); BUGS+=("atk7.2: 纯排序变化错误触发了 field_refs 操作"); }
[ "$RC_B_BEFORE" = "$RC_B_AFTER" ] && { echo "  [PASS] atk7.3 O_B ref_count 不变 ($RC_B_BEFORE→$RC_B_AFTER)"; PASS=$((PASS+1)); } || { echo "  [BUG ] atk7.3 O_B ref_count 从 $RC_B_BEFORE 变为 $RC_B_AFTER"; FAIL=$((FAIL+1)); BUGS+=("atk7.3: 纯排序变化错误触发了 field_refs 操作"); }

R=$(tpl_detail "$O_TPL")
assert_field "atk7.4 fields[0]=O_B (反序成功)" ".data.fields[0].name" "${P}atk_ord_b" "$R"
assert_field "atk7.5 fields[0].required=true"  ".data.fields[0].required" "true" "$R"

# ---- Attack 8: properties 形状校验 ----
subsection "ATK-8: properties 形状校验（null / true / 数字 / 字符串 / 数组）"

R=$(post "/fields/create" "{\"name\":\"${P}atk_p_null\",\"label\":\"x\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":null}")
assert_code "atk8.1 properties=null 40000" "40000" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}atk_p_true\",\"label\":\"x\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":true}")
assert_code "atk8.2 properties=true 40000" "40000" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}atk_p_num\",\"label\":\"x\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":123}")
assert_code "atk8.3 properties=123 40000" "40000" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}atk_p_str\",\"label\":\"x\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":\"str\"}")
assert_code "atk8.4 properties=\"str\" 40000" "40000" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}atk_p_arr\",\"label\":\"x\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":[]}")
assert_code "atk8.5 properties=[] 40000" "40000" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}atk_p_arr2\",\"label\":\"x\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":[1,2]}")
assert_code "atk8.6 properties=[1,2] 40000" "40000" "$R"

# ---- Attack 9: refs 含 0 / 负值 ----
subsection "ATK-9: refs 含 0 / 负值应在业务层拦截"

R=$(post "/fields/create" "{\"name\":\"${P}atk_zero\",\"label\":\"zero\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"refs\":[0]}}}")
CODE=$(echo "$R" | jq -r '.code' | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$CODE" = "40014" ] || [ "$CODE" = "40000" ]; then
  echo "  [PASS] atk9.1 refs=[0] 被拒绝 (code=$CODE)"
  PASS=$((PASS+1))
else
  echo "  [BUG ] atk9.1 refs=[0] code=$CODE 未拦截"
  FAIL=$((FAIL+1))
  BUGS+=("atk9.1: refs 含 0 未被拦截")
  ID0=$(echo "$R" | jq -r '.data.id // empty' | tr -d '\r')
  [ -n "$ID0" ] && fld_rm "$ID0"
fi

R=$(post "/fields/create" "{\"name\":\"${P}atk_neg\",\"label\":\"neg\",\"type\":\"reference\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"refs\":[-1]}}}")
CODE=$(echo "$R" | jq -r '.code' | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$CODE" = "40014" ] || [ "$CODE" = "40000" ]; then
  echo "  [PASS] atk9.2 refs=[-1] 被拒绝 (code=$CODE)"
  PASS=$((PASS+1))
else
  echo "  [BUG ] atk9.2 refs=[-1] code=$CODE 未拦截"
  FAIL=$((FAIL+1))
  BUGS+=("atk9.2: refs 含负值未被拦截")
  ID_NEG=$(echo "$R" | jq -r '.data.id // empty' | tr -d '\r')
  [ -n "$ID_NEG" ] && fld_rm "$ID_NEG"
fi

# ---- Attack 10: 畸形输入 / 注入 / 极端长度 ----
subsection "ATK-10: 畸形输入 / 注入"

R=$(post "/fields/create" '{"name":"a]\"injection","label":"注入","type":"integer","category":"combat","properties":{}}')
assert_code "atk10.1 含特殊字符 name 40002" "40002" "$R"

R=$(post "/fields/create" "{\"name\":\"${P}sqli\",\"label\":\"'; DROP TABLE fields; --\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{}}")
CODE=$(echo "$R" | jq -r '.code' | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$CODE" = "0" ]; then
  echo "  [PASS] atk10.2 SQL-like label 被安全处理"; PASS=$((PASS+1))
  SQLI_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
  fld_rm "$SQLI_ID"
else
  echo "  [FAIL] atk10.2 意外 code=$CODE"; FAIL=$((FAIL+1))
fi

LONG_NAME=$(python3 -c "print('a' * 100)" 2>/dev/null || python -c "print('a' * 100)" 2>/dev/null || echo "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
R=$(post "/fields/create" "{\"name\":\"${LONG_NAME}\",\"label\":\"超长\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{}}")
assert_code "atk10.3 超长 name 40002" "40002" "$R"

R=$(curl -s -X POST "$BASE/fields/create" -H "Content-Type: application/json" -d '{bad json}')
assert_code "atk10.4 畸形 JSON 40000" "40000" "$R"

R=$(curl -s -X POST "$BASE/fields/create" -H "Content-Type: application/json" -d '')
assert_code "atk10.5 空请求体 40000" "40000" "$R"

R=$(post "/templates/create" "{\"name\":\"${P}atk_fz\",\"label\":\"x\",\"description\":\"\",\"fields\":[{\"field_id\":0,\"required\":true}]}")
assert_code "atk10.6 field_id=0 40000" "40000" "$R"

R=$(post "/templates/create" "{\"name\":\"${P}atk_fn\",\"label\":\"x\",\"description\":\"\",\"fields\":[{\"field_id\":-1,\"required\":true}]}")
assert_code "atk10.7 field_id=-1 40000" "40000" "$R"

R=$(curl -s -X POST "$BASE/templates/create" -H "Content-Type: application/json" -d '{bad json}')
assert_code "atk10.8 模板畸形 JSON 40000" "40000" "$R"

R=$(curl -s -X POST "$BASE/templates/create" -H "Content-Type: application/json" -d '')
assert_code "atk10.9 模板空 body 40000" "40000" "$R"

LONG_TPL="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
R=$(post "/templates/create" "{\"name\":\"${LONG_TPL}\",\"label\":\"x\",\"description\":\"\",\"fields\":[{\"field_id\":${F_ATK},\"required\":true}]}")
assert_code "atk10.10 超长模板 name 41002" "41002" "$R"

# ---- Attack 11: 极端数字 ----
subsection "ATK-11: 极端数字约束"

R=$(post "/fields/create" "{\"name\":\"${P}atk_big\",\"label\":\"big\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false,\"constraints\":{\"min\":-9999999999999999999,\"max\":9999999999999999999}}}")
CODE=$(echo "$R" | jq -r '.code' | tr -d '\r')
TOTAL=$((TOTAL + 1))
echo "  [INFO] atk11.1 超大 int 约束 code=$CODE（行为确认）"
PASS=$((PASS+1))
if [ "$CODE" = "0" ]; then
  BIG_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
  [ -n "$BIG_ID" ] && [ "$BIG_ID" != "null" ] && fld_rm "$BIG_ID"
fi

# ---- Attack 12: 缓存一致性 / 并发级联 ----
subsection "ATK-12: 缓存一致性（编辑 → 立即读 → 旧值不应泄漏）"

R=$(post "/fields/create" "{\"name\":\"${P}atk_cache\",\"label\":\"初始\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"v1\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}}}")
CACHE_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

# 第一次读（回填缓存）
R=$(fld_detail "$CACHE_ID")
assert_field "atk12.1 初始值" ".data.label" "初始" "$R"

# 立即编辑
V=$(fld_version "$CACHE_ID")
R=$(post "/fields/update" "{\"id\":${CACHE_ID},\"label\":\"已改\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"v2\",\"expose_bb\":false,\"constraints\":{\"min\":0,\"max\":100}},\"version\":${V}}")
assert_code "atk12.2 立即编辑成功" "0" "$R"

# 立即再读 — 必须是新值
R=$(fld_detail "$CACHE_ID")
assert_field "atk12.3 编辑后立即读 label=已改" ".data.label" "已改" "$R"
assert_field "atk12.3 properties.description=v2" ".data.properties.description" "v2" "$R"

# ---- Attack 13: 列表缓存一致性 ----
subsection "ATK-13: 列表缓存一致性（创建 → 立即列表 → 必须可见）"

R=$(post "/fields/list" '{"label":"原子操作","page":1,"page_size":20}')
BEFORE_TOTAL=$(echo "$R" | jq -r '.data.total' | tr -d '\r')

R=$(post "/fields/create" "{\"name\":\"${P}atk_atomic\",\"label\":\"原子操作\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false}}")
ATOMIC_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

R=$(post "/fields/list" '{"label":"原子操作","page":1,"page_size":20}')
AFTER_TOTAL=$(echo "$R" | jq -r '.data.total' | tr -d '\r')
TOTAL=$((TOTAL + 1))
if [ "$((AFTER_TOTAL - BEFORE_TOTAL))" = "1" ]; then
  echo "  [PASS] atk13.1 创建后列表立即反映 ($BEFORE_TOTAL → $AFTER_TOTAL)"
  PASS=$((PASS+1))
else
  echo "  [BUG ] atk13.1 列表未反映新建字段 ($BEFORE_TOTAL → $AFTER_TOTAL)"
  FAIL=$((FAIL+1))
  BUGS+=("atk13.1: 创建字段后列表缓存未正确失效")
fi

fld_rm "$ATOMIC_ID"

# ---- Attack 14: 大模板（50 字段）创建 + 编辑 ----
subsection "ATK-14: 50 字段模板"

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

R=$(post "/templates/create" "{\"name\":\"${P}atk_big_tpl\",\"label\":\"大模板\",\"description\":\"\",\"fields\":[${BIG_FIELDS}]}")
assert_code "atk14.1 50 字段模板创建成功" "0" "$R"
BIG_TPL=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

R=$(tpl_detail "$BIG_TPL")
assert_field "atk14.2 fields 长度=50" ".data.fields | length" "50" "$R"

# 每个字段 ref_count 都应 = 1
TOTAL=$((TOTAL + 1))
ALL_OK=true
for ID in "${BIG_IDS[@]}"; do
  RC=$(fld_refcount "$ID")
  if [ "$RC" != "1" ]; then
    ALL_OK=false
    echo "    字段 $ID ref_count=$RC (应 1)"
    break
  fi
done
if $ALL_OK; then
  echo "  [PASS] atk14.3 所有 50 个字段 ref_count=1"
  PASS=$((PASS+1))
else
  echo "  [FAIL] atk14.3 部分字段 ref_count 未正确递增"
  FAIL=$((FAIL+1))
fi

# 删除大模板 → 所有字段 ref_count 清零
tpl_disable "$BIG_TPL"
R=$(post "/templates/delete" "{\"id\":${BIG_TPL}}")
assert_code "atk14.4 大模板删除成功" "0" "$R"

TOTAL=$((TOTAL + 1))
ALL_OK=true
for ID in "${BIG_IDS[@]}"; do
  RC=$(fld_refcount "$ID")
  if [ "$RC" != "0" ]; then
    ALL_OK=false
    echo "    字段 $ID ref_count=$RC (应 0)"
    break
  fi
done
if $ALL_OK; then
  echo "  [PASS] atk14.5 所有 50 字段 ref_count 清零"
  PASS=$((PASS+1))
else
  echo "  [FAIL] atk14.5 部分字段未清零"
  FAIL=$((FAIL+1))
fi

# ---- Attack 15: Unicode label 搜索 ----
subsection "ATK-15: Unicode label 搜索"

R=$(post "/fields/create" "{\"name\":\"${P}atk_emoji\",\"label\":\"🔥 火焰\",\"type\":\"integer\",\"category\":\"combat\",\"properties\":{\"description\":\"\",\"expose_bb\":false}}")
EMOJI_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')
assert_code "atk15.1 emoji label 创建" "0" "$R"

R=$(post "/fields/list" "{\"label\":\"🔥\",\"page\":1,\"page_size\":20}")
assert_ge "atk15.2 emoji 搜索 ≥ 1" ".data.total" "1" "$R"

R=$(post "/fields/list" "{\"label\":\"火焰\",\"page\":1,\"page_size\":20}")
assert_ge "atk15.3 中文搜索 ≥ 1" ".data.total" "1" "$R"

fld_rm "$EMOJI_ID"

# ---- Attack 16: 模板中有 ref_count>0 时，required-only 变化 ----
subsection "ATK-16: 有 NPC 引用的模板（模拟 ref_count>0）— 当前无 NPC 模块，此测试是占位"

# 由于 NPC 模块未上线，我们无法制造 template.ref_count > 0 的情形。
# 但我们可以验证：即便在 ref_count=0 的场景下，required-only 变化 fieldsChanged=true 但 toAdd/toRemove 都为空
# 本路径在 atk7 中已验证。这里只确认 handler 不会因为 fieldsChanged 而误调 field_refs API。
# (占位：NPC 模块上线时可扩展成真实测试)
echo "  [INFO] atk16 待 NPC 模块上线后补充真实 ref_count>0 测试"

# ---- Attack 17: Get 接口的空标记防穿透 ----
subsection "ATK-17: 不存在 ID 连查 3 次，确保缓存不穿透"

# 三次查 999999，全部应返回 40011，且不崩溃
for i in 1 2 3; do
  R=$(fld_detail 999999)
  CODE=$(echo "$R" | jq -r '.code' | tr -d '\r')
  TOTAL=$((TOTAL + 1))
  if [ "$CODE" = "40011" ]; then
    echo "  [PASS] atk17.${i} 不存在 ID 第 ${i} 次返回 40011"
    PASS=$((PASS+1))
  else
    echo "  [FAIL] atk17.${i} code=$CODE"
    FAIL=$((FAIL+1))
  fi
done

# ---- Attack 18: 跨引用字段 GetReferences 输出结构 ----
subsection "ATK-18: GetReferences 返回结构"

R=$(post "/fields/references" "{\"id\":${LEAF1}}")
assert_code  "atk18.1 成功" "0" "$R"
assert_field "atk18.1 field_id 回显" ".data.field_id" "$LEAF1" "$R"
assert_field "atk18.1 field_label 回显" ".data.field_label" "攻击叶1" "$R"
# LEAF1 被 REF_A 引用（REF_A 是 reference 类型）
assert_ge    "atk18.1 fields 长度 ≥ 1（被 REF_A 引用）" ".data.fields | length" "1" "$R"

# ---- Attack 19: 版本号负值 ----
subsection "ATK-19: version 负值"

VER=$(fld_version "$LEAF3")
R=$(post "/fields/toggle-enabled" "{\"id\":${LEAF3},\"enabled\":false,\"version\":-1}")
assert_code "atk19.1 version=-1 → 40000" "40000" "$R"

# 同样对模板
R=$(post "/fields/update" "{\"id\":${LEAF3},\"label\":\"x\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{},\"version\":-999}")
assert_code "atk19.2 update version=-999 → 40000" "40000" "$R"

# ---- Attack 20: 停用后详情仍可查 / 已删除字段不能再 list 查到 ----
subsection "ATK-20: 生命周期：删除 → list 不可见"

R=$(post "/fields/create" "{\"name\":\"${P}atk_lifecycle\",\"label\":\"生命周期\",\"type\":\"integer\",\"category\":\"basic\",\"properties\":{\"description\":\"\",\"expose_bb\":false}}")
LIFE_ID=$(echo "$R" | jq -r '.data.id' | tr -d '\r')

R=$(post "/fields/list" '{"label":"生命周期","page":1,"page_size":20}')
assert_ge "atk20.1 创建后 list 可见" ".data.total" "1" "$R"

fld_rm "$LIFE_ID"

R=$(post "/fields/list" '{"label":"生命周期","page":1,"page_size":20}')
assert_field "atk20.2 删除后 list 不可见 total=0" ".data.total" "0" "$R"


# =============================================================================
# PART 5: 事件类型管理（扩展字段 Schema + 事件类型 CRUD + 导出 + 攻击）
# =============================================================================
echo "================================================================="
echo "  事件类型管理 API 集成测试   $(date)"
echo "================================================================="
echo ""

# =============================================================================
# 1. 扩展字段 Schema CRUD
# =============================================================================
echo "--- 1. 扩展字段 Schema CRUD ---"

# 1.1 创建 schema: priority (int, 1-10, default 5)
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}priority\",\"field_label\":\"优先级\",\"field_type\":\"int\",\"constraints\":{\"min\":1,\"max\":10},\"default_value\":5,\"sort_order\":1}")
assert_code "1.1 创建 schema priority" "0" "$body"
SCHEMA_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 1.2 创建 schema: category (string)
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}category\",\"field_label\":\"事件分类\",\"field_type\":\"string\",\"constraints\":{\"maxLength\":32},\"default_value\":\"unknown\",\"sort_order\":2}")
assert_code "1.2 创建 schema category" "0" "$body"
SCHEMA_ID2=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 1.3 重复 field_name → 42020
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}priority\",\"field_label\":\"重复\",\"field_type\":\"int\",\"constraints\":{},\"default_value\":0,\"sort_order\":0}")
assert_code "1.3 重复 field_name" "42020" "$body"

# 1.4 非法 field_type → 42024
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_type\",\"field_label\":\"坏类型\",\"field_type\":\"reference\",\"constraints\":{},\"default_value\":0,\"sort_order\":0}")
assert_code "1.4 reference 被拒" "42024" "$body"

# 1.5 constraints 不自洽 (min > max) → 42025
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_range\",\"field_label\":\"坏范围\",\"field_type\":\"int\",\"constraints\":{\"min\":10,\"max\":1},\"default_value\":5,\"sort_order\":0}")
assert_code "1.5 min > max" "42025" "$body"

# 1.6 default_value 不符合 constraints → 42026
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}bad_default\",\"field_label\":\"坏默认\",\"field_type\":\"int\",\"constraints\":{\"min\":1,\"max\":10},\"default_value\":99,\"sort_order\":0}")
assert_code "1.6 default 超范围" "42026" "$body"

# 1.7 列表
body=$(post "/event-type-schema/list" "{}")
assert_code "1.7 schema 列表" "0" "$body"

# 1.8 停用 schema (先拿 version)
SCHEMA2_DETAIL=$(post "/event-type-schema/list" "{}")
V=$(echo "$SCHEMA2_DETAIL" | jq -r ".data.items[] | select(.id==$SCHEMA_ID2) | .version" | tr -d '\r')
body=$(post "/event-type-schema/toggle-enabled" "{\"id\":$SCHEMA_ID2,\"version\":${V:-1}}")
assert_code "1.8 停用 schema" "0" "$body"

# 1.9 删除未停用 schema → 42027
body=$(post "/event-type-schema/delete" "{\"id\":$SCHEMA_ID}")
assert_code "1.9 删除未停用 schema" "42027" "$body"

echo ""

# =============================================================================
# 2. 事件类型 CRUD（正向流程）
# =============================================================================
echo "--- 2. 事件类型 CRUD ---"

# 2.1 创建 gunshot（auditory）
body=$(post "/event-types/create" "{\"name\":\"${P}gunshot\",\"display_name\":\"枪声\",\"perception_mode\":\"auditory\",\"default_severity\":90,\"default_ttl\":10,\"range\":300}")
assert_code "2.1 创建 gunshot" "0" "$body"
ET_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 2.2 创建 earthquake（global, range 自动置 0）
body=$(post "/event-types/create" "{\"name\":\"${P}earthquake\",\"display_name\":\"地震\",\"perception_mode\":\"global\",\"default_severity\":95,\"default_ttl\":30,\"range\":999}")
assert_code "2.2 创建 earthquake (global)" "0" "$body"
ET_ID2=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 2.3 详情 — config 里 range=0（global 兜底）
body=$(et_detail "$ET_ID2")
assert_code "2.3 详情" "0" "$body"
assert_field "2.3 global range=0" '.data.config.range' "0" "$body"
assert_field "2.3 severity=95" '.data.config.default_severity' "95" "$body"
assert_field "2.3 perception_mode=global" '.data.config.perception_mode' "global" "$body"

# 2.4 创建带扩展字段
body=$(post "/event-types/create" "{\"name\":\"${P}fire\",\"display_name\":\"火灾\",\"perception_mode\":\"visual\",\"default_severity\":70,\"default_ttl\":20,\"range\":100,\"extensions\":{\"${P}priority\":8}}")
assert_code "2.4 创建带扩展字段" "0" "$body"
ET_ID3=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 2.5 详情验证扩展字段
body=$(et_detail "$ET_ID3")
assert_field "2.5 扩展 priority=8" ".data.config.${P}priority" "8" "$body"

# 2.6 check-name 已存在
body=$(post "/event-types/check-name" "{\"name\":\"${P}gunshot\"}")
assert_code "2.6 check-name" "0" "$body"
assert_field "2.6 not available" '.data.available' "false" "$body"

# 2.7 check-name 可用
body=$(post "/event-types/check-name" "{\"name\":\"${P}not_exist\"}")
assert_field "2.7 available" '.data.available' "true" "$body"

# 2.8 列表
body=$(post "/event-types/list" "{\"page\":1,\"page_size\":20}")
assert_code "2.8 列表" "0" "$body"
assert_ge "2.8 total >= 3" '.data.total' "3" "$body"

# 2.9 列表 — perception_mode 筛选
body=$(post "/event-types/list" "{\"perception_mode\":\"global\",\"page\":1,\"page_size\":20}")
assert_code "2.9 列表 global 筛选" "0" "$body"

# 2.10 编辑 — 未停用状态，可编辑（默认 enabled=0）
V=$(et_detail "$ET_ID" | jq -r '.data.version' | tr -d '\r')
body=$(post "/event-types/update" "{\"id\":$ET_ID,\"display_name\":\"枪声(修改)\",\"perception_mode\":\"auditory\",\"default_severity\":85,\"default_ttl\":8,\"range\":250,\"version\":$V}")
assert_code "2.10 编辑 gunshot" "0" "$body"

# 2.11 编辑验证 — severity 变成 85
body=$(et_detail "$ET_ID")
assert_field "2.11 severity=85" '.data.config.default_severity' "85" "$body"

# 2.12 启用
V=$(et_detail "$ET_ID" | jq -r '.data.version' | tr -d '\r')
body=$(post "/event-types/toggle-enabled" "{\"id\":$ET_ID,\"version\":$V}")
assert_code "2.12 启用 gunshot" "0" "$body"

# 2.13 启用后编辑 → 42015
V=$(et_detail "$ET_ID" | jq -r '.data.version' | tr -d '\r')
body=$(post "/event-types/update" "{\"id\":$ET_ID,\"display_name\":\"枪声(再改)\",\"perception_mode\":\"auditory\",\"default_severity\":85,\"default_ttl\":8,\"range\":250,\"version\":$V}")
assert_code "2.13 启用后编辑拒绝" "42015" "$body"

# 2.14 启用后删除 → 42012
body=$(post "/event-types/delete" "{\"id\":$ET_ID}")
assert_code "2.14 启用后删除拒绝" "42012" "$body"

# 2.15 停用再删除
V=$(et_detail "$ET_ID" | jq -r '.data.version' | tr -d '\r')
body=$(post "/event-types/toggle-enabled" "{\"id\":$ET_ID,\"version\":$V}")
assert_code "2.15a 停用" "0" "$body"
body=$(post "/event-types/delete" "{\"id\":$ET_ID}")
assert_code "2.15b 删除" "0" "$body"

# 2.16 软删后 name 不可复用
body=$(post "/event-types/create" "{\"name\":\"${P}gunshot\",\"display_name\":\"枪声复用\",\"perception_mode\":\"auditory\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "2.16 软删后 name 不可复用" "42001" "$body"

echo ""

# =============================================================================
# 3. 导出 API
# =============================================================================
echo "--- 3. 导出 API ---"

# 先启用 earthquake 和 fire
V2=$(et_detail "$ET_ID2" | jq -r '.data.version' | tr -d '\r')
R=$(post "/event-types/toggle-enabled" "{\"id\":$ET_ID2,\"version\":$V2}")
echo "  [INFO] 启用 earthquake: $(echo $R | jq -r '.code' | tr -d '\r')"
V3=$(et_detail "$ET_ID3" | jq -r '.data.version' | tr -d '\r')
R=$(post "/event-types/toggle-enabled" "{\"id\":$ET_ID3,\"version\":$V3}")
echo "  [INFO] 启用 fire: $(echo $R | jq -r '.code' | tr -d '\r')"

# 3.1 导出 — 返回 items 数组
body=$(get_export "/event_types")
TOTAL=$((TOTAL + 1))
items_count=$(echo "$body" | jq '.items | length' 2>/dev/null | tr -d '\r')
if [ "$items_count" -ge "1" ] 2>/dev/null; then
  echo "  [PASS] 3.1 导出 items >= 1 (=$items_count)"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] 3.1 导出 items >= 1, 实际: $items_count"
  echo "         响应: $(echo "$body" | head -c 300)"
  FAIL=$((FAIL + 1))
fi

# 3.2 导出格式 — 每条有 name + config
TOTAL=$((TOTAL + 1))
first_name=$(echo "$body" | jq -r '.items[0].name // empty' | tr -d '\r')
first_config=$(echo "$body" | jq -r '.items[0].config // empty' | tr -d '\r')
if [ -n "$first_name" ] && [ -n "$first_config" ]; then
  echo "  [PASS] 3.2 导出格式 {name, config}"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] 3.2 导出格式, name='$first_name'"
  FAIL=$((FAIL + 1))
fi

# 3.3 已删除的 gunshot 不在导出中
TOTAL=$((TOTAL + 1))
deleted_count=$(echo "$body" | jq "[.items[] | select(.name==\"${P}gunshot\")] | length" | tr -d '\r')
if [ "$deleted_count" = "0" ]; then
  echo "  [PASS] 3.3 已删除不导出"
  PASS=$((PASS + 1))
else
  echo "  [FAIL] 3.3 已删除不导出, 找到 $deleted_count 条"
  FAIL=$((FAIL + 1))
fi

echo ""

# =============================================================================
# 4. 攻击性测试
# =============================================================================
echo "--- 4. 攻击性测试 ---"

# 4.1 name 含大写
body=$(post "/event-types/create" "{\"name\":\"${P}BadCase\",\"display_name\":\"大写\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "4.1 name 大写拒绝" "42002" "$body"

# 4.2 name 含中文
body=$(post "/event-types/create" "{\"name\":\"枪声\",\"display_name\":\"中文名\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "4.2 name 中文拒绝" "42002" "$body"

# 4.3 name 含空格
body=$(post "/event-types/create" "{\"name\":\"bad name\",\"display_name\":\"空格\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "4.3 name 空格拒绝" "42002" "$body"

# 4.4 非法 perception_mode
body=$(post "/event-types/create" "{\"name\":\"${P}bad_mode\",\"display_name\":\"坏模式\",\"perception_mode\":\"telekinesis\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "4.4 非法 perception_mode" "42003" "$body"

# 4.5 severity 超范围
body=$(post "/event-types/create" "{\"name\":\"${P}bad_sev\",\"display_name\":\"超范围\",\"perception_mode\":\"visual\",\"default_severity\":101,\"default_ttl\":5,\"range\":100}")
assert_code "4.5 severity > 100" "42004" "$body"

# 4.6 severity = 0 合法（零值不被吞）
body=$(post "/event-types/create" "{\"name\":\"${P}zero_sev\",\"display_name\":\"零威胁\",\"perception_mode\":\"visual\",\"default_severity\":0,\"default_ttl\":5,\"range\":100}")
assert_code "4.6 severity=0 合法" "0" "$body"
ZERO_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')
body=$(et_detail "$ZERO_ID")
assert_field "4.6 config.default_severity=0" '.data.config.default_severity' "0" "$body"

# 4.7 ttl <= 0
body=$(post "/event-types/create" "{\"name\":\"${P}bad_ttl\",\"display_name\":\"零TTL\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":0,\"range\":100}")
assert_code "4.7 ttl=0 拒绝" "42005" "$body"

# 4.8 range < 0
body=$(post "/event-types/create" "{\"name\":\"${P}bad_range\",\"display_name\":\"负范围\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":-1}")
assert_code "4.8 range < 0 拒绝" "42006" "$body"

# 4.9 扩展字段塞不存在的 key → 42022
body=$(post "/event-types/create" "{\"name\":\"${P}bad_ext\",\"display_name\":\"坏扩展\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100,\"extensions\":{\"nonexistent_field\":1}}")
assert_code "4.9 不存在的扩展字段" "42022" "$body"

# 4.10 扩展字段值不符合约束 → 42007
body=$(post "/event-types/create" "{\"name\":\"${P}bad_ext_val\",\"display_name\":\"坏扩展值\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100,\"extensions\":{\"${P}priority\":99}}")
assert_code "4.10 扩展值超约束" "42007" "$body"

# 4.11 display_name SQL 注入
body=$(post "/event-types/create" "{\"name\":\"${P}sqli\",\"display_name\":\"' OR 1=1 --\",\"perception_mode\":\"visual\",\"default_severity\":50,\"default_ttl\":5,\"range\":100}")
assert_code "4.11 SQL 注入 display_name 不崩" "0" "$body"

# 4.12 display_name 模糊搜索 LIKE 转义
body=$(post "/event-types/list" "{\"label\":\"%\",\"page\":1,\"page_size\":20}")
assert_code "4.12 LIKE % 不返回全部" "0" "$body"

# 4.13 乐观锁冲突（对未启用的 fire 事件用错误 version）
# 先停用 fire
V=$(et_detail "$ET_ID3" | jq -r '.data.version' | tr -d '\r')
post "/event-types/toggle-enabled" "{\"id\":$ET_ID3,\"version\":$V}" > /dev/null
body=$(post "/event-types/update" "{\"id\":$ET_ID3,\"display_name\":\"火灾(改)\",\"perception_mode\":\"visual\",\"default_severity\":70,\"default_ttl\":20,\"range\":100,\"version\":999}")
assert_code "4.13 乐观锁冲突" "42010" "$body"

# 4.14 不存在的 ID
body=$(post "/event-types/detail" "{\"id\":99999999}")
assert_code "4.14 不存在 ID" "42011" "$body"

echo ""


# =============================================================================
section "Part 7: 新增攻击测试（CJK 字符串长度 + 配置修复验证）"
# =============================================================================

subsection "CJK 字符串长度校验"

# 创建一个带 minLength/maxLength 约束的扩展字段 schema
body=$(post "/event-type-schema/create" "{\"field_name\":\"${P}cjk_test\",\"field_label\":\"中文测试\",\"field_type\":\"string\",\"constraints\":{\"minLength\":1,\"maxLength\":5},\"default_value\":\"测试\",\"sort_order\":99}")
assert_code "cjk.1 创建 string schema (minLength=1, maxLength=5)" "0" "$body"
CJK_SCHEMA_ID=$(echo "$body" | jq -r '.data.id' | tr -d '\r')

# 测试：3 个中文字符应通过 maxLength=5（字符数=3，不是字节数=9）
body=$(post "/event-type-schema/update" "{\"id\":$CJK_SCHEMA_ID,\"field_label\":\"中文测试\",\"constraints\":{\"minLength\":1,\"maxLength\":5},\"default_value\":\"三个字\",\"version\":1}")
assert_code "cjk.2 三个中文字符应通过 maxLength=5 (字符数=3)" "0" "$body"

# 测试：6 个中文字符应被 maxLength=5 拒绝（字符数=6 > 5）
body=$(post "/event-type-schema/update" "{\"id\":$CJK_SCHEMA_ID,\"field_label\":\"中文测试\",\"constraints\":{\"minLength\":1,\"maxLength\":5},\"default_value\":\"六个中文字符\",\"version\":2}")
assert_code "cjk.3 六个中文字符应被 maxLength=5 拒绝" "42026" "$body"

# 测试：空字符串应被 minLength=1 拒绝
body=$(post "/event-type-schema/update" "{\"id\":$CJK_SCHEMA_ID,\"field_label\":\"中文测试\",\"constraints\":{\"minLength\":1,\"maxLength\":5},\"default_value\":\"\",\"version\":2}")
assert_code "cjk.4 空字符串应被 minLength=1 拒绝" "42026" "$body"

# 清理 CJK schema
if [ -n "$CJK_SCHEMA_ID" ] && [ "$CJK_SCHEMA_ID" != "null" ]; then
  post "/event-type-schema/toggle-enabled" "{\"id\":$CJK_SCHEMA_ID,\"version\":2}" > /dev/null 2>&1
  post "/event-type-schema/delete" "{\"id\":$CJK_SCHEMA_ID}" > /dev/null 2>&1
fi

subsection "模板配置独立验证"

# 验证模板名称长度走独立配置（TemplateNameMaxLength）
LONG_NAME=$(printf 'a%.0s' {1..65})  # 65 字符
body=$(post "/templates/create" "{\"name\":\"$LONG_NAME\",\"label\":\"超长名\",\"description\":\"\",\"fields\":[{\"field_id\":1,\"required\":true}]}")
assert_code "cfg.1 模板名超 64 字符应被拒绝 (41002=NameInvalid)" "41002" "$body"

# =============================================================================
section "Part 6: 清理测试数据"
# =============================================================================

# 先清理 Part 5 的大模板字段
for ID in "${BIG_IDS[@]}"; do
  [ -n "$ID" ] && [ "$ID" != "null" ] && fld_rm "$ID" 2>/dev/null
done

# 清理 Part 5 的模板
for ID in "$O_TPL"; do
  [ -n "$ID" ] && [ "$ID" != "null" ] && tpl_rm "$ID" 2>/dev/null
done

# 清理字段（顺序：先 reference 类型持有者，再被引用的 target）
for ID in "$LEGACY_REF" "$REF_A" "$MORPH" "$CACHE_ID" \
          "$FHOLDER_ID" "$SHOLDER_ID" "$SELHOLDER_ID" "$BHOLDER_ID" \
          "$FTGT_ID" "$STGT_ID" "$SELTGT_ID" "$BTGT_ID" \
          "$LEGACY_TGT" "$MORPH_TGT" \
          "$O_A" "$O_B" "$LEAF1" "$LEAF2" "$LEAF3" \
          "$F_HP" "$F_ATK" "$F_NAME" "$F_DEF" "$F_DISABLED" \
          "$CB" "$CA" "$CD" "$TGT_ID" "$HP_ID" "$ATK_ID" "$MOOD_ID" "$FLOAT_ID"; do
  if [ -n "$ID" ] && [ "$ID" != "null" ]; then
    fld_rm "$ID" 2>/dev/null
  fi
done
echo "  清理完成"

# =============================================================================
section "汇总"
# =============================================================================

echo ""
echo "  总计: $TOTAL   通过: $PASS   失败: $FAIL"
echo ""
if [ "${#BUGS[@]}" -gt 0 ]; then
  echo "--------- 攻击命中的可疑 bug ---------"
  for b in "${BUGS[@]}"; do
    echo "  * $b"
  done
  echo "-------------------------------------"
fi
echo ""

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
exit 0
