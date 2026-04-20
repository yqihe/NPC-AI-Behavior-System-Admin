-- 运行时 BB Key 注册表
-- 独立于 fields 表的第三类 BB Key 来源，对齐游戏服务端 internal/core/blackboard/keys.go 的 31 个静态声明。
-- 用途：FSM 条件 / BT check_bb_* 节点的 key 下拉在"字段"和"事件扩展字段"之外，新增"运行时 Key"一组。
-- 数据权威：服务端 keys.go；ADMIN 侧 seed 手工对齐（不走 API 互拉，CLAUDE.md 约定）。

CREATE TABLE IF NOT EXISTS runtime_bb_keys (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    name            VARCHAR(64)  NOT NULL,              -- BB Key 名，对齐 keys.go NewKey[T]("...") 第一参数，格式 ^[a-z][a-z0-9_]*$
    type            VARCHAR(16)  NOT NULL,              -- 规范化 4 枚举：integer / float / string / bool（Go float64→float、int64→integer）
    label           VARCHAR(64)  NOT NULL,              -- 中文标签（UI 下拉展示）
    description     VARCHAR(255) NOT NULL DEFAULT '',   -- 中文描述（UI tooltip）
    group_name      VARCHAR(32)  NOT NULL,              -- 分组（threat/event/fsm/npc/action/need/emotion/memory/social/decision/move），对齐 keys.go 分节注释

    enabled         TINYINT(1)   NOT NULL DEFAULT 1,    -- 启用状态（默认 1 启用，与 field 相反：运行时 key 是服务端权威定义，默认可用）
    version         INT          NOT NULL DEFAULT 1,    -- 乐观锁
    created_at      DATETIME     NOT NULL,
    updated_at      DATETIME     NOT NULL,
    deleted         TINYINT(1)   NOT NULL DEFAULT 0,    -- 软删除

    -- name 唯一约束不含 deleted：软删后 name 仍占唯一性，不可复用（对齐 bt_trees / fsm_configs 约定）
    UNIQUE KEY uk_name (name),
    -- 覆盖索引：列表分页查询（deleted + enabled 筛选 + id DESC 排序）
    INDEX idx_list (deleted, enabled, id DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
