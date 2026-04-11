# 字段管理 — 已实现功能清单

> **实现状态**：**后端 + 前端全部落地**（集成测试 199/199 通过 + 前端 FieldList/FieldForm/5 个约束面板/EnabledGuardDialog 集成完整）。
> 本文档是「用户场景 → 校验 → 调用链」的按功能展开说明；架构层的文件组织 / 缓存策略 / 跨模块对外接口见 `backend.md`，前端状态流 / 组件树 / 错误码处理见 `frontend.md`。
> 字段是 ADMIN 内部的管理概念，定义"NPC 可以有什么属性"。全程只和 MySQL 打交道，不涉及 MongoDB。字段值最终通过"模板 → NPC"打平写入 `npc_templates` 集合导出。
> **所有操作标识使用主键 ID (BIGINT)，`name` 仅用于创建时写入和唯一性校验。**
> **技术栈**：后端 Go（gin + sqlx + slog），前端 Vue 3.5 + TypeScript strict + Element Plus + Vite。

---

## 字段的三种状态

| 状态 | 字段管理页看到 | 其他模块看到 | 能被新引用 | 已有引用 |
|------|-------------|------------|----------|---------|
| 启用 | 可见，正常操作 | 可见可选 | 允许 | 正常 |
| 停用 | 可见，灰色标记 | 不可见 | 拒绝 | 保持不动 |
| 已删除 | 不可见 | 不可见 | 不可能 | 已清理 |

核心原则：**停用是"存量不动，增量拦截"；删除才真正清理引用关系。**

---

## 模块职责边界（分层硬规则）

字段管理严格遵守"分层职责"硬规则。`FieldService` **只持有字段模块自己的 store/cache**（`FieldStore` / `FieldRefStore` / `FieldCache` / `DictCache`），**不持有** `TemplateStore` / `TemplateCache` / `TemplateService`。

跨模块的拼装（例如"字段引用详情里补模板 label"）由 `FieldHandler` 作为"用例编排者"调 `TemplateService.GetByIDsLite` 完成。Service 层之间零依赖，所有跨模块串联都发生在 Handler 层。

对外，`FieldService` 暴露一组跨模块方法供模板管理的 handler 调用：

```
ValidateFieldsForTemplate / AttachToTemplateTx / DetachFromTemplateTx /
GetByIDsLite / InvalidateDetails
```

参见功能 12。

---

## 功能 1：字段列表

**场景 A — 在字段管理页，管理员要浏览所有字段。** 不传 `enabled` 筛选条件，启用和停用的字段都展示出来，管理员才能对停用字段做重新启用或删除操作。

**场景 B — 在模板管理页（或未来的行为树配置页），策划要从下拉框选一个字段加到模板里。** 传 `enabled=true`，只展示启用的字段。停用的字段不应该出现在选择列表中，避免策划选了一个不可用的字段。

两个场景走同一个接口，靠 `FieldListQuery.Enabled (*bool)` 的三态区分：`nil` 不筛选、`true` 仅启用、`false` 仅停用。支持按中文标签模糊搜索（`Label`）、按字段类型（`Type`）/标签分类（`Category`）精确筛选、后端分页（`Page` / `PageSize`，Service 层按 `pagCfg` 校正上下界）。列表项包含 `id`，前端用 id 发起后续操作。

Service 在返回列表前用 `DictCache.GetLabel` 给每行补 `type_label` / `category_label`，前端直接渲染，不再回查字典。

**调用链路：**

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fields/list` |
| Handler | `FieldHandler.List` — 直接透传 query |
| Service | `FieldService.List` — 分页参数校正 → 查 Redis 列表缓存 → miss 时查 MySQL → 内存翻译 type_label/category_label → 写缓存 |
| Store | `FieldCache.GetList` → `FieldStore.List`（覆盖索引）→ `FieldCache.SetList` |

---

## 功能 2：新建字段

**场景 — 在字段管理页，管理员要定义一个新的 NPC 属性（比如"生命值"、"阵营"）。** 填写字段标识、中文标签、类型、分类和动态属性后提交。

新建的字段默认是**未启用**状态（`enabled=false`）。这是一个刻意的设计：管理员创建字段后，往往还需要反复调整约束、默认值等配置。如果创建即启用，模板管理页的下拉列表会立刻出现这个半成品字段，策划可能在管理员还没配好之前就选了它。默认未启用就提供了一个"配置窗口期"——管理员可以反复编辑、确认无误后再手动启用，启用后其他模块才能看到并使用它。

字段标识（`name`）一旦创建不可修改（是唯一键），且含软删除记录也不能重复使用，防止历史数据混乱。

**校验分层**：
- **Handler**（`FieldHandler.Create`）做格式/必填校验：`name` 符合 `identPattern = ^[a-z][a-z0-9_]*$`、长度 ≤ `valCfg.FieldNameMaxLength`；`label` 非空且长度 ≤ `valCfg.FieldLabelMaxLength`；`type` / `category` 非空；`properties` 必须是 JSON 对象形状（首字符 `{`，拦 `null` / 数组 / 标量）。
- **Service**（`FieldService.Create`）做业务校验：字典存在性（`checkTypeExists` / `checkCategoryExists`）→ `name` 唯一性（含软删除）→ 对 `reference` 类型调 `validateReferenceRefs`（非空 / 存在 / 启用 / 非嵌套 / 无循环）→ 写入 MySQL → 对 reference 字段调 `syncFieldRefs` 写 `field_refs` 表 + 逐个 `IncrRefCountTx` → 清列表缓存。

**调用链路：**

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fields/create` |
| Handler | `FieldHandler.Create` — 格式校验 |
| Service | `FieldService.Create` — 字典 → 唯一性 → reference 校验 → `FieldStore.Create` → `syncFieldRefs`（单独事务）→ 清缓存 |
| Store | `FieldStore.Create` 返回 `lastInsertId` |

---

## 功能 3：字段详情

**场景 A — 在字段管理页，管理员点击某个字段查看或准备编辑。** 需要拿到完整字段信息，包括动态属性 `properties`。

**场景 B — 在模板管理页，策划选中一个字段后，前端要展示这个字段的约束信息（比如取值范围），用于渲染 NPC 表单。** 同样调详情接口拿完整数据。

无论字段是启用还是停用，详情都能查——已经加到模板里的停用字段，策划仍然需要看到它的配置内容。

Service 层使用 Cache-Aside + 分布式锁 + 空标记三件套：
1. 先查 Redis detail，命中即返回（命中到空标记时直接返回 `ErrFieldNotFound`）。
2. miss 时 `FieldCache.TryLock(id, 3s)` 防击穿；获得锁后再 double-check 一次缓存。
3. 锁失败不阻塞，降级直查 MySQL。
4. 查到（或查不到）都写 Redis；`field=nil` 时写空标记防穿透。

**调用链路：**

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fields/detail` |
| Handler | `FieldHandler.Get` — 校验 `id > 0` |
| Service | `FieldService.GetByID` — Cache-Aside + `TryLock` + double-check + 空标记 |
| Store | `FieldCache.GetDetail` → `TryLock` → `FieldStore.GetByID` → `FieldCache.SetDetail` |

---

## 功能 4：编辑字段

**场景 — 在字段管理页，管理员要修改字段的标签、类型、分类或约束。** 只有**未启用**状态才能编辑——启用中的字段已对外可见，允许随意编辑会导致引用方看到不稳定的配置。试图编辑启用中的字段返回 `40015 ErrFieldEditNotDisabled`。

字段一旦被模板或其他 reference 字段引用（`ref_count > 0`），有两个硬约束：
- **不能改类型**（`40006 ErrFieldRefChangeType`）。比如从"整数"改成"字符串"，已引用它的模板里填的值全乱了。
- **不能收紧约束**（`40007 ErrFieldRefTighten`）。比如最大值从 100 改成 50，模板里已经填了 80 的值就不合法了。详见功能 10。

对于 `reference` 类型字段，编辑时对**新增**的引用校验"存在 + 启用 + 非嵌套 + 无循环"；对**已有**的引用（在 `oldRefSet` 集合中）即使目标后来被停用或变成 reference 类型也允许保留——这就是"存量不动，增量拦截"的体现。

如果类型**从 reference 改成其他类型**，Service 会在写入后主动把它对其他字段的旧引用关系整体清掉（调 `syncFieldRefs(id, oldRefIDs, nil)`）。

写入使用乐观锁 `UPDATE ... WHERE id=? AND version=?`，rows=0 返回 `storemysql.ErrVersionConflict`，Service 层转 `40010 ErrFieldVersionConflict`。

**调用链路：**

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fields/update` |
| Handler | `FieldHandler.Update` — 校验 `id > 0` / `label` / `type` / `category` / `properties` 形状 / `version > 0` |
| Service | `FieldService.Update` — 字典校验 → `getFieldOrNotFound` → **enabled 必须为 false（41015）** → type 变更拦截（40006）→ `checkConstraintTightened`（40007）→ reference 校验（40013/40014/40016/40009）→ 乐观锁写入 → `syncFieldRefs` 同步引用关系（含 reference→非 reference 时的清空）→ 清自身/受影响方/列表缓存 |
| Store | `FieldStore.GetByID` → `FieldStore.Update`（WHERE id=? AND version=?）|

---

## 功能 5：删除字段

**场景 — 在字段管理页，管理员要彻底移除一个不再需要的字段。**

删除有两道门槛：
1. **必须先停用**（`40012 ErrFieldDeleteNotDisabled`）。这是给管理员一个缓冲期——停用后观察一段时间，确认没有问题再删。
2. **不能有引用**（`40005 ErrFieldRefDelete`）。如果还有模板或其他字段在引用它，删不掉。

删除是软删除（`deleted=1`），不是物理删除。事务内用 `FieldRefStore.HasRefsTx`（`FOR SHARE` 共享锁）重新检查引用关系以防 TOCTOU——在"前面查字段"到"现在删除"之间可能有新的引用挤进来。

如果字段**自身就是 reference 类型**，删除事务内还会调 `FieldRefStore.RemoveBySource(tx, 'field', id)` 清掉它对其他字段的引用关系，并对每个被引用方 `DecrRefCountTx`，一次事务内完成。

**调用链路：**

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fields/delete` |
| Handler | `FieldHandler.Delete` — 校验 `id > 0` |
| Service | `FieldService.Delete` — `getFieldOrNotFound` → `enabled=false` 校验 → 开 tx → `HasRefsTx`（FOR SHARE）→ `SoftDeleteTx` → 若自身是 reference 则 `RemoveBySource` + `DecrRefCountTx` → Commit → 清自身/受影响方/列表缓存 |
| Store | `FieldStore.GetByID` → `FieldRefStore.HasRefsTx` → `FieldStore.SoftDeleteTx` → `FieldRefStore.RemoveBySource` → `FieldStore.DecrRefCountTx` |

---

## 功能 6：字段名唯一性校验

**场景 — 在字段管理页新建字段时，管理员输入字段标识后离开输入框，前端实时告知这个名字能不能用。**

即使某个字段已经被软删除，它的标识也不能被新字段复用。因为字段标识会写入模板配置、导出给游戏服务端，历史数据中可能残留旧标识的引用。如果允许复用，新字段和旧数据的含义完全不同，会导致难以排查的数据错乱。

`FieldStore.ExistsByName` 查询**不过滤 `deleted` 列**，含软删除记录都视为已占用。

**调用链路：**

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fields/check-name` |
| Handler | `FieldHandler.CheckName` — 校验 `name` 非空 |
| Service | `FieldService.CheckName` — `FieldStore.ExistsByName` → 返回 `{available, message}` |

---

## 功能 7：字段引用详情（跨模块编排）

**场景 A — 在字段管理页，管理员想停用或删除某个字段之前，先看看谁在用它。** 接口返回两类引用方：哪些模板引用了它、哪些 reference 类型字段引用了它，附带引用方的中文标签。

**场景 B — 在字段管理页，删除接口返回"被引用无法删除"后，前端自动调用此接口展示引用详情，告诉管理员应该先去哪里解除引用。**

**分层职责**：`FieldService.GetReferences` **只负责字段模块内的数据**——查 `field_refs` 关系，并用 `FieldStore.GetByIDs` 批量拿"被其他 reference 字段引用"这一类的 label。返回的 `Templates` 数组里每项只带 `RefID`，`Label` 留空。`FieldHandler.GetReferences` 拿到结果后再跨模块调 `templateService.GetByIDsLite(templateIDs)`，填回到每条 template 引用上。

理论上引用方模板不可能缺失（因为字段被引用时模板不能被删，参见功能 5 和模板管理功能 5），如果真的缺失 handler 会 `slog.Warn` 并保留空 label。

**调用链路：**

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fields/references` |
| Handler | `FieldHandler.GetReferences` — 校验 `id > 0` → 调 Service → 跨模块调 `templateService.GetByIDsLite` 补 template label |
| Service | `FieldService.GetReferences` — `getFieldOrNotFound` → `FieldRefStore.GetByFieldID` → 按 `ref_type` 分组 → `FieldStore.GetByIDs` 批量取 field label（template 只填 RefID）|

---

## 功能 8：启用/停用切换

**场景 A — 在字段管理页，管理员新建完字段、确认配置无误后，启用它。** 启用后其他模块的字段下拉列表才能看到这个字段。

**场景 B — 在字段管理页，管理员要下线一个字段，先停用它。** 停用后：
- 其他模块的下拉列表立刻看不到它了，策划不会再选它
- 但已经引用它的模板不受影响，模板里已有的配置继续生效
- 如果确认不再需要，后续再执行删除

**停用一个被引用的字段是允许的**。这是"存量不动，增量拦截"的设计：已经在用的不打扰，新的不让用。

切换用乐观锁，版本冲突返回 `40010 ErrFieldVersionConflict`。

**调用链路：**

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fields/toggle-enabled` |
| Handler | `FieldHandler.ToggleEnabled` — 校验 `id > 0`、`version > 0` |
| Service | `FieldService.ToggleEnabled` — `getFieldOrNotFound` → 乐观锁更新 → 清自身 detail 缓存 + 列表缓存 |
| Store | `FieldStore.ToggleEnabled(id, enabled, version)` WHERE id=? AND version=? |

---

## 功能 9：字典选项查询

**场景 — 在字段管理页新建或编辑字段时，"字段类型"和"标签分类"的下拉选项不是前端写死的，而是从后端动态获取。** 这样运营团队可以随时在字典表里加新类型、新分类，不需要改代码重新部署。

字典数据在后端启动时从 MySQL 全量加载到内存（`cache.DictCache`），运行时直接读内存，不查表。Service 层也用同一份 `DictCache` 做 `type` / `category` 的 label 翻译（功能 1/7 都会用到）。

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

所以**被引用且类型未变**时，约束只能放宽不能收紧，违反规则返回 `40007 ErrFieldRefTighten`：

- **integer**：`min` 只能往小改、`max` 只能往大改（`step` 当前**不检查**）
- **float**：`min` 只能往小改、`max` 只能往大改、**`precision` 只能往大改**（降低 precision 会截断已存数据）
- **string**：`minLength` 只能往小改、`maxLength` 只能往大改、**`pattern` 只能移除不能新增/变更**（按下表判定）
- **select**：**`minSelect` 只能往小改、`maxSelect` 只能往大改**、`options` 只能加新选项不能删旧选项（按 `value` 比对）
- **boolean**：无约束，不检查
- **reference**：`refs` 的收紧由功能 11 的 `validateReferenceRefs` 单独处理（非空 / 存在 / 启用 / 非嵌套 / 无循环），**不在 `checkConstraintTightened` 内**

**pattern 判定表**：

| old | new | 结果 | 原因 |
|---|---|---|---|
| `""` | `""` | 允许 | 未变 |
| `""` | `"^x$"` | 拒绝 40007 | 新增 pattern：旧数据可能不匹配 |
| `"^x$"` | `"^x$"` | 允许 | 未变 |
| `"^x$"` | `"^y$"` | 拒绝 40007 | pattern 变化：旧数据可能不匹配新 |
| `"^x$"` | `""` | 允许 | 移除 pattern = 放宽 |

未被引用的字段（`ref_count == 0`）不受此限制，随便改。

此功能实现在 `service/field.go::checkConstraintTightened`，由功能 4（编辑字段）在 `old.RefCount > 0 && old.Type == req.Type` 时调用。

### 约束 key 命名契约（必须前后端严格对齐）

`properties.constraints` 是无 schema 的 `json.RawMessage`，DB 层不校验结构，命名靠前后端代码约定。**单一权威**为 seed 文件 `backend/cmd/seed/main.go` 中 `field_type` 字典每条记录的 `constraint_schema`。后端 `checkConstraintTightened` 和游戏服务端导出都直接读这些 key。前端 `frontend/src/components/FieldConstraint*.vue` 必须严格使用以下名称，**不得改成驼峰/下划线变体**——否则收紧检查（40007）静默失效。

| 字段类型 | constraint key | 说明 |
|---------|---------------|------|
| integer | `min` / `max` / `step` | 最小值/最大值/步长 |
| float | `min` / `max` / `precision` | 最小值/最大值/小数位数 |
| string | `minLength` / `maxLength` / `pattern` | 最小长度/最大长度/正则 |
| boolean | — | 无约束 |
| select | `options` / `minSelect` / `maxSelect` | 选项数组（每项 `{value, label}`）/最少选/最多选 |
| reference | `refs` | 被引用字段 ID 数组（前端 UI 用 `ref_fields` 富对象，提交前转为 `refs`）|

---

## 功能 11：reference 字段引用校验 + 引用关系维护

**场景 A — 在字段管理页创建 reference 类型字段时，要从下拉列表选择它引用哪些字段。** 下拉列表走功能 1 的 `enabled=true` 筛选，只展示启用的字段。提交时 Service 的 `validateReferenceRefs` 依次做 5 条校验：

| # | 规则 | 错误码 |
|---|---|---|
| 1 | `refs` 不能为空 | `40017 ErrFieldRefEmpty` |
| 2 | 每个目标字段必须存在 | `40014 ErrFieldRefNotFound` |
| 3 | 新增的目标字段必须启用 | `40013 ErrFieldRefDisabled` |
| 4 | **新增的目标字段不能是 reference 类型**（禁止嵌套） | `40016 ErrFieldRefNested` |
| 5 | 不能形成循环引用（DFS 检测） | `40009 ErrFieldCyclicRef` |

校验通过后 `syncFieldRefs` 写 `field_refs`（`ref_type='field'`）并对每个被引用字段 `IncrRefCountTx`。

**为什么禁止嵌套**：模板管理的前端设计假设 reference 字段只有"一层"——点开 popover 看到的子字段必然是 leaf（`integer / float / string / boolean / select`）。如果允许 `refB.refs = [refA]`，模板编辑页的 popover 会看到"子字段是另一个 reference"这种嵌套结构，要么递归展开要么放弃扁平化假设，成本很高。在字段管理层直接禁止嵌套是最简单的解法。

**场景 B — 在字段管理页编辑一个已有的 reference 类型字段，想加几个新引用或去掉几个旧引用。** 对**新增**的引用，目标必须启用且不能是 reference 类型（规则 3/4）；但对**已有**的引用（在 `oldRefSet` 集合中），即使那个字段后来被停用、或历史上就是嵌套的 reference，也允许保留不拦截。Service 层用 `oldRefSet` 集合区分"新增"与"已有"，只有新增的才走严格校验。

**场景 C — 在字段管理页删除一个 reference 类型字段时，它之前引用的那些字段的引用计数要减回去。** 这在删除事务内由 `RemoveBySource` + `DecrRefCountTx` 自动完成（功能 5）。

**循环检测**：`detectCyclicRef` 用 DFS 遍历 `currentID → newRefIDs → 每个 ref 的 refs → ...`，将 `currentID` 预先标记为已访问；遇到重复 ID 即返回 `40009`。

**前端双重防御**：`frontend/src/components/FieldConstraintReference.vue` 的 `loadEnabledFields` 在拿到启用字段列表后追加 `f.type !== 'reference'` 过滤，下拉从源头不再展示其他 reference 字段，用户不会误选；后端 `validateReferenceRefs` 的 `40016 ErrFieldRefNested` 作为兜底。`FieldForm.vue` 捕获 `40016` / `40017` 给出本地化中文 `ElMessage`（`FIELD_ERR.REF_NESTED` / `REF_EMPTY` 常量表见 `api/fields.ts`）。

### ⚠️ 已知小瑕疵：主记录与引用关系非原子

Create 时 `FieldStore.Create`（主记录）和 `syncFieldRefs`（引用关系同步）**不在同一个事务里**——主记录先 INSERT，`syncFieldRefs` 再单独开 tx 写。Update 路径同理。极端情况下主记录成功而引用关系失败时会出现不一致。待后续重构为统一事务。

相关实现：`service/field.go` 中的 `validateReferenceRefs` / `detectCyclicRef` / `syncFieldRefs` / `parseRefFieldIDs`。嵌入在功能 2（创建）、功能 4（编辑）、功能 5（删除）的 Service 层。

---

## 功能 12：跨模块对外接口（给模板管理调用）

为了让模板管理 handler 能在跨模块事务中操作字段模块的数据，`FieldService` 暴露以下对外方法（都不依赖 `TemplateService` / `TemplateCache`，严格单向依赖）：

| 方法 | 用途 | 事务归属 |
|------|------|---------|
| `ValidateFieldsForTemplate(ctx, fieldIDs)` | 模板创建/编辑时校验被勾选字段全部存在 + 启用 + 非 reference 类型。任一不存在返回 `41006 ErrTemplateFieldNotFound`，任一停用返回 `41005 ErrTemplateFieldDisabled`，任一为 reference 类型返回 `41012 ErrTemplateFieldIsReference`。底层 `FieldStore.GetByIDs` 一次批量取 | 事务外预校验 |
| `AttachToTemplateTx(ctx, tx, templateID, fieldIDs)` | 在外部 tx 内对每个 field 写入 `field_refs(field_id, 'template', templateID)` + `IncrRefCountTx`，返回 `fieldIDs` 副本供 handler commit 后清缓存 | 外部 tx |
| `DetachFromTemplateTx(ctx, tx, templateID, fieldIDs)` | 在外部 tx 内对每个 field 删除 `field_refs` 行 + `DecrRefCountTx`，返回 `fieldIDs` 副本 | 外部 tx |
| `GetByIDsLite(ctx, fieldIDs)` | 给模板详情接口拼装 `TemplateFieldItem` 用。按 `fieldIDs` 顺序对齐返回 `[]FieldLite`，缺失的位置保持 `FieldLite{ID:0}` 零值（handler 识别后 `slog.Warn` 并跳过）；`CategoryLabel` 由本方法用 `DictCache` 翻译填充 | 无 tx |
| `InvalidateDetails(ctx, fieldIDs)` | 模板写操作 commit 后由 handler 调用（因为模板写会改字段的 `ref_count`），批量清 detail 缓存，不返回 error | 无 tx |

**重要原则**：`41005` / `41006` / `41012` 这三个错误码归在**模板段位**（41xxx），因为它们**由模板管理页消费**，与字段管理自身的 `40011 ErrFieldNotFound` / `40013 ErrFieldRefDisabled` / `40016 ErrFieldRefNested` 语义不混用。

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
| 操作标识 | 主键 ID (BIGINT)，`name` 仅用于创建和唯一性校验 |
| 统一响应格式 | `handler.WrapCtx` 泛型包装，返回 `{Code, Data, Message}` |
| 错误码体系 | 17 个字段段错误码（40001-40017），语义分离 |
| 缓存穿透防护 | 空值标记，`FieldCache.SetDetail` 对 `nil` field 也写缓存 |
| 缓存击穿防护 | `GetByID` 使用分布式锁 `TryLock(id, 3s)` + double-check |
| 缓存雪崩防护 | TTL 加随机 jitter |
| 缓存批量失效 | 列表缓存版本号，`InvalidateList` 即失效 |
| 缓存类型安全 | 列表缓存使用 `FieldListData`（`[]FieldListItem`）|
| 缓存降级 | Redis 不可用时直接穿透到 MySQL |
| 缓存 Key | `fields:detail:{id}`、`fields:lock:{id}`、列表缓存版本号 key |
| 乐观锁 | `UPDATE ... WHERE id=? AND version=?`，rows=0 → `ErrVersionConflict` → 40010 |
| 软删除 | `deleted=1`，所有查询过滤 `WHERE deleted=0` |
| 引用计数 | `ref_count` 冗余字段，事务内原子维护（`IncrRefCountTx` / `DecrRefCountTx`）|
| TOCTOU 防护 | 删除在事务内 `HasRefsTx` 用 `FOR SHARE` 重新检查 `field_refs` |
| 覆盖索引 | `idx_list` 列表查询不回表 |
| 输入校验分层 | Handler 做格式校验（ID>0 / name 正则 / label 长度 / 必填 / `properties` JSON 对象形状 / version>0），Service 做业务校验（存在性 / 启用状态 / 引用 / 循环 / 嵌套 / 收紧）|
| 编辑限制 | 只有未启用状态才能编辑（`40015 ErrFieldEditNotDisabled`）|
| 跨模块边界 | `FieldService` 只持有自身 store/cache；跨模块拼装由 Handler 编排；跨模块事务由模板 handler 开 tx 后调 `*Tx` 方法 |
| 常量管理 | 字段类型、Redis key、TTL、引用类型统一为常量（`FieldTypeReference` / `RefTypeTemplate` / `RefTypeField` 等）|

---

## 错误码速查（字段段 40001-40017）

| 错误码 | 常量 | 含义 |
|--------|------|------|
| 40001 | `ErrFieldNameExists` | 字段标识已存在（含软删除） |
| 40002 | `ErrFieldNameInvalid` | 字段标识格式不合法 |
| 40003 | `ErrFieldTypeNotFound` | 字段类型不存在 |
| 40004 | `ErrFieldCategoryNotFound` | 标签分类不存在 |
| 40005 | `ErrFieldRefDelete` | 被引用无法删除 |
| 40006 | `ErrFieldRefChangeType` | 被引用无法修改类型 |
| 40007 | `ErrFieldRefTighten` | 被引用无法收紧约束（涵盖 min/max/minLength/maxLength/precision/pattern/minSelect/maxSelect/options 删除）|
| 40008 | `ErrFieldBBKeyInUse` | BB Key 被行为树引用（预留，未接入）|
| 40009 | `ErrFieldCyclicRef` | 循环引用 |
| 40010 | `ErrFieldVersionConflict` | 乐观锁版本冲突 |
| 40011 | `ErrFieldNotFound` | 字段不存在 |
| 40012 | `ErrFieldDeleteNotDisabled` | 删除前必须先停用 |
| 40013 | `ErrFieldRefDisabled` | 不能引用已停用的字段 |
| 40014 | `ErrFieldRefNotFound` | 引用的字段不存在 |
| 40015 | `ErrFieldEditNotDisabled` | 编辑前必须先停用 |
| 40016 | `ErrFieldRefNested` | reference 字段禁止嵌套引用 |
| 40017 | `ErrFieldRefEmpty` | reference 字段 `refs` 不能为空 |

---

## 已知限制

| 限制 | 说明 | 计划 |
|------|------|------|
| Create + `syncFieldRefs` 非原子 | reference 字段主记录和 `field_refs` 同步不在同一事务中，极端情况可能不一致（Create/Update 都是这样）| 后续重构为统一事务 |
| 通用 `ListData.Items` 为 `any` | HTTP 响应层仍用 `ListData{Items: any}`，仅缓存层做了类型安全（`FieldListData`）| 未来可泛型化 `ListData` |
| BB Key 校验未对接 | 错误码 `40008` 已定义，但 expose_bb 变更检查待 BT 模块提供接口 | BT 模块开发时对接 |
