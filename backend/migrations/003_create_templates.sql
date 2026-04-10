-- 模板管理相关表
-- 模板是 ADMIN 内部的"字段组合方案"。NPC 创建时选一个模板填值，
-- 创建后 NPC 把字段列表+值快照下来，与模板独立。

-- templates: 模板定义
CREATE TABLE IF NOT EXISTS templates (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    name            VARCHAR(64)  NOT NULL,              -- 模板标识，唯一，创建后不可变
    label           VARCHAR(128) NOT NULL,              -- 中文标签（搜索用）
    description     VARCHAR(512) NOT NULL DEFAULT '',   -- 描述（可选）
    fields          JSON         NOT NULL,              -- [{field_id, required}, ...] 数组顺序=NPC 表单展示顺序

    ref_count       INT          NOT NULL DEFAULT 0,    -- 被 NPC 引用数（冗余计数，事务内维护）
    enabled         TINYINT(1)   NOT NULL DEFAULT 0,    -- 启用状态（创建默认 0，给"配置窗口期"）
    version         INT          NOT NULL DEFAULT 1,    -- 乐观锁
    deleted         TINYINT(1)   NOT NULL DEFAULT 0,    -- 软删除
    created_at      DATETIME     NOT NULL,
    updated_at      DATETIME     NOT NULL,

    -- name 唯一约束不带 deleted：已删除的标识也不能复用，
    -- 防止历史 NPC 引用混乱（详见 features.md 功能 5）
    UNIQUE KEY uk_name (name),
    -- 覆盖索引：列表查询按 id DESC 扫描，含 enabled / label 用于内存过滤
    INDEX idx_list (deleted, id, name, label, ref_count, enabled, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
