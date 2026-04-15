-- 行为树节点类型注册表
-- 定义行为树编辑器中可使用的节点类型（composite / decorator / leaf）。
-- 内置类型由种子脚本初始化（is_builtin=1），不可删除/编辑。
-- 自定义类型由开发者通过系统设置 > 节点类型页注册，编辑器动态渲染对应参数表单。

CREATE TABLE IF NOT EXISTS bt_node_types (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    type_name       VARCHAR(64)  NOT NULL,              -- 节点类型标识，与导出 JSON 的 type 字段一致（如 sequence / check_bb_float）
    category        VARCHAR(16)  NOT NULL,              -- 节点分类：composite / decorator / leaf
    label           VARCHAR(128) NOT NULL,              -- 中文名（如 "序列"）
    description     TEXT,                              -- 描述（可空）
    param_schema    JSON         NOT NULL,              -- 参数定义列表，编辑器据此动态渲染表单；无参数时为 {"params":[]}

    is_builtin      TINYINT(1)   NOT NULL DEFAULT 0,    -- 1=内置种子，不可删除/编辑
    enabled         TINYINT(1)   NOT NULL DEFAULT 1,    -- 启用状态（内置类型默认启用；禁用后编辑器不显示该类型）
    version         INT          NOT NULL DEFAULT 1,    -- 乐观锁
    created_at      DATETIME     NOT NULL,
    updated_at      DATETIME     NOT NULL,
    deleted         TINYINT(1)   NOT NULL DEFAULT 0,    -- 软删除

    -- type_name 唯一约束不含 deleted：软删后 type_name 仍占唯一性，不可复用
    UNIQUE KEY uk_type_name (type_name),
    -- 覆盖索引：列表分页查询（按 category 筛选 + id DESC 排序）
    INDEX idx_list (deleted, enabled, category, id DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
