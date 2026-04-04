<template>
  <div :style="{ marginLeft: depth > 0 ? '24px' : '0', borderLeft: depth > 0 ? '2px solid #dcdfe6' : 'none', paddingLeft: depth > 0 ? '12px' : '0', marginBottom: '8px' }">
    <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 4px">
      <el-select v-model="node.type" size="small" style="width: 200px" @change="onTypeChange" placeholder="选择节点类型">
        <el-option-group label="复合节点（包含多个子节点）">
          <el-option v-for="t in compositeTypes" :key="t.value" :label="t.label" :value="t.value" />
        </el-option-group>
        <el-option-group label="装饰节点（包含一个子节点）">
          <el-option v-for="t in decoratorTypes" :key="t.value" :label="t.label" :value="t.value" />
        </el-option-group>
        <el-option-group label="叶子节点（执行具体操作）">
          <el-option v-for="t in leafTypes" :key="t.value" :label="t.label" :value="t.value" />
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
          <el-option label="running" value="running" />
        </el-select>
      </template>
      <template v-else-if="node.type === 'set_bb_value'">
        <el-select v-model="params.key" size="small" placeholder="BB Key" style="width: 180px" @change="emitChange">
          <el-option v-for="k in bbKeys" :key="k.value" :label="k.label" :value="k.value" />
        </el-select>
        <el-input v-model="params.value" size="small" placeholder="值" style="width: 150px" @input="emitChange" />
      </template>
      <template v-else-if="node.type === 'check_bb_float'">
        <el-select v-model="params.key" size="small" placeholder="BB Key" style="width: 180px" @change="emitChange">
          <el-option v-for="k in bbKeys" :key="k.value" :label="k.label" :value="k.value" />
        </el-select>
        <el-select v-model="params.op" size="small" style="width: 70px" @change="emitChange">
          <el-option v-for="op in ['==','!=','>','<','>=','<=']" :key="op" :label="op" :value="op" />
        </el-select>
        <el-input-number v-model="params.value" size="small" style="width: 100px" @change="emitChange" />
      </template>
      <template v-else-if="node.type === 'check_bb_string'">
        <el-select v-model="params.key" size="small" placeholder="BB Key" style="width: 180px" @change="emitChange">
          <el-option v-for="k in bbKeys" :key="k.value" :label="k.label" :value="k.value" />
        </el-select>
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

    <!-- 装饰节点（单个子节点） -->
    <div v-if="isDecorator">
      <BtNodeEditor
        :model-value="decoratorChild"
        :depth="depth + 1"
        :removable="false"
        @update:model-value="updateDecoratorChild"
      />
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

const compositeTypes = [
  { value: 'sequence', label: '顺序执行 (sequence)' },
  { value: 'selector', label: '选择执行 (selector)' },
  { value: 'parallel', label: '并行执行 (parallel)' },
]
const decoratorTypes = [
  { value: 'inverter', label: '结果反转 (inverter)' },
]
const leafTypes = [
  { value: 'check_bb_float', label: '检查数值 (check_bb_float)' },
  { value: 'check_bb_string', label: '检查文本 (check_bb_string)' },
  { value: 'set_bb_value', label: '设置黑板值 (set_bb_value)' },
  { value: 'stub_action', label: '占位动作 (stub_action)' },
]

const compositeTypeValues = compositeTypes.map(t => t.value)
const decoratorTypeValues = decoratorTypes.map(t => t.value)
const leafTypeValues = leafTypes.map(t => t.value)

// Blackboard Key 白名单（与游戏服务端 internal/core/blackboard/keys.go 对齐）
const bbKeys = [
  { value: 'threat_level', label: '威胁等级 (threat_level)' },
  { value: 'threat_source', label: '威胁来源 (threat_source)' },
  { value: 'threat_expire_at', label: '威胁过期时间 (threat_expire_at)' },
  { value: 'last_event_type', label: '最近事件类型 (last_event_type)' },
  { value: 'current_time', label: '当前时间 (current_time)' },
  { value: 'fsm_state', label: 'FSM 状态 (fsm_state)' },
  { value: 'npc_type', label: 'NPC 类型 (npc_type)' },
  { value: 'npc_pos_x', label: 'NPC X 坐标 (npc_pos_x)' },
  { value: 'npc_pos_z', label: 'NPC Z 坐标 (npc_pos_z)' },
  { value: 'current_action', label: '当前动作 (current_action)' },
  { value: 'alert_start_tick', label: '警戒开始 Tick (alert_start_tick)' },
  { value: 'exit_cleanup_done', label: '退出清理完成 (exit_cleanup_done)' },
]

const node = ref({ type: '' })
const params = ref({})
const children = ref([])
const decoratorChild = ref({})

const isComposite = computed(() => compositeTypeValues.includes(node.value.type))
const isDecorator = computed(() => decoratorTypeValues.includes(node.value.type))
const isLeaf = computed(() => leafTypeValues.includes(node.value.type))

function parseValue(val) {
  if (!val || typeof val !== 'object') {
    node.value = { type: '' }
    params.value = {}
    children.value = []
    decoratorChild.value = {}
    return
  }
  node.value = { type: val.type || '' }
  params.value = val.params ? { ...val.params } : {}
  children.value = val.children ? [...val.children] : []
  decoratorChild.value = val.child ? { ...val.child } : {}
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
  if (isDecorator.value) {
    result.child = { ...decoratorChild.value }
  }
  emit('update:modelValue', result)
}

function onTypeChange() {
  if (isLeaf.value) {
    children.value = []
    decoratorChild.value = {}
    if (node.value.type === 'stub_action') params.value = { name: '', result: 'success' }
    else if (node.value.type === 'set_bb_value') params.value = { key: '', value: '' }
    else if (node.value.type === 'check_bb_float') params.value = { key: '', op: '>=', value: 0 }
    else if (node.value.type === 'check_bb_string') params.value = { key: '', op: '==', value: '' }
  } else if (isDecorator.value) {
    children.value = []
    params.value = {}
    if (!decoratorChild.value.type) {
      decoratorChild.value = { type: 'stub_action', params: { name: '', result: 'success' } }
    }
  } else {
    params.value = {}
    decoratorChild.value = {}
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

function updateDecoratorChild(val) {
  decoratorChild.value = val
  emitChange()
}
</script>
