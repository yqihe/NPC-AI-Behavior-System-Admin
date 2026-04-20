# regions-module T17 前端手测 checklist

> 前置：admin-frontend http://localhost:3000 运行中；admin-backend 9821 healthy。
> 覆盖 [design.md §8.2](design.md) 中 #5 / #6 两个前端场景 + T15 特有的 help-text 截图。

共 3 场景，全程浏览器操作。每步都列出「操作」+「期望」。跑完把结果贴到 PR description（或直接在本文件旁边加一节 `result-2026-04-20.md`）。

---

## 场景 A（对应 §8.2 #5）：spawn_entry 红点分支

### 前置准备（终端先执行）

把 villager_guard 保持 enabled，另外准备一个启用了的 NPC A 和一个禁用了的 NPC B：

```bash
# 找一个当前 enabled=false 的 NPC（或临时 disable 一个非 villager_guard 的 NPC）
curl -sS -X POST http://localhost:9821/api/v1/npcs/list \
  -H 'Content-Type: application/json' \
  -d '{"enabled":false,"page":1,"page_size":3}'
```

> 如果都是 enabled，随便挑一个非 villager_guard 的 `toggle-enabled` 到 disable，记下它的 `name` 和 `label`。下面用「B_NAME / B_LABEL」占位。

### 手测步骤

1. 浏览器打开 `http://localhost:3000/regions/create`
2. 顶部表单填：
   - 区域标识：`smoke_ref_test`
   - 中文名：`引用红点测试`
   - 区域类型：选「野外」
3. Spawn 配置卡点击「添加 Spawn Entry」两次，得到 Entry 1 + Entry 2
4. Entry 1（引启用 NPC）：
   - NPC 引用下拉 → 选 `villager_guard`
   - 数量：保持 1
   - Spawn 坐标：只 1 个默认点 `{x:0, z:0}`
5. Entry 2（引禁用 NPC）：
   - NPC 引用下拉 → 选「B_NAME」
   - 数量：保持 1
   - Spawn 坐标：保留默认 1 个点
6. 点底部「保存」

### 期望

- 后端返 `code=47007`「spawn_entry.template_ref 指向未启用的 NPC 模板」（如果是不存在的 NPC 就是 47006）
- 前端顶部 toast 显示红色错误消息（message 为后端中文）
- ⚠ 当前实现：entry 卡片的 NPC 引用下拉框**不会**自动红边（因为后端没回传 which entry index — entryErrors 只对 template_ref 为空的前端校验生效）。toast 是主验证点。

### 记录

| 项 | 实际 |
|---|---|
| toast 文案 | |
| HTTP status | |
| 失败后留在表单页 / 跳走 | |
| 截图 | |

---

## 场景 B（对应 §8.2 #6）：乐观锁 47010 弹窗

### 前置

任选一个 enabled=false 的 region（若无，找一个 region 先 disable 它）。下面用 `R_ID` 占位它的 id。

### 手测步骤

1. 浏览器 Tab 1 打开 `http://localhost:3000/regions/R_ID/edit`，把中文名改成「Tab1 改动」，**暂不保存**
2. 浏览器 Tab 2 打开**相同** URL，把中文名改成「Tab2 改动」，点「保存」
3. 等 Tab 2 回到列表
4. 回 Tab 1，点「保存」

### 期望

- Tab 1 弹出 ElMessageBox — 标题「版本冲突」、正文「数据已被其他人修改，请刷新后重试。」
- 点「确定」后表单留原地（不自动跳转），用户可手动刷新或回列表
- list 页 villager 对应行的 display_name 是「Tab2 改动」

### 记录

| 项 | 实际 |
|---|---|
| Tab 2 保存是否 200 | |
| Tab 1 弹窗标题+正文 | |
| 列表最终 display_name | |
| 截图（Tab1 弹窗）| |

---

## 场景 C：respawn_seconds help-text 截图（T15 特有）

### 手测步骤

1. 浏览器打开 `http://localhost:3000/regions/create`
2. 点击「添加 Spawn Entry」
3. 聚焦到 Entry 1 的「重生间隔」字段

### 期望

- 输入框右侧「秒」后缀可见
- 输入框下方灰色小字：「Server v3+ 生效，当前仅保存不调度」
- 小字左侧有 InfoFilled 圆形图标（i 图标）

### 记录

- 截图（含 field label + input + 灰色 help 文案）：______

---

## 收尾

三场景全部完成后，按下述任一方式记录结果：

**选项 1（轻量）**：直接把上面三张记录表填好，粘到此文件末尾 `## 执行结果 2026-04-20` 一节。

**选项 2（PR description）**：把三场景结果复制粘贴到 feature/regions-module PR description。

然后我把 T17 标 `[x]` 完成，合并 PR。

---

## 清理（手测后）

如果场景 A 留下了 `smoke_ref_test` region：

```bash
# 先查 id
curl -sS -X POST http://localhost:9821/api/v1/regions/list \
  -H 'Content-Type: application/json' \
  -d '{"region_id":"smoke_ref_test","page":1,"page_size":5}'

# 按 id 删（若已保存成功），或直接忽略（保存失败本就未入库）
curl -sS -X POST http://localhost:9821/api/v1/regions/delete \
  -H 'Content-Type: application/json' -d '{"id":<ID>}'
```

如果场景 A 里临时 disable 了某 NPC，别忘了再 toggle 回 enabled。
