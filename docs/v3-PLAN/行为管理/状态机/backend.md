# 状态机管理 — 后端设计

> 通用架构/规范见 `docs/architecture/overview.md` 和 `docs/development/`。
> 本文档只记录状态机管理模块的实现事实与特有约束。

---

## 1. 目录结构

```
backend/
  internal/
    handler/
      fsm_config.go                    # 状态机 CRUD + 列表 + 详情 + toggle + check-name
    service/
      fsm_config.go                    # 状态机业务逻辑（含配置完整性校验、条件树递归校验、config_json 拼装）
    store/
      mysql/
        fsm_config.go                  # fsm_configs 表 CRUD
      redis/
        fsm_config_cache.go            # 状态机 Redis 缓存（detail + list + 分布式锁）
        config/keys.go                 # key 前缀 & 构造函数（fsm_configs:detail/list/lock）
    model/
      fsm_config.go                    # FsmConfig / FsmConfigListItem / FsmConfigListData / FsmConfigDetail / FsmConfigExportItem / 请求体 / FsmState / FsmTransition / FsmCondition
    errcode/
      codes.go                         # 43001-43012 错误码
    router/
      router.go                        # /api/v1/fsm-configs/* + /api/configs/fsm_configs
  migrations/
    006_create_fsm_configs.sql
```

---

## 2. 数据表

### fsm_configs

```sql
CREATE TABLE IF NOT EXISTS fsm_configs (
    id              BIGINT AUTO_INCREMENT PRIMARY KEY,
    name            VARCHAR(64)  NOT NULL,              -- FSM 唯一标识（如 wolf_fsm），创建后不可变
    display_name    VARCHAR(128) NOT NULL,              -- 中文名（搜索用）
    config_json     JSON         NOT NULL,              -- {initial_state, states, transitions} 完整配置，导出 API 原样输出

    enabled         TINYINT(1)   NOT NULL DEFAULT 0,    -- 启用状态（创建默认 0，给"配置窗口期"）
    version         INT          NOT NULL DEFAULT 1,    -- 乐观锁
    created_at      DATETIME     NOT NULL,
    updated_at      DATETIME     NOT NULL,
    deleted         TINYINT(1)   NOT NULL DEFAULT 0,    -- 软删除

    -- name 唯一约束不含 deleted：软删后 name 仍占唯一性，不可复用
    UNIQUE KEY uk_name (name),
    -- 覆盖索引：列表分页查询（id DESC 排序，含 enabled 用于筛选）
    INDEX idx_list (deleted, enabled, id DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

**设计要点：**
- `config_json` 是 `{initial_state, states, transitions}` 的完整 JSON，导出 API 直接原样输出，不经过 Go struct 中转。Service 层创建/编辑时用 `buildConfigJSON()` 组装，列表时 unmarshal 抽展示字段。
- `uk_name` 不含 `deleted` 列：软删后 name 永久不可复用。`ExistsByName` 查询不带 `deleted` 过滤。
- `enabled` 默认 0：创建后给"配置窗口期"，编辑/删除要求先停用。
- `idx_list (deleted, enabled, id DESC)`：列表分页覆盖索引，支持 enabled 筛选 + id 倒序分页。

---

## 3. API 接口

### 状态机管理（7 个接口）

| 方法 | 路径 | Handler | 说明 |
|------|------|---------|------|
| POST | `/api/v1/fsm-configs/list` | `FsmConfigHandler.List` | 分页列表，支持 label 模糊搜索 + enabled 筛选 |
| POST | `/api/v1/fsm-configs/create` | `FsmConfigHandler.Create` | 创建状态机，校验 name/displayName + 配置完整性 |
| POST | `/api/v1/fsm-configs/detail` | `FsmConfigHandler.Get` | 详情，返回 config_json 展开 |
| POST | `/api/v1/fsm-configs/update` | `FsmConfigHandler.Update` | 编辑（必须先停用），乐观锁，name 不可变。返回 `"保存成功"` |
| POST | `/api/v1/fsm-configs/delete` | `FsmConfigHandler.Delete` | 软删除（必须先停用）。返回 `{id, name, label}` |
| POST | `/api/v1/fsm-configs/check-name` | `FsmConfigHandler.CheckName` | name 完整格式校验（正则+长度）+ 唯一性校验 |
| POST | `/api/v1/fsm-configs/toggle-enabled` | `FsmConfigHandler.ToggleEnabled` | 启用/停用切换（调用方指定目标状态 `enabled`），乐观锁。返回 `"操作成功"` |

**Handler 层校验（与 Field/Template/EventType 一致模式）：**
- `checkName(name)`：非空 + `util.IdentPattern`（`^[a-z][a-z0-9_]*$`）+ 长度 <= NameMaxLength
- `checkDisplayName(displayName)`：非空 + 字符数 <= DisplayNameMaxLength
- 统一使用共享 `util.CheckID()` / `util.CheckVersion()`
- slog Debug 日志在校验之后打印，格式为中文点分（如 `"handler.创建状态机"`）

**业务规则（Service 层）：**
- Create：name 唯一性检查（含软删除）→ `validateConfig` 配置完整性校验 → `buildConfigJSON` 组装 → store.Create(req, configJSON) → 清列表缓存
- Update：查存在性 → 必须已停用（43010）→ `validateConfig` → `buildConfigJSON` → store.Update(req, configJSON) 乐观锁 → 清详情+列表缓存
- Delete：查存在性 → 必须已停用（43009）→ 软删除 → 清缓存。返回 `*model.DeleteResult{ID, Name, Label(=DisplayName)}`
- ToggleEnabled：接收 `*model.ToggleEnabledRequest`（调用方指定目标 `enabled` 状态，幂等安全），乐观锁更新 → 清缓存
- GetByID：Cache-Aside + 分布式锁防击穿 + 空标记防穿透。缓存错误处理使用 `err == nil && hit` 模式（Redis 错误降级直查 MySQL）
- CheckName：成功时返回 `{available: true, message: "该标识可用"}`
- 所有 store 错误统一 `slog.Error` + `fmt.Errorf("xxx: %w", err)` 包装

**Store 参数风格（与 Field/Template/EventType 一致）：**
- `Create(ctx, *model.CreateFsmConfigRequest, configJSON)` — 用请求结构体
- `Update(ctx, *model.UpdateFsmConfigRequest, configJSON)` — 同上

### 导出接口

| 方法 | 路径 | Handler | 说明 |
|------|------|---------|------|
| GET | `/api/configs/fsm_configs` | `ExportHandler.FsmConfigs` | 导出所有已启用状态机，`{items: [{name, config}]}` |

导出查询：`SELECT name, config_json AS config FROM fsm_configs WHERE deleted = 0 AND enabled = 1 ORDER BY id`，config_json 原样输出。

---

## 4. 缓存策略

### fsm_configs 缓存（Redis）

| Key 模式 | 含义 | TTL |
|----------|------|-----|
| `fsm_configs:detail:{id}` | 单条详情（含空标记防穿透） | 5min + 30s jitter |
| `fsm_configs:list:v{ver}:{label}:{enabled}:{page}:{pageSize}` | 列表分页缓存（带版本号） | 1min + 10s jitter |
| `fsm_configs:list:version` | 列表缓存版本号（INCR 使旧 key 自然过期） | 永久 |
| `fsm_configs:lock:{id}` | 分布式锁（SETNX 防缓存击穿） | 3s（可配置） |

**失效规则：**
- 单条写操作（Create / Update / Delete / ToggleEnabled）：`DEL fsm_configs:detail:{id}` + `INCR fsm_configs:list:version`
- 列表失效采用版本号递增方式，禁止 SCAN+DEL

**详情读取流程（Cache-Aside + 分布式锁 + 空标记，与 Field/Template/EventType 完全一致）：**
1. 查 Redis 缓存：`err == nil && hit` 才使用缓存结果（Redis 错误降级直查 MySQL）
2. 未命中 → SETNX 获取分布式锁（锁失败 `slog.Warn` 记录后继续）→ double-check 缓存
3. 查 MySQL → 写缓存（nil 写空标记防穿透）

---

## 5. 错误码

### 状态机管理（43001-43012）

| 错误码 | 常量 | 触发场景 |
|--------|------|----------|
| 43001 | `ErrFsmConfigNameExists` | 创建时 name 已存在（含软删除） |
| 43002 | `ErrFsmConfigNameInvalid` | name 为空 / 不匹配 `^[a-z][a-z0-9_]*$` / 超长 |
| 43003 | `ErrFsmConfigNotFound` | ID 对应记录不存在或已软删 |
| 43004 | `ErrFsmConfigStatesEmpty` | 未定义任何状态 / 状态数超上限 |
| 43005 | `ErrFsmConfigStateNameInvalid` | 状态名为空或重复 |
| 43006 | `ErrFsmConfigInitialInvalid` | 初始状态不在状态列表中 |
| 43007 | `ErrFsmConfigTransitionInvalid` | 转换规则引用了不存在的状态 / priority < 0 / 转换数超上限 |
| 43008 | `ErrFsmConfigConditionInvalid` | 条件表达式不合法（嵌套超深 / 叶组合混用 / 非法操作符 / value+ref_key 冲突） |
| 43009 | `ErrFsmConfigDeleteNotDisabled` | 删除前必须先停用 |
| 43010 | `ErrFsmConfigEditNotDisabled` | 编辑前必须先停用 |
| 43011 | `ErrFsmConfigVersionConflict` | 版本冲突（乐观锁，编辑 / toggle） |
| 43012 | `ErrFsmConfigRefDelete` | 被 NPC 引用，无法删除（占位，本期 ref_count 恒 0） |

---

## 6. 配置项

`config.yaml` 中 `fsm_config` 段：

```yaml
fsm_config:
  name_max_length: 64            # name 最大字符长度
  display_name_max_length: 128   # display_name 最大字符长度（按 rune 计）
  max_states: 50                 # 单个 FSM 最大状态数
  max_transitions: 200           # 单个 FSM 最大转换规则数
  condition_max_depth: 10        # 条件树最大嵌套深度
  cache_detail_ttl: 10m          # 详情缓存 TTL（base，实际加 jitter）
  cache_list_ttl: 5m             # 列表缓存 TTL（base，实际加 jitter）
  cache_lock_ttl: 3s             # 分布式锁 TTL
```

Go 结构体 `config.FsmConfigConfig`：

```go
type FsmConfigConfig struct {
    NameMaxLength        int           `yaml:"name_max_length"`
    DisplayNameMaxLength int           `yaml:"display_name_max_length"`
    MaxStates            int           `yaml:"max_states"`
    MaxTransitions       int           `yaml:"max_transitions"`
    ConditionMaxDepth    int           `yaml:"condition_max_depth"`
    CacheDetailTTL       time.Duration `yaml:"cache_detail_ttl"`
    CacheListTTL         time.Duration `yaml:"cache_list_ttl"`
    CacheLockTTL         time.Duration `yaml:"cache_lock_ttl"`
}
```

Handler 层使用 `NameMaxLength` / `DisplayNameMaxLength` 做格式校验；Service 层使用 `MaxStates` / `MaxTransitions` / `ConditionMaxDepth` 做业务上限校验；Cache 层使用 `CacheLockTTL` 做分布式锁超时。
