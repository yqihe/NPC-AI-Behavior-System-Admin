-- 重建 field_refs 表：VARCHAR name 关联 → BIGINT id 关联
-- 当前 field_refs 无生产数据（模板管理未开发），可直接重建

DROP TABLE IF EXISTS field_refs;

CREATE TABLE field_refs (
    field_id    BIGINT       NOT NULL,              -- 被引用的字段 ID
    ref_type    VARCHAR(16)  NOT NULL,              -- 引用来源：'template' / 'field'
    ref_id      BIGINT       NOT NULL,              -- 引用方 ID（模板 ID 或字段 ID）

    PRIMARY KEY (field_id, ref_type, ref_id),
    INDEX idx_ref (ref_type, ref_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
