# field-constraint-hardening — 任务拆解

7 个原子任务，严格按依赖顺序。每个任务完成后立刻 `bash tests/api_test.sh` 跑回归（memory: "写完必须先验证"），FAIL 计数必须只减不增。

---

## T1：errcode 新增 `ErrFieldRefNested` / `ErrFieldRefEmpty`（R1, R3, R4） ✅

**文件**：
- `backend/internal/errcode/codes.go`（唯一）

**改动**：
- 在字段管理段位（40001-40015）末尾追加 2 条常量：
  ```go
  ErrFieldRefNested = 40016 // reference 字段禁止嵌套引用
  ErrFieldRefEmpty  = 40017 // reference 字段必须至少引用一个目标
  ```
- 在 `messages` map 中追加对应的中文消息：
  - `ErrFieldRefNested: "不能引用 reference 类型字段，禁止嵌套"`
  - `ErrFieldRefEmpty: "reference 字段必须至少引用一个目标字段"`

**做完是什么样**：
- `go build ./...` 通过
- 后端容器重启后 `/health` 返回 ok
- `tests/api_test.sh` 跑完 FAIL 计数不变（T1 本身不改行为，纯加常量）

**依赖**：无

---

## T2：handler 层 `properties` 形状校验（R5 → atk11.6） ✅

**文件**：
- `backend/internal/handler/field.go`（唯一）

**改动**：
- 新增包内函数 `checkPropertiesShape(raw json.RawMessage) *errcode.Error`，用 `bytes.TrimSpace` + 首字符判 `{`；
- 在 `Create` 内把 `if req.Properties == nil` 这一条替换为 `checkPropertiesShape(req.Properties)` 调用；
- 在 `Update` 内做同样替换；
- import 需要补 `"bytes"` 和 `"encoding/json"`（encoding/json 可能已经间接引入，按实际情况）。

**做完是什么样**：
- `go build ./...` 通过
- `tests/api_test.sh` 的 `atk11.6` 从 `[BUG ]` 变为 `[PASS]`（`properties=[]` 返回 40000）
- 原有 `f2.9 缺 properties 40000`、`f4.5 ID=0 40000` 等校验场景继续 PASS
- FAIL 计数减 1

**依赖**：无（可与 T1 并行）

---

## T3：service 层抽离 `validateReferenceRefs` 并接入 Create/Update（R1, R2, R3, R4 → atk1.2, atk7.1） ✅

**文件**：
- `backend/internal/service/field.go`（唯一）

**改动**：
1. 新增方法 `validateReferenceRefs(ctx, currentID, newRefIDs, oldRefSet) error`，实现规则：
   - `newRefIDs` 为空 → `ErrFieldRefEmpty`
   - 每个 refID 查 `fieldStore.GetByID`，不存在 → `ErrFieldRefNotFound`
   - 对"新增 ref"（不在 `oldRefSet` 中）检查：
     - 未启用 → `ErrFieldRefDisabled`
     - 是 reference 类型 → `ErrFieldRefNested`
   - 末尾调用现有 `detectCyclicRef`
2. 在 `Create` 中把现有 refs 校验段（field.go:134-158）替换成一次 `validateReferenceRefs(ctx, 0, refFieldIDs, nil)` 调用；
3. 在 `Update` 中把现有 refs 校验段（field.go:270-304 的 `if req.Type == model.FieldTypeReference { ... }` 整块）替换成：构造 `oldRefSet`（仅当 `old.Type == reference`），然后调用 `validateReferenceRefs(ctx, req.ID, newRefIDs, oldRefSet)`；
4. `syncFieldRefs` / `parseRefFieldIDs` / `detectCyclicRef` **不动**。

**做完是什么样**：
- `go build ./...` 通过
- `tests/api_test.sh`：
  - `atk1.2 reference 嵌套应被拒绝` 变 PASS（此刻期望的错误码还是 40009，见 T5 会更新为 40016；**T3 完成后 atk1.2 应该已经返回 40016 但断言脚本期望 40009**，这一步暂时显示为 BUG。等 T5 脚本更新完才 PASS。**这里是过渡状态，预期如此**）
  - `atk7.1 reference.refs=[] 被允许创建` 变 PASS（脚本的分支判断支持 40000/40014，但新错误码 40017 不在其中——**同样是过渡状态**，等 T5 脚本更新后 PASS）
- 原有 `f11.1 B refs [A] 成功`、`f11.2 引用停用字段 40013`、`f11.3 引用不存在字段 40014` 继续 PASS
- **回归风险点**：Part 3 功能 11 用例覆盖度最高，必须全部 PASS

**依赖**：T1（需要 `ErrFieldRefNested` / `ErrFieldRefEmpty` 常量）

---

## T4：service 层 `checkConstraintTightened` 补齐 select / float / string 覆盖（R6, R7, R8 → atk3/4/6） ✅

**文件**：
- `backend/internal/service/field.go`（唯一）

**改动**：
1. 新增辅助函数 `getStringFromRaw(raw json.RawMessage) string`，json.Unmarshal 失败返回空串；
2. `checkConstraintTightened` 函数内：
   - `case "integer", "float"` 下新增 float 专属的 `precision` 检查（新值小于旧值 → 40007 `ErrFieldRefTighten`）；
   - `case "string"` 下新增 `pattern` 检查，判定规则：`newPat != "" && newPat != oldPat` → 40007；
   - `case "select"` 下新增 `minSelect` 检查（新值大于旧值 → 40007）和 `maxSelect` 检查（新值小于旧值 → 40007）。
3. 现有 integer/float 的 min/max 检查、string 的 minLength/maxLength 检查、select 的 options 删除检查**全部保留不动**。

**做完是什么样**：
- `go build ./...` 通过
- `tests/api_test.sh`：
  - `atk3.1 minSelect 1→2 应 40007` PASS
  - `atk3.2 maxSelect 3→2 应 40007` PASS
  - `atk3.3 对照: 删除 options 40007` 继续 PASS（回归检查）
  - `atk4.1 precision 4→2 应 40007` PASS
  - `atk6.1 加 pattern 应 40007` PASS
- 原有 `f10.3 min 收紧`、`f10.4 max 收紧`、`f10.5 约束放宽成功`、`f10.6 被引用改 type 40006` 继续 PASS
- FAIL 计数减 4（atk3.1 / atk3.2 / atk4.1 / atk6.1）

**依赖**：无（可与 T3 并行，但都改同一个 `service/field.go`，为避免冲突建议 T3 → T4 串行）

---

## T5：tests/api_test.sh 断言微调（R9）

**文件**：
- `tests/api_test.sh`（唯一）

**改动**：
1. `atk1.2 reference 嵌套应被拒绝` 的期望错误码从 `"40009"` 改为 `"40016"`；
2. `atk7.1` 的分支判断 `if [ "$CODE" = "40000" ] || [ "$CODE" = "40014" ]` 扩展为 `|| [ "$CODE" = "40017" ]`；
3. 其他攻击断言不动（它们已经够宽松）。

**做完是什么样**：
- `bash tests/api_test.sh` 全部通过：**总计 199、通过 199、失败 0、BUGS 为空**；
- `atk5.1 step 1→5 当前放行` 仍显示 PASS（保留现状，不在本 spec 范围）；
- `atk12.1 超大 int 约束` 仍显示 [INFO] 并计入 PASS。

**依赖**：T1 + T3 + T4（错误码常量存在 + service 行为已改好）

---

## T6：`docs/v3-PLAN/配置管理/字段管理/features.md` 同步（R10）

**文件**：
- `docs/v3-PLAN/配置管理/字段管理/features.md`（唯一）

**改动**：
1. **功能 10（约束收紧检查）** 小节的"被引用时的收紧规则"列表补三条：
   - float：`precision` 只能增不能减（保持精度）
   - string：`pattern` 只能移除不能新增/变更（放宽原则）
   - select：`minSelect` 只能减、`maxSelect` 只能增、`options` 只能加不能删（已有）
2. **功能 11（循环引用检测）** 小节增加一段"禁止 reference 嵌套"，说明：reference 字段的 `refs` 不能指向另一个 reference 字段。对已有的嵌套引用保持存量不动（只在新增路径拦截），解释这是为了避免模板 popover 的"一层"假设被打破；
3. **错误码速查表** 追加 2 行：
   - 40016 / `ErrFieldRefNested` / 引用的字段是 reference 类型（禁止嵌套）
   - 40017 / `ErrFieldRefEmpty` / reference 字段 refs 为空
4. **已知限制**里移除 atk7/atk1.2/atk3/4/6 相关的可选留言（如果有的话）；**保留** "Create + syncFieldRefs 非原子" 这一条不动（仍是已知限制）。

**做完是什么样**：
- features.md 表格列完整覆盖 R6-R8 的规则；
- 错误码表从 15 条变为 17 条；
- 文档内容与新的后端行为完全一致。

**依赖**：T1 + T3 + T4（改动文档基于实际代码行为）

---

## T7：`docs/development/` 陷阱沉淀（R10）

**文件**：
- `docs/development/go-pitfalls.md`
- `docs/development/dev-rules.md`（若存在 constraint 扩展的 dev rule，追加一条；不存在则可选）

**改动**：

1. `go-pitfalls.md` 的 "JSON / BSON 序列化" 小节追加：
   > **`json.RawMessage` 对 `null` 不变 nil**：客户端传 `"field": null`，`req.Field` 是 `[]byte("null")` 而非 `nil`。`if req.Field == nil` 漏掉这种情况。对形状有要求的 RawMessage 字段（如 `properties` 必须是对象），必须用 `bytes.TrimSpace` + 首字符判 `{` 在 handler 层拦截，或在 service 层用 `json.Unmarshal` 到具体结构再判空。
2. `go-pitfalls.md` 在"数据结构"或"业务约束"下追加（若没有类似小节，新建"业务约束校验"）：
   > **新增 constraint key 必须同步更新 `checkConstraintTightened`**：字段类型若新增约束字段（比如未来加 `date` 类型的 `minDate/maxDate`），必须在 `service/field.go` 的 `checkConstraintTightened` switch 中同步补"被引用时是否允许收紧"的规则。漏写会导致被引用字段可以静默收紧新约束，数据一致性失守——这是 field-constraint-hardening spec 专门堵过的一类 bug。

**做完是什么样**：
- `go-pitfalls.md` 新增 2 条陷阱条目，包含具体场景和修复指引；
- 未来新人读到这两条后知道不要再犯。

**依赖**：无（可与 T6 并行）

---

## 执行顺序总览

```
T1 (errcode)
  ↓
T2 (handler shape)   ← 可与 T1 并行
  ↓
T3 (service refs)    ← 依赖 T1
  ↓
T4 (service tightened 扩展)   ← 依赖 T3（避免同文件冲突）
  ↓
T5 (test 脚本微调)   ← 依赖 T1/T3/T4
  ↓
T6 (features.md 同步)
T7 (pitfalls 沉淀)   ← 可与 T6 并行
```

## 每步验证协议

每个 T 完成后：
1. 后端重新构建：`docker compose up -d --build admin-backend`
2. 等 `/health` ok
3. 执行 `bash tests/api_test.sh`
4. 检查 FAIL 计数和 BUGS 数组
5. 若 FAIL 计数相对上一步未下降或有新增 FAIL，**立刻停下排查**，不要继续下一个 T

完成 T5 后应达到终态：**总计 199 / 通过 199 / 失败 0 / BUGS=[]**。

## 文档同步确认

- [ ] `docs/v3-PLAN/配置管理/字段管理/features.md` 已同步（T6）
- [ ] `docs/development/go-pitfalls.md` 已沉淀（T7）
- [ ] 错误码表与 `errcode/codes.go` 常量表完全一致
- [ ] 无遗漏的跨模块文档（模板管理 features 不需要改，其错误码段位不受本次影响）

---

**Phase 3 完成，停下等待审批**。

审批通过后：
1. 当前在 `feature/template-management-backend` 分支，工作区有未提交改动（两处 features.md + 新 tests/api_test.sh）。**先征求用户意见**是合并到当前分支直接 commit，还是切出新 `feature/field-constraint-hardening` 分支；
2. 开始 `/spec-execute T1 field-constraint-hardening`。
