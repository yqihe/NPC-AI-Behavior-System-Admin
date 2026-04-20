-- 区域（region）配置
-- 对应游戏服务端 internal/runtime/zone/zone.go 的 Zone 结构，承载 spawn_table 静态配置。
-- Server HTTPSource 通过 GET /api/configs/regions 拉取本表 enabled=1 的记录 → ZoneManager 落地运行时 Zone。
-- 数据模型：扁平 2D（只 x/z 无 y），spawn_table 以 JSON 数组存 SpawnEntry（template_ref/count/spawn_points/wander_radius/respawn_seconds）。
-- 对应 spec：docs/specs/regions-module/

CREATE TABLE IF NOT EXISTS regions (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    region_id       VARCHAR(64)  NOT NULL,              -- 业务键（导出 envelope name），snake_case，格式 ^[a-z][a-z0-9_]*$，创建后不可变
    display_name    VARCHAR(128) NOT NULL,              -- 中文名（列表搜索 + UI 展示）
    region_type     VARCHAR(32)  NOT NULL,              -- 字典 region_type 组枚举：wilderness / town（本期锁两值）
    spawn_table     JSON         NOT NULL,              -- SpawnEntry[] JSON；可为空数组 '[]'；Server Unmarshal 直透

    enabled         TINYINT(1)   NOT NULL DEFAULT 0,    -- 启用状态（创建默认 0，留配置窗口期）
    version         INT          NOT NULL DEFAULT 1,    -- 乐观锁
    created_at      DATETIME     NOT NULL,
    updated_at      DATETIME     NOT NULL,
    deleted         TINYINT(1)   NOT NULL DEFAULT 0,    -- 软删除

    -- region_id 唯一约束不含 deleted：软删后 region_id 仍占唯一性，不可复用（对齐 bt_trees / fsm_configs / runtime_bb_keys 约定）
    UNIQUE KEY uk_region_id (region_id),
    -- 覆盖索引：列表分页查询（deleted + enabled 筛选 + id DESC 排序）
    INDEX idx_list (deleted, enabled, id DESC),
    -- 辅助索引：region_type 筛选（R5）
    INDEX idx_region_type (region_type, deleted)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
