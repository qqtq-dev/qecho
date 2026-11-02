import { createRouter, createWebHashHistory } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

// Tauri 内静态资源加载使用 hash 模式，避免刷新 404
const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    {
      path: '/',
      component: () => import('@/layouts/BasicLayout.vue'),
      redirect: '/dashboard',
      children: [
        { path: 'dashboard', component: () => import('@/views/dashboard/index.vue'), meta: { title: '概览' } },
        { path: 'banks', component: () => import('@/views/banks/index.vue'), meta: { title: '题库管理' } },
        { path: 'banks/:id', component: () => import('@/views/banks/detail.vue'), meta: { title: '题库详情' } },
        { path: 'practice', component: () => import('@/views/practice/index.vue'), meta: { title: '练习' } },
        { path: 'exams', component: () => import('@/views/exams/index.vue'), meta: { title: '考试' } },
        { path: 'wrong', component: () => import('@/views/wrong/index.vue'), meta: { title: '错题本' } },
        { path: 'stats', component: () => import('@/views/stats/index.vue'), meta: { title: '统计' } },
        {
          path: 'users',
          component: () => import('@/views/users/index.vue'),
          meta: { title: '用户管理', admin: true },
        },
        {
          path: 'backups',
          component: () => import('@/views/backups/index.vue'),
          meta: { title: '数据备份', admin: true },
        },
        { path: 'settings', component: () => import('@/views/settings/index.vue'), meta: { title: '设置' } },
      ],
    },
    {
      path: '/login',
      component: () => import('@/views/auth/login.vue'),
      meta: { public: true },
    },
    {
      path: '/register',
      component: () => import('@/views/auth/register.vue'),
      meta: { public: true },
    },
    { path: '/attempts/:id/play', component: () => import('@/views/attempt/play.vue') },
    { path: '/attempts/:id/result', component: () => import('@/views/attempt/result.vue') },
    { path: '/:pathMatch(.*)*', redirect: '/dashboard' },
  ],
})

router.beforeEach((to) => {
  const auth = useAuthStore()
  if (to.meta.public) {
    if (auth.loggedIn && (to.path === '/login' || to.path === '/register')) {
      return { path: '/dashboard' }
    }
    return true
  }
  if (!auth.loggedIn) {
    return { path: '/login', query: { redirect: to.fullPath } }
  }
  // 管理员专用页面：非管理员直接回概览，避免停在无数据的空页面
  if (to.meta.admin && !auth.isAdmin) {
    return { path: '/dashboard' }
  }
  return true
})

export default router
