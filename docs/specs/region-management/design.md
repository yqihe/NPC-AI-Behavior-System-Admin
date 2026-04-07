# 需求 3：设计方案

## 方案描述

GenericForm 已支持通过路由 meta 的 `configSchema` 传入 JSON Schema。当前区域路由没有传 schema，所以降级为 JSON 编辑器。

**方案**：修改路由配置，让区域页面在挂载时从 API 加载 `_region` schema 并传入 GenericForm。

由于 schema 需要异步加载（从 `/api/v1/component-schemas/_region` 获取），而路由 meta 是静态的，需要在 GenericForm 中增加"按 schema 名称异步加载"的能力。

具体改动：
1. 路由 meta 新增 `schemaName` 字段（如 `"_region"`）
2. GenericForm 在 `onMounted` 时检查 `schemaName` → 调用 `componentSchemaApi.get(schemaName)` → 取出 `config.schema` → 传给 SchemaForm

## 方案对比

### A（选定）：GenericForm 支持 schemaName 异步加载
改 GenericForm + 路由 meta。

### B（不选）：路由 meta 直接写死 schema 对象
路由文件中硬编码 region schema JSON。违反"schema 由服务端定义"原则，且路由文件会变得臃肿。

## 红线检查
合规。无新增后端改动、无安全隐患、schema 从 API 动态加载不硬编码。

## 测试策略
`npm run build` + 手动验证区域新建表单渲染。
