# 状态机管理 — 任务拆解（后端）

> 对应设计：[design.md](./design.md)
> 对应需求：[requirements.md](./requirements.md)

---

## T1: DDL + 数据模型 (R4, R7, R9) [x]

**涉及文件**：
- `backend/migrations/006_create_fsm_configs.sql`（新增）
- `backend/internal/model/fsm_config.go`（新增）

**做完了是什么样**：
- `006_create_fsm_configs.sql` 可执行，表结构含 `uk_name`、`idx_list` 索引
- `model/fsm_config.go` 包含所有结构体：FsmConfig、FsmConfigListItem、FsmConfigListData、FsmConfigDetail、FsmConfigExportItem、FsmConfigListQuery、CreateFsmConfigRequest、UpdateFsmConfigRequest、FsmState、FsmTransition、FsmCondition
- `FsmConfigListData` 实现 `ToListData()` 方法

---

## T2: 错误码 + 配置项 (R3, R8, R10) [x]

**��及文��**：
- `backend/internal/errcode/codes.go`（改动）
- `backend/internal/config/config.go`��改动）

**做完了是什么样**：
- `errcode/codes.go` 新增 43001-43012 错误码常量 + messages 映射
- `config.go` 新增 `FsmConfigConfig` 结构体（name_max_length、display_name_max_length、max_states、max_transitions、condition_max_depth、cache TTL）
- `Config` 结构体新增 `FsmConfig FsmConfigConfig` 字段

---

## T3: Store 层 — MySQL CRUD (R1, R4, R5, R6, R7, R9, R10, R17, R18, R19) [x]

**涉及文件**：
- `backend/internal/store/mysql/fsm_config.go`（新增）

**做完了是什么样**：
- `FsmConfigStore` 实现：Create、GetByID、ExistsByName、List（分页+模糊搜索+enabled 筛选）、Update（乐观锁）、Delete（软删除+乐观锁）、ToggleEnabled（乐观锁）、ExportAll（enabled=1 AND deleted=0）
- List 查询 LIKE 使用 `escapeLike()` 转义
- Create/Update 接收 Request 结构体（不展开位置参数）

---

## T4: Cache 层 — Redis 缓存 (R20, R21, R22, R23, R24) [x]

**涉及文件**：
- `backend/internal/store/redis/fsm_config_cache.go`（新增）
- `backend/internal/store/redis/keys.go`（改动）

**做完了是什么样**：
- `keys.go` 新增 `fsm_configs` 前缀常量 + `FsmConfigListKey` / `FsmConfigDetailKey` / `FsmConfigLockKey` 函数
- `FsmConfigCache` 实现：GetDetail / SetDetail / DelDetail / TryLock / Unlock / GetList / SetList / IncrListVersion / GetListVersion
- 与 EventTypeCache 同构：空标记、TTL+jitter、分布式锁、版本号方案
- Redis 不可用时返回 error（由 service 层降级直查 MySQL）

---

## T5: Service �� — 业务逻辑 + 条件树校验 (R1, R2, R8, R10, R11, R12, R13, R14, R15, R16, R25, R26) [x]

**涉及文件**：
- `backend/internal/service/fsm_config.go`（新增）

**做完了是什么样**：
- `FsmConfigService` 实现：List、Create、GetByID、Update、Delete、ToggleEnabled、CheckName、ExportAll
- `validateConfig` 校验：states 非空、状态名非空且不重复、initial_state ∈ states、from/to ∈ states、priority ≥ 0、状态数/转换数不超上限
- `validateCondition(cond, depth)` 递归校验：叶/组合互斥、op 合法、深度 ≤ configurable max
- `buildConfigJSON` 组装 config_json
- 启用拦截：enabled=true 时拒绝 Update/Delete
- 乐观锁：version 不匹配返回 43011
- 缓存读取用 `err == nil && hit` 模式
- store 错误 `slog.Error` + `fmt.Errorf` 包装
- ToggleEnabled 接收 `*model.ToggleEnabledRequest`

---

## T6: Handler 层 — 7 个 CRUD 接口 (R1, R2, R6, R25, R26) [x]

**涉及文件**：
- `backend/internal/handler/fsm_config.go`（新增）

**做完了是什么样**：
- `FsmConfigHandler` 实现：List、Create、Get、Update、Delete、CheckName、ToggleEnabled
- 前置校验：name 格式（identPattern）+ 长度、displayName 非空 + 长度、调共享 `checkID()` / `checkVersion()`
- 校验通过后才打 slog.Debug
- Update 返回 `*string("保存成功")`、Delete 返回 `*DeleteResult`、ToggleEnabled 返回 `*string("操作成功")`

---

## T7: 导出 API + 路由注册 + 装配 (R17, R18, R19) [x]

**涉及文件**：
- `backend/internal/handler/export.go`（改动）
- `backend/internal/router/router.go`（改动）
- `backend/cmd/admin/main.go`（改动）

**做完了是什么样**：
- `export.go`：ExportHandler 新增 `fsmConfigService` 字段 + `FsmConfigs()` 方法
- `router.go`：Setup 函数签名新增 `fch *handler.FsmConfigHandler`，注册 7 个 CRUD 路由 + 1 个导出路由
- `main.go`：装配 FsmConfigStore → FsmConfigCache → FsmConfigService → FsmConfigHandler → ExportHandler，注入 router.Setup

---

## T8: config.yaml 更新 + 迁移执行验证 (R20, R23) [x]

**涉及文件**：
- `backend/config.yaml`（改动）
- `docker-compose.yml` 或手动执行迁移

**做完了是什么样**��
- `config.yaml` 新增 `fsm_config` 段，含所有配置项和合理默认值
- 迁移文件在 Docker MySQL 中执行成功
- 服务启动无报错

---

## T9: API 测试 (R1-R26)

**涉及文件**：
- `tests/api_test.sh`（改动，追加 FSM 段）

**做完了是什么样**：
- 正常 CRUD 全流程：create → check-name → detail → list → update → toggle-enabled → delete
- 校验拦截：name 重复(43001)、name 格式(43002)、states ���空(43004)、状态名重复(43005)、initial_state 非法(43006)、transition 引用非法状态(43007)、condition 非法(43008)、启用中编辑(43010)、启用中删除(43009)、版本冲突(43011)
- 导出 API：创建+启用后 GET `/api/configs/fsm_configs` 返回 `{items: [{name, config}]}`，格式与 API 契约 §5 一致
- 所有测试通过

---

## 依赖顺序

```
T1 (model + DDL)
T2 (errcode + config)
  ├─ T3 (store) ── 依赖 T1 model + T2 errcode
  ├─ T4 (cache) ── 依�� T1 model
  │
  └──► T5 (service) ── 依赖 T3 + T4 + T2
         └──► T6 (handler) ── 依赖 T5 + T2
                └──► T7 (export + router + main) ── 依赖 T6
                       └��─► T8 (config.yaml + 迁移) ── 依赖 T7
                              └──► T9 (API 测试) ���─ 依赖 T8
```

T1 和 T2 可并行；T3 和 T4 可并行；T5-T9 顺序依赖。
