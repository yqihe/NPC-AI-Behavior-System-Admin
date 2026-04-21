#!/usr/bin/env bash
#
# scripts/e2e/verify.sh — e2e 对账脚本
#
# 用法：
#   bash scripts/e2e/verify.sh first-round     # happy path + disable fan-out
#   bash scripts/e2e/verify.sh dangling-region # R4.1 故障注入
#   bash scripts/e2e/verify.sh dangling-fsm    # R4.2 故障注入
#
# 环境变量（可选）：
#   SERVER_COMPOSE_DIR   默认 ../NPC-AI-Behavior-System-Server-v1
#   SERVER_SERVICE       默认 server（docker compose service 名）
#   SERVER_CONTAINER     默认从 docker compose ps 自动解析
#   SERVER_METRICS_URL   默认 http://localhost:8080/metrics
#
# 依赖 Server 侧容器运行在可访问的 docker compose 项目中；由 Server CC 确保 compose up 完成。

set -euo pipefail

MODE=${1:-}
case "$MODE" in
	first-round|dangling-region|dangling-fsm) ;;
	*) echo "用法: $0 {first-round|dangling-region|dangling-fsm}"; exit 2 ;;
esac

SERVER_COMPOSE_DIR=${SERVER_COMPOSE_DIR:-../NPC-AI-Behavior-System-Server-v1}
SERVER_SERVICE=${SERVER_SERVICE:-server}
SERVER_METRICS_URL=${SERVER_METRICS_URL:-http://localhost:9820/metrics}

REPO_ROOT=$(git rev-parse --show-toplevel)
cd "$REPO_ROOT"

[ -d "$SERVER_COMPOSE_DIR" ] || { echo "✗ SERVER_COMPOSE_DIR 不存在: $SERVER_COMPOSE_DIR"; exit 1; }

# ──────────────────────────────────────────────
# 拉 Server 日志 + 容器状态
# ──────────────────────────────────────────────

echo "=== 抓取 Server 日志 + 容器状态 (mode=$MODE) ==="

SERVER_CONTAINER=${SERVER_CONTAINER:-$(cd "$SERVER_COMPOSE_DIR" && docker compose ps -q "$SERVER_SERVICE" | head -1)}
[ -n "$SERVER_CONTAINER" ] || { echo "✗ 找不到 Server 容器 ID"; exit 1; }

RESTART_COUNT=$(docker inspect --format='{{.RestartCount}}' "$SERVER_CONTAINER" 2>/dev/null | tr -d '\r')

# 故障注入模式（fail-fast 循环）：日志窗口缩到最近一次启动之后，避免第一轮 happy 行漏入。
# first-round 模式：Server 稳定运行，全量 tail 即可。
LOG_FILE=/tmp/e2e_server_${MODE}.log
if [ "$MODE" = "first-round" ]; then
	(cd "$SERVER_COMPOSE_DIR" && docker compose logs --no-color --tail=500 "$SERVER_SERVICE") > "$LOG_FILE" 2>&1 \
		|| { echo "✗ docker compose logs 失败"; exit 1; }
else
	STARTED_AT=$(docker inspect --format='{{.State.StartedAt}}' "$SERVER_CONTAINER" 2>/dev/null | tr -d '\r')
	[ -n "$STARTED_AT" ] || { echo "✗ 无法读取容器 StartedAt"; exit 1; }
	(cd "$SERVER_COMPOSE_DIR" && docker compose logs --no-color --since "$STARTED_AT" "$SERVER_SERVICE") > "$LOG_FILE" 2>&1 \
		|| { echo "✗ docker compose logs --since 失败"; exit 1; }
fi

echo "  container=$SERVER_CONTAINER  restart_count=$RESTART_COUNT  log_file=$LOG_FILE"

# ──────────────────────────────────────────────
# 断言辅助
# ──────────────────────────────────────────────

# 计数指定正则匹配的行数（grep -c，无匹配返 0）
count_match() {
	grep -cE "$1" "$LOG_FILE" || true
}

# 断言：正则匹配行数 = expected
assert_count_eq() {
	local pattern=$1 expected=$2 desc=$3
	local actual
	actual=$(count_match "$pattern")
	if [ "$actual" = "$expected" ]; then
		echo "[✓] $desc: $actual 行"
	else
		echo "[✗] $desc: 期望 $expected 行，实际 $actual 行（正则: $pattern）"
		FAIL=$((FAIL + 1))
	fi
}

# 断言：正则匹配行数 >= min
assert_count_ge() {
	local pattern=$1 min=$2 desc=$3
	local actual
	actual=$(count_match "$pattern")
	if [ "$actual" -ge "$min" ]; then
		echo "[✓] $desc: $actual 行 (>= $min)"
	else
		echo "[✗] $desc: 期望 >=$min 行，实际 $actual 行（正则: $pattern）"
		FAIL=$((FAIL + 1))
	fi
}

# 断言：整数变量 >= min
assert_int_ge() {
	local actual=$1 min=$2 desc=$3
	if [ "$actual" -ge "$min" ]; then
		echo "[✓] $desc: $actual (>= $min)"
	else
		echo "[✗] $desc: 期望 >=$min，实际 $actual"
		FAIL=$((FAIL + 1))
	fi
}

# 断言：整数变量 = expected
assert_int_eq() {
	local actual=$1 expected=$2 desc=$3
	if [ "$actual" = "$expected" ]; then
		echo "[✓] $desc: $actual"
	else
		echo "[✗] $desc: 期望 $expected，实际 $actual"
		FAIL=$((FAIL + 1))
	fi
}

FAIL=0

# ──────────────────────────────────────────────
# Mode: first-round
# ──────────────────────────────────────────────

if [ "$MODE" = "first-round" ]; then
	echo
	echo "=== [first-round] 启动锚点断言 ==="

	assert_count_ge 'msg=config\.source.*type=http'                                          1 "生效源标注 config.source type=http"
	assert_count_eq 'msg=config\.http\.loaded.*endpoint=/api/configs/event_types.*count=5'   1 "event_types 加载 count=5"
	assert_count_eq 'msg=config\.http\.loaded.*endpoint=/api/configs/fsm_configs.*count=3'   1 "fsm_configs 加载 count=3"
	assert_count_eq 'msg=config\.http\.loaded.*endpoint=/api/configs/bt_trees.*count=6'      1 "bt_trees 加载 count=6"
	assert_count_eq 'msg=config\.http\.loaded.*endpoint=/api/configs/npc_templates.*count=4' 1 "npc_templates 加载 count=4（disable fan-out 已滤）"
	assert_count_eq 'msg=config\.http\.loaded.*endpoint=/api/configs/regions.*count=2'       1 "regions 加载 count=2"
	assert_count_eq 'msg=events\.loaded.*count=5'                                            1 "events.loaded count=5"
	assert_count_eq 'msg=zones\.loaded.*count=2'                                             1 "zones.loaded count=2"
	assert_count_eq 'msg=admin_spawn\.done.*spawned=4.*template_count=4'                     1 "admin_spawn.done spawned=4 template_count=4"

	echo
	echo "=== [first-round] 禁区断言（以下必须 0 行）==="
	assert_count_eq 'msg=cascade\.violations'       0 "cascade.violations"
	assert_count_eq 'msg=zones\.spawn_error'        0 "zones.spawn_error"
	assert_count_eq 'msg=admin_spawn\.parse_error'  0 "admin_spawn.parse_error"
	assert_count_eq 'msg=admin_spawn\.instance_error' 0 "admin_spawn.instance_error"
	assert_count_eq 'msg=config\.http_error'        0 "config.http_error"

	echo
	echo "=== [first-round] /metrics 活跃数（容器稳定运行中）==="
	# Server 可能需要 ≥1 tick 才有 metrics；如果前面启动等过 sleep 5s，这里直接取
	METRICS=$(curl -sf "$SERVER_METRICS_URL" 2>/dev/null || echo "")
	if [ -z "$METRICS" ]; then
		echo "[⚠] /metrics 无法访问 ($SERVER_METRICS_URL)；跳过 npc_active_count 核对"
	else
		ACTIVE_SUM=$(echo "$METRICS" | grep -E '^npc_active_count(\{[^}]*\})?\s+[0-9]+$' \
			| awk '{sum += $NF} END {print sum+0}')
		assert_int_eq "$ACTIVE_SUM" 6 "npc_active_count Σ（4 模板 + 2 zone）"
	fi

	echo
	echo "=== [first-round] 容器行为 ==="
	assert_int_eq "$RESTART_COUNT" 0 "Server 容器 RestartCount"
fi

# ──────────────────────────────────────────────
# Mode: dangling-region
# ──────────────────────────────────────────────

if [ "$MODE" = "dangling-region" ]; then
	echo
	echo "=== [dangling-region] 悬空详情锚点 ==="
	assert_count_ge 'msg=config\.http\.regions\.dangling.*region_id=e2e_village.*ref_value=missing_npc_xxx' 1 "regions.dangling 详情行"
	assert_count_ge 'msg=config\.http_error.*code=47011'                                     1 "config.http_error code=47011"

	echo
	echo "=== [dangling-region] 容器行为 ==="
	assert_int_ge "$RESTART_COUNT" 2 "Server 容器 RestartCount"

	echo
	echo "=== [dangling-region] 后续阶段未到 ==="
	assert_count_eq 'msg=zones\.loaded'        0 "zones.loaded（启动未到）"
	assert_count_eq 'msg=admin_spawn\.done'    0 "admin_spawn.done（启动未到）"
	# 注：/api/configs/regions 500 时 Server 侧不会打 config.http.loaded regions 行；前 4 端点可能已加载
fi

# ──────────────────────────────────────────────
# Mode: dangling-fsm
# ──────────────────────────────────────────────

if [ "$MODE" = "dangling-fsm" ]; then
	echo
	echo "=== [dangling-fsm] HTTP 错误锚点 ==="
	assert_count_ge 'msg=config\.http_error.*api/configs/npc_templates.*status 500'  1 "config.http_error npc_templates status 500"

	echo
	echo "=== [dangling-fsm] 前 3 端点已 loaded（npc_templates 未到）==="
	assert_count_eq 'msg=config\.http\.loaded.*endpoint=/api/configs/event_types.*count=5' 1 "event_types 加载 count=5"
	assert_count_eq 'msg=config\.http\.loaded.*endpoint=/api/configs/fsm_configs.*count=3' 1 "fsm_configs 加载 count=3"
	assert_count_eq 'msg=config\.http\.loaded.*endpoint=/api/configs/bt_trees.*count=6'    1 "bt_trees 加载 count=6"
	assert_count_eq 'msg=config\.http\.loaded.*endpoint=/api/configs/npc_templates'        0 "npc_templates 加载（应 0，500 先失败）"

	echo
	echo "=== [dangling-fsm] 容器行为 ==="
	assert_int_ge "$RESTART_COUNT" 2 "Server 容器 RestartCount"

	echo
	echo "=== [dangling-fsm] 后续阶段未到 ==="
	assert_count_eq 'msg=config\.http\.loaded.*endpoint=/api/configs/regions' 0 "regions 加载（启动未到）"
	assert_count_eq 'msg=zones\.loaded'       0 "zones.loaded（启动未到）"
	assert_count_eq 'msg=admin_spawn\.done'   0 "admin_spawn.done（启动未到）"
fi

# ──────────────────────────────────────────────
# 汇总
# ──────────────────────────────────────────────

echo
if [ "$FAIL" = "0" ]; then
	echo "=== [$MODE] PASS ✓ ==="
	exit 0
else
	echo "=== [$MODE] FAIL ✗ ($FAIL 项失败) ==="
	echo "日志原文：$LOG_FILE"
	exit 1
fi
