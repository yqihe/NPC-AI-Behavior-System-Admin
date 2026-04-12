<template>
  <div class="schema-form">
    <!-- 顶部导航 -->
    <div class="form-header">
      <el-icon class="back-icon" @click="$router.push('/event-type-schemas')"><ArrowLeft /></el-icon>
      <span class="back-text" @click="$router.push('/event-type-schemas')">返回</span>
      <span class="header-sep"></span>
      <span class="header-title">{{ isView ? '查看扩展字段' : isCreate ? '新建扩展字段' : '编辑扩展字段' }}</span>
    </div>

    <!-- 表单卡片 -->
    <div class="form-card">
      <div class="card-inner">
        <el-form
          ref="formRef"
          :model="form"
          :rules="rules"
          :disabled="isView"
          label-width="120px"
          label-position="right"
        >
          <!-- 字段标识 -->
          <el-form-item label="字段标识" prop="field_name">
            <template v-if="!isCreate">
              <el-input :model-value="form.field_name" disabled style="width: 100%">
                <template #prefix>
                  <el-icon><Lock /></el-icon>
                </template>
              </el-input>
              <div class="field-warn">
                <el-icon><WarningFilled /></el-icon>
                标识符创建后不可更改
              </div>
            </template>
            <template v-else>
              <el-input
                v-model="form.field_name"
                placeholder="如 custom_range、extra_weight（小写字母开头，仅含小写字母、数字、下划线）"
                style="width: 100%"
                @blur="checkNameFormat"
              />
              <div v-if="nameStatus === 'valid'" class="field-hint field-hint-success">
                <el-icon><CircleCheck /></el-icon>
                格式正确
              </div>
              <div v-else-if="nameStatus === 'invalid'" class="field-hint field-hint-error">
                <el-icon><CircleClose /></el-icon>
                小写字母开头，仅含小写字母、数字、下划线
              </div>
              <div v-else-if="nameStatus === 'taken'" class="field-hint field-hint-error">
                <el-icon><CircleClose /></el-icon>
                {{ nameMessage }}
              </div>
            </template>
          </el-form-item>

          <!-- 中文名 -->
          <el-form-item label="中文名" prop="field_label">
            <el-input
              v-model="form.field_label"
              placeholder="如 自定义范围、额外权重（策划可见的显示名称）"
              style="width: 100%"
            />
          </el-form-item>

          <!-- 字段类型 -->
          <el-form-item label="字段类型" prop="field_type">
            <el-select
              v-model="form.field_type"
              placeholder="请选择字段类型"
              style="width: 100%"
              :disabled="!isCreate"
              @change="handleTypeChange"
            >
              <el-option label="整数 (int)" value="int" />
              <el-option label="浮点数 (float)" value="float" />
              <el-option label="文本 (string)" value="string" />
              <el-option label="布尔 (bool)" value="bool" />
              <el-option label="选择 (select)" value="select" />
            </el-select>
            <div v-if="!isCreate" class="field-warn">
              <el-icon><WarningFilled /></el-icon>
              字段类型创建后不可更改
            </div>
          </el-form-item>

          <!-- 约束配置 -->
          <el-form-item v-if="form.field_type" label="约束配置">
            <FieldConstraintInteger
              v-if="form.field_type === 'int' || form.field_type === 'float'"
              v-model="form.constraints"
              :restricted="false"
              :type-name="form.field_type"
            />
            <FieldConstraintString
              v-else-if="form.field_type === 'string'"
              v-model="form.constraints"
              :restricted="false"
            />
            <div v-else-if="form.field_type === 'bool'" class="constraint-empty">
              布尔类型无需约束配置
            </div>
            <FieldConstraintSelect
              v-else-if="form.field_type === 'select'"
              v-model="form.constraints"
              :restricted="false"
            />
          </el-form-item>

          <!-- 默认值 -->
          <el-form-item v-if="form.field_type" label="默认值">
            <el-input-number
              v-if="form.field_type === 'int'"
              v-model="form.default_value_number"
              :controls="false"
              :min="constraintMin"
              :max="constraintMax"
              placeholder="选填"
              style="width: 200px"
            />
            <el-input-number
              v-else-if="form.field_type === 'float'"
              v-model="form.default_value_number"
              :controls="false"
              :step="0.1"
              :min="constraintMin"
              :max="constraintMax"
              placeholder="选填"
              style="width: 200px"
            />
            <el-input
              v-else-if="form.field_type === 'string'"
              v-model="form.default_value_string"
              placeholder="选填"
              style="width: 100%"
            />
            <el-switch
              v-else-if="form.field_type === 'bool'"
              v-model="form.default_value_bool"
            />
            <el-select
              v-else-if="form.field_type === 'select'"
              v-model="form.default_value_string"
              placeholder="请选择默认值"
              style="width: 200px"
              clearable
            >
              <el-option
                v-for="opt in selectOptions"
                :key="opt.value"
                :label="opt.label || opt.value"
                :value="opt.value"
              />
            </el-select>
            <div class="field-extra-hint">
              用于事件类型表单的初始值，策划创建事件类型时会自动填入此值
            </div>
          </el-form-item>

          <!-- 排序 -->
          <el-form-item label="排序" prop="sort_order">
            <el-input-number
              v-model="form.sort_order"
              :controls="false"
              :min="0"
              placeholder="数值越小越靠前"
              style="width: 200px"
            />
            <span class="field-extra">数值越小越靠前，默认 0</span>
          </el-form-item>

          <!-- 提交按钮 -->
          <div v-if="!isView" class="form-actions">
            <el-button @click="$router.push('/event-type-schemas')">取消</el-button>
            <el-button
              type="primary"
              :loading="submitting"
              @click="handleSubmit"
            >
              保存
            </el-button>
          </div>
        </el-form>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance } from 'element-plus'
import { ArrowLeft, Lock, WarningFilled, CircleCheck, CircleClose } from '@element-plus/icons-vue'
import { eventTypeApi, EXT_SCHEMA_ERR } from '@/api/eventTypes'
import type { BizError } from '@/api/request'
import FieldConstraintInteger from '@/components/FieldConstraintInteger.vue'
import FieldConstraintString from '@/components/FieldConstraintString.vue'
import FieldConstraintSelect from '@/components/FieldConstraintSelect.vue'

const route = useRoute()
const router = useRouter()
const isCreate = route.meta.isCreate as boolean
const isView = (route.meta.isView as boolean) || false

const formRef = ref<FormInstance>()
const submitting = ref(false)
const nameStatus = ref<'' | 'valid' | 'invalid' | 'taken'>('')
const nameMessage = ref('')
const version = ref(0)

interface SelectOption {
  value: string
  label: string
}

interface FormState {
  field_name: string
  field_label: string
  field_type: string
  constraints: Record<string, unknown>
  default_value_number: number | undefined
  default_value_string: string
  default_value_bool: boolean
  sort_order: number
}

const form = reactive<FormState>({
  field_name: '',
  field_label: '',
  field_type: '',
  constraints: {},
  default_value_number: undefined,
  default_value_string: '',
  default_value_bool: false,
  sort_order: 0,
})

const namePattern = /^[a-z][a-z0-9_]*$/

const rules = {
  field_name: [
    { required: true, message: '请输入字段标识', trigger: 'blur' },
    { pattern: namePattern, message: '小写字母开头，仅含小写字母、数字、下划线', trigger: 'blur' },
  ],
  field_label: [
    { required: true, message: '请输入中文名', trigger: 'blur' },
  ],
  field_type: [
    { required: true, message: '请选择字段类型', trigger: 'change' },
  ],
}

// ---------- 计算属性 ----------

const constraintMin = computed(() => {
  const v = form.constraints.min
  return typeof v === 'number' ? v : undefined
})

const constraintMax = computed(() => {
  const v = form.constraints.max
  return typeof v === 'number' ? v : undefined
})

const selectOptions = computed((): SelectOption[] => {
  const opts = form.constraints.options as SelectOption[] | undefined
  return opts || []
})

// ---------- 初始化 ----------

onMounted(async () => {
  if (!isCreate) {
    await loadDetail()
  }
})

async function loadDetail() {
  const id = Number(route.params.id)
  try {
    const res = await eventTypeApi.schemaList()
    const items = res.data?.items || []
    const target = items.find((s) => s.id === id)
    if (!target) {
      ElMessage.error('扩展字段不存在')
      router.push('/event-type-schemas')
      return
    }
    form.field_name = target.field_name
    form.field_label = target.field_label
    form.field_type = target.field_type
    form.constraints = target.constraints || {}
    form.sort_order = target.sort_order
    version.value = target.version

    // 按类型还原默认值
    setDefaultValueFromLoaded(target.field_type, target.default_value)
  } catch {
    // 拦截器已 toast
  }
}

function setDefaultValueFromLoaded(fieldType: string, val: unknown) {
  if (fieldType === 'int' || fieldType === 'float') {
    form.default_value_number = typeof val === 'number' ? val : undefined
  } else if (fieldType === 'string' || fieldType === 'select') {
    form.default_value_string = typeof val === 'string' ? val : ''
  } else if (fieldType === 'bool') {
    form.default_value_bool = Boolean(val)
  }
}

// ---------- 标识符格式校验 ----------

function checkNameFormat() {
  if (!form.field_name) {
    nameStatus.value = ''
    return
  }
  if (namePattern.test(form.field_name)) {
    nameStatus.value = 'valid'
  } else {
    nameStatus.value = 'invalid'
  }
}

// ---------- 类型切换 ----------

function handleTypeChange() {
  form.constraints = {}
  form.default_value_number = undefined
  form.default_value_string = ''
  form.default_value_bool = false
}

// ---------- 构建默认值 ----------

function buildDefaultValue(): unknown {
  switch (form.field_type) {
    case 'int':
      return form.default_value_number ?? 0
    case 'float':
      return form.default_value_number ?? 0
    case 'string':
      return form.default_value_string
    case 'bool':
      return form.default_value_bool
    case 'select':
      return form.default_value_string
    default:
      return null
  }
}

// ---------- 提交 ----------

async function handleSubmit() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  if (isCreate && nameStatus.value === 'invalid') {
    ElMessage.warning('字段标识格式不正确')
    return
  }

  submitting.value = true
  try {
    const defaultValue = buildDefaultValue()

    if (isCreate) {
      await eventTypeApi.schemaCreate({
        field_name: form.field_name,
        field_label: form.field_label,
        field_type: form.field_type,
        constraints: form.constraints,
        default_value: defaultValue,
        sort_order: form.sort_order,
      })
      ElMessage.success('创建成功，扩展字段默认为启用状态')
    } else {
      await eventTypeApi.schemaUpdate({
        id: Number(route.params.id),
        field_label: form.field_label,
        constraints: form.constraints,
        default_value: defaultValue,
        sort_order: form.sort_order,
        version: version.value,
      })
      ElMessage.success('保存成功')
    }
    router.push('/event-type-schemas')
  } catch (err: unknown) {
    const bizErr = err as BizError
    if (bizErr.code === EXT_SCHEMA_ERR.VERSION_CONFLICT) {
      ElMessageBox.alert('数据已被其他用户修改，请返回列表刷新后重试。', '版本冲突', { type: 'warning' })
      return
    }
    if (bizErr.code === EXT_SCHEMA_ERR.NAME_EXISTS) {
      nameStatus.value = 'taken'
      nameMessage.value = '该字段标识已存在（包括已删除的记录）'
      return
    }
    if (bizErr.code === EXT_SCHEMA_ERR.NAME_INVALID) {
      nameStatus.value = 'taken'
      nameMessage.value = bizErr.message || '字段标识格式不正确'
      return
    }
    if (bizErr.code === EXT_SCHEMA_ERR.EDIT_NOT_DISABLED) {
      ElMessage.warning('请先禁用该扩展字段后再编辑')
      return
    }
    if (bizErr.code === EXT_SCHEMA_ERR.NOT_FOUND) {
      ElMessage.error('扩展字段不存在')
      router.push('/event-type-schemas')
      return
    }
    // 其他错误（CONSTRAINTS_INVALID / DEFAULT_INVALID / TYPE_INVALID）拦截器已 toast
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.schema-form {
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.form-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 16px 24px;
  background: #fff;
  border-bottom: 1px solid #E4E7ED;
}

.back-icon {
  color: #409EFF;
  font-size: 18px;
  cursor: pointer;
}

.back-text {
  color: #409EFF;
  font-size: 14px;
  cursor: pointer;
}

.header-sep {
  width: 1px;
  height: 16px;
  background: #DCDFE6;
}

.header-title {
  font-size: 16px;
  font-weight: 600;
  color: #303133;
}

.form-card {
  flex: 1;
  padding: 24px 32px;
  overflow-y: auto;
}

.card-inner {
  max-width: 800px;
  margin: 0 auto;
  background: #fff;
  border: 1px solid #E4E7ED;
  border-radius: 8px;
  padding: 32px;
}

.field-hint {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 4px;
  font-size: 12px;
  color: #909399;
}

.field-hint-success {
  color: #67C23A;
}

.field-hint-error {
  color: #F56C6C;
}

.field-warn {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 4px;
  font-size: 12px;
  color: #E6A23C;
}

.field-extra {
  margin-left: 12px;
  font-size: 12px;
  color: #909399;
}

.field-extra-hint {
  margin-top: 4px;
  font-size: 12px;
  color: #909399;
}

.constraint-empty {
  padding: 16px;
  background: #F5F7FA;
  border-radius: 4px;
  color: #909399;
  font-size: 13px;
  width: 100%;
  text-align: center;
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  padding-top: 16px;
  gap: 12px;
}
</style>
