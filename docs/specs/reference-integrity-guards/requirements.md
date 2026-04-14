# reference-integrity-guards — 需求分析

## 动机

引用完整性保护是运营平台的核心安全约束之一。当前代码中存在 TOCTOU（检查后操作，Time-of-check-to-time-of-use）漏洞：

- **EventTypeSchema.Delete**：先 `HasRefs`（无锁），后 `SoftDelete`，中间窗口内并发事件类型可以写入 `schema_refs`，导致删除一个仍被引用的扩展字段定义。
- **Field.Update 类型变更检查**：先 `HasRefs`（无锁）判断"是否允许改 type"，后 `store.Update`，中间窗口内并发模板/字段可以写入 `field_refs`，导致已被引用的字段类型被更改。
- **EventTypeSchema.Update 约束收紧检查**：先 `HasRefs`（无锁）判断"是否需要检查约束收紧"，后 `store.Update`，中间窗口内并发事件类型可以写入 `schema_refs`，导致已被引用的扩展字段约束被收紧（现有值可能不再合法）。

不修则：在多用户并发操作场景（如两个策划同时操作）下，会出现悬垂引用或约束不一致的数据状态，且无错误提示，难以排查。

相比之下，`FieldService.Delete` 和 `FieldService.Update`（约束收紧检查）已正确使用 `HasRefsTx + FOR SHARE`；本 spec 是将同等防护覆盖到遗漏的两个路径。

## 优先级

**高**。

依据：
1. 数据一致性 bug，不是性能或体验问题。
2. 触发条件仅需两个并发请求，在实际测试阶段可复现。
3. 修复成本低（相关基础设施：`HasRefsTx`、`SoftDeleteTx`、事务模式已存在，直接复用）。

## 预期效果

### 场景 1：并发删除已被引用的扩展字段定义

**修复前**：
1. 用户 A 查询扩展字段 ID=5，`HasRefs` 返回 false（无引用）
2. 用户 B 创建事件类型，把扩展字段 5 加入 extensions → 写入 `schema_refs`
3. 用户 A 删除扩展字段 5 → 成功（schema_refs 存在悬垂数据）

**修复后**：
1. 用户 A 开事务，`HasRefsTx + FOR SHARE` 锁定 schema_refs 行
2. 用户 B 试图写 schema_refs → 被 FOR SHARE 阻塞（直到用户 A 事务结束）
3. 用户 A `HasRefsTx` 返回 false → 软删除 → 提交
4. 用户 B 解阻塞 → 写入 schema_refs（但字段已删，attachSchemaRefs 有启用性校验，会拒绝）

### 场景 2：并发给已被引用字段改类型

**修复前**：
1. 用户 A 查询字段 ID=3，`HasRefs` 返回 false
2. 用户 B 将字段 3 加入模板 → 写 `field_refs`
3. 用户 A 把字段 3 的 type 从 `integer` 改为 `string` → 成功（约束已损坏）

**修复后**：
1. 用户 A 开事务，`HasRefsTx + FOR SHARE` 锁定 field_refs 行
2. 用户 B 写 field_refs → 被阻塞
3. 用户 A `HasRefsTx` 返回 false → 允许改 type → 更新 → 提交
4. 用户 B 解阻塞 → 写 field_refs（字段已是 string 类型，后续行为一致）

### 场景 3：并发给被引用扩展字段收紧约束

**修复前**：
1. 用户 A 查询扩展字段 ID=7，`HasRefs` 返回 false
2. 用户 B 创建事件类型引用扩展字段 7 → 写 schema_refs
3. 用户 A 收紧扩展字段 7 的 min 约束 → 成功（现有事件类型的 default_value 可能违反新约束）

**修复后**：同场景 1 的锁机制，user A 持 FOR SHARE，user B 阻塞，user A 检查 HasRefs=false 跳过收紧检查→ 更新成功；如果 HasRefs=true 则强制约束检查。

## 依赖分析

**依赖**：
- `data-consistency-hardening` spec 已完成（事务模式、error-aware defer、cache-before-commit 已建立）
- `EventTypeSchemaStore.SoftDeleteTx` 需要新增（1 个 store 方法）
- `FieldStore.UpdateTx` 需要新增（1 个 store 方法）
- `EventTypeSchemaStore.UpdateTx` 需要新增（1 个 store 方法）

**谁依赖这个**：无后续 spec 强依赖，但修复后引用系统的可靠性基线提升，后续 NPC 模块上线时有正确的引用检查模式可参照。

## 改动范围

| 层 | 文件 | 改动 |
|---|---|---|
| store/mysql | event_type_schema.go | 新增 `SoftDeleteTx(ctx, tx, id)` |
| store/mysql | field.go | 新增 `UpdateTx(ctx, tx, req)` |
| store/mysql | event_type_schema.go | 新增 `UpdateTx(ctx, tx, req)` |
| service | event_type_schema.go | `Delete`：开事务 + `HasRefsTx` + `SoftDeleteTx` |
| service | event_type_schema.go | `Update`：开事务 + `HasRefsTx` + `UpdateTx` |
| service | field.go | `Update`：开事务 + `HasRefsTx` + `UpdateTx` |

预估：6 个文件，10-20 行净增，主要是方法签名扩展和事务封装。

## 扩展轴检查

- **新增配置类型**：不影响。本 spec 修复的是通用引用检查模式，新增类型自然复用同样模式。
- **新增表单字段**：不涉及。

无扩展轴影响，属纯防御性修复。

## 验收标准

**R1**：`EventTypeSchemaService.Delete` 使用事务 + `HasRefsTx`（FOR SHARE）+ `SoftDeleteTx`。

验证方法：阅读代码确认事务包裹 + FOR SHARE；`go test` 全通过。

**R2**：`FieldService.Update` 的 HasRefs 检查（类型变更 + 约束收紧）使用事务 + `HasRefsTx`（FOR SHARE）+ `UpdateTx`，乐观锁语义不变。

验证方法：阅读代码确认；curl 测试更新已被引用字段的类型返回 `ErrFieldRefChangeType`；`go test` 全通过。

**R3**：`EventTypeSchemaService.Update` 的 HasRefs 检查（约束收紧）使用事务 + `HasRefsTx`（FOR SHARE）+ `UpdateTx`，乐观锁语义不变。

验证方法：阅读代码确认；curl 测试收紧被引用扩展字段约束返回 `ErrExtSchemaRefTighten`；`go test` 全通过。

**R4**：`EventTypeSchemaStore`、`FieldStore`、`EventTypeSchemaStore` 分别新增 `SoftDeleteTx` / `UpdateTx`，签名与已有 `SoftDeleteTx`（在 template、field 中）的模式一致。

验证方法：`go build ./...` 通过；`go test ./...` 通过。

## 不做什么

- **不**修复 EventType.Delete 的无引用检查（TODO 明确标注"FSM/BT 上线后"，有意延迟）
- **不**修复 FsmConfig.Delete 的无 NPC 引用检查（同上）
- **不**为 EventType / FsmConfig 添加 GET /references 接口（NPC 未上线，结果恒为空，无意义）
- **不**添加数据库层外键约束（与"无外键，应用层校验"红线一致）
- **不**处理 Field.Update 以外的乐观锁问题
