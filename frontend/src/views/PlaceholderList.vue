<template>
  <div class="placeholder-page">
    <h2>{{ title }}</h2>
    <el-empty description="待接入动态表单">
      <template #image>
        <span style="font-size: 48px;">📋</span>
      </template>
    </el-empty>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useRoute } from 'vue-router'

const route = useRoute()
const title = computed(() => route.meta?.title || '页面加载中')
</script>

<style scoped>
.placeholder-page {
  padding: 24px;
}
.placeholder-page h2 {
  margin-bottom: 24px;
  color: #303133;
  font-size: 20px;
}
</style>
