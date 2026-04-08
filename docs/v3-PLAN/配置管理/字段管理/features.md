# 字段管理 — 功能需求

> 字段是 ADMIN 内部的管理概念，定义"NPC 可以有什么属性"。不写入 MongoDB，不导出给游戏服务端。
> 字段值最终通过 模板→NPC 打平写入 npc_templates 导出。

---

## 需求 1：字段列表

展示所有字段定义，支持搜索、筛选、分页。

- 搜索框按**中文标签**模糊搜索（后端 MySQL LIKE 查询）
- 筛选：字段类型下拉 + 标签分类下拉（选项从 dictionaries API 动态获取，缓存在 Redis）
- 搜索 / 重置按钮
- 列表按 ID 倒序排列，最新创建的在第一页
- 后端分页，每页 20 条
- 表格列：ID、字段名、中文标签、类型、标签分类、被引用数、创建时间、操作
- 表格列自适应页面宽度，均分
- 每行左侧有 checkbox（用于批量操作）
- 操作列：编辑、删除
- 类型/分类的中文 label 由后端内存 map 翻译，不 JOIN dictionaries 表

## 需求 2：新建字段

独立页面（保留侧边栏），表单分两区：

### 固定区（横线上方）

固定列，写死在前端和 MySQL 列中，不会变。

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| 字段标识 | 文本输入 | 是 | snake_case，正则 `^[a-z][a-z0-9_]*$`，创建后不可修改 |
| 中文标签 | 文本输入 | 是 | 运营可见的显示名 |
| 字段类型 | 下拉选择 | 是 | 从 dictionaries(group=field_type) 获取 |
| 标签分类 | 下拉选择 | 是 | 从 dictionaries(group=field_category) 获取 |

### 动态区（横线下方）

由 dictionaries(group=field_properties) 驱动，前端按 sort_order 遍历动态渲染。当前包含：

| 字段 | input_type | 必填 | 说明 |
|------|------------|------|------|
| 描述说明 | textarea | 否 | 解释字段用途 |
| 暴露 BB Key | radio_bool | 是 | 是否暴露为行为树黑板 Key，默认"否" |
| 默认值 | dynamic | 否 | 根据所选字段类型动态渲染对应输入控件 |
| 约束配置 | constraints | 否 | 根据所选字段类型动态展示约束项，详见需求 7 |

未来新增属性 = dictionaries 插一条 → 前端自动渲染 → **零代码改动**。

用户填写的动态区数据全部存入 MySQL fields.properties JSON 列。

- 底部操作：取消（返回列表）、确定（提交创建）

## 需求 3：编辑字段

独立页面（保留侧边栏），与新建共用表单布局，区别：

- 字段标识**锁定不可编辑**，灰色背景 + 锁图标 + 黄色提示"字段标识创建后不可修改"
- 所有其他字段预填当前值
- 约束配置展开当前类型的约束项并预填值
- 底部操作：取消、保存

### 编辑限制（硬约束）

- **字段类型**：被引用时（ref_count > 0）**禁止修改**，类型下拉置灰，提示"该字段已被 N 个模板/字段引用，无法修改类型"
- **约束收紧**：被引用时**禁止收紧**约束（如缩小 min/max 范围），提示"已有数据可能超出新约束范围，请先移除引用"
- **关闭 BB Key**：被行为树引用时**禁止关闭**，提示"该 Key 正被 N 棵行为树使用，无法关闭"
- **并发冲突**：乐观锁，保存时版本号不匹配则提示"该字段已被其他人修改，请刷新后重试"

## 需求 4：删除字段

**硬约束：被引用时禁止删除。**

引用来源有两种（通过 field_refs 表查询）：
- `ref_type = 'template'`：被模板引用
- `ref_type = 'field'`：被其他 reference 字段引用

| 场景 | 行为 |
|------|------|
| 无引用 | 弹出确认弹窗（绿色提示"可安全删除"），确认后软删除 |
| 有引用 | 弹出警告弹窗，列出引用方（模板名/字段名），**禁止删除**，提示"请先移除引用后再删除" |

- 软删除的字段标识仍占用唯一性
- 删除只操作 MySQL，不涉及 MongoDB

## 需求 5：字段名唯一性实时校验

- 新建页面输入字段标识时，失焦或输入停顿后实时调用后端接口校验
- 后端：`SELECT id FROM fields WHERE name = ? LIMIT 1`（走 uk_name 唯一索引）
- 三种状态：
  - 格式不合法：红色边框 + 红字提示格式要求（前端正则校验，不请求后端）
  - 已存在：红色边框 + 红字"该字段标识已存在"
  - 可用：绿色边框 + 绿字"该标识可用"

## 需求 6：字段引用详情

- 列表页"被引用数"列可点击（数字带链接样式）
- 点击后弹出弹窗，分两类展示：
  - 模板引用：模板名 + 模板分类
  - 字段引用：reference 字段名 + 中文标签
- 后端：先查 `field_refs` 表（主键索引），再 IN 查 templates/fields 表拿 label（不 JOIN）
- 引用数为 0 时显示为普通文本，不可点击

## 需求 7：约束配置（按类型动态展示）

选择字段类型后，约束配置区域动态渲染。约束项由 dictionaries(group=field_type) 中对应类型的 `extra.constraint_schema` 定义，前端 SchemaForm 按 schema 渲染。

### integer（整数）
| 约束项 | 说明 |
|--------|------|
| 最小值 | 数值输入 |
| 最大值 | 数值输入 |
| 步长 | 数值输入，默认 1 |

### float（浮点数）
| 约束项 | 说明 |
|--------|------|
| 最小值 | 数值输入 |
| 最大值 | 数值输入 |
| 小数位数 | 数值输入（precision） |

### string（文本）
| 约束项 | 说明 |
|--------|------|
| 最小长度 | 数值输入 |
| 最大长度 | 数值输入 |
| 正则校验 | 文本输入（选填） |

### boolean（布尔）
无约束配置项，显示提示"布尔类型无需约束配置"。

### select（选择）
| 约束项 | 说明 |
|--------|------|
| 选项列表 | 每行一个选项：值（value）+ 标签（label），支持添加/删除行 |
| 最少选择数 | 数值输入（minSelect），min=1 max=1 为单选 |
| 最多选择数 | 数值输入（maxSelect），max>1 为多选 |

默认值自动取第一个选项。值始终为数组格式。

### reference（引用/复合字段）
| 约束项 | 说明 |
|--------|------|
| 引用字段列表 | 从已有字段池中选择，支持添加/删除/拖拽排序 |

- 可引用普通字段和其他 reference 字段
- 引用 reference 字段时递归展开、自动去重
- **循环引用检测**：添加引用时实时检测，禁止形成循环（A→B→A）
- reference 字段本身不产生数据，模板勾选时展开为实际字段
- 导出时 reference 不出现，其包含的字段值直接打平到 NPC fields 中
- 展开预览：约束配置区域底部展示"模板勾选时实际包含的字段"列表

未选择类型时，显示占位提示"请先选择字段类型，约束项将根据类型动态展示"。

## 需求 8：批量操作

- 列表页每行左侧 checkbox，表头全选 checkbox
- 选中后表格上方出现批量操作栏，显示"已选 N 项"
- 选中行高亮
- 支持操作：
  - **批量删除**：逐条检查 field_refs 引用，有引用的跳过，汇总报告"N 项已删除，M 项因被引用无法删除"
  - **批量修改分类**：弹出分类选择下拉，确认后批量更新 MySQL fields.category
- 操作栏右侧"取消选择"按钮

## 需求 9：dictionaries 选项管理

字段管理依赖的下拉选项统一存储在 MySQL `dictionaries` 表，启动时全量加载到 Redis，运行时不查表。

当前需要的 group：
- `field_type`：字段类型（integer, float, string, boolean, select, reference），extra 含 constraint_schema
- `field_category`：标签分类（基础属性, 战斗属性, 感知属性, 移动属性, 交互属性, 个性属性）
- `field_properties`：动态表单属性（描述说明, 暴露BB Key, 默认值, 约束配置），extra 含 input_type

选项的增删改通过「系统设置 → Schema 管理」页面统一管理，字段管理页面本身只读取。
修改后刷新 Redis 缓存 + 后端内存 map。

## 需求 10：数据存储

### MySQL 表结构

**fields 表**：
- 固定列：id, name, label, type, category（骨架 + 搜索筛选）
- 动态列：properties JSON（描述、BB Key、默认值、约束配置、未来新增属性）
- 管理列：ref_count（冗余计数）, version（乐观锁）, deleted, created_at, updated_at
- 覆盖索引 idx_list：列表查询不回表
- 唯一索引 uk_name：唯一性校验 + 单条详情查询

**field_refs 表**：
- 联合主键 (field_name, ref_type, ref_name)
- 存储字段被谁引用：ref_type='template'（模板引用）/ ref_type='field'（reference 字段引用）
- 删除检查、引用详情查询走主键索引

**dictionaries 表**：
- 联合唯一键 (group_name, name)
- 启动加载到 Redis，运行时不查表
- 覆盖索引 idx_group_list

### 写入流程

| 操作 | MySQL | MongoDB |
|------|-------|---------|
| 创建字段 | INSERT fields | 不操作 |
| 编辑字段 | UPDATE fields（乐观锁） | 不操作 |
| 删除字段 | UPDATE fields SET deleted=1 | 不操作 |
| 查询列表 | SELECT fields（覆盖索引） | 不操作 |
| 查引用详情 | SELECT field_refs → IN 查 templates/fields | 不操作 |

**字段全程只和 MySQL 打交道，不涉及 MongoDB。**

### 扩展方式

| 扩展场景 | 做什么 | 改动量 |
|---------|--------|--------|
| 横线上方加通用属性 | dictionaries(field_properties) 插一条 | 零代码 |
| 横线下方加约束项 | dictionaries(field_type) 改 constraint_schema | 零代码 |
| 新增字段类型 | dictionaries(field_type) 插一条 + 带 constraint_schema | 零代码 |
| 新增标签分类 | dictionaries(field_category) 插一条 | 零代码 |
| 新增需要筛选的属性 | MySQL fields 加列 + 后端加筛选参数 | 小改动（极少发生） |

---

## 延后功能（毕设后）

| 功能 | 说明 |
|------|------|
| 字段导入/导出 | CSV/Excel 批量导入导出 |
| 列头排序 | 点击列头切换排序方式 |
| 字段克隆/复制 | 待定，用户考虑中 |
