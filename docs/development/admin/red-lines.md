# ADMIN 项目专属禁止红线

通用禁令见 `../standards/red-lines/`。

## 禁止破坏游戏服务端数据格式

- **禁止**修改游戏配置集合的 MongoDB 文档结构（`{name, config}` 格式由游戏服务端定义）。ADMIN 元数据集合（`component_schemas`、`npc_presets`）不受此限制
- **禁止**在 config 字段中添加游戏服务端不认识的字段。运营平台私有数据用独立 collection
- **禁止**校验用结构体字段类型与游戏服务端不一致（如 `default_severity` 必须 `float64` 不能 `int`）
- **禁止**将装饰节点（`inverter`）归类为复合节点。装饰用 `child`（单对象），复合用 `children`（数组）
- **禁止**放行游戏服务端不支持的枚举值（`op`、`policy`、`result`）。无效枚举在服务端静默降级，极难排查
- **禁止**写入不属于当前 NPC 模板的 Blackboard Key。BB Key 白名单由组件 schema 的 `blackboard_keys` 字段定义，BT 编辑器只允许选择当前 NPC 模板已启用组件声明的 keys

## 禁止引用完整性破坏

- **禁止**删除正被 NPC 类型引用的 FSM/BT 配置
- **禁止**创建 NPC 类型时引用不存在的 FSM 或 BT
- **禁止**联调时先更新引用方（NPC type）再创建被引用项（BT tree）——会被校验拦截

## 禁止绕过 REST API

- **禁止**用 mongosh 或脚本直接写 MongoDB。所有数据变更必须通过 REST API，保证缓存同步
- **禁止**联调时只修改 `configs/` 本地文件就回复 READY。`configs/` 是参考，API 才写入 MongoDB

## 禁止硬编码

- **禁止**在业务代码中直接写错误码数字。错误码统一定义在 `errcode/codes.go`，调用处引用常量
- **禁止**在业务代码中直接写错误消息字符串。默认消息在 `errcode/codes.go` 的 messages map 中管理
- **禁止**在代码中硬编码数据库连接字符串、端口号、连接池参数。全部写入 `config.yaml`，环境变量可覆盖
- **禁止**在业务代码中直接拼 Redis key 字符串。key 前缀和生成规则统一定义在 `store/redis/keys.go`
- **禁止**在代码中硬编码分页默认值、字段长度限制等可配置参数。统一在 `config.yaml` 中管理
- **禁止**在代码中硬编码引用类型字符串（如 `"template"`、`"field"`）。使用 `model.RefTypeTemplate` / `model.RefTypeField` 常量
- **禁止**在代码中硬编码字典组名字符串（如 `"field_type"`）。使用 `model.DictGroupFieldType` 等常量
- **禁止** handler 层校验使用错误的错误码。name 校验用 `ErrFieldNameInvalid`，label/其他用 `ErrBadRequest`，不混用

## 禁止 ADMIN 过度设计

- **禁止**实现用户认证/权限系统（毕设阶段所有用户等权）
- **禁止**实现配置版本控制/回滚（Git 已有版本控制）
- **禁止**实现实时协作编辑（单人编辑足够）
- **禁止**实现工作流审批（保存即生效）

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

## 禁止偏离已建立的跨模块代码模式

- **禁止**新模块 handler 的 Update/Delete/ToggleEnabled 返回 `*model.Empty{}`。必须与 Field/Template 一致：Update → `*util.SuccessMsg("保存成功")`、Delete → `*DeleteResult{ID, Name, Label}`、ToggleEnabled → `*util.SuccessMsg("操作成功")`
- **禁止**新模块 service 的 `ToggleEnabled` 使用 `(ctx, id, version)` 签名自行取反 `!et.Enabled`。必须接收 `*model.ToggleEnabledRequest`（调用方指定目标 `enabled` 状态）
- **禁止**新模块 service 缓存读取用 `_, hit, _ := cache.GetDetail(...)` 丢弃 error。必须用 `err == nil && hit` 模式（Redis 错误降级直查 MySQL，不误判为缓存命中）
- **禁止**新模块 service 对 store 错误直接 `return err` 不包装。必须 `slog.Error` + `fmt.Errorf("xxx: %w", err)` 对齐 Field/Template
- **禁止**新模块 store 的 Create/Update 使用展开的位置参数（如 ~~`Create(ctx, name, displayName, mode string, ...)`~~）。必须用 `*model.CreateXxxRequest` 结构体
- **禁止**新模块 handler 自定义 ID/Version/Required 校验逻辑。必须调 `util.CheckID()` / `util.CheckVersion()` / `util.CheckRequired()`
- **禁止**新模块 handler 在校验**之前**打 slog Debug 日志。日志必须在校验通过后打印
- **禁止**新模块前端 API 文件重复定义 `ListData<T>` / `CheckNameResult`。必须从 `fields.ts` 导入
- **禁止**新模块前端表单用 `detail.value!.xxx` 非空断言读取服务端数据。必须用独立 `ref()` 存储

## 禁止文件职责混放

- **禁止**在业务逻辑文件中定义跨模块共享的常量、工具函数、初始化代码。共享常量和工具放 `util/`，初始化聚合放 `setup/`，错误定义放 `errcode/`
- **禁止**在同一 store 文件中既放业务 CRUD 又放共享工具（如 `escapeLike` 只定义一次却被 4 个 store 使用）。跨文件共享的工具必须放 `util/` 包
- **禁止** store/redis 业务 cache 文件中定义 key 前缀、TTL 常量、key 生成函数。这些统一放 `store/redis/config/` 子包
- **禁止**同一 store 用 interface 类型接收 `db` 而其他 store 用 `*sqlx.DB`。全部统一 `*sqlx.DB`
- **禁止**新模块 redis cache 文件命名不带 `_cache` 后缀。统一 `{module}_cache.go`（如 `field_cache.go`、`fsm_config_cache.go`）

## 禁止表格排序按钮用 el-button text + Unicode 箭头

- **禁止**已选字段配置、字段优先级等 table 内行排序按钮用 `el-button text` 包 Unicode `↑` `↓`。视觉太粗 + 带按钮 padding + 不统一。必须用纯 `el-icon` 包 `ArrowUp` / `ArrowDown`，禁用态 `#C0C4CC` 灰、可点态 `#409EFF` 蓝、hover 态浅蓝底 `#ECF5FF`，两按钮 gap 14，容器 width 90 居中对齐（对齐 mockup `oE1Hj` / `ylI4t`）
