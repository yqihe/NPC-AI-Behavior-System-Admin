-- ============================================================
-- 测试数据重置脚本
-- 用途：清空所有业务数据，保留字典种子（下拉选项来源）
-- 执行后：dictionaries 完整保留，其余表清空，可按测试指南从零操作
-- ============================================================

SET FOREIGN_KEY_CHECKS = 0;

-- 引用关系表（先清，避免外键冲突）
TRUNCATE TABLE field_refs;
TRUNCATE TABLE schema_refs;

-- 业务数据表
TRUNCATE TABLE fields;
TRUNCATE TABLE templates;
TRUNCATE TABLE event_type_schema;
TRUNCATE TABLE event_types;
TRUNCATE TABLE fsm_state_dicts;
TRUNCATE TABLE fsm_configs;

SET FOREIGN_KEY_CHECKS = 1;

-- 验证结果
SELECT 'dictionaries'      AS `表名`, COUNT(*) AS `行数` FROM dictionaries
UNION ALL
SELECT 'fields',            COUNT(*) FROM fields
UNION ALL
SELECT 'field_refs',        COUNT(*) FROM field_refs
UNION ALL
SELECT 'templates',         COUNT(*) FROM templates
UNION ALL
SELECT 'event_type_schema', COUNT(*) FROM event_type_schema
UNION ALL
SELECT 'schema_refs',       COUNT(*) FROM schema_refs
UNION ALL
SELECT 'event_types',       COUNT(*) FROM event_types
UNION ALL
SELECT 'fsm_state_dicts',   COUNT(*) FROM fsm_state_dicts
UNION ALL
SELECT 'fsm_configs',       COUNT(*) FROM fsm_configs;
