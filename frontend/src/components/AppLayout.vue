<template>
  <el-container style="height: 100vh">
    <el-aside width="200px" style="background: #304156">
      <div
        style="padding: 20px; color: #fff; font-size: 16px; text-align: center; font-weight: bold; cursor: pointer"
        @click="$router.push('/')"
      >
        NPC AI 行为系统
      </div>
      <el-menu
        :default-active="activeMenu"
        background-color="#304156"
        text-color="#bfcbd9"
        active-text-color="#409eff"
        router
      >
        <el-menu-item index="/">
          <span>首页</span>
        </el-menu-item>

        <el-menu-item-group title="配置管理">
          <el-menu-item index="/npc-templates">
            <span>NPC 模板</span>
          </el-menu-item>
          <el-menu-item index="/event-types">
            <span>事件类型</span>
          </el-menu-item>
          <el-menu-item index="/fsm-configs">
            <span>状态机</span>
          </el-menu-item>
          <el-menu-item index="/bt-trees">
            <span>行为树</span>
          </el-menu-item>
        </el-menu-item-group>

        <el-menu-item-group title="世界管理">
          <el-menu-item index="/regions">
            <span>区域管理</span>
          </el-menu-item>
        </el-menu-item-group>

        <el-menu-item-group title="系统设置">
          <el-menu-item index="/schemas">
            <span>Schema 管理</span>
          </el-menu-item>
          <el-menu-item index="/exports">
            <span>导出管理</span>
          </el-menu-item>
        </el-menu-item-group>
      </el-menu>
    </el-aside>
    <el-main style="padding: 20px; background: #f0f2f5">
      <router-view :key="route.fullPath" />
    </el-main>
  </el-container>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()
const activeMenu = computed(() => {
  if (route.path === '/') return '/'
  const parts = route.path.split('/')
  return '/' + (parts[1] || '')
})
</script>
