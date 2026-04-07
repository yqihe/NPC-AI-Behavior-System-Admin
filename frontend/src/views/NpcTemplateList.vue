<template>
  <div class="npc-template-list">
    <div class="list-header">
      <h2>NPC 模板管理</h2>
      <el-button type="primary" @click="$router.push('/npc-templates/new')">
        新建
      </el-button>
    </div>

    <el-table
      v-loading="loading"
      :data="items"
      stripe
      style="width: 100%"
    >
      <el-table-column prop="name" label="模板名称" min-width="150" />
      <el-table-column label="预设" width="120">
        <template #default="{ row }">
          <el-tag type="primary" size="small">
            {{ row.config?.preset || '-' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="已启用组件" min-width="300">
        <template #default="{ row }">
          <el-tag
            v-for="comp in getComponents(row)"
            :key="comp"
            size="small"
            style="margin: 2px"
          >
            {{ comp }}
          </el-tag>
          <span v-if="getComponents(row).length === 0" style="color: #909399">无</span>
        </template>
      </el-table-column>
      <el-table-column label="操作" width="160" fixed="right">
        <template #default="{ row }">
          <el-button
            text
            type="primary"
            @click="$router.push(`/npc-templates/${encodeURIComponent(row.name)}`)"
          >
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
        <el-empty description="暂无 NPC 模板">
          <el-button type="primary" @click="$router.push('/npc-templates/new')">
            创建第一个
          </el-button>
        </el-empty>
      </template>
    </el-table>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { npcTemplateApi } from '@/api/generic'

const loading = ref(false)
const items = ref([])

function getComponents(row) {
  return Object.keys(row.config?.components || {})
}

async function loadList() {
  loading.value = true
  try {
    const res = await npcTemplateApi.list()
    items.value = res.data.items || []
  } catch { /* 拦截器已处理 */ }
  finally { loading.value = false }
}

async function handleDelete(name) {
  try {
    await npcTemplateApi.remove(name)
    ElMessage.success(`「${name}」已删除`)
    await loadList()
  } catch { /* 拦截器已处理 */ }
}

onMounted(loadList)
</script>

<style scoped>
.npc-template-list {
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
</style>
