# field-constraint-hardening — 需求分析

## 动机

集成测试 `tests/api_test.sh` Part 5 攻击段跑出 7 个失败项，全部集中在**字段管理的业务校验层**（`backend/internal/service/field.go` 与 `backend/internal/handler/field.go`）。这些不是性能或架构问题，而是**校验漏网**——后端放行了本该拒绝的请求，或接受了会污染数据的输入。

不修会发生：

- **脏数据落库**：`properties=[]`（数组而非对象）直接被 `json.RawMessage` 接收入库，之后 `parseProperties` 静默返回空对象，这条字段从此行为不可预测；
- **前端假设崩塌**：`reference` 字段的前端设计全部建立在"reference 只有一层、子字段必然是 leaf"之上（详见模板管理 features 功能 8），但后端允许嵌套，一旦真的有人嵌套，模板编辑页 popover 会看到另一个 reference、子字段扁平化展开逻辑整个失效；
- **约束收紧静默通过**：`select.minSelect/maxSelect`、`float.precision`、`string.pattern` 被引用字段修改后不会报错，已有模板里依赖老约束填的值会突然变非法——这违反了"被引用字段约束只能放宽不能收紧"这个核心不变式；
- **无效 reference 字段**：`refs=[]` 的 reference 字段能创建成功，点开弹层一片空白。

## 优先级

**P0 — 当前阶段的最高优先级**。

依据：

1. 字段管理是 V3 所有配置模块的基座（模板、NPC、FSM、BT 都会复用它的校验）。基座漏校验，后面每个模块都要重复补，成本只会更高；
2. 其中 atk11.6（`properties` 数组）和 atk1.2（reference 嵌套）会让**脏数据直接落库**，等到模板管理前端开始消费时才爆就晚了；
3. 其余 5 个是同一个函数 `checkConstraintTightened` 的覆盖面问题，一次修完成本最低。

## 预期效果

修复后下面 7 个场景的后端行为：

| 场景 | 当前 | 期望 |
|---|---|---|
| 创建 reference 字段，`refs` 指向另一个 reference 字段 | 成功 | 40009（复用循环引用码）或新增"禁止嵌套"错误码，创建失败 |
| 创建 reference 字段，`refs=[]` 空数组 | 成功 | 40000 或 40014，创建失败，提示"reference 字段必须至少引用一个目标字段" |
| 创建/编辑字段时 `properties` 传数组 `[]` | 成功，落库为 `[]` | 40000，拒绝，提示"properties 必须是对象" |
| 被引用 `select` 字段把 `minSelect` 从 1 改到 2 | 成功 | 40007 `ErrFieldRefTighten` |
| 被引用 `select` 字段把 `maxSelect` 从 3 改到 2 | 成功 | 40007 |
| 被引用 `float` 字段把 `precision` 从 4 改到 2 | 成功 | 40007 |
| 被引用 `string` 字段从无 `pattern` 加到 `^[0-9]+$` | 成功 | 40007 |

所有修复后 `bash tests/api_test.sh` 的 Part 5 攻击段必须全部 PASS（atk5 "step 变更" 维持"放行并记录"的现状，不列入本次改动）。

## 依赖分析

**依赖已完成的工作**：

- `backend/internal/service/field.go` 的 `checkConstraintTightened` / `parseProperties` / `parseRefFieldIDs` / `detectCyclicRef` 已存在，本次是在这些函数内部补规则，不改函数签名；
- `backend/internal/handler/field.go` 的 `Create` / `Update` 已存在，本次只在 handler 层加一个"`properties` 必须是 JSON 对象"的前置校验；
- 测试脚本 `tests/api_test.sh` 已经覆盖 7 个 bug 的断言，修复后直接重跑即可验证。

**谁依赖本需求**：

- **模板管理前端**：目前前端还在做的编辑页"reference popover 只有一层"假设要等后端嵌套拦截落地才能安全；
- **后续 NPC 管理**：NPC 创建时会按模板快照字段并校验 required，前提是字段的约束定义是可信的；
- **游戏服务端导出**：`/api/configs/*` 导出时读的就是这些 `properties`，坏数据直接上线。

## 改动范围

| 位置 | 性质 | 预估行数 |
|---|---|---|
| `backend/internal/service/field.go` | `checkConstraintTightened` 补 select/float/string 三个 case 的规则；新增 `validateReferenceRefs` 统一做"非空/非嵌套/存在/启用"校验，替换 Create / Update 中两处零散逻辑 | +80 / -20 |
| `backend/internal/handler/field.go` | `Create` / `Update` 前置校验新增"`properties` 必须是 JSON 对象"一条 | +12 |
| `backend/internal/errcode/codes.go` | 若选择新增"禁止嵌套 reference" 专用错误码：+1 常量、+1 消息；或复用 40009，不动 | +2 或 0 |
| `tests/api_test.sh` | 无需改断言，atk7.1/atk11.6 的分支判断已经允许多种错误码；仅 atk1.2 可能需要从 40009 宽松到 40009/4001X | +0 ~ +3 |
| `docs/v3-PLAN/配置管理/字段管理/features.md` | 功能 10（约束收紧）表格补三行；功能 11（循环引用）加一行"禁止 reference 嵌套" | +15 |
| `docs/development/go-pitfalls.md` | 新增"`json.RawMessage` 对象型字段必须在 handler 层预校验形状" 陷阱 | +15 |
| `docs/development/cache-pitfalls.md` 或 `field-pitfalls.md` | 新增"reference 字段禁止嵌套的强约束必须在后端校验而非仅靠前端" | +10 |

**总计**：2 个 `.go` 文件实质改动，1 个可选 errcode 扩充，2 个文档补齐，1 个测试脚本微调。严格在"字段管理模块"内部，不牵连模板/字典/缓存。

## 扩展轴检查

V3 运营平台的两个扩展方向：

1. **新增配置类型**（加一组 handler/service/store/validator）——本次改动**正向**：
   - `validateReferenceRefs` 抽成独立函数后，未来若状态机/行为树也需要类型校验"引用另一个配置项的字段"，可以直接复用；
   - `checkConstraintTightened` 补齐后，是对"约束收紧检查"这个通用校验策略的完善，新配置类型引入新约束 key 时有模式可循。

2. **新增表单字段**（加一个组件）——本次改动**正向**：
   - `properties` 形状校验让前端可以放心假设拿到的一定是对象而非数组，新增 constraint 子表单（比如未来的 `date-range` 字段）时不用再防御"万一是数组"；
   - 收紧检查覆盖到 precision/pattern/minSelect/maxSelect 后，新 constraint key 加入时开发者知道**必须同步更新 `checkConstraintTightened`**，形成一个可遵循的规约。

## 验收标准

- **R1**：创建 `type=reference` 且 `refs` 中任一元素指向 `type=reference` 的字段时，API 返回业务错误码（40009 或新码），不落库。
- **R2**：编辑 `type=reference` 字段新增 refs 时，新增的目标字段若为 reference 同样拒绝；保留已存在的嵌套引用不受本次拦截影响（存量不动原则），仅拦截"新增/变更产生的新嵌套"。
- **R3**：创建 `type=reference` 字段时 `refs=[]`（空数组）返回业务错误码，不落库；提示信息包含"至少引用一个字段"语义。
- **R4**：编辑 `type=reference` 字段将 `refs` 改为 `[]` 同样被拒绝，且错误码与 R3 一致。
- **R5**：`POST /fields/create` 与 `/fields/update` 收到 `properties` 字段值为 JSON 数组 `[]`、JSON 字符串、JSON 数字、`true`/`false` 或 `null` 时，在 handler 层返回 40000，不进 service。
- **R6**：被引用（`ref_count > 0`）的 `select` 字段，`minSelect` 变大、`maxSelect` 变小、删除任一 `options`，三种场景均返回 40007。
- **R7**：被引用的 `float` 字段，`precision` 变小返回 40007；变大或不变放行。
- **R8**：被引用的 `string` 字段，从无 `pattern` 新增 `pattern`、或 `pattern` 字符串变化返回 40007；`pattern` 为空字符串时视作"无 pattern"。
- **R9**：`tests/api_test.sh` 在 Part 5 中原来 7 个 `[BUG ]` 条目全部变为 `[PASS]`，脚本 FAIL 计数归 0，总计 ≥199、通过 ≥199。
- **R10**：`docs/v3-PLAN/配置管理/字段管理/features.md` 功能 10 的 constraint key 表更新后，前后端命名契约同步；`docs/development/` 下新增的陷阱条目能让新人读到后知道不要再犯。
- **R11**：修复不破坏现有任何通过用例——`tests/api_test.sh` Part 1-4 全部继续 PASS。

## 不做什么

- **不扩展校验到未被引用的字段**：`ref_count=0` 的字段仍然允许任意收紧、任意格式的 pattern、任意精度，这是"存量不动增量拦截"原则的另一面。
- **不改 constraint key 的命名契约**：seed 文件 `backend/cmd/seed/main.go` 的 `constraint_schema` 不动，本次只在后端校验补齐这些 key。
- **不做 `integer.step` 变更检查**（atk5 当前放行）：step 语义上不是"范围约束"，产品决策未定，留给后续。
- **不做 `properties.constraints` 整体 JSON Schema 校验**：那是另一个层次的大改动，本次只拦最明显的形状错误（数组/标量/null）。
- **不动模板管理 / 字典 / 缓存任何代码**：所有修复限制在字段模块内。
- **不修 Create + syncFieldRefs 非原子问题**（features 已知限制里记的）：本次只补校验，不动事务边界。
- **不新增前端代码**：约束收紧提示文案沿用现有的 40007 消息即可，前端无需改动。

---

**Phase 1 完成，停下等待审批**。审批通过后进入 Phase 2 设计方案。
