<template>
  <div class="condition-editor" :class="{ 'condition-editor-nested': depth > 0 }">

    <!-- 条件类型选择 -->
    <div class="condition-type-row">
      <el-radio-group
        :model-value="condType"
        :disabled="disabled"
        size="small"
        @change="handleTypeChange"
      >
        <el-radio-button value="none">无条件</el-radio-button>
        <el-radio-button value="leaf">单条件</el-radio-button>
        <el-radio-button value="group">组合条件</el-radio-button>
      </el-radio-group>
    </div>

    <!-- 单条件（叶节点） -->
    <div v-if="condType === 'leaf'" class="leaf-editor">
      <div class="leaf-row">
        <!-- BB Key 选择 -->
        <div class="leaf-field">
          <div class="leaf-label">BB Key</div>
          <BBKeySelector
            :model-value="modelValue.key || ''"
            :disabled="disabled"
            @update:model-value="handleKeyChange"
            @field-selected="handleFieldSelected"
          />
        </div>

        <!-- 操作符 -->
        <div class="leaf-field leaf-field-op">
          <div class="leaf-label">操作符</div>
          <el-select
            :model-value="modelValue.op || ''"
            :disabled="disabled"
            placeholder="选择"
            style="width: 100%"
            @change="(v: string) => emitPatch({ op: v })"
          >
            <el-option v-for="op in OP_OPTIONS" :key="op" :label="op" :value="op" />
          </el-select>
        </div>

        <!-- 值模式：直接值 / 引用 BB Key -->
        <div class="leaf-field">
          <div class="leaf-label">
            <el-radio-group
              :model-value="valueMode"
              :disabled="disabled"
              size="small"
              @change="handleValueModeChange"
            >
              <el-radio-button value="value">直接值</el-radio-button>
              <el-radio-button value="ref_key">引用 Key</el-radio-button>
            </el-radio-group>
          </div>

          <!-- 直接值控件 -->
          <template v-if="valueMode === 'value'">
            <!-- boolean -->
            <el-select
              v-if="selectedFieldType === 'bool'"
              :model-value="modelValue.value"
              :disabled="disabled"
              placeholder="选择"
              style="width: 100%"
              @change="(v: unknown) => emitPatch({ value: v })"
            >
              <el-option label="true" :value="true" />
              <el-option label="false" :value="false" />
            </el-select>
            <!-- integer -->
            <el-input-number
              v-else-if="selectedFieldType === 'integer'"
              :model-value="(modelValue.value as number | undefined)"
              :controls="false"
              :step="1"
              :precision="0"
              :disabled="disabled"
              style="width: 100%"
              @change="(v: number | undefined) => emitPatch({ value: v })"
            />
            <!-- float -->
            <el-input-number
              v-else-if="selectedFieldType === 'float'"
              :model-value="(modelValue.value as number | undefined)"
              :controls="false"
              :step="0.01"
              :disabled="disabled"
              style="width: 100%"
              @change="(v: number | undefined) => emitPatch({ value: v })"
            />
            <!-- string / select / 运行时 Key（未知类型）-->
            <el-input
              v-else
              :model-value="(modelValue.value as string | undefined) || ''"
              :disabled="disabled"
              placeholder="输入值"
              style="width: 100%"
              @input="(v: string) => emitPatch({ value: v })"
            />
          </template>

          <!-- 引用 Key -->
          <BBKeySelector
            v-else
            :model-value="modelValue.ref_key || ''"
            :disabled="disabled"
            @update:model-value="(v: string) => emitPatch({ ref_key: v })"
          />
        </div>
      </div>
    </div>

    <!-- 组合条件 -->
    <div v-else-if="condType === 'group'" class="group-editor">
      <!-- AND / OR 选择 -->
      <div class="group-logic-row">
        <el-radio-group
          :model-value="groupLogic"
          :disabled="disabled"
          size="small"
          @change="handleLogicChange"
        >
          <el-radio-button value="and">AND（全部满足）</el-radio-button>
          <el-radio-button value="or">OR（任一满足）</el-radio-button>
        </el-radio-group>
      </div>

      <!-- 子条件列表 -->
      <div class="group-children">
        <div
          v-for="(child, idx) in groupChildren"
          :key="idx"
          class="group-child"
        >
          <div class="child-header">
            <span class="child-index">条件 {{ idx + 1 }}</span>
            <el-button
              v-if="!disabled"
              type="danger"
              link
              size="small"
              @click="removeChild(idx)"
            >
              删除
            </el-button>
          </div>
          <FsmConditionEditor
            :model-value="child"
            :depth="depth + 1"
            :disabled="disabled"
            @update:model-value="(v) => updateChild(idx, v)"
          />
        </div>
      </div>

      <!-- 添加子条件按钮 -->
      <el-button
        v-if="!disabled"
        text
        type="primary"
        :disabled="depth >= MAX_DEPTH - 1"
        @click="addChild"
      >
        + 添加子条件
        <span v-if="depth >= MAX_DEPTH - 1" class="depth-hint">（已达最大嵌套层数）</span>
      </el-button>
    </div>

  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import BBKeySelector from './BBKeySelector.vue'
import type { FsmConditionNode } from '@/api/fsmConfigs'
import type { FieldListItem } from '@/api/fields'

const MAX_DEPTH = 10

const OP_OPTIONS = ['==', '!=', '>', '>=', '<', '<=', 'in'] as const

const props = defineProps<{
  modelValue: FsmConditionNode
  depth?: number
  disabled?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: FsmConditionNode]
}>()

const depth = computed(() => props.depth ?? 0)

// ─── 条件类型判断 ───

const condType = computed<'none' | 'leaf' | 'group'>(() => {
  const v = props.modelValue
  if (!v) return 'none'
  if (v.key) return 'leaf'
  if (v.and || v.or) return 'group'
  return 'none'
})

// ─── 值模式（直接值 / 引用 Key） ───

const valueMode = computed<'value' | 'ref_key'>(() =>
  props.modelValue.ref_key ? 'ref_key' : 'value',
)

// ─── 选中字段类型（用于 value 控件自适应） ───

const selectedFieldType = ref<string>('')

// 切换 key 时清空 value，重置字段类型
function handleKeyChange(key: string) {
  emit('update:modelValue', {
    key,
    op: props.modelValue.op || '',
  })
  if (!key) {
    selectedFieldType.value = ''
  }
}

function handleFieldSelected(field: FieldListItem | null) {
  if (field) {
    selectedFieldType.value = field.type
  } else {
    // 自由输入的运行时 Key，类型未知，降级为文本框
    selectedFieldType.value = ''
  }
}

// ─── 组合条件 ───

const groupLogic = computed<'and' | 'or'>(() =>
  props.modelValue.or ? 'or' : 'and',
)

const groupChildren = computed<FsmConditionNode[]>(() => {
  const v = props.modelValue
  if (v.and) return v.and
  if (v.or) return v.or
  return []
})

function handleLogicChange(logic: 'and' | 'or') {
  const children = groupChildren.value
  if (logic === 'and') {
    emit('update:modelValue', { and: children })
  } else {
    emit('update:modelValue', { or: children })
  }
}

function addChild() {
  const children = [...groupChildren.value, {}]
  const logic = groupLogic.value
  emit('update:modelValue', logic === 'and' ? { and: children } : { or: children })
}

function removeChild(idx: number) {
  const children = groupChildren.value.filter((_, i) => i !== idx)
  const logic = groupLogic.value
  emit('update:modelValue', logic === 'and' ? { and: children } : { or: children })
}

function updateChild(idx: number, val: FsmConditionNode) {
  const children = groupChildren.value.map((c, i) => (i === idx ? val : c))
  const logic = groupLogic.value
  emit('update:modelValue', logic === 'and' ? { and: children } : { or: children })
}

// ─── 类型切换 ───

function handleTypeChange(type: 'none' | 'leaf' | 'group') {
  if (type === 'none') {
    emit('update:modelValue', {})
  } else if (type === 'leaf') {
    emit('update:modelValue', { key: '', op: '' })
    selectedFieldType.value = ''
  } else {
    emit('update:modelValue', { and: [] })
  }
}

function handleValueModeChange(mode: 'value' | 'ref_key') {
  if (mode === 'value') {
    emit('update:modelValue', { key: props.modelValue.key, op: props.modelValue.op })
  } else {
    emit('update:modelValue', { key: props.modelValue.key, op: props.modelValue.op, ref_key: '' })
  }
}

// ─── 局部字段更新辅助 ───

function emitPatch(patch: Partial<FsmConditionNode>) {
  emit('update:modelValue', { ...props.modelValue, ...patch })
}

// 切换 key 时清空 selectedFieldType（若 key 被清空）
watch(() => props.modelValue.key, (newKey) => {
  if (!newKey) {
    selectedFieldType.value = ''
  }
})
</script>

<style scoped>
.condition-editor {
  width: 100%;
}

.condition-editor-nested {
  border-left: 2px solid #E4E7ED;
  padding-left: 12px;
  margin-top: 8px;
}

.condition-type-row {
  margin-bottom: 12px;
}

.leaf-editor {
  background: #FAFAFA;
  border: 1px solid #E4E7ED;
  border-radius: 4px;
  padding: 12px;
}

.leaf-row {
  display: flex;
  gap: 12px;
  align-items: flex-start;
}

.leaf-field {
  flex: 1;
  min-width: 0;
}

.leaf-field-op {
  flex: 0 0 120px;
}

.leaf-label {
  font-size: 12px;
  color: #606266;
  margin-bottom: 6px;
}

.group-editor {
  background: #FAFAFA;
  border: 1px solid #E4E7ED;
  border-radius: 4px;
  padding: 12px;
}

.group-logic-row {
  margin-bottom: 12px;
}

.group-children {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-bottom: 12px;
}

.group-child {
  background: #fff;
  border: 1px solid #DCDFE6;
  border-radius: 4px;
  padding: 10px 12px;
}

.child-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.child-index {
  font-size: 12px;
  color: #909399;
  font-weight: 500;
}

.depth-hint {
  font-size: 12px;
  color: #C0C4CC;
  margin-left: 4px;
}
</style>
