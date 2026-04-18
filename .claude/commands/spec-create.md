# /spec-create — 需求规划

从需求分析到任务拆解的完整规划流程。三个阶段，每阶段必须等用户审批后才能进入下一阶段。

## Usage
```
/spec-create <feature-name> [简要描述]
```

产出目录：`docs/specs/<feature-name>/`

---

## Phase 1：需求分析

产出 `requirements.md`，必须包含：

- **动机**：为什么要做这个，不做会怎样
- **优先级**：相对于项目当前阶段的紧迫程度，依据是什么
- **预期效果**：做完后系统行为应该是什么样，用具体场景描述
- **依赖分析**：依赖什么已完成的工作，谁依赖这个需求
- **改动范围**：预估涉及哪些包、多少文件
- **扩展轴检查**：见下方定义，判断本需求是否有利于两个扩展方向，若都不涉及，说明理由
- **验收标准**：编号列出（R1, R2, ...），每条可验证，不能模糊
- **不做什么**：明确排除的范围

**扩展轴定义**（运营平台的两个预设扩展方向）：
1. **新增配置类型**：加一组 handler/service/store/validator 即可，不改已有模块代码
2. **新增表单字段**：加一个表单组件即可，不改 SchemaForm 核心

**禁止**：
- 不准提出无法验证的需求（"提升性能"→ 必须量化："Tick 调度 1000 NPC 时延迟 < 50ms"）
- 不准把多个独立功能塞进一个 spec

**→ 停下，等用户审批后进入 Phase 2**

> 本阶段若发现新的需求分析盲区或反复踩的坑，记得在完成后追加到"经验沉淀"所指的文档。

---

## Phase 2：设计方案

产出 `design.md`，必须包含：

- **方案描述**：技术方案、数据结构、接口定义
- **方案对比**：至少列出一个备选方案，说明为什么不选
- **红线检查**：逐条对照以下红线文档，确认不违反。如果方案触及任何一条红线，必须修改方案或说明为什么需要修改红线本身（需用户批准）
    - 通用：`docs/development/standards/red-lines/general.md`
    - Go：`docs/development/standards/red-lines/go.md`
    - MySQL：`docs/development/standards/red-lines/mysql.md`
    - Redis：`docs/development/standards/red-lines/redis.md`
    - 缓存：`docs/development/standards/red-lines/cache.md`
    - 前端：`docs/development/standards/red-lines/frontend.md`
    - ADMIN 专属：`docs/development/admin/red-lines.md`
    - **若某份文档不存在**：报错停止，提示用户"红线文档 xxx.md 缺失，请先补齐或从本清单显式移除"。不准静默跳过——清单与实际文档必须保持同步。
- **扩展性影响**：这个设计是否影响运营平台的扩展方向（新增配置类型 / 新增表单字段），正面还是负面
- **依赖方向**：画出涉及的包之间的依赖关系，确认单向向下
- **陷阱检查**：按涉及的技术领域查阅对应开发规范（`docs/development/standards/dev-rules/` 下按技术拆分：`go.md`、`mysql.md`、`redis.md`、`mongodb.md`、`cache.md`、`frontend.md`）
- **配置变更**：是否需要新增/修改 JSON 配置文件，schema 是什么
- **测试策略**：怎么测，单元测试覆盖什么，e2e 覆盖什么

**禁止**：
- 不准设计违反 `docs/development/standards/red-lines/` 或 `docs/development/admin/red-lines.md` 红线的方案（除非审批修改红线）
- 不准跳过方案对比直接给一个方案
- 不准设计新增配置类型时需要改已有模块代码的方案

**→ 停下，等用户审批后进入 Phase 3**

> 本阶段若发现新的设计反模式或红线遗漏，记得在完成后追加到对应红线文档或 dev-rules。

---

## Phase 3：任务拆解

产出 `tasks.md`，把实现拆成原子任务：

- 每个任务：1-3 个文件，单一明确的产出
- 每个任务关联需求编号：`T1: 实现 FSM 配置加载 (R1, R3)`
- 每个任务标注涉及的文件路径
- 按依赖顺序排列
- 每个任务有明确的"做完了是什么样"的定义——必须具体到可验证，不能只写"测试通过"

**"做完了是什么样"示例**：
- ❌ 模糊：`单元测试通过`
- ❌ 模糊：`FSM 配置能正常加载`
- ✅ 具体：`在 xx_store.go 实现 GetByID(id int64) (*Model, error)，未找到返回 errcode.ErrNotFound；单测覆盖命中/未命中两种情况，均通过`
- ✅ 具体：`POST /api/v1/fsm-configs 接受 name + config JSON，重复 name 返回 409，成功返回 201 + 新建记录；e2e 用 curl 验证两种路径`

**禁止**：
- 不准拆出涉及超过 3 个文件的任务——太大就再拆
- 不准拆出定义模糊的任务（"完善 XX 模块"）
- 不准在任务中包含"顺便重构"、"顺便优化"

**→ 停下，等用户审批。审批后：**
1. **创建 feature 分支**：`git checkout -b feature/<feature-name>`（从当前主开发分支拉出）
2. **判断 T1 的判断密度，建议下一步**：
   - 如果 T1 涉及**重判断场景**（新模块设计 / 新 API 契约 / schema 变更 / 跨模块依赖 / 状态机 / 新引入抽象层）→ 建议先 `/backend-design-audit T1` 产出决策备忘，再进入 `/spec-execute`
   - 如果 T1 是**轻执行场景**（明确的 CRUD / 单点 bug 修复 / 纯样式调整）→ 直接 `/spec-execute T1 <feature-name>`

**不要默认跳过 audit，也不要默认自动 audit**——主动向用户提出建议并说明理由，等用户确认后再执行。

> 本阶段若发现新的拆解反模式（如总拆得太粗、漏掉测试任务），记得在完成后追加到本 Skill 或 dev-rules。

---

## 经验沉淀

执行过程中发现的新规则/禁令，按类型追加到对应文档（参考 `docs/development/admin/dev-rules.md` 经验沉淀指引）。发现本 Skill 遗漏的检查项，追加到本文件对应阶段。
