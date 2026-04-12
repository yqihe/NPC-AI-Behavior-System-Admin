<template>
  <div class="selected-card" :class="{ disabled }">
    <el-empty
      v-if="selectedFields.length === 0"
      description="请在上方字段选择卡中勾选字段"
      :image-size="80"
    />
    <el-table
      v-else
      :data="selectedFields"
      :row-class-name="rowClassName"
      border
    >
      <el-table-column label="字段标签" min-width="180">
        <template #default="{ row }: { row: TemplateFieldItem }">
          <div class="label-cell">
            <el-icon v-if="!row.enabled" class="warn-icon">
              <WarningFilled />
            </el-icon>
            <span>{{ row.label }}</span>
            <el-tag v-if="!row.enabled" size="small" type="warning">已禁用</el-tag>
          </div>
        </template>
      </el-table-column>
      <el-table-column prop="name" label="字段标识" min-width="180" />
      <el-table-column label="类型" width="110">
        <template #default="{ row }: { row: TemplateFieldItem }">
          <el-tag size="small" type="info">{{ row.type }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="必填" width="80" align="center">
        <template #default="{ row }: { row: TemplateFieldItem }">
          <el-checkbox
            :model-value="row.required"
            :disabled="disabled"
            @change="(v: string | number | boolean) => emit('update:required', row.field_id, Boolean(v))"
          />
        </template>
      </el-table-column>
      <el-table-column label="排序" width="90" align="center">
        <template #default="{ $index }: { $index: number }">
          <div class="sort-cell">
            <span
              class="sort-btn"
              :class="{ 'sort-btn-disabled': disabled || $index === 0 }"
              @click="moveUp($index)"
            >
              <el-icon><ArrowUp /></el-icon>
            </span>
            <span
              class="sort-btn"
              :class="{
                'sort-btn-disabled':
                  disabled || $index === selectedFields.length - 1,
              }"
              @click="moveDown($index)"
            >
              <el-icon><ArrowDown /></el-icon>
            </span>
          </div>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup lang="ts">
import { WarningFilled, ArrowUp, ArrowDown } from '@element-plus/icons-vue'
import type { TemplateFieldItem } from '@/api/templates'

const props = defineProps<{
  selectedFields: TemplateFieldItem[]
  disabled?: boolean
}>()

const emit = defineEmits<{
  'update:order': [newOrder: number[]]
  'update:required': [fieldId: number, required: boolean]
}>()

function rowClassName({ row }: { row: TemplateFieldItem }) {
  return row.enabled ? '' : 'row-field-disabled'
}

function moveUp(index: number) {
  if (index <= 0) return
  const ids = props.selectedFields.map((f) => f.field_id)
  const [moved] = ids.splice(index, 1)
  ids.splice(index - 1, 0, moved)
  emit('update:order', ids)
}

function moveDown(index: number) {
  if (index >= props.selectedFields.length - 1) return
  const ids = props.selectedFields.map((f) => f.field_id)
  const [moved] = ids.splice(index, 1)
  ids.splice(index + 1, 0, moved)
  emit('update:order', ids)
}
</script>

<style scoped>
.selected-card {
  background: #fff;
  border: 1px solid #ebeef5;
  border-radius: 6px;
  padding: 16px;
}

.selected-card.disabled {
  opacity: 0.55;
  pointer-events: none;
}

.label-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.warn-icon {
  color: #E6A23C;
  font-size: 16px;
}

:deep(.row-field-disabled) {
  opacity: 0.55;
}

.sort-cell {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 14px;
}

.sort-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  color: #409EFF;
  cursor: pointer;
  border-radius: 4px;
  transition: background-color 0.15s;
}

.sort-btn .el-icon {
  font-size: 14px;
}

.sort-btn:hover {
  background: #ECF5FF;
}

.sort-btn-disabled {
  color: #C0C4CC;
  cursor: not-allowed;
  pointer-events: none;
}
</style>
