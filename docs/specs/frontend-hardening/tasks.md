# frontend-hardening — 任务列表

## 状态

- [x] T1: List 页 Delete VERSION_CONFLICT 处理（EventTypeList + TemplateList）
- [x] T2: FieldForm.vue 硬编码错误码替换

---

## T1：List 页 Delete VERSION_CONFLICT 处理 (R2, R4)

**涉及文件**：
- `frontend/src/views/EventTypeList.vue`
- `frontend/src/views/TemplateList.vue`

**做什么**：

在两个文件的 `handleDelete` 的 catch 块中，在 `if (err === 'cancel') return` 之后补充 VERSION_CONFLICT 分支。

**EventTypeList.vue**：
```typescript
if ((err as BizError).code === EVENT_TYPE_ERR.VERSION_CONFLICT) {
  ElMessage.warning('数据已更新，请重新操作')
  fetchList()
  return
}
```

**TemplateList.vue**：
```typescript
if ((err as BizError).code === TEMPLATE_ERR.VERSION_CONFLICT) {
  ElMessage.warning('数据已更新，请重新操作')
  fetchList()
  return
}
```

两处均已有对应的 error 常量 import（`EVENT_TYPE_ERR` / `TEMPLATE_ERR`），无需新增 import。

**做完是什么样**：`npm run build` 通过；`npx vue-tsc --noEmit` 无错误。

---

## T2：FieldForm.vue 硬编码错误码替换 (R5)

**涉及文件**：
- `frontend/src/views/FieldForm.vue`

**做什么**：

将第 341 行的裸数字替换为已定义的常量：

```typescript
// Before
if ((err as BizError).code === 40011) {

// After
if ((err as BizError).code === FIELD_ERR.NOT_FOUND) {
```

`FIELD_ERR` 已在该文件现有代码中通过 `import { FIELD_ERR, ... } from '@/api/fields'` 引入，无需新增 import。

**做完是什么样**：文件中无裸数字 `40011`；`npx vue-tsc --noEmit` 无错误；`npm run build` 通过。

---

## 执行顺序

T1 → T2（独立，可任意顺序，建议按优先级先做 T1）
