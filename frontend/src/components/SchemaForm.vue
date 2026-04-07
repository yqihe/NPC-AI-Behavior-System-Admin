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

// 本地表单数据（双向绑定，JSON 比较防止循环触发）
const localData = ref({ ...props.modelValue })

watch(() => props.modelValue, (val) => {
  if (JSON.stringify(val) !== JSON.stringify(localData.value)) {
    localData.value = { ...val }
  }
}, { deep: true })

watch(localData, (val) => {
  if (JSON.stringify(val) !== JSON.stringify(props.modelValue)) {
    emit('update:modelValue', { ...val })
  }
}, { deep: true })

// ========== 条件字段解析 ==========

/**
 * 从 schema.allOf 中提取 if/then 条件规则。
 * 格式：[{ triggerField, triggerValue, showFields: [field1, field2] }]
 *
 * 示例输入（movement schema）：
 *   allOf: [
 *     { if: { properties: { move_type: { const: "wander" } } }, then: { required: ["wander_radius"] } }
 *   ]
 * 输出：[{ triggerField: "move_type", triggerValue: "wander", showFields: ["wander_radius"] }]
 */
function extractConditionalRules(schema) {
  if (!schema || !schema.allOf) return []

  const rules = []
  for (const entry of schema.allOf) {
    if (!entry.if || !entry.then) continue

    const ifProps = entry.if.properties
    if (!ifProps) continue

    for (const [field, condition] of Object.entries(ifProps)) {
      if (condition.const !== undefined) {
        const showFields = entry.then.required || []
        if (showFields.length > 0) {
          rules.push({
            triggerField: field,
            triggerValue: condition.const,
            showFields,
          })
        }
      }
    }
  }
  return rules
}

/**
 * 收集所有条件字段名（这些字段只在条件满足时显示）。
 */
function collectConditionalFields(rules) {
  const fields = new Set()
  for (const rule of rules) {
    for (const f of rule.showFields) {
      fields.add(f)
    }
  }
  return fields
}

// 条件规则（从原始 schema 提取）
const conditionalRules = computed(() => extractConditionalRules(props.schema))
const conditionalFields = computed(() => collectConditionalFields(conditionalRules.value))

// 处理 schema：移除 allOf（VueForm 不支持），保留所有字段为可选
const processedSchema = computed(() => {
  if (!props.schema) return {}

  const schema = JSON.parse(JSON.stringify(props.schema))

  // 移除 $schema 字段（VueForm 不需要）
  delete schema.$schema

  // 移除 allOf（条件字段用 uiSchema 控制显隐）
  if (schema.allOf) {
    delete schema.allOf
  }

  // 条件字段从 required 中移除（由条件逻辑控制）
  if (schema.required && conditionalFields.value.size > 0) {
    schema.required = schema.required.filter(f => !conditionalFields.value.has(f))
  }

  return schema
})

// 动态 uiSchema：根据当前表单数据控制条件字段的显隐
const dynamicUiSchema = computed(() => {
  const ui = {}
  const rules = conditionalRules.value

  if (rules.length === 0) return ui

  for (const field of conditionalFields.value) {
    // 默认隐藏所有条件字段
    let shouldShow = false

    for (const rule of rules) {
      if (rule.showFields.includes(field)) {
        // 检查触发条件是否满足
        if (localData.value[rule.triggerField] === rule.triggerValue) {
          shouldShow = true
          break
        }
      }
    }

    if (!shouldShow) {
      ui[field] = { 'ui:hidden': true }
    }
  }

  return ui
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
