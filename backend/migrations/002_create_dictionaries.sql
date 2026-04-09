-- 字典表：存储下拉选项、类型定义等可配置数据

CREATE TABLE IF NOT EXISTS dictionaries (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    group_name      VARCHAR(32)  NOT NULL,              -- 字典组（如 field_type、field_category）
    name            VARCHAR(64)  NOT NULL,              -- 程序标识
    label           VARCHAR(128) NOT NULL,              -- 中文展示名
    sort_order      INT          NOT NULL DEFAULT 0,    -- 排序权重
    extra           JSON         DEFAULT NULL,          -- 扩展数据（constraint_schema 等）
    enabled         TINYINT(1)   NOT NULL DEFAULT 1,    -- 启用状态
    created_at      DATETIME     NOT NULL,
    updated_at      DATETIME     NOT NULL,

    UNIQUE KEY uk_group_name (group_name, name),
    INDEX idx_group_list (group_name, enabled, sort_order, name, label)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
