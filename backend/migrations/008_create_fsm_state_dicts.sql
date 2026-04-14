CREATE TABLE IF NOT EXISTS fsm_state_dicts (
    id           BIGINT       NOT NULL AUTO_INCREMENT PRIMARY KEY,
    name         VARCHAR(64)  NOT NULL,
    display_name VARCHAR(128) NOT NULL,
    category     VARCHAR(64)  NOT NULL,
    description  VARCHAR(512) NOT NULL DEFAULT '',
    enabled      TINYINT(1)   NOT NULL DEFAULT 1,
    version      INT          NOT NULL DEFAULT 1,
    created_at   DATETIME     NOT NULL,
    updated_at   DATETIME     NOT NULL,
    deleted      TINYINT(1)   NOT NULL DEFAULT 0,

    UNIQUE KEY uk_name (name),
    INDEX idx_list (deleted, enabled, id DESC),
    INDEX idx_category (deleted, category)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
