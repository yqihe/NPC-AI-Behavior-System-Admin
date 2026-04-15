<template>
  <div class="state-list-editor">
    <!-- 加载中 -->
    <div v-if="loadingDicts" class="dicts-loading">
      <el-icon class="is-loading"><Loading /></el-icon>
      加载状态字典...
    </div>

    <!-- 状态列表 -->
    <div v-else class="state-rows">
      <div
        v-for="(name, idx) in modelValue"
        :key="idx"
        class="state-row"
      >
        <el-select
          :model-value="name"
          :disabled="disabled"
          placeholder="选择状态"
          style="width: 280px"
          filterable
          :class="{ 'is-error': isDuplicate(name, idx) }"
          @change="(v: string) => updateName(idx, v)"
        >
          <el-option
            v-for="s in availableStates"
            :key="s.name"
            :label="`${s.display_name}（${s.name}）`"
            :value="s.name"
            :disabled="isSelectedElsewhere(s.name, idx)"
          />
        </el-select>
        <span v-if="isDuplicate(name, idx)" class="dup-hint">重名</span>
        <el-button
          v-if="!disabled"
          type="danger"
          link
          size="small"
          style="margin-left: 8px"
          @click="removeName(idx)"
        >
          删除
        </el-button>
      </div>

      <!-- 空状态提示 -->
      <div v-if="modelValue.length === 0 && !disabled" class="empty-hint">
        点击下方按钮从状态字典中添加状态
      </div>
    </div>

    <!-- 添加按钮 -->
    <el-button
      v-if="!disabled && !loadingDicts"
      text
      type="primary"
      :disabled="availableStates.length === 0"
      @click="addName"
    >
      + 添加状态
    </el-button>
    <div v-if="!loadingDicts && availableStates.length === 0" class="no-dicts-hint">
      暂无已启用的状态字典，请先在"状态字典管理"中创建并启用
    </div>

    <!-- 初始状态选择 -->
    <div v-if="modelValue.length > 0" class="initial-row">
      <span class="initial-label">初始状态</span>
      <el-select
        :model-value="initialState"
        :disabled="disabled"
        placeholder="选择初始状态"
        style="width: 280px"
        @change="(v: string) => emit('update:initialState', v)"
      >
        <el-option
          v-for="name in validNames"
          :key="name"
          :label="labelOf(name)"
          :value="name"
        />
      </el-select>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Loading } from '@element-plus/icons-vue'

export interface StateDictOption {
  name: string
  display_name: string
}

const props = defineProps<{
  modelValue: string[]
  initialState: string
  disabled?: boolean
  availableStates: StateDictOption[]
  loadingDicts?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string[]]
  'update:initialState': [value: string]
}>()

const validNames = computed(() =>
  props.modelValue.filter((n, idx) => n && !isDuplicate(n, idx)),
)

function labelOf(name: string): string {
  const found = props.availableStates.find((s) => s.name === name)
  return found ? `${found.display_name}（${name}）` : name
}

function isDuplicate(name: string, idx: number): boolean {
  if (!name) return false
  return props.modelValue.some((n, i) => i !== idx && n === name)
}

function isSelectedElsewhere(name: string, currentIdx: number): boolean {
  return props.modelValue.some((n, i) => i !== currentIdx && n === name)
}

function addName() {
  emit('update:modelValue', [...props.modelValue, ''])
}

function removeName(idx: number) {
  const next = props.modelValue.filter((_, i) => i !== idx)
  emit('update:modelValue', next)

  if (props.modelValue[idx] === props.initialState) {
    const firstValid = next.find((n) => n)
    emit('update:initialState', firstValid || '')
  }
}

function updateName(idx: number, val: string) {
  const next = props.modelValue.map((n, i) => (i === idx ? val : n))
  emit('update:modelValue', next)

  if (props.modelValue[idx] === props.initialState) {
    emit('update:initialState', val)
  }
}
</script>

<style scoped>
.state-list-editor {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.state-rows {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.state-row {
  display: flex;
  align-items: center;
  gap: 4px;
}

:deep(.el-select.is-error .el-select__wrapper) {
  box-shadow: 0 0 0 1px #F56C6C inset;
}

.dup-hint {
  font-size: 12px;
  color: #F56C6C;
  flex-shrink: 0;
}

.dicts-loading {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 13px;
  color: #909399;
}

.empty-hint {
  font-size: 13px;
  color: #C0C4CC;
  padding: 4px 0;
}

.no-dicts-hint {
  font-size: 12px;
  color: #E6A23C;
  margin-top: 2px;
}

.initial-row {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-top: 8px;
  padding-top: 12px;
  border-top: 1px solid #F0F0F0;
}

.initial-label {
  font-size: 13px;
  color: #303133;
  font-weight: 500;
  flex-shrink: 0;
}
</style>
