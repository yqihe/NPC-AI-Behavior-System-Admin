<template>
  <div class="state-list-editor">
    <!-- 状态列表 -->
    <div class="state-rows">
      <div
        v-for="(name, idx) in modelValue"
        :key="idx"
        class="state-row"
      >
        <el-input
          :model-value="name"
          :disabled="disabled"
          placeholder="状态名（如 idle）"
          :class="{ 'is-error': isDuplicate(name, idx) }"
          style="width: 240px"
          @input="(v: string) => updateName(idx, v)"
        />
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
    </div>

    <!-- 添加按钮 -->
    <el-button
      v-if="!disabled"
      text
      type="primary"
      @click="addName"
    >
      + 添加状态
    </el-button>

    <!-- 初始状态选择 -->
    <div v-if="modelValue.length > 0" class="initial-row">
      <span class="initial-label">初始状态</span>
      <el-select
        :model-value="initialState"
        :disabled="disabled"
        placeholder="选择初始状态"
        style="width: 240px"
        @change="(v: string) => emit('update:initialState', v)"
      >
        <el-option
          v-for="name in validNames"
          :key="name"
          :label="name"
          :value="name"
        />
      </el-select>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  modelValue: string[]        // state name 数组
  initialState: string
  disabled?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string[]]
  'update:initialState': [value: string]
}>()

// 非空且无重名的 name 列表（用于 initial_state 下拉）
const validNames = computed(() =>
  props.modelValue.filter((n, idx) => n && !isDuplicate(n, idx)),
)

function isDuplicate(name: string, idx: number): boolean {
  if (!name) return false
  return props.modelValue.some((n, i) => i !== idx && n === name)
}

function addName() {
  emit('update:modelValue', [...props.modelValue, ''])
}

function removeName(idx: number) {
  const next = props.modelValue.filter((_, i) => i !== idx)
  emit('update:modelValue', next)

  // 若删除的是当前 initial_state，重置为第一个有效 name
  if (props.modelValue[idx] === props.initialState) {
    const firstValid = next.find((n) => n)
    emit('update:initialState', firstValid || '')
  }
}

function updateName(idx: number, val: string) {
  const next = props.modelValue.map((n, i) => (i === idx ? val : n))
  emit('update:modelValue', next)

  // 若修改的是当前 initial_state，同步更新
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

:deep(.el-input.is-error .el-input__wrapper) {
  box-shadow: 0 0 0 1px #F56C6C inset;
}

.dup-hint {
  font-size: 12px;
  color: #F56C6C;
  flex-shrink: 0;
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
