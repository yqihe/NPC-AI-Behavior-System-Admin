-- 字段管理相关表

-- fields: 字段定义
CREATE TABLE IF NOT EXISTS fields (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    name            VARCHAR(64)  NOT NULL,              -- 字段标识，唯一，创建后不可变
    label           VARCHAR(128) NOT NULL,              -- 中文标签（搜索用）
    type            VARCHAR(32)  NOT NULL,              -- 字段类型（筛选用）
    category        VARCHAR(32)  NOT NULL,              -- 标签分类（筛选用）
    properties      JSON         NOT NULL,              -- 动态属性（描述/BB Key/默认值/约束等）

    ref_count       INT          NOT NULL DEFAULT 0,    -- 被引用数（冗余计数，事务内维护）
    enabled         TINYINT(1)   NOT NULL DEFAULT 0,    -- 启用状态（0=停用，1=启用）
    version         INT          NOT NULL DEFAULT 1,    -- 乐观锁
    deleted         TINYINT(1)   NOT NULL DEFAULT 0,    -- 软删除
    created_at      DATETIME     NOT NULL,
    updated_at      DATETIME     NOT NULL,

    UNIQUE KEY uk_name (name),
    -- 覆盖索引：列表查询不回表（含 enabled 列）
    INDEX idx_list (deleted, id, name, label, type, category, ref_count, enabled, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- field_refs: 字段引用关系（全部使用 BIGINT ID 关联）
CREATE TABLE IF NOT EXISTS field_refs (
    field_id    BIGINT       NOT NULL,              -- 被引用的字段 ID
    ref_type    VARCHAR(16)  NOT NULL,              -- 引用来源：'template' / 'field'
    ref_id      BIGINT       NOT NULL,              -- 引用方 ID（模板 ID 或字段 ID）

    PRIMARY KEY (field_id, ref_type, ref_id),
    INDEX idx_ref (ref_type, ref_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
