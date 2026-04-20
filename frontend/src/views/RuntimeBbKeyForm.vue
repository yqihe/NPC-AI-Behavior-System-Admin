<template>
  <div class="field-form">
    <!-- 顶部导航 -->
    <div class="form-header">
      <el-icon class="back-icon" @click="$router.push('/runtime-bb-keys')"><ArrowLeft /></el-icon>
      <span class="back-text" @click="$router.push('/runtime-bb-keys')">返回</span>
      <span class="header-sep"></span>
      <span class="header-title">{{ isView ? '查看运行时 Key' : isCreate ? '新建运行时 Key' : '编辑运行时 Key' }}</span>
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
            <el-form-item label="英文标识" prop="name">
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
                  placeholder="如 threat_level、npc_pos_x（小写字母开头，仅含小写字母、数字、下划线，长度 2-64）"
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
                placeholder="如 威胁等级、NPC 位置 X（策划可见的显示名称）"
                style="width: 100%"
              />
            </el-form-item>

            <!-- 类型 -->
            <el-form-item label="类型" prop="type">
              <el-select
                v-model="form.type"
                placeholder="请选择类型"
                style="width: 100%"
                :disabled="isView || (!isCreate && hasRefs)"
              >
                <el-option
                  v-for="item in RUNTIME_BB_KEY_TYPES"
                  :key="item.value"
                  :label="item.label"
                  :value="item.value"
                />
              </el-select>
              <div v-if="!isCreate && hasRefs" class="field-warn">
                <el-icon><WarningFilled /></el-icon>
                该 Key 被引用中，无法更改类型
              </div>
            </el-form-item>

            <!-- 分组 -->
            <el-form-item label="分组" prop="group_name">
              <el-select
                v-model="form.group_name"
                placeholder="请选择分组"
                style="width: 100%"
              >
                <el-option
                  v-for="item in RUNTIME_BB_KEY_GROUPS"
                  :key="item.value"
                  :label="`${item.value} — ${item.label}`"
                  :value="item.value"
                />
              </el-select>
            </el-form-item>

            <!-- 描述 -->
            <el-form-item label="描述">
              <el-input
                v-model="form.description"
                type="textarea"
                :rows="3"
                placeholder="选填，描述该 Key 的用途、读写方、生命周期等"
                style="width: 100%"
              />
            </el-form-item>
          </el-form>
        </div>
      </div>
    </div>

    <!-- 底部操作栏（查看模式隐藏） -->
    <div v-if="!isView" class="form-footer">
      <el-button @click="$router.push('/runtime-bb-keys')">取消</el-button>
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
import {
  runtimeBbKeyApi,
  RUNTIME_BB_KEY_ERR,
  RUNTIME_BB_KEY_TYPES,
  RUNTIME_BB_KEY_GROUPS,
} from '@/api/runtimeBbKeys'
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
const hasRefs = ref(false)

interface FormState {
  name: string
  label: string
  type: string
  group_name: string
  description: string
}

const form = reactive<FormState>({
  name: '',
  label: '',
  type: '',
  group_name: '',
  description: '',
})

const namePattern = /^[a-z][a-z0-9_]{1,63}$/

const rules = {
  name: [
    { required: true, message: '请输入英文标识', trigger: 'blur' },
    { pattern: namePattern, message: '小写字母开头，仅含小写字母、数字、下划线，长度 2-64', trigger: 'blur' },
  ],
  label: [
    { required: true, message: '请输入中文标签', trigger: 'blur' },
  ],
  type: [
    { required: true, message: '请选择类型', trigger: 'change' },
  ],
  group_name: [
    { required: true, message: '请选择分组', trigger: 'change' },
  ],
}

// ---------- 初始化 ----------

onMounted(async () => {
  if (!isCreate) {
    await loadDetail()
  }
})

async function loadDetail() {
  const id = Number(route.params.id)
  try {
    const res = await runtimeBbKeyApi.detail(id)
    const data = res.data
    form.name = data.name
    form.label = data.label
    form.type = data.type
    form.group_name = data.group_name
    form.description = data.description || ''
    version.value = data.version
    hasRefs.value = data.has_refs || false
  } catch (err: unknown) {
    if ((err as BizError).code === RUNTIME_BB_KEY_ERR.NOT_FOUND) {
      router.push('/runtime-bb-keys')
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
    const res = await runtimeBbKeyApi.checkName(form.name)
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
    if (isCreate) {
      await runtimeBbKeyApi.create({
        name: form.name,
        type: form.type,
        label: form.label,
        description: form.description,
        group_name: form.group_name,
      })
      ElMessage.success('创建成功，运行时 Key 默认为禁用状态，确认无误后请手动启用')
    } else {
      await runtimeBbKeyApi.update({
        id: Number(route.params.id),
        type: form.type,
        label: form.label,
        description: form.description,
        group_name: form.group_name,
        version: version.value,
      })
      ElMessage.success('保存成功')
    }
    router.push('/runtime-bb-keys')
  } catch (err: unknown) {
    const bizErr = err as BizError
    if (bizErr.code === RUNTIME_BB_KEY_ERR.VERSION_CONFLICT) {
      ElMessageBox.alert('数据已被其他用户修改，请返回列表刷新后重试。', '版本冲突', { type: 'warning' })
      return
    }
    if (
      bizErr.code === RUNTIME_BB_KEY_ERR.NAME_EXISTS ||
      bizErr.code === RUNTIME_BB_KEY_ERR.NAME_INVALID ||
      bizErr.code === RUNTIME_BB_KEY_ERR.NAME_CONFLICT_WITH_FIELD
    ) {
      nameStatus.value = 'taken'
      nameMessage.value = bizErr.message
      return
    }
    if (bizErr.code === RUNTIME_BB_KEY_ERR.EDIT_NOT_DISABLED) {
      ElMessage.warning('请先禁用该运行时 Key 后再编辑')
      return
    }
    if (bizErr.code === RUNTIME_BB_KEY_ERR.TYPE_INVALID || bizErr.code === RUNTIME_BB_KEY_ERR.GROUP_NAME_INVALID) {
      ElMessage.error(bizErr.message)
      return
    }
    // 其他错误拦截器已 toast
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
</style>
