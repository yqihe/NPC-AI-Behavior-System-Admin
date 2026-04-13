# ADMIN 项目专属禁止红线

通用禁令见 `../standards/red-lines/`。

## 1. 禁止破坏游戏服务端数据格式

1. 禁止修改游戏配置集合的 MongoDB 文档结构（`{name, config}` 格式由服务端定义）
2. 禁止在 config 字段中添加服务端不认识的字段，私有数据用独立 collection
3. 禁止校验结构体字段类型与服务端不一致（如 `default_severity` 必须 `float64` 不能 `int`）
4. 禁止将装饰节点（`inverter`）归类为复合节点。装饰用 `child`（单对象），复合用 `children`（数组）
5. 禁止放行服务端不支持的枚举值（`op`/`policy`/`result`），无效枚举在服务端静默降级极难排查
6. 禁止写入不属于当前 NPC 模板的 BB Key

## 2. 禁止引用完整性破坏

1. 禁止删除被引用的配置（字段被模板/字段/FSM 引用时不可删、扩展字段被事件类型引用时不可删）
2. 禁止创建时引用不存在的配置
3. 禁止联调时先更新引用方再创建被引用项（会被校验拦截）
4. 禁止取消字段 `expose_bb` 时不检查 FSM BB Key 引用
5. 禁止用冗余计数器（ref_count）替代关系表（field_refs/schema_refs）做引用追踪——引用关系的权威数据源是关系表
6. 禁止编辑被引用配置时随意修改类型或收紧约束（类型不可改，约束只能放宽）

## 3. 禁止绕过 REST API

1. 禁止用 mongosh 或脚本直接写 MongoDB，所有数据变更必须通过 REST API 保证缓存同步
2. 禁止联调时只修改 `configs/` 本地文件就回复 READY

## 4. 禁止硬编码

1. 错误码数字 → `errcode/codes.go` 常量
2. 错误消息字符串 → `errcode/codes.go` messages map
3. DB 连接串/端口/连接池 → `config.yaml`（环境变量可覆盖）
4. Redis key 拼接 → `store/redis/config/` 子包生成
5. 分页默认值/字段长度限制 → `config.yaml`
6. 引用类型字符串（`"template"`/`"field"`/`"fsm"`/`"event_type"`）→ `util.RefTypeXxx` 常量
7. 字典组名（`"field_type"`）→ `util.DictGroupXxx` 常量
8. handler 校验用错误码：name 校验用 `ErrXxxNameInvalid`，其他用 `ErrBadRequest`，不混用

## 5. 禁止 ADMIN 过度设计

禁止实现：用户认证/权限系统、配置版本回滚、实时协作编辑、工作流审批。

## 6. 禁止暴露技术细节给策划

1. UI 中 BB Key 必须用中文标签，不显示原始标识符
2. 所有配置通过表单组件输入，不让策划手写 JSON
3. 错误提示必须是中文描述，不暴露堆栈或 Go error
4. 表单字段下方必须有灰色提示文字解释用途
5. 节点类型用"中文标签 (english)"格式

## 7. 禁止表单对非技术用户不友好

1. 下拉依赖为空时显示警告 + 跳转链接
2. 列表空数据用 `el-empty` + 引导按钮
3. 删除确认必须明确对象名和影响
4. 启用/禁用必须弹确认弹窗，启用说"启用后可被引用"，禁用说"已有引用不受影响"
5. 需要乐观锁的操作必须先 detail 拿最新 version，不直接用列表行数据

## 8. 禁止危险操作引导不一致

1. 「启用中禁止编辑/删除」场景必须走统一 `EnabledGuardDialog` 组件（视觉基线：橙色圆角图标 header + 加粗 lead + 灰色 reason + 灰底前置条件区 + 「知道了」+ 「立即禁用」橙底按钮）
2. EnabledGuardDialog 做泛型，通过 `entityType` 切换文案和 API，不每页写私有副本
3. 「立即禁用」后 delete 场景只刷新列表让用户再点删除，禁止连锁触发删除

## 9. 禁止侧栏多级用不可折叠容器

1. 禁止用 `el-menu-item-group` 做多级菜单，必须用 `el-sub-menu`（原生折叠箭头 + `default-openeds`）
2. 深色 sidebar 必须同时定义 `:hover` 和 `.is-active` 背景色

## 10. 禁止偏离跨模块代码模式

1. handler：Update → `*SuccessMsg("保存成功")`、Delete → `*DeleteResult{ID,Name,Label}`、ToggleEnabled → `*SuccessMsg("操作成功")`
2. service：ToggleEnabled 接收 `*ToggleEnabledRequest`（调用方指定目标状态），不自行取反
3. service：缓存读取 `err == nil && hit`，禁止 `_, hit, _` 丢弃 error
4. service：store 错误必须 `slog.Error + fmt.Errorf("xxx: %w", err)`，禁止 raw return
5. store：Create/Update 用 `*model.XxxRequest` 结构体，禁止展开位置参数
6. handler：`util.CheckID/CheckVersion/CheckRequired` 校验，slog Debug 在校验后
7. 前端 API：`ListData<T>` / `CheckNameResult` 从 `fields.ts` 导入
8. 前端表单：用独立 `ref()` 存 version/refCount，禁止 `detail.value!.xxx` 非空断言

## 11. 禁止文件职责混放

1. 共享常量/工具函数 → `util/`，初始化聚合 → `setup/`，错误定义 → `errcode/`
2. 跨 store 共享工具（如 `EscapeLike`）→ `util/`，禁止在 store 文件内定义
3. Redis key/TTL/前缀 → `store/redis/config/` 子包
4. db 字段统一 `*sqlx.DB`，禁止 interface
5. Redis cache 文件命名 `{module}_cache.go`
6. **每层文件夹下不允许子文件夹**（`store/redis/config/` 例外）

## 12. 禁止 Element Plus 表单 disabled 被子组件覆盖

1. `el-form :disabled="true"` 内子组件 `:disabled` 必须写 `:disabled="isView || condition"`（Element Plus `??` 合并会被覆盖）
2. `el-link`/`el-icon @click` 不受 form disabled 注入，需 `v-if` 或单独控制

## 13. 禁止业务错误码漏处理

1. 表单提交 `.catch` 必须逐一处理 API 定义的每个错误码，不能只写通用兜底
2. 新增后端错误码必须在同一 PR 更新前端 catch 块
