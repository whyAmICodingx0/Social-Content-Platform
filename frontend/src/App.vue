<script setup>
import { onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from './stores/auth'
import { useWsStore } from './stores/ws'
import AppHeader from './components/AppHeader.vue'
import LoadingState from './components/LoadingState.vue'
import { useUnreadStore } from './stores/unread'

const unread = useUnreadStore()

const auth = useAuthStore()
const ws = useWsStore()
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

// 全域監聽新訊息：不論在哪一頁，未讀數都要即時更新。
// 對話頁自己的 handler 負責顯示訊息，這裡只管數字。
ws.on('message.created', (data) => {
  // 自己傳的不算未讀
  if (data.sender?.username === auth.user?.username) return
  unread.increment()
})

// 登入時連線、登出時斷開。
// 用 watch 而非 onMounted：使用者可能在頁面停留期間登入或登出。
// immediate 會在 auth.init() 完成前先跑一次（此時未登入，
// 執行 disconnect() 無害），init() 完成後會再觸發一次。
// 登入時連線並取未讀數、登出時斷開並歸零
watch(
  () => auth.isAuthenticated,
  (isAuth) => {
    if (isAuth) {
      ws.connect()
      unread.refresh()
    } else {
      ws.disconnect()
      unread.reset()
    }
  },
  { immediate: true }
)
</script>

<template>
  <AppHeader />
  <main class="app-main">
    <!-- 身分確認完成前不渲染內容，避免畫面閃爍 -->
    <RouterView v-if="auth.ready" />
    <LoadingState v-else />
  </main>
</template>

<style scoped>
.app-main {
  padding: var(--space-8) 0 var(--space-16);
}
</style>