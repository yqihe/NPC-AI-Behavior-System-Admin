<template>
  <div class="condition-editor" :style="{ marginLeft: depth > 0 ? '20px' : '0' }">
    <div class="condition-header">
      <!-- 条件类型选择 -->
      <el-select
        :model-value="conditionType"
        placeholder="条件类型"
        style="width: 120px"
        @change="onTypeChange"
      >
        <el-option label="条件 (leaf)" value="leaf" />
        <el-option label="AND 组合" value="and" />
        <el-option label="OR 组合" value="or" />
      </el-select>

      <el-button
        v-if="removable"
        text
        type="danger"
        size="small"
        style="margin-left: 8px"
        @click="$emit('remove')"
      >
        删除
      </el-button>
    </div>

    <!-- leaf 模式：key + op + value/ref_key -->
    <div v-if="conditionType === 'leaf'" class="leaf-params">
      <el-form-item label="黑板 Key" style="margin-bottom: 8px">
        <el-input
          :model-value="condition.key || ''"
          placeholder="如 threat_level"
          style="width: 200px"
          @input="updateField('key', $event)"
        />
      </el-form-item>

      <el-form-item label="操作符" style="margin-bottom: 8px">
        <el-select
          :model-value="condition.op || ''"
          placeholder="选择操作符"
          style="width: 120px"
          @change="updateField('op', $event)"
        >
          <el-option v-for="op in operators" :key="op" :label="op" :value="op" />
        </el-select>
      </el-form-item>

      <el-form-item label="比较值" style="margin-bottom: 8px">
        <el-input
          :model-value="condition.value !== undefined ? String(condition.value) : ''"
          placeholder="字面量值"
          style="width: 200px"
          @input="updateValue($event)"
        />
      </el-form-item>

      <el-form-item label="或引用 Key（与比较值二选一）" style="margin-bottom: 8px">
        <el-input
          :model-value="condition.ref_key || ''"
          placeholder="引用另一个黑板 Key"
          style="width: 200px"
          @input="updateField('ref_key', $event)"
        />
      </el-form-item>
    </div>

    <!-- and/or 模式：子条件列表 -->
    <div v-if="conditionType === 'and' || conditionType === 'or'" class="composite-children">
      <condition-editor
        v-for="(child, index) in subConditions"
        :key="index"
        :model-value="child"
        :operators="operators"
        :depth="depth + 1"
        :removable="true"
        @update:model-value="updateSubCondition(index, $event)"
        @remove="removeSubCondition(index)"
      />
      <el-button size="small" @click="addSubCondition" style="margin-top: 4px">
        + 添加条件
      </el-button>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  modelValue: { type: Object, default: () => ({ key: '', op: '', value: '' }) },
  operators: { type: Array, default: () => ['==', '!=', '>', '>=', '<', '<=', 'in'] },
  depth: { type: Number, default: 0 },
  removable: { type: Boolean, default: false },
})

const emit = defineEmits(['update:modelValue', 'remove'])

const condition = computed(() => props.modelValue || {})

// 判断当前条件类型
const conditionType = computed(() => {
  if (condition.value.and) return 'and'
  if (condition.value.or) return 'or'
  return 'leaf'
})

// 子条件列表（and/or 模式）
const subConditions = computed(() => {
  if (condition.value.and) return condition.value.and
  if (condition.value.or) return condition.value.or
  return []
})

function onTypeChange(newType) {
  if (newType === 'leaf') {
    emit('update:modelValue', { key: '', op: '', value: '' })
  } else if (newType === 'and') {
    emit('update:modelValue', { and: [] })
  } else if (newType === 'or') {
    emit('update:modelValue', { or: [] })
  }
}

function updateField(field, value) {
  emit('update:modelValue', { ...condition.value, [field]: value })
}

function updateValue(raw) {
  // 尝试转为数字
  const num = Number(raw)
  const value = raw !== '' && !isNaN(num) ? num : raw
  emit('update:modelValue', { ...condition.value, value })
}

// ========== 子条件操作 ==========

function addSubCondition() {
  const key = conditionType.value // 'and' or 'or'
  const children = [...(condition.value[key] || []), { key: '', op: '', value: '' }]
  emit('update:modelValue', { [key]: children })
}

function updateSubCondition(index, val) {
  const key = conditionType.value
  const children = [...(condition.value[key] || [])]
  children[index] = val
  emit('update:modelValue', { [key]: children })
}

function removeSubCondition(index) {
  const key = conditionType.value
  const children = [...(condition.value[key] || [])]
  children.splice(index, 1)
  emit('update:modelValue', { [key]: children })
}
</script>

<style scoped>
.condition-editor {
  border-left: 2px solid #e4e7ed;
  padding: 8px 0 8px 12px;
  margin-bottom: 4px;
}
.condition-header {
  display: flex;
  align-items: center;
  margin-bottom: 8px;
}
.leaf-params {
  padding-left: 4px;
}
</style>
