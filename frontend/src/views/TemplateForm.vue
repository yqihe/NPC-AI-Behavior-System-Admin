<template>
  <div class="template-form">
    <!-- 顶部导航 -->
    <div class="form-header">
      <el-icon class="back-icon" @click="$router.push('/templates')">
        <ArrowLeft />
      </el-icon>
      <span class="back-text" @click="$router.push('/templates')">返回</span>
      <span class="header-sep"></span>
      <span class="header-title">
        {{ mode === 'view' ? '查看模板' : mode === 'create' ? '新建模板' : '编辑模板' }}
      </span>
      <span v-if="isEdit && template" class="header-sub">
        {{ template.label }}
      </span>
      <el-tag
        v-if="isLocked && template"
        type="warning"
        effect="dark"
        style="margin-left: 12px"
      >
        被 {{ template.ref_count }} 个 NPC 引用
      </el-tag>
    </div>

    <!-- 主体 -->
    <div class="form-body" v-loading="loading">
      <el-alert
        v-if="isLocked"
        class="locked-alert"
        type="warning"
        :closable="false"
        show-icon
      >
        该模板已被 {{ template!.ref_count }} 个 NPC 引用，字段勾选与必填配置不可修改（仅中文标签与描述可改）
      </el-alert>

      <!-- 卡片一：基本信息 -->
      <div class="form-card">
        <div class="card-title">基本信息</div>
        <el-form
          ref="formRef"
          :model="formState"
          :rules="rules"
          :disabled="isView"
          label-width="120px"
          label-position="right"
        >
          <el-form-item label="模板标识" prop="name">
            <template v-if="isEdit">
              <el-input :model-value="formState.name" disabled>
                <template #prefix>
                  <el-icon><Lock /></el-icon>
                </template>
              </el-input>
              <div class="field-warn">
                <el-icon><WarningFilled /></el-icon>
                模板标识创建后不可修改
              </div>
            </template>
            <template v-else>
              <el-input
                v-model="formState.name"
                placeholder="如 combat_npc（小写字母开头，仅含小写字母、数字、下划线）"
                @blur="onNameBlur"
              />
              <div v-if="nameStatus === 'checking'" class="field-hint">
                <el-icon class="is-loading"><Loading /></el-icon>
                校验中...
              </div>
              <div
                v-else-if="nameStatus === 'available'"
                class="field-hint field-hint-success"
              >
                <el-icon><CircleCheck /></el-icon>
                {{ nameMessage || '标识可用' }}
              </div>
              <div
                v-else-if="nameStatus === 'taken'"
                class="field-hint field-hint-error"
              >
                <el-icon><CircleClose /></el-icon>
                {{ nameMessage || '标识已被使用' }}
              </div>
            </template>
          </el-form-item>

          <el-form-item label="中文标签" prop="label">
            <el-input
              v-model="formState.label"
              placeholder="如 战斗 NPC"
              maxlength="64"
              show-word-limit
            />
          </el-form-item>

          <el-form-item label="描述" prop="description">
            <el-input
              v-model="formState.description"
              type="textarea"
              :rows="3"
              placeholder="用途说明（可选）"
              maxlength="512"
              show-word-limit
            />
          </el-form-item>
        </el-form>
      </div>

      <!-- 卡片二：字段选择 -->
      <div class="form-card">
        <div class="card-title">
          字段选择
          <el-tag v-if="isLocked" type="warning" size="small" style="margin-left: 8px">
            🔒 已锁定
          </el-tag>
          <span class="card-hint">
            点击字段 cell 切换勾选；紫色边框为 reference 字段，点击弹出子字段选择
          </span>
        </div>
        <TemplateFieldPicker
          v-model:selectedIds="selectedIds"
          :field-pool="fieldPool"
          :disabled="isLocked"
        />
      </div>

      <!-- 卡片三：已选字段配置 -->
      <div class="form-card">
        <div class="card-title">
          已选字段配置
          <el-tag v-if="isLocked" type="warning" size="small" style="margin-left: 8px">
            🔒 已锁定
          </el-tag>
          <span class="card-hint">共 {{ selectedFieldsView.length }} 个字段</span>
        </div>
        <TemplateSelectedFields
          :selected-fields="selectedFieldsView"
          :disabled="isLocked"
          @update:order="onOrderChange"
          @update:required="onRequiredChange"
        />
      </div>

      <!-- 操作栏（查看模式隐藏） -->
      <div v-if="!isView" class="form-actions">
        <el-button @click="$router.push('/templates')">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="onSubmit">
          保存
        </el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox, type FormInstance, type FormRules } from 'element-plus'
import {
  ArrowLeft,
  Lock,
  WarningFilled,
  Loading,
  CircleCheck,
  CircleClose,
} from '@element-plus/icons-vue'
import TemplateFieldPicker from '@/components/TemplateFieldPicker.vue'
import TemplateSelectedFields from '@/components/TemplateSelectedFields.vue'
import { fieldApi } from '@/api/fields'
import type { FieldListItem } from '@/api/fields'
import { templateApi, TEMPLATE_ERR } from '@/api/templates'
import type { TemplateDetail, TemplateFieldItem } from '@/api/templates'
import type { BizError } from '@/api/request'

const props = defineProps<{
  mode: 'create' | 'edit' | 'view'
  id?: number
}>()

const router = useRouter()
const isEdit = computed(() => props.mode === 'edit' || props.mode === 'view')
const isView = computed(() => props.mode === 'view')

// ---------- 状态 ----------

const formRef = ref<FormInstance | null>(null)
const loading = ref(false)
const submitting = ref(false)

const formState = reactive({
  name: '',
  label: '',
  description: '',
})

const template = ref<TemplateDetail | null>(null)
const fieldPool = ref<FieldListItem[]>([])
const selectedIds = ref<number[]>([])
const requiredMap = ref<Record<number, boolean>>({})

type NameStatus = 'idle' | 'checking' | 'available' | 'taken'
const nameStatus = ref<NameStatus>('idle')
const nameMessage = ref('')

const namePattern = /^[a-z][a-z0-9_]*$/

const rules: FormRules = {
  name: [
    { required: true, message: '请输入模板标识', trigger: 'blur' },
    {
      pattern: namePattern,
      message: '小写字母开头，仅含小写字母、数字、下划线',
      trigger: 'blur',
    },
  ],
  label: [{ required: true, message: '请输入中文标签', trigger: 'blur' }],
}

// ---------- 派生状态 ----------

const isLocked = computed(
  () => isEdit.value && (template.value?.ref_count ?? 0) > 0,
)

/** 编辑模式优先从 template.fields 拿禁用字段元数据；create 模式全从 fieldPool 构造 */
const selectedFieldsView = computed<TemplateFieldItem[]>(() => {
  const detailMap = new Map<number, TemplateFieldItem>()
  if (isEdit.value && template.value) {
    for (const f of template.value.fields) {
      detailMap.set(f.field_id, f)
    }
  }
  const result: TemplateFieldItem[] = []
  for (const id of selectedIds.value) {
    const fromDetail = detailMap.get(id)
    if (fromDetail) {
      result.push({
        ...fromDetail,
        required: requiredMap.value[id] ?? fromDetail.required,
      })
      continue
    }
    const fromPool = fieldPool.value.find((f) => f.id === id)
    if (fromPool) {
      result.push({
        field_id: fromPool.id,
        name: fromPool.name,
        label: fromPool.label,
        type: fromPool.type,
        category: fromPool.category,
        category_label: fromPool.category_label,
        enabled: fromPool.enabled,
        required: requiredMap.value[id] ?? false,
      })
    }
  }
  return result
})

// ---------- 初始化 ----------

onMounted(async () => {
  loading.value = true
  try {
    if (isEdit.value && props.id) {
      const [detailRes, fieldsRes] = await Promise.all([
        templateApi.detail(props.id),
        fieldApi.list({ enabled: true, page: 1, page_size: 1000 }),
      ])
      const detail = detailRes.data
      template.value = detail
      formState.name = detail.name
      formState.label = detail.label
      formState.description = detail.description
      selectedIds.value = detail.fields.map((f) => f.field_id)
      const rmap: Record<number, boolean> = {}
      for (const f of detail.fields) {
        rmap[f.field_id] = f.required
      }
      requiredMap.value = rmap
      // 后端返回 id DESC，字段选择卡展示按创建顺序正序（旧在前），排一次 id ASC
      fieldPool.value = [...(fieldsRes.data?.items || [])].sort((a, b) => a.id - b.id)
    } else {
      const fieldsRes = await fieldApi.list({
        enabled: true,
        page: 1,
        page_size: 1000,
      })
      // 后端返回 id DESC，字段选择卡展示按创建顺序正序（旧在前），排一次 id ASC
      fieldPool.value = [...(fieldsRes.data?.items || [])].sort((a, b) => a.id - b.id)
    }
  } catch (err) {
    const bizErr = err as BizError
    if (bizErr.code === TEMPLATE_ERR.NOT_FOUND) {
      ElMessage.error('模板不存在')
      router.push('/templates')
    }
  } finally {
    loading.value = false
  }
})

// ---------- 事件 ----------

async function onNameBlur() {
  if (isEdit.value) return
  const name = formState.name.trim()
  if (!name) {
    nameStatus.value = 'idle'
    return
  }
  if (!namePattern.test(name)) {
    nameStatus.value = 'taken'
    nameMessage.value = '格式不合法（小写字母开头，a-z / 0-9 / 下划线）'
    return
  }
  nameStatus.value = 'checking'
  try {
    const res = await templateApi.checkName(name)
    const result = res.data
    nameStatus.value = result.available ? 'available' : 'taken'
    nameMessage.value = result.message
  } catch {
    nameStatus.value = 'idle'
  }
}

function onOrderChange(newOrder: number[]) {
  selectedIds.value = newOrder
}

function onRequiredChange(fieldId: number, required: boolean) {
  requiredMap.value = { ...requiredMap.value, [fieldId]: required }
}

async function onSubmit() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  if (props.mode === 'create' && nameStatus.value === 'taken') {
    ElMessage.warning('模板标识不可用，请更换')
    return
  }

  if (selectedIds.value.length === 0) {
    ElMessage.error('请至少勾选一个字段')
    return
  }

  const payload = {
    name: formState.name,
    label: formState.label,
    description: formState.description,
    fields: selectedIds.value.map((id) => ({
      field_id: id,
      required: requiredMap.value[id] ?? false,
    })),
  }

  submitting.value = true
  try {
    if (props.mode === 'create') {
      await templateApi.create(payload)
      ElMessage.success('创建成功，模板默认为禁用状态，确认无误后请手动启用')
    } else {
      await templateApi.update({
        ...payload,
        id: props.id!,
        version: template.value!.version,
      })
      ElMessage.success('保存成功')
    }
    router.push('/templates')
  } catch (err) {
    const bizErr = err as BizError
    switch (bizErr.code) {
      case TEMPLATE_ERR.NAME_EXISTS:
      case TEMPLATE_ERR.NAME_INVALID:
        nameStatus.value = 'taken'
        nameMessage.value = bizErr.message
        return
      case TEMPLATE_ERR.NOT_FOUND:
        router.push('/templates')
        return
      case TEMPLATE_ERR.FIELD_DISABLED:
      case TEMPLATE_ERR.FIELD_NOT_FOUND:
        // 字段池可能已过期，重新拉
        await reloadFieldPool()
        return
      case TEMPLATE_ERR.FIELD_IS_REFERENCE:
        ElMessage.error('reference 字段必须先展开子字段再加入模板')
        await reloadFieldPool()
        return
      case TEMPLATE_ERR.VERSION_CONFLICT:
        ElMessageBox.alert(
          '该模板已被其他人修改，请刷新后重试。',
          '版本冲突',
          { type: 'warning' },
        ).then(() => router.push('/templates'))
        return
      default:
        // 其他错误走拦截器默认 toast
        return
    }
  } finally {
    submitting.value = false
  }
}

async function reloadFieldPool() {
  try {
    const res = await fieldApi.list({
      enabled: true,
      page: 1,
      page_size: 1000,
    })
    fieldPool.value = res.data?.items || []
  } catch {
    // 拦截器已 toast
  }
}
</script>

<style scoped>
.template-form {
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

.back-icon,
.back-text {
  color: #409EFF;
  cursor: pointer;
}

.back-icon {
  font-size: 18px;
}

.back-text {
  font-size: 14px;
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

.header-sub {
  font-size: 13px;
  color: #909399;
  margin-left: 8px;
}

.form-body {
  flex: 1;
  padding: 24px 32px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 16px;
  max-width: 1200px;
  margin: 0 auto;
  width: 100%;
}

.locked-alert {
  margin-bottom: 0;
}

.form-card {
  background: #fff;
  border: 1px solid #E4E7ED;
  border-radius: 8px;
  padding: 20px 24px;
}

.card-title {
  display: flex;
  align-items: center;
  font-size: 15px;
  font-weight: 600;
  color: #303133;
  padding-bottom: 16px;
  border-bottom: 1px solid #EBEEF5;
  margin-bottom: 20px;
}

.card-hint {
  margin-left: auto;
  font-size: 12px;
  color: #909399;
  font-weight: 400;
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

.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding: 16px 0 32px;
}
</style>
