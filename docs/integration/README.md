# 联调集成测试基线

本目录存放 ADMIN ↔ 服务端联调测试使用的基线文件，由 ADMIN 仓 verify 脚本消费。

## 文件

### `snapshot-section-4.json`

**来源**：`../NPC-AI-Behavior-System-Server/docs/integration/admin-snapshot-2026-04-18.md` §4 的 JSON 代码块（PR #18 合并当日从 ADMIN 抓的 `GET /api/configs/npc_templates` 原始响应）

**用途**：`scripts/verify-seed.sh`（T7）对比 `GET /api/configs/npc_templates` 实际导出与本文件做 diff，验证 seed 后数据与 snapshot §4 逐字段一致（R4 / R5 / R13.1）

**同步策略**：服务端仓 snapshot 更新时，手工同步两侧（本 spec 双边契约同步方式的延伸——测试基线归属于消费方 ADMIN，服务端仓的 `admin-snapshot-2026-04-18.md` 是**人读文档**，本 JSON 是**机读基线**，互不阻塞）

## 相关文档

- `docs/architecture/api-contract.md`：npc_templates 导出契约（schema 权威源）
- `docs/specs/external-contract-admin-shape-alignment/`：本基线的来源 spec
