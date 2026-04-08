-- dictionaries: 所有下拉选项统一管理
CREATE TABLE IF NOT EXISTS dictionaries (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    group_name      VARCHAR(32)  NOT NULL,              -- 分组标识
    name            VARCHAR(64)  NOT NULL,              -- 程序标识
    label           VARCHAR(128) NOT NULL,              -- 中文显示名
    sort_order      INT          NOT NULL DEFAULT 0,    -- 排序权重
    extra           JSON         DEFAULT NULL,          -- 扩展数据（constraint_schema / input_type 等）
    enabled         TINYINT(1)   NOT NULL DEFAULT 1,    -- 启用/停用
    created_at      DATETIME     NOT NULL,
    updated_at      DATETIME     NOT NULL,

    UNIQUE KEY uk_group_name (group_name, name),
    -- 覆盖索引：按 group 查全部选项
    INDEX idx_group_list (group_name, enabled, sort_order, name, label)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
