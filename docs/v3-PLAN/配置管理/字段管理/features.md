# 字段管理 — 已实现功能清单

> **实现状态**：后端 API + 前端 UI 全部实现。
> 字段是 ADMIN 内部的管理概念，定义"NPC 可以有什么属性"。全程只和 MySQL 打交道，不涉及 MongoDB。
> 字段值最终通过 模板→NPC 打平写入 npc_templates 导出。
> **所有操作标识使用主键 ID (BIGINT)，name 仅用于创建时写入和唯一性校验。**
> **技术栈**：后端 Go，前端 Vue 3 + TypeScript + Element Plus + Vite。

---

## 字段的三种状态

| 状态 | 字段管理页看到 | 其他模块看到 | 能被新引用 | 已有引用 |
|------|-------------|------------|----------|---------|
| 启用 | 可见，正常操作 | 可见可选 | 允许 | 正常 |
| 停用 | 可见，灰色标记 | 不可见 | 拒绝 | 保持不动 |
| 已删除 | 不可见 | 不可见 | 不可能 | 已清理 |

核心原则：**停用是"存量不动，增量拦截"；删除才真正清理引用关系。**

---

## 功能 1：字段列表

**场景 A — 在字段管理页，管理员要浏览所有字段。** 不传 enabled 筛选条件，启用和停用的字段都展示出来，管理员才能对停用字段做重新启用或删除操作。

**场景 B — 在模板管理页（或未来的行为树配置页），策划要从下拉框选一个字段加到模板里。** 传 `enabled=true`，只展示启用的字段。停用的字段不应该出现在选择列表中，避免策划选了一个不可用的字段。

两个场景走同一个接口，靠 `enabled` 参数区分。支持按中文标签模糊搜索、字段类型/标签分类精确筛选、后端分页。列表项包含 `id` 字段，前端用 id 发起后续操作。

**调用链路：**

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fields/list` |
| Handler | `FieldHandler.List` — 分页参数默认值/上限校正 |
| Service | `FieldService.List` — 查缓存 → 查 MySQL → 内存翻译 type_label/category_label → 写缓存 |
| Store | `FieldCache.GetList` → `FieldStore.List`（覆盖索引） → `FieldCache.SetList` |

---

## 功能 2：新建字段

**场景 — 在字段管理页，管理员要定义一个新的 NPC 属性（比如"生命值"、"阵营"）。** 填写字段标识、中文标签、类型、分类和动态属性后提交。

新建的字段默认是**未启用**状态（enabled=false）。这是一个刻意的设计：管理员创建字段后，往往还需要反复调整约束、默认值等配置。如果创建即启用，模板管理页的下拉列表会立刻出现这个半成品字段，策划可能在管理员还没配好之前就选了它。默认未启用就提供了一个"配置窗口期"——管理员可以反复编辑、确认无误后再手动启用，启用后其他模块才能看到并使用它。

字段标识（name）一旦创建不可修改（是唯一键），且含软删除记录也不能重复使用，防止历史数据混乱。

**调用链路：**

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fields/create` |
| Handler | `FieldHandler.Create` — 校验 name 格式/长度、label、type、category、properties 非空 |
| Service | `FieldService.Create` — 校验字典存在性 → name 唯一性 → reference 类型校验（存在性+启用+**禁止嵌套**+循环引用兜底） → 写入 MySQL → 写入 field_refs + IncrRefCount → 清列表缓存 |
| Store | `FieldStore.Create` 返回 lastInsertId |

---

## 功能 3：字段详情

**场景 A — 在字段管理页，管理员点击某个字段查看或准备编辑。** 需要拿到完整的字段信息，包括动态属性 properties。

**场景 B — 在模板管理页，策划选中一个字段后，前端要展示这个字段的约束信息（比如取值范围），用于渲染动态表单。** 同样调用详情接口拿到完整数据。

无论字段是启用还是停用，详情都能查。因为已经加到模板里的停用字段，策划仍然需要看到它的配置内容。

**调用链路：**

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fields/detail` |
| Handler | `FieldHandler.Get` — 校验 `id > 0` |
| Service | `FieldService.GetByID` — 查 Redis detail 缓存 → 分布式锁防击穿 → double-check → 查 MySQL → 写缓存（含空标记防穿透） |
| Store | `FieldCache.GetDetail(id)` → `TryLock(id)` → `FieldStore.GetByID(id)` → `FieldCache.SetDetail(id)` |

---

## 功能 4：编辑字段

**场景 — 在字段管理页，管理员要修改字段的标签、类型、分类或约束。** 只有未启用状态才能编辑——启用中的字段已对外可见，允许随意编辑会导致引用方看到不稳定的配置。

如果这个字段已经被模板或其他字段引用了（ref_count > 0），有两个硬约束：
- 不能改类型。比如从"整数"改成"字符串"，已经引用它的模板里填的值全乱了。
- 不能收紧约束。比如最大值从 100 改成 50，模板里已经填了 80 的值就不合法了。

编辑用乐观锁防止两个管理员同时改同一个字段互相覆盖。

**调用链路：**

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fields/update` |
| Handler | `FieldHandler.Update` — 校验 `id > 0`、label、type、category、properties、version |
| Service | `FieldService.Update` — 按 ID 查旧数据 → **校验 enabled=0** → 字典校验 → 类型变更/约束收紧检查 → reference 引用校验 → 乐观锁写入 → 同步引用关系 → 清缓存 |
| Store | `FieldStore.GetByID(id)` → `FieldStore.Update(req)` WHERE id=? AND version=? |

---

## 功能 5：删除字段

**场景 — 在字段管理页，管理员要彻底移除一个不再需要的字段。**

删除有两道门槛：
1. 必须先停用。这是给管理员一个缓冲期——停用后观察一段时间，确认没有问题再删。
2. 不能有引用。如果还有模板或其他字段在引用它，删不掉，接口会返回错误。

删除是软删除（标记 deleted=1），不是物理删除。如果这个字段本身是 reference 类型（引用了其他字段），删除时会自动清理它对其他字段的引用关系。

**调用链路：**

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fields/delete` |
| Handler | `FieldHandler.Delete` — 校验 `id > 0` |
| Service | `FieldService.Delete` — 按 ID 查 → 校验 enabled=0 → 事务内 FOR SHARE 检查引用 → 软删除 → 清理 reference 引用 + DecrRefCount → 清缓存 |
| Store | `FieldStore.GetByID` → `FieldRefStore.HasRefsTx(tx, id)` FOR SHARE → `FieldStore.SoftDeleteTx(tx, id)` |

---

## 功能 6：字段名唯一性校验

**场景 — 在字段管理页新建字段时，管理员输入字段标识后离开输入框，前端实时告知这个名字能不能用。**

即使某个字段已经被删除了，它的标识也不能被新字段复用。这是因为字段标识会写入模板配置、导出给游戏服务端，历史数据中可能残留旧标识的引用。如果允许复用，新字段和旧数据的含义完全不同，会导致难以排查的数据错乱。

**调用链路：**

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fields/check-name` |
| Handler | `FieldHandler.CheckName` — 校验 name 非空 |
| Service | `FieldService.CheckName` — `FieldStore.ExistsByName(name)` 含软删除记录 |

---

## 功能 7：字段引用详情

**场景 A — 在字段管理页，管理员想停用或删除某个字段之前，先看看谁在用它。** 接口返回两类引用方：哪些模板引用了它、哪些 reference 类型字段引用了它，附带引用方的中文名。

**场景 B — 在字段管理页，删除接口返回"被引用无法删除"后，前端自动调用此接口展示引用详情，告诉管理员应该先去哪里解除引用。**

**调用链路：**

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fields/references` |
| Handler | `FieldHandler.GetReferences` — 校验 `id > 0` |
| Service | `FieldService.GetReferences` — 按 ID 查字段 → `FieldRefStore.GetByFieldID(id)` → 按 ref_type 分组 → `FieldStore.GetByIDs(ids)` 批量取 label |

---

## 功能 8：启用/停用切换

**场景 A — 在字段管理页，管理员新建完字段、确认配置无误后，启用它。** 启用后其他模块的字段下拉列表才能看到这个字段。

**场景 B — 在字段管理页，管理员要下线一个字段，先停用它。** 停用后：
- 其他模块的下拉列表立刻看不到它了，策划不会再选它
- 但已经引用它的模板不受影响，模板里已有的配置继续生效
- 如果确认不再需要，后续再执行删除

停用一个被引用的字段是允许的。这是"存量不动，增量拦截"的设计：已经在用的不打扰，新的不让用。

**调用链路：**

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fields/toggle-enabled` |
| Handler | `FieldHandler.ToggleEnabled` — 校验 `id > 0`、version > 0 |
| Service | `FieldService.ToggleEnabled` — 按 ID 查字段 → 乐观锁更新 → 清缓存 |
| Store | `FieldStore.ToggleEnabled(id, enabled, version)` WHERE id=? AND version=? |

---

## 功能 9：字典选项查询

**场景 — 在字段管理页新建或编辑字段时，"字段类型"和"标签分类"的下拉选项不是前端写死的，而是从后端动态获取。** 这样运营团队可以随时在字典表里加新类型、新分类，不需要改代码重新部署。

字典数据启动时从 MySQL 全量加载到内存，运行时直接读内存，不查表。

**调用链路：**

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/dictionaries` |
| Handler | `DictionaryHandler.List` |
| Service | — （直接读内存缓存） |
| Store | `DictCache.ListByGroup` |

---

## 功能 10：约束收紧检查

**场景 — 在字段管理页编辑字段时，如果这个字段已经被模板引用了，管理员想修改它的约束。** 比如把生命值的最大值从 100 改成 50。

问题是：模板里可能已经有人填了 80。如果允许改成 50，那 80 就超出范围了，数据就不一致了。

所以被引用时，约束只能放宽不能收紧：
- 数值类型：最小值只能往小改，最大值只能往大改
- 字符串：最小长度只能往小改，最大长度只能往大改
- 下拉选项：只能加新选项，不能删旧选项

没有被引用的字段不受此限制，随便改。

此功能内嵌在功能 4（编辑字段）的 Service 层中。

### 约束 key 命名契约（必须前后端严格对齐）

`properties.constraints` 是无 schema 的 JSON RawMessage，DB 层不校验结构，命名靠前后端代码约定。**单一权威**为 seed 文件 `backend/cmd/seed/main.go` 中 `field_type` 字典每条记录的 `constraint_schema`。后端 `service.checkConstraintTightened` 和游戏服务端导出都直接读这些 key。前端 `frontend/src/components/FieldConstraint*.vue` 必须严格使用以下名称，**不得改成驼峰/下划线变体**——否则收紧检查（40007）静默失效。

| 字段类型 | constraint key | 说明 |
|---------|---------------|------|
| integer | `min` / `max` / `step` | 最小值/最大值/步长 |
| float | `min` / `max` / `precision` | 最小值/最大值/小数位数 |
| string | `minLength` / `maxLength` / `pattern` | 最小长度/最大长度/正则 |
| boolean | — | 无约束 |
| select | `options` / `minSelect` / `maxSelect` | 选项数组（每项 `{value, label}`）/最少选/最多选 |
| reference | `refs` | 被引用字段 ID 数组（前端 UI 用 `ref_fields` 富对象，提交前转 `refs`） |

---

## 功能 11：reference 字段引用约束与关系维护

**场景 A — 在字段管理页创建 reference 类型字段时，要从下拉列表选择它引用哪些 leaf 字段。** 下拉列表只展示**启用的非 reference 字段**（前端在功能 1 的 `enabled=true` 列表上再过滤掉 `type='reference'`）。选好后，后端做三层校验：被引用字段存在 + 启用 + **不是 reference 类型**，然后记录引用关系并更新被引用字段的引用计数。

**场景 B — 在字段管理页编辑一个已有的 reference 类型字段，想加几个新引用或去掉几个旧引用。** 对于新增的引用，被引用字段必须是启用的且**不是 reference 类型**，停用或 reference 都不让加。但对于已有的引用，即使那个字段后来被停用了也允许保持——这是"存量不动，增量拦截"。

**场景 C — 在字段管理页删除一个 reference 类型字段时，它之前引用的那些字段的引用计数要减回去。** 这在删除事务内自动完成。

### 嵌套禁止规则（重要）

**reference 字段只能引用 leaf 字段（integer / float / string / boolean / select），不能引用其他 reference 字段。** 这是一个硬约束，不允许任何例外。

**为什么禁止嵌套：**
- **语义清晰**：reference 是"快捷选择器"，不是抽象层。需要"分类的分类"时直接建一个更大的 reference 列出所有 leaf 字段，扁平直观。
- **天然防环**：reference 链最多只有一层 → 不可能形成环 → 模板侧的弹层永远只有一层 popover。
- **降低后端复杂度**：模板创建 / 字段引用展开 / 引用计数维护都不再需要递归。
- **降低 UX 复杂度**：管理员看到的引用列表永远是 leaf 字段，不会点开一层又一层。

**校验位置（前后端双保险）：**
- 前端：`FieldConstraintReference.vue` 的 `availableFields` computed 在 `enabled=true` 字段列表上过滤掉 `type === 'reference'`
- 后端：`FieldService.Create` / `Update` 的 reference 分支对每个新增 ref 校验 `f.Type != FieldTypeReference`，违反返回 `40016 ErrFieldRefNested`

**循环引用检测的现状：** 嵌套禁止后，理论上不可能出现环。`detectCyclicRef` DFS 函数保留作为防御性兜底（对抗未来通过非常规途径写入的脏数据），运行成本可忽略。

此功能内嵌在功能 2（创建）、功能 4（编辑）、功能 5（删除）的 Service 层中。

---

## 已移除功能

| 功能 | 原因 |
|------|------|
| 批量删除 | UI 不暴露批量操作，需要时由后端人员直接调用 |
| 批量修改分类 | 同上 |

---

## 横切关注点

| 关注点 | 实现方式 |
|--------|---------|
| 操作标识 | 主键 ID (BIGINT)，name 仅用于创建和唯一性校验 |
| 统一响应格式 | `handler.WrapCtx` 泛型包装，返回 `{Code, Data, Message}` |
| 错误码体系 | 16 个错误码（40001-40016），语义分离 |
| 缓存穿透防护 | 空值标记 `{"_null":true}`，未命中时也缓存 nil |
| 缓存击穿防护 | `GetByID` 使用分布式锁 `TryLock(id)` + double-check |
| 缓存雪崩防护 | TTL 加随机 jitter |
| 缓存批量失效 | 列表缓存版本号，INCR 即失效 |
| 缓存类型安全 | 列表缓存使用 `FieldListData`（`[]FieldListItem`） |
| 缓存降级 | Redis 不可用时直接穿透到 MySQL |
| 缓存 Key | `fields:detail:{id}`、`fields:lock:{id}`（改用 ID） |
| 乐观锁 | `UPDATE ... WHERE version = ?`，rows=0 返回版本冲突 |
| 软删除 | `deleted=1`，所有查询过滤 `WHERE deleted=0` |
| 引用计数 | `ref_count` 冗余字段，事务内原子维护 |
| TOCTOU 防护 | 删除在事务内 `FOR SHARE` 重新检查引用 |
| 覆盖索引 | `idx_list` 列表查询不回表 |
| 输入校验分层 | Handler 做格式校验（ID>0/name 正则/label 长度），Service 做业务校验（存在性/启用状态/引用） |
| 编辑限制 | 只有未启用状态才能编辑（40015 ErrFieldEditNotDisabled） |
| 常量管理 | 字段类型、Redis key、TTL、引用类型 统一为常量，不硬编码 |

---

## 已知限制

| 限制 | 说明 | 计划 |
|------|------|------|
| Create + syncFieldRefs 非原子 | reference 字段创建与引用关系同步在不同事务中，极端情况可能不一致 | 模板管理上线时统一重构为事务内操作 |
| 通用 ListData.Items 为 any | HTTP 响应层仍用 `ListData{Items: any}`，仅缓存层做了类型安全 | 未来可泛型化 ListData |
| BB Key 校验未对接 | 错误码 40008 已定义，但 expose_bb 变更检查待 BT 模块提供接口 | BT 模块开发时对接 |
| 模板 label 占位 | 引用详情中模板引用的 label 用占位值 | 模板管理完成后补全 |
