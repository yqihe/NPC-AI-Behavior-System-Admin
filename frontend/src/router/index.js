import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      name: 'Dashboard',
      component: () => import('@/views/Dashboard.vue'),
      meta: { title: '首页' },
    },
    // 配置管理
    {
      path: '/npc-templates',
      name: 'NpcTemplates',
      component: () => import('@/views/PlaceholderList.vue'),
      meta: { title: 'NPC 模板管理' },
    },
    {
      path: '/event-types',
      name: 'EventTypes',
      component: () => import('@/views/PlaceholderList.vue'),
      meta: { title: '事件类型管理' },
    },
    {
      path: '/fsm-configs',
      name: 'FsmConfigs',
      component: () => import('@/views/PlaceholderList.vue'),
      meta: { title: '状态机管理' },
    },
    {
      path: '/bt-trees',
      name: 'BtTrees',
      component: () => import('@/views/PlaceholderList.vue'),
      meta: { title: '行为树管理' },
    },
    // 世界管理
    {
      path: '/regions',
      name: 'Regions',
      component: () => import('@/views/PlaceholderList.vue'),
      meta: { title: '区域管理' },
    },
    // 系统设置
    {
      path: '/schemas',
      name: 'Schemas',
      component: () => import('@/views/PlaceholderList.vue'),
      meta: { title: 'Schema 管理' },
    },
    {
      path: '/exports',
      name: 'Exports',
      component: () => import('@/views/PlaceholderList.vue'),
      meta: { title: '导出管理' },
    },
  ],
})

export default router
