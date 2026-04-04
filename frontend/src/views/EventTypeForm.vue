<template>
  <div>
    <div style="display: flex; align-items: center; margin-bottom: 16px">
      <el-button @click="$router.push('/event-types')">返回列表</el-button>
      <h2 style="margin-left: 12px">{{ isEdit ? '编辑事件类型' : '新建事件类型' }}</h2>
    </div>

    <el-form ref="formRef" :model="form" :rules="rules" label-width="120px" style="max-width: 500px" v-loading="loading">
      <el-form-item label="事件名称" prop="name">
        <el-input v-model="form.name" :disabled="isEdit" placeholder="如 explosion" />
      </el-form-item>

      <el-form-item label="威胁等级" prop="default_severity">
        <el-slider v-model="form.default_severity" :min="0" :max="100" show-input />
      </el-form-item>

      <el-form-item label="持续时间(s)" prop="default_ttl">
        <el-input-number v-model="form.default_ttl" :min="0.1" :step="1" :precision="1" />
      </el-form-item>

      <el-form-item label="传播方式" prop="perception_mode">
        <el-radio-group v-model="form.perception_mode">
          <el-radio value="visual">视觉</el-radio>
          <el-radio value="auditory">听觉</el-radio>
          <el-radio value="global">全局</el-radio>
        </el-radio-group>
      </el-form-item>

      <el-form-item label="传播范围(m)" prop="range">
        <el-slider v-model="form.range" :min="0" :max="1000" :step="10" show-input />
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
import * as eventTypeApi from '@/api/eventType'

const route = useRoute()
const router = useRouter()
const formRef = ref(null)
const loading = ref(false)
const submitting = ref(false)

const isEdit = computed(() => route.params.name && route.params.name !== 'new')

const form = ref({
  name: '',
  default_severity: 50,
  default_ttl: 10.0,
  perception_mode: 'auditory',
  range: 200,
})

const rules = {
  name: [{ required: true, message: '请输入事件名称', trigger: 'blur' }],
  perception_mode: [{ required: true, message: '请选择传播方式', trigger: 'change' }],
}

async function loadData() {
  if (!isEdit.value) return
  loading.value = true
  try {
    const res = await eventTypeApi.get(route.params.name)
    const config = typeof res.data.config === 'string' ? JSON.parse(res.data.config) : res.data.config
    form.value = {
      name: config.name || res.data.name,
      default_severity: config.default_severity ?? 50,
      default_ttl: config.default_ttl ?? 10,
      perception_mode: config.perception_mode || 'auditory',
      range: config.range ?? 200,
    }
  } catch {
    // 拦截器已处理
  } finally {
    loading.value = false
  }
}

async function handleSubmit() {
  const valid = await formRef.value.validate().catch(() => false)
  if (!valid) return

  submitting.value = true
  try {
    const payload = {
      name: form.value.name,
      config: {
        name: form.value.name,
        default_severity: form.value.default_severity,
        default_ttl: form.value.default_ttl,
        perception_mode: form.value.perception_mode,
        range: form.value.range,
      },
    }
    if (isEdit.value) {
      await eventTypeApi.update(route.params.name, payload)
      ElMessage.success('更新成功')
    } else {
      await eventTypeApi.create(payload)
      ElMessage.success('创建成功')
    }
    router.push('/event-types')
  } catch {
    // 拦截器已处理
  } finally {
    submitting.value = false
  }
}

onMounted(loadData)
</script>
