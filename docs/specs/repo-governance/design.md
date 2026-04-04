# 设计方案：ADMIN 仓库配置审查与治理

## 方案描述

分两类操作：**GitHub 仓库设置**（gh CLI）和**代码文件改动**（git commit）。

### 1. GitHub 仓库设置（gh CLI）

#### 1a. 分支保护 (R1)

```bash
gh api repos/yqihe/NPC-AI-Behavior-System-Admin/branches/main/protection \
  -X PUT \
  -f enforce_admins=true \
  -F required_pull_request_reviews=null \
  -F required_status_checks=null \
  -F restrictions=null \
  --input - <<'EOF'
{
  "enforce_admins": true,
  "required_pull_request_reviews": null,
  "required_status_checks": null,
  "restrictions": null,
  "allow_force_pushes": false,
  "allow_deletions": false
}
EOF
```

注：免费版 GitHub 对私有仓库分支保护有限制。如果仓库是 public 则直接生效；如果是 private 需确认 plan 支持。

#### 1b. 合并策略：仅 Squash Merge (R2)

```bash
gh repo edit --enable-squash-merge --disable-merge-commit --disable-rebase-merge
```

#### 1c. 自动删除分支 (R3)

```bash
gh repo edit --delete-branch-on-merge
```

#### 1d. 清理废弃远端分支 (R6)

```bash
git push origin --delete feature/config-export
git push origin --delete feature/table-responsive
```

#### 1e. Topics (R7)

```bash
gh repo edit --add-topic vue,golang,mongodb,redis,element-plus,docker,npc-ai,behavior-tree,finite-state-machine
```

#### 1f. Description (R8)

```bash
gh repo edit --description "NPC AI 行为系统 — 运营管理平台 | 毕业设计：为策划提供可视化 NPC 配置管理，涵盖事件类型、状态机、行为树、NPC 类型的 CRUD 与校验"
```

#### 1g. 自定义 Labels (R9)

按 ADMIN 模块创建：

| Label | 颜色 | 说明 |
|-------|------|------|
| `module:event-type` | #1d76db | 事件类型模块 |
| `module:npc-type` | #0e8a16 | NPC 类型模块 |
| `module:fsm-config` | #e4e669 | 状态机模块 |
| `module:bt-tree` | #d4c5f9 | 行为树模块 |
| `module:dashboard` | #f9d0c4 | 仪表盘模块 |
| `module:api` | #c5def5 | API / 后端通用 |
| `module:frontend` | #bfdadc | 前端通用 |
| `infra` | #666666 | 基础设施 / CI / Docker |
| `security` | #b60205 | 安全相关 |

#### 1h. 关闭 Wiki (R10)

```bash
gh repo edit --disable-wiki
```

### 2. 代码文件改动

#### 2a. .env.example (R4)

新建 `.env.example`，包含所有后端可配置的环境变量：

```env
# MongoDB
MONGO_URI=mongodb://mongo:27017
MONGO_DATABASE=npc_ai

# Redis
REDIS_ADDR=redis:6379

# Server
LISTEN_ADDR=:9821
```

#### 2b. docker-compose.yml 环境变量引用 (R5)

将硬编码值改为 `${VAR:-default}` 语法：

```yaml
environment:
  MONGO_URI: ${MONGO_URI:-mongodb://mongo:27017}
  MONGO_DATABASE: ${MONGO_DATABASE:-npc_ai}
  REDIS_ADDR: ${REDIS_ADDR:-redis:6379}
  LISTEN_ADDR: ${LISTEN_ADDR:-:9821}
```

默认值与当前硬编码一致，开发体验零改变；生产环境通过 `.env` 文件覆盖。

#### 2c. .gitignore 修复

当前规则：
```
.env.*
!.env
!.env.*.example
```

`!.env` 允许 .env 被跟踪，这是安全隐患。改为：

```
.env
.env.*
!.env.example
!.env.*.example
```

#### 2d. dev-rules.md Git 规则更新 (R11)

更新 Git 规则章节，反映：
- 分支保护：main 禁止直接 push，只接受 PR
- 合并策略：仅 Squash Merge
- 自动删除分支

## 方案对比

| | 方案 A：gh CLI + 代码改动（选用） | 方案 B：GitHub Web UI 手动操作 |
|--|--|--|
| 做法 | 全部通过 gh CLI 脚本化执行 | 在 GitHub Settings 页面手动点击 |
| 优点 | 可复现、可审计、可在 spec 中记录确切命令 | 无 |
| 缺点 | 需确认 gh CLI 权限 | 不可复现、无审计记录、易遗漏 |
| 结论 | **选用** | 不选 |

## 红线检查

- `docs/standards/red-lines.md`：无涉及
- `docs/standards/go-red-lines.md`：无涉及（不改 Go 代码）
- `docs/standards/frontend-red-lines.md`：无涉及（不改前端代码）
- `docs/architecture/red-lines.md`：
  - "禁止破坏游戏服务端数据格式"：不涉及，不改 MongoDB 结构
  - "禁止绕过 REST API"：不涉及
  - docker-compose.yml 改动保持默认值一致，行为不变

**无违反。**

## 扩展性影响

不涉及两条扩展轴。仓库治理是基础设施层面，对功能扩展无影响。

## 依赖方向

无代码包依赖变更。

## Go 陷阱检查

不涉及 Go 代码。

## 前端陷阱检查

不涉及前端代码。

## 配置变更

- 新增 `.env.example`（模板文件，非运行时配置）
- `docker-compose.yml` 改用变量替换语法，默认值不变，运行行为不变

## 测试策略

- **GitHub 设置验证**：每个 gh CLI 操作后立即查询确认生效
- **docker-compose 验证**：`docker compose up --build` 确认服务正常启动（默认值兜底，行为不变）
- **文档验证**：读取 dev-rules.md 确认内容与实际设置一致
