# fsm-state-dict-backend — 任务列表

## 状态

- [x] T1: errcode + model
- [x] T2: config + migration
- [x] T3: Redis cache（keys + cache）
- [x] T4: MySQL store（fsm_state_dict + fsm_config 新增方法）
- [x] T5: Service
- [ ] T6: Handler
- [ ] T7: Router + setup 装配
- [ ] T8: Seed 数据

---

## T1：errcode + model (R3, R4, R5)

**涉及文件**：
- `backend/internal/errcode/codes.go`
- `backend/internal/model/fsm_state_dict.go`（新增）

**做什么**：

1. **`errcode/codes.go`**：在 `ErrFsmConfigRefDelete = 43012` 之后新增：
   ```go
   ErrFsmStateDictNameExists        = 43013
   ErrFsmStateDictNameInvalid       = 43014
   ErrFsmStateDictNotFound          = 43015
   ErrFsmStateDictDeleteNotDisabled = 43016
   ErrFsmStateDictVersionConflict   = 43017
   ErrFsmStateDictInUse             = 43020
   // 43018-43019、43021-43024 预留
   ```
   在 `errMessages` map 中补充对应中文描述。

2. **`model/fsm_state_dict.go`**（新建）：定义以下结构体：
   - `FsmStateDict`（完整 DB 行）
   - `FsmStateDictListItem`（列表 item，仅含 id/name/display_name/category/enabled/created_at）
   - `FsmStateDictListQuery`（列表查询参数：name/category/enabled/*bool/page/page_size）
   - `CreateFsmStateDictRequest`（name/display_name/category/description）
   - `CreateFsmStateDictResponse`（id/name）
   - `UpdateFsmStateDictRequest`（id/display_name/category/description/version）
   - `FsmStateDictDeleteResult`（id/name/display_name/referenced_by []FsmConfigRef）
   - `FsmConfigRef`（id/name/display_name/enabled）
   - `FsmStateDictListData`（items []FsmStateDictListItem/total/page/page_size）

**做完是什么样**：`go build ./...` 通过；`errcode.ErrFsmStateDictInUse` 可引用。

---

## T2：config + migration (R11-R14)

**涉及文件**：
- `backend/internal/config/config.go`
- `backend/migrations/008_create_fsm_state_dicts.sql`（新增）

**做什么**：

1. **`config.go`**：在 `Config` 结构体和 `FsmConfig FsmConfigConfig` 字段之后新增：
   ```go
   FsmStateDict FsmStateDictConfig `yaml:"fsm_state_dict"`
   ```
   并定义结构体：
   ```go
   type FsmStateDictConfig struct {
       NameMaxLength        int `yaml:"name_max_length"`
       DisplayNameMaxLength int `yaml:"display_name_max_length"`
       CategoryMaxLength    int `yaml:"category_max_length"`
       DescriptionMaxLength int `yaml:"description_max_length"`
   }
   ```
   在 `defaultConfig` 中填充默认值：`64 / 128 / 64 / 512`。

2. **`008_create_fsm_state_dicts.sql`**（新建）：按 design.md 中的 DDL 建表，包含 `UNIQUE KEY uk_name (name)`、`INDEX idx_list`、`INDEX idx_category`。

**做完是什么样**：`go build ./...` 通过；SQL 文件语法正确（可 dry-run）。

---

## T3：Redis cache（keys + cache）(R26-R29)

**涉及文件**：
- `backend/internal/store/redis/shared/keys.go`
- `backend/internal/store/redis/fsm_state_dict_cache.go`（新增）

**做什么**：

1. **`keys.go`**：追加：
   ```go
   const FsmStateDictListVersionKey = "fsm_state_dicts:list:version"

   func FsmStateDictListKey(version int64, name, category string, enabled *bool, page, pageSize int) string
   func FsmStateDictDetailKey(id int64) string
   func FsmStateDictLockKey(id int64) string
   ```

2. **`fsm_state_dict_cache.go`**（新建）：克隆 `fsm_config_cache.go` 结构，实现 `FsmStateDictCache`，包含：
   - `GetList / SetList / InvalidateList`（版本号方案）
   - `GetDetail / SetDetail / DelDetail`（Cache-Aside + NullMarker）
   - `TryLock / Unlock`（分布式锁防击穿）
   - 泛型化 key 传入（与 FsmConfigCache 相同 pattern）

**做完是什么样**：`go build ./...` 通过；`FsmStateDictCache` 可被 service 引用。

---

## T4：MySQL store (R6-R9, R17)

**涉及文件**：
- `backend/internal/store/mysql/fsm_state_dict.go`（新增）
- `backend/internal/store/mysql/fsm_config.go`（新增方法）

**做什么**：

1. **`fsm_state_dict.go`**（新建）：实现 `FsmStateDictStore`，方法：
   - `DB() *sqlx.DB`
   - `Create(ctx, req) (int64, error)`
   - `GetByID(ctx, id) (*FsmStateDict, error)`
   - `ExistsByName(ctx, name) (bool, error)`
   - `List(ctx, q) ([]FsmStateDictListItem, int64, error)`（支持 name 模糊 + category + enabled 筛选，返回带 total）
   - `Update(ctx, req) error`（乐观锁 WHERE version=?）
   - `SoftDelete(ctx, id) error`（rows==0 返回 ErrNotFound）
   - `ToggleEnabled(ctx, id, enabled bool, version int) error`
   - `ListCategories(ctx) ([]string, error)`（DISTINCT category WHERE deleted=0）

2. **`fsm_config.go`**（追加 1 个方法）：
   ```go
   func (s *FsmConfigStore) ListFsmConfigsReferencingState(ctx context.Context, stateName string, limit int) ([]model.FsmConfigRef, error)
   ```
   SQL：`JSON_SEARCH(config_json, 'one', ?, NULL, '$.states[*].name') IS NOT NULL LIMIT ?`

**做完是什么样**：`go build ./...` 通过；`FsmStateDictStore` 和 `ListFsmConfigsReferencingState` 可被 service 引用。

---

## T5：Service (R5, R17-R22, R26-R29)

**涉及文件**：
- `backend/internal/service/fsm_state_dict.go`（新增）

**做什么**：

克隆 `fsm_config` service 结构，实现 `FsmStateDictService`：
- `NewFsmStateDictService(store, fsmConfigStore, cache, cfg)`
- `List(ctx, q)` — Redis 版本号列表缓存 + 降级
- `GetByID(ctx, id)` — Redis 详情缓存 + TryLock 防击穿 + 降级
- `Create(ctx, req)` — ExistsByName + Store.Create + InvalidateList + cache.Reload
- `Update(ctx, req)` — getOrNotFound + ValidateUpdate + Store.Update + DelDetail + InvalidateList
- `Delete(ctx, id)` — getOrNotFound + enabled 检查 + `fsmConfigStore.ListFsmConfigsReferencingState(ctx, dict.Name, 20)` → 被引用时返回 `(&FsmStateDictDeleteResult{ReferencedBy: refs}, ErrFsmStateDictInUse)` + SoftDelete + DelDetail + InvalidateList
- `CheckName(ctx, name)` — ExistsByName
- `ToggleEnabled(ctx, id, version)` — getOrNotFound + Store.ToggleEnabled + DelDetail + InvalidateList
- `ListCategories(ctx)` — 直查 MySQL（不缓存）

**做完是什么样**：`go build ./...` 通过；Delete 被引用时返回 `(result, error)` 均非 nil。

---

## T6：Handler (R1-R4, R11-R16)

**涉及文件**：
- `backend/internal/handler/fsm_state_dict.go`（新增）

**做什么**：

实现 `FsmStateDictHandler`，8 个方法，严格对齐 `FsmConfigHandler`：
- `List(ctx, *FsmStateDictListQuery) (*FsmStateDictListData, error)`
- `Create(ctx, *CreateFsmStateDictRequest) (*CreateFsmStateDictResponse, error)` — 含 `shared.CheckName` + `shared.CheckLabel`（display_name）+ category/description 长度校验
- `Get(ctx, *IDRequest) (*FsmStateDict, error)`
- `Update(ctx, *UpdateFsmStateDictRequest) (*string, error)` — `shared.SuccessMsg("保存成功")`
- `Delete(ctx, *IDRequest) (*FsmStateDictDeleteResult, error)`
- `CheckName(ctx, *CheckNameRequest) (*CheckNameResult, error)`
- `ToggleEnabled(ctx, *ToggleEnabledRequest) (*string, error)` — `shared.SuccessMsg("操作成功")`
- `ListCategories(ctx, *struct{}) (*[]string, error)`

校验逻辑：
- `name`：`shared.CheckName(req.Name, cfg.NameMaxLength, ErrFsmStateDictNameInvalid, "状态标识")`
- `display_name`：`shared.CheckLabel(req.DisplayName, cfg.DisplayNameMaxLength, "状态中文名")`
- `category`：非空 + `utf8.RuneCountInString(req.Category) <= cfg.CategoryMaxLength`
- `description`：仅长度上限，允许空

**做完是什么样**：`go build ./...` 通过；Handler 8 个方法签名正确。

---

## T7：Router + setup 装配 (R1, R2)

**涉及文件**：
- `backend/internal/router/router.go`
- `backend/internal/setup/stores.go`
- `backend/internal/setup/caches.go`（或 services.go/handlers.go）

**做什么**：

1. **`stores.go`**：追加 `FsmStateDict *storemysql.FsmStateDictStore` + 初始化
2. **`caches.go`**：追加 `FsmStateDict *storeredis.FsmStateDictCache` + 初始化
3. **`services.go`**：追加 `FsmStateDict *service.FsmStateDictService` + 初始化（注入 stores.FsmStateDict + stores.FsmConfig + caches.FsmStateDict + cfg.FsmStateDict）
4. **`handlers.go`**：追加 `FsmStateDict *handler.FsmStateDictHandler` + 初始化
5. **`router.go`**：在 fsm_configs 路由组之后新增：
   ```go
   fsmStateDicts := v1.Group("/fsm-state-dicts")
   {
       fsmStateDicts.POST("/list", handler.WrapCtx(h.FsmStateDict.List))
       fsmStateDicts.POST("/create", handler.WrapCtx(h.FsmStateDict.Create))
       fsmStateDicts.POST("/detail", handler.WrapCtx(h.FsmStateDict.Get))
       fsmStateDicts.POST("/update", handler.WrapCtx(h.FsmStateDict.Update))
       fsmStateDicts.POST("/delete", handler.WrapCtx(h.FsmStateDict.Delete))
       fsmStateDicts.POST("/check-name", handler.WrapCtx(h.FsmStateDict.CheckName))
       fsmStateDicts.POST("/toggle-enabled", handler.WrapCtx(h.FsmStateDict.ToggleEnabled))
       fsmStateDicts.POST("/list-categories", handler.WrapCtx(h.FsmStateDict.ListCategories))
   }
   ```

注意：setup 文件分 stores/caches/services/handlers 4 个，每个各改 1 处，加 router.go 共 5 个文件——超出 3 文件上限。拆分为：
- **T7a**（本任务）：stores.go + caches.go + services.go
- **T7b**（下一任务）：handlers.go + router.go

但考虑改动极小（每文件各追加 2-3 行），允许合并在一个 task 内完成，由执行者判断。

**做完是什么样**：`go build ./...` 通过；`curl POST /api/v1/fsm-state-dicts/list` 返回 `{code:0}` 而非 404。

---

## T8：Seed 数据 (R23-R25)

**涉及文件**：
- `backend/cmd/seed/main.go`

**做什么**：

在现有 seed 逻辑之后追加 `seedFsmStateDicts(ctx, db)` 函数，预置 30+ 条状态：

分类（按 category）：
- **通用**（4条）：idle / moving / interacting / busy
- **战斗**（11条）：alert / engage / attack_melee / attack_ranged / cast_spell / dodge / stagger / dying / dead / flee / revive
- **移动**（6条）：patrol / wander / chase / return_home / follow / escort
- **社交**（5条）：greet / talk / trade / quest_offer / farewell
- **活动**（5条）：sleep / eat / sit / craft / gather

实现要求：
- 幂等：`INSERT IGNORE INTO fsm_state_dicts (name, display_name, category, ...) VALUES (...)` 或逐条 `ExistsByName` 跳过
- `enabled=1, version=1, deleted=0`
- 调用时机：在主 seed 函数末尾调用

**做完是什么样**：seed 脚本执行两次不报错，第二次输出"已存在，跳过"；数据库 `fsm_state_dicts` 含 31 条记录。

---

## 执行顺序

T1 → T2 → T3 → T4 → T5 → T6 → T7 → T8

T1 必须最先（后续 task 依赖 errcode 和 model）。
T3 可与 T2 并行，但为简单起见建议顺序执行。
T5 依赖 T3/T4；T6 依赖 T5；T7 依赖 T6；T8 独立但需服务启动才能验证。
