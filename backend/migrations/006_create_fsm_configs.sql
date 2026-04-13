-- 状态机管理
-- 定义 NPC 有哪些状态，什么条件下切换。
-- MySQL 单存储，导出 API 直接输出 config_json 列给游戏服务端。

-- fsm_configs: 状态机配置
CREATE TABLE IF NOT EXISTS fsm_configs (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    name            VARCHAR(64)  NOT NULL,              -- FSM 唯一标识（如 wolf_fsm），创建后不可变
    display_name    VARCHAR(128) NOT NULL,              -- 中文名（搜索用）
    config_json     JSON         NOT NULL,              -- {initial_state, states, transitions} 完整配置，导出 API 原样输出

    enabled         TINYINT(1)   NOT NULL DEFAULT 0,    -- 启用状态（创建默认 0，给"配置窗口期"）
    version         INT          NOT NULL DEFAULT 1,    -- 乐观锁
    created_at      DATETIME     NOT NULL,
    updated_at      DATETIME     NOT NULL,
    deleted         TINYINT(1)   NOT NULL DEFAULT 0,    -- 软删除

    -- name 唯一约束不含 deleted：软删后 name 仍占唯一性，不可复用
    UNIQUE KEY uk_name (name),
    -- 覆盖索引：列表分页查询（id DESC 排序，含 enabled 用于筛选）
    INDEX idx_list (deleted, enabled, id DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
