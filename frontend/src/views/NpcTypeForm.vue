<template>
  <div>
    <div style="display: flex; align-items: center; margin-bottom: 16px">
      <el-button @click="$router.push('/npc-types')">返回列表</el-button>
      <h2 style="margin-left: 12px">{{ isEdit ? '编辑 NPC 类型' : '新建 NPC 类型' }}</h2>
    </div>

    <el-form ref="formRef" :model="form" :rules="rules" label-width="140px" style="max-width: 600px" v-loading="pageLoading">
      <el-form-item label="类型名称" prop="type_name">
        <el-input v-model="form.type_name" :disabled="isEdit" placeholder="如 civilian" />
        <div class="field-hint" v-if="!isEdit">唯一标识，以小写字母开头，只能包含小写字母、数字和下划线</div>
        <div class="field-hint" v-else>名称创建后不可修改，如需改名请删除后重新创建</div>
      </el-form-item>

      <el-form-item label="��态机" prop="fsm_ref">
        <template v-if="fsmList.length > 0">
          <el-select v-model="form.fsm_ref" placeholder="选择状态机" @change="onFsmChange">
            <el-option v-for="f in fsmList" :key="f.name" :label="f.name" :value="f.name" />
          </el-select>
        </template>
        <template v-else>
          <el-alert type="warning" :closable="false" show-icon style="padding: 8px 12px">
            暂无可用的状态机，请先
            <el-link type="primary" @click="$router.push('/fsm-configs/new')">创建状态机</el-link>
          </el-alert>
        </template>
        <div class="field-hint">NPC 使用的状态机，决定 NPC 有哪些状态（如空闲、警觉、逃跑）</div>
      </el-form-item>

      <el-form-item label="视觉范围(米)" prop="visual_range">
        <el-slider v-model="form.visual_range" :min="0" :max="1000" :step="10" show-input />
        <div class="field-hint">NPC 能看到多远的事件，超出此范围的视觉事件会被忽略</div>
      </el-form-item>

      <el-form-item label="听觉范围(米)" prop="auditory_range">
        <el-slider v-model="form.auditory_range" :min="0" :max="1000" :step="10" show-input />
        <div class="field-hint">NPC 能听到多远的事件，超出此范围的听觉事件会被忽略</div>
      </el-form-item>

      <el-divider>行为树映射（每个状态对应一棵行为树）</el-divider>

      <div v-if="fsmStates.length > 0">
        <el-form-item v-for="state in fsmStates" :key="state" :label="state">
          <template v-if="btList.length > 0">
            <el-select v-model="form.bt_refs[state]" placeholder="选择行为树" clearable>
              <el-option v-for="bt in btList" :key="bt.name" :label="bt.name" :value="bt.name" />
            </el-select>
          </template>
          <template v-else>
            <el-alert type="warning" :closable="false" show-icon style="padding: 8px 12px">
              暂无可用的行为树，请先
              <el-link type="primary" @click="$router.push('/bt-trees/new')">创建行为树</el-link>
            </el-alert>
          </template>
        </el-form-item>
        <div class="field-hint" style="margin: -8px 0 16px 140px">
          每个状态需要绑定一棵行为树，行为树决定 NPC 在该状态下具体做什么
        </div>
      </div>

      <el-form-item v-else>
        <el-text type="info">请先选择状态机以加载状态列表</el-text>
      </el-form-item>

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
import * as npcTypeApi from '@/api/npcType'
import * as fsmConfigApi from '@/api/fsmConfig'
import * as btTreeApi from '@/api/btTree'
import { createNameRules } from '@/utils/nameRules'

const route = useRoute()
const router = useRouter()
const formRef = ref(null)
const pageLoading = ref(false)
const submitting = ref(false)

const isEdit = computed(() => route.params.name && route.params.name !== 'new')

const fsmList = ref([])
const btList = ref([])
const fsmStates = ref([])

const form = ref({
  type_name: '',
  fsm_ref: '',
  visual_range: 200,
  auditory_range: 500,
  bt_refs: {},
})

const rules = computed(() => ({
  type_name: isEdit.value
    ? [{ required: true, message: '请输入类型名称', trigger: 'blur' }]
    : createNameRules({ listApi: npcTypeApi.list, label: 'NPC 类型名称' }),
  fsm_ref: [{ required: true, message: '请选择状态机', trigger: 'change' }],
}))

async function loadOptions() {
  try {
    const [fsmRes, btRes] = await Promise.all([fsmConfigApi.list(), btTreeApi.list()])
    fsmList.value = fsmRes.data.items || []
    btList.value = btRes.data.items || []
  } catch { /* 拦截器已处理 */ }
}

async function onFsmChange(fsmName) {
  const fsm = fsmList.value.find(f => f.name === fsmName)
  if (!fsm) { fsmStates.value = []; return }
  const config = typeof fsm.config === 'string' ? JSON.parse(fsm.config) : fsm.config
  fsmStates.value = (config.states || []).map(s => s.name)
  // 清除不在新状态中的 bt_refs
  const newRefs = {}
  for (const state of fsmStates.value) {
    newRefs[state] = form.value.bt_refs[state] || ''
  }
  form.value.bt_refs = newRefs
}

async function loadData() {
  if (!isEdit.value) return
  pageLoading.value = true
  try {
    const res = await npcTypeApi.get(route.params.name)
    const config = typeof res.data.config === 'string' ? JSON.parse(res.data.config) : res.data.config
    form.value = {
      type_name: config.type_name || res.data.name,
      fsm_ref: config.fsm_ref || '',
      visual_range: config.perception?.visual_range ?? 200,
      auditory_range: config.perception?.auditory_range ?? 500,
      bt_refs: config.bt_refs || {},
    }
    // 加载 FSM states
    if (form.value.fsm_ref) {
      await onFsmChange(form.value.fsm_ref)
      // 恢复原有映射
      form.value.bt_refs = { ...form.value.bt_refs, ...(config.bt_refs || {}) }
    }
  } catch { /* 拦截器已处理 */ } finally { pageLoading.value = false }
}

async function handleSubmit() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  // 检查是否所有状态都绑定了行为树
  const missingBt = fsmStates.value.filter(s => !form.value.bt_refs[s])
  if (missingBt.length > 0) {
    ElMessage.warning(`以下状态还未绑定行为树：${missingBt.join('、')}`)
    return
  }

  submitting.value = true
  try {
    const payload = {
      name: form.value.type_name,
      config: {
        type_name: form.value.type_name,
        fsm_ref: form.value.fsm_ref,
        bt_refs: form.value.bt_refs,
        perception: {
          visual_range: form.value.visual_range,
          auditory_range: form.value.auditory_range,
        },
      },
    }
    if (isEdit.value) {
      await npcTypeApi.update(route.params.name, payload)
      ElMessage.success('更新成功')
    } else {
      await npcTypeApi.create(payload)
      ElMessage.success('创建成功')
    }
    router.push('/npc-types')
  } catch { /* 拦截器已处理 */ } finally { submitting.value = false }
}

onMounted(async () => {
  await loadOptions()
  await loadData()
})
</script>

<style scoped>
.field-hint {
  font-size: 12px;
  color: #909399;
  line-height: 1.4;
  margin-top: 4px;
}
</style>
