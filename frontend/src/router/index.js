import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '../views/HomeView.vue'
import { useAuthStore } from '../stores/auth'

const routes = [
  { path: '/', name: 'home', component: HomeView },
  {
    path: '/login',
    name: 'login',
    component: () => import('../views/LoginView.vue'),
    meta: { guestOnly: true },
  },
  {
    path: '/onboarding',
    name: 'onboarding',
    component: () => import('../views/OnboardingView.vue'),
    meta: { guestOnly: true },
  },
  // 之後會加入：/new、/edit/:id（V-5）、/@:username（V-6）、/@:username/:slug（V-4）
  {
    path: '/:pathMatch(.*)*',
    name: 'not-found',
    component: () => import('../views/NotFoundView.vue'),
  },
]

const router = createRouter({
  history: createWebHistory(),
  routes,
  scrollBehavior: () => ({ top: 0 }),
})

/**
 * 全域守衛。
 * 關鍵是第一行的 await：確保身分確認完成後才做判斷，
 * 否則頁面載入瞬間會把已登入的使用者誤判成訪客。
 */
router.beforeEach(async (to) => {
  const auth = useAuthStore()
  await auth.init()

  // 需要登入卻沒登入 → 導去登入頁，並記住原本要去哪
  if (to.meta.requiresAuth && !auth.isAuthenticated) {
    return { name: 'login', query: { redirect: to.fullPath } }
  }

  // 已登入卻要去登入 / onboarding 頁 → 沒有意義，導回首頁
  if (to.meta.guestOnly && auth.isAuthenticated) {
    return { name: 'home' }
  }

  return true
})

export default router