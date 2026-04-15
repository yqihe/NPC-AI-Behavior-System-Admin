# fsm-state-dict-polish — 需求分析

## 动机

fsm-state-dict-frontend 完成后，发现三处与企业标准不符：

1. **category 硬编码问题**：当前分类通过 `listCategories()` 从 `fsm_state_dicts` 表直接 DISTINCT 查询，不经过字典管理。种子数据里的「通用/战斗/移动/社交/活动」是硬写在 seed 脚本中的，运营人员无法在不改代码的情况下新增分类，也无法修改分类的中文展示名。其他模块（如字段分类 `field_category`）已经走字典管理，应保持一致。

2. **列表页不符合企业规范**：缺少 ID 列；`display_name` 列头叫「中文名」（其他模块一律叫「中文标签」）；`category` 列头叫「分类」（应叫「状态分类」区分语义）；搜索框绑定的是 `name`（状态标识），其他模块均按 `display_name`（中文标签）搜索。

3. **表单分类选择器粗糙**：用的是原生 `<datalist>` + `el-input`，而 FieldForm 等其他表单均使用 `el-select` + `dictApi.list()` 提供规范的选项列表；保存/取消按钮放在独立的 form-card 里，与 FieldForm 的内嵌表单风格不一致。

不修复的话：运营人员看到的 UI 与其他管理页不一致，体验割裂；分类数据无法通过字典管理维护，后续只能改代码新增分类。

## 优先级

高。fsm-state-dict 功能刚上线，趁数据少时修复成本最低；字典接入是架构一致性要求，不能长期遗留。

## 预期效果

**场景 1 — 字典管理**：后端在 dictionaries 表中增加 `fsm_state_category` 组，seed 数据写入 5 个分类（通用/战斗/移动/社交/活动）。前端表单页通过 `dictApi.list('fsm_state_category')` 加载选项，显示为 `el-select`（含 label + name）。

**场景 2 — 列表页标准对齐**：打开「状态字典」列表，第一列是 ID，列头依次为 ID / 状态标识 / 中文标签 / 状态分类 / 启用 / 创建时间 / 操作；搜索框 placeholder 为「搜索中文标签」，按 `display_name` 模糊搜索。

**场景 3 — 分类下拉**：新建/编辑状态字典，「状态分类」字段改为 `el-select`，选项从 dictionaries 加载（展示 label，提交 name）；保存和取消按钮在 `el-form` 底部，与 FieldForm 一致。

## 依赖分析

- **依赖**：`fsm-state-dict-backend`（已完成）、`fsm-state-dict-frontend`（已完成）、字典管理模块（DictCache 已支持任意 group）
- **谁依赖本需求**：无下游依赖，纯 UI/数据一致性修复

## 改动范围

| 层 | 文件 | 改动类型 |
|----|------|----------|
| 后端 seed | `backend/cmd/seed/main.go` | 新增 `fsm_state_category` 种子数据 |
| 后端 util | `backend/internal/util/const.go` | 新增 `DictGroupFsmStateCategory` 常量 |
| 后端 handler | `backend/internal/handler/fsm_state_dict.go` | 删除 `ListCategories` 方法（改由 dictionaries 提供） |
| 后端 service | `backend/internal/service/fsm_state_dict.go` | 删除 `ListCategories` 方法 |
| 后端 store | `backend/internal/store/mysql/fsm_state_dict.go` | 删除 `ListCategories` 方法 |
| 后端 router | `backend/internal/router/router.go` | 移除 `/fsm-state-dicts/list-categories` 路由 |
| 前端 API | `frontend/src/api/fsmStateDicts.ts` | 删除 `listCategories()` 方法 |
| 前端 视图 | `frontend/src/views/FsmStateDictList.vue` | 加 ID 列、改列头、改搜索字段 |
| 前端 视图 | `frontend/src/views/FsmStateDictForm.vue` | 分类改 el-select + dictApi、按钮内嵌 |

估计 9 个文件，全为修改（无新建）。

## 扩展轴检查

- **新增配置类型**：不涉及
- **新增表单字段**：不涉及
- 本 spec 是对已有模块的 UI/数据一致性修复，不影响两个扩展轴

## 验收标准

| # | 标准 | 验证方法 |
|---|------|----------|
| R1 | `GET /api/v1/dictionaries` 传 `{"group":"fsm_state_category"}` 返回 5 条选项（通用/战斗/移动/社交/活动） | curl 验证 |
| R2 | 前端分类下拉通过 `dictApi.list('fsm_state_category')` 加载，展示 label，提交 name | 浏览器验证下拉选项 |
| R3 | `/api/v1/fsm-state-dicts/list-categories` 路由已删除，调用返回 404 | curl 验证 |
| R4 | 列表页第一列为 ID（70px），列头依次 ID / 状态标识 / 中文标签 / 状态分类 / 启用 / 创建时间 / 操作 | 浏览器截图 |
| R5 | 搜索框绑定 `display_name`，输入「空闲」能过滤出对应记录 | 浏览器验证 |
| R6 | 新建/编辑表单「状态分类」为 `el-select`，选项来自字典，不可手动输入任意值 | 浏览器验证 |
| R7 | 保存/取消按钮在 `el-form` 内部底部（不在独立的 form-card 中） | 浏览器截图 |
| R8 | `npx vue-tsc --noEmit` 0 errors；`npm run build` 成功 | 命令输出 |

## 不做什么

- 不修改 fsm_state_dicts 表的 category 字段（仍存 name，无需 JOIN 查 label）
- 不给 category 增加外键约束（字典管理是软约束，已有数据可能用旧 name）
- 不迁移历史数据的 category 值（种子数据与字典 name 已对应，无需迁移）
- 不删除 `listCategories` 对应的数据库查询逻辑（只删接口和路由，SQL 方法保留供将来可能复用）
- 不改字典管理 UI（DictCache 已支持，不需要改）
