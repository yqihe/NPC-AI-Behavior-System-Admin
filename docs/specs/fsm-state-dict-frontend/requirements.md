# FSM 状态字典前端管理页面 — 需求分析

## 动机

`fsm-state-dict-backend` 已完成（T1–T8 全部合并到 main），提供了完整的 8 个 REST 接口，但前端至今没有对应页面。运营人员无法通过管理后台查看、新建、编辑或删除 FSM 状态字典条目；seed 脚本写入的 31 条预置状态只能在数据库里裸看。

不做的代价：状态字典功能对运营透明，形同虚设；FSM 配置编辑页未来需要通过下拉框选取合法状态名，若状态字典页缺失，运营无从管理可用状态列表。

## 优先级

**高**。后端已全部就绪，前端实现成本低（完全克隆 EventTypeList/Form 模式），且是后续 FSM 配置编辑页（状态下拉框数据源）的前置依赖。

## 预期效果

### 场景 1：浏览与搜索
运营打开侧边栏「状态机管理」→「状态字典」，看到分页表格，每行显示：名称（英文标识）、中文名、分类、启用状态、创建时间。可按名称模糊搜索，按分类精确过滤（下拉从 `list-categories` 动态加载），按启用/停用过滤。

### 场景 2：新建状态
点击「新建状态」→ 跳转到创建表单，填写 name（小写字母+数字+下划线，`^[a-z][a-z0-9_]*$`）、中文名、分类（可从已有分类下拉选择或手动输入）、描述（可选）。name 失焦时即时校验唯一性（`check-name`）。提交后跳回列表并刷新。

### 场景 3：编辑状态
点击「编辑」→ 跳转编辑表单，name 只读展示，其余字段可修改。提交携带 `version`，发生版本冲突（43017）时提示「数据已更新，请刷新后重试」。

### 场景 4：删除状态（无引用）
对已停用的状态点「删除」→ 二次确认后删除成功，列表刷新。

### 场景 5：删除状态（有 FSM 引用，错误码 43020）
点「删除」→ 后端返回 43020 + `referenced_by` 列表 → 弹窗展示「以下 FSM 配置引用了此状态，请先修改再删除」+ 引用方列表（名称、中文名、启用状态）。

### 场景 6：删除前未停用（错误码 43016）
对仍启用的状态点「删除」→ EnabledGuardDialog 弹出，提示需先停用，提供「立即停用」按钮。

### 场景 7：查看详情
点「查看」→ 所有字段只读展示（包含 created_at、updated_at）。

## 依赖分析

**依赖**：
- `fsm-state-dict-backend` 已完成（8 个接口、错误码 43013–43020）

**依赖本需求的工作**：
- FSM 配置编辑页（未来）：状态名下拉框的数据来自本管理页维护的字典
- fsm-management-frontend（未来）：会在同一侧边栏分组 `group-fsm` 下追加「状态机配置」菜单项

## 改动范围

| 文件 | 改动类型 |
|------|----------|
| `frontend/src/api/fsmStateDicts.ts` | 新增，API 封装 + TypeScript 类型 |
| `frontend/src/views/FsmStateDictList.vue` | 新增，列表页 |
| `frontend/src/views/FsmStateDictForm.vue` | 新增，创建/编辑/查看表单 |
| `frontend/src/router/index.ts` | 修改，追加 4 条路由 |
| `frontend/src/components/AppLayout.vue` | 修改，新增 `group-fsm` 子菜单（含状态字典入口） |
| `frontend/src/components/EnabledGuardDialog.vue` | 修改，扩展 `entityType` 支持 `'fsm-state-dict'` |

共 6 个文件，2 新建 + 1 新建 API + 3 修改。

## 扩展轴检查

- **新增配置类型**：本需求本身就是新增一个独立的配置管理入口（状态字典），符合「只需加一组 views/api/router 条目」的扩展方向，正面验证。
- **新增表单字段**：不涉及 SchemaForm 动态渲染，分类字段为可选 datalist 输入，不需要改表单渲染引擎。

## 验收标准

| 编号 | 验收标准 |
|------|----------|
| R1 | 侧边栏出现「状态机管理 → 状态字典」菜单项，点击跳转到列表页 |
| R2 | 列表页展示 name / display_name / category / enabled / created_at 五列，默认分页 20 条 |
| R3 | 按 name 模糊搜索 + category 精确下拉（动态加载）+ enabled 三态下拉，组合过滤正确 |
| R4 | 新建表单：name 格式校验（`^[a-z][a-z0-9_]*$`）+ 失焦唯一性校验（check-name），display_name / category 必填，description 可选 |
| R5 | 创建成功后跳回列表并刷新；创建时 name 已存在（43013）显示错误，不跳转 |
| R6 | 编辑表单：name 只读，其余字段可改，提交携带 version |
| R7 | 版本冲突（43017）：弹窗提示「数据已更新，请刷新后重试」，不跳转 |
| R8 | 列表行启用/停用 Switch：点击弹二次确认；确认后拉取最新 version 再 toggle；操作成功刷新该行；版本冲突刷新列表 |
| R9 | 删除已停用状态（无引用）：二次确认后删除，列表刷新 |
| R10 | 删除已启用状态（43016）：EnabledGuardDialog 弹出，提供「立即停用」按钮 |
| R11 | 删除被 FSM 引用状态（43020）：弹窗展示 referenced_by 列表（名称 + 中文名 + 启用状态标签），不删除 |
| R12 | 查看页：所有字段只读，含 created_at / updated_at |
| R13 | `npx vue-tsc --noEmit` 无类型错误；`npm run build` 构建成功 |

## 不做什么

- 不做批量操作（批量启用/停用/删除）
- 不做 name 在编辑时可修改（后端明确禁止）
- 不做 category 的枚举管理页面（category 为自由字符串，下拉只作辅助输入）
- 不做 FSM 配置管理页（属于 fsm-management-frontend spec 范围）
- 不做导入/导出功能
