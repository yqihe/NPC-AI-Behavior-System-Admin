-- 事件类型扩展字段 Schema
-- 运营通过 Schema 管理页定义事件类型的扩展字段（如 priority / category / cooldown 等）。
-- 新增扩展字段后，事件类型新建/编辑表单自动多出对应输入控件（SchemaForm 渲染）。

-- event_type_schema: 扩展字段定义
CREATE TABLE IF NOT EXISTS event_type_schema (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    field_name      VARCHAR(64)  NOT NULL,              -- 扩展字段 key，符合 ^[a-z][a-z0-9_]*$
    field_label     VARCHAR(128) NOT NULL,              -- 中文名
    field_type      VARCHAR(16)  NOT NULL,              -- int / float / string / bool / select（不支持 reference）
    constraints     JSON         NOT NULL,              -- 按 type 的约束（min/max/pattern/options 等）
    default_value   JSON         NOT NULL,              -- 前端表单初始值提示（不回填历史数据）
    sort_order      INT          NOT NULL DEFAULT 0,    -- 表单展示顺序

    enabled         TINYINT(1)   NOT NULL DEFAULT 1,    -- 默认启用（和事件类型的 enabled=0 不同）
    version         INT          NOT NULL DEFAULT 1,    -- 乐观锁
    created_at      DATETIME     NOT NULL,
    updated_at      DATETIME     NOT NULL,
    deleted         TINYINT(1)   NOT NULL DEFAULT 0,    -- 软删除

    -- field_name 唯一约束不含 deleted：软删后 field_name 仍占唯一性
    UNIQUE KEY uk_field_name (field_name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
