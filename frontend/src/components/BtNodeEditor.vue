<template>
  <div class="bt-node" :style="{ marginLeft: depth > 0 ? '24px' : '0' }">
    <div class="node-header">
      <!-- 节点类型选择 -->
      <el-select
        :model-value="node.type"
        placeholder="选择节点类型"
        style="width: 220px"
        @change="onTypeChange"
      >
        <el-option-group
          v-for="group in groupedTypes"
          :key="group.label"
          :label="group.label"
        >
          <el-option
            v-for="nt in group.items"
            :key="nt.name"
            :label="`${nt.config?.display_name || nt.name} (${nt.name})`"
            :value="nt.name"
          />
        </el-option-group>
      </el-select>

      <!-- 删除按钮 -->
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

    <!-- 参数表单 -->
    <div v-if="currentParamsSchema" class="node-params">
      <schema-form
        v-model="paramsData"
        :schema="currentParamsSchema"
        :form-footer="{ show: false }"
      />
    </div>

    <!-- 子节点区域 -->
    <div v-if="currentCategory === 'composite'" class="node-children">
      <bt-node-editor
        v-for="(child, index) in node.children || []"
        :key="index"
        :model-value="child"
        :node-types="nodeTypes"
        :depth="depth + 1"
        :removable="true"
        @update:model-value="updateChild(index, $event)"
        @remove="removeChild(index)"
      />
      <el-button size="small" @click="addChild" style="margin-top: 4px">
        + 添加子节点
      </el-button>
    </div>

    <div v-if="currentCategory === 'decorator'" class="node-children">
      <bt-node-editor
        v-if="node.child"
        :model-value="node.child"
        :node-types="nodeTypes"
        :depth="depth + 1"
        :removable="true"
        @update:model-value="updateDecoChild($event)"
        @remove="removeDecoChild"
      />
      <el-button v-else size="small" @click="addDecoChild" style="margin-top: 4px">
        + 添加子节点
      </el-button>
    </div>
  </div>
</template>

<script setup>
import { computed, watch, ref } from 'vue'
import SchemaForm from '@/components/SchemaForm.vue'

const props = defineProps({
  modelValue: { type: Object, default: () => ({ type: 'sequence', children: [] }) },
  nodeTypes: { type: Array, default: () => [] },
  depth: { type: Number, default: 0 },
  removable: { type: Boolean, default: false },
})

const emit = defineEmits(['update:modelValue', 'remove'])

const node = ref(JSON.parse(JSON.stringify(props.modelValue)))

watch(() => props.modelValue, (val) => {
  node.value = JSON.parse(JSON.stringify(val))
}, { deep: true })

function emitUpdate() {
  emit('update:modelValue', JSON.parse(JSON.stringify(node.value)))
}

// ========== 节点类型分组 ==========

const groupedTypes = computed(() => {
  const groups = { composite: [], decorator: [], leaf: [] }
  for (const nt of props.nodeTypes) {
    const cat = nt.config?.category || 'leaf'
    if (groups[cat]) groups[cat].push(nt)
  }
  return [
    { label: '复合节点', items: groups.composite },
    { label: '装饰节点', items: groups.decorator },
    { label: '叶子节点', items: groups.leaf },
  ].filter(g => g.items.length > 0)
})

// 当前节点的类型信息
const currentTypeDef = computed(() =>
  props.nodeTypes.find(nt => nt.name === node.value.type)
)

const currentCategory = computed(() =>
  currentTypeDef.value?.config?.category || ''
)

const currentParamsSchema = computed(() =>
  currentTypeDef.value?.config?.params_schema || null
)

// ========== 参数数据（展平在节点上） ==========

// 从节点对象中提取参数（排除 type / children / child）
const paramsData = computed({
  get() {
    const { type, children, child, ...params } = node.value
    return params
  },
  set(val) {
    const newNode = { type: node.value.type, ...val }
    // 保留子节点
    if (node.value.children) newNode.children = node.value.children
    if (node.value.child) newNode.child = node.value.child
    node.value = newNode
    emitUpdate()
  },
})

// ========== 类型切换 ==========

function onTypeChange(newType) {
  const typeDef = props.nodeTypes.find(nt => nt.name === newType)
  const category = typeDef?.config?.category || 'leaf'

  const newNode = { type: newType }
  if (category === 'composite') {
    newNode.children = []
  } else if (category === 'decorator') {
    // child 暂不初始化，等用户添加
  }
  // leaf: 无子节点

  node.value = newNode
  emitUpdate()
}

// ========== 子节点操作（composite） ==========

function addChild() {
  if (!node.value.children) node.value.children = []
  node.value.children.push({ type: 'sequence', children: [] })
  emitUpdate()
}

function updateChild(index, val) {
  node.value.children[index] = val
  emitUpdate()
}

function removeChild(index) {
  node.value.children.splice(index, 1)
  emitUpdate()
}

// ========== 子节点操作（decorator） ==========

function addDecoChild() {
  node.value.child = { type: 'sequence', children: [] }
  emitUpdate()
}

function updateDecoChild(val) {
  node.value.child = val
  emitUpdate()
}

function removeDecoChild() {
  delete node.value.child
  emitUpdate()
}
</script>

<style scoped>
.bt-node {
  border-left: 2px solid #dcdfe6;
  padding: 8px 0 8px 12px;
  margin-bottom: 4px;
}
.node-header {
  display: flex;
  align-items: center;
  margin-bottom: 8px;
}
.node-params {
  margin-bottom: 8px;
  padding-left: 4px;
}
.node-children {
  margin-top: 4px;
}
</style>
