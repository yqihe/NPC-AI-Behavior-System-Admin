# 前端开发规范（Vue 3 + Element Plus）

禁止红线见 `../red-lines/frontend.md`。

## 1. JavaScript 基础

1. `==` 隐式转换：`0 == ""` 为 true，一律用 `===`
2. 浮点精度：`0.1 + 0.2 !== 0.3`，滑块 step 为小数时需容差比较
3. `typeof null === "object"`，判断 null 用 `val === null`
4. 引用类型：`const a = arr; a.push(x)` 修改原始。浅拷贝 `[...arr]`，深拷贝 `structuredClone()`
5. 空数组是 truthy：`if ([])` 为 true，判空用 `.length === 0`
6. `parseInt` 始终传 radix：`parseInt(str, 10)`

## 2. Vue 3 响应式

1. 解构丢失响应性：`const { name } = reactive(obj)` 不再响应。用 `toRefs()` 或直接访问
2. ref 忘 `.value`：`<script setup>` 必须 `.value`，模板自动解包
3. reactive 不能整体替换：`state = reactive(newObj)` 不触发更新，用 `Object.assign(state, newObj)`
4. watch 深层对象：默认浅监听，需 `{ deep: true }` 或 watch 具体字段
5. computed 无副作用：不发请求、不修改响应式数据
6. v-for 必须有唯一稳定 key，不能用 index
7. 双向 deep watcher 死循环：spread 每次创建新对象触发无限循环，加 `JSON.stringify` 比较防止
8. 异步回调赋值前检查组件是否已卸载

## 3. Element Plus 组件

1. `el-form-item` 的 `prop` 必须与 `:model` 字段名一致，否则校验静默失效
2. 嵌套对象校验：`prop="config.range"` 点号路径
3. `el-dialog` 关闭再打开数据残留，在 `@open`/`@close` 重置
4. `el-select` v-model 类型必须与选项 value 类型一致
5. `el-slider` step 默认 1，小数需显式 `:step="0.1"`
6. `ElMessage`/`ElMessageBox` 是命令式 API，样式需在 `main.ts` 手动 import
7. 多级侧栏用 `el-sub-menu`（可折叠），不用 `el-menu-item-group`（不可折叠）
8. `el-form :disabled` 与子组件 `:disabled` 交互：Element Plus 用 `??` 合并，子组件传 `false` 会覆盖表单级。写 `:disabled="isView || condition"`
9. 自定义约束组件 `defineExpose({ validate })` 暴露 `validate(): string | null`，父组件提交前调用

## 4. Axios / HTTP

1. 拦截器吞错：弹 ElMessage 后没 `return Promise.reject`，调用方 `.then()` 收到 undefined
2. 并发竞态：快速双击提交重复数据，需 loading 禁用按钮
3. baseURL：dev 走 Vite proxy，prod 走 nginx，用 `VITE_API_BASE` 控制
4. 错误响应：`error.response.data.error` 是后端错误，`error.message` 是 Axios 自己的
5. 拦截器给 Error 挂 `code` 属性，调用方按错误码差异化处理
6. 列表接口可能不返回 version，需要乐观锁的操作先调 detail

## 5. CSS / 布局

1. scoped 穿透：`:deep(.el-xxx)` 覆盖 Element Plus 样式
2. flex 溢出：子元素 `min-width: auto` 导致长文本不换行，设 `min-width: 0`
3. 禁止固定 px 宽度：用 `width: 100%` 或 `flex: 1` 自适应
4. opacity 影响交互：禁用行整行 opacity 需排除开关/操作列，用选择器精确控制

## 6. Vite 构建

1. 环境变量前缀 `VITE_`
2. 路由懒加载用显式路径 `() => import('../views/Xxx.vue')`
3. proxy 只在 dev 生效，prod 由 nginx 反代
4. Docker 内 npm ci 超时：设 DNS 或换镜像源
5. Docker nginx 无热更新：开发用本地 `npm run dev`

## 7. 前后端数据格式对齐

1. **富对象转换边界**：`ref_fields` 富对象只在 `FieldForm.vue` 内部存在（`loadFieldDetail` 转入 + `buildSubmitProperties` 转出）。其他组件读字段 detail 必须读 `properties.constraints.refs`（后端权威 `number[]`）
2. **约束 key 必须对齐 seed**：所有约束面板 key 严格匹配 `constraint_schema` 命名（`min`/`max`/`minLength`/`maxLength`/`pattern`/`minSelect`/`maxSelect`/`options`/`refs`）。用错 key 约束收紧检查会静默失效
3. **JSON RawMessage 是哑契约**：JSON 子结构两端约定无 DB 校验，必须有单一权威来源（seed `constraint_schema`），前后端都引用

## 8. 构建与类型检查

1. **vue-tsc 必跑**：`vite build` 不做类型检查，CI 和提交前必须 `npx vue-tsc --noEmit`
2. **reactive 显式泛型**：`reactive({ enabled: null })` 推断字面量类型，后续赋值报错。必须 `reactive<FormState>({...})` 或显式接口类型
3. **回调参数显式类型**：`@change="(v: string | number | boolean) => ..."` 等，Element Plus 事件推断不友好

## 9. BT 节点编辑器

1. 装饰节点 ≠ 复合节点：`inverter` 用 `child`（单对象）
2. 类型切换时清理对应字段
3. BB Key 下拉用 `el-select`，白名单对齐服务端 `keys.go`
4. `stub_action` result：三个值 `success`/`failure`/`running`

## 10. Vue Router

同组件多路由不刷新：`<router-view :key="route.fullPath" />` 强制重建，或 `watch(() => route.fullPath, reload)` 监听变化。
