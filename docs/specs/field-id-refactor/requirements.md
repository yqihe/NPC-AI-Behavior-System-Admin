# 字段管理后端重构 — 需求分析

## 动机

当前字段管理所有接口用 `name` (VARCHAR 64) 做操作标识：请求体传 name、Store 层按 name 查、Redis key 带 name、field_refs 表用 name 关联。这带来三个问题：

1. **性能**：VARCHAR 比较慢于 BIGINT，field_refs 的 JOIN/IN 查询在数据量增长后劣势明显
2. **规范**：企业级 CRUD 系统通用做法是主键 ID 做操作标识，name 只用于展示和唯一性校验
3. **接口膨胀**：现有 10 个接口中，批量删除和批量修改分类对运营人员直接暴露不安全，应精简为后端人员手动调用

同时修正一条业务规则：**编辑字段应限制为未启用状态**。启用中的字段已对外可见，允许随意编辑会导致引用方看到不稳定的配置。

不做的后果：模板管理模块即将开发，如果继续用 name 关联，field_refs 表和跨模块集成契约都将基于 VARCHAR，后续再改成本翻倍。

## 优先级

**高 — 必须在模板管理开发前完成。** 模板管理会依赖 field_refs 表结构和字段管理的 API 契约。现在改是最低成本的窗口期。

## 预期效果

### 场景 1：创建字段
管理员在字段管理页填写表单提交。请求体包含 name/label/type/category/properties，无 ID。后端创建记录，返回 `{id, name}`。新字段默认 `enabled=0`。

### 场景 2：编辑字段
管理员点击某个未启用字段的编辑按钮。请求体包含 `id` + 全部可编辑字段 + `version`。后端按 ID 查找，校验 `enabled=0`，执行编辑逻辑（乐观锁）。如果字段已启用，返回错误。

### 场景 3：查看列表
管理员进入字段列表页。支持分页、按 label 模糊搜索、按 type/category/enabled 精确筛选。列表项包含 id 字段，前端用 id 发起后续操作。

### 场景 4：软删除
管理员点击某个未启用字段的删除按钮。请求体传 `{id}`。后端校验 `enabled=0` 且 `ref_count=0`（无引用），执行软删除。

### 场景 5：启用/停用切换
管理员点击 toggle 开关。请求体传 `{id, enabled, version}`。后端乐观锁更新。

### 场景 6：字段名唯一性校验
管理员新建字段时输入 name，失焦触发校验。请求体传 `{name}`（创建前校验，只能用 name）。

### 场景 7：引用详情
管理员点击引用数查看详情。请求体传 `{id}`。返回引用该字段的模板和字段列表。

### 场景 8：字典选项
前端渲染下拉框时调用。请求体传 `{group_name}`。返回该组的字典项列表。

## 依赖分析

| 方向 | 内容 |
|------|------|
| 依赖 | MySQL/Redis 基础设施（已就绪）、字典种子数据（已就绪） |
| 被依赖 | 模板管理（未开发，将依赖新的 field_refs 表结构和 API 契约） |
| 被依赖 | 行为树模块（未开发，将依赖 BB Key 查询，但接口不变） |

## 改动范围

| 包 | 文件数 | 改动性质 |
|---|--------|---------|
| backend/migrations/ | 1 新建 | 新增迁移脚本（重建 field_refs 表，idx_list 索引加 enabled） |
| backend/internal/model/ | 1 | 重写请求/响应结构体，FieldRef 改用 field_id |
| backend/internal/errcode/ | 1 | 新增错误码 ErrFieldEditNotDisabled，移除批量相关错误码 |
| backend/internal/handler/ | 1 | 重写 Handler，移除批量接口，校验改用 ID |
| backend/internal/service/ | 1 | 重写 Service，所有查找改用 ID，编辑加 enabled 检查 |
| backend/internal/store/mysql/ | 2 | field.go + field_ref.go 全部改用 ID |
| backend/internal/store/redis/ | 2 | field.go 缓存改用 ID，keys.go key 格式改用 ID |
| backend/internal/router/ | 1 | 移除批量路由 |
| backend/cmd/admin/ | 0 | 启动注入不变（接口签名不变） |
| **合计** | **10 文件** | |

## 扩展轴检查

- **新增配置类型**（如模板管理）：field_refs 改用 BIGINT ID 后，模板管理只需按新契约调用 `IncrRefCount(tx, fieldID)` / `DecrRefCount(tx, fieldID)`，不影响扩展性。**正面影响**。
- **新增表单字段**（如给 properties 加新 key）：不涉及，properties JSON 的扩展方式不变。**无影响**。

## 验收标准

### 接口层

| 编号 | 标准 |
|------|------|
| R1 | `POST /api/v1/fields/create` 接收 `{name,label,type,category,properties}`（无 id），返回 `{id,name}`，新建记录 `enabled=0` |
| R2 | `POST /api/v1/fields/update` 接收 `{id,label,type,category,properties,version}`（有 id，无 name），仅 `enabled=0` 时允许编辑，否则返回错误 |
| R3 | `POST /api/v1/fields/list` 接收分页+筛选参数，返回分页列表（每项含 id），仅展示 `deleted=0` 的记录 |
| R4 | `POST /api/v1/fields/delete` 接收 `{id}`，仅 `enabled=0` 且 `ref_count=0` 时允许软删除 |
| R5 | `POST /api/v1/fields/toggle-enabled` 接收 `{id,enabled,version}`，乐观锁切换启用状态 |
| R6 | `POST /api/v1/fields/check-name` 接收 `{name}`，返回 `{available,message}` |
| R7 | `POST /api/v1/fields/references` 接收 `{id}`，返回引用该字段的模板和字段列表（含 label） |
| R8 | `POST /api/v1/dictionaries` 行为不变 |

### 存储层

| 编号 | 标准 |
|------|------|
| R9 | `field_refs` 表主键改为 `(field_id BIGINT, ref_type, ref_id BIGINT)`，不再使用 VARCHAR name |
| R10 | Redis 详情缓存 key 改为 `fields:detail:{id}`，锁 key 改为 `fields:lock:{id}` |
| R11 | 覆盖索引 `idx_list` 包含 enabled 列，列表查询不回表 |

### 业务规则

| 编号 | 标准 |
|------|------|
| R12 | 编辑字段：`enabled=1` 时返回新错误码，禁止编辑 |
| R13 | 删除字段：`enabled=1` 时返回 `40012 ErrFieldDeleteNotDisabled` |
| R14 | 删除字段：`ref_count>0` 时返回 `40005 ErrFieldRefDelete` |
| R15 | 编辑时约束收紧检查保留（ref_count > 0 时约束只能放宽） |
| R16 | 编辑/创建 reference 类型时循环引用检测保留 |
| R17 | 编辑/创建 reference 类型时引用关系维护保留（field_refs + ref_count 事务内原子操作） |
| R18 | 缓存策略保留：版本号失效列表缓存、分布式锁防击穿、空标记防穿透、Redis 降级 |

### 移除项

| 编号 | 标准 |
|------|------|
| R19 | 无 `batch-delete` 路由和 Handler/Service 代码 |
| R20 | 无 `batch-category` 路由和 Handler/Service 代码 |

## 不做什么

- **不做物理删除**：只做软删除，已删除数据保留在数据库
- **不做批量操作 UI**：batch-delete 和 batch-category 接口不保留，需要时由后端人员直接操作数据库
- **不做前端改动**：本次只改后端，前端由用户自行处理
- **不改字典模块**：字典表结构和接口不变
- **不改 seed 脚本**：种子数据不受影响
- **不改导出 API**：导出给游戏服务端的接口不涉及字段管理
