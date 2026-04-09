# 后端架构禁止红线（ADMIN 运营平台）

仅含 ADMIN 后端架构专属禁令。通用禁令见 `../standards/red-lines.md`，Go 见 `../standards/go-red-lines.md`，MySQL 见 `../standards/mysql-red-lines.md`，Redis 见 `../standards/redis-red-lines.md`，缓存见 `../standards/cache-red-lines.md`。

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
