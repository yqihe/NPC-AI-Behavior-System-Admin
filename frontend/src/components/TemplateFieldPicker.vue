<template>
  <div class="picker-card" :class="{ disabled }">
    <el-empty
      v-if="groupedFields.length === 0"
      description="暂无启用字段，请先在字段管理中创建并启用"
      :image-size="80"
    />
    <div v-for="g in groupedFields" :key="g.category" class="group">
      <div class="group-header">
        <span class="group-title">{{ g.label }}</span>
        <span class="group-count">({{ g.selectedCount }}/{{ g.fields.length }})</span>
      </div>
      <div class="grid">
        <div
          v-for="f in g.fields"
          :key="f.id"
          class="cell"
          :class="{
            selected: isSelected(f.id),
            reference: f.type === 'reference',
            'has-sub-selected': f.type === 'reference' && hasAnySubSelected(f.id),
            disabled,
          }"
          @click="onCellClick(f)"
        >
          <div
            v-if="f.type !== 'reference'"
            class="checkbox"
            :class="{ checked: isSelected(f.id) }"
          >
            <el-icon v-if="isSelected(f.id)"><Check /></el-icon>
          </div>
          <el-icon v-else class="ref-icon"><Link /></el-icon>
          <span class="cell-label">{{ f.label }}</span>
          <span class="cell-meta">{{ f.name }} · {{ f.type }}</span>
          <el-icon v-if="f.type === 'reference'" class="chevron">
            <ArrowRight />
          </el-icon>
        </div>
      </div>
    </div>

    <TemplateRefPopover
      ref="popoverRef"
      :readonly="disabled"
      @confirm="onPopoverConfirm"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { Check, ArrowRight, Link } from '@element-plus/icons-vue'
import TemplateRefPopover from './TemplateRefPopover.vue'
import type { FieldListItem } from '@/api/fields'

interface FieldGroup {
  category: string
  label: string
  fields: FieldListItem[]
  selectedCount: number
}

const props = defineProps<{
  fieldPool: FieldListItem[]
  disabled?: boolean
  /** create 模式下 reference 子字段选择器过滤停用子字段；edit/view 保留 */
  mode?: 'create' | 'edit' | 'view'
}>()

const isCreateMode = computed(() => props.mode === 'create')

const selectedIds = defineModel<number[]>('selectedIds', { required: true })

const popoverRef = ref<InstanceType<typeof TemplateRefPopover> | null>(null)
// 记录当前 popover 正在处理哪个 reference 字段，用于 confirm 时差集清理
const pendingRefFieldId = ref<number | null>(null)
// 缓存每个 reference 字段的 allSubIds（由 popover 确认时更新），用于子字段高亮
const refFieldSubIdsMap = ref<Record<number, number[]>>({})

const groupedFields = computed<FieldGroup[]>(() => {
  const map = new Map<string, FieldGroup>()
  for (const f of props.fieldPool) {
    let g = map.get(f.category)
    if (!g) {
      g = {
        category: f.category,
        label: f.category_label || f.category,
        fields: [],
        selectedCount: 0,
      }
      map.set(f.category, g)
    }
    g.fields.push(f)
    if (f.type !== 'reference' && selectedIds.value.includes(f.id)) {
      g.selectedCount++
    }
  }
  return Array.from(map.values())
})

function isSelected(id: number): boolean {
  return selectedIds.value.includes(id)
}

function hasAnySubSelected(refFieldId: number): boolean {
  const subs = refFieldSubIdsMap.value[refFieldId]
  if (!subs || subs.length === 0) return false
  return subs.some((id) => selectedIds.value.includes(id))
}

function onCellClick(f: FieldListItem) {
  if (props.disabled) return
  if (f.type === 'reference') {
    pendingRefFieldId.value = f.id
    popoverRef.value?.open(f, selectedIds.value, isCreateMode.value)
    return
  }
  if (isSelected(f.id)) {
    selectedIds.value = selectedIds.value.filter((id) => id !== f.id)
  } else {
    selectedIds.value = [...selectedIds.value, f.id]
  }
}

function onPopoverConfirm(payload: {
  allSubIds: number[]
  selectedSubIds: number[]
}) {
  const { allSubIds, selectedSubIds } = payload
  // 记住这个 reference 的子字段集合，用于下次展示高亮
  if (pendingRefFieldId.value !== null) {
    refFieldSubIdsMap.value = {
      ...refFieldSubIdsMap.value,
      [pendingRefFieldId.value]: allSubIds,
    }
  }
  pendingRefFieldId.value = null
  // 差集清理：先从 selectedIds 里移除本 reference 负责的所有子字段，再合并本次勾选
  const withoutSubs = selectedIds.value.filter((id) => !allSubIds.includes(id))
  selectedIds.value = Array.from(new Set([...withoutSubs, ...selectedSubIds]))
}
</script>

<style scoped>
.picker-card {
  display: flex;
  flex-direction: column;
  gap: 24px;
  padding: 20px;
  background: #fff;
  border: 1px solid #ebeef5;
  border-radius: 6px;
}

.picker-card.disabled {
  opacity: 0.55;
  pointer-events: none;
}

.group {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.group-header {
  font-size: 13px;
  font-weight: 600;
  color: #303133;
  padding-bottom: 8px;
  border-bottom: 1px dashed #ebeef5;
}

.group-count {
  margin-left: 8px;
  font-size: 12px;
  color: #909399;
  font-weight: 400;
}

.grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 12px;
}

.cell {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 12px;
  height: 40px;
  background: #fff;
  border: 1px solid #dcdfe6;
  border-radius: 4px;
  cursor: pointer;
  user-select: none;
  transition: border-color 0.2s;
}

.cell:hover {
  border-color: #409eff;
}

.cell.reference {
  border-color: #9575cd;
  background: #f7f3fd;
}

.cell.reference:hover {
  border-color: #7e57c2;
}

.cell.reference.has-sub-selected {
  background: #ede7f6;
}

.cell .checkbox {
  width: 16px;
  height: 16px;
  border: 1px solid #dcdfe6;
  border-radius: 2px;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.cell .checkbox.checked {
  background: #409eff;
  border-color: #409eff;
  color: #fff;
  font-size: 12px;
}

.cell .ref-icon {
  color: #9575cd;
  font-size: 14px;
  flex-shrink: 0;
}

.cell-label {
  font-size: 13px;
  color: #303133;
  font-weight: 500;
  flex-shrink: 0;
}

.cell-meta {
  font-size: 11px;
  color: #909399;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  flex: 1;
}

.cell .chevron {
  color: #9575cd;
  font-size: 14px;
  flex-shrink: 0;
}

.cell.disabled {
  cursor: not-allowed;
}
</style>
