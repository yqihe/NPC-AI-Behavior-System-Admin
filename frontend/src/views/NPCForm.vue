<template>
  <div class="npc-form">
    <!-- 顶部导航 -->
    <div class="form-header">
      <el-icon class="back-icon" @click="$router.push('/npcs')"><ArrowLeft /></el-icon>
      <span class="back-text" @click="$router.push('/npcs')">返回</span>
      <span class="header-sep"></span>
      <span class="header-title">
        {{ isView ? '查看 NPC' : isCreate ? '新建 NPC' : '编辑 NPC' }}
      </span>
    </div>

    <!-- 表单滚动区 -->
    <div class="form-scroll" v-loading="loading">
      <div class="form-body">

        <!-- 卡片 A：基本信息 -->
        <div class="form-card">
          <div class="card-title">
            <span class="title-bar title-bar-blue"></span>
            <span class="title-text">基本信息</span>
          </div>
          <el-form
            ref="formRef"
            :model="form"
            :rules="rules"
            :disabled="isView"
            label-width="120px"
            label-position="right"
          >
            <!-- NPC 标识 -->
            <el-form-item label="NPC 标识" prop="name">
              <template v-if="!isCreate">
                <el-input :model-value="form.name" disabled style="width: 100%">
                  <template #prefix><el-icon><Lock /></el-icon></template>
                </el-input>
                <div class="field-warn">
                  <el-icon><WarningFilled /></el-icon>
                  NPC 标识创建后不可修改
                </div>
              </template>
              <template v-else>
                <el-input
                  v-model="form.name"
                  placeholder="如 wolf_npc（小写字母开头，仅含小写字母、数字、下划线）"
                  style="width: 100%"
                  @blur="checkNameUnique"
                />
                <div v-if="nameStatus === 'checking'" class="field-hint">
                  <el-icon class="is-loading"><Loading /></el-icon>
                  校验中...
                </div>
                <div v-else-if="nameStatus === 'available'" class="field-hint field-hint-success">
                  <el-icon><CircleCheck /></el-icon>
                  标识可用
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
                placeholder="如 灰狼"
                maxlength="64"
                show-word-limit
                style="width: 100%"
              />
            </el-form-item>

            <!-- 描述 -->
            <el-form-item label="描述" prop="description">
              <el-input
                v-model="form.description"
                type="textarea"
                :rows="3"
                placeholder="用途说明（可选）"
                maxlength="512"
                show-word-limit
              />
            </el-form-item>
          </el-form>
        </div>

        <!-- 卡片 B：字段值配置 -->
        <div class="form-card">
          <div class="card-title">
            <span class="title-bar title-bar-orange"></span>
            <span class="title-text">字段值配置</span>
          </div>

          <!-- 模板选择（新建） / 模板展示（编辑） -->
          <el-form label-width="120px" label-position="right">
            <el-form-item label="所用模板">
              <!-- 新建模式：下拉选择 -->
              <template v-if="isCreate">
                <template v-if="templateList.length === 0">
                  <el-alert type="warning" :closable="false" show-icon>
                    暂无可用模板，请先在
                    <el-link type="primary" :underline="false" style="margin: 0 4px" @click="$router.push('/templates')">
                      模板管理
                    </el-link>
                    中创建并启用模板。
                  </el-alert>
                </template>
                <el-select
                  v-else
                  v-model="templateId"
                  placeholder="选择模板"
                  style="width: 100%"
                  :disabled="isView"
                  @change="onTemplateChange"
                >
                  <el-option
                    v-for="tpl in templateList"
                    :key="tpl.id"
                    :label="`${tpl.label} (${tpl.name})`"
                    :value="tpl.id"
                  />
                </el-select>
                <div class="field-hint" style="color: #909399">选择模板后将渲染该模板的字段列表</div>
              </template>

              <!-- 编辑模式：只读展示 -->
              <template v-else>
                <el-input
                  :model-value="templateDisplay"
                  disabled
                  style="width: 100%; background: #F5F7FA"
                />
                <div class="field-hint" style="color: #E6A23C">
                  <el-icon><WarningFilled /></el-icon>
                  模板选择后不可更改
                </div>
              </template>
            </el-form-item>
          </el-form>

          <!-- 动态字段区 -->
          <template v-if="templateDetail">
            <el-divider />
            <el-form label-width="120px" label-position="right" :disabled="isView">
              <el-form-item
                v-for="field in templateDetail.fields"
                :key="field.field_id"
              >
                <template #label>
                  <span v-if="field.required" class="required-star">*</span>
                  <span>{{ field.label }}</span>
                </template>

                <!-- int / integer -->
                <el-input-number
                  v-if="field.type === 'int' || field.type === 'integer'"
                  :model-value="(fieldValues.get(field.field_id) as number | null) ?? null"
                  :precision="0"
                  :min="getConstraint(field.field_id, 'min') as number | undefined"
                  :max="getConstraint(field.field_id, 'max') as number | undefined"
                  :disabled="isView || !field.enabled"
                  controls-position="right"
                  style="width: 100%"
                  @change="(val: number | null) => setFieldValue(field.field_id, val)"
                />

                <!-- float -->
                <el-input-number
                  v-else-if="field.type === 'float'"
                  :model-value="(fieldValues.get(field.field_id) as number | null) ?? null"
                  :precision="(getConstraint(field.field_id, 'precision') as number | undefined) ?? 2"
                  :min="getConstraint(field.field_id, 'min') as number | undefined"
                  :max="getConstraint(field.field_id, 'max') as number | undefined"
                  :disabled="isView || !field.enabled"
                  controls-position="right"
                  style="width: 100%"
                  @change="(val: number | null) => setFieldValue(field.field_id, val)"
                />

                <!-- bool / boolean -->
                <el-switch
                  v-else-if="field.type === 'bool' || field.type === 'boolean'"
                  :model-value="(fieldValues.get(field.field_id) as boolean | null) ?? false"
                  :disabled="isView || !field.enabled"
                  @change="(val: boolean) => setFieldValue(field.field_id, val)"
                />

                <!-- select (single) -->
                <el-select
                  v-else-if="field.type === 'select' && !isMultiSelect(field.field_id)"
                  :model-value="(fieldValues.get(field.field_id) as string | null) ?? ''"
                  clearable
                  :disabled="isView || !field.enabled"
                  style="width: 100%"
                  @change="(val: string) => setFieldValue(field.field_id, val || null)"
                >
                  <el-option
                    v-for="opt in getSelectOptions(field.field_id)"
                    :key="opt.value"
                    :label="opt.label || opt.value"
                    :value="opt.value"
                  />
                </el-select>

                <!-- select (multiple) -->
                <el-select
                  v-else-if="field.type === 'select' && isMultiSelect(field.field_id)"
                  :model-value="(fieldValues.get(field.field_id) as string[]) ?? []"
                  multiple
                  clearable
                  :disabled="isView || !field.enabled"
                  style="width: 100%"
                  @change="(val: string[]) => setFieldValue(field.field_id, val.length ? val : null)"
                >
                  <el-option
                    v-for="opt in getSelectOptions(field.field_id)"
                    :key="opt.value"
                    :label="opt.label || opt.value"
                    :value="opt.value"
                  />
                </el-select>

                <!-- string (long) -->
                <el-input
                  v-else-if="field.type === 'string' && isLongString(field.field_id)"
                  :model-value="(fieldValues.get(field.field_id) as string | null) ?? ''"
                  type="textarea"
                  :rows="3"
                  :maxlength="getConstraint(field.field_id, 'maxLength') as number | undefined"
                  :disabled="isView || !field.enabled"
                  show-word-limit
                  @input="(val: string) => setFieldValue(field.field_id, val || null)"
                />

                <!-- string (short / default) -->
                <el-input
                  v-else
                  :model-value="(fieldValues.get(field.field_id) as string | null) ?? ''"
                  :maxlength="getConstraint(field.field_id, 'maxLength') as number | undefined"
                  :disabled="isView || !field.enabled"
                  show-word-limit
                  @input="(val: string) => setFieldValue(field.field_id, val || null)"
                />

                <!-- 停用字段 警告 -->
                <div v-if="!field.enabled" class="field-warn">
                  <el-icon><WarningFilled /></el-icon>
                  此字段已停用，值保留但不可修改
                </div>

                <!-- 字段名 hint -->
                <div class="field-hint field-name-hint">
                  <span class="mono">{{ field.name }}</span>
                  <el-tag size="small" type="info" style="margin-left: 6px">{{ field.type }}</el-tag>
                  <el-tag v-if="field.required" size="small" type="danger" style="margin-left: 4px">必填</el-tag>
                </div>
              </el-form-item>
            </el-form>
          </template>
          <template v-else-if="isCreate">
            <div class="empty-hint">请先选择模板</div>
          </template>
        </div>

        <!-- 卡片 C：行为配置 -->
        <div class="form-card">
          <div class="card-title">
            <span class="title-bar title-bar-green"></span>
            <span class="title-text">行为配置</span>
          </div>
          <BehaviorConfigPanel
            v-model="behaviorConfig"
            :disabled="isView"
            :fsm-list="fsmList"
            :bt-list="btList"
            :fsm-states="fsmStates"
            @update:model-value="onBehaviorChange"
          />
        </div>

      </div><!-- /form-body -->
    </div>

    <!-- 底部操作栏 -->
    <div v-if="!isView" class="form-footer">
      <el-button @click="$router.push('/npcs')">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="onSubmit">保存</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, watch } from 'vue'
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
import BehaviorConfigPanel from '@/components/BehaviorConfigPanel.vue'
import type { BehaviorConfig } from '@/components/BehaviorConfigPanel.vue'
import { npcApi, NPC_ERRORS } from '@/api/npc'
import { templateApi } from '@/api/templates'
import type { TemplateListItem, TemplateDetail } from '@/api/templates'
import { fieldApi } from '@/api/fields'
import type { FieldDetail } from '@/api/fields'
import { fsmConfigApi } from '@/api/fsmConfigs'
import type { FsmConfigListItem } from '@/api/fsmConfigs'
import { btTreeApi } from '@/api/btTrees'
import type { BtTreeListItem } from '@/api/btTrees'
import type { BizError } from '@/api/request'

const props = defineProps<{
  mode: 'create' | 'edit' | 'view'
  id?: number
}>()

const router = useRouter()
const isCreate = computed(() => props.mode === 'create')
const isView = computed(() => props.mode === 'view')

// ---------- 状态 ----------

const formRef = ref<FormInstance | null>(null)
const loading = ref(false)
const submitting = ref(false)
const version = ref(0)

const form = reactive({
  name: '',
  label: '',
  description: '',
})

// 模板相关
const templateList = ref<TemplateListItem[]>([])
const templateId = ref<number | null>(null)
const templateDetail = ref<TemplateDetail | null>(null)
/** field_id → FieldDetail（含约束）*/
const fieldDetailMap = ref<Map<number, FieldDetail>>(new Map())
/** field_id → value（JS 原生类型）*/
const fieldValues = ref<Map<number, unknown>>(new Map())

// 行为配置
const behaviorConfig = ref<BehaviorConfig>({ fsm_ref: '', bt_refs: {} })
const fsmList = ref<FsmConfigListItem[]>([])
const btList = ref<BtTreeListItem[]>([])
const fsmStates = ref<string[]>([])

// 标识唯一性
type NameStatus = 'idle' | 'checking' | 'available' | 'taken'
const nameStatus = ref<NameStatus>('idle')
const nameMessage = ref('')
const namePattern = /^[a-z][a-z0-9_]*$/

// ---------- 计算 ----------

const templateDisplay = computed(() => {
  if (!templateDetail.value) return ''
  return `${templateDetail.value.label} (${templateDetail.value.name})`
})

const rules: FormRules = {
  name: [
    { required: true, message: '请输入 NPC 标识', trigger: 'blur' },
    { pattern: namePattern, message: '小写字母开头，仅含小写字母、数字、下划线', trigger: 'blur' },
  ],
  label: [{ required: true, message: '请输入中文标签', trigger: 'blur' }],
}

// ---------- 约束辅助 ----------

function getConstraints(fieldId: number): Record<string, unknown> {
  const detail = fieldDetailMap.value.get(fieldId)
  return (detail?.properties?.constraints as Record<string, unknown> | undefined) ?? {}
}

function getConstraint(fieldId: number, key: string): unknown {
  return getConstraints(fieldId)[key]
}

interface SelectOption { value: string; label?: string }

function getSelectOptions(fieldId: number): SelectOption[] {
  const opts = getConstraints(fieldId)['options']
  if (!Array.isArray(opts)) return []
  return opts as SelectOption[]
}

function isMultiSelect(fieldId: number): boolean {
  const maxSel = getConstraint(fieldId, 'maxSelect')
  return typeof maxSel === 'number' && maxSel > 1
}

function isLongString(fieldId: number): boolean {
  const maxLen = getConstraint(fieldId, 'maxLength')
  return typeof maxLen === 'number' && maxLen > 64
}

// ---------- 字段值辅助 ----------

function setFieldValue(fieldId: number, val: unknown) {
  const newMap = new Map(fieldValues.value)
  newMap.set(fieldId, val)
  fieldValues.value = newMap
}

// ---------- 模板加载 ----------

async function loadTemplateDetail(id: number) {
  const res = await templateApi.detail(id)
  const detail = res.data
  templateDetail.value = detail

  // 批量拉字段详情（含约束）
  const fieldIds = detail.fields.map((f) => f.field_id)
  const detailMap = new Map<number, FieldDetail>()
  await Promise.all(
    fieldIds.map(async (fid) => {
      try {
        const r = await fieldApi.detail(fid)
        detailMap.set(fid, r.data)
      } catch {
        // 字段已删除，跳过
      }
    }),
  )
  fieldDetailMap.value = detailMap
}

async function onTemplateChange(id: number | null) {
  if (!id) {
    templateDetail.value = null
    fieldDetailMap.value = new Map()
    fieldValues.value = new Map()
    return
  }
  templateId.value = id
  fieldValues.value = new Map()
  await loadTemplateDetail(id)
}

// ---------- 行为配置 ----------

async function onBehaviorChange(val: BehaviorConfig) {
  behaviorConfig.value = val
  await loadFsmStates(val.fsm_ref)
}

async function loadFsmStates(fsmRef: string) {
  if (!fsmRef) {
    fsmStates.value = []
    return
  }
  const fsm = fsmList.value.find((f) => f.name === fsmRef)
  if (!fsm) {
    fsmStates.value = []
    return
  }
  try {
    const res = await fsmConfigApi.detail(fsm.id)
    fsmStates.value = res.data.config.states.map((s) => s.name)
  } catch {
    fsmStates.value = []
  }
}

// ---------- 初始化 ----------

onMounted(async () => {
  loading.value = true
  try {
    // 并行加载 FSM/BT 下拉列表
    const [fsmRes, btRes, tplRes] = await Promise.all([
      fsmConfigApi.list({ enabled: true, page: 1, page_size: 1000 }),
      btTreeApi.list({ enabled: true, page: 1, page_size: 1000 }),
      isCreate.value
        ? templateApi.list({ enabled: true, page: 1, page_size: 1000 })
        : Promise.resolve(null),
    ])
    fsmList.value = fsmRes.data?.items || []
    btList.value = btRes.data?.items || []
    if (tplRes) {
      templateList.value = tplRes.data?.items || []
    }

    // 编辑/查看模式：加载 NPC 详情
    if (!isCreate.value && props.id) {
      const npcRes = await npcApi.detail(props.id)
      const npc = npcRes.data
      version.value = npc.version
      form.name = npc.name
      form.label = npc.label
      form.description = npc.description

      // 加载模板详情
      await loadTemplateDetail(npc.template_id)
      templateId.value = npc.template_id

      // 回填字段值
      const vmap = new Map<number, unknown>()
      for (const f of npc.fields) {
        vmap.set(f.field_id, f.value)
      }
      fieldValues.value = vmap

      // 回填行为配置
      behaviorConfig.value = {
        fsm_ref: npc.fsm_ref || '',
        bt_refs: npc.bt_refs || {},
      }

      // 加载 FSM states
      if (npc.fsm_ref) {
        await loadFsmStates(npc.fsm_ref)
      }
    }
  } catch (err) {
    const bizErr = err as BizError
    if (bizErr.code === NPC_ERRORS.NOT_FOUND) {
      ElMessage.error('NPC 不存在')
      router.push('/npcs')
    }
  } finally {
    loading.value = false
  }
})

// ---------- 唯一性校验 ----------

async function checkNameUnique() {
  if (!isCreate.value) return
  const name = form.name.trim()
  if (!name) { nameStatus.value = 'idle'; return }
  if (!namePattern.test(name)) {
    nameStatus.value = 'taken'
    nameMessage.value = '格式不合法（小写字母开头，a-z / 0-9 / 下划线）'
    return
  }
  nameStatus.value = 'checking'
  try {
    const res = await npcApi.checkName(name)
    nameStatus.value = res.data.available ? 'available' : 'taken'
    nameMessage.value = res.data.message
  } catch {
    nameStatus.value = 'idle'
  }
}

// ---------- 提交 ----------

async function onSubmit() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  if (isCreate.value) {
    if (nameStatus.value === 'taken') {
      ElMessage.warning('NPC 标识不可用，请更换')
      return
    }
    if (!templateId.value) {
      ElMessage.error('请选择模板')
      return
    }
  }

  // 必填字段前端校验
  if (templateDetail.value) {
    for (const field of templateDetail.value.fields) {
      if (field.required) {
        const val = fieldValues.value.get(field.field_id)
        if (val === null || val === undefined || val === '') {
          ElMessage.error(`必填字段「${field.label}」不能为空`)
          return
        }
      }
    }
  }

  // 组装 field_values（包含模板所有字段，未填的为 null）
  const fieldValuesArr = templateDetail.value
    ? templateDetail.value.fields.map((f) => ({
        field_id: f.field_id,
        value: (fieldValues.value.get(f.field_id) ?? null) as number | string | boolean | null,
      }))
    : []

  submitting.value = true
  try {
    if (isCreate.value) {
      await npcApi.create({
        name: form.name,
        label: form.label,
        description: form.description || undefined,
        template_id: templateId.value!,
        field_values: fieldValuesArr,
        fsm_ref: behaviorConfig.value.fsm_ref || undefined,
        bt_refs: Object.keys(behaviorConfig.value.bt_refs).length
          ? behaviorConfig.value.bt_refs
          : undefined,
      })
      ElMessage.success('创建成功，NPC 已默认启用')
      router.push('/npcs')
    } else {
      await npcApi.update({
        id: props.id!,
        label: form.label,
        description: form.description || undefined,
        field_values: fieldValuesArr,
        fsm_ref: behaviorConfig.value.fsm_ref || undefined,
        bt_refs: Object.keys(behaviorConfig.value.bt_refs).length
          ? behaviorConfig.value.bt_refs
          : undefined,
        version: version.value,
      })
      ElMessage.success('保存成功')
      router.push('/npcs')
    }
  } catch (err) {
    const bizErr = err as BizError
    switch (bizErr.code) {
      case NPC_ERRORS.NAME_EXISTS:
        nameStatus.value = 'taken'
        nameMessage.value = 'NPC 标识已存在'
        break
      case NPC_ERRORS.NAME_INVALID:
        nameStatus.value = 'taken'
        nameMessage.value = bizErr.message
        break
      case NPC_ERRORS.TEMPLATE_NOT_FOUND:
        ElMessage.error('所选模板不存在，请刷新后重试')
        break
      case NPC_ERRORS.TEMPLATE_DISABLED:
        ElMessage.error('模板未启用，请刷新后重试')
        break
      case NPC_ERRORS.FIELD_REQUIRED:
        ElMessage.error('必填字段未填，请检查字段值配置')
        break
      case NPC_ERRORS.FIELD_VALUE_INVALID:
        ElMessage.error('字段值不符合约束，请检查输入')
        break
      case NPC_ERRORS.FSM_NOT_FOUND:
      case NPC_ERRORS.FSM_DISABLED:
        ElMessage.error('所选状态机不可用，请刷新后重试')
        break
      case NPC_ERRORS.BT_NOT_FOUND:
      case NPC_ERRORS.BT_DISABLED:
        ElMessage.error('所选行为树不可用，请刷新后重试')
        break
      case NPC_ERRORS.BT_STATE_INVALID:
        ElMessage.error('行为树绑定的状态名与状态机不匹配，请重新配置')
        break
      case NPC_ERRORS.BT_WITHOUT_FSM:
        ElMessage.error('选择行为树前请先选择状态机')
        break
      case NPC_ERRORS.VERSION_CONFLICT:
        ElMessageBox.alert(
          '该 NPC 已被其他人修改，请返回列表刷新后重试。',
          '版本冲突',
          { type: 'warning' },
        ).then(() => router.push('/npcs'))
        break
      default:
        // 其他错误拦截器已 toast
        break
    }
  } finally {
    submitting.value = false
  }
}

// watch fsm_ref：在行为配置 panel 内部触发，通过 onBehaviorChange 已处理；
// 此处补一个 watch 处理编辑回填后的初始 fsm_ref
watch(
  () => behaviorConfig.value.fsm_ref,
  (newRef, oldRef) => {
    if (newRef !== oldRef && newRef) {
      loadFsmStates(newRef)
    }
  },
)
</script>

<style scoped>
@import '@/styles/form-layout.css';

.npc-form {
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.required-star {
  color: #F56C6C;
  margin-right: 4px;
}

.field-hint {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 4px;
  font-size: 12px;
  color: #909399;
}

.field-hint-success { color: #67C23A; }
.field-hint-error   { color: #F56C6C; }

.field-warn {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-top: 4px;
  font-size: 12px;
  color: #E6A23C;
}

.field-name-hint {
  margin-top: 4px;
  font-size: 12px;
  color: #909399;
  display: flex;
  align-items: center;
}

.empty-hint {
  text-align: center;
  color: #909399;
  font-size: 14px;
  padding: 24px 0;
}

.mono {
  font-family: 'Courier New', Courier, monospace;
}
</style>
