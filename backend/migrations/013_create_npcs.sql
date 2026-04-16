-- NPC 管理
-- NPC 是模板的实例，创建时快照模板字段列表+值，与模板后续变更无关。
-- behavior 配置（fsm_ref + bt_refs）独立于字段系统，导出 API 按 api-contract.md 格式输出。

-- npcs: NPC 实例
CREATE TABLE IF NOT EXISTS npcs (
    id            BIGINT       AUTO_INCREMENT PRIMARY KEY,
    name          VARCHAR(64)  NOT NULL,                  -- NPC 唯一标识（如 wolf_common），创建后不可变
    label         VARCHAR(128) NOT NULL,                  -- 中文标签（搜索用）
    description   VARCHAR(512) NOT NULL DEFAULT '',       -- 描述（可选）

    -- 模板快照（创建时一次性写入，后续 NPC 与模板独立）
    template_id   BIGINT       NOT NULL,                  -- 用于引用计数查询（TemplateHandler.Delete/Update）
    template_name VARCHAR(64)  NOT NULL,                  -- 导出用：config.template_ref

    -- 字段值快照：[{field_id, name, required, value}, ...]
    -- field_id 用于编辑时回查字段元数据；name 用于导出 key；value 为 JSON 原始值
    fields        JSON         NOT NULL,

    -- 行为配置（独立于字段系统）
    fsm_ref       VARCHAR(64)  NOT NULL DEFAULT '',       -- 状态机名，空串=无行为配置
    bt_refs       JSON         NOT NULL,                  -- {"state_name": "bt_tree_name"}，空对象=无

    enabled       TINYINT(1)   NOT NULL DEFAULT 1,        -- 创建即启用（NPC 是成品，区别于模板的 0）
    version       INT          NOT NULL DEFAULT 1,        -- 乐观锁
    created_at    DATETIME     NOT NULL,
    updated_at    DATETIME     NOT NULL,
    deleted       TINYINT(1)   NOT NULL DEFAULT 0,        -- 软删除

    -- name 唯一约束不含 deleted：软删后 name 仍占唯一性，不可复用
    UNIQUE KEY uk_name (name),
    -- 覆盖索引：列表分页 + 多字段筛选（name/label 模糊、template_name 精确、enabled 三态）
    INDEX idx_list     (deleted, id, name, label, template_name, enabled, created_at),
    -- 模板引用计数查询（TemplateHandler.Delete/Update 激活 41007/41008）
    INDEX idx_template (template_id, deleted),
    -- FSM 引用计数查询（FsmConfigHandler.Delete 激活 43012）
    INDEX idx_fsm      (fsm_ref, deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- npc_bt_refs: NPC 与行为树的引用关系表
-- 记录每个 NPC 的 bt_refs 中引用了哪些行为树，写操作时同步维护。
-- 替代 JSON_SEARCH 全表扫方案，引用检查走索引（BtTreeHandler.Delete 激活 44012）。
-- 对齐 bt_node_type_refs 模式。
CREATE TABLE IF NOT EXISTS npc_bt_refs (
    npc_id        BIGINT       NOT NULL,   -- 引用方：NPC ID
    bt_tree_name  VARCHAR(128) NOT NULL,   -- 被引用的行为树标识（含斜杠路径，如 wolf/idle）

    PRIMARY KEY (npc_id, bt_tree_name),
    INDEX idx_bt_name (bt_tree_name(64))   -- 按行为树名查"哪些 NPC 引用了我"，前缀索引
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
