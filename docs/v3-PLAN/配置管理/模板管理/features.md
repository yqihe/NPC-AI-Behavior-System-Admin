# 模板管理 — 功能清单

> 通用架构/规范见 `docs/architecture/overview.md` 和 `docs/development/`。
> 后端实现细节见同目录 `backend.md`，前端设计见 `frontend.md`。

---

## 状态模型

| 状态 | 模板管理页看到 | NPC 管理页看到 | 能被新 NPC 选择 | 已有 NPC |
|------|-------------|------------|--------------|---------|
| 启用 | 可见，正常显示 | 可见可选 | 允许 | 正常 |
| 停用 | 可见，整行变灰 | 不可见 | 拒绝 | 不受影响（NPC 创建时已快照）|
| 已删除 | 不可见 | 不可见 | 不可能 | 已清理 |

核心原则：**停用是"存量不动，增量拦截"；删除才真正清理引用关系。**

模板和字段的关系：**NPC 创建后独立于模板**（NPC 把字段列表+值快照下来），模板的后续变更对已有 NPC 无影响。

**关于"详情页"**：模板没有独立的只读详情页，**编辑页本身就承担了"查看 + 修改"双重角色**。被引用时字段区锁死，相当于带轻量编辑能力的详情视图。

---

## 模块职责边界

模板管理严格遵守"分层职责"硬规则。`TemplateService` **只持有自身的 `TemplateStore` / `TemplateCache`**，**不持有** `FieldStore` / `FieldRefStore` / `FieldCache` / `DictCache`。所有跨模块的事情——字段存在性/启用校验、`field_refs` 维护、字段 `ref_count` 维护、字段详情补全、字段方缓存清理——都由 `TemplateHandler` 作为"用例编排者"显式调 `FieldService` 的对外方法（参见字段管理 features 功能 12）。

**跨模块事务的打开位置**：Create / Update / Delete 三个写路径里，`TemplateHandler` 自己用 `h.db.BeginTxx(ctx, nil)` 开事务并 `defer tx.Rollback()`，然后把 `*sqlx.Tx` 同时传给 `TemplateService.*Tx` 和 `FieldService.*Tx` 方法；`tx.Commit()` 之后再分别调两个 Service 的 `Invalidate*` 方法清各自的缓存。Service 层之间互不调用，只有 handler 扮演"跨模块裁判"角色。

对外，`TemplateService` 暴露的跨模块接口是只读的：`GetByIDsLite` 和 `ExistsByName`，给字段管理的 handler 补 label / 做预查使用（参见功能 11）。

---

## 功能 1：模板列表

### 场景描述

**场景 A — 在模板管理页，管理员要浏览所有模板。** 不传 `enabled` 筛选条件，启用和停用的模板都展示出来，管理员才能对停用模板做重新启用或删除操作。

**场景 B — 在 NPC 管理页，策划要从下拉框选一个模板创建 NPC。** 传 `enabled=true`，只展示启用的模板。停用的模板不应该出现在选择列表中，避免策划选了一个不可用的模板。

两个场景走同一个接口，靠 `TemplateListQuery.Enabled (*bool)` 三态区分。支持按中文标签模糊搜索（`Label`）、启用状态精确筛选、后端分页（Service 层按 `pagCfg` 校正上下界）。列表项包含 `id`，前端用 id 发起后续操作。

**排序规则**：按 `id` DESC（id 与 `created_at` 同向，新建的 id 必然更大），覆盖索引 `idx_list (deleted, enabled, id DESC)` 直接命中，不需要额外的 `created_at` 索引。

**列表项字段**：`id, name, label, enabled, ref_count, created_at`（**不返回** `fields` / `description`，减小网络传输）。

**列表展示规范：**

| 列 | 说明 |
|---|---|
| ID | 主键，倒序 |
| 模板标识 | `name`，等宽显示 |
| 中文标签 | `label`，主信息，列宽自适应 |
| 被引用数 | `ref_count`，蓝色高亮（点击拉起引用详情弹窗，对应功能 6）|
| 启用 | `enabled` 开关，绿/灰二态 |
| 创建时间 | `created_at` |
| 操作 | `编辑` `删除` 两个文字按钮，蓝/红配色 |

**视觉规则：**
- **停用模板**：除"操作"列外，整行 opacity 0.5（行号、标识、标签、被引用数、开关、创建时间一起变灰）；**操作列保持高亮**，让管理员能正常点击编辑/删除来处理停用模板。
- **不需要"已停用"文字标签** —— 整行变灰已经是足够强的视觉信号。
- **不展示"描述"和"字段数"列** —— 描述是创建时的辅助说明，列表场景下噪音大于价值；字段数从 `fields` JSON 拿要么回表要么冗余存，价值不抵成本。需要时点进编辑页一目了然。

### 校验规则

无 Handler 层校验（直接透传 query）。Service 层校正分页参数上下界。

### 调用链

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/templates/list` |
| Handler | `TemplateHandler.List` — 直接透传 query |
| Service | `TemplateService.List` — 分页参数校正 → 查 Redis 列表缓存 → miss 时查 MySQL → 写缓存 |
| Store | `TemplateCache.GetList` → `TemplateStore.List`（覆盖索引，`ORDER BY id DESC`）→ `TemplateCache.SetList` |

### 错误码

无专属错误码。

### 边界 case

- Redis 挂了跳过缓存，降级直查 MySQL。

---

## 功能 2：新建模板（跨模块事务）

### 场景描述

在模板管理页，管理员要定义一个新的"字段组合方案"（比如"战斗生物模板"、"场景NPC模板"）。填写模板标识、中文标签、描述，从启用字段中勾选所需字段，并为每个字段标记是否必填后提交。

新建的模板默认是**未启用**状态（`enabled=false`）。这是一个刻意的设计：管理员创建模板后，往往还需要反复调整字段勾选、必填配置。如果创建即启用，NPC 管理页的下拉列表会立刻出现这个半成品模板，策划可能在管理员还没配好之前就选了它。默认未启用就提供了一个"配置窗口期"。

模板标识（`name`）一旦创建不可修改（是唯一键），且含软删除记录也不能重复使用，防止历史数据混乱。

#### 字段勾选交互（按字段管理实际分类分组展示）

- **数据来源**：调字段列表接口 `enabled=true`，只展示启用的字段
- **按字段的 `category` 分组展示**：分组对应字段管理的字典（dictionary `field_category`），目前 6 类：

  | category key | 中文标签 |
  |---|---|
  | basic | 基础属性 |
  | combat | 战斗属性 |
  | perception | 感知属性 |
  | movement | 移动属性 |
  | interaction | 交互属性 |
  | personality | 个性属性 |

  **前端必须用接口返回的 `category_label`，禁止硬编码"基础属性 / 战斗数值 / 行为配置"等任何自造词** —— 字段管理新增分类时模板管理无需改前端。

- 每个分类一个折叠区块，区块标题显示分类的中文标签 + 已选/总数（如 `战斗属性 (3/5)`）
- **每行 3 个字段**的网格布局，每个单元格是一个复选框 + 字段标签 + 字段标识（`name · type`）
- **普通字段**：直接复选框勾选
- **reference 类型字段**：单元格上有特殊视觉标记（紫色边框 + `link-2` 图标 + `reference` 紫色徽章 + 右侧 chevron），**点击单元格弹出浮层（popover）**展示子字段（详见功能 8）
- 存储结果：扁平的实际字段 ID 列表，**无 reference 痕迹**
- 同一字段被多个 reference 引用、或既被直接勾选又被某个 reference 包含时，**自动去重**

#### 已选字段配置交互（必填 + 排序）

- 上半部分勾选字段后，下半部分"已选字段配置区"自动同步增删行
- 每行展示：字段标签 / 字段标识 / 字段类型 tag / 必填 checkbox / **上下移动按钮**
- 必填默认为 `false`，由管理员勾选
- **排序**：每行末尾两个 `↑ ↓` 图标按钮：
  - 第一行 `↑` 灰色禁用，最后一行 `↓` 灰色禁用，其余两个都可点
  - 点击 `↑` / `↓` 在前端 splice 数组，直接渲染新顺序
  - **允许跨分类移动** —— 顶部"字段选择"的分类只是为了选择时方便，最终 NPC 表单按"已选字段配置"的实际数组顺序渲染，而不是按 category 重新分组
  - 数组顺序即 `templates.fields` JSON 的存储顺序，也是 NPC 表单的展示顺序
  - 排序变化与字段勾选属于同一个"字段变更"语义：被引用模板（`ref_count > 0`）排序按钮也整列灰化禁用（后端 `isFieldsChanged` 会一并拦截，见功能 4）

**保存按钮**：文案就是 `保存` 两个字，**不要在按钮里写"默认未启用"**之类的提示 —— 如有需要可作为按钮旁的副文案，但不污染按钮本身的可读性。

### 校验规则

- **Handler**（`TemplateHandler.Create`）格式校验：
  - `identPattern` 校验 `name`
  - `label` 非空且长度 ≤ `valCfg.FieldLabelMaxLength`
  - `description` 长度 ≤ 512 字符
  - `fields` 非空且 `field_id > 0` 不重复
- **事务外预检**：
  - `templateService.ExistsByName(ctx, name)` 查 name 唯一性（给前端早失败）→ `41001`
  - `fieldService.ValidateFieldsForTemplate(ctx, fieldIDs)` 校验勾选字段全部存在 + 启用 + 非 reference 类型 → `41005` / `41006` / `41012`
- **事务内**（`TemplateService.CreateTx`）：
  - fields 基础校验 + name 唯一性（兜底）+ 序列化 `fields` JSON

### 调用链

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/templates/create` |
| Handler | `TemplateHandler.Create` — 格式校验 → `ExistsByName` 预查 → `ValidateFieldsForTemplate` → 开 tx → `CreateTx` → `AttachToTemplateTx` → Commit → 清两方缓存 |
| Service | `TemplateService.CreateTx`（事务内）+ `FieldService.ValidateFieldsForTemplate`（事务外）+ `FieldService.AttachToTemplateTx`（事务内）|
| Store | `TemplateStore.CreateTx` → `FieldRefStore.Add` + `FieldStore.IncrRefCountTx` |

**后端跨模块事务流程（Handler 编排）：**

1. 格式校验
2. 事务外预检：`ExistsByName` + `ValidateFieldsForTemplate`
3. `h.db.BeginTxx(ctx, nil)` 开事务，`defer tx.Rollback()`
4. `templateService.CreateTx(ctx, tx, req)` — 序列化 fields JSON + 写入
5. `fieldService.AttachToTemplateTx(ctx, tx, templateID, fieldIDs)` — 写 `field_refs` + `IncrRefCountTx`
6. `tx.Commit()`
7. Commit 后分别清两个模块缓存：`templateService.InvalidateList` + `fieldService.InvalidateDetails`

### 错误码

| 错误码 | 常量 | 触发场景 |
|--------|------|---------|
| 41001 | `ErrTemplateNameExists` | name 已存在 |
| 41002 | `ErrTemplateNameInvalid` | name 格式不合法（handler 前置） |
| 41004 | `ErrTemplateNoFields` | 未勾选任何字段 |
| 41005 | `ErrTemplateFieldDisabled` | 勾选了停用字段（由 `FieldService.ValidateFieldsForTemplate` 抛出） |
| 41006 | `ErrTemplateFieldNotFound` | 勾选的字段不存在（由 `FieldService.ValidateFieldsForTemplate` 抛出） |
| 41012 | `ErrTemplateFieldIsReference` | 勾选了 reference 类型字段（由 `FieldService.ValidateFieldsForTemplate` 抛出） |
| 40000 | `ErrBadRequest` | label / description / fields 格式错误 |

### 边界 case

- `name` 含软删除记录也不可复用。
- reference 类型字段不能直接加入模板（前端的 reference popover 只是快捷选择器，写入的只有 leaf 子字段 ID）。

---

## 功能 3：模板详情（跨模块拼装）

### 场景描述

**场景 A — 编辑入口**：管理员在列表点"编辑"，前端先调 detail 接口拿到完整模板（含字段精简信息），再渲染 `TemplateForm.vue`。

**场景 B — NPC 创建**：NPC 管理页选中模板后要知道模板有哪些字段、哪些必填，然后按字段 `id` 再调**字段详情接口**拿 `properties` 渲染动态表单。

### 校验规则

- **Handler**：`id > 0`。

### 调用链

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/templates/detail` |
| Handler | `TemplateHandler.Get` — 校验 `id > 0` → 取模板裸行 → 解 `fields` → 跨模块调 `fieldService.GetByIDsLite` → 拼装 `TemplateDetail` |
| Service | `TemplateService.GetByID`（Cache-Aside + 防击穿防穿透）+ `TemplateService.ParseFieldEntries`（公开工具方法）+ `FieldService.GetByIDsLite`（跨模块）|
| Store | `TemplateCache.GetDetail` → `TryLock` → `TemplateStore.GetByID` → `TemplateCache.SetDetail` |

**后端职责分层 & 跨模块拼装流程：**

`TemplateService.GetByID` **只返回 `*model.Template` 裸行**（含未解析的 `fields` JSON），它内部走自己的 Cache-Aside：
1. 查 `TemplateCache.GetDetail`，命中即返（命中空标记时返回 `ErrTemplateNotFound`）
2. miss 时 `TemplateCache.TryLock(id, 3s)` 防击穿 + double-check
3. 锁失败不阻塞，降级直查 `TemplateStore.GetByID`
4. `tpl=nil` 时写空标记防穿透

`TemplateHandler.Get` 拿到裸行之后做跨模块拼装：
1. `templateService.ParseFieldEntries(tpl.Fields)` 解出 `[]TemplateFieldEntry{FieldID, Required}`
2. 提取 `fieldIDs` 数组（保持顺序）
3. `fieldService.GetByIDsLite(ctx, fieldIDs)` — **按 `fieldIDs` 顺序对齐**返回 `[]FieldLite`，Service 内部用 `DictCache` 翻译 `CategoryLabel`
4. 按 entries 顺序组装 `[]TemplateFieldItem`，每项带 `FieldID / Name / Label / Type / Category / CategoryLabel / Enabled / Required`；缺失字段（理论上不应发生）`slog.Warn` 并跳过
5. 和模板基本信息一起包装成 `TemplateDetail` 返回

**为什么 `TemplateDetail` 不进缓存**：`FieldLite.Enabled` 依赖字段**当前**状态，如果把组装后的详情缓存到模板方，字段被停用时就得同时清模板详情缓存，耦合链太长。分层做法是：模板方只缓存裸行（受字段写影响小），字段方有自己的 detail 缓存（受模板写影响大），拼装每次都在 handler 层发生——两边命中各自的 cache，拼装本身开销极小。

**详情响应字段：**
- **模板基本信息**：`id, name, label, description, enabled, version, ref_count, created_at, updated_at`
- **字段列表（精简）**：每项 `{field_id, name, label, type, category, category_label, enabled, required}`
  - **不返回完整 `properties`**（NPC 管理页要渲染表单时会再调字段详情接口拿 properties）
  - **包含 `category` 与 `category_label`**，前端按分类分组展示
  - **字段已被停用时 `enabled=false`**，前端在字段卡中标灰色 + 警告图标，提示运营人员但仍保留引用关系

### 错误码

| 错误码 | 常量 | 触发场景 |
|--------|------|---------|
| 41003 | `ErrTemplateNotFound` | 模板不存在（或命中空标记） |
| 40000 | `ErrBadRequest` | `id` 不合法 |

### 边界 case

- 停用字段在详情中 `enabled=false`，前端标灰但保留引用关系。
- 缺失字段（理论上不应发生）`slog.Warn` 并跳过。

---

## 功能 4：编辑模板（兼具查看，跨模块事务）

### 场景描述

**场景 A — 管理员点击列表行的「编辑」，查看或修改一个未引用的模板。** 字段列表、必填配置、label/描述全部可改（但仍必须先停用）。

**场景 B — 管理员点击「编辑」一个已被 NPC 引用的模板。** 字段勾选区和必填配置区整体灰化只读，仅 label/描述可改。这种状态相当于"带轻量编辑能力的详情视图"。

编辑权限按"是否被 NPC 引用"分两档：

| 模板状态 | label / 描述 | 字段勾选 | 必填配置 | 字段顺序 |
|---------|-------------|---------|---------|----------|
| 无 NPC 引用（`ref_count=0`）| 可改 | 可加可减 | 可改 | 可改 |
| 有 NPC 引用（`ref_count>0`）| 可改 | **完全锁死** | **完全锁死** | **完全锁死** |

**为什么有 NPC 引用就锁字段列表**：NPC 创建后是独立的（字段列表+值已经快照），模板改字段对存量 NPC 没有任何实际影响。但如果允许随意改，模板就退化成了"全局可变的字段组"，失去了"可复用的配置方案"这个语义——策划永远在老模板上加字段，永远不创建新模板。

**只有未启用状态才能编辑** —— 启用中的模板已对外可见，允许随意编辑会导致 NPC 管理页看到不稳定的配置。试图编辑启用中的模板时返回 `41010 ErrTemplateEditNotDisabled`，前端弹"无法编辑"引导弹窗（详见功能 9）。

#### Service 层的 `fieldsChanged` 语义

`TemplateService.UpdateTx` 内用 `isFieldsChanged(old, new)` 判断 `fields` 是否变更——**集合、顺序、`required` 任一不同都算"变更"**：

```go
func isFieldsChanged(old, new []model.TemplateFieldEntry) bool {
    if len(old) != len(new) { return true }
    for i := range old {
        if old[i].FieldID != new[i].FieldID { return true }   // 集合 or 顺序
        if old[i].Required != new[i].Required { return true } // required
    }
    return false
}
```

这意味着：**单纯调整 required 或字段顺序，在 `ref_count > 0` 时也会被 `41008 ErrTemplateRefEditFields` 拒绝**。这是有意为之——排序决定 NPC 表单展示顺序，required 决定 NPC 创建校验，两者对已有 NPC 虽无直接影响但语义上属于"模板配置"，被 NPC 引用后统一锁死更符合"字段变更"的粗粒度语义。

**注意对 add/remove 的处理**：`isFieldsChanged=true` 但 `diffFieldIDs` 算出的 `toAdd` / `toRemove` 都为空（即"纯排序或纯 required 变化"）时，Service 仍然会更新 `fields` JSON（因为 `fields` 字段顺序本身有业务语义），但**不操作 `field_refs`**——handler 识别 `fieldsChanged && (len(toAdd)+len(toRemove) > 0)` 才调 Detach/Attach。此时只清模板自己的缓存，不打扰字段方缓存。

#### 前端实现：一个 `TemplateForm.vue` 同时承载新建 + 两种编辑状态

布局结构与新建页**完全一致**（基本信息 + 字段选择 + 已选字段配置 + 底部按钮），只通过下面 3 个 prop 切换：

| prop | 说明 |
|---|---|
| `mode: 'create' \| 'edit'` | 决定 标题文案、调 create / update 接口、`name` 字段是否 readonly |
| `refCount: number` | `>0` 时整体进入锁定态：顶部显示黄色警告条、字段卡和必填卡 opacity 0.55 + 卡标题旁加锁定 tag |
| `template: TemplateDetail` | edit 模式下回填，create 模式下为空 |

**编辑页与新建页的差异点（编辑特有）：**
- 顶部 SubHeader 标题改为 `编辑模板`，副标题展示模板的中文 label
- 模板标识 input：灰底 + lock 图标 + readonly；hint 改为"模板标识创建后不可修改"；不显示 `*`
- 已被引用时（`refCount > 0`）：顶部添加黄色警告条 "该模板已被 N 个 NPC 引用，字段勾选与必填配置不可修改"
- 已被引用时：SubHeader 右侧多一个橙色 tag "被 N 个 NPC 引用"
- 已被引用时：字段选择卡 + 已选字段配置卡整体 opacity 0.55，卡片标题右侧加锁定 tag

**reference 字段在编辑页的特殊语义**：reference 字段本身**不存在于模板数据中**（详见功能 8），模板只存了展开后的扁平字段 ID 列表。但在编辑页仍需展示 reference 字段单元格，因为用户当初是通过点击 reference 来批量选字段的，再次编辑时仍然要保留这个交互入口。reference 弹层每次都实时拉最新子字段列表，把"当前模板里已经有的子字段"勾选回来。不做"+N 新子字段"的差异提示——实现成本不抵收益。

### 校验规则

- **Handler**：`id > 0`、`label` 长度、`description ≤ 512`、`fields` 非空且不重复、`version > 0`
- **事务外预校验新增字段**：Handler 本地 `diffNewFieldIDs(oldEntries, req.Fields)` 快速算出 `toAddPre`，仅对**新增**字段调 `fieldService.ValidateFieldsForTemplate(ctx, toAddPre)`——校验存在 / 启用 / 非 reference 三项（`41005` / `41006` / `41012`）
- **Service**（`TemplateService.UpdateTx`）：
  - `old.Enabled` 必须为 false → `41010`
  - `isFieldsChanged` 判断
  - `old.RefCount > 0 && fieldsChanged` → `41008`
  - 乐观锁写入 → `41011`

### 调用链

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/templates/update` |
| Handler | `TemplateHandler.Update` — 格式校验 → `GetByID` + `ParseFieldEntries` → `ValidateFieldsForTemplate(toAddPre)` → 开 tx → `UpdateTx` → 条件 Detach/Attach → Commit → 清两方缓存 |
| Service | `TemplateService.UpdateTx`（enabled / ref / diff / 乐观锁）+ `FieldService.ValidateFieldsForTemplate` / `DetachFromTemplateTx` / `AttachToTemplateTx` |
| Store | `TemplateStore.GetByID` → `TemplateStore.UpdateTx` WHERE id=? AND version=? → `FieldRefStore.Remove` / `FieldRefStore.Add` + `FieldStore.DecrRefCountTx` / `IncrRefCountTx` |

**后端跨模块事务流程：**

1. 格式校验
2. 拿旧状态：`templateService.GetByID` + `ParseFieldEntries`
3. 事务外预校验新增字段：`fieldService.ValidateFieldsForTemplate(toAddPre)`
4. `h.db.BeginTxx(ctx, nil)` 开事务
5. `templateService.UpdateTx(ctx, tx, req, old, oldEntries)` — enabled / ref / diff / 乐观锁 → 返回 `(fieldsChanged, toAdd, toRemove, error)`
6. 若 `fieldsChanged && (len(toAdd) > 0 || len(toRemove) > 0)`：先 Detach 再 Attach
7. `tx.Commit()`
8. 清缓存：模板 detail + 列表 + 字段 details（若有 Detach/Attach）

### 错误码

| 错误码 | 常量 | 触发场景 |
|--------|------|---------|
| 41003 | `ErrTemplateNotFound` | 模板不存在 |
| 41004 | `ErrTemplateNoFields` | 未勾选任何字段 |
| 41005 | `ErrTemplateFieldDisabled` | 新增字段已停用 |
| 41006 | `ErrTemplateFieldNotFound` | 新增字段不存在 |
| 41008 | `ErrTemplateRefEditFields` | 被 NPC 引用，无法编辑字段列表 |
| 41010 | `ErrTemplateEditNotDisabled` | 编辑前必须先停用 |
| 41011 | `ErrTemplateVersionConflict` | 乐观锁版本冲突 |
| 41012 | `ErrTemplateFieldIsReference` | 新增字段是 reference 类型 |
| 40000 | `ErrBadRequest` | 格式/必填校验失败 |

### 边界 case

- `name` 不可修改。
- 纯排序/纯 required 变化时更新 `fields` JSON 但不操作 `field_refs`。
- `toRemove` 无需校验启用或类型（已经在模板里，保持"存量不动"）。

---

## 功能 5：删除模板（跨模块事务）

### 场景描述

在模板管理页，管理员要彻底移除一个不再需要的模板。

删除有两道门槛：
1. **必须先停用**（`41009 ErrTemplateDeleteNotDisabled`）。这是给管理员一个缓冲期——停用后观察一段时间，确认没有问题再删。
2. **不能有 NPC 引用**（`41007 ErrTemplateRefDelete`）。如果还有 NPC 在使用它，删不掉；前端自动调用引用详情接口告诉管理员去哪里解除引用。

删除是软删除（标记 `deleted=1`），不是物理删除。删除时会在同一事务中清理模板对所有字段的引用关系（`field_refs` 中所有 `ref_type='template'` 且 `ref_id=templateID` 的记录），并把对应字段的 `ref_count` 减回去。

### 校验规则

- **Handler**：`id > 0`
- **Service/Handler**：`enabled=false` → 事务内 `GetRefCountForDeleteTx`（FOR SHARE）→ `ref_count == 0`

### 调用链

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/templates/delete` |
| Handler | `TemplateHandler.Delete` — 校验 → `GetByID` 校验 `enabled` → `ParseFieldEntries` → 开 tx → `GetRefCountForDeleteTx`（FOR SHARE）→ `SoftDeleteTx` → `DetachFromTemplateTx` → Commit → 清两方缓存 |
| Service | `TemplateService.GetByID` / `GetRefCountForDeleteTx` / `SoftDeleteTx` + `FieldService.DetachFromTemplateTx` |
| Store | `TemplateStore.GetByID` → `TemplateStore.GetRefCountTx`（FOR SHARE）→ `TemplateStore.SoftDeleteTx` → `FieldRefStore.Remove` + `FieldStore.DecrRefCountTx` |

**后端跨模块事务流程：**

1. Handler 校验 `id > 0`
2. `templateService.GetByID(ctx, id)` 查模板；若 `enabled=true` 返回 `41009`
3. `templateService.ParseFieldEntries(tpl.Fields)` 拿到要 detach 的 `fieldIDs`
4. `h.db.BeginTxx(ctx, nil)` 开事务，`defer tx.Rollback()`
5. `templateService.GetRefCountForDeleteTx(ctx, tx, id)` 用 `FOR SHARE` 加读锁查 `ref_count`（防 TOCTOU）。若 `ref_count > 0` 返回 `41007`
6. `templateService.SoftDeleteTx(ctx, tx, id)` — 软删 `templates` 行
7. `fieldService.DetachFromTemplateTx(ctx, tx, id, fieldIDs)` — 批量删 `field_refs` + `DecrRefCountTx`
8. `tx.Commit()`
9. 清缓存：`templateService.InvalidateDetail` + `templateService.InvalidateList` + `fieldService.InvalidateDetails(affected)`

### 错误码

| 错误码 | 常量 | 触发场景 |
|--------|------|---------|
| 41003 | `ErrTemplateNotFound` | 模板不存在 |
| 41007 | `ErrTemplateRefDelete` | 被 NPC 引用，无法删除 |
| 41009 | `ErrTemplateDeleteNotDisabled` | 删除前必须先停用 |
| 40000 | `ErrBadRequest` | `id` 不合法 |

### 边界 case

- 事务内 `GetRefCountForDeleteTx` 用 `FOR SHARE` 防 TOCTOU 竞态。
- 删除时同一事务内清理所有字段引用关系 + 递减 `ref_count`。

---

## 功能 6：模板引用详情

### 场景描述

**场景 A — 在模板管理页，管理员想停用或删除某个模板之前，先看看哪些 NPC 在用它。** 列表页直接点击"被引用数"单元格上的蓝色数字即可拉起此弹窗。

**场景 B — 删除接口返回 `41007` 后，前端自动调用此接口展示引用详情，告诉管理员应该先去哪里解除引用。**

### 校验规则

- **Handler**：`id > 0`。

### 调用链

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/templates/references` |
| Handler | `TemplateHandler.GetReferences` — 校验 `id > 0` → `templateService.GetByID` → 返回 `{template_id, template_label, npcs: []}` 占位 |
| Service | `TemplateService.GetByID` |

**当前实现**：NPC 模块未上线前，handler 先调 `templateService.GetByID` 拿模板基本信息，然后返回 `NPCs: make([]TemplateReferenceItem, 0)` 空数组占位（用 `make` 而不是 nil，以避免 JSON 序列化成 `null`）。NPC 模块上线后再在 handler 层跨模块调 `NPCService` 填充真实数据。

弹窗内容（前端规划）：模板基本信息（label / name / 总引用数）+ NPC 名称搜索框 + NPC 列表（id / name / 创建时间 / 「查看」跳转按钮），下方支持「加载更多」分页。

### 错误码

| 错误码 | 常量 | 触发场景 |
|--------|------|---------|
| 41003 | `ErrTemplateNotFound` | 模板不存在 |
| 40000 | `ErrBadRequest` | `id` 不合法 |

### 边界 case

- NPC 模块未上线前返回空数组占位。

---

## 功能 7：启用 / 停用切换

### 场景描述

**场景 A — 在模板管理页，管理员新建完模板、确认配置无误后，启用它。** 启用后 NPC 管理页的模板下拉列表才能看到这个模板。

**场景 B — 在模板管理页，管理员要下线一个模板，先停用它。** 停用后：
- NPC 管理页的下拉列表立刻看不到它了，策划不会再用它创建新 NPC
- 但已经基于它创建的 NPC 不受影响（NPC 已快照独立）
- 如果确认不再需要，后续再执行删除

**停用一个被 NPC 引用的模板是允许的**。这是"存量不动，增量拦截"的设计：已经在用的不打扰，新的不让用。

切换用乐观锁，版本冲突返回 `41011 ErrTemplateVersionConflict`。此操作**不涉及字段模块**，纯 `TemplateService.ToggleEnabled` 单模块路径。

### 校验规则

- **Handler**：`id > 0`、`version > 0`。
- **Service**：按 ID 查 → 乐观锁更新。

### 调用链

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/templates/toggle-enabled` |
| Handler | `TemplateHandler.ToggleEnabled` — 校验 `id > 0`、`version > 0` |
| Service | `TemplateService.ToggleEnabled` — 按 ID 查 → 乐观锁更新 → 清 detail + 列表缓存 |
| Store | `TemplateStore.ToggleEnabled(id, enabled, version)` WHERE id=? AND version=? |

### 错误码

| 错误码 | 常量 | 触发场景 |
|--------|------|---------|
| 41003 | `ErrTemplateNotFound` | 模板不存在 |
| 41011 | `ErrTemplateVersionConflict` | 乐观锁版本冲突 |
| 40000 | `ErrBadRequest` | `id` / `version` 不合法 |

### 边界 case

- 被 NPC 引用的模板也可以停用。

---

## 功能 8：reference 字段弹层勾选（前端交互）

### 场景描述

在新建/编辑模板的字段勾选区，管理员看到一个 reference 类型字段的单元格。该单元格在网格中和普通字段一样占一格，但有特殊视觉标记（紫色边框 + `link-2` 图标 + reference 紫色徽章 + 右侧 chevron）。**鼠标点击该单元格时弹出一个浮层（popover）**，浮层中展示该 reference 引用的所有子字段，管理员可以从中**勾选一部分**（不必全选），点击浮层外部或确认按钮关闭浮层。

### 校验规则

前端交互功能，无后端校验。

### 调用链

此功能内嵌在功能 2（新建）、功能 4（编辑）的前端逻辑中，后端不做特殊处理。

### 错误码

无。

### 边界 case

- **前置约束（重要）：reference 字段禁止嵌套** —— 由字段管理 `validateReferenceRefs` + `detectCyclicRef` 强制保证。这意味着：
  - reference 的子字段一定是 leaf 字段（`integer / float / string / boolean / select`），**不可能是另一个 reference**
  - 弹层永远只有一层 popover，不存在"点开子字段又弹出新层"的情况
  - 模板侧不需要做递归展开，所有逻辑都是单层的
- **关键设计：**
  - reference 字段在模板里**不存在**——它只是 UI 上的"快捷选择器"
  - 模板存的是展开后的**实际字段 ID 列表**，扁平结构
  - 同一字段被多个 reference 引用、或既被直接勾选又被某个 reference 包含时，**自动去重**
  - 后端不知道哪些字段是从哪个 reference 来的
  - reference 字段后续修改其引用列表，**不影响已创建的模板**
  - 在编辑页，弹层永远拉最新的 reference 子字段列表，已被模板包含的子字段自动回勾
- **浮层 UI 要点：**
  - 浮层标题：reference 字段的中文标签 + 标识 + reference 紫色徽章
  - 蓝色信息条提示"勾选的子字段会扁平地写入模板，与其他来源去重"
  - **工具栏**：左侧"子字段 (N)"计数，右侧全选 / 全不选两个快捷按钮
  - 子字段列表：每行带 checkbox + 字段标签 + 字段标识 + 类型徽章
  - 已勾选的子字段保持勾选状态（即使浮层关闭重开）
  - 勾选/取消勾选的同时，外部"已选字段配置区"实时同步增删行
  - 底部：左侧"已选 X / N"计数 + 右侧取消/确定按钮

---

## 功能 9：启用状态前置校验弹窗（前端交互）

### 场景描述

管理员在列表上点击一个启用中模板的「编辑」或「删除」按钮。

后端会拒绝并返回错误码：
- 编辑：`41010 ErrTemplateEditNotDisabled`
- 删除：`41009 ErrTemplateDeleteNotDisabled`

但更好的做法是**前端在请求发出之前就拦截**——列表数据里已经有 `enabled` 字段，前端可以直接根据 `enabled === true` 判断并弹出引导式弹窗，避免一次无效的网络往返。后端的错误码作为兜底防御。

### 校验规则

前端交互功能，无后端校验。后端错误码 `41010` / `41009` 作为兜底。

### 调用链

前端拦截，无独立后端接口。

### 错误码

| 错误码 | 常量 | 触发场景 |
|--------|------|---------|
| 41009 | `ErrTemplateDeleteNotDisabled` | 删除启用中的模板（兜底） |
| 41010 | `ErrTemplateEditNotDisabled` | 编辑启用中的模板（兜底） |

### 边界 case

- **弹窗设计（编辑场景）：**
  - 标题：`无法编辑模板` + 橙色警告图标
  - 正文：解释"启用中模板对 NPC 管理页可见，允许任意修改可能导致策划在配置不稳定时选用"
  - 操作步骤区：1. 点击开关停用 2. 完成编辑后再次启用
  - 底部按钮：`知道了`（次要） + `立即停用`（橙色主按钮，点击直接调 `toggle-enabled` 并跳进编辑页）
- **弹窗设计（删除场景）：**
  - 标题：`无法删除模板` + 橙色警告图标
  - 正文：解释"删除是不可恢复的操作，先停用可以提供一个观察期"
  - 删除前置条件区：两条（模板已停用 / 无 NPC 引用）
  - 底部按钮：`知道了` + `立即停用`
- 两个弹窗复用同一个 `<EnabledGuardDialog>` 组件，通过 `action='edit' | 'delete'` 切换。

---

## 功能 10：模板名唯一性校验

### 场景描述

在模板管理页新建模板时，管理员输入模板标识后离开输入框，前端实时告知这个名字能不能用。

即使某个模板已经被软删除，它的标识也不能被新模板复用——历史 NPC 可能持有这个标识的快照，复用会导致难以排查的语义混乱。

### 校验规则

- **Handler**：`name` 格式（`^[a-z][a-z0-9_]*$` + 长度上限）。
- **Service**：`TemplateStore.ExistsByName`（含软删除记录）查 MySQL。

### 调用链

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/templates/check-name` |
| Handler | `TemplateHandler.CheckName` — 校验 `name` 格式 |
| Service | `TemplateService.CheckName` — `TemplateStore.ExistsByName(name)` 含软删除 → `{available, message}` |

### 错误码

| 错误码 | 常量 | 触发场景 |
|--------|------|---------|
| 41002 | `ErrTemplateNameInvalid` | `name` 格式不合法 |
| 40000 | `ErrBadRequest` | `name` 为空 |

### 边界 case

- 含软删除记录也视为已占用。

---

## 功能 11：跨模块对外接口（给字段管理调用）

### 场景描述

为了让字段管理的 `FieldHandler.GetReferences` 能在跨模块编排时补上模板 label，`TemplateService` 暴露了以下只读方法。

### 校验规则

各方法内嵌校验。

| 方法 | 用途 |
|------|------|
| `GetByIDsLite(ctx, ids)` | 批量查模板精简信息 `[]TemplateLite{ID, Name, Label}`，底层 `TemplateStore.GetByIDs` |
| `ExistsByName(ctx, name)` | 模板管理内部 handler 预查复用，也可供其他跨模块路径使用 |

### 调用链

由字段管理的 `FieldHandler.GetReferences` 编排调用，不独立暴露 HTTP 接口。

### 错误码

无专属错误码。

### 边界 case

- `TemplateService` 不持有 `FieldStore` / `FieldCache`，也不调 `FieldService`；反向的 `FieldService` 也不持有 `TemplateStore` / `TemplateCache`。两个 Service 互相"不认识"，所有跨模块编排都在 Handler 层显式串起来。

---

## 横切关注点

| 关注点 | 实现方式 |
|--------|---------|
| 操作标识 | 主键 ID (BIGINT)，`name` 仅用于创建和唯一性校验 |
| 统一响应格式 | `handler.WrapCtx` 泛型包装，返回 `{Code, Data, Message}` |
| 错误码体系 | 12 个模板段错误码（41001-41012）|
| 缓存穿透防护 | `TemplateCache.SetDetail` 对 `nil` tpl 也写空标记 |
| 缓存击穿防护 | `TemplateService.GetByID` 使用分布式锁 `TryLock(id, 3s)` + double-check |
| 缓存雪崩防护 | TTL 加随机 jitter |
| 缓存批量失效 | 列表缓存版本号，`InvalidateList` 即失效 |
| 缓存类型安全 | 列表缓存使用 `TemplateListData`（`[]TemplateListItem`）|
| 缓存降级 | Redis 不可用时直接穿透到 MySQL |
| 缓存 Key | `templates:detail:{id}`、`templates:lock:{id}` |
| 缓存边界 | **只缓存 `*Template` 裸行**，不缓存拼装后的 `TemplateDetail`（避免被字段方状态污染）|
| 乐观锁 | `UPDATE ... WHERE id=? AND version=?`，rows=0 → `ErrVersionConflict` → 41011 |
| 软删除 | `deleted=1`，所有查询过滤 `WHERE deleted=0` |
| 引用计数 | `ref_count` 冗余字段（被 NPC 引用数），事务内原子维护 |
| TOCTOU 防护 | 删除在事务内 `GetRefCountForDeleteTx` 用 `FOR SHARE` 重新检查 |
| 覆盖索引 | `idx_list (deleted, enabled, id DESC)` 列表查询不回表 |
| 输入校验分层 | Handler 做格式校验（`identPattern` / `label` 长度 / `description ≤ 512` / `fields` 非空不重复 / `id > 0` / `version > 0`），Service 做业务校验（存在性 / 启用状态 / `ref_count` / 集合 diff）|
| 编辑限制 | 只有未启用状态才能编辑（41010）；有 NPC 引用时字段列表锁死（41008）|
| 删除限制 | 只有未启用状态才能删除（41009）；有 NPC 引用时拒绝删除（41007）|
| 跨模块边界 | Service 只持有自身 store/cache；跨模块拼装/事务/缓存清理全部在 Handler 层编排；Service 之间零依赖 |
| 常量管理 | Redis key、TTL、`RefTypeTemplate` 统一为常量 |

---

## 错误码（模板段 41001-41012）

| 错误码 | 常量 | 含义 |
|--------|------|------|
| 41001 | `ErrTemplateNameExists` | 模板标识已存在（含软删除）|
| 41002 | `ErrTemplateNameInvalid` | 模板标识格式不合法 |
| 41003 | `ErrTemplateNotFound` | 模板不存在 |
| 41004 | `ErrTemplateNoFields` | 未勾选任何字段 |
| 41005 | `ErrTemplateFieldDisabled` | 勾选了停用字段（由 `FieldService.ValidateFieldsForTemplate` 抛出）|
| 41006 | `ErrTemplateFieldNotFound` | 勾选的字段不存在（由 `FieldService.ValidateFieldsForTemplate` 抛出）|
| 41007 | `ErrTemplateRefDelete` | 被 NPC 引用，无法删除 |
| 41008 | `ErrTemplateRefEditFields` | 被 NPC 引用，无法编辑字段列表（含顺序/必填）|
| 41009 | `ErrTemplateDeleteNotDisabled` | 删除前必须先停用 |
| 41010 | `ErrTemplateEditNotDisabled` | 编辑前必须先停用 |
| 41011 | `ErrTemplateVersionConflict` | 乐观锁版本冲突 |
| **41012** | **`ErrTemplateFieldIsReference`** | **勾选了 reference 类型字段（必须展开为 leaf 子字段后再加入）** |

---

## 与字段管理的集成回顾

1. **`field_refs` 维护**：创建/编辑/删除模板时，**在同一事务内**由 `FieldService.AttachToTemplateTx` / `DetachFromTemplateTx` 同步 `field_refs` 表和字段的 `ref_count`。事务由 `TemplateHandler` 打开，两个 Service 的 `*Tx` 方法都接收外部 `*sqlx.Tx`，commit 后由 handler 分别清两方缓存。

2. **补全字段引用详情的模板 label**：字段管理 `FieldHandler.GetReferences` 跨模块调 `TemplateService.GetByIDsLite` 补齐模板 label（字段管理 features 功能 7）。

3. **停用字段标注**：模板详情返回的 `TemplateFieldItem.Enabled` 来自 `FieldLite.Enabled`，反映字段**当前**状态；如果被停用前端会标灰 + 警告图标，但模板的引用关系仍然保留（"存量不动、增量拦截"）。

4. **新增引用必须是启用字段**：编辑模板新增字段勾选时，`FieldService.ValidateFieldsForTemplate` 拒绝停用字段，返回 `41005`；字段不存在返回 `41006`。

5. **字段分类标签复用**：模板详情中每个字段的 `CategoryLabel` 由 `FieldService.GetByIDsLite` 用 `DictCache` 翻译后返回，前端直接用，不硬编码分类文案。

6. **错误码段位约定**：`41005` / `41006` / `41012` 虽然是"字段状态/类型"错误，但由于由模板管理页消费，故归在 41xxx 段位，与字段段的 `40011 ErrFieldNotFound` / `40013 ErrFieldRefDisabled` / `40016 ErrFieldRefNested` 语义不混用。

7. **模板不能直接挂载 reference 类型字段**：`FieldService.ValidateFieldsForTemplate` 在 Create 和 Update 的事务前预校验阶段同时拦截 `f.Type == FieldTypeReference` 的情况，返回 `41012 ErrTemplateFieldIsReference`。模板 `fields` JSON 只允许 leaf 字段（`integer / float / string / boolean / select`），这和字段管理功能 11 "reference 禁止嵌套" 一起构成了"模板只看到扁平 leaf 集合"的全局约束——前端的 reference popover 只是"快捷选择器"，点击 reference 字段弹出子字段清单后，真正写入 `req.Fields` 的只有勾选的 leaf 子字段 ID，reference 字段本身永远不出现在 `req.Fields` 里。如果前端 bug 或直连 API 的工具试图把 reference 写进去，后端在事务前就会拒绝，`field_refs` 不会被污染，`ref_count` 保持为 0。

---

## 已知限制

| 限制 | 说明 | 计划 |
|------|------|------|
| 引用详情待对接 | NPC 模块未上线前，`references` 接口返回空 `npcs` 数组占位 | NPC 管理上线时在 handler 层加跨模块调用 |
| 默认值覆盖未支持 | 模板暂不支持覆盖字段的默认值（比如 `hp` 字段默认 100，战斗模板里改成 200）| 毕设后按需扩展 |
| reference 子字段差异提示 | reference 字段后续新增子字段时，编辑页**不**主动标记"+N 新字段" | 实现 diff 成本不抵价值；用户点开弹层即可看到最新列表 |
