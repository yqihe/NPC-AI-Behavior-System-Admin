<template>
  <div class="fsm-config-form">
    <div class="form-header">
      <h2>{{ isEdit ? '编辑状态机' : '新建状态机' }}</h2>
      <el-button @click="goBack">返回列表</el-button>
    </div>

    <el-form
      ref="formRef"
      :model="formModel"
      :rules="nameFieldRules"
      label-position="top"
      style="max-width: 900px"
    >
      <!-- 名称 -->
      <el-form-item label="状态机名称" prop="name">
        <el-input
          v-model="formModel.name"
          :disabled="isEdit"
          placeholder="如 civilian"
        />
      </el-form-item>

      <!-- States -->
      <el-form-item label="状态列表">
        <div class="states-area">
          <el-tag
            v-for="state in states"
            :key="state"
            closable
            style="margin: 2px"
            @close="removeState(state)"
          >
            {{ state }}
          </el-tag>
          <el-input
            v-model="newStateName"
            placeholder="输入状态名后回车"
            style="width: 160px; margin-left: 4px"
            size="small"
            @keyup.enter="addState"
          />
          <el-button size="small" @click="addState" style="margin-left: 4px">添加</el-button>
        </div>
      </el-form-item>

      <!-- Initial State -->
      <el-form-item label="初始状态">
        <el-select v-model="initialState" placeholder="选择初始状态" style="width: 200px">
          <el-option v-for="s in states" :key="s" :label="s" :value="s" />
        </el-select>
      </el-form-item>

      <!-- Transitions -->
      <el-form-item label="转换规则">
        <div
          v-for="(t, index) in transitions"
          :key="index"
          class="transition-item"
        >
          <div class="transition-header">
            <span class="transition-label">转换 {{ index + 1 }}</span>
            <el-button text type="danger" size="small" @click="removeTransition(index)">
              删除
            </el-button>
          </div>
          <div class="transition-fields">
            <el-form-item label="从状态" style="margin-bottom: 8px">
              <el-select v-model="t.from" placeholder="from" style="width: 150px">
                <el-option v-for="s in states" :key="s" :label="s" :value="s" />
              </el-select>
            </el-form-item>
            <el-form-item label="到状态" style="margin-bottom: 8px">
              <el-select v-model="t.to" placeholder="to" style="width: 150px">
                <el-option v-for="s in states" :key="s" :label="s" :value="s" />
              </el-select>
            </el-form-item>
            <el-form-item label="优先级" style="margin-bottom: 8px">
              <el-input-number v-model="t.priority" :min="1" style="width: 120px" />
            </el-form-item>
          </div>
          <el-form-item label="条件" style="margin-bottom: 8px">
            <condition-editor
              v-model="t.condition"
              :operators="conditionOperators"
            />
          </el-form-item>
          <el-divider />
        </div>
        <el-button @click="addTransition" style="margin-top: 4px">
          + 添加转换
        </el-button>
      </el-form-item>

      <!-- 保存 -->
      <el-form-item>
        <el-button type="primary" :loading="saving" @click="handleSave">
          保存
        </el-button>
      </el-form-item>
    </el-form>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import ConditionEditor from '@/components/ConditionEditor.vue'
import { fsmConfigApi } from '@/api/generic'
import { conditionTypeSchemaApi } from '@/api/schema'
import { createNameRules } from '@/utils/nameRules'

const route = useRoute()
const router = useRouter()

const routeName = route.params.name
const isEdit = computed(() => !!routeName && routeName !== 'new')

const formRef = ref(null)
const saving = ref(false)
const formModel = reactive({ name: '' })

const states = ref([])
const initialState = ref('')
const transitions = ref([])
const newStateName = ref('')

// 条件操作符（从 condition-type-schemas leaf 获取）
const conditionOperators = ref(['==', '!=', '>', '>=', '<', '<=', 'in'])

const nameFieldRules = computed(() => ({
  name: isEdit.value
    ? [{ required: true, message: '名称不能为空', trigger: 'blur' }]
    : createNameRules({ listApi: fsmConfigApi.list, label: '状态机' }),
}))

// ========== States ==========

function addState() {
  const name = newStateName.value.trim()
  if (!name) return
  if (states.value.includes(name)) {
    ElMessage.warning(`状态「${name}」已存在`)
    return
  }
  states.value.push(name)
  newStateName.value = ''
}

function removeState(name) {
  states.value = states.value.filter(s => s !== name)
  if (initialState.value === name) initialState.value = ''
}

// ========== Transitions ==========

function addTransition() {
  transitions.value.push({
    from: '',
    to: '',
    priority: 1,
    condition: { key: '', op: '', value: '' },
  })
}

function removeTransition(index) {
  transitions.value.splice(index, 1)
}

// ========== Save ==========

async function handleSave() {
  if (formRef.value) {
    try { await formRef.value.validate() } catch { return }
  }

  if (states.value.length === 0) {
    ElMessage.warning('请至少添加一个状态')
    return
  }
  if (!initialState.value) {
    ElMessage.warning('请选择初始状态')
    return
  }

  saving.value = true
  try {
    const payload = {
      name: formModel.name,
      config: {
        initial_state: initialState.value,
        states: states.value.map(name => ({ name })),
        transitions: transitions.value,
      },
    }

    if (isEdit.value) {
      await fsmConfigApi.update(routeName, payload)
      ElMessage.success('保存成功')
    } else {
      await fsmConfigApi.create(payload)
      ElMessage.success('创建成功')
    }
    goBack()
  } catch { /* 拦截器已处理 */ }
  finally { saving.value = false }
}

function goBack() {
  router.push('/fsm-configs')
}

// ========== Init ==========

onMounted(async () => {
  // 加载条件操作符
  try {
    const res = await conditionTypeSchemaApi.get('leaf')
    const ops = res.data.config?.params_schema?.properties?.op?.enum
    if (ops) conditionOperators.value = ops
  } catch { /* 使用默认操作符 */ }

  // 编辑模式
  if (isEdit.value) {
    try {
      const res = await fsmConfigApi.get(routeName)
      formModel.name = res.data.name
      const config = res.data.config || {}

      initialState.value = config.initial_state || ''
      states.value = (config.states || []).map(s => s.name || s)
      transitions.value = config.transitions || []
    } catch {
      ElMessage.error('加载状态机数据失败')
      goBack()
    }
  }
})
</script>

<style scoped>
.fsm-config-form {
  padding: 24px;
}
.form-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}
.form-header h2 {
  margin: 0;
  color: #303133;
  font-size: 20px;
}
.states-area {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
}
.transition-item {
  border: 1px solid #ebeef5;
  border-radius: 4px;
  padding: 12px;
  margin-bottom: 12px;
}
.transition-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}
.transition-label {
  font-weight: 500;
  color: #606266;
}
.transition-fields {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
}
</style>
