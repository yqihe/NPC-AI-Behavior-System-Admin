# 字段管理 — 功能清单

## 状态模型

| 状态 | 本模块可见 | 其他模块 | 能被新引用 | 已有引用 |
|------|-----------|---------|----------|---------|
| 启用 | 正常 | 可见可选 | 允许 | 正常 |
| 禁用 | 灰色 | 不可见 | 拒绝 | 保持 |
| 已删除 | 不可见 | 不可见 | 不可能 | 已清理 |

核心原则：**禁用 = 对新隐藏、对旧保留；删除 = 确认无引用后清理。**

## 功能列表

1. **CRUD**：标识符(name)、中文标签(label)、类型(type)、分类(category)、属性(properties JSON)
2. **类型系统**：integer / float / string / boolean / select / reference
3. **约束配置**：按类型不同 — integer/float(min/max/step/precision)、string(minLength/maxLength/pattern)、select(options/minSelect/maxSelect)、reference(refs 子字段 ID 列表)
4. **引用追踪**：`field_refs` 表记录哪些模板/字段/FSM 引用了该字段
5. **引用保护**：有引用时类型不可改、约束只能放宽（`util.CheckConstraintTightened`）
6. **删除保护**：先调 references API 查引用 → 有引用弹详情阻止 → 无引用确认删除 → 后端 `HasRefsTx(FOR SHARE)` 兜底
7. **启用/禁用**：乐观锁切换，启用前确认、禁用前确认
8. **BB Key 暴露**：`expose_bb=true` 的字段标识符成为 BB Key，可被 FSM 条件引用
9. **BB Key 保护**：取消 `expose_bb` 时检查 FSM 引用，有引用则拒绝(40008)
10. **引用详情 API**：返回模板引用方 + 字段引用方 + FSM 引用方
11. **详情 has_refs**：实时查 `field_refs` 返回 bool，不缓存

## 引用关系

| 引用方 | ref_type | 触发时机 |
|--------|----------|---------|
| 模板 | `template` | 模板创建/编辑勾选字段 |
| reference 字段 | `field` | 字段创建/编辑设置 refs |
| FSM 条件 | `fsm` | FSM 创建/编辑条件树中引用 BB Key |

## API 端点（8 个）

| 端点 | 说明 |
|------|------|
| POST /fields/list | 分页列表（支持 label/type/category/enabled 筛选） |
| POST /fields/create | 创建（默认 enabled=false） |
| POST /fields/detail | 详情（含 has_refs） |
| POST /fields/update | 编辑（乐观锁，必须先禁用） |
| POST /fields/delete | 软删除（必须先禁用 + 无引用） |
| POST /fields/check-name | 标识唯一性校验 |
| POST /fields/toggle-enabled | 启用/禁用切换 |
| POST /fields/references | 引用详情（模板 + 字段 + FSM） |
