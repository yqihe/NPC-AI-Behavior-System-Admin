-- 事件类型管理
-- 事件类型是"游戏世界里会发生什么事"的元数据登记。
-- MySQL 单存储，导出 API 直接输出 config_json 列给游戏服务端。

-- event_types: 事件类型定义
CREATE TABLE IF NOT EXISTS event_types (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    name            VARCHAR(64)  NOT NULL,              -- 事件标识，唯一，创建后不可变
    display_name    VARCHAR(128) NOT NULL,              -- 中文名（搜索用）
    perception_mode VARCHAR(16)  NOT NULL,              -- 感知模式：visual / auditory / global（facet 筛选）
    config_json     JSON         NOT NULL,              -- 系统字段 + 扩展字段的完整合并，导出 API 原样输出

    enabled         TINYINT(1)   NOT NULL DEFAULT 0,    -- 启用状态（创建默认 0，给"配置窗口期"）
    version         INT          NOT NULL DEFAULT 1,    -- 乐观锁
    created_at      DATETIME     NOT NULL,
    updated_at      DATETIME     NOT NULL,
    deleted         TINYINT(1)   NOT NULL DEFAULT 0,    -- 软删除

    -- name 唯一约束不含 deleted：软删后 name 仍占唯一性，不可复用
    UNIQUE KEY uk_name (name),
    -- 覆盖索引：列表分页查询（id DESC 排序，含 display_name / perception_mode / enabled 用于筛选）
    INDEX idx_list (deleted, enabled, id DESC),
    -- facet 筛选索引：按 perception_mode 精确过滤
    INDEX idx_perception (deleted, perception_mode)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
