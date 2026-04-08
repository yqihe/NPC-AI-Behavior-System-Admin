# 字段管理 — 已实现功能清单

> 字段是 ADMIN 内部的管理概念，定义"NPC 可以有什么属性"。全程只和 MySQL 打交道，不涉及 MongoDB。
> 字段值最终通过 模板→NPC 打平写入 npc_templates 导出。

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

两个场景走同一个接口，靠 `enabled` 参数区分。支持按中文标签模糊搜索、字段类型/标签分类精确筛选、后端分页。

**调用链路：**

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fields/list` |
| Handler | `FieldHandler.List` |
| Service | `FieldService.List` |
| Store | `FieldCache.GetList` → `FieldStore.List` → `FieldCache.SetList` |

---

## 功能 2：新建字段

**场景 — 在字段管理页，管理员要定义一个新的 NPC 属性（比如"生命值"、"阵营"）。** 填写字段标识、中文标签、类型、分类和动态属性后提交。

新建的字段默认是**未启用**状态（enabled=false）。这是一个刻意的设计：管理员创建字段后，往往还需要反复调整约束、默认值等配置。如果创建即启用，模板管理页的下拉列表会立刻出现这个半成品字段，策划可能在管理员还没配好之前就选了它。默认未启用就提供了一个"配置窗口期"——管理员可以反复编辑、确认无误后再手动启用，启用后其他模块才能看到并使用它。

字段标识（name）一旦创建不可修改（是唯一键），且含软删除记录也不能重复使用，防止历史数据混乱。

**调用链路：**

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fields/create` |
| Handler | `FieldHandler.Create` |
| Service | `FieldService.Create` |
| Store | `DictCache.Exists` → `FieldStore.ExistsByName` → `FieldStore.Create` → `FieldCache.InvalidateList` |

---

## 功能 3：字段详情

**场景 A — 在字段管理页，管理员点击某个字段查看或准备编辑。** 需要拿到完整的字段信息，包括动态属性 properties。

**场景 B — 在模板管理页，策划选中一个字段后，前端要展示这个字段的约束信息（比如取值范围），用于渲染动态表单。** 同样调用详情接口拿到完整数据。

无论字段是启用还是停用，详情都能查。因为已经加到模板里的停用字段，策划仍然需要看到它的配置内容。

**调用链路：**

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fields/detail` |
| Handler | `FieldHandler.Get` |
| Service | `FieldService.GetByName` |
| Store | `FieldCache.GetDetail` → `TryLock`（防击穿） → `FieldStore.GetByName` → `FieldCache.SetDetail` |

---

## 功能 4：编辑字段

**场景 A — 在字段管理页，管理员要修改字段的标签、类型、分类或约束。** 无论字段是启用还是停用都能编辑。

如果这个字段已经被模板或其他字段引用了（ref_count > 0），有两个硬约束：
- 不能改类型。比如从"整数"改成"字符串"，已经引用它的模板里填的值全乱了。
- 不能收紧约束。比如最大值从 100 改成 50，模板里已经填了 80 的值就不合法了。

**场景 B — 在字段管理页，管理员把一个 reference 类型字段改成了其他类型（比如改成字符串）。** 这意味着这个字段不再引用其他字段了，后端会自动清理它之前对其他字段的所有引用关系，那些被引用字段的引用计数也会自动减回去。管理员不需要手动去处理。

编辑用乐观锁防止两个管理员同时改同一个字段互相覆盖。

**调用链路：**

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fields/update` |
| Handler | `FieldHandler.Update` |
| Service | `FieldService.Update` |
| Store | `DictCache.Exists` → `getFieldOrNotFound` → `FieldStore.Update`（乐观锁） → 清缓存 |

---

## 功能 5：删除字段

**场景 — 在字段管理页，管理员要彻底移除一个不再需要的字段。**

删除有两道门槛：
1. 必须先停用。这是给管理员一个缓冲期——停用后观察一段时间，确认没有问题再删。
2. 不能有引用。如果还有模板或其他字段在引用它，删不掉，接口会返回具体是谁在引用，管理员据此去处理。

删除是软删除（标记 deleted=1），不是物理删除。如果这个字段本身是 reference 类型（引用了其他字段），删除时会自动清理它对其他字段的引用关系。

**调用链路：**

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fields/delete` |
| Handler | `FieldHandler.Delete` |
| Service | `FieldService.Delete` |
| Store | `getFieldOrNotFound` → 预检引用 → 事务内 FOR SHARE 复查 → 软删除 → 清缓存 |

---

## 功能 6：字段名唯一性校验

**场景 — 在字段管理页新建字段时，管理员输入字段标识后离开输入框，前端实时告知这个名字能不能用。**

即使某个字段已经被删除了，它的标识也不能被新字段复用。这是因为字段标识会写入模板配置、导出给游戏服务端，历史数据中可能残留旧标识的引用。如果允许复用，新字段和旧数据的含义完全不同，会导致难以排查的数据错乱。

**调用链路：**

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fields/check-name` |
| Handler | `FieldHandler.CheckName` |
| Service | `FieldService.CheckName` |
| Store | `FieldStore.ExistsByName` |

---

## 功能 7：字段引用详情

**场景 A — 在字段管理页，管理员想停用或删除某个字段之前，先看看谁在用它。** 接口返回两类引用方：哪些模板引用了它、哪些 reference 类型字段引用了它，附带引用方的中文名。

**场景 B — 在字段管理页，删除接口返回"被引用无法删除"后，前端自动调用此接口展示引用详情，告诉管理员应该先去哪里解除引用。**

**调用链路：**

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fields/references` |
| Handler | `FieldHandler.GetReferences` |
| Service | `FieldService.GetReferences` |
| Store | `getFieldOrNotFound` → `FieldRefStore.GetByFieldName` → `FieldStore.GetByNames`（拿 label） |

---

## 功能 8：批量删除

**场景 — 在字段管理页，管理员勾选多个字段一起删除，用于批量清理不需要的字段。**

逐条检查，能删的删，不能删的跳过，最后汇总报告告诉管理员哪些删了、哪些因为什么原因没删。每个字段的删除是独立事务，一个失败不影响其他。

跳过的原因包括：还在启用状态、被引用、字段不存在。

**调用链路：**

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fields/batch-delete` |
| Handler | `FieldHandler.BatchDelete` |
| Service | `FieldService.BatchDelete` |
| Store | 循环：预检引用 → 事务内 FOR SHARE 复查 → 软删除 → 清缓存 |

---

## 功能 9：批量修改分类

**场景 — 在字段管理页，管理员要把一批字段从"基础属性"分类移到"战斗属性"分类。** 勾选多个字段，选择目标分类，一次性修改。

无论字段是启用还是停用都能改分类，因为分类只是管理维度，不影响字段的实际功能。

**调用链路：**

| 层 | 入口 |
|---|---|
| Router | `POST /api/v1/fields/batch-category` |
| Handler | `FieldHandler.BatchUpdateCategory` |
| Service | `FieldService.BatchUpdateCategory` |
| Store | `DictCache.Exists` → `FieldStore.BatchUpdateCategory`（IN 查询） → 清缓存 |

---

## 功能 10：启用/停用切换

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
| Handler | `FieldHandler.ToggleEnabled` |
| Service | `FieldService.ToggleEnabled` |
| Store | `getFieldOrNotFound` → `FieldStore.ToggleEnabled`（乐观锁） → 清缓存 |

---

## 功能 11：字典选项查询

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

## 功能 12：约束收紧检查

**场景 — 在字段管理页编辑字段时，如果这个字段已经被模板引用了，管理员想修改它的约束。** 比如把生命值的最大值从 100 改成 50。

问题是：模板里可能已经有人填了 80。如果允许改成 50，那 80 就超出范围了，数据就不一致了。

所以被引用时，约束只能放宽不能收紧：
- 数值类型：最小值只能往小改，最大值只能往大改
- 字符串：最小长度只能往小改，最大长度只能往大改
- 下拉选项：只能加新选项，不能删旧选项

没有被引用的字段不受此限制，随便改。

---

## 功能 13：循环引用检测 + 引用关系维护

**场景 A — 在字段管理页创建 reference 类型字段时，要从下拉列表选择它引用哪些字段。** 下拉列表只展示启用的字段（走功能 1 的 `enabled=true` 筛选）。选好后，后端确认被引用的字段存在且是启用的，检查不会形成循环引用，然后记录引用关系并更新被引用字段的引用计数。

**场景 B — 在字段管理页编辑一个已有的 reference 类型字段，想加几个新引用或去掉几个旧引用。** 对于新增的引用，被引用字段必须是启用的，停用的不让加。但对于已有的引用，即使那个字段后来被停用了，也允许保持——不会强制管理员去掉它。这和"存量不动，增量拦截"是同一个原则。

**场景 C — 在字段管理页删除一个 reference 类型字段时，它之前引用的那些字段的引用计数要减回去。** 这在删除事务内自动完成。

---

## 横切关注点

| 关注点 | 实现方式 |
|--------|---------|
| 统一响应格式 | `handler.WrapCtx` 泛型包装，返回 `{Code, Data, Message}` |
| 错误码体系 | `ErrFieldNotFound`(40011) 字段不存在 / `ErrFieldRefNotFound`(40014) 引用字段不存在，语义分离 |
| 缓存穿透防护 | 空值标记 `{"_null":true}`，未命中时也缓存 nil |
| 缓存击穿防护 | `GetByName` 使用分布式锁 `TryLock` + double-check |
| 缓存雪崩防护 | TTL 加随机 jitter |
| 缓存批量失效 | 列表缓存版本号，INCR 即失效 |
| 缓存类型安全 | 列表缓存使用 `FieldListData`（`[]FieldListItem`） |
| 缓存降级 | Redis 不可用时直接穿透到 MySQL |
| 乐观锁 | `UPDATE ... WHERE version = ?`，rows=0 返回版本冲突 |
| 软删除 | `deleted=1`，所有查询过滤 `WHERE deleted=0` |
| 引用计数 | `ref_count` 冗余字段，事务内原子维护 |
| TOCTOU 防护 | 单条/批量删除均在事务内 `FOR SHARE` 重新检查引用 |
| 覆盖索引 | `idx_list` 列表查询不回表 |
| 输入校验分层 | Handler 做格式校验，Service 做业务校验 |
| 常量管理 | 字段类型、Redis key、TTL 统一为常量，不硬编码 |

---

## 已知限制

| 限制 | 说明 | 计划 |
|------|------|------|
| Create + syncFieldRefs 非原子 | reference 字段创建与引用关系同步在不同事务中，极端情况可能不一致 | 模板管理上线时统一重构为事务内操作 |
| BatchUpdateCategory 无乐观锁 | 批量修改分类不检查 version，可能覆盖并发编辑的 category | 批量操作设计如此，管理员操作频率低 |
| 通用 ListData.Items 为 any | HTTP 响应层仍用 `ListData{Items: any}`，仅缓存层做了类型安全 | 未来可泛型化 ListData |
