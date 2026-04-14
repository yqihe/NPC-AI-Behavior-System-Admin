import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      redirect: '/fields',
    },
    {
      path: '/fields',
      name: 'field-list',
      component: () => import('@/views/FieldList.vue'),
      meta: { title: '字段管理' },
    },
    {
      path: '/fields/create',
      name: 'field-create',
      component: () => import('@/views/FieldForm.vue'),
      meta: { title: '新建字段', isCreate: true },
    },
    {
      path: '/fields/:id/view',
      name: 'field-view',
      component: () => import('@/views/FieldForm.vue'),
      meta: { title: '查看字段', isCreate: false, isView: true },
    },
    {
      path: '/fields/:id/edit',
      name: 'field-edit',
      component: () => import('@/views/FieldForm.vue'),
      meta: { title: '编辑字段', isCreate: false },
    },
    {
      path: '/templates',
      name: 'template-list',
      component: () => import('@/views/TemplateList.vue'),
      meta: { title: '模板管理' },
    },
    {
      path: '/templates/create',
      name: 'template-create',
      component: () => import('@/views/TemplateForm.vue'),
      props: { mode: 'create' },
      meta: { title: '新建模板' },
    },
    {
      path: '/templates/:id/view',
      name: 'template-view',
      component: () => import('@/views/TemplateForm.vue'),
      props: (route) => ({ mode: 'view', id: Number(route.params.id) }),
      meta: { title: '查看模板' },
    },
    {
      path: '/templates/:id/edit',
      name: 'template-edit',
      component: () => import('@/views/TemplateForm.vue'),
      props: (route) => ({ mode: 'edit', id: Number(route.params.id) }),
      meta: { title: '编辑模板' },
    },
    {
      path: '/event-types',
      name: 'event-type-list',
      component: () => import('@/views/EventTypeList.vue'),
      meta: { title: '事件类型管理' },
    },
    {
      path: '/event-types/create',
      name: 'event-type-create',
      component: () => import('@/views/EventTypeForm.vue'),
      meta: { title: '新建事件类型', isCreate: true },
    },
    {
      path: '/event-types/:id/view',
      name: 'event-type-view',
      component: () => import('@/views/EventTypeForm.vue'),
      meta: { title: '查看事件类型', isCreate: false, isView: true },
    },
    {
      path: '/event-types/:id/edit',
      name: 'event-type-edit',
      component: () => import('@/views/EventTypeForm.vue'),
      meta: { title: '编辑事件类型', isCreate: false },
    },
    {
      path: '/event-type-schemas',
      name: 'event-type-schema-list',
      component: () => import('@/views/EventTypeSchemaList.vue'),
      meta: { title: '扩展字段管理' },
    },
    {
      path: '/event-type-schemas/create',
      name: 'event-type-schema-create',
      component: () => import('@/views/EventTypeSchemaForm.vue'),
      meta: { title: '新建扩展字段', isCreate: true },
    },
    {
      path: '/event-type-schemas/:id/view',
      name: 'event-type-schema-view',
      component: () => import('@/views/EventTypeSchemaForm.vue'),
      meta: { title: '查看扩展字段', isCreate: false, isView: true },
    },
    {
      path: '/event-type-schemas/:id/edit',
      name: 'event-type-schema-edit',
      component: () => import('@/views/EventTypeSchemaForm.vue'),
      meta: { title: '编辑扩展字段', isCreate: false },
    },
    {
      path: '/fsm-state-dicts',
      name: 'fsm-state-dict-list',
      component: () => import('@/views/FsmStateDictList.vue'),
      meta: { title: '状态字典' },
    },
    {
      path: '/fsm-state-dicts/create',
      name: 'fsm-state-dict-create',
      component: () => import('@/views/FsmStateDictForm.vue'),
      meta: { title: '新建状态', isCreate: true },
    },
    {
      path: '/fsm-state-dicts/:id/view',
      name: 'fsm-state-dict-view',
      component: () => import('@/views/FsmStateDictForm.vue'),
      meta: { title: '查看状态', isCreate: false, isView: true },
    },
    {
      path: '/fsm-state-dicts/:id/edit',
      name: 'fsm-state-dict-edit',
      component: () => import('@/views/FsmStateDictForm.vue'),
      meta: { title: '编辑状态', isCreate: false },
    },
    {
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      component: () => import('@/views/NotFound.vue'),
      meta: { title: '页面不存在' },
    },
  ],
})

export default router
