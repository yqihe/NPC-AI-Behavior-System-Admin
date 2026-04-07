<template>
  <div>
    <h2>NPC AI 行为系统 — 运营管理平台</h2>
    <p style="color: #606266; margin: 8px 0 24px">
      为策划/运营人员提供可视化配置界面，管理 NPC 模板、事件、状态机、行为树和区域配置。
    </p>

    <!-- 快捷入口 -->
    <el-card shadow="never">
      <template #header>
        <span style="font-weight: bold">配置概览</span>
      </template>
      <el-row :gutter="16">
        <el-col v-for="card in cards" :key="card.key" :span="card.span">
          <el-card shadow="hover" class="shortcut-card" @click="$router.push(card.path)">
            <div class="shortcut-title">{{ card.label }}</div>
            <div class="shortcut-count" v-loading="loading">{{ counts[card.key] }} {{ card.unit }}</div>
          </el-card>
        </el-col>
      </el-row>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import {
  npcTemplateApi,
  eventTypeApi,
  fsmConfigApi,
  btTreeApi,
  regionApi,
} from '@/api/generic'

const cards = [
  { key: 'npcTemplates', label: 'NPC 模板', path: '/npc-templates', unit: '个', span: 4 },
  { key: 'eventTypes', label: '事件类型', path: '/event-types', unit: '个', span: 4 },
  { key: 'fsmConfigs', label: '状态机', path: '/fsm-configs', unit: '个', span: 4 },
  { key: 'btTrees', label: '行为树', path: '/bt-trees', unit: '棵', span: 4 },
  { key: 'regions', label: '区域', path: '/regions', unit: '个', span: 4 },
]

const loading = ref(false)
const counts = ref({
  npcTemplates: 0,
  eventTypes: 0,
  fsmConfigs: 0,
  btTrees: 0,
  regions: 0,
})

async function loadCounts() {
  loading.value = true
  try {
    const [n, e, f, b, r] = await Promise.all([
      npcTemplateApi.list(),
      eventTypeApi.list(),
      fsmConfigApi.list(),
      btTreeApi.list(),
      regionApi.list(),
    ])
    counts.value = {
      npcTemplates: (n.data.items || []).length,
      eventTypes: (e.data.items || []).length,
      fsmConfigs: (f.data.items || []).length,
      btTrees: (b.data.items || []).length,
      regions: (r.data.items || []).length,
    }
  } catch { /* 拦截器已处理 */ }
  finally { loading.value = false }
}

onMounted(loadCounts)
</script>

<style scoped>
.shortcut-card {
  text-align: center;
  cursor: pointer;
  transition: transform 0.2s;
}
.shortcut-card:hover {
  transform: translateY(-2px);
}
.shortcut-title {
  font-size: 14px;
  color: #606266;
  margin-bottom: 8px;
}
.shortcut-count {
  font-size: 24px;
  font-weight: bold;
  color: #303133;
  margin-bottom: 8px;
}
</style>
