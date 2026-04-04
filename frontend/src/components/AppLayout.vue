<template>
  <el-container style="height: 100vh">
    <el-aside width="200px" style="background: #304156">
      <div style="padding: 20px; color: #fff; font-size: 16px; text-align: center; font-weight: bold">
        NPC 配置管理
      </div>
      <el-menu
        :default-active="activeMenu"
        background-color="#304156"
        text-color="#bfcbd9"
        active-text-color="#409eff"
        router
      >
        <el-menu-item index="/event-types">
          <span>事件管理</span>
        </el-menu-item>
        <el-menu-item index="/npc-types">
          <span>NPC 管理</span>
        </el-menu-item>
        <el-menu-item index="/fsm-configs">
          <span>状态机管理</span>
        </el-menu-item>
        <el-menu-item index="/bt-trees">
          <span>行为树管理</span>
        </el-menu-item>
      </el-menu>
    </el-aside>
    <el-main style="padding: 20px; background: #f0f2f5">
      <router-view />
    </el-main>
  </el-container>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()
const activeMenu = computed(() => {
  // 从 /event-types/xxx 提取 /event-types
  const parts = route.path.split('/')
  return '/' + (parts[1] || '')
})
</script>
