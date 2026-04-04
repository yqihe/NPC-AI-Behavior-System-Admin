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
6. 检查文档是否需要同步更新（参考 `docs/development/dev-rules.md` 文档同步章节）
7. 在 tasks.md 中将任务标记为 `[x]`
8. commit 当前改动
9. 停下，输出完成摘要，建议跑 `/verify <feature-name>`

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

参考 `docs/development/go-pitfalls.md`，重点关注：

- **序列化安全**：nil slice/map 初始化了吗？bson tag 写了吗？omitempty 会吞零值吗？
- **HTTP Handler**：写错误响应后 return 了吗？WriteHeader 只调一次了吗？
- **MongoDB 操作**：context 带超时了吗？ErrNoDocuments 单独判断了吗？MatchedCount==0 返回 404 了吗？
- **Redis 操作**：redis.Nil 判断了吗？缓存 key 有统一前缀吗？写操作后清缓存了吗？
- **错误处理**：error 没忽略吧？errors.Is/As 而非 ==？错误信息没暴露给前端吧？
- **nil 安全**：map 初始化了吗？指针解引用前检查 nil 了吗？

## 写前端代码时必须检查

参考 `docs/development/frontend-pitfalls.md`，重点关注：

- **响应式**：解构 reactive 用 toRefs 了吗？ref 用了 .value 吗？reactive 没整体替换吧？
- **Element Plus 表单**：el-form-item 的 prop 和 model 字段名一致吗？dialog 关闭时重置表单了吗？
- **请求处理**：按钮提交时有 loading 防重复吗？错误提示取的是 response.data.error 吗？
- **空值防御**：后端返回 null 时前端不会 crash 吧？v-for 有稳定的 :key 吗？
- **环境差异**：API baseURL 用环境变量了吗？没有硬编码地址吧？

## DEBUG 日志

统一格式（参考 `docs/development/dev-rules.md`）：
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
**Go 陷阱检查**：[检查了哪些项，有无发现]
**前端陷阱检查**：[检查了哪些项，有无发现]（仅涉及前端时）

→ 建议跑 `/verify <feature-name>` 验证
```

## 经验沉淀

执行过程中踩到的坑，按类型追加到对应文档：
- Go 相关 → `docs/development/go-pitfalls.md`
- 前端相关 → `docs/development/frontend-pitfalls.md`
- 新禁令 → `docs/architecture/red-lines.md`
- 新规则 → `docs/development/dev-rules.md`
