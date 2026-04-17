<template>
  <el-select
    :model-value="modelValue"
    filterable
    clearable
    :disabled="disabled"
    placeholder="选择 BB Key"
    style="width: 100%"
    @update:model-value="handleChange"
  >
    <el-option-group v-if="npcOptions.length > 0" label="NPC 字段">
      <el-option
        v-for="f in npcOptions"
        :key="f.name"
        :label="`${f.name} (${f.label})`"
        :value="f.name"
      />
    </el-option-group>
    <el-option-group v-if="schemaOptions.length > 0" label="事件扩展字段">
      <el-option
        v-for="f in schemaOptions"
        :key="f.name"
        :label="`${f.name} (${f.label})`"
        :value="f.name"
      />
    </el-option-group>
    <el-option
      v-if="npcOptions.length === 0 && schemaOptions.length === 0"
      label="暂无可用 BB Key（请先配置字段或事件类型）"
      value=""
      disabled
    />
  </el-select>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { fieldApi } from '@/api/fields'
import { eventTypeApi } from '@/api/eventTypes'

/** 规范化后的 BB Key 条目，供 FsmConditionEditor 使用 */
export interface BBKeyField {
  name: string
  label: string
  /** 规范化类型：integer / float / string / bool / select / reference */
  type: string
}

const props = defineProps<{
  modelValue: string
  disabled?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
  /** 选中已知字段时携带规范化 BBKeyField；清空时为 null */
  'field-selected': [field: BBKeyField | null]
}>()

const npcOptions = ref<BBKeyField[]>([])
const schemaOptions = ref<BBKeyField[]>([])

/**
 * 规范化字段类型名：
 *  - NPC 字段用 'boolean'，条件编辑器期望 'bool'  → 统一为 'bool'
 *  - 事件扩展字段用 'int'，条件编辑器期望 'integer' → 统一为 'integer'
 */
function normalizeType(raw: string): string {
  if (raw === 'boolean') return 'bool'
  if (raw === 'int') return 'integer'
  return raw
}

onMounted(async () => {
  try {
    const [fieldRes, schemaRes] = await Promise.all([
      fieldApi.list({ bb_exposed: true, enabled: true, page: 1, page_size: 200 }),
      eventTypeApi.schemaList({ enabled: true }),
    ])
    npcOptions.value = (fieldRes.data?.items || []).map((f) => ({
      name: f.name,
      label: f.label,
      type: normalizeType(f.type),
    }))
    schemaOptions.value = (schemaRes.data?.items || []).map((s) => ({
      name: s.field_name,
      label: s.field_label,
      type: normalizeType(s.field_type),
    }))
    // 初始值已有 key 时，补发 field-selected 让父组件获知字段类型
    if (props.modelValue) {
      const all = [...npcOptions.value, ...schemaOptions.value]
      const matched = all.find((f) => f.name === props.modelValue)
      emit('field-selected', matched || null)
    }
  } catch {
    // 拦截器已 toast
  }
})

function handleChange(val: string) {
  emit('update:modelValue', val || '')
  const all = [...npcOptions.value, ...schemaOptions.value]
  const matched = all.find((f) => f.name === val)
  emit('field-selected', matched || null)
}
</script>
