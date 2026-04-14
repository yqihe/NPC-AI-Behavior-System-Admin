-- 扩展字段引用关系表
-- 追踪哪些事件类型使用了哪些扩展字段定义
-- 结构与 field_refs 对齐：被引用方 ID + 引用来源类型 + 引用方 ID

CREATE TABLE IF NOT EXISTS schema_refs (
    schema_id   BIGINT       NOT NULL,              -- 被引用的扩展字段定义 ID
    ref_type    VARCHAR(16)  NOT NULL,              -- 引用来源：'event_type'
    ref_id      BIGINT       NOT NULL,              -- 引用方 ID（事件类型 ID）

    PRIMARY KEY (schema_id, ref_type, ref_id),
    INDEX idx_ref (ref_type, ref_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
