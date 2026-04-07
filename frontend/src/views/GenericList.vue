<template>
  <div class="generic-list">
    <div class="list-header">
      <h2>{{ title }}</h2>
      <el-button type="primary" @click="$router.push(`/${entityPath}/new`)">
        新建
      </el-button>
    </div>

    <el-table
      v-loading="loading"
      :data="items"
      stripe
      style="width: 100%"
    >
      <el-table-column prop="name" label="名称" min-width="180" />
      <el-table-column label="配置摘要" min-width="300">
        <template #default="{ row }">
          <span class="config-summary">{{ configSummary(row.config) }}</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="160" fixed="right">
        <template #default="{ row }">
          <el-button text type="primary" @click="$router.push(`/${entityPath}/${encodeURIComponent(row.name)}`)">
            编辑
          </el-button>
          <el-popconfirm
            :title="`确认删除「${row.name}」？删除后不可恢复。`"
            confirm-button-text="删除"
            cancel-button-text="取消"
            confirm-button-type="danger"
            @confirm="handleDelete(row.name)"
          >
            <template #reference>
              <el-button text type="danger">删除</el-button>
            </template>
          </el-popconfirm>
        </template>
      </el-table-column>

      <template #empty>
        <el-empty :description="`暂无${title}数据`">
          <el-button type="primary" @click="$router.push(`/${entityPath}/new`)">
            创建第一个
          </el-button>
        </el-empty>
      </template>
    </el-table>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { ElMessage } from 'element-plus'

const route = useRoute()
const title = route.meta?.title || '配置管理'
const entityPath = route.meta?.entityPath || ''
const api = route.meta?.api

const loading = ref(false)
const items = ref([])

async function loadList() {
  if (!api) return
  loading.value = true
  try {
    const res = await api.list()
    items.value = res.data.items || []
  } catch { /* 拦截器已处理 */ }
  finally { loading.value = false }
}

async function handleDelete(name) {
  if (!api) return
  try {
    await api.remove(name)
    ElMessage.success(`「${name}」已删除`)
    await loadList()
  } catch { /* 拦截器已处理 */ }
}

/**
 * 从 config 对象中提取前 3 个非嵌套字段作为摘要。
 */
function configSummary(config) {
  if (!config || typeof config !== 'object') return '-'
  const entries = Object.entries(config)
    .filter(([, v]) => v !== null && typeof v !== 'object')
    .slice(0, 3)
  if (entries.length === 0) return '-'
  return entries.map(([k, v]) => `${k}: ${v}`).join(' | ')
}

onMounted(loadList)
</script>

<style scoped>
.generic-list {
  padding: 24px;
}
.list-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}
.list-header h2 {
  margin: 0;
  color: #303133;
  font-size: 20px;
}
.config-summary {
  color: #909399;
  font-size: 13px;
}
</style>
