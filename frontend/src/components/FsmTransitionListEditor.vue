<template>
  <div class="transition-list-editor">
    <el-collapse v-model="activeItems" class="transition-collapse">
      <el-collapse-item
        v-for="(t, idx) in modelValue"
        :key="idx"
        :name="idx"
      >
        <!-- 折叠标题：摘要 -->
        <template #title>
          <div class="trans-summary">
            <span class="trans-route">
              {{ t.from || '?' }} → {{ t.to || '?' }}
            </span>
            <span class="trans-sep">|</span>
            <span class="trans-priority">优先级{{ t.priority }}</span>
            <span class="trans-sep">|</span>
            <span :class="hasCondition(t) ? 'trans-has-cond' : 'trans-no-cond'">
              {{ hasCondition(t) ? '已配置条件' : '无条件' }}
            </span>
          </div>
        </template>

        <!-- 规则详情 -->
        <div class="trans-body">
          <!-- from / to / priority -->
          <div class="trans-fields">
            <div class="trans-field">
              <span class="trans-field-label">从</span>
              <el-select
                :model-value="t.from"
                :disabled="disabled"
                placeholder="源状态"
                style="width: 160px"
                @change="(v: string) => updateField(idx, 'from', v)"
              >
                <el-option v-for="s in states" :key="s" :label="s" :value="s" />
              </el-select>
            </div>
            <div class="trans-field">
              <span class="trans-field-label">到</span>
              <el-select
                :model-value="t.to"
                :disabled="disabled"
                placeholder="目标状态"
                style="width: 160px"
                @change="(v: string) => updateField(idx, 'to', v)"
              >
                <el-option v-for="s in states" :key="s" :label="s" :value="s" />
              </el-select>
            </div>
            <div class="trans-field">
              <span class="trans-field-label">优先级</span>
              <el-input-number
                :model-value="t.priority"
                :disabled="disabled"
                :min="0"
                :controls="false"
                style="width: 100px"
                @change="(v: number | undefined) => updateField(idx, 'priority', v ?? 0)"
              />
            </div>
            <el-button
              v-if="!disabled"
              type="danger"
              link
              size="small"
              style="margin-left: auto"
              @click="removeTransition(idx)"
            >
              删除规则
            </el-button>
          </div>

          <!-- 条件编辑器 -->
          <div class="trans-cond-section">
            <div class="trans-cond-title">转换条件</div>
            <FsmConditionEditor
              :model-value="t.condition"
              :disabled="disabled"
              :depth="0"
              @update:model-value="(v) => updateCondition(idx, v)"
            />
          </div>
        </div>
      </el-collapse-item>
    </el-collapse>

    <!-- 添加规则 -->
    <el-button
      v-if="!disabled"
      text
      type="primary"
      style="margin-top: 8px"
      @click="addTransition"
    >
      + 添加转换规则
    </el-button>

    <div v-if="modelValue.length === 0" class="trans-empty">
      暂无转换规则（可选，不配置表示状态机无自动转换）
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import FsmConditionEditor from './FsmConditionEditor.vue'
import type { FsmTransition, FsmConditionNode } from '@/api/fsmConfigs'

const props = defineProps<{
  modelValue: FsmTransition[]
  states: string[]           // 来自 FsmStateListEditor 的状态名列表
  disabled?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: FsmTransition[]]
}>()

// 默认全部展开
const activeItems = ref<number[]>([])

function hasCondition(t: FsmTransition): boolean {
  const c = t.condition
  if (!c) return false
  return !!(c.key || c.and || c.or)
}

function addTransition() {
  const next: FsmTransition = {
    from: props.states[0] || '',
    to: props.states[0] || '',
    priority: 0,
    condition: {},
  }
  const newList = [...props.modelValue, next]
  emit('update:modelValue', newList)
  // 自动展开新添加的规则
  activeItems.value = [...activeItems.value, newList.length - 1]
}

function removeTransition(idx: number) {
  emit('update:modelValue', props.modelValue.filter((_, i) => i !== idx))
  activeItems.value = activeItems.value
    .filter((n) => n !== idx)
    .map((n) => (n > idx ? n - 1 : n))
}

function updateField(idx: number, field: 'from' | 'to' | 'priority', val: string | number) {
  const next = props.modelValue.map((t, i) => {
    if (i !== idx) return t
    return { ...t, [field]: val }
  })
  emit('update:modelValue', next)
}

function updateCondition(idx: number, cond: FsmConditionNode) {
  const next = props.modelValue.map((t, i) => {
    if (i !== idx) return t
    return { ...t, condition: cond }
  })
  emit('update:modelValue', next)
}
</script>

<style scoped>
.transition-list-editor {
  width: 100%;
}

.transition-collapse {
  border-radius: 4px;
}

.trans-summary {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
}

.trans-route {
  font-weight: 500;
  color: #303133;
}

.trans-sep {
  color: #C0C4CC;
}

.trans-priority {
  color: #606266;
}

.trans-has-cond {
  color: #409EFF;
}

.trans-no-cond {
  color: #909399;
}

.trans-body {
  padding: 4px 0 8px;
}

.trans-fields {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
  margin-bottom: 16px;
}

.trans-field {
  display: flex;
  align-items: center;
  gap: 8px;
}

.trans-field-label {
  font-size: 13px;
  color: #606266;
  flex-shrink: 0;
}

.trans-cond-section {
  border-top: 1px solid #F0F0F0;
  padding-top: 12px;
}

.trans-cond-title {
  font-size: 13px;
  font-weight: 500;
  color: #606266;
  margin-bottom: 10px;
}

.trans-empty {
  font-size: 13px;
  color: #C0C4CC;
  padding: 8px 0;
}
</style>
