# frontend-hardening — 需求分析

## 动机

前端各模块的错误处理深度不一致。以 `FieldList.vue` 为基准（最完整）：

- Delete 操作：按 `REF_DELETE` 弹引用详情弹窗，按 `DELETE_NOT_DISABLED` 弹引导停用弹窗，按 `VERSION_CONFLICT` 刷新提示
- Form 操作：所有业务错误码都有对应的中文提示或 UI 动作

但其他模块的 Delete catch 只有：

```typescript
} catch (err: unknown) {
  if (err === 'cancel') return
  // 其他错误拦截器已 toast
}
```

这意味着删除被拒时前端只有拦截器的通用 toast，无法引导用户下一步操作（如先停用再删除、先解除引用再删除）。

除错误处理外，`FieldForm.vue` 中存在一处硬编码错误码数字，违反 red-line §4（禁止硬编码）。

不修则：
1. 用户触发"删除已启用项"或"版本冲突"时只看到通用报错，无操作引导，体验差
2. 后端错误码枚举随版本变化时硬编码的数字会静默失效

---

## 优先级

**中**。当前功能可用，但 UX 存在已知缺口。FSM 前端开发前修完，避免新模块继续复制旧模式。

---

## 预期效果

### 场景 1：EventTypeList 删除已启用的事件类型

修复前：返回 42012（DELETE_NOT_DISABLED）→ 通用 toast "删除前必须先停用"，用户不知道怎么操作。

修复后：弹出 `EnabledGuardDialog`，提示"该事件类型正在启用中，需先停用才能删除"，提供"立即停用"按钮。

### 场景 2：EventTypeList 删除发生版本冲突

修复前：返回 42010（VERSION_CONFLICT）→ 通用 toast，列表数据仍为旧版本，用户再次点击删除还会冲突。

修复后：toast "数据已更新，请重新操作" + 自动刷新列表。

### 场景 3：TemplateList 删除已启用的模板

与场景 1 同构，对应 TEMPLATE_ERR.DELETE_NOT_DISABLED (41009)。

### 场景 4：TemplateList 删除发生版本冲突

与场景 2 同构，对应 TEMPLATE_ERR.VERSION_CONFLICT (41011)。

### 场景 5：FieldForm 字段不存在错误

修复前：`if ((err as BizError).code === 40011)` — 硬编码数字。

修复后：`if ((err as BizError).code === FIELD_ERR.NOT_FOUND)` — 使用已定义常量。

---

## 依赖分析

### 依赖的已完成工作

- `EnabledGuardDialog.vue`：已实现，支持 `entityType` prop 切换文案
- `FIELD_ERR` / `EVENT_TYPE_ERR` / `TEMPLATE_ERR` 常量：已在对应 `api/*.ts` 文件中定义
- `FieldList.vue`：金标准实现，直接参照

### 谁依赖这个需求

- **fsm-frontend**（后续 spec）：FSM 前端模块以修复后的模式为参照，不继承旧模式

---

## 改动范围

| 文件 | 改动内容 |
|---|---|
| `frontend/src/views/EventTypeList.vue` | Delete catch 补充 `DELETE_NOT_DISABLED`（EnabledGuardDialog）+ `VERSION_CONFLICT`（刷新） |
| `frontend/src/views/TemplateList.vue` | Delete catch 补充 `DELETE_NOT_DISABLED`（EnabledGuardDialog）+ `VERSION_CONFLICT`（刷新） |
| `frontend/src/views/FieldForm.vue` | 第 341 行 `40011` → `FIELD_ERR.NOT_FOUND` |

预估：3 个文件，净改动 < 20 行。

---

## 扩展轴检查

- **新增配置类型**：正面影响。修复后的 List.vue 错误处理模式成为新模块的标准参照，避免"复制旧代码"时带入缺陷。
- **新增表单字段**：不涉及。

---

## 验收标准

**R1**：`EventTypeList.vue` Delete catch 对 `EVENT_TYPE_ERR.DELETE_NOT_DISABLED` 弹出 `EnabledGuardDialog`，行为与 FieldList 中的同类逻辑一致。

**R2**：`EventTypeList.vue` Delete catch 对 `EVENT_TYPE_ERR.VERSION_CONFLICT` 调用 `fetchList()` 并 toast "数据已更新，请重新操作"。

**R3**：`TemplateList.vue` Delete catch 对 `TEMPLATE_ERR.DELETE_NOT_DISABLED` 弹出 `EnabledGuardDialog`。

**R4**：`TemplateList.vue` Delete catch 对 `TEMPLATE_ERR.VERSION_CONFLICT` 调用 `fetchList()` 并 toast "数据已更新，请重新操作"。

**R5**：`FieldForm.vue` 第 341 行改用 `FIELD_ERR.NOT_FOUND` 常量，不再出现裸数字 `40011`。

**R6**：`npm run build` 通过（含 TypeScript 类型检查）；`npx vue-tsc --noEmit` 无错误。

验证方法：代码审查 + 构建通过 + 浏览器手动验证（删除已启用项 → 弹窗出现）。

---

## 不做什么

1. **不处理 EventTypeSchemaList** 的错误处理——该模块现有处理已正确（有 DELETE_NOT_DISABLED + VERSION_CONFLICT）
2. **不处理 TemplateList.vue 的 REF_DELETE（41007）**——注释明确标注"NPC 上线后启用"，有意延迟
3. **不处理 EventTypeList 的 REF_DELETE（42008）**——当前通用 toast 已足够，无引用详情弹窗需求
4. **不重构错误处理为通用 composable**——DRY 优化，非 bug，延后到 FSM 前端后再评估是否值得
5. **不修改其他 Form 页面**——未发现其他同类硬编码
