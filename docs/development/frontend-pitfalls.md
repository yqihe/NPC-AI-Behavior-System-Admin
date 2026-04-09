# 前端常见陷阱（Vue 3 + Element Plus）

编写前端代码时主动检查。禁止红线见 `../standards/frontend-red-lines.md`。

## JavaScript 基础

- **`==` 隐式转换**：`0 == ""` 为 true。一律用 `===`
- **浮点精度**：`0.1 + 0.2 !== 0.3`。滑块 step 为小数时需容差比较
- **typeof null**：`typeof null === "object"`。判断 null 用 `val === null`
- **引用类型**：`const a = formData.states; a.push(x)` 修改原始数据。浅拷贝 `[...arr]`，深拷贝 `structuredClone()`
- **空数组是 truthy**：`if ([])` 为 true。判空用 `.length === 0`
- **parseInt**：始终传 radix `parseInt(str, 10)`

## Vue 3 响应式

- **解构丢失响应性**：`const { name } = reactive(obj)` 不再响应式。用 `toRefs()` 或直接访问
- **ref 忘记 `.value`**：`<script setup>` 中必须 `.value`，模板中自动解包
- **reactive 不能整体替换**：`state = reactive(newObj)` 不触发更新。用 `Object.assign(state, newObj)`
- **watch 深层对象**：默认浅监听，需 `{ deep: true }` 或 watch 具体字段
- **computed 无副作用**：不发请求、不修改其他响应式数据
- **v-for 必须有 key**：key 必须唯一稳定，不能用 index
- **双向 deep watcher 死循环**：`watch(prop, set local)` + `watch(local, emit)` 两个 deep watcher 互相触发无限循环（spread 每次创建新对象，deep watcher 视为变化）。必须加 `JSON.stringify` 比较防止循环
- **异步组件状态**：API 回来时组件可能已卸载，赋值前检查

## Element Plus 组件

- **el-form prop 匹配**：`el-form-item` 的 `prop` 必须与 `:model` 字段名一���，否则校验静默失效
- **嵌套对象校验**：prop 用点号路径 `prop="config.range"`
- **el-dialog 表单残留**：关闭���打开数据残留，需在 `@open`/`@close` 重置
- **el-select v-model 类型**：选项 value 是数字，v-model 也必须是数字
- **el-slider 精度**：step 默认 1，小数需显式 `:step="0.1"`
- **ElMessage/ElMessageBox 样式缺失**：auto-import 插件只处理模板组件，`ElMessage`、`ElMessageBox`、`ElNotification` 是命令式 JS API，样式不会自动引入。必须在 `main.ts` 手动导入：`import 'element-plus/theme-chalk/el-message.css'` 等，否则弹窗不显示

## Axios / HTTP

- **拦截器吞错**：响应拦截器弹 ElMessage 但没 `return Promise.reject`，调用方 `.then()` 收到 undefined
- **并发竞态**：快速双击提交导致重复数据，需防抖或 loading ��用按钮
- **baseURL 环境差异**：dev 走 Vite proxy，prod 走 nginx。用 `VITE_API_BASE` 控制
- **错误响应**：`error.response.data.error` 才是后端错误信���，`error.message` 是 Axios 自己的

- **拦截器需携带业务错误码**：拦截器 reject 时给 Error 对象挂 `code` 属性（`err.code = code`），调用方 `.catch(err)` 才能按错误码做差异化处理（弹窗/红字/跳转）
- **列表接口可能缺少字段**：列表接口返回精简数据（如不含 `version`），需要乐观锁的操作（toggle/update）必须先调 detail 获取完整数据再提交

## CSS / 布局

- **scoped 穿透**：`<style scoped>` 覆盖 Element Plus 组件样式用 `:deep(.el-xxx)`
- **flex 溢出**：子元素 `min-width: auto` 导致长文本不换行，设 `min-width: 0`
- **禁止固定 px 宽度**：输入框/下拉框不要写 `width: 360px` 等固定值，用 `width: 100%` 或 `flex: 1` 让控件自适应容器。筛选栏多个控件并排用 `flex: 1` 等比分配
- **opacity 影响子元素交互**：整行 `opacity: 0.5` 会让开关/按钮看起来不可点击。需要保留交互的列（开关、操作）不应被 opacity 覆盖，用选择器精确控制作用范围

## Vite 构建

- **环境变量前缀**：只有 `VITE_` 前缀的变量暴露到客户端���码
- **动态导入**：路由懒加载用显式路径 `() => import('../views/Xxx.vue')`
- **proxy 只在 dev 生效**：prod 由 nginx 反代
- **Docker 容器内 npm ci 网络超时**：Docker 容器 DNS 解析可能不稳定，导致 `npm ci` 下载依赖超时。解法：Docker Desktop 设置 DNS（`223.5.5.5`），或 Dockerfile 中 `npm config set registry https://registry.npmmirror.com`
- **Docker nginx 无热更新**：Docker 前端容器用 nginx 托管静态文件，改代码必须 `docker compose up --build`。开发阶段建议本地 `npm run dev`（Vite dev server 有 HMR）

## 前后端数据格式对齐

- **前后端 JSON key 不一致**：前端 UI 用富对象（如 `ref_fields: [{id, name, label}]`），后端存精简格式（如 `refs: [13, 14]`）。提交时必须转换成后端格式，编辑加载时必须反向还原。建议在提交函数中统一做 `buildSubmitProperties()` 转换，不要在组件内散落转换逻辑

## BT 节点编辑器

- **装饰节点 != 复合节点**：`inverter` 只有 `child`（单对象），不是 `children`（数组）
- **类型切换清理**：从复合切到装���时清空 `children` 并初始化 `decoratorChild`，反之亦然
- **BB Key 下拉**：必须用 `el-select`，不能 `el-input`。白名单来源与服务端 `keys.go` 对齐
- **stub_action result**：三个值 `success`/`failure`/`running`，不是两个

## Vue Router 组件复用

- **同组件多路由不刷新**：多个路由指向同一组件（如 GenericList）时，路由切换 Vue 复用实例，`onMounted` 不重新执行，setup 中赋值的常量（`route.meta`）也不更新。**解法**：在 `<router-view :key="route.fullPath" />` 加 key 强制重建；或在组件内 `watch(() => route.fullPath, reload)` 监听路由变化重新加载

---

*踩到新坑时追加到对应分类下。*
