# regions-module T17 前端手测执行结果

> 执行日期：2026-04-20
> 方式：通过 MCP Playwright 自动化驱动 admin-frontend :3000（host.docker.internal）
> 覆盖 [design.md §8.2](design.md) #5/#6 + T15 help-text 截图 = 3 场景

---

## 场景 A：spawn_entry 47007 红点分支 ✅

**设置**：
- `wolf_common` (NPC id=1) 先经 API `toggle-enabled` 置为 disable
- 新建 region `smoke_ref_test_a` 含 2 entry：Entry1 引 `villager_guard`（enabled）/ Entry2 引 `wolf_common`（disabled）

**点击 保存 后观察**：
- Element Plus `<alert>` toast 触发（accessibility tree `ref=e498`, `ref=e503`）
- 文案：**"spawn_table 引用的 NPC 模板未启用: [wolf_common]"**
- 对应后端 errcode.ErrRegionTemplateRefDisabled (47007) 中文透传
- 请求未落盘（region 未创建，列表不含 smoke_ref_test_a）
- 表单保留在 /regions/create 页，用户可修改后重试

**结论**：后端 47007 触发、前端拦截器成功 toast 中文错误、用户可感知并修正。与 T15 错误分支实现一致（`bizErr.code === REGION_ERR.TEMPLATE_REF_DISABLED` 走 `ElMessage.error(bizErr.message)`）。

**截图**：playwright-output/page-2026-04-20T15-17-43-868Z.png（含 Entry 2 wolf_common 选中 + 保存按钮）

---

## 场景 B：乐观锁 47010 弹窗 ✅

**设置**：
- API 创建禁用状态 region `smoke_opt_lock`（id=8, version=1）
- 打开 2 个浏览器 tab 均指向 `/regions/8/edit`
- 两 tab 的 form 初始都加载到 `version=1`

**操作顺序**：
1. Tab 2：中文名改「Tab2 改动」→ 保存 → 成功（redirect `/regions`，toast「保存成功」，列表 id=8 行 display_name 变为 "Tab2 改动"）→ DB `version=2`
2. Tab 1：中文名改「Tab1 改动」→ 保存 → 后端返 47010（WHERE version=1 零行影响）

**Tab 1 保存后观察**：
- `<dialog>` 「版本冲突」弹出（accessibility tree `ref=e190`）
- 正文：**"数据已被其他人修改，请刷新后重试。"**
- 辅助 toast：「该区域已被其他人修改，请刷新后重试」（request.ts 拦截器通用文案）
- 点 OK 后 dialog 关闭，表单留原位（不自动跳转，符合设计）

**结论**：乐观锁正常阻止后写、前端双通道提示（MessageBox + toast）、对应 T15 实现 `bizErr.code === REGION_ERR.VERSION_CONFLICT` 走 `ElMessageBox.alert(...)`。

**截图**：playwright-output/page-2026-04-20T15-21-18-809Z.png（版本冲突弹窗 + OK 按钮清晰）

---

## 场景 C：respawn_seconds help-text ✅

**操作**：`/regions/create` → 点「添加 Spawn Entry」→ 观察 Entry 1 的「重生间隔」Form Item。

**可见内容**（accessibility tree `ref=e237` / `ref=e404`）：
- label：**"重生间隔"**
- spinbutton（默认值 `0`，min=0 step=1）
- 单位后缀：**"秒"**
- 灰色 help-text 行：**"Server v3+ 生效，当前仅保存不调度"** + InfoFilled 图标前缀

**结论**：help-text 显式标注 "当前仅保存不调度"，对策划解释了 `respawn_seconds` 写入 DB 但 Server 当前版本不消费，属 v3+ roadmap 占位。与 T15 实现一致。

**截图**：playwright-output/element-2026-04-20T15-12-26-945Z.png（含 label + input + 秒后缀 + help-text）

---

## 清理

| 步骤 | 命令 | 结果 |
|------|------|------|
| 删 smoke_opt_lock (id=8) | `POST /regions/delete {id:8}` | 200 (label=Tab2 改动) |
| wolf_common 还原 enabled | `POST /npcs/toggle-enabled {id:1,enabled:true,version:2}` | 200 |
| /api/configs/regions 回归 | `GET` | 200 |

smoke_ref_test_a 未落盘（47007 拦截在前），无需清理。

---

## 汇总

| # | 场景 | 结果 |
|---|------|------|
| A (§8.2 #5) | spawn_entry 47007 前端 toast | ✅ |
| B (§8.2 #6) | 乐观锁 47010 MessageBox | ✅ |
| C (T15 特有) | respawn_seconds help-text 可见 | ✅ |

T17 前端 3/3 PASS。regions-module spec 全量闭环 — T1-T17 全 `[x]`。
