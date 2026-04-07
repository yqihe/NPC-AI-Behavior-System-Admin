import { createRouter, createWebHistory } from 'vue-router'
import {
  eventTypeApi,
  fsmConfigApi,
  btTreeApi,
  regionApi,
} from '@/api/generic'

/**
 * 为一个实体生成 list + new + edit 三条路由。
 */
function entityRoutes(path, title, api, options = {}) {
  const entityPath = path // 如 'event-types'
  const { allowSlash = false, configSchema = null, schemaName = null } = options

  return [
    {
      path: `/${path}`,
      name: `${path}-list`,
      component: () => import('@/views/GenericList.vue'),
      meta: { title, api, entityPath },
    },
    {
      path: `/${path}/new`,
      name: `${path}-new`,
      component: () => import('@/views/GenericForm.vue'),
      meta: { title, api, entityPath, allowSlash, configSchema, schemaName },
    },
    {
      path: `/${path}/:name(.*)`,
      name: `${path}-edit`,
      component: () => import('@/views/GenericForm.vue'),
      meta: { title, api, entityPath, allowSlash, configSchema, schemaName },
    },
  ]
}

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      name: 'Dashboard',
      component: () => import('@/views/Dashboard.vue'),
      meta: { title: '首页' },
    },

    // 配置管理 — NPC 模板使用专用页面
    {
      path: '/npc-templates',
      name: 'npc-templates-list',
      component: () => import('@/views/NpcTemplateList.vue'),
      meta: { title: 'NPC 模板管理' },
    },
    {
      path: '/npc-templates/new',
      name: 'npc-templates-new',
      component: () => import('@/views/NpcTemplateForm.vue'),
      meta: { title: 'NPC 模板' },
    },
    {
      path: '/npc-templates/:name(.*)',
      name: 'npc-templates-edit',
      component: () => import('@/views/NpcTemplateForm.vue'),
      meta: { title: 'NPC 模板' },
    },
    ...entityRoutes('event-types', '事件类型', eventTypeApi, { schemaName: '_event_type' }),
    // FSM — 专用编辑器
    {
      path: '/fsm-configs',
      name: 'fsm-configs-list',
      component: () => import('@/views/GenericList.vue'),
      meta: { title: '状态机管理', api: fsmConfigApi, entityPath: 'fsm-configs' },
    },
    {
      path: '/fsm-configs/new',
      name: 'fsm-configs-new',
      component: () => import('@/views/FsmConfigForm.vue'),
      meta: { title: '状态机' },
    },
    {
      path: '/fsm-configs/:name(.*)',
      name: 'fsm-configs-edit',
      component: () => import('@/views/FsmConfigForm.vue'),
      meta: { title: '状态机' },
    },
    // BT — 专用编辑器
    {
      path: '/bt-trees',
      name: 'bt-trees-list',
      component: () => import('@/views/GenericList.vue'),
      meta: { title: '行为树管理', api: btTreeApi, entityPath: 'bt-trees' },
    },
    {
      path: '/bt-trees/new',
      name: 'bt-trees-new',
      component: () => import('@/views/BtTreeForm.vue'),
      meta: { title: '行为树' },
    },
    {
      path: '/bt-trees/:name(.*)',
      name: 'bt-trees-edit',
      component: () => import('@/views/BtTreeForm.vue'),
      meta: { title: '行为树' },
    },

    // 世界管理
    ...entityRoutes('regions', '区域', regionApi, { schemaName: '_region' }),

    // 系统设置
    {
      path: '/schemas',
      name: 'Schemas',
      component: () => import('@/views/SchemaManager.vue'),
      meta: { title: 'Schema 管理' },
    },
    {
      path: '/exports',
      name: 'Exports',
      component: () => import('@/views/ExportManager.vue'),
      meta: { title: '导出管理' },
    },
  ],
})

export default router
