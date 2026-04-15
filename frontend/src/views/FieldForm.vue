<template>
  <div class="field-form">
    <!-- 顶部导航 -->
    <div class="form-header">
      <el-icon class="back-icon" @click="$router.push('/fields')"><ArrowLeft /></el-icon>
      <span class="back-text" @click="$router.push('/fields')">返回</span>
      <span class="header-sep"></span>
      <span class="header-title">{{ isView ? '查看字段' : isCreate ? '新建字段' : '编辑字段' }}</span>
    </div>

    <!-- 表单滚动区 -->
    <div class="form-scroll">
      <div class="form-body">
        <div class="form-card">
        <el-form
          ref="formRef"
          :model="form"
          :rules="rules"
          :disabled="isView"
          label-width="120px"
          label-position="right"
        >
          <!-- 标识符 -->
          <el-form-item label="字段标识符" prop="name">
            <template v-if="!isCreate">
              <el-input
                :model-value="form.name"
                disabled
                style="width: 100%"
              >
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
                v-model="form.name"
                placeholder="如 health、attack_power（小写字母开头，仅含小写字母、数字、下划线）"
                style="width: 100%"
                @blur="checkNameUnique"
              />
              <div v-if="nameStatus === 'checking'" class="field-hint">
                <el-icon class="is-loading"><Loading /></el-icon>
                校验中...
              </div>
              <div v-else-if="nameStatus === 'available'" class="field-hint field-hint-success">
                <el-icon><CircleCheck /></el-icon>
                标识符可用
              </div>
              <div v-else-if="nameStatus === 'taken'" class="field-hint field-hint-error">
                <el-icon><CircleClose /></el-icon>
                {{ nameMessage }}
              </div>
            </template>
          </el-form-item>

          <!-- 中文标签 -->
          <el-form-item label="中文标签" prop="label">
            <el-input
              v-model="form.label"
              placeholder="如 生命值、攻击力（策划可见的显示名称）"
              style="width: 100%"
            />
          </el-form-item>

          <!-- 描述 -->
          <el-form-item label="描述">
            <el-input
              v-model="form.properties.description"
              type="textarea"
              :rows="3"
              placeholder="选填，描述该字段的用途和含义"
              style="width: 100%"
            />
          </el-form-item>

          <!-- 字段类型 -->
          <el-form-item label="字段类型" prop="type">
            <el-select
              v-model="form.type"
              placeholder="请选择字段类型"
              style="width: 100%"
              :disabled="isView || (!isCreate && hasRefs)"
              @change="handleTypeChange"
            >
              <el-option
                v-for="item in typeOptions"
                :key="item.name"
                :label="`${item.label} (${item.name})`"
                :value="item.name"
              />
            </el-select>
            <div v-if="!isCreate && hasRefs" class="field-warn">
              <el-icon><WarningFilled /></el-icon>
              该字段被引用中，无法更改类型
            </div>
          </el-form-item>

          <!-- 分类 -->
          <el-form-item label="字段分类" prop="category">
            <el-select
              v-model="form.category"
              placeholder="请选择字段分类"
              style="width: 100%"
            >
              <el-option
                v-for="item in categoryOptions"
                :key="item.name"
                :label="item.label"
                :value="item.name"
              />
            </el-select>
          </el-form-item>

          <!-- 暴露 BB Key -->
          <el-form-item label="暴露 BB Key">
            <el-radio-group v-model="form.properties.expose_bb">
              <el-radio :value="false">否</el-radio>
              <el-radio :value="true">是（状态机和行为树可读取该字段）</el-radio>
            </el-radio-group>
          </el-form-item>

          <!-- 默认值 -->
          <el-form-item v-if="form.type" label="默认值">
            <el-input-number
              v-if="form.type === 'integer'"
              v-model="form.properties.default_value"
              :controls="false"
              placeholder="选填"
              style="width: 200px"
            />
            <el-input-number
              v-else-if="form.type === 'float'"
              v-model="form.properties.default_value"
              :controls="false"
              :step="0.1"
              placeholder="选填"
              style="width: 200px"
            />
            <el-input
              v-else-if="form.type === 'string'"
              v-model="form.properties.default_value"
              placeholder="选填"
              style="width: 100%"
            />
            <el-switch
              v-else-if="form.type === 'boolean'"
              v-model="form.properties.default_value"
            />
            <span v-else class="default-hint">
              {{ form.type === 'select' ? '默认值自动取第一个选项' : '引用类型无默认值' }}
            </span>
          </el-form-item>

          <!-- 约束配置 -->
          <el-form-item v-if="form.type" label="约束配置">
            <FieldConstraintInteger
              v-if="form.type === 'integer' || form.type === 'float'"
              ref="constraintRef"
              v-model="form.properties.constraints"
              :restricted="hasRefs"
              :type-name="form.type"
            />
            <FieldConstraintString
              v-else-if="form.type === 'string'"
              ref="constraintRef"
              v-model="form.properties.constraints"
              :restricted="hasRefs"
            />
            <div v-else-if="form.type === 'boolean'" class="constraint-empty">
              布尔类型无需约束配置
            </div>
            <FieldConstraintSelect
              v-else-if="form.type === 'select'"
              ref="constraintRef"
              v-model="form.properties.constraints"
              :restricted="hasRefs"
              :disabled="isView"
            />
            <FieldConstraintReference
              v-else-if="form.type === 'reference'"
              v-model="form.properties.constraints"
              :restricted="hasRefs"
              :disabled="isView"
              :current-field-id="isCreate ? 0 : Number(route.params.id)"
            />
          </el-form-item>

          <!-- 查看模式下展示时间戳 -->
          <template v-if="isView">
            <el-form-item label="创建时间">
              <el-input :model-value="formatTime(createdAt)" :disabled="true" style="width: 200px" />
            </el-form-item>
            <el-form-item label="更新时间">
              <el-input :model-value="formatTime(updatedAt)" :disabled="true" style="width: 200px" />
            </el-form-item>
          </template>

        </el-form>
        </div>
      </div>
    </div>

    <!-- 底部操作栏（查看模式隐藏） -->
    <div v-if="!isView" class="form-footer">
      <el-button @click="$router.push('/fields')">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="handleSubmit">保存</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance } from 'element-plus'
import { ArrowLeft, Lock, WarningFilled, Loading, CircleCheck, CircleClose } from '@element-plus/icons-vue'
import { fieldApi, FIELD_ERR } from '@/api/fields'
import type { BizError } from '@/api/request'
import { formatTime } from '@/utils/format'
import { dictApi } from '@/api/dictionaries'
import type { DictionaryItem } from '@/api/dictionaries'
import FieldConstraintInteger from '@/components/FieldConstraintInteger.vue'
import FieldConstraintString from '@/components/FieldConstraintString.vue'
import FieldConstraintSelect from '@/components/FieldConstraintSelect.vue'
import FieldConstraintReference from '@/components/FieldConstraintReference.vue'

const route = useRoute()
const router = useRouter()
const isCreate = route.meta.isCreate as boolean
const isView = (route.meta.isView as boolean) || false

const formRef = ref<FormInstance>()
const constraintRef = ref<{ validate: () => string | null } | null>(null)
const submitting = ref(false)
const nameStatus = ref<'' | 'checking' | 'available' | 'taken'>('')
const nameMessage = ref('')
const typeOptions = ref<DictionaryItem[]>([])
const categoryOptions = ref<DictionaryItem[]>([])
const version = ref(0)
const hasRefs = ref(false)
const createdAt = ref('')
const updatedAt = ref('')

interface FormState {
  name: string
  label: string
  type: string
  category: string
  properties: {
    description: string
    expose_bb: boolean
    default_value: unknown
    constraints: Record<string, unknown>
  }
}

const form = reactive<FormState>({
  name: '',
  label: '',
  type: '',
  category: '',
  properties: {
    description: '',
    expose_bb: false,
    default_value: null,
    constraints: {},
  },
})

const namePattern = /^[a-z][a-z0-9_]*$/

const rules = {
  name: [
    { required: true, message: '请输入字段标识符', trigger: 'blur' },
    { pattern: namePattern, message: '小写字母开头，仅含小写字母、数字、下划线', trigger: 'blur' },
  ],
  label: [
    { required: true, message: '请输入中文标签', trigger: 'blur' },
  ],
  type: [
    { required: true, message: '请选择字段类型', trigger: 'change' },
  ],
  category: [
    { required: true, message: '请选择字段分类', trigger: 'change' },
  ],
}

// ---------- 初始化 ----------

onMounted(async () => {
  loadDictionaries()
  if (!isCreate) {
    await loadFieldDetail()
  }
})

async function loadDictionaries() {
  try {
    const [typeRes, catRes] = await Promise.all([
      dictApi.list('field_type'),
      dictApi.list('field_category'),
    ])
    typeOptions.value = typeRes.data?.items || []
    categoryOptions.value = catRes.data?.items || []
  } catch {
    // 拦截器已 toast
  }
}

async function loadFieldDetail() {
  const id = Number(route.params.id)
  try {
    const res = await fieldApi.detail(id)
    const data = res.data
    form.name = data.name
    form.label = data.label
    form.type = data.type
    form.category = data.category
    if (data.properties) {
      form.properties.description = data.properties.description || ''
      form.properties.expose_bb = data.properties.expose_bb || false
      form.properties.default_value = data.properties.default_value ?? null
      const constraints = data.properties.constraints || {}
      // reference 类型：后端存 refs(ID数组)，前端需要 ref_fields(对象数组)
      if (data.type === 'reference' && constraints.refs) {
        const refIds = constraints.refs as number[]
        const refFieldItems = []
        for (const rid of refIds) {
          try {
            const refRes = await fieldApi.detail(rid)
            const rd = refRes.data
            refFieldItems.push({ id: rd.id, name: rd.name, label: rd.label, type: rd.type, type_label: '' })
          } catch {
            refFieldItems.push({ id: rid, name: `field_${rid}`, label: `字段${rid}`, type: 'unknown', type_label: '' })
          }
        }
        constraints.ref_fields = refFieldItems
        delete constraints.refs
      }
      form.properties.constraints = constraints
    }
    version.value = data.version
    hasRefs.value = data.has_refs || false
    createdAt.value = data.created_at || ''
    updatedAt.value = data.updated_at || ''
  } catch (err: unknown) {
    if ((err as BizError).code === FIELD_ERR.NOT_FOUND) {
      router.push('/fields')
    }
  }
}

// ---------- 标识符校验 ----------

async function checkNameUnique() {
  if (!form.name || !namePattern.test(form.name)) {
    nameStatus.value = ''
    return
  }
  nameStatus.value = 'checking'
  try {
    const res = await fieldApi.checkName(form.name)
    if (res.data?.available) {
      nameStatus.value = 'available'
      nameMessage.value = ''
    } else {
      nameStatus.value = 'taken'
      nameMessage.value = res.data?.message || '标识符已被使用'
    }
  } catch {
    nameStatus.value = ''
  }
}

// ---------- 类型切换 ----------

function handleTypeChange() {
  form.properties.constraints = {}
  form.properties.default_value = null
}

// ---------- 提交 ----------

/** 构建提交用的 properties，reference 类型需要把 ref_fields 转成后端的 refs 格式 */
function buildSubmitProperties() {
  const props = { ...form.properties, constraints: { ...form.properties.constraints } }
  if (form.type === 'reference' && props.constraints) {
    const refFields = (props.constraints as Record<string, unknown>).ref_fields as Array<{ id: number }> | undefined
    if (refFields) {
      ;(props.constraints as Record<string, unknown>).refs = refFields.map((f) => f.id)
      delete (props.constraints as Record<string, unknown>).ref_fields
    }
  }
  return props
}

async function handleSubmit() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  if (isCreate && nameStatus.value === 'taken') {
    ElMessage.warning('标识符已被使用，请更换')
    return
  }

  // 约束组件校验
  if (constraintRef.value?.validate) {
    const constraintErr = constraintRef.value.validate()
    if (constraintErr) {
      ElMessage.warning(constraintErr)
      return
    }
  }

  submitting.value = true
  try {
    const submitProps = buildSubmitProperties()
    if (isCreate) {
      await fieldApi.create({
        name: form.name,
        label: form.label,
        type: form.type,
        category: form.category,
        properties: submitProps,
      })
      ElMessage.success('创建成功，字段默认为禁用状态，确认无误后请手动启用')
    } else {
      await fieldApi.update({
        id: Number(route.params.id),
        label: form.label,
        type: form.type,
        category: form.category,
        properties: submitProps,
        version: version.value,
      })
      ElMessage.success('保存成功')
    }
    router.push('/fields')
  } catch (err: unknown) {
    const bizErr = err as BizError
    if (bizErr.code === FIELD_ERR.VERSION_CONFLICT) {
      ElMessageBox.alert('数据已被其他用户修改，请返回列表刷新后重试。', '版本冲突', { type: 'warning' })
      return
    }
    if (bizErr.code === FIELD_ERR.NAME_EXISTS || bizErr.code === FIELD_ERR.NAME_INVALID) {
      nameStatus.value = 'taken'
      nameMessage.value = bizErr.message
      return
    }
    if (bizErr.code === FIELD_ERR.REF_NESTED) {
      ElMessage.error('不能引用 reference 类型字段（禁止嵌套），请选择普通字段')
      return
    }
    if (bizErr.code === FIELD_ERR.REF_EMPTY) {
      ElMessage.error('reference 字段必须至少选择一个目标字段')
      return
    }
    if (bizErr.code === FIELD_ERR.CYCLIC_REF) {
      ElMessage.error('检测到循环引用，请检查引用字段链路')
      return
    }
    if (bizErr.code === FIELD_ERR.REF_CHANGE_TYPE) {
      ElMessage.error('该字段已被模板引用，无法更改类型')
      return
    }
    if (bizErr.code === FIELD_ERR.REF_TIGHTEN) {
      ElMessage.error(`该字段已被引用，约束只能放宽不能收紧���${bizErr.message}`)
      return
    }
    if (bizErr.code === FIELD_ERR.EDIT_NOT_DISABLED) {
      ElMessage.warning('请先禁用该字段后再编辑')
      return
    }
    if (bizErr.code === FIELD_ERR.REF_DISABLED) {
      ElMessage.error('引用的目标字段已被禁用，请先启用或更换')
      return
    }
    if (bizErr.code === FIELD_ERR.REF_NOT_FOUND) {
      ElMessage.error('引用的目标字段不存在，可能已被删除')
      return
    }
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.field-form {
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
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

.constraint-empty {
  padding: 16px;
  background: #F5F7FA;
  border-radius: 4px;
  color: #909399;
  font-size: 13px;
  width: 100%;
  text-align: center;
}

.default-hint {
  font-size: 13px;
  color: #909399;
}

</style>
