# fsm-state-dict-polish — 设计方案

## 方案描述

### 1. 后端：新增 `fsm_state_category` 字典组

**seed**：在 `backend/cmd/seed/main.go` 追加 5 条 `fsm_state_category` 字典数据：

| name | label | sort_order |
|------|-------|-----------|
| general | 通用 | 1 |
| combat | 战斗 | 2 |
| movement | 移动 | 3 |
| social | 社交 | 4 |
| activity | 活动 | 5 |

> 注意：name 由英文改为有语义的英文标识（`general`/`combat`/`movement`/`social`/`activity`），label 为中文展示名。现有 fsm_state_dicts 表的 category 列存的是中文（通用/战斗/移动/社交/活动），**不需要迁移**——category 是自由文本，前端新建/编辑时改用字典 name，但历史数据仍展示原值。

**常量**：在 `backend/internal/util/const.go` 追加：
```go
DictGroupFsmStateCategory = "fsm_state_category"
```

**删除路由**：`backend/internal/router/router.go` 移除：
```go
fsmStateDicts.POST("/list-categories", handler.WrapCtx(h.FsmStateDict.ListCategories))
```

**删除 handler 方法**：`backend/internal/handler/fsm_state_dict.go` 删除 `ListCategories` 方法（约 10 行）。

**删除 service 方法**：`backend/internal/service/fsm_state_dict.go` 删除 `ListCategories` 方法（约 8 行）。

**保留 store 方法**：`store/mysql/fsm_state_dict.go` 的 `ListCategories` 保留（SQL 逻辑无害，将来可能复用），仅移除上层调用链。

### 2. 前端：FsmStateDictList.vue

**列顺序调整**：
```
ID(70) / 状态标识(160) / 中文标签(160) / 状态分类(120) / 启用(80) / 创建时间(180) / 操作(160)
```

**搜索字段调整**：
- 搜索框 `v-model` 从 `query.name` 改为 `query.display_name`
- placeholder 改为「搜索中文标签」
- 后端 list API 的 `FsmStateDictListQuery.name` 字段改为 `display_name`（前端 TS 类型 + API 调用同步修改）
- 分类下拉从 `<datalist>` 改为 `el-select`，选项从 `dictApi.list('fsm_state_category')` 加载

### 3. 前端：FsmStateDictForm.vue

**分类字段**：
- 删除 `<datalist>` 和 `listCategories()` 调用
- 改为 `el-select`，选项从 `dictApi.list('fsm_state_category')` 加载
- `el-option` 的 `label` 展示字典 label（如「通用」），`value` 为字典 name（如 `general`）
- 提交到后端的 `category` 字段值为字典 name

**按钮位置**：保存/取消按钮从独立 `form-card` 移入 `el-form` 底部（`div.form-actions` 内嵌），与 FieldForm.vue 保持一致。

### 4. 前端：fsmStateDicts.ts

- 删除 `listCategories()` 方法
- `FsmStateDictListQuery` 的 `name?: string` 改为 `display_name?: string`（对齐后端搜索语义）

---

## 方案对比

### 备选方案：保留 `listCategories()` 接口，只用字典做下拉展示数据源

即：后端新增字典 seed，前端分类下拉改从字典接口加载，但 `/list-categories` 路由保留。

**不选原因**：
- 两个接口都能返回分类列表，职责重叠，违反"单一权威"原则
- `listCategories` 直查 DB DISTINCT，与字典管理脱钩，后续新增分类时仍需改种子并绕过字典管理
- 字典已经是分类数据的权威管理入口，保留旧接口会造成数据来源分裂

---

## 红线检查

### 通用红线（无违反）
- 无新增配置类型，不涉及"扩展轴"

### Go 红线（无违反）
- 删除方法不引入新代码，无潜在 nil/error 忽略风险

### MySQL 红线（无违反）
- 不改表结构，不改迁移脚本，仅 seed 追加数据

### 前端红线
- **R: 禁止用自由文本输入枚举类值**（red-lines/frontend.md §2）
  - ✅ 分类从 `el-input + datalist` 改为 `el-select`，运营人员只能选字典已有分类，不可任意输入
- **R: 禁止硬编码字典组名**（admin/red-lines.md §4 第7条）
  - ✅ 前端传字符串字面量 `'fsm_state_category'` 是 API 调用参数，后端用 `util.DictGroupFsmStateCategory` 常量。前端 API 层同样应抽常量
  - 实际操作：前端 `dictApi.list('fsm_state_category')` 的字符串与后端常量值对应，**前端不额外定义常量**（仅后端需要，因为后端跨层使用）
- **R: el-form disabled 子组件覆盖**：本次改动不涉及 disabled 属性变更，无风险
- **R: vue-tsc + build 必须通过**：T3/T4 结束后必须验证

### ADMIN 专属红线（无违反）
- 不改游戏服务端数据格式
- 不涉及引用完整性
- **字典组名常量**（admin/red-lines.md §4 第7条）：后端新增 `DictGroupFsmStateCategory` 常量 ✅
- **偏离跨模块代码模式**（admin/red-lines.md §10）：前端改动与 FieldForm 模式完全对齐 ✅

---

## 扩展性影响

- **正面**：category 纳入字典管理后，新增状态分类只需在字典管理页操作，无需改代码。符合"下拉依赖从数据库动态获取"企业级标准。
- **无负面影响**：仅修改已有模块，不影响新增配置类型或新增表单字段的扩展路径。

---

## 依赖方向

```
frontend/views/FsmStateDictList.vue
frontend/views/FsmStateDictForm.vue
  └─ frontend/api/fsmStateDicts.ts
  └─ frontend/api/dictionaries.ts        ← 新增依赖（已有模块）

backend/router/router.go                 ← 删除路由注册
backend/handler/fsm_state_dict.go        ← 删除 ListCategories
backend/service/fsm_state_dict.go        ← 删除 ListCategories
backend/util/const.go                    ← 新增常量（最底层）
backend/cmd/seed/main.go                 ← 追加 seed 数据
```

单向向下，无循环依赖。

---

## 陷阱检查（frontend.md dev-rules）

- **el-select v-model 类型一致**（§3.4）：字典 name 为 string，form.category 类型为 string，一致 ✅
- **v-for 必须有稳定 key**（§2.6）：`el-option :key="item.name"` ✅
- **dialog 关闭数据残留**（§3.3）：本次不改 dialog，不涉及
- **按钮 loading 防重复**（§4.2）：保留原有 `submitting` ref 控制，不受按钮移位影响 ✅
- **搜索字段改动需同步 API 类型**（§7）：`FsmStateDictListQuery.name` 改为 `display_name`，前后端 field 对齐 ✅

---

## 配置变更

无需修改 `config.yaml` / `config.docker.yaml`。仅 seed 数据追加，走现有 DictCache.Load() 即可。

---

## 测试策略

- **R1**：`curl -s -X POST .../dictionaries -d '{"group":"fsm_state_category"}'` 返回 5 条
- **R2/R6**：浏览器验证分类下拉展示 label（如「通用」），提交后后端存 name（如 `general`）
- **R3**：`curl .../fsm-state-dicts/list-categories` 返回 404（路由已删）
- **R4/R5**：浏览器截图验证列头 + 搜索行为
- **R7**：浏览器验证按钮位置
- **R8**：`npx vue-tsc --noEmit && npm run build`
