<template>
  <div class="export-manager">
    <h2>导出管理</h2>
    <p style="color: #909399; margin-bottom: 16px">
      以下接口供游戏服务端拉取配置。服务端启动时通过 HTTP API 获取全量配置。
    </p>

    <el-table :data="exportItems" v-loading="loading" stripe>
      <el-table-column prop="label" label="配置类型" width="150" />
      <el-table-column prop="url" label="导出 URL" min-width="300">
        <template #default="{ row }">
          <code>{{ row.url }}</code>
        </template>
      </el-table-column>
      <el-table-column label="数据条数" width="100" align="center">
        <template #default="{ row }">
          <el-tag :type="row.count > 0 ? 'success' : 'info'" size="small">
            {{ row.count }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="120" align="center">
        <template #default="{ row }">
          <el-button text type="primary" size="small" @click="copyUrl(row.url)">
            复制 URL
          </el-button>
        </template>
      </el-table-column>
    </el-table>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import {
  npcTemplateApi,
  eventTypeApi,
  fsmConfigApi,
  btTreeApi,
  regionApi,
} from '@/api/generic'

const loading = ref(false)

const exportConfigs = [
  { label: 'NPC 模板', collection: 'npc_templates', api: npcTemplateApi },
  { label: '事件类型', collection: 'event_types', api: eventTypeApi },
  { label: '状态机', collection: 'fsm_configs', api: fsmConfigApi },
  { label: '行为树', collection: 'bt_trees', api: btTreeApi },
  { label: '区域', collection: 'regions', api: regionApi },
]

const exportItems = ref(
  exportConfigs.map(c => ({
    label: c.label,
    url: `/api/configs/${c.collection}`,
    count: 0,
  }))
)

async function loadCounts() {
  loading.value = true
  try {
    const results = await Promise.all(exportConfigs.map(c => c.api.list()))
    for (let i = 0; i < results.length; i++) {
      exportItems.value[i].count = (results[i].data.items || []).length
    }
  } catch { /* 拦截器已处理 */ }
  finally { loading.value = false }
}

async function copyUrl(url) {
  const fullUrl = `${window.location.origin}${url}`
  try {
    await navigator.clipboard.writeText(fullUrl)
    ElMessage.success('URL 已复制到剪贴板')
  } catch {
    ElMessage.info(`请手动复制: ${fullUrl}`)
  }
}

onMounted(loadCounts)
</script>

<style scoped>
.export-manager {
  padding: 24px;
}
.export-manager h2 {
  margin: 0 0 8px;
  color: #303133;
  font-size: 20px;
}
code {
  background: #f5f7fa;
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 13px;
  color: #409eff;
}
</style>
