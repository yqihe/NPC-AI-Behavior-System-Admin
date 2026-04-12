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
- **扩展轴检查**：是否有利于运营平台的两个扩展方向（新增配置类型只需加一组 handler/service/store/validator；新增表单字段只需加组件），若都不涉及，说明理由
- **验收标准**：编号列出（R1, R2, ...），每条可验证，不能模糊
- **不做什么**：明确排除的范围

**禁止**：
- 不准提出无法验证的需求（"提升性能"→ 必须量化："Tick 调度 1000 NPC 时延迟 < 50ms"）
- 不准把多个独立功能塞进一个 spec

**→ 停下，等用户审批后进入 Phase 2**

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

---

## Phase 3：任务拆解

产出 `tasks.md`，把实现拆成原子任务：

- 每个任务：1-3 个文件，单一明确的产出
- 每个任务关联需求编号：`T1: 实现 FSM 配置加载 (R1, R3)`
- 每个任务标注涉及的文件路径
- 按依赖顺序排列
- 每个任务有明确的"做完了是什么样"的定义

**禁止**：
- 不准拆出涉及超过 3 个文件的任务——太大就再拆
- 不准拆出定义模糊的任务（"完善 XX 模块"）
- 不准在任务中包含"顺便重构"、"顺便优化"

**→ 停下，等用户审批。审批后：**
1. **创建 feature 分支**：`git checkout -b feature/<feature-name>`（从当前主开发分支拉出）
2. 开始 `/spec-execute T1 <feature-name>`

---

## 经验沉淀

执行过程中发现的新规则/禁令，按类型追加到对应文档（参考 `docs/development/admin/dev-rules.md` 经验沉淀指引）。发现本 Skill 遗漏的检查项，追加到本文件对应阶段。
