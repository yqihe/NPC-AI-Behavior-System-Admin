<template>
  <div class="fsm-config-form">
    <!-- 顶部导航 -->
    <div class="form-header">
      <el-icon class="back-icon" @click="$router.push('/fsm-configs')"><ArrowLeft /></el-icon>
      <span class="back-text" @click="$router.push('/fsm-configs')">返回</span>
      <span class="header-sep"></span>
      <span class="header-title">
        {{ isView ? '查看状态机' : isCreate ? '新建状态机' : '编辑状态机' }}
      </span>
    </div>

    <!-- 表单滚动区 -->
    <div class="form-scroll">
      <div class="form-body">

        <!-- 基本信息 -->
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
            <!-- 标识 -->
            <el-form-item label="状态机标识" prop="name">
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
                  placeholder="如 wolf_fsm（小写字母开头，仅含小写字母、数字、下划线）"
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
                placeholder="如 狼 FSM（策划可见的显示名称）"
                style="width: 100%"
              />
            </el-form-item>
          </el-form>
        </div>

        <!-- 状态配置 -->
        <div class="form-card">
          <div class="card-title">
            <span class="title-bar title-bar-green"></span>
            <span class="title-text">状态列表</span>
          </div>
          <FsmStateListEditor
            v-model="stateNames"
            v-model:initialState="initialState"
            :disabled="isView"
          />
          <div v-if="statesError" class="states-error">
            <el-icon><WarningFilled /></el-icon>
            {{ statesError }}
          </div>
        </div>

        <!-- 转换规则 -->
        <div class="form-card">
          <div class="card-title">
            <span class="title-bar title-bar-orange"></span>
            <span class="title-text">转换规则</span>
            <el-tag size="small" type="info" style="margin-left: 8px">可选</el-tag>
          </div>
          <FsmTransitionListEditor
            v-model="transitions"
            :states="validStateNames"
            :disabled="isView"
          />
        </div>

      </div><!-- /form-body -->
    </div>

    <!-- 底部操作栏（查看模式隐藏） -->
    <div v-if="!isView" class="form-footer">
      <el-button @click="$router.push('/fsm-configs')">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="handleSubmit">保存</el-button>
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
  CircleCheck, CircleClose,
} from '@element-plus/icons-vue'
import FsmStateListEditor from '@/components/FsmStateListEditor.vue'
import FsmTransitionListEditor from '@/components/FsmTransitionListEditor.vue'
import { fsmConfigApi, FSM_ERR } from '@/api/fsmConfigs'
import type { FsmTransition } from '@/api/fsmConfigs'
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
const statesError = ref('')

const form = reactive({
  name: '',
  display_name: '',
})

// 状态名数组（字符串列表，不含 { name: ... } 包装）
const stateNames = ref<string[]>([])
const initialState = ref('')
const transitions = ref<FsmTransition[]>([])

// 有效（非空且无重名）的状态名
const validStateNames = computed(() =>
  stateNames.value.filter((n, idx) =>
    n && stateNames.value.indexOf(n) === idx,
  ),
)

const namePattern = /^[a-z][a-z0-9_]*$/

const rules = {
  name: [
    { required: true, message: '请输入状态机标识', trigger: 'blur' },
    { pattern: namePattern, message: '小写字母开头，仅含小写字母、数字、下划线', trigger: 'blur' },
  ],
  display_name: [
    { required: true, message: '请输入中文名称', trigger: 'blur' },
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
    const res = await fsmConfigApi.detail(id)
    const data = res.data
    form.name = data.name
    form.display_name = data.display_name
    version.value = data.version

    const cfg = data.config || {}
    stateNames.value = (cfg.states || []).map((s) => s.name)
    initialState.value = cfg.initial_state || ''
    transitions.value = cfg.transitions || []
  } catch (err: unknown) {
    if ((err as BizError).code === FSM_ERR.NOT_FOUND) {
      ElMessage.error('状态机不存在')
      router.push('/fsm-configs')
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
    const res = await fsmConfigApi.checkName(form.name)
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
  // 前端校验：状态列表非空
  if (stateNames.value.filter((n) => n).length === 0) {
    statesError.value = '至少定义一个状态'
    return
  }
  statesError.value = ''

  const valid = await formRef.value?.validate().catch(() => false)
  if (!valid) return

  if (isCreate && nameStatus.value === 'taken') {
    ElMessage.warning('标识符已被使用，请更换')
    return
  }

  submitting.value = true
  try {
    const states = validStateNames.value.map((n) => ({ name: n }))

    if (isCreate) {
      await fsmConfigApi.create({
        name: form.name,
        display_name: form.display_name,
        initial_state: initialState.value,
        states,
        transitions: transitions.value,
      })
      ElMessage.success('创建成功，状态机默认为禁用状态，确认无误后请手动启用')
    } else {
      await fsmConfigApi.update({
        id: Number(route.params.id),
        display_name: form.display_name,
        initial_state: initialState.value,
        states,
        transitions: transitions.value,
        version: version.value,
      })
      ElMessage.success('保存成功')
    }
    router.push('/fsm-configs')
  } catch (err: unknown) {
    const bizErr = err as BizError
    if (bizErr.code === FSM_ERR.VERSION_CONFLICT) {
      ElMessageBox.alert('数据已被其他人修改，请刷新后重试。', '版本冲突', { type: 'warning' })
      return
    }
    if (bizErr.code === FSM_ERR.NAME_EXISTS || bizErr.code === FSM_ERR.NAME_INVALID) {
      nameStatus.value = 'taken'
      nameMessage.value = bizErr.message
      return
    }
    if (bizErr.code === FSM_ERR.NOT_FOUND) {
      ElMessage.error('状态机不存在')
      router.push('/fsm-configs')
      return
    }
    if (bizErr.code === FSM_ERR.EDIT_NOT_DISABLED) {
      ElMessage.warning('请先禁用该状态机后再编辑')
      return
    }
    if (
      bizErr.code === FSM_ERR.STATES_EMPTY ||
      bizErr.code === FSM_ERR.STATE_NAME_INVALID ||
      bizErr.code === FSM_ERR.INITIAL_INVALID ||
      bizErr.code === FSM_ERR.TRANSITION_INVALID ||
      bizErr.code === FSM_ERR.CONDITION_INVALID
    ) {
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
.fsm-config-form {
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

.states-error {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 12px;
  font-size: 13px;
  color: #F56C6C;
}
</style>
