<template>
  <div class="fsm-state-dict-form">
    <!-- 顶部导航 -->
    <div class="form-header">
      <el-icon class="back-icon" @click="$router.push('/fsm-state-dicts')"><ArrowLeft /></el-icon>
      <span class="back-text" @click="$router.push('/fsm-state-dicts')">返回</span>
      <span class="header-sep"></span>
      <span class="header-title">{{ isView ? '查看状态' : isCreate ? '新建状态' : '编辑状态' }}</span>
    </div>

    <!-- 表单滚动区 -->
    <div class="form-scroll">
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
          <!-- 状态标识 -->
          <el-form-item label="状态标识" prop="name">
            <template v-if="!isCreate || isView">
              <el-input :model-value="form.name" :disabled="true" style="width: 100%">
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
                placeholder="如 idle、attack_melee（小写字母开头，仅含小写字母、数字、下划线）"
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
                标识符已存在
              </div>
            </template>
          </el-form-item>

          <!-- 中文名 -->
          <el-form-item label="中文名" prop="display_name">
            <el-input
              v-model="form.display_name"
              placeholder="如 空闲、近战攻击"
              style="width: 100%"
            />
          </el-form-item>

          <!-- 分类 -->
          <el-form-item label="分类" prop="category">
            <el-input
              v-model="form.category"
              placeholder="如 通用、战斗、移动（可从已有分类选择或自行输入）"
              list="category-datalist"
              style="width: 100%"
            />
            <datalist id="category-datalist">
              <option v-for="cat in categories" :key="cat" :value="cat" />
            </datalist>
          </el-form-item>

          <!-- 描述 -->
          <el-form-item label="描述" prop="description">
            <el-input
              v-model="form.description"
              type="textarea"
              :autosize="{ minRows: 3, maxRows: 6 }"
              placeholder="可选，描述该状态的用途或触发条件"
              style="width: 100%"
            />
          </el-form-item>

          <!-- 查看模式下展示时间戳 -->
          <template v-if="isView">
            <el-form-item label="创建时间">
              <el-input :model-value="formatTime(createdAt)" :disabled="true" style="width: 100%" />
            </el-form-item>
            <el-form-item label="更新时间">
              <el-input :model-value="formatTime(updatedAt)" :disabled="true" style="width: 100%" />
            </el-form-item>
          </template>
        </el-form>
      </div>

      <!-- 表单操作（查看模式隐藏） -->
      <div v-if="!isView" class="form-card form-actions">
        <el-button @click="$router.push('/fsm-state-dicts')">取消</el-button>
        <el-button type="primary" :loading="submitting" @click="handleSubmit">
          保存
        </el-button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { FormInstance } from 'element-plus'
import {
  ArrowLeft, Lock, WarningFilled, Loading,
  CircleCheck, CircleClose,
} from '@element-plus/icons-vue'
import { fsmStateDictApi, FSM_STATE_DICT_ERR } from '@/api/fsmStateDicts'
import type { BizError } from '@/api/request'

const route = useRoute()
const router = useRouter()
const isCreate = (route.meta.isCreate as boolean) || false
const isView = (route.meta.isView as boolean) || false

const formRef = ref<FormInstance>()
const submitting = ref(false)
const nameStatus = ref<'' | 'checking' | 'available' | 'taken'>('')
const version = ref(0)
const categories = ref<string[]>([])
const createdAt = ref('')
const updatedAt = ref('')

interface FormState {
  name: string
  display_name: string
  category: string
  description: string
}

const form = reactive<FormState>({
  name: '',
  display_name: '',
  category: '',
  description: '',
})

const namePattern = /^[a-z][a-z0-9_]*$/

const rules = {
  name: [
    { required: true, message: '请输入状态标识', trigger: 'blur' },
    { pattern: namePattern, message: '小写字母开头，仅含小写字母、数字、下划线', trigger: 'blur' },
  ],
  display_name: [
    { required: true, message: '请输入中文名', trigger: 'blur' },
  ],
  category: [
    { required: true, message: '请输入分类', trigger: 'blur' },
  ],
}

// ---------- 初始化 ----------

onMounted(async () => {
  await loadCategories()
  if (!isCreate) {
    await loadDetail()
  }
})

async function loadCategories() {
  try {
    const res = await fsmStateDictApi.listCategories()
    categories.value = res.data ?? []
  } catch {
    // 非关键，静默失败
  }
}

async function loadDetail() {
  const id = Number(route.params.id)
  try {
    const res = await fsmStateDictApi.detail(id)
    const data = res.data
    form.name = data.name
    form.display_name = data.display_name
    form.category = data.category
    form.description = data.description ?? ''
    version.value = data.version
    createdAt.value = data.created_at
    updatedAt.value = data.updated_at
  } catch (err: unknown) {
    if ((err as BizError).code === FSM_STATE_DICT_ERR.NOT_FOUND) {
      ElMessage.error('状态不存在')
      router.push('/fsm-state-dicts')
    }
  }
}

// ---------- 标识符唯一性校验 ----------

async function checkNameUnique() {
  if (!form.name || !namePattern.test(form.name)) {
    nameStatus.value = ''
    return
  }
  nameStatus.value = 'checking'
  try {
    const res = await fsmStateDictApi.checkName(form.name)
    if (res.data?.available) {
      nameStatus.value = 'available'
    } else {
      nameStatus.value = 'taken'
    }
  } catch {
    nameStatus.value = ''
  }
}

// ---------- 提交 ----------

async function handleSubmit() {
  if (!formRef.value) return
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  if (isCreate && nameStatus.value === 'taken') {
    ElMessage.error('标识符已存在，请更换')
    return
  }

  submitting.value = true
  try {
    if (isCreate) {
      await fsmStateDictApi.create({
        name: form.name,
        display_name: form.display_name,
        category: form.category,
        description: form.description || undefined,
      })
      ElMessage.success('创建成功')
      router.push('/fsm-state-dicts')
    } else {
      const id = Number(route.params.id)
      await fsmStateDictApi.update({
        id,
        display_name: form.display_name,
        category: form.category,
        description: form.description,
        version: version.value,
      })
      ElMessage.success('保存成功')
      router.push('/fsm-state-dicts')
    }
  } catch (err: unknown) {
    const bizErr = err as BizError
    if (bizErr.code === FSM_STATE_DICT_ERR.NAME_EXISTS) {
      nameStatus.value = 'taken'
    } else if (bizErr.code === FSM_STATE_DICT_ERR.VERSION_CONFLICT) {
      await ElMessageBox.alert('数据已更新，请刷新后重试', '版本冲突', { type: 'warning' })
      // 不跳转，让用户手动刷新
    }
    // 其他错误拦截器已 toast
  } finally {
    submitting.value = false
  }
}

// ---------- 辅助 ----------

function formatTime(str: string) {
  if (!str) return ''
  const d = new Date(str)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`
}
</script>

<style scoped>
.fsm-state-dict-form {
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.form-header {
  display: flex;
  align-items: center;
  padding: 14px 24px;
  background: #fff;
  border-bottom: 1px solid #E4E7ED;
  flex-shrink: 0;
}

.back-icon {
  font-size: 18px;
  color: #606266;
  cursor: pointer;
}

.back-text {
  font-size: 14px;
  color: #606266;
  cursor: pointer;
  margin-left: 4px;
}

.back-text:hover,
.back-icon:hover {
  color: #409EFF;
}

.header-sep {
  margin: 0 12px;
  color: #C0C4CC;
}

.header-sep::before {
  content: '/';
}

.header-title {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
}

.form-scroll {
  flex: 1;
  overflow-y: auto;
  padding: 24px 32px;
}

.form-card {
  max-width: 800px;
  margin: 0 auto 16px;
  background: #fff;
  border: 1px solid #E4E7ED;
  border-radius: 8px;
  padding: 32px;
}

.card-title {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-bottom: 24px;
}

.title-bar {
  width: 4px;
  height: 18px;
  border-radius: 2px;
}

.title-bar-blue {
  background: #409EFF;
}

.title-text {
  font-size: 15px;
  font-weight: 600;
  color: #303133;
}

.form-actions {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding: 16px 24px;
}

.field-warn {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: #E6A23C;
  margin-top: 4px;
}

.field-hint {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}

.field-hint-success {
  color: #67C23A;
}

.field-hint-error {
  color: #F56C6C;
}
</style>
