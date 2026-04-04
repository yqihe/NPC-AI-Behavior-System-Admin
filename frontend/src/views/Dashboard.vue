<template>
  <div>
    <h2>NPC 配置管理平台</h2>
    <p style="color: #606266; margin: 8px 0 24px">为策划/运营人员提供可视化配置界面，创建和管理 NPC 的事件、状态机、行为树。</p>

    <!-- 创建顺序引导 -->
    <el-card shadow="never" style="margin-bottom: 20px">
      <template #header>
        <span style="font-weight: bold">首次配置？请按以下顺序创建</span>
      </template>
      <el-steps :active="0" finish-status="wait" align-center>
        <el-step title="1. 事件类型" description="定义游戏中会发生什么事（爆炸、枪声等）">
          <template #icon><span style="font-size: 20px">1</span></template>
        </el-step>
        <el-step title="2. 状态机" description="定义 NPC 有哪些状态（空闲、警觉、逃跑等）">
          <template #icon><span style="font-size: 20px">2</span></template>
        </el-step>
        <el-step title="3. 行为树" description="定义 NPC 在每个状态下做什么事">
          <template #icon><span style="font-size: 20px">3</span></template>
        </el-step>
        <el-step title="4. NPC 类型" description="把状态机和行为树组装成完整的 NPC">
          <template #icon><span style="font-size: 20px">4</span></template>
        </el-step>
      </el-steps>
      <el-alert
        type="info"
        :closable="false"
        show-icon
        style="margin-top: 16px"
        title="为什么有顺序？"
        description="因为后面的配置会用到前面的。NPC 需要选择状态机和行为树，所以要先建好它们。"
      />
    </el-card>

    <!-- 注意事项 -->
    <el-card shadow="never" style="margin-bottom: 20px">
      <template #header>
        <span style="font-weight: bold">注意事项</span>
      </template>
      <el-descriptions :column="1" border>
        <el-descriptions-item label="名称不能重复">每个事件/NPC/状态机/行为树的名称必须唯一，输入已存在的名称时会红字提醒</el-descriptions-item>
        <el-descriptions-item label="名称只能用英文">以小写字母开头，可用小写字母、数字、下划线。行为树名称额外允许斜杠 /（如 civilian/idle）</el-descriptions-item>
        <el-descriptions-item label="名称创建后不可修改">保存后名称会锁定（灰色不可点击），如需改名只能删除后重新创建</el-descriptions-item>
        <el-descriptions-item label="删除有影响">删除状态机或行为树前请确认没有 NPC 在使用它，否则会影响对应的 NPC 配置</el-descriptions-item>
        <el-descriptions-item label="配置何时生效">保存后配置写入数据库，需要重启游戏服务端后新配置才会在游戏中生效</el-descriptions-item>
      </el-descriptions>
    </el-card>

    <!-- 快捷入口 -->
    <el-card shadow="never">
      <template #header>
        <span style="font-weight: bold">快捷入口</span>
      </template>
      <el-row :gutter="16">
        <el-col :span="6">
          <el-card shadow="hover" class="shortcut-card" @click="$router.push('/event-types')">
            <div class="shortcut-title">事件类型</div>
            <div class="shortcut-count" v-loading="loading">{{ counts.eventTypes }} 个</div>
            <el-button type="primary" text size="small" @click.stop="$router.push('/event-types/new')">新建</el-button>
          </el-card>
        </el-col>
        <el-col :span="6">
          <el-card shadow="hover" class="shortcut-card" @click="$router.push('/fsm-configs')">
            <div class="shortcut-title">状态机</div>
            <div class="shortcut-count" v-loading="loading">{{ counts.fsmConfigs }} 个</div>
            <el-button type="primary" text size="small" @click.stop="$router.push('/fsm-configs/new')">新建</el-button>
          </el-card>
        </el-col>
        <el-col :span="6">
          <el-card shadow="hover" class="shortcut-card" @click="$router.push('/bt-trees')">
            <div class="shortcut-title">行为树</div>
            <div class="shortcut-count" v-loading="loading">{{ counts.btTrees }} 棵</div>
            <el-button type="primary" text size="small" @click.stop="$router.push('/bt-trees/new')">新建</el-button>
          </el-card>
        </el-col>
        <el-col :span="6">
          <el-card shadow="hover" class="shortcut-card" @click="$router.push('/npc-types')">
            <div class="shortcut-title">NPC 类型</div>
            <div class="shortcut-count" v-loading="loading">{{ counts.npcTypes }} 个</div>
            <el-button type="primary" text size="small" @click.stop="$router.push('/npc-types/new')">新建</el-button>
          </el-card>
        </el-col>
      </el-row>
    </el-card>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import * as eventTypeApi from '@/api/eventType'
import * as npcTypeApi from '@/api/npcType'
import * as fsmConfigApi from '@/api/fsmConfig'
import * as btTreeApi from '@/api/btTree'

const loading = ref(false)
const counts = ref({
  eventTypes: 0,
  fsmConfigs: 0,
  btTrees: 0,
  npcTypes: 0,
})

async function loadCounts() {
  loading.value = true
  try {
    const [e, f, b, n] = await Promise.all([
      eventTypeApi.list(),
      fsmConfigApi.list(),
      btTreeApi.list(),
      npcTypeApi.list(),
    ])
    counts.value = {
      eventTypes: (e.data.items || []).length,
      fsmConfigs: (f.data.items || []).length,
      btTrees: (b.data.items || []).length,
      npcTypes: (n.data.items || []).length,
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
