-- 行为树节点类型引用关系表
-- 记录每棵行为树使用了哪些节点类型，写操作时同步维护。
-- 替代原先全表扫描 bt_trees.config 的 JSON 解析方案，引用检查走索引。

CREATE TABLE IF NOT EXISTS bt_node_type_refs (
    bt_tree_id  BIGINT      NOT NULL,   -- 引用方：行为树 ID
    type_name   VARCHAR(64) NOT NULL,   -- 被引用的节点类型标识

    PRIMARY KEY (bt_tree_id, type_name),
    INDEX idx_type_name (type_name)     -- 按 type_name 查"哪些树引用了我"
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
