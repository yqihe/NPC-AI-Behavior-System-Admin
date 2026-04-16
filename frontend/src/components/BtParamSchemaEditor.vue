<template>
  <div class="bt-param-schema-editor">
    <!-- 参数行列表 -->
    <div v-if="modelValue.length > 0" class="param-rows">
      <!-- 表头 -->
      <div class="param-header">
        <div class="col-name">参数标识</div>
        <div class="col-label">显示名称</div>
        <div class="col-type">类型</div>
        <div class="col-required">必填</div>
        <div class="col-options">选项（select 类型）</div>
        <div v-if="!disabled" class="col-action"></div>
      </div>

      <!-- 每行数据 -->
      <div
        v-for="(row, idx) in modelValue"
        :key="idx"
        class="param-row"
      >
        <!-- name -->
        <div class="col-name">
          <el-input
            :model-value="row.name"
            :disabled="disabled"
            placeholder="如 speed"
            @input="(v: string) => updateField(idx, 'name', v)"
          />
        </div>

        <!-- label -->
        <div class="col-label">
          <el-input
            :model-value="row.label"
            :disabled="disabled"
            placeholder="如 移动速度"
            @input="(v: string) => updateField(idx, 'label', v)"
          />
        </div>

        <!-- type -->
        <div class="col-type">
          <el-select
            :model-value="row.type"
            :disabled="disabled"
            style="width: 100%"
            @change="(v: BtParamDef['type']) => updateField(idx, 'type', v)"
          >
            <el-option v-for="t in TYPE_OPTIONS" :key="t.value" :label="t.label" :value="t.value" />
          </el-select>
        </div>

        <!-- required -->
        <div class="col-required">
          <el-switch
            :model-value="row.required"
            :disabled="disabled"
            @change="(v: boolean) => updateField(idx, 'required', v)"
          />
        </div>

        <!-- options (only when type === 'select') -->
        <div class="col-options">
          <el-input
            v-if="row.type === 'select'"
            :model-value="row.options?.join(',') ?? ''"
            :disabled="disabled"
            placeholder="如 idle,patrol,flee"
            @input="(v: string) => updateOptions(idx, v)"
          />
          <span v-else class="options-na">—</span>
        </div>

        <!-- delete -->
        <div v-if="!disabled" class="col-action">
          <el-button type="danger" link size="small" @click="removeRow(idx)">删除</el-button>
        </div>
      </div>
    </div>

    <!-- 空状态 -->
    <div v-else class="param-empty">暂无参数，点击下方按钮添加</div>

    <!-- 添加按钮 -->
    <el-button
      v-if="!disabled"
      text
      type="primary"
      class="add-btn"
      @click="addRow"
    >
      + 添加参数
    </el-button>
  </div>
</template>

<script setup lang="ts">
export interface BtParamDef {
  name: string
  label: string
  type: 'bb_key' | 'select' | 'float' | 'integer' | 'bool' | 'string'
  required: boolean
  options?: string[]
}

const TYPE_OPTIONS: { label: string; value: BtParamDef['type'] }[] = [
  { label: 'bb_key', value: 'bb_key' },
  { label: 'select', value: 'select' },
  { label: 'float', value: 'float' },
  { label: 'integer', value: 'integer' },
  { label: 'bool', value: 'bool' },
  { label: 'string', value: 'string' },
]

const NAME_PATTERN = /^[a-z][a-z0-9_]*$/

const props = withDefaults(defineProps<{
  modelValue: BtParamDef[]
  disabled?: boolean
}>(), {
  disabled: false,
})

const emit = defineEmits<{
  'update:modelValue': [value: BtParamDef[]]
}>()

function cloneRows(): BtParamDef[] {
  return props.modelValue.map((r: BtParamDef) => ({ ...r }))
}

function addRow() {
  const rows = cloneRows()
  rows.push({ name: '', label: '', type: 'string', required: false, options: [] })
  emit('update:modelValue', rows)
}

function removeRow(idx: number) {
  const rows = cloneRows().filter((_, i) => i !== idx)
  emit('update:modelValue', rows)
}

function updateField<K extends keyof BtParamDef>(idx: number, field: K, value: BtParamDef[K]) {
  const rows = cloneRows()
  rows[idx][field] = value
  emit('update:modelValue', rows)
}

function updateOptions(idx: number, raw: string) {
  const rows = cloneRows()
  rows[idx].options = raw.split(',').map((s: string) => s.trim()).filter((s: string) => s !== '')
  emit('update:modelValue', rows)
}

function validate(): string | null {
  const rows = props.modelValue
  const seenNames = new Set<string>()

  for (let i = 0; i < rows.length; i++) {
    const row = rows[i]
    const rowNum = i + 1

    if (!row.name) {
      return `第 ${rowNum} 行：参数标识不能为空`
    }
    if (!row.label) {
      return `第 ${rowNum} 行：显示名称不能为空`
    }
    if (!NAME_PATTERN.test(row.name)) {
      return `第 ${rowNum} 行：参数标识格式错误（只允许小写字母、数字、下划线，以字母开头）`
    }
    if (seenNames.has(row.name)) {
      return `参数标识重复：${row.name}`
    }
    seenNames.add(row.name)
    if (row.type === 'select') {
      const opts = (row.options ?? []).filter((s: string) => s !== '')
      if (opts.length === 0) {
        return `第 ${rowNum} 行：select 类型必须填写选项`
      }
    }
  }

  return null
}

defineExpose({ validate })
</script>

<style scoped>
.bt-param-schema-editor {
  width: 100%;
}

.param-rows {
  border: 1px solid #E4E7ED;
  border-radius: 4px;
  overflow: hidden;
  margin-bottom: 12px;
}

.param-header,
.param-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
}

.param-header {
  background: #F5F7FA;
  font-size: 12px;
  color: #606266;
  font-weight: 500;
  border-bottom: 1px solid #E4E7ED;
}

.param-row {
  border-bottom: 1px solid #F2F6FC;
}

.param-row:last-child {
  border-bottom: none;
}

.col-name {
  flex: 1.2;
  min-width: 0;
}

.col-label {
  flex: 1.2;
  min-width: 0;
}

.col-type {
  flex: 0 0 110px;
}

.col-required {
  flex: 0 0 52px;
  display: flex;
  justify-content: center;
}

.col-options {
  flex: 1.5;
  min-width: 0;
}

.col-action {
  flex: 0 0 48px;
  display: flex;
  justify-content: center;
}

.options-na {
  color: #C0C4CC;
  font-size: 13px;
}

.param-empty {
  color: #909399;
  font-size: 13px;
  padding: 12px 0;
  text-align: center;
  margin-bottom: 8px;
}

.add-btn {
  padding-left: 0;
}
</style>
