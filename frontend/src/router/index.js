import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '../views/HomeView.vue'

const routes = [
  { path: '/', name: 'home', component: HomeView },
  {
    path: '/login',
    name: 'login',
    component: () => import('../views/LoginView.vue'),
  },
  // 之後的步驟會陸續加入：
  //   /onboarding        （V-2）
  //   /new、/edit/:id    （V-5）
  //   /@:username        （V-6）
  //   /@:username/:slug  （V-4）
  {
    path: '/:pathMatch(.*)*',
    name: 'not-found',
    component: () => import('../views/NotFoundView.vue'),
  },
]

export default createRouter({
  history: createWebHistory(),
  routes,
  // 換頁時回到頂端（SPA 預設會保留捲動位置）
  scrollBehavior: () => ({ top: 0 }),
})