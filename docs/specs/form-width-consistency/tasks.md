# form-width-consistency — 任务列表

## 状态

- [ ] T1: EventTypeForm.vue — 补 max-width + margin auto
- [ ] T2: FsmStateDictForm.vue — 补 max-width + 去灰底 + 补边框 + 调 padding

---

## T1：EventTypeForm.vue — 补 max-width + margin auto (R1, R5, R6)

**涉及文件**：
- `frontend/src/views/EventTypeForm.vue`（修改）

**做什么**：

在 `<style scoped>` 的 `.form-card` 规则追加 2 行：

```css
.form-card {
  background: #fff;
  border: 1px solid #E4E7ED;
  border-radius: 8px;
  padding: 32px;
  max-width: 800px;       /* ← 新增 */
  margin-left: auto;      /* ← 新增 */
  margin-right: auto;     /* ← 新增 */
}
```

模板不改动。

**做完是什么样**：
- 浏览器打开「事件类型 → 新建」，表单卡片宽度不超过 800px，居中显示
- `npx vue-tsc --noEmit` 通过

---

## T2：FsmStateDictForm.vue — 补 max-width + 去灰底 + 补边框 + 调 padding (R2, R3, R4, R6)

**涉及文件**：
- `frontend/src/views/FsmStateDictForm.vue`（修改）

**做什么**：

修改 `<style scoped>` 中两个规则：

```css
/* 变更前 */
.form-scroll {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
  background: #F5F7FA;
}

.form-card {
  background: #fff;
  border-radius: 8px;
  padding: 24px;
  margin-bottom: 16px;
}

/* 变更后 */
.form-scroll {
  flex: 1;
  overflow-y: auto;
  padding: 24px 32px;
}

.form-card {
  max-width: 800px;
  margin: 0 auto;
  background: #fff;
  border: 1px solid #E4E7ED;
  border-radius: 8px;
  padding: 32px;
}
```

模板不改动。

**做完是什么样**：
- 浏览器打开「状态字典 → 新建状态」，表单卡片最大宽度 800px，居中，有灰色边框，背景白色（不再有灰底）
- 与「字段管理 → 新建字段」视觉一致
- `npx vue-tsc --noEmit` + `npm run build` 通过

---

## 执行顺序

T1 → T2（均为独立 CSS 改动，无依赖，但顺序执行便于逐一验证）
