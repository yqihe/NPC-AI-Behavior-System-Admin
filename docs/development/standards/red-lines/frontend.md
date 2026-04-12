# 前端禁止红线

适用于所有 Vue 3 + Element Plus 项目。开发规范见 `../dev-rules/frontend.md`。

## 禁止数据源污染

- **禁止**对 `ref`/`reactive` 中的列表数据直接 `.filter()`/`.splice()` 做 UI 过滤。过滤/排序必须用 `computed` 派生，原始数据源不变
- **禁止** WebSocket/轮询数据经过过滤后才存入状态。实时数据先无条件存入原始数据源，再由 computed 过滤展示

## 禁止放行无效输入

- **禁止**用自由文本输入枚举类值。BB Key、操作符、节点类型、result 等有限集合必须用 `el-select`，不允许手动输入
- **禁止**同一数据源的下拉列表不一致。所有使用 BB Key 的组件必须引用同一常量数组
- **禁止**名称字段不做格式校验。必须限制格式（`/^[a-z][a-z0-9_]*$/`，BT 额外允许 `/`），blur 时即时校验

## 禁止 URL 编码遗漏

- **禁止**将含特殊字符（`/`、`?`、`#`、`%`）的参数直接拼接到 URL 路径。API 调用层必须 `encodeURIComponent()`
- **禁止** Vue Router 动态路由段不处理含 `/` 的参数。使用 catch-all 语法（`:name(.*)`）
- **禁止**只修前端不验后端就认为 URL 编码问题已修复。含 `/` 的参数涉及三层（路由 → API → 后端），必须端到端验证

## 禁止 JSON 子结构 key 各写各的

- **禁止**前后端各自在代码里硬编码 JSON RawMessage 的子 key 名。所有写入 `properties` / `constraints` / `extra` 等无 schema JSON 列的子结构必须有**单一权威**（如 seed 中的 `constraint_schema`），前后端都引用这份权威
- **禁止**用驼峰/下划线/全小写变体重命名 key（`minLength` ≠ `min_length` ≠ `minimum_length`）。DB 层不会报错，但收紧检查、导出契约、跨端读取会全部静默失效
- **禁止**在非表单组件里假设字段 detail 返回富对象。UI 富对象（如 `ref_fields: [{id, name, label}]`）只存在于 **编辑表单组件**（`FieldForm.vue`）的本地 state，只在 `loadFieldDetail` 转入、`buildSubmitProperties` 转出。**任何非表单组件读字段 detail 时必须读 `refs: number[]`**（后端权威），再自行并发拉子字段元数据。反模式：模板的 reference popover 假设 `ref_fields` 直接返回 → popover 永远空白

## 禁止跳过类型检查就上线

- **禁止**只跑 `vite build` 就认为前端没问题。`vite build` 不调用 `vue-tsc`，TS 错误能被静默打包。提交前/CI 必须显式跑 `npx vue-tsc --noEmit`
- **禁止** `reactive({...})` 不带显式泛型。字面量初值会被推断成最窄类型（`null`、`''`），后续赋值新字段或宽类型都会编译失败
- **禁止** `@update:model-value="(v) => ..."` 等回调省略参数类型注解。Element Plus 事件类型推断不友好，strict 模式下会隐式 any
