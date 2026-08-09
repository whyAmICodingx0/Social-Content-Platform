<script setup>
import { onMounted } from 'vue'
import { useAuthStore } from './stores/auth'
import AppHeader from './components/AppHeader.vue'

const auth = useAuthStore()

// App 啟動時確認一次登入狀態（cookie 讀不到，只能問後端）
onMounted(() => auth.fetchMe())
</script>

<template>
  <AppHeader />
  <main class="app-main">
    <RouterView />
  </main>
</template>

<style scoped>
.app-main {
  padding: var(--space-8) 0 var(--space-16);
}
</style>