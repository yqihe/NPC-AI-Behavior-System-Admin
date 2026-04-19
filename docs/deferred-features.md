# ADMIN 已知缺陷与延期项

记录已识别但未排期修复的产品缺陷或限制。每条记录包含：现状、影响、触发升级条件、引用。

---

## 跨字段联动校验缺席

- **现状**：fields 系统每个 field 的 `properties.constraints` 独立配置，无跨字段联动校验机制（如"field A=true 时要求 field B=true"）。
- **影响**：`enable_emotion=true ∧ enable_memory=false` 等非法组合 ADMIN 侧拒绝不了，靠服务端启动 fatal 兜底拦截。运营侧无写时反馈。
- **触发升级条件**：跨字段约束积累 ≥ 3 条，或运营误配触发线上 fatal ≥ 2 次。
- **引用**：`docs/architecture/api-contract.md` v1.1 §组件 opt-in 依赖矩阵
