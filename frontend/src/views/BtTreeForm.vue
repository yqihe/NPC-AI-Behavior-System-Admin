<template>
  <div>
    <div style="display: flex; align-items: center; margin-bottom: 16px">
      <el-button @click="$router.push('/bt-trees')">返回列表</el-button>
      <h2 style="margin-left: 12px">{{ isEdit ? '编辑行为树' : '新建行为树' }}</h2>
    </div>

    <el-form label-width="120px" style="max-width: 800px" v-loading="pageLoading">
      <el-form-item label="行为树名称">
        <el-input v-model="form.name" :disabled="isEdit" placeholder="如 civilian/idle" />
      </el-form-item>

      <el-divider>节点编辑</el-divider>

      <BtNodeEditor v-model="form.rootNode" :depth="0" />

      <el-form-item style="margin-top: 16px">
        <el-button type="primary" @click="handleSubmit" :loading="submitting">保存</el-button>
      </el-form-item>
    </el-form>
  </div>
</template>

<script setup>
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import * as btTreeApi from '@/api/btTree'
import BtNodeEditor from '@/components/BtNodeEditor.vue'

const route = useRoute()
const router = useRouter()
const pageLoading = ref(false)
const submitting = ref(false)

const isEdit = computed(() => route.params.name && route.params.name !== 'new')

const form = ref({
  name: '',
  rootNode: { type: 'sequence', children: [] },
})

async function loadData() {
  if (!isEdit.value) return
  pageLoading.value = true
  try {
    const res = await btTreeApi.get(route.params.name)
    const config = typeof res.data.config === 'string' ? JSON.parse(res.data.config) : res.data.config
    form.value = {
      name: res.data.name,
      rootNode: config || { type: 'sequence', children: [] },
    }
  } catch { /* 拦截器已处理 */ } finally { pageLoading.value = false }
}

async function handleSubmit() {
  if (!form.value.name) { ElMessage.warning('请输入行为树名称'); return }

  submitting.value = true
  try {
    const payload = {
      name: form.value.name,
      config: form.value.rootNode,
    }
    if (isEdit.value) {
      await btTreeApi.update(route.params.name, payload)
      ElMessage.success('更新成功')
    } else {
      await btTreeApi.create(payload)
      ElMessage.success('创建成功')
    }
    router.push('/bt-trees')
  } catch { /* 拦截器已处理 */ } finally { submitting.value = false }
}

onMounted(loadData)
</script>
