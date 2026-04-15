-- 行为树配置
-- 定义 NPC 在每个 FSM 状态下具体执行什么逻辑。
-- config 列存储完整节点树 JSON，导出 API 原样输出给游戏服务端。
-- NPC 配置通过 bt_refs（状态名 → 行为树 name）引用本表记录。

CREATE TABLE IF NOT EXISTS bt_trees (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    name            VARCHAR(128) NOT NULL,              -- 唯一标识（如 wolf/attack），格式 ^[a-z][a-z0-9_/]*$，创建后不可变
    display_name    VARCHAR(128) NOT NULL,              -- 中文名（列表搜索用）
    description     TEXT,                              -- 描述（可空）
    config          JSON         NOT NULL,              -- 根节点 JSON，树结构完整存储，导出 API 原样输出

    enabled         TINYINT(1)   NOT NULL DEFAULT 0,    -- 启用状态（创建默认 0，留配置窗口期）
    version         INT          NOT NULL DEFAULT 1,    -- 乐观锁
    created_at      DATETIME     NOT NULL,
    updated_at      DATETIME     NOT NULL,
    deleted         TINYINT(1)   NOT NULL DEFAULT 0,    -- 软删除

    -- name 唯一约束不含 deleted：软删后 name 仍占唯一性，不可复用
    UNIQUE KEY uk_name (name),
    -- 覆盖索引：列表分页查询（id DESC 排序，含 enabled 用于筛选）
    INDEX idx_list (deleted, enabled, id DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
