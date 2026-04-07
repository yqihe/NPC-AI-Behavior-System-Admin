<template>
  <el-collapse-item :name="componentName">
    <template #title>
      <span class="panel-title">
        {{ displayName }}
        <span class="panel-name">({{ componentName }})</span>
        <el-tag v-if="required" type="danger" size="small" style="margin-left: 8px">必选</el-tag>
      </span>
    </template>
    <schema-form
      v-model="localData"
      :schema="schema"
      :form-footer="{ show: false }"
    />
  </el-collapse-item>
</template>

<script setup>
import { ref, watch } from 'vue'
import SchemaForm from '@/components/SchemaForm.vue'

const props = defineProps({
  componentName: {
    type: String,
    required: true,
  },
  displayName: {
    type: String,
    default: '',
  },
  schema: {
    type: Object,
    default: null,
  },
  modelValue: {
    type: Object,
    default: () => ({}),
  },
  required: {
    type: Boolean,
    default: false,
  },
})

const emit = defineEmits(['update:modelValue'])

const localData = ref({ ...props.modelValue })

watch(() => props.modelValue, (val) => {
  localData.value = { ...val }
}, { deep: true })

watch(localData, (val) => {
  emit('update:modelValue', { ...val })
}, { deep: true })
</script>

<style scoped>
.panel-title {
  font-weight: 500;
  font-size: 14px;
}
.panel-name {
  color: #909399;
  font-weight: normal;
  margin-left: 4px;
}
</style>
