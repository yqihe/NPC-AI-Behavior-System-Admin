# UI/UX 禁止红线（ADMIN 运营平台）

面向策划/运营的非技术用户，所有界面必须对非技术用户友好。

## 禁止暴露技术细节给策划

- **禁止**在 UI 中展示原始 BB Key 名称（如 `threat_level`）。必须用中文标签（如"威胁等级"）
- **禁止**让策划手写 JSON。所有配置通过表单组件输入
- **禁止**让策划看到报错堆栈或 Go error 信息。错误提示必须是中文描述
- **禁止**表单只显示技术英文标签。所有字段下方必须有灰色提示文字，用自然语言解释
- **禁止**节点类型只显示英文。用中文标签 + 英文括注，如"顺序执行 (sequence)"

## 禁止表单对非技术用户不友好

- **禁止**下拉框依赖项为空时无引导。必须显示警告 + 跳转链接
- **禁止**列表页空数据时只显示空白表格。用 `el-empty` + 引导按钮
- **禁止**删除确认只写"确认删除？"。必须明确对象名和影响
- **禁止**NPC 表单保存时不检查行为树绑定完整性
- **禁止**新建时不在 blur 时检查名称重复
- **禁止**启用/禁用操作不弹确认弹窗。启用需说明「启用后可被引用」，禁用需说明「已有引用不受影响」
- **禁止**Toggle/编辑/删除等需要乐观锁的操作直接用列表数据的 version。列表接口可能不返回 version，必须先获取详情再操作

## 禁止危险操作引导不一致

- **禁止**对「启用状态下的危险操作拦截」用 `ElMessageBox.alert` 简陋单行提示。所有字段/模板/NPC/状态机/行为树的「启用中禁止编辑/删除」场景必须走统一的 `EnabledGuardDialog` 组件，视觉基线按 mockup `5aRMF` / `ka8Xu`：24×24 橙色圆角小图标 header + 加粗 lead 句 + 灰色 reason 段 + 灰底 `#F5F7FA` 前置条件/步骤区 + 「知道了」outline 按钮 + 「立即停用」橙底主按钮（带 SwitchButton 图标）
- **禁止**每个列表页自己写守卫弹窗的私有副本。`EnabledGuardDialog` 必须做成泛型组件，通过 `entityType: 'field' | 'template' | 'npc' | ...` 切换文案和 API 调度。新增一种配置类型时只需在 `open({action, entityType, entity})` 加一个 case，不需要新增组件
- **禁止**「立即停用」之后直接触发删除。edit 场景跳编辑页没问题；delete 场景 **只能停用 + 刷新列表让用户再点一次删除**，不能连锁触发删除以防误操作

## 禁止侧栏多级用不可折叠容器

- **禁止**用 `el-menu-item-group` 给菜单做多级结构。`el-menu-item-group` 只是静态分组标题 + 子项容器，不支持点击折叠。多级菜单必须用 `el-sub-menu`（原生支持折叠箭头、`default-openeds` 初始展开），一级分组标题用 `#title` slot 定义大号加粗字样（15px/600），二级项用 `el-menu-item` 缩进（`padding-left: 44px`）展示
- **禁止**sidebar 深色系下只给 `.is-active` 设蓝底，忽略 `:hover` 态。深色 sidebar 必须同时定义 `:deep(.el-menu-item:hover)` 和 `:deep(.el-sub-menu__title:hover)` 的背景色（如 `#1F2D3D`），否则 hover 时视觉无反馈

## 禁止表格排序按钮用 el-button text + Unicode 箭头

- **禁止**已选字段配置、字段优先级等 table 内行排序按钮用 `el-button text` 包 Unicode `↑` `↓`。视觉太粗 + 带按钮 padding + 不统一。必须用纯 `el-icon` 包 `ArrowUp` / `ArrowDown`，禁用态 `#C0C4CC` 灰、可点态 `#409EFF` 蓝、hover 态浅蓝底 `#ECF5FF`，两按钮 gap 14，容器 width 90 居中对齐（对齐 mockup `oE1Hj` / `ylI4t`）
