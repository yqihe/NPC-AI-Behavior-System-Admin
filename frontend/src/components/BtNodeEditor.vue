<template>
  <div :style="{ marginLeft: depth > 0 ? '24px' : '0', borderLeft: depth > 0 ? '2px solid #dcdfe6' : 'none', paddingLeft: depth > 0 ? '12px' : '0', marginBottom: '8px' }">
    <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 4px">
      <el-select v-model="node.type" size="small" style="width: 160px" @change="onTypeChange">
        <el-option-group label="复合节点">
          <el-option v-for="t in compositeTypes" :key="t" :label="t" :value="t" />
        </el-option-group>
        <el-option-group label="叶子节点">
          <el-option v-for="t in leafTypes" :key="t" :label="t" :value="t" />
        </el-option-group>
      </el-select>
      <el-button v-if="removable" size="small" type="danger" plain @click="$emit('remove')">删除</el-button>
    </div>

    <!-- 叶子节点参数 -->
    <div v-if="isLeaf && node.type" style="display: flex; gap: 8px; flex-wrap: wrap; margin: 4px 0">
      <template v-if="node.type === 'stub_action'">
        <el-input v-model="params.name" size="small" placeholder="动作名" style="width: 150px" @input="emitChange" />
        <el-select v-model="params.result" size="small" style="width: 100px" @change="emitChange">
          <el-option label="success" value="success" />
          <el-option label="failure" value="failure" />
        </el-select>
      </template>
      <template v-else-if="node.type === 'set_bb_value'">
        <el-input v-model="params.key" size="small" placeholder="BB Key" style="width: 150px" @input="emitChange" />
        <el-input v-model="params.value" size="small" placeholder="值" style="width: 150px" @input="emitChange" />
      </template>
      <template v-else-if="node.type === 'check_bb_float'">
        <el-input v-model="params.key" size="small" placeholder="BB Key" style="width: 130px" @input="emitChange" />
        <el-select v-model="params.op" size="small" style="width: 70px" @change="emitChange">
          <el-option v-for="op in ['==','!=','>','<','>=','<=']" :key="op" :label="op" :value="op" />
        </el-select>
        <el-input-number v-model="params.value" size="small" style="width: 100px" @change="emitChange" />
      </template>
      <template v-else-if="node.type === 'check_bb_string'">
        <el-input v-model="params.key" size="small" placeholder="BB Key" style="width: 130px" @input="emitChange" />
        <el-select v-model="params.op" size="small" style="width: 70px" @change="emitChange">
          <el-option v-for="op in ['==','!=']" :key="op" :label="op" :value="op" />
        </el-select>
        <el-input v-model="params.value" size="small" placeholder="值" style="width: 120px" @input="emitChange" />
      </template>
    </div>

    <!-- 复合节点子节点 -->
    <div v-if="isComposite">
      <BtNodeEditor
        v-for="(child, idx) in children"
        :key="idx"
        :model-value="child"
        :depth="depth + 1"
        :removable="true"
        @update:model-value="val => updateChild(idx, val)"
        @remove="removeChild(idx)"
      />
      <el-button size="small" @click="addChild" style="margin-top: 4px">添加子节点</el-button>
    </div>
  </div>
</template>

<script setup>
import { ref, watch, computed } from 'vue'

const props = defineProps({
  modelValue: { type: Object, default: () => ({}) },
  depth: { type: Number, default: 0 },
  removable: { type: Boolean, default: false },
})
const emit = defineEmits(['update:modelValue', 'remove'])

const compositeTypes = ['sequence', 'selector', 'parallel', 'inverter']
const leafTypes = ['check_bb_float', 'check_bb_string', 'set_bb_value', 'stub_action']

const node = ref({ type: '' })
const params = ref({})
const children = ref([])

const isComposite = computed(() => compositeTypes.includes(node.value.type))
const isLeaf = computed(() => leafTypes.includes(node.value.type))

function parseValue(val) {
  if (!val || typeof val !== 'object') {
    node.value = { type: '' }
    params.value = {}
    children.value = []
    return
  }
  node.value = { type: val.type || '' }
  params.value = val.params ? { ...val.params } : {}
  children.value = val.children ? [...val.children] : []
}

watch(() => props.modelValue, parseValue, { immediate: true, deep: true })

function emitChange() {
  const result = { type: node.value.type }
  if (isLeaf.value) {
    result.params = { ...params.value }
  }
  if (isComposite.value) {
    result.children = [...children.value]
  }
  emit('update:modelValue', result)
}

function onTypeChange() {
  if (isLeaf.value) {
    children.value = []
    if (node.value.type === 'stub_action') params.value = { name: '', result: 'success' }
    else if (node.value.type === 'set_bb_value') params.value = { key: '', value: '' }
    else if (node.value.type === 'check_bb_float') params.value = { key: '', op: '>=', value: 0 }
    else if (node.value.type === 'check_bb_string') params.value = { key: '', op: '==', value: '' }
  } else {
    params.value = {}
    if (children.value.length === 0) {
      children.value = [{ type: 'stub_action', params: { name: '', result: 'success' } }]
    }
  }
  emitChange()
}

function addChild() {
  children.value.push({ type: 'stub_action', params: { name: '', result: 'success' } })
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
