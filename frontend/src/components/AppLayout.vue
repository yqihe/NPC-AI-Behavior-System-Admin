<template>
  <el-container style="height: 100vh">
    <el-aside width="220px" class="sidebar">
      <div class="sidebar-logo" @click="$router.push('/')">
        ADMIN 运营平台
      </div>
      <div class="sidebar-sep"></div>
      <el-menu
        :default-active="activeMenu"
        background-color="#1D2B3A"
        text-color="#bfcbd9"
        active-text-color="#409EFF"
        router
      >
        <el-menu-item-group>
          <template #title>
            <span class="menu-group-title">配置管理</span>
          </template>
          <el-menu-item index="/fields">
            <el-icon><component :is="iconList" /></el-icon>
            <span>字段管理</span>
          </el-menu-item>
        </el-menu-item-group>
      </el-menu>
    </el-aside>
    <el-main style="padding: 0; background: #F5F7FA; overflow: hidden">
      <router-view :key="route.fullPath" />
    </el-main>
  </el-container>
</template>

<script setup lang="ts">
import { computed, h } from 'vue'
import { useRoute } from 'vue-router'
import { List } from '@element-plus/icons-vue'

const route = useRoute()

const iconList = List

const activeMenu = computed(() => {
  if (route.path === '/') return '/fields'
  const parts = route.path.split('/')
  return '/' + (parts[1] || '')
})
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

.menu-group-title {
  color: #8a9bae;
  font-size: 12px;
}

:deep(.el-menu) {
  border-right: none;
}

:deep(.el-menu-item-group__title) {
  padding: 16px 20px 8px 20px;
}

:deep(.el-menu-item) {
  height: 40px;
  line-height: 40px;
}

:deep(.el-menu-item.is-active) {
  background-color: #409EFF !important;
  color: #fff !important;
}
</style>
