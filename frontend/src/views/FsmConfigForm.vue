<template>
  <div>
    <div style="display: flex; align-items: center; margin-bottom: 16px">
      <el-button @click="$router.push('/fsm-configs')">返回列表</el-button>
      <h2 style="margin-left: 12px">{{ isEdit ? '编辑状态机' : '新建状态机' }}</h2>
    </div>

    <el-form ref="formRef" :model="form" label-width="120px" style="max-width: 700px" v-loading="pageLoading">
      <el-form-item label="状态机名称" required>
        <el-input v-model="form.name" :disabled="isEdit" placeholder="如 civilian" />
      </el-form-item>

      <!-- 状态管理 -->
      <el-form-item label="状态列表">
        <div>
          <el-tag
            v-for="(state, idx) in form.states"
            :key="idx"
            closable
            @close="removeState(idx)"
            style="margin-right: 8px; margin-bottom: 4px"
          >{{ state }}</el-tag>
          <el-input
            v-model="newState"
            size="small"
            style="width: 150px"
            placeholder="添加状态"
            @keyup.enter="addState"
          />
          <el-button size="small" @click="addState" style="margin-left: 4px">添加</el-button>
        </div>
      </el-form-item>

      <el-form-item label="初始状态">
        <el-select v-model="form.initial_state" placeholder="选择初始状态">
          <el-option v-for="s in form.states" :key="s" :label="s" :value="s" />
        </el-select>
      </el-form-item>

      <!-- 转换列表 -->
      <el-divider>状态转换</el-divider>

      <div v-for="(t, idx) in form.transitions" :key="idx" style="border: 1px solid #ebeef5; padding: 12px; margin-bottom: 12px; border-radius: 4px">
        <div style="display: flex; gap: 8px; flex-wrap: wrap; margin-bottom: 8px">
          <el-select v-model="t.from" placeholder="来源状态" size="small" style="width: 120px">
            <el-option v-for="s in form.states" :key="s" :label="s" :value="s" />
          </el-select>
          <span style="line-height: 32px">→</span>
          <el-select v-model="t.to" placeholder="目标状态" size="small" style="width: 120px">
            <el-option v-for="s in form.states" :key="s" :label="s" :value="s" />
          </el-select>
          <el-input-number v-model="t.priority" :min="1" size="small" style="width: 120px" placeholder="优先级" />
          <el-button size="small" type="danger" plain @click="removeTransition(idx)">删除</el-button>
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

const route = useRoute()
const router = useRouter()
const formRef = ref(null)
const pageLoading = ref(false)
const submitting = ref(false)
const newState = ref('')

const isEdit = computed(() => route.params.name && route.params.name !== 'new')

const form = ref({
  name: '',
  initial_state: '',
  states: [],
  transitions: [],
})

function addState() {
  const s = newState.value.trim()
  if (s && !form.value.states.includes(s)) {
    form.value.states.push(s)
  }
  newState.value = ''
}

function removeState(idx) {
  form.value.states.splice(idx, 1)
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
  if (!form.value.name) { ElMessage.warning('请输入状态机名称'); return }

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
