<template>
  <div>
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px">
      <h2>行为树管理</h2>
      <el-button type="primary" @click="$router.push('/bt-trees/new')">新建行为树</el-button>
    </div>

    <div v-loading="loading">
      <template v-if="groups.length > 0">
        <div v-for="group in groups" :key="group.label" style="margin-bottom: 24px">
          <h3 style="margin-bottom: 8px; color: #303133; font-size: 15px; border-left: 3px solid #409eff; padding-left: 8px">
            {{ group.label }}
            <el-tag size="small" style="margin-left: 8px">{{ group.items.length }} 棵</el-tag>
          </h3>
          <el-table :data="group.items" stripe size="small">
            <el-table-column label="名称" width="200">
              <template #default="{ row }">{{ row.shortName }}</template>
            </el-table-column>
            <el-table-column label="完整路径" width="200">
              <template #default="{ row }">{{ row.name }}</template>
            </el-table-column>
            <el-table-column label="根节点类型" width="150">
              <template #default="{ row }">{{ nodeTypeLabel(row.config?.type) }}</template>
            </el-table-column>
            <el-table-column label="子节点数" width="100">
              <template #default="{ row }">{{ (row.config?.children || []).length }}</template>
            </el-table-column>
            <el-table-column label="操作" width="180" fixed="right">
              <template #default="{ row }">
                <el-button size="small" @click="$router.push(`/bt-trees/${encodeURIComponent(row.name)}`)">编辑</el-button>
                <el-popconfirm :title="`确认删除行为树「${row.name}」？使用此行为树的 NPC 将受影响。`" @confirm="handleDelete(row.name)">
                  <template #reference>
                    <el-button size="small" type="danger">删除</el-button>
                  </template>
                </el-popconfirm>
              </template>
            </el-table-column>
          </el-table>
        </div>
      </template>
      <el-empty v-else-if="!loading" description="暂无行为树">
        <el-button type="primary" @click="$router.push('/bt-trees/new')">创建第一棵行为树</el-button>
      </el-empty>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import * as btTreeApi from '@/api/btTree'

const items = ref([])
const loading = ref(false)

const nodeTypeLabel = (type) => {
  const map = {
    sequence: '顺序 (sequence)',
    selector: '选择 (selector)',
    parallel: '并行 (parallel)',
    inverter: '反转 (inverter)',
  }
  return map[type] || type || ''
}

// 按 NPC 类型分组：civilian/idle → 分组名 "civilian"，shortName "idle"
// 没有 "/" 的归入"未分类"
const groups = computed(() => {
  const map = {}
  for (const item of items.value) {
    const slashIdx = item.name.indexOf('/')
    let groupKey, shortName
    if (slashIdx > 0) {
      groupKey = item.name.substring(0, slashIdx)
      shortName = item.name.substring(slashIdx + 1)
    } else {
      groupKey = '未分类'
      shortName = item.name
    }
    if (!map[groupKey]) map[groupKey] = []
    map[groupKey].push({ ...item, shortName })
  }
  // 按分组名排序，"未分类"排最后
  return Object.keys(map)
    .sort((a, b) => {
      if (a === '未分类') return 1
      if (b === '未分类') return -1
      return a.localeCompare(b)
    })
    .map(key => ({ label: key, items: map[key] }))
})

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
