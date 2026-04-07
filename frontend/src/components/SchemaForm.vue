<template>
  <div class="schema-form">
    <!-- 有 schema 时用 VueForm 渲染 -->
    <vue-form
      v-if="hasSchema"
      v-model="localData"
      :schema="processedSchema"
      :ui-schema="dynamicUiSchema"
      :form-footer="formFooter"
      :form-props="formProps"
      @submit="handleSubmit"
    />

    <!-- 无 schema 时降级为 JSON 编辑器 -->
    <div v-else class="json-editor-fallback">
      <el-alert
        type="info"
        :closable="false"
        show-icon
        title="自由编辑模式"
        description="未找到对应的 Schema 定义，请直接编辑 JSON 配置"
        style="margin-bottom: 16px"
      />
      <el-input
        v-model="jsonText"
        type="textarea"
        :rows="12"
        placeholder="请输入 JSON 格式的配置内容"
        @blur="parseJsonText"
      />
      <div v-if="jsonError" style="color: #f56c6c; margin-top: 4px; font-size: 12px">
        {{ jsonError }}
      </div>
      <div style="margin-top: 16px">
        <el-button type="primary" :disabled="!!jsonError" @click="handleJsonSubmit">
          保存
        </el-button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import VueForm from '@lljj/vue3-form-element'

const props = defineProps({
  modelValue: {
    type: Object,
    default: () => ({}),
  },
  schema: {
    type: Object,
    default: null,
  },
  readonly: {
    type: Boolean,
    default: false,
  },
})

const emit = defineEmits(['update:modelValue', 'submit'])

// ========== Schema 模式 ==========

const hasSchema = computed(() => {
  return props.schema && props.schema.type === 'object'
})

// 本地表单数据（双向绑定）
const localData = ref({ ...props.modelValue })

watch(() => props.modelValue, (val) => {
  localData.value = { ...val }
}, { deep: true })

watch(localData, (val) => {
  emit('update:modelValue', { ...val })
}, { deep: true })

// 处理 schema：移除 allOf（VueForm 不支持），保留所有字段为可选
const processedSchema = computed(() => {
  if (!props.schema) return {}

  const schema = JSON.parse(JSON.stringify(props.schema))

  // 移除 $schema 字段（VueForm 不需要）
  delete schema.$schema

  // 移除 allOf（条件字段在 T5 中用 uiSchema 处理）
  if (schema.allOf) {
    delete schema.allOf
  }

  return schema
})

// 条件字段的 uiSchema（T5 实现，此处占位）
const dynamicUiSchema = computed(() => {
  return {}
})

// VueForm 配置
const formFooter = computed(() => ({
  show: !props.readonly,
  okBtn: '保存',
  cancelBtn: '',
}))

const formProps = {
  labelPosition: 'top',
  labelWidth: 'auto',
}

function handleSubmit(data) {
  emit('submit', data)
}

// ========== JSON 编辑器模式 ==========

const jsonText = ref(JSON.stringify(props.modelValue || {}, null, 2))
const jsonError = ref('')

watch(() => props.modelValue, (val) => {
  jsonText.value = JSON.stringify(val || {}, null, 2)
}, { deep: true })

function parseJsonText() {
  try {
    const parsed = JSON.parse(jsonText.value)
    jsonError.value = ''
    emit('update:modelValue', parsed)
  } catch (e) {
    jsonError.value = 'JSON 格式错误：' + e.message
  }
}

function handleJsonSubmit() {
  parseJsonText()
  if (!jsonError.value) {
    emit('submit', JSON.parse(jsonText.value))
  }
}
</script>

<style scoped>
.schema-form {
  max-width: 800px;
}
.json-editor-fallback :deep(.el-textarea__inner) {
  font-family: 'Courier New', monospace;
  font-size: 13px;
}
</style>
