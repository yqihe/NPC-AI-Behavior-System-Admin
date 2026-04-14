<template>
  <el-container style="height: 100vh">
    <el-aside width="220px" class="sidebar">
      <div class="sidebar-logo" @click="$router.push('/')">
        ADMIN 运营平台
      </div>
      <div class="sidebar-sep"></div>
      <el-menu
        :default-active="activeMenu"
        :default-openeds="defaultOpeneds"
        background-color="#1D2B3A"
        text-color="#bfcbd9"
        active-text-color="#409EFF"
        router
      >
        <el-sub-menu index="group-npc">
          <template #title>
            <el-icon class="group-icon"><User /></el-icon>
            <span class="group-title">NPC 配置管理</span>
          </template>
          <el-menu-item index="/templates">
            <el-icon><Files /></el-icon>
            <span>模板管理</span>
          </el-menu-item>
          <el-menu-item index="/fields">
            <el-icon><List /></el-icon>
            <span>字段管理</span>
          </el-menu-item>
        </el-sub-menu>
        <el-sub-menu index="group-event">
          <template #title>
            <el-icon class="group-icon"><Lightning /></el-icon>
            <span class="group-title">事件源配置管理</span>
          </template>
          <el-menu-item index="/event-types">
            <el-icon><Lightning /></el-icon>
            <span>事件类型管理</span>
          </el-menu-item>
          <el-menu-item index="/event-type-schemas">
            <el-icon><Tickets /></el-icon>
            <span>扩展字段</span>
          </el-menu-item>
        </el-sub-menu>
        <el-sub-menu index="group-fsm">
          <template #title>
            <el-icon class="group-icon"><Cpu /></el-icon>
            <span class="group-title">状态机管理</span>
          </template>
          <el-menu-item index="/fsm-state-dicts">
            <el-icon><Collection /></el-icon>
            <span>状态字典</span>
          </el-menu-item>
        </el-sub-menu>
      </el-menu>
    </el-aside>
    <el-main style="padding: 0; background: #F5F7FA; overflow: hidden">
      <router-view :key="route.fullPath" />
    </el-main>
  </el-container>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { List, Files, Lightning, Tickets, User, Cpu, Collection } from '@element-plus/icons-vue'

const route = useRoute()

const activeMenu = computed(() => {
  if (route.path === '/') return '/fields'
  const parts = route.path.split('/')
  return '/' + (parts[1] || '')
})

// 哪些分组默认展开（所有一级分组 index）
const defaultOpeneds = ['group-npc', 'group-event', 'group-fsm']
</script>

<style scoped>
.sidebar {
  background: #1D2B3A;
  overflow-y: auto;
}

.sidebar-logo {
  padding: 20px;
  color: #fff;
  font-size: 16px;
  font-weight: 600;
  cursor: pointer;
}

.sidebar-sep {
  height: 1px;
  background: #2D3D4F;
  margin: 0;
}

.group-icon {
  color: #bfcbd9;
  font-size: 18px;
}

.group-title {
  font-size: 15px;
  font-weight: 600;
  color: #e6e8eb;
  letter-spacing: 0.5px;
}

:deep(.el-menu) {
  border-right: none;
}

/* sub-menu 一级分组标题栏 */
:deep(.el-sub-menu__title) {
  height: 52px;
  line-height: 52px;
  padding: 0 20px !important;
  background: #17212D !important;
}

:deep(.el-sub-menu__title:hover) {
  background: #1F2D3D !important;
}

:deep(.el-sub-menu .el-sub-menu__icon-arrow) {
  color: #8a9bae;
  right: 20px;
}

/* 二级菜单项 */
:deep(.el-menu-item) {
  height: 42px;
  line-height: 42px;
  padding-left: 44px !important;
  font-size: 14px;
}

:deep(.el-menu-item .el-icon) {
  font-size: 16px;
}

:deep(.el-menu-item:hover) {
  background: #1F2D3D !important;
}

:deep(.el-menu-item.is-active) {
  background-color: #409EFF !important;
  color: #fff !important;
}

:deep(.el-menu-item.is-active .el-icon) {
  color: #fff;
}
</style>
