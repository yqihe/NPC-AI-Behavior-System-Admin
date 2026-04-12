# /spec-execute — 执行任务

执行 spec 中的一个任务，写代码。

## Usage
```
/spec-execute <task-id> <feature-name>
```

---

## 分支规范

- **开始前**：确认已在 `feature/<feature-name>` 分支上。如果不在，先创建：`git checkout -b feature/<feature-name>`
- **每个任务完成后**：commit 当前改动（遵循 Git 规则中的 commit message 格式）
- **所有任务完成后**：
  1. 从远端拉取最新代码并合并：`git fetch origin && git merge origin/main`（如有冲突则解决）
  2. 确认编译/构建通过
  3. push 到远端：`git push -u origin feature/<feature-name>`
  4. 提示用户是否需要创建 PR

## 执行流程

1. 确认在正确的 feature 分支上（不在则创建并切换）
2. 加载 `docs/specs/<feature-name>/` 下的 requirements.md、design.md、tasks.md
3. 定位目标任务，确认未完成
4. 读取任务涉及的所有文件（先读再改，不准盲改）
5. 执行实现
6. 检查文档是否需要同步更新（参考 `docs/development/admin/dev-rules.md` 文档同步章节）
7. 在 tasks.md 中将任务标记为 `[x]`
8. **立即执行 `/verify <feature-name> --task=T[N]`**——写完代码必须先验证，不允许跳过
9. verify PASS → commit 当前改动 → 自动继续下一个 task
10. verify FAIL → 停下报告，修复后重新 verify

---

## 禁止

- **一次只做一个任务**，做完停下等审批，不准自动进入下一个
- **不准加没要求的功能**——任务说实现 A，就只实现 A
- **不准过度封装**——不准为一个调用点创建接口/抽象层
- **不准瞎重构**——不准顺手改不相关的代码，哪怕它"看起来可以更好"
- **不准盲改**——动任何文件前必须先读它，理解上下文
- **不准自己判定测试通过**——自己写的代码自己不当裁判，交给 `/verify`
- **不准假装测试通过**——不准在完成摘要里写"测试应该没问题"

## Agent 使用

- 可以开多个 Agent 并行提高效率
- Agent 必须专职：探索代码的不改代码，写代码的不做验证
- 不准读其他 Agent worktree 的中间文件，等返回结果
- 不准给 Agent 设不同模型

## 写 Go 代码时必须检查

按涉及的技术领域查阅对应开发规范（`docs/development/standards/dev-rules/`），重点关注：

- **Go 语言**（`go.md`）：nil slice/map 初始化了吗？bson tag 写了吗？omitempty 会吞零值吗？error 没忽略吧？map 初始化了吗？中文字符串长度用 `utf8.RuneCountInString` 了吗？
- **HTTP Handler**（`go.md`）：写错误响应后 return 了吗？WriteHeader 只调一次了吗？
- **MySQL**（`mysql.md`）：事务内查询用 tx 了吗？TOCTOU 防护用了 FOR SHARE 吗？LIKE 转义了吗？乐观锁 rows==0 语义清楚吗？
- **MongoDB**（`mongodb.md`）：context 带超时了吗？ErrNoDocuments 单独判断了吗？MatchedCount==0 返回 404 了吗？
- **Redis**（`redis.md`）：redis.Nil 判断了吗？缓存 key 用 keys.go 生成了吗？
- **缓存**（`cache.md`）：写操作后清了 list 和 detail 缓存吗？空值标记防穿透了吗？TTL 加抖动了吗？
- **测试**（`docs/development/admin/dev-rules.md` "测试脚本编写规范"章节）：jq 提取加 `tr -d '\r'` 了吗？环境重置清 Redis 了吗？断言错误码对准 errcode/codes.go 了吗？

## 写前端代码时必须检查

参考 `docs/development/standards/dev-rules/frontend.md`，重点关注：

- **响应式**：解构 reactive 用 toRefs 了吗？ref 用了 .value 吗？reactive 没整体替换吧？
- **Element Plus 表单**：el-form-item 的 prop 和 model 字段名一致吗？dialog 关闭时重置表单了吗？
- **请求处理**：按钮提交时有 loading 防重复吗？错误提示取的是 response.data.error 吗？
- **空值防御**：后端返回 null 时前端不会 crash 吧？v-for 有稳定的 :key 吗？
- **环境差异**：API baseURL 用环境变量了吗？没有硬编码地址吧？

## DEBUG 日志

统一格式（参考 `docs/development/admin/dev-rules.md`）：
```go
log.Debug("组件.动作", "key1", val1, "key2", val2)
```

**判断标准**：这行代码出 bug 时，有这条日志能帮助排查吗？能就加。

重点加日志的位置：
- API 请求入口（method、path、关键参数）
- MongoDB 操作（collection、操作类型、name）
- Redis 缓存命中/未命中/失效
- 配置校验失败（哪个字段、什么原因）
- 错误处理分支（原始 error 写 slog，不暴露给前端）

## 完成摘要模板

```
## Task T[N] 完成

**实现内容**：[一句话]
**改动文件**：[文件列表]
**满足需求**：[R1, R3, ...]
**新增测试**：[有/无，覆盖什么]
**文档同步**：[更新了哪些文档 / 无需更新]
**陷阱检查**：[按涉及技术领域检查了哪些项，有无发现]

→ 建议跑 `/verify <feature-name>` 验证
```

## 经验沉淀

执行过程中踩到的坑，按技术领域追加到对应文档（参考 `docs/development/admin/dev-rules.md` 经验沉淀指引表格）。
