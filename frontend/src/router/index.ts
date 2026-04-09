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
  ],
})

export default router
