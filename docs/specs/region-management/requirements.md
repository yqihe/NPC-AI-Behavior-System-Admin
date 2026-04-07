# 需求 3：区域管理

## 动机

区域管理页面目前使用 GenericForm 的 JSON 编辑器模式（无 schema 关联），运营人员需要手写 JSON。区域 schema 已在 MongoDB 中（`_region`），只需关联即可激活动态表单。

**不做会怎样**：运营人员需要手写区域 JSON，违背"禁止让策划手写 JSON"红线。

## 优先级

中。改动极小（1 个文件），但能让区域页面从不可用变为可用。

## 预期效果

运营点击"区域管理"→"新建"→ 看到动态表单（region_id / name / region_type 下拉 / boundary 坐标 / weather / spawn_table）→ 填写保存。

## 依赖分析

- 前置：需求 0（CRUD 框架）+ 需求 1（种子脚本已导入 `_region` schema）
- 被依赖：无

## 改动范围

1 个文件：`frontend/src/router/index.js`（路由 meta 加 configSchema）。
可能需要额外改动 GenericForm 来支持启动时加载 schema。

## 扩展轴检查

- 新增配置类型：不涉及
- 新增表单字段：✅ 修改 region.json → 重新导入 → 自动生效

## 验收标准

- **R1**：区域新建页面显示动态表单（非 JSON 编辑器）
- **R2**：表单包含 region_id / name / region_type（下拉）/ boundary / spawn_table 字段
- **R3**：区域列表页正常展示
- **R4**：`npm run build` 通过

## 不做什么

- ❌ 不做地图可视化
- ❌ 不做 spawn_table 引用校验
- ❌ 不新建专用页面
