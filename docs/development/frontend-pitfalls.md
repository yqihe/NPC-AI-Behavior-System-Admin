# 前端常见陷阱（Vue 3 + Element Plus）

编写前端代码时主动检查的清单。踩到新坑时追加。

## JavaScript 基础

- **`==` 隐式转换**：`0 == ""` 为 true，`null == undefined` 为 true。一律使用 `===` 严格比较
- **浮点精度**：`0.1 + 0.2 !== 0.3`。滑块组件的 step 如果是小数（如 0.1），比较值时不能用 `===`，需容差比较或用整数运算（乘 10 再除）
- **typeof null === "object"**：判断 null 必须显式 `val === null`，不能靠 typeof
- **数组/对象是引用**：`const a = formData.states; a.push(x)` 会修改原始 formData。需要浅拷贝用 `[...arr]` / `{...obj}`，深拷贝用 `JSON.parse(JSON.stringify())` 或 `structuredClone()`
- **`for...in` 遍历对象会拿到原型链属性**：遍历对象 key 用 `Object.keys()` / `Object.entries()`，不用 `for...in`
- **`parseInt` 不传 radix**：`parseInt("08")` 在老环境可能解析为八进制。始终传第二参数 `parseInt(str, 10)`
- **空数组是 truthy**：`if ([])` 为 true。判断数组为空必须检查 `.length === 0`

## Vue 3 响应式

- **解构丢失响应性**：`const { name, config } = reactive(obj)` — name 和 config 不再是响应式的。使用 `toRefs()` 解构，或直接访问 `state.name`
- **ref 忘记 `.value`**：在 `<script setup>` 中访问 ref 必须用 `.value`，模板中自动解包不需要。最常见的 bug 来源
- **reactive 不能整体替换**：`state = reactive(newObj)` 不会触发响应式更新（变量指向变了但 Vue 不知道）。用 `Object.assign(state, newObj)` 或改用 `ref`
- **watch 深层对象**：`watch(state, cb)` 默认是浅监听。对象内部属性变化需要 `{ deep: true }`，但深监听有性能开销。优先 watch 具体字段 `watch(() => state.name, cb)`
- **computed 不能有副作用**：`computed` 里不要发请求、修改其他响应式数据。它应该是纯计算
- **v-for 必须有 key**：`v-for="item in list"` 不带 `:key` 会导致 DOM 复用错乱（表单输入值错位）。key 必须是唯一稳定值，不能用 index（删除/排序时 index 会变）
- **异步组件中修改已卸载组件的响应式状态**：API 请求回来时组件可能已销毁，`ref.value = data` 会 warning。用 `onUnmounted` 中设置标志位，或在赋值前检查组件是否存活

## Element Plus 组件

- **el-form 校验时机**：`el-form-item` 的 `prop` 必须与 `el-form` 的 `:model` 中的字段名一致，否则校验不触发（静默失效，不报错）
  ```vue
  <!-- ❌ prop 和 model 字段名不匹配，校验不工作 -->
  <el-form :model="form">
    <el-form-item prop="eventName">  <!-- form 里是 name 不是 eventName -->
      <el-input v-model="form.name" />
    </el-form-item>
  </el-form>
  
  <!-- ✅ prop 必须与 model 字段路径一致 -->
  <el-form :model="form">
    <el-form-item prop="name">
      <el-input v-model="form.name" />
    </el-form-item>
  </el-form>
  ```
- **el-form 嵌套对象校验**：嵌套属性的 prop 需要用点号路径 `prop="config.range"`，对应 rules 也要以相同路径为 key
- **el-dialog 内表单残留数据**：关闭对话框再打开，表单数据残留。需要在 `@open` 或 `@close` 事件中重置表单（`formRef.resetFields()` 或手动赋初始值）
- **el-select v-model 类型**：如果选项的 value 是数字，v-model 绑定的变量也必须是数字。类型不一致会导致选中但显示为空
- **el-table 动态数据更新不渲染**：直接修改数组某一项的属性有时不触发更新。用 `list.value = [...list.value]` 触发，或确保用 `ref` 包裹数组
- **el-slider 精度**：`el-slider` 的 step 默认是 1，小数拖动需要显式设置 `:step="0.1"`，且 `:min` / `:max` 必须是 Number 不是 String

## Axios / HTTP 请求

- **请求/响应拦截器中的错误处理**：响应拦截器 reject 后，调用方的 `.catch()` 才能捕获。如果拦截器中吞掉了 error（如弹 ElMessage 但没 return Promise.reject），调用方 `.then()` 会收到 undefined
- **并发请求竞态**：用户快速点击两次"保存"，两个 POST 请求可能都成功导致重复数据。前端需要做防抖或提交时 loading 禁用按钮
- **baseURL 环境差异**：开发环境 Vite proxy 到 `localhost:9821`，Docker 内通过服务名 `admin-backend:9821`。用环境变量 `VITE_API_BASE` 控制，不要硬编码
- **大数字精度丢失**：JavaScript 的 `Number.MAX_SAFE_INTEGER` 是 2^53-1。如果 MongoDB ObjectId 或其他 ID 用数字传输会丢失精度。本项目用 `name` 作为业务主键（字符串），规避此问题
- **错误响应解析**：后端返回 `{"error": "名称已存在"}` 时，Axios 的 `error.response.data.error` 才是错误信息。`error.message` 是 Axios 自己的（如 "Request failed with status code 409"），不要展示给用户

## CSS / 布局

- **el-aside 宽度**：`el-aside` 默认宽度 300px，需显式设置 `width`。不设置时侧边栏在不同屏幕上表现不一致
- **scoped 样式穿透**：`<style scoped>` 内无法直接覆盖 Element Plus 组件内部样式。用 `:deep(.el-xxx)` 穿透选择器
- **flex 布局溢出**：flex 子元素默认 `min-width: auto`，长文本不会换行或出滚动条。需要设置 `min-width: 0` 或 `overflow: hidden`

## 构建 / Vite

- **环境变量前缀**：Vite 只暴露 `VITE_` 前缀的环境变量到客户端代码。`API_BASE=xxx` 在前端代码中取不到，必须叫 `VITE_API_BASE`
- **动态导入路径**：`import(\`./views/${name}.vue\`)` 不能用完全动态的变量，Vite 需要能静态分析出可能的路径范围。路由懒加载用 `() => import('../views/EventList.vue')` 显式路径
- **生产构建体积**：Element Plus 按需引入（`unplugin-vue-components` + `unplugin-auto-import`），不要全量引入，否则打包体积过大
- **proxy 只在 dev 生效**：`vite.config.js` 中的 `server.proxy` 只影响 `npm run dev`。生产构建后由 nginx 反代，proxy 配置不生效。需要 nginx.conf 配置对应的 proxy_pass

---

## BT 节点编辑器

- **装饰节点 ≠ 复合节点**：`inverter` 是装饰节点，只有一个子节点（`child`），不是复合节点（`children` 数组）。前端 BtNodeEditor 必须区分三类节点：复合（sequence/selector/parallel）、装饰（inverter）、叶子。产出的 JSON 中装饰节点使用 `child: {...}` 而非 `children: [...]`
- **装饰节点切换类型时**：从复合节点切换到装饰节点时，需清空 `children` 并初始化 `decoratorChild`；反之亦然。忘记清理会导致产出的 JSON 同时携带 `children` 和 `child`，游戏服务端解析会忽略多余字段但行为不可预期
- **BB Key 必须用下拉选择器**：`set_bb_value` / `check_bb_float` / `check_bb_string` 的 key 参数必须用 `el-select` 从白名单中选择，**不能用 `el-input` 手动输入**。原因：未注册的 BB key 会导致游戏服务端 panic。白名单定义在 BtNodeEditor.vue 的 `bbKeys` 常量中，与游戏服务端 `blackboard/keys.go` 对齐。下拉选项同时显示中文标签和原始 key 名
- **stub_action result 有三个值**：`success` / `failure` / `running`（不是只有两个）。前端 `el-select` 必须包含这三个选项。无效值服务端静默降级为 success
- **NPC 编辑页关联列表缓存窗口期**：NPC 编辑页的 FSM/BT 下拉列表通过后端 list API 获取，走 Redis 5 分钟缓存。极端情况下（刚创建的 FSM/BT 还未出现在缓存中），策划可能在下拉中看不到新配置。刷新页面即可解决。这是已知限制，低并发场景下几乎不会出现

---

*在开发过程中踩到新坑时追加到本文档对应分类下。*
