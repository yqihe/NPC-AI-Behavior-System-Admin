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
      path: '/event-types/:id/edit',
      name: 'event-type-edit',
      component: () => import('@/views/EventTypeForm.vue'),
      meta: { title: '编辑事件类型', isCreate: false },
    },
  ],
})

export default router
