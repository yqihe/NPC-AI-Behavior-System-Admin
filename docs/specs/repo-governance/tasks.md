# 任务拆解：ADMIN 仓库配置审查与治理

## [x] T1: GitHub 仓库设置——合并策略 + 自动删除分支 + Wiki (R2, R3, R10)

**文件**：无代码文件，纯 gh CLI 操作

**做什么**：
- `gh repo edit --enable-squash-merge --disable-merge-commit --disable-rebase-merge`
- `gh repo edit --delete-branch-on-merge`
- `gh repo edit --disable-wiki`

**做完是什么样**：仅 Squash Merge 可用，PR 合并后分支自动删除，Wiki 关闭。

## [x] T2: GitHub 仓库设置——Topics + Description + Labels (R7, R8, R9)

**文件**：无代码文件，纯 gh CLI 操作

**做什么**：
- 设�� Topics：vue, golang, mongodb, redis, element-plus, docker, npc-ai, behavior-tree, finite-state-machine
- 更新 Description
- 创建 9 个自定义 Labels

**做完是什么样**：仓库页面有技术栈标签、学术价值描述、模块分类标签。

## [x] T3: 清理废弃远端分支 (R6)

**文件**：无代码文件，纯 git 操作

**做什么**：
- `git push origin --delete feature/config-export`
- `git push origin --delete feature/table-responsive`
- 清理本地对应的远端追踪分支

**做完是什么样**：`git branch -r --merged main` 只剩 `origin/main`。

## [x] T4: .env.example + .gitignore 修复 + docker-compose.yml 变量化 (R4, R5)

**文件**：
- `.env.example`（新建）
- `.gitignore`（修改 env 部分）
- `docker-compose.yml`（环境变量 `${VAR:-default}` 语法）

**做什么**：
- 新建 `.env.example` 模板
- `.gitignore` 中 `!.env` 改为忽略 `.env`，保留 `!.env.example`
- `docker-compose.yml` 的 4 个环境变量改为 `${VAR:-default}` 引用

**做完是什么样**：`.env` 被 gitignore，模板文件存在，docker-compose 默认值不变但支持环境变量覆盖。

## T5: dev-rules.md Git 规则更新 (R11)

**文件**：`docs/development/dev-rules.md`

**做什么**：
- Git 规则章节新增：main 分支保护（禁止直接 push，只接受 PR）
- Git 规则章节新增：仅 Squash Merge
- Git 规则章节新增：PR 合并后远端分支自动删除
- 更新"提交即推送"部分，反映分支保护后的工作流变化

**做完是什么样**：dev-rules.md 的 Git 规则与 GitHub 仓库实际设置一致。

## T6: main 分支保护 (R1)

**文件**：无代码文件，纯 gh API 操作

**做什么**：
- 通过 `gh api` 设置 main 分支保护规则
- 验证保护生效

**做完是什么样**：`gh api repos/{owner}/{repo}/branches/main/protection` 返回 200。

**注意**：此任务放在最后执行。开启保护后 main 不能直接 push，T4/T5 的代码改动需要在此之前合并。

## 依赖顺序

```
T1 → T2 → T3 → T4 → T5 → T6
                     ↑
              T4/T5 必须在 T6（分支保护）之前合并到 main
              T6 放最后，开启���护后工作流变更生效
```
