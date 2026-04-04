import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      name: 'Dashboard',
      component: () => import('@/views/Dashboard.vue'),
    },
    {
      path: '/event-types',
      name: 'EventTypeList',
      component: () => import('@/views/EventTypeList.vue'),
    },
    {
      path: '/event-types/:name',
      name: 'EventTypeForm',
      component: () => import('@/views/EventTypeForm.vue'),
    },
    {
      path: '/fsm-configs',
      name: 'FsmConfigList',
      component: () => import('@/views/FsmConfigList.vue'),
    },
    {
      path: '/fsm-configs/:name',
      name: 'FsmConfigForm',
      component: () => import('@/views/FsmConfigForm.vue'),
    },
    {
      path: '/bt-trees',
      name: 'BtTreeList',
      component: () => import('@/views/BtTreeList.vue'),
    },
    {
      path: '/bt-trees/:name(.*)',
      name: 'BtTreeForm',
      component: () => import('@/views/BtTreeForm.vue'),
    },
    {
      path: '/npc-types',
      name: 'NpcTypeList',
      component: () => import('@/views/NpcTypeList.vue'),
    },
    {
      path: '/npc-types/:name',
      name: 'NpcTypeForm',
      component: () => import('@/views/NpcTypeForm.vue'),
    },
  ],
})

export default router
