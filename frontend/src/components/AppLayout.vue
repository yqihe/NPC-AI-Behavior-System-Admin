<template>
  <el-container style="height: 100vh">
    <el-aside width="220px" style="background: #1D2B3A">
      <div style="padding: 20px; color: #fff; font-size: 16px; font-weight: 600">
        ADMIN 运营平台
      </div>
    </el-aside>
    <el-main style="padding: 0; background: #F5F7FA">
      <router-view :key="route.fullPath" />
    </el-main>
  </el-container>
</template>

<script setup>
import { useRoute } from 'vue-router'

const route = useRoute()
</script>
