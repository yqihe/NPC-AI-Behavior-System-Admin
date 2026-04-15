# FSM Config Frontend — 需求分析

## 动机

后端状态机管理（`/api/v1/fsm-configs/*`）已全部实现并上线，包含 CRUD、启用切换、BB Key 引用追踪、乐观锁、缓存等。前端目前完全没有对应入口——策划无法通过管理平台创建或修改任何状态机配置，只能直接写 SQL 或调 API，违背"无需接触代码或 JSON"的产品定位。

不做的后果：行为树（下一个模块）依赖状态机的状态名来挂接节点，NPC 管理依赖状态机 name 做引用——前端状态机页面缺席会阻塞后续所有行为管理模块的开发和验收。

## 优先级

**高**。当前 V3 开发顺序：字段 → 模板 → 事件类型 → **状态机（当前）** → 行为树 → NPC。状态机前端是进入行为树开发的前置条件，延迟直接影响毕设进度。

## 预期效果

**场景 1：策划创建第一个状态机**
1. 进入「行为配置管理 → 状态机管理」列表页，点「新建状态机」
2. 填写标识 `wolf_fsm`、中文名「狼 FSM」
3. 在状态列表里添加 `idle / chase / attack`，选择初始状态 `idle`
4. 添加转换规则：`idle → chase`，优先级 2，条件：`player_distance < 80`
5. 点保存，列表页出现这条记录（默认停用），策划手动启用后游戏服务端可拉取

**场景 2：查看和编辑已有状态机**
1. 列表页点「查看」，进入只读详情，能看到所有状态和转换规则的结构化展示
2. 先禁用（或 EnabledGuardDialog 自动引导），再点「编辑」，修改条件树，保存

**场景 3：条件树复合逻辑**
策划需要表达「玩家距离 < 10 且 体力 > 攻击阈值」：
- 在条件区选「组合条件 AND」
- 添加两个叶条件，分别配 key/op/value 和 key/op/ref_key
- 保存后服务端校验通过

## 依赖分析

**依赖（已完成）：**
- 后端 `fsm_configs` 全套接口（已实现）
- `field-expose-bb-column`：后端字段表 `expose_bb` 独立列 + `bb_exposed` 过滤参数（已完成，BBKeySelector 依赖此接口）
- `EnabledGuardDialog` 组件（已有，需追加 `fsm-config` entityType 分支）
- `AppLayout.vue` 侧边栏（已有 `group-fsm` 分组，需追加「状态机管理」入口）
- `list-layout.css` / `form-layout.css` 全局样式（已有）
- `formatTime` 工具函数（已有，列表页 created_at 列使用）

**被依赖（尚未开始）：**
- 行为树前端：按「状态名 → bt_ref」挂接，需要先有状态机页面可验证数据
- NPC 管理前端：引用状态机 name

## 改动范围

| 文件 | 类型 | 说明 |
|------|------|------|
| `frontend/src/api/fsmConfigs.ts` | 新建 | 类型 + FSM_ERR + fsmConfigApi |
| `frontend/src/views/FsmConfigList.vue` | 新建 | 列表页 |
| `frontend/src/views/FsmConfigForm.vue` | 新建 | 新建/编辑/查看表单页 |
| `frontend/src/components/FsmStateListEditor.vue` | 新建 | 状态列表编辑器 |
| `frontend/src/components/FsmTransitionListEditor.vue` | 新建 | 转换规则列表编辑器 |
| `frontend/src/components/FsmConditionEditor.vue` | 新建 | 条件编辑器（三态 radio + 叶节点/组合节点） |
| `frontend/src/components/BBKeySelector.vue` | 新建 | BB Key 下拉（bb_exposed 字段 + 自由输入） |
| `frontend/src/components/EnabledGuardDialog.vue` | 修改 | 追加 `fsm-config` entityType 分支 |
| `frontend/src/components/AppLayout.vue` | 修改 | `group-fsm` 分组下追加「状态机管理」菜单项 |
| `frontend/src/router/index.ts` | 修改 | 追加 4 条路由（list / create / view / edit） |

共 10 个文件，7 新建 3 修改。

## 扩展轴检查

- **新增配置类型**：本 spec 本身就是在新增一个配置类型（FSM），完全遵循已有分层结构（新建 view + api 文件，不改其他模块核心逻辑），正面示范。
- **新增表单字段**：FsmConditionEditor 是首个递归组件，设计上自包含（通过 v-model 传递单节点数据），未来新增条件类型只需在组件内扩展分支，不改外层结构。

## 验收标准

**列表页**

- R1：列表页展示 id / name / display_name / initial_state / state_count / enabled / created_at 七列，数据来自后端分页接口
- R2：display_name 模糊搜索 + enabled 三态筛选（全部/仅启用/仅停用）正确传参后端
- R3：列表页 el-switch 切换启用状态，切换前弹 ElMessageBox.confirm 二次确认，成功后刷新列表；切换前先 detail 拿最新 version
- R4：已启用记录点「编辑」或「删除」时，由 EnabledGuardDialog 拦截引导先禁用
- R5：空列表时展示 el-empty + 「新建状态机」快捷按钮

**表单页**

- R6：新建/编辑/查看三种模式由 route.meta 区分，共用一个 FsmConfigForm.vue
- R7：新建模式下 name 字段失焦后调 checkName 接口，显示可用/不可用状态
- R8：编辑/查看模式下 name 字段只读，显示 Lock 图标
- R9：查看模式下整个 el-form disabled，底部 form-footer 隐藏，不展示时间戳
- R10：保存成功后跳转回列表页；新建时提示「创建成功，状态机默认为禁用状态，确认无误后请手动启用」，编辑时提示「保存成功」

**状态编辑器**

- R11：可动态添加/删除状态行，状态名实时检测重名，重名时行内标红提示
- R12：initial_state 下拉选项始终与当前 states 列表同步
- R13：删除当前 initial_state 对应的状态时，自动重置 initial_state 为 states[0]
- R14：状态列表为空时，点保存前端拦截（不触发接口），显示「至少定义一个状态」

**转换规则编辑器**

- R15：可动态添加/删除转换规则，from/to 下拉选项来自当前 states 列表
- R16：每条规则可折叠，收起时显示摘要 `from → to | 优先级N`；有条件时追加固定文案「| 已配置条件」，无条件时追加「| 无条件」
- R17：priority 使用 el-input-number，min=0，默认值 0

**条件编辑器**

- R18：条件类型三选一（无条件 / 单条件 / 组合条件），切换时清空对应字段
- R19：组合条件支持 AND / OR 二选一，可递归嵌套，最多 10 层（超出层数时禁用「添加子条件」按钮）
- R20：叶节点比较值模式二选一（直接值 value / 引用 BB Key ref_key），切换时清空另一字段
- R21：op 下拉枚举限定白名单：`==  !=  >  >=  <  <=  in`
- R22：BBKeySelector 调 `fieldApi.list({ bb_exposed: true, enabled: true })` 拉取候选字段，同时支持 `allow-create` 自由输入运行时 Key
- R23：value 输入控件根据所选 BB Key 的字段类型自适应：`integer` → el-input-number（step=1）；`float` → el-input-number（step=0.01）；`boolean` → el-select（true / false）；`string` / `select` / 运行时 Key → el-input（文本）；切换 key 时自动清空 value

**错误处理**

- R24：VERSION_CONFLICT（43011）→ ElMessageBox.alert 提示「数据已被其他人修改，请刷新后重试」，不跳转
- R25：NAME_EXISTS（43001）→ name 字段下方显示红字，不 toast
- R26：STATES_EMPTY（43004）/ STATE_NAME_INVALID（43005）/ INITIAL_INVALID（43006）/ TRANSITION_INVALID（43007）/ CONDITION_INVALID（43008）→ ElMessage.error 显示后端返回的 message，表单不关闭
- R27：NOT_FOUND（43003）→ ElMessage.error + 跳回列表
- R28：vue-tsc --noEmit 零错误通过

## 不做什么

- **不做**转换规则拖拽排序（交互复杂度高，非核心功能，延后）
- **不做**运行时 Key 表接口（后端未规划），BBKeySelector 的运行时 Key 来源仅为自由输入；运行时 Key 类型未知，value 控件统一降级为文本框
- **不做**条件摘要详情展示（R16 收起态仅固定文案，不解析条件树生成可读文本）
- **不做**导出 API 前端入口（属于「导出管理」模块范围）
- **不做**查看模式展示创建/更新时间（遵循项目约定：时间戳仅在列表页 created_at 列展示）
