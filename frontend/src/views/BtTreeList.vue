<template>
  <div>
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px">
      <h2>行为树管理</h2>
      <el-button type="primary" @click="$router.push('/bt-trees/new')">新建行为树</el-button>
    </div>

    <el-table :data="items" v-loading="loading" stripe>
      <el-table-column prop="name" label="名称" width="200" />
      <el-table-column label="根节点类型" width="150">
        <template #default="{ row }">{{ row.config?.type }}</template>
      </el-table-column>
      <el-table-column label="子节点数" width="100">
        <template #default="{ row }">{{ (row.config?.children || []).length }}</template>
      </el-table-column>
      <el-table-column label="操作" width="180" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="$router.push(`/bt-trees/${row.name}`)">编辑</el-button>
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
import * as btTreeApi from '@/api/btTree'

const items = ref([])
const loading = ref(false)

async function fetchList() {
  loading.value = true
  try {
    const res = await btTreeApi.list()
    items.value = (res.data.items || []).map(item => ({
      ...item,
      config: typeof item.config === 'string' ? JSON.parse(item.config) : item.config
    }))
  } catch { /* 拦截器已处理 */ } finally { loading.value = false }
}

async function handleDelete(name) {
  try {
    await btTreeApi.remove(name)
    ElMessage.success('删除成功')
    fetchList()
  } catch { /* 拦截器已处理 */ }
}

onMounted(fetchList)
</script>
