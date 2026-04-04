# 需求：ADMIN 仓库配置审查与治理

## 动机

游戏服务端已完成 GitHub 仓库治理（分支保护、合并策略、.env 安全、标签体系等），要求 ADMIN 平台对齐。当前 ADMIN 仓库存在以下问题：

1. **main 分支无保护**：任何人可直接 push / force push，误操作可能丢失代码
2. **合并策略全部开放**：merge commit / squash / rebase 三种并存，main 历史混乱
3. **无 .env.example 模板**：docker-compose.yml 硬编码连接串，新开发者不知道哪些环境变量可配置
4. **自动删除分支未开启**：PR 合并后废弃分支残留
5. **已有 2 个废弃远端分支**：`feature/config-export`、`feature/table-responsive`
6. **无 Topics**：GitHub 仓库缺少技术栈标签，不利于检索和答辩展示
7. **Description 过于简单**：缺少学术价值描述
8. **仅默认 Labels**：无按模块分类的自定义标签
9. **Wiki 未关闭**：未使用但仍开启

不做的话：两个仓库 Git 规范不一致，答辩时 main 历史混乱，且存在 force push 丢代码的风险。

## 优先级

高。涉及代码安全（分支保护）和项目规范统一。不阻塞功能开发，但��早做越好——后续所有 PR 都受合并策略影响。

## 预期效果

- main 分支受保护，只能通过 PR 合并，禁止直接 push 和 force push
- 仅允许 Squash Merge，main 历史每个 PR 一条 commit，干净整洁
- PR 合并后远端分支自动删除
- 提供 .env.example 模板，docker-compose.yml 通过环境变量引用
- 废弃远端分支清理干净
- 仓库有技术栈 Topics、有描述学术价值的 Description
- 有按 ADMIN 模块分类的自定义 Labels
- Wiki 关闭
- 开发文档（CLAUDE.md、dev-rules.md）与仓库实际设置一致

## 依赖分析

- 依赖：无
- 被依赖：后续所有 PR 流程受分支保护和合并策略约束

## 改动范围

| 类型 | 内容 |
|------|------|
| GitHub 仓库设置 | 分支保护、合并策略、自动删除分支、Topics、Description、Labels、Wiki |
| 新增文件 | `.env.example` |
| 修改文件 | `docker-compose.yml`（环境变量引用）、`docs/development/dev-rules.md`（Git 规则更新） |

预计涉及 3 个代码文件 + 多个 gh CLI 操作。

## 扩展轴检查

不涉及两条扩展轴（新增配置类型 / 新增表单字段）。本需求属于基础设施治理。

## 验收标准

- **R1**：main 分支保护开启——`gh api repos/{owner}/{repo}/branches/main/protection` 返回 200
- **R2**：仅允许 Squash Merge——`gh repo view --json squashMergeAllowed,mergeCommitAllowed,rebaseMergeAllowed` 显示仅 squash 为 true
- **R3**：自动删除分支开启——`gh repo view --json deleteBranchOnMerge` 为 true
- **R4**：存在 `.env.example` 文件，包含所有环境变量的模板（值为空或示例值）
- **R5**：`docker-compose.yml` 通过 `${VAR:-default}` 语法引用环境变量，不再硬编码
- **R6**：无已合并的废弃远端分支——`git branch -r --merged main` 只剩 `origin/main`
- **R7**：仓库 Topics 包含技术栈关键词（vue、golang、mongodb 等）
- **R8**：仓库 Description 体现项目学术价值
- **R9**：存在按 ADMIN 模块分类的自定义 Labels
- **R10**：Wiki 关闭——`gh repo view --json hasWikiEnabled` 为 false
- **R11**：`docs/development/dev-rules.md` 的 Git 规则章节反映分支保护和 Squash Merge 策略

## 不做什么

- 不做 CI/CD 流水线配置
- 不做 Issue 模板 / PR 模板
- 不做 GitHub Actions
- 不改代码逻辑，只改基���设施配置和文档
