<script setup>
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { authApi } from '../api'

const auth = useAuthStore()
const router = useRouter()

function login() {
  window.location.href = authApi.loginUrl
}

async function handleLogout() {
  await auth.logout()
  // 若當前在需要登入的頁面，留在原地會看到錯誤畫面
  router.push('/')
}
</script>

<template>
  <header class="header">
    <div class="container header__inner">
      <RouterLink to="/" class="header__logo">Inkwell</RouterLink>

      <nav class="header__nav">
        <!-- ready 之前不顯示，避免閃過錯誤狀態 -->
        <template v-if="auth.ready">
          <template v-if="auth.isAuthenticated">
            <RouterLink to="/new" class="header__link">寫文章</RouterLink>
            <RouterLink to="/me/posts" class="header__link">我的文章</RouterLink>
            <RouterLink :to="`/@${auth.user.username}`" class="header__user">
              {{ auth.user.display_name || auth.user.username }}
            </RouterLink>
            <button class="btn btn--ghost" @click="handleLogout">登出</button>
          </template>
          <button v-else class="btn btn--primary" @click="login">使用 Google 登入</button>
        </template>
      </nav>
    </div>
  </header>
</template>

<style scoped>
.header {
  border-bottom: 1px solid var(--color-border);
  background: var(--color-surface);
}

.header__inner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: 60px;
}

.header__logo {
  font-family: var(--font-serif);
  font-size: var(--text-xl);
  font-weight: 700;
  letter-spacing: -0.01em;
}

.header__nav {
  display: flex;
  align-items: center;
  gap: var(--space-4);
}

.header__user {
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.header__link {
  font-size: var(--text-sm);
  color: var(--color-text-muted);
}

.header__link:hover {
  color: var(--color-accent);
}

/* vue-router 會自動為當前頁面的連結加上這個 class */
.header__link.router-link-active {
  color: var(--color-text);
  font-weight: 600;
}

.header__user:hover {
  color: var(--color-accent);
}
</style>