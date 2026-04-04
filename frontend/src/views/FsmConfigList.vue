<template>
  <div>
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px">
      <h2>状态机管理</h2>
      <el-button type="primary" @click="$router.push('/fsm-configs/new')">新建状态机</el-button>
    </div>

    <el-table :data="items" v-loading="loading" stripe>
      <el-table-column prop="name" label="名称" width="150" />
      <el-table-column label="状态数" width="100">
        <template #default="{ row }">{{ (row.config?.states || []).length }}</template>
      </el-table-column>
      <el-table-column label="转换数" width="100">
        <template #default="{ row }">{{ (row.config?.transitions || []).length }}</template>
      </el-table-column>
      <el-table-column label="初始状态" width="120">
        <template #default="{ row }">{{ row.config?.initial_state }}</template>
      </el-table-column>
      <el-table-column label="操作" width="180" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="$router.push(`/fsm-configs/${row.name}`)">编辑</el-button>
          <el-popconfirm title="确认删除？" @confirm="handleDelete(row.name)">
            <template #reference>
              <el-button size="small" type="danger">删除</el-button>
            </template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import * as fsmConfigApi from '@/api/fsmConfig'

const items = ref([])
const loading = ref(false)

async function fetchList() {
  loading.value = true
  try {
    const res = await fsmConfigApi.list()
    items.value = (res.data.items || []).map(item => ({
      ...item,
      config: typeof item.config === 'string' ? JSON.parse(item.config) : item.config
    }))
  } catch { /* 拦截器已处理 */ } finally { loading.value = false }
}

async function handleDelete(name) {
  try {
    await fsmConfigApi.remove(name)
    ElMessage.success('删除成功')
    fetchList()
  } catch { /* 拦截器已处理 */ }
}

onMounted(fetchList)
</script>
