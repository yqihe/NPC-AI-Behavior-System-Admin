<template>
  <div class="constraint-panel">
    <div class="constraint-title">
      <el-tag size="small" type="info">select</el-tag>
      <span class="constraint-label">选择类型 — 约束配置</span>
    </div>
    <div v-if="restricted" class="constraint-warn">
      <el-icon><WarningFilled /></el-icon>
      已被引用，约束只能放宽不能收紧（不可删除已有选项）
    </div>

    <!-- 选项列表 -->
    <div class="options-header">
      <span class="options-label">选项列表</span>
      <el-link v-if="!disabled" type="primary" :underline="false" @click="addOption">
        <el-icon><Plus /></el-icon>
        添加选项
      </el-link>
    </div>

    <div class="options-table">
      <div class="options-col-header">
        <span class="col-value">值 (value)</span>
        <span class="col-label">标签 (label)</span>
        <span class="col-del"></span>
      </div>
      <div
        v-for="(opt, idx) in options"
        :key="idx"
        class="option-row"
      >
        <el-input
          :model-value="opt.value"
          placeholder="选项值"
          size="default"
          @update:model-value="(v: string) => updateOption(idx, 'value', v)"
        />
        <el-input
          :model-value="opt.label"
          placeholder="显示标签"
          size="default"
          @update:model-value="(v: string) => updateOption(idx, 'label', v)"
        />
        <el-icon v-if="!disabled" class="del-icon" @click="removeOption(idx)"><Delete /></el-icon>
      </div>
      <div v-if="options.length === 0" class="options-empty">
        暂无选项，请点击「添加选项」
      </div>
    </div>

    <!-- 选择数量 -->
    <el-row :gutter="16" style="margin-top: 16px">
      <el-col :span="12">
        <div class="constraint-field">
          <label class="constraint-field-label">最少选择数</label>
          <el-input-number
            :model-value="constraints.minSelect"
            :controls="false"
            :min="0"
            placeholder="默认 1"
            style="width: 100%"
            @update:model-value="(v: number | null | undefined) => updateField('minSelect', v)"
          />
        </div>
      </el-col>
      <el-col :span="12">
        <div class="constraint-field">
          <label class="constraint-field-label">最多选择数</label>
          <el-input-number
            :model-value="constraints.maxSelect"
            :controls="false"
            :min="0"
            placeholder="默认 1"
            style="width: 100%"
            @update:model-value="(v: number | null | undefined) => updateField('maxSelect', v)"
          />
        </div>
      </el-col>
    </el-row>

    <!-- 提示 -->
    <div class="select-hint">
      <el-icon><InfoFilled /></el-icon>
      min=1, max=1 为单选；max&gt;1 为多选。默认值自动取第一个选项。
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { WarningFilled, Plus, Delete, InfoFilled } from '@element-plus/icons-vue'

interface SelectOption {
  value: string
  label: string
}

interface SelectConstraints {
  options?: SelectOption[]
  minSelect?: number
  maxSelect?: number
  [key: string]: unknown
}

const props = defineProps<{
  modelValue?: SelectConstraints
  restricted?: boolean
  disabled?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: SelectConstraints]
}>()

const constraints = computed((): SelectConstraints => props.modelValue || {})
const options = computed((): SelectOption[] => constraints.value.options || [])

function emitUpdate(patch: Partial<SelectConstraints>) {
  emit('update:modelValue', { ...constraints.value, ...patch })
}

function addOption() {
  const newOptions = [...options.value, { value: '', label: '' }]
  emitUpdate({ options: newOptions })
}

function removeOption(idx: number) {
  const newOptions = options.value.filter((_, i) => i !== idx)
  emitUpdate({ options: newOptions })
}

function updateOption(idx: number, key: keyof SelectOption, val: string) {
  const newOptions = options.value.map((opt, i) =>
    i === idx ? { ...opt, [key]: val } : opt,
  )
  emitUpdate({ options: newOptions })
}

function updateField(key: string, val: number | null | undefined) {
  const next = { ...constraints.value }
  if (val === null || val === undefined) {
    delete next[key]
  } else {
    next[key] = val
  }
  emit('update:modelValue', next)
}

/** 供父组件调用的校验方法 */
function validate(): string | null {
  if (options.value.length === 0) {
    return '选择类型至少需要一个选项'
  }
  for (let i = 0; i < options.value.length; i++) {
    if (!options.value[i].value.trim()) {
      return `第 ${i + 1} 个选项的值不能为空`
    }
  }
  const values = options.value.map((o) => o.value.trim())
  if (new Set(values).size !== values.length) {
    return '选项值不能重复'
  }
  const minSel = constraints.value.minSelect as number | undefined
  const maxSel = constraints.value.maxSelect as number | undefined
  if (minSel !== undefined && maxSel !== undefined && minSel > maxSel) {
    return '最少选择数不能大于最多选择数'
  }
  return null
}

defineExpose({ validate })
</script>

<style scoped>
.constraint-panel {
  border: 1px solid #E4E7ED;
  border-radius: 8px;
  padding: 24px;
  background: #fff;
}

.constraint-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 16px;
}

.constraint-label {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
}

.constraint-warn {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-bottom: 12px;
  font-size: 12px;
  color: #E6A23C;
}

.options-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.options-label {
  font-size: 13px;
  color: #909399;
}

.options-table {
  margin-bottom: 8px;
}

.options-col-header {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}

.col-value,
.col-label {
  flex: 1;
  font-size: 12px;
  color: #909399;
}

.col-del {
  width: 16px;
}

.option-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 8px;
}

.option-row .el-input {
  flex: 1;
}

.del-icon {
  color: #F56C6C;
  cursor: pointer;
  font-size: 16px;
  flex-shrink: 0;
}

.options-empty {
  text-align: center;
  color: #C0C4CC;
  font-size: 13px;
  padding: 12px 0;
}

.constraint-field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.constraint-field-label {
  font-size: 13px;
  color: #909399;
}

.select-hint {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 12px;
  padding: 8px 12px;
  background: #F0F9EB;
  border-radius: 4px;
  font-size: 12px;
  color: #67C23A;
}
</style>
