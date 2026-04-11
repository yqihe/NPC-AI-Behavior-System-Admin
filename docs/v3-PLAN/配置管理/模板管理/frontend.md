# 模板管理 — frontend

> 实现状态：全部完成。spec 详见 [`docs/specs/field-template-frontend-sync`](../../../specs/field-template-frontend-sync)。

## 文件清单

```
frontend/src/
├── api/templates.ts                         # 类型定义 + TEMPLATE_ERR(41001-41012) + TEMPLATE_ERR_MSG + 8 个 API 函数
├── views/
│   ├── TemplateList.vue                     # 列表页
│   └── TemplateForm.vue                     # 新建/编辑共用（mode prop 切换）
└── components/
    ├── TemplateFieldPicker.vue              # 字段选择卡（按 category 分组 + 3 列网格 + reference popover 触发）
    ├── TemplateRefPopover.vue               # reference 子字段勾选弹层
    ├── TemplateSelectedFields.vue           # 已选字段配置卡（必填 / 排序 / 停用字段标灰警告）
    ├── TemplateReferencesDialog.vue         # 引用详情弹窗（NPC 未上线时空占位）
    └── EnabledGuardDialog.vue               # 启用守卫弹窗（编辑/删除两个场景复用）
```

挂载点：

- `router/index.ts` — 追加 `/templates`、`/templates/create`、`/templates/:id/edit` 三条路由
- `components/AppLayout.vue` — 菜单「字段管理」下方追加「模板管理」项

## 组件树

```
TemplateList.vue
  ├─ EnabledGuardDialog.vue       (启用守卫，编辑/删除两条路径共用)
  └─ TemplateReferencesDialog.vue (引用详情)

TemplateForm.vue (mode: 'create' | 'edit')
  ├─ TemplateFieldPicker.vue
  │   └─ TemplateRefPopover.vue
  └─ TemplateSelectedFields.vue
```

依赖方向单向向下：`views → components → api → request`。Service 层无，错误码走 `TEMPLATE_ERR` 常量。

## 关键状态流（`TemplateForm.vue`）

```
selectedIds: number[]                  ← 扁平的 field_id 数组，顺序即 templates.fields JSON 顺序
requiredMap: Record<number, boolean>   ← 按 field_id 索引的必填配置
fieldPool:  FieldListItem[]            ← 启用字段池（用于 picker 分组）
template:   TemplateDetail | null      ← 编辑模式原始数据，提供 version / ref_count / 停用字段元数据
```

- `selectedFieldsView` computed：**编辑模式优先从 `template.fields` 拿元数据**（含停用字段 `enabled=false`），`create` 模式全从 `fieldPool` 构造。
- `TemplateFieldPicker` 通过 `defineModel<number[]>('selectedIds')` 双向绑定；普通字段点击切换，reference 字段点击弹 popover。
- `TemplateRefPopover` 内部 `tempSelected` 状态隔离，确认时 emit `{allSubIds, selectedSubIds}`；`TemplateFieldPicker` 按差集清理再合并，保证「取消勾选某几个子字段」的语义正确。
- `ref_count > 0` 锁定：顶部黄色警告条 + picker/selected disabled + 卡标题 `🔒 已锁定` tag；reference popover 仍可打开但只读浏览（`readonly` prop）。

## reference 字段在模板里的约束

- **reference 字段本身永远不写入 `req.fields`**，模板存的是展开后的扁平 leaf `field_id`；
- 模板创建/编辑时如果前端 bug 或 devtools 直连 API 试图写入 reference 字段，后端 `FieldService.ValidateFieldsForTemplate` 会拦截并返回 `41012 ErrTemplateFieldIsReference`；
- 前端 `TemplateForm.vue` 在 `onSubmit` 的错误分支里对 41012 做兜底提示「reference 字段必须先展开子字段再加入模板」，并重拉字段池。

## 停用字段的视觉标注

- `TemplateFieldItem.enabled = false`（字段当前被停用）在 `TemplateSelectedFields.vue` 中：
  - 整行 `opacity: 0.55`（通过 `:row-class-name` 返回 `row-field-disabled`）
  - 标签列左侧加 ⚠ 橙色警告图标 + 「已停用」tag
- picker 只展示启用字段池，所以停用字段不会出现在「字段选择卡」中——只会出现在「已选字段配置卡」（从 `template.fields` 元数据拼出），保留现有引用关系（存量不动）。

## 字段分类分组

- `TemplateFieldPicker` 的分组标题用 `fieldPool[*].category_label`（后端 Service 层用 `DictCache` 翻译后返回），**不调用字典 API**、**不硬编码**中文文案。
- 分组顺序按 `fieldPool` 中第一次出现 category 的顺序（= 字段列表接口的 id DESC 排序）。

## 错误码处理

| 错误码 | 处理 |
|---|---|
| 41001 `NAME_EXISTS` / 41002 `NAME_INVALID` | `nameStatus='taken'` + 红色提示 |
| 41003 `NOT_FOUND` | 跳回列表 |
| 41004 `NO_FIELDS` | 提交前前端已拦截（兜底） |
| 41005 `FIELD_DISABLED` / 41006 `FIELD_NOT_FOUND` | 重拉字段池 |
| 41007 `REF_DELETE` | 列表删除路径下自动打开引用详情弹窗 |
| 41008 `REF_EDIT_FIELDS` | UI 已禁用字段变更（ref_count>0 锁定）理论不到 |
| 41009 `DELETE_NOT_DISABLED` / 41010 `EDIT_NOT_DISABLED` | `EnabledGuardDialog` 列表前置拦截，理论不到 |
| 41011 `VERSION_CONFLICT` | `ElMessageBox.alert` + 跳回列表 |
| **41012 `FIELD_IS_REFERENCE`** | **兜底**：弹「reference 字段必须先展开子字段再加入模板」+ 重拉字段池 |

## e2e 验收（手动）

见 `docs/specs/field-template-frontend-sync/design.md` 「测试策略 / 手动 e2e」16 步脚本。
