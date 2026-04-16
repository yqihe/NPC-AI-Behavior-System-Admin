<template>
  <el-dialog
    :model-value="modelValue"
    title="选择节点类型"
    width="520px"
    append-to-body
    :close-on-click-modal="false"
    @close="handleClose"
  >
    <div class="selector-body">
      <!-- 组合节点 -->
      <template v-if="compositeTypes.length > 0">
        <div class="category-label">组合节点</div>
        <div class="card-grid">
          <div
            v-for="t in compositeTypes"
            :key="t.type_name"
            class="type-card"
            @click="handleConfirm(t)"
          >
            <el-tag type="primary" size="small" effect="light" class="card-tag">composite</el-tag>
            <span class="card-name">{{ t.label }}</span>
            <span class="card-key">{{ t.type_name }}</span>
          </div>
        </div>
      </template>

      <!-- 装饰节点 -->
      <template v-if="decoratorTypes.length > 0">
        <div class="category-label">装饰节点</div>
        <div class="card-grid">
          <div
            v-for="t in decoratorTypes"
            :key="t.type_name"
            class="type-card"
            @click="handleConfirm(t)"
          >
            <el-tag type="warning" size="small" effect="light" class="card-tag">decorator</el-tag>
            <span class="card-name">{{ t.label }}</span>
            <span class="card-key">{{ t.type_name }}</span>
          </div>
        </div>
      </template>

      <!-- 叶子节点 -->
      <template v-if="leafTypes.length > 0">
        <div class="category-label">叶子节点</div>
        <div class="card-grid">
          <div
            v-for="t in leafTypes"
            :key="t.type_name"
            class="type-card"
            @click="handleConfirm(t)"
          >
            <el-tag type="success" size="small" effect="light" class="card-tag">leaf</el-tag>
            <span class="card-name">{{ t.label }}</span>
            <span class="card-key">{{ t.type_name }}</span>
          </div>
        </div>
      </template>

      <el-empty v-if="props.nodeTypes.length === 0" description="暂无可用节点类型" />
    </div>

    <template #footer>
      <el-button @click="handleClose">取消</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { BtNodeTypeMeta } from '@/api/btNodeTypes'

const props = defineProps<{
  modelValue: boolean
  nodeTypes: BtNodeTypeMeta[]
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  'select': [nodeType: BtNodeTypeMeta]
}>()

const compositeTypes = computed(() =>
  props.nodeTypes.filter((t) => t.category === 'composite'),
)
const decoratorTypes = computed(() =>
  props.nodeTypes.filter((t) => t.category === 'decorator'),
)
const leafTypes = computed(() =>
  props.nodeTypes.filter((t) => t.category === 'leaf'),
)

function handleClose() {
  emit('update:modelValue', false)
}

function handleConfirm(t: BtNodeTypeMeta) {
  emit('select', t)
  emit('update:modelValue', false)
}
</script>

<style scoped>
.selector-body {
  padding: 4px 0;
  max-height: 480px;
  overflow-y: auto;
}

.category-label {
  font-size: 12px;
  color: #909399;
  font-weight: 500;
  margin: 12px 0 8px;
}

.category-label:first-child {
  margin-top: 0;
}

.card-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px;
  margin-bottom: 4px;
}

.type-card {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 10px 12px;
  border: 1px solid #e4e7ed;
  border-radius: 6px;
  cursor: pointer;
  transition: border-color 0.15s, background-color 0.15s;
}

.type-card:hover {
  border-color: #409eff;
  background-color: #ecf5ff;
}

.card-tag {
  align-self: flex-start;
}

.card-name {
  font-size: 13px;
  font-weight: 600;
  color: #303133;
  line-height: 1.4;
}

.card-key {
  font-size: 11px;
  color: #909399;
  font-family: monospace;
}
</style>
