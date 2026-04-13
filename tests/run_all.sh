#!/bin/bash
# =============================================================================
# ADMIN 后端 API 全方位集成测试 — 总启动器
#
# 用法：bash tests/run_all.sh
#       bash tests/run_all.sh test_07_fsm.sh   # 只跑指定模块
#
# 流程：
#   1. 环境准备（Docker 重建 + MySQL 清空 + Seed + Redis 清空）
#   2. 按编号顺序 source 各模块测试脚本（共享变量）
#   3. 汇总结果
# =============================================================================

set -o pipefail

export LANG=en_US.UTF-8
export LC_ALL=en_US.UTF-8
if command -v chcp.com &>/dev/null; then
  chcp.com 65001 > /dev/null 2>&1
fi

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

MYSQL_CONTAINER="npc-admin-mysql"
MYSQL_CMD="docker exec -i $MYSQL_CONTAINER mysql -uroot -proot npc_ai_admin"
MIGRATIONS_DIR="$PROJECT_ROOT/backend/migrations"

# =============================================================================
# Phase 0: 环境准备
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

# 0.3 清空 Redis 缓存
echo "  [0.3] 清空 Redis 缓存 ..."
docker exec npc-admin-redis redis-cli FLUSHALL > /dev/null 2>&1

# 0.4 清空业务表（重置 AUTO_INCREMENT），保留字典表
echo "  [0.4] 清空业务数据 ..."
echo "SET FOREIGN_KEY_CHECKS=0;
DROP TABLE IF EXISTS field_refs;
DROP TABLE IF EXISTS fields;
DROP TABLE IF EXISTS templates;
DROP TABLE IF EXISTS event_types;
DROP TABLE IF EXISTS event_type_schema;
DROP TABLE IF EXISTS fsm_configs;
SET FOREIGN_KEY_CHECKS=1;" | $MYSQL_CMD 2>/dev/null

# 0.5 重建所有表
echo "  [0.5] 重建所有表 ..."
for f in "$MIGRATIONS_DIR"/0*.sql; do
  $MYSQL_CMD < "$f" 2>/dev/null
done

# 0.6 检查字典种子数据
DICT_COUNT=$(echo "SELECT COUNT(*) AS c FROM dictionaries;" | $MYSQL_CMD -N 2>/dev/null | tr -d '\r ')
if [ "$DICT_COUNT" -gt 0 ] 2>/dev/null; then
  echo "  字典已有 ${DICT_COUNT} 条，跳过 seed"
else
  echo "  [0.6] 执行种子脚本 ..."
  cd "$PROJECT_ROOT/backend"
  go run ./cmd/seed/ -config config.yaml 2>&1
  cd "$PROJECT_ROOT"
fi
echo ""

# 0.7 重启后端（DictCache / SchemaCache 需要加载种子数据）
echo "  [0.7] 重启后端容器 ..."
docker restart npc-admin-backend > /dev/null 2>&1

# 0.8 等待后端就绪
echo "  [0.8] 等待后端就绪 ..."
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
# 加载共享工具
# =============================================================================

source "$SCRIPT_DIR/helpers.sh"

# =============================================================================
# 执行测试
# =============================================================================

if [ -n "$1" ]; then
  # 指定模块：bash run_all.sh test_07_fsm.sh
  echo "  只运行指定模块: $1"
  source "$SCRIPT_DIR/$1"
else
  # 全量：按编号顺序 source 所有 test_*.sh
  for f in "$SCRIPT_DIR"/test_*.sh; do
    source "$f"
  done
fi

# =============================================================================
# 汇总
# =============================================================================

print_summary

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
exit 0
