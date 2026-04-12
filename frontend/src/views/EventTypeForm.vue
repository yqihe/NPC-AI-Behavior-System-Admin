<template>
  <div class="event-type-form">
    <!-- 顶部导航 -->
    <div class="form-header">
      <el-icon class="back-icon" @click="$router.push('/event-types')"><ArrowLeft /></el-icon>
      <span class="back-text" @click="$router.push('/event-types')">返回</span>
      <span class="header-sep"></span>
      <span class="header-title">{{ isView ? '查看事件类型' : isCreate ? '新建事件类型' : '编辑事件类型' }}</span>
    </div>

    <!-- 表单滚动区 -->
    <div class="form-scroll">
      <!-- 基本信息卡片 -->
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
          <!-- 事件标识 -->
          <el-form-item label="事件标识" prop="name">
            <template v-if="!isCreate || isView">
              <el-input :model-value="form.name" disabled style="width: 100%">
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
                placeholder="如 gunshot、player_spotted（小写字母开头，仅含小写字母、数字、下划线）"
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

          <!-- 中文名称 -->
          <el-form-item label="中文名称" prop="display_name">
            <el-input
              v-model="form.display_name"
              placeholder="如 枪声、发现玩家（策划可见的显示名称）"
              style="width: 100%"
            />
          </el-form-item>

          <!-- 感知模式 -->
          <el-form-item label="感知模式" prop="perception_mode">
            <el-select
              v-model="form.perception_mode"
              placeholder="请选择感知模式"
              style="width: 100%"
              @change="handleModeChange"
            >
              <el-option label="Visual（视觉感知）" value="visual" />
              <el-option label="Auditory（听觉感知）" value="auditory" />
              <el-option label="Global（全局感知）" value="global" />
            </el-select>
          </el-form-item>

          <!-- 默认严重度 -->
          <el-form-item label="默认严重度" prop="default_severity">
            <el-input-number
              v-model="form.default_severity"
              :controls="false"
              :min="0"
              :max="100"
              placeholder="0 ~ 100"
              style="width: 200px"
            />
            <span class="field-extra">范围 0 ~ 100</span>
          </el-form-item>

          <!-- 默认 TTL -->
          <el-form-item label="默认 TTL" prop="default_ttl">
            <el-input-number
              v-model="form.default_ttl"
              :controls="false"
              :min="0.1"
              :step="0.1"
              placeholder="> 0"
              style="width: 200px"
            />
            <span class="field-extra">必须大于 0（秒）</span>
          </el-form-item>

          <!-- 感知范围 -->
          <el-form-item label="感知范围" prop="range">
            <el-input-number
              v-model="form.range"
              :controls="false"
              :min="0"
              :disabled="form.perception_mode === 'global'"
              placeholder=">= 0"
              style="width: 200px"
            />
            <div v-if="form.perception_mode === 'global'" class="field-warn">
              <el-icon><WarningFilled /></el-icon>
              当感知模式为 Global 时范围自动置为 0
            </div>
            <span v-else class="field-extra">必须 ≥ 0</span>
          </el-form-item>
        </el-form>
      </div>

      <!-- 扩展字段卡片 -->
      <div v-if="extensionSchema.length > 0" class="form-card">
        <div class="card-title">
          <span class="title-bar title-bar-orange"></span>
          <span class="title-text">扩展字段</span>
          <el-tag size="small" type="warning" style="margin-left: 8px">可选</el-tag>
        </div>

        <div class="ext-info">
          <el-icon><InfoFilled /></el-icon>
          <span>扩展字段由事件类型 Schema 定义，以下字段将附加到 config_json 中</span>
        </div>

        <el-form :disabled="isView" label-width="120px" label-position="right">
          <el-form-item
            v-for="ext in sortedExtensionSchema"
            :key="ext.field_name"
            :label="ext.field_label"
            :class="{ 'ext-disabled': !ext.enabled }"
          >
            <!-- 禁用标注 -->
            <div v-if="!ext.enabled" class="ext-disabled-tag">
              <el-tag size="small" type="info">已禁用</el-tag>
            </div>
            <!-- int -->
            <el-input-number
              v-if="ext.field_type === 'int'"
              v-model="extensionValues[ext.field_name]"
              :controls="false"
              :disabled="!ext.enabled"
              :placeholder="`默认: ${ext.default_value}`"
              style="width: 200px"
              @change="() => markDirty(ext.field_name)"
            />
            <!-- float -->
            <el-input-number
              v-else-if="ext.field_type === 'float'"
              v-model="extensionValues[ext.field_name]"
              :controls="false"
              :step="0.1"
              :disabled="!ext.enabled"
              :placeholder="`默认: ${ext.default_value}`"
              style="width: 200px"
              @change="() => markDirty(ext.field_name)"
            />
            <!-- string -->
            <el-input
              v-else-if="ext.field_type === 'string'"
              v-model="extensionValues[ext.field_name]"
              :disabled="!ext.enabled"
              :placeholder="`默认: ${ext.default_value}`"
              style="width: 100%"
              @input="() => markDirty(ext.field_name)"
            />
            <!-- bool -->
            <el-switch
              v-else-if="ext.field_type === 'bool'"
              :model-value="Boolean(extensionValues[ext.field_name])"
              :disabled="!ext.enabled"
              @change="(val: string | number | boolean) => { extensionValues[ext.field_name] = Boolean(val); markDirty(ext.field_name) }"
            />
            <!-- select -->
            <el-select
              v-else-if="ext.field_type === 'select'"
              v-model="extensionValues[ext.field_name]"
              placeholder="请选择"
              :disabled="!ext.enabled"
              style="width: 200px"
              @change="() => markDirty(ext.field_name)"
            >
              <el-option
                v-for="opt in getSelectOptions(ext)"
                :key="String(opt)"
                :label="String(opt)"
                :value="opt"
              />
            </el-select>
            <div class="ext-hint">
              类型: {{ extTypeLabel(ext.field_type) }} · 默认值: {{ JSON.stringify(ext.default_value) }}
            </div>
          </el-form-item>
        </el-form>
      </div>

      <!-- 表单操作（查看模式隐藏） -->
      <div v-if="!isView" class="form-card form-footer">
        <el-button @click="$router.push('/event-types')">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">
          保存
        </el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance } from 'element-plus'
import {
  ArrowLeft, Lock, WarningFilled, Loading,
  CircleCheck, CircleClose, InfoFilled,
} from '@element-plus/icons-vue'
import { eventTypeApi, EVENT_TYPE_ERR } from '@/api/eventTypes'
import type { ExtensionSchemaItem } from '@/api/eventTypes'
import type { BizError } from '@/api/request'

const route = useRoute()
const router = useRouter()
const isCreate = route.meta.isCreate as boolean
const isView = (route.meta.isView as boolean) || false

const formRef = ref<FormInstance>()
const submitting = ref(false)
const nameStatus = ref<'' | 'checking' | 'available' | 'taken'>('')
const nameMessage = ref('')
const version = ref(0)

// 扩展字段
const extensionSchema = ref<ExtensionSchemaItem[]>([])
const extensionValues = reactive<Record<string, unknown>>({})
const dirtyExtensions = reactive(new Set<string>())

interface FormState {
  name: string
  display_name: string
  perception_mode: string
  default_severity: number | undefined
  default_ttl: number | undefined
  range: number | undefined
}

const form = reactive<FormState>({
  name: '',
  display_name: '',
  perception_mode: '',
  default_severity: undefined,
  default_ttl: undefined,
  range: undefined,
})

const namePattern = /^[a-z][a-z0-9_]*$/

const rules = {
  name: [
    { required: true, message: '请输入事件标识', trigger: 'blur' },
    { pattern: namePattern, message: '小写字母开头，仅含小写字母、数字、下划线', trigger: 'blur' },
  ],
  display_name: [
    { required: true, message: '请输入中文名称', trigger: 'blur' },
  ],
  perception_mode: [
    { required: true, message: '请选择感知模式', trigger: 'change' },
  ],
  default_severity: [
    { required: true, message: '请输入默认严重度', trigger: 'blur' },
  ],
  default_ttl: [
    { required: true, message: '请输入默认 TTL', trigger: 'blur' },
  ],
  range: [
    { required: true, message: '请输入感知范围', trigger: 'blur' },
  ],
}

// ---------- 初始化 ----------

onMounted(async () => {
  if (isCreate) {
    await loadExtensionSchema()
  } else {
    await loadDetail()
  }
})

async function loadExtensionSchema() {
  try {
    const res = await eventTypeApi.schemaListEnabled()
    const items = res.data?.items || []
    extensionSchema.value = items.map((s) => ({
      field_name: s.field_name,
      field_label: s.field_label,
      field_type: s.field_type,
      constraints: s.constraints,
      default_value: s.default_value,
      sort_order: s.sort_order,
      enabled: true,
    }))
    // 用默认值初始化扩展字段值（不标记 dirty）
    for (const ext of extensionSchema.value) {
      extensionValues[ext.field_name] = ext.default_value
    }
  } catch {
    // 拦截器已 toast
  }
}

async function loadDetail() {
  const id = Number(route.params.id)
  try {
    const res = await eventTypeApi.detail(id)
    const data = res.data
    form.name = data.name
    form.display_name = data.display_name
    version.value = data.version

    // 从 config 中提取系统字段
    const cfg = data.config || {}
    form.perception_mode = (cfg.perception_mode as string) || ''
    form.default_severity = cfg.default_severity as number | undefined
    form.default_ttl = cfg.default_ttl as number | undefined
    form.range = cfg.range as number | undefined

    // 扩展字段
    extensionSchema.value = data.extension_schema || []
    loadExtensionsFromConfig(cfg, extensionSchema.value)
  } catch (err: unknown) {
    if ((err as BizError).code === EVENT_TYPE_ERR.NOT_FOUND) {
      ElMessage.error('事件类型不存在')
      router.push('/event-types')
    }
  }
}

function loadExtensionsFromConfig(
  config: Record<string, unknown>,
  schema: ExtensionSchemaItem[],
) {
  const systemKeys = new Set([
    'display_name', 'default_severity', 'default_ttl',
    'perception_mode', 'range',
  ])
  for (const ext of schema) {
    if (ext.field_name in config && !systemKeys.has(ext.field_name)) {
      extensionValues[ext.field_name] = config[ext.field_name]
      dirtyExtensions.add(ext.field_name)
    } else {
      extensionValues[ext.field_name] = ext.default_value
    }
  }
}

// ---------- 排序：按 sort_order 升序 ----------

const sortedExtensionSchema = computed(() =>
  [...extensionSchema.value].sort((a, b) => a.sort_order - b.sort_order),
)

// ---------- 标识符校验 ----------

async function checkNameUnique() {
  if (!form.name || !namePattern.test(form.name)) {
    nameStatus.value = ''
    return
  }
  nameStatus.value = 'checking'
  try {
    const res = await eventTypeApi.checkName(form.name)
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

// ---------- 感知模式联动 ----------

function handleModeChange(mode: string) {
  if (mode === 'global') {
    form.range = 0
  }
}

// ---------- 扩展字段 ----------

function markDirty(fieldName: string) {
  dirtyExtensions.add(fieldName)
}

function extTypeLabel(type: string): string {
  const map: Record<string, string> = {
    int: '整数', float: '浮点数', string: '文本', bool: '布尔', select: '选择',
  }
  return map[type] || type
}

function getSelectOptions(ext: ExtensionSchemaItem): unknown[] {
  const constraints = ext.constraints || {}
  const options = constraints.options as Array<{ value: unknown }> | undefined
  if (!options) return []
  return options.map((o) => o.value)
}

function buildExtensions(): Record<string, unknown> {
  const result: Record<string, unknown> = {}
  for (const key of dirtyExtensions) {
    result[key] = extensionValues[key]
  }
  return result
}

// ---------- 提交 ----------

async function handleSubmit() {
  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  if (isCreate && nameStatus.value === 'taken') {
    ElMessage.warning('标识符已被使用，请更换')
    return
  }

  submitting.value = true
  try {
    const extensions = buildExtensions()

    if (isCreate) {
      await eventTypeApi.create({
        name: form.name,
        display_name: form.display_name,
        perception_mode: form.perception_mode,
        default_severity: form.default_severity!,
        default_ttl: form.default_ttl!,
        range: form.range!,
        extensions: Object.keys(extensions).length > 0 ? extensions : undefined,
      })
      ElMessage.success('创建成功，事件类型默认为禁用状态，确认无误后请手动启用')
    } else {
      await eventTypeApi.update({
        id: Number(route.params.id),
        display_name: form.display_name,
        perception_mode: form.perception_mode,
        default_severity: form.default_severity!,
        default_ttl: form.default_ttl!,
        range: form.range!,
        extensions,
        version: version.value,
      })
      ElMessage.success('保存成功')
    }
    router.push('/event-types')
  } catch (err: unknown) {
    const bizErr = err as BizError
    if (bizErr.code === EVENT_TYPE_ERR.VERSION_CONFLICT) {
      ElMessageBox.alert('数据已被其他用户修改，请返回列表刷新后重试。', '版本冲突', { type: 'warning' })
      return
    }
    if (bizErr.code === EVENT_TYPE_ERR.NAME_EXISTS || bizErr.code === EVENT_TYPE_ERR.NAME_INVALID) {
      nameStatus.value = 'taken'
      nameMessage.value = bizErr.message
      return
    }
    if (bizErr.code === EVENT_TYPE_ERR.NOT_FOUND) {
      ElMessage.error('事件类型不存在')
      router.push('/event-types')
      return
    }
    // 其他错误拦截器已 toast
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.event-type-form {
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

.form-scroll {
  flex: 1;
  padding: 24px 32px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-card {
  background: #fff;
  border: 1px solid #E4E7ED;
  border-radius: 8px;
  padding: 32px;
}

.card-title {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 20px;
}

.title-bar {
  width: 3px;
  height: 14px;
  border-radius: 2px;
}

.title-bar-blue {
  background: #409EFF;
}

.title-bar-orange {
  background: #E6A23C;
}

.title-text {
  font-size: 15px;
  font-weight: 600;
  color: #303133;
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

.ext-info {
  display: flex;
  align-items: center;
  gap: 8px;
  background: #F5F7FA;
  border-radius: 4px;
  padding: 12px 16px;
  margin-bottom: 20px;
  font-size: 12px;
  color: #909399;
}

.ext-hint {
  margin-top: 4px;
  font-size: 12px;
  color: #909399;
}

.ext-disabled {
  opacity: 0.55;
}

.ext-disabled-tag {
  margin-bottom: 4px;
}

.form-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding: 20px 32px;
}
</style>
