<template>
  <div>
    <h2>NPC AI 行为系统 — 运营管理平台</h2>
    <p style="color: #606266; margin: 8px 0 24px">
      在这里配置游戏中 NPC 的行为逻辑。不需要写代码，按照下方步骤操作即可。
    </p>

    <!-- 新手引导 -->
    <el-card shadow="never" style="margin-bottom: 20px">
      <template #header>
        <span style="font-weight: bold">第一次使用？按这个顺序来</span>
      </template>
      <el-steps :active="0" finish-status="wait" align-center>
        <el-step title="第 1 步：创建事件" description="定义游戏世界中会发生什么事，比如爆炸、枪声等">
          <template #icon><span style="font-size: 20px">1</span></template>
        </el-step>
        <el-step title="第 2 步：创建状态机" description="定义 NPC 有哪些状态，比如空闲、警觉、逃跑">
          <template #icon><span style="font-size: 20px">2</span></template>
        </el-step>
        <el-step title="第 3 步：创建行为树" description="定义 NPC 在每个状态下具体做什么">
          <template #icon><span style="font-size: 20px">3</span></template>
        </el-step>
        <el-step title="第 4 步：创建 NPC 模板" description="把以上内容组装成一个完整的 NPC 角色">
          <template #icon><span style="font-size: 20px">4</span></template>
        </el-step>
      </el-steps>
      <el-alert
        type="info"
        :closable="false"
        show-icon
        style="margin-top: 16px"
        title="为什么要按顺序？"
        description="因为后面的配置要用到前面的。比如创建 NPC 模板时需要选择状态机和行为树，所以要先把它们建好。"
      />
    </el-card>

    <!-- 快捷入口 -->
    <el-card shadow="never">
      <template #header>
        <span style="font-weight: bold">当前配置数量</span>
      </template>
      <el-row :gutter="16">
        <el-col v-for="card in cards" :key="card.key" :span="card.span">
          <el-card shadow="hover" class="shortcut-card" @click="$router.push(card.path)">
            <div class="shortcut-title">{{ card.label }}</div>
            <div class="shortcut-desc">{{ card.desc }}</div>
            <div class="shortcut-count" v-loading="loading">{{ counts[card.key] }} {{ card.unit }}</div>
            <el-button type="primary" text size="small" @click.stop="$router.push(card.path)">
              前往管理
            </el-button>
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
  { key: 'eventTypes', label: '事件类型', desc: '游戏中发生的事', path: '/event-types', unit: '个', span: 4 },
  { key: 'fsmConfigs', label: '状态机', desc: 'NPC 的状态切换', path: '/fsm-configs', unit: '个', span: 4 },
  { key: 'btTrees', label: '行为树', desc: 'NPC 的具体行为', path: '/bt-trees', unit: '棵', span: 4 },
  { key: 'npcTemplates', label: 'NPC 模板', desc: '完整的 NPC 角色', path: '/npc-templates', unit: '个', span: 4 },
  { key: 'regions', label: '游戏区域', desc: 'NPC 活动的场景', path: '/regions', unit: '个', span: 4 },
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
  font-size: 15px;
  font-weight: bold;
  color: #303133;
  margin-bottom: 4px;
}
.shortcut-desc {
  font-size: 12px;
  color: #909399;
  margin-bottom: 8px;
}
.shortcut-count {
  font-size: 24px;
  font-weight: bold;
  color: #409eff;
  margin-bottom: 8px;
}
</style>
