-- fields 表新增 expose_bb 独立列 + 索引
-- 来源：docs/specs/field-expose-bb-column（从 fsm-config-frontend R22 分出）
--
-- migration 规则：不做 ALTER TABLE，Drop + 重建

DROP TABLE IF EXISTS fields;

CREATE TABLE fields (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    name            VARCHAR(64)  NOT NULL,              -- 字段标识，唯一，创建后不可变
    label           VARCHAR(128) NOT NULL,              -- 中文标签（搜索用）
    type            VARCHAR(32)  NOT NULL,              -- 字段类型（筛选用）
    category        VARCHAR(32)  NOT NULL,              -- 标签分类（筛选用）
    properties      JSON         NOT NULL,              -- 动态属性（描述/BB Key/默认值/约束等）
    expose_bb       TINYINT(1)   NOT NULL DEFAULT 0,    -- 是否暴露给 BB 系统（独立列，可走索引过滤）

    enabled         TINYINT(1)   NOT NULL DEFAULT 0,    -- 启用状态（0=停用，1=启用）
    version         INT          NOT NULL DEFAULT 1,    -- 乐观锁
    deleted         TINYINT(1)   NOT NULL DEFAULT 0,    -- 软删除
    created_at      DATETIME     NOT NULL,
    updated_at      DATETIME     NOT NULL,

    UNIQUE KEY uk_name (name),
    -- 覆盖索引：列表查询不回表（含 enabled 列）
    INDEX idx_list (deleted, id, name, label, type, category, enabled, created_at),
    -- BBKeySelector 过滤专用索引
    INDEX idx_expose_bb (expose_bb)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
