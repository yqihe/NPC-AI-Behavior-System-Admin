# 事件扩展字段 Schema 管理 — 功能定义

> **实现状态**：已完成（后端 + 前端）。
> **归属**：属于事件类型模块的子功能，后端 API 路径 `/api/v1/event-type-schema/*`。

---

## 概述

让策划在 UI 上定义事件类型的可选附加字段，无需修改代码。每个 Schema 定义一个字段的标识、类型、约束和默认值，创建事件类型时表单自动渲染这些扩展字段。

## 支持的字段类型

| 类型 | 中文标签 | 约束 | 默认值 |
|------|----------|------|--------|
| `int` | 整数 | min / max / step | 数值 |
| `float` | 浮点数 | min / max / step | 数值 |
| `string` | 文本 | minLength / maxLength / pattern | 字符串 |
| `bool` | 布尔 | 无 | true/false |
| `select` | 选择 | options / minSelect / maxSelect | 选项值 |

不支持 `reference` 类型。

## 状态模型

创建后默认**启用**。必须先禁用才能编辑或删除。field_name / field_type 创建后不可变。

## 核心功能

1. **列表**：全量展示（无分页），默认按 ID 倒序，支持切换为按 sort_order 正序（排序切换按钮），支持启用状态筛选
2. **创建**：field_name + field_label + field_type + 约束 + 默认值 + sort_order
3. **编辑**：field_name / field_type 不可变，其余可改，乐观锁
4. **查看**：只读详情页
5. **启用/禁用**：toggle + 确认弹窗，启用态走 EnabledGuardDialog
6. **删除**：须先禁用，软删除，field_name 不可复用
7. **与事件类型集成**：创建事件类型时渲染启用的 Schema；详情页保留禁用但有值的 Schema（灰显 + 已禁用 tag）
