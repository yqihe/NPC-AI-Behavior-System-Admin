<template>
  <el-dialog
    v-model="visible"
    title="模板引用详情"
    width="520px"
    :close-on-click-modal="true"
    @close="onClose"
  >
    <div v-if="loading" class="refs-loading">
      <el-icon class="is-loading"><Loading /></el-icon>
      <span>加载中...</span>
    </div>
    <template v-else-if="data">
      <div class="refs-header">
        <span class="refs-label">{{ data.template_label }}</span>
        <span class="refs-name">({{ data.template_id }})</span>
        <span class="refs-spacer"></span>
        <span class="refs-count">共 {{ data.npcs.length }} 个 NPC 在使用</span>
      </div>
      <el-empty
        v-if="data.npcs.length === 0"
        description="暂无 NPC 引用"
        :image-size="80"
      />
      <el-table v-else :data="data.npcs" max-height="320">
        <el-table-column prop="npc_id" label="ID" width="80" />
        <el-table-column prop="npc_name" label="NPC 名称" />
      </el-table>
    </template>
    <template #footer>
      <el-button @click="visible = false">关闭</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { Loading } from '@element-plus/icons-vue'
import { templateApi } from '@/api/templates'
import type { TemplateListItem, TemplateReferenceDetail } from '@/api/templates'

const visible = ref(false)
const loading = ref(false)
const data = ref<TemplateReferenceDetail | null>(null)

async function open(template: TemplateListItem) {
  visible.value = true
  loading.value = true
  data.value = null
  try {
    const res = await templateApi.references(template.id)
    data.value = res.data
  } catch {
    // 拦截器已 toast
    visible.value = false
  } finally {
    loading.value = false
  }
}

function onClose() {
  data.value = null
  loading.value = false
}

defineExpose({ open })
</script>

<style scoped>
.refs-loading {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  padding: 40px 0;
  color: #909399;
  font-size: 13px;
}

.refs-header {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 12px 14px;
  margin-bottom: 12px;
  background: #F5F7FA;
  border-radius: 4px;
}

.refs-label {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
}

.refs-name {
  font-size: 12px;
  color: #909399;
}

.refs-spacer {
  flex: 1;
}

.refs-count {
  font-size: 12px;
  color: #606266;
}
</style>
