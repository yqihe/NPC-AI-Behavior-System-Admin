<template>
  <el-dialog
    :model-value="modelValue"
    title="选择节点类型"
    width="480px"
    append-to-body
    :close-on-click-modal="false"
    @close="handleClose"
  >
    <div class="node-type-selector-body">
      <el-radio-group v-model="selectedTypeName" class="radio-group">
        <!-- 组合节点 -->
        <template v-if="compositeTypes.length > 0">
          <div class="category-title">组合节点</div>
          <div class="radio-list">
            <el-radio
              v-for="t in compositeTypes"
              :key="t.type_name"
              :value="t.type_name"
            >
              {{ t.label }} ({{ t.type_name }})
            </el-radio>
          </div>
          <el-divider v-if="decoratorTypes.length > 0 || leafTypes.length > 0" />
        </template>

        <!-- 装饰节点 -->
        <template v-if="decoratorTypes.length > 0">
          <div class="category-title">装饰节点</div>
          <div class="radio-list">
            <el-radio
              v-for="t in decoratorTypes"
              :key="t.type_name"
              :value="t.type_name"
            >
              {{ t.label }} ({{ t.type_name }})
            </el-radio>
          </div>
          <el-divider v-if="leafTypes.length > 0" />
        </template>

        <!-- 叶子节点 -->
        <template v-if="leafTypes.length > 0">
          <div class="category-title">叶子节点</div>
          <div class="radio-list">
            <el-radio
              v-for="t in leafTypes"
              :key="t.type_name"
              :value="t.type_name"
            >
              {{ t.label }} ({{ t.type_name }})
            </el-radio>
          </div>
        </template>
      </el-radio-group>
    </div>

    <template #footer>
      <el-button @click="handleClose">取消</el-button>
      <el-button type="primary" :disabled="!selectedTypeName" @click="handleConfirm">
        确认
      </el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import type { BtNodeTypeMeta } from '@/api/btNodeTypes'

const props = defineProps<{
  modelValue: boolean
  nodeTypes: BtNodeTypeMeta[]
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  'select': [nodeType: BtNodeTypeMeta]
}>()

const selectedTypeName = ref<string>('')

const compositeTypes = computed(() =>
  props.nodeTypes.filter((t) => t.category === 'composite')
)
const decoratorTypes = computed(() =>
  props.nodeTypes.filter((t) => t.category === 'decorator')
)
const leafTypes = computed(() =>
  props.nodeTypes.filter((t) => t.category === 'leaf')
)

function handleClose() {
  emit('update:modelValue', false)
  selectedTypeName.value = ''
}

function handleConfirm() {
  const found = props.nodeTypes.find((t) => t.type_name === selectedTypeName.value)
  if (!found) return
  emit('select', found)
  emit('update:modelValue', false)
  selectedTypeName.value = ''
}
</script>

<style scoped>
.node-type-selector-body {
  padding: 4px 0;
}

.radio-group {
  display: flex;
  flex-direction: column;
  width: 100%;
}

.category-title {
  font-size: 12px;
  color: #909399;
  margin-bottom: 8px;
  font-weight: 500;
}

.radio-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-bottom: 4px;
}
</style>
