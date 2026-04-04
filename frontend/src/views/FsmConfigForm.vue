<template>
  <div>
    <div style="display: flex; align-items: center; margin-bottom: 16px">
      <el-button @click="$router.push('/fsm-configs')">返回列表</el-button>
      <h2 style="margin-left: 12px">{{ isEdit ? '编辑状态机' : '新建状态机' }}</h2>
    </div>

    <el-form ref="formRef" :model="form" :rules="rules" label-width="140px" style="max-width: 700px" v-loading="pageLoading">
      <el-form-item label="状态机名称" prop="name">
        <el-input v-model="form.name" :disabled="isEdit" placeholder="如 civilian" />
        <div class="field-hint" v-if="!isEdit">唯一标识，以小写字母开头，只能包含小写字母、数字和下划线</div>
        <div class="field-hint" v-else>名称创建后不可修改，如需改名请删除后重新创建</div>
      </el-form-item>

      <!-- 状态管理 -->
      <el-form-item label="状态列表" required>
        <div>
          <el-tag
            v-for="(state, idx) in form.states"
            :key="idx"
            closable
            @close="removeState(idx)"
            style="margin-right: 8px; margin-bottom: 4px"
          >{{ state }}</el-tag>
          <div style="display: flex; align-items: center; gap: 4px; margin-top: 4px">
            <el-input
              v-model="newState"
              size="small"
              style="width: 150px"
              placeholder="输入状态名"
              @keyup.enter="addState"
              :class="{ 'is-error-input': newStateError }"
            />
            <el-button size="small" @click="addState">添加</el-button>
          </div>
          <div v-if="newStateError" style="color: #f56c6c; font-size: 12px; margin-top: 2px">{{ newStateError }}</div>
        </div>
        <div class="field-hint">NPC 可能处于的状态，如 Idle（空闲）、Alarmed（警觉）、Flee（逃跑）</div>
      </el-form-item>

      <el-form-item label="初始状态" prop="initial_state">
        <el-select v-model="form.initial_state" placeholder="选择初始状态">
          <el-option v-for="s in form.states" :key="s" :label="s" :value="s" />
        </el-select>
        <div class="field-hint">NPC 创建后默认处于的状态</div>
      </el-form-item>

      <!-- 转换列表 -->
      <el-divider>状态转换规则</el-divider>
      <div class="field-hint" style="margin: -8px 0 16px 0">
        定义 NPC 在什么条件下从一个状态切换到另一个状态。优先级数字越小越优先执行。
      </div>

      <div v-for="(t, idx) in form.transitions" :key="idx" style="border: 1px solid #ebeef5; padding: 12px; margin-bottom: 12px; border-radius: 4px">
        <div style="display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 8px">
          <el-select v-model="t.from" placeholder="来源状态" size="small" style="width: 120px">
            <el-option v-for="s in form.states" :key="s" :label="s" :value="s" />
          </el-select>
          <span style="line-height: 32px">→</span>
          <el-select v-model="t.to" placeholder="目标状态" size="small" style="width: 120px">
            <el-option v-for="s in form.states" :key="s" :label="s" :value="s" />
          </el-select>
          <el-input-number v-model="t.priority" :min="1" size="small" style="width: 130px" />
          <span style="line-height: 32px; font-size: 12px; color: #909399">优先级</span>
          <el-button size="small" type="danger" plain @click="removeTransition(idx)">删除</el-button>
        </div>
        <div v-if="t.from && t.to && t.from === t.to" style="color: #e6a23c; font-size: 12px; margin-bottom: 4px">
          注意：来源和目标状态相同（自循环转换）
        </div>
        <div style="padding-left: 8px">
          <span style="font-size: 12px; color: #909399">触发条件：</span>
          <ConditionEditor v-model="t.condition" />
        </div>
      </div>

      <el-button @click="addTransition" style="margin-bottom: 16px">添加转换</el-button>

      <el-form-item>
        <el-button type="primary" @click="handleSubmit" :loading="submitting">保存</el-button>
      </el-form-item>
    </el-form>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import * as fsmConfigApi from '@/api/fsmConfig'
import ConditionEditor from '@/components/ConditionEditor.vue'
import { createNameRules } from '@/utils/nameRules'

const route = useRoute()
const router = useRouter()
const formRef = ref(null)
const pageLoading = ref(false)
const submitting = ref(false)
const newState = ref('')
const newStateError = ref('')

const isEdit = computed(() => route.params.name && route.params.name !== 'new')

const form = ref({
  name: '',
  initial_state: '',
  states: [],
  transitions: [],
})

const rules = computed(() => ({
  name: isEdit.value
    ? [{ required: true, message: '请输入状态机名称', trigger: 'blur' }]
    : createNameRules({ listApi: fsmConfigApi.list, label: '状态机名称' }),
  initial_state: [{ required: true, message: '请选择初始状态', trigger: 'change' }],
}))

function addState() {
  const s = newState.value.trim()
  newStateError.value = ''

  if (!s) {
    newStateError.value = '状态名不能为空'
    return
  }
  if (form.value.states.includes(s)) {
    newStateError.value = `状态 "${s}" 已存在，不能重复添加`
    return
  }
  form.value.states.push(s)
  newState.value = ''
}

function removeState(idx) {
  const removed = form.value.states[idx]
  form.value.states.splice(idx, 1)
  // 如果删除的是初始状态，清空选择
  if (form.value.initial_state === removed) {
    form.value.initial_state = ''
  }
}

function addTransition() {
  form.value.transitions.push({ from: '', to: '', priority: 5, condition: { key: '', op: '==', value: '' } })
}

function removeTransition(idx) {
  form.value.transitions.splice(idx, 1)
}

async function loadData() {
  if (!isEdit.value) return
  pageLoading.value = true
  try {
    const res = await fsmConfigApi.get(route.params.name)
    const config = typeof res.data.config === 'string' ? JSON.parse(res.data.config) : res.data.config
    form.value = {
      name: res.data.name,
      initial_state: config.initial_state || '',
      states: (config.states || []).map(s => s.name),
      transitions: (config.transitions || []).map(t => ({
        from: t.from || '',
        to: t.to || '',
        priority: t.priority || 5,
        condition: t.condition || { key: '', op: '==', value: '' },
      })),
    }
  } catch { /* 拦截器已处理 */ } finally { pageLoading.value = false }
}

async function handleSubmit() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  // 额外校验
  if (form.value.states.length === 0) {
    ElMessage.warning('请至少添加一个状态')
    return
  }

  // 检查转换引用的状态是否都存在
  for (let i = 0; i < form.value.transitions.length; i++) {
    const t = form.value.transitions[i]
    if (!t.from) {
      ElMessage.warning(`转换 #${i + 1}：请选择来源状态`)
      return
    }
    if (!t.to) {
      ElMessage.warning(`转换 #${i + 1}：请选择目标状态`)
      return
    }
  }

  submitting.value = true
  try {
    const payload = {
      name: form.value.name,
      config: {
        initial_state: form.value.initial_state,
        states: form.value.states.map(name => ({ name })),
        transitions: form.value.transitions.map(t => ({
          from: t.from,
          to: t.to,
          priority: t.priority,
          condition: t.condition,
        })),
      },
    }
    if (isEdit.value) {
      await fsmConfigApi.update(route.params.name, payload)
      ElMessage.success('更新成功')
    } else {
      await fsmConfigApi.create(payload)
      ElMessage.success('创建成功')
    }
    router.push('/fsm-configs')
  } catch { /* 拦截器已处理 */ } finally { submitting.value = false }
}

onMounted(loadData)
</script>

<style scoped>
.field-hint {
  font-size: 12px;
  color: #909399;
  line-height: 1.4;
  margin-top: 4px;
}
</style>
