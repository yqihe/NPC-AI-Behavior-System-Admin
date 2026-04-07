<template>
  <div class="schema-form">
    <!-- schema 加载中 -->
    <el-skeleton v-if="schemaLoading" :rows="4" animated />

    <!-- 有 schema 时渲染表单 -->
    <el-form
      v-else-if="hasSchema"
      :model="localData"
      label-position="top"
    >
      <template v-for="field in visibleFields" :key="field.name">
        <!-- 字符串 + 枚举 → 下拉选择 -->
        <el-form-item
          v-if="field.type === 'string' && field.enum"
          :label="field.title"
          :required="field.required"
        >
          <el-select
            :model-value="localData[field.name]"
            :placeholder="`请选择${field.title}`"
            style="width: 100%"
            @change="updateField(field.name, $event)"
          >
            <el-option v-for="opt in field.enum" :key="opt" :label="opt" :value="opt" />
          </el-select>
          <div v-if="field.description" class="field-desc">{{ field.description }}</div>
        </el-form-item>

        <!-- 字符串 → 文本输入 -->
        <el-form-item
          v-else-if="field.type === 'string'"
          :label="field.title"
          :required="field.required"
        >
          <el-input
            :model-value="localData[field.name] || ''"
            :placeholder="`请输入${field.title}`"
            @input="updateField(field.name, $event)"
          />
          <div v-if="field.description" class="field-desc">{{ field.description }}</div>
        </el-form-item>

        <!-- 数字（有 min/max）→ 滑块 -->
        <el-form-item
          v-else-if="field.type === 'number' && field.minimum !== undefined && field.maximum !== undefined"
          :label="field.title"
          :required="field.required"
        >
          <el-slider
            :model-value="localData[field.name] ?? field.default ?? field.minimum"
            :min="field.minimum"
            :max="field.maximum"
            :step="field.maximum <= 1 ? 0.1 : 1"
            show-input
            @input="updateField(field.name, $event)"
          />
          <div v-if="field.description" class="field-desc">{{ field.description }}</div>
        </el-form-item>

        <!-- 数字/整数 → 数字输入 -->
        <el-form-item
          v-else-if="field.type === 'number' || field.type === 'integer'"
          :label="field.title"
          :required="field.required"
        >
          <el-input-number
            :model-value="localData[field.name] ?? field.default ?? 0"
            :min="field.minimum"
            :max="field.maximum"
            :step="field.type === 'integer' ? 1 : 0.1"
            :precision="field.type === 'integer' ? 0 : 1"
            style="width: 200px"
            @change="updateField(field.name, $event)"
          />
          <div v-if="field.description" class="field-desc">{{ field.description }}</div>
        </el-form-item>

        <!-- 数组（字符串枚举项）→ 多选 -->
        <el-form-item
          v-else-if="field.type === 'array' && field.items?.enum"
          :label="field.title"
          :required="field.required"
        >
          <el-checkbox-group
            :model-value="localData[field.name] || []"
            @change="updateField(field.name, $event)"
          >
            <el-checkbox v-for="opt in field.items.enum" :key="opt" :label="opt" :value="opt" />
          </el-checkbox-group>
          <div v-if="field.description" class="field-desc">{{ field.description }}</div>
        </el-form-item>

        <!-- 数组（字符串项）→ 标签输入 -->
        <el-form-item
          v-else-if="field.type === 'array' && field.items?.type === 'string'"
          :label="field.title"
          :required="field.required"
        >
          <div class="tag-input">
            <el-tag
              v-for="(item, idx) in (localData[field.name] || [])"
              :key="idx"
              closable
              style="margin: 2px"
              @close="removeArrayItem(field.name, idx)"
            >{{ item }}</el-tag>
            <el-input
              v-model="tagInputs[field.name]"
              size="small"
              placeholder="输入后回车添加"
              style="width: 140px"
              @keyup.enter="addArrayItem(field.name)"
            />
          </div>
          <div v-if="field.description" class="field-desc">{{ field.description }}</div>
        </el-form-item>

        <!-- 对象 → 嵌套字段（一层）-->
        <el-form-item
          v-else-if="field.type === 'object' && field.properties"
          :label="field.title"
          :required="field.required"
        >
          <el-card shadow="never" style="width: 100%">
            <template v-for="(subProp, subKey) in field.properties" :key="subKey">
              <el-form-item
                :label="subProp.title || subKey"
                style="margin-bottom: 12px"
              >
                <el-input-number
                  v-if="subProp.type === 'number' || subProp.type === 'integer'"
                  :model-value="(localData[field.name] || {})[subKey] ?? subProp.default ?? 0"
                  :min="subProp.minimum"
                  :max="subProp.maximum"
                  :step="subProp.type === 'integer' ? 1 : 0.1"
                  style="width: 200px"
                  @change="updateNestedField(field.name, subKey, $event)"
                />
                <el-input
                  v-else
                  :model-value="(localData[field.name] || {})[subKey] || ''"
                  :placeholder="subProp.description || ''"
                  @input="updateNestedField(field.name, subKey, $event)"
                />
              </el-form-item>
            </template>
          </el-card>
          <div v-if="field.description" class="field-desc">{{ field.description }}</div>
        </el-form-item>

        <!-- 其他类型 → 文本输入 fallback -->
        <el-form-item v-else :label="field.title" :required="field.required">
          <el-input
            :model-value="localData[field.name] != null ? String(localData[field.name]) : ''"
            :placeholder="field.description || ''"
            @input="updateField(field.name, $event)"
          />
          <div v-if="field.description" class="field-desc">{{ field.description }}</div>
        </el-form-item>
      </template>

      <!-- 保存按钮 -->
      <el-form-item v-if="!readonly && showSubmit" style="margin-top: 24px">
        <el-button type="primary" @click="handleSubmit">保存</el-button>
      </el-form-item>
    </el-form>

    <!-- 无 schema -->
    <el-alert
      v-else
      type="warning"
      :closable="false"
      show-icon
      title="该配置类型尚未定义表单模板"
      description="请联系开发人员导入对应的 Schema 定义后再进行配置"
    />
  </div>
</template>

<script setup>
import { ref, reactive, computed, watch } from 'vue'

const props = defineProps({
  modelValue: { type: Object, default: () => ({}) },
  schema: { type: Object, default: null },
  readonly: { type: Boolean, default: false },
  schemaLoading: { type: Boolean, default: false },
  showSubmit: { type: Boolean, default: true },
})

const emit = defineEmits(['update:modelValue', 'submit'])

const hasSchema = computed(() => props.schema && props.schema.type === 'object' && props.schema.properties)

// 本地数据
const localData = ref({ ...props.modelValue })

watch(() => props.modelValue, (val) => {
  if (JSON.stringify(val) !== JSON.stringify(localData.value)) {
    localData.value = { ...val }
  }
}, { deep: true })

// ========== 条件字段 ==========

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
          rules.push({ triggerField: field, triggerValue: condition.const, showFields })
        }
      }
    }
  }
  return rules
}

function collectConditionalFields(rules) {
  const fields = new Set()
  for (const rule of rules) {
    for (const f of rule.showFields) fields.add(f)
  }
  return fields
}

const conditionalRules = computed(() => extractConditionalRules(props.schema))
const allConditionalFields = computed(() => collectConditionalFields(conditionalRules.value))

function isFieldVisible(fieldName) {
  if (!allConditionalFields.value.has(fieldName)) return true
  for (const rule of conditionalRules.value) {
    if (rule.showFields.includes(fieldName) && localData.value[rule.triggerField] === rule.triggerValue) {
      return true
    }
  }
  return false
}

// ========== 字段列表 ==========

const visibleFields = computed(() => {
  if (!props.schema?.properties) return []
  const required = props.schema.required || []

  return Object.entries(props.schema.properties)
    .filter(([name]) => isFieldVisible(name))
    .map(([name, prop]) => ({
      name,
      title: prop.title || name,
      description: prop.description || '',
      type: prop.type || 'string',
      required: required.includes(name),
      enum: prop.enum || null,
      minimum: prop.minimum,
      maximum: prop.maximum,
      default: prop.default,
      items: prop.items || null,
      properties: prop.properties || null,
    }))
})

// ========== 数据更新 ==========

function updateField(name, value) {
  localData.value = { ...localData.value, [name]: value }
  emit('update:modelValue', { ...localData.value })
}

function updateNestedField(parentName, subKey, value) {
  const parent = { ...(localData.value[parentName] || {}) }
  parent[subKey] = value
  updateField(parentName, parent)
}

// 标签输入辅助
const tagInputs = reactive({})

function addArrayItem(fieldName) {
  const val = (tagInputs[fieldName] || '').trim()
  if (!val) return
  const arr = [...(localData.value[fieldName] || []), val]
  updateField(fieldName, arr)
  tagInputs[fieldName] = ''
}

function removeArrayItem(fieldName, index) {
  const arr = [...(localData.value[fieldName] || [])]
  arr.splice(index, 1)
  updateField(fieldName, arr)
}

function handleSubmit() {
  emit('submit', { ...localData.value })
}
</script>

<style scoped>
.schema-form {
  max-width: 800px;
}
.field-desc {
  color: #909399;
  font-size: 12px;
  margin-top: 4px;
  line-height: 1.4;
}
.tag-input {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 4px;
}
</style>
