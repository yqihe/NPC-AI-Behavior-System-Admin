<template>
  <div>
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px">
      <h2>事件类型管理</h2>
      <el-button type="primary" @click="$router.push('/event-types/new')">新建事件</el-button>
    </div>

    <el-table :data="items" v-loading="loading" stripe>
      <el-table-column prop="name" label="名称" width="150" />
      <el-table-column label="威胁等级" width="100">
        <template #default="{ row }">{{ row.config?.default_severity }}</template>
      </el-table-column>
      <el-table-column label="持续时间(s)" width="120">
        <template #default="{ row }">{{ row.config?.default_ttl }}</template>
      </el-table-column>
      <el-table-column label="传播方式" width="100">
        <template #default="{ row }">{{ modeLabel(row.config?.perception_mode) }}</template>
      </el-table-column>
      <el-table-column label="传播范围(m)" width="120">
        <template #default="{ row }">{{ row.config?.range }}</template>
      </el-table-column>
      <el-table-column label="操作" width="180" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="$router.push(`/event-types/${row.name}`)">编辑</el-button>
          <el-popconfirm :title="`确认删除事件「${row.name}」？删除后不可恢复。`" @confirm="handleDelete(row.name)">
            <template #reference>
              <el-button size="small" type="danger">删除</el-button>
            </template>
          </el-popconfirm>
        </template>
      </el-table-column>
      <template #empty>
        <el-empty description="暂无事件类型">
          <el-button type="primary" @click="$router.push('/event-types/new')">创建第一个事件</el-button>
        </el-empty>
      </template>
    </el-table>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import * as eventTypeApi from '@/api/eventType'

const items = ref([])
const loading = ref(false)

const modeLabel = (mode) => {
  const map = { visual: '视觉', auditory: '听觉', global: '全局' }
  return map[mode] || mode || ''
}

async function fetchList() {
  loading.value = true
  try {
    const res = await eventTypeApi.list()
    items.value = (res.data.items || []).map(item => ({
      ...item,
      config: typeof item.config === 'string' ? JSON.parse(item.config) : item.config
    }))
  } catch {
    // 拦截器已处理错误提示
  } finally {
    loading.value = false
  }
}

async function handleDelete(name) {
  try {
    await eventTypeApi.remove(name)
    ElMessage.success('删除成功')
    fetchList()
  } catch {
    // 拦截器已处理
  }
}

onMounted(fetchList)
</script>
