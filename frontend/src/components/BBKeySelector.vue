<template>
  <el-select
    :model-value="modelValue"
    filterable
    allow-create
    clearable
    :disabled="disabled"
    placeholder="选择或输入 BB Key"
    style="width: 100%"
    @change="handleChange"
  >
    <el-option
      v-for="field in fieldOptions"
      :key="field.name"
      :label="`${field.name} (${field.label})`"
      :value="field.name"
    />
  </el-select>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { fieldApi } from '@/api/fields'
import type { FieldListItem } from '@/api/fields'

const props = defineProps<{
  modelValue: string
  disabled?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
  /** 选中已知字段时携带完整 item（自由输入时为 null） */
  'field-selected': [field: FieldListItem | null]
}>()

const fieldOptions = ref<FieldListItem[]>([])

onMounted(async () => {
  try {
    const res = await fieldApi.list({
      bb_exposed: true,
      enabled: true,
      page: 1,
      page_size: 200,
    })
    fieldOptions.value = res.data?.items || []
  } catch {
    // 拦截器已 toast
  }
})

function handleChange(val: string) {
  emit('update:modelValue', val || '')
  const matched = fieldOptions.value.find((f) => f.name === val)
  emit('field-selected', matched || null)
}
</script>
