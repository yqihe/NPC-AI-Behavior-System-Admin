<template>
  <div style="border: 1px solid #dcdfe6; border-radius: 4px; padding: 8px; margin-bottom: 8px; background: #fafafa">
    <!-- 类型选择 -->
    <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 8px">
      <el-select v-model="condType" size="small" style="width: 100px" @change="onTypeChange">
        <el-option label="条件" value="leaf" />
        <el-option label="AND" value="and" />
        <el-option label="OR" value="or" />
      </el-select>
      <el-button size="small" type="danger" plain @click="$emit('remove')" v-if="removable">删除</el-button>
    </div>

    <!-- 叶子条件 -->
    <div v-if="condType === 'leaf'" style="display: flex; gap: 8px; flex-wrap: wrap">
      <el-input v-model="leaf.key" size="small" placeholder="BB Key" style="width: 150px" @input="emitChange" />
      <el-select v-model="leaf.op" size="small" style="width: 80px" @change="emitChange">
        <el-option v-for="op in ops" :key="op" :label="op" :value="op" />
      </el-select>
      <el-input v-model="leaf.value" size="small" placeholder="值" style="width: 120px" @input="emitChange" />
      <el-input v-model="leaf.ref_key" size="small" placeholder="引用 Key（可选）" style="width: 150px" @input="emitChange" />
    </div>

    <!-- 组合条件 -->
    <div v-else>
      <ConditionEditor
        v-for="(child, idx) in children"
        :key="idx"
        :model-value="child"
        :removable="true"
        @update:model-value="val => updateChild(idx, val)"
        @remove="removeChild(idx)"
      />
      <el-button size="small" @click="addChild" style="margin-top: 4px">添加子条件</el-button>
    </div>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue'

const props = defineProps({
  modelValue: { type: Object, default: () => ({}) },
  removable: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue', 'remove'])

const ops = ['==', '!=', '>', '<', '>=', '<=']
const condType = ref('leaf')
const leaf = ref({ key: '', op: '==', value: '', ref_key: '' })
const children = ref([])

function parseValue(val) {
  if (!val || typeof val !== 'object') {
    condType.value = 'leaf'
    leaf.value = { key: '', op: '==', value: '', ref_key: '' }
    children.value = []
    return
  }
  if (val.and) {
    condType.value = 'and'
    children.value = [...val.and]
  } else if (val.or) {
    condType.value = 'or'
    children.value = [...val.or]
  } else {
    condType.value = 'leaf'
    leaf.value = {
      key: val.key || '',
      op: val.op || '==',
      value: val.value !== undefined ? String(val.value) : '',
      ref_key: val.ref_key || '',
    }
  }
}

watch(() => props.modelValue, parseValue, { immediate: true, deep: true })

function emitChange() {
  if (condType.value === 'leaf') {
    const result = { key: leaf.value.key, op: leaf.value.op }
    // 尝试将 value 解析为数字
    const numVal = Number(leaf.value.value)
    if (leaf.value.value !== '' && !isNaN(numVal)) {
      result.value = numVal
    } else {
      result.value = leaf.value.value
    }
    if (leaf.value.ref_key) {
      result.ref_key = leaf.value.ref_key
    }
    emit('update:modelValue', result)
  } else {
    emit('update:modelValue', { [condType.value]: [...children.value] })
  }
}

function onTypeChange() {
  if (condType.value === 'leaf') {
    children.value = []
  } else {
    leaf.value = { key: '', op: '==', value: '', ref_key: '' }
    if (children.value.length === 0) {
      children.value = [{ key: '', op: '==', value: '' }]
    }
  }
  emitChange()
}

function addChild() {
  children.value.push({ key: '', op: '==', value: '' })
  emitChange()
}

function removeChild(idx) {
  children.value.splice(idx, 1)
  emitChange()
}

function updateChild(idx, val) {
  children.value[idx] = val
  emitChange()
}
</script>
