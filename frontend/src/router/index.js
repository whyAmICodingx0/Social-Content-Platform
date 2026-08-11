import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '../views/HomeView.vue'
import { useAuthStore } from '../stores/auth'
import { setTitle } from '../utils/title'

const routes = [
  { path: '/', name: 'home', component: HomeView },
  {
    path: '/login',
    name: 'login',
    component: () => import('../views/LoginView.vue'),
    meta: { guestOnly: true, title: '登入' },
  },
  {
    path: '/onboarding',
    name: 'onboarding',
    component: () => import('../views/OnboardingView.vue'),
    meta: { guestOnly: true, title: '選擇 username' },
  },
  {
    // @ 是網址中的字面字元，:username 與 :slug 是參數。
    // /@alice/my-post → params = { username: 'alice', slug: 'my-post' }
    path: '/@:username/:slug',
    name: 'post',
    component: () => import('../views/PostView.vue'),
  },
  {
    path: '/new',
    name: 'post-new',
    component: () => import('../views/EditorView.vue'),
    meta: { requiresAuth: true, title: '寫新文章' },
  },
  {
    // 比 /@:username/:slug 多一段固定的 edit，vue-router 會正確區分
    path: '/@:username/:slug/edit',
    name: 'post-edit',
    component: () => import('../views/EditorView.vue'),
    meta: { requiresAuth: true },
  },
  {
    path: '/me/posts',
    name: 'my-posts',
    component: () => import('../views/MyPostsView.vue'),
    meta: { requiresAuth: true, title: '我的文章' },
  },
  {
    path: '/@:username',
    name: 'profile',
    component: () => import('../views/ProfileView.vue'),
  },
  {
    path: '/settings',
    name: 'settings',
    component: () => import('../views/SettingsView.vue'),
    meta: { requiresAuth: true, title: '個人檔案設定' },
  },
  {
    path: '/:pathMatch(.*)*',
    name: 'not-found',
    component: () => import('../views/NotFoundView.vue'),
    meta: { title: '找不到頁面' }
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

/**
 * 導航完成後套用靜態標題。
 * 動態頁面（文章、個人頁）會在資料載入後由元件自行覆蓋——
 * 元件的執行時機晚於這裡，所以不會被蓋回去。
 */
router.afterEach((to) => {
  setTitle(to.meta.title)
})

export default router