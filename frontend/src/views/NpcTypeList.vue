<template>
  <div>
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px">
      <h2>NPC 类型管理</h2>
      <el-button type="primary" @click="$router.push('/npc-types/new')">新建 NPC</el-button>
    </div>

    <el-table :data="items" v-loading="loading" stripe>
      <el-table-column prop="name" label="名称" min-width="150" />
      <el-table-column label="状态机" min-width="150">
        <template #default="{ row }">{{ row.config?.fsm_ref }}</template>
      </el-table-column>
      <el-table-column label="行为树数" min-width="100">
        <template #default="{ row }">{{ Object.keys(row.config?.bt_refs || {}).length }}</template>
      </el-table-column>
      <el-table-column label="视觉范围" min-width="100">
        <template #default="{ row }">{{ row.config?.perception?.visual_range }}</template>
      </el-table-column>
      <el-table-column label="听觉范围" min-width="100">
        <template #default="{ row }">{{ row.config?.perception?.auditory_range }}</template>
      </el-table-column>
      <el-table-column label="操作" width="180" fixed="right">
        <template #default="{ row }">
          <el-button size="small" @click="$router.push(`/npc-types/${row.name}`)">编辑</el-button>
          <el-popconfirm :title="`确认删除 NPC 类型「${row.name}」？删除后不可恢复。`" @confirm="handleDelete(row.name)">
            <template #reference>
              <el-button size="small" type="danger">删除</el-button>
            </template>
          </el-popconfirm>
        </template>
      </el-table-column>
      <template #empty>
        <el-empty description="暂无 NPC 类型">
          <el-button type="primary" @click="$router.push('/npc-types/new')">创建第一个 NPC</el-button>
        </el-empty>
      </template>
    </el-table>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import * as npcTypeApi from '@/api/npcType'

const items = ref([])
const loading = ref(false)

async function fetchList() {
  loading.value = true
  try {
    const res = await npcTypeApi.list()
    items.value = (res.data.items || []).map(item => ({
      ...item,
      config: typeof item.config === 'string' ? JSON.parse(item.config) : item.config
    }))
  } catch { /* 拦截器已处理 */ } finally { loading.value = false }
}

async function handleDelete(name) {
  try {
    await npcTypeApi.remove(name)
    ElMessage.success('删除成功')
    fetchList()
  } catch { /* 拦截器已处理 */ }
}

onMounted(fetchList)
</script>
