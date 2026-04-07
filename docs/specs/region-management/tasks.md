# 需求 3：任务拆解

## T1: GenericForm 支持 schemaName 异步加载 + 区域路由关联 (R1, R2, R3, R4)

**修改文件：**
- `frontend/src/views/GenericForm.vue` — onMounted 时根据 `route.meta.schemaName` 异步加载 schema
- `frontend/src/router/index.js` — regions 路由 meta 加 `schemaName: '_region'`

**做完了是什么样：** 区域新建页面显示动态表单（region_id / name / region_type 下拉 / boundary / spawn_table），不再是 JSON 编辑器。`npm run build` 通过。

---

## T2: 文档更新

**修改文件：**
- `docs/specs/v3-roadmap.md` — 需求 3 状态更新
- `docs/specs/region-management/tasks.md` — 标记完成
