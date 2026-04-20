-- 运行时 BB Key 引用关系表
-- 记录 FSM / BT 配置引用了哪些运行时 key；FSM/BT Create/Update/Delete 时 service 层同步维护。
-- 与 fieldService.SyncFsmBBKeyRefs 平行运行：同一个 FSM 条件 newKeys 集合里，字段 key → field_refs 表；
-- 运行时 key → 本表；未识别 name 由 FSM validator 前置 400 拦截。
--
-- 结构对称 field_refs：三元组 (runtime_key_id, ref_type, ref_id) + 反向覆盖索引。
-- 不设 FK（对齐项目既有约定，完整性靠 service 层）。

CREATE TABLE IF NOT EXISTS runtime_bb_key_refs (
    runtime_key_id  BIGINT      NOT NULL,                   -- 被引用方：运行时 key ID
    ref_type        VARCHAR(16) NOT NULL,                   -- 引用方类型：fsm | bt
    ref_id          BIGINT      NOT NULL,                   -- 引用方 ID（fsm_configs.id 或 bt_trees.id）
    created_at      DATETIME    NOT NULL,

    PRIMARY KEY (runtime_key_id, ref_type, ref_id),
    -- 反向索引：按 (ref_type, ref_id) 查"FSM/BT X 引用了哪些 runtime key"（级联删用）
    INDEX idx_reverse (ref_type, ref_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
