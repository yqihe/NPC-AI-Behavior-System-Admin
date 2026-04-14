# frontend-hardening — 设计方案

## 方案描述

### 核心思路

三处独立的小修复，无交叉依赖。最小侵入，以 `FieldList.vue` 的处理模式为参照。

---

### 修复 1：EventTypeList.vue Delete — VERSION_CONFLICT 处理

**发现**：`DELETE_NOT_DISABLED` 已通过前置 `if (row.enabled)` 守卫处理（符合 FieldList 模式）；缺失的是 `VERSION_CONFLICT`——用户在列表数据过期时触发删除，后端返回 42010，当前前端不刷新列表，用户再次点击还会冲突。

**修复**：catch 中补充 VERSION_CONFLICT 分支：

```typescript
// Before
} catch (err: unknown) {
  if (err === 'cancel') return
  // 其他错误拦截器已 toast
}

// After
} catch (err: unknown) {
  if (err === 'cancel') return
  if ((err as BizError).code === EVENT_TYPE_ERR.VERSION_CONFLICT) {
    ElMessage.warning('数据已更新，请重新操作')
    fetchList()
    return
  }
  // 其他错误拦截器已 toast
}
```

---

### 修复 2：TemplateList.vue Delete — VERSION_CONFLICT 处理

与修复 1 完全同构，使用 `TEMPLATE_ERR.VERSION_CONFLICT`：

```typescript
// After
} catch (err: unknown) {
  if (err === 'cancel') return
  if ((err as BizError).code === TEMPLATE_ERR.VERSION_CONFLICT) {
    ElMessage.warning('数据已更新，请重新操作')
    fetchList()
    return
  }
  // REF_DELETE(41007) 占位：NPC 上线后启用
  // 其他错误拦截器已 toast
}
```

---

### 修复 3：FieldForm.vue — 硬编码错误码替换

**位置**：`frontend/src/views/FieldForm.vue` 第 341 行。

```typescript
// Before
if ((err as BizError).code === 40011) {

// After
if ((err as BizError).code === FIELD_ERR.NOT_FOUND) {
```

`FIELD_ERR` 已在该文件中通过 `import { FIELD_ERR } from '@/api/fields'` 引入（现有代码中其他处已使用），无需新增 import。

---

## 方案对比

### 备选方案：提取通用 handleDeleteError composable

把删除错误处理逻辑抽成可复用的 composable，供所有列表页使用。

**不选原因**：
1. requirements 明确"不重构为通用 composable，延后到 FSM 前端后评估"
2. 每个模块的错误码不同（`FIELD_ERR` / `EVENT_TYPE_ERR` / `TEMPLATE_ERR`），复用收益有限
3. 当前只有 2 处同类修复，不达到"三处以上"的提取门槛

---

## 红线检查

| 红线 | 检查结果 |
|---|---|
| §4（禁止硬编码错误码）| ✓ 修复 3 消除唯一的裸数字用法 |
| §13（错误码漏处理）| ✓ 补充 VERSION_CONFLICT 后，Delete 操作的核心错误码均有处理 |
| Frontend：ref 用 .value | ✓ 不涉及 |
| Frontend：el-form disabled | ✓ 不涉及 |

**需求与实现的修正**：requirements.md 中 R1（EventTypeList DELETE_NOT_DISABLED）和 R3（TemplateList DELETE_NOT_DISABLED）经代码核查发现**已通过前置 enabled 守卫处理**，无需在 catch 中重复处理。设计阶段纠正，实际只执行 VERSION_CONFLICT 修复 + 硬编码修复（共 3 项）。

---

## 扩展性影响

- **新增配置类型**：正面。修复后的 List.vue 模式（enabled 守卫 + VERSION_CONFLICT catch）成为 FSM 前端等新模块的标准参照。

---

## 依赖方向

```
EventTypeList.vue → eventTypes.ts (EVENT_TYPE_ERR)
TemplateList.vue  → templates.ts  (TEMPLATE_ERR)
FieldForm.vue     → fields.ts     (FIELD_ERR) ← 已 import，只改引用
```

---

## 陷阱检查

### 前端
- `FIELD_ERR` 在 `FieldForm.vue` 已有其他使用处，确认已 import，不需要新增 import 语句。✓
- `EVENT_TYPE_ERR.VERSION_CONFLICT = 42010`、`TEMPLATE_ERR.VERSION_CONFLICT = 41011`，均已在对应 `api/*.ts` 文件中定义，无需新增常量。✓
- `fetchList()` 是 async 函数，catch 中调用不 await（与现有模式一致，不阻塞用户操作）。✓

---

## 配置变更

无。

---

## 测试策略

**构建验证**：`npm run build` + `npx vue-tsc --noEmit` 通过。

**浏览器验证**：
1. 两个浏览器标签同时打开 EventTypeList，Tab A 删除某条目（先停用），Tab B 同时也触发删除同一条目 → Tab B 应 toast "数据已更新，请重新操作" 并刷新列表
2. FieldForm 打开一个字段详情，在另一处将该字段删除，回到 FieldForm 提交 → 应跳转到列表页
