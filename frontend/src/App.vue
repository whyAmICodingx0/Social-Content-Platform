<script setup>
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from './stores/auth'
import AppHeader from './components/AppHeader.vue'
import LoadingState from './components/LoadingState.vue'

const auth = useAuthStore()
const router = useRouter()

onMounted(async () => {
  await auth.init()

  // 登入前若記錄過「原本要去的頁面」，登入成功後帶他回去。
  // OAuth 是整頁跳轉，前端狀態會消失，所以用 sessionStorage 傳遞。
  const target = sessionStorage.getItem('post_login_redirect')
  if (target) {
    sessionStorage.removeItem('post_login_redirect')
    if (auth.isAuthenticated) router.replace(target)
  }
})
</script>

<template>
  <AppHeader />
  <main class="app-main">
    <!-- 身分確認完成前不渲染內容，避免畫面閃爍 -->
    <RouterView v-if="auth.ready" />
    <RouterView v-if="auth.ready" />
    <LoadingState v-else />
  </main>
</template>

<style scoped>
.app-main {
  padding: var(--space-8) 0 var(--space-16);
}
</style>